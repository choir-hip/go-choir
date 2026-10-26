package store

// F1 (docs/problems/settled-document-refuses-owner-revision-2026-10-26.md):
// ContinueLifecycleDocument starts the next live trajectory on a document
// whose trajectory settled. Ways the reducer could fail, written before the
// code:
//
//  S1. A live trajectory is "continued" (two live trajectories on one
//      document), or a cancelled one is (cancel is a stop).
//  S2. The settled trajectory is rewritten: its status, version or seq move.
//  S3. The document pointer, the Texture desk's binding or the research
//      desk's binding stay on the settled trajectory, so the new snapshot has
//      no desk and the old one keeps it.
//  S4. The new trajectory loses the document binding, the settlement rule or
//      the continues link, or carries per-episode subject refs forward.
//  S5. A retried command mints a second trajectory instead of replaying; a
//      different command under the same id is accepted; a second continue of
//      the same settled trajectory succeeds after the first moved the
//      document.
//  S6. A stale head or an archived document is continued.
//  S7. An archive aimed at the settled trajectory archives the document the
//      new trajectory now owns.
//  S8. The lineage read follows a continues link to another document's
//      trajectory, loops on a cycle, or ignores its bound.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func startContinueTestDocument(t *testing.T, s *Store, key string, refs map[string]string) types.StartLifecycleRequest {
	t.Helper()
	docID := "doc-" + key
	subjectRefs := map[string]string{"artifact": "texture://documents/" + docID, "episode_note": "first episode only"}
	for k, v := range refs {
		subjectRefs[k] = v
	}
	now := time.Now().UTC()
	req := types.StartLifecycleRequest{
		OwnerID: "owner-continue", ComputerID: "computer-continue", CommandID: "start-" + key,
		TrajectoryID: "trajectory-" + key, Kind: types.TrajectoryKindDocument, SubjectRefs: subjectRefs,
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-" + key, Objective: "first episode of " + key},
		InitialDocument: types.Document{DocID: docID, Title: key, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-" + key, AuthorKind: types.AuthorUser, AuthorLabel: "owner-continue", Content: "head of " + key, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: "texture:" + docID, Profile: "texture", Role: "texture", ChannelID: docID},
	}
	req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err != nil {
		t.Fatalf("start %s: %v", key, err)
	}
	return req
}

func settleContinueTestTrajectory(t *testing.T, s *Store, ownerID, computerID, docID, trajectoryID string) types.TrajectoryRecord {
	t.Helper()
	ctx := context.Background()
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, work := range snapshot.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		settle := types.SettleLifecycleWorkRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "settle-work:" + work.WorkItemID, TrajectoryID: trajectoryID,
			WorkItemID: work.WorkItemID, ActingAgentID: "texture:" + docID, ResultRef: snapshot.Document.CurrentRevisionID,
		}
		settle.CommandDigest, _ = ComputeSettleLifecycleWorkDigest(settle)
		if _, err := s.SettleLifecycleWork(ctx, settle); err != nil {
			t.Fatalf("settle work: %v", err)
		}
	}
	snapshot, err = s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	req := types.SettleLifecycleTrajectoryRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "settle:" + trajectoryID, TrajectoryID: trajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
	}
	req.CommandDigest, _ = ComputeSettleLifecycleTrajectoryDigest(req)
	result, err := s.SettleLifecycleTrajectory(ctx, req)
	if err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	return result.Trajectory
}

func continueTestRequest(start types.StartLifecycleRequest, predecessorID, nextID, head string) types.ContinueLifecycleDocumentRequest {
	req := types.ContinueLifecycleDocumentRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "continue:" + nextID,
		TrajectoryID: nextID, PredecessorTrajectoryID: predecessorID, DocID: start.InitialDocument.DocID,
		ExpectedHeadRevisionID: head,
		InitialWork:            types.WorkItemRecord{WorkItemID: "work:" + nextID, Objective: "Revise the document for the owner's new request."},
	}
	req.CommandDigest, _ = ComputeContinueLifecycleDocumentDigest(req)
	return req
}

