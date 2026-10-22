package tracelog

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SegmentInfo describes one segment for observability and retention.
type SegmentInfo struct {
	Segno      uint64
	Sealed     bool
	Records    int // lines and boundaries
	Lines      int
	Bytes      int64 // the .tlog file
	IndexBytes int64 // the .tidx file, if sealed
	ValidEnd   int64
	CreatedAt  time.Time
	SealedAt   time.Time
	MinStream  int64
	MaxStream  int64
	MinTime    time.Time
}

// Floor is the oldest line the log still holds, so a reader can tell "older
// than retention" from "missing".
type Floor struct {
	Empty     bool
	Segment   uint64
	Time      time.Time
	StreamSeq int64
}

// maxLoadedIndexes bounds how many sealed segments keep their entries in
// memory at once. Summaries of all sealed segments are kept.
const maxLoadedIndexes = 4

// Reader reads a trace-log directory. A Reader made by Log.Reader also sees
// the open segment through the Log's in-memory index; one made by OpenReader
// reads the files alone, and scans an unsealed segment's whole records.
// Segments deleted by retention while a read is under way are skipped, never
// an error, and a file already open is read to its end.
type Reader struct {
	dir  string
	live *Log

	mu     sync.Mutex
	states map[uint64]*segState
	loaded []uint64

	rebuilds atomic.Int64
	skipped  atomic.Int64
}

// OpenReader returns a reader over dir. It touches nothing until used.
func OpenReader(dir string) *Reader {
	return &Reader{dir: filepath.Clean(dir), states: map[uint64]*segState{}}
}

// Reader returns the log's reader, which sees the open segment's index.
func (l *Log) Reader() *Reader {
	l.readerOnce.Do(func() {
		l.reader = &Reader{dir: l.dir, live: l, states: map[uint64]*segState{}}
	})
	return l.reader
}

// IndexRebuilds counts sealed segments whose .tidx was missing or bad and
// whose index was rebuilt by scanning the .tlog.
func (r *Reader) IndexRebuilds() int64 { return r.rebuilds.Load() }

// Skipped counts reads skipped because a file had gone or a record failed its
// checksum.
func (r *Reader) Skipped() int64 { return r.skipped.Load() }

// segFile is a segment as the directory shows it.
type segFile struct {
	segno    uint64
	tlogSize int64
	tlogMod  time.Time
	hasIdx   bool
	idxSize  int64
}

type badFile struct {
	name string
	size int64
	mod  time.Time
}

type listing struct {
	segs    []segFile
	orphans []segFile // a .tidx without its .tlog
	bad     []badFile
}

func (r *Reader) list() (listing, error) {
	var out listing
	ents, err := os.ReadDir(r.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return out, nil
		}
		return out, err
	}
	byNo := map[uint64]*segFile{}
	get := func(n uint64) *segFile {
		if byNo[n] == nil {
			byNo[n] = &segFile{segno: n}
		}
		return byNo[n]
	}
	hasTlog := map[uint64]bool{}
	for _, e := range ents {
		name := e.Name()
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // vanished between the listing and the stat
		}
		if n, ok := parseSegName(name, ".tlog"); ok {
			f := get(n)
			f.tlogSize, f.tlogMod = info.Size(), info.ModTime()
			hasTlog[n] = true
		} else if n, ok := parseSegName(name, ".tidx"); ok {
			f := get(n)
			f.hasIdx, f.idxSize = true, info.Size()
		} else if strings.HasSuffix(name, ".tlog.bad") {
			out.bad = append(out.bad, badFile{name: name, size: info.Size(), mod: info.ModTime()})
		}
	}
	for n, f := range byNo {
		if hasTlog[n] {
			out.segs = append(out.segs, *f)
		} else {
			out.orphans = append(out.orphans, *f)
		}
	}
	sort.Slice(out.segs, func(i, j int) bool { return out.segs[i].segno < out.segs[j].segno })
	sort.Slice(out.orphans, func(i, j int) bool { return out.orphans[i].segno < out.orphans[j].segno })
	r.mu.Lock()
	for n := range r.states {
		if !hasTlog[n] {
			delete(r.states, n)
		}
	}
	r.mu.Unlock()
	return out, nil
}

