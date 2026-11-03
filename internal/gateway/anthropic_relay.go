package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// SEC-08, gateway half: a relay for the Claude delegate's Messages API calls
// that holds the delegate's credential on the host, so no computer carries it
// (docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md;
// docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md, design
// section; FP7 built the key half in
// docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md).
//
// The credential is one of two kinds, chosen once at start: the owner's
// Claude OAuth token (owner, 2026-11-03: "there shouldn't be credentials on
// computers anyway"), copied by the unit's start stage from the host secrets
// dir, or a Console API key in gateway-provider.env. SEC-08 records that
// Anthropic's terms forbid a service that intermediates a subscription
// token; this relay serves only the owner's own computers, and the Console
// key replaces the token with no code change (the residual is the owner's).
// It is a narrow relay for the identities and window the gateway is told,
// and nothing else. It is dark until all of that exists: no credential, no
// identities or no open window each refuse every call, and nothing falls
// back to another credential.

const (
	// AnthropicAPIKeyEnv is the Console API key, in gateway-provider.env.
	AnthropicAPIKeyEnv = "ANTHROPIC_API_KEY"
	// AnthropicRelayIdentitiesEnv lists the product computer ids
	// (computer-…) the relay serves. A bearer authenticates as its VM id;
	// the relay reads the product computer id vmctl recorded on the
	// identity at issuance (AutoputerIdentity.ProductComputerID), never the
	// VM id, so a VM id on this list grants nothing.
	AnthropicRelayIdentitiesEnv = "GATEWAY_ANTHROPIC_RELAY_IDENTITIES"
	// AnthropicRelayExpiresEnv is the end of the window (RFC 3339 or
	// YYYY-MM-DD, UTC). Missing, unreadable or past means the relay is dark.
	AnthropicRelayExpiresEnv = "GATEWAY_ANTHROPIC_RELAY_EXPIRES"
	// AnthropicRelayModelsEnv lists the models the relay serves.
	AnthropicRelayModelsEnv = "GATEWAY_ANTHROPIC_RELAY_MODELS"
	// AnthropicRelayMaxInflightEnv is the per-identity concurrent-call cap.
	AnthropicRelayMaxInflightEnv = "GATEWAY_ANTHROPIC_RELAY_MAX_INFLIGHT"
	// AnthropicOAuthTokenFileEnv names the host copy of the owner's Claude
	// OAuth token. When it is set, the token is the relay's credential, and a
	// file that does not hold a token leaves the relay off (never the key).
	AnthropicOAuthTokenFileEnv = "GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE"
	// AnthropicRelayDailyTokensEnv is each computer's spend cap: input,
	// output and cache-creation tokens per UTC day (cached reads are not
	// charged).
	AnthropicRelayDailyTokensEnv = "GATEWAY_ANTHROPIC_RELAY_DAILY_TOKENS"
	// AnthropicRelaySpendFileEnv is where the day's charges are kept, so a
	// gateway restart does not reset a computer's cap. Unset keeps them in
	// memory only.
	AnthropicRelaySpendFileEnv = "GATEWAY_ANTHROPIC_RELAY_SPEND_FILE"

	anthropicAPIBase          = "https://api.anthropic.com"
	anthropicRelayPrefix      = "/provider/anthropic"
	anthropicDefaultVersion   = "2023-06-01"
	anthropicDefaultModel     = "claude-sonnet-5-5"
	anthropicRelayMaxBody     = 32 << 20 // the Messages API's own request limit
	anthropicRelayMaxInflight = 2
	anthropicRelayTimeout     = inferenceTimeout
	anthropicErrBodyMax       = 64 << 10
	anthropicUsageJSONMax     = 4 << 20
	anthropicUsageLineMax     = 1 << 20
	anthropicRelayDailyTokens = 50_000_000
	anthropicOAuthBeta        = "oauth-2025-04-20"
	maxAnthropicTokenBytes    = 8 << 10
	relayUserAgent            = "choir-gateway-anthropic-relay"
)

// The relay's credential kinds. A relay with a key and no kind (FP7's
// tests) is the API-key kind.
const (
	relayCredentialAPIKey = "api_key"
	relayCredentialOAuth  = "oauth"
)

