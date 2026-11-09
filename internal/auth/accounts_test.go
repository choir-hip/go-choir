package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// Failure-mode tests for docs/design/account-switcher-2026-11-09.md.

// jar is a minimal browser cookie jar (the test sends every cookie to every
// route; path scoping is checked separately).
type jar map[string]*http.Cookie

func (j jar) apply(rec *httptest.ResponseRecorder) {
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(j, c.Name)
			continue
		}
		j[c.Name] = c
	}
}

func (j jar) clone() jar {
	o := jar{}
	for k, v := range j {
		o[k] = v
	}
	return o
}

func (j jar) req(method, path string, body interface{}) *http.Request {
	var req *http.Request
	if body != nil {
		req = jsonReq(method, path, body, "")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	for _, c := range j {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	return req
}

func acctEnv(t *testing.T) *Handler {
	t.Helper()
	h, _ := testHandlerEnv(t)
	h.config.RotationGraceTTL = 15 * time.Second
	return h
}

func mkUser(t *testing.T, h *Handler, id string) *User {
	t.Helper()
	u, err := h.store.CreateUser(id, id+"@example.com")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// signIn signs the user in through the sign-in issuer, in the browser j.
func signIn(t *testing.T, h *Handler, j jar, u *User) {
	t.Helper()
	rec := httptest.NewRecorder()
	if _, err := h.signInSession(rec, j.req(http.MethodPost, "/auth/login/finish", nil), u, AuthMethodPasskey); err != nil {
		t.Fatal(err)
	}
	j.apply(rec)
}

func call(h *Handler, fn http.HandlerFunc, j jar, method, path string, body interface{}) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	fn(rec, j.req(method, path, body))
	j.apply(rec)
	return rec
}

func sessionOf(t *testing.T, h *Handler, j jar) sessionResponse {
	t.Helper()
	rec := call(h, h.HandleSession, j, http.MethodGet, "/auth/session", nil)
	var sr sessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sr); err != nil {
		t.Fatalf("session body: %v %s", err, rec.Body.String())
	}
	return sr
}

func switchTo(h *Handler, j jar, uid string) *httptest.ResponseRecorder {
	return call(h, h.HandleAccountSwitch, j, http.MethodPost, "/auth/accounts/switch", map[string]string{"user_id": uid})
}

func listAccounts(t *testing.T, h *Handler, j jar) []accountEntry {
	t.Helper()
	rec := call(h, h.HandleAccountsList, j, http.MethodGet, "/auth/accounts", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
	}
	var out []accountEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// twoAccounts signs a and b in, in that order; b is active.
func twoAccounts(t *testing.T) (*Handler, jar, *User, *User) {
	h := acctEnv(t)
	a, b := mkUser(t, h, "ua"), mkUser(t, h, "ub")
	j := jar{}
	signIn(t, h, j, a)
	signIn(t, h, j, b)
	return h, j, a, b
}

func setCookies(rec *httptest.ResponseRecorder) []string {
	var n []string
	for _, c := range rec.Result().Cookies() {
		n = append(n, c.Name)
	}
	return n
}

// 1
func TestAccountsSwitchRefusesNonMemberAndMissingSet(t *testing.T) {
	h, j, _, _ := twoAccounts(t)
	other := mkUser(t, h, "uz")
	signIn(t, h, jar{}, other) // signed in elsewhere, not in j's set
	if rec := switchTo(h, j.clone(), "uz"); rec.Code != http.StatusForbidden {
		t.Fatalf("non-member switch = %d", rec.Code)
	}
	nojar := j.clone()
	delete(nojar, AccountsCookieName)
	if rec := switchTo(h, nojar, "ua"); rec.Code != http.StatusForbidden {
		t.Fatalf("no set cookie switch = %d", rec.Code)
	}
	if got := listAccounts(t, h, nojar); len(got) != 0 {
		t.Fatalf("list without set cookie = %v", got)
	}
}

// 2
func TestAccountsSwitchRefusesAPIKey(t *testing.T) {
	h, j, _, _ := twoAccounts(t)
	req := j.req(http.MethodPost, "/auth/accounts/switch", map[string]string{"user_id": "ua"})
	req.Header.Set("Authorization", "Bearer choir_sk_x")
	rec := httptest.NewRecorder()
	h.HandleAccountSwitch(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("api key switch = %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/auth/accounts/switch", nil)
	req.Header.Set("Authorization", "Bearer choir_sk_x")
	rec = httptest.NewRecorder()
	h.HandleAccountSwitch(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("api key only = %d", rec.Code)
	}
}

// 3
func TestAccountsSwitchToDeadMemberIsRefusedWithReason(t *testing.T) {
	h, j, _, _ := twoAccounts(t)
	m, _ := h.store.GetAccountMember(mustSet(t, h, j).ID, "ua")
	if _, err := h.store.DB().Exec("UPDATE refresh_sessions SET expires_at = ? WHERE id = ?", time.Now().UTC().Add(-time.Minute), m.RefreshSessionID); err != nil {
		t.Fatal(err)
	}
	rec := switchTo(h, j, "ua")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), ReasonMemberSignedOut) {
		t.Fatalf("dead member switch = %d %s", rec.Code, rec.Body.String())
	}
	// Revoked (row gone) too.
	h2, j2, _, _ := twoAccounts(t)
	m2, _ := h2.store.GetAccountMember(mustSet(t, h2, j2).ID, "ua")
	_ = h2.store.DeleteRefreshSessionByID(m2.RefreshSessionID)
	if rec := switchTo(h2, j2, "ua"); rec.Code != http.StatusConflict {
		t.Fatalf("revoked member switch = %d", rec.Code)
	}
	// Listed once as signed out, then gone.
	got := listAccounts(t, h2, j2)
	if len(got) != 2 || got[1].SignedIn {
		t.Fatalf("first list = %+v", got)
	}
	if got = listAccounts(t, h2, j2); len(got) != 1 {
		t.Fatalf("second list = %+v", got)
	}
}

func mustSet(t *testing.T, h *Handler, j jar) *AccountSet {
	t.Helper()
	r := j.req(http.MethodGet, "/", nil)
	s := h.requestAccountSet(r)
	if s == nil {
		t.Fatal("no account set")
	}
	return s
}

// 4 + 5
func TestAccountsSwitchResolvesNewUserKeepsOldSessionLiveAndGraceHolds(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	if sr := sessionOf(t, h, j); sr.User == nil || sr.User.ID != b.ID {
		t.Fatalf("before switch = %+v", sr)
	}
	rec := switchTo(h, j, a.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("switch = %d %s", rec.Code, rec.Body.String())
	}
	if sr := sessionOf(t, h, j); sr.User == nil || sr.User.ID != a.ID {
		t.Fatalf("after switch = %+v", sr)
	}
	// B's session is still live.
	bm, _ := h.store.GetAccountMember(mustSet(t, h, j).ID, b.ID)
	if _, err := h.store.GetRefreshSessionByID(bm.RefreshSessionID); err != nil {
		t.Fatalf("previous account's session gone: %v", err)
	}
}

func TestAccountsRotationGraceHoldsForSwitchedToSession(t *testing.T) {
	h, j, a, _ := twoAccounts(t)
	// Remember A's refresh token as signed in (member A's first session).
	// Re-sign-in is not possible; use a switch to A, then replay the token
	// the switch response replaced via the grace entry.
	rec := switchTo(h, j, a.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("switch = %d", rec.Code)
	}
	fresh := j[RefreshTokenCookieName].Value
	// A second switch to A in the grace window reuses the pair (rotates once).
	rec = switchTo(h, j.clone(), a.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("second switch = %d", rec.Code)
	}
	var got string
	for _, c := range rec.Result().Cookies() {
		if c.Name == RefreshTokenCookieName {
			got = c.Value
		}
	}
	if got != fresh {
		t.Fatalf("second switch within grace rotated again (%q vs %q)", got, fresh)
	}
}

// 6 + 14
func TestAccountsSignoutPerMember(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	c := mkUser(t, h, "uc")
	signIn(t, h, j, c) // c active; a, b inactive
	// Inactive member: removed, no cookie written.
	rec := call(h, h.HandleAccountSignout, j, http.MethodPost, "/auth/accounts/signout", map[string]string{"user_id": a.ID})
	if rec.Code != http.StatusOK || len(setCookies(rec)) != 0 {
		t.Fatalf("inactive signout = %d cookies=%v", rec.Code, setCookies(rec))
	}
	if got := listAccounts(t, h, j); len(got) != 2 {
		t.Fatalf("after inactive signout = %+v", got)
	}
	// Active member: names the next.
	rec = call(h, h.HandleAccountSignout, j, http.MethodPost, "/auth/accounts/signout", map[string]string{"user_id": c.ID})
	var resp accountSignoutResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if rec.Code != http.StatusOK || resp.NextUserID != b.ID {
		t.Fatalf("active signout = %d %s", rec.Code, rec.Body.String())
	}
	if rec := switchTo(h, j, resp.NextUserID); rec.Code != http.StatusOK {
		t.Fatalf("follow-up switch = %d", rec.Code)
	}
	if sr := sessionOf(t, h, j); sr.User == nil || sr.User.ID != b.ID {
		t.Fatalf("after next switch = %+v", sr)
	}
	// Non-member: 403.
	rec = call(h, h.HandleAccountSignout, j, http.MethodPost, "/auth/accounts/signout", map[string]string{"user_id": a.ID})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-member signout = %d", rec.Code)
	}
	// Last member: set emptied, cookie cleared.
	rec = call(h, h.HandleAccountSignout, j, http.MethodPost, "/auth/accounts/signout", map[string]string{"user_id": b.ID})
	var cleared bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == AccountsCookieName && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("last signout did not clear set cookie: %v", setCookies(rec))
	}
	if _, err := h.store.GetAccountSetByHandleHash("x"); err == nil {
		t.Fatal("unexpected set")
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM account_set").Scan(&n)
	if n != 0 {
		t.Fatalf("sets left = %d", n)
	}
}

