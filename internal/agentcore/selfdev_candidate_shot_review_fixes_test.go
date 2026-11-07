//go:build linux

package agentcore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// S-B1, S-B2: the side directory itself is handed to the sandbox id (the
// store opens WAL beside the db and must create files there), the worker's
// own entries are handed over as trees, an absent sidecar is skipped, and the
// files layer's links are never chowned (they are the live inodes).
func TestHandOverGivesTheSideDirectoryAndSkipsAbsentEntries(t *testing.T) {
	side := t.TempDir()
	for _, d := range []string{"out", "work", "files"} {
		if err := os.Mkdir(filepath.Join(side, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(side, "state.db"), []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(side, "files", "live.txt"), []byte("live"), 0o644); err != nil {
		t.Fatal(err)
	}
	var lchowns, trees []string
	deps := candidateShotDeps{
		lchown:    func(p string, uid, gid int) error { lchowns = append(lchowns, p); return nil },
		chownTree: func(p string, uid, gid int) error { trees = append(trees, p); return nil },
	}
	// No state.texture in this store: the hand-over must not fail on it.
	if err := deps.handOver(side, 65534, 65534, "out", "work", "state.db", "state.texture"); err != nil {
		t.Fatalf("handOver: %v", err)
	}
	if len(lchowns) != 2 || lchowns[0] != side || lchowns[1] != filepath.Join(side, "files") {
		t.Fatalf("lchowned %v, want the side directory and the files layer's top", lchowns)
	}
	want := []string{filepath.Join(side, "out"), filepath.Join(side, "work"), filepath.Join(side, "state.db")}
	if strings.Join(trees, ",") != strings.Join(want, ",") {
		t.Fatalf("trees = %v, want %v", trees, want)
	}
	for _, p := range append(lchowns, trees...) {
		if strings.HasSuffix(p, "live.txt") {
			t.Fatal("a live linked file was chowned")
		}
	}
	// A failing chown of a present entry still fails the shot.
	deps.chownTree = func(string, int, int) error { return errors.New("EPERM") }
	if err := deps.handOver(side, 1, 1, "out"); err == nil {
		t.Fatal("a failed chown was swallowed")
	}
}

// The first worker of a process sweeps what a crashed one left in the work
// root, and a later worker does not sweep again.
func TestCandidateShotSweepsTheWorkRootOncePerProcess(t *testing.T) {
	wf := newWorkerFixture(t, "computer-worker-sweep")
	root := wf.rt.candidateShotDeps.workRoot
	stale := filepath.Join(root, "selfdev-crashed", "before")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	wf.rt.sweepShotWorkRootOnce()
	if _, err := os.Stat(filepath.Join(root, "selfdev-crashed")); err == nil {
		t.Fatal("a crashed run's leftovers survived the sweep")
	}
	later := filepath.Join(root, "selfdev-live")
	if err := os.MkdirAll(later, 0o755); err != nil {
		t.Fatal(err)
	}
	wf.rt.sweepShotWorkRootOnce()
	if _, err := os.Stat(later); err != nil {
		t.Fatal("a second sweep removed a running worker's directory")
	}
}
