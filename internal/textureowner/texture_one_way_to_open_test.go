package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The ordinary prompt path, pinned (owner, 2026-11-16, after fighting for it
// for months): "The whole policy is fast iterative responses with early
// evidence. V0 is the raw prompt. V1 is a fast response from texture model
// weights. V2 is after the first search round research message to texture.
// The point of the prompt bar router is to decide whether to open a new
// texture or work in an existing one; not to have multiple ways to open a new
// texture. Texture should be woken by new messages."
//
// Failure list (docs/problems/texture-story-ask-path-writes-no-fast-v1-and-
// research-delivers-once-2026-11-16.md): a second way to open a Texture (the
// story ask) gave the desk a different first objective, so no v1 was written
// for 12 minutes; a record arriving while Texture closed was never integrated.
// Each test names a clause of the policy and fails if it breaks.

func TestThereIsOneWayToOpenANewTextureAndItHandsTheDeskTheOwnersWords(t *testing.T) {
	core, handler := testAPISetup(t)
	const question = "History of SoftBank"
	w := postPromptBar(t, handler, "opener-1", map[string]any{"text": question, "command_id": "open-1"}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt bar = %d %s", w.Code, w.Body)
	}
	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	snap, err := core.Store().GetLifecycleSnapshot(ctx, "opener-1", core.TextureComputerID(), resp.TrajectoryID)
	if err != nil || len(snap.WorkItems) == 0 {
		t.Fatalf("snapshot err=%v", err)
	}
	if obj := snap.WorkItems[0].Objective; obj != question {
		t.Fatalf("the desk's first objective = %q, want the owner's words alone: no kind of ask changes how a new Texture starts", obj)
	}
	rev, err := core.Store().GetLifecycleRevision(ctx, "opener-1", core.TextureComputerID(), resp.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	_ = json.Unmarshal(rev.Metadata, &meta)
	if rev.Content != question {
		t.Fatalf("v0 = %q, want the raw prompt", rev.Content)
	}
	if _, kind := meta["asked"]; kind {
		t.Fatalf("v0 records an ask kind: %s", rev.Metadata)
	}
}

// V2 follows research's first message: a record arriving wakes Texture, and
// the turn that reads it commits a revision grounded in it, which is not
// marked from-weights. The wake works on a document the first turn left with
// a Wait and a Cast (v1 committed), as in the owner's run.
func TestAResearchMessageWakesTextureAndYieldsARevision(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	ctx := context.Background()
	w := postPromptBar(t, handler, "wake-owner", map[string]any{"text": "the merger ruling", "command_id": "wake-1"}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt bar = %d %s", w.Code, w.Body)
	}
	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	computer := core.TextureComputerID()
	runs, err := s.ListLifecycleRunsByChannel(ctx, "wake-owner", computer, resp.DocID, 0)
	if err != nil || len(runs) == 0 {
		t.Fatalf("no first run: %v", err)
	}
	r0 := runs[0]
	texture := r0.AgentID
	v1 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: resp.RevisionID, Wait: "research on the ruling",
		InboxRecords: []types.CommitmentRecord{researchCast(r0, "texture-cell:cast:wake")},
		Write:        &yaegikernel.TextureWrite{Edits: "@replace b1\nFrom what I know, a court ruled on a merger; this is from memory and not yet sourced.", Why: "fast answer from weights"}})
	if _, err := handler.CommitCellTextureAuthor(ctx, &r0, v1, "wake-v1"); err != nil {
		t.Fatalf("v1: %v", err)
	}
	parkRun(t, s, "wake-owner", computer, r0.RunID, "wake-passivate")

	seedTextureReport(t, s, r0, "research-cell:report:first-round")
	woken, err := handler.ReconcileAgentWake(ctx, "wake-owner", resp.DocID)
	if err != nil || woken == nil {
		t.Fatalf("a research message did not wake Texture: run=%+v err=%v", woken, err)
	}
	doc, err := s.GetLifecycleDocument(ctx, "wake-owner", computer, resp.DocID)
	if err != nil {
		t.Fatal(err)
	}
	v2 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: doc.CurrentRevisionID, Wait: "more research",
		InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1},
		Write:       &yaegikernel.TextureWrite{Edits: "@insert end\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.", Why: "research's first message"}})
	if _, err := handler.CommitCellTextureAuthor(ctx, woken, v2, "wake-v2"); err != nil {
		t.Fatalf("v2 from the first message: %v", err)
	}
	head, err := s.GetLifecycleRevision(ctx, "wake-owner", computer, mustHead(t, s, "wake-owner", computer, resp.DocID))
	if err != nil {
		t.Fatal(err)
	}
	if head.RevisionID == doc.CurrentRevisionID || !strings.Contains(head.Content, "may proceed") {
		t.Fatalf("the message produced no revision: %q", head.Content)
	}
	if meta := decodeRevisionMetadata(head.Metadata); meta["model_prior_interim"] != nil {
		t.Fatalf("v2 is marked from-weights: %#v", meta)
	}
	if unread, err := s.TextureWorkUnread(ctx, "wake-owner", computer, texture); err != nil || unread {
		t.Fatalf("the message is still unread after the revision: unread=%v err=%v", unread, err)
	}
}

