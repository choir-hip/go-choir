package tracelog

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// sealedLog builds a log with n sealed segments of five records each, one
// hour apart on the fake clock, and an open segment holding a few more.
func sealedLog(t *testing.T, dir string, n int, mut func(*Options)) (*Log, *fakeClock, int) {
	t.Helper()
	l, clk := openT(t, dir, func(o *Options) {
		o.SealRecords = 5
		if mut != nil {
			mut(o)
		}
	})
	i := 0
	for s := 0; s < n; s++ {
		for k := 0; k < 4; k++ { // 4 lines + the boundary or 5 lines fill a segment; rotate on the next append
			i++
			mustAppend(t, l, i)
		}
		clk.Advance(time.Hour)
	}
	for k := 0; k < 3; k++ {
		i++
		mustAppend(t, l, i)
	}
	return l, clk, i
}

// F7. Age bound: only sealed segments, oldest first; the open one stays.
func TestRetentionByAgeDeletesOldestSealedSegmentsOnly(t *testing.T) {
	dir := t.TempDir()
	l, clk, last := sealedLog(t, dir, 6, func(o *Options) { o.MaxAge = 72 * time.Hour })
	segsBefore, _ := l.Reader().Segments()
	clk.Advance(70 * time.Hour)
	res, err := l.Retain()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) == 0 {
		t.Fatal("retention deleted nothing")
	}
	segsAfter, _ := l.Reader().Segments()
	if segsAfter[len(segsAfter)-1].Sealed {
		t.Fatal("the open segment was touched")
	}
	if segsAfter[len(segsAfter)-1].Segno != segsBefore[len(segsBefore)-1].Segno {
		t.Fatal("the open segment changed")
	}
	// What is deleted is a prefix, oldest first, and every kept sealed segment is younger than the bound.
	for i, s := range res.Deleted {
		if s != segsBefore[i].Segno {
			t.Fatalf("deleted %v: not oldest-first", res.Deleted)
		}
	}
	for _, s := range segsAfter {
		if s.Sealed && clk.Now().Sub(s.SealedAt) > 72*time.Hour {
			t.Fatalf("segment %d, sealed %v ago, was kept past the age bound", s.Segno, clk.Now().Sub(s.SealedAt))
		}
	}
	for _, s := range res.Deleted {
		for _, name := range []string{tlog(s), tidx(s)} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				t.Fatalf("%s still exists", name)
			}
		}
	}
	// The floor moved and names the oldest kept line.
	fl, err := l.Reader().Floor()
	if err != nil || fl.Empty {
		t.Fatalf("Floor: %+v %v", fl, err)
	}
	if fl.Segment != segsAfter[0].Segno || fl.StreamSeq <= 0 {
		t.Fatalf("Floor = %+v, oldest kept segment %d", fl, segsAfter[0].Segno)
	}
	// Lines of deleted segments are gone, later lines are not.
	got, _ := l.Reader().ByOwnerAfter("owner-1", 0, 1000)
	if len(got) == 0 || got[0].Meta.StreamSeq < fl.StreamSeq || got[len(got)-1].Meta.StreamSeq != int64(last) {
		t.Fatalf("after retention: %d records, first %d, floor %d", len(got), got[0].Meta.StreamSeq, fl.StreamSeq)
	}
}

func TestRetentionByBytesStopsAtTheCapAndNeverDeletesTheOpenSegment(t *testing.T) {
	dir := t.TempDir()
	l, _, _ := sealedLog(t, dir, 6, nil)
	segs, _ := l.Reader().Segments()
	var total int64
	for _, s := range segs {
		total += s.Bytes + s.IndexBytes
	}
	capBytes := total / 2
	l.SetRetention(0, capBytes)
	res, err := l.Retain()
	if err != nil {
		t.Fatal(err)
	}
	after, _ := l.Reader().Segments()
	var kept int64
	for _, s := range after {
		kept += s.Bytes + s.IndexBytes
	}
	if kept > capBytes {
		t.Fatalf("kept %d bytes over the %d cap (deleted %v)", kept, capBytes, res.Deleted)
	}
	// A cap below even the open segment deletes every sealed segment and keeps the open one.
	l.SetRetention(0, 1)
	if _, err := l.Retain(); err != nil {
		t.Fatal(err)
	}
	after, _ = l.Reader().Segments()
	if len(after) != 1 || after[0].Sealed {
		t.Fatalf("segments after a 1 byte cap: %+v", after)
	}
	// The log still works.
	mustAppend(t, l, 999)
}

