package store

// The trace segment log, store side, slice S2 of R7
// (docs/design/trace-segment-log-2026-10-22.md): readers and seeds, inert.
//
// The log is a directory of segment files beside the store workspace, never
// inside it (restore quarantines and swaps the workspace). The store opens it
// when its directory exists, closes it in Close, reopens it in Reopen and
// writes the restore boundary there. Every event-list method adds a log half
// and merges it with what it reads today; the seeds consider the log. Nothing
// writes a trace line yet: the writer, the barrier and the setting are S4's,
// so with the log empty every answer is the one origin/main gives.
//
// The log is trace, not state. A log that will not open, or a segment that
// cannot be read, costs trace lines and counts an error; it never fails a
// store open, an append or a read.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tracelog"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// traceLogDirName is the log's directory, the store marker's sibling.
const traceLogDirName = "trace-log"

// traceLogComputerID is the ComputerID written into segment headers. The
// store does not know its computer at Open (the projection tape binds it
// later); the header field is informational, and S4's segment digest object
// carries the real one.
const traceLogComputerID = "unbound"

// traceLogDirFor is the log directory of the store whose marker is dbPath:
// beside the Dolt workspace (deriveTextureWorkspacePath), not in it, and a
// function of the store's own path alone, so a fork or a capsule store has its
// own (F19, F20).
func traceLogDirFor(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), traceLogDirName)
}

// testTraceLogHooks replace the log's file calls (tests only).
var testTraceLogHooks tracelog.Hooks

// traceLogBootID names this process in the records it writes.
var traceLogBootID = sync.OnceValue(func() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
})

// traceLogHandle is an open log: the writer, which holds the directory, and
// the reader that sees its open segment.
type traceLogHandle struct {
	log    *tracelog.Log
	reader *tracelog.Reader
	dir    string
}

// traceLogState is the store's hold on the log. The handle is swapped
// atomically, so a list method takes it with one load and never sees a torn
// one; mu serializes the opens and closes and guards the last open's outcome.
type traceLogState struct {
	handle atomic.Pointer[traceLogHandle]

	mu      sync.Mutex
	openErr string
	locked  bool

	readErrors    atomic.Int64
	barrierFsyncs atomic.Int64
	fallbacks     atomic.Int64
	dropped       atomic.Int64

	latMu sync.Mutex
	lat   []int64 // the last traceLogLatencyWindow barrier fsyncs, nanoseconds
	latAt int
}

const traceLogLatencyWindow = 1024

func (s *Store) traceLogHandle() *traceLogHandle { return s.traceLog.handle.Load() }

// openTraceLogIfPresent opens the log when its directory exists: a previous
// process wrote there, and its unsealed segment is recovered. A store that
// never had one creates no file (a scratch store packed into a projection base
// carries nothing; an install into an empty store directory still finds it
// empty). Open calls it last; it cannot fail the open.
func (s *Store) openTraceLogIfPresent() {
	dir := traceLogDirFor(s.path)
	if _, err := os.Lstat(dir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.traceLog.mu.Lock()
			s.traceLog.openErr = err.Error()
			s.traceLog.mu.Unlock()
		}
		return
	}
	if err := s.ensureTraceLog(); err != nil {
		log.Printf("store: trace log: not opened, the store runs without it: %v", err)
	}
}

// ensureTraceLog opens the log, creating its directory, if it is not open. It
// is idempotent. A directory another writer holds (ErrLocked) leaves the store
// without a log: it neither writes nor reads the other writer's lines.
func (s *Store) ensureTraceLog() error {
	st := &s.traceLog
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.handle.Load() != nil {
		return nil
	}
	dir := traceLogDirFor(s.path)
	l, err := tracelog.Open(tracelog.Options{
		Dir: dir, ComputerID: traceLogComputerID, BootID: traceLogBootID(), Hooks: testTraceLogHooks,
	})
	if err != nil {
		st.locked = errors.Is(err, tracelog.ErrLocked)
		st.openErr = err.Error()
		return err
	}
	st.locked, st.openErr = false, ""
	st.handle.Store(&traceLogHandle{log: l, reader: l.Reader(), dir: dir})
	return nil
}

