package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/engine"
	"natter-openwrt/go-natter/internal/stun"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &bytes.Buffer{})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	if got := stdout.String(); !strings.Contains(got, "Natter Go") {
		t.Fatalf("version output = %q, want Natter Go", got)
	}
}

func TestRunHelpPrintsUsageWithoutStartingEngine(t *testing.T) {
	var stdout, stderr bytes.Buffer
	engineCalled := false

	code := RunWithContext(context.Background(), []string{"--help"}, &stdout, &stderr, func(context.Context, config.Config) error {
		engineCalled = true
		return nil
	})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	if engineCalled {
		t.Fatal("--help started the mapping engine")
	}
	out := stdout.String()
	for _, want := range []string{
		"Expose your port behind full-cone NAT to the Internet.",
		"--check",
		"-h <address>",
		"hostname or address to keep-alive server",
		"-m <method>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output = %q, missing %q", out, want)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
}

func TestRunCheckParsesExistingOpenWrtArguments(t *testing.T) {
	var stdout bytes.Buffer
	notifyPath := filepath.Join(t.TempDir(), "cmcc.notify")
	if err := os.WriteFile(notifyPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var got config.Config
	engineCalled := false
	code := runWithContext(context.Background(), []string{
		"--check",
		"-k", "15",
		"-s", "turn.cloud-rtc.com:80",
		"-i", "pppoe-wan_cmcc",
		"-b", "0",
		"-m", "none",
		"-t", "0.0.0.0",
		"-p", "0",
		"-e", notifyPath,
	}, &stdout, &bytes.Buffer{}, func(context.Context, config.Config) error {
		engineCalled = true
		return nil
	}, func(_ context.Context, cfg config.Config, out io.Writer, _ io.Writer) error {
		got = cfg
		fmt.Fprintln(out, "> NatterCheck Go")
		return nil
	})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	if engineCalled {
		t.Fatal("--check started the mapping engine")
	}
	if got.BindValue != "pppoe-wan_cmcc" || got.BindPort != 0 || got.ForwardMethod != "none" {
		t.Fatalf("checker config = %+v", got)
	}
	if len(got.STUNServers) != 1 || got.STUNServers[0].Host != "turn.cloud-rtc.com" || got.STUNServers[0].Port != 80 {
		t.Fatalf("checker STUN servers = %+v", got.STUNServers)
	}
	if !strings.Contains(stdout.String(), "NatterCheck Go") {
		t.Fatalf("check output = %q, want checker output", stdout.String())
	}
}

func TestRunCheckPrintsReportWithoutFakeSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWithContext(context.Background(), []string{"--check"}, &stdout, &stderr,
		func(context.Context, config.Config) error {
			t.Fatal("--check started the mapping engine")
			return nil
		},
		func(_ context.Context, _ config.Config, out io.Writer, _ io.Writer) error {
			fmt.Fprintln(out, "> NatterCheck v2.2.1-go")
			fmt.Fprintln(out, "Checking TCP NAT...                  [   OK   ] ... NAT Type: 1")
			return nil
		})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	if strings.Contains(stdout.String(), "check: ok") {
		t.Fatalf("stdout = %q, must not report fake check success", stdout.String())
	}
	if !strings.Contains(stdout.String(), "> NatterCheck v2.2.1-go") {
		t.Fatalf("stdout = %q, want NatterCheck report", stdout.String())
	}
	if !strings.Contains(stdout.String(), "[   OK   ] ... NAT Type: 1") {
		t.Fatalf("stdout = %q, want TCP NAT result line", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	code := Run([]string{"-s", "bad:port"}, &bytes.Buffer{}, &bytes.Buffer{})

	if code == 0 {
		t.Fatal("Run accepted invalid arguments")
	}
}

func TestRunWithStartsEngineForMapping(t *testing.T) {
	var got config.Config
	var stderr bytes.Buffer
	code := RunWith([]string{
		"-i", "pppoe-wan_cmcc",
		"-m", "none",
	}, &bytes.Buffer{}, &stderr, func(cfg config.Config) error {
		got = cfg
		return nil
	})

	if code != 0 {
		t.Fatalf("RunWith returned code %d, want 0", code)
	}
	if got.BindValue != "pppoe-wan_cmcc" || got.ForwardMethod != "none" {
		t.Fatalf("runner config = %+v", got)
	}
	if !strings.Contains(stderr.String(), "Natter v2.2.1-go") {
		t.Fatalf("stderr = %q, want startup banner", stderr.String())
	}
}

func TestRunWithNoArgsPrintsHelpTip(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWith(nil, &bytes.Buffer{}, &stderr, func(config.Config) error {
		return nil
	})

	if code != 0 {
		t.Fatalf("RunWith returned code %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "Tips: Use `--help` to see help messages") {
		t.Fatalf("stderr = %q, want help tip", stderr.String())
	}
}

func TestRunWithReportsEngineError(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWith(nil, &bytes.Buffer{}, &stderr, func(config.Config) error {
		return errors.New("mapping failed")
	})

	if code != 1 {
		t.Fatalf("RunWith returned code %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "mapping failed") {
		t.Fatalf("stderr = %q, want mapping failed", stderr.String())
	}
}

func TestRunLogsUseTimestampedLevels(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWith([]string{"-m", "none"}, &bytes.Buffer{}, &stderr, func(config.Config) error {
		return errors.New("mapping failed")
	})
	if code != 1 {
		t.Fatalf("RunWith returned code %d, want 1", code)
	}

	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("stderr = %q, want startup and error log lines", stderr.String())
	}
	infoLine := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[I\] Natter v`)
	errorLine := regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[E\] natter: mapping failed$`)
	if !infoLine.MatchString(lines[0]) {
		t.Fatalf("startup line = %q, want timestamped info log", lines[0])
	}
	if !errorLine.MatchString(lines[len(lines)-1]) {
		t.Fatalf("error line = %q, want timestamped error log", lines[len(lines)-1])
	}
}

func TestRunVerboseLogsRuntimeConfig(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWith([]string{"-v", "-u", "-i", "pppoe-wan_cmcc", "-b", "40000", "-m", "none"}, &bytes.Buffer{}, &stderr, func(config.Config) error {
		return nil
	})
	if code != 0 {
		t.Fatalf("RunWith returned code %d, want 0", code)
	}
	for _, want := range []string{
		"[D] config:",
		"protocol=udp",
		"bind=pppoe-wan_cmcc:40000",
		"method=none",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, missing %q", stderr.String(), want)
		}
	}
}

func TestLogMappingPrintsRouteInformation(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{}, engine.Result{
		Method: "socket",
		Target: mustAddrPort("10.10.10.10:51413"),
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.10.10.2:40000"),
			Outer: mustAddrPort("203.0.113.10:62000"),
		},
	})

	for _, want := range []string{
		"[I] tcp://10.10.10.10:51413 <--socket--> tcp://10.10.10.2:40000 <--Natter--> tcp://203.0.113.10:62000",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, missing %q", stderr.String(), want)
		}
	}
}

