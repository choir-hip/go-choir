package taperepack

// Failure modes of the compaction backfill (compact.go), written first:
//
//  B1 A dry run measures and decides and writes nothing (no inode changes).
//  B2 --apply without a named computer (or --all-computers) is refused
//     before anything is read.
//  B3 Apply rewrites every sealed segment of the named computers, leaves the
//     open segment and unnamed computers alone, and every record reads the
//     same afterwards.
//  B4 A segment that cannot be verified is counted as an error and left as
//     it was; the run goes on to the next.
//  B5 Measure (compsize) is summed before and after over compacted segments.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

func sealedFixture(t *testing.T, root, id string, segs int) [][]byte {
	t.Helper()
	a := tapestore.NewAppender(root)
	var bodies [][]byte
	for s := 0; s < segs; s++ {
		for i := 0; i < 5; i++ {
			b := []byte(fmt.Sprintf(`{"computer":%q,"seg":%d,"i":%d}`, id, s, i))
			if err := a.Append(id, tapestore.Item{NS: tapestore.NSEvent, Body: b}); err != nil {
				t.Fatal(err)
			}
			bodies = append(bodies, b)
		}
		if err := a.Seal(id); err != nil {
			t.Fatal(err)
		}
	}
	// One more record leaves an open segment.
	b := []byte(`{"open":true,"computer":"` + id + `"}`)
	if err := a.Append(id, tapestore.Item{NS: tapestore.NSEvent, Body: b}); err != nil {
		t.Fatal(err)
	}
	return append(bodies, b)
}

func inodes(t *testing.T, root, id string) map[string]os.FileInfo {
	t.Helper()
	dir := filepath.Join(root, tapestore.SegDirName, id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]os.FileInfo{}
	for _, e := range entries {
		fi, err := os.Stat(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = fi
	}
	return out
}

func TestCompactBackfill(t *testing.T) {
	root := t.TempDir()
	a := sealedFixture(t, root, "computer-a", 3)
	b := sealedFixture(t, root, "computer-b", 2)
	beforeA, beforeB := inodes(t, root, "computer-a"), inodes(t, root, "computer-b")

	// B1: dry run over everything.
	rep, err := Compact(CompactOptions{Root: root, MinAvgExtentBytes: -1})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Segments != 5 || rep.WouldCompact != 5 || rep.Compacted != 0 || rep.Errors != 0 {
		t.Fatalf("dry run %+v", rep)
	}
	for name, fi := range inodes(t, root, "computer-a") {
		if !os.SameFile(fi, beforeA[name]) {
			t.Fatalf("dry run rewrote %s", name)
		}
	}

	// B2.
	if _, err := Compact(CompactOptions{Root: root, Apply: true}); err == nil {
		t.Fatal("apply without a computer accepted")
	}

	// B3 + B5: apply to computer-a only, with a fake measure.
	measured := 0
	rep, err = Compact(CompactOptions{Root: root, Computers: []string{"computer-a"}, Apply: true, MinAvgExtentBytes: -1, VerifyRecords: true, Pause: 1,
		Measure: func(path string) (int64, error) { measured++; fi, err := os.Stat(path); return fi.Size() * 2, err }})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Compacted != 3 || rep.Errors != 0 || measured != 6 || rep.DiskBefore != rep.DiskAfter || rep.DiskBefore <= 0 {
		t.Fatalf("apply %+v (measured %d)", rep, measured)
	}
	after := inodes(t, root, "computer-a")
	for _, segno := range []uint64{1, 2, 3} {
		n := tapestore.SegmentFileName(segno)
		if os.SameFile(after[n], beforeA[n]) {
			t.Fatalf("%s not rewritten", n)
		}
		if i := tapestore.IndexFileName(segno); !os.SameFile(after[i], beforeA[i]) {
			t.Fatalf("index %s touched", i)
		}
	}
	if open := tapestore.SegmentFileName(4); !os.SameFile(after[open], beforeA[open]) {
		t.Fatal("open segment touched")
	}
	for name, fi := range inodes(t, root, "computer-b") {
		if !os.SameFile(fi, beforeB[name]) {
			t.Fatalf("unnamed computer touched: %s", name)
		}
	}
	r := tapestore.NewReader(root)
	for id, bodies := range map[string][][]byte{"computer-a": a, "computer-b": b} {
		for _, body := range bodies {
			got, src, err := r.Read(id, tapestore.NSEvent, digestHex(body))
			if err != nil || src != tapestore.SourceSegment || !bytes.Equal(got, body) {
				t.Fatalf("%s: %v", id, err)
			}
		}
	}

	// B4: a damaged index refuses that segment and the run goes on.
	idx := filepath.Join(root, tapestore.SegDirName, "computer-b", tapestore.IndexFileName(1))
	raw, _ := os.ReadFile(idx)
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(idx, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	rep, err = Compact(CompactOptions{Root: root, AllComputers: true, Apply: true, MinAvgExtentBytes: -1, Pause: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Errors != 1 || rep.Compacted != 4 {
		t.Fatalf("damaged index run %+v", rep)
	}
	if fi, _ := os.Stat(filepath.Join(root, tapestore.SegDirName, "computer-b", tapestore.SegmentFileName(1))); !os.SameFile(fi, beforeB[tapestore.SegmentFileName(1)]) {
		t.Fatal("segment with a damaged index was rewritten")
	}
}

func digestHex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
