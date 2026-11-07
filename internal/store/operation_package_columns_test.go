package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// An existing store gains the package-operation columns when it opens
// (docs/design/package-operation-decision-2026-11-06.md 2.3): a table created
// before them is altered in place, empty by default, and a second open is a
// no-op.
func TestOpenAddsThePackageOperationColumnsToAnOlderTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, column := range []string{"kind", "package_release_id", "package_install_id"} {
		if _, err := s.DB().ExecContext(ctx, `ALTER TABLE self_development_operations DROP COLUMN `+column); err != nil {
			t.Fatalf("drop %s: %v", column, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		s, err = Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", pass, err)
		}
		for _, column := range []string{"kind", "package_release_id", "package_install_id"} {
			has, err := dbx.ColumnExists(ctx, s.DB(), dbx.SQLite, "self_development_operations", column)
			if err != nil || !has {
				t.Fatalf("pass %d: column %s present=%v err=%v", pass, column, has, err)
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
