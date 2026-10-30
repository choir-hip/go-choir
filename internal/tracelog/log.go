// Package tracelog is the guest-local trace segment log of
// docs/design/trace-segment-log-2026-10-22.md (R7, slice S1): an append-only
// log of trace lines in segment files beside the store, with a group fsync,
// sealing with an index, retention by age and bytes, and a reader.
//
// S1 ships the library with no production caller. It uses only the standard
// library and knows nothing of the store's types: a caller passes a Meta and
// the line's bytes, and gets Records back.
//
// Durability. Append writes a line to the page cache before it returns, so
// every in-process reader sees it at once. Sync (called by a timer once a
// second, and by the caller before an acknowledged tape append) makes
// everything written so far durable. A process crash loses nothing; a power
// loss loses at most the lines since the last Sync, always the newest ones.
//
// Fail-stop, from the host segment writer's five review rounds
// (docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md). After a
// failed fsync nothing past the last successful one is known to be durable,
// and bytes that can still be read are not proof that they are: the kernel
// may have dropped the dirty pages and marked them clean, so a second fsync
// succeeds having written nothing. A writer that carries on and repairs the
// file later has to remember what it does not know, and three rounds each
// found a way for that memory to outlive its lock. So the first failed write
// or fsync poisons the Log for good: the call that failed returns its error,
// every later call returns ErrPoisoned, and no file is written, truncated,
// sealed, renamed or deleted again. The caller sends the line to the tape.
// Recovery belongs to the next process: it holds the directory lock for its
// whole life, so no older writer can act while it recovers, it seals the
// segment it did not write at the end of its last whole record, and it
// writes only to a segment it created. No offset is ever reused, because
// nothing is ever truncated.
package tracelog

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Hooks replace the file calls the log makes, so a test can fail or cut any
// of them. Nil means the real call.
type Hooks struct {
	WriteAt  func(f *os.File, b []byte, off int64) (int, error)
	SyncFile func(f *os.File) error
	SyncDir  func(dir string) error
}

// Options configure a Log. Zero values take the defaults.
type Options struct {
	// Dir is the log directory, the trace-log sibling of the store workspace.
	Dir string
	// ComputerID is written into each segment header; it is informational.
	ComputerID string
	// BootID is the process identity a caller puts in Meta.BootID. It is
	// carried here only for the boundary records' sake and may be empty.
	BootID string

	// A segment is sealed at whichever comes first of these (checked before a
	// write, so a segment may pass a bound by one record).
	SealAge     time.Duration
	SealBytes   int64
	SealRecords int

	// SyncInterval is the group fsync period. Zero means one second; a
	// negative value means no timer (Sync is then explicit only).
	SyncInterval time.Duration

	// MaxAge and MaxBytes are the retention bounds Retain applies; zero means
	// no bound.
	MaxAge   time.Duration
	MaxBytes int64

	// HeadSeq, when set, supplies the head sequence recorded in the
	// boundaries the log writes by itself.
	HeadSeq func() int64

	Now   func() time.Time
	Hooks Hooks
	// Logf receives the log's rare messages (a damaged span found at
	// recovery, once per segment). Nil means the standard logger.
	Logf func(format string, args ...any)
}

// Stats are the log's counters, for the observability block (design S2).
type Stats struct {
	Appends         int64            `json:"appends"`
	Bytes           int64            `json:"bytes"` // record bytes written, headers included
	CountByKind     map[string]int64 `json:"count_by_kind"`
	BytesByKind     map[string]int64 `json:"bytes_by_kind"`
	Syncs           int64            `json:"syncs"`
	SyncErrors      int64            `json:"sync_errors"`
	SyncNanosTotal  int64            `json:"sync_nanos_total"`
	SyncNanosMax    int64            `json:"sync_nanos_max"`
	SegmentsCreated int64            `json:"segments_created"`
	SegmentsSealed  int64            `json:"segments_sealed"`
	// TornTailsCut counts unsealed segments found at Open whose last record
	// was partial. DamagedRecords counts spans of a segment that failed their
	// checks and were skipped, with DamagedBytes their size; the whole records
	// around them are kept (design 9.2 item 4). Quarantined counts files set
	// aside as .bad.
	TornTailsCut   int64 `json:"torn_tails_cut"`
	DamagedRecords int64 `json:"damaged_records"`
	DamagedBytes   int64 `json:"damaged_bytes"`
	Quarantined    int64 `json:"quarantined"`
	TooLarge       int64 `json:"too_large"`

	Poisoned    bool   `json:"poisoned"`
	PoisonError string `json:"poison_error,omitempty"`
	// UnsyncedLinesAtPoison is the number of lines that were written but not
	// covered by a successful fsync when the log was poisoned.
	UnsyncedLinesAtPoison int64 `json:"unsynced_lines_at_poison"`

	OpenEntries int   `json:"open_entries"`
	OpenBytes   int64 `json:"open_bytes"`
	SyncedBytes int64 `json:"synced_bytes"`
	// Dirty is true while the open segment holds bytes no fsync has covered.
	Dirty bool `json:"dirty"`
}

