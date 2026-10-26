package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crypto/ed25519"
	"crypto/rand"

	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Review 291 of Library S3 (tier 2, Devin and DeepSeek), written before the
// fixes. Failure modes pinned:
//
//	F1   a catalog read scans the listing kind by a JSON comparison with no
//	     index (and does so even when nothing is widened): the widened
//	     listings must be read through the registered lookup key, paged by
//	     its primary key, and a catalog with nothing widened reads no
//	     listing object of another owner; before the keys are live the
//	     catalog finds no other owner's track (it fails narrow, never scans);
//	F2   a read of another owner's release finds it by an unregistered JSON
//	     scan: release_id is a registered lookup key, and before the keys
//	     are live such a read is the missing release's 404;
//	F4   a captured widen receipt, replayed inside its TTL after a
//	     narrowing, widens again: a receipt already in the listing's chain
//	     refuses owner_act_replayed;
//	F5   a track read with a page size of 0 never ends; a narrowing of a
//	     withdrawn listing overwrites its state reason.

func TestPackageLookupsAreIndexBacked(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic})
	og := w.h.service.ogStore()
	db := og.store.corpus()

	// F1: the widened listings are a keyset page of the lookup key's
	// primary key.
	assertPlan(t, "widened listings", explainPlan(t, db, lookupKeyPageSQL, string(ReleaseListingKind), "true", "", 50),
		"og_lookup_keys", "object_kind", "lookup_key", "canonical_id")
	if !lookupKeyFor(string(ReleaseListingKind), "$.widened") {
		t.Fatal("F1 the listing's widened field is not a registered lookup key")
	}
	// A track read is the owner's: it reads through (object_kind, owner_id).
	q, a := metadataPageSQL(string(ReleaseListingKind), "$.package", "reader", "owner-a", "", 50)
	assertPlan(t, "an owner's track page", explainPlan(t, db, q, a...), "og_objects", "object_kind", "owner_id")
	// F2: a release is found by its id through the key.
	if !lookupKeyFor(string(PackageReleaseKind), "$.release_id") {
		t.Fatal("F2 release_id is not a registered lookup key")
	}
	if _, _, shape, none, err := og.metadataPredicate(context.Background(), string(PackageReleaseKind), "$.release_id", r1.id); err != nil || none || shape != "lookup_key" {
		t.Fatalf("F2 the release id lookup = %q none=%v %v, want the lookup key", shape, none, err)
	}
	// The key rows are there, from the writes themselves.
	objects, cursor, err := og.ListKeyedObjectsPage(context.Background(), string(ReleaseListingKind), "$.widened", "true", "", 50)
	if err != nil || len(objects) != 1 || cursor == "" {
		t.Fatalf("F1 widened page = %d %q %v, want the one widened version", len(objects), cursor, err)
	}
	if more, _, err := og.ListKeyedObjectsPage(context.Background(), string(ReleaseListingKind), "$.widened", "true", cursor, 50); err != nil || len(more) != 0 {
		t.Fatalf("F1 the page after the last = %d %v", len(more), err)
	}
}

