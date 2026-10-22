package store

// Deploy 2 of run telemetry in its own table (F-c): the writer
// (docs/design/choir-event-table-f-c-2026-10-22.md section 4.2). Ways the
// writer can fail, written before the code; each test names the one it
// catches:
//
//   - W-off: with the setting off the tape is not byte-identical to deploy 1
//     (a stamp, a changed body, an extra op);
//   - W-body: with the setting on a stamped op's body differs from the op the
//     acknowledged path writes, so live and an old binary's reading disagree
//     about the event itself, not only its table;
//   - F9: a kind outside the storage class is stamped, an unbound store
//     stamps, or the sealer's path forgets the stamp (W2);
//   - F1: a seed reads one table and a number is handed out twice once the
//     writer moves events (reopen, batching, parallel appenders);
//   - F2: the union reads differ between a store written with the setting
//     off, on, and toggled mid-run, all through AppendEvent;
//   - F3: replaying the tape the writer wrote does not reproduce the live
//     tables;
//   - F8: the lifecycle fold starts writing the table with the setting on;
//   - a refused write is stamped and reaches the tape, or no longer reaches
//     its caller (the preflight the acknowledged path runs is skipped);
//   - the counter tape_ops_by_table or the writes flag shows on the
//     observability surface while the setting is off (that surface must be
//     byte-identical to deploy 1's), or does not count stamped ops when on.

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// tapeOps returns every op of every tape event of the harness, in order.
func (h *tapeHarness) tapeOps() []computerevent.ProjectionOp {
	h.t.Helper()
	var out []computerevent.ProjectionOp
	for _, rec := range h.tape.snapshot() {
		if rec.Kind != computerevent.EventProjectionBatchRecorded {
			continue
		}
		out = append(out, h.tape.batchOps(h.t, rec)...)
	}
	return out
}

// opEventKind decodes the event kind of a choir.event object op, or "" for
// any other op.
func opEventKind(t *testing.T, op computerevent.ProjectionOp) (types.EventKind, bool) {
	t.Helper()
	if op.Kind != computerevent.ProjectionOpObject {
		return "", false
	}
	var obj objectgraph.Object
	if err := json.Unmarshal(op.Body, &obj); err != nil {
		t.Fatal(err)
	}
	if obj.ObjectKind != ogKindEvent {
		return "", false
	}
	var rec types.EventRecord
	if err := json.Unmarshal(obj.Body, &rec); err != nil {
		t.Fatal(err)
	}
	return rec.Kind, true
}

// W-off: the setting off (never set, or set and cleared) writes the tape
// deploy 1 writes, op for op and byte for byte, with no Table anywhere.
func TestEventTableWriterOffIsDeploy1(t *testing.T) {
	never := newTapeHarness(t)
	scriptedWorkload(t, never)

	cleared := newTapeHarness(t)
	cleared.store.SetEventTableWrites(true)
	cleared.store.SetEventTableWrites(false)
	scriptedWorkload(t, cleared)

	a, b := normalizedTape(t, never), normalizedTape(t, cleared)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("the cleared setting changed the tape:\nnever   %+v\ncleared %+v", a, b)
	}
	for _, h := range []*tapeHarness{never, cleared} {
		for _, op := range h.tapeOps() {
			if op.Table != "" {
				t.Fatalf("an op is stamped %q with the setting off", op.Table)
			}
		}
		if n := tableCount(t, h.store, "choir_events", ""); n != 0 {
			t.Fatalf("choir_events has %d rows with the setting off", n)
		}
		stats := h.store.TelemetryStats()
		if stats.EventTableWrites || len(stats.TapeOpsByTable) != 0 {
			t.Fatalf("observability shows the writer with the setting off: writes=%v by_table=%v", stats.EventTableWrites, stats.TapeOpsByTable)
		}
		raw, err := json.Marshal(stats)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "event_table") || strings.Contains(string(raw), "by_table") {
			t.Fatalf("the observability JSON changed with the setting off: %s", raw)
		}
	}
}

