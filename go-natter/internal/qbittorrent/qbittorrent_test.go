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
	got := PreferencesJSON(62000)
	if got != `{"listen_port":62000}` {
		t.Fatalf("PreferencesJSON(62000) = %q", got)
	}

	got = PreferencesJSON(0)
	if got != `{"listen_port":0}` {
		t.Fatalf("PreferencesJSON(0) = %q", got)
	}
}

func TestNormalizeURL(t *testing.T) {
	got := NormalizeURL("http://127.0.0.1:8080///")
	if got != "http://127.0.0.1:8080" {
		t.Fatalf("NormalizeURL() = %q", got)
	}
}
