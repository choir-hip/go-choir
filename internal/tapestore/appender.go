package tapestore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// ErrAppenderPoisoned is returned by every call of an Appender after its
// first failed fsync or failed write (see Appender). The call that failed
// returns its own error; every later call returns this one, wrapping the
// first failure's text, and touches nothing.
var ErrAppenderPoisoned = errors.New("tapestore: appender poisoned by an earlier write or fsync failure")

// Item is one record to append: a namespace and the exact bytes whose
// SHA-256 is the digest.
type Item struct {
	NS   NS
	Body []byte
}

// AppenderStats are the writer's lifetime counters. FilesCreated for the
// forward-fix E2E assertion (section 4.5) is SegmentsCreated+IndexesCreated.
type AppenderStats struct {
	Appends         uint64 `json:"appends"`          // Append calls that returned nil
	Records         uint64 `json:"records"`          // records newly written (duplicates excluded)
	Bytes           uint64 `json:"bytes"`            // record bytes written, headers included
	Syncs           uint64 `json:"syncs"`            // record fsyncs: one per Append call that wrote
	SegmentsCreated uint64 `json:"segments_created"` // seg-N.seg files created
	IndexesCreated  uint64 `json:"indexes_created"`  // seg-N.idx files created
	Seals           uint64 `json:"seals"`            // segments sealed
	TornTails       uint64 `json:"torn_tails"`       // torn tails cut while sealing a segment found on disk
	Errors          uint64 `json:"errors"`           // Append calls that failed after validation
	Poisoned        bool   `json:"poisoned"`         // the fail-stop rule has fired
}

// Appender is the production segment writer (appendix A.5). It is safe for
// concurrent use. Writes to one computer are serialized by a mutex and an
// advisory lock on the computer's segment directory; different computers do
// not contend.
//
// One Append writes the records it is given, in order, each with one write,
// then fsyncs the segment once, and returns only after that fsync. The
// records of one call land in one segment: the rotation limits are checked
// before a call, never inside it, so a segment may exceed a limit by one
// call's records.
//
// Fail-stop. After a failed fsync nothing past the last successful fsync is
// durable, and bytes that can still be read are not proof that they are: the
// kernel may have dropped the dirty pages and marked them clean, so a second
// fsync succeeds having written nothing (the PostgreSQL rule since 2018). A
// writer that goes on and repairs the file later has to remember what it
// does not know, and three review rounds each found a way for that memory to
// outlive the lock (docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md).
// So the first failed fsync, or failed write, poisons the Appender for good:
// the call that failed returns its error, and every later call, for any
// computer, returns ErrAppenderPoisoned without touching a file. The failed
// call's segment is left exactly as it is; nothing is truncated, sealed or
// retried. The caller stores that pin some other way (the legacy file path).
//
// Recovery belongs to the next writer, which may be another Appender in
// another process. A writer appends only to a segment it created itself. The
// first time it writes for a computer it finds the highest segment; if that
// is unsealed it did not write it (or wrote it and forgot), so it reads it
// back, keeps only the whole, verified records (a torn tail is cut off, which
// no reader can have seen as a record), fsyncs, seals it with an index of
// those records and fsyncs the directory, then starts a fresh segment of its
// own. A sealed segment is never modified. An offset is never reused, so a
// reader's cached location can only ever name the record that was there. The
// bytes of a poisoned writer's unacknowledged tail may therefore be sealed
// into the store (whole and verified, never acknowledged): a reader may serve
// them, and the pin that failed has stored them the legacy way, but they are
// not durable until the next writer's fsync, which is as good as the kernel
// makes it (see the residual in the goal file).
//
// The per-computer state (open segment number, size, key set) is a cache of
// the segment this Appender created. Every call revalidates it against the
// disk (the segment is still unsealed and has the cached size); a mismatch
// means another writer sealed it, and the call recovers as above. Recovery
// refuses anything but a torn tail with ErrCorrupt, changing nothing on disk
// when it refuses. A refusal does not poison: nothing was written.
type Appender struct {
	root        string
	sealBytes   int64
	sealRecords int
	rd          *Reader // directory listing only

	mu   sync.Mutex
	logs map[string]*computerLog

	poisonMu sync.Mutex
	poisonBy error
	poisoned atomic.Bool

	// Fault-injection seams for tests; nil means the real call.
	writeAt  func(f *os.File, b []byte, off int64) (int, error)
	syncFile func(f *os.File) error
	syncDir  func(path string) error

	appends, records, bytes, syncs, segsCreated, idxCreated, seals, torn, errs atomic.Uint64
}

