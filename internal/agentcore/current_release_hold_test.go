package agentcore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// Every in-process writer of `current` that can swap back to a prior release
// takes the updater's restore hold through lockCurrentRelease, so the updater's
// prune (another process) skips while it is held.
func TestLockCurrentReleaseHoldsTheUpdaterRestoreHold(t *testing.T) {
	root := t.TempDir()
	rt := &Runtime{selfdevUpdaterRoot: root}
	if restoreHoldTaken(t, root) {
		t.Fatal("nothing is held yet")
	}
	unlock, err := rt.lockCurrentRelease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !restoreHoldTaken(t, root) {
		t.Fatal("the restore hold is not taken while current is locked")
	}
	unlock()
	if restoreHoldTaken(t, root) {
		t.Fatal("the restore hold outlived the lock")
	}
	// Without an updater root (a test runtime) the lock still works.
	bare := &Runtime{}
	bareUnlock, err := bare.lockCurrentRelease(context.Background())
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
	unlock, err := rt.lockCurrentRelease(context.Background())
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

// restoreHoldTaken tests the advisory lock the updater's restore hold takes.
func restoreHoldTaken(t *testing.T, root string) bool {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(root, "operations", ".restore-in-flight"), os.O_RDWR, 0)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return true
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return false
}

// D2: a holder that never lets go yields the named error within the context,
// not a silent wait.
func TestLockCurrentReleaseGivesUpOnAStuckHolder(t *testing.T) {
	root := t.TempDir()
	held, err := updater.HoldRestore(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	rt := &Runtime{selfdevUpdaterRoot: root}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	unlock, err := rt.lockCurrentRelease(ctx)
	if !errors.Is(err, errRestoreHoldUnavailable) {
		t.Fatalf("err = %v, want errRestoreHoldUnavailable", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("the wait was not bounded")
	}
	unlock()
}

// B3 per caller: each swap-capable writer of `current` refuses on a hold error
// before it changes anything. The hold fails because operations/ is not a
// directory.
func breakTheRestoreHold(t *testing.T, root string) {
	t.Helper()
	ops := filepath.Join(root, "operations")
	_ = filepath.Walk(ops, func(p string, _ os.FileInfo, err error) error {
		if err == nil {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	if err := os.RemoveAll(ops); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ops, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSwapCapableWritersRefuseWhenTheRestoreHoldIsUnavailable(t *testing.T) {
	e := newPMEnv(t, pmPackageFiles("hello-app", pmManifest("hello-app"), "<p>hi</p>\n"), nil)
	ctx := context.Background()
	rec := e.migrate(t)
	if e.lastAttempt(t, rec).Outcome != pmDone {
		t.Fatal("setup: the migration must finish")
	}
	before := e.currentDigest(t)
	breakTheRestoreHold(t, e.updaterRoot)

	if _, err := e.rt.rollbackPackageMigration(ctx, rec.Pair); err == nil || !errors.Is(err, errRestoreHoldUnavailable) {
		t.Fatalf("rollbackPackageMigration = %v, want errRestoreHoldUnavailable", err)
	}
	target := e.baselineManifest(t)
	operation := selfdev.Operation{OperationID: "rollback-op", ComputerID: pmComputer, ReleaseDigest: target.ContentDigest}
	if err := e.rt.baseRestoreSelfDevelopmentOperation(ctx, operation, false); err == nil || !errors.Is(err, errRestoreHoldUnavailable) {
		t.Fatalf("baseRestoreSelfDevelopmentOperation = %v, want errRestoreHoldUnavailable", err)
	}
	if e.rt.eventAppender == nil {
		e.rt.eventAppender = &computerevent.ComputerEventAppender{}
	}
	if _, err := e.rt.RematerializeFromTape(ctx, pmComputer, selfdevprotocol.Checkpoint{}); err == nil || !errors.Is(err, ErrRematerializeUnavailable) || !strings.Contains(err.Error(), "restore hold") {
		t.Fatalf("RematerializeFromTape = %v, want ErrRematerializeUnavailable naming the hold", err)
	}
	if got := e.currentDigest(t); got != before {
		t.Fatalf("a refused writer changed current: %s -> %s", before, got)
	}
}
