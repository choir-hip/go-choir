package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Device authorization for `choir login` (docs/design/cli-agent-onboarding-
// 2026-11-03.md, red: auth and key minting). The CLI begins a request and
// holds a device_code; the owner approves the user_code from a passkey
// session; the CLI's next poll collects a full-access key, once.
//
//   POST /auth/device/begin    {label}               no auth
//   GET  /auth/device/lookup   ?user_code=           passkey session cookie
//   POST /auth/device/approve  {user_code, decision} passkey session cookie, JSON
//   POST /auth/device/token    {device_code}         no auth (the code is the secret)

const (
	// DeviceCodePrefix starts every device code.
	DeviceCodePrefix = "choir_dc_"
	// DeviceApprovePath is the frontend page that approves a request.
	DeviceApprovePath = "/device"
	// DeviceMaxLookupMissesPerUser limits user-code guessing by a signed-in
	// user (hourly), across lookup and approve.
	DeviceMaxLookupMissesPerUser = 10
	// DeviceTokenRateLimit is the per-IP per-minute limit for token polls
	// (cmd/auth wraps the route with its own limiter: the global one, 10 a
	// minute, would refuse a 5-second poll).
	DeviceTokenRateLimit = 60

	deviceLabelMax     = 64
	deviceDefaultLabel = "choir CLI"
	// 20 consonants with no look-alikes (RFC 8628 6.1); 8 of them is ~34 bits.
	deviceUserCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"
	deviceUserCodeLen      = 8
)

type deviceState struct {
	mu     sync.Mutex
	misses map[string][]time.Time
}

func (h *Handler) deviceMissesLocked(userID string, now time.Time) []time.Time {
	if h.device.misses == nil {
		h.device.misses = make(map[string][]time.Time)
	}
	cutoff := now.Add(-time.Hour)
	kept := h.device.misses[userID][:0:0]
	for _, t := range h.device.misses[userID] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	h.device.misses[userID] = kept
	return kept
}

// deviceGuessesExhausted reports whether userID used up its misses.
func (h *Handler) deviceGuessesExhausted(userID string) bool {
	h.device.mu.Lock()
	defer h.device.mu.Unlock()
	return len(h.deviceMissesLocked(userID, time.Now())) >= DeviceMaxLookupMissesPerUser
}

func (h *Handler) recordDeviceMiss(userID string) {
	h.device.mu.Lock()
	defer h.device.mu.Unlock()
	now := time.Now()
	h.device.misses[userID] = append(h.deviceMissesLocked(userID, now), now)
	if len(h.device.misses) > 4096 {
		for k, v := range h.device.misses {
			if len(v) == 0 || !v[len(v)-1].After(now.Add(-time.Hour)) {
				delete(h.device.misses, k)
			}
		}
	}
}

// normalizeUserCode upper-cases a typed code and drops dashes and spaces.
func normalizeUserCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func validUserCode(norm string) bool {
	if len(norm) != deviceUserCodeLen {
		return false
	}
	for _, r := range norm {
		if !strings.ContainsRune(deviceUserCodeAlphabet, r) {
			return false
		}
	}
	return true
}

func userCodeHash(norm string) string { return sha256SumHex("device-user-code:" + norm) }

func newUserCode() (display, norm string, err error) {
	max := big.NewInt(int64(len(deviceUserCodeAlphabet)))
	var b strings.Builder
	for i := 0; i < deviceUserCodeLen; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", "", err
		}
		b.WriteByte(deviceUserCodeAlphabet[n.Int64()])
	}
	norm = b.String()
	return norm[:4] + "-" + norm[4:], norm, nil
}

func newDeviceCode() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return DeviceCodePrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func newDeviceID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "da_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

