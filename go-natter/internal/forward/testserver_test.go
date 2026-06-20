package forward

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNoneForwarderDoesNothing(t *testing.T) {
	f := None{}
	if err := f.Start(StartOptions{IP: "127.0.0.1", Port: 12345, TargetIP: "127.0.0.1", TargetPort: 80}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := f.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestTestServerTCPRespondsWithNatterPage(t *testing.T) {
	f := &TestServer{}
	if err := f.Start(StartOptions{IP: "127.0.0.1", Port: 0}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = f.Stop() })

	client := http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + f.Addr().String())
	if err != nil {
		t.Fatalf("GET returned error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if resp.Header.Get("Server") != "Natter" {
		t.Fatalf("Server header = %q, want Natter", resp.Header.Get("Server"))
	}
	if !strings.Contains(string(body), "It works!") || !strings.Contains(string(body), "Natter") {
		t.Fatalf("body = %q, want Natter test page", body)
	}
}

func TestTestServerTCPAppliesInterfaceBinding(t *testing.T) {
	f := &TestServer{}
	if err := f.Start(StartOptions{IP: "127.0.0.1", Port: 0, Interface: "natter-missing-iface"}); err == nil {
		_ = f.Stop()
		t.Fatal("Start accepted a missing bind interface")
	}
}

func TestTestServerUDPRespondsWithNatterMessage(t *testing.T) {
	f := &TestServer{}
	if err := f.Start(StartOptions{IP: "127.0.0.1", Port: 0, UDP: true}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	t.Cleanup(func() { _ = f.Stop() })

	conn, err := net.DialUDP("udp", nil, f.Addr())
	if err != nil {
		t.Fatalf("DialUDP returned error: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("SetDeadline returned error: %v", err)
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got := string(buf[:n]); got != "It works! - Natter\r\n" {
		t.Fatalf("UDP response = %q", got)
	}
}
