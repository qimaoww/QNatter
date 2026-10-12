package engine

import (
	"context"
	"fmt"
	"time"

	"qnatter-openwrt/go-qnatter/internal/config"
	"qnatter-openwrt/go-qnatter/internal/upnp"
)

type UPnPClient struct {
	Client         upnp.Client
	DiscoverRouter func(context.Context, upnp.Client) (*upnp.Device, error)
	OnFoundRouter  func(string)

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
			BindIP:     bind.Source.Addr().String(),
			Interface:  bind.Interface,
			Timeout:    time.Second,
			HTTPClient: upnp.DefaultHTTPClient,
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
	// N-P2-9：没有设备/没有转发服务时必须报错。以前这里 return nil，engine
	// 于是把"UPnP 映射成功"记进 activeUPnP，用户看不到任何提示，实际端口
	// 并没有在路由器上打开。
	if device == nil {
		return fmt.Errorf("no IGD available: %w", upnp.ErrNoForwardService)
	}
	if device.ForwardService == nil {
		return fmt.Errorf("device %s has no port mapping service: %w", device.IP, upnp.ErrNoForwardService)
	}
	if c.OnFoundRouter != nil {
		c.OnFoundRouter(device.IP)
	}

	portMapping := upnp.PortMapping{
		RemoteHost:     "",
		ExternalPort:   mapping.ExternalPort,
		Protocol:       protocolName(mapping.UDP),
		InternalPort:   mapping.InternalPort,
		InternalClient: mapping.InternalClient,
		Description:    "QNatter",
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

// Revoke 删除路由器上的端口映射。没有已建立的映射时是空操作；
// 删除失败只返回错误，不清空状态，便于调用方重试。
func (c *UPnPClient) Revoke(ctx context.Context) error {
	if c.service == nil {
		return nil
	}
	service := *c.service
	mapping := c.mapping
	if err := service.DeletePortMapping(ctx, c.Client.HTTPClient, mapping); err != nil {
		return err
	}
	c.service = nil
	return nil
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
