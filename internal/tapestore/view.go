package tapestore

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
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
}

// OpenView lists the computer's segments and validates every sealed index
// (header, crc, order, fanout, segment size and header). A computer with no
// segment directory has an empty view. A damaged index, an index without its
// segment, or an unsealed segment below the highest one (a state the writer
// refuses to extend) is ErrCorrupt.
func (r *Reader) OpenView(computerID string) (*SegmentView, error) {
	dir, segs, err := r.list(computerID)
	if err != nil {
		return nil, err
	}
	v := &SegmentView{r: r, computer: computerID, dir: dir, segs: segs, sealed: map[uint64]*sealedSeg{}}
	r.retireSealed(dir, segs)
	for i, f := range segs {
		if f.hasIdx {
			s, err := r.loadSealed(dir, computerID, f.segno)
			if err != nil {
				return nil, err
			}
			v.sealed[f.segno] = s
			continue
		}
		if i != len(segs)-1 {
			return nil, corruptf("computer %s: segment %d is unsealed below the highest segment %d", computerID, f.segno, segs[len(segs)-1].segno)
		}
	}
	return v, nil
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
