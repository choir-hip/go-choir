package store

// The store-copy token and the capture's leftover cleanup
// (docs/design/capture-static-copy-2026-11-15.md, sections 2.4 and 2.6).
//
// A snapshot capture's scratch copy and a compaction's staging copy are both
// store-sized files written to one volume, and nothing ordered them. One
// in-process token does: the capture takes it without waiting (a refused
// capture retries at its next tick), compaction waits for it. A crash releases
// it because it lives in the process.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SnapshotScratchDirName is the directory beside state.db the capture's copy
// lives in. It is the name the previous binary's attempt start removes, so a
// rollback needs no new cleanup.
const SnapshotScratchDirName = "snapshot-tmp"

// storeCopyToken is the exclusion between a capture's scratch copy and a
// compaction's staging copy.
type storeCopyToken struct {
	once   sync.Once
	slot   chan struct{}
	mu     sync.Mutex
	holder string
	seq    uint64
	// freed is closed, and replaced, at every release: a waiter that read it
	// before trying to take the token cannot miss the release.
	freed chan struct{}
}

func (t *storeCopyToken) init() {
	t.once.Do(func() {
		t.slot = make(chan struct{}, 1)
		t.freed = make(chan struct{})
	})
}

// releaseFor returns an idempotent release of the hold numbered seq: a late
// second call by an old holder cannot free the next holder's hold.
func (t *storeCopyToken) releaseFor(seq uint64) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			t.mu.Lock()
			if t.seq == seq {
				t.holder = ""
				<-t.slot
				close(t.freed)
				t.freed = make(chan struct{})
			}
			t.mu.Unlock()
		})
	}
}

func (t *storeCopyToken) take(kind string) (func(), bool) {
	select {
	case t.slot <- struct{}{}:
	default:
		return nil, false
	}
	t.mu.Lock()
	t.seq++
	t.holder = kind
	seq := t.seq
	t.mu.Unlock()
	return t.releaseFor(seq), true
}

func (t *storeCopyToken) holderName() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.holder
}

// TryHoldStoreCopy is the capture's entry: it takes the token if it is free and
// otherwise returns the holder's name without waiting. release is idempotent.
func (s *Store) TryHoldStoreCopy(kind string) (release func(), holder string, ok bool) {
	if s == nil {
		return func() {}, "", true
	}
	s.copyToken.init()
	if release, ok := s.copyToken.take(kind); ok {
		return release, "", true
	}
	return nil, s.copyToken.holderName(), false
}

// HoldStoreCopy is compaction's entry: it waits for the token until ctx ends. A
// cancelled wait returns an error and takes nothing. release is idempotent.
func (s *Store) HoldStoreCopy(ctx context.Context, kind string) (release func(), err error) {
	if s == nil {
		return func() {}, nil
	}
	s.copyToken.init()
	for {
		s.copyToken.mu.Lock()
		freed := s.copyToken.freed
		s.copyToken.mu.Unlock()
		if release, ok := s.copyToken.take(kind); ok {
			return release, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-freed:
		}
	}
}

// scratchDir is the capture's scratch directory for the store at storePath.
func scratchDir(storePath string) string {
	return filepath.Join(filepath.Dir(StateDBPath(storePath)), SnapshotScratchDirName)
}

// RemoveSnapshotScratch removes the capture's scratch directory. It is the
// earliest boot step (before the compact rebase, the store open and the replay)
// and runs again at Runtime.Start; an absent directory is success.
func RemoveSnapshotScratch(storePath string) error {
	if err := os.RemoveAll(scratchDir(storePath)); err != nil {
		return fmt.Errorf("remove the snapshot scratch %s: %w", scratchDir(storePath), err)
	}
	return nil
}

// RemoveCaptureLeftovers is the capture's cleanup step, run under the token: the
// scratch directory and a compact staging file with its sidecars (never trusted
// after a crash, compact.go). It never touches the staged copy
// (state.db.compact) or its sidecar.
func RemoveCaptureLeftovers(storePath string) error {
	if err := RemoveSnapshotScratch(storePath); err != nil {
		return err
	}
	removeSQLiteFiles(StateDBPath(storePath) + compactStagingSuffix)
	return nil
}

// CompactStagedBytes is the size of the staged compact copy beside the store, 0
// when there is none. The capture names it in a refusal; it never removes it.
func CompactStagedBytes(storePath string) int64 {
	info, err := os.Stat(StateDBPath(storePath) + compactStagedSuffix)
	if err != nil {
		return 0
	}
	return info.Size()
}
