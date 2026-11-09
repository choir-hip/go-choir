package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Account switcher (docs/design/account-switcher-2026-11-09.md). One browser
// holds several signed-in accounts in an account set. The set is named by the
// opaque choir_accounts cookie; the database holds only its SHA-256.

// AccountsCookieName is the cookie that names the browser's account set.
const AccountsCookieName = "choir_accounts"

// AccountsCookieTTL is the choir_accounts cookie lifetime, refreshed at every
// sign-in and switch.
const AccountsCookieTTL = AccountSetIdleTTL

// ReasonAccountSwitched is the machine-readable refusal for a request that
// carries a non-active member's credentials.
const ReasonAccountSwitched = "account_switched"

// ReasonMemberSignedOut is the refusal for a switch to a member whose
// session is dead.
const ReasonMemberSignedOut = "member_signed_out"

// setAccountsCookie writes the set cookie. Path=/auth like choir_refresh: it
// never reaches the proxy (r3 N5); the value is opaque.
func (h *Handler) setAccountsCookie(w http.ResponseWriter, raw string) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccountsCookieName,
		Value:    raw,
		Path:     "/auth",
		MaxAge:   int(AccountsCookieTTL.Seconds()),
		Secure:   h.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearAccountsCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccountsCookieName,
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1,
		Secure:   h.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   h.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshTokenCookieName,
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1,
		Secure:   h.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func newAccountsHandle() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func accountsHandleHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// requestAccountHandleHash returns the hash of the set cookie the request
// carries, or "".
func requestAccountHandleHash(r *http.Request) string {
	c, err := r.Cookie(AccountsCookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return accountsHandleHash(c.Value)
}

// requestAccountSet resolves the set the request's choir_accounts cookie
// names, or nil.
func (h *Handler) requestAccountSet(r *http.Request) *AccountSet {
	hash := requestAccountHandleHash(r)
	if hash == "" {
		return nil
	}
	set, err := h.store.GetAccountSetByHandleHash(hash)
	if err != nil {
		return nil
	}
	return set
}

// addToAccountSet adds the account whose session was just issued to the
// browser's set (creating it), and writes the set cookie. A failure is logged
// and never fails the sign-in: the browser keeps today's single session.
func (h *Handler) addToAccountSet(w http.ResponseWriter, r *http.Request, user *User, rs *RefreshSession) {
	if rs == nil {
		return
	}
	newRaw, err := newAccountsHandle()
	if err != nil {
		log.Printf("[auth] operation=account_set_add user_id=%s result=error step=handle error=%q", user.ID, err)
		return
	}
	var oldSession string
	if set := h.requestAccountSet(r); set != nil {
		if m, merr := h.store.GetAccountMember(set.ID, user.ID); merr == nil {
			oldSession = m.RefreshSessionID
		}
	}
	var oldTokenHash string
	if oldSession != "" {
		if old, oerr := h.store.GetRefreshSessionByID(oldSession); oerr == nil {
			oldTokenHash = old.TokenHash
		}
	}
	_, created, err := h.store.AddAccountToSet(requestAccountHandleHash(r), accountsHandleHash(newRaw), user.ID, rs.ID)
	if err != nil {
		log.Printf("[auth] operation=account_set_add user_id=%s result=error step=store error=%q", user.ID, err)
		return
	}
	if oldTokenHash != "" {
		h.revokeRotatedGraceChain(oldTokenHash)
	}
	_ = created
	h.setAccountsCookie(w, newRaw)
}

// revokeRotatedGraceChain deletes the grace entries that lead to the session
// with the given token hash: the entry whose new refresh token is that
// session, then the entry that led to that one, and so on.
func (h *Handler) revokeRotatedGraceChain(tokenHash string) {
	h.rotationsMu.Lock()
	defer h.rotationsMu.Unlock()
	target := tokenHash
	for i := 0; i < 64; i++ {
		found := ""
		for k, v := range h.recentRotations {
			if sha256SumHex(v.newRefresh) == target {
				found = k
				break
			}
		}
		if found == "" {
			return
		}
		delete(h.recentRotations, found)
		target = found
	}
}

// graceLeadingTo returns the grace entry whose new refresh token is the
// session with the given token hash.
func (h *Handler) graceLeadingTo(tokenHash string) (rotatedGraceEntry, bool) {
	h.rotationsMu.Lock()
	defer h.rotationsMu.Unlock()
	now := time.Now().UTC()
	for _, v := range h.recentRotations {
		if now.Before(v.expiresAt) && sha256SumHex(v.newRefresh) == tokenHash {
			return v, true
		}
	}
	return rotatedGraceEntry{}, false
}

// accountGateRefuses is the renewal gate. When the request carries a set
// cookie and the presented refresh session (from the store, else from the
// rotation grace table) is a member of that set whose user is not the set's
// active member, it returns true and the active user. It runs before any
// grace lookup, so the grace path cannot reverse a switch.
func (h *Handler) accountGateRefuses(r *http.Request, refreshHash string) (bool, string) {
	set := h.requestAccountSet(r)
	if set == nil || refreshHash == "" {
		return false, ""
	}
	if rs, err := h.store.GetRefreshSessionByTokenHash(refreshHash); err == nil {
		if m, merr := h.store.GetAccountMemberBySession(set.ID, rs.ID); merr == nil && m.UserID != set.ActiveUserID {
			return true, set.ActiveUserID
		}
		return false, ""
	}
	if grace, ok := h.getRotatedGrace(refreshHash); ok && grace.user != nil {
		if nrs, err := h.store.GetRefreshSessionByTokenHash(sha256SumHex(grace.newRefresh)); err == nil {
			if m, merr := h.store.GetAccountMemberBySession(set.ID, nrs.ID); merr == nil && m.UserID != set.ActiveUserID {
				return true, set.ActiveUserID
			}
			return false, ""
		}
		// The grace target was rotated again: judge by the user.
		if m, merr := h.store.GetAccountMember(set.ID, grace.user.ID); merr == nil && m.UserID != set.ActiveUserID {
			return true, set.ActiveUserID
		}
	}
	return false, ""
}

// accountSwitchedError is the rotation refusal when a switch committed after
// the renewal passed its first gate.
type accountSwitchedError struct{ ActiveUserID string }

func (e *accountSwitchedError) Error() string { return "account switched" }

func isAccountSwitched(err error) bool {
	var e *accountSwitchedError
	return errors.As(err, &e)
}

func writeAccountSwitched(w http.ResponseWriter, active string) {
	writeJSON(w, http.StatusOK, sessionResponse{Authenticated: false, Reason: ReasonAccountSwitched, ActiveUserID: active})
}

// --- endpoints ---

type accountEntry struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Active   bool   `json:"active"`
	SignedIn bool   `json:"signed_in"`
}

