package updater

// Review round 1, the two guards the boot-time premise needs:
//
//  S1  the prune runs only while nothing else does. A later re-run (a manual
//      start, a re-activated target) beside a live daemon refuses: the daemon's
//      socket or its started marker exists (TestPruneRefusesWhileTheDaemonIsLive)
//  S2  a full volume: the hide step is a rename that can fail with ENOSPC
//      before anything is freed. Leftovers are deleted first
//      (TestPruneDeletesLeftoversBeforeHidingAnything), and a rename that fails
//      with ENOSPC deletes that entry in place, manifest first, retrying the
//      rename as soon as space is back so a partial tree is not left under its
//      real name (TestPruneFreesSpaceInPlaceWhenARenameFailsWithENOSPC,
//      TestPruneInPlaceDeletionOfAReleaseRemovesItsManifestFirst).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPruneRefusesWhileTheDaemonIsLive(t *testing.T) {
	for _, marker := range []string{"updater.sock", "updater-unit-entered", "runtime-unit-entered"} {
		u, root := newPruneUpdater(t)
		digest := stageTestRelease(t, u, root, "r1", false)
		setCurrent(t, root, digest)
		stale := filepath.Join(root, "incoming", ".stage-old")
		writeTestFile(t, filepath.Join(stale, "f"), 100)
		ageBy(t, stale, old)
		runDir := t.TempDir()
		writeTestFile(t, filepath.Join(runDir, marker), 0)
		report := Prune(PruneOptions{Root: root, RunDir: runDir, Now: func() time.Time { return pruneNow }})
		if !strings.Contains(report.Skipped, "running") || !strings.Contains(report.Skipped, marker) || !exists(stale) || report.Entries() != 0 {
			t.Fatalf("%s: pruned beside a live daemon: %+v", marker, report)
		}
		if !strings.Contains(report.Line(), "skipped (") {
			t.Fatalf("line = %q", report.Line())
		}
		// At boot /run/choir has neither.
		if report := Prune(PruneOptions{Root: root, RunDir: t.TempDir(), Now: func() time.Time { return pruneNow }}); report.Skipped != "" || exists(stale) {
			t.Fatalf("%s: a clean run dir did not prune: %+v", marker, report)
		}
	}
}

type renameFailer struct {
	enospcFor func(oldpath string, call int) bool
	calls     int
	seen      []string
}

func (r *renameFailer) rename(oldpath, newpath string) error {
	r.calls++
	r.seen = append(r.seen, filepath.Base(oldpath))
	if r.enospcFor != nil && r.enospcFor(oldpath, r.calls) {
		return &os.LinkError{Op: "rename", Old: oldpath, New: newpath, Err: syscall.ENOSPC}
	}
	return os.Rename(oldpath, newpath)
}

func fullVolumeOptions(root string, r *renameFailer) PruneOptions {
	return PruneOptions{Root: root, Now: func() time.Time { return pruneNow }, rename: r.rename}
}

func TestPruneDeletesLeftoversBeforeHidingAnything(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	leftover := filepath.Join(root, "incoming", ".prune-"+strings.Repeat("a", 64))
	writeTestFile(t, filepath.Join(leftover, "f"), 8192)
	var firstRename int
	failer := &renameFailer{}
	failer.enospcFor = func(oldpath string, call int) bool {
		if firstRename == 0 {
			firstRename = call
			if exists(leftover) {
				t.Errorf("the first rename ran while the earlier leftover was still on the volume")
			}
		}
		return false
	}
	stale := filepath.Join(root, "incoming", ".stage-old")
	writeTestFile(t, filepath.Join(stale, "f"), 100)
	ageBy(t, stale, old)
	report := Prune(fullVolumeOptions(root, failer))
	if firstRename == 0 || exists(stale) || report.BytesFreed < 8192+100 {
		t.Fatalf("report = %+v (first rename %d)", report, firstRename)
	}
}

