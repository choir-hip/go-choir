package tapestore

// Review-panel regressions for the A2 writer, written before the fixes.
//
//  V1  (blocking) A reader must never hold a record location that the writer
//      later removes or reuses. A failed Append (a failed fsync, or a short
//      write) must not truncate a whole record away: a live reader that cached
//      it would keep a stale location, and an equal-sized rewrite would then be
//      invisible (ErrNotFound) while the old digest reads as ErrCorrupt.
//      Regression: a reader active during the failed Append, then equal-sized
//      and larger appends.
//  V1b Records that survive a failed fsync are not trusted as durable: the
//      next Append rewrites their identical bytes before it fsyncs.
//  V3  (blocking) Directory durability is part of acknowledging: failures of
//      the directory fsyncs of the computer directory, tape-seg and the root
//      (segment creation), and of the segment directory at seal, are
//      returned; and a surviving file whose directory sync never completed is
//      not accepted by recovery until the sequence completes.
//  V4  Symlinks are refused: tape-seg/, tape-seg/<computer>, a segment file,
//      and a seg-N.idx.tmp that is a link never redirect a write; a stale
//      regular temp file is replaced.
//  V5  A reader retires its open-segment scan state when the segment is
//      sealed: repeated rotations keep the cache bounded.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestReaderCacheSurvivesAFailedFsyncAndLaterRewrites(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})

	first := body("AAAA failed-batch payload")
	second := append([]byte("BBBB"), bytes.Repeat([]byte("b"), len(first)-4)...) // same length as first
	if len(first) != len(second) {
		t.Fatalf("test needs equal lengths: %d vs %d", len(first), len(second))
	}
	a.syncFile = func(*os.File) error {
		mustRead(t, r, cid, NSPayload, first) // the reader caches the unsynced record
		return syscall.EIO
	}
	if err := a.Append(cid, Item{NSPayload, first}); !errors.Is(err, syscall.EIO) {
		t.Fatalf("Append = %v, want EIO", err)
	}
	a.syncFile = nil
	third := body("CCCC a larger payload that does not fit where the failed one was written")
	for _, next := range [][]byte{second, third} {
		mustAppend(t, a, cid, Item{NSPayload, next})
		if got, _, err := r.Read(cid, NSPayload, sum(next)); err != nil || !bytes.Equal(got, next) {
			t.Fatalf("the live reader cannot read a record appended after a failed Append: %v", err)
		}
		if _, _, err := r.Read(cid, NSPayload, sum(first)); errors.Is(err, ErrCorrupt) {
			t.Fatalf("the live reader reports corruption for the failed batch's record: %v", err)
		}
	}
	if scanCount(t, root, cid) < 3 {
		t.Fatal("scan lost acknowledged records")
	}
}

func TestReaderCacheSurvivesAShortWriteAndLaterRewrites(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	r := NewReader(root)
	mustAppend(t, a, cid, Item{NSPayload, body("acknowledged base record")})
	first, torn := body("DDDD first record of a failing batch"), body("EEEE second record that is cut short by ENOSPC")
	calls := 0
	a.writeAt = func(f *os.File, b []byte, off int64) (int, error) {
		calls++
		if calls == 2 {
			mustRead(t, r, cid, NSPayload, first) // the first record is whole and readable
			n, _ := f.WriteAt(b[:len(b)/2], off)
			return n, syscall.ENOSPC
		}
		return f.WriteAt(b, off)
	}
	if err := a.Append(cid, Item{NSPayload, first}, Item{NSPayload, torn}); !errors.Is(err, syscall.ENOSPC) {
		t.Fatalf("Append = %v", err)
	}
	a.writeAt = nil
	same := append([]byte("FFFF"), bytes.Repeat([]byte("f"), len(first)-4)...)
	for _, next := range [][]byte{same, body("GGGG a much larger record than anything written before it, so it overruns")} {
		mustAppend(t, a, cid, Item{NSPayload, next})
		if got, _, err := r.Read(cid, NSPayload, sum(next)); err != nil || !bytes.Equal(got, next) {
			t.Fatalf("the live reader cannot read a record appended after a short write: %v", err)
		}
		if _, _, err := r.Read(cid, NSPayload, sum(first)); errors.Is(err, ErrCorrupt) {
			t.Fatalf("stale location for the failed batch's first record: %v", err)
		}
	}
	if _, _, err := r.Read(cid, NSPayload, sum(torn)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("the cut-short record resolved: %v", err)
	}
	scanCount(t, root, cid)
}

