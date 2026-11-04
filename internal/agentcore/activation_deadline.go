package agentcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The activation deadline (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md,
// "a quota park is cancelled by the activation deadline before the reset").
//
// An activation's progress deadline is its budget from its start. A loop
// parked on an exhausted provider quota makes no progress by design, so the
// park suspends the deadline: the time parked is not charged, and the
// deadline moves out by it when the park ends. The suspension is bounded by
// the activation's hard wall (activationHardWall from its start): a loop
// still parked there ends with a fate that names the quota it waited for.
//
// The context is process memory: a crash ends the activation, and the boot
// rule closes its run (CLAUDE.md, restarts). The durable
// activation_budget_deadline continuation is armed at the first deadline;
// when it fires on a live activation whose deadline moved, it re-arms at the
// new one instead of closing the run (HandleActivationBudgetDeadline).

// activationHardWall bounds an activation from its start, park extensions
// included. RUNTIME_ACTIVATION_HARD_WALL (a Go duration) overrides it; it is
// never below the activation's budget.
var activationHardWall = func() time.Duration {
	if v := strings.TrimSpace(os.Getenv("RUNTIME_ACTIVATION_HARD_WALL")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return 6 * time.Hour
}()

// errActivationHardWall is the cause of an activation ended at its hard wall.
var errActivationHardWall = errors.New("activation hard wall reached")

type activationDeadlineKey struct{}

// activationDeadline is a context whose deadline a provider quota park
// suspends. Err is context.DeadlineExceeded at the deadline or the hard
// wall, or the parent's error.
type activationDeadline struct {
	parent context.Context
	start  time.Time

	mu       sync.Mutex
	deadline time.Time
	hardWall time.Time
	// parkedSince is non-zero while a park suspends the deadline; parked is
	// the wait it parks on, for the fate.
	parkedSince time.Time
	parked      *types.WaitState
	suspended   time.Duration
	timer       *time.Timer
	done        chan struct{}
	err         error
	hitWall     bool
}

func newActivationDeadline(parent context.Context, budget time.Duration) (*activationDeadline, context.CancelFunc) {
	now := time.Now()
	wall := activationHardWall
	if wall < budget {
		wall = budget
	}
	a := &activationDeadline{parent: parent, start: now, deadline: now.Add(budget), hardWall: now.Add(wall), done: make(chan struct{})}
	if pd, ok := parent.Deadline(); ok {
		if pd.Before(a.deadline) {
			a.deadline = pd
		}
		if pd.Before(a.hardWall) {
			a.hardWall = pd
		}
	}
	a.mu.Lock()
	a.armLocked(now)
	a.mu.Unlock()
	stop := make(chan struct{})
	go func() {
		select {
		case <-parent.Done():
			a.finish(parent.Err(), false)
		case <-a.done:
		case <-stop:
		}
	}()
	var once sync.Once
	return a, func() {
		once.Do(func() {
			a.finish(context.Canceled, false)
			close(stop)
		})
	}
}

// effectiveLocked is when the activation ends now: the hard wall while
// parked, else the deadline.
func (a *activationDeadline) effectiveLocked() time.Time {
	if !a.parkedSince.IsZero() {
		return a.hardWall
	}
	return a.deadline
}

func (a *activationDeadline) armLocked(now time.Time) {
	if a.err != nil {
		return
	}
	if a.timer != nil {
		a.timer.Stop()
	}
	at := a.effectiveLocked()
	a.timer = time.AfterFunc(max(at.Sub(now), 0), a.fire)
}

func (a *activationDeadline) fire() {
	a.mu.Lock()
	now := time.Now()
	at := a.effectiveLocked()
	if now.Before(at) {
		// Moved since this timer was armed.
		a.armLocked(now)
		a.mu.Unlock()
		return
	}
	wall := !a.parkedSince.IsZero() || !now.Before(a.hardWall)
	a.mu.Unlock()
	a.finish(context.DeadlineExceeded, wall)
}

func (a *activationDeadline) finish(err error, wall bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return
	}
	a.err, a.hitWall = err, wall && errors.Is(err, context.DeadlineExceeded)
	if a.timer != nil {
		a.timer.Stop()
	}
	close(a.done)
}

// suspend stops charging the deadline while the run parks on w; repeated
// calls during one park only update the wait.
func (a *activationDeadline) suspend(w *types.WaitState) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.parked = w.Clone()
	if !a.parkedSince.IsZero() || a.err != nil {
		return
	}
	now := time.Now()
	a.parkedSince = now
	a.armLocked(now)
}

