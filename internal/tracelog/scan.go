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

// end states of a pass over a segment's records.
type endState int

const (
	endClean   endState = iota // the records ran to the end of the file
	endTorn                    // a partial or invalid record at the end, nothing valid after it
	endDamaged                 // an invalid record followed by a valid one
)

// damageWindow is how far past an invalid record the scan looks for a valid
// one, to tell a torn tail from damage in the middle of a segment. A record
// is at most MaxRecordBytes, so a few of them cover any tail a crash leaves.
const damageWindow = 4 << 20

type scanResult struct {
	idx      *segIdx
	hdrLen   int
	created  time.Time
	validEnd int64
	size     int64
	state    endState
	bad      string
	class    headerClass
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
// each, and stops at the first record that is short or invalid. It returns
// the end of the last whole record and, when it stopped early, why. Records of
// a type this binary does not know are skipped. An unreadable file is an error.
func walk(f *os.File, start, size int64, visit func(off int64, typ byte, body []byte, total int) error) (end int64, bad string, err error) {
	r := bufio.NewReaderSize(io.NewSectionReader(f, start, size-start), 256<<10)
	var hdr [recHeaderSize]byte
	var body []byte
	off := start
	for off < size {
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return off, "short record header", nil
			}
			return off, "", err
		}
		h := parseRecHeader(hdr[:])
		if h.length > MaxRecordBytes {
			return off, fmt.Sprintf("record length %d", h.length), nil
		}
		if int64(h.length) > size-off-recHeaderSize {
			return off, "record runs past the end of the file", nil
		}
		if cap(body) < int(h.length) {
			body = make([]byte, h.length, max(int(h.length), 4096))
		}
		body = body[:h.length]
		if _, err := io.ReadFull(r, body); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return off, "short record body", nil
			}
			return off, "", err
		}
		c := crc32.Update(crc32.Checksum(hdr[0:4], crcTable), crcTable, hdr[8:9])
		if crc32.Update(c, crcTable, body) != h.crc {
			return off, "record checksum", nil
		}
		total := recHeaderSize + int(h.length)
		if h.typ == typeLine || h.typ == typeBoundary {
			if err := visit(off, h.typ, body, total); err != nil {
				if errors.Is(err, errBadRecord) {
					return off, err.Error(), nil
				}
				return off, "", err
			}
		}
		off += int64(total)
	}
	return off, "", nil
}

// errBadRecord marks a record whose checksum held but whose contents did not
// parse: it ends the whole records like a checksum failure does.
var errBadRecord = errors.New("record contents")

// validRecordAfter reports whether a whole, checksummed record starts anywhere
// in the window after from.
func validRecordAfter(f *os.File, from, size int64) (bool, error) {
	n := min(size-from-1, int64(damageWindow))
	if n < recHeaderSize {
		return false, nil
	}
	buf := make([]byte, n)
	if _, err := f.ReadAt(buf, from+1); err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	for i := 0; i+recHeaderSize <= len(buf); i++ {
		if buf[i+8] != typeLine && buf[i+8] != typeBoundary {
			continue
		}
		h := parseRecHeader(buf[i:])
		if h.length > MaxRecordBytes || i+recHeaderSize+int(h.length) > len(buf) {
			continue
		}
		c := crc32.Update(crc32.Checksum(buf[i:i+4], crcTable), crcTable, buf[i+8:i+recHeaderSize+int(h.length)])
		if c == h.crc {
			return true, nil
		}
	}
	return false, nil
}

// scanSegment reads a segment file front to back, builds its index and finds
// where its whole records end. It never writes. A header that is torn or bad
// is reported in class and the index is empty.
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
	var cid string
	res.class, res.hdrLen, res.created, cid = classifyHeader(head, segno)
	_ = cid
	if res.class != headerOK {
		return res, nil
	}
	res.validEnd, err = resume(f, &res, int64(res.hdrLen))
	return res, err
}

// resume continues a scan of an unsealed segment from start (the end of the
// last whole record a previous pass found), adding to res.idx.
func resume(f *os.File, res *scanResult, start int64) (int64, error) {
	end, bad, err := walk(f, start, res.size, func(off int64, typ byte, body []byte, total int) error {
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
	if err != nil {
		return end, err
	}
	res.bad = bad
	switch {
	case bad == "":
		res.state = endClean
	default:
		later, err := validRecordAfter(f, end, res.size)
		if err != nil {
			return end, err
		}
		if later {
			res.state = endDamaged
		} else {
			res.state = endTorn
		}
	}
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
