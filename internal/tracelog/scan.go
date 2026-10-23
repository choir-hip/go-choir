package tracelog

import (
	"bufio"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"time"
)

// Record is one line read back from a segment.
type Record struct {
	Meta    Meta
	Line    []byte
	Segment uint64
	// Offset is the record's position in its segment file.
	Offset int64
}

// Item is one record of a scan: a line (Boundary is nil) or a boundary.
type Item struct {
	Record
	Boundary *Boundary
}

// resyncChunk is how much of the file one resync read covers.
const resyncChunk = 4 << 20

type scanResult struct {
	idx      *segIdx
	hdrLen   int
	created  time.Time
	validEnd int64 // the end of the last whole, verified record
	size     int64
	// torn is set when bytes after validEnd are not a whole record and nothing
	// whole follows them: the tail a crash leaves.
	torn  bool
	bad   string
	class headerClass
}

// readHead reads up to segHeaderMax bytes from the start of f.
func readHead(f *os.File) ([]byte, error) {
	head := make([]byte, segHeaderMax)
	n, err := f.ReadAt(head, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return head[:n], nil
}

// walk reads whole, checksummed records from start to size, calling visit for
// each. When a record fails its bounds or checksum, walk resynchronizes with
// resync and goes on from the next record that verifies; the skipped bytes are
// returned as spans. It stops, with end at the start of the unreadable bytes,
// when nothing whole follows (a torn tail). A record whose checksum holds but
// whose contents visit rejects with errBadRecord is a span of exactly its
// length. Records of a type this binary does not know are skipped. Any other
// error from visit, or from the file, ends the walk with that error.
func walk(f *os.File, start, size int64, visit func(off int64, typ byte, body []byte, total int) error) (end int64, spans [][2]int64, bad string, err error) {
	newReader := func(at int64) *bufio.Reader {
		return bufio.NewReaderSize(io.NewSectionReader(f, at, size-at), 256<<10)
	}
	r := newReader(start)
	var hdr [recHeaderSize]byte
	var body []byte
	off := start
	for off < size {
		reason := ""
		var h recHeader
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return off, spans, "", err
			}
			reason = "short record header"
		} else if h = parseRecHeader(hdr[:]); h.length > MaxRecordBytes {
			reason = fmt.Sprintf("record length %d", h.length)
		} else if int64(h.length) > size-off-recHeaderSize {
			reason = "record runs past the end of the file"
		} else {
			if cap(body) < int(h.length) {
				body = make([]byte, h.length, max(int(h.length), 4096))
			}
			body = body[:h.length]
			if _, err := io.ReadFull(r, body); err != nil {
				if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
					return off, spans, "", err
				}
				reason = "short record body"
			} else {
				c := crc32.Update(crc32.Checksum(hdr[0:4], crcTable), crcTable, hdr[8:9])
				if crc32.Update(c, crcTable, body) != h.crc {
					reason = "record checksum"
				}
			}
		}
		if reason != "" {
			next, found, err := resync(f, off, size)
			if err != nil {
				return off, spans, "", err
			}
			if !found {
				return off, spans, reason, nil
			}
			spans = append(spans, [2]int64{off, next - off})
			off = next
			r = newReader(off)
			continue
		}
		total := recHeaderSize + int(h.length)
		if h.typ == typeLine || h.typ == typeBoundary {
			if err := visit(off, h.typ, body, total); err != nil {
				if !errors.Is(err, errBadRecord) {
					return off, spans, "", err
				}
				spans = append(spans, [2]int64{off, int64(total)})
			}
		}
		off += int64(total)
	}
	return off, spans, "", nil
}

// errBadRecord marks a record whose checksum held but whose contents did not
// parse.
var errBadRecord = errors.New("record contents")

