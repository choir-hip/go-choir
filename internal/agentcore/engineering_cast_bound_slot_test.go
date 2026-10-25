package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// R5-2 (docs/evidence/selfdev-trace-reviews-2026-10-25.md round 5): Texture
// re-cast "continue the repair" (06:40:17) into the round whose assignment
// 81bbc97b was still bound, and management run fb555bae opened a second
// implementation assignment, 4513328f, for the same request. The second
// round then rebuilt from the base. A Cast to engineering for a request
// whose assignment is still open is refused, naming the bound engineer's
// inbox, so management sends the change there as a Note (P1).
//
// Ways this can fail, written first: the second Cast is admitted (the bug);
// it is refused but commits something; the refusal does not name the inbox
// to Note; the Note to the bound engineer is refused too; a Cast after the
// assignment ended is still refused; another request's assignment blocks it.
func TestEngineeringCastOntoABoundRequestIsRefusedAndTheNoteGoesThrough(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthorityForDocument(s, "owner-r5-2", rt.TextureComputerID(), "reader", 1)
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.SeedEngineeringAssignmentAuthorityForDocument(s, "owner-r5-2", rt.TextureComputerID(), "other", 1)
	if err != nil {
		t.Fatal(err)
	}
	bound, _ := c1eOpenBound(t, s, seed, 0)
	run := c1eManagementRun(t, s, seed)

	// Texture re-casts into the same open round.
	recast := store.TextureCastRecord("texture-cell:Cast:recast-r5-2", seed.TextureInbox, seed.ParentAgentID, "continue the authorized repair")
	req := types.CommitInboxRecordsRequest{OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, CommandID: "recast-r5-2", Records: []types.CommitmentRecord{recast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatalf("Texture re-cast: %v", err)
	}

	cast := func(parent string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{LocalID: "cast-eng", Kind: yaegikernel.IntentCast, ToDesk: agentprofile.Engineering,
			Objective: "continue the repair", Statement: `{"parent":"` + parent + `"}`}
	}
	rows := func() int {
		all, err := s.ListEngineeringAssignments(ctx, seed.OwnerID, seed.ComputerID, seed.TrajectoryID)
		if err != nil {
			t.Fatal(err)
		}
		return len(all)
	}
	before, mgmtHead := rows(), c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.ParentAgentID)
	for i, parent := range []string{"texture-cell:Cast:recast-r5-2", seed.TextureCastID} {
		reduction, cellCtx := c1eManagementReduction(rt, seed, run, "r5-2-cast-"+string(rune('a'+i)))
		err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{cast(parent)}, 1)
		if err == nil || !strings.Contains(err.Error(), bound.AssignmentID) || !strings.Contains(err.Error(), "choir.Note(\"engineering:"+bound.AssignmentID+"\"") {
			t.Fatalf("second Cast for a bound request (parent %s): err = %v", parent, err)
		}
		if rows() != before || c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, seed.ParentAgentID) != mgmtHead {
			t.Fatal("a refused Cast committed something")
		}
	}

	// The Note to the bound engineer goes through.
	engineeringHead := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, bound.Binding.AssignedAgentID)
	note := yaegikernel.StagedIntent{LocalID: "note-eng", Kind: yaegikernel.IntentNote, ToDesk: bound.Binding.AssignedAgentID, Body: "the owner corrected the spec: keep the ids pdf and epub"}
	reduction, cellCtx := c1eManagementReduction(rt, seed, run, "r5-2-note")
	if err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{note}, 1); err != nil {
		t.Fatalf("Note to the bound engineer: %v", err)
	}
	if got := c1eInboxHead(t, s, seed.OwnerID, seed.ComputerID, bound.Binding.AssignedAgentID); got != engineeringHead+1 {
		t.Fatalf("the engineer's inbox head %d -> %d; want the Note", engineeringHead, got)
	}

	// Another request's Cast is not blocked by this one's assignment.
	otherRun := c1eManagementRun(t, s, other)
	reduction, cellCtx = c1eManagementReduction(rt, other, otherRun, "r5-2-other")
	if err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{cast(other.TextureCastID)}, 1); err != nil && strings.Contains(err.Error(), "still") {
		t.Fatalf("another request's Cast was refused for this one's assignment: %v", err)
	}

	// Once the assignment has ended, a Cast for the request is not refused for it.
	c1eCancel(t, rt, bound, "ended", false)
	reduction, cellCtx = c1eManagementReduction(rt, seed, run, "r5-2-after")
	if err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{cast("texture-cell:Cast:recast-r5-2")}, 1); err != nil && strings.Contains(err.Error(), bound.AssignmentID) {
		t.Fatalf("a Cast after the assignment ended was refused for it: %v", err)
	}
}

// Review bunch 238 A: the store's open is idempotent ("a replayed cell
// replays the open"), and recoverPartialActCommit re-reduces a cell whose
// acts half committed, re-deriving the same Cast record. The R5-2 refusal
// must not pre-empt that replay: the assignment this same Cast opened (same
// record id, same casting run) is exempt, and any other Cast for the round
// is still refused. Ways it can fail, written first: the replay is refused
// as "still bound" (the bug); the exemption keys on the run alone, so a
// second Cast from the same run is admitted; it keys on the record alone.
func TestEngineeringCastReplayOfTheSameCellIsNotRefused(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthorityForDocument(s, "owner-r5-2-replay", rt.TextureComputerID(), "reader", 1)
	if err != nil {
		t.Fatal(err)
	}
	run := c1eManagementRun(t, s, seed)
	in := yaegikernel.StagedIntent{LocalID: "cast-eng", Kind: yaegikernel.IntentCast, ToDesk: agentprofile.Engineering,
		Objective: "repair the Reader", Statement: `{"parent":"` + seed.TextureCastID + `"}`}
	const cell = "r5-2-replay-cell"
	reduction, cellCtx := c1eManagementReduction(rt, seed, run, cell)

	// The first commit of the cell opened the assignment (as
	// openDelegatedCastAssignment does: the Cast record id is the cell's).
	recordID := commitmentRecordForIntent(reduction.scope, in).RecordID
	open := store.SeedEngineeringOpenRequest(seed, 0, "cap-r5-2-replay")
	open.Binding.ParentControlID, open.Binding.ParentRunID = recordID, run.RunID
	open.Records.Records = []types.CommitmentRecord{store.EngineeringCastRecord(recordID, seed.ParentAgentID, open.AssignmentID, seed.TextureCastID, "repair the Reader")}
	open.Records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(open.Records)
	open.CommandDigest, _ = store.ComputeOpenEngineeringAssignmentDigest(open)
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatalf("first commit's open: %v", err)
	}

	// The replay of the same cell passes the R5-2 check (it reaches the
	// source preflight, which off a Linux guest is unavailable; on the guest
	// the store replays the open).
	if err := reduction.commitTray(cellCtx, []yaegikernel.StagedIntent{in}, 1); err != nil && strings.Contains(err.Error(), "no second assignment opens") {
		t.Fatalf("the replay of the cell that opened the assignment was refused: %v", err)
	}
	// Another cell of the same run is a second Cast, and is refused.
	other, otherCtx := c1eManagementReduction(rt, seed, run, "r5-2-replay-other")
	if err := other.commitTray(otherCtx, []yaegikernel.StagedIntent{in}, 1); err == nil || !strings.Contains(err.Error(), "no second assignment opens") {
		t.Fatalf("a second Cast from the same run: err = %v", err)
	}
}
