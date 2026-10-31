package tapestore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
)

// Reader resolves tape objects from segments first and legacy files second.
// It is safe for concurrent use and holds only immutable sealed-segment
// headers and the scan state of open segments. It never writes.
type Reader struct {
	root string // the artifacts root; segments live in <root>/tape-seg

	mu     sync.Mutex
	sealed map[string]*sealedSeg // by index path
	open   map[string]*openSeg   // by segment path

	segmentHits, legacyHits, notFound, corrupt, digestMismatch atomic.Uint64
	segmentErrorFallbacks                                      atomic.Uint64
	fallbackLogged                                             sync.Map // computer id -> struct{}: one log line each
}

// Stats are the reader's lifetime counters.
type Stats struct {
	SegmentHits    uint64 `json:"segment_hits"`
	LegacyHits     uint64 `json:"legacy_hits"`
	NotFound       uint64 `json:"not_found"`
	Corrupt        uint64 `json:"corrupt"`
	DigestMismatch uint64 `json:"digest_mismatch"`
	// SegmentErrorFallbacks counts reads whose segment lookup failed and
	// that were served from a legacy file hashing to the digest. Any
	// non-zero value is segment corruption to investigate.
	SegmentErrorFallbacks uint64 `json:"segment_error_fallbacks"`
}

// NewReader returns a reader over the artifacts root. It performs no I/O.
func NewReader(artifactsRoot string) *Reader {
	return &Reader{
		root:   filepath.Clean(artifactsRoot),
		sealed: map[string]*sealedSeg{},
		open:   map[string]*openSeg{},
	}
}

// Root is the artifacts root the reader was opened on.
func (r *Reader) Root() string { return r.root }

// Stats returns the counters.
func (r *Reader) Stats() Stats {
	return Stats{
		SegmentHits:    r.segmentHits.Load(),
		LegacyHits:     r.legacyHits.Load(),
		NotFound:       r.notFound.Load(),
		Corrupt:        r.corrupt.Load(),
		DigestMismatch: r.digestMismatch.Load(),

		SegmentErrorFallbacks: r.segmentErrorFallbacks.Load(),
	}
}

// Read returns the bytes of (ns, digest) for computerID: from a segment if
// one holds the digest, otherwise from the legacy file. The bytes are hashed
// and compared with digest before they are returned. A segment error falls
// back to the legacy file only when that file hashes to digest, so the bytes
// are exactly the object asked for; the fallback is counted
// (SegmentErrorFallbacks) and logged once per computer. Without such a file
// the segment error is returned. ReadSegment never falls back.
func (r *Reader) Read(computerID string, ns NS, digest string) ([]byte, Source, error) {
	if !ns.valid() {
		return nil, 0, fmt.Errorf("tapestore: invalid namespace %d", uint8(ns))
	}
	if !ValidDigest(digest) {
		return nil, 0, fmt.Errorf("tapestore: invalid digest %q", digest)
	}
	body, found, err := r.readSegment(computerID, ns, digest)
	if err != nil {
		if errors.Is(err, ErrDigestMismatch) {
			r.digestMismatch.Add(1)
		} else {
			r.corrupt.Add(1)
		}
		if errors.Is(err, ErrUnsupportedVersion) {
			return nil, 0, err // a format this binary does not know is not rot
		}
		legacy, lerr := r.readLegacy(ns, digest)
		if lerr == nil {
			r.segmentErrorFallbacks.Add(1)
			if _, seen := r.fallbackLogged.LoadOrStore(computerID, struct{}{}); !seen {
				log.Printf("tapestore: computer %s: segment read failed, served a digest-verified legacy copy: %v", computerID, err)
			}
			return legacy, SourceLegacy, nil
		}
		if !errors.Is(lerr, ErrNotFound) && !errors.Is(lerr, ErrDigestMismatch) {
			return nil, 0, errors.Join(err, lerr)
		}
		return nil, 0, err
	}
	if found {
		r.segmentHits.Add(1)
		return body, SourceSegment, nil
	}
	body, err = r.readLegacy(ns, digest)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			r.notFound.Add(1)
		case errors.Is(err, ErrDigestMismatch):
			r.digestMismatch.Add(1)
		}
		return nil, 0, err
	}
	r.legacyHits.Add(1)
	return body, SourceLegacy, nil
}

