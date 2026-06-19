package engine

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/forward"
	"natter-openwrt/go-natter/internal/stun"
)

var errNoSTUNMapping = errors.New("unexpected STUN call")

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

func TestRunLoopSkipsSTUNRecheckWhenTCPPortCheckIsOpen(t *testing.T) {
	events := []string{}
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
			KeepAlive: &fakeKeepAlive{events: &events},
			NewForwarder: func(method string) (forward.Forwarder, error) {
				return &fakeForwarder{events: &events}, nil
			},
			PortCheck: fakePortCheck{
				result: PortOpen,
				events: &events,
			},
		}, LoopOptions{Ticks: ticks, RecheckEvery: 1})
	}()

	ticks <- time.Now()
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("RunLoop returned error: %v", err)
	}
	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forward",
		"keepalive",
		"portcheck",
		"forward-stop",
		"keepalive-close",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}

func TestRunLoopUDPAlwaysUsesSTUNRecheck(t *testing.T) {
	events := []string{}
	ticks := make(chan time.Time)

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLoop(context.Background(), config.Config{UDP: true, ForwardMethod: "none"}, Dependencies{
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
			KeepAlive: &fakeKeepAlive{events: &events},
			NewForwarder: func(method string) (forward.Forwarder, error) {
				return &fakeForwarder{events: &events}, nil
			},
			PortCheck: fakePortCheck{
				result: PortOpen,
				events: &events,
			},
		}, LoopOptions{Ticks: ticks, RecheckEvery: 1})
	}()

	ticks <- time.Now()

	if err := <-errCh; !errors.Is(err, ErrMappingChanged) {
		t.Fatalf("RunLoop error = %v, want ErrMappingChanged", err)
	}
	if containsEvent(events, "portcheck") {
		t.Fatalf("UDP loop should not call portcheck, events=%#v", events)
	}
}

func TestRunLoopReportsLocalAddressChange(t *testing.T) {
	events := []string{}
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
				},
				events: &events,
			},
			KeepAlive: &fakeKeepAlive{
				events: &events,
				errs: []error{
					nil,
					os.NewSyscallError("write", syscall.EADDRNOTAVAIL),
				},
			},
			NewForwarder: func(method string) (forward.Forwarder, error) {
				return &fakeForwarder{events: &events}, nil
			},
		}, LoopOptions{Ticks: ticks})
	}()

	ticks <- time.Now()

	if err := <-errCh; !errors.Is(err, ErrLocalAddressChanged) {
		t.Fatalf("RunLoop error = %v, want ErrLocalAddressChanged", err)
	}
	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forward",
		"keepalive",
		"forward-stop",
		"keepalive-close",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}

func TestRunLoopRenewsActiveUPnPMappingOnTicks(t *testing.T) {
	events := []string{}
	upnp := &fakeUPnP{events: &events}
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLoop(ctx, config.Config{
			UPnP:              true,
			BindPort:          51413,
			ForwardMethod:     "none",
			KeepAliveInterval: 15,
		}, Dependencies{
			STUN: &fakeSTUN{
				mappings: []stun.Mapping{
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:51413"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:51413"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
				},
				events: &events,
			},
			KeepAlive: &fakeKeepAlive{events: &events},
			NewForwarder: func(method string) (forward.Forwarder, error) {
				return &fakeForwarder{events: &events}, nil
			},
			UPnP: upnp,
		}, LoopOptions{Ticks: ticks})
	}()

	ticks <- time.Now()
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("RunLoop returned error: %v", err)
	}
	if !containsEvent(events, "upnp-renew") {
		t.Fatalf("events = %#v, want upnp-renew", events)
	}
}

func TestRunLoopContinuesWhenUPnPRenewFails(t *testing.T) {
	events := []string{}
	upnp := &fakeUPnP{events: &events, renewErr: errors.New("renew failed")}
	ticks := make(chan time.Time)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunLoop(ctx, config.Config{
			UPnP:              true,
			BindPort:          51413,
			ForwardMethod:     "none",
			KeepAliveInterval: 15,
		}, Dependencies{
			STUN: &fakeSTUN{
				mappings: []stun.Mapping{
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:51413"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
					{
						Inner: netip.MustParseAddrPort("10.10.10.3:51413"),
						Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
					},
				},
				events: &events,
			},
			KeepAlive: &fakeKeepAlive{events: &events},
			NewForwarder: func(method string) (forward.Forwarder, error) {
				return &fakeForwarder{events: &events}, nil
			},
			UPnP: upnp,
		}, LoopOptions{Ticks: ticks})
	}()

	ticks <- time.Now()
	cancel()

	if err := <-errCh; err != nil {
		t.Fatalf("RunLoop returned error: %v", err)
	}
	if !containsEvent(events, "upnp-renew") {
		t.Fatalf("events = %#v, want upnp-renew", events)
	}
}

type fakePortCheck struct {
	result PortResult
	events *[]string
}

func (p fakePortCheck) TestLAN(context.Context, netip.AddrPort, netip.Addr) PortResult {
	if p.events != nil {
		*p.events = append(*p.events, "portcheck")
	}
	return p.result
}

func containsEvent(events []string, want string) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
}