// segState is what the reader remembers of one segment.
type segState struct {
	mu      sync.Mutex
	sealed  bool
	sum     *Summary
	idx     *segIdx     // sealed: loaded entries, or nil
	scan    *scanResult // unsealed: the incremental scan
	rebuilt bool        // sealed, but indexed by scanning because the .tidx was bad
}

func (r *Reader) state(f segFile) *segState {
	r.mu.Lock()
	defer r.mu.Unlock()
	st := r.states[f.segno]
	if st == nil || st.sealed != f.hasIdx {
		st = &segState{sealed: f.hasIdx}
		r.states[f.segno] = st
	}
	return st
}

// touch records that a sealed segment's entries were just used, and drops the
// entries of the least recently used ones beyond the bound. It is called with
// no segment lock held.
func (r *Reader) touch(segno uint64) {
	r.mu.Lock()
	for i, n := range r.loaded {
		if n == segno {
			r.loaded = append(r.loaded[:i], r.loaded[i+1:]...)
			break
		}
	}
	r.loaded = append(r.loaded, segno)
	var evict []*segState
	for len(r.loaded) > maxLoadedIndexes {
		if st := r.states[r.loaded[0]]; st != nil {
			evict = append(evict, st)
		}
		r.loaded = r.loaded[1:]
	}
	r.mu.Unlock()
	for _, st := range evict {
		st.mu.Lock()
		st.idx = nil
		st.mu.Unlock()
	}
}

func (r *Reader) tlogPath(segno uint64) string { return filepath.Join(r.dir, segmentName(segno)) }

