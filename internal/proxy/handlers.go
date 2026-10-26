package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// clientIdentityHeaders is the list of HTTP headers that must be stripped from
// client requests before forwarding to the autoputer. These headers could be used
// to impersonate or spoof user identity, so the proxy removes them all and
// only injects the JWT-verified user context via X-Authenticated-User.
const (
	proxyOriginalEscapedPathHeader = "X-Proxy-Original-Escaped-Path"
	proxyOriginalRawQueryHeader    = "X-Proxy-Original-Raw-Query"
)

var clientIdentityHeaders = []string{
	"X-Authenticated-User",
	"X-Authenticated-Email",
	"X-Authenticated-Scopes",
	// The guest's computer router takes this header as the computer binding
	// (agentcore api_self_development.go); only the proxy may set it
	// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F1).
	"X-Authenticated-Computer",
	"X-User-Id",
	"X-User-Name",
	"X-Forwarded-User",
	"X-Remote-User",
	"X-Auth-User",
	"X-Internal-Caller",
	hostauth.Header,
	// Headers other services trust from their own callers. No client value
	// reaches them through the proxy today; strip them so none ever does.
	"X-Internal-Updater",
	"X-Choir-Operator",
	"X-Choir-Operator-Token",
	"X-Choir-Lifecycle-Caller",
	"X-Choir-Capsule",
}

// proxyCarrierHeaders carry proxy-validated context from a handler to the
// reverse proxy director. A client must never pre-set them: the director
// reads them before it deletes them.
var proxyCarrierHeaders = []string{
	"X-Proxy-Trusted-User",
	"X-Proxy-Trusted-Email",
	"X-Proxy-Trusted-Scopes",
	"X-Resolved-Autoputer-URL",
	proxyOriginalEscapedPathHeader,
	proxyOriginalRawQueryHeader,
	"X-Original-Path",
	"X-Original-RawQuery",
}

// stripClientSuppliedHeaders removes every identity and carrier header from
// a public request at ingress, before any handler reads or forwards it.
func stripClientSuppliedHeaders(r *http.Request) {
	if r == nil || r.Header == nil {
		return
	}
	for _, hdr := range clientIdentityHeaders {
		r.Header.Del(hdr)
	}
	for _, hdr := range proxyCarrierHeaders {
		r.Header.Del(hdr)
	}
}

var (
	// autoputerResolveRetryWindow bounds retries for transient vmctl errors that
	// return quickly (e.g. connection refused, 503). It must stay below the
	// vmctl client timeout (DefaultClientTimeout = 60s) so a hung call does not
	// trigger a second long attempt.
	autoputerResolveRetryWindow    = 10 * time.Second
	autoputerResolveRetryBaseDelay = 200 * time.Millisecond
	autoputerResolveRetryMaxDelay  = time.Second
)

// errorResponse is a generic JSON error envelope.
type errorResponse struct {
	Error          string `json:"error"`
	Reason         string `json:"reason,omitempty"`
	UpstreamStatus int    `json:"upstream_status,omitempty"`
	// Recovery refusal fields: durable condition kind, retry hint and repair
	// job status. Omitted for ordinary errors.
	Kind              string                     `json:"kind,omitempty"`
	RetryAfterSeconds int                        `json:"retry_after_seconds,omitempty"`
	Repair            *vmctl.CheckpointJobStatus `json:"repair,omitempty"`
}

// proxyHealthResponse is the JSON structure returned by the proxy /health
// endpoint. It intentionally exposes only coarse service status and deployed
// build identity; host pressure, global VM inventory, vmctl URLs, and raw VM
// handles must remain off browser-public surfaces.
type proxyHealthResponse struct {
	Status       string                 `json:"status"`
	Service      string                 `json:"service"`
	Upstream     string                 `json:"upstream"`
	VMctlRouting string                 `json:"vmctl_routing,omitempty"`
	VMctlStatus  string                 `json:"vmctl_status,omitempty"`
	Lifecycle    lifecycleHealthSummary `json:"lifecycle,omitempty"`
	Build        buildinfo.Info         `json:"build"`
	// Platform is the commit of the last successful staging deploy, from
	// the activation receipt. It sits beside Build because Build is this
	// binary's own identity and stays blank after a partial deploy that did
	// not move the proxy (docs/problems/
	// partial-deploy-receipt-blanks-deployed-commit-2026-10-14.md); CI
	// diffs deploys from this answer.
	Platform      *buildinfo.Platform `json:"platform,omitempty"`
	UpstreamBuild *buildinfo.Info     `json:"upstream_build,omitempty"`
}

type proxyVMctlHealthSummary struct {
	Status          string                      `json:"status"`
	Service         string                      `json:"service"`
	ActiveVMs       int                         `json:"active_vms"`
	TotalOwnerships int                         `json:"total_ownerships"`
	IdleEligible    int                         `json:"idle_eligible"`
	Reclaim         vmctl.PressureReclaimPlan   `json:"reclaim"`
	Warmness        vmctl.WarmnessHealthSummary `json:"warmness"`
}

// AuthResult holds the result of access JWT or API key validation.
type AuthResult struct {
	UserID     string
	Email      string
	Valid      bool
	Scopes     []string // empty for cookie auth = full access
	ComputerID string   // optional stable-computer binding for API keys
	AuthMethod string   // "cookie" or "api_key"
	// CredentialRef names the credential for an owner-act receipt:
	// "session-jwt-sha256:<sha256 of the access JWT>" (the JWT has no jti)
	// or "api-key:<key id>". Never the secret itself.
	CredentialRef string
}

// APIKeyValidator is the interface the proxy uses to validate Bearer token
// (API key) auth. It is satisfied by *auth.Store. When no validator is
// configured (nil), API key auth is skipped and only cookie auth is used.
type APIKeyValidator interface {
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*auth.APIKey, error)
	TouchAPIKeyLastUsed(ctx context.Context, keyID string) error
	GetUserByID(id string) (*auth.User, error)
}

func requestDesktopID(r *http.Request) string {
	if r == nil {
		return vmctl.PrimaryDesktopID
	}
	if desktopID := strings.TrimSpace(r.URL.Query().Get("desktop_id")); desktopID != "" {
		return desktopID
	}
	if desktopID := strings.TrimSpace(r.Header.Get("X-Choir-Desktop")); desktopID != "" {
		return desktopID
	}
	return vmctl.PrimaryDesktopID
}

// Handler provides HTTP and WebSocket handlers for the proxy routes.
type Handler struct {
	cfg                        *Config
	pubKey                     ed25519.PublicKey
	reverseProxy               *httputil.ReverseProxy
	upgrader                   websocket.Upgrader
	dialer                     *websocket.Dialer
	corpusd                    *http.Client
	maild                      *http.Client
	autoputerHTTP              *http.Client
	replayAutoputerHTTP        *http.Client  // dedicated long-budget client for replay completeness
	residueImportAutoputerHTTP *http.Client  // dedicated long-budget client for residue import
	autoputerURL               *url.URL      // parsed autoputer URL for WS dial derivation
	vmctlClient                *vmctl.Client // optional vmctl client for VM-backed routing
	apiKeyTouchSent            sync.Map      // userID\x00computerID -> time of the last idle-clock touch sent to vmctl
	lifecycle                  *lifecycleRecorder
	recoveries                 *computeRecoveryTracker
	apiKeyValidator            APIKeyValidator // optional: enables Bearer token (API key) auth
	authStore                  *auth.Store     // optional: owned auth store for API key validation
	platformSignerDigest       string
	// ownerActSigner is the proxy's own host-owner-act key (S1): it signs
	// that the proxy authenticated the owner for one act. nil: no receipts
	// are minted (corpusd's CHOIR_OWNER_ACT_MODE decides what follows).
	ownerActSigner *owneract.Signer
	// routeCacheTTL bounds the short-lived computer-route memo
	// (resolveComputerURL + ensureComputerVersionRoute). vmctl remains the
	// route of record; the cache only avoids paying two resolve RPCs on every
	// request. Entries self-expire and are invalidated on upstream transport
	// failure so a moved VM never wedges behind a stale URL.
	routeCacheTTL time.Duration
	routeCacheMu  sync.Mutex
	routeCache    map[string]routeCacheEntry

	// Story expansion requests (platform_expand.go): dark unless
	// expandRequesters holds a user id and expandOff is false.
	expandRequesters map[string]bool
	expandOff        bool
	expandPublic     bool
	expandMu         sync.Mutex
	expand           *expandLedger
}

