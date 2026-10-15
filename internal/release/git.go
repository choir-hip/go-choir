package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrPatchDoesNotApply refuses a patch that does not apply to a tree: a
// conflict for the adopter, never a merge (owner ruling: no rebase work for
// Station B).
var ErrPatchDoesNotApply = errors.New("release patch does not apply to this tree")

// gitTimeout bounds every git call: a read path must not hang on git.
const gitTimeout = 60 * time.Second

// Head returns the repository's HEAD commit.
func Head(ctx context.Context, repo string) (string, error) {
	out, err := gitOutput(ctx, repo, nil, nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", fmt.Errorf("release: source HEAD: %w", err)
	}
	return out, nil
}

// CommitTree returns commit^{tree}.
func CommitTree(ctx context.Context, repo, commit string) (string, error) {
	if !gitObjectID.MatchString(commit) {
		return "", fmt.Errorf("release: commit must be a git object id")
	}
	out, err := gitOutput(ctx, repo, nil, nil, "rev-parse", "--verify", commit+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("release: commit %s not in the source repository: %w", commit, err)
	}
	return out, nil
}

// PatchedTree applies patch to commit's tree and returns the resulting git
// tree id, without writing to the repository: the index and every new
// object go to a throwaway directory, with the repository's objects as a
// read-only alternate (failure modes I3, A7). A patch that does not apply
// returns ErrPatchDoesNotApply with git's reason.
func PatchedTree(ctx context.Context, repo, commit string, patch []byte) (string, error) {
	if _, err := CommitTree(ctx, repo, commit); err != nil {
		return "", err
	}
	objects, err := gitOutput(ctx, repo, nil, nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("release: locate repository objects: %w", err)
	}
	if !filepath.IsAbs(objects) {
		objects = filepath.Join(repo, objects)
	}
	objects = filepath.Join(objects, "objects")
	scratch, err := os.MkdirTemp("", "choir-release-tree-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	if err := os.MkdirAll(filepath.Join(scratch, "objects"), 0o700); err != nil {
		return "", err
	}
	env := []string{
		"GIT_INDEX_FILE=" + filepath.Join(scratch, "index"),
		"GIT_OBJECT_DIRECTORY=" + filepath.Join(scratch, "objects"),
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=" + objects,
	}
	if _, err := gitOutput(ctx, repo, env, nil, "read-tree", commit); err != nil {
		return "", fmt.Errorf("release: read base tree: %w", err)
	}
	if _, err := gitOutput(ctx, repo, env, patch, "apply", "--cached", "--whitespace=nowarn", "-"); err != nil {
		return "", fmt.Errorf("%w: %v", ErrPatchDoesNotApply, err)
	}
	tree, err := gitOutput(ctx, repo, env, nil, "write-tree")
	if err != nil {
		return "", fmt.Errorf("release: write patched tree: %w", err)
	}
	if !gitObjectID.MatchString(tree) {
		return "", fmt.Errorf("release: git wrote a malformed tree id %q", tree)
	}
	return tree, nil
}

func gitOutput(ctx context.Context, repo string, env []string, stdin []byte, args ...string) (string, error) {
	if strings.TrimSpace(repo) == "" {
		return "", fmt.Errorf("source repository is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	// A caller's GIT_* environment must not redirect the repository.
	base := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			base = append(base, kv)
		}
	}
	cmd.Env = append(base, env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		reason := strings.TrimSpace(stderr.String())
		if len(reason) > 600 {
			reason = reason[:600]
		}
		if reason == "" {
			reason = err.Error()
		}
		return "", errors.New(reason)
	}
	return strings.TrimSpace(stdout.String()), nil
}
