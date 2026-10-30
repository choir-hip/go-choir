package store

// Telemetry retirement as a tape operation (object_retire.go,
// telemetry_retire.go). Ways it can fail, written before the code; each test
// names the one it catches:
//
//   - R1: the op removes a row it should keep (a newer row, another event
//     kind, another phase, another object kind) or keeps a row it should
//     remove, in either table;
//   - R2: live apply and replay disagree about what was dropped (the cutoff
//     read from the wall clock, an unordered bound);
//   - R3: an invalid body (no kinds, a zero cutoff, another object kind, a
//     negative bound, an unknown phase) is accepted, or reaches the tape;
//   - R4: the dry run applies the delete (paying it twice, or deleting when
//     the batch is later refused);
//   - R5: the job retires a kind the policy keeps (tool.result, the cost
//     lines, the trace inside its window), or retires nothing;
//   - R6: an unbound store (no tape) retires, which replay would not see;
//   - R7: the retirement removes derived keys of other objects.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// retireEvent is one scripted event line: its kind, phase (in the payload,
// where the store keeps it), age and table.
type retireEvent struct {
	id      string
	kind    types.EventKind
	phase   string
	at      time.Time
	stamped bool
}

func (e retireEvent) record() *types.EventRecord {
	payload := fmt.Sprintf(`{"id":%q}`, e.id)
	if e.phase != "" {
		payload = fmt.Sprintf(`{"id":%q,"phase":%q}`, e.id, e.phase)
	}
	return &types.EventRecord{
		EventID: e.id, RunID: "run-1", OwnerID: "owner-1", TrajectoryID: "traj-1",
		Kind: e.kind, Timestamp: e.at, Payload: json.RawMessage(payload),
	}
}

func retireIDs(t *testing.T, s *Store) []string {
	t.Helper()
	var ids []string
	for _, rec := range allStoredEvents(t, s) {
		ids = append(ids, rec.EventID)
	}
	sort.Strings(ids)
	return ids
}

func retireBody(t *testing.T, p computerevent.ObjectRetireProjection) json.RawMessage {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func retireOp(t *testing.T, p computerevent.ObjectRetireProjection) computerevent.ProjectionOp {
	t.Helper()
	return computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObjectRetire, Body: retireBody(t, p)}
}

var retireBase = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func retireScript() []retireEvent {
	old, mid, fresh := retireBase, retireBase.Add(10*24*time.Hour), retireBase.Add(20*24*time.Hour)
	return []retireEvent{
		{id: "inv-old", kind: types.EventToolInvoked, at: old},
		{id: "inv-old-t", kind: types.EventToolInvoked, at: old.Add(time.Minute), stamped: true},
		{id: "inv-fresh", kind: types.EventToolInvoked, at: fresh},
		{id: "res-old", kind: types.EventToolResult, at: old},
		{id: "cell-old", kind: types.EventDeskCell, at: old},
		{id: "cell-old-t", kind: types.EventDeskCell, at: old.Add(time.Minute), stamped: true},
		{id: "cell-mid", kind: types.EventDeskCell, at: mid},
		{id: "prog-none-old", kind: types.EventRunProgress, at: old},
		{id: "prog-none-old-t", kind: types.EventRunProgress, at: old.Add(time.Minute), stamped: true},
		{id: "prog-cost-old", kind: types.EventRunProgress, phase: "tool_loop", at: old},
		{id: "prog-cost-old-t", kind: types.EventRunProgress, phase: "jev_judgment", at: old.Add(time.Minute), stamped: true},
		{id: "prog-cost-fresh", kind: types.EventRunProgress, phase: "tool_loop", at: fresh},
		{id: "done-old", kind: types.EventRunCompleted, at: old},
	}
}

// retireSeed writes the script through the live path into the harness store.
func retireSeed(t *testing.T, h *tapeHarness, script []retireEvent) {
	t.Helper()
	ctx := context.Background()
	for _, e := range script {
		h.store.SetEventTableWrites(e.stamped)
		if err := h.store.AppendEvent(ctx, e.record()); err != nil {
			t.Fatalf("append %s: %v", e.id, err)
		}
	}
	h.store.SetEventTableWrites(false)
}