func TestAccountsLogoutAllRevokesMembersOnlyHere(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	// A's other device.
	other := jar{}
	signIn(t, h, other, a)
	hadSet := mustSet(t, h, j)
	rec := call(h, h.HandleAccountsLogoutAll, j, http.MethodPost, "/auth/accounts/logout-all", map[string]string{})
	if rec.Code != http.StatusOK {
		t.Fatalf("logout-all = %d", rec.Code)
	}
	if _, err := h.store.GetAccountSetByHandleHash(hadSet.HandleHash); err == nil {
		t.Fatal("set survived logout-all")
	}
	if ms, _ := h.store.ListAccountMembers(hadSet.ID); len(ms) != 0 {
		t.Fatal("members survived logout-all")
	}
	for _, name := range []string{AccessTokenCookieName, RefreshTokenCookieName, AccountsCookieName} {
		if _, ok := j[name]; ok {
			t.Fatalf("cookie %s survived", name)
		}
	}
	delete(other, AccessTokenCookieName)
	if sr := sessionOf(t, h, other); sr.User == nil || sr.User.ID != a.ID {
		t.Fatalf("other device signed out: %+v", sr)
	}
	_ = b
	// JSON content type required.
	r := httptest.NewRequest(http.MethodPost, "/auth/accounts/logout-all", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	h.HandleAccountsLogoutAll(rr, r)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("no content type = %d", rr.Code)
	}
}

