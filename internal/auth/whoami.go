package auth

// GET /auth/whoami: who a bearer key is (Autoradio lane S, item 2;
// docs/design/autoradio-app-ui-2026-11-10.md, server gap 2). GET /auth/session
// is cookie-only, so a phone signed in by the device flow had no route to ask
// "which account is this key". The answer holds nothing the key could not
// already read: its own key record (GET /auth/api-keys returns a key its own
// record), the owner's id (every owner route is keyed by it), and the public
// byline handle. The email is not in it. Ways it can fail, written before
// the code:
//
//  W1 It answers a cookie session or no credential (the route is the bearer's
//     only; a browser has /auth/session), or a revoked or expired key.
//  W2 It leaks the email address, its domain, or any sibling key's label,
//     scope or binding.
//  W3 The handle differs from the byline a published opinion carries, so the
//     phone shows one name for the same account.
//  W4 It writes: any method but GET changes state or is not refused.
//  W5 The response is cached by an intermediary.

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// PublicHandle is an account's public handle: a short stable label derived
// from the user id. The auth store holds no display name and the email is
// never shown. It is the byline of a published opinion piece.
func PublicHandle(userID string) string {
	sum := sha256.Sum256([]byte("choir-opinion-author:" + strings.TrimSpace(userID)))
	return "writer-" + hex.EncodeToString(sum[:])[:6]
}

type whoamiResponse struct {
	UserID string `json:"user_id"`
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
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !hasAPIKeyBearer(r) {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "a bearer key is required; a browser session reads /auth/session"})
		return
	}
	user, key := h.requireAuthUserAny(w, r)
	if user == nil || key == nil {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	scopes := key.Scopes
	if scopes == nil {
		scopes = []string{}
	}
	writeJSON(w, http.StatusOK, whoamiResponse{
		UserID:     user.ID,
		Handle:     PublicHandle(user.ID),
		KeyID:      key.ID,
		KeyLabel:   key.Label,
		Scopes:     scopes,
		ComputerID: key.ComputerID,
	})
}
