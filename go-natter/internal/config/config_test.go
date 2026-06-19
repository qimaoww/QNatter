package config

import "testing"

func TestParseArgsMatchesOpenWrtRunnerContract(t *testing.T) {
	cfg, err := ParseArgs([]string{
		"-k", "15",
		"-s", "turn.cloud-rtc.com:80",
		"-s", "fwa.lifesizecloud.com",
		"-i", "pppoe-wan_cmcc",
		"-b", "0",
		"-m", "none",
		"-t", "0.0.0.0",
		"-p", "0",
		"-e", "/var/run/natter/cmcc_.notify",
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
	if cfg.NotifyPath != "/var/run/natter/cmcc_.notify" {
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
