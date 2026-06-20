package forward

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"

	"natter-openwrt/go-natter/internal/socketopts"
)

type TestServer struct {
	mu     sync.Mutex
	tcp    net.Listener
	udp    *net.UDPConn
	udpAddr *net.UDPAddr
}

func (s *TestServer) Start(options StartOptions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if options.IP == "" {
		options.IP = "0.0.0.0"
	}
	if options.UDP {
		return s.startUDP(options)
	}
	return s.startTCP(options)
}

func (s *TestServer) Addr() *net.UDPAddr {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.udpAddr != nil {
		return s.udpAddr
	}
	if s.tcp != nil {
		addr := s.tcp.Addr().(*net.TCPAddr)
		return &net.UDPAddr{IP: addr.IP, Port: addr.Port}
	}
	return nil
}

func (s *TestServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.tcp != nil {
		err = s.tcp.Close()
		s.tcp = nil
	}
	if s.udp != nil {
		if closeErr := s.udp.Close(); err == nil {
			err = closeErr
		}
		s.udp = nil
		s.udpAddr = nil
	}
	return err
}

func (s *TestServer) startTCP(options StartOptions) error {
	listenConfig := net.ListenConfig{Control: socketopts.Control(socketopts.Options{
		Interface: options.Interface,
		Reuse:     true,
	})}
	ln, err := listenConfig.Listen(context.Background(), "tcp", fmt.Sprintf("%s:%d", options.IP, options.Port))
	if err != nil {
		return err
	}
	s.tcp = ln
	go func() {
		_ = http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Header().Set("Server", "Natter")
			_, _ = fmt.Fprint(w, "<html><body><h1>It works!</h1><hr/>Natter</body></html>\r\n")
		}))
	}()
	return nil
}

func (s *TestServer) startUDP(options StartOptions) error {
	listenConfig := net.ListenConfig{Control: socketopts.Control(socketopts.Options{
		Interface: options.Interface,
		Reuse:     true,
	})}
	packetConn, err := listenConfig.ListenPacket(context.Background(), "udp", fmt.Sprintf("%s:%d", options.IP, options.Port))
	if err != nil {
		return err
	}
	conn := packetConn.(*net.UDPConn)
	s.udp = conn
	s.udpAddr = conn.LocalAddr().(*net.UDPAddr)
	go s.serveUDP(conn)
	return nil
}

func (s *TestServer) serveUDP(conn *net.UDPConn) {
	buf := make([]byte, 8192)
	for {
		_, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		_, _ = conn.WriteToUDP([]byte("It works! - Natter\r\n"), addr)
	}
}
