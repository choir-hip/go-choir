package pkgrelease

import (
	"context"
	"strings"
	"testing"
)

// The draft of one package operation (forced modularity,
// docs/design/forced-modularity-build-2026-11-04.md sections 1 and 2): the
// source stack carries no package path, so the package before the operation
// is the base's, and the operation changes one package root only. Failure
// modes pinned:
//
//	P1  a path outside the root ships, or is dropped silently (the release
//	    would not be the change that was reviewed);
//	P2  a sibling package's path is let through (one operation, one package);
//	P3  the package's own tests are refused without saying they can be
//	    restored;
//	P4  a loose file directly under the packages directory reads as a
//	    package;
//	P5  an operation built on the base while the track has a head of
//	    another tree is put on that head (the head's change would be lost);
//	P6  an operation on a projection commit names that commit as its
//	    parent, which the host cannot verify.

const testPackageTests = "frontend/tests/reader"

func (f *fixture) draft(after string) (DraftSource, error) {
	f.t.Helper()
	return DraftPackage(context.Background(), DraftInput{Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(f.base, after)})
}

func TestDraftCarriesThePackagePathsOnly(t *testing.T) {
	f := newFixture(t)
	source, err := f.draft(f.tree(changed("<p>reader v2</p>\n")))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(source.Paths, ","); got != testRoot+"/App.svelte" || source.Parent.Kind != ParentBase || source.Parent.Commit != f.base {
		t.Fatalf("paths %s parent %+v", got, source.Parent)
	}
}

func TestDraftRefusesAnOutsidePath(t *testing.T) { // P1
	f := newFixture(t)
	_, err := f.draft(f.tree(map[string]entry{
		testRoot + "/App.svelte": {"100644", "<p>reader v2</p>\n"},
		"frontend/src/app.css":   {"100644", "body { color: red }\n"},
	}))
	wantReason(t, err, ReasonChangeNotConfined)
	if !strings.Contains(err.Error(), "frontend/src/app.css") {
		t.Fatalf("refusal %q does not name the outside path", err)
	}
}

func TestDraftRefusesASiblingPackage(t *testing.T) { // P2
	f := newFixture(t)
	_, err := f.draft(f.tree(map[string]entry{
		testRoot + "/App.svelte":                   {"100644", "<p>reader v2</p>\n"},
		"frontend/src/packages/sibling/App.svelte": {"100644", "<p>sibling</p>\n"},
	}))
	wantReason(t, err, ReasonChangeNotConfined)
	if !strings.Contains(err.Error(), "packages/sibling") {
		t.Fatalf("refusal %q does not name the sibling", err)
	}
}

func TestDraftRefusesThePackagesOwnTestsAndSaysTheyCanBeRestored(t *testing.T) { // P3
	f := newFixture(t)
	_, err := f.draft(f.tree(map[string]entry{
		testRoot + "/App.svelte":             {"100644", "<p>reader v2</p>\n"},
		testPackageTests + "/reader.spec.js": {"100644", "test()\n"},
	}))
	wantReason(t, err, ReasonChangeNotConfined)
	msg := err.Error()
	if !strings.Contains(msg, testPackageTests+"/reader.spec.js") || !strings.Contains(msg, "test files") || !strings.Contains(msg, "restore") {
		t.Fatalf("refusal %q does not name the package's tests as test files that can be restored", msg)
	}
}

func TestDraftTreatsALooseFileUnderPackagesAsOutside(t *testing.T) { // P4
	f := newFixture(t)
	_, err := f.draft(f.tree(map[string]entry{
		testRoot + "/App.svelte":           {"100644", "<p>reader v2</p>\n"},
		"frontend/src/packages/readerfile": {"100644", "x\n"},
	}))
	wantReason(t, err, ReasonChangeNotConfined)
}

func TestDraftGoesOnTheTrackHeadOnlyWhenItIsTheBasesPackage(t *testing.T) { // P5
	f := newFixture(t)
	after := f.tree(changed("<p>reader v2</p>\n"))
	in := DraftInput{Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(f.base, after),
		HeadReleaseID: strings.Repeat("f", 64), HeadTree: f.baseSubtree()}
	source, err := DraftPackage(context.Background(), in)
	if err != nil || source.Parent.Kind != ParentRelease || source.Parent.ReleaseID != in.HeadReleaseID {
		t.Fatalf("%+v %v", source.Parent, err)
	}
	in.HeadTree = f.subtree(f.tree(changed("<p>someone else's v2</p>\n")))
	_, err = DraftPackage(context.Background(), in)
	wantReason(t, err, ReasonParentUnpublished)
}

func TestDraftNamesThePlatformBaseAsTheParent(t *testing.T) { // P6
	f := newFixture(t)
	projection := f.tree(map[string]entry{"internal/x.go": {"100644", "package x\n"}})
	after := f.tree(map[string]entry{
		"internal/x.go":          {"100644", "package x\n"},
		testRoot + "/App.svelte": {"100644", "<p>reader v2</p>\n"},
	})
	source, err := DraftPackage(context.Background(), DraftInput{Repo: f.repo, Name: "reader", Base: projection, PlatformBase: f.base, Patch: f.fullDiff(projection, after)})
	if err != nil || source.Parent.Kind != ParentBase || source.Parent.Commit != f.base || source.Parent.Tree != f.baseSubtree() {
		t.Fatalf("%+v %v", source.Parent, err)
	}
}