// loadSealed fills st.sum (and st.idx when wantIdx) from the .tidx, or by
// scanning the .tlog when the index is missing or bad. st.mu is held.
func (r *Reader) loadSealed(f segFile, st *segState, wantIdx bool) error {
	if st.sum != nil && (!wantIdx || st.idx != nil) {
		return nil
	}
	path := filepath.Join(r.dir, indexName(f.segno))
	if !st.rebuilt {
		if wantIdx {
			b, err := os.ReadFile(path)
			if err == nil {
				h, sum, x, derr := decodeIndex(b)
				if derr == nil && h.segno == f.segno && h.tlogSize <= f.tlogSize {
					st.sum, st.idx = sum, x
					return nil
				}
			} else if errors.Is(err, fs.ErrNotExist) {
				return err
			}
		} else {
			fh, err := os.Open(path)
			if err == nil {
				sum, h, derr := readSummary(fh)
				fh.Close()
				if derr == nil && h.segno == f.segno && h.tlogSize <= f.tlogSize {
					st.sum = sum
					return nil
				}
			} else if errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	// The index is missing or bad: index the segment by scanning it.
	fh, err := openRegular(r.tlogPath(f.segno), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer fh.Close()
	res, err := scanSegment(fh, f.segno)
	if err != nil {
		return err
	}
	if !st.rebuilt {
		st.rebuilt = true
		r.rebuilds.Add(1)
	}
	st.idx = res.idx
	st.sum = res.idx.summarize(res.validEnd, res.created, f.tlogMod)
	return nil
}

// readSummary reads the header and summary from the front of a .tidx without
// reading its entries.
func readSummary(fh *os.File) (*Summary, idxHeader, error) {
	front := make([]byte, idxHeaderSize)
	if _, err := fh.ReadAt(front, 0); err != nil {
		return nil, idxHeader{}, corruptf("index header: %v", err)
	}
	h, err := parseIdxHeader(front)
	if err != nil {
		return nil, h, err
	}
	if h.summaryLen > 1<<28 {
		return nil, h, corruptf("index summary of %d bytes", h.summaryLen)
	}
	front = make([]byte, idxHeaderSize+int(h.summaryLen)+4)
	if _, err := fh.ReadAt(front, 0); err != nil {
		return nil, h, corruptf("index summary: %v", err)
	}
	h, sum, err := decodeSummary(front)
	return sum, h, err
}

// loadUnsealed brings the incremental scan of an unsealed segment up to the
// file's current size. st.mu is held.
func (r *Reader) loadUnsealed(f segFile, st *segState) error {
	fh, err := openRegular(r.tlogPath(f.segno), os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil {
		return err
	}
	if st.scan == nil || info.Size() < st.scan.validEnd {
		res, err := scanSegment(fh, f.segno)
		if err != nil {
			return err
		}
		st.scan = &res
		return nil
	}
	if st.scan.class != headerOK || info.Size() == st.scan.size {
		return nil
	}
	st.scan.size = info.Size()
	end, err := resume(fh, st.scan, st.scan.validEnd)
	if err != nil {
		return err
	}
	st.scan.validEnd = end
	return nil
}

// withIdx runs fn on the segment's index. The open segment of a live Log is
// read from the Log's memory under its mutex; any other is loaded from its
// .tidx or scanned. fn must be quick and must not call the reader.
func (r *Reader) withIdx(f segFile, fn func(x *segIdx)) error {
	if r.live != nil && r.live.withOpen(f.segno, func(s *openSeg) { fn(s.idx) }) {
		return nil
	}
	st := r.state(f)
	if st.sealed {
		err := func() error {
			st.mu.Lock()
			defer st.mu.Unlock()
			if err := r.loadSealed(f, st, true); err != nil {
				return err
			}
			fn(st.idx)
			return nil
		}()
		if err == nil {
			r.touch(f.segno)
		}
		return err
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := r.loadUnsealed(f, st); err != nil {
		return err
	}
	fn(st.scan.idx)
	return nil
}

// summary returns a sealed segment's summary, or nil when it has no usable one.
func (r *Reader) summary(f segFile) *Summary {
	if !f.hasIdx {
		return nil
	}
	st := r.state(f)
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := r.loadSealed(f, st, false); err != nil {
		return nil
	}
	return st.sum
}

// withOpen runs fn on the open segment if it is segno, under the Log's mutex.
func (l *Log) withOpen(segno uint64, fn func(s *openSeg)) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cur == nil || l.cur.segno != segno {
		return false
	}
	fn(l.cur)
	return true
}

// cand is a line chosen by an index lookup, before it is read.
type cand struct {
	segno  uint64
	off    int64
	n      uint32
	seq    int64
	stream int64
}

// load reads the chosen records, in order. A segment gone or a record that
// fails its checksum is skipped and counted.
func (r *Reader) load(cands []cand) ([]Record, error) {
	files := map[uint64]*os.File{}
	gone := map[uint64]bool{}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()
	out := make([]Record, 0, len(cands))
	for _, c := range cands {
		if gone[c.segno] {
			r.skipped.Add(1)
			continue
		}
		f := files[c.segno]
		if f == nil {
			var err error
			f, err = openRegular(r.tlogPath(c.segno), os.O_RDONLY, 0)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					gone[c.segno] = true
					r.skipped.Add(1)
					continue
				}
				return out, err
			}
			files[c.segno] = f
		}
		typ, body, err := readRecordAt(f, c.off, c.n)
		if err != nil || typ != typeLine {
			r.skipped.Add(1)
			continue
		}
		m, line, err := decodeLine(body)
		if err != nil {
			r.skipped.Add(1)
			continue
		}
		out = append(out, Record{Meta: m, Line: line, Segment: c.segno, Offset: c.off})
	}
	return out, nil
}

func goneErr(err error) bool { return errors.Is(err, fs.ErrNotExist) }

func sortCands(c []cand, bySeq bool) {
	sort.Slice(c, func(i, j int) bool {
		if bySeq {
			if c[i].seq != c[j].seq {
				return c[i].seq < c[j].seq
			}
		}
		if c[i].stream != c[j].stream {
			return c[i].stream < c[j].stream
		}
		return c[i].off < c[j].off
	})
}

// ByRun returns the run's lines with seq > afterSeq, ascending in seq, at most
// limit (200 when limit is not positive).
func (r *Reader) ByRun(owner, run string, afterSeq int64, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 200
	}
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	var cands []cand
	full, thresh := false, int64(math.MaxInt64)
	for _, f := range ls.segs {
		if sum := r.summary(f); sum != nil {
			rng, ok := sum.Runs[runKey(owner, run)]
			if !ok || rng[1] <= afterSeq || (full && rng[0] > thresh) {
				continue
			}
		}
		err := r.withIdx(f, func(x *segIdx) {
			o, ok1 := x.strs.lookup(owner)
			ru, ok2 := x.strs.lookup(run)
			if !ok1 || !ok2 {
				return
			}
			for _, i := range x.byRun[pair{o, ru}] {
				if e := x.ents[i]; e.seq > afterSeq {
					cands = append(cands, cand{f.segno, e.off, e.n, e.seq, e.stream})
				}
			}
		})
		if err != nil && !goneErr(err) {
			return nil, err
		}
		if len(cands) >= limit {
			sortCands(cands, true)
			cands = cands[:limit]
			full, thresh = true, cands[limit-1].seq
		}
	}
	sortCands(cands, true)
	if len(cands) > limit {
		cands = cands[:limit]
	}
	return r.load(cands)
}

// ByTrajectory returns the trajectory's first limit lines, ascending in
// stream_seq.
func (r *Reader) ByTrajectory(owner, traj string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 200
	}
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	var cands []cand
	full, thresh := false, int64(math.MaxInt64)
	for _, f := range ls.segs {
		if sum := r.summary(f); sum != nil {
			rng, ok := sum.Trajs[runKey(owner, traj)]
			if !ok || (full && rng[0] > thresh) {
				continue
			}
		}
		err := r.withIdx(f, func(x *segIdx) {
			o, ok1 := x.strs.lookup(owner)
			t, ok2 := x.strs.lookup(traj)
			if !ok1 || !ok2 {
				return
			}
			for _, i := range x.byTraj[pair{o, t}] {
				e := x.ents[i]
				cands = append(cands, cand{f.segno, e.off, e.n, e.seq, e.stream})
			}
		})
		if err != nil && !goneErr(err) {
			return nil, err
		}
		if len(cands) >= limit {
			sortCands(cands, false)
			cands = cands[:limit]
			full, thresh = true, cands[limit-1].stream
		}
	}
	sortCands(cands, false)
	if len(cands) > limit {
		cands = cands[:limit]
	}
	return r.load(cands)
}

