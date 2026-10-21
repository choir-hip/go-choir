package tapestore

// Stage A2 failure modes for the production segment writer (Appender),
// written before the code. Section 4.4, appendix A.5 and section 9.6 item 1
// of docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md.
//
//  W1  A batch of records is readable by the Reader from the segment, byte
//      for byte, and creates no legacy file. One Append is one fsync.
//  W2  The same (namespace, body) appended twice is one record, whether it
//      repeats inside one call or across calls of one writer. A new writer
//      writes it again, in its own segment (it never trusts a segment it did
//      not write; see the fail-stop tests).
//  W3  A torn tail or a segment whose header never landed, found by a new
//      writer, is cut, counted and sealed (appender_failstop_test.go, S6);
//      the torn record was never acknowledged, so it stays absent.
//  W4  Any other damage on open refuses with ErrCorrupt and changes no byte
//      on disk: a bad record with valid records after it, a bad segment
//      header, an intact record of an unsupported version, a segment that
//      belongs to another computer, an unsealed segment below the highest
//      one, an index with no segment, a damaged index on the sealed highest
//      segment. Nothing is silently truncated.
//  W5  Rotation at the record limit seals the segment (writes its index) and
//      starts the next; at the byte limit likewise. The limits are checked
//      before a batch, so one call's records are never split across
//      segments.
//  W6  A new writer never appends to an existing segment: it seals the
//      unsealed highest one and starts the next number, or starts the next
//      number after a sealed one.
//  W7  Goroutines writing the same computer are serialized: no torn record,
//      no duplicate, every record resolvable, under -race. Two Appender
//      values over one root (two owners of the directory lock) serialize too.
//  W8  (replaced by the fail-stop tests, appender_failstop_test.go S4/S5: a
//      failed write or fsync returns an error and poisons the writer.)
//  W9  Invalid computer ids, namespaces and empty batches are refused before
//      any file is created.
//  W10 Only internal/platform may name the production writer, and no code
//      outside this package may name the fixture writer.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

func newTestAppender(root string, sealBytes int64, sealRecords int) *Appender {
	return newAppender(root, sealBytes, sealRecords)
}

func mustAppend(t *testing.T, a *Appender, id string, items ...Item) {
	t.Helper()
	if err := a.Append(id, items...); err != nil {
		t.Fatalf("Append: %v", err)
	}
}