// 7
func TestAccountsSecondSignInReplacesMemberNeverDuplicates(t *testing.T) {
	h, j, a, _ := twoAccounts(t)
	set := mustSet(t, h, j)
	old, _ := h.store.GetAccountMember(set.ID, a.ID)
	signIn(t, h, j, a)
	ms, _ := h.store.ListAccountMembers(set.ID)
	if len(ms) != 2 {
		t.Fatalf("members = %d", len(ms))
	}
	cur, _ := h.store.GetAccountMember(set.ID, a.ID)
	if cur.RefreshSessionID == old.RefreshSessionID {
		t.Fatal("member session not replaced")
	}
	if _, err := h.store.GetRefreshSessionByID(old.RefreshSessionID); err == nil {
		t.Fatal("old session row survived")
	}
	if got := mustSet(t, h, j); got.ActiveUserID != a.ID {
		t.Fatalf("active = %s", got.ActiveUserID)
	}
}

func TestAccountsConcurrentFirstSignInsMakeOneSet(t *testing.T) {
	h := acctEnv(t)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		u := mkUser(t, h, "uc"+string(rune('a'+i)))
		sid := "sess" + u.ID
		if _, err := h.store.DB().Exec("INSERT INTO refresh_sessions (id,user_id,token_hash,created_at,expires_at,auth_method) VALUES (?,?,?,?,?,'passkey')", sid, u.ID, sid, time.Now().UTC(), time.Now().UTC().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			// All six present the same already-minted handle.
			if _, _, err := h.store.AddAccountToSet("shared-hash", "shared-hash", u.ID, sid); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var sets, members int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM account_set").Scan(&sets)
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM account_set_member").Scan(&members)
	if sets != 1 || members != 6 {
		t.Fatalf("sets=%d members=%d", sets, members)
	}
}

// 8
func TestAccountsSetCookieIsOpaqueAndStoredHashed(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	raw := j[AccountsCookieName].Value
	for _, leak := range []string{a.Email, b.Email, j[RefreshTokenCookieName].Value} {
		if strings.Contains(raw, leak) {
			t.Fatalf("set cookie leaks %q", leak)
		}
	}
	if len(raw) < 40 {
		t.Fatalf("handle too short: %d", len(raw))
	}
	var hash string
	_ = h.store.DB().QueryRow("SELECT handle_hash FROM account_set").Scan(&hash)
	if hash == raw || hash != accountsHandleHash(raw) {
		t.Fatal("db must hold only the sha256 of the handle")
	}
	// Scoped to /auth like the refresh cookie.
	rec := httptest.NewRecorder()
	h.signInSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), a, AuthMethodPasskey)
	for _, c := range rec.Result().Cookies() {
		if c.Name == AccountsCookieName && (c.Path != "/auth" || !c.HttpOnly || c.MaxAge != int(AccountsCookieTTL.Seconds())) {
			t.Fatalf("set cookie attrs = %+v", c)
		}
	}
}

