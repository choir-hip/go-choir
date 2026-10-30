package store

// Run-memory retirement (run_memory_retire.go). Ways it can fail, written
// before the tests:
//
//   - M1: a run that is not finished (running, blocked, passivated), or
//     finished inside the window, loses its entries;
//   - M2: an actor's newest runs lose their entries, so the next run of that
//     actor starts without its prior memory;
//   - M3: the inbox cursor rows, which outlive their run, are removed;
//   - M4: live apply and replay disagree about what was removed;
//   - M5: an invalid body (no runs, over the cap, padded ids) is accepted;
//   - M6: an unbound store retires, which replay would not see;
//   - M7: the dry run applies the delete.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func memSeed(t *testing.T, h *tapeHarness, runID, agentID string, state types.RunState, finishedAgo, entryAgo time.Duration, now time.Time) {
	t.Helper()
	ctx := context.Background()
	fin := now.Add(-finishedAgo)
	rec := types.RunRecord{RunID: runID, AgentID: agentID, OwnerID: "owner-1", ComputerID: "computer-1", State: state, CreatedAt: now.Add(-entryAgo - time.Hour), UpdatedAt: fin}
	if state.Terminal() {
		rec.FinishedAt = &fin
	}
	if err := h.store.CreateRunOG(ctx, rec); err != nil {
		t.Fatalf("create run %s: %v", runID, err)
	}
	for i, kind := range []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"} {
		_, err := h.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
			RunID: runID, OwnerID: "owner-1", AgentID: agentID, Kind: kind, Role: "user",
			Message: json.RawMessage(`{"text":"hello"}`), CreatedAt: now.Add(-entryAgo + time.Duration(i)*time.Second),
		})
		if err != nil {
			t.Fatalf("append %s: %v", runID, err)
		}
	}
}

func memRunsWithRows(t *testing.T, s *Store) map[string]int {
	t.Helper()
	rows, err := s.db.Query(`SELECT loop_id, COUNT(*) FROM run_memory_entries GROUP BY loop_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			t.Fatal(err)
		}
		out[id] = n
	}
	return out
}

func memScript(t *testing.T, h *tapeHarness, now time.Time) {
	day := 24 * time.Hour
	// agent-x: three finished runs; the newest two are protected (M2).
	memSeed(t, h, "x-old", "agent-x", types.RunCompleted, 30*day, 30*day, now)
	memSeed(t, h, "x-mid", "agent-x", types.RunCompleted, 20*day, 20*day, now)
	memSeed(t, h, "x-new", "agent-x", types.RunFailed, 15*day, 15*day, now)
	// agent-y: an old run still running (M1), and one blocked.
	memSeed(t, h, "y-running", "agent-y", types.RunRunning, 0, 40*day, now)
	memSeed(t, h, "y2-blocked", "agent-y2", types.RunBlocked, 40*day, 40*day, now)
	memSeed(t, h, "y3-passivated", "agent-y3", types.RunPassivated, 40*day, 40*day, now)
	// agent-w: finished but inside the window (M1).
	memSeed(t, h, "w-fresh", "agent-w", types.RunCompleted, 3*day, 3*day, now)
	// agent-v: four old finished runs; two oldest go.
	memSeed(t, h, "v1", "agent-v", types.RunCompleted, 50*day, 50*day, now)
	memSeed(t, h, "v2", "agent-v", types.RunCancelled, 45*day, 45*day, now)
	memSeed(t, h, "v3", "agent-v", types.RunCompleted, 44*day, 44*day, now)
	memSeed(t, h, "v4", "agent-v", types.RunCompleted, 43*day, 43*day, now)
}

func TestRunMemoryRetireSelectsOnlyFinishedAgedUnprotectedRuns(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	memScript(t, h, now)
	got, err := h.store.SelectRunMemoryRetirements(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"v1", "v2", "x-old"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected %v, want %v", got, want)
	}
}

func TestRunMemoryRetireKeepsCursorsAndReplayEqualsLive(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	memScript(t, h, now)
	before := memRunsWithRows(t, h.store)
	ops, err := h.store.RetireRunMemory(ctx, now)
	if err != nil || ops != 1 {
		t.Fatalf("RetireRunMemory = %d, %v; want 1 op", ops, err)
	}
	after := memRunsWithRows(t, h.store)
	for _, id := range []string{"v1", "v2", "x-old"} {
		if after[id] != 1 { // only the cursor row (M3)
			t.Fatalf("%s kept %d rows, want just its cursor", id, after[id])
		}
	}
	for id, n := range before {
		if id == "v1" || id == "v2" || id == "x-old" {
			continue
		}
		if after[id] != n {
			t.Fatalf("%s lost rows: %d -> %d", id, n, after[id])
		}
	}
	// An idle second run appends nothing.
	if ops, err := h.store.RetireRunMemory(ctx, now); err != nil || ops != 0 {
		t.Fatalf("second run = %d, %v", ops, err)
	}
	// M4: replay of the tape leaves the same rows.
	var batches [][]computerevent.ProjectionOp
	for _, rec := range h.tape.snapshot() {
		if rec.Kind == computerevent.EventProjectionBatchRecorded {
			batches = append(batches, h.tape.batchOps(t, rec))
		}
	}
	replay, _ := vocabUpcastOpenStore(t, "memretire-replay")
	tape := vocabUpcastTapeFromBatches(t, batches)
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))
	if got := memRunsWithRows(t, replay); !reflect.DeepEqual(got, after) {
		t.Fatalf("replay rows %v, live %v", got, after)
	}
}

func TestRunMemoryRetireRefusesAnInvalidBody(t *testing.T) {
	over := make([]string, computerevent.RunMemoryRetireMaxRuns+1)
	for i := range over {
		over[i] = "r"
	}
	for name, p := range map[string]computerevent.RunMemoryRetireProjection{
		"empty":  {},
		"over":   {RunIDs: over},
		"padded": {RunIDs: []string{" r"}},
		"blank":  {RunIDs: []string{"r"}, KeepKinds: []string{""}},
	} {
		if err := p.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRunMemoryRetireUnboundStoreRetiresNothing(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "memretire-unbound")
	if n, err := s.RetireRunMemory(context.Background(), time.Now()); err != nil || n != 0 {
		t.Fatalf("unbound = %d, %v", n, err)
	}
}
