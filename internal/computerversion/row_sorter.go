package computerversion

import (
	"bufio"
	"container/heap"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// witnessSpillBytes bounds the row encodings a table digest holds in memory.
// Past it, sorted runs spill to zstd files and are merged, so a table larger
// than memory hashes to the same digest as the in-memory sort did. The paper's
// 26 GB store was OOM-killed at 14 GB holding every row (2026-10-28).
const witnessSpillBytes = 512 << 20

// witnessSpillDirEnv names the directory spill runs go to (default: the
// system temp directory). Hosts point it at the disk with room.
const witnessSpillDirEnv = "CHOIR_WITNESS_SPILL_DIR"

// rowSorter digests rows as sha256 over the sorted encodings, each followed
// by a newline. Encodings must not contain a newline (JSON does not).
type rowSorter struct {
	dir   string
	limit int
	buf   []string
	size  int
	runs  []string
}

func newRowSorter(dir string, limit int) *rowSorter {
	return &rowSorter{dir: dir, limit: limit}
}

func (s *rowSorter) Add(row string) error {
	if strings.IndexByte(row, '\n') >= 0 {
		return errors.New("row encoding contains a newline")
	}
	s.buf = append(s.buf, row)
	s.size += len(row) + 16
	if s.size >= s.limit {
		return s.spill()
	}
	return nil
}

func (s *rowSorter) spill() error {
	sort.Strings(s.buf)
	f, err := os.CreateTemp(s.dir, "witness-run-*.zst")
	if err != nil {
		return fmt.Errorf("witness spill: %w", err)
	}
	s.runs = append(s.runs, f.Name())
	zw, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		f.Close()
		return err
	}
	bw := bufio.NewWriterSize(zw, 1<<20)
	for _, r := range s.buf {
		bw.WriteString(r)
		bw.WriteByte('\n')
	}
	err = errors.Join(bw.Flush(), zw.Close(), f.Close())
	if err != nil {
		return fmt.Errorf("witness spill: %w", err)
	}
	s.buf, s.size = nil, 0
	return nil
}

// Sum returns the digest. It consumes the sorter.
func (s *rowSorter) Sum() (string, error) {
	hasher := sha256.New()
	if len(s.runs) == 0 {
		sort.Strings(s.buf)
		for _, r := range s.buf {
			io.WriteString(hasher, r)
			hasher.Write([]byte("\n"))
		}
		s.buf = nil
		return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
	}
	if len(s.buf) > 0 {
		if err := s.spill(); err != nil {
			return "", err
		}
	}
	h := &runHeap{}
	defer h.close()
	for _, name := range s.runs {
		f, err := os.Open(name)
		if err != nil {
			return "", err
		}
		zr, err := zstd.NewReader(f, zstd.WithDecoderConcurrency(1))
		if err != nil {
			f.Close()
			return "", err
		}
		r := &runReader{f: f, zr: zr, br: bufio.NewReaderSize(zr, 1<<20)}
		h.open = append(h.open, r)
		ok, err := r.next()
		if err != nil {
			return "", err
		}
		if ok {
			heap.Push(h, r)
		}
	}
	for h.Len() > 0 {
		r := h.items[0]
		io.WriteString(hasher, r.cur)
		hasher.Write([]byte("\n"))
		ok, err := r.next()
		if err != nil {
			return "", err
		}
		if ok {
			heap.Fix(h, 0)
		} else {
			heap.Pop(h)
		}
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}

// Close removes the spill runs.
func (s *rowSorter) Close() {
	for _, name := range s.runs {
		os.Remove(name)
	}
	s.runs, s.buf = nil, nil
}

type runReader struct {
	f   *os.File
	zr  *zstd.Decoder
	br  *bufio.Reader
	cur string
}

func (r *runReader) next() (bool, error) {
	line, err := r.br.ReadString('\n')
	if err == io.EOF && line == "" {
		return false, nil
	}
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("witness spill read: %w", err)
	}
	r.cur = strings.TrimSuffix(line, "\n")
	return true, nil
}

type runHeap struct {
	items []*runReader
	open  []*runReader
}

func (h *runHeap) Len() int           { return len(h.items) }
func (h *runHeap) Less(i, j int) bool { return h.items[i].cur < h.items[j].cur }
func (h *runHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *runHeap) Push(x any)         { h.items = append(h.items, x.(*runReader)) }
func (h *runHeap) Pop() any {
	last := h.items[len(h.items)-1]
	h.items = h.items[:len(h.items)-1]
	return last
}

func (h *runHeap) close() {
	for _, r := range h.open {
		r.zr.Close()
		r.f.Close()
	}
}
