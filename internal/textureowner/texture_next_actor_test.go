package textureowner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/problems/texture-write-leaves-owner-request-unattended-2026-11-04.md.
// Staging, claude arm, 01:45:08Z: an owner revision opened Texture work, the
// desk's cell wrote only (no Cast, Wait or Done), the turn committed with the
// work open, and the desk passivated with nothing working on the request.
// Ways the fix can fail, written before it:
//   - the Write-only turn still commits, leaving the request open silently;
//   - the refusal commits part of the turn (a revision, a cursor) anyway;
//   - the fixes the refusal names (Write with Wait, Done or a Cast) are
//     themselves refused, or Write with Wait loses its why;
//   - a legitimate Write-only turn is refused: one while a round Texture
//     cast is open, or one after the request is already completed;
//   - the activation-end Idle turn leaves the request open with no visible
//     fate, or closes a request that something is working on, or closes it
//     for an activation that acted off the document;
//   - the closed request cannot be reopened by the owner's next revise, or
//     the owner and management do not see it.

func writeTurn(base string, edit func(*yaegikernel.TextureTurn)) yaegikernel.TextureTurn {
	turn := yaegikernel.TextureTurn{BaseRevisionID: base, Write: &yaegikernel.TextureWrite{Edits: "@replace b3\nA new closing line for the next round.", Why: "set up the next round"}}
	if edit != nil {
		edit(&turn)
	}
	return turn
}

func textureWork(t *testing.T, s *store.Store, rec types.RunRecord) types.WorkItemRecord {
	t.Helper()
	snapshot, err := s.GetLifecycleSnapshot(context.Background(), rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range snapshot.WorkItems {
		if w.WorkItemID == metadataStringValue(rec.Metadata, "lifecycle_work_item_id") {
			return w
		}
	}
	t.Fatal("the Texture work item is gone")
	return types.WorkItemRecord{}
}

func researchCast(rec types.RunRecord, id string) types.CommitmentRecord {
	texture, research := rec.AgentID, agentprofile.Research+":"+rec.ChannelID
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover the ruling", Body: `{"kind":"question","summary":"ruling"}`},
		Cast:      &types.CommitmentCast{}, Inbox: research, SenderInbox: texture, Addressee: research, ComputerID: rec.ComputerID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
	}
}

func TestWriteOnlyTurnWithTheOwnersRequestOpenAndNothingWorkingIsRefused(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unattended-write")
	ctx := context.Background()
	base := headOf(t, s, rec)
	_, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, writeTurn(base.RevisionID, nil)), "write-only")
	if err == nil || !strings.Contains(err.Error(), "the owner's request is still open and nothing is working on it") ||
		!strings.Contains(err.Error(), "Cast to start the work, Wait(why) if it waits on something named, or Done(why) if this revision answers it") {
		t.Fatalf("Write-only turn = %v, want the next-actor refusal", err)
	}
	if head := headOf(t, s, rec); head.RevisionID != base.RevisionID {
		t.Fatalf("the refused turn moved the head to %s", head.RevisionID)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s, want still open", w.Status)
	}

	// The same Write with a Wait naming what it waits on commits: the
	// revision lands, the request stays open, and the why is on the turn.
	withWait := writeTurn(base.RevisionID, func(turn *yaegikernel.TextureTurn) { turn.Wait = "the owner said they will send the figures" })
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, withWait), "write-wait"); err != nil {
		t.Fatalf("Write with Wait: %v", err)
	}
	v1 := headOf(t, s, rec)
	if v1.RevisionID == base.RevisionID {
		t.Fatal("Write with Wait committed no revision")
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work after Write with Wait = %s, want open", w.Status)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range snapshot.Events {
		if ev.Kind == types.LifecycleTextureTurnCommitted && strings.Contains(ev.Reason, "waiting: the owner said they will send the figures") {
			found = true
		}
	}
	if !found {
		t.Fatal("the Wait's why did not commit with the revision")
	}

	// With a Cast in the cell (the dispatch fold), the Write commits and
	// the round it opens is the next actor.
	withCast := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:next-actor")},
		Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nResearch is checking the ruling.", Why: "say what is being checked"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, withCast, "write-cast"); err != nil {
		t.Fatalf("Write with a Cast: %v", err)
	}
	v2 := headOf(t, s, rec)

	// While that round is open, a Write-only turn is a progress revision
	// and commits: research owes the desk a Report.
	progress := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v2.RevisionID,
		Write: &yaegikernel.TextureWrite{Edits: "@replace b3\nThe first filings are in.", Why: "progress"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, progress, "write-progress"); err != nil {
		t.Fatalf("Write-only turn while a cast round is open: %v", err)
	}
}

func TestWriteWithDoneCompletesAndALaterWriteOnlyTurnCommits(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unattended-done")
	ctx := context.Background()
	base := headOf(t, s, rec)
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, writeTurn(base.RevisionID, func(turn *yaegikernel.TextureTurn) { turn.Done = "the revision answers the request" })), "write-done"); err != nil {
		t.Fatalf("Write with Done: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemCompleted {
		t.Fatalf("work = %s, want completed", w.Status)
	}
	// No request is open, so a later Write-only turn (a records-route
	// correction) has nothing to leave unattended.
	rec.Metadata[textureRecordsRouteMetadata] = true
	v1 := headOf(t, s, rec)
	later := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Write: &yaegikernel.TextureWrite{Edits: "@replace b2\nA corrected opening.", Why: "correct"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, later, "write-later"); err != nil {
		t.Fatalf("Write-only turn with no open request: %v", err)
	}
}

