package textureowner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// SD-08 (docs/problems/owner-acts-on-a-candidate-do-not-wake-texture-2026-10-25.md):
// an owner-act record in a Texture inbox starts a Texture records turn on a
// document whose earlier turns are done and whose work is closed, the state
// reader-4 was in when the owner approved and applied. The Texture side is
// the existing records route; this pins that it carries the new record.
//
// Ways this can fail: the record wakes no one; the wake takes a shape that
// needs an open work item (the P7 failure); the turn fails activation
// authority; the record is not among the turn's unread records.
func TestOwnerActRecordStartsATextureRecordsTurn(t *testing.T) {
	core, h := testAPISetup(t)
	ctx := context.Background()
	// The head is Texture's own rendering ("awaiting the owner"), as
	// reader-4's was: not owner input.
	rec := seedCellAuthorLifecycleOn(t, core.Store(), core.TextureComputerID(), "owner-act", "wake",
		"# Reader change\n\nThe Reader is not verified or accepted.", func(rev *types.Revision) {
			rev.AuthorKind, rev.AuthorLabel = types.AuthorAppAgent, "texture"
			rev.BodyDoc, rev.Content = observationBodyDoc("The Reader is not verified or accepted."), ""
			rev.Metadata, _ = json.Marshal(map[string]any{"source": "texture_cell"})
		})
	h.createAgentMutationForRun(ctx, &rec)
	s := core.Store()
	texture := rec.AgentID

	ended := rec
	finished := time.Now().UTC()
	ended.State, ended.UpdatedAt, ended.FinishedAt = types.RunCompleted, finished, &finished
	end := types.ReplaceLifecycleActivationRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "end-owner-act-wake", TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: ended}
	end.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(end)
	if _, err := s.ReplaceLifecycleActivation(ctx, end); err != nil {
		t.Fatalf("end run: %v", err)
	}
	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "close-owner-act-wake-work", TrajectoryID: rec.TrajectoryID,
		WorkItemID: metadataStringValue(rec.Metadata, "lifecycle_work_item_id"), ActingAgentID: texture,
		RefusalRef: "refusal://test/answered", Reason: "the verify report was rendered",
	}
	refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := s.RefuseLifecycleWork(ctx, refuse); err != nil {
		t.Fatalf("close work: %v", err)
	}

	act, ok := types.NewSelfDevelopmentOwnerAct(types.SelfDevelopmentOwnerAct{
		TextureInbox: texture, ComputerID: rec.ComputerID, OperationID: "selfdev-owner-act", TrajectoryID: rec.TrajectoryID,
		Act: types.OwnerActApply, Body: "Self-development candidate selfdev-owner-act is applied: the computer now runs it, after it was approved.",
	})
	if !ok {
		t.Fatal("no owner-act record built")
	}
	commit := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: act.RecordID, Records: []types.CommitmentRecord{act}}
	commit.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(commit)
	receipt, err := s.CommitInboxRecords(ctx, commit)
	if err != nil {
		t.Fatalf("commit owner act: %v", err)
	}
	if len(receipt.Records) != 1 || receipt.Records[0].WakeID == "" {
		t.Fatalf("the owner-act commit minted no wake row: %+v", receipt.Records)
	}

	woken, err := h.ReconcileAgentWake(ctx, rec.OwnerID, rec.ChannelID)
	if err != nil || woken == nil {
		t.Fatalf("owner-act wake: run=%+v err=%v; want a Texture turn", woken, err)
	}
	if !metadataBoolValue(woken.Metadata, textureRecordsRouteMetadata) {
		t.Fatalf("the wake did not take the records route: %+v", woken.Metadata)
	}
	if err := h.ValidateActivationAuthority(ctx, rec.OwnerID, rec.ComputerID, texture, woken.RunID); err != nil {
		t.Fatalf("the records turn fails activation authority: %v", err)
	}
	unread, err := s.ListInboxRecords(ctx, rec.OwnerID, rec.ComputerID, texture, 0, 1<<40)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range unread {
		found = found || r.RecordID == act.RecordID
	}
	if !found {
		t.Fatalf("the owner-act record is not in %s", texture)
	}
}
