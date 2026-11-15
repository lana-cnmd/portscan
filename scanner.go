package portscan

import (
	"context"
	"time"
)

type config struct {
	concurrency    int
	connectTimeout time.Duration
	resolver       Resolver
}

type Option func(*config) error

type Scanner struct {
	cfg config
}

func New(opts ...Option) (*Scanner, error) {
	// TODO:
	return nil, nil

}

func (s *Scanner) Scan(ctx context.Context, hosts []string, ports ...PortSpec) (<-chan Result, error) {
	// TODO:
}
func WithConcurrency(n int) Option {
	// TODO:
}

func WithConnectTimeout(d time.Duration) Option {} // TODO:
func WithResolver(r Resolver) Option {
	// TODO:
}
