package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Email sign-in (docs/design/email-sign-in-2026-10-31.md, red: auth and
// session). An 8-digit code bound to the attempt that asked for it, or a
// single-use link, signs a user in by email. The session it makes carries its
// method (email_code or email_link) through every refresh and the desktop
// handoff, and a passkey-only gate (requirePasskeySession) keeps what outlives
// a session -- credentials, keys, other sessions, the data ROOT -- out of
// reach of an inbox-strength session.

// Authentication methods a session can carry.
const (
	AuthMethodPasskey   = "passkey"
	AuthMethodEmailCode = "email_code"
	AuthMethodEmailLink = "email_link"
)

const passkeyRequiredMessage = "Sign in with a passkey to do this."

// EmailAttemptCookieName is the cookie that binds a code to the client that
// asked for it (login-CSRF defense: an attacker cannot make a victim's browser
// verify the attacker's attempt).
const EmailAttemptCookieName = "choir_signin_attempt"

// EmailAttemptCookiePath scopes the attempt cookie to the email routes.
const EmailAttemptCookiePath = "/auth/email"

// EmailLinkTokenPrefix starts every sign-in link token.
const EmailLinkTokenPrefix = "choir_st_"

// EmailSignInLinkPath is the frontend page a sign-in link opens. The token
// travels in the fragment so it stays out of logs, Referer and history.
const EmailSignInLinkPath = "/signin/email"

const genericEmailVerifyError = "invalid or expired code"

// normalizeAuthMethod reads an empty method (a row or a token from before
// email sign-in) as passkey.
func normalizeAuthMethod(m string) string {
	if m == "" {
		return AuthMethodPasskey
	}
	return m
}

// isPasskeyMethod is true only for passkey sessions. Any other value, known
// or not, is treated as an email-strength session and gated.
func isPasskeyMethod(m string) bool { return normalizeAuthMethod(m) == AuthMethodPasskey }

func isEmailMethod(m string) bool { return !isPasskeyMethod(m) }

// emailSignInState is the in-memory half of the feature: the switch and the
// verify limiter.
type emailSignInState struct {
	enabled atomic.Bool

	verifyMu   sync.Mutex
	verifyHits map[string][]time.Time
}

func (h *Handler) initEmailSignIn() {
	if h.config != nil && h.config.EmailSignIn && len(h.config.EmailCodeKey) >= minEmailCodeKeyLen {
		h.emailSignIn.enabled.Store(true)
	}
	h.emailSignIn.verifyHits = make(map[string][]time.Time)
}

// EmailSignInEnabled reports the auth.email_sign_in switch.
func (h *Handler) EmailSignInEnabled() bool { return h.emailSignIn.enabled.Load() }

// SetEmailSignIn flips the switch. Turning it off deletes every unconsumed
// attempt and every email-method refresh session, so no outstanding code or
// link can still mint a session and no session already minted survives. Turning it
// on needs the HMAC key (fail closed).
func (h *Handler) SetEmailSignIn(on bool) error {
	if on {
		if h.config == nil || len(h.config.EmailCodeKey) < minEmailCodeKeyLen {
			return errors.New("email sign-in needs AUTH_EMAIL_CODE_KEY")
		}
		h.emailSignIn.enabled.Store(true)
		return nil
	}
	h.emailSignIn.enabled.Store(false)
	n, err := h.store.DeleteUnconsumedEmailAttempts(context.Background())
	if err != nil {
		return err
	}
	m, err := h.store.DeleteEmailRefreshSessions(context.Background())
	if err != nil {
		return err
	}
	h.revokeAllRotatedGrace()
	log.Printf("[auth] operation=email_signin_switch state=off deleted_attempts=%d deleted_email_sessions=%d", n, m)
	return nil
}

// ApplyEmailSignInSwitch runs once at startup: with the switch off it deletes
// the attempts a previous run left outstanding.
func (h *Handler) ApplyEmailSignInSwitch() error {
	if h.EmailSignInEnabled() {
		log.Printf("[auth] operation=email_signin_switch state=on")
		return nil
	}
	return h.SetEmailSignIn(false)
}

func (h *Handler) emailSessionTTL() time.Duration {
	if h.config != nil && h.config.EmailSessionTTL > 0 {
		return h.config.EmailSessionTTL
	}
	return DefaultEmailSessionTTL
}

// refreshCookieTTL is the refresh cookie lifetime for a session: the
// configured TTL, or for an email session the time left to its fixed expiry.
func (h *Handler) refreshCookieTTL(method string, expiresAt *time.Time) time.Duration {
	ttl := h.config.RefreshTokenTTL
	if !isEmailMethod(method) {
		return ttl
	}
	if limit := h.emailSessionTTL(); limit < ttl {
		ttl = limit
	}
	if expiresAt != nil {
		if left := time.Until(*expiresAt); left < ttl {
			ttl = left
		}
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	return ttl
}

// --- keyed hashes ---

func (h *Handler) emailKeyed(domain, msg string) string {
	mac := hmac.New(sha256.New, h.config.EmailCodeKey)
	mac.Write([]byte(domain))
	mac.Write([]byte{0})
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// emailAddressHash is the keyed hash of a normalized address, so a database
// read does not recover addresses by dictionary.
func (h *Handler) emailAddressHash(normalized string) string {
	return h.emailKeyed("email", normalized)
}

func (h *Handler) emailCodeHash(attemptID, purpose, code string) string {
	return h.emailKeyed("code", attemptID+"\x00"+purpose+"\x00"+code)
}

func normalizeEmailAddress(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func newEmailCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

func isEightDigits(s string) bool {
	if len(s) != 8 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// shortHash names a secret-ish handle in logs without revealing it.
func shortHash(s string) string { return sha256SumHex(s)[:8] }

// --- request shape ---

func requireJSONContentType(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/json"
}

type emailBeginRequest struct {
	Email string `json:"email"`
}

type emailBeginResponse struct {
	AttemptID string `json:"attempt_id"`
}

type emailVerifyRequest struct {
	AttemptID string `json:"attempt_id"`
	Code      string `json:"code"`
	Token     string `json:"token"`
}

func (h *Handler) setAttemptCookie(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     EmailAttemptCookieName,
		Value:    value,
		Path:     EmailAttemptCookiePath,
		MaxAge:   int(EmailAttemptTTL.Seconds()),
		Secure:   h.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) clearAttemptCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     EmailAttemptCookieName,
		Value:    "",
		Path:     EmailAttemptCookiePath,
		MaxAge:   -1,
		Secure:   h.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// --- begin ---

// HandleEmailBegin handles POST /auth/email/begin {email}. It answers the same
// way, with the same shape and status, whether or not the address has an
// eligible account (T2). Only an address that belongs to an account with at
// least one passkey and a verified address is mailed (no sign-up by email, D1;
// T10); anything else gets a row that matches nothing and no mail.
func (h *Handler) HandleEmailBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return
	}
	var req emailBeginRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "email is required"})
		return
	}
	if !isValidEmail(strings.TrimSpace(req.Email)) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "please enter a valid email address"})
		return
	}

	attemptID, cookieValue, err := newAttemptHandles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	// Switch off: answer as if every address were unknown, write nothing.
	if !h.EmailSignInEnabled() {
		h.setAttemptCookie(w, cookieValue)
		writeJSON(w, http.StatusOK, emailBeginResponse{AttemptID: attemptID})
		return
	}

	norm := normalizeEmailAddress(req.Email)
	user := h.lookupEligibleSignInUser(strings.TrimSpace(req.Email))

	a, code, linkToken, err := h.newEmailAttempt(r, attemptID, cookieValue, EmailAttemptPurposeSignIn, norm, user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if err := h.store.CreateEmailAttempt(r.Context(), a); err != nil {
		if errors.Is(err, ErrEmailRateLimited) {
			log.Printf("[auth] operation=email_begin email_hash=%s result=rate_limited", shortHash(a.EmailHash))
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many sign-in requests; try again later"})
			return
		}
		log.Printf("[auth] operation=email_begin email_hash=%s result=error step=create_attempt error=%q", shortHash(a.EmailHash), err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}

	h.setAttemptCookie(w, cookieValue)
	if user != nil {
		log.Printf("[auth] operation=email_begin attempt=%s email_hash=%s user_id=%s result=created", shortHash(attemptID), shortHash(a.EmailHash), user.ID)
		h.sendSignInMail(r, a, user.Email, code, h.signInLink(linkToken))
	} else {
		log.Printf("[auth] operation=email_begin attempt=%s email_hash=%s result=created note=no_eligible_account", shortHash(attemptID), shortHash(a.EmailHash))
	}
	writeJSON(w, http.StatusOK, emailBeginResponse{AttemptID: attemptID})
}

func newAttemptHandles() (attemptID, cookieValue string, err error) {
	raw := make([]byte, 16)
	if _, err = rand.Read(raw); err != nil {
		return "", "", err
	}
	attemptID = hex.EncodeToString(raw)
	cookieValue, err = randomToken(32)
	return attemptID, cookieValue, err
}

// lookupEligibleSignInUser returns the account that email sign-in may mail:
// it has at least one passkey credential and a verified address. Anything
// else (unknown, no passkey, unverified) is nil. The address is looked up
// exactly as stored, as recovery does; only email_hash is normalized.
func (h *Handler) lookupEligibleSignInUser(trimmed string) *User {
	user, err := h.store.GetUserByEmail(trimmed)
	if err != nil || user == nil || user.EmailVerifiedAt == nil {
		return nil
	}
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil || len(creds) == 0 {
		return nil
	}
	return user
}

// newEmailAttempt builds the row. For a nil user (no eligible account) the
// code and link hashes are random values that match nothing, so verify on it
// fails exactly like a wrong code.
func (h *Handler) newEmailAttempt(r *http.Request, attemptID, cookieValue, purpose, normEmail string, user *User) (a *EmailAttempt, code, linkToken string, err error) {
	now := time.Now().UTC()
	a = &EmailAttempt{
		ID:               attemptID,
		Purpose:          purpose,
		EmailHash:        h.emailAddressHash(normEmail),
		CookieHash:       sha256SumHex(cookieValue),
		IPHash:           hashIP(clientIP(r)),
		UserAgentSummary: summarizeUserAgent(r.UserAgent()),
		CreatedAt:        now,
		ExpiresAt:        now.Add(EmailAttemptTTL),
	}
	if user == nil {
		junk, err := randomToken(32)
		if err != nil {
			return nil, "", "", err
		}
		a.CodeHash = h.emailKeyed("nomatch", junk)
		a.LinkTokenHash = sha256SumHex("nomatch" + junk)
		return a, "", "", nil
	}
	a.UserID = user.ID
	code, err = newEmailCode()
	if err != nil {
		return nil, "", "", err
	}
	a.CodeHash = h.emailCodeHash(attemptID, purpose, code)
	if purpose == EmailAttemptPurposeSignIn {
		tok, err := randomToken(32)
		if err != nil {
			return nil, "", "", err
		}
		linkToken = EmailLinkTokenPrefix + tok
		a.LinkTokenHash = sha256SumHex(linkToken)
	} else {
		junk, err := randomToken(32)
		if err != nil {
			return nil, "", "", err
		}
		a.LinkTokenHash = sha256SumHex("nolink" + junk)
	}
	return a, code, linkToken, nil
}

// signInLink builds the link from configuration, never from the request.
func (h *Handler) signInLink(token string) string {
	if token == "" {
		return ""
	}
	origin := ""
	if h.config != nil && len(h.config.RPOrigins) > 0 {
		origin = strings.TrimRight(strings.TrimSpace(h.config.RPOrigins[0]), "/")
	}
	return origin + EmailSignInLinkPath + "#token=" + token
}

// --- verify ---

// HandleEmailVerify handles POST /auth/email/verify, either
// {attempt_id, code} with the attempt cookie, or {token} from the link.
// Content-Type must be application/json (a cross-site text/plain form cannot
// send it). Every failure answers the same.
func (h *Handler) HandleEmailVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !h.EmailSignInEnabled() {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "email sign-in is not available"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return
	}
	if !h.allowVerify(hashIP(clientIP(r))) {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many attempts; try again later"})
		return
	}
	var req emailVerifyRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	var (
		userID string
		method string
		err    error
	)
	switch {
	case req.Token != "":
		method = AuthMethodEmailLink
		if !strings.HasPrefix(req.Token, EmailLinkTokenPrefix) || len(req.Token) > 128 {
			err = ErrEmailAttemptInvalid
		} else {
			userID, err = h.store.ConsumeEmailLink(r.Context(), sha256SumHex(req.Token))
		}
	case req.AttemptID != "" && req.Code != "":
		method = AuthMethodEmailCode
		userID, err = h.consumeTypedCode(r, EmailAttemptPurposeSignIn, req.AttemptID, req.Code, "")
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "attempt_id and code, or token, is required"})
		return
	}
	if err != nil {
		h.failEmailVerify(w, "email_verify", method, err)
		return
	}

	// The account must still be eligible at the moment of sign-in.
	user, uerr := h.store.GetUserByID(userID)
	if uerr != nil || user.EmailVerifiedAt == nil {
		h.failEmailVerify(w, "email_verify", method, ErrEmailAttemptInvalid)
		return
	}
	if creds, cerr := h.store.GetCredentialsByUserID(user.ID); cerr != nil || len(creds) == 0 {
		h.failEmailVerify(w, "email_verify", method, ErrEmailAttemptInvalid)
		return
	}

	info, ierr := h.signInSession(w, r, user, method)
	if ierr != nil {
		log.Printf("[auth] operation=email_verify user_id=%s method=%s result=error step=issue_session error=%q", user.ID, method, ierr)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create session"})
		return
	}
	h.clearAttemptCookie(w)
	log.Printf("[auth] operation=email_verify user_id=%s method=%s result=success", user.ID, method)
	writeJSON(w, http.StatusOK, finishResponse{OK: true, User: info})
}

