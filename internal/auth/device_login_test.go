package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Written before the code (CLAUDE.md, Testing). How the scope default and the
// device flow (docs/design/cli-agent-onboarding-2026-11-03.md) could fail:
//
//  S1 omitted scopes still 400, or mint a key short of a scope (owner:acts).
//  S2 "all" drifts from validScopes, or includes the retired propose scope.
//  S3 an explicit list is widened, or an explicit empty list is accepted.
//  S4 a Bearer key that omits scopes gets owner:acts or more than it holds.
//  S5 existing keys change scopes.
//  D1 begin returns codes that are stored in plain text, or not unique.
//  D2 a poll before approval mints a key (pending must say pending).
//  D3 approval by an API key, an email session, or a non-JSON (form) POST.
//  D4 a denied request still yields a key.
//  D5 an expired request (approved or not) yields a key.
//  D6 a second collect mints a second key (reuse).
//  D7 polling faster than the interval is not slowed.
//  D8 lookup leaks other users' pending requests by guessing (no limit).
//  D9 the minted key lacks a scope, belongs to the wrong user, or the
//     user_code alone (without the device_code) collects it.
//  D10 the begin limit per IP does not hold.

func passkeyCookieReq(method, path string, body any, priv ed25519.PrivateKey, userID string) *http.Request {
	req := jsonReq(method, path, body, "")
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: issueTestAccessJWTForHandler(priv, userID)})
	return req
}

func emailSessionCookieReq(method, path string, body any, priv ed25519.PrivateKey, userID string) *http.Request {
	claims := jwt.MapClaims{
		"sub": userID, "email": "test@example.com", "iat": time.Now().Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(), "scope": "access", "am": AuthMethodEmailCode,
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims).SignedString(priv)
	if err != nil {
		panic(err)
	}
	req := jsonReq(method, path, body, "")
	req.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: signed})
	return req
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return m
}

func scopesOf(t *testing.T, v any) []string {
	t.Helper()
	raw, _ := v.([]any)
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		out = append(out, s.(string))
	}
	sort.Strings(out)
	return out
}

// S2: AllScopes is validScopes minus the retired scope, sorted.
func TestAllScopesIsTheRegistryMinusRetired(t *testing.T) {
	all := AllScopes()
	if !sort.StringsAreSorted(all) {
		t.Fatalf("AllScopes not sorted: %v", all)
	}
	want := 0
	for s := range validScopes {
		if s != retiredScopeSelfDevPropose {
			want++
		}
	}
	if len(all) != want {
		t.Fatalf("AllScopes has %d, want %d", len(all), want)
	}
	has := map[string]bool{}
	for _, s := range all {
		if !validScopes[s] {
			t.Fatalf("AllScopes has unknown %q", s)
		}
		has[s] = true
	}
	if has[retiredScopeSelfDevPropose] {
		t.Fatal("AllScopes includes the retired propose scope")
	}
	for _, s := range []string{OwnerActsScope, "admin", "manage:keys", "read:texture", "computer:self_development:approve"} {
		if !has[s] {
			t.Fatalf("AllScopes is missing %q", s)
		}
	}
	// A copy: callers may not mutate the registry through it.
	all[0] = "mutated"
	if AllScopes()[0] == "mutated" {
		t.Fatal("AllScopes returns shared storage")
	}
}

