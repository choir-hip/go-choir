package platform

import (
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Retrieval timing (rung 0 of docs/archive/reports/object-graph-retrieval-research-2026-10-14.md;
// docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md):
// every Store B retrieval call is timed and counted. Counts and latency
// buckets are on corpusd /health under "retrieval"; each call at the route
// level also writes one structured journal line, and primary-key and edge
// reads write one only when slow.

// Retrieval operation names. They are journal and /health keys: keep them
// stable.
const (
	retrievalListObjects      = "list_objects"
	retrievalGetObject        = "get_object"
	retrievalGetByMetadata    = "get_by_metadata"
	retrievalListByMetadata   = "list_by_metadata"
	retrievalListEdges        = "list_edges"
	retrievalListEdgesFrom    = "list_edges_from"
	retrievalListEdgesByKind  = "list_edges_by_kind"
	retrievalListEdgesTo      = "list_edges_to"
	retrievalGetEdge          = "get_edge"
	retrievalFrontPublication = "front_publications"
	retrievalCaptureSearch    = "capture_search"
	retrievalCaptureByURL     = "capture_by_url"
	retrievalCaptureIndexSync = "capture_index_backfill"
	// retrievalCaptureIndexWrite is a write, timed beside the reads: the
	// projection's cost per capture.
	retrievalCaptureIndexWrite = "capture_index_write"
)

// retrievalLogSlowOnly names operations that log only when slower than
// retrievalSlowLog: they run many times per page and are primary-key or
// single-index seeks.
var retrievalLogSlowOnly = map[string]bool{
	retrievalGetObject:         true,
	retrievalGetEdge:           true,
	retrievalListEdgesFrom:     true,
	retrievalListEdgesByKind:   true,
	retrievalListEdgesTo:       true,
	retrievalCaptureIndexWrite: true,
}

const retrievalSlowLog = 100 * time.Millisecond

// retrievalBucketsMS are the upper bounds of the latency buckets; the last
// bucket counts everything slower.
var retrievalBucketsMS = []float64{1, 5, 10, 50, 100, 500, 1000, 5000}

type retrievalStats struct {
	Calls       int64
	Errors      int64
	Rows        int64
	TotalMicros int64
	MaxMicros   int64
	// Buckets has len(retrievalBucketsMS)+1 entries.
	Buckets []int64
	// Shapes counts the query shape each call used (for example
	// "windowed" vs "unbounded", "lookup_key" vs "json_scan").
	Shapes map[string]int64
}

// RetrievalMetrics counts and times retrieval calls. The zero value is
// ready; a nil receiver records nothing.
type RetrievalMetrics struct {
	mu    sync.Mutex
	since time.Time
	ops   map[string]*retrievalStats
	// logf is the journal sink (log.Printf unless a test replaces it).
	logf func(format string, args ...any)
}

// RetrievalOpSnapshot is one operation's counters as /health reports them.
type RetrievalOpSnapshot struct {
	Calls  int64            `json:"calls"`
	Errors int64            `json:"errors"`
	Rows   int64            `json:"rows"`
	MeanMS float64          `json:"mean_ms"`
	MaxMS  float64          `json:"max_ms"`
	P50MS  float64          `json:"p50_le_ms"`
	P95MS  float64          `json:"p95_le_ms"`
	Shapes map[string]int64 `json:"shapes,omitempty"`
	// Buckets maps each bucket's upper bound ("le_5ms", ..., "gt_5000ms")
	// to its count.
	Buckets map[string]int64 `json:"buckets"`
}

// RetrievalSnapshot is the "retrieval" block of corpusd /health.
type RetrievalSnapshot struct {
	Since string                         `json:"since"`
	Ops   map[string]RetrievalOpSnapshot `json:"ops"`
	// Live names the retrieval indexes and projections in use.
	Live map[string]bool `json:"live,omitempty"`
}

func (m *RetrievalMetrics) record(op, shape string, elapsed time.Duration, rows int, err error) {
	if m == nil {
		return
	}
	micros := elapsed.Microseconds()
	ms := float64(micros) / 1000
	m.mu.Lock()
	if m.ops == nil {
		m.ops = map[string]*retrievalStats{}
		if m.since.IsZero() {
			m.since = time.Now().UTC()
		}
	}
	st := m.ops[op]
	if st == nil {
		st = &retrievalStats{Buckets: make([]int64, len(retrievalBucketsMS)+1), Shapes: map[string]int64{}}
		m.ops[op] = st
	}
	st.Calls++
	if err != nil {
		st.Errors++
	}
	st.Rows += int64(rows)
	st.TotalMicros += micros
	if micros > st.MaxMicros {
		st.MaxMicros = micros
	}
	idx := len(retrievalBucketsMS)
	for i, bound := range retrievalBucketsMS {
		if ms <= bound {
			idx = i
			break
		}
	}
	st.Buckets[idx]++
	if shape != "" {
		st.Shapes[shape]++
	}
	logf := m.logf
	m.mu.Unlock()

	if retrievalLogSlowOnly[op] && elapsed < retrievalSlowLog && err == nil {
		return
	}
	if logf == nil {
		logf = log.Printf
	}
	errText := ""
	if err != nil {
		errText = strings.ReplaceAll(err.Error(), " ", "_")
		if len(errText) > 160 {
			errText = errText[:160]
		}
	}
	logf("corpusd retrieval op=%s shape=%s ms=%.2f rows=%d err=%s", op, shapeOrDash(shape), ms, rows, errText)
}

func shapeOrDash(shape string) string {
	if shape == "" {
		return "-"
	}
	return shape
}

// Snapshot returns the counters for /health.
func (m *RetrievalMetrics) Snapshot() RetrievalSnapshot {
	out := RetrievalSnapshot{Ops: map[string]RetrievalOpSnapshot{}}
	if m == nil {
		return out
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.since.IsZero() {
		out.Since = m.since.Format(time.RFC3339)
	}
	for op, st := range m.ops {
		snap := RetrievalOpSnapshot{Calls: st.Calls, Errors: st.Errors, Rows: st.Rows,
			MaxMS: float64(st.MaxMicros) / 1000, Buckets: map[string]int64{}}
		if st.Calls > 0 {
			snap.MeanMS = float64(st.TotalMicros) / 1000 / float64(st.Calls)
		}
		snap.P50MS = bucketQuantile(st.Buckets, st.Calls, 0.50, snap.MaxMS)
		snap.P95MS = bucketQuantile(st.Buckets, st.Calls, 0.95, snap.MaxMS)
		for i, n := range st.Buckets {
			snap.Buckets[bucketName(i)] = n
		}
		if len(st.Shapes) > 0 {
			snap.Shapes = map[string]int64{}
			for k, v := range st.Shapes {
				snap.Shapes[k] = v
			}
		}
		out.Ops[op] = snap
	}
	return out
}

// bucketQuantile is the upper bound of the bucket holding quantile q (the
// observed maximum for the open last bucket).
func bucketQuantile(buckets []int64, calls int64, q, maxMS float64) float64 {
	if calls == 0 {
		return 0
	}
	want := int64(float64(calls)*q + 0.5)
	if want < 1 {
		want = 1
	}
	var seen int64
	for i, n := range buckets {
		seen += n
		if seen >= want {
			if i < len(retrievalBucketsMS) {
				return retrievalBucketsMS[i]
			}
			return maxMS
		}
	}
	return maxMS
}

func bucketName(i int) string {
	if i < len(retrievalBucketsMS) {
		return "le_" + trimFloat(retrievalBucketsMS[i]) + "ms"
	}
	return "gt_" + trimFloat(retrievalBucketsMS[len(retrievalBucketsMS)-1]) + "ms"
}

func trimFloat(f float64) string {
	s := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(f, 'f', 3, 64), "0"), ".")
	if s == "" {
		return "0"
	}
	return s
}

// retrievalOpNames lists the operations seen, sorted (tests and tools).
func (m *RetrievalMetrics) retrievalOpNames() []string {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.ops))
	for op := range m.ops {
		names = append(names, op)
	}
	sort.Strings(names)
	return names
}

// retrievalTimer starts timing one retrieval call; the returned func
// records it.
func (s *Store) retrievalTimer(op string) func(shape string, rows int, err error) {
	start := time.Now()
	return func(shape string, rows int, err error) {
		if s == nil {
			return
		}
		s.retrieval.record(op, shape, time.Since(start), rows, err)
	}
}

// RetrievalSnapshot returns the store's retrieval counters.
func (s *Store) RetrievalSnapshot() RetrievalSnapshot {
	if s == nil {
		return RetrievalSnapshot{Ops: map[string]RetrievalOpSnapshot{}}
	}
	snap := s.retrieval.Snapshot()
	snap.Live = s.RetrievalLive()
	return snap
}
