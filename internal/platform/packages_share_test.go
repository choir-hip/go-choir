package platform

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// The Library at corpusd after the owner's ruling of 2026-11-03 ("published
// releases mean published, for everyone to see ... no such thing as
// published only to the computer that made it"), written before the code.
// Failure modes pinned here:
//
//	P1   a release published by owner A is missing from owner B's single
//	     read, catalog or history; a release listed at owner scope or shared
//	     with named owners before the ruling stays hidden from an owner it
//	     does not name (the rule is read time, no migration);
//	P2   a withdrawn (or yanked) release is still shown to another owner;
//	     reinstated, it does not come back;
//	P3   publishing makes a release public without the owner confirming the
//	     paths that become public, or the confirmation does not say plainly
//	     that the release becomes visible to everyone; confirmed for other
//	     paths, it publishes;
//	P4   share is still a separate act: a scope change is written instead of
//	     refused as share_retired with the reason;
//	L4   another owner learns who a pre-ruling listing was shared with;
//	L21  a publisher screenshot, capture or evidence reference travels;
//	L24  two owners' tracks of one package mix: a catalog row, a history or
//	     a track head answers from both, or a publish takes another owner's
//	     release as its parent;
//	H1   the history: a withdrawn release leaves the catalog heads and the
//	     track head but stays in its publisher's history as withdrawn;
//	     reinstate brings it back while it can still head the track, and
//	     refuses track_moved once the track moved past it; a yanked release
//	     takes no owner act;
//	A2   a listing act without the owner's receipt (under enforce), with a
//	     receipt for another change, or by another owner, is written;
//	I2   (S2 residual bunch279-I2) a track at one page of listings refuses a
//	     publish, or a read answers from part of the track: every read pages
//	     to the end, and a publish at the old cap lists.

// listingChange is one owner act on a listing, as the proxy sends it.
func (w *pkgWorld) listingAct(b built, owner string, change ListingChange) *httptest.ResponseRecorder {
	w.t.Helper()
	body, _ := json.Marshal(PackageListingRequest{OwnerID: owner, ComputerID: computerOf(owner), ListingChange: change})
	r := httptest.NewRequest(http.MethodPut, listingsRoute+"/"+b.id, strings.NewReader(string(body)))
	r.Header.Set("X-Internal-Caller", "true")
	if w.signer != nil {
		receipt, err := w.signer.Mint(owneract.Act{
			Kind: owneract.ActPackageListing, OwnerUserID: owner, AuthMethod: owneract.AuthMethodSession,
			CredentialRef: "session:test", ComputerID: computerOf(owner), Subject: PackageListingSubject(b.id, change),
		}, time.Now())
		if err != nil {
			w.t.Fatal(err)
		}
		encoded, err := owneract.Encode(receipt)
		if err != nil {
			w.t.Fatal(err)
		}
		r.Header.Set(owneract.Header, encoded)
	}
	rec := httptest.NewRecorder()
	w.h.HandleListings(rec, r)
	return rec
}

func (w *pkgWorld) state(b built, owner, state string) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.listingAct(b, owner, ListingChange{State: state, Reason: "test"})
}

