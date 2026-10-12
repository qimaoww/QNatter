package notify

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"qnatter-openwrt/go-qnatter/internal/status"
)

// defaultUserScriptTimeout 是用户通知脚本的默认运行上限：脚本挂死不应
// 阻塞 StartSession（否则 procd 会在 term_timeout 后 SIGKILL，规则清理
// 被中断）。
const defaultUserScriptTimeout = 30 * time.Second

type Options struct {
	Instance   string
	StatusFile string
	RunDir     string
	UserScript string
	// Timeout 限制用户脚本的运行时间；<=0 时使用 defaultUserScriptTimeout。
	Timeout time.Duration
	// Context 可选：调用方的上下文，脚本会随它取消。
	Context context.Context
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
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultUserScriptTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, options.UserScript, args...)
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