func TestLogMappingOmitsForwardSegmentForNoneMethod(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{UDP: true}, engine.Result{
		Method: "none",
		Target: mustAddrPort("10.0.0.2:50000"),
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.0.0.2:50000"),
			Outer: mustAddrPort("198.51.100.8:62001"),
		},
	})

	want := "[I] udp://10.0.0.2:50000 <--Natter--> udp://198.51.100.8:62001"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, missing %q", stderr.String(), want)
	}
	if strings.Contains(stderr.String(), "<--none-->") {
		t.Fatalf("stderr = %q, must not include none as a forward segment", stderr.String())
	}
}

func TestLogMappingPrintsTestModeNotice(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{}, engine.Result{
		Method: "test",
		Target: mustAddrPort("10.10.10.2:40000"),
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.10.10.2:40000"),
			Outer: mustAddrPort("203.0.113.10:62000"),
		},
	})

	for _, want := range []string{
		"[I] Test mode is on.",
		"[I] Please check [ http://203.0.113.10:62000 ]",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, missing %q", stderr.String(), want)
		}
	}
}

func TestLogMappingPrintsUDPTestModeNotice(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{UDP: true}, engine.Result{
		Method: "test",
		Target: mustAddrPort("10.0.0.2:50000"),
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.0.0.2:50000"),
			Outer: mustAddrPort("198.51.100.8:62001"),
		},
	})

	want := "[I] Please check [ udp://198.51.100.8:62001 ]"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, missing %q", stderr.String(), want)
	}
}

