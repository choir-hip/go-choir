package objectgraph

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	embedded "github.com/dolthub/driver"
)

// openDoltTestWorkspace creates an embedded Dolt workspace in a temp directory,
// creates a test database, and returns a *sql.DB connected to that database.
// It reuses the doltSchema constant from dolt_store.go.
func openDoltTestWorkspace(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	workspacePath := t.TempDir()

	// Step 1: open root connection (no specific database) to create the DB.
	rootDSN := fmt.Sprintf(
		"file://%s?commitname=Test&commitemail=test@choir.local&multistatements=true",
		workspacePath,
	)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse root DSN: %v", err)
	}
	rootCfg.BackOff = newTestBackOff()
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	rootDB.SetMaxOpenConns(1)
	rootDB.SetMaxIdleConns(1)

	const dbName = "ogtest"
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + dbName); err != nil {
		_ = rootDB.Close()
		_ = rootConnector.Close()
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	// Step 2: open a database-scoped connection.
	dbDSN := fmt.Sprintf(
		"file://%s?commitname=Test&commitemail=test@choir.local&database=%s&multistatements=true&clientfoundrows=true",
		workspacePath,
		dbName,
	)

	var db *sql.DB
	var lastErr error
	for attempt := range 8 {
		dbCfg, err := embedded.ParseDSN(dbDSN)
		if err != nil {
			t.Fatalf("parse database DSN: %v", err)
		}
		dbCfg.BackOff = newTestBackOff()
		dc, err := embedded.NewConnector(dbCfg)
		if err != nil {
			lastErr = fmt.Errorf("new database connector: %w", err)
			time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
			continue
		}
		candidate := sql.OpenDB(dc)
		candidate.SetMaxOpenConns(1)
		candidate.SetMaxIdleConns(1)
		if pingErr := candidate.Ping(); pingErr == nil {
			db = candidate
			break
		} else {
			lastErr = pingErr
			_ = candidate.Close()
			_ = dc.Close()
			time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
		}
	}
	if db == nil {
		t.Fatalf("could not connect to dolt database: %v", lastErr)
	}

	// Apply schema (reuses doltSchema from dolt_store.go).
	if _, err := db.Exec(doltSchema); err != nil {
		_ = db.Close()
		t.Fatalf("apply schema: %v", err)
	}

	// Commit the initial schema so we have a main branch with a real commit.
	if _, err := db.Exec("CALL DOLT_ADD('.')"); err != nil {
		_ = db.Close()
		t.Fatalf("dolt add: %v", err)
	}
	if _, err := db.Exec("CALL DOLT_COMMIT('-a', '-m', 'initial schema')"); err != nil {
		_ = db.Close()
		t.Fatalf("initial commit: %v", err)
	}

	cleanup := func() {
		_ = db.Close()
	}
	return db, cleanup
}

func newTestBackOff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 25 * time.Millisecond
	b.RandomizationFactor = 0.2
	b.Multiplier = 1.6
	b.MaxInterval = 250 * time.Millisecond
	b.MaxElapsedTime = 2 * time.Second
	b.Reset()
	return backoff.WithMaxRetries(b, 8)
}

