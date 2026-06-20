package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"natter-openwrt/go-natter/internal/socketopts"
)

type SocketForwarder struct {
	mu       sync.Mutex
	listener net.Listener
	udp      *net.UDPConn
	addr     *net.TCPAddr
	udpAddr  *net.UDPAddr
	target   string
}

func (f *SocketForwarder) Start(options StartOptions) error {
	if options.IP == "" {
		options.IP = "0.0.0.0"
	}
	if options.TargetIP == "" {
		options.TargetIP = "0.0.0.0"
	}
	if options.IP == options.TargetIP && options.Port == options.TargetPort {
		return fmt.Errorf("cannot forward to the same address %s:%d", options.IP, options.Port)
	}
	if options.UDP {
		return f.startUDP(options)
	}

	listenConfig := net.ListenConfig{Control: socketopts.Control(socketopts.Options{
		Interface: options.Interface,
		Reuse:     true,
	})}
	ln, err := listenConfig.Listen(context.Background(), "tcp", fmt.Sprintf("%s:%d", options.IP, options.Port))
	if err != nil {
		return err
	}

	f.mu.Lock()
	f.listener = ln
	f.addr = ln.Addr().(*net.TCPAddr)
	f.target = fmt.Sprintf("%s:%d", options.TargetIP, options.TargetPort)
	f.mu.Unlock()

	go f.acceptLoop(ln)
	return nil
}

func (f *SocketForwarder) Addr() *net.TCPAddr {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addr
}

func (f *SocketForwarder) UDPAddr() *net.UDPAddr {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.udpAddr
}

func (f *SocketForwarder) Stop() error {
	f.mu.Lock()
	ln := f.listener
	udp := f.udp
	f.listener = nil
	f.addr = nil
	f.udp = nil
	f.udpAddr = nil
	f.mu.Unlock()

	var err error
	if ln != nil {
		err = ln.Close()
	}
	if udp != nil {
		if closeErr := udp.Close(); err == nil {
			err = closeErr
		}
	}
	return err
}

func (f *SocketForwarder) acceptLoop(ln net.Listener) {
	for {
		inbound, err := ln.Accept()
		if err != nil {
			return
		}
		go f.handleTCP(inbound)
	}
}

func (f *SocketForwarder) handleTCP(inbound net.Conn) {
	f.mu.Lock()
	target := f.target
	f.mu.Unlock()

	outbound, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		_ = inbound.Close()
		return
	}

	var once sync.Once
	closeBoth := func() {
		_ = inbound.Close()
		_ = outbound.Close()
	}
	go func() {
		_, _ = io.Copy(outbound, inbound)
		once.Do(closeBoth)
	}()
	go func() {
		_, _ = io.Copy(inbound, outbound)
		once.Do(closeBoth)
	}()
}

func (f *SocketForwarder) startUDP(options StartOptions) error {
	listenConfig := net.ListenConfig{Control: socketopts.Control(socketopts.Options{
		Interface: options.Interface,
		Reuse:     true,
	})}
	packetConn, err := listenConfig.ListenPacket(context.Background(), "udp", fmt.Sprintf("%s:%d", options.IP, options.Port))
	if err != nil {
		return err
	}
	conn := packetConn.(*net.UDPConn)

	f.mu.Lock()
	f.udp = conn
	f.udpAddr = conn.LocalAddr().(*net.UDPAddr)
	f.target = fmt.Sprintf("%s:%d", options.TargetIP, options.TargetPort)
	f.mu.Unlock()

	go f.udpLoop(conn)
	return nil
}

func (f *SocketForwarder) udpLoop(conn *net.UDPConn) {
	buf := make([]byte, 8192)
	clients := map[string]*net.UDPConn{}
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			for _, outbound := range clients {
				_ = outbound.Close()
			}
			return
		}

		key := clientAddr.String()
		outbound := clients[key]
		if outbound == nil {
			created, err := f.newUDPOutbound(conn, clientAddr)
			if err != nil {
				continue
			}
			outbound = created
			clients[key] = outbound
		}

		if _, err := outbound.Write(buf[:n]); err != nil {
			_ = outbound.Close()
			delete(clients, key)
		}
	}
}

func (f *SocketForwarder) newUDPOutbound(server *net.UDPConn, clientAddr *net.UDPAddr) (*net.UDPConn, error) {
	f.mu.Lock()
	target := f.target
	f.mu.Unlock()

	targetAddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		return nil, err
	}
	outbound, err := net.DialUDP("udp", nil, targetAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		buf := make([]byte, 8192)
		for {
			n, err := outbound.Read(buf)
			if err != nil {
				_ = outbound.Close()
				return
			}
			_, _ = server.WriteToUDP(buf[:n], clientAddr)
		}
	}()

	return outbound, nil
}
