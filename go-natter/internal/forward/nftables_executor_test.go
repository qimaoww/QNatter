package forward

import (
	"errors"
	"strings"
	"testing"
)

func TestNftablesForwarderStartAndStopDNAT(t *testing.T) {
	runner := &fakeNftRunner{
		outputs: map[string]string{
			NftablesDNATRule(nftTestOptions()): "insert rule ip natter natter_dnat # handle 42\n",
		},
	}
	f := testNftablesForwarder(runner)

	if err := f.Start(nftTestOptions()); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if f.DNATHandle != 42 {
		t.Fatalf("DNATHandle = %d, want 42", f.DNATHandle)
	}
	if err := f.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	wantCalls := []string{
		"list table ip natter",
		NftablesDNATRule(nftTestOptions()),
		"delete rule ip natter natter_dnat handle 42",
	}
	assertNftCalls(t, runner.calls, wantCalls)
}

func TestNftablesForwarderMarksRepliesForBoundInterface(t *testing.T) {
	options := nftTestOptions()
	options.Interface = "pppoe-wan_cmcc"
	policy := natterRoutePolicy(options.Interface)
	runner := &fakeNftRunner{
		outputs: map[string]string{
			NftablesDNATRule(options):                   "insert rule ip natter natter_dnat # handle 42\n",
			NftablesRouteMarkRule(options, policy.Mark): "insert rule ip natter natter_mark # handle 88\n",
		},
	}
	ip := &fakeIPRunner{}
	f := testNftablesForwarder(runner)
	f.RunIP = ip.Run

	if err := f.Start(options); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if f.RouteMarkHandle != 88 {
		t.Fatalf("RouteMarkHandle = %d, want 88", f.RouteMarkHandle)
	}
	if err := f.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	wantCalls := []string{
		"list table ip natter",
		NftablesDNATRule(options),
		"list chain ip natter natter_mark",
		NftablesRouteMarkRule(options, policy.Mark),
		"delete rule ip natter natter_dnat handle 42",
		"delete rule ip natter natter_mark handle 88",
	}
	assertNftCalls(t, runner.calls, wantCalls)
	assertIPCalls(t, ip.calls, []string{
		"route replace default dev pppoe-wan_cmcc table " + policy.Table,
		"rule del priority " + policy.Priority + " fwmark " + policy.Mark + " lookup " + policy.Table,
		"rule add priority " + policy.Priority + " fwmark " + policy.Mark + " lookup " + policy.Table,
	})
}

func TestNftablesForwarderStartAndStopSNAT(t *testing.T) {
	options := nftTestOptions()
	snatOptions := options
	snatOptions.SNATIP = "10.10.10.1"
	runner := &fakeNftRunner{
		outputs: map[string]string{
			NftablesDNATRule(options):     "insert rule ip natter natter_dnat # handle 42\n",
			NftablesSNATRule(snatOptions): "insert rule ip natter natter_snat # handle 77\n",
		},
	}
	f := testNftablesForwarder(runner)
	f.SNAT = true
	f.RouteSourceIP = func(target string) (string, error) {
		if target != options.TargetIP {
			t.Fatalf("RouteSourceIP target = %q, want %q", target, options.TargetIP)
		}
		return snatOptions.SNATIP, nil
	}

	if err := f.Start(options); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if f.DNATHandle != 42 || f.SNATHandle != 77 {
		t.Fatalf("handles = %d/%d, want 42/77", f.DNATHandle, f.SNATHandle)
	}
	if err := f.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	wantCalls := []string{
		"list table ip natter",
		NftablesDNATRule(options),
		NftablesSNATRule(snatOptions),
		"delete rule ip natter natter_dnat handle 42",
		"delete rule ip natter natter_snat handle 77",
	}
	assertNftCalls(t, runner.calls, wantCalls)
}

func TestNftablesForwarderCreatesTableWhenMissing(t *testing.T) {
	options := nftTestOptions()
	runner := &fakeNftRunner{
		errors: map[string]error{
			"list table ip natter": errors.New("table missing"),
		},
		outputs: map[string]string{
			NftablesDNATRule(options): "insert rule ip natter natter_dnat # handle 42\n",
		},
	}
	f := testNftablesForwarder(runner)

	if err := f.Start(options); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if len(runner.calls) < 2 || !strings.Contains(runner.calls[1], "table ip natter") {
		t.Fatalf("second nft call should create table, calls = %#v", runner.calls)
	}
}

func TestNftablesForwarderChecksVersionBeforeRules(t *testing.T) {
	runner := &fakeNftRunner{}
	f := &NftablesForwarder{
		Runner: runner,
		CheckVersion: func() error {
			return errors.New("nftables >= (0, 9, 6) not available")
		},
	}

	err := f.Start(nftTestOptions())
	if err == nil {
		t.Fatalf("Start returned nil error")
	}
	if !strings.Contains(err.Error(), "0, 9, 6") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("nft calls = %#v, want none before version check passes", runner.calls)
	}
}