// ReadLegacy reads only the legacy file, with the same digest check. It is
// for census tools that must prove a segment copy against the legacy one.
func (r *Reader) ReadLegacy(ns NS, digest string) ([]byte, error) {
	if !ns.valid() || !ValidDigest(digest) {
		return nil, fmt.Errorf("tapestore: invalid namespace or digest")
	}
	return r.readLegacy(ns, digest)
}

// ReadSegment reads only the segments. found is false when no segment holds
// the digest. A segment error is returned.
func (r *Reader) ReadSegment(computerID string, ns NS, digest string) ([]byte, bool, error) {
	if !ns.valid() || !ValidDigest(digest) {
		return nil, false, fmt.Errorf("tapestore: invalid namespace or digest")
	}
	return r.readSegment(computerID, ns, digest)
}

func (r *Reader) readLegacy(ns NS, digest string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(ns.LegacyRel(digest))))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s %s", ErrNotFound, ns, digest)
		}
		return nil, fmt.Errorf("tapestore: read legacy %s %s: %w", ns, digest, err)
	}
	if hexDigest(sha256.Sum256(raw)) != digest {
		return nil, fmt.Errorf("%w: legacy %s file does not hash to %s", ErrDigestMismatch, ns, digest)
	}
	return raw, nil
}

// HasSegments reports whether the computer has any segment or index file. An
// unreadable directory is an error: callers that refuse on segments must not
// treat "could not tell" as "none".
func (r *Reader) HasSegments(computerID string) (bool, error) {
	_, segs, err := r.list(computerID)
	if err != nil {
		return false, err
	}
	return len(segs) > 0, nil
}

// segFiles is one segment number's files on disk.
type segFiles struct {
	segno  uint64
	hasSeg bool
	hasIdx bool
}

// list reads the computer's segment directory. A missing directory (or an
// id that cannot name one) has no segments. An index without its segment is
// corruption.
func (r *Reader) list(computerID string) (string, []segFiles, error) {
	if !ValidComputerID(computerID) {
		return "", nil, nil
	}
	dir := filepath.Join(r.root, SegDirName, computerID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return dir, nil, nil
		}
		return dir, nil, fmt.Errorf("tapestore: list %s: %w", computerID, err)
	}
	byNo := map[uint64]*segFiles{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		segno, ext, ok := parseSegName(entry.Name())
		if !ok {
			continue
		}
		f := byNo[segno]
		if f == nil {
			f = &segFiles{segno: segno}
			byNo[segno] = f
		}
		if ext == "seg" {
			f.hasSeg = true
		} else {
			f.hasIdx = true
		}
	}
	segs := make([]segFiles, 0, len(byNo))
	for _, f := range byNo {
		if f.hasIdx && !f.hasSeg {
			return dir, nil, corruptf("computer %s: index %s has no segment file", computerID, IndexFileName(f.segno))
		}
		segs = append(segs, *f)
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].segno < segs[j].segno })
	return dir, segs, nil
}

// readSegment probes the computer's segments newest first. A hit is verified
// and returned. Absence from every segment is (nil, false, nil).
func (r *Reader) readSegment(computerID string, ns NS, digest string) ([]byte, bool, error) {
	dir, segs, err := r.list(computerID)
	if err != nil || len(segs) == 0 {
		return nil, false, err
	}
	var want [32]byte
	if err := decodeHex32(&want, digest); err != nil {
		return nil, false, err
	}
	last := segs[len(segs)-1].segno
	// Every sealed segment of the snapshot retires its open-scan state,
	// whichever segment this lookup ends up visiting: a hit on the newest
	// segment returns before older ones are probed, and the cache must stay
	// bounded by the number of open segments regardless.
	r.retireSealed(dir, segs)
	for i := len(segs) - 1; i >= 0; i-- {
		f := segs[i]
		var body []byte
		var found bool
		if f.hasIdx {
			s, err := r.loadSealed(dir, computerID, f.segno)
			if err != nil {
				return nil, false, err
			}
			body, found, err = s.read(ns, want)
			if err != nil {
				return nil, false, err
			}
		} else {
			o := r.openFor(filepath.Join(dir, SegmentFileName(f.segno)), f.segno)
			body, found, err = o.read(computerID, f.segno == last, ns, want)
			if err != nil {
				return nil, false, err
			}
		}
		if found {
			return body, true, nil
		}
	}
	return nil, false, nil
}

