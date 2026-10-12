package upnp

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const userAgent = "curl/8.0.0 (QNatter)"

const (
	// maxSOAPBodySize 限制 SOAP 响应体大小，避免被控设备造成内存放大。
	maxSOAPBodySize = 64 << 10 // 64 KiB
	// maxServiceCount 限制设备描述里被解析的服务数量，避免解析放大。
	maxServiceCount = 512
	// maxLeaseDuration 是 UPnP 端口映射允许的最长租期（秒，一周）。
	maxLeaseDuration = 7 * 24 * 60 * 60
)

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
		if len(services) >= maxServiceCount {
			break
		}
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
	protocol, err := mappingProtocol(mapping.Protocol)
	if err != nil {
		return nil, err
	}
	if err := validateMapping(mapping); err != nil {
		return nil, err
	}
	description := mapping.Description
	if description == "" {
		description = "QNatter"
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

func (s Service) NewDeletePortMappingRequest(mapping PortMapping) (*http.Request, error) {
	if !s.IsForward() {
		return nil, fmt.Errorf("unsupported UPnP service type %q", s.ServiceType)
	}
	protocol, err := mappingProtocol(mapping.Protocol)
	if err != nil {
		return nil, err
	}
	if err := validateMapping(mapping); err != nil {
		return nil, err
	}

	body := deletePortMappingBody(s.ServiceType, mapping, protocol)
	req, err := http.NewRequest(http.MethodPost, s.ControlURL, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("SOAPAction", strconv.Quote(s.ServiceType+"#DeletePortMapping"))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Connection", "close")
	return req, nil
}

func (s Service) AddPortMapping(ctx context.Context, client *http.Client, mapping PortMapping) error {
	req, err := s.NewAddPortMappingRequest(mapping)
	if err != nil {
		return err
	}
	return s.doSOAP(ctx, client, req, "AddPortMapping")
}

func (s Service) DeletePortMapping(ctx context.Context, client *http.Client, mapping PortMapping) error {
	req, err := s.NewDeletePortMappingRequest(mapping)
	if err != nil {
		return err
	}
	return s.doSOAP(ctx, client, req, "DeletePortMapping")
}

func (s Service) doSOAP(ctx context.Context, client *http.Client, req *http.Request, action string) error {
	client = httpClientOrDefault(client)
	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSOAPBodySize+1))
	if err != nil {
		return err
	}
	if len(body) > maxSOAPBodySize {
		return fmt.Errorf("upnp %s response too large (limit %d bytes)", action, maxSOAPBodySize)
	}
	if code, description := parseUPnPError(body); code != "" || description != "" {
		return fmt.Errorf("upnp %s failed: [%s] %s", action, code, description)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("upnp %s returned HTTP %d", action, resp.StatusCode)
	}
	return nil
}

func mappingProtocol(value string) (string, error) {
	protocol := strings.ToUpper(strings.TrimSpace(value))
	if protocol == "" {
		return "TCP", nil
	}
	if protocol != "TCP" && protocol != "UDP" {
		return "", fmt.Errorf("unsupported UPnP protocol %q", value)
	}
	return protocol, nil
}

// validateMapping 在拼接 SOAP 报文前校验字段范围：非法端口/租期不应
// 被静默发给路由器。
func validateMapping(mapping PortMapping) error {
	if mapping.ExternalPort < 0 || mapping.ExternalPort > 65535 {
		return fmt.Errorf("invalid UPnP external port %d", mapping.ExternalPort)
	}
	if mapping.InternalPort < 0 || mapping.InternalPort > 65535 {
		return fmt.Errorf("invalid UPnP internal port %d", mapping.InternalPort)
	}
	if mapping.LeaseDuration < 0 || mapping.LeaseDuration > maxLeaseDuration {
		return fmt.Errorf("invalid UPnP lease duration %d", mapping.LeaseDuration)
	}
	return nil
}

// sameHostPort 比较两个主机的 hostname 与有效端口（空端口按 scheme 的默认端口处理）。
func sameHostPort(aHost, aPort, aScheme, bHost, bPort, bScheme string) bool {
	if aHost == "" || bHost == "" {
		return false
	}
	return strings.EqualFold(aHost, bHost) && effectivePort(aPort, aScheme) == effectivePort(bPort, bScheme)
}

func effectivePort(port string, scheme string) string {
	if port != "" {
		return port
	}
	if strings.EqualFold(scheme, "https") {
		return "443"
	}
	return "80"
}

// sameOriginServices 只保留控制地址与设备描述同源的服务，阻断
// “描述投毒 → 向任意主机发 SOAP 请求”的 SSRF 路径。
func sameOriginServices(services []Service, origin string) []Service {
	base, err := url.Parse(origin)
	if err != nil || base.Host == "" {
		return nil
	}
	filtered := make([]Service, 0, len(services))
	for _, service := range services {
		parsed, err := url.Parse(service.ControlURL)
		if err != nil || parsed.Host == "" {
			continue
		}
		if sameHostPort(parsed.Hostname(), parsed.Port(), parsed.Scheme, base.Hostname(), base.Port(), base.Scheme) {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

func resolveURL(value string, baseURL string) string {
	if value == "" {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Host == "" {
		return ""
	}
	ref, err := url.Parse(value)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	if resolved.Host == "" {
		return ""
	}
	// 默认拒绝跨主机：设备描述里的 controlURL 必须与描述本身同源。
	// 否则被投毒的描述可以让 QNatter 向任意 http(s) 主机发 SOAP 请求。
	if !sameHostPort(resolved.Hostname(), resolved.Port(), resolved.Scheme, base.Hostname(), base.Port(), base.Scheme) {
		return ""
	}
	return resolved.String()
}

func parseUPnPError(body []byte) (string, string) {
	decoder := xml.NewDecoder(bytes.NewReader(body))
	var current string
	var code string
	var description string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", ""
		}
		switch item := token.(type) {
		case xml.StartElement:
			current = item.Name.Local
		case xml.CharData:
			value := strings.TrimSpace(string(item))
			if value == "" {
				continue
			}
			switch current {
			case "errorCode":
				code = value
			case "errorDescription":
				description = value
			}
		case xml.EndElement:
			if current == item.Name.Local {
				current = ""
			}
		}
	}
	return code, description
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

func deletePortMappingBody(serviceType string, mapping PortMapping, protocol string) string {
	return "<?xml version=\"1.0\" encoding=\"utf-8\"?>\r\n" +
		"<s:Envelope xmlns:s=\"http://schemas.xmlsoap.org/soap/envelope/\"\r\n" +
		"  s:encodingStyle=\"http://schemas.xmlsoap.org/soap/encoding/\">\r\n" +
		"  <s:Body>\r\n" +
		"    <m:DeletePortMapping xmlns:m=\"" + html.EscapeString(serviceType) + "\">\r\n" +
		"      <NewRemoteHost>" + html.EscapeString(mapping.RemoteHost) + "</NewRemoteHost>\r\n" +
		"      <NewExternalPort>" + strconv.Itoa(mapping.ExternalPort) + "</NewExternalPort>\r\n" +
		"      <NewProtocol>" + protocol + "</NewProtocol>\r\n" +
		"    </m:DeletePortMapping>\r\n" +
		"  </s:Body>\r\n" +
		"</s:Envelope>\r\n"
}