type accountUserRequest struct {
	UserID string `json:"user_id"`
}

func hasAPIKeyBearer(r *http.Request) bool {
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	return strings.HasPrefix(ah, "Bearer ")
}

// HandleAccountsList handles GET /auth/accounts (cookie-only).
func (h *Handler) HandleAccountsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if hasAPIKeyBearer(r) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api keys cannot list accounts"})
		return
	}
	set := h.requestAccountSet(r)
	if set == nil {
		writeJSON(w, http.StatusOK, []accountEntry{})
		return
	}
	listing, err := h.store.ListAccountSet(set)
	if err != nil {
		log.Printf("[auth] operation=accounts_list result=error error=%q", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	out := make([]accountEntry, 0, len(listing))
	for _, l := range listing {
		out = append(out, accountEntry{UserID: l.UserID, Email: l.Email, Active: l.Active, SignedIn: l.SignedIn})
	}
	writeJSON(w, http.StatusOK, out)
}

// accountMutationPreamble is the shared front of the account POSTs: method,
// no API key, JSON content type, the set from the cookie, the body.
func (h *Handler) accountMutationPreamble(w http.ResponseWriter, r *http.Request, needBody bool) (*AccountSet, *accountUserRequest, bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return nil, nil, false
	}
	if hasAPIKeyBearer(r) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api keys cannot manage accounts"})
		return nil, nil, false
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return nil, nil, false
	}
	var req accountUserRequest
	if needBody {
		if err := readJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return nil, nil, false
		}
		if strings.TrimSpace(req.UserID) == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "user_id is required"})
			return nil, nil, false
		}
	}
	set := h.requestAccountSet(r)
	if set == nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "no account set for this browser"})
		return nil, nil, false
	}
	return set, &req, true
}

