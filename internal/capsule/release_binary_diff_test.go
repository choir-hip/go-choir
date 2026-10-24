package capsule

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// U1 binary reuse (docs/design/computer-update-architecture-rnd-2026-10-24.md,
// "U1 build notes", R1 and R2): reuse across differing commits only when no
// binary-input path differs, and never on a failed comparison.

func TestPlanReleaseBinaryReuseWithDiffer(t *testing.T) {
	frontendOnly := []FileChange{{Path: "/workspace/platform/frontend/src/App.svelte", Kind: ChangeModified}}
	running := RunningBinary{Path: "/proc/self/exe", SHA256: strings.Repeat("a", 64), Commit: testCommit}
	other := strings.Repeat("b", 40)

	frontendDiff := func(context.Context, string, string) ([]string, error) {
		return []string{"frontend/src/a.svelte", "docs/x.md", "internal/agentcore/runtime_test.go"}, nil
	}
	proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(other), running, frontendDiff)
	if err != nil || proof != ReuseProofBinaryInputsEqual {
		t.Fatalf("a source that differs only in frontend, docs and tests reuses the binary: %q %v", proof, err)
	}

	// Equal commits need no proof, with or without a differ.
	for _, differ := range []BinaryInputDiffer{nil, frontendDiff} {
		if proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(testCommit), running, differ); err != nil || proof != "" {
			t.Fatalf("equal commits: %q %v", proof, err)
		}
	}

	// R1: a Go difference, a failed comparison and the old rule (no differ)
	// all send the desk to build.
	goDiff := func(context.Context, string, string) ([]string, error) {
		return []string{"frontend/src/a.svelte", "internal/agentcore/runtime.go", "go.mod"}, nil
	}
	failed := func(context.Context, string, string) ([]string, error) { return nil, errors.New("bad object") }
	cases := []struct {
		name   string
		differ BinaryInputDiffer
		want   string
	}{
		{"go input differs", goDiff, "internal/agentcore/runtime.go"},
		{"comparison fails", failed, "could not be compared"},
		{"no differ", nil, "running binary was built from " + testCommit},
	}
	for _, tc := range cases {
		proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(other), running, tc.differ)
		if err == nil || proof != "" || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: proof %q err %v, want it to name %q", tc.name, proof, err, tc.want)
		}
	}

	// The candidate's own change to a Go input still builds.
	goChange := []FileChange{{Path: "/workspace/platform/internal/x/y.go", Kind: ChangeModified}}
	if _, err := PlanReleaseBinaryReuseWith(context.Background(), goChange, sourceRef(testCommit), running, frontendDiff); err == nil {
		t.Fatalf("a Go change reused the running binary")
	}
}

func TestValidateReleaseBinaryAcceptsAMismatchOnlyWithTheProof(t *testing.T) {
	digest := strings.Repeat("a", 64)
	files := []FrozenReleaseFile{{Path: "bin/autoputer", SHA256: digest, Mode: 0o555}}
	other := strings.Repeat("b", 40)
	good := &ReleaseBinaryProvenance{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: other, ReuseProof: ReuseProofBinaryInputsEqual}
	if err := ValidateReleaseBinary(files, good); err != nil {
		t.Fatalf("proven mismatch refused: %v", err)
	}
	bad := []*ReleaseBinaryProvenance{
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: other},
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: other, ReuseProof: "trust_me"},
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: testCommit, ReuseProof: ReuseProofBinaryInputsEqual},
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: "", ReuseProof: ReuseProofBinaryInputsEqual},
	}
	for _, p := range bad {
		if err := ValidateReleaseBinary(files, p); err == nil {
			t.Errorf("ValidateReleaseBinary(%+v) accepted", p)
		}
	}
}

func TestGitPathDifferReadsTheSourceRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	put := func(rel, content string) {
		full := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q")
	put("frontend/a.txt", "one\n")
	put("internal/x.go", "package x\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	base := run("rev-parse", "HEAD")
	put("frontend/a.txt", "two\n")
	run("add", "-A")
	run("commit", "-q", "-m", "frontend only")
	frontend := run("rev-parse", "HEAD")
	put("internal/x.go", "package x\n\nvar Y = 1\n")
	run("add", "-A")
	run("commit", "-q", "-m", "go")
	goCommit := run("rev-parse", "HEAD")

	differ := GitPathDiffer(repo)
	running := RunningBinary{SHA256: strings.Repeat("a", 64), Commit: base}
	frontendOnly := []FileChange{{Path: "/workspace/platform/frontend/a.txt", Kind: ChangeModified}}
	if proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(frontend), running, differ); err != nil || proof != ReuseProofBinaryInputsEqual {
		t.Fatalf("base to a frontend-only commit reuses: %q %v", proof, err)
	}
	if _, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(goCommit), running, differ); err == nil || !strings.Contains(err.Error(), "internal/x.go") {
		t.Fatalf("base to a Go commit must build: %v", err)
	}
	// R1: a running commit the repository lacks is a failed comparison.
	lost := RunningBinary{SHA256: running.SHA256, Commit: strings.Repeat("9", 40)}
	if _, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(frontend), lost, differ); err == nil || !strings.Contains(err.Error(), "could not be compared") {
		t.Fatalf("missing running commit: %v", err)
	}
	if _, err := differ(context.Background(), "--output=x", frontend); err == nil {
		t.Fatal("non-commit arguments must be refused")
	}
}
