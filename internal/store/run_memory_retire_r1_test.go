package store

// R-1: the lookup that finds a run's record, the protection of the run the
// carry reader takes, the no-record hold and the catch-up. The failure list is
// the header of run_memory_retire_test.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func lifecycleMetadataAgent(runID, agentID string) map[string]any {
	m := lifecycleMetadata("run_id", runID, "computer-1", "trajectory-1", 1)
	m["agent_id"] = agentID
	return m
}

// memPutLifecycleRecord writes a run record under the computer-scoped id the
// lifecycle path uses; no entries.
func memPutLifecycleRecord(t *testing.T, h *tapeHarness, ownerID, runID, agentID string, state types.RunState, finishedAgo time.Duration, now time.Time) {
	t.Helper()
	memPutLifecycleRecordOn(t, h, "computer-1", ownerID, runID, agentID, state, finishedAgo, now)
}

func memPutLifecycleRecordOn(t *testing.T, h *tapeHarness, computerID, ownerID, runID, agentID string, state types.RunState, finishedAgo time.Duration, now time.Time) {
	t.Helper()
	fin := now.Add(-finishedAgo)
	rec := types.RunRecord{RunID: runID, AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: "trajectory-1", State: state, CreatedAt: fin.Add(-time.Hour), UpdatedAt: fin}
	if state.Terminal() {
		rec.FinishedAt = &fin
	}
	obj, err := lifecycleObject(ogKindRun, ownerID, computerID, runID, rec, lifecycleMetadataAgent(runID, agentID), rec.CreatedAt, rec.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.ogStore.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("put lifecycle run %s: %v", runID, err)
	}
}

// memSeedLifecycle writes the lifecycle-only record and entries for a run.
func memSeedLifecycle(t *testing.T, h *tapeHarness, ownerID, runID, agentID string, state types.RunState, finishedAgo, entryAgo time.Duration, now time.Time) {
	t.Helper()
	memPutLifecycleRecord(t, h, ownerID, runID, agentID, state, finishedAgo, now)
	if _, err := h.store.GetRunByOwnerOG(context.Background(), ownerID, runID); err == nil {
		t.Fatalf("fixture %s is readable by the owner-scoped id; it must be lifecycle-only", runID)
	}
	memSeedEntries(t, h, runID, agentID, []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}, entryAgo, now)
}

// memPutOwnerScoped writes an owner-scoped run object whose body and state are
// as given (a tombstone, an undecodable body or a run id that differs).
func memPutOwnerScoped(t *testing.T, h *tapeHarness, ownerID, runID string, body []byte, tombstone bool, now time.Time) {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(ogKindRun, ownerID, objectgraph.StableSuffixFromKey(runID))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := objectgraph.NormalizeMetadata(map[string]any{"run_id": runID})
	if err != nil {
		t.Fatal(err)
	}
	obj := objectgraph.Object{CanonicalID: id, ObjectKind: ogKindRun, OwnerID: ownerID, ContentHash: objectgraph.ContentHash(ogKindRun, body, meta), Body: body, Metadata: meta, CreatedAt: now, UpdatedAt: now, Tombstone: tombstone}
	if err := h.store.ogStore.PutBatch(context.Background(), objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("put owner-scoped run %s: %v", runID, err)
	}
}

