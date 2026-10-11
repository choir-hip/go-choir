//go:build experiment

// W1 experiment (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// can captures get real concurrency from embedded Dolt? Run with
//
//	go test -race -tags experiment ./internal/store -run TestEngineConcurrency -v
//
// A: one connector, several sessions at once (what raising MaxOpenConns would do).
// B: two connectors on two workspaces, one session each, at the same time
// (a separate store for captures).
package store

import (
	"database/sql"
	"fmt"
	"sync"
	"testing"

	embedded "github.com/dolthub/driver/v2"
)

func openExperimentDB(t *testing.T, dir string, conns int) *sql.DB {
	t.Helper()
	rootCfg, err := embedded.ParseDSN(fmt.Sprintf("file://%s?commitname=Exp&commitemail=exp@choir.local&multistatements=true", dir))
	if err != nil {
		t.Fatal(err)
	}
	rootConn, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatal(err)
	}
	root := sql.OpenDB(rootConn)
	if _, err := root.Exec("CREATE DATABASE IF NOT EXISTS exp"); err != nil {
		t.Fatal(err)
	}
	_ = root.Close()
	_ = rootConn.Close()

	cfg, err := embedded.ParseDSN(fmt.Sprintf("file://%s?commitname=Exp&commitemail=exp@choir.local&database=exp&multistatements=true", dir))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := embedded.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(conn)
	db.SetMaxOpenConns(conns)
	t.Cleanup(func() { _ = db.Close(); _ = conn.Close() })
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS t (id VARCHAR(64) PRIMARY KEY, meta JSON, body LONGBLOB)"); err != nil {
		t.Fatal(err)
	}
	return db
}

func hammer(t *testing.T, db *sql.DB, prefix string, workers, n int) int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	errs := 0
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				id := fmt.Sprintf("%s-%d-%d", prefix, w, i)
				meta := fmt.Sprintf(`{"title":"<b>item %d</b>","url":"https://x.example/?a=1&b=%d"}`, i, i)
				_, err1 := db.Exec("INSERT INTO t (id, meta, body) VALUES (?, ?, ?)", id, meta, []byte("body"))
				var title string
				err2 := db.QueryRow("SELECT JSON_UNQUOTE(JSON_EXTRACT(meta, '$.title')) FROM t WHERE id = ?", id).Scan(&title)
				if err1 != nil || err2 != nil {
					mu.Lock()
					errs++
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()
	return errs
}

func TestEngineConcurrencyOneConnectorManySessions(t *testing.T) {
	db := openExperimentDB(t, t.TempDir(), 8)
	t.Logf("A: errors=%d", hammer(t, db, "a", 8, 40))
}

func TestEngineConcurrencyTwoConnectorsTwoWorkspaces(t *testing.T) {
	texture := openExperimentDB(t, t.TempDir(), 1)
	captures := openExperimentDB(t, t.TempDir(), 1)
	var wg sync.WaitGroup
	var e1, e2 int
	wg.Add(2)
	go func() { defer wg.Done(); e1 = hammer(t, texture, "tex", 1, 200) }()
	go func() { defer wg.Done(); e2 = hammer(t, captures, "cap", 1, 200) }()
	wg.Wait()
	t.Logf("B: texture errors=%d captures errors=%d", e1, e2)
}

// The documented race (488e8cd0): a JSON value's buffer aliases a shared
// prolly node; unescapeHTMLCodepoints mutates it in place. Readers of the
// same stored rows at the same time share those nodes.
func seedShared(t *testing.T, db *sql.DB) {
	for i := 0; i < 20; i++ {
		meta := fmt.Sprintf(`{"title":"<b>shared %d</b>","note":"a & b > c"}`, i)
		if _, err := db.Exec("INSERT INTO t (id, meta, body) VALUES (?, ?, ?)", fmt.Sprintf("s-%d", i), meta, []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
}

func readShared(db *sql.DB, workers, n int) {
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < n; i++ {
				rows, err := db.Query("SELECT meta FROM t WHERE id LIKE 's-%'")
				if err != nil {
					continue
				}
				for rows.Next() {
					var m string
					_ = rows.Scan(&m)
				}
				_ = rows.Close()
			}
		}()
	}
	wg.Wait()
}

func TestEngineConcurrencySharedRowsOneConnector(t *testing.T) {
	db := openExperimentDB(t, t.TempDir(), 8)
	seedShared(t, db)
	readShared(db, 8, 30)
}

func TestEngineConcurrencySharedRowsTwoConnectors(t *testing.T) {
	a := openExperimentDB(t, t.TempDir(), 1)
	b := openExperimentDB(t, t.TempDir(), 1)
	seedShared(t, a)
	seedShared(t, b)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); readShared(a, 1, 120) }()
	go func() { defer wg.Done(); readShared(b, 1, 120) }()
	wg.Wait()
}

// If unescapeHTMLCodepoints compacts a cached node in place, a second read
// of the same JSON value sees the compacted bytes plus a stale tail.
func TestEngineJSONRereadIsStable(t *testing.T) {
	db := openExperimentDB(t, t.TempDir(), 1)
	if _, err := db.Exec(`INSERT INTO t (id, meta, body) VALUES ('r', '{"title":"<b>a & b</b>","x":1}', 'x')`); err != nil {
		t.Fatal(err)
	}
	var first, second, third string
	for _, dst := range []*string{&first, &second, &third} {
		if err := db.QueryRow("SELECT meta FROM t WHERE id = 'r'").Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("reads: %q | %q | %q", first, second, third)
	if first != second || second != third {
		t.Fatalf("JSON value changed between reads")
	}
}