func snapshotHasAgent(snapshot types.LifecycleSnapshot, agentID string) bool {
	for _, agent := range snapshot.Agents {
		if agent.AgentID == agentID {
			return true
		}
	}
	return false
}

// S1: only a settled trajectory continues.
func TestContinueLifecycleDocumentRefusesLiveAndCancelled(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	live := startContinueTestDocument(t, s, "live", nil)
	if _, err := s.ContinueLifecycleDocument(ctx, continueTestRequest(live, live.TrajectoryID, "next-live", live.InitialRevision.RevisionID)); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("S1: continuing a live trajectory err=%v, want invalid transition", err)
	}
	cancelled := startContinueTestDocument(t, s, "cancelled", nil)
	snapshot, err := s.GetLifecycleSnapshot(ctx, cancelled.OwnerID, cancelled.ComputerID, cancelled.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: cancelled.OwnerID, ComputerID: cancelled.ComputerID, CommandID: "cancel-continue-test", TrajectoryID: cancelled.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "stop",
	}
	cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
	if _, err := s.CancelLifecycleTrajectory(ctx, cancel); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ContinueLifecycleDocument(ctx, continueTestRequest(cancelled, cancelled.TrajectoryID, "next-cancelled", cancelled.InitialRevision.RevisionID)); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("S1: continuing a cancelled trajectory err=%v, want invalid transition", err)
	}
	for _, start := range []types.StartLifecycleRequest{live, cancelled} {
		doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
		if err != nil || doc.TrajectoryID != start.TrajectoryID {
			t.Fatalf("S1: refused continue moved %s to %q err=%v", start.InitialDocument.DocID, doc.TrajectoryID, err)
		}
	}
}

// S2-S4: the continue binds the document and its desks to the new
// trajectory and leaves the settled one as it was.
func TestContinueLifecycleDocumentStartsTheNextTrajectory(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "bound", nil)
	docID, deskID := start.InitialDocument.DocID, start.Agent.AgentID
	settled := settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)

	req := continueTestRequest(start, start.TrajectoryID, "trajectory-bound-2", start.InitialRevision.RevisionID)
	result, err := s.ContinueLifecycleDocument(ctx, req)
	if err != nil {
		t.Fatalf("continue: %v", err)
	}
	if result.Replay || result.Receipt.Kind != types.LifecycleStart || len(result.Events) != 1 || result.Events[0].Kind != types.LifecycleTrajectoryStarted {
		t.Fatalf("continue result receipt=%+v events=%+v replay=%v", result.Receipt, result.Events, result.Replay)
	}
	if refs := result.Events[0].ArtifactRefs; len(refs) != 2 || refs[0] != docID || refs[1] != start.InitialRevision.RevisionID {
		t.Fatalf("trajectory_started refs=%v, want [%s %s]", refs, docID, start.InitialRevision.RevisionID)
	}

	// S2.
	old, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if old.Status != types.TrajectorySettled || old.LifecycleVersion != settled.LifecycleVersion || old.ReducerSeq != settled.ReducerSeq {
		t.Fatalf("S2: settled trajectory changed: %+v -> %+v", settled, old)
	}

	// S4.
	next, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, req.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != types.TrajectoryLive || next.Kind != settled.Kind || next.SubjectRefs["doc_id"] != docID ||
		next.SubjectRefs["artifact"] != settled.SubjectRefs["artifact"] || next.SubjectRefs[types.TrajectoryRefContinues] != start.TrajectoryID {
		t.Fatalf("S4: next trajectory=%+v", next)
	}
	if _, carried := next.SubjectRefs["episode_note"]; carried {
		t.Fatalf("S4: a per-episode subject ref was carried: %+v", next.SubjectRefs)
	}
	if next.SettlementRule.Version != settled.SettlementRule.Version || !next.SettlementRule.RequireNoOpenWorkItems || len(next.SettlementRule.RequiredSubjectRefs) != 1 {
		t.Fatalf("S4: settlement rule=%+v", next.SettlementRule)
	}

	// S3.
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.TrajectoryID != req.TrajectoryID || doc.CurrentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("S3: document=%+v", doc)
	}
	nextSnapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, req.TrajectoryID)
	if err != nil {
		t.Fatalf("S3: next snapshot: %v", err)
	}
	if !snapshotHasAgent(nextSnapshot, deskID) || nextSnapshot.HeadRevision.RevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("S3: next snapshot agents=%+v head=%s", nextSnapshot.Agents, nextSnapshot.HeadRevision.RevisionID)
	}
	open := 0
	for _, work := range nextSnapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == deskID && work.AuthorityProfile == "texture" {
			open++
		}
	}
	if open != 1 || len(nextSnapshot.WorkItems) != 1 {
		t.Fatalf("S3: next work items=%+v, want one open Texture item", nextSnapshot.WorkItems)
	}
	oldSnapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("S2: settled snapshot no longer reads: %v", err)
	}
	if snapshotHasAgent(oldSnapshot, deskID) || oldSnapshot.Trajectory.Status != types.TrajectorySettled {
		t.Fatalf("S3: settled snapshot still holds the desk or changed status: %+v", oldSnapshot.Agents)
	}
}

