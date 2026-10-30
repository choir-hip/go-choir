package store

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ListRunMemoryEntriesForAgent is the paper's top store_stalled holder
// (docs/archive/problems/run-memory-entries-hold-the-engine-2026-10-23.md). Its
// callers are three sites in agentcore; the stall line cannot tell them
// apart and records no duration. This file is the observation: a call
// counter, and one log line for a call over the slow threshold, at most once
// a minute (as the stall snapshots are), naming the site the caller put on
// the context, the rows, and where the time went.
const (
	runMemorySlowAfter = 250 * time.Millisecond
	runMemorySlowEvery = time.Minute
)

type runMemorySiteKey struct{}

// WithRunMemoryReadSite names the call site of a run-memory read on ctx, for
// the slow-read log line. An empty site is "unlabelled".
func WithRunMemoryReadSite(ctx context.Context, site string) context.Context {
	return context.WithValue(ctx, runMemorySiteKey{}, site)
}

func runMemoryReadSite(ctx context.Context) string {
	if site, _ := ctx.Value(runMemorySiteKey{}).(string); site != "" {
		return site
	}
	return "unlabelled"
}

// runMemoryAgentListObserver counts calls and rate-limits the slow log.
type runMemoryAgentListObserver struct {
	calls atomic.Int64
	mu    sync.Mutex
	last  time.Time
	// suppressed counts slow calls not logged since the last logged one.
	suppressed int
	// slowAfter and logf are test seams; zero values mean the defaults.
	slowAfter time.Duration
	logf      func(format string, args ...any)
}

// RunMemoryAgentListCalls is the number of ListRunMemoryEntriesForAgent calls
// since the store opened.
func (s *Store) RunMemoryAgentListCalls() int64 { return s.runMemoryAgentList.calls.Load() }

// runMemoryAgentListRead is what one call measured.
type runMemoryAgentListRead struct {
	site       string
	agentID    string
	kinds      int
	rows       int
	connWait   time.Duration // waiting for the write pool's one connection
	firstRow   time.Duration // from the connection to the first row (plan, fetch, sort)
	iterate    time.Duration // first row to the end (scan and decode)
	elapsed    time.Duration // whole call
	callsSoFar int64
}

// observe logs a slow call. now is the call's end.
func (o *runMemoryAgentListObserver) observe(now time.Time, r runMemoryAgentListRead) {
	slow := o.slowAfter
	if slow == 0 {
		slow = runMemorySlowAfter
	}
	if r.elapsed <= slow {
		return
	}
	o.mu.Lock()
	if !o.last.IsZero() && now.Sub(o.last) < runMemorySlowEvery {
		o.suppressed++
		o.mu.Unlock()
		return
	}
	o.last = now
	suppressed := o.suppressed
	o.suppressed = 0
	o.mu.Unlock()
	logf := o.logf
	if logf == nil {
		logf = log.Printf
	}
	logf("store: run_memory_agent_list slow site=%s agent=%s kinds=%d rows_returned=%d conn_wait_ms=%d first_row_ms=%d iterate_ms=%d elapsed_ms=%d calls_since_open=%d slow_suppressed=%d",
		r.site, r.agentID, r.kinds, r.rows, r.connWait.Milliseconds(), r.firstRow.Milliseconds(), r.iterate.Milliseconds(),
		r.elapsed.Milliseconds(), r.callsSoFar, suppressed)
}
