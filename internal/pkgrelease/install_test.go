package pkgrelease

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

// Install planning (docs/design/package-install-2026-11-04.md), failure
// modes written before the code:
//
//	F3  blob bytes that are not the record's are planned;
//	F4  content outside the root is planned;
//	F5  a secret in the tree_patch is planned;
//	I1  the install patch changes a path outside the package root;
//	I2  the patch, applied to the computer's commit, does not make the
//	    release's tree at the root (an upgrade, a new package);
//	I3  an already installed release yields a non-empty patch;
//	I4  an uninstall leaves anything at the root, or touches anything else.

// applyAt applies patch to commit's tree in a throwaway index and returns
// the resulting tree.
func (f *fixture) applyAt(commit string, patch []byte) string {
	f.t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(f.t.TempDir(), "index")}
	gitT(f.t, f.repo, env, "", "read-tree", commit)
	gitT(f.t, f.repo, env, string(patch), "apply", "--cached", "-")
	return gitT(f.t, f.repo, env, "", "write-tree")
}

func TestPlanInstallUpgradesOnlyThePackageRoot(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>reader v2</p>\n")))
	plan, err := PlanInstall(context.Background(), f.repo, f.base, record, blobs.TreePatch, false)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.LocalPresent || plan.Local != f.baseSubtree() || len(plan.Patch) == 0 {
		t.Fatalf("plan = %+v", plan)
	}
	after := f.applyAt(f.base, plan.Patch)
	if got := f.subtree(after); got != record.Source.Tree {
		t.Fatalf("I2: root after install = %s, want %s", got, record.Source.Tree)
	}
	if paths, err := DiffPaths(context.Background(), f.repo, f.base, after); err != nil || len(OutsideRoot(paths, testRoot)) != 0 {
		t.Fatalf("I1: paths %v (%v)", paths, err)
	}

	// I3: once installed, nothing to do.
	commit := gitT(t, f.repo, nil, "", "commit-tree", after, "-p", f.base, "-m", "installed")
	again, err := PlanInstall(context.Background(), f.repo, commit, record, blobs.TreePatch, false)
	if err != nil || len(again.Patch) != 0 || again.Local != record.Source.Tree {
		t.Fatalf("I3: plan = %+v err %v", again, err)
	}

	// I4: uninstall removes the root and nothing else.
	un, err := PlanInstall(context.Background(), f.repo, commit, record, blobs.TreePatch, true)
	if err != nil {
		t.Fatal(err)
	}
	removed := f.applyAt(commit, un.Patch)
	if _, present, _ := Subtree(context.Background(), f.repo, removed, testRoot); present {
		t.Fatal("I4: the root survives the uninstall")
	}
	if paths, _ := DiffPaths(context.Background(), f.repo, commit, removed); len(OutsideRoot(paths, testRoot)) != 0 {
		t.Fatalf("I4: uninstall touched %v", paths)
	}
}

func TestPlanInstallAddsANewPackage(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>reader v2</p>\n")))
	// A commit with no package at the root: the computer never had it.
	bare := f.tree(map[string]entry{testRoot + "/App.svelte": {"-", ""}, testRoot + "/choir-package.json": {"-", ""}})
	commit := gitT(t, f.repo, nil, "", "commit-tree", bare, "-m", "bare")
	plan, err := PlanInstall(context.Background(), f.repo, commit, record, blobs.TreePatch, false)
	if err != nil || plan.LocalPresent {
		t.Fatalf("plan = %+v err %v", plan, err)
	}
	if got := f.subtree(f.applyAt(commit, plan.Patch)); got != record.Source.Tree {
		t.Fatalf("I2: root = %s, want %s", got, record.Source.Tree)
	}
}

func TestPlanInstallRefusesContentTheRecordDoesNotHold(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>reader v2</p>\n")))

	// F3: other bytes.
	_, err := PlanInstall(context.Background(), f.repo, f.base, record, append([]byte{}, blobs.ReviewPatch[:len(blobs.ReviewPatch)-1]...), false)
	wantReason(t, err, ReasonBlobMismatch)

	// F4: a tree_patch reaching outside the root, with a record that names it.
	outside := f.tree(map[string]entry{testRoot + "/App.svelte": {"100644", "<p>v2</p>\n"}, "frontend/src/app.css": {"100644", "body{color:red}\n"}})
	forged := f.fullDiff(EmptyTree, outside)
	bad := record
	bad.Source.TreePatch = *blobOf(forged)
	_, err = PlanInstall(context.Background(), f.repo, f.base, bad, forged, false)
	wantReason(t, err, ReasonContentNotConfined)
	wantReason(t, VerifyContent(context.Background(), bad, forged), ReasonContentNotConfined)

	// F5: a secret.
	srec, sblobs := f.release(f.tree(changed("GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n")))
	_, err = PlanInstall(context.Background(), f.repo, f.base, srec, sblobs.TreePatch, false)
	wantReason(t, err, ReasonSecretFound)

	// The good release verifies.
	if err := VerifyContent(context.Background(), record, blobs.TreePatch); err != nil {
		t.Fatal(err)
	}
}
