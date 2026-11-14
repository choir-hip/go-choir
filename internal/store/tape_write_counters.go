package store

import (
	"bytes"
	"context"
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
	opsByFamily     map[string]int64
	bytesByFamily   map[string]int64
	opsByObjectKind map[string]int64
	opBytesByObject map[string]int64
	runMemoryByKind map[string]int64
	runMemoryBytes  map[string]int64        // body bytes of those entries, same keys
	rewrites        map[string]*rewriteKind // object_rewrite_counters.go
	// lifecycleForms counts committed lifecycle commands by the form of their
	// stored result (lifecycleForm* in lifecycle.go), so the saving of naming
	// a revision instead of embedding it is read on a live computer.
	lifecycleForms map[string]int64
}

func bumpKey(m *map[string]int64, key string, n int64) {
	if *m == nil {
		*m = map[string]int64{}
	}
	// Leave room for "other" so len(*m) never exceeds maxCounterKeys.
	if _, ok := (*m)[key]; !ok && key != "other" && len(*m) >= maxCounterKeys-1 {
		key = "other"
	}
	(*m)[key] += n
}

// Per-event versus per-op attribution: tape_events_by_family and
// tape_bytes_by_family give a whole event, whatever it holds, to the family of
// its first op (continuity with earlier readings), so a batch [document,
// revision, sources...] is all "object:<document kind>". The per-op numbers
// (tape_ops_by_object_kind, tape_op_bytes_by_object_kind, object_rewrites,
// run_memory_*_by_kind) count every op under its own kind and its own body
// bytes and are the ones to read for what a kind costs.

// tapeFamily names the writer family of one writer's ops: the telemetry sealer
// by its authority, otherwise the first op decides ("run_memory", "edge",
// "object:<kind>" or the op kind). One writer's ops are one family by their
// first op. An event that carries several writers (coalescing, riding
// telemetry) is classified per writer in tallyTapeParts; this function must
// never be asked about the flat ops of such an event.
func tapeFamily(authority string, ops []computerevent.ProjectionOp) string {
	if authority == telemetryProjectionAuthority {
		return "telemetry"
	}
	if len(ops) == 0 {
		return "empty"
	}
	return opFamily(ops[0])
}

// familyMixed names an event whose writers are not all of one family.
const familyMixed = "mixed"

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

// tapeEventTally is what one accepted event adds to the counters, worked out
// before any lock is taken.
type tapeEventTally struct {
	family       string
	partFamilies []string // one per writer, in order
	partOps      []int
	payloadBytes int64
	objKinds     []string
	objBytes     []int64
	objHashes    []uint64 // of the canonical ids, parallel to objKinds
	rmKeys       []string
	rmBytes      []int64 // body bytes, parallel to rmKeys
}

// tallyTapeEvent classifies an event written by one writer.
func tallyTapeEvent(authority string, ops []computerevent.ProjectionOp, payloadBytes int) tapeEventTally {
	return tallyTapeParts(authority, [][]computerevent.ProjectionOp{ops}, false, payloadBytes)
}

// tallyTapeParts classifies an event by its writers: parts are the ops of each
// writer in the order they sit in the event, the first being telemetry that
// rode along when firstIsTelemetry. Each writer is counted under its own
// family in ops_by_family; the event is counted once, under that family when
// all writers agree and under "mixed" when they do not, so events_by_family
// still sums to events. It takes no lock and does no JSON decode.
func tallyTapeParts(authority string, parts [][]computerevent.ProjectionOp, firstIsTelemetry bool, payloadBytes int) tapeEventTally {
	t := tapeEventTally{payloadBytes: int64(payloadBytes)}
	for i, part := range parts {
		fam := tapeFamily(authority, part)
		if firstIsTelemetry && i == 0 {
			fam = "telemetry"
		}
		t.partFamilies = append(t.partFamilies, fam)
		t.partOps = append(t.partOps, len(part))
		switch {
		case i == 0:
			t.family = fam
		case t.family != fam:
			t.family = familyMixed
		}
		for _, op := range part {
			switch op.Kind {
			case computerevent.ProjectionOpObject:
				t.objKinds = append(t.objKinds, objectKindOf(op.CanonicalID))
				t.objBytes = append(t.objBytes, int64(len(op.Body)))
				t.objHashes = append(t.objHashes, hashID(op.CanonicalID))
			case computerevent.ProjectionOpRunMemoryEntry:
				t.rmKeys = append(t.rmKeys, runMemoryKey(op.Body))
				t.rmBytes = append(t.rmBytes, int64(len(op.Body)))
			}
		}
	}
	if len(parts) == 0 {
		t.family = "empty"
	}
	return t
}

