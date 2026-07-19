package capsule

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyImmutableSourceTreePinsTrackedCleanFiles(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "snapshot")
	mustRunGit(t, source, "init")
	mustRunGit(t, source, "config", "user.name", "Capsule Test")
	mustRunGit(t, source, "config", "user.email", "capsule@test.invalid")
	if err := os.MkdirAll(filepath.Join(source, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "cmd", "run"), []byte("#!/bin/sh\necho pinned\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "README"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("README", filepath.Join(source, "CURRENT")); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, source, "add", ".")
	mustRunGit(t, source, "commit", "-m", "fixture")
	if err := os.WriteFile(filepath.Join(source, ".env.local"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	digest, err := copyImmutableSourceTree(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 {
		t.Fatalf("digest length = %d, want 64", len(digest))
	}
	if _, err := os.Stat(filepath.Join(target, ".env.local")); !os.IsNotExist(err) {
		t.Fatalf("untracked secret entered snapshot: %v", err)
	}
	if raw, err := os.ReadFile(filepath.Join(target, "cmd", "run")); err != nil || string(raw) != "#!/bin/sh\necho pinned\n" {
		t.Fatalf("copied executable = %q, %v", raw, err)
	}
	if info, err := os.Stat(filepath.Join(target, "cmd", "run")); err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("executable mode = %v, %v", info, err)
	}
	if link, err := os.Readlink(filepath.Join(target, "CURRENT")); err != nil || link != "README" {
		t.Fatalf("copied symlink = %q, %v", link, err)
	}
}

func TestCopyImmutableSourceTreeRefusesDirtyTrackedFiles(t *testing.T) {
	source := t.TempDir()
	mustRunGit(t, source, "init")
	mustRunGit(t, source, "config", "user.name", "Capsule Test")
	mustRunGit(t, source, "config", "user.email", "capsule@test.invalid")
	path := filepath.Join(source, "tracked")
	if err := os.WriteFile(path, []byte("clean"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, source, "add", "tracked")
	mustRunGit(t, source, "commit", "-m", "fixture")
	if err := os.WriteFile(path, []byte("dirty"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := copyImmutableSourceTree(source, filepath.Join(t.TempDir(), "snapshot"))
	if err == nil || !strings.Contains(err.Error(), "dirty tracked files") {
		t.Fatalf("dirty source error = %v", err)
	}
}

func mustRunGit(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
}
