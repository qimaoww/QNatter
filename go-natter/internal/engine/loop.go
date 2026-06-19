package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"natter-openwrt/go-natter/internal/config"
)

type LoopOptions struct {
	Ticks        <-chan time.Time
	RecheckEvery int
}

var (
	ErrMappingChanged  = errors.New("mapped address changed")
	ErrKeepAliveFailed = errors.New("keep-alive failed")
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
				return fmt.Errorf("%w: %v", ErrKeepAliveFailed, err)
			}
			count++
			if count >= recheckEvery {
				count = 0
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
