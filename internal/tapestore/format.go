// Package tapestore is the per-computer tape segment store: a read library
// that resolves an event, payload or pin receipt from a segment file first
// and from the legacy one-file-per-digest layout second, and a segment writer
// that exists for test fixtures and offline tools only.
//
// Stage A1 of docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md:
// no production code path writes a segment. The format is specified in
// appendix A of that document; this file is its implementation.
//
// The package uses only the standard library, so the offline readers
// (projectionbase, ledgerwitness) can import it without a cycle.
//
// Invariants:
//   - Every byte Read returns has been hashed with SHA-256 and compared with
//     the digest the caller asked for.
//   - A segment error is returned, never answered from the legacy layout.
//     Only "this digest is not in any segment" falls through to legacy.
package tapestore

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
)

// NS is the namespace byte of a record.
type NS uint8

const (
	NSEvent   NS = 1 // legacy sha256/computer-event/<digest>
	NSPayload NS = 2 // legacy sha256/computer-event-payload/<digest>
	NSReceipt NS = 3 // legacy sha256/pin-receipts/<digest>.json
)

func (n NS) valid() bool { return n >= NSEvent && n <= NSReceipt }

func (n NS) String() string {
	switch n {
	case NSEvent:
		return "event"
	case NSPayload:
		return "payload"
	case NSReceipt:
		return "receipt"
	}
	return fmt.Sprintf("ns(%d)", uint8(n))
}

// LegacyRel is the slash-separated path of a digest in the legacy layout,
// relative to the artifacts root.
func (n NS) LegacyRel(digest string) string {
	switch n {
	case NSEvent:
		return "sha256/computer-event/" + digest
	case NSPayload:
		return "sha256/computer-event-payload/" + digest
	case NSReceipt:
		return "sha256/pin-receipts/" + digest + ".json"
	}
	return ""
}

// Source says which layout served a read.
type Source int

const (
	SourceSegment Source = 1
	SourceLegacy  Source = 2
)

var (
	// ErrNotFound: the digest is in no segment and no legacy file.
	ErrNotFound = errors.New("tapestore: not found")
	// ErrCorrupt: a segment or index failed a check. Reader.Read answers it
	// from a legacy file only when that file hashes to the digest.
	ErrCorrupt = errors.New("tapestore: corrupt segment store")
	// ErrUnsupportedVersion: a segment, index or record carries a version
	// this binary does not know (a newer format, or a downgrade). It is also
	// ErrCorrupt, and it never falls back to legacy.
	ErrUnsupportedVersion = errors.New("tapestore: unsupported segment format version")
	// ErrDigestMismatch: bytes do not hash to the requested digest.
	ErrDigestMismatch = errors.New("tapestore: digest mismatch")
)

func corruptf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, fmt.Sprintf(format, args...))
}

func unsupportedf(format string, args ...any) error {
	return fmt.Errorf("%w: %w: %s", ErrCorrupt, ErrUnsupportedVersion, fmt.Sprintf(format, args...))
}

// Layout constants. All integers are big endian. See appendix A.
const (
	// SegDirName is the directory under the artifacts root.
	SegDirName = "tape-seg"

	// MaxRecordBytes bounds one record's body.
	MaxRecordBytes = 64 << 20
	// DefaultSealBytes and DefaultSealRecords are the rotation thresholds A2
	// uses (section 4.1). The reader does not depend on them.
	DefaultSealBytes   = 64 << 20
	DefaultSealRecords = 50_000

	segHeaderSize  = 32
	segVersion     = 1
	recHeaderSize  = 46
	recVersion     = 1    // body stored as is
	recVersionZstd = 2    // body stored as one zstd frame; the digest is over the decompressed bytes
	idxFixedSize   = 1064 // header 40 + fanout 256*4
	idxEntrySize   = 45
	idxTrailerSize = 4
	idxVersion     = 1
	maxSegmentSize = 1 << 40
)

var (
	segMagic = [4]byte{'C', 'H', 'T', 'S'}
	recMagic = [4]byte{'C', 'H', 'R', 'C'}
	idxMagic = [4]byte{'C', 'H', 'I', 'X'}

	castagnoli = crc32.MakeTable(crc32.Castagnoli)
)

