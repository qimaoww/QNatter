package engine

import (
	"context"
	"time"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/upnp"
)

type UPnPClient struct {
	Client         upnp.Client
	DiscoverRouter func(context.Context, upnp.Client) (*upnp.Device, error)

	service *upnp.Service
	mapping upnp.PortMapping
}

func NewUPnPMapperFromConfig(cfg config.Config) (UPnPMapper, error) {
	bind, err := BindFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &UPnPClient{
		Client: upnp.Client{
			BindIP:    bind.Source.Addr().String(),
			Interface: bind.Interface,
			Timeout:   time.Second,
		},
	}, nil
}

func (c *UPnPClient) Forward(ctx context.Context, mapping UPnPMapping) error {
	client := c.Client
	client.BindIP = mapping.InternalClient
	device, err := c.discoverRouter(ctx, client)
	if err != nil {
		return err
	}
	if device == nil || device.ForwardService == nil {
		c.service = nil
		return nil
	}

	portMapping := upnp.PortMapping{
		RemoteHost:     "",
		ExternalPort:   mapping.ExternalPort,
		Protocol:       protocolName(mapping.UDP),
		InternalPort:   mapping.InternalPort,
		InternalClient: mapping.InternalClient,
		Description:    "Natter",
		LeaseDuration:  mapping.LeaseDuration,
	}
	if err := device.ForwardService.AddPortMapping(ctx, client.HTTPClient, portMapping); err != nil {
		c.service = nil
		return err
	}
	service := *device.ForwardService
	c.service = &service
	c.mapping = portMapping
	return nil
}

func (c *UPnPClient) Renew(ctx context.Context) error {
	if c.service == nil {
		return nil
	}
	return c.service.AddPortMapping(ctx, c.Client.HTTPClient, c.mapping)
}

func (c *UPnPClient) discoverRouter(ctx context.Context, client upnp.Client) (*upnp.Device, error) {
	if c.DiscoverRouter != nil {
		return c.DiscoverRouter(ctx, client)
	}
	return client.DiscoverRouter(ctx)
}

func protocolName(udp bool) string {
	if udp {
		return "udp"
	}
	return "tcp"
}
