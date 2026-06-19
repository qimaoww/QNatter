package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"natter-openwrt/go-natter/internal/config"
)

func TestRunWithRetryRestartsOnMappingChange(t *testing.T) {
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := RunWithRetry(ctx, config.Config{}, func(context.Context) error {
		calls++
		if calls == 1 {
			return ErrMappingChanged
		}
		cancel()
		return nil
	}, RetryOptions{Sleep: noRetrySleep})
	if err != nil {
		t.Fatalf("RunWithRetry returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("loop calls = %d, want 2", calls)
	}
}

func TestRunWithRetryReturnsMappingChangeWhenExitWhenChanged(t *testing.T) {
	calls := 0
	err := RunWithRetry(context.Background(), config.Config{ExitWhenChanged: true}, func(context.Context) error {
		calls++
		return ErrMappingChanged
	}, RetryOptions{})
	if !errors.Is(err, ErrMappingChanged) {
		t.Fatalf("RunWithRetry returned %v, want ErrMappingChanged", err)
	}
	if calls != 1 {
		t.Fatalf("loop calls = %d, want 1", calls)
	}
}

func TestRunWithRetryReturnsLocalAddressChangeWhenExitWhenChanged(t *testing.T) {
	calls := 0
	err := RunWithRetry(context.Background(), config.Config{ExitWhenChanged: true}, func(context.Context) error {
		calls++
		return ErrLocalAddressChanged
	}, RetryOptions{
		Sleep: func(context.Context, time.Duration) error {
			t.Fatalf("RunWithRetry slept instead of returning ErrLocalAddressChanged")
			return nil
		},
	})
	if !errors.Is(err, ErrLocalAddressChanged) {
		t.Fatalf("RunWithRetry returned %v, want ErrLocalAddressChanged", err)
	}
	if calls != 1 {
		t.Fatalf("loop calls = %d, want 1", calls)
	}
}

func TestRunWithRetryRestartsKeepAliveFailures(t *testing.T) {
	keepAliveErr := errors.New("keepalive failed")
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := RunWithRetry(ctx, config.Config{}, func(context.Context) error {
		calls++
		if calls == 1 {
			return keepAliveErr
		}
		cancel()
		return nil
	}, RetryOptions{Retryable: func(error) bool { return true }, Sleep: noRetrySleep})
	if err != nil {
		t.Fatalf("RunWithRetry returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("loop calls = %d, want 2", calls)
	}
}

func TestRunWithRetryRestartsWhenTargetPortClosed(t *testing.T) {
	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := RunWithRetry(ctx, config.Config{}, func(context.Context) error {
		calls++
		if calls == 1 {
			return ErrTargetClosed
		}
		cancel()
		return nil
	}, RetryOptions{Sleep: noRetrySleep})
	if err != nil {
		t.Fatalf("RunWithRetry returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("loop calls = %d, want 2", calls)
	}
}

func noRetrySleep(context.Context, time.Duration) error {
	return nil
}

func TestRunWithRetrySleepsBetweenRetries(t *testing.T) {
	calls := 0
	sleeps := []time.Duration{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := RunWithRetry(ctx, config.Config{KeepAliveInterval: 7}, func(context.Context) error {
		calls++
		if calls == 1 {
			return ErrMappingChanged
		}
		cancel()
		return nil
	}, RetryOptions{
		Sleep: func(_ context.Context, d time.Duration) error {
			sleeps = append(sleeps, d)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunWithRetry returned error: %v", err)
	}
	if len(sleeps) != 1 || sleeps[0] != 7*time.Second {
		t.Fatalf("sleeps = %#v, want 7s", sleeps)
	}
}