// resync finds where the next whole record starts after a bad one at from. It
// looks at every offset after from for a plausible header (a known type, a
// length within MaxRecordBytes and inside the file) whose record's own
// checksum verifies, and reports the first such offset. The checksum alone
// decides: a header-shaped pattern inside a payload passes the plausibility
// test and fails the checksum (design 9.2 item 4). found is false when nothing
// whole follows.
func resync(f *os.File, from, size int64) (next int64, found bool, err error) {
	buf := make([]byte, resyncChunk+recHeaderSize+MaxRecordBytes)
	for base := from + 1; base+recHeaderSize <= size; base += resyncChunk {
		n, err := f.ReadAt(buf[:min(int64(len(buf)), size-base)], base)
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, false, err
		}
		last := min(resyncChunk, n-recHeaderSize+1) // candidate offsets 0..last-1
		for i := 0; i < last; i++ {
			if buf[i+8] != typeLine && buf[i+8] != typeBoundary {
				continue
			}
			h := parseRecHeader(buf[i:])
			if h.length > MaxRecordBytes || i+recHeaderSize+int(h.length) > n {
				continue
			}
			c := crc32.Update(crc32.Checksum(buf[i:i+4], crcTable), crcTable, buf[i+8:i+recHeaderSize+int(h.length)])
			if c == h.crc {
				return base + int64(i), true, nil
			}
		}
		if n < len(buf) {
			break // the file ended inside this read
		}
	}
	return 0, false, nil
}

// scanSegment reads a segment file front to back, builds its index and finds
// where its whole records end. It never writes. A header that is torn or bad
// is reported in class and the index is empty; an unsupported one is an error.
func scanSegment(f *os.File, segno uint64) (scanResult, error) {
	info, err := f.Stat()
	if err != nil {
		return scanResult{}, err
	}
	res := scanResult{idx: newIdx(), size: info.Size()}
	head, err := readHead(f)
	if err != nil {
		return res, err
	}
	res.class, res.hdrLen, res.created, _ = classifyHeader(head, segno)
	if res.class == headerUnsupported {
		return res, unsupportedSegment(segno, "header")
	}
	if res.class != headerOK {
		return res, nil
	}
	res.validEnd, err = resume(f, &res, int64(res.hdrLen))
	if err != nil && errors.Is(err, ErrUnsupportedVersion) {
		err = unsupportedSegment(segno, "record")
	}
	return res, err
}

// resume continues a scan of an unsealed segment from start (the end of the
// last whole record a previous pass found), adding to res.idx.
func resume(f *os.File, res *scanResult, start int64) (int64, error) {
	end, spans, bad, err := walk(f, start, res.size, func(off int64, typ byte, body []byte, total int) error {
		switch typ {
		case typeLine:
			m, _, err := decodeLine(body)
			if err != nil {
				if errors.Is(err, ErrUnsupportedVersion) {
					return err
				}
				return fmt.Errorf("%w: %v", errBadRecord, err)
			}
			res.idx.add(off, total, &m)
		case typeBoundary:
			if _, err := decodeBoundary(body); err != nil {
				return fmt.Errorf("%w: %v", errBadRecord, err)
			}
			res.idx.nBound++
		}
		return nil
	})
	for _, sp := range spans {
		res.idx.addSpan(sp[0], sp[1])
	}
	if err != nil {
		return end, err
	}
	res.bad, res.torn = bad, bad != ""
	return end, nil
}

// readRecordAt reads and verifies the record at off, n bytes long.
func readRecordAt(f *os.File, off int64, n uint32) (byte, []byte, error) {
	if n < recHeaderSize || n > recHeaderSize+MaxRecordBytes {
		return 0, nil, corruptf("record of %d bytes at %d", n, off)
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, off); err != nil {
		return 0, nil, fmt.Errorf("%w: read record at %d: %v", ErrCorrupt, off, err)
	}
	h := parseRecHeader(buf)
	if int(h.length) != len(buf)-recHeaderSize || recordCRC(buf) != h.crc {
		return 0, nil, corruptf("record at %d fails its checksum", off)
	}
	return h.typ, buf[recHeaderSize:], nil
}

// openRegular opens a segment or index file, refusing a symlink or any
// non-regular file at the path.
func openRegular(path string, flag int, perm os.FileMode) (*os.File, error) {
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return nil, corruptf("%s is not a regular file", path)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return os.OpenFile(path, flag|openNoFollow, perm)
}

// requirePlainDir refuses a path that exists as anything but a real
// directory (a symlink above all), so no write is redirected elsewhere.
func requirePlainDir(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !fi.IsDir() {
		return corruptf("%s is not a plain directory", path)
	}
	return nil
}
