package notify

import (
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"

	"qnatter-openwrt/go-qnatter/internal/status"
)

type Options struct {
	Instance   string
	StatusFile string
	RunDir     string
	UserScript string
}

type Result struct {
	UserNotifyError string
}

func Run(options Options, mapping status.Mapping) (Result, error) {
	if mapping.Instance == "" {
		mapping.Instance = defaultString(options.Instance, "default")
	}
	if mapping.Message == "" {
		mapping.Message = "mapped"
	}

	statusFile := options.StatusFile
	if statusFile == "" {
		runDir := defaultString(options.RunDir, "/var/run/qnatter")
		statusFile = filepath.Join(runDir, status.RuntimeSlug(mapping.Instance)+".json")
	}
	if err := status.WriteMapping(statusFile, mapping); err != nil {
		return Result{}, err
	}

	if options.UserScript == "" {
		return Result{}, nil
	}

	if err := requireExecutable(options.UserScript); err != nil {
		return Result{UserNotifyError: err.Error()}, nil
	}

	args := notifyArgs(mapping.Protocol, mapping.Inner, mapping.Outer)
	cmd := exec.Command(options.UserScript, args...)
	if err := cmd.Run(); err != nil {
		return Result{UserNotifyError: fmt.Sprintf("notify script failed: %v", err)}, nil
	}

	return Result{}, nil
}

func requireExecutable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("notify script is not executable: %s", path)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("notify script is not executable: %s", path)
	}
	return nil
}

func notifyArgs(protocol string, inner netip.AddrPort, outer netip.AddrPort) []string {
	return []string{
		protocol,
		inner.Addr().String(),
		fmt.Sprintf("%d", inner.Port()),
		outer.Addr().String(),
		fmt.Sprintf("%d", outer.Port()),
	}
}

func defaultString(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
