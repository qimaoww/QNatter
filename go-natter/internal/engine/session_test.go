package engine

import (
	"context"
	"net/netip"
	"reflect"
	"testing"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/forward"
	"natter-openwrt/go-natter/internal/stun"
)

func TestStartSessionReturnsClosableRuntimeState(t *testing.T) {
	events := []string{}
	keepAlive := &fakeKeepAlive{events: &events}
	fwd := &fakeForwarder{events: &events}

	session, err := StartSession(context.Background(), config.Config{
		ForwardMethod: "none",
	}, Dependencies{
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
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}
	if session.Result.Target != netip.MustParseAddrPort("10.10.10.3:41000") {
		t.Fatalf("session target = %s, want natter address", session.Result.Target)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forwarder:none",
		"forward",
		"forward-stop",
		"keepalive-close",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}
