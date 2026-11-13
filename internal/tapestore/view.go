package tapestore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// SegLoc locates one record of one segment of a computer.
type SegLoc struct {
	Segno  uint64
	Sealed bool
	NS     NS
	Digest [32]byte
	Off    int64
	Length uint32
}

// SegmentView is a listing of one computer's segments for bulk work: the
// repack joins millions of legacy files against the store, and a point lookup
// per file lists the directory and opens every index. A view lists once and
// answers by the first byte of the digest: Bucket(b) reads only the byte-b
// range of each sealed index (through its fanout table) and the byte-b
// entries of the open segment, so a join over 256 buckets holds about 1/256
// of the records at a time. Read verifies a located record completely.
//
// A view never writes. Bucket and SegmentLocs use the file list as of OpenView:
// a segment created after it is not seen, and the open segment is rescanned
// by every Bucket call, so records appended later are. ReadObject re-lists
// when a read finds no intact copy (see there). It is not safe for concurrent
// use.
type SegmentView struct {
	r        *Reader
	computer string
	dir      string
	segs     []segFiles
	sealed   map[uint64]*sealedSeg

	hint   int    // index into segs of the last ReadObject hit
	probes uint64 // segment lookups made by ReadObject

	reloads, fallbacks uint64
}

// OpenView lists the computer's segments and validates every sealed index
// (header, crc, order, fanout, segment size and header). A computer with no
// segment directory has an empty view. A damaged index, an index without its
// segment, or an unsealed segment below the highest one (a state the writer
// refuses to extend) is ErrCorrupt.
func (r *Reader) OpenView(computerID string) (*SegmentView, error) {
	v := &SegmentView{r: r, computer: computerID}
	if err := v.load(); err != nil {
		return nil, err
	}
	return v, nil
}

// load lists the computer's directory and validates every sealed index. The
// reader's sealed-index cache makes a reload cost the listing plus the
// indexes not seen before.
func (v *SegmentView) load() error {
	r, computerID := v.r, v.computer
	dir, segs, err := r.list(computerID)
	if err != nil {
		return err
	}
	sealed := map[uint64]*sealedSeg{}
	r.retireSealed(dir, segs)
	for i, f := range segs {
		if f.hasIdx {
			s, err := r.loadSealed(dir, computerID, f.segno)
			if err != nil {
				return err
			}
			sealed[f.segno] = s
			continue
		}
		if i != len(segs)-1 {
			return corruptf("computer %s: segment %d is unsealed below the highest segment %d", computerID, f.segno, segs[len(segs)-1].segno)
		}
	}
	v.dir, v.segs, v.sealed = dir, segs, sealed
	if v.hint >= len(segs) {
		v.hint = 0
	}
	return nil
}

// Probes is the number of segment lookups ReadObject has made: one per
// segment index consulted or open segment rescanned.
func (v *SegmentView) Probes() uint64 { return v.probes }

// Reloads is the number of times a read found the directory changed and
// reloaded the view; Fallbacks the number of reads the per-call Reader path
// answered because the view could not (a reload failed, or a segment file the
// view listed was gone).
func (v *SegmentView) Reloads() uint64   { return v.reloads }
func (v *SegmentView) Fallbacks() uint64 { return v.fallbacks }

// ReadObject is Reader.Read for one object of this computer: the same
// segment-then-legacy order, digest check, counters and legacy fallback on a
// segment error, but the segments come from the view's listing and validated
// indexes, probed from the segment of the last hit and the next one, then
// every segment newest first. Events are appended in order, so a walk in
// order finds the next one in the same or the next segment.
//
// The outcome never depends on the hint or the walk order. For a digest, the
// view returns the verified bytes if any segment holds an intact copy;
// otherwise the error of the newest damaged copy (ErrCorrupt, as
// Reader.Read); otherwise absent, and then the legacy file as Reader.Read
// does. Reader.Read decides by the newest copy alone, which would need a
// probe of every newer segment on every read. The two differ in one case: a
// digest stored twice, the newer copy damaged and an older copy intact.
// Reader.Read reports the damage; the view returns the intact copy, checked
// against the digest and the record crc, so the bytes are the object's.
// Damage with no intact copy anywhere still stops the walk.
//
// When no intact copy is found, the directory is listed again; if it changed
// (a segment created or sealed since the view loaded) the view reloads and
// probes again, so a walk that runs while the tape grows sees what
// Reader.Read would. A reload that fails, or an error that is not damage (a
// segment or index file the view listed is gone), is answered by
// Reader.readSegment on a fresh listing, exactly as Reader.Read answers. A
// view holds no files open and needs no Close. It is not safe for concurrent
// use.
func (v *SegmentView) ReadObject(ns NS, digest string) ([]byte, Source, error) {
	if !ns.valid() {
		return nil, 0, fmt.Errorf("tapestore: invalid namespace %d", uint8(ns))
	}
	if !ValidDigest(digest) {
		return nil, 0, fmt.Errorf("tapestore: invalid digest %q", digest)
	}
	return v.r.readWith(v.computer, ns, digest, v.lookup)
}

