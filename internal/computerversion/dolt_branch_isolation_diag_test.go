package computerversion

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	embedded "github.com/dolthub/driver"
)

// TestDoltBranchIsolationDiagnostic is a historical 2026-07-07 diagnostic that
// used *sql.DB (the connection pool). It showed that DOLT_CHECKOUT appeared
// not to provide branch isolation because each statement ran on a different
// DoltSession. TestDoltEmbeddedBranchIsolationPinnedConnection superseded this:
// on a pinned *sql.Conn, DOLT_CHECKOUT works and branch isolation is
// deterministic. This diagnostic is kept as source material for the D-PROMO
// evidence chain.
//
// This diagnostic checks:
//  1. What branch is active before/after checkout?
//  2. Is the working set actually swapped on checkout?
//  3. Does the embedded driver support session-level branch switching?
func TestDoltBranchIsolationDiagnostic(t *testing.T) {
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
	rootDB.SetMaxOpenConns(1)
	rootDB.SetMaxIdleConns(1)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS diagdb"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=diagdb&multistatements=true&clientfoundrows=true", root)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		t.Fatalf("parse db dsn: %v", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		t.Fatalf("new db connector: %v", err)
	}
	db := sql.OpenDB(dbConnector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
		_ = dbConnector.Close()
	})
	ctx := context.Background()

	// Setup: create table, insert main data, commit.
	if _, err := db.ExecContext(ctx, "CREATE TABLE items (id INT PRIMARY KEY, name VARCHAR(255) NOT NULL)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO items (id, name) VALUES (1, 'main-alpha')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := db.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', 'main initial')"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Check active branch before checkout.
	var activeBranch string
	if err := db.QueryRowContext(ctx, "SELECT active_branch()").Scan(&activeBranch); err != nil {
		t.Logf("active_branch() query failed: %v (may not be supported)", err)
	} else {
		t.Logf("active branch before checkout: %s", activeBranch)
	}

	// Create branch.
	if _, err := db.ExecContext(ctx, "CALL DOLT_BRANCH('candidate-1')"); err != nil {
		t.Fatalf("dolt_branch: %v", err)
	}
	t.Log("DOLT_BRANCH('candidate-1') succeeded")

	// List branches.
	branchRows, err := db.QueryContext(ctx, "SELECT * FROM dolt_branches")
	if err != nil {
		t.Logf("dolt_branches query failed: %v", err)
	} else {
		cols, _ := branchRows.Columns()
		t.Logf("dolt_branches columns: %v", cols)
		for branchRows.Next() {
			vals := make([]interface{}, len(cols))
			ptrs := make([]interface{}, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := branchRows.Scan(ptrs...); err != nil {
				t.Logf("scan branch row: %v", err)
				continue
			}
			row := make(map[string]interface{})
			for i, col := range cols {
				row[col] = vals[i]
			}
			t.Logf("  branch: %+v", row)
		}
		branchRows.Close()
	}

	// Checkout candidate branch.
	if _, err := db.ExecContext(ctx, "CALL DOLT_CHECKOUT('candidate-1')"); err != nil {
		t.Fatalf("dolt_checkout: %v", err)
	}
	t.Log("DOLT_CHECKOUT('candidate-1') succeeded")

	// Check active branch after checkout.
	if err := db.QueryRowContext(ctx, "SELECT active_branch()").Scan(&activeBranch); err != nil {
		t.Logf("active_branch() after checkout failed: %v", err)
	} else {
		t.Logf("active branch after checkout: %s", activeBranch)
	}

	// Check what data is visible on the candidate branch.
	var candidateCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&candidateCount); err != nil {
		t.Fatalf("count on candidate: %v", err)
	}
	t.Logf("items count on candidate-1: %d (expected 1, inherited from main)", candidateCount)

	// Insert data on candidate branch.
	if _, err := db.ExecContext(ctx, "INSERT INTO items (id, name) VALUES (2, 'candidate-beta')"); err != nil {
		t.Fatalf("insert on candidate: %v", err)
	}
	if _, err := db.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', 'candidate change')"); err != nil {
		t.Fatalf("commit on candidate: %v", err)
	}
	t.Log("inserted and committed on candidate-1")

	// Check count on candidate.
	var candidateCountAfter int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&candidateCountAfter); err != nil {
		t.Fatalf("count after insert: %v", err)
	}
	t.Logf("items count on candidate-1 after insert: %d (expected 2)", candidateCountAfter)

	// Checkout main.
	if _, err := db.ExecContext(ctx, "CALL DOLT_CHECKOUT('main')"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	t.Log("DOLT_CHECKOUT('main') succeeded")

	// Check active branch after checkout back to main.
	if err := db.QueryRowContext(ctx, "SELECT active_branch()").Scan(&activeBranch); err != nil {
		t.Logf("active_branch() after checkout main failed: %v", err)
	} else {
		t.Logf("active branch after checkout main: %s", activeBranch)
	}

	// Check what data is visible on main after checkout.
	var mainCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&mainCount); err != nil {
		t.Fatalf("count on main: %v", err)
	}
	t.Logf("items count on main after checkout: %d (expected 1 for isolation)", mainCount)

	if mainCount == 1 {
		t.Log("RESULT: branch isolation WORKS in this single pooled-connection run")
	} else {
		t.Logf("RESULT: branch isolation DOES NOT WORK in this pooled-connection run — main has %d rows after candidate checkout", mainCount)
		t.Log("This means DOLT_CHECKOUT in the pooled run did not swap the working set (a connection-pooling artifact).")
		t.Log("See TestDoltEmbeddedBranchIsolationPinnedConnection for the deterministic pinned-connection settlement.")
	}

	// Also try AS OF query to verify history reads work.
	var asOfCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items AS OF 'HEAD~1'").Scan(&asOfCount); err != nil {
		t.Logf("AS OF query failed: %v (may not be supported in embedded mode)", err)
	} else {
		t.Logf("AS OF 'HEAD~1' count: %d", asOfCount)
	}
}
