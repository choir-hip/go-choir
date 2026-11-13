package tapestore

import (
	"bytes"
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
// A view never writes. It is a snapshot of the file list: a segment created
// after OpenView is not seen, and the open segment is rescanned by every
// Bucket call, so records appended later are. It is not safe for concurrent
// use.
type SegmentView struct {
	r        *Reader
	computer string
	dir      string
	segs     []segFiles
	sealed   map[uint64]*sealedSeg

	hint   int    // index into segs of the last ReadObject hit
	probes uint64 // segment lookups made by ReadObject
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

// ReadObject is Reader.Read for one object of this computer: the same
// segment-then-legacy order, digest check, counters and legacy fallback on a
// segment error, but the segments come from the view's listing and validated
// indexes, probed from the segment of the last hit. Events are appended in
// order, so a walk in order finds the next one in the same or the next
// segment. A miss there falls back to every segment newest first, so an
// object any segment holds is found. Where a digest sits in two segments the
// copy found first is returned (both are checked against the digest and the
// record crc), so a damaged copy in a newer segment than the hit is not
// looked at. When every segment misses, the directory is listed again and,
// if it changed (a segment created or sealed since the view loaded), the
// view reloads and probes again, so a walk that runs while the tape grows
// sees what Reader.Read would. If that reload fails, the per-call Reader
// path answers. A view is not safe for concurrent use.
func (v *SegmentView) ReadObject(ns NS, digest string) ([]byte, Source, error) {
	if !ns.valid() {
		return nil, 0, fmt.Errorf("tapestore: invalid namespace %d", uint8(ns))
	}
	if !ValidDigest(digest) {
		return nil, 0, fmt.Errorf("tapestore: invalid digest %q", digest)
	}
	return v.r.readWith(v.computer, ns, digest, v.lookup)
}

func (v *SegmentView) lookup(ns NS, digest string) ([]byte, bool, error) {
	var want [32]byte
	if err := decodeHex32(&want, digest); err != nil {
		return nil, false, err
	}
	for pass := 0; pass < 2; pass++ {
		first, second := v.hint, v.hint+1
		for _, i := range [2]int{first, second} {
			if i >= len(v.segs) {
				continue
			}
			if body, found, err := v.probe(i, ns, want); err != nil || found {
				if found {
					v.hint = i
				}
				return body, found, err
			}
		}
		for i := len(v.segs) - 1; i >= 0; i-- {
			if i == first || i == second {
				continue
			}
			if body, found, err := v.probe(i, ns, want); err != nil || found {
				if found {
					v.hint = i
				}
				return body, found, err
			}
		}
		if pass == 1 {
			break
		}
		// Every segment missed: has the tape grown since the view loaded?
		_, now, err := v.r.list(v.computer)
		if err != nil || sameSegFiles(now, v.segs) {
			return nil, false, err
		}
		if err := v.load(); err != nil {
			return v.r.readSegment(v.computer, ns, digest)
		}
	}
	return nil, false, nil
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
