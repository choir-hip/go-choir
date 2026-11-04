package pkgrelease

import (
	"context"
	"fmt"
	"os"
)

// Export side of a package release (design 10.2): rebuild the release's
// package subtree inside a local repository from bytes the caller fetched,
// and prove it is the record's tree. Both functions write the rebuilt
// objects into repo (they are about to be committed there) and touch
// neither its index nor its working tree.
//
// Failure modes, written before the code:
//
//	E1  the patch is not the record's (another blob, truncated): VerifyBlob
//	    refuses it before git sees it;
//	E2  the patch names a path outside the package root (../, another
//	    package, a dotfile at the top) or carries a symlink, submodule,
//	    odd mode or .gitattributes: confined refuses the rebuilt tree;
//	E3  the rebuilt subtree is not record.source.tree (a patch of another
//	    release, a source patch of a later operation): tree_mismatch;
//	E4  built_on is not in repo (a source-patch rebuild): refused, never
//	    fetched.

// ExportTreeFromBlob rebuilds the release tree from its source.tree_patch
// (the catalog's blob route) and returns the subtree id, which equals
// record.Source.Tree.
func ExportTreeFromBlob(ctx context.Context, repo string, rec Record, treePatch []byte) (string, error) {
	if err := VerifyBlob(rec.Source.TreePatch, treePatch); err != nil {
		return "", err
	}
	return exportTree(ctx, repo, rec, EmptyTree, treePatch, true)
}

// ExportTreeFromSourcePatch rebuilds the release tree from the publisher's
// self-development source patch applied to record.Source.BuiltOn (choir
// self-dev patch); the patch may change other paths, and only the
// package subtree is taken. The tree id check is the verification.
func ExportTreeFromSourcePatch(ctx context.Context, repo string, rec Record, sourcePatch []byte) (string, error) {
	if len(sourcePatch) == 0 {
		return "", refuse(ReasonInvalid, "an empty source patch")
	}
	base, err := gitLine(ctx, repo, nil, nil, "rev-parse", "--verify", "-q", rec.Source.BuiltOn+"^{tree}")
	if err != nil || !gitObjectID.MatchString(base) {
		return "", refuse(ReasonParentUnverifiable, "built_on %s is not in this repository; fetch it first", rec.Source.BuiltOn)
	}
	return exportTree(ctx, repo, rec, base, sourcePatch, false)
}

// exportTree applies patch onto the tree onto and takes the subtree at the
// root. wholeConfined requires the whole result to lie under the root (a
// tree_patch); otherwise only the subtree is checked (a source patch may
// change other paths, which are left behind).
func exportTree(ctx context.Context, repo string, rec Record, onto string, patch []byte, wholeConfined bool) (string, error) {
	rec = rec.Normalize()
	root := rec.Source.Root
	if root != Root(rec.Package.Name) {
		return "", refuse(ReasonInvalid, "source.root %q is not %s", root, Root(rec.Package.Name))
	}
	tmp, err := os.MkdirTemp("", "choir-pkgexport-")
	if err != nil {
		return "", err
	}
	s := &scratch{dir: repo, tmp: tmp}
	defer s.close()
	full, err := s.apply(ctx, onto, patch)
	if err != nil {
		return "", refuse(ReasonInvalid, "the patch does not apply: %v", err)
	}
	tree, present, err := subtreeIn(ctx, s.dir, nil, full, root)
	if err != nil || !present {
		return "", refuse(ReasonTreeMismatch, "the patch leaves no %s", root)
	}
	if tree != rec.Source.Tree {
		return "", refuse(ReasonTreeMismatch, "the patch makes %s at %s; the record names %s", tree, root, rec.Source.Tree)
	}
	only := full
	if !wholeConfined {
		if only, err = s.prefixed(ctx, root, tree); err != nil {
			return "", fmt.Errorf("pkgrelease: prefix the subtree: %w", err)
		}
	}
	if err := s.confined(ctx, only, root); err != nil {
		return "", err
	}
	return tree, nil
}
