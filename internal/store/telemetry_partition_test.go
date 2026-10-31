package store

// Telemetry in time-partitioned files (telemetry_partition.go,
// docs/design/one-tape-durability-2026-10-31.md section 6c). Ways it can
// fail, written before the code; each test names the ones it catches:
//
//   - T1 determinism: a row's partition comes from the wall clock, or from
//     anything but the op on the tape, so live apply and replay put it in
//     different files (or replay puts it in og_objects);
//   - T2 readers: a reader answers differently over partitions than over the
//     object graph (a missed half, a cut before the merge, a cursor that
//     skips a partition row);
//   - T3 retirement: a retire op over partitions keeps or drops other rows
//     than the old row delete would, including the keep_cited rows a run
//     acceptance cites; or a whole-day retire leaves the file live;
//   - T4 crash between the partition commit and the state commit: a row
//     whose event the state file has not applied stays readable after the
//     restart, or is lost when the event is finalized again;
//   - T5 crash mid-retire: a retired partition is read again after a restart
//     because its file is still there; a deletion cut short leaves a WAL that
//     a later file of the same name would adopt; a deletion runs while a
//     compact copy whose head is below the retirement is staged;
//   - T6 a late row of a retired day is invisible (written to the retired
//     file);
//   - T7 the dry run creates or writes a partition;
//   - T8 the job: retires a partition before its whole day is past the
//     window, or never retires it, or appends ops on an idle run;
//   - T9 the snapshot hold does not keep a file in place;
//   - T10 the writer stamps before the store floor is written (an older
//     binary would open a store whose partitions it cannot read);
//   - T11 a sequence or stream number is handed out twice because a seed
//     skipped the partitions.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// partitionHarness is a tape harness whose writer stamps tool.result for the
// partitions (on) or does not (off).
func partitionHarness(t *testing.T, on bool) *tapeHarness {
	t.Helper()
	// One directory per harness: a test may hold two.
	dir := filepath.Join(storeTestDir(), t.Name(), map[bool]string{true: "partitions", false: "object-graph"}[on])
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "runtime.db")
	h := &tapeHarness{t: t, path: path, computerID: "computer-telemetry", tape: newRecordingTape(t)}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	prepareGenesis(t, s, h.computerID, "genesis-telemetry")
	h.bind()
	t.Cleanup(func() { _ = h.store.Close() })
	waitLeaseReady(t, h.store)
	if err := h.store.SetTelemetryPartitionWrites(on); err != nil {
		t.Fatal(err)
	}
	if on && !h.store.TelemetryPartitionWritesOn() {
		t.Fatal("the partition writer is not on")
	}
	return h
}

func partitionSeed(t *testing.T, h *tapeHarness, script []retireEvent) {
	t.Helper()
	for _, e := range script {
		if err := h.store.AppendEvent(context.Background(), e.record()); err != nil {
			t.Fatalf("append %s: %v", e.id, err)
		}
	}
}

func partitionFiles(t *testing.T, s *Store) []string {
	t.Helper()
	entries, err := os.ReadDir(s.partitions.dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".db") {
			out = append(out, strings.TrimSuffix(e.Name(), ".db"))
		}
	}
	sort.Strings(out)
	return out
}

// partitionContent is every live partition's rows, by name, as
// "canonical_id|updated_at|body".
func partitionContent(t *testing.T, s *Store) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, p := range s.partitions.live() {
		rows, err := readStoredRows(context.Background(), p.db.Read, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			out[p.name] = append(out[p.name], r.row.canonicalID+"|"+r.row.updatedAt.UTC().Format(time.RFC3339Nano)+"|"+string(r.row.body))
		}
		sort.Strings(out[p.name])
	}
	return out
}

