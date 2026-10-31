package tapestore

// Failure modes of sealed-segment compaction (compact.go), written down
// before the code:
//
//  C1 The copy is byte for byte: the segment's bytes, its index, every index
//     offset and every record read the same after; mode and mtime are kept.
//  C2 Crash mid-copy: the segment is untouched, the partial temp file is
//     ignored by the reader, the view and a new appender, and the next
//     compaction of the segment removes it once stale and succeeds.
//  C3 A copy that does not match the original refuses the rename
//     (ErrCompactMismatch): the segment keeps its inode and bytes, and no
//     temp file is left.
//  C4 A reader reading during the copy never errors, and a reader holding
//     the old file open across the rename reads the old bytes.
//  C5 An appender writing the computer's open segment and another computer
//     during a compaction is not blocked, and every record stays readable.
//  C6 Seal while a reader reads: the seal-time worker compacts every sealed
//     segment while readers read and appends rotate; nothing fails.
//  C7 An unsealed segment is never touched.
//  C8 A segment removed (a purge) or replaced (another compactor) during the
//     copy is not resurrected or overwritten.
//  C9 Rollback by revert: a compacted store with a stray temp file reads,
//     views and appends with code that knows nothing of compaction (the
//     reader, the view and the appender list only canonical names).
//  C10 Cancel mid-copy leaves the segment unchanged and no temp file.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// buildSealed appends n records to computer id with a seal every perSeg
// records and returns the bodies. The last segment is left open.
func buildSealed(t *testing.T, root, id string, n, perSeg int) [][]byte {
	t.Helper()
	a := newTestAppender(root, 1<<30, perSeg)
	var bodies [][]byte
	for i := 0; i < n; i++ {
		b := body(`{"kind":"projection_batch_recorded","seq":%d,"pad":"%s"}`, i, strings.Repeat("x", 300+i%50))
		mustAppend(t, a, id, Item{NSEvent, b})
		bodies = append(bodies, b)
	}
	return bodies
}

func readAll(t *testing.T, r *Reader, id string, bodies [][]byte) {
	t.Helper()
	for i, b := range bodies {
		got, src, err := r.Read(id, NSEvent, hexDigest(digestBytes(b)))
		if err != nil || src != SourceSegment || !bytes.Equal(got, b) {
			t.Fatalf("record %d: src %v err %v", i, src, err)
		}
	}
}

func compactTemps(t *testing.T, root, id string) []string {
	t.Helper()
	var out []string
	for _, n := range segNames(t, root, id) {
		if strings.Contains(n, compactTempInfix) {
			out = append(out, n)
		}
	}
	return out
}

var always = CompactOptions{MinAvgExtentBytes: -1, VerifyRecords: true}

func TestCompactIsByteForByte(t *testing.T) { // C1
	root := t.TempDir()
	bodies := buildSealed(t, root, cid, 30, 10)
	for segno := uint64(1); segno <= 2; segno++ {
		sp, ip := segPath(root, cid, segno), idxPath(root, cid, segno)
		beforeSeg, _ := os.ReadFile(sp)
		beforeIdx, _ := os.ReadFile(ip)
		old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
		if err := os.Chtimes(sp, old, old); err != nil {
			t.Fatal(err)
		}
		fiBefore, _ := os.Stat(sp)
		res, err := CompactSealed(root, cid, segno, always)
		if err != nil || res.Action != CompactActionCompacted {
			t.Fatalf("segment %d: %+v %v", segno, res, err)
		}
		if res.RecordsVerified != 10 || res.SHA256 == "" {
			t.Fatalf("segment %d: %+v", segno, res)
		}
		afterSeg, _ := os.ReadFile(sp)
		afterIdx, _ := os.ReadFile(ip)
		if !bytes.Equal(beforeSeg, afterSeg) || !bytes.Equal(beforeIdx, afterIdx) {
			t.Fatalf("segment %d: bytes changed", segno)
		}
		fiAfter, _ := os.Stat(sp)
		if os.SameFile(fiBefore, fiAfter) {
			t.Fatalf("segment %d: not rewritten (same inode)", segno)
		}
		if !fiAfter.ModTime().Equal(fiBefore.ModTime()) || fiAfter.Mode() != fiBefore.Mode() {
			t.Fatalf("segment %d: mtime/mode %v %v, want %v %v", segno, fiAfter.ModTime(), fiAfter.Mode(), fiBefore.ModTime(), fiBefore.Mode())
		}
	}
	readAll(t, NewReader(root), cid, bodies)
	if tmps := compactTemps(t, root, cid); len(tmps) != 0 {
		t.Fatalf("temp files left: %v", tmps)
	}
}

