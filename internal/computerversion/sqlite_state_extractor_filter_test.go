package computerversion

import "testing"

const pinsDDL = `CREATE TABLE pins (id INTEGER PRIMARY KEY, kind TEXT NOT NULL, seq INTEGER NOT NULL, pins_json TEXT NOT NULL)`

// A replay that stored JSON null where the live append stored "[]" is the
// same state: the column is a JSON array whose empty form has two spellings
// (problems/replay-of-de-differs-on-the-event-index-only-2026-11-14.md).
func TestSQLiteExtractorReadsJSONNullAsEmptyArrayInListedColumns(t *testing.T) {
	empty := sqliteFixture(t, pinsDDL, `INSERT INTO pins VALUES (1, 'a', 1, '[]'), (2, 'b', 2, '["x"]')`)
	null := sqliteFixture(t, pinsDDL, `INSERT INTO pins VALUES (1, 'a', 1, 'null'), (2, 'b', 2, '["x"]')`)
	list := map[string]map[string]struct{}{"pins": {"pins_json": {}}}
	key := "sqlite:testdb:table:pins"

	if obsValue(extractSet(t, SQLiteStateExtractor{DB: empty, Database: "testdb"}), key) ==
		obsValue(extractSet(t, SQLiteStateExtractor{DB: null, Database: "testdb"}), key) {
		t.Fatal("without the option null and [] must stay distinct (stored witnesses keep their meaning)")
	}
	a := obsValue(extractSet(t, SQLiteStateExtractor{DB: empty, Database: "testdb", EmptyJSONArrayColumns: list}), key)
	b := obsValue(extractSet(t, SQLiteStateExtractor{DB: null, Database: "testdb", EmptyJSONArrayColumns: list}), key)
	if a != b {
		t.Fatalf("null and [] differ under the option: %s vs %s", a, b)
	}
	// A real difference in the same column still shows.
	other := sqliteFixture(t, pinsDDL, `INSERT INTO pins VALUES (1, 'a', 1, '[]'), (2, 'b', 2, '["y"]')`)
	if c := obsValue(extractSet(t, SQLiteStateExtractor{DB: other, Database: "testdb", EmptyJSONArrayColumns: list}), key); c == a {
		t.Fatal("the option hid a real difference")
	}
}

func TestSQLiteExtractorRowFilterKeepsOnlyMatchingRows(t *testing.T) {
	both := sqliteFixture(t, pinsDDL, `INSERT INTO pins VALUES (1, 'keep', 1, '[]'), (2, 'drop', 2, '[]')`)
	one := sqliteFixture(t, pinsDDL, `INSERT INTO pins VALUES (1, 'keep', 1, '[]')`)
	key := "sqlite:testdb:table:pins"
	filters := map[string]string{"pins": `kind <> 'drop'`}
	if obsValue(extractSet(t, SQLiteStateExtractor{DB: both, Database: "testdb"}), key) ==
		obsValue(extractSet(t, SQLiteStateExtractor{DB: one, Database: "testdb"}), key) {
		t.Fatal("unfiltered extractions of different rows must differ")
	}
	a := obsValue(extractSet(t, SQLiteStateExtractor{DB: both, Database: "testdb", RowFilters: filters}), key)
	b := obsValue(extractSet(t, SQLiteStateExtractor{DB: one, Database: "testdb", RowFilters: filters}), key)
	if a != b {
		t.Fatalf("filtered extractions differ: %s vs %s", a, b)
	}
	changed := sqliteFixture(t, pinsDDL, `INSERT INTO pins VALUES (1, 'keep', 9, '[]')`)
	if c := obsValue(extractSet(t, SQLiteStateExtractor{DB: changed, Database: "testdb", RowFilters: filters}), key); c == a {
		t.Fatal("the filter hid a difference in a kept row")
	}
}
