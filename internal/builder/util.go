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
//
// The admission is decided on the tree git committed, never on the patch
// text (H15-H17 of docs/archive/reports/p5-host-build-layered-apply-2026-10-15.md):
// after the commit, the changes are read with `diff --raw --no-renames`
// against the base, links and git metadata are refused, and admit (when set)
// decides the rest; a file the apply wrote that the commit does not carry
// (an ignored path) is refused too, so nix reads exactly the commit.
func patchedFlakeRef(ctx context.Context, sourceDir, patchPath, installable, baseCommit string, admit func([]PatchedChange) error) (ref, patchedCommit, patchSHA string, cleanup func(), err error) {
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
		cmd.Env = syntheticGitEnv()
		out, runErr := cmd.CombinedOutput()
		if runErr != nil {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), runErr, strings.TrimSpace(string(out)))
		}
		return nil
	}
	// The host's deployed checkout is owned by the deploy user; name it safe
	// for this one command instead of trusting every repository.
	if err := run(sourceDir, "-c", "safe.directory="+sourceDir, "clone", "--local", sourceDir, tmp); err != nil {
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
	// H17: after `add -A` every file the apply wrote is staged unless it is
	// ignored; an ignored file would sit in the tree nix reads without being
	// in the commit the receipt names.
	others, err := runGitOut(ctx, tmp, "ls-files", "-z", "--others")
	if err != nil {
		cleanup()
		return "", "", "", nil, fmt.Errorf("list untracked files: %w", err)
	}
	if others = strings.Trim(others, "\x00"); others != "" {
		cleanup()
		return "", "", "", nil, &PatchRefusal{Path: strings.SplitN(others, "\x00", 2)[0], Detail: "the patch writes files the commit does not carry (not committed)"}
	}
	// Commit as a synthetic revision so the build receipt names a concrete
	// commit and the flake's self.rev is non-dirty, non-"local". The commit
	// is reproducible (H12): a fixed identity at the base commit's own
	// committer date, so it depends only on the base and the patch, and so
	// do self.rev and self.lastModifiedDate, which the build stamps.
	baseDate, err := runGitOut(ctx, tmp, "show", "-s", "--format=%cI", baseCommit)
	if err != nil {
		cleanup()
		return "", "", "", nil, fmt.Errorf("read base commit date: %w", err)
	}
	commitMsg := "choir-builder source patch " + patchSHA[:12]
	commitCmd := exec.CommandContext(ctx, "git", "-c", "commit.gpgsign=false", "commit", "-q", "--no-verify", "-m", commitMsg)
	commitCmd.Dir = tmp
	commitCmd.Env = append(syntheticGitEnv(),
		"GIT_AUTHOR_NAME=choir-builder", "GIT_AUTHOR_EMAIL=builder@choir.local", "GIT_AUTHOR_DATE="+baseDate,
		"GIT_COMMITTER_NAME=choir-builder", "GIT_COMMITTER_EMAIL=builder@choir.local", "GIT_COMMITTER_DATE="+baseDate)
	if out, commitErr := commitCmd.CombinedOutput(); commitErr != nil {
		cleanup()
		return "", "", "", nil, fmt.Errorf("git commit: %w: %s", commitErr, strings.TrimSpace(string(out)))
	}
	head, err := runGitOut(ctx, tmp, "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	// H15, H16: admit what git committed, not what the patch text says.
	raw, err := runGitOut(ctx, tmp, "diff", "--raw", "-z", "--no-renames", "--no-abbrev", baseCommit, "HEAD")
	if err != nil {
		cleanup()
		return "", "", "", nil, fmt.Errorf("read the committed changes: %w", err)
	}
	changes, err := parseRawDiff(raw)
	if err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	if err := RefuseTreeMetadata(changes); err != nil {
		cleanup()
		return "", "", "", nil, err
	}
	if admit != nil {
		if err := admit(changes); err != nil {
			cleanup()
			return "", "", "", nil, fmt.Errorf("patched tree refused: %w", err)
		}
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

// syntheticGitEnv is the environment the patch path runs git in: the process
// environment without any git identity, date or config override, so nothing
// the caller or the host user's git config sets changes the synthetic commit.
func syntheticGitEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
}

func runGitOut(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = syntheticGitEnv()
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
