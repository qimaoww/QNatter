//go:build linux

package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/procname"
)

func TestRunSetsProcessName(t *testing.T) {
	original, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Skipf("cannot read /proc/self/comm: %v", err)
	}
	t.Cleanup(func() {
		_ = procname.Set(strings.TrimSpace(string(original)))
	})

	code := run(context.Background(), []string{"--version"}, &bytes.Buffer{}, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("run returned code %d, want 0", code)
	}
	raw, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if got := strings.TrimSpace(string(raw)); got != "Natter" {
		t.Fatalf("comm name = %q, want Natter", got)
	}
}
