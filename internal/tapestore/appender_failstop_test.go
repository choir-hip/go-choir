package tapestore

// Fail-stop failure modes for the segment writer, written before the fix.
//
// Three review rounds each found a new hole in the same place: a writer that
// keeps going after an fsync error and later repairs the file (abandon the
// tail, remember a frontier, retire the segment, rewrite suspect records).
// The substrate rule is the PostgreSQL one since 2018: the first failed
// fsync, or failed write that leaves the segment in an unknown state, makes
// the Appender unusable for good. Recovery is a different writer's job, and
// that writer only ever reads, truncates a torn tail of, and seals a segment
// it did not write; it appends to a segment of its own.
//
//  S1  (round 3, P1) Cross-process stale retirement. Writer A fails twice and
//      goes quiet; writer B (another process: a second Appender over the same
//      directory lock) appends Q and acknowledges it; A wakes up. A must not
//      truncate, seal or otherwise touch anything, and Q stays readable.
//  S2  (round 3, P1a) No offset reuse between truncate and seal. A writer
//      that dies (or whose cleanup keeps failing) after cutting a tail and
//      before sealing must not let the next writer append at the cut offset:
//      a reader that cached the cut record would answer ErrNotFound for the
//      new equal-sized record and ErrCorrupt for the old digest.
//  S3  (round 2, P1b) Writer handoff after a failed fsync. B, asked to append
//      the record whose fsync failed under A, writes it again, in its own
//      segment; it never infers durability from readable bytes of A's
//      segment, with or without the tail surviving.
//  S4  A poisoned Appender never touches a file again: not the failed
//      segment, not another computer's, not the directory tree; every later
//      call (Append, AppendFunc, any computer) returns ErrAppenderPoisoned
//      and AppendFunc never runs its build function. A failed write is as
//      fatal as a failed fsync and leaves its partial bytes where they are.
//  S5  Any failed fsync poisons, not only the record fsync: segment creation,
//      directory entries, the seal's index and directory, and the fsync of
//      recovery itself.
//  S6  Recovery by a new Appender: an unsealed segment it did not write is
//      never appended to. It keeps only whole verified records (a torn tail
//      is cut), seals it with an index of those records, and starts a fresh
//      segment for its own writes. A sealed segment is never modified again.
//  S7  Repeated failures cost at most one sealed segment and one new segment
//      per Appender (per process); a poisoned Appender costs nothing.
//  S8  Acknowledged records are always readable afterwards, by a long-lived
//      and by a fresh reader, under any interleaving of two writers, failed
//      fsyncs with and without tail loss, and failed writes. Random seeds.
//  S9  A reader never returns a wrong record for a digest: bytes served are
//      always the bytes that hash to it; a record that was never
//      acknowledged is found whole or not found, never ErrCorrupt.
//  S10 (platform, see internal/platform/tape_segments_failstop_test.go) A
//      failed or poisoned append falls back to the legacy file path for that
//      pin, so the event is still durably stored.

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

// fstopDisk is a sync hook with a switch. While failing, every fsync returns
// EIO; with lose set it first cuts the file back to what its last successful
// fsync covered (what a dropped dirty page leaves behind). onFail runs
// before the loss, on every failure.
type fstopDisk struct {
	mu      sync.Mutex
	failing bool
	lose    bool
	onFail  func(f *os.File)
	synced  map[string]int64
}

func newFstopDisk() *fstopDisk { return &fstopDisk{synced: map[string]int64{}} }

func (d *fstopDisk) set(failing, lose bool) {
	d.mu.Lock()
	d.failing, d.lose = failing, lose
	d.mu.Unlock()
}

func (d *fstopDisk) sync(f *os.File) error {
	d.mu.Lock()
	failing, lose, onFail, synced := d.failing, d.lose, d.onFail, d.synced[f.Name()]
	d.mu.Unlock()
	if !failing {
		err := f.Sync()
		if info, serr := f.Stat(); err == nil && serr == nil {
			d.mu.Lock()
			d.synced[f.Name()] = info.Size()
			d.mu.Unlock()
		}
		return err
	}
	if onFail != nil {
		onFail(f)
	}
	if lose && synced > 0 {
		_ = f.Truncate(synced)
	}
	return syscall.EIO
}