func TestRetentionRemovesOrphanIndexesAndOldQuarantine(t *testing.T) {
	dir := t.TempDir()
	l, clk, _ := sealedLog(t, dir, 3, func(o *Options) { o.MaxAge = time.Hour })
	segs, _ := l.Reader().Segments()
	// An orphan .tidx whose .tlog is gone (a crash between the two unlinks).
	if err := os.Remove(filepath.Join(dir, tlog(segs[0].Segno))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, tlog(99)+".bad"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	clk.Advance(48 * time.Hour)
	// The .bad file's age is its mtime; make it old.
	old := clk.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(filepath.Join(dir, tlog(99)+".bad"), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Retain(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, tidx(segs[0].Segno))); err == nil {
		t.Fatal("orphan index survived retention")
	}
	if _, err := os.Stat(filepath.Join(dir, tlog(99)+".bad")); err == nil {
		t.Fatal("old quarantined file survived retention")
	}
}

// F7: a reader holding a segment open finishes it after retention unlinks it,
// and segments that vanish before it gets to them are skipped, not errors.
func TestReaderFinishesAfterRetentionDeletesUnderIt(t *testing.T) {
	dir := t.TempDir()
	l, clk, _ := sealedLog(t, dir, 5, func(o *Options) { o.MaxAge = time.Hour })
	segs, _ := l.Reader().Segments()
	clk.Advance(24 * time.Hour)
	var seen []int64
	var segsSeen []uint64
	first := true
	err := l.Reader().Scan(func(it Item) bool {
		if first {
			first = false
			if _, err := l.Retain(); err != nil {
				t.Errorf("Retain during a scan: %v", err)
			}
		}
		if it.Boundary == nil {
			seen = append(seen, it.Meta.StreamSeq)
			if len(segsSeen) == 0 || segsSeen[len(segsSeen)-1] != it.Segment {
				segsSeen = append(segsSeen, it.Segment)
			}
		}
		return true
	})
	if err != nil {
		t.Fatalf("Scan across a retention pass: %v", err)
	}
	// The segment being read when retention ran is read to its end.
	if len(seen) < segs[0].Lines {
		t.Fatalf("scan saw %d lines, the segment it was inside holds %d", len(seen), segs[0].Lines)
	}
	if len(segsSeen) == 0 || segsSeen[0] != segs[0].Segno {
		t.Fatalf("scan began in segment %v, want %d", segsSeen, segs[0].Segno)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Fatalf("scan out of order: %v", seen)
		}
	}
}

func TestQueriesRaceWithAppendRotationAndRetention(t *testing.T) {
	dir := t.TempDir()
	l, clk := openT(t, dir, func(o *Options) { o.SealRecords = 20; o.MaxAge = time.Hour })
	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= 600; i++ {
			if err := l.Append(meta(i), lineFor(i)); err != nil {
				t.Error(err)
				return
			}
			if i%25 == 0 {
				clk.Advance(30 * time.Minute)
				if _, err := l.Retain(); err != nil {
					t.Error(err)
					return
				}
			}
		}
		close(stop)
	}()
	for q := 0; q < 3; q++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				recs, err := l.Reader().ByRun("owner-1", "run-a", 0, 50)
				if err != nil {
					t.Errorf("ByRun: %v", err)
					return
				}
				for i := 1; i < len(recs); i++ {
					if recs[i].Meta.Seq <= recs[i-1].Meta.Seq {
						t.Errorf("not ascending: %d after %d", recs[i].Meta.Seq, recs[i-1].Meta.Seq)
						return
					}
				}
				if _, err := OpenReader(dir).ByOwnerAfter("owner-1", 0, 20); err != nil {
					t.Errorf("static ByOwnerAfter: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
