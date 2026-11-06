package platform

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
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

// The operator account's yank (docs/design/platform-operator-account-2026-11-06.md),
// written before the code. Failure modes pinned here:
//
//	O1  the route answers a receipt while no account holds the role;
//	O2  a receipt for an account outside CHOIR_PLATFORM_OPERATOR_USERS writes;
//	O3  a receipt that binds another release, target or reason writes;
//	O4  a receipt for a computer act, or from an API key, writes;
//	O5  a request carries a receipt and a token and one of them wins;
//	O6  a receipt from a guest transport writes;
//	O7  the act writes without its receipt recorded first, or the same
//	    receipt acts twice;
//	O8  observe mode (CHOIR_OWNER_ACT_MODE unset) admits an unverified act;
//	O9  the roster parses a malformed or upper-case id, or too many.

const testOperatorUser = "1111aaaa-2222-3333-4444-55555555beef"

func (w *pkgWorld) operatorReceipt(user, computerID, method string, req PlatformYankRequest) string {
	w.t.Helper()
	receipt, err := w.signer.Mint(owneract.Act{
		Kind: owneract.ActPlatformYank, OwnerUserID: user, AuthMethod: method,
		CredentialRef: "session-jwt-sha256:test", ComputerID: computerID, Subject: PlatformYankSubject(req),
	}, time.Now())
	if err != nil {
		w.t.Fatal(err)
	}
	encoded, err := owneract.Encode(receipt)
	if err != nil {
		w.t.Fatal(err)
	}
	return encoded
}

func (w *pkgWorld) yankByReceipt(receipt string, req PlatformYankRequest, mutate func(*http.Request)) *httptest.ResponseRecorder {
	w.t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, platformOperatorYankRoute, strings.NewReader(string(body)))
	r.Header.Set(owneract.Header, receipt)
	if mutate != nil {
		mutate(r)
	}
	rec := httptest.NewRecorder()
	w.h.HandlePlatformYank(rec, r)
	return rec
}

