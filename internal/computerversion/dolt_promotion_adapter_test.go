package computerversion

import (
	"context"
	"fmt"
	"database/sql"
	"testing"

	embedded "github.com/dolthub/driver"
)

// openPromotionTestDB creates a temporary Dolt workspace with a database
// and initial data, then returns the workspace path. The fixture closes
// its own connection before returning so the adapter can open its own
// writable connection.
func openPromotionTestDB(t *testing.T, database, ddl string, inserts []string) string {
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
	rootDB.SetMaxOpenConns(1)
	rootDB.SetMaxIdleConns(1)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + database); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=%s&multistatements=true&clientfoundrows=true", root, database)
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

	if ddl != "" {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("apply ddl: %v", err)
		}
	}
	for _, ins := range inserts {
		if _, err := db.Exec(ins); err != nil {
			t.Fatalf("insert: %v\n  query: %s", err, ins)
		}
	}
	if _, err := db.Exec("CALL DOLT_COMMIT('-Am', 'test fixture initial')"); err != nil {
		_ = err // "nothing to commit" is fine
	}

	// Close the fixture connection so the adapter can open a writable one.
	_ = db.Close()
	_ = dbConnector.Close()

	return root
}

// TestPromotionAdapterFork verifies that Fork creates a tag at the current
// HEAD and returns the fork commit hash.
func TestPromotionAdapterFork(t *testing.T) {
	workspace := openPromotionTestDB(t, "promodb",
		"CREATE TABLE items (id INT PRIMARY KEY, name VARCHAR(255) NOT NULL)",
		[]string{"INSERT INTO items (id, name) VALUES (1, 'alpha')"},
	)
	ctx := context.Background()

	adapter := DoltPromotionAdapter{
		WorkspacePath: workspace,
		Database:      "promodb",
	}

	fork, err := adapter.Fork(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if fork.ForkTag == "" {
		t.Error("fork tag is empty")
	}
	if fork.ForkCommit == "" {
		t.Error("fork commit is empty")
	}

	// Verify the fork tag points to the fork commit.
	tagHash, err := adapter.GetTagHash(ctx, fork.ForkTag)
	if err != nil {
		t.Fatalf("get tag hash: %v", err)
	}
	if tagHash != fork.ForkCommit {
		t.Errorf("fork tag hash %q != fork commit %q", tagHash, fork.ForkCommit)
	}

	t.Logf("fork: tag=%s commit=%s", fork.ForkTag, fork.ForkCommit)
}

// TestPromotionAdapterFullCycle tests the complete promotion lifecycle:
// fork → commit (candidate changes) → promote → verify → rollback → verify.
func TestPromotionAdapterFullCycle(t *testing.T) {
	workspace := openPromotionTestDB(t, "promodb",
		"CREATE TABLE items (id INT PRIMARY KEY, name VARCHAR(255) NOT NULL)",
		[]string{"INSERT INTO items (id, name) VALUES (1, 'alpha')"},
	)
	ctx := context.Background()

	adapter := DoltPromotionAdapter{
		WorkspacePath: workspace,
		Database:      "promodb",
	}

	// Step 1: Fork — record the current HEAD as the fork tag.
	fork, err := adapter.Fork(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	t.Logf("step 1 fork: tag=%s commit=%s", fork.ForkTag, fork.ForkCommit)

	// Step 2: Commit candidate changes.
	// We need to open a connection to insert data, then commit via the adapter.
	db, connector, err := openDoltWorkspace(workspace, "promodb", "Choir", "system@choir.local")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO items (id, name) VALUES (2, 'candidate-beta')"); err != nil {
		t.Fatalf("insert candidate data: %v", err)
	}
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}

	commitHash, err := adapter.Commit(ctx, "candidate-1: capsule transaction batch 1")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if commitHash == "" {
		t.Fatal("commit hash is empty")
	}
	t.Logf("step 2 commit: hash=%s", commitHash)

	// Step 3: Promote — tag the current HEAD as the promotion certificate.
	promo, err := adapter.Promote(ctx, "candidate-1", fork.ForkTag)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if promo.PromotionTag == "" {
		t.Error("promotion tag is empty")
	}
	if promo.MergeCommit != commitHash {
		t.Errorf("promotion merge commit %q != last commit %q", promo.MergeCommit, commitHash)
	}
	if promo.ForkTag != fork.ForkTag {
		t.Errorf("promotion fork tag %q != fork tag %q", promo.ForkTag, fork.ForkTag)
	}
	t.Logf("step 3 promote: tag=%s commit=%s fork=%s", promo.PromotionTag, promo.MergeCommit, promo.ForkTag)

	// Step 4: Verify the promotion tag points to the merge commit.
	promoTagHash, err := adapter.GetTagHash(ctx, promo.PromotionTag)
	if err != nil {
		t.Fatalf("get promotion tag hash: %v", err)
	}
	if promoTagHash != promo.MergeCommit {
		t.Errorf("promotion tag hash %q != merge commit %q", promoTagHash, promo.MergeCommit)
	}
	t.Logf("step 4 verify: promotion tag points to %s", promoTagHash)

	// Step 5: Verify the data has 2 rows (candidate data is present).
	db, connector, err = openDoltWorkspace(workspace, "promodb", "Choir", "system@choir.local")
	if err != nil {
		t.Fatalf("open db for verify: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatalf("count after promote: %v", err)
	}
	if count != 2 {
		t.Errorf("post-promote count: expected 2, got %d", count)
	}
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}
	t.Logf("step 5 verify: %d rows after promotion", count)

	// Step 6: Rollback — reset to the fork tag.
	if err := adapter.Rollback(ctx, fork.ForkTag); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	t.Logf("step 6 rollback: reset to fork tag %s", fork.ForkTag)

	// Step 7: Verify the data has 1 row (candidate data is gone).
	db, connector, err = openDoltWorkspace(workspace, "promodb", "Choir", "system@choir.local")
	if err != nil {
		t.Fatalf("open db for post-rollback verify: %v", err)
	}
	var postRollbackCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&postRollbackCount); err != nil {
		t.Fatalf("count after rollback: %v", err)
	}
	if postRollbackCount != 1 {
		t.Errorf("post-rollback count: expected 1, got %d", postRollbackCount)
	} else {
		t.Logf("step 7 verify: %d rows after rollback (candidate data removed)", postRollbackCount)
	}
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}

	// Step 8: Verify the fork tag still points to the original fork commit.
	forkTagHash, err := adapter.GetTagHash(ctx, fork.ForkTag)
	if err != nil {
		t.Fatalf("get fork tag hash after rollback: %v", err)
	}
	if forkTagHash != fork.ForkCommit {
		t.Errorf("fork tag hash after rollback %q != original fork commit %q", forkTagHash, fork.ForkCommit)
	}
	t.Logf("step 8 verify: fork tag still points to %s", forkTagHash)
}

