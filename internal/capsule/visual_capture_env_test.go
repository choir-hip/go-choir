package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// The gate (docs/design/visual-evidence-slice-2026-10-15.md V1, V4): the
// Chromium path reaches a capsule broker only when the host's grant is on the
// credential disk and the image names an executable Chromium, and only jobs
// get it from the broker.

func writeGrant(t *testing.T, dir, status string) {
	t.Helper()
	body := `{"computer_id":"computer-test-1","status":"` + status + `","basis":"computer","granted_at":"2026-10-20T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(dir, visualcapture.GrantFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fakeChromium(t *testing.T, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chromium")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCaptureBrokerEnvNeedsTheGrantAndTheImageBinary(t *testing.T) {
	browser := fakeChromium(t, 0o755)

	// Granted: the browser and the fontconfig file, nothing else.
	dir := t.TempDir()
	writeGrant(t, dir, visualcapture.GrantGranted)
	env := captureBrokerEnv(dir, browser, " /nix/store/x-fonts.conf ")
	if len(env) != 2 || env[0] != visualcapture.BrowserEnv+"="+browser || env[1] != visualcapture.FontconfigEnv+"=/nix/store/x-fonts.conf" {
		t.Fatalf("granted env: %q", env)
	}
	if env := captureBrokerEnv(dir, browser, ""); len(env) != 1 {
		t.Fatalf("no fontconfig named: %q", env)
	}

	denied := map[string][]string{
		"no grant on the credential disk": {t.TempDir(), browser},
		"no credential directory":         {"", browser},
		"image names no Chromium":         {dir, ""},
		"Chromium missing from the image": {dir, filepath.Join(t.TempDir(), "absent")},
		"Chromium not executable":         {dir, fakeChromium(t, 0o644)},
		"Chromium is a directory":         {dir, t.TempDir()},
	}
	for name, args := range denied {
		if env := captureBrokerEnv(args[0], args[1], "/nix/store/x-fonts.conf"); env != nil {
			t.Errorf("%s: env %q", name, env)
		}
	}

	withheld := t.TempDir()
	writeGrant(t, withheld, "withheld")
	if env := captureBrokerEnv(withheld, browser, ""); env != nil {
		t.Errorf("a withheld grant gave env %q", env)
	}
	malformed := t.TempDir()
	if err := os.WriteFile(filepath.Join(malformed, visualcapture.GrantFile), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if env := captureBrokerEnv(malformed, browser, ""); env != nil {
		t.Errorf("a malformed grant gave env %q", env)
	}
}

func TestCaptureJobEnvCarriesOnlyTheCaptureKeys(t *testing.T) {
	autoputer := map[string]string{
		visualcapture.BrowserEnv:    "/nix/store/x-chromium/bin/chromium",
		visualcapture.FontconfigEnv: "/nix/store/x-fonts.conf",
		"RUNTIME_GATEWAY_TOKEN":     "secret-token",
		"CHOIR_CLAUDE_DELEGATE":     "x",
	}
	lookup := func(k string) (string, bool) { v, ok := autoputer[k]; return v, ok }
	env := CaptureJobEnv(lookup)
	if strings.Join(env, "\n") != visualcapture.BrowserEnv+"=/nix/store/x-chromium/bin/chromium\n"+visualcapture.FontconfigEnv+"=/nix/store/x-fonts.conf" {
		t.Fatalf("job env: %q", env)
	}
	if env := CaptureJobEnv(func(string) (string, bool) { return "", false }); len(env) != 0 {
		t.Fatalf("an ungranted broker gave jobs %q", env)
	}
	for _, bad := range []string{"/nix/store/x\nINJECTED=1", "bad\x00path", "   "} {
		autoputer[visualcapture.BrowserEnv] = bad
		if got := CaptureJobEnv(lookup); strings.Contains(strings.Join(got, "\n"), "INJECTED") || strings.Contains(strings.Join(got, "\n"), "bad") || strings.Contains(strings.Join(got, "\n"), visualcapture.BrowserEnv) {
			t.Errorf("value %q reached a job: %q", bad, got)
		}
	}
}

// V1: the broker's own launch environment and the cell and direct-exec
// allowlists never carry the capture keys on their own; only the job manager
// adds them.
func TestBrokerProcessEnvDoesNotForwardCaptureKeysFromTheAutoputer(t *testing.T) {
	autoputer := map[string]string{
		"PATH":                                "/nix/store/x-busybox/bin",
		visualcapture.BrowserEnv:              "/nix/store/x-chromium/bin/chromium",
		visualcapture.FontconfigEnv:           "/nix/store/x-fonts.conf",
		"CHOIR_CAPTURE_BROWSER_BIN":           "/nix/store/x-chromium/bin/chromium",
		"CHOIR_VISUAL_CAPTURE_CREDENTIAL_DIR": "/run/choir-bootstrap",
	}
	env := BrokerProcessEnv(func(k string) (string, bool) { v, ok := autoputer[k]; return v, ok })
	if joined := strings.Join(env, "\n"); strings.Contains(joined, "CAPTURE") || strings.Contains(joined, "chromium") {
		t.Fatalf("broker launch env leaked capture settings: %q", env)
	}
}
