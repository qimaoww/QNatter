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
	Runner        Runner
	SNAT          bool
	DNATHandle    int
	SNATHandle    int
	ReadIPForward func() (string, error)
	CheckVersion  func() error
	RouteSourceIP func(string) (string, error)
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
	if _, err := runner.Run("list table ip natter"); err != nil {
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

	return nil
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
		_, err := runner.Run(fmt.Sprintf("delete rule ip natter natter_dnat handle %d", f.DNATHandle))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		f.DNATHandle = 0
	}
	if f.SNATHandle > 0 {
		_, err := runner.Run(fmt.Sprintf("delete rule ip natter natter_snat handle %d", f.SNATHandle))
		if err != nil && firstErr == nil {
			firstErr = err
		}
		f.SNATHandle = 0
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
