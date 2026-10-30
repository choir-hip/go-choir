package guestdolt

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/migrate/doltsrc"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// TestDefaultOpenerUsesTheRealStore migrates through the real store schema
// (the path autoputer takes): a legacy workspace beside a runtime store path
// becomes state.db, which the store then opens with the rows in place.
// Failure modes: the scratch directory or a WAL sidecar survives; the real
// store refuses the migrated file; a second boot migrates again.
func TestDefaultOpenerUsesTheRealStore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "choir.db")
	ws := store.LegacyDoltWorkspacePath(storePath)
	h, err := doltsrc.Create(ctx, ws, doltsrc.DatabaseName)
	if err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{
		`CREATE TABLE user_preferences (owner_id VARCHAR(64) NOT NULL, preference_key VARCHAR(64) NOT NULL, value_json LONGTEXT NOT NULL, updated_at DATETIME NOT NULL, extra_old_col INT NULL, PRIMARY KEY (owner_id, preference_key))`,
		`CREATE TABLE retired_table (id INT PRIMARY KEY)`,
		`INSERT INTO user_preferences VALUES ('o1', 'theme', '{"dark":true}', '2026-05-06 07:08:09', 7), ('o1', 'lang', '"fr"', '2026-05-06 07:08:10', NULL)`,
	} {
		if _, err := h.DB.ExecContext(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}

	rep, err := MigrateStore(ctx, storePath)
	if err != nil || !rep.OK {
		t.Fatalf("MigrateStore: %v %+v", err, rep)
	}
	if _, serr := os.Stat(store.StateDBPath(storePath)); serr != nil {
		t.Fatalf("state file missing: %v", serr)
	}
	if _, serr := os.Stat(TempPath(store.StateDBPath(storePath)) + ".d"); !errors.Is(serr, os.ErrNotExist) {
		t.Errorf("scratch directory left behind")
	}
	if len(rep.Copy.SourceOnlyTables) != 1 || rep.Copy.SourceOnlyTables[0] != "retired_table" {
		t.Errorf("source-only tables = %v", rep.Copy.SourceOnlyTables)
	}
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatalf("store.Open on the migrated file: %v", err)
	}
	var n int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM user_preferences`).Scan(&n); err != nil || n != 2 {
		t.Errorf("user_preferences rows = %d %v", n, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	// Second boot: nothing to do.
	rep, err = MigrateStore(ctx, storePath)
	if err != nil || !rep.Skipped {
		t.Errorf("second run: %v %+v", err, rep)
	}
}
