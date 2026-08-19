package store

import (
	"path/filepath"
	"testing"
)

func TestTextureDocumentAliasDocIndexReshapesLegacyName(t *testing.T) {
	showKeys := func(t *testing.T, s *Store) map[string]string {
		t.Helper()
		rows, err := s.textureHandle().Query("SHOW COLUMNS FROM texture_document_aliases")
		if err != nil {
			t.Fatalf("show columns: %v", err)
		}
		defer rows.Close()
		keys := map[string]string{}
		for rows.Next() {
			var field, typ, null, key, extra string
			var def any
			if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err != nil {
				t.Fatalf("scan columns: %v", err)
			}
			keys[field] = key
		}
		return keys
	}
	indexCols := func(t *testing.T, s *Store) []string {
		t.Helper()
		rows, err := s.textureHandle().Query(`
SELECT column_name
FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND table_name = 'texture_document_aliases'
  AND index_name = 'idx_texture_aliases_doc'
ORDER BY seq_in_index`)
		if err != nil {
			t.Fatalf("query alias doc index: %v", err)
		}
		defer rows.Close()
		var cols []string
		for rows.Next() {
			var col string
			if err := rows.Scan(&col); err != nil {
				t.Fatalf("scan alias doc index: %v", err)
			}
			cols = append(cols, col)
		}
		return cols
	}

	legacyPath := filepath.Join(t.TempDir(), "legacy-alias-index.db")
	legacy, err := OpenTextureWorkspace(legacyPath)
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}
	for _, statement := range []string{
		"DROP INDEX idx_texture_aliases_doc ON texture_document_aliases",
		"ALTER TABLE texture_document_aliases DROP PRIMARY KEY, ADD PRIMARY KEY (owner_id, source_path)",
		"ALTER TABLE texture_document_aliases DROP COLUMN computer_id",
		"CREATE INDEX idx_texture_aliases_doc ON texture_document_aliases(doc_id)",
	} {
		if _, err := legacy.textureHandle().Exec(statement); err != nil {
			_ = legacy.Close()
			t.Fatalf("prepare legacy %q: %v", statement, err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}

	migrated, err := OpenTextureWorkspace(legacyPath)
	if err != nil {
		t.Fatalf("reopen migrated: %v", err)
	}
	t.Cleanup(func() { _ = migrated.Close() })

	fresh, err := OpenTextureWorkspace(filepath.Join(t.TempDir(), "fresh-alias-index.db"))
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	t.Cleanup(func() { _ = fresh.Close() })

	want := []string{"owner_id", "computer_id", "doc_id"}
	if got := indexCols(t, migrated); len(got) != len(want) {
		t.Fatalf("migrated idx_texture_aliases_doc = %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("migrated idx_texture_aliases_doc = %v, want %v", got, want)
			}
		}
	}
	if got := indexCols(t, fresh); len(got) != len(want) {
		t.Fatalf("fresh idx_texture_aliases_doc = %v, want %v", got, want)
	}

	migratedKeys := showKeys(t, migrated)
	freshKeys := showKeys(t, fresh)
	if migratedKeys["doc_id"] != "" || freshKeys["doc_id"] != "" {
		t.Fatalf("doc_id SHOW COLUMNS key migrated=%q fresh=%q, want empty", migratedKeys["doc_id"], freshKeys["doc_id"])
	}
	if migratedKeys["computer_id"] != "PRI" || freshKeys["computer_id"] != "PRI" {
		t.Fatalf("computer_id key migrated=%q fresh=%q, want PRI", migratedKeys["computer_id"], freshKeys["computer_id"])
	}
}
