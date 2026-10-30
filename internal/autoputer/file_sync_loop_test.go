package autoputer

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// Failure modes of the periodic file sync (2026-10-30, the lost-disk drill on
// d@e): an idle computer is hibernated about ten minutes after it boots and
// wakes cold, which restarts the 15-minute ticker, so a save never reached the
// host and a lost disk lost it. The loop must sync at a scan that finds the
// tree changed, not sync at a scan that finds it unchanged, and keep the
// interval as the floor. The scan must see any writer (mail and agents write
// beside the files API): an added, rewritten or removed file.
func TestFileSyncLoopSyncsAtTheScanAfterAChange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var dirty atomic.Bool
	var runs atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		runFileSyncLoop(ctx, time.Hour, 20*time.Millisecond, func() bool { return dirty.Load() }, func() { runs.Add(1); dirty.Store(false) })
	}()
	time.Sleep(100 * time.Millisecond)
	if got := runs.Load(); got != 0 {
		t.Fatalf("synced %d times with an unchanged tree, want 0", got)
	}
	dirty.Store(true)
	deadline := time.Now().Add(time.Second)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := runs.Load(); got != 1 {
		t.Fatalf("synced %d times after one change, want 1", got)
	}
	cancel()
	<-done
}

func TestChangedSinceSyncSeesAddedRewrittenAndRemovedFiles(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("a.txt", "a")
	s := newFileSync(root, "", nil, "computer-test", nil, nil, nil)
	if !s.changedSinceSync() {
		t.Fatal("a tree never synced this boot reads as unchanged")
	}
	empty := newFileSync(t.TempDir(), "", nil, "computer-test", nil, nil, nil)
	empty.cacheIsTree = true
	if empty.changedSinceSync() {
		t.Fatal("an empty tree that synced reads as changed")
	}
	record := func() {
		t.Helper()
		s.fileCache = map[string]cachedSyncedFile{}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			info, _ := d.Info()
			rel, _ := filepath.Rel(root, path)
			s.fileCache[filepath.ToSlash(rel)] = cachedSyncedFile{size: info.Size(), modTime: info.ModTime()}
			return nil
		})
		s.cacheIsTree = true
	}
	record()
	if s.changedSinceSync() {
		t.Fatal("an unchanged tree reads as changed")
	}
	write("mail.eml", "new")
	if !s.changedSinceSync() {
		t.Fatal("an added file reads as unchanged")
	}
	record()
	write("a.txt", "rewritten")
	if !s.changedSinceSync() {
		t.Fatal("a rewritten file reads as unchanged")
	}
	record()
	if err := os.Remove(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	if !s.changedSinceSync() {
		t.Fatal("a removed file reads as unchanged")
	}
}
