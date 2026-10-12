package engine

import (
	"context"
	"errors"
	"syscall"
	"time"

	"qnatter-openwrt/go-qnatter/internal/config"
	"qnatter-openwrt/go-qnatter/internal/stun"
)

type RetryOptions struct {
	Sleep     func(context.Context, time.Duration) error
	Retryable func(error) bool
	OnRetry   func(error, time.Duration)
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
		delay := retryDelay(cfg)
		if options.OnRetry != nil {
			options.OnRetry(err, delay)
		}
		if err := sleepRetry(ctx, delay, options.Sleep); err != nil {
			return nil
		}
	}
}

func retryable(err error, fn func(error) bool) bool {
	if fn != nil {
		return fn(err)
	}
	return errors.Is(err, ErrMappingChanged) || errors.Is(err, ErrKeepAliveFailed) ||
		errors.Is(err, ErrTargetClosed) || errors.Is(err, ErrLocalAddressChanged) ||
		errors.Is(err, stun.ErrNoServerAvailable) || errors.Is(err, syscall.EADDRNOTAVAIL) ||
		errors.Is(err, syscall.ENODEV)
}

// 重试退避被夹取在 [minRetryDelay, maxRetryDelay]：既不会因为 -k 的
// 溢出值退化为无退避热循环，也不会因为极端配置等待过久。
const (
	minRetryDelay = time.Second
	maxRetryDelay = time.Hour
)

func retryDelay(cfg config.Config) time.Duration {
	delay := 15 * time.Second
	if cfg.KeepAliveInterval > 0 {
		// 先用秒级上界夹取再做乘法：int 秒 -> time.Duration 的乘法在
		// 超大值上会回绕成负值，time.NewTimer(负值) 立即触发，使
		// RunWithRetry 变成无退避热循环。
		seconds := int64(cfg.KeepAliveInterval)
		if maxSeconds := int64(maxRetryDelay / time.Second); seconds > maxSeconds {
			seconds = maxSeconds
		}
		delay = time.Duration(seconds) * time.Second
	}
	if delay < minRetryDelay {
		return minRetryDelay
	}
	return delay
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