// W-body and F9: with the setting on the tape differs from the setting off
// only by Table, and only on ops of the storage class.
func TestEventTableWriterOnDiffersOnlyByTheStamp(t *testing.T) {
	off := newTapeHarness(t)
	scriptedWorkload(t, off)
	on := newTapeHarness(t)
	on.store.SetEventTableWrites(true)
	scriptedWorkload(t, on)

	if a, b := normalizedTape(t, off), normalizedTape(t, on); !reflect.DeepEqual(a, b) {
		t.Fatalf("with the setting on an op body, kind, authority or count changed:\noff %+v\non  %+v", a, b)
	}
	stamped := 0
	for _, op := range on.tapeOps() {
		kind, isEvent := opEventKind(t, op)
		want := ""
		if isEvent && kind.Storage() == types.StorageEventTable {
			want = choirEventsTable
		}
		if op.Table != want {
			t.Fatalf("op of kind %q (event %v) stamped %q, want %q", kind, isEvent, op.Table, want)
		}
		if op.Table != "" {
			stamped++
		}
	}
	if stamped != 6 {
		t.Fatalf("stamped %d ops, want the workload's 6 batched events", stamped)
	}
	if n := tableCount(t, on.store, "choir_events", ""); n != stamped {
		t.Fatalf("choir_events rows = %d, want %d", n, stamped)
	}
	stats := on.store.TelemetryStats()
	if !stats.EventTableWrites || stats.TapeOpsByTable[choirEventsTable] != int64(stamped) {
		t.Fatalf("observability: writes=%v by_table=%v, want on and %d", stats.EventTableWrites, stats.TapeOpsByTable, stamped)
	}
	// The reads are the same whichever table holds the event.
	for _, run := range []string{"run-a", "run-b", ""} {
		a, b := eventIDs(mustEvents(t)(off.store.ListEventsAfter(context.Background(), run, 0, 100))), eventIDs(mustEvents(t)(on.store.ListEventsAfter(context.Background(), run, 0, 100)))
		if a != b {
			t.Fatalf("run %q reads %s with the setting off and %s on", run, a, b)
		}
	}
}

// F9, the forcing test: one event of every kind with the setting on. Only
// the storage class is stamped; every other kind stays in og_objects.
func TestEventTableWriterStampsOnlyTheStorageClass(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	h.store.SetEventTableWrites(true)
	kinds := append(types.EventKinds(), types.EventKind("loop.future"))
	for i, kind := range kinds {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("k-%d", i), "run-k", "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("append %s: %v", kind, err)
		}
	}
	want := 0
	for _, kind := range kinds {
		if kind.Storage() == types.StorageEventTable {
			want++
		}
	}
	if want != 10 {
		t.Fatalf("fixture: the storage class names %d kinds, want the ten batched kinds", want)
	}
	for _, op := range h.tapeOps() {
		kind, isEvent := opEventKind(t, op)
		if stamped := op.Table == choirEventsTable; stamped != (isEvent && kind.Storage() == types.StorageEventTable) {
			t.Fatalf("event kind %q stamped=%v", kind, stamped)
		}
	}
	if n := tableCount(t, h.store, "choir_events", ""); n != want {
		t.Fatalf("choir_events rows = %d, want %d", n, want)
	}
	if n := tableCount(t, h.store, "og_objects", "object_kind = 'choir.event'"); n != len(kinds)-want {
		t.Fatalf("og_objects events = %d, want %d", n, len(kinds)-want)
	}
}

// F9: a store with no tape never stamps, whatever the setting says: it has no
// tape position to key a row on (W6).
func TestEventTableWriterUnboundStoreNeverStamps(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	s.SetEventTableWrites(true)
	if err := s.AppendEvent(ctx, eventTableRecord("u-1", "run-u", "owner-1", "traj-u", types.EventToolResult, 0, 0, eventTableBase)); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, s, "choir_events", ""); n != 0 {
		t.Fatalf("an unbound store wrote %d choir_events rows", n)
	}
	if n := tableCount(t, s, "og_objects", "object_kind = 'choir.event'"); n != 1 {
		t.Fatalf("an unbound store's event is not in og_objects (%d rows)", n)
	}
}

