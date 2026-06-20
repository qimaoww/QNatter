package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/keepalive"
	"natter-openwrt/go-natter/internal/stun"
	"natter-openwrt/go-natter/internal/upnp"
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

func TestNewUPnPMapperFromConfigUsesBindInterface(t *testing.T) {
	mapper, err := NewUPnPMapperFromConfig(config.Config{
		BindValue: "pppoe-wan_cmcc",
	})
	if err != nil {
		t.Fatalf("NewUPnPMapperFromConfig returned error: %v", err)
	}
	client, ok := mapper.(*UPnPClient)
	if !ok {
		t.Fatalf("mapper = %T, want *UPnPClient", mapper)
	}
	if client.Client.Interface != "pppoe-wan_cmcc" {
		t.Fatalf("UPnP interface = %q, want pppoe-wan_cmcc", client.Client.Interface)
	}
	if client.Client.Timeout <= 0 {
		t.Fatalf("UPnP timeout = %s, want positive", client.Client.Timeout)
	}
}

func TestUPnPClientForwardDiscoversAddsAndRenewsMapping(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("SOAPAction") != `"urn:schemas-upnp-org:service:WANIPConnection:1#AddPortMapping"` {
			t.Fatalf("SOAPAction = %q", r.Header.Get("SOAPAction"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		bodies = append(bodies, string(body))
		fmt.Fprint(w, `<s:Envelope><s:Body></s:Body></s:Envelope>`)
	}))
	defer server.Close()

	mapper := &UPnPClient{
		Client: upnp.Client{
			Interface:  "pppoe-wan_cmcc",
			HTTPClient: server.Client(),
		},
		DiscoverRouter: func(ctx context.Context, client upnp.Client) (*upnp.Device, error) {
			if client.BindIP != "10.10.10.3" {
				t.Fatalf("discovery BindIP = %q, want 10.10.10.3", client.BindIP)
			}
			if client.Interface != "pppoe-wan_cmcc" {
				t.Fatalf("discovery interface = %q, want pppoe-wan_cmcc", client.Interface)
			}
			return &upnp.Device{
				IP: "192.168.1.1",
				ForwardService: &upnp.Service{
					ServiceType: "urn:schemas-upnp-org:service:WANIPConnection:1",
					ServiceID:   "urn:upnp-org:serviceId:WANIPConn1",
					ControlURL:  server.URL + "/control",
				},
			}, nil
		},
	}
	var foundRouter string
	mapper.OnFoundRouter = func(ip string) {
		foundRouter = ip
	}

	err := mapper.Forward(context.Background(), UPnPMapping{
		ExternalPort:   51413,
		InternalPort:   51413,
		InternalClient: "10.10.10.3",
		UDP:            true,
		LeaseDuration:  45,
	})
	if err != nil {
		t.Fatalf("Forward returned error: %v", err)
	}
	if foundRouter != "192.168.1.1" {
		t.Fatalf("found router = %q, want 192.168.1.1", foundRouter)
	}
	if err := mapper.Renew(context.Background()); err != nil {
		t.Fatalf("Renew returned error: %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("request count = %d, want 2", len(bodies))
	}
	for _, body := range bodies {
		for _, want := range []string{
			"<NewExternalPort>51413</NewExternalPort>",
			"<NewProtocol>UDP</NewProtocol>",
			"<NewInternalPort>51413</NewInternalPort>",
			"<NewInternalClient>10.10.10.3</NewInternalClient>",
			"<NewLeaseDuration>45</NewLeaseDuration>",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("body missing %q:\n%s", want, body)
			}
		}
	}
}
