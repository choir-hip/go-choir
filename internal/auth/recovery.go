package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// --- Account Recovery (M7) ---
//
// Email magic link recovery provides a fallback path for users who have lost
// their WebAuthn device. Recovery does NOT bypass WebAuthn — it creates a new
// WebAuthn registration challenge so the user can register a new passkey.
// The user must complete the WebAuthn ceremony to regain access.
//
// Magic link tokens are opaque (not user ID or email), stored as SHA-256
// hashes (same pattern as API keys), single-use, and time-limited (15 min).

// recoveryRequest is the JSON body for POST /auth/recovery/request.
type recoveryRequest struct {
	Email string `json:"email"`
}

// recoveryResponse is the JSON response for POST /auth/recovery/request.
// In production, the token is sent via email and the Token field is omitted.
// For development/testing, the token is returned so tests can verify the flow.
type recoveryResponse struct {
	OK    bool   `json:"ok"`
	Token string `json:"token,omitempty"` // dev/test only; production sends via email
}

// recoveryVerifyRequest is the JSON body for POST /auth/recovery/verify.
type recoveryVerifyRequest struct {
	Token string `json:"token"`
}

// hashIP returns a short, non-reversible hash of the client IP for logging
// and rate-limiting storage. This avoids storing raw IP addresses in the
// recovery_tokens table. The hash is the full SHA-256 hex digest.
func hashIP(ip string) string {
	h := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("%x", h)
}

// sha256SumHex returns the SHA-256 hex digest of the given string.
func sha256SumHex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// clientIP extracts the client IP address from the request. It checks
// X-Forwarded-For first (for requests behind a reverse proxy), then falls
// back to RemoteAddr. Only the first IP in X-Forwarded-For is used.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first (leftmost) IP in the list.
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// RemoteAddr is "host:port" — strip the port.
	addr := r.RemoteAddr
	if idx := strings.LastIndexByte(addr, ':'); idx >= 0 {
		addr = addr[:idx]
	}
	return strings.TrimSpace(addr)
}

// HandleRecoveryRequest handles POST /auth/recovery/request.
// It generates a magic link recovery token for the given email. The token
// is stored as a SHA-256 hash (never in plaintext), is single-use, and
// expires after 15 minutes. Rate limiting is enforced per-email (3/hour) and
// per-IP (5/hour) before any DB write.
//
// To prevent email enumeration, the endpoint returns the same success
// response regardless of whether the email corresponds to a real user.
// For non-existent users, a dummy token record is created for rate-limiting
// purposes but the token cannot be used for recovery.
func (h *Handler) HandleRecoveryRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req recoveryRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "email is required"})
		return
	}

	if !isValidEmail(req.Email) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "please enter a valid email address"})
		return
	}

	emailHash := hashEmail(req.Email)
	ip := clientIP(r)
	ipHash := hashIP(ip)

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// --- Rate limiting (enforced BEFORE any DB write) ---
	oneHourAgo := time.Now().UTC().Add(-1 * time.Hour)

	emailCount, err := h.store.CountRecoveryTokensByEmailSince(ctx, emailHash, oneHourAgo)
	if err != nil {
		log.Printf("[auth] operation=recovery_request email_hash=%s result=error step=count_email error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if emailCount >= RecoveryMaxPerEmail {
		log.Printf("[auth] operation=recovery_request email_hash=%s result=rate_limited reason=email_limit count=%d", emailHash, emailCount)
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many recovery requests for this email"})
		return
	}

	ipCount, err := h.store.CountRecoveryTokensByIPSince(ctx, ipHash, oneHourAgo)
	if err != nil {
		log.Printf("[auth] operation=recovery_request email_hash=%s result=error step=count_ip error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if ipCount >= RecoveryMaxPerIP {
		log.Printf("[auth] operation=recovery_request email_hash=%s result=rate_limited reason=ip_limit count=%d", emailHash, ipCount)
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many recovery requests from this address"})
		return
	}

	// --- Look up user (anti-enumeration: same response either way) ---
	user, err := h.store.GetUserByEmail(req.Email)
	var userID string
	if err == nil {
		userID = user.ID
		log.Printf("[auth] operation=recovery_request email_hash=%s user_id=%s step=user_found", emailHash, user.ID)
	} else {
		// User not found — create a dummy record for rate limiting.
		// The token will not be usable for recovery (ConsumeRecoveryToken
		// rejects tokens with no user_id).
		log.Printf("[auth] operation=recovery_request email_hash=%s step=user_not_found", emailHash)
	}

	// --- Create recovery token (hash stored, raw token returned once) ---
	token, err := h.store.CreateRecoveryToken(ctx, userID, req.Email, emailHash, ipHash)
	if err != nil {
		log.Printf("[auth] operation=recovery_request email_hash=%s result=error step=create_token error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create recovery token"})
		return
	}

	log.Printf("[auth] operation=recovery_request email_hash=%s result=success step=token_created", emailHash)

	// In production, the token would be sent via email and not returned here.
	// For development/testing, we return it so the flow can be verified.
	writeJSON(w, http.StatusOK, recoveryResponse{OK: true, Token: token})
}

