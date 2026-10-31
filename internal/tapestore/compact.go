package tapestore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Compaction of sealed segments: rule F of
// docs/design/small-object-storage-2026-10-31.md.
//
// Why. A segment is written by appends, each fsynced. On btrfs every fsync of
// a partly filled last 4 KiB block writes that block again elsewhere (copy on
// write), and the extent that held the old copy stays allocated as long as
// any block of it is still referenced (extent bookending). Measured on Node B
// 2026-10-31 (compsize, metadata only): the 830 sealed segments reference
// 29.38 GB and occupy 50.46 GB in 5,148,116 extents, about one wasted 4 KiB
// block per append. The files also carry the btrfs no-compress inode flag
// (lsattr "m"), which btrfs sets by itself the first time a write of the file
// does not compress (sealed payloads are ciphertext). A sealed segment never
// changes again, so writing it once more as a fresh file, sequentially, gives
// a few large extents and frees every bookended block.
//
// What CompactSealed does, for one sealed segment:
//
//  1. Checks that the segment is sealed (its index exists and validates, the
//     segment's size and header match it). An unsealed segment is never
//     touched: it is the appender's.
//  2. Opens the segment and creates a fresh temp file beside it
//     (seg-N.seg.compact-*, a name no reader, view or appender lists).
//  3. Streams the segment into the temp file through one buffer (1 MiB by
//     default), hashing the source bytes as it goes, paced to a byte rate.
//  4. Copies mode, owner and mtime, fsyncs the temp file.
//  5. Re-reads the temp file and compares its size and SHA-256 with the
//     source's; optionally reads back every indexed record (crc and SHA-256
//     of the body). Any difference refuses the rename (ErrCompactMismatch)
//     and removes the temp file.
//  6. Checks that the path still names the file it copied (not removed by a
//     purge, not replaced by another compactor) and the index still exists.
//  7. Renames the temp file over the segment and fsyncs the directory.
//
// The copy is byte for byte, so every index offset, the index itself, the
// segment header and every record are unchanged: no reader, view, census or
// older binary can tell the difference except by inode number.
//
// Concurrency. Compaction takes no appender lock and never blocks an append: a
// sealed segment is immutable, so a copy of it is valid whenever it is made.
// Two compactors of one segment (the seal-time worker and the backfill tool)
// each install a verified identical copy; the second rename replaces one
// identical file with another. A reader that has the old file open keeps
// reading the old inode (Linux rename semantics); a reader that opens the
// path after the rename reads the new one; both hold the same bytes. The
// Reader caches only a sealed segment's fanout and size, and opens the
// segment by path per read.
//
// Crash table (power loss or kill at each step):
//
//	before step 2              nothing changed
//	during 2-5 (temp partial,   the segment is untouched; the temp file is
//	  not fsynced or unverified) ignored by every reader and removed by the
//	                             next compaction of that segment once older
//	                             than StaleTempAge
//	after 5, before the rename  as above; the temp is whole but unused
//	after the rename, before    the directory names either the old file or
//	  the directory fsync        the new one; the new one was fsynced before
//	                             the rename, both are the same bytes
//	after the directory fsync   the new file; the old extents are freed when
//	                             the last open fd closes and btrfs commits
//
// Failure modes and their tests (compact_test.go): crash mid-copy (temp left,
// segment intact, next run sweeps it); a copy that differs refuses the rename;
// a reader reading during the copy and holding the old fd across the rename;
// an appender writing the open segment and another computer meanwhile (not
// blocked); a seal while a reader reads (the seal-time worker); a segment
// removed or replaced during the copy is not resurrected; rollback by revert
// (an old binary reads a compacted store and ignores a stray temp file).
//
// Cost. Memory: one buffer (BufferBytes, 1 MiB by default) plus, with
// VerifyRecords, the index entries (at most 50,000 x 45 B = 2.3 MB) and one
// record body at a time. CPU: one SHA-256 pass over the source and one over
// the copy (about 0.1 s per 64 MiB segment), plus btrfs's own zstd when
// Compress is set (in kernel writeback). IO: one sequential read and two
// sequential passes over the copy (write, verify read), paced.

// ErrCompactMismatch is returned when the copy of a segment does not match
// the original; the rename is refused and the segment is unchanged.
var ErrCompactMismatch = errors.New("tapestore: compacted copy does not match the sealed segment")

// ErrCompactCanceled is returned when Cancel closed during a copy; the
// segment is unchanged and the temp file removed.
var ErrCompactCanceled = errors.New("tapestore: compaction canceled")

