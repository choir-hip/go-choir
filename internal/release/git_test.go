package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Failure modes pinned: I3/A7 (the tree hash and the apply check must not
// write to the source repository), A5 (a patch that does not apply to the
// adopter's own tree is refused, never merged).

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "a.txt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	return dir, git(t, dir, "rev-parse", "HEAD")
}

// fingerprint is everything a write to the repository would move: the
// index bytes, the loose and packed object set, HEAD and the worktree status.
type fingerprint struct {
	index   []byte
	objects []string
	head    string
	status  string
}

func repoFingerprint(t *testing.T, dir string) fingerprint {
	t.Helper()
	index, _ := os.ReadFile(filepath.Join(dir, ".git", "index"))
	var objects []string
	root := filepath.Join(dir, ".git", "objects")
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, path)
			objects = append(objects, rel)
		}
		return nil
	})
	// --no-optional-locks: a plain status refreshes a racy index (stat data
	// of files written in the same second as the commit) and rewrites
	// .git/index, so the fingerprint itself would change the repository.
	return fingerprint{
		index:   index,
		objects: objects,
		head:    git(t, dir, "rev-parse", "HEAD"),
		status:  git(t, dir, "--no-optional-locks", "status", "--porcelain"),
	}
}

// fingerprintDiff names each part of the fingerprint that moved, empty when
// none did: a failure must say which writer to look for.
func fingerprintDiff(before, after fingerprint) string {
	var parts []string
	if !bytes.Equal(before.index, after.index) {
		parts = append(parts, fmt.Sprintf("index: %d bytes sha256 %x -> %d bytes sha256 %x",
			len(before.index), sha256.Sum256(before.index), len(after.index), sha256.Sum256(after.index)))
	}
	was := map[string]bool{}
	for _, p := range before.objects {
		was[p] = true
	}
	is := map[string]bool{}
	for _, p := range after.objects {
		is[p] = true
	}
	var added, removed []string
	for _, p := range after.objects {
		if !was[p] {
			added = append(added, p)
		}
	}
	for _, p := range before.objects {
		if !is[p] {
			removed = append(removed, p)
		}
	}
	if len(added) > 0 || len(removed) > 0 {
		parts = append(parts, fmt.Sprintf("objects: added %v, removed %v", added, removed))
	}
	if before.head != after.head {
		parts = append(parts, fmt.Sprintf("HEAD: %s -> %s", before.head, after.head))
	}
	if before.status != after.status {
		parts = append(parts, fmt.Sprintf("status: %q -> %q", before.status, after.status))
	}
	return strings.Join(parts, "; ")
}

func TestPatchedTreeEqualsTheCommittedTreeAndLeavesTheRepositoryUntouched(t *testing.T) {
	dir, base := newRepo(t)
	// Make the change in a scratch clone, so the repository under test sees
	// none of it, and take the patch and the tree a real commit gets.
	scratch := t.TempDir()
	git(t, scratch, "clone", "-q", dir, ".")
	if err := os.WriteFile(filepath.Join(scratch, "frontend", "a.txt"), []byte("one\nTWO\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scratch, "frontend", "new.txt"), []byte("token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, scratch, "add", "-A")
	patch := []byte(git(t, scratch, "diff", "--cached", "--no-color") + "\n")
	git(t, scratch, "commit", "-q", "-m", "change")
	want := git(t, scratch, "rev-parse", "HEAD^{tree}")

	before := repoFingerprint(t, dir)
	got, err := PatchedTree(context.Background(), dir, base, patch)
	if err != nil {
		t.Fatalf("PatchedTree: %v", err)
	}
	if got != want {
		t.Fatalf("tree = %s, want %s (the tree a commit of the patched files gets)", got, want)
	}
	if diff := fingerprintDiff(before, repoFingerprint(t, dir)); diff != "" {
		t.Fatalf("PatchedTree wrote to the repository: %s", diff)
	}
	tree, err := CommitTree(context.Background(), dir, base)
	if err != nil || tree != git(t, dir, "rev-parse", base+"^{tree}") {
		t.Fatalf("CommitTree = %s, %v", tree, err)
	}
	head, err := Head(context.Background(), dir)
	if err != nil || head != base {
		t.Fatalf("Head = %s, %v", head, err)
	}
}

func TestPatchedTreeRefusesAPatchThatDoesNotApply(t *testing.T) {
	dir, base := newRepo(t)
	conflict := []byte(strings.Join([]string{
		"diff --git a/frontend/a.txt b/frontend/a.txt",
		"--- a/frontend/a.txt",
		"+++ b/frontend/a.txt",
		"@@ -1,3 +1,3 @@",
		" one",
		"-not what the base has",
		"+TWO",
		" three",
		"",
	}, "\n"))
	before := repoFingerprint(t, dir)
	if _, err := PatchedTree(context.Background(), dir, base, conflict); !errors.Is(err, ErrPatchDoesNotApply) {
		t.Fatalf("err = %v, want ErrPatchDoesNotApply", err)
	}
	if diff := fingerprintDiff(before, repoFingerprint(t, dir)); diff != "" {
		t.Fatalf("a refused apply check wrote to the repository: %s", diff)
	}
	if _, err := PatchedTree(context.Background(), dir, strings.Repeat("0", 40), conflict); err == nil || errors.Is(err, ErrPatchDoesNotApply) {
		t.Fatalf("a base commit the repository lacks must be its own error, got %v", err)
	}
}
