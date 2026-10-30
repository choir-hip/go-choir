package computerversion

import (
	"database/sql"
	"testing"
)

func TestSQLiteExtractorIgnoresGeneratedColumns(t *testing.T) {
	gen := `CREATE TABLE objs (id INTEGER PRIMARY KEY, body TEXT, kind TEXT GENERATED ALWAYS AS (substr(body, 1, 1)) VIRTUAL)`
	a := sqliteFixture(t, gen, `INSERT INTO objs (id, body) VALUES (1, 'xyz')`)
	c := sqliteFixture(t, `CREATE TABLE objs (id INTEGER PRIMARY KEY, body TEXT)`, `INSERT INTO objs (id, body) VALUES (1, 'xyz')`)
	table := func(db *sql.DB) string {
		return obsValue(extractSet(t, SQLiteStateExtractor{DB: db, Database: "d"}), "sqlite:d:table:objs")
	}
	if table(a) != table(c) {
		t.Fatal("a generated column changed the table's row digest")
	}
}
