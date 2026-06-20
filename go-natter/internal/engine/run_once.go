package engine

import (
	"context"
	"fmt"
	"net/netip"

	"natter-openwrt/go-natter/internal/config"
	"natter-openwrt/go-natter/internal/forward"
	"natter-openwrt/go-natter/internal/portcheck"
	"natter-openwrt/go-natter/internal/status"
	"natter-openwrt/go-natter/internal/stun"
)

type STUNClient interface {
	GetMapping(context.Context) (stun.Mapping, error)
}

type KeepAlive interface {
	KeepAlive() error
	Close() error
}

type UPnPMapper interface {
	Forward(context.Context, UPnPMapping) error
	Renew(context.Context) error
}

type UPnPMapping struct {
	ExternalPort   int
	InternalPort   int
	InternalClient string
	UDP            bool
	LeaseDuration  int
}

type PortResult = portcheck.Result

const (
	PortClosed  = portcheck.Closed
	PortUnknown = portcheck.Unknown
	PortOpen    = portcheck.Open
)

type LANPortChecker interface {
	TestLAN(context.Context, netip.AddrPort, netip.Addr) PortResult
}

type PortChecker interface {
	LANPortChecker
	TestWAN(context.Context, int, netip.Addr) PortResult
}

type Dependencies struct {
	STUN         STUNClient
	KeepAlive    KeepAlive
	NewKeepAlive func(stun.Mapping) (KeepAlive, error)
	NewForwarder func(string) (forward.Forwarder, error)
	PortCheck    LANPortChecker
	InitialCheck PortChecker
	Notify       func(status.Mapping) error
	OnMapped     func(Result)
	OnUPnPError  func(string, error)
	UPnP         UPnPMapper
}

type Result struct {
	Method   string
	Mapping  stun.Mapping
	Target   netip.AddrPort
	Unstable bool
	Ports    PortReport
}

type PortReport struct {
	Checked   bool
	TargetLAN PortResult
	NatterLAN PortResult
	OuterLAN  PortResult
	OuterWAN  PortResult
}

type Session struct {
	Result    Result
	Forwarder forward.Forwarder
	KeepAlive KeepAlive
	UPnP      UPnPMapper
}

func (s *Session) Close() error {
	var firstErr error
	if s.Forwarder != nil {
		if err := s.Forwarder.Stop(); err != nil {
			firstErr = err
		}
		s.Forwarder = nil
	}
	if s.KeepAlive != nil {
		if err := s.KeepAlive.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.KeepAlive = nil
	}
	return firstErr
}

func RunOnce(ctx context.Context, cfg config.Config, deps Dependencies) (Result, error) {
	session, err := StartSession(ctx, cfg, deps)
	if err != nil {
		return Result{}, err
	}
	defer session.Close()
	return session.Result, nil
}