// fstopTree is every directory and file under root with its bytes.
func fstopTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			out["dir:"+rel] = ""
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func fstopSame(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	var diff []string
	for k, v := range before {
		if w, ok := after[k]; !ok {
			diff = append(diff, "removed "+k)
		} else if v != w {
			diff = append(diff, fmt.Sprintf("changed %s (%d -> %d bytes)", k, len(v), len(w)))
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			diff = append(diff, "created "+k)
		}
	}
	if len(diff) > 0 {
		t.Fatalf("%s touched the store: %v", what, diff)
	}
}

func fstopSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

// fstopRecords scans a computer and returns, per (ns, digest), the segment
// numbers that hold it.
func fstopRecords(t *testing.T, root, id string) map[openKey][]uint64 {
	t.Helper()
	out := map[openKey][]uint64{}
	if err := NewReader(root).Scan(id, func(r Record) error {
		d := sha256Of(r.Digest)
		out[openKey{r.NS, d}] = append(out[openKey{r.NS, d}], r.Segno)
		return nil
	}); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return out
}

func sha256Of(hexd string) (d [32]byte) {
	if err := decodeHex32(&d, hexd); err != nil {
		panic(err)
	}
	return d
}

func segsOf(m map[openKey][]uint64, ns NS, b []byte) []uint64 {
	return m[openKey{ns, digestBytes(b)}]
}

// S1
func TestFailStopStaleRetirementCannotTruncateAnotherWritersRecord(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	base := body("acknowledged base record")
	mustAppend(t, a, cid, Item{NSPayload, base})
	a.syncFile = func(*os.File) error { return syscall.EIO } // the disk keeps failing under A
	for i := 0; i < 2; i++ {
		if err := a.Append(cid, Item{NSPayload, body("A's failing append %d", i)}); err == nil {
			t.Fatalf("A's append %d succeeded on a failing disk", i)
		}
	}
	a.syncFile = nil

	b := NewAppender(root) // another process over the same directory
	q := body("Q, acknowledged by writer B")
	mustAppend(t, b, cid, Item{NSPayload, q})
	mustRead(t, NewReader(root), cid, NSPayload, q)

	// A wakes up with a healthy disk. It must not replay anything.
	snap := fstopTree(t, root)
	err := a.Append(cid, Item{NSPayload, body("A resumes")})
	if !errors.Is(err, ErrAppenderPoisoned) {
		t.Fatalf("A.Append after its failures = %v, want ErrAppenderPoisoned", err)
	}
	fstopSame(t, "a poisoned writer waking up", snap, fstopTree(t, root))
	fresh := NewReader(root)
	mustRead(t, fresh, cid, NSPayload, q)
	mustRead(t, fresh, cid, NSPayload, base)
	scanCount(t, root, cid)
}

// S2
func TestFailStopNoOffsetReuseBetweenTruncateAndSeal(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})
	first := body("AAAA failed-batch payload")
	second := body("BBBB failed-batch payload")
	if len(first) != len(second) {
		t.Fatalf("test needs equal lengths: %d vs %d", len(first), len(second))
	}
	cached := false
	a.syncFile = func(*os.File) error {
		if !cached {
			cached = true
			mustRead(t, r, cid, NSPayload, first) // the live reader caches the unsynced record's offset
		}
		return syscall.EIO // and the cleanup of the failed call never finishes either
	}
	if err := a.Append(cid, Item{NSPayload, first}); err == nil {
		t.Fatal("the append succeeded on a failing disk")
	}
	a.syncFile = nil

	// The next writer finds whatever the dead one left: no index, maybe a
	// cut tail. It must not append at the offset the reader has cached.
	b := NewAppender(root)
	mustAppend(t, b, cid, Item{NSPayload, second})
	if got, _, err := r.Read(cid, NSPayload, sum(second)); err != nil || !bytes.Equal(got, second) {
		t.Fatalf("the live reader cannot read the record acknowledged after the failure: %v", err)
	}
	got, _, err := r.Read(cid, NSPayload, sum(first))
	switch {
	case err == nil && bytes.Equal(got, first): // complete bytes of an unacknowledged record may be served
	case errors.Is(err, ErrNotFound):
	default:
		t.Fatalf("the unacknowledged record = %v, want its bytes or ErrNotFound (never ErrCorrupt or other bytes)", err)
	}
	recs := fstopRecords(t, root, cid)
	if segs := segsOf(recs, NSPayload, second); len(segs) != 1 || segs[0] == 1 {
		t.Fatalf("the new record is in segments %v, want only a segment the new writer created", segs)
	}
}