// F9, W2: the sealer stamps what it seals with the setting on. The stamp is
// decided when the event is appended, so a record queued with the setting on
// is stamped even if the setting goes off before its seal, and the reverse.
func TestEventTableWriterSealerStamps(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	sl := h.enableBatching(time.Hour)
	h.store.SetEventTableWrites(true)
	for i := 0; i < 3; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("on-%d", i), "run-s", "owner-1", types.EventToolResult, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	h.store.SetEventTableWrites(false)
	for i := 0; i < 2; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("off-%d", i), "run-s", "owner-1", types.EventToolResult, telemetryBase.Add(time.Duration(10+i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	sl.drain()
	var sealedStamped, sealedPlain int
	for _, rec := range h.tape.snapshot() {
		if rec.Authority != telemetryProjectionAuthority {
			continue
		}
		for _, op := range h.tape.batchOps(t, rec) {
			if err := recheckTelemetryOp(op); err != nil {
				t.Fatalf("a sealed op fails the recheck: %v", err)
			}
			var obj objectgraph.Object
			if err := json.Unmarshal(op.Body, &obj); err != nil {
				t.Fatal(err)
			}
			var ev types.EventRecord
			if err := json.Unmarshal(obj.Body, &ev); err != nil {
				t.Fatal(err)
			}
			switch {
			case strings.HasPrefix(ev.EventID, "on-") && op.Table == choirEventsTable:
				sealedStamped++
			case strings.HasPrefix(ev.EventID, "off-") && op.Table == "":
				sealedPlain++
			default:
				t.Fatalf("sealed %s with Table %q", ev.EventID, op.Table)
			}
		}
	}
	if sealedStamped != 3 || sealedPlain != 2 {
		t.Fatalf("sealed %d stamped and %d plain, want 3 and 2", sealedStamped, sealedPlain)
	}
	if n := tableCount(t, h.store, "choir_events", ""); n != 3 {
		t.Fatalf("choir_events rows = %d, want 3", n)
	}
	if got := eventIDs(h.storedEvents("run-s")); strings.Count(got, ",") != 4 {
		t.Fatalf("run-s reads %s, want all five events", got)
	}
}

// A write the acknowledged path refuses is refused with the setting on too,
// before anything reaches the tape, and the caller sees the refusal.
func TestEventTableWriterRunsThePreflight(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	h.store.SetEventTableWrites(true)
	before := len(h.tape.snapshot())
	bad := h.event("bad-1", "run-bad", "owner-1", types.EventToolResult, telemetryBase)
	bad.RunID = strings.Repeat("r", 300) // longer than the run_id columns hold
	if err := h.store.AppendEvent(ctx, bad); err == nil {
		t.Fatal("an event no table can hold was accepted with the setting on")
	}
	if after := len(h.tape.snapshot()); after != before {
		t.Fatalf("a refused event reached the tape: %d events, was %d", after, before)
	}
	if n := tableCount(t, h.store, "choir_events", ""); n != 0 {
		t.Fatalf("a refused event wrote %d rows", n)
	}
}

// F1 through the writer: the setting on, batched and acknowledged events
// across runs and owners, reopens (each a new process whose marks start
// empty), batching, and parallel appenders: no number is handed out twice.
func TestEventTableWriterHandsOutNoDuplicateNumbers(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	h.store.SetEventTableWrites(true)
	kinds := []types.EventKind{types.EventToolResult, types.EventRunStarted, types.EventRunDelta, types.EventChannelMessage, types.EventDeskCell}
	n := 0
	appendSome := func(count int) {
		t.Helper()
		for i := 0; i < count; i++ {
			n++
			run := []string{"run-1", "run-2", "run-3"}[n%3]
			owner := []string{"owner-1", "owner-2"}[n%2]
			if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("d-%d", n), run, owner, kinds[n%len(kinds)], telemetryBase.Add(time.Duration(n)*time.Second))); err != nil {
				t.Fatal(err)
			}
		}
	}
	appendSome(30)
	h.reopen()
	h.store.SetEventTableWrites(true)
	appendSome(30)
	assertNoDuplicateNumbers(t, h.store)

	// A reopen with the setting off: new events go to og_objects and still
	// number above the ones in choir_events.
	h.reopen()
	appendSome(10)
	assertNoDuplicateNumbers(t, h.store)

	h.reopen()
	h.store.SetEventTableWrites(true)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 8; i++ {
				rec := h.event(fmt.Sprintf("p-%d-%d", g, i), []string{"run-1", "run-2"}[g%2], "owner-1", kinds[(g+i)%len(kinds)], telemetryBase.Add(time.Hour))
				if err := h.store.AppendEvent(ctx, rec); err != nil {
					errs <- err
				}
			}
		}(g)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	assertNoDuplicateNumbers(t, h.store)

	h.reopen()
	h.store.SetEventTableWrites(true)
	sl := h.enableBatching(time.Hour)
	appendSome(20)
	sl.drain()
	assertNoDuplicateNumbers(t, h.store)
	if tableCount(t, h.store, "choir_events", "") == 0 || tableCount(t, h.store, "og_objects", "object_kind = 'choir.event'") == 0 {
		t.Fatal("fixture: both tables should hold events")
	}
}

