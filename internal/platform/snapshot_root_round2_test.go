package platform

// Review round 2 (written before the repair):
//  I1. a commit whose outcome is unknown (the row is durable, Commit returned
//      an error) does not move the root generation, so the collector's
//      shortcut unlinks a chunk the newest root names;
//  I2. the tombstone is checked once, before the verify, so a destroy that
//      tombstones between that check and the commit is not seen (201 for a
//      computer being purged), and the registration does not hold the
//      tombstone gate across its commit;
//  I3. a tombstone refusal is a 409 the guest reads as "retake the snapshot";
//  I4. two overlapping registrations sharing a chunk clear each other's pin.

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

// I1.
func TestAnInDoubtRootCommitStillKeepsTheChunkFromTheGenerationShortcut(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	d, p := f.snapChunk("capture", 40*time.Hour)
	rel := "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + d
	gen := f.store.snapshotRootGen.Load() // what a sweep reads before classifying
	f.store.snapshotRootCommitResult = func(error) error { return errors.New("commit reply lost") }
	if code := f.snapDeclare(h, "r", 1, d); code == http.StatusCreated {
		t.Fatal("the in-doubt registration reported success")
	}
	f.store.snapshotRootCommitResult = nil
	if names := f.snapRootNames(); !names[gcDigest("root/r")] {
		t.Fatal("setup: the row should be durable although Commit returned an error")
	}
	cand := gcCandidate{ns: "file-cas-chunks", rel: rel, path: p, rootGen: gen, rootGenSet: true}
	op, _, err := f.service.unlinkIfStillUnreachable(f.ctx, cand, time.Now().Add(-24*time.Hour))
	if err != nil || op == "deleted" || !exists(p) {
		t.Fatalf("an unlink after an in-doubt commit deleted a chunk the live root names: %q %v", op, err)
	}
}

// I1: the blocked-commit test's post-commit unlink, with the generation the
// sweep read before the registration.
func TestAnUnlinkAfterACommitWithAClassificationGenerationKeepsTheChunk(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	d, p := f.snapChunk("capture", 40*time.Hour)
	rel := "sha256/file-cas-chunks/" + snapVerifyComputer + "/" + d
	gen := f.store.snapshotRootGen.Load()
	if code := f.snapDeclare(h, "r", 1, d); code != http.StatusCreated {
		t.Fatalf("declare = %d", code)
	}
	cand := gcCandidate{ns: "file-cas-chunks", rel: rel, path: p, rootGen: gen, rootGenSet: true}
	op, _, err := f.service.unlinkIfStillUnreachable(f.ctx, cand, time.Now().Add(-24*time.Hour))
	if err != nil || op == "deleted" || !exists(p) {
		t.Fatalf("deleted: %q %v", op, err)
	}
}

// I2, one way: the tombstone appears between the early check and the commit.
func TestARootIsRefusedWhenATombstoneAppearsBetweenTheCheckAndTheCommit(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	d, _ := f.snapChunk("c", time.Hour)
	f.service.snapshotRootAfterVerify = func() {
		f.store.tombstones.gate.Lock()
		err := f.store.writeTapeTombstone(f.ctx, TapeTombstone{ComputerID: snapVerifyComputer, Status: tapeTombstoneArchiving, HeadSequence: 5, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "ts-x", ManifestSHA256: platformTestDigest('2'), ProofJSON: "{}"})
		f.store.tombstones.gate.Unlock()
		if err != nil {
			t.Error(err)
		}
	}
	if code := f.snapDeclare(h, "r", 1, d); code != http.StatusGone {
		t.Fatalf("root registered across a tombstone = %d, want 410", code)
	}
	if n := len(f.snapRootNames()); n != 0 {
		t.Fatalf("a root was recorded for a computer being purged: %d", n)
	}
}

// I2, the other way: the registration holds the tombstone gate across its
// commit, so a destroy apply's tombstone write waits for it.
func TestTheRegistrationHoldsTheTombstoneGateAcrossItsCommit(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	d, _ := f.snapChunk("c", time.Hour)
	var held, called bool
	f.service.snapshotRootInCommit = func() {
		called = true
		if f.store.tombstones.gate.TryLock() {
			f.store.tombstones.gate.Unlock()
			return
		}
		held = true
	}
	if code := f.snapDeclare(h, "r", 1, d); code != http.StatusCreated {
		t.Fatalf("declare = %d", code)
	}
	if !called || !held {
		t.Fatalf("gate not held across the commit (hook called %v, gate held %v)", called, held)
	}
	if !f.store.tombstones.gate.TryLock() {
		t.Fatal("the gate was not released after the registration")
	}
	f.store.tombstones.gate.Unlock()
}

// I4: the pending set counts, so the first registration to finish does not
// release a chunk the other still needs.
func TestOverlappingRegistrationsSharingAChunkKeepItPendingUntilBothFinish(t *testing.T) {
	f := newGCFixture(t)
	h := NewHandler(f.service)
	shared, _ := f.snapChunk("shared", time.Hour)
	key := snapshotChunkKey(snapVerifyComputer, shared)
	var inner bool
	f.service.snapshotRootAfterVerify = func() {
		if inner {
			return
		}
		inner = true
		if code := f.snapDeclare(h, "second", 2, shared); code != http.StatusCreated {
			t.Errorf("second registration = %d", code)
		}
		if !f.store.snapshotPending.has(key) {
			t.Error("the second registration released the first one's pin")
		}
	}
	if code := f.snapDeclare(h, "first", 1, shared); code != http.StatusCreated {
		t.Fatalf("first registration = %d", code)
	}
	if f.store.snapshotPending.has(key) {
		t.Fatal("a pin leaked after both registrations finished")
	}
}
