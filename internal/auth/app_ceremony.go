package auth

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
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
//	POST /auth/register/mail      {email, binding}  the web's routes, shared: mail an 8-digit code to the address
//	POST /auth/register/begin     {email, attempt_id, code, binding}  spend the code, get creation options (no account yet)
//	POST /auth/app/login/begin                       no body: assertion options with no allow list
//	POST /auth/app/register/finish {app, credential} verifies, stores the passkey, mints the key
//	POST /auth/app/login/finish    {app, credential} verifies, mints the key
//
// `app` is a bundle id from internal/appids, never free text. The credential is
// the WebAuthn response exactly as the web posts it. The origin a native iOS
// passkey reports is https://<the relying party id> (Apple Developer Forums;
// Exact's passkeys manifest `clientOrigin.ios`), which is one origin
// AUTH_RP_ORIGINS lists. An Android app reports `android:apk-key-hash:<base64url
// of the signing certificate's SHA-256>` (Credential Manager), and that origin
// is listed too, for the package the association file names.
//
// Rate bounds are the existing classes: begin 10 a minute and finish 30 a
// minute per address. A discoverable challenge has no user row to hang on, so
// it is held in memory, 5 minutes, used once.
//
// What a stranger can make this table hold is bounded two ways, and neither
// refuses anyone:
//   - a source holds at most appChallengePerAddr live challenges (its oldest is
//     dropped for a new one); a source is an IPv4 address or an IPv6 /64, since
//     one host owns a whole /64 and would otherwise mint addresses at will;
//   - the table holds at most appChallengeMax in all. At that ceiling the
//     oldest challenge anywhere is dropped for the new one. Every challenge
//     lives 5 minutes, so the table is a queue in expiry order: expired entries
//     are dropped from the front and every put and take is O(1).
//
// The ceiling is reached only by about appChallengeMax/300 begins a second
// from many different sources (each source is held to its begin rate), and
// then the oldest live challenge of whoever is oldest, a victim included, is
// dropped and that person's finish answers "challenge not found": they tap
// again. A stranger who shares the victim's source can do the same with eight
// begins (the per-source bound). Both are denials of one retry, never a
// replay or a refusal of sign-in to others. About 1 KiB an entry puts the
// ceiling near 20 MiB. The auth service is one process, so a restart drops the
// table and an app begins again. A stateless signed challenge was weighed and
// not chosen: single use then needs a used-nonce record anyway, which is this
// table with a different name.

const (
	appChallengePerAddr = 8
	appChallengeMax     = 20000
	appKeyLabel         = "Autoradio app: "
)

type appChallenge struct {
	session webauthn.SessionData
	source  string
	expires time.Time
	elem    *list.Element // position in the expiry queue
}

type appChallenges struct {
	mu       sync.Mutex
	m        map[string]*appChallenge
	order    *list.List          // challenge ids, oldest first
	bySource map[string][]string // source -> its live challenge ids, oldest first
}

// appSource is the bound's unit: an IPv4 address, or the /64 of an IPv6 one.
func appSource(addr string) string {
	if ip := net.ParseIP(addr); ip != nil && ip.To4() == nil {
		return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return addr
}

func (c *appChallenges) drop(id string) {
	v, ok := c.m[id]
	if !ok {
		return
	}
	delete(c.m, id)
	c.order.Remove(v.elem)
	ids := c.bySource[v.source]
	for i, x := range ids {
		if x == id {
			ids = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(ids) == 0 {
		delete(c.bySource, v.source)
	} else {
		c.bySource[v.source] = ids
	}
}

// put keeps a challenge for addr; see the bounds above.
func (c *appChallenges) put(session webauthn.SessionData, addr string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]*appChallenge)
		c.order = list.New()
		c.bySource = make(map[string][]string)
	}
	for e := c.order.Front(); e != nil; e = c.order.Front() {
		if !now.After(c.m[e.Value.(string)].expires) {
			break
		}
		c.drop(e.Value.(string))
	}
	src := appSource(addr)
	c.drop(session.Challenge)
	for len(c.bySource[src]) >= appChallengePerAddr {
		c.drop(c.bySource[src][0])
	}
	for len(c.m) >= appChallengeMax {
		c.drop(c.order.Front().Value.(string))
	}
	v := &appChallenge{session: session, source: src, expires: now.Add(SessionChallengeTTL)}
	v.elem = c.order.PushBack(session.Challenge)
	c.m[session.Challenge] = v
	c.bySource[src] = append(c.bySource[src], session.Challenge)
}

// take removes and returns the challenge: a challenge is used once.
func (c *appChallenges) take(challenge string, now time.Time) (webauthn.SessionData, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[challenge]
	if !ok {
		return webauthn.SessionData{}, false
	}
	c.drop(challenge)
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
	if h.demoOff(user.ID) {
		return nil, ErrAccountOff
	}
	// No key is minted for an account that asked to be deleted.
	if h.store.AccountAccessRevoked(user.ID) {
		return nil, ErrDeletionPending
	}
	scopes := AllScopes()
	label := appKeyLabel + app
	if h.demoMintHook != nil {
		h.demoMintHook()
	}
	id, secret, err := h.store.CreateAPIKey(ctx, user.ID, label, scopes, nil)
	if err != nil {
		return nil, err
	}
	// A mint that raced `off` (demo_account.go) is taken back.
	if h.demoOff(user.ID) {
		_ = h.store.RevokeAPIKey(ctx, user.ID, id)
		return nil, ErrAccountOff
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
	user, dbCred, ok := h.verifyRegistration(w, r, credential)
	if !ok {
		return
	}
	key, err := h.mintAppKey(r.Context(), user, app)
	if errors.Is(err, ErrDeletionPending) {
		writeDeletionState(w, h.store, user.ID)
		return
	}
	if err != nil {
		writeIssueError(w, err, "app_register_finish", user.ID, "failed to create the key")
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
	// A verified passkey of an account that asked to be deleted gets the
	// cancel offer and no key (deletion_handlers.go).
	if h.offerDeletionCancel(w, user.ID) {
		return
	}
	key, err := h.mintAppKey(r.Context(), user, app)
	if errors.Is(err, ErrAddressUnproved) {
		log.Printf("[auth] operation=app_login_finish user_id=%s result=refused reason=address_unproved", user.ID)
		writeUnproved(w)
		return
	}
	if err != nil {
		writeIssueError(w, err, "app_login_finish", user.ID, "failed to create the key")
		return
	}
	log.Printf("[auth] operation=app_login_finish user_id=%s credential_id=%s key_id=%s app=%s result=success", user.ID, string(cred.ID), key.KeyID, app)
	writeKeyJSON(w, key)
}