func segNames(t *testing.T, root, id string) []string {
	t.Helper()
	entries, err := os.ReadDir(segDir(root, id))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func scanCount(t *testing.T, root, id string) int {
	t.Helper()
	n := 0
	if err := NewReader(root).Scan(id, func(Record) error { n++; return nil }); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return n
}

func legacyFileCount(t *testing.T, root string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(filepath.Join(root, "sha256"), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func TestAppenderWritesReadableRecordsInOneFsync(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	ev, pl, rc := body(`{"event":"projection"}`), body("projection batch payload"), body(`{"receipt":"pin"}`)
	mustAppend(t, a, cid, Item{NSPayload, pl}, Item{NSReceipt, rc}, Item{NSEvent, ev})
	r := NewReader(root)
	for _, c := range []struct {
		ns NS
		b  []byte
	}{{NSEvent, ev}, {NSPayload, pl}, {NSReceipt, rc}} {
		if src := mustRead(t, r, cid, c.ns, c.b); src != SourceSegment {
			t.Fatalf("%s served from %v, want the segment", c.ns, src)
		}
	}
	if legacyFileCount(t, root) != 0 {
		t.Fatal("the segment writer created legacy files")
	}
	if names := segNames(t, root, cid); len(names) != 1 || names[0] != SegmentFileName(1) {
		t.Fatalf("files = %v, want exactly the open segment", names)
	}
	st := a.Stats()
	if st.Appends != 1 || st.Records != 3 || st.Syncs != 1 || st.SegmentsCreated != 1 || st.IndexesCreated != 0 {
		t.Fatalf("stats = %+v", st)
	}
	mustAppend(t, a, cid, Item{NSPayload, body("second")})
	if st := a.Stats(); st.Syncs != 2 || st.SegmentsCreated != 1 {
		t.Fatalf("second append: stats = %+v, want one fsync per Append and no new file", st)
	}
	if scanCount(t, root, cid) != 4 {
		t.Fatal("scan count")
	}
}

func TestAppenderDuplicatesAreNoops(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	pl := body("same bytes")
	mustAppend(t, a, cid, Item{NSPayload, pl}, Item{NSPayload, pl})
	mustAppend(t, a, cid, Item{NSPayload, pl})
	// Same bytes, other namespace: a different key, a record.
	mustAppend(t, a, cid, Item{NSEvent, pl})
	if n := scanCount(t, root, cid); n != 2 {
		t.Fatalf("records = %d, want 2", n)
	}
	// A new writer does not trust the segment it finds: it writes the same
	// records again, in a segment of its own, and leaves the old one alone.
	before, _ := os.Stat(segPath(root, cid, 1))
	b := NewAppender(root)
	mustAppend(t, b, cid, Item{NSPayload, pl}, Item{NSEvent, pl})
	after, _ := os.Stat(segPath(root, cid, 1))
	if before.Size() != after.Size() {
		t.Fatalf("a new writer appended to the old segment: %d -> %d bytes", before.Size(), after.Size())
	}
	if n := scanCount(t, root, cid); n != 4 {
		t.Fatalf("records = %d, want 2 in each of the two segments", n)
	}
	mustRead(t, NewReader(root), cid, NSPayload, pl)
}

func appendTail(t *testing.T, path string, b []byte) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		t.Fatal(err)
	}
}

// A short file that is not a prefix of this segment's header is damage, not a
// torn creation (the torn creation itself is appender_failstop_test.go, S6).
func TestAppenderRefusesAShortSegmentThatIsNotAHeaderPrefix(t *testing.T) {
	root := t.TempDir()
	mustAppend(t, NewAppender(root), cid, Item{NSPayload, body("x")})
	patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[1] ^= 0xff; return b[:10] })
	if err := NewAppender(root).Append(cid, Item{NSPayload, body("y")}); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("short file that is not a header prefix = %v, want ErrCorrupt", err)
	}
}

func TestAppenderRefusesAnyOtherDamageAndChangesNothing(t *testing.T) {
	three := func(root string) {
		mustAppend(t, NewAppender(root), cid, Item{NSPayload, body("rec one, first")}, Item{NSPayload, body("rec two")}, Item{NSPayload, body("rec three")})
	}
	cases := map[string]func(t *testing.T, root string){
		"bad record with valid records after it": func(t *testing.T, root string) {
			three(root)
			patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[segHeaderSize+recHeaderSize+2] ^= 0x40; return b })
		},
		"bad segment header": func(t *testing.T, root string) {
			three(root)
			patchFile(t, segPath(root, cid, 1), func(b []byte) []byte { b[0] ^= 0xff; return b })
		},
		"intact record of an unsupported version": func(t *testing.T, root string) {
			three(root)
			patchFile(t, segPath(root, cid, 1), func(b []byte) []byte {
				off := segHeaderSize
				b[off+4] = 9
				refixRecordCRC(b, off)
				return b
			})
		},
		"intact record of an unknown namespace in the tail": func(t *testing.T, root string) {
			three(root)
			patchFile(t, segPath(root, cid, 1), func(b []byte) []byte {
				off := len(b) - (recHeaderSize + len("rec three"))
				b[off+5] = 9
				refixRecordCRC(b, off)
				return b
			})
		},
		"a segment of another computer": func(t *testing.T, root string) {
			mustAppend(t, NewAppender(root), "computer-b", Item{NSPayload, body("b")})
			if err := os.MkdirAll(segDir(root, cid), 0o750); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(segPath(root, "computer-b", 1))
			if err := os.WriteFile(segPath(root, cid, 1), raw, 0o640); err != nil {
				t.Fatal(err)
			}
		},
		"an unsealed segment below the highest": func(t *testing.T, root string) {
			build(t, root, cid, 1, false, rec{NSPayload, body("a")})
			build(t, root, cid, 2, false, rec{NSPayload, body("b")})
		},
		"an index with no segment": func(t *testing.T, root string) {
			build(t, root, cid, 1, true, rec{NSPayload, body("a")})
			if err := os.Remove(segPath(root, cid, 1)); err != nil {
				t.Fatal(err)
			}
		},
		"a damaged index on the sealed highest segment": func(t *testing.T, root string) {
			build(t, root, cid, 1, true, rec{NSPayload, body("a")}, rec{NSPayload, body("b")})
			patchFile(t, idxPath(root, cid, 1), func(b []byte) []byte { b[len(b)-9] ^= 0xff; return b })
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			setup(t, root)
			snap := snapshotDir(t, segDir(root, cid))
			a := NewAppender(root)
			err := a.Append(cid, Item{NSPayload, body("must not be written")})
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("Append = %v, want ErrCorrupt", err)
			}
			if got := snapshotDir(t, segDir(root, cid)); !snapshotsEqual(snap, got) {
				t.Fatalf("a refused Append changed the store:\nbefore %v\nafter  %v", snapKeys(snap), snapKeys(got))
			}
			if st := a.Stats(); st.TornTails != 0 || st.Records != 0 {
				t.Fatalf("stats = %+v after a refusal", st)
			}
		})
	}
}