// HandleAccountSwitch handles POST /auth/accounts/switch {user_id}.
func (h *Handler) HandleAccountSwitch(w http.ResponseWriter, r *http.Request) {
	set, req, ok := h.accountMutationPreamble(w, r, true)
	if !ok {
		return
	}
	member, err := h.store.GetAccountMember(set.ID, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "account is not signed in on this browser"})
		return
	}
	h.rotateMu.Lock()
	defer h.rotateMu.Unlock()

	// Re-read the set under the lock: a racing switch may have moved it.
	if fresh, ferr := h.store.GetAccountSetByHandleHash(set.HandleHash); ferr == nil {
		set = fresh
	}
	member, err = h.store.GetAccountMember(set.ID, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "account is not signed in on this browser"})
		return
	}
	rs, err := h.store.GetRefreshSessionByID(member.RefreshSessionID)
	if err != nil || time.Now().UTC().After(rs.ExpiresAt) {
		if err == nil {
			_ = h.store.DeleteRefreshSessionByID(rs.ID)
		}
		writeJSON(w, http.StatusConflict, sessionResponse{Authenticated: false, Error: "that account is signed out", Reason: ReasonMemberSignedOut, ActiveUserID: set.ActiveUserID})
		return
	}
	user, err := h.store.GetUserByID(rs.UserID)
	if err != nil {
		writeJSON(w, http.StatusConflict, sessionResponse{Authenticated: false, Error: "that account is signed out", Reason: ReasonMemberSignedOut})
		return
	}
	respond := func() {
		h.setAccountsCookie(w, mustCookieValue(r, AccountsCookieName))
		writeJSON(w, http.StatusOK, sessionResponse{
			Authenticated: true, ActiveUserID: user.ID,
			User: &userInfo{ID: user.ID, Email: user.Email, CreatedAt: user.CreatedAt.Format(time.RFC3339)},
		})
	}
	method := normalizeAuthMethod(rs.AuthMethod)
	// Every switch rotates the member's session and writes the fresh pair (the
	// server holds only token hashes). The one exception is a racing switch
	// that already rotated this member inside the grace window: reuse that
	// pair so a double switch rotates once.
	if set.ActiveUserID == user.ID {
		if grace, gok := h.graceLeadingTo(rs.TokenHash); gok {
			jwtTok, jerr := h.issueAccessJWT(user, method)
			if jerr != nil {
				writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to switch account"})
				return
			}
			exp := rs.ExpiresAt
			h.setAuthCookiesFor(w, jwtTok, grace.newRefresh, h.refreshCookieTTL(method, &exp))
			respond()
			return
		}
	}
	if _, _, rerr := h.rotateLocked(w, r, rs, user, set.ID); rerr != nil {
		if errors.Is(rerr, ErrRotationLost) {
			writeJSON(w, http.StatusConflict, sessionResponse{Authenticated: false, Error: "that account is signed out", Reason: ReasonMemberSignedOut})
			return
		}
		log.Printf("[auth] operation=account_switch user_id=%s result=error step=rotate error=%q", user.ID, rerr)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to switch account"})
		return
	}
	log.Printf("[auth] operation=account_switch user_id=%s result=success", user.ID)
	respond()
}

func mustCookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// signoutMemberSession deletes a member's refresh session and the grace
// entries leading to it.
func (h *Handler) signoutMemberSession(sessionID string) {
	if rs, err := h.store.GetRefreshSessionByID(sessionID); err == nil {
		h.revokeRotatedGraceChain(rs.TokenHash)
		h.deleteRotatedGraceByTokenHash(rs.TokenHash)
	}
	_ = h.store.DeleteRefreshSessionByID(sessionID)
}

type accountSignoutResponse struct {
	Authenticated bool   `json:"authenticated"`
	NextUserID    string `json:"next_user_id,omitempty"`
	SetEmpty      bool   `json:"set_empty,omitempty"`
}