// S3
func TestFailStopWriterHandoffRewritesTheFailedRecord(t *testing.T) {
	for _, lose := range []bool{false, true} {
		t.Run(fmt.Sprintf("tail lost=%v", lose), func(t *testing.T) {
			root := t.TempDir()
			disk := newFstopDisk()
			a := NewAppender(root)
			a.syncFile = disk.sync
			base := body("acknowledged base record")
			mustAppend(t, a, cid, Item{NSPayload, base})
			p := body("payload whose fsync failed under writer A")
			disk.set(true, lose)
			if err := a.Append(cid, Item{NSPayload, p}); !errors.Is(err, syscall.EIO) {
				t.Fatalf("A.Append = %v, want EIO", err)
			}
			disk.set(false, false)

			b := NewAppender(root)
			var rewrites, toSeg1 int
			b.writeAt = func(f *os.File, rec []byte, off int64) (int, error) {
				if len(rec) == recHeaderSize+len(p) {
					rewrites++
				}
				if f.Name() == segPath(root, cid, 1) {
					toSeg1++
				}
				return f.WriteAt(rec, off)
			}
			mustAppend(t, b, cid, Item{NSPayload, p})
			if rewrites != 1 {
				t.Fatalf("writer B wrote the failed record %d times, want 1: it must not call readable bytes of A's segment a durable duplicate", rewrites)
			}
			if toSeg1 != 0 {
				t.Fatalf("writer B wrote %d records into A's segment", toSeg1)
			}
			recs := fstopRecords(t, root, cid)
			found := false
			for _, s := range segsOf(recs, NSPayload, p) {
				found = found || s > 1
			}
			if !found {
				t.Fatalf("the record is only in segments %v, want a copy in B's own segment", segsOf(recs, NSPayload, p))
			}
			fresh := NewReader(root)
			mustRead(t, fresh, cid, NSPayload, p)
			mustRead(t, fresh, cid, NSPayload, base)
		})
	}
}

// S4
func TestFailStopPoisonedAppenderNeverTouchesAFileAgain(t *testing.T) {
	root := t.TempDir()
	a := newAppender(root, 1<<30, 1) // one record fills a segment: any later call would rotate
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged in computer a")})
	mustAppend(t, a, "computer-b", Item{NSPayload, body("acknowledged in computer b")})
	a.syncFile = func(*os.File) error { return syscall.EIO }
	first := a.Append(cid, Item{NSPayload, body("the failing append")})
	if first == nil || errors.Is(first, ErrAppenderPoisoned) {
		t.Fatalf("the failing call returned %v, want its own fsync error", first)
	}
	a.syncFile = nil
	snap := fstopTree(t, root)
	builds := 0
	build := func() ([]Item, error) { builds++; return []Item{{NSPayload, body("from build")}}, nil }
	for i := 0; i < 3; i++ {
		for _, id := range []string{cid, "computer-b", "computer-never-seen"} {
			if err := a.Append(id, Item{NSPayload, body("after the failure %d", i)}); !errors.Is(err, ErrAppenderPoisoned) {
				t.Fatalf("Append(%s) = %v, want ErrAppenderPoisoned", id, err)
			}
			if err := a.AppendFunc(id, build); !errors.Is(err, ErrAppenderPoisoned) {
				t.Fatalf("AppendFunc(%s) = %v, want ErrAppenderPoisoned", id, err)
			}
		}
	}
	if builds != 0 {
		t.Fatalf("build ran %d times on a poisoned appender", builds)
	}
	fstopSame(t, "a poisoned appender", snap, fstopTree(t, root))
	if errors.Is(first, ErrAppenderPoisoned) {
		t.Fatal("the failing call must carry its own cause")
	}
}

