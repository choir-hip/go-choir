package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Failure modes for the telemetry sealer are listed, with the test that covers
// each, in docs/reports/telemetry-batching-design-2026-10-20.md ("Implementation
// plan and failure modes"). The tests here were written before the sealer.

const (
	telemetryAuthority = "authority:vm-local-projection-telemetry"
	ordinaryAuthority  = "authority:vm-local-projection"
	farFuture          = time.Hour // a window that never fires inside a test
)

// ---- P1-F1: a semantic kind can never be batched ---------------------------

// Type level: telemetryRecord has only unexported fields and is built only by
// newTelemetryRecord, so no code outside this package can put an op in the
// queue and no code inside it does so by accident.
func TestTelemetryRecordIsUnforgeable(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	structSeen := false
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Name.Name != "telemetryRecord" {
						continue
					}
					structSeen = true
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						t.Fatalf("telemetryRecord is not a struct")
					}
					for _, f := range st.Fields.List {
						for _, n := range f.Names {
							if n.IsExported() {
								t.Errorf("telemetryRecord has exported field %s: a caller outside the package could forge a record", n.Name)
							}
						}
						if len(f.Names) == 0 {
							t.Errorf("telemetryRecord embeds a type; embedded fields can be promoted and set")
						}
					}
				}
			case *ast.FuncDecl:
				if d.Name.Name == "newTelemetryRecord" {
					continue
				}
				ast.Inspect(d, func(n ast.Node) bool {
					if lit, ok := n.(*ast.CompositeLit); ok {
						if id, ok := lit.Type.(*ast.Ident); ok && id.Name == "telemetryRecord" {
							t.Errorf("%s: %s builds a telemetryRecord literal; only newTelemetryRecord may", name, d.Name.Name)
						}
					}
					return true
				})
			}
		}
	}
	if !structSeen {
		t.Fatal("telemetryRecord type not found")
	}
}

// Runtime: every acknowledged kind is refused by the constructor, and with the
// flag on it neither enters the queue nor leaves the ordinary authority.
func TestSemanticKindsNeverEnterTheQueue(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	batched, acknowledged := 0, 0
	for i, kind := range types.EventKinds() {
		rec := h.event(fmt.Sprintf("kind-%d", i), "run-kinds", "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Millisecond))
		if kind.Batchable() {
			batched++
			if _, err := newTelemetryRecord(rec); err != nil {
				t.Errorf("newTelemetryRecord(%s): %v", kind, err)
			}
			continue
		}
		acknowledged++
		if _, err := newTelemetryRecord(rec); err == nil {
			t.Errorf("newTelemetryRecord accepted acknowledged kind %s", kind)
		}
		before := len(h.tape.snapshot())
		if err := h.store.AppendEvent(ctx, rec); err != nil {
			t.Fatalf("append %s: %v", kind, err)
		}
		if depth := sl.depth(); depth != 0 {
			t.Fatalf("acknowledged kind %s left %d records queued", kind, depth)
		}
		after := h.tape.snapshot()
		if len(after) != before+1 || after[len(after)-1].Authority != ordinaryAuthority {
			t.Fatalf("acknowledged kind %s: tape grew %d -> %d, last authority %q", kind, before, len(after), after[len(after)-1].Authority)
		}
	}
	if batched != 10 || acknowledged != 33 {
		t.Fatalf("exercised %d batched and %d acknowledged kinds, want 10 and 33", batched, acknowledged)
	}
	// The unknown kind is acknowledged too.
	if _, err := newTelemetryRecord(h.event("unknown", "run-kinds", "owner-1", types.EventKind("loop.made_up"), telemetryBase)); err == nil {
		t.Fatal("newTelemetryRecord accepted a kind outside the table")
	}
}

// P1-F2: a forged record, put in the queue by a test, is caught by the
// re-check at seal time; the whole batch falls back to singles under the
// ordinary authority and the violation is counted.
func TestSealTimeRecheckRefusesViolation(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	good := h.event("good-1", "run-v", "owner-1", types.EventRunProgress, telemetryBase)
	if err := h.store.AppendEvent(ctx, good); err != nil {
		t.Fatal(err)
	}
	forged := h.event("forged-1", "run-v", "owner-1", types.EventRunCompleted, telemetryBase.Add(time.Second))
	forged.Seq, forged.StreamSeq = 2, 2
	obj, err := buildEventObject(forged, forged.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(obj)
	sl.mu.Lock()
	sl.queue = append(sl.queue, telemetryRecord{
		kind: forged.Kind, runID: forged.RunID, ownerID: forged.OwnerID, seq: 2, streamSeq: 2,
		object: obj, op: computerevent.ProjectionOp{Kind: computerevent.ProjectionOpObject, CanonicalID: obj.CanonicalID, Body: body},
		size: len(body), n: sl.enqueuedN + 1,
	})
	sl.enqueuedN++
	sl.mu.Unlock()

	sl.drain()
	stats := h.store.TelemetryStats().Batching
	if stats.ClassViolations != 1 {
		t.Fatalf("telemetry_class_violation = %d, want 1", stats.ClassViolations)
	}
	for _, entry := range h.shape() {
		if entry.Authority == telemetryAuthority {
			t.Fatalf("a telemetry-authority batch was written after a class violation: %+v", entry)
		}
	}
	got := map[string]bool{}
	for _, entry := range h.shape() {
		for _, item := range entry.Items {
			got[item] = true
		}
	}
	if !got["loop.progress:good-1"] || !got["loop.completed:forged-1"] {
		t.Fatalf("the batch's records were lost instead of falling back to singles: %v", got)
	}
}

// ---- P1-F3/F4: barrier coverage and the exemption ---------------------------

// design 3.4 closing paragraph / 4.3: exactly two callers may exempt a write
// from the append barrier.
func TestBarrierExemptionHasExactlyTwoCallSites(t *testing.T) {
	var sites []string
	forEachNonTestGoFile(t, func(rel string, file *ast.File, _ *token.FileSet) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "WithBarrierExemption" {
				sites = append(sites, rel)
			}
			return true
		})
	})
	sort.Strings(sites)
	want := []string{"internal/store/run_memory.go", "internal/store/telemetry_sealer.go"}
	if strings.Join(sites, ",") != strings.Join(want, ",") {
		t.Fatalf("WithBarrierExemption call sites = %v, want exactly %v", sites, want)
	}
}