// closeTraceLog fsyncs, seals and releases the log. A failure is logged and
// counted by the log itself; it never fails Close.
func (s *Store) closeTraceLog() {
	st := &s.traceLog
	st.mu.Lock()
	defer st.mu.Unlock()
	h := st.handle.Swap(nil)
	st.locked, st.openErr = false, ""
	if h == nil {
		return
	}
	if err := h.log.Close(); err != nil {
		log.Printf("store: trace log: close: %v", err)
	}
}

// adoptTraceLog moves the log a Reopen's scratch Store opened onto s. The
// scratch Store is zeroed right after, so the handle must not stay with it.
func (s *Store) adoptTraceLog(from *Store) {
	st := &s.traceLog
	st.mu.Lock()
	defer st.mu.Unlock()
	from.traceLog.mu.Lock()
	h := from.traceLog.handle.Swap(nil)
	openErr, locked := from.traceLog.openErr, from.traceLog.locked
	from.traceLog.mu.Unlock()
	st.handle.Store(h)
	st.openErr, st.locked = openErr, locked
}

// writeTraceLogRestoreBoundary records that the workspace was swapped under
// the log (design 7.3). It creates the log for a computer that never had one,
// so a restore always leaves its boundary. Its failure is logged and nothing
// else: a boundary is annotation.
func (s *Store) writeTraceLogRestoreBoundary() {
	if err := s.ensureTraceLog(); err != nil {
		log.Printf("store: trace log: no restore boundary: %v", err)
		return
	}
	h := s.traceLogHandle()
	if h == nil {
		return
	}
	var head int64
	if tape := s.projectionTape; tape != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if hd, err := s.Head(ctx, tape.computerID); err == nil && hd != nil {
			head = int64(hd.Sequence)
		}
		cancel()
	}
	if err := h.log.AppendBoundary(tracelog.ReasonRestore, head); err != nil {
		log.Printf("store: trace log: restore boundary: %v", err)
	}
}

// --- readers ----------------------------------------------------------------

// noteTraceLogReadError counts a failed or partial log read. The page is
// served from the other half and the rest of the log.
func (s *Store) noteTraceLogReadError(op string, err error) {
	if err == nil {
		return
	}
	if s.traceLog.readErrors.Add(1) == 1 {
		log.Printf("store: trace log: %s: %v (further read errors are counted in observability)", op, err)
	}
}

// traceLogEntries decodes the records a reader returned into union entries.
// err is the reader's: records it found are kept with it.
func (s *Store) traceLogEntries(op string, recs []tracelog.Record, err error) []eventTableEntry {
	s.noteTraceLogReadError(op, err)
	if len(recs) == 0 {
		return nil
	}
	out := make([]eventTableEntry, 0, len(recs))
	for _, r := range recs {
		var rec types.EventRecord
		if jerr := json.Unmarshal(r.Line, &rec); jerr != nil {
			s.noteTraceLogReadError(op+": decode", jerr)
			continue
		}
		out = append(out, eventTableEntry{canonicalID: "trace_log:" + r.Meta.EventID, updatedAt: rec.Timestamp.UTC(), rec: rec})
	}
	return out
}

// The four log halves of the union reads (design 6.2). Each fetches at least
// the caller's limit. A store with no open log has an empty half.

func (s *Store) traceLogHalfRun(runID string, afterSeq int64, limit int) []eventTableEntry {
	h := s.traceLogHandle()
	if h == nil {
		return nil
	}
	recs, err := h.reader.ByRunAnyOwner(runID, afterSeq, limit)
	return s.traceLogEntries("run", recs, err)
}

func (s *Store) traceLogHalfOwnerAfter(ownerID string, afterStreamSeq int64, limit int) []eventTableEntry {
	h := s.traceLogHandle()
	if h == nil {
		return nil
	}
	recs, err := h.reader.ByOwnerAfter(ownerID, afterStreamSeq, limit)
	return s.traceLogEntries("owner_after", recs, err)
}