func snapshotDir(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = b
	}
	return out
}

func snapshotsEqual(a, b map[string][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || !bytes.Equal(v, w) {
			return false
		}
	}
	return true
}

func snapKeys(m map[string][]byte) []string {
	var out []string
	for k, v := range m {
		out = append(out, fmt.Sprintf("%s(%d)", k, len(v)))
	}
	sort.Strings(out)
	return out
}

func TestAppenderRotatesAtTheRecordLimit(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, 1<<30, 3)
	var bodies [][]byte
	for i := 0; i < 7; i++ {
		b := body("record number %d", i)
		bodies = append(bodies, b)
		mustAppend(t, a, cid, Item{NSPayload, b})
	}
	want := []string{IndexFileName(1), SegmentFileName(1), IndexFileName(2), SegmentFileName(2), SegmentFileName(3)}
	sort.Strings(want)
	if got := segNames(t, root, cid); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("files = %v, want %v", got, want)
	}
	r := NewReader(root)
	for _, b := range bodies {
		if src := mustRead(t, r, cid, NSPayload, b); src != SourceSegment {
			t.Fatal("not served from a segment")
		}
	}
	if scanCount(t, root, cid) != 7 {
		t.Fatal("scan count") // Scan also checks each sealed segment against its index
	}
	st := a.Stats()
	if st.SegmentsCreated != 3 || st.IndexesCreated != 2 || st.Seals != 2 || st.Syncs != 7 {
		t.Fatalf("stats = %+v", st)
	}
	if _, err := os.Stat(idxPath(root, cid, 1) + ".tmp"); err == nil {
		t.Fatal("index temp file left behind")
	}
}

func TestAppenderNeverSplitsABatchAcrossSegments(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, 1<<30, 3)
	mustAppend(t, a, cid, Item{NSPayload, body("1")}, Item{NSPayload, body("2")}, Item{NSPayload, body("3")}, Item{NSPayload, body("4")}, Item{NSPayload, body("5")})
	if got := segNames(t, root, cid); len(got) != 1 {
		t.Fatalf("files = %v, want one segment holding the whole batch", got)
	}
	mustAppend(t, a, cid, Item{NSPayload, body("6")})
	if got := segNames(t, root, cid); len(got) != 3 { // seg1 + idx1 + seg2
		t.Fatalf("files = %v, want the full segment sealed before the next batch", got)
	}
	if scanCount(t, root, cid) != 6 {
		t.Fatal("scan count")
	}
}

