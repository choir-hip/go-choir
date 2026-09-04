package actor

import (
	"sync"
	"time"
)

// Bounded adaptive coalescing for parent wakes (RLM Target Architecture, Step
// 5): child completions stream into the parent's mailbox, and the supervisor
// wakes the parent once per batch instead of once per child. The wake is
// bounded on every axis, so no join barrier can deadlock on a hung child:
//
//   - all-complete fires immediately when every expected child settled;
//   - quiescence fires after a debounce window with no new events;
//   - timeout fires whatever settled by the deadline, stragglers stay pending;
//   - error tombstones ride the same debounce, releasing the wait promptly.
//
// A Coalescer fires exactly once. It is the supervisor's batching primitive;
// delivery itself stays on the Go-channel mailbox ("Go delivers").

// WakeReason names the bound that released a batch.
type WakeReason int

const (
	// WakeAllComplete fired because every expected child settled.
	WakeAllComplete WakeReason = iota
	// WakeQuiescence fired after a silent window: no new events arrived.
	WakeQuiescence
	// WakeTimeout fired at the deadline with partial results.
	WakeTimeout
	// WakeTombstone fired after a child failure released the wait.
	WakeTombstone
)

// WakeBatch is one parent wake: settled child IDs plus failure tombstones.
type WakeBatch struct {
	Completed  []string
	Tombstones map[string]string
	Reason     WakeReason
}

// DefaultCoalesceWindow is the quiescence debounce: 500ms of silence ends
// the batch.
const DefaultCoalesceWindow = 500 * time.Millisecond

// Coalescer batches child settlements into one parent wake.
type Coalescer struct {
	mu         sync.Mutex
	want       int
	window     time.Duration
	completed  map[string]bool
	tombstones map[string]string
	fired      bool
	timer      *time.Timer
	deadline   *time.Timer
	out        chan WakeBatch
}

// NewCoalescer batches want child settlements: window bounds quiescence,
// timeout bounds the whole batch. want <= 0 means unknown cardinality: only
// quiescence and timeout can fire.
func NewCoalescer(want int, window, timeout time.Duration) *Coalescer {
	if window <= 0 {
		window = DefaultCoalesceWindow
	}
	c := &Coalescer{
		want:       want,
		window:     window,
		completed:  map[string]bool{},
		tombstones: map[string]string{},
		out:        make(chan WakeBatch, 1),
	}
	if timeout > 0 {
		c.deadline = time.AfterFunc(timeout, func() { c.fire(WakeTimeout) })
	}
	return c
}

// Complete records a child completion. The final expected settlement fires
// immediately; anything else restarts the quiescence window.
func (c *Coalescer) Complete(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fired || c.completed[id] {
		return
	}
	c.completed[id] = true
	if c.settledLocked() {
		c.fireLocked(WakeAllComplete)
		return
	}
	c.debounceLocked()
}

// Fail records a child failure as an error tombstone. The tombstone releases
// the parent's wait through the same debounce: prompt, but still coalesced
// with completions racing it in the window.
func (c *Coalescer) Fail(id, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fired {
		return
	}
	c.completed[id] = true
	c.tombstones[id] = reason
	if c.settledLocked() {
		if len(c.tombstones) > 0 {
			c.fireLocked(WakeTombstone)
		} else {
			c.fireLocked(WakeAllComplete)
		}
		return
	}
	c.debounceLocked()
}

// Wait returns the batch channel. It always yields exactly one batch: every
// bound fires, so no waiter blocks forever.
func (c *Coalescer) Wait() <-chan WakeBatch {
	return c.out
}

// Stop releases timers for a batch nobody waits on.
func (c *Coalescer) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
	if c.deadline != nil {
		c.deadline.Stop()
	}
}

func (c *Coalescer) settledLocked() bool {
	return c.want > 0 && len(c.completed) >= c.want
}

func (c *Coalescer) debounceLocked() {
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(c.window, func() { c.fire(WakeQuiescence) })
}

func (c *Coalescer) fire(reason WakeReason) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fireLocked(reason)
}

func (c *Coalescer) fireLocked(reason WakeReason) {
	if c.fired {
		return
	}
	c.fired = true
	if c.timer != nil {
		c.timer.Stop()
	}
	if c.deadline != nil {
		c.deadline.Stop()
	}
	completed := make([]string, 0, len(c.completed))
	for id := range c.completed {
		if _, failed := c.tombstones[id]; !failed {
			completed = append(completed, id)
		}
	}
	tombstones := make(map[string]string, len(c.tombstones))
	for id, cause := range c.tombstones {
		tombstones[id] = cause
	}
	if reason == WakeAllComplete && len(tombstones) > 0 {
		reason = WakeTombstone
	}
	c.out <- WakeBatch{Completed: completed, Tombstones: tombstones, Reason: reason}
}
