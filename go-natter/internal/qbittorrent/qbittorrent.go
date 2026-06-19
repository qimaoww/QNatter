package qbittorrent

import (
	"fmt"
	"strings"
)

func ValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

func SelectListenPort(innerPort int, outerPort int, configuredPort int) int {
	if ValidPort(configuredPort) {
		return configuredPort
	}
	if ValidPort(outerPort) {
		return outerPort
	}
	if ValidPort(innerPort) {
		return innerPort
	}
	return 0
}

func PreferencesJSON(listenPort int) string {
	if !ValidPort(listenPort) {
		listenPort = 0
	}
	return fmt.Sprintf(`{"listen_port":%d}`, listenPort)
}

func NormalizeURL(url string) string {
	return strings.TrimRight(url, "/")
}
