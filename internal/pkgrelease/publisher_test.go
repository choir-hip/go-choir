package pkgrelease

import (
	"context"
	"strings"
	"testing"
)

// Publisher-side confinement compares the result, not the history (problem
// 23): an outside path the operation changed counts only when its content in
// the applied tree differs from the stack's platform base. (An outside path
// the operation left alone never counts: fix 11, pinned in agentcore's
// TestPackageDraftCarriesAnEarlierUnpublishedRoundOfTheStack.) Failure modes
// pinned:
//
//	C1  an outside path an earlier round changed and this round restored to
//	    the base still refuses, so no round of the stack can publish;
//	C2  the refusal names an outside path that equals the base again;
//	C3  an outside path that really differs from the base is let through;
//	C4  the package's own tests under frontend/tests/<name>/ are let
//	    through, or refused without saying they are tests that can be
//	    restored.

const testPackageTests = "frontend/tests/reader"

func (f *fixture) draft(before string, stack []string, after string) (DraftSource, error) {
	f.t.Helper()
	return DraftPackage(context.Background(), DraftInput{
		Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(f.base, after),
		Before: before, BeforeLabel: "the previous round", Stack: stack,
	})
}

func TestDraftAllowsAnOutsidePathRestoredToTheBase(t *testing.T) {
	f := newFixture(t)
	round1 := f.tree(map[string]entry{
		testRoot + "/App.svelte":             {"100644", "<p>reader v2</p>\n"},
		"frontend/src/app.css":               {"100644", "body { color: red }\n"},
		testPackageTests + "/reader.spec.js": {"100644", "test()\n"},
	})
	// Round 2 changes the package again and restores every outside path.
	round2 := f.tree(changed("<p>reader v3</p>\n"))
	source, err := f.draft(round1, []string{round1}, round2)
	if err != nil {
		t.Fatalf("a stack whose outside paths equal the base again is publishable: %v", err)
	}
	if got := strings.Join(source.Paths, ","); got != testRoot+"/App.svelte" {
		t.Fatalf("paths = %s, want only the package path", got)
	}
	if source.Parent.Kind != ParentBase {
		t.Fatalf("parent = %+v, want the base", source.Parent)
	}
}

func TestDraftAllowsARoundThatOnlyRestoresAnOutsidePath(t *testing.T) {
	f := newFixture(t)
	round1 := f.tree(map[string]entry{
		testRoot + "/App.svelte": {"100644", "<p>reader v2</p>\n"},
		"frontend/src/app.css":   {"100644", "body { color: red }\n"},
	})
	// Round 2 only puts app.css back: the release carries round 1's package
	// change.
	round2 := f.tree(changed("<p>reader v2</p>\n"))
	source, err := f.draft(round1, []string{round1}, round2)
	if err != nil {
		t.Fatalf("a round that restores the outside path makes the stack publishable: %v", err)
	}
	if got := strings.Join(source.Paths, ","); got != testRoot+"/App.svelte" {
		t.Fatalf("paths = %s, want the carried package path", got)
	}
}

func TestDraftRefusesOnlyTheOutsidePathsThatDifferFromTheBase(t *testing.T) {
	f := newFixture(t)
	round1 := f.tree(map[string]entry{
		testRoot + "/App.svelte":  {"100644", "<p>reader v2</p>\n"},
		"frontend/src/app.css":    {"100644", "body { color: red }\n"},
		"frontend/src/lib/sdk.ts": {"100644", "export {}\n"},
	})
	// Round 2 restores app.css and changes sdk.ts again, which the base
	// lacks.
	round2 := f.tree(map[string]entry{
		testRoot + "/App.svelte":  {"100644", "<p>reader v3</p>\n"},
		"frontend/src/lib/sdk.ts": {"100644", "export { a }\n"},
	})
	_, err := f.draft(round1, []string{round1}, round2)
	wantReason(t, err, ReasonChangeNotConfined)
	msg := err.Error()
	if !strings.Contains(msg, "frontend/src/lib/sdk.ts") {
		t.Fatalf("refusal %q does not name the path that differs", msg)
	}
	if strings.Contains(msg, "app.css") {
		t.Fatalf("refusal %q names a path that equals the base again", msg)
	}
}

