package stun

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestClientGetMappingReturnsInnerAndOuterAddress(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 1, 2, 3, 4, 5, 6, 7, 8}
	transport := &fakeTransport{
		inner:    netip.MustParseAddrPort("192.0.2.10:50000"),
		response: mappedResponse(txid, netip.MustParseAddrPort("203.0.113.9:62000")),
	}
	client := Client{
		Servers: []Server{{Host: "stun.example", Port: 3478}},
		Source:  netip.MustParseAddrPort("0.0.0.0:0"),
		TxID:    fixedTxID(txid),
		Do:      transport.Exchange,
	}

	mapping, err := client.GetMapping(context.Background())
	if err != nil {
		t.Fatalf("GetMapping returned error: %v", err)
	}

	if mapping.Inner != netip.MustParseAddrPort("192.0.2.10:50000") {
		t.Fatalf("inner = %s, want 192.0.2.10:50000", mapping.Inner)
	}
	if mapping.Outer != netip.MustParseAddrPort("203.0.113.9:62000") {
		t.Fatalf("outer = %s, want 203.0.113.9:62000", mapping.Outer)
	}
	if client.Source != mapping.Inner {
		t.Fatalf("client source = %s, want updated inner address", client.Source)
	}
	if got := binary.BigEndian.Uint32(transport.requests[0][8:12]); got != 0x4e415452 {
		t.Fatalf("transaction id prefix = %#x, want NATR", got)
	}
}

func TestClientGetMappingRotatesUnavailableServer(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 9, 8, 7, 6, 5, 4, 3, 2}
	firstErr := errors.New("timeout")
	transport := &fakeTransport{
		errs:  []error{firstErr, nil},
		inner: netip.MustParseAddrPort("192.0.2.11:50001"),
		response: mappedResponse(
			txid,
			netip.MustParseAddrPort("198.51.100.7:61000"),
		),
	}
	client := Client{
		Servers: []Server{
			{Host: "bad.example", Port: 3478},
			{Host: "good.example", Port: 3478},
		},
		Source: netip.MustParseAddrPort("0.0.0.0:0"),
		TxID:   fixedTxID(txid),
		Do:     transport.Exchange,
	}

	mapping, err := client.GetMapping(context.Background())
	if err != nil {
		t.Fatalf("GetMapping returned error: %v", err)
	}

	if len(transport.servers) != 2 {
		t.Fatalf("server attempts = %#v, want two attempts", transport.servers)
	}
	if transport.servers[0].Host != "bad.example" || transport.servers[1].Host != "good.example" {
		t.Fatalf("server attempts = %#v, want bad then good", transport.servers)
	}
	if client.Servers[0].Host != "good.example" {
		t.Fatalf("first server after rotation = %s, want good.example", client.Servers[0].Host)
	}
	if mapping.Server.Host != "good.example" {
		t.Fatalf("mapping server = %s, want good.example", mapping.Server.Host)
	}
}

func TestClientGetMappingReturnsTypedErrorWhenAllServersUnavailable(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 4, 4, 4, 4, 4, 4, 4, 4}
	transport := &fakeTransport{
		errs: []error{
			errors.New("first timeout"),
			errors.New("second timeout"),
		},
	}
	client := Client{
		Servers: []Server{
			{Host: "first.example", Port: 3478},
			{Host: "second.example", Port: 3478},
		},
		Source: netip.MustParseAddrPort("0.0.0.0:0"),
		TxID:   fixedTxID(txid),
		Do:     transport.Exchange,
	}

	_, err := client.GetMapping(context.Background())
	if !errors.Is(err, ErrNoServerAvailable) {
		t.Fatalf("GetMapping error = %v, want ErrNoServerAvailable", err)
	}
	for _, want := range []string{
		"tcp://first.example:3478 is unavailable: first timeout",
		"tcp://second.example:3478 is unavailable: second timeout",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("GetMapping error = %q, missing %q", err.Error(), want)
		}
	}
	if len(transport.servers) != 2 {
		t.Fatalf("server attempts = %#v, want two attempts", transport.servers)
	}
}

func TestNetworkTransportExchangesTCP(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 1, 1, 1, 1, 1, 1, 1, 1}
	server, stop := startLocalTCPStun(t, txid, netip.MustParseAddrPort("203.0.113.20:53000"))
	defer stop()

	transport := NetworkTransport{Timeout: time.Second}
	inner, response, err := transport.Exchange(
		context.Background(),
		"tcp",
		server,
		netip.MustParseAddrPort("127.0.0.1:0"),
		BuildBindingRequest(txid),
	)
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}
	if !inner.Addr().IsLoopback() || inner.Port() == 0 {
		t.Fatalf("inner = %s, want loopback with allocated port", inner)
	}
	outer, err := ParseMappedAddress(response, txid)
	if err != nil {
		t.Fatalf("ParseMappedAddress returned error: %v", err)
	}
	if outer != netip.MustParseAddrPort("203.0.113.20:53000") {
		t.Fatalf("outer = %s, want 203.0.113.20:53000", outer)
	}
}