// isDamage reports a segment error that is a verdict on the store (as
// opposed to a path error from a stale listing).
func isDamage(err error) bool {
	return errors.Is(err, ErrCorrupt) || errors.Is(err, ErrUnsupportedVersion) || errors.Is(err, ErrDigestMismatch)
}

func (v *SegmentView) lookup(ns NS, digest string) ([]byte, bool, error) {
	var want [32]byte
	if err := decodeHex32(&want, digest); err != nil {
		return nil, false, err
	}
	for pass := 0; pass < 2; pass++ {
		body, found, err := v.search(ns, want)
		if found {
			return body, true, nil
		}
		if err != nil && !isDamage(err) {
			return v.fallback(ns, digest)
		}
		if pass == 1 {
			return nil, false, err
		}
		// No intact copy: has the tape grown since the view loaded?
		_, now, lerr := v.r.list(v.computer)
		if lerr != nil {
			return nil, false, lerr
		}
		if sameSegFiles(now, v.segs) {
			return nil, false, err
		}
		if v.load() != nil {
			return v.fallback(ns, digest)
		}
		v.reloads++
	}
	return nil, false, nil
}

// fallback answers one read with the per-call Reader path, and refreshes the
// view if it can.
func (v *SegmentView) fallback(ns NS, digest string) ([]byte, bool, error) {
	v.fallbacks++
	_ = v.load()
	return v.r.readSegment(v.computer, ns, digest)
}

// search probes the hinted segment and the next, then every segment newest
// first. An intact copy is returned at once. Damage is remembered and the
// search goes on; with no intact copy it returns the newest damaged
// segment's error. A path error is returned at once.
func (v *SegmentView) search(ns NS, want [32]byte) ([]byte, bool, error) {
	var damage error
	damageAt := -1
	try := func(i int) ([]byte, bool, error, bool) {
		body, found, err := v.probe(i, ns, want)
		switch {
		case found:
			v.hint = i
			return body, true, nil, true
		case err != nil && !isDamage(err):
			return nil, false, err, true
		case err != nil && i > damageAt:
			damage, damageAt = err, i
		}
		return nil, false, nil, false
	}
	first := v.hint
	for i := first; i <= first+1; i++ {
		if i >= len(v.segs) {
			continue
		}
		if body, found, err, done := try(i); done {
			return body, found, err
		}
	}
	for i := len(v.segs) - 1; i >= 0; i-- {
		if i == first || i == first+1 {
			continue
		}
		if body, found, err, done := try(i); done {
			return body, found, err
		}
	}
	return nil, false, damage
}

func sameSegFiles(a, b []segFiles) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// probe looks (ns, want) up in segment i of the view.
func (v *SegmentView) probe(i int, ns NS, want [32]byte) ([]byte, bool, error) {
	v.probes++
	f := v.segs[i]
	if f.hasIdx {
		return v.sealed[f.segno].read(ns, want)
	}
	o := v.r.openFor(filepath.Join(v.dir, SegmentFileName(f.segno)), f.segno)
	return o.read(v.computer, i == len(v.segs)-1, ns, want)
}

// Segments is the number of segments listed.
func (v *SegmentView) Segments() int { return len(v.segs) }

// Bucket returns the location of every record whose digest begins with byte
// b, in every segment, newest segment last. A record present in two segments
// is listed twice. An error in any segment is returned: the computer's store
// cannot be vouched for.
func (v *SegmentView) Bucket(b byte) ([]SegLoc, error) {
	var out []SegLoc
	for i, f := range v.segs {
		if f.hasIdx {
			locs, err := v.sealed[f.segno].bucket(b)
			if err != nil {
				return nil, err
			}
			out = append(out, locs...)
			continue
		}
		path := filepath.Join(v.dir, SegmentFileName(f.segno))
		locs, err := v.r.openFor(path, f.segno).bucket(v.computer, i == len(v.segs)-1, b)
		if err != nil {
			return nil, err
		}
		out = append(out, locs...)
	}
	return out, nil
}

