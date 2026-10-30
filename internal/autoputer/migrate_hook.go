package autoputer

import (
	"context"
	"encoding/json"
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
// leaves the old data untouched and the next boot retries. Once the state
// file exists, a later boot deletes the legacy Dolt database (C7 of the
// cutover; removeMigratedLegacyStore).
func migrateLegacyStoreIfNeeded(ctx context.Context, storePath string) error {
	statePath := store.StateDBPath(storePath)
	if _, err := os.Stat(statePath); err == nil {
		removeMigratedLegacyStore(storePath)
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

// removeMigratedLegacyStore deletes the legacy Dolt database directories of a
// computer whose own migration report says every table verified (C7 of the
// data system cutover: old data goes once the differential is green). The
// sidecar files the new store keeps at the same path stay. It never fails the
// boot: a deletion error is logged and the next boot tries again.
func removeMigratedLegacyStore(storePath string) {
	statePath := store.StateDBPath(storePath)
	if _, err := os.Stat(statePath); err != nil {
		return
	}
	legacy := store.LegacyDoltWorkspacePath(storePath)
	if !legacyDoltWorkspaceHoldsDatabase(legacy) {
		return
	}
	b, err := os.ReadFile(guestdolt.ReportPath(statePath))
	if err != nil {
		log.Printf("autoputer: legacy Dolt workspace %s kept: no migration report: %v", legacy, err)
		return
	}
	var rep guestdolt.Report
	if err := json.Unmarshal(b, &rep); err != nil || !rep.OK || rep.Skipped || rep.Copy == nil {
		log.Printf("autoputer: legacy Dolt workspace %s kept: migration report is not a verified copy", legacy)
		return
	}
	entries, err := os.ReadDir(legacy)
	if err != nil {
		log.Printf("autoputer: legacy Dolt workspace %s kept: %v", legacy, err)
		return
	}
	for _, entry := range entries {
		dir := filepath.Join(legacy, entry.Name())
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, ".dolt")); err != nil || !info.IsDir() {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("autoputer: remove legacy Dolt database %s: %v", dir, err)
			continue
		}
		log.Printf("autoputer: removed legacy Dolt database %s (migration verified %s)", dir, rep.FinishedAt.Format("2006-01-02T15:04:05Z"))
	}
}