func (s *Store) traceLogHalfOwnerNewest(ownerID string, limit int) []eventTableEntry {
	h := s.traceLogHandle()
	if h == nil {
		return nil
	}
	recs, err := h.reader.ByOwnerNewest(ownerID, limit)
	return s.traceLogEntries("owner_newest", recs, err)
}

func (s *Store) traceLogHalfTrajectory(ownerID, trajectoryID string, limit int) []eventTableEntry {
	h := s.traceLogHandle()
	if h == nil {
		return nil
	}
	recs, err := h.reader.ByTrajectoryNewest(ownerID, trajectoryID, limit)
	return s.traceLogEntries("trajectory", recs, err)
}

// mergeTraceLogHalf adds the log half to the Dolt union, deduplicated by
// event_id before any cut. A record in both (a rollback window, design 8) is
// served once and the Dolt copy is kept. An event with no id is never taken
// for a duplicate. With an empty half it returns base itself, unchanged.
func mergeTraceLogHalf(base, half []eventTableEntry) []eventTableEntry {
	if len(half) == 0 {
		return base
	}
	seen := make(map[string]struct{}, len(base)+len(half))
	for _, e := range base {
		if e.rec.EventID != "" {
			seen[e.rec.EventID] = struct{}{}
		}
	}
	out := make([]eventTableEntry, 0, len(base)+len(half))
	out = append(out, base...)
	for _, e := range half {
		if id := e.rec.EventID; id != "" {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
		}
		out = append(out, e)
	}
	return out
}

// --- seeds ------------------------------------------------------------------

// traceLogRunMaxSeq is the highest seq the log holds for the run, or zero. The
// per-run seed takes the maximum of every source (design 4.4).
func (s *Store) traceLogRunMaxSeq(ownerID, runID string) int64 {
	h := s.traceLogHandle()
	if h == nil {
		return 0
	}
	n, err := h.reader.MaxRunSeq(ownerID, runID)
	s.noteTraceLogReadError("seed_run", err)
	return n
}

// traceLogMaxStreamSeq is the highest stream_seq in the log, or zero. The
// stream_seq lease's cross-check and its exact seed take it (the lease
// supersedes the design's 512 gap: it already covers every number published).
func (s *Store) traceLogMaxStreamSeq() int64 {
	h := s.traceLogHandle()
	if h == nil {
		return 0
	}
	n, err := h.reader.MaxStreamSeq()
	s.noteTraceLogReadError("seed_stream", err)
	return n
}

// --- observability ----------------------------------------------------------

// TraceLogFloor is the oldest line the log still holds, so a reader can tell
// "older than retention" from "missing".
type TraceLogFloor struct {
	Segment   uint64    `json:"segment"`
	Time      time.Time `json:"time"`
	StreamSeq int64     `json:"stream_seq"`
}

// TraceLogStats is the trace_log block of /api/runtime/observability (design
// S2). Counters of the log itself restart with the process (and with a
// Reopen); Fallbacks, Dropped, ReadErrors and the barrier counters are the
// store's and survive it. Nothing writes the log in S2, so the write counters
// read zero; they are the proof's instruments for S4.
type TraceLogStats struct {
	// Open: this process holds the log directory. Locked: another writer does.
	Open      bool   `json:"open"`
	Locked    bool   `json:"locked,omitempty"`
	OpenError string `json:"open_error,omitempty"`
	Dir       string `json:"dir"`

	Records       int64            `json:"records"`
	Bytes         int64            `json:"bytes"`
	RecordsByKind map[string]int64 `json:"records_by_kind"`
	BytesByKind   map[string]int64 `json:"bytes_by_kind"`

	Fsyncs      int64 `json:"fsyncs"`
	FsyncErrors int64 `json:"fsync_errors"`
	// BarrierFsyncs counts the fsyncs the ordering barrier made before an
	// acknowledged tape append, and BarrierFsyncP99MS is their p99 over the
	// last 1,024 (the S4 gate is 5 ms).
	BarrierFsyncs     int64   `json:"barrier_fsyncs"`
	BarrierFsyncP99MS float64 `json:"barrier_fsync_p99_ms"`

	Segments       int            `json:"segments"`
	SealedSegments int            `json:"sealed_segments"`
	RetainedBytes  int64          `json:"retained_bytes"`
	RetentionFloor *TraceLogFloor `json:"retention_floor"`

	// Dropped counts lines refused at disk-critical; Fallbacks lines sent to
	// the tape because the log could not take them.
	Dropped   int64 `json:"dropped"`
	Fallbacks int64 `json:"fallbacks"`

	Poisoned    bool   `json:"poisoned"`
	PoisonError string `json:"poison_error,omitempty"`
	// ReadErrors counts failed or partial log reads and undecodable lines.
	ReadErrors     int64 `json:"read_errors"`
	DamagedRecords int64 `json:"damaged_records"`
	TooLarge       int64 `json:"too_large"`
}

