package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgsMatchesOpenWrtRunnerContract(t *testing.T) {
	notifyPath := filepath.Join(t.TempDir(), "cmcc.notify")
	if err := os.WriteFile(notifyPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := ParseArgs([]string{
		"-k", "15",
		"-s", "turn.cloud-rtc.com:80",
		"-s", "fwa.lifesizecloud.com",
		"-i", "pppoe-wan_cmcc",
		"-b", "0",
		"-m", "none",
		"-t", "0.0.0.0",
		"-p", "0",
		"-e", notifyPath,
	})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if cfg.KeepAliveInterval != 15 {
		t.Fatalf("keepalive = %d, want 15", cfg.KeepAliveInterval)
	}
	if cfg.BindValue != "pppoe-wan_cmcc" {
		t.Fatalf("bind value = %q, want pppoe-wan_cmcc", cfg.BindValue)
	}
	if cfg.BindPort != 0 {
		t.Fatalf("bind port = %d, want 0", cfg.BindPort)
	}
	if cfg.ForwardMethod != "none" {
		t.Fatalf("forward method = %q, want none", cfg.ForwardMethod)
	}
	if cfg.TargetIP != "0.0.0.0" || cfg.TargetPort != 0 {
		t.Fatalf("target = %s:%d, want 0.0.0.0:0", cfg.TargetIP, cfg.TargetPort)
	}
	if cfg.NotifyPath != notifyPath {
		t.Fatalf("notify path = %q", cfg.NotifyPath)
	}
	if len(cfg.STUNServers) != 2 {
		t.Fatalf("STUN server count = %d, want 2", len(cfg.STUNServers))
	}
	if cfg.STUNServers[0].Host != "turn.cloud-rtc.com" || cfg.STUNServers[0].Port != 80 {
		t.Fatalf("first STUN = %+v, want turn.cloud-rtc.com:80", cfg.STUNServers[0])
	}
	if cfg.STUNServers[1].Host != "fwa.lifesizecloud.com" || cfg.STUNServers[1].Port != 3478 {
		t.Fatalf("second STUN = %+v, want default port 3478", cfg.STUNServers[1])
	}
}

func TestParseArgsDefaults(t *testing.T) {
	cfg, err := ParseArgs(nil)
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}

	if cfg.KeepAliveInterval != 15 {
		t.Fatalf("default keepalive = %d, want 15", cfg.KeepAliveInterval)
	}
	if cfg.BindValue != "0.0.0.0" {
		t.Fatalf("default bind value = %q, want 0.0.0.0", cfg.BindValue)
	}
	if cfg.TargetIP != "0.0.0.0" {
		t.Fatalf("default target IP = %q, want 0.0.0.0", cfg.TargetIP)
	}
	if len(cfg.STUNServers) != 11 {
		t.Fatalf("default TCP STUN server count = %d, want 11", len(cfg.STUNServers))
	}
	if cfg.STUNServers[10].Host != "turn.cloud-rtc.com" || cfg.STUNServers[10].Port != 80 {
		t.Fatalf("last default TCP STUN = %+v, want turn.cloud-rtc.com:80", cfg.STUNServers[10])
	}
}

func TestParseArgsNormalizesIPv4LikePython(t *testing.T) {
	cfg, err := ParseArgs([]string{"-i", "10.1", "-t", "127.1"})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.BindValue != "10.0.0.1" {
		t.Fatalf("bind value = %q, want 10.0.0.1", cfg.BindValue)
	}
	if cfg.TargetIP != "127.0.0.1" {
		t.Fatalf("target IP = %q, want 127.0.0.1", cfg.TargetIP)
	}
}

func TestParseArgsAcceptsSTUNServerSchemePrefixes(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"-s", "tcp://turn.cloud-rtc.com:80",
		"-s", "udp://stun.example.com",
	})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.STUNServers[0].Host != "turn.cloud-rtc.com" || cfg.STUNServers[0].Port != 80 {
		t.Fatalf("first STUN = %+v, want turn.cloud-rtc.com:80", cfg.STUNServers[0])
	}
	if cfg.STUNServers[1].Host != "stun.example.com" || cfg.STUNServers[1].Port != 3478 {
		t.Fatalf("second STUN = %+v, want stun.example.com:3478", cfg.STUNServers[1])
	}
}

func TestParseArgsAcceptsIPv6STUNServers(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"-s", "[2001:db8::1]:5349",
		"-s", "2001:db8::2",
	})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.STUNServers[0].Host != "2001:db8::1" || cfg.STUNServers[0].Port != 5349 {
		t.Fatalf("first STUN = %+v, want 2001:db8::1:5349", cfg.STUNServers[0])
	}
	if cfg.STUNServers[1].Host != "2001:db8::2" || cfg.STUNServers[1].Port != 3478 {
		t.Fatalf("second STUN = %+v, want 2001:db8::2:3478", cfg.STUNServers[1])
	}
}

func TestParseArgsAcceptsIPv6KeepAliveServers(t *testing.T) {
	for _, server := range []string{"2001:db8::1", "[2001:db8::2]", "[2001:db8::3]:443"} {
		t.Run(server, func(t *testing.T) {
			cfg, err := ParseArgs([]string{"-h", server})
			if err != nil {
				t.Fatalf("ParseArgs returned error: %v", err)
			}
			if cfg.KeepAliveServer != server {
				t.Fatalf("keepalive server = %q, want %q", cfg.KeepAliveServer, server)
			}
		})
	}
}

func TestParseArgsRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "zero keepalive", args: []string{"-k", "0"}},
		{name: "negative bind port", args: []string{"-b", "-1"}},
		{name: "large target port", args: []string{"-p", "65536"}},
		{name: "invalid target IP", args: []string{"-t", "not-an-ip"}},
		{name: "invalid keepalive server port", args: []string{"-h", "example.com:bad"}},
		{name: "missing notify file", args: []string{"-e", filepath.Join(t.TempDir(), "missing")}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseArgs(tc.args); err == nil {
				t.Fatalf("ParseArgs(%v) returned nil error", tc.args)
			}
		})
	}
}

func TestParseArgsAcceptsExistingNotifyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notify.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := ParseArgs([]string{"-e", path})
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if cfg.NotifyPath != path {
		t.Fatalf("notify path = %q, want %q", cfg.NotifyPath, path)
	}
}
