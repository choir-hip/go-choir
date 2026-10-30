package builder

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	ref, patchedCommit, patchSHA, cleanup, err := patchedFlakeRef(context.Background(), dir, patchPath, "#autoputer", base, nil)
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
		ref, commit, _, cleanup, err := patchedFlakeRef(context.Background(), dir, patchPath, "#autoputer", base, nil)
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
	if _, _, _, _, err := patchedFlakeRef(context.Background(), t.TempDir(), "x.patch", "#x", "", nil); err == nil ||
		!strings.Contains(err.Error(), "base commit") {
		t.Fatalf("missing-base error = %v", err)
	}
}

// patchRepo is a base repository with a recipe file, a nix dir, an ignored
// dir and a Go file; it returns the repo dir and the base commit.
func patchRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	for rel, content := range map[string]string{
		"flake.nix":  "a\n",
		"nix/a.nix":  "a\n",
		"main.go":    "package main\n",
		".gitignore": "ignored/\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "base")
	return dir, run("rev-parse", "HEAD")
}

func writePatch(t *testing.T, patch string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "source.patch")
	if err := os.WriteFile(p, []byte(patch), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// H15 (docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md): the paths
// the admission sees are the paths git committed, however the patch spells
// them: any first segment, no git header, C-quoted names.
func TestPatchedTreeAdmissionSeesWhatGitWrites(t *testing.T) {
	for name, tc := range map[string]struct{ patch, want string }{
		"any first segment": {"diff --git x/flake.nix y/flake.nix\n--- x/flake.nix\n+++ y/flake.nix\n@@ -1 +1 @@\n-a\n+b\n", "flake.nix"},
		"no git header":     {"--- foo/flake.nix\n+++ bar/flake.nix\n@@ -1 +1 @@\n-a\n+b\n", "flake.nix"},
		"quoted names":      {"diff --git \"a/nix/a.nix\" \"b/nix/a.nix\"\n--- \"a/nix/a.nix\"\n+++ \"b/nix/a.nix\"\n@@ -1 +1 @@\n-a\n+b\n", "nix/a.nix"},
		"octal escape":      {"diff --git \"a/fl\\141ke.nix\" \"b/fl\\141ke.nix\"\n--- \"a/fl\\141ke.nix\"\n+++ \"b/fl\\141ke.nix\"\n@@ -1 +1 @@\n-a\n+b\n", "flake.nix"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, base := patchRepo(t)
			var seen []PatchedChange
			refused := errors.New("refused by admission")
			_, _, _, cleanup, err := patchedFlakeRef(context.Background(), dir, writePatch(t, tc.patch), "#autoputer", base, func(changes []PatchedChange) error {
				seen = changes
				return refused
			})
			if cleanup != nil {
				cleanup()
			}
			if !errors.Is(err, refused) {
				t.Fatalf("err = %v, want the admission's refusal", err)
			}
			if len(seen) != 1 || seen[0].Path != tc.want {
				t.Fatalf("admission saw %+v, want %s", seen, tc.want)
			}
		})
	}
}

// H16: a symlink, a gitlink, or git metadata that changes what the tree
// means is refused even with no caller policy.
func TestPatchedTreeRefusesLinksAndGitMetadata(t *testing.T) {
	for name, patch := range map[string]string{
		"symlink":       "diff --git a/link b/link\nnew file mode 120000\n--- /dev/null\n+++ b/link\n@@ -0,0 +1 @@\n+/etc/passwd\n\\ No newline at end of file\n",
		"gitattributes": "diff --git a/sub/.gitattributes b/sub/.gitattributes\nnew file mode 100644\n--- /dev/null\n+++ b/sub/.gitattributes\n@@ -0,0 +1 @@\n+* filter=x\n",
		"gitmodules":    "diff --git a/.gitmodules b/.gitmodules\nnew file mode 100644\n--- /dev/null\n+++ b/.gitmodules\n@@ -0,0 +1 @@\n+[submodule]\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir, base := patchRepo(t)
			_, _, _, cleanup, err := patchedFlakeRef(context.Background(), dir, writePatch(t, patch), "#autoputer", base, nil)
			if cleanup != nil {
				cleanup()
			}
			var refusal *PatchRefusal
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a patch refusal", err)
			}
		})
	}
}

// H17: the tree nix evaluates is the committed one; a patch that writes a
// file the commit does not carry (an ignored path) is refused.
func TestPatchedTreeMustEqualTheCommit(t *testing.T) {
	dir, base := patchRepo(t)
	patch := "diff --git a/ignored/x.go b/ignored/x.go\nnew file mode 100644\n--- /dev/null\n+++ b/ignored/x.go\n@@ -0,0 +1 @@\n+package x\n"
	_, _, _, cleanup, err := patchedFlakeRef(context.Background(), dir, writePatch(t, patch), "#autoputer", base, nil)
	if cleanup != nil {
		cleanup()
	}
	if err == nil || !strings.Contains(err.Error(), "not committed") {
		t.Fatalf("err = %v, want the uncommitted file refused", err)
	}
}

// H25: every nix call in the host build path disables remote builders (so a
// guest-authored derivation never builds on node-a-builder, which another
// project shares), keeps local building on, and restates the sandbox.
func TestEveryNixCallBuildsOnlyHereSandboxed(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "argv")
	// One line per call; each argument wrapped in [] so an empty one shows.
	script := "#!/bin/sh\nline=''\nfor a in \"$@\"; do line=\"$line[$a]\"; done\necho \"$line\" >> " + logPath +
		"\necho /nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0\n"
	if err := os.WriteFile(filepath.Join(bin, "nix"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	ctx := context.Background()
	if _, err := buildResult(ctx, ".#autoputer", filepath.Join(t.TempDir(), "result"), time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := evalDrvPath(ctx, ".#autoputer"); err != nil {
		t.Fatal(err)
	}
	if _, err := pathInfoClosure(ctx, "/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	calls := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(calls) < 4 { // build, eval outPath, eval drvPath, path-info
		t.Fatalf("nix calls = %q", calls)
	}
	for _, call := range calls {
		for _, want := range []string{"[--option][builders][]", "[--option][max-jobs][2]", "[--option][sandbox][true]"} {
			if !strings.Contains(call, want) {
				t.Errorf("nix call %s lacks %s", call, want)
			}
		}
	}
}

// H18: the build step is bound by the build's own timeout, not by the
// per-call nix bound for evaluations.
func TestBuildStepIsBoundByTheBuildTimeout(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do if [ \"$a\" = build ]; then sleep 1; fi; done\necho /nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0\n"
	if err := os.WriteFile(filepath.Join(bin, "nix"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	saved := nixTimeout
	nixTimeout = 200 * time.Millisecond
	defer func() { nixTimeout = saved }()
	out, err := buildResult(context.Background(), ".#autoputer", filepath.Join(t.TempDir(), "result"), 5*time.Second)
	if err != nil || out != "/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-autoputer-0.1.0" {
		t.Fatalf("build under a 5s build bound with a 200ms eval bound = %q, %v", out, err)
	}
	if _, err := buildResult(context.Background(), ".#autoputer", filepath.Join(t.TempDir(), "result"), 100*time.Millisecond); err == nil {
		t.Fatal("a build past its bound succeeded")
	}
}
