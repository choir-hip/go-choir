package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Updates and the platform's break-glass act at corpusd (F1), written
// before the code. Failure modes pinned here:
//
//	U1  the update read offers another track's release, a withdrawn one, or
//	    one at or below the installed seq;
//	U2  the soak is read from anything but the listing's own write time: a
//	    release listed 25 hours ago is not "soaked", or one listed now is;
//	U3  the update read tells an installer a withdrawn release's reason;
//	Y1  the yank route answers without a named operator's token, from a
//	    non-host transport, or with the gate closed;
//	Y2  a yank without a reason is written; a forced target that is not a
//	    listed release of the same track is written;
//	Y3  the yank is not a new listing version chained by its predecessor's
//	    digest, with state_actor platform:<operator>;
//	Y4  an owner's listing act can write yanked, or undo a yank;
//	F1  a publisher's force request changes a listing or an install: it is a
//	    note and executes nothing; another owner cannot write one.

// storeListedAt files b with a first listing at seq, written at at.
func (w *pkgWorld) storeListedAt(b built, seq int, at time.Time) {
	w.t.Helper()
	w.storeRecordOnly(b)
	listing := pkgrelease.Listing{
		RecordKind: pkgrelease.RecordKindPackageRelease, RecordID: b.id, Package: b.record.Package.Name, Track: b.record.Track,
		Seq: seq, Scopes: []string{pkgrelease.ScopeOwner, pkgrelease.ScopePublic}, State: pkgrelease.StateListed, StateReason: "published", StateActor: "owner_act:test", Version: 1,
	}
	obj, err := listingObject(b.record.Publisher.OwnerID, b.record.Publisher.ComputerID, listing, at)
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		w.t.Fatal(err)
	}
}

func (w *pkgWorld) updates(owner string, installs ...PackageUpdateInstall) []PackageUpdateFacts {
	w.t.Helper()
	body, _ := json.Marshal(PackageUpdatesRequest{OwnerID: owner, Installs: installs})
	rec := w.do(pkgCall{method: http.MethodPost, path: packagesRoute + "/updates", internal: true, body: body})
	if rec.Code != http.StatusOK {
		w.t.Fatalf("updates = %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Updates []PackageUpdateFacts `json:"updates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		w.t.Fatal(err)
	}
	return out.Updates
}

func (w *pkgWorld) yank(operator, token string, req PlatformYankRequest) *httptest.ResponseRecorder {
	w.t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, platformOperatorYankRoute, strings.NewReader(string(body)))
	if operator != "" {
		r.Header.Set("X-Choir-Operator", operator)
		r.Header.Set("X-Choir-Operator-Token", token)
	}
	rec := httptest.NewRecorder()
	w.h.HandlePlatformYank(rec, r)
	return rec
}

func TestUpdateReadOffersTheSameTracksNewerListedReleasesWithTheSoak(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>one</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>two</p>\n", "owner-a", &r1)
	w.storeListedAt(r2, 2, time.Now().UTC().Add(-25*time.Hour))
	r3 := w.build("<p>three</p>\n", "owner-a", &r2)
	w.published(r3)
	other := w.build("<p>other track</p>\n", "owner-b", nil)
	w.published(other)

	facts := w.updates("owner-c", PackageUpdateInstall{Package: "reader", Track: "owner-a", ReleaseID: r1.id})
	if len(facts) != 1 || facts[0].Installed == nil || facts[0].Installed.State != pkgrelease.StateListed {
		t.Fatalf("U1 facts = %+v", facts)
	}
	f := facts[0]
	if f.Newest == nil || f.Newest.Release.ReleaseID != r3.id {
		t.Fatalf("U1 newest = %+v, want r3", f.Newest)
	}
	if f.Soaked == nil || f.Soaked.Release.ReleaseID != r2.id {
		t.Fatalf("U2 soaked = %+v, want r2 (listed 25h ago)", f.Soaked)
	}
	if f.Newest.Release.Soaked(time.Now()) {
		t.Fatal("U2 r3, listed now, reads as soaked")
	}
	if f.Forced != nil {
		t.Fatalf("no platform act, forced = %+v", f.Forced)
	}
	// Installed at the head: nothing newer.
	if top := w.updates("owner-c", PackageUpdateInstall{Package: "reader", Track: "owner-a", ReleaseID: r3.id}); top[0].Newest != nil || top[0].Soaked != nil {
		t.Fatalf("U1 at the head = %+v", top[0])
	}
	// U3: a withdrawn installed release reads as unavailable, without its reason.
	if rec := w.state(r1, "owner-a", pkgrelease.StateWithdrawn); rec.Code != http.StatusOK {
		t.Fatalf("withdraw = %d %s", rec.Code, rec.Body.String())
	}
	if got := w.updates("owner-c", PackageUpdateInstall{Package: "reader", Track: "owner-a", ReleaseID: r1.id}); got[0].Installed.State != "unavailable" || got[0].Installed.StateReason != "" {
		t.Fatalf("U3 withdrawn = %+v", got[0].Installed)
	}
}

