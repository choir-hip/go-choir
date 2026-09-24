package actor

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openKernelLog(t *testing.T) *SQLiteLog {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "actor.db")+"?_pragma=busy_timeout(60000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	l, err := NewSQLiteLog(db)
	if err != nil {
		t.Fatalf("NewSQLiteLog: %v", err)
	}
	return l
}

func mkUpdate(id, to string) Update {
	return Update{UpdateID: id, ToAgentID: to, FromAgentID: "test", Kind: "k", Content: "c", CreatedAt: time.Now()}
}

// The pending projection must reproduce the sweep's backlog set for due
// events: every agent with an unprocessed due update appears, and only those.
func TestPendingProjectionReproducesSweepSet(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()

	for _, u := range []Update{
		mkUpdate("a1", "agent-a"),
		mkUpdate("a2", "agent-a"),
		mkUpdate("b1", "agent-b"),
		mkUpdate("c1", "agent-c"),
	} {
		if _, err := l.Append(ctx, u); err != nil {
			t.Fatalf("append %s: %v", u.UpdateID, err)
		}
	}
	// Incorporate one of agent-a's events; agent-c's event is fully pending.
	if err := l.MarkProcessed(ctx, "agent-a", "a1"); err != nil {
		t.Fatalf("mark: %v", err)
	}

	pending, err := l.PendingAgents(ctx, time.Now())
	if err != nil {
		t.Fatalf("PendingAgents: %v", err)
	}
	sweep, err := l.AgentsWithBacklog(ctx)
	if err != nil {
		t.Fatalf("AgentsWithBacklog: %v", err)
	}
	if len(pending) != len(sweep) {
		t.Fatalf("pending %v != sweep %v", pending, sweep)
	}
	set := map[string]bool{}
	for _, id := range pending {
		set[id] = true
	}
	for _, id := range sweep {
		if !set[id] {
			t.Fatalf("sweep agent %s missing from pending projection", id)
		}
	}
}

// A scheduled (not_before) event must NOT appear in the due projection until
// it comes due; the due-index reports it as the next wake.
func TestScheduledEventDeferredFromProjection(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()

	future := time.Now().Add(time.Hour)
	u := mkUpdate("sched-1", "agent-s")
	u.NotBefore = future
	if _, err := l.Append(ctx, u); err != nil {
		t.Fatalf("append: %v", err)
	}
	// Force the not_before column (Append predates the field; set directly).
	if _, err := l.db.ExecContext(ctx, `UPDATE actor_updates SET not_before = ? WHERE update_id = ?`, future.UTC(), "sched-1"); err != nil {
		t.Fatalf("set not_before: %v", err)
	}

	pending, err := l.PendingAgents(ctx, time.Now())
	if err != nil {
		t.Fatalf("PendingAgents: %v", err)
	}
	for _, id := range pending {
		if id == "agent-s" {
			t.Fatalf("scheduled event appeared in due projection before not_before")
		}
	}
	next, ok, err := l.NextDue(ctx, time.Now())
	if err != nil || !ok {
		t.Fatalf("NextDue: ok=%v err=%v", ok, err)
	}
	if next.Before(future.Add(-time.Second)) {
		t.Fatalf("NextDue %v before scheduled %v", next, future)
	}
}

// The fenced commit must reject a stale epoch: a second activation that
// snapshotted the same starting epoch cannot commit after the first advanced
// the head.
func TestFencedCommitRejectsStaleEpoch(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()

	if _, err := l.Append(ctx, mkUpdate("e1", "agent-x")); err != nil {
		t.Fatalf("append: %v", err)
	}
	epoch0, err := l.Epoch(ctx, "agent-x")
	if err != nil {
		t.Fatalf("epoch: %v", err)
	}
	// First activation commits at epoch0.
	if _, err := l.Commit(ctx, "agent-x", epoch0, nil, []string{"e1"}, nil); err != nil {
		t.Fatalf("commit 1: %v", err)
	}
	// A stale activation that also snapshotted epoch0 must fail.
	if _, err := l.Commit(ctx, "agent-x", epoch0, nil, []string{"e1"}, nil); err != ErrEpochConflict {
		t.Fatalf("stale commit: got %v, want ErrEpochConflict", err)
	}
}

