package proxy

// An account that asked to be deleted loses access at that moment, though its
// access token is signed and unexpired (docs/design/account-create-delete-
// 2026-11-23.md; failure 5 of internal/auth/deletion_handlers.go).

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeletionRecordRevokesSignedTokenAndKeyAtTheGate(t *testing.T) {
	env := newOwnerActEnv(t)
	keySecret := env.key(t, "read:runtime")
	jwtReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
		r.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(env.privateKey, env.user.ID)})
		return r
	}
	keyReq := func() *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
		r.Header.Set("Authorization", "Bearer "+keySecret)
		return r
	}
	if _, err := env.handler.authenticate(jwtReq()); err != nil {
		t.Fatalf("token before deletion: %v", err)
	}
	if _, err := env.handler.authenticate(keyReq()); err != nil {
		t.Fatalf("key before deletion: %v", err)
	}
	if _, _, err := env.store.RequestAccountDeletion(env.user.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := env.handler.authenticate(jwtReq()); err == nil {
		t.Fatal("a signed, unexpired access token still authenticates after the deletion request")
	}
	// The key is revoked by the same transaction; even a key row the revoke
	// missed is refused by the record.
	if _, err := env.store.DB().Exec(`UPDATE api_keys SET revoked_at = NULL`); err != nil {
		t.Fatal(err)
	}
	if _, err := env.handler.authenticate(keyReq()); err == nil {
		t.Fatal("a key still authenticates after the deletion request")
	}
	rec := httptest.NewRecorder()
	env.handler.HandleAPI(rec, jwtReq())
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("the API answers %d to a revoked account", rec.Code)
	}
}
