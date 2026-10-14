package store

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// model_id on desk records (docs/problems/desk-records-lack-model-id-2026-10-14.md
// M1): the model that produced an act is observation metadata. A cell
// replayed after a model failover must replay its receipt, not be refused
// as a different command, and the stored record keeps the first model.

func TestInboxReplayIgnoresModelID(t *testing.T) {
	s := openTestStore(t)
	first := inboxNote("model-note-1", "texture:doc-m", "research:doc-m")
	first.Provenance.ModelID = "opencode-go/deepseek-v4.1-flash"
	receipt := mustCommitInbox(t, s, "cmd-model-1", nil, first)

	replayed := first
	replayed.Provenance.ModelID = "chatgpt/gpt-6-luna"
	again, err := commitInbox(t, s, "cmd-model-1", nil, replayed)
	if err != nil {
		t.Fatalf("replay with another model refused: %v", err)
	}
	if len(again.Records) != len(receipt.Records) || again.Records[0].InboxSeq != receipt.Records[0].InboxSeq {
		t.Fatalf("replay = %+v, want the first receipt %+v", again, receipt)
	}
	stored, err := s.ListInboxRecords(t.Context(), inboxTestOwner, inboxTestComputer, "research:doc-m", 0, 0)
	if err != nil || len(stored) != 1 {
		t.Fatalf("stored = %d records, err %v", len(stored), err)
	}
	if got := stored[0].Provenance.ModelID; got != "opencode-go/deepseek-v4.1-flash" {
		t.Fatalf("stored model_id = %q, want the first commit's model", got)
	}
}

func TestReplayDigestsIgnoreModelID(t *testing.T) {
	rec := inboxNote("model-note-2", "management:o", "texture:doc")
	withModel := rec
	withModel.Provenance.ModelID = "chatgpt/gpt-6-luna"

	a, _ := ComputeCommitLifecycleActDigest(types.CommitLifecycleActRequest{CommandID: "c", Record: rec})
	b, _ := ComputeCommitLifecycleActDigest(types.CommitLifecycleActRequest{CommandID: "c", Record: withModel})
	if a == "" || a != b {
		t.Fatalf("lifecycle act digest depends on model_id: %q vs %q", a, b)
	}
	c, _ := ComputeApplyTextureTurnDigest(types.ApplyTextureTurnRequest{CommandID: "c", InboxRecords: []types.CommitmentRecord{rec}})
	d, _ := ComputeApplyTextureTurnDigest(types.ApplyTextureTurnRequest{CommandID: "c", InboxRecords: []types.CommitmentRecord{withModel}})
	if c == "" || c != d {
		t.Fatalf("texture turn digest depends on a dispatched record's model_id: %q vs %q", c, d)
	}
	// The digest must not clear the caller's slice.
	req := types.ApplyTextureTurnRequest{CommandID: "c", InboxRecords: []types.CommitmentRecord{withModel}}
	_, _ = ComputeApplyTextureTurnDigest(req)
	if req.InboxRecords[0].Provenance.ModelID != "chatgpt/gpt-6-luna" {
		t.Fatalf("digest mutated the request's records")
	}
}
