package engine

import (
	"net/netip"
	"testing"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/keepalive"
	"natter-openwrt/go-natter/internal/stun"
)

func TestBindFromConfigSeparatesIPAndInterface(t *testing.T) {
	ipBind, err := BindFromConfig(config.Config{
		BindValue: "10.10.10.3",
		BindPort:  40000,
	})
	if err != nil {
		t.Fatalf("BindFromConfig returned error: %v", err)
	}
	if ipBind.Source != netip.MustParseAddrPort("10.10.10.3:40000") || ipBind.Interface != "" {
		t.Fatalf("ip bind = %+v, want source IP without interface", ipBind)
	}

	ifaceBind, err := BindFromConfig(config.Config{
		BindValue: "pppoe-wan_cmcc",
		BindPort:  0,
	})
	if err != nil {
		t.Fatalf("BindFromConfig returned error: %v", err)
	}
	if ifaceBind.Source != netip.MustParseAddrPort("0.0.0.0:0") || ifaceBind.Interface != "pppoe-wan_cmcc" {
		t.Fatalf("interface bind = %+v, want wildcard source with interface", ifaceBind)
	}
}

func TestNewSTUNClientFromConfigUsesBindAndServers(t *testing.T) {
	client, err := NewSTUNClientFromConfig(config.Config{
		UDP:       true,
		BindValue: "pppoe-wan_cmcc",
		BindPort:  50000,
		STUNServers: []config.STUNServer{
			{Host: "stun.example", Port: 3478},
		},
	})
	if err != nil {
		t.Fatalf("NewSTUNClientFromConfig returned error: %v", err)
	}

	if client.Source != netip.MustParseAddrPort("0.0.0.0:50000") {
		t.Fatalf("client source = %s, want 0.0.0.0:50000", client.Source)
	}
	if !client.UDP {
		t.Fatal("client UDP = false, want true")
	}
	if len(client.Servers) != 1 || client.Servers[0] != (stun.Server{Host: "stun.example", Port: 3478}) {
		t.Fatalf("client servers = %#v", client.Servers)
	}
	if client.Transport.Interface != "pppoe-wan_cmcc" || !client.Transport.Reuse {
		t.Fatalf("client transport = %+v, want interface bind with reuse", client.Transport)
	}
}

func TestNewKeepAliveFromConfigUsesMappingSourceAndDefaultPorts(t *testing.T) {
	tcpClient, err := NewKeepAliveFromConfig(config.Config{
		KeepAliveServer: "keepalive.example",
		BindValue:       "pppoe-wan_ct",
	}, stun.Mapping{
		Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
	})
	if err != nil {
		t.Fatalf("NewKeepAliveFromConfig returned error: %v", err)
	}
	tcp, ok := tcpClient.(*keepalive.TCPClient)
	if !ok {
		t.Fatalf("tcp keepalive = %T, want *keepalive.TCPClient", tcpClient)
	}
	if tcp.Host != "keepalive.example" || tcp.Port != 80 {
		t.Fatalf("tcp endpoint = %s:%d, want keepalive.example:80", tcp.Host, tcp.Port)
	}
	if tcp.Source != netip.MustParseAddrPort("10.10.10.3:41000") || tcp.Interface != "pppoe-wan_ct" {
		t.Fatalf("tcp bind = %s/%q", tcp.Source, tcp.Interface)
	}

	udpClient, err := NewKeepAliveFromConfig(config.Config{
		UDP:             true,
		KeepAliveServer: "119.29.29.29:5353",
	}, stun.Mapping{
		Inner: netip.MustParseAddrPort("10.0.0.2:42000"),
	})
	if err != nil {
		t.Fatalf("NewKeepAliveFromConfig returned error: %v", err)
	}
	udp, ok := udpClient.(*keepalive.UDPClient)
	if !ok {
		t.Fatalf("udp keepalive = %T, want *keepalive.UDPClient", udpClient)
	}
	if udp.Host != "119.29.29.29" || udp.Port != 5353 {
		t.Fatalf("udp endpoint = %s:%d, want 119.29.29.29:5353", udp.Host, udp.Port)
	}
	if udp.Source != netip.MustParseAddrPort("10.0.0.2:42000") {
		t.Fatalf("udp source = %s, want mapping inner", udp.Source)
	}
}
