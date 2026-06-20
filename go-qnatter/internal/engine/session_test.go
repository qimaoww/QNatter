package engine

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"qnatter-openwrt/go-qnatter/internal/config"
	"qnatter-openwrt/go-qnatter/internal/forward"
	"qnatter-openwrt/go-qnatter/internal/stun"
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
		t.Fatalf("session target = %s, want qnatter address", session.Result.Target)
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

func TestStartSessionUsesActualLocalPortForUPnPWhenBindPortIsZero(t *testing.T) {
	events := []string{}
	upnp := &fakeUPnP{events: &events}

	_, err := StartSession(context.Background(), config.Config{
		UPnP:              true,
		BindPort:          0,
		ForwardMethod:     "none",
		KeepAliveInterval: 15,
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
		KeepAlive: &fakeKeepAlive{events: &events},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			return &fakeForwarder{events: &events}, nil
		},
		UPnP: upnp,
	})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	if upnp.mapping.ExternalPort != 41000 || upnp.mapping.InternalPort != 41000 {
		t.Fatalf("UPnP ports = %d/%d, want actual local port 41000", upnp.mapping.ExternalPort, upnp.mapping.InternalPort)
	}
}

func TestStartSessionContinuesWhenUPnPForwardFails(t *testing.T) {
	events := []string{}
	var upnpErr error

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
		OnUPnPError: func(operation string, err error) {
			events = append(events, "upnp-error:"+operation)
			upnpErr = err
		},
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
		"upnp-error:forward port",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
	if upnpErr == nil || upnpErr.Error() != "upnp conflict" {
		t.Fatalf("UPnP error = %v, want upnp conflict", upnpErr)
	}
}

func TestStartSessionReturnsRetryWhenTargetPortClosed(t *testing.T) {
	events := []string{}

	_, err := StartSession(context.Background(), config.Config{
		RetryTarget:   true,
		ForwardMethod: "socket",
		TargetIP:      "10.10.10.10",
		TargetPort:    51413,
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
		KeepAlive: &fakeKeepAlive{events: &events},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			events = append(events, "forwarder:"+method)
			return &fakeForwarder{events: &events}, nil
		},
		PortCheck: fakePortCheck{
			result: PortClosed,
			events: &events,
		},
	})
	if !errors.Is(err, ErrTargetClosed) {
		t.Fatalf("StartSession error = %v, want ErrTargetClosed", err)
	}

	wantEvents := []string{
		"stun",
		"keepalive",
		"stun",
		"forwarder:socket",
		"forward",
		"portcheck",
		"forward-stop",
		"keepalive-close",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}