func retireP(kinds []string, before time.Time, phase string, maxRows int) computerevent.ObjectRetireProjection {
	return computerevent.ObjectRetireProjection{
		ObjectKind: computerevent.ObjectRetireKindEvent, EventKinds: kinds,
		UpdatedBefore: before.UTC().Format(time.RFC3339Nano), Phase: phase, MaxRows: maxRows, Reason: "test",
	}
}

func hasID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// R1, R7: the op removes exactly the aged rows of the named kinds and phase,
// in og_objects and choir_events, and leaves the rest and the derived keys of
// other objects.
func TestObjectRetireRemovesExactlyTheAgedMatchingRows(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	retireSeed(t, h, retireScript())

	// A derived-key row of another object must survive the retirement.
	if _, err := h.store.db.Exec(`INSERT INTO og_key_index (canonical_id, key_name, key_value, owner_id, updated_at, content_hash, derive_ok)
		VALUES ('obj:other', 'run.agent_id', 'agent-1', 'owner-1', ?, 'h', 1)`, retireBase); err != nil {
		t.Fatal(err)
	}
	before := retireIDs(t, h.store)
	if len(before) != len(retireScript()) {
		t.Fatalf("seeded %d events, want %d", len(before), len(retireScript()))
	}
	if tableCount(t, h.store, "choir_events", "") == 0 {
		t.Fatal("the script stamped no event into choir_events")
	}

	cutoff := retireBase.Add(15 * 24 * time.Hour)
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{
		retireOp(t, retireP([]string{"tool.invoked", "desk.cell"}, cutoff, "", 0)),
	}); err != nil {
		t.Fatal(err)
	}
	got := retireIDs(t, h.store)
	for _, gone := range []string{"inv-old", "inv-old-t", "cell-old", "cell-old-t", "cell-mid"} {
		if hasID(got, gone) {
			t.Errorf("%s was not retired", gone)
		}
	}
	for _, kept := range []string{"inv-fresh", "res-old", "prog-none-old", "prog-cost-old", "done-old", "prog-cost-fresh"} {
		if !hasID(got, kept) {
			t.Errorf("%s was retired but should stay", kept)
		}
	}
	if want := len(before) - 5; len(got) != want {
		t.Fatalf("%d events remain, want %d: %v", len(got), want, got)
	}

	// Phase filters: phaseless goes, phased stays, then phased goes.
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{
		retireOp(t, retireP([]string{"loop.progress"}, cutoff, computerevent.ObjectRetirePhaseNone, 0)),
	}); err != nil {
		t.Fatal(err)
	}
	got = retireIDs(t, h.store)
	if hasID(got, "prog-none-old") || hasID(got, "prog-none-old-t") {
		t.Errorf("phaseless loop.progress was not retired: %v", got)
	}
	if !hasID(got, "prog-cost-old") || !hasID(got, "prog-cost-old-t") || !hasID(got, "prog-cost-fresh") {
		t.Errorf("a phased loop.progress was retired by the phaseless op: %v", got)
	}
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{
		retireOp(t, retireP([]string{"loop.progress"}, cutoff, computerevent.ObjectRetirePhaseSome, 0)),
	}); err != nil {
		t.Fatal(err)
	}
	got = retireIDs(t, h.store)
	if hasID(got, "prog-cost-old") || hasID(got, "prog-cost-old-t") || !hasID(got, "prog-cost-fresh") {
		t.Errorf("phased retirement wrong: %v", got)
	}
	if !hasID(got, "res-old") || !hasID(got, "done-old") {
		t.Errorf("another kind was retired: %v", got)
	}
	if n := tableCount(t, h.store, "og_key_index", "canonical_id = 'obj:other'"); n != 1 {
		t.Errorf("derived key of another object: %d rows, want 1", n)
	}
}

