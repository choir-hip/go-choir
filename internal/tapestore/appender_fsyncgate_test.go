package tapestore

// Second review round (gpt-6.1-sol), failure modes written before the fix.
// The lesson is the fsyncgate one: after a failed fsync nothing past the last
// successful fsync is durable, and bytes that can still be read are not proof
// that they are. Writeback loss is modeled by a sync hook that cuts the file
// back to its last synced size (what a dropped dirty page leaves behind) and
// then fails.
//
//  F1 (P1) Recovery must not reuse an offset a live reader may have cached.
//      A call whose fsync fails, followed by loss of the unsynced tail, must
//      not let a later equal-sized record take the failed record's offset:
//      the reader that cached the failed record would then answer
//      ErrNotFound for the new record and ErrCorrupt for the old digest.
//  F2 (P1) Unsynced-record knowledge must survive a writer handoff. After
//      Appender A's fsync fails, Appender B (another instance over the same
//      root: a restart, a second owner) that is asked to append the same
//      record must write it again; it may not find the readable bytes, call
//      them a durable duplicate and acknowledge on a fresh fsync that has
//      nothing dirty to flush.
//  F3 (P1) If the failed call's own cleanup cannot finish (its fsync fails
//      too), the writer must remember and finish it before it writes
//      anything, and must not write to the abandoned segment meanwhile.
//  F4 (P1) Repeated failures: every failed call's records are abandoned,
//      never reused, and acknowledged records before them stay readable.
//  F5 (P2) The reader's open-scan cache is bounded by the number of open
//      segments, whichever segment a lookup happens to hit (see
//      TestReaderRetiresOpenScanStateWhenASegmentIsSealed).

import (
	"errors"
	"os"
	"sync"
	"syscall"
	"testing"
)

// lossyFsync returns a sync hook that fails n times; each failure first cuts
// the file back to frontier (the unsynced tail is lost) and runs beforeLoss.
func lossyFsync(n int, frontier int64, beforeLoss func()) func(*os.File) error {
	var mu sync.Mutex
	left := n
	ran := false
	return func(f *os.File) error {
		mu.Lock()
		fail := left > 0
		if fail {
			left--
		}
		mu.Unlock()
		if !fail {
			return f.Sync()
		}
		if beforeLoss != nil && !ran {
			ran = true
			beforeLoss()
		}
		_ = f.Truncate(frontier)
		return syscall.EIO
	}
}

func fileSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

func TestFailedFsyncThenTailLossDoesNotReuseACachedOffset(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})
	frontier := fileSize(t, segPath(root, cid, 1))

	first := body("AAAA failed-batch payload")
	second := body("BBBB failed-batch payload")
	if len(first) != len(second) {
		t.Fatalf("test needs equal lengths: %d vs %d", len(first), len(second))
	}
	a.syncFile = lossyFsync(1, frontier, func() { mustRead(t, r, cid, NSPayload, first) }) // the reader caches the unsynced record
	if err := a.Append(cid, Item{NSPayload, first}); !errors.Is(err, syscall.EIO) {
		t.Fatalf("Append = %v, want EIO", err)
	}
	mustAppend(t, a, cid, Item{NSPayload, second})
	if got, _, err := r.Read(cid, NSPayload, sum(second)); err != nil || string(got) != string(second) {
		t.Fatalf("the live reader cannot read a record acknowledged after the loss: %v", err)
	}
	if _, _, err := r.Read(cid, NSPayload, sum(first)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the failed, never acknowledged record = %v, want ErrNotFound (not ErrCorrupt)", err)
	}
	mustRead(t, r, cid, NSPayload, body("acknowledged base record"))
	scanCount(t, root, cid)
}

func TestAnotherWriterRewritesARecordWhoseFsyncFailed(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})
	p := body("payload whose fsync failed under writer A")
	a.syncFile = func(*os.File) error { return syscall.EIO } // no loss: the bytes stay readable
	if err := a.Append(cid, Item{NSPayload, p}); !errors.Is(err, syscall.EIO) {
		t.Fatalf("A.Append = %v", err)
	}
	a.syncFile = nil

	b := NewAppender(root) // the handoff: another instance, no memory of A's failure
	var rewrites int
	b.writeAt = func(f *os.File, rec []byte, off int64) (int, error) {
		if len(rec) == recHeaderSize+len(p) {
			rewrites++
		}
		return f.WriteAt(rec, off)
	}
	mustAppend(t, b, cid, Item{NSPayload, p})
	if rewrites != 1 {
		t.Fatalf("writer B wrote the failed record %d times, want 1: it acknowledged readable but unsynced bytes as a durable duplicate", rewrites)
	}
	mustRead(t, NewReader(root), cid, NSPayload, p)
	scanCount(t, root, cid)
}