// TestPromotionAdapterRollbackToNonexistentTag verifies that rollback fails
// if the fork tag doesn't exist.
func TestPromotionAdapterRollbackToNonexistentTag(t *testing.T) {
	workspace := openPromotionTestDB(t, "promodb", "", nil)
	ctx := context.Background()

	adapter := DoltPromotionAdapter{
		WorkspacePath: workspace,
		Database:      "promodb",
	}

	err := adapter.Rollback(ctx, "fork-nonexistent-12345")
	if err == nil {
		t.Fatal("expected error for nonexistent fork tag, got nil")
	}
}

// TestPromotionAdapterGetTagHashNonexistent verifies that GetTagHash fails
// for a nonexistent tag.
func TestPromotionAdapterGetTagHashNonexistent(t *testing.T) {
	workspace := openPromotionTestDB(t, "promodb", "", nil)
	ctx := context.Background()

	adapter := DoltPromotionAdapter{
		WorkspacePath: workspace,
		Database:      "promodb",
	}

	_, err := adapter.GetTagHash(ctx, "nonexistent-tag")
	if err == nil {
		t.Fatal("expected error for nonexistent tag, got nil")
	}
}

// TestPromotionAdapterRejectsEmptyInputs verifies input validation.
func TestPromotionAdapterRejectsEmptyInputs(t *testing.T) {
	workspace := openPromotionTestDB(t, "promodb", "", nil)
	ctx := context.Background()

	adapter := DoltPromotionAdapter{
		WorkspacePath: workspace,
		Database:      "promodb",
	}

	// Fork with empty candidate ID.
	if _, err := adapter.Fork(ctx, ""); err == nil {
		t.Error("expected error for empty candidate ID in Fork")
	}

	// Commit with empty message.
	if _, err := adapter.Commit(ctx, ""); err == nil {
		t.Error("expected error for empty message in Commit")
	}

	// Promote with empty candidate ID.
	if _, err := adapter.Promote(ctx, "", "fork-tag"); err == nil {
		t.Error("expected error for empty candidate ID in Promote")
	}

	// Promote with empty fork tag.
	if _, err := adapter.Promote(ctx, "candidate-1", ""); err == nil {
		t.Error("expected error for empty fork tag in Promote")
	}

	// Rollback with empty fork tag.
	if err := adapter.Rollback(ctx, ""); err == nil {
		t.Error("expected error for empty fork tag in Rollback")
	}

	// GetTagHash with empty tag name.
	if _, err := adapter.GetTagHash(ctx, ""); err == nil {
		t.Error("expected error for empty tag name in GetTagHash")
	}
}

