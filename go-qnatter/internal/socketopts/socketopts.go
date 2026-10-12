package socketopts

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
)

// errMissingSetter 表示调用方请求了某个选项却没有提供对应的 setter：
// 静默跳过会让“设置了选项”与“实际生效”不一致。
var errMissingSetter = errors.New("socketopts: missing setter for requested option")

type Options struct {
	Interface string
	// Reuse 同时设置 SO_REUSEADDR 与 SO_REUSEPORT。它是 keep-alive/STUN
	// 套接字与转发监听套接字共用同一内网端口所必需的；副作用是同一
	// addr:port 上的第二个实例不会收到 EADDRINUSE，而会被内核负载均衡
	// （表现为“时好时坏”），需要严格独占时请设置 DisableReusePort。
	Reuse bool
	// DisableReusePort 只关闭 SO_REUSEPORT，保留 SO_REUSEADDR。
	DisableReusePort bool
}

type Setters struct {
	Int    func(fd uintptr, level int, opt int, value int) error
	String func(fd uintptr, level int, opt int, value string) error
}

func LocalAddr(network string, source netip.AddrPort) (net.Addr, error) {
	if !source.IsValid() {
		return nil, nil
	}
	ip := net.IP(source.Addr().AsSlice())
	port := int(source.Port())
	switch network {
	case "tcp", "tcp4":
		return &net.TCPAddr{IP: ip, Port: port}, nil
	case "udp", "udp4":
		return &net.UDPAddr{IP: ip, Port: port}, nil
	default:
		return nil, fmt.Errorf("unsupported network %q", network)
	}
}

func NetworkForSource(network string, source netip.AddrPort) string {
	if !source.IsValid() || !source.Addr().Is4() {
		return network
	}
	switch network {
	case "tcp":
		return "tcp4"
	case "udp":
		return "udp4"
	default:
		return network
	}
}
