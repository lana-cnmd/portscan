package portscan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type config struct {
	concurrency    int
	connectTimeout time.Duration
	resolver       Resolver
}

type Option func(*config) error

// Scanner выполняет TCP сканирование хостов и портов (устанавливая TCP-соединения).
type Scanner struct {
	cfg *config
}

type scanJob struct {
	host string
	ip   net.IPAddr
	port uint16
}

// New создает новый экземпляр Scanner с заданными параметрами.
func New(opts ...Option) (*Scanner, error) {
	cfg := &config{
		concurrency:    10,
		connectTimeout: time.Second,
		resolver:       net.DefaultResolver,
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("portscan: invalid option: %w", err)
		}
	}

	return &Scanner{cfg: cfg}, nil
}

// Scan запускает сканирование TCP портов для указанных хостов и параметров портов.
// Метод возвращает канал, из которого можно считывать результаты по мере их поступления.
// Канал закрывается после того, как все результаты будут переданы, либо в случае отмены контекста.
func (s *Scanner) Scan(ctx context.Context, hosts []string, portSpecs ...PortSpec) (<-chan Result, error) {
	uniqueHosts, err := normalizeHosts(hosts)
	if err != nil {
		return nil, err
	}

	ports, err := mergePorts(portSpecs...)
	if err != nil {
		return nil, err
	}

	resChan := make(chan Result, s.cfg.concurrency)
	jobs := make(chan scanJob, s.cfg.concurrency)

	wg := &sync.WaitGroup{}

	for range s.cfg.concurrency {
		wg.Go(func() {
			for job := range jobs {
				select {
				case resChan <- s.scanSinglePort(ctx, job.host, job.ip, job.port):
				case <-ctx.Done():
					return
				}
			}
		})
	}

	go func() {
		defer close(jobs)

		for _, host := range uniqueHosts {
			if ctx.Err() != nil {
				break
			}

			ips, err := s.resolveHost(ctx, host)
			if err != nil {
				for _, port := range ports {
					select {
					case resChan <- Result{Host: host, Port: port, Err: err, State: StateError}:
					case <-ctx.Done():
						return
					}
				}

				continue
			}

			for _, ipAddr := range ips {
				for _, port := range ports {
					select {
					case jobs <- scanJob{host: host, port: port, ip: ipAddr}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resChan)
	}()

	return resChan, nil
}

func (s *Scanner) scanSinglePort(ctx context.Context, host string, ipAddr net.IPAddr, port uint16) Result {
	addr := net.JoinHostPort(ipAddr.String(), strconv.Itoa(int(port)))
	now := time.Now()
	d := net.Dialer{Timeout: s.cfg.connectTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	durr := time.Since(now)

	if err == nil {
		conn.Close()
	}

	state := classify(err)

	return Result{Host: host, IP: ipAddr.IP, Port: port, Duration: durr, State: state, Err: err}
}

func (s *Scanner) resolveHost(ctx context.Context, host string) ([]net.IPAddr, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}

	ips, err := s.cfg.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("dns lookup failed for %s: %w", host, err)
	}

	return ips, nil
}

func classify(err error) State {
	var netErr net.Error

	switch {
	case err == nil:
		return StateOpen
	case errors.Is(err, context.Canceled):
		return StateCanceled
	case errors.As(err, &netErr) && netErr.Timeout():
		return StateTimeout
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return StateUnreachable
	case errors.Is(err, syscall.ECONNREFUSED):
		return StateClosed
	default:
		return StateError
	}
}

// WithConcurrency задает значение concurrency для конфига вместо дефолтного.
// Возвращает ошибку, если значение n <= 0.
func WithConcurrency(n int) Option {
	return func(c *config) error {
		if n <= 0 {
			return &InvalidConcurrencyError{Value: n}
		}

		c.concurrency = n

		return nil
	}
}

// WithConnectTimeout задает значение таймаута подключения для конфига вместо дефолтного.
// Возвращает ошибку, если значение d <= 0.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *config) error {
		if d <= 0 {
			return &InvalidTimeoutError{Value: d}
		}

		c.connectTimeout = d

		return nil
	}
}

// WithResolver задает резолвер вместо дефолтного.
// Возвращает ошибку, если значение r nil.
func WithResolver(r Resolver) Option {
	return func(c *config) error {
		if r == nil {
			return ErrNilResolver
		}

		v := reflect.ValueOf(r)
		if v.Kind() == reflect.Pointer && v.IsNil() {
			return ErrNilResolver
		}

		c.resolver = r

		return nil
	}
}
