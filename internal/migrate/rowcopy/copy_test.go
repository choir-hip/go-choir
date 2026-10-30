package rowcopy

// Failure modes, written down before the code (the testing rule: if a system
// is tested in isolation, list every way it can fail first). Each is a test
// below unless marked otherwise.
//
//  1. Column type conversion: Dolt DATETIME/TINYINT/JSON/LONGBLOB/DOUBLE must
//     arrive as SQLite DATETIME/INTEGER/TEXT/BLOB/REAL and as PostgreSQL
//     TIMESTAMPTZ/SMALLINT/JSONB/BYTEA/DOUBLE PRECISION, and verify.
//     (TestCopyDoltToSQLite, TestCopyDoltToPostgres)
//  2. NULLs survive as NULLs; a NULL into a NOT NULL target column becomes
//     the zero value and is counted, not silently lost. (same tests;
//     TestNullIntoNotNullIsCoercedAndCounted)
//  3. Zero dates (0000-00-00, Go's zero time) become NULL, or the zero time
//     for a NOT NULL column. (TestConvertZeroDates)
//  4. Large blobs and long text move intact and in bounded batches.
//     (TestCopyDoltToSQLite: a 6 MiB blob and a 3 MiB text across batches)
//  5. Unicode and embedded quotes are not mangled; NUL bytes in text are
//     refused for PostgreSQL with a message naming the row.
//     (TestCopyDoltToPostgres, TestConvertRejectsNULForPostgres)
//  6. Primary-key ordering: text keys order differently in each engine, so
//     the digest must not depend on order. (mixed-case and accented keys in
//     the fixture; TestDigestIsOrderIndependent)
//  7. Interrupted copy: a cancelled context, or a failing insert midway,
//     returns an error and (guest) leaves no partial state file; a rerun
//     starts clean. (TestInterruptedCopyReturnsError, TestRerunIsClean;
//     the file cleanup is in internal/migrate/guestdolt)
//  8. Digest mismatch: if the target holds something other than what was
//     sent (changed value, deleted row), Run fails and names the table.
//     (TestMismatchIsDetected)
//  9. Schema drift: columns only in one side are dropped from the copy and
//     reported, never an error. (TestCopyDoltToSQLite)
// 10. Unsigned values past int64 are refused for integer columns.
//     (TestConvertUnsignedOverflow)

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
	"github.com/yusefmosiah/go-choir/internal/migrate/doltsrc"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

const doltDDL = `
CREATE TABLE docs (
  id VARCHAR(64) NOT NULL,
  title TEXT NULL,
  body LONGTEXT NULL,
  meta JSON NULL,
  created_at DATETIME NULL,
  n BIGINT NOT NULL DEFAULT 0,
  flag TINYINT NOT NULL DEFAULT 0,
  data LONGBLOB NULL,
  score DOUBLE NULL,
  legacy_only VARCHAR(10) NULL,
  nullable_to_notnull VARCHAR(20) NULL,
  PRIMARY KEY (id)
)`

const doltEdges = `
CREATE TABLE edges (
  a VARCHAR(32) NOT NULL,
  b VARCHAR(32) NOT NULL,
  w INT NOT NULL,
  PRIMARY KEY (a, b)
)`

const doltLog = `CREATE TABLE logs (msg TEXT NOT NULL, at DATETIME NULL)`
const doltGone = `CREATE TABLE gone_table (id INT PRIMARY KEY)`

const sqliteDDL = `
CREATE TABLE docs (
  id TEXT NOT NULL PRIMARY KEY,
  title TEXT,
  body TEXT,
  meta TEXT,
  created_at DATETIME,
  n INTEGER NOT NULL DEFAULT 0,
  flag INTEGER NOT NULL DEFAULT 0,
  data BLOB,
  score REAL,
  new_only TEXT NOT NULL DEFAULT 'x',
  nullable_to_notnull TEXT NOT NULL DEFAULT ''
);
CREATE TABLE edges (a TEXT NOT NULL, b TEXT NOT NULL, w INTEGER NOT NULL, PRIMARY KEY (a, b));
CREATE TABLE logs (msg TEXT NOT NULL, at DATETIME);
CREATE TABLE fresh_table (id INTEGER PRIMARY KEY);`

