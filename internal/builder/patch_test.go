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

func TestPatchedFlakeRefRequiresBaseCommit(t *testing.T) {
	if _, _, _, _, err := patchedFlakeRef(context.Background(), t.TempDir(), "x.patch", "#x", ""); err == nil ||
		!strings.Contains(err.Error(), "base commit") {
		t.Fatalf("missing-base error = %v", err)
	}
}
