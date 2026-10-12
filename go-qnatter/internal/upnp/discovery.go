package upnp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"qnatter-openwrt/go-qnatter/internal/socketopts"
)

const (
	ssdpAddress = "239.255.255.250:1900"
	// ssdpMX 是 M-SEARCH 的 MX 值：设备应在 0..MX 秒内响应。监听窗口
	// 必须与它一致，否则接近 MX 才发出的响应会被丢弃。
	ssdpMX = 2
	// locationHostLookupTimeout 限制把 LOCATION 主机名解析回来源地址的时间，
	// 避免发现阶段被不响应的 DNS 拖住。
	locationHostLookupTimeout = 2 * time.Second
	// maxSSDPReplies 是接收响应条数的上限（含重复与无效响应）：没有上限时
	// 局域网内任意主机都能用大量响应把发现阶段拖长。
	maxSSDPReplies = 256
	// maxSSDPLocations 是返回给调用方的 location 上限：每个 location 都会
	// 触发一次同步 HTTP 抓取（设备描述），必须单独设限，否则伪造响应可以
	// 放大成大量同步请求（审计实测单窗口 5000 条）。
	maxSSDPLocations = 64
	// maxSSDPSources 是不同来源 IP 的上限：来源去重 + 上限，避免响应来源
	// 完全不受约束。
	maxSSDPSources = 16
	// ssdpReadBufferSize / maxSSDPDatagram 是读缓冲的初始值与上限：
	// 读到满缓冲（可能被截断）时扩容到 UDP 报文上限，本次响应明确丢弃，
	// 而不是把半条 LOCATION 当成完整响应。
	ssdpReadBufferSize = 4096
	maxSSDPDatagram    = 65507
)

type Location struct {
	IP  string
	URL string
}

type DiscoverOptions struct {
	BindIP     string
	Interface  string
	Timeout    time.Duration
	OpenPacket func(context.Context, string, string, DiscoverOptions) (PacketConn, error)
}

type PacketConn interface {
	WriteTo([]byte, net.Addr) (int, error)
	ReadFrom([]byte) (int, net.Addr, error)
	SetDeadline(time.Time) error
	Close() error
}

func Discover(ctx context.Context, options DiscoverOptions) ([]Location, error) {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = ssdpMX * time.Second
	}
	discoverCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	openPacket := options.OpenPacket
	if openPacket == nil {
		openPacket = defaultOpenPacket
	}
	address := ":0"
	if options.BindIP != "" && options.BindIP != "0.0.0.0" {
		address = net.JoinHostPort(options.BindIP, "0")
	}
	conn, err := openPacket(discoverCtx, "udp4", address, options)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	target, err := net.ResolveUDPAddr("udp4", ssdpAddress)
	if err != nil {
		return nil, err
	}
	return DiscoverLocations(discoverCtx, conn, target)
}

func DiscoverLocations(ctx context.Context, conn PacketConn, target net.Addr) ([]Location, error) {
	// 设备按 MX 随机延迟响应，监听窗口必须 ≥ MX。
	deadline := time.Now().Add(ssdpMX * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}

	for _, message := range searchMessages() {
		if _, err := conn.WriteTo(message, target); err != nil {
			return nil, err
		}
	}

	var locations []Location
	seen := make(map[Location]bool)
	sources := make(map[string]bool)
	buffer := make([]byte, ssdpReadBufferSize)
	replies := 0
	for {
		select {
		case <-ctx.Done():
			return locations, nil
		default:
		}
		if replies >= maxSSDPReplies || len(locations) >= maxSSDPLocations {
			// 配额用完：继续读只会放大后续的同步 HTTP 抓取。
			break
		}
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return nil, err
		}
		replies++
		if n >= len(buffer) && len(buffer) < maxSSDPDatagram {
			// 响应可能被缓冲截断：明确丢弃这一条，并扩容供后续响应使用
			// （UDP 数据报无法重读，所以截断的那条只能丢）。
			next := len(buffer) * 2
			if next > maxSSDPDatagram {
				next = maxSSDPDatagram
			}
			buffer = make([]byte, next)
			continue
		}
		ip := ""
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			ip = udpAddr.IP.String()
		} else if addr != nil {
			ip = addr.String()
		}
		if ip != "" {
			if !sources[ip] && len(sources) >= maxSSDPSources {
				continue
			}
			sources[ip] = true
		}
		location, ok := parseSSDPResponse(buffer[:n], ip)
		if !ok || seen[location] {
			continue
		}
		seen[location] = true
		locations = append(locations, location)
	}
	return locations, nil
}

func defaultOpenPacket(ctx context.Context, network string, address string, options DiscoverOptions) (PacketConn, error) {
	listenConfig := net.ListenConfig{
		Control: socketopts.Control(socketopts.Options{
			Interface: options.Interface,
			Reuse:     true,
		}),
	}
	return listenConfig.ListenPacket(ctx, network, address)
}

func searchMessages() [][]byte {
	return [][]byte{
		searchMessage("ssdp:all"),
		searchMessage("upnp:rootdevice"),
	}
}

func searchMessage(st string) []byte {
	return []byte("M-SEARCH * HTTP/1.1\r\n" +
		"ST: " + st + "\r\n" +
		fmt.Sprintf("MX: %d\r\n", ssdpMX) +
		"MAN: \"ssdp:discover\"\r\n" +
		"HOST: " + ssdpAddress + "\r\n" +
		"\r\n")
}

func parseSSDPResponse(data []byte, ip string) (Location, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		name, value, ok := strings.Cut(line, ":")
		if !ok || !strings.EqualFold(name, "LOCATION") {
			continue
		}
		location := strings.TrimSpace(value)
		if !sameLocationHost(ip, location) {
			return Location{}, false
		}
		return Location{IP: ip, URL: location}, true
	}
	return Location{}, false
}

// sameLocationHost 要求 LOCATION 的主机确实是这条 SSDP 响应的来源主机。
//
// 没有这道校验时，局域网内任意主机（或伪造的 SSDP 响应）都能把 LOCATION
// 指向自己，于是 QNatter 会把设备描述请求、以及随后的 SOAP 请求发给攻击者，
// 形成盲 SSRF 与日志 oracle。只接受两种形式：与来源相同的 IP 字面量，或能
// 解析回来源 IP 的主机名；其余情况（含解析失败）一律拒绝（fail-closed）。
func sameLocationHost(sourceIP, location string) bool {
	parsed, err := url.Parse(location)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	if net.ParseIP(host) != nil {
		return sameIP(host, sourceIP)
	}
	lookupCtx, cancel := context.WithTimeout(context.Background(), locationHostLookupTimeout)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupHost(lookupCtx, host)
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		if sameIP(addr, sourceIP) {
			return true
		}
	}
	return false
}

func sameIP(a, b string) bool {
	ipA, ipB := net.ParseIP(a), net.ParseIP(b)
	if ipA == nil || ipB == nil {
		return false
	}
	return ipA.Equal(ipB)
}