// S1: a session create with scopes omitted gets every scope.
func TestCreateAPIKeyOmittedScopesMeansAll(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("omit-user", "omit@example.com")
	rec := httptest.NewRecorder()
	h.HandleCreateAPIKey(rec, passkeyCookieReq(http.MethodPost, "/auth/api-keys", map[string]any{"label": "agent"}, priv, u.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	got := scopesOf(t, decodeBody(t, rec)["scopes"])
	if strings.Join(got, ",") != strings.Join(AllScopes(), ",") {
		t.Fatalf("scopes %v, want %v", got, AllScopes())
	}
}

// S3: an explicit list is kept exactly; an explicit empty list is refused.
func TestCreateAPIKeyExplicitScopesKept(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("explicit-user", "explicit@example.com")
	rec := httptest.NewRecorder()
	h.HandleCreateAPIKey(rec, passkeyCookieReq(http.MethodPost, "/auth/api-keys", map[string]any{"label": "narrow", "scopes": []string{"read:runtime"}}, priv, u.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if got := scopesOf(t, decodeBody(t, rec)["scopes"]); len(got) != 1 || got[0] != "read:runtime" {
		t.Fatalf("scopes %v", got)
	}
	rec = httptest.NewRecorder()
	h.HandleCreateAPIKey(rec, passkeyCookieReq(http.MethodPost, "/auth/api-keys", map[string]any{"label": "empty", "scopes": []string{}}, priv, u.ID))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("explicit empty scopes: status %d, want 400", rec.Code)
	}
}

// S4: a Bearer key that omits scopes gets what it may delegate, never owner:acts.
func TestCreateAPIKeyOmittedScopesFromKeyIsDelegable(t *testing.T) {
	h, _ := testHandlerEnv(t)
	u, _ := h.store.CreateUser("bearer-user", "bearer@example.com")
	ctx := context.Background()
	mint := func(parent []string) []string {
		_, secret, err := h.store.CreateAPIKey(ctx, u.ID, "parent", parent, nil)
		if err != nil {
			t.Fatal(err)
		}
		req := jsonReq(http.MethodPost, "/auth/api-keys", map[string]any{"label": "child"}, "")
		req.Header.Set("Authorization", "Bearer "+secret)
		rec := httptest.NewRecorder()
		h.HandleCreateAPIKey(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("parent %v: status %d: %s", parent, rec.Code, rec.Body.String())
		}
		return scopesOf(t, decodeBody(t, rec)["scopes"])
	}
	adminChild := mint([]string{"admin", OwnerActsScope})
	for _, s := range adminChild {
		if s == OwnerActsScope {
			t.Fatal("a key minted owner:acts by omission")
		}
	}
	if len(adminChild) != len(AllScopes())-1 {
		t.Fatalf("admin child %v, want every scope but owner:acts", adminChild)
	}
	mgrChild := mint([]string{"manage:keys", "read:texture", OwnerActsScope})
	if strings.Join(mgrChild, ",") != "manage:keys,read:texture" {
		t.Fatalf("manager child %v", mgrChild)
	}
}

// S5: the default does not touch keys that already exist.
func TestExistingKeysKeepTheirScopes(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("existing-user", "existing@example.com")
	ctx := context.Background()
	_, secret, err := h.store.CreateAPIKey(ctx, u.ID, "old reader", []string{"read:runtime"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.HandleCreateAPIKey(rec, passkeyCookieReq(http.MethodPost, "/auth/api-keys", map[string]any{"label": "new"}, priv, u.ID))
	if rec.Code != http.StatusCreated {
		t.Fatal(rec.Body.String())
	}
	ak, err := h.store.GetAPIKeyByHash(ctx, hashAPIKeySecret(secret))
	if err != nil {
		t.Fatal(err)
	}
	if len(ak.Scopes) != 1 || ak.Scopes[0] != "read:runtime" {
		t.Fatalf("existing key scopes changed: %v", ak.Scopes)
	}
}

// --- device flow ---

type deviceBeginBody struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

func deviceBegin(t *testing.T, h *Handler, label, ip string) deviceBeginBody {
	t.Helper()
	req := jsonReq(http.MethodPost, "/auth/device/begin", map[string]string{"label": label}, ip)
	req.Header.Set("User-Agent", "choir-cli/test (darwin)")
	rec := httptest.NewRecorder()
	h.HandleDeviceBegin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("begin: status %d: %s", rec.Code, rec.Body.String())
	}
	var b deviceBeginBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func devicePoll(h *Handler, deviceCode string) (int, map[string]any) {
	rec := httptest.NewRecorder()
	h.HandleDeviceToken(rec, jsonReq(http.MethodPost, "/auth/device/token", map[string]string{"device_code": deviceCode}, ""))
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m
}

func deviceDecide(h *Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.HandleDeviceApprove(rec, req)
	return rec
}

// rewindPoll lets a test poll again without waiting out the interval.
func rewindPoll(t *testing.T, h *Handler) {
	t.Helper()
	if _, err := h.store.DB().Exec("UPDATE device_authorizations SET last_poll_at = NULL"); err != nil {
		t.Fatal(err)
	}
}

// D1, D2, D9, D6: the happy path, then reuse.
func TestDeviceFlowApproveCollectOnce(t *testing.T) {
	h, priv := testHandlerEnv(t)
	h.config.RPOrigins = []string{"https://choir.news"}
	u, _ := h.store.CreateUser("device-user", "device@example.com")

	b := deviceBegin(t, h, "laptop agent", "203.0.113.7")
	if !strings.HasPrefix(b.DeviceCode, DeviceCodePrefix) || len(b.DeviceCode) < 40 {
		t.Fatalf("device code %q", b.DeviceCode)
	}
	if len(b.UserCode) != 9 || b.UserCode[4] != '-' {
		t.Fatalf("user code %q", b.UserCode)
	}
	if b.VerificationURI != "https://choir.news/device" || b.VerificationURIComplete != "https://choir.news/device?code="+b.UserCode {
		t.Fatalf("uris %q %q", b.VerificationURI, b.VerificationURIComplete)
	}
	if b.ExpiresIn != int(DeviceAuthorizationTTL/time.Second) || b.Interval != DevicePollIntervalSeconds {
		t.Fatalf("expires_in %d interval %d", b.ExpiresIn, b.Interval)
	}
	// D1: neither code is stored in plain text.
	var stored string
	_ = h.store.DB().QueryRow("SELECT device_code_hash || user_code_hash FROM device_authorizations").Scan(&stored)
	if strings.Contains(stored, b.DeviceCode) || strings.Contains(stored, strings.ReplaceAll(b.UserCode, "-", "")) {
		t.Fatal("codes stored in plain text")
	}
	b2 := deviceBegin(t, h, "other", "203.0.113.7")
	if b2.DeviceCode == b.DeviceCode || b2.UserCode == b.UserCode {
		t.Fatal("codes repeat")
	}

	// D2: pending.
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "authorization_pending" {
		t.Fatalf("pending poll: %d %v", code, m)
	}

	// Lookup shows who is asking. The code is accepted in any case, with or without the dash.
	rec := httptest.NewRecorder()
	q := url.Values{"user_code": {strings.ToLower(strings.ReplaceAll(b.UserCode, "-", ""))}}
	h.HandleDeviceLookup(rec, passkeyCookieReq(http.MethodGet, "/auth/device/lookup?"+q.Encode(), nil, priv, u.ID))
	if rec.Code != http.StatusOK {
		t.Fatalf("lookup %d: %s", rec.Code, rec.Body.String())
	}
	look := decodeBody(t, rec)
	if look["label"] != "laptop agent" || look["status"] != "pending" || !strings.Contains(fmt.Sprint(look["requested_from"]), "203.0.113.x") {
		t.Fatalf("lookup body %v", look)
	}

	// Approve.
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "approve"}, priv, u.ID)); rec.Code != http.StatusOK {
		t.Fatalf("approve %d: %s", rec.Code, rec.Body.String())
	}
	// A second decision on the same code is refused.
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "deny"}, priv, u.ID)); rec.Code != http.StatusNotFound {
		t.Fatalf("second decision %d, want 404", rec.Code)
	}

	// D9: the user code alone collects nothing.
	rewindPoll(t, h)
	if code, m := devicePoll(h, b.UserCode); code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("user code as device code: %d %v", code, m)
	}

	rewindPoll(t, h)
	code, m := devicePoll(h, b.DeviceCode)
	if code != http.StatusOK {
		t.Fatalf("collect %d %v", code, m)
	}
	secret, _ := m["api_key"].(string)
	if !strings.HasPrefix(secret, APIKeyPrefix) {
		t.Fatalf("api_key %q", secret)
	}
	ak, err := h.store.GetAPIKeyByHash(context.Background(), hashAPIKeySecret(secret))
	if err != nil {
		t.Fatal(err)
	}
	if ak.UserID != u.ID || ak.ID != m["key_id"] || ak.Label != "choir login: laptop agent" || ak.ExpiresAt != nil {
		t.Fatalf("key %+v body %v", ak, m)
	}
	got := append([]string(nil), ak.Scopes...)
	sort.Strings(got)
	if strings.Join(got, ",") != strings.Join(AllScopes(), ",") {
		t.Fatalf("device key scopes %v", got)
	}

	// D6: reuse.
	rewindPoll(t, h)
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "invalid_grant" {
		t.Fatalf("reuse: %d %v", code, m)
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM api_keys WHERE user_id = ?", u.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("%d keys minted, want 1", n)
	}
}