func runBody(t *testing.T, runID string, state types.RunState, at time.Time) []byte {
	t.Helper()
	fin := at
	b, err := json.Marshal(types.RunRecord{RunID: runID, OwnerID: "owner-1", ComputerID: "computer-1", State: state, CreatedAt: at, UpdatedAt: at, FinishedAt: &fin})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var entryKinds = []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}

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
	// L2: a record of the same run id under another owner is not this run's, so
	// this run has no record.
	memSeedLifecycle(t, h, "owner-2", "l-foreign", "worker-7", types.RunRunning, 0, 40*day, now)
	// N1, N2: no record anywhere; the hold is thirty days.
	memSeedEntries(t, h, "n-old", "worker-8", entryKinds, 31*day, now)
	memSeedEntries(t, h, "n-hold", "worker-8b", entryKinds, 20*day, now)
	memSeedEntries(t, h, "n-new", "worker-9", entryKinds, 3*day, now)
	// P1: a worker's newest runs are not protected (one agent, three runs).
	memSeed(t, h, "p-1", "worker-10", types.RunCompleted, 12*day, 12*day, now)
	memSeed(t, h, "p-2", "worker-10", types.RunCompleted, 11*day, 11*day, now)
	memSeed(t, h, "p-3", "worker-10", types.RunCompleted, 10*day, 10*day, now)
	// P2: a document custodian keeps its newest two completed runs.
	memSeedLifecycle(t, h, "owner-1", "d-1", docAgent("r1"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "d-2", docAgent("r1"), types.RunCompleted, 35*day, 35*day, now)
	memSeedLifecycle(t, h, "owner-1", "d-3", docAgent("r1"), types.RunCompleted, 32*day, 32*day, now)
	// P2, the repair's fixture: completed 40d, failed 5d, failed 2d. The failed
	// runs are in the window and take no slot; the completed run is the carry
	// source and keeps its entries. An older failed run takes no slot either.
	memSeedLifecycle(t, h, "owner-1", "c-done", docAgent("cf"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "c-f1", docAgent("cf"), types.RunFailed, 5*day, 5*day, now)
	memSeedLifecycle(t, h, "owner-1", "c-f2", docAgent("cf"), types.RunFailed, 2*day, 2*day, now)
	memSeedLifecycle(t, h, "owner-1", "c-f0", docAgent("cf"), types.RunFailed, 20*day, 20*day, now)
	// P2: a run with no record takes no slot; the unreadable record is protected.
	memSeedEntries(t, h, "c-nr", docAgent("nr"), entryKinds, 50*day, now)
	memSeedLifecycle(t, h, "owner-1", "nr-a", docAgent("nr"), types.RunCompleted, 45*day, 45*day, now)
	memSeedLifecycle(t, h, "owner-1", "nr-b", docAgent("nr"), types.RunCompleted, 44*day, 44*day, now)
	memSeedLifecycle(t, h, "owner-1", "nr-c", docAgent("nr"), types.RunCompleted, 43*day, 43*day, now)
	// G1: the run's newest entry is a receipt with no agent id.
	memSeedLifecycle(t, h, "owner-1", "g-1", docAgent("g"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "g-2", docAgent("g"), types.RunCompleted, 35*day, 35*day, now)
	memSeedLifecycle(t, h, "owner-1", "g-3", docAgent("g"), types.RunCompleted, 33*day, 33*day, now)
	if _, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{RunID: "g-3", OwnerID: "owner-1", Kind: "texture_cell_author_receipt", CreatedAt: now.Add(-33*day + time.Minute)}); err != nil {
		t.Fatal(err)
	}
	// L3: an unreadable owner-scoped record is kept.
	memPutOwnerScoped(t, h, "owner-1", "u-bad", []byte(`"not a run"`), false, now)
	memSeedEntries(t, h, "u-bad", "worker-u", entryKinds, 40*day, now)
	// L3: a second copy that is not terminal keeps the run (owner-scoped completed,
	// lifecycle passivated).
	memSeed(t, h, "m-multi", "worker-m", types.RunCompleted, 40*day, 40*day, now)
	memPutLifecycleRecord(t, h, "owner-1", "m-multi", "worker-m", types.RunPassivated, 40*day, now)
	// L3: a tombstoned owner-scoped copy that would keep the run is a miss: the
	// live lifecycle copy (completed) decides, and the run retires.
	memPutOwnerScoped(t, h, "owner-1", "t-tomb", runBody(t, "t-tomb", types.RunPassivated, now.Add(-40*day)), true, now)
	memPutLifecycleRecord(t, h, "owner-1", "t-tomb", "worker-t", types.RunCompleted, 40*day, now)
	memSeedEntries(t, h, "t-tomb", "worker-t", entryKinds, 40*day, now)
	// L3: an owner-scoped object whose run id differs is a miss as well.
	memPutOwnerScoped(t, h, "owner-1", "q-mis", runBody(t, "other-run", types.RunPassivated, now.Add(-40*day)), false, now)
	memPutLifecycleRecord(t, h, "owner-1", "q-mis", "worker-q", types.RunCompleted, 40*day, now)
	memSeedEntries(t, h, "q-mis", "worker-q", entryKinds, 40*day, now)
}

// r1Retired are the runs the pass must name; r1NoRecord of them have no record.
var r1Retired = []string{
	"l-done", "l-failed", "l-foreign", "n-old", "p-1", "p-2", "p-3", "d-1", "c-f0",
	"c-nr", "nr-a", "g-1", "t-tomb", "q-mis",
}
var r1NoRecord = []string{"l-foreign", "n-old", "c-nr"}

func TestRunMemoryRetireR1Selection(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	got, c, err := h.store.selectRunMemory(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), r1Retired...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selected %v, want %v", got, want)
	}
	byName := censusByName(c)
	for name, runs := range map[string]int64{
		"retired_no_run_record":       int64(len(r1NoRecord)),
		"retired":                     int64(len(r1Retired) - len(r1NoRecord)),
		"kept_no_run_record_in_hold":  1, // n-hold
		"kept_newest_entry_in_window": 3, // n-new, c-f1, c-f2
		"kept_carry_source_7_30d":     0,
		"kept_carry_source_over_30d":  7, // d-2 d-3, c-done, nr-b nr-c, g-2 g-3
		"kept_run_record_unreadable":  1, // u-bad
		"kept_state_passivated":       2, // l-pass, m-multi (a second, non-terminal copy)
		"kept_state_blocked":          1,
		"kept_state_other:running":    1,
		"kept_finished_in_window":     1,
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

// D1, D2: live apply and replay agree on the new cases, a second pass is idle,
// cursors stay, kept runs lose nothing. O1: the no-record ops say so.
func TestRunMemoryRetireR1ReplayEqualsLiveAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	before := memRunsWithRows(t, h.store)
	ops, err := h.store.RetireRunMemory(ctx, now)
	if err != nil || ops != 3 { // 11 recorded runs in two ops, 3 with no record in one
		t.Fatalf("RetireRunMemory = %d, %v; want 3 ops", ops, err)
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
	reasons := map[string][]string{}
	for _, rec := range h.tape.snapshot() {
		if rec.Kind != computerevent.EventProjectionBatchRecorded {
			continue
		}
		ops := h.tape.batchOps(t, rec)
		batches = append(batches, ops)
		for _, op := range ops {
			if op.Kind != computerevent.ProjectionOpRunMemoryRetire {
				continue
			}
			p, err := computerevent.DecodeRunMemoryRetire(op.Body)
			if err != nil {
				t.Fatal(err)
			}
			reasons[p.Reason] = append(reasons[p.Reason], p.RunIDs...)
		}
	}
	if len(reasons) != 2 {
		t.Fatalf("ops carry %d reasons, want 2 (recorded, no record): %v", len(reasons), reasons)
	}
	for reason, ids := range reasons {
		sort.Strings(ids)
		if containsAll(reason, "no run record") {
			if !reflect.DeepEqual(ids, sortedCopy(r1NoRecord)) {
				t.Errorf("no-record reason %q names %v", reason, ids)
			}
		} else if len(ids) != len(r1Retired)-len(r1NoRecord) {
			t.Errorf("recorded reason %q names %v", reason, ids)
		}
	}
	replay, _ := vocabUpcastOpenStore(t, "memretire-r1-replay")
	tape := vocabUpcastTapeFromBatches(t, batches)
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))
	if got := memRunsWithRows(t, replay); !reflect.DeepEqual(got, after) {
		t.Fatalf("replay rows %v, live %v", got, after)
	}
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		found := false
		for i := 0; i+len(p) <= len(s); i++ {
			if s[i:i+len(p)] == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// P2: after the pass a document custodian's next run still carries: with a
// failed run newer than the completed one, the completed run keeps its entries.
func TestRunMemoryRetireR1ProtectedAgentStillCarries(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	if _, err := h.store.RetireRunMemory(ctx, now); err != nil {
		t.Fatal(err)
	}
	for agent, wantRun := range map[string]string{"r1": "d-3", "cf": "c-done"} {
		runID, entries, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent(agent), "")
		if err != nil {
			t.Fatalf("carry source of %s after retirement: %v", agent, err)
		}
		// c-f1 and c-f2 are failed: not sources. The carry reader takes c-done.
		if runID != wantRun || len(entries) != 3 {
			t.Fatalf("carry source of %s = %s with %d entries, want %s with 3", agent, runID, len(entries), wantRun)
		}
	}
	// G1: the run whose newest entry is a receipt kept its entries.
	if got, _ := h.store.ListRunMemoryEntries(ctx, "owner-1", "g-3"); len(got) != 4 {
		t.Fatalf("g-3 holds %d entries, want 4", len(got))
	}
	// A retired run's memory is asked for: its cursor only, not an error.
	if got, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "l-done"); err != nil || len(got) != 1 || got[0].Kind != "rlm_inbox_cursor" {
		t.Fatalf("retired run read = %d entries, %v; want only its cursor", len(got), err)
	}
}

