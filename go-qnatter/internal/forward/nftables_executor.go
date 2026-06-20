package forward

import (
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

type Runner interface {
	Run(command string) (string, error)
}

type NftRunner struct{}

func (NftRunner) Run(command string) (string, error) {
	output, err := exec.Command("nft", "--echo", "--handle", command).CombinedOutput()
	return string(output), err
}

type NftablesForwarder struct {
	Runner          Runner
	SNAT            bool
	DNATHandle      int
	SNATHandle      int
	ConnMarkHandle  int
	RouteMarkHandle int
	ReadIPForward   func() (string, error)
	CheckVersion    func() error
	RouteSourceIP   func(string) (string, error)
	RunIP           func(args ...string) (string, error)
}

func (f *NftablesForwarder) Start(options StartOptions) error {
	if options.IP == options.TargetIP && options.Port == options.TargetPort {
		return fmt.Errorf("cannot forward to the same address %s:%d", options.IP, options.Port)
	}
	if err := f.checkVersion(); err != nil {
		return err
	}
	if err := f.checkIPForward(options); err != nil {
		return err
	}

	runner := f.runner()
	if _, err := runner.Run("list table ip qnatter"); err != nil {
		if _, createErr := runner.Run(NftablesInitialRules()); createErr != nil {
			return createErr
		}
	}

	output, err := runner.Run(NftablesDNATRule(options))
	if err != nil {
		return err
	}
	handle, err := ParseNftablesHandle(output)
	if err != nil {
		return err
	}
	f.DNATHandle = handle

	if f.SNAT {
		snatOptions := options
		if snatOptions.SNATIP == "" {
			if sourceIP, sourceErr := f.routeSourceIP(options.TargetIP); sourceErr == nil && sourceIP != "" {
				snatOptions.SNATIP = sourceIP
			}
		}
		output, err = runner.Run(NftablesSNATRule(snatOptions))
		if err != nil {
			_ = f.Stop()
			return err
		}
		handle, err = ParseNftablesHandle(output)
		if err != nil {
			_ = f.Stop()
			return err
		}
		f.SNATHandle = handle
	}
	if options.Interface != "" {
		policy := routePolicyFromOptions(options)
		if err := f.ensureRoutePolicy(options.Interface, policy); err != nil {
			_ = f.Stop()
			return err
		}
		if err := f.ensureRouteMarkChain(runner); err != nil {
			_ = f.Stop()
			return err
		}
		output, err = runner.Run(NftablesConnMarkRule(options, policy.Mark))
		if err != nil {
			_ = f.Stop()
			return err
		}
		handle, err = ParseNftablesHandle(output)
		if err != nil {
			_ = f.Stop()
			return err
		}
		f.ConnMarkHandle = handle

		output, err = runner.Run(NftablesRouteMarkRule(options, policy.Mark))
		if err != nil {
			_ = f.Stop()
			return err
		}
		handle, err = ParseNftablesHandle(output)
		if err != nil {
			_ = f.Stop()
			return err
		}
		f.RouteMarkHandle = handle
	}

	return nil
}

func (f *NftablesForwarder) ensureRouteMarkChain(runner Runner) error {
	if _, err := runner.Run("list chain ip qnatter qnatter_mark"); err == nil {
		return nil
	}
	_, err := runner.Run(NftablesRouteMarkInitialRules())
	return err
}

func (f *NftablesForwarder) ensureRoutePolicy(iface string, policy routePolicy) error {
	if err := f.runIPChecked("route", "replace", "default", "dev", iface, "table", policy.Table); err != nil {
		return err
	}
	_, _ = f.ipRunner()("rule", "del", "priority", policy.Priority, "fwmark", policy.Mark, "lookup", policy.Table)
	return f.runIPChecked("rule", "add", "priority", policy.Priority, "fwmark", policy.Mark, "lookup", policy.Table)
}

func (f *NftablesForwarder) runIPChecked(args ...string) error {
	output, err := f.ipRunner()(args...)
	if err == nil {
		return nil
	}
	text := strings.TrimSpace(output)
	if text == "" {
		return fmt.Errorf("ip %s: %w", strings.Join(args, " "), err)
	}
	return fmt.Errorf("ip %s: %w: %s", strings.Join(args, " "), err, text)
}

func (f *NftablesForwarder) ipRunner() func(args ...string) (string, error) {
	if f.RunIP != nil {
		return f.RunIP
	}
	return ipCommandOutput
}

func (f *NftablesForwarder) routeSourceIP(target string) (string, error) {
	routeSource := f.RouteSourceIP
	if routeSource == nil {
		routeSource = defaultRouteSourceIP
	}
	return routeSource(target)
}

func (f *NftablesForwarder) Stop() error {
	runner := f.runner()
	var firstErr error

	if f.DNATHandle > 0 {
		_, err := runner.Run(fmt.Sprintf("delete rule ip qnatter qnatter_dnat handle %d", f.DNATHandle))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		f.DNATHandle = 0
	}
	if f.SNATHandle > 0 {
		_, err := runner.Run(fmt.Sprintf("delete rule ip qnatter qnatter_snat handle %d", f.SNATHandle))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		f.SNATHandle = 0
	}
	if f.ConnMarkHandle > 0 {
		_, err := runner.Run(fmt.Sprintf("delete rule ip qnatter qnatter_mark handle %d", f.ConnMarkHandle))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		f.ConnMarkHandle = 0
	}
	if f.RouteMarkHandle > 0 {
		_, err := runner.Run(fmt.Sprintf("delete rule ip qnatter qnatter_mark handle %d", f.RouteMarkHandle))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		f.RouteMarkHandle = 0
	}

	return firstErr
}

func (f *NftablesForwarder) runner() Runner {
	if f.Runner != nil {
		return f.Runner
	}
	return NftRunner{}
}

func (f *NftablesForwarder) checkVersion() error {
	check := f.CheckVersion
	if check == nil {
		check = DefaultNftablesVersionChecker{}.Check
	}
	return check()
}

func (f *NftablesForwarder) checkIPForward(options StartOptions) error {
	if options.IP == options.TargetIP {
		return nil
	}
	read := f.ReadIPForward
	if read == nil {
		read = readSystemIPForward
	}
	value, err := read()
	if err != nil {
		return nil
	}
	if strings.TrimSpace(value) != "1" {
		return fmt.Errorf("IP forwarding is not allowed. Please do `sysctl net.ipv4.ip_forward=1`")
	}
	return nil
}

func readSystemIPForward() (string, error) {
	raw, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	return string(raw), err
}

type DefaultNftablesVersionChecker struct {
	Output func(name string, args ...string) (string, error)
}

func (c DefaultNftablesVersionChecker) Check() error {
	output := c.Output
	if output == nil {
		output = nftCommandOutput
	}
	text, err := output("nft", "--version")
	if err != nil {
		return fmt.Errorf("nftables >= (0, 9, 6) not available")
	}
	match := regexp.MustCompile(`nftables v([0-9]+)\.([0-9]+)\.([0-9]+)`).FindStringSubmatch(text)
	if match == nil {
		return fmt.Errorf("nftables >= (0, 9, 6) not available")
	}
	version := make([]int, 0, 3)
	for _, part := range match[1:] {
		value, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("nftables >= (0, 9, 6) not available")
		}
		version = append(version, value)
	}
	if compareVersion(version, []int{0, 9, 6}) < 0 {
		return fmt.Errorf("nftables >= (0, 9, 6) not available")
	}
	return nil
}

