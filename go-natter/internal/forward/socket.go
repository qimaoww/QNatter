package forward

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

type SocketForwarder struct {
	mu       sync.Mutex
	listener net.Listener
	addr     *net.TCPAddr
	target   string
}

func (f *SocketForwarder) Start(options StartOptions) error {
	if options.UDP {
		return fmt.Errorf("UDP socket forwarding is not implemented yet")
	}
	if options.IP == "" {
		options.IP = "0.0.0.0"
	}
	if options.TargetIP == "" {
		options.TargetIP = "0.0.0.0"
	}
	if options.IP == options.TargetIP && options.Port == options.TargetPort {
		return fmt.Errorf("cannot forward to the same address %s:%d", options.IP, options.Port)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", options.IP, options.Port))
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

func (f *SocketForwarder) Stop() error {
	f.mu.Lock()
	ln := f.listener
	f.listener = nil
	f.addr = nil
	f.mu.Unlock()

	if ln == nil {
		return nil
	}
	return ln.Close()
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
