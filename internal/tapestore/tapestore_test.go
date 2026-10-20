package tapestore

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Failure-mode tests for the A1 segment reader. The numbered failure modes
// are section 9.4 of docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md.

const cid = "computer-a"

type rec struct {
	ns   NS
	body []byte
}

func body(format string, args ...any) []byte { return []byte(fmt.Sprintf(format, args...)) }

func sum(b []byte) string { d := sha256.Sum256(b); return hexDigest(d) }

func segDir(root, id string) string { return filepath.Join(root, SegDirName, id) }

// build writes one segment of records, optionally sealed, and returns the
// writer's entry table (offsets) for tamper helpers.
func build(t *testing.T, root, id string, segno uint64, seal bool, recs ...rec) map[openKey]idxEntry {
	t.Helper()
	w, err := CreateSegment(root, id, segno)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if _, err := w.Append(r.ns, r.body); err != nil {
			t.Fatal(err)
		}
	}
	entries := w.entries
	if seal {
		if err := w.Seal(); err != nil {
			t.Fatal(err)
		}
	} else if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return entries
}

func legacyPut(t *testing.T, root string, ns NS, b []byte) string {
	t.Helper()
	d := sum(b)
	p := filepath.Join(root, filepath.FromSlash(ns.LegacyRel(d)))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o640); err != nil {
		t.Fatal(err)
	}
	return d
}

func segPath(root, id string, segno uint64) string {
	return filepath.Join(segDir(root, id), SegmentFileName(segno))
}

func idxPath(root, id string, segno uint64) string {
	return filepath.Join(segDir(root, id), IndexFileName(segno))
}

func patchFile(t *testing.T, path string, fn func([]byte) []byte) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, fn(b), 0o640); err != nil {
		t.Fatal(err)
	}
}

// patchIdx edits an index and recomputes its trailer crc, so only the edited
// property (not the crc) is what the reader must catch.
func patchIdx(t *testing.T, path string, fn func([]byte)) {
	t.Helper()
	patchFile(t, path, func(b []byte) []byte {
		fn(b)
		n := len(b) - idxTrailerSize
		binary.BigEndian.PutUint32(b[n:], crc32.Checksum(b[:n], castagnoli))
		return b
	})
}

// refixRecordCRC recomputes the crc of the record at off.
func refixRecordCRC(b []byte, off int) {
	length := int(binary.BigEndian.Uint32(b[off+38 : off+42]))
	binary.BigEndian.PutUint32(b[off+42:off+46], recordCRC(b[off+4:off+42], b[off+recHeaderSize:off+recHeaderSize+length]))
}

func mustRead(t *testing.T, r *Reader, id string, ns NS, want []byte) Source {
	t.Helper()
	got, src, err := r.Read(id, ns, sum(want))
	if err != nil {
		t.Fatalf("Read %s: %v", ns, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Read %s returned different bytes", ns)
	}
	return src
}

func mustCorrupt(t *testing.T, r *Reader, id string, ns NS, want []byte) {
	t.Helper()
	got, _, err := r.Read(id, ns, sum(want))
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Read = %d bytes, err %v; want ErrCorrupt", len(got), err)
	}
	if got != nil {
		t.Fatal("a corrupt read returned bytes")
	}
}

