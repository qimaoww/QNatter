package check

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/portcheck"
)

func TestRunPrintsNatterCheckReport(t *testing.T) {
	var stdout bytes.Buffer
	runner := Runner{
		Docker: DockerEnv{GOOS: "darwin"},
		TCP: func(context.Context, config.Config) (Result, error) {
			return Result{Status: OK, Info: "NAT Type: Full Cone"}, nil
		},
		UDP: func(context.Context, config.Config) (Result, error) {
			return Result{Status: NA, Info: "NAT Type: Unknown"}, nil
		},
	}

	err := runner.Run(context.Background(), config.Config{}, &stdout)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"> NatterCheck v2.2.1-go\n\n",
		"Checking TCP NAT...",
		"[   OK   ] ... NAT Type: Full Cone",
		"Checking UDP NAT...",
		"[   NA   ] ... NAT Type: Unknown",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output = %q, missing %q", out, want)
		}
	}
}

func TestRunConvertsCheckErrorToFailLine(t *testing.T) {
	var stdout bytes.Buffer
	runner := Runner{
		Docker: DockerEnv{GOOS: "darwin"},
		TCP: func(context.Context, config.Config) (Result, error) {
			return Result{}, errors.New("tcp probe failed")
		},
		UDP: func(context.Context, config.Config) (Result, error) {
			return Result{Status: COMPAT, Info: "NAT Type: Restricted"}, nil
		},
	}

	err := runner.Run(context.Background(), config.Config{}, &stdout)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "[  FAIL  ] ... tcp probe failed") {
		t.Fatalf("output = %q, want TCP failure line", out)
	}
	if !strings.Contains(out, "[ COMPAT ] ... NAT Type: Restricted") {
		t.Fatalf("output = %q, want UDP compat line", out)
	}
}

func TestNATResultMatchesPythonStatusRules(t *testing.T) {
	tests := []struct {
		name   string
		nat    NATType
		status Status
		info   string
	}{
		{name: "unknown", nat: NATUnknown, status: NA, info: "NAT Type: -1"},
		{name: "open internet", nat: NATOpenInternet, status: OK, info: "NAT Type: 0"},
		{name: "full cone", nat: NATFullCone, status: OK, info: "NAT Type: 1"},
		{name: "restricted", nat: NATRestricted, status: FAIL, info: "NAT Type: 2"},
		{name: "port restricted", nat: NATPortRestricted, status: FAIL, info: "NAT Type: 3"},
		{name: "symmetric", nat: NATSymmetric, status: FAIL, info: "NAT Type: 4"},
		{name: "symmetric udp firewall", nat: NATSymmetricUDPFirewall, status: FAIL, info: "NAT Type: 5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResultFromNATType(tt.nat)
			if got.Status != tt.status || got.Info != tt.info {
				t.Fatalf("ResultFromNATType(%d) = %+v, want status %v info %q", tt.nat, got, tt.status, tt.info)
			}
		})
	}
}