func nftCommandOutput(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	return string(output), err
}

func ipCommandOutput(args ...string) (string, error) {
	output, err := exec.Command("ip", args...).CombinedOutput()
	return string(output), err
}

type routePolicy struct {
	Mark     string
	Table    string
	Priority string
}

func qnatterRoutePolicy(slot int) routePolicy {
	return routePolicy{
		Mark:     fmt.Sprintf("0x4e000%03x", slot),
		Table:    strconv.Itoa(20000 + slot),
		Priority: strconv.Itoa(20000 + slot),
	}
}

func routePolicyFromOptions(options StartOptions) routePolicy {
	if options.RouteMark != "" && options.RouteTable != "" && options.RoutePriority != "" {
		return routePolicy{
			Mark:     options.RouteMark,
			Table:    options.RouteTable,
			Priority: options.RoutePriority,
		}
	}
	return qnatterRoutePolicy(0)
}

func defaultRouteSourceIP(target string) (string, error) {
	output, err := exec.Command("ip", "-4", "route", "get", target).CombinedOutput()
	if err != nil {
		return "", err
	}
	return parseRouteSourceIP(string(output))
}

func parseRouteSourceIP(output string) (string, error) {
	fields := strings.Fields(output)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != "src" {
			continue
		}
		if _, err := netip.ParseAddr(fields[i+1]); err != nil {
			return "", err
		}
		return fields[i+1], nil
	}
	return "", fmt.Errorf("route source address not found")
}
