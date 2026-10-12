package engine

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"qnatter-openwrt/go-qnatter/internal/config"
	"qnatter-openwrt/go-qnatter/internal/forward"
	"qnatter-openwrt/go-qnatter/internal/portcheck"
	"qnatter-openwrt/go-qnatter/internal/status"
	"qnatter-openwrt/go-qnatter/internal/stun"
)

// upnpRevokeTimeout 限制退出时撤销 UPnP 映射的等待时间：撤销是尽力而为，
// 不应因为路由器无响应而无限拖住进程退出。
const upnpRevokeTimeout = 5 * time.Second

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
	Revoke(context.Context) error
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
	STUN               STUNClient
	KeepAlive          KeepAlive
	NewKeepAlive       func(stun.Mapping) (KeepAlive, error)
	NewForwarder       func(string) (forward.Forwarder, error)
	PortCheck          LANPortChecker
	InitialCheck       PortChecker
	Notify             func(status.Mapping) error
	OnMapped           func(Result)
	OnTransientFailure func(string, error, int, int)
	OnUPnPError        func(string, error)
	OnCloseError       func(error)
	UPnP               UPnPMapper
}

type Result struct {
	Method   string
	Mapping  stun.Mapping
	Target   netip.AddrPort
	Unstable bool
	Ports    PortReport
}

type PortReport struct {
	Checked    bool
	TargetLAN  PortResult
	QNatterLAN PortResult
	OuterLAN   PortResult
	OuterWAN   PortResult
}

type Session struct {
	Result    Result
	Forwarder forward.Forwarder
	KeepAlive KeepAlive
	UPnP      UPnPMapper
}

// Close 释放本次会话持有的全部资源：转发规则、keep-alive socket 与
// 路由器上的 UPnP 端口映射。重复调用是安全的，返回第一个错误。
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
	if s.UPnP != nil {
		// 撤销路由器上的映射，避免端口被占到租约自然过期。
		ctx, cancel := context.WithTimeout(context.Background(), upnpRevokeTimeout)
		err := s.UPnP.Revoke(ctx)
		cancel()
		if err != nil && firstErr == nil {
			firstErr = err
		}
		s.UPnP = nil
	}
	return firstErr
}

func RunOnce(ctx context.Context, cfg config.Config, deps Dependencies) (Result, error) {
	session, err := StartSession(ctx, cfg, deps)
	if err != nil {
		return Result{}, err
	}
	if err := session.Close(); err != nil {
		return session.Result, err
	}
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
	// 从这里开始已经持有资源（socket / 转发规则 / UPnP 映射）：任何
	// 失败路径都必须回收，统一由下面的 defer 负责，避免泄漏 fd、规则
	// 与路由器端口映射。
	session := &Session{KeepAlive: keepAlive}
	committed := false
	defer func() {
		if committed {
			return
		}
		if err := session.Close(); err != nil && deps.OnCloseError != nil {
			deps.OnCloseError(err)
		}
	}()

	if err := keepAlive.KeepAlive(); err != nil {
		return nil, err
	}
	// first.Inner 是 keep-alive socket 绑定的本地地址/端口：NewKeepAlive
	// 用它作为 Source，注入 deps.KeepAlive 时按同一契约由调用方保证。
	// 后面所有决策都必须与它一致（G-14）。
	observed, err := deps.STUN.GetMapping(ctx)
	if err != nil {
		return nil, err
	}
	if !observed.Inner.IsValid() {
		observed.Inner = first.Inner
	}
	mapping := observed
	if first.Inner.IsValid() && observed.Inner.IsValid() && observed.Inner != first.Inner {
		// G-14：第二次查询换了本地源端口，则 observed.Outer 属于另一个本地
		// 端口，而 keep-alive socket 保活的是 first.Inner 上的映射。继续按
		// observed 下发转发规则，等于用一个"没人保活、也不是监听端口"的映射
		// 对外宣称成功（报文会变成黑洞）。这里统一回到第一次观测 ——
		// Inner/Outer 取同一对，keep-alive、转发监听与上报三者一致 ——
		// 并把这次不一致通过 Unstable 暴露给用户。
		mapping = first
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
	session.Forwarder = fwd

	bind, err := BindFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	target := resolveTarget(cfg, method, mapping)
	options := forward.StartOptions{
		IP:            mapping.Inner.Addr().String(),
		Port:          int(mapping.Inner.Port()),
		TargetIP:      target.Addr().String(),
		TargetPort:    int(target.Port()),
		UDP:           cfg.UDP,
		Interface:     bind.Interface,
		RouteMark:     cfg.RouteMark,
		RouteTable:    cfg.RouteTable,
		RoutePriority: cfg.RoutePriority,
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
			// 映射一建立就挂到 session 上：此后任何失败路径（Notify、
			// 端口检查、targetClosed 重试）都会撤销它。
			session.UPnP = deps.UPnP
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
		Unstable: unstableMapping(first, observed),
		Ports:    portReport,
	}
	if deps.OnMapped != nil {
		deps.OnMapped(result)
	}

	session.Result = result
	session.UPnP = activeUPnP
	if cfg.RetryTarget && !cfg.UDP && targetClosed(ctx, deps, session, portReport) {
		// defer 负责回收 forwarder / keep-alive / UPnP 映射。
		return nil, ErrTargetClosed
	}
	committed = true
	return session, nil
}

// unstableMapping 报告两次 STUN 观测是否不一致：Outer 变化说明 NAT 映射在
// 漂移；Inner 变化说明本地源端口变了 —— keep-alive socket 保活的映射与
// 转发监听使用的映射已经不是同一对。两者都必须上报，否则用户只看到
// "网络正常"（G-14：以前只比较 Outer，Inner 变化完全不上报）。
func unstableMapping(first, second stun.Mapping) bool {
	if first.Inner.IsValid() && second.Inner.IsValid() && first.Inner != second.Inner {
		return true
	}
	return first.Outer.IsValid() && second.Outer.IsValid() && first.Outer != second.Outer
}

func checkInitialPorts(ctx context.Context, cfg config.Config, deps Dependencies, target netip.AddrPort, mapping stun.Mapping) PortReport {
	if cfg.UDP || deps.InitialCheck == nil {
		return PortReport{}
	}
	checker := deps.InitialCheck
	source := mapping.Inner.Addr()
	return PortReport{
		Checked:    true,
		TargetLAN:  checker.TestLAN(ctx, target, source),
		QNatterLAN: checker.TestLAN(ctx, mapping.Inner, source),
		OuterLAN:   checker.TestLAN(ctx, mapping.Outer, source),
		OuterWAN:   checker.TestWAN(ctx, int(mapping.Outer.Port()), source),
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
