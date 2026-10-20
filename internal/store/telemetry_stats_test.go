package store

import (
	"context"
	"fmt"
	"sync"
	"testing"

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
	// 11 tape events: 10 standalone AppendEvents and one run-memory entry.
	if stats.TapeEvents != 11 || stats.TapeOps != 11 {
		t.Errorf("tape events/ops = %d/%d, want 11/11", stats.TapeEvents, stats.TapeOps)
	}
	if stats.OpsPerTapeEvent["1"] != 11 {
		t.Errorf("ops_per_tape_event = %v, want every event at 1 op", stats.OpsPerTapeEvent)
	}
	if stats.TapeEventsByAuthority["authority:vm-local-projection"] != 11 {
		t.Errorf("tape_events_by_authority = %v", stats.TapeEventsByAuthority)
	}
	if stats.TapeOpsByKind["object_recorded"] != 10 || stats.TapeOpsByKind["run_memory_entry_recorded"] != 1 {
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
	if stats.TapeEvents != workers*each {
		t.Fatalf("counted %d tape events, want %d", stats.TapeEvents, workers*each)
	}
}
