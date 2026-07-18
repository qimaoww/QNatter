package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"

	"qnatter-openwrt/go-qnatter/internal/check"
	"qnatter-openwrt/go-qnatter/internal/config"
	"qnatter-openwrt/go-qnatter/internal/engine"
	"qnatter-openwrt/go-qnatter/internal/forward"
	"qnatter-openwrt/go-qnatter/internal/notify"
	"qnatter-openwrt/go-qnatter/internal/portcheck"
	"qnatter-openwrt/go-qnatter/internal/status"
	"qnatter-openwrt/go-qnatter/internal/stun"
)

const Version = "2.2.2-go"

const helpText = `usage: qnatter [options]

Expose your port behind full-cone NAT to the Internet.

options:
  --version, -V       show the version of QNatter and exit
  --help              show this help message and exit
  --check             run qnatter-check and exit
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
			fmt.Fprintf(stdout, "QNatter Go %s\n", Version)
			return 0
		}
		if arg == "--help" {
			fmt.Fprint(stdout, helpText)
			return 0
		}
	}

	cfg, err := config.ParseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "qnatter: %v\n", err)
		return 2
	}

	if cfg.Check {
		if err := checkRun(ctx, cfg, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "qnatter: %v\n", err)
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
			logExitReason(stderr, err)
			return 0
		}
		logLine(stderr, "E", "qnatter: %v", err)
		return 1
	}
	return 0
}

func logExitReason(stderr io.Writer, err error) {
	switch {
	case errors.Is(err, engine.ErrMappingChanged):
		logLine(stderr, "I", "QNatter is exiting because mapped address has changed")
	case errors.Is(err, engine.ErrLocalAddressChanged):
		logLine(stderr, "I", "QNatter is exiting because local IP address has changed")
	}
}

func printStartup(stderr io.Writer, showTip bool) {
	logLine(stderr, "I", "QNatter v%s", Version)
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
	bind, err := engine.BindFromConfig(cfg)
	if err != nil {
		return err
	}
	lanPortChecker := portcheck.Checker{}
	wanPortChecker := portcheck.Checker{Interface: bind.Interface}
	return engine.RunWithRetry(ctx, cfg, func(ctx context.Context) error {
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
				logNotifyScript(log, cfg.NotifyPath)
				result, err := notify.Run(notify.Options{
					Instance:   os.Getenv("QNATTER_INSTANCE"),
					StatusFile: os.Getenv("QNATTER_STATUS_FILE"),
					UserScript: cfg.NotifyPath,
				}, mapping)
				logNotifyResult(log, result)
				return err
			},
			OnMapped: func(result engine.Result) {
				logMapping(log, cfg, result)
			},
			OnUPnPError: func(operation string, err error) {
				logUPnPError(log, operation, err)
			},
			PortCheck:    lanPortChecker,
			InitialCheck: splitPortChecker{lan: lanPortChecker, wan: wanPortChecker},
		}
		if cfg.UPnP {
			logUPnPScanning(log)
			upnpMapper, err := engine.NewUPnPMapperFromConfig(cfg)
			if err != nil {
				return err
			}
			if client, ok := upnpMapper.(*engine.UPnPClient); ok {
				client.OnFoundRouter = func(ip string) {
					logUPnPFoundRouter(log, ip)
				}
			}
			deps.UPnP = upnpMapper
		}
		return engine.RunLoop(ctx, cfg, deps, engine.LoopOptions{})
	}, engine.RetryOptions{
		OnRetry: func(err error, delay time.Duration) {
			logRetry(log, err, delay)
		},
	})
}

func logRetry(w io.Writer, err error, delay time.Duration) {
	switch {
	case errors.Is(err, syscall.ENODEV):
		logLine(w, "W", "bind interface is unavailable; retrying in %s", delay)
	case errors.Is(err, syscall.EADDRNOTAVAIL):
		logLine(w, "W", "local address is unavailable; retrying in %s", delay)
	default:
		logLine(w, "W", "%v; retrying in %s", err, delay)
	}
}

func logNotifyScript(w io.Writer, path string) {
	if path == "" {
		return
	}
	logLine(w, "I", "Calling script: %s", path)
}

func logNotifyResult(w io.Writer, result notify.Result) {
	if result.UserNotifyError == "" {
		return
	}
	logLine(w, "W", "%s", result.UserNotifyError)
}

func logUPnPError(w io.Writer, operation string, err error) {
	if err == nil {
		return
	}
	logLine(w, "E", "upnp: failed to %s: %v", operation, err)
}

func logUPnPFoundRouter(w io.Writer, ip string) {
	if ip == "" {
		return
	}
	logLine(w, "I", "[UPnP] Found router %s", ip)
}

func logUPnPScanning(w io.Writer) {
	logLine(w, "I", "Scanning UPnP Devices...")
}

func logMapping(w io.Writer, cfg config.Config, result engine.Result) {
	protocol := "tcp"
	if cfg.UDP {
		protocol = "udp"
	}

	if result.Unstable {
		logLine(w, "W", "Network is unstable, or not full cone")
	}

	route := ""
	if result.Method != "none" && result.Method != "test" {
		route += fmt.Sprintf("%s://%s <--%s--> ", protocol, result.Target, result.Method)
	}
	route += fmt.Sprintf("%s://%s <--QNatter--> %s://%s", protocol, result.Mapping.Inner, protocol, result.Mapping.Outer)
	logLine(w, "I", "%s", route)

	if result.Method == "test" {
		scheme := "http"
		if cfg.UDP {
			scheme = "udp"
		}
		logLine(w, "I", "Test mode is on.")
		logLine(w, "I", "Please check [ %s://%s ]", scheme, result.Mapping.Outer)
	}

	if result.Ports.Checked {
		switch {
		case result.Ports.TargetLAN == engine.PortClosed:
			logLine(w, "W", "!! Target port is closed !!")
		case result.Ports.TargetLAN == engine.PortOpen && result.Ports.OuterLAN == engine.PortClosed && result.Ports.OuterWAN == engine.PortClosed:
			logLine(w, "W", "!! Hole punching failed !!")
		case result.Ports.OuterLAN == engine.PortOpen && result.Ports.OuterWAN == engine.PortClosed:
			logLine(w, "W", "!! You may be behind a firewall !!")
		}
	}
}
