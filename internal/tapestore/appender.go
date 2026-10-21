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
	Syncs           uint64 `json:"syncs"`            // record fsyncs: one per Append call
	SegmentsCreated uint64 `json:"segments_created"` // seg-N.seg files created
	IndexesCreated  uint64 `json:"indexes_created"`  // seg-N.idx files created
	Seals           uint64 `json:"seals"`            // segments sealed
	TornTails       uint64 `json:"torn_tails"`       // torn tails truncated on recovery
	Errors          uint64 `json:"errors"`           // Append calls that failed after validation
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
// call's records. A call that fails part-way truncates the segment back to
// where the call began, so no part of an unacknowledged batch is left
// (a failure of that truncation too leaves a torn tail, which the next call
// finds and truncates on recovery).
//
// The per-computer state (open segment number, valid size, key set) is a
// cache. Every call revalidates it against the disk (the segment's size and
// the absence of its index); any mismatch discards it and recovers by
// scanning the open segment. Recovery truncates a torn tail and refuses
// anything else with ErrCorrupt, changing nothing on disk when it refuses.
type Appender struct {
	root        string
	sealBytes   int64
	sealRecords int
	rd          *Reader // directory listing only

	mu   sync.Mutex
	logs map[string]*computerLog

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
	// suspect are whole records written by calls that then failed; they
	// outlive cache invalidation (see appendLocked).
	suspect    map[openKey]suspectRec
	suspectSeg uint64
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
		TornTails: a.torn.Load(), Errors: a.errs.Load(),
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
// mutex and the directory lock), after the open segment has been recovered
// and before anything is written, so what it reads of the open segment is
// stable and complete: no other writer is mid-batch, and a batch that failed
// has left only whole records, which this call rewrites and fsyncs before
// it returns. build returns the items to append; none is allowed (the call
// then only makes the segment durable). An error from build aborts the call
// with nothing written. build must not call the Appender.
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
	dir := filepath.Join(a.root, SegDirName, id)
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

	if !a.cacheCurrent(l, dir) {
		l.valid = false
		if err := a.recover(l, dir, id, d); err != nil {
			return err
		}
	}
	a.pruneSuspect(l)
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
	// Rotation: a full open segment is sealed before this call writes.
	if l.size >= a.sealBytes || len(l.entries) >= a.sealRecords {
		if err := a.seal(l, dir, id, d); err != nil {
			l.valid = false
			return err
		}
		if err := a.startSegment(l, dir, id, l.segno+1, d); err != nil {
			l.valid = false
			return err
		}
	}

	f, err := os.OpenFile(filepath.Join(dir, SegmentFileName(l.segno)), os.O_RDWR, 0)
	if err != nil {
		l.valid = false
		return fmt.Errorf("tapestore: open segment %d: %w", l.segno, err)
	}
	defer f.Close()

	// Records left by a failed call are whole and may be cached by readers, so
	// they are never removed or moved; but the kernel may have dropped their
	// dirty pages when the fsync failed, so rewrite their identical bytes
	// before the fsync that this call acknowledges.
	if err := a.redirty(l, f); err != nil {
		l.valid = false
		return err
	}
	off := l.size
	pending := map[openKey]suspectRec{}
	var wrote uint64
	fail := func(err error, cut bool) error {
		// Nothing of this call is acknowledged. A record written whole stays
		// (a reader may hold its location); only a partial record is cut off,
		// which no reader has cached. The next call revalidates from disk.
		if cut {
			_ = f.Truncate(off)
		}
		if l.suspect == nil {
			l.suspect = map[openKey]suspectRec{}
		}
		l.suspectSeg = l.segno
		for k, r := range pending {
			l.suspect[k] = r
		}
		l.valid = false
		return err
	}
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
			return fail(fmt.Errorf("tapestore: append record: %w", werr), true)
		}
		pending[key] = suspectRec{off: off, rec: rec}
		off += int64(len(rec))
		wrote++
	}
	// One fsync per call, even when every item was already present: a record
	// found by recovery may follow a failed fsync and must be made durable
	// before this call acknowledges it.
	var serr error
	if a.syncFile != nil {
		serr = a.syncFile(f)
	} else {
		serr = f.Sync()
	}
	if serr != nil {
		return fail(fmt.Errorf("tapestore: fsync segment: %w", serr), false)
	}
	a.syncs.Add(1)
	start := l.size
	for k, r := range pending {
		l.entries[k] = idxEntry{digest: k.digest, ns: k.ns, off: uint64(r.off), length: uint32(len(r.rec) - recHeaderSize)}
	}
	l.size = off
	l.suspect = nil
	a.records.Add(wrote)
	a.bytes.Add(uint64(off - start))
	return nil
}

// suspectRec is a whole record of a failed call: its offset and exact bytes.
type suspectRec struct {
	off int64
	rec []byte
}

// pruneSuspect forgets suspect records that recovery did not find at their
// offset in the current open segment (a crash or a truncation took them).
func (a *Appender) pruneSuspect(l *computerLog) {
	for k, r := range l.suspect {
		e, ok := l.entries[k]
		if l.suspectSeg != l.segno || !ok || int64(e.off) != r.off {
			delete(l.suspect, k)
		}
	}
}

