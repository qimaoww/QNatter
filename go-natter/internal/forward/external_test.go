package forward

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSocatForwarderBuildsTCPAndUDPCommands(t *testing.T) {
	starter := &fakeProcessStarter{}
	f := &SocatForwarder{Starter: starter, Checker: &fakeVersionChecker{}, Sleep: noStartupSleep}

	if err := f.Start(StartOptions{
		IP:         "10.10.10.3",
		Port:       41000,
		TargetIP:   "10.10.10.10",
		TargetPort: 62000,
	}); err != nil {
		t.Fatalf("Start TCP returned error: %v", err)
	}
	wantTCP := []string{"socat", "TCP4-LISTEN:41000,reuseaddr,fork,max-children=128", "TCP4:10.10.10.10:62000"}
	if !reflect.DeepEqual(starter.commands[0], wantTCP) {
		t.Fatalf("TCP command = %#v, want %#v", starter.commands[0], wantTCP)
	}

	if err := f.Stop(); err != nil {
		t.Fatalf("Stop TCP returned error: %v", err)
	}

	if err := f.Start(StartOptions{
		Port:       41001,
		TargetIP:   "10.10.10.10",
		TargetPort: 62001,
		UDP:        true,
	}); err != nil {
		t.Fatalf("Start UDP returned error: %v", err)
	}
	wantUDP := []string{"socat", "-T60", "UDP4-LISTEN:41001,reuseaddr,fork,max-children=128", "UDP4:10.10.10.10:62001"}
	if !reflect.DeepEqual(starter.commands[1], wantUDP) {
		t.Fatalf("UDP command = %#v, want %#v", starter.commands[1], wantUDP)
	}
}

func TestGostForwarderBuildsTCPAndUDPCommands(t *testing.T) {
	starter := &fakeProcessStarter{}
	f := &GostForwarder{Starter: starter, Checker: &fakeVersionChecker{}, Sleep: noStartupSleep}

	if err := f.Start(StartOptions{
		Port:       41000,
		TargetIP:   "10.10.10.10",
		TargetPort: 62000,
	}); err != nil {
		t.Fatalf("Start TCP returned error: %v", err)
	}
	wantTCP := []string{"gost", "-L=tcp://:41000/10.10.10.10:62000"}
	if !reflect.DeepEqual(starter.commands[0], wantTCP) {
		t.Fatalf("TCP command = %#v, want %#v", starter.commands[0], wantTCP)
	}

	if err := f.Stop(); err != nil {
		t.Fatalf("Stop TCP returned error: %v", err)
	}

	if err := f.Start(StartOptions{
		Port:       41001,
		TargetIP:   "10.10.10.10",
		TargetPort: 62001,
		UDP:        true,
	}); err != nil {
		t.Fatalf("Start UDP returned error: %v", err)
	}
	wantUDP := []string{"gost", "-L=udp://:41001/10.10.10.10:62001?ttl=60s"}
	if !reflect.DeepEqual(starter.commands[1], wantUDP) {
		t.Fatalf("UDP command = %#v, want %#v", starter.commands[1], wantUDP)
	}
}

func TestExternalForwarderStopsProcess(t *testing.T) {
	starter := &fakeProcessStarter{}
	process := &fakeProcess{}
	starter.next = process
	f := &SocatForwarder{Starter: starter, Checker: &fakeVersionChecker{}, Sleep: noStartupSleep}

	if err := f.Start(StartOptions{Port: 41000, TargetIP: "10.10.10.10", TargetPort: 62000}); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := f.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if !process.terminated || !process.waited {
		t.Fatalf("process terminated=%v waited=%v, want both true", process.terminated, process.waited)
	}
}

func TestExternalForwarderRejectsSameAddress(t *testing.T) {
	f := &GostForwarder{Starter: &fakeProcessStarter{}, Checker: &fakeVersionChecker{}, Sleep: noStartupSleep}

	if err := f.Start(StartOptions{
		IP:         "10.10.10.3",
		Port:       41000,
		TargetIP:   "10.10.10.3",
		TargetPort: 41000,
	}); err == nil {
		t.Fatal("Start accepted forwarding to the same address")
	}
}