// openSeg is the segment this Log created and writes.
type openSeg struct {
	segno   uint64
	f       *os.File
	path    string
	created time.Time
	off     int64 // end of the last whole record written
	synced  int64 // end of what a successful fsync covered
	// syncedLines is how many lines that fsync covered.
	syncedLines int
	records     int // lines and boundaries
	idx         *segIdx
	refs        int  // fsyncs in flight on f
	closeAfter  bool // close f when refs reaches zero
	sealed      bool
}

// Log is the single writer of a trace-log directory. It is safe for
// concurrent use. See the package comment for the fail-stop rule.
type Log struct {
	opts Options
	dir  string
	lock *os.File // the directory, flocked for the life of the Log

	// mu guards everything below. It is a leaf in the caller's lock order. An
	// fsync never runs under it, except the one that seals a segment.
	mu       sync.Mutex
	syncMu   sync.Mutex // serializes Sync; taken before mu
	cur      *openSeg
	nextSeg  uint64
	pending  string // boundary reason to write at the next segment's start
	closed   bool
	poisoned error
	buf      []byte // the line being appended
	bbuf     []byte // a boundary being written
	stats    Stats

	// failing is set by a failed fsync the moment it returns, before the
	// failing call waits for mu, so a seal that holds mu can see it (and
	// failErr says why).
	failing atomic.Bool
	failErr atomic.Pointer[error]

	retainMu sync.Mutex
	retainMA time.Duration
	retainMB int64

	stop chan struct{}
	done sync.WaitGroup

	readerOnce sync.Once
	reader     *Reader
}

// Open takes the directory lock, recovers any segment a dead writer left
// unsealed, and returns a Log that creates its first segment on the first
// write. It returns ErrLocked when another writer holds the directory.
func Open(opts Options) (*Log, error) {
	if opts.Dir == "" || opts.ComputerID == "" {
		return nil, errors.New("tracelog: Dir and ComputerID are required")
	}
	if opts.SealAge <= 0 {
		opts.SealAge = DefaultSealAge
	}
	if opts.SealBytes <= 0 {
		opts.SealBytes = DefaultSealBytes
	}
	if opts.SealRecords <= 0 {
		opts.SealRecords = DefaultSealRecords
	}
	opts.SealRecords = max(opts.SealRecords, 2)
	if opts.SyncInterval == 0 {
		opts.SyncInterval = DefaultSyncInterval
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	dir := filepath.Clean(opts.Dir)
	if err := requirePlainDir(dir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("tracelog: create %s: %w", dir, err)
	}
	if err := requirePlainDir(dir); err != nil {
		return nil, err
	}
	d, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("tracelog: open %s: %w", dir, err)
	}
	ok, err := tryLockDir(d)
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("tracelog: lock %s: %w", dir, err)
	}
	if !ok {
		d.Close()
		return nil, ErrLocked
	}
	l := &Log{opts: opts, dir: dir, lock: d, retainMA: opts.MaxAge, retainMB: opts.MaxBytes, stop: make(chan struct{})}
	l.stats.CountByKind, l.stats.BytesByKind = map[string]int64{}, map[string]int64{}
	if err := l.recover(); err != nil {
		d.Close() // releases the lock
		return nil, err
	}
	if opts.SyncInterval > 0 {
		l.done.Add(1)
		go l.syncLoop()
	}
	return l, nil
}

// Dir is the log directory.
func (l *Log) Dir() string { return l.dir }