// F2 through the writer: one script appended into a store with the setting
// off, one with it on, and one where it toggles mid-run. Every union read is
// identical, numbers included (the writer allocates them).
func TestEventTableWriterUnionReadsMatch(t *testing.T) {
	script := eventTableScript(60)
	run := func(toggle func(i int) bool) *Store {
		h := newTapeHarness(t)
		for i, rec := range script {
			h.store.SetEventTableWrites(toggle(i))
			cp := *rec
			cp.Seq, cp.StreamSeq = 0, 0
			if err := h.store.AppendEvent(context.Background(), &cp); err != nil {
				t.Fatal(err)
			}
			if cp.Seq != rec.Seq || cp.StreamSeq != rec.StreamSeq {
				t.Fatalf("event %s numbered %d/%d, the script says %d/%d", rec.EventID, cp.Seq, cp.StreamSeq, rec.Seq, rec.StreamSeq)
			}
		}
		return h.store
	}
	off := run(func(int) bool { return false })
	on := run(func(int) bool { return true })
	toggled := run(func(i int) bool { return i >= 20 && i < 40 })
	if tableCount(t, on, "choir_events", "") == 0 || tableCount(t, toggled, "choir_events", "") == 0 || tableCount(t, off, "choir_events", "") != 0 {
		t.Fatal("fixture: the writer did not follow the setting")
	}
	want := eventTableReads(t, off)
	assertSameReads(t, "setting on", want, eventTableReads(t, on))
	assertSameReads(t, "setting toggled", want, eventTableReads(t, toggled))
}

// F3 through the writer: the tape the writer recorded, replayed into a fresh
// store, puts every event in the same table at the same position.
func TestEventTableWriterReplayEqualsLive(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	for i, rec := range eventTableScript(30) {
		h.store.SetEventTableWrites(i%10 < 6)
		cp := *rec
		cp.Seq, cp.StreamSeq = 0, 0
		if err := h.store.AppendEvent(ctx, &cp); err != nil {
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
	replay, _ := vocabUpcastOpenStore(t, "writer-replay")
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))

	rows := func(s *Store) []string {
		t.Helper()
		var out []string
		r, err := s.db.Query(`SELECT canonical_id, op_index, run_id, seq, stream_seq FROM choir_events ORDER BY tape_seq, op_index`)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		for r.Next() {
			var id, run string
			var idx, seq, stream int64
			if err := r.Scan(&id, &idx, &run, &seq, &stream); err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprintf("%s@%d %s#%d/%d", id, idx, run, seq, stream))
		}
		return out
	}
	live, replayed := rows(h.store), rows(replay)
	if len(live) == 0 || !reflect.DeepEqual(live, replayed) {
		t.Fatalf("choir_events differ:\nlive   %v\nreplay %v", live, replayed)
	}
	if a, b := tableCount(t, h.store, "og_objects", "object_kind = 'choir.event'"), tableCount(t, replay, "og_objects", "object_kind = 'choir.event'"); a != b || a == 0 {
		t.Fatalf("og_objects events: live %d, replay %d", a, b)
	}
	for _, runID := range []string{"run-1", "run-2", "run-3"} {
		a := eventIDs(mustEvents(t)(h.store.ListEventsAfter(ctx, runID, 0, 1000)))
		b := eventIDs(mustEvents(t)(replay.ListEventsAfter(ctx, runID, 0, 1000)))
		if a != b {
			t.Fatalf("%s reads %s live and %s replayed", runID, a, b)
		}
	}
}

