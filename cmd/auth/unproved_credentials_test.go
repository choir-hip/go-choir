package main

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/yusefmosiah/go-choir/internal/auth"
)

// An account whose address was never proved acts through nothing. The rule is
// enforced where a credential is validated (the access-token validator, the API-key lookup,
// the refresh-session lookup, the device and desktop grant redemptions), so this test does not
// name handlers: it takes every route in the route table, and for every kind of credential an unproved
// account can hold it asserts the answer is the answer an anonymous caller gets. A new route is covered by
// being in the table.

func sha(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

type unprovedEnv struct {
	h       *auth.Handler
	store   *auth.Store
	userID  string
	secrets map[string]string // credential kind -> secret
}

func newUnprovedEnv(t *testing.T) *unprovedEnv {
	t.Helper()
	dir := t.TempDir()
	store, err := auth.OpenStore(filepath.Join(dir, "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := &auth.Config{
		Port: "0", DBPath: filepath.Join(dir, "auth.db"), RPID: "localhost", RPOrigins: []string{"http://localhost:4173"},
		JWTPrivateKeyPath: filepath.Join(dir, "jwt-key"), AccessTokenTTL: 5 * time.Minute, RefreshTokenTTL: 720 * time.Hour,
	}
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	wa, err := webauthn.New(&webauthn.Config{RPID: cfg.RPID, RPDisplayName: "test", RPOrigins: cfg.RPOrigins})
	if err != nil {
		t.Fatal(err)
	}
	h := auth.NewHandler(store, wa, cfg, priv)
	ctx := context.Background()
	now := time.Now().UTC()

	// The account is made as if before the proof rule: it holds a passkey, a key, a session,
	// a handoff code and an approved device login. Then its address is marked unproved.
	u, err := store.CreateUser("u-unproved", "unproved@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateCredential(&auth.Credential{ID: "c1", UserID: u.ID, PublicKey: []byte("k"), AttestationType: "none", Transport: "[]", AAGUID: make([]byte, 16), Flags: "{}", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	_, key, err := store.CreateAPIKey(ctx, u.ID, "old key", auth.AllScopes(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRefreshSession(&auth.RefreshSession{ID: "rs1", UserID: u.ID, TokenHash: sha("refresh-secret"), CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateDesktopExchangeCode(&auth.DesktopExchangeCode{Code: "desktop-code", UserID: u.ID, CreatedAt: now, ExpiresAt: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	dc := auth.DeviceCodePrefix + "device-secret-0123456789012345678901234567890"
	if err := store.CreateDeviceAuthorization(ctx, &auth.DeviceAuthorization{ID: "da1", DeviceCodeHash: sha(dc), UserCodeHash: sha("UC"), IPHash: "ip", CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideDeviceAuthorization(ctx, "da1", u.ID, true, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	sign := func(extra jwt.MapClaims) string {
		c := jwt.MapClaims{"sub": u.ID, "email": u.Email, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(), "scope": "access", "am": "passkey"}
		for k, v := range extra {
			c[k] = v
		}
		s, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(priv)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return &unprovedEnv{h: h, store: store, userID: u.ID, secrets: map[string]string{
		"legacy access token (no ev claim)": sign(nil),
		"access token that carries ev":      sign(jwt.MapClaims{"ev": now.Unix()}),
		"api key":                           key,
		"refresh cookie":                    "refresh-secret",
		"device code":                       dc,
		"desktop exchange code":             "desktop-code",
	}}
}

// footprint is what the account holds, counted: a route driven with its credential must leave it as it was.
func (e *unprovedEnv) footprint(t *testing.T) string {
	t.Helper()
	var out []string
	for _, q := range []string{"refresh_sessions", "api_keys", "desktop_exchange_codes", "device_authorizations", "credentials", "account_set_member"} {
		col := "user_id"
		var n int
		if err := e.store.DB().QueryRow("SELECT COUNT(*) FROM "+q+" WHERE "+col+" = ?", e.userID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%s=%d", q, n))
	}
	return strings.Join(out, " ")
}

func call(h http.HandlerFunc, method, path string, mut func(*http.Request), body string) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if mut != nil {
		mut(req)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec.Code, rec.Body.String()
}

func TestAnUnprovedAccountActsThroughNoRoute(t *testing.T) {
	e := newUnprovedEnv(t)
	for _, rt := range authRoutes(e.h) {
		// Every pattern carries its method: nothing is inferred, so a POST route is driven as a POST.
		i := strings.IndexByte(rt.pattern, ' ')
		if i <= 0 {
			t.Fatalf("route %q has no method; put it on the pattern", rt.pattern)
		}
		method, path := rt.pattern[:i], rt.pattern[i+1:]
		path = strings.ReplaceAll(path, "{id}", "x")
		// A route with a switch in its query is driven both ways: logging out of every device identifies the account and acts on it.
		for _, suffix := range append([]string{""}, routeVariants[strings.SplitN(path, "?", 2)[0]]...) {
			e.driveRoute(t, rt, method, path+suffix)
		}
	}
}

// routeVariants are the query strings a route is also driven with.
var routeVariants = map[string][]string{"/auth/logout": {"?all=true"}}

func (e *unprovedEnv) driveRoute(t *testing.T, rt authRoute, method, path string) {
	t.Helper()
	{
		anonCode, _ := call(rt.handler, method, path, nil, "")
		// What a credential that is simply not valid gets: some routes answer by the shape
		// of the credential (a bearer header is refused by routes that are cookie-only).
		garbage := func(kind string) (int, string) {
			var mut func(*http.Request)
			switch kind {
			case "api key":
				mut = func(r *http.Request) { r.Header.Set("Authorization", "Bearer choir_sk_not-a-key") }
			case "refresh cookie":
				mut = func(r *http.Request) {
					r.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookieName, Value: "not-a-session"})
				}
			default:
				mut = func(r *http.Request) {
					r.AddCookie(&http.Cookie{Name: auth.AccessTokenCookieName, Value: "not.a.token"})
				}
			}
			return call(rt.handler, method, path, mut, "")
		}
		for kind, secret := range e.secrets {
			secret := secret
			var mut func(*http.Request)
			var body string
			switch kind {
			case "api key":
				mut = func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+secret) }
			case "refresh cookie":
				mut = func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.RefreshTokenCookieName, Value: secret}) }
			case "device code":
				body = `{"device_code":"` + secret + `"}`
			case "desktop exchange code":
				body = `{"code":"` + secret + `"}`
			default:
				mut = func(r *http.Request) { r.AddCookie(&http.Cookie{Name: auth.AccessTokenCookieName, Value: secret}) }
			}
			// A grant is redeemed only at its own route, by a POST.
			if kind == "device code" || kind == "desktop exchange code" {
				if (kind == "device code" && !strings.HasSuffix(path, "/auth/device/token")) || (kind == "desktop exchange code" && !strings.HasSuffix(path, "/auth/desktop/redeem")) {
					continue
				}
				code, got := call(rt.handler, "POST", path, nil, body)
				if code/100 == 2 || strings.Contains(got, "api_key") || strings.Contains(got, "unproved@example.com") {
					t.Errorf("%s redeemed an unproved account's %s: %d %.120s", rt.pattern, kind, code, got)
				}
				continue
			}
			before := e.footprint(t)
			code, got := call(rt.handler, method, path, mut, body)
			if after := e.footprint(t); after != before {
				t.Errorf("%s with an unproved account's %s changed its rows: %s, then %s", rt.pattern+strings.TrimPrefix(path, strings.SplitN(path, "?", 2)[0]), kind, before, after)
			}
			if strings.Contains(got, "choir_sk_") || strings.Contains(got, `"authenticated":true`) || strings.Contains(got, "unproved@example.com") {
				t.Errorf("%s with an unproved account's %s answered with the account: %d %.120s", rt.pattern, kind, code, got)
				continue
			}
			if wantCode, wantBody := garbage(kind); code != wantCode && code != anonCode {
				t.Errorf("%s with an unproved account's %s answered %d, a credential that is not valid gets %d (%.80s / %.80s)", rt.pattern, kind, code, wantCode, got, wantBody)
			}
		}
	}
}

// The table has a route for every grant redemption the test above reaches by name.
func TestTheRouteTableHasTheRedemptionRoutes(t *testing.T) {
	e := newUnprovedEnv(t)
	have := map[string]bool{}
	for _, rt := range authRoutes(e.h) {
		have[rt.pattern] = true
	}
	for _, want := range []string{"POST /auth/device/token", "POST /auth/desktop/redeem", "POST /auth/logout", "POST /auth/accounts/switch", "GET /auth/whoami", "POST /auth/api-keys"} {
		if !have[want] {
			t.Errorf("route table lacks %q", want)
		}
	}
}
