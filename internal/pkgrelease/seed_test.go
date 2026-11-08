package pkgrelease

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A host that holds no seed commit (corpusd) rebuilds the capsule's seed from
// the track head's tree patch: the update drafts on the head, where the same
// patch against the plain base is refused.
func TestDraftSeededFromTheHeadsTreePatch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	v2 := f.tree(map[string]entry{testRoot + "/App.svelte": {"100644", "<p>v2</p>\n"}, testRoot + "/notes.txt": {"100644", "n\n"}})
	s, err := newScratch(ctx, f.repo)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	full, err := s.prefixed(ctx, testRoot, f.subtree(v2))
	if err != nil {
		t.Fatal(err)
	}
	headPatch, err := s.diff(ctx, EmptyTree, full)
	if err != nil {
		t.Fatal(err)
	}
	// The operation was built on the seed: v2's package, v3's App.svelte.
	seed, err := s.seeded(ctx, f.base, testRoot, headPatch)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.subtree(seed); got != f.subtree(v2) {
		t.Fatalf("seed package %s, want %s", got, f.subtree(v2))
	}
	v3 := f.tree(map[string]entry{testRoot + "/App.svelte": {"100644", "<p>v3</p>\n"}, testRoot + "/notes.txt": {"100644", "n\n"}})
	in := DraftInput{Repo: f.repo, Name: "reader", Base: f.base, Patch: f.fullDiff(v2, v3), HeadReleaseID: strings.Repeat("f", 64), HeadTree: f.subtree(v2)}
	if _, err := DraftPackage(ctx, in); err == nil {
		t.Fatal("the plain base drafts an update of a head it does not hold")
	}
	in.SeedPatch = headPatch
	source, err := DraftPackage(ctx, in)
	if err != nil || source.Parent.Kind != ParentRelease || source.Parent.ReleaseID != in.HeadReleaseID {
		t.Fatalf("%+v %v", source.Parent, err)
	}
	if got := strings.Join(source.Paths, ","); got != testRoot+"/App.svelte" {
		t.Fatalf("the update changes %s, want App.svelte only", got)
	}
}

// corpusd's repository is a checkout at the deploy commit, whose package
// source differs from the computer's base: the seed still builds on the base
// (Devin r1: git rm --cached refused an index entry that matched neither the
// checkout's HEAD nor its work tree).
func TestSeedBuildsOnABaseTheCheckoutDoesNotHold(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	app := filepath.Join(f.repo, filepath.FromSlash(testRoot+"/App.svelte"))
	if err := os.WriteFile(app, []byte("<p>deployed</p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, f.repo, nil, "", "commit", "-q", "-am", "deploy")
	v2 := f.tree(map[string]entry{testRoot + "/App.svelte": {"100644", "<p>v2</p>\n"}})
	s, err := newScratch(ctx, f.repo)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	full, err := s.prefixed(ctx, testRoot, f.subtree(v2))
	if err != nil {
		t.Fatal(err)
	}
	headPatch, err := s.diff(ctx, EmptyTree, full)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := s.seeded(ctx, f.base, testRoot, headPatch)
	if err != nil {
		t.Fatalf("seed on a base the checkout does not hold: %v", err)
	}
	if got := f.subtree(seed); got != f.subtree(v2) {
		t.Fatalf("seed package %s, want %s", got, f.subtree(v2))
	}
}
