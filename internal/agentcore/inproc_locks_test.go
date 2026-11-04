package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/holdlock"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The in-process locks of the self-development path
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md, Part 4).
// Ways they could fail, written before the code:
//  1. a waiter behind a wedged holder never returns;
//  2. it returns, but says nothing of who held the lock or for how long;
//  3. an engineering assignment waiting on the open lock is invisible to
//     management (no typed wait in its frame);
//  4. the lock wait outlives the acquisition, so management sees a wait
//     that ended;
//  5. a crash boot's marking waits out a materializer pass (an apply);
//  6. every commit during a pass launches another drain that blocks on the
//     pass's lock, so goroutines pile up behind one apply.

func shortenOpenLockWait(t *testing.T, quiet, total time.Duration) {
	t.Helper()
	prevQuiet, prevTotal := engineeringOpenLockQuiet, engineeringOpenLockWait
	engineeringOpenLockQuiet, engineeringOpenLockWait = quiet, total
	t.Cleanup(func() { engineeringOpenLockQuiet, engineeringOpenLockWait = prevQuiet, prevTotal })
}

func lockTestAssignment() types.EngineeringAssignment {
	a := types.EngineeringAssignment{AssignmentID: "a-lock", Disposition: types.EngineeringAssignmentOpen}
	a.Binding.OwnerID, a.Binding.ComputerID, a.Binding.Attempt = "owner", "computer", 1
	return a
}

func TestEngineeringOpenLockWaiterGivesUpNamingTheHolderAndShowsTheWait(t *testing.T) {
	shortenOpenLockWait(t, 10*time.Millisecond, 300*time.Millisecond)
	rt := &Runtime{}
	hold, err := rt.engineeringAssignmentOpenMu.Acquire(context.Background(), engineeringOpenLockName, "spawn saga of assignment a-other attempt 1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	a := lockTestAssignment()
	done := make(chan error, 1)
	go func() {
		release, err := rt.lockEngineeringOpen(context.Background(), "spawn saga of assignment a-lock attempt 1", &a)
		if release != nil {
			release()
		}
		done <- err
	}()
	// While it waits, management's frame shows a lock wait naming the holder.
	var seen *types.WaitState
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline) && seen == nil; time.Sleep(5 * time.Millisecond) {
		for _, v := range rt.assignmentWaitViews("owner", "computer") {
			if v.Wait != nil && v.Wait.Kind == types.WaitKindLock {
				seen = v.Wait
			}
		}
	}
	if seen == nil {
		t.Fatal("the open-lock wait never showed in management's frame")
	}
	if seen.Class != types.WaitClassContention || len(seen.Holders) != 1 || seen.Holders[0].Who != "spawn saga of assignment a-other attempt 1" || seen.Until.IsZero() {
		t.Fatalf("wait = %+v, want contention naming the holder with a bound", seen)
	}
	select {
	case err := <-done:
		var held *holdlock.HeldError
		if !errors.As(err, &held) || held.Holder != "spawn saga of assignment a-other attempt 1" {
			t.Fatalf("err = %v, want the holder named", err)
		}
		if !strings.Contains(err.Error(), "waiting for the engineering assignment open lock held by spawn saga of assignment a-other attempt 1 for ") {
			t.Fatalf("err = %q", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter never returned behind a held open lock")
	}
	// The wait ended with the give-up: the entry stays (its tally), with
	// nothing current.
	views := rt.assignmentWaitViews("owner", "computer")
	if len(views) != 1 || views[0].Wait != nil {
		t.Fatalf("views = %+v, want one entry with no current wait", views)
	}
}

func TestEngineeringOpenLockWaitEndsWhenTheLockIsTaken(t *testing.T) {
	shortenOpenLockWait(t, 10*time.Millisecond, 5*time.Second)
	rt := &Runtime{}
	hold, _ := rt.engineeringAssignmentOpenMu.Acquire(context.Background(), engineeringOpenLockName, "reconcile of trajectory t-1", time.Second)
	go func() {
		time.Sleep(100 * time.Millisecond)
		hold()
	}()
	a := lockTestAssignment()
	release, err := rt.lockEngineeringOpen(context.Background(), "spawn saga of assignment a-lock attempt 1", &a)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if who, _, _ := rt.engineeringAssignmentOpenMu.Holder(); who != "spawn saga of assignment a-lock attempt 1" {
		t.Fatalf("holder = %q", who)
	}
	for _, v := range rt.assignmentWaitViews("owner", "computer") {
		if v.Wait != nil {
			t.Fatalf("the lock wait outlived the acquisition: %+v", v.Wait)
		}
	}
}

func TestSelfdevMaterializeLockWaiterGivesUpNamingTheHolder(t *testing.T) {
	prev := selfdevMaterializeLockWait
	selfdevMaterializeLockWait = 50 * time.Millisecond
	t.Cleanup(func() { selfdevMaterializeLockWait = prev })
	rt := &Runtime{}
	hold, err := rt.lockSelfdevMaterialize(context.Background(), "materializer pass")
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	start := time.Now()
	_, err = rt.lockSelfdevMaterialize(context.Background(), "abandon of operation op-1")
	if err == nil || !strings.Contains(err.Error(), "waiting for the self-development materialize lock held by materializer pass for ") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the waiter outlived its bound")
	}
}

func TestCrashBootMarkingDoesNotWaitForAMaterializerPass(t *testing.T) {
	rt := &Runtime{}
	hold, _ := rt.lockSelfdevMaterialize(context.Background(), "materializer pass")
	defer hold()
	done := make(chan struct{})
	go func() {
		rt.setSelfdevCrashMarks(map[string]bool{"op-1": true})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the crash boot's marking waited for the materializer pass")
	}
	if !rt.selfdevCrashInterruptedOp("op-1") {
		t.Fatal("the mark was lost")
	}
}

func TestSelfdevReconcileTriggersDuringAPassLaunchNoSecondDrain(t *testing.T) {
	rt := &Runtime{}
	passes := make(chan struct{}, 64)
	gate := make(chan struct{})
	running := 0
	maxRunning := 0
	var mu = make(chan struct{}, 1)
	rt.selfdevReconcilePass = func() {
		mu <- struct{}{}
		running++
		maxRunning = max(maxRunning, running)
		<-mu
		passes <- struct{}{}
		<-gate // hang-guard: the test closes gate below
		mu <- struct{}{}
		running--
		<-mu
	}
	rt.startSelfdevReconcileDrain()
	select {
	case <-passes:
	case <-time.After(2 * time.Second):
		t.Fatal("the first pass never ran")
	}
	// Fifty commits during the pass: one more pass at most, never fifty
	// goroutines blocked behind the first.
	for range 50 {
		rt.startSelfdevReconcileDrain()
	}
	close(gate)
	select {
	case <-passes:
	case <-time.After(2 * time.Second):
		t.Fatal("the commits during the pass were never reconciled")
	}
	deadline := time.Now().Add(2 * time.Second)
	for rt.selfdevReconcileRunning.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if rt.selfdevReconcileRunning.Load() {
		t.Fatal("the drain never finished")
	}
	if extra := len(passes); extra != 0 {
		t.Fatalf("%d further passes ran for one burst", extra)
	}
	if maxRunning != 1 {
		t.Fatalf("%d passes ran at once", maxRunning)
	}
}

// holdForTest takes a lock for a test with no bound of its own, as the
// sync.Mutex it replaced did.
func holdForTest(m *holdlock.Mutex) func() {
	release, err := m.Acquire(context.Background(), "a test's lock", "the test", 0)
	if err != nil {
		panic(err)
	}
	return release
}
