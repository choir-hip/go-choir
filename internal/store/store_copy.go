package store

// The boot sweep of what a crashed capture or a crashed compaction leaves
// beside the state file. No capture writes a copy of the store any more
// (docs/design/snapshot-one-pass-2026-11-22.md), so the store-copy token that
// ordered a capture's scratch copy against a compaction's staging copy is gone:
// compaction takes nothing and waits for nothing.

import (
	"fmt"
	"os"
	"path/filepath"
)

// SnapshotScratchDirName is the directory beside state.db an old capture's copy
// lived in. A binary from before the one-pass capture can leave it behind, so
// boot still removes it.
const SnapshotScratchDirName = "snapshot-tmp"

// scratchDir is the old capture's scratch directory for the store at storePath.
func scratchDir(storePath string) string {
	return filepath.Join(filepath.Dir(StateDBPath(storePath)), SnapshotScratchDirName)
}

// RemoveSnapshotScratch removes the old capture's scratch directory. It is the
// earliest boot step (before the compact rebase, the store open and the replay)
// and runs again at Runtime.Start; an absent directory is success.
func RemoveSnapshotScratch(storePath string) error {
	if err := os.RemoveAll(scratchDir(storePath)); err != nil {
		return fmt.Errorf("remove the snapshot scratch %s: %w", scratchDir(storePath), err)
	}
	return nil
}

// RemoveBootScratch is the boot sweep before the store opens: the old scratch
// directory and a compaction's staging file with its sidecars. A compaction
// killed in flight leaves that file, up to the store's size, and the capture
// that used to sweep it no longer runs; nothing at boot is compacting. It never
// touches the staged copy (state.db.compact), its sidecar or the store. Absent
// files are success.
func RemoveBootScratch(storePath string) error {
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
