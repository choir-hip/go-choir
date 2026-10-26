package pkgrelease

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrBaseUnavailable is a base commit, or its object store, the host does
// not hold.
var ErrBaseUnavailable = errors.New("pkgrelease: base commit not in the host object store")

// BaseTrees reads a package subtree of a platform base commit from the
// host's own object store (design 3.2, check 3).
type BaseTrees interface {
	// PackageTree is the tree id at root in commit. present is false when
	// the commit holds no such directory (a new package). An error wraps
	// ErrBaseUnavailable when the host does not hold the commit.
	PackageTree(ctx context.Context, commit, root string) (tree string, present bool, err error)
}

// GitStore is the deploy checkout's object store, read only: the
// directory the host build already reads (CORPUSD_HOST_BUILD_SOURCE_DIR).
// Only rev-parse runs there; the worktree is never read.
type GitStore struct {
	Dir string
}

// PackageTree reads git rev-parse <commit>:<root>. A commit the store
// does not hold, or a store that is not there, is ErrBaseUnavailable.
func (g GitStore) PackageTree(ctx context.Context, commit, root string) (string, bool, error) {
	if !gitObjectID.MatchString(commit) {
		return "", false, fmt.Errorf("%w: %q is not a commit id", ErrBaseUnavailable, commit)
	}
	if info, err := os.Stat(g.Dir); err != nil || !info.IsDir() {
		return "", false, fmt.Errorf("%w: no object store at %q", ErrBaseUnavailable, g.Dir)
	}
	if _, err := gitLine(ctx, g.Dir, nil, nil, "rev-parse", "--verify", "-q", commit+"^{commit}"); err != nil {
		return "", false, fmt.Errorf("%w: %s", ErrBaseUnavailable, commit)
	}
	tree, present, err := subtreeIn(ctx, g.Dir, nil, commit, root)
	if err != nil {
		return "", false, fmt.Errorf("%w: %v", ErrBaseUnavailable, err)
	}
	return tree, present, nil
}