type computerLog struct {
	mu      sync.Mutex
	valid   bool
	segno   uint64
	size    int64
	entries map[openKey]idxEntry
}

// fail poisons the Appender with its first failure and returns err. It is
// called at every error after a file has been, or may have been, changed.
func (a *Appender) fail(err error) error {
	a.poisonMu.Lock()
	if a.poisonBy == nil {
		a.poisonBy = err
		a.poisoned.Store(true)
	}
	a.poisonMu.Unlock()
	return err
}

// check returns ErrAppenderPoisoned once the fail-stop rule has fired.
func (a *Appender) check() error {
	if !a.poisoned.Load() {
		return nil
	}
	a.poisonMu.Lock()
	defer a.poisonMu.Unlock()
	return fmt.Errorf("%w: first failure: %v", ErrAppenderPoisoned, a.poisonBy)
}

// NewAppender returns a writer over the artifacts root with the production
// rotation thresholds (64 MiB or 50,000 records). It performs no I/O.
func NewAppender(artifactsRoot string) *Appender {
	return newAppender(artifactsRoot, DefaultSealBytes, DefaultSealRecords)
}

func newAppender(artifactsRoot string, sealBytes int64, sealRecords int) *Appender {
	root := filepath.Clean(artifactsRoot)
	return &Appender{root: root, sealBytes: sealBytes, sealRecords: sealRecords, rd: NewReader(root), logs: map[string]*computerLog{}}
}

// Root is the artifacts root the writer was opened on.
func (a *Appender) Root() string { return a.root }

// Stats returns the counters.
func (a *Appender) Stats() AppenderStats {
	return AppenderStats{
		Appends: a.appends.Load(), Records: a.records.Load(), Bytes: a.bytes.Load(), Syncs: a.syncs.Load(),
		SegmentsCreated: a.segsCreated.Load(), IndexesCreated: a.idxCreated.Load(), Seals: a.seals.Load(),
		TornTails: a.torn.Load(), Errors: a.errs.Load(), Poisoned: a.poisoned.Load(),
	}
}

func (a *Appender) logFor(id string) *computerLog {
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.logs[id]
	if l == nil {
		l = &computerLog{}
		a.logs[id] = l
	}
	return l
}

// Append writes the items to the computer's open segment and returns after
// one fsync covering them. An item already in the open segment is skipped.
// An error means none of the call's records is acknowledged.
func (a *Appender) Append(computerID string, items ...Item) error {
	if len(items) == 0 {
		return errors.New("tapestore: nothing to append")
	}
	return a.appendFunc(computerID, items, nil)
}

// AppendFunc is Append for a caller that must decide what to write by
// reading the store. build runs with the computer's write lock held (the
// mutex and the directory lock), after the writer's own segment has been
// established and before anything is written, so what it reads of the open
// segment is stable and complete: no other writer is mid-batch, and the only
// unsealed segment is this writer's, holding only records that a finished call
// fsynced. A poisoned Appender never runs build. build returns the items to
// append; none is allowed (the call then writes nothing). An error from build
// aborts the call with nothing written and does not poison. build must not
// call the Appender.
func (a *Appender) AppendFunc(computerID string, build func() ([]Item, error)) error {
	if build == nil {
		return errors.New("tapestore: AppendFunc needs a build function")
	}
	return a.appendFunc(computerID, nil, build)
}

func checkItems(items []Item) error {
	for _, it := range items {
		if !it.NS.valid() {
			return fmt.Errorf("tapestore: invalid namespace %d", uint8(it.NS))
		}
		if len(it.Body) > MaxRecordBytes {
			return fmt.Errorf("tapestore: record of %d bytes exceeds the %d byte limit", len(it.Body), MaxRecordBytes)
		}
	}
	return nil
}

