package tapestore

// The sweep's reference walk reads one event at a time from a tape of
// thousands of segments (docs/problems/tape-sweep-walk-probes-every-segment-
// per-event-2026-11-13.md). A view answers those reads with one listing and
// one index validation, probing from the segment of the last hit.
//
// Failure list, each a test below:
//
//  W1  ReadObject returns exactly what Reader.Read returns (bytes, source,
//      error text and class) over a fixture with sealed segments, an open
//      segment, a duplicate record in two segments, events out of segment
//      order, a record absent from every segment (legacy copy, or none), a
//      legacy copy that does not hash, and a damaged record with and without
//      a legacy copy.
//  W2  On an in-order tape of many segments the probes per read are at most
//      two, not the segment count.
//  W3  A record appended after the view opened (open segment grown, that
//      segment sealed, new segments created) is still found.
//  W4  A computer with no segments reads legacy files only.
//  Benchmark: BenchmarkViewReadVsReaderRead runs both paths on 400 segments.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type readOutcome struct {
	body []byte
	src  Source
	err  error
}

func sameOutcome(a, b readOutcome) bool {
	if !bytes.Equal(a.body, b.body) || a.src != b.src {
		return false
	}
	if (a.err == nil) != (b.err == nil) {
		return false
	}
	if a.err == nil {
		return true
	}
	for _, class := range []error{ErrCorrupt, ErrNotFound, ErrDigestMismatch, ErrUnsupportedVersion} {
		if errors.Is(a.err, class) != errors.Is(b.err, class) {
			return false
		}
	}
	return a.err.Error() == b.err.Error()
}

func TestW1ViewReadEqualsReaderRead(t *testing.T) {
	root := t.TempDir()
	const id, idD = "comp-w", "comp-wd"
	pad := strings.Repeat("p", 60)
	ev := func(i int) []byte { return body(`{"event":%d,"pad":"%s"}`, i, pad) }

	// Segments 1..4 sealed, 5 open. Events 0..39, ten a segment.
	for seg := 1; seg <= 5; seg++ {
		var recs []rec
		for i := (seg - 1) * 10; i < seg*10; i++ {
			recs = append(recs, rec{NSEvent, ev(i)})
		}
		if seg == 2 {
			recs = append(recs, rec{NSEvent, ev(35)}) // duplicate of a record in segment 4
		}
		build(t, root, id, uint64(seg), seg < 5, recs...)
	}

	// Absent from every segment: a legacy copy, none at all, and a legacy
	// file that does not hash.
	legacyOnly := body("legacy only %s", pad)
	legacyPut(t, root, NSEvent, legacyOnly)
	absent := sum(body("never written"))
	bad := sum(body("rotten legacy"))
	p := filepath.Join(root, filepath.FromSlash(NSEvent.LegacyRel(bad)))
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("not the bytes"), 0o640); err != nil {
		t.Fatal(err)
	}

	// Damaged records, on a second computer with the same segment shape: one
	// with an intact legacy copy, one without.
	dmg := body("damaged record %s", pad)
	dmgNoLegacy := body("damaged record without legacy %s", pad)
	entries := build(t, root, idD, 1, true, rec{NSEvent, dmg}, rec{NSEvent, dmgNoLegacy}, rec{NSEvent, ev(1)})
	build(t, root, idD, 2, true, rec{NSEvent, ev(2)})
	build(t, root, idD, 3, false, rec{NSEvent, ev(3)})
	for _, b := range [][]byte{dmg, dmgNoLegacy} {
		off := entries[openKey{NSEvent, digestBytes(b)}].off
		patchFile(t, segPath(root, idD, 1), func(raw []byte) []byte { raw[off+recHeaderSize+3] ^= 0x10; return raw })
	}
	legacyPut(t, root, NSEvent, dmg)

	type probe struct{ id, d string }
	var probes []probe
	for _, i := range []int{0, 39, 5, 12, 11, 25, 24, 35, 1, 30, 31, 2, 38, 35, 0, 17} { // out of segment order, repeats
		probes = append(probes, probe{id, sum(ev(i))})
	}
	probes = append(probes,
		probe{id, sum(legacyOnly)}, probe{id, absent}, probe{id, bad},
		probe{idD, sum(dmg)}, probe{idD, sum(dmgNoLegacy)}, probe{idD, sum(ev(1))},
		probe{idD, sum(ev(3))}, probe{idD, sum(ev(2))}, probe{idD, absent},
		probe{id, sum(ev(7))},
	)

	oldR, newR := NewReader(root), NewReader(root)
	views := map[string]*SegmentView{}
	for _, c := range []string{id, idD} {
		v, err := newR.OpenView(c)
		if err != nil {
			t.Fatalf("OpenView %s: %v", c, err)
		}
		views[c] = v
	}
	var corrupt, notFound, mismatch, legacy bool
	for n, p := range probes {
		var want, got readOutcome
		want.body, want.src, want.err = oldR.Read(p.id, NSEvent, p.d)
		got.body, got.src, got.err = views[p.id].ReadObject(NSEvent, p.d)
		if !sameOutcome(want, got) {
			t.Fatalf("probe %d (%s %s): Reader.Read = %+v, ReadObject = %+v", n, p.id, p.d[:8], want, got)
		}
		corrupt = corrupt || errors.Is(want.err, ErrCorrupt)
		notFound = notFound || errors.Is(want.err, ErrNotFound)
		mismatch = mismatch || errors.Is(want.err, ErrDigestMismatch)
		legacy = legacy || (want.err == nil && want.src == SourceLegacy)
	}
	if !corrupt || !notFound || !mismatch || !legacy {
		t.Fatalf("fixture covers corrupt=%v notfound=%v mismatch=%v legacy=%v", corrupt, notFound, mismatch, legacy)
	}
	if got, want := newR.Stats(), oldR.Stats(); got != want {
		t.Fatalf("reader counters differ: view %+v, per-event %+v", got, want)
	}
}

