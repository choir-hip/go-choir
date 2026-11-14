package storetables

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// What can go wrong in a per-table store reading, each covered below:
//   - the header numbers wrong: page_count times page_size is the file, the
//     freelist is the part of it a delete leaves behind;
//   - a table's bytes and rows not the sum of its pages (the reading is the
//     whole answer for a table it finished);
//   - a table's indexes not charged to the table, so the roll-up hides the
//     index share that is most of an object table;
//   - a walk that overruns its budget instead of stopping and saying so;
//   - one huge table using the whole budget and leaving every later one
//     unread, with no row for it at all;
//   - a WITHOUT ROWID table (no rowid extents) failing the whole reading;
//   - a table name that needs quoting breaking the extents query;
//   - two requests reading at once, each paying the full cost;
//   - a plain read walking the file, or a cancelled read replacing a good
//     last reading;
//   - the reading writing anything, or holding the write connection.

func openTestDB(t *testing.T) *dbx.SQLiteDB {
	t.Helper()
	db, err := dbx.OpenSQLite(filepath.Join(t.TempDir(), "state.db"), dbx.SQLiteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func exec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// fill makes a store shaped like the guest's: a header table with indexes, a
// body table keyed by the header's rowid, a WITHOUT ROWID table, and a table
// whose name needs quoting.
func fill(t *testing.T, db *sql.DB, rows int) {
	t.Helper()
	exec(t, db, `CREATE TABLE og_objects (obj_id INTEGER PRIMARY KEY, canonical_id TEXT NOT NULL UNIQUE, kind TEXT NOT NULL, updated_at TEXT NOT NULL)`)
	exec(t, db, `CREATE INDEX idx_og_objects_kind ON og_objects(kind, canonical_id)`)
	exec(t, db, `CREATE INDEX idx_og_objects_updated ON og_objects(updated_at)`)
	exec(t, db, `CREATE TABLE og_bodies (obj_id INTEGER PRIMARY KEY, body BLOB)`)
	exec(t, db, `CREATE TABLE og_key_index (canonical_id TEXT NOT NULL, key_name TEXT NOT NULL, PRIMARY KEY (canonical_id, key_name)) WITHOUT ROWID`)
	exec(t, db, `CREATE TABLE "odd-name ""x""" (a INTEGER)`)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= rows; i++ {
		id := fmt.Sprintf("obj:kind%d:owner:sha256-%064d", i%5, i)
		if _, err := tx.Exec(`INSERT INTO og_objects(obj_id, canonical_id, kind, updated_at) VALUES (?,?,?,?)`, i, id, fmt.Sprintf("k%d", i%5), fmt.Sprintf("2026-11-%02d", i%28+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO og_bodies(obj_id, body) VALUES (?,?)`, i, []byte(strings.Repeat("x", 6000+i%7))); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO og_key_index VALUES (?,?)`, id, "k"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`INSERT INTO "odd-name ""x""" VALUES (1)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func treeNamed(t *testing.T, r *Reading, name string) Tree {
	t.Helper()
	for _, tr := range r.Trees {
		if tr.Name == name {
			return tr
		}
	}
	t.Fatalf("no tree %q in %+v", name, r.Trees)
	return Tree{}
}

func tableNamed(t *testing.T, r *Reading, name string) TableTotal {
	t.Helper()
	for _, tt := range r.Tables {
		if tt.Table == name {
			return tt
		}
	}
	t.Fatalf("no table total %q in %+v", name, r.Tables)
	return TableTotal{}
}

func TestReadingMatchesTheFile(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 400)
	var sc Scanner
	rep := sc.Read(context.Background(), db.Read, DefaultLimits, true)
	r := rep.Reading
	if r == nil || rep.Busy {
		t.Fatalf("no reading: %+v", rep)
	}
	var pageCount, freelist, pageSize int64
	_ = db.Read.QueryRow(`PRAGMA page_count`).Scan(&pageCount)
	_ = db.Read.QueryRow(`PRAGMA freelist_count`).Scan(&freelist)
	_ = db.Read.QueryRow(`PRAGMA page_size`).Scan(&pageSize)
	h := r.Header
	if h.PageCount != pageCount || h.FreelistCount != freelist || h.PageSize != pageSize || h.FileBytes != pageCount*pageSize || h.FreeBytes != freelist*pageSize {
		t.Fatalf("header = %+v, pragmas %d/%d/%d", h, pageCount, freelist, pageSize)
	}
	if r.Truncated {
		t.Fatalf("a small file was cut short: %+v", r)
	}
	bodies := treeNamed(t, r, "og_bodies")
	if !bodies.Complete || bodies.Type != "table" || bodies.Rows != 400 || bodies.OverflowPages == 0 {
		t.Fatalf("og_bodies = %+v", bodies)
	}
	if bodies.RowidMin == nil || bodies.RowidMax == nil || *bodies.RowidMin != 1 || *bodies.RowidMax != 400 {
		t.Fatalf("og_bodies rowid extents = %v %v", bodies.RowidMin, bodies.RowidMax)
	}
	if bodies.Bytes != (bodies.LeafPages+bodies.InternalPages+bodies.OverflowPages)*pageSize {
		t.Fatalf("og_bodies bytes %d is not its pages %d/%d/%d at %d", bodies.Bytes, bodies.LeafPages, bodies.InternalPages, bodies.OverflowPages, pageSize)
	}
	idx := treeNamed(t, r, "idx_og_objects_kind")
	if idx.Type != "index" || idx.Table != "og_objects" || idx.Rows != 400 || !idx.Complete || idx.RowidMax != nil {
		t.Fatalf("idx_og_objects_kind = %+v", idx)
	}
	// The WITHOUT ROWID table has no rowid: read, with no extents.
	ki := treeNamed(t, r, "og_key_index")
	if !ki.Complete || ki.Rows != 400 || ki.RowidMin != nil {
		t.Fatalf("og_key_index = %+v", ki)
	}
	// A name that needs quoting is read.
	odd := treeNamed(t, r, `odd-name "x"`)
	if !odd.Complete || odd.Rows != 1 || odd.RowidMax == nil || *odd.RowidMax != 1 {
		t.Fatalf("odd table = %+v", odd)
	}
	// The roll-up charges every index, including the unique one SQLite
	// names itself, to its table.
	ot := tableNamed(t, r, "og_objects")
	if ot.Indexes != 3 || ot.IndexBytes == 0 || ot.TableBytes == 0 || ot.Bytes != ot.TableBytes+ot.IndexBytes || !ot.Complete {
		t.Fatalf("og_objects total = %+v", ot)
	}
	// Every page of the file is a tree page or a free page (plus the
	// schema table, the pointer map and the lock-byte page, none here).
	var sum int64
	for _, tr := range r.Trees {
		sum += tr.Bytes
	}
	if sum+h.FreeBytes > h.FileBytes || sum < h.FileBytes-h.FreeBytes-2*pageSize {
		t.Fatalf("trees %d + free %d vs file %d", sum, h.FreeBytes, h.FileBytes)
	}
	if r.Trees[0].Bytes < r.Trees[len(r.Trees)-1].Bytes {
		t.Fatalf("trees are not largest first: %+v", r.Trees)
	}
}

func TestFreelistShowsWhatADeleteLeavesBehind(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 600)
	exec(t, db.Write, `DELETE FROM og_bodies`)
	var sc Scanner
	r := sc.Read(context.Background(), db.Read, DefaultLimits, true).Reading
	if r.Header.FreelistCount == 0 || r.Header.FreeBytes != r.Header.FreelistCount*r.Header.PageSize {
		t.Fatalf("the freed pages are not in the header: %+v", r.Header)
	}
	if b := treeNamed(t, r, "og_bodies"); b.Rows != 0 {
		t.Fatalf("og_bodies = %+v", b)
	}
}

func TestABudgetStopsTheReadingAndSaysSo(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 3000)
	var sc Scanner
	start := time.Now()
	r := sc.Read(context.Background(), db.Read, Limits{Budget: time.Nanosecond}, true).Reading
	if time.Since(start) > 5*time.Second {
		t.Fatalf("a nanosecond budget took %s", time.Since(start))
	}
	if r == nil || !r.Truncated {
		t.Fatalf("a spent budget did not say truncated: %+v", r)
	}
	if r.Header.PageCount == 0 {
		t.Fatalf("the header is free and is always read: %+v", r.Header)
	}
	// Every tree has a row, read or not, so a reader sees what was left.
	if len(r.Trees) < 5 {
		t.Fatalf("trees = %d", len(r.Trees))
	}
	for _, tr := range r.Trees {
		if !tr.Complete && tr.Rows > 0 && !tr.Partial {
			t.Fatalf("an unfinished tree with rows does not say partial: %+v", tr)
		}
	}
}

func TestOneTreeDoesNotTakeTheWholeBudget(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 4000)
	var sc Scanner
	// A budget that finishes the small trees and cannot finish the biggest
	// at its share: the small ones are still read.
	sc.testHook = func(name string, d time.Duration) time.Duration {
		if name == "og_bodies" {
			return time.Nanosecond
		}
		return d
	}
	r := sc.Read(context.Background(), db.Read, Limits{Budget: time.Minute}, true).Reading
	b := treeNamed(t, r, "og_bodies")
	if b.Complete || !r.Truncated {
		t.Fatalf("og_bodies was not cut at its share: %+v", b)
	}
	for _, name := range []string{"og_objects", "og_key_index", "idx_og_objects_kind", "idx_og_objects_updated"} {
		if tr := treeNamed(t, r, name); !tr.Complete {
			t.Fatalf("%s starved by the cut tree: %+v", name, tr)
		}
	}
}

func TestOneReadingAtATimeAndTheLastIsKept(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 200)
	var sc Scanner

	plain := sc.Read(context.Background(), db.Read, DefaultLimits, false)
	if plain.Reading != nil || plain.Busy {
		t.Fatalf("a plain read with no reading yet walked or was busy: %+v", plain)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	sc.testHook = func(name string, d time.Duration) time.Duration {
		once.Do(func() { close(entered) })
		<-release
		return d
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var first Report
	go func() {
		defer wg.Done()
		first = sc.Read(context.Background(), db.Read, DefaultLimits, true)
	}()
	<-entered
	second := sc.Read(context.Background(), db.Read, DefaultLimits, true)
	if !second.Busy || second.Reading != nil {
		t.Fatalf("a second reading ran beside the first: %+v", second)
	}
	close(release)
	wg.Wait()
	if first.Reading == nil || first.Busy {
		t.Fatalf("first = %+v", first)
	}
	again := sc.Read(context.Background(), db.Read, DefaultLimits, false)
	if again.Reading == nil || !again.Reading.ReadAt.Equal(first.Reading.ReadAt) {
		t.Fatalf("a plain read after a reading does not show it: %+v", again)
	}

	// A reading cut by its request's cancellation does not replace it.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sc.testHook = nil
	cut := sc.Read(ctx, db.Read, DefaultLimits, true)
	if cut.Reading == nil || !cut.Reading.Truncated {
		t.Fatalf("a cancelled reading = %+v", cut.Reading)
	}
	if kept := sc.Read(context.Background(), db.Read, DefaultLimits, false); kept.Reading == nil || !kept.Reading.ReadAt.Equal(first.Reading.ReadAt) || kept.Reading.Truncated {
		t.Fatalf("a cancelled reading replaced the good one: %+v", kept.Reading)
	}
}

func TestTheReadingWritesNothing(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 100)
	var before, after int64
	_ = db.Write.QueryRow(`PRAGMA data_version`).Scan(&before)
	var sc Scanner
	sc.Read(context.Background(), db.Read, DefaultLimits, true)
	_ = db.Write.QueryRow(`PRAGMA data_version`).Scan(&after)
	if before != after {
		t.Fatalf("data_version moved %d -> %d: the reading changed the file", before, after)
	}
	// The write connection is free throughout: a write does not wait on it.
	exec(t, db.Write, `INSERT INTO og_bodies(obj_id, body) VALUES (?, ?)`, 9999, []byte("x"))
}

func TestBudgetIsBounded(t *testing.T) {
	if got := clampBudget(0); got != DefaultLimits.Budget {
		t.Fatalf("zero budget = %s", got)
	}
	if got := clampBudget(time.Hour); got != MaxBudget {
		t.Fatalf("an hour = %s, want the cap %s", got, MaxBudget)
	}
	if got := clampBudget(7 * time.Second); got != 7*time.Second {
		t.Fatalf("7s = %s", got)
	}
	if MaxBudget >= 120*time.Second {
		t.Fatalf("the cap %s reaches the server's 120 s write timeout", MaxBudget)
	}
}

func TestANilDatabaseIsAnErrorNotAPanic(t *testing.T) {
	var sc Scanner
	r := sc.Read(context.Background(), nil, DefaultLimits, true).Reading
	if r == nil || r.Header.Error == "" || !r.Truncated {
		t.Fatalf("a nil database = %+v", r)
	}
}

func TestHeaderKeepsTheFirstErrorInAFixedOrder(t *testing.T) {
	db := openTestDB(t)
	_ = db.Read.Close()
	for i := 0; i < 20; i++ {
		h := readHeader(context.Background(), db.Read)
		if !strings.HasPrefix(h.Error, "PRAGMA page_size") {
			t.Fatalf("header error = %q, want the first pragma's", h.Error)
		}
	}
}

func TestExtentsAreSkippedOncePastTheDeadline(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 50)
	var sc Scanner
	sc.testHook = func(name string, d time.Duration) time.Duration {
		if name == "og_bodies" {
			time.Sleep(400 * time.Millisecond) // past the whole budget
			return time.Second
		}
		return d
	}
	r := sc.Read(context.Background(), db.Read, Limits{Budget: 200 * time.Millisecond}, true).Reading
	if b := treeNamed(t, r, "og_bodies"); b.RowidMax != nil {
		t.Fatalf("extents were read after the deadline: %+v", b)
	}
	if !r.Truncated {
		t.Fatalf("later trees were not reached but the reading is not truncated: %+v", r)
	}
}

func TestOneTreesSingleReadIsCapped(t *testing.T) {
	db := openTestDB(t)
	fill(t, db.Write, 50)
	old := maxTreeRead
	maxTreeRead = 3 * time.Second
	defer func() { maxTreeRead = old }()
	var sc Scanner
	var biggest time.Duration
	sc.testHook = func(name string, d time.Duration) time.Duration {
		biggest = max(biggest, d)
		return d
	}
	sc.Read(context.Background(), db.Read, Limits{Budget: time.Minute}, true)
	if biggest == 0 || biggest > 3*time.Second {
		t.Fatalf("the largest single-read share was %s, want at most the 3s cap", biggest)
	}
	if MaxTreeRead != 15*time.Second {
		t.Fatalf("MaxTreeRead = %s", MaxTreeRead)
	}
}

func TestSchemaDetectionToleratesWhitespace(t *testing.T) {
	db := openTestDB(t)
	exec(t, db.Write, "CREATE TABLE w (a TEXT PRIMARY KEY)  WITHOUT\n\tROWID")
	exec(t, db.Write, `CREATE TABLE r (a INTEGER)`)
	list, err := readSchema(context.Background(), db.Read)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, st := range list {
		got[st.name] = st.rowid
	}
	if got["w"] || !got["r"] {
		t.Fatalf("rowid detection = %v", got)
	}
}
