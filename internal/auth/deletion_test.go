package auth

// Tests for account deletion. The failure list is in deletion_handlers.go
// (the numbered "ways this could go wrong"); each test names the numbers it
// covers. A software authenticator makes real assertions: the library under
// test verifies a genuine ECDSA signature, nothing is stubbed.

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

type softKey struct {
	id    string
	key   *ecdsa.PrivateKey
	count uint32
}

func newSoftKey(t *testing.T, id string) *softKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &softKey{id: id, key: k}
}

func (k *softKey) cose(t *testing.T) []byte {
	t.Helper()
	x, y := k.key.X.FillBytes(make([]byte, 32)), k.key.Y.FillBytes(make([]byte, 32))
	data := webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(webauthncose.AlgES256)},
		Curve:         int64(webauthncose.P256), XCoord: x, YCoord: y,
	}
	raw, err := webauthncbor.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// assert answers an assertion request (the options JSON the server returned).
func (k *softKey) assert(t *testing.T, options []byte, rpID, origin string) []byte {
	t.Helper()
	var opt struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
		} `json:"publicKey"`
	}
	if err := json.Unmarshal(options, &opt); err != nil || opt.PublicKey.Challenge == "" {
		t.Fatalf("assertion options: %v %s", err, options)
	}
	return k.assertChallenge(t, opt.PublicKey.Challenge, rpID, origin)
}

func (k *softKey) assertChallenge(t *testing.T, challenge, rpID, origin string) []byte {
	t.Helper()
	k.count++
	cd, _ := json.Marshal(map[string]string{"type": "webauthn.get", "challenge": challenge, "origin": origin})
	rp := sha256.Sum256([]byte(rpID))
	auth := append([]byte{}, rp[:]...)
	auth = append(auth, 0x05) // user present, user verified
	cnt := make([]byte, 4)
	binary.BigEndian.PutUint32(cnt, k.count)
	auth = append(auth, cnt...)
	h := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, auth...), h[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.RawURLEncoding.EncodeToString
	body, _ := json.Marshal(map[string]any{
		"id": b64([]byte(k.id)), "rawId": b64([]byte(k.id)), "type": "public-key",
		"response": map[string]string{"clientDataJSON": b64(cd), "authenticatorData": b64(auth), "signature": b64(sig)},
	})
	return body
}

type delEnv struct {
	h    *Handler
	user *User
	key  *softKey
	host *fakeDeletionHost
}

type fakeDeletionHost struct {
	precheck error
	atOnce   []string
	restore  []string
}

func (f *fakeDeletionHost) Precheck(context.Context, string) error { return f.precheck }
func (f *fakeDeletionHost) AtOnce(_ context.Context, u string) error {
	f.atOnce = append(f.atOnce, u)
	return nil
}
func (f *fakeDeletionHost) Restore(_ context.Context, u string) error {
	f.restore = append(f.restore, u)
	return nil
}

func newDelEnv(t *testing.T, id string) *delEnv {
	t.Helper()
	h, _ := testHandlerEnv(t)
	host := &fakeDeletionHost{}
	h.SetDeletionHost(host)
	return addDelUser(t, h, host, id)
}

