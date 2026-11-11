package auth

// GET /auth/whoami: who a bearer key is (Autoradio lane S, item 2;
// docs/design/autoradio-app-ui-2026-11-10.md, server gap 2). GET /auth/session
// is cookie-only, so a phone signed in by the device flow had no route to ask
// "which account is this key". The answer is the key's own record (what GET
// /auth/api-keys already returns a key about itself) and the public byline
// handle. It carries neither the email nor the owner's internal user id: a
// low-scope key (a claim key, read:base) must not learn it. The call touches
// the key's last-used time, so it is not strictly read-only. Ways it can
// fail, written before the code:
//
//  W1 It answers a cookie session or no credential (the route is the bearer's
//     only; a browser has /auth/session), or a revoked or expired key.
//  W2 It leaks the email address, its domain, or any sibling key's label,
//     scope or binding.
//  W3 The handle differs from the byline a published opinion carries, so the
//     phone shows one name for the same account.
//  W4 It writes: any method but GET changes state or is not refused.
//  W5 The response is cached by an intermediary.
//  W6 A valid cookie beside a bogus or absent bearer is taken as the
//     credential and answers an empty 200: the bearer itself is validated.
//  W7 The owner's internal user id appears in the answer.

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

// PublicHandle is an account's public handle: a short stable label derived
// from the user id. The auth store holds no display name and the email is
// never shown. It is the byline of a published opinion piece.
func PublicHandle(userID string) string {
	sum := sha256.Sum256([]byte("choir-opinion-author:" + strings.TrimSpace(userID)))
	return "writer-" + hex.EncodeToString(sum[:])[:6]
}

type whoamiResponse struct {
	Handle string `json:"handle"`
	KeyID  string `json:"key_id"`
	// KeyLabel and Scopes are the calling key's own record.
	KeyLabel string   `json:"key_label"`
	Scopes   []string `json:"scopes"`
	// ComputerID is the one computer the key is bound to; empty means an
	// owner-wide key, whose computers the app finds as it does today.
	ComputerID string `json:"computer_id"`
}

// HandleWhoami handles GET /auth/whoami for a Bearer API key.
func (h *Handler) HandleWhoami(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store") // every answer, the refusals too
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var key *APIKey
	if hasAPIKeyBearer(r) {
		if token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")); token != "" {
			ak, err := h.store.GetAPIKeyByHash(r.Context(), hashAPIKeySecret(token))
			if err == nil && ak != nil && ak.RevokedAt == nil && (ak.ExpiresAt == nil || ak.ExpiresAt.After(time.Now())) {
				if _, err := h.store.GetUserByID(ak.UserID); err == nil {
					key = ak
				}
			}
		}
	}
	if key == nil { // W1, W6: only the bearer itself is a credential here
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "a valid bearer key is required; a browser session reads /auth/session"})
		return
	}
	_ = h.store.TouchAPIKeyLastUsed(r.Context(), key.ID)
	scopes := key.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	writeJSON(w, http.StatusOK, whoamiResponse{
		Handle:     PublicHandle(key.UserID),
		KeyID:      key.ID,
		KeyLabel:   key.Label,
		Scopes:     scopes,
		ComputerID: key.ComputerID,
	})
}
