package sourceproj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// gitTimeout bounds every git call: reconcile runs on the cast path and must
// not hang on git.
const gitTimeout = 120 * time.Second

var commitID = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// git runs one git command in repo and returns stdout without the trailing
// newline. A caller's GIT_* environment must not redirect the repository, and
// neither the user's nor the system's git config may change what a plumbing
// command writes (the projection commit must be reproducible).
func git(ctx context.Context, repo string, env []string, stdin []byte, args ...string) (string, error) {
	out, err := gitRaw(ctx, repo, env, stdin, args...)
	return strings.TrimRight(string(out), "\n"), err
}

func gitRaw(ctx context.Context, repo string, env []string, stdin []byte, args ...string) ([]byte, error) {
	if strings.TrimSpace(repo) == "" {
		return nil, errors.New("source repository is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	// No hook of the repository's own config may run under a ref-mutating call.
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo, "-c", "core.hooksPath=" + os.DevNull}, args...)...)
	base := make([]string, 0, len(os.Environ())+4)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			base = append(base, kv)
		}
	}
	base = append(base, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0")
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
		return stdout.Bytes(), &gitError{args: args, reason: reason, cause: err}
	}
	return stdout.Bytes(), nil
}

type gitError struct {
	args   []string
	reason string
	cause  error
}

func (e *gitError) Error() string { return fmt.Sprintf("git %s: %s", e.args[0], e.reason) }
func (e *gitError) Unwrap() error { return e.cause }

// exitCode1 reports whether err is a git command that exited with status 1
// (the "differences found" answer of diff --quiet and apply --check).
func exitCode1(err error) bool {
	var ge *gitError
	if !errors.As(err, &ge) {
		return false
	}
	var ee *exec.ExitError
	return errors.As(ge.cause, &ee) && ee.ExitCode() == 1
}

// resolveCommit returns the full id of rev when it names a commit in repo.
func resolveCommit(ctx context.Context, repo, rev string) (string, bool) {
	rev = strings.TrimSpace(rev)
	if rev == "" || strings.HasPrefix(rev, "-") {
		return "", false
	}
	out, err := git(ctx, repo, nil, nil, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil || !commitID.MatchString(out) {
		return "", false
	}
	return out, true
}

// scratchIndex runs fn with a throwaway index file; every object a plumbing
// command writes goes to repo's real object store (a tree the projection
// commits has to live there).
func scratchIndex(ctx context.Context, repo string, fn func(env []string) error) error {
	dir, err := os.MkdirTemp("", "choir-sourceproj-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return fn([]string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")})
}

// patchedTree applies patch to base's tree in a scratch index and returns the
// resulting tree id, written to repo's object store. A patch that does not
// apply returns an ErrPatchDoesNotApply-wrapped error.
func patchedTree(ctx context.Context, repo, base string, patch []byte, reverse bool) (string, error) {
	var tree string
	err := scratchIndex(ctx, repo, func(env []string) error {
		if _, err := git(ctx, repo, env, nil, "read-tree", base); err != nil {
			return fmt.Errorf("read base tree: %w", err)
		}
		args := []string{"apply", "--cached", "--whitespace=nowarn"}
		if reverse {
			args = append(args, "--reverse")
		}
		if _, err := git(ctx, repo, env, patch, append(args, "-")...); err != nil {
			return fmt.Errorf("%w: %v", ErrPatchDoesNotApply, err)
		}
		out, err := git(ctx, repo, env, nil, "write-tree")
		if err != nil {
			return fmt.Errorf("write tree: %w", err)
		}
		if !commitID.MatchString(out) {
			return fmt.Errorf("git wrote a malformed tree id %q", out)
		}
		tree = out
		return nil
	})
	return tree, err
}

// ErrPatchDoesNotApply marks a patch that does not apply to its base tree.
var ErrPatchDoesNotApply = errors.New("the patch does not apply to its base")

// treeDiffPaths lists the paths whose entry (mode or blob) differs between
// two trees, optionally limited to pathspecs. Quoting is off (-z).
func treeDiffPaths(ctx context.Context, repo, a, b string, only []string) ([]string, error) {
	var out []string
	run := func(specs []string) error {
		args := []string{"diff-tree", "-r", "--no-renames", "--name-only", "-z", a, b}
		if len(specs) > 0 {
			args = append(args, "--")
			args = append(args, specs...)
		}
		raw, err := gitRaw(ctx, repo, nil, nil, args...)
		if err != nil {
			return err
		}
		for _, p := range strings.Split(string(raw), "\x00") {
			if p != "" {
				out = append(out, p)
			}
		}
		return nil
	}
	if len(only) == 0 {
		return out, run(nil)
	}
	const chunk = 100
	for i := 0; i < len(only); i += chunk {
		end := i + chunk
		if end > len(only) {
			end = len(only)
		}
		specs := make([]string, 0, end-i)
		for _, p := range only[i:end] {
			// A literal path, never a glob.
			specs = append(specs, ":(literal)"+p)
		}
		if err := run(specs); err != nil {
			return nil, err
		}
	}
	return out, nil
}