// P2: the carry reader skips a run whose only entries are kept cursor rows.
func TestLatestActorRunMemorySkipsARunHoldingOnlyCursors(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memSeedLifecycle(t, h, "owner-1", "k-old", docAgent("k"), types.RunCompleted, 20*day, 20*day, now)
	memPutLifecycleRecord(t, h, "owner-1", "k-new", docAgent("k"), types.RunCompleted, 9*day, now)
	memSeedEntries(t, h, "k-new", docAgent("k"), []types.RunMemoryEntryKind{"rlm_inbox_cursor"}, 9*day, now)
	runID, entries, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("k"), "")
	if err != nil || runID != "k-old" || len(entries) != 3 {
		t.Fatalf("carry source = %q with %d entries, %v; want k-old with 3", runID, len(entries), err)
	}
}

// R1: the selection reads through the read pool. The write handle has one
// connection; hold it, and a read on it would wait out the deadline.
func TestRunMemoryRetireR1SelectionNeverUsesTheWriteConnection(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	hold, err := h.store.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	started := time.Now()
	ids, _, err := h.store.selectRunMemory(ctx, now)
	if err != nil || len(ids) != len(r1Retired) {
		t.Fatalf("selection with the write connection held: %d runs, %v", len(ids), err)
	}
	if time.Since(started) > 15*time.Second {
		t.Fatalf("selection took %s: it waited for the write connection", time.Since(started))
	}
}

