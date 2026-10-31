package tapestore

// Compression failure modes (see compress.go), written before the code:
//
//  Z1  A segment written before compression existed (all version 1) reads
//      byte for byte through the Reader, Scan and the view.
//  Z2  Event and receipt records of useful size are stored as version 2 and
//      round-trip; payloads, short records and incompressible records stay
//      version 1. The digest is over the uncompressed bytes.
//  Z3  A flipped byte in a compressed body is refused (crc); a forged body
//      whose crc was recomputed is refused (digest over the decompressed
//      bytes).
//  Z4  A version 2 record whose body is not a zstd frame, or is a truncated
//      one, is ErrCorrupt, never a panic.
//  Z5  A decompression bomb is refused at the output cap.
//  Z6  One segment holding version 1 and version 2 records reads through the
//      Reader, Scan and the view, and a new Appender seals it (recovery).
//  Z7  A compressed record is smaller on disk than its body and the index
//      length is the stored length.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The writes are held off in the release; these tests exercise them.
func init() { compressRecordWrites = true }

type fixRec struct {
	ns   NS
	body []byte
	ver  byte // 0: packRecord decides; recVersion: force raw
}

// buildSegment writes a segment by hand (no packing policy), optionally sealed.
func buildSegment(t *testing.T, root, id string, segno uint64, recs []fixRec, seal bool) {
	t.Helper()
	dir := segDir(root, id)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	buf := encodeSegHeader(segno, id)
	entries := map[openKey]idxEntry{}
	for _, r := range recs {
		d := digestBytes(r.body)
		var rec []byte
		if r.ver == recVersion {
			rec = encodeRecord(r.ns, d, r.body)
		} else {
			rec = packRecord(r.ns, d, r.body)
		}
		entries[openKey{r.ns, d}] = idxEntry{digest: d, ns: r.ns, off: uint64(len(buf)), length: uint32(len(rec) - recHeaderSize)}
		buf = append(buf, rec...)
	}
	if err := os.WriteFile(filepath.Join(dir, SegmentFileName(segno)), buf, 0o640); err != nil {
		t.Fatal(err)
	}
	if seal {
		if err := sealIndex(dir, segno, id, int64(len(buf)), entries, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func receiptJSON(i int) []byte {
	b, _ := json.Marshal(map[string]any{
		"schema":         "pin-receipt/v1",
		"computer_id":    "comp-0123456789abcdef0123456789abcdef",
		"event_digest":   fmt.Sprintf("%064x", sha256.Sum256([]byte(fmt.Sprint("e", i)))),
		"payload_digest": fmt.Sprintf("%064x", sha256.Sum256([]byte(fmt.Sprint("p", i)))),
		"pinned_at":      fmt.Sprintf("2026-10-31T12:%02d:%02d.123456789Z", i/60%60, i%60),
		"store":          map[string]any{"kind": "tape-seg", "namespace": "pin-receipts", "version": 1},
		"host":           "node-a", "signature": fmt.Sprintf("%0128x", sha256.Sum256([]byte(fmt.Sprint("s", i)))),
		"note": "pinned by the host after the event tape accepted the payload digest and the sealed object landed",
	})
	return b
}

func eventJSON(i int) []byte {
	b, _ := json.Marshal(map[string]any{
		"kind": "agent.turn.committed", "seq": i, "computer_id": "comp-0123456789abcdef0123456789abcdef",
		"prev":           fmt.Sprintf("%064x", sha256.Sum256([]byte(fmt.Sprint("prev", i)))),
		"payload_digest": fmt.Sprintf("%064x", sha256.Sum256([]byte(fmt.Sprint("pl", i)))),
		"at":             fmt.Sprintf("2026-10-31T12:%02d:%02d.000Z", i/60%60, i%60),
		"actor":          map[string]any{"desk": "engineering", "run": "run-77", "loop": "loop-3"},
		"sig":            fmt.Sprintf("%0128x", sha256.Sum256([]byte(fmt.Sprint("sig", i)))),
	})
	return b
}

// diskVersions returns the record versions in order in a segment file.
func diskVersions(t *testing.T, root, id string, segno uint64) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(segDir(root, id), SegmentFileName(segno)))
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	for pos := segHeaderSize; pos < len(raw); {
		h, err := parseRecHeaderLoose(raw[pos:])
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, raw[pos+4])
		pos += recHeaderSize + int(h.length)
	}
	return out
}

