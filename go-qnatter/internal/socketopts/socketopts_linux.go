//go:build linux

package socketopts

import "syscall"

const soBindToDevice = syscall.SO_BINDTODEVICE

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
					if !options.DisableReusePort {
						if err := setters.Int(fd, syscall.SOL_SOCKET, soReusePort, 1); err != nil {
							controlErr = err
							return
						}
					}
				}
			}
			if options.Interface != "" {
				if setters.String == nil {
					controlErr = errMissingSetter
					return
				}
				controlErr = setters.String(fd, syscall.SOL_SOCKET, soBindToDevice, options.Interface)
			}
		})
		if err != nil {
			return err
		}
		return controlErr
	}
}
