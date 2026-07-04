package engine

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"qnatter-openwrt/go-qnatter/internal/config"
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
				if err := session.UPnP.Renew(ctx); err != nil && deps.OnUPnPError != nil {
					deps.OnUPnPError("renew upnp", err)
				}
			}
			count++
			if count >= recheckEvery {
				count = 0
				mapping, err := deps.STUN.GetMapping(ctx)
				if err != nil {
					if errors.Is(err, syscall.EADDRNOTAVAIL) {
						return fmt.Errorf("%w: %v", ErrLocalAddressChanged, err)
					}
					return err
				}
				if mapping.Outer != session.Result.Mapping.Outer {
					return ErrMappingChanged
				}
			}
		}
	}
}
