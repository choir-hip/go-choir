package sourceproj

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Clean re-application (owner ruling 2026-11-04, "clean rebases without
// approval"; docs/design/u4-stack-rebase-build-2026-10-26.md, "Owner
// ruling"). When the applied stack sits on a base the computer no longer
// booted (stack_base_moved), the stack's own patch bytes are tried against
// the booted base the way the host builder applies a patch (`git apply`, no
// three-way merge, no fuzz; a hunk may only move by an offset). The same
// bytes, not a re-derived diff, are what a clean rebase carries, so the
// change applied is the change the owner approved.

// ReapplyVerdict is what Reapply found.
type ReapplyVerdict struct {
	// Clean: the patch applies to the booted base as it is, and changes it.
	Clean bool `json:"clean"`
	// Tree is the booted base's tree with the patch applied (Clean only).
	Tree string `json:"tree,omitempty"`
	// Reason says why it is not clean.
	Reason string `json:"reason,omitempty"`
}

// Reapply checks whether patch, approved on stackBase, applies unchanged to
// booted. An error is a failure to check (a missing object, git failing);
// a patch that does not apply is a verdict, not an error.
func Reapply(ctx context.Context, repo, stackBase, booted string, patch []byte) (ReapplyVerdict, error) {
	stackBase, booted = strings.TrimSpace(stackBase), strings.TrimSpace(booted)
	if len(patch) == 0 {
		return ReapplyVerdict{Reason: "the applied change carries no source patch"}, nil
	}
	if stackBase == booted {
		return ReapplyVerdict{Reason: "the stack is already on the booted base"}, nil
	}
	// A scratch index: HEAD, the work tree and the refs are not touched, so
	// this needs no lock beside Reconcile (git object writes are safe).
	bootedCommit, ok := resolveCommit(ctx, repo, booted)
	if !ok {
		return ReapplyVerdict{}, fmt.Errorf("the booted base %s is not in the repository", booted)
	}
	if _, ok := resolveCommit(ctx, repo, stackBase); !ok {
		return ReapplyVerdict{}, fmt.Errorf("the stack base %s is not in the repository", stackBase)
	}
	tree, err := patchedTree(ctx, repo, bootedCommit, patch, false)
	if errors.Is(err, ErrPatchDoesNotApply) {
		return ReapplyVerdict{Reason: "the applied change does not apply to the booted base: " + strings.TrimSpace(strings.TrimPrefix(err.Error(), ErrPatchDoesNotApply.Error()+":"))}, nil
	}
	if err != nil {
		return ReapplyVerdict{}, err
	}
	bootedTree, err := git(ctx, repo, nil, nil, "rev-parse", bootedCommit+"^{tree}")
	if err != nil {
		return ReapplyVerdict{}, err
	}
	if tree == bootedTree {
		return ReapplyVerdict{Reason: "the applied change makes no difference on the booted base"}, nil
	}
	return ReapplyVerdict{Clean: true, Tree: tree}, nil
}
