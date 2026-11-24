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
func FormatDemoCredential(username, password string) (string, error) {
	if username == "" || len(username) > demoMaxField || strings.ContainsAny(username, "\r\n") {
		return "", errors.New("user name must be one short line")
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
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		return "", nil, errors.New("credential file: want a user name line and a hash line")
	}
	user := strings.TrimSpace(lines[0])
	f := strings.Split(strings.TrimSpace(lines[1]), "$")
	// "", argon2id, v=19, m=..,t=..,p=.., salt, hash
	if user == "" || len(f) != 6 || f[0] != "" || f[1] != "argon2id" || f[2] != "v="+strconv.Itoa(argon2.Version) {
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
	return userID != "" && h.demoAccountID() == userID
}

// demoAccountID is the configured account: the host file's first line (read
// each time, so a change needs no restart), or the test-only field.
func (h *Handler) demoAccountID() string {
	if h.config == nil {
		return ""
	}
	if h.config.DemoAccountID != "" {
		return h.config.DemoAccountID
	}
	if h.config.DemoAccountIDFile == "" {
		return ""
	}
	data, err := os.ReadFile(h.config.DemoAccountIDFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(data), "\n", 2)[0])
}

func (h *Handler) loadDemo() (string, *demoHash, *User) {
	id := h.demoAccountID()
	if id == "" || h.config.DemoCredentialFile == "" {
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
	// Main's proof rule: an account whose address is not proved mints no key, so the route
	// is "not available" and not a server error.
	if err != nil || !mayMint(u) {
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
	id, secret, err := h.store.CreateAPIKey(context.Background(), user.ID, label, DemoScopes, &exp)
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
	return keys, sessions, s.DeleteRefreshSessionsByUserID(userID)
}