func mustHead(t *testing.T, s interface {
	GetLifecycleDocument(context.Context, string, string, string) (types.Document, error)
}, owner, computer, docID string) string {
	t.Helper()
	doc, err := s.GetLifecycleDocument(context.Background(), owner, computer, docID)
	if err != nil {
		t.Fatal(err)
	}
	return doc.CurrentRevisionID
}

// parkRun parks a Texture run as the runtime does after a committed turn
// (agentcore passivateIdleToolLoopRun, sleepTextureMutationAfterIdle): the
// run's pending mutation is completed with the committed revision, or put to
// sleep after a turn that committed none, and the run is passivated with the
// idle park fields.
func parkRun(t *testing.T, s *store.Store, owner, computer, runID, command string) {
	t.Helper()
	ctx := context.Background()
	parked, err := s.GetLifecycleRun(ctx, owner, computer, runID)
	if err != nil {
		t.Fatal(err)
	}
	mutationComputer := strings.TrimSpace(parked.ComputerID)
	mutation, err := s.GetAgentMutationByRun(ctx, owner, mutationComputer, runID)
	if err != nil {
		t.Fatalf("get the run's mutation: %v", err)
	}
	if mutation != nil && mutation.State == "pending" {
		turn, turnErr := s.GetAppliedTextureTurnByCallerRun(ctx, owner, computer, parked.TrajectoryID, runID)
		switch {
		case turnErr == nil && turn.TextureTurn != nil && turn.TextureTurn.Outcome == types.TextureTurnRevision && turn.Revision != nil:
			if err := s.CompleteAgentMutation(ctx, owner, mutationComputer, runID, turn.Revision.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
				t.Fatalf("complete the mutation: %v", err)
			}
		case turnErr == nil && turn.TextureTurn != nil:
			if err := s.SleepAgentMutationAfterTextureTurn(ctx, owner, mutationComputer, parked.TrajectoryID, runID); err != nil && err != store.ErrMutationAlreadyCompleted {
				t.Fatalf("sleep the mutation: %v", err)
			}
		}
	}
	parked.State, parked.UpdatedAt, parked.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
	parked.Metadata["passivated_reason"], parked.Metadata["actor_sleep_state"] = "durable_transition_success", "idle"
	parked.Metadata["actor_sleep_at"] = parked.UpdatedAt.Format(time.RFC3339Nano)
	parked.Metadata["actor_park_on_idle"], parked.Metadata["actor_park_idle_seconds"] = true, 120
	park := types.ReplaceLifecycleActivationRequest{OwnerID: owner, ComputerID: computer, CommandID: command, TrajectoryID: parked.TrajectoryID, AgentID: parked.AgentID, Run: parked}
	park.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(park)
	if _, err := s.ReplaceLifecycleActivation(ctx, park); err != nil {
		t.Fatalf("park the run: %v", err)
	}
}