func TestDefaultNftablesVersionCheckerParsesSupportedVersion(t *testing.T) {
	checker := DefaultNftablesVersionChecker{
		Output: func(name string, args ...string) (string, error) {
			if name != "nft" || len(args) != 1 || args[0] != "--version" {
				t.Fatalf("command = %s %v, want nft --version", name, args)
			}
			return "nftables v1.0.8 (Old Doc Yak #3)\n", nil
		},
	}

	if err := checker.Check(); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestDefaultNftablesVersionCheckerRejectsOldVersion(t *testing.T) {
	checker := DefaultNftablesVersionChecker{
		Output: func(name string, args ...string) (string, error) {
			return "nftables v0.9.5 (Topsy)\n", nil
		},
	}

	if err := checker.Check(); err == nil {
		t.Fatalf("old nftables version accepted")
	}
}

func TestNftablesForwarderRejectsCrossIPForwardWhenIPForwardDisabled(t *testing.T) {
	options := nftTestOptions()
	runner := &fakeNftRunner{}
	f := &NftablesForwarder{
		Runner:       runner,
		CheckVersion: func() error { return nil },
		ReadIPForward: func() (string, error) {
			return "0\n", nil
		},
	}

	err := f.Start(options)
	if err == nil {
		t.Fatalf("Start returned nil error")
	}
	if !strings.Contains(err.Error(), "IP forwarding is not allowed") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("nft calls = %#v, want none before ip_forward passes", runner.calls)
	}
}

func TestNftablesForwarderSkipsIPForwardCheckForSameIPForward(t *testing.T) {
	options := nftTestOptions()
	options.TargetIP = options.IP
	runner := &fakeNftRunner{
		outputs: map[string]string{
			NftablesDNATRule(options): "insert rule ip natter natter_dnat # handle 42\n",
		},
	}
	f := &NftablesForwarder{
		Runner:       runner,
		CheckVersion: func() error { return nil },
		ReadIPForward: func() (string, error) {
			t.Fatalf("ReadIPForward should not be called when source and target IP match")
			return "", nil
		},
	}

	if err := f.Start(options); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if f.DNATHandle != 42 {
		t.Fatalf("DNATHandle = %d, want 42", f.DNATHandle)
	}
}

func TestNftablesForwarderRollsBackDNATWhenSNATFails(t *testing.T) {
	options := nftTestOptions()
	snatOptions := options
	snatOptions.SNATIP = "10.10.10.1"
	runner := &fakeNftRunner{
		errors: map[string]error{
			NftablesSNATRule(snatOptions): errors.New("snat failed"),
		},
		outputs: map[string]string{
			NftablesDNATRule(options): "insert rule ip natter natter_dnat # handle 42\n",
		},
	}
	f := testNftablesForwarder(runner)
	f.SNAT = true
	f.RouteSourceIP = func(target string) (string, error) {
		return snatOptions.SNATIP, nil
	}

	if err := f.Start(options); err == nil {
		t.Fatal("Start returned nil when SNAT insertion failed")
	}

	wantDelete := "delete rule ip natter natter_dnat handle 42"
	if runner.calls[len(runner.calls)-1] != wantDelete {
		t.Fatalf("last call = %q, want rollback %q", runner.calls[len(runner.calls)-1], wantDelete)
	}
	if f.DNATHandle != 0 || f.SNATHandle != 0 {
		t.Fatalf("handles after rollback = %d/%d, want 0/0", f.DNATHandle, f.SNATHandle)
	}
}

func TestParseRouteSourceIP(t *testing.T) {
	source, err := parseRouteSourceIP("10.10.10.180 dev eth1 src 10.10.10.1 uid 0")
	if err != nil {
		t.Fatalf("parseRouteSourceIP returned error: %v", err)
	}
	if source != "10.10.10.1" {
		t.Fatalf("source = %q, want 10.10.10.1", source)
	}
}

func TestParseRouteSourceIPRejectsMissingSource(t *testing.T) {
	if _, err := parseRouteSourceIP("10.10.10.180 dev eth1"); err == nil {
		t.Fatal("parseRouteSourceIP accepted route without src")
	}
}

func nftTestOptions() StartOptions {
	return StartOptions{
		IP:         "198.51.100.73",
		Port:       8931,
		TargetIP:   "10.10.10.10",
		TargetPort: 51413,
	}
}

func testNftablesForwarder(runner Runner) *NftablesForwarder {
	return &NftablesForwarder{
		Runner:       runner,
		CheckVersion: func() error { return nil },
	}
}

type fakeNftRunner struct {
	calls   []string
	outputs map[string]string
	errors  map[string]error
}

func (r *fakeNftRunner) Run(command string) (string, error) {
	r.calls = append(r.calls, command)
	if err := r.errors[command]; err != nil {
		return "", err
	}
	return r.outputs[command], nil
}

type fakeIPRunner struct {
	calls []string
}

func (r *fakeIPRunner) Run(args ...string) (string, error) {
	r.calls = append(r.calls, strings.Join(args, " "))
	return "", nil
}

func assertIPCalls(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ip calls = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ip call %d = %q, want %q (all calls %#v)", i, got[i], want[i], got)
		}
	}
}

func assertNftCalls(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("call count = %d, want %d\ncalls=%#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call[%d] = %q, want %q\ncalls=%#v", i, got[i], want[i], got)
		}
	}
}
