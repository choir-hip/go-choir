package agentcore

import (
	"context"
	"errors"
	"time"

	"github.com/yusefmosiah/go-choir/internal/holdlock"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// In-process locks of the self-development path, bounded
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md, Part 4).
// Each had been a sync.Mutex: a waiter waited as long as the holder held
// it, and a holder wedged in a store call (the actor handler's context has
// no deadline) held every waiter with it, with nothing saying who. Now a
// waiter gives up at a bound, its error names the holder and how long it
// has held the lock, and an engineering assignment's wait on the open lock
// is a typed wait (kind lock) that management's frame and Texture's
// progress relay show.

const (
	engineeringOpenLockName    = "the engineering assignment open lock"
	selfdevMaterializeLockName = "the self-development materialize lock"
)

// engineeringOpenLockWait bounds a wait for the open lock. A holder runs
// one spawn saga (the spawn itself is bounded at 90 s) or one trajectory
// reconcile; five minutes is several of them. engineeringOpenLockQuiet is
// how long a waiter waits before its wait is shown: a passing wait is not
// management's business. Vars only so a test can shorten them.
var (
	engineeringOpenLockWait  = 5 * time.Minute
	engineeringOpenLockQuiet = 2 * time.Second
)

// selfdevMaterializeLockWait bounds a wait for the materializer's lock. Its
// waiters are the materializer pass (behind a settle) and the owner's
// settle acts; a settle's own wait is shorter (selfdevAbandonWait).
var selfdevMaterializeLockWait = 5 * time.Minute

// lockEngineeringOpen takes the open/spawn/bind saga lock for who. When
// assignment is set and the wait lasts past engineeringOpenLockQuiet, the
// wait is the assignment's typed wait until it ends.
func (rt *Runtime) lockEngineeringOpen(ctx context.Context, who string, assignment *types.EngineeringAssignment) (func(), error) {
	mu := &rt.engineeringAssignmentOpenMu
	if assignment == nil {
		return mu.Acquire(ctx, engineeringOpenLockName, who, engineeringOpenLockWait)
	}
	start := time.Now()
	release, err := mu.Acquire(ctx, engineeringOpenLockName, who, engineeringOpenLockQuiet)
	if err == nil || ctx.Err() != nil {
		return release, err
	}
	a := *assignment
	rt.observeAssignmentWait(ctx, a, func(since time.Time) *types.WaitState {
		return openLockWaitState(a, mu, since, start.Add(engineeringOpenLockWait))
	})
	// max <= 0 would mean no bound of its own: never let the rest round to it.
	release, err = mu.Acquire(ctx, engineeringOpenLockName, who, max(engineeringOpenLockWait-time.Since(start), time.Millisecond))
	outcome := "it took the lock"
	var held *holdlock.HeldError
	switch {
	case errors.As(err, &held):
		outcome = "it gave up: " + held.Error()
	case err != nil:
		outcome = "it stopped waiting: " + err.Error()
	}
	rt.endAssignmentWaitKind(context.WithoutCancel(ctx), a.AssignmentID, a.Binding.Attempt, types.WaitKindLock, outcome)
	return release, err
}

// openLockWaitState is a wait for the open lock as a typed wait.
func openLockWaitState(a types.EngineeringAssignment, mu *holdlock.Mutex, since, until time.Time) *types.WaitState {
	w := &types.WaitState{Kind: types.WaitKindLock, Class: types.WaitClassContention, Subject: "assignment " + a.AssignmentID,
		Resource: engineeringOpenLockName, Since: since.UTC(), Until: until.UTC(),
		Detail: "one engineering assignment opens, spawns and binds at a time; the waiter gives up at the bound"}
	if who, held, ok := mu.Holder(); ok {
		w.Holders = []types.WaitHolder{{Who: who, What: "holds the lock", Since: held.UTC()}}
	}
	return w
}

// lockSelfdevMaterialize takes the materializer's lock for who.
func (rt *Runtime) lockSelfdevMaterialize(ctx context.Context, who string) (func(), error) {
	return rt.selfdevMaterializeMu.Acquire(ctx, selfdevMaterializeLockName, who, selfdevMaterializeLockWait)
}

// setSelfdevCrashMarks records a crash boot's interrupted operations and
// resets the boot's spent settlement attempts. It runs in the boot phase
// selfdev_close_after_crash, before the post-commit observer that launches
// materializer passes is installed (runtime.go), so no pass reads the
// marks or spends an attempt before they land (review r2, DeepSeek R2-1).
func (rt *Runtime) setSelfdevCrashMarks(interrupted map[string]bool) {
	rt.selfdevCrashMu.Lock()
	rt.selfdevCrashInterrupted = interrupted
	rt.selfdevSettlementSpent = map[string]bool{}
	rt.selfdevCrashMu.Unlock()
}

// selfdevCrashInterruptedOp reports whether a crash boot marked the
// operation interrupted.
func (rt *Runtime) selfdevCrashInterruptedOp(operationID string) bool {
	rt.selfdevCrashMu.Lock()
	defer rt.selfdevCrashMu.Unlock()
	return rt.selfdevCrashInterrupted[operationID]
}