// resume ends the suspension: the deadline moves out by the time parked,
// never past the hard wall.
func (a *activationDeadline) resume() {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.parkedSince.IsZero() || a.err != nil {
		// An activation that ended while parked keeps the park for its fate.
		return
	}
	now := time.Now()
	parked := now.Sub(a.parkedSince)
	a.suspended += parked
	a.deadline = a.deadline.Add(parked)
	if a.deadline.After(a.hardWall) {
		a.deadline = a.hardWall
	}
	a.parkedSince, a.parked = time.Time{}, nil
	a.armLocked(now)
}

// endsAt is when the activation ends if nothing changes, whether it is
// over, and whether a park moved its deadline (it parks now or has parked):
// only a moved deadline is re-armed by the durable continuation.
func (a *activationDeadline) endsAt() (time.Time, bool, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.effectiveLocked(), a.err != nil, !a.parkedSince.IsZero() || a.suspended > 0
}

// fate is the run's error when the activation ended at its deadline or wall.
func (a *activationDeadline) fate() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	// The durable wake can beat the in-process timer at the wall: past the
	// wall is the wall's fate either way (review r1 Devin, non-blocking).
	if !a.hitWall && time.Now().Before(a.hardWall) {
		return activationBudgetExceededReason
	}
	msg := fmt.Sprintf("%s: the activation's hard wall (%s from its start) passed", errActivationHardWall, shortWaitDuration(a.hardWall.Sub(a.start)))
	if w := a.parked; w.Active() {
		msg += " while it " + w.SentenceAt(time.Now())
		if !w.ExpectedUntil.IsZero() {
			msg += "; the quota was expected back at " + w.ExpectedUntil.UTC().Format(time.RFC3339)
		}
	}
	return msg
}

func (a *activationDeadline) Deadline() (time.Time, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.parkedSince.IsZero() {
		// The deadline as it will stand if the park ended now.
		d := a.deadline.Add(time.Since(a.parkedSince))
		if d.After(a.hardWall) {
			d = a.hardWall
		}
		return d, true
	}
	return a.deadline, true
}

func (a *activationDeadline) Done() <-chan struct{} { return a.done }

func (a *activationDeadline) Err() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.err
}

func (a *activationDeadline) Value(key any) any {
	if key == (activationDeadlineKey{}) {
		return a
	}
	return a.parent.Value(key)
}

// activationDeadlineFrom is ctx's activation deadline, if any.
func activationDeadlineFrom(ctx context.Context) *activationDeadline {
	if ctx == nil {
		return nil
	}
	a, _ := ctx.Value(activationDeadlineKey{}).(*activationDeadline)
	return a
}

// activationFate is the run's error for an activation context that ended
// at its deadline: the budget, or the hard wall naming the park.
func activationFate(ctx context.Context) string {
	if a := activationDeadlineFrom(ctx); a != nil {
		return a.fate()
	}
	return activationBudgetExceededReason
}

// liveActivations maps a run to its activation deadline while it runs.
type liveActivations struct{ m sync.Map }

func (l *liveActivations) put(runID string, a *activationDeadline) { l.m.Store(runID, a) }
func (l *liveActivations) drop(runID string, a *activationDeadline) {
	l.m.CompareAndDelete(runID, a)
}
func (l *liveActivations) get(runID string) *activationDeadline {
	v, _ := l.m.Load(runID)
	a, _ := v.(*activationDeadline)
	return a
}
