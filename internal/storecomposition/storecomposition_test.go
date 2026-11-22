package storecomposition

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// What can go wrong in a composition reading, each covered below:
//   - counts or bytes by kind wrong, or bodies not joined to their headers;
//   - versions: superseded and tombstoned objects counted as current;
//   - duplicate content hashes missed, or the first copy counted as one;
//   - run memory not split by kind and age;
//   - the report carrying text from a row (a body, title, url, key, a kind
//     that is not a kind);
//   - the reading blocking the writer, or pinning the WAL so checkpoints
//     cannot finish;
//   - no deadline, or a deadline that loses what was read;
//   - a restart resuming, or showing a dead reading as running;
//   - two readings at once;
//   - a table with no rowid, or a table that needs quoting, failing it.

func openDB(t *testing.T) *dbx.SQLiteDB {
	t.Helper()
	db, err := dbx.OpenSQLite(filepath.Join(t.TempDir(), "state.db"), dbx.SQLiteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, q := range []string{
		objectgraph.ObjectHeaderTableDDL(), objectgraph.ObjectBodyTableDDL,
		`CREATE TABLE run_memory_entries (entry_id TEXT PRIMARY KEY, loop_id TEXT NOT NULL, kind TEXT NOT NULL, message_json TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL)`,
		`CREATE TABLE choir_events (ev_id INTEGER PRIMARY KEY, canonical_id TEXT NOT NULL, object_kind TEXT NOT NULL, owner_id TEXT NOT NULL, computer_id TEXT NOT NULL DEFAULT '', version_id TEXT NOT NULL DEFAULT '', content_hash TEXT NOT NULL, metadata TEXT NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, tombstone INTEGER NOT NULL DEFAULT 0, superseded_by TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE choir_event_bodies (ev_id INTEGER PRIMARY KEY, body BLOB NOT NULL)`,
		`CREATE TABLE og_key_index (canonical_id TEXT NOT NULL, key_name TEXT NOT NULL, PRIMARY KEY (canonical_id, key_name)) WITHOUT ROWID`,
		`CREATE TABLE "odd ""name" (a TEXT)`,
	} {
		if _, err := db.Write.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	return db
}

type obj struct {
	id, kind, hash string
	body           []byte
	tomb           bool
	superseded     string
}

func put(t *testing.T, db *sql.DB, o obj) {
	t.Helper()
	res, err := db.Exec(`INSERT INTO og_objects(canonical_id, object_kind, owner_id, content_hash, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?,?,?,?,?,?,?,?,?)`,
		o.id, o.kind, "o", o.hash, "{}", "2026-11-01 00:00:00", "2026-11-01 00:00:00", o.tomb, o.superseded)
	if err != nil {
		t.Fatal(err)
	}
	if o.body != nil {
		n, _ := res.LastInsertId()
		if _, err := db.Exec(`INSERT INTO og_bodies(obj_id, body) VALUES (?,?)`, n, o.body); err != nil {
			t.Fatal(err)
		}
	}
}

func readAll(t *testing.T, s *Scanner, db *sql.DB) Report {
	t.Helper()
	if _, started := s.Start(dbFn(db)); !started {
		t.Fatal("not started")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	return *s.Status()
}

func kindOf(t *testing.T, o *Objects, kind string) KindStat {
	t.Helper()
	for _, k := range o.Kinds {
		if k.Kind == kind {
			return k
		}
	}
	t.Fatalf("no kind %q in %+v", kind, o.Kinds)
	return KindStat{}
}

func TestCountsBytesVersionsAndDuplicatesByKind(t *testing.T) {
	db := openDB(t)
	b := func(n int) []byte { return []byte(strings.Repeat("x", n)) }
	put(t, db.Write, obj{id: "a1", kind: "choir.web_capture", hash: "h1", body: b(100)})
	put(t, db.Write, obj{id: "a2", kind: "choir.web_capture", hash: "h2", body: b(300)})
	put(t, db.Write, obj{id: "a3", kind: "choir.web_capture", hash: "h2", body: b(300)}) // duplicate of a2
	put(t, db.Write, obj{id: "t1", kind: "texture.doc", hash: "h3", body: b(1000), superseded: "t2"})
	put(t, db.Write, obj{id: "t2", kind: "texture.doc", hash: "h4", body: b(2000)})
	put(t, db.Write, obj{id: "t3", kind: "texture.doc", hash: "h5", tomb: true}) // no body row
	if _, err := db.Write.Exec(`INSERT INTO og_bodies(obj_id, body) VALUES (9999, ?)`, b(7)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := db.Write.Exec(`INSERT INTO og_key_index VALUES (?,?)`, fmt.Sprint("c", i), "k"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Write.Exec(`INSERT INTO "odd ""name" VALUES ('abcd')`); err != nil {
		t.Fatal(err)
	}
	s := Open("", Config{BatchRows: 2, MinYield: time.Microsecond}) // batches smaller than the table
	rep := readAll(t, s, db.Read)
	if rep.State != StateDone || rep.Incomplete || rep.Objects == nil || !rep.Objects.Complete {
		t.Fatalf("report = %+v", rep)
	}
	cap := kindOf(t, rep.Objects, "choir.web_capture")
	if cap.Objects != 3 || cap.Current != 3 || cap.BodyCount != 3 || cap.BodyBytes != 700 || cap.BodyMin != 100 || cap.BodyMax != 300 {
		t.Fatalf("capture = %+v", cap)
	}
	if want := int64(300); cap.DuplicateObjects != 1 || cap.DuplicateBytes != want {
		t.Fatalf("duplicates = %d objects, %d bytes, want 1 and %d", cap.DuplicateObjects, cap.DuplicateBytes, want)
	}
	if cap.BodyMedianAtMost < 300 || cap.BodyMedianAtMost > 511 {
		t.Fatalf("median bound = %d", cap.BodyMedianAtMost)
	}
	tex := kindOf(t, rep.Objects, "texture.doc")
	if tex.Objects != 3 || tex.Current != 1 || tex.Superseded != 1 || tex.Tombstoned != 1 || tex.BodyCount != 2 || tex.BodyBytes != 3000 {
		t.Fatalf("texture = %+v", tex)
	}
	if tex.SupersededBytes != hdr(t, db.Read, "t1")+1000 || tex.CurrentBytes != hdr(t, db.Read, "t2")+2000 {
		t.Fatalf("texture bytes = %+v", tex)
	}
	if rep.Objects.Total.Objects != 6 || rep.Objects.Total.DuplicateObjects != 1 {
		t.Fatalf("total = %+v", rep.Objects.Total)
	}
	got := map[string]TableStat{}
	for _, ts := range rep.Tables {
		got[ts.Table] = ts
	}
	if g := got["og_bodies"]; g.Rows != 6 || g.OrphanRows != 1 || !g.Complete {
		t.Fatalf("og_bodies = %+v", g)
	}
	if g := got["og_objects"]; g.Rows != 6 || !g.Complete {
		t.Fatalf("og_objects = %+v", g)
	}
	if g := got["og_key_index"]; g.Rows != 5 || g.PayloadBytes != 5*(2+1) || !g.Complete {
		t.Fatalf("og_key_index (WITHOUT ROWID) = %+v", g)
	}
	if g := got[`odd "name`]; g.Rows != 1 || g.PayloadBytes != 4 {
		t.Fatalf("quoted table = %+v", g)
	}
	if rep.Progress.TablesDone != rep.Progress.TablesTotal || rep.Progress.RowsWalked == 0 || rep.PageCount == 0 {
		t.Fatalf("progress = %+v", rep.Progress)
	}
}

func hdr(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	rs, err := db.Query(`SELECT name FROM pragma_table_xinfo('og_objects') WHERE hidden IN (0, 3)`)
	if err != nil {
		t.Fatal(err)
	}
	var cols []string
	for rs.Next() {
		var c string
		_ = rs.Scan(&c)
		cols = append(cols, c)
	}
	_ = rs.Close()
	var n int64
	if err := db.QueryRow(`SELECT `+sumExpr(cols, "")+` FROM og_objects WHERE canonical_id = ?`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunMemoryByKindAndAge(t *testing.T) {
	db := openDB(t)
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	add := func(id, kind string, at time.Time, msg string) {
		if _, err := db.Write.Exec(`INSERT INTO run_memory_entries(entry_id, loop_id, kind, message_json, created_at) VALUES (?,?,?,?,?)`, id, "L", kind, msg, at.Format("2006-01-02 15:04:05")); err != nil {
			t.Fatal(err)
		}
	}
	add("e1", "message", now.Add(-2*time.Hour), strings.Repeat("m", 100))
	add("e2", "message", now.AddDate(0, 0, -3), strings.Repeat("m", 200))
	add("e3", "compaction", now.AddDate(0, 0, -30), strings.Repeat("m", 400))
	add("e4", "message", now.AddDate(0, 0, -30), "")
	s := Open("", Config{BatchRows: 3, MinYield: time.Microsecond, Now: func() time.Time { return now }})
	rep := readAll(t, s, db.Read)
	if rep.RunMemory == nil || !rep.RunMemory.Complete {
		t.Fatalf("run memory = %+v", rep.RunMemory)
	}
	rowsBytes := func(cs []RMCell, l string) (int64, int64) {
		for _, c := range cs {
			if c.Label == l {
				return c.Rows, c.Bytes
			}
		}
		return -1, -1
	}
	if r, _ := rowsBytes(rep.RunMemory.ByKind, "message"); r != 3 {
		t.Fatalf("by kind = %+v", rep.RunMemory.ByKind)
	}
	if r, _ := rowsBytes(rep.RunMemory.ByAge, AgeDay); r != 1 {
		t.Fatalf("by age = %+v", rep.RunMemory.ByAge)
	}
	if r, _ := rowsBytes(rep.RunMemory.ByAge, AgeWeek); r != 1 {
		t.Fatalf("by age = %+v", rep.RunMemory.ByAge)
	}
	if r, b := rowsBytes(rep.RunMemory.ByAge, AgeOlder); r != 2 || b < 400 {
		t.Fatalf("by age = %+v", rep.RunMemory.ByAge)
	}
}

// The report type has no free-text field: every string field is on this
// list, each a table name, a kind or a fixed word.
func TestReportTypeCannotCarryContent(t *testing.T) {
	allowed := map[string]bool{
		"Report.State": true, "Report.Reason": true, "Report.Scope": true, "Report.ReadingNote": true, "Progress.Phase": true,
		"TableStat.Table": true, "TableStat.Failure": true, "KindStat.Kind": true,
		"Objects.HeaderTable": true, "Objects.BodyTable": true, "Objects.DigestNote": true,
		"RMCell.Label": true, "RMKindAg.Kind": true, "RMKindAg.Age": true,
	}
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(ty reflect.Type) {
		for ty.Kind() == reflect.Pointer || ty.Kind() == reflect.Slice {
			ty = ty.Elem()
		}
		if ty.Kind() != reflect.Struct || ty == reflect.TypeOf(time.Time{}) || seen[ty] {
			return
		}
		seen[ty] = true
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			ft := f.Type
			for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice {
				ft = ft.Elem()
			}
			switch ft.Kind() {
			case reflect.String:
				if !allowed[ty.Name()+"."+f.Name] {
					t.Errorf("%s.%s is a string field not on the allowed list", ty.Name(), f.Name)
				}
			case reflect.Map, reflect.Interface, reflect.Array:
				t.Errorf("%s.%s is a %s: it could carry content", ty.Name(), f.Name, ft.Kind())
			case reflect.Struct:
				walk(ft)
			}
		}
	}
	walk(reflect.TypeOf(Report{}))
}

func TestReportCarriesNoRowText(t *testing.T) {
	db := openDB(t)
	secrets := []string{"SECRET-BODY-TEXT", "https://secret.example/page", "SECRET TITLE", "secret-key-name"}
	put(t, db.Write, obj{id: "https://secret.example/page", kind: "SECRET TITLE with spaces", hash: "secret-key-name", body: []byte("SECRET-BODY-TEXT")})
	put(t, db.Write, obj{id: "x", kind: "choir.ok", hash: "h", body: []byte("SECRET-BODY-TEXT")})
	if _, err := db.Write.Exec(`INSERT INTO run_memory_entries(entry_id, loop_id, kind, message_json, created_at) VALUES ('SECRET-BODY-TEXT','L','SECRET TITLE','SECRET-BODY-TEXT','2026-11-20 00:00:00')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO og_key_index VALUES ('https://secret.example/page','secret-key-name')`); err != nil {
		t.Fatal(err)
	}
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	data, _ := json.Marshal(rep)
	for _, s := range secrets {
		if strings.Contains(string(data), s) {
			t.Fatalf("report carries %q: %s", s, data)
		}
	}
	if kindOf(t, rep.Objects, OtherKind).Objects != 1 || kindOf(t, rep.Objects, "choir.ok").Objects != 1 {
		t.Fatalf("kinds = %+v", rep.Objects.Kinds)
	}
}

// A writer is never blocked by the reading, and the WAL is not pinned: a
// passive checkpoint run between batches completes (everything in the WAL is
// checkpointed), and a truncating one at the end leaves the WAL empty.
func TestWriterNotBlockedAndWALCheckpoints(t *testing.T) {
	db := openDB(t)
	tx, _ := db.Write.Begin()
	for i := 0; i < 3000; i++ {
		if _, err := tx.Exec(`INSERT INTO og_objects(canonical_id, object_kind, owner_id, content_hash, metadata, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
			fmt.Sprint("s", i), "k", "o", fmt.Sprint("h", i), "{}", "2026-11-01", "2026-11-01"); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	s := Open("", Config{BatchRows: 50, MinYield: 3 * time.Millisecond})
	if _, started := s.Start(dbFn(db.Read)); !started {
		t.Fatal("not started")
	}
	var maxWrite time.Duration
	complete, writes := 0, 0
	for s.Status().State == StateRunning {
		t0 := time.Now()
		if _, err := db.Write.Exec(`INSERT INTO og_objects(canonical_id, object_kind, owner_id, content_hash, metadata, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
			fmt.Sprint("w", writes), "k", "o", "hw", "{}", "2026-11-01", "2026-11-01"); err != nil {
			t.Fatal(err)
		}
		writes++
		maxWrite = max(maxWrite, time.Since(t0))
		var busy, logFrames, ckpt int
		if err := db.Write.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &ckpt); err != nil {
			t.Fatal(err)
		}
		if busy == 0 && logFrames == ckpt {
			complete++
		}
		time.Sleep(time.Millisecond)
	}
	if rep := s.Status(); rep.State != StateDone {
		t.Fatalf("report = %+v", rep)
	}
	if writes < 3 || complete < writes/2 {
		t.Fatalf("writes %d, checkpoints that completed %d", writes, complete)
	}
	if maxWrite > 2*time.Second {
		t.Fatalf("a write waited %v", maxWrite)
	}
	var busy, logFrames, ckpt int
	if err := db.Write.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &ckpt); err != nil || busy != 0 {
		t.Fatalf("truncate checkpoint: busy=%d err=%v", busy, err)
	}
	if fi, err := os.Stat(db.Path + "-wal"); err == nil && fi.Size() != 0 {
		t.Fatalf("wal is %d bytes after a truncating checkpoint", fi.Size())
	}
}

func TestDeadlineStopsAndReportsWhatItHas(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 400; i++ {
		put(t, db.Write, obj{id: fmt.Sprint("d", i), kind: "k", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	// 400 batches of at least 5 ms (2 s) against a 150 ms deadline.
	s := Open("", Config{BatchRows: 1, MinYield: 5 * time.Millisecond, Deadline: 150 * time.Millisecond})
	rep := readAll(t, s, db.Read)
	if rep.State != StateIncomplete || !rep.Incomplete || rep.Reason != ReasonDeadline || rep.FinishedAt == nil {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Objects == nil || rep.Objects.Complete || rep.Objects.Total.Objects == 0 || rep.Objects.Total.Objects >= 400 {
		t.Fatalf("partial objects = %+v", rep.Objects)
	}
}

func TestRestartMarksInterruptedAndDoesNotResume(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 400; i++ {
		put(t, db.Write, obj{id: fmt.Sprint("r", i), kind: "k", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	file := filepath.Join(t.TempDir(), "store-composition.json")
	a := Open(file, Config{BatchRows: 1, MinYield: 20 * time.Millisecond, PersistEvery: time.Hour})
	if _, started := a.Start(dbFn(db.Read)); !started {
		t.Fatal("not started")
	}
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), `"running"`) {
		t.Fatalf("file during the reading: %s %v", data, err)
	}
	// The process dies here: a second scanner opens the same file.
	b := Open(file, Config{})
	st := b.Status()
	if st == nil || st.State != StateInterrupted || !st.Incomplete || st.Reason != ReasonRestart {
		t.Fatalf("after restart = %+v", st)
	}
	time.Sleep(100 * time.Millisecond)
	if b.Status().State != StateInterrupted {
		t.Fatal("the restarted scanner resumed or changed state on its own")
	}
	data, _ = os.ReadFile(file)
	if !strings.Contains(string(data), `"interrupted"`) {
		t.Fatalf("file after restart: %s", data)
	}
	a.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = a.Wait(ctx)
}

func TestFinishedReportSurvivesInFile(t *testing.T) {
	db := openDB(t)
	put(t, db.Write, obj{id: "f1", kind: "k", hash: "h", body: []byte("b")})
	file := filepath.Join(t.TempDir(), "store-composition.json")
	rep := readAll(t, Open(file, Config{MinYield: time.Microsecond}), db.Read)
	again := Open(file, Config{}).Status()
	if again == nil || again.State != StateDone || again.Objects == nil || again.Objects.Total.Objects != rep.Objects.Total.Objects {
		t.Fatalf("reloaded = %+v", again)
	}
}

func TestOneAtATime(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 100; i++ {
		put(t, db.Write, obj{id: fmt.Sprint("o", i), kind: "k", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	s := Open("", Config{BatchRows: 1, MinYield: 10 * time.Millisecond})
	first, started := s.Start(dbFn(db.Read))
	if !started {
		t.Fatal("first start refused")
	}
	second, started2 := s.Start(dbFn(db.Read))
	if started2 || second.State != StateRunning || !second.StartedAt.Equal(first.StartedAt) {
		t.Fatalf("second start: started=%v %+v", started2, second)
	}
	s.Cancel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.State != StateInterrupted || st.Reason != ReasonCancelled {
		t.Fatalf("after cancel = %+v", st)
	}
	if _, started := s.Start(dbFn(db.Read)); !started {
		t.Fatal("a new reading after the last ended must start")
	}
	s.Cancel()
	_ = s.Wait(ctx)
}

// spyDriver wraps the sqlite driver, recording every statement and failing the
// ones whose text contains failOn.
type spyDriver struct {
	inner  driver.Driver
	mu     sync.Mutex
	seen   []string
	failOn string
}

func (d *spyDriver) Open(name string) (driver.Conn, error) {
	c, err := d.inner.Open(name)
	if err != nil {
		return nil, err
	}
	return &spyConn{Conn: c, d: d}, nil
}

type spyConn struct {
	driver.Conn
	d *spyDriver
}

func (c *spyConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.d.mu.Lock()
	c.d.seen = append(c.d.seen, q)
	fail := c.d.failOn != "" && strings.Contains(q, c.d.failOn)
	c.d.mu.Unlock()
	if fail {
		return nil, fmt.Errorf("injected failure")
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, args)
}

func (c *spyConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, args)
}

func (c *spyConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
}

func (c *spyConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, q)
}

var spyN int

// spyOpen opens the store file read-only through a recording driver.
func spyOpen(t *testing.T, path string) (*sql.DB, *spyDriver) {
	t.Helper()
	probe, _ := sql.Open("sqlite", "")
	inner := probe.Driver()
	_ = probe.Close()
	spyN++
	name := fmt.Sprintf("sqlite-spy-%d", spyN)
	d := &spyDriver{inner: inner}
	sql.Register(name, d)
	db, err := sql.Open(name, dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, d
}

// Every statement that touches a data table is bounded by a LIMIT: no single
// statement is a read snapshot that spans the table, whatever the fixture's
// size. Orphan bodies (the part no header range bounds) are walked in batches.
func TestEveryDataStatementIsBounded(t *testing.T) {
	db := openDB(t)
	tx, _ := db.Write.Begin()
	for i := 0; i < 3000; i++ {
		if _, err := tx.Exec(`INSERT INTO og_bodies(obj_id, body) VALUES (?, ?)`, 1000+i, []byte("orph")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	put(t, db.Write, obj{id: "h1", kind: "k", hash: "a", body: []byte("b")})
	spy, d := spyOpen(t, db.Path)
	rep := readAll(t, Open("", Config{BatchRows: 100, MinYield: time.Microsecond}), spy)
	if rep.State != StateDone {
		t.Fatalf("report = %+v", rep)
	}
	data := 0
	for _, q := range d.seen {
		if strings.HasPrefix(q, "PRAGMA") || strings.Contains(q, "pragma_table") || strings.Contains(q, "pragma_index") {
			continue
		}
		data++
		if !strings.Contains(q, "LIMIT") {
			t.Errorf("an unbounded statement: %s", q)
		}
	}
	if data < 30 {
		t.Fatalf("only %d bounded data statements for 3000 orphan rows at 100 per batch", data)
	}
	var bodies TableStat
	for _, ts := range rep.Tables {
		if ts.Table == "og_bodies" {
			bodies = ts
		}
	}
	if bodies.Rows != 3001 || bodies.OrphanRows != 3000 || bodies.OrphanBytes != 3000*(4+8) || bodies.PayloadBytes != 3000*(4+8)+(1+8) {
		t.Fatalf("og_bodies = %+v", bodies)
	}
	if rep.Objects.OrphanBodyRows != 3000 || rep.Objects.OrphanBodyBytes != 3000*(4+8) {
		t.Fatalf("orphan line = %+v", rep.Objects)
	}
}

// A hash seen in og_objects is not a duplicate in choir_events: the seen-set
// is per pair.
func TestDuplicateSetIsPerPair(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 3; i++ {
		put(t, db.Write, obj{id: fmt.Sprint("o", i), kind: "k", hash: fmt.Sprint("hash", i), body: []byte("b")})
		res, err := db.Write.Exec(`INSERT INTO choir_events(canonical_id, object_kind, owner_id, content_hash, metadata, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
			fmt.Sprint("e", i), "choir.event", "o", fmt.Sprint("hash", i), "{}", "2026-11-01", "2026-11-01")
		if err != nil {
			t.Fatal(err)
		}
		n, _ := res.LastInsertId()
		if _, err := db.Write.Exec(`INSERT INTO choir_event_bodies VALUES (?,?)`, n, []byte("b")); err != nil {
			t.Fatal(err)
		}
	}
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	if rep.Events == nil || rep.Events.Total.Objects != 3 || rep.Events.Total.DuplicateObjects != 0 || rep.Objects.Total.DuplicateObjects != 0 {
		t.Fatalf("events = %+v objects = %+v", rep.Events, rep.Objects.Total)
	}
}

// A running reading shows what it has so far, and so does one that failed or
// hit its deadline, with the phase it stopped in.
func TestRunningReadingShowsAPartialComposition(t *testing.T) {
	db := openDB(t)
	for i := 0; i < 300; i++ {
		put(t, db.Write, obj{id: fmt.Sprint("p", i), kind: "k", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	// 300 rows at 5 per batch is 60 batches of at least 10 ms: the walk takes
	// 600 ms at the least, so a 250 ms deadline always comes first.
	s := Open("", Config{BatchRows: 5, MinYield: 10 * time.Millisecond, Deadline: 250 * time.Millisecond})
	s.Start(dbFn(db.Read))
	var saw bool
	for i := 0; i < 400 && !saw; i++ {
		st := s.Status()
		if st.State == StateRunning && st.Objects != nil && st.Objects.Total.Objects > 0 && !st.Objects.Complete && st.Progress.Phase == "og_objects" {
			saw = true
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !saw {
		t.Fatalf("a running reading never showed a partial composition: %+v", s.Status())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = s.Wait(ctx)
	st := s.Status()
	if st.State != StateIncomplete || st.Progress.Phase == "" || st.Objects == nil || st.Objects.Total.Objects == 0 {
		t.Fatalf("after the deadline = %+v", st)
	}
}

// One table's read failure is recorded against that table and the rest of
// the reading goes on.
func TestOneTableFailingDoesNotEndTheReading(t *testing.T) {
	db := openDB(t)
	put(t, db.Write, obj{id: "x", kind: "k", hash: "h", body: []byte("b")})
	if _, err := db.Write.Exec(`CREATE TABLE zz_boom (a TEXT)`); err != nil {
		t.Fatal(err)
	}
	spy, d := spyOpen(t, db.Path)
	d.failOn = `"zz_boom"`
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), spy)
	if rep.State != StateIncomplete || rep.Reason != ReasonReadError || !rep.Incomplete {
		t.Fatalf("report = %+v", rep)
	}
	var boom, objs TableStat
	for _, ts := range rep.Tables {
		switch ts.Table {
		case "zz_boom":
			boom = ts
		case "og_objects":
			objs = ts
		}
	}
	if boom.Failure != ReasonReadError || boom.Complete || !objs.Complete || objs.Rows != 1 {
		t.Fatalf("boom = %+v objects = %+v", boom, objs)
	}
}

// A persisted file that is not ours is ignored; a corrupt one too.
func TestCorruptAndForeignFilesAreIgnored(t *testing.T) {
	for name, content := range map[string]string{
		"corrupt":   "{not json",
		"empty":     "",
		"foreign":   `{"hello":"world","state":"running"}`,
		"unknown":   `{"state":"exploded","started_at":"2026-11-01T00:00:00Z","progress":{"rows_walked":1,"batches":1,"tables_done":0,"tables_total":0}}`,
		"wrongtype": `[1,2,3]`,
	} {
		file := filepath.Join(t.TempDir(), "store-composition.json")
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		s := Open(file, Config{})
		if st := s.Status(); st != nil {
			t.Errorf("%s: loaded %+v", name, st)
		}
		db := openDB(t)
		put(t, db.Write, obj{id: "x", kind: "k", hash: "h", body: []byte("b")})
		if rep := readAll(t, s, db.Read); rep.State != StateDone {
			t.Errorf("%s: a new reading after it = %+v", name, rep)
		}
	}
}

// A WITHOUT ROWID table with a composite text key, read in batches smaller
// than it, is counted exactly.
func TestCompositeTextKeyWithoutRowidAcrossBatches(t *testing.T) {
	db := openDB(t)
	tx, _ := db.Write.Begin()
	rows := 0
	for i := 0; i < 12; i++ {
		for j := 0; j < 3; j++ {
			if _, err := tx.Exec(`INSERT INTO og_key_index VALUES (?,?)`, fmt.Sprint("c", i), fmt.Sprint("k", j)); err != nil {
				t.Fatal(err)
			}
			rows++
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rep := readAll(t, Open("", Config{BatchRows: 4, MinYield: time.Microsecond}), db.Read)
	for _, ts := range rep.Tables {
		if ts.Table == "og_key_index" {
			if ts.Rows != int64(rows) || !ts.Complete {
				t.Fatalf("og_key_index = %+v, want %d rows", ts, rows)
			}
			return
		}
	}
	t.Fatal("og_key_index missing")
}

// The unit: an integer or real is 8 bytes, null 0, text and blob their byte
// length; stored generated columns count, virtual ones do not.
func TestPayloadBytesAreOneUnit(t *testing.T) {
	db := openDB(t)
	for _, q := range []string{
		`CREATE TABLE u (a INTEGER, b REAL, c TEXT, d BLOB, e TEXT, g TEXT GENERATED ALWAYS AS (c || 'zzzz') VIRTUAL, h TEXT GENERATED ALWAYS AS (c || 'yyyy') STORED)`,
		`INSERT INTO u(a,b,c,d,e) VALUES (123456789, 1.5, 'héllo', x'0102', NULL)`,
	} {
		if _, err := db.Write.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	for _, ts := range rep.Tables {
		if ts.Table == "u" {
			// 8 + 8 + 6 (é is two bytes) + 2 + 0 + stored h (6+4)
			if want := int64(8 + 8 + 6 + 2 + 0 + 10); ts.PayloadBytes != want {
				t.Fatalf("payload = %d, want %d", ts.PayloadBytes, want)
			}
			return
		}
	}
	t.Fatal("table u missing")
}

func dbFn(db *sql.DB) func() *sql.DB { return func() *sql.DB { return db } }

// The data statements, not only their text, are bounded: every one is an
// index search (or the first batch of a key-ordered scan) with no sort, so a
// batch costs a batch whatever the table.
func TestDataStatementPlansAreBounded(t *testing.T) {
	db := openDB(t)
	put(t, db.Write, obj{id: "p1", kind: "k", hash: "h", body: []byte("b")})
	if _, err := db.Write.Exec(`INSERT INTO og_key_index VALUES ('a','b')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Write.Exec(`INSERT INTO run_memory_entries(entry_id, loop_id, kind, created_at) VALUES ('e','L','k','2026-11-01')`); err != nil {
		t.Fatal(err)
	}
	spy, d := spyOpen(t, db.Path)
	readAll(t, Open("", Config{BatchRows: 1, MinYield: time.Microsecond}), spy)
	seen := map[string]bool{}
	for _, q := range d.seen {
		if strings.Contains(q, "LIMIT") && !strings.Contains(q, "pragma_") {
			seen[q] = true
		}
	}
	if len(seen) < 6 {
		t.Fatalf("only %d distinct data statements seen", len(seen))
	}
	for q := range seen {
		args := make([]any, strings.Count(q, "?"))
		for i := range args {
			args[i] = int64(0)
		}
		rs, err := db.Read.Query("EXPLAIN QUERY PLAN "+q, args...)
		if err != nil {
			t.Fatalf("%v: %s", err, q)
		}
		var plan []string
		for rs.Next() {
			var id, parent, unused int
			var detail string
			if err := rs.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		_ = rs.Close()
		all := strings.Join(plan, " | ")
		if strings.Contains(all, "TEMP B-TREE") {
			t.Errorf("a sort per batch: %s\n  %s", q, all)
		}
		tableScan := false
		for _, d := range plan {
			// The outer aggregate scans the bounded subquery's rows; that is not a table scan.
			if strings.HasPrefix(d, "SCAN") && !strings.Contains(d, "subquery") && !strings.Contains(d, "SUBQUERY") {
				tableScan = true
			}
		}
		if tableScan && strings.Contains(q, "WHERE") {
			t.Errorf("a scan under a WHERE (not a search): %s\n  %s", q, all)
		}
		if strings.Contains(q, "LEFT JOIN") && !strings.Contains(all, "LEFT-JOIN") {
			t.Errorf("the body join is not an indexed left join: %s", all)
		}
		if strings.Contains(q, "NOT EXISTS") && strings.Count(all, "SEARCH") < 2 {
			t.Errorf("the anti-join is not two searches: %s", all)
		}
	}
}

// A WITHOUT ROWID table whose key has a descending column is not walked (a
// sort per batch would make the walk quadratic): it is reported unsupported
// and the reading goes on.
func TestDescendingKeyTableIsReportedAndSkipped(t *testing.T) {
	db := openDB(t)
	put(t, db.Write, obj{id: "x", kind: "k", hash: "h", body: []byte("b")})
	if _, err := db.Write.Exec(`CREATE TABLE zz_desc (a TEXT, b INTEGER, PRIMARY KEY (a DESC, b)) WITHOUT ROWID`); err != nil {
		t.Fatal(err)
	}
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	for _, ts := range rep.Tables {
		if ts.Table == "zz_desc" {
			if ts.Failure != ReasonUnsupportedKey || ts.Complete {
				t.Fatalf("zz_desc = %+v", ts)
			}
			if rep.Objects == nil || !rep.Objects.Complete {
				t.Fatalf("the reading did not go on: %+v", rep.Objects)
			}
			return
		}
	}
	t.Fatal("zz_desc missing")
}

// A store replaced under a running reading ends it incomplete, never done:
// the read pool swapped (a Reopen), or the state file renamed over.
func TestReplacedStoreEndsTheReading(t *testing.T) {
	a, b := openDB(t), openDB(t)
	for i := 0; i < 200; i++ {
		put(t, a.Write, obj{id: fmt.Sprint("a", i), kind: "tag.a", hash: fmt.Sprint("h", i), body: []byte("b")})
		put(t, b.Write, obj{id: fmt.Sprint("b", i), kind: "tag.b", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	var swapped atomic.Bool
	pool := func() *sql.DB {
		if swapped.Load() {
			return b.Read
		}
		return a.Read
	}
	s := Open("", Config{BatchRows: 5, MinYield: 5 * time.Millisecond})
	s.Start(pool)
	for s.Status().Progress.RowsWalked == 0 {
		time.Sleep(time.Millisecond)
	}
	swapped.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if st.State != StateIncomplete || st.Reason != ReasonStoreReplaced || !st.Incomplete {
		t.Fatalf("after a pool swap = %+v", st)
	}
	if st.Objects != nil {
		for _, k := range st.Objects.Kinds {
			if k.Kind == "tag.b" {
				t.Fatalf("the reading blended in the second store: %+v", st.Objects.Kinds)
			}
		}
	}
}

func TestStateFileSwappedUnderTheReadingEndsIt(t *testing.T) {
	db := openDB(t)
	other := openDB(t)
	for i := 0; i < 200; i++ {
		put(t, db.Write, obj{id: fmt.Sprint("s", i), kind: "k", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	s := Open("", Config{BatchRows: 5, MinYield: 5 * time.Millisecond, StateFile: db.Path})
	s.Start(dbFn(db.Read))
	for s.Status().Progress.RowsWalked == 0 {
		time.Sleep(time.Millisecond)
	}
	if err := os.Rename(other.Path, db.Path); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	if st := s.Status(); st.State != StateIncomplete || st.Reason != ReasonStoreReplaced {
		t.Fatalf("after a file swap = %+v", st)
	}
}

// A persisted report of another file is dropped; one of this file is kept;
// the WAL's size and the scope are in the report.
func TestPersistedReportIsStampedWithItsFile(t *testing.T) {
	a, b := openDB(t), openDB(t)
	put(t, a.Write, obj{id: "x", kind: "k", hash: "h", body: []byte("b")})
	file := filepath.Join(t.TempDir(), "store-composition.json")
	rep := readAll(t, Open(file, Config{MinYield: time.Microsecond, StateFile: a.Path}), a.Read)
	if rep.Store.Inode == 0 || rep.Store.Bytes == 0 || rep.Scope == "" || rep.WALBytes == 0 {
		t.Fatalf("stamp = %+v wal=%d scope=%q", rep.Store, rep.WALBytes, rep.Scope)
	}
	if st := Open(file, Config{StateFile: a.Path}).Status(); st == nil || st.State != StateDone {
		t.Fatalf("same file: %+v", st)
	}
	if st := Open(file, Config{StateFile: b.Path}).Status(); st != nil {
		t.Fatalf("another file's report was kept: %+v", st)
	}
}

// The persisted file is read back bounded: too large, or too many entries,
// and it is ignored.
func TestOversizedAndCrowdedPersistedFilesAreRefused(t *testing.T) {
	good, _ := json.Marshal(Report{State: StateDone, Scope: Scope, ReadingNote: ReadingNote})
	big := append(append([]byte(nil), good...), []byte(strings.Repeat(" ", MaxPersistedBytes+1))...)
	crowded := Report{State: StateDone, Scope: Scope, ReadingNote: ReadingNote, Tables: make([]TableStat, maxPersistedTables+1)}
	crowdedJSON, _ := json.Marshal(crowded)
	for name, content := range map[string][]byte{"big": big, "crowded": crowdedJSON} {
		file := filepath.Join(t.TempDir(), "store-composition.json")
		if err := os.WriteFile(file, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if st := Open(file, Config{}).Status(); st != nil {
			t.Errorf("%s: loaded", name)
		}
	}
	file := filepath.Join(t.TempDir(), "store-composition.json")
	_ = os.WriteFile(file, good, 0o600)
	if st := Open(file, Config{}).Status(); st == nil || st.State != StateDone {
		t.Fatalf("a normal file was refused: %+v", st)
	}
}

// The pool is checked and used as one: a func that gives the old store to
// the check and the new one to the statement (alternating calls once the walk
// is under way) must end the reading, never blend the second store in.
func TestPoolChangingBetweenCheckAndStatementNeverBlends(t *testing.T) {
	a, b := openDB(t), openDB(t)
	for i := 0; i < 200; i++ {
		put(t, a.Write, obj{id: fmt.Sprint("a", i), kind: "tag.a", hash: fmt.Sprint("h", i), body: []byte("b")})
		put(t, b.Write, obj{id: fmt.Sprint("b", i), kind: "tag.b", hash: fmt.Sprint("h", i), body: []byte("b")})
	}
	var alternate atomic.Bool
	var n atomic.Int64
	pool := func() *sql.DB {
		if alternate.Load() && n.Add(1)%2 == 0 {
			return b.Read
		}
		return a.Read
	}
	s := Open("", Config{BatchRows: 5, MinYield: 5 * time.Millisecond})
	s.Start(pool)
	for s.Status().Progress.RowsWalked == 0 {
		time.Sleep(time.Millisecond)
	}
	alternate.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := s.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	st := s.Status()
	if st.State != StateIncomplete || st.Reason != ReasonStoreReplaced {
		t.Fatalf("state = %s reason = %s", st.State, st.Reason)
	}
	if st.Objects != nil {
		for _, k := range st.Objects.Kinds {
			if k.Kind == "tag.b" {
				t.Fatalf("the second store was read: %+v", st.Objects.Kinds)
			}
		}
	}
}

// A swap that lands on the last batch is still caught: the reading is not
// reported done.
func TestSwapOnTheTerminalBatchIsNotDone(t *testing.T) {
	a, b := openDB(t), openDB(t)
	put(t, a.Write, obj{id: "only", kind: "tag.a", hash: "h", body: []byte("b")})
	var calls atomic.Int64
	swapAfter := int64(-1)
	pool := func() *sql.DB {
		if c := calls.Add(1); swapAfter >= 0 && c > swapAfter {
			return b.Read
		}
		return a.Read
	}
	// Learn how many fetches a clean reading makes, then swap for the last one.
	clean := Open("", Config{MinYield: time.Microsecond})
	if rep := readAll(t, clean, a.Read); rep.State != StateDone {
		t.Fatal(rep.State)
	}
	s := Open("", Config{MinYield: time.Microsecond})
	s.Start(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = s.Wait(ctx)
	total := calls.Load()
	for _, after := range []int64{total - 1, total - 2, total - 3} {
		calls.Store(0)
		swapAfter = after
		s2 := Open("", Config{MinYield: time.Microsecond})
		s2.Start(pool)
		if err := s2.Wait(ctx); err != nil {
			t.Fatal(err)
		}
		if st := s2.Status(); st.State == StateDone {
			t.Fatalf("a swap after fetch %d of %d was reported done: %+v", after, calls.Load(), st)
		}
	}
}

// The walk never fetches the pool except through the one checked accessor.
func TestOnlyOneFetchOfThePool(t *testing.T) {
	src, err := os.ReadFile("storecomposition.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(src), "r.source()"); n != 1 {
		t.Fatalf("the pool func is called in %d places; it must be one (the checked accessor)", n)
	}
}

// A persisted report whose strings are not from the vocabulary is ignored.
func TestHostilePersistedStringsAreIgnored(t *testing.T) {
	ok := func() Report {
		return Report{State: StateDone, Scope: Scope}
	}
	_ = ok
	base := `{"state":"done","started_at":"2026-11-22T00:00:00Z","progress":{"rows_walked":1,"batches":1,"tables_done":0,"tables_total":0},"store":{},"scope":%s,"reading_note":"` + ReadingNote + `"%s}`
	scope := `"` + Scope + `"`
	for name, content := range map[string]string{
		"scope":    fmt.Sprintf(base, `"SECRET scope text"`, ``),
		"note":     fmt.Sprintf(strings.Replace(base, ReadingNote, "SECRET note", 1), scope, ``),
		"reason":   fmt.Sprintf(base, scope, `,"reason":"free text here"`),
		"kind":     fmt.Sprintf(base, scope, `,"objects":{"header_table":"og_objects","body_table":"og_bodies","complete":true,"kinds":[{"kind":"SECRET KIND with spaces"}],"total":{"kind":"total"},"orphan_body_rows":0,"orphan_body_bytes":0,"digest_note":"x"}`),
		"longname": fmt.Sprintf(base, scope, `,"tables":[{"table":"`+strings.Repeat("n", 100000)+`","rows":1,"payload_bytes":1,"complete":true}]`),
		"control":  fmt.Sprintf(base, scope, `,"tables":[{"table":"a\u0007b","rows":1,"payload_bytes":1,"complete":true}]`),
	} {
		file := filepath.Join(t.TempDir(), "store-composition.json")
		if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if st := Open(file, Config{}).Status(); st != nil {
			t.Errorf("%s: a hostile file was served: %+v", name, st)
		}
	}
	good := fmt.Sprintf(base, scope, ``)
	file := filepath.Join(t.TempDir(), "store-composition.json")
	_ = os.WriteFile(file, []byte(good), 0o600)
	if st := Open(file, Config{}).Status(); st == nil {
		t.Fatal("a valid file was refused")
	}
}

// Keys at the edge of the integer range, and a single-column WITHOUT ROWID
// table with negative integer keys, are counted exactly.
func TestKeyEdgesAreExact(t *testing.T) {
	db := openDB(t)
	for _, q := range []string{
		`CREATE TABLE zz_edge (a TEXT)`,
		`INSERT INTO zz_edge(rowid, a) VALUES (-9223372036854775808, 'min'), (-1, 'm'), (0, 'z'), (9223372036854775807, 'max')`,
		`CREATE TABLE zz_int (k INTEGER PRIMARY KEY, v TEXT) WITHOUT ROWID`,
		`INSERT INTO zz_int VALUES (-5,'a'), (-1,'b'), (0,'c'), (7,'d'), (9223372036854775807,'e')`,
		`CREATE TABLE zz_txt (k TEXT PRIMARY KEY) WITHOUT ROWID`,
		`INSERT INTO zz_txt VALUES ('a'), ('ab'), ('abc'), ('b'), ('')`,
	} {
		if _, err := db.Write.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Write.Exec(`INSERT INTO og_objects(obj_id, canonical_id, object_kind, owner_id, content_hash, metadata, created_at, updated_at) VALUES (-9223372036854775808, 'minobj', 'k', 'o', 'h', '{}', '2026-11-01', '2026-11-01')`); err != nil {
		t.Fatal(err)
	}
	rep := readAll(t, Open("", Config{BatchRows: 2, MinYield: time.Microsecond}), db.Read)
	want := map[string]int64{"zz_edge": 4, "zz_int": 5, "zz_txt": 5, "og_objects": 1}
	for _, ts := range rep.Tables {
		if w, ok := want[ts.Table]; ok {
			if ts.Rows != w || !ts.Complete {
				t.Errorf("%s = %+v, want %d rows", ts.Table, ts, w)
			}
			delete(want, ts.Table)
		}
	}
	if len(want) != 0 {
		t.Fatalf("tables missing: %v", want)
	}
}

// More tables than a loadable report holds: the file written is one the
// loader accepts, and says how many tables it left out.
func TestReportWithManyTablesIsWrittenLoadable(t *testing.T) {
	db := openDB(t)
	tx, _ := db.Write.Begin()
	for i := 0; i < maxPersistedTables+50; i++ {
		if _, err := tx.Exec(fmt.Sprintf(`CREATE TABLE zz_t%05d (a INTEGER)`, i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "store-composition.json")
	readAll(t, Open(file, Config{BatchRows: 100, MinYield: time.Microsecond}), db.Read)
	again := Open(file, Config{}).Status()
	if again == nil || again.State != StateDone || len(again.Tables) > maxPersistedTables {
		t.Fatalf("the written report was not loadable: %+v", again)
	}
	data, _ := os.ReadFile(file)
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	if omitted, _ := raw["tables_omitted"].(float64); omitted < 50 {
		t.Fatalf("tables_omitted = %v", raw["tables_omitted"])
	}
}

// A reading cut short only by an unsupported key says so, not read_error.
func TestOnlyUnsupportedKeysGiveTheirOwnReason(t *testing.T) {
	db := openDB(t)
	if _, err := db.Write.Exec(`CREATE TABLE zz_desc (a TEXT, b INTEGER, PRIMARY KEY (a DESC, b)) WITHOUT ROWID`); err != nil {
		t.Fatal(err)
	}
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	if rep.State != StateIncomplete || rep.Reason != ReasonUnsupportedKey {
		t.Fatalf("state = %s reason = %s", rep.State, rep.Reason)
	}
}

// A table with no stored column (all of them generated is refused by SQLite,
// so the nearest thing: only a rowid) is walked and counted, not reported
// complete at zero rows.
func TestTableWithOnlyAGeneratedColumnIsWalked(t *testing.T) {
	db := openDB(t)
	if _, err := db.Write.Exec(`CREATE TABLE zz_virt (a INTEGER GENERATED ALWAYS AS (1) VIRTUAL)`); err != nil {
		t.Skipf("SQLite refuses a table with no stored column, so none can exist: %v", err)
	}
	if _, err := db.Write.Exec(`INSERT INTO zz_virt DEFAULT VALUES`); err != nil {
		t.Fatal(err)
	}
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	for _, ts := range rep.Tables {
		if ts.Table == "zz_virt" && (ts.Rows != 1 || ts.PayloadBytes != 0 || !ts.Complete) {
			t.Fatalf("zz_virt = %+v", ts)
		}
	}
}

// The writer never produces a file its own loader refuses: a table name that
// is long or has a control character is written as a placeholder.
func TestHostileTableNamesRoundTrip(t *testing.T) {
	db := openDB(t)
	long := "zz_" + strings.Repeat("n", 200)
	for _, name := range []string{"zz_a\x07b", long} {
		if _, err := db.Write.Exec(`CREATE TABLE ` + quoteIdent(name) + ` (a INTEGER)`); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(t.TempDir(), "store-composition.json")
	readAll(t, Open(file, Config{MinYield: time.Microsecond}), db.Read)
	again := Open(file, Config{}).Status()
	if again == nil || again.State != StateDone {
		t.Fatalf("the file the writer wrote was refused by the loader: %+v", again)
	}
	placeholders := 0
	for _, ts := range again.Tables {
		if strings.HasPrefix(ts.Table, "table_") {
			placeholders++
		}
		if !nameOK(ts.Table) {
			t.Fatalf("a bad name was persisted: %q", ts.Table)
		}
	}
	if placeholders != 2 {
		t.Fatalf("placeholders = %d", placeholders)
	}
}

// Different failures roll up to read_error.
func TestMixedFailuresRollUpToReadError(t *testing.T) {
	db := openDB(t)
	for _, q := range []string{
		`CREATE TABLE zz_desc (a TEXT, b INTEGER, PRIMARY KEY (a DESC, b)) WITHOUT ROWID`,
		`CREATE TABLE zz_boom (a TEXT)`,
	} {
		if _, err := db.Write.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	spy, d := spyOpen(t, db.Path)
	d.failOn = `"zz_boom"`
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), spy)
	if rep.State != StateIncomplete || rep.Reason != ReasonReadError {
		t.Fatalf("state = %s reason = %s", rep.State, rep.Reason)
	}
}

func TestReportCarriesTheLiveStoreNote(t *testing.T) {
	db := openDB(t)
	rep := readAll(t, Open("", Config{MinYield: time.Microsecond}), db.Read)
	if rep.ReadingNote != ReadingNote || !strings.Contains(ReadingNote, "not a snapshot") {
		t.Fatalf("reading note = %q", rep.ReadingNote)
	}
}

func TestSeenBeforeAllocatesNothing(t *testing.T) {
	set := make([]uint64, dupBits/64)
	if n := testing.AllocsPerRun(100, func() { seenBefore(set, "sha256-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef") }); n != 0 {
		t.Fatalf("seenBefore allocates %v per call", n)
	}
	if seenBefore(set, "x") || !seenBefore(set, "x") {
		t.Fatal("first sighting must be new, second a repeat")
	}
}