// D4: denied.
func TestDeviceFlowDenied(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("deny-user", "deny@example.com")
	b := deviceBegin(t, h, "x", "")
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "deny"}, priv, u.ID)); rec.Code != http.StatusOK {
		t.Fatalf("deny %d: %s", rec.Code, rec.Body.String())
	}
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "access_denied" {
		t.Fatalf("denied poll: %d %v", code, m)
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&n)
	if n != 0 {
		t.Fatal("a denied request minted a key")
	}
}

// D5: expired, both before and after approval.
func TestDeviceFlowExpired(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("exp-user", "exp@example.com")
	pending := deviceBegin(t, h, "a", "")
	approved := deviceBegin(t, h, "b", "")
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": approved.UserCode, "decision": "approve"}, priv, u.ID)); rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if _, err := h.store.DB().Exec("UPDATE device_authorizations SET expires_at = ?", time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{pending.DeviceCode, approved.DeviceCode} {
		if code, m := devicePoll(h, d); code != http.StatusBadRequest || m["error"] != "expired_token" {
			t.Fatalf("expired poll: %d %v", code, m)
		}
	}
	// An expired code cannot be approved.
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": pending.UserCode, "decision": "approve"}, priv, u.ID)); rec.Code != http.StatusNotFound {
		t.Fatalf("approve expired %d", rec.Code)
	}
	var n int
	_ = h.store.DB().QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&n)
	if n != 0 {
		t.Fatal("an expired request minted a key")
	}
}

