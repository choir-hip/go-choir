package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/appids"
	"golang.org/x/crypto/argon2"
)

// The reviewer sign-in (owner, 2026-11-24: "yes, we should make a demo
// account as well"). Apple's reviewers cannot hold a passkey for this site and
// sign-up may be closed at the cap, so one configured account can be opened
// with a user name and a long random password.
//
//	POST /auth/app/demo/login {app, username, password}  ->  the app's key
//
// What makes it narrow:
//   - The account is the id in AUTH_DEMO_ACCOUNT_ID_FILE, never anything in the request; the
//     body refuses unknown fields, so a request cannot even try to name one.
//   - The credential is a host file (AUTH_DEMO_CREDENTIAL_FILE): line 1 the
//     user name, line 2 an argon2id hash in the standard encoded form. It is
//     read on every attempt, so removing the file turns the route off at once
//     and replacing it rotates the password at once. No file, no account id,
//     or a user row that does not exist: 404 "not available".
//   - The key carries DemoScopes only (see there for what is withheld), and
//     expires.
//   - Bounds (below) are on attempts, never on the account: nobody can lock
//     the real reviewer out by guessing, and every bound lifts by itself.

// DemoScopes is the grant of a reviewer key: the data scopes the app's pages
// use (runtime, texture, base, read and write), nothing else. Withheld, against
// the full app key (AllScopes):
//
//	owner:acts                         arm, approve, restore, rematerialize, package publish and install
//	admin                              implies every scope above; also lets a key act as any scope
//	manage:keys                        minting, listing and revoking keys, so no second key and no escape from the expiry
//	computer:lifecycle                 computer update, hibernate and lifecycle acts
//	computer:self_development:*        the owner's self-development controls
//	precommit:read/write, acceptance:read   the calibration and acceptance surfaces
//
// Already refused to every API key by the proxy, whatever its scopes: sending
// mail and the digest (isEmailOwnerActRoute) and publishing as an opinion
// (cookie only). Account deletion needs a fresh passkey assertion and a
// session, which this account's reviewer does not hold. What a scope cannot
// withhold is named in docs/ship/demo-account.md (model use on the demo
// computer; a take the reviewer makes).
var DemoScopes = []string{"read:base", "read:runtime", "read:texture", "write:base", "write:runtime", "write:texture"}

const (
	// DemoKeyTTL is how long a reviewer key lives. An Apple review and a
	// TestFlight round fit well inside it; the reset command revokes sooner.
	DemoKeyTTL = 30 * 24 * time.Hour

	// DemoAttemptsPerAddress attempts (right or wrong) per address in
	// DemoAddressWindow; the window expires by itself. An address is an IPv4
	// address or an IPv6 /64.
	DemoAttemptsPerAddress = 5
	DemoAddressWindow      = 10 * time.Minute

	// DemoAttemptsGlobal attempts across all addresses in DemoGlobalWindow.
	// This is the only bound a crowd can use against the reviewer, and only
	// while the crowd keeps coming: it lifts when the window passes and never
	// counts against the account. The password is random and long, so the
	// bounds exist for cost and noise, not as the lock.
	DemoAttemptsGlobal = 60
	DemoGlobalWindow   = 10 * time.Minute

	demoVerifyParallel = 2 // argon2 uses 64 MiB; never more than this at once
	demoMinPassword    = 24
	demoMaxField       = 256
	demoMaxTracked     = 4096
)

type demoGate struct {
	mu     sync.Mutex
	now    func() time.Time
	addr   map[string][]time.Time
	global []time.Time
	sem    chan struct{}
}

func newDemoGate(now func() time.Time) *demoGate {
	return &demoGate{now: now, addr: make(map[string][]time.Time), sem: make(chan struct{}, demoVerifyParallel)}
}

func recent(ts []time.Time, now time.Time, window time.Duration) []time.Time {
	i := 0
	for i < len(ts) && !now.Before(ts[i].Add(window)) {
		i++
	}
	return ts[i:]
}

