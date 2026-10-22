package tracelog

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAppendAndReadBackLiveAndStatic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "trace-log")
	l, _ := openT(t, dir, nil)
	for i := 1; i <= 10; i++ {
		mustAppend(t, l, i)
	}
	got, err := l.Reader().ByRun("owner-1", "run-a", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("live ByRun = %d records, want 10", len(got))
	}
	for i, r := range got {
		if r.Meta.Seq != int64(i+1) || string(r.Line) != string(lineFor(i+1)) || r.Meta.EventID != fmt.Sprintf("ev-%06d", i+1) || r.Meta.BootID != "boot-1" || r.Meta.AfterTapeSeq != int64((i+1)/2) {
			t.Fatalf("record %d = %+v %q", i, r.Meta, r.Line)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	streams, reasons := scanDir(t, dir)
	if !eqInts(streams, seq(1, 10)) || len(reasons) != 1 || reasons[0] != "open" {
		t.Fatalf("after Close: streams %v reasons %v", streams, reasons)
	}
	// Close sealed the segment: an index exists and the file set is exactly one pair.
	names := fileNames(t, dir)
	if len(names) != 2 || names[0] != tidx(1) && names[1] != tidx(1) {
		t.Fatalf("files after Close = %v", names)
	}
}

func TestReaderQueries(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.SealRecords = 7 })
	// Two owners, two runs, two trajectories, interleaved; stream_seq is global.
	n := 0
	for i := 0; i < 40; i++ {
		n++
		m := meta(n)
		m.RunID, m.OwnerID, m.TrajectoryID = "run-a", "owner-1", "traj-1"
		m.Seq = int64(i/2 + 1)
		if i%4 == 3 {
			m.RunID, m.TrajectoryID = "run-b", "traj-2"
		}
		if i%5 == 4 {
			m.OwnerID = "owner-2"
		}
		if err := l.Append(m, lineFor(n)); err != nil {
			t.Fatal(err)
		}
	}
	r := l.Reader()
	all := func(owner, run string) []Record {
		out, err := r.ByRun(owner, run, 0, 1000)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	runA := all("owner-1", "run-a")
	for i := 1; i < len(runA); i++ {
		if runA[i].Meta.Seq < runA[i-1].Meta.Seq {
			t.Fatalf("ByRun not ascending: %d after %d", runA[i].Meta.Seq, runA[i-1].Meta.Seq)
		}
	}
	// afterSeq and limit.
	after5, _ := r.ByRun("owner-1", "run-a", 5, 3)
	if len(after5) != 3 || after5[0].Meta.Seq <= 5 {
		t.Fatalf("ByRun after 5 limit 3 = %d records, first seq %d", len(after5), after5[0].Meta.Seq)
	}
	if got := all("owner-2", "run-a"); len(got) == 0 {
		t.Fatal("owner-2 run-a: expected records")
	}
	if got := all("owner-3", "run-a"); len(got) != 0 {
		t.Fatalf("owner-3 has %d records", len(got))
	}
	// ByOwnerAfter is ascending in stream_seq and respects the cursor.
	own, err := r.ByOwnerAfter("owner-1", 10, 1000)
	if err != nil {
		t.Fatal(err)
	}
	prev := int64(10)
	for _, rec := range own {
		if rec.Meta.OwnerID != "owner-1" || rec.Meta.StreamSeq <= prev {
			t.Fatalf("ByOwnerAfter: owner %s stream %d after %d", rec.Meta.OwnerID, rec.Meta.StreamSeq, prev)
		}
		prev = rec.Meta.StreamSeq
	}
	lim, _ := r.ByOwnerAfter("owner-1", 0, 4)
	if len(lim) != 4 || lim[0].Meta.StreamSeq != 1 {
		t.Fatalf("ByOwnerAfter limit 4 = %d records starting at %d", len(lim), lim[0].Meta.StreamSeq)
	}
	// Newest is newest-first and bounded.
	newest, _ := r.ByOwnerNewest("owner-1", 5)
	if len(newest) != 5 {
		t.Fatalf("ByOwnerNewest = %d", len(newest))
	}
	for i := 1; i < len(newest); i++ {
		if newest[i].Meta.StreamSeq >= newest[i-1].Meta.StreamSeq {
			t.Fatal("ByOwnerNewest is not newest-first")
		}
	}
	if newest[0].Meta.StreamSeq != own[len(own)-1].Meta.StreamSeq {
		t.Fatalf("newest owner-1 line %d, want %d", newest[0].Meta.StreamSeq, own[len(own)-1].Meta.StreamSeq)
	}
	// Trajectory.
	tr, err := r.ByTrajectory("owner-1", "traj-2", 1000)
	if err != nil || len(tr) == 0 {
		t.Fatalf("ByTrajectory: %v, %d records", err, len(tr))
	}
	for _, rec := range tr {
		if rec.Meta.TrajectoryID != "traj-2" || rec.Meta.OwnerID != "owner-1" {
			t.Fatalf("ByTrajectory returned %+v", rec.Meta)
		}
	}
	// High-water marks across sealed and open segments.
	if hw, _ := r.MaxStreamSeq(); hw != 40 {
		t.Fatalf("MaxStreamSeq = %d, want 40", hw)
	}
	if hw, _ := r.MaxRunSeq("owner-1", "run-a"); hw != runA[len(runA)-1].Meta.Seq {
		t.Fatalf("MaxRunSeq = %d, want %d", hw, runA[len(runA)-1].Meta.Seq)
	}
	// A static reader over the same live directory agrees with the live one.
	static, err := OpenReader(dir).ByOwnerAfter("owner-1", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	liveAll, _ := r.ByOwnerAfter("owner-1", 0, 1000)
	if len(static) != len(liveAll) {
		t.Fatalf("static reader %d records, live %d", len(static), len(liveAll))
	}
}

func TestIndexFallbackWhenTidxIsCorruptOrMissing(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.SealRecords = 5 })
	for i := 1; i <= 17; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	want, _ := OpenReader(dir).ByRun("owner-1", "run-a", 0, 1000)
	// Flip a byte in one index, delete another.
	p := filepath.Join(dir, tidx(1))
	raw := mustRead(t, p)
	raw[len(raw)/2] ^= 0xff
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, tidx(2))); err != nil {
		t.Fatal(err)
	}
	r := OpenReader(dir)
	got, err := r.ByRun("owner-1", "run-a", 0, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("with a bad index: %d records, want %d", len(got), len(want))
	}
	if r.IndexRebuilds() < 2 {
		t.Fatalf("IndexRebuilds = %d, want at least 2", r.IndexRebuilds())
	}
}