// Context keys used to carry the resolved upstream URL and its invalidator
// from the request handler into the reverse-proxy ErrorHandler (the Director
// strips the header before the upstream request is issued).
type proxyContextKey string

const (
	resolvedAutoputerURLContextKey proxyContextKey = "resolved_autoputer_url"
	routeInvalidatorContextKey     proxyContextKey = "route_invalidator"
)

// NewHandler creates a proxy Handler with the given config and auth public key.
// It initializes the reverse-proxy transport and WebSocket dialer. Production
// request handling resolves an immutable D-ROUTE slot and then its vmctl-owned
// disposable realization; missing route authority fails closed.

func NewHandler(cfg *Config, pubKey ed25519.PublicKey) (*Handler, error) {
	if strings.TrimSpace(cfg.CorpusdURL) == "" {
		cfg.CorpusdURL = DefaultCorpusdURL
	}
	if strings.TrimSpace(cfg.MaildURL) == "" {
		cfg.MaildURL = DefaultMaildURL
	}
	autoputerURL, err := url.Parse(cfg.ComputerURL)
	if err != nil {
		return nil, fmt.Errorf("parse autoputer URL %s: %w", cfg.ComputerURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(autoputerURL)

	// Context keys used to carry the resolved upstream URL and its invalidator
	// from the request handler into the reverse-proxy ErrorHandler (the Director
	// strips the header before the upstream request is issued).
	type proxyContextKey string

	const (
		resolvedAutoputerURLContextKey proxyContextKey = "resolved_autoputer_url"
		routeInvalidatorContextKey     proxyContextKey = "route_invalidator"
	)

	// A transport failure against a cached route means the cached URL may
	// point at a moved or dead VM — drop it so the next request re-resolves
	// instead of wedging behind the stale target for the TTL window. The
	// resolved URL is stashed in the request context because the Director
	// strips the header before the upstream request is issued.
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		if resolved, ok := req.Context().Value(resolvedAutoputerURLContextKey).(string); ok && resolved != "" {
			if invalidator, ok := req.Context().Value(routeInvalidatorContextKey).(func(string)); ok {
				invalidator(resolved)
			}
		}
		log.Printf("proxy: upstream transport error path=%s err=%v", req.URL.Path, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
	}

	// Flush immediately for SSE streaming responses (for example Trace) and
	// other streaming endpoints. A value of -1 means flush after each
	// write to the client, which ensures SSE events arrive incrementally
	// rather than being buffered (VAL-RUNTIME-005).
	proxy.FlushInterval = -1

	// Customize the director to preserve the original request path and query
	// without rewriting. The default NewSingleHostReverseProxy director
	// replaces the path, but we want the autoputer to receive the same public
	// path (e.g., /api/shell/bootstrap) so that prefix preservation is
	// observable end to end.
	//
	// The director also handles user-context injection: it strips all
	// client-supplied identity headers (to prevent spoofing), then sets
	// X-Authenticated-User from the trusted X-Proxy-Trusted-User header
	// that the proxy handler sets after JWT validation.
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Override the path and query to preserve the original public request.
		// Path and RawPath must be restored together: restoring only decoded Path
		// would normalize noncanonical escapes before the autoputer can reject them.
		escapedPath := req.Header.Get(proxyOriginalEscapedPathHeader)
		decodedPath, err := url.PathUnescape(escapedPath)
		reconstructed := &url.URL{Path: decodedPath, RawPath: escapedPath}
		if err != nil || escapedPath == "" || reconstructed.EscapedPath() != escapedPath {
			// The handler validates this trusted header before proxying. Keep this
			// defensive fallback off every public route if that invariant is broken.
			req.URL.Path = "/.choir-invalid-proxy-path"
			req.URL.RawPath = ""
			req.URL.RawQuery = ""
		} else {
			req.URL.Path = decodedPath
			req.URL.RawPath = escapedPath
			req.URL.RawQuery = req.Header.Get(proxyOriginalRawQueryHeader)
		}

		// Check if vmctl resolved a per-user VM autoputer URL. When set,
		// override the target to the resolved VM (VAL-VM-001, VAL-VM-002).
		if resolved := req.Header.Get("X-Resolved-Autoputer-URL"); resolved != "" {
			if resolvedURL, err := url.Parse(resolved); err == nil {
				req.URL.Scheme = resolvedURL.Scheme
				req.URL.Host = resolvedURL.Host
				req.Host = resolvedURL.Host
			}
		} else {
			// Set the Host header to the default autoputer host.
			req.Host = autoputerURL.Host
		}

		// Strip ALL client-supplied identity headers to prevent spoofing.
		// Only the proxy-verified user context is allowed through.
		for _, hdr := range clientIdentityHeaders {
			req.Header.Del(hdr)
		}
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")

		// Inject trusted user context from the proxy-validated JWT.
		trustedUser := req.Header.Get("X-Proxy-Trusted-User")
		if trustedUser != "" {
			req.Header.Set("X-Authenticated-User", trustedUser)
		}
		trustedEmail := req.Header.Get("X-Proxy-Trusted-Email")
		if trustedEmail != "" {
			req.Header.Set("X-Authenticated-Email", trustedEmail)
		}
		trustedScopes := req.Header.Get("X-Proxy-Trusted-Scopes")
		if trustedScopes != "" {
			req.Header.Set("X-Authenticated-Scopes", trustedScopes)
		}

		// Clean up internal proxy headers before forwarding.
		req.Header.Del("X-Proxy-Trusted-User")
		req.Header.Del("X-Proxy-Trusted-Email")
		req.Header.Del("X-Proxy-Trusted-Scopes")
		req.Header.Del(proxyOriginalEscapedPathHeader)
		req.Header.Del(proxyOriginalRawQueryHeader)
		// Strip legacy names too so client-supplied values cannot leak upstream.
		req.Header.Del("X-Original-Path")
		req.Header.Del("X-Original-RawQuery")
		req.Header.Del("X-Resolved-Autoputer-URL")
	}

	// Optional vmctl client for VM-backed routing.
	var vmctlCli *vmctl.Client
	if cfg.VmctlRoutingEnabled() {
		vmctlCli = vmctl.NewClientWithTimeout(cfg.VmctlURL, cfg.VmctlTimeout)
		log.Printf("proxy: vmctl-backed routing enabled (vmctl=%s timeout=%s)", cfg.VmctlURL, cfg.VmctlTimeout)
	}

	// Optional auth store for API key (Bearer token) validation. When
	// AuthDBPath is configured, the proxy opens the auth database and can
	// validate API keys as a fallback to cookie-based JWT auth.
	var authStore *auth.Store
	if strings.TrimSpace(cfg.AuthDBPath) != "" {
		as, err := auth.OpenStore(cfg.AuthDBPath)
		if err != nil {
			return nil, fmt.Errorf("open auth store for api key validation: %w", err)
		}
		authStore = as
		log.Printf("proxy: api key (bearer token) auth enabled (auth_db=%s)", cfg.AuthDBPath)

		// Bootstrap admin API key: if CHOIR_BOOTSTRAP_ADMIN_API_KEY is set,
		// seed it into the auth DB on first run (when no API keys exist).
		// This is the escape hatch for headless agents (the choir CLI) to
		// authenticate before any WebAuthn-provisioned key exists. The
		// first-run-only guard makes repeated starts safe.
		if bootstrapKey := strings.TrimSpace(os.Getenv("CHOIR_BOOTSTRAP_ADMIN_API_KEY")); bootstrapKey != "" {
			if _, seeded, err := authStore.SeedBootstrapAdminAPIKey(context.Background(), bootstrapKey); err != nil {
				log.Printf("proxy: bootstrap admin api key: %v", err)
				// Non-fatal: the proxy can still validate existing keys.
				// A failed seed is logged but does not block startup.
			} else if seeded {
				log.Printf("proxy: bootstrap admin api key activated — revoke it once a WebAuthn-provisioned key exists")
			}
		}
	}

	platformSignerDigest, err := computerevent.PlatformControlTrustDigest()
	if err != nil {
		return nil, fmt.Errorf("load platform trust anchor: %w", err)
	}
	ownerActSigner, ownerActKeyPath, err := owneract.LoadSignerFromEnv()
	switch {
	case err != nil:
		log.Printf("proxy: owner-act key unusable, owner acts go to corpusd without a receipt: %v", err)
		ownerActSigner = nil
	case ownerActSigner == nil && ownerActKeyPath != "":
		log.Printf("proxy: no owner-act key at %s, owner acts go to corpusd without a receipt", ownerActKeyPath)
	case ownerActSigner == nil:
	default:
		log.Printf("proxy: owner-act key %s loaded from %s", ownerActSigner.KeyID(), ownerActKeyPath)
	}

	// Build the handler. When authStore is nil, apiKeyValidator must be a
	// nil interface (not a typed-nil *auth.Store) so the nil check in
	// validateAPIKey works correctly.
	replayTimeout := cfg.ReplayCompletenessTimeout
	if replayTimeout <= 0 {
		replayTimeout = DefaultReplayCompletenessTimeout
	}
	residueImportTimeout := cfg.ResidueImportTimeout
	if residueImportTimeout <= 0 {
		residueImportTimeout = DefaultResidueImportTimeout
	}
	autoputerHTTP := &http.Client{Timeout: 30 * time.Second}
	replayAutoputerHTTP := &http.Client{Timeout: replayTimeout, Transport: autoputerHTTP.Transport}
	residueImportAutoputerHTTP := &http.Client{Timeout: residueImportTimeout, Transport: autoputerHTTP.Transport}

	h := &Handler{
		cfg:          cfg,
		pubKey:       pubKey,
		reverseProxy: proxy,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// The proxy is the trust boundary for origin validation.
				// Accept all origins here; the deployed Caddy layer and
				// same-origin cookie policy enforce origin checks.
				return true
			},
		},
		dialer:                     websocket.DefaultDialer,
		corpusd:                    &http.Client{Timeout: 30 * time.Second},
		maild:                      &http.Client{Timeout: 30 * time.Second},
		autoputerHTTP:              autoputerHTTP,
		replayAutoputerHTTP:        replayAutoputerHTTP,
		residueImportAutoputerHTTP: residueImportAutoputerHTTP,
		autoputerURL:               autoputerURL,
		vmctlClient:                vmctlCli,
		lifecycle:                  newLifecycleRecorder(),
		recoveries:                 newComputeRecoveryTracker(),
		authStore:                  authStore,
		platformSignerDigest:       platformSignerDigest,
		ownerActSigner:             ownerActSigner,
		expandRequesters:           expandRequesterSet(cfg.ExpandRequesters),
		expandOff:                  cfg.ExpandOff,
		expandPublic:               cfg.ExpandPublic,
	}
	if authStore != nil {
		h.apiKeyValidator = authStore
	}
	return h, nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("proxy handler: json encode error: %v", err)
	}
}

