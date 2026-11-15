package portscan

import (
	"net"
	"time"
)

// Result содержит результат проверки одного TCP-порта.
type Result struct {
	Host     string
	IP       net.IP
	Port     uint16
	State    State
	Duration time.Duration
	Err      error
}
