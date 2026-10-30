package autoputer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/migrate/guestdolt"
	"github.com/yusefmosiah/go-choir/internal/migrate/rowcopy"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Ways the legacy-store deletion could go wrong, each checked below:
//   - it deletes a Dolt database whose migration was never verified (no
//     report, a failed report, or a skipped one);
//   - it deletes the sidecar files the new store keeps in the same directory
//     (schema receipt, floor, lease marker);
//   - it deletes a directory that is not a Dolt database;
//   - it runs when the state file is absent;
//   - it treats a zero-byte state file as a finished migration: the hook
//     must neither skip the migration nor delete the Dolt database for one
//     (review of the cutover, 2026-10-30: store.Open opens a zero-byte file
//     as a fresh store, so the computer would serve empty state).
func legacyFixture(t *testing.T) (storePath, legacy string) {
	t.Helper()
	dir := t.TempDir()
	storePath = filepath.Join(dir, "state")
	if err := os.WriteFile(storePath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	legacy = store.LegacyDoltWorkspacePath(storePath)
	for _, d := range []string{"texture/.dolt/noms", "notes"} {
		if err := os.MkdirAll(filepath.Join(legacy, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(legacy, "store-schema.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	return storePath, legacy
}

func writeState(t *testing.T, storePath string, rep *guestdolt.Report) {
	t.Helper()
	statePath := store.StateDBPath(storePath)
	if err := os.WriteFile(statePath, []byte("sqlite"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rep == nil {
		return
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(guestdolt.ReportPath(statePath), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func TestLegacyStoreRemovedOnlyAfterVerifiedMigration(t *testing.T) {
	verified := &guestdolt.Report{OK: true, Copy: &rowcopy.Report{}}
	cases := []struct {
		name   string
		rep    *guestdolt.Report
		state  bool
		remove bool
	}{
		{"verified", verified, true, true},
		{"no report", nil, true, false},
		{"failed report", &guestdolt.Report{OK: false, Copy: &rowcopy.Report{}}, true, false},
		{"skipped report", &guestdolt.Report{OK: true, Skipped: true}, true, false},
		{"no state file", nil, false, false},
		{"zero-byte state file", verified, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storePath, legacy := legacyFixture(t)
			if tc.state {
				writeState(t, storePath, tc.rep)
			}
			if tc.name == "zero-byte state file" {
				if err := os.Truncate(store.StateDBPath(storePath), 0); err != nil {
					t.Fatal(err)
				}
			}
			removeMigratedLegacyStore(storePath)
			if got := !exists(filepath.Join(legacy, "texture")); got != tc.remove {
				t.Fatalf("Dolt database removed = %v, want %v", got, tc.remove)
			}
			if !exists(filepath.Join(legacy, "store-schema.json")) || !exists(filepath.Join(legacy, "notes")) {
				t.Fatal("removed a sidecar or a directory that is not a Dolt database")
			}
		})
	}
}

func TestMigrateHookRemovesLegacyStoreOnLaterBoot(t *testing.T) {
	storePath, legacy := legacyFixture(t)
	writeState(t, storePath, &guestdolt.Report{OK: true, Copy: &rowcopy.Report{}})
	if err := migrateLegacyStoreIfNeeded(context.Background(), storePath); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(legacy, "texture")) {
		t.Fatal("legacy Dolt database kept after a verified migration")
	}
}

func TestMigrateHookDoesNotSkipForZeroByteStateFile(t *testing.T) {
	storePath, legacy := legacyFixture(t)
	statePath := store.StateDBPath(storePath)
	if err := os.WriteFile(statePath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// The fixture's Dolt database is not a real one, so the migrator fails;
	// what matters is that the hook tried, and kept the Dolt database.
	if err := migrateLegacyStoreIfNeeded(context.Background(), storePath); err == nil {
		t.Fatal("a zero-byte state file was taken as a finished migration")
	}
	if !exists(filepath.Join(legacy, "texture")) {
		t.Fatal("the Dolt database was deleted beside a zero-byte state file")
	}
}
