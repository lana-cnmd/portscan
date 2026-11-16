package portscan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tt := []struct {
		name             string
		opts             []Option
		expectedErr      error
		expectedConc     int
		expectedTimeout  time.Duration
		expectedResolver *net.Resolver
	}{
		{
			name:             "No options - defaults",
			opts:             nil,
			expectedErr:      nil,
			expectedConc:     10,
			expectedTimeout:  time.Second,
			expectedResolver: net.DefaultResolver,
		},
		{
			name:             "WithConcurrency valid",
			opts:             []Option{WithConcurrency(100)},
			expectedErr:      nil,
			expectedConc:     100,
			expectedTimeout:  time.Second,
			expectedResolver: net.DefaultResolver,
		},
		{
			name:        "WithConcurrency zero",
			opts:        []Option{WithConcurrency(0)},
			expectedErr: &InvalidConcurrencyError{Value: 0},
		},
		{
			name:        "WithConcurrency negative",
			opts:        []Option{WithConcurrency(-5)},
			expectedErr: &InvalidConcurrencyError{Value: -5},
		},
		{
			name:        "WithConnectTimeout zero",
			opts:        []Option{WithConnectTimeout(0)},
			expectedErr: &InvalidTimeoutError{Value: 0},
		},
		{
			name:        "WithConnectTimeout negative",
			opts:        []Option{WithConnectTimeout(-time.Second)},
			expectedErr: &InvalidTimeoutError{Value: -time.Second},
		},
		{
			name:             "WithResolver valid",
			opts:             []Option{WithResolver(&net.Resolver{})},
			expectedErr:      nil,
			expectedConc:     10,
			expectedTimeout:  time.Second,
			expectedResolver: &net.Resolver{},
		},
		{
			name:        "WithResolver nil",
			opts:        []Option{WithResolver(nil)},
			expectedErr: ErrNilResolver,
		},
		{
			name:        "WithResolver typed nil",
			opts:        []Option{WithResolver((*net.Resolver)(nil))},
			expectedErr: ErrNilResolver,
		},
		{
			name:             "Combination of options",
			opts:             []Option{WithConcurrency(100), WithConnectTimeout(500 * time.Millisecond)},
			expectedErr:      nil,
			expectedConc:     100,
			expectedTimeout:  500 * time.Millisecond,
			expectedResolver: net.DefaultResolver,
		},
		{
			name:        "First option fails",
			opts:        []Option{WithConcurrency(0), WithConnectTimeout(time.Second)},
			expectedErr: &InvalidConcurrencyError{Value: 0},
		},
		{
			name:        "Middle option fails",
			opts:        []Option{WithConnectTimeout(time.Second), WithConcurrency(0), WithResolver(nil)},
			expectedErr: &InvalidConcurrencyError{Value: 0},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			scanner, err := New(tc.opts...)

			if tc.expectedErr != nil {
				require.Error(t, err)
				require.Nil(t, scanner)

				var (
					actualConc    *InvalidConcurrencyError
					actualTimeout *InvalidTimeoutError
				)

				{
					var expected *InvalidConcurrencyError
					var expected1 *InvalidTimeoutError
					switch {
					case errors.As(tc.expectedErr, &expected):
						require.ErrorAs(t, err, &actualConc)
						assert.Equal(t, expected.Value, actualConc.Value)
					case errors.As(tc.expectedErr, &expected1):
						require.ErrorAs(t, err, &actualTimeout)
						assert.Equal(t, expected1.Value, actualTimeout.Value)
					default:
						require.Contains(t, err.Error(), tc.expectedErr.Error())
					}
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, scanner)
			require.NotNil(t, scanner.cfg)
			assert.Equal(t, tc.expectedConc, scanner.cfg.concurrency)
			assert.Equal(t, tc.expectedTimeout, scanner.cfg.connectTimeout)
			assert.Equal(t, tc.expectedResolver, scanner.cfg.resolver)
		})
	}
}

// timeoutErr минимальная реализация net.Error с Timeout() == true.
type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

