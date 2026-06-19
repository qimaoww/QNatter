package forward

import "testing"

func TestResolveMethodMatchesPythonDefaults(t *testing.T) {
	tests := []struct {
		name string
		opts FactoryOptions
		want string
	}{
		{
			name: "default direct test server",
			opts: FactoryOptions{
				TargetIP: "0.0.0.0",
			},
			want: "test",
		},
		{
			name: "bound interface without target uses none",
			opts: FactoryOptions{
				BindValue: "pppoe-wan_cmcc",
				TargetIP:  "0.0.0.0",
			},
			want: "none",
		},
		{
			name: "target uses socket",
			opts: FactoryOptions{
				TargetIP:   "10.10.10.10",
				TargetPort: 51413,
			},
			want: "socket",
		},
		{
			name: "auto follows default rules",
			opts: FactoryOptions{
				Method:     "auto",
				TargetIP:   "10.10.10.10",
				TargetPort: 51413,
			},
			want: "socket",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveMethod(tc.opts)
			if got != tc.want {
				t.Fatalf("ResolveMethod = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNewForwarderCreatesImplementedMethods(t *testing.T) {
	tests := []struct {
		method string
		check  func(Forwarder) bool
	}{
		{method: "none", check: func(f Forwarder) bool { _, ok := f.(None); return ok }},
		{method: "test", check: func(f Forwarder) bool { _, ok := f.(*TestServer); return ok }},
		{method: "socket", check: func(f Forwarder) bool { _, ok := f.(*SocketForwarder); return ok }},
		{method: "nftables", check: func(f Forwarder) bool {
			nft, ok := f.(*NftablesForwarder)
			return ok && !nft.SNAT
		}},
		{method: "nftables-snat", check: func(f Forwarder) bool {
			nft, ok := f.(*NftablesForwarder)
			return ok && nft.SNAT
		}},
		{method: "socat", check: func(f Forwarder) bool { _, ok := f.(*SocatForwarder); return ok }},
		{method: "gost", check: func(f Forwarder) bool { _, ok := f.(*GostForwarder); return ok }},
	}

	for _, tc := range tests {
		t.Run(tc.method, func(t *testing.T) {
			f, err := NewForwarder(tc.method)
			if err != nil {
				t.Fatalf("NewForwarder returned error: %v", err)
			}
			if !tc.check(f) {
				t.Fatalf("NewForwarder(%q) returned %T", tc.method, f)
			}
		})
	}
}

func TestNewForwarderRejectsUnsupportedMethods(t *testing.T) {
	for _, method := range []string{"iptables", "does-not-exist"} {
		t.Run(method, func(t *testing.T) {
			if _, err := NewForwarder(method); err == nil {
				t.Fatalf("NewForwarder(%q) returned nil error", method)
			}
		})
	}
}