func (l *Log) now() time.Time { return l.opts.Now() }

func (l *Log) logf(format string, args ...any) {
	if l.opts.Logf != nil {
		l.opts.Logf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// SetRetention changes the bounds Retain applies.
func (l *Log) SetRetention(maxAge time.Duration, maxBytes int64) {
	l.retainMu.Lock()
	l.retainMA, l.retainMB = maxAge, maxBytes
	l.retainMu.Unlock()
}

// Poisoned returns the first failure that poisoned the log, or nil.
func (l *Log) Poisoned() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.poisoned
}

// failLocked poisons the log with its first failure and returns err. It is
// called at every error after a file has been, or may have been, changed.
func (l *Log) failLocked(err error) error {
	if l.poisoned == nil {
		l.poisoned = err
		l.stats.Poisoned = true
		l.stats.PoisonError = err.Error()
		if s := l.cur; s != nil {
			l.stats.UnsyncedLinesAtPoison = int64(len(s.idx.ents) - s.syncedLines)
		}
	}
	return err
}

// usableLocked returns ErrClosed or ErrPoisoned when the log may not be used.
func (l *Log) usableLocked() error {
	if l.closed {
		return ErrClosed
	}
	if l.poisoned != nil {
		return fmt.Errorf("%w: first failure: %v", ErrPoisoned, l.poisoned)
	}
	return nil
}

// Stats returns a snapshot of the counters.
func (l *Log) Stats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.stats
	st.CountByKind, st.BytesByKind = copyCounts(l.stats.CountByKind), copyCounts(l.stats.BytesByKind)
	if s := l.cur; s != nil {
		st.OpenEntries, st.OpenBytes, st.SyncedBytes = s.records, s.off, s.synced
		st.Dirty = s.off > s.synced
	}
	return st
}

