package agentcore

import (
	"context"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Desk memory pressure (MA6.4 r3, r4). A gVisor desk worker is admitted
// only while every worker's claim fits the desks cgroup; a spawn that does
// not fit waits for a reap. An idle session (a persistent desk between
// cells) would hold its claim for its whole retention, so while a spawn
// heads the queue the spawner ends idle sessions, least recently used
// first, one per call, and again whenever a session goes idle (done
// notifies the queue).

// spawnDeskSessionWorker starts a session's worker (a test holds a start
// in flight through it).
var spawnDeskSessionWorker = yaegikernel.SpawnDeskSessionWorker

// deskSpawn is a session's worker start in flight outside the pool mutex.
type deskSpawn struct {
	// done closes when the start ends.
	done chan struct{}
	// cancel ends the start's desks-memory wait; released (under the pool
	// mutex) says release or closeAll ran meanwhile, so the worker it
	// started is closed rather than kept.
	cancel   context.CancelFunc
	released bool
}

// deskPools are the pools that hold at least one session or start: a pool
// joins when it keeps a worker and leaves when its last session ends
// (forgetIfEmptyLocked), so a finished research child's pool is not kept.
var deskPools = struct {
	mu    sync.Mutex
	pools map[*deskSessionWorkers]struct{}
}{pools: map[*deskSessionWorkers]struct{}{}}

// registerDeskPool adds m; the caller holds m.mu (lock order: a pool's
// mutex, then deskPools.mu).
func registerDeskPool(m *deskSessionWorkers) {
	deskPools.mu.Lock()
	deskPools.pools[m] = struct{}{}
	deskPools.mu.Unlock()
}

// forgetIfEmptyLocked removes m from deskPools when it holds no session and
// no start. The caller holds m.mu.
func (m *deskSessionWorkers) forgetIfEmptyLocked() {
	if len(m.sessions) != 0 || len(m.spawning) != 0 {
		return
	}
	deskPools.mu.Lock()
	delete(deskPools.pools, m)
	deskPools.mu.Unlock()
}

func init() {
	yaegikernel.SetDeskMemoryPressureHandler(endIdleDeskSession)
}

// endIdleDeskSession ends the least recently used idle session of any pool
// and reports whether it ended one. A busy session, or one whose worker is
// starting, is never ended. The session is detached under its pool's mutex
// and closed outside it: a close ends the worker (seconds, for a sandboxed
// worker's confirmed kill) and must not hold up that pool's other cells.
func endIdleDeskSession() bool {
	deskPools.mu.Lock()
	pools := make([]*deskSessionWorkers, 0, len(deskPools.pools))
	for m := range deskPools.pools {
		pools = append(pools, m)
	}
	deskPools.mu.Unlock()

	var (
		victim     *deskSessionWorkers
		victimKey  string
		victimUsed time.Time
	)
	for _, m := range pools {
		m.mu.Lock()
		for key, s := range m.sessions {
			if s.busyRun != "" {
				continue
			}
			if victim == nil || s.lastUsed.Before(victimUsed) {
				victim, victimKey, victimUsed = m, key, s.lastUsed
			}
		}
		m.mu.Unlock()
	}
	if victim == nil {
		return false
	}
	victim.mu.Lock()
	s := victim.sessions[victimKey]
	if s == nil || s.busyRun != "" {
		victim.mu.Unlock()
		// It was taken meanwhile: report progress so the caller looks again.
		return true
	}
	delete(victim.sessions, victimKey)
	if len(victim.sessions) == 0 && victim.reaper != nil {
		victim.reaper.Stop()
		victim.reaper = nil
	}
	victim.forgetIfEmptyLocked()
	victim.mu.Unlock()
	s.close()
	return true
}
