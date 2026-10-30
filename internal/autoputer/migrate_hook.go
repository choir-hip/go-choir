package autoputer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/migrate/guestdolt"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// migrateLegacyStoreIfNeeded runs the guest Dolt to SQLite migrator (WP-F,
// internal/migrate/guestdolt) once: when the legacy embedded Dolt workspace
// exists and the SQLite state file does not. The migrator only reads the Dolt
// workspace and keeps it until the cutover deletes it, so a failed migration
// leaves the old data untouched and the next boot retries.
func migrateLegacyStoreIfNeeded(ctx context.Context, storePath string) error {
	statePath := store.StateDBPath(storePath)
	if _, err := os.Stat(statePath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat state file: %w", err)
	}
	legacy := store.LegacyDoltWorkspacePath(storePath)
	if !legacyDoltWorkspaceHoldsDatabase(legacy) {
		return nil
	}
	log.Printf("autoputer: migrating legacy Dolt workspace %s to %s", legacy, statePath)
	report, err := guestdolt.Migrate(ctx, legacy, statePath)
	if err != nil {
		// A partial state file must not look like a finished migration.
		_ = os.Remove(statePath)
		_ = os.Remove(statePath + "-wal")
		_ = os.Remove(statePath + "-shm")
		return fmt.Errorf("guest dolt migration: %w", err)
	}
	log.Printf("autoputer: guest dolt migration complete: %+v", report)
	return nil
}

// legacyDoltWorkspaceHoldsDatabase reports whether workspace is an embedded
// Dolt root holding a database (a child directory with a .dolt directory).
// The sidecar directory the new store keeps at the same path (schema receipt,
// floor, lease marker) is not a database.
func legacyDoltWorkspaceHoldsDatabase(workspace string) bool {
	entries, err := os.ReadDir(workspace)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(workspace, entry.Name(), ".dolt")); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