// E1: the census is finished on an error path.
func TestRunMemorySelectionFinishesTheCensusOnAnError(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	r1Script(t, h, now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, c, err := h.store.selectRunMemory(ctx, now)
	if err == nil {
		t.Fatal("a cancelled context did not fail the selection")
	}
	if len(c.Dispositions) != len(runMemoryCensusOrder) {
		t.Fatalf("census not finished on the error path: %d dispositions", len(c.Dispositions))
	}
}

func seedBacklog(t *testing.T, h *tapeHarness, prefix string, n int, now time.Time) {
	for i := 0; i < n; i++ {
		memSeedEntries(t, h, prefix+strconv.Itoa(1000+i), "worker-"+prefix, []types.RunMemoryEntryKind{"message", "message"}, 40*24*time.Hour, now)
	}
}

// B1, B2, E1: a backlog of three passes drains back to back, then stops; the
// census keeps the first pass and the wake's totals.
func TestRunMemoryRetireR1BacklogDrainsBackToBack(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	maxOps := runMemoryRetireMaxOps
	runMemoryRetireMaxOps = 1 // 100 runs a pass
	t.Cleanup(func() { runMemoryRetireMaxOps = maxOps })
	const total = 2*computerevent.RunMemoryRetireMaxRuns + 30
	seedBacklog(t, h, "b", total, now)
	ops, passes, err := h.store.RetireRunMemoryBacklog(ctx, now, time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	if passes != 3 { // 100, 100, 30: the last leaves nothing over the cap
		t.Errorf("passes = %d, want 3", passes)
	}
	if ops != total/runMemoryRetireRunsPerOp {
		t.Errorf("ops = %d, want %d", ops, total/runMemoryRetireRunsPerOp)
	}
	if left := memRunsWithRows(t, h.store); len(left) != 0 {
		t.Errorf("%d runs still hold rows", len(left))
	}
	c := h.store.RunMemoryRetirementCensus()
	if c.FirstPass == nil || censusByName(*c.FirstPass)["retired_no_run_record"].Runs != 100 || censusByName(*c.FirstPass)["not_reached_over_cap"].Runs != 130 {
		t.Fatalf("first pass of the wake not kept: %+v", c.FirstPass)
	}
	if c.Passes != 3 || c.Wake == nil || c.Wake.Passes != 3 {
		t.Fatalf("wake = %+v, passes %d", c.Wake, c.Passes)
	}
	var wakeRuns int64
	for _, d := range c.Wake.Retired {
		wakeRuns += d.Runs
	}
	if wakeRuns != total {
		t.Fatalf("wake retired %d runs, want %d (%+v)", wakeRuns, total, c.Wake)
	}
}

// B2: a wake stops at its pass cap, when ready says no, and on a cancelled
// context; a nil store does nothing.
func TestRunMemoryRetireR1BacklogBounds(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	maxOps, maxPasses := runMemoryRetireMaxOps, runMemoryWakeMaxPasses
	runMemoryRetireMaxOps = 1
	t.Cleanup(func() { runMemoryRetireMaxOps, runMemoryWakeMaxPasses = maxOps, maxPasses })
	seedBacklog(t, h, "w", 350, now)

	runMemoryWakeMaxPasses = 2
	if _, passes, err := h.store.RetireRunMemoryBacklog(ctx, now, time.Millisecond, nil); err != nil || passes != 2 {
		t.Fatalf("pass cap: passes = %d, %v; want 2", passes, err)
	}
	if left := len(memRunsWithRows(t, h.store)); left != 150 {
		t.Fatalf("%d runs left after two capped passes, want 150", left)
	}
	runMemoryWakeMaxPasses = 32
	ready := func() bool { return false }
	if _, passes, err := h.store.RetireRunMemoryBacklog(ctx, now, time.Millisecond, ready); err != nil || passes != 1 {
		t.Fatalf("ready false: passes = %d, %v; want 1", passes, err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := h.store.RetireRunMemoryBacklog(cctx, now, time.Hour, nil); err == nil {
		t.Errorf("a cancelled context did not stop the catch-up")
	}
	var nilStore *Store
	if ops, passes, err := nilStore.RetireRunMemoryBacklog(ctx, now, time.Millisecond, nil); err != nil || ops != 0 || passes != 0 {
		t.Errorf("nil store = %d, %d, %v", ops, passes, err)
	}
}

// P2: a document custodian's run whose record does not decode is protected
// (it might be the carry source), and does not use a slot.
func TestRunMemoryRetireR1UnreadableDocumentRunIsProtected(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memPutOwnerScoped(t, h, "owner-1", "ur-bad", []byte(`"not a run"`), false, now)
	memSeedEntries(t, h, "ur-bad", docAgent("ur"), entryKinds, 50*day, now)
	memSeedLifecycle(t, h, "owner-1", "ur-a", docAgent("ur"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "ur-b", docAgent("ur"), types.RunCompleted, 41*day, 41*day, now)
	memSeedLifecycle(t, h, "owner-1", "ur-c", docAgent("ur"), types.RunCompleted, 42*day, 42*day, now)
	ids, c, err := h.store.selectRunMemory(context.Background(), now)
	if err != nil || !reflect.DeepEqual(ids, []string{"ur-c"}) {
		t.Fatalf("selected %v, %v; want only ur-c (the unreadable run takes no slot, and is kept)", ids, err)
	}
	if got := censusByName(c)["kept_carry_source_over_30d"].Runs; got != 3 {
		t.Fatalf("carry sources = %d, want 3 (ur-bad, ur-a, ur-b): %+v", got, c.Dispositions)
	}
}

func seedCursorOnly(t *testing.T, h *tapeHarness, runID, agentID string, ago time.Duration, now time.Time) {
	t.Helper()
	memSeedEntries(t, h, runID, agentID, []types.RunMemoryEntryKind{"rlm_inbox_cursor"}, ago, now)
}

func wantSelected(t *testing.T, h *tapeHarness, now time.Time, want ...string) RunMemoryCensus {
	t.Helper()
	ids, c, err := h.store.selectRunMemory(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if want == nil {
		want = []string{}
	}
	if ids == nil {
		ids = []string{}
	}
	sort.Strings(want)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("selected %v, want %v (%+v)", ids, want, c.Dispositions)
	}
	return c
}

// RR2-B1: a run holding only the kept cursor rows is never a carry source (the
// reader skips it), so it must not take a carry slot either.
func TestRunMemoryRetireCursorOnlyRunsTakeNoCarrySlot(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memSeedLifecycle(t, h, "owner-1", "cs-old", docAgent("cs"), types.RunCompleted, 40*day, 40*day, now)
	for i, id := range []string{"cs-3", "cs-4"} {
		memPutLifecycleRecord(t, h, "owner-1", id, docAgent("cs"), types.RunCompleted, time.Duration(10+i)*day, now)
		seedCursorOnly(t, h, id, docAgent("cs"), time.Duration(10+i)*day, now)
	}
	wantSelected(t, h, now) // cs-old is the carry source and stays
}

// RR2-B2: three or more runs with one finish time: the reader and the selector
// order them the same way (newest first, then run id descending).
func TestCarryReaderAndRetirementAgreeOnEqualFinishTimes(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	ids := []string{"tie-1", "tie-2", "tie-3", "tie-4", "tie-5", "tie-6"}
	for _, id := range ids {
		memSeedLifecycle(t, h, "owner-1", id, docAgent("tie"), types.RunCompleted, 40*day, 40*day, now)
	}
	wantSelected(t, h, now, "tie-1", "tie-2", "tie-3", "tie-4")
	runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("tie"), "")
	if err != nil || runID != "tie-6" {
		t.Fatalf("carry source = %q, %v; want tie-6 (the first slot)", runID, err)
	}
}

// RR2-B2: a stale completed copy and a newer failed copy of one run: both sides
// resolve to the failed copy, which is no source.
func TestCarryReaderAndRetirementResolveCopiesAlike(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memSeedLifecycle(t, h, "owner-1", "cp-0", docAgent("cp"), types.RunCompleted, 50*day, 50*day, now)
	memSeed(t, h, "cp-1", docAgent("cp"), types.RunCompleted, 40*day, 40*day, now) // owner-scoped, stale
	memPutLifecycleRecord(t, h, "owner-1", "cp-1", docAgent("cp"), types.RunFailed, 30*day, now)
	wantSelected(t, h, now, "cp-1")
	runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("cp"), "")
	if err != nil || runID != "cp-0" {
		t.Fatalf("carry source = %q, %v; want cp-0 (cp-1 resolves to failed)", runID, err)
	}
}

// RR2-B2: the reader reads one computer; the slots are per computer.
func TestCarrySlotsAreKeyedByComputer(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memPutLifecycleRecordOn(t, h, "computer-1", "owner-1", "k1", docAgent("kc"), types.RunCompleted, 40*day, now)
	memSeedEntries(t, h, "k1", docAgent("kc"), entryKinds, 40*day, now)
	for i, id := range []string{"k2a", "k2b"} {
		memPutLifecycleRecordOn(t, h, "computer-2", "owner-1", id, docAgent("kc"), types.RunCompleted, time.Duration(35+i)*day, now)
		memSeedEntries(t, h, id, docAgent("kc"), entryKinds, time.Duration(35+i)*day, now)
	}
	wantSelected(t, h, now)
	runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("kc"), "")
	if err != nil || runID != "k1" {
		t.Fatalf("carry source on computer-1 = %q, %v; want k1", runID, err)
	}
}

// RR2-S1: a run whose rows carry two agent ids (a worker's id sorts above the
// document inbox) still groups under the document agent.
func TestRunWithTwoAgentIDsStaysAProtectedCarrySource(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	for i, id := range []string{"ta-1", "ta-2", "ta-3"} {
		memSeedLifecycle(t, h, "owner-1", id, docAgent("ta"), types.RunCompleted, time.Duration(40-3*i)*day, time.Duration(40-3*i)*day, now)
	}
	if _, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{RunID: "ta-3", OwnerID: "owner-1", AgentID: "worker-zzz", Kind: "rlm_inbox_cursor", CreatedAt: now.Add(-31 * day)}); err != nil {
		t.Fatal(err)
	}
	wantSelected(t, h, now, "ta-1")
}

// RR2-S1: the same for owner ids on the rows.
func TestRunWithTwoOwnerIDsStaysAProtectedCarrySource(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	for i, id := range []string{"to-1", "to-2", "to-3"} {
		memSeedLifecycle(t, h, "owner-1", id, docAgent("to"), types.RunCompleted, time.Duration(40-3*i)*day, time.Duration(40-3*i)*day, now)
	}
	if _, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{RunID: "to-3", OwnerID: "owner-9", Kind: "rlm_inbox_cursor", CreatedAt: now.Add(-31 * day)}); err != nil {
		t.Fatal(err)
	}
	wantSelected(t, h, now, "to-1")
}

