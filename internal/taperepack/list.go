package taperepack

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Spill record formats. All integers are big endian.
//
//	list: ns u8 | digest [32]                                   (33 bytes)
//	edge: artifact [32] | ns u8 | computer u32 | length u32 | flags u8   (42 bytes)
const (
	listRecSize = 33
	edgeRecSize = 42

	// Edge flag bits.
	edgeEligible = 1 // the receipt is eligible for the copy
	edgeRejected = 2 // a PinReceipt the shape guard rejected: it names the file but attributes nothing
	edgeRef      = 4 // an event body in a segment names the digest: guards, never copies
)

// computerTable interns computer ids for the edge spill.
type computerTable struct {
	names []string
	index map[string]uint32
}

func newComputerTable(names []string) *computerTable {
	t := &computerTable{names: append([]string(nil), names...), index: map[string]uint32{}}
	for i, n := range t.names {
		t.index[n] = uint32(i)
	}
	return t
}

func (t *computerTable) intern(name string) (uint32, error) {
	if i, ok := t.index[name]; ok {
		return i, nil
	}
	if len(t.names) >= 1<<22 {
		return 0, errors.New("taperepack: more than four million distinct computer ids in the receipts")
	}
	if len(name) > 256 {
		name = name[:256]
	}
	i := uint32(len(t.names))
	t.names = append(t.names, name)
	t.index[name] = i
	return i, nil
}

type spill struct {
	files [buckets]*os.File
	w     [buckets]*bufio.Writer
}

func newSpill(dir, prefix string) (*spill, error) {
	s := &spill{}
	for b := 0; b < buckets; b++ {
		f, err := os.OpenFile(filepath.Join(dir, fmt.Sprintf("%s-%02x.bin", prefix, b)), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			s.close()
			return nil, err
		}
		s.files[b], s.w[b] = f, bufio.NewWriterSize(f, 8<<10)
	}
	return s, nil
}

