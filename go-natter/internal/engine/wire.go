package engine

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/keepalive"
	"natter-openwrt/go-natter/internal/stun"
)

type Bind struct {
	Source    netip.AddrPort
	Interface string
}

func BindFromConfig(cfg config.Config) (Bind, error) {
	value := cfg.BindValue
	if value == "" {
		value = "0.0.0.0"
	}

	if addr, err := netip.ParseAddr(value); err == nil {
		return Bind{Source: netip.AddrPortFrom(addr, uint16(cfg.BindPort))}, nil
	}

	return Bind{
		Source:    netip.AddrPortFrom(netip.MustParseAddr("0.0.0.0"), uint16(cfg.BindPort)),
		Interface: value,
	}, nil
}

func NewSTUNClientFromConfig(cfg config.Config) (*stun.Client, error) {
	bind, err := BindFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	servers := make([]stun.Server, 0, len(cfg.STUNServers))
	for _, server := range cfg.STUNServers {
		servers = append(servers, stun.Server{Host: server.Host, Port: server.Port})
	}
	return &stun.Client{
		Servers: servers,
		Source:  bind.Source,
		UDP:     cfg.UDP,
		Transport: stun.NetworkTransport{
			Interface: bind.Interface,
			Reuse:     true,
		},
	}, nil
}

func NewKeepAliveFromConfig(cfg config.Config, mapping stun.Mapping) (KeepAlive, error) {
	bind, err := BindFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	defaultPort := 80
	if cfg.UDP {
		defaultPort = 53
	}
	host, port, err := splitHostPortDefault(cfg.KeepAliveServer, defaultPort)
	if err != nil {
		return nil, err
	}
	if cfg.UDP {
		return &keepalive.UDPClient{
			Host:      host,
			Port:      port,
			Source:    mapping.Inner,
			Interface: bind.Interface,
		}, nil
	}
	return &keepalive.TCPClient{
		Host:      host,
		Port:      port,
		Source:    mapping.Inner,
		Interface: bind.Interface,
	}, nil
}

func splitHostPortDefault(value string, defaultPort int) (string, int, error) {
	host := value
	port := defaultPort
	if host == "" {
		return "", 0, fmt.Errorf("empty host")
	}
	if strings.HasPrefix(value, "[") {
		end := strings.LastIndex(value, "]")
		if end < 0 {
			return "", 0, fmt.Errorf("empty host")
		}
		host = value[1:end]
		if rest := value[end+1:]; rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return "", 0, fmt.Errorf("invalid port in %q", value)
			}
			parsed, err := parsePort(rest[1:])
			if err != nil {
				return "", 0, fmt.Errorf("invalid port in %q", value)
			}
			port = parsed
		}
	} else if strings.Count(value, ":") == 1 {
		idx := strings.LastIndex(value, ":")
		host = value[:idx]
		parsed, err := parsePort(value[idx+1:])
		if err != nil {
			return "", 0, fmt.Errorf("invalid port in %q", value)
		}
		port = parsed
	}
	if host == "" {
		return "", 0, fmt.Errorf("empty host")
	}
	return host, port, nil
}

func parsePort(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("invalid port")
	}
	return parsed, nil
}
