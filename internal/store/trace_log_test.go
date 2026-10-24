package store

// Slice S2 of R7 (docs/design/trace-segment-log-2026-10-22.md, section 9):
// readers and seeds, inert. The store opens, closes and reopens the trace log
// beside its workspace; every event-list method merges a log half; the seeds
// consider the log; the setting is registered and not acted on; the restore
// boundary is written. Nothing writes a trace line, so with the log empty every
// answer must be what origin/main answers.
//
// Ways it can fail, written before the code (design section 10 first, then the
// ones S2 adds). Each has its test below.
//
//	F1   a seq or stream_seq is handed out twice across Dolt and the log after
//	     a reopen or a restore: TestTraceLogSeedsConsiderTheLog,
//	     TestReopenWritesOneRestoreBoundaryAndNeverReusesNumbers.
//	F2   a reader forgets the log half, or merges or cuts it wrongly:
//	     TestTraceLogSplitEqualsTheAllDoltOracle (every method, every cursor and
//	     limit, including cursor paging), TestTraceLogDedupesByEventIDKeepingTheDoltCopy,
//	     TestEveryEventListMethodIsOnTheUnion.
//	F2'  an empty log changes a page, a cursor or a seed from what main gives:
//	     TestTraceLogEmptyLogIsByteIdenticalToOriginMain (a golden made on main).
//	F11  the class leaks, or S2 writes a line: TestNothingWritesATraceLineInS2.
//	F12  two writers on one directory: TestTraceLogLockedByAnotherWriterRunsWithoutTheLog.
//	F17  the restore loses the log or reuses numbers: the Reopen test above.
//	F19  the log sits inside the workspace restore quarantines:
//	     TestTraceLogIsBesideTheWorkspaceNotInsideIt.
//	F20  a fork inherits the parent's log: the path is derived from the store's
//	     own marker path (same test).
//	F22  the boot path grows a resumer or a file: TestTraceLogIsNotCreatedByOpen.
//	S2a  an unopenable log fails the store's Open, or a bad segment fails a read:
//	     TestTraceLogOpenFailureNeverFailsTheStore, TestTraceLogUnreadableSegmentDegrades.
//	S2b  the setting does anything: TestSettingTraceLogIsRegisteredAndNotActedOn.
//	S2c  the observability block lacks a field the proof reads, or lies about
//	     poison: TestTraceLogStatsBlock, TestTraceLogStatsReportPoison.
//	S2d  a close leaves the directory locked, so the reopen runs without the
//	     log: TestTraceLogCloseReleasesTheDirectoryAndReopenTakesItBack.
//	S2e  a racing reader sees a torn handle: TestTraceLogReadsRaceAppendsAndEnsure.
//	S2f  the ErrLocked store reads a foreign process's lines as its own: same test
//	     as F12.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tracelog"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func traceLogStorePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func openTraceLogStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// injectTraceLine puts a record in the log, as S4's writer will: its JSON as
// the line and the envelope from the record.
func injectTraceLine(t *testing.T, s *Store, rec types.EventRecord) {
	t.Helper()
	h := s.traceLogHandle()
	if h == nil || h.log == nil {
		t.Fatal("the store has no open trace log")
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	m := tracelog.Meta{EventID: rec.EventID, RunID: rec.RunID, OwnerID: rec.OwnerID, TrajectoryID: rec.TrajectoryID,
		Kind: string(rec.Kind), Seq: rec.Seq, StreamSeq: rec.StreamSeq, Time: rec.Timestamp, BootID: "test"}
	if err := h.log.Append(m, raw); err != nil {
		t.Fatalf("append to the trace log: %v", err)
	}
}

func traceLogBoundaries(t *testing.T, dir string) []string {
	t.Helper()
	var reasons []string
	err := tracelog.OpenReader(dir).Scan(func(it tracelog.Item) bool {
		if it.Boundary != nil {
			reasons = append(reasons, it.Boundary.Reason)
		}
		return true
	})
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	return reasons
}

func countReason(reasons []string, reason string) int {
	n := 0
	for _, r := range reasons {
		if r == reason {
			n++
		}
	}
	return n
}

// F22: Open adds no file and no resumer. The directory appears when something
// needs it (a restore boundary now, the writer in S4), so a base blob packed
// from a scratch store, an install into an "empty" store directory and every
// test directory look as they did.
func TestTraceLogIsNotCreatedByOpen(t *testing.T) {
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if _, err := os.Stat(traceLogDirFor(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Open created %s (stat err %v)", traceLogDirFor(path), err)
	}
	if st := s.TraceLogStats(); st.Open || st.OpenError != "" {
		t.Fatalf("a store with no log directory reports %+v", st)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(traceLogDirFor(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Close created %s", traceLogDirFor(path))
	}
}

// F19, F20: the log is the store marker's sibling, never inside the Dolt
// workspace that restore quarantines and swaps, and it follows the store's own
// path (a fork has its own).
func TestTraceLogIsBesideTheWorkspaceNotInsideIt(t *testing.T) {
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	dir := traceLogDirFor(path)
	if want := filepath.Join(filepath.Dir(path), "trace-log"); dir != want {
		t.Fatalf("trace log dir = %q, want %q", dir, want)
	}
	workspace := s.TexturePath()
	if rel, err := filepath.Rel(workspace, dir); err == nil && !strings.HasPrefix(rel, "..") {
		t.Fatalf("the trace log %s is inside the workspace %s", dir, workspace)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("trace log dir: %v, %v", info, err)
	}
	if got := s.TraceLogStats(); !got.Open || got.Dir != dir {
		t.Fatalf("stats = %+v, want open at %s", got, dir)
	}
	// A second store has its own log.
	other := traceLogStorePath(t, "state")
	if traceLogDirFor(other) == dir {
		t.Fatalf("two store paths share a trace log directory: %s", dir)
	}
}

// S2d: Close seals the log and releases the directory; the next Open takes the
// log back when its directory exists; an idempotent Close is harmless.
func TestTraceLogCloseReleasesTheDirectoryAndReopenTakesItBack(t *testing.T) {
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	rec := types.EventRecord{EventID: "l-1", RunID: "run-x", OwnerID: "owner-1", Kind: types.EventToolResult, Seq: 1, StreamSeq: 1, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
	injectTraceLine(t, s, rec)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	l, err := tracelog.Open(tracelog.Options{Dir: traceLogDirFor(path), ComputerID: "probe", SyncInterval: -1})
	if err != nil {
		t.Fatalf("the directory is still locked after Close: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	s2 := openTraceLogStore(t, path)
	if st := s2.TraceLogStats(); !st.Open {
		t.Fatalf("a store opened over an existing log directory has no log: %+v", st)
	}
	got, err := s2.ListEventsAfter(context.Background(), "run-x", 0, 10)
	if err != nil || len(got) != 1 || got[0].EventID != "l-1" {
		t.Fatalf("the sealed line is not served after a reopen: %v, %v", got, err)
	}
}

// F2': the differential against origin/main. The golden file was made by
// running traceLogReadMatrix and traceLogSeedProbe on the commit before any S2
// code. The log is absent in one run and open but empty in the other; both
// must hash to the golden answers, page for page and seed for seed.
func TestTraceLogEmptyLogIsByteIdenticalToOriginMain(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "trace_log_empty_matrix.golden"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		name, value, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("bad golden line %q", line)
		}
		want[name] = value
	}
	for _, variant := range []string{"no log directory", "open empty log"} {
		t.Run(variant, func(t *testing.T) {
			path := traceLogStorePath(t, "state")
			s := openTraceLogStore(t, path)
			if variant == "open empty log" {
				if err := s.ensureTraceLog(); err != nil {
					t.Fatal(err)
				}
			}
			recs := appendTraceLogScript(t, s)
			got := map[string]string{}
			for name, page := range traceLogReadMatrix(t, s, recs) {
				got[name] = fmt.Sprintf("%x", sha256.Sum256([]byte(page)))
			}
			for name, v := range traceLogSeedProbe(t, s) {
				got[name] = v
			}
			if len(got) != len(want) {
				t.Fatalf("%d answers, golden has %d", len(got), len(want))
			}
			var bad []string
			for name, v := range want {
				if got[name] != v {
					bad = append(bad, name)
				}
			}
			sort.Strings(bad)
			if len(bad) > 0 {
				t.Fatalf("%d of %d answers differ from origin/main, first: %q", len(bad), len(want), bad[0])
			}
		})
	}
}

// F2: the oracle. Store A holds a script entirely in Dolt. Store B holds the
// same records, numbers and timestamps, with every trace-class record in the
// log and none in Dolt. Every list method must answer the same on both.
func TestTraceLogSplitEqualsTheAllDoltOracle(t *testing.T) {
	ctx := context.Background()
	a := openTraceLogStore(t, traceLogStorePath(t, "a"))
	recs := appendTraceLogScript(t, a)

	b := openTraceLogStore(t, traceLogStorePath(t, "b"))
	if err := b.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	inLog, inDolt := 0, 0
	for _, rec := range recs {
		rec := rec
		if rec.Kind.Storage() == types.StorageTraceLog {
			injectTraceLine(t, b, rec)
			inLog++
			continue
		}
		if err := b.AppendEventOG(ctx, &rec); err != nil {
			t.Fatal(err)
		}
		inDolt++
	}
	if inLog < 40 || inDolt < 20 {
		t.Fatalf("the script is not a real split: %d in the log, %d in Dolt", inLog, inDolt)
	}

	wantMatrix := traceLogReadMatrix(t, a, recs)
	gotMatrix := traceLogReadMatrix(t, b, recs)
	if len(gotMatrix) != len(wantMatrix) {
		t.Fatalf("%d answers, want %d", len(gotMatrix), len(wantMatrix))
	}
	for name, want := range wantMatrix {
		if got := gotMatrix[name]; got != want {
			t.Fatalf("%s\nsplit: %s\nall-Dolt: %s", name, got, want)
		}
	}

	// The cursor walk a client does: page by page until empty, on the run
	// route's seq cursor and the stream's stream_seq cursor.
	walkRun := func(s *Store) []string {
		var ids []string
		for after := int64(0); ; {
			page, err := s.ListEventsAfter(ctx, "run-a", after, 4)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) == 0 {
				return ids
			}
			for _, r := range page {
				ids = append(ids, r.EventID)
				after = max(after, r.Seq)
			}
		}
	}
	walkStream := func(s *Store) []string {
		var ids []string
		for after := int64(0); ; {
			page, err := s.ListEventsByOwnerAfter(ctx, "owner-1", after, 5)
			if err != nil {
				t.Fatal(err)
			}
			if len(page) == 0 {
				return ids
			}
			for _, r := range page {
				ids = append(ids, r.EventID)
				after = r.StreamSeq
			}
		}
	}
	if g, w := walkRun(b), walkRun(a); fmt.Sprint(g) != fmt.Sprint(w) {
		t.Fatalf("run cursor walk differs:\nsplit %v\noracle %v", g, w)
	}
	if g, w := walkStream(b), walkStream(a); fmt.Sprint(g) != fmt.Sprint(w) || len(w) == 0 {
		t.Fatalf("stream cursor walk differs:\nsplit %v\noracle %v", g, w)
	}
}

// F2: a record in both halves (a rollback window, section 8) is served once,
// and the copy kept is Dolt's.
func TestTraceLogDedupesByEventIDKeepingTheDoltCopy(t *testing.T) {
	ctx := context.Background()
	s := openTraceLogStore(t, traceLogStorePath(t, "state"))
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	dolt := types.EventRecord{EventID: "dup-1", RunID: "run-d", OwnerID: "owner-1", TrajectoryID: "traj-d", Kind: types.EventToolResult,
		Seq: 1, StreamSeq: 11, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{"from":"dolt"}`)}
	if err := s.AppendEventOG(ctx, &dolt); err != nil {
		t.Fatal(err)
	}
	logCopy := dolt
	logCopy.Payload = json.RawMessage(`{"from":"log"}`)
	injectTraceLine(t, s, logCopy)
	only := types.EventRecord{EventID: "only-log", RunID: "run-d", OwnerID: "owner-1", TrajectoryID: "traj-d", Kind: types.EventRunProgress,
		Seq: 2, StreamSeq: 12, Timestamp: traceLogScriptBase.Add(time.Second), Payload: json.RawMessage(`{}`)}
	injectTraceLine(t, s, only)

	check := func(name string, got []types.EventRecord, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != 2 || got[0].EventID != "dup-1" && got[1].EventID != "dup-1" {
			t.Fatalf("%s = %v, want dup-1 once and only-log", name, got)
		}
		n := 0
		for _, r := range got {
			if r.EventID == "dup-1" {
				n++
				if string(r.Payload) != `{"from":"dolt"}` {
					t.Fatalf("%s kept the log copy: %s", name, r.Payload)
				}
			}
		}
		if n != 1 {
			t.Fatalf("%s served dup-1 %d times", name, n)
		}
	}
	l1, e1 := s.ListEventsAfter(ctx, "run-d", 0, 10)
	check("ListEventsAfter", l1, e1)
	l2, e2 := s.ListEventsByOwner(ctx, "owner-1", 10)
	check("ListEventsByOwner", l2, e2)
	l3, e3 := s.ListEventsByOwnerAfter(ctx, "owner-1", 0, 10)
	check("ListEventsByOwnerAfter", l3, e3)
	l4, e4 := s.ListEventsByTrajectory(ctx, "owner-1", "traj-d", 10)
	check("ListEventsByTrajectory", l4, e4)
}

// F2: a source guard. Every ListEvents* method of the store is on the union
// (calls a traceLogHalf helper) or delegates to one that is; the one reader
// that serves acknowledged kinds only is named.
func TestEveryEventListMethodIsOnTheUnion(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "store.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile("store.go")
	seen := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || !strings.HasPrefix(fn.Name.Name, "ListEvents") && !strings.HasPrefix(fn.Name.Name, "ListAcknowledgedEvents") {
			continue
		}
		seen[fn.Name.Name] = true
		body := string(src[fset.Position(fn.Body.Pos()).Offset:fset.Position(fn.Body.End()).Offset])
		switch fn.Name.Name {
		case "ListEvents":
			if !strings.Contains(body, "s.ListEventsAfter(") {
				t.Errorf("ListEvents no longer delegates to ListEventsAfter")
			}
		case "ListAcknowledgedEventsByChannel":
			// Acknowledged kinds are never in the log (design 6.1).
			if strings.Contains(body, "traceLogHalf") {
				t.Errorf("%s reads the log, but it serves acknowledged kinds only", fn.Name.Name)
			}
		default:
			if !strings.Contains(body, "s.traceLogHalf") {
				t.Errorf("%s does not merge the trace-log half", fn.Name.Name)
			}
		}
	}
	for _, name := range []string{"ListEvents", "ListEventsAfter", "ListEventsByOwner", "ListEventsByOwnerAfter", "ListEventsByTrajectory", "ListAcknowledgedEventsByChannel"} {
		if !seen[name] {
			t.Errorf("store.go has no %s; the guard needs updating", name)
		}
	}
}

// F1: the seeds. A line the log holds must keep its number from being handed
// out again after a restart, for the per-run seq and for the store-wide
// stream_seq; a run the log does not hold is seeded exactly as before.
func TestTraceLogSeedsConsiderTheLog(t *testing.T) {
	ctx := context.Background()
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	appendOne := func(s *Store, run, owner, id string) *types.EventRecord {
		t.Helper()
		rec := &types.EventRecord{EventID: id, RunID: run, OwnerID: owner, Kind: types.EventRunStarted, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
		if err := s.AppendEvent(ctx, rec); err != nil {
			t.Fatal(err)
		}
		return rec
	}
	for i := 0; i < 3; i++ {
		appendOne(s, "run-a", "owner-1", fmt.Sprintf("a-%d", i))
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	// A previous process wrote lines the tape does not have: run-a at seq 500,
	// another owner's run-x at seq 900, and a stream_seq far above the lease.
	dir := traceLogDirFor(path)
	l, err := tracelog.Open(tracelog.Options{Dir: dir, ComputerID: "probe", SyncInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	const farStream = int64(9_000_000_000)
	for _, m := range []tracelog.Meta{
		{EventID: "log-a", RunID: "run-a", OwnerID: "owner-1", Kind: "tool.result", Seq: 500, StreamSeq: farStream - 10, Time: traceLogScriptBase},
		{EventID: "log-x", RunID: "run-x", OwnerID: "owner-2", Kind: "tool.result", Seq: 900, StreamSeq: farStream, Time: traceLogScriptBase},
	} {
		if err := l.Append(m, []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}

	s2 := openTraceLogStore(t, path)
	a := appendOne(s2, "run-a", "owner-1", "a-next")
	if want := 500 + telemetrySeqGap + 1; a.Seq != want {
		t.Fatalf("run-a seq after a log line at 500 = %d, want %d (log maximum plus the gap)", a.Seq, want)
	}
	if a.StreamSeq <= farStream {
		t.Fatalf("stream_seq %d after a log line at %d: a number would be handed out twice", a.StreamSeq, farStream)
	}
	// The owner is part of the key: owner-1's run-x is not owner-2's.
	if x := appendOne(s2, "run-x", "owner-1", "x-1"); x.Seq != 1 {
		t.Fatalf("owner-1 run-x seq = %d, want 1 (the log line belongs to owner-2)", x.Seq)
	}
	if x := appendOne(s2, "run-x", "owner-2", "x-2"); x.Seq != 900+telemetrySeqGap+1 {
		t.Fatalf("owner-2 run-x seq = %d, want %d", x.Seq, 900+telemetrySeqGap+1)
	}
	if n := appendOne(s2, "run-new", "owner-1", "n-1"); n.Seq != 1 {
		t.Fatalf("a run the log has never seen starts at %d, want 1", n.Seq)
	}
}

// F1, F17: Reopen is the swap that follows a restore (rematerialize,
// projection base install). It reopens the log, which the swap did not touch,
// writes one restore boundary per Reopen, and the numbers stay ahead of the
// log's whatever the swapped workspace holds.
func TestReopenWritesOneRestoreBoundaryAndNeverReusesNumbers(t *testing.T) {
	ctx := context.Background()
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	var last types.EventRecord
	for i := 0; i < 4; i++ {
		rec := &types.EventRecord{EventID: fmt.Sprintf("r-%d", i), RunID: "run-r", OwnerID: "owner-1", Kind: types.EventRunStarted, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
		if err := s.AppendEvent(ctx, rec); err != nil {
			t.Fatal(err)
		}
		last = *rec
	}
	// Lines the restored workspace will not have: the log is ahead of it.
	ahead := types.EventRecord{EventID: "ahead-1", RunID: "run-r", OwnerID: "owner-1", TrajectoryID: "traj-r", Kind: types.EventToolResult,
		Seq: last.Seq + 10, StreamSeq: last.StreamSeq + 10, Timestamp: traceLogScriptBase.Add(time.Minute), Payload: json.RawMessage(`{}`)}
	injectTraceLine(t, s, ahead)

	dir := traceLogDirFor(path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if n := countReason(traceLogBoundaries(t, dir), tracelog.ReasonRestore); n != 0 {
		t.Fatalf("%d restore boundaries before any Reopen", n)
	}
	// The swap. Reopen adopts the (here unchanged) workspace and reopens the log.
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if n := countReason(traceLogBoundaries(t, dir), tracelog.ReasonRestore); n != 1 {
		t.Fatalf("%d restore boundaries after one Reopen, want 1", n)
	}
	// The line that outlived the swap is still served.
	got, err := s.ListEventsAfter(ctx, "run-r", last.Seq, 10)
	if err != nil || len(got) != 1 || got[0].EventID != "ahead-1" {
		t.Fatalf("after Reopen the log's line is %v, %v", got, err)
	}
	// Within the process the in-memory marks carry across the swap.
	next := &types.EventRecord{EventID: "r-after", RunID: "run-r", OwnerID: "owner-1", Kind: types.EventRunStarted, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
	if err := s.AppendEvent(ctx, next); err != nil {
		t.Fatal(err)
	}
	if next.Seq <= last.Seq || next.StreamSeq <= last.StreamSeq {
		t.Fatalf("seq %d / stream_seq %d after Reopen reuse %d / %d", next.Seq, next.StreamSeq, last.Seq, last.StreamSeq)
	}
	// And a process that starts after the restore seeds from the log too.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2 := openTraceLogStore(t, path)
	later := &types.EventRecord{EventID: "r-later", RunID: "run-r", OwnerID: "owner-1", Kind: types.EventRunStarted, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
	if err := s2.AppendEvent(ctx, later); err != nil {
		t.Fatal(err)
	}
	if later.Seq <= ahead.Seq || later.StreamSeq <= ahead.StreamSeq {
		t.Fatalf("a boot after the restore hands out seq %d / stream_seq %d, but the log holds %d / %d", later.Seq, later.StreamSeq, ahead.Seq, ahead.StreamSeq)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if n := countReason(traceLogBoundaries(t, dir), tracelog.ReasonRestore); n != 2 {
		t.Fatalf("%d restore boundaries after two Reopens, want 2", n)
	}
	if st := s.TraceLogStats(); !st.Open {
		t.Fatalf("the log is not open after Reopen: %+v", st)
	}
}

// A store whose log directory never existed gets one at Reopen (a restore into
// a computer that never wrote trace lines still leaves its boundary).
func TestReopenCreatesTheLogForTheBoundaryWhenThereWasNone(t *testing.T) {
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if n := countReason(traceLogBoundaries(t, traceLogDirFor(path)), tracelog.ReasonRestore); n != 1 {
		t.Fatalf("%d restore boundaries, want 1", n)
	}
	// Boundaries are skipped by readers: nothing is served.
	got, err := s.ListEventsByOwner(context.Background(), "owner-1", 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("ListEventsByOwner over a log with only a boundary = %v, %v", got, err)
	}
	if st := s.TraceLogStats(); st.Records != 0 || st.RetentionFloor != nil {
		t.Fatalf("a boundary counted as a line: %+v", st)
	}
}

// S2a: the log is never a reason the store does not open.
func TestTraceLogOpenFailureNeverFailsTheStore(t *testing.T) {
	path := traceLogStorePath(t, "state")
	if err := os.WriteFile(traceLogDirFor(path), []byte("a file where the directory should be"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := openTraceLogStore(t, path) // the open must succeed
	st := s.TraceLogStats()
	if st.Open || st.OpenError == "" {
		t.Fatalf("stats = %+v, want not open with an error", st)
	}
	rec := &types.EventRecord{EventID: "e-1", RunID: "run-o", OwnerID: "owner-1", Kind: types.EventToolResult, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
	if err := s.AppendEvent(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListEventsAfter(context.Background(), "run-o", 0, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("lists without a log: %v, %v", got, err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// And a Reopen that cannot write its boundary does not fail either.
	if err := s.Reopen(path); err != nil {
		t.Fatalf("Reopen with an unusable log directory: %v", err)
	}
}

// F12, S2f: another writer holds the directory. The store runs without the log
// (as if the setting were off) and does not read the other writer's lines as
// its own.
func TestTraceLogLockedByAnotherWriterRunsWithoutTheLog(t *testing.T) {
	path := traceLogStorePath(t, "state")
	l, err := tracelog.Open(tracelog.Options{Dir: traceLogDirFor(path), ComputerID: "other", SyncInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Append(tracelog.Meta{EventID: "foreign", RunID: "run-f", OwnerID: "owner-1", Kind: "tool.result", Seq: 1, StreamSeq: 1, Time: traceLogScriptBase}, []byte(`{"event_id":"foreign"}`)); err != nil {
		t.Fatal(err)
	}
	s := openTraceLogStore(t, path)
	st := s.TraceLogStats()
	if st.Open || !st.Locked {
		t.Fatalf("stats = %+v, want not open and locked", st)
	}
	got, err := s.ListEventsAfter(context.Background(), "run-f", 0, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("a locked store served the other writer's lines: %v, %v", got, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if got := s.TraceLogStats(); got.Open {
		t.Fatalf("the store took the log without being asked: %+v", got)
	}
}

// S2a: a segment this binary cannot read costs that segment's lines and counts
// an error; the rest of the page is served.
func TestTraceLogUnreadableSegmentDegrades(t *testing.T) {
	ctx := context.Background()
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	dolt := types.EventRecord{EventID: "d-1", RunID: "run-u", OwnerID: "owner-1", Kind: types.EventRunStarted, Seq: 1, StreamSeq: 1, Timestamp: traceLogScriptBase, Payload: json.RawMessage(`{}`)}
	if err := s.AppendEventOG(ctx, &dolt); err != nil {
		t.Fatal(err)
	}
	injectTraceLine(t, s, types.EventRecord{EventID: "l-1", RunID: "run-u", OwnerID: "owner-1", Kind: types.EventToolResult, Seq: 2, StreamSeq: 2, Timestamp: traceLogScriptBase.Add(time.Second), Payload: json.RawMessage(`{}`)})
	// A segment written by a newer binary: our magic, a version we do not know.
	future := append([]byte("CHTRACE1"), 0, 0, 0, 99)
	future = append(future, make([]byte, 64)...)
	if err := os.WriteFile(filepath.Join(traceLogDirFor(path), "seg-00000099.tlog"), future, 0o600); err != nil {
		t.Fatal(err)
	}
	before := s.TraceLogStats().ReadErrors
	got, err := s.ListEventsAfter(ctx, "run-u", 0, 10)
	if err != nil {
		t.Fatalf("a segment of an unknown version failed the read: %v", err)
	}
	if len(got) != 2 || got[0].EventID != "d-1" || got[1].EventID != "l-1" {
		t.Fatalf("page = %v, want d-1 then l-1", got)
	}
	if after := s.TraceLogStats().ReadErrors; after <= before {
		t.Fatalf("read_errors %d -> %d: the failure was not counted", before, after)
	}
}

// F11: nothing writes a trace line in S2, whatever the kind and whatever the
// setting says. And only the log file of this package touches the writer.
func TestNothingWritesATraceLineInS2(t *testing.T) {
	ctx := context.Background()
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	if err := s.PutComputerSetting(ctx, ComputerSetting{OwnerID: "owner-1", ComputerID: "computer-1", Key: SettingTraceLog, Value: true}); err != nil {
		t.Fatal(err)
	}
	for i, kind := range types.EventKinds() {
		rec := &types.EventRecord{EventID: fmt.Sprintf("k-%d", i), RunID: "run-k", OwnerID: "owner-1", Kind: kind, Timestamp: traceLogScriptBase.Add(time.Duration(i) * time.Second), Payload: json.RawMessage(`{}`)}
		if err := s.AppendEvent(ctx, rec); err != nil {
			t.Fatalf("append %s: %v", kind, err)
		}
	}
	st := s.TraceLogStats()
	if st.Records != 0 || st.Bytes != 0 {
		t.Fatalf("the log took %d records (%d bytes) in S2", st.Records, st.Bytes)
	}
	if sinks := s.TelemetryStats().EventsBySink["trace_log"]; len(sinks) != 0 {
		t.Fatalf("events_by_sink.trace_log = %v, want none", sinks)
	}
	// Source guard: no production file of this package calls Append on the log.
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		raw, _ := os.ReadFile(f)
		for _, call := range []string{"log.Append(", "log.Sync(", "Log.Append("} {
			if strings.Contains(string(raw), call) {
				t.Errorf("%s calls %s: the writer is slice S4", f, call)
			}
		}
	}
}

// S2b: store.event_table's neighbour. The setting is a known computer setting
// with default off, an owner can store it, and nothing reads it.
func TestSettingTraceLogIsRegisteredAndNotActedOn(t *testing.T) {
	if SettingTraceLog != "store.trace_log" {
		t.Fatalf("SettingTraceLog = %q", SettingTraceLog)
	}
	if def, known := KnownComputerSettings[SettingTraceLog]; !known || def {
		t.Fatalf("store.trace_log known=%v default=%v, want known and off", known, def)
	}
	ctx := context.Background()
	s := openTraceLogStore(t, traceLogStorePath(t, "state"))
	if err := s.PutComputerSetting(ctx, ComputerSetting{OwnerID: "owner-1", ComputerID: "computer-1", Key: SettingTraceLog, Value: true}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetComputerSetting(ctx, "owner-1", "computer-1", SettingTraceLog)
	if err != nil || !got.Value {
		t.Fatalf("setting = %+v, %v", got, err)
	}
	// Nothing outside the registry and tests names it: no branch reads it.
	root := filepath.Join("..", "..")
	var readers []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "docs", "frontend", "scratchpad":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		if strings.Contains(string(raw), "SettingTraceLog") || strings.Contains(string(raw), `"store.trace_log"`) {
			readers = append(readers, filepath.ToSlash(p))
		}
		return nil
	})
	if len(readers) != 1 || !strings.HasSuffix(readers[0], "internal/store/computer_setting.go") {
		t.Fatalf("store.trace_log is named outside its registration: %v (S4 is the slice that acts on it)", readers)
	}
}

// S2c: the observability block the proof reads (design 9.1 step 4 (f), (g)).
func TestTraceLogStatsBlock(t *testing.T) {
	path := traceLogStorePath(t, "state")
	s := openTraceLogStore(t, path)
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		injectTraceLine(t, s, types.EventRecord{EventID: fmt.Sprintf("s-%d", i), RunID: "run-s", OwnerID: "owner-1", Kind: types.EventToolResult,
			Seq: int64(i), StreamSeq: int64(i), Timestamp: traceLogScriptBase.Add(time.Duration(i) * time.Second), Payload: json.RawMessage(`{}`)})
	}
	injectTraceLine(t, s, types.EventRecord{EventID: "s-4", RunID: "run-s", OwnerID: "owner-1", Kind: types.EventDeskCell,
		Seq: 4, StreamSeq: 4, Timestamp: traceLogScriptBase.Add(4 * time.Second), Payload: json.RawMessage(`{}`)})
	if err := s.traceLogHandle().log.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []time.Duration{time.Millisecond, 2 * time.Millisecond, 9 * time.Millisecond} {
		s.recordTraceLogBarrier(d)
	}

	st := s.TraceLogStats()
	if !st.Open || st.Records != 4 || st.RecordsByKind["tool.result"] != 3 || st.RecordsByKind["desk.cell"] != 1 {
		t.Fatalf("records: %+v", st)
	}
	if st.BytesByKind["tool.result"] <= 0 || st.Bytes <= 0 || st.Fsyncs < 1 {
		t.Fatalf("bytes and fsyncs: %+v", st)
	}
	if st.BarrierFsyncs != 3 || st.BarrierFsyncP99MS < 8.9 || st.BarrierFsyncP99MS > 9.1 {
		t.Fatalf("barrier fsyncs %d p99 %.2f ms, want 3 and 9", st.BarrierFsyncs, st.BarrierFsyncP99MS)
	}
	if st.Segments != 1 || st.RetainedBytes <= 0 {
		t.Fatalf("segments %d retained %d", st.Segments, st.RetainedBytes)
	}
	if st.RetentionFloor == nil || st.RetentionFloor.StreamSeq != 1 || st.RetentionFloor.Segment != 1 {
		t.Fatalf("retention floor %+v, want the first line", st.RetentionFloor)
	}
	if st.Dropped != 0 || st.Fallbacks != 0 || st.Poisoned || st.ReadErrors != 0 {
		t.Fatalf("dropped %d fallbacks %d poisoned %v read errors %d", st.Dropped, st.Fallbacks, st.Poisoned, st.ReadErrors)
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var block map[string]any
	if err := json.Unmarshal(raw, &block); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"open", "dir", "records", "bytes", "records_by_kind", "bytes_by_kind", "fsyncs", "barrier_fsyncs",
		"barrier_fsync_p99_ms", "segments", "retained_bytes", "retention_floor", "dropped", "fallbacks", "poisoned", "read_errors"} {
		if _, ok := block[key]; !ok {
			t.Errorf("trace_log block lacks %q: %s", key, raw)
		}
	}
	// The floor is a line the log holds; once the log is empty the key is null.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

// S2c: a poisoned log says so, and says it before anything is lost.
func TestTraceLogStatsReportPoison(t *testing.T) {
	old := testTraceLogHooks
	t.Cleanup(func() { testTraceLogHooks = old })
	testTraceLogHooks = tracelog.Hooks{SyncFile: func(*os.File) error { return syscall.EIO }}
	s := openTraceLogStore(t, traceLogStorePath(t, "state"))
	if err := s.ensureTraceLog(); err != nil {
		t.Fatal(err)
	}
	// The segment header's fsync is the first one: it fails on the first line.
	line := tracelog.Meta{EventID: "p-1", RunID: "r", OwnerID: "o", Kind: "tool.result", Seq: 1, StreamSeq: 1, Time: traceLogScriptBase}
	if err := s.traceLogHandle().log.Append(line, []byte(`{}`)); err == nil {
		t.Fatal("the append survived a failing fsync")
	}
	st := s.TraceLogStats()
	if !st.Poisoned || st.PoisonError == "" {
		t.Fatalf("stats after a failed fsync = %+v, want poisoned", st)
	}
	// A poisoned log still serves what it holds and the store still lists.
	if _, err := s.ListEventsByOwner(context.Background(), "o", 5); err != nil {
		t.Fatalf("a poisoned log failed a read: %v", err)
	}
}

// S2e: readers take the handle with one atomic load. Run under -race.
func TestTraceLogReadsRaceAppendsAndEnsure(t *testing.T) {
	ctx := context.Background()
	s := openTraceLogStore(t, traceLogStorePath(t, "state"))
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := s.ListEventsAfter(ctx, "run-race", 0, 20); err != nil {
					t.Errorf("ListEventsAfter: %v", err)
					return
				}
				_, _ = s.ListEventsByOwner(ctx, "owner-1", 5)
				_, _ = s.ListEventsByOwnerAfter(ctx, "owner-1", 0, 5)
				_, _ = s.ListEventsByTrajectory(ctx, "owner-1", "traj-race", 5)
				_ = s.TraceLogStats()
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			_ = s.ensureTraceLog()
			time.Sleep(time.Millisecond)
		}
	}()
	time.Sleep(20 * time.Millisecond)
	for i := 1; i <= 200; i++ {
		injectTraceLine(t, s, types.EventRecord{EventID: fmt.Sprintf("race-%d", i), RunID: "run-race", OwnerID: "owner-1", TrajectoryID: "traj-race", Kind: types.EventToolResult,
			Seq: int64(i), StreamSeq: int64(i), Timestamp: traceLogScriptBase.Add(time.Duration(i) * time.Second), Payload: json.RawMessage(`{}`)})
	}
	close(stop)
	wg.Wait()
	got, err := s.ListEventsAfter(ctx, "run-race", 0, 1000)
	if err != nil || len(got) != 200 {
		t.Fatalf("after the race the run has %d events, err %v", len(got), err)
	}
}
