package agentcore

import (
	"context"
	"log"
	"sync"
	"time"
)

// SL slice 2 (docs/definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md):
// the projector is event-driven. It sweeps when the store signals a committed
// wake write, when the earliest failed dispatch is due again, or on a slow
// audit timer that exists only to expose a missed signal.

const (
	// wakeDispatchMaxAttempts bounds dispatch attempts per wake (O1).
	wakeDispatchMaxAttempts = 5
	wakeRetryBaseDelay      = time.Second
	actorWakeAuditInterval  = time.Minute
	actorWakeResweepDelay   = 50 * time.Millisecond
)

type wakeRetryState struct {
	attempts int
	next     time.Time
}

// wakeRetryTracker counts failed dispatches per wake. It is process-local by
// design: after a restart, pre-boot work follows the restart rule, and a
// wake that fails again starts a fresh, still bounded, budget.
type wakeRetryTracker struct {
	mu   sync.Mutex
	now  func() time.Time
	byID map[string]*wakeRetryState
}

func (t *wakeRetryTracker) clock() time.Time {
	if t.now != nil {
		return t.now()
	}
	return time.Now()
}

func (t *wakeRetryTracker) due(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.byID[id]
	return state == nil || !t.clock().Before(state.next)
}

// failed records a failed dispatch and reports the attempt count and whether
// the budget is spent. Backoff doubles from wakeRetryBaseDelay.
func (t *wakeRetryTracker) failed(id string, _ error) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byID == nil {
		t.byID = map[string]*wakeRetryState{}
	}
	state := t.byID[id]
	if state == nil {
		state = &wakeRetryState{}
		t.byID[id] = state
	}
	state.attempts++
	state.next = t.clock().Add(wakeRetryBaseDelay << (state.attempts - 1))
	return state.attempts, state.attempts >= wakeDispatchMaxAttempts
}

func (t *wakeRetryTracker) clear(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.byID, id)
}

// nextDue is the earliest pending retry, zero when none.
func (t *wakeRetryTracker) nextDue() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	var next time.Time
	for _, state := range t.byID {
		if next.IsZero() || state.next.Before(next) {
			next = state.next
		}
	}
	return next
}

func (rt *Runtime) runActorWakeProjector(ctx context.Context, stop <-chan struct{}) {
	signal := rt.store.ActorWakeSignal()
	reason := "start"
	for {
		out := rt.sweepActorWakeOutbox(ctx)
		if reason == "audit" && out.dispatched > 0 {
			log.Printf("runtime: actor wake outbox audit found %d ready wake(s) no signal announced (missed signal)", out.dispatched)
		}
		wait := actorWakeAuditInterval
		if rt.wakeAuditInterval > 0 {
			wait = rt.wakeAuditInterval
		}
		next := "audit"
		if out.more {
			wait, next = actorWakeResweepDelay, "more"
		} else if due := rt.wakeRetries.nextDue(); !due.IsZero() {
			if d := due.Sub(rt.wakeRetries.clock()); d < wait {
				wait, next = d, "retry"
				if wait < 0 {
					wait = 0
				}
			}
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-stop:
			timer.Stop()
			return
		case <-signal:
			reason = "signal"
		case <-timer.C:
			reason = next
		}
		timer.Stop()
	}
}
