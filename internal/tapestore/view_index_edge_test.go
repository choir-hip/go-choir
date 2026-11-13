package tapestore

// Round 1 review of the digest index (both seats):
//
//  I4  candidates compares prefixes, not a computed upper bound: the top
//      44-bit prefix (all ones) used to wrap hi to 0 and name no segment, so
//      a record in a sealed segment read as absent. Also the prefix just
//      below it, prefix 0, an absent prefix, and a prefix collision (two
//      ordinals under one prefix: both are candidates, once each).
//  I5  A digest stored in two sealed segments with damage in either copy, or
//      both: the indexed view returns what the plain view returns.
//  I6  An absent read does not list the directory when the raw names are
//      unchanged, still reloads when a segment appears, and is not fooled by
//      an unrelated file.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func syntheticIndexView(keys []uint64, segnos []uint64) *SegmentView {
	slices.Sort(keys)
	v := &SegmentView{index: &digestIndex{segnos: segnos, keys: keys}, segAt: map[uint64]int{}}
	for i, s := range segnos {
		v.segAt[s] = i
	}
	return v
}

func digestWithPrefix(p44 uint64) [32]byte {
	var d [32]byte
	binary.BigEndian.PutUint64(d[0:8], p44<<digestIndexOrdBits|0xabcde) // low 20 bits of the 8 bytes are not in the prefix
	return d
}

func TestI4CandidatesComparePrefixesNotABound(t *testing.T) {
	const top = uint64(1)<<44 - 1
	segnos := []uint64{10, 11, 12, 13, 14}
	v := syntheticIndexView([]uint64{
		0<<digestIndexOrdBits | 0,
		(top-1)<<digestIndexOrdBits | 1,
		top<<digestIndexOrdBits | 2,
		top<<digestIndexOrdBits | 3, // a second segment under the same prefix
		5<<digestIndexOrdBits | 4,
	}, segnos)
	cases := []struct {
		name string
		p    uint64
		want []int
	}{
		{"prefix 0", 0, []int{0}},
		{"below the top", top - 1, []int{1}},
		{"the top prefix", top, []int{2, 3}},
		{"a prefix with no entry", 6, nil},
		{"a prefix between", top - 2, nil},
	}
	for _, c := range cases {
		got := v.candidates(digestWithPrefix(c.p))
		if !slices.Equal(got, c.want) {
			t.Fatalf("%s: candidates %v, want %v", c.name, got, c.want)
		}
	}
	// Two entries with the same (prefix, ordinal) are one candidate.
	dup := syntheticIndexView([]uint64{7<<digestIndexOrdBits | 0, 7<<digestIndexOrdBits | 0}, []uint64{10})
	if got := dup.candidates(digestWithPrefix(7)); !slices.Equal(got, []int{0}) {
		t.Fatalf("same segment twice: %v", got)
	}
}

func TestI5IndexedViewMatchesPlainViewOnDamagedDuplicates(t *testing.T) {
	for _, dmg := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
		root := t.TempDir()
		const id = "comp-i5"
		dup := body("indexed duplicate %s", strings.Repeat("d", 40))
		e1 := build(t, root, id, 1, true, rec{NSEvent, dup}, rec{NSEvent, body("a")})
		e2 := build(t, root, id, 2, true, rec{NSEvent, dup})
		build(t, root, id, 3, false, rec{NSEvent, body("tail")})
		if dmg[0] {
			damageRecord(t, root, id, 1, e1, dup)
		}
		if dmg[1] {
			damageRecord(t, root, id, 2, e2, dup)
		}
		plain, err := NewReader(root).OpenView(id)
		if err != nil {
			t.Fatal(err)
		}
		idx, err := NewReader(root).OpenIndexedView(id, NSEvent)
		if err != nil {
			t.Fatal(err)
		}
		var want, got readOutcome
		want.body, want.src, want.err = plain.ReadObject(NSEvent, sum(dup))
		got.body, got.src, got.err = idx.ReadObject(NSEvent, sum(dup))
		if !sameOutcome(want, got) {
			t.Fatalf("damage %v: plain %+v, indexed %+v", dmg, want, got)
		}
		if !dmg[0] || !dmg[1] {
			if got.err != nil || !bytes.Equal(got.body, dup) {
				t.Fatalf("damage %v: want the intact copy, got %+v", dmg, got)
			}
		} else if !errors.Is(got.err, ErrCorrupt) {
			t.Fatalf("both damaged: %v", got.err)
		}
		// Candidates are probed in ordinal order: a damaged older copy forces
		// the second probe; an intact older copy ends the search at once.
		if p := idx.Probes(); p < 2 && dmg[0] {
			t.Fatalf("damage %v: %d probes, want both candidates probed", dmg, p)
		}
	}
}

func TestI6AbsentReadsSkipTheListingWhileTheNamesAreUnchanged(t *testing.T) {
	root := t.TempDir()
	const id = "comp-i6"
	app := newTestAppender(root, 1<<30, 2)
	for _, w := range []string{"one", "two", "three", "four", "five"} {
		mustAppend(t, app, id, Item{NSEvent, body("%s", w)})
	}
	v, err := NewReader(root).OpenIndexedView(id, NSEvent)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, _, err := v.ReadObject(NSEvent, sum(body("absent %d", i))); !errors.Is(err, ErrNotFound) {
			t.Fatalf("absent: %v", err)
		}
	}
	if v.Absent() != 5 || v.ListingsSkipped() != 5 {
		t.Fatalf("absent %d, listings skipped %d, want 5 and 5", v.Absent(), v.ListingsSkipped())
	}
	// An unrelated file (a compaction temp) changes the raw names: the full
	// compare runs, finds the segments unchanged, and the read is still absent.
	if err := os.WriteFile(filepath.Join(root, SegDirName, id, "x.tmp"), []byte("t"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, err := v.ReadObject(NSEvent, sum(body("absent 9"))); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent with a temp file: %v", err)
	}
	if v.ListingsSkipped() != 5 || v.Reloads() != 0 {
		t.Fatalf("skipped %d reloads %d after an unrelated file", v.ListingsSkipped(), v.Reloads())
	}
	// And the names are remembered again: the next absent read skips.
	if _, _, err := v.ReadObject(NSEvent, sum(body("absent 10"))); !errors.Is(err, ErrNotFound) || v.ListingsSkipped() != 6 {
		t.Fatalf("after the temp file: %v skipped %d", err, v.ListingsSkipped())
	}
	// A record in a segment created since is found (a reload), not absent.
	if err := app.Seal(id); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, app, id, Item{NSEvent, body("grown")})
	if got, _, err := v.ReadObject(NSEvent, sum(body("grown"))); err != nil || !bytes.Equal(got, body("grown")) {
		t.Fatalf("a record in a new segment: %v", err)
	}
	if v.Reloads() == 0 {
		t.Fatal("the view did not reload for a new segment")
	}
}
