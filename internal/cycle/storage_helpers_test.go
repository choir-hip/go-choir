package cycle

import (
	"database/sql"
	"fmt"
	"testing"

	embedded "github.com/dolthub/driver"
)

// openTestStorage creates an embedded Dolt-backed Storage for cycle package
// tests. It mirrors the platform package's openTestPlatformStore pattern: a
// root Dolt repo is created in a temp dir, a "platform" database is
// provisioned, and the cycle schema is bootstrapped via NewStorageFromDB.
func openTestStorage(t *testing.T) *Storage {
	t.Helper()
	root := t.TempDir()
	return openTestStorageAtRoot(t, root)
}

// openTestStorageAtRoot opens (or reopens) an embedded Dolt-backed Storage at
// the given repo root. Used by tests that close and reopen storage to verify
// persistence across restarts.
func openTestStorageAtRoot(t *testing.T, root string) *Storage {
	t.Helper()
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
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS platform"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=platform&multistatements=true&clientfoundrows=true", root)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		t.Fatalf("parse db dsn: %v", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		t.Fatalf("new db connector: %v", err)
	}
	db := sql.OpenDB(dbConnector)
	store, err := NewStorageFromDB(db)
	if err != nil {
		t.Fatalf("bootstrap cycle storage: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_ = dbConnector.Close()
	})
	return store
}