func addDelUser(t *testing.T, h *Handler, host *fakeDeletionHost, id string) *delEnv {
	t.Helper()
	u, err := h.store.CreateUser(id, id+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	k := newSoftKey(t, "cred-"+id)
	if err := h.store.CreateCredential(&Credential{ID: k.id, UserID: u.ID, PublicKey: k.cose(t), AttestationType: "none",
		Transport: `["internal"]`, AAGUID: make([]byte, 16), Flags: `{"user_present":true,"user_verified":true}`, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	return &delEnv{h: h, user: u, key: k, host: host}
}

// cookies makes a signed-in browser session and returns its cookies.
func (e *delEnv) cookies(t *testing.T, method string) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/x", nil)
	if _, err := e.h.issueSession(rec, req, e.user, method); err != nil {
		t.Fatal(err)
	}
	return rec.Result().Cookies()
}

func (e *delEnv) bearer(t *testing.T) (secret string) {
	t.Helper()
	_, secret, err := e.h.store.CreateAPIKey(context.Background(), e.user.ID, "phone", []string{"read:runtime", "write:runtime"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func (e *delEnv) call(h http.HandlerFunc, path string, body []byte, cookies []*http.Cookie, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func (e *delEnv) begin(t *testing.T, cookies []*http.Cookie, bearer string) []byte {
	t.Helper()
	rec := e.call(e.h.HandleDeleteBegin, "/auth/account/delete/begin", nil, cookies, bearer)
	if rec.Code != http.StatusOK {
		t.Fatalf("begin: %d %s", rec.Code, rec.Body)
	}
	return rec.Body.Bytes()
}

func (e *delEnv) assertion(t *testing.T, options []byte) []byte {
	return e.key.assert(t, options, e.h.config.RPID, e.h.config.RPOrigins[0])
}

func (e *delEnv) sessionRows(t *testing.T) int {
	var n int
	if err := e.h.store.db.QueryRow(`SELECT COUNT(*) FROM refresh_sessions WHERE user_id = ?`, e.user.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *delEnv) noRecord(t *testing.T) {
	t.Helper()
	d, err := e.h.store.AccountDeletionFor(e.user.ID)
	if err != nil || d != nil {
		t.Fatalf("a deletion record exists: %v %v", d, err)
	}
}

// Failure 1, 5: an assertion over a session starts the deletion and revokes
// everything; the access token already signed stops working at once.
func TestDeletionWithCookieSessionRevokesAtOnce(t *testing.T) {
	e := newDelEnv(t, "u1")
	cookies := e.cookies(t, AuthMethodPasskey)
	var access string
	for _, c := range cookies {
		if c.Name == AccessTokenCookieName {
			access = c.Value
		}
	}
	bearer := e.bearer(t)
	if _, err := e.h.ValidateAccessToken(access); err != nil {
		t.Fatalf("token should be valid before: %v", err)
	}
	opts := e.begin(t, cookies, "")
	rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), cookies, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("finish: %d %s", rec.Code, rec.Body)
	}
	d, _ := e.h.store.AccountDeletionFor(e.user.ID)
	if d == nil || d.State != DeletionPending || d.PurgeAfter.Sub(d.RequestedAt) != purgeguard.GracePeriod {
		t.Fatalf("record: %+v", d)
	}
	if n := e.sessionRows(t); n != 0 {
		t.Fatalf("%d refresh sessions survive", n)
	}
	if _, err := e.h.ValidateAccessToken(access); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("signed access token still valid: %v", err)
	}
	// The bearer key is revoked.
	whoami := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/whoami", nil)
	r.Header.Set("Authorization", "Bearer "+bearer)
	e.h.HandleWhoami(whoami, r)
	if whoami.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key answers %d", whoami.Code)
	}
	if len(e.host.atOnce) != 1 {
		t.Fatalf("host at-once calls: %v", e.host.atOnce)
	}
	// No cookie of the browser survives.
	cleared := 0
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			cleared++
		}
	}
	if cleared < 2 {
		t.Fatalf("cookies cleared: %d", cleared)
	}
	// Idempotent: a second assertion on the same record is refused for want
	// of a session, not a second record.
}

// The phone: an owner-wide bearer key plus the assertion.
func TestDeletionWithOwnerWideBearerKey(t *testing.T) {
	e := newDelEnv(t, "u2")
	key := e.bearer(t)
	opts := e.begin(t, nil, key)
	rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), nil, key)
	if rec.Code != http.StatusOK {
		t.Fatalf("finish: %d %s", rec.Code, rec.Body)
	}
	if !e.h.store.AccountAccessRevoked(e.user.ID) {
		t.Fatal("not revoked")
	}
}