func (s *spill) close() error {
	var first error
	for b := 0; b < buckets; b++ {
		if s.w[b] != nil {
			if err := s.w[b].Flush(); err != nil && first == nil {
				first = err
			}
		}
		if s.files[b] != nil {
			if fsyncWork {
				if err := s.files[b].Sync(); err != nil && first == nil {
					first = err
				}
			}
			if err := s.files[b].Close(); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

// eachDirEntry streams a directory in batches, so a directory of millions of
// files is never held whole.
func eachDirEntry(dir string, fn func(os.DirEntry) error) error {
	d, err := os.Open(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer d.Close()
	for {
		batch, err := d.ReadDir(1024)
		for _, de := range batch {
			if ferr := fn(de); ferr != nil {
				return ferr
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

var listing = []struct {
	ns     tapestore.NS
	dir    string
	suffix string
}{
	{tapestore.NSEvent, eventNamespace, ""},
	{tapestore.NSPayload, payloadNamespace, ""},
	{tapestore.NSReceipt, receiptNamespace, ".json"},
}

// list streams the three legacy directories into the bucket spill files and
// reads each pin receipt and each event body once for its edges (the join
// reads every file again for its verdict).
func (e *engine) list() error {
	ls, err := newSpill(filepath.Join(e.dir, "list"), "l")
	if err != nil {
		return err
	}
	es, err := newSpill(filepath.Join(e.dir, "edge"), "e")
	if err != nil {
		ls.close()
		return err
	}
	runErr := func() error {
		for _, l := range listing {
			dir := filepath.Join(e.root, "sha256", l.dir)
			err := eachDirEntry(dir, func(de os.DirEntry) error {
				name := de.Name()
				digest, ok := strings.CutSuffix(name, l.suffix)
				if !ok || !tapestore.ValidDigest(digest) {
					e.st.Listing["other_files"]++
					return nil
				}
				if !de.Type().IsRegular() {
					// tapestore reads a symlinked legacy file and verifies
					// its digest, so the references it holds are unseen here
					// (review round 4, F2): fail closed.
					e.st.Listing["other_files"]++
					e.st.Listing["non_regular_legacy"]++
					e.attrError(legacyNonRegularKey, fmt.Sprintf("%s/%s is not a regular file", l.dir, name))
					return nil
				}
				e.st.Listing[nsName(l.ns)]++
				var rec [listRecSize]byte
				rec[0] = byte(l.ns)
				if _, err := hex.Decode(rec[1:], []byte(digest)); err != nil {
					return err
				}
				if _, err := ls.w[rec[1]].Write(rec[:]); err != nil {
					return err
				}
				switch l.ns {
				case tapestore.NSReceipt:
					return e.listReceipt(es, filepath.Join(dir, name))
				case tapestore.NSEvent:
					return e.listLegacyEvent(es, digest, filepath.Join(dir, name))
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("taperepack: list %s: %w", l.dir, err)
			}
		}
		return e.listSegments(es)
	}()
	if cerr := ls.close(); runErr == nil {
		runErr = cerr
	}
	if cerr := es.close(); runErr == nil {
		runErr = cerr
	}
	return runErr
}

// legacyEventsKey names the attribution error of a legacy event body that
// could not be read or decoded.
//
// The keys are not valid computer ids, so they cannot collide with one.
const (
	legacyEventsKey     = "(legacy-events)"
	legacyNonRegularKey = "(legacy-non-regular)"
)

// listLegacyEvent reads one legacy event file and spills an edgeRef for every
// digest its body names, for the computer the body says it belongs to: the
// sweep keeps what any chain event body names, segment or legacy (review
// round 3, L1). A file that is gone makes no edge. One that cannot be read,
// is too large to read, or does not say which computer it belongs to names
// digests nobody can see: an attribution error, so the pass is unhealthy and
// deletes nothing.
func (e *engine) listLegacyEvent(es *spill, digest, path string) error {
	e.pace.wait()
	raw, st, err := e.readCapped(path, tapestore.MaxRecordBytes)
	if err != nil {
		return err
	}
	e.filesSeen++
	switch st {
	case readOK:
	case readVanished:
		return nil
	default:
		e.attrError(legacyEventsKey, fmt.Sprintf("event %s cannot be read for its references", digest))
		return nil
	}
	e.st.Listing["legacy_event_bodies"]++
	var head struct {
		ComputerID string `json:"computer_id"`
	}
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&head); err != nil || !tapestore.ValidComputerID(head.ComputerID) {
		e.attrError(legacyEventsKey, fmt.Sprintf("event %s does not decode to a computer id", digest))
		return nil
	}
	for _, d := range hexRefs(raw) {
		for _, ns := range []tapestore.NS{tapestore.NSEvent, tapestore.NSPayload} {
			if err := e.spillEdge(es, d, ns, head.ComputerID, 0, edgeRef); err != nil {
				return err
			}
		}
	}
	return nil
}

// listReceipt reads one legacy pin receipt and spills its edge. A receipt that
// is gone makes no edge; one that cannot be read, or that claims to be a
// receipt and names nothing, makes the run unhealthy and deletes nothing.
func (e *engine) listReceipt(es *spill, path string) error {
	e.pace.wait()
	raw, st, err := e.readCapped(path, maxReceiptBytes)
	if err != nil {
		return err
	}
	e.filesSeen++
	switch st {
	case readOK:
		return e.spillReceipt(es, raw, "")
	case readVanished:
		return nil
	default:
		e.st.Listing["unnamable_receipts"]++
		return nil
	}
}

// spillReceipt classifies one pin receipt (a legacy file, or a record in a
// computer's segment) and spills its edge. dir is the computer whose segment
// held it, or empty for a legacy file; a receipt in a segment attributes that
// computer as well as the one it names (more requirements, never fewer).
func (e *engine) spillReceipt(es *spill, raw []byte, dir string) error {
	ed, st := classifyReceipt(raw)
	switch st {
	case receiptForeign:
		return nil
	case receiptUnnamable:
		e.st.Listing["unnamable_receipts"]++
		return nil
	case receiptRejected:
		e.st.Listing["rejected_receipts"]++
		for _, ns := range []tapestore.NS{tapestore.NSEvent, tapestore.NSPayload} {
			if err := e.spillEdge(es, ed.artifact, ns, ed.computer, 0, edgeRejected); err != nil {
				return err
			}
		}
		return nil
	}
	length := ed.length
	eligible := ed.eligible
	if length < 0 || length > tapestore.MaxRecordBytes {
		length, eligible = 0, false
	}
	var flags byte
	if eligible {
		flags = edgeEligible
	}
	if err := e.spillEdge(es, ed.artifact, ed.ns, ed.computer, uint32(length), flags); err != nil {
		return err
	}
	if dir != "" && dir != ed.computer {
		return e.spillEdge(es, ed.artifact, ed.ns, dir, uint32(length), flags)
	}
	return nil
}

func (e *engine) spillEdge(es *spill, artifact [32]byte, ns tapestore.NS, computer string, length uint32, flags byte) error {
	idx, err := e.table.intern(computer)
	if err != nil {
		return err
	}
	var rec [edgeRecSize]byte
	copy(rec[0:32], artifact[:])
	rec[32] = byte(ns)
	binary.BigEndian.PutUint32(rec[33:37], idx)
	binary.BigEndian.PutUint32(rec[37:41], length)
	rec[41] = flags
	_, err = es.w[rec[0]].Write(rec[:])
	return err
}

// attrError records that a computer's segments could not be read to the end,
// so the receipts and event references in them are unknown. The run is
// unhealthy and unlinks nothing (attrBroken).
func (e *engine) attrError(computer, msg string) {
	if e.st.AttrErrors == nil {
		e.st.AttrErrors = map[string]string{}
	}
	if _, ok := e.st.AttrErrors[computer]; !ok {
		e.st.AttrErrors[computer] = msg
	}
}

// attrBroken is true when the attribution is known to be incomplete: a
// computer's segments could not be listed or read, or something that claims
// to be a pin receipt names no artifact.
func (e *engine) attrBroken() bool {
	return len(e.st.AttrErrors) > 0 || e.st.Listing["unnamable_receipts"] > 0
}

// listSegments adds the edges the legacy receipts cannot show (review round
// 2, R1). With the segment flag on, a repin of an artifact whose legacy file
// exists appends the receipt to the computer's segment only and writes no
// legacy receipt, and the computer keeps reading the shared legacy file. So
// every computer's segments, sealed and unsealed (attribution must be complete
// even though only sealed copies count toward coverage), are read for their
// pin receipts and for the digests their event bodies name.
func (e *engine) listSegments(es *spill) error {
	return eachDirEntry(filepath.Join(e.root, tapestore.SegDirName), func(de os.DirEntry) error {
		name := de.Name()
		if de.Type()&os.ModeSymlink != 0 {
			// OpenView would follow it for coverage; attribution does not.
			e.attrError(name, "tape-seg entry is a symbolic link")
			return nil
		}
		if !de.IsDir() {
			return nil
		}
		if !tapestore.ValidComputerID(name) {
			e.attrError(name, "tape-seg directory is not a valid computer id")
			return nil
		}
		return e.listComputerSegments(es, name)
	})
}

func (e *engine) listComputerSegments(es *spill, computer string) error {
	view, err := e.reader.OpenView(computer)
	if err != nil {
		e.attrError(computer, err.Error())
		return nil
	}
	for b := 0; b < buckets; b++ {
		locs, err := view.Bucket(byte(b))
		if err != nil {
			e.attrError(computer, err.Error())
			return nil
		}
		for _, l := range locs {
			if l.NS != tapestore.NSReceipt && l.NS != tapestore.NSEvent {
				continue
			}
			e.pace.wait()
			body, err := view.Read(l)
			if err != nil {
				// The legacy file of the same digest, when it still verifies,
				// says the same; otherwise the record's content is unknown.
				var ok bool
				if body, ok = e.legacyBody(l.NS, l.Digest); !ok {
					e.attrError(computer, fmt.Sprintf("%s %x: %v", l.NS, l.Digest, err))
					return nil
				}
			}
			if l.NS == tapestore.NSReceipt {
				e.st.Listing["segment_receipts"]++
				if err := e.spillReceipt(es, body, computer); err != nil {
					return err
				}
				continue
			}
			for _, d := range hexRefs(body) {
				for _, ns := range []tapestore.NS{tapestore.NSEvent, tapestore.NSPayload} {
					if err := e.spillEdge(es, d, ns, computer, 0, edgeRef); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// legacyBody reads the legacy file of a digest and returns it only if it
// hashes to that digest. The slice is a copy.
func (e *engine) legacyBody(ns tapestore.NS, digest [32]byte) ([]byte, bool) {
	raw, st, err := e.readCapped(filepath.Join(e.root, filepath.FromSlash(ns.LegacyRel(hex.EncodeToString(digest[:])))), tapestore.MaxRecordBytes)
	if err != nil || st != readOK || sha256.Sum256(raw) != digest {
		return nil, false
	}
	return append([]byte(nil), raw...), true
}

type readStatus int

const (
	readOK readStatus = iota
	readVanished
	readTooLarge
	readNotRegular
)

// readCapped reads a regular file of at most limit bytes into the engine's
// reusable buffer. The returned slice is valid until the next call.
func (e *engine) readCapped(path string, limit int64) ([]byte, readStatus, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, readVanished, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, readNotRegular, nil
	}
	if info.Size() > limit {
		return nil, readTooLarge, nil
	}
	n := int(info.Size())
	if cap(e.buf) < n {
		e.buf = make([]byte, n, n+n/4)
	}
	buf := e.buf[:n]
	if _, err := io.ReadFull(f, buf); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return nil, readVanished, nil // truncated under us: not a stable file
		}
		return nil, 0, err
	}
	return buf, readOK, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