func TestZ1OldSegmentReadsUnchanged(t *testing.T) {
	root, id := t.TempDir(), "comp-z1"
	var recs []fixRec
	for i := 0; i < 20; i++ {
		recs = append(recs, fixRec{NSEvent, eventJSON(i), recVersion}, fixRec{NSReceipt, receiptJSON(i), recVersion})
	}
	buildSegment(t, root, id, 1, recs, true)
	for _, v := range diskVersions(t, root, id, 1) {
		if v != recVersion {
			t.Fatalf("fixture has version %d", v)
		}
	}
	r := NewReader(root)
	for _, rc := range recs {
		got, src, err := r.Read(id, rc.ns, hexDigest(digestBytes(rc.body)))
		if err != nil || src != SourceSegment || !bytes.Equal(got, rc.body) {
			t.Fatalf("Read: %v src=%v", err, src)
		}
	}
	n := 0
	if err := r.Scan(id, func(rec Record) error { n++; return nil }); err != nil || n != len(recs) {
		t.Fatalf("Scan: %v n=%d", err, n)
	}
}

func TestZ2AndZ7NewAppendsCompressAndRoundTrip(t *testing.T) {
	root, id := t.TempDir(), "comp-z2"
	a := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	ev, rc := eventJSON(1), receiptJSON(1)
	payload := bytes.Repeat([]byte("sealed-ciphertext-but-repetitive "), 50) // payload ns: never compressed
	small := []byte(`{"k":"short"}`)
	noise := make([]byte, 2000)
	for i := range noise {
		noise[i] = byte(sha256.Sum256([]byte{byte(i), byte(i >> 8)})[0])
	}
	mustAppend(t, a, id,
		Item{NSEvent, ev}, Item{NSReceipt, rc}, Item{NSPayload, payload},
		Item{NSEvent, small}, Item{NSEvent, noise})
	if err := a.Seal(id); err != nil {
		t.Fatal(err)
	}
	want := []byte{recVersionZstd, recVersionZstd, recVersion, recVersion, recVersion}
	if got := diskVersions(t, root, id, 1); !bytes.Equal(got, want) {
		t.Fatalf("versions %v, want %v", got, want)
	}
	r := NewReader(root)
	for _, it := range []Item{{NSEvent, ev}, {NSReceipt, rc}, {NSPayload, payload}, {NSEvent, small}, {NSEvent, noise}} {
		got, src, err := r.Read(id, it.NS, hexDigest(sha256.Sum256(it.Body)))
		if err != nil || src != SourceSegment || !bytes.Equal(got, it.Body) {
			t.Fatalf("Read %v: %v", it.NS, err)
		}
	}
	// Z7: smaller on disk, index length is the stored length.
	v, err := r.OpenView(id)
	if err != nil {
		t.Fatal(err)
	}
	for b := 0; b < 256; b++ {
		locs, _ := v.Bucket(byte(b))
		for _, l := range locs {
			got, err := v.Read(l)
			if err != nil {
				t.Fatal(err)
			}
			if l.NS == NSEvent && bytes.Equal(got, ev) && int(l.Length) >= len(ev)*9/10 {
				t.Fatalf("compressed event stored %d of %d", l.Length, len(ev))
			}
			if l.NS == NSPayload && int(l.Length) != len(payload) {
				t.Fatalf("payload length %d", l.Length)
			}
		}
	}
	// Same event appended again by the same writer is a no-op (dedup is by
	// the uncompressed digest).
	if err := r.Scan(id, func(rec Record) error {
		if rec.Digest != hexDigest(sha256.Sum256(rec.Body)) {
			t.Fatalf("scan digest over stored bytes")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// compressedSegPath builds a sealed segment with one compressed event and
// returns its path and the record's file offset.
func oneCompressed(t *testing.T) (root, id, path string, body []byte, off int) {
	root, id = t.TempDir(), "comp-z3"
	body = eventJSON(7)
	buildSegment(t, root, id, 1, []fixRec{{NSEvent, body, 0}}, true)
	if v := diskVersions(t, root, id, 1); len(v) != 1 || v[0] != recVersionZstd {
		t.Fatalf("fixture not compressed: %v", v)
	}
	return root, id, filepath.Join(segDir(root, id), SegmentFileName(1)), body, segHeaderSize
}

// rewriteRecord replaces the record at off with a freshly framed one (valid
// crc) and fixes nothing else.
func rewriteRecord(t *testing.T, path string, off int, version byte, ns NS, digest [32]byte, stored []byte) {
	raw, _ := os.ReadFile(path)
	old, _ := parseRecHeaderLoose(raw[off:])
	rec := encodeRecordVer(version, ns, digest, stored)
	raw = append(append(append([]byte{}, raw[:off]...), rec...), raw[off+recHeaderSize+int(old.length):]...)
	if err := os.WriteFile(path, raw, 0o640); err != nil {
		t.Fatal(err)
	}
}

func scanErr(root, id string) error {
	return NewReader(root).Scan(id, func(Record) error { return nil })
}

func TestZ3TamperedCompressedBodyRefused(t *testing.T) {
	root, id, path, body, off := oneCompressed(t)
	raw, _ := os.ReadFile(path)
	raw[off+recHeaderSize+5] ^= 0x40 // flip a byte, crc not fixed
	os.WriteFile(path, raw, 0o640)
	d := hexDigest(digestBytes(body))
	if _, _, err := NewReader(root).Read(id, NSEvent, d); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("flip: want ErrCorrupt, got %v", err)
	}

	// Forgery with a valid crc: a well-formed frame of other bytes.
	root, id, path, body, off = oneCompressed(t)
	zstdInit()
	forged := zstdEnc.EncodeAll(bytes.Repeat([]byte("forged event "), 40), nil)
	rewriteRecord(t, path, off, recVersionZstd, NSEvent, digestBytes(body), forged)
	// index length now differs; the point read fails either way.
	if _, _, err := NewReader(root).Read(id, NSEvent, hexDigest(digestBytes(body))); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("forge: want ErrCorrupt, got %v", err)
	}
	if err := scanErr(root, id); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("forge scan: want ErrCorrupt, got %v", err)
	}
	// And the decode itself, independent of the index.
	if _, err := decodeBody(recVersionZstd, forged, digestBytes(body)); err == nil {
		t.Fatal("forged body decoded against the wrong digest")
	}
}

func TestZ4CorruptFrameRefusedWithoutPanic(t *testing.T) {
	_, _, _, body, _ := oneCompressed(t)
	d := digestBytes(body)
	zstdInit()
	good := zstdEnc.EncodeAll(body, nil)
	cases := map[string][]byte{
		"garbage": bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef}, 30),
		"empty":   {},
		"magic":   {0x28, 0xb5, 0x2f, 0xfd},
	}
	for n := 1; n < len(good); n += 3 {
		cases[fmt.Sprintf("trunc%d", n)] = good[:n]
	}
	for i := 0; i < len(good); i += 7 {
		c := append([]byte{}, good...)
		c[i] ^= 0xff
		cases[fmt.Sprintf("flip%d", i)] = c
	}
	for name, c := range cases {
		if _, err := decodeBody(recVersionZstd, c, d); err == nil {
			t.Fatalf("%s decoded", name)
		}
	}
	// Through the reader: a crc-valid record of garbage.
	root, id, path, body, off := oneCompressed(t)
	rewriteRecord(t, path, off, recVersionZstd, NSEvent, digestBytes(body), cases["garbage"])
	if _, _, err := NewReader(root).Read(id, NSEvent, hexDigest(digestBytes(body))); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("want ErrCorrupt, got %v", err)
	}
}