func TestSyncTimerAndExplicitSync(t *testing.T) {
	d := newDisk()
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.Hooks = d.hooks(); o.SyncInterval = 10 * time.Millisecond })
	mustAppend(t, l, 1)
	deadline := time.Now().Add(2 * time.Second)
	for l.Stats().Dirty && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if st := l.Stats(); st.Dirty || st.Syncs == 0 {
		t.Fatalf("the timer never synced: %+v", st)
	}
	// Explicit Sync on a clean log does not fsync again.
	before := l.Stats().Syncs
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := l.Stats().Syncs; got != before {
		t.Fatalf("Sync on a clean log fsynced: %d -> %d", before, got)
	}
}

func TestExplicitSyncCoversEverythingWrittenBeforeIt(t *testing.T) {
	d := newDisk()
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.Hooks = d.hooks() })
	for i := 1; i <= 5; i++ {
		mustAppend(t, l, i)
	}
	if !l.Stats().Dirty {
		t.Fatal("not dirty after appends")
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	st := l.Stats()
	if st.Dirty || st.SyncedBytes != st.OpenBytes {
		t.Fatalf("after Sync: %+v", st)
	}
	if d.syncedSize(filepath.Join(dir, tlog(1))) != st.OpenBytes {
		t.Fatal("the file's synced size is not the log's written size")
	}
}

func TestFsyncDoesNotHoldTheAppendLock(t *testing.T) {
	d := newDisk()
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.Hooks = d.hooks() })
	mustAppend(t, l, 1)
	d.set(func(d *disk) { d.block = make(chan struct{}); d.entered = make(chan struct{}, 1) })
	done := make(chan error, 1)
	go func() { done <- l.Sync() }()
	select {
	case <-d.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("Sync never reached the fsync")
	}
	appended := make(chan error, 1)
	go func() { appended <- l.Append(meta(2), lineFor(2)) }()
	select {
	case err := <-appended:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Append blocked behind an fsync in flight")
	}
	d.set(func(d *disk) { close(d.block); d.block = nil })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	// The line appended during the fsync is not covered by it.
	if !l.Stats().Dirty {
		t.Fatal("a line written during an fsync was counted as synced")
	}
}