const (
	defaultCompactBuffer       = 1 << 20
	defaultMinAvgExtentBytes   = 64 << 10
	defaultStaleCompactTempAge = time.Hour
	compactTempInfix           = ".compact-"
)

// CompactOptions configure CompactSealed.
type CompactOptions struct {
	// BufferBytes is the copy buffer; 0 is 1 MiB.
	BufferBytes int
	// BytesPerSecond paces the copy (read side); 0 is unpaced.
	BytesPerSecond int64
	// VerifyRecords reads every indexed record back from the copy (crc and
	// SHA-256) before the rename, on top of the whole-file digest.
	VerifyRecords bool
	// Compress asks btrfs to compress the copy (the btrfs.compression=zstd
	// property on the new inode, so the no-compress heuristic cannot switch
	// it off after one incompressible chunk). Ignored where unsupported.
	Compress bool
	// MinAvgExtentBytes: a segment with no more extents than ceil(size /
	// MinAvgExtentBytes) is already compact and is skipped. 0 is 64 KiB; negative never
	// skips. Where extents cannot be counted (not Linux) nothing is skipped.
	MinAvgExtentBytes int64
	// DryRun measures and decides, and writes nothing.
	DryRun bool
	// StaleTempAge: temp files of this segment older than this are removed
	// before the copy. 0 is one hour.
	StaleTempAge time.Duration
	// Cancel, when closed, aborts a copy in progress.
	Cancel <-chan struct{}

	// Test seams.
	testAfterCopy    func(tmp *os.File) error // after the fsync, before the verify
	testBeforeRename func(tmp string) error   // after the verify, before the path checks
}

// Compaction actions.
const (
	CompactActionCompacted    = "compacted"
	CompactActionWouldCompact = "would_compact"
	CompactActionSkipped      = "skipped"
)

// CompactResult describes one segment's compaction.
type CompactResult struct {
	Computer string `json:"computer"`
	Segno    uint64 `json:"segno"`
	Size     int64  `json:"size"`
	Action   string `json:"action"`
	Skip     string `json:"skip,omitempty"`
	// StBlocksBefore/After are st_blocks x 512. On btrfs st_blocks counts the
	// bytes the file references, not the bookended extents it pins, so it
	// does not show the reclaim; disk usage (compsize) does.
	StBlocksBefore int64 `json:"st_blocks_bytes_before"`
	StBlocksAfter  int64 `json:"st_blocks_bytes_after,omitempty"`
	// ExtentsBefore/After are FIEMAP extent counts; -1 where unknown.
	ExtentsBefore int  `json:"extents_before"`
	ExtentsAfter  int  `json:"extents_after,omitempty"`
	Shared        bool `json:"shared,omitempty"`
	// SHA256 is the digest of the segment's bytes, equal in both files.
	SHA256          string `json:"sha256,omitempty"`
	RecordsVerified int    `json:"records_verified,omitempty"`
}

