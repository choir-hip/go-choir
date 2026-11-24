package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The second review of the named sweep (2026-11-24, two seats) sent it back
// for one shape: the apply proved and deleted one computer at a time, outside
// the lock that the paths which add references take, and skipped or went on
// when something failed. The apply is now all or nothing under one gate.
// Written before the code; each test was seen failing on the reviewed head.
//
//	W1  A payload two named computers share is deleted for one and kept for
//	    the other when the second fails its re-proof.
//	    TestNamedApplyDeletesNothingWhenOneComputerFailsItsReproof
//	W2  A lifecycle operation (a restore) pending after the dry run is not seen
//	    by the apply; one prepared after the tombstones is accepted.
//	    TestApplyRefusesAPendingOperationAndATombstonedComputerRefusesOne
//	W3  An error proving the chunk digests of one computer skips it and the
//	    next computer, which has no chunks, is deleted.
//	    TestAChunkProofErrorFailsTheWholeRun
//	W4  A file refreshed after the dry run is reported kept, but the computer
//	    is marked deleted and its escrow shredded.
//	    TestARefreshedFileFailsTheRunAndTheComputerIsNotMarkedDeleted
//	W5  A tombstoned, unpurged computer that is not being deleted has its
//	    payload references left out of the live set.
//	    TestATombstonedComputerNotBeingDeletedStillCountsItsReferences

func TestNamedApplyDeletesNothingWhenOneComputerFailsItsReproof(t *testing.T) {
	f := newTapeFixture(t)
	c1 := f.fullComputer("dead-1", 1)
	c2 := f.fullComputer("dead-2", 1)
	shared := []byte("a payload both removed computers name")
	s1 := f.appendSeq(c1, shared, tapeAged)
	f.appendSeq(c2, shared, tapeAged)
	dry := f.dryRun(namedReq("dead-1", "dead-2"))
	f.reg.owned["dead-2"] = true
	before1, before2 := f.snapshotOf("dead-1"), f.snapshotOf("dead-2")

	report, err := f.apply(dry, namedReq("dead-1", "dead-2"))
	if err == nil || !strings.Contains(err.Error(), "dead-2") {
		t.Fatalf("apply answered %v (%+v) while dead-2 failed its re-proof", err, report.Apply)
	}
	if !f.exists(tapeNSPayload, s1.payload) {
		t.Fatal("the payload the surviving computer names was deleted")
	}
	f.allExist(c1, true)
	f.allExist(c2, true)
	if !before1.equal(f.snapshotOf("dead-1")) || !before2.equal(f.snapshotOf("dead-2")) {
		t.Fatal("a run that failed its re-proof deleted files or rows")
	}
	if st := f.tombstoneStatus("dead-1") + f.tombstoneStatus("dead-2"); st != "" {
		t.Fatalf("a run that deleted nothing left tombstones: %q", st)
	}
	if report.Apply.Deleted != 0 {
		t.Fatalf("the receipt says %d files were deleted", report.Apply.Deleted)
	}
}

