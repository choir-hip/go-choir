// Package holdlock is a mutual-exclusion lock whose waiters give up at a
// deadline and say who held it
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md, class c).
//
// A sync.Mutex waiter blocks until the holder lets go; when the holder hangs,
// so does every waiter, and nothing records who holds it or since when. A
// Mutex here is a one-slot channel semaphore: Acquire waits on the caller's
// context and a bound of its own, records the holder and the time it took
// the lock, and a waiter that gives up returns a HeldError naming both:
// "waiting for the self-development materialize lock held by materializer
// pass for 4m10s". The zero value is an unlocked Mutex.
package holdlock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Mutex is a lock with a holder record and bounded waiters.
type Mutex struct {
	init   sync.Once
	slot   chan struct{}
	mu     sync.Mutex
	holder string
	since  time.Time
	// gen counts acquisitions, so a stale release frees nothing.
	gen uint64
}

func (m *Mutex) ch() chan struct{} {
	m.init.Do(func() { m.slot = make(chan struct{}, 1) })
	return m.slot
}

// HeldError is a waiter that gave up: Lock names the lock, Holder who held
// it, HeldFor how long it had held it, Waited how long this waiter waited.
// It wraps the cause: context.DeadlineExceeded at the bound, or the
// caller's context error.
type HeldError struct {
	Lock    string
	Holder  string
	HeldFor time.Duration
	Waited  time.Duration
	Cause   error
}

func (e *HeldError) Error() string {
	holder := e.Holder
	if holder == "" {
		holder = "a holder that just let go"
	}
	return fmt.Sprintf("waiting for %s held by %s for %s: gave up after %s: %v",
		e.Lock, holder, e.HeldFor.Round(time.Second), e.Waited.Round(time.Millisecond), e.Cause)
}

func (e *HeldError) Unwrap() error { return e.Cause }

// Acquire takes the lock for who, waiting at most max (no bound of its own
// when max <= 0; the caller's context then bounds it). It returns the
// release, which is safe to call more than once. A free lock is taken even
// when ctx is already done: the caller checks its context inside the
// critical section, as it would after sync.Mutex.Lock.
func (m *Mutex) Acquire(ctx context.Context, lock, who string, max time.Duration) (func(), error) {
	if release, ok := m.TryAcquire(who); ok {
		return release, nil
	}
	start := time.Now()
	waitCtx := ctx
	if max > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, max)
		defer cancel()
	}
	select {
	case m.ch() <- struct{}{}:
		return m.took(who), nil
	case <-waitCtx.Done():
		holder, since, _ := m.Holder()
		held := time.Duration(0)
		if !since.IsZero() {
			held = time.Since(since)
		}
		cause := ctx.Err()
		if cause == nil {
			cause = context.DeadlineExceeded
		}
		return nil, &HeldError{Lock: lock, Holder: holder, HeldFor: held, Waited: time.Since(start), Cause: cause}
	}
}

// TryAcquire takes the lock for who if it is free.
func (m *Mutex) TryAcquire(who string) (func(), bool) {
	select {
	case m.ch() <- struct{}{}:
		return m.took(who), true
	default:
		return nil, false
	}
}

func (m *Mutex) took(who string) func() {
	m.mu.Lock()
	m.gen++
	gen := m.gen
	m.holder, m.since = who, time.Now()
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		if m.gen != gen {
			m.mu.Unlock()
			return
		}
		m.holder, m.since = "", time.Time{}
		m.gen++ // this release is spent
		m.mu.Unlock()
		<-m.ch()
	}
}

// Holder reports who holds the lock and since when.
func (m *Mutex) Holder() (who string, since time.Time, held bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.holder, m.since, !m.since.IsZero()
}
