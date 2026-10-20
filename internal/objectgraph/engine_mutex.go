package objectgraph

import (
	"runtime"
	"sort"
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

	// Live view (guarded by statsMu): who holds the lock now, since when, and
	// who is waiting. EngineMutexStats records a wait only after it ends, so a
	// wait that never ends is invisible there; this is what a hang shows.
	holderOp    string
	holderSince time.Time
	nextWaiter  uint64
	waiters     map[uint64]engineWaiter
}

type engineWaiter struct {
	op    string
	since time.Time
}

// EngineMutexLive is the lock's state at one instant.
type EngineMutexLive struct {
	HolderOp      string              `json:"holder_op,omitempty"`
	HeldMS        int64               `json:"held_ms,omitempty"`
	Waiters       []EngineMutexWaiter `json:"waiters"`
	WaiterCount   int                 `json:"waiter_count"`
	OldestWaitMS  int64               `json:"oldest_wait_ms"`
	ObservedAtUTC string              `json:"observed_at"`
}

// EngineMutexWaiter is one goroutine waiting for the engine lock.
type EngineMutexWaiter struct {
	Op     string `json:"op"`
	WaitMS int64  `json:"wait_ms"`
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
	id := m.beginWait(op, start)
	m.mu.Lock()
	wait := time.Since(start)
	m.acquiredAt = time.Now()
	m.endWait(id, op, m.acquiredAt)
	m.record(op, wait, true)
}

func (m *engineMutex) Unlock() {
	hold := time.Since(m.acquiredAt)
	op := callerName()
	m.record(op, hold, false)
	m.statsMu.Lock()
	m.holderOp, m.holderSince = "", time.Time{}
	m.statsMu.Unlock()
	m.mu.Unlock()
}

func (m *engineMutex) beginWait(op string, since time.Time) uint64 {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	if m.waiters == nil {
		m.waiters = map[uint64]engineWaiter{}
	}
	m.nextWaiter++
	m.waiters[m.nextWaiter] = engineWaiter{op: op, since: since}
	return m.nextWaiter
}

func (m *engineMutex) endWait(id uint64, op string, acquired time.Time) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	delete(m.waiters, id)
	m.holderOp, m.holderSince = op, acquired
}

// live returns the current holder and the waiters, oldest wait first.
func (m *engineMutex) live(now time.Time) EngineMutexLive {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	out := EngineMutexLive{Waiters: []EngineMutexWaiter{}, ObservedAtUTC: now.UTC().Format(time.RFC3339Nano)}
	if m.holderOp != "" {
		out.HolderOp, out.HeldMS = m.holderOp, now.Sub(m.holderSince).Milliseconds()
	}
	type aged struct {
		op    string
		since time.Time
	}
	var waiting []aged
	for _, w := range m.waiters {
		waiting = append(waiting, aged{w.op, w.since})
	}
	sort.Slice(waiting, func(i, j int) bool { return waiting[i].since.Before(waiting[j].since) })
	for _, w := range waiting {
		out.Waiters = append(out.Waiters, EngineMutexWaiter{Op: w.op, WaitMS: now.Sub(w.since).Milliseconds()})
	}
	out.WaiterCount = len(waiting)
	if len(waiting) > 0 {
		out.OldestWaitMS = out.Waiters[0].WaitMS
	}
	return out
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

// EngineMutexLive reports who holds the embedded-engine lock now and who is
// waiting for it, with ages. Unlike EngineMutexStats it shows a wait that has
// not ended, which is what a hung store call looks like.
func (s *DoltStore) EngineMutexLive() *EngineMutexLive {
	if s == nil || s.engineMu == nil {
		return nil
	}
	live := s.engineMu.live(time.Now())
	return &live
}