func (a *Appender) appendFunc(computerID string, items []Item, build func() ([]Item, error)) error {
	if !ValidComputerID(computerID) {
		return fmt.Errorf("tapestore: invalid computer id %q", computerID)
	}
	if err := checkItems(items); err != nil {
		return err
	}
	l := a.logFor(computerID)
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := a.appendLocked(l, computerID, items, build); err != nil {
		a.errs.Add(1)
		return err
	}
	a.appends.Add(1)
	return nil
}

func (a *Appender) appendLocked(l *computerLog, id string, items []Item, build func() ([]Item, error)) error {
	if err := a.check(); err != nil {
		return err
	}
	dir := filepath.Join(a.root, SegDirName, id)
	for _, p := range []string{filepath.Join(a.root, SegDirName), dir} {
		if err := requirePlainDir(p); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("tapestore: create %s: %w", dir, err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("tapestore: open %s: %w", dir, err)
	}
	defer d.Close()
	unlock, err := lockDir(d)
	if err != nil {
		return fmt.Errorf("tapestore: lock %s: %w", dir, err)
	}
	defer unlock()
	// Another computer's call may have poisoned the writer while this one
	// waited for the lock.
	if err := a.check(); err != nil {
		return err
	}

	if !a.cacheCurrent(l, dir) {
		l.valid = false
		if err := a.begin(l, dir, id, d); err != nil {
			return err
		}
	}
	if build != nil {
		built, err := build()
		if err != nil {
			return err
		}
		if err := checkItems(built); err != nil {
			return err
		}
		items = built
	}
	// Rotation: a full segment of ours is sealed before this call writes.
	if l.size >= a.sealBytes || len(l.entries) >= a.sealRecords {
		l.valid = false
		if err := a.seal(dir, id, l.segno, l.size, l.entries); err != nil {
			return a.fail(err)
		}
		if err := a.startSegment(l, dir, id, l.segno+1, d); err != nil {
			return a.fail(err)
		}
	}

	f, err := openRegular(filepath.Join(dir, SegmentFileName(l.segno)), os.O_RDWR, 0)
	if err != nil {
		l.valid = false
		return fmt.Errorf("tapestore: open segment %d: %w", l.segno, err)
	}
	defer f.Close()

	// From the first write on, any failure leaves the segment in a state this
	// writer cannot vouch for: it poisons the Appender and nothing is undone.
	off := l.size
	pending := map[openKey]pendingRec{}
	for _, it := range items {
		digest := digestBytes(it.Body)
		key := openKey{it.NS, digest}
		if _, dup := l.entries[key]; dup {
			continue
		}
		if _, dup := pending[key]; dup {
			continue
		}
		rec := encodeRecord(it.NS, digest, it.Body)
		var n int
		var werr error
		if a.writeAt != nil {
			n, werr = a.writeAt(f, rec, off)
		} else {
			n, werr = f.WriteAt(rec, off)
		}
		if werr == nil && n != len(rec) {
			werr = io.ErrShortWrite
		}
		if werr != nil {
			return a.fail(fmt.Errorf("tapestore: append record: %w", werr))
		}
		pending[key] = pendingRec{off: off, rec: rec}
		off += int64(len(rec))
	}
	if len(pending) == 0 {
		return nil // every record is one this writer already fsynced
	}
	if serr := a.syncF(f); serr != nil {
		return a.fail(fmt.Errorf("tapestore: fsync segment: %w", serr))
	}
	// Do not acknowledge on behalf of a writer another computer's call poisoned.
	if err := a.check(); err != nil {
		return err
	}
	a.syncs.Add(1)
	start := l.size
	for k, r := range pending {
		l.entries[k] = idxEntry{digest: k.digest, ns: k.ns, off: uint64(r.off), length: uint32(len(r.rec) - recHeaderSize)}
	}
	l.size = off
	a.records.Add(uint64(len(pending)))
	a.bytes.Add(uint64(off - start))
	return nil
}

// pendingRec is a whole record of the call in progress: its offset and bytes.
type pendingRec struct {
	off int64
	rec []byte
}

// syncF fsyncs a segment file (or the test hook standing in for it).
func (a *Appender) syncF(f *os.File) error {
	if a.syncFile != nil {
		return a.syncFile(f)
	}
	return f.Sync()
}

// cacheCurrent reports whether the cached state still describes the disk:
// the segment has not been sealed (by another writer) and has exactly the
// cached size.
func (a *Appender) cacheCurrent(l *computerLog, dir string) bool {
	if !l.valid {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, IndexFileName(l.segno))); err == nil || !errors.Is(err, os.ErrNotExist) {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, SegmentFileName(l.segno)))
	return err == nil && info.Size() == l.size
}

