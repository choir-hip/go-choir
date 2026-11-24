package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

// Creating an account is two steps that cannot be separated, the web's and the
// phone app's alike (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md):
//
//	POST /auth/register/mail  {email, binding}                      mail an 8-digit code to the address
//	POST /auth/register/begin {email, attempt_id, code, binding}    spend the code, get passkey options
//	POST /auth/register/finish | /auth/app/register/finish          make the account, the passkey and the proof together
//
// No account, and no row bound to the address, exists before finish. The code
// is the existing email-attempt machinery (email_signin_attempts, purpose
// register, no user): single-use, ten minutes, five wrong tries, limits per
// address and per IP, stored only as a keyed hash. `binding` is a secret the
// client makes and keeps (the web's attempt cookie does this job for sign-in):
// a code works only for the client that asked for it, and a phone has no cookie
// jar to rely on.

const (
	// registerPendingTTL is how long the passkey sheet may stay open after the code is spent.
	registerPendingTTL        = SessionChallengeTTL
	registerProofRequiredCode = "proof_required"
)

type registerMailRequest struct {
	Email   string `json:"email"`
	Binding string `json:"binding"`
}

type registerBeginRequest struct {
	Email     string `json:"email"`
	AttemptID string `json:"attempt_id"`
	Code      string `json:"code"`
	Binding   string `json:"binding"`
}

type proofRequiredResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// validBinding is a client-made secret of at least 128 bits, base64url or hex, not a guess.
func validBinding(b string) bool {
	if len(b) < 32 || len(b) > 256 {
		return false
	}
	for _, c := range b {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return false
		}
	}
	return true
}

// proofAvailable is false when the service cannot mail a code (no HMAC key, no
// mailer): sign-up then refuses, it does not fall back to an unproved account.
func (h *Handler) proofAvailable() bool {
	return h.config != nil && len(h.config.EmailCodeKey) >= minEmailCodeKeyLen && h.signInMailer != nil
}