// RR2-S1: rows and record disagree about the agent: fail closed.
func TestRunWhoseRowsAndRecordDisagreeOnTheAgentIsKept(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memPutLifecycleRecord(t, h, "owner-1", "dis-1", docAgent("other"), types.RunCompleted, 40*day, now)
	memSeedEntries(t, h, "dis-1", docAgent("dis"), entryKinds, 40*day, now)
	memPutLifecycleRecord(t, h, "owner-1", "dis-2", docAgent("dis"), types.RunCompleted, 40*day, now)
	memSeedEntries(t, h, "dis-2", "worker-dis", entryKinds, 40*day, now)
	c := wantSelected(t, h, now)
	if got := censusByName(c)["kept_run_agent_disagrees"].Runs + censusByName(c)["kept_carry_source_over_30d"].Runs; got != 2 {
		t.Fatalf("disagreeing runs kept = %d, want 2 (%+v)", got, c.Dispositions)
	}
}

// RR2-S2: a read error other than not-found is the pass's error, not an
// unreadable record.
func TestRunRecordLookupReturnsReadErrors(t *testing.T) {
	h := newTapeHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.store.runRecordForRetirement(ctx, "owner-1", "any-run")
	if err == nil || errors.Is(err, errRunRecordUnreadable) || errors.Is(err, ErrNotFound) {
		t.Fatalf("lookup with a cancelled context = %v; want the context error", err)
	}
}

// RR2-S3: a pass that fails mid-append counts what it applied.
func TestCensusCountsWhatWasApplied(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	seedBacklog(t, h, "m", 25, now)
	telemetryRetireChunkPause = time.Hour
	go func() {
		time.Sleep(1500 * time.Millisecond)
		cancel()
	}()
	ops, err := h.store.RetireRunMemory(ctx, now)
	if err == nil || ops != 1 {
		t.Fatalf("RetireRunMemory = %d, %v; want 1 op then the context error", ops, err)
	}
	c := h.store.RunMemoryRetirementCensus()
	by := censusByName(c)
	if by["retired_no_run_record"].Runs != 10 || by["kept_append_failed"].Runs != 15 {
		t.Fatalf("census after a mid-pass failure: %+v", c.Dispositions)
	}
	if c.Wake == nil {
		t.Fatal("no wake")
	}
	for _, d := range c.Wake.Retired {
		if d.Disposition == "retired_no_run_record" && d.Runs != 10 {
			t.Fatalf("wake counts %d retired runs, want the 10 applied", d.Runs)
		}
	}
	if left := len(memRunsWithRows(t, h.store)); left != 15 {
		t.Fatalf("%d runs hold rows, want 15", left)
	}
}

// RR2-S4: a run with nothing the carry can use (message, compaction) is no
// source and takes no slot, whatever else it holds.
func TestRunWithOnlyReceiptsIsNoCarrySource(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memPutLifecycleRecord(t, h, "owner-1", "ro-new", docAgent("ro"), types.RunCompleted, 9*day, now)
	memSeedEntries(t, h, "ro-new", docAgent("ro"), []types.RunMemoryEntryKind{"texture_cell_author_receipt", "rlm_inbox_cursor"}, 9*day, now)
	for i, id := range []string{"ro-a", "ro-b", "ro-c"} {
		memSeedLifecycle(t, h, "owner-1", id, docAgent("ro"), types.RunCompleted, time.Duration(20+i)*day, time.Duration(20+i)*day, now)
	}
	wantSelected(t, h, now, "ro-c", "ro-new")
	runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("ro"), "")
	if err != nil || runID != "ro-a" {
		t.Fatalf("carry source = %q, %v; want ro-a", runID, err)
	}
}

