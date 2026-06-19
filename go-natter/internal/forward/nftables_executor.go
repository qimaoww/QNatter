package forward

import (
	"fmt"
	"os"
	"os/exec"
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
}

func (f *NftablesForwarder) Start(options StartOptions) error {
	if options.IP == options.TargetIP && options.Port == options.TargetPort {
		return fmt.Errorf("cannot forward to the same address %s:%d", options.IP, options.Port)
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
		output, err = runner.Run(NftablesSNATRule(options))
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