// ByOwnerAfter returns the owner's lines with stream_seq > after, ascending,
// at most limit.
func (r *Reader) ByOwnerAfter(owner string, after int64, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 200
	}
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	var cands []cand
	full, thresh := false, int64(math.MaxInt64)
	for _, f := range ls.segs {
		if sum := r.summary(f); sum != nil {
			if !sum.hasOwner(owner) || sum.MaxStream <= after || (full && sum.MinStream > thresh) {
				continue
			}
		}
		err := r.withIdx(f, func(x *segIdx) {
			o, ok := x.strs.lookup(owner)
			if !ok {
				return
			}
			start, matched := 0, 0
			if x.monotone {
				start = sort.Search(len(x.ents), func(i int) bool { return x.ents[i].stream > after })
			}
			for i := start; i < len(x.ents); i++ {
				e := x.ents[i]
				if e.owner != o || e.stream <= after {
					continue
				}
				cands = append(cands, cand{f.segno, e.off, e.n, e.seq, e.stream})
				matched++
				if x.monotone && matched >= limit {
					break
				}
			}
		})
		if err != nil && !goneErr(err) {
			return nil, err
		}
		if len(cands) >= limit {
			sortCands(cands, false)
			cands = cands[:limit]
			full, thresh = true, cands[limit-1].stream
		}
	}
	sortCands(cands, false)
	if len(cands) > limit {
		cands = cands[:limit]
	}
	return r.load(cands)
}

