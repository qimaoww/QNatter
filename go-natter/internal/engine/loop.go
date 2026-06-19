package engine

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/portcheck"
)

type LoopOptions struct {
	Ticks        <-chan time.Time
	RecheckEvery int
}

var (
	ErrMappingChanged      = errors.New("mapped address changed")
	ErrKeepAliveFailed     = errors.New("keep-alive failed")
	ErrTargetClosed        = errors.New("target port closed")
	ErrLocalAddressChanged = errors.New("local address changed")
)

func RunLoop(ctx context.Context, cfg config.Config, deps Dependencies, options LoopOptions) error {
	session, err := StartSession(ctx, cfg, deps)
	if err != nil {
		return err
	}
	defer session.Close()

	ticks := options.Ticks
	var ticker *time.Ticker
	if ticks == nil {
		interval := time.Duration(cfg.KeepAliveInterval) * time.Second
		if interval <= 0 {
			interval = 15 * time.Second
		}
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
		ticks = ticker.C
	}

	recheckEvery := options.RecheckEvery
	if recheckEvery <= 0 {
		recheckEvery = 20
	}
	count := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
			if err := session.KeepAlive.KeepAlive(); err != nil {
				if errors.Is(err, syscall.EADDRNOTAVAIL) {
					return fmt.Errorf("%w: %v", ErrLocalAddressChanged, err)
				}
				return fmt.Errorf("%w: %v", ErrKeepAliveFailed, err)
			}
			if session.UPnP != nil {
				_ = session.UPnP.Renew(ctx)
			}
			count++
			if count >= recheckEvery {
				count = 0
				if !cfg.UDP && lanPortOpen(ctx, deps, session) {
					continue
				}
				mapping, err := deps.STUN.GetMapping(ctx)
				if err != nil {
					return err
				}
				if mapping.Outer != session.Result.Mapping.Outer {
					return ErrMappingChanged
				}
			}
		}
	}
}

func lanPortOpen(ctx context.Context, deps Dependencies, session *Session) bool {
	checker := deps.PortCheck
	if checker == nil {
		checker = portcheck.Checker{}
	}
	return checker.TestLAN(ctx, session.Result.Mapping.Outer, session.Result.Mapping.Inner.Addr()) != PortClosed
}
