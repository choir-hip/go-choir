package sourceproj

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/packagepath"
)

// RefKeptPrefix + an operation id holds the projection commit of a stack the
// forced-modularity migration took apart (docs/design/forced-modularity-build-2026-11-04.md,
// "Build notes: M3 boot migration"): the whole stack, package source
// included, kept so nothing the migration moves out is ever lost. The ref is
// created, never moved.
const RefKeptPrefix = "refs/choir/kept/"

// StackSplit is a stack patch sorted by layer: its after-tree, the platform
// paths it changes, and for each package root it changes, that root's own
// patch on the same base.
type StackSplit struct {
	Tree     string
	Platform []string
	Roots    []string
	ByRoot   map[string][]byte
}

// SplitStack applies patch to base and splits what it changes by layer. Each
// root's patch is git's binary diff of the root between base and the patched
// tree, so it applies to base on its own.
func SplitStack(ctx context.Context, repo, base string, patch []byte) (StackSplit, error) {
	var out StackSplit
	commit, ok := resolveCommit(ctx, repo, base)
	if !ok {
		return out, fmt.Errorf("the stack base %s is not in the guest source repository", base)
	}
	tree, err := patchedTree(ctx, repo, commit, patch, false)
	if err != nil {
		return out, err
	}
	paths, err := treeDiffPaths(ctx, repo, commit, tree, nil)
	if err != nil {
		return out, err
	}
	split := packagepath.Classify(paths)
	out.Tree, out.Platform, out.Roots, out.ByRoot = tree, split.Platform, split.Roots, map[string][]byte{}
	baseTree, err := git(ctx, repo, nil, nil, "rev-parse", commit+"^{tree}")
	if err != nil {
		return out, err
	}
	for _, root := range split.Roots {
		raw, err := gitRaw(ctx, repo, nil, nil, "diff", "--binary", "--full-index", "--no-color", "--no-renames", "--no-ext-diff", baseTree, tree, "--", ":(literal)"+root)
		if err != nil {
			return out, err
		}
		out.ByRoot[root] = raw
	}
	return out, nil
}

// KeepStack writes the stack's deterministic projection commit (the same
// commit reconcile would project) and points RefKeptPrefix+op at it. It is
// idempotent: the commit is a function of the operation, and an existing ref
// is checked, never moved.
func KeepStack(ctx context.Context, repo string, op Op) (string, error) {
	if strings.TrimSpace(op.OperationID) == "" || strings.ContainsAny(op.OperationID, " /\\:~^?*[") {
		return "", fmt.Errorf("keep stack: operation id %q cannot name a ref", op.OperationID)
	}
	base, ok := resolveCommit(ctx, repo, op.Base)
	if !ok {
		return "", fmt.Errorf("keep stack: the stack base %s is not in the guest source repository", op.Base)
	}
	tree, err := patchedTree(ctx, repo, base, op.Patch, false)
	if err != nil {
		return "", err
	}
	commit, err := projectionCommit(ctx, repo, &op, base, tree)
	if err != nil {
		return "", err
	}
	ref := RefKeptPrefix + op.OperationID
	if existing, ok := resolveCommit(ctx, repo, ref); ok {
		if existing != commit {
			return "", fmt.Errorf("keep stack: %s already names %s, not %s", ref, existing, commit)
		}
		return commit, nil
	}
	// Created only: the zero old value refuses to move an existing ref.
	if _, err := git(ctx, repo, nil, nil, "update-ref", ref, commit, strings.Repeat("0", len(commit))); err != nil {
		return "", err
	}
	return commit, nil
}

// SubtreeAt is the tree id at path in treeish, and whether it is there.
func SubtreeAt(ctx context.Context, repo, treeish, path string) (string, bool) {
	out, err := git(ctx, repo, nil, nil, "rev-parse", "--verify", "--quiet", treeish+":"+path)
	if err != nil || !commitID.MatchString(out) {
		return "", false
	}
	return out, true
}
