package tapestore

// Round 1 review of the view read (both seats): the outcome of ReadObject must
// not depend on the hint or on walk order. Semantics under test:
//
//   For any tape and digest, ReadObject returns the verified bytes if ANY
//   segment holds an intact copy (whichever probe finds it), otherwise the
//   error of the newest damaged copy, otherwise absent; then the legacy file
//   exactly as Reader.Read does. Reader.Read decides by the newest copy alone.
//   The two differ in one order-independent case only: a damaged copy in a
//   newer segment than an intact copy. Reader.Read reports the damage; the
//   view returns the intact copy, which was checked against the digest and
//   the record crc.
//
//  W5  A duplicate digest with the damage in the older copy, and in the newer
//      copy, gives the same outcome whatever segment the hint is on.
//  W6  Randomized tapes (duplicates, damage, legacy-only, rotten legacy,
//      absent) agree with the model for every hint, and with Reader.Read
//      except in the one documented case.
//  W7  A segment or index file gone under the view is answered exactly as
//      Reader.Read answers for the new listing, never as a raw path error.
//  W8  A reload that succeeds keeps reads on the view (Reloads, no Fallbacks);
//      one that fails is answered by Reader.Read (Fallbacks).

import (
	"bytes"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func damageRecord(t *testing.T, root, id string, segno uint64, entries map[openKey]idxEntry, b []byte) {
	t.Helper()
	off := entries[openKey{NSEvent, digestBytes(b)}].off
	patchFile(t, segPath(root, id, segno), func(raw []byte) []byte { raw[off+recHeaderSize+3] ^= 0x10; return raw })
}

func TestW5DuplicateWithDamageGivesTheSameOutcomeForEveryHint(t *testing.T) {
	for _, damagedSeg := range []uint64{1, 2} {
		root := t.TempDir()
		const id = "comp-w5"
		dup := body("duplicate event %s", strings.Repeat("d", 50))
		other := body("other event")
		e1 := build(t, root, id, 1, true, rec{NSEvent, dup}, rec{NSEvent, other})
		e2 := build(t, root, id, 2, true, rec{NSEvent, dup})
		build(t, root, id, 3, false, rec{NSEvent, body("tail")})
		if damagedSeg == 1 {
			damageRecord(t, root, id, 1, e1, dup)
		} else {
			damageRecord(t, root, id, 2, e2, dup)
		}
		reader := NewReader(root)
		_, _, rerr := reader.Read(id, NSEvent, sum(dup))
		if (damagedSeg == 2) != errors.Is(rerr, ErrCorrupt) {
			t.Fatalf("damage in segment %d: Reader.Read = %v", damagedSeg, rerr)
		}
		for hint := 0; hint < 3; hint++ {
			v, err := NewReader(root).OpenView(id)
			if err != nil {
				t.Fatal(err)
			}
			v.hint = hint
			got, src, err := v.ReadObject(NSEvent, sum(dup))
			if err != nil || src != SourceSegment || !bytes.Equal(got, dup) {
				t.Fatalf("damage in segment %d, hint %d: %v %v, want the intact copy", damagedSeg, hint, src, err)
			}
		}
	}
	// Both copies damaged: the error, whatever the hint.
	root := t.TempDir()
	const id = "comp-w5b"
	dup := body("both damaged")
	e1 := build(t, root, id, 1, true, rec{NSEvent, dup})
	e2 := build(t, root, id, 2, true, rec{NSEvent, dup})
	build(t, root, id, 3, false, rec{NSEvent, body("tail")})
	damageRecord(t, root, id, 1, e1, dup)
	damageRecord(t, root, id, 2, e2, dup)
	_, _, want := NewReader(root).Read(id, NSEvent, sum(dup))
	for hint := 0; hint < 3; hint++ {
		v, _ := NewReader(root).OpenView(id)
		v.hint = hint
		_, _, got := v.ReadObject(NSEvent, sum(dup))
		if !errors.Is(got, ErrCorrupt) || got.Error() != want.Error() {
			t.Fatalf("hint %d: %v, want %v", hint, got, want)
		}
	}
}

type modelCopy struct {
	segno   uint64
	damaged bool
}

func TestW6RandomTapesAgreeWithTheModelForEveryHint(t *testing.T) {
	shadowed := 0
	for seed := int64(0); seed < 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		root := t.TempDir()
		const id = "comp-w6"
		nseg := 3 + rng.Intn(5)
		bodies := make([][]byte, 14)
		for i := range bodies {
			bodies[i] = body("w6 seed %d body %d %s", seed, i, strings.Repeat("z", 20+rng.Intn(40)))
		}
		copies := make([][]modelCopy, len(bodies))
		for s := 1; s <= nseg; s++ {
			var recs []rec
			var idxs []int
			for i := range bodies {
				if rng.Intn(4) == 0 {
					recs = append(recs, rec{NSEvent, bodies[i]})
					idxs = append(idxs, i)
				}
			}
			if len(recs) == 0 {
				recs, idxs = []rec{{NSEvent, bodies[0]}}, []int{0}
			}
			entries := build(t, root, id, uint64(s), s < nseg, recs...)
			for _, i := range idxs {
				c := modelCopy{segno: uint64(s)}
				if s < nseg && rng.Intn(4) == 0 {
					damageRecord(t, root, id, uint64(s), entries, bodies[i])
					c.damaged = true
				}
				copies[i] = append(copies[i], c)
			}
		}
		legacy := make([]int, len(bodies)) // 0 none, 1 intact, 2 rotten
		for i := range bodies {
			legacy[i] = rng.Intn(3)
			switch legacy[i] {
			case 1:
				legacyPut(t, root, NSEvent, bodies[i])
			case 2:
				p := filepath.Join(root, filepath.FromSlash(NSEvent.LegacyRel(sum(bodies[i]))))
				if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("rotten"), 0o640); err != nil {
					t.Fatal(err)
				}
			}
		}
		absent := sum(body("absent %d", seed))
		for i := -1; i < len(bodies); i++ {
			digest := absent
			var want readOutcome
			var cs []modelCopy
			if i >= 0 {
				digest, cs = sum(bodies[i]), copies[i]
			}
			intact, newestDamaged := false, false
			for _, c := range cs {
				if c.damaged {
					newestDamaged = true
				} else {
					intact = true
				}
			}
			switch {
			case intact:
				want = readOutcome{body: bodies[i], src: SourceSegment}
			default:
				// no segment copy, or only damaged ones: the legacy file
				switch {
				case i >= 0 && legacy[i] == 1:
					want = readOutcome{body: bodies[i], src: SourceLegacy}
				case i >= 0 && legacy[i] == 2 && !newestDamaged:
					want = readOutcome{err: ErrDigestMismatch}
				case newestDamaged:
					want = readOutcome{err: ErrCorrupt}
				default:
					want = readOutcome{err: ErrNotFound}
				}
			}
			ro := NewReader(root)
			var ref readOutcome
			ref.body, ref.src, ref.err = ro.Read(id, NSEvent, digest)
			// The documented difference: a damaged copy newer than an intact one.
			docDiff := false
			if intact {
				newest := cs[len(cs)-1]
				docDiff = newest.damaged
			}
			if docDiff {
				shadowed++
			}
			for hint := 0; hint <= nseg; hint++ {
				v, err := NewReader(root).OpenView(id)
				if err != nil {
					t.Fatal(err)
				}
				v.hint = hint
				var got readOutcome
				got.body, got.src, got.err = v.ReadObject(NSEvent, digest)
				if want.err == nil {
					if got.err != nil || got.src != want.src || !bytes.Equal(got.body, want.body) {
						t.Fatalf("seed %d body %d hint %d: got %+v, model %+v", seed, i, hint, got, want)
					}
				} else if got.err == nil || !errors.Is(got.err, want.err) {
					t.Fatalf("seed %d body %d hint %d: got %+v, model %+v", seed, i, hint, got, want)
				}
				if !docDiff && !sameOutcome(ref, got) {
					t.Fatalf("seed %d body %d hint %d: view %+v, Reader.Read %+v", seed, i, hint, got, ref)
				}
			}
		}
	}
	if shadowed == 0 {
		t.Fatal("no generated tape exercised a damaged copy newer than an intact one")
	}
}