func (h *Handler) failEmailVerify(w http.ResponseWriter, op, method string, err error) {
	if errors.Is(err, ErrEmailAttemptInvalid) {
		log.Printf("[auth] operation=%s method=%s result=rejected", op, method)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: genericEmailVerifyError})
		return
	}
	log.Printf("[auth] operation=%s method=%s result=error error=%q", op, method, err)
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
}

// consumeTypedCode runs the code path shared by sign-in and verify-address:
// the attempt cookie must match, the code is compared under the HMAC key.
func (h *Handler) consumeTypedCode(r *http.Request, purpose, attemptID, code, wantUserID string) (string, error) {
	cookie, cerr := r.Cookie(EmailAttemptCookieName)
	if cerr != nil || cookie.Value == "" {
		return "", ErrEmailAttemptInvalid
	}
	if len(attemptID) != 32 || !isEightDigits(code) {
		return "", ErrEmailAttemptInvalid
	}
	want := h.emailCodeHash(attemptID, purpose, code)
	return h.store.ConsumeEmailCode(r.Context(), attemptID, sha256SumHex(cookie.Value), purpose, wantUserID,
		func(stored string) bool {
			return hmac.Equal([]byte(stored), []byte(want))
		})
}

// allowVerify is the per-IP hourly verify limit.
func (h *Handler) allowVerify(ipHash string) bool {
	now := time.Now()
	cutoff := now.Add(-time.Hour)
	st := &h.emailSignIn
	st.verifyMu.Lock()
	defer st.verifyMu.Unlock()
	hits := st.verifyHits[ipHash][:0:0]
	for _, t := range st.verifyHits[ipHash] {
		if t.After(cutoff) {
			hits = append(hits, t)
		}
	}
	if len(hits) >= EmailMaxVerifyPerIP {
		st.verifyHits[ipHash] = hits
		return false
	}
	st.verifyHits[ipHash] = append(hits, now)
	if len(st.verifyHits) > 4096 {
		for k, v := range st.verifyHits {
			if len(v) == 0 || !v[len(v)-1].After(cutoff) {
				delete(st.verifyHits, k)
			}
		}
	}
	return true
}