// HandleRecoveryVerify handles POST /auth/recovery/verify.
// It validates the magic link token (hash, expiry, single-use), and on
// success creates a new WebAuthn registration challenge for the user.
// This does NOT auto-login — the user must complete WebAuthn registration
// via /auth/register/finish to regain access.
func (h *Handler) HandleRecoveryVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	var req recoveryVerifyRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if req.Token == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "token is required"})
		return
	}

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Consume the token (validates hash, expiry, single-use).
	rt, err := h.store.ConsumeRecoveryToken(ctx, req.Token)
	if err != nil {
		log.Printf("[auth] operation=recovery_verify result=error step=consume_token error=%q", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid or expired recovery token"})
		return
	}

	emailHash := hashEmail(rt.Email)
	log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s step=token_valid", emailHash, rt.UserID)

	// Look up the user.
	user, err := h.store.GetUserByID(rt.UserID)
	if err != nil {
		log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=error step=user_lookup error=%q", emailHash, rt.UserID, err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "user not found"})
		return
	}

	// Get existing credentials (to exclude duplicates in registration).
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil {
		log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=error step=get_credentials error=%q", emailHash, user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	// Build a WebAuthn user adapter with existing credentials.
	waUser, err := newWebAuthnUser(user, creds)
	if err != nil {
		log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=error step=webauthn_adapter error=%q", emailHash, user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	// Begin WebAuthn registration (creates a new challenge for adding a device).
	creation, session, err := h.webauthn.BeginRegistration(waUser,
		webauthn.WithConveyancePreference("none"),
		webauthn.WithAuthenticatorSelection(
			webauthn.SelectAuthenticator("platform", nil, "required"),
		),
	)
	if err != nil {
		log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=error step=begin_registration error=%q", emailHash, user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to begin registration"})
		return
	}

	// Serialize the WebAuthn session data for the finish handler.
	sessionDataJSON, err := json.Marshal(session)
	if err != nil {
		log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=error step=marshal_session error=%q", emailHash, user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	// Persist the challenge state (type "registration" so /auth/register/finish works).
	challengeState := &ChallengeState{
		ID:                  session.Challenge,
		UserID:              user.ID,
		Challenge:           session.Challenge,
		Type:                "registration",
		WebAuthnSessionData: string(sessionDataJSON),
		CreatedAt:           time.Now().UTC(),
		ExpiresAt:           time.Now().UTC().Add(SessionChallengeTTL),
	}
	if err := h.store.SaveChallengeState(challengeState); err != nil {
		log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=error step=save_challenge error=%q", emailHash, user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save challenge"})
		return
	}

	log.Printf("[auth] operation=recovery_verify email_hash=%s user_id=%s result=success step=registration_challenge_created", emailHash, user.ID)
	writeJSON(w, http.StatusOK, creation)
}

// --- Multi-Device Passkey Management (M7) ---

// credentialInfo is the JSON representation of a credential for the listing
// endpoint. It deliberately excludes public keys, transports, and other
// sensitive authenticator data.
type credentialInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

