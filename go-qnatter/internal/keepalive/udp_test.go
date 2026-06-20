package keepalive

import (
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestUDPKeepAliveSendsDNSQuery(t *testing.T) {
	packetCh := make(chan []byte, 1)
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 1500)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		packetCh <- append([]byte(nil), buf[:n]...)
		_, _ = conn.WriteTo([]byte{0, 1, 0, 0}, addr)
	}()

	addr := conn.LocalAddr().(*net.UDPAddr)
	client := UDPClient{
		Host:      "127.0.0.1",
		Port:      addr.Port,
		Source:    netip.MustParseAddrPort("127.0.0.1:0"),
		Interface: "lo",
		Timeout:   500 * time.Millisecond,
	}
	if err := client.KeepAlive(); err != nil {
		t.Fatalf("KeepAlive returned error: %v", err)
	}

	select {
	case packet := <-packetCh:
		if len(packet) < 34 {
			t.Fatalf("DNS query length = %d, want at least 34", len(packet))
		}
		if packet[2] != 0x01 || packet[3] != 0x00 {
			t.Fatalf("DNS flags = %02x%02x, want 0100", packet[2], packet[3])
		}
		wantName := []byte("\x09keepalive\x06qnatter\x00")
		name := packet[12 : 12+len(wantName)]
		if string(name) != string(wantName) {
			t.Fatalf("DNS query name = %x, want %x", name, wantName)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive UDP keepalive packet")
	}
}

func TestUDPKeepAliveTimesOutWithoutResponse(t *testing.T) {
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket returned error: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	addr := conn.LocalAddr().(*net.UDPAddr)
	client := UDPClient{
		Host:    "127.0.0.1",
		Port:    addr.Port,
		Timeout: 50 * time.Millisecond,
	}
	if err := client.KeepAlive(); err == nil {
		t.Fatal("KeepAlive returned nil without a UDP response")
	}
}
