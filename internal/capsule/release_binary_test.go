package capsule

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ways reusing the running binary at Freeze can go wrong
// (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, A1-A8):
//
//  A1. A Go input is classified as not touching the binary, so the reused
//      binary does not reflect the frozen source.
//  A2. The source pin and the running binary are different commits, or the
//      running binary is a dev build with no commit.
//  A3. The staged copy differs from the running executable.
//  A4. A binary the desk built is replaced by the running one.
//  A5. Verification accepts a reused binary that is no longer the running
//      build.
//  A6/A7. The provenance record disagrees with the release files, or a
//      legacy bundle without one is refused.

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func sourceRef(commit string) string {
	return "capsule-source-git:" + commit + ":sha256:" + strings.Repeat("d", 64)
}

func TestBinaryInputPathMirrorsTheFlakeFilter(t *testing.T) {
	inputs := []string{
		"go.mod", "go.sum", "go.work", "go.work.sum", "flake.nix", "flake.lock",
		"cmd/autoputer/main.go", "cmd/autoputer", "cmd",
		"internal/agentcore/runtime.go", "internal/runtimeprompts/overlays/rlm_engineering_runtime.yaml",
		"internal/newpkg", "internal/x/testdata/fixture.json",
		"vendor/github.com/x/y/z.go", "nix/autoputer-vm.nix",
		"scripts/tool.go", "frontend/tools/gen.go",
	}
	for _, rel := range inputs {
		if !BinaryInputPath(rel) {
			t.Errorf("BinaryInputPath(%q) = false, want true", rel)
		}
	}
	notInputs := []string{
		"", ".", "frontend/public/choir-drill/x.txt", "frontend/src/App.svelte", "frontend",
		"docs/problems/x.md", "README.md", "skills/throughline/SKILL.md",
		"internal/agentcore/runtime_test.go", "cmd/autoputer/main_test.go", "scripts/reeng_tracer.mjs",
		"frontend/go.mod.txt",
	}
	for _, rel := range notInputs {
		if BinaryInputPath(rel) {
			t.Errorf("BinaryInputPath(%q) = true, want false", rel)
		}
	}
}

func TestSourceChangesTouchingBinaryReadsOnlyThePlatformWorkspace(t *testing.T) {
	changes := []FileChange{
		{Path: "/workspace/platform/frontend/public/a.txt", Kind: ChangeAdded},
		{Path: "workspace/platform/internal/agentcore/x.go", Kind: ChangeDeleted},
		{Path: "/var/lib/artifact/release/frontend/index.html", Kind: ChangeAdded},
		{Path: "/tmp/go-build/abc.go", Kind: ChangeAdded},
		{Path: "/workspace/platform", Kind: ChangeModified, Mode: os.ModeDir},
		{Path: "/workspace/platform/go.sum", Kind: ChangeModified},
	}
	got := SourceChangesTouchingBinary(changes)
	want := []string{"go.sum", "internal/agentcore/x.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("SourceChangesTouchingBinary = %v, want %v", got, want)
	}
}

func TestPlanReleaseBinaryReuse(t *testing.T) {
	frontendOnly := []FileChange{{Path: "/workspace/platform/frontend/public/choir-drill/x.txt", Kind: ChangeAdded}}
	running := RunningBinary{Path: "/proc/self/exe", SHA256: strings.Repeat("a", 64), Commit: testCommit}
	if err := PlanReleaseBinaryReuse(frontendOnly, sourceRef(testCommit), running); err != nil {
		t.Fatalf("frontend-only change at the running commit refused reuse: %v", err)
	}
	cases := []struct {
		name    string
		changes []FileChange
		ref     string
		running RunningBinary
		want    string
	}{
		{"go change", []FileChange{{Path: "/workspace/platform/internal/x/y.go", Kind: ChangeModified}}, sourceRef(testCommit), running, "internal/x/y.go"},
		{"other commit", frontendOnly, sourceRef(strings.Repeat("b", 40)), running, "running binary was built from " + testCommit},
		{"dev build", frontendOnly, sourceRef(testCommit), RunningBinary{SHA256: running.SHA256, Commit: "local"}, "has no release commit"},
		{"candidate artifact source", frontendOnly, "capsule-subject:sha256:" + strings.Repeat("c", 64), running, "not pinned to a git commit"},
		{"no running hash", frontendOnly, sourceRef(testCommit), RunningBinary{Commit: testCommit}, "running binary identity"},
	}
	for _, tc := range cases {
		err := PlanReleaseBinaryReuse(tc.changes, tc.ref, tc.running)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want it to name %q", tc.name, err, tc.want)
		}
	}
}