// D3: who may approve.
func TestDeviceApproveNeedsPasskeyCookieAndJSON(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("gate-user", "gate@example.com")
	b := deviceBegin(t, h, "x", "")
	body := map[string]string{"user_code": b.UserCode, "decision": "approve"}

	// No session.
	if rec := deviceDecide(h, jsonReq(http.MethodPost, "/auth/device/approve", body, "")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d", rec.Code)
	}
	// An API key, even a full one.
	_, secret, _ := h.store.CreateAPIKey(context.Background(), u.ID, "full", AllScopes(), nil)
	req := jsonReq(http.MethodPost, "/auth/device/approve", body, "")
	req.Header.Set("Authorization", "Bearer "+secret)
	if rec := deviceDecide(h, req); rec.Code != http.StatusUnauthorized {
		t.Fatalf("api key: %d", rec.Code)
	}
	// An email session.
	if rec := deviceDecide(h, emailSessionCookieReq(http.MethodPost, "/auth/device/approve", body, priv, u.ID)); rec.Code != http.StatusForbidden {
		t.Fatalf("email session: %d", rec.Code)
	}
	// A form post (cross-site CSRF shape).
	form := httptest.NewRequest(http.MethodPost, "/auth/device/approve", bytes.NewBufferString(`{"user_code":"`+b.UserCode+`","decision":"approve"}`))
	form.Header.Set("Content-Type", "text/plain")
	form.AddCookie(&http.Cookie{Name: AccessTokenCookieName, Value: issueTestAccessJWTForHandler(priv, u.ID)})
	if rec := deviceDecide(h, form); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain: %d", rec.Code)
	}
	// A bad decision word.
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "yes"}, priv, u.ID)); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad decision: %d", rec.Code)
	}
	// Still pending after all of that.
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "authorization_pending" {
		t.Fatalf("after refused approvals: %d %v", code, m)
	}
	// Lookup too: email session refused.
	rec := httptest.NewRecorder()
	h.HandleDeviceLookup(rec, emailSessionCookieReq(http.MethodGet, "/auth/device/lookup?user_code="+b.UserCode, nil, priv, u.ID))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("email lookup: %d", rec.Code)
	}
}

