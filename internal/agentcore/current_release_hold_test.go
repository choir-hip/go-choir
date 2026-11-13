package agentcore

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Every in-process writer of `current` that can swap back to a prior release
// takes the updater's restore hold through lockCurrentRelease, so the updater's
// prune (another process) skips while it is held.
func TestLockCurrentReleaseHoldsTheUpdaterRestoreHold(t *testing.T) {
	root := t.TempDir()
	rt := &Runtime{selfdevUpdaterRoot: root}
	if updater.RestoreInFlight(root) {
		t.Fatal("nothing is held yet")
	}
	unlock := rt.lockCurrentRelease()
	if !updater.RestoreInFlight(root) {
		t.Fatal("the restore hold is not taken while current is locked")
	}
	unlock()
	if updater.RestoreInFlight(root) {
		t.Fatal("the restore hold outlived the lock")
	}
	// Without an updater root (a test runtime) the lock still works.
	bare := &Runtime{}
	bare.lockCurrentRelease()()
}

// The accepted price of a bounded updater root: a restore to a checkpoint whose
// release was pruned refuses before changing anything, with the named reason.
func TestPinnedFrontendIdentityOfAPrunedReleaseIsUnderivable(t *testing.T) {
	_, err := pinnedFrontendIdentity(t.TempDir(), strings.Repeat("a", 64))
	if err == nil || !strings.Contains(err.Error(), "served SPA is underivable") {
		t.Fatalf("err = %v", err)
	}
}
