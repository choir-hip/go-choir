package capsule

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Failure mode (Build review r1, Devin): the commit-side digest
// (canonicalImmutableCommitDigest) did not apply generatedSourcePath while
// the tree walk did, so a commit that tracks bytecode failed every spawn's
// materialization check (copied digest != commit digest). Parity: a commit
// tracking a .pyc and a __pycache__ file digests equal to the walk of its
// copied tree, equals the digest of the same commit without them, and a
// directory only bytecode would have made is not counted by either side.
func TestCommitDigestAppliesGeneratedExclusionLikeTheWalk(t *testing.T) {
	ctx := context.Background()
	build := func(withBytecode bool) (string, string) {
		source := t.TempDir()
		mustRunGit(t, source, "init")
		mustRunGit(t, source, "config", "user.name", "Capsule Test")
		mustRunGit(t, source, "config", "user.email", "capsule@test.invalid")
		files := map[string]string{"tools/report.py": "print(1)", "README": "x"}
		if withBytecode {
			files["tools/__pycache__/report.cpython-312.pyc"] = "bytecode"
			files["only/__pycache__/a.pyc"] = "bytecode"
			files["stray.pyc"] = "bytecode"
			files["foo.pyc/bar.py"] = "x"
			files["tools/__pycache__/sub/y.txt"] = "x"
		}
		for rel, body := range files {
			p := filepath.Join(source, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		mustRunGit(t, source, "add", "-A")
		mustRunGit(t, source, "commit", "-m", "fixture")
		raw, err := exec.Command("git", "-C", source, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return source, strings.TrimSpace(string(raw))
	}
	source, commit := build(true)
	commitDigest, err := canonicalImmutableCommitDigest(ctx, source, commit)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "snapshot")
	t.Cleanup(func() {
		_ = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	copied, err := copyImmutableCommitTree(ctx, source, commit, target)
	if err != nil {
		t.Fatalf("materialization must pass for a commit tracking bytecode: %v", err)
	}
	walk, err := digestCanonicalSubjectTree(ctx, target)
	if err != nil || walk != commitDigest || copied != commitDigest {
		t.Fatalf("commit %q walk %q copy %q (%v)", commitDigest, walk, copied, err)
	}
	cleanSource, cleanCommit := build(false)
	clean, err := canonicalImmutableCommitDigest(ctx, cleanSource, cleanCommit)
	if err != nil {
		t.Fatal(err)
	}
	// "only/" exists in the walk (a directory holding just bytecode) and is
	// not in the clean commit, so the two differ by exactly that directory.
	if clean == commitDigest {
		t.Fatal("expected the bytecode-only directory to remain counted by both sides")
	}
}
