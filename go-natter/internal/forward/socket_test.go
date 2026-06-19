package forward

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestSocketForwarderTCPForwardsBothDirections(t *testing.T) {
	target := startEchoServer(t)

	f := &SocketForwarder{}
	if err := f.Start(StartOptions{
		IP:         "127.0.0.1",
		Port:       0,
		TargetIP:   "127.0.0.1",
		TargetPort: target.Addr().(*net.TCPAddr).Port,
	}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = f.Stop() })

	conn, err := net.DialTimeout("tcp", f.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("DialTimeout returned error: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetDeadline returned error: %v", err)
	}

	if _, err := fmt.Fprintln(conn, "hello"); err != nil {
		t.Fatalf("write returned error: %v", err)
	}
	got, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString returned error: %v", err)
	}
	if got != "echo: hello\n" {
		t.Fatalf("forwarded response = %q", got)
	}
}

func TestSocketForwarderRejectsSameAddress(t *testing.T) {
	f := &SocketForwarder{}
	err := f.Start(StartOptions{
		IP:         "127.0.0.1",
		Port:       12345,
		TargetIP:   "127.0.0.1",
		TargetPort: 12345,
	})
	if err == nil {
		t.Fatal("Start accepted forwarding to the same address")
	}
}

func startEchoServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen returned error: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				_, _ = fmt.Fprintf(conn, "echo: %s", line)
			}()
		}
	}()

	return ln
}
