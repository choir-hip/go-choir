package textureowner

// A cell-author receipt is a run memory entry; run memory retirement groups a
// run's rows and reads its agent from them. A receipt with no agent id lets a
// document custodian's run group under no agent (RR-B2).

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestCellAuthorReceiptCarriesTheRunsAgentID(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "receipt.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := &Handler{Store: s}
	rec := &types.RunRecord{RunID: "run-receipt", OwnerID: "owner-r", AgentID: "texture:doc-r"}
	h.recordCellAuthorReceipt(context.Background(), rec, "identity-1", "receipt-1")
	entries, err := s.ListRunMemoryEntries(context.Background(), "owner-r", "run-receipt")
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %d, %v", len(entries), err)
	}
	if entries[0].AgentID != "texture:doc-r" {
		t.Fatalf("receipt agent id = %q, want the run's", entries[0].AgentID)
	}
}
