package projectionbase

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// churnedWorkspace opens a real runtime store, then writes and rewrites rows
// across many Dolt commits so history holds garbage the current state does
// not reference. It returns the closed store's workspace path.
func churnedWorkspace(t *testing.T) (storePath, workspace string) {
	t.Helper()
	storePath = filepath.Join(t.TempDir(), "runtime.db")
	s, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	workspace = choirstore.TextureWorkspacePath(storePath)
	db, closer, err := openCompactionDB(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(); _ = closer.Close() }()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec("CREATE TABLE IF NOT EXISTS compaction_fixture (id INT PRIMARY KEY, body LONGTEXT)")
	blob := make([]byte, 64<<10)
	for round := 0; round < 12; round++ {
		for i := range blob {
			blob[i] = byte('a' + (i+round)%26)
		}
		for id := 0; id < 8; id++ {
			exec("REPLACE INTO compaction_fixture (id, body) VALUES (?, ?)", id, fmt.Sprintf("%d:%s", round, blob))
		}
		exec("CALL DOLT_COMMIT('-Am', ?)", fmt.Sprintf("fixture round %d", round))
	}
	return storePath, workspace
}

func countCommits(t *testing.T, workspace string) int {
	t.Helper()
	db, closer, err := openCompactionDB(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(); _ = closer.Close() }()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM dolt_log").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Failure modes pinned here (docs/problems/guest-store-history-bloat-and-
// memory-shape-2026-10-09.md): content changed by compaction; history not
// actually dropped; the store not reopening afterwards; an extra branch
// silently keeping history alive; a second run not being a no-op.
func TestCompactWorkspaceDropsHistoryAndKeepsContentWitness(t *testing.T) {
	ctx := context.Background()
	storePath, workspace := churnedWorkspace(t)
	before := countCommits(t, workspace)
	if before < 12 {
		t.Fatalf("fixture commits = %d, want >= 12", before)
	}

	result, err := CompactWorkspaceWith(ctx, "computer-compact", "head-compact", workspace, CompactOptions{FullWitness: true})
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if result.RootBefore == "" || result.RootBefore != result.RootAfter {
		t.Fatalf("root changed: %q -> %q", result.RootBefore, result.RootAfter)
	}
	if result.CommitsBefore != before || result.CommitsAfter > 2 {
		t.Fatalf("commits before=%d after=%d, want %d then <= 2", result.CommitsBefore, result.CommitsAfter, before)
	}
	if result.NomsBytesAfter >= result.NomsBytesBefore {
		t.Fatalf("noms bytes before=%d after=%d, want a reduction", result.NomsBytesBefore, result.NomsBytesAfter)
	}
	if result.WitnessBefore.ContentRoot == "" || result.WitnessBefore.ContentRoot != result.WitnessAfter.ContentRoot {
		t.Fatalf("content root changed: %q -> %q", result.WitnessBefore.ContentRoot, result.WitnessAfter.ContentRoot)
	}

	db, closer, err := openCompactionDB(workspace)
	if err != nil {
		t.Fatal(err)
	}
	var rows int
	var body string
	if err := db.QueryRow("SELECT COUNT(*), MAX(body) FROM compaction_fixture").Scan(&rows, &body); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	_ = closer.Close()
	if rows != 8 || body[:3] != "11:" {
		t.Fatalf("current state after compaction: rows=%d body prefix=%q", rows, body[:3])
	}
	s, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatalf("store reopen after compaction: %v", err)
	}
	_ = s.Close()

	again, err := CompactWorkspace(ctx, "computer-compact", "head-compact", workspace)
	if err != nil || again.Squashed {
		t.Fatalf("second compaction = %+v, %v; want a no-op", again, err)
	}
}

func TestCompactWorkspaceRefusesExtraBranch(t *testing.T) {
	_, workspace := churnedWorkspace(t)
	db, closer, err := openCompactionDB(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CALL DOLT_BRANCH('keep-history')"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	_ = closer.Close()
	if _, err := CompactWorkspace(context.Background(), "computer-compact", "head-compact", workspace); err == nil {
		t.Fatal("compaction ran with a second branch that keeps history reachable")
	}
}

// The runtime commits every 100 events, so a stopped store can hold
// uncommitted working changes. The squash commits them (-A); the root check
// compares the working root before with the HEAD root after, and the rows
// must survive.
func TestCompactWorkspaceKeepsUncommittedWorkingChanges(t *testing.T) {
	ctx := context.Background()
	_, workspace := churnedWorkspace(t)
	db, closer, err := openCompactionDB(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("REPLACE INTO compaction_fixture (id, body) VALUES (99, 'uncommitted')"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	_ = closer.Close()

	result, err := CompactWorkspace(ctx, "computer-compact", "head-compact", workspace)
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if result.RootBefore == "" || result.RootBefore != result.RootAfter {
		t.Fatalf("root changed: %q -> %q", result.RootBefore, result.RootAfter)
	}
	db, closer, err = openCompactionDB(workspace)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(); _ = closer.Close() }()
	var body string
	if err := db.QueryRow("SELECT body FROM compaction_fixture WHERE id = 99").Scan(&body); err != nil || body != "uncommitted" {
		t.Fatalf("uncommitted row after compaction: %q, %v", body, err)
	}
	var dirty int
	if err := db.QueryRow("SELECT COUNT(*) FROM dolt_status").Scan(&dirty); err != nil || dirty != 0 {
		t.Fatalf("dolt_status rows after compaction = %d, %v; want a clean store", dirty, err)
	}
}