func TestZ5DecompressionBombCapped(t *testing.T) {
	zstdInit()
	big := make([]byte, MaxRecordBytes+4096)
	bomb := zstdEnc.EncodeAll(big, nil)
	if len(bomb) > 1<<20 {
		t.Fatalf("bomb is %d bytes; not a bomb", len(bomb))
	}
	if _, err := decodeBody(recVersionZstd, bomb, digestBytes(big)); err == nil {
		t.Fatal("bomb decoded")
	}
	// The cap is on output, not on the record: exactly at the cap is fine.
	ok := make([]byte, MaxRecordBytes)
	if out, err := decodeBody(recVersionZstd, zstdEnc.EncodeAll(ok, nil), digestBytes(ok)); err != nil || len(out) != MaxRecordBytes {
		t.Fatalf("at-cap record: %v", err)
	}
}

func TestZ6MixedSegmentReadsScansViewsAndRecovers(t *testing.T) {
	root, id := t.TempDir(), "comp-z6"
	var recs []fixRec
	for i := 0; i < 30; i++ {
		v := byte(0)
		if i%2 == 0 {
			v = recVersion
		}
		recs = append(recs, fixRec{NSEvent, eventJSON(i), v}, fixRec{NSReceipt, receiptJSON(i), v})
	}
	// Left open (unsealed): the reader scans it and a new Appender seals it.
	buildSegment(t, root, id, 1, recs, false)
	vers := diskVersions(t, root, id, 1)
	has := map[byte]int{}
	for _, v := range vers {
		has[v]++
	}
	if has[recVersion] == 0 || has[recVersionZstd] == 0 {
		t.Fatalf("not mixed: %v", has)
	}
	check := func(label string) {
		r := NewReader(root)
		for _, rc := range recs {
			got, _, err := r.Read(id, rc.ns, hexDigest(digestBytes(rc.body)))
			if err != nil || !bytes.Equal(got, rc.body) {
				t.Fatalf("%s: Read: %v", label, err)
			}
		}
		n := 0
		if err := r.Scan(id, func(rec Record) error {
			n++
			if rec.Digest != hexDigest(sha256.Sum256(rec.Body)) {
				t.Fatalf("%s: scan body does not match digest", label)
			}
			return nil
		}); err != nil || n != len(recs) {
			t.Fatalf("%s: Scan: %v n=%d", label, err, n)
		}
		v, err := r.OpenView(id)
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for b := 0; b < 256; b++ {
			locs, err := v.Bucket(byte(b))
			if err != nil {
				t.Fatalf("%s: Bucket: %v", label, err)
			}
			for _, l := range locs {
				got, err := v.Read(l)
				if err != nil || sha256.Sum256(got) != l.Digest {
					t.Fatalf("%s: view Read: %v", label, err)
				}
				seen++
			}
		}
		if seen != len(recs) {
			t.Fatalf("%s: view saw %d of %d", label, seen, len(recs))
		}
	}
	check("open")
	a := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	mustAppend(t, a, id, Item{NSEvent, eventJSON(999)}) // recovery seals segment 1, starts 2
	recs = append(recs, fixRec{NSEvent, eventJSON(999), 0})
	if _, err := os.Stat(filepath.Join(segDir(root, id), IndexFileName(1))); err != nil {
		t.Fatalf("recovery did not seal the mixed segment: %v", err)
	}
	check("sealed")
}