func copyCounts(m map[string]int64) map[string]int64 {
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Append writes one line. The line is in the page cache when it returns, not
// yet durable (see Sync). An error means the line was not accepted: the caller
// writes it to the tape. ErrTooLarge changes nothing; every other error
// poisons the log.
func (l *Log) Append(m Meta, line []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.usableLocked(); err != nil {
		return err
	}
	rec, err := encodeLine(l.buf, &m, line)
	l.buf = rec
	if err != nil {
		l.stats.TooLarge++
		return err
	}
	if err := l.prepareLocked(); err != nil {
		return err
	}
	s := l.cur
	off := s.off
	if err := l.writeLocked(s, rec); err != nil {
		return err
	}
	s.idx.add(off, len(rec), &m)
	s.records++
	l.stats.Appends++
	l.stats.Bytes += int64(len(rec))
	l.stats.CountByKind[m.Kind]++
	l.stats.BytesByKind[m.Kind] += int64(len(rec))
	return nil
}

// AppendBoundary writes a boundary record (a restore, a setting change).
func (l *Log) AppendBoundary(reason string, headSeq int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.usableLocked(); err != nil {
		return err
	}
	b := Boundary{Reason: reason, HeadSeq: headSeq, At: l.now()}
	if len(reason) > MaxRecordBytes/2 {
		l.stats.TooLarge++
		return ErrTooLarge // refused before a segment is touched
	}
	if err := l.prepareLocked(); err != nil {
		return err
	}
	return l.writeBoundaryLocked(l.cur, b)
}

func (l *Log) writeBoundaryLocked(s *openSeg, b Boundary) error {
	// Its own buffer: Append has a line encoded in l.buf when it creates a
	// segment, and the segment's first boundary must not overwrite it.
	var err error
	if l.bbuf, err = encodeBoundary(l.bbuf, b); err != nil {
		l.stats.TooLarge++
		return err
	}
	if err := l.writeLocked(s, l.bbuf); err != nil {
		return err
	}
	s.idx.nBound++
	s.records++
	return nil
}

// prepareLocked makes sure there is an open segment with room: it seals a
// full or aged one and creates the next. Any failure poisons.
func (l *Log) prepareLocked() error {
	if s := l.cur; s != nil {
		if s.off < l.opts.SealBytes && s.records < l.opts.SealRecords && !(s.records > 0 && l.now().Sub(s.created) >= l.opts.SealAge) {
			return nil
		}
		if err := l.sealCurrentLocked(); err != nil {
			return err
		}
	}
	return l.startSegmentLocked()
}

// startSegmentLocked creates the next segment exclusively, writes and fsyncs
// its header and makes the directory entry durable, then writes the pending
// boundary as its first record.
func (l *Log) startSegmentLocked() error {
	segno := l.nextSeg
	if segno == 0 {
		segno = 1
	}
	path := filepath.Join(l.dir, segmentName(segno))
	f, err := openRegular(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return l.failLocked(fmt.Errorf("tracelog: create segment %d: %w", segno, err))
	}
	now := l.now()
	s := &openSeg{segno: segno, f: f, path: path, created: now, idx: newIdx()}
	hdr := encodeSegHeader(segno, now, l.opts.ComputerID)
	if n, err := l.writeAt(f, hdr, 0); err != nil || n != len(hdr) {
		if err == nil {
			err = fmt.Errorf("short write of %d of %d bytes", n, len(hdr))
		}
		f.Close()
		return l.failLocked(fmt.Errorf("tracelog: write segment header: %w", err))
	}
	if err := l.syncFile(f); err != nil {
		f.Close()
		return l.failLocked(fmt.Errorf("tracelog: fsync segment header: %w", err))
	}
	if err := l.syncDir(l.dir); err != nil {
		f.Close()
		return l.failLocked(fmt.Errorf("tracelog: fsync directory: %w", err))
	}
	s.off, s.synced = int64(len(hdr)), int64(len(hdr))
	l.cur = s
	l.nextSeg = segno + 1
	l.stats.SegmentsCreated++
	if l.pending != "" {
		reason := l.pending
		l.pending = ""
		var head int64
		if l.opts.HeadSeq != nil {
			head = l.opts.HeadSeq()
		}
		if err := l.writeBoundaryLocked(s, Boundary{Reason: reason, HeadSeq: head, At: now}); err != nil {
			return err
		}
	}
	return nil
}

// writeLocked appends one whole record at the segment's write offset. A
// failed or short write poisons the log and leaves the segment as it is.
func (l *Log) writeLocked(s *openSeg, rec []byte) error {
	if l.poisoned != nil {
		return fmt.Errorf("%w: first failure: %v", ErrPoisoned, l.poisoned)
	}
	n, err := l.writeAt(s.f, rec, s.off)
	if err == nil && n != len(rec) {
		err = fmt.Errorf("short write of %d of %d bytes", n, len(rec))
	}
	if err != nil {
		return l.failLocked(fmt.Errorf("tracelog: append record: %w", err))
	}
	s.off += int64(len(rec))
	return nil
}

func (l *Log) writeAt(f *os.File, b []byte, off int64) (int, error) {
	if l.opts.Hooks.WriteAt != nil {
		return l.opts.Hooks.WriteAt(f, b, off)
	}
	return f.WriteAt(b, off)
}

func (l *Log) syncFile(f *os.File) error {
	if l.opts.Hooks.SyncFile != nil {
		return l.opts.Hooks.SyncFile(f)
	}
	return f.Sync()
}

func (l *Log) syncDir(dir string) error {
	if l.opts.Hooks.SyncDir != nil {
		return l.opts.Hooks.SyncDir(dir)
	}
	return realSyncDir(dir)
}

func realSyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Sync makes every line written before the call durable. It returns at once
// when nothing is unsynced. The fsync runs without the append mutex, so
// appends proceed during it; a line written during an fsync is not covered by
// it. A failure poisons the log.
func (l *Log) Sync() error {
	l.syncMu.Lock()
	defer l.syncMu.Unlock()
	l.mu.Lock()
	if err := l.usableLocked(); err != nil {
		l.mu.Unlock()
		return err
	}
	s := l.cur
	if s == nil || s.off <= s.synced {
		l.mu.Unlock()
		return nil
	}
	target, lines := s.off, len(s.idx.ents)
	s.refs++
	l.mu.Unlock()

	start := time.Now()
	err := l.syncFile(s.f)
	took := time.Since(start).Nanoseconds()
	if err != nil {
		// Flag it before waiting for mu: a seal in progress holds mu and must
		// not install an index after an fsync has failed.
		e := fmt.Errorf("tracelog: fsync segment: %w", err)
		l.failErr.CompareAndSwap(nil, &e)
		l.failing.Store(true)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	s.refs--
	if s.refs == 0 && s.closeAfter {
		s.f.Close()
	}
	l.stats.SyncNanosTotal += took
	l.stats.SyncNanosMax = max(l.stats.SyncNanosMax, took)
	if err != nil {
		l.stats.SyncErrors++
		return l.failLocked(fmt.Errorf("tracelog: fsync segment: %w", err))
	}
	// Do not acknowledge on behalf of a log that something else poisoned
	// while this fsync ran.
	if l.poisoned != nil {
		return fmt.Errorf("%w: first failure: %v", ErrPoisoned, l.poisoned)
	}
	l.stats.Syncs++
	if target > s.synced {
		s.synced, s.syncedLines = target, lines
	}
	return nil
}

func (l *Log) syncLoop() {
	defer l.done.Done()
	t := time.NewTicker(l.opts.SyncInterval)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			if err := l.Sync(); err != nil && !errors.Is(err, ErrPoisoned) && !errors.Is(err, ErrClosed) {
				_ = err // Sync has poisoned the log; every caller sees it
			}
		}
	}
}

