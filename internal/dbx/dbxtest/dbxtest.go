// Package dbxtest gives tests a fresh SQLite or PostgreSQL database.
//
// Postgres starts one throwaway cluster per test binary (initdb and pg_ctl
// from PATH, a unix socket in a temp dir, no TCP) and creates one database
// per test. Call Main from TestMain so the cluster is stopped at exit:
//
//	func TestMain(m *testing.M) { os.Exit(dbxtest.Main(m)) }
//
// Set CHOIR_TEST_PG_DSN to use an existing server instead (CI service
// container); each test still gets its own database.
package dbxtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// SQLite opens a fresh SQLite database in t.TempDir.
func SQLite(t testing.TB) *dbx.SQLiteDB {
	t.Helper()
	db, err := dbx.OpenSQLite(filepath.Join(t.TempDir(), "test.db"), dbx.SQLiteOptions{})
	if err != nil {
		t.Fatalf("dbxtest: open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

var (
	clusterOnce sync.Once
	clusterDSN  string // admin DSN to the "postgres" database
	clusterErr  error
	clusterStop func()
)

// Main runs the tests and stops the cluster Postgres started, if any.
func Main(m interface{ Run() int }) int {
	code := m.Run()
	if clusterStop != nil {
		clusterStop()
	}
	return code
}

// PostgresDSN creates a fresh database and returns its DSN. The database is
// dropped when the test ends.
func PostgresDSN(t testing.TB) string {
	t.Helper()
	admin := adminDSN(t)
	name := "t_" + randomHex(8)
	adminDB, err := dbx.OpenPostgres(admin, dbx.PGOptions{MaxOpenConns: 2})
	if err != nil {
		t.Fatalf("dbxtest: open admin: %v", err)
	}
	defer adminDB.Close()
	if _, err := adminDB.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatalf("dbxtest: create database: %v", err)
	}
	t.Cleanup(func() {
		db, err := dbx.OpenPostgres(admin, dbx.PGOptions{MaxOpenConns: 1})
		if err != nil {
			return
		}
		defer db.Close()
		_, _ = db.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`)
	})
	return withDatabase(admin, name)
}

// Postgres opens a fresh database through dbx.OpenPostgres.
func Postgres(t testing.TB) *sql.DB {
	t.Helper()
	db, err := dbx.OpenPostgres(PostgresDSN(t), dbx.PGOptions{})
	if err != nil {
		t.Fatalf("dbxtest: open postgres: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func adminDSN(t testing.TB) string {
	t.Helper()
	if dsn := strings.TrimSpace(os.Getenv("CHOIR_TEST_PG_DSN")); dsn != "" {
		return dsn
	}
	clusterOnce.Do(startCluster)
	if clusterErr != nil {
		t.Skipf("dbxtest: no PostgreSQL available (%v); install postgresql or set CHOIR_TEST_PG_DSN", clusterErr)
	}
	return clusterDSN
}

func startCluster() {
	initdb, err := exec.LookPath("initdb")
	if err != nil {
		clusterErr = err
		return
	}
	pgctl, err := exec.LookPath("pg_ctl")
	if err != nil {
		clusterErr = err
		return
	}
	// Unix socket paths are limited to ~100 bytes; use a short dir.
	dir, err := os.MkdirTemp("", "pgt")
	if err != nil {
		clusterErr = err
		return
	}
	data := filepath.Join(dir, "data")
	out, err := exec.Command(initdb, "-D", data, "-U", "postgres", "-A", "trust", "--no-sync", "-E", "UTF8", "--locale=C").CombinedOutput()
	if err != nil {
		clusterErr = fmt.Errorf("initdb: %v: %s", err, lastLine(out))
		_ = os.RemoveAll(dir)
		return
	}
	opts := fmt.Sprintf("-k %s -c listen_addresses='' -c fsync=off -c synchronous_commit=off -c full_page_writes=off -c max_connections=200", dir)
	out, err = exec.Command(pgctl, "-D", data, "-o", opts, "-l", filepath.Join(dir, "log"), "-w", "-t", "60", "start").CombinedOutput()
	if err != nil {
		clusterErr = fmt.Errorf("pg_ctl start: %v: %s", err, lastLine(out))
		_ = os.RemoveAll(dir)
		return
	}
	clusterDSN = fmt.Sprintf("host=%s user=postgres dbname=postgres sslmode=disable", dir)
	clusterStop = func() {
		_ = exec.Command(pgctl, "-D", data, "-m", "immediate", "-w", "stop").Run()
		_ = os.RemoveAll(dir)
	}
	// Wait for accept.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		db, err := dbx.OpenPostgres(clusterDSN, dbx.PGOptions{MaxOpenConns: 1})
		if err == nil {
			_ = db.PingContext(context.Background())
			_ = db.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	clusterErr = fmt.Errorf("postgres did not accept connections")
}

func withDatabase(dsn, name string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err == nil {
			u.Path = "/" + name
			return u.String()
		}
	}
	fields := strings.Fields(dsn)
	kept := fields[:0]
	for _, f := range fields {
		if !strings.HasPrefix(f, "dbname=") {
			kept = append(kept, f)
		}
	}
	return strings.Join(append(kept, "dbname="+name), " ")
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func lastLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
