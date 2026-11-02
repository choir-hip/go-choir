package dbxplan_test

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/dbx/dbxplan"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

// TestSQLiteSupportsReturningAndNumberedParams pins the two SQLite features
// the split's writers depend on in the modernc driver: RETURNING on an
// upsert (the header statement returns its obj_id, on insert and on
// conflict) and ?NNN parameters bound several times in one statement.
func TestSQLiteSupportsReturningAndNumberedParams(t *testing.T) {
	db := dbxtest.SQLite(t).Write
	if _, err := db.Exec(`CREATE TABLE h (obj_id INTEGER PRIMARY KEY, k TEXT NOT NULL UNIQUE, v TEXT, d TEXT)`); err != nil {
		t.Fatal(err)
	}
	upsert := `INSERT INTO h (k, v, d) VALUES (?1, ?2, ?2 || ?2) ON CONFLICT (k) DO UPDATE SET v = excluded.v, d = excluded.d RETURNING obj_id`
	var first, second int64
	if err := db.QueryRow(upsert, "a", "x").Scan(&first); err != nil {
		t.Fatalf("insert returning: %v", err)
	}
	if err := db.QueryRow(upsert, "a", "y").Scan(&second); err != nil {
		t.Fatalf("upsert returning: %v", err)
	}
	if first != second {
		t.Fatalf("upsert moved the row: %d then %d", first, second)
	}
	var v, d string
	if err := db.QueryRow(`SELECT v, d FROM h WHERE obj_id = ?1`, first).Scan(&v, &d); err != nil {
		t.Fatal(err)
	}
	if v != "y" || d != "yy" {
		t.Fatalf("numbered params: v=%q d=%q", v, d)
	}
	// DO NOTHING on a conflict returns no row.
	err := db.QueryRow(`INSERT INTO h (k, v) VALUES (?1, ?2) ON CONFLICT (k) DO NOTHING RETURNING obj_id`, "a", "z").Scan(&second)
	if err == nil {
		t.Fatalf("DO NOTHING returned a row")
	}
}

func TestPlanAssertions(t *testing.T) {
	db := dbxtest.SQLite(t).Write
	for _, ddl := range []string{
		`CREATE TABLE og_objects (obj_id INTEGER PRIMARY KEY, canonical_id TEXT NOT NULL UNIQUE, object_kind TEXT NOT NULL, updated_at TEXT)`,
		`CREATE INDEX idx_kind_updated ON og_objects(object_kind, updated_at)`,
		`CREATE TABLE og_bodies (obj_id INTEGER PRIMARY KEY, body BLOB)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	dbxplan.HeaderOnly(t, db, "idx_kind_updated", `SELECT canonical_id FROM og_objects WHERE object_kind = ? ORDER BY updated_at DESC LIMIT 5`, "k")
	dbxplan.BodyByRowid(t, db, "b", `SELECT h.canonical_id, b.body FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id WHERE h.object_kind = ?`, "k")
	dbxplan.BodyByRowid(t, db, "og_bodies", `SELECT obj_id, body FROM og_bodies WHERE obj_id IN (?, ?)`, 1, 2)
	dbxplan.NoIndexWalkCensus(t, db, "og_objects", `SELECT canonical_id, updated_at FROM og_objects`)
}