// AnthropicRelay is the relay's configuration and state.
type AnthropicRelay struct {
	// key is the credential's value; kind says how it is sent.
	key  string
	kind string
	// credentialReason says why a named token file gave no credential.
	credentialReason string
	identities       map[string]bool
	expires          time.Time
	models           map[string]bool
	baseURL          string
	client           *http.Client
	maxBody          int64
	maxInflight      int
	dailyTokens      int64
	now              func() time.Time

	mu       sync.Mutex
	inflight map[string]int
	usage    relayUsage
	// spent is each identity's charged tokens on the UTC day spentDay;
	// spendFile, when set, holds them across restarts.
	spent     map[string]int64
	spentDay  string
	spendFile string
}

// relaySpendRecord is the spend file.
type relaySpendRecord struct {
	Day   string           `json:"day"`
	Spent map[string]int64 `json:"spent"`
}

// relayUsage is the spend record since the gateway started. The per-computer
// cap is the day's charge (spent), not this record.
type relayUsage struct {
	Requests            int64
	InputTokens         int64
	OutputTokens        int64
	CachedInputTokens   int64
	CacheCreationTokens int64
}

// NewAnthropicRelayFromEnv reads the relay's configuration.
func NewAnthropicRelayFromEnv() *AnthropicRelay {
	return newAnthropicRelayFromGetenv(os.Getenv)
}

func newAnthropicRelayFromGetenv(get func(string) string) *AnthropicRelay {
	r := &AnthropicRelay{
		identities:  map[string]bool{},
		models:      map[string]bool{},
		baseURL:     anthropicAPIBase, // never from the environment
		maxBody:     anthropicRelayMaxBody,
		maxInflight: anthropicRelayMaxInflight,
		dailyTokens: anthropicRelayDailyTokens,
		now:         time.Now,
		client: &http.Client{Transport: &http.Transport{
			Proxy:                 nil,
			ResponseHeaderTimeout: 5 * time.Minute,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		}},
	}
	split := func(raw string) []string {
		return strings.FieldsFunc(raw, func(c rune) bool { return c == ',' || c == ' ' || c == '\n' || c == '\t' || c == '\r' })
	}
	for _, id := range split(get(AnthropicRelayIdentitiesEnv)) {
		r.identities[id] = true
	}
	models := split(get(AnthropicRelayModelsEnv))
	if len(models) == 0 {
		models = []string{anthropicDefaultModel}
	}
	for _, m := range models {
		r.models[m] = true
	}
	if raw := strings.TrimSpace(get(AnthropicRelayExpiresEnv)); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			r.expires = t.UTC()
		} else if t, err := time.Parse("2006-01-02", raw); err == nil {
			r.expires = t.UTC()
		}
	}
	if n, err := strconv.Atoi(strings.TrimSpace(get(AnthropicRelayMaxInflightEnv))); err == nil && n > 0 {
		r.maxInflight = n
	}
	// An unset, unreadable or non-positive cap is the default cap, never
	// none.
	if n, err := strconv.ParseInt(strings.TrimSpace(get(AnthropicRelayDailyTokensEnv)), 10, 64); err == nil && n > 0 {
		r.dailyTokens = n
	}
	if path := strings.TrimSpace(get(AnthropicRelaySpendFileEnv)); path != "" {
		r.spendFile = path
		r.loadSpend()
	}
	// One credential, chosen here. A named token file is the choice even
	// when it is unusable: the relay is then off, and never falls back to
	// the key.
	if path := strings.TrimSpace(get(AnthropicOAuthTokenFileEnv)); path != "" {
		if token, reason := readAnthropicOAuthToken(path); token != "" {
			r.key, r.kind = token, relayCredentialOAuth
		} else {
			r.credentialReason = reason
		}
	} else if key := strings.TrimSpace(get(AnthropicAPIKeyEnv)); key != "" {
		r.key, r.kind = key, relayCredentialAPIKey
	}
	return r
}

// readAnthropicOAuthToken reads the host copy of the owner's token. The
// reason never quotes the file.
func readAnthropicOAuthToken(path string) (string, string) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return "", "the token file is absent or unreadable"
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return "", "the token path is not a regular file"
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", "the token file is readable by group or others; it must be 0400"
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return "", "the token file is not owned by the gateway's user"
	}
	data, err := io.ReadAll(io.LimitReader(f, maxAnthropicTokenBytes+1))
	if err != nil || len(data) > maxAnthropicTokenBytes {
		return "", "the token file is unreadable or too large"
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 20 {
		return "", "the token file does not hold a token"
	}
	for i := 0; i < len(token); i++ {
		if c := token[i]; c <= ' ' || c > '~' {
			return "", "the token file does not hold a token"
		}
	}
	return token, ""
}

