package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func sumCounts(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}

// Storage solutions 2, S5 counters: what the tape is written with, by writer
// family and by object kind, and in bytes. They never gate behaviour.
func TestTapeWriteCountersFamilyBytesAndKinds(t *testing.T) {
	h := newTapeHarness(t)
	scriptedWorkload(t, h)
	stats := h.store.TelemetryStats()

	if got := sumCounts(stats.TapeEventsByFamily); got != stats.TapeEvents {
		t.Errorf("tape_events_by_family sums to %d, want tape_events %d (%v)", got, stats.TapeEvents, stats.TapeEventsByFamily)
	}
	if stats.TapeEventsByFamily["run_memory"] != 1 {
		t.Errorf("run_memory events = %d, want 1 (%v)", stats.TapeEventsByFamily["run_memory"], stats.TapeEventsByFamily)
	}
	// Bytes are the marshaled projection batch each event pinned.
	h.tape.mu.Lock()
	var wantBytes int64
	for _, p := range h.tape.payloads {
		wantBytes += int64(len(p))
	}
	h.tape.mu.Unlock()
	if wantBytes == 0 || stats.TapeBytes != wantBytes {
		t.Errorf("tape_bytes = %d, want the %d bytes the tape pinned", stats.TapeBytes, wantBytes)
	}
	if got := sumCounts(stats.TapeBytesByFamily); got != stats.TapeBytes {
		t.Errorf("tape_bytes_by_family sums to %d, want %d", got, stats.TapeBytes)
	}
	// Every object op is counted under its object kind, with the body bytes.
	if got := sumCounts(stats.TapeOpsByObjectKind); got != stats.TapeOpsByKind["object_recorded"] {
		t.Errorf("tape_ops_by_object_kind sums to %d, want %d (%v)", got, stats.TapeOpsByKind["object_recorded"], stats.TapeOpsByObjectKind)
	}
	if len(stats.TapeOpsByObjectKind) == 0 || sumCounts(stats.TapeOpBytesByObjectKind) <= 0 {
		t.Errorf("object kind bytes missing: %v %v", stats.TapeOpsByObjectKind, stats.TapeOpBytesByObjectKind)
	}
	if stats.RunMemoryOpsByKind["message"] != 1 {
		t.Errorf("run_memory_ops_by_kind = %v, want one message", stats.RunMemoryOpsByKind)
	}
}