func TestApplyRefusesAPendingOperationAndATombstonedComputerRefusesOne(t *testing.T) {
	t.Run("a pending operation after the dry run", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 1)
		f.fullComputer("dead-2", 1)
		dry := f.dryRun(namedReq("dead-1", "dead-2"))
		if _, err := f.store.db.Exec(`INSERT INTO computer_lifecycle_operations (computer_id,idempotency_key,request_commitment,action,prior_lifecycle_state,prior_realization_epoch,status,created_at) VALUES (?,?,?,?,?,?,?,?)`,
			"dead-2", "k1", platformTestDigest('a'), "recover", "hibernated", 1, "pending", time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		before1, before2 := f.snapshotOf("dead-1"), f.snapshotOf("dead-2")
		if _, err := f.apply(dry, namedReq("dead-1", "dead-2")); err == nil || !strings.Contains(err.Error(), "dead-2") {
			t.Fatalf("apply answered %v for a computer with a pending operation", err)
		}
		if !before1.equal(f.snapshotOf("dead-1")) || !before2.equal(f.snapshotOf("dead-2")) {
			t.Fatal("files or rows were deleted")
		}
	})
	t.Run("a lifecycle operation prepared once the sweep holds the computer", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 2)
		dry := f.dryRun(namedReq("dead-1"))
		var once sync.Once
		var wg sync.WaitGroup
		var prepErr, rootErr error
		f.svc.tapeSweep.beforeDelete = func(tapeSweepItem) {
			once.Do(func() {
				wg.Add(1)
				go func() {
					defer wg.Done()
					req := LifecycleControlRequest{Phase: "prepare", ComputerID: "dead-1", IdempotencyKey: "restore-during-sweep", Action: "recover", PriorState: "hibernated", PriorEpoch: 1}
					req.RequestCommitment, _ = lifecycleControlCommitment(req)
					_, prepErr = f.svc.PrepareLifecycleControl(f.ctx, req)
					_, rootErr = f.svc.RecordVerifiedSnapshotRoot(f.ctx, "dead-1", platformTestDigest('e'), 1, []string{chunkDigest("sealed chunk dead-1 1")})
				}()
			})
		}
		if _, err := f.apply(dry, namedReq("dead-1")); err != nil {
			t.Fatal(err)
		}
		wg.Wait()
		if !errors.Is(prepErr, ErrComputerTapeDeleted) {
			t.Fatalf("a restore prepared against a computer the sweep deleted answered %v", prepErr)
		}
		if !errors.Is(rootErr, ErrComputerTapeDeleted) {
			t.Fatalf("a snapshot root registered against it answered %v", rootErr)
		}
		var n int
		if err := f.store.db.QueryRow(`SELECT COUNT(*) FROM computer_lifecycle_operations WHERE computer_id=?`, "dead-1").Scan(&n); err != nil || n != 0 {
			t.Fatalf("an operation row for the deleted computer exists: %d %v", n, err)
		}
	})
}

func TestAChunkProofErrorFailsTheWholeRun(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.fullComputer("live-1", 1)
	f.fullComputer("dead-1", 1) // has chunks
	c2 := f.chain("dead-2", 2, tapeAged)
	dry := f.dryRun(namedReq("dead-1", "dead-2"))
	// A living computer's file-root manifest becomes unreadable after the dry
	// run: the chunk proof cannot know what it names.
	rootDir := f.path("sha256/file-cas-roots/live-1")
	entries, err := os.ReadDir(rootDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no living root file to break: %v", err)
	}
	broken := filepath.Join(rootDir, entries[0].Name())
	if err := os.Remove(broken); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(rootDir, "nowhere"), broken); err != nil {
		t.Fatal(err)
	}
	before := f.snapshotOf("dead-1")
	report, aerr := f.apply(dry, namedReq("dead-1", "dead-2"))
	if aerr == nil {
		t.Fatalf("apply went on past a chunk proof it could not make: %+v", report.Apply)
	}
	f.allExist(c2, true)
	if !before.equal(f.snapshotOf("dead-1")) {
		t.Fatal("the computer whose chunks could not be proved lost files or rows")
	}
	if report.Apply.Deleted != 0 {
		t.Fatalf("the receipt says %d files were deleted", report.Apply.Deleted)
	}
}

func TestARefreshedFileFailsTheRunAndTheComputerIsNotMarkedDeleted(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(namedReq("dead-1"))
	var chunk string
	for _, rel := range f.treeFiles("dead-1") {
		if strings.Contains(rel, "file-cas-chunks") {
			chunk = rel
			break
		}
	}
	if chunk == "" {
		t.Fatal("no chunk to refresh")
	}
	now := time.Now()
	if err := os.Chtimes(f.path(chunk), now, now); err != nil {
		t.Fatal(err)
	}
	report, err := f.apply(dry, namedReq("dead-1"))
	if err == nil || !strings.Contains(err.Error(), "dead-1") {
		t.Fatalf("a run that kept a listed file answered %v", err)
	}
	if _, serr := os.Stat(f.path(chunk)); serr != nil {
		t.Fatalf("the refreshed file is gone: %v", serr)
	}
	if st := f.tombstoneStatus("dead-1"); st == tapeTombstoneDeleted {
		t.Fatal("a computer with a kept file is marked deleted")
	}
	if f.rowCount("computer_key_escrows", "dead-1") != 1 {
		t.Fatal("the escrow row of a computer with a kept file was shredded")
	}
	if report.Apply.ComputersDeleted != 0 || report.Apply.KeptRefreshed == 0 {
		t.Fatalf("the receipt does not say what happened: %+v", report.Apply)
	}
	if report.ReportPath == "" {
		t.Fatal("a failed run left no receipt")
	}
	if _, serr := os.Stat(report.ReportPath); serr != nil {
		t.Fatalf("the receipt file of a failed run is missing: %v", serr)
	}
}