// credentialKind is the kind the relay sends, or "" without a credential.
func (r *AnthropicRelay) credentialKind() string {
	switch {
	case r == nil || r.key == "":
		return ""
	case r.kind == "":
		return relayCredentialAPIKey
	}
	return r.kind
}

// relayDay is the UTC day a charge belongs to.
func (r *AnthropicRelay) relayDay() string { return r.now().UTC().Format("2006-01-02") }

// spentToday is identity's charge on the current UTC day.
func (r *AnthropicRelay) spentToday(identity string) int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.spentDay != r.relayDay() {
		return 0
	}
	return r.spent[identity]
}

// charge adds a call's tokens to identity's day, and keeps the day in the
// spend file when one is set.
func (r *AnthropicRelay) charge(identity string, u relayUsage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if day := r.relayDay(); r.spentDay != day || r.spent == nil {
		r.spent, r.spentDay = map[string]int64{}, day
	}
	r.spent[identity] += u.InputTokens + u.OutputTokens + u.CacheCreationTokens
	if r.spendFile != "" {
		if err := r.saveSpendLocked(); err != nil {
			log.Printf("gateway: anthropic relay: could not keep the day's spend: %v", err)
		}
	}
}

// loadSpend reads the spend file; another day's record, or none, starts the
// day at zero.
func (r *AnthropicRelay) loadSpend() {
	raw, err := os.ReadFile(r.spendFile)
	if err != nil {
		return
	}
	var rec relaySpendRecord
	if json.Unmarshal(raw, &rec) != nil || rec.Day != r.relayDay() {
		return
	}
	r.spent, r.spentDay = map[string]int64{}, rec.Day
	for id, n := range rec.Spent {
		if n > 0 {
			r.spent[id] = n
		}
	}
}

func (r *AnthropicRelay) saveSpendLocked() error {
	raw, err := json.Marshal(relaySpendRecord{Day: r.spentDay, Spent: r.spent})
	if err != nil {
		return err
	}
	tmp := r.spendFile + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, r.spendFile)
}

// budgetSpent reports whether identity has used its day's cap.
func (r *AnthropicRelay) budgetSpent(identity string) bool {
	return r.dailyTokens > 0 && r.spentToday(identity) >= r.dailyTokens
}

func (r *AnthropicRelay) windowOpen() bool {
	return r != nil && !r.expires.IsZero() && r.now().Before(r.expires)
}

// status is "off" without a key, "dark" with a key but no identities or no
// open window, "on" otherwise. It names neither.
func (r *AnthropicRelay) status() string {
	switch {
	case r == nil || r.key == "":
		return "off"
	case len(r.identities) == 0 || !r.windowOpen():
		return "dark"
	}
	return "on"
}

func (r *AnthropicRelay) usageSnapshot() relayUsage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.usage
}

// grantState is whether the relay serves identity now, and why not. It is
// the same order of checks a relay call makes, without the pin.
//
// identity is the product computer id the caller's bearer was issued for
// (relayCaller), "" when the bearer predates it.
func (r *AnthropicRelay) grantState(identity string) (bool, string) {
	switch {
	case r == nil:
		return false, "the gateway has no Claude delegate relay"
	case identity == "":
		return false, relayLegacyReason
	case !r.identities[identity]:
		return false, "this computer has no Claude delegate grant"
	case !r.windowOpen():
		return false, "the Claude delegate window is closed"
	case r.key == "":
		return false, "the gateway holds no Claude credential"
	case r.budgetSpent(identity):
		return false, "this computer spent its Claude delegate budget for the day"
	}
	return true, ""
}

func (r *AnthropicRelay) acquire(identity string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight == nil {
		r.inflight = map[string]int{}
	}
	if r.inflight[identity] >= r.maxInflight {
		return false
	}
	r.inflight[identity]++
	return true
}

func (r *AnthropicRelay) release(identity string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight[identity] > 0 {
		r.inflight[identity]--
	}
	if r.inflight[identity] == 0 {
		delete(r.inflight, identity)
	}
}

// SetAnthropicRelay attaches the relay. A handler without one answers every
// relay call with 503.
func (h *Handler) SetAnthropicRelay(r *AnthropicRelay) {
	h.anthropicRelay = r
}

type relayRoute struct {
	method string
	path   string // upstream path
	body   bool
}

