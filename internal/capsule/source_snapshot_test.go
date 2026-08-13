package capsule

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCanonicalSubjectDigestCoversCompleteReconstructableTreeOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace", "platform")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := digestCanonicalSubjectTree(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(root)), "outside-cache"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside, err := digestCanonicalSubjectTree(context.Background(), root)
	if err != nil || outside != first {
		t.Fatalf("outside mutation changed subject: %q %q %v", first, outside, err)
	}
	if err := os.Chmod(filepath.Join(root, "file"), 0o755); err != nil {
		t.Fatal(err)
	}
	modeChanged, _ := digestCanonicalSubjectTree(context.Background(), root)
	if modeChanged == first {
		t.Fatal("executable mode change absent from subject digest")
	}
	if err := os.Chmod(filepath.Join(root, "file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "empty")); err != nil {
		t.Fatal(err)
	}
	emptyRemoved, _ := digestCanonicalSubjectTree(context.Background(), root)
	if emptyRemoved == first {
		t.Fatal("empty directory absent from complete-tree digest")
	}
	if err := os.WriteFile(filepath.Join(root, "file"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	contentChanged, _ := digestCanonicalSubjectTree(context.Background(), root)
	if contentChanged == first {
		t.Fatal("file content absent from subject digest")
	}
}

func TestCanonicalSubjectCopyDeterministicAndRejectsEscapingSymlink(t *testing.T) {
	source := filepath.Join(t.TempDir(), "subject")
	if err := os.MkdirAll(filepath.Join(source, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "dir", "file"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("dir/file", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	if err := copyCanonicalSubjectTree(context.Background(), source, first); err != nil {
		t.Fatal(err)
	}
	if err := copyCanonicalSubjectTree(context.Background(), source, second); err != nil {
		t.Fatal(err)
	}
	one, _ := digestCanonicalSubjectTree(context.Background(), first)
	two, _ := digestCanonicalSubjectTree(context.Background(), second)
	if one != two {
		t.Fatalf("reconstruction digest %q != %q", one, two)
	}
	if err := os.Remove(filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../escape", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if err := copyCanonicalSubjectTree(context.Background(), source, filepath.Join(t.TempDir(), "refused")); err == nil {
		t.Fatal("escaping candidate symlink accepted")
	}
}

func TestImmutableGitDigestMatchesMaterializedCompleteTree(t *testing.T) {
	source := t.TempDir()
	runSourceGit(t, source, "init")
	runSourceGit(t, source, "config", "user.name", "Test")
	runSourceGit(t, source, "config", "user.email", "test@example.invalid")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	large := make([]byte, 3<<20)
	for i := range large {
		large[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "large"), large, 0o755); err != nil {
		t.Fatal(err)
	}
	runSourceGit(t, source, "add", ".")
	runSourceGit(t, source, "commit", "-m", "subject")
	commit, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := canonicalImmutableCommitDigest(context.Background(), source, commit)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "snapshot")
	defer func() {
		_ = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if info.IsDir() {
					_ = os.Chmod(path, 0o700)
				} else if info.Mode().IsRegular() {
					_ = os.Chmod(path, 0o600)
				}
			}
			return nil
		})
	}()
	materialized, err := copyImmutableCommitTree(context.Background(), source, commit, target)
	if err != nil {
		t.Fatal(err)
	}
	if preflight != materialized {
		t.Fatalf("preflight=%q materialized=%q", preflight, materialized)
	}
	if info, err := os.Stat(filepath.Join(target, "nested", "large")); err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
}

func runSourceGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestPreflightCommitDigestSurvivesHeadRace(t *testing.T) {
	source := t.TempDir()
	runSourceGit(t, source, "init")
	runSourceGit(t, source, "config", "user.name", "Test")
	runSourceGit(t, source, "config", "user.email", "test@example.invalid")
	path := filepath.Join(source, "subject")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSourceGit(t, source, "add", ".")
	runSourceGit(t, source, "commit", "-m", "first")
	commit, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := canonicalImmutableCommitDigest(context.Background(), source, commit)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSourceGit(t, source, "add", ".")
	runSourceGit(t, source, "commit", "-m", "second")
	current, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if current == commit {
		t.Fatal("fixture did not move HEAD")
	}
	target := filepath.Join(t.TempDir(), "old-subject")
	defer func() {
		_ = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	}()
	actual, err := copyImmutableCommitTree(context.Background(), source, commit, target)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("HEAD race changed exact preflight subject: %q != %q", actual, expected)
	}
	raw, err := os.ReadFile(filepath.Join(target, "subject"))
	if err != nil || string(raw) != "first" {
		t.Fatalf("spawn source=%q err=%v", raw, err)
	}
}

func TestImmutableGitPreflightRejectsEscapingSymlink(t *testing.T) {
	source := t.TempDir()
	runSourceGit(t, source, "init")
	runSourceGit(t, source, "config", "user.name", "Test")
	runSourceGit(t, source, "config", "user.email", "test@example.invalid")
	if err := os.Symlink("../escape", filepath.Join(source, "escape")); err != nil {
		t.Fatal(err)
	}
	runSourceGit(t, source, "add", ".")
	runSourceGit(t, source, "commit", "-m", "escape")
	commit, err := immutableGitCommitIdentity(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalImmutableCommitDigest(context.Background(), source, commit); err == nil {
		t.Fatal("escaping git symlink accepted by read-only preflight")
	}
}

func TestCanonicalSubjectDigestLargeFileHonorsCancellation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace", "platform")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "large"), make([]byte, 8<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := digestCanonicalSubjectTree(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("large subject cancellation error=%v", err)
	}
}

func TestRequireFrozenComputerSurfaceSource(t *testing.T) {
	missing := t.TempDir()
	if err := requireFrozenComputerSurfaceSource(missing); err == nil {
		t.Fatal("freeze accepted a tree with no computer-surface frontend source")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "frontend", "index.html"), []byte("<html>computer</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := requireFrozenComputerSurfaceSource(root); err != nil {
		t.Fatal(err)
	}
}
