package agentcore

// Devin round 1 on the boot follow: the daemon is another process, so the
// follow must see a held apply lock, re-check right before the swap, fail
// closed on an unlistable operations dir, and name what is actually served.

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// B1a: an apply that holds its key lock but has not journaled yet.
func TestBootDefersTheFollowWhileAnApplyKeyLockIsHeld(t *testing.T) {
	computerID := "computer-follow-lock"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	ops := filepath.Join(updaterRoot, "operations")
	if err := os.MkdirAll(ops, 0o700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(ops, "apply-1.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	unlock := func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) }

	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>old deploy</html>" {
		t.Fatalf("restaged under a held apply lock: served %q", got)
	}
	unlock()
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>new deploy</html>" {
		t.Fatalf("a free lock blocked the follow: served %q", got)
	}
}

// B1b: current moved (an apply swapped it) between the first check and the swap.
func TestBootDoesNotSwapWhenCurrentMovedAfterTheFirstCheck(t *testing.T) {
	computerID := "computer-follow-moved"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	otherDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>daemon applied</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	rt.followBaseAt = func() { pointCurrent(t, updaterRoot, otherDigest) }

	manifest, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline)
	if err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>daemon applied</html>" {
		t.Fatalf("the follow overwrote a current that moved: served %q", got)
	}
	if manifest.ContentDigest != otherDigest || rt.startupManifest().Digest != otherDigest {
		t.Fatalf("the record names %s, not the live current %s", manifest.ContentDigest, otherDigest)
	}
}

// B1b: a journal that opened between the first check and the swap.
func TestBootDoesNotSwapWhenAJournalOpenedAfterTheFirstCheck(t *testing.T) {
	computerID := "computer-follow-journal-late"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	rt.followBaseAt = func() {
		ops := filepath.Join(updaterRoot, "operations")
		_ = os.MkdirAll(ops, 0o700)
		_ = os.WriteFile(filepath.Join(ops, "late.json"), []byte(`{"phase":"prepared","result":{}}`), 0o600)
	}
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>old deploy</html>" {
		t.Fatalf("swapped under a journal that opened late: served %q", got)
	}
}

// N1: an operations dir that exists but cannot be listed is not settled.
func TestBootDefersTheFollowWhenTheOperationsDirCannotBeListed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists any directory")
	}
	computerID := "computer-follow-unlistable"
	rt, updaterRoot, baseline := followBaseRuntime(t, computerID)
	staleDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>old deploy</html>")
	pointCurrent(t, updaterRoot, staleDigest)
	ops := filepath.Join(updaterRoot, "operations")
	if err := os.MkdirAll(ops, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ops, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ops, 0o700) })
	if _, err := rt.ensureServingBaselineAtBoot(context.Background(), computerID, baseline); err != nil {
		t.Fatal(err)
	}
	if got := servedIndex(t, updaterRoot); got != "<html>old deploy</html>" {
		t.Fatalf("restaged with an unlistable operations dir: served %q", got)
	}
}
