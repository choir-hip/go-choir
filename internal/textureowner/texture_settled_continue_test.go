package textureowner

// F1 (docs/problems/settled-document-refuses-owner-revision-2026-10-26.md):
// a trajectory is one episode of work and a document outlives its episodes.
// When a document's trajectory has settled, the owner's next revise starts a
// new live trajectory bound to the same document, carrying the head forward;
// the settled trajectory stays settled. Ways it could fail, written before
// the code; each has a test below or in internal/store and internal/agentcore:
//
//  C1. The revise is refused with 409 "durable lifecycle state is
//      unavailable or terminal" (the bug).
//  C2. The settled trajectory is reopened to live or otherwise rewritten
//      (settled is terminal in the store and in the restart rules).
//  C3. The new trajectory is not bound to the document: document.TrajectoryID
//      does not move, or the new trajectory lacks subject_refs doc_id or the
//      continues_trajectory_id link to the settled one.
//  C4. The head is not carried: the owner revision's parent is not the
//      settled head, its content differs, or owner_prompt is missing.
//  C5. The Texture desk is not reached: no open Texture work item on the new
//      trajectory, no owner-revision wake for the new trajectory, or the desk
//      agent stays bound to the settled trajectory.
//  C6. A retry with the same client_request_id starts a second trajectory or
//      commits a second owner revision.
//  C7. Two revises racing on one settled document start two trajectories.
//  C8. A cancelled trajectory is continued: cancel is a stop, so it stays a
//      409 and nothing moves.
//  C9. A stale expected head starts a trajectory before the head check
//      refuses the revise.
//  C10. The settled trajectory's own snapshot stops reading (its history
//      must stay readable after the document moves on).
//  C11. A continued trajectory that settles in turn cannot be continued (the
//      chain must extend, one link per episode).
//  C13. The actor's wake reconcile on the continued document cannot start
//      the Texture run (staging 2026-10-26: "replace durable activation:
//      lifecycle invalid transition", every second, no run ever).
//  C12. An engineering round's carried work on the settled trajectory is not
//      offered to the next trajectory's engineer (internal/agentcore).

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const continuesTrajectoryRef = "continues_trajectory_id"

// settleDocumentTrajectory completes the open Texture work on trajectoryID
// and settles the trajectory on the document's current head, the way a
// finished episode ends.
func settleDocumentTrajectory(t *testing.T, s *store.Store, ownerID, computerID, docID, trajectoryID string) {
	t.Helper()
	ctx := context.Background()
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatalf("snapshot before settle: %v", err)
	}
	agentID := "texture:" + docID
	for _, work := range snapshot.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		settle := types.SettleLifecycleWorkRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "settle-work:" + work.WorkItemID,
			TrajectoryID: trajectoryID, WorkItemID: work.WorkItemID, ActingAgentID: agentID,
			ResultRef: snapshot.Document.CurrentRevisionID,
		}
		settle.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settle)
		if _, err := s.SettleLifecycleWork(ctx, settle); err != nil {
			t.Fatalf("settle work %s: %v", work.WorkItemID, err)
		}
	}
	snapshot, err = s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatalf("snapshot after work settle: %v", err)
	}
	req := types.SettleLifecycleTrajectoryRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "settle-trajectory:" + trajectoryID,
		TrajectoryID: trajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.Document.CurrentRevisionID,
	}
	req.CommandDigest, _ = store.ComputeSettleLifecycleTrajectoryDigest(req)
	if _, err := s.SettleLifecycleTrajectory(ctx, req); err != nil {
		t.Fatalf("settle trajectory %s: %v", trajectoryID, err)
	}
	if got, err := s.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID); err != nil || got.Status != types.TrajectorySettled {
		t.Fatalf("trajectory %s status=%s err=%v, want settled", trajectoryID, got.Status, err)
	}
}