// --- verify address (passkey session) ---

// HandleVerifyAddressBegin handles POST /auth/email/verify-address/begin: a
// signed-in passkey user asks for a code at their stored address, through the
// same attempt machinery (purpose verify_address). It sets the attempt cookie.
func (h *Handler) HandleVerifyAddressBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	user := h.requirePasskeySession(w, r)
	if user == nil {
		return
	}
	if !h.EmailSignInEnabled() {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "email sign-in is not available"})
		return
	}
	if user.EmailVerifiedAt != nil {
		writeJSON(w, http.StatusOK, map[string]any{"verified": true})
		return
	}
	attemptID, cookieValue, err := newAttemptHandles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	a, code, _, err := h.newEmailAttempt(r, attemptID, cookieValue, EmailAttemptPurposeVerifyAddress, normalizeEmailAddress(user.Email), user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if err := h.store.CreateEmailAttempt(r.Context(), a); err != nil {
		if errors.Is(err, ErrEmailRateLimited) {
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many requests; try again later"})
			return
		}
		log.Printf("[auth] operation=verify_address_begin user_id=%s result=error step=create_attempt error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	h.setAttemptCookie(w, cookieValue)
	h.sendSignInMail(r, a, user.Email, code, "")
	writeJSON(w, http.StatusOK, emailBeginResponse{AttemptID: attemptID})
}

// HandleVerifyAddressFinish handles POST /auth/email/verify-address/finish
// {attempt_id, code}: it sets users.email_verified_at.
func (h *Handler) HandleVerifyAddressFinish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	user := h.requirePasskeySession(w, r)
	if user == nil {
		return
	}
	if !h.EmailSignInEnabled() {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "email sign-in is not available"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return
	}
	if !h.allowVerify(hashIP(clientIP(r))) {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many attempts; try again later"})
		return
	}
	var req emailVerifyRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if _, err := h.consumeTypedCode(r, EmailAttemptPurposeVerifyAddress, req.AttemptID, req.Code, user.ID); err != nil {
		h.failEmailVerify(w, "verify_address_finish", "email_code", err)
		return
	}
	if err := h.store.SetEmailVerified(user.ID); err != nil {
		log.Printf("[auth] operation=verify_address_finish user_id=%s result=error error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	h.clearAttemptCookie(w)
	log.Printf("[auth] operation=verify_address_finish user_id=%s result=success", user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "verified": true})
}

// --- the passkey gate (T5) ---

// accessCookieMethod is the method claim of a valid access cookie, passkey
// when there is none.
func (h *Handler) accessCookieMethod(r *http.Request) string {
	cookie, err := r.Cookie(AccessTokenCookieName)
	if err != nil || cookie.Value == "" {
		return AuthMethodPasskey
	}
	_, m, err := h.validateAccessTokenMethod(cookie.Value)
	if err != nil {
		return AuthMethodPasskey
	}
	return m
}

// sessionMethodFromRequest is the method of the session a request presents:
// the access cookie's claim, else the refresh row's (an expired access token
// still names the session through its refresh cookie). ok is false when
// neither can be read: the method is indeterminate.
func (h *Handler) sessionMethodFromRequest(r *http.Request) (string, bool) {
	if cookie, err := r.Cookie(AccessTokenCookieName); err == nil && cookie.Value != "" {
		if _, m, err := h.validateAccessTokenMethod(cookie.Value); err == nil {
			return m, true
		}
	}
	if cookie, err := r.Cookie(RefreshTokenCookieName); err == nil && cookie.Value != "" {
		if rs, err := h.store.GetRefreshSessionByTokenHash(sha256SumHex(cookie.Value)); err == nil {
			return normalizeAuthMethod(rs.AuthMethod), true
		}
	}
	return "", false
}

// requirePasskeySession is requireAuthUser plus the "am" claim not email_*.
// It writes a 401 or a 403 and returns nil on refusal. It guards what outlives
// a session: credentials, keys, other sessions and the data ROOT.
func (h *Handler) requirePasskeySession(w http.ResponseWriter, r *http.Request) *User {
	cookie, err := r.Cookie(AccessTokenCookieName)
	if err != nil || cookie.Value == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return nil
	}
	userID, method, err := h.validateAccessTokenMethod(cookie.Value)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return nil
	}
	user, err := h.store.GetUserByID(userID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return nil
	}
	if isEmailMethod(method) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: passkeyRequiredMessage})
		return nil
	}
	return user
}

