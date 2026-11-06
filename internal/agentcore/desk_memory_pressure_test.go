package agentcore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA6.4 r3: while a gVisor spawn waits for desks memory, the spawner ends
// idle sessions, least recently used first and never a busy one, across
// every pool (a research child's included). Ways it could fail: it ends a
// busy session (a cell killed mid-run), ends the wrong one (a recent
// session before an old one), misses a pool, or deadlocks with a spawn in
// progress (the spawn runs outside the pool mutex: a concurrent first cell
// of the same session must wait for it, never start a second worker).

func TestDeskMemoryPressureEndsLeastRecentlyUsedIdleSession(t *testing.T) {
	deskTestWorkerBin(t)
	cfg := deskPoolCfg(t)
	// Pools are process-wide: end the idle sessions earlier tests left, so the
	// assertions below see only this test's sessions.
	for i := 0; i < 1000 && endIdleDeskSession(); i++ {
	}
	clock := time.Now()
	desks := newDeskSessionWorkers()
	desks.now = func() time.Time { return clock }
	t.Cleanup(desks.closeAll)
	child := newResearchChildDeskWorkers("research_web", "c1")
	child.now = func() time.Time { return clock }
	t.Cleanup(child.closeAll)

	old, err := desks.acquire(context.Background(), "agent:old", true, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatal(err)
	}
	desks.done(old)
	clock = clock.Add(time.Minute)
	recent, err := child.acquire(context.Background(), "child:1", false, "run-2", cfg("run-2"))
	if err != nil {
		t.Fatal(err)
	}
	child.done(recent)
	clock = clock.Add(time.Minute)
	busy, err := desks.acquire(context.Background(), "agent:busy", true, "run-3", cfg("run-3"))
	if err != nil {
		t.Fatal(err)
	}

	if !endIdleDeskSession() {
		t.Fatal("an idle session was there to end")
	}
	if !old.worker.Dead() || recent.worker.Dead() || busy.worker.Dead() {
		t.Fatalf("want the oldest idle session ended: old %v recent %v busy %v", old.worker.Dead(), recent.worker.Dead(), busy.worker.Dead())
	}
	if !endIdleDeskSession() || !recent.worker.Dead() {
		t.Fatal("the next call ends the idle session of the other pool")
	}
	if endIdleDeskSession() {
		t.Fatal("only a busy session is left: nothing to end")
	}
	if busy.worker.Dead() {
		t.Fatal("a busy session was ended")
	}
	desks.done(busy)
}

func TestDeskSessionConcurrentFirstCellsShareOneWorker(t *testing.T) {
	deskTestWorkerBin(t)
	cfg := deskPoolCfg(t)
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	type res struct {
		lease deskSessionLease
		err   error
	}
	out := make(chan res, 2)
	for _, run := range []string{"run-1", "run-2"} {
		go func(run string) {
			lease, err := workers.acquire(context.Background(), "agent:x", true, run, cfg(run))
			if err == nil {
				time.Sleep(50 * time.Millisecond)
				workers.done(lease)
			}
			out <- res{lease, err}
		}(run)
	}
	a, b := <-out, <-out
	if a.err != nil || b.err != nil {
		t.Fatalf("acquire: %v / %v", a.err, b.err)
	}
	if a.lease.worker != b.lease.worker || workers.count() != 1 {
		t.Fatalf("two first cells of one session started %d sessions", workers.count())
	}
}

// MA6.4 r4: a session released (or every session closed) while its worker
// starts lands no session: the start's desks-memory wait is cancelled, a
// worker that started anyway is closed, and the pool leaves deskPools.
// Ways it could fail: the late worker is kept (a session nobody owns,
// holding a claim), the waiting start is not cancelled (closeAll waits for
// a deadline), or the empty pool stays registered.
func TestDeskSessionReleaseCancelsSpawnInFlight(t *testing.T) {
	deskTestWorkerBin(t)
	cfg := deskPoolCfg(t)
	gate := make(chan struct{})
	entered := make(chan struct{}, 1)
	var started []*yaegikernel.DeskSessionWorker
	var mu sync.Mutex
	spawnDeskSessionWorker = func(c yaegikernel.DeskSessionWorkerConfig) (*yaegikernel.DeskSessionWorker, error) {
		entered <- struct{}{}
		<-gate
		w, err := yaegikernel.SpawnDeskSessionWorker(c)
		if w != nil {
			mu.Lock()
			started = append(started, w)
			mu.Unlock()
		}
		return w, err
	}
	t.Cleanup(func() { spawnDeskSessionWorker = yaegikernel.SpawnDeskSessionWorker })
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	out := make(chan error, 1)
	go func() {
		_, err := workers.acquire(context.Background(), "agent:x", true, "run-1", cfg("run-1"))
		out <- err
	}()
	<-entered
	workers.release("agent:x")
	close(gate)
	if err := <-out; err == nil {
		t.Fatal("a start released in flight landed a session")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(started) != 1 || !started[0].Dead() {
		t.Fatalf("the late worker was kept (started %d)", len(started))
	}
	if workers.count() != 0 {
		t.Fatalf("%d sessions after release", workers.count())
	}
	deskPools.mu.Lock()
	_, registered := deskPools.pools[workers]
	deskPools.mu.Unlock()
	if registered {
		t.Fatal("the empty pool stayed in deskPools")
	}
}

func TestDeskSessionCloseAllCancelsWaitingSpawn(t *testing.T) {
	cfg := deskPoolCfg(t)
	entered := make(chan struct{}, 1)
	spawnDeskSessionWorker = func(c yaegikernel.DeskSessionWorkerConfig) (*yaegikernel.DeskSessionWorker, error) {
		entered <- struct{}{}
		// A start waiting for desks memory: it ends only with its context.
		<-c.Context.Done()
		return nil, c.Context.Err()
	}
	t.Cleanup(func() { spawnDeskSessionWorker = yaegikernel.SpawnDeskSessionWorker })
	workers := newDeskSessionWorkers()
	out := make(chan error, 1)
	go func() {
		_, err := workers.acquire(context.Background(), "agent:x", true, "run-1", cfg("run-1"))
		out <- err
	}()
	<-entered
	start := time.Now()
	workers.closeAll()
	select {
	case err := <-out:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("acquire: %v, want the start cancelled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("closeAll left a start waiting")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("the cancel took %s", time.Since(start))
	}
}

// TestDeskPoolLeavesWhenLastSessionEnds: a pool is forgotten when its last
// session ends by release, not only under memory pressure.
func TestDeskPoolLeavesWhenLastSessionEnds(t *testing.T) {
	deskTestWorkerBin(t)
	cfg := deskPoolCfg(t)
	child := newResearchChildDeskWorkers("research_web", "c9")
	t.Cleanup(child.closeAll)
	lease, err := child.acquire(context.Background(), "child:9", false, "run-1", cfg("run-1"))
	if err != nil {
		t.Fatal(err)
	}
	registered := func() bool {
		deskPools.mu.Lock()
		defer deskPools.mu.Unlock()
		_, ok := deskPools.pools[child]
		return ok
	}
	if !registered() {
		t.Fatal("a pool with a session is not registered")
	}
	child.done(lease)
	child.release(lease.key)
	if registered() {
		t.Fatal("the pool stayed registered after its last session ended")
	}
}