// computerHash binds a segment and its index to a computer: the first 12
// bytes of SHA-256 over the computer id.
func computerHash(computerID string) [12]byte {
	sum := sha256.Sum256([]byte(computerID))
	var out [12]byte
	copy(out[:], sum[:12])
	return out
}

// ValidComputerID reports whether id can name a segment directory. The same
// rule gates the reader and the writer.
func ValidComputerID(id string) bool {
	if id == "" || len(id) > 200 || id == "." || id == ".." {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-' || c == ':':
			if i == 0 && c == '.' {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ValidDigest reports whether s is a lowercase 64-hex SHA-256.
func ValidDigest(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < 64; i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// SegmentFileName is seg-NNNNNNNN.seg.
func SegmentFileName(segno uint64) string { return fmt.Sprintf("seg-%08d.seg", segno) }

// IndexFileName is seg-NNNNNNNN.idx.
func IndexFileName(segno uint64) string { return fmt.Sprintf("seg-%08d.idx", segno) }

// parseSegName parses seg-NNNNNNNN.(seg|idx).
func parseSegName(name string) (segno uint64, ext string, ok bool) {
	if !strings.HasPrefix(name, "seg-") {
		return 0, "", false
	}
	rest := name[len("seg-"):]
	dot := strings.IndexByte(rest, '.')
	if dot < 8 {
		return 0, "", false
	}
	digits, ext := rest[:dot], rest[dot+1:]
	if ext != "seg" && ext != "idx" {
		return 0, "", false
	}
	var n uint64
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		if c < '0' || c > '9' || n > (1<<60) {
			return 0, "", false
		}
		n = n*10 + uint64(c-'0')
	}
	// Only the canonical spelling names a segment, so one segment has one
	// pair of file names.
	if name != SegmentFileName(n) && name != IndexFileName(n) {
		return 0, "", false
	}
	return n, ext, true
}

func digestBytes(b []byte) [32]byte { return sha256.Sum256(b) }

func hexDigest(d [32]byte) string { return hex.EncodeToString(d[:]) }

// --- segment header --------------------------------------------------------

func encodeSegHeader(segno uint64, computerID string) []byte {
	b := make([]byte, segHeaderSize)
	copy(b[0:4], segMagic[:])
	b[4] = segVersion
	binary.BigEndian.PutUint64(b[8:16], segno)
	h := computerHash(computerID)
	copy(b[16:28], h[:])
	binary.BigEndian.PutUint32(b[28:32], crc32.Checksum(b[0:28], castagnoli))
	return b
}

func checkSegHeader(b []byte, segno uint64, computerID string) error {
	if len(b) < segHeaderSize {
		return corruptf("segment %d: header truncated (%d bytes)", segno, len(b))
	}
	if [4]byte(b[0:4]) != segMagic {
		return corruptf("segment %d: bad magic", segno)
	}
	if b[4] != segVersion {
		return unsupportedf("segment %d: unsupported version %d", segno, b[4])
	}
	if crc32.Checksum(b[0:28], castagnoli) != binary.BigEndian.Uint32(b[28:32]) {
		return corruptf("segment %d: header crc mismatch", segno)
	}
	if got := binary.BigEndian.Uint64(b[8:16]); got != segno {
		return corruptf("segment %d: header names segment %d", segno, got)
	}
	want := computerHash(computerID)
	if [12]byte(b[16:28]) != want {
		return corruptf("segment %d: belongs to a different computer", segno)
	}
	return nil
}

// --- records -----------------------------------------------------------------

func encodeRecord(ns NS, digest [32]byte, body []byte) []byte {
	return encodeRecordVer(recVersion, ns, digest, body)
}

// encodeRecordVer frames body (the bytes that go on disk) under version.
func encodeRecordVer(version byte, ns NS, digest [32]byte, body []byte) []byte {
	b := make([]byte, recHeaderSize+len(body))
	copy(b[0:4], recMagic[:])
	b[4] = version
	b[5] = byte(ns)
	copy(b[6:38], digest[:])
	binary.BigEndian.PutUint32(b[38:42], uint32(len(body)))
	copy(b[recHeaderSize:], body)
	binary.BigEndian.PutUint32(b[42:46], recordCRC(b[4:42], body))
	return b
}

// recordCRC covers the version, namespace, digest and length fields, then the
// body: every byte of the record except the magic and the crc itself.
func recordCRC(header []byte, body []byte) uint32 {
	c := crc32.Update(0, castagnoli, header)
	return crc32.Update(c, castagnoli, body)
}

type recHeader struct {
	version byte
	ns      NS
	digest  [32]byte
	length  uint32
	crc     uint32
}

// parseRecHeader checks magic, version, namespace and length bound. It does
// not check the crc (the body is needed for that).
func parseRecHeader(b []byte) (recHeader, error) {
	h, err := parseRecHeaderLoose(b)
	if err != nil {
		return h, err
	}
	if b[4] != recVersion && b[4] != recVersionZstd {
		return h, fmt.Errorf("%w: record version %d", ErrUnsupportedVersion, b[4])
	}
	if !h.ns.valid() {
		return h, fmt.Errorf("unknown record namespace %d", b[5])
	}
	return h, nil
}

// parseRecHeaderLoose checks only the magic and the length bound, so a
// record with an unsupported version or namespace can still be crc-checked
// and reported as unsupported rather than mistaken for a torn write.
func parseRecHeaderLoose(b []byte) (recHeader, error) {
	var h recHeader
	if len(b) < recHeaderSize {
		return h, errors.New("short record header")
	}
	if [4]byte(b[0:4]) != recMagic {
		return h, errors.New("bad record magic")
	}
	h.version = b[4]
	h.ns = NS(b[5])
	copy(h.digest[:], b[6:38])
	h.length = binary.BigEndian.Uint32(b[38:42])
	if h.length > MaxRecordBytes {
		return h, fmt.Errorf("record length %d above limit", h.length)
	}
	h.crc = binary.BigEndian.Uint32(b[42:46])
	return h, nil
}

// --- index -----------------------------------------------------------------

type idxEntry struct {
	digest [32]byte
	ns     NS
	off    uint64
	length uint32
}

func encodeIdxEntry(b []byte, e idxEntry) {
	copy(b[0:32], e.digest[:])
	b[32] = byte(e.ns)
	binary.BigEndian.PutUint64(b[33:41], e.off)
	binary.BigEndian.PutUint32(b[41:45], e.length)
}

func decodeIdxEntry(b []byte) idxEntry {
	var e idxEntry
	copy(e.digest[:], b[0:32])
	e.ns = NS(b[32])
	e.off = binary.BigEndian.Uint64(b[33:41])
	e.length = binary.BigEndian.Uint32(b[41:45])
	return e
}

// encodeIndex builds a complete .idx file. entries must be sorted by
// (digest, ns) and unique.
func encodeIndex(segno uint64, computerID string, segSize uint64, entries []idxEntry) []byte {
	b := make([]byte, idxFixedSize+idxEntrySize*len(entries)+idxTrailerSize)
	copy(b[0:4], idxMagic[:])
	b[4] = idxVersion
	binary.BigEndian.PutUint64(b[8:16], segno)
	binary.BigEndian.PutUint64(b[16:24], segSize)
	binary.BigEndian.PutUint32(b[24:28], uint32(len(entries)))
	h := computerHash(computerID)
	copy(b[28:40], h[:])
	var counts [256]uint32
	for _, e := range entries {
		counts[e.digest[0]]++
	}
	var cum uint32
	for i := 0; i < 256; i++ {
		cum += counts[i]
		binary.BigEndian.PutUint32(b[40+4*i:44+4*i], cum)
	}
	for i, e := range entries {
		encodeIdxEntry(b[idxFixedSize+idxEntrySize*i:], e)
	}
	body := len(b) - idxTrailerSize
	binary.BigEndian.PutUint32(b[body:], crc32.Checksum(b[:body], castagnoli))
	return b
}
