package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"natter-openwrt/go-natter/internal/config"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{"--version"}, &stdout, &bytes.Buffer{})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	if got := stdout.String(); !strings.Contains(got, "Natter Go") {
		t.Fatalf("version output = %q, want Natter Go", got)
	}
}

func TestRunCheckParsesExistingOpenWrtArguments(t *testing.T) {
	var stdout bytes.Buffer
	code := Run([]string{
		"--check",
		"-k", "15",
		"-s", "turn.cloud-rtc.com:80",
		"-i", "pppoe-wan_cmcc",
		"-b", "0",
		"-m", "none",
		"-t", "0.0.0.0",
		"-p", "0",
		"-e", "/var/run/natter/cmcc_.notify",
	}, &stdout, &bytes.Buffer{})

	if code != 0 {
		t.Fatalf("Run returned code %d, want 0", code)
	}
	out := stdout.String()
	for _, want := range []string{
		"check: ok",
		"bind=pppoe-wan_cmcc:0",
		"stun=turn.cloud-rtc.com:80",
		"method=none",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("check output = %q, missing %q", out, want)
		}
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	code := Run([]string{"-s", "bad:port"}, &bytes.Buffer{}, &bytes.Buffer{})

	if code == 0 {
		t.Fatal("Run accepted invalid arguments")
	}
}

func TestRunWithStartsEngineForMapping(t *testing.T) {
	var got config.Config
	code := RunWith([]string{
		"-i", "pppoe-wan_cmcc",
		"-m", "none",
	}, &bytes.Buffer{}, &bytes.Buffer{}, func(cfg config.Config) error {
		got = cfg
		return nil
	})

	if code != 0 {
		t.Fatalf("RunWith returned code %d, want 0", code)
	}
	if got.BindValue != "pppoe-wan_cmcc" || got.ForwardMethod != "none" {
		t.Fatalf("runner config = %+v", got)
	}
}

func TestRunWithReportsEngineError(t *testing.T) {
	var stderr bytes.Buffer
	code := RunWith(nil, &bytes.Buffer{}, &stderr, func(config.Config) error {
		return errors.New("mapping failed")
	})

	if code != 1 {
		t.Fatalf("RunWith returned code %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "mapping failed") {
		t.Fatalf("stderr = %q, want mapping failed", stderr.String())
	}
}
