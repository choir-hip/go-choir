package actor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// docs/problems/fp4-actor-retry-no-backoff-wake-latency-spawn-mailbox-2026-10-22.md
// F1. A failed handler was redelivered at once, so eight attempts took about
// a second and a conflict that clears in a moment became a dead letter.
// Ways the backoff can fail: no wait between deliveries (the dead letter
// stays); a wait that blocks other mailboxes (one failing actor stalls the
// computer); a wait that lets a later update of the same mailbox run past
// the failed one (tape order); a wait that never ends (the update is neither
// handled nor poisoned); the attempt limit lost (an always-failing update
// retries forever).

// deliveryLog records when each update was delivered, in order.
type deliveryLog struct {
	mu    sync.Mutex
	at    map[string][]time.Time
	order []string
}

func newDeliveryLog() *deliveryLog { return &deliveryLog{at: map[string][]time.Time{}} }

func (l *deliveryLog) add(id string) []time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.at[id] = append(l.at[id], time.Now())
	l.order = append(l.order, id)
	return append([]time.Time(nil), l.at[id]...)
}

func (l *deliveryLog) times(id string) []time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]time.Time(nil), l.at[id]...)
}

func (l *deliveryLog) seq() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.order...)
}

func waitProcessed(t *testing.T, l *SQLiteLog, agentID, updateID string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if _, processed, err := l.UpdateStatus(context.Background(), agentID, updateID); err == nil && processed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s/%s not processed within %s", agentID, updateID, within)
}

// A conflict that clears 700 ms after the first delivery is handled, not
// dead-lettered, under the kernel's limit of 8 attempts; and the deliveries
// are spaced (the first wait is at least half the 250 ms base).
func TestDispatcherBacksOffAFailedHandlerSoABriefConflictIsNotADeadLetter(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()
	deliveries := newDeliveryLog()
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		at := deliveries.add(u.UpdateID)
		if time.Since(at[0]) < 700*time.Millisecond {
			return memory, fmt.Errorf("lifecycle command digest conflict")
		}
		return memory, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{PollInterval: 15 * time.Millisecond, MaxAttempts: 8, ErrorSink: "error-sink"})
	go d.Run(ctx)
	defer d.Stop()

	if _, err := l.Append(ctx, mkUpdate("fate", "management")); err != nil {
		t.Fatal(err)
	}
	d.Notify()
	waitProcessed(t, l, "management", "fate", 10*time.Second)

	if exists, _, _ := l.UpdateStatus(ctx, "error-sink", "fate:delivery_failed"); exists {
		times := deliveries.times("fate")
		t.Fatalf("a conflict that cleared in 700 ms was dead-lettered after %d deliveries in %s", len(times), times[len(times)-1].Sub(times[0]))
	}
	times := deliveries.times("fate")
	if len(times) < 2 {
		t.Fatalf("deliveries = %d, want a failure then a success", len(times))
	}
	if gap := times[1].Sub(times[0]); gap < 100*time.Millisecond {
		t.Fatalf("the first redelivery came %s after the failure, want a backoff of at least 125 ms", gap)
	}
}

// The attempt limit holds: an always-failing update is poisoned on its third
// attempt, after the two waits between (at least 0.375 s with jitter, at most
// 0.75 s plus scheduling).
func TestDispatcherRetryBackoffKeepsTheAttemptLimit(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()
	deliveries := newDeliveryLog()
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		deliveries.add(u.UpdateID)
		return memory, fmt.Errorf("always fails")
	})
	d := NewDispatcher(l, h, DispatcherOptions{PollInterval: 15 * time.Millisecond, MaxAttempts: 3, ErrorSink: "error-sink"})
	go d.Run(ctx)
	defer d.Stop()

	if _, err := l.Append(ctx, mkUpdate("bad", "agent-a")); err != nil {
		t.Fatal(err)
	}
	d.Notify()
	waitProcessed(t, l, "agent-a", "bad", 10*time.Second)
	if exists, _, _ := l.UpdateStatus(ctx, "error-sink", "bad:delivery_failed"); !exists {
		t.Fatal("an always-failing update was not dead-lettered at the attempt limit")
	}
	times := deliveries.times("bad")
	if len(times) != 3 {
		t.Fatalf("deliveries = %d, want 3 (MaxAttempts)", len(times))
	}
	window := times[2].Sub(times[0])
	if window < 350*time.Millisecond || window > 3*time.Second {
		t.Fatalf("retry window = %s, want about 0.375 to 0.75 s (250 ms then 500 ms, jittered)", window)
	}
}

// While one mailbox waits out its backoff, another mailbox's update is
// handled at once, and a later update of the waiting mailbox does not run
// past the failed one.
func TestDispatcherRetryBackoffHoldsOnlyTheFailingMailboxInOrder(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()
	deliveries := newDeliveryLog()
	failedOnce := make(chan time.Time, 1)
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		at := deliveries.add(u.UpdateID)
		if u.UpdateID == "a1" && len(at) == 1 {
			failedOnce <- at[0]
			return memory, fmt.Errorf("transient")
		}
		return memory, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{PollInterval: 15 * time.Millisecond, MaxAttempts: 8, ErrorSink: "error-sink"})
	go d.Run(ctx)
	defer d.Stop()

	if _, err := l.Append(ctx, mkUpdate("a1", "agent-a")); err != nil {
		t.Fatal(err)
	}
	d.Notify()
	var failedAt time.Time
	select {
	case failedAt = <-failedOnce:
	case <-time.After(5 * time.Second):
		t.Fatal("a1 never delivered")
	}
	// Created after a1, so a2 is behind it on agent-a's tape.
	time.Sleep(5 * time.Millisecond)
	if _, err := l.Append(ctx, mkUpdate("a2", "agent-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Append(ctx, mkUpdate("b1", "agent-b")); err != nil {
		t.Fatal(err)
	}
	d.Notify()
	waitProcessed(t, l, "agent-b", "b1", 5*time.Second)
	b := deliveries.times("b1")
	if wait := b[0].Sub(failedAt); wait > 100*time.Millisecond {
		t.Fatalf("agent-b's update waited %s behind agent-a's backoff", wait)
	}
	waitProcessed(t, l, "agent-a", "a2", 10*time.Second)
	var aOrder []string
	for _, id := range deliveries.seq() {
		if id == "a1" || id == "a2" {
			aOrder = append(aOrder, id)
		}
	}
	if len(aOrder) != 3 || aOrder[0] != "a1" || aOrder[1] != "a1" || aOrder[2] != "a2" {
		t.Fatalf("agent-a deliveries = %v, want [a1 a1 a2]: a2 must not run past the failed a1", aOrder)
	}
}
