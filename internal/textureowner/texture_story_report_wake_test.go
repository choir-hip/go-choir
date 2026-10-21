package textureowner

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// passivateIdle leaves a Texture run the way the runtime does after a
// non-revision turn: the mutation sleeps and the run is passivated (idle).
func passivateIdle(t *testing.T, s *store.Store, run *types.RunRecord, command string) {
	t.Helper()
	passivateWithReason(t, s, run, command, "idle")
}

// passivateWithReason is passivateIdle with the passivation reason the run
// row carries. Staging's owner-revision run was left with
// "durable_transition_success" (toolloop.go:838), actor_sleep_state "idle"
// and the park-on-idle fields.
func passivateWithReason(t *testing.T, s *store.Store, run *types.RunRecord, command, reason string) {
	t.Helper()
	if err := s.SleepAgentMutationAfterTextureTurn(t.Context(), run.OwnerID, run.ComputerID, run.TrajectoryID, run.RunID); err != nil {
		t.Fatalf("sleep mutation: %v", err)
	}
	rec, err := s.GetLifecycleRun(t.Context(), run.OwnerID, run.ComputerID, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	rec.State, rec.UpdatedAt, rec.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
	rec.Metadata["passivated_reason"] = reason
	rec.Metadata["actor_sleep_state"] = "idle"
	rec.Metadata["actor_park_on_idle"] = true
	rec.Metadata["actor_park_idle_seconds"] = 120
	req := types.ReplaceLifecycleActivationRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: command, TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: rec}
	req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
	if _, err := s.ReplaceLifecycleActivation(t.Context(), req); err != nil {
		t.Fatalf("passivate: %v", err)
	}
}

// Staging 2026-10-21 (paper computer, story 3c566d4a): owner reopen, owner
// revise, a Texture turn that waits and casts to research, research's report
// back to the story's Texture, and then no Texture turn ever. Ways it can
// fail, written before the test:
//   - the report's wake finds the passivated run carrying the reopened work
//     id and refuses or treats it as quiescent (reactivation skipped, an
//     authority check, "does not authorize active candidate"), so no run is
//     reactivated;
//   - the owner revision still counts as pending after the Wait turn, so the
//     wake takes the owner-revision shape instead of the records shape;
//   - research's completion carries lifecycle_terminal_settlement_requested
//     (set by the store whenever a run that owned its agent's activation ends
//     terminal), and the settlement it triggers closes the reopened Texture
//     work item or settles the story, so the wake finds nothing armed or an
//     ineligible trajectory and returns nil silently.
//
// Result (2026-10-21): none of these happens at this layer, with either
// passivation reason. The staging stall is not in the controller; these
// tests pin that layer (problem doc P6).

type storyWaited struct {
	core            *agentcore.Runtime
	handler         *Handler
	owner           string
	doc             types.Document
	r1              *types.RunRecord
	texture, resear string
	workID          string
}

func storyWaitingOnResearch(t *testing.T, reason string) storyWaited {
	t.Helper()
	core, handler := testAPISetup(t)
	var occurrenceContent string
	core.SetDispatchActor(func(_ context.Context, _, _, _, kind, content, _, _ string) error {
		if kind == "coagent_result" {
			occurrenceContent = content
		}
		return nil
	})
	s := core.Store()
	ctx := t.Context()
	owner, doc := openOwnerStory(t, handler, "report-"+reason)
	texture := agentprofile.Texture + ":" + doc.DocID
	research := agentprofile.Research + ":" + doc.DocID

	// Yesterday: Texture read the opening Cast and waited.
	r0, err := handler.ReconcileAgentWake(ctx, owner, doc.DocID)
	if err != nil || r0 == nil {
		t.Fatalf("first run = %+v, %v", r0, err)
	}
	wait0 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: doc.CurrentRevisionID, Wait: "read the opening",
		InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1}})
	if _, err := handler.CommitCellTextureAuthor(ctx, r0, wait0, "wait-0"); err != nil {
		t.Fatalf("first turn: %v", err)
	}
	passivateIdle(t, s, r0, "passivate-r0")

	// Today: Research more.
	if rsp := postOwnerInstruction(t, handler, "/api/texture/documents/"+doc.DocID+"/revise", owner, "expand-1", "research more", doc.CurrentRevisionID); rsp.Code != http.StatusAccepted {
		t.Fatalf("revise status=%d body=%s", rsp.Code, rsp.Body.String())
	}
	// The owner revision reaches the desk as an occurrence, the way the
	// actor handler resolves and reconciles it (actorruntime/handler.go).
	occurrence, state, err := handler.ResolveTextureActorOccurrence(ctx, owner, doc.ComputerID, texture, occurrenceContent)
	if err != nil || state != TextureActorOccurrencePending {
		t.Fatalf("resolve occurrence state=%q err=%v", state, err)
	}
	r1, err := handler.ReconcileActorOccurrenceWake(ctx, owner, doc.ComputerID, texture, occurrence.ResolvedTargetWorkItemID, occurrence)
	if err != nil || r1 == nil {
		t.Fatalf("owner revision run = %+v, %v", r1, err)
	}
	// What the handler leaves on the run before it executes (handler.go:~780).
	r1.Metadata["actor_reactivate_existing_memory"] = true
	r1.Metadata["actor_reactivated_from_passivated"] = true
	r1.Metadata["texture_trigger_occurrence"] = occurrenceContent
	workID := metadataStringValue(r1.Metadata, "lifecycle_work_item_id")
	if workID == "" || r1.Metadata["request_intent"] != "apply_owner_revision" {
		t.Fatalf("owner revision run metadata: %+v", r1.Metadata)
	}
	head, err := s.GetLifecycleDocument(ctx, owner, doc.ComputerID, doc.DocID)
	if err != nil {
		t.Fatal(err)
	}
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:tray-1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "research has been requested", Body: `{"kind":"question","summary":"more"}`},
		Cast:      &types.CommitmentCast{}, Inbox: research, SenderInbox: texture, Addressee: research, ComputerID: doc.ComputerID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
	}
	wait1 := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: head.CurrentRevisionID, Wait: "research has been requested", InboxRecords: []types.CommitmentRecord{cast}})
	if _, err := handler.CommitCellTextureAuthor(ctx, r1, wait1, "wait-1"); err != nil {
		t.Fatalf("owner revision turn: %v", err)
	}
	passivateWithReason(t, s, r1, "passivate-r1", reason)
	return storyWaited{core: core, handler: handler, owner: owner, doc: doc, r1: r1, texture: texture, resear: research, workID: workID}
}

