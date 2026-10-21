package agentcore

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A Texture run's frame carries the reader's request while it is pending,
// derived from the ledger, not from a record the run may already have read
// (docs/design/research-more-goes-to-research-2026-10-21.md failure mode 11).
func TestTextureFrameCarriesAPendingReaderRequest(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-frame-ask", rt.TextureComputerID()
	management := agentprofile.Management + ":" + owner
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-open-ask:cast:c1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Harbor reopens"},
		Cast:      &types.CommitmentCast{Captures: []string{"cap-court"}, Publish: types.CastPublishAuto},
		Inbox:     types.StoryOpenInbox, SenderInbox: management, Addressee: types.StoryOpenInbox,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: management},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:open-ask", Records: []types.CommitmentRecord{cast}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := s.CommitInboxRecords(ctx, req)
	if err != nil || len(receipt.Records) != 1 {
		t.Fatalf("open story: %+v, %v", receipt, err)
	}
	texture := receipt.Records[0].Inbox
	docID := strings.TrimPrefix(texture, agentprofile.Texture+":")
	doc, err := s.GetLifecycleDocument(ctx, owner, computer, docID)
	if err != nil {
		t.Fatalf("story document: %v", err)
	}
	rec := &types.RunRecord{RunID: "run-frame-ask", AgentID: texture, OwnerID: owner, ComputerID: computer, ChannelID: docID,
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, Metadata: map[string]any{"doc_id": docID}}

	frame, err := rt.currentTextureFrame(ctx, rec)
	if err != nil || frame.ReaderRequest != "" {
		t.Fatalf("frame before the ask: request %q, %v", frame.ReaderRequest, err)
	}
	if _, err := s.CommitStoryResearchRequest(ctx, store.StoryResearchRequest{OwnerID: owner, ComputerID: computer, DocumentID: docID, ClientRequestID: "expand:pub-a:rev-1", ExpectedHeadRevisionID: doc.CurrentRevisionID}); err != nil {
		t.Fatalf("ask: %v", err)
	}
	frame, err = rt.currentTextureFrame(ctx, rec)
	if err != nil || frame.ReaderRequest != "expand:pub-a:rev-1" || !strings.Contains(frame.Render(), "A reader asked for more research") {
		t.Fatalf("frame after the ask: request %q, %v", frame.ReaderRequest, err)
	}
}