// F8 with the setting on: the lifecycle fold's event stays in og_objects,
// where its absent-condition reads, and a second identical fold conflicts.
func TestEventTableWriterLeavesTheLifecycleFoldAlone(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	h.store.SetEventTableWrites(true)
	start := lifecycleStartFixture()
	if _, err := h.store.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	run := lifecycleRunFixture(start, "run-fold-on", types.RunRunning)
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID:    "project-fold-on:" + start.TrajectoryID,
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: run,
	}
	project.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(project)
	if _, err := h.store.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project activation: %v", err)
	}
	run.State = types.RunCompleted
	run.UpdatedAt = run.UpdatedAt.Add(time.Second)
	ev := &types.EventRecord{
		EventID: "ev-fold-on", RunID: run.RunID, AgentID: run.AgentID,
		OwnerID: run.OwnerID, TrajectoryID: run.TrajectoryID,
		Kind: types.EventRunCompleted, Timestamp: run.UpdatedAt,
	}
	if err := h.store.UpdateRunWithEvent(ctx, run, ev); err != nil {
		t.Fatalf("update run with event: %v", err)
	}
	if n := tableCount(t, h.store, "choir_events", ""); n != 0 {
		t.Fatalf("the lifecycle fold wrote %d choir_events rows with the setting on", n)
	}
	if n := tableCount(t, h.store, "og_objects", "object_kind = 'choir.event' AND JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.event_id')) = 'ev-fold-on'"); n != 1 {
		t.Fatalf("the folded event is not in og_objects (%d rows)", n)
	}
	for _, op := range h.tapeOps() {
		if op.Table != "" {
			kind, _ := opEventKind(t, op)
			t.Fatalf("the lifecycle path stamped an op of kind %q", kind)
		}
	}
}

// The type-based stamp guard (event_table_stamp_guard_test.go) allows one
// file, event_table.go. Inside it, only eventObjectOp may stamp, and it
// must: the writer's single stamping site, behind the switch.
func TestEventTableOnlyEventObjectOpStamps(t *testing.T) {
	root := repoRoot(t)
	c := newStampChecker(t)
	for _, p := range listExportData(t) {
		if p.ImportPath != "github.com/yusefmosiah/go-choir/internal/store" {
			continue
		}
		var files []*ast.File
		for _, name := range p.GoFiles {
			file, err := parser.ParseFile(c.fset, filepath.Join(p.Dir, name), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			files = append(files, file)
		}
		info, errs := c.check(p.ImportPath, files)
		if len(errs) > 0 {
			t.Fatalf("type errors: %v", errs)
		}
		var span [2]int
		for _, file := range files {
			if filepath.Base(c.fset.Position(file.Pos()).Filename) != "event_table.go" {
				continue
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "eventObjectOp" {
					span = [2]int{c.fset.Position(fn.Pos()).Line, c.fset.Position(fn.End()).Line}
				}
			}
		}
		if span[0] == 0 {
			t.Fatal("eventObjectOp is gone from event_table.go")
		}
		stamps := 0
		for _, pos := range c.tableStamps(files, info) {
			rel, _ := filepath.Rel(root, pos.Filename)
			if filepath.ToSlash(rel) != "internal/store/event_table.go" {
				continue
			}
			if pos.Line < span[0] || pos.Line > span[1] {
				t.Fatalf("event_table.go:%d stamps ProjectionOp.Table outside eventObjectOp (lines %d-%d)", pos.Line, span[0], span[1])
			}
			stamps++
		}
		if stamps == 0 {
			t.Fatal("eventObjectOp no longer stamps; update the stamp guard's allowlist")
		}
		return
	}
	t.Fatal("internal/store was not listed")
}