// sanitizeDeviceLabel drops control characters and bounds the length; the
// label is shown to the owner on the approve page.
func sanitizeDeviceLabel(label string) string {
	var b strings.Builder
	for _, r := range label {
		if unicode.IsControl(r) || r == utf8.RuneError {
			continue
		}
		if b.Len()+utf8.RuneLen(r) > deviceLabelMax {
			break
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return deviceDefaultLabel
	}
	return out
}

func (h *Handler) deviceApproveURI() string {
	origin := ""
	if h.config != nil && len(h.config.RPOrigins) > 0 {
		origin = strings.TrimRight(strings.TrimSpace(h.config.RPOrigins[0]), "/")
	}
	return origin + DeviceApprovePath
}

type deviceBeginRequest struct {
	Label string `json:"label"`
}

type deviceBeginResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// deviceErrorResponse is the RFC 8628 error shape the CLI reads.
type deviceErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

// HandleDeviceBegin handles POST /auth/device/begin.
func (h *Handler) HandleDeviceBegin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req deviceBeginRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	deviceCode, err := newDeviceCode()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start login"})
		return
	}
	id, err := newDeviceID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start login"})
		return
	}
	now := time.Now().UTC()
	ip := clientIP(r)
	a := &DeviceAuthorization{
		ID:               id,
		DeviceCodeHash:   sha256SumHex(deviceCode),
		Label:            sanitizeDeviceLabel(req.Label),
		UserAgentSummary: summarizeDeviceAgent(r.UserAgent()),
		PlaceSummary:     summarizePlace(ip),
		IPHash:           hashIP(ip),
		CreatedAt:        now,
		ExpiresAt:        now.Add(DeviceAuthorizationTTL),
	}
	// A user code that collides with a pending row (expired or not; the
	// partial unique index covers both) is drawn again.
	var display string
	for i := 0; i < 5; i++ {
		var norm string
		display, norm, err = newUserCode()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start login"})
			return
		}
		a.UserCodeHash = userCodeHash(norm)
		existing, lerr := h.store.GetDeviceAuthorizationByUserCode(r.Context(), a.UserCodeHash)
		if errors.Is(lerr, sql.ErrNoRows) || (lerr == nil && existing.Status != DeviceStatusPending) {
			break
		}
	}
	if err := h.store.CreateDeviceAuthorization(r.Context(), a); err != nil {
		if errors.Is(err, ErrDeviceRateLimited) {
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many login requests from this address; try again later"})
			return
		}
		log.Printf("[auth] operation=device_begin result=error error=%q", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start login"})
		return
	}
	log.Printf("[auth] operation=device_begin id=%s result=success", a.ID)
	uri := h.deviceApproveURI()
	writeJSON(w, http.StatusOK, deviceBeginResponse{
		DeviceCode:              deviceCode,
		UserCode:                display,
		VerificationURI:         uri,
		VerificationURIComplete: uri + "?code=" + display,
		ExpiresIn:               int(DeviceAuthorizationTTL / time.Second),
		Interval:                DevicePollIntervalSeconds,
	})
}

// summarizeDeviceAgent names the requesting tool: the choir CLI by its
// User-Agent, otherwise the browser summary.
func summarizeDeviceAgent(ua string) string {
	if strings.HasPrefix(strings.ToLower(ua), "choir-cli") {
		s := summarizeUserAgent(ua)
		if i := strings.Index(s, " on "); i >= 0 {
			return "the choir CLI" + s[i:]
		}
		return "the choir CLI"
	}
	return summarizeUserAgent(ua)
}

type deviceLookupResponse struct {
	Label         string `json:"label"`
	RequestedFrom string `json:"requested_from"`
	RequestedAt   string `json:"requested_at"`
	ExpiresAt     string `json:"expires_at"`
	Status        string `json:"status"`
	Grants        string `json:"grants"`
}

const deviceGrantText = "a full-access API key: every scope, owner acts included, on every computer you own, until you revoke it"

// lookupForUser resolves a typed code for a signed-in user, counting misses.
// It writes the error response and returns nil on failure.
func (h *Handler) lookupForUser(w http.ResponseWriter, r *http.Request, user *User, code string) (*DeviceAuthorization, string) {
	if h.deviceGuessesExhausted(user.ID) {
		writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "too many unknown codes; try again in an hour"})
		return nil, ""
	}
	norm := normalizeUserCode(code)
	if !validUserCode(norm) {
		h.recordDeviceMiss(user.ID)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: ErrDeviceNotFound.Error()})
		return nil, ""
	}
	hash := userCodeHash(norm)
	a, err := h.store.GetDeviceAuthorizationByUserCode(r.Context(), hash)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Printf("[auth] operation=device_lookup result=error error=%q", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to look up the code"})
			return nil, ""
		}
		h.recordDeviceMiss(user.ID)
		writeJSON(w, http.StatusNotFound, errorResponse{Error: ErrDeviceNotFound.Error()})
		return nil, ""
	}
	return a, hash
}

// HandleDeviceLookup handles GET /auth/device/lookup?user_code=. The approve
// page shows the owner which device is asking before they decide.
func (h *Handler) HandleDeviceLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	user := h.requirePasskeySession(w, r)
	if user == nil {
		return
	}
	a, _ := h.lookupForUser(w, r, user, r.URL.Query().Get("user_code"))
	if a == nil {
		return
	}
	writeJSON(w, http.StatusOK, deviceLookupResponse{
		Label:         a.Label,
		RequestedFrom: a.UserAgentSummary + ", " + a.PlaceSummary,
		RequestedAt:   a.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt:     a.ExpiresAt.UTC().Format(time.RFC3339),
		Status:        a.EffectiveStatus(time.Now().UTC()),
		Grants:        deviceGrantText,
	})
}

