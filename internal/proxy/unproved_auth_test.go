package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// The proxy accepts what the auth service would accept, and no more: a token with no proof claim
// ("ev") and the key of an account whose address is not proved are both refused on /api/*. The proxy
// holds its own token check (it reads no user row), so each half is pinned here.

func TestAnAccessCookieWithoutTheProofClaimIsRefused(t *testing.T) {
	h, priv, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("ev-less", "ev-less@example.com")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"sub": user.ID, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(), "ev": now.Unix(), "scope": "access"})
	legacy := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{"sub": user.ID, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(), "scope": "access"})
	for name, tok := range map[string]*jwt.Token{"with ev": ok, "without ev": legacy} {
		signed, err := tok.SignedString(priv)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: signed})
		rec := httptest.NewRecorder()
		h.HandleBootstrap(rec, req)
		if name == "without ev" && rec.Code != http.StatusUnauthorized {
			t.Fatalf("a correctly signed access cookie with no ev claim answered %d, want 401", rec.Code)
		}
		if name == "with ev" && rec.Code == http.StatusUnauthorized {
			t.Fatalf("the same token with ev was refused: the test would pass for the wrong reason")
		}
	}
}

func TestTheKeyOfAnUnprovedAccountIsRefused(t *testing.T) {
	h, _, _, store := testProxyEnvWithAuthStore(t)
	user, err := store.CreateUser("unproved-key", "unproved-key@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateAPIKey(t.Context(), user.ID, "old key", []string{"read:runtime"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
		req.Header.Set("Authorization", "Bearer "+secret)
		rec := httptest.NewRecorder()
		h.HandleBootstrap(rec, req)
		return rec.Code
	}
	if call() == http.StatusUnauthorized {
		t.Fatal("the key of a proved account was refused: the test would pass for the wrong reason")
	}
	if _, err := store.DB().Exec(`UPDATE users SET email_verified_at = NULL WHERE id = ?`, user.ID); err != nil {
		t.Fatal(err)
	}
	if got := call(); got != http.StatusUnauthorized {
		t.Fatalf("the key of an unproved account answered %d, want 401", got)
	}
}
