package store

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Invariant 2: a leader that panics before it takes the group strands nobody.
// Its own member is withdrawn (its caller saw the panic, not an outcome), the
// token is freed, and the writers still pending are committed by a later
// leader in one event, in arrival order.
func TestGroupCommitLateTakePanicBeforeTake(t *testing.T) {
	l := newLateHarness(t)
	var fired atomic.Bool
	hook := func() {
		if fired.CompareAndSwap(false, true) {
			panic("leader panics before the take")
		}
	}
	l.store.tapeGroup.beforeTake.Store(&hook)
	l.holdFlush()
	defer l.releaseFlush()
	panicked := make(chan any, 1)
	won := make(chan struct{}, 1)
	wonHook := func() {
		select {
		case won <- struct{}{}:
		default:
		}
	}
	l.store.tapeGroup.afterToken.Store(&wonHook)
	go func() {
		defer func() { panicked <- recover() }()
		_ = l.store.AppendEvent(context.Background(), l.event("a", "run-a", "owner-1", types.EventRunCompleted, telemetryBase))
	}()
	<-won
	l.store.tapeGroup.afterToken.Store(nil)
	l.startMember(context.Background(), "b", 2)
	l.startMember(context.Background(), "c", 3)
	l.releaseFlush()
	select {
	case p := <-panicked:
		if p == nil {
			t.Fatal("the leader's caller saw no panic")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the panicking leader never returned")
	}
	for id, err := range l.collect(2) {
		if err != nil {
			t.Fatalf("%s: %v (a writer pending behind a panicking leader must still commit)", id, err)
		}
	}
	if got := l.eventsHolding("loop.completed:a"); len(got) != 0 {
		t.Fatalf("the panicked leader's own write reached the tape later: %v", got)
	}
	if at, items := l.itemsOf("b"); at < 0 || strings.Join(items, ",") != "loop.completed:b,loop.completed:c" {
		t.Fatalf("event %d items %v, want b then c in one event", at, items)
	}
	if n := l.pendingMembers(); n != 0 {
		t.Fatalf("%d members left pending", n)
	}
	// The token was freed: a later write commits at once.
	if err := l.store.AppendEvent(context.Background(), l.event("d", "run-d", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatal(err)
	}
}

// Invariant 2, after the take: a leader that panics after taking settles every
// taken member with an error and strands none.
func TestGroupCommitLateTakePanicAfterTakeSettlesTaken(t *testing.T) {
	l := newLateHarness(t)
	l.holdFlush()
	defer l.releaseFlush()
	won := make(chan struct{}, 1)
	wonHook := func() {
		select {
		case won <- struct{}{}:
		default:
		}
	}
	l.store.tapeGroup.afterToken.Store(&wonHook)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				l.results <- writeResult{"a", errors.New("leader panicked")}
			}
		}()
		err := l.store.AppendEvent(context.Background(), l.event("a", "run-a", "owner-1", types.EventRunCompleted, telemetryBase))
		l.results <- writeResult{"a", err}
	}()
	<-won
	l.store.tapeGroup.afterToken.Store(nil)
	l.startMember(context.Background(), "b", 2)
	l.tape.mu.Lock()
	l.tape.casErr = func(computerevent.CASRequest) error { panic("cas panics after the take") }
	l.tape.mu.Unlock()
	l.releaseFlush()
	got := l.collect(2)
	for id, err := range got {
		if err == nil {
			t.Fatalf("%s acknowledged though the leader panicked", id)
		}
	}
	if b := got["b"]; b == nil || !strings.Contains(b.Error(), "leader panicked") {
		t.Fatalf("b: %v, want the leader-panicked error", b)
	}
}

// The counters: len(pending) at take time, and the members the early snapshot
// would have missed.
func TestGroupCommitLateTakeCounters(t *testing.T) {
	l := newLateHarness(t)
	before := l.store.TelemetryStats().GroupCommit
	l.holdFlush()
	defer l.releaseFlush()
	l.startLeader("a")
	l.startMember(context.Background(), "b", 2)
	l.startMember(context.Background(), "c", 3)
	l.releaseFlush()
	for id, err := range l.collect(3) {
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	after := l.store.TelemetryStats().GroupCommit
	if d := after.LateJoiners - before.LateJoiners; d != 2 {
		t.Fatalf("late joiners +%d, want 2 (b and c arrived after a won the token)", d)
	}
	if d := after.PendingAtTake["3-4"] - before.PendingAtTake["3-4"]; d != 1 {
		t.Fatalf("pending_at_take %v, want one take of 3 pending", after.PendingAtTake)
	}
	for i, want := range []struct {
		n     int
		label string
	}{{1, "1"}, {2, "2"}, {3, "3-4"}, {4, "3-4"}, {5, "5-8"}, {8, "5-8"}, {9, "9+"}, {500, "9+"}} {
		if got := pendingAtTakeLabels[pendingAtTakeBucket(want.n)]; got != want.label {
			t.Fatalf("case %d: %d pending is bucket %q, want %q", i, want.n, got, want.label)
		}
	}
}