func TestDraftRefusesThePackagesOwnTestsAndSaysTheyCanBeRestored(t *testing.T) {
	f := newFixture(t)
	after := f.tree(map[string]entry{
		testRoot + "/App.svelte":             {"100644", "<p>reader v2</p>\n"},
		testPackageTests + "/reader.spec.js": {"100644", "test()\n"},
	})
	_, err := f.draft(f.base, nil, after)
	wantReason(t, err, ReasonChangeNotConfined)
	msg := err.Error()
	if !strings.Contains(msg, testPackageTests+"/reader.spec.js") || !strings.Contains(msg, "test files") || !strings.Contains(msg, "restore") {
		t.Fatalf("refusal %q does not name the package's tests as test files that can be restored", msg)
	}
}

func TestDraftRefusesARestoreOnlyRoundWhosePackageIsItsParentsAsUnchanged(t *testing.T) {
	f := newFixture(t)
	// Round 1 changed the reader and app.css, and its reader is the track
	// head. Round 2 only puts app.css back: the package is the head's
	// already, so there is nothing to release.
	round1 := f.tree(map[string]entry{
		testRoot + "/App.svelte": {"100644", "<p>reader v2</p>\n"},
		"frontend/src/app.css":   {"100644", "body { color: red }\n"},
	})
	round2 := f.tree(changed("<p>reader v2</p>\n"))
	_, err := DraftPackage(context.Background(), DraftInput{
		Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(f.base, round2),
		Before: round1, BeforeLabel: "the previous round", Stack: []string{round1},
		HeadReleaseID: strings.Repeat("f", 64), HeadTree: f.subtree(round1),
	})
	wantReason(t, err, ReasonUnchanged)
}

// Problem 29: a sibling package's subtree is not outside. Each package is
// published from its own subtree and the release carries only that subtree,
// so a stack carrying two packages can publish both.
func TestDraftAllowsASiblingPackageSubtree(t *testing.T) {
	f := newFixture(t)
	after := f.tree(map[string]entry{
		testRoot + "/App.svelte":                    {"100644", "<p>reader v2</p>\n"},
		"frontend/src/packages/sibling/App.svelte":  {"100644", "<p>sibling</p>\n"},
		"frontend/src/packages/sibling/Icon.svelte": {"100644", "<i/>\n"},
	})
	source, err := f.draft(f.base, nil, after)
	if err != nil {
		t.Fatalf("a sibling package's subtree is not outside: %v", err)
	}
	if got := strings.Join(source.Paths, ","); got != testRoot+"/App.svelte" {
		t.Fatalf("paths = %s, want only this package's path", got)
	}
}

func TestDraftRefusesSharedLibEvenWithSiblingChanges(t *testing.T) {
	f := newFixture(t)
	after := f.tree(map[string]entry{
		testRoot + "/App.svelte":                   {"100644", "<p>reader v2</p>\n"},
		"frontend/src/packages/sibling/App.svelte": {"100644", "<p>sibling</p>\n"},
		"frontend/src/lib/app-sdk.ts":              {"100644", "export {}\n"},
		testPackageTests + "/reader.spec.js":       {"100644", "test()\n"},
	})
	_, err := f.draft(f.base, nil, after)
	wantReason(t, err, ReasonChangeNotConfined)
	msg := err.Error()
	if !strings.Contains(msg, "frontend/src/lib/app-sdk.ts") || !strings.Contains(msg, testPackageTests+"/reader.spec.js") {
		t.Fatalf("refusal %q does not name the shared lib and the own tests", msg)
	}
	if strings.Contains(msg, "packages/sibling") {
		t.Fatalf("refusal %q names a sibling package path", msg)
	}
}

