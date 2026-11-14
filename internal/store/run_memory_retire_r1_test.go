package store

// R-1: the lookup that finds a run's record, the narrowed newest-runs
// protection and the catch-up (the failure list is the header of
// run_memory_retire_test.go: N1, N2, L1, L2, P1, P2, D1, D2, B1, B2).

import (
	"context"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// memSeedLifecycle writes a run record under the computer-scoped id the
// lifecycle path uses (ownerID and computerID in scope, run_id in metadata),
// and entries for it.
func memSeedLifecycle(t *testing.T, h *tapeHarness, ownerID, runID, agentID string, state types.RunState, finishedAgo, entryAgo time.Duration, now time.Time) {
	t.Helper()
	fin := now.Add(-finishedAgo)
	rec := types.RunRecord{RunID: runID, AgentID: agentID, OwnerID: ownerID, ComputerID: "computer-1", TrajectoryID: "trajectory-1", State: state, CreatedAt: now.Add(-entryAgo - time.Hour), UpdatedAt: fin}
	if state.Terminal() {
		rec.FinishedAt = &fin
	}
	obj, err := lifecycleObject(ogKindRun, ownerID, "computer-1", runID, rec, lifecycleMetadata("run_id", runID, "computer-1", "trajectory-1", 1), rec.CreatedAt, rec.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.ogStore.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("put lifecycle run %s: %v", runID, err)
	}
	if _, err := h.store.GetRunByOwnerOG(context.Background(), ownerID, runID); err == nil {
		t.Fatalf("fixture %s is readable by the owner-scoped id; it must be lifecycle-only", runID)
	}
	memSeedEntries(t, h, runID, agentID, []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}, entryAgo, now)
}

func r1Script(t *testing.T, h *tapeHarness, now time.Time) {
	day := 24 * time.Hour
	// L1: lifecycle records, terminal, finished long ago: retire.
	memSeedLifecycle(t, h, "owner-1", "l-done", "worker-1", types.RunCompleted, 30*day, 30*day, now)
	memSeedLifecycle(t, h, "owner-1", "l-failed", "worker-2", types.RunFailed, 30*day, 30*day, now)
	// L2: lifecycle records that must stay.
	memSeedLifecycle(t, h, "owner-1", "l-pass", "worker-3", types.RunPassivated, 30*day, 30*day, now)
	memSeedLifecycle(t, h, "owner-1", "l-blocked", "worker-4", types.RunBlocked, 30*day, 30*day, now)
	memSeedLifecycle(t, h, "owner-1", "l-running", "worker-5", types.RunRunning, 0, 30*day, now)
	memSeedLifecycle(t, h, "owner-1", "l-fresh", "worker-6", types.RunCompleted, 2*day, 30*day, now)
	// L2: a record of the same run id under another owner is not this run's.
	memSeedLifecycle(t, h, "owner-2", "l-foreign", "worker-7", types.RunRunning, 0, 30*day, now)
	memSeedEntries(t, h, "l-foreign", "worker-7", []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}, 30*day, now)
	// N1, N2: no record anywhere.
	memSeedEntries(t, h, "n-old", "worker-8", []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}, 30*day, now)
	memSeedEntries(t, h, "n-new", "worker-9", []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}, 3*day, now)
	// P1: a worker's newest runs are not protected (one agent, three runs).
	memSeed(t, h, "p-1", "worker-10", types.RunCompleted, 12*day, 12*day, now)
	memSeed(t, h, "p-2", "worker-10", types.RunCompleted, 11*day, 11*day, now)
	memSeed(t, h, "p-3", "worker-10", types.RunCompleted, 10*day, 10*day, now)
	// P2: a document custodian keeps its newest two (lifecycle records).
	memSeedLifecycle(t, h, "owner-1", "d-1", docAgent("r1"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "d-2", docAgent("r1"), types.RunCompleted, 35*day, 35*day, now)
	memSeedLifecycle(t, h, "owner-1", "d-3", docAgent("r1"), types.RunCompleted, 32*day, 32*day, now)
}

var r1Retired = []string{"l-done", "l-failed", "l-foreign", "n-old", "p-1", "p-2", "p-3", "d-1"}