// relayRoutes is everything the relay serves: what `claude -p` needs and
// nothing else. No batches, files, organizations or legacy completions.
var relayRoutes = map[string]relayRoute{
	"/v1/messages":              {http.MethodPost, "/v1/messages", true},
	"/v1/messages/count_tokens": {http.MethodPost, "/v1/messages/count_tokens", true},
	"/v1/models":                {http.MethodGet, "/v1/models", false},
}

// relayAuthFailure answers a failed authentication or pin check in the
// Messages API's error envelope, with the gateway's status and retry hint.
func relayAuthFailure(w http.ResponseWriter, err error) {
	status, retry, message := authFailure(err)
	if retry != "" {
		w.Header().Set("Retry-After", retry)
	}
	errType := "authentication_error"
	if status == http.StatusTooManyRequests {
		errType = "rate_limit_error"
	} else if status != http.StatusUnauthorized {
		errType = "api_error"
	}
	relayError(w, status, errType, message)
}

// relayLegacyReason is the grant read's and the relay call's answer to a
// bearer issued before vmctl named the product computer id.
const relayLegacyReason = "this computer's gateway credential predates the computer id; cold boot to re-issue it"

// relayCaller authenticates a relay or grant-read caller. vmID is the id the
// bearer authenticates as, which the pin (relayPeerCheck) and the rate
// limiter's other routes key by; computerID is the product computer id
// recorded on that same identity, which the grant list, the budget and the
// in-flight bound key by. Both come from one validation
// (ValidateCredentialIdentity), so a relay call and a grant read always
// judge the same computer. computerID is "" for a bearer issued before it
// was recorded (relayLegacyReason).
func (h *Handler) relayCaller(r *http.Request) (vmID, computerID string, err error) {
	return h.authenticateBearerIdentity(r)
}

// relayPeerCheck is the relay's own SEC-05 check, independent of
// GATEWAY_BEARER_BINDING (problem record, finding 11): the relay spends a key
// the gateway holds, so it serves only a bearer whose identity is already
// pinned (by the host's ensure, or an earlier first use on another route) to
// the caller's address. It reads the pin and never writes it; an unpinned
// identity waits for the host's ensure.
func (h *Handler) relayPeerCheck(computerID string, r *http.Request) error {
	ip := remoteIP(r)
	if ip == nil {
		return errPeerUnbindable
	}
	peer := ip.String()
	state, pin, err := h.registry.CheckPeer(computerID, peer, false)
	switch {
	case err != nil:
		h.bindingLogf("relay-could-not-check/"+computerID, "gateway: anthropic relay refused %s from %s: %v", computerID, peer, err)
		return errPeerUnbindable
	case state == PeerMatch:
		return nil
	case state == PeerUnpinned:
		h.bindingLogf("relay-unpinned/"+computerID, "gateway: anthropic relay refused %s from %s: no pin", computerID, peer)
		return errPeerUnpinned
	}
	h.bindingLogf("relay-mismatch/"+computerID+"/"+peer, "gateway: anthropic relay refused %s from %s (pinned to %s)", computerID, peer, pin)
	return errPeerPinMismatch
}

// relayAddressNotAttested is the grant read's error type for a bearer whose
// pin does not name the caller's address (or names none yet). A rebooted
// computer takes a new guest address, and vmctl moves the pin to it
// (credentials/ensure with peer_ip) only after the boot returns, so a guest
// that resumes work at boot can read its grant first
// (docs/problems/claude-delegate-grant-read-races-host-address-attestation-2026-11-03.md).
const relayAddressNotAttested = "address_not_attested"

// relayGrantPeerFailure answers a grant read whose pin check failed. An
// address the host has not attested gets its own status (409) and type, so
// the guest can tell it from a rate limit or a missing grant and wait it out;
// a copied bearer gets the same answer, which names no address. Any other
// failure is relayAuthFailure's answer.
func relayGrantPeerFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, errPeerPinMismatch) || errors.Is(err, errPeerUnpinned) {
		w.Header().Set("Retry-After", bindingRetryAfter)
		relayError(w, http.StatusConflict, relayAddressNotAttested,
			"the host has not attested this computer's current address to the gateway yet (a restarted computer gets a new address, attested within seconds); retry shortly")
		return
	}
	relayAuthFailure(w, err)
}

func relayError(w http.ResponseWriter, status int, errType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"type": errType, "message": message},
	})
}