// S3: a research desk bound to the settled trajectory moves with the
// document.
func TestContinueLifecycleDocumentMovesTheResearchDesk(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "research", nil)
	docID := start.InitialDocument.DocID
	researchID := "research:" + docID
	desk := types.AgentRecord{AgentID: researchID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, Profile: "research", Role: "research", ChannelID: docID, LifecycleVersion: 1, LastReducerSeq: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	meta := lifecycleMetadata("agent_id", researchID, start.ComputerID, start.TrajectoryID, 1)
	meta["channel_id"] = docID
	obj, err := lifecycleObject(ogKindAgent, start.OwnerID, start.ComputerID, researchID, desk, meta, desk.CreatedAt, desk.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PutObject(ctx, obj); err != nil {
		t.Fatal(err)
	}
	settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	req := continueTestRequest(start, start.TrajectoryID, "trajectory-research-2", start.InitialRevision.RevisionID)
	if _, err := s.ContinueLifecycleDocument(ctx, req); err != nil {
		t.Fatalf("continue: %v", err)
	}
	next, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, req.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshotHasAgent(next, researchID) {
		t.Fatalf("S3: research desk stayed on the settled trajectory: %+v", next.Agents)
	}
}

// S5: replay, conflict, and a second continue of the same settled trajectory.
func TestContinueLifecycleDocumentReplaysAndRefusesASecond(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "replay", nil)
	settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, start.InitialDocument.DocID, start.TrajectoryID)
	req := continueTestRequest(start, start.TrajectoryID, "trajectory-replay-2", start.InitialRevision.RevisionID)
	first, err := s.ContinueLifecycleDocument(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.ContinueLifecycleDocument(ctx, req)
	if err != nil || !again.Replay || again.Trajectory.TrajectoryID != first.Trajectory.TrajectoryID {
		t.Fatalf("S5: retry result=%+v err=%v, want a replay of %s", again.Trajectory, err, first.Trajectory.TrajectoryID)
	}
	conflict := req
	conflict.InitialWork.Objective = "a different intent under the same command"
	conflict.CommandDigest, _ = ComputeContinueLifecycleDocumentDigest(conflict)
	if _, err := s.ContinueLifecycleDocument(ctx, conflict); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("S5: same command, different digest err=%v, want command conflict", err)
	}
	second := continueTestRequest(start, start.TrajectoryID, "trajectory-replay-3", start.InitialRevision.RevisionID)
	if _, err := s.ContinueLifecycleDocument(ctx, second); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("S5: a second continue of the same settled trajectory err=%v, want concurrent state change", err)
	}
	if _, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, second.TrajectoryID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("S5: the refused second continue left trajectory %s (err=%v)", second.TrajectoryID, err)
	}
}