// MaintainResult says what one Maintain call did.
type MaintainResult struct {
	Sealed bool
	Retain RetainResult
}

// Maintain is the periodic tick: it seals an open segment that has passed its
// age even when no line has arrived, then applies retention.
func (l *Log) Maintain() (MaintainResult, error) {
	var res MaintainResult
	l.mu.Lock()
	if err := l.usableLocked(); err != nil {
		l.mu.Unlock()
		return res, err
	}
	if s := l.cur; s != nil && s.records > 0 && l.now().Sub(s.created) >= l.opts.SealAge {
		if err := l.sealCurrentLocked(); err != nil {
			l.mu.Unlock()
			return res, err
		}
		res.Sealed = true
	}
	l.mu.Unlock()
	r, err := l.Retain()
	res.Retain = r
	return res, err
}

// sealCurrentLocked fsyncs the open segment, writes its index and makes it
// read-only for good. A failure poisons the log.
func (l *Log) sealCurrentLocked() error {
	s := l.cur
	if err := l.syncFile(s.f); err != nil {
		l.stats.SyncErrors++
		return l.failLocked(fmt.Errorf("tracelog: fsync segment %d to seal: %w", s.segno, err))
	}
	sum := s.idx.summarize(s.off, s.created, l.now())
	notFailed := func() error {
		if l.failing.Load() {
			if e := l.failErr.Load(); e != nil {
				return fmt.Errorf("%w: not installing an index after: %v", ErrPoisoned, *e)
			}
			return ErrPoisoned
		}
		return nil
	}
	if err := writeIndex(l.dir, s.segno, s.off, s.off, sum, s.idx, l.syncFile, l.syncDir, notFailed); err != nil {
		if e := l.failErr.Load(); e != nil {
			err = *e // the failure that came first
		}
		return l.failLocked(err)
	}
	s.sealed = true
	s.synced, s.syncedLines = s.off, len(s.idx.ents)
	if s.refs == 0 {
		s.f.Close()
	} else {
		s.closeAfter = true
	}
	l.cur = nil
	l.stats.SegmentsSealed++
	return nil
}

// Close stops the timer, syncs and seals the open segment, and releases the
// directory. A poisoned log only closes its descriptors: it touches no file.
// Close is idempotent.
func (l *Log) Close() error {
	// Held to the end, the directory release included: a Retain in progress
	// finishes first, and a later one finds the log closed.
	l.retainMu.Lock()
	defer l.retainMu.Unlock()
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil
	}
	l.closed = true
	close(l.stop)
	l.mu.Unlock()
	l.done.Wait()

	l.mu.Lock()
	defer l.mu.Unlock()
	var err error
	if l.poisoned == nil && l.cur != nil {
		// Sealed even when nothing was written beyond the header, so the next
		// open does not read a clean stop as a crash.
		err = l.sealCurrentLocked()
	}
	if s := l.cur; s != nil {
		if s.refs == 0 {
			s.f.Close()
		} else {
			s.closeAfter = true
		}
	}
	l.lock.Close() // releases the directory lock
	return err
}