// documentTrajectories lists every trajectory bound to docID.
func documentTrajectories(t *testing.T, s *store.Store, ownerID, computerID, docID string) []types.TrajectoryRecord {
	t.Helper()
	all, err := s.ListLifecycleTrajectoriesByOwner(context.Background(), ownerID, computerID, 1000)
	if err != nil {
		t.Fatalf("list trajectories: %v", err)
	}
	var out []types.TrajectoryRecord
	for _, trajectory := range all {
		if trajectory.SubjectRefs["doc_id"] == docID {
			out = append(out, trajectory)
		}
	}
	return out
}

type continueDispatch struct {
	toAgentID, kind, content, trajectoryID string
}

func recordContinueDispatches(core *agentcore.Runtime) (*sync.Mutex, *[]continueDispatch) {
	var mu sync.Mutex
	var dispatches []continueDispatch
	core.SetDispatchActor(func(_ context.Context, _, _, toAgentID, kind, content, trajectoryID, _ string) error {
		mu.Lock()
		defer mu.Unlock()
		dispatches = append(dispatches, continueDispatch{toAgentID: toAgentID, kind: kind, content: content, trajectoryID: trajectoryID})
		return nil
	})
	return &mu, &dispatches
}

// C1-C5, C10: the revise on a settled document is accepted as a new
// trajectory carrying the head, and the settled one is left as it was.
func TestOwnerReviseOnSettledDocumentStartsNextTrajectory(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	ctx := t.Context()
	start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
		// A produced head names its trajectory in metadata, as the desk's revisions do.
		req.InitialRevision.Metadata = json.RawMessage(`{"trajectory_id":"` + req.TrajectoryID + `"}`)
	})
	docID := start.InitialDocument.DocID
	mu, dispatches := recordContinueDispatches(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	settled, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	settledHead, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}

	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+docID+"/revise", start.OwnerID, "after-settled", "build on this: add a section on costs", settledHead.CurrentRevisionID)
	if response.Code != http.StatusAccepted {
		t.Fatalf("C1: revise on a settled document status=%d body=%s", response.Code, response.Body.String())
	}
	var receipt textureOwnerRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil || receipt.RevisionID == "" || receipt.Replay {
		t.Fatalf("receipt=%+v err=%v", receipt, err)
	}

	// C2: the settled trajectory is untouched.
	after, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != types.TrajectorySettled || after.LifecycleVersion != settled.LifecycleVersion || after.ReducerSeq != settled.ReducerSeq {
		t.Fatalf("C2: settled trajectory changed: before=%+v after=%+v", settled, after)
	}

	// C3: the document moved to one new live trajectory linked to the old.
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.TrajectoryID == "" || doc.TrajectoryID == start.TrajectoryID {
		t.Fatalf("C3: document trajectory did not move: %q", doc.TrajectoryID)
	}
	next, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, doc.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if next.Status != types.TrajectoryLive || next.SubjectRefs["doc_id"] != docID || next.SubjectRefs[continuesTrajectoryRef] != start.TrajectoryID {
		t.Fatalf("C3: next trajectory=%+v", next)
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 2 {
		t.Fatalf("C3: document has %d trajectories, want 2: %+v", len(got), got)
	}

	// C4: the owner revision carries the settled head forward.
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, next.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot of the next trajectory: %v", err)
	}
	head := snapshot.HeadRevision
	if head.RevisionID != receipt.RevisionID || head.ParentRevisionID != settledHead.CurrentRevisionID || head.AuthorKind != types.AuthorUser || head.TrajectoryID != next.TrajectoryID {
		t.Fatalf("C4: head=%+v, want owner revision %s on %s with parent %s", head, receipt.RevisionID, next.TrajectoryID, settledHead.CurrentRevisionID)
	}
	parent, err := s.GetLifecycleRevision(ctx, start.OwnerID, start.ComputerID, settledHead.CurrentRevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if head.Content != parent.Content {
		t.Fatalf("C4: owner revision content %q differs from the carried head %q", head.Content, parent.Content)
	}
	// C4b: the metadata label names the live trajectory the record is on, not
	// the settled one the carried-forward head metadata still names.
	var headMeta map[string]any
	if err := json.Unmarshal(head.Metadata, &headMeta); err != nil {
		t.Fatalf("C4b: decode owner revision metadata %s: %v", head.Metadata, err)
	}
	if got, _ := headMeta["trajectory_id"].(string); got != doc.TrajectoryID {
		t.Fatalf("C4b: owner revision metadata trajectory_id=%q, want live trajectory %q (settled %q)", got, doc.TrajectoryID, start.TrajectoryID)
	}
	var metadata map[string]any
	if err := json.Unmarshal(head.Metadata, &metadata); err != nil || metadata["owner_prompt"] != "build on this: add a section on costs" || metadata["input_origin"] != textureInputOriginUserPrompt {
		t.Fatalf("C4: owner revision metadata=%s err=%v", head.Metadata, err)
	}

	// C5: Texture holds one open work item and the desk is bound here.
	open := 0
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == start.Agent.AgentID {
			open++
		}
	}
	if open != 1 {
		t.Fatalf("C5: next trajectory open Texture work=%d, want 1: %+v", open, snapshot.WorkItems)
	}
	deskBound := false
	for _, agent := range snapshot.Agents {
		if agent.AgentID == start.Agent.AgentID {
			deskBound = true
		}
	}
	if !deskBound {
		t.Fatalf("C5: desk %s is not in the next trajectory's snapshot: %+v", start.Agent.AgentID, snapshot.Agents)
	}
	if pending, _, ok := store.PendingTextureOwnerRevision(snapshot); !ok || pending.RevisionID != receipt.RevisionID {
		t.Fatalf("C5: pending owner revision=%+v ok=%v", pending, ok)
	}
	mu.Lock()
	woke := false
	for _, d := range *dispatches {
		if d.kind != "coagent_result" {
			continue
		}
		occurrence, err := agentcore.DecodeTextureActorOccurrence(d.content)
		if err == nil && occurrence.Kind == agentcore.TextureActorOccurrenceDocumentRevision && occurrence.HeadRevisionID == receipt.RevisionID {
			if d.trajectoryID != next.TrajectoryID || d.toAgentID != start.Agent.AgentID {
				t.Errorf("C5: owner-revision wake to %s on %s, want %s on %s", d.toAgentID, d.trajectoryID, start.Agent.AgentID, next.TrajectoryID)
			}
			woke = true
		}
	}
	mu.Unlock()
	if !woke {
		t.Fatalf("C5: no owner-revision wake: %+v", *dispatches)
	}

	// C10: the settled trajectory still reads, on its own terminal head.
	old, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("C10: settled snapshot no longer reads: %v", err)
	}
	if old.Trajectory.Status != types.TrajectorySettled || old.HeadRevision.RevisionID != settledHead.CurrentRevisionID {
		t.Fatalf("C10: settled snapshot status=%s head=%s, want settled on %s", old.Trajectory.Status, old.HeadRevision.RevisionID, settledHead.CurrentRevisionID)
	}
}

