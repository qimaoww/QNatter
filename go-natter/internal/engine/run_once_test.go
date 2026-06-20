package engine

import (
	"context"
	"net/netip"
	"reflect"
	"testing"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/forward"
	"natter-openwrt/go-natter/internal/status"
	"natter-openwrt/go-natter/internal/stun"
)

func TestRunOnceEstablishesSocketForwardAndNotifies(t *testing.T) {
	events := []string{}
	stunClient := &fakeSTUN{
		mappings: []stun.Mapping{
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:61000"),
			},
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:62000"),
			},
		},
		events: &events,
	}
	keepAlive := &fakeKeepAlive{events: &events}
	fwd := &fakeForwarder{events: &events}
	var notifyMapping status.Mapping

	result, err := RunOnce(context.Background(), config.Config{
		InstanceID:    "mc_ct",
		BindValue:     "pppoe-wan_cmcc",
		RouteMark:     "0x4e000002",
		RouteTable:    "20002",
		RoutePriority: "20002",
		TargetIP:      "10.10.10.10",
		ForwardMethod: "socket",
	}, Dependencies{
		STUN:      stunClient,
		KeepAlive: keepAlive,
		NewForwarder: func(method string) (forward.Forwarder, error) {
			events = append(events, "forwarder:"+method)
			if method != "socket" {
				t.Fatalf("forward method = %q, want socket", method)
			}
			return fwd, nil
		},
		Notify: func(mapping status.Mapping) error {
			events = append(events, "notify")
			notifyMapping = mapping
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	wantEvents := []string{"stun", "keepalive", "stun", "forwarder:socket", "forward", "notify", "forward-stop", "keepalive-close"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
	wantStart := forward.StartOptions{
		IP:            "10.10.10.2",
		Port:          40000,
		TargetIP:      "10.10.10.10",
		TargetPort:    62000,
		Interface:     "pppoe-wan_cmcc",
		RouteMark:     "0x4e000002",
		RouteTable:    "20002",
		RoutePriority: "20002",
	}
	if fwd.start != wantStart {
		t.Fatalf("forward start = %+v, want %+v", fwd.start, wantStart)
	}
	if notifyMapping.Protocol != "tcp" {
		t.Fatalf("notify protocol = %q, want tcp", notifyMapping.Protocol)
	}
	if notifyMapping.Inner != netip.MustParseAddrPort("10.10.10.10:62000") {
		t.Fatalf("notify inner = %s, want target address", notifyMapping.Inner)
	}
	if notifyMapping.Outer != netip.MustParseAddrPort("203.0.113.10:62000") {
		t.Fatalf("notify outer = %s, want second STUN mapping", notifyMapping.Outer)
	}
	if result.Method != "socket" || result.Target != notifyMapping.Inner {
		t.Fatalf("result = %+v, want socket target %s", result, notifyMapping.Inner)
	}
}

func TestRunOnceReportsMappedResultAfterForwarding(t *testing.T) {
	stunClient := &fakeSTUN{
		mappings: []stun.Mapping{
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:62000"),
			},
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:62000"),
			},
		},
	}
	var mapped Result
	var called bool

	_, err := RunOnce(context.Background(), config.Config{
		ForwardMethod: "socket",
		TargetIP:      "10.10.10.10",
		TargetPort:    51413,
	}, Dependencies{
		STUN:      stunClient,
		KeepAlive: &fakeKeepAlive{},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			return &fakeForwarder{}, nil
		},
		OnMapped: func(result Result) {
			called = true
			mapped = result
		},
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if !called {
		t.Fatal("OnMapped was not called")
	}
	if mapped.Method != "socket" {
		t.Fatalf("mapped method = %q, want socket", mapped.Method)
	}
	if mapped.Target != netip.MustParseAddrPort("10.10.10.10:51413") {
		t.Fatalf("mapped target = %s, want 10.10.10.10:51413", mapped.Target)
	}
	if mapped.Mapping.Outer != netip.MustParseAddrPort("203.0.113.10:62000") {
		t.Fatalf("mapped outer = %s, want 203.0.113.10:62000", mapped.Mapping.Outer)
	}
}

func TestRunOnceMarksUnstableWhenOuterMappingChanges(t *testing.T) {
	stunClient := &fakeSTUN{
		mappings: []stun.Mapping{
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:61000"),
			},
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:62000"),
			},
		},
	}

	result, err := RunOnce(context.Background(), config.Config{
		ForwardMethod: "none",
	}, Dependencies{
		STUN:      stunClient,
		KeepAlive: &fakeKeepAlive{},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			return &fakeForwarder{}, nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if !result.Unstable {
		t.Fatalf("result unstable = false, want true when outer mapping changes")
	}
}

func TestRunOnceReportsInitialTCPPortChecks(t *testing.T) {
	stunClient := &fakeSTUN{
		mappings: []stun.Mapping{
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:62000"),
			},
			{
				Inner: netip.MustParseAddrPort("10.10.10.2:40000"),
				Outer: netip.MustParseAddrPort("203.0.113.10:62000"),
			},
		},
	}
	checker := sequencePortCheck{
		lan: []PortResult{PortClosed, PortOpen, PortClosed},
		wan: []PortResult{PortUnknown},
	}

	result, err := RunOnce(context.Background(), config.Config{
		ForwardMethod: "socket",
		TargetIP:      "10.10.10.10",
		TargetPort:    51413,
	}, Dependencies{
		STUN:      stunClient,
		KeepAlive: &fakeKeepAlive{},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			return &fakeForwarder{}, nil
		},
		InitialCheck: &checker,
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Ports.TargetLAN != PortClosed {
		t.Fatalf("target LAN check = %v, want closed", result.Ports.TargetLAN)
	}
	if result.Ports.NatterLAN != PortOpen {
		t.Fatalf("natter LAN check = %v, want open", result.Ports.NatterLAN)
	}
	if result.Ports.OuterLAN != PortClosed {
		t.Fatalf("outer LAN check = %v, want closed", result.Ports.OuterLAN)
	}
	if result.Ports.OuterWAN != PortUnknown {
		t.Fatalf("outer WAN check = %v, want unknown", result.Ports.OuterWAN)
	}
}

func TestRunOnceNoneForwardTargetsNatterAddressForUDP(t *testing.T) {
	stunClient := &fakeSTUN{
		mappings: []stun.Mapping{
			{
				Inner: netip.MustParseAddrPort("10.0.0.2:50000"),
				Outer: netip.MustParseAddrPort("198.51.100.8:62001"),
			},
			{
				Inner: netip.MustParseAddrPort("10.0.0.2:50000"),
				Outer: netip.MustParseAddrPort("198.51.100.8:62001"),
			},
		},
	}
	fwd := &fakeForwarder{}
	var notifyMapping status.Mapping

	_, err := RunOnce(context.Background(), config.Config{
		UDP:           true,
		BindValue:     "pppoe-wan_ct",
		ForwardMethod: "none",
	}, Dependencies{
		STUN:      stunClient,
		KeepAlive: &fakeKeepAlive{},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			if method != "none" {
				t.Fatalf("forward method = %q, want none", method)
			}
			return fwd, nil
		},
		Notify: func(mapping status.Mapping) error {
			notifyMapping = mapping
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	wantStart := forward.StartOptions{
		IP:         "10.0.0.2",
		Port:       50000,
		TargetIP:   "10.0.0.2",
		TargetPort: 50000,
		UDP:        true,
		Interface:  "pppoe-wan_ct",
	}
	if fwd.start != wantStart {
		t.Fatalf("forward start = %+v, want %+v", fwd.start, wantStart)
	}
	if notifyMapping.Protocol != "udp" {
		t.Fatalf("notify protocol = %q, want udp", notifyMapping.Protocol)
	}
	if notifyMapping.Inner != netip.MustParseAddrPort("10.0.0.2:50000") {
		t.Fatalf("notify inner = %s, want natter address", notifyMapping.Inner)
	}
}

func TestRunOnceCanBuildKeepAliveAfterFirstSTUNMapping(t *testing.T) {
	events := []string{}
	first := stun.Mapping{
		Inner: netip.MustParseAddrPort("10.10.10.3:41000"),
		Outer: netip.MustParseAddrPort("203.0.113.11:62010"),
	}
	stunClient := &fakeSTUN{
		mappings: []stun.Mapping{
			first,
			first,
		},
		events: &events,
	}
	fwd := &fakeForwarder{}

	_, err := RunOnce(context.Background(), config.Config{
		ForwardMethod: "none",
	}, Dependencies{
		STUN: stunClient,
		NewKeepAlive: func(mapping stun.Mapping) (KeepAlive, error) {
			events = append(events, "keepalive-factory")
			if mapping != first {
				t.Fatalf("keepalive mapping = %+v, want first mapping %+v", mapping, first)
			}
			return &fakeKeepAlive{events: &events}, nil
		},
		NewForwarder: func(string) (forward.Forwarder, error) {
			return fwd, nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}

	wantEvents := []string{"stun", "keepalive-factory", "keepalive", "stun", "keepalive-close"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %#v, want %#v", events, wantEvents)
	}
}

func TestRunOnceClosesSessionResources(t *testing.T) {
	events := []string{}

	_, err := RunOnce(context.Background(), config.Config{
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
		KeepAlive: &fakeKeepAlive{events: &events},
		NewForwarder: func(method string) (forward.Forwarder, error) {
			events = append(events, "forwarder:"+method)
			return &fakeForwarder{events: &events}, nil
		},
	})
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
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

type fakeSTUN struct {
	mappings []stun.Mapping
	events   *[]string
}

func (s *fakeSTUN) GetMapping(context.Context) (stun.Mapping, error) {
	if s.events != nil {
		*s.events = append(*s.events, "stun")
	}
	if len(s.mappings) == 0 {
		return stun.Mapping{}, errNoSTUNMapping
	}
	mapping := s.mappings[0]
	s.mappings = s.mappings[1:]
	return mapping, nil
}

type fakeKeepAlive struct {
	events *[]string
	err    error
	errs   []error
}

func (k *fakeKeepAlive) KeepAlive() error {
	if k.events != nil {
		*k.events = append(*k.events, "keepalive")
	}
	if len(k.errs) > 0 {
		err := k.errs[0]
		k.errs = k.errs[1:]
		return err
	}
	return k.err
}

func (k *fakeKeepAlive) Close() error {
	if k.events != nil {
		*k.events = append(*k.events, "keepalive-close")
	}
	return nil
}

type fakeForwarder struct {
	start  forward.StartOptions
	events *[]string
}

func (f *fakeForwarder) Start(options forward.StartOptions) error {
	if f.events != nil {
		*f.events = append(*f.events, "forward")
	}
	f.start = options
	return nil
}

func (f *fakeForwarder) Stop() error {
	if f.events != nil {
		*f.events = append(*f.events, "forward-stop")
	}
	return nil
}

type fakeUPnP struct {
	mapping  UPnPMapping
	events   *[]string
	err      error
	renewErr error
}

type sequencePortCheck struct {
	lan []PortResult
	wan []PortResult
}

func (p *sequencePortCheck) TestLAN(context.Context, netip.AddrPort, netip.Addr) PortResult {
	if len(p.lan) == 0 {
		return PortUnknown
	}
	result := p.lan[0]
	p.lan = p.lan[1:]
	return result
}

func (p *sequencePortCheck) TestWAN(context.Context, int, netip.Addr) PortResult {
	if len(p.wan) == 0 {
		return PortUnknown
	}
	result := p.wan[0]
	p.wan = p.wan[1:]
	return result
}

func (u *fakeUPnP) Forward(_ context.Context, mapping UPnPMapping) error {
	if u.events != nil {
		*u.events = append(*u.events, "upnp-forward")
	}
	u.mapping = mapping
	return u.err
}

func (u *fakeUPnP) Renew(context.Context) error {
	if u.events != nil {
		*u.events = append(*u.events, "upnp-renew")
	}
	return u.renewErr
}
