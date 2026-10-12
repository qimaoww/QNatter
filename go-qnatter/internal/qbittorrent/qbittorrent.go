// Package qbittorrent 校验 qBittorrent WebUI 的地址与监听端口。
//
// 状态（N-P2-11）：本包在 Go 侧没有任何生产调用方 —— 实际的 qBittorrent
// 联动由 shell 侧 qnatter/files/qnatter-qbittorrent.sh 完成（uci/init 调用
// 它），Go 侧只是保留的历史实现。删除本包会连同 qbittorrent_test.go 一起
// 删除既有测试，因此按"保留 + 注释 + 补齐校验"处理：保留期间不把旧的静默
// 行为留给未来的调用方。
package qbittorrent

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidPort 报告端口是否在 1..65535 内。
func ValidPort(port int) bool {
	return port >= 1 && port <= 65535
}

// SelectListenPort 按 shell 侧同样的优先级选择监听端口。
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

// PreferencesJSON 序列化 WebUI 偏好设置。
//
// 非法端口返回错误，而不是像以前那样静默写成 "listen_port":0：0 在
// qBittorrent 里表示"随机端口"，会把配置错误伪装成一个合法值。
func PreferencesJSON(listenPort int) (string, error) {
	if !ValidPort(listenPort) {
		return "", fmt.Errorf("qbittorrent: invalid listen port %d (want 1..65535)", listenPort)
	}
	return fmt.Sprintf(`{"listen_port":%d}`, listenPort), nil
}

// NormalizeURL 规范化 WebUI 基础地址：只接受 http/https，且必须带主机名。
//
// 修复点：以前对任意 scheme 都放行（file:、gopher: 等都能通过），并且用
// strings.TrimRight(url, "/") 把 "/" 直接规整成空串，调用方随后会拼出
// 无效地址。
func NormalizeURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("qbittorrent: invalid URL %q: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("qbittorrent: unsupported URL scheme %q (want http or https)", parsed.Scheme)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("qbittorrent: URL %q has no host", raw)
	}
	normalized := strings.TrimRight(parsed.String(), "/")
	if normalized == "" || normalized == parsed.Scheme+":" {
		return "", fmt.Errorf("qbittorrent: URL %q has no host", raw)
	}
	return normalized, nil
}
