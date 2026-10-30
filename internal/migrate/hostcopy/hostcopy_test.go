package hostcopy

// Failure modes (the isolation and protocol paths that rowcopy's embedded
// Dolt tests cannot reach):
//  1. The MySQL driver's read-only REPEATABLE READ transaction is refused by
//     the server, or the text-protocol value types (int64, []byte, time.Time)
//     break a conversion. (TestRunAgainstDoltServer copies through a real
//     `dolt sql-server`.)
//  2. A --tables filter spanning two databases fails because the table is in
//     only one. (same test)
//  3. Parallel copies deadlock on the shared source pool. (same test, 3 tables
//     at Parallel 3 with a pool of 4)
//  4. Rows written to the source while it is copied show up in the digest
//     but not in the copy. (TestSourceWritesDuringCopyDoNotBreakVerify)
//
// The test skips when `dolt` is not on PATH.

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/dbx/dbxtest"
)

func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }

// doltServer starts a dolt sql-server with databases platform and corpus and
// returns the base DSN prefix (user@tcp(addr)/).
func doltServer(t *testing.T) string {
	t.Helper()
	dolt, err := exec.LookPath("dolt")
	if err != nil {
		t.Skip("dolt not on PATH")
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	env := append(os.Environ(), "HOME="+home)
	for _, kv := range [][]string{{"user.name", "t"}, {"user.email", "t@example.invalid"}} {
		c := exec.Command(dolt, "config", "--global", "--add", kv[0], kv[1])
		c.Env = env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("dolt config: %v: %s", err, out)
		}
	}
	data := filepath.Join(root, "data")
	for _, db := range []string{"platform", "corpus"} {
		dir := filepath.Join(data, db)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		c := exec.Command(dolt, "init")
		c.Dir, c.Env = dir, env
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("dolt init: %v: %s", err, out)
		}
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	srv := exec.Command(dolt, "sql-server", "--host", "127.0.0.1", "--port", fmt.Sprint(port))
	srv.Dir, srv.Env = data, env
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Process.Kill(); _ = srv.Wait() })
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(60 * time.Second)
	for {
		db, err := sql.Open("mysql", "root@tcp("+addr+")/")
		if err == nil && db.Ping() == nil {
			db.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("dolt sql-server did not start")
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "root@tcp(" + addr + ")/"
}

func openPG(dsn string) (*sql.DB, error) { return dbx.OpenPostgres(dsn, dbx.PGOptions{MaxOpenConns: 2}) }

func mustExec(t *testing.T, db *sql.DB, stmts ...string) {
	t.Helper()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

func setup(t *testing.T) (Config, *sql.DB) {
	t.Helper()
	base := doltServer(t)
	a, err := sql.Open("mysql", base+"platform?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	b, err := sql.Open("mysql", base+"corpus?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	mustExec(t, a,
		`CREATE TABLE receipts (seq BIGINT UNSIGNED NOT NULL, computer VARCHAR(64) NOT NULL, body LONGTEXT NULL, at DATETIME NULL, ok TINYINT NOT NULL DEFAULT 0, PRIMARY KEY (computer, seq))`,
		`CREATE TABLE owners (id VARCHAR(64) NOT NULL PRIMARY KEY, meta JSON NULL, blob_data LONGBLOB NULL)`,
		`CREATE TABLE dead_a (id INT PRIMARY KEY)`,
		`INSERT INTO owners VALUES ('o1', '{"b":1,"a":[1,2]}', x'00ff10'), ('o2', NULL, NULL)`)
	for i := 0; i < 2000; i++ {
		mustExec(t, a, fmt.Sprintf(`INSERT INTO receipts VALUES (%d, 'c%d', 'body %d é', '2026-05-06 07:08:09', %d)`, i, i%7, i, i%2))
	}
	mustExec(t, b,
		`CREATE TABLE og_objects (id VARCHAR(64) NOT NULL PRIMARY KEY, body LONGTEXT NULL, created_at DATETIME NULL)`,
		`INSERT INTO og_objects VALUES ('x', 'hello', '2026-01-02 03:04:05'), ('y', NULL, NULL)`)

	ctrl := dbxtest.PostgresDSN(t)
	graph := dbxtest.PostgresDSN(t)
	pc, err := openPG(ctrl)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	pg, err := openPG(graph)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	mustExec(t, pc,
		`CREATE TABLE receipts (seq BIGINT NOT NULL, computer TEXT NOT NULL, body TEXT, at TIMESTAMPTZ, ok SMALLINT NOT NULL DEFAULT 0, PRIMARY KEY (computer, seq))`,
		`CREATE TABLE owners (id TEXT NOT NULL PRIMARY KEY, meta JSONB, blob_data BYTEA)`)
	mustExec(t, pg, `CREATE TABLE og_objects (id TEXT NOT NULL PRIMARY KEY, body TEXT, created_at TIMESTAMPTZ)`)
	return Config{
		SourceControlDSN: base + "platform?parseTime=true",
		SourceGraphDSN:   base + "corpus?parseTime=true",
		TargetControlDSN: ctrl,
		TargetGraphDSN:   graph,
		Parallel:         3,
	}, a
}

func TestRunAgainstDoltServer(t *testing.T) {
	cfg, _ := setup(t)
	rep, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v\n%+v\n%+v", err, rep.Control, rep.Graph)
	}
	if !rep.OK || len(rep.Control.Tables) != 2 || len(rep.Graph.Tables) != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Control.SourceOnlyTables[0] != "dead_a" {
		t.Errorf("source-only = %v", rep.Control.SourceOnlyTables)
	}
	var n int
	pc, _ := openPG(cfg.TargetControlDSN)
	defer pc.Close()
	if err := pc.QueryRow(`SELECT COUNT(*) FROM receipts`).Scan(&n); err != nil || n != 2000 {
		t.Errorf("receipts = %d %v", n, err)
	}
	// Filter spanning both databases, rerun clean.
	cfg.Tables = []string{"og_objects", "owners"}
	rep, err = Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("filtered rerun: %v", err)
	}
	if len(rep.Control.Tables) != 1 || len(rep.Graph.Tables) != 1 {
		t.Errorf("filter copied %d + %d tables", len(rep.Control.Tables), len(rep.Graph.Tables))
	}
	cfg.Tables = []string{"nope"}
	if _, err := Run(context.Background(), cfg); err == nil {
		t.Error("a filter naming no table must fail")
	}
	cfg.Tables = nil
	cfg.DryRun = true
	rep, err = Run(context.Background(), cfg)
	if err != nil || rep.Control.Tables[0].SourceRows == 0 {
		t.Errorf("dry run: %v %+v", err, rep)
	}
}

func TestSourceWritesDuringCopyDoNotBreakVerify(t *testing.T) {
	cfg, a := setup(t)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 100000; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_, _ = a.Exec(fmt.Sprintf(`INSERT INTO receipts VALUES (%d, 'live', 'late', NULL, 0)`, i))
		}
	}()
	rep, err := Run(context.Background(), cfg)
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("copy under live writes: %v\n%+v", err, rep.Control)
	}
	for _, tb := range rep.Control.Tables {
		if tb.SourceDigest != tb.TargetDigest || tb.SourceRows != tb.TargetRows {
			t.Errorf("%s digest/rows differ: %+v", tb.Name, tb)
		}
	}
}
