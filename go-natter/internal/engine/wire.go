package engine

import (
	"net/netip"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/endpoint"
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
	host, port, err := endpoint.SplitHostPortDefault(cfg.KeepAliveServer, defaultPort)
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