// S4, failed write
func TestFailStopFailedWriteIsFatalAndLeavesItsBytes(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged before the disk filled")})
	calls := 0
	a.writeAt = func(f *os.File, b []byte, off int64) (int, error) {
		calls++
		if calls == 2 {
			n, _ := f.WriteAt(b[:len(b)/2], off)
			return n, syscall.ENOSPC
		}
		return f.WriteAt(b, off)
	}
	err := a.Append(cid, Item{NSPayload, body("first record of the failed batch, written whole")}, Item{NSReceipt, body("second record of the failed batch, cut short")})
	if !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("Append = %v, want ENOSPC", err)
	}
	a.writeAt = nil
	snap := fstopTree(t, root)
	if err := a.Append(cid, Item{NSPayload, body("after the disk freed")}); !errors.Is(err, ErrAppenderPoisoned) {
		t.Fatalf("Append after a failed write = %v, want ErrAppenderPoisoned", err)
	}
	fstopSame(t, "a writer after a failed write", snap, fstopTree(t, root))

	// The next writer reads past the whole record, cuts the partial one, seals.
	b := NewAppender(root)
	later := body("written by the next writer")
	mustAppend(t, b, cid, Item{NSPayload, later})
	mustRead(t, NewReader(root), cid, NSPayload, later)
	if _, _, err := NewReader(root).Read(cid, NSReceipt, sum(body("second record of the failed batch, cut short"))); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the partial record resolved: %v", err)
	}
	scanCount(t, root, cid)
}

// S5
func TestFailStopAnyFailedFsyncPoisons(t *testing.T) {
	type stage struct {
		name  string
		setup func(a *Appender, root string)
	}
	failDir := func(match func(root, p string) bool) func(a *Appender, root string) {
		return func(a *Appender, root string) {
			a.syncDir = func(p string) error {
				if match(root, p) {
					return syscall.EIO
				}
				return realSyncDir(p)
			}
		}
	}
	for _, st := range []stage{
		{"directory entry of a new segment: computer directory", failDir(func(root, p string) bool { return p == segDir(root, cid) })},
		{"directory entry of a new segment: tape-seg", failDir(func(root, p string) bool { return p == filepath.Join(root, SegDirName) })},
		{"directory entry of a new segment: root", failDir(func(root, p string) bool { return p == filepath.Clean(root) })},
	} {
		t.Run(st.name, func(t *testing.T) {
			root := t.TempDir()
			a := NewAppender(root)
			st.setup(a, root)
			if err := a.Append(cid, Item{NSPayload, body("first")}); !errors.Is(err, syscall.EIO) {
				t.Fatalf("Append = %v, want the directory fsync error", err)
			}
			a.syncDir = nil
			snap := fstopTree(t, root)
			if err := a.Append(cid, Item{NSPayload, body("second")}); !errors.Is(err, ErrAppenderPoisoned) {
				t.Fatalf("Append after the failure = %v, want ErrAppenderPoisoned", err)
			}
			fstopSame(t, "a poisoned writer", snap, fstopTree(t, root))
		})
	}
	t.Run("directory fsync of a seal", func(t *testing.T) {
		root := t.TempDir()
		a := newAppender(root, 1<<30, 1)
		mustAppend(t, a, cid, Item{NSPayload, body("fills the segment")})
		a.syncDir = func(p string) error {
			if _, err := os.Stat(idxPath(root, cid, 1)); err == nil && p == segDir(root, cid) {
				return syscall.EIO
			}
			return realSyncDir(p)
		}
		if err := a.Append(cid, Item{NSPayload, body("triggers the seal")}); !errors.Is(err, syscall.EIO) {
			t.Fatalf("Append = %v, want the seal's directory fsync error", err)
		}
		a.syncDir = nil
		snap := fstopTree(t, root)
		if err := a.Append(cid, Item{NSPayload, body("after the failed seal")}); !errors.Is(err, ErrAppenderPoisoned) {
			t.Fatalf("Append after the failed seal = %v, want ErrAppenderPoisoned", err)
		}
		fstopSame(t, "a poisoned writer", snap, fstopTree(t, root))
	})
	t.Run("fsync of recovery", func(t *testing.T) {
		root := t.TempDir()
		base := body("acknowledged base record")
		mustAppend(t, NewAppender(root), cid, Item{NSPayload, base}) // a previous process, gone
		b := NewAppender(root)
		b.syncFile = func(*os.File) error { return syscall.EIO }
		if err := b.Append(cid, Item{NSPayload, body("x")}); !errors.Is(err, syscall.EIO) {
			t.Fatalf("B.Append = %v, want EIO from recovering the old segment", err)
		}
		b.syncFile = nil
		snap := fstopTree(t, root)
		if err := b.Append(cid, Item{NSPayload, body("y")}); !errors.Is(err, ErrAppenderPoisoned) {
			t.Fatalf("B.Append after its recovery failed = %v, want ErrAppenderPoisoned", err)
		}
		fstopSame(t, "a poisoned writer", snap, fstopTree(t, root))
		c := NewAppender(root) // the next process recovers fine
		later := body("written by the next process")
		mustAppend(t, c, cid, Item{NSPayload, later})
		fresh := NewReader(root)
		mustRead(t, fresh, cid, NSPayload, later)
		mustRead(t, fresh, cid, NSPayload, base)
		scanCount(t, root, cid)
	})
}

