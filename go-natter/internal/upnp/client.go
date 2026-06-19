package upnp

import (
	"context"
	"net/http"
	"time"
)

type Client struct {
	BindIP            string
	Interface         string
	Timeout           time.Duration
	HTTPClient        *http.Client
	DiscoverLocations func(context.Context) ([]Location, error)
}

func (c Client) DiscoverRouter(ctx context.Context) (*Device, error) {
	discoverLocations := c.DiscoverLocations
	if discoverLocations == nil {
		discoverLocations = func(ctx context.Context) ([]Location, error) {
			return Discover(ctx, DiscoverOptions{
				BindIP:    c.BindIP,
				Interface: c.Interface,
				Timeout:   c.Timeout,
			})
		}
	}

	locations, err := discoverLocations(ctx)
	if err != nil {
		return nil, err
	}
	for _, location := range locations {
		device, err := LoadDeviceDescription(ctx, c.HTTPClient, location.IP, location.URL)
		if err != nil {
			continue
		}
		if device.ForwardService != nil {
			return device, nil
		}
	}
	return nil, nil
}