func TestConcurrentSyncAndAppendAreRaceFree(t *testing.T) {
	d := newDisk()
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.Hooks = d.hooks(); o.SealRecords = 50 })
	var wg sync.WaitGroup
	var next atomic.Int64
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				n := int(next.Add(1))
				if err := l.Append(meta(n), lineFor(n)); err != nil {
					t.Error(err)
					return
				}
				if i%17 == 0 {
					if err := l.Sync(); err != nil {
						t.Error(err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	streams, _ := scanDir(t, dir)
	if len(streams) != 800 {
		t.Fatalf("%d lines survived, want 800", len(streams))
	}
}

func TestRotationBySizeRecordsAndAge(t *testing.T) {
	dir := t.TempDir()
	l, clk := openT(t, dir, func(o *Options) { o.SealRecords = 6; o.SealAge = time.Hour })
	for i := 1; i <= 20; i++ {
		mustAppend(t, l, i)
	}
	segs, err := l.Reader().Segments()
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) < 3 {
		t.Fatalf("20 lines at 6 records per segment made %d segments", len(segs))
	}
	for _, s := range segs[:len(segs)-1] {
		if !s.Sealed || s.Records > 6 {
			t.Fatalf("segment %d sealed=%v records=%d", s.Segno, s.Sealed, s.Records)
		}
	}
	if segs[len(segs)-1].Sealed {
		t.Fatal("the newest segment must be the open one")
	}
	// Age: one hour on, the next line starts a new segment.
	n := len(segs)
	clk.Advance(61 * time.Minute)
	mustAppend(t, l, 21)
	segs, _ = l.Reader().Segments()
	if len(segs) != n+1 {
		t.Fatalf("age rotation: %d segments, want %d", len(segs), n+1)
	}
	// Bytes.
	dir2 := t.TempDir()
	l2, _ := openT(t, dir2, func(o *Options) { o.SealBytes = 2048 })
	for i := 1; i <= 100; i++ {
		mustAppend(t, l2, i)
	}
	segs2, _ := l2.Reader().Segments()
	if len(segs2) < 3 {
		t.Fatalf("byte rotation made %d segments", len(segs2))
	}
	for _, s := range segs2[:len(segs2)-1] {
		if s.Bytes > 2048+int64(len(lineFor(100)))+200 {
			t.Fatalf("segment %d is %d bytes past the 2048 bound", s.Segno, s.Bytes)
		}
	}
}

func TestMaintainSealsAnAgedIdleSegment(t *testing.T) {
	dir := t.TempDir()
	l, clk := openT(t, dir, func(o *Options) { o.SealAge = time.Hour })
	mustAppend(t, l, 1)
	res, err := l.Maintain()
	if err != nil || res.Sealed {
		t.Fatalf("Maintain on a young segment: %+v %v", res, err)
	}
	clk.Advance(2 * time.Hour)
	res, err = l.Maintain()
	if err != nil || !res.Sealed {
		t.Fatalf("Maintain on an old segment: %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, tidx(1))); err != nil {
		t.Fatal("no index after Maintain sealed the segment")
	}
	// The next line opens a fresh segment.
	mustAppend(t, l, 2)
	segs, _ := l.Reader().Segments()
	if len(segs) != 2 {
		t.Fatalf("%d segments after Maintain and one more line", len(segs))
	}
}

// F18: the open segment's in-memory index is bounded by the record cap.
func TestOpenSegmentIndexIsBounded(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.SealRecords = 100 })
	for i := 1; i <= 1000; i++ {
		mustAppend(t, l, i)
		if e := l.Stats().OpenEntries; e > 100 {
			t.Fatalf("open index holds %d entries at line %d, cap is 100", e, i)
		}
	}
	if st := l.Stats(); st.SegmentsSealed < 9 {
		t.Fatalf("SegmentsSealed = %d", st.SegmentsSealed)
	}
}

// F13: an oversized line is refused without touching the log.
func TestOversizedLineIsRefusedAndHarmless(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, nil)
	mustAppend(t, l, 1)
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	before := tree(t, dir)
	err := l.Append(meta(2), make([]byte, MaxRecordBytes+1))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized Append = %v, want ErrTooLarge", err)
	}
	sameTree(t, "oversized line", before, tree(t, dir))
	if st := l.Stats(); st.TooLarge != 1 || st.Poisoned {
		t.Fatalf("stats after an oversized line: %+v", st)
	}
	// Just under the limit is accepted, and read back whole.
	big := make([]byte, MaxRecordBytes-512)
	for i := range big {
		big[i] = 'x'
	}
	if err := l.Append(meta(3), big); err != nil {
		t.Fatalf("a line under the limit was refused: %v", err)
	}
	got, _ := l.Reader().ByRun("owner-1", "run-a", 2, 10)
	if len(got) != 1 || len(got[0].Line) != len(big) {
		t.Fatalf("big line read back as %d records", len(got))
	}
}

