package engine

import (
	"context"
	"errors"
	"time"

	"natter-openwrt/go-natter/internal/config"
)

type RetryOptions struct {
	Sleep     func(context.Context, time.Duration) error
	Retryable func(error) bool
}

func RunWithRetry(ctx context.Context, cfg config.Config, run func(context.Context) error, options RetryOptions) error {
	for {
		err := run(ctx)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		if cfg.ExitWhenChanged && (errors.Is(err, ErrMappingChanged) || errors.Is(err, ErrLocalAddressChanged)) {
			return err
		}
		if !retryable(err, options.Retryable) {
			return err
		}
		if err := sleepRetry(ctx, retryDelay(cfg), options.Sleep); err != nil {
			return nil
		}
	}
}

func retryable(err error, fn func(error) bool) bool {
	if fn != nil {
		return fn(err)
	}
	return errors.Is(err, ErrMappingChanged) || errors.Is(err, ErrKeepAliveFailed) ||
		errors.Is(err, ErrTargetClosed) || errors.Is(err, ErrLocalAddressChanged)
}

func retryDelay(cfg config.Config) time.Duration {
	if cfg.KeepAliveInterval > 0 {
		return time.Duration(cfg.KeepAliveInterval) * time.Second
	}
	return 15 * time.Second
}

func sleepRetry(ctx context.Context, delay time.Duration, sleep func(context.Context, time.Duration) error) error {
	if sleep != nil {
		return sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