func TestAppenderRewritesUnsyncedRecordsBeforeAcknowledgingThem(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	p := body("payload whose first fsync failed")
	a.syncFile = func(*os.File) error { return syscall.EIO }
	if err := a.Append(cid, Item{NSPayload, p}); err == nil {
		t.Fatal("expected a failed fsync")
	}
	a.syncFile = nil
	var offs []int64
	var lens []int
	a.writeAt = func(f *os.File, b []byte, off int64) (int, error) {
		offs, lens = append(offs, off), append(lens, len(b))
		return f.WriteAt(b, off)
	}
	mustAppend(t, a, cid, Item{NSPayload, p}) // the retry: a duplicate of a record the kernel may have dropped
	want := int64(segHeaderSize)
	found := false
	for i, o := range offs {
		if o == want && lens[i] == recHeaderSize+len(p) {
			found = true
		}
	}
	if !found {
		t.Fatalf("the retried record's bytes were not rewritten at their offset before the fsync (writes at %v)", offs)
	}
	if n := scanCount(t, root, cid); n != 1 {
		t.Fatalf("records = %d, want 1", n)
	}
}

type syncCall struct {
	path string
	err  error
}

func TestAppenderCompletesDirectoryDurabilityBeforeAcknowledging(t *testing.T) {
	for _, target := range []string{"root", "tape-seg", "computer"} {
		t.Run(target, func(t *testing.T) {
			root := t.TempDir()
			paths := map[string]string{"root": filepath.Clean(root), "tape-seg": filepath.Join(root, SegDirName), "computer": segDir(root, cid)}
			a := NewAppender(root)
			var calls []syncCall
			failed := false
			a.syncDir = func(p string) error {
				if p == paths[target] && !failed {
					failed = true
					calls = append(calls, syncCall{p, syscall.EIO})
					return syscall.EIO
				}
				err := realSyncDir(p)
				calls = append(calls, syncCall{p, err})
				return err
			}
			if err := a.Append(cid, Item{NSPayload, body("first")}); !errors.Is(err, syscall.EIO) {
				t.Fatalf("Append = %v, want the directory fsync error", err)
			}
			mark := len(calls)
			// The file created by the failed Append now survives. The next Append
			// must finish the sequence, not accept the file as it is.
			mustAppend(t, a, cid, Item{NSPayload, body("second")})
			for name, p := range paths {
				ok := false
				for _, c := range calls[mark:] {
					if c.path == p && c.err == nil {
						ok = true
					}
				}
				if !ok {
					t.Errorf("no successful fsync of %s (%s) between the failure and the acknowledgement", name, p)
				}
			}
			// A fresh process (cold cache) over the same files syncs again before it acknowledges.
			b := NewAppender(root)
			var cold []syncCall
			b.syncDir = func(p string) error { err := realSyncDir(p); cold = append(cold, syncCall{p, err}); return err }
			mustAppend(t, b, cid, Item{NSPayload, body("third")})
			if len(cold) == 0 {
				t.Error("recovery of an existing open segment did not run the directory sequence")
			}
		})
	}
}

func TestAppenderReturnsASealDirectoryFsyncFailure(t *testing.T) {
	root := t.TempDir()
	a := newAppender(root, 1<<30, 1)
	mustAppend(t, a, cid, Item{NSPayload, body("fills the segment")})
	var calls []syncCall
	failed := false
	a.syncDir = func(p string) error {
		if _, err := os.Stat(idxPath(root, cid, 1)); err == nil && p == segDir(root, cid) && !failed {
			failed = true
			calls = append(calls, syncCall{p, syscall.EIO})
			return syscall.EIO
		}
		err := realSyncDir(p)
		calls = append(calls, syncCall{p, err})
		return err
	}
	if err := a.Append(cid, Item{NSPayload, body("triggers the seal")}); !errors.Is(err, syscall.EIO) {
		t.Fatalf("Append = %v, want the seal's directory fsync error", err)
	}
	mark := len(calls)
	mustAppend(t, a, cid, Item{NSPayload, body("after the failed seal")})
	ok := false
	for _, c := range calls[mark:] {
		if c.path == segDir(root, cid) && c.err == nil {
			ok = true
		}
	}
	if !ok {
		t.Fatal("the renamed index was acknowledged without a successful directory fsync")
	}
	scanCount(t, root, cid)
}

func TestSealIndexPropagatesDirectorySyncErrors(t *testing.T) {
	dir := t.TempDir()
	err := sealIndex(dir, 1, cid, segHeaderSize, map[openKey]idxEntry{}, func(string) error { return syscall.EIO })
	if !errors.Is(err, syscall.EIO) {
		t.Fatalf("sealIndex = %v, want EIO", err)
	}
}

