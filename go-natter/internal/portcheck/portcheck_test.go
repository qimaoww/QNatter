package portcheck

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestLANReportsOpenPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	defer listener.Close()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	checker := Checker{}
	result := checker.TestLAN(context.Background(), netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(addr.Port)), netip.Addr{})

	if result != Open {
		t.Fatalf("TestLAN = %v, want Open", result)
	}
}

func TestLANReportsClosedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	_ = listener.Close()

	checker := Checker{}
	result := checker.TestLAN(context.Background(), netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(addr.Port)), netip.Addr{})

	if result != Closed {
		t.Fatalf("TestLAN = %v, want Closed", result)
	}
}

func TestWANReportsOpenWhenEitherProbeIsOpen(t *testing.T) {
	checker := Checker{
		IfconfigProbe:     fixedProbe(Unknown, nil),
		TransmissionProbe: fixedProbe(Open, nil),
	}

	result := checker.TestWAN(context.Background(), 51413, netip.Addr{})

	if result != Open {
		t.Fatalf("TestWAN = %v, want Open", result)
	}
}

func TestWANReportsClosedOnlyWhenBothProbesAreClosed(t *testing.T) {
	checker := Checker{
		IfconfigProbe:     fixedProbe(Closed, nil),
		TransmissionProbe: fixedProbe(Closed, nil),
	}

	result := checker.TestWAN(context.Background(), 51413, netip.Addr{})

	if result != Closed {
		t.Fatalf("TestWAN = %v, want Closed", result)
	}
}

func TestWANReportsUnknownWhenAnyProbeIsUnknown(t *testing.T) {
	checker := Checker{
		IfconfigProbe:     fixedProbe(Closed, nil),
		TransmissionProbe: fixedProbe(Unknown, errors.New("bad gateway")),
	}

	result := checker.TestWAN(context.Background(), 51413, netip.Addr{})

	if result != Unknown {
		t.Fatalf("TestWAN = %v, want Unknown", result)
	}
}

func TestParseIfconfigResponse(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
		want Result
	}{
		{name: "reachable", body: []byte(`{"reachable":true}`), want: Open},
		{name: "unreachable", body: []byte(`{"reachable":false}`), want: Closed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseIfconfigResponse(tc.body)
			if err != nil {
				t.Fatalf("ParseIfconfigResponse returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("ParseIfconfigResponse = %v, want %v", got, tc.want)
			}
		})
	}

	if _, err := ParseIfconfigResponse([]byte(`not json`)); err == nil {
		t.Fatal("ParseIfconfigResponse accepted invalid JSON")
	}
}

func TestParseTransmissionResponse(t *testing.T) {
	tests := map[string]Result{
		"1":     Open,
		"0":     Closed,
		" 1 \n": Open,
	}
	for body, want := range tests {
		got, err := ParseTransmissionResponse([]byte(body))
		if err != nil {
			t.Fatalf("ParseTransmissionResponse(%q) returned error: %v", body, err)
		}
		if got != want {
			t.Fatalf("ParseTransmissionResponse(%q) = %v, want %v", body, got, want)
		}
	}

	if _, err := ParseTransmissionResponse([]byte("maybe")); err == nil {
		t.Fatal("ParseTransmissionResponse accepted invalid body")
	}
}

func fixedProbe(result Result, err error) Probe {
	return func(context.Context, int, netip.Addr) (Result, error) {
		return result, err
	}
}
