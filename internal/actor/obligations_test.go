package actor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// SL "what is owed" for the actor tape
// (docs/problems/texture-create-occurrence-deferred-never-refires-2026-10-09.md).
// Failure modes: a deferred event is invisible; a due unprocessed event is
// hidden; processed events are counted; an in-flight activation has no age.

func TestUnprocessedSummaryCountsDueAndDeferred(t *testing.T) {
	ctx := context.Background()
	l := openKernelLog(t)
	now := time.Now().UTC()
	for _, u := range []Update{mkUpdate("due-1", "texture:a"), mkUpdate("due-2", "research:b"), mkUpdate("done", "texture:a"), mkUpdate("deferred", "texture:c")} {
		if _, err := l.Append(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.MarkProcessed(ctx, "texture:a", "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.DeferUpdate(ctx, "texture:c", "deferred", func(int) time.Time { return now.Add(time.Minute) }); err != nil {
		t.Fatal(err)
	}

	got, err := l.UnprocessedSummary(ctx, now, 100)
	if err != nil {
		t.Fatal(err)
	}
	if got.Due != 2 || got.Deferred != 1 {
		t.Fatalf("due=%d deferred=%d, want 2 and 1: %+v", got.Due, got.Deferred, got)
	}
	if got.MaxDeferCount != 1 || got.NextNotBefore.IsZero() {
		t.Fatalf("defer accounting missing: %+v", got)
	}
	for _, s := range got.Samples {
		if s.UpdateID == "done" {
			t.Fatalf("processed update listed: %+v", s)
		}
	}
	if len(got.Samples) != 3 {
		t.Fatalf("samples = %d, want 3", len(got.Samples))
	}
}

func TestDispatcherReportsInFlightActivationAge(t *testing.T) {
	l := openKernelLog(t)
	release := make(chan struct{})
	entered := make(chan struct{})
	d := NewDispatcher(l, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		close(entered)
		<-release
		return memory, nil
	}), DispatcherOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := l.Append(ctx, mkUpdate("u1", "texture:slow")); err != nil {
		t.Fatal(err)
	}
	go d.Run(ctx)
	defer d.Stop()
	<-entered
	inFlight := d.InFlight()
	started, ok := inFlight["texture:slow"]
	if !ok || started.IsZero() || time.Since(started) < 0 {
		t.Fatalf("in-flight activation not reported with a start time: %+v", inFlight)
	}
	close(release)
}

// H2 of texture-create-occurrence-deferred-never-refires: a handler deferral
// must re-fire on its own (due-timer), without any new event for the actor.
func TestDeferredEventRefiresWithoutNewEvent(t *testing.T) {
	l := openKernelLog(t)
	calls := make(chan time.Time, 8)
	var n atomic.Int32
	d := NewDispatcher(l, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		calls <- time.Now()
		if n.Add(1) == 1 {
			return nil, ErrDeferUnprocessed
		}
		return memory, nil
	}), DispatcherOptions{PollInterval: time.Hour})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := l.Append(ctx, mkUpdate("create", "texture:doc")); err != nil {
		t.Fatal(err)
	}
	go d.Run(ctx)
	defer d.Stop()
	first := <-calls
	select {
	case second := <-calls:
		if gap := second.Sub(first); gap > 5*time.Second {
			t.Fatalf("re-fire after %s, want about the 500 ms backoff", gap)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deferred event never re-fired without a new event (H2)")
	}
}