// S6
func TestFailStopRecoverySealsAnUnownedSegmentAndStartsItsOwn(t *testing.T) {
	unacked := body("a record whose fsync and CAS never happened, long enough to cut in two")
	whole := encodeRecord(NSPayload, digestBytes(unacked), unacked)
	cases := map[string][]byte{
		"clean (the previous writer simply stopped)": nil,
		"partial record header":                      whole[:recHeaderSize-9],
		"header but no body":                         whole[:recHeaderSize],
		"half a body":                                whole[:recHeaderSize+len(unacked)/2],
		"whole record minus one byte":                whole[:len(whole)-1],
	}
	for name, tail := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			first, second := body("acknowledged one"), body("acknowledged two")
			mustAppend(t, NewAppender(root), cid, Item{NSEvent, first}, Item{NSReceipt, second})
			good := fstopSize(t, segPath(root, cid, 1))
			if tail != nil {
				appendTail(t, segPath(root, cid, 1), tail)
			}

			b := NewAppender(root)
			var toOld int
			b.writeAt = func(f *os.File, rec []byte, off int64) (int, error) {
				if f.Name() == segPath(root, cid, 1) {
					toOld++
				}
				return f.WriteAt(rec, off)
			}
			later := body("written after the crash")
			mustAppend(t, b, cid, Item{NSPayload, later})
			if toOld != 0 {
				t.Fatalf("the new writer appended %d records to a segment it did not write", toOld)
			}
			if _, err := os.Stat(idxPath(root, cid, 1)); err != nil {
				t.Fatalf("the unowned segment was not sealed: %v", err)
			}
			if got := fstopSize(t, segPath(root, cid, 1)); got != good {
				t.Fatalf("sealed segment is %d bytes, want %d (whole records only)", got, good)
			}
			if tail != nil && b.Stats().TornTails != 1 {
				t.Fatalf("TornTails = %d, want 1", b.Stats().TornTails)
			}
			recs := fstopRecords(t, root, cid)
			if len(recs) != 3 || segsOf(recs, NSEvent, first)[0] != 1 || segsOf(recs, NSReceipt, second)[0] != 1 || segsOf(recs, NSPayload, later)[0] != 2 {
				t.Fatalf("records by segment = %v, want the two old ones in 1 and the new one in 2", recs)
			}
			r := NewReader(root)
			if _, _, err := r.Read(cid, NSPayload, sum(unacked)); !errors.Is(err, ErrNotFound) {
				t.Fatalf("the torn record resolved: %v", err)
			}
			// A sealed segment is never modified again, by this writer or the next.
			sealed := map[string]string{}
			for k, v := range fstopTree(t, segDir(root, cid)) {
				if k == SegmentFileName(1) || k == IndexFileName(1) {
					sealed[k] = v
				}
			}
			mustAppend(t, b, cid, Item{NSPayload, body("more from b")})
			mustAppend(t, NewAppender(root), cid, Item{NSPayload, body("from a third writer")})
			after := fstopTree(t, segDir(root, cid))
			for k, v := range sealed {
				if after[k] != v {
					t.Fatalf("sealed file %s was modified", k)
				}
			}
			if len(sealed) != 2 {
				t.Fatalf("test bug: found sealed files %v", sealed)
			}
		})
	}
	t.Run("a segment whose header never landed", func(t *testing.T) {
		for _, n := range []int{0, 5, segHeaderSize - 1} {
			root := t.TempDir()
			mustAppend(t, NewAppender(root), cid, Item{NSPayload, body("lost with its torn header")})
			if err := os.Truncate(segPath(root, cid, 1), int64(n)); err != nil {
				t.Fatal(err)
			}
			b := NewAppender(root)
			fresh := body("fresh")
			mustAppend(t, b, cid, Item{NSPayload, fresh})
			if _, err := os.Stat(idxPath(root, cid, 1)); err != nil {
				t.Fatalf("n=%d: torn segment not sealed: %v", n, err)
			}
			recs := fstopRecords(t, root, cid)
			if len(recs) != 1 || segsOf(recs, NSPayload, fresh)[0] != 2 {
				t.Fatalf("n=%d: records by segment = %v", n, recs)
			}
			mustRead(t, NewReader(root), cid, NSPayload, fresh)
		}
	})
}

