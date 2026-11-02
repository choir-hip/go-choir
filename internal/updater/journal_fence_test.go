package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
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
//  3. a dead journal that moved `current` is fenced, hiding an installed
//     release; or a terminal applied one is reported as nothing installed.

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

func TestFenceRefusesWhileAnApplyHoldsTheKey(t *testing.T) {
	_, root, _ := fenceTestUpdater(t)
	unlock, acquired, err := lockApplyKey(root, "idem-busy", true)
	if err != nil || !acquired {
		t.Fatalf("lock: acquired=%v err=%v", acquired, err)
	}
	fence, err := FenceApply(root, "idem-busy", "settled")
	if err != nil || fence.State != FenceInFlight || fence.Fenced {
		t.Fatalf("fence while an apply holds the key = %+v (%v); want in flight", fence, err)
	}
	if _, found, _ := ReadJournalOutcome(root, "idem-busy"); found {
		t.Fatal("a fence was written under a running apply")
	}
	unlock()
	if fence, err := FenceApply(root, "idem-busy", "settled"); err != nil || fence.State != FenceNothingInstalled || !fence.Fenced {
		t.Fatalf("fence after the apply let go = %+v (%v)", fence, err)
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
	fence, err := FenceApply(root, "idem-2", "operation settled before the updater started")
	if err != nil || fence.State != FenceNothingInstalled || !fence.Fenced || !fence.Outcome.Terminal || fence.Outcome.Result.Outcome != "refused" {
		t.Fatalf("fence = %+v (%v)", fence, err)
	}
	restarts := service.restarts
	result, err := engine.Apply(context.Background(), late)
	if err == nil || result.Outcome != "refused" {
		t.Fatalf("late apply = %+v (%v); want the fence's refusal", result, err)
	}
	current, _, err := engine.currentRelease()
	if err != nil || current != first.Manifest.ContentDigest || service.restarts != restarts {
		t.Fatalf("late apply moved current to %s (restarts %d -> %d, %v)", current, restarts, service.restarts, err)
	}
	// A second fence reads the terminal journal and writes nothing.
	if again, err := FenceApply(root, "idem-2", "again"); err != nil || again.Fenced || again.State != FenceNothingInstalled || again.Outcome.Failure != "operation settled before the updater started" {
		t.Fatalf("second fence = %+v (%v)", again, err)
	}
}

// Failure 3: the journal's phase decides; only a dead journal that never moved
// `current` is fenced.
func TestFenceReadsTheJournalPhase(t *testing.T) {
	engine, root, _ := fenceTestUpdater(t)
	applied := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-a", "idem-applied", "applied release")
	if _, err := engine.Apply(context.Background(), applied); err != nil {
		t.Fatal(err)
	}
	if fence, err := FenceApply(root, "idem-applied", "x"); err != nil || fence.State != FenceApplied || fence.Fenced || fence.Outcome.Result.Outcome != "applied" {
		t.Fatalf("applied journal fence = %+v (%v)", fence, err)
	}
	for phase, want := range map[string]FenceState{
		"prepared":          FenceNothingInstalled,
		"pointer_swapped":   FenceInterrupted,
		"restart_requested": FenceInterrupted,
		"recovering":        FenceInterrupted,
	} {
		key := "idem-" + phase
		if err := writeJournal(filepath.Join(root, "operations", safeName(key)+".json"), operationJournal{
			RequestCommitment: "c", Phase: phase, TargetReleaseDigest: "t", StartedAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		fence, err := FenceApply(root, key, "x")
		if err != nil || fence.State != want || fence.Fenced != (want == FenceNothingInstalled) {
			t.Fatalf("%s: fence = %+v (%v); want %s", phase, fence, err, want)
		}
		outcome, _, _ := ReadJournalOutcome(root, key)
		if outcome.Terminal != (want == FenceNothingInstalled) {
			t.Fatalf("%s: journal terminal=%v after the fence", phase, outcome.Terminal)
		}
	}
}
