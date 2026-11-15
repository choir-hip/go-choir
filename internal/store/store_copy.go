package store

// Scaffold for the static-copy capture
// (docs/design/capture-static-copy-2026-11-15.md, sections 2.4 and 2.6): inert
// until the code commit. The names are the design's.

import "context"

// SnapshotScratchDirName is the directory beside state.db the capture's copy
// lives in. It is the name the previous binary's attempt start removes.
const SnapshotScratchDirName = "snapshot-tmp"

// TryHoldStoreCopy is the capture's entry to the store-copy token (scaffold:
// grants every request).
func (s *Store) TryHoldStoreCopy(kind string) (release func(), holder string, ok bool) {
	return func() {}, "", true
}

// HoldStoreCopy is compaction's entry to the token (scaffold: grants at once).
func (s *Store) HoldStoreCopy(ctx context.Context, kind string) (release func(), err error) {
	return func() {}, nil
}

// RemoveSnapshotScratch removes the capture's scratch directory (scaffold).
func RemoveSnapshotScratch(storePath string) error { return nil }

// RemoveCaptureLeftovers removes the scratch directory and a never-trusted
// compact staging file with its sidecars, never the staged copy (scaffold).
func RemoveCaptureLeftovers(storePath string) error { return nil }
