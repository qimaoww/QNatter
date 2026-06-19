package upnp

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDiscoverOpensBoundUDPSocket(t *testing.T) {
	var openedNetwork string
	var openedAddress string
	conn := &fakePacketConn{}

	locations, err := Discover(context.Background(), DiscoverOptions{
		BindIP:    "192.0.2.10",
		Interface: "pppoe-wan_cmcc",
		OpenPacket: func(ctx context.Context, network string, address string, options DiscoverOptions) (PacketConn, error) {
			openedNetwork = network
			openedAddress = address
			if options.Interface != "pppoe-wan_cmcc" {
				t.Fatalf("interface = %q", options.Interface)
			}
			return conn, nil
		},
	})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if len(locations) != 0 {
		t.Fatalf("locations = %#v, want none", locations)
	}
	if openedNetwork != "udp4" {
		t.Fatalf("opened network = %q, want udp4", openedNetwork)
	}
	if openedAddress != "192.0.2.10:0" {
		t.Fatalf("opened address = %q, want 192.0.2.10:0", openedAddress)
	}
	if len(conn.writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(conn.writes))
	}
}

func TestSearchMessagesMatchPythonDiscoveryTargets(t *testing.T) {
	messages := searchMessages()
	if len(messages) != 2 {
		t.Fatalf("search message count = %d, want 2", len(messages))
	}

	tests := []struct {
		index int
		st    string
	}{
		{index: 0, st: "ssdp:all"},
		{index: 1, st: "upnp:rootdevice"},
	}
	for _, tc := range tests {
		msg := string(messages[tc.index])
		for _, want := range []string{
			"M-SEARCH * HTTP/1.1\r\n",
			"ST: " + tc.st + "\r\n",
			"MX: 2\r\n",
			"MAN: \"ssdp:discover\"\r\n",
			"HOST: 239.255.255.250:1900\r\n",
			"\r\n",
		} {
			if !strings.Contains(msg, want) {
				t.Fatalf("message %d missing %q:\n%s", tc.index, want, msg)
			}
		}
	}
}

func TestParseSSDPResponseExtractsLocation(t *testing.T) {
	location, ok := parseSSDPResponse([]byte("HTTP/1.1 200 OK\r\nCACHE-CONTROL: max-age=120\r\nLOCATION: http://192.168.1.1:5000/rootDesc.xml\r\nST: upnp:rootdevice\r\n\r\n"), "192.168.1.1")
	if !ok {
		t.Fatalf("parseSSDPResponse ok = false")
	}
	if location.IP != "192.168.1.1" {
		t.Fatalf("IP = %q", location.IP)
	}
	if location.URL != "http://192.168.1.1:5000/rootDesc.xml" {
		t.Fatalf("URL = %q", location.URL)
	}
}

func TestDiscoverLocationsSendsSearchesAndDeduplicatesResponses(t *testing.T) {
	target := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}
	conn := &fakePacketConn{
		responses: []fakePacket{
			{
				data: []byte("HTTP/1.1 200 OK\r\nLOCATION: http://192.168.1.1:5000/rootDesc.xml\r\n\r\n"),
				addr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1900},
			},
			{
				data: []byte("HTTP/1.1 200 OK\r\nLOCATION: http://192.168.1.1:5000/rootDesc.xml\r\n\r\n"),
				addr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1900},
			},
			{
				data: []byte("HTTP/1.1 200 OK\r\nLOCATION: http://192.168.2.1:5000/rootDesc.xml\r\n\r\n"),
				addr: &net.UDPAddr{IP: net.ParseIP("192.168.2.1"), Port: 1900},
			},
		},
	}

	locations, err := DiscoverLocations(context.Background(), conn, target)
	if err != nil {
		t.Fatalf("DiscoverLocations returned error: %v", err)
	}
	if len(conn.writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(conn.writes))
	}
	if conn.writeAddrs[0].String() != target.String() || conn.writeAddrs[1].String() != target.String() {
		t.Fatalf("writes went to %#v", conn.writeAddrs)
	}
	if len(locations) != 2 {
		t.Fatalf("location count = %d, want 2: %#v", len(locations), locations)
	}
	if locations[0].IP != "192.168.1.1" || locations[0].URL != "http://192.168.1.1:5000/rootDesc.xml" {
		t.Fatalf("first location = %#v", locations[0])
	}
	if locations[1].IP != "192.168.2.1" || locations[1].URL != "http://192.168.2.1:5000/rootDesc.xml" {
		t.Fatalf("second location = %#v", locations[1])
	}
}

type fakePacket struct {
	data []byte
	addr net.Addr
}

type fakePacketConn struct {
	writes     [][]byte
	writeAddrs []net.Addr
	responses  []fakePacket
}

func (c *fakePacketConn) WriteTo(data []byte, addr net.Addr) (int, error) {
	c.writes = append(c.writes, append([]byte(nil), data...))
	c.writeAddrs = append(c.writeAddrs, addr)
	return len(data), nil
}

func (c *fakePacketConn) ReadFrom(data []byte) (int, net.Addr, error) {
	if len(c.responses) == 0 {
		return 0, nil, timeoutError{}
	}
	packet := c.responses[0]
	c.responses = c.responses[1:]
	copy(data, packet.data)
	return len(packet.data), packet.addr, nil
}

func (c *fakePacketConn) SetDeadline(time.Time) error {
	return nil
}

func (c *fakePacketConn) Close() error {
	return nil
}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "timeout"
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Temporary() bool {
	return true
}

var _ net.Error = timeoutError{}

func TestDiscoverLocationsReturnsWriteErrors(t *testing.T) {
	conn := &errorPacketConn{err: errors.New("network unreachable")}
	_, err := DiscoverLocations(context.Background(), conn, &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900})
	if err == nil {
		t.Fatalf("DiscoverLocations returned nil error")
	}
}

type errorPacketConn struct {
	err error
}

func (c *errorPacketConn) WriteTo([]byte, net.Addr) (int, error) {
	return 0, c.err
}

func (c *errorPacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, nil, timeoutError{}
}

func (c *errorPacketConn) SetDeadline(time.Time) error {
	return nil
}

func (c *errorPacketConn) Close() error {
	return nil
}
