package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRunMemoryAppendListAndLatest(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	first, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:   "run-memory-test",
		OwnerID: "owner-1",
		AgentID: "agent-1",
		Kind:    types.RunMemoryEntryMessage,
		Role:    "user",
		Message: json.RawMessage(`{"role":"user","content":"hello"}`),
	})
	if err != nil {
		t.Fatalf("append first: %v", err)
	}
	second, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:            "run-memory-test",
		OwnerID:          "owner-1",
		AgentID:          "agent-1",
		Kind:             types.RunMemoryEntryCompaction,
		Summary:          "checkpoint",
		FirstKeptEntryID: first.EntryID,
		TokensBefore:     42,
		Reason:           "threshold",
		Details:          map[string]any{"kept_messages": 1},
	})
	if err != nil {
		t.Fatalf("append second: %v", err)
	}

	entries, err := s.ListRunMemoryEntries(ctx, "owner-1", "run-memory-test")
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries: got %d, want 2", len(entries))
	}
	if entries[0].Seq != 1 || entries[1].Seq != 2 {
		t.Fatalf("seqs: got %d,%d want 1,2", entries[0].Seq, entries[1].Seq)
	}
	if entries[1].ParentEntryID != first.EntryID {
		t.Fatalf("parent link: got %q, want %q", entries[1].ParentEntryID, first.EntryID)
	}
	if got := entries[1].Details["kept_messages"]; got == nil {
		t.Fatalf("details missing kept_messages")
	}

	latest, err := s.LatestRunMemoryEntry(ctx, "run-memory-test")
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if latest.EntryID != second.EntryID {
		t.Fatalf("latest entry: got %q, want %q", latest.EntryID, second.EntryID)
	}

	retrieved, err := s.GetRunMemoryEntry(ctx, "owner-1", first.EntryID)
	if err != nil {
		t.Fatalf("get entry: %v", err)
	}
	if retrieved.EntryID != first.EntryID || string(retrieved.Message) != `{"role":"user","content":"hello"}` {
		t.Fatalf("retrieved entry = %+v", retrieved)
	}

	otherOwnerEntries, err := s.ListRunMemoryEntries(ctx, "owner-2", "run-memory-test")
	if err != nil {
		t.Fatalf("list other owner: %v", err)
	}
	if len(otherOwnerEntries) != 0 {
		t.Fatalf("other owner entries: got %d, want 0", len(otherOwnerEntries))
	}

	if _, err := s.GetRunMemoryEntry(ctx, "owner-2", first.EntryID); err == nil {
		t.Fatalf("cross-owner get succeeded, want error")
	}
}
