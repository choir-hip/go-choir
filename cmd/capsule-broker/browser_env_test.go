//go:build linux

package main

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// Jobs (where an engineer runs Playwright) get the store Chromium and the
// fontconfig file from the broker's environment on every computer.
func TestJobsCarryTheBrowserKeys(t *testing.T) {
	t.Setenv(capsule.BrowserEnv, "/nix/store/x-chromium/bin/chromium")
	t.Setenv(capsule.FontconfigEnv, "/nix/store/x-fonts.conf")
	jobs := strings.Join(newJobManager("/run/capsule/broker", false).Env, "\n")
	for _, want := range []string{capsule.BrowserEnv + "=/nix/store/x-chromium/bin/chromium", capsule.FontconfigEnv + "=/nix/store/x-fonts.conf"} {
		if !strings.Contains(jobs, want) {
			t.Errorf("job env lacks %q: %q", want, jobs)
		}
	}
}