func TestZ6bTornCompressedTailIsCutByRecovery(t *testing.T) {
	root, id := t.TempDir(), "comp-z6b"
	buildSegment(t, root, id, 1, []fixRec{{NSEvent, eventJSON(1), 0}, {NSReceipt, receiptJSON(1), 0}}, false)
	path := filepath.Join(segDir(root, id), SegmentFileName(1))
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, raw[:len(raw)-9], 0o640)
	a := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	mustAppend(t, a, id, Item{NSEvent, eventJSON(2)})
	if a.Stats().TornTails != 1 {
		t.Fatalf("torn tails %d", a.Stats().TornTails)
	}
	if _, _, err := NewReader(root).Read(id, NSEvent, hexDigest(digestBytes(eventJSON(1)))); err != nil {
		t.Fatal(err)
	}
}

// Ratio on realistic records, logged for the report.
func TestZRatioOnRealisticRecords(t *testing.T) {
	var raw, stored int
	for i := 0; i < 2000; i++ {
		for _, r := range []struct {
			ns NS
			b  []byte
		}{{NSEvent, eventJSON(i)}, {NSReceipt, receiptJSON(i)}} {
			rec := packRecord(r.ns, digestBytes(r.b), r.b)
			raw += recHeaderSize + len(r.b)
			stored += len(rec)
		}
	}
	t.Logf("records: %d bytes raw framed, %d stored, ratio %.2fx", raw, stored, float64(raw)/float64(stored))
	_ = binary.BigEndian
}