// R1: no other object kind is touched, even with an event's metadata shape.
func TestObjectRetireLeavesOtherObjectKinds(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "retire-other-kinds")
	other := objectgraph.Object{
		CanonicalID: "obj:other-kind", ObjectKind: "choir.note", OwnerID: "owner-1",
		ContentHash: "h1", Body: json.RawMessage(`{"payload":{"phase":""}}`),
		Metadata: json.RawMessage(`{"kind":"tool.invoked"}`), CreatedAt: retireBase, UpdatedAt: retireBase,
	}
	evt := retireEvent{id: "inv", kind: types.EventToolInvoked, at: retireBase}
	otherBody, err := json.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}
	projectAt(t, s, 2,
		computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: other.CanonicalID, Body: otherBody},
		eventTableOp(t, evt.record(), false))
	projectAt(t, s, 3, retireOp(t, retireP([]string{"tool.invoked"}, retireBase.Add(time.Hour), "", 0)))
	if n := tableCount(t, s, "og_objects", "object_kind = 'choir.note'"); n != 1 {
		t.Fatalf("a non-event object was retired (%d rows)", n)
	}
	if n := tableCount(t, s, "og_objects", "object_kind = 'choir.event'"); n != 0 {
		t.Fatalf("the event stayed (%d rows)", n)
	}
}

// R2: a tape holding the retire ops, replayed into a fresh store, leaves the
// same rows as the live apply: bounded and unbounded, both tables.
func TestObjectRetireReplayEqualsLive(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	retireSeed(t, h, retireScript())
	cutoff := retireBase.Add(15 * 24 * time.Hour)
	for _, p := range []computerevent.ObjectRetireProjection{
		retireP([]string{"tool.invoked", "desk.cell"}, cutoff, "", 2), // bounded: oldest two first
		retireP([]string{"tool.invoked", "desk.cell"}, cutoff, "", 2),
		retireP([]string{"tool.invoked", "desk.cell"}, cutoff, "", 2),
		retireP([]string{"loop.progress"}, cutoff, computerevent.ObjectRetirePhaseNone, 0),
		retireP([]string{"loop.progress"}, cutoff, computerevent.ObjectRetirePhaseSome, 1),
	} {
		if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{retireOp(t, p)}); err != nil {
			t.Fatal(err)
		}
	}
	var batches [][]computerevent.ProjectionOp
	for _, rec := range h.tape.snapshot() {
		if rec.Kind == computerevent.EventProjectionBatchRecorded {
			batches = append(batches, h.tape.batchOps(t, rec))
		}
	}
	tape := vocabUpcastTapeFromBatches(t, batches)
	replay, _ := vocabUpcastOpenStore(t, "retire-replay")
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))

	live, replayed := retireIDs(t, h.store), retireIDs(t, replay)
	if len(live) == 0 || !reflect.DeepEqual(live, replayed) {
		t.Fatalf("events differ:\nlive   %v\nreplay %v", live, replayed)
	}
	if len(live) >= len(retireScript()) {
		t.Fatalf("nothing was retired: %v", live)
	}
	for _, table := range []string{"og_objects", "choir_events"} {
		a := tableCount(t, h.store, table, retireEventWhere(table))
		b := tableCount(t, replay, table, retireEventWhere(table))
		if a != b {
			t.Errorf("%s event rows: live %d, replay %d", table, a, b)
		}
	}
}

// R2: a bounded op removes the oldest rows first, by (updated_at, canonical_id).
func TestObjectRetireBoundRemovesOldestFirst(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "retire-bound")
	var ops []computerevent.ProjectionOp
	for i := 0; i < 5; i++ {
		e := retireEvent{id: fmt.Sprintf("inv-%d", i), kind: types.EventToolInvoked, at: retireBase.Add(time.Duration(i) * time.Hour)}
		ops = append(ops, eventTableOp(t, e.record(), false))
	}
	projectAt(t, s, 2, ops...)
	projectAt(t, s, 3, retireOp(t, retireP([]string{"tool.invoked"}, retireBase.Add(24*time.Hour), "", 2)))
	got := retireIDs(t, s)
	if want := []string{"inv-2", "inv-3", "inv-4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("remaining %v, want %v", got, want)
	}
}

