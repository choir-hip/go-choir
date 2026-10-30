package store

import "path/filepath"

// The data system cutover moves the guest store from the embedded Dolt
// workspace to one SQLite file (docs/design/data-system-build-sheet-2026-10-30.md).
// Both paths derive from the configured store path (the legacy marker,
// provideriface.DefaultStorePath), so the guest migrator and the store agree
// without configuration.

// StateDBPath is the SQLite state file for storePath. It is never storePath
// itself: a legacy SQLite file may still sit there as inert evidence.
func StateDBPath(storePath string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(storePath)), "state.db")
}

// LegacyDoltWorkspacePath is the embedded Dolt workspace the migrator reads.
func LegacyDoltWorkspacePath(storePath string) string {
	return deriveTextureWorkspacePath(storePath)
}