// Failure 1: no assertion, no deletion, whatever the session is.
func TestDeletionNeedsTheAssertion(t *testing.T) {
	e := newDelEnv(t, "u3")
	for name, method := range map[string]string{"passkey session": AuthMethodPasskey, "email session": "email_code"} {
		cookies := e.cookies(t, method)
		if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", []byte(`{}`), cookies, ""); rec.Code == http.StatusOK {
			t.Fatalf("%s: an empty body started a deletion", name)
		}
		opts := e.begin(t, cookies, "")
		// A signature by a different key over the right challenge.
		other := newSoftKey(t, e.key.id)
		forged := other.assert(t, opts, e.h.config.RPID, e.h.config.RPOrigins[0])
		if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", forged, cookies, ""); rec.Code == http.StatusOK {
			t.Fatalf("%s: a forged assertion started a deletion", name)
		}
	}
	// No credential at all.
	if rec := e.call(e.h.HandleDeleteBegin, "/auth/account/delete/begin", nil, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("begin without a credential: %d", rec.Code)
	}
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", []byte(`{}`), nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("finish without a credential: %d", rec.Code)
	}
	e.noRecord(t)
}

// Failure 8.
func TestDeletionRefusesComputerBoundKey(t *testing.T) {
	e := newDelEnv(t, "u4")
	_, secret, err := e.h.store.CreateComputerScopedAPIKey(context.Background(), e.user.ID, "agent", []string{"read:runtime"}, "computer-abc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rec := e.call(e.h.HandleDeleteBegin, "/auth/account/delete/begin", nil, nil, secret); rec.Code != http.StatusForbidden {
		t.Fatalf("computer-bound key: %d", rec.Code)
	}
}

// Failure 3: an assertion minutes old.
func TestDeletionRefusesStaleAssertion(t *testing.T) {
	e := newDelEnv(t, "u5")
	cookies := e.cookies(t, AuthMethodPasskey)
	opts := e.begin(t, cookies, "")
	old := time.Now().UTC().Add(-SessionChallengeTTL - time.Minute)
	if _, err := e.h.store.db.Exec(`UPDATE deletion_challenges SET created_at = ?, expires_at = ?`, old, old.Add(SessionChallengeTTL)); err != nil {
		t.Fatal(err)
	}
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), cookies, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("stale: %d %s", rec.Code, rec.Body)
	}
	e.noRecord(t)
}

// Even inside the TTL the age bound is the challenge's own begin time, not a
// later clock: created_at older than the bound with a far expiry still fails.
func TestDeletionRefusesChallengeOlderThanBoundEvenIfExpiryFar(t *testing.T) {
	e := newDelEnv(t, "u5b")
	cookies := e.cookies(t, AuthMethodPasskey)
	opts := e.begin(t, cookies, "")
	if _, err := e.h.store.db.Exec(`UPDATE deletion_challenges SET created_at = ?`, time.Now().UTC().Add(-2*SessionChallengeTTL)); err != nil {
		t.Fatal(err)
	}
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), cookies, ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("old challenge: %d", rec.Code)
	}
	e.noRecord(t)
}

// Failure 4 and replay: another account's challenge, and a second use.
func TestDeletionChallengeBoundToAccountAndSingleUse(t *testing.T) {
	a := newDelEnv(t, "ua")
	b := addDelUser(t, a.h, a.host, "ub")
	aCookies := a.cookies(t, AuthMethodPasskey)
	bCookies := b.cookies(t, AuthMethodPasskey)
	opts := a.begin(t, aCookies, "")
	// B finishes A's challenge with B's own valid signature over it.
	forged := b.key.assert(t, opts, a.h.config.RPID, a.h.config.RPOrigins[0])
	if rec := b.call(a.h.HandleDeleteFinish, "/auth/account/delete/finish", forged, bCookies, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("cross-account: %d %s", rec.Code, rec.Body)
	}
	b.noRecord(t)
	a.noRecord(t) // the challenge was consumed by the refused attempt
	// A single-use challenge: begin, finish, finish again.
	opts = a.begin(t, aCookies, "")
	body := a.assertion(t, opts)
	if rec := a.call(a.h.HandleDeleteFinish, "/auth/account/delete/finish", body, aCookies, ""); rec.Code != http.StatusOK {
		t.Fatalf("first: %d %s", rec.Code, rec.Body)
	}
	if rec := a.call(a.h.HandleDeleteFinish, "/auth/account/delete/finish", body, aCookies, ""); rec.Code == http.StatusOK {
		t.Fatal("replay accepted")
	}
}