func TestCompactCrashMidCopyLeavesTheSegmentAndIsSweptLater(t *testing.T) { // C2
	root := t.TempDir()
	bodies := buildSealed(t, root, cid, 20, 10)
	before, _ := os.ReadFile(segPath(root, cid, 1))
	crash := errors.New("simulated crash")
	opts := always
	opts.testAfterCopy = func(*os.File) error { return crash }
	if _, err := CompactSealed(root, cid, 1, opts); !errors.Is(err, crash) {
		t.Fatalf("err %v", err)
	}
	// A real crash leaves the temp file behind (the error path removes it):
	// plant a torn one, as a kill -9 mid-copy would.
	torn := filepath.Join(segDir(root, cid), SegmentFileName(1)+compactTempInfix+"123")
	if err := os.WriteFile(torn, before[:len(before)/2], 0o640); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(segPath(root, cid, 1)); !bytes.Equal(before, after) {
		t.Fatal("segment changed")
	}
	readAll(t, NewReader(root), cid, bodies)
	if v, err := NewReader(root).OpenView(cid); err != nil || v.Segments() != 2 {
		t.Fatalf("view: %v", err)
	}
	mustAppend(t, NewAppender(root), cid, Item{NSEvent, body("after crash")})

	// Young temp files are left alone (a live compactor's); stale ones go.
	if _, err := CompactSealed(root, cid, 1, always); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(torn); err != nil {
		t.Fatalf("young temp removed: %v", err)
	}
	old := time.Now().Add(-2 * time.Hour)
	_ = os.Chtimes(torn, old, old)
	if _, err := CompactSealed(root, cid, 1, always); err != nil {
		t.Fatal(err)
	}
	if tmps := compactTemps(t, root, cid); len(tmps) != 0 {
		t.Fatalf("stale temp not swept: %v", tmps)
	}
	readAll(t, NewReader(root), cid, bodies)
}

func TestCompactMismatchRefusesTheRename(t *testing.T) { // C3
	for _, verify := range []bool{false, true} {
		t.Run(fmt.Sprintf("verifyRecords=%v", verify), func(t *testing.T) {
			root := t.TempDir()
			buildSealed(t, root, cid, 20, 10)
			sp := segPath(root, cid, 1)
			before, _ := os.ReadFile(sp)
			fiBefore, _ := os.Stat(sp)
			opts := CompactOptions{MinAvgExtentBytes: -1, VerifyRecords: verify}
			opts.testAfterCopy = func(f *os.File) error {
				_, err := f.WriteAt([]byte{0xff}, int64(len(before)/2))
				return err
			}
			if _, err := CompactSealed(root, cid, 1, opts); !errors.Is(err, ErrCompactMismatch) {
				t.Fatalf("err %v, want ErrCompactMismatch", err)
			}
			fiAfter, _ := os.Stat(sp)
			after, _ := os.ReadFile(sp)
			if !os.SameFile(fiBefore, fiAfter) || !bytes.Equal(before, after) {
				t.Fatal("segment replaced after a mismatch")
			}
			if tmps := compactTemps(t, root, cid); len(tmps) != 0 {
				t.Fatalf("temp left: %v", tmps)
			}
		})
	}
	// A truncated copy is refused too.
	root := t.TempDir()
	buildSealed(t, root, cid, 20, 10)
	opts := always
	opts.testAfterCopy = func(f *os.File) error { return f.Truncate(100) }
	if _, err := CompactSealed(root, cid, 1, opts); !errors.Is(err, ErrCompactMismatch) {
		t.Fatalf("truncated copy: %v", err)
	}
}