// C6: the same client request twice starts one trajectory and commits one
// owner revision.
func TestOwnerReviseOnSettledDocumentRetryStartsOneTrajectory(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	ctx := t.Context()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	recordContinueDispatches(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	path := "/api/texture/documents/" + docID + "/revise"
	head := start.InitialRevision.RevisionID

	first := postOwnerInstruction(t, handler, path, start.OwnerID, "settled-retry", "next step", head)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first revise status=%d body=%s", first.Code, first.Body.String())
	}
	afterFirst, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	_ = postOwnerInstruction(t, handler, path, start.OwnerID, "settled-retry", "next step", head)
	afterRetry, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if afterRetry.TrajectoryID != afterFirst.TrajectoryID || afterRetry.CurrentRevisionID != afterFirst.CurrentRevisionID {
		t.Fatalf("C6: the retry moved the document: first=(%s,%s) retry=(%s,%s)", afterFirst.TrajectoryID, afterFirst.CurrentRevisionID, afterRetry.TrajectoryID, afterRetry.CurrentRevisionID)
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 2 {
		t.Fatalf("C6: document has %d trajectories after a retry, want 2", len(got))
	}
}

// C7: two different revises racing on one settled document start one
// trajectory between them.
func TestOwnerReviseRaceOnSettledDocumentStartsOneTrajectory(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	recordContinueDispatches(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	path := "/api/texture/documents/" + docID + "/revise"
	head := start.InitialRevision.RevisionID

	codes := make([]int, 2)
	var wg sync.WaitGroup
	release := make(chan struct{})
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-release
			codes[i] = postOwnerInstruction(t, handler, path, start.OwnerID, []string{"race-a", "race-b"}[i], []string{"first ask", "second ask"}[i], head).Code
		}(i)
	}
	close(release)
	wg.Wait()

	accepted := 0
	for _, code := range codes {
		switch code {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
		default:
			t.Fatalf("C7: racing revise status %d", code)
		}
	}
	if accepted < 1 {
		t.Fatalf("C7: neither racing revise was accepted: %v", codes)
	}
	trajectories := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID)
	if len(trajectories) != 2 {
		t.Fatalf("C7: racing revises left %d trajectories, want 2: %+v", len(trajectories), trajectories)
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.GetLifecycleTrajectory(t.Context(), start.OwnerID, start.ComputerID, doc.TrajectoryID)
	if err != nil || next.Status != types.TrajectoryLive || next.SubjectRefs[continuesTrajectoryRef] != start.TrajectoryID {
		t.Fatalf("C7: document bound to %+v err=%v", next, err)
	}
}

