package tapestore

import (
	"fmt"
	"testing"
)

// V5  SegmentLocs(i) covers exactly the union of Bucket(0..255) for sealed and
// open segments, sorted by offset.
func TestV5SegmentLocsIsTheUnionOfBucketsSortedByOffset(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, DefaultSealBytes, 7)
	for i := 0; i < 40; i++ {
		mustAppend(t, a, "comp-v", Item{NS: NS(1 + i%3), Body: body("record %d of the segment locs test", i)})
	}
	v, err := NewReader(root).OpenView("comp-v")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for b := 0; b < 256; b++ {
		locs, err := v.Bucket(byte(b))
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range locs {
			want[fmt.Sprintf("%d/%d/%x/%d", l.Segno, l.NS, l.Digest, l.Off)] = l.Sealed
		}
	}
	got := map[string]bool{}
	sealed, open := 0, 0
	for i := 0; i < v.Segments(); i++ {
		locs, err := v.SegmentLocs(i)
		if err != nil {
			t.Fatal(err)
		}
		for j, l := range locs {
			if j > 0 && locs[j-1].Off >= l.Off {
				t.Fatalf("segment %d: offsets not strictly increasing at %d", i, j)
			}
			k := fmt.Sprintf("%d/%d/%x/%d", l.Segno, l.NS, l.Digest, l.Off)
			if _, dup := got[k]; dup {
				t.Fatalf("duplicate %s", k)
			}
			got[k] = l.Sealed
			if l.Sealed {
				sealed++
			} else {
				open++
			}
		}
	}
	if len(got) != len(want) {
		t.Fatalf("SegmentLocs found %d records, Bucket found %d", len(got), len(want))
	}
	for k, s := range want {
		if gs, ok := got[k]; !ok || gs != s {
			t.Fatalf("record %s missing or differently sealed", k)
		}
	}
	if sealed == 0 || open == 0 {
		t.Fatalf("want sealed and open, got sealed=%d open=%d", sealed, open)
	}
	if _, err := v.SegmentLocs(v.Segments()); err == nil {
		t.Fatal("out-of-range segment index must fail")
	}
}
