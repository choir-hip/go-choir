package sourceproj

import (
	"context"
	"fmt"
	"strings"
)

// RefSeedPrefix names the seed commits a package Cast pinned, so git keeps
// them while a capsule's source artifact names them.
const RefSeedPrefix = "refs/choir/seed/"

// SeedCommit writes the commit a package operation's capsule is cast from:
// head's tree with the package subtree at root replaced by the tree the
// treePatch (the installed release's tree, as an empty-tree to tree patch with
// the root prefix) builds. It carries head's stack base as a trailer, so the
// release the operation drafts names the platform base, not the seed. The
// commit is deterministic in its inputs.
func SeedCommit(ctx context.Context, repo, head, root string, treePatch []byte) (string, error) {
	full, ok := resolveCommit(ctx, repo, head)
	if !ok {
		return "", fmt.Errorf("seed: %s is not in the source repository", head)
	}
	head = full
	root = strings.Trim(root, "/")
	if root == "" {
		return "", fmt.Errorf("seed: no package root")
	}
	var tree string
	err := scratchIndex(ctx, repo, func(env []string) error {
		if _, err := git(ctx, repo, env, nil, "read-tree", head); err != nil {
			return fmt.Errorf("read head tree: %w", err)
		}
		// The installed tree replaces whatever the base holds at the root.
		if _, err := git(ctx, repo, env, nil, "rm", "-r", "-q", "-f", "--cached", "--ignore-unmatch", "--", root); err != nil {
			return fmt.Errorf("clear %s: %w", root, err)
		}
		if _, err := git(ctx, repo, env, treePatch, "apply", "--cached", "--whitespace=nowarn", "-"); err != nil {
			return fmt.Errorf("%w: %v", ErrPatchDoesNotApply, err)
		}
		out, err := git(ctx, repo, env, nil, "write-tree")
		if err != nil || !commitID.MatchString(out) {
			return fmt.Errorf("write tree: %v", err)
		}
		tree = out
		return nil
	})
	if err != nil {
		return "", err
	}
	stackBase, stacked := StackBase(ctx, repo, head)
	if !stacked {
		stackBase = head
	}
	env := []string{
		"GIT_AUTHOR_NAME=Choir Projection", "GIT_AUTHOR_EMAIL=projection@choir.local", "GIT_AUTHOR_DATE=1000000000 +0000",
		"GIT_COMMITTER_NAME=Choir Projection", "GIT_COMMITTER_EMAIL=projection@choir.local", "GIT_COMMITTER_DATE=1000000000 +0000",
	}
	msg := fmt.Sprintf("choir: seed %s\n\n%s: %s\n", root, TrailerStackBase, stackBase)
	commit, err := git(ctx, repo, env, []byte(msg), "-c", "commit.gpgsign=false", "commit-tree", tree, "-p", head, "-F", "-")
	if err != nil || !commitID.MatchString(commit) {
		return "", fmt.Errorf("seed: write commit: %v", err)
	}
	if _, err := git(ctx, repo, nil, nil, "update-ref", RefSeedPrefix+commit, commit); err != nil {
		return "", fmt.Errorf("seed: keep %s: %w", commit, err)
	}
	return commit, nil
}
