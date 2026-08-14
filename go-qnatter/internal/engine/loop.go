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
	Ticks                 <-chan time.Time
	RecheckEvery          int
	KeepAliveFailureLimit int
	STUNFailureLimit      int
}

const defaultTransientFailureLimit = 3

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
	keepAliveFailureLimit := transientFailureLimit(options.KeepAliveFailureLimit)
	stunFailureLimit := transientFailureLimit(options.STUNFailureLimit)
	count := 0
	keepAliveFailures := 0
	stunFailures := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticks:
			if err := session.KeepAlive.KeepAlive(); err != nil {
				if localAddressUnavailable(err) {
					return fmt.Errorf("%w: %w", ErrLocalAddressChanged, err)
				}
				keepAliveFailures++
				if keepAliveFailures >= keepAliveFailureLimit {
					return fmt.Errorf("%w after %d consecutive attempts: %w", ErrKeepAliveFailed, keepAliveFailures, err)
				}
				reportTransientFailure(deps, "keep-alive", err, keepAliveFailures, keepAliveFailureLimit)
				continue
			}
			keepAliveFailures = 0
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
					if localAddressUnavailable(err) {
						return fmt.Errorf("%w: %w", ErrLocalAddressChanged, err)
					}
					stunFailures++
					if stunFailures >= stunFailureLimit {
						return err
					}
					reportTransientFailure(deps, "STUN recheck", err, stunFailures, stunFailureLimit)
					continue
				}
				stunFailures = 0
				if mapping.Outer != session.Result.Mapping.Outer {
					return ErrMappingChanged
				}
			}
		}
	}
}

func transientFailureLimit(limit int) int {
	if limit > 0 {
		return limit
	}
	return defaultTransientFailureLimit
}

func localAddressUnavailable(err error) bool {
	return errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.ENODEV)
}

func reportTransientFailure(deps Dependencies, operation string, err error, attempt int, limit int) {
	if deps.OnTransientFailure != nil {
		deps.OnTransientFailure(operation, err, attempt, limit)
	}
}