// HandleAnthropicRelay serves /provider/anthropic/v1/{messages,
// messages/count_tokens,models}. The order: path and method, the computer's
// bearer (validated, nothing written), the allowlist and window, the key, the
// bearer's pin (which must already name the caller's address, in every
// binding mode), the rate and in-flight bounds, the body and its model, and
// only then the upstream call.
func (h *Handler) HandleAnthropicRelay(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if r.URL.Path == anthropicRelayPrefix+"/grant" {
		h.handleAnthropicRelayGrant(w, r)
		return
	}
	route, ok := relayRoutes[strings.TrimPrefix(r.URL.Path, anthropicRelayPrefix)]
	if !strings.HasPrefix(r.URL.Path, anthropicRelayPrefix+"/") || !ok {
		relayError(w, http.StatusNotFound, "not_found_error", "not found")
		return
	}
	if r.Method != route.method {
		w.Header().Set("Allow", route.method)
		relayError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}
	// The bearer is validated without touching its pin, and the relay's own
	// configuration is checked before the pin is read: a relay call never
	// writes identity state, whatever it answers (problem record, finding 12).
	vmID, computerID, err := h.relayCaller(r)
	if err != nil {
		log.Printf("gateway: anthropic relay authentication denied: %v", err)
		relayAuthFailure(w, err)
		return
	}
	relay := h.anthropicRelay
	if relay == nil {
		relayError(w, http.StatusServiceUnavailable, "api_error", "anthropic relay is not configured")
		return
	}
	if computerID == "" {
		log.Printf("gateway: anthropic relay refused VM %s: its credential predates the computer id (cold boot re-issues it)", vmID)
		relayError(w, http.StatusForbidden, "permission_error", relayLegacyReason)
		return
	}
	if !relay.identities[computerID] || !relay.windowOpen() {
		log.Printf("gateway: anthropic relay refused %s (VM %s): not enabled for this computer or the window is closed", computerID, vmID)
		relayError(w, http.StatusForbidden, "permission_error", "anthropic relay is not enabled for this computer")
		return
	}
	if relay.key == "" {
		log.Printf("gateway: anthropic relay refused %s (VM %s): no credential is configured", computerID, vmID)
		relayError(w, http.StatusServiceUnavailable, "api_error", "anthropic relay is not configured")
		return
	}
	if err := h.relayPeerCheck(vmID, r); err != nil {
		relayAuthFailure(w, err)
		return
	}
	if h.rateLimiter != nil {
		bucket := rateLimitBucketKey(computerID, "anthropic_relay")
		if !h.rateLimiter.Record(bucket) {
			_, _, resetIn := h.rateLimiter.Status(bucket)
			w.Header().Set("Retry-After", strconv.Itoa(max(1, int(resetIn.Seconds()))))
			relayError(w, http.StatusTooManyRequests, "rate_limit_error", "rate limit exceeded")
			return
		}
	}
	if relay.budgetSpent(computerID) {
		log.Printf("gateway: anthropic relay refused %s: the day's token budget is spent", computerID)
		w.Header().Set("Retry-After", "3600")
		relayError(w, http.StatusTooManyRequests, "rate_limit_error", "this computer spent its Claude delegate budget for the day")
		return
	}
	if !relay.acquire(computerID) {
		w.Header().Set("Retry-After", "5")
		relayError(w, http.StatusTooManyRequests, "rate_limit_error", "too many concurrent relay calls")
		return
	}
	defer relay.release(computerID)

	var body []byte
	model := ""
	if route.body {
		if mt := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0])); mt != "application/json" {
			relayError(w, http.StatusUnsupportedMediaType, "invalid_request_error", "content-type must be application/json")
			return
		}
		var err error
		body, err = io.ReadAll(http.MaxBytesReader(w, r.Body, relay.maxBody))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				relayError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body is too large")
				return
			}
			relayError(w, http.StatusBadRequest, "invalid_request_error", "could not read the request body")
			return
		}
		var modelErr string
		model, modelErr = requestModel(body)
		if modelErr != "" {
			relayError(w, http.StatusBadRequest, "invalid_request_error", modelErr)
			return
		}
		if !relay.models[model] {
			log.Printf("gateway: anthropic relay refused %s: model not served by the relay", computerID)
			relayError(w, http.StatusForbidden, "permission_error", "model is not served by this relay")
			return
		}
	}

	query := ""
	if r.URL.Query().Get("beta") == "true" {
		query = "?beta=true"
	}
	ctx, cancel := context.WithTimeout(r.Context(), anthropicRelayTimeout)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	up, err := http.NewRequestWithContext(ctx, route.method, strings.TrimRight(relay.baseURL, "/")+route.path+query, reader)
	if err != nil {
		relayError(w, http.StatusBadGateway, "api_error", "relay could not build the upstream request")
		return
	}
	// Only these cross, and the key is the gateway's own. Nothing the caller
	// sent that could authenticate, identify or route does.
	beta := ""
	if v := strings.TrimSpace(r.Header.Get("Anthropic-Beta")); v != "" && len(v) <= 1024 && relayTokenText.MatchString(v) {
		beta = v
	}
	if relay.credentialKind() == relayCredentialOAuth {
		up.Header.Set("Authorization", "Bearer "+relay.key)
		if !strings.Contains(beta, anthropicOAuthBeta) {
			beta = strings.TrimLeft(beta+","+anthropicOAuthBeta, ",")
		}
	} else {
		up.Header.Set("X-Api-Key", relay.key)
	}
	up.Header.Set("Anthropic-Version", anthropicDefaultVersion)
	if v := strings.TrimSpace(r.Header.Get("Anthropic-Version")); v != "" && len(v) <= 32 && relayTokenText.MatchString(v) {
		up.Header.Set("Anthropic-Version", v)
	}
	if beta != "" {
		up.Header.Set("Anthropic-Beta", beta)
	}
	if route.body {
		up.Header.Set("Content-Type", "application/json")
	}
	up.Header.Set("Accept", "application/json")
	if a := r.Header.Get("Accept"); strings.Contains(a, "text/event-stream") {
		up.Header.Set("Accept", "text/event-stream")
	}
	up.Header.Set("Accept-Encoding", "identity")
	// The client's own identification, when it is plain text: Anthropic may
	// treat a Claude Code call differently from an anonymous one.
	up.Header.Set("User-Agent", relayUserAgent)
	if v := strings.TrimSpace(r.Header.Get("User-Agent")); v != "" && len(v) <= 200 && relayAgentText.MatchString(v) {
		up.Header.Set("User-Agent", v)
	}
	if v := strings.TrimSpace(r.Header.Get("X-App")); v != "" && len(v) <= 32 && relayTokenText.MatchString(v) {
		up.Header.Set("X-App", v)
	}

	// A copy of the client that never follows a redirect, so the key cannot
	// be carried to another host whatever client was configured.
	client := *relay.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(up)
	if err != nil {
		if ctx.Err() != nil && r.Context().Err() != nil {
			// The request may have reached Anthropic: charge its input as
			// an estimate (four bytes a token), so a caller that drops its
			// calls before the answer still spends its cap.
			relay.charge(computerID, relayUsage{InputTokens: int64(len(body) / 4)})
			log.Printf("gateway: anthropic relay %s: client went away (charged %d estimated input tokens)", computerID, len(body)/4)
			return
		}
		log.Printf("gateway: anthropic relay %s: upstream call failed: %s", computerID, relay.redact(sanitizeError(err)))
		relayError(w, http.StatusBadGateway, "api_error", "relay upstream unavailable")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		log.Printf("gateway: anthropic relay %s: upstream answered %d with a redirect; not followed", computerID, resp.StatusCode)
		relayError(w, http.StatusBadGateway, "api_error", "relay upstream redirected")
		return
	}
	copyRelayHeaders(w.Header(), resp.Header)
	for name, values := range w.Header() {
		for i, v := range values {
			values[i] = relay.redact(v)
		}
		w.Header()[name] = values
	}

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, anthropicErrBodyMax))
		out := []byte(relay.redact(string(errBody)))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(out)
		log.Printf("gateway: anthropic relay %s model=%s path=%s status=%d bytes_in=%d bytes_out=%d dur=%s",
			computerID, model, route.path, resp.StatusCode, len(body), len(out), time.Since(started).Round(time.Millisecond))
		return
	}

	w.WriteHeader(resp.StatusCode)
	flusher, _ := w.(http.Flusher)
	tap := newUsageTap(strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream"))
	var out int64
	buf := make([]byte, 32<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			tap.write(buf[:n])
			if _, werr := w.Write(buf[:n]); werr != nil {
				break
			}
			out += int64(n)
			if flusher != nil {
				flusher.Flush()
			}
		}
		if rerr != nil {
			break
		}
	}
	u := tap.finish()
	relay.recordUsage(u)
	relay.charge(computerID, u)
	log.Printf("gateway: anthropic relay %s credential=%s model=%s path=%s status=%d bytes_in=%d bytes_out=%d tokens=%d+%d cached=%d day_spent=%d dur=%s",
		computerID, relay.credentialKind(), model, route.path, resp.StatusCode, len(body), out, u.InputTokens, u.OutputTokens, u.CachedInputTokens, relay.spentToday(computerID), time.Since(started).Round(time.Millisecond))
}

