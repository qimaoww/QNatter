package upnp

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type Device struct {
	IP             string
	Location       string
	Services       []Service
	ForwardService *Service
}

func LoadDeviceDescription(ctx context.Context, client *http.Client, ip string, location string) (*Device, error) {
	if client == nil {
		client = http.DefaultClient
	}
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	services, err := ParseServices(body, location)
	if err != nil {
		return nil, err
	}

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
