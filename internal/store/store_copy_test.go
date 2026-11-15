package store

// Tests of the store-copy token and the capture's leftover cleanup, written
// from the failure list of docs/design/capture-static-copy-2026-11-15.md
// section 3 before the code.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Failures 28 and 42: the token is exclusive, its release is idempotent (a late
// second release does not free a later holder), a waiter that is cancelled does
// not take it, and a waiter that is not cancelled gets it when it is released.
func TestTheStoreCopyTokenIsExclusiveAndReleasesIdempotently(t *testing.T) {
	s := &Store{}
	releaseA, _, ok := s.TryHoldStoreCopy("capture")
	if !ok {
		t.Fatal("the first request was refused")
	}
	if _, holder, ok := s.TryHoldStoreCopy("compact"); ok || holder != "capture" {
		t.Fatalf("a held token granted (%v) or named %q", ok, holder)
	}
	releaseA()
	releaseB, _, ok := s.TryHoldStoreCopy("compact")
	if !ok {
		t.Fatal("refused after release")
	}
	releaseA() // a second release by the old holder
	if _, _, ok := s.TryHoldStoreCopy("capture"); ok {
		t.Fatal("a late second release freed the next holder's token")
	}
	releaseB()
}

func TestAWaiterIsCancelledWithoutTakingTheTokenAndAnotherProceedsOnRelease(t *testing.T) {
	s := &Store{}
	release, _, _ := s.TryHoldStoreCopy("capture")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := s.HoldStoreCopy(ctx, "compact"); err == nil {
		t.Fatal("a cancelled wait returned a token")
	}
	got := make(chan error, 1)
	go func() {
		rel, err := s.HoldStoreCopy(context.Background(), "compact")
		if err == nil {
			rel()
		}
		got <- err
	}()
	select {
	case err := <-got:
		t.Fatalf("the waiter got the token while it was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	release()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the waiter never got the token after the release")
	}
	if _, _, ok := s.TryHoldStoreCopy("capture"); !ok {
		t.Fatal("the cancelled waiter or the finished one left the token held")
	}
}

func TestTheTokenSurvivesConcurrentUse(t *testing.T) {
	s := &Store{}
	var wg sync.WaitGroup
	inside := 0
	var mu sync.Mutex
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rel, err := s.HoldStoreCopy(context.Background(), "compact")
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			inside++
			if inside != 1 {
				t.Error("two holders at once")
			}
			mu.Unlock()
			time.Sleep(2 * time.Millisecond)
			mu.Lock()
			inside--
			mu.Unlock()
			rel()
		}()
	}
	wg.Wait()
}

// Failure 42: compaction waits for a capture's token; cancelled, it leaves the
// staging file alone.
func TestCompactionWaitsForTheTokenAndTouchesNothingWhileItWaits(t *testing.T) {
	s, path := compactTestStore(t, 20, 10, 400)
	release, _, ok := s.TryHoldStoreCopy("capture")
	if !ok {
		t.Fatal("token refused")
	}
	defer release()
	staging := path + compactStagingSuffix
	if err := os.WriteFile(staging, []byte("a copy the capture's token protects"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	staged, err := s.prepareCompactCopy(ctx, true)
	if staged || err == nil {
		t.Fatalf("staged=%v err=%v: compaction ran while the capture held the token", staged, err)
	}
	if raw, rerr := os.ReadFile(staging); rerr != nil || string(raw) != "a copy the capture's token protects" {
		t.Fatalf("the staging file was touched while compaction waited: %q %v", raw, rerr)
	}
}

func leftoverFixture(t *testing.T) (marker string, files map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	marker = filepath.Join(dir, "store")
	files = map[string][]byte{
		"state.db":                               []byte("the live store"),
		"state.db.compact":                       []byte("the staged copy"),
		"state.db.compact.json":                  []byte(`{"sequence":7}`),
		"state.db.compact-staging":               []byte("never trusted"),
		"state.db.compact-staging-wal":           []byte("w"),
		"state.db.compact-staging-shm":           []byte("s"),
		"state.db.compact-staging-journal":       []byte("j"),
		SnapshotScratchDirName + "/state.db":     []byte("scratch"),
		SnapshotScratchDirName + "/state.db-wal": []byte("w"),
		SnapshotScratchDirName + "/state.db-shm": []byte("s"),
	}
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return marker, files
}

// Failures 20, 21 and 26: the cleanup removes the scratch and the never-trusted
// staging file with every sidecar, and never the staged copy or its sidecar.
func TestTheCaptureCleanupNeverTouchesTheStagedCopy(t *testing.T) {
	marker, files := leftoverFixture(t)
	dir := filepath.Dir(marker)
	if err := RemoveCaptureLeftovers(marker); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"state.db.compact-staging", "state.db.compact-staging-wal", "state.db.compact-staging-shm", "state.db.compact-staging-journal", SnapshotScratchDirName} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Fatalf("%s survived: %v", gone, err)
		}
	}
	for _, kept := range []string{"state.db", "state.db.compact", "state.db.compact.json"} {
		raw, err := os.ReadFile(filepath.Join(dir, kept))
		if err != nil || !bytes.Equal(raw, files[kept]) {
			t.Fatalf("%s changed or vanished: %q %v", kept, raw, err)
		}
	}
}

// Failure 22 (the helper): the boot sweep removes the scratch directory and
// nothing else.
func TestTheBootSweepRemovesOnlyTheScratch(t *testing.T) {
	marker, files := leftoverFixture(t)
	dir := filepath.Dir(marker)
	if err := RemoveSnapshotScratch(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, SnapshotScratchDirName)); !os.IsNotExist(err) {
		t.Fatalf("the scratch survived the boot sweep: %v", err)
	}
	for name, body := range files {
		if filepath.Dir(name) == SnapshotScratchDirName {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(raw, body) {
			t.Fatalf("the boot sweep touched %s: %q %v", name, raw, err)
		}
	}
	if err := RemoveSnapshotScratch(marker); err != nil {
		t.Fatalf("sweeping an absent scratch must succeed: %v", err)
	}
}