func TestPlatformYankByOperatorAccountSession(t *testing.T) {
	w := newPkgWorld(t)
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	w.signer = signer
	verifier, err := owneract.NewVerifier(signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	// O8: observe, the default on Node B; the route verifies anyway.
	w.h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActObserve, verifier))
	r1 := w.build("<p>one</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>two</p>\n", "owner-a", &r1)
	w.published(r2)
	req := PlatformYankRequest{ReleaseID: r1.id, Reason: "CVE-2: the reader leaks", ForcedRelease: r2.id}
	good := func() string {
		return w.operatorReceipt(testOperatorUser, owneract.PlatformComputerID, owneract.AuthMethodSession, req)
	}
	chainLen := func() int {
		chains, _, err := w.h.trackChains(context.Background(), "owner-a", "reader")
		if err != nil {
			t.Fatal(err)
		}
		return len(chains[r1.id])
	}

	// O1: nobody holds the role and no token is configured.
	if rec := w.yankByReceipt(good(), req, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("O1 closed = %d %s", rec.Code, rec.Body.String())
	}
	if n, err := w.h.ConfigurePlatformOperatorUsers(testOperatorUser); err != nil || n != 1 {
		t.Fatalf("configure = %d %v", n, err)
	}
	// O2: another account.
	other := w.operatorReceipt("99999999-2222-3333-4444-555555555555", owneract.PlatformComputerID, owneract.AuthMethodSession, req)
	if rec := w.yankByReceipt(other, req, nil); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "not a platform operator") {
		t.Fatalf("O2 non-operator = %d %s", rec.Code, rec.Body.String())
	}
	// O3: the receipt binds another reason, another target.
	if rec := w.yankByReceipt(good(), PlatformYankRequest{ReleaseID: r1.id, Reason: "something else", ForcedRelease: r2.id}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("O3 other reason = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.yankByReceipt(good(), PlatformYankRequest{ReleaseID: r1.id, Reason: req.Reason}, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("O3 dropped target = %d %s", rec.Code, rec.Body.String())
	}
	// O4: a computer act's receipt; an API key's receipt.
	if rec := w.yankByReceipt(w.operatorReceipt(testOperatorUser, computerOf("owner-a"), owneract.AuthMethodSession, req), req, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("O4 computer receipt = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.yankByReceipt(w.operatorReceipt(testOperatorUser, owneract.PlatformComputerID, owneract.AuthMethodAPIKey, req), req, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("O4 api key receipt = %d %s", rec.Code, rec.Body.String())
	}
	// O5: both credentials.
	w.h.ConfigurePlatformOperators("ops:secret")
	both := func(r *http.Request) {
		r.Header.Set("X-Choir-Operator", "ops")
		r.Header.Set("X-Choir-Operator-Token", "secret")
	}
	if rec := w.yankByReceipt(good(), req, both); rec.Code != http.StatusBadRequest {
		t.Fatalf("O5 both credentials = %d %s", rec.Code, rec.Body.String())
	}
	// O6: a guest transport.
	if rec := w.yankByReceipt(good(), req, func(r *http.Request) { r.RemoteAddr = "10.0.0.7:4000" }); rec.Code != http.StatusForbidden {
		t.Fatalf("O6 guest transport = %d %s", rec.Code, rec.Body.String())
	}
	if n := chainLen(); n != 1 {
		t.Fatalf("a refused act wrote: chain length %d", n)
	}
	var refused int
	if err := w.h.service.store.db.QueryRow(`SELECT COUNT(*) FROM owner_act_receipts WHERE act_kind=?`, owneract.ActPlatformYank).Scan(&refused); err != nil || refused != 0 {
		t.Fatalf("refused acts recorded: %d %v", refused, err)
	}

	// The act.
	receipt := good()
	rec := w.yankByReceipt(receipt, req, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("operator yank = %d %s", rec.Code, rec.Body.String())
	}
	chains, _, err := w.h.trackChains(context.Background(), "owner-a", "reader")
	if err != nil {
		t.Fatal(err)
	}
	last := chains[r1.id][len(chains[r1.id])-1]
	if last.State != pkgrelease.StateYanked || last.StateActor != "platform:user:"+testOperatorUser || last.ForcedRelease != r2.id {
		t.Fatalf("operator yank version = %+v", last)
	}
	// O7: the receipt is on record with the version it left, once.
	want, _ := last.Digest()
	var owner, computer, bound string
	if err := w.h.service.store.db.QueryRow(`SELECT owner_user_id, computer_id, mode_receipt_digest FROM owner_act_receipts WHERE act_kind=?`, owneract.ActPlatformYank).Scan(&owner, &computer, &bound); err != nil {
		t.Fatalf("O7 receipt record: %v", err)
	}
	if owner != testOperatorUser || computer != owneract.PlatformComputerID || bound != want {
		t.Fatalf("O7 receipt record = %s %s %s, want listing %s", owner, computer, bound, want)
	}
	if rec := w.yankByReceipt(receipt, req, nil); rec.Code != http.StatusConflict {
		t.Fatalf("O7 replay = %d %s", rec.Code, rec.Body.String())
	}
	// A fresh receipt for the same act changes nothing and is recorded.
	if rec := w.yankByReceipt(good(), req, nil); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"changed":true`) {
		t.Fatalf("repeat act = %d %s", rec.Code, rec.Body.String())
	}
	// The break-glass token path is unchanged.
	if rec := w.yank("ops", "secret", PlatformYankRequest{ReleaseID: r2.id, Reason: "break glass"}); rec.Code != http.StatusOK {
		t.Fatalf("token path = %d %s", rec.Code, rec.Body.String())
	}
	// No verifier key: the receipt path refuses, the token path still works.
	w.h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActEnforce, nil))
	if rec := w.yankByReceipt(good(), req, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no verifier = %d %s", rec.Code, rec.Body.String())
	}
}

func TestParsePlatformOperatorUsers(t *testing.T) {
	for _, raw := range []string{"", "none", " "} {
		if users, err := ParsePlatformOperatorUsers(raw); err != nil || len(users) != 0 {
			t.Fatalf("%q = %v %v", raw, users, err)
		}
	}
	users, err := ParsePlatformOperatorUsers(testOperatorUser + "," + "99999999-2222-3333-4444-555555555555")
	if err != nil || len(users) != 2 || !users[testOperatorUser] {
		t.Fatalf("two ids = %v %v", users, err)
	}
	many := []string{}
	for i := 0; i < 9; i++ {
		many = append(many, fmt.Sprintf("%08d-2222-3333-4444-555555555555", i))
	}
	for _, raw := range []string{
		strings.ToUpper(testOperatorUser), "owner@example.com", testOperatorUser + ",", "x", strings.Join(many, ","),
	} {
		if users, err := ParsePlatformOperatorUsers(raw); err == nil || len(users) != 0 {
			t.Fatalf("O9 %q = %v %v", raw, users, err)
		}
	}
}