// C8: a cancelled trajectory is not continued.
func TestOwnerReviseOnCancelledDocumentStaysRefused(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	ctx := t.Context()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	mu, dispatches := recordContinueDispatches(core)
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "cancel-before-continue",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "owner stopped the work",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := s.CancelLifecycleTrajectory(ctx, cancel); err != nil {
		t.Fatal(err)
	}
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+docID+"/revise", start.OwnerID, "after-cancel", "continue anyway", snapshot.HeadRevision.RevisionID)
	if response.Code != http.StatusConflict {
		t.Fatalf("C8: revise on a cancelled document status=%d body=%s", response.Code, response.Body.String())
	}
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.TrajectoryID != start.TrajectoryID {
		t.Fatalf("C8: cancelled document moved to %s", doc.TrajectoryID)
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 1 {
		t.Fatalf("C8: cancelled document has %d trajectories", len(got))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(*dispatches) != 0 {
		t.Fatalf("C8: cancelled document dispatched %+v", *dispatches)
	}
}

// C9: a revise against a stale head starts nothing.
func TestOwnerReviseOnSettledDocumentWithStaleHeadStartsNothing(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	recordContinueDispatches(core)
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+docID+"/revise", start.OwnerID, "stale-head", "next step", "revision-that-is-not-the-head")
	if response.Code != http.StatusConflict {
		t.Fatalf("C9: stale-head revise status=%d body=%s", response.Code, response.Body.String())
	}
	doc, err := s.GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.TrajectoryID != start.TrajectoryID {
		t.Fatalf("C9: a stale-head revise moved the document to %s", doc.TrajectoryID)
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 1 {
		t.Fatalf("C9: a stale-head revise left %d trajectories", len(got))
	}
}

// C11: the chain extends: a continued trajectory that settles is continued
// by the next revise, linked to it and not to the first.
func TestOwnerReviseContinuesAContinuedDocumentAgain(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	ctx := t.Context()
	start := startObservationLifecycle(t, s)
	docID := start.InitialDocument.DocID
	recordContinueDispatches(core)
	path := "/api/texture/documents/" + docID + "/revise"

	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
	if r := postOwnerInstruction(t, handler, path, start.OwnerID, "chain-1", "second episode", start.InitialRevision.RevisionID); r.Code != http.StatusAccepted {
		t.Fatalf("first continue status=%d body=%s", r.Code, r.Body.String())
	}
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	second := doc.TrajectoryID
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, second)
	if r := postOwnerInstruction(t, handler, path, start.OwnerID, "chain-2", "third episode", doc.CurrentRevisionID); r.Code != http.StatusAccepted {
		t.Fatalf("C11: second continue status=%d body=%s", r.Code, r.Body.String())
	}
	doc, err = s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
	if err != nil {
		t.Fatal(err)
	}
	third, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, doc.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if third.TrajectoryID == second || third.TrajectoryID == start.TrajectoryID || third.Status != types.TrajectoryLive || third.SubjectRefs[continuesTrajectoryRef] != second {
		t.Fatalf("C11: third trajectory=%+v, want live and continuing %s", third, second)
	}
	if got := documentTrajectories(t, s, start.OwnerID, start.ComputerID, docID); len(got) != 3 {
		t.Fatalf("C11: document has %d trajectories, want 3", len(got))
	}
}