// insertObjects inserts n objects with the given canonical_id prefix.
func insertObjects(t *testing.T, db *sql.DB, prefix string, n int) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range n {
		cid := fmt.Sprintf("%s-%04d", prefix, i)
		_, err := db.Exec(
			`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cid, "choir.test", "owner-test", "", "", "sha256:fake", []byte("body"), `{"seq":`+fmt.Sprint(i)+`}`, now, now, 0, "",
		)
		if err != nil {
			t.Fatalf("insert %s: %v", cid, err)
		}
	}
}

// countObjects returns the number of rows in og_objects.
func countObjects(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM og_objects").Scan(&count); err != nil {
		t.Fatalf("count objects: %v", err)
	}
	return count
}

// checkoutBranch checks out (creating if -b) a branch.
func checkoutBranch(t *testing.T, db *sql.DB, args ...string) {
	t.Helper()
	parts := []string{"CALL DOLT_CHECKOUT("}
	for i, a := range args {
		if i > 0 {
			parts = append(parts, ", ")
		}
		parts = append(parts, "'"+a+"'")
	}
	parts = append(parts, ")")
	if _, err := db.Exec(strings.Join(parts, "")); err != nil {
		t.Fatalf("dolt checkout %v: %v", args, err)
	}
}

// commitAll commits all staged changes with the given message.
func commitAll(t *testing.T, db *sql.DB, msg string) {
	t.Helper()
	if _, err := db.Exec("CALL DOLT_ADD('.')"); err != nil {
		t.Fatalf("dolt add: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf("CALL DOLT_COMMIT('-a', '-m', '%s')", msg)); err != nil {
		t.Fatalf("dolt commit: %v", err)
	}
}

// mergeBranch merges the given branch into the current branch and returns
// success/failure, the raw result or error string, and latency.
// DOLT_MERGE is a stored procedure, so we use CALL instead of SELECT.
func mergeBranch(t *testing.T, db *sql.DB, branch string) (bool, string, time.Duration) {
	t.Helper()
	start := time.Now()
	_, err := db.Exec(fmt.Sprintf("CALL DOLT_MERGE('%s')", branch))
	elapsed := time.Since(start)
	if err != nil {
		return false, err.Error(), elapsed
	}
	return true, "ok", elapsed
}

// createBranch measures branch creation latency.
func createBranch(t *testing.T, db *sql.DB, branch string) time.Duration {
	t.Helper()
	start := time.Now()
	checkoutBranch(t, db, "-b", branch)
	return time.Since(start)
}

// commitWithLatency commits all changes and returns latency.
func commitWithLatency(t *testing.T, db *sql.DB, msg string) time.Duration {
	t.Helper()
	start := time.Now()
	commitAll(t, db, msg)
	return time.Since(start)
}

// TestDoltBranchAppendMostly tests that two workers writing disjoint PK sets
// on separate branches can be merged into main with zero conflicts.
func TestDoltBranchAppendMostly(t *testing.T) {
	db, cleanup := openDoltTestWorkspace(t)
	defer cleanup()

	// Create branches from main.
	latencyW1Branch := createBranch(t, db, "worker-1")
	checkoutBranch(t, db, "main")
	latencyW2Branch := createBranch(t, db, "worker-2")
	checkoutBranch(t, db, "main")

	// Worker-1: insert 50 objects.
	checkoutBranch(t, db, "worker-1")
	insertObjects(t, db, "w1", 50)
	latencyW1Commit := commitWithLatency(t, db, "worker-1 inserts")

	// Worker-2: insert 50 objects.
	checkoutBranch(t, db, "main")
	checkoutBranch(t, db, "worker-2")
	insertObjects(t, db, "w2", 50)
	latencyW2Commit := commitWithLatency(t, db, "worker-2 inserts")

	// Merge worker-1 into main.
	checkoutBranch(t, db, "main")
	ok1, detail1, latencyMerge1 := mergeBranch(t, db, "worker-1")
	if !ok1 {
		t.Fatalf("merge worker-1 failed: %s", detail1)
	}

	// Merge worker-2 into main.
	ok2, detail2, latencyMerge2 := mergeBranch(t, db, "worker-2")
	if !ok2 {
		t.Fatalf("merge worker-2 failed: %s", detail2)
	}

	// Verify main has all 100 objects.
	count := countObjects(t, db)
	if count != 100 {
		t.Fatalf("expected 100 objects on main, got %d", count)
	}

	t.Logf("=== TEST 1: Append-Mostly (no conflicts) ===")
	t.Logf("Branch create worker-1: %v", latencyW1Branch)
	t.Logf("Branch create worker-2: %v", latencyW2Branch)
	t.Logf("Commit worker-1 (50 objects): %v", latencyW1Commit)
	t.Logf("Commit worker-2 (50 objects): %v", latencyW2Commit)
	t.Logf("Merge worker-1 into main: %v (result: %s)", latencyMerge1, detail1)
	t.Logf("Merge worker-2 into main: %v (result: %s)", latencyMerge2, detail2)
	t.Logf("Final object count on main: %d", count)
	t.Logf("Conflicts: 0 (disjoint PK sets)")
}

// TestDoltBranchSameRowConflict tests Dolt's behavior when both main and a
// worker branch modify the same row.
func TestDoltBranchSameRowConflict(t *testing.T) {
	db, cleanup := openDoltTestWorkspace(t)
	defer cleanup()

	// Insert object X on main, commit.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"obj-X", "choir.test", "owner-test", "", "", "sha256:initial", []byte("initial"), `{"version":"initial"}`, now, now, 0, "",
	)
	if err != nil {
		t.Fatalf("insert obj-X: %v", err)
	}
	commitAll(t, db, "add object X on main")

	// Create branch worker-3 from main.
	createBranch(t, db, "worker-3")

	// On main: update object X's metadata to value A, commit.
	checkoutBranch(t, db, "main")
	_, err = db.Exec(`UPDATE og_objects SET metadata = ?, updated_at = ? WHERE canonical_id = ?`, `{"version":"A"}`, time.Now().UTC().Format(time.RFC3339Nano), "obj-X")
	if err != nil {
		t.Fatalf("update obj-X on main: %v", err)
	}
	commitAll(t, db, "main updates X to A")

	// On worker-3: update object X's metadata to value B, commit.
	checkoutBranch(t, db, "worker-3")
	_, err = db.Exec(`UPDATE og_objects SET metadata = ?, updated_at = ? WHERE canonical_id = ?`, `{"version":"B"}`, time.Now().UTC().Format(time.RFC3339Nano), "obj-X")
	if err != nil {
		t.Fatalf("update obj-X on worker-3: %v", err)
	}
	commitAll(t, db, "worker-3 updates X to B")

	// Switch to main, merge worker-3.
	checkoutBranch(t, db, "main")
	start := time.Now()
	_, mergeErr := db.Exec("CALL DOLT_MERGE('worker-3')")
	mergeLatency := time.Since(start)

	t.Logf("=== TEST 2: Same-Row Conflict ===")
	t.Logf("Merge worker-3 into main latency: %v", mergeLatency)

	if mergeErr != nil {
		t.Logf("Merge returned error: %v", mergeErr)
		t.Logf("Conflict type: merge error (Dolt reports conflict via error)")
		t.Logf("Error string: %s", mergeErr.Error())

		// Check if Dolt reports conflicts via DOLT_CONFLICTS.
		var conflictCount int
		conflictErr := db.QueryRow("SELECT COUNT(*) FROM dolt_conflicts_og_objects").Scan(&conflictCount)
		if conflictErr != nil {
			t.Logf("dolt_conflicts_og_objects query error: %v", conflictErr)
		} else {
			t.Logf("dolt_conflicts_og_objects count: %d", conflictCount)
		}

		// Try to abort the merge.
		_, abortErr := db.Exec("CALL DOLT_MERGE('--abort')")
		if abortErr != nil {
			t.Logf("merge --abort error: %v", abortErr)
		} else {
			t.Logf("merge --abort succeeded")
		}
	} else {
		t.Logf("Merge succeeded (no error returned)")
		t.Logf("No conflict reported by merge (Dolt may have auto-resolved)")
	}

	// Check the final state of obj-X on main.
	var metadata string
	if err := db.QueryRow("SELECT metadata FROM og_objects WHERE canonical_id = 'obj-X'").Scan(&metadata); err != nil {
		t.Fatalf("read obj-X after merge: %v", err)
	}
	t.Logf("Final obj-X metadata on main: %s", metadata)

	// Check conflict status.
	var conflictCount int
	conflictErr := db.QueryRow("SELECT COUNT(*) FROM dolt_conflicts_og_objects").Scan(&conflictCount)
	if conflictErr != nil {
		t.Logf("Post-merge conflict query error: %v", conflictErr)
	} else {
		t.Logf("Post-merge dolt_conflicts_og_objects count: %d", conflictCount)
	}

	// Document the behavior.
	t.Logf("")
	t.Logf("Conflict behavior notes:")
	if mergeErr != nil {
		t.Logf("  - Dolt reports same-row conflicts via merge error")
		t.Logf("  - The merge does not silently succeed; it surfaces the conflict")
		t.Logf("  - Conflict details are available via dolt_conflicts_<table>")
		t.Logf("  - Resolution: use DOLT_MERGE('--abort') to cancel, or resolve conflicts manually")
	} else {
		t.Logf("  - Dolt auto-resolved the conflict (likely took one side)")
		t.Logf("  - Final metadata value: %s", metadata)
	}
}

// TestDoltBranchScale tests merge latency with 500 objects on a single branch.
func TestDoltBranchScale(t *testing.T) {
	db, cleanup := openDoltTestWorkspace(t)
	defer cleanup()

	// Create branch worker-4.
	branchLatency := createBranch(t, db, "worker-4")
	checkoutBranch(t, db, "main")

	// On worker-4: insert 500 objects, commit.
	checkoutBranch(t, db, "worker-4")
	insertObjects(t, db, "w4", 500)
	commitLatency := commitWithLatency(t, db, "worker-4 inserts 500 objects")

	// Merge into main.
	checkoutBranch(t, db, "main")
	ok, detail, mergeLatency := mergeBranch(t, db, "worker-4")
	if !ok {
		t.Fatalf("merge worker-4 failed: %s", detail)
	}

	count := countObjects(t, db)
	if count != 500 {
		t.Fatalf("expected 500 objects on main, got %d", count)
	}

	t.Logf("=== TEST 3: Scale (500 objects) ===")
	t.Logf("Branch create worker-4: %v", branchLatency)
	t.Logf("Commit worker-4 (500 objects): %v", commitLatency)
	t.Logf("Merge worker-4 into main: %v (result: %s)", mergeLatency, detail)
	t.Logf("Final object count on main: %d", count)
}

// TestDoltBranchLatencyTable prints a consolidated latency summary across all tests.
func TestDoltBranchLatencyTable(t *testing.T) {
	// This is a meta-test that runs the full workflow and prints a summary table.
	// It's separate so the individual tests can run independently.
	db, cleanup := openDoltTestWorkspace(t)
	defer cleanup()

	// Measure branch creation.
	b1Start := time.Now()
	checkoutBranch(t, db, "-b", "lat-worker-1")
	b1Latency := time.Since(b1Start)
	checkoutBranch(t, db, "main")

	b2Start := time.Now()
	checkoutBranch(t, db, "-b", "lat-worker-2")
	b2Latency := time.Since(b2Start)
	checkoutBranch(t, db, "main")

	// Worker-1: 50 objects.
	checkoutBranch(t, db, "lat-worker-1")
	insertObjects(t, db, "latw1", 50)
	c1Start := time.Now()
	commitAll(t, db, "lat-worker-1 50 objects")
	c1Latency := time.Since(c1Start)

	// Worker-2: 50 objects.
	checkoutBranch(t, db, "main")
	checkoutBranch(t, db, "lat-worker-2")
	insertObjects(t, db, "latw2", 50)
	c2Start := time.Now()
	commitAll(t, db, "lat-worker-2 50 objects")
	c2Latency := time.Since(c2Start)

	// Merge both.
	checkoutBranch(t, db, "main")
	_, _, m1Latency := mergeBranch(t, db, "lat-worker-1")
	_, _, m2Latency := mergeBranch(t, db, "lat-worker-2")

	// Scale: 500 objects.
	checkoutBranch(t, db, "-b", "lat-worker-4")
	insertObjects(t, db, "latw4", 500)
	c4Start := time.Now()
	commitAll(t, db, "lat-worker-4 500 objects")
	c4Latency := time.Since(c4Start)

	checkoutBranch(t, db, "main")
	_, _, m4Latency := mergeBranch(t, db, "lat-worker-4")

	count := countObjects(t, db)

	t.Logf("=== CONSOLIDATED LATENCY TABLE ===")
	t.Logf("")
	t.Logf("| Operation                         | Latency     |")
	t.Logf("|-----------------------------------|-------------|")
	t.Logf("| Branch create (worker-1)          | %v |", b1Latency)
	t.Logf("| Branch create (worker-2)          | %v |", b2Latency)
	t.Logf("| Commit 50 objects (worker-1)      | %v |", c1Latency)
	t.Logf("| Commit 50 objects (worker-2)      | %v |", c2Latency)
	t.Logf("| Merge 50 objects (worker-1)       | %v |", m1Latency)
	t.Logf("| Merge 50 objects (worker-2)       | %v |", m2Latency)
	t.Logf("| Commit 500 objects (worker-4)     | %v |", c4Latency)
	t.Logf("| Merge 500 objects (worker-4)      | %v |", m4Latency)
	t.Logf("")
	t.Logf("| Total objects on main             | %d          |", count)
}
