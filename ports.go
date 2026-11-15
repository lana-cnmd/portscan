package portscan

// PortSpec представляет спецификацию портов для сканирования.
type PortSpec struct {
	ports []uint16
}

// Ports создаёт PortSpec из перечисленных портов.
// Возвращает *InvalidPortError, если хотя бы один порт равен 0.
func Ports(ports ...uint16) (PortSpec, error) {
	validPorts := make([]uint16, 0, len(ports))
	for _, port := range ports {
		if port == 0 {
			return PortSpec{}, &InvalidPortError{Port: port}
		}
		validPorts = append(validPorts, port)
	}
	return PortSpec{ports: validPorts}, nil
}

// Range создаёт PortSpec из диапазона портов от start до end включительно.
// Возвращает *InvalidPortError, если start или end равен 0.
// Возвращает *InvalidRangeError, если start > end.
func Range(start, end uint16) (PortSpec, error) {
	if start == 0 {
		return PortSpec{}, &InvalidPortError{Port: start}
	}

	if end == 0 {
		return PortSpec{}, &InvalidPortError{Port: end}
	}

	if start > end {
		return PortSpec{}, &InvalidRangeError{Start: start, End: end}
	}

	validPorts := make([]uint16, 0, end-start+1)

	for i := start; i < end; i++ {
		validPorts = append(validPorts, i)
	}
	validPorts = append(validPorts, end)

	return PortSpec{ports: validPorts}, nil
}