func TestSegmentRoundTripSealedAndOpen(t *testing.T) {
	root := t.TempDir()
	ev, pl, rc := body(`{"event":1}`), body("payload one"), body(`{"receipt":1}`)
	ev2, pl2 := body(`{"event":2}`), body("payload two")
	build(t, root, cid, 1, true, rec{NSEvent, ev}, rec{NSPayload, pl}, rec{NSReceipt, rc})
	build(t, root, cid, 2, false, rec{NSEvent, ev2}, rec{NSPayload, pl2})
	r := NewReader(root)
	for _, c := range []struct {
		ns NS
		b  []byte
	}{{NSEvent, ev}, {NSPayload, pl}, {NSReceipt, rc}, {NSEvent, ev2}, {NSPayload, pl2}} {
		if src := mustRead(t, r, cid, c.ns, c.b); src != SourceSegment {
			t.Fatalf("%s served from %v", c.ns, src)
		}
	}
	// A namespace is part of the key: the same bytes are not a receipt.
	if _, _, err := r.Read(cid, NSReceipt, sum(ev)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("event bytes resolved as a receipt: %v", err)
	}
	var seen int
	if err := r.Scan(cid, func(Record) error { seen++; return nil }); err != nil || seen != 5 {
		t.Fatalf("Scan saw %d records, err %v", seen, err)
	}
	if has, err := r.HasSegments(cid); err != nil || !has {
		t.Fatalf("HasSegments = %v, %v", has, err)
	}
	if has, err := r.HasSegments("computer-none"); err != nil || has {
		t.Fatalf("HasSegments(none) = %v, %v", has, err)
	}
	if s := r.Stats(); s.SegmentHits != 5 {
		t.Fatalf("stats = %+v", s)
	}
}

// FM1: a bit flip in a sealed segment's record is corruption, even though an
// intact legacy copy exists.
func TestFM1BitFlipInSealedSegmentFailsClosed(t *testing.T) {
	root := t.TempDir()
	pl := body("payload that will rot %s", strings.Repeat("x", 200))
	entries := build(t, root, cid, 1, true, rec{NSPayload, pl})
	legacyPut(t, root, NSPayload, pl)
	off := entries[openKey{NSPayload, sha256.Sum256(pl)}].off
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+recHeaderSize+10] ^= 0x40; return b })
	mustCorrupt(t, NewReader(root), cid, NSPayload, pl)
}

// FM2: a record whose crc was recomputed after its body changed still fails
// the SHA-256 check.
func TestFM2RecomputedCRCStillFailsDigest(t *testing.T) {
	root := t.TempDir()
	pl := body("honest payload bytes")
	entries := build(t, root, cid, 1, true, rec{NSPayload, pl})
	off := int(entries[openKey{NSPayload, sha256.Sum256(pl)}].off)
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte {
		b[off+recHeaderSize] ^= 0x01
		refixRecordCRC(b, off)
		return b
	})
	mustCorrupt(t, NewReader(root), cid, NSPayload, pl)
}

// FM3: a sealed segment cut at a record boundary, mid-record or inside its
// header is corruption for any lookup that has to probe it.
func TestFM3TruncatedSealedSegment(t *testing.T) {
	a, b := body("first record"), body("second record, longer than the first")
	for _, c := range []struct {
		name string
		cut  func(entries map[openKey]idxEntry, size int) int
	}{
		{"at a record boundary", func(e map[openKey]idxEntry, _ int) int { return int(e[openKey{NSPayload, sha256.Sum256(b)}].off) }},
		{"mid record", func(e map[openKey]idxEntry, size int) int { return size - 5 }},
		{"inside the header", func(map[openKey]idxEntry, int) int { return 10 }},
		{"to zero", func(map[openKey]idxEntry, int) int { return 0 }},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			entries := build(t, root, cid, 1, true, rec{NSPayload, a}, rec{NSPayload, b})
			legacyPut(t, root, NSPayload, a)
			legacyPut(t, root, NSPayload, b)
			patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { return raw[:c.cut(entries, len(raw))] })
			mustCorrupt(t, NewReader(root), cid, NSPayload, a)
			mustCorrupt(t, NewReader(root), cid, NSPayload, b)
			// A digest in neither segment nor legacy still probes the broken
			// segment and fails closed rather than answering not-found.
			if _, _, err := NewReader(root).Read(cid, NSPayload, sum([]byte("unrelated"))); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("lookup past a truncated segment = %v, want ErrCorrupt", err)
			}
		})
	}
}

// A segment truncated after the reader cached its index still fails closed.
func TestTruncationAfterFirstTouchFailsClosed(t *testing.T) {
	root := t.TempDir()
	a := body("cached then cut")
	build(t, root, cid, 1, true, rec{NSPayload, a})
	r := NewReader(root)
	mustRead(t, r, cid, NSPayload, a)
	patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { return raw[:len(raw)-7] })
	mustCorrupt(t, r, cid, NSPayload, a)
}

