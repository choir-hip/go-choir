package agentcore

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// A package operation that updates an installed package (docs/design/
// forced-modularity-build-2026-11-04.md, Deferred item 2). An installed
// package keeps no source on the computer, so a capsule cast from the
// projection's HEAD holds the booted base's copy of it, and every update
// after the first release was refused package_parent_unpublished. Failure
// modes pinned:
//
//	S1  the update drafts on the base while the track head is the installed
//	    release (the defect: package_parent_unpublished);
//	S2  the seed commit is not the installed tree: the draft's parent is the
//	    head only when the seed's package is the head's tree, and the release
//	    still names the platform base as built_on, not the seed;
//	S3  the seed touches a path outside the package root, so a platform
//	    operation cast beside it would carry package paths;
//	S4  an install keeps no tree patch, so nothing can seed the next update;
//	S5  an install from before the patch was kept crashes the Cast, or
//	    silently falls back to the base, instead of refusing with the missing
//	    seed named;
//	S6  a package that is not installed is seeded anyway (a new package is
//	    cast from HEAD).

func installedHead(t *testing.T, w *draftWorld) (headID, headTree string, treePatch []byte) {
	t.Helper()
	w.updateOperation("op-v2", map[string]string{draftRoot + "/notes.txt": "v2\n"})
	rec := w.draft("op-v2", nil)
	record, releaseID, blobs := decodeDraft(t, rec)
	return releaseID, record.Source.Tree, blobs.TreePatch
}

func installRecord(t *testing.T, w *draftWorld, releaseID string, treePatch []byte) {
	t.Helper()
	if err := w.rt.store.PutPackageInstall(context.Background(), store.PackageInstall{
		OwnerID: "owner-a", ComputerID: draftComputer, InstallID: "install-reader", ReleaseID: releaseID,
		Kind: installKindPackage, Package: "reader", Action: packageActionInstall, Attempt: 1, Mode: store.PackageInstallModeRuntime,
		PatchSHA256: computerevent.DigestBytes(treePatch),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPackageUpdateSeedsFromTheInstalledHead(t *testing.T) { // S1, S2, S3
	w := newDraftWorld(t)
	w.rt.packageModules = pkgruntime.Store{Dir: t.TempDir()}
	headID, headTree, treePatch := installedHead(t, w)
	if _, err := w.rt.packageModules.PutPatch(treePatch); err != nil {
		t.Fatal(err)
	}
	installRecord(t, w, headID, treePatch)
	head := url.Values{"head_release_id": {headID}, "head_tree": {headTree}}

	// S1: a capsule cast from the base (what main does) cannot update the head.
	w.updateOperation("op-base", map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	wantDraftReason(t, w.draft("op-base", head), http.StatusConflict, pkgrelease.ReasonParentUnpublished)

	// S2: a capsule cast from the seed drafts as an update of the head.
	ctx := context.Background()
	seed, err := w.rt.packageSeedCommit(ctx, "owner-a", draftComputer, "reader", w.base)
	if err != nil || seed == "" || seed == w.base {
		t.Fatalf("seed = %q, %v", seed, err)
	}
	if got := releaseGit(t, w.repo, "rev-parse", seed+":"+draftRoot); got != headTree {
		t.Fatalf("the seed holds %s at the package root, want the installed tree %s", got, headTree)
	}
	// S3: nothing outside the package root differs from HEAD.
	if outside := pkgrelease.OutsideRoot(strings.Fields(releaseGit(t, w.repo, "diff-tree", "-r", "--name-only", "--no-renames", w.base, seed)), draftRoot); len(outside) > 0 {
		t.Fatalf("the seed changes paths outside the package: %v", outside)
	}
	platformBase := w.base
	w.base = seed
	w.updateOperation("op-seeded", map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	record, _, _ := decodeDraft(t, w.draft("op-seeded", head))
	if record.Parent.Kind != pkgrelease.ParentRelease || record.Parent.ReleaseID != headID {
		t.Fatalf("parent = %+v, want the installed head %s", record.Parent, headID)
	}
	if record.Source.BuiltOn != platformBase {
		t.Fatalf("built_on = %s, want the platform base %s, not the seed", record.Source.BuiltOn, platformBase)
	}
}

func TestInstallKeepsItsTreePatchAndAPreChangeInstallRefusesNamingTheSeed(t *testing.T) { // S4, S5, S6
	e := newInstallEnv(t)
	e.rt.releaseSourceDir = e.repo
	ctx := context.Background()
	if seed, err := e.rt.packageSeedCommit(ctx, e.owner, e.computer, installName, e.base); seed != "" || err != nil { // S6
		t.Fatalf("an uninstalled package is seeded: %q %v", seed, err)
	}
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusCreated {
		t.Fatalf("install = %d %s", code, reason)
	}
	sha := computerevent.DigestBytes(tree)
	if kept, err := e.rt.packageModules.ReadPatch(sha); err != nil || string(kept) != string(tree) { // S4
		t.Fatalf("the install did not keep its tree patch: %v", err)
	}
	if seed, err := e.rt.packageSeedCommit(ctx, e.owner, e.computer, installName, e.base); err != nil || seed == "" {
		t.Fatalf("seed = %q, %v", seed, err)
	}
	// S5: the patch is gone (an install from before it was kept).
	e.rt.packageModules = pkgruntime.Store{Dir: t.TempDir()}
	_, err := e.rt.packageSeedCommit(ctx, e.owner, e.computer, installName, e.base)
	if err == nil || !strings.Contains(err.Error(), reasonPackageSeedMissing) || !strings.Contains(err.Error(), sha) {
		t.Fatalf("a missing seed must refuse naming it, got %v", err)
	}
}

// updateOperation is an applied operation whose bundle holds the
// before/after pair of one view that an update's release requires.
func (w *draftWorld) updateOperation(id string, files map[string]string) {
	w.t.Helper()
	w.mediaOperation(id, files, map[string][]byte{
		"01-before-home.png": mediaPNG(w.t, 64, 40, 3, false),
		"01-after-home.png":  mediaPNG(w.t, 64, 40, 5, false),
	}, nil)
}
