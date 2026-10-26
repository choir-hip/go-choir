package pkgrelease

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
)

// Subtree is the tree id at root in treeish (a commit or a tree) in repo.
// present is false when there is no such directory.
func Subtree(ctx context.Context, repo, treeish, root string) (tree string, present bool, err error) {
	return subtreeIn(ctx, repo, nil, treeish, root)
}

// DiffPaths lists the paths that differ between two trees, sorted, with no
// rename detection (a rename is its two paths).
func DiffPaths(ctx context.Context, repo, a, b string) ([]string, error) {
	out, err := gitRun(ctx, repo, nil, nil, "diff-tree", "-r", "-z", "--name-only", "--no-renames", "--no-commit-id", a, b)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// OutsideRoot lists the paths that are not root itself or under it, in
// their order.
func OutsideRoot(paths []string, root string) []string {
	var out []string
	for _, p := range paths {
		if p != root && !strings.HasPrefix(p, root+"/") {
			out = append(out, p)
		}
	}
	return out
}

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
	s, err := newScratch(ctx, repo)
	if err != nil {
		return Blobs{}, err
	}
	defer s.close()
	full, err := s.prefixed(ctx, root, tree)
	if err != nil {
		return Blobs{}, fmt.Errorf("pkgrelease: the release tree at %s: %w", root, err)
	}
	parentFull, err := s.prefixed(ctx, root, parentTree)
	if err != nil {
		return Blobs{}, fmt.Errorf("pkgrelease: the parent tree at %s: %w", root, err)
	}
	var out Blobs
	if out.TreePatch, err = s.diff(ctx, EmptyTree, full); err != nil {
		return Blobs{}, err
	}
	if out.ReviewPatch, err = s.diff(ctx, parentFull, full); err != nil {
		return Blobs{}, err
	}
	if parentTree != EmptyTree {
		if out.ParentPatch, err = s.diff(ctx, EmptyTree, parentFull); err != nil {
			return Blobs{}, err
		}
	}
	return out, nil
}

// ReadFile reads one file of a tree.
func ReadFile(ctx context.Context, repo, tree, path string) ([]byte, error) {
	raw, err := gitRun(ctx, repo, nil, nil, "cat-file", "blob", tree+":"+path)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxManifestBytes {
		return nil, fmt.Errorf("pkgrelease: %s exceeds %d bytes", path, maxManifestBytes)
	}
	return raw, nil
}