// SetAPIKeyValidator sets the API key validator used for Bearer token auth.
// This is primarily used in tests to inject a mock validator without opening
// a real auth database. In production, the validator is configured via
// Config.AuthDBPath in NewHandler.
func (h *Handler) SetAPIKeyValidator(v APIKeyValidator) {
	h.apiKeyValidator = v
}

// setTrustedAuthHeaders injects the proxy-validated user context as internal
// carrier headers for the reverse proxy director to consume. The director
// strips all client-supplied identity headers and replaces them with these
// trusted values before forwarding to the upstream.
func (h *Handler) setTrustedAuthHeaders(r *http.Request, authResult *AuthResult) {
	// Clear first: Email and Scopes are set only when present, so no stale
	// or client value may survive for the director to copy.
	r.Header.Del("X-Proxy-Trusted-User")
	r.Header.Del("X-Proxy-Trusted-Email")
	r.Header.Del("X-Proxy-Trusted-Scopes")
	r.Header.Set("X-Proxy-Trusted-User", authResult.UserID)
	if authResult.Email != "" {
		r.Header.Set("X-Proxy-Trusted-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		r.Header.Set("X-Proxy-Trusted-Scopes", strings.Join(authResult.Scopes, ","))
	}
}

// validateAccessJWT validates the access JWT from the choir_access cookie.
// It returns the user ID if valid, or an error if the token is missing,
// invalid, expired, tampered, or not an access-scoped token.
func (h *Handler) validateAccessJWT(r *http.Request) (*AuthResult, error) {
	cookie, err := r.Cookie("choir_access")
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, errors.New("missing access token cookie")
		}
		return nil, fmt.Errorf("read access cookie: %w", err)
	}

	if cookie.Value == "" {
		return nil, errors.New("empty access token cookie")
	}

	token, err := jwt.Parse(cookie.Value, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return h.pubKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return nil, errors.New("invalid token subject")
	}

	scope, _ := claims["scope"].(string)
	if scope != "access" {
		return nil, errors.New("token is not an access token")
	}
	email, _ := claims["email"].(string)

	tokenSum := sha256.Sum256([]byte(cookie.Value))
	return &AuthResult{UserID: userID, Email: strings.TrimSpace(email), Valid: true, AuthMethod: "cookie",
		CredentialRef: "session-jwt-sha256:" + hex.EncodeToString(tokenSum[:])}, nil
}

// authenticate tries cookie-based JWT auth first (browser sessions), then
// falls back to Bearer token (API key) auth for headless access. This is the
// single entry point for all protected route auth — existing WebAuthn session
// flows are unchanged; API keys are an additional path.
func (h *Handler) authenticate(r *http.Request) (*AuthResult, error) {
	// 1. Try cookie-based JWT (browser sessions).
	if result, err := h.validateAccessJWT(r); err == nil {
		return result, nil
	}
	// 2. Try Bearer token (API keys for headless access).
	if result, err := h.validateAPIKey(r); err == nil {
		return result, nil
	}
	return nil, errors.New("no valid authentication")
}

func (h *Handler) authorizeAPIKeyScope(w http.ResponseWriter, r *http.Request, authResult *AuthResult) bool {
	if authResult == nil || authResult.AuthMethod != "api_key" {
		return true
	}
	requiredScope := requiredAPIKeyScope(r)
	if requiredScope == "" || hasAPIKeyScope(authResult.Scopes, "admin") || hasAPIKeyScope(authResult.Scopes, requiredScope) {
		return true
	}
	writeJSON(w, http.StatusForbidden, errorResponse{Error: "missing required scope: " + requiredScope})
	return false
}

func requiredAPIKeyScope(r *http.Request) string {
	if r == nil || r.URL == nil {
		return "read:runtime"
	}
	action := "read"
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
	default:
		action = "write"
	}
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/precommit/"):
		// Calibration level 2: agents' claim keys carry only the narrow
		// precommit scopes, never read/write:runtime.
		return "precommit:" + action
	case strings.HasPrefix(r.URL.Path, "/api/base/"):
		return action + ":base"
	case strings.HasPrefix(r.URL.Path, "/api/texture/"):
		return action + ":texture"
	default:
		return action + ":runtime"
	}
}

func hasAPIKeyScope(scopes []string, want string) bool {
	for _, scope := range scopes {
		if scope == want {
			return true
		}
	}
	return false
}