// RR2-S6: a backlog error does not skip the rest of the wake.
func TestWakeGoesOnPastABacklogError(t *testing.T) {
	h := newTapeHarness(t)
	delay, fn, done := telemetryRetireFirstDelay, retireRunMemoryBacklogFn, retirementWakeDone
	t.Cleanup(func() { telemetryRetireFirstDelay, retireRunMemoryBacklogFn, retirementWakeDone = delay, fn, done })
	telemetryRetireFirstDelay = 10 * time.Millisecond
	retireRunMemoryBacklogFn = func(*Store, context.Context, func() bool) (int, int, error) {
		return 7, 2, errors.New("append failed")
	}
	finished := make(chan struct{}, 1)
	retirementWakeDone = func() {
		select {
		case finished <- struct{}{}:
		default:
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.store.StartTelemetryRetirement(ctx, nil, nil)
	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("the wake did not reach its maintenance after a backlog error")
	}
}

// RR2-S7: one census per wake in the log, not one per pass.
func TestBacklogLogsOneCensusPerWake(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	maxOps := runMemoryRetireMaxOps
	runMemoryRetireMaxOps = 1
	t.Cleanup(func() { runMemoryRetireMaxOps = maxOps })
	seedBacklog(t, h, "l", 230, now)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	if _, passes, err := h.store.RetireRunMemoryBacklog(ctx, now, time.Millisecond, nil); err != nil || passes != 3 {
		t.Fatalf("passes = %d, %v", passes, err)
	}
	headers := bytes.Count(buf.Bytes(), []byte("run memory census: "))
	one := len(runMemoryCensusOrder) + 1
	if headers == 0 || headers > one+4 {
		t.Fatalf("%d census lines for a 3 pass wake; want one census (about %d) and a wake line", headers, one)
	}
}

// RR3-F5: what this proves is narrow: for a completed producer run of a live
// trajectory the report is refused before and after its memory is retired (the
// same refusal). That the late-evidence branch reads no memory is by reading
// lifecycle.go (inside !lateEvidenceOnly), not by this test. RR2-S5: the
// management late-report path never meets retired memory. The
// producer run of a live trajectory must be pending, running or passivated
// (lifecycle.go persistentManagementRunStateAllowed), none of which retirement
// selects; a terminal producer run is refused (live trajectory) or reads no
// memory (late evidence), so retiring its memory changes nothing. The test
// drives the real report path with a completed producer run before and after
// retirement.
func TestManagementReportRefusalIsUnchangedWithTheProducersMemoryRetired(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	s, start, caller, _ := setupLifecycleTextureTargetFixtureWithStore(t, h.store)
	now := time.Now().UTC()
	managementID := "management:" + start.OwnerID
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: managementID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, Profile: "management", Role: "management", ChannelID: managementID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	packet := textureTurnControlPacket("retired-super")
	digest, _ := ComputeLifecycleUpdatePayloadDigest(packet, "control retired-super")
	workID := "retired-super-work"
	turnReq := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	turnReq.CommandID = "turn-retired-super"
	turnReq.Controls = []types.TextureTurnControl{{ControlID: "control-retired-super", TargetAgentID: managementID, TargetWorkItemID: workID, OpenWork: &types.WorkItemRecord{WorkItemID: workID, Objective: "execute exact request", AuthorityProfile: "management", AssignedAgentID: managementID}, Packet: packet, Content: "control retired-super", PayloadDigest: digest}}
	setTextureTurnDigest(t, &turnReq, TextureSourceGraphWriteSet{})
	turn, err := s.ApplyTextureTurn(ctx, turnReq)
	if err != nil {
		t.Fatal(err)
	}
	run := types.RunRecord{RunID: "retired-super-run", OwnerID: start.OwnerID, ComputerID: start.ComputerID, AgentID: managementID, AgentProfile: "management", AgentRole: "management", ChannelID: start.InitialDocument.DocID, State: types.RunRunning, Metadata: map[string]any{"assignment_trajectory_id": start.TrajectoryID, "work_item_ids": []string{workID}, "lifecycle_work_item_id": workID}, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindLifecycleControlDelivery(ctx, bindControlRequestForTest(t, s, start, run, turn.Controls)); err != nil {
		t.Fatal(err)
	}
	injection, _ := json.Marshal(map[string]any{
		"schema": "choir.lifecycle_injection.v1", "packet_type": "coagent_update",
		"owner_id": start.OwnerID, "computer_id": start.ComputerID, "trajectory_id": start.TrajectoryID,
		"target_agent_id": managementID, "target_run_id": run.RunID,
		"updates": []map[string]any{{"update_id": turn.Controls[0].UpdateID}},
	})
	message, _ := json.Marshal(map[string]any{"role": "user", "content": "Choir authenticated coagent update packet.\n\n" + string(injection)})
	old := now.Add(-20 * 24 * time.Hour)
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{RunID: run.RunID, OwnerID: start.OwnerID, AgentID: managementID, Kind: types.RunMemoryEntryMessage, Role: types.RunMemoryRoleRuntimeInjection, Message: message, CreatedAt: old}); err != nil {
		t.Fatal(err)
	}
	// The run completes more than a week ago.
	run.State, run.UpdatedAt = types.RunCompleted, old
	run.FinishedAt = &old
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	reportPacket := types.SourcePacket{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_result", Summary: "late progress", Notes: []string{"evidence:progress"}}
	reportDigest, _ := ComputeLifecycleUpdatePayloadDigest(reportPacket, "late progress")
	report := types.QueueLifecycleUpdateRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "queue-retired-super", TrajectoryID: start.TrajectoryID,
		TargetAgentID: caller.AgentID, ProducerAgentID: managementID, ControlBindingID: turn.Controls[0].UpdateID, TargetWorkItemID: start.InitialWork.WorkItemID,
		ConsumedDeliveryUpdateIDs: []string{turn.Controls[0].UpdateID},
		ProducerUpdateID:          "retired-progress-occurrence", UpdateID: "retired-progress-result", ChannelID: start.InitialDocument.DocID, Role: "management", SourceRunID: run.RunID,
		Packet: reportPacket, Content: "late progress", WorkDisposition: types.WorkItemCompleted, WorkItemID: workID, PayloadDigest: reportDigest}
	report.CommandDigest, _ = ComputeQueuePersistentManagementReportDigest(report)
	_, before := s.QueueLifecycleUpdate(ctx, report)
	ops, err := s.RetireRunMemory(ctx, now)
	if err != nil || ops == 0 {
		t.Fatalf("retirement of the completed producer run = %d ops, %v; want it retired", ops, err)
	}
	if got, _ := s.ListRunMemoryEntries(ctx, start.OwnerID, run.RunID); len(got) != 0 {
		t.Fatalf("producer memory not retired: %d entries", len(got))
	}
	report.CommandID = "queue-retired-super-2"
	report.CommandDigest, _ = ComputeQueuePersistentManagementReportDigest(report)
	_, after := s.QueueLifecycleUpdate(ctx, report)
	if (before == nil) != (after == nil) || (before != nil && before.Error() != after.Error()) {
		t.Fatalf("report outcome changed with the producer's memory retired: before=%v after=%v", before, after)
	}
}

// RR2-B2: the reader skips a tombstoned record (the selector does).
func TestCarryReaderSkipsATombstonedRecord(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memSeedLifecycle(t, h, "owner-1", "tb-live", docAgent("tb"), types.RunCompleted, 40*day, 40*day, now)
	fin := now.Add(-20 * day)
	rec := types.RunRecord{RunID: "tb-gone", AgentID: docAgent("tb"), OwnerID: "owner-1", ComputerID: "computer-1", State: types.RunCompleted, CreatedAt: fin, UpdatedAt: fin, FinishedAt: &fin}
	obj, err := lifecycleObject(ogKindRun, "owner-1", "computer-1", "tb-gone", rec, lifecycleMetadataAgent("tb-gone", docAgent("tb")), fin, fin)
	if err != nil {
		t.Fatal(err)
	}
	obj.Tombstone = true
	if err := h.store.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatal(err)
	}
	memSeedEntries(t, h, "tb-gone", docAgent("tb"), entryKinds, 20*day, now)
	runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("tb"), "")
	if err != nil || runID != "tb-live" {
		t.Fatalf("carry source = %q, %v; want tb-live (tb-gone is tombstoned)", runID, err)
	}
}