// recordTraceLogBarrier counts one ordering-barrier fsync and keeps its
// latency for the p99.
func (s *Store) recordTraceLogBarrier(d time.Duration) {
	st := &s.traceLog
	st.barrierFsyncs.Add(1)
	st.latMu.Lock()
	if len(st.lat) < traceLogLatencyWindow {
		st.lat = append(st.lat, d.Nanoseconds())
	} else {
		st.lat[st.latAt] = d.Nanoseconds()
		st.latAt = (st.latAt + 1) % traceLogLatencyWindow
	}
	st.latMu.Unlock()
}

func (st *traceLogState) barrierP99MS() float64 {
	st.latMu.Lock()
	lat := append([]int64(nil), st.lat...)
	st.latMu.Unlock()
	if len(lat) == 0 {
		return 0
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	rank := (len(lat)*99 + 99) / 100 // nearest rank: ceil(0.99 n)
	return float64(lat[rank-1]) / float64(time.Millisecond)
}

// TraceLogStats returns a snapshot. It reads the log's counters and its
// directory listing and never takes a number or waits on the store.
func (s *Store) TraceLogStats() TraceLogStats {
	st := &s.traceLog
	out := TraceLogStats{
		Dir: traceLogDirFor(s.path), RecordsByKind: map[string]int64{}, BytesByKind: map[string]int64{},
		BarrierFsyncs: st.barrierFsyncs.Load(), BarrierFsyncP99MS: st.barrierP99MS(),
		Dropped: st.dropped.Load(), Fallbacks: st.fallbacks.Load(), ReadErrors: st.readErrors.Load(),
	}
	st.mu.Lock()
	out.Locked, out.OpenError = st.locked, st.openErr
	st.mu.Unlock()
	h := st.handle.Load()
	if h == nil {
		return out
	}
	out.Open = true
	ls := h.log.Stats()
	out.Records, out.Bytes = ls.Appends, ls.Bytes
	out.RecordsByKind, out.BytesByKind = ls.CountByKind, ls.BytesByKind
	out.Fsyncs, out.FsyncErrors = ls.Syncs, ls.SyncErrors
	out.Poisoned, out.PoisonError = ls.Poisoned, ls.PoisonError
	out.DamagedRecords, out.TooLarge = ls.DamagedRecords, ls.TooLarge
	// A segment this binary cannot read is left out of the list and reported
	// in the error; the rest is still counted.
	// (Not counted as a read error: a stats read must not move its own counter.)
	segs, _ := h.reader.Segments()
	for _, seg := range segs {
		out.Segments++
		if seg.Sealed {
			out.SealedSegments++
		}
		out.RetainedBytes += seg.Bytes + seg.IndexBytes
	}
	if floor, _ := h.reader.Floor(); !floor.Empty && floor.Segment != 0 {
		out.RetentionFloor = &TraceLogFloor{Segment: floor.Segment, Time: floor.Time, StreamSeq: floor.StreamSeq}
	}
	return out
}
