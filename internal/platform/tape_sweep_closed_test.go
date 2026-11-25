package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computertiers"
)

// The reviewers of the named sweep (2026-11-24) found that every source of
// "this computer is not living" failed open. The invariant now: each source
// fails CLOSED when it is missing, empty, stale or unreadable, and the delete
// is bound to the dry-run manifest and re-proved at apply. Written before the
// code; each test was seen failing first.
//
//	K1  An empty registry while event heads exist is not "nobody owns anything".
//	K2  A registry that does not list a protected computer that has a head is
//	    stale or wrong: refuse.
//	K3  A living computer's unreadable event drops its payload refs from the
//	    live set: refuse, and refuse again at apply when a later event is
//	    unreadable.
//	K4  A chunk digest a living snapshot root, an in-flight registration, a
//	    living file root or a destroyed computer that is not being purged names:
//	    refuse that computer. A digest two named computers share is fine.
//	K5  Fork lineage is acknowledged per computer, on the command line, never as
//	    the global flag.
//	K6  A named computer not fully deleted makes the apply fail (a cap mid
//	    computer, a rows failure), and the cap leaves no false "archived".
//	K7  A pending lifecycle operation refuses the computer.
//	K8  The protected list is the tiers' platform and proof computers too.
//	K9  An extra-refs query failure during a named dry run is an error, not an
//	    empty manifest with a hash.

func (f *tapeFixture) emptyRegistry() {
	f.reg.owned = map[string]bool{}
	f.reg.listed = map[string]bool{}
}

func TestSweepRefusesAnEmptyRegistryWhileHeadsExist(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	f.emptyRegistry()
	_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
	if err == nil || !strings.Contains(err.Error(), "registry") {
		t.Fatalf("named dry run with an empty registry answered %v", err)
	}
	dry, err := f.svc.RunTapeSweep(f.ctx, withAction(destroyedReq(true), TapeSweepActionDryRun))
	if err != nil || dry.DestroyedListed != 0 || !strings.Contains(dry.DestroyedRefused, "registry") {
		t.Fatalf("unnamed dry run listed %d with refused=%q err=%v", dry.DestroyedListed, dry.DestroyedRefused, err)
	}
	if len(f.treeFiles("dead-1")) == 0 {
		t.Fatal("files were deleted")
	}
}

func TestApplyRefusesWhenTheRegistryWentEmptyAfterTheDryRun(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(namedReq("dead-1"))
	f.emptyRegistry()
	if _, err := f.apply(dry, namedReq("dead-1")); err == nil {
		t.Fatal("apply with an empty registry succeeded")
	}
	if len(f.treeFiles("dead-1")) == 0 {
		t.Fatal("files were deleted against an empty registry")
	}
}

func TestSweepRefusesARegistryThatDoesNotListAProtectedComputer(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	f.fullComputer(purgePaperID, 1) // has a head; the registry does not list it
	_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
	if err == nil || !strings.Contains(err.Error(), purgePaperID) {
		t.Fatalf("a registry without the paper was accepted: %v", err)
	}
}

func TestSweepRefusesWhenALivingChainIsUnreadable(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	live := f.chain("live-1", 1, tapeAged)
	dead := f.chain("dead-1", 1, tapeAged)
	shared := []byte("payload both name")
	f.appendSeq(live, shared, tapeAged)
	deadShared := f.appendSeq(dead, shared, tapeAged)
	f.ownTree("dead-1", tapeAged)
	// The living computer's second event is gone from the host: its payload
	// reference to the shared payload can no longer be read.
	if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, live.seqs[1].event))); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
	if err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("a dry run with an unreadable living chain answered %v", err)
	}
	if !f.exists(tapeNSPayload, deadShared.payload) {
		t.Fatal("the shared payload was deleted")
	}
}

