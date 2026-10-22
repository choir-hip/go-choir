package tapestore

// Bulk-read and seal additions for the repack tool (FP10, docs/problems/
// platform-artifact-small-file-growth-2026-10-19.md, "Repack tool v2"). The
// repack joins ten million legacy files against the segment store; a point
// lookup per file lists the directory and opens every index, so it needs a
// bucketed view of a computer's segments and a verified read by location.
//
//  V1  Every record of a computer, sealed and open, is found by exactly one
//      Bucket(first digest byte) call, with the namespace and length it was
//      written with, and Read returns its exact bytes.
//  V2  Read fails closed (ErrCorrupt) on a record whose bytes were damaged,
//      even when its index entry is intact.
//  V3  OpenView refuses a computer with a damaged sealed index, and a
//      non-final unsealed segment, with ErrCorrupt.
//  V4  A view of a computer with no segment directory is empty and not an error.
//  S1  Appender.Seal seals the writer's own open segment (an index appears,
//      the records still read), and the next Append starts the next segment.
//  S2  Seal with nothing open, or on a segment another writer owns, changes
//      nothing on disk.
//  S3  Seal on a poisoned Appender returns ErrAppenderPoisoned and touches
//      nothing.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestV1ViewBucketFindsEveryRecordAndReadReturnsItsBytes(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, DefaultSealBytes, 7)
	want := map[string][]byte{}
	for i := 0; i < 40; i++ {
		ns := NS(1 + i%3)
		b := body("record %d of the view test", i)
		mustAppend(t, a, "comp-v", Item{NS: ns, Body: b})
		want[fmt.Sprintf("%d/%s", ns, sum(b))] = b
	}
	v, err := NewReader(root).OpenView("comp-v")
	if err != nil {
		t.Fatalf("OpenView: %v", err)
	}
	if v.Segments() < 2 {
		t.Fatalf("want sealed and open segments, got %d", v.Segments())
	}
	seen := map[string]int{}
	sealed, open := 0, 0
	for b := 0; b < 256; b++ {
		locs, err := v.Bucket(byte(b))
		if err != nil {
			t.Fatalf("Bucket(%d): %v", b, err)
		}
		for _, l := range locs {
			if l.Digest[0] != byte(b) {
				t.Fatalf("Bucket(%d) returned digest %x", b, l.Digest)
			}
			key := fmt.Sprintf("%d/%x", l.NS, l.Digest)
			seen[key]++
			got, err := v.Read(l)
			if err != nil {
				t.Fatalf("Read %s: %v", key, err)
			}
			if string(got) != string(want[key]) || int(l.Length) != len(got) {
				t.Fatalf("Read %s: %q length %d", key, got, l.Length)
			}
			if l.Sealed {
				sealed++
			} else {
				open++
			}
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("found %d records, wrote %d", len(seen), len(want))
	}
	for k, n := range seen {
		if n != 1 {
			t.Fatalf("%s found %d times", k, n)
		}
	}
	if sealed == 0 || open == 0 {
		t.Fatalf("expected both sealed and open hits, got sealed=%d open=%d", sealed, open)
	}
}

func TestV2ViewReadFailsClosedOnADamagedRecord(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, DefaultSealBytes, 2)
	b := body("to be damaged")
	mustAppend(t, a, "comp-v", Item{NS: NSPayload, Body: b}, Item{NS: NSEvent, Body: body("neighbour")})
	mustAppend(t, a, "comp-v", Item{NS: NSEvent, Body: body("second segment")})
	v, err := NewReader(root).OpenView("comp-v")
	if err != nil {
		t.Fatal(err)
	}
	var loc SegLoc
	found := false
	for i := 0; i < 256 && !found; i++ {
		locs, _ := v.Bucket(byte(i))
		for _, l := range locs {
			if l.NS == NSPayload {
				loc, found = l, true
			}
		}
	}
	if !found {
		t.Fatal("record not located")
	}
	patchFile(t, segPath(root, "comp-v", loc.Segno), func(raw []byte) []byte {
		raw[int(loc.Off)+recHeaderSize] ^= 0xff // a body byte; the index is untouched
		return raw
	})
	if _, err := v.Read(loc); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("Read of a damaged record: %v, want ErrCorrupt", err)
	}
}

