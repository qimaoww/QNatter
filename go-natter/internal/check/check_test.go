package check

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/config"
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

func TestDefaultRunReportsUnimplementedChecksWithoutFakeSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer

	err := Run(context.Background(), config.Config{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "check: ok") {
		t.Fatalf("output = %q, must not report fake success", out)
	}
	if !strings.Contains(out, "[  FAIL  ] ... Go TCP NAT check is not implemented yet") {
		t.Fatalf("output = %q, want TCP not implemented line", out)
	}
	if !strings.Contains(out, "[  FAIL  ] ... Go UDP NAT check is not implemented yet") {
		t.Fatalf("output = %q, want UDP not implemented line", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
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
