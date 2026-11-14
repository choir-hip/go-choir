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