func TestAppenderRotatesAtTheByteLimit(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, 300, 1000)
	pad := strings.Repeat("x", 100)
	for i := 0; i < 5; i++ {
		mustAppend(t, a, cid, Item{NSPayload, body("%d-%s", i, pad)})
	}
	// 32 + 2*(46+102) = 328 >= 300: seal after two records, so 2 + 2 + 1.
	want := []string{IndexFileName(1), SegmentFileName(1), IndexFileName(2), SegmentFileName(2), SegmentFileName(3)}
	sort.Strings(want)
	if got := segNames(t, root, cid); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("files = %v, want %v", got, want)
	}
	if scanCount(t, root, cid) != 5 {
		t.Fatal("scan count")
	}
}

func TestAppenderReopenSealsTheOldSegmentAndStartsTheNext(t *testing.T) {
	root := t.TempDir()
	var all [][]byte
	put := func(a *Appender) {
		b := body("rec %d", len(all))
		all = append(all, b)
		mustAppend(t, a, cid, Item{NSPayload, b})
	}
	a := newTestAppender(root, 1<<30, 2)
	put(a)
	put(a)
	put(a) // seg1 sealed with 2, seg2 open with 1
	if got := segNames(t, root, cid); len(got) != 3 {
		t.Fatalf("files = %v", got)
	}
	b := newTestAppender(root, 1<<30, 2)
	put(b) // a new writer does not continue segment 2: it seals it and starts segment 3
	if got := segNames(t, root, cid); len(got) != 5 {
		t.Fatalf("files = %v, want segment 2 sealed and segment 3 open", got)
	}
	r := NewReader(root)
	for _, x := range all {
		mustRead(t, r, cid, NSPayload, x)
	}
	if scanCount(t, root, cid) != len(all) {
		t.Fatal("scan count")
	}

	// The highest segment is sealed (a crash between the seal and the next
	// segment's creation): the next number is used and nothing is rewritten.
	root2 := t.TempDir()
	sealed := body("in a sealed highest segment")
	build(t, root2, cid, 4, true, rec{NSPayload, sealed})
	before := snapshotDir(t, segDir(root2, cid))
	fresh := body("first record of segment 5")
	mustAppend(t, NewAppender(root2), cid, Item{NSPayload, fresh})
	for name, v := range before {
		if got, err := os.ReadFile(filepath.Join(segDir(root2, cid), name)); err != nil || !bytes.Equal(got, v) {
			t.Fatalf("sealed file %s was modified", name)
		}
	}
	if _, err := os.Stat(segPath(root2, cid, 5)); err != nil {
		t.Fatalf("segment 5 expected: %v", err)
	}
	r2 := NewReader(root2)
	mustRead(t, r2, cid, NSPayload, sealed)
	mustRead(t, r2, cid, NSPayload, fresh)
}

