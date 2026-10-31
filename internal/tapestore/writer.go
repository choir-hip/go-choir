package tapestore

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// SegmentWriter appends records to one segment and seals it with an index.
//
// THIS IS NOT THE PRODUCTION WRITER. It exists for test fixtures and for
// offline tools (the byte-identical repack tool, deferred). A guard test
// fails if a non-test source outside this package references it. The
// production writer, with the per-computer lock, open-segment recovery and
// rotation, is Appender (appender.go).
//
// A writer is not safe for concurrent use.
type SegmentWriter struct {
	dir        string
	computerID string
	segno      uint64
	f          *os.File
	off        int64
	entries    map[openKey]idxEntry
	sealed     bool
}

// CreateSegment creates seg-<segno>.seg exclusively under
// <artifactsRoot>/tape-seg/<computerID>/ and writes its header.
func CreateSegment(artifactsRoot, computerID string, segno uint64) (*SegmentWriter, error) {
	if !ValidComputerID(computerID) {
		return nil, fmt.Errorf("tapestore: invalid computer id %q", computerID)
	}
	dir := filepath.Join(filepath.Clean(artifactsRoot), SegDirName, computerID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("tapestore: create %s: %w", dir, err)
	}
	f, err := os.OpenFile(filepath.Join(dir, SegmentFileName(segno)), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return nil, fmt.Errorf("tapestore: create segment %d: %w", segno, err)
	}
	if _, err := f.Write(encodeSegHeader(segno, computerID)); err != nil {
		f.Close()
		return nil, fmt.Errorf("tapestore: write segment header: %w", err)
	}
	return &SegmentWriter{dir: dir, computerID: computerID, segno: segno, f: f, off: segHeaderSize, entries: map[openKey]idxEntry{}}, nil
}

// Append adds one record and returns the SHA-256 hex of body. Appending a
// (namespace, body) pair already in this segment is a no-op.
func (w *SegmentWriter) Append(ns NS, body []byte) (string, error) {
	if w.sealed || w.f == nil {
		return "", errors.New("tapestore: segment writer is closed")
	}
	if !ns.valid() {
		return "", fmt.Errorf("tapestore: invalid namespace %d", uint8(ns))
	}
	if len(body) > MaxRecordBytes {
		return "", fmt.Errorf("tapestore: record of %d bytes exceeds the %d byte limit", len(body), MaxRecordBytes)
	}
	digest := digestBytes(body)
	key := openKey{ns, digest}
	if _, dup := w.entries[key]; dup {
		return hexDigest(digest), nil
	}
	rec := packRecord(ns, digest, body)
	if _, err := w.f.Write(rec); err != nil {
		return "", fmt.Errorf("tapestore: append record: %w", err)
	}
	w.entries[key] = idxEntry{digest: digest, ns: ns, off: uint64(w.off), length: uint32(len(rec) - recHeaderSize)}
	w.off += int64(len(rec))
	return hexDigest(digest), nil
}

// Size is the segment's current size in bytes.
func (w *SegmentWriter) Size() int64 { return w.off }

// Records is the number of distinct records appended.
func (w *SegmentWriter) Records() int { return len(w.entries) }

// Sync flushes the segment to stable storage.
func (w *SegmentWriter) Sync() error { return w.f.Sync() }

// Seal fsyncs the segment, writes its index to a temp file, fsyncs it,
// renames it into place and fsyncs the directory (section 4.1, rotation).
func (w *SegmentWriter) Seal() error {
	if w.sealed || w.f == nil {
		return errors.New("tapestore: segment writer is closed")
	}
	if err := w.f.Sync(); err != nil {
		return fmt.Errorf("tapestore: fsync segment: %w", err)
	}
	if err := sealIndex(w.dir, w.segno, w.computerID, w.off, w.entries, nil); err != nil {
		return err
	}
	w.sealed = true
	err := w.f.Close()
	w.f = nil
	return err
}

// sealIndex writes seg-<segno>.idx for a segment that is already fsynced:
// temp file, fsync, rename, fsync of the directory (appendix A.5 rule 3). A
// crash before the rename leaves the segment open, which readers scan.
func sealIndex(dir string, segno uint64, computerID string, size int64, entries map[openKey]idxEntry, syncDir func(string) error) error {
	if syncDir == nil {
		syncDir = realSyncDir
	}
	idx := encodeIndex(segno, computerID, uint64(size), sortedEntries(entries))
	final := filepath.Join(dir, IndexFileName(segno))
	tmp := final + ".tmp"
	// A temp file left by a crash is replaced; anything that is not a plain
	// file (a symlink above all) is refused, and the create refuses to follow.
	if fi, err := os.Lstat(tmp); err == nil {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("tapestore: %s is not a regular file", tmp)
		}
		if err := os.Remove(tmp); err != nil {
			return fmt.Errorf("tapestore: remove stale index temp: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	t, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|openNoFollow, 0o640)
	if err != nil {
		return fmt.Errorf("tapestore: create index: %w", err)
	}
	if _, err := t.Write(idx); err != nil {
		t.Close()
		return fmt.Errorf("tapestore: write index: %w", err)
	}
	if err := t.Sync(); err != nil {
		t.Close()
		return fmt.Errorf("tapestore: fsync index: %w", err)
	}
	if err := t.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("tapestore: install index: %w", err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("tapestore: fsync directory after installing index: %w", err)
	}
	return nil
}

// realSyncDir opens a directory and fsyncs it.
func realSyncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// Close releases the file and leaves the segment unsealed (open).
func (w *SegmentWriter) Close() error {
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
