package notify

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/status"
)

func TestRunWritesStatusAndCallsUserScript(t *testing.T) {
	dir := t.TempDir()
	statusFile := filepath.Join(dir, "cmcc.json")
	argsFile := filepath.Join(dir, "args.txt")
	script := filepath.Join(dir, "notify.sh")
	scriptBody := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\n"
	if err := os.WriteFile(script, []byte(scriptBody), 0o755); err != nil {
		t.Fatalf("WriteFile script returned error: %v", err)
	}

	mapping := status.Mapping{
		Instance: "cmcc",
		Protocol: "tcp",
		Inner:    netip.MustParseAddrPort("10.10.77.188:44627"),
		Outer:    netip.MustParseAddrPort("198.51.100.73:8931"),
		Message:  "mapped",
		Now:      func() string { return "2026-06-20 03:22:44" },
	}

	result, err := Run(Options{
		Instance:   "cmcc",
		StatusFile: statusFile,
		UserScript: script,
	}, mapping)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.UserNotifyError != "" {
		t.Fatalf("user notify error = %q, want empty", result.UserNotifyError)
	}

	raw, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatalf("ReadFile status returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("status JSON invalid: %v", err)
	}
	if got["instance"] != "cmcc" || got["message"] != "mapped" {
		t.Fatalf("unexpected status JSON: %s", raw)
	}

	argsRaw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ReadFile user notify args returned error: %v", err)
	}
	gotArgs := strings.Split(strings.TrimSpace(string(argsRaw)), "\n")
	wantArgs := []string{"tcp", "10.10.77.188", "44627", "198.51.100.73", "8931"}
	if strings.Join(gotArgs, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("notify args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestRunIgnoresEmptyUserScript(t *testing.T) {
	statusFile := filepath.Join(t.TempDir(), "default.json")
	mapping := status.Mapping{
		Instance: "default",
		Protocol: "tcp",
		Inner:    netip.MustParseAddrPort("192.0.2.197:41353"),
		Outer:    netip.MustParseAddrPort("203.0.113.123:7533"),
		Message:  "mapped",
	}

	result, err := Run(Options{Instance: "default", StatusFile: statusFile}, mapping)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.UserNotifyError != "" {
		t.Fatalf("user notify error = %q, want empty", result.UserNotifyError)
	}
	if _, err := os.Stat(statusFile); err != nil {
		t.Fatalf("status file was not written: %v", err)
	}
}

func TestRunReportsNonExecutableUserScriptWithoutFailingStatus(t *testing.T) {
	dir := t.TempDir()
	statusFile := filepath.Join(dir, "default.json")
	script := filepath.Join(dir, "notify.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile script returned error: %v", err)
	}

	result, err := Run(Options{Instance: "default", StatusFile: statusFile, UserScript: script}, status.Mapping{
		Instance: "default",
		Protocol: "tcp",
		Inner:    netip.MustParseAddrPort("192.0.2.197:41353"),
		Outer:    netip.MustParseAddrPort("203.0.113.123:7533"),
		Message:  "mapped",
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(result.UserNotifyError, "not executable") {
		t.Fatalf("user notify error = %q, want not executable", result.UserNotifyError)
	}
	if _, err := os.Stat(statusFile); err != nil {
		t.Fatalf("status file was not written: %v", err)
	}
}