// priorDeskRun gives the settled episode a desk run that ran and finished,
// the shape of a document whose last episode really worked: the desk's
// activation record has been through a run on the settled trajectory.
func priorDeskRun(t *testing.T, s *store.Store, start types.StartLifecycleRequest) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "run-prior-episode", AgentID: start.Agent.AgentID, ChannelID: start.InitialDocument.DocID,
		TrajectoryID: start.TrajectoryID, AgentProfile: start.Agent.Profile, AgentRole: start.Agent.Role,
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, State: types.RunRunning,
		Prompt: "first episode", CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID, "work_item_ids": []string{start.InitialWork.WorkItemID}, "doc_id": start.InitialDocument.DocID},
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "project-prior-episode",
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: run,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project prior run: %v", err)
	}
	term := types.TerminalizeRunRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "terminalize-prior-episode",
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, RunID: run.RunID,
		TerminalState: types.RunCompleted, Reason: "done",
	}
	term.CommandDigest, _ = store.ComputeTerminalizeRunDigest(term)
	if _, err := s.TerminalizeRun(ctx, term); err != nil {
		t.Fatalf("terminalize prior run: %v", err)
	}
}

// C13: the owner's revise on a settled document is acted on: the wake it
// sends resolves and the actor's reconcile starts the Texture run on the
// continued trajectory. A document made by the conductor stores its first
// trajectory id in every revision's durable metadata (texture_handoff.go),
// and a run copies durable metadata over its own, so the run must still bind
// to the document's current trajectory, not the one the head names.
func TestOwnerReviseOnSettledDocumentStartsTheTextureRun(t *testing.T) {
	cases := []struct {
		name          string
		priorRun      bool
		headNamesTraj bool
	}{
		{"fresh desk", false, false},
		{"desk that ran in the settled episode", true, false},
		{"head names the settled trajectory", false, true},
		{"conductor document that ran", true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			core, handler := testAPISetup(t)
			s := core.Store()
			ctx := t.Context()
			start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
				if c.headNamesTraj {
					req.InitialRevision.Metadata = json.RawMessage(`{"trajectory_id":"` + req.TrajectoryID + `","revision_role":"input","input_origin":"user_prompt"}`)
				}
			})
			docID := start.InitialDocument.DocID
			mu, dispatches := recordContinueDispatches(core)
			if c.priorRun {
				priorDeskRun(t, s, start)
			}
			settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, docID, start.TrajectoryID)
			head := start.InitialRevision.RevisionID
			response := postOwnerInstruction(t, handler, "/api/texture/documents/"+docID+"/revise", start.OwnerID, "after-settled", "build on this", head)
			if response.Code != http.StatusAccepted {
				t.Fatalf("revise status=%d body=%s", response.Code, response.Body.String())
			}
			doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, docID)
			if err != nil || doc.TrajectoryID == start.TrajectoryID {
				t.Fatalf("document trajectory=%q err=%v", doc.TrajectoryID, err)
			}
			mu.Lock()
			var wake string
			for _, d := range *dispatches {
				if d.kind == "coagent_result" && d.toAgentID == start.Agent.AgentID {
					wake = d.content
				}
			}
			mu.Unlock()
			if wake == "" {
				t.Fatalf("no wake to %s: %+v", start.Agent.AgentID, *dispatches)
			}
			occurrence, state, err := handler.ResolveTextureActorOccurrence(ctx, start.OwnerID, start.ComputerID, start.Agent.AgentID, wake)
			if err != nil || state != TextureActorOccurrencePending {
				t.Fatalf("C13: resolve occurrence state=%q err=%v", state, err)
			}
			run, err := handler.ReconcileActorOccurrenceWake(ctx, start.OwnerID, start.ComputerID, start.Agent.AgentID, occurrence.ResolvedTargetWorkItemID, occurrence)
			if err != nil || run == nil {
				t.Fatalf("C13: reconcile run=%+v err=%v; want a Texture run on the continued trajectory", run, err)
			}
			if run.TrajectoryID != doc.TrajectoryID || metadataStringValue(run.Metadata, "lifecycle_work_item_id") != occurrence.ResolvedTargetWorkItemID {
				t.Fatalf("C13: run trajectory=%s work=%v, want %s on %s", run.TrajectoryID, run.Metadata["lifecycle_work_item_id"], occurrence.ResolvedTargetWorkItemID, doc.TrajectoryID)
			}
		})
	}
}

