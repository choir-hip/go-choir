package taperepack

import (
	"bufio"
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
// reads each pin receipt once for its edge.
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
				if !ok || !tapestore.ValidDigest(digest) || !de.Type().IsRegular() {
					e.st.Listing["other_files"]++
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
				if l.ns != tapestore.NSReceipt {
					return nil
				}
				return e.listReceipt(es, filepath.Join(dir, name))
			})
			if err != nil {
				return fmt.Errorf("taperepack: list %s: %w", l.dir, err)
			}
		}
		return nil
	}()
	if cerr := ls.close(); runErr == nil {
		runErr = cerr
	}
	if cerr := es.close(); runErr == nil {
		runErr = cerr
	}
	return runErr
}

// listReceipt reads one pin receipt and spills its edge. A receipt that is
// gone, too large or not a pin receipt makes no edge.
func (e *engine) listReceipt(es *spill, path string) error {
	e.pace.wait()
	raw, st, err := e.readCapped(path, maxReceiptBytes)
	if err != nil {
		return err
	}
	e.filesSeen++
	if st != readOK {
		return nil
	}
	ed, ok := parseReceipt(raw)
	if !ok {
		return nil
	}
	idx, err := e.table.intern(ed.computer)
	if err != nil {
		return err
	}
	var rec [edgeRecSize]byte
	copy(rec[0:32], ed.artifact[:])
	rec[32] = byte(ed.ns)
	binary.BigEndian.PutUint32(rec[33:37], idx)
	length := ed.length
	eligible := ed.eligible
	if length < 0 || length > tapestore.MaxRecordBytes {
		length, eligible = 0, false
	}
	binary.BigEndian.PutUint32(rec[37:41], uint32(length))
	if eligible {
		rec[41] = 1
	}
	_, err = es.w[rec[0]].Write(rec[:])
	return err
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
