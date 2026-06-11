package actor

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testLog(t *testing.T) *SQLiteLog {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "actor.db")+"?_busy_timeout=60000")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	log, err := NewSQLiteLog(db)
	if err != nil {
		t.Fatalf("new log: %v", err)
	}
	return log
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

func processedCount(t *testing.T, log *SQLiteLog, agentID string) func() int {
	t.Helper()
	return func() int {
		backlog, err := log.Unprocessed(context.Background(), agentID)
		if err != nil {
			t.Fatalf("unprocessed: %v", err)
		}
		return len(backlog)
	}
}

func TestAppendIdempotent(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	u := Update{UpdateID: "u1", ToAgentID: "a1", Content: "hello", CreatedAt: time.Now().UTC()}
	first, err := log.Append(ctx, u)
	if err != nil || !first {
		t.Fatalf("first append = (%v, %v), want (true, nil)", first, err)
	}
	second, err := log.Append(ctx, u)
	if err != nil || second {
		t.Fatalf("second append = (%v, %v), want (false, nil)", second, err)
	}
	backlog, err := log.Unprocessed(ctx, "a1")
	if err != nil || len(backlog) != 1 {
		t.Fatalf("backlog = %v (err %v), want exactly 1", backlog, err)
	}
}

func TestSendActivatesColdActorAndProcesses(t *testing.T) {
	log := testLog(t)
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return append(memory, []byte(u.Content+";")...), nil
	}), Options{})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "a1", Content: "wake"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return handled.Load() == 1 }, "update handled")
	waitFor(t, 5*time.Second, func() bool { return !rt.Resident("a1") }, "actor passivated on quiescence")

	memory, err := log.LoadSnapshot(context.Background(), "a1")
	if err != nil || string(memory) != "wake;" {
		t.Fatalf("snapshot = %q (err %v), want %q", memory, err, "wake;")
	}
}

func TestWarmSteerDuringActivation(t *testing.T) {
	log := testLog(t)
	release := make(chan struct{})
	var mu sync.Mutex
	var seen []string
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		mu.Lock()
		seen = append(seen, u.UpdateID)
		first := len(seen) == 1
		mu.Unlock()
		if first {
			<-release // hold the activation warm
		}
		return memory, nil
	}), Options{})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "a1", Content: "first"}); err != nil {
		t.Fatalf("send u1: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { mu.Lock(); defer mu.Unlock(); return len(seen) == 1 }, "first update in flight")
	if !rt.Resident("a1") {
		t.Fatal("actor must be warm while handling")
	}
	// Steer the warm actor: this must not start a second activation.
	if err := rt.Send(context.Background(), Update{UpdateID: "u2", ToAgentID: "a1", Content: "steer"}); err != nil {
		t.Fatalf("send u2: %v", err)
	}
	close(release)
	waitFor(t, 5*time.Second, func() bool { mu.Lock(); defer mu.Unlock(); return len(seen) == 2 }, "steered update handled in same residency")
	waitFor(t, 5*time.Second, func() bool { return processedCount(t, log, "a1")() == 0 }, "all marked processed")
}

func TestNoLostWakeUnderConcurrentSendsAndPassivations(t *testing.T) {
	log := testLog(t)
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return memory, nil
	}), Options{})
	defer rt.Stop()

	const senders, perSender = 8, 25
	var wg sync.WaitGroup
	for s := range senders {
		wg.Add(1)
		go func(s int) {
			defer wg.Done()
			for i := range perSender {
				u := Update{UpdateID: fmt.Sprintf("u-%d-%d", s, i), ToAgentID: "a1"}
				if err := rt.Send(context.Background(), u); err != nil {
					t.Errorf("send: %v", err)
					return
				}
			}
		}(s)
	}
	wg.Wait()
	// The fast handler passivates between bursts constantly; every send must
	// still land — the passivation/delivery race is the lost-wake bug.
	waitFor(t, 15*time.Second, func() bool { return processedCount(t, log, "a1")() == 0 }, "every update processed")
	if handled.Load() < senders*perSender {
		t.Fatalf("handled %d < sent %d", handled.Load(), senders*perSender)
	}
}

func TestPassivationThenRewake(t *testing.T) {
	log := testLog(t)
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return append(memory, 'x'), nil
	}), Options{})
	defer rt.Stop()

	ctx := context.Background()
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u1: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return handled.Load() == 1 && !rt.Resident("a1") }, "first activation passivated")

	if err := rt.Send(ctx, Update{UpdateID: "u2", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u2: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return handled.Load() == 2 && !rt.Resident("a1") }, "re-wake handled and passivated")
	memory, _ := log.LoadSnapshot(ctx, "a1")
	if string(memory) != "xx" {
		t.Fatalf("memory across activations = %q, want %q", memory, "xx")
	}
}

func TestBootSweepRecoversCrashWindowBacklog(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	// Simulate the crash window: an update durably appended, but the process
	// died before delivery (no runtime existed to deliver it).
	if _, err := log.Append(ctx, Update{UpdateID: "u1", ToAgentID: "a1", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("append: %v", err)
	}

	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return memory, nil
	}), Options{})
	defer rt.Stop()

	if err := rt.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return handled.Load() == 1 }, "boot sweep delivered the stranded update")
}

func TestEvictionIsCrashEquivalentAndSweepRewakes(t *testing.T) {
	log := testLog(t)
	started := make(chan struct{}, 16)
	block := make(chan struct{})
	var blockOnce sync.Once
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		started <- struct{}{}
		blockOnce.Do(func() {
			select {
			case <-block:
			case <-ctx.Done():
			}
		})
		if ctx.Err() != nil {
			return memory, ctx.Err()
		}
		handled.Add(1)
		return memory, nil
	}), Options{})
	defer rt.Stop()

	ctx := context.Background()
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-started // handler is mid-update

	rt.Evict("a1")
	waitFor(t, 5*time.Second, func() bool { return !rt.Resident("a1") }, "evicted actor released")
	if got := processedCount(t, log, "a1")(); got != 1 {
		t.Fatalf("backlog after eviction = %d, want 1 (crash-equivalent: nothing lost, nothing marked)", got)
	}

	close(block)
	if err := rt.Sweep(ctx); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return handled.Load() == 1 && processedCount(t, log, "a1")() == 0 }, "sweep re-woke and completed the work")
}

func TestHandlerErrorRetriesWithoutLoss(t *testing.T) {
	log := testLog(t)
	var attempts atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		if attempts.Add(1) == 1 {
			return memory, fmt.Errorf("transient failure")
		}
		return memory, nil
	}), Options{HandlerRetryBackoff: 10 * time.Millisecond})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return processedCount(t, log, "a1")() == 0 }, "retried and processed")
	if attempts.Load() < 2 {
		t.Fatalf("attempts = %d, want >= 2 (at-least-once)", attempts.Load())
	}
}