// S7
func TestFailStopRepeatedFailuresGrowByAtMostOneSegmentPerWriter(t *testing.T) {
	root := t.TempDir()
	var acked [][]byte
	const writers = 8
	for w := 1; w <= writers; w++ {
		disk := newFstopDisk()
		a := NewAppender(root)
		a.syncFile = disk.sync
		prime := body("writer %d primes its segment", w)
		mustAppend(t, a, cid, Item{NSPayload, prime})
		acked = append(acked, prime)
		disk.set(true, w%2 == 0)
		if err := a.Append(cid, Item{NSPayload, body("writer %d fails", w)}); err == nil {
			t.Fatalf("writer %d: append succeeded on a failing disk", w)
		}
		files := len(segNames(t, root, cid))
		for i := 0; i < 10; i++ { // the disk keeps failing, then heals: nothing may be created
			if i == 5 {
				disk.set(false, false)
			}
			_ = a.Append(cid, Item{NSPayload, body("writer %d retry %d", w, i)})
			_ = a.AppendFunc(cid, func() ([]Item, error) { return []Item{{NSPayload, body("writer %d build %d", w, i)}}, nil })
		}
		if got := len(segNames(t, root, cid)); got != files {
			t.Fatalf("writer %d: a poisoned appender created files (%d -> %d)", w, files, got)
		}
		// seg-1; then per later writer: its predecessor's index and its own segment.
		if want := 2*w - 1; files != want {
			t.Fatalf("after %d writers there are %d files, want %d (one sealed index and one segment per writer)", w, files, want)
		}
	}
	fresh := NewReader(root)
	for _, p := range acked {
		mustRead(t, fresh, cid, NSPayload, p)
	}
	scanCount(t, root, cid)
}