// TestPromotionAdapterMultipleCommits verifies that multiple capsule
// transaction batches can be committed before promotion.
func TestPromotionAdapterMultipleCommits(t *testing.T) {
	workspace := openPromotionTestDB(t, "promodb",
		"CREATE TABLE items (id INT PRIMARY KEY, name VARCHAR(255) NOT NULL)",
		[]string{"INSERT INTO items (id, name) VALUES (1, 'alpha')"},
	)
	ctx := context.Background()

	adapter := DoltPromotionAdapter{
		WorkspacePath: workspace,
		Database:      "promodb",
	}

	// Fork.
	fork, err := adapter.Fork(ctx, "candidate-1")
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	// Multiple commit cycles (simulating multiple capsule transaction batches).
	for i := 2; i <= 4; i++ {
		db, connector, err := openDoltWorkspace(workspace, "promodb", "Choir", "system@choir.local")
		if err != nil {
			t.Fatalf("open db for commit %d: %v", i, err)
		}
		insert := fmt.Sprintf("INSERT INTO items (id, name) VALUES (%d, 'item-%d')", i, i)
		if _, err := db.ExecContext(ctx, insert); err != nil {
			t.Fatalf("insert item %d: %v", i, err)
		}
		_ = db.Close()
		if connector != nil {
			_ = connector.Close()
		}

		msg := fmt.Sprintf("candidate-1: capsule transaction batch %d", i-1)
		hash, err := adapter.Commit(ctx, msg)
		if err != nil {
			t.Fatalf("commit %d: %v", i, err)
		}
		t.Logf("commit %d: hash=%s", i, hash)
	}

	// Promote.
	promo, err := adapter.Promote(ctx, "candidate-1", fork.ForkTag)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	t.Logf("promote: tag=%s commit=%s", promo.PromotionTag, promo.MergeCommit)

	// Verify 4 rows.
	db, connector, err := openDoltWorkspace(workspace, "promodb", "Choir", "system@choir.local")
	if err != nil {
		t.Fatalf("open db for verify: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 rows, got %d", count)
	}
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}

	// Rollback.
	if err := adapter.Rollback(ctx, fork.ForkTag); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Verify 1 row after rollback.
	db, connector, err = openDoltWorkspace(workspace, "promodb", "Choir", "system@choir.local")
	if err != nil {
		t.Fatalf("open db for post-rollback: %v", err)
	}
	var postCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items").Scan(&postCount); err != nil {
		t.Fatalf("post-rollback count: %v", err)
	}
	if postCount != 1 {
		t.Errorf("post-rollback: expected 1 row, got %d", postCount)
	}
	_ = db.Close()
	if connector != nil {
		_ = connector.Close()
	}

	t.Logf("multiple commits + promote + rollback: verified (4 → 1 after rollback)")
}