// RR2-S1: a run whose rows and record disagree about the agent takes no slot.
func TestDisagreeingRunTakesNoCarrySlot(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	memPutLifecycleRecord(t, h, "owner-1", "dg-new", docAgent("other"), types.RunCompleted, 30*day, now)
	memSeedEntries(t, h, "dg-new", docAgent("dg"), entryKinds, 30*day, now)
	for i, id := range []string{"dg-a", "dg-b", "dg-c"} {
		memSeedLifecycle(t, h, "owner-1", id, docAgent("dg"), types.RunCompleted, time.Duration(35+i)*day, time.Duration(35+i)*day, now)
	}
	wantSelected(t, h, now, "dg-c")
}

func newMixedStore(t *testing.T) (*tapeHarness, time.Time, time.Duration) {
	h := newTapeHarness(t)
	return h, time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC), 24 * time.Hour
}

// RR3-F1: a run whose document-agent rows carry two owners is kept (the record
// cannot be read under one owner and the delete is by loop id).
func TestMixedOwnerDocumentRowsKeepTheRun(t *testing.T) {
	h, now, day := newMixedStore(t)
	for i, id := range []string{"mo-1", "mo-2", "mo-3"} {
		memSeedLifecycle(t, h, "owner-1", id, docAgent("mo"), types.RunCompleted, time.Duration(40-3*i)*day, time.Duration(40-3*i)*day, now)
	}
	if _, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{RunID: "mo-3", OwnerID: "owner-9", AgentID: docAgent("mo"), Kind: "rlm_inbox_cursor", CreatedAt: now.Add(-31 * day)}); err != nil {
		t.Fatal(err)
	}
	c := wantSelected(t, h, now)
	if got := censusByName(c)["kept_mixed_owner_rows"].Runs; got != 1 {
		t.Fatalf("kept_mixed_owner_rows = %d, want 1 (%+v)", got, c.Dispositions)
	}
}

// RR3-F2: equal-finish copies of one run with different states resolve to the
// carry-source state on both sides, whatever order the copies are read in.
func TestEqualFinishCopiesResolveToTheSourceState(t *testing.T) {
	ctx := context.Background()
	h, now, day := newMixedStore(t)
	for i := 0; i < 6; i++ {
		id, agent := "eq-"+strconv.Itoa(i), docAgent("eq"+strconv.Itoa(i))
		memSeed(t, h, id, agent, types.RunFailed, 40*day, 40*day, now) // owner-scoped copy
		memPutLifecycleRecord(t, h, "owner-1", id, agent, types.RunCompleted, 40*day, now)
	}
	wantSelected(t, h, now)
	for i := 0; i < 6; i++ {
		runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("eq"+strconv.Itoa(i)), "")
		if err != nil || runID != "eq-"+strconv.Itoa(i) {
			t.Fatalf("carry source %d = %q, %v", i, runID, err)
		}
	}
}

// RR3-F2: copies of one run on two computers disagree about where it is: kept.
func TestCopiesOnTwoComputersKeepTheRun(t *testing.T) {
	h, now, day := newMixedStore(t)
	memPutLifecycleRecordOn(t, h, "computer-1", "owner-1", "x2c", "worker-x2c", types.RunCompleted, 40*day, now)
	memPutLifecycleRecordOn(t, h, "computer-2", "owner-1", "x2c", "worker-x2c", types.RunCompleted, 40*day, now)
	memSeedEntries(t, h, "x2c", "worker-x2c", entryKinds, 40*day, now)
	c := wantSelected(t, h, now)
	if got := censusByName(c)["kept_run_copies_disagree"].Runs; got != 1 {
		t.Fatalf("kept_run_copies_disagree = %d, want 1 (%+v)", got, c.Dispositions)
	}
}