// listCredentialsResponse is the JSON response for GET /auth/credentials.
type listCredentialsResponse struct {
	Credentials []credentialInfo `json:"credentials"`
}

// renameCredentialRequest is the JSON body for POST /auth/credentials/rename.
type renameCredentialRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// credentialToInfo converts a Credential to its safe JSON form (no secrets).
func credentialToInfo(c *Credential) credentialInfo {
	var lastUsedAt *string
	if c.LastUsedAt != nil {
		s := c.LastUsedAt.UTC().Format(time.RFC3339)
		lastUsedAt = &s
	}
	return credentialInfo{
		ID:         c.ID,
		Name:       c.Name,
		CreatedAt:  c.CreatedAt.UTC().Format(time.RFC3339),
		LastUsedAt: lastUsedAt,
	}
}

// HandleListCredentials handles GET /auth/credentials.
// It requires a valid choir_access cookie and returns the user's WebAuthn
// credentials (ID, name, created_at, last_used_at). It does NOT return
// public keys, transports, or other sensitive authenticator data.
func (h *Handler) HandleListCredentials(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	user := h.requireAuthUser(w, r)
	if user == nil {
		return
	}

	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil {
		log.Printf("[auth] operation=list_credentials user_id=%s result=error error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list credentials"})
		return
	}

	resp := listCredentialsResponse{Credentials: make([]credentialInfo, 0, len(creds))}
	for i := range creds {
		resp.Credentials = append(resp.Credentials, credentialToInfo(&creds[i]))
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleDeleteCredential handles DELETE /auth/credentials/{id}.
// It requires a valid choir_access cookie and removes the specified credential.
// The user cannot delete their last remaining credential (must have at least
// 2, or use recovery to add a new device first). Only the credential owner
// can delete their own credentials.
func (h *Handler) HandleDeleteCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	user := h.requireAuthUser(w, r)
	if user == nil {
		return
	}

	// Extract the credential ID from the path: /auth/credentials/{id}.
	credID := strings.TrimPrefix(r.URL.Path, "/auth/credentials/")
	credID = strings.Trim(credID, "/")
	if credID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "credential id is required"})
		return
	}

	// Check credential count and ownership — cannot delete last credential.
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil {
		log.Printf("[auth] operation=delete_credential user_id=%s cred_id=%s result=error step=count error=%q", user.ID, credID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to check credentials"})
		return
	}

	// Check if the credential belongs to this user (ownership check).
	var found bool
	for _, c := range creds {
		if c.ID == credID {
			found = true
			break
		}
	}
	if !found {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "credential not found"})
		return
	}

	// Last-credential guard: cannot delete the only remaining credential.
	if len(creds) <= 1 {
		log.Printf("[auth] operation=delete_credential user_id=%s cred_id=%s result=rejected reason=last_credential count=%d", user.ID, credID, len(creds))
		writeJSON(w, http.StatusConflict, errorResponse{Error: "cannot delete your last credential — use recovery to add a new device first"})
		return
	}

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if err := h.store.DeleteCredential(ctx, user.ID, credID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "credential not found"})
			return
		}
		log.Printf("[auth] operation=delete_credential user_id=%s cred_id=%s result=error error=%q", user.ID, credID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to delete credential"})
		return
	}

	log.Printf("[auth] operation=delete_credential user_id=%s cred_id=%s result=success remaining=%d", user.ID, credID, len(creds)-1)
	w.WriteHeader(http.StatusNoContent)
}

