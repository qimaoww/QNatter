package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"qnatter-openwrt/go-qnatter/internal/app"
	"qnatter-openwrt/go-qnatter/internal/procname"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	_ = procname.Set("QNatter")
	return app.RunContext(ctx, args, stdout, stderr)
}
