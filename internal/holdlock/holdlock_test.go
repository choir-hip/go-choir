package holdlock

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Ways a bounded lock could fail, written before the code:
//  1. a waiter behind a held lock never returns (the hang this replaces);
//  2. it returns, but its error does not name the holder or how long it held;
//  3. a cancelled caller keeps waiting until the bound;
//  4. a waiter that gave up still takes the lock later (a leaked slot);
//  5. a double release frees a slot someone else now holds;
//  6. the holder record outlives the release (a stale name in a later error).

func TestHeldLockWaiterReturnsAtItsBoundNamingTheHolder(t *testing.T) {
	var m Mutex
	release, err := m.Acquire(context.Background(), "the test lock", "op-1 apply", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	start := time.Now()
	_, err = m.Acquire(context.Background(), "the test lock", "op-2", 50*time.Millisecond)
	if err == nil {
		t.Fatal("a waiter behind a held lock acquired it")
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Fatalf("the waiter returned after %s, past its 50ms bound", waited)
	}
	var held *HeldError
	if !errors.As(err, &held) || held.Holder != "op-1 apply" || held.Lock != "the test lock" {
		t.Fatalf("err = %#v, want a HeldError naming op-1 apply", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "waiting for the test lock held by op-1 apply for ") {
		t.Fatalf("err = %q, want the holder and its hold time named", msg)
	}
}

func TestCancelledWaiterReturnsBeforeItsBound(t *testing.T) {
	var m Mutex
	release, _ := m.Acquire(context.Background(), "the test lock", "holder", time.Second)
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := m.Acquire(ctx, "the test lock", "waiter", time.Hour)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled waiter kept waiting")
	}
}

func TestGivenUpWaiterLeaksNoSlotAndReleaseIsIdempotent(t *testing.T) {
	var m Mutex
	release, _ := m.Acquire(context.Background(), "the test lock", "first", time.Second)
	if _, err := m.Acquire(context.Background(), "the test lock", "gave-up", 10*time.Millisecond); err == nil {
		t.Fatal("acquired a held lock")
	}
	release()
	release() // a second release must not free a slot someone else holds
	if who, _, held := m.Holder(); held {
		t.Fatalf("holder %q outlived its release", who)
	}
	second, ok := m.TryAcquire("second")
	if !ok {
		t.Fatal("the lock stayed taken after its holder released it")
	}
	release() // stale release from the first holder
	if _, ok := m.TryAcquire("third"); ok {
		t.Fatal("a stale release freed the second holder's slot")
	}
	if who, since, held := m.Holder(); !held || who != "second" || since.IsZero() {
		t.Fatalf("holder = %q %v %v, want second", who, since, held)
	}
	second()
}

func TestWaiterTakesTheLockWhenFreed(t *testing.T) {
	var m Mutex
	release, _ := m.Acquire(context.Background(), "the test lock", "first", time.Second)
	go func() {
		time.Sleep(20 * time.Millisecond)
		release()
	}()
	next, err := m.Acquire(context.Background(), "the test lock", "next", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if who, _, _ := m.Holder(); who != "next" {
		t.Fatalf("holder = %q, want next", who)
	}
	next()
}
