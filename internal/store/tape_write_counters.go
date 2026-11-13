package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Tape write counters (storage solutions 2, station S5; the census of
// docs/evidence/tape-write-census-2026-11-12.md, section 2.4). They answer
// "what is the tape written with, and what does each event cost" so that a
// change to the write path (batching run memory, coalescing queued writers) is
// measured against a before. They are always on, read-only and never gate
// behaviour. They are served on /api/runtime/observability inside
// telemetry_batching and summarised in the log once an hour.

// tapeSummaryInterval is how often the one-line summary is logged.
const tapeSummaryInterval = time.Hour

// maxCounterKeys bounds each counter map so a stray key cannot grow it
// without limit; keys past the bound count under "other".
const maxCounterKeys = 64

// tapeWriteMaps is the keyed half of the counters. It is guarded by
// telemetryCounters.mu.
type tapeWriteMaps struct {
	eventsByFamily  map[string]int64
	bytesByFamily   map[string]int64
	opsByObjectKind map[string]int64
	opBytesByObject map[string]int64
	runMemoryByKind map[string]int64
}

func bumpKey(m *map[string]int64, key string, n int64) {
	if *m == nil {
		*m = map[string]int64{}
	}
	if _, ok := (*m)[key]; !ok && len(*m) >= maxCounterKeys {
		key = "other"
	}
	(*m)[key] += n
}

// tapeFamily names the writer family of an event: the telemetry sealer by its
// authority, otherwise the first op decides ("run_memory", "edge",
// "object:<kind>" or the op kind).
func tapeFamily(authority string, ops []computerevent.ProjectionOp) string {
	if authority == telemetryProjectionAuthority {
		return "telemetry"
	}
	if len(ops) == 0 {
		return "empty"
	}
	return opFamily(ops[0])
}

func opFamily(op computerevent.ProjectionOp) string {
	switch op.Kind {
	case computerevent.ProjectionOpRunMemoryEntry:
		return "run_memory"
	case computerevent.ProjectionOpObjectEdge:
		return "edge"
	case computerevent.ProjectionOpObject:
		return "object:" + objectKindOf(op.CanonicalID)
	}
	return op.Kind
}

func objectKindOf(canonicalID string) string {
	kind, _, _, err := objectgraph.ParseCanonicalID(canonicalID)
	if err != nil || kind == "" {
		return "unknown"
	}
	return string(kind)
}

// record counts one accepted event. The caller holds telemetryCounters.mu.
func (m *tapeWriteMaps) record(authority string, ops []computerevent.ProjectionOp, payloadBytes int) {
	family := tapeFamily(authority, ops)
	bumpKey(&m.eventsByFamily, family, 1)
	bumpKey(&m.bytesByFamily, family, int64(payloadBytes))
	for _, op := range ops {
		switch op.Kind {
		case computerevent.ProjectionOpObject:
			kind := objectKindOf(op.CanonicalID)
			bumpKey(&m.opsByObjectKind, kind, 1)
			bumpKey(&m.opBytesByObject, kind, int64(len(op.Body)))
		case computerevent.ProjectionOpRunMemoryEntry:
			var head struct {
				Kind   string `json:"kind"`
				Reason string `json:"reason"`
			}
			key := "unknown"
			if json.Unmarshal(op.Body, &head) == nil && head.Kind != "" {
				key = head.Kind
				if head.Reason != "" {
					key += "/" + head.Reason
				}
			}
			bumpKey(&m.runMemoryByKind, key, 1)
		}
	}
}

// copyInto fills the wire fields of out. The caller holds the mutex.
func (m *tapeWriteMaps) copyInto(out *TelemetryStats) {
	out.TapeEventsByFamily = copyCounts(m.eventsByFamily)
	out.TapeBytesByFamily = copyCounts(m.bytesByFamily)
	out.TapeOpsByObjectKind = copyCounts(m.opsByObjectKind)
	out.TapeOpBytesByObjectKind = copyCounts(m.opBytesByObject)
	out.RunMemoryOpsByKind = copyCounts(m.runMemoryByKind)
}

// enterTapeWrite counts a caller entering a tape write and returns the func
// that counts it leaving. It reads the depth the caller found on arrival.
func (c *telemetryCounters) enterTapeWrite() func() {
	if c == nil {
		return func() {}
	}
	started := time.Now()
	depth := int(c.appenderInflight.Add(1))
	c.appenderDepthHist[opsBucket(depth)].Add(1)
	return func() {
		c.appenderInflight.Add(-1)
		c.appenderCalls.Add(1)
		c.appenderCallNanos.Add(time.Since(started).Nanoseconds())
	}
}

func totalCount(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}

// SetCorpusdCallSource tells the store where to read the guest's corpusd
// request counts (computerevent.HTTPClient.CallCounts). Unset, the counts are
// empty. Observability only. The counts cover every request the client sends
// (head reads by the file sync included), so calls per tape event is an upper
// bound on the append path's own calls.
func (s *Store) SetCorpusdCallSource(source func() map[string]int64) {
	if s == nil {
		return
	}
	if source == nil {
		s.corpusdSource.Store(nil)
		return
	}
	s.corpusdSource.Store(&source)
}

func (s *Store) corpusdCalls() map[string]int64 {
	if p := s.corpusdSource.Load(); p != nil {
		if m := (*p)(); m != nil {
			return m
		}
	}
	return map[string]int64{}
}

// tapeWriteTotals is the part of the counters the hourly summary differences.
type tapeWriteTotals struct {
	Events, Ops, Bytes, Calls int64
	ByFamily                  map[string]int64
}

func (s *Store) tapeWriteTotals() tapeWriteTotals {
	st := s.TelemetryStats()
	return tapeWriteTotals{
		Events: st.TapeEvents, Ops: st.TapeOps, Bytes: st.TapeBytes,
		Calls: totalCount(st.CorpusdCalls), ByFamily: st.TapeEventsByFamily,
	}
}

// formatTapeWriteSummary is the one log line for the window between two
// readings: events, ops, bytes, corpusd calls per event, then events by
// family, largest first.
func formatTapeWriteSummary(window time.Duration, prev, cur tapeWriteTotals) string {
	events := cur.Events - prev.Events
	callsPerEvent := 0.0
	if events > 0 {
		callsPerEvent = float64(cur.Calls-prev.Calls) / float64(events)
	}
	type fam struct {
		name string
		n    int64
	}
	var fams []fam
	for name, n := range cur.ByFamily {
		if d := n - prev.ByFamily[name]; d > 0 {
			fams = append(fams, fam{name, d})
		}
	}
	sort.Slice(fams, func(i, j int) bool {
		if fams[i].n != fams[j].n {
			return fams[i].n > fams[j].n
		}
		return fams[i].name < fams[j].name
	})
	var b strings.Builder
	fmt.Fprintf(&b, "tape write summary: window=%s events=%d ops=%d bytes=%d calls_per_event=%.2f by_family:",
		window, events, cur.Ops-prev.Ops, cur.Bytes-prev.Bytes, callsPerEvent)
	for i, f := range fams {
		if i >= 12 {
			break
		}
		fmt.Fprintf(&b, " %s=%d", f.name, f.n)
	}
	return b.String()
}

// runTapeWriteSummary logs the summary every interval until ctx ends.
func (s *Store) runTapeWriteSummary(ctx context.Context, interval time.Duration, logf func(string, ...any)) {
	prev := s.tapeWriteTotals()
	last := time.Now()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			cur := s.tapeWriteTotals()
			logf("%s", formatTapeWriteSummary(now.Sub(last).Round(time.Millisecond), prev, cur))
			prev, last = cur, now
		}
	}
}

// tapeSummaryLoop owns the summary goroutine: started when the projection
// tape is bound, stopped by Close.
type tapeSummaryLoop struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func (s *Store) startTapeWriteSummary() {
	l := &s.tapeSummary
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	l.cancel, l.done = cancel, done
	go func() {
		defer close(done)
		s.runTapeWriteSummary(ctx, tapeSummaryInterval, log.Printf)
	}()
}

func (s *Store) stopTapeWriteSummary() {
	l := &s.tapeSummary
	l.mu.Lock()
	cancel, done := l.cancel, l.done
	l.cancel, l.done = nil, nil
	l.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}