// RR3-F2: a record with no computer id is invisible to the reader (it reads one
// computer's runs); it must not take a real computer's slot.
func TestRecordWithoutComputerTakesNoRealSlot(t *testing.T) {
	ctx := context.Background()
	h, now, day := newMixedStore(t)
	memPutLifecycleRecordOn(t, h, "", "owner-1", "e-0", docAgent("ec"), types.RunCompleted, 35*day, now)
	memSeedEntries(t, h, "e-0", docAgent("ec"), entryKinds, 35*day, now)
	memSeedLifecycle(t, h, "owner-1", "e-1", docAgent("ec"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "e-2", docAgent("ec"), types.RunCompleted, 41*day, 41*day, now)
	wantSelected(t, h, now) // e-0 is invisible to the reader and is kept, in a slot of its own
	runID, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", docAgent("ec"), "")
	if err != nil || runID != "e-1" {
		t.Fatalf("carry source = %q, %v; want e-1", runID, err)
	}
}

// RR3-F3: a loop id the op would refuse (padded) is skipped and counted; the
// pass goes on with the rest.
func TestMalformedLoopIDDoesNotStopThePass(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h, now, day := newMixedStore(t)
	memSeedEntries(t, h, "good-1", "worker-g", entryKinds, 40*day, now)
	memSeedEntries(t, h, " pad-1 ", "worker-g", entryKinds, 40*day, now)
	ops, err := h.store.RetireRunMemory(ctx, now)
	if err != nil || ops != 1 {
		t.Fatalf("RetireRunMemory = %d, %v; want 1 op and no error", ops, err)
	}
	rows := memRunsWithRows(t, h.store)
	if rows["good-1"] != 1 || rows[" pad-1 "] != 3 {
		t.Fatalf("rows after the pass: %v", rows)
	}
	if got := censusByName(h.store.RunMemoryRetirementCensus())["kept_loop_id_invalid"].Runs; got != 1 {
		t.Fatalf("kept_loop_id_invalid = %d, want 1", got)
	}
}

// RR3-F4: a wake reads the record of a run kept in pass 1 once; pass 2 does not
// read it again; the counters say so; the plan equals the unmemoized plan.
func TestWakeMemoSkipsRecordReadsForKeptRuns(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h, now, day := newMixedStore(t)
	maxOps := runMemoryRetireMaxOps
	runMemoryRetireMaxOps = 1
	t.Cleanup(func() { runMemoryRetireMaxOps = maxOps })
	for i := 0; i < 20; i++ {
		memSeed(t, h, "a-pass-"+strconv.Itoa(100+i), "worker-p", types.RunPassivated, 30*day, 30*day, now)
	}
	seedBacklog(t, h, "z", 150, now)
	ops, passes, err := h.store.RetireRunMemoryBacklog(ctx, now, time.Millisecond, nil)
	if err != nil || passes != 2 || ops != 15 {
		t.Fatalf("wake = %d ops, %d passes, %v; want 15, 2", ops, passes, err)
	}
	c := h.store.RunMemoryRetirementCensus()
	if c.FirstPass == nil || c.FirstPass.RecordReads.Selection != 120 {
		t.Fatalf("first pass reads = %+v, want 120 (20 kept, 100 retired)", c.FirstPass)
	}
	if c.RecordReads.Selection != 50 {
		t.Fatalf("second pass read %d records, want 50 (the 20 kept runs are known)", c.RecordReads.Selection)
	}
	if c.Wake == nil || c.Wake.RecordReads.Selection != 170 {
		t.Fatalf("wake reads = %+v, want 170", c.Wake)
	}
	if left := len(memRunsWithRows(t, h.store)); left != 20 {
		t.Fatalf("%d runs hold rows, want the 20 passivated", left)
	}
}

func TestWakeMemoPlanEqualsTheUnmemoizedPlan(t *testing.T) {
	ctx := context.Background()
	h, now, day := newMixedStore(t)
	for i := 0; i < 5; i++ {
		memSeed(t, h, "a-pass-"+strconv.Itoa(100+i), "worker-p", types.RunPassivated, 30*day, 30*day, now)
		memSeedLifecycle(t, h, "owner-1", "l-"+strconv.Itoa(100+i), "worker-l", types.RunCompleted, 30*day, 30*day, now)
	}
	memSeedLifecycle(t, h, "owner-1", "d-1", docAgent("wm"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "d-2", docAgent("wm"), types.RunCompleted, 41*day, 41*day, now)
	memSeedLifecycle(t, h, "owner-1", "d-3", docAgent("wm"), types.RunCompleted, 42*day, 42*day, now)
	plain, _, err := h.store.selectRunMemory(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	h.store.runMemoryWake = newRunRecordWakeMemo()
	t.Cleanup(func() { h.store.runMemoryWake = nil })
	first, c1, err := h.store.selectRunMemory(ctx, now)
	second, c2, err2 := h.store.selectRunMemory(ctx, now)
	if err != nil || err2 != nil || !reflect.DeepEqual(plain, first) || !reflect.DeepEqual(plain, second) {
		t.Fatalf("plans differ: plain %v, memo %v, memo again %v (%v, %v)", plain, first, second, err, err2)
	}
	if c1.RecordReads.SlotPrePass+c1.RecordReads.Selection == 0 || c2.RecordReads.SlotPrePass != 0 {
		t.Fatalf("reads: first %+v, second %+v; the second must read no document record", c1.RecordReads, c2.RecordReads)
	}
}

// RR3-F7: a record read error aborts the pass, in the slot pre-pass and in the
// selection; it is not kept, retired or counted unreadable.
func TestRecordReadErrorAbortsThePass(t *testing.T) {
	h, now, day := newMixedStore(t)
	memSeedLifecycle(t, h, "owner-1", "doc-run", docAgent("ab"), types.RunCompleted, 40*day, 40*day, now)
	memSeedLifecycle(t, h, "owner-1", "worker-run", "worker-ab", types.RunCompleted, 40*day, 40*day, now)
	t.Cleanup(func() { runRecordLookupHook = nil })
	for _, failing := range []string{"doc-run", "worker-run"} {
		boom := errors.New("disk i/o " + failing)
		runRecordLookupHook = func(_, runID string) error {
			if runID == failing {
				return boom
			}
			return nil
		}
		_, c, err := h.store.selectRunMemory(context.Background(), now)
		if !errors.Is(err, boom) {
			t.Fatalf("%s: selection error = %v, want the read error", failing, err)
		}
		if len(c.Dispositions) != len(runMemoryCensusOrder) {
			t.Fatalf("%s: census not finished: %d dispositions", failing, len(c.Dispositions))
		}
	}
}
