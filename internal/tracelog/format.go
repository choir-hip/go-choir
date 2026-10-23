package tracelog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"time"
)

// Errors. ErrCorrupt wraps every failed check of a file's own bytes.
var (
	// ErrPoisoned is returned by every call of a Log after its first failed
	// write or fsync (the fail-stop rule; see Log).
	ErrPoisoned = errors.New("tracelog: log poisoned by an earlier write or fsync failure")
	// ErrLocked: another writer, in this process or another, holds the directory.
	ErrLocked = errors.New("tracelog: directory is locked by another writer")
	// ErrClosed: the Log was closed.
	ErrClosed = errors.New("tracelog: log closed")
	// ErrTooLarge: a record would exceed MaxRecordBytes. Nothing was written.
	ErrTooLarge = errors.New("tracelog: record exceeds the size limit")
	// ErrCorrupt: a file failed a check of its own bytes.
	ErrCorrupt = errors.New("tracelog: corrupt log file")
	// ErrUnsupportedVersion: a segment or index carries a version this binary
	// does not know. It is also ErrCorrupt and is never repaired or set aside.
	ErrUnsupportedVersion = errors.New("tracelog: unsupported format version")
)

// unsupportedSegment is the error for a segment this binary cannot read.
func unsupportedSegment(segno uint64, what string) error {
	return fmt.Errorf("%w: %w: segment %d %s", ErrCorrupt, ErrUnsupportedVersion, segno, what)
}

func corruptf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrCorrupt}, args...)...)
}