func decodeHex32(dst *[32]byte, digest string) error {
	for i := 0; i < 32; i++ {
		hi, lo := unhex(digest[2*i]), unhex(digest[2*i+1])
		if hi < 0 || lo < 0 {
			return fmt.Errorf("tapestore: invalid digest %q", digest)
		}
		dst[i] = byte(hi<<4 | lo)
	}
	return nil
}

func unhex(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	}
	return -1
}

// readRecordAt reads and fully verifies the record at off in a segment file:
// header fields against want, length bound, crc, and SHA-256 of the body.
func readRecordAt(f io.ReaderAt, segLabel string, off int64, limit int64, ns NS, want [32]byte, wantLen int64) ([]byte, error) {
	if off < segHeaderSize || off+recHeaderSize > limit {
		return nil, corruptf("%s: record offset %d outside the segment", segLabel, off)
	}
	head := make([]byte, recHeaderSize)
	if _, err := f.ReadAt(head, off); err != nil {
		return nil, corruptf("%s: read record header at %d: %v", segLabel, off, err)
	}
	h, err := parseRecHeader(head)
	if err != nil {
		if errors.Is(err, ErrUnsupportedVersion) {
			return nil, unsupportedf("%s: record at %d: %v", segLabel, off, err)
		}
		return nil, corruptf("%s: record at %d: %v", segLabel, off, err)
	}
	if h.ns != ns || h.digest != want {
		return nil, corruptf("%s: record at %d is not %s %x", segLabel, off, ns, want)
	}
	if wantLen >= 0 && int64(h.length) != wantLen {
		return nil, corruptf("%s: record at %d has length %d, index says %d", segLabel, off, h.length, wantLen)
	}
	end := off + recHeaderSize + int64(h.length)
	if end > limit {
		return nil, corruptf("%s: record at %d runs past the end of the segment", segLabel, off)
	}
	body := make([]byte, h.length)
	if h.length > 0 {
		if _, err := f.ReadAt(body, off+recHeaderSize); err != nil {
			return nil, corruptf("%s: read record body at %d: %v", segLabel, off, err)
		}
	}
	if recordCRC(head[4:42], body) != h.crc {
		return nil, corruptf("%s: record at %d fails its crc", segLabel, off)
	}
	out, err := decodeBody(h.version, body, want)
	if err != nil {
		return nil, corruptf("%s: record at %d: %v", segLabel, off, err)
	}
	return out, nil
}

// --- sealed segments ---------------------------------------------------------

type sealedSeg struct {
	segno    uint64
	segPath  string
	idxPath  string
	segSize  int64
	count    uint32
	fanout   [256]uint32
	computer string
}

func (r *Reader) loadSealed(dir, computerID string, segno uint64) (*sealedSeg, error) {
	idxPath := filepath.Join(dir, IndexFileName(segno))
	r.mu.Lock()
	s := r.sealed[idxPath]
	r.mu.Unlock()
	if s != nil {
		return s, nil
	}
	s, err := openSealed(dir, computerID, segno)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.sealed[idxPath] = s
	r.mu.Unlock()
	return s, nil
}