func TestApplyRefusesWhenALivingEventAfterTheDryRunIsUnreadable(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	live := f.chain("live-1", 1, tapeAged)
	dead := f.chain("dead-1", 1, tapeAged)
	shared := []byte("payload both name")
	deadShared := f.appendSeq(dead, shared, tapeAged)
	f.ownTree("dead-1", tapeAged)
	dry := f.dryRun(namedReq("dead-1"))
	added := f.appendSeq(live, shared, 0)
	if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, added.event))); err != nil {
		t.Fatal(err)
	}
	if _, err := f.apply(dry, namedReq("dead-1")); err == nil {
		t.Fatal("apply went on with an unreadable event in the delta")
	}
	if !f.exists(tapeNSPayload, deadShared.payload) {
		t.Fatal("the shared payload was deleted")
	}
}

func TestSweepRefusesAComputerWhoseChunkDigestSomethingLivingNames(t *testing.T) {
	body := "sealed chunk dead-1 0" // the first chunk ownTree writes for dead-1
	shared := chunkDigest(body)
	cases := map[string]func(f *tapeFixture){
		"a living snapshot root": func(f *tapeFixture) {
			f.live("live-1")
			f.fullComputer("live-1", 1)
			if err := f.store.recordSnapshotRoot(f.ctx, "live-1", platformTestDigest('9'), 2, []string{shared}); err != nil {
				t.Fatal(err)
			}
		},
		"a registration in flight": func(f *tapeFixture) {
			f.live("live-1")
			f.fullComputer("live-1", 1)
			f.store.snapshotPending.add([]string{snapshotChunkKey("live-1", shared)})
		},
		"a living computer's file root": func(f *tapeFixture) {
			f.live("live-1")
			f.fullComputer("live-1", 1)
			rel := "sha256/file-cas-roots/live-1/" + platformTestDigest('7') + ".json"
			if err := os.MkdirAll(filepath.Dir(f.path(rel)), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.path(rel), []byte(`{"files":[{"chunks":["`+shared+`"]}]}`), 0o640); err != nil {
				t.Fatal(err)
			}
		},
		"a destroyed computer this run does not purge": func(f *tapeFixture) {
			f.fullComputer("dead-2", 1)
			if err := f.store.recordSnapshotRoot(f.ctx, "dead-2", platformTestDigest('8'), 2, []string{shared}); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newTapeFixture(t)
			f.fullComputer("dead-1", 1)
			setup(f)
			before := f.treeFiles("dead-1")
			_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
			if err == nil || !strings.Contains(err.Error(), "snapshot") || !strings.Contains(err.Error(), "dead-1") {
				t.Fatalf("a shared chunk digest was not refused: %v", err)
			}
			if strings.Join(f.treeFiles("dead-1"), ",") != strings.Join(before, ",") {
				t.Fatal("files changed")
			}
		})
	}
	t.Run("a digest two named computers share is deleted from both", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 1)
		f.fullComputer("dead-2", 1)
		if err := f.store.recordSnapshotRoot(f.ctx, "dead-2", platformTestDigest('8'), 2, []string{shared}); err != nil {
			t.Fatal(err)
		}
		dry := f.dryRun(namedReq("dead-1", "dead-2"))
		if _, err := f.apply(dry, namedReq("dead-1", "dead-2")); err != nil {
			t.Fatal(err)
		}
		if len(f.treeFiles("dead-1"))+len(f.treeFiles("dead-2")) != 0 {
			t.Fatal("files survived")
		}
	})
}

func TestApplyReprovesTheChunkDigestsAgainstWhatIsLivingNow(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.fullComputer("live-1", 1)
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(namedReq("dead-1"))
	shared := chunkDigest("sealed chunk dead-1 0")
	if err := f.store.recordSnapshotRoot(f.ctx, "live-1", platformTestDigest('9'), 3, []string{shared}); err != nil {
		t.Fatal(err)
	}
	before := f.treeFiles("dead-1")
	if _, err := f.apply(dry, namedReq("dead-1")); err == nil {
		t.Fatal("apply deleted a computer whose chunk a living snapshot now names")
	}
	if strings.Join(f.treeFiles("dead-1"), ",") != strings.Join(before, ",") {
		t.Fatal("files changed")
	}
}

