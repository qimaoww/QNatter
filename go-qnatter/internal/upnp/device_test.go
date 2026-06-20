package upnp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoadDeviceDescriptionSelectsForwardService(t *testing.T) {
	var gotUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserAgent = r.Header.Get("User-Agent")
		fmt.Fprint(w, `<?xml version="1.0"?>
<root>
  <device>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:WANCommonIFC1</serviceId>
        <controlURL>/upnp/control/WANCommonIFC1</controlURL>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:WANIPConn1</serviceId>
        <controlURL>/upnp/control/WANIPConn1</controlURL>
      </service>
    </serviceList>
  </device>
</root>`)
	}))
	defer server.Close()

	device, err := LoadDeviceDescription(context.Background(), server.Client(), "192.168.1.1", server.URL+"/root.xml")
	if err != nil {
		t.Fatalf("LoadDeviceDescription returned error: %v", err)
	}
	if gotUserAgent != userAgent {
		t.Fatalf("User-Agent = %q, want %q", gotUserAgent, userAgent)
	}
	if device.IP != "192.168.1.1" {
		t.Fatalf("device IP = %q", device.IP)
	}
	if len(device.Services) != 2 {
		t.Fatalf("service count = %d, want 2", len(device.Services))
	}
	if device.ForwardService == nil {
		t.Fatalf("ForwardService is nil")
	}
	if device.ForwardService.ControlURL != server.URL+"/upnp/control/WANIPConn1" {
		t.Fatalf("forward control URL = %q", device.ForwardService.ControlURL)
	}
}

func TestLoadDeviceDescriptionRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := LoadDeviceDescription(context.Background(), server.Client(), "192.168.1.1", server.URL+"/root.xml")
	if err == nil {
		t.Fatalf("LoadDeviceDescription returned nil error")
	}
}