func TestAppenderSerializesGoroutinesOfOneComputer(t *testing.T) {
	root := t.TempDir()
	a := newTestAppender(root, 1<<30, 17)
	const workers, per = 8, 25
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < per; i++ {
				err := a.Append(cid, Item{NSPayload, body("w%d-i%d-payload", w, i)}, Item{NSReceipt, body("w%d-i%d-receipt", w, i)})
				if err != nil {
					t.Errorf("worker %d: %v", w, err)
					return
				}
			}
		}(w)
	}
	// Readers run beside the writers.
	r := NewReader(root)
	stop := make(chan struct{})
	var rg sync.WaitGroup
	rg.Add(1)
	go func() {
		defer rg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				if _, _, err := r.Read(cid, NSPayload, sum(body("w0-i0-payload"))); err != nil && !errors.Is(err, ErrNotFound) {
					t.Errorf("concurrent read: %v", err)
					return
				}
			}
		}
	}()
	wg.Wait()
	close(stop)
	rg.Wait()
	seen := map[string]bool{}
	if err := NewReader(root).Scan(cid, func(rec Record) error {
		k := fmt.Sprintf("%d/%s", rec.NS, rec.Digest)
		if seen[k] {
			return fmt.Errorf("duplicate record %s", k)
		}
		seen[k] = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != workers*per*2 {
		t.Fatalf("records = %d, want %d", len(seen), workers*per*2)
	}
	for w := 0; w < workers; w++ {
		for i := 0; i < per; i++ {
			mustRead(t, r, cid, NSPayload, body("w%d-i%d-payload", w, i))
			mustRead(t, r, cid, NSReceipt, body("w%d-i%d-receipt", w, i))
		}
	}
}

func TestAppenderTwoInstancesOverOneRootSerialize(t *testing.T) {
	root := t.TempDir()
	one, two := newTestAppender(root, 1<<30, 11), newTestAppender(root, 1<<30, 7)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			a := one
			if w%2 == 1 {
				a = two
			}
			for i := 0; i < 30; i++ {
				if err := a.Append(cid, Item{NSPayload, body("w%d-i%d", w, i)}); err != nil {
					t.Errorf("worker %d: %v", w, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	if n := scanCount(t, root, cid); n != 120 {
		t.Fatalf("records = %d, want 120", n)
	}
	// Different computers do not share a lock or a segment.
	var wg2 sync.WaitGroup
	for _, id := range []string{"computer-x", "computer-y", "computer-z"} {
		wg2.Add(1)
		go func(id string) {
			defer wg2.Done()
			for i := 0; i < 20; i++ {
				if err := one.Append(id, Item{NSPayload, body("%s-%d", id, i)}); err != nil {
					t.Errorf("%s: %v", id, err)
					return
				}
			}
		}(id)
	}
	wg2.Wait()
	for _, id := range []string{"computer-x", "computer-y", "computer-z"} {
		if n := scanCount(t, root, id); n != 20 {
			t.Fatalf("%s records = %d", id, n)
		}
	}
}

func TestAppenderRefusesBadInputBeforeCreatingAnything(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	for name, c := range map[string]struct {
		id    string
		items []Item
	}{
		"empty id":         {"", []Item{{NSPayload, body("x")}}},
		"path traversal":   {"../x", []Item{{NSPayload, body("x")}}},
		"slash":            {"a/b", []Item{{NSPayload, body("x")}}},
		"leading dot":      {".hidden", []Item{{NSPayload, body("x")}}},
		"unknown ns":       {cid, []Item{{NS(9), body("x")}}},
		"zero ns":          {cid, []Item{{NS(0), body("x")}}},
		"empty batch":      {cid, nil},
		"unknown ns later": {cid, []Item{{NSPayload, body("ok")}, {NS(7), body("x")}}},
	} {
		if err := a.Append(c.id, c.items...); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, SegDirName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused Append created tape-seg")
	}
}

// W10. The fixture writer stays out of production code everywhere; the
// production writer is named only by internal/platform and by the offline
// repack tool, internal/taperepack (copy only; A2 item 3, station FP9). The
// appender's per-computer directory lock and recovery rule are what make a
// second writing process safe; no other package may write segments.
func TestOnlyPlatformNamesTheProductionWriter(t *testing.T) {
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
	platform := filepath.Join(dir, "internal", "platform")
	repack := filepath.Join(dir, "internal", "taperepack")
	var offenders []string
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
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
		if bytes.Contains(raw, []byte("tapestore.NewAppender")) || bytes.Contains(raw, []byte("tapestore.Appender")) {
			if !strings.HasPrefix(path, platform+string(filepath.Separator)) && !strings.HasPrefix(path, repack+string(filepath.Separator)) {
				offenders = append(offenders, path)
			}
		}
		return nil
	})
	if len(offenders) > 0 {
		t.Fatalf("only internal/platform may use the production segment writer; found %v", offenders)
	}
}