func TestPruneFreesSpaceInPlaceWhenARenameFailsWithENOSPC(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	big := filepath.Join(root, "incoming", ".stage-big")
	for i := 0; i < 3; i++ {
		writeTestFile(t, filepath.Join(big, fmt.Sprintf("f%d", i)), 1<<20+4096)
	}
	ageBy(t, big, old)
	small := filepath.Join(root, "incoming", ".stage-small")
	writeTestFile(t, filepath.Join(small, "f"), 100)
	ageBy(t, small, old)
	// Every rename of the first victim fails until one file of it is gone; the
	// volume has room again after that (the retry follows a sync).
	failer := &renameFailer{}
	failer.enospcFor = func(oldpath string, call int) bool {
		return filepath.Base(oldpath) == ".stage-big" && exists(filepath.Join(big, "f0"))
	}
	report := Prune(fullVolumeOptions(root, failer))
	if report.Errors != 0 || exists(big) || exists(small) {
		t.Fatalf("report = %+v", report)
	}
	if report.BytesFreed < 3<<20 || report.Entries() != 2 {
		t.Fatalf("report = %+v", report)
	}
	if left := hiddenLeftovers(t, root); len(left) != 0 {
		t.Fatalf("leftovers: %v", left)
	}
	// Total ENOSPC: nothing can be renamed. Everything in incoming/ is still
	// freed, in place.
	u2, root2 := newPruneUpdater(t)
	d2 := stageTestRelease(t, u2, root2, "r1", false)
	setCurrent(t, root2, d2)
	for i := 0; i < 3; i++ {
		dir := filepath.Join(root2, "incoming", fmt.Sprintf(".stage-%d", i))
		writeTestFile(t, filepath.Join(dir, "bin", "tool"), 5000)
		ageBy(t, dir, old)
	}
	always := &renameFailer{enospcFor: func(string, int) bool { return true }}
	report = Prune(fullVolumeOptions(root2, always))
	entries, _ := os.ReadDir(filepath.Join(root2, "incoming"))
	if len(entries) != 0 || report.IncomingRemoved != 3 || report.Errors != 0 || report.BytesFreed < 3*5000 {
		t.Fatalf("report = %+v entries = %v", report, entries)
	}
	if _, err := ReadCurrentManifest(root2); err != nil {
		t.Fatalf("current unreadable: %v", err)
	}
}

// A release halfway deleted under its real name must not look whole: the
// manifest goes first, so ReadPinnedManifest refuses it and the next boot's
// prune removes the corpse (TestPruneRemovesAReleaseCorpseAtOnce).
func TestPruneInPlaceDeletionOfAReleaseRemovesItsManifestFirst(t *testing.T) {
	u, root := newPruneUpdater(t)
	keep := stageTestRelease(t, u, root, "keep", false)
	victim := stageTestRelease(t, u, root, "victim", false)
	setCurrent(t, root, keep)
	ageBy(t, filepath.Join(root, "releases", victim), 30*24*time.Hour)
	victimDir := filepath.Join(root, "releases", victim)
	manifestGoneBeforeAnythingElse := false
	// The release is already staged and read-only; record what is left after
	// the first file the in-place deletion removed.
	entries := 0
	failer2 := &renameFailer{enospcFor: func(oldpath string, call int) bool {
		if oldpath != victimDir {
			return false
		}
		if !exists(filepath.Join(victimDir, "release-manifest.json")) {
			if es, err := os.ReadDir(victimDir); err == nil {
				entries = len(es)
			}
			manifestGoneBeforeAnythingElse = entries > 0
			return false
		}
		return true
	}}
	report := Prune(PruneOptions{Root: root, Now: func() time.Time { return pruneNow }, rename: failer2.rename, freeChunk: 1})
	if !manifestGoneBeforeAnythingElse {
		t.Fatalf("the manifest was not the first file removed in place (entries left %d): %+v", entries, report)
	}
	if exists(victimDir) || !exists(filepath.Join(root, "releases", keep)) {
		t.Fatalf("report = %+v", report)
	}
}
