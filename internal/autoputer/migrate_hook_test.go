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
//   - it runs when the state file is absent.
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storePath, legacy := legacyFixture(t)
			if tc.state {
				writeState(t, storePath, tc.rep)
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