func TestForkLineageIsAcknowledgedPerComputerOnTheRequest(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	f.fullComputer("dead-2", 1)
	dry := f.dryRun(namedReq("dead-1", "dead-2"))
	if !dry.DestroyedProofs[0].AcceptUnrecordedForkLineage || strings.Join(dry.ForkLineageAcknowledgedFor, ",") != "dead-1,dead-2" {
		t.Fatalf("the report does not say whom the acknowledgement named: %+v", dry.ForkLineageAcknowledgedFor)
	}
	// Apply must repeat it exactly.
	req := namedReq("dead-1", "dead-2")
	req.AcceptUnrecordedForkLineageFor = []string{"dead-1"}
	if _, err := f.apply(dry, req); err == nil {
		t.Fatal("apply with a different acknowledgement was accepted")
	}
	req.AcceptUnrecordedForkLineageFor = nil
	if _, err := f.apply(dry, req); err == nil {
		t.Fatal("apply with no acknowledgement was accepted")
	}
	if len(f.treeFiles("dead-1")) == 0 {
		t.Fatal("a refused apply deleted files")
	}
}

func TestNamedApplyFailsWhenAComputerIsNotFullyDeleted(t *testing.T) {
	t.Run("a cap mid computer", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 3)
		dry := f.dryRun(namedReq("dead-1"))
		req := namedReq("dead-1")
		req.MaxDeletes = 4
		report, err := f.apply(dry, req)
		if err == nil || !strings.Contains(err.Error(), "dead-1") {
			t.Fatalf("a capped apply answered %v (%+v)", err, report.Apply)
		}
		if report.Apply.DeferredByCap == 0 {
			t.Fatalf("the cap did not bite: %+v", report.Apply)
		}
		if st := f.tombstoneStatus("dead-1"); st == tapeTombstoneDeleted {
			t.Fatal("a partly deleted computer is marked deleted")
		}
	})
	t.Run("the rows transaction fails", func(t *testing.T) {
		f := newTapeFixture(t)
		f.fullComputer("dead-1", 1)
		dry := f.dryRun(namedReq("dead-1"))
		f.svc.tapeSweep.beforeRowsDelete = func(string) error { return os.ErrPermission }
		if _, err := f.apply(dry, namedReq("dead-1")); err == nil {
			t.Fatal("a rows failure left the apply successful")
		}
	})
}

func TestSweepRefusesAComputerWithAPendingLifecycleOperation(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	if _, err := f.store.db.Exec(`INSERT INTO computer_lifecycle_operations (computer_id,idempotency_key,request_commitment,action,prior_lifecycle_state,prior_realization_epoch,status,created_at) VALUES (?,?,?,?,?,?,?,?)`,
		"dead-1", "k1", platformTestDigest('a'), "resume", "hibernated", 1, "pending", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
	if err == nil || !strings.Contains(err.Error(), "lifecycle") {
		t.Fatalf("a computer with a pending operation answered %v", err)
	}
}

func TestProtectedListIncludesThePlatformTiers(t *testing.T) {
	tiers, err := computertiers.Load()
	if err != nil {
		t.Fatal(err)
	}
	ids := append(tiers.PlatformComputerIDs(), purgePaperID, purgeOwnerID)
	for _, id := range ids {
		if tapeSweepProtected(id) == nil {
			t.Fatalf("%s is not protected", id)
		}
	}
	if tapeSweepProtected("dead-1") != nil {
		t.Fatal("an ordinary computer is protected")
	}
}

func TestNamedDryRunFailsWhenTheExtraRefsCannotBeRead(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	if _, err := f.store.db.Exec(`ALTER TABLE computer_version_code_closures RENAME TO computer_version_code_closures_gone`); err != nil {
		t.Skipf("cannot break the extra-refs query here: %v", err)
	}
	_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq("dead-1"), TapeSweepActionDryRun))
	if err == nil {
		t.Fatal("a named dry run that could not read the extra references answered success")
	}
}

func TestReportCarriesWhatTheNamedRunProved(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	dry := f.dryRun(namedReq("dead-1"))
	raw, _ := json.Marshal(dry)
	for _, want := range []string{`"registry_computers"`, `"fork_lineage_acknowledged_for"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("the report lacks %s: %s", want, raw)
		}
	}
}
