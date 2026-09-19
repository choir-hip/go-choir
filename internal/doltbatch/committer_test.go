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

func TestNothingToCommitIsSuccess(t *testing.T) {
	db := openTestDB(t)
	c := New(db, "test", time.Hour)
	defer func() { _ = c.Close(context.Background()) }()

	// No writes since the last commit — CommitNow must tolerate
	// "nothing to commit" like the old per-mutation path did.
	if err := c.CommitNow(context.Background(), "idempotent retry"); err != nil {
		t.Fatalf("CommitNow with clean working set: %v", err)
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
