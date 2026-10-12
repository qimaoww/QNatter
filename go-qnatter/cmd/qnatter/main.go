package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"qnatter-openwrt/go-qnatter/internal/app"
	"qnatter-openwrt/go-qnatter/internal/procname"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	// os.Exit 不执行 defer，defer stop() 永远不会跑（N-P2-13）：必须在这里
	// 显式注销信号处理再退出。
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if err := procname.Set("QNatter"); err != nil {
		// 进程名只影响 ps/top 的可观测性，失败不应让程序无法启动，但也不能
		// 静默丢弃：否则运维在进程列表里看到的是 Go 默认名，会误判。
		fmt.Fprintf(stderr, "qnatter: failed to set process name: %v\n", err)
	}
	return app.RunContext(ctx, args, stdout, stderr)
}