func partitionRowCount(t *testing.T, s *Store, name string) int {
	t.Helper()
	p, err := s.partitions.get(context.Background(), name)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := p.db.Read.QueryRow(`SELECT COUNT(*) FROM choir_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// readerIDs is a run's events as the product reads them.
func readerIDs(t *testing.T, s *Store) []string {
	t.Helper()
	events, err := s.ListEventsAfter(context.Background(), "run-1", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(events))
	for _, e := range events {
		ids = append(ids, e.EventID)
	}
	return ids
}

func tapeTables(t *testing.T, h *tapeHarness) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, rec := range h.tape.snapshot() {
		for _, op := range h.tape.batchOps(t, rec) {
			if op.Kind != computerevent.ProjectionOpObject {
				continue
			}
			var obj objectgraph.Object
			if err := json.Unmarshal(op.Body, &obj); err != nil {
				t.Fatal(err)
			}
			var ev types.EventRecord
			if json.Unmarshal(obj.Body, &ev) == nil && ev.EventID != "" {
				out[ev.EventID] = op.Table
			}
		}
	}
	return out
}

var partitionDay1 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// T1: the partition is the op's: the class of its kind and the UTC day of
// the event's own time, on the tape; replay of that tape puts every row in
// the same file.
func TestTelemetryPartitionLandsInTheDayOfTheEventAndReplaysThere(t *testing.T) {
	h := partitionHarness(t, true)
	day3Last := time.Date(2026, 10, 3, 23, 59, 59, 900_000_000, time.UTC)
	partitionSeed(t, h, []retireEvent{
		{id: "res-1", kind: types.EventToolResult, at: partitionDay1},
		{id: "res-2", kind: types.EventToolResult, at: partitionDay1.Add(time.Hour)},
		{id: "res-3", kind: types.EventToolResult, at: day3Last},
		{id: "cell-1", kind: types.EventDeskCell, at: partitionDay1},
	})
	tables := tapeTables(t, h)
	if tables["res-1"] != telemetryPartitionStamp || tables["res-3"] != telemetryPartitionStamp || tables["cell-1"] != "" {
		t.Fatalf("stamps on the tape: %v", tables)
	}
	if got, want := partitionFiles(t, h.store), []string{"tool_result-20261001", "tool_result-20261003"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partition files %v, want %v (T1)", got, want)
	}
	if n := tableCount(t, h.store, "og_objects", "object_kind = 'choir.event' AND json_extract(metadata, '$.kind') = 'tool.result'"); n != 0 {
		t.Fatalf("%d tool results in og_objects", n)
	}
	if got, want := readerIDs(t, h.store), []string{"res-1", "res-2", "res-3", "cell-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reader %v, want %v", got, want)
	}

	var batches [][]computerevent.ProjectionOp
	for _, rec := range h.tape.snapshot() {
		if rec.Kind == computerevent.EventProjectionBatchRecorded {
			batches = append(batches, h.tape.batchOps(t, rec))
		}
	}
	tape := vocabUpcastTapeFromBatches(t, batches)
	replay, _ := vocabUpcastOpenStore(t, "partition-replay")
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))
	if live, replayed := partitionContent(t, h.store), partitionContent(t, replay); len(live) != 2 || !reflect.DeepEqual(live, replayed) {
		t.Fatalf("partitions differ (T1):\nlive   %v\nreplay %v", live, replayed)
	}
	if got := readerIDs(t, replay); !reflect.DeepEqual(got, readerIDs(t, h.store)) {
		t.Fatalf("replay reader %v", got)
	}
	// A replay into a fresh workspace creates partitions after Open's floor
	// check: the first file floors it (T10).
	if floor, err := replay.StoreFloor(); err != nil || floor < telemetryPartitionFloorEpoch {
		t.Fatalf("the replayed store's floor is %d (err %v) (T10)", floor, err)
	}
}

func partitionReaderScript() []retireEvent {
	var script []retireEvent
	for i, id := range []string{"a", "b", "c", "d", "e", "f"} {
		kind := types.EventToolResult
		if i%3 == 2 {
			kind = types.EventToolInvoked
		}
		script = append(script, retireEvent{id: id, kind: kind, at: partitionDay1.Add(time.Duration(i) * 7 * time.Hour)})
	}
	return script
}

type readerAnswers struct {
	RunAll, RunAfter, Owner, OwnerAfter, Trajectory []string
}

func answersOf(t *testing.T, s *Store) readerAnswers {
	t.Helper()
	ctx := context.Background()
	key := func(events []types.EventRecord, err error) []string {
		if err != nil {
			t.Fatal(err)
		}
		out := make([]string, 0, len(events))
		for _, e := range events {
			out = append(out, e.EventID+"/"+jsonInt(e.Seq)+"/"+jsonInt(e.StreamSeq))
		}
		return out
	}
	var a readerAnswers
	a.RunAll = key(s.ListEventsAfter(ctx, "run-1", 0, 1000))
	a.RunAfter = key(s.ListEventsAfter(ctx, "run-1", 2, 2))
	a.Owner = key(s.ListEventsByOwner(ctx, "owner-1", 4))
	runAll, _ := s.ListEventsAfter(ctx, "run-1", 0, 1000)
	a.OwnerAfter = key(s.ListEventsByOwnerAfter(ctx, "owner-1", runAll[1].StreamSeq, 3))
	a.Trajectory = key(s.ListEventsByTrajectory(ctx, "owner-1", "traj-1", 100))
	return a
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// T2, T11: every reader answers over partitions as it does over the object
// graph, numbers included (the seeds read the partitions).
func TestTelemetryPartitionReadersAnswerAsTheObjectGraph(t *testing.T) {
	og, parts := partitionHarness(t, false), partitionHarness(t, true)
	partitionSeed(t, og, partitionReaderScript())
	partitionSeed(t, parts, partitionReaderScript())
	if len(partitionFiles(t, parts.store)) == 0 || len(partitionFiles(t, og.store)) != 0 {
		t.Fatalf("the harnesses do not differ in where tool results land: %v / %v", partitionFiles(t, parts.store), partitionFiles(t, og.store))
	}
	want, got := answersOf(t, og.store), answersOf(t, parts.store)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("readers differ (T2):\nobject graph %+v\npartitions   %+v", want, got)
	}
	// T11: a restart forgets the in-memory numbers; the seeds read the
	// partitions, so the next event of the run continues the run's numbers.
	parts.reopen()
	waitLeaseReady(t, parts.store)
	if err := parts.store.SetTelemetryPartitionWrites(true); err != nil {
		t.Fatal(err)
	}
	next := retireEvent{id: "g", kind: types.EventToolResult, at: partitionDay1.Add(50 * time.Hour)}.record()
	if err := parts.store.AppendEvent(context.Background(), next); err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateNumbersAll(t, parts.store)
}

// assertNoDuplicateNumbersAll is assertNoDuplicateNumbers over the readers,
// which include the partitions.
func assertNoDuplicateNumbersAll(t *testing.T, s *Store) {
	t.Helper()
	events, err := s.ListEventsAfter(context.Background(), "run-1", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	seqs, streams := map[int64]string{}, map[int64]string{}
	for _, e := range events {
		if prev, dup := seqs[e.Seq]; dup {
			t.Fatalf("seq %d handed out twice: %s and %s (T11)", e.Seq, prev, e.EventID)
		}
		seqs[e.Seq] = e.EventID
		if prev, dup := streams[e.StreamSeq]; dup {
			t.Fatalf("stream_seq %d handed out twice: %s and %s (T11)", e.StreamSeq, prev, e.EventID)
		}
		streams[e.StreamSeq] = e.EventID
	}
}

func citeResults(t *testing.T, s *Store, ids ...string) {
	t.Helper()
	refs := []types.RunAcceptanceEvidenceRef{{RefID: "run:run-1", Kind: "run", RunID: "run-1"}}
	for _, id := range ids {
		refs = append(refs, types.RunAcceptanceEvidenceRef{RefID: "event:" + id, Kind: "tool.result", EventID: id})
	}
	if _, err := s.UpsertRunAcceptance(context.Background(), types.RunAcceptanceRecord{
		OwnerID: "owner-1", TargetMissionID: "mission-1", TrajectoryID: "traj-1", EvidenceRefs: refs,
	}); err != nil {
		t.Fatal(err)
	}
}

// T3: the same retire ops leave the same answers over partitions as the row
// delete leaves over the object graph, keep_cited included; a day the op
// covers whole is retired and its file deleted, its cited rows still read.
func TestTelemetryPartitionRetireEqualsTheRowDelete(t *testing.T) {
	ctx := context.Background()
	day := 24 * time.Hour
	d1, d2, d3, d4 := partitionDay1, partitionDay1.Add(day), partitionDay1.Add(2*day), partitionDay1.Add(3*day)
	script := []retireEvent{
		{id: "d1-early", kind: types.EventToolResult, at: d1.Add(-10 * time.Hour)},
		{id: "d1-noon", kind: types.EventToolResult, at: d1},
		{id: "d1-cited", kind: types.EventToolResult, at: d1.Add(time.Minute)},
		{id: "d2-a", kind: types.EventToolResult, at: d2},
		{id: "d2-cited", kind: types.EventToolResult, at: d2.Add(time.Hour)},
		{id: "d3-a", kind: types.EventToolResult, at: d3},
		{id: "d3-cited", kind: types.EventToolResult, at: d3.Add(time.Hour)},
		{id: "d4-a", kind: types.EventToolResult, at: d4},
		{id: "d4-inv", kind: types.EventToolInvoked, at: d4},
	}
	dayOf := func(t time.Time) time.Time { return telemetryPartitionDayOf(t) }
	ops := [][]computerevent.ProjectionOp{
		// A part of day 1, keep_cited: a row delete inside the partition.
		{retireOp(t, func() computerevent.ObjectRetireProjection {
			p := retireP([]string{"tool.result"}, dayOf(d1), d1.Add(30*time.Minute), "")
			p.KeepCited = computerevent.ObjectRetireKeepRunAcceptance
			return p
		}())},
		// Day 2 whole, keep_cited: the partition is retired, its cited row kept.
		{retireOp(t, func() computerevent.ObjectRetireProjection {
			p := retireP([]string{"tool.result"}, dayOf(d2), dayOf(d2).Add(day), "")
			p.KeepCited = computerevent.ObjectRetireKeepRunAcceptance
			return p
		}())},
		// Day 3 whole, no keep_cited: every row goes, cited or not.
		{retireOp(t, retireP([]string{"tool.result", "tool.invoked"}, dayOf(d3), dayOf(d3).Add(day), ""))},
		// Day 4 whole, phased only: a tool result has no phase, nothing goes.
		{retireOp(t, retireP([]string{"tool.result"}, dayOf(d4), dayOf(d4).Add(day), computerevent.ObjectRetirePhaseSome))},
		// The rest of day 1, keep_cited, as a whole-day op after the part.
		{retireOp(t, func() computerevent.ObjectRetireProjection {
			p := retireP([]string{"tool.result"}, dayOf(d1), dayOf(d1).Add(day), "")
			p.KeepCited = computerevent.ObjectRetireKeepRunAcceptance
			return p
		}())},
	}
	og, parts := partitionHarness(t, false), partitionHarness(t, true)
	for _, h := range []*tapeHarness{og, parts} {
		partitionSeed(t, h, script)
		citeResults(t, h.store, "d1-cited", "d2-cited", "d3-cited")
	}
	if !reflect.DeepEqual(readerIDs(t, og.store), readerIDs(t, parts.store)) {
		t.Fatal("the harnesses differ before any retirement")
	}
	for i, batch := range ops {
		for _, h := range []*tapeHarness{og, parts} {
			if err := h.store.AppendProjectionOps(ctx, batch); err != nil {
				t.Fatalf("op %d: %v", i, err)
			}
		}
		if want, got := readerIDs(t, og.store), readerIDs(t, parts.store); !reflect.DeepEqual(want, got) {
			t.Fatalf("after op %d (T3):\nrow delete %v\npartitions %v", i, want, got)
		}
	}
	want := []string{"d1-cited", "d2-cited", "d4-a", "d4-inv"}
	if got := readerIDs(t, parts.store); !reflect.DeepEqual(got, want) {
		t.Fatalf("remaining %v, want %v", got, want)
	}
	// Days 1 to 3 are retired (day 1 by the last op, after its part).
	stats := parts.store.TelemetryPartitionStats()
	if stats.Retired != 3 {
		t.Fatalf("retired partitions: %+v, want days 1 to 3", stats)
	}
	if n := tableCount(t, parts.store, "choir_events", "event_kind = 'tool.result'"); n != 2 {
		t.Fatalf("%d cited rows moved to choir_events, want 2 (d1-cited, d2-cited)", n)
	}
	before := partitionFiles(t, parts.store)
	deleted, err := parts.store.GCTelemetryPartitions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 3 {
		t.Fatalf("deleted %d files of %v, want 3", deleted, before)
	}
	if got, want := partitionFiles(t, parts.store), []string{"tool_result-20261004"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files after the deletion %v, want %v", got, want)
	}
	if got := readerIDs(t, parts.store); !reflect.DeepEqual(got, want) {
		t.Fatalf("the deletion changed what is read: %v", got)
	}
	// And a restart reads the same.
	parts.reopen()
	if got := readerIDs(t, parts.store); !reflect.DeepEqual(got, want) {
		t.Fatalf("after a restart: %v", got)
	}
}

// T4: a crash after the partitions commit and before the state file does.
// In this harness the platform's head is the local projection's, so the
// event never became canonical: the restart must drop the row above the head
// (a phantom otherwise), and the recovery discards the prepared event. A
// canonical event finalized again writes its rows once (the upsert on the
// tape position).
func TestTelemetryPartitionCrashBetweenThePartitionAndTheStateCommit(t *testing.T) {
	ctx := context.Background()
	h := partitionHarness(t, true)
	partitionSeed(t, h, []retireEvent{{id: "kept", kind: types.EventToolResult, at: partitionDay1}})
	var crash atomic.Bool
	crash.Store(true)
	partitionFlushedHook = func(uint64) error {
		if crash.Load() {
			return errSimulatedCrash
		}
		return nil
	}
	t.Cleanup(func() { partitionFlushedHook = nil })
	if err := h.store.AppendEvent(ctx, retireEvent{id: "torn", kind: types.EventToolResult, at: partitionDay1.Add(time.Minute)}.record()); err == nil {
		t.Fatal("the simulated crash did not fail the append")
	}
	crash.Store(false)
	name := telemetryPartitionName("tool_result", partitionDay1)
	if n := partitionRowCount(t, h.store, name); n != 2 {
		t.Fatalf("the partition holds %d rows before the restart, want 2 (the torn one above the head)", n)
	}
	h.reopen()
	if n := partitionRowCount(t, h.store, name); n != 1 {
		t.Fatalf("after the restart the partition holds %d rows, want 1 (T4)", n)
	}
	if err := h.appender.RecoverPrepared(ctx); err != nil {
		t.Fatal(err)
	}
	if got := readerIDs(t, h.store); !reflect.DeepEqual(got, []string{"kept"}) {
		t.Fatalf("reader after the restart %v: a row of an event the state file never applied (T4)", got)
	}
	waitLeaseReady(t, h.store)
	if err := h.store.SetTelemetryPartitionWrites(true); err != nil {
		t.Fatal(err)
	}
	partitionSeed(t, h, []retireEvent{{id: "again", kind: types.EventToolResult, at: partitionDay1.Add(time.Minute)}})
	if got := readerIDs(t, h.store); !reflect.DeepEqual(got, []string{"kept", "again"}) {
		t.Fatalf("reader %v", got)
	}

	// Finalized twice at one position: one row.
	s, _ := vocabUpcastOpenStore(t, "partition-refinalize")
	op := eventTableOp(t, retireEvent{id: "twice", kind: types.EventToolResult, at: partitionDay1}.record(), false)
	op.Table = telemetryPartitionStamp
	projectAt(t, s, 2, op)
	projectAt(t, s, 2, op)
	if n := partitionRowCount(t, s, name); n != 1 {
		t.Fatalf("a re-finalized event left %d rows (T4)", n)
	}
}

var errSimulatedCrash = &simulatedCrash{}

type simulatedCrash struct{}

func (*simulatedCrash) Error() string { return "simulated crash between the partition and the state commit" }

// T5: a retired partition whose file outlived a crash is not read again; a
// WAL left by a cut-short deletion is removed; a staged compact copy holds
// the deletion.
func TestTelemetryPartitionRetiredFileOutlivesACrash(t *testing.T) {
	ctx := context.Background()
	h := partitionHarness(t, true)
	partitionSeed(t, h, []retireEvent{
		{id: "old", kind: types.EventToolResult, at: partitionDay1},
		{id: "new", kind: types.EventToolResult, at: partitionDay1.Add(48 * time.Hour)},
	})
	day := telemetryPartitionDayOf(partitionDay1)
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{retireOp(t, retireP([]string{"tool.result"}, day, day.Add(24*time.Hour), ""))}); err != nil {
		t.Fatal(err)
	}
	// The crash: no deletion ran.
	h.abandon()
	s, err := Open(h.path)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	h.bind()
	if got := readerIDs(t, h.store); !reflect.DeepEqual(got, []string{"new"}) {
		t.Fatalf("a retired partition was read after the restart: %v (T5)", got)
	}
	// A staged compact copy holds the deletion.
	staged := h.store.statePath + compactStagedSuffix
	if err := os.WriteFile(staged, []byte("copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := h.store.GCTelemetryPartitions(ctx); err != nil || n != 0 {
		t.Fatalf("deleted %d with a staged copy (err %v) (T5)", n, err)
	}
	if err := os.Remove(staged); err != nil {
		t.Fatal(err)
	}
	if n, err := h.store.GCTelemetryPartitions(ctx); err != nil || n != 1 {
		t.Fatalf("deleted %d (err %v), want 1", n, err)
	}
	// A WAL whose database is gone is removed at the next Open.
	stray := filepath.Join(h.store.partitions.dir, telemetryPartitionName("tool_result", partitionDay1)+".db-wal")
	if err := os.WriteFile(stray, []byte("torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.reopen()
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Fatalf("a stray WAL survived the open (T5): %v", err)
	}
}

// T6: a row of a day whose partition is retired lands in choir_events and
// is read.
func TestTelemetryPartitionLateRowOfARetiredDayIsRead(t *testing.T) {
	ctx := context.Background()
	h := partitionHarness(t, true)
	partitionSeed(t, h, []retireEvent{{id: "first", kind: types.EventToolResult, at: partitionDay1}})
	day := telemetryPartitionDayOf(partitionDay1)
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{retireOp(t, retireP([]string{"tool.result"}, day, day.Add(24*time.Hour), ""))}); err != nil {
		t.Fatal(err)
	}
	partitionSeed(t, h, []retireEvent{{id: "late", kind: types.EventToolResult, at: partitionDay1.Add(time.Hour)}})
	if got := readerIDs(t, h.store); !reflect.DeepEqual(got, []string{"late"}) {
		t.Fatalf("reader %v, want the late row (T6)", got)
	}
	if n := tableCount(t, h.store, "choir_events", ""); n != 1 {
		t.Fatalf("choir_events holds %d rows, want the late one", n)
	}
}

// T7: the dry run validates a partition op and writes nothing.
func TestTelemetryPartitionDryRunWritesNothing(t *testing.T) {
	ctx := context.Background()
	h := partitionHarness(t, true)
	op := eventTableOp(t, retireEvent{id: "dry", kind: types.EventToolResult, at: partitionDay1}.record(), false)
	op.Table = telemetryPartitionStamp
	batch := &computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: h.computerID, EventID: "dry", Ops: []computerevent.ProjectionOp{op},
	}
	if err := h.store.DryRunProjectionBatch(ctx, h.computerID, batch); err != nil {
		t.Fatal(err)
	}
	if files := partitionFiles(t, h.store); len(files) != 0 {
		t.Fatalf("the dry run created %v (T7)", files)
	}
	bad := op
	bad.Body = json.RawMessage(`{"object_kind":"choir.event"}`)
	batch.Ops = []computerevent.ProjectionOp{bad}
	if err := h.store.DryRunProjectionBatch(ctx, h.computerID, batch); err == nil {
		t.Fatal("the dry run accepted a partition op with no identity")
	}
}

// T8: the job retires a partition once its whole day is past the window, in
// one op, and an idle run appends nothing.
func TestRetireTelemetryRetiresWholePartitionDays(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := partitionHarness(t, true)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	partitionSeed(t, h, []retireEvent{
		{id: "res-9d", kind: types.EventToolResult, at: now.Add(-9 * day)},
		{id: "res-9d-cited", kind: types.EventToolResult, at: now.Add(-9*day + time.Hour)},
		{id: "res-7.4d", kind: types.EventToolResult, at: now.Add(-7*day - 10*time.Hour)},
		{id: "res-6d", kind: types.EventToolResult, at: now.Add(-6 * day)},
	})
	citeResults(t, h.store, "res-9d-cited")
	tapeBefore := len(h.tape.snapshot())
	if _, err := h.store.RetireTelemetry(ctx, now); err != nil {
		t.Fatal(err)
	}
	if got := len(h.tape.snapshot()) - tapeBefore; got != 1 {
		t.Fatalf("the job appended %d events, want one whole-day op", got)
	}
	// res-7.4d's day (10-23) ends after the cutoff (10-23 12:00): it stays.
	if got, want := readerIDs(t, h.store), []string{"res-9d-cited", "res-7.4d", "res-6d"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("remaining %v, want %v (T8)", got, want)
	}
	if _, err := h.store.GCTelemetryPartitions(ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := partitionFiles(t, h.store), []string{"tool_result-20261023", "tool_result-20261024"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files %v, want %v", got, want)
	}
	tapeBefore = len(h.tape.snapshot())
	if _, err := h.store.RetireTelemetry(ctx, now); err != nil {
		t.Fatal(err)
	}
	if got := len(h.tape.snapshot()) - tapeBefore; got != 0 {
		t.Fatalf("an idle run appended %d events (T8)", got)
	}
}

// T9: while a snapshot holds the partitions, a retired file is not deleted.
func TestTelemetryPartitionSnapshotHoldKeepsTheFile(t *testing.T) {
	ctx := context.Background()
	h := partitionHarness(t, true)
	partitionSeed(t, h, []retireEvent{{id: "old", kind: types.EventToolResult, at: partitionDay1}})
	day := telemetryPartitionDayOf(partitionDay1)
	if err := h.store.AppendProjectionOps(ctx, []computerevent.ProjectionOp{retireOp(t, retireP([]string{"tool.result"}, day, day.Add(24*time.Hour), ""))}); err != nil {
		t.Fatal(err)
	}
	release := h.store.HoldTelemetryPartitions()
	done := make(chan int, 1)
	go func() {
		n, _ := h.store.GCTelemetryPartitions(ctx)
		done <- n
	}()
	select {
	case n := <-done:
		release()
		t.Fatalf("the deletion ran under the hold (%d files) (T9)", n)
	case <-time.After(200 * time.Millisecond):
	}
	if files := partitionFiles(t, h.store); len(files) != 1 {
		t.Fatalf("files under the hold: %v", files)
	}
	release()
	if n := <-done; n != 1 {
		t.Fatalf("after the hold the deletion removed %d files, want 1", n)
	}
	// The head the snapshot reads calls a retired partition dead.
	tx, err := h.store.ReadDB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	files, err := h.store.TelemetrySnapshotFiles(ctx, tx)
	if err != nil || len(files) != 0 {
		t.Fatalf("snapshot files %v (err %v), want none", files, err)
	}
}

// T10: the writer floors the store before it stamps, and an unbound store
// refuses it.
func TestTelemetryPartitionWriterFloorsTheStore(t *testing.T) {
	h := partitionHarness(t, true)
	floor, err := h.store.StoreFloor()
	if err != nil || floor < telemetryPartitionFloorEpoch {
		t.Fatalf("store floor %d (err %v), want %d (T10)", floor, err, telemetryPartitionFloorEpoch)
	}
	if storeschema.Version < telemetryPartitionFloorEpoch {
		t.Fatalf("the build's epoch %d is below the partition floor", storeschema.Version)
	}
	unbound, _ := vocabUpcastOpenStore(t, "partition-unbound")
	if err := unbound.SetTelemetryPartitionWrites(true); err == nil || unbound.partitionWrites.Load() {
		t.Fatal("an unbound store turned the partition writer on")
	}
}

// T1 on the sealer's path (W2): batched tool results are stamped in the
// sealed batch and land in their day's partition when the batch seals.
func TestTelemetryPartitionSealedBatchLandsInPartitions(t *testing.T) {
	h := partitionHarness(t, true)
	sl := h.enableBatching(farFuture)
	partitionSeed(t, h, []retireEvent{
		{id: "sealed-1", kind: types.EventToolResult, at: partitionDay1},
		{id: "sealed-2", kind: types.EventToolResult, at: partitionDay1.Add(30 * time.Hour)},
	})
	sl.drain()
	if got, want := partitionFiles(t, h.store), []string{"tool_result-20261001", "tool_result-20261002"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("partition files %v, want %v (T1, sealer)", got, want)
	}
	if tables := tapeTables(t, h); tables["sealed-1"] != telemetryPartitionStamp || tables["sealed-2"] != telemetryPartitionStamp {
		t.Fatalf("stamps on the sealed batch: %v", tables)
	}
	if got := readerIDs(t, h.store); !reflect.DeepEqual(got, []string{"sealed-1", "sealed-2"}) {
		t.Fatalf("reader %v", got)
	}
}
