package runtime

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestWirePublishDebouncerFiresOnCountThreshold(t *testing.T) {
	d := newWirePublishDebouncer()
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	for i := 0; i < WireReconcilerPublishCountThreshold-1; i++ {
		if _, fire := d.record(fmt.Sprintf("doc-%d", i), fmt.Sprintf("rev-%d", i), wirePublishLineage{}, now); fire {
			t.Fatalf("publish %d should not fire reconciler yet", i+1)
		}
	}
	batch, fire := d.record("doc-final", "rev-final", wirePublishLineage{}, now)
	if !fire {
		t.Fatal("10th publish should fire reconciler")
	}
	if len(batch.DocIDs) != WireReconcilerPublishCountThreshold {
		t.Fatalf("batch doc ids = %d, want %d", len(batch.DocIDs), WireReconcilerPublishCountThreshold)
	}
}

func TestWirePublishDebouncerFiresAfterIntervalSinceLastDispatch(t *testing.T) {
	d := newWirePublishDebouncer()
	start := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	if _, fire := d.record("doc-1", "rev-1", wirePublishLineage{}, start); fire {
		t.Fatal("first publish should not fire immediately")
	}
	d.mu.Lock()
	d.lastDispatch = start
	d.mu.Unlock()

	later := start.Add(WireReconcilerPublishDebounceInterval)
	batch, fire := d.record("doc-2", "rev-2", wirePublishLineage{}, later)
	if !fire {
		t.Fatal("publish after debounce interval should fire reconciler")
	}
	if len(batch.DocIDs) != 2 {
		t.Fatalf("batch doc ids = %d, want 2", len(batch.DocIDs))
	}
}

func TestWirePublishDebouncerFireDueRespectsInterval(t *testing.T) {
	d := newWirePublishDebouncer()
	start := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	if _, fire := d.record("doc-1", "rev-1", wirePublishLineage{}, start); fire {
		t.Fatal("first publish should not fire immediately")
	}

	if _, fire := d.fireDue(start.Add(100 * time.Second)); fire {
		t.Fatal("timer should not fire before interval elapses")
	}
	batch, fire := d.fireDue(start.Add(WireReconcilerPublishDebounceInterval))
	if !fire || len(batch.DocIDs) != 1 {
		t.Fatalf("timer fire = %v batch = %+v", fire, batch)
	}
}

func TestWirePublishDebouncerPreservesSingleCycleLineage(t *testing.T) {
	d := newWirePublishDebouncer()
	start := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	lineage := wirePublishLineage{CycleID: "cycle-1", RequestID: "processor-1", RequestKind: "processor"}
	if _, fire := d.record("doc-1", "rev-1", lineage, start); fire {
		t.Fatal("first publish should not fire immediately")
	}
	batch, fire := d.fireDue(start.Add(WireReconcilerPublishDebounceInterval))
	if !fire {
		t.Fatal("lineage batch should fire when due")
	}
	if batch.MixedLineage || batch.CycleID != lineage.CycleID || batch.RequestID != lineage.RequestID || batch.RequestKind != lineage.RequestKind {
		t.Fatalf("batch lineage = %+v, want %+v", batch, lineage)
	}
}

func TestWirePublishDebouncerFailsClosedForMixedCycleLineage(t *testing.T) {
	d := newWirePublishDebouncer()
	start := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	d.record("doc-1", "rev-1", wirePublishLineage{CycleID: "cycle-1"}, start)
	d.record("doc-2", "rev-2", wirePublishLineage{CycleID: "cycle-2"}, start)
	batch, fire := d.fireDue(start.Add(WireReconcilerPublishDebounceInterval))
	if !fire || !batch.MixedLineage {
		t.Fatalf("mixed batch fire=%t batch=%+v", fire, batch)
	}
}

func TestDispatchStoryCorpusReconcilerCarriesSingleCycleLineage(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, wirePublishBatch{
		DocIDs:      []string{"doc-1"},
		RevisionIDs: []string{"rev-1"},
		CycleID:     "cycle-1",
		RequestID:   "processor-1",
		RequestKind: "processor",
	})
	runs, err := handler.rt.ListRunsByOwner(ctx, universalWirePlatformOwnerID(), 20)
	if err != nil {
		t.Fatalf("list reconciler runs: %v", err)
	}
	for _, run := range runs {
		if metadataStringValue(run.Metadata, runMetadataAgentProfile) != AgentProfileReconciler {
			continue
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_cycle_id"); got != "cycle-1" {
			t.Fatalf("reconciler cycle id = %q, want cycle-1", got)
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_request_id"); got != "reconciler_publish_cycle-1" {
			t.Fatalf("reconciler request id = %q, want reconciler_publish_cycle-1", got)
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_request_kind"); got != "reconciler" {
			t.Fatalf("reconciler request kind = %q, want reconciler", got)
		}
		if got := metadataStringValue(run.Metadata, "source_network_request_id"); got != "processor-1" {
			t.Fatalf("source request id = %q, want processor-1", got)
		}
		return
	}
	t.Fatal("reconciler run not found")
}

func TestDispatchStoryCorpusReconcilerDeduplicatesOneRunPerCycle(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	batch := wirePublishBatch{
		DocIDs:      []string{"doc-1"},
		RevisionIDs: []string{"rev-1"},
		CycleID:     "cycle-1",
		RequestID:   "processor-1",
		RequestKind: "processor",
	}
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, batch)
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, batch)
	runs, err := handler.rt.Store().ListRunsByIngestionHandoff(ctx, universalWirePlatformOwnerID(), AgentProfileReconciler, "reconciler_publish_cycle-1", "reconciler", 20)
	if err != nil {
		t.Fatalf("list cycle reconciler runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("cycle reconciler runs = %d, want 1", len(runs))
	}
}

func TestDispatchStoryCorpusReconcilerOmitsFalseMixedCycleAttribution(t *testing.T) {
	_, handler := testAPISetup(t)
	ctx := context.Background()
	handler.rt.dispatchStoryCorpusReconcilerFromPublishBatch(ctx, wirePublishBatch{
		DocIDs:       []string{"doc-1", "doc-2"},
		RevisionIDs:  []string{"rev-1", "rev-2"},
		CycleID:      "cycle-1",
		MixedLineage: true,
	})
	runs, err := handler.rt.ListRunsByOwner(ctx, universalWirePlatformOwnerID(), 20)
	if err != nil {
		t.Fatalf("list reconciler runs: %v", err)
	}
	for _, run := range runs {
		if metadataStringValue(run.Metadata, runMetadataAgentProfile) != AgentProfileReconciler {
			continue
		}
		if got := metadataStringValue(run.Metadata, "ingestion_handoff_cycle_id"); got != "" {
			t.Fatalf("mixed-lineage reconciler cycle id = %q, want empty", got)
		}
		return
	}
	t.Fatal("reconciler run not found")
}
