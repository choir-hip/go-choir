package platform

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// The owner ruled on 2026-11-24 ("yes, delete tape of deleted computer") that
// the tape segments and snapshot chunks of five removed computers are deleted.
// The destroyed sweep already proves and deletes a destroyed computer's tape,
// but it acts on every computer it proves destroyed (nineteen on Node B) and
// only an acknowledgement stands between a dry run and all of them. The named
// form takes the exact list of ids.
//
// Every way this can fail, written before the code, and the test that pins it:
//
//	N1  A computer nobody named is deleted because it is also destroyed.
//	    TestNamedSweepTouchesOnlyTheNamedComputers
//	N2  A named computer that has an ownership record (or is listed, or has a
//	    route, or is protected, or has no head at all) is deleted, or is skipped
//	    silently so the operator believes it was handled.
//	    TestNamedSweepRefusesWhatItCannotDelete
//	N3  Apply runs with a list other than the one the dry run was made for, or
//	    with none, or the dry run had none and apply names some.
//	    TestNamedApplyMustRepeatTheExactList
//	N4  A computer gains an owner between the dry run and the apply and is
//	    deleted anyway, or the apply reports success while skipping it.
//	    TestNamedApplyFailsAndDeletesNothingWhenAComputerGainedAnOwner
//	N5  (moved to tape_sweep_closed_test.go: a shared digest now refuses.)
//	N6  The dry run does not say, per computer, what it would delete, when the
//	    computer last wrote, and what the ownership lookup answered; and the
//	    receipt does not say which ids it was for.
//	    TestNamedSweepReportsEachComputerAndLeavesAReceipt
//	N7  The named form combined with the orphan sweep, an unknown sweep, a
//	    malformed id or a very long list.
//	    TestNamedRequestShape

// namedReq is a named request: the fork-lineage acknowledgement is given per
// computer, for exactly the named ones, never as the global flag.
func namedReq(ids ...string) TapeSweepRequest {
	return TapeSweepRequest{Sweeps: []string{TapeSweepDestroyed}, NoArchive: true, ComputerIDs: ids, AcceptUnrecordedForkLineageFor: ids}
}

func TestNamedSweepTouchesOnlyTheNamedComputers(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	f.fullComputer("live-1", 2)
	f.fullComputer("dead-1", 2)
	f.fullComputer("dead-2", 2)
	otherFiles := f.treeFiles("dead-2")
	otherRows := f.rowCount("computer_key_escrows", "dead-2")

	dry := f.dryRun(namedReq("dead-1"))
	if dry.DestroyedListed != 1 || len(dry.ComputerIDs) != 1 || dry.ComputerIDs[0] != "dead-1" {
		t.Fatalf("dry run listed %d computers for ids %v", dry.DestroyedListed, dry.ComputerIDs)
	}
	for _, it := range f.manifestItems(dry) {
		if it.Computer != "dead-1" {
			t.Fatalf("manifest lists %s, which nobody named: %+v", it.Computer, it)
		}
	}
	if _, listed := f.manifestRows(dry)["dead-2"]; listed {
		t.Fatal("manifest holds rows of a computer nobody named")
	}
	if _, err := f.apply(dry, namedReq("dead-1")); err != nil {
		t.Fatal(err)
	}
	if got := f.treeFiles("dead-1"); len(got) != 0 {
		t.Fatalf("the named computer's files survived: %v", got)
	}
	if got := f.treeFiles("dead-2"); strings.Join(got, ",") != strings.Join(otherFiles, ",") {
		t.Fatalf("an unnamed destroyed computer lost files: %v want %v", got, otherFiles)
	}
	if f.rowCount("computer_key_escrows", "dead-2") != otherRows || f.tombstoneStatus("dead-2") != "" {
		t.Fatal("an unnamed destroyed computer's rows or tombstone changed")
	}
}

