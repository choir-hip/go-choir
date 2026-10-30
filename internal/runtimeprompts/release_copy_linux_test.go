//go:build linux

package runtimeprompts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// The engineering overlay's release-copy step must work under the capsule's
// real constraints (docs/archive/problems/adopt-1418-adopter-desks-finish-without-an-operation-2026-10-15.md):
// the build output carries 0444 modes (vite copies frontend/public files that
// makeSubjectTreeReadOnly made read-only), and a first copy that exited
// non-zero (cp -a: the capsule's seccomp refuses the xattr/ACL reads, EPERM)
// leaves those 0444 files in the release tree, where a plain cp -r retry
// cannot open them for writing (EACCES). This runs the overlay's own copy
// command twice as a non-root user over such a tree, and pins that the plain
// cp -r retry it replaces fails on the same tree.
func TestEngineeringOverlayReleaseCopyIsRetrySafeOverReadOnlyOutput(t *testing.T) {
	overlay := RLMEngineeringOverlay(RLMEngineeringOverlayOptions{HasSelfDevelopmentOperation: true})
	dest := "/" + capsule.ReleaseTreePrefix + "frontend/"
	start := strings.Index(overlay, "cp -rf ")
	if start < 0 {
		t.Fatalf("the overlay carries no cp -rf release copy")
	}
	end := strings.Index(overlay[start:], dest)
	if end < 0 {
		t.Fatalf("the overlay copy does not target %s", dest)
	}
	copyCmd := overlay[start : start+end+len(dest)] // "cp -rf <flags> dist/. <dest>"

	if err := exec.Command("cp", "--no-preserve=mode", "/dev/null", filepath.Join(t.TempDir(), "x")).Run(); err != nil {
		t.Skipf("needs GNU cp (--no-preserve): %v", err)
	}

	work, err := os.MkdirTemp("", "release-copy-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = exec.Command("chmod", "-R", "a+rwX", work).Run()
		_ = os.RemoveAll(work)
	})
	frontend := filepath.Join(work, "frontend")
	dist := filepath.Join(frontend, "dist")
	releaseFrontend := filepath.Join(work, "release", "frontend")
	open := func(dirs ...string) {
		t.Helper()
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0o777); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o777); err != nil {
				t.Fatal(err)
			}
		}
	}
	open(work, dist, filepath.Join(dist, "legal"), frontend, filepath.Dir(releaseFrontend), releaseFrontend)

	// populate writes the build output; desktop-bridge.html and legal/*.md
	// are frontend/public files and keep the read-only source mode.
	populate := func(version string) {
		t.Helper()
		for rel, mode := range map[string]os.FileMode{"index.html": 0o644, "desktop-bridge.html": 0o444, "legal/terms-of-service.md": 0o444} {
			path := filepath.Join(dist, rel)
			_ = os.Chmod(path, 0o644)
			if err := os.WriteFile(path, []byte(rel+" "+version), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	run := func(command string) ([]byte, error) {
		cmd := exec.Command("sh", "-c", "cd "+frontend+" && "+strings.ReplaceAll(command, dest, releaseFrontend+"/"))
		if os.Getuid() == 0 { // root ignores 0444; run as the capsule's unprivileged user
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		}
		return cmd.CombinedOutput()
	}
	check := func(version string) {
		t.Helper()
		for _, rel := range []string{"index.html", "desktop-bridge.html", "legal/terms-of-service.md"} {
			got, err := os.ReadFile(filepath.Join(releaseFrontend, rel))
			if want := rel + " " + version; err != nil || string(got) != want {
				t.Errorf("%s = %q (err %v), want %q", rel, got, err, want)
			}
		}
	}

	populate("v1")
	if out, err := run(copyCmd); err != nil {
		t.Fatalf("first copy failed: %v\n%s", err, out)
	}
	check("v1")
	populate("v2") // the next cell: the build output changed
	if out, err := run(copyCmd); err != nil {
		t.Fatalf("retry copy over the first copy failed: %v\n%s", err, out)
	}
	check("v2")

	// The reproduced failure: a plain cp -r retry over files a first copy
	// left 0444 (what the pre-fix recipe's cp -a left behind).
	oldCmd := "cp -r dist/. " + dest
	if err := os.RemoveAll(releaseFrontend); err != nil {
		t.Fatal(err)
	}
	open(releaseFrontend)
	populate("v3")
	if out, err := run(oldCmd); err != nil {
		t.Fatalf("plain cp -r first copy failed: %v\n%s", err, out)
	}
	populate("v4")
	if out, err := run(oldCmd); err == nil {
		t.Fatalf("plain cp -r retry over 0444 files succeeded; the failure this test pins did not reproduce:\n%s", out)
	}
}
