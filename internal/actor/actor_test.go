package actor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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

type faultLog struct {
	base              *SQLiteLog
	mu                sync.Mutex
	markFailures      int
	saveAttempts      int
	saveSnapshotsFail bool
}

func (l *faultLog) Append(ctx context.Context, u Update) (bool, error) {
	return l.base.Append(ctx, u)
}

func (l *faultLog) Unprocessed(ctx context.Context, agentID string) ([]Update, error) {
	return l.base.Unprocessed(ctx, agentID)
}

func (l *faultLog) MarkProcessed(ctx context.Context, agentID, updateID string) error {
	l.mu.Lock()
	if l.markFailures > 0 {
		l.markFailures--
		l.mu.Unlock()
		return errors.New("injected mark processed failure")
	}
	l.mu.Unlock()
	return l.base.MarkProcessed(ctx, agentID, updateID)
}

func (l *faultLog) AgentsWithBacklog(ctx context.Context) ([]string, error) {
	return l.base.AgentsWithBacklog(ctx)
}

func (l *faultLog) SaveSnapshot(ctx context.Context, agentID string, memory []byte) error {
	l.mu.Lock()
	l.saveAttempts++
	fail := l.saveSnapshotsFail
	l.mu.Unlock()
	if fail {
		return errors.New("injected snapshot failure")
	}
	return l.base.SaveSnapshot(ctx, agentID, memory)
}

func (l *faultLog) LoadSnapshot(ctx context.Context, agentID string) ([]byte, error) {
	return l.base.LoadSnapshot(ctx, agentID)
}

func (l *faultLog) SaveAttempts() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.saveAttempts
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
	}), Options{IdleTimeout: 100 * time.Millisecond})
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
	}), Options{IdleTimeout: 100 * time.Millisecond})
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

func TestMarkProcessedFailureRetriesWithoutAdvancingMemory(t *testing.T) {
	base := testLog(t)
	log := &faultLog{base: base, markFailures: 1}
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return append(memory, u.Content...), nil
	}), Options{IdleTimeout: 100 * time.Millisecond, HandlerRetryBackoff: 10 * time.Millisecond})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "a1", Content: "x"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		return handled.Load() >= 2 && processedCount(t, base, "a1")() == 0
	}, "retried and marked processed after injected MarkProcessed failure")
	waitFor(t, 5*time.Second, func() bool { return !rt.Resident("a1") }, "actor passivated after retry")

	memory, err := base.LoadSnapshot(context.Background(), "a1")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if string(memory) != "x" {
		t.Fatalf("snapshot memory = %q, want %q", memory, "x")
	}
}

func TestSnapshotFailureKeepsActorResident(t *testing.T) {
	base := testLog(t)
	log := &faultLog{base: base, saveSnapshotsFail: true}
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return append(memory, u.Content...), nil
	}), Options{IdleTimeout: 50 * time.Millisecond, HandlerRetryBackoff: 10 * time.Millisecond})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "a1", Content: "x"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		return handled.Load() == 1 && processedCount(t, base, "a1")() == 0
	}, "update processed before snapshot failure")
	waitFor(t, 5*time.Second, func() bool { return log.SaveAttempts() > 0 }, "snapshot save attempted")

	if !rt.Resident("a1") {
		t.Fatal("actor deregistered after snapshot failure")
	}
	memory, err := base.LoadSnapshot(context.Background(), "a1")
	if err != nil {
		t.Fatalf("load snapshot: %v", err)
	}
	if len(memory) != 0 {
		t.Fatalf("snapshot memory saved despite injected failure: %q", memory)
	}
}

func TestNoLostWakeUnderConcurrentSendsAndPassivations(t *testing.T) {
	log := testLog(t)
	var handled atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		handled.Add(1)
		return memory, nil
	}), Options{IdleTimeout: 100 * time.Millisecond})
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
	}), Options{IdleTimeout: 100 * time.Millisecond})
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
	}), Options{IdleTimeout: 100 * time.Millisecond})
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
	}), Options{IdleTimeout: 100 * time.Millisecond})
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
	}), Options{HandlerRetryBackoff: 10 * time.Millisecond, IdleTimeout: 100 * time.Millisecond})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return processedCount(t, log, "a1")() == 0 }, "retried and processed")
	if attempts.Load() < 2 {
		t.Fatalf("attempts = %d, want >= 2 (at-least-once)", attempts.Load())
	}
}