// HandleAccountSignout handles POST /auth/accounts/signout {user_id}.
func (h *Handler) HandleAccountSignout(w http.ResponseWriter, r *http.Request) {
	set, req, ok := h.accountMutationPreamble(w, r, true)
	if !ok {
		return
	}
	member, err := h.store.GetAccountMember(set.ID, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "account is not signed in on this browser"})
		return
	}
	wasActive := set.ActiveUserID == member.UserID
	h.signoutMemberSession(member.RefreshSessionID)
	next, emptied, err := h.store.RemoveAccountMember(set.ID, member.UserID)
	if err != nil {
		log.Printf("[auth] operation=account_signout user_id=%s result=error error=%q", member.UserID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	resp := accountSignoutResponse{}
	switch {
	case emptied:
		h.clearAuthCookies(w)
		h.clearAccountsCookie(w)
		resp.SetEmpty = true
	case wasActive:
		if next != "" {
			resp.NextUserID = next
			resp.Authenticated = true // the browser stays signed in; the follow-up switch sets the pair
			// The signed-out account's pair must not linger in the jar.
			h.clearAuthCookies(w)
		} else {
			h.clearAuthCookies(w)
		}
	default:
		resp.Authenticated = true
	}
	log.Printf("[auth] operation=account_signout user_id=%s result=success active=%t set_empty=%t", member.UserID, wasActive, emptied)
	writeJSON(w, http.StatusOK, resp)
}

// HandleAccountsLogoutAll handles POST /auth/accounts/logout-all: this
// browser's accounts only. Other devices of those users stay signed in.
func (h *Handler) HandleAccountsLogoutAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if hasAPIKeyBearer(r) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "api keys cannot manage accounts"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return
	}
	if set := h.requestAccountSet(r); set != nil {
		members, err := h.store.ListAccountMembers(set.ID)
		if err != nil {
			log.Printf("[auth] operation=accounts_logout_all result=error step=list error=%q", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
			return
		}
		for _, m := range members {
			h.revokeRotatedGraceForUser(m.UserID)
			h.signoutMemberSession(m.RefreshSessionID)
		}
		if err := h.store.DeleteAccountSet(set.ID); err != nil {
			log.Printf("[auth] operation=accounts_logout_all result=error step=delete_set error=%q", err)
		}
	}
	// A pair that is not a member (no set, or a session outside it) is this
	// browser's too: sign it out the way logout does.
	if c, err := r.Cookie(RefreshTokenCookieName); err == nil && c.Value != "" {
		hash := sha256SumHex(c.Value)
		h.deleteRotatedGraceByTokenHash(hash)
		if rs, rerr := h.store.GetRefreshSessionByTokenHash(hash); rerr == nil {
			_ = h.store.DeleteRefreshSessionByID(rs.ID)
		}
	}
	h.clearAuthCookies(w)
	h.clearAccountsCookie(w)
	writeJSON(w, http.StatusOK, sessionResponse{Authenticated: false})
}

// afterLogoutSession removes the signed-out session's member row from the
// browser's set and names the next signed-in member. It writes no auth
// cookie (HandleLogout clears them); it clears the set cookie when the last
// member goes.
func (h *Handler) afterLogoutSession(w http.ResponseWriter, r *http.Request, sessionID string) string {
	set := h.requestAccountSet(r)
	if set == nil || sessionID == "" {
		return ""
	}
	m, err := h.store.GetAccountMemberBySession(set.ID, sessionID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[auth] operation=logout step=member_lookup error=%q", err)
		}
		return ""
	}
	next, emptied, err := h.store.RemoveAccountMember(set.ID, m.UserID)
	if err != nil {
		log.Printf("[auth] operation=logout step=remove_member error=%q", err)
		return ""
	}
	if emptied {
		h.clearAccountsCookie(w)
		return ""
	}
	return next
}

// StartAccountSweeper runs SweepAccountSets on an interval.
func (h *Handler) sweepAccountSets() {
	m, s, err := h.store.SweepAccountSets(time.Now().UTC())
	if err != nil {
		log.Printf("[auth] operation=account_sweep result=error error=%q", err)
		return
	}
	if m > 0 || s > 0 {
		log.Printf("[auth] operation=account_sweep members=%d sets=%d", m, s)
	}
}