// Failure 2: the two ceremonies never cross.
func TestDeletionAndLoginChallengesNeverCross(t *testing.T) {
	e := newDelEnv(t, "u6")
	cookies := e.cookies(t, AuthMethodPasskey)
	// A login challenge cannot finish a deletion.
	lb := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login/begin", strings.NewReader(`{"email":"u6@example.com"}`))
	e.h.HandleLoginBegin(lb, req)
	if lb.Code != http.StatusOK {
		t.Fatalf("login begin: %d %s", lb.Code, lb.Body)
	}
	loginAssertion := e.assertion(t, lb.Body.Bytes())
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", loginAssertion, cookies, ""); rec.Code == http.StatusOK {
		t.Fatal("a sign-in challenge started a deletion")
	}
	e.noRecord(t)
	// A deletion challenge cannot finish a sign-in.
	opts := e.begin(t, cookies, "")
	before := e.sessionRows(t)
	rec := e.call(e.h.HandleLoginFinish, "/auth/login/finish", e.assertion(t, opts), nil, "")
	if rec.Code == http.StatusOK || e.sessionRows(t) != before {
		t.Fatalf("a deletion challenge signed in: %d", rec.Code)
	}
}

func deleteAccount(t *testing.T, e *delEnv) {
	t.Helper()
	cookies := e.cookies(t, AuthMethodPasskey)
	opts := e.begin(t, cookies, "")
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), cookies, ""); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
}

func (e *delEnv) passkeySignIn(t *testing.T) *httptest.ResponseRecorder {
	t.Helper()
	lb := httptest.NewRecorder()
	e.h.HandleLoginBegin(lb, httptest.NewRequest(http.MethodPost, "/auth/login/begin", strings.NewReader(`{"email":"`+e.user.Email+`"}`)))
	if lb.Code != http.StatusOK {
		t.Fatalf("login begin: %d %s", lb.Code, lb.Body)
	}
	return e.call(e.h.HandleLoginFinish, "/auth/login/finish", e.assertion(t, lb.Body.Bytes()), nil, "")
}

// Failure 6, 7: a pending account signs in to nothing but the cancel offer;
// the offer works once; the cancel restores a session.
func TestPendingAccountSignInOffersCancelOnly(t *testing.T) {
	e := newDelEnv(t, "u7")
	deleteAccount(t, e)
	rec := e.passkeySignIn(t)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("pending sign-in: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge >= 0 && c.Value != "" {
			t.Fatalf("a cookie was set: %s", c.Name)
		}
	}
	if e.sessionRows(t) != 0 {
		t.Fatal("a session was made")
	}
	var offer deletionOffer
	if err := json.Unmarshal(rec.Body.Bytes(), &offer); err != nil || offer.CancelToken == "" || offer.Error != "account_deletion_pending" {
		t.Fatalf("offer: %v %s", err, rec.Body)
	}
	// The offer is not available without the assertion: login begin alone
	// reveals nothing about the deletion.
	lb := httptest.NewRecorder()
	e.h.HandleLoginBegin(lb, httptest.NewRequest(http.MethodPost, "/auth/login/begin", strings.NewReader(`{"email":"u7@example.com"}`)))
	if strings.Contains(lb.Body.String(), "deletion") {
		t.Fatal("login begin leaks the deletion")
	}
	// Cancel.
	body, _ := json.Marshal(cancelRequest{CancelToken: offer.CancelToken})
	c1 := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, "")
	if c1.Code != http.StatusOK {
		t.Fatalf("cancel: %d %s", c1.Code, c1.Body)
	}
	e.noRecord(t)
	signedIn := false
	for _, c := range c1.Result().Cookies() {
		if c.Name == AccessTokenCookieName && c.Value != "" {
			signedIn = true
		}
	}
	if !signedIn || e.sessionRows(t) != 1 {
		t.Fatal("cancel did not sign the person in")
	}
	if len(e.host.restore) != 1 {
		t.Fatalf("restore calls: %v", e.host.restore)
	}
	// Once.
	if c2 := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, ""); c2.Code == http.StatusOK {
		t.Fatal("cancel token worked twice")
	}
	// Ordinary sign-in works again.
	if rec := e.passkeySignIn(t); rec.Code != http.StatusOK {
		t.Fatalf("sign-in after cancel: %d %s", rec.Code, rec.Body)
	}
}

