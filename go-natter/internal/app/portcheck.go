package app

import (
	"context"
	"net/netip"

	"natter-openwrt/go-natter/internal/portcheck"
)

type splitPortChecker struct {
	lan portcheck.Checker
	wan portcheck.Checker
}

func (c splitPortChecker) TestLAN(ctx context.Context, addr netip.AddrPort, source netip.Addr) portcheck.Result {
	return c.lan.TestLAN(ctx, addr, source)
}

func (c splitPortChecker) TestWAN(ctx context.Context, port int, source netip.Addr) portcheck.Result {
	return c.wan.TestWAN(ctx, port, source)
}