func TestATombstonedComputerNotBeingDeletedStillCountsItsReferences(t *testing.T) {
	setup := func(t *testing.T) (*tapeFixture, tapeSeq, *tapeChain) {
		f := newTapeFixture(t)
		deadChain := f.fullComputer("dead-1", 1)
		x := f.chain("half-gone", 1, tapeAged)
		shared := []byte("payload dead-1 and the half-deleted computer both name")
		sd := f.appendSeq(deadChain, shared, tapeAged)
		f.appendSeq(x, shared, tapeAged)
		// A tombstone for a computer this run does not delete, deletion
		// unfinished: its chain rows are still there.
		if err := f.store.writeTapeTombstone(f.ctx, TapeTombstone{ComputerID: "half-gone", Status: tapeTombstoneArchived, HeadSequence: 2, CanonicalEventHead: platformTestDigest('1'), ManifestRun: "r", ManifestSHA256: platformTestDigest('2'), ProofJSON: "{}"}); err != nil {
			t.Fatal(err)
		}
		return f, sd, x
	}
	t.Run("readable: its payload is kept", func(t *testing.T) {
		f, sd, _ := setup(t)
		dry, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
		if err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if !f.exists(tapeNSPayload, sd.payload) {
			t.Fatal("dry run deleted a payload")
		}
		if _, err := f.apply(dry, namedReq("dead-1")); err != nil {
			t.Fatal(err)
		}
		if !f.exists(tapeNSPayload, sd.payload) {
			t.Fatal("the payload a tombstoned, unpurged computer names was deleted")
		}
	})
	t.Run("unreadable: the sweep is refused", func(t *testing.T) {
		f, sd, x := setup(t)
		if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, x.seqs[1].event))); err != nil {
			t.Fatal(err)
		}
		_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
		if err == nil || !strings.Contains(err.Error(), "unreadable") {
			t.Fatalf("a dry run that cannot read a tombstoned computer's references answered %v", err)
		}
		if !f.exists(tapeNSPayload, sd.payload) {
			t.Fatal("the payload was deleted")
		}
	})
}

// Round-2 review follow-ups (2026-11-24, both seats accepted; these are the
// should-fix items): the gate is not held across vmctl, a destroyed apply must
// name its computers, the caps are checked for the whole named run in phase
// one, and every named computer is archived before the first unlink.
//
//	R1  The exclusive gate is held while vmctl is asked about a computer, so
//	    every append stalls behind a slow vmctl.
//	    TestTheGateIsNotHeldWhileVmctlIsAsked
//	R2  The route accepts a destroyed apply with no computer_ids.
//	    TestTheRouteRefusesADestroyedApplyWithoutNamedComputers
//	R3  A named manifest over max_deletes/max_bytes is tombstoned and partly
//	    deleted instead of refused whole; and the clamp makes a larger run
//	    impossible.
//	    TestANamedApplyOverTheCapsIsRefusedWholeInPhaseOne
//	R4  Computer 2's archive fails after computer 1 is already deleted.
//	    TestEveryNamedComputerIsArchivedBeforeTheFirstUnlink

func TestTheGateIsNotHeldWhileVmctlIsAsked(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(namedReq("dead-1"))
	var asked, heldDuringCall int
	f.reg.onCall = func() {
		asked++
		if !f.store.tombstones.gate.TryRLock() {
			heldDuringCall++
			return
		}
		f.store.tombstones.gate.RUnlock()
	}
	if _, err := f.apply(dry, namedReq("dead-1")); err != nil {
		t.Fatal(err)
	}
	if asked == 0 {
		t.Fatal("the registry was never asked")
	}
	if heldDuringCall != 0 {
		t.Fatalf("the exclusive tombstone gate was held during %d of %d vmctl calls", heldDuringCall, asked)
	}
}

