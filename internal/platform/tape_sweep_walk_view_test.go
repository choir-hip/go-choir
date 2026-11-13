package platform

// The reference walk reads each chain event through one view per computer
// (docs/problems/tape-sweep-walk-probes-every-segment-per-event-2026-11-13.md).
// The reference set must be byte-identical to what a per-event Reader.Read
// walk gives. Failure list:
//
//  X1  Over several sealed segments and an open one, with events out of
//      segment order, a duplicate record in two segments, an event missing
//      everywhere, an event whose legacy copy does not hash, and an event
//      present only as a legacy file, every walked row (EventReadable,
//      PayloadRefs, pins) equals the per-event reference, and so do the
//      persisted reference blob bytes and their sha256.
//  X2  A damaged segment record stops the walk with ErrCorrupt.
//  X3  Events written to a new segment, and to a sealed open segment, while
//      the walk runs are still read.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// referenceRows walks the chain without reading events, then reads each event
// the way the walk did before the view: one Reader.Read per event.
func (f *tapeFixture) referenceRows(id string, head uint64) []tapeChainRow {
	f.t.Helper()
	var rows []tapeChainRow
	if err := f.svc.walkTapeChain(f.ctx, id, 0, head, false, func(r tapeChainRow) error {
		rd := tapestore.NewReader(f.root)
		raw, _, err := rd.Read(id, tapestore.NSEvent, r.EventDigest)
		if errors.Is(err, tapestore.ErrCorrupt) {
			f.t.Fatalf("reference read: %v", err)
		}
		if err == nil && computerevent.DigestBytes(raw) == r.EventDigest {
			r.EventReadable = true
			r.PayloadRefs = hex64RE.FindAllString(string(raw), -1)
		}
		rows = append(rows, r)
		return nil
	}); err != nil {
		f.t.Fatal(err)
	}
	return rows
}

func (f *tapeFixture) walkedRows(id string, head uint64, visit func(tapeChainRow)) ([]tapeChainRow, error) {
	var rows []tapeChainRow
	err := f.svc.walkTapeChain(f.ctx, id, 0, head, true, func(r tapeChainRow) error {
		rows = append(rows, r)
		if visit != nil {
			visit(r)
		}
		return nil
	})
	return rows, err
}

func rowsBlob(rows []tapeChainRow) ([]byte, string) {
	var refs []uint64
	for _, r := range rows {
		refs = append(refs, tapeRowPrefixes(r)...)
	}
	blob := encodeTapeRefs(refs)
	sum := sha256.Sum256(blob)
	return blob, hex.EncodeToString(sum[:])
}

func TestX1WalkReadsThroughOneViewWithTheSameRows(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	c := f.chain("live-1", 40, tapeAged)
	ev := func(i int) segItem { return segItem{tapestore.NSEvent, c.seqs[i].event} }

	// Events 0..29 into segments 1..4 out of order; event 3 twice. Events
	// 30..34 stay legacy files; 35 is deleted; 36 has a legacy file that does
	// not hash; 37..39 go to the open segment.
	segmentize(t, f.root, "live-1", 1, true, true, ev(7), ev(3), ev(20), ev(0))
	segmentize(t, f.root, "live-1", 2, true, false, ev(3), ev(29), ev(1), ev(2), ev(4), ev(5), ev(6))
	for _, i := range []int{7, 20, 0} {
		if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, c.seqs[i].event))); err != nil {
			t.Fatal(err)
		}
	}
	segmentize(t, f.root, "live-1", 3, true, false, ev(21), ev(8), ev(9), ev(10), ev(11), ev(12), ev(13), ev(14), ev(15), ev(16), ev(17), ev(18), ev(19))
	segmentize(t, f.root, "live-1", 4, true, false, ev(22), ev(23), ev(24), ev(25), ev(26), ev(27), ev(28))
	segmentize(t, f.root, "live-1", 5, false, false, ev(37), ev(38), ev(39))
	if err := os.Remove(f.path(tapeSweepRel(tapeNSEvent, c.seqs[35].event))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path(tapeSweepRel(tapeNSEvent, c.seqs[36].event)), []byte("rotten"), 0o640); err != nil {
		t.Fatal(err)
	}

	want := f.referenceRows("live-1", 40)
	got, err := f.walkedRows("live-1", 40, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		for i := range want {
			if i < len(got) && !reflect.DeepEqual(want[i], got[i]) {
				t.Fatalf("row %d differs:\nwant %+v\n got %+v", i+1, want[i], got[i])
			}
		}
		t.Fatalf("rows differ: want %d, got %d", len(want), len(got))
	}
	unreadable := 0
	for _, r := range got {
		if !r.EventReadable {
			unreadable++
		}
	}
	if unreadable != 2 {
		t.Fatalf("%d unreadable events, want the missing one and the rotten one", unreadable)
	}
	wb, ws := rowsBlob(want)
	gb, gs := rowsBlob(got)
	if string(wb) != string(gb) || ws != gs {
		t.Fatalf("persisted reference bytes differ: %s vs %s", ws, gs)
	}
}

func TestX2WalkStopsOnADamagedSegmentRecord(t *testing.T) {
	f := newTapeFixture(t)
	f.live("live-1")
	c := f.chain("live-1", 12, tapeAged)
	var a, b []segItem
	for i, s := range c.seqs {
		it := segItem{tapestore.NSEvent, s.event}
		if i < 6 {
			a = append(a, it)
		} else {
			b = append(b, it)
		}
	}
	segmentize(t, f.root, "live-1", 1, true, false, a...)
	segmentize(t, f.root, "live-1", 2, true, false, b...)
	corruptSegments(t, f.root, "live-1")
	_, err := f.walkedRows("live-1", 12, nil)
	if !errors.Is(err, tapestore.ErrCorrupt) {
		t.Fatalf("walk over a damaged segment: %v, want ErrCorrupt", err)
	}
}