// requestModel reads the top-level "model" of a Messages request, and refuses
// a body whose model is ambiguous: repeated, spelled in another case (Go's
// decoder would match it, a stricter or looser upstream parser might not), or
// not a plain string. The allowlist must check the model the upstream will
// run, so a doubt is a refusal.
func requestModel(body []byte) (string, string) {
	dec := json.NewDecoder(bytes.NewReader(body))
	tok, err := dec.Token()
	if d, ok := tok.(json.Delim); err != nil || !ok || d != '{' {
		return "", "request body must be a JSON object"
	}
	model, found := "", false
	for dec.More() {
		keyTok, err := dec.Token()
		key, ok := keyTok.(string)
		if err != nil || !ok {
			return "", "request body must be a JSON object"
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return "", "request body must be a JSON object"
		}
		if !strings.EqualFold(key, "model") {
			continue
		}
		if key != "model" || found {
			return "", "model must appear exactly once"
		}
		if json.Unmarshal(raw, &model) != nil {
			return "", "model must be a string"
		}
		found = true
	}
	if _, err := dec.Token(); err != nil { // the closing brace
		return "", "request body must be a JSON object"
	}
	if dec.More() {
		return "", "request body must be one JSON object"
	}
	if !found || model == "" {
		return "", "model is required"
	}
	return model, ""
}

