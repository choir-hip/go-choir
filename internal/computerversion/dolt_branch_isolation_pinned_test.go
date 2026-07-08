package computerversion

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	embedded "github.com/dolthub/driver"
)

// TestDoltEmbeddedBranchIsolationPinnedConnection verifies that branch-based
// candidate isolation is deterministic in the embedded Dolt driver when every
// SQL statement runs on a single pinned database/sql connection (*sql.Conn).
//
// The earlier 2026-07-07 experiment used *sql.DB and observed that checkout did
// not stick because statements were dispatched through the connection pool onto
// different DoltSessions. This test pins one connection for the entire sequence:
//
//  1. Create a database and table, insert main data, and commit on main.
//  2. Create a candidate branch (DOLT_BRANCH).
//  3. Checkout the candidate branch on the same pinned connection.
//  4. Insert candidate data and commit on the candidate branch.
//  5. Checkout main on the same pinned connection and verify main is unchanged.
//  6. Merge the candidate branch into main (DOLT_MERGE).
//  7. Tag the merge commit (DOLT_TAG).
//  8. Verify the tag points to HEAD.
//  9. Roll back to the pre-merge HEAD (DOLT_RESET --hard) and verify main is
//     restored to one row.
//  10. Roll forward to the tag and verify main is back to two rows.
//
// The settlement bar is repeat-N determinism: `go test -run
// TestDoltEmbeddedBranchIsolationPinnedConnection -count=10` must pass without
// flaking.
func TestDoltEmbeddedBranchIsolationPinnedConnection(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()

	// Step 0: create the database through a short-lived root connection.
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
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS pinneddb"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	// Open the database with one connection in the pool and pin it for the
	// entire test. This is the key difference from the earlier pooled experiment.
	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=pinneddb&multistatements=true&clientfoundrows=true", root)
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

	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("pin connection: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// Step 1: create table and insert main data, then commit.
	if _, err := conn.ExecContext(ctx, "CREATE TABLE items (id INT PRIMARY KEY, name VARCHAR(255) NOT NULL)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO items (id, name) VALUES (1, 'main-alpha')"); err != nil {
		t.Fatalf("insert main: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', 'main initial')"); err != nil {
		t.Fatalf("commit main: %v", err)
	}

	// Step 2: record pre-merge HEAD for later rollback.
	var mainHead string
	if err := conn.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&mainHead); err != nil {
		t.Fatalf("get main head: %v", err)
	}
	t.Logf("main HEAD before branch: %s", mainHead)

	// Step 3: create and checkout candidate branch.
	if _, err := conn.ExecContext(ctx, "CALL DOLT_BRANCH('candidate-1')"); err != nil {
		t.Fatalf("dolt_branch: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "CALL DOLT_CHECKOUT('candidate-1')"); err != nil {
		t.Fatalf("dolt_checkout candidate: %v", err)
	}

	// Step 4: insert candidate data and commit.
	if _, err := conn.ExecContext(ctx, "INSERT INTO items (id, name) VALUES (2, 'candidate-beta')"); err != nil {
		t.Fatalf("insert candidate: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', 'candidate change')"); err != nil {
		t.Fatalf("commit candidate: %v", err)
	}

	var candidateCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&candidateCount); err != nil {
		t.Fatalf("count candidate: %v", err)
	}
	if candidateCount != 2 {
		t.Fatalf("candidate branch expected 2 rows, got %d", candidateCount)
	}
	t.Logf("candidate branch has %d rows (ok)", candidateCount)

	// Step 5: checkout main and verify isolation.
	if _, err := conn.ExecContext(ctx, "CALL DOLT_CHECKOUT('main')"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	var mainCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&mainCount); err != nil {
		t.Fatalf("count main: %v", err)
	}
	if mainCount != 1 {
		t.Fatalf("branch isolation FAILED: main has %d rows after checkout, expected 1", mainCount)
	}
	t.Logf("main has %d rows after checkout (isolation ok)", mainCount)

	// Step 6: merge candidate into main.
	if _, err := conn.ExecContext(ctx, "CALL DOLT_MERGE('candidate-1')"); err != nil {
		t.Fatalf("dolt_merge: %v", err)
	}
	var postMergeCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&postMergeCount); err != nil {
		t.Fatalf("count post-merge: %v", err)
	}
	if postMergeCount != 2 {
		t.Fatalf("merge verification FAILED: main has %d rows, expected 2", postMergeCount)
	}
	t.Logf("post-merge main has %d rows (ok)", postMergeCount)

	// Step 7: tag the merge commit.
	if _, err := conn.ExecContext(ctx, "CALL DOLT_TAG('promote-v1')"); err != nil {
		t.Fatalf("dolt_tag: %v", err)
	}
	var tagHash, postMergeHead string
	if err := conn.QueryRowContext(ctx, "SELECT HASHOF('promote-v1')").Scan(&tagHash); err != nil {
		t.Fatalf("get tag hash: %v", err)
	}
	if err := conn.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&postMergeHead); err != nil {
		t.Fatalf("get head: %v", err)
	}
	if tagHash != postMergeHead {
		t.Fatalf("tag hash %s != HEAD %s", tagHash, postMergeHead)
	}
	t.Logf("promote-v1 tag points to HEAD %s (ok)", tagHash)

	// Step 8: rollback to pre-merge HEAD and verify one row.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("CALL DOLT_RESET('--hard', '%s')", mainHead)); err != nil {
		t.Fatalf("dolt_reset to pre-merge: %v", err)
	}
	var postResetCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&postResetCount); err != nil {
		t.Fatalf("count post-reset: %v", err)
	}
	if postResetCount != 1 {
		t.Fatalf("rollback FAILED: main has %d rows after reset, expected 1", postResetCount)
	}
	t.Logf("post-rollback main has %d row (ok)", postResetCount)

	// Step 9: roll forward to the promotion tag and verify two rows.
	if _, err := conn.ExecContext(ctx, "CALL DOLT_RESET('--hard', 'promote-v1')"); err != nil {
		t.Fatalf("dolt_reset to tag: %v", err)
	}
	var postTagResetCount int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&postTagResetCount); err != nil {
		t.Fatalf("count post-tag-reset: %v", err)
	}
	if postTagResetCount != 2 {
		t.Fatalf("roll-forward FAILED: main has %d rows, expected 2", postTagResetCount)
	}
	t.Logf("post-tag-reset main has %d rows (ok)", postTagResetCount)
}