// The label follows the record on every owner-visible commit path: a head that
// already names another trajectory (a settled one, from before the label was
// restamped) must not pass that name on through the public revisions route or
// the evidence embed.
const staleTrajectoryLabel = `{"trajectory_id":"trajectory-stale"}`

func TestRevisionsRouteLabelsTheRecordsTrajectory(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
		req.InitialRevision.Metadata = json.RawMessage(staleTrajectoryLabel)
	})
	before, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	request := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "direct owner correction", ParentRevisionID: start.InitialRevision.RevisionID,
		IdempotencyKey: "label-owner-edit", ExpectedLifecycleVersion: before.Trajectory.LifecycleVersion,
	})
	response := httptest.NewRecorder()
	handler.HandleTextureRevisions(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("direct edit status=%d body=%s", response.Code, response.Body.String())
	}
	after, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(after.HeadRevision.Metadata, &meta); err != nil {
		t.Fatalf("decode metadata %s: %v", after.HeadRevision.Metadata, err)
	}
	if got, _ := meta["trajectory_id"].(string); got != start.TrajectoryID || after.HeadRevision.TrajectoryID != start.TrajectoryID {
		t.Fatalf("revisions route: metadata trajectory_id=%q record=%q, want both %q", got, after.HeadRevision.TrajectoryID, start.TrajectoryID)
	}
}

func TestEvidenceEmbedLabelsTheRecordsTrajectory(t *testing.T) {
	h, core, rec, _ := evidenceSetup(t, "label", func(r *types.Revision) {
		r.Metadata = json.RawMessage(staleTrajectoryLabel)
	})
	if _, err := h.EmbedCandidateEvidence(t.Context(), rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatalf("embed: %v", err)
	}
	head := headOf(t, core.Store(), rec)
	var meta map[string]any
	if err := json.Unmarshal(head.Metadata, &meta); err != nil {
		t.Fatalf("decode metadata %s: %v", head.Metadata, err)
	}
	if got, _ := meta["trajectory_id"].(string); got != rec.TrajectoryID || head.TrajectoryID != rec.TrajectoryID {
		t.Fatalf("evidence embed: metadata trajectory_id=%q record=%q, want both %q", got, head.TrajectoryID, rec.TrajectoryID)
	}
}