func TestLogMappingWarnsWhenMappingIsUnstable(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{}, engine.Result{
		Method:   "none",
		Target:   mustAddrPort("10.10.10.2:40000"),
		Unstable: true,
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.10.10.2:40000"),
			Outer: mustAddrPort("203.0.113.10:62000"),
		},
	})

	if !strings.Contains(stderr.String(), "[W] Network is unstable, or not full cone") {
		t.Fatalf("stderr = %q, missing unstable network warning", stderr.String())
	}
}

func TestLogMappingWarnsWhenTargetPortIsClosed(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{}, engine.Result{
		Method: "socket",
		Target: mustAddrPort("10.10.10.10:51413"),
		Ports:  engine.PortReport{Checked: true, TargetLAN: engine.PortClosed},
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.10.10.2:40000"),
			Outer: mustAddrPort("203.0.113.10:62000"),
		},
	})

	if !strings.Contains(stderr.String(), "[W] !! Target port is closed !!") {
		t.Fatalf("stderr = %q, missing target port warning", stderr.String())
	}
}

func TestLogMappingWarnsWhenHolePunchingFails(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{}, engine.Result{
		Method: "socket",
		Target: mustAddrPort("10.10.10.10:51413"),
		Ports: engine.PortReport{
			Checked:   true,
			TargetLAN: engine.PortOpen,
			OuterLAN:  engine.PortClosed,
			OuterWAN:  engine.PortClosed,
		},
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.10.10.2:40000"),
			Outer: mustAddrPort("203.0.113.10:62000"),
		},
	})

	if !strings.Contains(stderr.String(), "[W] !! Hole punching failed !!") {
		t.Fatalf("stderr = %q, missing hole punching warning", stderr.String())
	}
}

func TestLogMappingWarnsWhenBehindFirewall(t *testing.T) {
	var stderr bytes.Buffer
	logMapping(&stderr, config.Config{}, engine.Result{
		Method: "socket",
		Target: mustAddrPort("10.10.10.10:51413"),
		Ports: engine.PortReport{
			Checked:  true,
			OuterLAN: engine.PortOpen,
			OuterWAN: engine.PortClosed,
		},
		Mapping: stun.Mapping{
			Inner: mustAddrPort("10.10.10.2:40000"),
			Outer: mustAddrPort("203.0.113.10:62000"),
		},
	})

	if !strings.Contains(stderr.String(), "[W] !! You may be behind a firewall !!") {
		t.Fatalf("stderr = %q, missing firewall warning", stderr.String())
	}
}

func TestRunWithContextPassesContextToEngine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var gotCanceled bool
	code := RunWithContext(ctx, []string{"-m", "none"}, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, cfg config.Config) error {
		gotCanceled = ctx.Err() != nil
		return nil
	})

	if code != 0 {
		t.Fatalf("RunWithContext returned code %d, want 0", code)
	}
	if !gotCanceled {
		t.Fatal("runner did not receive canceled context")
	}
}

func TestRunWithContextTreatsExitWhenChangedAsCleanExit(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWithContext(context.Background(), []string{"-q"}, &bytes.Buffer{}, &stderr, func(ctx context.Context, cfg config.Config) error {
		if !cfg.ExitWhenChanged {
			t.Fatal("ExitWhenChanged = false, want true")
		}
		return engine.ErrMappingChanged
	})

	if code != 0 {
		t.Fatalf("RunWithContext returned code %d, want clean exit", code)
	}
	if !strings.Contains(stderr.String(), "[I] Natter is exiting because mapped address has changed") {
		t.Fatalf("stderr = %q, missing mapped-address exit reason", stderr.String())
	}
}

func TestRunWithContextTreatsLocalAddressChangeAsCleanExitWhenRequested(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWithContext(context.Background(), []string{"-q"}, &bytes.Buffer{}, &stderr, func(ctx context.Context, cfg config.Config) error {
		if !cfg.ExitWhenChanged {
			t.Fatal("ExitWhenChanged = false, want true")
		}
		return engine.ErrLocalAddressChanged
	})

	if code != 0 {
		t.Fatalf("RunWithContext returned code %d, want clean exit", code)
	}
	if !strings.Contains(stderr.String(), "[I] Natter is exiting because local IP address has changed") {
		t.Fatalf("stderr = %q, missing local-address exit reason", stderr.String())
	}
}

func mustAddrPort(value string) netip.AddrPort {
	return netip.MustParseAddrPort(value)
}