func TestClassify(t *testing.T) {
	tt := []struct {
		name          string
		err           error
		expectedState State
	}{
		{
			name:          "Nil error",
			err:           nil,
			expectedState: StateOpen,
		},
		{
			name:          "Canceled",
			err:           context.Canceled,
			expectedState: StateCanceled,
		},
		{
			name:          "DeadlineExceeded",
			err:           context.DeadlineExceeded,
			expectedState: StateTimeout,
		},
		{
			name:          "Network timeout",
			err:           timeoutErr{},
			expectedState: StateTimeout,
		},
		{
			name:          "Wrapped network timeout",
			err:           fmt.Errorf("dial: %w", timeoutErr{}),
			expectedState: StateTimeout,
		},
		{
			name:          "Connection refused",
			err:           syscall.ECONNREFUSED,
			expectedState: StateClosed,
		},
		{
			name:          "Wrapped connection refused",
			err:           fmt.Errorf("dial: %w", syscall.ECONNREFUSED),
			expectedState: StateClosed,
		},
		{
			name:          "Host unreachable",
			err:           syscall.EHOSTUNREACH,
			expectedState: StateUnreachable,
		},
		{
			name:          "Wrapped host unreachable",
			err:           fmt.Errorf("dial: %w", syscall.EHOSTUNREACH),
			expectedState: StateUnreachable,
		},
		{
			name:          "Net unreachable",
			err:           syscall.ENETUNREACH,
			expectedState: StateUnreachable,
		},
		{
			name:          "Wrapped net unreachable",
			err:           fmt.Errorf("dial: %w", syscall.ENETUNREACH),
			expectedState: StateUnreachable,
		},
		{
			name:          "Unknown error",
			err:           errors.New("Unknown error"),
			expectedState: StateError,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedState, classify(tc.err))
		})
	}
}

type fakeResolver struct {
	ips   map[string][]net.IPAddr
	err   error
	calls []string
}

func (f *fakeResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	f.calls = append(f.calls, host)
	if f.err != nil {
		return nil, f.err
	}

	return f.ips[host], nil
}

func TestScanner_resolveHost(t *testing.T) {
	dnsErr := errors.New("dns server unavailable")

	tt := []struct {
		name            string
		host            string
		resolverIPs     map[string][]net.IPAddr
		resolverErr     error
		expectedAddrs   []net.IPAddr
		expectedErrIs   error
		expectedErrText string
		expectResolver  bool
		expectedHost    string
	}{
		{
			name:           "IPv4 literal",
			host:           "192.168.1.1",
			expectedAddrs:  []net.IPAddr{{IP: net.ParseIP("192.168.1.1")}},
			expectResolver: false,
		},
		{
			name:           "IPv6 literal",
			host:           "::1",
			expectedAddrs:  []net.IPAddr{{IP: net.ParseIP("::1")}},
			expectResolver: false,
		},
		{
			name: "DNS success",
			host: "google.com",
			resolverIPs: map[string][]net.IPAddr{
				"google.com": {{IP: net.ParseIP("142.250.74.46")}},
			},
			expectedAddrs:  []net.IPAddr{{IP: net.ParseIP("142.250.74.46")}},
			expectResolver: true,
			expectedHost:   "google.com",
		},
		{
			name:           "DNS error",
			host:           "google.com",
			resolverErr:    dnsErr,
			expectedErrIs:  dnsErr,
			expectResolver: true,
			expectedHost:   "google.com",
		},
		{
			name:            "DNS error wrapped with host",
			host:            "google.com",
			resolverErr:     dnsErr,
			expectedErrText: "dns lookup failed for google.com: dns server unavailable",
			expectResolver:  true,
			expectedHost:    "google.com",
		},
		{
			name: "Multi-IP",
			host: "example.com",
			resolverIPs: map[string][]net.IPAddr{
				"example.com": {
					{IP: net.ParseIP("1.1.1.1")},
					{IP: net.ParseIP("2.2.2.2")},
					{IP: net.ParseIP("3.3.3.3")},
				},
			},
			expectedAddrs: []net.IPAddr{
				{IP: net.ParseIP("1.1.1.1")},
				{IP: net.ParseIP("2.2.2.2")},
				{IP: net.ParseIP("3.3.3.3")},
			},
			expectResolver: true,
			expectedHost:   "example.com",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			resolver := &fakeResolver{
				ips: tc.resolverIPs,
				err: tc.resolverErr,
			}
			s := &Scanner{cfg: &config{resolver: resolver}}

			addrs, err := s.resolveHost(context.Background(), tc.host)

			if tc.expectedErrIs != nil || tc.expectedErrText != "" {
				require.Error(t, err)
				assert.Nil(t, addrs)

				if tc.expectedErrIs != nil {
					require.ErrorIs(t, err, tc.expectedErrIs)
				}

				if tc.expectedErrText != "" {
					assert.EqualError(t, err, tc.expectedErrText)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedAddrs, addrs)
			}

			if tc.expectResolver {
				require.Len(t, resolver.calls, 1, "resolver must be called exactly once")
				assert.Equal(t, tc.expectedHost, resolver.calls[0])
			} else {
				assert.Empty(t, resolver.calls, "resolver must not be called for IP literal")
			}
		})
	}
}

