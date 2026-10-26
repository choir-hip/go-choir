package pkgrelease

import (
	"context"
	"errors"
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

// PackageTree reads git rev-parse <commit>:<root>.
func (g GitStore) PackageTree(ctx context.Context, commit, root string) (string, bool, error) {
	return "", false, errNotBuilt
}

// Shallow reports git rev-parse --is-shallow-repository on the store. A
// shallow store refuses every base parent older than the deploy, which
// fails closed; corpusd reports the answer in its health detail.
func (g GitStore) Shallow(ctx context.Context) (bool, error) { return false, errNotBuilt }

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
func VerifyHost(ctx context.Context, in HostInput) error { return errNotBuilt }