func TestTapeWriteCountersTelemetryFamily(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(time.Hour)
	ctx := context.Background()
	for i, id := range []string{"w-1", "w-2", "w-3"} {
		if err := h.store.AppendEvent(ctx, h.event(id, "run-w", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.store.Close(); err != nil { // Close seals the window
		t.Fatal(err)
	}
	stats := h.store.TelemetryStats()
	if stats.TapeEventsByFamily["telemetry"] < 1 {
		t.Errorf("telemetry family = %v", stats.TapeEventsByFamily)
	}
}

func TestTapeWriteCountersCorpusdCallsPerEvent(t *testing.T) {
	h := newTapeHarness(t)
	var mu sync.Mutex
	calls := map[string]int64{}
	h.store.SetCorpusdCallSource(func() map[string]int64 {
		mu.Lock()
		defer mu.Unlock()
		return copyCounts(calls)
	})
	scriptedWorkload(t, h)
	stats := h.store.TelemetryStats()
	if len(stats.CorpusdCalls) != 0 || stats.CorpusdCallsPerTapeEvent != 0 {
		t.Fatalf("no calls yet: %v %v", stats.CorpusdCalls, stats.CorpusdCallsPerTapeEvent)
	}
	mu.Lock()
	calls["head"] = 12
	calls["pin"] = 24
	calls["append"] = 12
	mu.Unlock()
	stats = h.store.TelemetryStats()
	if stats.CorpusdCalls["pin"] != 24 {
		t.Errorf("corpusd_calls = %v", stats.CorpusdCalls)
	}
	want := 48.0 / float64(stats.TapeEvents)
	if stats.CorpusdCallsPerTapeEvent < want-1e-9 || stats.CorpusdCallsPerTapeEvent > want+1e-9 {
		t.Errorf("corpusd_calls_per_tape_event = %v, want %v", stats.CorpusdCallsPerTapeEvent, want)
	}
}

func TestTapeWriteSummaryLine(t *testing.T) {
	prev := tapeWriteTotals{Events: 100, Ops: 150, Bytes: 1000, Calls: 400, ByFamily: map[string]int64{"run_memory": 60, "telemetry": 10}}
	cur := tapeWriteTotals{Events: 300, Ops: 450, Bytes: 5000, Calls: 1200, ByFamily: map[string]int64{"run_memory": 160, "telemetry": 40, "object:event": 5}}
	line := formatTapeWriteSummary(time.Hour, prev, cur)
	for _, want := range []string{"events=200", "ops=300", "bytes=4000", "calls_per_event=4.00", "run_memory=100", "telemetry=30", "object:event=5", "window=1h0m0s"} {
		if !strings.Contains(line, want) {
			t.Errorf("summary %q lacks %q", line, want)
		}
	}
	if strings.Count(line, "\n") != 0 {
		t.Errorf("summary is not one line: %q", line)
	}
	idle := formatTapeWriteSummary(time.Hour, cur, cur)
	if !strings.Contains(idle, "events=0") || strings.Contains(idle, "NaN") || strings.Contains(idle, "Inf") {
		t.Errorf("idle summary = %q", idle)
	}
}

func TestTapeWriteSummaryLoopLogsEachInterval(t *testing.T) {
	h := newTapeHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var mu sync.Mutex
	var lines []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.store.runTapeWriteSummary(ctx, 20*time.Millisecond, func(format string, args ...any) {
			mu.Lock()
			lines = append(lines, fmt.Sprintf(format, args...))
			mu.Unlock()
		})
	}()
	waitFor(t, "two summary lines", 5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) >= 2
	})
	cancel()
	<-done
	if !strings.Contains(lines[0], "tape write summary") {
		t.Errorf("line = %q", lines[0])
	}
}

// The number that predicts what coalescing would save: how many callers are
// inside the tape write when a new one arrives (storage solutions 2, S5; the
// census, section 4.5). One writer holds the CAS; three more arrive behind it.
func TestTapeWriteCountersAppenderDepthAtEnter(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	before := h.store.TelemetryStats().AppenderDepthAtEnter
	gate := h.holdNextCAS()
	done := make(chan error, 4)
	go func() {
		done <- h.store.AppendEvent(context.Background(), h.event("depth-lead", "run-depth", "owner-1", types.EventRunCompleted, telemetryBase))
	}()
	<-gate.entered
	for i, run := range distinctRunMemoryRuns(h.store, 3) {
		run := run
		id := fmt.Sprintf("depth-rm-%d", i)
		go func() {
			_, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
			done <- err
		}()
		waitFor(t, id+" inside the tape write", 5*time.Second, func() bool { return h.store.telemetryStats.appenderInflight.Load() >= int64(i+2) })
	}
	close(gate.release)
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	after := h.store.TelemetryStats()
	grew := func(label string) int64 { return after.AppenderDepthAtEnter[label] - before[label] }
	if grew("2") < 1 || grew("3-4") < 2 {
		t.Errorf("depth at enter grew by %v from %v to %v, want the 2nd arrival at depth 2 and two more at 3-4", map[string]int64{"2": grew("2"), "3-4": grew("3-4")}, before, after.AppenderDepthAtEnter)
	}
	if after.AppenderInflight != 0 {
		t.Errorf("appender_inflight = %d after every write returned, want 0", after.AppenderInflight)
	}
	if after.AppenderCallMeanMS <= 0 {
		t.Errorf("appender_call_mean_ms = %v", after.AppenderCallMeanMS)
	}
}