// FM4: a torn tail on the last unsealed segment leaves earlier records
// readable and the torn record absent.
func TestFM4TornTailOnLastOpenSegment(t *testing.T) {
	first, second := body("whole record"), body("record that will be torn %s", strings.Repeat("y", 100))
	for _, c := range []struct {
		name string
		tear func(b []byte, off int) []byte
	}{
		{"mid header", func(b []byte, off int) []byte { return b[:off+20] }},
		{"mid body", func(b []byte, off int) []byte { return b[:len(b)-30] }},
		{"zero filled tail", func(b []byte, off int) []byte {
			out := append([]byte(nil), b[:off]...)
			return append(out, make([]byte, 200)...)
		}},
		{"header only", func(b []byte, off int) []byte { return b[:off+recHeaderSize] }},
	} {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			entries := build(t, root, cid, 1, false, rec{NSPayload, first}, rec{NSPayload, second})
			off := int(entries[openKey{NSPayload, sha256.Sum256(second)}].off)
			whole, _ := os.ReadFile(segPath(root, cid, 1))
			patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { return c.tear(b, off) })
			r := NewReader(root)
			mustRead(t, r, cid, NSPayload, first)
			if _, _, err := r.Read(cid, NSPayload, sum(second)); !errors.Is(err, ErrNotFound) {
				t.Fatalf("torn record = %v, want ErrNotFound", err)
			}
			// The torn record falls through to legacy when legacy has it.
			legacyPut(t, root, NSPayload, second)
			if src := mustRead(t, r, cid, NSPayload, second); src != SourceLegacy {
				t.Fatalf("torn record served from %v, want legacy", src)
			}
			// A writer that finishes the append makes it visible to the same
			// reader (incremental rescan).
			if err := os.WriteFile(segPath(root, cid, 1), whole, 0o640); err != nil {
				t.Fatal(err)
			}
			if src := mustRead(t, r, cid, NSPayload, second); src != SourceSegment {
				t.Fatalf("completed record served from %v, want segment", src)
			}
		})
	}
}

// A writer that truncates its torn tail on open shrinks the file under a
// reader that already scanned it.
func TestOpenSegmentShrinkRescans(t *testing.T) {
	root := t.TempDir()
	a, b := body("keeper"), body("later record")
	entries := build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSPayload, b})
	r := NewReader(root)
	mustRead(t, r, cid, NSPayload, b)
	off := int(entries[openKey{NSPayload, sha256.Sum256(b)}].off)
	patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { return raw[:off] })
	mustRead(t, r, cid, NSPayload, a)
	if _, _, err := r.Read(cid, NSPayload, sum(b)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("truncated record = %v, want ErrNotFound", err)
	}
}

// FM5: a bad record that has valid records after it is corruption, not a
// torn tail; so is a bad record in a non-last unsealed segment.
func TestFM5MidSegmentCorruptionIsNotATornTail(t *testing.T) {
	a, b, c := body("record a"), body("record b"), body("record c")
	t.Run("valid records follow", func(t *testing.T) {
		root := t.TempDir()
		entries := build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSPayload, b}, rec{NSPayload, c})
		off := entries[openKey{NSPayload, sha256.Sum256(b)}].off
		patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { raw[off+recHeaderSize] ^= 0xff; return raw })
		r := NewReader(root)
		mustCorrupt(t, r, cid, NSPayload, a)
		mustCorrupt(t, r, cid, NSPayload, c)
	})
	t.Run("non-last unsealed segment with a torn tail", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSPayload, b})
		build(t, root, cid, 2, false, rec{NSPayload, c})
		patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { return raw[:len(raw)-3] })
		// c is in segment 2 and is found first; a lookup that has to reach
		// segment 1 must fail closed.
		mustRead(t, NewReader(root), cid, NSPayload, c)
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
		if _, _, err := NewReader(root).Read(cid, NSPayload, sum(body("absent"))); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("miss past a torn non-last segment = %v", err)
		}
	})
	t.Run("garbage larger than any record", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, false, rec{NSPayload, a})
		patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte {
			return append(raw, bytes.Repeat([]byte{0xAB}, recHeaderSize+MaxRecordBytes+1)...)
		})
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
}

