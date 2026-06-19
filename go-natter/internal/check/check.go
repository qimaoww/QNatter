package check

import (
	"context"
	"fmt"
	"io"

	"natter-openwrt/go-natter/internal/config"
)

const Version = "2.2.1-go"

type Status int

const (
	NA Status = iota
	OK
	COMPAT
	FAIL
)

type Result struct {
	Status Status
	Info   string
}

type Probe func(context.Context, config.Config) (Result, error)

type Runner struct {
	TCP Probe
	UDP Probe
}

func Run(ctx context.Context, cfg config.Config, stdout io.Writer, stderr io.Writer) error {
	return Runner{
		TCP: unimplementedTCP,
		UDP: unimplementedUDP,
	}.Run(ctx, cfg, stdout)
}

func (r Runner) Run(ctx context.Context, cfg config.Config, stdout io.Writer) error {
	fmt.Fprintf(stdout, "> NatterCheck v%s\n\n", Version)
	r.printInfo(ctx, cfg, stdout, "Checking TCP NAT...", r.TCP)
	r.printInfo(ctx, cfg, stdout, "Checking UDP NAT...", r.UDP)
	return nil
}

func (r Runner) printInfo(ctx context.Context, cfg config.Config, stdout io.Writer, label string, probe Probe) {
	fmt.Fprintf(stdout, "%-36s ", label)
	result, err := probe(ctx, cfg)
	if err != nil {
		result = Result{Status: FAIL, Info: err.Error()}
	}
	fmt.Fprintf(stdout, "%s ... %s\n", result.Status.rep(), result.Info)
}

func (s Status) rep() string {
	switch s {
	case NA:
		return "[   NA   ]"
	case OK:
		return "[   OK   ]"
	case COMPAT:
		return "[ COMPAT ]"
	case FAIL:
		return "[  FAIL  ]"
	default:
		return "[  FAIL  ]"
	}
}

func unimplementedTCP(context.Context, config.Config) (Result, error) {
	return Result{}, fmt.Errorf("Go TCP NAT check is not implemented yet")
}

func unimplementedUDP(context.Context, config.Config) (Result, error) {
	return Result{}, fmt.Errorf("Go UDP NAT check is not implemented yet")
}
