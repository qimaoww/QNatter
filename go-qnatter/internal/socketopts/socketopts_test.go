package socketopts

import (
	"net"
	"net/netip"
	"syscall"
	"testing"
)

func TestLocalAddrForTCPAndUDP(t *testing.T) {
	source := netip.MustParseAddrPort("192.0.2.10:45678")

	tcp, err := LocalAddr("tcp", source)
	if err != nil {
		t.Fatalf("LocalAddr tcp returned error: %v", err)
	}
	tcpAddr, ok := tcp.(*net.TCPAddr)
	if !ok {
		t.Fatalf("tcp local addr = %T, want *net.TCPAddr", tcp)
	}
	if tcpAddr.IP.String() != "192.0.2.10" || tcpAddr.Port != 45678 {
		t.Fatalf("tcp local addr = %s:%d", tcpAddr.IP, tcpAddr.Port)
	}

	udp, err := LocalAddr("udp", source)
	if err != nil {
		t.Fatalf("LocalAddr udp returned error: %v", err)
	}
	udpAddr, ok := udp.(*net.UDPAddr)
	if !ok {
		t.Fatalf("udp local addr = %T, want *net.UDPAddr", udp)
	}
	if udpAddr.IP.String() != "192.0.2.10" || udpAddr.Port != 45678 {
		t.Fatalf("udp local addr = %s:%d", udpAddr.IP, udpAddr.Port)
	}
}

func TestNetworkForSourceForcesIPv4WhenBindingIPv4Source(t *testing.T) {
	source := netip.MustParseAddrPort("0.0.0.0:0")

	if got := NetworkForSource("tcp", source); got != "tcp4" {
		t.Fatalf("tcp network = %q, want tcp4", got)
	}
	if got := NetworkForSource("udp", source); got != "udp4" {
		t.Fatalf("udp network = %q, want udp4", got)
	}
}

func TestNetworkForSourceKeepsBaseNetworkWithoutIPv4Source(t *testing.T) {
	if got := NetworkForSource("tcp", netip.AddrPort{}); got != "tcp" {
		t.Fatalf("invalid source network = %q, want tcp", got)
	}
	if got := NetworkForSource("tcp", netip.MustParseAddrPort("[2001:db8::1]:5000")); got != "tcp" {
		t.Fatalf("IPv6 source network = %q, want tcp", got)
	}
}

func TestControlAppliesInterfaceAndReuseOptions(t *testing.T) {
	raw := fakeRawConn{fd: 99}
	var intCalls []intCall
	var stringCalls []stringCall

	control := ControlWith(Options{Interface: "pppoe-wan_cmcc", Reuse: true}, Setters{
		Int: func(fd uintptr, level int, opt int, value int) error {
			intCalls = append(intCalls, intCall{fd: fd, level: level, opt: opt, value: value})
			return nil
		},
		String: func(fd uintptr, level int, opt int, value string) error {
			stringCalls = append(stringCalls, stringCall{fd: fd, level: level, opt: opt, value: value})
			return nil
		},
	})

	if err := control("tcp", "stun.example:3478", raw); err != nil {
		t.Fatalf("control returned error: %v", err)
	}

	hasReuseAddr := false
	for _, call := range intCalls {
		if call.fd == 99 && call.level == syscall.SOL_SOCKET && call.opt == syscall.SO_REUSEADDR && call.value == 1 {
			hasReuseAddr = true
		}
	}
	if !hasReuseAddr {
		t.Fatalf("reuse addr call missing: %#v", intCalls)
	}

	if len(stringCalls) != 1 {
		t.Fatalf("string calls = %#v, want one SO_BINDTODEVICE call", stringCalls)
	}
	if stringCalls[0].fd != 99 || stringCalls[0].level != syscall.SOL_SOCKET ||
		stringCalls[0].opt != soBindToDevice || stringCalls[0].value != "pppoe-wan_cmcc" {
		t.Fatalf("bind device call = %#v", stringCalls[0])
	}
}

type fakeRawConn struct {
	fd uintptr
}

func (f fakeRawConn) Control(fn func(uintptr)) error {
	fn(f.fd)
	return nil
}

func (f fakeRawConn) Read(func(uintptr) bool) error {
	return nil
}

func (f fakeRawConn) Write(func(uintptr) bool) error {
	return nil
}

type intCall struct {
	fd    uintptr
	level int
	opt   int
	value int
}

type stringCall struct {
	fd    uintptr
	level int
	opt   int
	value string
}