// redirty rewrites the identical bytes of the suspect records at their
// offsets, so that the next fsync has dirty pages to write.
func (a *Appender) redirty(l *computerLog, f *os.File) error {
	for _, r := range l.suspect {
		var n int
		var err error
		if a.writeAt != nil {
			n, err = a.writeAt(f, r.rec, r.off)
		} else {
			n, err = f.WriteAt(r.rec, r.off)
		}
		if err == nil && n != len(r.rec) {
			err = io.ErrShortWrite
		}
		if err != nil {
			return fmt.Errorf("tapestore: rewrite unsynced record: %w", err)
		}
	}
	return nil
}

// cacheCurrent reports whether the cached state still describes the disk:
// the segment has not been sealed and has exactly the cached size.
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

// recover loads the open segment of the computer (creating the first or the
// next one when there is none), truncating a torn tail and refusing any
// other damage. A refusal changes nothing on disk.
func (a *Appender) recover(l *computerLog, dir, id string, d *os.File) error {
	_, segs, err := a.rd.list(id)
	if err != nil {
		return err
	}
	if len(segs) == 0 {
		return a.startSegment(l, dir, id, 1, d)
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
		return a.startSegment(l, dir, id, last.segno+1, d)
	}
	return a.recoverOpen(l, dir, id, last.segno)
}

func (a *Appender) recoverOpen(l *computerLog, dir, id string, segno uint64) error {
	path := filepath.Join(dir, SegmentFileName(segno))
	f, err := os.OpenFile(path, os.O_RDWR, 0)
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
		if err := f.Truncate(0); err != nil {
			return fmt.Errorf("tapestore: truncate %s: %w", label, err)
		}
		if _, err := f.WriteAt(header, 0); err != nil {
			return fmt.Errorf("tapestore: rewrite header of %s: %w", label, err)
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("tapestore: fsync %s: %w", label, err)
		}
		a.torn.Add(1)
		l.valid, l.segno, l.size, l.entries = true, segno, segHeaderSize, map[openKey]idxEntry{}
		return nil
	}
	head := make([]byte, segHeaderSize)
	if err := readFull(f, head, 0); err != nil {
		return fmt.Errorf("tapestore: read header of %s: %w", label, err)
	}
	if err := checkSegHeader(head, segno, id); err != nil {
		return err
	}
	entries := map[openKey]idxEntry{}
	pos := int64(segHeaderSize)
	torn := false
	for pos < size {
		h, next, bad, err := scanRecord(f, pos, size)
		if err != nil {
			return err
		}
		if bad != "" {
			if size-pos > recHeaderSize+MaxRecordBytes {
				return corruptf("%s: %d unreadable bytes at %d exceed any single record: %s", label, size-pos, pos, bad)
			}
			later, err := validRecordAfter(f, pos, size)
			if err != nil {
				return err
			}
			if later {
				return corruptf("%s: bad record at %d is followed by valid records: %s", label, pos, bad)
			}
			torn = true
			break
		}
		body := make([]byte, h.length)
		if err := readFull(f, body, pos+recHeaderSize); err != nil {
			return fmt.Errorf("tapestore: read %s: %w", label, err)
		}
		if sha256.Sum256(body) != h.digest {
			return corruptf("%s: record at %d does not hash to its digest", label, pos)
		}
		k := openKey{h.ns, h.digest}
		if _, dup := entries[k]; !dup {
			entries[k] = idxEntry{digest: h.digest, ns: h.ns, off: uint64(pos), length: h.length}
		}
		pos = next
	}
	if torn {
		if err := f.Truncate(pos); err != nil {
			return fmt.Errorf("tapestore: truncate torn tail of %s: %w", label, err)
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("tapestore: fsync %s: %w", label, err)
		}
		a.torn.Add(1)
	}
	l.valid, l.segno, l.size, l.entries = true, segno, pos, entries
	return nil
}

// startSegment creates segment segno exclusively, writes and fsyncs its
// header and makes its directory entries durable.
func (a *Appender) startSegment(l *computerLog, dir, id string, segno uint64, d *os.File) error {
	f, err := os.OpenFile(filepath.Join(dir, SegmentFileName(segno)), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return fmt.Errorf("tapestore: create segment %d: %w", segno, err)
	}
	defer f.Close()
	a.segsCreated.Add(1)
	if _, err := f.WriteAt(encodeSegHeader(segno, id), 0); err != nil {
		return fmt.Errorf("tapestore: write segment header: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("tapestore: fsync segment header: %w", err)
	}
	if err := d.Sync(); err != nil {
		return fmt.Errorf("tapestore: fsync %s: %w", dir, err)
	}
	for _, parent := range []string{filepath.Dir(dir), a.root} {
		_ = a.syncDirPath(parent)
	}
	l.valid, l.segno, l.size, l.entries = true, segno, segHeaderSize, map[openKey]idxEntry{}
	return nil
}

// seal fsyncs the open segment and writes its index (appendix A.5 rule 3).
func (a *Appender) seal(l *computerLog, dir, id string, d *os.File) error {
	f, err := os.OpenFile(filepath.Join(dir, SegmentFileName(l.segno)), os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("tapestore: open segment %d to seal: %w", l.segno, err)
	}
	defer f.Close()
	if err := a.redirty(l, f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("tapestore: fsync segment: %w", err)
	}
	l.suspect = nil
	if err := sealIndex(dir, l.segno, id, l.size, l.entries, a.syncDirPath); err != nil {
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
