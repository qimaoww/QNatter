package qbittorrent

import "testing"

func TestValidPort(t *testing.T) {
	for _, port := range []int{1, 51413, 65535} {
		if !ValidPort(port) {
			t.Fatalf("ValidPort(%d) = false, want true", port)
		}
	}
	for _, port := range []int{-1, 0, 65536} {
		if ValidPort(port) {
			t.Fatalf("ValidPort(%d) = true, want false", port)
		}
	}
}

func TestSelectListenPortMatchesShellPrecedence(t *testing.T) {
	tests := []struct {
		name       string
		innerPort  int
		outerPort  int
		configured int
		want       int
	}{
		{name: "configured wins", innerPort: 5000, outerPort: 62000, configured: 51413, want: 51413},
		{name: "outer fallback", innerPort: 5000, outerPort: 62000, configured: 0, want: 62000},
		{name: "inner fallback", innerPort: 5000, outerPort: 0, configured: 0, want: 5000},
		{name: "none valid", innerPort: 0, outerPort: 0, configured: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SelectListenPort(tt.innerPort, tt.outerPort, tt.configured)
			if got != tt.want {
				t.Fatalf("SelectListenPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPreferencesJSON(t *testing.T) {
	got, err := PreferencesJSON(62000)
	if err != nil {
		t.Fatalf("PreferencesJSON(62000) returned error: %v", err)
	}
	if got != `{"listen_port":62000}` {
		t.Fatalf("PreferencesJSON(62000) = %q", got)
	}

	// N-P2-11：非法端口必须报错，而不是静默写成 listen_port=0（0 表示
	// "随机端口"，会把配置错误伪装成合法值）。
	for _, port := range []int{0, -1, 65536} {
		if got, err := PreferencesJSON(port); err == nil {
			t.Fatalf("PreferencesJSON(%d) = %q, want error", port, got)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	got, err := NormalizeURL("http://127.0.0.1:8080///")
	if err != nil {
		t.Fatalf("NormalizeURL returned error: %v", err)
	}
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("NormalizeURL() = %q", got)
	}

	if got, err := NormalizeURL("https://example.com/"); err != nil || got != "https://example.com" {
		t.Fatalf("NormalizeURL(https://example.com/) = %q, %v", got, err)
	}

	// scheme 白名单：非 http/https 一律拒绝。
	for _, raw := range []string{"/", "", "ftp://127.0.0.1:8080", "file:///etc/passwd", "http://"} {
		if got, err := NormalizeURL(raw); err == nil {
			t.Fatalf("NormalizeURL(%q) = %q, want error", raw, got)
		}
	}
}