func TestW7SegmentOrIndexGoneUnderTheViewIsAnsweredLikeReaderRead(t *testing.T) {
	for _, removeIdx := range []bool{false, true} {
		root := t.TempDir()
		const id = "comp-w7"
		a, b, c := body("in one"), body("in two"), body("in three")
		build(t, root, id, 1, true, rec{NSEvent, a})
		build(t, root, id, 2, true, rec{NSEvent, b})
		build(t, root, id, 3, true, rec{NSEvent, c})
		build(t, root, id, 4, false, rec{NSEvent, body("tail")})
		v, err := NewReader(root).OpenView(id)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := v.ReadObject(NSEvent, sum(c)); err != nil { // hint away from segment 2
			t.Fatal(err)
		}
		if err := os.Remove(segPath(root, id, 2)); err != nil {
			t.Fatal(err)
		}
		if removeIdx {
			if err := os.Remove(idxPath(root, id, 2)); err != nil {
				t.Fatal(err)
			}
		}
		for _, x := range [][]byte{b, a, c, body("never")} {
			var want, got readOutcome
			want.body, want.src, want.err = NewReader(root).Read(id, NSEvent, sum(x))
			got.body, got.src, got.err = v.ReadObject(NSEvent, sum(x))
			// Reader.Read lists the directory on every call and so reports a
			// .seg gone under its .idx for every digest. The view trusts its
			// listing until a read has no intact copy or touches the gone
			// file, so for a digest in another segment it returns the intact
			// bytes. That is the one allowed difference here.
			intactElsewhere := got.err == nil && got.src == SourceSegment && bytes.Equal(got.body, x) && errors.Is(want.err, ErrCorrupt)
			if !sameOutcome(want, got) && !intactElsewhere {
				t.Fatalf("removeIdx=%v %q: Reader.Read %+v, view %+v", removeIdx, x, want, got)
			}
			if got.err != nil && !errors.Is(got.err, ErrCorrupt) && !errors.Is(got.err, ErrNotFound) {
				t.Fatalf("removeIdx=%v %q: a raw error reached the caller: %v", removeIdx, x, got.err)
			}
		}
	}
}