// Layout. All integers are big endian.
const (
	// MaxRecordBytes bounds one record's body: the envelope and the line.
	MaxRecordBytes = 1 << 20

	// Defaults for Options.
	DefaultSealAge      = time.Hour
	DefaultSealBytes    = 64 << 20
	DefaultSealRecords  = 50_000
	DefaultSyncInterval = time.Second

	segMagic      = "CHTRACE1"
	idxMagic      = "CHTIDX01"
	formatVersion = 1

	recHeaderSize = 9 // u32 length, u32 crc32c, u8 type
	typeLine      = 1
	typeBoundary  = 2
	envVersion    = 1

	maxComputerIDLen = 256
	segHeaderFixed   = 8 + 4 + 8 + 8 + 2 // magic, version, segno, created, cid length
	segHeaderMax     = segHeaderFixed + maxComputerIDLen + 4
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Boundary reasons.
const (
	ReasonOpen           = "open"
	ReasonCrashRecovered = "crash_recovered"
	ReasonRestore        = "restore"
	ReasonSettingOn      = "setting_on"
	ReasonSettingOff     = "setting_off"
)

// Meta is what the log needs to know about a line without decoding it. The
// caller (the store) fills it from the event record and passes the record's
// serialized form as the line.
type Meta struct {
	EventID      string
	RunID        string
	OwnerID      string
	TrajectoryID string
	Kind         string
	Seq          int64
	StreamSeq    int64
	Time         time.Time
	// AfterTapeSeq is the appender's acknowledged head sequence when the line
	// was written; BootID names the process.
	AfterTapeSeq int64
	BootID       string
}

// Boundary is a record that marks a change in how the log was written.
type Boundary struct {
	Reason  string    `json:"reason"`
	HeadSeq int64     `json:"head_seq"`
	At      time.Time `json:"at"`
}

func segmentName(n uint64) string { return fmt.Sprintf("seg-%08d.tlog", n) }
func indexName(n uint64) string   { return fmt.Sprintf("seg-%08d.tidx", n) }

// parseSegName reports the segment number of a seg-NNNNNNNN.<ext> name.
func parseSegName(name, ext string) (uint64, bool) {
	const prefix = "seg-"
	if len(name) != len(prefix)+8+len(ext) || name[:len(prefix)] != prefix || name[len(prefix)+8:] != ext {
		return 0, false
	}
	var n uint64
	for _, c := range name[len(prefix) : len(prefix)+8] {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, true
}

// --- segment header --------------------------------------------------------

func encodeSegHeader(segno uint64, created time.Time, computerID string) []byte {
	if len(computerID) > maxComputerIDLen {
		computerID = computerID[:maxComputerIDLen]
	}
	b := make([]byte, 0, segHeaderFixed+len(computerID)+4)
	b = append(b, segMagic...)
	b = binary.BigEndian.AppendUint32(b, formatVersion)
	b = binary.BigEndian.AppendUint64(b, segno)
	b = binary.BigEndian.AppendUint64(b, uint64(created.UnixNano()))
	b = binary.BigEndian.AppendUint16(b, uint16(len(computerID)))
	b = append(b, computerID...)
	return binary.BigEndian.AppendUint32(b, crc32.Checksum(b, crcTable))
}

type headerClass int

const (
	headerOK headerClass = iota
	headerTorn
	headerBad
	headerUnsupported
)

// classifyHeader says what a segment file's first bytes are. head holds up to
// segHeaderMax bytes of the file. A file shorter than a whole header whose
// bytes so far agree with the header this segment would have is torn: the
// crash came before the header was durable, so no record was ever
// acknowledged in it. Anything else that is not a valid header is bad.
func classifyHeader(head []byte, segno uint64) (class headerClass, hdrLen int, created time.Time, computerID string) {
	want := make([]byte, 0, 20)
	want = append(want, segMagic...)
	want = binary.BigEndian.AppendUint32(want, formatVersion)
	want = binary.BigEndian.AppendUint64(want, segno)
	n := len(head)
	if n > len(want) {
		n = len(want)
	}
	if string(head[:min(len(head), 8)]) != segMagic[:min(len(head), 8)] {
		return headerBad, 0, time.Time{}, ""
	}
	if len(head) >= 12 {
		// Our magic with any other version is not foreign: it is a format this
		// binary does not know (older or newer), never to be set aside.
		if v := binary.BigEndian.Uint32(head[8:12]); v != formatVersion {
			return headerUnsupported, 0, time.Time{}, ""
		}
	}
	if string(head[:n]) != string(want[:n]) {
		return headerBad, 0, time.Time{}, ""
	}
	if len(head) < segHeaderFixed {
		return headerTorn, 0, time.Time{}, ""
	}
	cidLen := int(binary.BigEndian.Uint16(head[segHeaderFixed-2 : segHeaderFixed]))
	if cidLen > maxComputerIDLen {
		return headerBad, 0, time.Time{}, ""
	}
	total := segHeaderFixed + cidLen + 4
	if len(head) < total {
		return headerTorn, 0, time.Time{}, ""
	}
	if crc32.Checksum(head[:total-4], crcTable) != binary.BigEndian.Uint32(head[total-4:total]) {
		return headerBad, 0, time.Time{}, ""
	}
	created = time.Unix(0, int64(binary.BigEndian.Uint64(head[20:28]))).UTC()
	return headerOK, total, created, string(head[segHeaderFixed : segHeaderFixed+cidLen])
}

// --- records ---------------------------------------------------------------

const noTime = math.MinInt64

func timeNanos(t time.Time) int64 {
	if t.IsZero() {
		return noTime
	}
	return t.UnixNano()
}

func nanosTime(n int64) time.Time {
	if n == noTime {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}

func appendStr(b []byte, s string) []byte {
	b = binary.AppendUvarint(b, uint64(len(s)))
	return append(b, s...)
}

// finishRecord fills in the length and checksum of a record whose body starts
// at recHeaderSize.
func finishRecord(b []byte) []byte {
	binary.BigEndian.PutUint32(b[0:4], uint32(len(b)-recHeaderSize))
	binary.BigEndian.PutUint32(b[4:8], recordCRC(b))
	return b
}

// recordCRC covers the length, the type and the body, so a damaged length is
// caught as well as damaged bytes.
func recordCRC(b []byte) uint32 {
	return crc32.Update(crc32.Checksum(b[0:4], crcTable), crcTable, b[8:])
}

// encodeLine appends a line record (header, envelope, line) to buf[:0].
func encodeLine(buf []byte, m *Meta, line []byte) ([]byte, error) {
	buf = append(buf[:0], 0, 0, 0, 0, 0, 0, 0, 0, typeLine, envVersion)
	buf = appendStr(buf, m.EventID)
	buf = appendStr(buf, m.RunID)
	buf = appendStr(buf, m.OwnerID)
	buf = appendStr(buf, m.TrajectoryID)
	buf = appendStr(buf, m.Kind)
	buf = appendStr(buf, m.BootID)
	buf = binary.AppendVarint(buf, m.Seq)
	buf = binary.AppendVarint(buf, m.StreamSeq)
	buf = binary.AppendVarint(buf, timeNanos(m.Time))
	buf = binary.AppendVarint(buf, m.AfterTapeSeq)
	if len(buf)-recHeaderSize+len(line) > MaxRecordBytes {
		return buf, ErrTooLarge
	}
	buf = append(buf, line...)
	return finishRecord(buf), nil
}

func encodeBoundary(buf []byte, b Boundary) ([]byte, error) {
	if len(b.Reason) > MaxRecordBytes/2 {
		return buf, ErrTooLarge
	}
	buf = append(buf[:0], 0, 0, 0, 0, 0, 0, 0, 0, typeBoundary)
	buf = binary.AppendUvarint(buf, uint64(len(b.Reason)))
	buf = append(buf, b.Reason...)
	buf = binary.AppendVarint(buf, b.HeadSeq)
	buf = binary.AppendVarint(buf, timeNanos(b.At))
	if len(buf)-recHeaderSize > MaxRecordBytes {
		return buf, ErrTooLarge
	}
	return finishRecord(buf), nil
}

type recHeader struct {
	length uint32
	crc    uint32
	typ    byte
}

func parseRecHeader(b []byte) recHeader {
	return recHeader{length: binary.BigEndian.Uint32(b[0:4]), crc: binary.BigEndian.Uint32(b[4:8]), typ: b[8]}
}

// decoder reads the fields of an envelope.
type decoder struct {
	b   []byte
	err error
}

func (d *decoder) uvarint() uint64 {
	if d.err != nil {
		return 0
	}
	v, n := binary.Uvarint(d.b)
	if n <= 0 {
		d.err = corruptf("bad varint")
		return 0
	}
	d.b = d.b[n:]
	return v
}

func (d *decoder) varint() int64 {
	if d.err != nil {
		return 0
	}
	v, n := binary.Varint(d.b)
	if n <= 0 {
		d.err = corruptf("bad varint")
		return 0
	}
	d.b = d.b[n:]
	return v
}

func (d *decoder) str() string {
	n := d.uvarint()
	if d.err != nil {
		return ""
	}
	if n > uint64(len(d.b)) {
		d.err = corruptf("string of %d bytes in %d", n, len(d.b))
		return ""
	}
	s := string(d.b[:n])
	d.b = d.b[n:]
	return s
}

// decodeLine parses a type 1 body into its Meta and the line bytes (a slice of
// body, not a copy).
func decodeLine(body []byte) (Meta, []byte, error) {
	if len(body) < 1 {
		return Meta{}, nil, corruptf("empty line record")
	}
	if body[0] != envVersion {
		return Meta{}, nil, fmt.Errorf("%w: %w: envelope version %d", ErrCorrupt, ErrUnsupportedVersion, body[0])
	}
	d := decoder{b: body[1:]}
	var m Meta
	m.EventID, m.RunID, m.OwnerID, m.TrajectoryID, m.Kind, m.BootID = d.str(), d.str(), d.str(), d.str(), d.str(), d.str()
	m.Seq, m.StreamSeq = d.varint(), d.varint()
	m.Time = nanosTime(d.varint())
	m.AfterTapeSeq = d.varint()
	if d.err != nil {
		return Meta{}, nil, d.err
	}
	return m, d.b, nil
}

func decodeBoundary(body []byte) (Boundary, error) {
	d := decoder{b: body}
	n := d.uvarint()
	if d.err != nil || n > uint64(len(d.b)) {
		return Boundary{}, corruptf("bad boundary record")
	}
	var b Boundary
	b.Reason = string(d.b[:n])
	d.b = d.b[n:]
	b.HeadSeq = d.varint()
	b.At = nanosTime(d.varint())
	if d.err != nil {
		return Boundary{}, d.err
	}
	return b, nil
}