func TestExternalForwarderChecksToolBeforeStart(t *testing.T) {
	checker := &fakeVersionChecker{err: errors.New("socat >= (1, 7, 2) not available")}
	starter := &fakeProcessStarter{}
	f := &SocatForwarder{Starter: starter, Checker: checker}

	err := f.Start(StartOptions{
		Port:       41000,
		TargetIP:   "10.10.10.10",
		TargetPort: 62000,
	})
	if err == nil {
		t.Fatalf("Start returned nil error")
	}
	if !strings.Contains(err.Error(), "1, 7, 2") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(checker.commands, []string{"socat"}) {
		t.Fatalf("checked commands = %#v", checker.commands)
	}
	if len(starter.commands) != 0 {
		t.Fatalf("starter commands = %#v, want none", starter.commands)
	}
}

func TestExternalForwarderDetectsQuickExitAfterStartupDelay(t *testing.T) {
	process := &fakeProcess{}
	starter := &fakeProcessStarter{next: process}
	var slept []time.Duration
	f := &GostForwarder{
		Starter:      starter,
		Checker:      &fakeVersionChecker{},
		StartupDelay: time.Second,
		Sleep: func(delay time.Duration) {
			slept = append(slept, delay)
			process.exited = true
		},
	}

	err := f.Start(StartOptions{
		Port:       41000,
		TargetIP:   "10.10.10.10",
		TargetPort: 62000,
	})
	if err == nil {
		t.Fatalf("Start returned nil error")
	}
	if !strings.Contains(err.Error(), "gost exited too quickly") {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(slept, []time.Duration{time.Second}) {
		t.Fatalf("slept = %#v, want 1s", slept)
	}
	if !process.terminated || !process.waited {
		t.Fatalf("process terminated=%v waited=%v, want cleanup after quick exit", process.terminated, process.waited)
	}
}

func TestExternalForwarderDefaultStartupDelayMatchesPython(t *testing.T) {
	if got := defaultStartupDelay(0); got != time.Second {
		t.Fatalf("default delay = %s, want 1s", got)
	}
	if got := defaultStartupDelay(250 * time.Millisecond); got != 250*time.Millisecond {
		t.Fatalf("custom delay = %s, want 250ms", got)
	}
}

func TestDefaultVersionCheckerParsesSupportedTools(t *testing.T) {
	checker := DefaultVersionChecker{
		Output: func(name string, args ...string) (string, error) {
			switch name {
			case "socat":
				return "socat by Gerhard Rieger and contributors - socat version 1.7.4.4 on linux\n", nil
			case "gost":
				return "gost 2.11.5 (go1.20 linux/amd64)\n", nil
			default:
				t.Fatalf("unexpected command %s %v", name, args)
				return "", nil
			}
		},
	}

	if err := checker.Check("socat"); err != nil {
		t.Fatalf("socat check returned error: %v", err)
	}
	if err := checker.Check("gost"); err != nil {
		t.Fatalf("gost check returned error: %v", err)
	}
}

func TestDefaultVersionCheckerRejectsOldTools(t *testing.T) {
	checker := DefaultVersionChecker{
		Output: func(name string, args ...string) (string, error) {
			switch name {
			case "socat":
				return "socat version 1.7.1.0\n", nil
			case "gost":
				return "gost v2.2\n", nil
			default:
				t.Fatalf("unexpected command %s %v", name, args)
				return "", nil
			}
		},
	}

	if err := checker.Check("socat"); err == nil {
		t.Fatalf("old socat accepted")
	}
	if err := checker.Check("gost"); err == nil {
		t.Fatalf("old gost accepted")
	}
}

type fakeProcessStarter struct {
	commands [][]string
	next     ExternalProcess
}

func (s *fakeProcessStarter) Start(name string, args ...string) (ExternalProcess, error) {
	command := append([]string{name}, args...)
	s.commands = append(s.commands, command)
	if s.next != nil {
		return s.next, nil
	}
	return &fakeProcess{}, nil
}

type fakeProcess struct {
	terminated bool
	waited     bool
	exited     bool
}

func (p *fakeProcess) Terminate() error {
	p.terminated = true
	return nil
}

func (p *fakeProcess) Kill() error {
	p.terminated = true
	return nil
}

func (p *fakeProcess) Wait() error {
	p.waited = true
	return nil
}

func (p *fakeProcess) Exited() bool {
	return p.exited
}

type fakeVersionChecker struct {
	commands []string
	err      error
}

func (c *fakeVersionChecker) Check(command string) error {
	c.commands = append(c.commands, command)
	return c.err
}

func noStartupSleep(time.Duration) {}