func TestAppenderRefusesSymlinks(t *testing.T) {
	t.Run("computer directory", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, SegDirName), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, segDir(root, cid)); err != nil {
			t.Fatal(err)
		}
		if err := NewAppender(root).Append(cid, Item{NSPayload, body("x")}); err == nil {
			t.Fatal("Append followed a symlinked computer directory")
		}
		if entries, _ := os.ReadDir(outside); len(entries) != 0 {
			t.Fatalf("a write escaped to %v", entries)
		}
	})
	t.Run("tape-seg", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, SegDirName)); err != nil {
			t.Fatal(err)
		}
		if err := NewAppender(root).Append(cid, Item{NSPayload, body("x")}); err == nil {
			t.Fatal("Append followed a symlinked tape-seg")
		}
		if entries, _ := os.ReadDir(outside); len(entries) != 0 {
			t.Fatalf("a write escaped to %v", entries)
		}
	})
	t.Run("segment file", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		mustAppend(t, NewAppender(root), cid, Item{NSPayload, body("real")})
		real := filepath.Join(outside, "seg")
		if err := os.Rename(segPath(root, cid, 1), real); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(real, segPath(root, cid, 1)); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(real)
		if err := NewAppender(root).Append(cid, Item{NSPayload, body("must not be appended through a link")}); err == nil {
			t.Fatal("Append wrote through a symlinked segment")
		}
		if after, _ := os.ReadFile(real); !bytes.Equal(before, after) {
			t.Fatal("the link target was modified")
		}
	})
	t.Run("index temp file", func(t *testing.T) {
		root, outside := t.TempDir(), t.TempDir()
		a := newAppender(root, 1<<30, 1)
		mustAppend(t, a, cid, Item{NSPayload, body("fills the segment")})
		victim := filepath.Join(outside, "victim")
		if err := os.WriteFile(victim, []byte("do not touch"), 0o640); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(victim, idxPath(root, cid, 1)+".tmp"); err != nil {
			t.Fatal(err)
		}
		if err := a.Append(cid, Item{NSPayload, body("triggers the seal")}); err == nil {
			t.Fatal("the seal wrote through a symlinked temp file")
		}
		if got, _ := os.ReadFile(victim); string(got) != "do not touch" {
			t.Fatalf("the link target was modified: %q", got)
		}
	})
	t.Run("stale regular temp file is replaced", func(t *testing.T) {
		root := t.TempDir()
		a := newAppender(root, 1<<30, 1)
		mustAppend(t, a, cid, Item{NSPayload, body("fills the segment")})
		if err := os.WriteFile(idxPath(root, cid, 1)+".tmp", []byte("left by a crash"), 0o640); err != nil {
			t.Fatal(err)
		}
		mustAppend(t, a, cid, Item{NSPayload, body("triggers the seal")})
		if scanCount(t, root, cid) != 2 {
			t.Fatal("scan count")
		}
	})
}

func TestReaderRetiresOpenScanStateWhenASegmentIsSealed(t *testing.T) {
	root := t.TempDir()
	a := newAppender(root, 1<<30, 2)
	r := NewReader(root)
	var mu sync.Mutex
	for i := 0; i < 12; i++ {
		b := body("record %d", i)
		mustAppend(t, a, cid, Item{NSPayload, b})
		mustRead(t, r, cid, NSPayload, b)
	}
	// No rereading of old records: only the newest segment was ever probed by
	// the loop above, and the cache must still be bounded (second review round).
	mu.Lock()
	defer mu.Unlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.open) > 1 {
		t.Fatalf("the reader keeps scan state for %d open segments after %d were sealed", len(r.open), len(r.sealed))
	}
}

// V2 (tapestore half): AppendFunc's build step runs inside the writer's lock,
// so it never reads the open segment while another call is mid-batch, and
// what it reads includes a failed call's whole records, which the same call
// then makes durable.
func TestAppendFuncBuildsOnlyBetweenOtherCalls(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	p := body("payload of the call that fails its fsync")
	entered, release := make(chan struct{}), make(chan struct{})
	var aDone, buildSawDone bool
	var mu sync.Mutex
	a.syncFile = func(f *os.File) error {
		select {
		case <-entered:
			return f.Sync()
		default:
		}
		close(entered)
		<-release
		return syscall.EIO
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = a.Append(cid, Item{NSPayload, p})
		mu.Lock()
		aDone = true
		mu.Unlock()
	}()
	<-entered
	var sawP bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := a.AppendFunc(cid, func() ([]Item, error) {
			mu.Lock()
			buildSawDone = aDone
			mu.Unlock()
			_, found, err := NewReader(root).ReadSegment(cid, NSPayload, sum(p))
			sawP = found
			if err != nil {
				return nil, err
			}
			return nil, nil // P is present: write nothing, only make it durable
		})
		if err != nil {
			t.Errorf("AppendFunc: %v", err)
		}
	}()
	time.Sleep(150 * time.Millisecond)
	close(release)
	wg.Wait()
	if !buildSawDone {
		t.Fatal("build ran while another call was still inside its batch")
	}
	if !sawP {
		t.Fatal("build did not see the failed call's whole record")
	}
	mustRead(t, NewReader(root), cid, NSPayload, p)
}