// relayTokenText admits the characters an anthropic-version or anthropic-beta
// value uses and nothing that could split a header.
var relayTokenText = regexp.MustCompile(`^[A-Za-z0-9._,: -]+$`)

// relayAgentText admits a user agent such as "claude-cli/2.1.0 (external,
// sdk-cli)" and nothing that could split a header.
var relayAgentText = regexp.MustCompile(`^[A-Za-z0-9._,:;/()+ -]+$`)

var relayKeyShape = regexp.MustCompile(`sk-ant-[A-Za-z0-9_\-]{8,}`)

// redact removes the gateway's key, and anything shaped like an Anthropic
// key, from text bound for a log or a client.
func (r *AnthropicRelay) redact(s string) string {
	if r != nil && r.key != "" {
		s = strings.ReplaceAll(s, r.key, "[redacted]")
	}
	return relayKeyShape.ReplaceAllString(s, "[redacted]")
}

// copyRelayHeaders passes the response headers a client needs and drops the
// rest (cookies, edge and server identification, anything unnamed).
func copyRelayHeaders(dst, src http.Header) {
	for name, values := range src {
		canon := http.CanonicalHeaderKey(name)
		switch {
		case canon == "Content-Type", canon == "Cache-Control", canon == "Request-Id", canon == "X-Request-Id", canon == "Retry-After",
			strings.HasPrefix(canon, "Anthropic-"):
		default:
			continue
		}
		for _, v := range values {
			if strings.ContainsAny(v, "\r\n") {
				continue
			}
			dst.Add(canon, v)
		}
	}
	if strings.HasPrefix(dst.Get("Content-Type"), "text/event-stream") {
		dst.Set("X-Accel-Buffering", "no")
	}
}

func (r *AnthropicRelay) recordUsage(u relayUsage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage.Requests++
	r.usage.InputTokens += u.InputTokens
	r.usage.OutputTokens += u.OutputTokens
	r.usage.CachedInputTokens += u.CachedInputTokens
	r.usage.CacheCreationTokens += u.CacheCreationTokens
}

// usageTap reads token counts out of a relayed reply without holding it: a
// server-sent-event stream line by line (the last non-zero value of each
// counter wins, since message_delta repeats the running output total), a JSON
// reply in a bounded buffer. A counter it cannot read is zero; the reply is
// never altered.
type usageTap struct {
	sse     bool
	line    bytes.Buffer
	skip    bool
	json    bytes.Buffer
	overrun bool
	usage   relayUsage
}

func newUsageTap(sse bool) *usageTap { return &usageTap{sse: sse} }

type usageFields struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

type usageCarrier struct {
	Usage   *usageFields `json:"usage"`
	Message *struct {
		Usage *usageFields `json:"usage"`
	} `json:"message"`
}