// FM6: the index and the segment must agree.
func TestFM6IndexSegmentMismatch(t *testing.T) {
	a, b := body("alpha record"), body("bravo record")
	setup := func(t *testing.T) (string, map[openKey]idxEntry) {
		root := t.TempDir()
		e := build(t, root, cid, 3, true, rec{NSPayload, a}, rec{NSPayload, b})
		return root, e
	}
	cases := []struct {
		name string
		do   func(t *testing.T, root string, e map[openKey]idxEntry)
	}{
		{"sealed size field", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchIdx(t, idxPath(root, cid, 3), func(b []byte) { binary.BigEndian.PutUint64(b[16:24], binary.BigEndian.Uint64(b[16:24])+1) })
		}},
		{"segment number field", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchIdx(t, idxPath(root, cid, 3), func(b []byte) { binary.BigEndian.PutUint64(b[8:16], 4) })
		}},
		{"computer hash", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchIdx(t, idxPath(root, cid, 3), func(b []byte) { b[30] ^= 0xff })
		}},
		{"bad trailer crc", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchFile(t, idxPath(root, cid, 3), func(b []byte) []byte { b[len(b)-1] ^= 0xff; return b })
		}},
		{"flipped entry byte", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchFile(t, idxPath(root, cid, 3), func(b []byte) []byte { b[idxFixedSize+40] ^= 0x01; return b })
		}},
		{"entry offset points at another record", func(t *testing.T, root string, e map[openKey]idxEntry) {
			ea, eb := e[openKey{NSPayload, sha256.Sum256(a)}], e[openKey{NSPayload, sha256.Sum256(b)}]
			patchIdx(t, idxPath(root, cid, 3), func(raw []byte) {
				for i := 0; i < 2; i++ {
					at := raw[idxFixedSize+idxEntrySize*i:]
					en := decodeIdxEntry(at)
					if en.digest == ea.digest {
						en.off = eb.off
					} else {
						en.off = ea.off
					}
					encodeIdxEntry(at, en)
				}
			})
		}},
		{"entry length", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchIdx(t, idxPath(root, cid, 3), func(raw []byte) {
				binary.BigEndian.PutUint32(raw[idxFixedSize+41:idxFixedSize+45], 3)
				binary.BigEndian.PutUint32(raw[idxFixedSize+idxEntrySize+41:idxFixedSize+idxEntrySize+45], 3)
			})
		}},
		{"index without its segment", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			if err := os.Remove(segPath(root, cid, 3)); err != nil {
				t.Fatal(err)
			}
		}},
		{"truncated index", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchFile(t, idxPath(root, cid, 3), func(b []byte) []byte { return b[:idxFixedSize+10] })
		}},
		{"segment header names another segment", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchFile(t, segPath(root, cid, 3), func(b []byte) []byte {
				binary.BigEndian.PutUint64(b[8:16], 9)
				binary.BigEndian.PutUint32(b[28:32], crc32.Checksum(b[0:28], castagnoli))
				return b
			})
		}},
		{"segment belongs to another computer", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchFile(t, segPath(root, cid, 3), func(b []byte) []byte {
				b[20] ^= 0xff
				binary.BigEndian.PutUint32(b[28:32], crc32.Checksum(b[0:28], castagnoli))
				return b
			})
		}},
		{"fanout disagrees with entries", func(t *testing.T, root string, _ map[openKey]idxEntry) {
			patchIdx(t, idxPath(root, cid, 3), func(b []byte) {
				// Move all entries into the wrong first-byte bucket.
				for i := 0; i < 256; i++ {
					binary.BigEndian.PutUint32(b[40+4*i:44+4*i], 0)
				}
				binary.BigEndian.PutUint32(b[40+4*255:44+4*255], 2)
			})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, e := setup(t)
			legacyPut(t, root, NSPayload, a)
			legacyPut(t, root, NSPayload, b)
			c.do(t, root, e)
			mustCorrupt(t, NewReader(root), cid, NSPayload, a)
			mustCorrupt(t, NewReader(root), cid, NSPayload, b)
		})
	}
}