func TestCatalogReadsNoWidenedListingWhenNoneIsWidened(t *testing.T) {
	w := newPkgWorld(t)
	a1 := w.build("<p>a</p>\n", "owner-a", nil)
	w.published(a1)
	b1 := w.build("<p>b</p>\n", "owner-b", nil)
	w.published(b1)

	w.h.widenedListingReads.Store(0)
	catalogRows(t, w.read(packagesRoute+"/catalog?owner_id=owner-b"))
	if n := w.h.widenedListingReads.Load(); n != 0 {
		t.Fatalf("F1 a catalog read with nothing widened read %d widened listing objects", n)
	}
	w.share(a1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b")
	w.wantVisible(a1, "owner-b")
	if n := w.h.widenedListingReads.Load(); n == 0 {
		t.Fatal("F1 the counter does not count widened reads")
	}

	// Before the keys are live: no scan, and nothing of another owner's
	// (fail narrow) until the backfill switches them on.
	og := w.h.service.ogStore()
	og.store.retrievalState.lookupKeyLive.Store(false)
	w.h.widenedListingReads.Store(0)
	for _, row := range catalogRows(t, w.read(packagesRoute+"/catalog?owner_id=owner-b")) {
		if row.Track == "owner-a" {
			t.Fatalf("F1 before the keys are live the catalog found another owner's track: %+v", row)
		}
	}
	if n := w.h.widenedListingReads.Load(); n != 0 {
		t.Fatalf("F1 before the keys are live the catalog read %d widened listings", n)
	}
	missing := w.read(packagesRoute + "/releases/" + strings.Repeat("0", 64) + "?owner_id=owner-b")
	if got := w.read(packagesRoute + "/releases/" + a1.id + "?owner_id=owner-b"); got.Code != http.StatusNotFound || got.Body.String() != missing.Body.String() {
		t.Fatalf("F2 before the keys are live a foreign read = %d %s, want the missing release's 404", got.Code, got.Body.String())
	}
	// The owner's own reads do not need the keys.
	if got := w.read(packagesRoute + "/releases/" + a1.id + "?owner_id=owner-a"); got.Code != http.StatusOK {
		t.Fatalf("the owner's read before the keys are live = %d", got.Code)
	}
	og.store.retrievalState.lookupKeyLive.Store(true)
	w.wantVisible(a1, "owner-b")
}

func TestListingReceiptIsNotReplayed(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := owneract.NewSigner(private)
	verifier, _ := owneract.NewVerifier(public)
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	w.h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActEnforce, verifier))
	w.signer = signer

	widen := ListingChange{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}}
	widen.ConfirmPaths = w.preview(r1, "owner-a", widen).PathsDigest
	receipt, err := signer.Mint(owneract.Act{Kind: owneract.ActPackageListing, OwnerUserID: "owner-a", AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session:test", ComputerID: computerOf("owner-a"), Subject: PackageListingSubject(r1.id, widen)}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	captured, _ := owneract.Encode(receipt)
	send := func(change ListingChange, header string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(PackageListingRequest{OwnerID: "owner-a", ComputerID: computerOf("owner-a"), ListingChange: change})
		r := httptest.NewRequest(http.MethodPut, listingsRoute+"/"+r1.id, strings.NewReader(string(body)))
		r.Header.Set("X-Internal-Caller", "true")
		r.Header.Set(owneract.Header, header)
		rec := httptest.NewRecorder()
		w.h.HandleListings(rec, r)
		return rec
	}
	// A refused act writes nothing, so its receipt is not spent: the
	// confirmation can follow the preview under the same receipt.
	unconfirmed := widen
	unconfirmed.ConfirmPaths = ""
	wantPkgRefusal(t, send(unconfirmed, captured), http.StatusConflict, pkgrelease.ReasonConfirmPathsRequired)
	if rec := send(widen, captured); rec.Code != http.StatusOK {
		t.Fatalf("the widen = %d %s", rec.Code, rec.Body.String())
	}
	w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner})
	w.wantInvisible(r1, "owner-c")
	// F4: the captured receipt, replayed after the narrowing.
	wantPkgRefusal(t, send(widen, captured), http.StatusConflict, pkgrelease.ReasonOwnerActReplayed)
	w.wantInvisible(r1, "owner-c")
}

func TestTrackReadsEndWithAZeroPageSize(t *testing.T) {
	saved := trackPageSize
	trackPageSize = 0
	t.Cleanup(func() { trackPageSize = saved })
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	w.published(r2)
	if page := history(t, w.read(packagesRoute+"/catalog/reader/releases?owner_id=owner-a")); len(page.Releases) != 2 {
		t.Fatalf("F5 history with a zero page size = %+v", page)
	}
}

func TestNarrowingKeepsTheStateReason(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b")
	if rec := w.listingAct(r1, "owner-a", ListingChange{State: pkgrelease.StateWithdrawn, Reason: "bad build"}); rec.Code != http.StatusOK {
		t.Fatalf("withdraw = %d %s", rec.Code, rec.Body.String())
	}
	got := w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner})
	if !got.Changed || got.Listing.State != pkgrelease.StateWithdrawn || got.Listing.StateReason != "bad build" {
		t.Fatalf("F5 a narrowing of a withdrawn listing = %+v, want state withdrawn and its reason kept", got.Listing)
	}
}
