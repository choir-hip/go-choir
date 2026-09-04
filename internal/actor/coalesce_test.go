package actor

import (
	"testing"
	"time"
)

func waitBatch(t *testing.T, c *Coalescer, timeout time.Duration) WakeBatch {
	t.Helper()
	select {
	case batch := <-c.Wait():
		return batch
	case <-time.After(timeout):
		t.Fatal("coalescer never fired: unbounded wait")
		return WakeBatch{}
	}
}

// TestCoalescerAllCompleteFiresImmediately proves the fast path: when every
// expected child settles, the parent wakes at once, not after the window.
func TestCoalescerAllCompleteFiresImmediately(t *testing.T) {
	c := NewCoalescer(2, time.Hour, time.Hour)
	defer c.Stop()
	c.Complete("a")
	c.Complete("b")
	batch := waitBatch(t, c, 2*time.Second)
	if batch.Reason != WakeAllComplete || len(batch.Completed) != 2 {
		t.Fatalf("batch = %+v", batch)
	}
}

// TestCoalescerDebouncesRapidCompletions proves token thrift: three
// completions inside the window yield one quiescent wake, not three.
func TestCoalescerDebouncesRapidCompletions(t *testing.T) {
	c := NewCoalescer(0, 80*time.Millisecond, time.Hour)
	defer c.Stop()
	c.Complete("a")
	c.Complete("b")
	c.Complete("c")
	batch := waitBatch(t, c, 2*time.Second)
	if batch.Reason != WakeQuiescence || len(batch.Completed) != 3 {
		t.Fatalf("batch = %+v", batch)
	}
	select {
	case extra := <-c.Wait():
		t.Fatalf("second wake fired: %+v", extra)
	case <-time.After(150 * time.Millisecond):
	}
}

// TestCoalescerTimeoutBoundsHungChildren proves no barrier deadlock: one
// hung child cannot hold the parent past the deadline.
func TestCoalescerTimeoutBoundsHungChildren(t *testing.T) {
	c := NewCoalescer(3, time.Hour, 100*time.Millisecond)
	defer c.Stop()
	c.Complete("a")
	start := time.Now()
	batch := waitBatch(t, c, 2*time.Second)
	if batch.Reason != WakeTimeout || len(batch.Completed) != 1 {
		t.Fatalf("batch = %+v", batch)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("timeout bound exceeded")
	}
}

// TestCoalescerTombstoneReleasesWait proves crash release: a child failure
// wakes the parent with the tombstone attached, without waiting for
// stragglers beyond the debounce window.
func TestCoalescerTombstoneReleasesWait(t *testing.T) {
	c := NewCoalescer(3, 80*time.Millisecond, time.Hour)
	defer c.Stop()
	c.Complete("a")
	c.Fail("b", "OOM-killed")
	batch := waitBatch(t, c, 2*time.Second)
	if len(batch.Completed) != 1 || batch.Completed[0] != "a" {
		t.Fatalf("completed = %+v", batch)
	}
	if batch.Tombstones["b"] != "OOM-killed" {
		t.Fatalf("tombstones = %+v", batch)
	}
}

// TestCoalescerFiresExactlyOnce proves single-wake discipline: late
// stragglers after the timeout cannot reopen the batch.
func TestCoalescerFiresExactlyOnce(t *testing.T) {
	c := NewCoalescer(2, time.Hour, 80*time.Millisecond)
	defer c.Stop()
	c.Complete("a")
	waitBatch(t, c, 2*time.Second)
	c.Complete("b")
	select {
	case extra := <-c.Wait():
		t.Fatalf("second wake fired: %+v", extra)
	case <-time.After(150 * time.Millisecond):
	}
}
