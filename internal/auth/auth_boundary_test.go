package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Failure modes pinned here
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F3, F4):
//
//	a key without manage:keys lists its siblings' labels, scopes and
//	bindings; the list is unbounded; a zero-scope key is minted; a key
//	(admin included) mints or delegates owner:acts; the owner's session
//	cannot mint owner:acts; manage:keys loses the full list.

func bearerReq(method, path string, body []byte, secret string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+secret)
	return req
}

func decodeKeyList(t *testing.T, rec *httptest.ResponseRecorder) listAPIKeysResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp listAPIKeysResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAPIKeyListIsScopedAndBounded(t *testing.T) {
	h, priv := testHandlerEnv(t)
	ctx := context.Background()
	user, err := h.store.CreateUser("key-list-owner", "key-list-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	agentID, agentSecret, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, "agent", []string{"precommit:write"}, "computer-a", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, managerSecret, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, "manager", []string{"manage:keys"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, _, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, fmt.Sprintf("sibling-%d", i), []string{"admin"}, "computer-a", nil); err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	h.HandleListAPIKeys(rec, bearerReq(http.MethodGet, "/auth/api-keys", nil, agentSecret))
	agentView := decodeKeyList(t, rec)
	if len(agentView.Keys) != 1 || agentView.Keys[0].ID != agentID {
		t.Fatalf("F3: a precommit-only key listed %d keys, want only itself: %+v", len(agentView.Keys), agentView.Keys)
	}

	rec = httptest.NewRecorder()
	h.HandleListAPIKeys(rec, bearerReq(http.MethodGet, "/auth/api-keys", nil, managerSecret))
	if got := len(decodeKeyList(t, rec).Keys); got != 5 {
		t.Fatalf("manage:keys list = %d keys, want all 5", got)
	}

	rec = httptest.NewRecorder()
	h.HandleListAPIKeys(rec, authedAPIKeyReq(http.MethodGet, "/auth/api-keys?limit=2", nil, priv, user.ID))
	if got := len(decodeKeyList(t, rec).Keys); got != 2 {
		t.Fatalf("F3: limit=2 returned %d keys", got)
	}
	for _, bad := range []string{"0", "-1", "501", "x"} {
		rec = httptest.NewRecorder()
		h.HandleListAPIKeys(rec, authedAPIKeyReq(http.MethodGet, "/auth/api-keys?limit="+bad, nil, priv, user.ID))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("F3: limit=%s status = %d, want 400", bad, rec.Code)
		}
	}
}

func TestAPIKeyCreateRefusesZeroScopesAndKeyMintedOwnerActs(t *testing.T) {
	h, priv := testHandlerEnv(t)
	ctx := context.Background()
	user, err := h.store.CreateUser("owner-acts-owner", "owner-acts-owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	create := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.HandleCreateAPIKey(rec, req)
		return rec
	}

	empty := []byte(`{"label":"empty","scopes":[]}`)
	if rec := create(authedAPIKeyReq(http.MethodPost, "/auth/api-keys", bytes.NewReader(empty), priv, user.ID)); rec.Code != http.StatusBadRequest {
		t.Fatalf("F3: zero-scope key = %d %s, want 400", rec.Code, rec.Body.String())
	}

	ownerActs := []byte(`{"label":"owner","scopes":["computer:self_development:approve","owner:acts"],"computer_id":"computer-a"}`)
	_, adminSecret, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, "admin", []string{"admin"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, managerSecret, err := h.store.CreateComputerScopedAPIKey(ctx, user.ID, "manager", []string{"manage:keys", "computer:self_development:approve", OwnerActsScope}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, secret := range map[string]string{"admin": adminSecret, "manage:keys holding owner:acts": managerSecret} {
		if rec := create(bearerReq(http.MethodPost, "/auth/api-keys", ownerActs, secret)); rec.Code != http.StatusForbidden {
			t.Fatalf("F4: %s key minted owner:acts: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	// An admin key still delegates ordinary scopes.
	plain := []byte(`{"label":"agent","scopes":["computer:self_development:approve"],"computer_id":"computer-a"}`)
	if rec := create(bearerReq(http.MethodPost, "/auth/api-keys", plain, adminSecret)); rec.Code != http.StatusCreated {
		t.Fatalf("admin delegation of an ordinary scope = %d %s", rec.Code, rec.Body.String())
	}
	// The owner's session mints owner:acts.
	rec := create(authedAPIKeyReq(http.MethodPost, "/auth/api-keys", bytes.NewReader(ownerActs), priv, user.ID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("F4: owner session could not mint owner:acts: %d %s", rec.Code, rec.Body.String())
	}
	var created createAPIKeyResponse
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil || !apiKeyHasScope(created.Scopes, OwnerActsScope) {
		t.Fatalf("owner-minted key scopes = %v (%v)", created.Scopes, err)
	}
}