// FM7: unknown versions, magics, namespaces and oversize lengths.
func TestFM7UnsupportedFormats(t *testing.T) {
	a := body("versioned record")
	t.Run("segment version", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, false, rec{NSPayload, a})
		patchFile(t, segPath(root, cid, 1), func(b []byte) []byte {
			b[4] = 2
			binary.BigEndian.PutUint32(b[28:32], crc32.Checksum(b[0:28], castagnoli))
			return b
		})
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
	t.Run("segment magic", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, false, rec{NSPayload, a})
		patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[0] = 'X'; return b })
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
	t.Run("index version", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, true, rec{NSPayload, a})
		patchIdx(t, idxPath(root, cid, 1), func(b []byte) { b[4] = 2 })
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
	t.Run("index magic", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, true, rec{NSPayload, a})
		patchIdx(t, idxPath(root, cid, 1), func(b []byte) { b[1] = 'Z' })
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
	t.Run("intact record with an unknown namespace is not a torn write", func(t *testing.T) {
		root := t.TempDir()
		entries := build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSEvent, body("tail")})
		off := int(entries[openKey{NSEvent, sha256.Sum256(body("tail"))}].off)
		patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+5] = 9; refixRecordCRC(b, off); return b })
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
	t.Run("intact record with an unknown version", func(t *testing.T) {
		root := t.TempDir()
		entries := build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSEvent, body("tail")})
		off := int(entries[openKey{NSEvent, sha256.Sum256(body("tail"))}].off)
		patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+4] = 7; refixRecordCRC(b, off); return b })
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
	t.Run("oversize length in a sealed segment", func(t *testing.T) {
		root := t.TempDir()
		entries := build(t, root, cid, 1, true, rec{NSPayload, a})
		off := int(entries[openKey{NSPayload, sha256.Sum256(a)}].off)
		patchFile(t, segPath(root, cid, 1), func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[off+38:off+42], MaxRecordBytes+1)
			return b
		})
		mustCorrupt(t, NewReader(root), cid, NSPayload, a)
	})
}

// FM8: a digest in both layouts is served from the segment, with equal bytes;
// a corrupt segment copy is an error, not a fallback.
func TestFM8DigestInBothLayouts(t *testing.T) {
	pl := body("in both layouts %s", strings.Repeat("z", 64))
	root := t.TempDir()
	entries := build(t, root, cid, 1, true, rec{NSPayload, pl})
	legacyPut(t, root, NSPayload, pl)
	r := NewReader(root)
	got, src, err := r.Read(cid, NSPayload, sum(pl))
	if err != nil || src != SourceSegment || !bytes.Equal(got, pl) {
		t.Fatalf("both layouts: src %v err %v", src, err)
	}
	legacy, err := r.ReadLegacy(NSPayload, sum(pl))
	if err != nil || !bytes.Equal(legacy, got) {
		t.Fatalf("legacy copy differs from the segment copy: %v", err)
	}
	// Legacy rot does not matter once the segment holds the digest.
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(NSPayload.LegacyRel(sum(pl)))), []byte("rotten"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, src, err := NewReader(root).Read(cid, NSPayload, sum(pl)); err != nil || src != SourceSegment {
		t.Fatalf("segment copy not served over rotten legacy: %v %v", src, err)
	}
	// Segment rot with an intact legacy copy fails closed.
	legacyPut(t, root, NSPayload, pl)
	off := entries[openKey{NSPayload, sha256.Sum256(pl)}].off
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+recHeaderSize+1] ^= 1; return b })
	mustCorrupt(t, NewReader(root), cid, NSPayload, pl)
	// A different computer has no segment for it, so it reads legacy.
	if src := mustRead(t, NewReader(root), "computer-b", NSPayload, pl); src != SourceLegacy {
		t.Fatalf("other computer served from %v", src)
	}
}

