package computerversion

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	embedded "github.com/dolthub/driver/v2"
)

// TestDriverV2Smoke is the pre-flight embedded-driver smoke: root connect,
// SHOW DATABASES, CREATE DATABASE, per-database connect, multi-statement
// execution, client-found-rows, and CALL DOLT_GC followed by close+reopen.
// It pins the driver/v2 compatibility contract the pre-flight depends on
// (DSN/root+per-database connector semantics unchanged from v1).
func TestDriverV2Smoke(t *testing.T) {
	root := t.TempDir()

	openRoot := func() (*sql.DB, interface{ Close() error }, error) {
		cfg, err := embedded.ParseDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root))
		if err != nil {
			return nil, nil, err
		}
		connector, err := embedded.NewConnector(cfg)
		if err != nil {
			return nil, nil, err
		}
		db := sql.OpenDB(connector)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		return db, connector, nil
	}
	openDB := func(database string) (*sql.DB, interface{ Close() error }, error) {
		cfg, err := embedded.ParseDSN(fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true", root, database))
		if err != nil {
			return nil, nil, err
		}
		connector, err := embedded.NewConnector(cfg)
		if err != nil {
			return nil, nil, err
		}
		db := sql.OpenDB(connector)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		return db, connector, nil
	}

	rootDB, connector, err := openRoot()
	if err != nil {
		t.Fatalf("root connect: %v", err)
	}
	defer func() {
		_ = rootDB.Close()
		if connector != nil {
			_ = connector.Close()
		}
	}()

	// SHOW DATABASES on the root connection.
	var dbNames strings.Builder
	rows, err := rootDB.Query("SHOW DATABASES")
	if err != nil {
		t.Fatalf("SHOW DATABASES: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan database name: %v", err)
		}
		dbNames.WriteString(name)
		dbNames.WriteString(",")
	}
	if !strings.Contains(dbNames.String(), "information_schema") {
		t.Fatalf("SHOW DATABASES missing information_schema: %q", dbNames.String())
	}

	// CREATE DATABASE through the root connection.
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS smokev2"); err != nil {
		t.Fatalf("CREATE DATABASE: %v", err)
	}
	// dolt driver/v2: while the root (no-database) engine connection is open,
	// a second engine on the same workspace attaches the database connection
	// read-only. Close the root engine before the per-database write path
	// (mirrors the production store's single-writer engine lifetime).
	if err := rootDB.Close(); err != nil {
		t.Fatalf("close root engine: %v", err)
	}
	if connector != nil {
		if err := connector.Close(); err != nil {
			t.Fatalf("close root connector: %v", err)
		}
		connector = nil
	}

	// Multi-statement execution + client-found-rows on a per-database connect
	// (mirrors the production DSN: multistatements=true&clientfoundrows=true).
	db, connector2, err := openDB("smokev2")
	if err != nil {
		t.Fatalf("per-db connect: %v", err)
	}
	defer func() {
		_ = db.Close()
		if connector2 != nil {
			_ = connector2.Close()
		}
	}()

	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS t (id INT PRIMARY KEY, v TEXT); INSERT INTO t VALUES (1, 'a'), (2, 'b');"); err != nil {
		t.Fatalf("multi-statement exec: %v", err)
	}
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM t").Scan(&got); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if got != 2 {
		t.Fatalf("row count = %d, want 2", got)
	}

	// CALL DOLT_GC, close, reopen, and verify the row survives.
	if _, err := db.Exec("CALL DOLT_GC()"); err != nil {
		t.Fatalf("DOLT_GC: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close after gc: %v", err)
	}
	db2, connector3, err := openDB("smokev2")
	if err != nil {
		t.Fatalf("reopen connect: %v", err)
	}
	defer func() {
		_ = db2.Close()
		if connector3 != nil {
			_ = connector3.Close()
		}
	}()
	if err := db2.QueryRow("SELECT COUNT(*) FROM t").Scan(&got); err != nil {
		t.Fatalf("reopen query: %v", err)
	}
	if got != 2 {
		t.Fatalf("reopened row count = %d, want 2", got)
	}
}
