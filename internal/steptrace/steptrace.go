// Package steptrace records which named step of a long operation is running
// right now, with its start time, and the last few that finished. It exists
// so a hung operation can be located from the product API without SSH
// (docs/standing-questions.md question 9): the guest's console log is
// nearly empty, and a wait that never ends writes no log line at all.
//
// A Tracker is cheap, safe for concurrent use, and nil-safe: Begin on a nil
// Tracker returns a no-op, so a caller never has to check whether tracing is
// wired. Steps carry names and scopes only, never payloads or secrets.
package steptrace

import (
	"sort"
	"sync"
	"time"
)

const (
	// recentLimit is how many finished steps a Tracker keeps.
	recentLimit = 64
	// errorLimit bounds the error text kept for a finished step.
	errorLimit = 200
)

// Tracker holds the in-flight steps and a short ring of finished ones.
type Tracker struct {
	mu     sync.Mutex
	next   uint64
	active map[uint64]activeStep
	recent []DoneStep
	now    func() time.Time
}

type activeStep struct {
	scope, name string
	startedAt   time.Time
}

// InFlightStep is a step that has begun and not ended.
type InFlightStep struct {
	Scope     string `json:"scope"`
	Step      string `json:"step"`
	StartedAt string `json:"started_at"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// DoneStep is a step that ended, with how long it took and how it ended.
type DoneStep struct {
	Scope      string `json:"scope"`
	Step       string `json:"step"`
	StartedAt  string `json:"started_at"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Snapshot is the tracker's state at one instant: in-flight steps oldest
// first, finished steps oldest first.
type Snapshot struct {
	InFlight []InFlightStep `json:"in_flight"`
	Recent   []DoneStep     `json:"recent"`
}

// New returns an empty Tracker.
func New() *Tracker {
	return &Tracker{active: map[uint64]activeStep{}, now: time.Now}
}

// Begin starts a step and returns the function that ends it. The returned
// function records the outcome (nil error for success) and is safe to call
// once; further calls do nothing. Begin on a nil Tracker returns a no-op.
func (t *Tracker) Begin(scope, name string) func(error) {
	if t == nil {
		return func(error) {}
	}
	t.mu.Lock()
	t.next++
	id := t.next
	started := t.now()
	t.active[id] = activeStep{scope: scope, name: name, startedAt: started}
	t.mu.Unlock()
	return func(err error) {
		t.mu.Lock()
		defer t.mu.Unlock()
		if _, open := t.active[id]; !open {
			return
		}
		delete(t.active, id)
		done := DoneStep{Scope: scope, Step: name, StartedAt: started.UTC().Format(time.RFC3339Nano), DurationMS: t.now().Sub(started).Milliseconds()}
		if err != nil {
			text := err.Error()
			if len(text) > errorLimit {
				text = text[:errorLimit]
			}
			done.Error = text
		}
		t.recent = append(t.recent, done)
		if len(t.recent) > recentLimit {
			t.recent = append([]DoneStep(nil), t.recent[len(t.recent)-recentLimit:]...)
		}
	}
}

// Snapshot returns the current state. On a nil Tracker both lists are empty.
func (t *Tracker) Snapshot() Snapshot {
	out := Snapshot{InFlight: []InFlightStep{}, Recent: []DoneStep{}}
	if t == nil {
		return out
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	type ordered struct {
		step InFlightStep
		at   time.Time
		id   uint64
	}
	var open []ordered
	for id, step := range t.active {
		open = append(open, ordered{InFlightStep{Scope: step.scope, Step: step.name, StartedAt: step.startedAt.UTC().Format(time.RFC3339Nano), ElapsedMS: now.Sub(step.startedAt).Milliseconds()}, step.startedAt, id})
	}
	sort.Slice(open, func(i, j int) bool {
		if open[i].at.Equal(open[j].at) {
			return open[i].id < open[j].id
		}
		return open[i].at.Before(open[j].at)
	})
	for _, o := range open {
		out.InFlight = append(out.InFlight, o.step)
	}
	out.Recent = append(out.Recent, t.recent...)
	return out
}
