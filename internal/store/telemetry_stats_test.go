package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// P0-5: the counters that price batching before it is enabled. They count
// what the code does today, so they must be exactly right on a known
// workload, and safe under concurrent appends (run with -race).
func TestTelemetryStatsCountEventsAndBatchSizes(t *testing.T) {
	h := newTapeHarness(t)
	scriptedWorkload(t, h)
	stats := h.store.TelemetryStats()

	wantKinds := map[string]int64{
		"loop.submitted": 1, "loop.progress": 3, "tool.invoked": 1, "tool.result": 1, "loop.delta": 1,
		"loop.completed": 1, "channel.message": 1, "runtime.health": 1,
	}
	for kind, want := range wantKinds {
		if got := stats.EventsAppendedByKind[kind]; got != want {
			t.Errorf("events_appended_by_kind[%s] = %d, want %d", kind, got, want)
		}
	}
	if stats.EventsAppendedBatchable != 6 || stats.EventsAppendedAcknowledged != 4 {
		t.Errorf("batchable/acknowledged = %d/%d, want 6/4", stats.EventsAppendedBatchable, stats.EventsAppendedAcknowledged)
	}
	if got := stats.BatchableShare; got < 0.599 || got > 0.601 {
		t.Errorf("batchable_share = %v, want 0.6", got)
	}
	// 12 tape events: 10 standalone AppendEvents, one run-memory entry and the
	// stream_seq lease the first allocation wrote (store bookkeeping, one
	// object op).
	if stats.TapeEvents != 12 || stats.TapeOps != 12 {
		t.Errorf("tape events/ops = %d/%d, want 12/12", stats.TapeEvents, stats.TapeOps)
	}
	if stats.OpsPerTapeEvent["1"] != 12 {
		t.Errorf("ops_per_tape_event = %v, want every event at 1 op", stats.OpsPerTapeEvent)
	}
	if stats.TapeEventsByAuthority["authority:vm-local-projection"] != 12 {
		t.Errorf("tape_events_by_authority = %v", stats.TapeEventsByAuthority)
	}
	if stats.TapeOpsByKind["object_recorded"] != 11 || stats.TapeOpsByKind["run_memory_entry_recorded"] != 1 {
		t.Errorf("tape_ops_by_kind = %v", stats.TapeOpsByKind)
	}
	// Flag off: nothing is batched, so the realised telemetry batch size is 1.
	if stats.Batching.Enabled || stats.Batching.BatchesSealed != 0 || stats.Batching.RecordsEnqueued != 0 {
		t.Errorf("batching stats with the flag off = %+v", stats.Batching)
	}
}

func TestTelemetryStatsAreRaceFree(t *testing.T) {
	h := newTapeHarness(t)
	ctx := context.Background()
	const workers, each = 8, 12
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				kind := types.EventRunProgress
				if i%3 == 0 {
					kind = types.EventRunStarted
				}
				rec := h.event(fmt.Sprintf("race-%d-%d", w, i), fmt.Sprintf("run-%d", w), "owner-1", kind, telemetryBase)
				if err := h.store.AppendEvent(ctx, rec); err != nil {
					t.Errorf("append: %v", err)
				}
				_ = h.store.TelemetryStats()
			}
		}(w)
	}
	wg.Wait()
	stats := h.store.TelemetryStats()
	if total := stats.EventsAppendedBatchable + stats.EventsAppendedAcknowledged; total != workers*each {
		t.Fatalf("counted %d appends, want %d", total, workers*each)
	}
	// One tape event per append, plus the stream_seq lease the first
	// allocation wrote.
	if stats.TapeEvents != workers*each+1 {
		t.Fatalf("counted %d tape events, want %d", stats.TapeEvents, workers*each+1)
	}
}

