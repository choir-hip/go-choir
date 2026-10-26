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

// Library S3 at corpusd (docs/design/library-package-propagation-2026-10-26.md
// 4.1, 4.2, 4.4, 5, 15.1, 16, 17 S3), written before the code. Failure modes
// pinned here:
//
//	L4   across every scope, a release is visible to a reader its current
//	     listing does not name: a single read must answer 404 with the body
//	     of a missing release, and a list (catalog, history) 200 with nothing;
//	     a narrowing must take the release away again; a sharee must not
//	     learn the other sharees; the host switch CHOIR_PACKAGE_SHARE_READS=
//	     owner must bring every read back to the owner's own;
//	L21  private data leaves in a widening: a widening without
//	     --confirm-paths, or confirmed for other paths, is written; the
//	     refusal does not show the full path list and the review diff size;
//	     a narrowing asks for a confirmation; the secret scan does not run
//	     again at widening; a publisher screenshot, capture or evidence
//	     reference travels in a listing;
//	L24  two owners' tracks of one package mix: a catalog row, a history or
//	     a track head answers from both, or a publish takes another owner's
//	     shared release as its parent;
//	H1   the history: a withdrawn release leaves the catalog heads and the
//	     track head but stays in the history as withdrawn; reinstate brings
//	     it back while it can still head the track, and refuses track_moved
//	     once the track moved past it; a yanked release takes no owner act;
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

// preview asks for a widening without a confirmation and returns the
// preview the refusal carries.
func (w *pkgWorld) preview(b built, owner string, change ListingChange) pkgrelease.Preview {
	w.t.Helper()
	change.ConfirmPaths = ""
	rec := w.listingAct(b, owner, change)
	var got PackageListingRefusal
	if rec.Code != http.StatusConflict || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Reason != pkgrelease.ReasonConfirmPathsRequired || got.Preview == nil {
		w.t.Fatalf("L21 an unconfirmed widening = %d %s, want 409 confirm_paths_required with a preview", rec.Code, rec.Body.String())
	}
	return *got.Preview
}

// share widens or narrows b's listing, confirming the paths when it widens.
func (w *pkgWorld) share(b built, owner string, scopes []string, owners ...string) PackageListingResult {
	w.t.Helper()
	change := ListingChange{Scopes: scopes, SharedWith: &pkgrelease.SharedWith{Owners: owners}}
	rec := w.listingAct(b, owner, change)
	if rec.Code == http.StatusConflict && strings.Contains(rec.Body.String(), pkgrelease.ReasonConfirmPathsRequired) {
		change.ConfirmPaths = w.preview(b, owner, change).PathsDigest
		rec = w.listingAct(b, owner, change)
	}
	var got PackageListingResult
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		w.t.Fatalf("share %v %v = %d %s", scopes, owners, rec.Code, rec.Body.String())
	}
	return got
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
		w.t.Fatalf("L4 %s's read of a release shared with it = %d %s", reader, rec.Code, rec.Body.String())
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

func TestPackageShareScopesFollowL4AcrossEveryScope(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)

	// owner: only owner-a.
	w.wantInvisible(r1, "owner-b")
	w.wantInvisible(r1, "owner-c")

	// shared with owner-b and owner-d: b and d see it, c does not.
	result := w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b", "owner-d")
	if !result.Changed || result.Listing.Version != 2 || result.Listing.Seq != 1 || result.Listing.PrevVersionDigest == "" {
		t.Fatalf("a share is a new listing version on the same seq: %+v", result.Listing)
	}
	view := w.wantVisible(r1, "owner-b")
	w.wantVisible(r1, "owner-d")
	w.wantInvisible(r1, "owner-c")
	// A sharee does not learn the other sharees.
	if strings.Contains(string(shareJSON(t, view)), "owner-d") {
		t.Fatalf("L4 owner-b's view names another sharee: %s", shareJSON(t, view))
	}
	// The owner still sees every sharee.
	if owner := w.wantVisible(r1, "owner-a"); strings.Join(owner.Listing.SharedWith.Owners, ",") != "owner-b,owner-d" {
		t.Fatalf("the owner's view of its listing = %+v", owner.Listing)
	}

	// public: every owner.
	w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic})
	w.wantVisible(r1, "owner-c")
	w.wantVisible(r1, "owner-b")

	// The host switch: CHOIR_PACKAGE_SHARE_READS=owner reads at owner scope.
	w.h.SetPackageShareReads(ParsePackageShareReads("owner"))
	w.wantInvisible(r1, "owner-c")
	w.wantVisible(r1, "owner-a")
	w.h.SetPackageShareReads(ParsePackageShareReads(""))
	w.wantVisible(r1, "owner-c")

	// Narrowed back to owner: every other reader loses it.
	if got := w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner}); !got.Changed || got.Preview != nil {
		t.Fatalf("a narrowing = %+v, want a version and no preview", got)
	}
	w.wantInvisible(r1, "owner-b")
	w.wantInvisible(r1, "owner-c")

	// S3 cannot enforce the platform scope or named computers: refused by
	// name, nothing written.
	for _, change := range []ListingChange{
		{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePlatform}},
		{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, SharedWith: &pkgrelease.SharedWith{Computers: []string{"computer-b1"}}},
	} {
		wantPkgRefusal(t, w.listingAct(r1, "owner-a", change), http.StatusUnprocessableEntity, pkgrelease.ReasonScopeNotBuilt)
	}
	if v := w.wantVisible(r1, "owner-a"); v.Listing.Version != 4 {
		t.Fatalf("a refused act wrote a version: %+v", v.Listing)
	}
}

func shareJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPackageWideningNeedsConfirmedPaths(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	change := ListingChange{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, SharedWith: &pkgrelease.SharedWith{Owners: []string{"owner-b"}}}

	// L21: the refusal shows what would leave: every path and the review
	// diff's size.
	preview := w.preview(r1, "owner-a", change)
	paths := strings.Join(preview.Paths, "\n")
	for _, want := range []string{pkgRoot + "/App.svelte", pkgRoot + "/choir-package.json"} {
		if !strings.Contains(paths, want) {
			t.Fatalf("L21 the preview omits %s: %v", want, preview.Paths)
		}
	}
	if preview.ReviewPatchBytes != r1.record.Source.ReviewPatch.Bytes {
		t.Fatalf("L21 the preview's review size = %d, want %d", preview.ReviewPatchBytes, r1.record.Source.ReviewPatch.Bytes)
	}
	// Confirmed for other paths: refused, nothing written.
	change.ConfirmPaths = strings.Repeat("f", 64)
	wantPkgRefusal(t, w.listingAct(r1, "owner-a", change), http.StatusConflict, pkgrelease.ReasonConfirmPathsMismatch)
	w.wantInvisible(r1, "owner-b")

	// Confirmed: written.
	change.ConfirmPaths = preview.PathsDigest
	if rec := w.listingAct(r1, "owner-a", change); rec.Code != http.StatusOK {
		t.Fatalf("a confirmed widening = %d %s", rec.Code, rec.Body.String())
	}
	view := w.wantVisible(r1, "owner-b")
	// L21: no publisher screenshot, capture or evidence travels.
	for _, raw := range [][]byte{shareJSON(t, view), shareJSON(t, preview)} {
		lower := strings.ToLower(string(raw))
		for _, banned := range []string{"screenshot", "capture", "evidence", "visual"} {
			if strings.Contains(lower, banned) {
				t.Fatalf("L21 a listing view carries %q: %s", banned, raw)
			}
		}
	}
	// Repeating the same act writes nothing new.
	again := w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b")
	if again.Changed || again.Listing.Version != 2 {
		t.Fatalf("an unchanged share wrote a version: %+v", again)
	}
	// A wider set (one more owner) is a widening again.
	change.SharedWith = &pkgrelease.SharedWith{Owners: []string{"owner-b", "owner-c"}}
	change.ConfirmPaths = ""
	wantPkgRefusal(t, w.listingAct(r1, "owner-a", change), http.StatusConflict, pkgrelease.ReasonConfirmPathsRequired)
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

func TestPackageWideningRunsTheSecretScanAgain(t *testing.T) {
	w := newPkgWorld(t)
	secret := w.build("GITHUB_TOKEN=ghp_"+strings.Repeat("A", 36)+"\n", "owner-a", nil)
	w.storeUnchecked(secret)
	change := ListingChange{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}, ConfirmPaths: strings.Repeat("0", 64)}
	wantPkgRefusal(t, w.listingAct(secret, "owner-a", change), http.StatusUnprocessableEntity, pkgrelease.ReasonSecretFound)
	w.wantInvisible(secret, "owner-c")
	// A withdraw needs no scan: the owner can always pull a release back.
	if rec := w.state(secret, "owner-a", pkgrelease.StateWithdrawn); rec.Code != http.StatusOK {
		t.Fatalf("withdrawing a release that fails the scan = %d %s", rec.Code, rec.Body.String())
	}
}

