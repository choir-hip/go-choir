package store

// Deploy 1 of run telemetry in its own table (F-c):
// docs/design/choir-event-table-f-c-2026-10-22.md. Every failure mode in
// section 5 was written down before the code; each test below names the one
// it catches. Deploy 1 has no writer that stamps an op, so these tests put
// stamped ops on the tape or project them directly, exactly as deploy 2's
// writer will (section 2.2): an object_recorded op whose Table is
// "choir_events".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var eventTableBase = time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)

// eventTableRecord is one scripted event: the numbers deploy 2's writer
// would allocate are set by the script.
func eventTableRecord(id, run, owner, trajectory string, kind types.EventKind, seq, stream int64, at time.Time) *types.EventRecord {
	return &types.EventRecord{
		EventID: id, RunID: run, OwnerID: owner, TrajectoryID: trajectory, ChannelID: "channel-" + run,
		Kind: kind, Seq: seq, StreamSeq: stream, Timestamp: at,
		Payload: json.RawMessage(fmt.Sprintf(`{"id":%q}`, id)),
	}
}

// eventTableOp is the object_recorded op of one event; stamped sets Table.
func eventTableOp(t *testing.T, rec *types.EventRecord, stamped bool) computerevent.ProjectionOp {
	t.Helper()
	obj, err := buildEventObject(rec, rec.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	return eventTableObjectOp(t, obj, stamped)
}

func eventTableObjectOp(t *testing.T, obj objectgraph.Object, stamped bool) computerevent.ProjectionOp {
	t.Helper()
	body, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	op := computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body}
	if stamped {
		op.Table = choirEventsTable
	}
	return op
}

