package status

import (
	"encoding/json"
	"net/netip"
	"os"
	"strings"
	"testing"
)

func TestWriteMappingStatusMatchesLuCIContract(t *testing.T) {
	file := t.TempDir() + "/cmcc.json"
	mapping := Mapping{
		Instance: "cmcc",
		Protocol: "tcp",
		Inner:    netip.MustParseAddrPort("10.10.77.188:44627"),
		Outer:    netip.MustParseAddrPort("198.51.100.73:8931"),
		Message:  "mapped",
		Now:      func() string { return "2026-06-20 03:22:44" },
	}

	if err := WriteMapping(file, mapping); err != nil {
		t.Fatalf("WriteMapping returned error: %v", err)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("status JSON is invalid: %v\n%s", err, raw)
	}

	assertString(t, got, "instance", "cmcc")
	assertString(t, got, "protocol", "tcp")
	assertString(t, got, "inner_ip", "10.10.77.188")
	assertNumber(t, got, "inner_port", 44627)
	assertString(t, got, "outer_ip", "198.51.100.73")
	assertNumber(t, got, "outer_port", 8931)
	assertString(t, got, "updated_at", "2026-06-20 03:22:44")
	assertString(t, got, "message", "mapped")
}

func TestWriteMappingDefaultsInstanceAndMessage(t *testing.T) {
	file := t.TempDir() + "/default.json"
	mapping := Mapping{
		Protocol: "udp",
		Inner:    netip.MustParseAddrPort("192.0.2.10:50000"),
		Outer:    netip.MustParseAddrPort("198.51.100.20:62000"),
		Now:      func() string { return "2026-06-20 04:30:00" },
	}

	if err := WriteMapping(file, mapping); err != nil {
		t.Fatalf("WriteMapping returned error: %v", err)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("status JSON is invalid: %v\n%s", err, raw)
	}
	assertString(t, got, "instance", "default")
	assertString(t, got, "message", "mapped")
}

func TestWriteMappingDoesNotHTMLEscapeStatusStrings(t *testing.T) {
	file := t.TempDir() + "/html.json"
	mapping := Mapping{
		Instance: "cmcc",
		Protocol: "tcp",
		Inner:    netip.MustParseAddrPort("192.0.2.10:40000"),
		Outer:    netip.MustParseAddrPort("198.51.100.10:62000"),
		Message:  "qBittorrent <tag>&value",
		Now:      func() string { return "2026-06-20 05:00:00" },
	}

	if err := WriteMapping(file, mapping); err != nil {
		t.Fatalf("WriteMapping returned error: %v", err)
	}

	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(raw) == "" || raw[len(raw)-1] != '\n' {
		t.Fatalf("status JSON must end with newline: %q", raw)
	}
	if got := string(raw); !strings.Contains(got, `"message":"qBittorrent <tag>&value"`) {
		t.Fatalf("status JSON HTML-escaped message: %s", got)
	}
	if strings.Contains(string(raw), `\u003c`) || strings.Contains(string(raw), `\u003e`) || strings.Contains(string(raw), `\u0026`) {
		t.Fatalf("status JSON contains HTML unicode escapes: %s", raw)
	}
}

func TestWriteMappingAtomicallyReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/status.json"
	link := dir + "/old-status.json"

	if err := os.WriteFile(file, []byte("old status\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Link(file, link); err != nil {
		t.Skipf("hard links are not available: %v", err)
	}

	mapping := Mapping{
		Instance: "cmcc",
		Protocol: "tcp",
		Inner:    netip.MustParseAddrPort("192.0.2.10:40000"),
		Outer:    netip.MustParseAddrPort("198.51.100.10:62000"),
		Now:      func() string { return "2026-06-20 05:30:00" },
	}

	if err := WriteMapping(file, mapping); err != nil {
		t.Fatalf("WriteMapping returned error: %v", err)
	}

	raw, err := os.ReadFile(link)
	if err != nil {
		t.Fatalf("ReadFile hard link returned error: %v", err)
	}
	if got := string(raw); got != "old status\n" {
		t.Fatalf("WriteMapping updated the old inode in place: %q", got)
	}
}

func assertString(t *testing.T, got map[string]any, key string, want string) {
	t.Helper()
	if got[key] != want {
		t.Fatalf("%s = %#v, want %q", key, got[key], want)
	}
}

func assertNumber(t *testing.T, got map[string]any, key string, want float64) {
	t.Helper()
	if got[key] != want {
		t.Fatalf("%s = %#v, want %.0f", key, got[key], want)
	}
}
