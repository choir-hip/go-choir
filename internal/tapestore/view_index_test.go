package tapestore

// docs/problems/tape-sweep-walk-probes-every-segment-per-event-2026-11-13.md,
// round 2: on the paper the hint did not help (about 935 probes per event over
// 1,789 segments). The repack appends legacy files bucket by first digest
// byte (internal/taperepack/join.go), so consecutive events sit in unrelated
// segments. An indexed view keeps a sorted array of (digest prefix, segment)
// for one namespace, built while the sealed indexes are read, and uses it
// only to choose which segments to probe.
//
//  I1  The indexed view and the plain view return the same outcome for every
//      digest over randomized scattered tapes with duplicates, damage,
//      legacy-only and absent digests, an open segment holding records, and a
//      tape that grows (sealed segments, a new open one) after the view opened.
//  I2  On a scattered tape of 300 segments a hit costs about one probe, and
//      an absent digest probes no sealed segment.
//  I3  The index costs at most 12 bytes an entry (reported).
//  Benchmark: BenchmarkIndexedViewScattered, 400 scattered segments.

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestI1IndexedViewEqualsPlainViewOnScatteredTapes(t *testing.T) {
	for seed := int64(0); seed < 30; seed++ {
		rng := rand.New(rand.NewSource(seed))
		root := t.TempDir()
		const id = "comp-i1"
		bodies := make([][]byte, 40)
		for i := range bodies {
			bodies[i] = body("i1 seed %d body %d %s", seed, i, strings.Repeat("q", 10+rng.Intn(50)))
		}
		nseg := 4 + rng.Intn(6)
		place := func(first, last int, _ bool) {
			for s := first; s <= last; s++ {
				var recs []rec
				var idxs []int
				for i := range bodies {
					if rng.Intn(5) == 0 { // scattered, with duplicates across segments
						recs = append(recs, rec{NSEvent, bodies[i]})
						idxs = append(idxs, i)
					}
				}
				if len(recs) == 0 {
					recs, idxs = []rec{{NSEvent, bodies[s%len(bodies)]}}, []int{s % len(bodies)}
				}
				sealed := true
				entries := build(t, root, id, uint64(s), sealed, recs...)
				for _, i := range idxs {
					if sealed && rng.Intn(5) == 0 {
						damageRecord(t, root, id, uint64(s), entries, bodies[i])
					}
				}
			}
		}
		place(1, nseg, false)
		app := newTestAppender(root, 1<<30, 1000)
		appendSome := func() {
			for i := range bodies {
				if rng.Intn(6) == 0 {
					mustAppend(t, app, id, Item{NSEvent, bodies[i]})
				}
			}
		}
		appendSome() // the open segment holds records
		for i := range bodies {
			switch rng.Intn(4) {
			case 0:
				legacyPut(t, root, NSEvent, bodies[i])
			case 1:
				p := filepath.Join(root, filepath.FromSlash(NSEvent.LegacyRel(sum(bodies[i]))))
				if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("rotten"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
		}
		plain, err := NewReader(root).OpenView(id)
		if err != nil {
			t.Fatal(err)
		}
		indexed, err := NewReader(root).OpenIndexedView(id, NSEvent)
		if err != nil {
			t.Fatal(err)
		}
		if indexed.IndexEntries() == 0 {
			t.Fatalf("seed %d: no index built", seed)
		}
		check := func(stage string) {
			t.Helper()
			var digests []string
			for i := range bodies {
				digests = append(digests, sum(bodies[i]))
			}
			digests = append(digests, sum(body("absent %d", seed)))
			for _, d := range digests {
				var want, got readOutcome
				want.body, want.src, want.err = plain.ReadObject(NSEvent, d)
				got.body, got.src, got.err = indexed.ReadObject(NSEvent, d)
				if !sameOutcome(want, got) {
					t.Fatalf("seed %d %s digest %s: plain %+v, indexed %+v", seed, stage, d[:8], want, got)
				}
			}
		}
		check("initial")
		// The tape grows after both views opened: the open segment gains
		// records and is sealed, and a new segment opens.
		appendSome()
		check("open segment grown")
		if err := app.Seal(id); err != nil {
			t.Fatal(err)
		}
		appendSome()
		check("sealed and a new open segment")
		if err := app.Seal(id); err != nil {
			t.Fatal(err)
		}
		appendSome()
		check("grown again")
	}
}

func scatteredTape(tb testing.TB, root, id string, segs, per int) [][]byte {
	tb.Helper()
	rng := rand.New(rand.NewSource(7))
	n := segs * per
	order := rng.Perm(n) // event i lives in segment order[i]/per: scattered
	bodies := make([][]byte, n)
	for i := range bodies {
		bodies[i] = []byte(fmt.Sprintf(`{"event":%d,"pad":"%s"}`, i, strings.Repeat("x", 100)))
	}
	bySeg := make([][]int, segs)
	for i, p := range order {
		bySeg[p/per] = append(bySeg[p/per], i)
	}
	for s := 0; s < segs; s++ {
		w, err := CreateSegment(root, id, uint64(s+1))
		if err != nil {
			tb.Fatal(err)
		}
		for _, i := range bySeg[s] {
			if _, err := w.Append(NSEvent, bodies[i]); err != nil {
				tb.Fatal(err)
			}
		}
		if err := w.Seal(); err != nil {
			tb.Fatal(err)
		}
	}
	return bodies
}

func TestI2ScatteredHitsCostAboutOneProbeAndAbsentCostsNoSealedProbe(t *testing.T) {
	root := t.TempDir()
	const id = "comp-i2"
	bodies := scatteredTape(t, root, id, 300, 5)
	v, err := NewReader(root).OpenIndexedView(id, NSEvent)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range bodies { // in event order: the placement is scattered
		if got, src, err := v.ReadObject(NSEvent, sum(b)); err != nil || src != SourceSegment || !bytes.Equal(got, b) {
			t.Fatalf("event %d: %v %v", i, src, err)
		}
	}
	if got, bound := v.Probes(), uint64(len(bodies))*11/10; got > bound {
		t.Fatalf("%d probes for %d scattered reads over %d segments, want about one each (at most %d)", got, len(bodies), v.Segments(), bound)
	}
	before := v.SealedProbes()
	for i := 0; i < 200; i++ {
		if _, _, err := v.ReadObject(NSEvent, sum(body("absent %d", i))); err == nil {
			t.Fatal("an absent digest was found")
		}
	}
	if got := v.SealedProbes() - before; got != 0 {
		t.Fatalf("absent digests probed %d sealed segments, want 0", got)
	}
	// The plain view needs about half the segments on the same reads.
	pv, _ := NewReader(root).OpenView(id)
	for _, b := range bodies[:200] {
		if _, _, err := pv.ReadObject(NSEvent, sum(b)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("indexed: %.2f probes/read; plain view: %.1f probes/read over %d segments", float64(v.Probes())/float64(len(bodies)), float64(pv.Probes())/200, v.Segments())
}

func TestI3IndexBytesPerEntry(t *testing.T) {
	root := t.TempDir()
	const id = "comp-i3"
	scatteredTape(t, root, id, 200, 50) // 10,000 events
	var m0, m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)
	v, err := NewReader(root).OpenIndexedView(id, NSEvent)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&m1)
	n := v.IndexEntries()
	if n != 10000 {
		t.Fatalf("%d entries, want 10000", n)
	}
	per := float64(v.IndexBytes()) / float64(n)
	if per > 12 {
		t.Fatalf("%.1f bytes an entry, want at most 12", per)
	}
	t.Logf("%d entries, %d bytes: %.1f bytes/entry; for 11,000,000 events: %.0f MB (heap delta %.1f bytes/entry)", n, v.IndexBytes(), per, per*11e6/1e6, float64(int64(m1.HeapAlloc)-int64(m0.HeapAlloc))/float64(n))
	runtime.KeepAlive(v)
}

func BenchmarkIndexedViewScattered(b *testing.B) {
	root := b.TempDir()
	const id = "comp-bench-i"
	bodies := scatteredTape(b, root, id, 400, 5)
	run := func(name string, open func() (*SegmentView, error)) {
		b.Run(name, func(b *testing.B) {
			v, err := open()
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, _, err := v.ReadObject(NSEvent, sum(bodies[i%len(bodies)])); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(v.Probes())/float64(b.N), "probes/read")
		})
	}
	run("plain_view", func() (*SegmentView, error) { return NewReader(root).OpenView(id) })
	run("indexed_view", func() (*SegmentView, error) { return NewReader(root).OpenIndexedView(id, NSEvent) })
}