func TestCompactConcurrentReaders(t *testing.T) { // C4
	root := t.TempDir()
	bodies := buildSealed(t, root, cid, 60, 10)
	// An fd held on the old file across the rename keeps the old bytes.
	held, err := os.Open(segPath(root, cid, 1))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	want, _ := os.ReadFile(segPath(root, cid, 1))

	var stop atomic.Bool
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := NewReader(root) // one reader per goroutine, and a shared one below
			for !stop.Load() {
				for _, b := range bodies {
					got, _, err := r.Read(cid, NSEvent, hexDigest(digestBytes(b)))
					if err != nil || !bytes.Equal(got, b) {
						errs <- fmt.Errorf("read during compaction: %v", err)
						return
					}
				}
			}
		}()
	}
	for round := 0; round < 5; round++ {
		for segno := uint64(1); segno <= 5; segno++ {
			if _, err := CompactSealed(root, cid, segno, always); err != nil {
				t.Fatal(err)
			}
		}
	}
	stop.Store(true)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	got := make([]byte, len(want))
	if _, err := held.ReadAt(got, 0); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("held fd: %v", err)
	}
}

func TestCompactDoesNotBlockAppends(t *testing.T) { // C5
	root := t.TempDir()
	bodies := buildSealed(t, root, cid, 20, 10)
	a := NewAppender(root)
	inCopy, release := make(chan struct{}), make(chan struct{})
	opts := always
	opts.testBeforeRename = func(string) error {
		close(inCopy)
		<-release // hang-guard: released below after the bounded appends
		return nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := CompactSealed(root, cid, 1, opts)
		done <- err
	}()
	select {
	case <-inCopy:
	case <-time.After(10 * time.Second):
		t.Fatal("compaction never reached the rename")
	}
	appended := make(chan error, 1)
	go func() {
		err := a.Append(cid, Item{NSEvent, body("during compaction, same computer")})
		if err == nil {
			err = a.Append("computer-b", Item{NSEvent, body("during compaction, other computer")})
		}
		appended <- err
	}()
	select {
	case err := <-appended:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("append blocked by a compaction")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("compaction did not finish")
	}
	r := NewReader(root)
	readAll(t, r, cid, append(bodies, body("during compaction, same computer")))
	if _, _, err := r.Read("computer-b", NSEvent, hexDigest(digestBytes(body("during compaction, other computer")))); err != nil {
		t.Fatal(err)
	}
}

func TestSealTimeCompactionWhileReadersRead(t *testing.T) { // C6
	root := t.TempDir()
	a := newTestAppender(root, 1<<30, 7)
	a.StartSealCompaction(SealCompaction{Delay: time.Millisecond, BytesPerSecond: 1 << 30, Logf: t.Logf})
	defer a.StopSealCompaction()
	var mu sync.Mutex
	var written [][]byte
	var stop atomic.Bool
	readErr := make(chan error, 1)
	go func() {
		r := NewReader(root)
		for !stop.Load() {
			mu.Lock()
			snap := append([][]byte(nil), written...)
			mu.Unlock()
			for _, b := range snap {
				if got, _, err := r.Read(cid, NSEvent, hexDigest(digestBytes(b))); err != nil || !bytes.Equal(got, b) {
					readErr <- fmt.Errorf("read: %v", err)
					return
				}
			}
		}
		readErr <- nil
	}()
	for i := 0; i < 70; i++ {
		b := body("seal-time %d %s", i, strings.Repeat("y", 200))
		mustAppend(t, a, cid, Item{NSEvent, b})
		mu.Lock()
		written = append(written, b)
		mu.Unlock()
	}
	deadline := time.Now().Add(20 * time.Second)
	for a.Stats().Compactions+a.Stats().CompactionSkips < a.Stats().Seals && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	stop.Store(true)
	select {
	case err := <-readErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("reader did not stop")
	}
	st := a.Stats()
	if st.Seals < 9 || st.Compactions+st.CompactionSkips != st.Seals || st.CompactionErrors != 0 || st.CompactionDropped != 0 {
		t.Fatalf("stats %+v", st)
	}
	readAll(t, NewReader(root), cid, written)
	if tmps := compactTemps(t, root, cid); len(tmps) != 0 {
		t.Fatalf("temps left: %v", tmps)
	}
}