func TestFailedCleanupIsFinishedBeforeAnyLaterWrite(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})
	frontier := fileSize(t, segPath(root, cid, 1))
	first := body("AAAA failed-batch payload")
	second := body("BBBB failed-batch payload")
	// Every fsync fails: the call's own cleanup cannot finish either.
	a.syncFile = lossyFsync(1<<20, frontier, func() { mustRead(t, r, cid, NSPayload, first) })
	if err := a.Append(cid, Item{NSPayload, first}); !errors.Is(err, syscall.EIO) {
		t.Fatalf("Append = %v, want EIO", err)
	}
	if err := a.Append(cid, Item{NSPayload, second}); !errors.Is(err, syscall.EIO) {
		t.Fatalf("Append with the disk still failing = %v, want EIO", err)
	}
	// The disk recovers. Nothing may be written to segment 1 from now on.
	a.syncFile = nil
	var toOld int
	a.writeAt = func(f *os.File, rec []byte, off int64) (int, error) {
		if f.Name() == segPath(root, cid, 1) {
			toOld++
		}
		return f.WriteAt(rec, off)
	}
	mustAppend(t, a, cid, Item{NSPayload, second})
	if toOld != 0 {
		t.Fatalf("%d writes went to the abandoned segment", toOld)
	}
	if got, _, err := r.Read(cid, NSPayload, sum(second)); err != nil || string(got) != string(second) {
		t.Fatalf("read of the record written after the cleanup: %v", err)
	}
	if _, _, err := r.Read(cid, NSPayload, sum(first)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the failed record = %v, want ErrNotFound", err)
	}
	if _, err := os.Stat(idxPath(root, cid, 1)); err != nil {
		t.Fatalf("the abandoned segment was never sealed at its synced frontier: %v", err)
	}
	if got := fileSize(t, segPath(root, cid, 1)); got != frontier {
		t.Fatalf("abandoned segment is %d bytes, want its synced frontier %d", got, frontier)
	}
	scanCount(t, root, cid)
}

func TestRepeatedFsyncFailuresNeverReuseAnOffset(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	base := body("acknowledged base record")
	mustAppend(t, a, cid, Item{NSPayload, base})
	var failed [][]byte
	for i := 0; i < 4; i++ {
		// Always append at the end of the highest segment; the frontier is its size now.
		last := lastSegPath(t, root)
		frontier := fileSize(t, last)
		p := body("failed %d payload of the same length", i)
		failed = append(failed, p)
		a.syncFile = lossyFsync(1, frontier, func() { mustRead(t, r, cid, NSPayload, p) })
		if err := a.Append(cid, Item{NSPayload, p}); !errors.Is(err, syscall.EIO) {
			t.Fatalf("Append %d = %v", i, err)
		}
		a.syncFile = nil
		ok := body("acked %d payload of the same length!", i)
		mustAppend(t, a, cid, Item{NSPayload, ok})
		mustRead(t, r, cid, NSPayload, ok)
		mustRead(t, r, cid, NSPayload, base)
	}
	for _, p := range failed {
		if _, _, err := r.Read(cid, NSPayload, sum(p)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("failed record = %v, want ErrNotFound", err)
		}
	}
	if n := scanCount(t, root, cid); n != 5 {
		t.Fatalf("records = %d, want the base and 4 acknowledged", n)
	}
}

// lastSegPath is the path of the highest-numbered segment file.
func lastSegPath(t *testing.T, root string) string {
	t.Helper()
	_, segs, err := NewReader(root).list(cid)
	if err != nil || len(segs) == 0 {
		t.Fatalf("list: %v (%d segments)", err, len(segs))
	}
	return segPath(root, cid, segs[len(segs)-1].segno)
}