func TestNamedSweepRefusesWhatItCannotDelete(t *testing.T) {
	cases := []struct {
		name  string
		setup func(f *tapeFixture)
		id    string
		want  string
	}{
		{"an ownership record", func(f *tapeFixture) { f.reg.owned["dead-1"] = true }, "dead-1", "ownership record exists"},
		{"a registry entry", func(f *tapeFixture) { f.reg.listed["dead-1"] = true }, "dead-1", "in registry list"},
		{"the paper", func(f *tapeFixture) { f.fullComputer(purgePaperID, 1) }, purgePaperID, "protected"},
		{"the owner's computer", func(f *tapeFixture) { f.fullComputer(purgeOwnerID, 1) }, purgeOwnerID, "protected"},
		{"no head at all", func(f *tapeFixture) {}, "never-existed", "no head"},
		{"a head that moved inside the quiet window", func(f *tapeFixture) {
			f.chain("recent-1", 1, 0)
		}, "recent-1", "quiet window"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newTapeFixture(t)
			f.fullComputer("dead-1", 1)
			c.setup(f)
			before := f.treeFiles(c.id)
			_, err := f.svc.RunTapeSweep(f.ctx, withAction(namedReq(c.id), TapeSweepActionDryRun))
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), c.id) {
				t.Fatalf("dry run for %s answered %v, want a refusal naming the id and %q", c.id, err, c.want)
			}
			if got := f.treeFiles(c.id); strings.Join(got, ",") != strings.Join(before, ",") {
				t.Fatalf("a refused dry run changed files: %v", got)
			}
		})
	}
}

func withAction(r TapeSweepRequest, action string) TapeSweepRequest {
	r.Action = action
	return r
}

func TestNamedApplyMustRepeatTheExactList(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	f.fullComputer("dead-2", 1)
	dry := f.dryRun(namedReq("dead-1", "dead-2"))
	for name, ids := range map[string][]string{
		"fewer":     {"dead-1"},
		"more":      {"dead-1", "dead-2", "dead-3"},
		"different": {"dead-1", "dead-3"},
		"none":      nil,
	} {
		before1, before2 := f.treeFiles("dead-1"), f.treeFiles("dead-2")
		if _, err := f.apply(dry, namedReq(ids...)); err == nil {
			t.Fatalf("%s: apply with %v was accepted for a dry run of dead-1 and dead-2", name, ids)
		}
		if len(f.treeFiles("dead-1")) != len(before1) || len(f.treeFiles("dead-2")) != len(before2) {
			t.Fatalf("%s: a refused apply deleted files", name)
		}
	}
	// A dry run made without a list cannot be applied with one.
	f2 := newTapeFixture(t)
	f2.fullComputer("dead-1", 1)
	unnamed := f2.dryRun(destroyedReq(true))
	if _, err := f2.apply(unnamed, namedReq("dead-1")); err == nil {
		t.Fatal("a manifest made for every destroyed computer was applied with a list")
	}
	// The list in a different order is the same list.
	if _, err := f.apply(dry, namedReq("dead-2", "dead-1")); err != nil {
		t.Fatalf("the same ids in another order were refused: %v", err)
	}
	if len(f.treeFiles("dead-1"))+len(f.treeFiles("dead-2")) != 0 {
		t.Fatal("the matching apply left files")
	}
}

func TestNamedApplyFailsAndDeletesNothingWhenAComputerGainedAnOwner(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	f.fullComputer("dead-2", 1)
	dry := f.dryRun(namedReq("dead-1", "dead-2"))
	f.reg.owned["dead-2"] = true
	before1, before2 := f.snapshotOf("dead-1"), f.snapshotOf("dead-2")
	report, err := f.apply(dry, namedReq("dead-1", "dead-2"))
	if err == nil || !strings.Contains(err.Error(), "dead-2") {
		t.Fatalf("apply reported success (%v) while dead-2 gained an owner: %+v", err, report.Apply)
	}
	// All or nothing: the other named computer is not deleted either, so a
	// payload they share is never taken from the one that survived.
	if !before1.equal(f.snapshotOf("dead-1")) || !before2.equal(f.snapshotOf("dead-2")) {
		t.Fatal("a refused run changed files or rows")
	}
}