// 10
func TestAccountsMembersFollowRotationAndDoubleSwitchRotatesOnce(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	for i := 0; i < 3; i++ {
		if sr := sessionOf(t, h, j); sr.User == nil || sr.User.ID != b.ID {
			t.Fatalf("renew %d: %+v", i, sr)
		}
		// Expire the access cookie so the next check renews.
		delete(j, AccessTokenCookieName)
	}
	for _, l := range listAccounts(t, h, j) {
		if !l.SignedIn {
			t.Fatalf("member %s lost its session after renewals", l.UserID)
		}
	}
	// rotated_from is populated by the transactional rotation.
	var withParent int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM refresh_sessions WHERE rotated_from IS NOT NULL AND rotated_from <> ''").Scan(&withParent)
	if withParent == 0 {
		t.Fatal("rotated_from never populated")
	}
	// Racing double switch to A.
	var wg sync.WaitGroup
	set := mustSet(t, h, j)
	before, _ := h.store.GetAccountMember(set.ID, a.ID)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		jc := j.clone()
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			h.HandleAccountSwitch(rec, jc.req(http.MethodPost, "/auth/accounts/switch", map[string]string{"user_id": a.ID}))
			if rec.Code != http.StatusOK {
				t.Errorf("racing switch = %d", rec.Code)
			}
		}()
	}
	wg.Wait()
	after, _ := h.store.GetAccountMember(set.ID, a.ID)
	cur, err := h.store.GetRefreshSessionByID(after.RefreshSessionID)
	if err != nil || cur.RotatedFrom != before.RefreshSessionID {
		t.Fatalf("double switch rotated more than once: %+v err=%v", cur, err)
	}
}

// 11
func TestAccountsStaleTabRenewalGetsAccountSwitchedAndWritesNoCookie(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	stale := j.clone() // a tab whose jar view is B's pair
	delete(stale, AccessTokenCookieName)
	if rec := switchTo(h, j, a.ID); rec.Code != http.StatusOK {
		t.Fatalf("switch = %d", rec.Code)
	}
	// stale carries B's refresh cookie; active is now A.
	rec := call(h, h.HandleSession, stale, http.MethodGet, "/auth/session", nil)
	var sr sessionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if sr.Authenticated || sr.Reason != ReasonAccountSwitched || sr.ActiveUserID != a.ID {
		t.Fatalf("stale renewal = %+v", sr)
	}
	if len(setCookies(rec)) != 0 {
		t.Fatalf("stale renewal wrote cookies %v", setCookies(rec))
	}
	if sr := sessionOf(t, h, j); sr.User == nil || sr.User.ID != a.ID {
		t.Fatalf("following request = %+v", sr)
	}
	// Inside the grace window: B renews (rotates) and then A is switched to; B's
	// pre-rotation token is only in the grace table.
	h2, j2, a2, b2 := twoAccounts(t)
	oldB := j2[RefreshTokenCookieName].Value
	delete(j2, AccessTokenCookieName)
	if sr := sessionOf(t, h2, j2); sr.User == nil || sr.User.ID != b2.ID {
		t.Fatalf("renew = %+v", sr)
	}
	if rec := switchTo(h2, j2, a2.ID); rec.Code != http.StatusOK {
		t.Fatalf("switch = %d", rec.Code)
	}
	graceJar := jar{AccountsCookieName: j2[AccountsCookieName], RefreshTokenCookieName: {Name: RefreshTokenCookieName, Value: oldB}}
	rec = call(h2, h2.HandleSession, graceJar, http.MethodGet, "/auth/session", nil)
	sr = sessionResponse{}
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if sr.Authenticated || sr.Reason != ReasonAccountSwitched || len(setCookies(rec)) != 0 {
		t.Fatalf("grace-window stale renewal = %+v cookies=%v", sr, setCookies(rec))
	}
	_ = b
}

