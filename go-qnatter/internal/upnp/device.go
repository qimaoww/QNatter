package upnp

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// maxDescriptionSize 限制设备描述响应体大小，避免被控设备用超大响应
// 造成内存放大（原实现无上限地 io.ReadAll）。
const maxDescriptionSize = 1 << 20 // 1 MiB

type Device struct {
	IP             string
	Location       string
	Services       []Service
	ForwardService *Service
}

func LoadDeviceDescription(ctx context.Context, client *http.Client, ip string, location string) (*Device, error) {
	client = httpClientOrDefault(client)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "close")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("upnp description %s returned HTTP %d", location, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDescriptionSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDescriptionSize {
		return nil, fmt.Errorf("upnp description %s too large (limit %d bytes)", location, maxDescriptionSize)
	}
	services, err := ParseServices(body, location)
	if err != nil {
		return nil, err
	}
	// 只保留控制地址与设备描述同源的服务：防御被投毒的描述把 SOAP 请求
	// 指向任意主机（盲 SSRF）。
	services = sameOriginServices(services, location)

	device := &Device{
		IP:       ip,
		Location: location,
		Services: services,
	}
	for i := range device.Services {
		if device.Services[i].IsForward() {
			device.ForwardService = &device.Services[i]
			break
		}
	}
	return device, nil
}