// R3: an invalid body is refused, and what is refused never reaches the tape.
func TestObjectRetireRefusesAnInvalidBody(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	retireSeed(t, h, retireScript()[:3])
	tapeBefore := len(h.tape.snapshot())
	valid := retireP([]string{"tool.invoked"}, retireBase, "", 0)
	cases := map[string]func(p *computerevent.ObjectRetireProjection){
		"no kinds":        func(p *computerevent.ObjectRetireProjection) { p.EventKinds = nil },
		"blank kind":      func(p *computerevent.ObjectRetireProjection) { p.EventKinds = []string{" "} },
		"zero cutoff":     func(p *computerevent.ObjectRetireProjection) { p.UpdatedBefore = time.Time{}.Format(time.RFC3339Nano) },
		"missing cutoff":  func(p *computerevent.ObjectRetireProjection) { p.UpdatedBefore = "" },
		"garbled cutoff":  func(p *computerevent.ObjectRetireProjection) { p.UpdatedBefore = "yesterday" },
		"other kind":      func(p *computerevent.ObjectRetireProjection) { p.ObjectKind = "choir.run" },
		"no object kind":  func(p *computerevent.ObjectRetireProjection) { p.ObjectKind = "" },
		"negative bound":  func(p *computerevent.ObjectRetireProjection) { p.MaxRows = -1 },
		"unknown phase":   func(p *computerevent.ObjectRetireProjection) { p.Phase = "sometimes" },
		"padded kind":     func(p *computerevent.ObjectRetireProjection) { p.EventKinds = []string{" tool.invoked"} },
		"empty-but-valid": nil,
	}
	for name, mutate := range cases {
		p := valid
		p.EventKinds = append([]string(nil), valid.EventKinds...)
		if mutate == nil {
			if err := p.Validate(); err != nil {
				t.Errorf("%s: valid body refused: %v", name, err)
			}
			continue
		}
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s: Validate accepted the body", name)
		}
		err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{retireOp(t, p)})
		if err == nil {
			t.Errorf("%s: the append was accepted", name)
		} else if !errors.Is(err, computerevent.ErrBatchRefused) && !errors.Is(err, computerevent.ErrProjectionBatchInvalid) {
			t.Errorf("%s: unexpected error class: %v", name, err)
		}
	}
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObjectRetire, Body: json.RawMessage(`not json`)}}); err == nil {
		t.Error("a body that is not JSON was accepted")
	}
	if got := len(h.tape.snapshot()); got != tapeBefore {
		t.Fatalf("a refused retire reached the tape: %d events, was %d", got, tapeBefore)
	}
	if got := retireIDs(t, h.store); len(got) != 3 {
		t.Fatalf("a refused retire removed rows: %v", got)
	}
	// A v1 batch cannot carry the op.
	v1 := computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV1, ProjectorVersion: computerevent.ProjectorVersionV1,
		ComputerID: "c", EventID: "e", Ops: []computerevent.ProjectionOp{retireOp(t, valid)}}
	if err := v1.Validate(); err == nil {
		t.Error("a v1 batch carrying a retire op validated")
	}
}

// R4: the dry run validates a retire op and removes nothing.
func TestObjectRetireDryRunDeletesNothing(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	retireSeed(t, h, retireScript())
	before := retireIDs(t, h.store)
	batch := &computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: h.computerID, EventID: "dry",
		Ops: []computerevent.ProjectionOp{retireOp(t, retireP([]string{"tool.invoked", "desk.cell", "loop.progress", "tool.result"}, retireBase.Add(365*24*time.Hour), "", 0))},
	}
	if err := h.store.DryRunProjectionBatch(ctx, h.computerID, batch); err != nil {
		t.Fatal(err)
	}
	if after := retireIDs(t, h.store); !reflect.DeepEqual(before, after) {
		t.Fatalf("the dry run removed rows: %v -> %v", before, after)
	}
	bad := retireP(nil, retireBase, "", 0)
	batch.Ops = []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObjectRetire, Body: retireBody(t, bad)}}
	if err := h.store.DryRunProjectionBatch(ctx, h.computerID, batch); err == nil {
		t.Fatal("the dry run accepted an invalid retire body")
	}
}

func withRetireKnobs(t *testing.T) {
	t.Helper()
	chunk, pause, maxChunks := telemetryRetireChunkRows, telemetryRetireChunkPause, telemetryRetireMaxChunks
	telemetryRetireChunkRows, telemetryRetireChunkPause = 2, time.Millisecond
	t.Cleanup(func() {
		telemetryRetireChunkRows, telemetryRetireChunkPause, telemetryRetireMaxChunks = chunk, pause, maxChunks
	})
}

