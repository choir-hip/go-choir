package capsule

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

func TestCopyImmutableSourceTreePinsTrackedCleanFiles(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "snapshot")
	t.Cleanup(func() {
		_ = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
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

	digest, err := copyImmutableSourceTree(context.Background(), source, target)
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

	_, err := copyImmutableSourceTree(context.Background(), source, filepath.Join(t.TempDir(), "snapshot"))
	if err == nil || !strings.Contains(err.Error(), "dirty tracked files") {
		t.Fatalf("dirty source error = %v", err)
	}
}

func TestCopyImmutableCommitTreeIgnoresMutableWorktree(t *testing.T) {
	source := t.TempDir()
	mustRunGit(t, source, "init")
	mustRunGit(t, source, "config", "user.name", "Capsule Test")
	mustRunGit(t, source, "config", "user.email", "capsule@test.invalid")
	path := filepath.Join(source, "tracked")
	if err := os.WriteFile(path, []byte("committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, source, "add", "tracked")
	mustRunGit(t, source, "commit", "-m", "fixture")
	rawCommit, err := exec.Command("git", "-C", source, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("mutated-after-commit"), 0o644); err != nil {
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
	if _, err := copyImmutableCommitTree(context.Background(), source, strings.TrimSpace(string(rawCommit)), target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "tracked"))
	if err != nil || string(content) != "committed" {
		t.Fatalf("object-pinned snapshot content = %q, %v", content, err)
	}
}

func TestCopyImmutableSourceTreeHonorsCancellation(t *testing.T) {
	source := t.TempDir()
	target := filepath.Join(t.TempDir(), "snapshot")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := copyImmutableSourceTree(ctx, source, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled source snapshot error = %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("canceled source snapshot created target: %v", err)
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

func TestPreflightSourceSnapshotIsReadOnlyAndCommitPinned(t *testing.T) {
	source := t.TempDir()
	mustRunGit(t, source, "init")
	mustRunGit(t, source, "config", "user.name", "Capsule Test")
	mustRunGit(t, source, "config", "user.email", "capsule@test.invalid")
	path := filepath.Join(source, "tracked")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, source, "add", ".")
	mustRunGit(t, source, "commit", "-m", "first")
	state := filepath.Join(t.TempDir(), "must-not-exist")
	e := &Executor{stateDir: state, sourceDir: source}
	preflight, err := e.PreflightSourceSnapshot(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("read-only preflight mutated executor state: %v", err)
	}
	if !strings.HasPrefix(preflight.ArtifactRef, "capsule-source-git:") || len(preflight.SubjectDigest) != 64 {
		t.Fatalf("preflight=%+v", preflight)
	}
	if err := os.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRunGit(t, source, "add", ".")
	mustRunGit(t, source, "commit", "-m", "second")
	raw := strings.TrimPrefix(preflight.ArtifactRef, "capsule-source-git:")
	split := strings.Index(raw, ":sha256:")
	if split <= 0 {
		t.Fatal("invalid exact source ref")
	}
	target := filepath.Join(t.TempDir(), "pinned")
	t.Cleanup(func() {
		_ = filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
			if err == nil && info.IsDir() {
				_ = os.Chmod(path, 0o700)
			}
			return nil
		})
	})
	digest, err := copyImmutableCommitTree(context.Background(), source, raw[:split], target)
	if err != nil {
		t.Fatal(err)
	}
	if digest != preflight.SubjectDigest {
		t.Fatalf("pinned spawn digest=%q preflight=%q", digest, preflight.SubjectDigest)
	}
}

func TestPersistGrantedCandidateIsReconstructableAndReusable(t *testing.T) {
	state := t.TempDir()
	merged := filepath.Join(t.TempDir(), "root")
	subject := filepath.Join(merged, "workspace", "platform")
	if err := os.MkdirAll(filepath.Join(subject, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subject, "candidate"), []byte("candidate bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	caps := &Capsule{ID: "capsule", State: StateFrozen, MergedDir: merged, SourceSnapshotDigest: strings.Repeat("a", 64)}
	capability := &Capability{CapabilityID: "cap", Handle: "handle", AgentRunID: "run", AgentRole: RoleCoSuper, TargetCapsule: caps.ID, ExpiresAt: time.Now().Add(time.Hour)}
	e := &Executor{stateDir: state, capsules: map[string]*Capsule{caps.ID: caps}, capabilities: map[capKey]*Capability{{AgentRunID: "run", Handle: "handle"}: capability}, revokedCaps: map[string]bool{}}
	candidate, err := e.PersistGrantedCandidate(context.Background(), "run", "handle")
	if err != nil {
		t.Fatal(err)
	}
	preflight, err := e.PreflightSourceSnapshot(context.Background(), candidate.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}
	if candidate != preflight {
		t.Fatalf("candidate=%+v preflight=%+v", candidate, preflight)
	}
	root, _, err := e.subjectArtifactPath(candidate.ArtifactRef)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				if info.IsDir() {
					_ = os.Chmod(path, 0o700)
				} else if info.Mode().IsRegular() {
					_ = os.Chmod(path, 0o600)
				}
			}
			return nil
		})
	})
	if raw, err := os.ReadFile(filepath.Join(root, "candidate")); err != nil || string(raw) != "candidate bytes" {
		t.Fatalf("reconstructed=%q err=%v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(root, "empty")); err != nil {
		t.Fatalf("empty directory not reconstructed: %v", err)
	}
}
