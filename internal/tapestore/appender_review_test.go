package tapestore

// Review-panel regressions for the A2 writer. The fail-stop redesign
// (appender_failstop_test.go) replaced V1 and V1b: a failed call no longer
// repairs its segment, it poisons the writer, so a reader's cached location
// can never be removed or reused and the next writer writes anything it is
// asked to append afresh, in its own segment.
//
//  V3  (blocking) Directory durability is part of acknowledging: failures of
//      the directory fsyncs of the computer directory, tape-seg and the root
//      (segment creation), and of the segment directory at seal, are
//      returned and poison the writer; the next writer completes the whole
//      directory sequence before it acknowledges anything on top of a file
//      whose directory entry may never have been synced.
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
			failed := false
			a.syncDir = func(p string) error {
				if p == paths[target] && !failed {
					failed = true
					return syscall.EIO
				}
				return realSyncDir(p)
			}
			if err := a.Append(cid, Item{NSPayload, body("first")}); !errors.Is(err, syscall.EIO) {
				t.Fatalf("Append = %v, want the directory fsync error", err)
			}
			// The file created by the failed Append survives. The writer is
			// poisoned; the next writer must finish the whole sequence before
			// it acknowledges anything.
			b := NewAppender(root)
			var calls []syncCall
			b.syncDir = func(p string) error { err := realSyncDir(p); calls = append(calls, syncCall{p, err}); return err }
			mustAppend(t, b, cid, Item{NSPayload, body("second")})
			for name, p := range paths {
				ok := false
				for _, c := range calls {
					if c.path == p && c.err == nil {
						ok = true
					}
				}
				if !ok {
					t.Errorf("no successful fsync of %s (%s) before the next writer acknowledged", name, p)
				}
			}
		})
	}
}

func TestAppenderSealDirectoryFsyncFailureIsReturnedAndRedoneByTheNextWriter(t *testing.T) {
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
	// The renamed index exists but its directory entry was never synced: the
	// next writer syncs the directory before it acknowledges.
	b := NewAppender(root)
	var calls []syncCall
	b.syncDir = func(p string) error { err := realSyncDir(p); calls = append(calls, syncCall{p, err}); return err }
	mustAppend(t, b, cid, Item{NSPayload, body("after the failed seal")})
	ok := false
	for _, c := range calls {
		if c.path == segDir(root, cid) && c.err == nil {
			ok = true
		}
	}
	if !ok {
		t.Fatal("the next writer acknowledged without a successful directory fsync")
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
// so it never reads the open segment while another call is mid-batch.
func TestAppendFuncBuildsOnlyBetweenOtherCalls(t *testing.T) {
	root := t.TempDir()
	a := NewAppender(root)
	p := body("payload of the slow call")
	entered, release := make(chan struct{}), make(chan struct{})
	var aDone, buildSawDone bool
	var mu sync.Mutex
	first := true
	a.syncFile = func(f *os.File) error {
		mu.Lock()
		hold := first
		first = false
		mu.Unlock()
		if !hold {
			return f.Sync()
		}
		close(entered)
		<-release
		return f.Sync()
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
			return nil, nil
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
		t.Fatal("build did not see the record of the call that finished before it")
	}
	mustRead(t, NewReader(root), cid, NSPayload, p)
}
