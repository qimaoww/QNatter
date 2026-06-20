package endpoint

import "testing"

func TestSplitHostPortDefault(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		defaultPort int
		wantHost    string
		wantPort    int
	}{
		{name: "host default port", value: "example.com", defaultPort: 80, wantHost: "example.com", wantPort: 80},
		{name: "host explicit port", value: "example.com:443", defaultPort: 80, wantHost: "example.com", wantPort: 443},
		{name: "ipv4 explicit port", value: "192.0.2.10:5353", defaultPort: 53, wantHost: "192.0.2.10", wantPort: 5353},
		{name: "bare ipv6 default port", value: "2001:db8::1", defaultPort: 80, wantHost: "2001:db8::1", wantPort: 80},
		{name: "bracketed ipv6 default port", value: "[2001:db8::2]", defaultPort: 53, wantHost: "2001:db8::2", wantPort: 53},
		{name: "bracketed ipv6 explicit port", value: "[2001:db8::3]:443", defaultPort: 80, wantHost: "2001:db8::3", wantPort: 443},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host, port, err := SplitHostPortDefault(tc.value, tc.defaultPort)
			if err != nil {
				t.Fatalf("SplitHostPortDefault returned error: %v", err)
			}
			if host != tc.wantHost || port != tc.wantPort {
				t.Fatalf("SplitHostPortDefault = %s:%d, want %s:%d", host, port, tc.wantHost, tc.wantPort)
			}
		})
	}
}

func TestSplitHostPortDefaultRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", ":443", "example.com:bad", "example.com:0", "example.com:65536", "[2001:db8::1]extra"} {
		t.Run(value, func(t *testing.T) {
			if _, _, err := SplitHostPortDefault(value, 80); err == nil {
				t.Fatalf("SplitHostPortDefault(%q) returned nil error", value)
			}
		})
	}
}

func TestParsePort(t *testing.T) {
	port, err := ParsePort("3478")
	if err != nil {
		t.Fatalf("ParsePort returned error: %v", err)
	}
	if port != 3478 {
		t.Fatalf("ParsePort = %d, want 3478", port)
	}
}

func TestParsePortRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "bad", "0", "65536"} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParsePort(value); err == nil {
				t.Fatalf("ParsePort(%q) returned nil error", value)
			}
		})
	}
}
