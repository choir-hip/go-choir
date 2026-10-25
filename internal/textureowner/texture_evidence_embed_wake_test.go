package textureowner

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// P7 of trace review round 4 (docs/evidence/selfdev-trace-reviews-2026-10-25.md):
// management's final Report (Texture inbox seq 4, 06:01:37) never started a
// Texture turn. The lifecycle snapshot shows why: at Freeze (06:00:00) the
// station TE evidence embed advanced the head to 6ab01f15, an appagent
// revision labelled texture whose metadata has no desk "source", so
// PendingTextureOwnerRevision counted it as unconsumed owner input. The wake
// then took the owner-revision shape, which needs an open Texture work item;
// both had completed (05:21:53, 05:27:33), so the submit was refused and the
// controller returned no run, leaving the records unread.
//
// Ways this can fail, written before the fix: the embed counts as owner
// input (the bug); excluding it also hides a real owner edit made after it;
// the records wake still requires an open work item; the run it starts
// fails activation authority.
func TestEvidenceEmbedIsNotOwnerInputAndARecordStillWakesTexture(t *testing.T) {
	h, core, rec, _ := evidenceSetup(t, "embed-wake")
	ctx := context.Background()
	s := core.Store()
	texture := rec.AgentID

	// Texture's earlier turns are done: its run ended and its work closed.
	ended := rec
	finished := time.Now().UTC()
	ended.State, ended.UpdatedAt, ended.FinishedAt = types.RunCompleted, finished, &finished
	end := types.ReplaceLifecycleActivationRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "end-embed-wake", TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: ended}
	end.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(end)
	if _, err := s.ReplaceLifecycleActivation(ctx, end); err != nil {
		t.Fatalf("end run: %v", err)
	}
	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "close-embed-wake-work", TrajectoryID: rec.TrajectoryID,
		WorkItemID: metadataStringValue(rec.Metadata, "lifecycle_work_item_id"), ActingAgentID: texture,
		RefusalRef: "refusal://test/answered", Reason: "the owner's request was answered",
	}
	refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := s.RefuseLifecycleWork(ctx, refuse); err != nil {
		t.Fatalf("close work: %v", err)
	}

	if _, err := h.EmbedCandidateEvidence(ctx, rec.OwnerID, rec.ComputerID, rec.ComputerID, "selfdev-op-te"); err != nil {
		t.Fatalf("embed: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, pending := store.PendingTextureOwnerRevision(snapshot); pending {
		t.Fatalf("the evidence embed %s counts as pending owner input", snapshot.HeadRevision.RevisionID)
	}

	// Management's final Report to the Texture.
	now := time.Now().UTC()
	report := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "management-report-embed-wake", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "management-cell:report:embed-wake", Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "verify failed: /read.html renders only its header"}}},
		Inbox:  texture, SenderInbox: agentprofile.Management + ":" + rec.OwnerID, Addressee: texture, ComputerID: rec.ComputerID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: agentprofile.Management + ":" + rec.OwnerID, CommittedAt: now.Format(time.RFC3339Nano)},
	}}}
	report.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(report)
	if _, err := s.CommitInboxRecords(ctx, report); err != nil {
		t.Fatalf("commit report: %v", err)
	}

	woken, err := h.ReconcileAgentWake(ctx, rec.OwnerID, rec.ChannelID)
	if err != nil || woken == nil {
		t.Fatalf("records wake after the evidence embed: run=%+v err=%v; want a Texture turn for the report", woken, err)
	}
	if !metadataBoolValue(woken.Metadata, textureRecordsRouteMetadata) {
		t.Fatalf("the wake did not take the records route: %+v", woken.Metadata)
	}
	if err := h.ValidateActivationAuthority(ctx, rec.OwnerID, rec.ComputerID, texture, woken.RunID); err != nil {
		t.Fatalf("the records turn fails activation authority: %v", err)
	}
}
