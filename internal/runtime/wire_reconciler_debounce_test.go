package runtime

import (
	"fmt"
	"testing"
	"time"
)

func TestWirePublishDebouncerFiresOnCountThreshold(t *testing.T) {
	d := newWirePublishDebouncer()
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	for i := 0; i < WireReconcilerPublishCountThreshold-1; i++ {
		if _, fire := d.record(fmt.Sprintf("doc-%d", i), fmt.Sprintf("rev-%d", i), now); fire {
			t.Fatalf("publish %d should not fire reconciler yet", i+1)
		}
	}
	batch, fire := d.record("doc-final", "rev-final", now)
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
	if _, fire := d.record("doc-1", "rev-1", start); fire {
		t.Fatal("first publish should not fire immediately")
	}
	d.mu.Lock()
	d.lastDispatch = start
	d.mu.Unlock()

	later := start.Add(WireReconcilerPublishDebounceInterval)
	batch, fire := d.record("doc-2", "rev-2", later)
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
	if _, fire := d.record("doc-1", "rev-1", start); fire {
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
