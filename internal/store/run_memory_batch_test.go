package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes of the run-memory window (docs/design/tape-group-commit-2026-10-31.md
// section 9), written before the code:
//   W1 a kind whose loss causes duplicate or wrong work after a restart waits
//      in the window (cursor, cell fate, compaction, receipt, unknown kinds);
//   W2 a live run cannot read its own queued messages, or reads them
//      differently from the rows they become;
//   W3 seq or parent is reused or broken while entries are queued, including
//      after the tail cache forgets the run;
//   W4 a crash leaves a hole (a later entry durable, an earlier one lost);
//   W5 a run marked finished on the tape is missing queued memory, so the next
//      run would seed from a partial log;
//   W6 a queued entry that cannot be sealed is lost silently and the run goes
//      on with a hole;
//   W7 the planned-restart drain loses queued memory;
//   W8 a read across runs misses queued entries;
//   W9 the batched append (opening and carried messages) is not one write, or
//      mixes runs.

func rmMessage(run, id string) types.RunMemoryEntry {
	return types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", AgentID: "agent-1",
		Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(fmt.Sprintf(`{"role":"user","content":%q}`, id))}
}

func windowHarness(t *testing.T) (*tapeHarness, *telemetrySealer) {
	t.Helper()
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	h.store.SetTapeGroupCommit(true)
	h.warm()
	sl.drain()
	return h, sl
}

func (h *tapeHarness) rmOnTape(id string) bool { return len(h.eventsHolding("run_memory:"+id)) > 0 }

// W1.
func TestRunMemoryWindowHoldsOnlyMessages(t *testing.T) {
	h, sl := windowHarness(t)
	ctx := context.Background()
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w1", "msg-1")); err != nil {
		t.Fatal(err)
	}
	if sl.depth() != 1 || h.rmOnTape("msg-1") {
		t.Fatalf("a message is not queued (depth %d, on tape %v)", sl.depth(), h.rmOnTape("msg-1"))
	}
	for _, kind := range []types.RunMemoryEntryKind{types.RunMemoryEntryCompaction, types.RunMemoryEntryCellFate, "rlm_inbox_cursor", "texture_cell_author_receipt", "made_up_kind"} {
		e := rmMessage("run-w1", "e-"+string(kind))
		e.Kind = kind
		if _, err := h.store.AppendRunMemoryEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
		if !h.rmOnTape("e-" + string(kind)) {
			t.Fatalf("%s was not written acknowledged before its append returned", kind)
		}
		if _, err := newRunMemoryRecord(computerevent.RunMemoryEntryProjection{Kind: string(kind)}, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry}); err == nil {
			t.Fatalf("newRunMemoryRecord accepted %s", kind)
		}
	}
	// Each acknowledged write rode the queue: the message reached the tape
	// ahead of the first one.
	if !h.rmOnTape("msg-1") || sl.depth() != 0 {
		t.Fatal("the queued message did not ride the next acknowledged write")
	}
	body, _ := json.Marshal(computerevent.RunMemoryEntryProjection{Kind: "rlm_inbox_cursor"})
	if err := recheckTelemetryOp(computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry, Body: body}); err == nil {
		t.Fatal("the seal-time recheck passed a cursor entry")
	}
}

// W2, W3.
func TestRunMemoryWindowReadsAndNumbersQueuedEntries(t *testing.T) {
	h, sl := windowHarness(t)
	ctx := context.Background()
	for i := 1; i <= 3; i++ {
		if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w2", fmt.Sprintf("m%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	// The cache forgets the run: the next append must seal and read the table.
	h.store.runMem.mu.Lock()
	h.store.runMem.tails = nil
	h.store.runMem.mu.Unlock()
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w2", "m4")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w2", "m5")); err != nil {
		t.Fatal(err)
	}
	if sl.depth() == 0 {
		t.Fatal("nothing queued: the test reads no queued entry")
	}
	queued, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-w2")
	if err != nil {
		t.Fatal(err)
	}
	latest, err := h.store.LatestRunMemoryEntry(ctx, "run-w2")
	if err != nil {
		t.Fatal(err)
	}
	byID, err := h.store.GetRunMemoryEntry(ctx, "owner-1", "m5")
	if err != nil {
		t.Fatal(err)
	}
	sl.drain()
	sealed, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-w2")
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) != 5 || !reflect.DeepEqual(queued, sealed) {
		t.Fatalf("queued read %+v\nsealed read %+v", queued, sealed)
	}
	for i, e := range sealed {
		if e.Seq != int64(i+1) {
			t.Fatalf("entry %s has seq %d, want %d", e.EntryID, e.Seq, i+1)
		}
		if i > 0 && e.ParentEntryID != sealed[i-1].EntryID {
			t.Fatalf("entry %s parent %q, want %q", e.EntryID, e.ParentEntryID, sealed[i-1].EntryID)
		}
	}
	if !reflect.DeepEqual(latest, sealed[4]) || !reflect.DeepEqual(byID, sealed[4]) {
		t.Fatalf("latest %+v / by id %+v differ from the row %+v", latest, byID, sealed[4])
	}
}

