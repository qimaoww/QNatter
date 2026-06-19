package engine

import (
	"context"
	"errors"
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

func TestStartSessionStartsUPnPMappingWhenEnabled(t *testing.T) {
	events := []string{}
	upnp := &fakeUPnP{events: &events}

	session, err := StartSession(context.Background(), config.Config{
		UPnP:              true,
		BindValue:         "pppoe-wan_cmcc",
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
			events = append(events, "forwarder:"+method)
			return &fakeForwarder{events: &events}, nil
		},
		UPnP: upnp,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	wantMapping := UPnPMapping{
		ExternalPort:   51413,
		InternalPort:   51413,
		InternalClient: "10.10.10.3",
		LeaseDuration:  45,
	}
	if upnp.mapping != wantMapping {
		t.Fatalf("UPnP mapping = %+v, want %+v", upnp.mapping, wantMapping)
	}
	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forwarder:none",
		"forward",
		"upnp-forward",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}

	if session.UPnP != upnp {
		t.Fatalf("session UPnP = %#v, want injected mapper", session.UPnP)
	}
}

func TestStartSessionContinuesWhenUPnPForwardFails(t *testing.T) {
	events := []string{}

	session, err := StartSession(context.Background(), config.Config{
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
			events = append(events, "forwarder:"+method)
			return &fakeForwarder{events: &events}, nil
		},
		UPnP: &fakeUPnP{events: &events, err: errors.New("upnp conflict")},
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}
	if session.UPnP != nil {
		t.Fatalf("session UPnP = %#v, want nil after failed UPnP setup", session.UPnP)
	}
	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forwarder:none",
		"forward",
		"upnp-forward",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}
