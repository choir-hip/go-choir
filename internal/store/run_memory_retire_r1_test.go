package store

// R-1: the lookup that finds a run's record, the protection of the run the
// carry reader takes, the no-record hold and the catch-up. The failure list is
// the header of run_memory_retire_test.go.

import (
	"context"
	"encoding/json"
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
	fin := now.Add(-finishedAgo)
	rec := types.RunRecord{RunID: runID, AgentID: agentID, OwnerID: ownerID, ComputerID: "computer-1", TrajectoryID: "trajectory-1", State: state, CreatedAt: fin.Add(-time.Hour), UpdatedAt: fin}
	if state.Terminal() {
		rec.FinishedAt = &fin
	}
	obj, err := lifecycleObject(ogKindRun, ownerID, "computer-1", runID, rec, lifecycleMetadataAgent(runID, agentID), rec.CreatedAt, rec.UpdatedAt)
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
