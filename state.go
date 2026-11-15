package portscan

// State описывает результат проверки одного порта.
type State int

const (
	// StateOpen - TCP-соединение установлено.
	StateOpen State = iota
	// StateClosed - соединение явно отклонено (RST/ECONNREFUSED).
	StateClosed
	// StateTimeout - подключение не завершилось за установленное время.
	StateTimeout
	// StateUnreachable - узел или сеть недоступны.
	StateUnreachable
	// StateCanceled - проверка отменена через context.Context.
	StateCanceled
	// StateError - произошла другая ошибка.
	StateError
)

// String возвращает короткие имена состояний портов
func (s State) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateClosed:
		return "closed"
	case StateTimeout:
		return "timeout"
	case StateUnreachable:
		return "unreachable"
	case StateCanceled:
		return "canceled"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}
