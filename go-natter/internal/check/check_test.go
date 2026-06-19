package check

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/config"
)

func TestRunPrintsNatterCheckReport(t *testing.T) {
	var stdout bytes.Buffer
	runner := Runner{
		TCP: func(context.Context, config.Config) (Result, error) {
			return Result{Status: OK, Info: "NAT Type: Full Cone"}, nil
		},
		UDP: func(context.Context, config.Config) (Result, error) {
			return Result{Status: NA, Info: "NAT Type: Unknown"}, nil
		},
	}

	err := runner.Run(context.Background(), config.Config{}, &stdout)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"> NatterCheck v2.2.1-go\n\n",
		"Checking TCP NAT...",
		"[   OK   ] ... NAT Type: Full Cone",
		"Checking UDP NAT...",
		"[   NA   ] ... NAT Type: Unknown",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, missing %q", out, want)
		}
	}
}

func TestRunConvertsCheckErrorToFailLine(t *testing.T) {
	var stdout bytes.Buffer
	runner := Runner{
		TCP: func(context.Context, config.Config) (Result, error) {
			return Result{}, errors.New("tcp probe failed")
		},
		UDP: func(context.Context, config.Config) (Result, error) {
			return Result{Status: COMPAT, Info: "NAT Type: Restricted"}, nil
		},
	}

	err := runner.Run(context.Background(), config.Config{}, &stdout)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "[  FAIL  ] ... tcp probe failed") {
		t.Fatalf("output = %q, want TCP failure line", out)
	}
	if !strings.Contains(out, "[ COMPAT ] ... NAT Type: Restricted") {
		t.Fatalf("output = %q, want UDP compat line", out)
	}
}

func TestDefaultRunReportsUnimplementedChecksWithoutFakeSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), config.Config{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "check: ok") {
		t.Fatalf("output = %q, must not report fake success", out)
	}
	if !strings.Contains(out, "[  FAIL  ] ... Go TCP NAT check is not implemented yet") {
		t.Fatalf("output = %q, want TCP not implemented line", out)
	}
	if !strings.Contains(out, "[  FAIL  ] ... Go UDP NAT check is not implemented yet") {
		t.Fatalf("output = %q, want UDP not implemented line", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
}
