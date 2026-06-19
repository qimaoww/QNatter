package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/engine"
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
			fmt.Fprintln(out, "Checking TCP NAT...                  [  FAIL  ] ... Go TCP NAT check is not implemented yet")
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
	if !strings.Contains(stdout.String(), "[  FAIL  ] ... Go TCP NAT check is not implemented yet") {
		t.Fatalf("stdout = %q, want TCP not implemented line", stdout.String())
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
	code := RunWith([]string{
		"-i", "pppoe-wan_cmcc",
		"-m", "none",
	}, &bytes.Buffer{}, &bytes.Buffer{}, func(cfg config.Config) error {
		got = cfg
		return nil
	})

	if code != 0 {
		t.Fatalf("RunWith returned code %d, want 0", code)
	}
	if got.BindValue != "pppoe-wan_cmcc" || got.ForwardMethod != "none" {
		t.Fatalf("runner config = %+v", got)
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
	code := RunWithContext(context.Background(), []string{"-q"}, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, cfg config.Config) error {
		if !cfg.ExitWhenChanged {
			t.Fatal("ExitWhenChanged = false, want true")
		}
		return engine.ErrMappingChanged
	})

	if code != 0 {
		t.Fatalf("RunWithContext returned code %d, want clean exit", code)
	}
}

func TestRunWithContextTreatsLocalAddressChangeAsCleanExitWhenRequested(t *testing.T) {
	code := RunWithContext(context.Background(), []string{"-q"}, &bytes.Buffer{}, &bytes.Buffer{}, func(ctx context.Context, cfg config.Config) error {
		if !cfg.ExitWhenChanged {
			t.Fatal("ExitWhenChanged = false, want true")
		}
		return engine.ErrLocalAddressChanged
	})

	if code != 0 {
		t.Fatalf("RunWithContext returned code %d, want clean exit", code)
	}
}