// D7: slow_down.
func TestDevicePollSlowDown(t *testing.T) {
	h, _ := testHandlerEnv(t)
	b := deviceBegin(t, h, "x", "")
	if _, m := devicePoll(h, b.DeviceCode); m["error"] != "authorization_pending" {
		t.Fatalf("first poll %v", m)
	}
	if code, m := devicePoll(h, b.DeviceCode); code != http.StatusBadRequest || m["error"] != "slow_down" {
		t.Fatalf("fast poll: %d %v", code, m)
	}
}

// D8: unknown-code lookups are limited per user.
func TestDeviceLookupLimitsGuesses(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("guess-user", "guess@example.com")
	b := deviceBegin(t, h, "x", "")
	for i := 0; i < DeviceMaxLookupMissesPerUser; i++ {
		rec := httptest.NewRecorder()
		h.HandleDeviceLookup(rec, passkeyCookieReq(http.MethodGet, "/auth/device/lookup?user_code=BCDF-GHJK", nil, priv, u.ID))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("miss %d: %d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.HandleDeviceLookup(rec, passkeyCookieReq(http.MethodGet, "/auth/device/lookup?user_code="+b.UserCode, nil, priv, u.ID))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after misses, a real code: %d, want 429", rec.Code)
	}
	if rec := deviceDecide(h, passkeyCookieReq(http.MethodPost, "/auth/device/approve", map[string]string{"user_code": b.UserCode, "decision": "approve"}, priv, u.ID)); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("approve after misses: %d, want 429", rec.Code)
	}
}

// D10: the begin limit per IP.
func TestDeviceBeginLimitPerIP(t *testing.T) {
	h, _ := testHandlerEnv(t)
	for i := 0; i < DeviceMaxBeginPerIP; i++ {
		deviceBegin(t, h, "x", "198.51.100.9")
	}
	rec := httptest.NewRecorder()
	h.HandleDeviceBegin(rec, jsonReq(http.MethodPost, "/auth/device/begin", map[string]string{"label": "x"}, "198.51.100.9"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("begin over limit: %d", rec.Code)
	}
	deviceBegin(t, h, "x", "198.51.100.10")
}

// The label is bounded and stripped of control characters; empty gets a default.
func TestDeviceBeginLabel(t *testing.T) {
	h, priv := testHandlerEnv(t)
	u, _ := h.store.CreateUser("label-user", "label@example.com")
	b := deviceBegin(t, h, "evil\x1b[31m"+strings.Repeat("a", 200), "")
	rec := httptest.NewRecorder()
	h.HandleDeviceLookup(rec, passkeyCookieReq(http.MethodGet, "/auth/device/lookup?user_code="+b.UserCode, nil, priv, u.ID))
	label := fmt.Sprint(decodeBody(t, rec)["label"])
	if strings.ContainsAny(label, "\x1b") || len(label) > deviceLabelMax {
		t.Fatalf("label %q", label)
	}
	b = deviceBegin(t, h, "", "")
	rec = httptest.NewRecorder()
	h.HandleDeviceLookup(rec, passkeyCookieReq(http.MethodGet, "/auth/device/lookup?user_code="+b.UserCode, nil, priv, u.ID))
	if decodeBody(t, rec)["label"] != "choir CLI" {
		t.Fatalf("default label: %s", rec.Body.String())
	}
}