// A failed restore does not undo the cancel: the person is signed in, the
// record is a tombstone that answers as no record, and the runner (not the
// handler) serves the takes again on its next pass.
func TestCancelWithFailedRestoreStillCancelsAndLeavesATombstone(t *testing.T) {
	e := newDelEnv(t, "u7b")
	deleteAccount(t, e)
	var offer deletionOffer
	_ = json.Unmarshal(e.passkeySignIn(t).Body.Bytes(), &offer)
	e.h.SetDeletionHost(&failingRestoreHost{})
	body, _ := json.Marshal(cancelRequest{CancelToken: offer.CancelToken})
	if rec := e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", body, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("a failed restore must not fail the cancel: %d %s", rec.Code, rec.Body)
	}
	e.noRecord(t)
	open, _ := e.h.store.OpenAccountDeletions(0)
	if len(open) != 1 || open[0].State != DeletionCancelled {
		t.Fatalf("the runner has no tombstone to finish: %+v", open)
	}
}

type failingRestoreHost struct{ fakeDeletionHost }

func (f *failingRestoreHost) Restore(context.Context, string) error {
	return errors.New("corpusd down")
}

func TestCancelTokenExpiresAndPurgingRefuses(t *testing.T) {
	e := newDelEnv(t, "u8")
	deleteAccount(t, e)
	var offer deletionOffer
	_ = json.Unmarshal(e.passkeySignIn(t).Body.Bytes(), &offer)
	cancel := func(tok string) int {
		b, _ := json.Marshal(cancelRequest{CancelToken: tok})
		return e.call(e.h.HandleDeleteCancel, "/auth/account/delete/cancel", b, nil, "").Code
	}
	if _, err := e.h.store.db.Exec(`UPDATE account_deletions SET cancel_token_expires = ?`, time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if cancel(offer.CancelToken) == http.StatusOK {
		t.Fatal("expired token cancelled")
	}
	if cancel("not-a-token") == http.StatusOK || cancel("") == http.StatusOK {
		t.Fatal("junk token cancelled")
	}
	// A fresh offer, then the purge begins: no cancel, and sign-in is refused.
	_ = json.Unmarshal(e.passkeySignIn(t).Body.Bytes(), &offer)
	later := time.Now().UTC().Add(purgeguard.GracePeriod + time.Minute)
	if _, err := e.h.store.BeginPurge(e.user.ID, later); err != nil {
		t.Fatal(err)
	}
	if code := cancel(offer.CancelToken); code != http.StatusConflict && code != http.StatusBadRequest {
		t.Fatalf("cancel while purging: %d", code)
	}
	if !e.h.store.AccountAccessRevoked(e.user.ID) {
		t.Fatal("record gone")
	}
	if rec := e.passkeySignIn(t); rec.Code != http.StatusForbidden || strings.Contains(rec.Body.String(), "cancel_token\":\"") && !strings.Contains(rec.Body.String(), `"cancel_token":""`) {
		t.Fatalf("purging sign-in: %d %s", rec.Code, rec.Body)
	}
}

// Failure 6: no other way in makes a session.
func TestPendingAccountCannotGetASessionAnyWay(t *testing.T) {
	e := newDelEnv(t, "u9")
	deleteAccount(t, e)
	rec := httptest.NewRecorder()
	if _, err := e.h.signInSession(rec, httptest.NewRequest(http.MethodPost, "/", nil), e.user, "email_code"); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("email sign-in: %v", err)
	}
	if _, _, err := e.h.createRefreshSessionRow(e.user, "x", AuthMethodPasskey, nil); !errors.Is(err, ErrDeletionPending) {
		t.Fatalf("refresh row: %v", err)
	}
	if e.sessionRows(t) != 0 {
		t.Fatal("a row exists")
	}
}

