//go:build linux

package procname

import (
	"os"
	"strings"
	"testing"
)

func TestSetUpdatesLinuxCommName(t *testing.T) {
	original, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Skipf("cannot read /proc/self/comm: %v", err)
	}
	t.Cleanup(func() {
		_ = Set(strings.TrimSpace(string(original)))
	})

	if err := Set("QNatter"); err != nil {
		t.Fatalf("Set returned error: %v", err)
	}
	raw, err := os.ReadFile("/proc/self/comm")
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if got := string(raw); got != "QNatter\n" {
		t.Fatalf("comm name = %q, want QNatter", got)
	}
}