// projectAt projects ops at tape position pos in one committed transaction,
// as finalize does, without a tape: the reader tests need rows, not a chain.
func projectAt(t *testing.T, s *Store, pos uint64, ops ...computerevent.ProjectionOp) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	batch := computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: "computer-event-table", EventID: fmt.Sprintf("event-%d", pos), Ops: ops}
	if err := projectBatch(ctx, tx, batch, pos, nil); err != nil {
		t.Fatalf("project at %d: %v", pos, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func tableCount(t *testing.T, s *Store, table, where string, args ...any) int {
	t.Helper()
	q := "SELECT COUNT(*) FROM " + table
	if where != "" {
		q += " WHERE " + where
	}
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func tableHash(t *testing.T, s *Store, table string) string {
	t.Helper()
	var h string
	if err := s.db.QueryRow(`SELECT DOLT_HASHOF_TABLE(?)`, table).Scan(&h); err != nil {
		t.Fatalf("hash of %s: %v", table, err)
	}
	return h
}

// allStoredEvents decodes every event row of both tables.
func allStoredEvents(t *testing.T, s *Store) []types.EventRecord {
	t.Helper()
	var out []types.EventRecord
	for _, q := range []string{
		`SELECT body FROM og_objects WHERE object_kind = 'choir.event'`,
		`SELECT body FROM choir_events`,
	} {
		rows, err := s.db.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var body []byte
			if err := rows.Scan(&body); err != nil {
				t.Fatal(err)
			}
			var rec types.EventRecord
			if err := json.Unmarshal(body, &rec); err != nil {
				t.Fatal(err)
			}
			out = append(out, rec)
		}
		if err := rows.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func assertNoDuplicateNumbers(t *testing.T, s *Store) {
	t.Helper()
	seqs := map[string]string{}
	streams := map[int64]string{}
	for _, rec := range allStoredEvents(t, s) {
		key := fmt.Sprintf("%s/%s/%d", rec.RunID, rec.OwnerID, rec.Seq)
		if prev, dup := seqs[key]; dup {
			t.Fatalf("seq handed out twice: %s for %s and %s", key, prev, rec.EventID)
		}
		seqs[key] = rec.EventID
		if prev, dup := streams[rec.StreamSeq]; dup {
			t.Fatalf("stream_seq %d handed out twice: %s and %s", rec.StreamSeq, prev, rec.EventID)
		}
		streams[rec.StreamSeq] = rec.EventID
	}
}

// appendStamped puts one projection batch of stamped event ops on the
// harness tape.
func (h *tapeHarness) appendStamped(recs ...*types.EventRecord) {
	h.t.Helper()
	ops := make([]computerevent.ProjectionOp, 0, len(recs))
	for _, rec := range recs {
		ops = append(ops, eventTableOp(h.t, rec, true))
	}
	if err := h.store.AppendProjectionOps(context.Background(), ops); err != nil {
		h.t.Fatalf("append stamped: %v", err)
	}
}

// F1: a seed that reads og_objects only hands a number out twice once events
// live in choir_events. Each reopen is a new process whose marks start empty.
func TestEventTableSeedsReadBothTables(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	for i := int64(1); i <= 5; i++ {
		h.appendStamped(eventTableRecord(fmt.Sprintf("a-%d", i), "run-a", "owner-1", "traj-a", types.EventToolResult, i, i, eventTableBase.Add(time.Duration(i)*time.Second)))
	}
	h.appendStamped(
		eventTableRecord("b-1", "run-b", "owner-1", "traj-b", types.EventRunDelta, 1, 6, eventTableBase.Add(6*time.Second)),
		eventTableRecord("b-2", "run-b", "owner-1", "traj-b", types.EventRunDelta, 2, 7, eventTableBase.Add(7*time.Second)),
		eventTableRecord("b-3", "run-b", "owner-1", "traj-b", types.EventRunDelta, 3, 8, eventTableBase.Add(8*time.Second)),
	)
	if n := tableCount(t, h.store, "choir_events", ""); n != 8 {
		t.Fatalf("choir_events rows = %d, want 8", n)
	}
	if n := tableCount(t, h.store, "og_objects", "object_kind = 'choir.event'"); n != 0 {
		t.Fatalf("stamped ops reached og_objects: %d rows", n)
	}

	h.reopen()
	next := h.event("a-6", "run-a", "owner-1", types.EventRunStarted, eventTableBase.Add(9*time.Second))
	if err := h.store.AppendEvent(ctx, next); err != nil {
		t.Fatal(err)
	}
	if next.Seq != 6 || next.StreamSeq != 9 {
		t.Fatalf("after a reopen run-a got seq %d stream_seq %d, want 6 and 9", next.Seq, next.StreamSeq)
	}
	// A run with no rows in either table: MAX over no rows is NULL, and the
	// append still succeeds with seq 1.
	fresh := h.event("c-1", "run-c", "owner-1", types.EventRunStarted, eventTableBase.Add(10*time.Second))
	if err := h.store.AppendEvent(ctx, fresh); err != nil {
		t.Fatalf("a run's first append: %v", err)
	}
	if fresh.Seq != 1 || fresh.StreamSeq != 10 {
		t.Fatalf("run-c got seq %d stream_seq %d, want 1 and 10", fresh.Seq, fresh.StreamSeq)
	}
	assertNoDuplicateNumbers(t, h.store)

	// Parallel appenders after another reopen.
	h.reopen()
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				run := []string{"run-a", "run-b"}[g%2]
				rec := h.event(fmt.Sprintf("p-%d-%d", g, i), run, "owner-1", types.EventRunStarted, eventTableBase.Add(time.Minute))
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

	// With the sealer on: the seed still starts above both tables.
	h.reopen()
	sl := h.enableBatching(time.Hour)
	batched := h.event("b-4", "run-b", "owner-1", types.EventRunDelta, eventTableBase.Add(2*time.Minute))
	if err := h.store.AppendEvent(ctx, batched); err != nil {
		t.Fatal(err)
	}
	if batched.Seq <= 3 || batched.StreamSeq <= 30 {
		t.Fatalf("with batching run-b got seq %d stream_seq %d, at or below a stored number", batched.Seq, batched.StreamSeq)
	}
	sl.drain()
	assertNoDuplicateNumbers(t, h.store)
}

// F1, after a reconstruction: a replayed store seeds from the replayed table.
func TestEventTableSeedsAfterReplay(t *testing.T) {
	ctx := context.Background()
	var batches [][]computerevent.ProjectionOp
	for i := int64(1); i <= 4; i++ {
		batches = append(batches, []computerevent.ProjectionOp{eventTableOp(t, eventTableRecord(fmt.Sprintf("r-%d", i), "run-r", vocabUpcastOwnerID, "traj-r", types.EventToolInvoked, i, 100+i, eventTableBase.Add(time.Duration(i)*time.Second)), true)})
	}
	tape := vocabUpcastTapeFromBatches(t, batches)
	s, _ := vocabUpcastOpenStore(t, "seed-after-replay")
	vocabUpcastReplayTape(t, s, tape, 0, len(tape))
	rec := eventTableRecord("r-5", "run-r", vocabUpcastOwnerID, "traj-r", types.EventRunStarted, 0, 0, eventTableBase.Add(time.Minute))
	if err := s.AppendEvent(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.Seq != 5 || rec.StreamSeq != 105 {
		t.Fatalf("after replay got seq %d stream_seq %d, want 5 and 105", rec.Seq, rec.StreamSeq)
	}
	assertNoDuplicateNumbers(t, s)
}

// eventTableScript is the differential script of F2: three runs, two owners,
// two trajectories, batched and acknowledged kinds, timestamps in
// allocation order.
func eventTableScript(n int) []*types.EventRecord {
	runs := []string{"run-1", "run-2", "run-3"}
	seqs := map[string]int64{}
	out := make([]*types.EventRecord, 0, n)
	for i := 1; i <= n; i++ {
		run := runs[i%len(runs)]
		owner := "owner-1"
		if i%7 == 0 {
			owner = "owner-2"
		}
		traj := "traj-1"
		if run == "run-3" {
			traj = "traj-2"
		}
		kind := []types.EventKind{types.EventToolResult, types.EventRunDelta, types.EventRunStarted, types.EventDeskCell, types.EventRunCompleted}[i%5]
		seqs[run+owner]++
		out = append(out, eventTableRecord(fmt.Sprintf("e-%03d", i), run, owner, traj, kind, seqs[run+owner], int64(i), eventTableBase.Add(time.Duration(i)*time.Second)))
	}
	return out
}

// eventTableStoreFromScript projects the script three ops per position.
// stamp decides, per script index, whether a batched event is stamped.
func eventTableStoreFromScript(t *testing.T, name string, script []*types.EventRecord, stamp func(i int) bool) *Store {
	t.Helper()
	s, _ := vocabUpcastOpenStore(t, name)
	var ops []computerevent.ProjectionOp
	pos := uint64(2)
	for i, rec := range script {
		ops = append(ops, eventTableOp(t, rec, rec.Kind.Storage() == types.StorageEventTable && stamp(i)))
		if len(ops) == 3 || i == len(script)-1 {
			projectAt(t, s, pos, ops...)
			ops = nil
			pos++
		}
	}
	return s
}

func eventIDs(events []types.EventRecord) string {
	parts := make([]string, 0, len(events))
	for _, e := range events {
		parts = append(parts, fmt.Sprintf("%s#%d/%d", e.EventID, e.Seq, e.StreamSeq))
	}
	return strings.Join(parts, ",")
}

// mustEvents returns a check for one read: mustEvents(t)(s.ListEvents...).
func mustEvents(t *testing.T) func([]types.EventRecord, error) []types.EventRecord {
	return func(events []types.EventRecord, err error) []types.EventRecord {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return events
	}
}

// eventTableReads runs every union read over the script's keys and returns
// one line per call.
func eventTableReads(t *testing.T, s *Store) []string {
	t.Helper()
	ctx := context.Background()
	var out []string
	add := func(call string, events []types.EventRecord) {
		raw, err := json.Marshal(events)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, call+" => "+eventIDs(events)+" "+string(raw))
	}
	for _, run := range []string{"run-1", "run-2", "run-3", "run-none"} {
		for _, after := range []int64{0, 1, 2, 5, 100} {
			for _, limit := range []int{1, 2, 3, 5, 1000} {
				add(fmt.Sprintf("ListEventsAfter(%s,%d,%d)", run, after, limit), mustEvents(t)(s.ListEventsAfter(ctx, run, after, limit)))
			}
		}
		// handleRunEvents pages with limit+1; walk the run four at a time.
		var paged []types.EventRecord
		after := int64(0)
		for {
			page := mustEvents(t)(s.ListEventsAfter(ctx, run, after, 4+1))
			if len(page) > 4 {
				page = page[:4]
			}
			if len(page) == 0 {
				break
			}
			paged = append(paged, page...)
			after = page[len(page)-1].Seq
		}
		add("paged("+run+")", paged)
	}
	for _, owner := range []string{"owner-1", "owner-2", "owner-none"} {
		for _, after := range []int64{0, 3, 10, 40} {
			for _, limit := range []int{1, 4, 1000} {
				add(fmt.Sprintf("ListEventsByOwnerAfter(%s,%d,%d)", owner, after, limit), mustEvents(t)(s.ListEventsByOwnerAfter(ctx, owner, after, limit)))
			}
		}
		for _, limit := range []int{1, 5, 1000} {
			add(fmt.Sprintf("ListEventsByOwner(%s,%d)", owner, limit), mustEvents(t)(s.ListEventsByOwner(ctx, owner, limit)))
		}
		for _, traj := range []string{"traj-1", "traj-2"} {
			for _, limit := range []int{1, 5, 3000} {
				add(fmt.Sprintf("ListEventsByTrajectory(%s,%s,%d)", owner, traj, limit), mustEvents(t)(s.ListEventsByTrajectory(ctx, owner, traj, limit)))
			}
		}
	}
	return out
}

func assertSameReads(t *testing.T, label string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: %d reads, want %d", label, len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s differs from the single-table store:\nwant %s\ngot  %s", label, want[i], got[i])
		}
	}
}

// F2: a reader forgets the union, or merges or cuts wrongly. One script into
// a store with nothing stamped, one with every batched event stamped, and one
// whose stamping toggles on and off mid-run: every read is identical.
func TestEventTableUnionReadsMatchSingleTable(t *testing.T) {
	script := eventTableScript(60)
	off := eventTableStoreFromScript(t, "union-off", script, func(int) bool { return false })
	on := eventTableStoreFromScript(t, "union-on", script, func(int) bool { return true })
	toggled := eventTableStoreFromScript(t, "union-toggle", script, func(i int) bool { return i >= 20 && i < 40 })
	if tableCount(t, on, "choir_events", "") == 0 || tableCount(t, toggled, "choir_events", "") == 0 {
		t.Fatal("fixture: nothing was stamped")
	}
	want := eventTableReads(t, off)
	assertSameReads(t, "setting on", want, eventTableReads(t, on))
	assertSameReads(t, "setting toggled", want, eventTableReads(t, toggled))

	// One event in both tables (a bug, or a store an old binary touched)
	// comes back once.
	planted := eventTableStoreFromScript(t, "union-planted", script, func(int) bool { return true })
	var dup *types.EventRecord
	for _, rec := range script {
		if rec.Kind.Storage() == types.StorageEventTable && rec.RunID == "run-1" {
			dup = rec
			break
		}
	}
	projectAt(t, planted, 1000, eventTableOp(t, dup, false))
	if tableCount(t, planted, "og_objects", "object_kind = 'choir.event' AND JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.event_id')) = ?", dup.EventID) != 1 {
		t.Fatal("fixture: the planted copy is not in og_objects")
	}
	assertSameReads(t, "planted in both tables", want, eventTableReads(t, planted))
}

// F2, the trajectory window (round 2, DeepSeek F3): a trajectory over 10,000
// events reads today's window, the newest 10,000 by timestamp, and keeps its
// oldest limit by stream_seq.
func TestEventTableTrajectoryWindowOverTenThousand(t *testing.T) {
	if testing.Short() {
		t.Skip("writes 10,050 events into three stores")
	}
	const n = 10050
	script := make([]*types.EventRecord, 0, n)
	for i := 1; i <= n; i++ {
		kind := types.EventToolResult
		if i%10 == 0 {
			kind = types.EventRunStarted
		}
		script = append(script, eventTableRecord(fmt.Sprintf("t-%05d", i), "run-big", "owner-1", "traj-big", kind, int64(i), int64(i), eventTableBase.Add(time.Duration(i)*time.Second)))
	}
	build := func(name string, stamp func(i int) bool) *Store {
		s, _ := vocabUpcastOpenStore(t, name)
		const per = 500
		for start := 0; start < n; start += per {
			end := min(start+per, n)
			ops := make([]computerevent.ProjectionOp, 0, end-start)
			for i := start; i < end; i++ {
				ops = append(ops, eventTableOp(t, script[i], script[i].Kind.Storage() == types.StorageEventTable && stamp(i)))
			}
			projectAt(t, s, uint64(2+start/per), ops...)
		}
		return s
	}
	ctx := context.Background()
	read := func(s *Store, limit int) string {
		return eventIDs(mustEvents(t)(s.ListEventsByTrajectory(ctx, "owner-1", "traj-big", limit)))
	}
	off := build("traj-off", func(int) bool { return false })
	on := build("traj-on", func(int) bool { return true })
	toggled := build("traj-toggle", func(i int) bool { return i >= 4000 && i < 8000 })
	for _, limit := range []int{500, 3000} {
		want := read(off, limit)
		if !strings.HasPrefix(want, "t-00051#") {
			t.Fatalf("fixture: today's window should start at the 51st event, got %.40s", want)
		}
		if got := read(on, limit); got != want {
			t.Fatalf("limit %d, setting on: got %.80s..., want %.80s...", limit, got, want)
		}
		if got := read(toggled, limit); got != want {
			t.Fatalf("limit %d, toggled: got %.80s..., want %.80s...", limit, got, want)
		}
	}
}

// F2, the documented boundary of ListEventsByOwner (section 4.4): when a
// caller's timestamps disagree with allocation order, the og_objects half
// picks the newest by timestamp and the choir_events half the newest by
// stream_seq. The difference is a row at the cut. This test pins it.
func TestEventTableOwnerNewestCutWithSwappedTimestamps(t *testing.T) {
	ctx := context.Background()
	// e1 is allocated first but stamped later in time.
	e1 := eventTableRecord("swap-1", "run-s", "owner-1", "traj-s", types.EventToolResult, 1, 1, eventTableBase.Add(2*time.Second))
	e2 := eventTableRecord("swap-2", "run-s", "owner-1", "traj-s", types.EventToolResult, 2, 2, eventTableBase.Add(time.Second))
	off, _ := vocabUpcastOpenStore(t, "swap-off")
	projectAt(t, off, 2, eventTableOp(t, e1, false), eventTableOp(t, e2, false))
	on, _ := vocabUpcastOpenStore(t, "swap-on")
	projectAt(t, on, 2, eventTableOp(t, e1, true), eventTableOp(t, e2, true))
	if got := eventIDs(mustEvents(t)(off.ListEventsByOwner(ctx, "owner-1", 1))); !strings.HasPrefix(got, "swap-1#") {
		t.Fatalf("og_objects newest by timestamp = %s, want swap-1", got)
	}
	if got := eventIDs(mustEvents(t)(on.ListEventsByOwner(ctx, "owner-1", 1))); !strings.HasPrefix(got, "swap-2#") {
		t.Fatalf("choir_events newest by stream_seq = %s, want swap-2 (the documented cut difference)", got)
	}
	// With room for both, the two stores agree.
	if a, b := eventIDs(mustEvents(t)(off.ListEventsByOwner(ctx, "owner-1", 10))), eventIDs(mustEvents(t)(on.ListEventsByOwner(ctx, "owner-1", 10))); a != b {
		t.Fatalf("whole result differs: %s against %s", a, b)
	}
}

// eventTableMixedTape is F3's mixed tape: unstamped singles, stamped singles,
// a batch with both, a toggle, and a stamped op of a kind no binary knows
// yet (F22).
func eventTableMixedTape(t *testing.T) []vocabUpcastTapeEvent {
	t.Helper()
	owner := vocabUpcastOwnerID
	rec := func(i int, kind types.EventKind) *types.EventRecord {
		return eventTableRecord(fmt.Sprintf("m-%02d", i), "run-m", owner, "traj-m", kind, int64(i), int64(i), eventTableBase.Add(time.Duration(i)*time.Second))
	}
	return vocabUpcastTapeFromBatches(t, [][]computerevent.ProjectionOp{
		{eventTableOp(t, rec(1, types.EventRunStarted), false)},
		{eventTableOp(t, rec(2, types.EventToolInvoked), true)},
		{eventTableOp(t, rec(3, types.EventToolResult), true), eventTableOp(t, rec(4, types.EventRunDelta), true), eventTableOp(t, rec(5, types.EventRunProgress), false)},
		{eventTableOp(t, rec(6, types.EventToolResult), false)}, // the setting off
		{eventTableOp(t, rec(7, types.EventDeskCell), true)},    // and on again
		{eventTableOp(t, rec(8, types.EventKind("loop.future")), true)},
		{eventTableOp(t, rec(9, types.EventRunCompleted), false), eventTableOp(t, rec(10, types.EventDeskEgress), true)},
	})
}

// eventTableWitness extracts the production witness of a closed store's
// workspace, with the production extractor configuration.
func eventTableWitness(t *testing.T, path string) (computerversion.ObservationSet, selfdevprotocol.VMLocalContentWitness) {
	t.Helper()
	extractor := computerversion.DoltStateExtractor{
		WorkspacePath: deriveTextureWorkspacePath(path),
		Database:      "texture",
		IgnoredTables: map[string]struct{}{objectgraph.DerivedKeysTable: {}},
		IgnoredContentColumns: map[string]map[string]struct{}{
			"computer_event_index":            {"prepared_at": {}, "finalized_at": {}},
			"computer_event_projection_heads": {"updated_at": {}},
		},
	}
	set, err := extractor.Extract(context.Background(), computerversion.ExtractRequest{
		Name:    "event-table-witness",
		Version: computerversion.ComputerVersion{CodeRef: "test", ArtifactProgramRef: "test"},
	})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(set, set, computerversion.EquivalenceChecker{}.CheckObservationSets(set, set))
	if err != nil {
		t.Fatalf("witness: %v", err)
	}
	return set, witness
}

func observed(set computerversion.ObservationSet, key string) bool {
	for _, o := range set.Observations {
		if o.Key == key {
			return true
		}
	}
	return false
}

// F3 and F22: live finalize and replay finalize of one mixed tape write the
// same rows; the workspaces' witnesses match.
func TestEventTableReplayEqualsLive(t *testing.T) {
	tape := eventTableMixedTape(t)
	live, livePath := vocabUpcastOpenStore(t, "mixed-live")
	for _, ev := range tape {
		vocabUpcastApplyLiveEvent(t, live, ev)
	}
	replay, replayPath := vocabUpcastOpenStore(t, "mixed-replay")
	vocabUpcastReplayTape(t, replay, tape, 0, len(tape))

	if n := tableCount(t, live, "choir_events", ""); n != 6 {
		t.Fatalf("live choir_events rows = %d, want 6", n)
	}
	if n := tableCount(t, live, "og_objects", "object_kind = 'choir.event'"); n != 4 {
		t.Fatalf("live og_objects events = %d, want 4", n)
	}
	if n := tableCount(t, live, "choir_events", "event_kind = 'loop.future'"); n != 1 {
		t.Fatalf("the unknown kind was not stored as given: %d rows", n)
	}
	for _, table := range []string{"choir_events", "og_objects"} {
		if a, b := tableHash(t, live, table), tableHash(t, replay, table); a != b {
			t.Fatalf("%s differs between live and replay", table)
		}
	}
	ctx := context.Background()
	if a, b := eventIDs(mustEvents(t)(live.ListEventsAfter(ctx, "run-m", 0, 100))), eventIDs(mustEvents(t)(replay.ListEventsAfter(ctx, "run-m", 0, 100))); a != b || strings.Count(a, ",") != 9 {
		t.Fatalf("live reads %s, replay reads %s", a, b)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}
	if err := replay.Close(); err != nil {
		t.Fatal(err)
	}
	liveSet, liveWitness := eventTableWitness(t, livePath)
	_, replayWitness := eventTableWitness(t, replayPath)
	if err := selfdevprotocol.WitnessContentMatches(replayWitness, liveWitness); err != nil {
		t.Fatalf("replay witness: %v", err)
	}
	if !observed(liveSet, "dolt:texture:table:choir_events") {
		t.Fatal("a non-empty choir_events is not observed")
	}
}

// F13: restore to a prior head keeps no row after it.
func TestEventTableReconstructThroughTargetKeepsNoLaterRows(t *testing.T) {
	tape := eventTableMixedTape(t)
	const h1 = 4 // genesis, then three batches
	through, throughPath := vocabUpcastOpenStore(t, "through-h1")
	vocabUpcastReplayTape(t, through, tape, 0, h1)
	var maxSeq uint64
	if err := through.db.QueryRow(`SELECT COALESCE(MAX(tape_seq), 0) FROM choir_events`).Scan(&maxSeq); err != nil {
		t.Fatal(err)
	}
	if maxSeq == 0 || maxSeq > h1 {
		t.Fatalf("max tape_seq after replay through %d = %d", h1, maxSeq)
	}
	atH1, atH1Path := vocabUpcastOpenStore(t, "live-at-h1")
	for _, ev := range tape[:h1] {
		vocabUpcastApplyLiveEvent(t, atH1, ev)
	}
	_ = through.Close()
	_ = atH1.Close()
	_, a := eventTableWitness(t, throughPath)
	_, b := eventTableWitness(t, atH1Path)
	if err := selfdevprotocol.WitnessContentMatches(a, b); err != nil {
		t.Fatalf("witness through h1 differs from one taken at h1: %v", err)
	}
}

// F4: a checkpoint witnessed before deploy 1 has no choir_events. The new
// DDL creates the table empty; an empty table is not observed, so the old
// witness still matches. One stamped op makes the table observed.
func TestEventTableEmptyTableKeepsOldWitness(t *testing.T) {
	tape := eventTableMixedTape(t)
	s, path := vocabUpcastOpenStore(t, "old-witness")
	vocabUpcastApplyLiveEvent(t, s, tape[0])
	vocabUpcastApplyLiveEvent(t, s, tape[1]) // unstamped
	if _, err := s.db.Exec(`DROP TABLE choir_events`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	oldSet, oldWitness := eventTableWitness(t, path) // a workspace with no table: no probe, no error
	if observed(oldSet, "dolt:texture:schema:choir_events") {
		t.Fatal("fixture: the pre-deploy-1 workspace has the table")
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, reopened, "choir_events", ""); n != 0 {
		t.Fatalf("the new DDL did not create an empty table: %d", n)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	newSet, newWitness := eventTableWitness(t, path)
	if observed(newSet, "dolt:texture:schema:choir_events") || observed(newSet, "dolt:texture:table:choir_events") {
		t.Fatal("an empty choir_events is observed")
	}
	if err := selfdevprotocol.WitnessContentMatches(newWitness, oldWitness); err != nil {
		t.Fatalf("a pre-deploy-1 checkpoint no longer matches under the new DDL: %v", err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	vocabUpcastApplyLiveEvent(t, again, tape[2]) // stamped
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
	stampedSet, stampedWitness := eventTableWitness(t, path)
	if !observed(stampedSet, "dolt:texture:table:choir_events") {
		t.Fatal("a stamped row left the table unobserved")
	}
	if selfdevprotocol.WitnessContentMatches(stampedWitness, oldWitness) == nil {
		t.Fatal("test premise: a later head must not match the old checkpoint")
	}
}

// The optional set names the table bootstrap creates (section 3.3).
func TestEventTableIsTheExtractorsOptionalTable(t *testing.T) {
	if got := computerversion.OptionalTables(); !reflect.DeepEqual(got, []string{choirEventsTable}) {
		t.Fatalf("computerversion.OptionalTables() = %v, want [%s]", got, choirEventsTable)
	}
	s := openProjectStore(t)
	if n := tableCount(t, s, "choir_events", ""); n != 0 {
		t.Fatalf("a fresh store's choir_events has %d rows", n)
	}
}

// F6: the dry run's provisional position is the finalized tape position, and
// the dry run refuses every shape finalize would refuse, so nothing the table
// cannot hold becomes canonical.
func TestEventTableDryRunPositionIsTheTapePosition(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	var provisional []uint64
	h.dryRunHook = func(*computerevent.ProjectionBatch) error {
		pos, err := dryRunTapePosition(ctx, h.store.db, h.computerID)
		if err != nil {
			return err
		}
		provisional = append(provisional, pos)
		return nil
	}
	for i := int64(1); i <= 3; i++ {
		h.appendStamped(eventTableRecord(fmt.Sprintf("d-%d", i), "run-d", "owner-1", "traj-d", types.EventToolResult, i, i, eventTableBase.Add(time.Duration(i)*time.Second)))
	}
	rows, err := h.store.db.Query(`SELECT tape_seq FROM choir_events ORDER BY tape_seq`)
	if err != nil {
		t.Fatal(err)
	}
	var final []uint64
	for rows.Next() {
		var seq uint64
		if err := rows.Scan(&seq); err != nil {
			t.Fatal(err)
		}
		final = append(final, seq)
	}
	_ = rows.Close()
	if !reflect.DeepEqual(provisional, final) {
		t.Fatalf("dry-run positions %v, finalized tape_seq %v", provisional, final)
	}
	for _, rec := range h.tape.snapshot() {
		if !contains(final, rec.Sequence) {
			t.Fatalf("tape sequence %d has no row", rec.Sequence)
		}
	}

	// A store that was never projected has no head row: position 1.
	empty, _ := vocabUpcastOpenStore(t, "never-projected")
	if pos, err := dryRunTapePosition(ctx, empty.db, "computer-never-projected"); err != nil || pos != 1 {
		t.Fatalf("no head row: position %d, %v; want 1", pos, err)
	}
}

func contains(xs []uint64, x uint64) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func TestEventTableRefusesAtTheDryRunNeverOnTheTape(t *testing.T) {
	h := newTapeHarness(t)
	good := func() objectgraph.Object {
		obj, err := buildEventObject(eventTableRecord("ok", "run-x", "owner-1", "traj-x", types.EventToolResult, 1, 1, eventTableBase), eventTableBase)
		if err != nil {
			t.Fatal(err)
		}
		return obj
	}
	long := strings.Repeat("r", 300)
	cases := map[string]objectgraph.Object{}
	{
		obj, err := buildEventObject(eventTableRecord("long-run", long, "owner-1", "traj-x", types.EventToolResult, 1, 1, eventTableBase), eventTableBase)
		if err != nil {
			t.Fatal(err)
		}
		cases["run_id of 300"] = obj
	}
	{
		obj, err := buildEventObject(eventTableRecord("long-traj", "run-x", "owner-1", long, types.EventToolResult, 1, 1, eventTableBase), eventTableBase)
		if err != nil {
			t.Fatal(err)
		}
		cases["trajectory_id of 300"] = obj
	}
	{
		obj, err := buildEventObject(eventTableRecord("long-kind", "run-x", "owner-1", "traj-x", types.EventKind(strings.Repeat("k", 200)), 1, 1, eventTableBase), eventTableBase)
		if err != nil {
			t.Fatal(err)
		}
		cases["metadata kind of 200"] = obj
	}
	{
		obj := good()
		obj.CreatedAt, obj.UpdatedAt = time.Time{}, time.Time{}
		cases["zero timestamps"] = obj
	}
	{
		obj := good()
		obj.UpdatedAt = time.Time{}
		cases["zero updated_at"] = obj
	}
	{
		obj := good()
		obj.Tombstone = true
		cases["tombstone"] = obj
	}
	{
		obj := good()
		obj.SupersededBy = "obj:choir.event:owner-1:other"
		cases["superseded_by"] = obj
	}
	{
		obj := good()
		obj.ObjectKind = "choir.run"
		cases["not a choir.event"] = obj
	}
	{
		obj := good()
		obj.Body = []byte(`[1,2,3]`)
		cases["body not an event record"] = obj
	}
	{
		obj := good()
		obj.Metadata = json.RawMessage(`{"seq":"one"}`)
		cases["metadata seq not a number"] = obj
	}
	for name, obj := range cases {
		before := len(h.tape.snapshot())
		err := h.store.AppendProjectionOps(context.Background(), []computerevent.ProjectionOp{eventTableObjectOp(t, obj, true)})
		if err == nil {
			t.Fatalf("%s: a stamped op the table cannot hold was accepted", name)
		}
		if !errors.Is(err, computerevent.ErrProjectionBatchInvalid) {
			t.Fatalf("%s: refusal %v is not ErrProjectionBatchInvalid", name, err)
		}
		if after := len(h.tape.snapshot()); after != before {
			t.Fatalf("%s: the refused op reached the tape", name)
		}
	}
	if n := tableCount(t, h.store, "choir_events", ""); n != 0 {
		t.Fatalf("refused ops left %d rows", n)
	}
}

func TestEventTableAcceptsWhatALaterBinaryMayWrite(t *testing.T) {
	h := newTapeHarness(t)
	scoped := func() objectgraph.Object {
		obj, err := buildEventObject(eventTableRecord("scoped", "run-y", "owner-1", "traj-y", types.EventToolResult, 1, 1, eventTableBase), eventTableBase)
		if err != nil {
			t.Fatal(err)
		}
		obj.ComputerID, obj.VersionID = "computer-y", "version-y"
		return obj
	}()
	kindless := func() objectgraph.Object {
		body, _ := json.Marshal(eventTableRecord("kindless", "run-y", "owner-1", "traj-y", "", 2, 2, eventTableBase))
		meta, err := objectgraph.NormalizeMetadata(map[string]any{"run_id": "run-y", "seq": 2, "stream_seq": 2, "trajectory_id": "traj-y"})
		if err != nil {
			t.Fatal(err)
		}
		hash := objectgraph.ContentHash(ogKindEvent, body, meta)
		id, _ := objectgraph.BuildCanonicalID(ogKindEvent, "owner-1", objectgraph.StableSuffixFromContent(hash))
		return objectgraph.Object{CanonicalID: id, ObjectKind: ogKindEvent, OwnerID: "owner-1", ContentHash: hash, Body: body, Metadata: meta, CreatedAt: eventTableBase, UpdatedAt: eventTableBase}
	}()
	future, err := buildEventObject(eventTableRecord("future", "run-y", "owner-1", "traj-y", types.EventKind("loop.future"), 3, 3, eventTableBase), eventTableBase)
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range []objectgraph.Object{scoped, kindless, future} {
		if err := h.store.AppendProjectionOps(context.Background(), []computerevent.ProjectionOp{eventTableObjectOp(t, obj, true)}); err != nil {
			t.Fatalf("%s refused: %v", obj.CanonicalID, err)
		}
	}
	var computerID, versionID string
	if err := h.store.db.QueryRow(`SELECT computer_id, version_id FROM choir_events WHERE canonical_id = ?`, scoped.CanonicalID).Scan(&computerID, &versionID); err != nil || computerID != "computer-y" || versionID != "version-y" {
		t.Fatalf("computer_id %q version_id %q (%v)", computerID, versionID, err)
	}
	var kind string
	if err := h.store.db.QueryRow(`SELECT event_kind FROM choir_events WHERE canonical_id = ?`, kindless.CanonicalID).Scan(&kind); err != nil || kind != "" {
		t.Fatalf("absent metadata kind stored as %q (%v), want ''", kind, err)
	}
	if err := h.store.db.QueryRow(`SELECT event_kind FROM choir_events WHERE canonical_id = ?`, future.CanonicalID).Scan(&kind); err != nil || kind != "loop.future" {
		t.Fatalf("unknown kind stored as %q (%v)", kind, err)
	}
}

// A row reproduces the object byte for byte: the columns og_objects has.
func TestEventTableRowIsTheWholeObject(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	rec := eventTableRecord("whole", "run-w", "owner-1", "traj-w", types.EventToolResult, 4, 9, eventTableBase)
	obj, err := buildEventObject(rec, rec.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	projectAt(t, s, 7, eventTableObjectOp(t, obj, true))
	var (
		tapeSeq, opIndex                                       uint64
		id, objectKind, owner, contentHash, metadata, runID, k string
		traj                                                   string
		body                                                   []byte
		seq, stream                                            int64
		created, updated                                       time.Time
	)
	err = s.db.QueryRowContext(ctx, `SELECT tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at, run_id, seq, stream_seq, trajectory_id, event_kind FROM choir_events`).
		Scan(&tapeSeq, &opIndex, &id, &objectKind, &owner, &contentHash, &body, &metadata, &created, &updated, &runID, &seq, &stream, &traj, &k)
	if err != nil {
		t.Fatal(err)
	}
	if tapeSeq != 7 || opIndex != 0 || id != obj.CanonicalID || objectKind != "choir.event" || owner != "owner-1" || contentHash != obj.ContentHash ||
		string(body) != string(obj.Body) || metadata != string(obj.Metadata) || !created.Equal(obj.CreatedAt) || !updated.Equal(obj.UpdatedAt) ||
		runID != "run-w" || seq != 4 || stream != 9 || traj != "traj-w" || k != string(types.EventToolResult) {
		t.Fatalf("row does not reproduce the object: seq=%d op=%d id=%s kind=%s run=%s %d/%d traj=%s ek=%s", tapeSeq, opIndex, id, objectKind, runID, seq, stream, traj, k)
	}
}

// F7: re-finalizing or re-projecting the same event never wedges on the key.
func TestEventTableReprojectionIsIdempotent(t *testing.T) {
	ctx := context.Background()
	tape := eventTableMixedTape(t)
	s, _ := vocabUpcastOpenStore(t, "idempotent")
	vocabUpcastApplyLiveEvent(t, s, tape[0])
	vocabUpcastApplyLiveEvent(t, s, tape[1])
	vocabUpcastApplyLiveEvent(t, s, tape[2]) // stamped, at sequence 3
	before := tableHash(t, s, "choir_events")
	digest, err := tape[2].Event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	batch := &computerevent.ProjectionBatch{Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: tape[2].Event.ComputerID, EventID: tape[2].Event.EventID, EventDigest: digest, Ops: vocabUpcastCopyOps(tape[2].Ops)}
	if err := s.FinalizeBatch(ctx, tape[2].Event.ComputerID, digest, tape[2].Receipt, batch); err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	// The finalize after RecoverPrepared re-projects the committed batch at
	// the same position.
	projectAt(t, s, 3, vocabUpcastCopyOps(tape[2].Ops)...)
	if n := tableCount(t, s, "choir_events", ""); n != 1 {
		t.Fatalf("rows after re-projection = %d, want 1", n)
	}
	if after := tableHash(t, s, "choir_events"); after != before {
		t.Fatal("re-projection changed the row")
	}
}

// F14: event-table reads race the projector on the shared engine. Run with
// -race.
func TestEventTableReadsRaceTheProjector(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	done := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for r := 0; r < 3; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				for _, read := range []func() error{
					func() error { _, err := h.store.ListEventsAfter(ctx, "run-race", 0, 50); return err },
					func() error { _, err := h.store.ListEventsByOwnerAfter(ctx, "owner-1", 0, 50); return err },
					func() error { _, err := h.store.ListEventsByOwner(ctx, "owner-1", 50); return err },
					func() error { _, err := h.store.ListEventsByTrajectory(ctx, "owner-1", "traj-race", 50); return err },
				} {
					if err := read(); err != nil {
						errs <- err
						return
					}
				}
			}
		}()
	}
	for i := int64(1); i <= 20; i++ {
		h.appendStamped(eventTableRecord(fmt.Sprintf("race-%d", i), "run-race", "owner-1", "traj-race", types.EventToolResult, i, i, eventTableBase.Add(time.Duration(i)*time.Second)))
	}
	close(done)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if got := len(mustEvents(t)(h.store.ListEventsAfter(ctx, "run-race", 0, 100))); got != 20 {
		t.Fatalf("read %d events, want 20", got)
	}
}

// F15: boot creates the table empty and never reads or rewrites og_objects.
func TestEventTableBootLeavesObjectGraphAlone(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	const rows = 3000
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for start := 0; start < rows; start += 500 {
		var sb strings.Builder
		var args []any
		sb.WriteString(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES `)
		for i := start; i < start+500; i++ {
			if i > start {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,NOW(),NOW(),0,'')")
			args = append(args, fmt.Sprintf("obj:choir.event:o1:key-%06d", i), "choir.event", "o1", "", "", fmt.Sprintf("h-%d", i),
				fmt.Sprintf(`{"run_id":"r%d","seq":%d}`, i%10, i), fmt.Sprintf(`{"run_id":"r%d","seq":%d,"stream_seq":%d}`, i%10, i, i))
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP TABLE choir_events`); err != nil { // a pre-deploy-1 store
		t.Fatal(err)
	}
	before := tableHash(t, s, "og_objects")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	reopened, err := Open(path)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if after := tableHash(t, reopened, "og_objects"); after != before {
		t.Fatal("Open rewrote og_objects")
	}
	if n := tableCount(t, reopened, "choir_events", ""); n != 0 {
		t.Fatalf("Open filled choir_events: %d rows", n)
	}
	if took > 30*time.Second {
		t.Fatalf("Open took %s", took)
	}
}

// F16: each choir_events query reads its index, in index order.
func TestEventTableQueriesUseTheirIndexes(t *testing.T) {
	s := openProjectStore(t)
	for i := int64(1); i <= 40; i++ {
		projectAt(t, s, uint64(i+1), eventTableOp(t, eventTableRecord(fmt.Sprintf("p-%d", i), fmt.Sprintf("run-%d", i%4), "owner-1", fmt.Sprintf("traj-%d", i%3), types.EventToolResult, i, i, eventTableBase.Add(time.Duration(i)*time.Second)), true))
	}
	type planCase struct {
		name, index string
		query       string
		args        []any
	}
	q := func(name, index string, query string, args []any) planCase {
		return planCase{name: name, index: index, query: query, args: args}
	}
	var cases []planCase
	{
		query, args := eventTableRunAfterSQL("run-1", 3, 10)
		cases = append(cases, q("run after", "choir_events.run_id,choir_events.seq", query, args))
	}
	{
		query, args := eventTableOwnerAfterSQL("owner-1", 3, 10)
		cases = append(cases, q("owner after", "choir_events.owner_id,choir_events.stream_seq", query, args))
	}
	{
		query, args := eventTableOwnerNewestSQL("owner-1", 10)
		cases = append(cases, q("owner newest", "choir_events.owner_id,choir_events.stream_seq", query, args))
	}
	{
		query, args := eventTableTrajectoryNewestSQL("traj-1", 10)
		cases = append(cases, q("trajectory newest", "choir_events.trajectory_id,choir_events.stream_seq", query, args))
	}
	{
		query, args := eventTableRunMaxSeqSQL("run-1")
		cases = append(cases, q("run max seq", "choir_events.run_id,choir_events.seq", query, args))
	}
	{
		query, args := eventTableNewestStreamSQL()
		cases = append(cases, q("store newest", "choir_events.tape_seq,choir_events.op_index", query, args))
	}
	for _, c := range cases {
		plan := explainEventTablePlan(t, s, c.query, c.args...)
		if !strings.Contains(plan, "IndexedTableAccess(choir_events)") || !strings.Contains(plan, "index: ["+c.index+"]") {
			t.Fatalf("%s does not read index [%s]:\n%s\n%s", c.name, c.index, c.query, plan)
		}
		if strings.Contains(plan, "Sort(") || strings.Contains(plan, "TopN") {
			t.Fatalf("%s sorts instead of reading in index order:\n%s", c.name, plan)
		}
	}
}

func explainEventTablePlan(t *testing.T, s *Store, query string, args ...any) string {
	t.Helper()
	rows, err := s.db.Query("EXPLAIN PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// F20: the three caller-less methods are gone (section 1.7), and the channel
// read carries its contract in its name (F10).
func TestEventTableDeletedMethodsAreGone(t *testing.T) {
	typ := reflect.TypeOf(&Store{})
	for _, name := range []string{"ListEventsByTrajectoryAfter", "ListEventsOG", "ListEventsByOwnerOG", "ListEventsByChannel"} {
		if _, ok := typ.MethodByName(name); ok {
			t.Fatalf("Store.%s still exists", name)
		}
	}
	if _, ok := typ.MethodByName("ListAcknowledgedEventsByChannel"); !ok {
		t.Fatal("Store.ListAcknowledgedEventsByChannel is missing")
	}
}

// F10: the channel read covers acknowledged kinds only; its one caller reads
// an acknowledged kind, which stays in og_objects.
func TestEventTableChannelReadIsAcknowledgedOnly(t *testing.T) {
	if types.EventSystem1Judgment.Durability() != types.DurabilityAcknowledged || types.EventSystem1Judgment.Storage() != types.StorageObjectGraph {
		t.Fatal("the System1 judgment must stay acknowledged and in og_objects")
	}
	ctx := context.Background()
	s := openProjectStore(t)
	ack := eventTableRecord("sys1", "run-c", "owner-1", "traj-c", types.EventSystem1Judgment, 1, 1, eventTableBase)
	ack.ChannelID = "system1:computer"
	projectAt(t, s, 2, eventTableOp(t, ack, false))
	got := mustEvents(t)(s.ListAcknowledgedEventsByChannel(ctx, "owner-1", "system1:computer", 10))
	if len(got) != 1 || got[0].EventID != "sys1" {
		t.Fatalf("channel read = %s", eventIDs(got))
	}
}

// F21: residue import never re-emits a choir_events row as an op.
func TestEventTableResidueSnapshotIgnoresTheTable(t *testing.T) {
	s := openProjectStore(t)
	rec := eventTableRecord("residue", "run-z", "owner-1", "traj-z", types.EventToolResult, 1, 1, eventTableBase)
	projectAt(t, s, 2, eventTableOp(t, rec, true))
	objs, err := s.snapshotResidueObjects(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, obj := range objs {
		if obj.ObjectKind == ogKindEvent {
			t.Fatalf("residue snapshot read a choir_events row: %s", obj.CanonicalID)
		}
	}
}

// Diagnostics (section 3.3 item 3): the replay fingerprints name
// choir_events rows by tape position.
func TestEventTableFingerprintsNameTheRows(t *testing.T) {
	s := openProjectStore(t)
	rec := eventTableRecord("fp", "run-f", "owner-1", "traj-f", types.EventToolResult, 1, 1, eventTableBase)
	projectAt(t, s, 5, eventTableOp(t, rec, true), eventTableOp(t, eventTableRecord("fp-2", "run-f", "owner-1", "traj-f", types.EventToolResult, 2, 2, eventTableBase), true))
	fps, err := s.ListObjectGraphFingerprints(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, fp := range fps {
		if fp.Table == choirEventsTable {
			names = append(names, fp.CanonicalID)
		}
	}
	sort.Strings(names)
	if want := []string{"choir_events/5/0", "choir_events/5/1"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("choir_events fingerprints %v, want %v", names, want)
	}
}

// F9's second look inside the sealer: a sealed op is unstamped or stamped
// choir_events, nothing else.
func TestEventTableSealerRecheckAdmitsOnlyTheEventTable(t *testing.T) {
	rec := eventTableRecord("seal", "run-s", "owner-1", "traj-s", types.EventToolResult, 1, 1, eventTableBase)
	for table, ok := range map[string]bool{"": true, choirEventsTable: true, "og_objects": false, "run_memory_entries": false} {
		op := eventTableOp(t, rec, false)
		op.Table = table
		if err := recheckTelemetryOp(op); (err == nil) != ok {
			t.Fatalf("recheck of Table %q = %v", table, err)
		}
	}
}
