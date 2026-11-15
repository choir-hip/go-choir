package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Station S5 part 2, owner ruling D8 (2026-11-12): writers queued behind the
// in-flight append share one tape event, with no setting. Every test here
// leaves tape.group_commit alone: coalescing must be the behaviour of a store
// nobody configured. The failure list, written before the code:
//
//  C1  queued writers share one event, none answered before its CAS
//  C2  a failed combined append fails every member and is not retried by the group
//  C3  arrival order is the order of the ops inside the event
//  C4  a mixed event is counted by member family; events_by_family still sums to events
//  C5  a conditional batch never joins a group
//  C6  the run-memory key comes from a fixed list of reasons
//  C7  run-memory messages are acknowledged only after their CAS (no window)
//  C8  tape.group_commit, the 10 s telemetry window and the run-memory window are gone
//  C9  a lone writer is one event, at once

func TestCoalesceQueuedWritersShareOneEventWithNoSetting(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	for i := 0; i < 5; i++ {
		h.startAcked(results, fmt.Sprintf("m%d", i), fmt.Sprintf("run-m%d", i), i+1)
	}
	select {
	case r := <-results:
		t.Fatalf("%s returned before the held CAS was released (err %v): acknowledged before durable", r.id, r.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(gate.release)
	for i := 0; i < 6; i++ {
		if r := <-results; r.err != nil {
			t.Fatalf("%s: %v", r.id, r.err)
		}
	}
	shared := h.eventsHolding("loop.completed:m0")
	if len(shared) != 1 {
		t.Fatalf("m0 is in events %v, want exactly one", shared)
	}
	for i := 1; i < 5; i++ {
		if got := h.eventsHolding(fmt.Sprintf("loop.completed:m%d", i)); len(got) != 1 || got[0] != shared[0] {
			t.Fatalf("m%d is in events %v, want the shared event %d", i, got, shared[0])
		}
	}
}

func TestCoalesceFailedCombinedAppendFailsEveryMemberOnce(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	for i := 0; i < 3; i++ {
		h.startAcked(results, fmt.Sprintf("u%d", i), fmt.Sprintf("run-u%d", i), i+1)
	}
	var mu sync.Mutex
	attempts := 0
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error {
		mu.Lock()
		attempts++
		mu.Unlock()
		return errors.New("cas outcome unknown")
	}
	h.tape.mu.Unlock()
	close(gate.release)
	for i := 0; i < 4; i++ {
		r := <-results
		if r.id == "lead" {
			if r.err != nil {
				t.Fatalf("lead: %v", r.err)
			}
			continue
		}
		if r.err == nil || !strings.Contains(r.err.Error(), "cas outcome unknown") {
			t.Fatalf("%s: %v, want the shared CAS error (no member may be acknowledged)", r.id, r.err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	// One combined request plus the appender's same-request re-send; the
	// group layer does not append the members one by one.
	if attempts != 2 {
		t.Fatalf("CAS attempts after the lead = %d, want 2", attempts)
	}
	for i := 0; i < 3; i++ {
		if got := h.eventsHolding(fmt.Sprintf("loop.completed:u%d", i)); len(got) != 0 {
			t.Fatalf("u%d reached the tape (%v) although its caller was told it failed", i, got)
		}
	}
}

func TestCoalesceKeepsArrivalOrderInsideTheEvent(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 16)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	const n = 8
	for i := 0; i < n; i++ {
		h.startAcked(results, fmt.Sprintf("o%d", i), fmt.Sprintf("run-o%d", i), i+1)
	}
	close(gate.release)
	for i := 0; i < n+1; i++ {
		if r := <-results; r.err != nil {
			t.Fatalf("%s: %v", r.id, r.err)
		}
	}
	var got []string
	for _, e := range h.shape() {
		for _, it := range e.Items {
			if strings.HasPrefix(it, "loop.completed:o") {
				got = append(got, it)
			}
		}
	}
	want := make([]string, n)
	for i := range want {
		want[i] = fmt.Sprintf("loop.completed:o%d", i)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ops in tape order %v, want arrival order %v", got, want)
	}
	if shared := h.eventsHolding("loop.completed:o0"); len(shared) != 1 || len(h.eventsHolding("loop.completed:o7")) != 1 || h.eventsHolding("loop.completed:o7")[0] != shared[0] {
		t.Fatal("the eight queued writers did not share one event")
	}
}

// statsKeyed reads a counter map of the observability snapshot by its wire
// name, so a test compiles before the field exists.
func statsKeyed(t *testing.T, s *Store, name string) map[string]int64 {
	t.Helper()
	raw, err := json.Marshal(s.TelemetryStats())
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	var out map[string]int64
	if blob, ok := all[name]; ok {
		if err := json.Unmarshal(blob, &out); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	return out
}

func sumKeyed(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}

func TestCoalesceMixedEventIsCountedByMemberFamily(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	runs := distinctRunMemoryRuns(h.store, 3)
	h.startAcked(results, "obj", "run-obj", 1)
	for i, run := range runs {
		id, run := fmt.Sprintf("rm-%d", i), run
		go func() {
			_, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
			results <- writeResult{id, err}
		}()
		waitFor(t, id+" pending", 5*time.Second, func() bool { return h.pendingMembers() >= i+2 })
	}
	close(gate.release)
	for i := 0; i < 5; i++ {
		if r := <-results; r.err != nil {
			t.Fatalf("%s: %v", r.id, r.err)
		}
	}
	if len(h.eventsHolding("run_memory:rm-0")) != 1 || h.eventsHolding("run_memory:rm-0")[0] != h.eventsHolding("loop.completed:obj")[0] {
		t.Fatal("the object and run-memory writers did not share an event")
	}
	st := h.store.TelemetryStats()
	if got := sumKeyed(st.TapeEventsByFamily); got != st.TapeEvents {
		t.Fatalf("tape_events_by_family sums to %d, want tape_events %d (%v)", got, st.TapeEvents, st.TapeEventsByFamily)
	}
	ops := statsKeyed(t, h.store, "tape_ops_by_family")
	if ops["run_memory"] != 3 {
		t.Fatalf("tape_ops_by_family = %v: the three run-memory members must be counted under run_memory, not under ops[0]'s family", ops)
	}
	if got := sumKeyed(ops); got != st.TapeOps {
		t.Fatalf("tape_ops_by_family sums to %d, want tape_ops %d (%v)", got, st.TapeOps, ops)
	}
}

func TestCoalesceConditionalBatchNeverJoinsAGroup(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	h.startAcked(results, "plain", "run-plain", 1)
	now := time.Now().UTC()
	obj, err := lifecycleObject(ogKindMigrationMarker, "owner-1", h.computerID, "cond-marker", map[string]string{"x": "y"}, map[string]any{}, now, now)
	if err != nil {
		t.Fatal(err)
	}
	condDone := make(chan error, 1)
	go func() {
		condDone <- h.store.ogStore.PutBatchConditional(context.Background(),
			[]objectgraph.ObjectCondition{{CanonicalID: obj.CanonicalID}}, objectgraph.Batch{Objects: []objectgraph.Object{obj}})
	}()
	time.Sleep(100 * time.Millisecond)
	if n := h.pendingMembers(); n != 1 {
		t.Fatalf("pending group members = %d, want 1: the conditional batch must not queue as a member", n)
	}
	close(gate.release)
	if err := <-condDone; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if r := <-results; r.err != nil {
			t.Fatalf("%s: %v", r.id, r.err)
		}
	}
	for _, e := range h.shape() {
		hasPlain := false
		for _, it := range e.Items {
			hasPlain = hasPlain || it == "loop.completed:plain"
		}
		if hasPlain && len(e.Items) > 1 {
			t.Fatalf("the plain writer shares an event with another op: %v", e.Items)
		}
	}
}

func TestRunMemoryKeyUsesAFixedListOfReasons(t *testing.T) {
	cases := []struct{ kind, reason, want string }{
		{"message", "actor_rewarm", "message/actor_rewarm"},
		{"cell_fate", "tray committed", "cell_fate/tray committed"},
		{"cell_fate", "tray recovered", "cell_fate/tray recovered"},
		{"compaction", "threshold", "compaction/threshold"},
		{"compaction", "context_overflow", "compaction/context_overflow"},
		{"cell_fate", "reflect: call of reflect.Value.IsNil on string Value", "cell_fate/other"},
		{"cell_fate", "reflect: slice index out of range", "cell_fate/other"},
		{"cell_fate", "cell terminal deadline exceeded", "cell_fate/cell terminal deadline exceeded"},
		{"message", "", "message"},
	}
	for _, c := range cases {
		body, err := json.Marshal(computerevent.RunMemoryEntryProjection{EntryID: "e", Kind: c.kind, Reason: c.reason})
		if err != nil {
			t.Fatal(err)
		}
		if got := runMemoryKey(body); got != c.want {
			t.Errorf("%s/%q: runMemoryKey = %q, want %q", c.kind, c.reason, got, c.want)
		}
	}
}

func TestRunMemoryMessageIsAcknowledgedOnlyAfterItsCAS(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	h.warm()
	gate := h.holdNextCAS()
	done := make(chan error, 1)
	go func() {
		_, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{EntryID: "rm-ack", RunID: "run-ack", OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
		done <- err
	}()
	<-gate.entered
	select {
	case err := <-done:
		t.Fatalf("a run-memory message returned (%v) before its CAS: the run-memory window is back", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(gate.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(h.eventsHolding("run_memory:rm-ack")) != 1 {
		t.Fatal("the acknowledged message is not on the tape")
	}
}

func TestTheSettingAndBothLossWindowsAreGone(t *testing.T) {
	banned := []string{"tape.group_commit", "SettingTapeGroupCommit", "SetTapeGroupCommit", "TapeGroupCommitOn",
		"telemetryGroupWindow", "runMemoryKindBatched", "ErrRunMemoryLost", "markRunMemoryDamaged"}
	for _, dir := range []string{".", "../agentcore"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			// Code only: a comment may still tell the history.
			fset := token.NewFileSet()
			var sc scanner.Scanner
			sc.Init(fset.AddFile(f, fset.Base(), len(src)), src, nil, 0)
			for {
				_, tok, lit := sc.Scan()
				if tok == token.EOF {
					break
				}
				if tok != token.IDENT && tok != token.STRING {
					continue
				}
				for _, word := range banned {
					if strings.Contains(lit, word) {
						t.Errorf("%s still names %q in code: the setting and both loss windows are deleted (D8)", f, word)
					}
				}
			}
		}
	}
}

func TestCoalesceLoneWriterIsOneEventAtOnce(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	before := len(h.shape())
	start := time.Now()
	if err := h.store.AppendEvent(context.Background(), h.event("solo", "run-solo", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("a lone writer took %s", d)
	}
	if shape := h.shape(); len(shape) != before+1 || len(shape[before].Items) != 1 {
		t.Fatalf("lone writer shape %+v", shape[before:])
	}
}