func (t *usageTap) take(c usageCarrier) {
	for _, f := range []*usageFields{c.Usage, func() *usageFields {
		if c.Message != nil {
			return c.Message.Usage
		}
		return nil
	}()} {
		if f == nil {
			continue
		}
		if f.InputTokens > 0 {
			t.usage.InputTokens = f.InputTokens
		}
		if f.OutputTokens > 0 {
			t.usage.OutputTokens = f.OutputTokens
		}
		if f.CacheReadInputTokens > 0 {
			t.usage.CachedInputTokens = f.CacheReadInputTokens
		}
		if f.CacheCreationInputTokens > 0 {
			t.usage.CacheCreationTokens = f.CacheCreationInputTokens
		}
	}
}

func (t *usageTap) write(p []byte) {
	if !t.sse {
		if t.overrun {
			return
		}
		if t.json.Len()+len(p) > anthropicUsageJSONMax {
			t.overrun = true
			t.json.Reset()
			return
		}
		t.json.Write(p)
		return
	}
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			if !t.skip {
				if t.line.Len()+len(p) > anthropicUsageLineMax {
					t.skip = true
					t.line.Reset()
				} else {
					t.line.Write(p)
				}
			}
			return
		}
		if !t.skip {
			if t.line.Len()+i <= anthropicUsageLineMax {
				t.line.Write(p[:i])
				t.handleLine(t.line.Bytes())
			}
		}
		t.line.Reset()
		t.skip = false
		p = p[i+1:]
	}
}

func (t *usageTap) handleLine(line []byte) {
	line = bytes.TrimSpace(line)
	data, ok := bytes.CutPrefix(line, []byte("data:"))
	if !ok || !bytes.Contains(data, []byte(`"usage"`)) {
		return
	}
	var c usageCarrier
	if json.Unmarshal(bytes.TrimSpace(data), &c) == nil {
		t.take(c)
	}
}

func (t *usageTap) finish() relayUsage {
	if t.sse {
		if t.line.Len() > 0 && !t.skip {
			t.handleLine(t.line.Bytes())
		}
	} else if !t.overrun && t.json.Len() > 0 {
		var c usageCarrier
		if json.Unmarshal(t.json.Bytes(), &c) == nil {
			t.take(c)
		}
	}
	return t.usage
}

// Status is the relay's state for the startup log: "off", "dark" or "on",
// with the credential kind (never its value) or why a named token file gave
// none.
func (r *AnthropicRelay) Status() string {
	st := r.status()
	if kind := r.credentialKind(); kind != "" {
		return st + " (credential " + kind + ")"
	}
	if r != nil && r.credentialReason != "" {
		return st + " (" + r.credentialReason + ")"
	}
	return st
}

// relayGrant is the grant read's answer: the calling computer's own state.
type relayGrant struct {
	Available       bool   `json:"available"`
	Reason          string `json:"reason,omitempty"`
	ExpiresAt       string `json:"expires_at,omitempty"`
	Model           string `json:"model,omitempty"`
	DailyTokenCap   int64  `json:"daily_token_cap,omitempty"`
	DailyTokensUsed int64  `json:"daily_tokens_used"`
}

// handleAnthropicRelayGrant answers GET /provider/anthropic/grant: whether
// the relay serves the calling computer now. The bearer is validated without
// writing its pin, and an available answer needs the pin to name the
// caller's address, as a relay call does. It never names another computer or
// the credential.
func (h *Handler) handleAnthropicRelayGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		relayError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method not allowed")
		return
	}
	vmID, computerID, err := h.relayCaller(r)
	if err != nil {
		relayAuthFailure(w, err)
		return
	}
	// The pin is checked whatever the answer, so a copied bearer learns
	// nothing about the grant.
	if err := h.relayPeerCheck(vmID, r); err != nil {
		relayGrantPeerFailure(w, err)
		return
	}
	relay := h.anthropicRelay
	ok, reason := relay.grantState(computerID)
	out := relayGrant{Available: ok, Reason: reason}
	if relay != nil && computerID != "" && relay.identities[computerID] {
		if !relay.expires.IsZero() {
			out.ExpiresAt = relay.expires.UTC().Format(time.RFC3339)
		}
		out.Model = anthropicDefaultModel
		if !relay.models[out.Model] {
			for m := range relay.models {
				out.Model = m
				break
			}
		}
		out.DailyTokenCap = relay.dailyTokens
		out.DailyTokensUsed = relay.spentToday(computerID)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(out)
}