type deviceApproveRequest struct {
	UserCode string `json:"user_code"`
	Decision string `json:"decision"`
}

// HandleDeviceApprove handles POST /auth/device/approve. Only the owner's
// passkey session may decide: an API key cannot (approval mints a key with
// owner:acts, which no key may mint) and an email session cannot (it may not
// mint keys, email sign-in T5). JSON only, so a cross-site form cannot post it.
func (h *Handler) HandleDeviceApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if !requireJSONContentType(r) {
		writeJSON(w, http.StatusUnsupportedMediaType, errorResponse{Error: "Content-Type must be application/json"})
		return
	}
	user := h.requirePasskeySession(w, r)
	if user == nil {
		return
	}
	var req deviceApproveRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if req.Decision != "approve" && req.Decision != "deny" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: `decision must be "approve" or "deny"`})
		return
	}
	a, _ := h.lookupForUser(w, r, user, req.UserCode)
	if a == nil {
		return
	}
	if err := h.store.DecideDeviceAuthorization(r.Context(), a.ID, user.ID, req.Decision == "approve", time.Now().UTC()); err != nil {
		if errors.Is(err, ErrDeviceNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: ErrDeviceNotFound.Error()})
			return
		}
		log.Printf("[auth] operation=device_decide user_id=%s result=error error=%q", user.ID, err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to record the decision"})
		return
	}
	log.Printf("[auth] operation=device_decide user_id=%s id=%s decision=%s result=success", user.ID, a.ID, req.Decision)
	writeJSON(w, http.StatusOK, map[string]string{"status": map[bool]string{true: DeviceStatusApproved, false: DeviceStatusDenied}[req.Decision == "approve"], "label": a.Label})
}

type deviceTokenRequest struct {
	DeviceCode string `json:"device_code"`
}

type deviceTokenResponse struct {
	APIKey string   `json:"api_key"`
	KeyID  string   `json:"key_id"`
	Label  string   `json:"label"`
	Scopes []string `json:"scopes"`
}

// HandleDeviceToken handles POST /auth/device/token: the CLI's poll.
func (h *Handler) HandleDeviceToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req deviceTokenRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "invalid_request", Description: err.Error()})
		return
	}
	code := strings.TrimSpace(req.DeviceCode)
	if !strings.HasPrefix(code, DeviceCodePrefix) {
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "invalid_grant", Description: "unknown or used device code"})
		return
	}
	res, err := h.store.PollDeviceAuthorization(r.Context(), sha256SumHex(code), AllScopes(),
		time.Duration(DevicePollIntervalSeconds)*time.Second, time.Now().UTC())
	if err != nil {
		log.Printf("[auth] operation=device_token result=error error=%q", err)
		writeJSON(w, http.StatusInternalServerError, deviceErrorResponse{Error: "server_error"})
		return
	}
	switch res.Status {
	case DeviceStatusApproved:
		log.Printf("[auth] operation=device_token key_id=%s result=success", res.KeyID)
		writeJSON(w, http.StatusOK, deviceTokenResponse{APIKey: res.Secret, KeyID: res.KeyID, Label: res.Label, Scopes: res.Scopes})
	case DeviceStatusPending:
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "authorization_pending", Description: "waiting for the owner to approve"})
	case "slow_down":
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "slow_down", Description: "poll no faster than the interval"})
	case DeviceStatusDenied:
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "access_denied", Description: "the owner denied this login"})
	case DeviceStatusExpired:
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "expired_token", Description: "the login request expired; run choir login again"})
	default: // unknown or consumed
		writeJSON(w, http.StatusBadRequest, deviceErrorResponse{Error: "invalid_grant", Description: "unknown or used device code"})
	}
}

// sweepChallenges deletes expired WebAuthn challenge rows, which nothing else
// removes (every begin writes one).
func (h *Handler) sweepChallenges() {
	if n, err := h.store.CleanExpiredChallenges(); err != nil {
		log.Printf("[auth] operation=challenge_sweep result=error error=%q", err)
	} else if n > 0 {
		log.Printf("[auth] operation=challenge_sweep deleted=%d", n)
	}
}

// StartDeviceSweeper deletes device authorizations older than 24 hours, expired
// challenges and stale account sets every interval until ctx ends.
func (h *Handler) StartDeviceSweeper(ctx context.Context, interval time.Duration) {
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
				h.sweepAccountSets()
				h.sweepChallenges()
				if n, err := h.store.SweepDeviceAuthorizations(ctx, time.Now().UTC().Add(-DeviceAuthorizationRetention)); err != nil {
					log.Printf("[auth] operation=device_sweep result=error error=%q", err)
				} else if n > 0 {
					log.Printf("[auth] operation=device_sweep deleted=%d", n)
				}
			}
		}
	}()
}