func TestCompactNeverTouchesAnUnsealedSegment(t *testing.T) { // C7
	root := t.TempDir()
	buildSealed(t, root, cid, 15, 10) // segment 2 is open
	sp := segPath(root, cid, 2)
	fiBefore, _ := os.Stat(sp)
	res, err := CompactSealed(root, cid, 2, always)
	if err != nil || res.Action != CompactActionSkipped || res.Skip != "unsealed" {
		t.Fatalf("%+v %v", res, err)
	}
	fiAfter, _ := os.Stat(sp)
	if !os.SameFile(fiBefore, fiAfter) || fiAfter.Size() != fiBefore.Size() {
		t.Fatal("unsealed segment touched")
	}
}

func TestCompactDoesNotResurrectOrOverwrite(t *testing.T) { // C8
	t.Run("removed", func(t *testing.T) {
		root := t.TempDir()
		buildSealed(t, root, cid, 20, 10)
		opts := always
		opts.testBeforeRename = func(string) error {
			if err := os.Remove(idxPath(root, cid, 1)); err != nil {
				return err
			}
			return os.Remove(segPath(root, cid, 1))
		}
		res, err := CompactSealed(root, cid, 1, opts)
		if err != nil || res.Action != CompactActionSkipped {
			t.Fatalf("%+v %v", res, err)
		}
		if _, err := os.Stat(segPath(root, cid, 1)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed segment resurrected: %v", err)
		}
		if tmps := compactTemps(t, root, cid); len(tmps) != 0 {
			t.Fatalf("temps left: %v", tmps)
		}
	})
	t.Run("replaced by another compactor", func(t *testing.T) {
		root := t.TempDir()
		bodies := buildSealed(t, root, cid, 20, 10)
		var other os.FileInfo
		opts := always
		opts.testBeforeRename = func(string) error {
			if _, err := CompactSealed(root, cid, 1, always); err != nil {
				return err
			}
			other, _ = os.Stat(segPath(root, cid, 1))
			return nil
		}
		res, err := CompactSealed(root, cid, 1, opts)
		if err != nil || res.Action != CompactActionSkipped {
			t.Fatalf("%+v %v", res, err)
		}
		cur, _ := os.Stat(segPath(root, cid, 1))
		if !os.SameFile(cur, other) {
			t.Fatal("the other compactor's copy was overwritten")
		}
		readAll(t, NewReader(root), cid, bodies)
	})
}

