package pkgrelease

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

// manifestDescription is the manifest's display description as one line of
// at most 200 characters (whitespace collapsed). The host derives it from the
// manifest it verifies and stores it beside the release as listing-side
// metadata: the record carries none, so the record format is unchanged.
func manifestDescription(manifest apppackage.Manifest) string {
	// One line of at most 200 characters: the record's Validate refuses
	// more, and a manifest may hold a longer or multi-line description.
	line := strings.Join(strings.Fields(manifest.Display.Description), " ")
	if r := []rune(line); len(r) > 200 {
		line = strings.TrimSpace(string(r[:200]))
	}
	return line
}

// treeSource is the package's files in a git tree in a scratch repository:
// root is the package directory inside full. Every path is listed from the
// tree, a git tree path cannot leave the tree, and a symlink blob (mode 120000)
// is refused by the confinement check before this runs.
type treeSource struct {
	ctx        context.Context
	dir        string
	env        []string
	full, root string
}

func treeFiles(ctx context.Context, dir string, env []string, full, root string) apppackage.Source {
	return treeSource{ctx: ctx, dir: dir, env: env, full: full, root: root}
}

func (t treeSource) List() ([]string, error) {
	out, err := gitRun(t.ctx, t.dir, t.env, nil, "ls-tree", "-r", "-z", t.full+":"+t.root)
	if err != nil {
		return nil, fmt.Errorf("the release holds no %s", t.root)
	}
	// Only regular files: a symlink or a submodule is refused by the
	// confinement check, and is never read as source.
	var paths []string
	for _, line := range strings.Split(string(out), "\x00") {
		meta, name, ok := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if ok && len(fields) == 3 && fields[1] == "blob" && (fields[0] == "100644" || fields[0] == "100755") {
			paths = append(paths, name)
		}
	}
	return paths, nil
}

func (t treeSource) Read(path string) ([]byte, error) {
	raw, err := gitRun(t.ctx, t.dir, t.env, nil, "cat-file", "blob", t.full+":"+t.root+"/"+path)
	if err != nil {
		return nil, fmt.Errorf("the release holds no %s", path)
	}
	if len(raw) > apppackage.MaxSourceFileBytes {
		return nil, fmt.Errorf("%s is over %d bytes", path, apppackage.MaxSourceFileBytes)
	}
	return raw, nil
}

// CheckManifest validates the package's choir-package.json bytes for a
// release of package name published through the Library (origin published):
// the manifest parses as choir.package/v2, names the package, refuses what a
// publisher may not state or ask for (the platform's own facts, a privileged
// grant, a desk before the Phase 6 sandbox), and declares no kinds before S12
// (package_kinds_need_platform). files reads the package's source files for the
// behaviour-surface hash and the desk import gate. It returns the record's
// manifest summary and the manifest version.
//
// It runs the tiered review's install-time part (L3, review.go): T2 refused,
// the T0 data review; the listing's T1 contract requirement runs at publish.
func CheckManifest(name string, raw []byte, files apppackage.Source) (Manifest, string, error) {
	summary, version, _, err := checkManifest(name, raw, files)
	return summary, version, err
}

// checkManifest is CheckManifest that also returns the parsed manifest, from
// which the host derives what the Library shows (VerifyHostFacts).
func checkManifest(name string, raw []byte, files apppackage.Source) (Manifest, string, apppackage.Manifest, error) {
	return checkManifestOn(name, raw, files, apppackage.CurrentPlatform())
}

