package agentcore

import (
	"errors"
	"os"
	"path/filepath"
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
	unlock, err := rt.lockCurrentRelease()
	if err != nil {
		t.Fatal(err)
	}
	if !updater.RestoreInFlight(root) {
		t.Fatal("the restore hold is not taken while current is locked")
	}
	unlock()
	if updater.RestoreInFlight(root) {
		t.Fatal("the restore hold outlived the lock")
	}
	// Without an updater root (a test runtime) the lock still works.
	bare := &Runtime{}
	bareUnlock, err := bare.lockCurrentRelease()
	if err != nil {
		t.Fatal(err)
	}
	bareUnlock()
}

// B3: a hold that cannot be taken (a full or read-only volume) is reported, so
// a restore that swaps refuses before it changes anything. The mutex is still
// returned held, for the caller that may proceed read-only.
func TestLockCurrentReleaseReportsAHoldItCannotTake(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{selfdevUpdaterRoot: filepath.Join(file, "updater")}
	unlock, err := rt.lockCurrentRelease()
	if err == nil || !errors.Is(err, errRestoreHoldUnavailable) {
		t.Fatalf("err = %v, want errRestoreHoldUnavailable", err)
	}
	if rt.currentReleaseMu.TryLock() {
		t.Fatal("the mutex was not held with the error")
	}
	unlock()
	if !rt.currentReleaseMu.TryLock() {
		t.Fatal("unlock did not release the mutex")
	}
	rt.currentReleaseMu.Unlock()
}

// The accepted price of a bounded updater root: a restore to a checkpoint whose
// release was pruned refuses before changing anything, with the named reason.
func TestPinnedFrontendIdentityOfAPrunedReleaseIsUnderivable(t *testing.T) {
	_, err := pinnedFrontendIdentity(t.TempDir(), strings.Repeat("a", 64))
	if err == nil || !strings.Contains(err.Error(), "served SPA is underivable") {
		t.Fatalf("err = %v", err)
	}
}
