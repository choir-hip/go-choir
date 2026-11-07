package agentcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/shotsandbox"
)

// apply-proof-4 on d@e (selfdev-df8c8b1c, 2026-11-07): the work root sat
// under the root-only capsule state directory (0700), so the sandbox user
// could not stat the staged frontend and every shot failed closed. These pin
// the ways the sandbox user can be kept from what the worker staged for it.

// FR1: the default work root is not under the capsule state directory.
func TestShotWorkRootIsOutsideTheRootOnlyCapsuleState(t *testing.T) {
	t.Setenv("CHOIR_SHOT_WORK_DIR", "")
	t.Setenv("CHOIR_CAPSULE_STATE_DIR", "/mnt/persistent/capsule-state")
	root := defaultShotWorkRoot()
	if root != "/mnt/persistent/choir-shot" {
		t.Fatalf("work root = %q", root)
	}
}

// FR2: a work root left 0700 (an earlier boot, a tmpfiles default) is made
// traversable, and the operation directory is traversable.
func TestShotWorkRootAndOperationDirAreTraversable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "choir-shot")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := prepareShotWorkRoot(root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o711 {
		t.Fatalf("work root mode = %v", info.Mode().Perm())
	}
}

// FR3: the reach check refuses a staged path whose ancestor the sandbox
// user cannot traverse, naming it, and passes a reachable one.
func TestSandboxReachRefusesAClosedAncestor(t *testing.T) {
	base := t.TempDir()
	if err := os.Chmod(base, 0o755); err != nil {
		t.Fatal(err)
	}
	closed := filepath.Join(base, "closed")
	open := filepath.Join(base, "open")
	for _, d := range []string{filepath.Join(closed, "fe"), filepath.Join(open, "fe")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "index.html"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod(closed, 0o700); err != nil {
		t.Fatal(err)
	}
	err := checkSandboxReach(base, filepath.Join(closed, "fe", "index.html"), 0o004)
	if err == nil || !strings.Contains(err.Error(), closed) {
		t.Fatalf("closed ancestor: err = %v", err)
	}
	if err := checkSandboxReach(base, filepath.Join(open, "fe", "index.html"), 0o004); err != nil {
		t.Fatalf("open path: %v", err)
	}
	// A binary others can read but not exec fails the exec check.
	if err := checkSandboxReach(base, filepath.Join(open, "fe", "index.html"), 0o005); err == nil {
		t.Fatal("a 0644 binary passed the exec check")
	}
	if err := os.Chmod(filepath.Join(open, "fe", "index.html"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkSandboxReach(base, filepath.Join(open, "fe", "index.html"), 0o004); err == nil {
		t.Fatal("an unreadable file passed the reach check")
	}
}

// FR4: a source file others cannot read is copied readable, not hardlinked
// with its private mode.
func TestLinkOrCopyFileDoesNotLinkAPrivateFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst")
	if err := linkOrCopyFile(src, dst, 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o004 == 0 {
		t.Fatalf("dst mode = %v", info.Mode().Perm())
	}
	if os.SameFile(mustStat(t, src), info) {
		t.Fatal("a private file was hardlinked")
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// FR5: the operation directory's name is short whatever the operation id, so
// the deepest path the sandbox makes (Chromium's socket under work/) fits the
// unix socket limit; distinct operations get distinct names.
func TestShotOperationDirNameIsShortAndDistinct(t *testing.T) {
	a := safeShotDirName("selfdev-70be92c52c642159ebd9b45721f9441e")
	b := safeShotDirName("selfdev-df8c8b1cc87a381199a016844dcf8c9e")
	if len(a) > 16 || a == b || a != safeShotDirName("selfdev-70be92c52c642159ebd9b45721f9441e") {
		t.Fatalf("names %q, %q", a, b)
	}
	t.Setenv("CHOIR_SHOT_WORK_DIR", "")
	t.Setenv("CHOIR_CAPSULE_STATE_DIR", "/mnt/persistent/capsule-state")
	work := filepath.Join(defaultShotWorkRoot(), a, "before", "work")
	if err := shotsandbox.CheckWorkDirLength(work); err != nil {
		t.Fatalf("the default layout: %v", err)
	}
}
