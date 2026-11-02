package computerversion_test

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/store/splitfixture"

	"database/sql"
)

func openRO(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{ReadOnly: true}, true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func digests(t *testing.T, path string, logical bool) computerversion.StateDigests {
	t.Helper()
	e := computerversion.SQLiteStateExtractor{DB: openRO(t, path), Database: "texture"}
	if logical {
		e.LogicalTables = store.WitnessLogicalTables()
	}
	d, err := e.Digests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// A split store's logical tables digest as the one table each was: every
// table digest equals the old layout's, the body tables are not observed,
// and only the schema hash and content root move.
func TestLogicalTableDigestEqualsOldGenericDigest(t *testing.T) {
	oldPath, newPath, err := splitfixture.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old := digests(t, oldPath, true)
	split := digests(t, newPath, true)

	for _, table := range []string{"og_objects", "choir_events"} {
		if old.Tables[table] == "" || old.Tables[table] != split.Tables[table] {
			t.Fatalf("%s digest old=%q split=%q", table, old.Tables[table], split.Tables[table])
		}
	}
	for table, d := range old.Tables {
		if split.Tables[table] != d {
			t.Errorf("table %s: old %s, split %s", table, d, split.Tables[table])
		}
	}
	if len(split.Tables) != len(old.Tables) {
		t.Errorf("split observes %d tables, old %d", len(split.Tables), len(old.Tables))
	}
	for _, member := range []string{"og_bodies", "choir_event_bodies"} {
		if _, ok := split.Tables[member]; ok {
			t.Errorf("member %s observed as a table", member)
		}
		if _, ok := split.TableSchemas[member]; ok {
			t.Errorf("member %s has a table schema digest", member)
		}
	}
	if split.SchemaHash == old.SchemaHash || split.ContentRoot == old.ContentRoot {
		t.Error("schema hash and content root must change at the cut")
	}
	// Without logical tables the split file digests its header tables
	// physically, which differs: the logical tables are what make it equal.
	generic := digests(t, newPath, false)
	if generic.Tables["og_objects"] == old.Tables["og_objects"] {
		t.Error("generic digest of the split header unexpectedly equals the old digest")
	}
}

// An old-layout file has no member tables, so the logical tables take the
// generic path and the digests are those of an extractor without them.
func TestOldLayoutFileDigestsGenerically(t *testing.T) {
	oldPath, _, err := splitfixture.Build(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	with := digests(t, oldPath, true)
	without := digests(t, oldPath, false)
	if with.ContentRoot != without.ContentRoot || with.SchemaHash != without.SchemaHash {
		t.Fatalf("old layout changed by logical tables: %s vs %s", with.ContentRoot, without.ContentRoot)
	}
	for table, d := range without.Tables {
		if with.Tables[table] != d || with.TableSchemas[table] != without.TableSchemas[table] {
			t.Errorf("table %s differs", table)
		}
	}
}
