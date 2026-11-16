# portscan

Go-пакет для TCP Connect-сканирования портов на одном или нескольких хостах. Поддерживает IPv4, IPv6 и DNS-имена, конкурентное выполнение с настраиваемым лимитом, отмену через `context.Context` и возврат результатов через канал.

## Требования

- Go 1.25.5

## Установка

```bash
go get github.com/lana-cnmd/portscan
```

## Использование

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/твой-ник/portscan"
)

func main() {
    scanner, err := portscan.New(
        portscan.WithConcurrency(100),
        portscan.WithConnectTimeout(500*time.Millisecond),
    )
    if err != nil {
        panic(err)
    }

    ctx := context.Background()
    results, err := scanner.Scan(
        ctx,
        []string{"192.168.1.10", "example.com"},
        portscan.Range(1, 1000),
        portscan.Ports(8080, 8443),
    )
    if err != nil {
        panic(err)
    }

    for r := range results {
        fmt.Printf("%s (%s):%d %s\n", r.Host, r.IP, r.Port, r.State)
    }
}
```

## Публичный API

### Создание сканера

```go
func New(opts ...Option) (*Scanner, error)
```

Дефолты: `concurrency=10`, `connectTimeout=1s`, `resolver=net.DefaultResolver`.

### Опции

| Опция | Описание | Ошибка |
|-------|----------|--------|
| `WithConcurrency(n int)` | Лимит одновременных проверок | `*InvalidConcurrencyError` при `n <= 0` |
| `WithConnectTimeout(d time.Duration)` | Таймаут TCP-подключения | `*InvalidTimeoutError` при `d <= 0` |
| `WithResolver(r Resolver)` | Подмена DNS-резолвера | `ErrNilResolver` при `nil` или typed nil |

### Порты

```go
func Ports(ports ...uint16) (PortSpec, error)
func Range(start, end uint16) (PortSpec, error)
```

`Ports` — список портов, `Range` — диапазон включительно. Множество спецификаций можно передать в `Scan` вариадиком — они объединяются, дедуплицируются и сортируются.

### Запуск сканирования

```go
func (s *Scanner) Scan(ctx context.Context, hosts []string, portSpecs ...PortSpec) (<-chan Result, error)
```

Возвращает канал результатов. Читается через `for r := range results`. Канал закрывается, когда все результаты отправлены или контекст отменён.

Синхронные ошибки (до запуска):
- `ErrNoTargets` — пустой список хостов;
- `ErrNoPorts` — не задано портов;
- `ErrEmptyHost` — в списке есть пустой хост `""`.

### Типы

```go
type State int // StateOpen, StateClosed, StateTimeout, StateUnreachable, StateCanceled, StateError

type Result struct {
    Host     string
    IP       net.IP
    Port     uint16
    State    State
    Duration time.Duration
    Err      error
}

type Resolver interface {
    LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}
```

`Resolver` совместим с `*net.Resolver` — `net.DefaultResolver` автоматически удовлетворяет интерфейсу.

### Состояния порта

| State | Когда |
|-------|-------|
| `StateOpen` | TCP-соединение установлено |
| `StateClosed` | соединение явно отклонено (ECONNREFUSED) |
| `StateTimeout` | подключение не завершилось за установленное время |
| `StateUnreachable` | узел или сеть недоступны (EHOSTUNREACH/ENETUNREACH) |
| `StateCanceled` | проверка отменена через context.Context |
| `StateError` | другая ошибка (например, DNS) |

### Поведение в краевых случаях

| Ситуация | Поведение |
|----------|-----------|
| Порт 0 | `*InvalidPortError` |
| Порт > 65535 | Невозможно (тип `uint16`) |
| Обратный диапазон (`Range(1000, 1)`) | `*InvalidRangeError` |
| Дубликаты портов | Дедупликация в `Scan` |
| Пустой список портов | `ErrNoPorts` |
| Пустой список хостов | `ErrNoTargets` |
| Пустой хост `""` | `ErrEmptyHost` (fail-fast) |
| Неразрешимый DNS | `Result{State: StateError}` на каждый порт |
| Один DNS → несколько IP | Отдельный `Result` на каждый IP |

## Зависимости

- `github.com/stretchr/testify` — используется только в `*_test.go` для утверждений (`assert`, `require`). Обоснование: делает табличные тесты лаконичнее, чем ручное сравнение через `reflect.DeepEqual`. На production-код не влияет.

## Тестирование

```bash
go test -race ./...
```

## Ограничения

- Только TCP Connect scan. SYN, UDP, raw sockets не реализованы (по ТЗ не требуются).
- Определение ОС, обход фаерволов, эксплуатация уязвимостей — не реализованы (по ТЗ не требуются).
- Поддержка CIDR — не реализована (опционально по ТЗ).
- Классификация состояний через `syscall.E*` тестировалась на Linux/macOS. На Windows коды ошибок могут отличаться — корректная классификация не гарантируется.
