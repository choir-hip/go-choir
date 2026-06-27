package objectgraph

import (
	"bytes"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// startDoltTCPServer starts a dolt sql-server in a temp directory, initializes
// a database named "ogtest", applies the object graph schema, makes an initial
// commit, and returns the TCP port plus a cleanup function.
func startDoltTCPServer(t *testing.T) (int, func()) {
	t.Helper()

	doltPath, err := exec.LookPath("dolt")
	if err != nil {
		t.Skipf("dolt binary not found in PATH: %v", err)
	}

	dataDir := t.TempDir()
	const dbName = "ogtest"
	dbDir := filepath.Join(dataDir, dbName)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("create db dir: %v", err)
	}

	// Initialize dolt repo inside the database directory.
	initCmd := exec.Command(doltPath, "init", "--name", "Test", "--email", "test@choir.local")
	initCmd.Dir = dbDir
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("dolt init: %v\noutput: %s", err, out)
	}

	// Find a free TCP port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	// Start the sql-server.
	cmd := exec.Command(doltPath, "sql-server",
		"-H", "127.0.0.1",
		"-P", fmt.Sprintf("%d", port),
		"--data-dir", dataDir,
		"-l", "warning",
	)
	var logBuf bytes.Buffer
	cmd.Stdout = &logBuf
	cmd.Stderr = &logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sql-server: %v", err)
	}

	// Wait for the server to accept connections.
	dsn := fmt.Sprintf("root@tcp(127.0.0.1:%d)/%s?parseTime=true&multiStatements=true", port, dbName)
	var setupDB *sql.DB
	for i := 0; i < 40; i++ {
		setupDB, err = sql.Open("mysql", dsn)
		if err == nil {
			err = setupDB.Ping()
			if err == nil {
				break
			}
			setupDB.Close()
			setupDB = nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if setupDB == nil {
		_ = cmd.Process.Kill()
		t.Fatalf("sql-server not ready after 10s: %v\nlog:\n%s", err, logBuf.String())
	}

	// Apply schema (reuses doltSchema from dolt_store.go).
	if _, err := setupDB.Exec(doltSchema); err != nil {
		setupDB.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("apply schema: %v\nlog:\n%s", err, logBuf.String())
	}

	// Initial commit so main branch has a real commit.
	if _, err := setupDB.Exec("CALL DOLT_ADD('.')"); err != nil {
		setupDB.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("dolt add: %v", err)
	}
	if _, err := setupDB.Exec("CALL DOLT_COMMIT('-a', '-m', 'initial schema')"); err != nil {
		setupDB.Close()
		_ = cmd.Process.Kill()
		t.Fatalf("initial commit: %v", err)
	}

	cleanup := func() {
		setupDB.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
	return port, cleanup
}

// openTCPConn opens a new TCP connection to the dolt sql-server, simulating
// a separate worker VM connecting from its own client.
func openTCPConn(t *testing.T, port int) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("root@tcp(127.0.0.1:%d)/ogtest?parseTime=true&multiStatements=true", port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open tcp conn: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatalf("ping tcp conn: %v", err)
	}
	return db
}

// tcpInsertObjects inserts n objects with the given canonical_id prefix.
func tcpInsertObjects(t *testing.T, db *sql.DB, prefix string, n int) {
	t.Helper()
	now := time.Now().UTC()
	for i := range n {
		cid := fmt.Sprintf("%s-%04d", prefix, i)
		_, err := db.Exec(
			`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			cid, "choir.test", "owner-test", "", "", "sha256:fake", []byte("body"),
			fmt.Sprintf(`{"seq":%d}`, i), now, now, false, "",
		)
		if err != nil {
			t.Fatalf("insert %s: %v", cid, err)
		}
	}
}

// tcpCountObjects returns the number of rows in og_objects.
func tcpCountObjects(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM og_objects").Scan(&count); err != nil {
		t.Fatalf("count objects: %v", err)
	}
	return count
}

// tcpCheckoutBranch checks out (creating if -b) a branch.
func tcpCheckoutBranch(t *testing.T, db *sql.DB, args ...string) {
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

// tcpCommitAll stages and commits all changes with the given message.
func tcpCommitAll(t *testing.T, db *sql.DB, msg string) {
	t.Helper()
	if _, err := db.Exec("CALL DOLT_ADD('.')"); err != nil {
		t.Fatalf("dolt add: %v", err)
	}
	if _, err := db.Exec(fmt.Sprintf("CALL DOLT_COMMIT('-a', '-m', '%s')", msg)); err != nil {
		t.Fatalf("dolt commit: %v", err)
	}
}

// tcpCommitAllRaw is like tcpCommitAll but returns the error instead of fataling.
func tcpCommitAllRaw(t *testing.T, db *sql.DB, msg string) error {
	t.Helper()
	if _, err := db.Exec("CALL DOLT_ADD('.')"); err != nil {
		return fmt.Errorf("dolt add: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf("CALL DOLT_COMMIT('-a', '-m', '%s')", msg)); err != nil {
		return fmt.Errorf("dolt commit: %w", err)
	}
	return nil
}

// tcpMergeBranch merges the given branch into the current branch and returns
// success/failure, detail string, and latency.
func tcpMergeBranch(t *testing.T, db *sql.DB, branch string) (bool, string, time.Duration) {
	t.Helper()
	start := time.Now()
	_, err := db.Exec(fmt.Sprintf("CALL DOLT_MERGE('%s')", branch))
	elapsed := time.Since(start)
	if err != nil {
		return false, err.Error(), elapsed
	}
	return true, "ok", elapsed
}

// tcpObjectExists checks whether an object with the given canonical_id exists.
func tcpObjectExists(t *testing.T, db *sql.DB, id string) bool {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM og_objects WHERE canonical_id = ?", id).Scan(&count); err != nil {
		t.Fatalf("check object exists %s: %v", id, err)
	}
	return count > 0
}

// TestDoltTCPBranchMultiClient validates Dolt branch/merge workflow over a real
// TCP SQL server connection, simulating multiple worker VMs connecting from
// separate clients.
func TestDoltTCPBranchMultiClient(t *testing.T) {
	port, cleanup := startDoltTCPServer(t)
	defer cleanup()

	// ========================================================================
	// TEST 1: Multi-client branch isolation
	// ========================================================================
	t.Log("=== TEST 1: Multi-client branch isolation ===")

	connA := openTCPConn(t, port)
	defer connA.Close()
	connB := openTCPConn(t, port)
	defer connB.Close()

	// Connection A (worker-1 VM): create and checkout worker-1 branch.
	startA := time.Now()
	tcpCheckoutBranch(t, connA, "-b", "worker-1")
	latencyBranchA := time.Since(startA)

	// Connection B (worker-2 VM): create and checkout worker-2 branch.
	startB := time.Now()
	tcpCheckoutBranch(t, connB, "-b", "worker-2")
	latencyBranchB := time.Since(startB)

	t.Logf("Branch create worker-1 (conn A): %v", latencyBranchA)
	t.Logf("Branch create worker-2 (conn B): %v", latencyBranchB)

	// Connection A: insert 25 objects with prefix "w1-".
	startInsA := time.Now()
	tcpInsertObjects(t, connA, "w1", 25)
	latencyInsA := time.Since(startInsA)

	// Connection B: insert 25 objects with prefix "w2-".
	startInsB := time.Now()
	tcpInsertObjects(t, connB, "w2", 25)
	latencyInsB := time.Since(startInsB)

	t.Logf("Insert 25 objects worker-1 (conn A): %v", latencyInsA)
	t.Logf("Insert 25 objects worker-2 (conn B): %v", latencyInsB)

	// Connection A: commit.
	startCommitA := time.Now()
	tcpCommitAll(t, connA, "worker-1 batch")
	latencyCommitA := time.Since(startCommitA)

	// Connection B: commit.
	startCommitB := time.Now()
	tcpCommitAll(t, connB, "worker-2 batch")
	latencyCommitB := time.Since(startCommitB)

	t.Logf("Commit worker-1 (conn A): %v", latencyCommitA)
	t.Logf("Commit worker-2 (conn B): %v", latencyCommitB)

	// Verify branch isolation: each connection should only see its own 25 objects.
	countA := tcpCountObjects(t, connA)
	countB := tcpCountObjects(t, connB)
	t.Logf("Object count on worker-1 (conn A): %d", countA)
	t.Logf("Object count on worker-2 (conn B): %d", countB)
	if countA != 25 {
		t.Errorf("conn A (worker-1) expected 25 objects, got %d", countA)
	}
	if countB != 25 {
		t.Errorf("conn B (worker-2) expected 25 objects, got %d", countB)
	}

	// Connection C (merge VM): checkout main, merge both branches.
	connC := openTCPConn(t, port)
	defer connC.Close()

	tcpCheckoutBranch(t, connC, "main")

	// Merge worker-1.
	ok1, detail1, latencyMerge1 := tcpMergeBranch(t, connC, "worker-1")
	t.Logf("Merge worker-1 into main (conn C): %v (result: %s)", latencyMerge1, detail1)
	if !ok1 {
		t.Fatalf("merge worker-1 failed: %s", detail1)
	}

	// Merge worker-2.
	ok2, detail2, latencyMerge2 := tcpMergeBranch(t, connC, "worker-2")
	t.Logf("Merge worker-2 into main (conn C): %v (result: %s)", latencyMerge2, detail2)
	if !ok2 {
		t.Fatalf("merge worker-2 failed: %s", detail2)
	}

	// Verify main has all 50 objects.
	countMain := tcpCountObjects(t, connC)
	t.Logf("Object count on main after merges (conn C): %d", countMain)
	if countMain != 50 {
		t.Fatalf("expected 50 objects on main, got %d", countMain)
	}

	t.Log("TEST 1 PASSED: multi-client branch isolation confirmed")
	t.Log("  - Each TCP connection has independent branch/session state")
	t.Log("  - Writes on one branch are not visible on another")
	t.Log("  - Merge from a separate connection (merge VM) succeeds")

	// ========================================================================
	// TEST 2: Concurrent writes to same branch
	// ========================================================================
	t.Log("")
	t.Log("=== TEST 2: Concurrent writes to same branch ===")

	// Create worker-3 branch from main using connC.
	startW3Branch := time.Now()
	tcpCheckoutBranch(t, connC, "-b", "worker-3")
	latencyW3Branch := time.Since(startW3Branch)
	tcpCheckoutBranch(t, connC, "main")
	t.Logf("Branch create worker-3 (conn C): %v", latencyW3Branch)

	// Open connections D and E, both checkout worker-3.
	connD := openTCPConn(t, port)
	defer connD.Close()
	connE := openTCPConn(t, port)
	defer connE.Close()

	tcpCheckoutBranch(t, connD, "worker-3")
	tcpCheckoutBranch(t, connE, "worker-3")

	// Connection D inserts 10 objects.
	tcpInsertObjects(t, connD, "w3d", 10)

	// Connection E inserts 10 objects.
	tcpInsertObjects(t, connE, "w3e", 10)

	// Both commit — does this work?
	startCommitD := time.Now()
	commitDErr := tcpCommitAllRaw(t, connD, "worker-3 conn D batch")
	latencyCommitD := time.Since(startCommitD)

	startCommitE := time.Now()
	commitEErr := tcpCommitAllRaw(t, connE, "worker-3 conn E batch")
	latencyCommitE := time.Since(startCommitE)

	t.Logf("Commit conn D (10 objects): %v (err: %v)", latencyCommitD, commitDErr)
	t.Logf("Commit conn E (10 objects): %v (err: %v)", latencyCommitE, commitEErr)

	// Document the behavior.
	if commitDErr == nil && commitEErr == nil {
		t.Log("Both commits succeeded — concurrent writes to same branch work")
	} else if commitDErr != nil && commitEErr == nil {
		t.Logf("Conn D commit failed, conn E succeeded: %v", commitDErr)
	} else if commitDErr == nil && commitEErr != nil {
		t.Logf("Conn D succeeded, conn E commit failed: %v", commitEErr)
	} else {
		t.Logf("Both commits failed: D=%v, E=%v", commitDErr, commitEErr)
	}

	// Check object counts on worker-3 from each connection.
	countD := tcpCountObjects(t, connD)
	countE := tcpCountObjects(t, connE)
	t.Logf("Object count on worker-3 (conn D): %d", countD)
	t.Logf("Object count on worker-3 (conn E): %d", countE)

	// Merge worker-3 into main from connC.
	tcpCheckoutBranch(t, connC, "main")
	ok3, detail3, latencyMerge3 := tcpMergeBranch(t, connC, "worker-3")
	t.Logf("Merge worker-3 into main (conn C): %v (result: %s)", latencyMerge3, detail3)
	if !ok3 {
		t.Logf("Merge worker-3 failed: %s — documenting conflict", detail3)
		var conflictCount int
		if err := connC.QueryRow("SELECT COUNT(*) FROM dolt_conflicts_og_objects").Scan(&conflictCount); err != nil {
			t.Logf("Conflict query error: %v", err)
		} else {
			t.Logf("Conflict count: %d", conflictCount)
		}
	} else {
		countMainAfterW3 := tcpCountObjects(t, connC)
		t.Logf("Object count on main after worker-3 merge: %d", countMainAfterW3)
		t.Logf("Expected 70 objects on main (50 from test 1 + 20 from worker-3), got %d", countMainAfterW3)
	}

	t.Log("TEST 2 COMPLETED: concurrent same-branch behavior documented")

	// ========================================================================
	// TEST 3: Branch isolation verification
	// ========================================================================
	t.Log("")
	t.Log("=== TEST 3: Branch isolation verification ===")

	// Connection A is still on worker-1 branch — insert another object.
	now := time.Now().UTC()
	newID := "w1-extra-99"
	_, err := connA.Exec(
		`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID, "choir.test", "owner-test", "", "", "sha256:fake", []byte("body"),
		`{"seq":99}`, now, now, false, "",
	)
	if err != nil {
		t.Fatalf("insert %s on worker-1: %v", newID, err)
	}
	t.Logf("Inserted %s on worker-1 (conn A, uncommitted)", newID)

	// Query main from connection C — verify the new object is NOT visible.
	existsOnMain := tcpObjectExists(t, connC, newID)
	t.Logf("Object %s visible on main (conn C) before commit: %v", newID, existsOnMain)
	if existsOnMain {
		t.Errorf("uncommitted object from worker-1 should NOT be visible on main")
	}

	// Commit on worker-1.
	tcpCommitAll(t, connA, "worker-1 extra object")
	t.Logf("Committed %s on worker-1", newID)

	// Still should not be on main until merged.
	existsOnMainAfterCommit := tcpObjectExists(t, connC, newID)
	t.Logf("Object %s visible on main (conn C) after commit but before merge: %v",
		newID, existsOnMainAfterCommit)
	if existsOnMainAfterCommit {
		t.Errorf("committed object on worker-1 should NOT be visible on main until merged")
	}

	// Merge worker-1 to main.
	ok4, detail4, latencyMerge4 := tcpMergeBranch(t, connC, "worker-1")
	t.Logf("Merge worker-1 into main (conn C): %v (result: %s)", latencyMerge4, detail4)
	if !ok4 {
		t.Fatalf("merge worker-1 (extra object) failed: %s", detail4)
	}

	// Now verify it appears on main.
	existsOnMainAfterMerge := tcpObjectExists(t, connC, newID)
	t.Logf("Object %s visible on main (conn C) after merge: %v", newID, existsOnMainAfterMerge)
	if !existsOnMainAfterMerge {
		t.Errorf("object from worker-1 should be visible on main after merge")
	}

	// Final count.
	finalCount := tcpCountObjects(t, connC)
	t.Logf("Final object count on main: %d", finalCount)

	t.Log("TEST 3 PASSED: branch isolation verified")
	t.Log("  - Uncommitted writes on worker-1 are not visible on main")
	t.Log("  - Committed but unmerged writes are still not visible on main")
	t.Log("  - After merge, the object appears on main")

	// ========================================================================
	// Summary
	// ========================================================================
	t.Log("")
	t.Log("=== TCP BRANCH TEST SUMMARY ===")
	t.Logf("Total objects on main: %d", finalCount)
	t.Log("Key findings:")
	t.Log("  1. Each TCP connection gets its own session/branch state")
	t.Log("  2. Branch isolation works: writes on one branch are not visible on another")
	t.Log("  3. Merge from a separate connection (merge VM) succeeds")
	t.Log("  4. Concurrent writes to same branch: behavior documented in TEST 2")
}
