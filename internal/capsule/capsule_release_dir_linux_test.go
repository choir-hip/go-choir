//go:build linux

package capsule

// Failure mode 1 of capsule_release_dir_test.go, reproduced against a real
// overlay: the lower layer holds a release tree owned by root (an unmapped
// uid inside the capsule), and the capsule's build copies its dist into
// /var/lib/artifact/release/frontend as the capsule's mapped uid. Needs
// root and overlayfs with userxattr; skips otherwise.

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCapsuleReleaseDirHostIDIsTheMappedUID(t *testing.T) {
	if capsuleNamespaceHostID != 65534 {
		t.Fatalf("capsuleNamespaceHostID = %d; portable tests assume 65534", capsuleNamespaceHostID)
	}
}

// releaseOverlay mounts an overlay whose lower layer holds a root-owned
// release tree, optionally preparing the capsule release dir first.
func releaseOverlay(t *testing.T, prepare bool) (merged string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("overlay ownership reproduction needs root")
	}
	base, err := os.MkdirTemp("", "capsule-release-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	// The capsule uid must be able to traverse the test's directories.
	if err := os.Chmod(base, 0o755); err != nil {
		t.Fatal(err)
	}
	lower := filepath.Join(base, "lower")
	upper := filepath.Join(base, "upper")
	work := filepath.Join(base, "work")
	merged = filepath.Join(base, "merged")
	lowerRelease := filepath.Join(lower, "var", "lib", "artifact", "release")
	if err := os.MkdirAll(filepath.Join(lowerRelease, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{lower, filepath.Join(lower, "var"), filepath.Join(lower, "var", "lib"), filepath.Join(lower, "var", "lib", "artifact"), lowerRelease, filepath.Join(lowerRelease, "frontend")} {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(lowerRelease, "frontend", "index.html"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if prepare {
		if err := prepareCapsuleReleaseDir(upper, capsuleNamespaceHostID, capsuleNamespaceHostID); err != nil {
			t.Skipf("release dir prepare unavailable on this filesystem: %v", err)
		}
	}
	if err := MountOverlayFS(merged, upper, work, lower); err != nil {
		t.Skipf("overlay mount unavailable: %v", err)
	}
	t.Cleanup(func() { _ = UnmountOverlayFS(merged) })
	return merged
}

// copyAsCapsule runs the engineering build's release copy as the capsule's
// mapped uid.
func copyAsCapsule(t *testing.T, merged string) ([]byte, error) {
	t.Helper()
	dist := filepath.Join(filepath.Dir(merged), "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	release := filepath.Join(merged, "var", "lib", "artifact", "release")
	cmd := exec.Command("sh", "-c", `mkdir -p "$R/frontend" && cp -rf --no-preserve=mode,ownership,timestamps "$D"/. "$R/frontend/"`)
	cmd.Env = append(os.Environ(), "R="+release, "D="+dist)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: capsuleNamespaceHostID, Gid: capsuleNamespaceHostID}}
	return cmd.CombinedOutput()
}

func TestReleaseCopyFailsOverLowerReleaseOwnedByAnotherUID(t *testing.T) {
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp unavailable")
	}
	merged := releaseOverlay(t, false)
	if out, err := copyAsCapsule(t, merged); err == nil {
		t.Skipf("control copy unexpectedly succeeded without the fix (kernel/ownership differs): %s", out)
	}
}

func TestReleaseCopySucceedsAndHidesLowerReleaseAfterPrepare(t *testing.T) {
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp unavailable")
	}
	merged := releaseOverlay(t, true)
	release := filepath.Join(merged, "var", "lib", "artifact", "release")
	entries, err := os.ReadDir(release)
	if err != nil || len(entries) != 0 {
		t.Fatalf("lower release files show through (failure mode 3): entries=%v err=%v", entries, err)
	}
	if out, err := copyAsCapsule(t, merged); err != nil {
		t.Fatalf("release copy failed as the capsule uid (failure mode 1): %v\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(release, "frontend", "index.html"))
	if err != nil || string(data) != "new" {
		t.Fatalf("copied file: data=%q err=%v", data, err)
	}
}