// Shallow reports git rev-parse --is-shallow-repository on the store. A
// shallow store refuses every base parent older than the deploy, which
// fails closed; corpusd reports the answer in its health detail.
func (g GitStore) Shallow(ctx context.Context) (bool, error) {
	out, err := gitLine(ctx, g.Dir, nil, nil, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	switch out {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("git answered %q to --is-shallow-repository", out)
}

// HostInput is what corpusd hands the host checks: the record and the
// bytes it stores.
type HostInput struct {
	Record      Record
	TreePatch   []byte
	ReviewPatch []byte
	// ParentPatch is the record's parent_patch bytes (a base parent).
	ParentPatch []byte
	// ParentRelease and ParentTreePatch are the listed parent release and
	// its stored tree_patch (a release parent).
	ParentRelease   *Record
	ParentTreePatch []byte
	// Base is the host's object store; nil refuses every base parent with
	// package_parent_unverifiable.
	Base BaseTrees
}

// VerifyHost runs the host-verified checks of design 3.2 on the stored
// bytes: the content is confined to the root, the subtree matches, the
// review patch matches from a parent the host itself verified, no secret is
// added, and the manifest validates. The track head (track_moved) is the
// caller's check: it needs the listings.
//
// A release parent's id is the caller's to have resolved: ParentRelease is
// the listed record it read under that id.
func VerifyHost(ctx context.Context, in HostInput) error {
	rec := in.Record.Normalize()
	if err := rec.Validate(); err != nil {
		return err
	}
	root := rec.Source.Root
	if err := VerifyBlob(rec.Source.TreePatch, in.TreePatch); err != nil {
		return err
	}
	if err := VerifyBlob(rec.Source.ReviewPatch, in.ReviewPatch); err != nil {
		return err
	}
	switch {
	case rec.ParentPatch != nil:
		if err := VerifyBlob(*rec.ParentPatch, in.ParentPatch); err != nil {
			return err
		}
	case len(in.ParentPatch) != 0:
		return refuse(ReasonInvalid, "parent_patch bytes for a record that names none")
	}
	s, err := newScratch(ctx, "")
	if err != nil {
		return fmt.Errorf("pkgrelease: scratch repository: %w", err)
	}
	defer s.close()

	// Check 1: the content, applied to the empty tree, is confined.
	full, err := s.apply(ctx, EmptyTree, in.TreePatch)
	if err != nil {
		return refuse(ReasonInvalid, "tree_patch does not apply to the empty tree: %v", err)
	}
	if err := s.confined(ctx, full, root); err != nil {
		return err
	}
	// Check 2: the subtree at the root is the record's tree.
	tree, present, err := subtreeIn(ctx, s.dir, nil, full, root)
	if err != nil || !present || tree != rec.Source.Tree {
		return refuse(ReasonTreeMismatch, "tree_patch makes %q at %s, the record names %s", tree, root, rec.Source.Tree)
	}

	// Check 3: the parent's full-path tree, from bytes the host verified.
	parentFull, err := hostParent(ctx, s, rec, in)
	if err != nil {
		return err
	}
	reviewed, err := s.apply(ctx, parentFull, in.ReviewPatch)
	if err != nil {
		return refuse(ReasonReviewMismatch, "review_patch does not apply to the parent: %v", err)
	}
	// Both sides hold only the root, so the review patch must rebuild the
	// release's full tree exactly: the same subtree and nothing beside it.
	if reviewed != full {
		return refuse(ReasonReviewMismatch, "review_patch turns the parent into tree %s, not the release's %s", reviewed, full)
	}

	// Check 4: no added line carries a secret.
	if kinds := AddedSecrets(in.TreePatch); len(kinds) > 0 {
		return refuse(ReasonSecretFound, "tree_patch adds secret content (%s)", strings.Join(kinds, ", "))
	}

	// Check 5: the manifest validates and is the record's summary.
	raw, err := gitRun(ctx, s.dir, nil, nil, "cat-file", "blob", full+":"+root+"/choir-package.json")
	if err != nil || len(raw) > maxManifestBytes {
		return refuse(ReasonManifestInvalid, "the release holds no readable %s/choir-package.json", root)
	}
	summary, version, err := CheckManifest(rec.Package.Name, raw)
	if err != nil {
		return err
	}
	if summary != rec.Manifest || version != rec.Package.ManifestVersion {
		return refuse(ReasonManifestInvalid, "the record's manifest summary %+v (version %s) is not the bytes' %+v (version %s)", rec.Manifest, rec.Package.ManifestVersion, summary, version)
	}
	return nil
}

// hostParent is the parent's full-path tree in s, from the parent release's
// stored tree_patch, or from parent_patch checked against the host's own
// base tree (review 265, finding 2).
func hostParent(ctx context.Context, s *scratch, rec Record, in HostInput) (string, error) {
	root := rec.Source.Root
	if rec.Parent.Kind == ParentRelease {
		parent := in.ParentRelease
		if parent == nil || in.ParentTreePatch == nil {
			return "", refuse(ReasonParentMismatch, "the parent release %s is not a listed release on this track", rec.Parent.ReleaseID)
		}
		if parent.Source.Tree != rec.Parent.Tree || parent.Package.Name != rec.Package.Name || parent.Track != rec.Track {
			return "", refuse(ReasonParentMismatch, "the parent release holds %s of %s on track %s; the record names %s of %s on %s",
				parent.Source.Tree, parent.Package.Name, parent.Track, rec.Parent.Tree, rec.Package.Name, rec.Track)
		}
		if err := VerifyBlob(parent.Source.TreePatch, in.ParentTreePatch); err != nil {
			return "", err
		}
		full, err := s.apply(ctx, EmptyTree, in.ParentTreePatch)
		if err != nil {
			return "", refuse(ReasonParentMismatch, "the parent release's tree_patch does not apply: %v", err)
		}
		if tree, present, err := subtreeIn(ctx, s.dir, nil, full, root); err != nil || !present || tree != rec.Parent.Tree {
			return "", refuse(ReasonParentMismatch, "the parent release's stored tree is %q, not %s", tree, rec.Parent.Tree)
		}
		return full, nil
	}
	if in.Base == nil {
		return "", refuse(ReasonParentUnverifiable, "this host holds no base object store, so the base %s cannot be read", rec.Parent.Commit)
	}
	baseTree, basePresent, err := in.Base.PackageTree(ctx, rec.Parent.Commit, root)
	if err != nil {
		return "", refuse(ReasonParentUnverifiable, "the host cannot read %s at base %s: %v", root, rec.Parent.Commit, err)
	}
	if rec.Parent.Tree == EmptyTree {
		if basePresent {
			return "", refuse(ReasonParentMismatch, "the base %s holds %s at %s, so the parent is not the empty tree", rec.Parent.Commit, baseTree, root)
		}
		return EmptyTree, nil
	}
	full, err := s.apply(ctx, EmptyTree, in.ParentPatch)
	if err != nil {
		return "", refuse(ReasonParentMismatch, "parent_patch does not apply to the empty tree: %v", err)
	}
	if err := s.confined(ctx, full, root); err != nil {
		return "", err
	}
	tree, present, err := subtreeIn(ctx, s.dir, nil, full, root)
	if err != nil || !present || tree != rec.Parent.Tree {
		return "", refuse(ReasonParentMismatch, "parent_patch makes %q at %s, the record names %s", tree, root, rec.Parent.Tree)
	}
	if !basePresent || baseTree != rec.Parent.Tree {
		return "", refuse(ReasonParentMismatch, "the host's base %s holds %q at %s, the record names %s", rec.Parent.Commit, baseTree, root, rec.Parent.Tree)
	}
	return full, nil
}
