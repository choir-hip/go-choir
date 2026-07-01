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
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// clientIdentityHeaders is the list of HTTP headers that must be stripped from
// client requests before forwarding to the sandbox. These headers could be used
// to impersonate or spoof user identity, so the proxy removes them all and
// only injects the JWT-verified user context via X-Authenticated-User.
var clientIdentityHeaders = []string{
	"X-Authenticated-User",
	"X-Authenticated-Email",
	"X-Authenticated-Scopes",
	"X-User-Id",
	"X-User-Name",
	"X-Forwarded-User",
	"X-Remote-User",
	"X-Auth-User",
	"X-Internal-Caller",
}

var (
	sandboxResolveRetryWindow    = 10 * time.Second
	sandboxResolveRetryBaseDelay = 200 * time.Millisecond
	sandboxResolveRetryMaxDelay  = time.Second
)

// errorResponse is a generic JSON error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// proxyHealthResponse is the JSON structure returned by the proxy /health
// endpoint. It intentionally exposes only coarse service status and deployed
// build identity; host pressure, global VM inventory, vmctl URLs, and raw VM
// handles must remain off browser-public surfaces.
type proxyHealthResponse struct {
	Status        string                 `json:"status"`
	Service       string                 `json:"service"`
	Upstream      string                 `json:"upstream"`
	VMctlRouting  string                 `json:"vmctl_routing,omitempty"`
	VMctlStatus   string                 `json:"vmctl_status,omitempty"`
	Lifecycle     lifecycleHealthSummary `json:"lifecycle,omitempty"`
	Build         buildinfo.Info         `json:"build"`
	UpstreamBuild *buildinfo.Info        `json:"upstream_build,omitempty"`
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
	AuthMethod string   // "cookie" or "api_key"
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
	cfg             *Config
	pubKey          ed25519.PublicKey
	reverseProxy    *httputil.ReverseProxy
	upgrader        websocket.Upgrader
	dialer          *websocket.Dialer
	corpusd       *http.Client
	maild           *http.Client
	sandboxHTTP     *http.Client
	sandboxURL      *url.URL      // parsed sandbox URL for WS dial derivation
	vmctlClient     *vmctl.Client // optional vmctl client for VM-backed routing
	lifecycle       *lifecycleRecorder
	recoveries      *computeRecoveryTracker
	apiKeyValidator APIKeyValidator // optional: enables Bearer token (API key) auth
	authStore       *auth.Store     // optional: owned auth store for API key validation
}