func freePort(t *testing.T) uint16 {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	return uint16(l.Addr().(*net.TCPAddr).Port)
}

func TestScanSinglePort(t *testing.T) {
	tt := []struct {
		name      string
		setup     func(t *testing.T) (ctx context.Context, host string, ip net.IPAddr, port uint16, cleanup func())
		timeout   time.Duration
		wantState State
	}{
		{
			name: "Open port",
			setup: func(t *testing.T) (context.Context, string, net.IPAddr, uint16, func()) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				port := uint16(ln.Addr().(*net.TCPAddr).Port)

				return context.Background(),
					"127.0.0.1",
					net.IPAddr{IP: net.ParseIP("127.0.0.1")},
					port,
					func() { ln.Close() }
			},
			timeout:   500 * time.Millisecond,
			wantState: StateOpen,
		},
		{
			name: "Closed port",
			setup: func(t *testing.T) (context.Context, string, net.IPAddr, uint16, func()) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)

				port := uint16(ln.Addr().(*net.TCPAddr).Port)
				ln.Close()

				return context.Background(),
					"127.0.0.1",
					net.IPAddr{IP: net.ParseIP("127.0.0.1")},
					port,
					func() {}
			},
			timeout:   500 * time.Millisecond,
			wantState: StateClosed,
		},
		{
			name: "Timeout",
			setup: func(t *testing.T) (context.Context, string, net.IPAddr, uint16, func()) {
				return context.Background(),
					"192.0.2.1",
					net.IPAddr{IP: net.ParseIP("192.0.2.1")},
					80,
					func() {}
			},
			timeout:   100 * time.Millisecond,
			wantState: StateTimeout,
		},
		{
			name: "Canceled context",
			setup: func(t *testing.T) (context.Context, string, net.IPAddr, uint16, func()) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx,
					"192.0.2.1",
					net.IPAddr{IP: net.ParseIP("192.0.2.1")},
					80,
					func() {}
			},
			timeout:   500 * time.Millisecond,
			wantState: StateCanceled,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx, host, ip, port, cleanup := tc.setup(t)
			defer cleanup()

			scanner, err := New(WithConnectTimeout(tc.timeout))
			require.NoError(t, err)

			res := scanner.scanSinglePort(ctx, host, ip, port)

			assert.Equal(t, tc.wantState, res.State)
			assert.Equal(t, host, res.Host)
			assert.Equal(t, port, res.Port)
			assert.Greater(t, res.Duration, time.Duration(0))

			if tc.wantState == StateOpen {
				assert.NoError(t, res.Err)
			} else {
				assert.Error(t, res.Err)
			}
		})
	}
}