// validateAPIKey validates an API key from the Authorization: Bearer header.
// It extracts the token, SHA-256 hashes it, looks up the key in the auth
// store, checks it is not revoked or expired, updates last_used_at, and
// returns an AuthResult with the user ID and scopes.
func (h *Handler) validateAPIKey(r *http.Request) (*AuthResult, error) {
	if h.apiKeyValidator == nil {
		return nil, errors.New("api key auth not configured")
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("no authorization header")
	}

	// Expect "Bearer choir_sk_...".
	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return nil, errors.New("authorization header is not a bearer token")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	if token == "" {
		return nil, errors.New("empty bearer token")
	}

	// Only accept choir_sk_ prefixed tokens.
	if !strings.HasPrefix(token, auth.APIKeyPrefix) {
		return nil, errors.New("bearer token is not an api key")
	}

	// Hash the token with SHA-256.
	hSum := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(hSum[:])

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	ak, err := h.apiKeyValidator.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("api key not found: %w", err)
	}

	// Check expiry.
	if ak.ExpiresAt != nil && time.Now().UTC().After(*ak.ExpiresAt) {
		return nil, errors.New("api key expired")
	}

	// Look up the user to get the email for header injection.
	user, err := h.apiKeyValidator.GetUserByID(ak.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found for api key: %w", err)
	}

	// Update last_used_at (non-fatal on error).
	_ = h.apiKeyValidator.TouchAPIKeyLastUsed(ctx, ak.ID)

	return &AuthResult{
		UserID:     ak.UserID,
		Email:      user.Email,
		Valid:      true,
		Scopes:     ak.Scopes,
		ComputerID: ak.ComputerID,
		AuthMethod: "api_key",
		// The key id, never the secret or its hash.
		CredentialRef: "api-key:" + ak.ID,
	}, nil
}

// preserveOriginalRequestTarget captures the request target in headers that
// only the proxy director consumes. Client values are always deleted first.
// RawPath is rejected rather than normalized when it is not a valid encoding
// hint for Path.
func preserveOriginalRequestTarget(r *http.Request) bool {
	if r == nil || r.URL == nil || r.Header == nil {
		return false
	}
	r.Header.Del(proxyOriginalEscapedPathHeader)
	r.Header.Del(proxyOriginalRawQueryHeader)

	escapedPath := r.URL.EscapedPath()
	decodedPath, err := url.PathUnescape(escapedPath)
	reconstructed := &url.URL{Path: decodedPath, RawPath: escapedPath}
	if err != nil || escapedPath == "" || !strings.HasPrefix(escapedPath, "/") || decodedPath != r.URL.Path || reconstructed.EscapedPath() != escapedPath {
		return false
	}
	if r.URL.RawPath != "" && r.URL.RawPath != escapedPath {
		return false
	}

	r.Header.Set(proxyOriginalEscapedPathHeader, escapedPath)
	r.Header.Set(proxyOriginalRawQueryHeader, r.URL.RawQuery)
	return true
}