func demoAddrKey(ip string) string {
	p := net.ParseIP(ip)
	if p == nil {
		return "unknown"
	}
	if v4 := p.To4(); v4 != nil {
		return v4.String()
	}
	return p.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// allow counts one attempt. It answers how long until the bound that refused
// it frees.
func (g *demoGate) allow(ip string) (bool, time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	key := demoAddrKey(ip)
	if len(g.addr) >= demoMaxTracked {
		for k, v := range g.addr {
			if len(recent(v, now, DemoAddressWindow)) == 0 {
				delete(g.addr, k)
			}
		}
		if _, known := g.addr[key]; !known && len(g.addr) >= demoMaxTracked {
			key = "overflow" // a flood of new addresses shares one bucket
		}
	}
	a := recent(g.addr[key], now, DemoAddressWindow)
	if len(a) >= DemoAttemptsPerAddress {
		g.addr[key] = a
		return false, a[0].Add(DemoAddressWindow).Sub(now)
	}
	gl := recent(g.global, now, DemoGlobalWindow)
	if len(gl) >= DemoAttemptsGlobal {
		g.global = gl
		return false, gl[0].Add(DemoGlobalWindow).Sub(now)
	}
	g.addr[key] = append(a, now)
	g.global = append(gl, now)
	return true, 0
}

// ---- the credential file ----

type demoHash struct {
	m, t uint32
	p    uint8
	salt []byte
	sum  []byte
}

// FormatDemoCredential makes the contents of the credential file for a user
// name and a password. The generating command (cmd/demo-account) uses it.
// validDemoName is the one rule for the reviewer's user name, shared by the command that makes the
// credential and the service that reads it: one short line, no "@" (the app takes a name with an
// "@" for an email address), no space at either end.
func validDemoName(name string) error {
	if name == "" || len(name) > demoMaxField || strings.ContainsAny(name, "\r\n@") || name != strings.TrimSpace(name) {
		return errors.New("user name must be one short line with no @ and no space at either end (the app takes a name with an @ for an email address)")
	}
	return nil
}

func FormatDemoCredential(username, password string) (string, error) {
	if err := validDemoName(username); err != nil {
		return "", err
	}
	if len(password) < demoMinPassword || len(password) > demoMaxField {
		return "", fmt.Errorf("password must be %d to %d characters", demoMinPassword, demoMaxField)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	const m, t, p = 64 * 1024, 3, 2
	sum := argon2.IDKey([]byte(password), salt, t, m, p, 32)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("%s\n$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s\n", username, argon2.Version, m, t, p, enc.EncodeToString(salt), enc.EncodeToString(sum)), nil
}

func parseDemoCredential(data []byte) (string, *demoHash, error) {
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n") // the name keeps its spaces, so the shared rule can refuse them
	if len(lines) != 2 {
		return "", nil, errors.New("credential file: want a user name line and a hash line")
	}
	user := lines[0]
	if err := validDemoName(user); err != nil {
		return "", nil, fmt.Errorf("credential file: %w", err)
	}
	f := strings.Split(strings.TrimSpace(lines[1]), "$")
	// "", argon2id, v=19, m=..,t=..,p=.., salt, hash
	if len(f) != 6 || f[0] != "" || f[1] != "argon2id" || f[2] != "v="+strconv.Itoa(argon2.Version) {
		return "", nil, errors.New("credential file: not an argon2id hash")
	}
	h := &demoHash{}
	var m, t, p uint64
	for _, kv := range strings.Split(f[3], ",") {
		k, v, ok := strings.Cut(kv, "=")
		n, err := strconv.ParseUint(v, 10, 32)
		if !ok || err != nil {
			return "", nil, errors.New("credential file: bad parameters")
		}
		switch k {
		case "m":
			m = n
		case "t":
			t = n
		case "p":
			p = n
		}
	}
	// Bounds both ways: weak parameters are no protection and huge ones would
	// let a bad file exhaust the host on every attempt.
	if m < 16*1024 || m > 256*1024 || t < 1 || t > 10 || p < 1 || p > 8 {
		return "", nil, errors.New("credential file: parameters out of range")
	}
	h.m, h.t, h.p = uint32(m), uint32(t), uint8(p)
	var err error
	if h.salt, err = base64.RawStdEncoding.DecodeString(f[4]); err != nil || len(h.salt) < 16 {
		return "", nil, errors.New("credential file: bad salt")
	}
	if h.sum, err = base64.RawStdEncoding.DecodeString(f[5]); err != nil || len(h.sum) < 16 || len(h.sum) > 64 {
		return "", nil, errors.New("credential file: bad hash")
	}
	return user, h, nil
}

func verifyDemoPassword(h *demoHash, password string) bool {
	got := argon2.IDKey([]byte(password), h.salt, h.t, h.m, h.p, uint32(len(h.sum)))
	return subtle.ConstantTimeCompare(got, h.sum) == 1
}

// IsDemoAccount says whether userID is the reviewer account. It does not
// depend on the credential file: the account stays protected (not deletable,
// credentials not changeable) while sign-in is off.
func (h *Handler) IsDemoAccount(userID string) bool {
	id, _ := h.demoAccountID()
	return userID != "" && id == userID
}

// The host files of the reviewer account. One copy: the host command (cmd/demo-account) imports
// these, and a test holds them equal to what nix/node-b.nix gives the auth service.
const (
	DemoCredentialPath      = "/var/lib/go-choir/auth-signing/demo/demo-credential"
	DemoAccountIDPath       = "/var/lib/go-choir/auth-signing/demo/demo-account-id"
	StaleDemoCredentialPath = "/var/lib/go-choir/auth-signing/demo-credential" // before the move into demo/
)

// ErrAccountOff: the reviewer account is turned off, so nothing mints for it.
var ErrAccountOff = errors.New("this account is turned off")

// writeAccountOff is the refusal for a sign-in that reaches an account that is turned off: not a
// server error, and it says why.
func writeAccountOff(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}{ErrAccountOff.Error(), "account_off"})
}

