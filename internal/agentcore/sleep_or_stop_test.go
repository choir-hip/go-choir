package agentcore

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// The background sleeps of the runtime end at Stop
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md, Part 4).
// Ways they could fail, written before the code:
//  1. a goroutine sleeps through Stop and then acts on a stopped runtime:
//     restarts the guest, writes a run's end, reads /proc;
//  2. Stop is seen only after the whole sleep (up to an hour);
//  3. a sleep that is not stopped returns early.

func stopRuntimeBackground(rt *Runtime) {
	rt.bgStopOnce.Do(func() { close(rt.backgroundStop()) })
}

func TestSleepOrStopEndsAtStop(t *testing.T) {
	rt := &Runtime{}
	start := time.Now()
	if !rt.sleepOrStop(20 * time.Millisecond) {
		t.Fatal("a sleep with no Stop reported a stop")
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Fatal("the sleep returned early")
	}
	done := make(chan bool, 1)
	go func() { done <- rt.sleepOrStop(time.Hour) }()
	time.Sleep(20 * time.Millisecond)
	stopRuntimeBackground(rt)
	select {
	case slept := <-done:
		if slept {
			t.Fatal("a stopped sleep reported a full sleep")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an hour's sleep ignored Stop")
	}
}

func TestRunEndRetryStopsAtStop(t *testing.T) {
	attempts := 0
	persisted, err := retryRunEnd(func() (bool, error) {
		attempts++
		return false, errors.New("store down")
	}, nil, func(time.Duration) bool { return false }, []time.Duration{time.Hour, time.Hour})
	if persisted || err == nil || attempts != 1 {
		t.Fatalf("persisted=%v err=%v attempts=%d, want one attempt then the error at Stop", persisted, err, attempts)
	}
}

func TestRestoreRestartIdlePollerEndsAtStop(t *testing.T) {
	rt := fencedHibernateRuntime(t)
	restarted := 0
	rt.selfdevRestartCurrent = func(context.Context) error { restarted++; return nil }
	prevPoll, prevWindow := restoreRestartIdlePoll, restoreRestartIdleWindow
	restoreRestartIdlePoll, restoreRestartIdleWindow = 20*time.Millisecond, 5*time.Second
	t.Cleanup(func() { restoreRestartIdlePoll, restoreRestartIdleWindow = prevPoll, prevWindow })
	if _, ok := rt.hibernate.close(time.Minute); !ok {
		t.Fatal("setup: fence did not close")
	}
	if scheduled, deferred := rt.requestRestoreRestart(context.Background(), "ckpt"); scheduled || deferred == "" {
		t.Fatalf("scheduled=%v deferred=%q, want deferred while fenced", scheduled, deferred)
	}
	stopRuntimeBackground(rt)
	time.Sleep(60 * time.Millisecond)
	rt.hibernate.lift()
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(PlannedRestartMarkerPath(rt.cfg.StorePath)); !os.IsNotExist(err) {
		t.Fatalf("the idle poller wrote a marker after Stop: %v", err)
	}
	if restarted != 0 {
		t.Fatalf("restarted %d times after Stop", restarted)
	}
}

func TestRestoreRestartDelayEndsAtStop(t *testing.T) {
	rt := fencedHibernateRuntime(t)
	restarted := make(chan struct{}, 1)
	rt.selfdevRestartCurrent = func(context.Context) error { restarted <- struct{}{}; return nil }
	prev := restoreRestartDelay
	restoreRestartDelay = 100 * time.Millisecond
	t.Cleanup(func() { restoreRestartDelay = prev })
	if scheduled, deferred := rt.requestRestoreRestart(context.Background(), "ckpt"); !scheduled {
		t.Fatalf("scheduled=%v deferred=%q, want scheduled on an idle computer", scheduled, deferred)
	}
	stopRuntimeBackground(rt)
	select {
	case <-restarted:
		t.Fatal("the restore restarted the guest after Stop")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestCacheReturnPostReadingEndsAtStop(t *testing.T) {
	rt := &Runtime{cacheReturnPostDelay: time.Hour}
	logged := make(chan string, 1)
	rt.cacheReturnLog = func(line string) { logged <- line }
	done := make(chan struct{})
	go func() {
		rt.cacheReturnPostReading()
		close(done)
	}()
	stopRuntimeBackground(rt)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the post-act reading slept through Stop")
	}
	if len(logged) != 0 {
		t.Fatalf("the reading ran after Stop: %q", <-logged)
	}
}