// W4, W5.
func TestRunMemoryWindowCrashLosesOnlyATail(t *testing.T) {
	h, _ := windowHarness(t)
	ctx := context.Background()
	for _, id := range []string{"a1", "a2"} {
		if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w4", id)); err != nil {
			t.Fatal(err)
		}
	}
	// A run-state change is acknowledged: the queued messages go first.
	if err := h.store.AppendEvent(ctx, h.event("done-w4", "run-w4", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"b1", "b2"} {
		if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w4", id)); err != nil {
			t.Fatal(err)
		}
	}
	h.reopen()
	got, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-w4")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range got {
		ids = append(ids, e.EntryID)
	}
	if strings.Join(ids, ",") != "a1,a2" {
		t.Fatalf("after the crash run memory = %v, want a1,a2: everything before the acknowledged write, only the open window lost", ids)
	}
}

// W6.
func TestRunMemoryWindowLostEntryFailsTheRun(t *testing.T) {
	h, sl := windowHarness(t)
	ctx := context.Background()
	h.dryRunHook = func(batch *computerevent.ProjectionBatch) error {
		for _, op := range batch.Ops {
			if op.CanonicalID == "poison" {
				return errors.New("refused")
			}
		}
		return nil
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w6", "ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w6", "poison")); err != nil {
		t.Fatal(err)
	}
	sl.drain()
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w6", "after")); !errors.Is(err, ErrRunMemoryLost) {
		t.Fatalf("append after a lost entry: %v, want ErrRunMemoryLost", err)
	}
	if _, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-w6"); !errors.Is(err, ErrRunMemoryLost) {
		t.Fatalf("read after a lost entry: %v, want ErrRunMemoryLost", err)
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w6-other", "fine")); err != nil {
		t.Fatalf("another run was failed too: %v", err)
	}
}

// W7.
func TestRunMemoryWindowPlannedDrainLosesNothing(t *testing.T) {
	h, _ := windowHarness(t)
	ctx := context.Background()
	for _, id := range []string{"p1", "p2", "p3"} {
		if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w7", id)); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.store.SetTelemetrySynchronous(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w7", "p4")); err != nil {
		t.Fatal(err)
	}
	if !h.rmOnTape("p4") {
		t.Fatal("with synchronous mode held, a message still waited in the window")
	}
	h.reopen()
	got, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-w7")
	if err != nil || len(got) != 4 {
		t.Fatalf("after the drained restart: %d entries (%v), want 4", len(got), err)
	}
}

// W8.
func TestRunMemoryWindowCrossRunReadSeals(t *testing.T) {
	h, sl := windowHarness(t)
	ctx := context.Background()
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-w8", "x1")); err != nil {
		t.Fatal(err)
	}
	got, err := h.store.ListRunMemoryEntriesForAgent(ctx, "owner-1", "agent-1", []string{string(types.RunMemoryEntryMessage)})
	if err != nil {
		t.Fatal(err)
	}
	if sl.depth() != 0 || len(got) != 1 {
		t.Fatalf("agent read: depth %d, %d entries; want the queue sealed and the entry read", sl.depth(), len(got))
	}
}

// W9: one write for a run's opening messages, with the switch off (no
// durability change) and on.
func TestRunMemoryBatchedAppendIsOneWrite(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	ctx := context.Background()
	before := len(h.tape.snapshot())
	batch := []types.RunMemoryEntry{rmMessage("run-w9", "o1"), rmMessage("run-w9", "o2"), rmMessage("run-w9", "o3")}
	batch[0].Kind = types.RunMemoryEntryCompaction
	out, err := h.store.AppendRunMemoryEntries(ctx, batch)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(h.tape.snapshot()) - before; n != 1 {
		t.Fatalf("three entries took %d tape events, want 1", n)
	}
	for i, e := range out {
		if e.Seq != int64(i+1) || (i > 0 && e.ParentEntryID != out[i-1].EntryID) {
			t.Fatalf("entry %d = seq %d parent %q", i, e.Seq, e.ParentEntryID)
		}
	}
	if _, err := h.store.AppendRunMemoryEntries(ctx, []types.RunMemoryEntry{rmMessage("run-a", "x"), rmMessage("run-b", "y")}); err == nil {
		t.Fatal("a batch across two runs was accepted")
	}

	sl := h.enableBatching(farFuture)
	h.store.SetTapeGroupCommit(true)
	sl.drain()
	more := []types.RunMemoryEntry{rmMessage("run-w9", "o4"), rmMessage("run-w9", "o5")}
	more[1].Kind = types.RunMemoryEntryCompaction
	if _, err := h.store.AppendRunMemoryEntries(ctx, more); err != nil {
		t.Fatal(err)
	}
	o4, o5 := h.eventsHolding("run_memory:o4"), h.eventsHolding("run_memory:o5")
	if len(o4) != 1 || len(o5) != 1 || o4[0] != o5[0] {
		t.Fatalf("o4 in %v, o5 in %v: the queued message must ride the batch's acknowledged entry", o4, o5)
	}
	all, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-w9")
	if err != nil || len(all) != 5 || all[4].Seq != 5 || all[4].ParentEntryID != "o4" {
		t.Fatalf("run memory after both batches: %+v (%v)", all, err)
	}
}
