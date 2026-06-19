package upnp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseServicesFindsForwardServiceAndResolvesURLs(t *testing.T) {
	description := []byte(`<?xml version="1.0"?>
<root>
  <device>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:WANIPConnection:2</serviceType>
        <serviceId>urn:upnp-org:serviceId:WANIPConn1</serviceId>
        <SCPDURL>/igd/ip.xml</SCPDURL>
        <controlURL>/igd/control/WANIPConn</controlURL>
        <eventSubURL>/igd/event/WANIPConn</eventSubURL>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1</serviceType>
        <serviceId>urn:upnp-org:serviceId:WANCommonIFC1</serviceId>
        <controlURL>/igd/control/WANCommonIFC1</controlURL>
      </service>
    </serviceList>
  </device>
</root>`)

	services, err := ParseServices(description, "http://192.168.1.1:5000/rootDesc.xml")
	if err != nil {
		t.Fatalf("ParseServices returned error: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("service count = %d, want 2", len(services))
	}

	forward := services[0]
	if !forward.IsForward() {
		t.Fatalf("first service IsForward = false")
	}
	if forward.ControlURL != "http://192.168.1.1:5000/igd/control/WANIPConn" {
		t.Fatalf("control URL = %q", forward.ControlURL)
	}
	if forward.SCPDURL != "http://192.168.1.1:5000/igd/ip.xml" {
		t.Fatalf("SCPD URL = %q", forward.SCPDURL)
	}
	if forward.EventSubURL != "http://192.168.1.1:5000/igd/event/WANIPConn" {
		t.Fatalf("event URL = %q", forward.EventSubURL)
	}
	if services[1].IsForward() {
		t.Fatalf("WANCommonInterfaceConfig must not be treated as forward-capable")
	}
}

func TestAddPortMappingRequestMatchesIGDSoap(t *testing.T) {
	service := Service{
		ServiceType: "urn:schemas-upnp-org:service:WANPPPConnection:1",
		ServiceID:   "urn:upnp-org:serviceId:WANPPPConn1",
		ControlURL:  "http://192.168.1.1:5000/upnp/control/WANPPPConn1",
	}

	req, err := service.NewAddPortMappingRequest(PortMapping{
		RemoteHost:     "",
		ExternalPort:   62000,
		Protocol:       "udp",
		InternalPort:   51413,
		InternalClient: "10.10.10.9",
		Description:    "Natter",
		LeaseDuration:  45,
	})
	if err != nil {
		t.Fatalf("NewAddPortMappingRequest returned error: %v", err)
	}

	if req.Method != "POST" {
		t.Fatalf("method = %s, want POST", req.Method)
	}
	if req.URL.String() != service.ControlURL {
		t.Fatalf("URL = %s, want %s", req.URL.String(), service.ControlURL)
	}
	if got := req.Header.Get("SOAPAction"); got != `"urn:schemas-upnp-org:service:WANPPPConnection:1#AddPortMapping"` {
		t.Fatalf("SOAPAction = %q", got)
	}
	if got := req.Header.Get("User-Agent"); got != "curl/8.0.0 (Natter)" {
		t.Fatalf("User-Agent = %q", got)
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	body := string(bodyBytes)
	for _, want := range []string{
		"<m:AddPortMapping xmlns:m=\"urn:schemas-upnp-org:service:WANPPPConnection:1\">",
		"<NewRemoteHost></NewRemoteHost>",
		"<NewExternalPort>62000</NewExternalPort>",
		"<NewProtocol>UDP</NewProtocol>",
		"<NewInternalPort>51413</NewInternalPort>",
		"<NewInternalClient>10.10.10.9</NewInternalClient>",
		"<NewPortMappingDescription>Natter</NewPortMappingDescription>",
		"<NewLeaseDuration>45</NewLeaseDuration>",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("request body does not contain %q:\n%s", want, body)
		}
	}
}

func TestAddPortMappingRequestRejectsUnsupportedService(t *testing.T) {
	service := Service{
		ServiceType: "urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1",
		ControlURL:  "http://192.168.1.1/control",
	}
	if _, err := service.NewAddPortMappingRequest(PortMapping{
		ExternalPort:   62000,
		Protocol:       "tcp",
		InternalPort:   62000,
		InternalClient: "10.10.10.9",
	}); err == nil {
		t.Fatalf("NewAddPortMappingRequest returned nil error")
	}
}

func TestAddPortMappingSendsSOAPRequest(t *testing.T) {
	var gotSOAPAction string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSOAPAction = r.Header.Get("SOAPAction")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll returned error: %v", err)
		}
		gotBody = string(body)
		fmt.Fprint(w, `<?xml version="1.0"?><s:Envelope><s:Body></s:Body></s:Envelope>`)
	}))
	defer server.Close()

	service := Service{
		ServiceType: "urn:schemas-upnp-org:service:WANIPConnection:1",
		ServiceID:   "urn:upnp-org:serviceId:WANIPConn1",
		ControlURL:  server.URL + "/upnp/control/WANIPConn1",
	}

	err := service.AddPortMapping(context.Background(), server.Client(), PortMapping{
		ExternalPort:   62000,
		Protocol:       "tcp",
		InternalPort:   51413,
		InternalClient: "10.10.10.9",
		LeaseDuration:  45,
	})
	if err != nil {
		t.Fatalf("AddPortMapping returned error: %v", err)
	}
	if gotSOAPAction != `"urn:schemas-upnp-org:service:WANIPConnection:1#AddPortMapping"` {
		t.Fatalf("SOAPAction = %q", gotSOAPAction)
	}
	if !strings.Contains(gotBody, "<NewProtocol>TCP</NewProtocol>") {
		t.Fatalf("body missing TCP protocol:\n%s", gotBody)
	}
	if !strings.Contains(gotBody, "<NewPortMappingDescription>Natter</NewPortMappingDescription>") {
		t.Fatalf("body missing default description:\n%s", gotBody)
	}
}

func TestAddPortMappingReturnsIGDErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<s:Envelope><s:Body><s:Fault><detail><UPnPError>
<errorCode>718</errorCode>
<errorDescription>ConflictInMappingEntry</errorDescription>
</UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
	}))
	defer server.Close()

	service := Service{
		ServiceType: "urn:schemas-upnp-org:service:WANIPConnection:1",
		ServiceID:   "urn:upnp-org:serviceId:WANIPConn1",
		ControlURL:  server.URL + "/upnp/control/WANIPConn1",
	}

	err := service.AddPortMapping(context.Background(), server.Client(), PortMapping{
		ExternalPort:   62000,
		Protocol:       "tcp",
		InternalPort:   51413,
		InternalClient: "10.10.10.9",
	})
	if err == nil {
		t.Fatalf("AddPortMapping returned nil error")
	}
	if !strings.Contains(err.Error(), "718") || !strings.Contains(err.Error(), "ConflictInMappingEntry") {
		t.Fatalf("error = %v", err)
	}
}