func TestDraftTreatsAPackageNamePrefixAsOutside(t *testing.T) {
	f := newFixture(t)
	// "reader2" is a sibling, "readerfile" directly under packages is not a
	// package subtree.
	after := f.tree(map[string]entry{
		testRoot + "/App.svelte":                   {"100644", "<p>reader v2</p>\n"},
		"frontend/src/packages/readerfile":         {"100644", "x\n"},
		"frontend/src/packages/reader2/App.svelte": {"100644", "y\n"},
	})
	_, err := f.draft(f.base, nil, after)
	wantReason(t, err, ReasonChangeNotConfined)
	if !strings.Contains(err.Error(), "frontend/src/packages/readerfile") || strings.Contains(err.Error(), "reader2") {
		t.Fatalf("refusal %q: want the loose file named and the sibling not", err.Error())
	}
}

// Problem 29: after a re-graft (stack_rematerialized) the earlier round's
// projection commit sits on the old stack base while this round's bundle is
// cumulative from the booted one. The source before is the earlier round's
// change grafted onto the booted base, so what the earlier round changed is
// not this operation's change.
func TestDraftGraftsABeforeOnAnOlderStackBase(t *testing.T) {
	f := newFixture(t)
	oldBase := f.tree(map[string]entry{"frontend/src/app.css": {"100644", "body { old }\n"}})
	// Round 1 on the old base: a shared lib change (published elsewhere) and
	// a sibling package.
	round1 := f.tree(map[string]entry{
		"frontend/src/app.css":                     {"100644", "body { old }\n"},
		"frontend/src/lib/app-sdk.ts":              {"100644", "export {}\n"},
		"frontend/src/packages/sibling/App.svelte": {"100644", "<p>s</p>\n"},
	})
	// Round 2 on the booted base, cumulative: round 1's paths and the reader.
	round2 := f.tree(map[string]entry{
		testRoot + "/App.svelte":                   {"100644", "<p>reader v2</p>\n"},
		"frontend/src/lib/app-sdk.ts":              {"100644", "export {}\n"},
		"frontend/src/packages/sibling/App.svelte": {"100644", "<p>s</p>\n"},
	})
	in := DraftInput{
		Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(f.base, round2),
		Before: round1, BeforeLabel: "the previous round", Stack: []string{round1}, BeforeBase: oldBase,
	}
	source, err := DraftPackage(context.Background(), in)
	if err != nil {
		t.Fatalf("a round whose parent sits on the re-grafted old base is measured against it grafted: %v", err)
	}
	if got := strings.Join(source.Paths, ","); got != testRoot+"/App.svelte" {
		t.Fatalf("paths = %s, want only the reader", got)
	}
	// Measured from the base (the old fallback) the whole cumulative change
	// counts as this round's.
	in.BeforeBase, in.Before, in.Stack = "", f.base, nil
	_, err = DraftPackage(context.Background(), in)
	wantReason(t, err, ReasonChangeNotConfined)
}

func TestDraftDoesNotGraftWhereTheBaseMovedAtAChangedPath(t *testing.T) {
	f := newFixture(t)
	oldBase := f.tree(map[string]entry{"frontend/src/lib/app-sdk.ts": {"100644", "export { old }\n"}})
	// The earlier round changed app-sdk.ts on the old base; the booted base
	// has its own app-sdk.ts (none), so the paths moved and nothing grafts.
	round1 := f.tree(map[string]entry{"frontend/src/lib/app-sdk.ts": {"100644", "export {}\n"}})
	round2 := f.tree(map[string]entry{
		testRoot + "/App.svelte":      {"100644", "<p>reader v2</p>\n"},
		"frontend/src/lib/app-sdk.ts": {"100644", "export {}\n"},
	})
	_, err := DraftPackage(context.Background(), DraftInput{
		Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(f.base, round2),
		Before: round1, BeforeLabel: "the previous round", Stack: []string{round1}, BeforeBase: oldBase,
	})
	wantReason(t, err, ReasonChangeNotConfined)
}