// 12a, 12b, 12c
func TestAccountsDivergedJarReconcilesByOneSwitchNoLoop(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	// The jar diverges: it holds A's pair (a lost switch response) while the
	// set says B is active. Build A's pair by switching, then restoring the set.
	set := mustSet(t, h, j)
	if rec := switchTo(h, j, a.ID); rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	diverged := j.clone()
	if err := h.store.SetActiveAccount(set.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	// 12a: the first renewal (no access cookie) is refused with the active user.
	d1 := diverged.clone()
	delete(d1, AccessTokenCookieName)
	rec := call(h, h.HandleSession, d1, http.MethodGet, "/auth/session", nil)
	var sr sessionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if sr.Reason != ReasonAccountSwitched || sr.ActiveUserID != b.ID {
		t.Fatalf("diverged first request = %+v", sr)
	}
	// 12b: the reconcile switch succeeds while the jar still carries A's valid
	// access JWT.
	if _, ok := diverged[AccessTokenCookieName]; !ok {
		t.Fatal("test needs the previous member's access JWT in the jar")
	}
	if rec := switchTo(h, diverged, sr.ActiveUserID); rec.Code != http.StatusOK {
		t.Fatalf("reconcile switch with previous member's JWT = %d %s", rec.Code, rec.Body.String())
	}
	// After one reload the session is B and stays B (no second refusal).
	for i := 0; i < 2; i++ {
		if got := sessionOf(t, h, diverged); got.User == nil || got.User.ID != b.ID {
			t.Fatalf("after reconcile %d: %+v", i, got)
		}
	}
}

func TestAccountsSameMemberSwitchRotatesAndSessionResolves(t *testing.T) {
	h, j, _, b := twoAccounts(t)
	set := mustSet(t, h, j)
	before, _ := h.store.GetAccountMember(set.ID, b.ID)
	oldRefresh := j[RefreshTokenCookieName].Value
	rec := switchTo(h, j, b.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-member switch = %d", rec.Code)
	}
	if j[RefreshTokenCookieName].Value == oldRefresh {
		t.Fatal("same-member switch did not write a fresh refresh cookie")
	}
	after, _ := h.store.GetAccountMember(set.ID, b.ID)
	if after.RefreshSessionID == before.RefreshSessionID {
		t.Fatal("same-member switch did not rotate")
	}
	if got := mustSet(t, h, j); got.ActiveUserID != b.ID {
		t.Fatal("active_user_id changed")
	}
	delete(j, AccessTokenCookieName)
	if sr := sessionOf(t, h, j); sr.User == nil || sr.User.ID != b.ID {
		t.Fatalf("following session = %+v", sr)
	}
}

// 13
func TestAccountsLogoutParamsUnchanged(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	other := jar{}
	signIn(t, h, other, b) // b's other device
	other2 := jar{}
	signIn(t, h, other2, b)
	rec := call(h, h.HandleLogout, j, http.MethodPost, "/auth/logout?all_devices=true", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout all_devices = %d", rec.Code)
	}
	// b's sessions on every device are gone.
	delete(other, AccessTokenCookieName)
	if sr := sessionOf(t, h, other); sr.Authenticated {
		t.Fatal("other device of b survived all_devices")
	}
	// a stays a member of nothing; its session is untouched by the user-scoped delete.
	_ = a
	// Email sessions still cannot revoke all devices.
	he, m := emailEnv(t)
	addEmailUser(t, he, "ue", "e@example.com", true, true)
	ej := jar{}
	for _, c := range signInByEmail(t, he, m, "e@example.com") {
		ej[c.Name] = c
	}
	rec = call(he, he.HandleLogout, ej, http.MethodPost, "/auth/logout?all=true", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("email session all=true = %d", rec.Code)
	}
}

func TestAccountsLogoutRemovesMemberAndNamesNext(t *testing.T) {
	h, j, a, b := twoAccounts(t)
	rec := call(h, h.HandleLogout, j, http.MethodPost, "/auth/logout", nil)
	var sr sessionResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &sr)
	if sr.NextUserID != a.ID {
		t.Fatalf("logout response = %+v", sr)
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM account_set_member").Scan(&n)
	if n != 1 {
		t.Fatalf("members after logout = %d", n)
	}
	_ = b
}

// 15
func TestAccountsPostsRequireJSONContentType(t *testing.T) {
	h, j, a, _ := twoAccounts(t)
	for name, fn := range map[string]http.HandlerFunc{
		"/auth/accounts/switch":  h.HandleAccountSwitch,
		"/auth/accounts/signout": h.HandleAccountSignout,
	} {
		req := httptest.NewRequest(http.MethodPost, name, strings.NewReader(`{"user_id":"`+a.ID+`"}`))
		req.Header.Set("Content-Type", "text/plain")
		for _, c := range j {
			req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
		rec := httptest.NewRecorder()
		fn(rec, req)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("%s without json = %d", name, rec.Code)
		}
	}
}

// 16
func TestAccountsDesktopRedeemSessionMintsNoSetAndSweepCleans(t *testing.T) {
	h := acctEnv(t)
	u := mkUser(t, h, "ur")
	rec := httptest.NewRecorder()
	if _, err := h.issueSessionExpiring(rec, httptest.NewRequest(http.MethodGet, "/", nil), u, AuthMethodPasskey, nil); err != nil {
		t.Fatal(err)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == AccountsCookieName {
			t.Fatal("plain issuer minted a set cookie")
		}
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM account_set").Scan(&n)
	if n != 0 {
		t.Fatal("plain issuer minted a set")
	}

	// Sweep: dead members, empty sets and 30-day-idle sets go.
	h2, j, a, b := twoAccounts(t)
	set := mustSet(t, h2, j)
	m, _ := h2.store.GetAccountMember(set.ID, a.ID)
	_ = h2.store.DeleteRefreshSessionByID(m.RefreshSessionID)
	if mem, _, err := h2.store.SweepAccountSets(time.Now()); err != nil || mem != 1 {
		t.Fatalf("sweep members=%d err=%v", mem, err)
	}
	if ms, _ := h2.store.ListAccountMembers(set.ID); len(ms) != 1 || ms[0].UserID != b.ID {
		t.Fatalf("members after sweep = %+v", ms)
	}
	if _, sets, _ := h2.store.SweepAccountSets(time.Now().Add(AccountSetIdleTTL + time.Hour)); sets != 1 {
		t.Fatalf("idle sets swept = %d", sets)
	}
	// Empty set.
	if _, err := h2.store.DB().Exec("INSERT INTO account_set (id, handle_hash, active_user_id, created_at, last_used_at) VALUES ('e','eh','',?,?)", time.Now().UTC(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, sets, _ := h2.store.SweepAccountSets(time.Now()); sets != 1 {
		t.Fatalf("empty sets swept = %d", sets)
	}
}

// Set creation through the real email sign-in endpoint.
func TestAccountsEmailVerifyCreatesSet(t *testing.T) {
	h, m := emailEnv(t)
	addEmailUser(t, h, "ue", "e@example.com", true, true)
	cs := signInByEmail(t, h, m, "e@example.com")
	if cookieByName(cs, AccountsCookieName) == nil {
		t.Fatal("email verify did not create a set")
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM account_set_member").Scan(&n)
	if n != 1 {
		t.Fatalf("members = %d", n)
	}
}

// The four sign-in endpoints use the set-adding issuer; the desktop handoff
// does not.
func TestAccountsSignInEndpointsUseSetIssuer(t *testing.T) {
	for file, want := range map[string]int{"handlers.go": 2, "email_signin.go": 1} {
		src := readSource(t, file)
		if got := strings.Count(src, "h.signInSession("); got != want {
			t.Fatalf("%s: signInSession calls = %d, want %d", file, got, want)
		}
		if strings.Contains(src, "h.issueSession(w, r, user, AuthMethodPasskey)") {
			t.Fatalf("%s: a sign-in endpoint still uses the plain issuer", file)
		}
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