// CheckManifest validates the package's choir-package.json bytes for a
// release of package name: the manifest parses, names the package, and
// declares no kinds before S12 (package_kinds_need_platform). It returns
// the record's manifest summary and the manifest version.
func CheckManifest(name string, raw []byte) (Manifest, string, error) {
	// Kinds first: a first-party package's kinds are generated into
	// internal/apppackage/firstparty_gen.go, outside the package, so a
	// release cannot carry them until runtime install (S12).
	var probe struct {
		Kinds []json.RawMessage `json:"kinds"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil && len(probe.Kinds) > 0 {
		return Manifest{}, "", refuse(ReasonKindsNeedPlatform, "the manifest declares %d kinds; a package's kinds need the platform until runtime install (S12)", len(probe.Kinds))
	}
	manifest, err := apppackage.ParseManifest(raw)
	if err != nil {
		return Manifest{}, "", refuse(ReasonManifestInvalid, "%v", err)
	}
	if manifest.Name != name {
		return Manifest{}, "", refuse(ReasonManifestInvalid, "the manifest names package %q, the release %q", manifest.Name, name)
	}
	return Manifest{
		SHA256: computerevent.DigestBytes(raw), ABI: manifest.App.ABI,
		KindsCount: len(manifest.Kinds), OpensCount: len(manifest.Opens),
	}, manifest.Version, nil
}

// AddedSecrets runs the release's refusal-grade secret scan over the lines
// a patch adds (the scan of internal/agentcore's sourcePatchAddedSecrets,
// here so corpusd runs it on the bytes it stores).
func AddedSecrets(patch []byte) []string {
	seen := map[string]bool{}
	var kinds []string
	scanner := bufio.NewScanner(bytes.NewReader(patch))
	scanner.Buffer(make([]byte, 64<<10), MaxBlobBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, []byte("+")) || bytes.HasPrefix(line, []byte("+++ ")) {
			continue
		}
		for _, kind := range computerevent.DetectRefusalSecrets(line[1:]) {
			if !seen[kind] {
				seen[kind] = true
				kinds = append(kinds, kind)
			}
		}
	}
	if scanner.Err() != nil && len(kinds) == 0 {
		kinds = append(kinds, "unscannable_line")
	}
	return kinds
}

// AppliedRef is the projection commit sourceproj left for one applied
// operation (refs/choir/applied/<applied event digest>), read from its
// trailers: the operation it applied and the stack base its patch was
// applied to.
type AppliedRef struct {
	Commit    string
	StackBase string
}

// AppliedRefs reads every applied ref in repo, by operation id.
func AppliedRefs(ctx context.Context, repo string) (map[string]AppliedRef, error) {
	out, err := gitLine(ctx, repo, nil, nil, "for-each-ref", "--format=%(objectname)", sourceproj.RefAppliedPrefix)
	if err != nil {
		return nil, err
	}
	refs := map[string]AppliedRef{}
	for _, commit := range strings.Fields(out) {
		if !gitObjectID.MatchString(commit) {
			continue
		}
		message, err := gitLine(ctx, repo, nil, nil, "log", "-1", "--format=%B", commit)
		if err != nil {
			continue
		}
		var operationID, stackBase string
		for _, line := range strings.Split(message, "\n") {
			if v, ok := strings.CutPrefix(line, "Choir-Operation: "); ok {
				operationID = strings.TrimSpace(v)
			}
			if v, ok := strings.CutPrefix(line, sourceproj.TrailerStackBase+": "); ok {
				stackBase = strings.TrimSpace(v)
			}
		}
		if operationID != "" && gitObjectID.MatchString(stackBase) {
			refs[operationID] = AppliedRef{Commit: commit, StackBase: stackBase}
		}
	}
	return refs, nil
}

// DraftInput is what the publisher's draft reads: the operation's bundle
// (its base and patch) and the effective source just before it.
type DraftInput struct {
	Repo string
	Name string
	// Base and Patch are the bundle's: under U1 the patch is cumulative from
	// the stack base.
	Base  string
	Patch []byte
	// Before is the commit whose tree is the source just before the
	// operation applied (the previous applied operation's projection commit,
	// or Base), and BeforeLabel names it in a refusal.
	Before      string
	BeforeLabel string
	// HeadReleaseID and HeadTree are the publisher's track head, when the
	// track has one (corpusd's answer, through the proxy).
	HeadReleaseID string
	HeadTree      string
}

// DraftSource is the package part of a release the publisher drafts.
type DraftSource struct {
	Root            string
	Paths           []string
	Tree            string
	Parent          Parent
	Blobs           Blobs
	Manifest        Manifest
	ManifestVersion string
}

// DraftPackage runs the publisher-side checks of design 3.2 and builds the
// release's package part, in one scratch over the guest repository:
//
//  1. operation confinement: the paths the operation changed (the source
//     before it against the source after it) all lie under the package
//     root, or package_change_not_confined names the others;
//  2. own change only: the package subtree before the operation is the
//     base's or the track head's, or package_parent_unpublished.
//
// These are the publisher's own checks: the host holds no copy of this
// repository, so it cannot redo them. What it can check, it does again on
// the bytes it stores (VerifyHost).
func DraftPackage(ctx context.Context, in DraftInput) (DraftSource, error) {
	root := Root(in.Name)
	s, err := newScratch(ctx, in.Repo)
	if err != nil {
		return DraftSource{}, err
	}
	defer s.close()
	after, err := s.apply(ctx, in.Base, in.Patch)
	if err != nil {
		return DraftSource{}, fmt.Errorf("pkgrelease: the operation's patch does not apply to its base %s: %w", in.Base, err)
	}
	before, err := s.git(ctx, nil, nil, "rev-parse", "--verify", "-q", in.Before+"^{tree}")
	if err != nil {
		return DraftSource{}, fmt.Errorf("pkgrelease: the source before the operation (%s) is unreadable: %w", in.BeforeLabel, err)
	}
	out, err := gitRun(ctx, s.dir, s.env, nil, "diff-tree", "-r", "-z", "--name-only", "--no-renames", before, after)
	if err != nil {
		return DraftSource{}, err
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return DraftSource{}, refuse(ReasonUnchanged, "the operation changed nothing against %s", in.BeforeLabel)
	}
	if outside := OutsideRoot(paths, root); len(outside) > 0 {
		return DraftSource{}, refuse(ReasonChangeNotConfined,
			"the operation changed paths outside %s: %s. A change outside a package is never adopted between computers; propose it for the platform with `choir platform publish`, or publish the package part alone if it stands alone", root, listSome(outside))
	}
	tree, present, err := subtreeIn(ctx, s.dir, s.env, after, root)
	if err != nil {
		return DraftSource{}, err
	}
	if !present {
		return DraftSource{}, refuse(ReasonInvalid, "the operation deletes %s; a release is a package's tree", root)
	}
	subtreeOr := func(treeish string) (string, error) {
		t, ok, err := subtreeIn(ctx, s.dir, s.env, treeish, root)
		if err != nil || !ok {
			return EmptyTree, err
		}
		return t, nil
	}
	parentTree, err := subtreeOr(before)
	if err != nil {
		return DraftSource{}, err
	}
	baseTree, err := subtreeOr(in.Base)
	if err != nil {
		return DraftSource{}, err
	}
	var parent Parent
	switch {
	case in.HeadReleaseID != "" && parentTree == in.HeadTree:
		parent = Parent{Kind: ParentRelease, ReleaseID: in.HeadReleaseID, Tree: parentTree}
	case parentTree == baseTree:
		// On the base. If the track already has a head, corpusd refuses
		// track_moved: a track has one linear history.
		parent = Parent{Kind: ParentBase, Commit: in.Base, Tree: baseTree}
	default:
		return DraftSource{}, refuse(ReasonParentUnpublished,
			"%s before this operation is %s (from %s), which is neither the base's %s nor the track head's: it carries an earlier, unpublished change. Publish that change's package release first, then this one on it", root, parentTree, in.BeforeLabel, baseTree)
	}
	full, err := s.prefixed(ctx, root, tree)
	if err != nil {
		return DraftSource{}, err
	}
	parentFull, err := s.prefixed(ctx, root, parent.Tree)
	if err != nil {
		return DraftSource{}, err
	}
	var blobs Blobs
	if blobs.TreePatch, err = s.diff(ctx, EmptyTree, full); err != nil {
		return DraftSource{}, err
	}
	if blobs.ReviewPatch, err = s.diff(ctx, parentFull, full); err != nil {
		return DraftSource{}, err
	}
	if parent.Kind == ParentBase && parent.Tree != EmptyTree {
		if blobs.ParentPatch, err = s.diff(ctx, EmptyTree, parentFull); err != nil {
			return DraftSource{}, err
		}
	}
	for _, b := range []struct {
		name string
		raw  []byte
	}{{"tree_patch", blobs.TreePatch}, {"review_patch", blobs.ReviewPatch}, {"parent_patch", blobs.ParentPatch}} {
		if len(b.raw) > MaxBlobBytes {
			return DraftSource{}, refuse(ReasonBlobTooLarge, "%s is %d bytes; the cap is %d (2 MiB)", b.name, len(b.raw), MaxBlobBytes)
		}
	}
	if kinds := AddedSecrets(blobs.TreePatch); len(kinds) > 0 {
		return DraftSource{}, refuse(ReasonSecretFound, "the package adds secret content (%s)", strings.Join(kinds, ", "))
	}
	raw, err := gitRun(ctx, s.dir, s.env, nil, "cat-file", "blob", full+":"+root+"/choir-package.json")
	if err != nil || len(raw) > maxManifestBytes {
		return DraftSource{}, refuse(ReasonManifestInvalid, "%s/choir-package.json is missing or unreadable", root)
	}
	manifest, version, err := CheckManifest(in.Name, raw)
	if err != nil {
		return DraftSource{}, err
	}
	return DraftSource{Root: root, Paths: paths, Tree: tree, Parent: parent, Blobs: blobs, Manifest: manifest, ManifestVersion: version}, nil
}
