package tapestore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// Compression of event and pin-receipt records.
//
// Failure modes this file must hold (tests in compress_test.go):
//   - an old record (version 1) reads exactly as before;
//   - a compressed record (version 2) round-trips and its digest is checked
//     over the DECOMPRESSED bytes: the digest is the object's identity on the
//     tape and in the Postgres receipts, and never covers the stored form;
//   - a tampered compressed body is refused (the record crc catches a flip
//     on disk; a recrc'd forgery decompresses to bytes that do not hash to
//     the digest, or fails to decode);
//   - a corrupt zstd frame is an ErrCorrupt error, never a panic;
//   - a decompression bomb is capped at MaxRecordBytes of output;
//   - one segment may hold version 1 and version 2 records side by side, and
//     scan, view, the reader and writer recovery all handle the mix;
//   - an older binary meets version 2 as an unsupported record version and
//     fails closed (ErrUnsupportedVersion), never as a torn write.
//
// The index entry length is the STORED length, so a reader can bound the
// read; the digest in the index and the record header is the uncompressed
// digest. Sealed payloads are ciphertext and are never compressed.

const (
	// minCompressBytes: records shorter than this are stored as is.
	minCompressBytes = 200
	// minSavingsPct: a record that compresses by less than this stays raw.
	minSavingsPct = 10
)

var (
	zstdOnce sync.Once
	zstdEnc  *zstd.Encoder
	zstdDec  *zstd.Decoder
)

func zstdInit() {
	zstdOnce.Do(func() {
		var err error
		zstdEnc, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithEncoderConcurrency(4))
		if err != nil {
			panic(fmt.Sprintf("tapestore: zstd encoder: %v", err))
		}
		zstdDec, err = zstd.NewReader(nil, zstd.WithDecoderConcurrency(4), zstd.WithDecoderMaxMemory(MaxRecordBytes))
		if err != nil {
			panic(fmt.Sprintf("tapestore: zstd decoder: %v", err))
		}
	})
}

// compressible reports whether a namespace carries plain JSON worth
// compressing. Payloads are sealed ciphertext.
func compressible(ns NS) bool { return ns == NSEvent || ns == NSReceipt }

// packRecord frames body for writing: zstd when the namespace is compressible,
// the body is large enough and the saving is worth it, raw otherwise.
func packRecord(ns NS, digest [32]byte, body []byte) []byte {
	if compressible(ns) && len(body) >= minCompressBytes {
		zstdInit()
		c := zstdEnc.EncodeAll(body, make([]byte, 0, len(body)/2))
		if len(c) > 0 && len(c)*100 <= len(body)*(100-minSavingsPct) {
			return encodeRecordVer(recVersionZstd, ns, digest, c)
		}
	}
	return encodeRecord(ns, digest, body)
}

// decodeBody turns a record's stored bytes into the object bytes and checks
// them against the digest. It never panics on hostile input.
func decodeBody(version byte, stored []byte, want [32]byte) (out []byte, err error) {
	switch version {
	case recVersion:
		out = stored
	case recVersionZstd:
		zstdInit()
		defer func() {
			if r := recover(); r != nil {
				out, err = nil, fmt.Errorf("zstd decode panicked: %v", r)
			}
		}()
		out, err = zstdDec.DecodeAll(stored, nil)
		if err != nil {
			if errors.Is(err, zstd.ErrDecoderSizeExceeded) {
				return nil, fmt.Errorf("decompressed body exceeds the %d byte limit", MaxRecordBytes)
			}
			return nil, fmt.Errorf("zstd body does not decode: %v", err)
		}
		if len(out) > MaxRecordBytes {
			return nil, fmt.Errorf("decompressed body of %d bytes exceeds the %d byte limit", len(out), MaxRecordBytes)
		}
	default:
		return nil, fmt.Errorf("%w: record version %d", ErrUnsupportedVersion, version)
	}
	if sha256.Sum256(out) != want {
		return nil, fmt.Errorf("body does not hash to its digest %x", want)
	}
	return out, nil
}