// design 6.2 item 5: the sealer takes no store lock. A flush that took eventMu
// would deadlock against the projection store's Prepare/Finalize (which take
// it under the appender mutex); one that took trajectoryMu or runMemoryMu would
// invert the order of lifecycle writers that barrier while holding them.
func TestSealerNeverTakesStoreLocks(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "telemetry_sealer.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	banned := map[string]bool{"eventMu": true, "trajectoryMu": true, "runMemoryMu": true, "workerUpdateMu": true, "textureMutationMu": true, "channelMsgMu": true, "presenceMu": true}
	ast.Inspect(file, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && banned[sel.Sel.Name] {
			t.Errorf("telemetry_sealer.go references %s at %s; the sealer must not take store locks", sel.Sel.Name, fset.Position(sel.Pos()))
		}
		return true
	})
}

// ---- P1-F5: ordering across a barrier ---------------------------------------

func TestOrderingIsPreservedAcrossBarrier(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	at := func(i int) time.Time { return telemetryBase.Add(time.Duration(i) * time.Second) }
	for i, step := range []struct {
		id   string
		kind types.EventKind
	}{
		{"t1", types.EventToolInvoked}, {"t2", types.EventToolResult},
		{"a1", types.EventRunCompleted},
		{"t3", types.EventRunProgress},
	} {
		if err := h.store.AppendEvent(ctx, h.event(step.id, "run-o", "owner-1", step.kind, at(i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.store.DrainTelemetry(ctx); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range h.shape() {
		kind := "A"
		if e.Authority == telemetryAuthority {
			kind = "T"
		}
		got = append(got, kind+"["+strings.Join(e.Items, ",")+"]")
	}
	// The boot watermark (a runtime.health event) is written before the first
	// batched enqueue, then the tape reads: [t1,t2] before a1 before [t3].
	joined := strings.Join(got, " ")
	idx := func(s string) int { return strings.Index(joined, s) }
	if idx("T[tool.invoked:t1,tool.result:t2]") < 0 || idx("A[loop.completed:a1]") < 0 || idx("T[loop.progress:t3]") < 0 {
		t.Fatalf("tape shape = %s", joined)
	}
	if !(idx("T[tool.invoked:t1,tool.result:t2]") < idx("A[loop.completed:a1]") && idx("A[loop.completed:a1]") < idx("T[loop.progress:t3]")) {
		t.Fatalf("a semantic event landed out of order relative to telemetry: %s", joined)
	}
	events := h.storedEvents("run-o")
	for i, ev := range events {
		if ev.Seq != int64(i+1) && i > 0 && ev.Seq <= events[i-1].Seq {
			t.Fatalf("seq not ascending: %+v", events)
		}
	}
	if len(events) != 4 {
		t.Fatalf("stored %d events, want 4", len(events))
	}
}

// ---- P1-F6: a crash loses at most the open window, never a semantic event ---

func TestCrashLosesOnlyTheOpenWindow(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	at := func(i int) time.Time { return telemetryBase.Add(time.Duration(i) * time.Second) }
	step := func(id string, kind types.EventKind, i int) {
		t.Helper()
		if err := h.store.AppendEvent(ctx, h.event(id, "run-c", "owner-1", kind, at(i))); err != nil {
			t.Fatalf("append %s: %v", id, err)
		}
	}
	step("s1", types.EventRunSubmitted, 1)
	step("t2", types.EventRunProgress, 2)
	step("t3", types.EventToolResult, 3)
	step("a4", types.EventRunStarted, 4)  // barrier: t2 and t3 are sealed first
	step("t5", types.EventRunProgress, 5) // open window
	step("t6", types.EventToolInvoked, 6) // open window
	if depth := h.store.telemetry.Load().depth(); depth != 2 {
		t.Fatalf("queue depth before the crash = %d, want 2", depth)
	}
	h.reopen()

	var ids []string
	for _, ev := range h.storedEvents("run-c") {
		ids = append(ids, ev.EventID)
	}
	if strings.Join(ids, ",") != "s1,t2,t3,a4" {
		t.Fatalf("events after the crash = %v, want s1,t2,t3,a4: every acknowledged event present, only the open window (t5,t6) lost, no hole", ids)
	}

	// An acknowledged event enqueued after telemetry is never lost with it:
	// the barrier seals the telemetry first, so both are on the tape.
	h.enableBatching(farFuture)
	step("t7", types.EventRunProgress, 7)
	step("a8", types.EventRunCompleted, 8)
	h.reopen()
	ids = nil
	for _, ev := range h.storedEvents("run-c") {
		ids = append(ids, ev.EventID)
	}
	if got := strings.Join(ids, ","); got != "s1,t2,t3,a4,t7,a8" {
		t.Fatalf("events after the second crash = %v, want s1,t2,t3,a4,t7,a8", got)
	}
}

// ---- P1-F7: sequence numbers are never reused -------------------------------

// issued records every (run, seq) and stream_seq a test handed out, across
// "processes", so a later number can be checked against all earlier ones.
type issuedNumbers struct {
	runSeq map[string]map[int64]string
	stream map[int64]string
	maxRun map[string]int64
	maxStr int64
}

func newIssued() *issuedNumbers {
	return &issuedNumbers{runSeq: map[string]map[int64]string{}, stream: map[int64]string{}, maxRun: map[string]int64{}}
}

func (n *issuedNumbers) note(t *testing.T, rec *types.EventRecord) {
	t.Helper()
	if prev, dup := n.stream[rec.StreamSeq]; dup {
		t.Fatalf("stream_seq %d issued twice: %s and %s", rec.StreamSeq, prev, rec.EventID)
	}
	n.stream[rec.StreamSeq] = rec.EventID
	if rec.StreamSeq > n.maxStr {
		n.maxStr = rec.StreamSeq
	}
	if n.runSeq[rec.RunID] == nil {
		n.runSeq[rec.RunID] = map[int64]string{}
	}
	if prev, dup := n.runSeq[rec.RunID][rec.Seq]; dup {
		t.Fatalf("run %s seq %d issued twice: %s and %s", rec.RunID, rec.Seq, prev, rec.EventID)
	}
	n.runSeq[rec.RunID][rec.Seq] = rec.EventID
	if rec.Seq > n.maxRun[rec.RunID] {
		n.maxRun[rec.RunID] = rec.Seq
	}
}

func (h *tapeHarness) appendNoting(n *issuedNumbers, id, run string, kind types.EventKind, i int) *types.EventRecord {
	h.t.Helper()
	rec := h.event(id, run, "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Second))
	if err := h.store.AppendEvent(context.Background(), rec); err != nil {
		h.t.Fatalf("append %s: %v", id, err)
	}
	n.note(h.t, rec)
	return rec
}

// Case 1: events 2..6 are published and lost; the next process must not hand
// them out again.
func TestNoSeqReuseCrashReseed(t *testing.T) {
	h := newTapeHarness(t)
	n := newIssued()
	h.enableBatching(farFuture)
	h.appendNoting(n, "s1", "run-s", types.EventRunSubmitted, 1)
	for i := 2; i <= 6; i++ {
		h.appendNoting(n, fmt.Sprintf("t%d", i), "run-s", types.EventRunProgress, i)
	}
	lostMaxSeq, lostMaxStream := n.maxRun["run-s"], n.maxStr
	h.reopen()
	h.enableBatching(farFuture)
	rec := h.appendNoting(n, "t7", "run-s", types.EventRunProgress, 7)
	if rec.Seq <= lostMaxSeq || rec.StreamSeq <= lostMaxStream {
		t.Fatalf("after a crash the next numbers are seq %d stream %d; the lost process published up to seq %d stream %d", rec.Seq, rec.StreamSeq, lostMaxSeq, lostMaxStream)
	}
}

// The first append of a run after a crash is usually its close event, an
// acknowledged kind (finding 2 of the plan): it must not take a lost number.
func TestNoSeqReuseCloseEventAfterCrash(t *testing.T) {
	h := newTapeHarness(t)
	n := newIssued()
	h.enableBatching(farFuture)
	h.appendNoting(n, "s1", "run-x", types.EventRunSubmitted, 1)
	for i := 2; i <= 6; i++ {
		h.appendNoting(n, fmt.Sprintf("t%d", i), "run-x", types.EventRunProgress, i)
	}
	lostMaxSeq, lostMaxStream := n.maxRun["run-x"], n.maxStr
	h.reopen()
	h.enableBatching(farFuture)
	closed := h.appendNoting(n, "close", "run-x", types.EventRunCancelled, 8)
	if closed.Seq <= lostMaxSeq || closed.StreamSeq <= lostMaxStream {
		t.Fatalf("close event after a crash took seq %d stream %d; the lost process published up to seq %d stream %d", closed.Seq, closed.StreamSeq, lostMaxSeq, lostMaxStream)
	}
}

// Case 2: crash, the next boot publishes telemetry that stays queued, crash
// again with no other durable write between. The second boot must still clear
// everything the first boot published. A run that was alive at crash 1 is
// closed by the crash boot with an acknowledged event and takes no more events
// (runtime.go closePreBootRunsAfterCrash), so the second-boot telemetry belongs
// to a run created after crash 1.
func TestNoSeqReuseCrashThenCrash(t *testing.T) {
	h := newTapeHarness(t)
	n := newIssued()
	h.enableBatching(farFuture)
	h.appendNoting(n, "s1", "run-d", types.EventRunSubmitted, 1)
	h.appendNoting(n, "t2", "run-d", types.EventRunProgress, 2)
	h.reopen() // crash 1
	h.enableBatching(farFuture)
	h.appendNoting(n, "close-d", "run-d", types.EventRunCancelled, 3) // the crash boot's close
	h.appendNoting(n, "s-e", "run-e", types.EventRunSubmitted, 4)
	for i := 5; i <= 40; i++ {
		h.appendNoting(n, fmt.Sprintf("b1-%d", i), "run-e", types.EventRunProgress, i)
	}
	boot1MaxSeq, boot1MaxStream := n.maxRun["run-e"], n.maxStr
	h.reopen() // crash 2: of boot 1's telemetry only the boot watermark is durable
	h.enableBatching(farFuture)
	closed := h.appendNoting(n, "close-e", "run-e", types.EventRunCancelled, 100)
	fresh := h.appendNoting(n, "b2-1", "run-f", types.EventRunSubmitted, 101)
	if closed.Seq <= boot1MaxSeq || closed.StreamSeq <= boot1MaxStream || fresh.StreamSeq <= boot1MaxStream {
		t.Fatalf("second boot took seq %d stream %d / %d; the first boot published up to seq %d stream %d", closed.Seq, closed.StreamSeq, fresh.StreamSeq, boot1MaxSeq, boot1MaxStream)
	}
}

// Case 3: while a queue holds numbers that are not stored yet, a reseed (the
// 100,000-key cache drop, the failed-write mark deletion, an asynchronous seal
// failure) must not re-issue them.
func TestNoSeqReuseInProcessReseedUnderQueue(t *testing.T) {
	h := newTapeHarness(t)
	n := newIssued()
	h.enableBatching(farFuture)
	h.appendNoting(n, "s1", "run-r", types.EventRunSubmitted, 1)
	for i := 2; i <= 5; i++ {
		h.appendNoting(n, fmt.Sprintf("t%d", i), "run-r", types.EventRunProgress, i)
	}
	maxQueued := n.maxRun["run-r"]

	// The cache drop: every mark is forgotten while t2..t5 are only queued.
	h.store.eventMu.Lock()
	h.store.eventSeqs = map[eventSeqKey]int64{}
	h.store.eventMu.Unlock()
	rec := h.appendNoting(n, "t6", "run-r", types.EventRunProgress, 6)
	if rec.Seq <= maxQueued {
		t.Fatalf("seq %d re-issued after the cache drop; the queue holds up to %d", rec.Seq, maxQueued)
	}

	// The queue floor alone: a run whose every event is still queued has no
	// stored maximum to skip a gap over, so only the floor keeps the reseed
	// above the queued numbers.
	for i := 1; i <= 3; i++ {
		h.appendNoting(n, fmt.Sprintf("fl%d", i), "run-floor", types.EventRunProgress, i)
	}
	h.store.eventMu.Lock()
	h.store.eventSeqs = map[eventSeqKey]int64{}
	h.store.eventMu.Unlock()
	if rec := h.appendNoting(n, "fl4", "run-floor", types.EventRunProgress, 4); rec.Seq != 4 {
		t.Fatalf("run-floor seq after the cache drop = %d, want 4 (the queue holds 1..3)", rec.Seq)
	}

	// The failed-write path: an acknowledged append that fails must not delete
	// the mark and hand the number out again.
	refuse := errors.New("refused for the test")
	h.store.ogStore.SetWriteValidator(func(context.Context, []objectgraph.Object, []objectgraph.Edge) error { return refuse })
	failed := h.event("a-fail", "run-r", "owner-1", types.EventRunCompleted, telemetryBase.Add(10*time.Second))
	if err := h.store.AppendEvent(context.Background(), failed); !errors.Is(err, refuse) {
		t.Fatalf("refused append: %v", err)
	}
	h.store.ogStore.SetWriteValidator(h.store.vocabWriteGuard)
	n.note(t, failed) // its numbers were issued even though the write failed
	rec = h.appendNoting(n, "t7", "run-r", types.EventRunProgress, 11)
	if rec.Seq <= failed.Seq {
		t.Fatalf("seq %d re-issued after a failed write (failed took %d)", rec.Seq, failed.Seq)
	}

	// An asynchronous seal failure: the queue is lost, its numbers are not
	// forgotten.
	h.tape.mu.Lock()
	h.tape.pinErr = func([]byte) error { return errors.New("platform outage") }
	h.tape.mu.Unlock()
	h.store.telemetry.Load().drain()
	h.tape.mu.Lock()
	h.tape.pinErr = nil
	h.tape.mu.Unlock()
	if loss := h.store.TelemetryStats().Batching.UnknownOutcomeLoss; loss == 0 {
		t.Fatal("the outage did not lose the queued records; the scenario did not run")
	}
	before := rec.Seq
	rec = h.appendNoting(n, "t8", "run-r", types.EventRunProgress, 12)
	if rec.Seq <= before {
		t.Fatalf("seq %d re-issued after an asynchronous seal failure (last was %d)", rec.Seq, before)
	}
}

// The boot watermark: one acknowledged record per boot that carries
// telemetry, written before the first batched enqueue, not per record.
func TestBootWatermarkWrittenOncePerBoot(t *testing.T) {
	h := newTapeHarness(t)
	n := newIssued()
	h.enableBatching(farFuture)
	h.appendNoting(n, "s0", "run-w", types.EventRunSubmitted, 0) // a run begins with an acknowledged event
	for i := 1; i <= 3; i++ {
		h.appendNoting(n, fmt.Sprintf("t%d", i), "run-w", types.EventRunProgress, i)
	}
	count := func() int {
		c := 0
		for _, e := range h.shape() {
			for _, item := range e.Items {
				if strings.HasPrefix(item, "runtime.health:") {
					c++
					if e.Authority != ordinaryAuthority {
						t.Fatalf("boot watermark written under %s, want the ordinary (acknowledged) authority", e.Authority)
					}
				}
			}
		}
		return c
	}
	if got := count(); got != 1 {
		t.Fatalf("boot watermark events after one boot = %d, want 1", got)
	}
	if got := h.store.TelemetryStats().Batching.BootWatermarks; got != 1 {
		t.Fatalf("boot_watermark_writes = %d, want 1", got)
	}
	if first := h.shape()[0]; !strings.HasPrefix(first.Items[0], "loop.submitted:") {
		t.Fatalf("tape starts %v, want the acknowledged submit first", first)
	}
	if second := h.shape()[1]; !strings.HasPrefix(second.Items[0], "runtime.health:") {
		t.Fatalf("the watermark must precede the first telemetry, tape continues %v", second)
	}
	h.reopen()
	h.enableBatching(farFuture)
	h.appendNoting(n, "t4", "run-w", types.EventRunProgress, 4)
	if got := count(); got != 2 {
		t.Fatalf("boot watermark events after two boots = %d, want 2", got)
	}
}

// ---- P1-F9: the window ------------------------------------------------------

// The deadline is measured from the first unsealed record and is not reset by
// later ones: a steady stream cannot postpone the seal.
func TestDeadlineIsFromFirstRecordNotSliding(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(400 * time.Millisecond)
	ctx := context.Background()
	// The first allocation of a process seeds the stream_seq lease (an exact
	// seed and a tape append, slow under -race on a loaded runner). Seed it
	// before the clock starts: the clock measures the seal deadline.
	waitLeaseReady(t, h.store)
	start := time.Now()
	for i := 0; i < 4; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("d%d", i), "run-dl", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
		time.Sleep(110 * time.Millisecond) // a sliding timer would be pushed to ~840ms
	}
	sealedBy := func() time.Duration {
		waitFor(t, "the deadline seal", 3*time.Second, func() bool { return h.store.TelemetryStats().Batching.BatchesSealed >= 1 })
		return time.Since(start)
	}()
	if sealedBy > 700*time.Millisecond {
		t.Fatalf("first seal at %s with a 400ms window and a record every 110ms: the deadline slid", sealedBy)
	}
	if got := h.store.TelemetryStats().Batching.SealReasons["deadline"]; got < 1 {
		t.Fatalf("seal reasons = %v, want a deadline seal", h.store.TelemetryStats().Batching.SealReasons)
	}
}

func TestQuietTailSealsAtDeadline(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(150 * time.Millisecond)
	if err := h.store.AppendEvent(context.Background(), h.event("only", "run-q", "owner-1", types.EventToolResult, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "a quiet record to reach the tape", 3*time.Second, func() bool {
		for _, e := range h.shape() {
			for _, item := range e.Items {
				if item == "tool.result:only" {
					return true
				}
			}
		}
		return false
	})
}

func TestCountCapSealsSixtyFourOps(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	for i := 0; i < 64; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("c%d", i), "run-n", "owner-1", types.EventRunDelta, telemetryBase.Add(time.Duration(i)*time.Millisecond))); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "the 64-op count seal", 5*time.Second, func() bool { return h.store.TelemetryStats().Batching.BatchesSealed >= 1 })
	stats := h.store.TelemetryStats().Batching
	if stats.OpsSealed != 64 || stats.SealedOpsPerBatch["33-64"] != 1 || stats.SealReasons["count"] != 1 {
		t.Fatalf("count seal stats = %+v", stats)
	}
}

func TestByteCapChunksAndOversizedOpSealsAlone(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	big := func(id string, kb int) *types.EventRecord {
		rec := h.event(id, "run-b", "owner-1", types.EventToolResult, telemetryBase)
		rec.Payload = json.RawMessage(fmt.Sprintf(`{"output":%q}`, strings.Repeat("x", kb*1024)))
		return rec
	}
	// One op larger than the cap seals alone.
	if err := h.store.AppendEvent(ctx, big("huge", 300)); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the oversized op to seal", 5*time.Second, func() bool { return h.store.TelemetryStats().Batching.BatchesSealed >= 1 })
	if got := h.store.TelemetryStats().Batching; got.OpsSealed != 1 || got.SealReasons["bytes"] != 1 {
		t.Fatalf("oversized op stats = %+v", got)
	}
	// Five 60 KiB ops are chunked so no sealed batch exceeds 256 KiB of ops.
	for i := 0; i < 5; i++ {
		if err := h.store.AppendEvent(ctx, big(fmt.Sprintf("b%d", i), 60)); err != nil {
			t.Fatal(err)
		}
	}
	sl.drain()
	for _, rec := range h.tape.snapshot() {
		total := 0
		for _, op := range h.tape.batchOps(t, rec) {
			total += len(op.Body)
		}
		if n := len(h.tape.batchOps(t, rec)); n > 1 && total > telemetryMaxBatchBytes {
			t.Fatalf("a %d-op batch holds %d bytes of ops, over the %d cap", n, total, telemetryMaxBatchBytes)
		}
	}
}

// ---- P1-F10: reads ----------------------------------------------------------

func TestConsistentReadSealsPending(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("r%d", i), "run-rd", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	sl := h.store.telemetry.Load()
	if sl.depth() != 3 {
		t.Fatalf("queue depth = %d, want 3", sl.depth())
	}
	reads := map[string]func(context.Context) (int, error){
		"ListEvents": func(c context.Context) (int, error) {
			e, err := h.store.ListEvents(c, "run-rd", 10)
			return len(e), err
		},
		"ListEventsAfter": func(c context.Context) (int, error) {
			e, err := h.store.ListEventsAfter(c, "run-rd", 0, 10)
			return len(e), err
		},
		"ListEventsByOwner": func(c context.Context) (int, error) {
			e, err := h.store.ListEventsByOwner(c, "owner-1", 10)
			return len(e), err
		},
		"ListEventsByOwnerAfter": func(c context.Context) (int, error) {
			e, err := h.store.ListEventsByOwnerAfter(c, "owner-1", 0, 10)
			return len(e), err
		},
		"ListEventsByTrajectory": func(c context.Context) (int, error) {
			e, err := h.store.ListEventsByTrajectory(c, "owner-1", "trajectory-run-rd", 10)
			return len(e), err
		},
	}
	names := make([]string, 0, len(reads))
	for name := range reads {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, class := range []ReadConsistency{ReadConsistent, ReadRelaxed} {
			// Re-queue three records for each read so every method meets a pending window.
			base := fmt.Sprintf("%s-%d", name, class)
			for i := 0; i < 3; i++ {
				rec := h.event(fmt.Sprintf("%s-%d", base, i), "run-rd", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Hour))
				rec.TrajectoryID = "trajectory-run-rd"
				if err := h.store.AppendEvent(ctx, rec); err != nil {
					t.Fatal(err)
				}
			}
			if sl.depth() == 0 {
				t.Fatalf("%s: nothing pending before the read", name)
			}
			if _, err := reads[name](WithReadConsistency(ctx, class)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if d := sl.depth(); d != 0 {
				t.Fatalf("%s (class %d): %d records still queued after the read", name, class, d)
			}
		}
	}
	if got := len(h.storedEvents("run-rd")); got != 3+len(names)*2*3 {
		t.Fatalf("stored %d events, want %d", got, 3+len(names)*2*3)
	}
}

// design 6.2 item 5: a Consistent read under the lifecycle writers' locks
// neither deadlocks nor waits on them.
func TestConsistentReadUnderStoreLocks(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	if err := h.store.AppendEvent(ctx, h.event("lk1", "run-lk", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	h.store.trajectoryMu.Lock()
	h.store.runMemoryMu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := h.store.ListEventsAfter(ctx, "run-lk", 0, 10)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a Consistent read deadlocked against trajectoryMu/runMemoryMu")
	}
	h.store.runMemoryMu.Unlock()
	h.store.trajectoryMu.Unlock()
}

// ---- P1-F11/F13: capacity, deadlock, outage ---------------------------------

// The cap path flushes through the real appender, whose projection Prepare
// takes eventMu. A flush issued under eventMu would hang here.
func TestBackpressureAtCapacityDoesNotDeadlock(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	done := make(chan struct{})
	maxDepth := 0
	go func() {
		defer close(done)
		for i := 0; i < 700; i++ {
			run := fmt.Sprintf("run-cap-%d", i%5)
			if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("cap-%d", i), run, "owner-1", types.EventRunProgress, telemetryBase.Add(time.Duration(i)*time.Millisecond))); err != nil {
				t.Errorf("append %d: %v", i, err)
				return
			}
			if d := sl.depth(); d > maxDepth {
				maxDepth = d
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("appends at the capacity limit hung: a flush is running under eventMu")
	}
	if maxDepth > telemetryQueueCapacity {
		t.Fatalf("queue depth reached %d, over the hard cap %d", maxDepth, telemetryQueueCapacity)
	}
	sl.drain()
	stats := h.store.TelemetryStats().Batching
	if stats.OpsSealed != 700 || stats.RecordsDropped != 0 {
		t.Fatalf("sealed %d of 700, dropped %d", stats.OpsSealed, stats.RecordsDropped)
	}
	if stats.CapacityFlushes == 0 {
		t.Fatalf("no capacity flush happened; the cap was not exercised (max depth %d)", maxDepth)
	}
}

func TestPlatformOutageIsBounded(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	// The stream_seq lease is seeded by the first allocation of the process;
	// an outage before that makes the first allocations wait for it, up to the
	// seeding deadline (the lease fails closed), which is not this test's
	// subject. The outage here is a running computer's.
	if err := h.store.AppendEvent(ctx, h.event("seed", "run-out", "owner-1", types.EventRunSubmitted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	h.tape.mu.Lock()
	h.tape.pinErr = func([]byte) error { return errors.New("platform outage") }
	h.tape.mu.Unlock()
	maxDepth := 0
	for i := 0; i < 1000; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("o%d", i), "run-out", "owner-1", types.EventRunDelta, telemetryBase.Add(time.Duration(i)*time.Millisecond))); err != nil {
			// A watermark write failing during the outage is allowed to surface
			// as an acknowledged-path error; telemetry itself must not error.
			if !strings.Contains(err.Error(), "outage") {
				t.Fatalf("append %d: %v", i, err)
			}
		}
		if d := sl.depth(); d > maxDepth {
			maxDepth = d
		}
	}
	if maxDepth > telemetryQueueCapacity {
		t.Fatalf("queue depth %d over the cap %d during an outage", maxDepth, telemetryQueueCapacity)
	}
	stats := h.store.TelemetryStats().Batching
	if stats.BatchesSealed != 0 {
		t.Fatalf("batches sealed during an outage: %d", stats.BatchesSealed)
	}
	h.tape.mu.Lock()
	h.tape.pinErr = nil
	h.tape.mu.Unlock()
	if err := h.store.AppendEvent(ctx, h.event("recovered", "run-out", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	sl.drain()
	found := false
	for _, e := range h.shape() {
		for _, item := range e.Items {
			if item == "loop.progress:recovered" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("telemetry did not resume after the outage")
	}
}

// A refused batch (nothing pinned) splits to single writes; a record that is
// still refused alone is dropped and counted; the rest reach the tape.
func TestSealFailureBeforePinSplitsToSingles(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	h.dryRunHook = func(batch *computerevent.ProjectionBatch) error {
		for _, op := range batch.Ops {
			if strings.Contains(string(op.Body), "poison-record") || strings.Contains(string(op.Body), "cG9pc29uLXJlY29yZ") {
				return errors.New("poison refused by the dry run")
			}
		}
		return nil
	}
	ctx := context.Background()
	for _, id := range []string{"ok-1", "poison-record", "ok-2"} {
		if err := h.store.AppendEvent(ctx, h.event(id, "run-p", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
			t.Fatal(err)
		}
	}
	sl.drain()
	var items []string
	for _, e := range h.shape() {
		for _, item := range e.Items {
			if strings.HasPrefix(item, "loop.progress:") {
				items = append(items, item)
				if e.Authority != telemetryAuthority {
					t.Errorf("split single %s written under %s, want the telemetry authority", item, e.Authority)
				}
			}
		}
	}
	if strings.Join(items, ",") != "loop.progress:ok-1,loop.progress:ok-2" {
		t.Fatalf("tape records after a split = %v, want ok-1 and ok-2 only", items)
	}
	stats := h.store.TelemetryStats().Batching
	if stats.SplitToSingles != 1 || stats.PoisonDropped != 1 {
		t.Fatalf("split/poison counters = %d/%d, want 1/1", stats.SplitToSingles, stats.PoisonDropped)
	}
}

// After the pin the outcome is unknown: the batch is not retried (a retry
// would be a second tape event), the loss is counted.
func TestSealFailureAfterPinIsNotRetried(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	// Write the watermark first so the failing CAS is the telemetry batch's.
	if err := h.store.AppendEvent(ctx, h.event("w", "run-u", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	sl.drain()
	var mu sync.Mutex
	attempts := 0
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		return errors.New("cas outcome unknown")
	}
	h.tape.mu.Unlock()
	for i := 0; i < 3; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("u%d", i), "run-u", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
			t.Fatal(err)
		}
	}
	sl.drain()
	mu.Lock()
	got := attempts
	mu.Unlock()
	if got != 1 {
		t.Fatalf("CAS attempted %d times for one sealed batch, want exactly 1 (no retry)", got)
	}
	stats := h.store.TelemetryStats().Batching
	if stats.UnknownOutcomeLoss != 3 || stats.SealFailures != 1 {
		t.Fatalf("loss counters = %+v", stats)
	}
}

// ---- P1-F12: races ----------------------------------------------------------

func TestConcurrentTelemetryAndBarriers(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(15 * time.Millisecond)
	n := newIssued()
	ctx := context.Background()
	const workers, each = 8, 40
	var wg sync.WaitGroup
	var mu sync.Mutex
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			run := fmt.Sprintf("run-con-%d", w%4)
			for i := 0; i < each; i++ {
				kind := types.EventRunProgress
				if i%9 == 8 {
					kind = types.EventRunStarted // a barrier now and then
				}
				rec := h.event(fmt.Sprintf("con-%d-%d", w, i), run, "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Millisecond))
				if err := h.store.AppendEvent(ctx, rec); err != nil {
					t.Errorf("append: %v", err)
					return
				}
				mu.Lock()
				n.note(t, rec)
				mu.Unlock()
				if i%13 == 0 {
					_, _ = h.store.ListEventsAfter(ctx, run, 0, 5)
				}
			}
		}(w)
	}
	wg.Wait()
	sl.drain()
	seen := map[string]int{}
	lastSeqOfRun := map[string]int64{}
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
			if err := json.Unmarshal(obj.Body, &ev); err != nil {
				t.Fatal(err)
			}
			seen[ev.EventID]++
			if ev.Kind.Batchable() {
				if ev.Seq <= lastSeqOfRun[ev.RunID] {
					t.Fatalf("telemetry of %s out of order on the tape: seq %d after %d", ev.RunID, ev.Seq, lastSeqOfRun[ev.RunID])
				}
				lastSeqOfRun[ev.RunID] = ev.Seq
			}
		}
	}
	for w := 0; w < workers; w++ {
		for i := 0; i < each; i++ {
			if c := seen[fmt.Sprintf("con-%d-%d", w, i)]; c != 1 {
				t.Fatalf("event con-%d-%d appears %d times on the tape, want exactly once", w, i, c)
			}
		}
	}
}

// ---- P1-F14: turning it off, synchronous mode -------------------------------

func TestFlagOffDrainsThenAcknowledges(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("f%d", i), "run-f", "owner-1", types.EventToolResult, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.store.ConfigureTelemetryBatching(TelemetryBatchConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if d := h.store.telemetry.Load().depth(); d != 0 {
		t.Fatalf("%d records still queued after the flag went off", d)
	}
	before := len(h.tape.snapshot())
	if err := h.store.AppendEvent(ctx, h.event("after", "run-f", "owner-1", types.EventToolResult, telemetryBase.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	snap := h.tape.snapshot()
	if len(snap) != before+1 || snap[len(snap)-1].Authority != ordinaryAuthority {
		t.Fatalf("with the flag off a telemetry event must be one ordinary acknowledged write, got %d -> %d events, last %q", before, len(snap), snap[len(snap)-1].Authority)
	}
	if h.store.TelemetryBatchingEnabled() {
		t.Fatal("TelemetryBatchingEnabled after turning the flag off")
	}
}

func TestSynchronousModeDrainsThenAcknowledges(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("s%d", i), "run-sy", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.store.SetTelemetrySynchronous(ctx, true); err != nil {
		t.Fatal(err)
	}
	if d := h.store.telemetry.Load().depth(); d != 0 {
		t.Fatalf("%d records queued after entering synchronous mode", d)
	}
	before := len(h.tape.snapshot())
	if err := h.store.AppendEvent(ctx, h.event("late", "run-sy", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if got := len(h.tape.snapshot()); got != before+1 {
		t.Fatalf("a record in synchronous mode must reach the tape before AppendEvent returns (%d -> %d)", before, got)
	}
}

// Synchronous mode is held, not toggled: two planned-restart holders (a guest
// apply and the host's request) each keep it on until both release, and an
// extra release cannot take the count below zero.
func TestSynchronousModeIsHeldUntilEveryHolderReleases(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := h.store.SetTelemetrySynchronous(ctx, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.store.SetTelemetrySynchronous(ctx, false); err != nil {
		t.Fatal(err)
	}
	if h.store.TelemetryBatchingEnabled() {
		t.Fatal("the first holder's release reopened the queue for the second holder")
	}
	if err := h.store.SetTelemetrySynchronous(ctx, false); err != nil {
		t.Fatal(err)
	}
	if !h.store.TelemetryBatchingEnabled() {
		t.Fatal("synchronous mode outlived both holders")
	}
	_ = h.store.SetTelemetrySynchronous(ctx, false) // unbalanced release
	if err := h.store.SetTelemetrySynchronous(ctx, true); err != nil {
		t.Fatal(err)
	}
	if h.store.TelemetryBatchingEnabled() {
		t.Fatal("an unbalanced release drove the hold count negative")
	}
}

// ---- flag-off and shape -----------------------------------------------------

// Configuring the flag off explicitly changes nothing: still the baseline tape.
func TestFlagConfiguredOffTapeIsByteIdentical(t *testing.T) {
	h := newTapeHarness(t)
	if err := h.store.ConfigureTelemetryBatching(TelemetryBatchConfig{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if h.store.telemetry.Load() != nil {
		t.Fatal("configuring batching off created a sealer; a computer that never turned it on must run the old code")
	}
	scriptedWorkload(t, h)
	got, err := json.MarshalIndent(normalizedTape(t, h), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/telemetry_flag_off_tape.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got)+"\n" != string(want) {
		t.Fatal("tape with the flag configured off differs from the baseline golden")
	}
}

// The op the sealer queues is the op the acknowledged path writes for the same
// event: same canonical id, same body.
func TestTelemetryOpEqualsAcknowledgedOp(t *testing.T) {
	h := newTapeHarness(t)
	ctx := context.Background()
	rec := h.event("same-1", "run-same", "owner-1", types.EventToolResult, telemetryBase)
	rec.Seq, rec.StreamSeq = 7, 9
	if err := h.store.AppendEventOG(ctx, rec); err != nil {
		t.Fatal(err)
	}
	snap := h.tape.snapshot()
	acknowledged := h.tape.batchOps(t, snap[len(snap)-1])
	tr, err := newTelemetryRecord(rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(acknowledged) != 1 || acknowledged[0].CanonicalID != tr.op.CanonicalID || string(acknowledged[0].Body) != string(tr.op.Body) {
		t.Fatalf("queued op differs from the acknowledged op:\n queued: %s\n  acked: %s", tr.op.Body, acknowledged[0].Body)
	}
}

// The sealed batch is one ordinary projection_batch_recorded event holding N
// choir.event ops, and a sealed batch replays to the same stored events.
func TestSealedBatchIsOneEventWithNOps(t *testing.T) {
	h := newTapeHarness(t)
	sl := h.enableBatching(farFuture)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := h.store.AppendEvent(ctx, h.event(fmt.Sprintf("n%d", i), "run-1", "owner-1", types.EventToolInvoked, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	sl.drain()
	var telemetryEvents []tapeRecord
	for _, rec := range h.tape.snapshot() {
		if rec.Authority == telemetryAuthority {
			telemetryEvents = append(telemetryEvents, rec)
		}
	}
	if len(telemetryEvents) != 1 || telemetryEvents[0].Kind != computerevent.EventProjectionBatchRecorded {
		t.Fatalf("telemetry tape events = %+v, want one projection_batch_recorded", telemetryEvents)
	}
	if ops := h.tape.batchOps(t, telemetryEvents[0]); len(ops) != 5 {
		t.Fatalf("sealed batch holds %d ops, want 5", len(ops))
	}
	if events := h.storedEvents("run-1"); len(events) != 5 {
		t.Fatalf("stored %d events from the sealed batch, want 5", len(events))
	}
	stats := h.store.TelemetryStats().Batching
	if stats.MeanOpsPerBatch != 5 || stats.RecordsEnqueued != 5 {
		t.Fatalf("batching stats = %+v", stats)
	}
}