// begin establishes the segment this Appender writes to for the computer: it
// seals the highest segment if that is unsealed (it did not write it, or
// wrote it before a restart; see Appender) and starts the next one. Refusals
// (ErrCorrupt, unreadable files) happen before anything on disk changes and
// do not poison.
func (a *Appender) begin(l *computerLog, dir, id string, d *os.File) error {
	_, segs, err := a.rd.list(id)
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		return a.startOwn(l, dir, id, 1, d)
	}
	for _, s := range segs[:len(segs)-1] {
		if !s.hasIdx {
			return corruptf("computer %s: segment %d is unsealed below the highest segment %d", id, s.segno, segs[len(segs)-1].segno)
		}
	}
	last := segs[len(segs)-1]
	if last.hasIdx {
		if _, err := openSealed(dir, id, last.segno); err != nil {
			return err
		}
	} else if err := a.sealUnowned(dir, id, last.segno); err != nil {
		return err
	}
	return a.startOwn(l, dir, id, last.segno+1, d)
}

func (a *Appender) startOwn(l *computerLog, dir, id string, segno uint64, d *os.File) error {
	if err := a.startSegment(l, dir, id, segno, d); err != nil {
		return a.fail(err)
	}
	return nil
}

// sealUnowned reads an unsealed segment this writer did not write, keeps its
// whole verified records, cuts a torn tail, fsyncs and seals it. It reads
// and decides first, and refuses damage with ErrCorrupt before changing
// anything; after that every failure poisons the Appender.
func (a *Appender) sealUnowned(dir, id string, segno uint64) error {
	path := filepath.Join(dir, SegmentFileName(segno))
	f, err := openRegular(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("tapestore: open segment %d: %w", segno, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	label := fmt.Sprintf("computer %s segment %d", id, segno)
	header := encodeSegHeader(segno, id)
	entries := map[openKey]idxEntry{}
	end := int64(segHeaderSize) // end of the whole, verified records
	torn := false
	if size < segHeaderSize {
		// Creation was torn before the header landed: the file must be a
		// prefix of the header this segment would have, else it is damage.
		got := make([]byte, size)
		if err := readFull(f, got, 0); err != nil {
			return fmt.Errorf("tapestore: read %s: %w", label, err)
		}
		if string(got) != string(header[:size]) {
			return corruptf("%s: %d bytes that are not a prefix of its header", label, size)
		}
		torn = true
	} else {
		head := make([]byte, segHeaderSize)
		if err := readFull(f, head, 0); err != nil {
			return fmt.Errorf("tapestore: read header of %s: %w", label, err)
		}
		if err := checkSegHeader(head, segno, id); err != nil {
			return err
		}
		for end < size {
			h, next, bad, err := scanRecord(f, end, size)
			if err != nil {
				return err
			}
			if bad != "" {
				if size-end > recHeaderSize+MaxRecordBytes {
					return corruptf("%s: %d unreadable bytes at %d exceed any single record: %s", label, size-end, end, bad)
				}
				later, err := validRecordAfter(f, end, size)
				if err != nil {
					return err
				}
				if later {
					return corruptf("%s: bad record at %d is followed by valid records: %s", label, end, bad)
				}
				torn = true
				break
			}
			body := make([]byte, h.length)
			if err := readFull(f, body, end+recHeaderSize); err != nil {
				return fmt.Errorf("tapestore: read %s: %w", label, err)
			}
			if sha256.Sum256(body) != h.digest {
				return corruptf("%s: record at %d does not hash to its digest", label, end)
			}
			k := openKey{h.ns, h.digest}
			if _, dup := entries[k]; !dup {
				entries[k] = idxEntry{digest: h.digest, ns: h.ns, off: uint64(end), length: h.length}
			}
			end = next
		}
	}

	// Decided: seal what is whole. From here a failure poisons.
	if size < segHeaderSize {
		if err := f.Truncate(0); err != nil {
			return a.fail(fmt.Errorf("tapestore: truncate %s: %w", label, err))
		}
		if _, err := f.WriteAt(header, 0); err != nil {
			return a.fail(fmt.Errorf("tapestore: rewrite header of %s: %w", label, err))
		}
	} else if torn {
		if err := f.Truncate(end); err != nil {
			return a.fail(fmt.Errorf("tapestore: truncate torn tail of %s: %w", label, err))
		}
	}
	if err := a.syncF(f); err != nil {
		return a.fail(fmt.Errorf("tapestore: fsync %s: %w", label, err))
	}
	if err := sealIndex(dir, segno, id, end, entries, a.syncDirPath); err != nil {
		return a.fail(err)
	}
	if torn {
		a.torn.Add(1)
	}
	a.idxCreated.Add(1)
	a.seals.Add(1)
	return nil
}

// startSegment creates segment segno exclusively, writes and fsyncs its
// header and makes its directory entries durable.
func (a *Appender) startSegment(l *computerLog, dir, id string, segno uint64, d *os.File) error {
	f, err := openRegular(filepath.Join(dir, SegmentFileName(segno)), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("tapestore: create segment %d: %w", segno, err)
	}
	defer f.Close()
	a.segsCreated.Add(1)
	if _, err := f.WriteAt(encodeSegHeader(segno, id), 0); err != nil {
		return fmt.Errorf("tapestore: write segment header: %w", err)
	}
	if err := a.syncF(f); err != nil {
		return fmt.Errorf("tapestore: fsync segment header: %w", err)
	}
	if err := a.syncTree(dir); err != nil {
		return err
	}
	l.valid, l.segno, l.size, l.entries = true, segno, segHeaderSize, map[openKey]idxEntry{}
	return nil
}

// syncTree fsyncs the computer directory, tape-seg and the artifacts root, in
// that order, so a created segment, a renamed index and the directories that
// hold them survive a power loss. Every failure is returned: nothing is
// acknowledged on top of a directory entry that may not be durable.
func (a *Appender) syncTree(dir string) error {
	for _, p := range []string{dir, filepath.Dir(dir), a.root} {
		if err := a.syncDirPath(p); err != nil {
			return fmt.Errorf("tapestore: fsync directory %s: %w", p, err)
		}
	}
	return nil
}

// requirePlainDir refuses a path that exists as anything but a real directory
// (a symlink above all), so no write is redirected outside the root.
func requirePlainDir(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !fi.IsDir() {
		return corruptf("%s is not a plain directory", path)
	}
	return nil
}

// openRegular opens a segment file, refusing a symlink or any non-regular
// file at the path.
func openRegular(path string, flag int, perm os.FileMode) (*os.File, error) {
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return nil, corruptf("%s is not a regular file", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return os.OpenFile(path, flag|openNoFollow, perm)
}

// seal fsyncs a segment this writer created and writes its index (appendix
// A.5 rule 3).
func (a *Appender) seal(dir, id string, segno uint64, size int64, entries map[openKey]idxEntry) error {
	f, err := openRegular(filepath.Join(dir, SegmentFileName(segno)), os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("tapestore: open segment %d to seal: %w", segno, err)
	}
	defer f.Close()
	if err := a.syncF(f); err != nil {
		return fmt.Errorf("tapestore: fsync segment: %w", err)
	}
	if err := sealIndex(dir, segno, id, size, entries, a.syncDirPath); err != nil {
		return err
	}
	a.idxCreated.Add(1)
	a.seals.Add(1)
	return nil
}

// syncDirPath fsyncs a directory (or the test hook standing in for it).
func (a *Appender) syncDirPath(path string) error {
	if a.syncDir != nil {
		return a.syncDir(path)
	}
	return realSyncDir(path)
}

// SetSyncHookForTest replaces the segment fsync. It is a test seam for the
// platform's pin-path tests; production never calls it. Not safe to call
// while an Append is running.
func (a *Appender) SetSyncHookForTest(fn func(*os.File) error) { a.syncFile = fn }
