package endpoint

import (
	"fmt"
	"strconv"
	"strings"
)

func SplitHostPortDefault(value string, defaultPort int) (string, int, error) {
	if value == "" {
		return "", 0, fmt.Errorf("empty host")
	}

	host := value
	port := defaultPort
	if strings.HasPrefix(value, "[") {
		end := strings.LastIndex(value, "]")
		if end < 0 {
			return "", 0, fmt.Errorf("empty host")
		}
		host = value[1:end]
		if rest := value[end+1:]; rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return "", 0, fmt.Errorf("invalid port in %q", value)
			}
			parsed, err := ParsePort(rest[1:])
			if err != nil {
				return "", 0, fmt.Errorf("invalid port in %q", value)
			}
			port = parsed
		}
	} else if strings.Count(value, ":") == 1 {
		idx := strings.LastIndex(value, ":")
		host = value[:idx]
		parsed, err := ParsePort(value[idx+1:])
		if err != nil {
			return "", 0, fmt.Errorf("invalid port in %q", value)
		}
		port = parsed
	}

	if host == "" {
		return "", 0, fmt.Errorf("empty host")
	}
	return host, port, nil
}

func ParsePort(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("invalid port")
	}
	return parsed, nil
}