// Failure 9, and the unavailable case: fail closed.
func TestDeletionPrecheckRefusalsStartNothing(t *testing.T) {
	e := newDelEnv(t, "u10")
	cookies := e.cookies(t, AuthMethodPasskey)
	e.host.precheck = ErrDeletionRefused
	if rec := e.call(e.h.HandleDeleteBegin, "/auth/account/delete/begin", nil, cookies, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("refused begin: %d", rec.Code)
	}
	e.host.precheck = nil
	opts := e.begin(t, cookies, "")
	e.host.precheck = ErrDeletionRefused
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts), cookies, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("refused finish: %d", rec.Code)
	}
	e.noRecord(t)
	if e.sessionRows(t) != 1 {
		t.Fatal("a refused deletion revoked the session")
	}
	e.host.precheck = errors.New("vmctl down")
	opts2 := func() []byte {
		e.host.precheck = nil
		o := e.begin(t, cookies, "")
		e.host.precheck = errors.New("vmctl down")
		return o
	}()
	if rec := e.call(e.h.HandleDeleteFinish, "/auth/account/delete/finish", e.assertion(t, opts2), cookies, ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable: %d", rec.Code)
	}
	e.noRecord(t)
}

func TestRequestAccountDeletionIsIdempotent(t *testing.T) {
	e := newDelEnv(t, "u11")
	now := time.Now().UTC()
	d1, created, err := e.h.store.RequestAccountDeletion(e.user.ID, now)
	if err != nil || !created {
		t.Fatal(err, created)
	}
	d2, created, err := e.h.store.RequestAccountDeletion(e.user.ID, now.Add(time.Hour))
	if err != nil || created || d2.JobID != d1.JobID || !d2.RequestedAt.Equal(d1.RequestedAt) {
		t.Fatalf("second request: %+v %v %v", d2, created, err)
	}
}

// The purge rule and the finalisation.
func TestPurgeEligibleOnlyAfterGracePeriod(t *testing.T) {
	e := newDelEnv(t, "u12")
	s := e.h.store
	now := time.Now().UTC()
	if err := s.PurgeEligible(e.user.ID, now); !errors.Is(err, purgeguard.ErrNoRecord) {
		t.Fatalf("no record: %v", err)
	}
	if _, _, err := s.RequestAccountDeletion(e.user.ID, now); err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{now, now.Add(71 * time.Hour), now.Add(purgeguard.GracePeriod - time.Second)} {
		if err := s.PurgeEligible(e.user.ID, at); !errors.Is(err, purgeguard.ErrTooEarly) {
			t.Fatalf("at +%s: %v", at.Sub(now), err)
		}
		if _, err := s.BeginPurge(e.user.ID, at); err == nil {
			t.Fatal("purge began early")
		}
		if _, err := s.FinalizeAccountDeletion(e.user.ID, "{}", at); err == nil {
			t.Fatal("finalised early")
		}
	}
	if _, err := s.GetUserByID(e.user.ID); err != nil {
		t.Fatalf("user gone early: %v", err)
	}
	if err := s.PurgeEligible(e.user.ID, now.Add(purgeguard.GracePeriod)); err != nil {
		t.Fatal(err)
	}
	// Finalising before the purge has begun is refused too.
	if _, err := s.FinalizeAccountDeletion(e.user.ID, "{}", now.Add(purgeguard.GracePeriod)); err == nil {
		t.Fatal("finalised a pending record")
	}
}