func writeExecutable(t *testing.T, dir string, payload []byte) (string, string) {
	t.Helper()
	path := filepath.Join(dir, "autoputer-running")
	if err := os.WriteFile(path, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	return path, hex.EncodeToString(sum[:])
}

func TestStageRunningBinaryCopiesTheVerifiedExecutable(t *testing.T) {
	src := t.TempDir()
	path, digest := writeExecutable(t, src, []byte("\x7fELF running build"))
	root := t.TempDir()
	file, size, err := StageRunningBinary(context.Background(), root, RunningBinary{Path: path, SHA256: digest, Commit: testCommit})
	if err != nil {
		t.Fatalf("StageRunningBinary: %v", err)
	}
	if file.Path != ReleaseBinaryPath || file.SHA256 != digest || file.Mode != 0o555 || size != int64(len("\x7fELF running build")) {
		t.Fatalf("staged file = %+v size %d, want %s %s 0555", file, size, ReleaseBinaryPath, digest)
	}
	staged, err := os.ReadFile(filepath.Join(root, "bin", "autoputer"))
	if err != nil || string(staged) != "\x7fELF running build" {
		t.Fatalf("staged bytes = %q, %v", staged, err)
	}
	info, err := os.Stat(filepath.Join(root, "bin", "autoputer"))
	if err != nil || info.Mode().Perm() != 0o555 {
		t.Fatalf("staged mode = %v, %v", info.Mode(), err)
	}

	// A3: the executable changed after its identity was taken.
	other := t.TempDir()
	if _, _, err := StageRunningBinary(context.Background(), other, RunningBinary{Path: path, SHA256: strings.Repeat("0", 64), Commit: testCommit}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("a changed executable staged: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(other, "bin", "autoputer")); !os.IsNotExist(statErr) {
		t.Fatalf("a refused copy left a file behind: %v", statErr)
	}
}

func TestValidateReleaseBinaryBindsProvenanceToTheReleaseFile(t *testing.T) {
	digest := strings.Repeat("a", 64)
	files := []FrozenReleaseFile{{Path: "bin/autoputer", SHA256: digest, Mode: 0o555}, {Path: "frontend/index.html", SHA256: strings.Repeat("b", 64), Mode: 0o444}}
	if err := ValidateReleaseBinary(files, nil); err != nil {
		t.Fatalf("legacy bundle without provenance refused: %v", err)
	}
	good := []*ReleaseBinaryProvenance{
		{Origin: ReleaseBinaryCapsuleBuilt, SHA256: digest},
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: testCommit},
	}
	for _, p := range good {
		if err := ValidateReleaseBinary(files, p); err != nil {
			t.Errorf("ValidateReleaseBinary(%+v) = %v", p, err)
		}
	}
	bad := []*ReleaseBinaryProvenance{
		{Origin: "copied", SHA256: digest},
		{Origin: ReleaseBinaryCapsuleBuilt, SHA256: strings.Repeat("c", 64)},
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest},
		{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: strings.Repeat("e", 40)},
	}
	for _, p := range bad {
		if err := ValidateReleaseBinary(files, p); err == nil {
			t.Errorf("ValidateReleaseBinary(%+v) accepted", p)
		}
	}
	if err := ValidateReleaseBinary(files[1:], good[0]); err == nil {
		t.Errorf("provenance without a bin/autoputer file accepted")
	}
}

func TestCheckReusedBinaryIsRunning(t *testing.T) {
	digest := strings.Repeat("a", 64)
	reused := &ReleaseBinaryProvenance{Origin: ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: testCommit, SourceBaseCommit: testCommit}
	if err := CheckReusedBinaryIsRunning(reused, RunningBinary{SHA256: digest, Commit: testCommit}); err != nil {
		t.Fatalf("matching running build refused: %v", err)
	}
	if err := CheckReusedBinaryIsRunning(reused, RunningBinary{SHA256: strings.Repeat("f", 64), Commit: strings.Repeat("9", 40)}); err == nil || !strings.Contains(err.Error(), "no longer") {
		t.Fatalf("a redeployed computer accepted a stale reused binary: %v", err)
	}
	// A desk-built binary and a legacy bundle are not bound to the running build.
	if err := CheckReusedBinaryIsRunning(&ReleaseBinaryProvenance{Origin: ReleaseBinaryCapsuleBuilt, SHA256: digest}, RunningBinary{}); err != nil {
		t.Fatalf("capsule-built binary checked against the running build: %v", err)
	}
	if err := CheckReusedBinaryIsRunning(nil, RunningBinary{}); err != nil {
		t.Fatalf("legacy bundle checked against the running build: %v", err)
	}
}

func TestSourceArtifactCommit(t *testing.T) {
	if got := SourceArtifactCommit(sourceRef(testCommit)); got != testCommit {
		t.Fatalf("SourceArtifactCommit = %q", got)
	}
	for _, ref := range []string{"", "capsule-subject:sha256:" + strings.Repeat("c", 64), "capsule-source-git:zz:sha256:" + strings.Repeat("d", 64), "capsule-source-git:" + testCommit} {
		if got := SourceArtifactCommit(ref); got != "" {
			t.Errorf("SourceArtifactCommit(%q) = %q, want empty", ref, got)
		}
	}
}