const pgDDL = `
CREATE TABLE docs (
  id TEXT NOT NULL PRIMARY KEY,
  title TEXT,
  body TEXT,
  meta JSONB,
  created_at TIMESTAMPTZ,
  n BIGINT NOT NULL DEFAULT 0,
  flag SMALLINT NOT NULL DEFAULT 0,
  data BYTEA,
  score DOUBLE PRECISION,
  new_only TEXT NOT NULL DEFAULT 'x',
  nullable_to_notnull TEXT NOT NULL DEFAULT ''
);
CREATE TABLE edges (a TEXT NOT NULL, b TEXT NOT NULL, w BIGINT NOT NULL, PRIMARY KEY (a, b));
CREATE TABLE logs (msg TEXT NOT NULL, at TIMESTAMPTZ);
CREATE TABLE fresh_table (id BIGINT PRIMARY KEY);`

// fixture builds the Dolt source and returns it with its expected shape.
func fixture(t *testing.T) *doltsrc.Handle {
	t.Helper()
	ctx := context.Background()
	h, err := doltsrc.Create(ctx, filepath.Join(t.TempDir(), "ws"), "texture")
	if err != nil {
		t.Fatalf("create dolt: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	for _, ddl := range []string{doltDDL, doltEdges, doltLog, doltGone} {
		if _, err := h.DB.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("ddl: %v", err)
		}
	}
	bigBlob := make([]byte, 6<<20)
	if _, err := rand.Read(bigBlob); err != nil {
		t.Fatal(err)
	}
	bigText := strings.Repeat("longtext line é 日本語\n", 3<<20/26)
	// Out-of-order, mixed-case and accented keys.
	ids := []string{"b", "a", "B", "Z", "é", "ä", "doc-10", "doc-2", "日本", "x'y\"z"}
	for i, id := range ids {
		var meta any = `{"z": 1, "a": [1, 2.50, {"k": "v"}], "u": "é"}`
		var created any = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Add(time.Duration(i) * time.Hour)
		var title any = "Title ünïcode 日本語 " + id
		if i%3 == 0 {
			meta, created, title = nil, nil, nil
		}
		data := []byte{0, 1, 2, 0xff, byte(i)}
		body := "short body"
		var data2 any = data
		if id == "a" {
			data2, body = bigBlob, bigText
		}
		if _, err := h.DB.ExecContext(ctx, `INSERT INTO docs (id, title, body, meta, created_at, n, flag, data, score, legacy_only, nullable_to_notnull) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'legacy', NULL)`,
			id, title, body, meta, created, int64(i)*1000000007, i%2, data2, float64(i)/3); err != nil {
			t.Fatalf("insert %q: %v", id, err)
		}
	}
	for i := 0; i < 1500; i++ {
		if _, err := h.DB.ExecContext(ctx, `INSERT INTO edges (a, b, w) VALUES (?, ?, ?)`, fmt.Sprintf("n%04d", i%97), fmt.Sprintf("m%04d", i), i); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 5; i++ {
		if _, err := h.DB.ExecContext(ctx, `INSERT INTO logs (msg, at) VALUES (?, ?)`, fmt.Sprintf("line %d", i), nil); err != nil {
			t.Fatal(err)
		}
	}
	return h
}

func execAll(t *testing.T, db *sql.DB, script string) {
	t.Helper()
	for _, stmt := range strings.Split(script, ";") {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

func checkReport(t *testing.T, rep *Report) map[string]TableReport {
	t.Helper()
	by := map[string]TableReport{}
	for _, tr := range rep.Tables {
		by[tr.Name] = tr
		if !tr.OK || tr.SourceDigest != tr.TargetDigest || tr.SourceRows != tr.TargetRows {
			t.Errorf("table %s not verified: %+v", tr.Name, tr)
		}
	}
	if got := strings.Join(rep.SourceOnlyTables, ","); got != "gone_table" {
		t.Errorf("source-only tables = %q, want gone_table", got)
	}
	if got := strings.Join(rep.TargetOnlyTables, ","); got != "fresh_table" {
		t.Errorf("target-only tables = %q, want fresh_table", got)
	}
	d := by["docs"]
	if strings.Join(d.SourceOnlyColumns, ",") != "legacy_only" || strings.Join(d.TargetOnlyColumns, ",") != "new_only" {
		t.Errorf("docs column drift = src %v / dst %v", d.SourceOnlyColumns, d.TargetOnlyColumns)
	}
	if d.SourceRows != 10 || by["edges"].SourceRows != 1500 || by["logs"].SourceRows != 5 {
		t.Errorf("row counts = %d/%d/%d", d.SourceRows, by["edges"].SourceRows, by["logs"].SourceRows)
	}
	if d.NullCoerced["nullable_to_notnull"] != 10 {
		t.Errorf("null_coerced = %v, want 10 for nullable_to_notnull", d.NullCoerced)
	}
	return by
}

func TestCopyDoltToSQLite(t *testing.T) {
	src := fixture(t)
	dst := dbxtest.SQLite(t)
	execAll(t, dst.Write, sqliteDDL)
	// Rows already in the target are replaced, not merged.
	if _, err := dst.Write.Exec(`INSERT INTO docs (id) VALUES ('stale')`); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), src.DB, dst.Write, dbx.SQLite, Options{BatchRows: 7, BatchBytes: 1 << 20})
	if err != nil {
		t.Fatalf("Run: %v\n%+v", err, rep)
	}
	checkReport(t, rep)

	var title sql.NullString
	var created sql.NullTime
	var n, flag int64
	if err := dst.Read.QueryRow(`SELECT title, created_at, n, flag FROM docs WHERE id = 'ä'`).Scan(&title, &created, &n, &flag); err != nil {
		t.Fatal(err)
	}
	if !title.Valid || !strings.Contains(title.String, "ünïcode") || !created.Valid || created.Time.Year() != 2026 {
		t.Errorf("row ä = %v %v", title, created)
	}
	var stale int
	_ = dst.Read.QueryRow(`SELECT COUNT(*) FROM docs WHERE id = 'stale'`).Scan(&stale)
	if stale != 0 {
		t.Error("stale target row survived")
	}
	var blob []byte
	if err := dst.Read.QueryRow(`SELECT data FROM docs WHERE id = 'a'`).Scan(&blob); err != nil || len(blob) != 6<<20 {
		t.Errorf("big blob: %d bytes, %v", len(blob), err)
	}
	var newOnly string
	_ = dst.Read.QueryRow(`SELECT new_only FROM docs WHERE id = 'a'`).Scan(&newOnly)
	if newOnly != "x" {
		t.Errorf("target-only column default = %q", newOnly)
	}
	var fresh int
	_ = dst.Read.QueryRow(`SELECT COUNT(*) FROM fresh_table`).Scan(&fresh)
	if fresh != 0 {
		t.Error("target-only table touched")
	}
}

func TestCopyDoltToPostgres(t *testing.T) {
	src := fixture(t)
	dst := dbxtest.Postgres(t)
	execAll(t, dst, pgDDL)
	if _, err := dst.Exec(`INSERT INTO docs (id) VALUES ('stale')`); err != nil {
		t.Fatal(err)
	}
	rep, err := Run(context.Background(), src.DB, dst, dbx.Postgres, Options{})
	if err != nil {
		t.Fatalf("Run: %v\n%+v", err, rep)
	}
	checkReport(t, rep)

	var meta string
	var created sql.NullTime
	var flag int16
	if err := dst.QueryRow(`SELECT meta::text, created_at, flag FROM docs WHERE id = 'a'`).Scan(&meta, &created, &flag); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(meta, `"u": "é"`) || !created.Valid {
		t.Errorf("row a: meta=%s created=%v", meta, created)
	}
	var kinds string
	if err := dst.QueryRow(`SELECT jsonb_typeof(meta) FROM docs WHERE id = 'a'`).Scan(&kinds); err != nil || kinds != "object" {
		t.Errorf("meta is not jsonb: %q %v", kinds, err)
	}
	var blobLen int
	if err := dst.QueryRow(`SELECT length(data) FROM docs WHERE id = 'a'`).Scan(&blobLen); err != nil || blobLen != 6<<20 {
		t.Errorf("big blob: %d bytes, %v", blobLen, err)
	}
	var quoted int
	if err := dst.QueryRow(`SELECT COUNT(*) FROM docs WHERE id = 'x''y"z'`).Scan(&quoted); err != nil || quoted != 1 {
		t.Errorf("quoted key: %d %v", quoted, err)
	}
	var uni string
	if err := dst.QueryRow(`SELECT title FROM docs WHERE id = '日本'`).Scan(&uni); err != nil || !strings.Contains(uni, "日本語 日本") {
		t.Errorf("unicode key: %q %v", uni, err)
	}
	// Rerun: truncate then copy again gives the same state.
	rep2, err := Run(context.Background(), src.DB, dst, dbx.Postgres, Options{})
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	checkReport(t, rep2)
	var count int
	_ = dst.QueryRow(`SELECT COUNT(*) FROM docs`).Scan(&count)
	if count != 10 {
		t.Errorf("rerun left %d docs rows", count)
	}
}

func TestRerunIsClean(t *testing.T) {
	src := fixture(t)
	dst := dbxtest.SQLite(t)
	execAll(t, dst.Write, sqliteDDL)
	for i := 0; i < 2; i++ {
		rep, err := Run(context.Background(), src.DB, dst.Write, dbx.SQLite, Options{Tables: []string{"edges", "logs"}})
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if len(rep.Tables) != 2 {
			t.Fatalf("filter copied %d tables", len(rep.Tables))
		}
	}
	var n int
	_ = dst.Read.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&n)
	if n != 1500 {
		t.Errorf("edges = %d after two runs", n)
	}
	_ = dst.Read.QueryRow(`SELECT COUNT(*) FROM docs`).Scan(&n)
	if n != 0 {
		t.Errorf("unselected table was copied: %d", n)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	src := fixture(t)
	dst := dbxtest.SQLite(t)
	execAll(t, dst.Write, sqliteDDL)
	rep, err := Run(context.Background(), src.DB, dst.Write, dbx.SQLite, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Tables) != 3 || rep.Tables[0].SourceRows == 0 {
		t.Errorf("dry-run report = %+v", rep.Tables)
	}
	var n int
	_ = dst.Read.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&n)
	if n != 0 {
		t.Errorf("dry run wrote %d rows", n)
	}
}