func TestW8ReloadKeepsReadsOnTheViewAndAFailedReloadFallsBack(t *testing.T) {
	root := t.TempDir()
	const id = "comp-w8"
	a := newTestAppender(root, 1<<30, 1000)
	for i := 0; i < 4; i++ {
		mustAppend(t, a, id, Item{NSEvent, body("w8 early %d", i)})
	}
	v, err := NewReader(root).OpenView(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Seal(id); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		mustAppend(t, a, id, Item{NSEvent, body("w8 late %d", i)})
	}
	for i := 0; i < 4; i++ {
		if _, src, err := v.ReadObject(NSEvent, sum(body("w8 late %d", i))); err != nil || src != SourceSegment {
			t.Fatalf("late %d: %v %v", i, src, err)
		}
	}
	if v.Reloads() == 0 || v.Fallbacks() != 0 {
		t.Fatalf("reloads %d fallbacks %d, want a clean reload", v.Reloads(), v.Fallbacks())
	}
	// A new unsealed segment below a higher one cannot load: the Reader answers.
	build(t, root, id, 9, false, rec{NSEvent, body("w8 nine")})
	build(t, root, id, 10, false, rec{NSEvent, body("w8 ten")})
	if _, src, err := v.ReadObject(NSEvent, sum(body("w8 ten"))); err != nil || src != SourceSegment {
		t.Fatalf("after a failed reload: %v %v", src, err)
	}
	if v.Fallbacks() == 0 {
		t.Fatal("the failed reload was not counted as a fallback")
	}
}
