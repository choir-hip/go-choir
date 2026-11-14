package updater

// Review round 2 on the boot-time prune (9843b93b1). Failure list:
//
//  C1  a store object is never freed in place: it has no marker, and
//      MaterializeClosure treats an existing object as whole, so a kill inside
//      an in-place delete would leave a partial object that is trusted
//      (TestPruneNeverFreesAStoreObjectInPlace)
//  C2  incoming/ and releases/ are actually deleted before gc-roots and store
//      are hidden, so the freed bytes exist before store renames need
//      directory blocks (TestPruneDeletesIncomingAndReleasesBeforeHidingStore)
//  C3  the budget bounds the whole run, hide phases included
//      (TestPruneBudgetBoundsTheHidePhases)
//  C4  a skipped run is not a measurement (TestPruneSkippedLineIsNotAMeasurement)

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPruneNeverFreesAStoreObjectInPlace(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	object := filepath.Join(root, "store", "orphan-object")
	for i := 0; i < 4; i++ {
		writeTestFile(t, filepath.Join(object, "bin", fmt.Sprintf("f%d", i)), 1<<20+4096)
	}
	ageBy(t, object, old)
	stale := filepath.Join(root, "incoming", ".stage-old")
	writeTestFile(t, filepath.Join(stale, "f"), 100)
	ageBy(t, stale, old)
	failer := &renameFailer{enospcFor: func(oldpath string, call int) bool {
		return strings.Contains(oldpath, string(filepath.Separator)+"store"+string(filepath.Separator))
	}}
	report := Prune(fullVolumeOptions(root, failer))
	for i := 0; i < 4; i++ {
		if !exists(filepath.Join(object, "bin", fmt.Sprintf("f%d", i))) {
			t.Fatalf("file %d of a store object was deleted in place: a partial object under its real name is trusted by MaterializeClosure", i)
		}
	}
	if exists(stale) {
		t.Fatal("incoming/ was not freed in place on a full volume")
	}
	if !strings.Contains(report.Line(), "enospc_skipped=1") {
		t.Fatalf("the skip is not in the line: %s", report.Line())
	}
	// A later boot with room hides and deletes it whole.
	second := pruneRoot(root)
	if exists(object) || second.StoreRemoved != 1 {
		t.Fatalf("the next boot did not collect it: %+v", second)
	}
}

func TestPruneDeletesIncomingAndReleasesBeforeHidingStore(t *testing.T) {
	u, root := newPruneUpdater(t)
	keep := stageTestRelease(t, u, root, "keep", false)
	dead := stageTestRelease(t, u, root, "dead", false)
	setCurrent(t, root, keep)
	ageBy(t, filepath.Join(root, "releases", dead), 30*24*time.Hour)
	stale := filepath.Join(root, "incoming", ".stage-old")
	writeTestFile(t, filepath.Join(stale, "f"), 100)
	ageBy(t, stale, old)
	orphan := filepath.Join(root, "store", "orphan")
	writeTestFile(t, filepath.Join(orphan, "f"), 100)
	ageBy(t, orphan, old)
	checked := false
	failer := &renameFailer{enospcFor: func(oldpath string, call int) bool {
		if strings.Contains(oldpath, string(filepath.Separator)+"store"+string(filepath.Separator)) {
			checked = true
			if left := hiddenLeftovers(t, root); len(left) != 0 {
				t.Errorf("hidden entries still on the volume when the first store entry was renamed: %v", left)
			}
		}
		return false
	}}
	Prune(fullVolumeOptions(root, failer))
	if !checked {
		t.Fatal("the store entry was never renamed")
	}
}

func TestPruneBudgetBoundsTheHidePhases(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	for i := 0; i < 20; i++ {
		dir := filepath.Join(root, "incoming", fmt.Sprintf(".stage-%02d", i))
		writeTestFile(t, filepath.Join(dir, "f"), 100)
		ageBy(t, dir, old)
	}
	failer := &renameFailer{}
	first := Prune(PruneOptions{Root: root, Now: func() time.Time { return pruneNow }, Clock: fakeClock(time.Second), Budget: 4 * time.Second, rename: failer.rename})
	if !first.BudgetExhausted || failer.calls >= 20 {
		t.Fatalf("the hide phase ran past the budget: %d renames, %+v", failer.calls, first)
	}
	second := pruneRoot(root)
	if second.BudgetExhausted || second.Errors != 0 {
		t.Fatalf("second = %+v", second)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "incoming"))
	if len(entries) != 0 {
		t.Fatalf("incoming after the second run: %v", entries)
	}
}

func TestPruneSkippedLineIsNotAMeasurement(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	runDir := t.TempDir()
	writeTestFile(t, filepath.Join(runDir, "updater.sock"), 0)
	line := Prune(PruneOptions{Root: root, RunDir: runDir}).Line()
	if strings.Contains(line, "releases_kept") || strings.Contains(line, "bytes_freed") {
		t.Fatalf("a skipped run prints counters as if measured: %s", line)
	}
	missing := Prune(PruneOptions{Root: filepath.Join(t.TempDir(), "none")})
	if !strings.Contains(missing.Skipped, "no root") || strings.Contains(missing.Line(), "releases_kept") {
		t.Fatalf("a missing root: %+v / %s", missing, missing.Line())
	}
	file := filepath.Join(t.TempDir(), "file")
	writeTestFile(t, file, 1)
	if got := Prune(PruneOptions{Root: file}); !strings.Contains(got.Skipped, "no root") {
		t.Fatalf("a non-directory root: %+v", got)
	}
}

