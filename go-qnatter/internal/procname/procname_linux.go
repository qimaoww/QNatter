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
	if err := os.WriteFile("/proc/self/comm", encoded, 0o644); err != nil {
		return err
	}
	prctlName := make([]byte, 16)
	copy(prctlName, encoded)
	if _, _, errno := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetName, uintptr(unsafe.Pointer(&prctlName[0])), 0, 0, 0, 0); errno != 0 {
		return errno
	}
	return nil
}

func commName(name string) []byte {
	encoded := []byte(name)
	if idx := bytes.IndexByte(encoded, '\n'); idx >= 0 {
		encoded = encoded[:idx]
	}
	if len(encoded) == 0 {
		encoded = []byte("QNatter")
	}
	if len(encoded) > 15 {
		encoded = encoded[:15]
	}
	return encoded
}