func TestIdleTurnWithTheOwnersRequestUnattendedClosesItVisibly(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "unattended-idle")
	ctx := context.Background()
	base := headOf(t, s, rec)
	idle := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Idle: agentcore.TextureIdleNoActReason})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, idle, "idle-1"); err != nil {
		t.Fatalf("Idle turn: %v", err)
	}
	w := textureWork(t, s, rec)
	if w.Status != types.WorkItemRefused || !strings.HasPrefix(w.Reason, UnattendedRequestReason) || !strings.Contains(w.Reason, rec.RunID) {
		t.Fatalf("work = %s %q, want refused as unattended naming the run", w.Status, w.Reason)
	}
	// A retried activation end replays both commits.
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, idle, "idle-1"); err != nil {
		t.Fatalf("replayed Idle turn: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	// The owner sees it on the document; the owner's next revise reopens.
	if got := textureRequestUnattended(snapshot, rec.ChannelID); !strings.HasPrefix(got, UnattendedRequestReason) {
		t.Fatalf("document unattended state = %q", got)
	}
	if wait := textureRequestUnattendedWait(snapshot, rec.ChannelID, time.Now()); wait == nil || wait.Kind != types.WaitKindNoNextActor ||
		wait.Class != types.WaitClassPolicy || !strings.Contains(wait.Sentence, "no one is working on this request") {
		t.Fatalf("document wait record = %+v, want a no_next_actor policy wait", wait)
	}
	if !textureWorkSettledByDesk(snapshot, rec.ChannelID) {
		t.Fatal("an unattended request does not let the owner's next revise reopen work")
	}
	// Management sees it in its inbox, as a system notice that wakes no one.
	records, err := s.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, agentprofile.Management+":"+rec.OwnerID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	notices := 0
	for _, r := range records {
		if types.IsUnattendedRequestNotice(r) && strings.Contains(r.Directive.Body, w.WorkItemID) && strings.Contains(r.Directive.Body, "Wait record: waiting for the owner's next revise") {
			notices++
			if store.RecordWakesReader(r) {
				t.Fatal("the notice wakes management")
			}
		}
	}
	if notices != 1 {
		t.Fatalf("management notices = %d, want 1 (records %+v)", notices, records)
	}
}

func TestIdleTurnLeavesTheRequestOpenWhenSomethingWorksOnIt(t *testing.T) {
	t.Run("an open round Texture cast", func(t *testing.T) {
		h, s, rec := cellAuthorSetup(t, "unattended-idle-round")
		ctx := context.Background()
		base := headOf(t, s, rec)
		wait := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research", InboxRecords: []types.CommitmentRecord{researchCast(rec, "texture-cell:cast:idle-round")}})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, wait, "wait-cast"); err != nil {
			t.Fatalf("Wait with a Cast: %v", err)
		}
		idle := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Idle: agentcore.TextureIdleNoActReason})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, idle, "idle-round"); err != nil {
			t.Fatalf("Idle turn: %v", err)
		}
		if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
			t.Fatalf("work = %s %q, want open while research owes a Report", w.Status, w.Reason)
		}
	})
	t.Run("an activation that acted off the document", func(t *testing.T) {
		h, s, rec := cellAuthorSetup(t, "unattended-idle-offdoc")
		ctx := context.Background()
		base := headOf(t, s, rec)
		idle := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Idle: agentcore.TextureIdleOffDocReason})
		if _, err := h.CommitCellTextureAuthor(ctx, &rec, idle, "idle-offdoc"); err != nil {
			t.Fatalf("Idle turn: %v", err)
		}
		if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
			t.Fatalf("work = %s, want open", w.Status)
		}
	})
}

func TestNextActorReadsTheTrajectory(t *testing.T) {
	agent := agentprofile.Texture + ":doc-1"
	open := types.WorkItemRecord{WorkItemID: "w1", Status: types.WorkItemOpen, AssignedAgentID: agent, AuthorityProfile: agentprofile.Texture}
	for _, tc := range []struct {
		name string
		snap types.LifecycleSnapshot
		want bool
	}{
		{"document with open Texture work", types.LifecycleSnapshot{Trajectory: types.TrajectoryRecord{Kind: types.TrajectoryKindDocument}, WorkItems: []types.WorkItemRecord{open}}, true},
		{"task trajectory", types.LifecycleSnapshot{Trajectory: types.TrajectoryRecord{Kind: types.TrajectoryKindTask}, WorkItems: []types.WorkItemRecord{open}}, true},
		{"story", types.LifecycleSnapshot{Trajectory: types.TrajectoryRecord{Kind: types.TrajectoryKindStory}, WorkItems: []types.WorkItemRecord{open}}, false},
		{"publication", types.LifecycleSnapshot{Trajectory: types.TrajectoryRecord{Kind: types.TrajectoryKindPublication}, WorkItems: []types.WorkItemRecord{open}}, false},
		{"another agent's work", types.LifecycleSnapshot{WorkItems: []types.WorkItemRecord{{Status: types.WorkItemOpen, AssignedAgentID: "research:doc-1", AuthorityProfile: agentprofile.Research}}}, false},
	} {
		if _, got := openTextureRequest(tc.snap, agent); got != tc.want {
			t.Errorf("%s: open request = %v, want %v", tc.name, got, tc.want)
		}
	}
	live := types.LifecycleSnapshot{EngineeringAssignments: []types.EngineeringAssignment{{AssignmentID: "a1", Disposition: types.EngineeringAssignmentCompleted}, {AssignmentID: "a2", Disposition: types.EngineeringAssignmentBound}}}
	if got := liveEngineeringAssignment(live); got != "a2" {
		t.Fatalf("live assignment = %q, want a2", got)
	}
	live.EngineeringAssignments = live.EngineeringAssignments[:1]
	if got := liveEngineeringAssignment(live); got != "" {
		t.Fatalf("live assignment = %q, want none", got)
	}
}