// HandleRenameCredential handles POST /auth/credentials/rename.
// It requires a valid choir_access cookie and updates the user-facing name
// of a credential. Only the credential owner can rename their own credentials.
func (h *Handler) HandleRenameCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	user := h.requireAuthUser(w, r)
	if user == nil {
		return
	}

	var req renameCredentialRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "credential id is required"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "name is required"})
		return
	}

	// Limit name length to prevent abuse.
	if len(req.Name) > 100 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "name must be 100 characters or fewer"})
		return
	}

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if err := h.store.RenameCredential(ctx, user.ID, req.ID, req.Name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "credential not found"})
			return
		}
		log.Printf("[auth] operation=rename_credential user_id=%s cred_id=%s result=error error=%q", user.ID, req.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to rename credential"})
		return
	}

	log.Printf("[auth] operation=rename_credential user_id=%s cred_id=%s result=success", user.ID, req.ID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- Session Management (M7) ---

// sessionInfo is the JSON representation of a session for the listing
// endpoint. It deliberately excludes the token hash.
type sessionInfo struct {
	ID         string  `json:"id"`
	DeviceInfo string  `json:"device_info"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

// listSessionsResponse is the JSON response for GET /auth/sessions.
type listSessionsResponse struct {
	Sessions []sessionInfo `json:"sessions"`
}

// sessionToInfo converts a RefreshSession to its safe JSON form (no token hash).
func sessionToInfo(rs *RefreshSession) sessionInfo {
	var lastUsedAt *string
	if rs.LastUsedAt != nil {
		s := rs.LastUsedAt.UTC().Format(time.RFC3339)
		lastUsedAt = &s
	}
	return sessionInfo{
		ID:         rs.ID,
		DeviceInfo: rs.DeviceInfo,
		CreatedAt:  rs.CreatedAt.UTC().Format(time.RFC3339),
		LastUsedAt: lastUsedAt,
	}
}

// HandleListSessions handles GET /auth/sessions.
// It requires a valid choir_access cookie and returns the user's active
// refresh sessions (ID, device info, created_at, last_used_at). Token hashes
// are never exposed.
func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	user := h.requireAuthUser(w, r)
	if user == nil {
		return
	}

	sessions, err := h.store.ListRefreshSessionsByUserID(user.ID)
	if err != nil {
		log.Printf("[auth] operation=list_sessions user_id=%s result=error error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list sessions"})
		return
	}

	resp := listSessionsResponse{Sessions: make([]sessionInfo, 0, len(sessions))}
	for i := range sessions {
		resp.Sessions = append(resp.Sessions, sessionToInfo(&sessions[i]))
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleRevokeSession handles DELETE /auth/sessions/{id}.
// It requires a valid choir_access cookie and revokes (deletes) the specified
// refresh session. The current session (identified by the refresh cookie)
// cannot be revoked via this endpoint — use /auth/logout instead.
// Only the session owner can revoke their own sessions.
func (h *Handler) HandleRevokeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	user := h.requireAuthUser(w, r)
	if user == nil {
		return
	}

	// Extract the session ID from the path: /auth/sessions/{id}.
	sessionID := strings.TrimPrefix(r.URL.Path, "/auth/sessions/")
	sessionID = strings.Trim(sessionID, "/")
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "session id is required"})
		return
	}

	// Check if this is the current session (by comparing the refresh cookie
	// hash to the session's token hash). The current session must be revoked
	// via /auth/logout, not this endpoint.
	if cookie, err := r.Cookie(RefreshTokenCookieName); err == nil && cookie.Value != "" {
		cookieHash := sha256SumHex(cookie.Value)
		rs, err := h.store.GetRefreshSessionByID(sessionID)
		if err == nil && rs.TokenHash == cookieHash {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "cannot revoke current session — use /auth/logout instead"})
			return
		}
	}

	// Look up the session to verify ownership before deleting.
	rs, err := h.store.GetRefreshSessionByID(sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "session not found"})
			return
		}
		log.Printf("[auth] operation=revoke_session user_id=%s session_id=%s result=error step=lookup error=%q", user.ID, sessionID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to revoke session"})
		return
	}

	// Ownership check.
	if rs.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "session not found"})
		return
	}

	if err := h.store.DeleteRefreshSessionByID(sessionID); err != nil {
		log.Printf("[auth] operation=revoke_session user_id=%s session_id=%s result=error step=delete error=%q", user.ID, sessionID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to revoke session"})
		return
	}

	log.Printf("[auth] operation=revoke_session user_id=%s session_id=%s result=success", user.ID, sessionID)
	w.WriteHeader(http.StatusNoContent)
}
