package portscan

import (
	"context"
	"net"
)

// Resolver определяет интерфейс для DNS-резолвинга хостов в IP-адреса.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}