// openSealed validates an index end to end (size, crc, header, fanout) and
// the segment it describes (size and header), then keeps only the fanout.
func openSealed(dir, computerID string, segno uint64) (*sealedSeg, error) {
	idxPath := filepath.Join(dir, IndexFileName(segno))
	segPath := filepath.Join(dir, SegmentFileName(segno))
	raw, err := os.ReadFile(idxPath)
	if err != nil {
		return nil, fmt.Errorf("tapestore: read index %s: %w", idxPath, err)
	}
	label := fmt.Sprintf("computer %s index %d", computerID, segno)
	if len(raw) < idxFixedSize+idxTrailerSize {
		return nil, corruptf("%s: truncated (%d bytes)", label, len(raw))
	}
	if [4]byte(raw[0:4]) != idxMagic {
		return nil, corruptf("%s: bad magic", label)
	}
	if raw[4] != idxVersion {
		return nil, unsupportedf("%s: unsupported version %d", label, raw[4])
	}
	body := len(raw) - idxTrailerSize
	if crc32.Checksum(raw[:body], castagnoli) != binary.BigEndian.Uint32(raw[body:]) {
		return nil, corruptf("%s: crc mismatch", label)
	}
	if got := binary.BigEndian.Uint64(raw[8:16]); got != segno {
		return nil, corruptf("%s: names segment %d", label, got)
	}
	if [12]byte(raw[28:40]) != computerHash(computerID) {
		return nil, corruptf("%s: belongs to a different computer", label)
	}
	segSize := binary.BigEndian.Uint64(raw[16:24])
	count := binary.BigEndian.Uint32(raw[24:28])
	if segSize < segHeaderSize || segSize > maxSegmentSize {
		return nil, corruptf("%s: implausible segment size %d", label, segSize)
	}
	if want := idxFixedSize + idxEntrySize*int(count) + idxTrailerSize; len(raw) != want {
		return nil, corruptf("%s: size %d does not match %d entries", label, len(raw), count)
	}
	s := &sealedSeg{segno: segno, segPath: segPath, idxPath: idxPath, segSize: int64(segSize), count: count, computer: computerID}
	var prev uint32
	for i := 0; i < 256; i++ {
		v := binary.BigEndian.Uint32(raw[40+4*i : 44+4*i])
		if v < prev || v > count {
			return nil, corruptf("%s: fanout not monotonic at %d", label, i)
		}
		s.fanout[i], prev = v, v
	}
	if prev != count {
		return nil, corruptf("%s: fanout total %d is not the entry count %d", label, prev, count)
	}
	// The whole index is in memory already: check that the entries are
	// strictly sorted and that the fanout describes them. A lying fanout
	// would otherwise hide a record as "absent".
	var bucket [256]uint32
	var last *idxEntry
	for i := 0; i < int(count); i++ {
		e := decodeIdxEntry(raw[idxFixedSize+idxEntrySize*i:])
		if !e.ns.valid() {
			return nil, corruptf("%s: entry %d has an unknown namespace", label, i)
		}
		if last != nil {
			if c := bytes.Compare(last.digest[:], e.digest[:]); c > 0 || (c == 0 && last.ns >= e.ns) {
				return nil, corruptf("%s: entries not strictly sorted at %d", label, i)
			}
		}
		cp := e
		last = &cp
		bucket[e.digest[0]]++
	}
	var cum uint32
	for i := 0; i < 256; i++ {
		cum += bucket[i]
		if s.fanout[i] != cum {
			return nil, corruptf("%s: fanout disagrees with the entries at bucket %d", label, i)
		}
	}
	info, err := os.Stat(segPath)
	if err != nil {
		return nil, fmt.Errorf("tapestore: stat segment %s: %w", segPath, err)
	}
	if info.Size() != int64(segSize) {
		return nil, corruptf("%s: segment is %d bytes, index sealed %d", label, info.Size(), segSize)
	}
	f, err := os.Open(segPath)
	if err != nil {
		return nil, fmt.Errorf("tapestore: open segment %s: %w", segPath, err)
	}
	defer f.Close()
	head := make([]byte, segHeaderSize)
	if _, err := f.ReadAt(head, 0); err != nil {
		return nil, corruptf("%s: read segment header: %v", label, err)
	}
	if err := checkSegHeader(head, segno, computerID); err != nil {
		return nil, err
	}
	return s, nil
}

