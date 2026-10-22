package platform

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/doltbatch"
)

// Store A's snapshot switch (PLATFORM_DOLT_SNAPSHOTS; docs/problems/
// platform-dolt-oldgen-rate-and-squash-window-2026-10-22.md). Failure modes
// pinned: F1 Store A still commits with the switch off; F4 the switch leaks
// into Store B's committer when the pools are separate; F5 a recovery
// boundary write fails because nothing was committed.

func storeALogCount(t *testing.T, s *Store) int {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM dolt_log").Scan(&n); err != nil {
		t.Fatalf("count dolt_log: %v", err)
	}
	return n
}

func TestStoreASnapshotsOffNeverCommits(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	s.useStoreASnapshotMode(doltbatch.ModeOff)
	base := storeALogCount(t, s)

	ctx := context.Background()
	if err := s.RecordReplayWatermark(ctx, "computer-snapshot-off", 1, "sha256:aa"); err != nil {
		t.Fatalf("record replay watermark with snapshots off: %v", err) // F5
	}
	s.markDirty("debounced write")
	if err := s.committer.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got := storeALogCount(t, s); got != base {
		t.Fatalf("dolt_log = %d, want %d: Store A must not DOLT_COMMIT with snapshots off", got, base) // F1
	}
	if got := platformTableCount(t, s, "computer_replay_watermarks"); got != 1 {
		t.Fatalf("watermark rows = %d, want 1", got)
	}
}

func TestStoreASnapshotsCommitByDefault(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	s.useStoreASnapshotMode(doltbatch.ModeCommit)
	base := storeALogCount(t, s)
	if err := s.RecordReplayWatermark(context.Background(), "computer-snapshot-on", 1, "sha256:aa"); err != nil {
		t.Fatalf("record replay watermark: %v", err)
	}
	if got := storeALogCount(t, s); got != base+1 {
		t.Fatalf("dolt_log = %d, want %d: the boundary commit is the default", got, base+1)
	}
}

func TestStoreASnapshotModeLeavesStoreBCommitter(t *testing.T) {
	s, _ := openTestPlatformStore(t)
	// Separate pools: Store B has its own committer (openStore with a corpus DSN).
	corpus := doltbatch.New(s.db, "platform-corpus", 0)
	s.corpusCommitter = corpus
	s.useStoreASnapshotMode(doltbatch.ModeOff)
	if s.corpusCommitter != corpus || s.corpusCommitter.Mode() != doltbatch.ModeCommit {
		t.Fatalf("Store B committer changed by Store A's switch") // F4
	}
	if s.committer.Mode() != doltbatch.ModeOff {
		t.Fatalf("Store A committer mode = %q, want off", s.committer.Mode())
	}

	// Shared pool (no corpus DSN): one database, one committer, so both follow.
	shared, _ := openTestPlatformStore(t)
	shared.useStoreASnapshotMode(doltbatch.ModeOff)
	if shared.corpusCommitter != shared.committer {
		t.Fatalf("shared pool: corpus committer must stay the Store A committer")
	}
}

// Review round 1 R3: the env seam. Each mutant must fail a test: a wrong env
// name (P2), an unknown value that turns snapshots off (P3), openStore not
// applying the env (P1). The name is the literal string, not the constant.
func TestStoreASnapshotEnvSeam(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want doltbatch.Mode
	}{
		{map[string]string{}, doltbatch.ModeCommit},
		{map[string]string{"PLATFORM_DOLT_SNAPSHOTS": "off"}, doltbatch.ModeOff},
		{map[string]string{"PLATFORM_DOLT_SNAPSHOTS": "  OFF "}, doltbatch.ModeOff},
		{map[string]string{"PLATFORM_DOLT_SNAPSHOTS": "commit"}, doltbatch.ModeCommit},
		{map[string]string{"PLATFORM_DOLT_SNAPSHOT": "off"}, doltbatch.ModeCommit}, // P2: another name does nothing
		{map[string]string{"DOLT_SNAPSHOTS": "off"}, doltbatch.ModeCommit},
	}
	for _, typo := range []string{"of", "false", "0", "disabled", "none", `"off"`, "off # no"} {
		cases = append(cases, struct {
			env  map[string]string
			want doltbatch.Mode
		}{map[string]string{"PLATFORM_DOLT_SNAPSHOTS": typo}, doltbatch.ModeCommit}) // P3
	}
	for _, tc := range cases {
		s, _ := openTestPlatformStore(t)
		got := s.applyStoreASnapshotEnv(func(k string) string { return tc.env[k] })
		if got != tc.want || s.committer.Mode() != tc.want {
			t.Errorf("env %v: applied %q, committer %q; want %q", tc.env, got, s.committer.Mode(), tc.want)
		}
	}
}

// P1: openStore applies the process environment. Read from the source, so the
// guard runs where no dolt sql-server is available (CI); the server-backed
// test below checks the same thing end to end when the dolt CLI is present.
func TestOpenStoreAppliesSnapshotEnvInSource(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "store.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "openStore" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "applyStoreASnapshotEnv" || len(call.Args) != 1 {
				return true
			}
			if arg, ok := call.Args[0].(*ast.SelectorExpr); ok && arg.Sel.Name == "Getenv" {
				if pkg, ok := arg.X.(*ast.Ident); ok && pkg.Name == "os" {
					found = true
				}
			}
			return true
		})
	}
	if !found {
		t.Fatal("openStore does not call s.applyStoreASnapshotEnv(os.Getenv)")
	}
}

func TestOpenStoreAppliesSnapshotEnvAgainstServer(t *testing.T) {
	dolt, err := exec.LookPath("dolt")
	if err != nil {
		t.Skip("dolt CLI not available")
	}
	parent := t.TempDir()
	repo := filepath.Join(parent, "platform")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(dolt, args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "DOLT_ROOT_PATH="+parent)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("dolt %v: %v\n%s", args, err, out)
		}
	}
	run("config", "--global", "--add", "user.name", "Choir Platform")
	run("config", "--global", "--add", "user.email", "platform@choir.news")
	run("init")
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	server := exec.Command(dolt, "sql-server", "--host", "127.0.0.1", "--port", fmt.Sprint(port))
	server.Dir = repo
	server.Env = append(os.Environ(), "DOLT_ROOT_PATH="+parent)
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Process.Signal(os.Interrupt); _ = server.Wait() }()
	dsn := fmt.Sprintf("root@tcp(127.0.0.1:%d)/platform?parseTime=true&multiStatements=true&clientFoundRows=true", port)

	open := func() *Store {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			s, err := OpenStoreWithoutBootstrap(ctx, dsn, "")
			cancel()
			if err == nil {
				return s
			}
			if time.Now().After(deadline) {
				t.Fatalf("open store: %v", err)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	t.Setenv("PLATFORM_DOLT_SNAPSHOTS", "off")
	s := open()
	if s.committer.Mode() != doltbatch.ModeOff || s.corpusCommitter != s.committer {
		t.Fatalf("PLATFORM_DOLT_SNAPSHOTS=off: committer %q (shared %v); want off, shared", s.committer.Mode(), s.corpusCommitter == s.committer)
	}
	_ = s.Close()
	t.Setenv("PLATFORM_DOLT_SNAPSHOTS", "of")
	s = open()
	if s.committer.Mode() != doltbatch.ModeCommit {
		t.Fatalf("PLATFORM_DOLT_SNAPSHOTS=of: committer %q; want commit", s.committer.Mode())
	}
	_ = s.Close()
}