func TestMismatchIsDetected(t *testing.T) {
	cases := []struct {
		name    string
		corrupt string
		want    string
	}{
		{"changed value", `UPDATE edges SET w = w + 1 WHERE b = 'm0007'`, "digest mismatch"},
		{"deleted row", `DELETE FROM edges WHERE b = 'm0007'`, "row count mismatch"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := fixture(t)
			dst := dbxtest.SQLite(t)
			execAll(t, dst.Write, sqliteDDL)
			opts := Options{beforeVerify: func(ctx context.Context, table string) {
				if table != "edges" {
					return
				}
				// The pinned connection is the only one; corrupt through it.
				if _, err := dst.Write.ExecContext(ctx, tc.corrupt); err != nil {
					t.Errorf("corrupt: %v", err)
				}
			}}
			// The runner pins the single SQLite connection, so a hook that
			// writes through the pool would block; run the hook on a second
			// handle to the same file instead.
			side, err := dbx.OpenSQLite(dst.Path, dbx.SQLiteOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer side.Close()
			opts.beforeVerify = func(ctx context.Context, table string) {
				if table == "edges" {
					if _, err := side.Write.ExecContext(ctx, tc.corrupt); err != nil {
						t.Errorf("corrupt: %v", err)
					}
				}
			}
			rep, err := Run(context.Background(), src.DB, dst.Write, dbx.SQLite, opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if rep.OK || len(rep.Failed()) != 1 || rep.Failed()[0] != "edges" {
				t.Errorf("report: ok=%v failed=%v", rep.OK, rep.Failed())
			}
		})
	}
}

func TestMismatchIsDetectedPostgres(t *testing.T) {
	src := fixture(t)
	dst := dbxtest.Postgres(t)
	execAll(t, dst, pgDDL)
	_, err := Run(context.Background(), src.DB, dst, dbx.Postgres, Options{beforeVerify: func(ctx context.Context, table string) {
		if table == "docs" {
			if _, err := dst.ExecContext(ctx, `UPDATE docs SET title = title || '!' WHERE id = 'a'`); err != nil {
				t.Errorf("corrupt: %v", err)
			}
		}
	}})
	if err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("err = %v, want digest mismatch", err)
	}
}