// The dispatcher delivers a due event through the projection — no Go channel,
// no Sweep. The handler observes the event; Commit advances the head.
func TestDispatcherDeliversViaProjection(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()

	var handled int32
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		atomic.AddInt32(&handled, 1)
		return memory, nil
	})
	d := NewDispatcher(l, h, DispatcherOptions{PollInterval: 20 * time.Millisecond})
	go d.Run(ctx)
	defer d.Stop()

	if _, err := l.Append(ctx, mkUpdate("d1", "agent-d")); err != nil {
		t.Fatalf("append: %v", err)
	}
	d.Notify()

	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&handled) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&handled) == 0 {
		t.Fatal("dispatcher did not deliver due event via projection")
	}
	// The event must be incorporated (in the state head).
	exists, processed, err := l.UpdateStatus(ctx, "agent-d", "d1")
	if err != nil || !exists || !processed {
		t.Fatalf("event not incorporated: exists=%v processed=%v err=%v", exists, processed, err)
	}
}

// Kernel mode: Runtime.Send appends and signals the dispatcher — the
// projection is the delivery authority, no Go channel, no Sweep. A restart
// (new runtime over the same log) resumes the pending delivery from the
// tape.
func TestKernelRuntimeDeliversAndResumes(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()

	var handled int32
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		atomic.AddInt32(&handled, 1)
		return memory, nil
	})

	rt := NewKernelRuntime(l, l, h, Options{}, DispatcherOptions{PollInterval: 20 * time.Millisecond})
	rt.StartKernel(ctx)

	if err := rt.Send(ctx, mkUpdate("k1", "agent-k")); err != nil {
		t.Fatalf("send: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&handled) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&handled) == 0 {
		t.Fatal("kernel runtime did not deliver via projection")
	}
	rt.Stop()

	// Restart-resume: append a second event while stopped, then a fresh
	// kernel runtime over the same log must deliver it from the tape — no
	// Sweep, no channel.
	if _, err := l.Append(ctx, mkUpdate("k2", "agent-k")); err != nil {
		t.Fatalf("append k2: %v", err)
	}
	rt2 := NewKernelRuntime(l, l, h, Options{}, DispatcherOptions{PollInterval: 20 * time.Millisecond})
	rt2.StartKernel(ctx)
	defer rt2.Stop()

	deadline = time.Now().Add(3 * time.Second)
	for atomic.LoadInt32(&handled) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if atomic.LoadInt32(&handled) < 2 {
		t.Fatalf("restart did not resume pending delivery: handled=%d", handled)
	}
}

// A poison event (handler always errors) is retried MaxAttempts times, then
// a delivery_failed event is emitted to the error sink and the poisoned
// event is incorporated so it stops re-firing.
func TestDispatcherPoisonEventRoutesToErrorSink(t *testing.T) {
	l := openKernelLog(t)
	ctx := context.Background()

	var calls int32
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		return memory, fmt.Errorf("always fails")
	})
	d := NewDispatcher(l, h, DispatcherOptions{
		PollInterval: 15 * time.Millisecond,
		MaxAttempts:  2,
		ErrorSink:    "error-sink",
	})
	go d.Run(ctx)
	defer d.Stop()

	if _, err := l.Append(ctx, mkUpdate("p1", "agent-p")); err != nil {
		t.Fatalf("append: %v", err)
	}
	d.Notify()

	// Wait for the poisoned event to be incorporated (stops re-firing).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, processed, err := l.UpdateStatus(ctx, "agent-p", "p1")
		if err == nil && processed {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	_, processed, err := l.UpdateStatus(ctx, "agent-p", "p1")
	if err != nil || !processed {
		t.Fatalf("poison event not incorporated: processed=%v err=%v", processed, err)
	}
	// The delivery_failed event must be on the error sink's tape.
	exists, _, err := l.UpdateStatus(ctx, "error-sink", "p1:delivery_failed")
	if err != nil || !exists {
		t.Fatalf("delivery_failed not emitted to error sink: exists=%v err=%v", exists, err)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected >=2 attempts, got %d", calls)
	}
}
