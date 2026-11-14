package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain lets a test run the real main() in a child process: the dispatch of
// the `prune` subcommand happens there, before any signer or socket is set up.
func TestMain(m *testing.M) {
	if os.Getenv("CHOIR_UPDATER_TEST_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func staleRoot(t *testing.T) (root, stale string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "updater")
	for _, dir := range []string{"releases", "operations", "incoming", "store", "gc-roots"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	stale = filepath.Join(root, "incoming", ".stage-old")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "f"), bytes.Repeat([]byte("x"), 8192), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, p := range []string{stale, filepath.Join(stale, "f")} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return root, stale
}

// The prune runs before the signer, the network and the runtime exist: the
// real binary entry, with no signer socket, no identity, no flags of the
// daemon, prunes and exits 0 with one line.
func TestPruneSubcommandRunsWithNoSignerSocketOrIdentity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("main dispatch is the linux binary's")
	}
	root, stale := staleRoot(t)
	cmd := exec.Command(os.Args[0], "prune", "--root", root, "--budget", "30s")
	cmd.Env = []string{"CHOIR_UPDATER_TEST_RUN_MAIN=1", "PATH=" + os.Getenv("PATH")}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("prune exited with %v: %s", err, stderr.String())
	}
	if _, err := os.Lstat(stale); err == nil {
		t.Fatal("the stale temporary survived")
	}
	lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "updater: prune:") || !strings.Contains(lines[0], "bytes_freed=8192") {
		t.Fatalf("output = %q", stderr.String())
	}
}

func TestPruneMainUsageAndRootErrorsExitTwoAndNeverTouchAnything(t *testing.T) {
	root, stale := staleRoot(t)
	var out bytes.Buffer
	for _, args := range [][]string{
		{},
		{"--root", ""},
		{"--root", root, "--budget", "0s"},
		{"--root", root, "extra"},
		{"--root", filepath.Join(root, "missing")},
		{"--root", filepath.Join(root, "incoming", ".stage-old", "f")},
		{"--nope"},
	} {
		out.Reset()
		if code := pruneMain(args, &out); code != 2 {
			t.Fatalf("pruneMain(%v) = %d, want 2 (%s)", args, code, out.String())
		}
	}
	if _, err := os.Lstat(stale); err != nil {
		t.Fatal("a usage error pruned")
	}
}

func TestPruneMainSkipsOnABrokenJournalAndStillExitsZero(t *testing.T) {
	root, stale := staleRoot(t)
	if err := os.WriteFile(filepath.Join(root, "operations", "broken.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := pruneMain([]string{"--root", root}, &out); code != 0 {
		t.Fatalf("a skipped prune exited %d: %s", code, out.String())
	}
	if _, err := os.Lstat(stale); err != nil {
		t.Fatal("pruned under an unreadable journal")
	}
	if !strings.Contains(out.String(), "skipped (journal broken.json cannot be read)") {
		t.Fatalf("output = %q", out.String())
	}
}
