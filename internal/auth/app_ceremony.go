package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/yusefmosiah/go-choir/internal/appids"
)

// The phone app signs in and creates an account with the platform's passkey
// screen, inside the app (owner, 2026-11-23). The ceremony is the web's own
// (register/begin, then a finish), with two differences: the finish ends in a
// bearer key for the app, with the grant the device approval gives, and the
// sign-in ceremony needs no email (a discoverable credential).
//
//	POST /auth/register/begin     {email}            the web's route, unchanged: creation options
//	POST /auth/app/login/begin                       no body: assertion options with no allow list
//	POST /auth/app/register/finish {app, credential} verifies, stores the passkey, mints the key
//	POST /auth/app/login/finish    {app, credential} verifies, mints the key
//
// `app` is a bundle id from internal/appids, never free text. The credential is
// the WebAuthn response exactly as the web posts it. The origin a native iOS
// passkey reports is https://<the relying party id> (Apple Developer Forums;
// Exact's passkeys manifest `clientOrigin.ios`), which is the one origin
// AUTH_RP_ORIGINS already lists; no origin is added for the app.
//
// Rate bounds are the existing classes: begin 10 a minute and finish 30 a
// minute per address. A discoverable challenge has no user row to hang on, so
// it is held in memory, 5 minutes, used once. Memory is bounded by what the
// begin rate allows an address, and no address can use up a shared space: each
// address holds at most appChallengePerAddr live challenges (its oldest is
// dropped for a new one), expired entries are swept on every begin, and there
// is no global count that a few addresses could fill to refuse everyone else.
// The auth service is one process, so a restart drops the table and an app
// begins again, which a person does by tapping once more. A stateless signed
// challenge was weighed and not chosen: it cannot be single-use, and a
// replayed assertion would mint a second key.

const (
	appChallengePerAddr = 8
	appKeyLabel         = "Autoradio app: "
)

type appChallenge struct {
	session webauthn.SessionData
	addr    string
	expires time.Time
}

type appChallenges struct {
	mu sync.Mutex
	m  map[string]appChallenge
}

// put keeps a challenge for addr, dropping expired entries and, when addr is at
// its bound, its oldest live challenge. One pass over the table: its size is
// what the begin rate allows within the 5-minute life, not a ceiling.
func (c *appChallenges) put(session webauthn.SessionData, addr string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]appChallenge)
	}
	var mine []string
	for k, v := range c.m {
		if now.After(v.expires) {
			delete(c.m, k)
		} else if v.addr == addr {
			mine = append(mine, k)
		}
	}
	for len(mine) >= appChallengePerAddr {
		oldest := 0
		for i, k := range mine {
			if c.m[k].expires.Before(c.m[mine[oldest]].expires) {
				oldest = i
			}
		}
		delete(c.m, mine[oldest])
		mine = append(mine[:oldest], mine[oldest+1:]...)
	}
	c.m[session.Challenge] = appChallenge{session: session, addr: addr, expires: now.Add(SessionChallengeTTL)}
}

// take removes and returns the challenge: a challenge is used once.
func (c *appChallenges) take(challenge string, now time.Time) (webauthn.SessionData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[challenge]
	if !ok {
		return webauthn.SessionData{}, false
	}
	delete(c.m, challenge)
	if now.After(v.expires) {
		return webauthn.SessionData{}, false
	}
	return v.session, true
}

// appFinishRequest is the body of both finish routes.
type appFinishRequest struct {
	App        string          `json:"app"`
	Credential json.RawMessage `json:"credential"`
}

type appKeyResponse struct {
	APIKey string   `json:"api_key"`
	KeyID  string   `json:"key_id"`
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

// readAppFinish reads the finish body and checks the app name against the
// fixed list. It answers the request and returns false on a refusal.
func readAppFinish(w http.ResponseWriter, r *http.Request) (app string, credential []byte, ok bool) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return "", nil, false
	}
	body, err := readBody(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return "", nil, false
	}
	var req appFinishRequest
	if err := json.Unmarshal(body, &req); err != nil || len(bytes.TrimSpace(req.Credential)) == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "app and credential are required"})
		return "", nil, false
	}
	if !appids.Known(req.App) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unknown app"})
		return "", nil, false
	}
	return req.App, req.Credential, true
}

// writeKeyJSON answers a bearer key: never cached by anything on the way.
func writeKeyJSON(w http.ResponseWriter, key *appKeyResponse) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, key)
}