func TestV3OpenViewRefusesDamagedStores(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, DefaultSealBytes, 2)
	for i := 0; i < 6; i++ {
		mustAppend(t, a, "comp-bad", Item{NS: NSEvent, Body: body("r%d", i)})
	}
	patchFile(t, idxPath(root, "comp-bad", 1), func(raw []byte) []byte { raw[len(raw)/2] ^= 0x01; return raw })
	if _, err := NewReader(root).OpenView("comp-bad"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("damaged index: %v, want ErrCorrupt", err)
	}

	// A non-final unsealed segment is corruption too.
	root2 := t.TempDir()
	build(t, root2, "comp-gap", 1, false, rec{NSEvent, body("a")})
	build(t, root2, "comp-gap", 2, false, rec{NSEvent, body("b")})
	v, err := NewReader(root2).OpenView("comp-gap")
	if err == nil {
		for i := 0; i < 256 && err == nil; i++ {
			_, err = v.Bucket(byte(i))
		}
	}
	if !errors.Is(err, ErrCorrupt) {
		t.Fatalf("unsealed segment below the highest: %v, want ErrCorrupt", err)
	}
}

func TestV4ViewOfAComputerWithNoSegmentsIsEmpty(t *testing.T) {
	v, err := NewReader(t.TempDir()).OpenView("nobody")
	if err != nil {
		t.Fatal(err)
	}
	if v.Segments() != 0 {
		t.Fatalf("segments = %d", v.Segments())
	}
	locs, err := v.Bucket(0)
	if err != nil || len(locs) != 0 {
		t.Fatalf("Bucket = %v, %v", locs, err)
	}
}

func TestS1SealClosesTheWritersOwnSegmentAndTheNextAppendStartsAnother(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	b := body("sealed by request")
	mustAppend(t, a, "comp-s", Item{NS: NSPayload, Body: b})
	if err := a.Seal("comp-s"); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if _, err := os.Stat(idxPath(root, "comp-s", 1)); err != nil {
		t.Fatalf("no index after Seal: %v", err)
	}
	if got, src, err := NewReader(root).Read("comp-s", NSPayload, sum(b)); err != nil || src != SourceSegment || string(got) != string(b) {
		t.Fatalf("Read after Seal: %q %v %v", got, src, err)
	}
	mustAppend(t, a, "comp-s", Item{NS: NSPayload, Body: body("after")})
	if _, err := os.Stat(segPath(root, "comp-s", 2)); err != nil {
		t.Fatalf("the next Append did not start segment 2: %v", err)
	}
	if err := a.Seal("comp-s"); err != nil {
		t.Fatal(err)
	}
	if n := scanCount(t, root, "comp-s"); n != 2 {
		t.Fatalf("scan found %d records, want 2", n)
	}
}

func TestS2SealWithNothingOpenOrAnotherOwnersSegmentChangesNothing(t *testing.T) {
	root := t.TempDir()
	if err := newTestAppender(root, DefaultSealBytes, DefaultSealRecords).Seal("comp-s"); err != nil {
		t.Fatalf("Seal with nothing open: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, SegDirName, "comp-s")); err == nil {
		t.Fatal("Seal created a directory")
	}
	owner := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	mustAppend(t, owner, "comp-s", Item{NS: NSEvent, Body: body("owner's record")})
	before := segNames(t, root, "comp-s")
	other := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	if err := other.Seal("comp-s"); err != nil {
		t.Fatalf("Seal of a segment it does not own: %v", err)
	}
	if after := segNames(t, root, "comp-s"); fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("files changed: %v -> %v", before, after)
	}
	if err := other.Seal("../escape"); err == nil {
		t.Fatal("Seal accepted an invalid computer id")
	}
}

func TestS3SealOnAPoisonedAppenderRefusesAndTouchesNothing(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, DefaultSealBytes, DefaultSealRecords)
	mustAppend(t, a, "comp-s", Item{NS: NSEvent, Body: body("first")})
	a.SetSyncHookForTest(func(*os.File) error { return errors.New("injected fsync failure") })
	if err := a.Append("comp-s", Item{NS: NSEvent, Body: body("second")}); err == nil {
		t.Fatal("expected the injected failure")
	}
	before := segNames(t, root, "comp-s")
	if err := a.Seal("comp-s"); !errors.Is(err, ErrAppenderPoisoned) {
		t.Fatalf("Seal on a poisoned appender: %v", err)
	}
	if after := segNames(t, root, "comp-s"); fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("files changed: %v -> %v", before, after)
	}
}
