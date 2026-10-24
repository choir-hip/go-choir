package sourceproj

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// StackBase reads the stack base a projection commit records. ok is false for
// a commit the projection did not write (the base itself, a seed commit).
func StackBase(ctx context.Context, repo, commit string) (string, bool) {
	if !commitID.MatchString(strings.ToLower(strings.TrimSpace(commit))) {
		return "", false
	}
	msg, err := git(ctx, repo, nil, nil, "log", "-1", "--format=%B", commit)
	if err != nil {
		return "", false
	}
	prefix := TrailerStackBase + ": "
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, prefix) {
			base := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if commitID.MatchString(base) {
				return base, true
			}
		}
	}
	return "", false
}

// Cumulative turns a candidate's patch, authored against pinned (the commit the
// capsule was cast from), into the patch from the stack base. When pinned is
// not a projection commit there is no stack and the candidate's patch is
// returned unchanged with pinned as its base, byte for byte what Freeze wrote
// before U1.
//
// The bundle's patch has to be the cumulative one: corpusd names the host
// build from the bundle's patch and base, and the builder accepts no base but
// the booted image's commit (section 4.6, "U1 build notes").
func Cumulative(ctx context.Context, repo, pinned string, patch []byte) (cumulative []byte, base string, err error) {
	if !Enabled() {
		return patch, pinned, nil
	}
	stackBase, ok := StackBase(ctx, repo, pinned)
	if !ok {
		return patch, pinned, nil
	}
	pinnedFull, ok := resolveCommit(ctx, repo, pinned)
	if !ok {
		return nil, "", fmt.Errorf("sourceproj: pinned commit %s is not in the source repository", pinned)
	}
	target, err := patchedTree(ctx, repo, pinnedFull, patch, false)
	if err != nil {
		return nil, "", fmt.Errorf("sourceproj: the candidate patch does not apply to its pinned source %s: %w", pinnedFull, err)
	}
	raw, err := gitRaw(ctx, repo, nil, nil, "diff-tree", "-p", "-r", "--binary", "--no-renames", "--no-color", "--no-ext-diff", stackBase, target)
	if err != nil {
		return nil, "", fmt.Errorf("sourceproj: compose the cumulative patch: %w", err)
	}
	if len(raw) == 0 {
		return nil, "", fmt.Errorf("sourceproj: the candidate leaves the tree equal to the stack base %s, so there is nothing to release", stackBase)
	}
	// F1: the composed patch must reproduce exactly the tree the candidate
	// built on, or Freeze must not vouch for it.
	got, err := patchedTree(ctx, repo, stackBase, raw, false)
	if err != nil || got != target {
		return nil, "", fmt.Errorf("sourceproj: the cumulative patch does not reproduce the candidate tree (got %s, want %s): %v", got, target, err)
	}
	return raw, stackBase, nil
}

// SHA256 is the digest a bundle records for a patch.
func SHA256(patch []byte) string {
	sum := sha256.Sum256(patch)
	return hex.EncodeToString(sum[:])
}
