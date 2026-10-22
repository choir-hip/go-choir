package objectgraph

import (
	"database/sql"
	"testing"
	"time"
)

func TestCallerNameAttribution(t *testing.T) {
	m := newEngineMutex()
	// Direct lock/unlock should attribute to the test function.
	m.Lock()
	time.Sleep(time.Millisecond)
	m.Unlock()
	// Deferred unlock inside a helper should attribute to the helper.
	helper := func() {
		m.Lock()
		defer m.Unlock()
		time.Sleep(time.Millisecond)
	}
	helper()
	st := m.stats
	if _, ok := st["TestCallerNameAttribution"]; !ok {
		t.Fatalf("missing TestCallerNameAttribution stats; got keys: %v", keysOf(st))
	}
	if _, ok := st["func1"]; !ok {
		// closure names render as TestX.func1 in older runtimes; accept any
		// non-Lock/Unlock key as second attribution.
		found := false
		for k := range st {
			if k != "TestCallerNameAttribution" && k != "Lock" && k != "Unlock" {
				found = true
			}
		}
		if !found {
			t.Fatalf("second lock attribution fell through to Lock/Unlock; keys: %v", keysOf(st))
		}
	}
}

func keysOf(m map[string]*EngineMutexOpStats) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

// WithEngine serializes with every other use of the shared engine mutex
// (docs/design/choir-event-table-f-c-2026-10-22.md, Q8): fn does not run
// while another store over the same engine holds the lock, and its op is
// accounted. Round 1b (Sonnet): the race detector does not see the embedded
// engine's internal races, so the lock is pinned here directly.
func TestWithEngineExcludesOtherEngineUse(t *testing.T) {
	db := openTestDoltDB(t)
	writer := NewDoltStore(db)
	reader := NewDoltStore(db)
	reader.ShareEngineMutex(writer)

	writer.engineMu.LockAs("test.holder")
	ran := make(chan struct{})
	go func() {
		_ = reader.WithEngine("test.with_engine", func(*sql.DB) error {
			close(ran)
			return nil
		})
	}()
	select {
	case <-ran:
		t.Fatal("WithEngine ran while another store held the shared engine mutex")
	case <-time.After(200 * time.Millisecond):
	}
	writer.engineMu.UnlockAs("test.holder")
	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatal("WithEngine never ran after the holder released the mutex")
	}
	if writer.EngineMutexStats()["test.with_engine"].Calls != 1 {
		t.Fatalf("WithEngine's op is not accounted on the shared mutex: %v", writer.EngineMutexStats())
	}
}