func TestPlatformYankIsTheOperatorsActAlone(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>one</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>two</p>\n", "owner-a", &r1)
	w.published(r2)
	other := w.build("<p>b</p>\n", "owner-b", nil)
	w.published(other)

	// Y1: closed gate, then wrong token, then a guest transport.
	if rec := w.yank("ops", "secret", PlatformYankRequest{ReleaseID: r1.id, Reason: "x"}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Y1 closed gate = %d", rec.Code)
	}
	w.h.ConfigurePlatformOperators("ops:secret")
	if rec := w.yank("ops", "wrong", PlatformYankRequest{ReleaseID: r1.id, Reason: "x"}); rec.Code != http.StatusForbidden {
		t.Fatalf("Y1 wrong token = %d", rec.Code)
	}
	if rec := w.yank("", "", PlatformYankRequest{ReleaseID: r1.id, Reason: "x"}); rec.Code != http.StatusForbidden {
		t.Fatalf("Y1 no operator = %d", rec.Code)
	}
	body, _ := json.Marshal(PlatformYankRequest{ReleaseID: r1.id, Reason: "x"})
	guest := httptest.NewRequest(http.MethodPost, platformOperatorYankRoute, strings.NewReader(string(body)))
	guest.RemoteAddr = "10.0.0.7:4000"
	guest.Header.Set("X-Choir-Operator", "ops")
	guest.Header.Set("X-Choir-Operator-Token", "secret")
	grec := httptest.NewRecorder()
	w.h.HandlePlatformYank(grec, guest)
	if grec.Code != http.StatusForbidden {
		t.Fatalf("Y1 guest transport = %d", grec.Code)
	}
	// Y2: a reason is required; the forced target is a listed release of the same track.
	if rec := w.yank("ops", "secret", PlatformYankRequest{ReleaseID: r1.id}); rec.Code != http.StatusBadRequest {
		t.Fatalf("Y2 no reason = %d", rec.Code)
	}
	if rec := w.yank("ops", "secret", PlatformYankRequest{ReleaseID: r1.id, Reason: "cve", ForcedRelease: other.id}); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("Y2 another track's target = %d %s", rec.Code, rec.Body.String())
	}
	// Y3: the forced update is a new chained version.
	rec := w.yank("ops", "secret", PlatformYankRequest{ReleaseID: r1.id, Reason: "CVE-1: the reader leaks", ForcedRelease: r2.id})
	if rec.Code != http.StatusOK {
		t.Fatalf("yank = %d %s", rec.Code, rec.Body.String())
	}
	chains, _, err := w.h.trackChains(context.Background(), "owner-a", "reader")
	if err != nil {
		t.Fatal(err)
	}
	chain := chains[r1.id]
	if len(chain) != 2 {
		t.Fatalf("Y3 chain = %+v", chain)
	}
	prev, _ := chain[0].Digest()
	last := chain[1]
	if last.State != pkgrelease.StateYanked || last.StateActor != "platform:ops" || last.ForcedRelease != r2.id || last.PrevVersionDigest != prev || last.StateReason != "CVE-1: the reader leaks" {
		t.Fatalf("Y3 yank version = %+v", last)
	}
	facts := w.updates("owner-c", PackageUpdateInstall{Package: "reader", Track: "owner-a", ReleaseID: r1.id})
	if facts[0].Installed.State != pkgrelease.StateYanked || facts[0].Installed.StateReason == "" || facts[0].Forced == nil || facts[0].Forced.Release.ReleaseID != r2.id {
		t.Fatalf("the installer reads the forced update: %+v", facts[0])
	}
	// Y4: the owner cannot reinstate a yanked release.
	if rec := w.state(r1, "owner-a", pkgrelease.StateListed); rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), `"changed":true`) {
		t.Fatalf("Y4 owner reinstated a yank: %s", rec.Body.String())
	}
	if rec := w.listingAct(r2, "owner-a", ListingChange{State: pkgrelease.StateYanked}); rec.Code == http.StatusOK {
		t.Fatalf("Y4 an owner act wrote yanked: %s", rec.Body.String())
	}
}

func TestPublisherForceRequestIsANoteAndExecutesNothing(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>one</p>\n", "owner-a", nil)
	w.published(r1)
	before, _, err := w.h.trackChains(context.Background(), "owner-a", "reader")
	if err != nil {
		t.Fatal(err)
	}
	send := func(owner string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(PackageForceRequest{OwnerID: owner, ComputerID: computerOf(owner), Note: "please force everyone onto my fix"})
		return w.do(pkgCall{method: http.MethodPost, path: packagesRoute + "/releases/" + r1.id + "/force-requests", internal: true, body: body})
	}
	if rec := send("owner-b"); rec.Code != http.StatusNotFound {
		t.Fatalf("F1 another owner's request = %d", rec.Code)
	}
	rec := send("owner-a")
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), `"executed":false`) {
		t.Fatalf("F1 request = %d %s", rec.Code, rec.Body.String())
	}
	after, _, err := w.h.trackChains(context.Background(), "owner-a", "reader")
	if err != nil {
		t.Fatal(err)
	}
	if len(after[r1.id]) != len(before[r1.id]) || after[r1.id][len(after[r1.id])-1].State != pkgrelease.StateListed {
		t.Fatalf("F1 the request changed the listing: %+v", after[r1.id])
	}
	notes, err := w.h.releaseStore().ListObjectsByMetadataPage(context.Background(), string(PackageForceRequestKind), "$.release_id", r1.id, "owner-a", "", objectgraph.NormalizedLimit(10))
	if err != nil || len(notes) != 1 {
		t.Fatalf("F1 note = %d %v", len(notes), err)
	}
}
