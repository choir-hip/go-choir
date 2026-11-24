package auth

// Account deletion routes (docs/design/account-create-delete-2026-11-23.md,
// the deletion half, with the owner's rulings of 2026-11-23):
//
//	POST /auth/account/delete/begin   a fresh deletion assertion challenge
//	POST /auth/account/delete/finish  the assertion starts the deletion
//	POST /auth/account/delete/cancel  a passkey sign-in's one-use offer
//	                                  cancels a pending deletion
//
// What starts a deletion: a passkey assertion made for this purpose, answered
// within SessionChallengeTTL (five minutes) of its begin, with user
// verification required, by a credential of the account that the caller
// (a signed-in browser session, or an owner-wide bearer key) is. The caller
// alone is not enough in either direction: a session, an email session or an
// old session cannot start it without the assertion, and an assertion with no
// caller is refused because the challenge is bound to the account the caller
// proved at begin. The challenge lives in its own table, so it cannot be
// finished as a sign-in.
//
// Ways this could go wrong, written before the code (each has a test in
// deletion_test.go):
//  1. a session, email session or bearer key starts a deletion with no
//     assertion;
//  2. a sign-in challenge is replayed to start one, or a deletion challenge
//     is finished as a sign-in;
//  3. an assertion minutes old (past the challenge TTL) starts one;
//  4. an assertion for another account's challenge starts this one;
//  5. a deletion leaves a session, key or exchange code alive, or the
//     access token already signed keeps working;
//  6. a passkey sign-in of a pending account mints a session, or reveals the
//     state without an assertion;
//  7. the cancel token works twice, after expiry, or once the purge began;
//  8. a computer-bound key (an agent's) starts a deletion;
//  9. a protected account (the paper, the owner) is accepted.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/yusefmosiah/go-choir/internal/appids"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// DeletionHost is the host side of a request, implemented by the purge
// runner (internal/accountpurge) and wired in cmd/auth.
type DeletionHost interface {
	// Precheck refuses an account that may never be deleted by this path (it
	// owns a critical_protected or public_platform computer, or a computer
	// that is held or has a live capsule) and fails closed when the
	// computers cannot be listed.
	Precheck(ctx context.Context, userID string) error
	// AtOnce runs the steps that follow the revocation at once: the
	// account's computers are stopped and its published takes stop being
	// served. A failure is retried by the runner's loop.
	AtOnce(ctx context.Context, userID string) error
	// Restore undoes AtOnce after a cancel: takes are served again. The
	// computer starts on the owner's next request.
	Restore(ctx context.Context, userID string) error
}

// SetDeletionHost wires the host side.
func (h *Handler) SetDeletionHost(host DeletionHost) { h.deletionHost = host }

// ErrDeletionRefused is a Precheck refusal the person is told about.
var ErrDeletionRefused = errors.New("this account cannot be deleted here")

// SetDeletionRefuser names the accounts a person may not delete (the reviewer
// account: the reviewer-account lane passes Handler.IsDemoAccount here, in
// cmd/auth). nil refuses nothing.
func (h *Handler) SetDeletionRefuser(f func(userID string) bool) { h.deletionRefuser = f }

func (h *Handler) deletionRefused(userID string) bool {
	return h.deletionRefuser != nil && h.deletionRefuser(userID)
}

func writeDeletionPending(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, deletionPendingResponse{Error: "account_deletion_pending"})
}

// writeDeletionPurging answers a sign-in or a registration of an account whose
// purge has begun: nothing can be cancelled and nothing is offered.
func writeDeletionPurging(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, deletionPendingResponse{Error: "account_deletion_purging"})
}

// writeDeletionState answers with the state the account is really in.
func writeDeletionState(w http.ResponseWriter, store *Store, userID string) {
	if d, err := store.AccountDeletionFor(userID); err == nil && d != nil && d.State == DeletionPurging {
		writeDeletionPurging(w)
		return
	}
	writeDeletionPending(w)
}

type deletionPendingResponse struct {
	Error string `json:"error"`
}

type deletionOffer struct {
	Error       string `json:"error"`
	PurgeAfter  string `json:"purge_after"`
	RequestedAt string `json:"requested_at"`
	CancelToken string `json:"cancel_token"`
}