// researchCompletes runs and completes the research desk's records-route run
// the way the runtime ends it, then runs the settlement the store's terminal
// flag triggers.
func researchCompletes(t *testing.T, w storyWaited) {
	t.Helper()
	s := w.core.Store()
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "research-run-" + w.doc.DocID, OwnerID: w.owner, ComputerID: w.doc.ComputerID,
		AgentID: w.resear, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research,
		ChannelID: w.doc.DocID, TrajectoryID: w.doc.TrajectoryID, State: types.RunRunning,
		Metadata:  map[string]any{"request_source": "inbox_record"},
		CreatedAt: now, UpdatedAt: now,
	}
	for i, state := range []types.RunState{types.RunRunning, types.RunCompleted} {
		run.State, run.UpdatedAt = state, time.Now().UTC()
		if state.Terminal() {
			fin := run.UpdatedAt
			run.FinishedAt = &fin
		}
		req := types.ReplaceLifecycleActivationRequest{OwnerID: w.owner, ComputerID: w.doc.ComputerID, CommandID: "research-run-" + string(rune('a'+i)), TrajectoryID: w.doc.TrajectoryID, AgentID: w.resear, Run: run}
		req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
		if _, err := s.ReplaceLifecycleActivation(t.Context(), req); err != nil {
			t.Fatalf("research run %s: %v", state, err)
		}
	}
	stored, err := s.GetLifecycleRun(t.Context(), w.owner, w.doc.ComputerID, run.RunID)
	if err != nil || !store.LifecycleTerminalSettlementRequested(stored) {
		t.Fatalf("research run flag requested=%v err=%v; want the store to set lifecycle_terminal_settlement_requested", store.LifecycleTerminalSettlementRequested(stored), err)
	}
	if err := s.ReconcileLifecycleSettlementForTerminalRun(t.Context(), stored); err != nil {
		t.Fatalf("terminal settlement: %v", err)
	}
}

func commitResearchReport(t *testing.T, w storyWaited) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: w.owner, ComputerID: w.doc.ComputerID, CommandID: "report-1", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell:report:tray-1", Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "found more"}}}, Inbox: w.texture, SenderInbox: w.resear, Addressee: w.texture,
		ComputerID: w.doc.ComputerID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: w.resear},
	}}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := w.core.Store().CommitInboxRecords(t.Context(), req); err != nil {
		t.Fatalf("report: %v", err)
	}
}

func TestStoryReportAfterOwnerReviseWaitWakesTexture(t *testing.T) {
	for _, reason := range []string{"idle", "durable_transition_success"} {
		t.Run(reason, func(t *testing.T) {
			w := storyWaitingOnResearch(t, reason)
			ctx := t.Context()
			researchCompletes(t, w)
			commitResearchReport(t, w)

			snapshot, err := w.core.Store().GetLifecycleSnapshot(ctx, w.owner, w.doc.ComputerID, w.doc.TrajectoryID)
			if err != nil || snapshot.Trajectory.Status != types.TrajectoryLive {
				t.Fatalf("after research's settlement: trajectory=%+v err=%v; want live", snapshot.Trajectory.Status, err)
			}
			open := false
			for _, work := range snapshot.WorkItems {
				if work.WorkItemID == w.workID && work.Status == types.WorkItemOpen {
					open = true
				}
			}
			if !open {
				t.Fatalf("research's settlement closed the reopened work %s: %+v", w.workID, snapshot.WorkItems)
			}

			woken, err := w.handler.ReconcileAgentWake(ctx, w.owner, w.doc.DocID)
			if err != nil || woken == nil {
				t.Fatalf("report wake run=%+v err=%v; want Texture reactivated for the report", woken, err)
			}
			if woken.RunID != w.r1.RunID || metadataStringValue(woken.Metadata, "lifecycle_work_item_id") != w.workID {
				t.Fatalf("woken run %s work %q; want %s on %s", woken.RunID, metadataStringValue(woken.Metadata, "lifecycle_work_item_id"), w.r1.RunID, w.workID)
			}
			if err := w.handler.ValidateActivationAuthority(ctx, w.owner, w.doc.ComputerID, w.texture, woken.RunID); err != nil {
				t.Fatalf("validate reactivated run: %v", err)
			}
			if err := w.core.ExecuteActivationSyncChecked(ctx, woken); err != nil {
				t.Fatalf("execute reactivated run: %v", err)
			}
		})
	}
}