func TestInterruptedCopyReturnsError(t *testing.T) {
	src := fixture(t)
	dst := dbxtest.SQLite(t)
	execAll(t, dst.Write, sqliteDDL)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := Run(ctx, src.DB, dst.Write, dbx.SQLite, Options{beforeVerify: func(_ context.Context, table string) {
		if table == "docs" {
			cancel()
		}
	}})
	if err == nil {
		t.Fatal("cancelled run returned nil")
	}
	// The pool is still usable and a rerun is clean.
	rep, err := Run(context.Background(), src.DB, dst.Write, dbx.SQLite, Options{})
	if err != nil {
		t.Fatalf("rerun after cancel: %v", err)
	}
	checkReport(t, rep)
}

func TestBadTextFailsLoudlyNamingThePrimaryKey(t *testing.T) {
	ctx := context.Background()
	h, err := doltsrc.Create(ctx, filepath.Join(t.TempDir(), "ws"), "texture")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if _, err := h.DB.ExecContext(ctx, `CREATE TABLE notes (id VARCHAR(20) PRIMARY KEY, body LONGTEXT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range [][2]string{{"ok1", "fine"}, {"nul1", "a\x00b"}, {"nul2", "\x00"}, {"ok2", "é"}} {
		if _, err := h.DB.ExecContext(ctx, `INSERT INTO notes VALUES (?, ?)`, row[0], row[1]); err != nil {
			t.Skipf("this Dolt refuses NUL in text, so the case cannot arise: %v", err)
		}
	}
	dst := dbxtest.Postgres(t)
	execAll(t, dst, `CREATE TABLE notes (id TEXT PRIMARY KEY, body TEXT)`)

	rep, err := Run(ctx, h.DB, dst, dbx.Postgres, Options{DryRun: true})
	if err == nil {
		t.Fatal("dry run must fail when rows hold NUL")
	}
	tr := rep.Tables[0]
	if tr.BadTextRows != 2 || tr.BadText["body: NUL byte"] != 2 || len(tr.BadTextSamples) != 2 || !strings.Contains(tr.BadTextSamples[0], "pk=(nul1)") {
		t.Errorf("dry-run report = %+v", tr)
	}
	_, err = Run(ctx, h.DB, dst, dbx.Postgres, Options{})
	if err == nil || !strings.Contains(err.Error(), "pk=(nul1)") || !strings.Contains(err.Error(), "NUL byte") {
		t.Errorf("real run error = %v, want table, column and pk named", err)
	}
	if _, _, err := convert("a\xffb", classText, true, true); err == nil || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Errorf("invalid UTF-8 must be refused: %v", err)
	}
}

func TestNullIntoNotNullIsCoercedAndCounted(t *testing.T) {
	v, res, err := convert(nil, classText, true, false)
	if err != nil || v != "" || !res.coerced {
		t.Errorf("text: %v %+v %v", v, res, err)
	}
	v, res, err = convert(nil, classInt, false, false)
	if err != nil || v != nil || res.coerced {
		t.Errorf("nullable: %v %+v %v", v, res, err)
	}
}

func TestConvertZeroDates(t *testing.T) {
	for _, in := range []any{time.Time{}, "0000-00-00 00:00:00", []byte("0000-00-00"), ""} {
		v, res, err := convert(in, classTime, false, true)
		if err != nil || v != nil || !res.zeroDate {
			t.Errorf("%#v -> %v %+v %v, want NULL zero-date", in, v, res, err)
		}
		v, res, err = convert(in, classTime, true, true)
		if err != nil || v == nil || res.zeroDate {
			t.Errorf("%#v NOT NULL -> %v %+v %v, want zero time", in, v, res, err)
		}
	}
	v, _, err := convert([]byte("2026-03-04 05:06:07.123456"), classTime, false, true)
	if err != nil || v.(time.Time).Nanosecond() != 123456000 {
		t.Errorf("parsed = %v %v", v, err)
	}
}

func TestConvertUnsignedOverflow(t *testing.T) {
	if _, _, err := convert(uint64(1<<63+5), classInt, true, true); err == nil {
		t.Error("uint64 past int64 into an integer column must fail")
	}
	v, _, err := convert(uint64(1<<63+5), classText, true, true)
	if err != nil || v != "9223372036854775813" {
		t.Errorf("into text: %v %v", v, err)
	}
	if _, _, err := convert([]byte("18446744073709551615"), classInt, true, true); err == nil {
		t.Error("textual uint64 past int64 must fail")
	}
}

func TestConvertRejectsNULForPostgres(t *testing.T) {
	if _, _, err := convert("a\x00b", classText, true, true); err == nil {
		t.Error("NUL text must be refused for PostgreSQL")
	}
	if _, _, err := convert("a\x00b", classText, true, false); err != nil {
		t.Errorf("SQLite accepts NUL text: %v", err)
	}
}

func TestDigestIsOrderIndependent(t *testing.T) {
	rows := [][]any{{"b", int64(1)}, {"B", int64(2)}, {"é", int64(3)}, {"a", int64(4)}}
	sum := func(order []int) string {
		var d digest
		for _, i := range order {
			d.beginRow()
			_ = d.col(classText, rows[i][0])
			_ = d.col(classInt, rows[i][1])
			d.endRow()
		}
		return d.Hex()
	}
	if sum([]int{0, 1, 2, 3}) != sum([]int{3, 2, 1, 0}) || sum([]int{0, 1, 2, 3}) != sum([]int{2, 0, 3, 1}) {
		t.Error("digest depends on row order")
	}
	if sum([]int{0, 1, 2}) == sum([]int{0, 1, 2, 3}) {
		t.Error("digest ignores a row")
	}
	// Duplicated rows are not cancelled out (a sum, not an xor).
	var d1, d2 digest
	for i := 0; i < 2; i++ {
		d1.beginRow()
		_ = d1.col(classText, "x")
		d1.endRow()
	}
	d2.beginRow()
	d2.endRow()
	if d1.Hex() == d2.Hex() {
		t.Error("duplicate rows cancel")
	}
}

func TestCanonicalJSONAndTimeAreEngineIndependent(t *testing.T) {
	var a, b bytes.Buffer
	_ = canon(&a, classJSON, `{"z": 1, "a": [2.50, {"k": "v"}]}`)
	_ = canon(&b, classJSON, []byte(`{"a":[2.50,{"k":"v"}],"z":1}`))
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("json canonical form differs by key order or whitespace")
	}
	a.Reset()
	b.Reset()
	t1 := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.FixedZone("x", 3600))
	_ = canon(&a, classTime, t1)
	_ = canon(&b, classTime, "2026-01-02 02:04:05.123456")
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("time canonical form differs between time.Time and text")
	}
}