// HandleRegisterMail handles POST /auth/register/mail. It answers the same
// shape for every well-formed address. A new address gets a code; an address
// that has an account gets the recovery link instead (the mail is its proof),
// so nobody can attach a second holder to it and nothing says which it was.
func (h *Handler) HandleRegisterMail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return
	}
	var req registerMailRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "email is required"})
		return
	}
	if !isValidEmail(email) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "please enter a valid email address"})
		return
	}
	if !validBinding(req.Binding) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "binding is required"})
		return
	}
	if !h.proofAvailable() {
		log.Printf("[auth] operation=register_mail result=error step=proof_unavailable")
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "sign-up is unavailable right now; please try again later"})
		return
	}
	emailHash := hashEmail(email)

	// The answer must not say whether the address has an account: the same
	// shape, the same limits, the same refusals for both. An address that has
	// one is looked up without regard to case (one mailbox, one account).
	existing, lerr := h.store.GetUserByEmail(email)
	known := lerr == nil && existing != nil
	if !known && !errors.Is(lerr, sql.ErrNoRows) {
		log.Printf("[auth] operation=register_mail email_hash=%s result=error step=lookup error=%q", emailHash, lerr)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	// The cap refuses a known address too (it is told the same thing; only a new
	// address is put on the waitlist), before any mail goes out.
	waitlisted := email
	if known {
		waitlisted = ""
	}
	if h.refuseIfRegistrationClosed(w, "register_mail", emailHash, "", waitlisted) {
		return
	}

	attemptID, err := randomHex(16)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	code, err := newEmailCode()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	now := time.Now().UTC()
	a := &EmailAttempt{
		ID:               attemptID,
		Purpose:          EmailAttemptPurposeRegister,
		EmailHash:        h.emailAddressHash(normalizeEmailAddress(email)),
		CodeHash:         h.emailCodeHash(attemptID, EmailAttemptPurposeRegister, code),
		LinkTokenHash:    sha256SumHex("nolink" + attemptID + code),
		CookieHash:       sha256SumHex(req.Binding),
		IPHash:           hashIP(clientIP(r)),
		UserAgentSummary: summarizeUserAgent(r.UserAgent()),
		CreatedAt:        now,
		ExpiresAt:        now.Add(EmailAttemptTTL),
	}
	if known {
		// A known address gets a row that matches nothing, and counts against the
		// same limits, so the 429 comes at the same request for either kind.
		junk, jerr := randomToken(32)
		if jerr != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
			return
		}
		a.CodeHash = h.emailKeyed("nomatch", junk)
	}
	if err := h.store.CreateEmailAttempt(r.Context(), a); err != nil {
		if errors.Is(err, ErrEmailRateLimited) {
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many requests; try again later"})
			return
		}
		log.Printf("[auth] operation=register_mail email_hash=%s result=error step=create_attempt error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if known {
		// The holder proves the address by the recovery mail. Its own limits are
		// not shown to the caller: past them no mail goes out and the answer is the same.
		if status, _ := h.startRecovery(r, email); status != http.StatusOK {
			log.Printf("[auth] operation=register_mail email_hash=%s result=recovery_not_sent status=%d", emailHash, status)
		}
	} else {
		// The mail says "verify your address": maild has the two purposes it has.
		mailed := *a
		mailed.Purpose = EmailAttemptPurposeVerifyAddress
		h.sendSignInMail(r, &mailed, email, code, "")
	}
	log.Printf("[auth] operation=register_mail attempt=%s email_hash=%s result=created", shortHash(attemptID), emailHash)
	writeJSON(w, http.StatusOK, emailBeginResponse{AttemptID: attemptID})
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// HandleRegisterBegin handles POST /auth/register/begin. It spends the mailed
// code and returns passkey creation options for an account that does not exist
// yet: a user id made now, held in a pending row with the proved address. It
// writes no user, no credential and nothing bound to the address.
func (h *Handler) HandleRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "content type must be application/json"})
		return
	}
	var req registerBeginRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "email is required"})
		return
	}
	if !isValidEmail(email) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "please enter a valid email address"})
		return
	}
	if req.AttemptID == "" || req.Code == "" || !validBinding(req.Binding) {
		writeJSON(w, http.StatusBadRequest, proofRequiredResponse{Error: "verify your address first: a code was mailed to it", Code: registerProofRequiredCode})
		return
	}
	if !h.proofAvailable() {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "sign-up is unavailable right now; please try again later"})
		return
	}
	emailHash := hashEmail(email)
	if len(req.AttemptID) != 32 || !isEightDigits(req.Code) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: genericEmailVerifyError})
		return
	}
	if !h.allowVerify(hashIP(clientIP(r))) {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many attempts; try again later"})
		return
	}
	want := h.emailCodeHash(req.AttemptID, EmailAttemptPurposeRegister, req.Code)
	if _, err := h.store.ConsumeEmailCode(r.Context(), req.AttemptID, sha256SumHex(req.Binding), EmailAttemptPurposeRegister, "",
		h.emailAddressHash(normalizeEmailAddress(email)), func(stored string) bool { return hmac.Equal([]byte(stored), []byte(want)) }); err != nil {
		h.failEmailVerify(w, "register_begin", "email_code", err)
		return
	}
	// The code is spent. The address must still have no account (it may have got
	// one between the mail and now); the cap is re-checked.
	if _, err := h.store.GetUserByEmail(email); err == nil {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "email already registered"})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		log.Printf("[auth] operation=register_begin email_hash=%s result=error step=lookup error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if h.refuseIfRegistrationClosed(w, "register_begin", emailHash, "", email) {
		return
	}

	// The account does not exist: a user in memory carries the id and the name
	// the passkey is made for.
	pending := &User{ID: uuid.NewString(), Email: email, CreatedAt: time.Now().UTC()}
	waUser, err := newWebAuthnUser(pending, nil)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	creation, session, err := h.webauthn.BeginRegistration(waUser,
		webauthn.WithConveyancePreference("none"),
		webauthn.WithAuthenticatorSelection(
			webauthn.SelectAuthenticator("platform", nil, "required"),
		),
		webauthn.WithExtensions(protocol.AuthenticationExtensions{
			"prf": map[string]any{},
		}),
	)
	if err != nil {
		log.Printf("[auth] operation=register_begin email_hash=%s result=error step=begin_registration error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to begin registration"})
		return
	}
	raw, err := json.Marshal(session)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	now := time.Now().UTC()
	if err := h.store.SavePendingRegistration(&PendingRegistration{
		Challenge: session.Challenge, Email: email, UserID: pending.ID, SessionData: string(raw),
		CreatedAt: now, ExpiresAt: now.Add(registerPendingTTL),
	}); err != nil {
		log.Printf("[auth] operation=register_begin email_hash=%s result=error step=save_pending error=%q", emailHash, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save challenge"})
		return
	}
	log.Printf("[auth] operation=register_begin email_hash=%s result=success challenge_type=registration proved=true", emailHash)
	writeJSON(w, http.StatusOK, creation)
}

const addressUnprovedCode = "address_unproved"

// mayMint is true for an account whose address was proved (every account made
// by register/finish or by a mail-proved recovery) and for the operator's
// synthetic bootstrap account. Anything else is a row from before the proof
// rule: it signs in to nothing and mints nothing until recovery proves it.
func mayMint(u *User) bool {
	return u != nil && (u.EmailVerifiedAt != nil || u.ID == BootstrapAdminAPIKeyUserID)
}

func writeUnproved(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, proofRequiredResponse{
		Error: "this account's address has not been verified: recover the account by mail to sign in",
		Code:  addressUnprovedCode,
	})
}