func TestScan_BasicScenarios(t *testing.T) {
	tt := []struct {
		name      string
		setup     func(t *testing.T) (hosts []string, ports []uint16, resolver Resolver, cleanup func())
		wantLen   int
		wantState State
	}{
		{
			name: "Open port",
			setup: func(t *testing.T) ([]string, []uint16, Resolver, func()) {
				ln, _ := net.Listen("tcp", "127.0.0.1:0")
				port := uint16(ln.Addr().(*net.TCPAddr).Port)

				return []string{"127.0.0.1"}, []uint16{port}, nil, func() { ln.Close() }
			},
			wantLen:   1,
			wantState: StateOpen,
		},
		{
			name: "DNS failure",
			setup: func(t *testing.T) ([]string, []uint16, Resolver, func()) {
				fake := &fakeResolver{err: fmt.Errorf("no such host")}

				return []string{"nosuchhost.invalid"}, []uint16{80, 443}, fake, func() {}
			},
			wantLen:   2, // 1 хост × 2 порта
			wantState: StateError,
		},
		{
			name: "Multi-IP DNS",
			setup: func(t *testing.T) ([]string, []uint16, Resolver, func()) {
				ln, _ := net.Listen("tcp", "127.0.0.1:0")
				port := uint16(ln.Addr().(*net.TCPAddr).Port)
				fake := &fakeResolver{
					ips: map[string][]net.IPAddr{
						"multi.test": {
							{IP: net.ParseIP("127.0.0.1")},
							{IP: net.ParseIP("127.0.0.1")},
						},
					},
				}

				return []string{"multi.test"}, []uint16{port}, fake, func() { ln.Close() }
			},
			wantLen:   2, // 2 IP × 1 порт
			wantState: StateOpen,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			hosts, ports, resolver, cleanup := tc.setup(t)
			defer cleanup()

			var opts []Option
			if resolver != nil {
				opts = append(opts, WithResolver(resolver))
			}

			scanner, err := New(opts...)
			require.NoError(t, err)

			spec, err := Ports(ports...)
			require.NoError(t, err)

			ctx := context.Background()
			resChan, err := scanner.Scan(ctx, hosts, spec)
			require.NoError(t, err)

			var results []Result
			for r := range resChan {
				results = append(results, r)
			}

			require.Len(t, results, tc.wantLen)

			for _, r := range results {
				assert.Equal(t, tc.wantState, r.State)
			}
		})
	}
}

func TestScan_CancelContext(t *testing.T) {
	scanner, _ := New(
		WithConnectTimeout(2*time.Second),
		WithConcurrency(2),
	)
	spec, _ := Range(1, 50)

	ctx, cancel := context.WithCancel(context.Background())
	resChan, _ := scanner.Scan(ctx, []string{"192.0.2.1"}, spec)

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})

	go func() {
		for range resChan {
		}

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("канал не закрылся после отмены контекста")
	}
}

func TestScan_SlowReader(t *testing.T) {
	scanner, _ := New(WithConcurrency(10), WithConnectTimeout(200*time.Millisecond))
	spec, _ := Range(50000, 50010)

	ctx := context.Background()
	resChan, _ := scanner.Scan(ctx, []string{"127.0.0.1"}, spec)

	var results []Result
	for r := range resChan {
		results = append(results, r)

		time.Sleep(20 * time.Millisecond)
	}

	require.Len(t, results, 11)
}

func TestScan_GoroutineLeak(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	port := uint16(ln.Addr().(*net.TCPAddr).Port)

	before := runtime.NumGoroutine()

	scanner, _ := New(WithConcurrency(2))
	spec, _ := Ports(port)

	ctx := context.Background()
	resChan, _ := scanner.Scan(ctx, []string{"127.0.0.1"}, spec)

	for range resChan {
	}

	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > before+1 {
		t.Errorf("утечка горутин: before=%d, after=%d", before, after)
	}
}

func TestScan_ParallelCalls(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()

	port := uint16(ln.Addr().(*net.TCPAddr).Port)

	scanner, _ := New(WithConcurrency(5))
	spec, _ := Ports(port)

	const numCalls = 10

	var wg sync.WaitGroup
	wg.Add(numCalls)

	for i := 0; i < numCalls; i++ {
		go func() {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			resChan, err := scanner.Scan(ctx, []string{"127.0.0.1"}, spec)
			if err != nil {
				return
			}

			for range resChan {
			}
		}()
	}

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("параллельные вызовы зависли")
	}
}

func TestScan_LargeScan(t *testing.T) {
	scanner, _ := New(WithConcurrency(20), WithConnectTimeout(100*time.Millisecond))
	spec, _ := Range(50000, 50099)

	ctx := context.Background()
	resChan, _ := scanner.Scan(ctx, []string{"127.0.0.1"}, spec)

	var results []Result
	for r := range resChan {
		results = append(results, r)
	}

	require.Len(t, results, 100)
}
