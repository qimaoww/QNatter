package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"qnatter-openwrt/go-qnatter/internal/socketopts"
)

const (
	// maxUDPClients 限制 UDP 转发为每个源地址维护的出站 socket 数量：
	// 无上限时大量源地址（BT peer、扫描/反射流量）会耗尽进程 fd。
	maxUDPClients = 1024
	// udpClientIdleTimeout 之后没有流量的出站 socket 会被回收。
	udpClientIdleTimeout = 2 * time.Minute
	// udpClientSweepInterval 是空闲回收的检查周期。
	udpClientSweepInterval = 30 * time.Second
)

type SocketForwarder struct {
	mu       sync.Mutex
	listener net.Listener
	udp      *net.UDPConn
	addr     *net.TCPAddr
	udpAddr  *net.UDPAddr
	target   string
	// MaxUDPClients / UDPClientIdle 覆盖 UDP 转发客户端的上限与空闲回收
	// 时间（<=0 时使用默认值）。
	MaxUDPClients int
	UDPClientIdle time.Duration
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
	// G-12：监听套接字必须带 SO_REUSEPORT（与 keep-alive socket 共用同一
	// 内网端口），代价是同端口的第二个监听者不会得到 EADDRINUSE 而是被内核
	// 随机分流。绑定前先探测占用，明确拒绝而不是静默共享。
	if err := sharedListenPortError(options.IP, options.Port, options.UDP); err != nil {
		return err
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

	// 单向 EOF 是 half-close（客户端 shutdown(WR) 或目标端关闭自己的写端），
	// 不是整条连接的结束：只关闭同向的写端，让反方向继续传完剩余数据。
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(outbound, inbound)
		closeWrite(outbound)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(inbound, outbound)
		closeWrite(inbound)
	}()
	go func() {
		wg.Wait()
		_ = inbound.Close()
		_ = outbound.Close()
	}()
}

func closeWrite(conn net.Conn) {
	if closer, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closer.CloseWrite()
		return
	}
	_ = conn.Close()
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

	maxClients := f.MaxUDPClients
	if maxClients <= 0 {
		maxClients = maxUDPClients
	}
	idle := f.UDPClientIdle
	if idle <= 0 {
		idle = udpClientIdleTimeout
	}
	clients := newUDPClientSet(maxClients, idle)
	defer clients.closeAll()

	sweepEvery := idle / 2
	if sweepEvery > udpClientSweepInterval {
		sweepEvery = udpClientSweepInterval
	}
	if sweepEvery < 10*time.Millisecond {
		sweepEvery = 10 * time.Millisecond
	}
	sweeper := time.NewTicker(sweepEvery)
	defer sweeper.Stop()
	go func() {
		for now := range sweeper.C {
			clients.sweepIdle(now)
		}
	}()

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}

		key := clientAddr.String()
		client := clients.touch(key)
		if client == nil {
			created, err := f.newUDPOutbound(conn, clientAddr, func(outbound *net.UDPConn) {
				clients.forget(key, outbound)
			})
			if err != nil {
				continue
			}
			client = clients.add(key, created)
			if client == nil {
				// 达到上限：丢弃新源地址，而不是继续分配 fd。
				_ = created.Close()
				continue
			}
		}

		if _, err := client.conn.Write(buf[:n]); err != nil {
			clients.forget(key, client.conn)
		}
	}
}

func (f *SocketForwarder) newUDPOutbound(server *net.UDPConn, clientAddr *net.UDPAddr, onDone func(*net.UDPConn)) (*net.UDPConn, error) {
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
				if onDone != nil {
					onDone(outbound)
				} else {
					_ = outbound.Close()
				}
				return
			}
			_, _ = server.WriteToUDP(buf[:n], clientAddr)
		}
	}()

	return outbound, nil
}

type udpClient struct {
	conn *net.UDPConn
	seen time.Time
}

// udpClientSet 保存“每个源地址一个出站 socket”的状态，提供数量上限与
// 空闲回收，避免 fd/goroutine 随源地址数量单调增长。
type udpClientSet struct {
	mu      sync.Mutex
	clients map[string]*udpClient
	max     int
	idle    time.Duration
}

func newUDPClientSet(max int, idle time.Duration) *udpClientSet {
	return &udpClientSet{
		clients: make(map[string]*udpClient),
		max:     max,
		idle:    idle,
	}
}

// touch 返回该源地址已有的出站 socket，并刷新其活跃时间。
func (s *udpClientSet) touch(key string) *udpClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	client := s.clients[key]
	if client != nil {
		client.seen = time.Now()
	}
	return client
}

// add 登记新客户端；达到上限时返回 nil（调用方应关闭并丢弃新 socket）。
func (s *udpClientSet) add(key string, conn *net.UDPConn) *udpClient {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clients) >= s.max {
		return nil
	}
	client := &udpClient{conn: conn, seen: time.Now()}
	s.clients[key] = client
	return client
}

// forget 删除并关闭指定出站 socket（读循环结束或写失败时调用）。
func (s *udpClientSet) forget(key string, conn *net.UDPConn) {
	s.mu.Lock()
	client := s.clients[key]
	if client == nil || client.conn != conn {
		s.mu.Unlock()
		return
	}
	delete(s.clients, key)
	s.mu.Unlock()
	_ = conn.Close()
}

// sweepIdle 关闭并删除空闲超时的客户端。
func (s *udpClientSet) sweepIdle(now time.Time) {
	s.mu.Lock()
	expired := make([]*net.UDPConn, 0)
	for key, client := range s.clients {
		if now.Sub(client.seen) >= s.idle {
			delete(s.clients, key)
			expired = append(expired, client.conn)
		}
	}
	s.mu.Unlock()

	for _, conn := range expired {
		_ = conn.Close()
	}
}

// closeAll 关闭所有出站 socket（监听端结束时调用）。
func (s *udpClientSet) closeAll() {
	s.mu.Lock()
	conns := make([]*net.UDPConn, 0, len(s.clients))
	for key, client := range s.clients {
		delete(s.clients, key)
		conns = append(conns, client.conn)
	}
	s.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}