func StartSession(ctx context.Context, cfg config.Config, deps Dependencies) (*Session, error) {
	if deps.STUN == nil {
		return nil, fmt.Errorf("missing STUN client")
	}

	first, err := deps.STUN.GetMapping(ctx)
	if err != nil {
		return nil, err
	}
	keepAlive := deps.KeepAlive
	if keepAlive == nil && deps.NewKeepAlive != nil {
		keepAlive, err = deps.NewKeepAlive(first)
		if err != nil {
			return nil, err
		}
	}
	if keepAlive == nil {
		return nil, fmt.Errorf("missing keep-alive client")
	}
	if err := keepAlive.KeepAlive(); err != nil {
		return nil, err
	}
	mapping, err := deps.STUN.GetMapping(ctx)
	if err != nil {
		return nil, err
	}
	if !mapping.Inner.IsValid() {
		mapping.Inner = first.Inner
	}

	method := forward.ResolveMethod(forward.FactoryOptions{
		Method:     cfg.ForwardMethod,
		BindValue:  cfg.BindValue,
		BindPort:   cfg.BindPort,
		TargetIP:   cfg.TargetIP,
		TargetPort: cfg.TargetPort,
	})
	newForwarder := deps.NewForwarder
	if newForwarder == nil {
		newForwarder = forward.NewForwarder
	}
	fwd, err := newForwarder(method)
	if err != nil {
		return nil, err
	}

	target := resolveTarget(cfg, method, mapping)
	options := forward.StartOptions{
		IP:         mapping.Inner.Addr().String(),
		Port:       int(mapping.Inner.Port()),
		TargetIP:   target.Addr().String(),
		TargetPort: int(target.Port()),
		UDP:        cfg.UDP,
	}
	if err := fwd.Start(options); err != nil {
		return nil, err
	}

	var activeUPnP UPnPMapper
	if cfg.UPnP {
		if deps.UPnP == nil {
			return nil, fmt.Errorf("missing UPnP mapper")
		}
		upnpPort := int(mapping.Inner.Port())
		if err := deps.UPnP.Forward(ctx, UPnPMapping{
			ExternalPort:   upnpPort,
			InternalPort:   upnpPort,
			InternalClient: mapping.Inner.Addr().String(),
			UDP:            cfg.UDP,
			LeaseDuration:  cfg.KeepAliveInterval * 3,
		}); err == nil {
			activeUPnP = deps.UPnP
		} else if deps.OnUPnPError != nil {
			deps.OnUPnPError("forward port", err)
		}
	}

	if deps.Notify != nil {
		protocol := "tcp"
		if cfg.UDP {
			protocol = "udp"
		}
		if err := deps.Notify(status.Mapping{
			Protocol: protocol,
			Inner:    target,
			Outer:    mapping.Outer,
			Message:  "mapped",
		}); err != nil {
			return nil, err
		}
	}

	portReport := checkInitialPorts(ctx, cfg, deps, target, mapping)
	result := Result{
		Method:   method,
		Mapping:  mapping,
		Target:   target,
		Unstable: first.Outer.IsValid() && mapping.Outer.IsValid() && first.Outer != mapping.Outer,
		Ports:    portReport,
	}
	if deps.OnMapped != nil {
		deps.OnMapped(result)
	}

	session := &Session{
		Result:    result,
		Forwarder: fwd,
		KeepAlive: keepAlive,
		UPnP:      activeUPnP,
	}
	if cfg.RetryTarget && !cfg.UDP && targetClosed(ctx, deps, session, portReport) {
		_ = session.Close()
		return nil, ErrTargetClosed
	}
	return session, nil
}

func checkInitialPorts(ctx context.Context, cfg config.Config, deps Dependencies, target netip.AddrPort, mapping stun.Mapping) PortReport {
	if cfg.UDP || deps.InitialCheck == nil {
		return PortReport{}
	}
	checker := deps.InitialCheck
	source := mapping.Inner.Addr()
	return PortReport{
		Checked:   true,
		TargetLAN: checker.TestLAN(ctx, target, source),
		NatterLAN: checker.TestLAN(ctx, mapping.Inner, source),
		OuterLAN:  checker.TestLAN(ctx, mapping.Outer, source),
		OuterWAN:  checker.TestWAN(ctx, int(mapping.Outer.Port()), source),
	}
}

func targetClosed(ctx context.Context, deps Dependencies, session *Session, report PortReport) bool {
	if report.Checked {
		return report.TargetLAN == PortClosed
	}
	return targetPortClosed(ctx, deps, session)
}

func resolveTarget(cfg config.Config, method string, mapping stun.Mapping) netip.AddrPort {
	if method == "none" || method == "test" {
		return mapping.Inner
	}

	targetIP := cfg.TargetIP
	if targetIP == "" || targetIP == "0.0.0.0" || targetIP == "127.0.0.1" {
		targetIP = mapping.Inner.Addr().String()
	}
	targetPort := cfg.TargetPort
	if targetPort == 0 {
		targetPort = int(mapping.Outer.Port())
	}
	addr, err := netip.ParseAddr(targetIP)
	if err != nil {
		return mapping.Inner
	}
	return netip.AddrPortFrom(addr, uint16(targetPort))
}

func targetPortClosed(ctx context.Context, deps Dependencies, session *Session) bool {
	checker := deps.PortCheck
	if checker == nil {
		checker = portcheck.Checker{}
	}
	return checker.TestLAN(ctx, session.Result.Target, session.Result.Mapping.Inner.Addr()) == PortClosed
}
