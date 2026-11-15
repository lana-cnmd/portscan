package portscan

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrNoTargets возвращается методом Scan,
	// если список хостов пуст или не содержит ни одного валидного хоста.
	ErrNoTargets = errors.New("portscan: no targets to scan")
	// ErrNoPorts возвращается методом Scan,
	// если не задано ни одного порта или все PortSpec оказались пустыми.
	ErrNoPorts = errors.New("portscan: no ports to scan")
)

// InvalidPortError описывает ошибку невалидного значения порта.
// Возвращается функциями Ports и Range, если порт равен 0.
type InvalidPortError struct {
	Port uint16
}

func (e *InvalidPortError) Error() string {
	return fmt.Sprintf("portscan: invalid port %d", e.Port)
}

// InvalidRangeError описывает ошибку обратного диапазона при задании портов.
// Возвращается функцией Range, если Start больше End.
type InvalidRangeError struct {
	Start, End uint16
}

func (e *InvalidRangeError) Error() string {
	return fmt.Sprintf("portscan: invalid range %d-%d (start must be <= end)", e.Start, e.End)
}

// InvalidConcurrencyError описывает ошибку невалидного значения количества одновременных проверок.
// Возвращается функцией WithConcurrency, если передано значение <= 0.
type InvalidConcurrencyError struct {
	Value int
}

func (e *InvalidConcurrencyError) Error() string {
	return fmt.Sprintf("portscan: invalid concurrency value %d (must be > 0)", e.Value)
}

// InvalidTimeoutError описывает ошибку невалидного значения таймаута подключения.
// Возвращается функцией WithConnectTimeout, если передано значение <= 0.
type InvalidTimeoutError struct {
	Value time.Duration
}

func (e *InvalidTimeoutError) Error() string {
	return fmt.Sprintf("portscan: invalid connection timeout value %v (must be > 0)", e.Value)
}
