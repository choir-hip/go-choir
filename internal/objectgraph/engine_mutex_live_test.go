package objectgraph

import (
	"testing"
	"time"
)

// A wait that has not ended is the shape of a hung store call; the cumulative
// stats cannot show it (a wait is recorded when it ends). The live view names
// the holder and every waiter with its age.
func TestEngineMutexLiveShowsHolderAndWaitersBeforeTheyEnd(t *testing.T) {
	m := newEngineMutex()
	if live := m.live(time.Now()); live.HolderOp != "" || live.WaiterCount != 0 || len(live.Waiters) != 0 {
		t.Fatalf("idle lock = %+v", live)
	}

	holderCall := func() { m.Lock() }
	holderCall()

	acquired := make(chan struct{})
	release := make(chan struct{})
	waiterCall := func() {
		m.Lock()
		close(acquired)
		<-release
		m.Unlock()
	}
	go waiterCall()

	deadline := time.Now().Add(5 * time.Second)
	var live EngineMutexLive
	for time.Now().Before(deadline) {
		live = m.live(time.Now())
		if live.WaiterCount == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if live.HolderOp == "" || live.WaiterCount != 1 || len(live.Waiters) != 1 || live.Waiters[0].Op == "" {
		t.Fatalf("held lock with one waiter = %+v", live)
	}
	time.Sleep(20 * time.Millisecond)
	later := m.live(time.Now())
	if later.OldestWaitMS < 20 || later.HeldMS < 20 {
		t.Fatalf("ages do not grow: %+v", later)
	}

	m.Unlock()
	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter never acquired the lock")
	}
	handed := m.live(time.Now())
	if handed.WaiterCount != 0 || handed.HolderOp == "" {
		t.Fatalf("after handoff = %+v", handed)
	}
	close(release)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m.live(time.Now()).HolderOp == "" {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("lock still shown held after release: %+v", m.live(time.Now()))
}