// CompactSealed rewrites one sealed segment as a fresh, sequentially written
// file with identical bytes (see the comment above). It returns a result for
// every segment it looked at; an error means nothing was installed.
func CompactSealed(artifactsRoot, computerID string, segno uint64, opts CompactOptions) (CompactResult, error) {
	res := CompactResult{Computer: computerID, Segno: segno, ExtentsBefore: -1}
	if !ValidComputerID(computerID) {
		return res, fmt.Errorf("tapestore: invalid computer id %q", computerID)
	}
	root := filepath.Clean(artifactsRoot)
	dir := filepath.Join(root, SegDirName, computerID)
	for _, p := range []string{filepath.Join(root, SegDirName), dir} {
		if err := requirePlainDir(p); err != nil {
			return res, err
		}
	}
	segPath := filepath.Join(dir, SegmentFileName(segno))
	idxPath := filepath.Join(dir, IndexFileName(segno))
	if _, err := os.Lstat(idxPath); errors.Is(err, fs.ErrNotExist) {
		res.Action, res.Skip = CompactActionSkipped, "unsealed"
		return res, nil
	} else if err != nil {
		return res, err
	}
	sealed, err := openSealed(dir, computerID, segno)
	if err != nil {
		return res, err
	}
	src, err := openRegular(segPath, os.O_RDONLY, 0)
	if err != nil {
		return res, fmt.Errorf("tapestore: open segment %d: %w", segno, err)
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil {
		return res, err
	}
	size := srcInfo.Size()
	res.Size = size
	if size != sealed.segSize {
		return res, corruptf("computer %s segment %d: %d bytes, index sealed %d", computerID, segno, size, sealed.segSize)
	}
	res.StBlocksBefore = statBlocks(srcInfo)
	n, shared, ferr := fileExtents(src)
	if ferr == nil {
		res.ExtentsBefore, res.Shared = n, shared
	}
	if shared {
		// A snapshot or reflink holds these extents too: a rewrite frees
		// nothing and doubles the space until the other holder goes.
		res.Action, res.Skip = CompactActionSkipped, "shared extents"
		return res, nil
	}
	min := opts.MinAvgExtentBytes
	if min == 0 {
		min = defaultMinAvgExtentBytes
	}
	// Compact: no more extents than a file of this size needs at min bytes
	// each (a file under min bytes in one extent is compact).
	if min > 0 && res.ExtentsBefore > 0 && int64(res.ExtentsBefore) <= (size+min-1)/min {
		res.Action, res.Skip = CompactActionSkipped, "already compact"
		return res, nil
	}
	if opts.DryRun {
		res.Action = CompactActionWouldCompact
		return res, nil
	}
	sweepCompactTemps(dir, segno, opts.StaleTempAge)

	tmp, err := os.CreateTemp(dir, SegmentFileName(segno)+compactTempInfix+"*")
	if err != nil {
		return res, fmt.Errorf("tapestore: create compaction temp: %w", err)
	}
	tmpPath := tmp.Name()
	installed := false
	defer func() {
		tmp.Close()
		if !installed {
			_ = os.Remove(tmpPath)
		}
	}()
	if opts.Compress {
		setCompressHint(tmp)
	}
	if err := tmp.Chmod(srcInfo.Mode().Perm()); err != nil {
		return res, fmt.Errorf("tapestore: chmod compaction temp: %w", err)
	}
	if err := copyOwner(srcInfo, tmp); err != nil {
		return res, fmt.Errorf("tapestore: chown compaction temp: %w", err)
	}

	bufSize := opts.BufferBytes
	if bufSize <= 0 {
		bufSize = defaultCompactBuffer
	}
	buf := make([]byte, bufSize)
	srcSum, err := pacedCopy(tmp, src, size, buf, opts)
	if err != nil {
		return res, err
	}
	mtime := srcInfo.ModTime()
	if err := os.Chtimes(tmpPath, mtime, mtime); err != nil {
		return res, fmt.Errorf("tapestore: set mtime of compaction temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return res, fmt.Errorf("tapestore: fsync compaction temp: %w", err)
	}
	if opts.testAfterCopy != nil {
		if err := opts.testAfterCopy(tmp); err != nil {
			return res, err
		}
	}

	// Verify the copy against the original.
	tmpInfo, err := tmp.Stat()
	if err != nil {
		return res, err
	}
	if tmpInfo.Size() != size {
		return res, fmt.Errorf("%w: copy is %d bytes, segment %d", ErrCompactMismatch, tmpInfo.Size(), size)
	}
	copySum, err := hashFile(tmp, size, buf)
	if err != nil {
		return res, fmt.Errorf("tapestore: re-read compaction temp: %w", err)
	}
	if !bytes.Equal(copySum, srcSum) {
		return res, fmt.Errorf("%w: computer %s segment %d: copy sha256 %x, segment %x", ErrCompactMismatch, computerID, segno, copySum, srcSum)
	}
	head := make([]byte, segHeaderSize)
	if err := readFull(tmp, head, 0); err != nil {
		return res, err
	}
	if err := checkSegHeader(head, segno, computerID); err != nil {
		return res, fmt.Errorf("%w: %v", ErrCompactMismatch, err)
	}
	if opts.VerifyRecords {
		n, err := verifyIndexedRecords(tmp, sealed, size)
		if err != nil {
			return res, fmt.Errorf("%w: %v", ErrCompactMismatch, err)
		}
		res.RecordsVerified = n
	}
	res.SHA256 = hex.EncodeToString(srcSum)
	if opts.testBeforeRename != nil {
		if err := opts.testBeforeRename(tmpPath); err != nil {
			return res, err
		}
	}

	// The path must still name the file that was copied, and still be sealed.
	cur, err := os.Lstat(segPath)
	if err != nil || !os.SameFile(cur, srcInfo) {
		res.Action, res.Skip = CompactActionSkipped, "segment replaced or removed during the copy"
		return res, nil
	}
	if _, err := os.Lstat(idxPath); err != nil {
		res.Action, res.Skip = CompactActionSkipped, "index removed during the copy"
		return res, nil
	}
	if err := os.Rename(tmpPath, segPath); err != nil {
		return res, fmt.Errorf("tapestore: install compacted segment %d: %w", segno, err)
	}
	installed = true
	if err := realSyncDir(dir); err != nil {
		// The rename is done and both names hold the same fsynced bytes.
		return res, fmt.Errorf("tapestore: fsync directory after compaction: %w", err)
	}
	res.Action = CompactActionCompacted
	if after, err := tmp.Stat(); err == nil {
		res.StBlocksAfter = statBlocks(after)
	}
	if n, _, err := fileExtents(tmp); err == nil {
		res.ExtentsAfter = n
	} else {
		res.ExtentsAfter = -1
	}
	dropCache(src)
	dropCache(tmp)
	return res, nil
}

// pacedCopy streams size bytes of src into dst through buf and returns the
// SHA-256 of what it read.
func pacedCopy(dst, src *os.File, size int64, buf []byte, opts CompactOptions) ([]byte, error) {
	h := sha256.New()
	start := time.Now()
	var off int64
	for off < size {
		if err := canceled(opts.Cancel); err != nil {
			return nil, err
		}
		chunk := buf
		if rest := size - off; rest < int64(len(chunk)) {
			chunk = chunk[:rest]
		}
		if err := readFull(src, chunk, off); err != nil {
			return nil, fmt.Errorf("tapestore: read segment for compaction: %w", err)
		}
		h.Write(chunk)
		if _, err := dst.Write(chunk); err != nil {
			return nil, fmt.Errorf("tapestore: write compaction temp: %w", err)
		}
		off += int64(len(chunk))
		if opts.BytesPerSecond > 0 {
			due := time.Duration(float64(off) / float64(opts.BytesPerSecond) * float64(time.Second))
			if wait := due - time.Since(start); wait > 0 {
				if err := sleepCancel(wait, opts.Cancel); err != nil {
					return nil, err
				}
			}
		}
	}
	// The file must not have grown: a sealed segment never does.
	var one [1]byte
	if n, _ := src.ReadAt(one[:], size); n != 0 {
		return nil, corruptf("sealed segment grew during compaction")
	}
	return h.Sum(nil), nil
}

func hashFile(f *os.File, size int64, buf []byte) ([]byte, error) {
	var h hash.Hash = sha256.New()
	if _, err := io.CopyBuffer(h, io.NewSectionReader(f, 0, size), buf); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// verifyIndexedRecords reads every record the index names from f, with the
// reader's full check. It holds the index entries and one record at a time.
func verifyIndexedRecords(f *os.File, s *sealedSeg, size int64) (int, error) {
	raw, err := os.ReadFile(s.idxPath)
	if err != nil {
		return 0, err
	}
	if len(raw) != idxFixedSize+idxEntrySize*int(s.count)+idxTrailerSize {
		return 0, corruptf("index %d changed size", s.segno)
	}
	label := fmt.Sprintf("computer %s segment %d (compacted copy)", s.computer, s.segno)
	for i := 0; i < int(s.count); i++ {
		e := decodeIdxEntry(raw[idxFixedSize+idxEntrySize*i:])
		if _, err := readRecordAt(f, label, int64(e.off), size, e.ns, e.digest, int64(e.length)); err != nil {
			return i, err
		}
	}
	return int(s.count), nil
}

// sweepCompactTemps removes this segment's compaction temp files older than
// age. A younger one may belong to a compactor still running; removing it
// would only make that compactor's rename fail, harmlessly.
func sweepCompactTemps(dir string, segno uint64, age time.Duration) {
	if age <= 0 {
		age = defaultStaleCompactTempAge
	}
	prefix := SegmentFileName(segno) + compactTempInfix
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), prefix) || !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < age {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// SealedSegments lists the computer's sealed segment numbers, ascending.
func (r *Reader) SealedSegments(computerID string) ([]uint64, error) {
	_, segs, err := r.list(computerID)
	if err != nil {
		return nil, err
	}
	var out []uint64
	for _, s := range segs {
		if s.hasIdx {
			out = append(out, s.segno)
		}
	}
	return out, nil
}

func canceled(c <-chan struct{}) error {
	if c == nil {
		return nil
	}
	select {
	case <-c:
		return ErrCompactCanceled
	default:
		return nil
	}
}

func sleepCancel(d time.Duration, c <-chan struct{}) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-c:
		return ErrCompactCanceled
	}
}

// RunLowPriority runs fn on an OS thread of its own whose CPU priority is
// nice 19 and IO class idle (Linux; elsewhere fn just runs), and waits for
// it. The thread is discarded when fn returns, so the priority never leaks
// to other goroutines.
func RunLowPriority(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		lockLowPriorityThread()
		fn()
	}()
	<-done
}
