package pkgrelease

import "context"

// Subtree is the tree id at root in treeish (a commit or a tree) in repo.
// present is false when there is no such directory.
func Subtree(ctx context.Context, repo, treeish, root string) (tree string, present bool, err error) {
	return "", false, errNotBuilt
}

// DiffPaths lists the paths that differ between two trees, sorted, with no
// rename detection (a rename is its two paths).
func DiffPaths(ctx context.Context, repo, a, b string) ([]string, error) { return nil, errNotBuilt }

// OutsideRoot lists the paths that are not root itself or under it.
func OutsideRoot(paths []string, root string) []string { return nil }

// Blobs are a release's three patches.
type Blobs struct {
	TreePatch   []byte
	ReviewPatch []byte
	// ParentPatch is nil when the parent is the empty tree.
	ParentPatch []byte
}

// BuildBlobs makes the three patches of a release whose subtree at root is
// tree and whose parent subtree is parentTree (EmptyTree for a new package):
// tree_patch from the empty tree to the full-path tree holding only root,
// review_patch from the parent's full-path tree to it, and parent_patch from
// the empty tree to the parent's. Every object it writes goes to a
// throwaway directory; repo's objects are a read-only alternate.
func BuildBlobs(ctx context.Context, repo, root, parentTree, tree string) (Blobs, error) {
	return Blobs{}, errNotBuilt
}

// ReadFile reads one file of a tree.
func ReadFile(ctx context.Context, repo, tree, path string) ([]byte, error) { return nil, errNotBuilt }

// CheckManifest validates the package's choir-package.json bytes for a
// release of package name: the manifest parses, names the package, and
// declares no kinds before S12 (package_kinds_need_platform). It returns
// the record's manifest summary and the manifest version.
func CheckManifest(name string, raw []byte) (Manifest, string, error) {
	return Manifest{}, "", errNotBuilt
}

// AddedSecrets runs the release's refusal-grade secret scan over the lines
// a patch adds.
func AddedSecrets(patch []byte) []string { return nil }
