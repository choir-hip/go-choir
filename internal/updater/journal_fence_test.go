package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// docs/problems/selfdev-stale-open-operations-block-engineering-2026-11-02.md,
// "Journal-first settlement". Ways the fence can fail, written first:
//  1. a settle reads "no journal" while an Apply for the key is running (the
//     journal is written only after staging) and settles a row the Apply then
//     installs under;
//  2. an Apply that arrives after the settle (queued, or its client gave up)
//     stages and swaps anyway;
//  3. a dead journal is judged by its phase instead of by `current`, so a
//     swapped-then-crashed apply is called nothing installed (or the reverse),
//     or a terminal applied one is reported as nothing installed;
//  4. an older daemon that takes no lock is trusted to hold it;
//  5. a corrupt journal wedges every settle and Apply for its key.

func fenceTestUpdater(t *testing.T) (*Updater, string, *fakeServiceManager) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	service := &fakeServiceManager{}
	engine, err := New(root, "computer-test", "realization-test", service, fakeHealthProber{}, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
	if err != nil {
		t.Fatal(err)
	}
	return engine, root, service
}

func holdOrFatal(t *testing.T, root, key string) *ApplyKeyHold {
	t.Helper()
	hold, err := HoldApplyKey(root, key)
	if err != nil {
		t.Fatal(err)
	}
	return hold
}

func TestHoldRefusesWhileAnApplyHoldsTheKey(t *testing.T) {
	_, root, _ := fenceTestUpdater(t)
	unlock, acquired, err := lockApplyKey(root, "idem-busy", true)
	if err != nil || !acquired {
		t.Fatalf("lock: acquired=%v err=%v", acquired, err)
	}
	if hold := holdOrFatal(t, root, "idem-busy"); hold.Verdict != VerdictInFlight {
		t.Fatalf("verdict while an apply holds the key = %s", hold.Verdict)
	}
	unlock()
	hold := holdOrFatal(t, root, "idem-busy")
	defer hold.Release()
	if hold.Verdict != VerdictNothingInstalled || !hold.Dead() {
		t.Fatalf("verdict after the apply let go = %+v", hold)
	}
}

// Failure 2: the apply the client gave up on arrives after the settle.
func TestAnApplyAfterTheFenceReturnsItAndInstallsNothing(t *testing.T) {
	engine, root, service := fenceTestUpdater(t)
	first := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-1", "idem-1", "first release")
	if _, err := engine.Apply(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	late := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-2", "idem-2", "late release")
	hold := holdOrFatal(t, root, "idem-2")
	if hold.Verdict != VerdictNothingInstalled {
		t.Fatalf("verdict = %+v", hold)
	}
	if err := hold.Fence("operation settled before the updater started"); err != nil {
		t.Fatal(err)
	}
	hold.Release()
	restarts := service.restarts
	result, err := engine.Apply(context.Background(), late)
	if err == nil || result.Outcome != "refused" {
		t.Fatalf("late apply = %+v (%v); want the fence's refusal", result, err)
	}
	current, _, err := engine.currentRelease()
	if err != nil || current != first.Manifest.ContentDigest || service.restarts != restarts {
		t.Fatalf("late apply moved current to %s (restarts %d -> %d, %v)", current, restarts, service.restarts, err)
	}
	again := holdOrFatal(t, root, "idem-2")
	defer again.Release()
	if again.Verdict != VerdictFailed || !strings.HasPrefix(again.Outcome.Failure, "operation settled before the updater started") {
		t.Fatalf("after the fence = %+v", again)
	}
}

// Failure 3: a dead journal is arbitrated on `current`, whatever its phase.
func TestDeadJournalIsArbitratedOnCurrent(t *testing.T) {
	engine, root, _ := fenceTestUpdater(t)
	prior := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-p", "idem-prior", "prior release")
	if _, err := engine.Apply(context.Background(), prior); err != nil {
		t.Fatal(err)
	}
	if hold := holdOrFatal(t, root, "idem-prior"); hold.Verdict != VerdictApplied {
		t.Fatalf("applied journal verdict = %s", hold.Verdict)
	} else {
		hold.Release()
	}
	target := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-t", "idem-target", "target release")
	targetDir := filepath.Join(root, "releases", target.Manifest.ContentDigest)
	if err := engine.stageRelease(target.SourceDir, targetDir, target.Manifest); err != nil {
		t.Fatal(err)
	}
	priorDigest := prior.Manifest.ContentDigest
	write := func(key, phase string) {
		if err := writeJournal(filepath.Join(root, "operations", safeName(key)+".json"), operationJournal{
			RequestCommitment: "c", Phase: phase, PriorReleaseDigest: priorDigest, TargetReleaseDigest: target.Manifest.ContentDigest, StartedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(key string, want ApplyVerdict) {
		t.Helper()
		hold := holdOrFatal(t, root, key)
		defer hold.Release()
		if hold.Verdict != want || !hold.Dead() {
			t.Fatalf("%s: verdict = %+v; want %s", key, hold, want)
		}
		if err := hold.Fence("settled"); err != nil {
			t.Fatal(err)
		}
		if outcome, _, _ := ReadJournalOutcome(root, key); !outcome.Terminal || outcome.Result.Outcome != "refused" {
			t.Fatalf("%s: journal after the fence = %+v", key, outcome)
		}
	}
	// `current` is the prior: a swapped-phase journal still installed nothing.
	write("idem-swapped-prior", "pointer_swapped")
	check("idem-swapped-prior", VerdictNothingInstalled)
	// `current` is the target: a prepared-phase journal is still installed.
	if err := engine.swapCurrent(targetDir); err != nil {
		t.Fatal(err)
	}
	write("idem-prepared-target", "prepared")
	check("idem-prepared-target", VerdictInstalled)
	write("idem-recovering-target", "recovering")
	check("idem-recovering-target", VerdictInstalled)
	// Neither: unknown, and still terminal after the fence.
	if err := engine.swapCurrent(filepath.Join(root, "releases", priorDigest)); err != nil {
		t.Fatal(err)
	}
	if err := writeJournal(filepath.Join(root, "operations", safeName("idem-neither")+".json"), operationJournal{
		RequestCommitment: "c", Phase: "restart_requested", PriorReleaseDigest: strings.Repeat("1", 64), TargetReleaseDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatal(err)
	}
	check("idem-neither", VerdictUnknown)
	// Failure 5: a corrupt journal is dead and unknown, and the fence repairs it.
	if err := os.WriteFile(filepath.Join(root, "operations", safeName("idem-corrupt")+".json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	check("idem-corrupt", VerdictUnknown)
}

// Failure 4: an older daemon writes no marker and takes no lock.
func TestAnUnprovenLockNeverCallsAJournalDead(t *testing.T) {
	_, root, _ := fenceTestUpdater(t)
	if err := os.Remove(filepath.Join(root, "operations", keyLockMarker)); err != nil {
		t.Fatal(err)
	}
	if hold := holdOrFatal(t, root, "idem-old"); hold.Verdict != VerdictLockUnproven || hold.Dead() {
		t.Fatalf("missing journal under an old daemon = %+v; want lock_unproven", hold)
	}
	if err := writeJournal(filepath.Join(root, "operations", safeName("idem-old-done")+".json"), operationJournal{Phase: "completed", Result: ApplyResult{Outcome: "applied"}}); err != nil {
		t.Fatal(err)
	}
	// A terminal journal needs no lock: it is still the authority.
	hold := holdOrFatal(t, root, "idem-old-done")
	defer hold.Release()
	if hold.Verdict != VerdictApplied {
		t.Fatalf("terminal journal under an old daemon = %+v", hold)
	}
}