// TestNonBlockingBackpressureReturnsErrInboxFull verifies that when
// backpressure is enabled in non-blocking mode, Send returns ErrInboxFull
// when the mailbox is full. The update is still durably logged.
func TestNonBlockingBackpressureReturnsErrInboxFull(t *testing.T) {
	log := testLog(t)
	// Handler that blocks forever — keeps the actor warm and the mailbox full.
	block := make(chan struct{})
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		<-block
		return memory, nil
	}), Options{
		MailboxCapacity: 2,
		Backpressure:    true,
		SendMode:        SendModeNonBlocking,
		IdleTimeout:     100 * time.Millisecond,
	})
	defer func() {
		close(block)
		rt.Stop()
	}()

	ctx := context.Background()
	// First send activates the actor (cold). The handler blocks on the
	// first update, keeping the actor warm.
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u1: %v", err)
	}
	// Wait for the actor to be resident (handler is blocking).
	waitFor(t, 5*time.Second, func() bool { return rt.Resident("a1") }, "actor resident")

	// Fill the mailbox buffer (capacity 2): u2 and u3 fill it.
	if err := rt.Send(ctx, Update{UpdateID: "u2", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u2: %v", err)
	}
	if err := rt.Send(ctx, Update{UpdateID: "u3", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u3: %v", err)
	}
	// u4 should get ErrInboxFull — the mailbox is full.
	err := rt.Send(ctx, Update{UpdateID: "u4", ToAgentID: "a1"})
	if !errors.Is(err, ErrInboxFull) {
		t.Fatalf("send u4: err = %v, want ErrInboxFull", err)
	}
	// The update IS in the durable log even though the mailbox was full.
	backlog, err := log.Unprocessed(ctx, "a1")
	if err != nil {
		t.Fatalf("unprocessed: %v", err)
	}
	// u1 is being processed (handler blocked on it), u2-u4 are unprocessed.
	// Actually u1 is in-flight (handler is blocked on it, not yet marked
	// processed). So all 4 are unprocessed in the log.
	if len(backlog) < 3 {
		t.Fatalf("backlog = %d, want >= 3 (u2,u3,u4 durably logged despite ErrInboxFull)", len(backlog))
	}
}

// TestBlockingBackpressureWaitsForSpace verifies that blocking backpressure
// waits for the mailbox to drain and succeeds when space becomes available.
func TestBlockingBackpressureWaitsForSpace(t *testing.T) {
	log := testLog(t)
	// Handler blocks on the first update, then releases on subsequent ones.
	release := make(chan struct{})
	var releaseOnce sync.Once
	closeRelease := func() { releaseOnce.Do(func() { close(release) }) }
	var processed atomic.Int64
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		n := processed.Add(1)
		if n == 1 {
			// First update: block until released, keeping the actor warm
			// and the mailbox full.
			<-release
			return memory, nil
		}
		return memory, nil
	}), Options{
		MailboxCapacity: 1,
		Backpressure:    true,
		SendMode:        SendModeBlocking,
		SendTimeout:     5 * time.Second,
		IdleTimeout:     100 * time.Millisecond,
	})
	defer func() {
		closeRelease()
		rt.Stop()
	}()

	ctx := context.Background()
	// u1: cold activate, handler blocks on u1 (first call).
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u1: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return processed.Load() >= 1 }, "u1 in flight (handler blocked)")

	// u2: fills the mailbox (capacity 1). Handler is blocked on u1, so
	// the mailbox stays full.
	if err := rt.Send(ctx, Update{UpdateID: "u2", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u2: %v", err)
	}

	// u3: blocking send. The mailbox is full (u2 is in it, handler is
	// blocked on u1). This should block until the handler finishes u1
	// and reads u2 from the mailbox.
	sendDone := make(chan error, 1)
	go func() {
		sendDone <- rt.Send(ctx, Update{UpdateID: "u3", ToAgentID: "a1"})
	}()
	// Verify the send is actually blocking (not returning immediately).
	select {
	case err := <-sendDone:
		t.Fatalf("blocking send returned before release: %v", err)
	case <-time.After(200 * time.Millisecond):
		// Good — still blocking.
	}

	// Release the handler; u1 finishes, loop reads u2 from mailbox,
	// mailbox has space, u3's blocking send succeeds.
	closeRelease()
	select {
	case err := <-sendDone:
		if err != nil {
			t.Fatalf("blocking send u3 after release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocking send u3 did not complete after release")
	}
}

// TestBlockingBackpressureTimeoutReturnsErrInboxFull verifies that blocking
// backpressure returns ErrInboxFull after the timeout expires.
func TestBlockingBackpressureTimeoutReturnsErrInboxFull(t *testing.T) {
	log := testLog(t)
	block := make(chan struct{})
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		<-block
		return memory, nil
	}), Options{
		MailboxCapacity: 1,
		Backpressure:    true,
		SendMode:        SendModeBlocking,
		SendTimeout:     100 * time.Millisecond, // short timeout for test
		IdleTimeout:     100 * time.Millisecond,
	})
	defer func() {
		close(block)
		rt.Stop()
	}()

	ctx := context.Background()
	// u1: cold activate, handler blocks.
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u1: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return rt.Resident("a1") }, "actor resident")

	// u2: fills the mailbox (capacity 1).
	if err := rt.Send(ctx, Update{UpdateID: "u2", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u2: %v", err)
	}

	// u3: blocking send, should timeout and return ErrInboxFull.
	start := time.Now()
	err := rt.Send(ctx, Update{UpdateID: "u3", ToAgentID: "a1"})
	elapsed := time.Since(start)
	if !errors.Is(err, ErrInboxFull) {
		t.Fatalf("send u3: err = %v, want ErrInboxFull", err)
	}
	if elapsed < 90*time.Millisecond {
		t.Fatalf("send u3 returned in %v, want >= ~100ms (timeout)", elapsed)
	}
}

