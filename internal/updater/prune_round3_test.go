package updater

// Review round 3 on the boot-time prune (62c8cb18e):
//
//  D1  a failed stage keeps its source: a transient failure (ENOSPC) must not
//      delete the only local copy of a downloaded payload
//      (TestApplyKeepsItsSourceWhenStagingFails); a deterministic pre-staging
//      refusal still drops it (TestApplyDropsARefusedPayload) and a staged
//      release still drops it (TestApplyDropsItsStagedSource)
//  D2  an in-place deletion the budget cut short is counted in the line
//      (TestPruneCountsAnInPlaceDeletionTheBudgetCut)

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyKeepsItsSourceWhenStagingFails(t *testing.T) {
	u, root, _ := newApplyRig(t, "")
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-tampered", "idem-tampered", "tampered")
	// The payload no longer matches its manifest: staging verifies and fails.
	makeTreeWritable(request.SourceDir)
	victim := filepath.Join(request.SourceDir, request.Manifest.Files[0].Path)
	if err := os.WriteFile(victim, []byte("not what the manifest says"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Apply(context.Background(), request); err == nil {
		t.Fatal("a payload that does not match its manifest was applied")
	}
	if !exists(victim) {
		t.Fatal("a failed stage deleted the only local copy of the payload; the boot prune collects it after its window")
	}
	if exists(filepath.Join(root, "releases", request.Manifest.ContentDigest, "release-manifest.json")) {
		t.Fatal("the failed stage left a release behind")
	}
}

func TestPruneCountsAnInPlaceDeletionTheBudgetCut(t *testing.T) {
	u, root := newPruneUpdater(t)
	digest := stageTestRelease(t, u, root, "r1", false)
	setCurrent(t, root, digest)
	big := filepath.Join(root, "incoming", ".stage-big")
	for i := 0; i < 40; i++ {
		writeTestFile(t, filepath.Join(big, fmt.Sprintf("f%02d", i)), 100)
	}
	ageBy(t, big, old)
	always := &renameFailer{enospcFor: func(string, int) bool { return true }}
	report := Prune(PruneOptions{Root: root, Now: func() time.Time { return pruneNow }, Clock: fakeClock(time.Second), Budget: 12 * time.Second, rename: always.rename})
	line := report.Line()
	if !report.BudgetExhausted || !strings.Contains(line, "inplace_unfinished=1") {
		t.Fatalf("a deletion in place that the budget cut is not in the line: %s", line)
	}
	// The next boot finishes it.
	if second := pruneRoot(root); exists(big) || second.Errors != 0 {
		t.Fatalf("second = %+v", second)
	}
}

// D3: the source is dropped on a deterministic pre-staging refusal only. A floor
// that cannot be read, or a closure replay that fails for want of space, keeps
// it for the boot prune.
func TestApplyDropsItsSourceOnADeterministicStateRefusal(t *testing.T) {
	rig := newFloorRig(t, 2)
	rig.floor(t, 3)
	request := rig.release(t, "op-below-floor", 2)
	if _, err := rig.engine.Apply(context.Background(), request); err == nil {
		t.Fatal("a release below the floor was applied")
	}
	if exists(request.SourceDir) {
		t.Fatal("a deterministic refusal kept its payload")
	}
}

func TestApplyKeepsItsSourceWhenTheFloorCannotBeRead(t *testing.T) {
	rig := newFloorRig(t, 2)
	if err := os.WriteFile(filepath.Join(filepath.Dir(rig.schema), "store-floor.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := rig.release(t, "op-floor-unreadable", 2)
	if _, err := rig.engine.Apply(context.Background(), request); err == nil {
		t.Fatal("a release was applied although the floor could not be read")
	}
	if !exists(request.SourceDir) {
		t.Fatal("an unreadable floor (not a refusal of the release) deleted the payload")
	}
}

func TestApplyKeepsItsSourceWhenTheClosureReplayFails(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	baseManifestPath, bootedDigest := layeredTestBase(t)
	u, root, _ := newApplyRig(t, baseManifestPath)
	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-replay", bootedDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	// The release is already staged (a replayed or resumed apply), and the
	// private store cannot be written.
	if err := u.stageRelease(request.SourceDir, filepath.Join(root, "releases", request.Manifest.ContentDigest), request.Manifest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "store"), []byte("a file, not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Apply(context.Background(), request); err == nil {
		t.Fatal("the replay succeeded without a store")
	}
	if !exists(request.SourceDir) {
		t.Fatal("a failed closure replay deleted the payload")
	}
}

// The deterministicRefusal wrapper itself: a base-commit mismatch is dropped, a
// receipt that cannot be read (transient, not a verdict on the release) is kept.
func TestApplyDropsItsSourceOnABaseCommitMismatch(t *testing.T) {
	basePath, _ := compatTestBase(t, "commit-abc123")
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	engine, _ := compatEngine(t, root, basePath, writeSchemaReceipt(t, t.TempDir(), 1))
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-base", "idem-base", "base")
	request.Manifest.BaseCommit = "commit-other"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(context.Background(), request); err == nil || !strings.Contains(err.Error(), ReasonBaseMovedSinceFreeze) {
		t.Fatalf("apply = %v, want the base-moved refusal", err)
	}
	if exists(request.SourceDir) {
		t.Fatal("a base-commit mismatch kept its payload")
	}
}

func TestApplyKeepsItsSourceWhenTheSchemaReceiptCannotBeRead(t *testing.T) {
	rig := newFloorRig(t, 2)
	if err := os.WriteFile(rig.schema, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	request := rig.release(t, "op-receipt-unreadable", 2)
	if _, err := rig.engine.Apply(context.Background(), request); err == nil {
		t.Fatal("a release was applied although the schema receipt could not be read")
	}
	if !exists(request.SourceDir) {
		t.Fatal("an unreadable receipt (no verdict on the release) deleted the payload")
	}
}
