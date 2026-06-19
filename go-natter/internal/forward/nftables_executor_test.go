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
	f := &NftablesForwarder{Runner: runner}

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

func TestNftablesForwarderStartAndStopSNAT(t *testing.T) {
	options := nftTestOptions()
	runner := &fakeNftRunner{
		outputs: map[string]string{
			NftablesDNATRule(options): "insert rule ip natter natter_dnat # handle 42\n",
			NftablesSNATRule(options): "insert rule ip natter natter_snat # handle 77\n",
		},
	}
	f := &NftablesForwarder{Runner: runner, SNAT: true}

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
		NftablesSNATRule(options),
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
	f := &NftablesForwarder{Runner: runner}

	if err := f.Start(options); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if len(runner.calls) < 2 || !strings.Contains(runner.calls[1], "table ip natter") {
		t.Fatalf("second nft call should create table, calls = %#v", runner.calls)
	}
}

func TestNftablesForwarderRollsBackDNATWhenSNATFails(t *testing.T) {
	options := nftTestOptions()
	runner := &fakeNftRunner{
		errors: map[string]error{
			NftablesSNATRule(options): errors.New("snat failed"),
		},
		outputs: map[string]string{
			NftablesDNATRule(options): "insert rule ip natter natter_dnat # handle 42\n",
		},
	}
	f := &NftablesForwarder{Runner: runner, SNAT: true}

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

func nftTestOptions() StartOptions {
	return StartOptions{
		IP:         "198.51.100.73",
		Port:       8931,
		TargetIP:   "10.10.10.10",
		TargetPort: 51413,
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
