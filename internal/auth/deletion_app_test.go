package auth

// The phone's whole path through account deletion, with the same software
// passkey the app ceremony tests use: create an account, delete it with the
// key and a fresh assertion, find the app locked out, sign in again inside the
// 72 hours and cancel, and be handed a new key.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func authedPost(h http.HandlerFunc, path string, body []byte, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestPhoneDeletesItsAccountAndCancelsByASignIn(t *testing.T) {
	h := appEnv(t)
	host := &fakeDeletionHost{}
	h.SetDeletionHost(host)
	p := newSoftPasskey(t)
	created := decodeMap(t, createAccount(t, h, "phone@example.com", p, testOrigin))
	key := created["api_key"].(string)
	user, _ := h.store.GetUserByEmail("phone@example.com")

	// The key alone starts nothing.
	begin := authedPost(h.HandleDeleteBegin, "/auth/account/delete/begin", nil, key)
	if begin.Code != 200 {
		t.Fatalf("begin: %d %s", begin.Code, begin.Body)
	}
	if rec := authedPost(h.HandleDeleteFinish, "/auth/account/delete/finish", []byte(`{}`), key); rec.Code == 200 {
		t.Fatal("a key with no assertion started a deletion")
	}
	// A fresh begin (the failed finish consumed nothing but the body was empty), then the assertion.
	begin = authedPost(h.HandleDeleteBegin, "/auth/account/delete/begin", nil, key)
	assertion := p.assert(t, decodeMap(t, begin), testOrigin)
	fin := authedPost(h.HandleDeleteFinish, "/auth/account/delete/finish", assertion, key)
	if fin.Code != 200 {
		t.Fatalf("finish: %d %s", fin.Code, fin.Body)
	}
	if !h.store.AccountAccessRevoked(user.ID) {
		t.Fatal("not revoked")
	}
	// The old key is dead.
	who := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/auth/whoami", nil)
	r.Header.Set("Authorization", "Bearer "+key)
	h.HandleWhoami(who, r)
	if who.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key: %d", who.Code)
	}

	// Signing in again mints no key; it offers the cancel.
	keysBefore := countKeys(t, h, user.ID)
	lb := postJSON(h.HandleAppLoginBegin, map[string]string{})
	cred := p.assert(t, decodeMap(t, lb), testOrigin)
	lf := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, cred))
	if lf.Code != http.StatusForbidden || countKeys(t, h, user.ID) != keysBefore {
		t.Fatalf("app sign-in of a pending account: %d keys %d -> %d %s", lf.Code, keysBefore, countKeys(t, h, user.ID), lf.Body)
	}
	offer := decodeMap(t, lf)
	token, _ := offer["cancel_token"].(string)
	if token == "" || offer["error"] != "account_deletion_pending" || offer["purge_after"] == "" {
		t.Fatalf("offer: %v", offer)
	}
	// The cancel, asking for the app's key.
	body, _ := json.Marshal(map[string]string{"cancel_token": token, "app": testApp})
	c := postJSON(h.HandleDeleteCancel, body)
	if c.Code != 200 {
		t.Fatalf("cancel: %d %s", c.Code, c.Body)
	}
	assertNoCookies(t, c)
	newKey, _ := decodeMap(t, c)["api_key"].(string)
	if !strings.HasPrefix(newKey, APIKeyPrefix) {
		t.Fatal("no key after cancel")
	}
	if h.store.AccountAccessRevoked(user.ID) {
		t.Fatal("still revoked after cancel")
	}
	who = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/auth/whoami", nil)
	r.Header.Set("Authorization", "Bearer "+newKey)
	h.HandleWhoami(who, r)
	if who.Code != http.StatusOK {
		t.Fatalf("the new key: %d", who.Code)
	}
	// An unknown app name gets nothing.
	again := postJSON(h.HandleDeleteCancel, []byte(`{"cancel_token":"x","app":"evil.app"}`))
	if again.Code == 200 {
		t.Fatal("cancel accepted a junk token")
	}
	_ = time.Now
}

// A purging account cannot mint a key at all.
func TestPurgingAccountMintsNoKey(t *testing.T) {
	h := appEnv(t)
	p := newSoftPasskey(t)
	created := decodeMap(t, createAccount(t, h, "gone@example.com", p, testOrigin))
	_ = created
	user, _ := h.store.GetUserByEmail("gone@example.com")
	now := time.Now().UTC().Add(-100 * time.Hour)
	if _, _, err := h.store.RequestAccountDeletion(user.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.BeginPurge(user.ID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	before := countKeys(t, h, user.ID)
	lb := postJSON(h.HandleAppLoginBegin, map[string]string{})
	lf := postJSON(h.HandleAppLoginFinish, appFinishBody(testApp, p.assert(t, decodeMap(t, lb), testOrigin)))
	if lf.Code != http.StatusForbidden || countKeys(t, h, user.ID) != before {
		t.Fatalf("purging sign-in: %d %s", lf.Code, lf.Body)
	}
	if strings.Contains(lf.Body.String(), `"cancel_token":"`) && !strings.Contains(lf.Body.String(), `"cancel_token":""`) {
		t.Fatal("a purging account was offered a cancel")
	}
}