func TestCompactedStoreReadsWithoutCompactionCode(t *testing.T) { // C9
	root := t.TempDir()
	bodies := buildSealed(t, root, cid, 25, 10)
	for segno := uint64(1); segno <= 2; segno++ {
		if _, err := CompactSealed(root, cid, segno, always); err != nil {
			t.Fatal(err)
		}
	}
	stray := filepath.Join(segDir(root, cid), SegmentFileName(2)+compactTempInfix+"999")
	if err := os.WriteFile(stray, []byte("partial"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := parseSegName(filepath.Base(stray)); ok {
		t.Fatal("a compaction temp parses as a segment name")
	}
	r := NewReader(root)
	readAll(t, r, cid, bodies)
	v, err := r.OpenView(cid)
	if err != nil || v.Segments() != 3 {
		t.Fatalf("view %v", err)
	}
	if n := scanCount(t, root, cid); n != 25 {
		t.Fatalf("scan %d", n)
	}
	// A fresh appender (the restart path) seals the open segment and goes on.
	mustAppend(t, NewAppender(root), cid, Item{NSEvent, body("after rollback")})
	if _, _, err := NewReader(root).Read(cid, NSEvent, hexDigest(digestBytes(body("after rollback")))); err != nil {
		t.Fatal(err)
	}
}

func TestCompactCancelLeavesNothing(t *testing.T) { // C10
	root := t.TempDir()
	buildSealed(t, root, cid, 20, 10)
	sp := segPath(root, cid, 1)
	fiBefore, _ := os.Stat(sp)
	cancel := make(chan struct{})
	close(cancel)
	opts := always
	opts.Cancel = cancel
	if _, err := CompactSealed(root, cid, 1, opts); !errors.Is(err, ErrCompactCanceled) {
		t.Fatalf("err %v", err)
	}
	fiAfter, _ := os.Stat(sp)
	if !os.SameFile(fiBefore, fiAfter) {
		t.Fatal("segment replaced")
	}
	if tmps := compactTemps(t, root, cid); len(tmps) != 0 {
		t.Fatalf("temps left: %v", tmps)
	}
}

func TestCompactCountsExtentsOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("FIEMAP is Linux only")
	}
	root := t.TempDir()
	buildSealed(t, root, cid, 20, 10)
	res, err := CompactSealed(root, cid, 1, CompactOptions{MinAvgExtentBytes: -1, Compress: true})
	if err != nil || res.Action != CompactActionCompacted {
		t.Fatalf("%+v %v", res, err)
	}
	if res.ExtentsBefore == -1 && res.ExtentsAfter == -1 {
		// -1 is "FIEMAP unsupported": the CI runner's temp filesystem has no
		// extent map, which btrfs and ext4 on Node B and the agents VM have.
		t.Skip("FIEMAP unsupported on this filesystem")
	}
	if res.ExtentsBefore < 1 || res.ExtentsAfter < 1 || res.StBlocksBefore <= 0 || res.StBlocksAfter <= 0 {
		t.Fatalf("measurements missing: %+v", res)
	}
	// The default threshold now sees a compact file (one or a few extents).
	res, err = CompactSealed(root, cid, 1, CompactOptions{})
	if err != nil || res.Action != CompactActionSkipped || res.Skip != "already compact" {
		t.Fatalf("second pass %+v %v", res, err)
	}
	// A low-priority thread runs and returns.
	ran := false
	RunLowPriority(func() { ran = true })
	if !ran {
		t.Fatal("RunLowPriority did not run")
	}
}

func TestCompactDryRunAndPacing(t *testing.T) {
	root := t.TempDir()
	buildSealed(t, root, cid, 20, 10)
	sp := segPath(root, cid, 1)
	fiBefore, _ := os.Stat(sp)
	res, err := CompactSealed(root, cid, 1, CompactOptions{MinAvgExtentBytes: -1, DryRun: true})
	if err != nil || res.Action != CompactActionWouldCompact {
		t.Fatalf("%+v %v", res, err)
	}
	if fiAfter, _ := os.Stat(sp); !os.SameFile(fiBefore, fiAfter) {
		t.Fatal("dry run wrote")
	}
	// Pacing: about size/rate seconds.
	size := fiBefore.Size()
	start := time.Now()
	if _, err := CompactSealed(root, cid, 1, CompactOptions{MinAvgExtentBytes: -1, BytesPerSecond: size * 5, BufferBytes: 512}); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el < 150*time.Millisecond {
		t.Fatalf("pacing not applied: %v for %d bytes at %d B/s", el, size, size*5)
	}
}