func TestCloseSealsIsIdempotentAndRefusesAppends(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, nil)
	mustAppend(t, l, 1)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if err := l.Append(meta(2), lineFor(2)); !errors.Is(err, ErrClosed) {
		t.Fatalf("Append after Close = %v", err)
	}
	// A clean close means the next open is `open`, not `crash_recovered`.
	l2, _ := openT(t, dir, nil)
	mustAppend(t, l2, 2)
	if err := l2.Close(); err != nil {
		t.Fatal(err)
	}
	streams, reasons := scanDir(t, dir)
	if !eqInts(streams, []int64{1, 2}) || strings.Join(reasons, ",") != "open,open" {
		t.Fatalf("streams %v reasons %v", streams, reasons)
	}
	if st := l2.Stats(); st.TornTailsCut != 0 || st.DamagedTails != 0 {
		t.Fatalf("recovery counters moved on a clean reopen: %+v", st)
	}
}

func TestBoundariesAreRecordedAndSkippedByQueries(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, nil)
	mustAppend(t, l, 1)
	if err := l.AppendBoundary("restore", 77); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, l, 2)
	var got []Boundary
	_ = l.Reader().Scan(func(it Item) bool {
		if it.Boundary != nil {
			got = append(got, *it.Boundary)
		}
		return true
	})
	if len(got) != 2 || got[1].Reason != "restore" || got[1].HeadSeq != 77 {
		t.Fatalf("boundaries = %+v", got)
	}
	recs, _ := l.Reader().ByOwnerAfter("owner-1", 0, 10)
	if len(recs) != 2 {
		t.Fatalf("queries returned %d records, boundaries must be skipped", len(recs))
	}
}

// F12: two writers on one directory.
func TestSecondWriterCannotOpenTheSameDirectory(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, nil)
	mustAppend(t, l, 1)
	_, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1})
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("second Open = %v, want ErrLocked", err)
	}
	// The first keeps writing, and the failed Open changed nothing.
	mustAppend(t, l, 2)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l2, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1})
	if err != nil {
		t.Fatalf("Open after the first closed: %v", err)
	}
	_ = l2.Close()
	streams, _ := scanDir(t, dir)
	if !eqInts(streams, []int64{1, 2}) {
		t.Fatalf("lines = %v", streams)
	}
}

func TestSecondProcessCannotOpenTheSameDirectory(t *testing.T) {
	if d := os.Getenv("TRACELOG_HELPER_DIR"); d != "" {
		_, err := Open(Options{Dir: d, ComputerID: "computer-1", SyncInterval: -1})
		if errors.Is(err, ErrLocked) {
			fmt.Println("RESULT=locked")
		} else {
			fmt.Printf("RESULT=%v\n", err)
		}
		return
	}
	dir := t.TempDir()
	l, _ := openT(t, dir, nil)
	mustAppend(t, l, 1)
	cmd := exec.Command(os.Args[0], "-test.run=^TestSecondProcessCannotOpenTheSameDirectory$")
	cmd.Env = append(os.Environ(), "TRACELOG_HELPER_DIR="+dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "RESULT=locked") {
		t.Fatalf("second process was not locked out:\n%s", out)
	}
	mustAppend(t, l, 2)
}

func TestSymlinkedDirectoryAndSegmentAreRefused(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(Options{Dir: link, ComputerID: "c", SyncInterval: -1}); err == nil {
		t.Fatal("Open followed a symlinked log directory")
	}
	// A segment name that is a symlink is refused, and its target untouched.
	dir := filepath.Join(root, "log")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "victim")
	if err := os.WriteFile(target, []byte("do not touch"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, tlog(1))); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(Options{Dir: dir, ComputerID: "c", SyncInterval: -1}); err == nil {
		t.Fatal("Open accepted a symlink as a segment")
	}
	if string(mustRead(t, target)) != "do not touch" {
		t.Fatal("the symlink target was modified")
	}
}

func TestOptionsAreValidated(t *testing.T) {
	if _, err := Open(Options{Dir: "", ComputerID: "c"}); err == nil {
		t.Fatal("empty Dir accepted")
	}
	if _, err := Open(Options{Dir: t.TempDir(), ComputerID: ""}); err == nil {
		t.Fatal("empty ComputerID accepted")
	}
}