// S8
func TestFailStopAcknowledgedRecordsAreAlwaysReadable(t *testing.T) {
	for seed := int64(1); seed <= 25; seed++ {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			root := t.TempDir()
			live := NewReader(root)
			disks := [2]*fstopDisk{newFstopDisk(), newFstopDisk()}
			mk := func(i int) *Appender {
				disks[i] = newFstopDisk()
				a := newAppender(root, int64(300+rng.Intn(500)), 2+rng.Intn(5))
				a.syncFile = disks[i].sync
				return a
			}
			apps := [2]*Appender{mk(0), mk(1)}
			var dead []*Appender
			var acked [][]byte
			for step := 0; step < 60; step++ {
				i := rng.Intn(2)
				a := apps[i]
				p := body("seed %d step %d %s", seed, step, strings.Repeat("x", rng.Intn(90)))
				mode := 0 // healthy
				if rng.Intn(4) == 0 {
					mode = 1 + rng.Intn(3) // 1 fsync fails, tail kept; 2 fsync fails, tail lost; 3 short write
				}
				switch mode {
				case 1:
					disks[i].set(true, false)
				case 2:
					disks[i].set(true, true)
				case 3:
					a.writeAt = func(f *os.File, b []byte, off int64) (int, error) {
						n, _ := f.WriteAt(b[:len(b)/2], off)
						return n, syscall.ENOSPC
					}
				}
				err := a.Append(cid, Item{NSPayload, p})
				disks[i].set(false, false)
				a.writeAt = nil
				if mode == 0 && err != nil {
					t.Fatalf("step %d: healthy Append = %v", step, err)
				}
				if err == nil {
					acked = append(acked, p)
				} else {
					dead = append(dead, a)
					apps[i] = mk(i) // the process restarts
				}
				if len(dead) > 0 && rng.Intn(3) == 0 { // a poisoned writer wakes up
					d := dead[rng.Intn(len(dead))]
					snap := fstopTree(t, root)
					if err := d.Append(cid, Item{NSPayload, body("zombie %d", step)}); !errors.Is(err, ErrAppenderPoisoned) {
						t.Fatalf("step %d: zombie Append = %v, want ErrAppenderPoisoned", step, err)
					}
					fstopSame(t, "a zombie writer", snap, fstopTree(t, root))
				}
				fresh := NewReader(root)
				for _, q := range acked {
					mustRead(t, live, cid, NSPayload, q)
					mustRead(t, fresh, cid, NSPayload, q)
				}
			}
			scanCount(t, root, cid)
		})
	}
}

// S9
func TestFailStopReaderNeverServesAWrongRecord(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})
	frontier := fstopSize(t, segPath(root, cid, 1))
	first := body("AAAA a record no one acknowledged")
	second := append([]byte("BBBB"), bytes.Repeat([]byte("b"), len(first)-4)...) // same length as first
	disk := newFstopDisk()
	cachedOnce := false
	disk.onFail = func(*os.File) {
		if !cachedOnce {
			cachedOnce = true
			mustRead(t, r, cid, NSPayload, first) // the live reader caches the unsynced record
		}
	}
	a.syncFile = disk.sync
	disk.set(true, true) // fsync fails and the tail is lost
	_ = a.Append(cid, Item{NSPayload, first})
	disk.set(false, false)
	if got := fstopSize(t, segPath(root, cid, 1)); got != frontier {
		t.Fatalf("test model: segment is %d bytes, want the lost tail cut to %d", got, frontier)
	}
	mustAppend(t, NewAppender(root), cid, Item{NSPayload, second})
	for _, c := range []struct {
		name string
		b    []byte
	}{{"never acknowledged", first}, {"acknowledged", second}} {
		got, _, err := r.Read(cid, NSPayload, sum(c.b))
		if err == nil && !bytes.Equal(got, c.b) {
			t.Fatalf("%s: the reader served other bytes for the digest", c.name)
		}
		if c.name == "acknowledged" && err != nil {
			t.Fatalf("acknowledged record: %v", err)
		}
		if c.name == "never acknowledged" && err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatalf("never acknowledged record = %v, want ErrNotFound", err)
		}
	}
}