func TestPackageWithdrawLeavesTheHeadsAndReinstateReturnsIt(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	w.published(r2)
	w.share(r2, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b")
	w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b")

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
		withdrawn.Listing.Version != 3 || !strings.HasPrefix(withdrawn.Listing.StateActor, "owner_act:") {
		t.Fatalf("withdraw = %d %s", rec.Code, rec.Body.String())
	}
	for _, reader := range []string{"owner-a", "owner-b"} {
		if got := head(reader); got != r1.id {
			t.Fatalf("H1 %s's catalog head after a withdraw = %s, want r1", reader, got)
		}
		if got := states(reader); got[r2.id] != pkgrelease.StateWithdrawn || got[r1.id] != pkgrelease.StateListed {
			t.Fatalf("H1 %s's history after a withdraw = %v", reader, got)
		}
	}
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-a"); !strings.Contains(rec.Body.String(), r1.id) {
		t.Fatalf("H1 the track head after a withdraw = %s, want r1", rec.Body.String())
	}
	// Withdrawing again writes nothing; a withdrawn release is not widened.
	if rec := w.state(r2, "owner-a", pkgrelease.StateWithdrawn); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"changed":true`) {
		t.Fatalf("a second withdraw = %d %s", rec.Code, rec.Body.String())
	}
	wantPkgRefusal(t, w.listingAct(r2, "owner-a", ListingChange{Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}, ConfirmPaths: strings.Repeat("0", 64)}),
		http.StatusConflict, pkgrelease.ReasonListingState)

	// H1: reinstated while nothing was published past it, r2 heads again.
	if rec := w.state(r2, "owner-a", pkgrelease.StateListed); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"state":"listed"`) {
		t.Fatalf("reinstate = %d %s", rec.Code, rec.Body.String())
	}
	if got := head("owner-b"); got != r2.id {
		t.Fatalf("H1 the head after a reinstate = %s, want r2", got)
	}
	if got := states("owner-b"); got[r2.id] != pkgrelease.StateListed {
		t.Fatalf("H1 the history after a reinstate = %v", got)
	}

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
	// An owner act sets only listed or withdrawn.
	wantPkgRefusal(t, w.state(r1, "owner-a", pkgrelease.StateSuspended), http.StatusUnprocessableEntity, pkgrelease.ReasonInvalid)
}

func TestTwoTracksOfOnePackageStayApart(t *testing.T) {
	w := newPkgWorld(t)
	a1 := w.build("<p>a</p>\n", "owner-a", nil)
	w.published(a1)
	b1 := w.build("<p>b</p>\n", "owner-b", nil)
	w.published(b1)
	w.share(a1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, "owner-b")

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
	// A release shared with owner-b is still not a parent on owner-b's track.
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
		Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}, ConfirmPaths: strings.Repeat("0", 64)}})
	receipt, _ := signer.Mint(owneract.Act{Kind: owneract.ActPackageListing, OwnerUserID: "owner-a", AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session:test", ComputerID: computerOf("owner-a"), Subject: PackageListingSubject(r1.id, ListingChange{
			Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopeShared}, SharedWith: &pkgrelease.SharedWith{Owners: []string{"owner-b"}}})}, time.Now())
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

// L33 for S3: a widened listing version is an ordinary listing row; an
// older binary's generic routes list it, and the store opens with it.
func TestWidenedListingRowsAreInert(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	w.share(r1, "owner-a", []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic})
	objects, err := w.h.releaseStore().ListObjects(context.Background(), objectgraph.ListFilter{Kind: ReleaseListingKind, OwnerID: "owner-a", Limit: 50})
	if err != nil || len(objects) != 2 {
		t.Fatalf("listing rows = %d %v", len(objects), err)
	}
	if err := w.h.service.Health(context.Background()); err != nil {
		t.Fatalf("store health beside widened rows: %v", err)
	}
}