func TestRunMemoryRetireR1Selection(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	got, c, err := h.store.selectRunMemory(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), r1Retired...)
	sortStrings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected %v, want %v", got, want)
	}
	byName := censusByName(c)
	for name, runs := range map[string]int64{
		"retired":                           7, // l-done, l-failed, p-1..3, d-1
		"retired_no_run_record":             2, // n-old, l-foreign
		"kept_newest_entry_in_window":       1, // n-new
		"kept_newest_two_of_agent_7_30d":    0,
		"kept_newest_two_of_agent_over_30d": 2, // d-2, d-3
		"kept_state_passivated":             1,
		"kept_state_blocked":                1,
		"kept_state_other:running":          1,
		"kept_finished_in_window":           1,
	} {
		if byName[name].Runs != runs {
			t.Errorf("%s = %d runs, want %d (%+v)", name, byName[name].Runs, runs, c.Dispositions)
		}
	}
	if _, ok := byName["kept_run_record_not_found"]; ok {
		t.Errorf("the not-found disposition is gone with the rule: %+v", c.Dispositions)
	}
	var runs int64
	for _, d := range c.Dispositions {
		runs += d.Runs
	}
	if runs != c.Groups {
		t.Errorf("dispositions sum to %d, groups %d", runs, c.Groups)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// D1, D2: live apply and replay agree on the new cases, a second pass is idle,
// cursors stay, kept runs lose nothing.
func TestRunMemoryRetireR1ReplayEqualsLiveAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	before := memRunsWithRows(t, h.store)
	if ops, err := h.store.RetireRunMemory(ctx, now); err != nil || ops != 1 {
		t.Fatalf("RetireRunMemory = %d, %v; want 1 op", ops, err)
	}
	after := memRunsWithRows(t, h.store)
	gone := map[string]bool{}
	for _, id := range r1Retired {
		gone[id] = true
		if after[id] != 1 {
			t.Errorf("%s kept %d rows, want just its cursor", id, after[id])
		}
	}
	for id, n := range before {
		if !gone[id] && after[id] != n {
			t.Errorf("kept run %s lost rows: %d -> %d", id, n, after[id])
		}
	}
	if ops, err := h.store.RetireRunMemory(ctx, now); err != nil || ops != 0 {
		t.Fatalf("second pass = %d, %v; want idle", ops, err)
	}
	var batches [][]computerevent.ProjectionOp
	for _, rec := range h.tape.snapshot() {
		if rec.Kind == computerevent.EventProjectionBatchRecorded {
			batches = append(batches, h.tape.batchOps(t, rec))
		}
	}
	replay, _ := vocabUpcastOpenStore(t, "memretire-r1-replay")
	tape := vocabUpcastTapeFromBatches(t, batches)
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))
	if got := memRunsWithRows(t, replay); !reflect.DeepEqual(got, after) {
		t.Fatalf("replay rows %v, live %v", got, after)
	}
}

// P2: after the pass a document custodian's next run still carries its
// conversation: the newest completed run holding entries is found.
func TestRunMemoryRetireR1ProtectedAgentStillCarries(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	if _, err := h.store.RetireRunMemory(ctx, now); err != nil {
		t.Fatal(err)
	}
	runID, entries, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("r1"), "")
	if err != nil {
		t.Fatalf("carry source after retirement: %v", err)
	}
	if runID != "d-3" || len(entries) != 3 {
		t.Fatalf("carry source %s with %d entries, want d-3 with 3", runID, len(entries))
	}
	// A retired run's memory is asked for: an empty log, not an error.
	if got, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "l-done"); err != nil || len(got) != 1 || got[0].Kind != "rlm_inbox_cursor" {
		t.Fatalf("retired run read = %d entries, %v; want only its cursor", len(got), err)
	}
}

// B1, B2: a backlog of three passes drains back to back, then stops.
func TestRunMemoryRetireR1BacklogDrainsBackToBack(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	maxOps := runMemoryRetireMaxOps
	runMemoryRetireMaxOps = 1 // 100 runs a pass
	t.Cleanup(func() { runMemoryRetireMaxOps = maxOps })
	const total = 2*computerevent.RunMemoryRetireMaxRuns + 30
	for i := 0; i < total; i++ {
		memSeedEntries(t, h, "b"+strconv.Itoa(1000+i), "worker-b", []types.RunMemoryEntryKind{"message", "message"}, 20*day, now)
	}
	ops, passes, err := h.store.RetireRunMemoryBacklog(ctx, now, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if passes != 4 { // 100, 100, 30, then the pass that finds nothing
		t.Errorf("passes = %d, want 4", passes)
	}
	if ops != total/runMemoryRetireRunsPerOp {
		t.Errorf("ops = %d, want %d", ops, total/runMemoryRetireRunsPerOp)
	}
	if left := memRunsWithRows(t, h.store); len(left) != 0 {
		t.Errorf("%d runs still hold rows", len(left))
	}
	// A cancelled context stops the catch-up between passes.
	for i := 0; i < 150; i++ {
		memSeedEntries(t, h, "c"+strconv.Itoa(1000+i), "worker-c", []types.RunMemoryEntryKind{"message"}, 20*day, now)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := h.store.RetireRunMemoryBacklog(cctx, now, time.Hour); err == nil {
		t.Errorf("a cancelled context did not stop the catch-up")
	}
}