// A commit on a settled trajectory is Unbound: the store erases the record's
// trajectory field, so the label names the trajectory the commit ran under.
func TestRevisionsRouteOnSettledDocumentLabelsTheTrajectoryItRanUnder(t *testing.T) {
	core, handler := testAPISetup(t)
	s := core.Store()
	start := startObservationLifecycle(t, s, func(req *types.StartLifecycleRequest) {
		req.InitialRevision.Metadata = json.RawMessage(staleTrajectoryLabel)
	})
	settleDocumentTrajectory(t, s, start.OwnerID, start.ComputerID, start.InitialDocument.DocID, start.TrajectoryID)
	settled, err := s.GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	request := textureRequest(t, http.MethodPost, "/api/texture/documents/"+start.InitialDocument.DocID+"/revisions", textureCreateRevisionRequest{
		Content: "owner edit after settling", ParentRevisionID: start.InitialRevision.RevisionID,
		IdempotencyKey: "label-settled-edit", ExpectedLifecycleVersion: settled.Trajectory.LifecycleVersion,
	})
	response := httptest.NewRecorder()
	handler.HandleTextureRevisions(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("settled edit status=%d body=%s", response.Code, response.Body.String())
	}
	var created textureRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetLifecycleRevision(t.Context(), start.OwnerID, start.ComputerID, created.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(stored.Metadata, &meta); err != nil {
		t.Fatalf("decode metadata %s: %v", stored.Metadata, err)
	}
	if got, _ := meta["trajectory_id"].(string); got != start.TrajectoryID {
		t.Fatalf("settled commit: metadata trajectory_id=%q, want the trajectory it ran under %q (record field %q)", got, start.TrajectoryID, stored.TrajectoryID)
	}
}

// A record with no trajectory carries no trajectory label, not a client's or a
// parent's.
func TestRevisionsRouteOnUnboundDocumentCarriesNoTrajectoryLabel(t *testing.T) {
	_, handler := testAPISetup(t)
	createW := httptest.NewRecorder()
	handler.HandleTextureCreateDocument(createW, textureRequest(t, http.MethodPost, "/api/texture/documents", textureCreateDocRequest{Title: "Unbound label"}))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create doc status=%d body=%s", createW.Code, createW.Body.String())
	}
	var created textureCreateDocResponse
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	post := func(content, metadata, parent string) textureRevisionResponse {
		t.Helper()
		w := httptest.NewRecorder()
		handler.HandleTextureRevisions(w, textureRequest(t, http.MethodPost, "/api/texture/documents/"+created.DocID+"/revisions", textureCreateRevisionRequest{
			Content: content, Metadata: json.RawMessage(metadata), ParentRevisionID: parent,
		}))
		if w.Code != http.StatusCreated {
			t.Fatalf("revision status=%d body=%s", w.Code, w.Body.String())
		}
		var rev textureRevisionResponse
		if err := json.NewDecoder(w.Body).Decode(&rev); err != nil {
			t.Fatal(err)
		}
		return rev
	}
	first := post("first", `{"trajectory_id":"client-claim"}`, "")
	second := post("second", `{}`, first.RevisionID)
	for _, rev := range []textureRevisionResponse{first, second} {
		var meta map[string]any
		if err := json.Unmarshal(rev.Metadata, &meta); err != nil {
			t.Fatalf("decode metadata %s: %v", rev.Metadata, err)
		}
		if got, ok := meta["trajectory_id"]; ok {
			t.Fatalf("unbound revision %s carries trajectory_id=%v in %s", rev.RevisionID, got, rev.Metadata)
		}
	}
}