// ByOwnerNewest returns the owner's newest limit lines, newest first.
func (r *Reader) ByOwnerNewest(owner string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 200
	}
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	var cands []cand
	full, thresh := false, int64(math.MinInt64)
	newestFirst := func(c []cand) {
		sort.Slice(c, func(i, j int) bool {
			if c[i].stream != c[j].stream {
				return c[i].stream > c[j].stream
			}
			return c[i].off > c[j].off
		})
	}
	for i := len(ls.segs) - 1; i >= 0; i-- {
		f := ls.segs[i]
		if sum := r.summary(f); sum != nil {
			if !sum.hasOwner(owner) || (full && sum.MaxStream < thresh) {
				continue
			}
		}
		err := r.withIdx(f, func(x *segIdx) {
			o, ok := x.strs.lookup(owner)
			if !ok {
				return
			}
			matched := 0
			for j := len(x.ents) - 1; j >= 0; j-- {
				e := x.ents[j]
				if e.owner != o {
					continue
				}
				cands = append(cands, cand{f.segno, e.off, e.n, e.seq, e.stream})
				matched++
				if x.monotone && matched >= limit {
					break
				}
			}
		})
		if err != nil && !goneErr(err) {
			return nil, err
		}
		if len(cands) >= limit {
			newestFirst(cands)
			cands = cands[:limit]
			full, thresh = true, cands[limit-1].stream
		}
	}
	newestFirst(cands)
	if len(cands) > limit {
		cands = cands[:limit]
	}
	return r.load(cands)
}