var (
	rmKindNeedle   = []byte(`"kind":"`)
	rmReasonNeedle = []byte(`"reason":"`)
)

// runMemoryReasons is the closed list of reasons that may follow a run-memory
// kind in a counter key. A reason is free text in the entry (a cell's fate
// carries the error that ended it: on the paper, a reflect panic message), so
// only these, the ones our own code writes, become part of a key; anything
// else counts as "<kind>/other".
var runMemoryReasons = map[string]bool{
	"actor_rewarm":                    true, // message, compaction: a carried actor conversation
	"threshold":                       true, // compaction
	"context_overflow":                true, // compaction
	"tray committed":                  true, // cell_fate
	"tray recovered":                  true, // cell_fate
	"cell terminal deadline exceeded": true, // cell_fate
}

// runMemoryKey is "kind" or "kind/reason" of a run memory entry body, or
// "unknown". The body is a flat object of strings and numbers (see
// RunMemoryEntryProjection), so a quote inside any string value is escaped and
// the unescaped needle `"kind":"` can only be the top-level key: a scan finds
// it without decoding the multi-KB message and details strings. A value with
// an escape, or an absurd length, is "unknown". A reason outside
// runMemoryReasons is "other", so the key space is bounded by the kinds.
func runMemoryKey(body []byte) string {
	kind, ok := flatStringField(body, rmKindNeedle)
	if !ok || kind == "" {
		return "unknown"
	}
	reason, ok := flatStringField(body, rmReasonNeedle)
	if !ok || reason == "" {
		return kind
	}
	if !runMemoryReasons[reason] {
		reason = "other"
	}
	return kind + "/" + reason
}

func flatStringField(body, needle []byte) (string, bool) {
	i := bytes.Index(body, needle)
	if i < 0 {
		return "", false
	}
	rest := body[i+len(needle):]
	end := bytes.IndexByte(rest, '"')
	if end < 0 || end > 64 || bytes.IndexByte(rest[:end], 92) >= 0 {
		return "", false
	}
	return string(rest[:end]), true
}

// add counts one tallied event. The caller holds telemetryCounters.mu.
func (m *tapeWriteMaps) add(t tapeEventTally) {
	bumpKey(&m.eventsByFamily, t.family, 1)
	bumpKey(&m.bytesByFamily, t.family, t.payloadBytes)
	for i, fam := range t.partFamilies {
		bumpKey(&m.opsByFamily, fam, int64(t.partOps[i]))
	}
	for i, kind := range t.objKinds {
		bumpKey(&m.opsByObjectKind, kind, 1)
		bumpKey(&m.opBytesByObject, kind, t.objBytes[i])
		var h uint64
		hasHash := i < len(t.objHashes)
		if hasHash {
			h = t.objHashes[i]
		}
		m.addRewrite(kind, t.objBytes[i], h, hasHash)
	}
	for i, key := range t.rmKeys {
		bumpKey(&m.runMemoryByKind, key, 1)
		if i < len(t.rmBytes) {
			bumpKey(&m.runMemoryBytes, key, t.rmBytes[i])
		}
	}
}

// copyInto fills the wire fields of out. The caller holds the mutex.
func (m *tapeWriteMaps) copyInto(out *TelemetryStats) {
	out.TapeEventsByFamily = copyCounts(m.eventsByFamily)
	out.TapeBytesByFamily = copyCounts(m.bytesByFamily)
	out.TapeOpsByFamily = copyCounts(m.opsByFamily)
	out.TapeOpsByObjectKind = copyCounts(m.opsByObjectKind)
	out.TapeOpBytesByObjectKind = copyCounts(m.opBytesByObject)
	out.RunMemoryOpsByKind = copyCounts(m.runMemoryByKind)
	out.RunMemoryBytesByKind = copyCounts(m.runMemoryBytes)
	out.ObjectRewrites = m.rewriteStats()
	out.LifecycleCommandForms = copyCounts(m.lifecycleForms)
}

// countLifecycleCommandForm counts one committed lifecycle command by the
// form of its stored result.
func (c *telemetryCounters) countLifecycleCommandForm(form string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	bumpKey(&c.tapeWrites.lifecycleForms, form, 1)
	c.mu.Unlock()
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
	// A reading can go backwards when the corpusd call source is replaced:
	// clamp, so the line never shows a negative.
	events := max(0, cur.Events-prev.Events)
	calls := max(0, cur.Calls-prev.Calls)
	callsPerEvent := 0.0
	if events > 0 {
		callsPerEvent = float64(calls) / float64(events)
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
		window, events, max(0, cur.Ops-prev.Ops), max(0, cur.Bytes-prev.Bytes), callsPerEvent)
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
