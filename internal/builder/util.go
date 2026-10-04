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
