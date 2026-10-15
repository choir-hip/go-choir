package builder

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// S2-f: a source patch is applied to a detached clone at its pinned base
// commit, committed as a synthetic revision, and exposed as a git+file:
// installable ref — the builder never evaluates the caller's live tree.
func TestPatchedFlakeRefAppliesAndCommitsPatch(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("main.go", "package main\n\nfunc main() {}\n")
	git("add", "-A")
	git("commit", "-qm", "base")
	baseOut, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSpace(string(baseOut))

	patch := `--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 package main
 
+// patched
 func main() {}
`
	patchPath := filepath.Join(t.TempDir(), "source.patch")
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}

	ref, patchedCommit, patchSHA, cleanup, err := patchedFlakeRef(context.Background(), dir, patchPath, "#autoputer", base)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if patchedCommit == base || len(patchedCommit) != 40 {
		t.Fatalf("synthetic commit = %q, want a new rev", patchedCommit)
	}
	if len(patchSHA) != 64 {
		t.Fatalf("patch sha = %q", patchSHA)
	}
	if !strings.HasPrefix(ref, "git+file://") || !strings.HasSuffix(ref, "#autoputer") {
		t.Fatalf("installable ref = %q", ref)
	}

	// The synthetic commit carries the patch content; the caller's checkout
	// is untouched.
	tmpDir := strings.TrimPrefix(ref, "git+file://")
	tmpDir = tmpDir[:strings.Index(tmpDir, "#")]
	patched, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(patched), "// patched") {
		t.Fatalf("patched worktree content = %q", patched)
	}
	live, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(live), "// patched") {
		t.Fatalf("caller's live tree was mutated")
	}
}

// H12: the synthetic commit depends only on the base commit and the patch,
// never on wall time or the calling user's git identity, so code_commit (and
// the flake's self.rev and lastModifiedDate) is reproducible.
func TestPatchedFlakeRefCommitIsReproducible(t *testing.T) {
	dir := t.TempDir()
	run := func(env []string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run(nil, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(nil, "add", "-A")
	run([]string{"GIT_AUTHOR_DATE=2026-10-01T12:00:00Z", "GIT_COMMITTER_DATE=2026-10-01T12:00:00Z"},
		"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "base")
	base := run(nil, "rev-parse", "HEAD")
	patchPath := filepath.Join(t.TempDir(), "source.patch")
	patch := "--- a/main.go\n+++ b/main.go\n@@ -1,3 +1,4 @@\n package main\n \n+// patched\n func main() {}\n"
	if err := os.WriteFile(patchPath, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	build := func(env map[string]string) (string, string) {
		t.Helper()
		for k, v := range env {
			t.Setenv(k, v)
		}
		ref, commit, _, cleanup, err := patchedFlakeRef(context.Background(), dir, patchPath, "#autoputer", base)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		repo := strings.TrimPrefix(ref, "git+file://")
		repo = repo[:strings.Index(repo, "#")]
		cmd := exec.Command("git", "log", "-1", "--format=%an <%ae> %aI | %cn <%ce> %cI")
		cmd.Dir = repo
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		return commit, strings.TrimSpace(string(out))
	}
	first, firstMeta := build(map[string]string{"GIT_AUTHOR_DATE": "2030-01-01T00:00:00Z", "GIT_AUTHOR_NAME": "someone"})
	second, secondMeta := build(map[string]string{"GIT_COMMITTER_DATE": "2031-01-01T00:00:00Z", "GIT_COMMITTER_EMAIL": "x@y"})
	if first != second {
		t.Fatalf("synthetic commit not reproducible: %s (%s) != %s (%s)", first, firstMeta, second, secondMeta)
	}
	if !strings.Contains(firstMeta, "2026-10-01T12:00:00Z") || !strings.Contains(firstMeta, "choir-builder <builder@choir.local>") {
		t.Fatalf("synthetic commit metadata = %q, want the builder identity at the base commit's date", firstMeta)
	}
}

func TestPatchedFlakeRefRequiresBaseCommit(t *testing.T) {
	if _, _, _, _, err := patchedFlakeRef(context.Background(), t.TempDir(), "x.patch", "#x", ""); err == nil ||
		!strings.Contains(err.Error(), "base commit") {
		t.Fatalf("missing-base error = %v", err)
	}
}
