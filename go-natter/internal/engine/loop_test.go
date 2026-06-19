package engine

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/forward"
	"natter-openwrt/go-natter/internal/stun"
)

func TestRunLoopKeepsAliveOnTicksAndClosesOnCancel(t *testing.T) {
	events := []string{}
	keepAlive := &fakeKeepAlive{events: &events}
	fwd := &fakeForwarder{events: &events}
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLoop(ctx, config.Config{ForwardMethod: "none"}, Dependencies{
			STUN: &fakeSTUN{
				mappings: []stun.Mapping{
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
				},
				events: &events,
			},
			KeepAlive: keepAlive,
			NewForwarder: func(method string) (forward.Forwarder, error) {
				events = append(events, "forwarder:"+method)
				return fwd, nil
			},
		}, LoopOptions{Ticks: ticks})
	}()

	ticks <- time.Now()
	ticks <- time.Now()
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("RunLoop returned error: %v", err)
	}

	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forwarder:none",
		"forward",
		"keepalive",
		"keepalive",
		"forward-stop",
		"keepalive-close",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}

func TestRunLoopRechecksMappingAndStopsWhenOuterAddressChanges(t *testing.T) {
	events := []string{}
	keepAlive := &fakeKeepAlive{events: &events}
	fwd := &fakeForwarder{events: &events}
	ticks := make(chan time.Time)

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLoop(context.Background(), config.Config{ForwardMethod: "none"}, Dependencies{
			STUN: &fakeSTUN{
				mappings: []stun.Mapping{
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62011"),
					},
				},
				events: &events,
			},
			KeepAlive: keepAlive,
			NewForwarder: func(method string) (forward.Forwarder, error) {
				events = append(events, "forwarder:"+method)
				return fwd, nil
			},
		}, LoopOptions{Ticks: ticks, RecheckEvery: 2})
	}()

	ticks <- time.Now()
	ticks <- time.Now()

	err := <-errCh
	if !errors.Is(err, ErrMappingChanged) {
		t.Fatalf("RunLoop error = %v, want ErrMappingChanged", err)
	}
	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forwarder:none",
		"forward",
		"keepalive",
		"keepalive",
		"stun",
		"forward-stop",
		"keepalive-close",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}