// read looks (ns, digest) up in the index and returns the verified record.
func (s *sealedSeg) read(ns NS, digest [32]byte) ([]byte, bool, error) {
	b0 := int(digest[0])
	lo := uint32(0)
	if b0 > 0 {
		lo = s.fanout[b0-1]
	}
	hi := s.fanout[b0]
	if lo == hi {
		return nil, false, nil
	}
	idx, err := os.Open(s.idxPath)
	if err != nil {
		return nil, false, fmt.Errorf("tapestore: open index %s: %w", s.idxPath, err)
	}
	defer idx.Close()
	buf := make([]byte, int(hi-lo)*idxEntrySize)
	if _, err := idx.ReadAt(buf, int64(idxFixedSize)+int64(lo)*idxEntrySize); err != nil {
		return nil, false, corruptf("index %d: read entries: %v", s.segno, err)
	}
	var found *idxEntry
	var prev *idxEntry
	for i := 0; i < len(buf); i += idxEntrySize {
		e := decodeIdxEntry(buf[i : i+idxEntrySize])
		if int(e.digest[0]) != b0 || !e.ns.valid() {
			return nil, false, corruptf("index %d: entry outside its fanout bucket or with an unknown namespace", s.segno)
		}
		if prev != nil {
			if c := bytes.Compare(prev.digest[:], e.digest[:]); c > 0 || (c == 0 && prev.ns >= e.ns) {
				return nil, false, corruptf("index %d: entries not strictly sorted", s.segno)
			}
		}
		cp := e
		prev = &cp
		if e.digest == digest && e.ns == ns {
			found = &cp
		}
	}
	if found == nil {
		return nil, false, nil
	}
	seg, err := os.Open(s.segPath)
	if err != nil {
		return nil, false, fmt.Errorf("tapestore: open segment %s: %w", s.segPath, err)
	}
	defer seg.Close()
	body, err := readRecordAt(seg, fmt.Sprintf("computer %s segment %d", s.computer, s.segno), int64(found.off), s.segSize, ns, digest, int64(found.length))
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

// --- open (unsealed) segments --------------------------------------------------

type openKey struct {
	ns     NS
	digest [32]byte
}

type openLoc struct {
	off    int64
	length uint32
}

type openSeg struct {
	mu      sync.Mutex
	path    string
	segno   uint64
	headOK  bool
	scanned int64 // bytes known to hold only whole, crc-valid records
	entries map[openKey]openLoc
}

// retireSealed drops the open-scan state of every sealed segment in a
// directory snapshot, under one lock.
func (r *Reader) retireSealed(dir string, segs []segFiles) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range segs {
		if f.hasIdx {
			delete(r.open, filepath.Join(dir, SegmentFileName(f.segno)))
		}
	}
}

func (r *Reader) openFor(path string, segno uint64) *openSeg {
	r.mu.Lock()
	defer r.mu.Unlock()
	o := r.open[path]
	if o == nil {
		o = &openSeg{path: path, segno: segno, entries: map[openKey]openLoc{}}
		r.open[path] = o
	}
	return o
}

