package capsule

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The binary-reuse gate is read each time Freeze builds its differ, not once
// at construction (docs/problems/freeze-rebuilds-the-binary-after-an-apply-2026-10-26.md):
// the autoputer builds its executor before the per-computer source.projection
// setting is loaded, so a gate read at construction was false on a computer
// whose projection is on by setting. Ways it could fail: the gate is latched
// at the first read; a Go-input diff reuses the binary; a false gate stops
// refusing with "build bin/autoputer".

// gateTestRepo makes a git repository with a base commit, a frontend-only
// commit on top of it and a Go-input commit on top of that.
func gateTestRepo(t *testing.T) (repo, base, frontend, goCommit string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo = t.TempDir()
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
	base = run("rev-parse", "HEAD")
	put("frontend/a.txt", "two\n")
	run("add", "-A")
	run("commit", "-q", "-m", "frontend only")
	frontend = run("rev-parse", "HEAD")
	put("internal/x.go", "package x\n\nvar Y = 1\n")
	run("add", "-A")
	run("commit", "-q", "-m", "go")
	goCommit = run("rev-parse", "HEAD")
	return repo, base, frontend, goCommit
}

// checkGateGoesLive drives a gate that starts false and turns true after the
// differ source was constructed. differ is how the caller reads the differ
// (the executor's path, or the gate itself); setGate installs the gate.
func checkGateGoesLive(t *testing.T, base, frontend, goCommit string, setGate func(func() bool), differ func() BinaryInputDiffer) {
	t.Helper()
	running := RunningBinary{SHA256: strings.Repeat("a", 64), Commit: base}
	frontendOnly := []FileChange{{Path: "/workspace/platform/frontend/a.txt", Kind: ChangeModified}}

	on := false
	setGate(func() bool { return on })

	// Gate false: the old rule, the desk builds.
	if proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(frontend), running, differ()); err == nil || proof != "" || !strings.Contains(err.Error(), "build bin/autoputer") {
		t.Fatalf("gate false: proof %q err %v, want a refusal that says build bin/autoputer", proof, err)
	}

	// The projection setting loads after construction and the gate turns on.
	on = true
	if proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(frontend), running, differ()); err != nil || proof != ReuseProofBinaryInputsEqual {
		t.Fatalf("gate true after construction, frontend-only diff: proof %q err %v, want %q", proof, err, ReuseProofBinaryInputsEqual)
	}
	if proof, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(goCommit), running, differ()); err == nil || proof != "" || !strings.Contains(err.Error(), "internal/x.go") {
		t.Fatalf("gate true, Go-input diff: proof %q err %v, want a refusal naming internal/x.go", proof, err)
	}

	// The setting turns off again: the gate is live in both directions.
	on = false
	if _, err := PlanReleaseBinaryReuseWith(context.Background(), frontendOnly, sourceRef(frontend), running, differ()); err == nil || !strings.Contains(err.Error(), "build bin/autoputer") {
		t.Fatalf("gate false again: err %v, want a refusal that says build bin/autoputer", err)
	}
}

func TestReuseGateIsReadEachTimeTheDifferIsBuilt(t *testing.T) {
	var gate reuseGate
	repo, base, frontend, goCommit := gateTestRepo(t)
	// An unset gate is off.
	if gate.differ(repo) != nil {
		t.Fatal("an unset gate must build no differ")
	}
	checkGateGoesLive(t, base, frontend, goCommit, gate.set, func() BinaryInputDiffer { return gate.differ(repo) })
}
