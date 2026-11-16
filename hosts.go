package portscan

func normalizeHosts(hosts []string) ([]string, error) {
	if len(hosts) == 0 {
		return nil, ErrNoTargets
	}

	seen := make(map[string]struct{}, len(hosts))
	uniqueHosts := make([]string, 0, len(seen))

	for _, host := range hosts {
		if host == "" {
			return nil, ErrEmptyHost
		}

		if _, ok := seen[host]; !ok {
			uniqueHosts = append(uniqueHosts, host)
			seen[host] = struct{}{}
		}
	}

	return uniqueHosts, nil
}