// offerDeletionCancel answers a verified passkey sign-in of an account whose
// deletion is pending: no session, the cancel offer. It reports whether it
// answered.
func (h *Handler) offerDeletionCancel(w http.ResponseWriter, userID string) bool {
	d, err := h.store.AccountDeletionFor(userID)
	if err != nil {
		log.Printf("[auth] operation=login_finish user_id=%s result=error step=deletion_lookup error=%q", userID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return true
	}
	if d == nil {
		return false
	}
	if d.State != DeletionPending {
		// The purge has begun; there is nothing to cancel and no session, and
		// the person is told so (never offered a cancel that cannot work).
		writeDeletionPurging(w)
		return true
	}
	token, err := h.store.OfferDeletionCancel(userID, time.Now().UTC())
	if errors.Is(err, ErrDeletionNotCancellable) {
		// The 72 hours are over and the runner has not yet picked the record
		// up: the removal is due, there is nothing to offer.
		writeDeletionPurging(w)
		return true
	}
	if err != nil {
		writeDeletionPending(w)
		return true
	}
	log.Printf("[auth] operation=login_finish user_id=%s result=deletion_pending step=cancel_offered", userID)
	writeJSON(w, http.StatusForbidden, deletionOffer{
		Error: "account_deletion_pending", CancelToken: token,
		PurgeAfter: d.PurgeAfter.UTC().Format(time.RFC3339), RequestedAt: d.RequestedAt.UTC().Format(time.RFC3339),
	})
	return true
}

// deletionCaller identifies the caller of begin and finish: a browser session
// or an owner-wide bearer key. A key bound to one computer (an agent's) is
// refused.
func (h *Handler) deletionCaller(w http.ResponseWriter, r *http.Request) *User {
	user, key := h.requireAuthUserAny(w, r)
	if user == nil {
		return nil
	}
	if key != nil && strings.TrimSpace(key.ComputerID) != "" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "a key bound to one computer cannot delete the account"})
		return nil
	}
	return user
}

// HandleDeleteBegin handles POST /auth/account/delete/begin.
func (h *Handler) HandleDeleteBegin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	user := h.deletionCaller(w, r)
	if user == nil {
		return
	}
	if h.deletionRefused(user.ID) {
		log.Printf("[auth] operation=delete_begin user_id=%s result=refused reason=reviewer_account", user.ID)
		writeJSON(w, http.StatusForbidden, errorResponse{Error: ErrDeletionRefused.Error()})
		return
	}
	if h.deletionHost != nil {
		if err := h.deletionHost.Precheck(r.Context(), user.ID); err != nil {
			h.writePrecheckRefusal(w, user.ID, err)
			return
		}
	}
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil || len(creds) == 0 {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "the account has no passkey to confirm with"})
		return
	}
	waUser, err := newWebAuthnUser(user, creds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	assertion, session, err := h.webauthn.BeginLogin(waUser, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to begin"})
		return
	}
	raw, err := json.Marshal(session)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	now := time.Now().UTC()
	if err := h.store.SaveDeletionChallenge(session.Challenge, user.ID, string(raw), now, now.Add(SessionChallengeTTL)); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to save challenge"})
		return
	}
	log.Printf("[auth] operation=delete_begin user_id=%s result=success", user.ID)
	writeJSON(w, http.StatusOK, assertion)
}

func (h *Handler) writePrecheckRefusal(w http.ResponseWriter, userID string, err error) {
	if errors.Is(err, ErrDeletionRefused) {
		log.Printf("[auth] operation=delete result=refused user_id=%s", userID)
		writeJSON(w, http.StatusForbidden, errorResponse{Error: err.Error()})
		return
	}
	// The computers could not be listed: fail closed, say to try again.
	log.Printf("[auth] operation=delete result=unavailable user_id=%s error=%q", userID, err)
	writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "the deletion could not be checked just now; try again in a minute"})
}

type deleteFinishResponse struct {
	OK          bool   `json:"ok"`
	JobID       string `json:"job_id"`
	RequestedAt string `json:"requested_at"`
	PurgeAfter  string `json:"purge_after"`
}