// FM9 and FM10: legacy-only and absent digests.
func TestFM9LegacyOnlyAndFM10Absent(t *testing.T) {
	root := t.TempDir()
	build(t, root, cid, 1, true, rec{NSPayload, body("something else")})
	r := NewReader(root)
	ev := body(`{"legacy":"event"}`)
	legacyPut(t, root, NSEvent, ev)
	if src := mustRead(t, r, cid, NSEvent, ev); src != SourceLegacy {
		t.Fatalf("legacy-only served from %v", src)
	}
	rc := body(`{"legacy":"receipt"}`)
	legacyPut(t, root, NSReceipt, rc)
	if src := mustRead(t, r, cid, NSReceipt, rc); src != SourceLegacy {
		t.Fatalf("legacy receipt served from %v", src)
	}
	if _, _, err := r.Read(cid, NSPayload, sum([]byte("nowhere"))); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent = %v, want ErrNotFound", err)
	}
	// A legacy file that does not hash to its name is a digest mismatch.
	bad := sum([]byte("expected content"))
	p := filepath.Join(root, filepath.FromSlash(NSPayload.LegacyRel(bad)))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("other content"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Read(cid, NSPayload, bad); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("mismatching legacy = %v, want ErrDigestMismatch", err)
	}
	if s := r.Stats(); s.LegacyHits != 2 || s.NotFound != 1 || s.DigestMismatch != 1 {
		t.Fatalf("stats = %+v", s)
	}
}

// FM11: concurrent readers on sealed and open segments while a writer
// appends. Run with -race.
func TestFM11ConcurrentReaders(t *testing.T) {
	root := t.TempDir()
	var sealedBodies, openBodies [][]byte
	var recs []rec
	for i := 0; i < 300; i++ {
		b := body("sealed payload %d %s", i, strings.Repeat("s", i%50))
		sealedBodies = append(sealedBodies, b)
		recs = append(recs, rec{NSPayload, b})
	}
	build(t, root, cid, 1, true, recs...)
	w, err := CreateSegment(root, cid, 2)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		b := body("open payload %d", i)
		openBodies = append(openBodies, b)
		if _, err := w.Append(NSPayload, b); err != nil {
			t.Fatal(err)
		}
	}
	r := NewReader(root)
	var wg sync.WaitGroup
	var live [][]byte
	var liveMu sync.Mutex
	stop := make(chan struct{})
	wg.Add(1)
	go func() { // writer
		defer wg.Done()
		for i := 0; i < 200; i++ {
			b := body("late payload %d", i)
			if _, err := w.Append(NSPayload, b); err != nil {
				t.Error(err)
				return
			}
			liveMu.Lock()
			live = append(live, b)
			liveMu.Unlock()
		}
		close(stop)
	}()
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for n := 0; ; n++ {
				for _, set := range [][][]byte{sealedBodies, openBodies} {
					b := set[(n*7+g)%len(set)]
					got, _, err := r.Read(cid, NSPayload, sum(b))
					if err != nil || !bytes.Equal(got, b) {
						t.Errorf("reader %d: %v", g, err)
						return
					}
				}
				liveMu.Lock()
				var b []byte
				if len(live) > 0 {
					b = live[(n+g)%len(live)]
				}
				liveMu.Unlock()
				if b != nil {
					// Appended before this read began, so it must be whole.
					got, _, err := r.Read(cid, NSPayload, sum(b))
					if err != nil || !bytes.Equal(got, b) {
						t.Errorf("reader %d late record: %v", g, err)
						return
					}
				}
				select {
				case <-stop:
					return
				default:
				}
			}
		}(g)
	}
	wg.Wait()
	w.Close()
}

