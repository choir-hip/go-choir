package objectgraph

import (
	"runtime"
	"strings"
	"sync"
	"time"
)

// engineMutex measures acquisition wait and hold time on the embedded Dolt
// engine lock. It replaces *sync.Mutex as DoltStore.engineMu with an
// identical Lock/Unlock API so call sites do not change; every wait and hold
// is attributed to the caller's function name (one per code path, low
// cardinality on this small surface).
type engineMutex struct {
	mu      sync.Mutex
	stats   map[string]*EngineMutexOpStats
	statsMu sync.Mutex
	// acquiredAt records the lock acquisition instant for the current holder.
	// Safe without extra locking: mu serializes holders, and Unlock always runs
	// in the goroutine that holds mu.
	acquiredAt time.Time
}

// EngineMutexOpStats is one call path's cumulative counters. Read via
// EngineMutexStats; fields are updated under statsMu while a copy is taken.
type EngineMutexOpStats struct {
	Calls        uint64 `json:"calls"`
	WaitNanos    uint64 `json:"wait_ns"`
	HoldNanos    uint64 `json:"hold_ns"`
	MaxWaitNanos uint64 `json:"max_wait_ns"`
	MaxHoldNanos uint64 `json:"max_hold_ns"`
}

func newEngineMutex() *engineMutex {
	return &engineMutex{stats: make(map[string]*EngineMutexOpStats)}
}

// callerName returns the function name of the method that acquired the lock
// (skipping Lock itself and the runtime frame).
func callerName() string {
	var pcs [4]uintptr
	// skip runtime.Callers + callerName + Lock|Unlock → caller's function.
	n := runtime.Callers(3, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		name := frame.Function
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if i := strings.Index(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if name != "" && !strings.HasPrefix(name, "runtime.") {
			return name
		}
		if !more {
			return "unknown"
		}
	}
}

func (m *engineMutex) Lock() {
	op := callerName()
	start := time.Now()
	m.mu.Lock()
	wait := time.Since(start)
	m.acquiredAt = time.Now()
	m.record(op, wait, true)
}

func (m *engineMutex) Unlock() {
	hold := time.Since(m.acquiredAt)
	op := callerName()
	m.record(op, hold, false)
	m.mu.Unlock()
}

func (m *engineMutex) record(op string, d time.Duration, isWait bool) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	st := m.stats[op]
	if st == nil {
		st = &EngineMutexOpStats{}
		m.stats[op] = st
	}
	if isWait {
		st.Calls++
		st.WaitNanos += uint64(d)
		if uint64(d) > st.MaxWaitNanos {
			st.MaxWaitNanos = uint64(d)
		}
	} else {
		st.HoldNanos += uint64(d)
		if uint64(d) > st.MaxHoldNanos {
			st.MaxHoldNanos = uint64(d)
		}
	}
}

// EngineMutexStats returns cumulative per-call-path counters for the
// embedded-engine lock: total wait and hold time plus their maximums. Used by
// the guest /health surface to distinguish lock queueing from lock-held work
// during the latency mission's drain probe.
func (s *DoltStore) EngineMutexStats() map[string]EngineMutexOpStats {
	if s == nil || s.engineMu == nil {
		return nil
	}
	s.engineMu.statsMu.Lock()
	defer s.engineMu.statsMu.Unlock()
	out := make(map[string]EngineMutexOpStats, len(s.engineMu.stats))
	for op, st := range s.engineMu.stats {
		out[op] = *st
	}
	return out
}
