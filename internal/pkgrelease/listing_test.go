package pkgrelease

import (
	"context"
	"strings"
	"testing"
)

// Library S3 listing shapes (docs/design/library-package-propagation-2026-10-26.md
// 4.1, 4.2, 4.4), written before the code. Failure modes pinned:
//
//	L4   a listing is visible to a reader its scope set does not name: an
//	     owner listing to anyone but the track's owner, a shared listing to
//	     an owner it does not name, any scope to an admin as such;
//	S1   a scope S3 cannot enforce is accepted: platform (S4) and named
//	     computers (no computer reads packages before S5) must refuse by
//	     name, not be stored and ignored;
//	S2   a widening is not recognized as one (it would skip --confirm-paths
//	     and the second secret scan), or a narrowing is taken for one;
//	L21  the widening preview omits a path that leaves, or passes a secret.

func listingOn(scopes []string, owners ...string) Listing {
	return Listing{
		RecordKind: RecordKindPackageRelease, RecordID: strings.Repeat("a", 64), Package: "reader", Track: "owner-a",
		Seq: 1, Scopes: scopes, SharedWith: SharedWith{Owners: owners}, State: StateListed, StateActor: "owner_act:x", Version: 1,
	}
}

func TestListingScopeSetsS3Accepts(t *testing.T) {
	for _, ok := range []Listing{
		listingOn([]string{ScopeOwner}),
		listingOn([]string{ScopeOwner, ScopeShared}, "owner-b"),
		listingOn([]string{ScopeShared, ScopeOwner}, "owner-c", "owner-b"),
		listingOn([]string{ScopeOwner, ScopePublic}),
		listingOn([]string{ScopeOwner, ScopeShared, ScopePublic}, "owner-b"),
	} {
		if err := ok.Validate(); err != nil {
			t.Fatalf("%v %v: %v", ok.Scopes, ok.SharedWith.Owners, err)
		}
	}
	for name, c := range map[string]struct {
		listing Listing
		reason  string
	}{
		"platform before S4":    {listingOn([]string{ScopeOwner, ScopePlatform}), ReasonScopeNotBuilt},
		"no owner scope":        {listingOn([]string{ScopeShared}, "owner-b"), ReasonInvalid},
		"shared with nobody":    {listingOn([]string{ScopeOwner, ScopeShared}), ReasonInvalid},
		"owners without shared": {listingOn([]string{ScopeOwner}, "owner-b"), ReasonInvalid},
		"shared with the owner": {listingOn([]string{ScopeOwner, ScopeShared}, "owner-a"), ReasonInvalid},
		"an unknown scope":      {listingOn([]string{ScopeOwner, "everyone"}), ReasonInvalid},
		"a malformed owner id":  {listingOn([]string{ScopeOwner, ScopeShared}, "owner b/../"), ReasonInvalid},
		"a duplicated scope":    {listingOn([]string{ScopeOwner, ScopeOwner}), ReasonInvalid},
		"a duplicated owner":    {listingOn([]string{ScopeOwner, ScopeShared}, "owner-b", "owner-b"), ReasonInvalid},
		"named computers (S5)": {func() Listing {
			l := listingOn([]string{ScopeOwner, ScopeShared}, "owner-b")
			l.SharedWith.Computers = []string{"computer-b1"}
			return l
		}(), ReasonScopeNotBuilt},
	} {
		if got := ReasonOf(c.listing.Validate()); got != c.reason {
			t.Fatalf("S1 %s: reason %q, want %q", name, got, c.reason)
		}
	}
}

func TestListingVisibleToFollowsTheScopeSet(t *testing.T) {
	owner := listingOn([]string{ScopeOwner})
	shared := listingOn([]string{ScopeOwner, ScopeShared}, "owner-b")
	public := listingOn([]string{ScopeOwner, ScopePublic})
	for _, c := range []struct {
		listing Listing
		reader  string
		want    bool
	}{
		{owner, "owner-a", true}, {owner, "owner-b", false}, {owner, "", false},
		{shared, "owner-a", true}, {shared, "owner-b", true}, {shared, "owner-c", false},
		{public, "owner-a", true}, {public, "owner-c", true}, {public, "", false},
	} {
		if got := c.listing.VisibleTo(c.reader); got != c.want {
			t.Fatalf("L4 %v %v to %q = %v, want %v", c.listing.Scopes, c.listing.SharedWith.Owners, c.reader, got, c.want)
		}
	}
}

func TestWidensIsAnyNewReader(t *testing.T) {
	owner := listingOn([]string{ScopeOwner})
	sharedB := listingOn([]string{ScopeOwner, ScopeShared}, "owner-b")
	sharedBC := listingOn([]string{ScopeOwner, ScopeShared}, "owner-b", "owner-c")
	public := listingOn([]string{ScopeOwner, ScopePublic})
	for _, c := range []struct {
		from, to Listing
		want     bool
	}{
		{owner, sharedB, true}, {sharedB, sharedBC, true}, {sharedB, public, true}, {owner, public, true},
		{sharedBC, sharedB, false}, {sharedB, owner, false}, {public, owner, false}, {sharedB, sharedB, false},
		// Public to shared-with-b narrows: b could read it already.
		{public, sharedB, false},
	} {
		if got := Widens(c.from, c.to); got != c.want {
			t.Fatalf("S2 %v%v -> %v%v widens = %v, want %v", c.from.Scopes, c.from.SharedWith.Owners, c.to.Scopes, c.to.SharedWith.Owners, got, c.want)
		}
	}
}

func TestWideningPreviewListsEveryPathAndRescans(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>v2</p>\n")))
	preview, err := WideningPreview(context.Background(), record, blobs.TreePatch, blobs.ReviewPatch)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(preview.Paths, "\n")
	for _, want := range []string{record.Source.Root + "/App.svelte", record.Source.Root + "/choir-package.json"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("L21 the preview omits %s: %v", want, preview.Paths)
		}
	}
	if preview.ReviewPatchBytes != len(blobs.ReviewPatch) || len(preview.PathsDigest) != 64 {
		t.Fatalf("L21 preview = %+v", preview)
	}
	again, _ := WideningPreview(context.Background(), record, blobs.TreePatch, blobs.ReviewPatch)
	if again.PathsDigest != preview.PathsDigest {
		t.Fatal("the paths digest is not stable")
	}
	// The digest binds the release: the same paths of another release are
	// another confirmation.
	other := record
	other.Operation.OperationID = "op-other"
	if p, _ := WideningPreview(context.Background(), other, blobs.TreePatch, blobs.ReviewPatch); p.PathsDigest == preview.PathsDigest {
		t.Fatal("the paths digest does not bind the release")
	}

	// The scan runs again: a stored patch that adds a secret refuses.
	secret := f.tree(changed("GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n"))
	record, blobs = f.release(secret)
	wantReason(t, func() error {
		_, err := WideningPreview(context.Background(), record, blobs.TreePatch, blobs.ReviewPatch)
		return err
	}(), ReasonSecretFound)
}
