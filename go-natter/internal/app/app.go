package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"natter-openwrt/go-natter/internal/check"
	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/engine"
	"natter-openwrt/go-natter/internal/forward"
	"natter-openwrt/go-natter/internal/notify"
	"natter-openwrt/go-natter/internal/status"
	"natter-openwrt/go-natter/internal/stun"
)

const Version = "2.2.1-go"

const helpText = `usage: natter [options]

Expose your port behind full-cone NAT to the Internet.

options:
  --version, -V       show the version of Natter and exit
  --help              show this help message and exit
  --check             run natter-check and exit
  -v                  verbose mode, printing debug messages
  -q                  exit when mapped address is changed
  -u                  UDP mode
  -U                  enable UPnP/IGD discovery
  -k <interval>       seconds between each keep-alive
  -s <address>        hostname or address to STUN server
  -h <address>        hostname or address to keep-alive server
  -e <path>           script path for notifying mapped address

bind options:
  -i <interface>      network interface name or IP to bind
  -b <port>           port number to bind

forward options:
  -m <method>         forward method, common values are 'nftables', 'socat', 'gost' and 'socket'
  -t <address>        IP address of forward target
  -p <port>           port number of forward target
  -r                  keep retrying until the port of forward target is open
`

type EngineRunner func(context.Context, config.Config) error
type CheckRunner func(context.Context, config.Config, io.Writer, io.Writer) error

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	return RunContext(context.Background(), args, stdout, stderr)
}

func RunContext(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return runWithContext(ctx, args, stdout, stderr, func(ctx context.Context, cfg config.Config) error {
		return runEngineWithLog(ctx, cfg, stderr)
	}, check.Run)
}

func RunWith(args []string, stdout io.Writer, stderr io.Writer, run func(config.Config) error) int {
	return RunWithContext(context.Background(), args, stdout, stderr, func(ctx context.Context, cfg config.Config) error {
		return run(cfg)
	})
}

func RunWithContext(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, run EngineRunner) int {
	return runWithContext(ctx, args, stdout, stderr, run, check.Run)
}

func runWithContext(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, run EngineRunner, checkRun CheckRunner) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-V" {
			fmt.Fprintf(stdout, "Natter Go %s\n", Version)
			return 0
		}
		if arg == "--help" {
			fmt.Fprint(stdout, helpText)
			return 0
		}
	}

	cfg, err := config.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "natter: %v\n", err)
		return 2
	}

	if cfg.Check {
		if err := checkRun(ctx, cfg, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "natter: %v\n", err)
			return 1
		}
		return 0
	}

	printStartup(stderr, len(args) == 0)
	if cfg.Verbose {
		logConfig(stderr, cfg)
	}
	if err := run(ctx, cfg); err != nil {
		if cfg.ExitWhenChanged && (errors.Is(err, engine.ErrMappingChanged) || errors.Is(err, engine.ErrLocalAddressChanged)) {
			return 0
		}
		logLine(stderr, "E", "natter: %v", err)
		return 1
	}
	return 0
}

func printStartup(stderr io.Writer, showTip bool) {
	logLine(stderr, "I", "Natter v%s", Version)
	if showTip {
		logLine(stderr, "I", "Tips: Use `--help` to see help messages")
	}
}

func logLine(w io.Writer, level string, format string, args ...any) {
	fmt.Fprintf(w, "%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, fmt.Sprintf(format, args...))
}

func logConfig(w io.Writer, cfg config.Config) {
	protocol := "tcp"
	if cfg.UDP {
		protocol = "udp"
	}
	method := cfg.ForwardMethod
	if method == "" {
		method = "auto"
	}
	logLine(w, "D", "config: protocol=%s bind=%s:%d method=%s target=%s:%d", protocol, cfg.BindValue, cfg.BindPort, method, cfg.TargetIP, cfg.TargetPort)
}

func runEngine(ctx context.Context, cfg config.Config) error {
	return runEngineWithLog(ctx, cfg, io.Discard)
}

func runEngineWithLog(ctx context.Context, cfg config.Config, log io.Writer) error {
	stunClient, err := engine.NewSTUNClientFromConfig(cfg)
	if err != nil {
		return err
	}
	deps := engine.Dependencies{
		STUN: stunClient,
		NewKeepAlive: func(mapping stun.Mapping) (engine.KeepAlive, error) {
			return engine.NewKeepAliveFromConfig(cfg, mapping)
		},
		NewForwarder: forward.NewForwarder,
		Notify: func(mapping status.Mapping) error {
			_, err := notify.Run(notify.Options{
				Instance:   os.Getenv("NATTER_INSTANCE"),
				StatusFile: os.Getenv("NATTER_STATUS_FILE"),
				UserScript: cfg.NotifyPath,
			}, mapping)
			return err
		},
	}
	if cfg.UPnP {
		upnpMapper, err := engine.NewUPnPMapperFromConfig(cfg)
		if err != nil {
			return err
		}
		deps.UPnP = upnpMapper
	}
	return engine.RunWithRetry(ctx, cfg, func(ctx context.Context) error {
		return engine.RunLoop(ctx, cfg, deps, engine.LoopOptions{})
	}, engine.RetryOptions{
		OnRetry: func(err error, delay time.Duration) {
			logLine(log, "W", "%v; retrying in %s", err, delay)
		},
	})
}
