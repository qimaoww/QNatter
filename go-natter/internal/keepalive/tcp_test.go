package keepalive

import (
	"bufio"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestTCPKeepAliveSendsNatterHeadRequest(t *testing.T) {
	requestCh := make(chan string, 1)
	ln := startTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			t.Errorf("ReadString returned error: %v", err)
			return
		}
		requestCh <- strings.TrimSpace(line)
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"))
		time.Sleep(time.Second)
	})

	client := TCPClient{
		Host:      "127.0.0.1",
		Port:      ln.Addr().(*net.TCPAddr).Port,
		Source:    netip.MustParseAddrPort("127.0.0.1:0"),
		Interface: "lo",
		Timeout:   500 * time.Millisecond,
	}
	if err := client.KeepAlive(); err != nil {
		t.Fatalf("KeepAlive returned error: %v", err)
	}

	select {
	case got := <-requestCh:
		if got != "HEAD /natter-keep-alive HTTP/1.1" {
			t.Fatalf("request line = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not receive keepalive request")
	}
}

func TestTCPKeepAliveReportsClosedConnection(t *testing.T) {
	ln := startTCPServer(t, func(conn net.Conn) {
		_ = conn.Close()
	})

	client := TCPClient{
		Host:    "127.0.0.1",
		Port:    ln.Addr().(*net.TCPAddr).Port,
		Timeout: 200 * time.Millisecond,
	}
	if err := client.KeepAlive(); err == nil {
		t.Fatal("KeepAlive returned nil for a closed connection")
	}
}

func startTCPServer(t *testing.T, handler func(net.Conn)) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		handler(conn)
	}()

	return ln
}