func TestW2ViewProbesStayNearOnePerReadOnAnInOrderTape(t *testing.T) {
	root := t.TempDir()
	const id = "comp-w2"
	const n, per = 300, 3
	a := newTestAppender(root, 1<<30, per)
	var digests []string
	for i := 0; i < n; i++ {
		b := body(`{"event":%d}`, i)
		mustAppend(t, a, id, Item{NSEvent, b})
		digests = append(digests, sum(b))
	}
	v, err := NewReader(root).OpenView(id)
	if err != nil {
		t.Fatal(err)
	}
	if v.Segments() < n/per {
		t.Fatalf("fixture has %d segments", v.Segments())
	}
	for i, d := range digests {
		if _, src, err := v.ReadObject(NSEvent, d); err != nil || src != SourceSegment {
			t.Fatalf("event %d: %v %v", i, src, err)
		}
	}
	if got, bound := v.Probes(), uint64(2*n); got > bound {
		t.Fatalf("%d probes for %d in-order reads over %d segments, want at most %d", got, n, v.Segments(), bound)
	}
}

func TestW3ViewFindsRecordsWrittenAfterItOpened(t *testing.T) {
	root := t.TempDir()
	const id = "comp-w3"
	a := newTestAppender(root, 1<<30, 3)
	for i := 0; i < 7; i++ {
		mustAppend(t, a, id, Item{NSEvent, body("early %d", i)})
	}
	v, err := NewReader(root).OpenView(id)
	if err != nil {
		t.Fatal(err)
	}
	segs := v.Segments()
	if _, _, err := v.ReadObject(NSEvent, sum(body("early 0"))); err != nil {
		t.Fatal(err)
	}
	// Grows the open segment, seals it, and creates new segments.
	for i := 0; i < 8; i++ {
		b := body("late %d", i)
		mustAppend(t, a, id, Item{NSEvent, b})
		got, src, err := v.ReadObject(NSEvent, sum(b))
		if err != nil || src != SourceSegment || !bytes.Equal(got, b) {
			t.Fatalf("late %d: %v %v", i, src, err)
		}
	}
	if err := a.Seal(id); err != nil {
		t.Fatal(err)
	}
	fresh, err := NewReader(root).OpenView(id)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Segments() <= segs {
		t.Fatalf("fixture did not add segments (%d -> %d)", segs, fresh.Segments())
	}
	for i := 0; i < 7; i++ {
		if _, _, err := v.ReadObject(NSEvent, sum(body("early %d", i))); err != nil {
			t.Fatalf("early %d after growth: %v", i, err)
		}
	}
	for i := 0; i < 8; i++ {
		if _, _, err := v.ReadObject(NSEvent, sum(body("late %d", i))); err != nil {
			t.Fatalf("late %d after seal: %v", i, err)
		}
	}
}

func TestW4ViewOfAComputerWithNoSegmentsReadsLegacy(t *testing.T) {
	root := t.TempDir()
	b := body("legacy only")
	d := legacyPut(t, root, NSEvent, b)
	v, err := NewReader(root).OpenView("comp-none")
	if err != nil {
		t.Fatal(err)
	}
	got, src, err := v.ReadObject(NSEvent, d)
	if err != nil || src != SourceLegacy || !bytes.Equal(got, b) {
		t.Fatalf("%v %v", src, err)
	}
	if _, _, err := v.ReadObject(NSEvent, sum(body("absent"))); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
}

func BenchmarkViewReadVsReaderRead(b *testing.B) {
	root := b.TempDir()
	const id = "comp-bench"
	a := newTestAppender(root, 1<<30, 5)
	var digests []string
	for i := 0; i < 400*5; i++ {
		bd := []byte(fmt.Sprintf(`{"event":%d,"pad":"%s"}`, i, strings.Repeat("x", 200)))
		if err := a.Append(id, Item{NSEvent, bd}); err != nil {
			b.Fatal(err)
		}
		digests = append(digests, sum(bd))
	}
	b.Run("reader_read", func(b *testing.B) {
		r := NewReader(root)
		for i := 0; i < b.N; i++ {
			if _, _, err := r.Read(id, NSEvent, digests[i%len(digests)]); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("view_in_order", func(b *testing.B) {
		v, err := NewReader(root).OpenView(id)
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < b.N; i++ {
			if _, _, err := v.ReadObject(NSEvent, digests[i%len(digests)]); err != nil {
				b.Fatal(err)
			}
		}
	})
}