// HandleDeleteFinish handles POST /auth/account/delete/finish: the body is
// the WebAuthn assertion.
func (h *Handler) HandleDeleteFinish(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	caller := h.deletionCaller(w, r)
	if caller == nil {
		return
	}
	if h.deletionRefused(caller.ID) {
		log.Printf("[auth] operation=delete_finish user_id=%s result=refused reason=reviewer_account", caller.ID)
		writeJSON(w, http.StatusForbidden, errorResponse{Error: ErrDeletionRefused.Error()})
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	challengeID, err := extractChallengeFromBody(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid webauthn response"})
		return
	}
	userID, sessionJSON, created, expires, err := h.store.TakeDeletionChallenge(challengeID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "challenge not found or already used"})
		return
	}
	now := time.Now().UTC()
	if userID != caller.ID {
		// A challenge made for another account (or by a caller that is not
		// the account) never starts this deletion.
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "challenge is not for this account"})
		return
	}
	if now.After(expires) || now.Sub(created) > SessionChallengeTTL {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "the confirmation is too old; confirm again"})
		return
	}
	var session webauthn.SessionData
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid session data"})
		return
	}
	creds, err := h.store.GetCredentialsByUserID(caller.ID)
	if err != nil || len(creds) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "user has no credentials"})
		return
	}
	waUser, err := newWebAuthnUser(caller, creds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid assertion"})
		return
	}
	credential, err := h.webauthn.ValidateLogin(waUser, session, parsed)
	if err != nil {
		log.Printf("[auth] operation=delete_finish user_id=%s result=error step=verify error=%q", caller.ID, err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "confirmation failed"})
		return
	}
	_ = h.store.UpdateCredentialSignCount(string(credential.ID), int64(credential.Authenticator.SignCount))
	_ = h.store.TouchCredentialLastUsed(string(credential.ID))

	if h.deletionHost != nil {
		if err := h.deletionHost.Precheck(r.Context(), caller.ID); err != nil {
			h.writePrecheckRefusal(w, caller.ID, err)
			return
		}
	}
	d, _, err := h.store.RequestAccountDeletion(caller.ID, now)
	if err != nil {
		log.Printf("[auth] operation=delete_finish user_id=%s result=error step=request error=%q", caller.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "could not record the deletion"})
		return
	}
	h.revokeRotatedGraceForUser(caller.ID)
	// Access is revoked in the store; clear this browser's cookies too.
	h.clearAuthCookies(w)
	if h.deletionHost != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Second)
		if err := h.deletionHost.AtOnce(ctx, caller.ID); err != nil {
			// The record stands and the runner retries; the person is not
			// told "failed", because nothing they can do would help.
			log.Printf("[auth] operation=delete_finish user_id=%s result=at_once_deferred error=%q", caller.ID, err)
		}
		cancel()
	}
	log.Printf("[auth] operation=delete_finish user_id=%s job_id=%s result=success", caller.ID, d.JobID)
	writeJSON(w, http.StatusOK, deleteFinishResponse{OK: true, JobID: d.JobID,
		RequestedAt: d.RequestedAt.UTC().Format(time.RFC3339), PurgeAfter: d.PurgeAfter.UTC().Format(time.RFC3339)})
}

type cancelRequest struct {
	CancelToken string `json:"cancel_token"`
	// App, when it names a known app (internal/appids), asks for the app's
	// key instead of a browser session: the phone signs in with a key.
	App string `json:"app,omitempty"`
}

// HandleDeleteCancel handles POST /auth/account/delete/cancel: the one-use
// token a verified passkey sign-in was offered cancels the pending deletion
// and signs the person in. Nothing was purged, so nothing is restored from
// anywhere: takes are served again and the computer starts on use.
func (h *Handler) HandleDeleteCancel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req cancelRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	now := time.Now().UTC()
	// Validate everything that can refuse before anything is cancelled: a
	// refused request must leave the deletion and the offer as they were.
	if req.App != "" && !appids.Known(req.App) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unknown app"})
		return
	}
	peek, err := h.store.PeekCancelOffer(req.CancelToken, now)
	switch {
	case errors.Is(err, ErrDeletionNotCancellable):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "the deletion has begun and cannot be cancelled"})
		return
	case err != nil || peek == nil:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "the cancel offer is invalid or expired; sign in again"})
		return
	}
	// The cancel is the one durable act and comes first: it leaves a tombstone
	// the runner finishes. The takes are served again after it, never before,
	// so a pass that was holding them back cannot undo the restore (it finds
	// the tombstone and restores them itself).
	d, err := h.store.CancelAccountDeletion(req.CancelToken, now)
	switch {
	case errors.Is(err, ErrDeletionNotCancellable):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "the deletion has begun and cannot be cancelled"})
		return
	case err != nil || d == nil:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "the cancel offer is invalid or expired; sign in again"})
		return
	}
	restored := true
	if h.deletionHost != nil {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Second)
		err := h.deletionHost.Restore(ctx, d.UserID)
		cancel()
		if err != nil {
			// Not fatal: the tombstone stays and the runner serves the takes
			// again on its next pass. The person is signed in either way.
			log.Printf("[auth] operation=delete_cancel user_id=%s job_id=%s result=restore_deferred error=%q", d.UserID, d.JobID, err)
			restored = false
		}
	}
	if restored {
		if err := h.store.FinishCancelledDeletion(d.UserID); err != nil {
			log.Printf("[auth] operation=delete_cancel user_id=%s job_id=%s result=warning step=finish_tombstone error=%q", d.UserID, d.JobID, err)
		}
	}
	user, err := h.store.GetUserByID(d.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal error"})
		return
	}
	if req.App != "" {
		key, err := h.mintAppKey(r.Context(), user, req.App)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create the key"})
			return
		}
		log.Printf("[auth] operation=delete_cancel user_id=%s job_id=%s app=%s result=success", d.UserID, d.JobID, req.App)
		writeJSON(w, http.StatusOK, key)
		return
	}
	info, err := h.signInSession(w, r, user, AuthMethodPasskey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create session"})
		return
	}
	log.Printf("[auth] operation=delete_cancel user_id=%s job_id=%s result=success", d.UserID, d.JobID)
	writeJSON(w, http.StatusOK, finishResponse{OK: true, User: info})
}

// compile-time use of the grace constant in this package's contract.
var _ = purgeguard.GracePeriod
