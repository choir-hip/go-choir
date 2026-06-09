package store

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestRunMemoryAppendNormalizesUnicodeText(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	invalid := string([]byte{'o', 'k', ' ', 0xe2, 0x80})
	message := json.RawMessage(`{"role":"assistant","content":"compaction — résumé ✅"}`)

	entry, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:            "run-memory-unicode",
		OwnerID:          "owner-1",
		AgentID:          "agent-1",
		Kind:             types.RunMemoryEntryCompaction,
		Role:             "assistant",
		Message:          message,
		Summary:          invalid + " summary — checkpoint ✅",
		FirstKeptEntryID: invalid + " first",
		TokensBefore:     42,
		Reason:           invalid + " reason",
		Model:            invalid + " model",
		Details: map[string]any{
			"note": invalid + " details — checkpoint ✅",
		},
	})
	if err != nil {
		t.Fatalf("append unicode run memory: %v", err)
	}

	got, err := s.GetRunMemoryEntry(ctx, "owner-1", entry.EntryID)
	if err != nil {
		t.Fatalf("get unicode run memory: %v", err)
	}
	if !utf8.ValidString(got.Summary) || !utf8.ValidString(got.FirstKeptEntryID) ||
		!utf8.ValidString(got.Reason) || !utf8.ValidString(got.Model) ||
		!utf8.ValidString(string(got.Message)) {
		t.Fatalf("run memory text was not normalized: %+v message=%q", got, string(got.Message))
	}
	if !strings.Contains(got.Summary, "\uFFFD") {
		t.Fatalf("summary did not replace invalid source text: %q", got.Summary)
	}
	if !strings.Contains(got.Summary+got.Reason+got.Model+string(got.Message), "—") {
		t.Fatalf("ordinary unicode punctuation was stripped: %+v message=%q", got, string(got.Message))
	}
}
