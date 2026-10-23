package taperepack

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

type locKey struct {
	ns     tapestore.NS
	digest [32]byte
}

type listRec struct {
	ns     tapestore.NS
	digest [32]byte
}

type edgeVal struct {
	computer uint32
	length   uint32
	eligible bool
}

// compIdx is one computer's segment records of the current bucket.
type compIdx struct {
	view *tapestore.SegmentView
	locs map[locKey][]tapestore.SegLoc
	err  error
}

type pendItem struct {
	ns   tapestore.NS
	body []byte
}

type bucketCtx struct {
	b       int
	edges   map[locKey][]edgeVal
	comps   map[string]*compIdx
	pending map[string][]pendItem
	pendLen int
	part    *Report
}

func (e *engine) readList(b int) ([]listRec, error) {
	f, err := os.Open(filepath.Join(e.dir, "list", fmt.Sprintf("l-%02x.bin", b)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []listRec
	br := bufio.NewReaderSize(f, 64<<10)
	var rec [listRecSize]byte
	for {
		if _, err := io.ReadFull(br, rec[:]); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		r := listRec{ns: tapestore.NS(rec[0])}
		copy(r.digest[:], rec[1:])
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ns != out[j].ns {
			return out[i].ns < out[j].ns
		}
		return bytes.Compare(out[i].digest[:], out[j].digest[:]) < 0
	})
	return out, nil
}

func (e *engine) readEdges(b int) (map[locKey][]edgeVal, error) {
	f, err := os.Open(filepath.Join(e.dir, "edge", fmt.Sprintf("e-%02x.bin", b)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[locKey][]edgeVal{}
	br := bufio.NewReaderSize(f, 64<<10)
	var rec [edgeRecSize]byte
	for {
		if _, err := io.ReadFull(br, rec[:]); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		var k locKey
		copy(k.digest[:], rec[0:32])
		k.ns = tapestore.NS(rec[32])
		out[k] = append(out[k], edgeVal{
			computer: binary.BigEndian.Uint32(rec[33:37]),
			length:   binary.BigEndian.Uint32(rec[37:41]),
			eligible: rec[41] == 1,
		})
	}
	return out, nil
}

// bucket joins one bucket of legacy files against the segment store.
func (e *engine) bucket(b int) (*Report, error) {
	files, err := e.readList(b)
	if err != nil {
		return nil, err
	}
	edges, err := e.readEdges(b)
	if err != nil {
		return nil, err
	}
	bc := &bucketCtx{b: b, edges: edges, comps: map[string]*compIdx{}, pending: map[string][]pendItem{}, part: newReport()}
	for _, f := range files {
		if err := e.file(bc, f); err != nil {
			return nil, err
		}
		if cap(e.buf) > 8<<20 {
			e.buf = nil
		}
		if e.opts.Apply && bc.pendLen > pendingCapBytes {
			if err := e.flushLargest(bc); err != nil {
				return nil, err
			}
		}
	}
	appended := false
	if e.opts.Apply {
		before := bc.part.Copy.Appends
		for _, c := range sortedKeys(bc.pending) {
			if err := e.flush(bc, c); err != nil {
				return nil, err
			}
		}
		appended = bc.part.Copy.Appends > before || bc.part.Copy.CopiedRecords > 0
		if appended && e.opts.testAfterAppend != nil {
			if err := e.opts.testAfterAppend(b); err != nil {
				return nil, err
			}
		}
	}
	return bc.part, nil
}

// comp returns the computer's records of this bucket, reading them once.
func (e *engine) comp(bc *bucketCtx, computer string) *compIdx {
	if ci := bc.comps[computer]; ci != nil {
		return ci
	}
	ci := &compIdx{locs: map[locKey][]tapestore.SegLoc{}}
	bc.comps[computer] = ci
	ci.view, ci.err = e.reader.OpenView(computer)
	if ci.err == nil {
		var locs []tapestore.SegLoc
		if locs, ci.err = ci.view.Bucket(byte(bc.b)); ci.err == nil {
			for _, l := range locs {
				k := locKey{l.NS, l.Digest}
				ci.locs[k] = append(ci.locs[k], l)
			}
		}
	}
	if ci.err != nil {
		if _, ok := bc.part.SegmentErrors[computer]; !ok {
			bc.part.SegmentErrors[computer] = ci.err.Error()
		}
	}
	return ci
}

// file reads one legacy file and decides its verdict.
func (e *engine) file(bc *bucketCtx, f listRec) error {
	digest := hex.EncodeToString(f.digest[:])
	path := filepath.Join(e.root, filepath.FromSlash(f.ns.LegacyRel(digest)))
	e.pace.wait()
	raw, st, err := e.readCapped(path, tapestore.MaxRecordBytes)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	e.filesSeen++
	e.report(fmt.Sprintf("bucket %d of %d", bc.b+1, buckets))
	v := Verdict{ns: f.ns, digest: digest}
	switch {
	case st == readVanished:
		v.class = classVanished
	case st == readTooLarge:
		v.class = classTooLarge
	case st == readNotRegular:
		v.class = classLegacyBad
	case sha256.Sum256(raw) != f.digest:
		v.class = classLegacyBad
	default:
		v.size = int64(len(raw))
		e.decide(bc, &v, raw)
	}
	bc.part.countGlobal(f.ns, v.class)
	if e.opts.OnVerdict != nil {
		if err := e.opts.OnVerdict(v); err != nil {
			return err
		}
	}
	return nil
}

// decide attributes a verified legacy file to the computers that need it and
// compares it with their segment copies. raw hashes to the file's name.
func (e *engine) decide(bc *bucketCtx, v *Verdict, raw []byte) {
	var computers []string
	switch v.ns {
	case tapestore.NSReceipt:
		ed, ok := parseReceipt(raw)
		if !ok || !ed.eligible {
			v.class = classIneligible
			return
		}
		computers = []string{ed.computer}
	default:
		edges := bc.edges[locKey{v.ns, mustDigest(v.digest)}]
		if len(edges) == 0 {
			v.class = classUnreceipted
			return
		}
		var event *computerevent.Event
		eventTried := false
		seen := map[string]bool{}
		for _, ed := range edges {
			if !ed.eligible || int64(ed.length) != int64(len(raw)) || int(ed.computer) >= len(e.table.names) {
				continue
			}
			name := e.table.names[ed.computer]
			if v.ns == tapestore.NSEvent {
				if !eventTried {
					eventTried = true
					if ev, err := computerevent.DecodeHistoricEvent(raw); err == nil && ev.EventKind == computerevent.EventProjectionBatchRecorded {
						event = &ev
					}
				}
				if event == nil || event.ComputerID != name {
					continue
				}
			}
			if !seen[name] {
				seen[name] = true
				computers = append(computers, name)
			}
		}
		if len(computers) == 0 {
			v.class = classIneligible
			return
		}
		sort.Strings(computers)
	}
	// want decides scope only: a file no wanted computer needs is out of
	// scope. The verdict is the worst across every computer that needs the
	// file, wanted or not, because the unlink removes the one file for all of
	// them (review round 1, F1); only the writes are gated on want.
	if e.want != nil {
		inScope := false
		for _, c := range computers {
			if e.want[c] {
				inScope = true
				break
			}
		}
		if !inScope {
			v.class = classOutOfScope
			return
		}
	}
	v.computers = computers
	worst := classCovered
	key := locKey{v.ns, mustDigest(v.digest)}
	for _, c := range computers {
		cl := e.cover(bc, c, key, raw)
		if e.want != nil && !e.want[c] {
			if rank(cl) > rank(worst) {
				worst = cl
			}
			continue
		}
		bc.part.countComputer(c, v.ns, cl)
		if cl == classUncovered {
			if e.opts.Apply {
				bc.pending[c] = append(bc.pending[c], pendItem{ns: v.ns, body: append([]byte(nil), raw...)})
				bc.pendLen += len(raw)
			}
			if e.opts.Copy && !e.opts.Apply {
				bc.part.Copy.WouldCopyRecords++
				bc.part.Copy.WouldCopyBytes += int64(len(raw))
			}
		}
		if rank(cl) > rank(worst) {
			worst = cl
		}
	}
	v.class = worst
}

func rank(c class) int {
	switch c {
	case classCovered:
		return 0
	case classUncovered:
		return 1
	case classBlocked:
		return 2
	case classCorrupt:
		return 3
	}
	return 0
}

func mustDigest(s string) [32]byte {
	var d [32]byte
	_, _ = hex.Decode(d[:], []byte(s))
	return d
}

// cover is the verdict of one computer for one record: covered only when it
// has at least one copy in a sealed segment and every sealed copy verifies and
// equals raw. A copy in an unsealed open tail may not be fsynced (the tail is
// made durable by the next writer's fsync or by Seal), so it does not count
// (review round 1, F2).
func (e *engine) cover(bc *bucketCtx, computer string, key locKey, raw []byte) class {
	ci := e.comp(bc, computer)
	if ci.err != nil {
		return classBlocked
	}
	var locs []tapestore.SegLoc
	for _, l := range ci.locs[key] {
		if l.Sealed {
			locs = append(locs, l)
		}
	}
	if len(locs) == 0 {
		return classUncovered
	}
	for _, l := range locs {
		body, err := ci.view.Read(l)
		if err != nil || !bytes.Equal(body, raw) {
			return classCorrupt
		}
	}
	return classCovered
}

func (e *engine) flushLargest(bc *bucketCtx) error {
	for bc.pendLen > pendingLowBytes {
		var best string
		bestLen := -1
		for _, c := range sortedKeys(bc.pending) {
			n := 0
			for _, it := range bc.pending[c] {
				n += len(it.body)
			}
			if n > bestLen {
				best, bestLen = c, n
			}
		}
		if bestLen <= 0 {
			return nil
		}
		if err := e.flush(bc, best); err != nil {
			return err
		}
	}
	return nil
}

// flush appends a computer's pending records of the bucket through the
// production appender, then reads each one back from the segments.
func (e *engine) flush(bc *bucketCtx, computer string) error {
	items := bc.pending[computer]
	delete(bc.pending, computer)
	for _, it := range items {
		bc.pendLen -= len(it.body)
	}
	if len(items) == 0 {
		return nil
	}
	e.touched[computer] = true
	var written []pendItem
	err := e.app.AppendFunc(computer, func() ([]tapestore.Item, error) {
		// Under the computer's lock: what the segments hold now, including
		// what a live writer or an earlier run appended since the join read
		// them. Skip anything already present.
		view, err := e.reader.OpenView(computer)
		if err != nil {
			return nil, err
		}
		locs, err := view.Bucket(byte(bc.b))
		if err != nil {
			return nil, err
		}
		have := map[locKey]bool{}
		for _, l := range locs {
			have[locKey{l.NS, l.Digest}] = true
		}
		written = written[:0]
		var out []tapestore.Item
		for _, it := range items {
			k := locKey{it.ns, sha256.Sum256(it.body)}
			if have[k] {
				continue
			}
			have[k] = true
			written = append(written, it)
			out = append(out, tapestore.Item{NS: it.ns, Body: it.body})
		}
		return out, nil
	})
	if err != nil {
		return fmt.Errorf("computer %s: append: %w", computer, err)
	}
	bc.part.Copy.AlreadyPresent += int64(len(items) - len(written))
	if len(written) == 0 {
		return nil
	}
	bc.part.Copy.Appends++
	// Read back from the segments only.
	view, err := e.reader.OpenView(computer)
	if err != nil {
		return fmt.Errorf("computer %s: read back: %w", computer, err)
	}
	locs, err := view.Bucket(byte(bc.b))
	if err != nil {
		return fmt.Errorf("computer %s: read back: %w", computer, err)
	}
	byKey := map[locKey][]tapestore.SegLoc{}
	for _, l := range locs {
		k := locKey{l.NS, l.Digest}
		byKey[k] = append(byKey[k], l)
	}
	for _, it := range written {
		k := locKey{it.ns, sha256.Sum256(it.body)}
		ls := byKey[k]
		if len(ls) == 0 {
			return fmt.Errorf("computer %s: %s %x does not read back from a segment", computer, it.ns, k.digest)
		}
		for _, l := range ls {
			body, err := view.Read(l)
			if err != nil || !bytes.Equal(body, it.body) {
				return fmt.Errorf("computer %s: %s %x read back differs or fails: %v", computer, it.ns, k.digest, err)
			}
		}
		bc.part.Copy.CopiedRecords++
		bc.part.Copy.CopiedBytes += int64(len(it.body))
	}
	return nil
}