// requirePasskeySessionAny is requireAuthUserAny with the same gate on a
// cookie session. A Bearer API key is never an email session and keeps its
// existing scope rules.
func (h *Handler) requirePasskeySessionAny(w http.ResponseWriter, r *http.Request) (*User, *APIKey) {
	user, key := h.requireAuthUserAny(w, r)
	if user == nil {
		return nil, nil
	}
	if key == nil && isEmailMethod(h.accessCookieMethod(r)) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: passkeyRequiredMessage})
		return nil, nil
	}
	return user, key
}

// --- sweeper ---

// StartEmailSweeper deletes sign-in attempts older than 24 hours every
// interval until ctx ends. The rows are the rate-limit history, so 24 hours is
// the floor.
func (h *Handler) StartEmailSweeper(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if n, err := h.store.SweepEmailAttempts(ctx, time.Now().UTC().Add(-EmailAttemptRetention)); err != nil {
					log.Printf("[auth] operation=email_sweep result=error error=%q", err)
				} else if n > 0 {
					log.Printf("[auth] operation=email_sweep deleted=%d", n)
				}
			}
		}
	}()
}

// --- user agent and place summaries for the mail text ---

// summarizeUserAgent names a browser and system coarsely ("Safari on macOS").
func summarizeUserAgent(ua string) string {
	l := strings.ToLower(ua)
	browser := "a browser"
	switch {
	case strings.Contains(l, "choir"):
		browser = "the Choir app"
	case strings.Contains(l, "edg/"):
		browser = "Edge"
	case strings.Contains(l, "firefox"):
		browser = "Firefox"
	case strings.Contains(l, "chrome") || strings.Contains(l, "crios"):
		browser = "Chrome"
	case strings.Contains(l, "safari"):
		browser = "Safari"
	}
	system := ""
	switch {
	case strings.Contains(l, "iphone"):
		system = "iPhone"
	case strings.Contains(l, "ipad"):
		system = "iPad"
	case strings.Contains(l, "android"):
		system = "Android"
	case strings.Contains(l, "mac os") || strings.Contains(l, "macintosh") || strings.Contains(l, "darwin"):
		system = "macOS"
	case strings.Contains(l, "windows"):
		system = "Windows"
	case strings.Contains(l, "linux"):
		system = "Linux"
	}
	if system == "" {
		return browser
	}
	return browser + " on " + system
}

// summarizePlace names the network a request came from, coarsely: the IPv4
// address with its last octet masked, or the IPv6 /48. There is no geo lookup
// in the auth service; the network is what it can say.
func summarizePlace(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return "an unknown network"
	}
	if v4 := parsed.To4(); v4 != nil {
		return fmt.Sprintf("network %d.%d.%d.x", v4[0], v4[1], v4[2])
	}
	b := parsed.To16()
	return fmt.Sprintf("network %x:%x:%x::/48", uint16(b[0])<<8|uint16(b[1]), uint16(b[2])<<8|uint16(b[3]), uint16(b[4])<<8|uint16(b[5]))
}
