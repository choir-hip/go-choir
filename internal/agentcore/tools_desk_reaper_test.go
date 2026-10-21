package agentcore

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// fakeReaperClock is a settable clock plus a capture of the one reaper timer
// the pool arms, so a test fires the reaper without waiting retention out.
type fakeReaperClock struct {
	mu     sync.Mutex
	now    time.Time
	armed  []time.Duration
	fire   func()
	active bool
}

func (c *fakeReaperClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeReaperClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func (c *fakeReaperClock) afterFunc(d time.Duration, f func()) deskReaperTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.armed = append(c.armed, d)
	c.fire = f
	c.active = true
	return &fakeReaperTimer{c: c}
}

type fakeReaperTimer struct{ c *fakeReaperClock }

func (t *fakeReaperTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := t.c.active
	t.c.active = false
	return was
}

// firePending runs the armed callback once, as the timer would.
func (c *fakeReaperClock) firePending(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	f, active := c.fire, c.active
	c.active = false
	c.mu.Unlock()
	if !active || f == nil {
		t.Fatal("no reaper timer armed")
	}
	f()
}

func (c *fakeReaperClock) isArmed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active
}

func newReaperPool(clock *fakeReaperClock) *deskSessionWorkers {
	w := newDeskSessionWorkers()
	w.now = clock.Now
	w.afterFunc = clock.afterFunc
	w.retention = 10 * time.Minute
	return w
}

// An idle session past retention is closed by the reaper with no acquire or
// done anywhere on the computer: the quiet computer gives its memory back.
func TestDeskReaperClosesIdleSessionWithoutAnyCell(t *testing.T) {
	deskTestWorkerBin(t)
	clock := &fakeReaperClock{now: time.Now()}
	workers := newReaperPool(clock)
	t.Cleanup(workers.closeAll)
	cfg := deskPoolCfg(t)
	lease, err := workers.acquire(context.Background(), "agent:a", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	workers.done(lease.key)
	if !clock.isArmed() {
		t.Fatal("done left an idle session with no reaper armed")
	}
	if last := clock.armed[len(clock.armed)-1]; last < 9*time.Minute || last > 10*time.Minute+time.Second {
		t.Fatalf("reaper armed for %v, want about the retention (10m)", last)
	}
	clock.advance(11 * time.Minute)
	clock.firePending(t)
	if n := workers.count(); n != 0 {
		t.Fatalf("reaper left %d idle sessions past retention", n)
	}
	if !lease.worker.Dead() {
		t.Fatal("reaper dropped the session but left its worker process alive")
	}
	if clock.isArmed() {
		t.Fatal("reaper re-armed with nothing left to watch")
	}
}

// The reaper never closes a worker mid-turn, even one whose cell has run past
// retention; it closes it only after the cell ends and retention passes again.
func TestDeskReaperNeverReapsABusySession(t *testing.T) {
	deskTestWorkerBin(t)
	clock := &fakeReaperClock{now: time.Now()}
	workers := newReaperPool(clock)
	t.Cleanup(workers.closeAll)
	cfg := deskPoolCfg(t)
	busy, err := workers.acquire(context.Background(), "agent:busy", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatalf("acquire busy: %v", err)
	}
	idle, err := workers.acquire(context.Background(), "agent:idle", true, "run-2", cfg("run-2"))
	if err != nil {
		t.Fatalf("acquire idle: %v", err)
	}
	workers.done(idle.key)
	clock.advance(time.Hour) // the busy cell has now run far past retention
	clock.firePending(t)
	if busy.worker.Dead() || workers.count() != 1 {
		t.Fatalf("reaper reaped a worker mid-turn: dead=%v live=%d", busy.worker.Dead(), workers.count())
	}
	if !idle.worker.Dead() {
		t.Fatal("reaper kept the idle session past retention")
	}
	workers.done(busy.key)
	if !clock.isArmed() {
		t.Fatal("ending the busy cell did not arm the reaper")
	}
	clock.advance(time.Minute)
	clock.firePending(t)
	if busy.worker.Dead() {
		t.Fatal("reaper closed a session one minute after its cell ended")
	}
	clock.advance(11 * time.Minute)
	// the previous firing re-armed for the remaining retention
	clock.firePending(t)
	if workers.count() != 0 {
		t.Fatal("reaper left the session once idle past retention")
	}
}

// closeAll stops the reaper so a shut-down pool leaves no timer behind.
func TestDeskReaperStopsOnCloseAll(t *testing.T) {
	deskTestWorkerBin(t)
	clock := &fakeReaperClock{now: time.Now()}
	workers := newReaperPool(clock)
	cfg := deskPoolCfg(t)
	lease, err := workers.acquire(context.Background(), "agent:a", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	workers.done(lease.key)
	workers.closeAll()
	if clock.isArmed() {
		t.Fatal("closeAll left the reaper timer armed")
	}
}

// The daemon spawns the dedicated desk-worker binary when one sits beside its
// own executable, and re-executes itself otherwise.
func TestDeskWorkerBinaryPrefersDedicatedSibling(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "autoputer")
	if got := deskWorkerBinaryFor(exe); got != exe {
		t.Fatalf("no sibling: got %q, want the daemon's own executable", got)
	}
	sibling := filepath.Join(dir, "desk-worker")
	if err := os.Mkdir(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := deskWorkerBinaryFor(exe); got != exe {
		t.Fatalf("a directory named desk-worker must not be used: got %q", got)
	}
	_ = os.Remove(sibling)
	if err := os.WriteFile(sibling, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := deskWorkerBinaryFor(exe); got != exe {
		t.Fatalf("a non-executable desk-worker must not be used: got %q", got)
	}
	if err := os.Chmod(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := deskWorkerBinaryFor(exe); got != sibling {
		t.Fatalf("got %q, want the sibling %q", got, sibling)
	}
}