// read refreshes the scan of an unsealed segment and looks the digest up.
// isLast says whether this is the highest-numbered segment, the only place a
// torn tail is tolerated.
func (o *openSeg) read(computerID string, isLast bool, ns NS, digest [32]byte) ([]byte, bool, error) {
	f, err := os.Open(o.path)
	if err != nil {
		return nil, false, fmt.Errorf("tapestore: open segment %s: %w", o.path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("tapestore: stat segment %s: %w", o.path, err)
	}
	size := info.Size()
	label := fmt.Sprintf("computer %s segment %d", computerID, o.segno)

	o.mu.Lock()
	err = o.refresh(f, size, label, computerID, isLast)
	loc, ok := o.entries[openKey{ns, digest}]
	o.mu.Unlock()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	body, err := readRecordAt(f, label, loc.off, size, ns, digest, int64(loc.length))
	if err != nil {
		return nil, false, err
	}
	return body, true, nil
}

// refresh scans the records between the last known-good end and size.
// Caller holds o.mu.
func (o *openSeg) refresh(f io.ReaderAt, size int64, label, computerID string, isLast bool) error {
	if size < o.scanned {
		// The file shrank (a recovering writer cut its torn tail before sealing
		// it). Start over.
		o.headOK, o.scanned, o.entries = false, 0, map[openKey]openLoc{}
	}
	if !o.headOK {
		if size < segHeaderSize {
			if isLast {
				return nil // creation torn before the header landed: empty
			}
			return corruptf("%s: unsealed segment truncated inside its header", label)
		}
		head := make([]byte, segHeaderSize)
		if _, err := f.ReadAt(head, 0); err != nil {
			return corruptf("%s: read header: %v", label, err)
		}
		if err := checkSegHeader(head, o.segno, computerID); err != nil {
			return err
		}
		o.headOK, o.scanned = true, segHeaderSize
	}
	pos := o.scanned
	for pos < size {
		h, next, bad, err := scanRecord(f, pos, size)
		if err != nil {
			return err
		}
		if bad != "" {
			if !isLast {
				return corruptf("%s: bad record at %d in a non-final unsealed segment: %s", label, pos, bad)
			}
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
			o.scanned = pos // torn tail: absent; a recovering writer cuts it and seals the segment
			return nil
		}
		k := openKey{h.ns, h.digest}
		if _, dup := o.entries[k]; !dup {
			o.entries[k] = openLoc{off: pos, length: h.length}
		}
		pos = next
	}
	o.scanned = pos
	return nil
}

// scanRecord checks the record at pos without hashing its body: header
// fields, bounds and crc. bad is non-empty when no whole, crc-valid record
// starts at pos; err is a real I/O failure.
func scanRecord(f io.ReaderAt, pos, size int64) (h recHeader, next int64, bad string, err error) {
	if pos+recHeaderSize > size {
		return h, 0, "record header cut short", nil
	}
	head := make([]byte, recHeaderSize)
	if err := readFull(f, head, pos); err != nil {
		return h, 0, "", err
	}
	h, perr := parseRecHeaderLoose(head)
	if perr != nil {
		return h, 0, perr.Error(), nil
	}
	next = pos + recHeaderSize + int64(h.length)
	if next > size {
		return h, 0, "record body cut short", nil
	}
	body := make([]byte, h.length)
	if err := readFull(f, body, pos+recHeaderSize); err != nil {
		return h, 0, "", err
	}
	if recordCRC(head[4:42], body) != h.crc {
		return h, 0, "record crc mismatch", nil
	}
	// The record is whole and intact, so it is not a torn write. If this
	// reader does not understand it, fail closed rather than skip it.
	if (head[4] != recVersion && head[4] != recVersionZstd) || !h.ns.valid() {
		return h, 0, "", unsupportedf("record at %d is intact but unsupported (version %d, namespace %d)", pos, head[4], head[5])
	}
	return h, next, "", nil
}

// readFull reads len(b) bytes at off. A short read at the end of the file
// is reported as io.ErrUnexpectedEOF so callers can tell it from a device
// error.
func readFull(f io.ReaderAt, b []byte, off int64) error {
	if len(b) == 0 {
		return nil
	}
	n, err := f.ReadAt(b, off)
	if n == len(b) {
		return nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}

// validRecordAfter reports whether any whole, crc-valid record starts after
// pos. Used to tell a torn tail (nothing valid follows) from corruption in
// the middle of a segment.
func validRecordAfter(f io.ReaderAt, pos, size int64) (bool, error) {
	if size-pos-1 <= 0 {
		return false, nil
	}
	rest := make([]byte, size-pos-1)
	if err := readFull(f, rest, pos+1); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	for i := 0; ; {
		j := bytes.Index(rest[i:], recMagic[:])
		if j < 0 {
			return false, nil
		}
		i += j
		if len(rest)-i >= recHeaderSize {
			if h, err := parseRecHeaderLoose(rest[i : i+recHeaderSize]); err == nil {
				end := i + recHeaderSize + int(h.length)
				if end <= len(rest) && recordCRC(rest[i+4:i+42], rest[i+recHeaderSize:end]) == h.crc {
					return true, nil
				}
			}
		}
		i++
	}
}
