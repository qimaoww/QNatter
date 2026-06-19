//go:build linux

package procname

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

const prSetName = 15

func Set(name string) error {
	encoded := commName(name)
	if err := os.WriteFile("/proc/self/comm", append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	_, _, _ = syscall.RawSyscall6(syscall.SYS_PRCTL, prSetName, uintptr(unsafe.Pointer(&encoded[0])), 0, 0, 0, 0)
	return nil
}

func commName(name string) []byte {
	encoded := []byte(name)
	if idx := bytes.IndexByte(encoded, '\n'); idx >= 0 {
		encoded = encoded[:idx]
	}
	if len(encoded) == 0 {
		encoded = []byte("Natter")
	}
	if len(encoded) > 15 {
		encoded = encoded[:15]
	}
	return encoded
}
