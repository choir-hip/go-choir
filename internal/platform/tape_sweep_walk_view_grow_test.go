package platform

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// X3: the walk runs while the real Appender appends to the open segment,
// seals it and starts new ones. The view reloads cleanly (no fallback to the
// per-call reader) and the later events are read through it.
func TestX3WalkFindsEventsWrittenToSegmentsDuringTheWalk(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	c := f.chain("live-1", 30, tapeAged)
	app := tapestore.NewAppender(f.root)
	appendEv := func(i int) {
		p := f.path(tapeSweepRel(tapeNSEvent, c.seqs[i].event))
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Append("live-1", tapestore.Item{NS: tapestore.NSEvent, Body: raw}); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		appendEv(i)
	}
	if err := app.Seal("live-1"); err != nil { // segment 1 sealed
		t.Fatal(err)
	}
	for i := 5; i < 8; i++ { // segment 2 open
		appendEv(i)
	}
	var probes, reloads, fallbacks uint64
	var segs int
	f.svc.tapeSweep.walkViewDone = func(_ string, v *tapestore.SegmentView) {
		probes, reloads, fallbacks, segs = v.Probes(), v.Reloads(), v.Fallbacks(), v.Segments()
	}
	done := false
	rows, err := f.walkedRows("live-1", 30, func(r tapeChainRow) {
		if r.Sequence == 4 && !done {
			done = true
			for i := 8; i < 12; i++ { // the open segment grows
				appendEv(i)
			}
			if err := app.Seal("live-1"); err != nil { // segment 2 sealed
				t.Fatal(err)
			}
			for i := 12; i < 20; i++ { // segment 3, sealed, then segment 4 open
				appendEv(i)
			}
			if err := app.Seal("live-1"); err != nil {
				t.Fatal(err)
			}
			for i := 20; i < 30; i++ {
				appendEv(i)
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 30 {
		t.Fatalf("walked %d rows", len(rows))
	}
	for _, r := range rows {
		if !r.EventReadable {
			t.Fatalf("sequence %d unreadable", r.Sequence)
		}
	}
	if reloads == 0 || fallbacks != 0 {
		t.Fatalf("reloads %d fallbacks %d: the walk must reload the view, not fall back", reloads, fallbacks)
	}
	if segs != 4 || probes > 3*30 {
		t.Fatalf("segments %d probes %d, want 4 and a handful per read", segs, probes)
	}
}

// X4: an event stored twice, its older copy damaged. The per-event path read
// the newest copy; the walk must not stop because the hint sits on the
// damaged one.
func TestX4WalkDoesNotStopOnADamagedOlderDuplicate(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	c := f.chain("live-1", 10, tapeAged)
	ev := func(i int) segItem { return segItem{tapestore.NSEvent, c.seqs[i].event} }
	// Segment 1 starts with event 3 (its first record, damaged below), then
	// holds 0..2, so the hint sits there when the walk reaches event 3.
	segmentize(t, f.root, "live-1", 1, true, true, ev(3), ev(0), ev(1), ev(2))
	segmentize(t, f.root, "live-1", 2, true, false, ev(3), ev(4), ev(5), ev(6), ev(7), ev(8), ev(9))
	for i := 0; i < 3; i++ {
		if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, c.seqs[i].event))); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(f.root, tapestore.SegDirName, "live-1", tapestore.SegmentFileName(1))
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw[32+46+3] ^= 0x55
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	rows, err := f.walkedRows("live-1", 10, nil)
	if err != nil {
		t.Fatalf("walk stopped: %v", err)
	}
	if want := f.referenceRows("live-1", 10); !reflect.DeepEqual(want, rows) {
		t.Fatal("rows differ from the per-event reference")
	}
	for _, r := range rows {
		if !r.EventReadable {
			t.Fatalf("sequence %d unreadable", r.Sequence)
		}
	}
}

// X5: a computer whose view cannot open (a damaged index) is walked with the
// per-event reads.
func TestX5WalkOfAComputerWhoseViewCannotOpenUsesPerEventReads(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	c := f.chain("live-1", 6, tapeAged)
	var a, b []segItem
	for i, s := range c.seqs {
		it := segItem{tapestore.NSEvent, s.event}
		if i < 3 {
			a = append(a, it)
		} else {
			b = append(b, it)
		}
	}
	segmentize(t, f.root, "live-1", 1, true, true, a...) // legacy copies stay
	segmentize(t, f.root, "live-1", 2, true, true, b...)
	p := filepath.Join(f.root, tapestore.SegDirName, "live-1", tapestore.IndexFileName(1))
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 0x01
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	opened := true
	f.svc.tapeSweep.walkViewDone = func(_ string, v *tapestore.SegmentView) { opened = v != nil }
	rows, err := f.walkedRows("live-1", 6, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opened {
		t.Fatal("the view opened over a damaged index")
	}
	for _, r := range rows {
		if !r.EventReadable {
			t.Fatalf("sequence %d unreadable (legacy copies exist)", r.Sequence)
		}
	}
}

// X6: the same fixtures walked with the digest index on (the fixtures are
// shorter than tapeWalkIndexMinEvents, so the index is forced).
func TestX6IndexedWalkGivesTheSameRows(t *testing.T) {
	old := tapeWalkIndexMinEvents
	tapeWalkIndexMinEvents = 0
	defer func() { tapeWalkIndexMinEvents = old }()
	t.Run("rows", TestX1WalkReadsThroughOneViewWithTheSameRows)
	t.Run("corrupt stop", TestX2WalkStopsOnADamagedSegmentRecord)
	t.Run("growth", TestX3WalkFindsEventsWrittenToSegmentsDuringTheWalk)
	t.Run("damaged older duplicate", TestX4WalkDoesNotStopOnADamagedOlderDuplicate)
	t.Run("view cannot open", TestX5WalkOfAComputerWhoseViewCannotOpenUsesPerEventReads)
}

// X7: a references walk in several chunks shares one indexed view, and the
// tape grows between chunks: the later chunks' events are appended to new
// segments after the view was opened.
func TestX7SharedIndexedViewAcrossRefChunksSeesTheGrowingTape(t *testing.T) {
	old := tapeWalkIndexMinEvents
	tapeWalkIndexMinEvents = 0
	defer func() { tapeWalkIndexMinEvents = old }()
	f := newTapeFixture(t)
	f.live("live-1")
	f.svc.tapeSweep.refChunkEvents = 2
	c := f.chain("live-1", 10, tapeAged)
	orphan := f.put(tapeNSPayload, []byte("orphan payload x7"), tapeAged)
	app := tapestore.NewAppender(f.root)
	appendEv := func(i int) {
		p := f.path(tapeSweepRel(tapeNSEvent, c.seqs[i].event))
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := app.Append("live-1", tapestore.Item{NS: tapestore.NSEvent, Body: raw}); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 4; i++ {
		appendEv(i)
	}
	if err := app.Seal("live-1"); err != nil {
		t.Fatal(err)
	}
	var reloads, fallbacks uint64
	var entries int
	f.svc.tapeSweep.walkViewDone = func(_ string, v *tapestore.SegmentView) {
		if v != nil {
			reloads, fallbacks, entries = v.Reloads(), v.Fallbacks(), v.IndexEntries()
		}
	}
	grown := false
	f.svc.tapeSweep.afterRefChunk = func(_ string, end uint64) error {
		if end == 2 && !grown {
			grown = true
			for i := 4; i < 8; i++ { // sealed new segment
				appendEv(i)
			}
			if err := app.Seal("live-1"); err != nil {
				t.Fatal(err)
			}
			for i := 8; i < 10; i++ { // open new segment
				appendEv(i)
			}
		}
		return nil
	}
	rep := f.dryRun(TapeSweepRequest{Sweeps: []string{TapeSweepOrphans}})
	if rep.Orphans[tapeNSPayload].Refused != "" {
		t.Fatalf("the walk refused: %s", rep.Orphans[tapeNSPayload].Refused)
	}
	if got := listedDigests(f.manifestItems(rep), TapeSweepOrphans); !got[orphan] || len(got) != 1 {
		t.Fatalf("manifest lists %v, want only the orphan", got)
	}
	if entries == 0 || reloads == 0 || fallbacks != 0 {
		t.Fatalf("index entries %d reloads %d fallbacks %d: want an index, a clean reload and no fallback", entries, reloads, fallbacks)
	}
}