// SegmentLocs returns the location of every record of segment i (0-based, in
// the order of Segments), sorted by file offset, so a caller can read the
// segment front to back instead of jumping by digest. It holds one segment's
// locations only. Error semantics are those of Bucket: any error is returned.
func (v *SegmentView) SegmentLocs(i int) ([]SegLoc, error) {
	if i < 0 || i >= len(v.segs) {
		return nil, fmt.Errorf("tapestore: segment index %d out of range (%d segments)", i, len(v.segs))
	}
	f := v.segs[i]
	var out []SegLoc
	if f.hasIdx {
		s := v.sealed[f.segno]
		for b := 0; b < 256; b++ {
			locs, err := s.bucket(byte(b))
			if err != nil {
				return nil, err
			}
			out = append(out, locs...)
		}
	} else {
		path := filepath.Join(v.dir, SegmentFileName(f.segno))
		o := v.r.openFor(path, f.segno)
		for b := 0; b < 256; b++ {
			locs, err := o.bucket(v.computer, i == len(v.segs)-1, byte(b))
			if err != nil {
				return nil, err
			}
			out = append(out, locs...)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Off < out[b].Off })
	return out, nil
}

// Read returns the record at loc after the full check: header fields,
// length, crc and SHA-256 of the body against the digest in loc.
func (v *SegmentView) Read(loc SegLoc) ([]byte, error) {
	label := fmt.Sprintf("computer %s segment %d", v.computer, loc.Segno)
	if s := v.sealed[loc.Segno]; s != nil {
		f, err := os.Open(s.segPath)
		if err != nil {
			return nil, fmt.Errorf("tapestore: open segment %s: %w", s.segPath, err)
		}
		defer f.Close()
		return readRecordAt(f, label, loc.Off, s.segSize, loc.NS, loc.Digest, int64(loc.Length))
	}
	path := filepath.Join(v.dir, SegmentFileName(loc.Segno))
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("tapestore: open segment %s: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("tapestore: stat segment %s: %w", path, err)
	}
	return readRecordAt(f, label, loc.Off, info.Size(), loc.NS, loc.Digest, int64(loc.Length))
}

// bucket reads the byte-b range of a sealed index.
func (s *sealedSeg) bucket(b byte) ([]SegLoc, error) {
	lo := uint32(0)
	if b > 0 {
		lo = s.fanout[b-1]
	}
	hi := s.fanout[b]
	if lo == hi {
		return nil, nil
	}
	idx, err := os.Open(s.idxPath)
	if err != nil {
		return nil, fmt.Errorf("tapestore: open index %s: %w", s.idxPath, err)
	}
	defer idx.Close()
	buf := make([]byte, int(hi-lo)*idxEntrySize)
	if _, err := idx.ReadAt(buf, int64(idxFixedSize)+int64(lo)*idxEntrySize); err != nil {
		return nil, corruptf("index %d: read entries: %v", s.segno, err)
	}
	out := make([]SegLoc, 0, hi-lo)
	var prev *idxEntry
	for i := 0; i < len(buf); i += idxEntrySize {
		e := decodeIdxEntry(buf[i : i+idxEntrySize])
		if e.digest[0] != b || !e.ns.valid() {
			return nil, corruptf("index %d: entry outside its fanout bucket or with an unknown namespace", s.segno)
		}
		if prev != nil {
			if c := bytes.Compare(prev.digest[:], e.digest[:]); c > 0 || (c == 0 && prev.ns >= e.ns) {
				return nil, corruptf("index %d: entries not strictly sorted", s.segno)
			}
		}
		cp := e
		prev = &cp
		out = append(out, SegLoc{Segno: s.segno, Sealed: true, NS: e.ns, Digest: e.digest, Off: int64(e.off), Length: e.length})
	}
	return out, nil
}

// bucket refreshes the scan of an unsealed segment and lists its byte-b
// records.
func (o *openSeg) bucket(computerID string, isLast bool, b byte) ([]SegLoc, error) {
	f, err := os.Open(o.path)
	if err != nil {
		return nil, fmt.Errorf("tapestore: open segment %s: %w", o.path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("tapestore: stat segment %s: %w", o.path, err)
	}
	label := fmt.Sprintf("computer %s segment %d", computerID, o.segno)
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.refresh(f, info.Size(), label, computerID, isLast); err != nil {
		return nil, err
	}
	var out []SegLoc
	for k, loc := range o.entries {
		if k.digest[0] == b {
			out = append(out, SegLoc{Segno: o.segno, NS: k.ns, Digest: k.digest, Off: loc.off, Length: loc.length})
		}
	}
	return out, nil
}
