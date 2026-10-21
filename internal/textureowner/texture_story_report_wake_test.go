package textureowner

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// passivateIdle leaves a Texture run the way the runtime does after a
// non-revision turn: the mutation sleeps and the run is passivated (idle).
func passivateIdle(t *testing.T, s *store.Store, run *types.RunRecord, command string) {
	t.Helper()
	if err := s.SleepAgentMutationAfterTextureTurn(t.Context(), run.OwnerID, run.ComputerID, run.TrajectoryID, run.RunID); err != nil {
		t.Fatalf("sleep mutation: %v", err)
	}
	rec, err := s.GetLifecycleRun(t.Context(), run.OwnerID, run.ComputerID, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	rec.State, rec.UpdatedAt, rec.FinishedAt = types.RunPassivated, time.Now().UTC(), nil
	rec.Metadata["passivated_reason"] = "idle"
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
//     wake takes the owner-revision shape instead of the records shape.
//
// Result (2026-10-21): neither happens at this layer. With the passivated
// run carrying the reopened work id, ReconcileAgentWake reactivates it on the
// records route, ValidateActivationAuthority accepts it, and it executes. The
// staging stall is therefore not in the controller; this test pins that and
// leaves the actor handler (handleInboxRecord) and the real idle park path as
// the unexercised layers (problem doc P6).
func TestStoryReportAfterOwnerReviseWaitWakesTexture(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	s := core.Store()
	ctx := t.Context()
	owner, doc := openOwnerStory(t, handler, "report")
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
	r1, err := handler.ReconcileAgentWake(ctx, owner, doc.DocID)
	if err != nil || r1 == nil {
		t.Fatalf("owner revision run = %+v, %v", r1, err)
	}
	workID := metadataStringValue(r1.Metadata, "lifecycle_work_item_id")
	if workID == "" {
		t.Fatalf("owner revision run carries no reopened work: %+v", r1.Metadata)
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
	passivateIdle(t, s, r1, "passivate-r1")

	// Research files its report back to the story's Texture.
	report := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: doc.ComputerID, CommandID: "report-1", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell:report:tray-1", Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "found more"}}}, Inbox: texture, SenderInbox: research, Addressee: texture,
		ComputerID: doc.ComputerID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: research},
	}}}
	report.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(report)
	if _, err := s.CommitInboxRecords(ctx, report); err != nil {
		t.Fatalf("report: %v", err)
	}

	woken, err := handler.ReconcileAgentWake(ctx, owner, doc.DocID)
	if err != nil || woken == nil {
		t.Fatalf("report wake run=%+v err=%v; want Texture reactivated for the report", woken, err)
	}
	t.Logf("woken run=%s (r1=%s) state=%s work=%q records_route=%v", woken.RunID, r1.RunID, woken.State, metadataStringValue(woken.Metadata, "lifecycle_work_item_id"), woken.Metadata[textureRecordsRouteMetadata])
	if err := handler.ValidateActivationAuthority(ctx, owner, doc.ComputerID, texture, woken.RunID); err != nil {
		t.Fatalf("validate reactivated run: %v", err)
	}
	execErr := core.ExecuteActivationSyncChecked(ctx, woken)
	after, _ := s.GetLifecycleRun(ctx, owner, doc.ComputerID, woken.RunID)
	t.Logf("execute err=%v run state=%s error=%q", execErr, after.State, after.Error)
}