// mintAppKey gives the app the grant a device approval gives: a full-access key
// (every scope), owner-wide, until revoked.
func (h *Handler) mintAppKey(ctx context.Context, user *User, app string) (*appKeyResponse, error) {
	scopes := AllScopes()
	label := appKeyLabel + app
	id, secret, err := h.store.CreateAPIKey(ctx, user.ID, label, scopes, nil)
	if err != nil {
		return nil, err
	}
	return &appKeyResponse{APIKey: secret, KeyID: id, Label: label, Scopes: scopes}, nil
}

// HandleAppLoginBegin handles POST /auth/app/login/begin: assertion options
// with no allow list and no user, so the platform offers the passkeys it holds
// for this site.
func (h *Handler) HandleAppLoginBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	assertion, session, err := h.webauthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		log.Printf("[auth] operation=app_login_begin result=error step=begin_login error=%q", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to begin login"})
		return
	}
	h.appChallenges.put(*session, clientIP(r), time.Now().UTC())
	writeJSON(w, http.StatusOK, assertion)
}

// HandleAppRegisterFinish handles POST /auth/app/register/finish.
func (h *Handler) HandleAppRegisterFinish(w http.ResponseWriter, r *http.Request) {
	app, credential, ok := readAppFinish(w, r)
	if !ok {
		return
	}
	user, dbCred, ok := h.verifyRegistration(w, credential)
	if !ok {
		return
	}
	key, err := h.mintAppKey(r.Context(), user, app)
	if err != nil {
		log.Printf("[auth] operation=app_register_finish user_id=%s result=error step=mint_key error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create the key"})
		return
	}
	log.Printf("[auth] operation=app_register_finish user_id=%s credential_id=%s key_id=%s app=%s result=success", user.ID, dbCred.ID, key.KeyID, app)
	writeKeyJSON(w, key)
}

// HandleAppLoginFinish handles POST /auth/app/login/finish.
func (h *Handler) HandleAppLoginFinish(w http.ResponseWriter, r *http.Request) {
	app, credential, ok := readAppFinish(w, r)
	if !ok {
		return
	}
	challengeID, err := extractChallengeFromBody(credential)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid webauthn response: " + err.Error()})
		return
	}
	// Taken before anything is verified: a replayed finish finds nothing.
	session, found := h.appChallenges.take(challengeID, time.Now().UTC())
	if !found {
		log.Printf("[auth] operation=app_login_finish result=error step=challenge_lookup")
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "challenge not found or already used"})
		return
	}
	parsed, err := protocol.ParseCredentialRequestResponseBody(bytes.NewReader(credential))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid login response: " + err.Error()})
		return
	}
	var user *User
	lookup := func(rawID, userHandle []byte) (webauthn.User, error) {
		u, err := h.store.GetUserByID(string(userHandle))
		if err != nil {
			return nil, errors.New("no account for this passkey")
		}
		creds, err := h.store.GetCredentialsByUserID(u.ID)
		if err != nil || len(creds) == 0 {
			return nil, errors.New("no account for this passkey")
		}
		user = u
		return newWebAuthnUser(u, creds)
	}
	cred, err := h.webauthn.ValidateDiscoverableLogin(lookup, session, parsed)
	if err != nil || user == nil {
		log.Printf("[auth] operation=app_login_finish result=error step=verify_login error=%q", err)
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "login verification failed"})
		return
	}
	if err := h.store.UpdateCredentialSignCount(string(cred.ID), int64(cred.Authenticator.SignCount)); err != nil {
		log.Printf("[auth] operation=app_login_finish user_id=%s result=warning step=update_sign_count error=%q", user.ID, err)
	}
	if err := h.store.TouchCredentialLastUsed(string(cred.ID)); err != nil {
		log.Printf("[auth] operation=app_login_finish user_id=%s result=warning step=touch_last_used error=%q", user.ID, err)
	}
	key, err := h.mintAppKey(r.Context(), user, app)
	if err != nil {
		log.Printf("[auth] operation=app_login_finish user_id=%s result=error step=mint_key error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create the key"})
		return
	}
	log.Printf("[auth] operation=app_login_finish user_id=%s credential_id=%s key_id=%s app=%s result=success", user.ID, string(cred.ID), key.KeyID, app)
	writeKeyJSON(w, key)
}
