package store

import (
	"context"
	"testing"
)

// Repair R1: a read of kinds outside the window must not seal (reads do not
// write); an agent-wide message read still does.
func TestRunMemoryAgentReadSealsOnlyForWindowedKinds(t *testing.T) {
	h, sl := windowHarness(t)
	ctx := context.Background()
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-r1", "msg-r1")); err != nil {
		t.Fatal(err)
	}
	if sl.depth() != 1 {
		t.Fatalf("depth %d, want the message queued", sl.depth())
	}
	if _, err := h.store.ListRunMemoryEntriesForAgent(ctx, "owner-1", "agent-1", []string{"rlm_inbox_cursor"}); err != nil {
		t.Fatal(err)
	}
	if sl.depth() != 1 || h.rmOnTape("msg-r1") {
		t.Fatalf("a cursor-kind read sealed the window (depth %d)", sl.depth())
	}
	if _, err := h.store.ListRunMemoryEntriesForAgent(ctx, "owner-1", "agent-1", nil); err != nil {
		t.Fatal(err)
	}
	if sl.depth() != 0 || !h.rmOnTape("msg-r1") {
		t.Fatalf("an all-kinds read did not seal (depth %d)", sl.depth())
	}
}

// Repair R2: turning the setting off drains the queued run memory before it
// returns, so off really stops the lossy window.
func TestSetTapeGroupCommitOffSealsTheWindow(t *testing.T) {
	h, sl := windowHarness(t)
	if _, err := h.store.AppendRunMemoryEntry(context.Background(), rmMessage("run-r2", "msg-r2")); err != nil {
		t.Fatal(err)
	}
	if sl.depth() != 1 {
		t.Fatalf("depth %d, want the message queued", sl.depth())
	}
	h.store.SetTapeGroupCommit(false)
	if sl.depth() != 0 || !h.rmOnTape("msg-r2") {
		t.Fatalf("off left run memory queued (depth %d, on tape %v)", sl.depth(), h.rmOnTape("msg-r2"))
	}
}

// Repair note 2: the store reports a damaged run, so the runtime can refuse to
// complete it as a clean run.
func TestRunMemoryDamagedIsReported(t *testing.T) {
	h, _ := windowHarness(t)
	if h.store.RunMemoryDamaged("run-d") {
		t.Fatal("a fresh run reported damaged")
	}
	h.store.markRunMemoryDamaged("run-d")
	if !h.store.RunMemoryDamaged("run-d") {
		t.Fatal("a damaged run was not reported")
	}
}