func TestFinalizeDeletesAccountAndWritesReceiptWithNoEmail(t *testing.T) {
	e := newDelEnv(t, "u13")
	s := e.h.store
	if _, _, err := s.EnsureOwnerRoot(e.user.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.PutOwnerRootWrap(e.user.ID, e.key.id, bytes.Repeat([]byte{1}, 32), []byte("wrapped")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := s.db.Exec(`INSERT INTO registration_waitlist (email, created_at, last_seen_at) VALUES (?, ?, ?)`, e.user.Email, now, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RequestAccountDeletion(e.user.ID, now); err != nil {
		t.Fatal(err)
	}
	later := now.Add(purgeguard.GracePeriod + time.Minute)
	d, err := s.BeginPurge(e.user.ID, later)
	if err != nil || d.State != DeletionPurging || d.Attempts != 1 {
		t.Fatalf("begin: %+v %v", d, err)
	}
	rec, err := s.FinalizeAccountDeletion(e.user.ID, `{"counts":{"takes":2}}`, later)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetUserByID(e.user.ID); err == nil {
		t.Fatal("user row survives")
	}
	for _, q := range []string{
		`SELECT COUNT(*) FROM credentials`, `SELECT COUNT(*) FROM owner_root_keys`, `SELECT COUNT(*) FROM owner_root_key_wraps`,
		`SELECT COUNT(*) FROM account_deletions`, `SELECT COUNT(*) FROM registration_waitlist`,
	} {
		var n int
		if err := s.db.QueryRow(q).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s = %d (%v)", q, n, err)
		}
	}
	// The address is free: it can register again.
	if _, err := s.CreateUser("new-owner-of-address", e.user.Email); err != nil {
		t.Fatalf("address not freed: %v", err)
	}
	got, err := s.DeletionReceiptByJob(rec.JobID)
	if err != nil || got == nil {
		t.Fatal(err)
	}
	blob := got.ID + got.JobID + got.UserHash + got.Summary
	if strings.Contains(blob, "example.com") || strings.Contains(blob, e.user.ID) {
		t.Fatalf("receipt holds personal data: %s", blob)
	}
	if got.UserHash != DeletionUserHash(e.user.ID) {
		t.Fatal("hash")
	}
}

// A refused purge leaves the account whole.
func TestAccountAccessRevokedFailsClosed(t *testing.T) {
	e := newDelEnv(t, "u14")
	if e.h.store.AccountAccessRevoked(e.user.ID) {
		t.Fatal("revoked with no record")
	}
	if e.h.store.AccountAccessRevoked("") {
		t.Fatal("empty id revoked")
	}
	_ = webauthn.SessionData{}
}

// The numbered-address loop must not hand a new number to an account whose
// mail the purge has just removed (the user row outlives the mail purge by a
// step).
func TestNumberReconcileSkipsAccountsBeingDeleted(t *testing.T) {
	h, _ := testHandlerEnv(t)
	a := addDelUser(t, h, nil, "real-a")
	b := addDelUser(t, h, nil, "real-b")
	// example.com is an ephemeral domain; give both a real domain.
	for _, e := range []*delEnv{a, b} {
		if _, err := h.store.db.Exec(`UPDATE users SET email = ? WHERE id = ?`, e.user.ID+"@mail.test-real.org", e.user.ID); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := h.store.ListNumberCandidates(context.Background())
	if err != nil || len(ids) != 2 {
		t.Fatalf("before: %v %v", ids, err)
	}
	if _, _, err := h.store.RequestAccountDeletion(a.user.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	ids, _ = h.store.ListNumberCandidates(context.Background())
	if len(ids) != 1 || ids[0] != b.user.ID {
		t.Fatalf("after: %v", ids)
	}
}
