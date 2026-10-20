//go:build linux

package main

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// V1 (docs/design/visual-evidence-slice-2026-10-15.md): the Chromium path the
// executor gives a granted computer's broker reaches the broker's jobs and
// nothing else it runs.

func TestOnlyJobsCarryTheCaptureKeys(t *testing.T) {
	t.Setenv(visualcapture.BrowserEnv, "/nix/store/x-chromium/bin/chromium")
	t.Setenv(visualcapture.FontconfigEnv, "/nix/store/x-fonts.conf")
	lookup := func(k string) (string, bool) {
		switch k {
		case visualcapture.BrowserEnv:
			return "/nix/store/x-chromium/bin/chromium", true
		case visualcapture.FontconfigEnv:
			return "/nix/store/x-fonts.conf", true
		}
		return "", false
	}

	jobs := strings.Join(newJobManager("/run/capsule/broker", false).Env, "\n")
	for _, want := range []string{visualcapture.BrowserEnv + "=/nix/store/x-chromium/bin/chromium", visualcapture.FontconfigEnv + "=/nix/store/x-fonts.conf"} {
		if !strings.Contains(jobs, want) {
			t.Errorf("job env lacks %q: %q", want, jobs)
		}
	}

	// Direct exec, session workers and cells build their environments from
	// allowlists; a caller cannot ask for the keys either.
	direct := strings.Join(directExecEnvFrom(lookup, []string{visualcapture.BrowserEnv + "=/bin/sh", "CHOIR_CAPTURE_X=1"}), "\n")
	worker := strings.Join(sessionWorkerEnv(false, lookup), "\n")
	for name, env := range map[string]string{"direct exec": direct, "session worker": worker} {
		if strings.Contains(env, "CAPTURE") || strings.Contains(env, "chromium") {
			t.Errorf("%s env carries capture settings: %q", name, env)
		}
	}
	for _, kv := range yaegikernel.BuildToolchainCarry(lookup) {
		if strings.Contains(kv, "CAPTURE") {
			t.Errorf("toolchain carry forwards %q", kv)
		}
	}
}

func TestAnUngrantedBrokerGivesJobsNoCaptureKeys(t *testing.T) {
	t.Setenv(visualcapture.BrowserEnv, "")
	t.Setenv(visualcapture.FontconfigEnv, "")
	if env := strings.Join(newJobManager("/run/capsule/broker", false).Env, "\n"); strings.Contains(env, "CAPTURE") {
		t.Fatalf("job env carries capture settings without a grant: %q", env)
	}
}
