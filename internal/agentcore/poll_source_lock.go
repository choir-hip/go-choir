package agentcore

import (
	"encoding/json"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// pollSourceLocks serializes choir.PollSource per source within a run (MA6.6
// review r1, D3): code mode serves a cell's calls in parallel, and two polls
// of one source would race on its cursor. Entries are reference counted and
// removed when the last holder unlocks, so the map holds only polls in flight.
type pollSourceLocks struct {
	mu    sync.Mutex
	locks map[string]*pollSourceLock
}

type pollSourceLock struct {
	mu   sync.Mutex
	refs int
}

// lockPollSource takes the lock of the source the payload names within the
// run and returns its unlock. A payload that names no source locks the run's
// empty key, so it still serializes with itself.
func (rt *Runtime) lockPollSource(runID string, payload json.RawMessage) func() {
	var p yaegikernel.PollSourcePayload
	_ = json.Unmarshal(payload, &p)
	key := strings.TrimSpace(runID) + "\x00" + strings.TrimSpace(p.SourceID)
	l := &rt.pollSources
	l.mu.Lock()
	if l.locks == nil {
		l.locks = map[string]*pollSourceLock{}
	}
	e := l.locks[key]
	if e == nil {
		e = &pollSourceLock{}
		l.locks[key] = e
	}
	e.refs++
	l.mu.Unlock()
	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		l.mu.Lock()
		if e.refs--; e.refs == 0 {
			delete(l.locks, key)
		}
		l.mu.Unlock()
	}
}