// writeIssueError is the one answer to an issuer's error on an HTTP route: ErrAccountOff is the 403
// refusal, anything else a 500 with the route's own message. Every route that issues a session,
// a token or a key ends in it, so no route can answer an off account differently.
func writeIssueError(w http.ResponseWriter, err error, op, userID, msg string) {
	if refuseIfAccountOff(w, err, op, userID) {
		return
	}
	log.Printf("[auth] operation=%s user_id=%s result=error step=issue error=%q", op, userID, err)
	writeJSON(w, http.StatusInternalServerError, errorResponse{Error: msg})
}

// refuseIfAccountOff answers a sign-in step that failed because the reviewer account is turned
// off: the one refusal, 403 account_off, for every route that issues a session, a token or a
// key. It reports whether it answered. Every caller of issueAccessJWT, issueSession*,
// signInSession, rotateRefreshSession, rotateLocked and mintAppKey ends in writeIssueError.
func refuseIfAccountOff(w http.ResponseWriter, err error, op, userID string) bool {
	if !errors.Is(err, ErrAccountOff) {
		return false
	}
	log.Printf("[auth] operation=%s user_id=%s result=refused reason=account_off", op, userID)
	writeAccountOff(w)
	return true
}

// demoOff says whether userID is the reviewer account while it is off: the
// credential file is missing, unreadable or malformed. Off, no other way into the
// account works either (passkey, mail, recovery, device approval, a session made
// before): the host command's "off" must leave nothing usable. Read each time, so it
// takes effect with the file and needs no restart. A stateless access token made
// before lives at most AUTH_ACCESS_TOKEN_TTL (5 minutes) at the proxy, which does not
// ask this service.
func (h *Handler) demoOff(userID string) bool {
	id, err := h.demoAccountID()
	return userID != "" && id == userID && (err != nil || h.demoCredentialOff())
}

func (h *Handler) demoCredentialOff() bool {
	if h.config == nil || h.config.DemoCredentialFile == "" {
		return true
	}
	data, err := os.ReadFile(h.config.DemoCredentialFile)
	if err != nil {
		return true
	}
	_, _, err = parseDemoCredential(data)
	return err != nil
}

// ValidDemoCredential says whether data is a demo credential file (the host command checks
// the file it is about to remove).
func ValidDemoCredential(data []byte) bool {
	_, _, err := parseDemoCredential(data)
	return err == nil
}

// demoAccountID returns the current identity, or the last observed identity
// AND an error. The cached value keeps a known reviewer off during a file
// outage; it never authorizes deletion or password sign-in. An explicit
// host marker ("none") distinguishes no reviewer from a lost identity file.
func (h *Handler) demoAccountID() (string, error) {
	if h.config == nil {
		return "", nil
	}
	if h.config.DemoAccountID != "" {
		return h.config.DemoAccountID, nil
	}
	if h.config.DemoAccountIDFile == "" {
		return "", nil
	}
	h.demoIdentityMu.Lock()
	defer h.demoIdentityMu.Unlock()
	data, err := os.ReadFile(h.config.DemoAccountIDFile)
	if err != nil {
		return h.lastDemoAccountID, fmt.Errorf("reviewer identity unavailable: %w", err)
	}
	id := strings.TrimSpace(string(data))
	if id == "none" {
		id = ""
	} else if id == "" || strings.ContainsAny(id, " \t\r\n\x00") {
		return h.lastDemoAccountID, errors.New("reviewer identity file is invalid")
	}
	h.lastDemoAccountID = id
	return id, nil
}

func (h *Handler) loadDemo() (string, *demoHash, *User) {
	id, err := h.demoAccountID()
	if err != nil || id == "" || h.config.DemoCredentialFile == "" {
		return "", nil, nil
	}
	data, err := os.ReadFile(h.config.DemoCredentialFile)
	if err != nil {
		return "", nil, nil
	}
	user, hash, err := parseDemoCredential(data)
	if err != nil {
		log.Printf("[auth] operation=app_demo_login result=unavailable reason=credential_file_invalid")
		return "", nil, nil
	}
	u, err := h.store.GetUserByID(id)
	// An account whose address is not proved mints no key, so the route is "not available" and
	// not a server error. The plain rule: mayMint also exempts the operator's synthetic
	// bootstrap account, which must never be the reviewer's.
	if err != nil || u == nil || u.EmailVerifiedAt == nil || u.ID == BootstrapAdminAPIKeyUserID {
		return "", nil, nil
	}
	return user, hash, u
}

