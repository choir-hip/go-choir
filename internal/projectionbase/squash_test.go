package projectionbase

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Failure modes pinned here (docs/problems/store-a-weekly-squash-2026-10-20.md):
// FM1 root hash changes; FM2 a branch, tag, remote or stash keeps history
// reachable and the refusal must leave the original untouched; FM3 state a
// commit cannot hold (dolt_ignore); FM4 history not dropped; FM6 temp output
// outside the store; the uncommitted working set must survive. The store is
// laid out as on Node B: a parent directory (the sql-server's HOME, holding a
// global .dolt config) with the repository in <parent>/platform.

const storeADatabase = "platform"

// storeAFixture builds <parent>/platform with churned history. When the dolt
// CLI is on PATH it uses it for `dolt init`, as platform-dolt-init does on
// Node B (global config under <parent>/.dolt); otherwise it creates the
// database through the embedded driver.
func storeAFixture(t *testing.T) (parent string) {
	t.Helper()
	parent = t.TempDir()
	// STORE_A_TEST_NO_DOLT_CLI=1 exercises the embedded-only fixture path.
	if dolt, err := exec.LookPath("dolt"); err == nil && os.Getenv("STORE_A_TEST_NO_DOLT_CLI") == "" {
		repo := filepath.Join(parent, storeADatabase)
		if err := os.MkdirAll(repo, 0o750); err != nil {
			t.Fatal(err)
		}
		run := func(dir string, args ...string) {
			t.Helper()
			cmd := exec.Command(dolt, args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "DOLT_ROOT_PATH="+parent)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("dolt %v: %v\n%s", args, err, out)
			}
		}
		run(repo, "config", "--global", "--add", "user.name", "Choir Platform")
		run(repo, "config", "--global", "--add", "user.email", "platform@choir.news")
		run(repo, "init")
	} else {
		db, closer, err := openDoltDB(parent, "", "fixture")
		if err == nil {
			_, err = db.Exec("CREATE DATABASE " + storeADatabase)
			_ = db.Close()
			_ = closer.Close()
		}
		if err != nil {
			t.Fatalf("create database: %v", err)
		}
	}
	db, closer, err := openDoltDB(parent, storeADatabase, "fixture")
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
	exec("CREATE TABLE computer_event_heads (computer_id VARCHAR(64) PRIMARY KEY, head_seq BIGINT, head LONGTEXT)")
	exec("CREATE TABLE og_objects (id INT PRIMARY KEY, body LONGTEXT)")
	exec("CALL DOLT_COMMIT('-Am', 'schema')")
	blob := make([]byte, 64<<10)
	for round := 0; round < 20; round++ {
		for i := range blob {
			blob[i] = byte('a' + (i+round)%26)
		}
		for id := 0; id < 6; id++ {
			exec("REPLACE INTO og_objects (id, body) VALUES (?, ?)", id, fmt.Sprintf("%d:%s", round, blob))
		}
		exec("REPLACE INTO computer_event_heads VALUES ('computer-a', ?, ?)", round, fmt.Sprintf("head-%d", round))
		exec("CALL DOLT_COMMIT('-Am', ?)", fmt.Sprintf("snapshot %d", round))
	}
	return parent
}

func storeAExec(t *testing.T, parent string, queries ...string) {
	t.Helper()
	db, closer, err := openDoltDB(parent, storeADatabase, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(); _ = closer.Close() }()
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
}

func storeAScalar(t *testing.T, parent, query string) string {
	t.Helper()
	db, closer, err := openDoltDB(parent, storeADatabase, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close(); _ = closer.Close() }()
	var v string
	if err := db.QueryRow(query).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

// treeDigest hashes every regular file (path, size, bytes) below dir.
func treeDigest(t *testing.T, dir string) string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		rel, _ := filepath.Rel(dir, p)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(h, "%s\x00%d\x00", rel, len(b))
		h.Write(b)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func squashOptions(parent string) SquashOptions {
	return SquashOptions{Dir: parent, Database: storeADatabase, Label: "store a squash"}
}

func TestSquashAndGCKeepsRootHashAndDropsHistory(t *testing.T) {
	parent := storeAFixture(t)
	commits := storeAScalar(t, parent, "SELECT COUNT(*) FROM dolt_log")
	if commits != "22" {
		t.Fatalf("fixture commits = %s, want 22 (init + schema + 20 rounds)", commits)
	}
	bytesBefore := workspaceBytes(parent)
	cleanup, err := UseStoreTempDir(parent, storeADatabase)
	if err != nil {
		t.Fatal(err)
	}
	result, err := SquashAndGC(context.Background(), squashOptions(parent))
	cleanup()
	if err != nil {
		t.Fatalf("squash: %v", err)
	}
	if !result.Squashed || result.CommitsBefore != 22 || result.CommitsAfter > 2 {
		t.Fatalf("result = %+v, want squashed from 22 to <= 2 commits", result)
	}
	if result.RootBefore == "" || result.RootBefore != result.RootAfter {
		t.Fatalf("root changed: %q -> %q", result.RootBefore, result.RootAfter)
	}
	if got := storeAScalar(t, parent, "SELECT dolt_hashof_db('WORKING')"); got != result.RootBefore {
		t.Fatalf("working root after reopen = %s, want %s", got, result.RootBefore)
	}
	if got := storeAScalar(t, parent, "SELECT COUNT(*) FROM dolt_log"); got != "1" && got != "2" {
		t.Fatalf("commits after = %s", got)
	}
	if got := storeAScalar(t, parent, "SELECT head FROM computer_event_heads WHERE computer_id = 'computer-a'"); got != "head-19" {
		t.Fatalf("event head row after squash = %q", got)
	}
	if got := storeAScalar(t, parent, "SELECT COUNT(*) FROM og_objects"); got != "6" {
		t.Fatalf("og_objects rows after squash = %s", got)
	}
	if bytesAfter := workspaceBytes(parent); bytesAfter >= bytesBefore {
		t.Fatalf("store bytes before=%d after=%d, want a reduction", bytesBefore, bytesAfter)
	}
	// FM6: the GC's temp output lived in the store, and nothing is left of it
	// (Dolt itself recreates an empty .dolt/temptf on open).
	if entries, err := os.ReadDir(StoreTempDir(parent, storeADatabase)); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	} else if len(entries) != 0 {
		t.Fatalf("store temp dir holds %d leftovers after cleanup", len(entries))
	}
	// A second run is a no-op that still keeps the root.
	again, err := SquashAndGC(context.Background(), squashOptions(parent))
	if err != nil || again.Squashed || again.RootBefore != result.RootBefore {
		t.Fatalf("second run = %+v, %v; want a no-op with the same root", again, err)
	}
}