// TestActorFailureNotificationOnPanic verifies that when a handler panics,
// the OnActorFailure callback is invoked with the agent ID and an error,
// and the actor goroutine exits cleanly.
func TestActorFailureNotificationOnPanic(t *testing.T) {
	log := testLog(t)
	var failureAgentID string
	var failureErr error
	var failureMu sync.Mutex
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		panic("intentional test panic")
	}), Options{
		IdleTimeout: 100 * time.Millisecond,
		OnActorFailure: func(agentID string, err error) {
			failureMu.Lock()
			failureAgentID = agentID
			failureErr = err
			failureMu.Unlock()
		},
	})
	defer rt.Stop()

	if err := rt.Send(context.Background(), Update{UpdateID: "u1", ToAgentID: "panic-actor"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool {
		failureMu.Lock()
		defer failureMu.Unlock()
		return failureErr != nil
	}, "OnActorFailure callback invoked")

	failureMu.Lock()
	defer failureMu.Unlock()
	if failureAgentID != "panic-actor" {
		t.Fatalf("failure agentID = %q, want %q", failureAgentID, "panic-actor")
	}
	if failureErr == nil || !strings.Contains(failureErr.Error(), "panic") {
		t.Fatalf("failure err = %v, want error containing 'panic'", failureErr)
	}
	// The actor should have been deregistered (goroutine exited).
	waitFor(t, 5*time.Second, func() bool { return !rt.Resident("panic-actor") }, "panic actor deregistered")
}

// TestDrainCompletesWithinTimeout verifies that Drain waits for in-flight
// handlers to complete and returns within the timeout when handlers respect
// context cancellation.
func TestDrainCompletesWithinTimeout(t *testing.T) {
	log := testLog(t)
	started := make(chan struct{}, 1)
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done() // respect cancellation
		return memory, ctx.Err()
	}), Options{IdleTimeout: 30 * time.Second}) // long idle so actor stays resident
	defer rt.Stop()

	ctx := context.Background()
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-started // handler is mid-update, blocking on ctx.Done

	// Drain with a generous timeout. The handler should see ctx cancellation
	// and exit, so Drain completes well within the timeout.
	done := make(chan struct{})
	go func() {
		rt.Drain(5 * time.Second)
		close(done)
	}()
	select {
	case <-done:
		// Good — drain completed.
	case <-time.After(10 * time.Second):
		t.Fatal("drain did not complete within 10s")
	}
}