// Scan visits every record of every segment in order, boundaries included,
// reading the files front to back. A segment deleted before the scan reaches
// it is skipped; one deleted while it is being read is read to its end. fn
// returns false to stop.
func (r *Reader) Scan(fn func(Item) bool) error {
	ls, err := r.list()
	if err != nil {
		return err
	}
	for _, f := range ls.segs {
		fh, err := openRegular(r.tlogPath(f.segno), os.O_RDONLY, 0)
		if err != nil {
			if goneErr(err) {
				r.skipped.Add(1)
				continue
			}
			return err
		}
		stop, err := scanItems(fh, f.segno, fn)
		fh.Close()
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
	return nil
}

func scanItems(fh *os.File, segno uint64, fn func(Item) bool) (stopped bool, err error) {
	info, err := fh.Stat()
	if err != nil {
		return false, err
	}
	head, err := readHead(fh)
	if err != nil {
		return false, err
	}
	class, hdrLen, _, _ := classifyHeader(head, segno)
	if class != headerOK {
		return false, nil
	}
	_, _, err = walk(fh, int64(hdrLen), info.Size(), func(off int64, typ byte, body []byte, total int) error {
		if stopped {
			return nil
		}
		switch typ {
		case typeLine:
			m, line, err := decodeLine(body)
			if err != nil {
				return err
			}
			line = append([]byte(nil), line...)
			if !fn(Item{Record: Record{Meta: m, Line: line, Segment: segno, Offset: off}}) {
				stopped = true
			}
		case typeBoundary:
			b, err := decodeBoundary(body)
			if err != nil {
				return err
			}
			if !fn(Item{Record: Record{Segment: segno, Offset: off}, Boundary: &b}) {
				stopped = true
			}
		}
		return nil
	})
	return stopped, err
}

// Segments lists the segments, oldest first.
func (r *Reader) Segments() ([]SegmentInfo, error) {
	ls, err := r.list()
	if err != nil {
		return nil, err
	}
	out := make([]SegmentInfo, 0, len(ls.segs))
	for _, f := range ls.segs {
		info, err := r.info(f)
		if err != nil {
			if goneErr(err) {
				continue
			}
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

func (r *Reader) info(f segFile) (SegmentInfo, error) {
	si := SegmentInfo{Segno: f.segno, Sealed: f.hasIdx, Bytes: f.tlogSize, IndexBytes: f.idxSize}
	fromIdx := func(x *segIdx, validEnd int64) {
		si.Records, si.Lines, si.ValidEnd = len(x.ents)+x.nBound, len(x.ents), validEnd
		si.MinStream, si.MaxStream = x.minStream, x.maxStream
		if len(x.ents) > 0 {
			si.MinTime = nanosTime(x.minTs)
		}
	}
	if r.live != nil && r.live.withOpen(f.segno, func(s *openSeg) {
		si.Sealed = false
		si.CreatedAt = s.created
		fromIdx(s.idx, s.off)
	}) {
		return si, nil
	}
	if f.hasIdx {
		sum := r.summary(f)
		if sum == nil {
			// No usable index: fall back to scanning for this segment.
			st := r.state(f)
			st.mu.Lock()
			err := r.loadSealed(f, st, true)
			sum = st.sum
			st.mu.Unlock()
			if err != nil {
				return si, err
			}
		}
		si.Records, si.Lines, si.ValidEnd = sum.Records, sum.Lines, sum.ValidEnd
		si.MinStream, si.MaxStream = sum.MinStream, sum.MaxStream
		si.CreatedAt, si.SealedAt = time.Unix(0, sum.CreatedAt).UTC(), time.Unix(0, sum.SealedAt).UTC()
		if sum.Lines > 0 {
			si.MinTime = nanosTime(sum.MinTs)
		}
		return si, nil
	}
	st := r.state(f)
	st.mu.Lock()
	defer st.mu.Unlock()
	if err := r.loadUnsealed(f, st); err != nil {
		return si, err
	}
	si.CreatedAt = st.scan.created
	fromIdx(st.scan.idx, st.scan.validEnd)
	return si, nil
}

// Floor is the oldest line still held.
func (r *Reader) Floor() (Floor, error) {
	segs, err := r.Segments()
	if err != nil {
		return Floor{}, err
	}
	for _, s := range segs {
		if s.Lines > 0 {
			return Floor{Segment: s.Segno, Time: s.MinTime, StreamSeq: s.MinStream}, nil
		}
	}
	return Floor{Empty: true}, nil
}

// MaxStreamSeq is the highest stream_seq in the newest segment that has a
// line, or zero. The store's seeds read it (design 4.4).
func (r *Reader) MaxStreamSeq() (int64, error) {
	segs, err := r.Segments()
	if err != nil {
		return 0, err
	}
	for i := len(segs) - 1; i >= 0; i-- {
		if segs[i].Lines > 0 {
			return segs[i].MaxStream, nil
		}
	}
	return 0, nil
}

// MaxRunSeq is the highest seq the log holds for a run, or zero.
func (r *Reader) MaxRunSeq(owner, run string) (int64, error) {
	ls, err := r.list()
	if err != nil {
		return 0, err
	}
	var best int64
	for _, f := range ls.segs {
		if sum := r.summary(f); sum != nil {
			if rng, ok := sum.Runs[runKey(owner, run)]; ok {
				best = max(best, rng[1])
			}
			continue
		}
		err := r.withIdx(f, func(x *segIdx) {
			o, ok1 := x.strs.lookup(owner)
			ru, ok2 := x.strs.lookup(run)
			if !ok1 || !ok2 {
				return
			}
			for _, i := range x.byRun[pair{o, ru}] {
				best = max(best, x.ents[i].seq)
			}
		})
		if err != nil && !goneErr(err) {
			return 0, err
		}
	}
	return best, nil
}