// A refusal before the squash leaves the original untouched: same files,
// same bytes, same history.
func TestSquashAndGCRefusesRefsThatKeepHistory(t *testing.T) {
	remoteDir := t.TempDir()
	cases := []struct {
		name  string
		setup string
	}{
		{"extra branch", "CALL DOLT_BRANCH('keep-history')"},
		{"tag", "CALL DOLT_TAG('v1')"},
		{"remote", "CALL DOLT_REMOTE('add', 'origin', 'file://" + remoteDir + "')"},
		{"stash", "REPLACE INTO og_objects VALUES (77, 'stashed'); CALL DOLT_STASH('push', 's1')"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := storeAFixture(t)
			storeAExec(t, parent, tc.setup)
			// Settle any files the setup or a first open touches, so the
			// digest compares the refusal's own effect.
			before := treeDigest(t, parent)
			commitsBefore := storeAScalar(t, parent, "SELECT COUNT(*) FROM dolt_log")
			settled := treeDigest(t, parent)
			if settled != before {
				t.Logf("a read-only open changes the tree digest; comparing against the settled state")
				before = settled
			}
			_, err := SquashAndGC(context.Background(), squashOptions(parent))
			if err == nil {
				t.Fatal("squash ran with a ref that keeps history reachable")
			}
			if !IsRefusedBeforeMutation(err) {
				t.Fatalf("err = %v, want a refusal before mutation", err)
			}
			var refused *RefusedError
			if !errors.As(err, &refused) || refused.Reason == "" {
				t.Fatalf("refusal has no reason: %v", err)
			}
			if after := treeDigest(t, parent); after != before {
				t.Fatalf("a refusal changed the store's files")
			}
			if got := storeAScalar(t, parent, "SELECT COUNT(*) FROM dolt_log"); got != commitsBefore {
				t.Fatalf("commits %s -> %s after a refusal", commitsBefore, got)
			}
		})
	}
}

// State a commit cannot hold keeps HEAD and WORKING roots apart for good. The
// squash refuses before the GC, so the expensive step never runs on it. (The
// squash commit has been made by then; the caller's backup restores.)
func TestSquashAndGCRefusesIgnoredTablesBeforeGC(t *testing.T) {
	parent := storeAFixture(t)
	storeAExec(t, parent,
		"CREATE TABLE ignored_projection (id INT PRIMARY KEY)",
		"INSERT INTO dolt_ignore VALUES ('ignored_projection', true)",
		"INSERT INTO ignored_projection VALUES (1)",
		"CALL DOLT_COMMIT('-Am', 'ignore config')",
	)
	_, err := SquashAndGC(context.Background(), squashOptions(parent))
	if err == nil {
		t.Fatal("squash passed with an ignored table that HEAD cannot hold")
	}
	if IsRefusedBeforeMutation(err) {
		t.Fatalf("err = %v: this refusal comes after the squash commit and must not claim the store is untouched", err)
	}
	// The data is intact even so: the working root still has the row.
	if got := storeAScalar(t, parent, "SELECT COUNT(*) FROM ignored_projection"); got != "1" {
		t.Fatalf("ignored table rows = %s", got)
	}
}

