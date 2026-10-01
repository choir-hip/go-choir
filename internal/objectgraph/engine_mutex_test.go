package objectgraph

import (
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
