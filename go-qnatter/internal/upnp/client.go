package upnp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// DefaultHTTPClient 用于所有 UPnP 设备交互。
//
// 它必须带超时（否则设备只接受连接不回包时调用方会被永久阻塞），
// 并且禁止跨主机重定向（否则被投毒的设备描述可以把请求转发到任意主机）。
// 未显式配置 Client.HTTPClient 时使用它，不再回退到无超时的 http.DefaultClient。
var DefaultHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableKeepAlives:     true,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return errors.New("upnp: cross-host redirect rejected")
		}
		origin := via[0].URL
		if !sameHostPort(req.URL.Hostname(), req.URL.Port(), req.URL.Scheme, origin.Hostname(), origin.Port(), origin.Scheme) {
			return errors.New("upnp: cross-host redirect rejected")
		}
		if len(via) >= maxRedirects {
			return errors.New("upnp: too many redirects")
		}
		return nil
	},
}

// maxRedirects 是允许的最大重定向跳数。
const maxRedirects = 3

// ErrNoForwardService 表示发现了 SSDP 响应/设备描述，但其中没有任何可用于
// 端口映射的 IGD/WAN 服务（或干脆没有设备可读）。
//
// N-P2-9：以前这条路径返回 (nil, nil)，调用方按 err == nil 当成"UPnP 映射
// 成功"，用户侧完全静默（端口其实没有映射成功）。现在返回明确错误，由
// engine 上报为告警。
var ErrNoForwardService = errors.New("upnp: no IGD port mapping service found")

func httpClientOrDefault(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return DefaultHTTPClient
}

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
	httpClient := httpClientOrDefault(c.HTTPClient)
	for _, location := range locations {
		device, err := LoadDeviceDescription(ctx, httpClient, location.IP, location.URL)
		if err != nil {
			continue
		}
		if device.ForwardService != nil {
			return device, nil
		}
	}
	// 未发现可用的 IGD 时明确报错（而不是返回 nil, nil 让调用方误判成功）。
	return nil, fmt.Errorf("%w (%d location(s) probed)", ErrNoForwardService, len(locations))
}