// The runtime commits on a debounce, so a stopped store can hold uncommitted
// working changes; the squash commits them and the rows survive.
func TestSquashAndGCKeepsUncommittedWorkingChanges(t *testing.T) {
	parent := storeAFixture(t)
	storeAExec(t, parent, "REPLACE INTO og_objects VALUES (99, 'uncommitted')")
	result, err := SquashAndGC(context.Background(), squashOptions(parent))
	if err != nil {
		t.Fatalf("squash: %v", err)
	}
	if result.RootBefore != result.RootAfter {
		t.Fatalf("root changed: %q -> %q", result.RootBefore, result.RootAfter)
	}
	if got := storeAScalar(t, parent, "SELECT body FROM og_objects WHERE id = 99"); got != "uncommitted" {
		t.Fatalf("uncommitted row after squash: %q", got)
	}
	if got := storeAScalar(t, parent, "SELECT COUNT(*) FROM dolt_status"); got != "0" {
		t.Fatalf("dolt_status rows = %s; want a clean store", got)
	}
}

// FM7: a store a dolt sql-server has open is refused before anything is
// touched, using the real server for the signal (the info file it writes).
func TestSquashAndGCRefusesWhileServerHoldsStore(t *testing.T) {
	dolt, err := exec.LookPath("dolt")
	if err != nil || os.Getenv("STORE_A_TEST_NO_DOLT_CLI") != "" {
		t.Skip("dolt CLI not available")
	}
	parent := storeAFixture(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	cmd := exec.Command(dolt, "sql-server", "--host", "127.0.0.1", "--port", fmt.Sprint(port))
	cmd.Dir = filepath.Join(parent, storeADatabase)
	cmd.Env = append(os.Environ(), "DOLT_ROOT_PATH="+parent)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Signal(os.Interrupt); _ = cmd.Wait() }()
	info := filepath.Join(cmd.Dir, ".dolt", "sql-server.info")
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(info); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sql-server never wrote sql-server.info")
		}
		time.Sleep(100 * time.Millisecond)
	}
	before := treeDigest(t, filepath.Join(parent, storeADatabase, ".dolt", "noms"))
	_, err = SquashAndGC(context.Background(), squashOptions(parent))
	if !IsRefusedBeforeMutation(err) {
		t.Fatalf("squash beside a running server = %v, want a refusal before mutation", err)
	}
	if after := treeDigest(t, filepath.Join(parent, storeADatabase, ".dolt", "noms")); after != before {
		t.Fatal("a refusal beside a running server changed the store")
	}
}

// A missing database is an error that changes nothing (no database is created).
func TestSquashAndGCMissingDatabase(t *testing.T) {
	parent := storeAFixture(t)
	before := treeDigest(t, parent)
	opts := squashOptions(parent)
	opts.Database = "nonexistent"
	if _, err := SquashAndGC(context.Background(), opts); err == nil {
		t.Fatal("squash of a missing database succeeded")
	}
	if after := treeDigest(t, parent); after != before {
		t.Fatal("a missing database changed the store's files")
	}
}

// Review round 1 R1 (docs/problems/platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md):
// with Store A's snapshots off, HEAD never moves after a squash, so the next
// run meets at most two commits and a working set ahead of HEAD. It must
// commit that working set, keep the root and leave at most two commits, not
// refuse after mutation (which the script answers with a full restore).
func TestSquashAndGCSquashesAgainWithSnapshotsOff(t *testing.T) {
	parent := storeAFixture(t)
	first, err := SquashAndGC(context.Background(), squashOptions(parent))
	if err != nil {
		t.Fatalf("first squash: %v", err)
	}
	if first.CommitsAfter > 2 {
		t.Fatalf("first squash left %d commits", first.CommitsAfter)
	}

	// Snapshots off: writes reach the working set and no commit is made.
	storeAExec(t, parent,
		"REPLACE INTO og_objects VALUES (100, 'after the first squash')",
		"REPLACE INTO computer_event_heads VALUES ('computer-a', 21, 'head-21')")
	second, err := SquashAndGC(context.Background(), squashOptions(parent))
	if err != nil {
		t.Fatalf("second squash with the working set ahead of HEAD: %v (refused before mutation: %v)", err, IsRefusedBeforeMutation(err))
	}
	if second.RootBefore != second.RootAfter {
		t.Fatalf("root changed: %q -> %q", second.RootBefore, second.RootAfter)
	}
	if !second.Squashed {
		t.Fatal("the second squash did not commit the working set")
	}
	if second.CommitsAfter > 2 {
		t.Fatalf("second squash left %d commits; want at most 2", second.CommitsAfter)
	}
	if got := storeAScalar(t, parent, "SELECT body FROM og_objects WHERE id = 100"); got != "after the first squash" {
		t.Fatalf("row written after the first squash: %q", got)
	}
	if got := storeAScalar(t, parent, "SELECT COUNT(*) FROM dolt_status"); got != "0" {
		t.Fatalf("dolt_status rows = %s; want HEAD to hold the working set", got)
	}

	// A third run on a clean store with two commits changes nothing and succeeds.
	third, err := SquashAndGC(context.Background(), squashOptions(parent))
	if err != nil {
		t.Fatalf("third squash on a clean store: %v", err)
	}
	if third.RootBefore != third.RootAfter || third.CommitsAfter > 2 {
		t.Fatalf("third squash: root %q -> %q, commits %d", third.RootBefore, third.RootAfter, third.CommitsAfter)
	}
}
