package check

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"

	"natter-openwrt/go-natter/internal/config"
)

const Version = "2.2.1-go"

const (
	stunBindingResponse      uint16 = 0x0101
	stunAttrMappedAddress    uint16 = 0x0001
	stunAttrXORMappedAddress uint16 = 0x0020
	stunFamilyIPv4           byte   = 0x01
	stunMagicCookie          uint32 = 0x2112a442
)

type Status int

const (
	NA Status = iota
	OK
	COMPAT
	FAIL
)

type Result struct {
	Status Status
	Info   string
}

type NATType int

const (
	NATUnknown              NATType = -1
	NATOpenInternet         NATType = 0
	NATFullCone             NATType = 1
	NATRestricted           NATType = 2
	NATPortRestricted       NATType = 3
	NATSymmetric            NATType = 4
	NATSymmetricUDPFirewall NATType = 5
)

type Probe func(context.Context, config.Config) (Result, error)

type Runner struct {
	Docker DockerEnv
	TCP    Probe
	UDP    Probe
}

type DockerEnv struct {
	GOOS          string
	DockerEnvPath string
	Eth0MACPath   string
	Hostname      func() (string, error)
	LookupIPv4    func(string) (string, error)
}

func Run(ctx context.Context, cfg config.Config, stdout io.Writer, stderr io.Writer) error {
	return Runner{
		TCP: unimplementedTCP,
		UDP: unimplementedUDP,
	}.Run(ctx, cfg, stdout)
}

func (r Runner) Run(ctx context.Context, cfg config.Config, stdout io.Writer) error {
	if err := CheckDockerNetwork(r.Docker); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "> NatterCheck v%s\n\n", Version)
	r.printInfo(ctx, cfg, stdout, "Checking TCP NAT...", r.TCP)
	r.printInfo(ctx, cfg, stdout, "Checking UDP NAT...", r.UDP)
	return nil
}

func (r Runner) printInfo(ctx context.Context, cfg config.Config, stdout io.Writer, label string, probe Probe) {
	fmt.Fprintf(stdout, "%-36s ", label)
	result, err := probe(ctx, cfg)
	if err != nil {
		result = Result{Status: FAIL, Info: err.Error()}
	}
	fmt.Fprintf(stdout, "%s ... %s\n", result.Status.rep(), result.Info)
}

func (s Status) rep() string {
	switch s {
	case NA:
		return "[   NA   ]"
	case OK:
		return "[   OK   ]"
	case COMPAT:
		return "[ COMPAT ]"
	case FAIL:
		return "[  FAIL  ]"
	default:
		return "[  FAIL  ]"
	}
}

func ResultFromNATType(nat NATType) Result {
	status := FAIL
	switch nat {
	case NATOpenInternet, NATFullCone:
		status = OK
	case NATUnknown:
		status = NA
	}
	return Result{Status: status, Info: fmt.Sprintf("NAT Type: %d", nat)}
}

func ParseSTUNMappedAddress(data []byte, txid [16]byte) (netip.AddrPort, error) {
	if len(data) < 20 {
		return netip.AddrPort{}, errors.New("short STUN response")
	}
	if binary.BigEndian.Uint16(data[0:2]) != stunBindingResponse {
		return netip.AddrPort{}, errors.New("not a STUN binding response")
	}
	msgLen := int(binary.BigEndian.Uint16(data[2:4]))
	if len(data) < 20+msgLen {
		return netip.AddrPort{}, errors.New("truncated STUN response")
	}
	if string(data[4:20]) != string(txid[:]) {
		return netip.AddrPort{}, errors.New("STUN transaction id mismatch")
	}

	payload := data[20 : 20+msgLen]
	for len(payload) >= 4 {
		attrType := binary.BigEndian.Uint16(payload[0:2])
		attrLen := int(binary.BigEndian.Uint16(payload[2:4]))
		if len(payload) < 4+attrLen {
			return netip.AddrPort{}, errors.New("truncated STUN attribute")
		}
		if attrType == stunAttrMappedAddress || attrType == stunAttrXORMappedAddress {
			return parseSTUNAddressAttribute(attrType, payload[4:4+attrLen])
		}
		padded := (attrLen + 3) &^ 3
		if len(payload) < 4+padded {
			return netip.AddrPort{}, errors.New("truncated STUN attribute padding")
		}
		payload = payload[4+padded:]
	}
	return netip.AddrPort{}, errors.New("mapped address attribute not found")
}

func parseSTUNAddressAttribute(attrType uint16, value []byte) (netip.AddrPort, error) {
	if len(value) < 8 {
		return netip.AddrPort{}, errors.New("short STUN address attribute")
	}
	if value[1] != stunFamilyIPv4 {
		return netip.AddrPort{}, fmt.Errorf("unsupported STUN address family %d", value[1])
	}
	port := binary.BigEndian.Uint16(value[2:4])
	ip := [4]byte{}
	copy(ip[:], value[4:8])
	if attrType == stunAttrXORMappedAddress {
		port ^= uint16(stunMagicCookie >> 16)
		cookie := [4]byte{}
		binary.BigEndian.PutUint32(cookie[:], stunMagicCookie)
		for i := range ip {
			ip[i] ^= cookie[i]
		}
	}
	return netip.AddrPortFrom(netip.AddrFrom4(ip), port), nil
}

func unimplementedTCP(context.Context, config.Config) (Result, error) {
	return Result{}, fmt.Errorf("Go TCP NAT check is not implemented yet")
}

func unimplementedUDP(context.Context, config.Config) (Result, error) {
	return Result{}, fmt.Errorf("Go UDP NAT check is not implemented yet")
}

func CheckDockerNetwork(env DockerEnv) error {
	env = env.withDefaults()
	if env.GOOS != "linux" {
		return nil
	}
	if !fileExists(env.DockerEnvPath) || !fileExists(env.Eth0MACPath) {
		return nil
	}
	rawMAC, err := os.ReadFile(env.Eth0MACPath)
	if err != nil {
		return nil
	}
	hostname, err := env.Hostname()
	if err != nil {
		return nil
	}
	ip, err := env.LookupIPv4(hostname)
	if err != nil {
		return nil
	}
	dockerMAC, err := dockerMACForIPv4(ip)
	if err != nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(string(rawMAC)), dockerMAC) {
		return fmt.Errorf("Docker's `--net=host` option is required")
	}
	return nil
}

func (env DockerEnv) withDefaults() DockerEnv {
	if env.GOOS == "" {
		env.GOOS = runtime.GOOS
	}
	if env.DockerEnvPath == "" {
		env.DockerEnvPath = "/.dockerenv"
	}
	if env.Eth0MACPath == "" {
		env.Eth0MACPath = "/sys/class/net/eth0/address"
	}
	if env.Hostname == nil {
		env.Hostname = os.Hostname
	}
	if env.LookupIPv4 == nil {
		env.LookupIPv4 = lookupIPv4
	}
	return env
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func lookupIPv4(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return net.IP(v4).String(), nil
		}
	}
	return "", fmt.Errorf("no IPv4 address for %s", host)
}

func dockerMACForIPv4(ip string) (string, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return "", fmt.Errorf("invalid IPv4 address: %s", ip)
	}
	raw := addr.As4()
	return fmt.Sprintf("02:42:%02x:%02x:%02x:%02x", raw[0], raw[1], raw[2], raw[3]), nil
}
