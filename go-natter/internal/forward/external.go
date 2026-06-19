package forward

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

type ExternalProcess interface {
	Terminate() error
	Kill() error
	Wait() error
	Exited() bool
}

type ProcessStarter interface {
	Start(name string, args ...string) (ExternalProcess, error)
}

type VersionChecker interface {
	Check(command string) error
}

type DefaultVersionChecker struct {
	Output func(name string, args ...string) (string, error)
}

type ExecStarter struct{}

func (ExecStarter) Start(name string, args ...string) (ExternalProcess, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &osProcess{cmd: cmd}, nil
}

type osProcess struct {
	cmd *exec.Cmd
}

func (p *osProcess) Terminate() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Signal(os.Interrupt)
}

func (p *osProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}

func (p *osProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *osProcess) Exited() bool {
	return p.cmd.ProcessState != nil && p.cmd.ProcessState.Exited()
}

type SocatForwarder struct {
	Starter     ProcessStarter
	Checker     VersionChecker
	Process     ExternalProcess
	UDPTimeout  int
	MaxChildren int
}

func (f *SocatForwarder) Start(options StartOptions) error {
	if options.IP == options.TargetIP && options.Port == options.TargetPort {
		return fmt.Errorf("cannot forward to the same address %s:%d", options.IP, options.Port)
	}
	timeout := f.UDPTimeout
	if timeout == 0 {
		timeout = 60
	}
	maxChildren := f.MaxChildren
	if maxChildren == 0 {
		maxChildren = 128
	}
	proto := "TCP"
	args := []string{}
	if options.UDP {
		proto = "UDP"
		args = append(args, fmt.Sprintf("-T%d", timeout))
	}
	args = append(args,
		fmt.Sprintf("%s4-LISTEN:%d,reuseaddr,fork,max-children=%d", proto, options.Port, maxChildren),
		fmt.Sprintf("%s4:%s:%d", proto, options.TargetIP, options.TargetPort),
	)
	return f.start("socat", args...)
}

func (f *SocatForwarder) Stop() error {
	return stopExternal(&f.Process)
}

type GostForwarder struct {
	Starter    ProcessStarter
	Checker    VersionChecker
	Process    ExternalProcess
	UDPTimeout int
}

func (f *GostForwarder) Start(options StartOptions) error {
	if options.IP == options.TargetIP && options.Port == options.TargetPort {
		return fmt.Errorf("cannot forward to the same address %s:%d", options.IP, options.Port)
	}
	timeout := f.UDPTimeout
	if timeout == 0 {
		timeout = 60
	}
	proto := "tcp"
	if options.UDP {
		proto = "udp"
	}
	arg := fmt.Sprintf("-L=%s://:%d/%s:%d", proto, options.Port, options.TargetIP, options.TargetPort)
	if options.UDP {
		arg += fmt.Sprintf("?ttl=%ds", timeout)
	}
	return f.start("gost", arg)
}

func (f *GostForwarder) Stop() error {
	return stopExternal(&f.Process)
}

func (f *SocatForwarder) start(name string, args ...string) error {
	if err := checkExternal(f.Checker, name); err != nil {
		return err
	}
	process, err := startExternal(f.Starter, name, args...)
	if err != nil {
		return err
	}
	f.Process = process
	return nil
}

func (f *GostForwarder) start(name string, args ...string) error {
	if err := checkExternal(f.Checker, name); err != nil {
		return err
	}
	process, err := startExternal(f.Starter, name, args...)
	if err != nil {
		return err
	}
	f.Process = process
	return nil
}

func startExternal(starter ProcessStarter, name string, args ...string) (ExternalProcess, error) {
	if starter == nil {
		starter = ExecStarter{}
	}
	process, err := starter.Start(name, args...)
	if err != nil {
		return nil, err
	}
	if process.Exited() {
		_ = process.Kill()
		_ = process.Wait()
		return nil, fmt.Errorf("%s exited too quickly", name)
	}
	return process, nil
}

func checkExternal(checker VersionChecker, name string) error {
	if checker == nil {
		checker = DefaultVersionChecker{}
	}
	return checker.Check(name)
}

func (c DefaultVersionChecker) Check(command string) error {
	switch command {
	case "socat":
		return c.check(command, []string{"-V"}, regexp.MustCompile(`socat version ([0-9]+)\.([0-9]+)\.([0-9]+)`), []int{1, 7, 2})
	case "gost":
		return c.check(command, []string{"-V"}, regexp.MustCompile(`gost v?([0-9]+)\.([0-9]+)`), []int{2, 3})
	default:
		return nil
	}
}

func (c DefaultVersionChecker) check(command string, args []string, pattern *regexp.Regexp, minimum []int) error {
	output := c.Output
	if output == nil {
		output = execCommandOutput
	}
	text, err := output(command, args...)
	if err != nil {
		return fmt.Errorf("%s >= %s not available", command, versionString(minimum))
	}
	match := pattern.FindStringSubmatch(text)
	if match == nil {
		return fmt.Errorf("%s >= %s not available", command, versionString(minimum))
	}
	version := make([]int, 0, len(match)-1)
	for _, part := range match[1:] {
		value, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("%s >= %s not available", command, versionString(minimum))
		}
		version = append(version, value)
	}
	if compareVersion(version, minimum) < 0 {
		return fmt.Errorf("%s >= %s not available", command, versionString(minimum))
	}
	return nil
}

func execCommandOutput(name string, args ...string) (string, error) {
	output, err := exec.Command(name, args...).CombinedOutput()
	return string(output), err
}

func compareVersion(version []int, minimum []int) int {
	maxLen := len(version)
	if len(minimum) > maxLen {
		maxLen = len(minimum)
	}
	for i := 0; i < maxLen; i++ {
		var got int
		if i < len(version) {
			got = version[i]
		}
		var want int
		if i < len(minimum) {
			want = minimum[i]
		}
		if got < want {
			return -1
		}
		if got > want {
			return 1
		}
	}
	return 0
}

func versionString(parts []int) string {
	if len(parts) == 0 {
		return "()"
	}
	value := fmt.Sprintf("(%d", parts[0])
	for _, part := range parts[1:] {
		value += fmt.Sprintf(", %d", part)
	}
	return value + ")"
}

func stopExternal(process *ExternalProcess) error {
	if process == nil || *process == nil {
		return nil
	}
	proc := *process
	*process = nil
	if proc.Exited() {
		return nil
	}
	if err := proc.Terminate(); err != nil {
		return err
	}
	return proc.Wait()
}
