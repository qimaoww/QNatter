package portcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"natter-openwrt/go-natter/internal/socketopts"
)

type Result int

const (
	Closed  Result = -1
	Unknown Result = 0
	Open    Result = 1
)

type Probe func(context.Context, int, netip.Addr) (Result, error)

type Checker struct {
	Timeout           time.Duration
	Interface         string
	IfconfigProbe     Probe
	TransmissionProbe Probe
}

func (c Checker) TestLAN(ctx context.Context, addr netip.AddrPort, source netip.Addr) Result {
	timeout := c.timeout(1 * time.Second)
	dialer := net.Dialer{Timeout: timeout}
	if source.IsValid() {
		dialer.LocalAddr = &net.TCPAddr{IP: net.IP(source.AsSlice())}
	}
	dialer.Control = socketopts.Control(socketopts.Options{Interface: c.Interface})
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", addr.String())
	if err != nil {
		return Closed
	}
	_ = conn.Close()
	return Open
}

func (c Checker) TestWAN(ctx context.Context, port int, source netip.Addr) Result {
	ifconfig := c.IfconfigProbe
	if ifconfig == nil {
		ifconfig = c.testIfconfig
	}
	transmission := c.TransmissionProbe
	if transmission == nil {
		transmission = c.testTransmission
	}

	ret1, err1 := ifconfig(ctx, port, source)
	if err1 == nil && ret1 == Open {
		return Open
	}
	ret2, err2 := transmission(ctx, port, source)
	if err2 == nil && ret2 == Open {
		return Open
	}
	if err1 == nil && err2 == nil && ret1 == Closed && ret2 == Closed {
		return Closed
	}
	return Unknown
}

func (c Checker) testIfconfig(ctx context.Context, port int, source netip.Addr) (Result, error) {
	body, err := c.httpGet(ctx, "ifconfig.co", fmt.Sprintf("/port/%d", port), source)
	if err != nil {
		return Unknown, err
	}
	return ParseIfconfigResponse(body)
}

func (c Checker) testTransmission(ctx context.Context, port int, source netip.Addr) (Result, error) {
	body, err := c.httpGet(ctx, "portcheck.transmissionbt.com", fmt.Sprintf("/%d", port), source)
	if err != nil {
		return Unknown, err
	}
	return ParseTransmissionResponse(body)
}

func (c Checker) httpGet(ctx context.Context, host string, path string, source netip.Addr) ([]byte, error) {
	timeout := c.timeout(8 * time.Second)
	dialer := &net.Dialer{Timeout: timeout}
	if source.IsValid() {
		dialer.LocalAddr = &net.TCPAddr{IP: net.IP(source.AsSlice())}
	}
	dialer.Control = socketopts.Control(socketopts.Options{Interface: c.Interface})

	client := http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: dialer.DialContext,
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+path, nil)
	if err != nil {
		return nil, err
	}
	req.Host = host
	req.Header.Set("User-Agent", "curl/8.0.0 (Natter)")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (c Checker) timeout(fallback time.Duration) time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return fallback
}

func ParseIfconfigResponse(body []byte) (Result, error) {
	var response struct {
		Reachable bool `json:"reachable"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Unknown, err
	}
	if response.Reachable {
		return Open, nil
	}
	return Closed, nil
}

func ParseTransmissionResponse(body []byte) (Result, error) {
	switch strings.TrimSpace(string(body)) {
	case "1":
		return Open, nil
	case "0":
		return Closed, nil
	default:
		return Unknown, fmt.Errorf("unexpected transmission portcheck response")
	}
}
