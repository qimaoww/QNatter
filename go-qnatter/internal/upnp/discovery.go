package upnp

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"strings"
	"time"

	"qnatter-openwrt/go-qnatter/internal/socketopts"
)

const ssdpAddress = "239.255.255.250:1900"

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
		timeout = time.Second
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
	deadline := time.Now().Add(time.Second)
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
	buffer := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return locations, nil
		default:
		}
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			return nil, err
		}
		ip := ""
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			ip = udpAddr.IP.String()
		} else if addr != nil {
			ip = addr.String()
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
		"MX: 2\r\n" +
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
		if !strings.HasPrefix(location, "http://") {
			return Location{}, false
		}
		return Location{IP: ip, URL: location}, true
	}
	return Location{}, false
}