// chunkDigest is the digest of a chunk body.
func chunkDigest(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func TestNamedSweepReportsEachComputerAndLeavesAReceipt(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 3)
	f.fullComputer("dead-2", 1)
	dry := f.dryRun(namedReq("dead-2", "dead-1"))
	if len(dry.ComputerIDs) != 2 || dry.ComputerIDs[0] != "dead-1" || dry.ComputerIDs[1] != "dead-2" {
		t.Fatalf("the dry run's list is not sorted ids: %v", dry.ComputerIDs)
	}
	if len(dry.DestroyedProofs) != 2 {
		t.Fatalf("proofs = %+v", dry.DestroyedProofs)
	}
	for _, pr := range dry.DestroyedProofs {
		if pr.OwnershipLookup != "not_found" || pr.HeadUpdatedAt == "" || !pr.Deletable || pr.InRegistryList || pr.HasRoute {
			t.Fatalf("proof lacks the ownership answer or last activity: %+v", pr)
		}
		seg, chunks := pr.Classes[tapeNSSeg], pr.Classes[tapeNSFileCASChunks]
		if seg.Files != 1 || seg.Bytes == 0 || chunks.Files != 3 || chunks.Bytes == 0 || pr.Classes[tapeNSFileCASRoots].Files != 1 {
			t.Fatalf("proof %s has no bytes by class: %+v", pr.ComputerID, pr.Classes)
		}
		if pr.Classes[tapeNSPayload].Files == 0 {
			t.Fatalf("proof %s lists no loose tape files: %+v", pr.ComputerID, pr.Classes)
		}
	}
	lines := f.manifestLines(dry)
	var head tapeSweepManifestHeader
	if err := json.Unmarshal(lines[0], &head); err != nil || strings.Join(head.Computers, ",") != "dead-1,dead-2" {
		t.Fatalf("the manifest header does not carry the list the hash covers: %+v %v", head, err)
	}
	applied, err := f.apply(dry, namedReq("dead-1", "dead-2"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(applied.ComputerIDs, ",") != "dead-1,dead-2" || applied.Apply.ComputersDeleted != 2 {
		t.Fatalf("the apply receipt does not say which computers it deleted: %+v", applied)
	}
	raw, err := os.ReadFile(applied.ReportPath)
	if err != nil || !strings.Contains(string(raw), `"dead-1"`) {
		t.Fatalf("the receipt file does not carry the ids: %v", err)
	}
	if f.tombstoneStatus("dead-1") != tapeTombstoneDeleted || f.tombstoneStatus("dead-2") != tapeTombstoneDeleted {
		t.Fatal("the named computers are not tombstoned deleted")
	}
	t.Logf("dry-run proof sample:\n%s", func() string { b, _ := json.MarshalIndent(dry.DestroyedProofs[0], "", " "); return string(b) }())
}

func TestNamedRequestShape(t *testing.T) {
	f := newTapeFixture(t)
	f.fullComputer("dead-1", 1)
	long := make([]string, 65)
	for i := range long {
		long[i] = fmt.Sprintf("dead-%d", i)
	}
	cases := map[string]TapeSweepRequest{
		"with the orphan sweep": {Sweeps: []string{TapeSweepOrphans, TapeSweepDestroyed}, AcceptUnrecordedForkLineageFor: []string{"dead-1"}, ComputerIDs: []string{"dead-1"}},
		"with every sweep":      {AcceptUnrecordedForkLineageFor: []string{"dead-1"}, ComputerIDs: []string{"dead-1"}},
		"a path as an id":       namedReq("../dead-1"),
		"an empty id":           namedReq("dead-1", " "),
		"a duplicate":           namedReq("dead-1", "dead-1"),
		"sixty-five ids":        namedReq(long...),
		"without the ack":       {Sweeps: []string{TapeSweepDestroyed}, ComputerIDs: []string{"dead-1"}},
		"the global ack":        {Sweeps: []string{TapeSweepDestroyed}, ComputerIDs: []string{"dead-1"}, AcceptUnrecordedForkLineage: true},
		"an ack for another":    {Sweeps: []string{TapeSweepDestroyed}, ComputerIDs: []string{"dead-1"}, AcceptUnrecordedForkLineageFor: []string{"dead-2"}},
		"an ack for more":       {Sweeps: []string{TapeSweepDestroyed}, ComputerIDs: []string{"dead-1"}, AcceptUnrecordedForkLineageFor: []string{"dead-1", "dead-2"}},
	}
	for name, req := range cases {
		if _, err := f.svc.RunTapeSweep(f.ctx, withAction(req, TapeSweepActionDryRun)); err == nil {
			t.Fatalf("%s: the request was accepted", name)
		}
	}
}
