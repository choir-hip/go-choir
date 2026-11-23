package autoputer

import (
	"os"
	"strings"
	"testing"
)

// Failure 22 (the order half): the capture's scratch is swept at the earliest
// boot step, before the compact rebase, the store open and the tape replay, so
// a boot on a volume a crashed capture filled can still rebase, open and
// replay. Run is one long function with no seam that simulates a full volume,
// so this pins the ORDER of the calls in its source; the d@e kill drill covers
// the rest (docs/design/capture-static-copy-2026-11-15.md, section 4).
func TestTheScratchIsSweptBeforeTheRebaseTheOpenAndTheReplay(t *testing.T) {
	raw, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	run := strings.Index(src, "func Run()")
	sweep := strings.Index(src, "store.RemoveBootScratch(")
	rebase := strings.Index(src, "store.RebaseOntoCompactCopy(")
	open := strings.Index(src, "store.Open(rtCfg.StorePath)")
	if run < 0 || sweep < run || rebase < 0 || open < 0 || !(sweep < rebase && rebase < open) {
		t.Fatalf("source order in Run: sweep %d, rebase %d, open %d (Run at %d): the sweep must come first", sweep, rebase, open, run)
	}
}
