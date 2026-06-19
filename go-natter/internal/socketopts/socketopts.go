package socketopts

import (
	"fmt"
	"net"
	"net/netip"
	"syscall"
)

const (
	soBindToDevice = syscall.SO_BINDTODEVICE
	soReusePort    = 0x0f
)

type Options struct {
	Interface string
	Reuse     bool
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

func Control(options Options) func(network string, address string, conn syscall.RawConn) error {
	return ControlWith(options, Setters{
		Int: func(fd uintptr, level int, opt int, value int) error {
			return syscall.SetsockoptInt(int(fd), level, opt, value)
		},
		String: func(fd uintptr, level int, opt int, value string) error {
			return syscall.SetsockoptString(int(fd), level, opt, value)
		},
	})
}

func ControlWith(options Options, setters Setters) func(network string, address string, conn syscall.RawConn) error {
	return func(network string, address string, conn syscall.RawConn) error {
		var controlErr error
		err := conn.Control(func(fd uintptr) {
			if options.Reuse {
				if setters.Int != nil {
					if err := setters.Int(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
						controlErr = err
						return
					}
					if err := setters.Int(fd, syscall.SOL_SOCKET, soReusePort, 1); err != nil {
						controlErr = err
						return
					}
				}
			}
			if options.Interface != "" && setters.String != nil {
				controlErr = setters.String(fd, syscall.SOL_SOCKET, soBindToDevice, options.Interface)
			}
		})
		if err != nil {
			return err
		}
		return controlErr
	}
}
