package doltbatch

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	embedded "github.com/dolthub/driver/v2"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	root := t.TempDir()
	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse root dsn: %v", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS batchtest"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dsn := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=batchtest&multistatements=true", root)
	cfg, err := embedded.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		t.Fatalf("new connector: %v", err)
	}
	db := sql.OpenDB(connector)
	t.Cleanup(func() {
		_ = db.Close()
		_ = connector.Close()
	})
	if _, err := db.Exec("CREATE TABLE t (id INT PRIMARY KEY, v VARCHAR(64))"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func doltLogCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM dolt_log").Scan(&n); err != nil {
		t.Fatalf("count dolt_log: %v", err)
	}
	return n
}

func TestMarkDebouncesIntoSingleCommit(t *testing.T) {
	db := openTestDB(t)
	base := doltLogCount(t, db)

	c := New(db, "test", 50*time.Millisecond)
	defer func() { _ = c.Close(context.Background()) }()
	for i := range 5 {

		if _, err := db.Exec("INSERT INTO t VALUES (?, ?)", i, "v"); err != nil {
			t.Fatalf("insert: %v", err)
		}
		c.Mark(fmt.Sprintf("insert %d", i))
	}

	deadline := time.Now().Add(5 * time.Second)
	for doltLogCount(t, db) == base && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := doltLogCount(t, db); got != base+1 {
		t.Fatalf("dolt_log = %d, want %d — five marks must coalesce into one commit", got, base+1)
	}
}

func TestCommitNowCommitsSynchronously(t *testing.T) {
	db := openTestDB(t)
	base := doltLogCount(t, db)

	c := New(db, "test", time.Hour) // debounce far out: only CommitNow may commit
	defer func() { _ = c.Close(context.Background()) }()

	if _, err := db.Exec("INSERT INTO t VALUES (1, 'x')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := c.CommitNow(context.Background(), "checkpoint boundary"); err != nil {
		t.Fatalf("CommitNow: %v", err)
	}
	if got := doltLogCount(t, db); got != base+1 {
		t.Fatalf("dolt_log = %d, want %d after CommitNow", got, base+1)
	}
}

func TestCloseFlushesPendingDirty(t *testing.T) {
	db := openTestDB(t)
	base := doltLogCount(t, db)

	c := New(db, "test", time.Hour)
	if _, err := db.Exec("INSERT INTO t VALUES (1, 'x')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	c.Mark("pending write")
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := doltLogCount(t, db); got != base+1 {
		t.Fatalf("dolt_log = %d, want %d — Close must flush pending dirty state", got, base+1)
	}
}

// Snapshot mode off (docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md):
// Store A's history has no reader, and every snapshot commit retains the
// rows it changed in oldgen forever. Failure modes pinned here, written
// before the code: F1 off still commits; F2 a write is lost without the
// snapshot; F3 an unknown value turns snapshots off; F6 Close errors.

func TestParseModeDefaultsToCommit(t *testing.T) {
	cases := []struct {
		raw     string
		want    Mode
		wantErr bool
	}{
		{"", ModeCommit, false},
		{"commit", ModeCommit, false},
		{" off ", ModeOff, false},
		{"OFF", ModeOff, false},
		{"of", ModeCommit, true}, // F3: a typo keeps the snapshots
		{"false", ModeCommit, true},
	}
	for _, tc := range cases {
		got, err := ParseMode(tc.raw)
		if got != tc.want || (err != nil) != tc.wantErr {
			t.Errorf("ParseMode(%q) = %q, %v; want %q, error %v", tc.raw, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestModeOffNeverCommits(t *testing.T) {
	db := openTestDB(t)
	base := doltLogCount(t, db)

	c := NewWithMode(db, "test", 10*time.Millisecond, ModeOff)
	if c.Mode() != ModeOff {
		t.Fatalf("Mode() = %q, want off", c.Mode())
	}
	if _, err := db.Exec("INSERT INTO t VALUES (1, 'x')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	c.Mark("debounced write")
	time.Sleep(100 * time.Millisecond) // ten debounce windows
	if err := c.CommitNow(context.Background(), "checkpoint boundary"); err != nil {
		t.Fatalf("CommitNow in off mode: %v", err) // F5: boundary callers must not fail
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush in off mode: %v", err)
	}
	if err := c.Close(context.Background()); err != nil {
		t.Fatalf("Close in off mode: %v", err) // F6
	}
	if got := doltLogCount(t, db); got != base {
		t.Fatalf("dolt_log = %d, want %d: off mode must never DOLT_COMMIT", got, base) // F1
	}
	var v string
	if err := db.QueryRow("SELECT v FROM t WHERE id = 1").Scan(&v); err != nil || v != "x" {
		t.Fatalf("row after off-mode writes = %q, %v; want x", v, err)
	}
}

// F2: SQL COMMIT alone is durable: the row survives closing and reopening the
// store with no Dolt commit ever made.
func TestModeOffWriteSurvivesReopen(t *testing.T) {
	root := t.TempDir()
	open := func() (*sql.DB, func()) {
		dsn := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
		cfg, err := embedded.ParseDSN(dsn)
		if err != nil {
			t.Fatalf("parse dsn: %v", err)
		}
		connector, err := embedded.NewConnector(cfg)
		if err != nil {
			t.Fatalf("new connector: %v", err)
		}
		db := sql.OpenDB(connector)
		return db, func() { _ = db.Close(); _ = connector.Close() }
	}

	db, closeDB := open()
	for _, q := range []string{
		"CREATE DATABASE IF NOT EXISTS reopen",
		"USE reopen",
		"CREATE TABLE t (id INT PRIMARY KEY, v VARCHAR(64))",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	if _, err := conn.ExecContext(context.Background(), "USE reopen"); err != nil {
		t.Fatalf("use: %v", err)
	}
	var base int
	if err := conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM dolt_log").Scan(&base); err != nil {
		t.Fatalf("count dolt_log: %v", err)
	}
	c := NewWithMode(db, "test", time.Hour, ModeOff)
	if _, err := conn.ExecContext(context.Background(), "INSERT INTO t VALUES (7, 'kept')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := c.CommitNow(context.Background(), "boundary"); err != nil {
		t.Fatalf("CommitNow: %v", err)
	}
	_ = c.Close(context.Background())
	_ = conn.Close()
	closeDB()

	db, closeDB = open()
	defer closeDB()
	conn, err = db.Conn(context.Background())
	if err != nil {
		t.Fatalf("conn after reopen: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "USE reopen"); err != nil {
		t.Fatalf("use after reopen: %v", err)
	}
	var v string
	if err := conn.QueryRowContext(context.Background(), "SELECT v FROM t WHERE id = 7").Scan(&v); err != nil || v != "kept" {
		t.Fatalf("row after reopen = %q, %v; want kept: SQL COMMIT must be durable without a snapshot", v, err)
	}
	var after int
	if err := conn.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM dolt_log").Scan(&after); err != nil {
		t.Fatalf("count dolt_log after reopen: %v", err)
	}
	if after != base {
		t.Fatalf("dolt_log = %d after reopen, want %d", after, base)
	}
}