// FM12: unsafe computer ids and digests never touch the filesystem outside
// the root.
func TestFM12PathSafety(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "escape-"+filepath.Base(root))
	if err := os.MkdirAll(filepath.Join(outside, "x"), 0o750); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(outside)
	r := NewReader(root)
	for _, id := range []string{"", ".", "..", "../x", "a/b", `a\b`, "a\x00b", ".hidden", strings.Repeat("a", 201)} {
		if ValidComputerID(id) {
			t.Fatalf("%q accepted", id)
		}
		if has, err := r.HasSegments(id); err != nil || has {
			t.Fatalf("HasSegments(%q) = %v, %v", id, has, err)
		}
		if _, _, found := func() ([]byte, Source, bool) {
			b, f, err := r.ReadSegment(id, NSPayload, sum([]byte("x")))
			return b, 0, f && err == nil
		}(); found {
			t.Fatalf("segment read for %q found something", id)
		}
		if _, err := CreateSegment(root, id, 1); err == nil {
			t.Fatalf("CreateSegment accepted %q", id)
		}
	}
	for _, d := range []string{"", "abc", strings.Repeat("A", 64), strings.Repeat("g", 64), "../" + strings.Repeat("a", 61), strings.Repeat("a", 65)} {
		if _, _, err := r.Read(cid, NSPayload, d); err == nil || errors.Is(err, ErrNotFound) {
			t.Fatalf("digest %q not rejected: %v", d, err)
		}
	}
	if _, _, err := r.Read(cid, NS(0), sum([]byte("x"))); err == nil {
		t.Fatal("namespace 0 accepted")
	}
}

// FM13: the same digest in two segments returns the same verified bytes; a
// newer segment is consulted first.
func TestFM13DuplicateDigestAcrossSegments(t *testing.T) {
	root := t.TempDir()
	pl := body("duplicated across segments")
	build(t, root, cid, 1, true, rec{NSPayload, pl})
	build(t, root, cid, 2, true, rec{NSPayload, pl}, rec{NSEvent, body("other")})
	r := NewReader(root)
	mustRead(t, r, cid, NSPayload, pl)
	// Rot in the older copy is not consulted once the newer one answers.
	off := int(binary.BigEndian.Uint64(func() []byte {
		b, _ := os.ReadFile(idxPath(root, cid, 1))
		return b[idxFixedSize+33 : idxFixedSize+41]
	}()))
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[off+recHeaderSize] ^= 1; return b })
	mustRead(t, NewReader(root), cid, NSPayload, pl)
	// A lookup that must reach an index that fails its crc fails closed.
	patchFile(t, idxPath(root, cid, 1), func(b []byte) []byte { b[len(b)-1] ^= 0xff; return b })
	mustRead(t, NewReader(root), cid, NSPayload, pl) // still answered by segment 2
	if _, _, err := NewReader(root).Read(cid, NSEvent, sum(body("not present anywhere"))); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("miss past a corrupt index = %v", err)
	}
}

// Many records across every fanout bucket: all present digests resolve,
// absent ones do not, and Scan audits the lot.
func TestIndexFanoutManyRecords(t *testing.T) {
	root := t.TempDir()
	var recs []rec
	var bodies [][]byte
	for i := 0; i < 3000; i++ {
		b := body("record %d", i)
		bodies = append(bodies, b)
		recs = append(recs, rec{NS(1 + i%3), b})
	}
	build(t, root, cid, 1, true, recs...)
	r := NewReader(root)
	for i, b := range bodies {
		mustRead(t, r, cid, NS(1+i%3), b)
		if _, _, err := r.Read(cid, NS(1+(i+1)%3), sum(b)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("record %d resolved in the wrong namespace: %v", i, err)
		}
	}
	for i := 0; i < 500; i++ {
		if _, _, err := r.Read(cid, NSPayload, sum(body("absent %d", i))); !errors.Is(err, ErrNotFound) {
			t.Fatalf("absent %d = %v", i, err)
		}
	}
	var n int
	if err := r.Scan(cid, func(Record) error { n++; return nil }); err != nil || n != 3000 {
		t.Fatalf("Scan = %d, %v", n, err)
	}
}