// R7 S0: the counters FP0 reads for the "before" figures of the trace
// segment log (docs/design/trace-segment-log-2026-10-22.md, section 9): the
// mean size of an event line by kind, and the sink each event went to.
// Read through the JSON the observability endpoint serves, so the test pins
// the wire names too.
func TestTelemetryStatsReportPayloadBytesAndSinkByKind(t *testing.T) {
	for _, batching := range []bool{false, true} {
		name := "flag off"
		if batching {
			name = "batching on"
		}
		t.Run(name, func(t *testing.T) {
			h := newTapeHarness(t)
			if batching {
				h.enableBatching(time.Hour) // a window that never fires: the barrier and Close seal
			}
			ctx := context.Background()
			sizes := map[types.EventKind][]int{
				types.EventToolResult:     {100, 300, 500},
				types.EventRunProgress:    {40, 60},
				types.EventRunStarted:     {20},
				types.EventChannelMessage: {10, 30},
			}
			i := 0
			for kind, list := range sizes {
				for _, size := range list {
					i++
					rec := h.event(fmt.Sprintf("s0-%d", i), "run-s0", "owner-1", kind, telemetryBase.Add(time.Duration(i)*time.Second))
					rec.Payload = json.RawMessage(`{"pad":"` + strings.Repeat("x", size-10) + `"}`)
					if len(rec.Payload) != size {
						t.Fatalf("payload is %d bytes, want %d", len(rec.Payload), size)
					}
					if err := h.store.AppendEvent(ctx, rec); err != nil {
						t.Fatal(err)
					}
				}
			}
			raw, err := json.Marshal(h.store.TelemetryStats())
			if err != nil {
				t.Fatal(err)
			}
			var wire struct {
				Bytes map[string]int64            `json:"event_payload_bytes_by_kind"`
				Mean  map[string]float64          `json:"mean_event_payload_bytes_by_kind"`
				Sink  map[string]map[string]int64 `json:"events_by_sink"`
			}
			if err := json.Unmarshal(raw, &wire); err != nil {
				t.Fatal(err)
			}
			for kind, list := range sizes {
				total := 0
				for _, s := range list {
					total += s
				}
				k := string(kind)
				if wire.Bytes[k] != int64(total) {
					t.Errorf("event_payload_bytes_by_kind[%s] = %d, want %d", k, wire.Bytes[k], total)
				}
				if want := float64(total) / float64(len(list)); wire.Mean[k] != want {
					t.Errorf("mean_event_payload_bytes_by_kind[%s] = %v, want %v", k, wire.Mean[k], want)
				}
				// Today every event goes to the tape (the sealer counts as the tape).
				if wire.Sink["tape"][k] != int64(len(list)) {
					t.Errorf("events_by_sink.tape[%s] = %d, want %d", k, wire.Sink["tape"][k], len(list))
				}
				if wire.Sink["trace_log"][k] != 0 || wire.Sink["dropped"][k] != 0 {
					t.Errorf("events_by_sink for %s: trace_log %d dropped %d, want 0", k, wire.Sink["trace_log"][k], wire.Sink["dropped"][k])
				}
			}
			// All three sinks are always present, so a reader sees zero rather than absence.
			for _, sink := range []string{"tape", "trace_log", "dropped"} {
				if wire.Sink[sink] == nil {
					t.Errorf("events_by_sink has no %q map: %v", sink, wire.Sink)
				}
			}
		})
	}
}

// A refused append is counted as appended (the emission happened) and as no
// sink, so appended minus the sinks is the number that failed.
func TestTelemetryStatsSinkIsNotCountedForAFailedAppend(t *testing.T) {
	h := newTapeHarness(t)
	ctx := context.Background()
	ok := h.event("ok-1", "run-f", "owner-1", types.EventRunProgress, telemetryBase)
	if err := h.store.AppendEvent(ctx, ok); err != nil {
		t.Fatal(err)
	}
	h.tape.mu.Lock()
	h.tape.pinErr = func([]byte) error { return errors.New("pin refused") }
	h.tape.mu.Unlock()
	bad := h.event("bad-1", "run-f", "owner-1", types.EventRunProgress, telemetryBase.Add(time.Second))
	if err := h.store.AppendEvent(ctx, bad); err == nil {
		t.Fatal("the harness did not fail the append")
	}
	raw, _ := json.Marshal(h.store.TelemetryStats())
	var wire struct {
		Appended map[string]int64            `json:"events_appended_by_kind"`
		Sink     map[string]map[string]int64 `json:"events_by_sink"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	k := string(types.EventRunProgress)
	if wire.Appended[k] != 2 || wire.Sink["tape"][k] != 1 {
		t.Fatalf("appended %d, tape sink %d; want 2 and 1", wire.Appended[k], wire.Sink["tape"][k])
	}
}