// R5: the job applies the policy of section 11.6 and leaves tool.result, the
// cost lines and the trace inside its window.
func TestRetireTelemetryAppliesThePolicy(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	script := []retireEvent{
		{id: "inv-1", kind: types.EventToolInvoked, at: now.Add(-time.Hour)},
		{id: "inv-2", kind: types.EventToolInvoked, at: now.Add(-40 * day), stamped: true},
		{id: "tx-prog", kind: types.EventTextureAgentRevisionProgress, at: now.Add(-2 * time.Hour)},
		{id: "prog-none-1", kind: types.EventRunProgress, at: now.Add(-time.Hour)},
		{id: "prog-none-2", kind: types.EventRunProgress, at: now.Add(-30 * day), stamped: true},
		{id: "cost-new", kind: types.EventRunProgress, phase: "tool_loop", at: now.Add(-day)},
		{id: "jev-new", kind: types.EventRunProgress, phase: "jev_judgment", at: now.Add(-3 * day), stamped: true},
		{id: "cost-old", kind: types.EventRunProgress, phase: "tool_loop", at: now.Add(-20 * day)},
		{id: "budget-old", kind: types.EventRunProgress, phase: "tool_loop_budget_usage", at: now.Add(-20 * day), stamped: true},
		{id: "cell-new", kind: types.EventDeskCell, at: now.Add(-13 * day)},
		{id: "cell-old", kind: types.EventDeskCell, at: now.Add(-15 * day)},
		{id: "egress-old", kind: types.EventDeskEgress, at: now.Add(-16 * day), stamped: true},
		{id: "egress-new", kind: types.EventDeskEgress, at: now.Add(-2 * day)},
		{id: "result-ancient", kind: types.EventToolResult, at: now.Add(-100 * day)},
		{id: "done-ancient", kind: types.EventRunCompleted, at: now.Add(-100 * day)},
	}
	retireSeed(t, h, script)
	ops, err := h.store.RetireTelemetry(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if ops == 0 {
		t.Fatal("the job appended nothing")
	}
	got := retireIDs(t, h.store)
	// A phased progress line past 14 days (cost-old, budget-old) goes with
	// the trace window; tool.result and the run lifecycle stay.
	want := []string{"cost-new", "jev-new", "cell-new", "egress-new", "result-ancient", "done-ancient"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("remaining\n got %v\nwant %v", got, want)
	}
	// A second run finds nothing to do and appends nothing.
	tapeBefore := len(h.tape.snapshot())
	ops, err = h.store.RetireTelemetry(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if ops != 0 || len(h.tape.snapshot()) != tapeBefore {
		t.Fatalf("an idle run appended %d ops", ops)
	}
}

// R5: the kept cost lines still read as cost entries after the retirement.
func TestRetireTelemetryKeepsTheCostLinesReadable(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	rec := &types.EventRecord{
		EventID: "cost-line", RunID: "run-1", OwnerID: "owner-1", TrajectoryID: "traj-1",
		Kind: types.EventRunProgress, Timestamp: now.Add(-time.Hour),
		Payload: json.RawMessage(`{"phase":"tool_loop","model":"gpt-5","input_tokens":100,"output_tokens":10}`),
	}
	if err := h.store.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.RetireTelemetry(ctx, now); err != nil {
		t.Fatal(err)
	}
	events, err := h.store.ListEventsByOwner(ctx, "owner-1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].EventID != "cost-line" {
		t.Fatalf("the cost line was retired: %v", events)
	}
}

// retireEventWhere selects the event rows of a table (og_objects also holds
// the stream_seq lease, which a fresh replay store does not carry).
func retireEventWhere(table string) string {
	if table == "og_objects" {
		return "object_kind = 'choir.event'"
	}
	return ""
}

// R6: a store with no bound tape retires nothing.
func TestRetireTelemetryNeedsTheTape(t *testing.T) {
	s, _ := vocabUpcastOpenStore(t, "retire-unbound")
	projectAt(t, s, 2, eventTableOp(t, retireEvent{id: "inv", kind: types.EventToolInvoked, at: retireBase}.record(), false))
	ops, err := s.RetireTelemetry(context.Background(), retireBase.Add(365*24*time.Hour))
	if err != nil || ops != 0 {
		t.Fatalf("ops=%d err=%v", ops, err)
	}
	if got := retireIDs(t, s); len(got) != 1 {
		t.Fatalf("an unbound store retired: %v", got)
	}
}