// S6: a stale head or an archived document is not continued.
func TestContinueLifecycleDocumentRefusesStaleHeadAndArchived(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "stale", nil)
	docID := start.InitialDocument.DocID
	settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	if _, err := s.ContinueLifecycleDocument(ctx, continueTestRequest(start, start.TrajectoryID, "trajectory-stale-2", "not-the-head")); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("S6: stale head err=%v, want concurrent state change", err)
	}
	if _, err := s.ArchiveTextureDocumentAuthority(ctx, docID, start.OwnerID, start.ComputerID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := s.ContinueLifecycleDocument(ctx, continueTestRequest(start, start.TrajectoryID, "trajectory-stale-3", start.InitialRevision.RevisionID)); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("S6: archived document err=%v, want invalid transition", err)
	}
}

// S7: an archive aimed at the settled trajectory does not archive the
// document its successor owns.
func TestArchiveAimedAtASupersededTrajectoryIsRefused(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "archive", nil)
	docID := start.InitialDocument.DocID
	settled := settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	req := continueTestRequest(start, start.TrajectoryID, "trajectory-archive-2", start.InitialRevision.RevisionID)
	if _, err := s.ContinueLifecycleDocument(ctx, req); err != nil {
		t.Fatal(err)
	}
	archive := types.ArchiveLifecycleArtifactRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "archive-superseded", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: settled.LifecycleVersion, ExpectedHeadRevisionID: start.InitialRevision.RevisionID, Reason: "stale archive",
	}
	archive.CommandDigest, _ = ComputeArchiveLifecycleArtifactDigest(archive)
	if _, err := s.ArchiveLifecycleArtifact(ctx, archive); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("S7: archive via the superseded trajectory err=%v, want concurrent state change", err)
	}
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil || doc.ArchivedAt != nil || doc.TrajectoryID != req.TrajectoryID {
		t.Fatalf("S7: document after the stale archive=%+v err=%v", doc, err)
	}
}

// S8: the lineage follows continues links of one document only, newest
// first, within its bound and without looping.
func TestLifecycleTrajectoryLineage(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := startContinueTestDocument(t, s, "lineage", nil)
	docID := start.InitialDocument.DocID
	settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	second := continueTestRequest(start, start.TrajectoryID, "trajectory-lineage-2", start.InitialRevision.RevisionID)
	if _, err := s.ContinueLifecycleDocument(ctx, second); err != nil {
		t.Fatal(err)
	}
	settleContinueTestTrajectory(t, s, start.OwnerID, start.ComputerID, docID, second.TrajectoryID)
	third := continueTestRequest(start, second.TrajectoryID, "trajectory-lineage-3", start.InitialRevision.RevisionID)
	if _, err := s.ContinueLifecycleDocument(ctx, third); err != nil {
		t.Fatal(err)
	}
	got, err := s.LifecycleTrajectoryLineage(ctx, start.OwnerID, start.ComputerID, third.TrajectoryID, 8)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{third.TrajectoryID, second.TrajectoryID, start.TrajectoryID}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("S8: lineage=%v, want %v", got, want)
	}
	if got, err := s.LifecycleTrajectoryLineage(ctx, start.OwnerID, start.ComputerID, third.TrajectoryID, 2); err != nil || len(got) != 2 {
		t.Fatalf("S8: bounded lineage=%v err=%v, want 2", got, err)
	}

	// A hand-written link to another document's trajectory is not followed.
	foreign := startContinueTestDocument(t, s, "foreign", map[string]string{types.TrajectoryRefContinues: start.TrajectoryID})
	if got, err := s.LifecycleTrajectoryLineage(ctx, start.OwnerID, start.ComputerID, foreign.TrajectoryID, 8); err != nil || len(got) != 1 || got[0] != foreign.TrajectoryID {
		t.Fatalf("S8: lineage followed another document's trajectory: %v err=%v", got, err)
	}

	// A cycle ends the read.
	cycleA := startContinueTestDocument(t, s, "cycle", map[string]string{types.TrajectoryRefContinues: "trajectory-cycle"})
	if got, err := s.LifecycleTrajectoryLineage(ctx, start.OwnerID, start.ComputerID, cycleA.TrajectoryID, 8); err != nil || len(got) != 1 {
		t.Fatalf("S8: self-cycle lineage=%v err=%v, want only itself", got, err)
	}
}