// NewHandler creates a proxy Handler with the given config and auth public key.
// It initializes the reverse proxy pointing at the configured sandbox URL and
// the WebSocket upgrader/dialer for live-channel proxying. If vmctl routing
// is configured (cfg.VmctlURL != ""), the handler resolves user VM ownership
// through vmctl instead of falling back to the static host sandbox URL
// (VAL-VM-001, VAL-VM-002).
func NewHandler(cfg *Config, pubKey ed25519.PublicKey) (*Handler, error) {
	if strings.TrimSpace(cfg.CorpusdURL) == "" {
		cfg.CorpusdURL = DefaultCorpusdURL
	}
	if strings.TrimSpace(cfg.MaildURL) == "" {
		cfg.MaildURL = DefaultMaildURL
	}
	sandboxURL, err := url.Parse(cfg.SandboxURL)
	if err != nil {
		return nil, fmt.Errorf("parse sandbox URL %s: %w", cfg.SandboxURL, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(sandboxURL)

	// Flush immediately for SSE streaming responses (for example Trace) and
	// other streaming endpoints. A value of -1 means flush after each
	// write to the client, which ensures SSE events arrive incrementally
	// rather than being buffered (VAL-RUNTIME-005).
	proxy.FlushInterval = -1

	// Customize the director to preserve the original request path and query
	// without rewriting. The default NewSingleHostReverseProxy director
	// replaces the path, but we want the sandbox to receive the same public
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
		req.URL.Path = req.Header.Get("X-Original-Path")
		req.URL.RawPath = ""
		req.URL.RawQuery = req.Header.Get("X-Original-RawQuery")

		// Check if vmctl resolved a per-user VM sandbox URL. When set,
		// override the target to the resolved VM (VAL-VM-001, VAL-VM-002).
		if resolved := req.Header.Get("X-Resolved-Sandbox-URL"); resolved != "" {
			if resolvedURL, err := url.Parse(resolved); err == nil {
				req.URL.Scheme = resolvedURL.Scheme
				req.URL.Host = resolvedURL.Host
				req.Host = resolvedURL.Host
			}
		} else {
			// Set the Host header to the default sandbox host.
			req.Host = sandboxURL.Host
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
		req.Header.Del("X-Original-Path")
		req.Header.Del("X-Original-RawQuery")
		req.Header.Del("X-Resolved-Sandbox-URL")
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

	// Build the handler. When authStore is nil, apiKeyValidator must be a
	// nil interface (not a typed-nil *auth.Store) so the nil check in
	// validateAPIKey works correctly.
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
		dialer:      websocket.DefaultDialer,
		corpusd:   &http.Client{Timeout: 30 * time.Second},
		maild:       &http.Client{Timeout: 30 * time.Second},
		sandboxHTTP: &http.Client{Timeout: 30 * time.Second},
		sandboxURL:  sandboxURL,
		vmctlClient: vmctlCli,
		lifecycle:   newLifecycleRecorder(),
		recoveries:  newComputeRecoveryTracker(),
		authStore:   authStore,
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

	return &AuthResult{UserID: userID, Email: strings.TrimSpace(email), Valid: true, AuthMethod: "cookie"}, nil
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
		AuthMethod: "api_key",
	}, nil
}

// HandleBootstrap handles GET /api/shell/bootstrap.
// It validates the access JWT cookie, denies requests with missing or invalid
// auth, and forwards authenticated requests to the sandbox upstream.
// When vmctl routing is enabled, resolves through VM ownership instead of
// the static sandbox fallback (VAL-VM-001, VAL-VM-002).
// The proxy injects the authenticated user context via the
// X-Authenticated-User header and preserves the original request path, method,
// query string, and upstream status/body.
func (h *Handler) HandleBootstrap(w http.ResponseWriter, r *http.Request) {
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

	// Resolve the sandbox URL for this user.
	desktopID := requestDesktopID(r)
	resolveStarted := time.Now()
	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		log.Printf("proxy: failed to resolve sandbox for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		h.lifecycle.record("bootstrap.resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record("bootstrap.total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record("bootstrap.resolve", "ok", time.Since(resolveStarted))

	// Auth is valid. Store the trusted user context for the director to inject.
	h.setTrustedAuthHeaders(r, authResult)

	// Preserve the original path and query for the director to use.
	r.Header.Set("X-Original-Path", r.URL.Path)
	r.Header.Set("X-Original-RawQuery", r.URL.RawQuery)

	// If vmctl resolved a different URL, override the reverse proxy target.
	if sandboxURL != h.cfg.SandboxURL {
		r.Header.Set("X-Resolved-Sandbox-URL", sandboxURL)
	}

	upstreamStarted := time.Now()
	recorder := &lifecycleStatusRecorder{ResponseWriter: w}
	h.reverseProxy.ServeHTTP(recorder, r)
	h.lifecycle.record("bootstrap.upstream", lifecycleHTTPStatus(recorder.status), time.Since(upstreamStarted))
	h.lifecycle.record("bootstrap.total", lifecycleHTTPStatus(recorder.status), time.Since(started))
}

// HandleProtectedAPI is a generic handler for /api/* routes that require auth.
// It validates the access JWT and forwards authenticated requests to the
// sandbox. When vmctl routing is enabled, it resolves the user's VM through
// vmctl ownership instead of using the static sandbox URL (VAL-VM-001,
// VAL-VM-002).
func (h *Handler) HandleProtectedAPI(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	stagePrefix := "api"
	if r != nil && r.URL != nil && r.URL.Path == "/api/prompt-bar" {
		stagePrefix = "prompt_bar"
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

	// Resolve the sandbox URL for this user. Universal Wire stories read the
	// platform computer's embedded store, not the caller's personal computer.
	desktopID := requestDesktopID(r)
	resolveOwnerID, resolveDesktopID := protectedAPIResolveTarget(r, authResult.UserID, desktopID)
	resolveStarted := time.Now()
	sandboxURL, err := h.resolveSandboxURL(r.Context(), resolveOwnerID, resolveDesktopID)
	if err != nil {
		log.Printf("proxy: failed to resolve sandbox for owner %s desktop %s (caller %s): %v", resolveOwnerID, resolveDesktopID, authResult.UserID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		h.lifecycle.record(stagePrefix+".resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record(stagePrefix+".total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record(stagePrefix+".resolve", "ok", time.Since(resolveStarted))

	// Auth is valid. Store the trusted user context for the director.
	h.setTrustedAuthHeaders(r, authResult)
	r.Header.Set("X-Original-Path", r.URL.Path)
	r.Header.Set("X-Original-RawQuery", r.URL.RawQuery)

	// If vmctl resolved a different URL, override the reverse proxy target.
	if sandboxURL != h.cfg.SandboxURL {
		r.Header.Set("X-Resolved-Sandbox-URL", sandboxURL)
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
// /api/* requests are forwarded by default so new sandbox apps do not require
// proxy allowlist changes. Signed-out callers still see a 401 denial rather
// than a route-specific 404 that might suggest which routes exist.
func (h *Handler) HandleAPI(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Route protocol-specific protected paths.
	switch {
	case path == "/api/shell/bootstrap":
		h.HandleBootstrap(w, r)
		return
	case path == "/api/compute/status":
		h.HandleComputeStatus(w, r)
		return
	case path == "/api/compute/recovery":
		h.HandleComputeRecovery(w, r)
		return
	case path == "/api/pulse/summary":
		h.HandlePulseSummary(w, r)
		return
	case path == "/api/app-change-packages/pull":
		h.HandleAppChangePackagePull(w, r)
		return
	case isAppChangePackageReviewEvidencePath(path):
		h.HandleAppChangePackageReviewEvidence(w, r)
		return
	case strings.HasPrefix(path, "/api/system/"):
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	case path == "/api/ws":
		h.HandleWS(w, r)
		return
	case path == "/api/super-console/ws":
		h.HandleSuperConsoleWS(w, r)
	case path == "/api/terminal/ws":
		writeJSON(w, http.StatusGone, errorResponse{Error: "terminal app has been replaced by Super Console"})
		return
	case path == "/api/platform/texture/publications":
		h.HandleTexturePublication(w, r)
		return
	case path == "/api/platform/publications/resolve":
		h.HandlePlatformPublicationResolve(w, r)
		return
	case path == "/api/platform/publications/export":
		h.HandlePlatformPublicationExport(w, r)
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
	case isPlatformTextureReadRequest(r):
		h.HandlePlatformTextureRead(w, r)
		return
	case strings.HasPrefix(path, "/api/"):
		// All HTTP /api/* routes are auth-gated at the proxy level and
		// forwarded to the sandbox with trusted user context injected. The
		// sandbox owns app route dispatch and route-specific 404s.
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
// VM-backed sandbox. When vmctl routing is enabled, the WS dial targets the
// user's resolved VM (VAL-VM-006). The proxy injects the authenticated user
// context via the X-Authenticated-User header on the sandbox dial and strips
// any client-supplied identity headers.
func (h *Handler) HandleWS(w http.ResponseWriter, r *http.Request) {
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

	// Step 2: Resolve the sandbox URL for this user (VAL-VM-006).
	desktopID := requestDesktopID(r)
	resolveStarted := time.Now()
	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		log.Printf("proxy WS: failed to resolve sandbox for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
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

	// Step 4: Dial the sandbox WebSocket endpoint.
	// Use the resolved sandbox URL instead of the static host fallback.
	sandboxWSURL := h.sandboxWSURLForTarget(sandboxURL, r.URL.RawQuery)
	sandboxHeader := http.Header{}
	// Inject the trusted user context; strip any client-supplied value.
	// The proxy is the trust boundary — only verified identity flows.
	sandboxHeader.Set("X-Authenticated-User", authResult.UserID)
	if authResult.Email != "" {
		sandboxHeader.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		sandboxHeader.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}

	dialStarted := time.Now()
	sandboxConn, _, err := h.dialer.Dial(sandboxWSURL, sandboxHeader)
	if err != nil {
		log.Printf("proxy WS: dial sandbox %s: %v", sandboxWSURL, err)
		// Close the client connection since we can't reach the sandbox.
		_ = clientW.writeMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "upstream unavailable"))
		h.lifecycle.record("ws.dial", "error", time.Since(dialStarted))
		h.lifecycle.record("ws.total", "dial_error", time.Since(started))
		return
	}
	h.lifecycle.record("ws.dial", "connected", time.Since(dialStarted))
	h.lifecycle.record("ws.total", "connected", time.Since(started))
	defer func() { _ = sandboxConn.Close() }()

	// Wrap the sandbox connection in its own wsWriter for the same reason as
	// clientW: the client->sandbox relay and the teardown close-frame write
	// both write to sandboxConn.
	sandboxW := &wsWriter{conn: sandboxConn}

	// Step 5: Relay frames bidirectionally until either side closes or errors.
	relayDone := make(chan struct{}, 2)

	// Client -> Sandbox relay.
	go func() {
		defer func() { relayDone <- struct{}{} }()
		h.relayFrames(clientConn, sandboxW, "client->sandbox")
	}()

	// Sandbox -> Client relay.
	go func() {
		defer func() { relayDone <- struct{}{} }()
		h.relayFrames(sandboxConn, clientW, "sandbox->client")
	}()

	// Wait for one direction to finish, then close both connections.
	<-relayDone

	// Send close messages to both sides to unblock the other relay goroutine.
	// These writes are serialized against the still-running relay goroutine's
	// writes via the per-connection wsWriter mutex.
	_ = clientW.writeMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = sandboxW.writeMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	// Wait briefly for the second goroutine to finish.
	<-relayDone
}

// sandboxWSURL derives the WebSocket URL for the sandbox /api/ws endpoint
// from the configured HTTP sandbox URL (static fallback).
// nolint:unused
func (h *Handler) sandboxWSURL() string {
	return sandboxWSURLForBase(h.sandboxURL.String(), "")
}

// HandleSuperConsoleWS handles GET /api/super-console/ws. It validates the access JWT
// cookie, denies requests with missing or invalid auth without upgrading, and
// relays WebSocket frames bidirectionally between the client and the sandbox
// singleton zot PTY endpoint. This allows the browser to connect to zot through
// the auth-gated proxy without exposing a raw terminal app.
func (h *Handler) HandleSuperConsoleWS(w http.ResponseWriter, r *http.Request) {
	// Step 1: Validate auth BEFORE upgrading.
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}

	// Step 2: Resolve the sandbox URL for this user.
	desktopID := requestDesktopID(r)
	sandboxURL, err := h.resolveSandboxURL(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		log.Printf("proxy super console WS: failed to resolve sandbox for user %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to resolve user sandbox"})
		return
	}

	// Step 3: Upgrade the client connection to WebSocket.
	clientConn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() { _ = clientConn.Close() }()

	// Wrap the client connection in a wsWriter so every write to it is
	// serialized. See HandleWS for the race explanation: the relay goroutine
	// and the teardown close-frame write both write to clientConn, and
	// gorilla/websocket connections are not safe for concurrent writes.
	clientW := &wsWriter{conn: clientConn}

	// Step 4: Dial the sandbox terminal WebSocket endpoint.
	terminalWSURL := h.superConsoleWSURLForTarget(sandboxURL, r.URL.RawQuery)
	sandboxHeader := http.Header{}
	sandboxHeader.Set("X-Authenticated-User", authResult.UserID)
	if authResult.Email != "" {
		sandboxHeader.Set("X-Authenticated-Email", authResult.Email)
	}
	if len(authResult.Scopes) > 0 {
		sandboxHeader.Set("X-Authenticated-Scopes", strings.Join(authResult.Scopes, ","))
	}

	sandboxConn, _, err := h.dialer.Dial(terminalWSURL, sandboxHeader)
	if err != nil {
		log.Printf("proxy super console WS: dial sandbox %s: %v", terminalWSURL, err)
		_ = clientW.writeMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "upstream unavailable"))
		return
	}
	defer func() { _ = sandboxConn.Close() }()

	// Wrap the sandbox connection in its own wsWriter; the client->sandbox
	// relay and the teardown close-frame write both write to sandboxConn.
	sandboxW := &wsWriter{conn: sandboxConn}

	// Step 5: Relay frames bidirectionally until either side closes or errors.
	relayDone := make(chan struct{}, 2)

	go func() {
		defer func() { relayDone <- struct{}{} }()
		h.relayFrames(clientConn, sandboxW, "client->super-console")
	}()

	go func() {
		defer func() { relayDone <- struct{}{} }()
		h.relayFrames(sandboxConn, clientW, "super-console->client")
	}()

	<-relayDone

	// Serialized against the still-running relay goroutine via the
	// per-connection wsWriter mutex.
	_ = clientW.writeMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = sandboxW.writeMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	<-relayDone
}

// superConsoleWSURLForTarget derives the Super Console WebSocket URL for a specific
// sandbox target URL.
func (h *Handler) superConsoleWSURLForTarget(targetURL, rawQuery string) string {
	u, err := url.Parse(targetURL)
	if err != nil {
		return "ws://127.0.0.1:8085/api/super-console/ws"
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	u.Path = "/api/super-console/ws"
	u.RawQuery = rawQuery
	return u.String()
}

// sandboxWSURLForTarget derives the WebSocket URL for a specific sandbox URL.
func (h *Handler) sandboxWSURLForTarget(targetURL, rawQuery string) string {
	return sandboxWSURLForBase(targetURL, rawQuery)
}

// sandboxWSURLForBase derives the WebSocket URL from an HTTP base URL.
func sandboxWSURLForBase(baseURL, rawQuery string) string {
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

// protectedAPIResolveTarget chooses which computer sandbox should serve an
// authenticated /api/* request. Universal Wire edition state lives on the
// always-on platform computer.
func protectedAPIResolveTarget(r *http.Request, userID, desktopID string) (ownerID, resolvedDesktopID string) {
	if r == nil || r.URL == nil {
		return userID, desktopID
	}
	path := r.URL.Path
	if path == "/api/universal-wire/stories" {
		return vmctl.UniversalWirePlatformOwnerID, vmctl.UniversalWirePlatformDesktopID
	}
	return userID, desktopID
}

// resolveSandboxURL resolves the sandbox URL for an authenticated user.
// It consults the vmctl ownership registry to route the user to their
// assigned VM (VAL-VM-001). When vmctl is not configured, it falls back
// to the static SandboxURL — but with the host sandbox deleted (PR 5),
// this fallback will fail with a visible connection error (I3: no silent
// failures).
func (h *Handler) resolveSandboxURL(ctx context.Context, userID, desktopID string) (string, error) {
	if h.vmctlClient == nil {
		return h.cfg.SandboxURL, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	delay := sandboxResolveRetryBaseDelay
	for attempt := 0; ; attempt++ {
		sandboxURL, err := h.resolveSandboxURLOnce(ctx, userID, desktopID)
		if err == nil {
			if attempt > 0 {
				log.Printf("proxy: resolved sandbox after transient vmctl error attempts=%d elapsed=%s", attempt+1, time.Since(start).Round(time.Millisecond))
			}
			return sandboxURL, nil
		}
		if !isTransientVMCTLResolveError(err) || time.Since(start) >= sandboxResolveRetryWindow {
			return "", err
		}
		if delay <= 0 {
			delay = time.Millisecond
		}
		if delay > sandboxResolveRetryMaxDelay {
			delay = sandboxResolveRetryMaxDelay
		}
		if time.Since(start)+delay > sandboxResolveRetryWindow {
			delay = sandboxResolveRetryWindow - time.Since(start)
			if delay <= 0 {
				return "", err
			}
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("resolve sandbox canceled after transient vmctl error: %w", err)
		case <-time.After(delay):
		}
		delay *= 2
	}
}

func (h *Handler) resolveSandboxURLOnce(ctx context.Context, userID, desktopID string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	desktopID = strings.TrimSpace(desktopID)
	if desktopID == "" {
		desktopID = vmctl.PrimaryDesktopID
	}

	if desktopID == vmctl.PrimaryDesktopID {
		resp, err := h.vmctlClient.ResolveDesktopContext(ctx, userID, desktopID)
		if err != nil {
			return "", err
		}
		if !resp.Published {
			return "", fmt.Errorf("desktop %s is not published", desktopID)
		}
		return resp.SandboxURL, nil
	}

	if userID == vmctl.UniversalWirePlatformOwnerID && desktopID == vmctl.UniversalWirePlatformDesktopID {
		resp, err := h.vmctlClient.ResolveDesktopContext(ctx, userID, desktopID)
		if err != nil {
			return "", err
		}
		if !resp.Published {
			return "", fmt.Errorf("desktop %s is not published", desktopID)
		}
		return resp.SandboxURL, nil
	}

	resp, err := h.vmctlClient.LookupDesktopContext(ctx, userID, desktopID)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("desktop %s is not published", desktopID)
	}
	if !resp.Published {
		return "", fmt.Errorf("desktop %s is not published", desktopID)
	}

	return resp.SandboxURL, nil
}

func isTransientVMCTLResolveError(err error) bool {
	if err == nil {
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
// upstream sandbox reachability in addition to the proxy's own health,
// making the protected-request backend health observable (VAL-DEPLOY-008).
// The response includes:
//   - status: "ok" when the proxy and upstream are healthy, "degraded" when
//     the proxy is up but the upstream is unreachable
//   - upstream: "ok" or "unreachable"
//   - vmctl_routing: "enabled" or omitted when using static routing
//
// When vmctl routing is enabled, the static sandbox upstream probe is
// skipped (the host sandbox was deleted in PR 5; per-user VMs are the
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
		// The static sandbox URL probe is skipped because the host sandbox
		// was deleted (PR 5) and per-user VMs are the real upstream.
		upstreamStatus = "vmctl"
	} else {
		// Static routing fallback: probe the configured sandbox URL.
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
	req.Header.Set("X-Internal-Caller", "true")
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

// probeUpstreamHealth probes the upstream sandbox's /health endpoint with a
// short timeout. When available, it also surfaces the sandbox build identity so
// deployed browsers can prove they are talking to the expected backend build.
func (h *Handler) probeUpstreamHealth() (bool, *buildinfo.Info) {
	client := &http.Client{Timeout: 2 * time.Second}
	url := h.sandboxURL.String() + "/health"
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

// RegisterRoutes registers all proxy routes on the given server.
// The proxy /health handler is registered via SetHealthHandler to
// override the default server health handler with one that reports
// upstream sandbox reachability.
// HandlePlatformTextureRead serves published Texture document and revision reads
// from corpusd's DoltDB for Universal Wire articles. Published articles
// carry their full revision history in corpusd, not the platform sandbox.
func (h *Handler) HandlePlatformTextureRead(w http.ResponseWriter, r *http.Request) {
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}

	path := r.URL.Path
	var corpusdPath string
	switch {
	case strings.HasPrefix(path, "/api/texture/documents/") && strings.HasSuffix(path, "/revisions"):
		docID := strings.TrimPrefix(path, "/api/texture/documents/")
		docID = strings.TrimSuffix(docID, "/revisions")
		corpusdPath = "/internal/platform/texture/documents/" + url.PathEscape(docID) + "/revisions"
	case strings.HasPrefix(path, "/api/texture/documents/") && strings.HasSuffix(path, "/history"):
		docID := strings.TrimPrefix(path, "/api/texture/documents/")
		docID = strings.TrimSuffix(docID, "/history")
		corpusdPath = "/internal/platform/texture/documents/" + url.PathEscape(docID) + "/revisions"
	case strings.HasPrefix(path, "/api/texture/documents/") && strings.HasSuffix(path, "/stream"):
		// Published articles don't need live SSE; return empty event stream
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		return
	case strings.HasPrefix(path, "/api/texture/documents/"):
		docID := strings.TrimPrefix(path, "/api/texture/documents/")
		corpusdPath = "/internal/platform/texture/documents/" + url.PathEscape(docID)
	case strings.HasPrefix(path, "/api/texture/revisions/"):
		revisionID := strings.TrimPrefix(path, "/api/texture/revisions/")
		corpusdPath = "/internal/platform/texture/revisions/" + url.PathEscape(revisionID)
	default:
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}

	target, err := joinBasePath(h.cfg.CorpusdURL, corpusdPath)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build corpusd URL"})
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to build corpusd request"})
		return
	}
	httpReq.Header.Set("X-Internal-Caller", "true")

	resp, err := h.corpusd.Do(httpReq)
	if err != nil {
		log.Printf("proxy: corpusd texture read: %v", err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to read from corpusd"})
		return
	}
	defer func() { _ = resp.Body.Close() }()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("proxy: corpusd texture read copy: %v", err)
	}
}

// HandlePlatformObjectGraph forwards object graph API requests from the
// runtime (inside a VM) to corpusd. The runtime's objectgraph.HTTPStore
// sets X-Internal-Caller: true; the proxy checks this header on the incoming
// request and then sets its own X-Internal-Caller header when forwarding to
// corpusd, so the caller cannot forge arbitrary header values.
//
// Security model: this route uses the same X-Internal-Caller trust signal as
// HandleInternalWirePlatformPublish. The Caddy edge (nix/node-b.nix) blocks
// all /internal/* routes from the public internet with a 403, so this handler
// is only reachable from inside VMs via the VM-to-host network. VM processes
// are already trusted to run runtime code. A hard cryptographic boundary
// (shared secret between proxy and corpusd) is out of scope for this PR.
// Both /internal/platform/objects and /internal/platform/edges (with optional
// sub-paths and query strings) are forwarded as-is.
func (h *Handler) HandlePlatformObjectGraph(w http.ResponseWriter, r *http.Request) {
	// Require the internal-caller header on the incoming request, matching
	// the HandleInternalWirePlatformPublish pattern.
	if r.Header.Get("X-Internal-Caller") != "true" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "internal caller required"})
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
	httpReq.Header.Set("X-Internal-Caller", "true")
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("proxy: corpusd objectgraph copy: %v", err)
	}
}

// isPlatformTextureReadRequest returns true for read-only Texture requests that
// should be served from corpusd's published store rather than the sandbox.
func isPlatformTextureReadRequest(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	if strings.TrimSpace(r.URL.Query().Get("read_owner")) != vmctl.UniversalWirePlatformOwnerID {
		return false
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/texture/documents/") {
		return true
	}
	return strings.HasPrefix(path, "/api/texture/revisions/")
}

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
	s.HandleFunc("/internal/wire/platform/publications/texture", h.HandleInternalWirePlatformPublish)
	// Object graph API: forward runtime objectgraph HTTPStore calls to
	// corpusd. The runtime (inside a VM) reaches corpusd through the
	// proxy; these routes pass through /internal/platform/objects and
	// /internal/platform/edges to corpusd's object graph handler.
	s.HandleFunc("/internal/platform/objects", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/objects/", h.HandlePlatformObjectGraph)
	s.HandleFunc("/internal/platform/edges", h.HandlePlatformObjectGraph)
}