func catalogRows(t *testing.T, rec *httptest.ResponseRecorder) []PackageCatalogEntry {
	t.Helper()
	var catalog struct {
		Packages []PackageCatalogEntry `json:"packages"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &catalog) != nil {
		t.Fatalf("catalog = %d %s", rec.Code, rec.Body.String())
	}
	return catalog.Packages
}

type historyPage struct {
	Track         string               `json:"track"`
	Releases      []PackageReleaseView `json:"releases"`
	NextBeforeSeq int                  `json:"next_before_seq"`
}

func history(t *testing.T, rec *httptest.ResponseRecorder) historyPage {
	t.Helper()
	var page historyPage
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &page) != nil {
		t.Fatalf("history = %d %s", rec.Code, rec.Body.String())
	}
	return page
}

// wantInvisible is L4 for one reader: the single read is the missing
// release's 404, the catalog and the track's history are 200 and empty.
func (w *pkgWorld) wantInvisible(b built, reader string) {
	w.t.Helper()
	got := w.read(packagesRoute + "/releases/" + b.id + "?owner_id=" + reader)
	missing := w.read(packagesRoute + "/releases/" + strings.Repeat("0", 64) + "?owner_id=" + reader)
	if got.Code != http.StatusNotFound || got.Body.String() != missing.Body.String() {
		w.t.Fatalf("L4 %s's single read = %d %s, want the missing release's 404 %s", reader, got.Code, got.Body.String(), missing.Body.String())
	}
	for _, row := range catalogRows(w.t, w.read(packagesRoute+"/catalog?owner_id="+reader)) {
		if row.Track == b.record.Track {
			w.t.Fatalf("L4 %s's catalog shows %s's track: %+v", reader, b.record.Track, row)
		}
	}
	if page := history(w.t, w.read(packagesRoute+"/catalog/reader/releases?owner_id="+reader+"&track="+b.record.Track)); len(page.Releases) != 0 {
		w.t.Fatalf("L4 %s's history of %s's track = %+v, want empty", reader, b.record.Track, page.Releases)
	}
}

func (w *pkgWorld) wantVisible(b built, reader string) PackageReleaseView {
	w.t.Helper()
	rec := w.read(packagesRoute + "/releases/" + b.id + "?owner_id=" + reader)
	var view PackageReleaseView
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &view) != nil || view.ReleaseID != b.id {
		w.t.Fatalf("P1 %s's read of a published release = %d %s", reader, rec.Code, rec.Body.String())
	}
	found := false
	for _, row := range catalogRows(w.t, w.read(packagesRoute+"/catalog?owner_id="+reader)) {
		found = found || (row.Track == b.record.Track && row.Package == "reader")
	}
	if !found {
		w.t.Fatalf("L4 %s's catalog lacks %s's reader track", reader, b.record.Track)
	}
	page := history(w.t, w.read(packagesRoute+"/catalog/reader/releases?owner_id="+reader+"&track="+b.record.Track))
	if len(page.Releases) == 0 || !strings.Contains(w.read(packagesRoute+"/catalog/reader/releases?owner_id="+reader+"&track="+b.record.Track).Body.String(), b.id) {
		w.t.Fatalf("L4 %s's history of %s's track lacks the release: %+v", reader, b.record.Track, page)
	}
	return view
}

func TestPublishedReleaseIsVisibleToEveryOwner(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	for _, reader := range []string{"owner-a", "owner-b", "owner-c"} {
		w.wantVisible(r1, reader)
	}
	// Another owner's view names no sharee and carries no publisher capture.
	view := w.wantVisible(r1, "owner-b")
	if len(view.Listing.SharedWith.Owners) != 0 {
		t.Fatalf("L4 owner-b's view names owners: %+v", view.Listing)
	}
	raw := strings.ToLower(string(shareJSON(t, view)))
	for _, banned := range []string{"screenshot", "capture", "evidence", "visual"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("L21 a public view carries %q: %s", banned, raw)
		}
	}
	// A signed-out read (no owner) is still refused, never unscoped.
	if rec := w.read(packagesRoute + "/releases/" + r1.id); rec.Code != http.StatusBadRequest {
		t.Fatalf("a read naming no owner = %d", rec.Code)
	}
}

// storeListed files a release and a first listing with the given scope
// set, as a publish before the 2026-11-03 ruling (or a share) wrote it.
func (w *pkgWorld) storeListed(b built, scopes []string, owners ...string) {
	w.t.Helper()
	w.storeRecordOnly(b)
	listing := pkgrelease.Listing{
		RecordKind: pkgrelease.RecordKindPackageRelease, RecordID: b.id, Package: b.record.Package.Name, Track: b.record.Track,
		Seq: 1, Scopes: scopes, SharedWith: pkgrelease.SharedWith{Owners: owners}, State: pkgrelease.StateListed, StateReason: "published", StateActor: "owner_act:test", Version: 1,
	}
	obj, err := listingObject(b.record.Publisher.OwnerID, b.record.Publisher.ComputerID, listing, time.Now().UTC())
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		w.t.Fatal(err)
	}
}

func TestReleasesListedBeforeTheRulingArePublic(t *testing.T) {
	w := newPkgWorld(t)
	ownerOnly := w.build("<p>owner only</p>\n", "owner-a", nil)
	w.storeListed(ownerOnly, []string{pkgrelease.ScopeOwner})
	shared := w.build("<p>shared</p>\n", "owner-b", nil)
	w.storeListed(shared, []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-d")
	for _, reader := range []string{"owner-a", "owner-c", "owner-d"} {
		w.wantVisible(ownerOnly, reader)
	}
	for _, reader := range []string{"owner-b", "owner-c", "owner-d"} {
		w.wantVisible(shared, reader)
	}
	// L4: an owner the old share did not name does not learn who it named.
	if view := w.wantVisible(shared, "owner-c"); strings.Contains(string(shareJSON(t, view)), "owner-d") {
		t.Fatalf("L4 owner-c's view names the old sharee: %s", shareJSON(t, view))
	}
	if view := w.wantVisible(shared, "owner-b"); strings.Join(view.Listing.SharedWith.Owners, ",") != "owner-d" {
		t.Fatalf("the publisher's own view of its listing = %+v", view.Listing)
	}
}

func TestPublishPreviewsWhatBecomesPublicAndNeedsItConfirmed(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	rec := w.publishConfirming(r1, "", "")
	var refusal PackageListingRefusal
	if rec.Code != http.StatusConflict || json.Unmarshal(rec.Body.Bytes(), &refusal) != nil || refusal.Reason != pkgrelease.ReasonConfirmPathsRequired || refusal.Preview == nil {
		t.Fatalf("P3 an unconfirmed publish = %d %s, want 409 confirm_paths_required with a preview", rec.Code, rec.Body.String())
	}
	// P3: the confirmation says plainly that everyone will see it.
	for _, want := range []string{"public", "every signed-in owner", "install", "--confirm-paths " + refusal.Preview.PathsDigest} {
		if !strings.Contains(refusal.Error, want) {
			t.Fatalf("P3 the publish confirmation %q does not say %q", refusal.Error, want)
		}
	}
	paths := strings.Join(refusal.Preview.Paths, "\n")
	for _, want := range []string{pkgRoot + "/App.svelte", pkgRoot + "/choir-package.json"} {
		if !strings.Contains(paths, want) {
			t.Fatalf("L21 the preview omits %s: %v", want, refusal.Preview.Paths)
		}
	}
	if refusal.Preview.ReviewPatchBytes != r1.record.Source.ReviewPatch.Bytes {
		t.Fatalf("L21 the preview's review size = %d, want %d", refusal.Preview.ReviewPatchBytes, r1.record.Source.ReviewPatch.Bytes)
	}
	// Nothing was published by the preview.
	if rec := w.read(packagesRoute + "/releases/" + r1.id + "?owner_id=owner-a"); rec.Code != http.StatusNotFound {
		t.Fatalf("P3 the preview published: %d %s", rec.Code, rec.Body.String())
	}
	// Confirmed for other paths: refused, nothing written.
	wantPkgRefusal(t, w.publishConfirming(r1, "", strings.Repeat("f", 64)), http.StatusConflict, pkgrelease.ReasonConfirmPathsMismatch)
	w.wantInvisible(r1, "owner-b")
	// Confirmed: published, and public.
	if rec := w.publishConfirming(r1, "", refusal.Preview.PathsDigest); rec.Code != http.StatusCreated {
		t.Fatalf("a confirmed publish = %d %s", rec.Code, rec.Body.String())
	}
	w.wantVisible(r1, "owner-b")
	// A republish of what is already public needs no confirmation.
	if rec := w.publishConfirming(r1, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("a republish = %d %s", rec.Code, rec.Body.String())
	}
}

func TestShareIsRetired(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	for _, change := range []ListingChange{
		{Scopes: []string{pkgrelease.ScopeOwner}},
		{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}},
		{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, SharedWith: &pkgrelease.SharedWith{Owners: []string{"owner-b"}}},
		{State: pkgrelease.StateWithdrawn, SharedWith: &pkgrelease.SharedWith{Owners: []string{"owner-b"}}},
	} {
		got := wantPkgRefusal(t, w.listingAct(r1, "owner-a", change), http.StatusGone, pkgrelease.ReasonShareRetired)
		if !strings.Contains(got.Error, "public") || !strings.Contains(got.Error, "withdraw") {
			t.Fatalf("P4 the retired share does not explain: %q", got.Error)
		}
	}
	if v := w.wantVisible(r1, "owner-a"); v.Listing.Version != 1 {
		t.Fatalf("P4 a retired share wrote a version: %+v", v.Listing)
	}
	w.wantVisible(r1, "owner-c")
}

func shareJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// storeUnchecked files a release and its first listing the way a publish
// that passed would, without the host checks: a release that a later scan
// would catch (the scanner learned a pattern after publish).
func (w *pkgWorld) storeUnchecked(b built) {
	w.t.Helper()
	w.storeRecordOnly(b)
	listing := pkgrelease.Listing{
		RecordKind: pkgrelease.RecordKindPackageRelease, RecordID: b.id, Package: b.record.Package.Name, Track: b.record.Track,
		Seq: 1, Scopes: []string{pkgrelease.ScopeOwner}, State: pkgrelease.StateListed, StateReason: "published", StateActor: "owner_act:test", Version: 1,
	}
	obj, err := listingObject(b.record.Publisher.OwnerID, b.record.Publisher.ComputerID, listing, time.Now().UTC())
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		w.t.Fatal(err)
	}
}

func TestWithdrawNeedsNoScan(t *testing.T) {
	w := newPkgWorld(t)
	secret := w.build("GITHUB_TOKEN=ghp_"+strings.Repeat("A", 36)+"\n", "owner-a", nil)
	w.storeUnchecked(secret)
	// The owner can always take a release down, even one a later scan
	// would catch, and then nobody else sees it.
	if rec := w.state(secret, "owner-a", pkgrelease.StateWithdrawn); rec.Code != http.StatusOK {
		t.Fatalf("withdrawing a release that fails the scan = %d %s", rec.Code, rec.Body.String())
	}
	w.wantInvisible(secret, "owner-c")
}

func TestPackageWithdrawLeavesTheHeadsAndReinstateReturnsIt(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	w.published(r2)

	head := func(reader string) string {
		t.Helper()
		for _, row := range catalogRows(t, w.read(packagesRoute+"/catalog?owner_id="+reader)) {
			if row.Track == "owner-a" && row.Package == "reader" {
				return row.HeadReleaseID
			}
		}
		return ""
	}
	states := func(reader string) map[string]string {
		t.Helper()
		out := map[string]string{}
		for _, view := range history(t, w.read(packagesRoute+"/catalog/reader/releases?owner_id="+reader+"&track=owner-a")).Releases {
			out[view.ReleaseID] = view.Listing.State
		}
		return out
	}

	// H1: withdrawn, r2 leaves the heads and stays in the history.
	rec := w.state(r2, "owner-a", pkgrelease.StateWithdrawn)
	var withdrawn PackageListingResult
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &withdrawn) != nil || withdrawn.Listing.State != pkgrelease.StateWithdrawn ||
		withdrawn.Listing.Version != 2 || !strings.HasPrefix(withdrawn.Listing.StateActor, "owner_act:") {
		t.Fatalf("withdraw = %d %s", rec.Code, rec.Body.String())
	}
	for _, reader := range []string{"owner-a", "owner-b"} {
		if got := head(reader); got != r1.id {
			t.Fatalf("H1 %s's catalog head after a withdraw = %s, want r1", reader, got)
		}
	}
	// H1: the publisher's history keeps it as withdrawn; P2: nobody else
	// sees it at all.
	if got := states("owner-a"); got[r2.id] != pkgrelease.StateWithdrawn || got[r1.id] != pkgrelease.StateListed {
		t.Fatalf("H1 the publisher's history after a withdraw = %v", got)
	}
	if got := states("owner-b"); len(got) != 1 || got[r1.id] != pkgrelease.StateListed {
		t.Fatalf("P2 another owner's history after a withdraw = %v, want r1 only", got)
	}
	missing := w.read(packagesRoute + "/releases/" + strings.Repeat("0", 64) + "?owner_id=owner-b")
	if got := w.read(packagesRoute + "/releases/" + r2.id + "?owner_id=owner-b"); got.Code != http.StatusNotFound || got.Body.String() != missing.Body.String() {
		t.Fatalf("P2 owner-b's read of a withdrawn release = %d %s, want the missing release's 404", got.Code, got.Body.String())
	}
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-a"); !strings.Contains(rec.Body.String(), r1.id) {
		t.Fatalf("H1 the track head after a withdraw = %s, want r1", rec.Body.String())
	}
	// Withdrawing again writes nothing.
	if rec := w.state(r2, "owner-a", pkgrelease.StateWithdrawn); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"changed":true`) {
		t.Fatalf("a second withdraw = %d %s", rec.Code, rec.Body.String())
	}

	// H1: reinstated while nothing was published past it, r2 heads again.
	if rec := w.state(r2, "owner-a", pkgrelease.StateListed); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"listed"`) {
		t.Fatalf("reinstate = %d %s", rec.Code, rec.Body.String())
	}
	if got := head("owner-b"); got != r2.id {
		t.Fatalf("H1 the head after a reinstate = %s, want r2", got)
	}
	if got := states("owner-b"); got[r2.id] != pkgrelease.StateListed {
		t.Fatalf("P2 another owner's history after a reinstate = %v", got)
	}
	w.wantVisible(r2, "owner-b")

	// Withdrawn again, and the track moves past it: r3 on r1. Reinstating r2
	// now would fork the track, so it refuses.
	w.state(r2, "owner-a", pkgrelease.StateWithdrawn)
	r3 := w.build("<p>v4</p>\n", "owner-a", &r1)
	if v := w.published(r3); v.Listing.Seq != 3 {
		t.Fatalf("r3 seq = %d", v.Listing.Seq)
	}
	wantPkgRefusal(t, w.state(r2, "owner-a", pkgrelease.StateListed), http.StatusConflict, pkgrelease.ReasonTrackMoved)

	// A yanked release takes no owner act (the yank is the host's, S11).
	view := w.wantVisible(r3, "owner-a")
	yanked := view.Listing
	prev, _ := yanked.Digest()
	yanked.Version, yanked.PrevVersionDigest, yanked.State, yanked.StateReason, yanked.StateActor = yanked.Version+1, prev, pkgrelease.StateYanked, "advisory", "host:security:test"
	obj, err := listingObject("owner-a", "computer-a1", yanked, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{pkgrelease.StateListed, pkgrelease.StateWithdrawn} {
		wantPkgRefusal(t, w.state(r3, "owner-a", state), http.StatusConflict, pkgrelease.ReasonListingState)
	}
	// P2: a yanked release is not shown to another owner.
	if got := states("owner-b"); got[r3.id] != "" {
		t.Fatalf("P2 another owner sees a yanked release: %v", got)
	}
	// An owner act sets only listed or withdrawn.
	wantPkgRefusal(t, w.state(r1, "owner-a", pkgrelease.StateSuspended), http.StatusUnprocessableEntity, pkgrelease.ReasonInvalid)
}

func TestTwoTracksOfOnePackageStayApart(t *testing.T) {
	w := newPkgWorld(t)
	a1 := w.build("<p>a</p>\n", "owner-a", nil)
	w.published(a1)
	b1 := w.build("<p>b</p>\n", "owner-b", nil)
	w.published(b1)

	// L24: one catalog row per (package, track), each with its own head.
	rows := catalogRows(t, w.read(packagesRoute+"/catalog?owner_id=owner-b"))
	byTrack := map[string]PackageCatalogEntry{}
	for _, row := range rows {
		byTrack[row.Track] = row
	}
	if len(rows) != 2 || byTrack["owner-a"].HeadReleaseID != a1.id || byTrack["owner-b"].HeadReleaseID != b1.id ||
		byTrack["owner-a"].Releases != 1 || byTrack["owner-b"].Releases != 1 {
		t.Fatalf("L24 owner-b's catalog = %+v", rows)
	}
	// A history is one track: the reader's own by default.
	if page := history(t, w.read(packagesRoute+"/catalog/reader/releases?owner_id=owner-b")); page.Track != "owner-b" || len(page.Releases) != 1 || page.Releases[0].ReleaseID != b1.id {
		t.Fatalf("L24 owner-b's default history = %+v", page)
	}
	if page := history(t, w.read(packagesRoute+"/catalog/reader/releases?owner_id=owner-b&track=owner-a")); page.Track != "owner-a" || len(page.Releases) != 1 || page.Releases[0].ReleaseID != a1.id {
		t.Fatalf("L24 owner-b's history of owner-a's track = %+v", page)
	}
	// The track head a publish drafts on is the publisher's own.
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-b"); !strings.Contains(rec.Body.String(), b1.id) || strings.Contains(rec.Body.String(), a1.id) {
		t.Fatalf("L24 owner-b's track head = %s", rec.Body.String())
	}
	// Another owner's public release is still not a parent on owner-b's track.
	onShared := w.build("<p>b on a</p>\n", "owner-b", &a1)
	wantPkgRefusal(t, w.publish(onShared), http.StatusConflict, pkgrelease.ReasonTrackMoved)
	// And owner-b cannot act on owner-a's listing: it reads as missing.
	if rec := w.listingAct(a1, "owner-b", ListingChange{State: pkgrelease.StateWithdrawn}); rec.Code != http.StatusNotFound {
		t.Fatalf("L24 owner-b's act on owner-a's release = %d %s, want 404", rec.Code, rec.Body.String())
	}
}

func TestPackageListingActVerifiesTheOwnersReceipt(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := owneract.NewVerifier(public)
	if err != nil {
		t.Fatal(err)
	}
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	w.h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActEnforce, verifier))

	// A2: no receipt under enforce.
	if rec := w.state(r1, "owner-a", pkgrelease.StateWithdrawn); rec.Code != http.StatusForbidden {
		t.Fatalf("A2 a withdraw without a receipt = %d %s", rec.Code, rec.Body.String())
	}
	// A2: a receipt for another change.
	w.signer = signer
	body, _ := json.Marshal(PackageListingRequest{OwnerID: "owner-a", ComputerID: computerOf("owner-a"), ListingChange: ListingChange{
		State: pkgrelease.StateWithdrawn}})
	receipt, _ := signer.Mint(owneract.Act{Kind: owneract.ActPackageListing, OwnerUserID: "owner-a", AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session:test", ComputerID: computerOf("owner-a"), Subject: PackageListingSubject(r1.id, ListingChange{
			State: pkgrelease.StateListed})}, time.Now())
	encoded, _ := owneract.Encode(receipt)
	r := httptest.NewRequest(http.MethodPut, listingsRoute+"/"+r1.id, strings.NewReader(string(body)))
	r.Header.Set("X-Internal-Caller", "true")
	r.Header.Set(owneract.Header, encoded)
	rec := httptest.NewRecorder()
	w.h.HandleListings(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("A2 a receipt for another change = %d %s", rec.Code, rec.Body.String())
	}
	// A publish receipt is not a listing receipt.
	r = httptest.NewRequest(http.MethodPut, listingsRoute+"/"+r1.id, strings.NewReader(string(body)))
	r.Header.Set("X-Internal-Caller", "true")
	r.Header.Set(owneract.Header, w.receiptFor(r1))
	rec = httptest.NewRecorder()
	w.h.HandleListings(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("A2 a publish receipt on a listing act = %d %s", rec.Code, rec.Body.String())
	}
	// The owner's receipt: written, its actor the receipt.
	rec = w.state(r1, "owner-a", pkgrelease.StateWithdrawn)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "owner_act:unverified") {
		t.Fatalf("A2 the owner's withdraw = %d %s", rec.Code, rec.Body.String())
	}
	// Host only: a guest never reaches the route.
	r = httptest.NewRequest(http.MethodPut, listingsRoute+"/"+r1.id, strings.NewReader(string(body)))
	r.RemoteAddr = guestTap
	rec = httptest.NewRecorder()
	w.h.HandleListings(rec, r)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a guest's listing act = %d", rec.Code)
	}
	// No authority configured refuses (as publish does).
	w.h.ConfigurePackageOwnerActs(nil)
	wantPkgRefusal(t, w.state(r1, "owner-a", pkgrelease.StateListed), http.StatusServiceUnavailable, "owner_act_unconfigured")
}

// I2: reads page to the end of a track, and a publish past one page lists.
func TestPackageTrackPagesPastOnePage(t *testing.T) {
	saved := trackPageSize
	trackPageSize = 2
	t.Cleanup(func() { trackPageSize = saved })
	w := newPkgWorld(t)
	var prev *built
	var ids []string
	for i, content := range []string{"<p>v2</p>\n", "<p>v3</p>\n", "<p>v4</p>\n", "<p>v5</p>\n", "<p>v6</p>\n"} {
		b := w.build(content, "owner-a", prev)
		if v := w.published(b); v.Listing.Seq != i+1 {
			t.Fatalf("I2 release %d seq = %d", i+1, v.Listing.Seq)
		}
		ids = append(ids, b.id)
		prev = &b
	}
	// A state change adds versions past the page too.
	w.state(*prev, "owner-a", pkgrelease.StateWithdrawn)
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-a"); !strings.Contains(rec.Body.String(), ids[3]) {
		t.Fatalf("I2 head = %s, want the fourth release", rec.Body.String())
	}
	rows := catalogRows(t, w.read(packagesRoute+"/catalog?owner_id=owner-a"))
	if len(rows) != 1 || rows[0].Releases != 5 || rows[0].HeadReleaseID != ids[3] {
		t.Fatalf("I2 catalog = %+v", rows)
	}
	// The history pages newest first by before_seq.
	var seen []int
	before := 0
	for i := 0; i < 5; i++ {
		path := packagesRoute + "/catalog/reader/releases?owner_id=owner-a&limit=2"
		if before > 0 {
			path += "&before_seq=" + strconv.Itoa(before)
		}
		page := history(t, w.read(path))
		for _, view := range page.Releases {
			seen = append(seen, view.Listing.Seq)
		}
		if page.NextBeforeSeq == 0 {
			break
		}
		before = page.NextBeforeSeq
	}
	if got := shareJSON(t, seen); string(got) != "[5,4,3,2,1]" {
		t.Fatalf("I2 the paged history = %s, want [5,4,3,2,1]", got)
	}
	if page := history(t, w.read(packagesRoute+"/catalog/reader/releases?owner_id=owner-a")); len(page.Releases) != 5 || page.NextBeforeSeq != 0 {
		t.Fatalf("I2 the unpaged history = %d releases, next %d", len(page.Releases), page.NextBeforeSeq)
	}
}

// L33: a public listing version is an ordinary listing row; an older
// binary's generic routes list it, and the store opens with it.
func TestPublicListingRowsAreInert(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	w.state(r1, "owner-a", pkgrelease.StateWithdrawn)
	objects, err := w.h.releaseStore().ListObjects(context.Background(), objectgraph.ListFilter{Kind: ReleaseListingKind, OwnerID: "owner-a", Limit: 50})
	if err != nil || len(objects) != 2 {
		t.Fatalf("listing rows = %d %v", len(objects), err)
	}
	if err := w.h.service.Health(context.Background()); err != nil {
		t.Fatalf("store health beside widened rows: %v", err)
	}
}