// checkManifestOn is checkManifest on a stated platform.
func checkManifestOn(name string, raw []byte, files apppackage.Source, platform apppackage.Platform) (Manifest, string, apppackage.Manifest, error) {
	// Kinds first: a first-party package's kinds are generated into
	// internal/apppackage/firstparty_gen.go, outside the package, so a
	// release cannot carry them until runtime install (S12).
	var probe struct {
		Kinds []json.RawMessage `json:"kinds"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil && len(probe.Kinds) > 0 {
		return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonKindsNeedPlatform, "the manifest declares %d kinds; a package's kinds need the platform until runtime install (S12)", len(probe.Kinds))
	}
	// The tiered review (L3, review.go): a T2 release is refused with its
	// own reason before the manifest checks would name its privileged grant.
	if err := reviewPlatform(raw); err != nil {
		return Manifest{}, "", apppackage.Manifest{}, err
	}
	manifest, err := apppackage.ParseManifestOn(raw, apppackage.OriginPublished, platform)
	if err != nil {
		switch {
		case errors.Is(err, apppackage.ErrPublisherDeclared):
			return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonPublisherDeclared, "%v", err)
		case errors.Is(err, apppackage.ErrPrivilegedGrant):
			return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonPrivilegedGrant, "%v", err)
		case errors.Is(err, apppackage.ErrDeskNeedsSandbox):
			return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonDeskNeedsSandbox, "%v", err)
		}
		return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonManifestInvalid, "%v", err)
	}
	if manifest.Name != name {
		return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonManifestInvalid, "the manifest names package %q, the release %q", manifest.Name, name)
	}
	surface, err := manifest.BehaviourSurface(files)
	if err != nil {
		if errors.Is(err, apppackage.ErrGoModule) {
			return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonGoModuleRefused, "%v", err)
		}
		return Manifest{}, "", apppackage.Manifest{}, refuse(ReasonManifestInvalid, "%v", err)
	}
	if err := reviewData(manifest, files); err != nil {
		return Manifest{}, "", apppackage.Manifest{}, err
	}
	summary := Manifest{
		SHA256: computerevent.DigestBytes(raw), Schema: manifest.Schema,
		KindsCount: len(manifest.Kinds), OpensCount: len(manifest.Contributes.Opens), DesksCount: len(manifest.Contributes.Desks),
		Class: string(manifest.Class()), Level: string(manifest.Level()), BehaviourSurface: surface,
	}
	if manifest.App != nil {
		summary.ABI = manifest.App.ABI
		summary.RequiresSDK = manifest.App.RequiresSDK
	}
	return summary, manifest.Version, manifest, nil
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

// DraftInput is what the publisher's draft reads: one package operation's
// frozen bundle (its base and patch). Forced modularity
// (docs/design/forced-modularity-build-2026-11-04.md, sections 1 and 2): the
// source stack carries no package path, so the package before the operation
// is the base's, and the operation's patch changes this package's root only.
type DraftInput struct {
	Repo string
	Name string
	// Base and Patch are the bundle's: the patch is the package operation's
	// own change on the commit the capsule was cast from.
	Base  string
	Patch []byte
	// PlatformBase is the platform commit the release names as built_on and
	// as a base parent: the stack base when Base is a projection commit
	// (whose package subtree is the stack base's, since the stack holds no
	// package path), Base otherwise. Empty means Base.
	PlatformBase string
	// HeadReleaseID and HeadTree are the publisher's track head, when the
	// track has one (corpusd's answer, through the proxy).
	HeadReleaseID string
	HeadTree      string
	// SeedPatch is the head release's tree patch, for a host that holds no
	// seed commit: the draft's base is then Base with the package replaced by
	// the head's tree, the commit a package operation's capsule was seeded
	// with (sourceproj.SeedCommit). Empty means Base as it is.
	SeedPatch []byte
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
//  1. confinement: every path the operation changed lies under the package
//     root, or package_change_not_confined names the others (a package
//     operation is one package; a sibling package's path is outside too);
//  2. own change only: the package before the operation is the base's. Its
//     parent is the track head when the head's tree is that package, the
//     base when the track has no head, and otherwise the draft refuses
//     package_parent_unpublished: the operation was built on another version
//     than the track's head.
//
// Only the package subtree travels, and the parent is one the host verifies
// (the base's subtree, or a listed release), so the track keeps one linear
// history. What the host can check, it does again on the bytes it stores
// (VerifyHost).
func DraftPackage(ctx context.Context, in DraftInput) (DraftSource, error) {
	root := Root(in.Name)
	platformBase := strings.TrimSpace(in.PlatformBase)
	if platformBase == "" {
		platformBase = in.Base
	}
	s, err := newScratch(ctx, in.Repo)
	if err != nil {
		return DraftSource{}, err
	}
	defer s.close()
	if len(in.SeedPatch) > 0 {
		seeded, err := s.seeded(ctx, in.Base, root, in.SeedPatch)
		if err != nil {
			return DraftSource{}, refuse(ReasonParentUnverifiable, "the track head's tree does not seed base %s: %v", in.Base, err)
		}
		in.Base = seeded
	}
	after, err := s.apply(ctx, in.Base, in.Patch)
	if err != nil {
		return DraftSource{}, refuse(ReasonPatchInapplicable, "the operation's patch does not apply to its base %s: %v", in.Base, err)
	}
	opPaths, err := diffPaths(ctx, s, in.Base, after)
	if err != nil {
		return DraftSource{}, err
	}
	if len(opPaths) == 0 {
		return DraftSource{}, refuse(ReasonUnchanged, "the operation changed nothing against its base %s", in.Base)
	}
	if outside := OutsideRoot(opPaths, root); len(outside) > 0 {
		return DraftSource{}, refuse(ReasonChangeNotConfined, "%s", notConfinedDetail(in.Name, root, outside))
	}
	paths := opPaths
	tree, present, err := subtreeIn(ctx, s.dir, s.env, after, root)
	if err != nil {
		return DraftSource{}, err
	}
	if !present {
		return DraftSource{}, refuse(ReasonInvalid, "the operation deletes %s; a release is a package's tree", root)
	}
	parentTree, ok, err := subtreeIn(ctx, s.dir, s.env, in.Base, root)
	if err != nil {
		return DraftSource{}, err
	}
	if !ok {
		parentTree = EmptyTree
	}
	var parent Parent
	switch {
	case in.HeadReleaseID != "" && parentTree == in.HeadTree:
		parent = Parent{Kind: ParentRelease, ReleaseID: in.HeadReleaseID, Tree: parentTree}
	case in.HeadReleaseID == "":
		parent = Parent{Kind: ParentBase, Commit: platformBase, Tree: parentTree}
	default:
		return DraftSource{}, refuse(ReasonParentUnpublished,
			"%s before this operation is the base's %s, which is not the track head's %s: the operation was built on another version than the track's head, so it cannot go on it", root, parentTree, in.HeadTree)
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
	files := treeFiles(ctx, s.dir, s.env, full, root)
	manifest, version, parsed, err := checkManifest(in.Name, raw, files)
	if err != nil {
		return DraftSource{}, err
	}
	// The publisher sees the listing's T1 requirement before the host does.
	if err := reviewListing(parsed, files); err != nil {
		return DraftSource{}, err
	}
	return DraftSource{Root: root, Paths: paths, Tree: tree, Parent: parent, Blobs: blobs, Manifest: manifest, ManifestVersion: version}, nil
}

// diffPaths lists the paths that differ between two tree-ishes, sorted.
func diffPaths(ctx context.Context, s *scratch, from, to string) ([]string, error) {
	out, err := gitRun(ctx, s.dir, s.env, nil, "diff-tree", "-r", "-z", "--name-only", "--no-renames", from, to)
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

// notConfinedDetail is the package_change_not_confined prose: the outside
// paths that differ from the base, with the package's own tests (under
// frontend/tests/<name>/) named apart. Those stay refused: a release carries
// only the package subtree, so they would not ship, and the design does not
// count them as part of the package.
func notConfinedDetail(name, root string, outside []string) string {
	testsRoot := packageTestsRoot(name)
	var tests, other []string
	for _, p := range outside {
		if strings.HasPrefix(p, testsRoot+"/") {
			tests = append(tests, p)
		} else {
			other = append(other, p)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "the operation changed paths outside %s that differ from the base", root)
	if len(other) > 0 {
		fmt.Fprintf(&b, ": %s. A change outside a package is never adopted between computers; propose it for the platform with `choir platform publish`, or restore those paths to the base and publish the package part alone if it stands alone", listSome(other))
	}
	if len(tests) > 0 {
		sep := ": "
		if len(other) > 0 {
			sep = ". Also, "
		}
		fmt.Fprintf(&b, "%sthe package's own test files %s are outside the package and a release does not carry them; restore them to the base to publish", sep, listSome(tests))
	}
	return b.String()
}

// packageTestsRoot is where a package's own end-to-end tests live, outside
// its subtree.
func packageTestsRoot(name string) string { return "frontend/tests/" + name }

// TreeSource is the package's files at root in the full-path tree of repo: the
// source the behaviour surface covers. Publishers and tests build the record's
// manifest summary from it.
func TreeSource(ctx context.Context, repo, fullTree, root string) apppackage.Source {
	return treeFiles(ctx, repo, nil, fullTree, root)
}
