package actor

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// docs/problems/activation-wait-times-2026-11-09.md, fix 1: d@e's management
// desk waited 3 to 8 minutes with 1 to 2 of 8 slots used, and the wait line
// could not say why (H1, the serial fence). The line now names the fence that
// skipped the agent. Observation only; ways it can fail, written before it:
//   - the skip bookkeeping changes a scheduling decision (a busy, held or
//     roomless agent starts anyway, or a free one is held back);
//   - the line names the wrong fence, or keeps a stale cause into the next
//     activation;
//   - the line grows with the backlog (kinds are capped).

func waitCauseDispatcher(t *testing.T, opts DispatcherOptions) (*Dispatcher, *atomic.Int32) {
	t.Helper()
	l := openKernelLog(t)
	var handled atomic.Int32
	h := HandlerFunc(func(ctx context.Context, agentID string, u Update, mem []byte) ([]byte, error) {
		handled.Add(1)
		return mem, nil
	})
	d := NewDispatcher(l, h, opts)
	now := time.Now().UTC()
	appendAt(t, l, "u1", "agent:a", now.Add(-2*time.Minute), time.Time{})
	return d, &handled
}

func TestWaitLineNamesTheSerialFenceAndLeavesTheDecisionAlone(t *testing.T) {
	d, handled := waitCauseDispatcher(t, DispatcherOptions{MaxConcurrent: 8})
	holder := time.Now().Add(-90 * time.Second)
	d.running["agent:a"] = holder // its own activation is in flight

	d.dispatchPending(context.Background())
	d.wg.Wait()
	if handled.Load() != 0 {
		t.Fatal("a busy agent was activated: the cause bookkeeping changed the fence")
	}
	line, ok := d.activationWaitLine("agent:a", []Update{
		{UpdateID: "u1", Kind: "inbox_record", CreatedAt: time.Now().Add(-2 * time.Minute)},
		{UpdateID: "u2", Kind: "inbox_record", CreatedAt: time.Now().Add(-time.Minute)},
		{UpdateID: "u3", Kind: "coagent_result", CreatedAt: time.Now().Add(-time.Minute)},
	}, time.Now())
	if !ok {
		t.Fatal("no wait line for a 2 minute wait")
	}
	for _, want := range []string{"agent=agent:a", "slots=1/8", "cause=busy", "holder_for=1m", "kinds=inbox_record:2,coagent_result:1"} {
		if !strings.Contains(line, want) {
			t.Errorf("wait line %q lacks %q", line, want)
		}
	}
	// The note is consumed: the next activation does not inherit the cause.
	again, _ := d.activationWaitLine("agent:a", []Update{{UpdateID: "u9", Kind: "inbox_record", CreatedAt: time.Now().Add(-time.Minute)}}, time.Now())
	if !strings.Contains(again, "cause=none") {
		t.Errorf("a later wait line kept a stale cause: %q", again)
	}
}

func TestWaitLineNamesARetryHoldAndAFullSlotTable(t *testing.T) {
	d, handled := waitCauseDispatcher(t, DispatcherOptions{MaxConcurrent: 1})
	d.heldUntil["agent:a"] = time.Now().Add(time.Minute)
	d.dispatchPending(context.Background())
	if handled.Load() != 0 {
		t.Fatal("a held agent was activated")
	}
	line, _ := d.activationWaitLine("agent:a", []Update{{UpdateID: "u1", Kind: "k", CreatedAt: time.Now().Add(-2 * time.Minute)}}, time.Now())
	if !strings.Contains(line, "cause=held") || strings.Contains(line, "holder_for") {
		t.Errorf("held wait line = %q", line)
	}

	// No hold; another agent fills the one shared slot.
	delete(d.heldUntil, "agent:a")
	d.running["agent:other"] = time.Now()
	d.dispatchPending(context.Background())
	if handled.Load() != 0 {
		t.Fatal("an agent with no free slot was activated")
	}
	line, _ = d.activationWaitLine("agent:a", []Update{{UpdateID: "u1", Kind: "k", CreatedAt: time.Now().Add(-2 * time.Minute)}}, time.Now())
	if !strings.Contains(line, "cause=noroom") || !strings.Contains(line, "slots=1/1") {
		t.Errorf("no-room wait line = %q", line)
	}
}

func TestWaitLineIsQuietForAShortWaitAndCapsKinds(t *testing.T) {
	d, _ := waitCauseDispatcher(t, DispatcherOptions{})
	if line, ok := d.activationWaitLine("agent:a", []Update{{UpdateID: "u1", Kind: "k", CreatedAt: time.Now().Add(-time.Second)}}, time.Now()); ok {
		t.Fatalf("a 1 second wait logged: %q", line)
	}
	var many []Update
	for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
		many = append(many, Update{Kind: k})
	}
	if got := queuedKinds(many); got != "a:1,b:1,c:1,d:1,+2" {
		t.Errorf("queuedKinds = %q, want the first four and a count of the rest", got)
	}
}