func TestNetworkTransportCanBindInterface(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 3, 3, 3, 3, 3, 3, 3, 3}
	server, stop := startLocalTCPStun(t, txid, netip.MustParseAddrPort("203.0.113.22:53002"))
	defer stop()

	transport := NetworkTransport{Timeout: time.Second, Interface: "lo", Reuse: true}
	inner, _, err := transport.Exchange(
		context.Background(),
		"tcp",
		server,
		netip.MustParseAddrPort("127.0.0.1:0"),
		BuildBindingRequest(txid),
	)
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}
	if !inner.Addr().IsLoopback() {
		t.Fatalf("inner = %s, want loopback address", inner)
	}
}

func TestNetworkTransportExchangesUDP(t *testing.T) {
	txid := [12]byte{'N', 'A', 'T', 'R', 2, 2, 2, 2, 2, 2, 2, 2}
	server, stop := startLocalUDPStun(t, txid, netip.MustParseAddrPort("203.0.113.21:53001"))
	defer stop()

	transport := NetworkTransport{Timeout: time.Second}
	inner, response, err := transport.Exchange(
		context.Background(),
		"udp",
		server,
		netip.MustParseAddrPort("127.0.0.1:0"),
		BuildBindingRequest(txid),
	)
	if err != nil {
		t.Fatalf("Exchange returned error: %v", err)
	}
	if !inner.Addr().IsLoopback() || inner.Port() == 0 {
		t.Fatalf("inner = %s, want loopback with allocated port", inner)
	}
	outer, err := ParseMappedAddress(response, txid)
	if err != nil {
		t.Fatalf("ParseMappedAddress returned error: %v", err)
	}
	if outer != netip.MustParseAddrPort("203.0.113.21:53001") {
		t.Fatalf("outer = %s, want 203.0.113.21:53001", outer)
	}
}

type fakeTransport struct {
	servers  []Server
	requests [][]byte
	errs     []error
	inner    netip.AddrPort
	response []byte
}

func (t *fakeTransport) Exchange(ctx context.Context, network string, server Server, source netip.AddrPort, request []byte) (netip.AddrPort, []byte, error) {
	t.servers = append(t.servers, server)
	t.requests = append(t.requests, append([]byte(nil), request...))
	if len(t.errs) > 0 {
		err := t.errs[0]
		t.errs = t.errs[1:]
		if err != nil {
			return netip.AddrPort{}, nil, err
		}
	}
	return t.inner, t.response, nil
}

func fixedTxID(txid [12]byte) func() ([12]byte, error) {
	return func() ([12]byte, error) {
		return txid, nil
	}
}

func mappedResponse(txid [12]byte, mapped netip.AddrPort) []byte {
	response := make([]byte, 32)
	binary.BigEndian.PutUint16(response[0:2], bindingSuccess)
	binary.BigEndian.PutUint16(response[2:4], 12)
	binary.BigEndian.PutUint32(response[4:8], MagicCookie)
	copy(response[8:20], txid[:])
	binary.BigEndian.PutUint16(response[20:22], attrXORMappedAddress)
	binary.BigEndian.PutUint16(response[22:24], 8)
	response[24] = 0
	response[25] = 1
	binary.BigEndian.PutUint16(response[26:28], mapped.Port()^uint16(MagicCookie>>16))
	ip := mapped.Addr().As4()
	cookieBytes := [4]byte{}
	binary.BigEndian.PutUint32(cookieBytes[:], MagicCookie)
	for i := range ip {
		response[28+i] = ip[i] ^ cookieBytes[i]
	}
	return response
}

func startLocalTCPStun(t *testing.T, txid [12]byte, mapped netip.AddrPort) (Server, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		request := make([]byte, 20)
		if _, err := io.ReadFull(conn, request); err != nil {
			return
		}
		_, _ = conn.Write(mappedResponse(txid, mapped))
	}()

	addr := listener.Addr().(*net.TCPAddr)
	return Server{Host: "127.0.0.1", Port: addr.Port}, func() {
		_ = listener.Close()
		<-done
	}
}

func startLocalUDPStun(t *testing.T, txid [12]byte, mapped netip.AddrPort) (Server, func()) {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 1500)
		_, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		_, _ = conn.WriteTo(mappedResponse(txid, mapped), addr)
	}()

	addr := conn.LocalAddr().(*net.UDPAddr)
	return Server{Host: "127.0.0.1", Port: addr.Port}, func() {
		_ = conn.Close()
		<-done
	}
}