// TestDrainTimeoutLogsRemaining verifies that Drain logs (but does not hang)
// when an actor handler ignores context cancellation and the timeout expires.
func TestDrainTimeoutLogsRemaining(t *testing.T) {
	log := testLog(t)
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-block // ignore ctx cancellation
		return memory, nil
	}), Options{IdleTimeout: 30 * time.Second})

	ctx := context.Background()
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	<-started // handler is mid-update, blocking on block (not ctx)

	// Drain with a short timeout. The handler ignores cancellation, so
	// Drain should timeout and return (not hang).
	done := make(chan struct{})
	go func() {
		rt.Drain(100 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
		// Good — drain returned after timeout.
	case <-time.After(5 * time.Second):
		t.Fatal("drain hung past timeout")
	}

	// Clean up: close block so the handler can exit and the goroutine
	// can be collected. We need to call Stop to wait for the goroutine.
	close(block)
	rt.Stop()
}

// TestLegacySendNoBackpressure verifies that without backpressure enabled,
// Send silently drops to the log when the mailbox is full (legacy behavior,
// backward compatible). No error is returned.
func TestLegacySendNoBackpressure(t *testing.T) {
	log := testLog(t)
	block := make(chan struct{})
	rt := NewRuntime(log, HandlerFunc(func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
		<-block
		return memory, nil
	}), Options{
		MailboxCapacity: 2,
		// Backpressure NOT enabled — legacy behavior
		IdleTimeout: 100 * time.Millisecond,
	})
	defer func() {
		close(block)
		rt.Stop()
	}()

	ctx := context.Background()
	if err := rt.Send(ctx, Update{UpdateID: "u1", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u1: %v", err)
	}
	waitFor(t, 5*time.Second, func() bool { return rt.Resident("a1") }, "actor resident")

	// Fill the mailbox (capacity 2).
	if err := rt.Send(ctx, Update{UpdateID: "u2", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u2: %v", err)
	}
	if err := rt.Send(ctx, Update{UpdateID: "u3", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u3: %v", err)
	}
	// u4: mailbox is full, but no backpressure — should NOT return an error.
	// The update is silently dropped to the log.
	if err := rt.Send(ctx, Update{UpdateID: "u4", ToAgentID: "a1"}); err != nil {
		t.Fatalf("send u4: err = %v, want nil (legacy no-backpressure)", err)
	}
	// The update IS in the durable log.
	backlog, err := log.Unprocessed(ctx, "a1")
	if err != nil {
		t.Fatalf("unprocessed: %v", err)
	}
	if len(backlog) < 3 {
		t.Fatalf("backlog = %d, want >= 3 (u2,u3,u4 in log)", len(backlog))
	}
}

func TestSQLiteLogRebindMailboxPreservesUpdatesAndSnapshot(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	legacyID, scopedID := "legacy-agent", "owner\x00computer\x00legacy-agent"
	if appended, err := log.Append(ctx, Update{UpdateID: "legacy-update", ToAgentID: legacyID, Content: "retained", CreatedAt: time.Now().UTC()}); err != nil || !appended {
		t.Fatalf("append legacy update: appended=%v err=%v", appended, err)
	}
	if appended, err := log.Append(ctx, Update{UpdateID: "processed-update", ToAgentID: legacyID, Content: "settled", CreatedAt: time.Now().UTC()}); err != nil || !appended {
		t.Fatalf("append processed legacy update: appended=%v err=%v", appended, err)
	}
	if err := log.MarkProcessed(ctx, legacyID, "processed-update"); err != nil {
		t.Fatalf("mark legacy update processed: %v", err)
	}
	if err := log.SaveSnapshot(ctx, legacyID, []byte("retained-memory")); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	migrated, err := log.RebindMailbox(ctx, legacyID, scopedID)
	if err != nil || !migrated {
		t.Fatalf("rebind mailbox: migrated=%v err=%v", migrated, err)
	}
	if updates, err := log.Unprocessed(ctx, legacyID); err != nil || len(updates) != 0 {
		t.Fatalf("legacy updates after rebind: %v, %v", updates, err)
	}
	updates, err := log.Unprocessed(ctx, scopedID)
	if err != nil || len(updates) != 1 || updates[0].UpdateID != "legacy-update" {
		t.Fatalf("scoped updates after rebind: %+v, %v", updates, err)
	}
	memory, err := log.LoadSnapshot(ctx, scopedID)
	if err != nil || string(memory) != "retained-memory" {
		t.Fatalf("scoped snapshot after rebind: %q, %v", memory, err)
	}
	var processedMailbox string
	var processedAt sql.NullTime
	if err := log.db.QueryRowContext(ctx, `SELECT to_agent_id, processed_at FROM actor_updates WHERE update_id = ?`, "processed-update").Scan(&processedMailbox, &processedAt); err != nil {
		t.Fatalf("load processed update after rebind: %v", err)
	}
	if processedMailbox != scopedID || !processedAt.Valid {
		t.Fatalf("processed update after rebind: mailbox=%q processed=%v", processedMailbox, processedAt.Valid)
	}
	migrated, err = log.RebindMailbox(ctx, legacyID, scopedID)
	if err != nil || migrated {
		t.Fatalf("idempotent rebind: migrated=%v err=%v", migrated, err)
	}
}

func TestSQLiteLogRebindMailboxMixedSnapshotsKeepsNewestDestination(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	if err := log.SaveSnapshot(ctx, "legacy", []byte("legacy-memory")); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := log.SaveSnapshot(ctx, "scoped", []byte("scoped-memory")); err != nil {
		t.Fatalf("save scoped snapshot: %v", err)
	}
	legacyAt := time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC)
	scopedAt := legacyAt.Add(time.Minute)
	if _, err := log.db.ExecContext(ctx, `UPDATE actor_snapshots SET updated_at = CASE agent_id WHEN 'legacy' THEN ? ELSE ? END`, legacyAt, scopedAt); err != nil {
		t.Fatalf("set snapshot times: %v", err)
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || !migrated {
		t.Fatalf("merge snapshots: migrated=%v err=%v", migrated, err)
	}
	if memory, err := log.LoadSnapshot(ctx, "legacy"); err != nil || memory != nil {
		t.Fatalf("legacy snapshot after merge: %q, %v", memory, err)
	}
	if memory, err := log.LoadSnapshot(ctx, "scoped"); err != nil || string(memory) != "scoped-memory" {
		t.Fatalf("scoped snapshot after merge: %q, %v", memory, err)
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || migrated {
		t.Fatalf("repeated merge: migrated=%v err=%v", migrated, err)
	}
}

func TestSQLiteLogRebindMailboxMergesDestinationUpdates(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	for _, update := range []Update{
		{UpdateID: "legacy-update", ToAgentID: "legacy", CreatedAt: time.Now().UTC()},
		{UpdateID: "scoped-update", ToAgentID: "scoped", CreatedAt: time.Now().UTC()},
	} {
		if appended, err := log.Append(ctx, update); err != nil || !appended {
			t.Fatalf("append %s: appended=%v err=%v", update.UpdateID, appended, err)
		}
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || !migrated {
		t.Fatalf("merge updates: migrated=%v err=%v", migrated, err)
	}
	legacy, legacyErr := log.Unprocessed(ctx, "legacy")
	scoped, scopedErr := log.Unprocessed(ctx, "scoped")
	if legacyErr != nil || scopedErr != nil || len(legacy) != 0 || len(scoped) != 2 {
		t.Fatalf("backlogs after merge: legacy=%+v (%v), scoped=%+v (%v)", legacy, legacyErr, scoped, scopedErr)
	}
}

func TestSQLiteLogRebindMailboxMixedSnapshotsMovesNewerLegacy(t *testing.T) {
	log := testLog(t)
	ctx := context.Background()
	if err := log.SaveSnapshot(ctx, "legacy", []byte("legacy-memory")); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := log.SaveSnapshot(ctx, "scoped", []byte("scoped-memory")); err != nil {
		t.Fatalf("save scoped snapshot: %v", err)
	}
	scopedAt := time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC)
	legacyAt := scopedAt.Add(time.Minute)
	if _, err := log.db.ExecContext(ctx, `UPDATE actor_snapshots SET updated_at = CASE agent_id WHEN 'legacy' THEN ? ELSE ? END`, legacyAt, scopedAt); err != nil {
		t.Fatalf("set snapshot times: %v", err)
	}
	if migrated, err := log.RebindMailbox(ctx, "legacy", "scoped"); err != nil || !migrated {
		t.Fatalf("merge snapshots: migrated=%v err=%v", migrated, err)
	}
	if memory, err := log.LoadSnapshot(ctx, "scoped"); err != nil || string(memory) != "legacy-memory" {
		t.Fatalf("scoped snapshot after merge: %q, %v", memory, err)
	}
}