// HandleBootstrap handles GET /api/shell/bootstrap.
// It validates the access JWT cookie, denies requests with missing or invalid
// auth, and forwards authenticated requests to the autoputer upstream.
// When vmctl routing is enabled, resolves through VM ownership instead of
// the static autoputer fallback (VAL-VM-001, VAL-VM-002).
// The proxy injects the authenticated user context via the
// X-Authenticated-User header and preserves the original request path, method,
// query string, and upstream status/body.
func (h *Handler) HandleBootstrap(w http.ResponseWriter, r *http.Request) {
	stripClientSuppliedHeaders(r)
	started := time.Now()
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		h.lifecycle.record("bootstrap.method", "method_not_allowed", time.Since(started))
		return
	}

	// Validate auth (cookie JWT or Bearer API key).
	authStarted := time.Now()
	authResult, err := h.authenticate(r)
	if err != nil {
		// Missing or invalid auth — deny with a machine-readable auth failure.
		// Do NOT reach the upstream.
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record("bootstrap.auth", "unauthorized", time.Since(authStarted))
		h.lifecycle.record("bootstrap.total", "unauthorized", time.Since(started))
		return
	}
	h.lifecycle.record("bootstrap.auth", "ok", time.Since(authStarted))
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		h.lifecycle.record("bootstrap.authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record("bootstrap.total", "forbidden", time.Since(started))
		return
	}
	if !preserveOriginalRequestTarget(r) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request path"})
		h.lifecycle.record("bootstrap.path", "invalid", time.Since(started))
		h.lifecycle.record("bootstrap.total", "invalid_path", time.Since(started))
		return
	}

	// Resolve only the exact API-key-bound computer (cookie routing is unchanged).
	desktopID := requestDesktopID(r)
	computerTarget, ok := h.requireAPIKeyComputerTarget(w, r, authResult, "", desktopID)
	if !ok {
		return
	}
	resolveStarted := time.Now()
	autoputerURL, err := h.resolveComputerURLForComputerTarget(r.Context(), authResult, computerTarget, desktopID)
	if err != nil {
		log.Printf("proxy: failed to resolve autoputer for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeResolveError(w, err)
		h.lifecycle.record("bootstrap.resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record("bootstrap.total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record("bootstrap.resolve", "ok", time.Since(resolveStarted))

	// Auth is valid. Store the trusted user context for the director to inject.
	h.setTrustedAuthHeaders(r, authResult)

	// The original escaped path and query were captured after authentication.

	// If vmctl resolved a different URL, override the reverse proxy target.
	if autoputerURL != h.cfg.ComputerURL {
		h.setResolvedRouteContext(r, autoputerURL)
		r.Header.Set("X-Resolved-Autoputer-URL", autoputerURL)
	}

	upstreamStarted := time.Now()
	recorder := &lifecycleStatusRecorder{ResponseWriter: w}
	h.reverseProxy.ServeHTTP(recorder, r)
	h.lifecycle.record("bootstrap.upstream", lifecycleHTTPStatus(recorder.status), time.Since(upstreamStarted))
	h.lifecycle.record("bootstrap.total", lifecycleHTTPStatus(recorder.status), time.Since(started))
}

// HandleProtectedAPI is a generic handler for /api/* routes that require auth.
// It validates the access JWT and forwards authenticated requests to the
// autoputer. When vmctl routing is enabled, it resolves the user's VM through
// vmctl ownership instead of using the static autoputer URL (VAL-VM-001,
// VAL-VM-002).
func (h *Handler) HandleProtectedAPI(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	stagePrefix := "api"
	if r != nil && r.URL != nil && r.URL.Path == "/api/prompt-bar" {
		stagePrefix = "prompt_bar"
	}
	// The isolation self-test's oom and seccomp legs load the guest on
	// purpose and are host-internal (the guest serves them on
	// /internal/runtime/isolation-selftest, which this proxy never forwards).
	// No owner session or API key reaches them through /api.
	if r != nil && r.URL != nil && r.URL.Path == "/api/runtime/isolation-selftest" && r.URL.Query().Has("leg") {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	// Validate auth (cookie JWT or Bearer API key).
	authStarted := time.Now()
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record(stagePrefix+".auth", "unauthorized", time.Since(authStarted))
		h.lifecycle.record(stagePrefix+".total", "unauthorized", time.Since(started))
		return
	}
	h.lifecycle.record(stagePrefix+".auth", "ok", time.Since(authStarted))
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		h.lifecycle.record(stagePrefix+".authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record(stagePrefix+".total", "forbidden", time.Since(started))
		return
	}
	if !preserveOriginalRequestTarget(r) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request path"})
		h.lifecycle.record(stagePrefix+".path", "invalid", time.Since(started))
		h.lifecycle.record(stagePrefix+".total", "invalid_path", time.Since(started))
		return
	}

	// Resolve only the authenticated user's exact API-key-bound computer.
	desktopID := requestDesktopID(r)
	computerTarget, ok := h.requireAPIKeyComputerTarget(w, r, authResult, "", desktopID)
	if !ok {
		h.lifecycle.record(stagePrefix+".authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record(stagePrefix+".total", "forbidden", time.Since(started))
		return
	}
	resolveStarted := time.Now()
	autoputerURL, err := h.resolveComputerURLForComputerTarget(r.Context(), authResult, computerTarget, desktopID)
	if err != nil {
		log.Printf("proxy: failed to resolve autoputer for owner %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeResolveError(w, err)
		h.lifecycle.record(stagePrefix+".resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record(stagePrefix+".total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record(stagePrefix+".resolve", "ok", time.Since(resolveStarted))

	// Auth is valid. Store the trusted user context for the director.
	h.setTrustedAuthHeaders(r, authResult)

	// If vmctl resolved a different URL, override the reverse proxy target.
	if autoputerURL != h.cfg.ComputerURL {
		h.setResolvedRouteContext(r, autoputerURL)
		r.Header.Set("X-Resolved-Autoputer-URL", autoputerURL)
	}

	upstreamStarted := time.Now()
	recorder := &lifecycleStatusRecorder{ResponseWriter: w}
	h.reverseProxy.ServeHTTP(recorder, r)
	h.lifecycle.record(stagePrefix+".upstream", lifecycleHTTPStatus(recorder.status), time.Since(upstreamStarted))
	h.lifecycle.record(stagePrefix+".total", lifecycleHTTPStatus(recorder.status), time.Since(started))
}

// HandleAPI routes /api/* traffic. It applies auth gating for every HTTP
// /api/* route and dispatches to specific handlers only where the proxy must
// speak a different protocol, such as WebSocket upgrades. Authenticated HTTP
// /api/* requests are forwarded by default so new autoputer apps do not require
// proxy allowlist changes. Signed-out callers still see a 401 denial rather
// than a route-specific 404 that might suggest which routes exist.
func (h *Handler) HandleAPI(w http.ResponseWriter, r *http.Request) {
	stripClientSuppliedHeaders(r)
	path := r.URL.Path

	// Route protocol-specific protected paths.
	switch {
	case path == "/api/shell/bootstrap":
		h.HandleBootstrap(w, r)
		return
	case path == "/api/acceptance/execution-identity":
		h.HandleExecutionIdentity(w, r)
		return
	case path == "/api/compute/status":
		h.HandleComputeStatus(w, r)
		return
	case path == "/api/compute/recovery":
		h.HandleComputeRecovery(w, r)
		return
	case isComputerLifecyclePath(path):
		h.HandleComputerLifecycle(w, r)
		return
	case isComputerColdRecoverPath(path):
		h.HandleComputerWorkspaceReplace(w, r)
		return
	case isComputerWorkspaceReplacePath(path):
		h.HandleComputerWorkspaceReplace(w, r)
		return
	case isComputerRematerializePath(path):
		h.HandleComputerWorkspaceReplace(w, r)
		return
	case isComputerRestorePath(path):
		h.HandleComputerWorkspaceReplace(w, r)
		return
	case isComputerCheckpointPath(path):
		h.HandleComputerWorkspaceReplace(w, r)
		return
	case isComputerImportResidueSnapshotPath(path):
		h.HandleComputerWorkspaceReplace(w, r)
		return
	case isComputerProducerReportsPath(path):
		h.HandleComputerProducerReports(w, r)
		return
	case isComputerBootstrapChainPath(path):
		h.HandleComputerBootstrapChain(w, r)
		return
	case isComputerRecoveryJobPath(path):
		h.HandleComputerRecoveryJob(w, r)
		return
	case isSelfDevelopmentModePath(path):
		h.HandleSelfDevelopmentMode(w, r)
		return
	case isSelfDevelopmentReplayCompletenessPath(path):
		h.HandleSelfDevelopmentReplayCompleteness(w, r)
		return
	case isSelfDevelopmentGuestPath(path):
		h.HandleSelfDevelopmentGuest(w, r)
		return
	case isComputerReleasePublishPath(path):
		h.HandleComputerReleasePublish(w, r)
		return
	case isReleasesPath(path):
		h.HandleReleases(w, r)
		return
	case strings.HasPrefix(path, "/api/computers/") && strings.Contains(path, "/self-development/"):
		writeJSON(w, http.StatusConflict, errorResponse{Error: "self-development effects are disabled"})
		return
	case path == "/api/pulse/summary":
		h.HandlePulseSummary(w, r)
		return
	case path == "/api/ws":
		h.HandleWS(w, r)
		return
	case path == "/api/super-console/ws":
		// The Management Console's zot session is removed
		// (docs/problems/remove-zot-2026-10-22.md). Answer here so the
		// generic forward never carries it to a guest still on an old image.
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	case path == "/api/platform/texture/publications":
		h.HandleTexturePublication(w, r)
		return
	case path == "/api/platform/texture/opinions":
		h.HandleOpinionPublication(w, r)
		return
	case path == "/api/platform/publications/resolve":
		h.HandlePlatformPublicationResolve(w, r)
		return
	case path == "/api/platform/publications/export":
		h.HandlePlatformPublicationExport(w, r)
		return
	case path == "/api/platform/publications/front":
		h.HandlePlatformFrontStories(w, r)
		return
	case path == "/api/platform/publications/related":
		h.HandlePlatformPublicationRelated(w, r)
		return
	case path == "/api/platform/publications/expand":
		h.HandlePlatformPublicationExpand(w, r)
		return
	case path == "/api/platform/publications/expansion":
		h.HandlePlatformPublicationExpansion(w, r)
		return
	case path == "/api/platform/retrieval/search":
		h.HandlePlatformRetrievalSearch(w, r)
		return
	case strings.HasPrefix(path, "/api/platform/publications/") && strings.HasSuffix(path, "/proposals"):
		h.HandlePublicationProposal(w, r)
		return
	case path == "/api/email/resend/webhook":
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	case strings.HasPrefix(path, "/api/email/"):
		h.HandleEmailAPI(w, r)
		return
	case strings.HasPrefix(path, "/api/notifications/"):
		h.HandleNotificationAPI(w, r)
		return
	case strings.HasPrefix(path, "/api/computers/"):
		// Every computer-scoped route is matched above, where the proxy
		// checks ownership and scope and sets the computer binding itself.
		// A spelling the matchers miss (a trailing slash) must not reach the
		// guest through the generic forward, which checks only the
		// read/write:runtime scope (auth-boundary-gaps F1).
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	case strings.HasPrefix(path, "/api/"):
		// All HTTP /api/* routes are auth-gated at the proxy level and
		// forwarded to the autoputer with trusted user context injected. The
		// autoputer owns app route dispatch and route-specific 404s.
		h.HandleProtectedAPI(w, r)
		return
	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
}

// HandlePulseSummary returns public-safe aggregate launch usage and health
// facts. It is intentionally unauthenticated and must not expose raw user IDs,
// email lists, content, IPs, devices, referrers, or per-user timelines.
func (h *Handler) HandlePulseSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "pulse summary requires vmctl routing"})
		return
	}
	summary, err := h.vmctlClient.PulseSummaryContext(r.Context())
	if err != nil {
		log.Printf("proxy: pulse summary: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to load pulse summary"})
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// HandleWS handles GET /api/ws. It validates the access JWT cookie, denies
// requests with missing or invalid auth without upgrading the connection, and
// relays WebSocket frames bidirectionally between the client and the
// VM-backed autoputer. When vmctl routing is enabled, the WS dial targets the
// user's resolved VM (VAL-VM-006). The proxy injects the authenticated user
// context via the X-Authenticated-User header on the autoputer dial and strips
// any client-supplied identity headers.
func (h *Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
	stripClientSuppliedHeaders(r)
	started := time.Now()
	// Step 1: Validate auth BEFORE upgrading. Missing or invalid auth is
	// denied with a machine-readable 401 JSON response and no WS upgrade.
	authStarted := time.Now()
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		h.lifecycle.record("ws.auth", "unauthorized", time.Since(authStarted))
		h.lifecycle.record("ws.total", "unauthorized", time.Since(started))
		return
	}
	h.lifecycle.record("ws.auth", "ok", time.Since(authStarted))
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		h.lifecycle.record("ws.authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record("ws.total", "forbidden", time.Since(started))
		return
	}

	// Step 2: Resolve only the exact API-key-bound computer (VAL-VM-006).
	desktopID := requestDesktopID(r)
	computerTarget, ok := h.requireAPIKeyComputerTarget(w, r, authResult, "", desktopID)
	if !ok {
		h.lifecycle.record("ws.authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record("ws.total", "forbidden", time.Since(started))
		return
	}
	resolveStarted := time.Now()
	autoputerURL, err := h.resolveComputerURLForComputerTarget(r.Context(), authResult, computerTarget, desktopID)
	if err != nil {
		log.Printf("proxy WS: failed to resolve autoputer for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeResolveError(w, err)
		h.lifecycle.record("ws.resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record("ws.total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record("ws.resolve", "ok", time.Since(resolveStarted))

	// Step 3: Upgrade the client connection to WebSocket.
	clientConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade failed — nothing to relay. The upgrader already wrote
		// an HTTP error response.
		return
	}
	defer func() { _ = clientConn.Close() }()

	// Wrap the client connection in a wsWriter so every write to it is
	// serialized. The relay goroutine and the teardown close-frame write below
	// both write to clientConn; without serialization the race detector flags
	// the overlapping WriteMessage calls on gorilla/websocket's shared writer
	// state.
	clientW := &wsWriter{conn: clientConn}

	// Step 4: Dial the autoputer WebSocket endpoint.
	// Use the resolved autoputer URL instead of the static host fallback.
	autoputerWSURL := h.autoputerWSURLForTarget(autoputerURL, r.URL.RawQuery)
	autoputerHeader := http.Header{}
	// Inject the trusted user context; strip any client-supplied value.
	// The proxy is the trust boundary — only verified identity flows.
	autoputerHeader.Set("X-Authenticated-User", authResult.UserID)
	if authResult.Email != "" {
		autoputerHeader.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		autoputerHeader.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}

	dialStarted := time.Now()
	autoputerConn, _, err := h.dialer.Dial(autoputerWSURL, autoputerHeader)
	if err != nil {
		log.Printf("proxy WS: dial autoputer %s: %v", autoputerWSURL, err)
		// Close the client connection since we can't reach the autoputer.
		_ = clientW.writeMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "upstream unavailable"))
		h.lifecycle.record("ws.dial", "error", time.Since(dialStarted))
		h.lifecycle.record("ws.total", "dial_error", time.Since(started))
		return
	}
	h.lifecycle.record("ws.dial", "connected", time.Since(dialStarted))
	h.lifecycle.record("ws.total", "connected", time.Since(started))
	defer func() { _ = autoputerConn.Close() }()

	// Wrap the autoputer connection in its own wsWriter for the same reason as
	// clientW: the client->autoputer relay and the teardown close-frame write
	// both write to autoputerConn.
	autoputerW := &wsWriter{conn: autoputerConn}

	// Step 5: Relay frames bidirectionally until either side closes or errors.
	relayDone := make(chan struct{}, 2)

	// Client -> Autoputer relay.
	go func() {
		defer func() { relayDone <- struct{}{} }()
		h.relayFrames(clientConn, autoputerW, "client->autoputer")
	}()

	// Autoputer -> Client relay.
	go func() {
		defer func() { relayDone <- struct{}{} }()
		h.relayFrames(autoputerConn, clientW, "autoputer->client")
	}()

	// Wait for one direction to finish, then close both connections.
	<-relayDone

	// Send close messages to both sides to unblock the other relay goroutine.
	// These writes are serialized against the still-running relay goroutine's
	// writes via the per-connection wsWriter mutex.
	_ = clientW.writeMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = autoputerW.writeMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	// Wait briefly for the second goroutine to finish.
	<-relayDone
}

// autoputerWSURLForTarget derives the WebSocket URL for a specific autoputer URL.
func (h *Handler) autoputerWSURLForTarget(targetURL, rawQuery string) string {
	return autoputerWSURLForBase(targetURL, rawQuery)
}

// autoputerWSURLForBase derives the WebSocket URL from an HTTP base URL.
func autoputerWSURLForBase(baseURL, rawQuery string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "ws://127.0.0.1:8085/api/ws"
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u.Path = "/api/ws"
	u.RawQuery = rawQuery
	return u.String()
}

// vmctlOpContext detaches a vmctl lifecycle/resolve call from the inbound
// HTTP request cancel so a browser abort cannot cancel VM assignment. The
// operation is still bounded by the configured vmctl timeout.
func (h *Handler) vmctlOpContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := DefaultVmctlTimeout
	if h != nil && h.cfg != nil && h.cfg.VmctlTimeout > 0 {
		timeout = h.cfg.VmctlTimeout
	}
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

// resolveComputerURL resolves one immutable D-ROUTE slot before consulting
// vmctl for its current disposable realization. Missing route authority is a
// product-visible refusal; there is no static autoputer fallback.
func (h *Handler) resolveComputerURL(ctx context.Context, userID, desktopID string) (string, error) {
	if h.vmctlClient == nil {
		if h.cfg != nil && h.cfg.AllowDirectAutoputerForTests {
			return h.cfg.ComputerURL, nil
		}
		return "", fmt.Errorf("ComputerVersion route authority is not configured")
	}
	ctx, cancel := h.vmctlOpContext(ctx)
	defer cancel()
	// Serve from the short-TTL route cache when it has a resolved URL —
	// a hit skips both the immutable-route check and ResolveDesktopContext.
	// Stale entries self-expire; a transport failure against a cached URL
	// invalidates it via the reverse-proxy ErrorHandler.
	if cached, ok := h.routeCacheGet(userID, desktopID); ok && cached.computerURL != "" {
		return cached.computerURL, nil
	}

	start := time.Now()
	delay := autoputerResolveRetryBaseDelay
	for attempt := 0; ; attempt++ {
		autoputerURL, err := h.resolveComputerURLOnce(ctx, userID, desktopID)
		if err == nil {
			if attempt > 0 {
				log.Printf("proxy: resolved autoputer after transient vmctl error attempts=%d elapsed=%s", attempt+1, time.Since(start).Round(time.Millisecond))
			}
			return autoputerURL, nil
		}
		if !isTransientVMCTLResolveError(err) || time.Since(start) >= autoputerResolveRetryWindow {
			return "", err
		}
		if delay <= 0 {
			delay = time.Millisecond
		}
		if delay > autoputerResolveRetryMaxDelay {
			delay = autoputerResolveRetryMaxDelay
		}
		if time.Since(start)+delay > autoputerResolveRetryWindow {
			delay = autoputerResolveRetryWindow - time.Since(start)
			if delay <= 0 {
				return "", err
			}
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("resolve autoputer canceled after transient vmctl error: %w", err)
		case <-time.After(delay):
		}
		delay *= 2
	}
}

// routeCacheEntry is a short-TTL memo of one (user,desktop) resolution:
// computer_url is the resolved autoputer base URL, route_ok records that the
// immutable ComputerVersion route was confirmed inside the TTL window. Both
// are advisory only — vmctl stays the route of record and a transport
// failure against a cached URL invalidates the entry immediately.
type routeCacheEntry struct {
	computerURL string
	routeOK     bool
	expiresAt   time.Time
}

const defaultRouteCacheTTL = 2 * time.Second

func (h *Handler) routeCacheTTLFor() time.Duration {
	if h != nil && h.routeCacheTTL > 0 {
		return h.routeCacheTTL
	}
	return defaultRouteCacheTTL
}

func routeCacheKey(userID, desktopID string) string {
	return strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(desktopID)
}

func (h *Handler) routeCacheGet(userID, desktopID string) (routeCacheEntry, bool) {
	if h == nil {
		return routeCacheEntry{}, false
	}
	key := routeCacheKey(userID, desktopID)
	h.routeCacheMu.Lock()
	entry, ok := h.routeCache[key]
	h.routeCacheMu.Unlock()
	if !ok || !entry.expiresAt.After(time.Now()) {
		return routeCacheEntry{}, false
	}
	return entry, true
}

func (h *Handler) routeCachePut(userID, desktopID string, entry routeCacheEntry) {
	if h == nil {
		return
	}
	h.routeCacheMu.Lock()
	defer h.routeCacheMu.Unlock()
	if h.routeCache == nil {
		h.routeCache = make(map[string]routeCacheEntry)
	} else if len(h.routeCache) >= 1024 {
		h.routeCache = make(map[string]routeCacheEntry)
	}
	entry.expiresAt = time.Now().Add(h.routeCacheTTLFor())
	h.routeCache[routeCacheKey(userID, desktopID)] = entry
}

// invalidateRouteByURL drops every cached entry resolving to url (the
// transports see only the URL, not the key) so a VM that moved or wedged
// immediately re-resolves instead of serving a dead upstream for the TTL.
func (h *Handler) invalidateRouteByURL(url string) {
	if h == nil || url == "" {
		return
	}
	h.routeCacheMu.Lock()
	defer h.routeCacheMu.Unlock()
	for key, entry := range h.routeCache {
		if entry.computerURL == url {
			delete(h.routeCache, key)
		}
	}
}

func (h *Handler) ensureComputerVersionRoute(ctx context.Context, userID, desktopID string) error {
	if h.cfg != nil && h.cfg.AllowDirectAutoputerForTests {
		return nil
	}
	if h.vmctlClient == nil {
		return fmt.Errorf("ComputerVersion route authority is not configured")
	}
	if cached, ok := h.routeCacheGet(userID, desktopID); ok && cached.routeOK {
		return nil
	}
	prior, hadPrior := h.routeCacheGet(userID, desktopID)
	routeSlotID, err := routeledger.RouteSlotID(userID, desktopID)
	if err != nil {
		return err
	}
	resolution, err := h.vmctlClient.ResolveComputerVersionRouteOrAbsent(ctx, routeSlotID)
	if err != nil {
		return fmt.Errorf("resolve immutable ComputerVersion route %s: %w", routeSlotID, err)
	}
	if resolution.RouteAbsent &&
		userID == vmctl.UniversalWirePlatformOwnerID && desktopID == vmctl.UniversalWirePlatformDesktopID {
		return fmt.Errorf("platform computer requires an immutable ComputerVersion route")
	}
	if hadPrior {
		prior.routeOK = true
		h.routeCachePut(userID, desktopID, prior)
	} else {
		h.routeCachePut(userID, desktopID, routeCacheEntry{routeOK: true})
	}
	return nil
}

func (h *Handler) resolveComputerURLOnce(ctx context.Context, userID, desktopID string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	userID = strings.TrimSpace(userID)
	desktopID = strings.TrimSpace(desktopID)
	if desktopID == "" {
		desktopID = vmctl.PrimaryDesktopID
	}
	if desktopID != vmctl.PrimaryDesktopID &&
		!(userID == vmctl.UniversalWirePlatformOwnerID && desktopID == vmctl.UniversalWirePlatformDesktopID) {
		return "", fmt.Errorf("desktop %s is not a routable computer", desktopID)
	}
	if err := h.ensureComputerVersionRoute(ctx, userID, desktopID); err != nil {
		return "", err
	}
	resp, err := h.vmctlClient.ResolveDesktopContext(ctx, userID, desktopID)
	if err != nil {
		return "", err
	}
	h.routeCachePut(userID, desktopID, routeCacheEntry{computerURL: resp.ComputerURL, routeOK: true})
	return resp.ComputerURL, nil
}

func isTransientVMCTLResolveError(err error) bool {
	if err == nil {
		return false
	}
	// A durable recovery refusal is deterministic for its inputs: retrying it
	// inside the transient window is a request storm against blocked state.
	var refusal *vmctl.RecoveryRefusalError
	if errors.As(err, &refusal) {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, marker := range []string{
		"resolve call failed",
		"lookup call failed",
		"connect: connection refused",
		"connection reset by peer",
		"connection refused",
		"eof",
		"status 502",
		"status 503",
		"status 504",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// recoveryRefusalFromError extracts a typed vmctl recovery refusal.
func recoveryRefusalFromError(err error) *vmctl.RecoveryRefusalError {
	if err == nil {
		return nil
	}
	var refusal *vmctl.RecoveryRefusalError
	if errors.As(err, &refusal) {
		return refusal
	}
	return nil
}

// writeRecoveryRefusalResponse renders the durable recovery condition as the
// product-visible structured 503: reason, kind, repair job status, and
// Retry-After. It is deterministic; callers must not retry it as transient.
func writeRecoveryRefusalResponse(w http.ResponseWriter, refusal *vmctl.RecoveryRefusalError) {
	if refusal == nil {
		return
	}
	retryAfter := refusal.RetryAfterSeconds
	if retryAfter <= 0 {
		retryAfter = 60
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeJSON(w, http.StatusServiceUnavailable, errorResponse{
		Error:             "computer recovery blocked",
		Reason:            refusal.Reason,
		Kind:              refusal.Kind,
		RetryAfterSeconds: retryAfter,
		Repair:            refusal.Repair,
	})
}

// isResolveTimeoutError reports whether an error from resolveComputerURL is a
// timeout (vmctl client deadline, context deadline, or URL-level timeout).
// This matters for the public response code: timeouts are 504 Gateway Timeout,
// other resolve failures are 502 Bad Gateway.
func isResolveTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return uerr.Timeout()
	}
	return false
}

// writeResolveError writes the appropriate JSON error response for a autoputer
// resolve failure. A durable recovery refusal is a structured 503 with
// Retry-After; timeouts are 504 Gateway Timeout; other failures are 502.
func writeResolveError(w http.ResponseWriter, err error) {
	if refusal := recoveryRefusalFromError(err); refusal != nil {
		writeRecoveryRefusalResponse(w, refusal)
		return
	}
	status := http.StatusBadGateway
	if isResolveTimeoutError(err) {
		status = http.StatusGatewayTimeout
	}
	writeJSON(w, status, errorResponse{Error: "failed to resolve user autoputer"})
}

// writeAutoputerResolveFailure renders a resolve failure for callers with
// their own message: a durable recovery refusal still becomes the structured
// 503 with Retry-After; everything else keeps the caller's status and text.
func writeAutoputerResolveFailure(w http.ResponseWriter, status int, message string, err error) {
	if refusal := recoveryRefusalFromError(err); refusal != nil {
		writeRecoveryRefusalResponse(w, refusal)
		return
	}
	if status >= 500 && err != nil {
		log.Printf("proxy: %s: %v", message, err)
	}
	writeJSON(w, status, errorResponse{Error: message})
}

// logComputerAuthorityFailure records why a request was answered "computer
// authority unavailable": the computer, the route, the realization's state
// and the cause, never a credential or the guest's address (failure 14,
// docs/problems/engineering-completion-path-cluster-2026-10-20.md).
func logComputerAuthorityFailure(r *http.Request, computerID string, target *resolvedComputerTarget, err error) {
	route := ""
	if r != nil && r.URL != nil {
		route = r.URL.Path
	}
	state, hasURL := "", false
	if target != nil {
		state = target.State
		hasURL = strings.TrimSpace(target.ComputerURL) != ""
	}
	cause := "none"
	if err != nil {
		cause = err.Error()
	}
	log.Printf("proxy: computer authority unavailable: computer=%s route=%s state=%q has_url=%t cause=%s", computerID, route, state, hasURL, cause)
}

// setResolvedRouteContext stashes the resolved upstream URL and the route
// invalidator on the request context so the reverse-proxy ErrorHandler can
// drop a stale cache entry on transport failure even after the Director has
// stripped the X-Resolved-Autoputer-URL header.
func (h *Handler) setResolvedRouteContext(r *http.Request, resolvedURL string) {
	if r == nil || resolvedURL == "" {
		return
	}
	ctx := r.Context()
	ctx = context.WithValue(ctx, resolvedAutoputerURLContextKey, resolvedURL)
	ctx = context.WithValue(ctx, routeInvalidatorContextKey, h.invalidateRouteByURL)
	*r = *r.WithContext(ctx)
}

// wsWriter serializes writes to a single websocket.Conn. gorilla/websocket
// connections are NOT safe for concurrent use: WriteMessage mutates shared
// internal messageWriter state (the conn's writer/pos fields). In the relay
// loop one goroutine writes data frames to dst while the main handler
// goroutine writes a CloseMessage to the same dst to tear down the relay, and
// the race detector flags the overlapping WriteMessage calls. The mutex here
// serializes every write to a given connection so the relay goroutine and the
// teardown close-frame write never touch the conn's writer state at the same
// time. Reads remain independent of writes per gorilla's contract, so
// ReadMessage on the underlying conn is still called directly.
type wsWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// writeMessage writes a single WebSocket message under the connection's
// write lock, preventing concurrent writes from corrupting message framing.
func (w *wsWriter) writeMessage(messageType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(messageType, data)
}

// relayFrames copies WebSocket messages from src to dst until an error occurs
// or the connection is closed. It preserves the message type (text or binary).
// dst.writeMessage serializes writes so the relay never races the teardown
// close-frame write issued by the caller on the same destination connection.
func (h *Handler) relayFrames(src *websocket.Conn, dst *wsWriter, direction string) {
	for {
		mt, msg, err := src.ReadMessage()
		if err != nil {
			// Normal close or expected error — stop relaying silently.
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			// Abnormal closure or EOF is normal teardown when the other side
			// drops; no need to log noisily.
			if errors.Is(err, io.EOF) || websocket.IsCloseError(err, websocket.CloseAbnormalClosure) {
				return
			}
			// Unexpected errors are worth logging for debugging.
			log.Printf("proxy WS relay %s: read error: %v", direction, err)
			return
		}
		if err := dst.writeMessage(mt, msg); err != nil {
			// Write error means the other side is gone; stop relaying silently.
			return
		}
	}
}

// HandleHealth handles GET /health for the proxy service. It checks the
// upstream autoputer reachability in addition to the proxy's own health,
// making the protected-request backend health observable (VAL-DEPLOY-008).
// The response includes:
//   - status: "ok" when the proxy and upstream are healthy, "degraded" when
//     the proxy is up but the upstream is unreachable
//   - upstream: "ok" or "unreachable"
//   - vmctl_routing: "enabled" or omitted when using static routing
//
// When vmctl routing is enabled, the static autoputer upstream probe is
// skipped (the host autoputer was deleted in PR 5; per-user VMs are the
// real upstream and vmctl health is the authoritative signal).
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	status := "ok"
	upstreamStatus := "ok"
	var upstreamBuild *buildinfo.Info

	if h.cfg.VmctlRoutingEnabled() {
		// vmctl routing: vmctl health is the authoritative upstream signal.
		// The static autoputer URL probe is skipped because the host autoputer
		// was deleted (PR 5) and per-user VMs are the real upstream.
		upstreamStatus = "vmctl"
	} else {
		// Static routing fallback: probe the configured autoputer URL.
		upstreamHealthy, build := h.probeUpstreamHealth()
		upstreamBuild = build
		if !upstreamHealthy {
			upstreamStatus = "unreachable"
			status = "degraded"
		}
	}

	resp := proxyHealthResponse{
		Status:        status,
		Service:       "proxy",
		Upstream:      upstreamStatus,
		Lifecycle:     h.lifecycle.summary(),
		Build:         buildinfo.Snapshot("proxy"),
		UpstreamBuild: upstreamBuild,
	}
	if platform := buildinfo.PlatformSnapshot("proxy"); platform.DeployedCommit != "" {
		resp.Platform = &platform
	}

	// Report vmctl routing status (VAL-VM-002).
	if h.cfg.VmctlRoutingEnabled() {
		resp.VMctlRouting = "enabled"
		if vmctlHealth, ok := h.probeVMctlHealth(); ok {
			resp.VMctlStatus = vmctlHealth.Status
			if vmctlHealth.Status != "ok" {
				resp.Status = "degraded"
			}
		} else {
			resp.VMctlStatus = "unavailable"
			resp.Status = "degraded"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) probeVMctlHealth() (*proxyVMctlHealthSummary, bool) {
	if h == nil || h.cfg == nil || strings.TrimSpace(h.cfg.VmctlURL) == "" {
		return nil, false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(h.cfg.VmctlURL, "/")+"/health", nil)
	if err != nil {
		return nil, false
	}
	hostauth.MarkInternal(req)
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	var body proxyVMctlHealthSummary
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, false
	}
	return &body, true
}

// probeUpstreamHealth probes the upstream autoputer's /health endpoint with a
// short timeout. When available, it also surfaces the autoputer build identity so
// deployed browsers can prove they are talking to the expected backend build.
func (h *Handler) probeUpstreamHealth() (bool, *buildinfo.Info) {
	client := &http.Client{Timeout: 2 * time.Second}
	url := h.autoputerURL.String() + "/health"
	resp, err := client.Get(url)
	if err != nil {
		return false, nil
	}
	defer func() { _ = resp.Body.Close() }()
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300

	var body struct {
		Build buildinfo.Info `json:"build"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return healthy, nil
	}
	if body.Build.Commit == "" {
		return healthy, nil
	}
	return healthy, &body.Build
}

// HandleProviderDeny denies all browser requests to /provider/* routes.
// Provider routes are only reachable via internal service-to-service
// communication (gateway). Browser callers must never use /provider/*
// as a raw inference bypass (VAL-GATEWAY-002).
func (h *Handler) HandleProviderDeny(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusForbidden, errorResponse{
		Error: "provider routes are not available to browser callers",
	})
}

// HandleVMctlDeny denies all browser requests to /internal/vmctl/* routes.
// VM control endpoints are internal-only and must not be exposed publicly
// (VAL-VM-012).
func (h *Handler) HandleVMctlDeny(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusForbidden, errorResponse{
		Error: "vmctl control endpoints are not publicly accessible",
	})
}

// HandlePlatformObjectGraph forwards object graph API requests from the
// runtime (inside a VM) to corpusd. Guest requests are bound by their tap
// source address to the owning computer before the proxy becomes corpusd's
// loopback internal caller.
func (h *Handler) HandlePlatformObjectGraph(w http.ResponseWriter, r *http.Request) {
	if err := h.requirePlatformObjectGraphCaller(r); err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: err.Error()})
		return
	}
	target, err := joinBasePath(h.cfg.CorpusdURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build corpusd URL"})
		return
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	httpReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build corpusd request"})
		return
	}
	// The proxy is the trusted internal caller to corpusd. Strip any
	// caller-supplied X-Internal-Caller header and set it ourselves so a
	// VM process cannot forge internal-caller privileges.
	hostauth.MarkInternal(httpReq)
	if v := r.Header.Get("Content-Type"); v != "" {
		httpReq.Header.Set("Content-Type", v)
	}

	resp, err := h.corpusd.Do(httpReq)
	if err != nil {
		log.Printf("proxy: corpusd objectgraph: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to reach corpusd"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("proxy: corpusd objectgraph copy: %v", err)
	}
}

// RegisterRoutes registers all proxy routes on the given server.
// The proxy /health handler is registered via SetHealthHandler to
// override the default server health handler with one that reports
// upstream autoputer reachability.
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/api/shell/bootstrap", h.HandleBootstrap)
	s.HandleFunc("/api/ws", h.HandleWS)
	s.HandleFunc("/api/", h.HandleAPI)
	// VAL-GATEWAY-002: Deny all browser access to /provider/* routes.
	// The gateway is the only component authorized to call upstream
	// providers; browser callers must never bypass the runtime/proxy
	// boundary to invoke inference directly.
	s.HandleFunc("/provider/", h.HandleProviderDeny)
	// VAL-VM-012: Deny all browser access to /internal/vmctl/* routes.
	// vmctl control endpoints are internal-only; they must not be
	// exposed as public browser-facing routes.
	s.HandleFunc("/internal/vmctl/", h.HandleVMctlDeny)
	// Object graph API: forward runtime objectgraph HTTPStore calls to
	// corpusd. The runtime (inside a VM) reaches corpusd through the
	// proxy; these routes pass through /internal/platform/objects and
	// /internal/platform/edges to corpusd's object graph handler.
	s.HandleFunc("/internal/platform/objects", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/objects/", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/edges", h.HandlePlatformObjectGraph)
	// Autopaper captures (W1b2): a guest writes one poll's captures and reads
	// shared capture bodies through the same bound-guest pass-through.
	s.HandleFunc("/internal/platform/captures", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/captures/bodies/", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/captures/", h.HandlePlatformObjectGraph)
	// Research's read-only search over published stories and opinion pieces
	// (guest_authority.go carves out this one GET path).
	s.HandleFunc("/internal/platform/published/search", h.HandlePlatformObjectGraph)
	// Living stories (Cut 2 2b): a guest publishes its own story revisions
	// through the owner's publish path, and writes its stories' support
	// verdicts to Store B, both bound to the guest's owner and computer.
	s.HandleFunc(types.StoryPublishPath, h.HandleGuestTexturePublication)
	s.HandleFunc(types.StoryVerdictsPath, h.HandlePlatformObjectGraph)
	s.HandleFunc(types.StoryRelationsPath, h.HandlePlatformObjectGraph)
	s.HandleFunc("/assets/", h.HandleComputerSurface)
	s.HandleFunc("/", h.HandleComputerSurface)
}
