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
//	     index: the published listings (every one, since 2026-11-03) must
//	     be read through the registered lookup key (record_kind), paged by
//	     its primary key; before the keys are live the catalog finds no
//	     other owner's track (it fails narrow, never scans);
//	F2   a read of another owner's release finds it by an unregistered JSON
//	     scan: release_id is a registered lookup key, and before the keys
//	     are live such a read is the missing release's 404;
//	F4   a captured withdraw receipt, replayed inside its TTL after a
//	     reinstate, withdraws again: a receipt already in the listing's
//	     chain refuses owner_act_replayed;
//	F5   a track read with a page size of 0 never ends.

func TestPackageLookupsAreIndexBacked(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	og := w.h.service.ogStore()
	db := og.store.corpus()

	// F1: the published listings are a keyset page of the lookup key's
	// primary key.
	assertPlan(t, "published listings", explainPlan(t, db, lookupKeyPageSQL, string(ReleaseListingKind), pkgrelease.RecordKindPackageRelease, "", 50),
		"og_lookup_keys", "object_kind", "lookup_key", "canonical_id")
	if !lookupKeyFor(string(ReleaseListingKind), "$.record_kind") {
		t.Fatal("F1 the listing's record_kind field is not a registered lookup key")
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
	objects, cursor, err := og.ListKeyedObjectsPage(context.Background(), string(ReleaseListingKind), "$.record_kind", pkgrelease.RecordKindPackageRelease, "", 50)
	if err != nil || len(objects) != 1 || cursor == "" {
		t.Fatalf("F1 published page = %d %q %v, want the one listing version", len(objects), cursor, err)
	}
	if more, _, err := og.ListKeyedObjectsPage(context.Background(), string(ReleaseListingKind), "$.record_kind", pkgrelease.RecordKindPackageRelease, cursor, 50); err != nil || len(more) != 0 {
		t.Fatalf("F1 the page after the last = %d %v", len(more), err)
	}
}

func TestCatalogFindsOtherOwnersOnlyThroughTheKey(t *testing.T) {
	w := newPkgWorld(t)
	a1 := w.build("<p>a</p>\n", "owner-a", nil)
	w.published(a1)
	b1 := w.build("<p>b</p>\n", "owner-b", nil)
	w.published(b1)

	w.h.publicListingReads.Store(0)
	w.wantVisible(a1, "owner-b")
	if n := w.h.publicListingReads.Load(); n == 0 {
		t.Fatal("F1 the counter does not count the keyed listing reads")
	}

	// Before the keys are live: no scan, and nothing of another owner's
	// (fail narrow) until the backfill switches them on.
	og := w.h.service.ogStore()
	og.store.retrievalState.lookupKeyLive.Store(false)
	w.h.publicListingReads.Store(0)
	for _, row := range catalogRows(t, w.read(packagesRoute+"/catalog?owner_id=owner-b")) {
		if row.Track == "owner-a" {
			t.Fatalf("F1 before the keys are live the catalog found another owner's track: %+v", row)
		}
	}
	if n := w.h.publicListingReads.Load(); n != 0 {
		t.Fatalf("F1 before the keys are live the catalog read %d listings", n)
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

// The listings filed before 2026-11-03 carry no record_kind key row under
// the old registration (only widened ones had a row, keyed "true"): the
// boot backfill re-keys them, so the catalog finds every published track.
func TestBackfillRekeysListingsFiledUnderTheOldKey(t *testing.T) {
	w := newPkgWorld(t)
	a1 := w.build("<p>a</p>\n", "owner-a", nil)
	w.published(a1)
	og := w.h.service.ogStore()
	db := og.store.corpus()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `UPDATE og_lookup_keys SET lookup_key = 'true' WHERE object_kind = ?`, string(ReleaseListingKind)); err != nil {
		t.Fatal(err)
	}
	b1 := w.build("<p>b</p>\n", "owner-b", nil)
	w.published(b1)
	if _, err := db.ExecContext(ctx, `DELETE FROM og_lookup_keys WHERE object_kind = ? AND lookup_key <> 'true'`, string(ReleaseListingKind)); err != nil {
		t.Fatal(err)
	}
	tracks := func() string {
		t.Helper()
		var out []string
		for _, row := range catalogRows(t, w.read(packagesRoute+"/catalog?owner_id=owner-c")) {
			out = append(out, row.Track)
		}
		return strings.Join(out, ",")
	}
	if got := tracks(); got != "" {
		t.Fatalf("the catalog before the backfill = %q, want nothing (the old keys)", got)
	}
	if err := og.store.ensureLookupKeys(ctx); err != nil {
		t.Fatal(err)
	}
	if got := tracks(); got != "owner-a,owner-b" {
		t.Fatalf("the catalog after the backfill = %q, want both tracks", got)
	}
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

	withdraw := ListingChange{State: pkgrelease.StateWithdrawn}
	receipt, err := signer.Mint(owneract.Act{Kind: owneract.ActPackageListing, OwnerUserID: "owner-a", AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session:test", ComputerID: computerOf("owner-a"), Subject: PackageListingSubject(r1.id, withdraw)}, time.Now())
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
	if rec := send(withdraw, captured); rec.Code != http.StatusOK {
		t.Fatalf("the withdraw = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.state(r1, "owner-a", pkgrelease.StateListed); rec.Code != http.StatusOK {
		t.Fatalf("the reinstate = %d %s", rec.Code, rec.Body.String())
	}
	w.wantVisible(r1, "owner-c")
	// F4: the captured receipt, replayed after the reinstate.
	wantPkgRefusal(t, send(withdraw, captured), http.StatusConflict, pkgrelease.ReasonOwnerActReplayed)
	w.wantVisible(r1, "owner-c")
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