func TestScanAuditsWholeFileProperties(t *testing.T) {
	a, b := body("scan a"), body("scan b")
	t.Run("record appended after sealing", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, true, rec{NSPayload, a})
		extra := encodeRecord(NSPayload, digestBytes(b), b)
		patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { return append(raw, extra...) })
		if err := NewReader(root).Scan(cid, func(Record) error { return nil }); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("Scan = %v", err)
		}
	})
	t.Run("record missing from the index", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSPayload, b})
		// Seal an index that lists only one of the two records.
		w := &SegmentWriter{dir: segDir(root, cid), computerID: cid, segno: 1}
		st, _ := os.Stat(segPath(root, cid, 1))
		e := idxEntry{digest: digestBytes(a), ns: NSPayload, off: segHeaderSize, length: uint32(len(a))}
		if err := os.WriteFile(idxPath(root, cid, 1), encodeIndex(1, cid, uint64(st.Size()), []idxEntry{e}), 0o640); err != nil {
			t.Fatal(err)
		}
		_ = w
		if err := NewReader(root).Scan(cid, func(Record) error { return nil }); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("Scan = %v", err)
		}
	})
	t.Run("torn tail on the last open segment is tolerated", func(t *testing.T) {
		root := t.TempDir()
		build(t, root, cid, 1, false, rec{NSPayload, a}, rec{NSPayload, b})
		patchFile(t, segPath(root, cid, 1), func(raw []byte) []byte { return raw[:len(raw)-4] })
		var n int
		if err := NewReader(root).Scan(cid, func(Record) error { n++; return nil }); err != nil || n != 1 {
			t.Fatalf("Scan = %d, %v", n, err)
		}
	})
}

func TestWriterIsIdempotentAndBounded(t *testing.T) {
	root := t.TempDir()
	w, err := CreateSegment(root, cid, 1)
	if err != nil {
		t.Fatal(err)
	}
	d1, _ := w.Append(NSPayload, body("same"))
	size := w.Size()
	d2, _ := w.Append(NSPayload, body("same"))
	if d1 != d2 || w.Size() != size || w.Records() != 1 {
		t.Fatal("duplicate append changed the segment")
	}
	if _, err := w.Append(NS(7), body("x")); err == nil {
		t.Fatal("invalid namespace accepted")
	}
	if _, err := w.Append(NSPayload, make([]byte, MaxRecordBytes+1)); err == nil {
		t.Fatal("oversize record accepted")
	}
	if err := w.Seal(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(NSPayload, body("late")); err == nil {
		t.Fatal("append after seal accepted")
	}
	if _, err := CreateSegment(root, cid, 1); err == nil {
		t.Fatal("segment number reused")
	}
	// Stray files in the directory (a writer's temp index) are ignored.
	if err := os.WriteFile(filepath.Join(segDir(root, cid), "seg-00000009.idx.tmp"), []byte("junk"), 0o640); err != nil {
		t.Fatal(err)
	}
	mustRead(t, NewReader(root), cid, NSPayload, body("same"))
}

// FM14 and the A1 contract: no tape-seg directory means pure legacy.
func TestNoSegmentDirectoryIsPureLegacy(t *testing.T) {
	root := t.TempDir()
	r := NewReader(root)
	pl := body("legacy world")
	legacyPut(t, root, NSPayload, pl)
	if src := mustRead(t, r, cid, NSPayload, pl); src != SourceLegacy {
		t.Fatalf("served from %v", src)
	}
	if _, err := os.Stat(filepath.Join(root, SegDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the reader created tape-seg")
	}
}

// A1 has no production writer: nothing outside this package may name the
// segment writer.
func TestNoProductionWriterReferencesSegmentWriter(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("module root not found")
		}
		dir = parent
	}
	self := filepath.Join(dir, "internal", "tapestore")
	var offenders []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "node_modules", ".git", "sourcecycled", "ograph-bench":
				return filepath.SkipDir
			}
			if path == self {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(raw, []byte("tapestore.CreateSegment")) || bytes.Contains(raw, []byte("tapestore.SegmentWriter")) {
			offenders = append(offenders, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(offenders) > 0 {
		t.Fatalf("A1 has no segment writer in production; found references in %v", offenders)
	}
}
