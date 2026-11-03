package capsule

import (
	"os"
	"path/filepath"
	"testing"
)

// The browser is not a privilege (docs/problems/selfdev-screens-are-not-files-2026-11-03.md):
// every computer's capsule broker carries the store Chromium's path and the
// fontconfig file when the image names an executable Chromium, with no grant.
// What can go wrong: the image names no Chromium, or a path that is missing,
// not executable or a directory (each must give no browser key rather than a
// path Playwright then fails on); a blank fontconfig must not be passed.

func fakeChromium(t *testing.T, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "chromium")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBrowserBrokerEnvNeedsOnlyTheImageBinary(t *testing.T) {
	browser := fakeChromium(t, 0o755)
	env := browserBrokerEnv(browser, " /nix/store/x-fonts.conf ")
	if len(env) != 2 || env[0] != BrowserEnv+"="+browser || env[1] != FontconfigEnv+"=/nix/store/x-fonts.conf" {
		t.Fatalf("env: %q", env)
	}
	if env := browserBrokerEnv(browser, " "); len(env) != 1 {
		t.Fatalf("no fontconfig named: %q", env)
	}
	for name, path := range map[string]string{
		"image names no Chromium":         "",
		"Chromium missing from the image": filepath.Join(t.TempDir(), "absent"),
		"Chromium not executable":         fakeChromium(t, 0o644),
		"Chromium is a directory":         t.TempDir(),
	} {
		if env := browserBrokerEnv(path, "/nix/store/x-fonts.conf"); env != nil {
			t.Errorf("%s: env %q", name, env)
		}
	}
}

func TestBrowserJobEnvPassesOnlyTheBrowserKeys(t *testing.T) {
	values := map[string]string{BrowserEnv: " /nix/store/x-chromium/bin/chromium ", FontconfigEnv: "/nix/store/x-fonts.conf", "OTHER": "x"}
	lookup := func(k string) (string, bool) { v, ok := values[k]; return v, ok }
	env := BrowserJobEnv(lookup)
	if len(env) != 2 || env[0] != BrowserEnv+"=/nix/store/x-chromium/bin/chromium" || env[1] != FontconfigEnv+"=/nix/store/x-fonts.conf" {
		t.Fatalf("job env: %q", env)
	}
	values[BrowserEnv] = "/bin/sh\nEVIL=1"
	if env := BrowserJobEnv(lookup); len(env) != 1 {
		t.Fatalf("a value with a newline crossed: %q", env)
	}
}