func TestCheckTCPNATTypeMatchesPythonDecisionTree(t *testing.T) {
	tests := []struct {
		name     string
		fullCone int
		cone     int
		want     NATType
	}{
		{name: "open internet", fullCone: tcpFullConeOpenInternet, want: NATOpenInternet},
		{name: "full cone", fullCone: tcpFullConeReachable, want: NATFullCone},
		{name: "full cone unknown", fullCone: tcpFullConeUnknown, want: NATUnknown},
		{name: "port restricted", fullCone: tcpFullConeBlocked, cone: tcpConeStable, want: NATPortRestricted},
		{name: "symmetric", fullCone: tcpFullConeBlocked, cone: tcpConeSymmetric, want: NATSymmetric},
		{name: "cone unknown", fullCone: tcpFullConeBlocked, cone: tcpConeUnknown, want: NATUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckTCPNATType(context.Background(), TCPNATOptions{
				CheckFullCone: func(context.Context, int) (int, error) {
					return tt.fullCone, nil
				},
				CheckCone: func(context.Context) (int, error) {
					return tt.cone, nil
				},
			})
			if err != nil {
				t.Fatalf("CheckTCPNATType returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CheckTCPNATType = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCheckTCPNATTypeSkipsConeWhenFullConeIsConclusive(t *testing.T) {
	calledCone := false
	got, err := CheckTCPNATType(context.Background(), TCPNATOptions{
		CheckFullCone: func(context.Context, int) (int, error) {
			return tcpFullConeReachable, nil
		},
		CheckCone: func(context.Context) (int, error) {
			calledCone = true
			return tcpConeSymmetric, nil
		},
	})
	if err != nil {
		t.Fatalf("CheckTCPNATType returned error: %v", err)
	}
	if got != NATFullCone {
		t.Fatalf("CheckTCPNATType = %d, want full cone", got)
	}
	if calledCone {
		t.Fatal("CheckTCPNATType called cone checker after conclusive full-cone result")
	}
}

func TestCheckTCPConeMatchesPythonDecisionTree(t *testing.T) {
	source := netip.MustParseAddr("192.0.2.10")
	mapped := netip.MustParseAddrPort("198.51.100.10:50000")
	servers := []netip.AddrPort{
		netip.MustParseAddrPort("203.0.113.1:3478"),
		netip.MustParseAddrPort("203.0.113.2:3478"),
		netip.MustParseAddrPort("203.0.113.3:3478"),
		netip.MustParseAddrPort("203.0.113.4:3478"),
	}

	tests := []struct {
		name      string
		responses map[netip.AddrPort]tcpProbeResponse
		want      int
	}{
		{
			name: "stable after three matching mappings",
			responses: map[netip.AddrPort]tcpProbeResponse{
				servers[0]: {result: STUNTestResult{Mapped: mapped}},
				servers[1]: {result: STUNTestResult{Mapped: mapped}},
				servers[2]: {result: STUNTestResult{Mapped: mapped}},
			},
			want: tcpConeStable,
		},
		{
			name: "symmetric when a later mapping differs",
			responses: map[netip.AddrPort]tcpProbeResponse{
				servers[0]: {result: STUNTestResult{Mapped: mapped}},
				servers[1]: {result: STUNTestResult{Mapped: netip.MustParseAddrPort("198.51.100.10:50001")}},
			},
			want: tcpConeSymmetric,
		},
		{
			name: "unknown with fewer than three successful mappings",
			responses: map[netip.AddrPort]tcpProbeResponse{
				servers[0]: {result: STUNTestResult{Mapped: mapped}},
				servers[1]: {result: STUNTestResult{Mapped: mapped}},
			},
			want: tcpConeUnknown,
		},
		{
			name: "skips unavailable servers while counting successes",
			responses: map[netip.AddrPort]tcpProbeResponse{
				servers[0]: {err: errors.New("timeout")},
				servers[1]: {result: STUNTestResult{Mapped: mapped}},
				servers[2]: {result: STUNTestResult{Mapped: mapped}},
				servers[3]: {result: STUNTestResult{Mapped: mapped}},
			},
			want: tcpConeStable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckTCPCone(context.Background(), TCPConeOptions{
				Servers:    servers,
				SourceAddr: source,
				SourcePort: 40000,
				Interface:  "pppoe-wan_cmcc",
				Reuse:      true,
				Probe:      fakeTCPProbe(tt.responses, nil),
			})
			if err != nil {
				t.Fatalf("CheckTCPCone returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CheckTCPCone = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCheckTCPConePassesBindOptionsToProbe(t *testing.T) {
	server := netip.MustParseAddrPort("203.0.113.1:3478")
	var got []TCPProbeRequest

	_, err := CheckTCPCone(context.Background(), TCPConeOptions{
		Servers:    []netip.AddrPort{server},
		SourceAddr: netip.MustParseAddr("192.0.2.10"),
		SourcePort: 40000,
		Interface:  "pppoe-wan_cmcc",
		Reuse:      true,
		Probe: fakeTCPProbe(map[netip.AddrPort]tcpProbeResponse{
			server: {result: STUNTestResult{Mapped: netip.MustParseAddrPort("198.51.100.10:50000")}},
		}, &got),
	})
	if err != nil {
		t.Fatalf("CheckTCPCone returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("probe calls = %d, want 1", len(got))
	}
	request := got[0]
	if request.Server != server {
		t.Fatalf("server = %s, want %s", request.Server, server)
	}
	if request.SourceAddr != netip.MustParseAddr("192.0.2.10") {
		t.Fatalf("source addr = %s, want 192.0.2.10", request.SourceAddr)
	}
	if request.SourcePort != 40000 {
		t.Fatalf("source port = %d, want 40000", request.SourcePort)
	}
	if request.Interface != "pppoe-wan_cmcc" {
		t.Fatalf("interface = %q, want pppoe-wan_cmcc", request.Interface)
	}
	if !request.Reuse {
		t.Fatal("reuse = false, want true")
	}
}

func TestCheckTCPFullConeMatchesPythonDecisionTree(t *testing.T) {
	source := netip.MustParseAddr("192.0.2.10")
	inner := netip.MustParseAddrPort("192.0.2.10:40000")
	mapped := netip.MustParseAddrPort("198.51.100.10:50000")

	tests := []struct {
		name       string
		mapping    STUNTestResult
		portResult portcheck.Result
		want       int
	}{
		{
			name:    "open internet when source equals mapped",
			mapping: STUNTestResult{Source: inner, Mapped: inner},
			want:    tcpFullConeOpenInternet,
		},
		{
			name:       "reachable full cone when public port is open",
			mapping:    STUNTestResult{Source: inner, Mapped: mapped},
			portResult: portcheck.Open,
			want:       tcpFullConeReachable,
		},
		{
			name:       "blocked when public port is closed",
			mapping:    STUNTestResult{Source: inner, Mapped: mapped},
			portResult: portcheck.Closed,
			want:       tcpFullConeBlocked,
		},
		{
			name:       "unknown when public port check is inconclusive",
			mapping:    STUNTestResult{Source: inner, Mapped: mapped},
			portResult: portcheck.Unknown,
			want:       tcpFullConeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CheckTCPFullCone(context.Background(), TCPFullConeOptions{
				SourceAddr: source,
				SourcePort: int(inner.Port()),
				Interface:  "pppoe-wan_cmcc",
				Reuse:      true,
				Listen: func(ctx context.Context, request TCPFullConeListenRequest) (io.Closer, error) {
					return noopCloser{}, nil
				},
				GetMapping: func(ctx context.Context, request TCPFullConeMappingRequest) (STUNTestResult, io.Closer, error) {
					return tt.mapping, noopCloser{}, nil
				},
				CheckPort: func(ctx context.Context, request TCPFullConePortCheckRequest) (portcheck.Result, error) {
					if request.Port != int(mapped.Port()) && tt.mapping.Mapped != inner {
						t.Fatalf("port check port = %d, want %d", request.Port, mapped.Port())
					}
					if request.SourceAddr != source {
						t.Fatalf("port check source = %s, want %s", request.SourceAddr, source)
					}
					return tt.portResult, nil
				},
			})
			if err != nil {
				t.Fatalf("CheckTCPFullCone returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CheckTCPFullCone = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCheckTCPFullConeReturnsUnknownOnSetupFailure(t *testing.T) {
	mappingCalled, portCalled := false, false
	got, err := CheckTCPFullCone(context.Background(), TCPFullConeOptions{
		SourceAddr: netip.MustParseAddr("192.0.2.10"),
		SourcePort: 40000,
		Listen: func(context.Context, TCPFullConeListenRequest) (io.Closer, error) {
			return nil, errors.New("listen failed")
		},
		GetMapping: func(context.Context, TCPFullConeMappingRequest) (STUNTestResult, io.Closer, error) {
			mappingCalled = true
			return STUNTestResult{}, nil, nil
		},
		CheckPort: func(context.Context, TCPFullConePortCheckRequest) (portcheck.Result, error) {
			portCalled = true
			return portcheck.Open, nil
		},
	})
	if err != nil {
		t.Fatalf("CheckTCPFullCone returned error: %v", err)
	}
	if got != tcpFullConeUnknown {
		t.Fatalf("CheckTCPFullCone = %d, want unknown", got)
	}
	if mappingCalled || portCalled {
		t.Fatalf("mappingCalled=%v portCalled=%v, want both false", mappingCalled, portCalled)
	}
}

func TestCheckTCPFullConeClosesResources(t *testing.T) {
	listener := &trackingCloser{}
	keepAlive := &trackingCloser{}

	got, err := CheckTCPFullCone(context.Background(), TCPFullConeOptions{
		SourceAddr: netip.MustParseAddr("192.0.2.10"),
		SourcePort: 40000,
		Listen: func(context.Context, TCPFullConeListenRequest) (io.Closer, error) {
			return listener, nil
		},
		GetMapping: func(context.Context, TCPFullConeMappingRequest) (STUNTestResult, io.Closer, error) {
			return STUNTestResult{
				Source: netip.MustParseAddrPort("192.0.2.10:40000"),
				Mapped: netip.MustParseAddrPort("198.51.100.10:50000"),
			}, keepAlive, nil
		},
		CheckPort: func(context.Context, TCPFullConePortCheckRequest) (portcheck.Result, error) {
			return portcheck.Open, nil
		},
	})
	if err != nil {
		t.Fatalf("CheckTCPFullCone returned error: %v", err)
	}
	if got != tcpFullConeReachable {
		t.Fatalf("CheckTCPFullCone = %d, want reachable", got)
	}
	if !listener.closed || !keepAlive.closed {
		t.Fatalf("listener closed=%v keepAlive closed=%v, want both true", listener.closed, keepAlive.closed)
	}
}

func TestDefaultTCPFullConeListenBindsSourceAddress(t *testing.T) {
	listener, err := defaultTCPFullConeListen(context.Background(), TCPFullConeListenRequest{
		Source: netip.MustParseAddrPort("127.0.0.1:0"),
		Reuse:  true,
	})
	if err != nil {
		t.Fatalf("defaultTCPFullConeListen returned error: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	if !addr.IP.IsLoopback() || addr.Port == 0 {
		t.Fatalf("listener addr = %s, want allocated loopback port", listener.Addr())
	}
}

func TestCheckTCPFullConeUsesDefaultListener(t *testing.T) {
	got, err := CheckTCPFullCone(context.Background(), TCPFullConeOptions{
		SourceAddr: netip.MustParseAddr("127.0.0.1"),
		SourcePort: 0,
		GetMapping: func(ctx context.Context, request TCPFullConeMappingRequest) (STUNTestResult, io.Closer, error) {
			if !request.Source.Addr().IsLoopback() || request.Source.Port() == 0 {
				t.Fatalf("mapping source = %s, want allocated loopback port", request.Source)
			}
			return STUNTestResult{Source: request.Source, Mapped: request.Source}, noopCloser{}, nil
		},
	})
	if err != nil {
		t.Fatalf("CheckTCPFullCone returned error: %v", err)
	}
	if got != tcpFullConeOpenInternet {
		t.Fatalf("CheckTCPFullCone = %d, want open internet", got)
	}
}

func TestCheckTCPFullConeUsesDefaultMapping(t *testing.T) {
	txid := [16]byte{0x21, 0x12, 0xa4, 0x42, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9}
	mapped := netip.MustParseAddrPort("198.51.100.40:54000")
	stunServer, _, stopSTUN := startLocalTCPCheckSTUN(t, txid, mapped)
	defer stopSTUN()
	keepAliveServer, keepAliveRequestCh, stopKeepAlive := startLocalCheckKeepAlive(t)
	defer stopKeepAlive()

	got, err := CheckTCPFullCone(context.Background(), TCPFullConeOptions{
		SourceAddr:      netip.MustParseAddr("127.0.0.1"),
		SourcePort:      0,
		Reuse:           true,
		KeepAliveServer: keepAliveServer,
		STUNServers:     []netip.AddrPort{stunServer},
		TxID:            func() ([16]byte, error) { return txid, nil },
		CheckPort: func(ctx context.Context, request TCPFullConePortCheckRequest) (portcheck.Result, error) {
			if request.Port != int(mapped.Port()) {
				t.Fatalf("port check port = %d, want %d", request.Port, mapped.Port())
			}
			return portcheck.Open, nil
		},
	})
	if err != nil {
		t.Fatalf("CheckTCPFullCone returned error: %v", err)
	}
	if got != tcpFullConeReachable {
		t.Fatalf("CheckTCPFullCone = %d, want reachable", got)
	}

	request := <-keepAliveRequestCh
	if !strings.Contains(request, "GET /~ HTTP/1.1\r\n") {
		t.Fatalf("keepalive request = %q, want HTTP keepalive request", request)
	}
	if !strings.Contains(request, "Connection: keep-alive\r\n") {
		t.Fatalf("keepalive request = %q, want keep-alive header", request)
	}
}

func TestCheckTCPFullConeUsesDefaultPortChecker(t *testing.T) {
	source := netip.MustParseAddr("192.0.2.10")
	mapped := netip.MustParseAddrPort("198.51.100.10:50000")
	var gotPort int
	var gotSource netip.Addr

	got, err := CheckTCPFullCone(context.Background(), TCPFullConeOptions{
		SourceAddr: source,
		SourcePort: 40000,
		Listen: func(context.Context, TCPFullConeListenRequest) (io.Closer, error) {
			return noopCloser{}, nil
		},
		GetMapping: func(context.Context, TCPFullConeMappingRequest) (STUNTestResult, io.Closer, error) {
			return STUNTestResult{
				Source: netip.MustParseAddrPort("192.0.2.10:40000"),
				Mapped: mapped,
			}, noopCloser{}, nil
		},
		PortChecker: portcheck.Checker{
			IfconfigProbe: func(ctx context.Context, port int, source netip.Addr) (portcheck.Result, error) {
				gotPort = port
				gotSource = source
				return portcheck.Open, nil
			},
			TransmissionProbe: fixedPortProbe(portcheck.Unknown, nil),
		},
	})
	if err != nil {
		t.Fatalf("CheckTCPFullCone returned error: %v", err)
	}
	if got != tcpFullConeReachable {
		t.Fatalf("CheckTCPFullCone = %d, want reachable", got)
	}
	if gotPort != int(mapped.Port()) {
		t.Fatalf("port checker port = %d, want %d", gotPort, mapped.Port())
	}
	if gotSource != source {
		t.Fatalf("port checker source = %s, want %s", gotSource, source)
	}
}

func TestCheckUDPNATTypeMatchesPythonDecisionTree(t *testing.T) {
	source := netip.MustParseAddrPort("192.0.2.10:40000")
	mapped := netip.MustParseAddrPort("198.51.100.10:50000")
	servers := []netip.AddrPort{
		netip.MustParseAddrPort("203.0.113.1:3478"),
		netip.MustParseAddrPort("203.0.113.2:3478"),
	}

	tests := []struct {
		name      string
		responses map[udpProbeKey]udpProbeResponse
		want      NATType
	}{
		{
			name: "symmetric when normal mappings differ",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: netip.MustParseAddrPort("198.51.100.10:50001")}},
				{server: servers[1], changeIP: true, changePort: true}: {
					result: STUNTestResult{Source: source, Mapped: mapped, IPChanged: true, PortChanged: true},
				},
			},
			want: NATSymmetric,
		},
		{
			name: "open internet when source equals mapped and test2 responds",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: source}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: source}},
				{server: servers[1], changeIP: true, changePort: true}: {
					result: STUNTestResult{Source: source, Mapped: source, IPChanged: true, PortChanged: true},
				},
			},
			want: NATOpenInternet,
		},
		{
			name: "symmetric udp firewall when source equals mapped and test2 is silent",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: source}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: source}},
			},
			want: NATSymmetricUDPFirewall,
		},
		{
			name: "full cone when test2 responds behind nat",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1], changeIP: true, changePort: true}: {
					result: STUNTestResult{Source: source, Mapped: mapped, IPChanged: true, PortChanged: true},
				},
			},
			want: NATFullCone,
		},
		{
			name: "restricted when test3 responds behind nat",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1], changePort: true}: {
					result: STUNTestResult{Source: source, Mapped: mapped, PortChanged: true},
				},
			},
			want: NATRestricted,
		},
		{
			name: "port restricted when test2 and test3 are silent behind nat",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
			},
			want: NATPortRestricted,
		},
		{
			name: "unknown with fewer than two normal responses",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
			},
			want: NATUnknown,
		},
		{
			name: "skips unusable changed response and tries later server",
			responses: map[udpProbeKey]udpProbeResponse{
				{server: servers[0]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1]}: {result: STUNTestResult{Source: source, Mapped: mapped}},
				{server: servers[1], changeIP: true, changePort: true}: {
					result: STUNTestResult{Source: source, Mapped: mapped, IPChanged: false, PortChanged: true},
				},
				{server: netip.MustParseAddrPort("203.0.113.3:3478")}: {
					result: STUNTestResult{Source: source, Mapped: mapped},
				},
				{server: netip.MustParseAddrPort("203.0.113.3:3478"), changeIP: true, changePort: true}: {
					result: STUNTestResult{Source: source, Mapped: mapped, IPChanged: true, PortChanged: true},
				},
			},
			want: NATFullCone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testServers := append([]netip.AddrPort(nil), servers...)
			if strings.Contains(tt.name, "later server") {
				testServers = append(testServers, netip.MustParseAddrPort("203.0.113.3:3478"))
			}
			got, err := CheckUDPNATType(context.Background(), UDPNATOptions{
				Servers:    testServers,
				SourcePort: int(source.Port()),
				Probe:      fakeUDPProbe(tt.responses),
			})
			if err != nil {
				t.Fatalf("CheckUDPNATType returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("CheckUDPNATType = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseSTUNMappedAddressSupportsRFC3489MappedAddress(t *testing.T) {
	txid := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	response := stunResponse(txid, stunAttrMappedAddress, mappedAddressAttr(netip.MustParseAddrPort("198.51.100.7:45678")))

	addr, err := ParseSTUNMappedAddress(response, txid)
	if err != nil {
		t.Fatalf("ParseSTUNMappedAddress returned error: %v", err)
	}
	if addr != netip.MustParseAddrPort("198.51.100.7:45678") {
		t.Fatalf("mapped address = %s, want 198.51.100.7:45678", addr)
	}
}

func TestParseSTUNMappedAddressSupportsXORMappedAddress(t *testing.T) {
	txid := [16]byte{0x21, 0x12, 0xa4, 0x42, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	response := stunResponse(txid, stunAttrXORMappedAddress, xorMappedAddressAttr(netip.MustParseAddrPort("203.0.113.9:54321")))

	addr, err := ParseSTUNMappedAddress(response, txid)
	if err != nil {
		t.Fatalf("ParseSTUNMappedAddress returned error: %v", err)
	}
	if addr != netip.MustParseAddrPort("203.0.113.9:54321") {
		t.Fatalf("mapped address = %s, want 203.0.113.9:54321", addr)
	}
}

func TestParseSTUNMappedAddressRejectsWrongTransaction(t *testing.T) {
	txid := [16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	other := [16]byte{2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	response := stunResponse(other, stunAttrMappedAddress, mappedAddressAttr(netip.MustParseAddrPort("198.51.100.7:45678")))

	if _, err := ParseSTUNMappedAddress(response, txid); err == nil {
		t.Fatal("ParseSTUNMappedAddress accepted wrong transaction id")
	}
}

func TestBuildSTUNBindingRequestUsesPythonWireFormat(t *testing.T) {
	txid := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}

	msg := BuildSTUNBindingRequest(txid, false, false)

	if len(msg) != 20 {
		t.Fatalf("request length = %d, want 20", len(msg))
	}
	if got := binary.BigEndian.Uint16(msg[0:2]); got != stunBindingRequest {
		t.Fatalf("message type = %#x, want binding request", got)
	}
	if got := binary.BigEndian.Uint16(msg[2:4]); got != 0 {
		t.Fatalf("payload length = %d, want 0", got)
	}
	if got := [16]byte(msg[4:20]); got != txid {
		t.Fatalf("transaction id = %x, want %x", got, txid)
	}
}

func TestBuildSTUNBindingRequestCanAskServerToChangeAddress(t *testing.T) {
	txid := [16]byte{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}

	msg := BuildSTUNBindingRequest(txid, true, true)

	if len(msg) != 28 {
		t.Fatalf("request length = %d, want 28", len(msg))
	}
	if got := binary.BigEndian.Uint16(msg[2:4]); got != 8 {
		t.Fatalf("payload length = %d, want 8", got)
	}
	if got := [16]byte(msg[4:20]); got != txid {
		t.Fatalf("transaction id = %x, want %x", got, txid)
	}
	if got := binary.BigEndian.Uint16(msg[20:22]); got != stunAttrChangeRequest {
		t.Fatalf("attribute type = %#x, want change request", got)
	}
	if got := binary.BigEndian.Uint16(msg[22:24]); got != 4 {
		t.Fatalf("attribute length = %d, want 4", got)
	}
	if got := binary.BigEndian.Uint32(msg[24:28]); got != stunChangeIP|stunChangePort {
		t.Fatalf("change request flags = %#x, want change IP and port", got)
	}
}

func TestTCPSTUNTestReturnsSourceAndMappedAddress(t *testing.T) {
	txid := [16]byte{0x21, 0x12, 0xa4, 0x42, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	mapped := netip.MustParseAddrPort("203.0.113.20:62000")
	server, requestCh, stop := startLocalTCPCheckSTUN(t, txid, mapped)
	defer stop()

	result, err := TCPSTUNTest(context.Background(), TCPSTUNOptions{
		Server:  server,
		Source:  netip.MustParseAddrPort("127.0.0.1:0"),
		Timeout: time.Second,
		TxID:    func() ([16]byte, error) { return txid, nil },
	})
	if err != nil {
		t.Fatalf("TCPSTUNTest returned error: %v", err)
	}

	request := <-requestCh
	if got := [16]byte(request[4:20]); got != txid {
		t.Fatalf("request transaction id = %x, want %x", got, txid)
	}
	if result.Source.Addr() != netip.MustParseAddr("127.0.0.1") || result.Source.Port() == 0 {
		t.Fatalf("source address = %s, want allocated loopback port", result.Source)
	}
	if result.Mapped != mapped {
		t.Fatalf("mapped address = %s, want %s", result.Mapped, mapped)
	}
}

func TestTCPSTUNTestCanBindInterface(t *testing.T) {
	txid := [16]byte{0x21, 0x12, 0xa4, 0x42, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}
	mapped := netip.MustParseAddrPort("203.0.113.22:62002")
	server, _, stop := startLocalTCPCheckSTUN(t, txid, mapped)
	defer stop()

	result, err := TCPSTUNTest(context.Background(), TCPSTUNOptions{
		Server:    server,
		Source:    netip.MustParseAddrPort("127.0.0.1:0"),
		Interface: "lo",
		Reuse:     true,
		Timeout:   time.Second,
		TxID:      func() ([16]byte, error) { return txid, nil },
	})
	if err != nil {
		t.Fatalf("TCPSTUNTest returned error: %v", err)
	}
	if result.Mapped != mapped {
		t.Fatalf("mapped address = %s, want %s", result.Mapped, mapped)
	}
}

func TestUDPSTUNTestReturnsMappingAndResponseChangeFlags(t *testing.T) {
	txid := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	mapped := netip.MustParseAddrPort("198.51.100.30:53000")
	server, requestsCh, stop := startLocalUDPCheckSTUN(t, txid, mapped, 2)
	defer stop()

	result, err := UDPSTUNTest(context.Background(), UDPSTUNOptions{
		Server:     server,
		Source:     netip.MustParseAddrPort("127.0.0.1:0"),
		Timeout:    time.Second,
		Repeat:     2,
		ChangeIP:   true,
		ChangePort: true,
		TxID:       func() ([16]byte, error) { return txid, nil },
	})
	if err != nil {
		t.Fatalf("UDPSTUNTest returned error: %v", err)
	}

	requests := <-requestsCh
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	for _, request := range requests {
		if got := [16]byte(request[4:20]); got != txid {
			t.Fatalf("request transaction id = %x, want %x", got, txid)
		}
		if got := binary.BigEndian.Uint32(request[24:28]); got != stunChangeIP|stunChangePort {
			t.Fatalf("change request flags = %#x, want change IP and port", got)
		}
	}
	if result.Source.Addr() != netip.MustParseAddr("127.0.0.1") || result.Source.Port() == 0 {
		t.Fatalf("source address = %s, want allocated loopback port", result.Source)
	}
	if result.Mapped != mapped {
		t.Fatalf("mapped address = %s, want %s", result.Mapped, mapped)
	}
	if result.IPChanged {
		t.Fatalf("IPChanged = true, want false for loopback responder")
	}
	if !result.PortChanged {
		t.Fatalf("PortChanged = false, want true from alternate responder port")
	}
}

func TestUDPSTUNTestCanBindInterface(t *testing.T) {
	txid := [16]byte{2, 1, 0, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	mapped := netip.MustParseAddrPort("198.51.100.32:53002")
	server, _, stop := startLocalUDPCheckSTUN(t, txid, mapped, 1)
	defer stop()

	result, err := UDPSTUNTest(context.Background(), UDPSTUNOptions{
		Server:    server,
		Source:    netip.MustParseAddrPort("127.0.0.1:0"),
		Interface: "lo",
		Reuse:     true,
		Timeout:   time.Second,
		Repeat:    1,
		TxID:      func() ([16]byte, error) { return txid, nil },
	})
	if err != nil {
		t.Fatalf("UDPSTUNTest returned error: %v", err)
	}
	if result.Mapped != mapped {
		t.Fatalf("mapped address = %s, want %s", result.Mapped, mapped)
	}
}

func TestRunWithDependenciesPerformsUDPCheckFromConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var gotOptions UDPNATOptions

	err := runWithDependencies(context.Background(), config.Config{
		STUNServers: []config.STUNServer{
			{Host: "stun.example", Port: 3478},
			{Host: "198.51.100.7", Port: 1234},
		},
		KeepAliveServer: "203.0.113.10:8080",
		BindValue:       "pppoe-wan_cmcc",
		BindPort:        40000,
	}, &stdout, &stderr, Dependencies{
		Docker: DockerEnv{GOOS: "darwin"},
		Resolve: func(ctx context.Context, host string) ([]netip.Addr, error) {
			if host != "stun.example" {
				t.Fatalf("resolver host = %q, want stun.example", host)
			}
			return []netip.Addr{netip.MustParseAddr("203.0.113.9")}, nil
		},
		CheckTCP: func(context.Context, TCPCheckOptions) (NATType, error) {
			return NATUnknown, nil
		},
		CheckUDP: func(ctx context.Context, options UDPNATOptions) (NATType, error) {
			gotOptions = options
			return NATFullCone, nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "check: ok") {
		t.Fatalf("output = %q, must not report fake success", out)
	}
	if !strings.Contains(out, "[   OK   ] ... NAT Type: 1") {
		t.Fatalf("output = %q, want UDP NAT result line", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
	wantServers := []netip.AddrPort{
		netip.MustParseAddrPort("203.0.113.9:3478"),
		netip.MustParseAddrPort("198.51.100.7:1234"),
	}
	if len(gotOptions.Servers) != len(wantServers) {
		t.Fatalf("UDP servers = %#v, want %#v", gotOptions.Servers, wantServers)
	}
	for i := range wantServers {
		if gotOptions.Servers[i] != wantServers[i] {
			t.Fatalf("UDP servers = %#v, want %#v", gotOptions.Servers, wantServers)
		}
	}
	if gotOptions.SourcePort != 40000 {
		t.Fatalf("UDP source port = %d, want 40000", gotOptions.SourcePort)
	}
	if gotOptions.Interface != "pppoe-wan_cmcc" {
		t.Fatalf("UDP interface = %q, want pppoe-wan_cmcc", gotOptions.Interface)
	}
	if !gotOptions.Reuse {
		t.Fatal("UDP Reuse = false, want true")
	}
}

func TestRunWithDependenciesPerformsTCPCheckFromConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var gotOptions TCPCheckOptions

	err := runWithDependencies(context.Background(), config.Config{
		STUNServers: []config.STUNServer{
			{Host: "stun.example", Port: 3478},
			{Host: "198.51.100.8", Port: 1234},
		},
		KeepAliveServer: "keepalive.example:8080",
		BindValue:       "pppoe-wan_ct",
		BindPort:        40000,
	}, &stdout, &stderr, Dependencies{
		Docker: DockerEnv{GOOS: "darwin"},
		Resolve: func(ctx context.Context, host string) ([]netip.Addr, error) {
			switch host {
			case "stun.example":
				return []netip.Addr{netip.MustParseAddr("203.0.113.9")}, nil
			case "keepalive.example":
				return []netip.Addr{netip.MustParseAddr("203.0.113.10")}, nil
			default:
				t.Fatalf("unexpected resolver host %q", host)
				return nil, nil
			}
		},
		CheckTCP: func(ctx context.Context, options TCPCheckOptions) (NATType, error) {
			gotOptions = options
			return NATFullCone, nil
		},
		CheckUDP: func(context.Context, UDPNATOptions) (NATType, error) {
			return NATUnknown, nil
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Checking TCP NAT...") {
		t.Fatalf("output = %q, want TCP check line", out)
	}
	if !strings.Contains(out, "[   OK   ] ... NAT Type: 1") {
		t.Fatalf("output = %q, want TCP NAT result line", out)
	}
	wantServers := []netip.AddrPort{
		netip.MustParseAddrPort("203.0.113.9:3478"),
		netip.MustParseAddrPort("198.51.100.8:1234"),
	}
	if len(gotOptions.STUNServers) != len(wantServers) {
		t.Fatalf("TCP STUN servers = %#v, want %#v", gotOptions.STUNServers, wantServers)
	}
	for i := range wantServers {
		if gotOptions.STUNServers[i] != wantServers[i] {
			t.Fatalf("TCP STUN servers = %#v, want %#v", gotOptions.STUNServers, wantServers)
		}
	}
	if gotOptions.KeepAliveServer != netip.MustParseAddrPort("203.0.113.10:8080") {
		t.Fatalf("keepalive server = %s, want 203.0.113.10:8080", gotOptions.KeepAliveServer)
	}
	if gotOptions.SourceAddr != netip.IPv4Unspecified() || gotOptions.SourcePort != 40000 {
		t.Fatalf("source = %s:%d, want 0.0.0.0:40000", gotOptions.SourceAddr, gotOptions.SourcePort)
	}
	if gotOptions.Interface != "pppoe-wan_ct" {
		t.Fatalf("interface = %q, want pppoe-wan_ct", gotOptions.Interface)
	}
	if !gotOptions.Reuse {
		t.Fatal("reuse = false, want true")
	}
}

func TestRunWithDependenciesReportsUDPResolutionFailure(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := runWithDependencies(context.Background(), config.Config{
		STUNServers: []config.STUNServer{{Host: "stun.example", Port: 3478}},
	}, &stdout, &stderr, Dependencies{
		Docker: DockerEnv{GOOS: "darwin"},
		Resolve: func(context.Context, string) ([]netip.Addr, error) {
			return nil, errors.New("dns failed")
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "[  FAIL  ] ... no UDP STUN server address is available") {
		t.Fatalf("output = %q, want UDP resolution failure", out)
	}
}

func stunResponse(txid [16]byte, attrType uint16, attrValue []byte) []byte {
	response := make([]byte, 24+len(attrValue))
	binary.BigEndian.PutUint16(response[0:2], stunBindingResponse)
	binary.BigEndian.PutUint16(response[2:4], uint16(4+len(attrValue)))
	copy(response[4:20], txid[:])
	binary.BigEndian.PutUint16(response[20:22], attrType)
	binary.BigEndian.PutUint16(response[22:24], uint16(len(attrValue)))
	copy(response[24:], attrValue)
	return response
}

func mappedAddressAttr(addr netip.AddrPort) []byte {
	value := make([]byte, 8)
	value[1] = stunFamilyIPv4
	binary.BigEndian.PutUint16(value[2:4], addr.Port())
	ip := addr.Addr().As4()
	copy(value[4:8], ip[:])
	return value
}

func xorMappedAddressAttr(addr netip.AddrPort) []byte {
	value := mappedAddressAttr(addr)
	binary.BigEndian.PutUint16(value[2:4], addr.Port()^uint16(stunMagicCookie>>16))
	ip := addr.Addr().As4()
	cookie := [4]byte{}
	binary.BigEndian.PutUint32(cookie[:], stunMagicCookie)
	for i := range ip {
		value[4+i] = ip[i] ^ cookie[i]
	}
	return value
}

func startLocalTCPCheckSTUN(t *testing.T, txid [16]byte, mapped netip.AddrPort) (netip.AddrPort, <-chan []byte, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen tcp: %v", err)
	}
	requestCh := make(chan []byte, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		request := make([]byte, 20)
		if _, err := io.ReadFull(conn, request); err != nil {
			return
		}
		requestCh <- request
		_, _ = conn.Write(stunResponse(txid, stunAttrXORMappedAddress, xorMappedAddressAttr(mapped)))
	}()

	addr := listener.Addr().(*net.TCPAddr)
	server := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(addr.Port))
	return server, requestCh, func() {
		_ = listener.Close()
		<-done
	}
}

func startLocalCheckKeepAlive(t *testing.T) (netip.AddrPort, <-chan string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen keepalive tcp: %v", err)
	}
	requestCh := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 512)
		n, _ := conn.Read(buf)
		requestCh <- string(buf[:n])
		_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nConnection: keep-alive\r\n\r\n"))
		time.Sleep(50 * time.Millisecond)
	}()

	addr := listener.Addr().(*net.TCPAddr)
	server := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(addr.Port))
	return server, requestCh, func() {
		_ = listener.Close()
		<-done
	}
}

func startLocalUDPCheckSTUN(t *testing.T, txid [16]byte, mapped netip.AddrPort, repeat int) (netip.AddrPort, <-chan [][]byte, func()) {
	t.Helper()
	serverConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp server: %v", err)
	}
	responseConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		_ = serverConn.Close()
		t.Fatalf("listen udp responder: %v", err)
	}
	requestsCh := make(chan [][]byte, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		requests := make([][]byte, 0, repeat)
		var client net.Addr
		for len(requests) < repeat {
			buf := make([]byte, 1500)
			n, addr, err := serverConn.ReadFrom(buf)
			if err != nil {
				return
			}
			client = addr
			requests = append(requests, append([]byte(nil), buf[:n]...))
		}
		requestsCh <- requests
		_, _ = responseConn.WriteTo(stunResponse(txid, stunAttrMappedAddress, mappedAddressAttr(mapped)), client)
	}()

	addr := serverConn.LocalAddr().(*net.UDPAddr)
	server := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(addr.Port))
	return server, requestsCh, func() {
		_ = serverConn.Close()
		_ = responseConn.Close()
		<-done
	}
}

type udpProbeKey struct {
	server     netip.AddrPort
	changeIP   bool
	changePort bool
}

type udpProbeResponse struct {
	result STUNTestResult
	err    error
}

func fakeUDPProbe(responses map[udpProbeKey]udpProbeResponse) UDPProbe {
	return func(ctx context.Context, request UDPProbeRequest) (STUNTestResult, error) {
		if err := ctx.Err(); err != nil {
			return STUNTestResult{}, err
		}
		response, ok := responses[udpProbeKey{
			server:     request.Server,
			changeIP:   request.ChangeIP,
			changePort: request.ChangePort,
		}]
		if !ok {
			return STUNTestResult{}, errors.New("no response")
		}
		if response.err != nil {
			return STUNTestResult{}, response.err
		}
		return response.result, nil
	}
}

type tcpProbeResponse struct {
	result STUNTestResult
	err    error
}

func fakeTCPProbe(responses map[netip.AddrPort]tcpProbeResponse, calls *[]TCPProbeRequest) TCPProbe {
	return func(ctx context.Context, request TCPProbeRequest) (STUNTestResult, error) {
		if err := ctx.Err(); err != nil {
			return STUNTestResult{}, err
		}
		if calls != nil {
			*calls = append(*calls, request)
		}
		response, ok := responses[request.Server]
		if !ok {
			return STUNTestResult{}, errors.New("no response")
		}
		if response.err != nil {
			return STUNTestResult{}, response.err
		}
		return response.result, nil
	}
}

func fixedPortProbe(result portcheck.Result, err error) portcheck.Probe {
	return func(context.Context, int, netip.Addr) (portcheck.Result, error) {
		return result, err
	}
}

type noopCloser struct{}

func (noopCloser) Close() error {
	return nil
}

type trackingCloser struct {
	closed bool
}

func (c *trackingCloser) Close() error {
	c.closed = true
	return nil
}

func TestCheckDockerNetworkRejectsDockerBridgeNetwork(t *testing.T) {
	root := t.TempDir()
	dockerEnv := filepath.Join(root, ".dockerenv")
	eth0MAC := filepath.Join(root, "eth0", "address")
	if err := os.WriteFile(dockerEnv, nil, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(eth0MAC), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(eth0MAC, []byte("02:42:ac:11:00:02\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	err := CheckDockerNetwork(DockerEnv{
		GOOS:          "linux",
		DockerEnvPath: dockerEnv,
		Eth0MACPath:   eth0MAC,
		Hostname:      func() (string, error) { return "container", nil },
		LookupIPv4:    func(string) (string, error) { return "172.17.0.2", nil },
	})
	if err == nil {
		t.Fatal("CheckDockerNetwork returned nil, want Docker host network error")
	}
	if !strings.Contains(err.Error(), "Docker's `--net=host` option is required") {
		t.Fatalf("error = %v, want --net=host message", err)
	}
}

func TestCheckDockerNetworkSkipsWhenNotDocker(t *testing.T) {
	called := false
	err := CheckDockerNetwork(DockerEnv{
		GOOS:          "linux",
		DockerEnvPath: filepath.Join(t.TempDir(), ".dockerenv"),
		Hostname: func() (string, error) {
			called = true
			return "container", nil
		},
	})
	if err != nil {
		t.Fatalf("CheckDockerNetwork returned error: %v", err)
	}
	if called {
		t.Fatal("CheckDockerNetwork resolved hostname outside Docker")
	}
}

func TestRunnerStopsBeforeReportOnDockerBridgeNetwork(t *testing.T) {
	root := t.TempDir()
	dockerEnv := filepath.Join(root, ".dockerenv")
	eth0MAC := filepath.Join(root, "eth0", "address")
	if err := os.WriteFile(dockerEnv, nil, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(eth0MAC), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(eth0MAC, []byte("02:42:ac:11:00:02\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	var stdout bytes.Buffer
	runner := Runner{
		Docker: DockerEnv{
			GOOS:          "linux",
			DockerEnvPath: dockerEnv,
			Eth0MACPath:   eth0MAC,
			Hostname:      func() (string, error) { return "container", nil },
			LookupIPv4:    func(string) (string, error) { return "172.17.0.2", nil },
		},
	}
	err := runner.Run(context.Background(), config.Config{}, &stdout)
	if err == nil {
		t.Fatal("Run returned nil, want Docker host network error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no report before Docker error", stdout.String())
	}
}
