package upnp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const userAgent = "curl/8.0.0 (Natter)"

var forwardServiceTypes = map[string]bool{
	"urn:schemas-upnp-org:service:WANIPConnection:1":  true,
	"urn:schemas-upnp-org:service:WANIPConnection:2":  true,
	"urn:schemas-upnp-org:service:WANPPPConnection:1": true,
}

type Service struct {
	ServiceType string
	ServiceID   string
	SCPDURL     string
	ControlURL  string
	EventSubURL string
}

type PortMapping struct {
	RemoteHost     string
	ExternalPort   int
	Protocol       string
	InternalPort   int
	InternalClient string
	Description    string
	LeaseDuration  int
}

type serviceXML struct {
	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	SCPDURL     string `xml:"SCPDURL"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
}

func ParseServices(description []byte, baseURL string) ([]Service, error) {
	decoder := xml.NewDecoder(bytes.NewReader(description))
	var services []Service
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "service" {
			continue
		}
		var raw serviceXML
		if err := decoder.DecodeElement(&raw, &start); err != nil {
			return nil, err
		}
		service := Service{
			ServiceType: strings.TrimSpace(raw.ServiceType),
			ServiceID:   strings.TrimSpace(raw.ServiceID),
			SCPDURL:     resolveURL(strings.TrimSpace(raw.SCPDURL), baseURL),
			ControlURL:  resolveURL(strings.TrimSpace(raw.ControlURL), baseURL),
			EventSubURL: resolveURL(strings.TrimSpace(raw.EventSubURL), baseURL),
		}
		if service.ServiceType == "" || service.ServiceID == "" || service.ControlURL == "" {
			continue
		}
		services = append(services, service)
	}
	return services, nil
}

func (s Service) IsForward() bool {
	return forwardServiceTypes[s.ServiceType] && s.ServiceID != "" && s.ControlURL != ""
}

func (s Service) NewAddPortMappingRequest(mapping PortMapping) (*http.Request, error) {
	if !s.IsForward() {
		return nil, fmt.Errorf("unsupported UPnP service type %q", s.ServiceType)
	}
	protocol := strings.ToUpper(mapping.Protocol)
	if protocol == "" {
		protocol = "TCP"
	}
	if protocol != "TCP" && protocol != "UDP" {
		return nil, fmt.Errorf("unsupported UPnP protocol %q", mapping.Protocol)
	}
	description := mapping.Description
	if description == "" {
		description = "Natter"
	}

	body := addPortMappingBody(s.ServiceType, mapping, protocol, description)
	req, err := http.NewRequest(http.MethodPost, s.ControlURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("SOAPAction", strconv.Quote(s.ServiceType+"#AddPortMapping"))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Connection", "close")
	return req, nil
}

func resolveURL(value string, baseURL string) string {
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return value
	}
	ref, err := url.Parse(value)
	if err != nil {
		return value
	}
	return base.ResolveReference(ref).String()
}

func addPortMappingBody(serviceType string, mapping PortMapping, protocol string, description string) string {
	return "<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n" +
		"<s:Envelope xmlns:s=\"http://schemas.xmlsoap.org/soap/envelope/\"\r\n" +
		"  s:encodingStyle=\"http://schemas.xmlsoap.org/soap/encoding/\">\r\n" +
		"  <s:Body>\r\n" +
		"    <m:AddPortMapping xmlns:m=\"" + html.EscapeString(serviceType) + "\">\r\n" +
		"      <NewRemoteHost>" + html.EscapeString(mapping.RemoteHost) + "</NewRemoteHost>\r\n" +
		"      <NewExternalPort>" + strconv.Itoa(mapping.ExternalPort) + "</NewExternalPort>\r\n" +
		"      <NewProtocol>" + protocol + "</NewProtocol>\r\n" +
		"      <NewInternalPort>" + strconv.Itoa(mapping.InternalPort) + "</NewInternalPort>\r\n" +
		"      <NewInternalClient>" + html.EscapeString(mapping.InternalClient) + "</NewInternalClient>\r\n" +
		"      <NewEnabled>1</NewEnabled>\r\n" +
		"      <NewPortMappingDescription>" + html.EscapeString(description) + "</NewPortMappingDescription>\r\n" +
		"      <NewLeaseDuration>" + strconv.Itoa(mapping.LeaseDuration) + "</NewLeaseDuration>\r\n" +
		"    </m:AddPortMapping>\r\n" +
		"  </s:Body>\r\n" +
		"</s:Envelope>\r\n"
}
