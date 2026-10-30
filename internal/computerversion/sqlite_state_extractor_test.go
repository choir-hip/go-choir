package computerversion

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

var testVersion = ComputerVersion{CodeRef: "test-code-ref", ArtifactProgramRef: "test-artifact-ref"}

func sqliteFixture(t *testing.T, ddl string, stmts ...string) *sql.DB {
	t.Helper()
	db := dbxtest.SQLite(t)
	if ddl != "" {
		if _, err := db.Write.Exec(ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	for _, stmt := range stmts {
		if _, err := db.Write.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
	return db.Read
}

func obsValue(set ObservationSet, key string) string {
	for _, o := range set.Observations {
		if o.Key == key {
			return o.Value
		}
	}
	return "<absent>"
}

func extractSet(t *testing.T, e SQLiteStateExtractor) ObservationSet {
	t.Helper()
	set, err := e.Extract(context.Background(), ExtractRequest{Name: "test", Version: testVersion})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

const recordsDDL = `CREATE TABLE records (id INTEGER PRIMARY KEY, status TEXT NOT NULL, prepared_at DATETIME NOT NULL, updated_at DATETIME NOT NULL)`

func TestSQLiteExtractorObservations(t *testing.T) {
	db := sqliteFixture(t, recordsDDL+`; CREATE INDEX records_status ON records(status)`,
		`INSERT INTO records VALUES (1, 'active', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`)
	set := extractSet(t, SQLiteStateExtractor{DB: db, Database: "testdb"})
	for _, key := range []string{"sqlite:testdb:schema", "sqlite:testdb:schema:records", "sqlite:testdb:table:records", "sqlite:testdb:content_root"} {
		if v := obsValue(set, key); v == "<absent>" || !strings.HasPrefix(v, "sha256:") {
			t.Fatalf("%s = %q", key, v)
		}
	}
	// Deterministic: a second extraction of the same state is identical.
	again := extractSet(t, SQLiteStateExtractor{DB: db, Database: "testdb"})
	if obsValue(set, "sqlite:testdb:content_root") != obsValue(again, "sqlite:testdb:content_root") {
		t.Fatal("content root is not deterministic")
	}
}

func TestSQLiteExtractorIgnoredColumnsPreserveSemanticDrift(t *testing.T) {
	a := sqliteFixture(t, recordsDDL, `INSERT INTO records VALUES (1, 'active', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`)
	b := sqliteFixture(t, recordsDDL, `INSERT INTO records VALUES (1, 'active', '2027-01-01 00:00:00', '2027-01-01 00:00:00')`)
	c := sqliteFixture(t, recordsDDL, `INSERT INTO records VALUES (1, 'changed', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`)
	ignore := map[string]map[string]struct{}{"records": {"prepared_at": {}, "updated_at": {}}}
	table := func(db *sql.DB) string {
		return obsValue(extractSet(t, SQLiteStateExtractor{DB: db, Database: "testdb", IgnoredContentColumns: ignore}), "sqlite:testdb:table:records")
	}
	if table(a) != table(b) {
		t.Fatal("ignored timestamp changed the table digest")
	}
	if table(a) == table(c) {
		t.Fatal("a semantic column change did not change the table digest")
	}
	// Without the exclusion, the timestamps are content.
	plain := func(db *sql.DB) string {
		return obsValue(extractSet(t, SQLiteStateExtractor{DB: db, Database: "testdb"}), "sqlite:testdb:table:records")
	}
	if plain(a) == plain(b) {
		t.Fatal("test premise: timestamps should change the digest when not ignored")
	}
}

func TestSQLiteExtractorInsertionOrderDoesNotMatter(t *testing.T) {
	ddl := `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`
	a := sqliteFixture(t, ddl, `INSERT INTO items VALUES (1,'a')`, `INSERT INTO items VALUES (2,'b')`)
	b := sqliteFixture(t, ddl, `INSERT INTO items VALUES (2,'b')`, `INSERT INTO items VALUES (1,'a')`)
	root := func(db *sql.DB) string {
		return obsValue(extractSet(t, SQLiteStateExtractor{DB: db, Database: "d"}), "sqlite:d:content_root")
	}
	if root(a) != root(b) {
		t.Fatal("insertion order changed the content root")
	}
	// A table without a primary key digests its sorted rows.
	noPK := `CREATE TABLE logs (msg TEXT, n INTEGER)`
	c := sqliteFixture(t, noPK, `INSERT INTO logs VALUES ('x',1)`, `INSERT INTO logs VALUES ('y',2)`)
	d := sqliteFixture(t, noPK, `INSERT INTO logs VALUES ('y',2)`, `INSERT INTO logs VALUES ('x',1)`)
	if root(c) != root(d) {
		t.Fatal("a keyless table's digest depends on insertion order")
	}
}

func TestSQLiteExtractorSchemaAndTypesAreContent(t *testing.T) {
	a := sqliteFixture(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)`, `INSERT INTO t VALUES (1,'1')`)
	b := sqliteFixture(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT, extra TEXT)`, `INSERT INTO t VALUES (1,'1',NULL)`)
	c := sqliteFixture(t, `CREATE TABLE t (id INTEGER PRIMARY KEY, v)`, `INSERT INTO t VALUES (1,1)`)
	get := func(db *sql.DB, key string) string {
		return obsValue(extractSet(t, SQLiteStateExtractor{DB: db, Database: "d"}), key)
	}
	if get(a, "sqlite:d:schema") == get(b, "sqlite:d:schema") {
		t.Fatal("a column change did not change the schema hash")
	}
	// Integer 1 and text '1' are different values.
	if get(a, "sqlite:d:table:t") == get(c, "sqlite:d:table:t") {
		t.Fatal("a value of another storage class hashed equal")
	}
}

func TestSQLiteExtractorLeavesIgnoredTablesOut(t *testing.T) {
	ddl := `CREATE TABLE records (id INTEGER PRIMARY KEY, v TEXT NOT NULL); CREATE TABLE derived_index (id INTEGER PRIMARY KEY, k TEXT); CREATE INDEX derived_k ON derived_index(k)`
	filled := sqliteFixture(t, ddl, `INSERT INTO records VALUES (1,'a')`, `INSERT INTO derived_index VALUES (1,'x'), (2,'y')`)
	empty := sqliteFixture(t, ddl, `INSERT INTO records VALUES (1,'a')`)
	ignored := map[string]struct{}{"derived_index": {}}
	a := extractSet(t, SQLiteStateExtractor{DB: filled, Database: "d", IgnoredTables: ignored})
	b := extractSet(t, SQLiteStateExtractor{DB: empty, Database: "d", IgnoredTables: ignored})
	for _, key := range []string{"sqlite:d:table:derived_index", "sqlite:d:schema:derived_index"} {
		if obsValue(a, key) != "<absent>" {
			t.Fatalf("%s is observed although the table is ignored", key)
		}
	}
	if obsValue(a, "sqlite:d:content_root") != obsValue(b, "sqlite:d:content_root") {
		t.Fatal("an ignored table's rows changed the content root")
	}
	un := func(db *sql.DB) string {
		return obsValue(extractSet(t, SQLiteStateExtractor{DB: db, Database: "d"}), "sqlite:d:content_root")
	}
	if un(filled) == un(empty) {
		t.Fatal("test premise: the derived table should change the content root when not ignored")
	}
}

func TestSQLiteExtractorRejectsBadInput(t *testing.T) {
	db := sqliteFixture(t, "")
	ctx := context.Background()
	if _, err := (SQLiteStateExtractor{DB: db, Database: "d"}).Extract(ctx, ExtractRequest{Name: "t"}); err == nil {
		t.Fatal("expected an error for an invalid version")
	}
	if _, err := (SQLiteStateExtractor{Database: "d"}).Extract(ctx, ExtractRequest{Name: "t", Version: testVersion}); err == nil {
		t.Fatal("expected an error for a missing store")
	}
	if _, err := (SQLiteStateExtractor{DB: db}).Extract(ctx, ExtractRequest{Name: "t", Version: testVersion}); err == nil {
		t.Fatal("expected an error for a missing database name")
	}
}

func TestSQLiteExtractorEmptyTableDigest(t *testing.T) {
	db := sqliteFixture(t, `CREATE TABLE keyed (id INTEGER PRIMARY KEY, v TEXT); CREATE TABLE keyless (v TEXT)`)
	set := extractSet(t, SQLiteStateExtractor{DB: db, Database: "d"})
	for _, table := range []string{"keyed", "keyless"} {
		if got, want := obsValue(set, "sqlite:d:table:"+table), EmptyTableDigest(table); got != want {
			t.Fatalf("%s: empty table digest = %s, want %s", table, got, want)
		}
	}
}