func TestTheRouteRefusesADestroyedApplyWithoutNamedComputers(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	req := TapeSweepRequest{Action: TapeSweepActionApply, Sweeps: []string{TapeSweepDestroyed}, AcceptUnrecordedForkLineage: true, RunID: newTapeSweepRunID(time.Now()), ManifestSHA256: platformTestDigest('a')}
	if _, err := f.svc.StartTapeSweep(req); err == nil || !strings.Contains(err.Error(), "computer_ids") {
		t.Fatalf("an unnamed destroyed apply through the route answered %v", err)
	}
	// A dry run without names is still a dry run.
	dry := req
	dry.Action, dry.RunID, dry.ManifestSHA256 = TapeSweepActionDryRun, "", ""
	if _, err := f.svc.StartTapeSweep(dry); err != nil {
		t.Fatalf("an unnamed dry run was refused: %v", err)
	}
}

func TestANamedApplyOverTheCapsIsRefusedWholeInPhaseOne(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 3)
	f.fullComputer("dead-2", 3)
	dry := f.dryRun(namedReq("dead-1", "dead-2"))
	req := namedReq("dead-1", "dead-2")
	req.MaxDeletes = 20
	before1, before2 := f.snapshotOf("dead-1"), f.snapshotOf("dead-2")
	report, err := f.apply(dry, req)
	if err == nil || !strings.Contains(err.Error(), "max_deletes") {
		t.Fatalf("a manifest over the cap answered %v", err)
	}
	if !before1.equal(f.snapshotOf("dead-1")) || !before2.equal(f.snapshotOf("dead-2")) {
		t.Fatal("files or rows were deleted")
	}
	if st := f.tombstoneStatus("dead-1") + f.tombstoneStatus("dead-2"); st != "" {
		t.Fatalf("a refused run left tombstones: %q", st)
	}
	if report.Apply.Deleted != 0 || report.Apply.DeferredByCap == 0 {
		t.Fatalf("receipt = %+v", report.Apply)
	}
	// The clamp does not make a larger named run impossible: an explicit cap
	// above the default is honored for a named request, never for an unnamed one.
	big := namedReq("dead-1")
	big.MaxBytes = 3 * DefaultTapeSweepMaxBytes
	big.MaxDeletes = 3 * DefaultTapeSweepMaxDeletes
	p, err := f.svc.clampTapeSweepRequest(withAction(big, TapeSweepActionDryRun))
	if err != nil || p.maxBytes != big.MaxBytes || p.maxDeletes != big.MaxDeletes {
		t.Fatalf("named caps clamped: %+v %v", p, err)
	}
	unnamed := destroyedReq(true)
	unnamed.MaxBytes = 3 * DefaultTapeSweepMaxBytes
	p, err = f.svc.clampTapeSweepRequest(withAction(unnamed, TapeSweepActionDryRun))
	if err != nil || p.maxBytes != DefaultTapeSweepMaxBytes {
		t.Fatalf("unnamed caps not clamped: %+v %v", p, err)
	}
	// And the named run applies under a cap that fits.
	req.MaxDeletes = 100000
	if _, err := f.apply(dry, req); err != nil {
		t.Fatal(err)
	}
}

func TestEveryNamedComputerIsArchivedBeforeTheFirstUnlink(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	c2 := f.fullComputer("dead-2", 1)
	req := namedReq("dead-1", "dead-2")
	req.NoArchive = false
	dry := f.dryRun(req)
	// dead-2 has a listed file nobody can read: its archive cannot be made.
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	unreadable := f.path(tapeSweepRel(tapeNSEvent, c2.seqs[0].event))
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(unreadable, 0o640)
	before1 := f.snapshotOf("dead-1")
	report, err := f.apply(dry, req)
	if err == nil {
		t.Fatalf("apply went on with an archive it could not make: %+v", report.Apply)
	}
	if !before1.equal(f.snapshotOf("dead-1")) || report.Apply.Deleted != 0 {
		t.Fatalf("dead-1 was deleted before dead-2 was archived: %+v", report.Apply)
	}
}
