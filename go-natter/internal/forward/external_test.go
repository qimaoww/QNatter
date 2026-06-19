package forward

import (
	"reflect"
	"testing"
)

func TestSocatForwarderBuildsTCPAndUDPCommands(t *testing.T) {
	starter := &fakeProcessStarter{}
	f := &SocatForwarder{Starter: starter}

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
	f := &GostForwarder{Starter: starter}

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
	f := &SocatForwarder{Starter: starter}

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
	f := &GostForwarder{Starter: &fakeProcessStarter{}}

	if err := f.Start(StartOptions{
		IP:         "10.10.10.3",
		Port:       41000,
		TargetIP:   "10.10.10.3",
		TargetPort: 41000,
	}); err == nil {
		t.Fatal("Start accepted forwarding to the same address")
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
	return false
}