// Owner, 2026-11-16: "Texture should be woken by new messages." A research
// message that arrives after Texture's Done wakes Texture and yields a
// revision when the trajectory is live. It is live here: the v1 turn cast
// research, so its round is open, and the store keeps a document trajectory
// live while a conversation of its Texture has an open round
// (lifecycleSettlementReady). A trajectory that has truly settled is another
// matter: see TestAMessageToASettledTextureWakesNothing.
func TestAResearchMessageAfterDoneOnALiveTrajectoryWakesTextureAndYieldsARevision(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	ctx := context.Background()
	w := postPromptBar(t, handler, "settled-owner", map[string]any{"text": "the merger ruling", "command_id": "settled-1"}, false)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt bar = %d %s", w.Code, w.Body)
	}
	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	computer := core.TextureComputerID()
	runs, err := s.ListLifecycleRunsByChannel(ctx, "settled-owner", computer, resp.DocID, 0)
	if err != nil || len(runs) == 0 {
		t.Fatalf("no first run: %v", err)
	}
	r0 := runs[0]
	texture := r0.AgentID
	v1 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: resp.RevisionID, Done: "answered from what I know",
		InboxRecords: []types.CommitmentRecord{researchCast(r0, "texture-cell:cast:settled")},
		Write:        &yaegikernel.TextureWrite{Edits: "@replace b1\nFrom what I know, a court ruled on a merger; this is from memory and not yet sourced.", Why: "fast answer from weights"}})
	if _, err := handler.CommitCellTextureAuthor(ctx, &r0, v1, "settled-v1"); err != nil {
		t.Fatalf("v1: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, "settled-owner", computer, r0.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("trajectory after the Done = %s; with its research round open it must be live", snapshot.Trajectory.Status)
	}
	parkRun(t, s, "settled-owner", computer, r0.RunID, "settled-park")
	// Research ends its run (the round's work closes and settlement runs),
	// and only then does its final message reach Texture.
	research := "research:" + resp.DocID
	now := time.Now().UTC()
	rr := types.RunRecord{
		RunID: "research-run-settled", OwnerID: "settled-owner", ComputerID: computer,
		AgentID: research, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research,
		ChannelID: resp.DocID, TrajectoryID: r0.TrajectoryID, State: types.RunRunning,
		Metadata:  map[string]any{"request_source": "inbox_record"},
		CreatedAt: now, UpdatedAt: now,
	}
	for i, state := range []types.RunState{types.RunRunning, types.RunCompleted} {
		rr.State, rr.UpdatedAt = state, time.Now().UTC()
		if state.Terminal() {
			fin := rr.UpdatedAt
			rr.FinishedAt = &fin
		}
		req := types.ReplaceLifecycleActivationRequest{OwnerID: "settled-owner", ComputerID: computer, CommandID: "settled-research-" + string(rune('a'+i)), TrajectoryID: r0.TrajectoryID, AgentID: research, Run: rr}
		req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
		if _, err := s.ReplaceLifecycleActivation(ctx, req); err != nil {
			t.Fatalf("research run %s: %v", state, err)
		}
	}
	if stored, err := s.GetLifecycleRun(ctx, "settled-owner", computer, rr.RunID); err == nil {
		_ = s.ReconcileLifecycleSettlementForTerminalRun(ctx, stored)
	}
	snapshot, _ = s.GetLifecycleSnapshot(ctx, "settled-owner", computer, r0.TrajectoryID)
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("trajectory after research's run ended = %s, want live", snapshot.Trajectory.Status)
	}
	seedTextureReport(t, s, r0, "research-cell:report:after-settlement")
	woken, err := handler.ReconcileAgentWake(ctx, "settled-owner", resp.DocID)
	if err != nil || woken == nil {
		t.Fatalf("a research message after settlement did not wake Texture: run=%+v err=%v (trajectory %s)", woken, err, snapshot.Trajectory.Status)
	}
	doc, _ := s.GetLifecycleDocument(ctx, "settled-owner", computer, resp.DocID)
	v2 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: doc.CurrentRevisionID, Wait: "more research",
		InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1},
		Write:       &yaegikernel.TextureWrite{Edits: "@insert end\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.", Why: "research's message"}})
	if _, err := handler.CommitCellTextureAuthor(ctx, woken, v2, "settled-v2"); err != nil {
		t.Fatalf("the revision from the message: %v", err)
	}
}

// seedTextureReport commits a report from the research desk to the Texture
// desk's inbox, as research's message would arrive.
func seedTextureReport(t *testing.T, s *store.Store, rec types.RunRecord, id string) {
	t.Helper()
	seedTextureReportFrom(t, s, rec, id, agentprofile.Research+":"+rec.ChannelID)
}

// seedTextureReportFrom is seedTextureReport from any sender inbox.
func seedTextureReportFrom(t *testing.T, s *store.Store, rec types.RunRecord, id, research string) {
	t.Helper()
	texture := rec.AgentID
	report := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "seed-" + id, Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "a packet from research"}}}, Inbox: texture, SenderInbox: research, Addressee: texture,
		ComputerID: rec.ComputerID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: research},
	}}}
	report.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(report)
	if _, err := s.CommitInboxRecords(context.Background(), report); err != nil {
		t.Fatalf("seed report %s: %v", id, err)
	}
}
