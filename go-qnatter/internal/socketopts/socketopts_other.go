//go:build !linux

package socketopts

import (
	"errors"
	"syscall"
)

// errUnsupported 表示当前平台不支持 SO_BINDTODEVICE / SO_REUSEPORT。
var errUnsupported = errors.New("socket options are only supported on linux")

func Control(options Options) func(network string, address string, conn syscall.RawConn) error {
	return ControlWith(options, Setters{})
}

func ControlWith(options Options, _ Setters) func(network string, address string, conn syscall.RawConn) error {
	return func(network string, address string, conn syscall.RawConn) error {
		if options.Interface == "" && !options.Reuse {
			return nil
		}
		return errUnsupported
	}
}