type demoLoginRequest struct {
	App      string `json:"app"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type demoKeyResponse struct {
	appKeyResponse
	Reviewer bool `json:"reviewer"` // the app shows the reviewer's delete page text from this
}

// HandleAppDemoLogin handles POST /auth/app/demo/login.
func (h *Handler) HandleAppDemoLogin(w http.ResponseWriter, r *http.Request) {
	// Every reply of this route may carry a 30-day bearer key: nothing on the way may keep it.
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	wantUser, hash, user := h.loadDemo()
	if user == nil {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not available"})
		return
	}
	ip := clientIP(r)
	addr := addrPrefix(ip)
	body, err := readBody(r)
	var req demoLoginRequest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err != nil || dec.Decode(&req) != nil || len(req.Username) > demoMaxField || len(req.Password) > demoMaxField || req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "app, username and password are required"})
		return
	}
	if !appids.Known(req.App) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unknown app"})
		return
	}
	g := h.demoGate
	ok, retry := g.allow(ip)
	if !ok {
		secs := writeRateLimited(w, retry)
		log.Printf("[auth] operation=app_demo_login result=rate_limited addr=%s retry_after=%d", addr, secs)
		return
	}
	select {
	case g.sem <- struct{}{}:
		defer func() { <-g.sem }()
	default:
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "busy; try again in a moment"})
		log.Printf("[auth] operation=app_demo_login result=busy addr=%s", addr)
		return
	}
	// Both halves always run, so a wrong user name costs what a wrong password
	// costs; the user names are compared as fixed-length digests.
	du, gu := sha256.Sum256([]byte(wantUser)), sha256.Sum256([]byte(req.Username))
	userOK := subtle.ConstantTimeCompare(du[:], gu[:]) == 1
	passOK := verifyDemoPassword(hash, req.Password)
	if !(userOK && passOK) {
		log.Printf("[auth] operation=app_demo_login result=denied addr=%s", addr)
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "the user name or password is not right"})
		return
	}
	label := "Autoradio reviewer: " + req.App
	exp := g.now().Add(DemoKeyTTL)
	if h.demoMintHook != nil {
		h.demoMintHook()
	}
	id, secret, err := h.store.CreateAPIKey(context.Background(), user.ID, label, DemoScopes, &exp)
	// `off` removes the credential and then proves the keys revoked. A login that passed the
	// check above before the removal could insert after that proof: look again, and take the
	// key back, so no key outlives the credential by more than this call.
	if err == nil && h.demoOff(user.ID) {
		_ = h.store.RevokeAPIKey(context.Background(), user.ID, id)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not available"})
		return
	}
	if err != nil {
		log.Printf("[auth] operation=app_demo_login result=error step=mint_key addr=%s error=%q", addr, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to create the key"})
		return
	}
	log.Printf("[auth] operation=app_demo_login result=success addr=%s key_id=%s app=%s", addr, id, req.App)
	writeJSON(w, http.StatusOK, demoKeyResponse{appKeyResponse{APIKey: secret, KeyID: id, Label: label, Scopes: DemoScopes}, true})
}

// ResetDemoAccess revokes every API key and deletes every refresh session of
// the account, for the host's reset command. It answers how many keys it
// revoked and how many sessions it found.
func ResetDemoAccess(ctx context.Context, s *Store, userID string) (keys, sessions int, err error) {
	list, err := s.ListAPIKeys(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	for _, k := range list {
		if k.RevokedAt != nil {
			continue
		}
		if err := s.RevokeAPIKey(ctx, userID, k.ID); err != nil {
			return keys, 0, err
		}
		keys++
	}
	rs, err := s.ListRefreshSessionsByUserID(userID)
	if err != nil {
		return keys, 0, err
	}
	sessions = len(rs)
	if err := s.DeleteRefreshSessionsByUserID(userID); err != nil {
		return keys, sessions, err
	}
	// The grants that would mint a key or a session later: an approved device
	// authorization, an unredeemed desktop code, a recovery link, an email sign-in
	// attempt, a passkey challenge, the account-set membership. (RevokeUnprovedAccess
	// deletes the same rows for the accounts it closes.)
	for _, q := range []string{
		"DELETE FROM device_authorizations WHERE user_id = ?",
		"DELETE FROM desktop_exchange_codes WHERE user_id = ?",
		"DELETE FROM recovery_tokens WHERE user_id = ?",
		"DELETE FROM email_signin_attempts WHERE user_id = ?",
		"DELETE FROM challenge_state WHERE user_id = ?",
		"DELETE FROM account_set_member WHERE user_id = ?",
	} {
		if _, err := s.db.Exec(q, userID); err != nil {
			return keys, sessions, err
		}
	}
	return keys, sessions, nil
}
