package builder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// digestSHA256Hex returns the lowercase hex SHA-256 of b.
func digestSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeFileAtomic writes content to path with the given mode via a temp file
// + atomic rename, so a partially written artifact is never observed.
func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	if path == "" {
		return fmt.Errorf("builder: output path required")
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".builder-tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// patchedFlakeRef clones sourceDir into a temp repo, checks out baseCommit,
// applies the unified-diff patch file, commits the result as a synthetic
// revision, and returns a git+file: installable ref that nix evaluates at
// exactly that revision. This is the S2-f join surface: capsules ship
// source patches; the builder materializes the patched source it builds
// rather than trusting a caller's live tree. The synthetic commit gives
// the receipt a concrete, reproducible code_commit.
func patchedFlakeRef(ctx context.Context, sourceDir, patchPath, installable, baseCommit string) (ref, patchedCommit, patchSHA string, cleanup func(), err error) {
	if sourceDir == "" {
		return "", "", "", nil, fmt.Errorf("builder: source patch requires --source-dir")
	}
	if baseCommit == "" {
		return "", "", "", nil, fmt.Errorf("builder: source patch requires a base commit")
	}
	blob, err := os.ReadFile(patchPath)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("read patch: %w", err)
	}
	sum := sha256.Sum256(blob)
	patchSHA = hex.EncodeToString(sum[:])
	tmp, err := os.MkdirTemp("", "choir-builder-patch-*")
	if err != nil {
		return "", "", "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	run := func(dir string, args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), runErr, strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := run(sourceDir, "clone", "--local", sourceDir, tmp); err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	if err := run(tmp, "checkout", baseCommit); err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	if err := run(tmp, "apply", patchPath); err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	if err := run(tmp, "add", "-A"); err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	// Commit as a synthetic revision so the build receipt names a concrete
	// commit and the flake's self.rev is non-dirty, non-"local".
	commitMsg := "choir-builder source patch " + patchSHA[:12]
	if err := run(tmp, "-c", "user.name=choir-builder", "-c", "user.email=builder@choir.local", "commit", "-m", commitMsg); err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	head, err := runGitOut(ctx, tmp, "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	// Re-anchor the installable attr onto the patched repo's flake.
	attr := installable
	if idx := strings.Index(installable, "#"); idx >= 0 {
		attr = installable[idx:]
	} else {
		attr = "#" + installable
	}
	return "git+file://" + tmp + attr, strings.TrimSpace(head), patchSHA, cleanup, nil
}

func runGitOut(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// deriveSourceCommit reports the HEAD commit of the checkout at dir and
// whether the worktree is dirty. This is the provenance authority for S2-c:
// the builder names the commit it actually built, not the commit a caller
// claims. A checkout that is not a git repository fails the build.
func deriveSourceCommit(ctx context.Context, dir string) (commit string, dirty bool, err error) {
	head, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", false, fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	commit = strings.TrimSpace(head)
	if len(commit) != 40 {
		return "", false, fmt.Errorf("git rev-parse HEAD returned %q", commit)
	}
	status, err := runGit(ctx, dir, "status", "--porcelain")
	if err != nil {
		return "", false, fmt.Errorf("git status: %w", err)
	}
	return commit, strings.TrimSpace(status) != "", nil
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
