package upnp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDiscoverRouterSelectsFirstForwardCapableDevice(t *testing.T) {
	noForward := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?>
<root><device><serviceList>
  <service>
    <serviceType>urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1</serviceType>
    <serviceId>urn:upnp-org:serviceId:WANCommonIFC1</serviceId>
    <controlURL>/upnp/control/WANCommonIFC1</controlURL>
  </service>
</serviceList></device></root>`)
	}))
	defer noForward.Close()

	forward := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?>
<root><device><serviceList>
  <service>
    <serviceType>urn:schemas-upnp-org:service:WANPPPConnection:1</serviceType>
    <serviceId>urn:upnp-org:serviceId:WANPPPConn1</serviceId>
    <controlURL>/upnp/control/WANPPPConn1</controlURL>
  </service>
</serviceList></device></root>`)
	}))
	defer forward.Close()

	client := Client{
		HTTPClient: forward.Client(),
		DiscoverLocations: func(ctx context.Context) ([]Location, error) {
			return []Location{
				{IP: "192.168.1.1", URL: noForward.URL + "/root.xml"},
				{IP: "192.168.2.1", URL: forward.URL + "/root.xml"},
			}, nil
		},
	}

	device, err := client.DiscoverRouter(context.Background())
	if err != nil {
		t.Fatalf("DiscoverRouter returned error: %v", err)
	}
	if device == nil {
		t.Fatalf("DiscoverRouter returned nil device")
	}
	if device.IP != "192.168.2.1" {
		t.Fatalf("device IP = %q, want 192.168.2.1", device.IP)
	}
	if device.ForwardService == nil || device.ForwardService.ServiceType != "urn:schemas-upnp-org:service:WANPPPConnection:1" {
		t.Fatalf("forward service = %#v", device.ForwardService)
	}
}

func TestClientDiscoverRouterReportsMissingForwardService(t *testing.T) {
	noForward := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?>
<root><device><serviceList>
  <service>
    <serviceType>urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1</serviceType>
    <serviceId>urn:upnp-org:serviceId:WANCommonIFC1</serviceId>
    <controlURL>/upnp/control/WANCommonIFC1</controlURL>
  </service>
</serviceList></device></root>`)
	}))
	defer noForward.Close()

	client := Client{
		HTTPClient: noForward.Client(),
		DiscoverLocations: func(ctx context.Context) ([]Location, error) {
			return []Location{{IP: "192.168.1.1", URL: noForward.URL + "/root.xml"}}, nil
		},
	}

	// N-P2-9：没有可用的转发服务时必须返回明确错误，而不是 (nil, nil)，
	// 否则调用方会把"没有映射"当成"映射成功"。
	device, err := client.DiscoverRouter(context.Background())
	if !errors.Is(err, ErrNoForwardService) {
		t.Fatalf("DiscoverRouter error = %v, want ErrNoForwardService", err)
	}
	if device != nil {
		t.Fatalf("DiscoverRouter returned %#v, want nil", device)
	}
}
