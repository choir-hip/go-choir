package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway/searchplane"
	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/hostfetch"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/server"
)

const inferenceTimeout = 10 * time.Minute

// HealthResponse is the JSON structure returned by GET /health.
type gatewayHealthResponse struct {
	Status               string   `json:"status"`
	Service              string   `json:"service"`
	Provider             string   `json:"provider"`
	ActiveIdentities     int      `json:"active_identities"`
	RateLimitMaxRequests int      `json:"rate_limit_max_requests,omitempty"`
	RateLimitWindowMs    int64    `json:"rate_limit_window_ms,omitempty"`
	SearchProviders      []string `json:"search_providers,omitempty"`
	// PromptCache is the per-provider cached-input share since the gateway
	// started: sample it twice and subtract for an hourly ratio.
	PromptCache map[string]gatewayCacheHealth `json:"prompt_cache,omitempty"`
	// BearerBinding is the SEC-05 bearer binding: its mode, how many active
	// computers are pinned, and what it has refused or would refuse.
	BearerBinding *gatewayBindingHealth `json:"bearer_binding,omitempty"`
	// Credentials is how long the active gateway credentials have left. A
	// bearer that is only ever refreshed expires one TTL after its last cold
	// boot unless the host's ensure renews it.
	Credentials *gatewayCredentialHealth `json:"credentials,omitempty"`
	// AnthropicRelay is "off" (no key), "dark" (a key, but no identity or no
	// open window) or "on". It names neither the key nor an identity.
	AnthropicRelay string `json:"anthropic_relay,omitempty"`
	// ModelScheduler is the RES-11 model-call scheduler: its mode, the calls
	// in flight and queued, and what it has granted, refused and cancelled.
	ModelScheduler *gatewaySchedulerHealth `json:"model_scheduler,omitempty"`
	// SyntheticGate is the Synthetic concurrency gate: its limit, calls in
	// flight, and what has waited, timed out or been cancelled.
	SyntheticGate *provider.SyntheticGateStats `json:"synthetic_gate,omitempty"`
	// BodyCaps is the request body caps: mode, limits, refusals and the
	// largest body seen per route class (B7).
	BodyCaps *gatewayBodyCapsHealth `json:"body_caps,omitempty"`
	// ImageMedia is the image decode slots and the downscaled output
	// budget, with their waits and refusals (FP0 round 5, F7).
	ImageMedia *provider.ImageMediaStats `json:"image_media,omitempty"`
}

// gatewayCacheHealth is provider.CacheStats plus its hit rate.
type gatewayCacheHealth struct {
	provider.CacheStats
	HitRate float64 `json:"hit_rate"`
}

// ProviderRequest is the JSON payload for POST /provider/v1/inference.
// Autoputer callers send this to request an LLM inference call through
// the gateway, which injects host-side credentials before forwarding
// to the upstream provider (VAL-GATEWAY-004).
type ProviderRequest struct {
	// Provider is the requested provider ("chatgpt", "bedrock", "zai",
	// "fireworks", "opencode-go", or "opencode-zen"). If empty, the gateway
	// requires model-based routing.
	Provider string `json:"provider,omitempty"`

	// Model is an optional model override.
	Model string `json:"model,omitempty"`
	// ConversationID is the durable caller-run identity used for provider
	// session affinity. It is routing metadata, not a trust input.
	ConversationID string `json:"conversation_id,omitempty"`
	// CacheKey is the caller's prompt-cache thread key (one desk on one
	// computer at one prompt version). Like ConversationID it is routing
	// metadata, not a trust input; providers that route cache lookups by a
	// client key send it.
	CacheKey string `json:"cache_key,omitempty"`

	// Messages is the conversation history in Anthropic Messages format.
	Messages []provider.Message `json:"messages"`

	// System is the optional system prompt.
	System string `json:"system,omitempty"`

	// Tools is the list of tool definitions.
	Tools []provider.ToolDef `json:"tools,omitempty"`

	// ToolChoice optionally constrains compatible provider tool selection.
	ToolChoice string `json:"tool_choice,omitempty"`

	// MaxTokens is the maximum output tokens.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Stream controls streaming. The gateway forces this to false for
	// now because it proxies non-streaming calls.
	Stream bool `json:"stream,omitempty"`

	// ReasoningEffort optionally sets provider-specific reasoning effort for
	// this request.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// ProviderResponse is the JSON response for successful inference calls.
type ProviderResponse struct {
	// ID is the provider-assigned response ID.
	ID string `json:"id"`

	// Text is the concatenated text content.
	Text string `json:"text"`

	// ReasoningContent is hidden provider context for compatible reasoning
	// models. It is returned to autoputer runtimes for multi-turn provider
	// continuity, not rendered as user-facing answer text.
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// Model is the model that produced the response.
	Model string `json:"model"`

	// StopReason is why generation stopped.
	StopReason string `json:"stop_reason"`

	// Usage contains token counts.
	Usage provider.Usage `json:"usage"`

	// ToolCalls contains any tool invocation requests.
	ToolCalls []provider.ContentToolCall `json:"tool_calls,omitempty"`

	// ProviderName identifies which provider handled this request.
	ProviderName string `json:"provider_name"`
}

// ErrorResponse is the JSON error envelope returned by the gateway.
// Errors are sanitized and never contain credentials, raw auth headers,
// or upstream response bodies (VAL-GATEWAY-007).
type ErrorResponse struct {
	Error string `json:"error"`
}

// Handler provides HTTP handlers for the gateway service.
type Handler struct {
	registry     *IdentityRegistry
	provider     provider.Provider        // single provider mode (backward compat)
	providers    *provider.MultiProvider  // multi-provider mode (may be nil)
	rateLimiter  *PerAutoputerRateLimiter // per-autoputer rate limiter (may be nil)
	searchClient *SearchClient            // web search client with rotation (may be nil)
	breakers     *BreakerRegistry         // per-provider circuit breakers (may be nil)
	jevTransport *JevTransport            // OpenRouter Decisions transport (may be nil)
	// sourceAPI serves /provider/v1/source-api; nil means the client built
	// from the environment (source_api.go).
	sourceAPI *SourceAPIClient
	// hostFetch serves /provider/v1/fetch; nil means the default service
	// built from the environment (host_fetch.go).
	hostFetch *hostFetchService
	// serviceCheckers maps a dependency service name to its health probe.
	// Used by GET /health/{service} so operators can observe backend
	// dependency health from
	// outside the gateway without auth (M22b / C20). May be nil; in that
	// case /health/{service} reports "not configured".
	serviceCheckers map[string]health.Checker
	// bindingMode is the bearer-binding rollout mode (bearer_binding.go);
	// the zero value enforces.
	bindingMode BindingMode
	binding     bindingCounters
	bindingLogState
	// anthropicRelay serves /provider/anthropic/* (SEC-08, Option K); nil
	// means every relay call is answered 503.
	anthropicRelay *AnthropicRelay
	// modelScheduler, when set, replaces the fixed-window limiter for model
	// calls (inference and OpenAI-compatible chat) with a fair queue
	// (model_scheduler.go, RES-11).
	modelScheduler *ModelScheduler
	// bodyCaps bounds every request body the gateway decodes
	// (body_caps.go, B7); nil means the code defaults, enforced.
	bodyCaps *bodyCaps
}

const internalCallerHeader = "X-Internal-Caller"

// NewHandlerWithRateLimit creates a gateway Handler with per-autoputer rate
// limiting and a single provider. Each autoputer identity gets an independent
// request quota so one noisy autoputer cannot starve another
// (VAL-GATEWAY-005, VAL-CROSS-115).
func NewHandlerWithRateLimit(registry *IdentityRegistry, p provider.Provider, rl *PerAutoputerRateLimiter) *Handler {
	return &Handler{
		registry:     registry,
		provider:     p,
		providers:    nil,
		rateLimiter:  rl,
		searchClient: NewSearchClient(),
		bodyCaps:     newHandlerBodyCaps(),
	}
}

// NewMultiHandlerWithRateLimit creates a gateway Handler with multi-provider
// routing and per-autoputer rate limiting.
func NewMultiHandlerWithRateLimit(registry *IdentityRegistry, mp *provider.MultiProvider, rl *PerAutoputerRateLimiter) *Handler {
	return &Handler{
		registry:     registry,
		provider:     nil,
		providers:    mp,
		rateLimiter:  rl,
		searchClient: NewSearchClient(),
		bodyCaps:     newHandlerBodyCaps(),
	}
}

// SetBreakers attaches a per-provider circuit breaker registry so the gateway
// can expose breaker state and reset them via ops endpoints. This is optional;
// when nil, breaker-related endpoints report "not configured".
func (h *Handler) SetBreakers(r *BreakerRegistry) {
	h.breakers = r
}

// SetJevTransport attaches the independently gated Jev transport. A nil
// transport leaves the registered route unavailable rather than falling back
// to another model or endpoint.
func (h *Handler) SetJevTransport(transport *JevTransport) {
	h.jevTransport = transport
}

// SetServiceCheckers attaches the per-service health probes used by
// GET /health/{service}. The map keys are service names; the values are
// health.Checker implementations. This is optional; when nil or missing a service, /health/{service} reports
// "not configured" for that service. The checkers must be cheap and
// side-effect free (M22b / C20).
func (h *Handler) SetServiceCheckers(checkers map[string]health.Checker) {
	h.serviceCheckers = checkers
}

// HandleHealth handles GET /health for the gateway service.
// It reports the active provider(s), identity count, and rate limiter config.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	providerName := "none"
	if h.providers != nil {
		// Multi-provider mode: report all registered providers.
		names := h.providers.Names()
		if len(names) > 0 {
			providerName = strings.Join(names, ",")
		}
	} else if h.provider != nil {
		providerName = h.provider.Name()
	}

	resp := gatewayHealthResponse{
		Status:           "ok",
		Service:          "gateway",
		Provider:         providerName,
		ActiveIdentities: h.registry.ActiveCount(),
	}

	if h.rateLimiter != nil {
		used, limit, resetIn := h.rateLimiter.Status("__health_check__")
		_ = used
		_ = resetIn
		resp.RateLimitMaxRequests = limit
		resp.RateLimitWindowMs = h.rateLimiter.window.Milliseconds()
	}

	if h.searchClient != nil {
		resp.SearchProviders = h.searchClient.AvailableProviders()
	}

	resp.BearerBinding = h.bindingHealth()
	resp.Credentials = h.credentialHealth()
	resp.AnthropicRelay = h.anthropicRelay.status()
	resp.ModelScheduler = h.schedulerHealth()
	resp.BodyCaps = h.bodyCapsHealth()
	if st, ok := provider.SyntheticGateStatus(); ok {
		resp.SyntheticGate = &st
	}
	media := provider.ImageMediaStatus()
	resp.ImageMedia = &media

	if stats := provider.CacheStatsSnapshot(); len(stats) > 0 {
		resp.PromptCache = make(map[string]gatewayCacheHealth, len(stats))
		for name, s := range stats {
			resp.PromptCache[name] = gatewayCacheHealth{CacheStats: s, HitRate: s.HitRate()}
		}
	}

	writeGatewayJSON(w, http.StatusOK, resp)
}

// serviceHealthResponse is the JSON body returned by GET /health/{service}.
// It exposes only coarse status and a generic error message — never
// credentials, internal URLs, or upstream response bodies (M22b / C20:
// health endpoints are public but expose no secrets).
type serviceHealthResponse struct {
	Status    string  `json:"status"`
	Service   string  `json:"service"`
	LatencyMs float64 `json:"latency_ms,omitempty"`
	Error     string  `json:"error,omitempty"`
	Breaker   *string `json:"breaker,omitempty"`
}

// HandleServiceHealth handles GET /health/{service} for the gateway. It probes
// a single named backend dependency and reports its coarse status. The
// endpoint is public (no auth) and exposes no secrets: the response contains only the service name, a status of
// "ok"/"unhealthy"/"not configured", a generic error message, and — when a
// circuit breaker is registered for the service — the breaker state.
//
// This makes backend dependency health observable from outside the gateway
// without disrupting existing request routing (M22b / C20).
func (h *Handler) HandleServiceHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	// Extract the service name from the path. The route is registered as
	// "/health/{service}" so r.PathValue("service") is the segment. Fall
	// back to trimming the "/health/" prefix for older mux registrations.
	service := r.PathValue("service")
	if service == "" {
		service = strings.TrimPrefix(r.URL.Path, "/health/")
	}
	service = strings.TrimSpace(service)
	if service == "" {
		writeGatewayJSON(w, http.StatusNotFound, ErrorResponse{Error: "service not specified"})
		return
	}

	if h.serviceCheckers == nil {
		writeGatewayJSON(w, http.StatusOK, serviceHealthResponse{
			Status:  "not configured",
			Service: service,
		})
		return
	}

	checker, ok := h.serviceCheckers[service]
	if !ok {
		writeGatewayJSON(w, http.StatusOK, serviceHealthResponse{
			Status:  "not configured",
			Service: service,
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	start := time.Now()
	err := checker.Check(ctx)
	latency := float64(time.Since(start).Microseconds()) / 1000.0

	resp := serviceHealthResponse{
		Service:   service,
		LatencyMs: latency,
	}
	if err != nil {
		resp.Status = string(health.StatusUnhealthy)
		resp.Error = "dependency check failed"
	} else {
		resp.Status = string(health.StatusOK)
	}

	// Surface the circuit breaker state for this service when one is
	// registered. This is a single word ("closed", "open", "half-open") and
	// carries no secret material.
	if h.breakers != nil {
		if b := h.breakers.breakerFor(service); b != nil {
			state := b.State().String()
			resp.Breaker = &state
		}
	}

	code := http.StatusOK
	if err != nil {
		code = http.StatusServiceUnavailable
	}
	writeGatewayJSON(w, code, resp)
}

// HandleInference handles POST /provider/v1/inference.
// It authenticates the autoputer caller via Bearer token, resolves the
// appropriate provider (multi-provider routing or single-provider
// fallback), injects host-side provider credentials, calls the upstream
// provider, and returns the response with sanitized errors
// (VAL-GATEWAY-001, VAL-GATEWAY-003, VAL-GATEWAY-004, VAL-GATEWAY-007).
func (h *Handler) HandleInference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	// Step 1: Authenticate the autoputer caller.
	computerID, err := h.authenticateAutoputer(r)
	if err != nil {
		log.Printf("gateway: authentication denied: %v", err)
		writeAuthFailure(w, err)
		return
	}

	// Step 2: Enforce per-autoputer rate limiting (VAL-GATEWAY-005). With the
	// model scheduler on, model calls are scheduled fairly at step 5 instead
	// (RES-11): no fixed window, bounded concurrency, queue and wait.
	if h.rateLimiter != nil && !h.schedulerOn() {
		if !h.rateLimiter.Record(computerID) {
			_, _, resetIn := h.rateLimiter.Status(computerID)
			retrySeconds := int(resetIn.Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			log.Printf("gateway: rate limit exceeded for autoputer %s", computerID)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
			writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{
				Error: fmt.Sprintf("rate limit exceeded for autoputer %s (limit %d requests per %s)",
					computerID, h.rateLimiter.maxReqs, h.rateLimiter.window),
			})
			return
		}
	}

	// RES-11: take a slot for the whole call, a stream included, before the
	// body is read, so a queued call holds no body (B7). The release runs
	// on every exit, a panic too.
	release, r, ok := h.scheduleModelCall(w, r, computerID, false)
	if !ok {
		return
	}
	defer release()

	// Step 3: Decode the request, under the model body cap (B7).
	var req ProviderRequest
	releaseBody, err := h.decodeBodyHeld(w, r, bodyRouteModel, computerID, &req, false)
	defer releaseBody() // the decoded request lives for the whole call (F1)
	if err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	// Step 4: Resolve the provider (multi-provider or single-provider).
	p, err := h.resolveProvider(req)
	if err != nil {
		log.Printf("gateway: provider resolution failed for autoputer %s: %v", computerID, err)
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	if p == nil {
		log.Printf("gateway: no provider configured for request from autoputer %s", computerID)
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error: "no provider configured",
		})
		return
	}

	// Step 5: Build the LLM request and call the provider.
	llmReq := provider.LLMRequest{
		Provider:        req.Provider,
		Model:           req.Model,
		System:          req.System,
		Messages:        req.Messages,
		Tools:           req.Tools,
		ToolChoice:      req.ToolChoice,
		MaxTokens:       req.MaxTokens,
		Stream:          req.Stream,
		ReasoningEffort: req.ReasoningEffort,
		ConversationID:  req.ConversationID,
		CacheKey:        req.CacheKey,
	}

	// Images are logged by count, size and short sha256, never by data
	// (problems/models-configured-text-only-2026-10-15.md, FM11).
	log.Printf("gateway: inference request from autoputer %s (provider=%s model=%s messages=%d tools=%d tool_choice=%s system_chars=%d max_tokens=%d reasoning=%s stream=%v %s)",
		computerID, p.Name(), req.Model, len(req.Messages), len(req.Tools), req.ToolChoice, len(req.System), req.MaxTokens, req.ReasoningEffort, req.Stream, provider.SummarizeMedia(req.Messages))

	ctx, cancel := context.WithTimeout(r.Context(), inferenceTimeout)
	defer cancel()

	// If the client requests streaming, use SSE streaming.
	if req.Stream {
		h.handleStreamingInference(w, r, p, llmReq, computerID)
		return
	}

	resp, err := p.Call(ctx, llmReq)
	if err != nil {
		// Sanitize the error: never include raw upstream bodies,
		// credentials, or auth headers in the response
		// (VAL-GATEWAY-007).
		sanitized := sanitizeError(err)
		log.Printf("gateway: provider call failed for autoputer %s: %v (sanitized: %s)",
			computerID, err, sanitized)
		status := http.StatusBadGateway
		if provider.IsMediaError(err) {
			// A refused image is the caller's error: not retried as a
			// provider outage.
			status = http.StatusBadRequest
		}
		writeGatewayJSON(w, status, ErrorResponse{Error: sanitized})
		return
	}

	// Step 6: Return the successful response.
	log.Printf("gateway: inference succeeded for autoputer %s (provider=%s tokens=%d+%d cached=%d text_len=%d)",
		computerID, resp.ProviderName, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens, len(resp.Text))

	writeGatewayJSON(w, http.StatusOK, ProviderResponse{
		ID:               resp.ID,
		Text:             resp.Text,
		ReasoningContent: resp.ReasoningContent,
		Model:            resp.Model,
		StopReason:       resp.StopReason,
		Usage:            resp.Usage,
		ToolCalls:        resp.ToolCalls,
		ProviderName:     resp.ProviderName,
	})
}

// handleStreamingInference handles streaming inference requests by setting up
// an SSE response and forwarding chunks from the provider to the client in
// real-time (VAL-LLM-003, VAL-LLM-004).
func (h *Handler) handleStreamingInference(w http.ResponseWriter, r *http.Request, p provider.Provider, llmReq provider.LLMRequest, computerID string) {
	// Set up SSE response headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	flusher, canFlush := w.(http.Flusher)

	ctx, cancel := context.WithTimeout(r.Context(), inferenceTimeout)
	defer cancel()

	resp, err := p.Stream(ctx, llmReq, func(chunk provider.StreamChunk) {
		// Marshal the chunk as SSE data.
		data, err := json.Marshal(chunk)
		if err != nil {
			log.Printf("gateway: marshal stream chunk: %v", err)
			return
		}

		// Write SSE event.
		fmt.Fprintf(w, "data: %s\n\n", data)
		if canFlush {
			flusher.Flush()
		}
	})

	if err != nil {
		// If we haven't written any SSE data yet, we can return an HTTP error.
		// Otherwise, send an error event.
		sanitized := sanitizeError(err)
		log.Printf("gateway: streaming provider call failed for autoputer %s: %v (sanitized: %s)",
			computerID, err, sanitized)

		errData, _ := json.Marshal(map[string]string{"error": sanitized})
		fmt.Fprintf(w, "data: %s\n\n", errData)
		if canFlush {
			flusher.Flush()
		}
		return
	}

	// Send final [DONE] marker.
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}

	log.Printf("gateway: streaming inference succeeded for autoputer %s (provider=%s tokens=%d+%d cached=%d text_len=%d)",
		computerID, resp.ProviderName, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.CachedInputTokens, len(resp.Text))
}

// resolveProvider selects the appropriate provider for the given request.
// In multi-provider mode, it routes based on the explicit provider field
// or the model parameter. In single-provider mode, it validates that the
// requested provider matches the configured one.
func (h *Handler) resolveProvider(req ProviderRequest) (provider.Provider, error) {
	// Multi-provider mode: route based on provider name or model.
	if h.providers != nil {
		return h.resolveFromMultiProvider(req)
	}

	// Single-provider mode: validate the requested provider matches.
	if req.Provider != "" && h.provider != nil && req.Provider != h.provider.Name() {
		return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
	}

	return h.provider, nil
}

// resolveFromMultiProvider selects a provider from the multi-provider
// registry. It tries explicit provider name first, then model-based routing.
func (h *Handler) resolveFromMultiProvider(req ProviderRequest) (provider.Provider, error) {
	// Step 1: Explicit provider name routing.
	if req.Provider != "" {
		p := h.providers.Get(req.Provider)
		if p == nil {
			return nil, fmt.Errorf("unsupported provider: %s", req.Provider)
		}
		// The host catalog decides which models a catalogued provider serves;
		// a retired or unknown model is refused here rather than passed
		// through (docs/model-pools-design-2026-10-12.md).
		if req.Model != "" && modelcatalog.ProviderCatalogued(req.Provider) {
			if _, ok := modelcatalog.Lookup(req.Provider, req.Model); !ok {
				return nil, fmt.Errorf("model %q is not in the catalog for provider %s", req.Model, req.Provider)
			}
		}
		return p, nil
	}

	// Step 2: Model-based routing using SupportedModels.
	if req.Model != "" {
		for _, mi := range provider.SupportedModels() {
			if mi.ID == req.Model {
				p := h.providers.Get(mi.Provider)
				if p != nil {
					return p, nil
				}
			}
		}

		// Uncatalogued GPT models fail routing loudly: the catalog is the
		// only authority for which ChatGPT models exist.
		if strings.HasPrefix(req.Model, "glm-") {
			if p := h.providers.Get("zai"); p != nil {
				return p, nil
			}
		}
		if strings.Contains(req.Model, "anthropic") || strings.Contains(req.Model, "claude") {
			if p := h.providers.Get("bedrock"); p != nil {
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("provider or model is required")
}

// HandleIssueCredential handles POST /provider/v1/credentials/issue.
// This is an internal operator endpoint for issuing autoputer credentials after
// the authenticated desktop/bootstrap path has resolved a host-side runtime.
// The browser must never receive these credentials directly, so only explicit
// loopback-marked host callers may issue them.
func (h *Handler) HandleIssueCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "access denied"})
		return
	}

	var req struct {
		ComputerID string `json:"computer_id"`
	}
	if err := h.decodeBody(w, r, bodyRouteOperator, "host", &req, false); err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	if req.ComputerID == "" {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "computer_id is required"})
		return
	}

	result, err := h.registry.IssueCredential(req.ComputerID)
	if err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to issue credential"})
		return
	}

	log.Printf("gateway: issued credential for autoputer %s (expires=%s)", req.ComputerID, result.ExpiresAt)

	writeGatewayJSON(w, http.StatusCreated, result)
}

// HandleEnsureCredential handles POST /provider/v1/credentials/ensure.
// It lets host-side lifecycle control reconcile an existing VM bootstrap token
// into the durable gateway identity store after a gateway restart or upgrade.
// It is internal-only and never returns the raw token.
func (h *Handler) HandleEnsureCredential(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "access denied"})
		return
	}

	var req struct {
		RawToken string `json:"raw_token"`
		// PeerIP is the computer's current guest address, which moves the
		// Jev pin to it (EnsureCredentialForPeer). Optional.
		PeerIP string `json:"peer_ip,omitempty"`
	}
	if err := h.decodeBody(w, r, bodyRouteOperator, "host", &req, false); err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.RawToken) == "" {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "raw_token is required"})
		return
	}
	// A peer that is not a Choir guest address (a loopback runtime in
	// development, say) never moves the Jev pin, and never fails the
	// credential's reconciliation, which is this route's first job.
	peer := strings.TrimSpace(req.PeerIP)
	if peer != "" {
		if ip := net.ParseIP(peer); ip == nil || !isChoirGuestPeer(ip) {
			log.Printf("gateway: credential ensure named peer %q, which is not a Choir guest address; the Jev pin is not moved", peer)
			peer = ""
		}
	}

	result, err := h.registry.EnsureCredentialForPeer(req.RawToken, peer)
	if err != nil {
		writeGatewayJSON(w, http.StatusConflict, ErrorResponse{Error: "credential could not be ensured"})
		return
	}
	if result.Status == "imported" {
		log.Printf("gateway: imported existing credential for autoputer %s (expires=%s)", result.ComputerID, result.ExpiresAt)
	}
	if result.JevPeerMoved {
		log.Printf("gateway: Jev bearer of autoputer %s moved to its current guest address %s", result.ComputerID, peer)
	}
	writeGatewayJSON(w, http.StatusOK, result)
}

// authenticateAutoputer validates the Bearer token from the Authorization
// header against the identity registry. Returns the autoputer ID if valid
// (VAL-GATEWAY-003).
func (h *Handler) authenticateAutoputer(r *http.Request) (string, error) {
	computerID, err := h.authenticateBearer(r)
	if err != nil {
		return "", err
	}
	// SEC-05: the bearer works only from its computer's own address.
	if err := h.checkBearerBinding(computerID, r); err != nil {
		return "", err
	}
	return computerID, nil
}

// authenticateBearer is authenticateAutoputer without the SEC-05 binding: the
// bearer and the runtime-peer rule only, writing nothing. The Jev route uses
// it with its own pin check and 403 (jevPeerAllowed); the Anthropic relay
// uses it with a pin check of its own that ignores the rollout mode.
func (h *Handler) authenticateBearer(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", fmt.Errorf("missing authorization header")
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return "", fmt.Errorf("invalid authorization scheme")
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return "", fmt.Errorf("empty bearer token")
	}

	computerID, err := h.registry.ValidateCredential(token)
	if err != nil {
		return "", err
	}
	if !isAuthorizedRuntimePeer(r) {
		return "", fmt.Errorf("gateway access is limited to choir runtime peers")
	}
	return computerID, nil
}

// sanitizeError converts a provider error into a safe, bounded error
// message that does not leak credentials, raw auth headers, or stack
// traces (VAL-GATEWAY-007).
func sanitizeError(err error) string {
	msg := err.Error()

	// The provider package already returns sanitized errors
	// (e.g., "bedrock: status 503 (sanitized)"), but we add an
	// additional layer of protection here.

	// Strip any accidental credential-like patterns.
	// This is defense-in-depth; the provider layer should already
	// sanitize.
	for _, pattern := range []string{"Bearer ", "x-api-key ", "Authorization:"} {
		if idx := strings.Index(msg, pattern); idx >= 0 {
			// Truncate at the first sign of credential leakage.
			msg = msg[:idx] + "(redacted)"
		}
	}

	// Bound the error message length.
	if len(msg) > 500 {
		msg = msg[:497] + "..."
	}

	return msg
}

// isLocalhost checks whether the request originated from a loopback address.
// Operator-only credential endpoints must key off the actual peer address, not
// the Host header, which a remote caller can spoof once the gateway listens on
// non-loopback interfaces.
func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isAuthorizedCredentialCaller(r *http.Request) bool {
	if r == nil {
		return false
	}
	// Loopback, marker, then the host-internal credential: the credential
	// and operator routes mint and import gateway identities
	// (docs/problems/loopback-is-a-credential-2026-10-14.md).
	return isLocalhost(r) && strings.EqualFold(strings.TrimSpace(r.Header.Get(internalCallerHeader)), "true") && hostauth.Authorize(r)
}

func isAuthorizedRuntimePeer(r *http.Request) bool {
	ip := remoteIP(r)
	if ip == nil {
		return false
	}
	return runtimePeerAllowed(ip, testing.Testing())
}

// runtimePeerAllowed is the runtime-peer rule with the test-binary switch
// explicit, so a test can assert the production rule: loopback or a Choir
// guest address, and the RFC 5737 documentation ranges (httptest's
// RemoteAddr) only inside a test binary, never in production.
func runtimePeerAllowed(ip net.IP, inTestBinary bool) bool {
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || isChoirGuestPeer(ip) || (inTestBinary && hostauth.IsDocumentationNetwork(ip))
}

func remoteIP(r *http.Request) net.IP {
	if r == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(host)
}

// Choir guests boot on host tap networks with the host side at *.1 and the
// guest side at *.2. Keep this aligned with vmmanager's bounded private
// allocation pool so a leaked bearer token is not enough to call the gateway
// from arbitrary hosts.
func isChoirGuestPeer(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return (ip4[0] == 172 && ip4[2] == 0 && ip4[3] == 2) ||
		(ip4[0] == 10 && ip4[1] >= 200 && ip4[1] <= 215 && ip4[3] == 2)
}

// writeGatewayJSON writes a JSON response.
func writeGatewayJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("gateway: json encode error: %v", err)
	}
}

// HandleSearch handles POST /provider/v1/search for web search requests.
// It authenticates the autoputer caller, validates the request, starts every
// eligible provider at once, and streams NDJSON: a start line, one line per
// provider as that provider answers, and a final line with M of N answered.
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}

	// Step 1: Authenticate the autoputer caller.
	computerID, err := h.authenticateAutoputer(r)
	if err != nil {
		log.Printf("gateway: search authentication denied: %v", err)
		writeAuthFailure(w, err)
		return
	}

	// Step 2: Enforce per-autoputer search rate limiting. Search uses a separate
	// bucket from inference so exploratory search bursts cannot starve the
	// model calls needed to summarize findings or revise documents.
	if h.rateLimiter != nil {
		bucketKey := rateLimitBucketKey(computerID, "search")
		if !h.rateLimiter.Record(bucketKey) {
			_, _, resetIn := h.rateLimiter.Status(bucketKey)
			retrySeconds := int(resetIn.Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			log.Printf("gateway: search rate limit exceeded for autoputer %s", computerID)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
			writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{
				Error: fmt.Sprintf("rate limit exceeded for autoputer %s", computerID),
			})
			return
		}
	}

	// Step 3: Decode the request.
	var req SearchRequest
	if err := h.decodeBody(w, r, bodyRouteSearch, computerID, &req, false); err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Query == "" {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "query is required"})
		return
	}

	// Step 4: Check if search is configured.
	if h.searchClient == nil || len(h.searchClient.AvailableProviders()) == 0 {
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error: "search not configured: no search providers available",
		})
		return
	}

	log.Printf("gateway: search request from autoputer %s (query=%q max_results=%d)",
		computerID, req.Query, req.MaxResults)

	// Step 5: Start every eligible provider and stream each one's answer as
	// it lands. The stream is bounded by the provider timeout plus a margin.
	ctx, cancel := context.WithTimeout(r.Context(), h.searchClient.StreamBound())
	defer cancel()

	stream, err := h.searchClient.StreamFor(ctx, computerID, req)
	if err != nil {
		sanitized := sanitizeError(err)
		log.Printf("gateway: search failed for autoputer %s: %v", computerID, sanitized)
		writeGatewayJSON(w, http.StatusBadGateway, ErrorResponse{Error: sanitized})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	writeLine := func(v any) {
		if err := enc.Encode(v); err != nil {
			log.Printf("gateway: search stream write for autoputer %s: %v", computerID, err)
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	writeLine(searchStreamStart(stream))
	// Under budget routing a paid provider may join after the start line,
	// so the stream ends when its arrivals close (or at the bound), not
	// when the start line's providers have answered.
	arrived := map[string]bool{}
	answered := 0
	var resultURLs []string
	for {
		var arrival searchplane.Arrival
		var ok bool
		select {
		case arrival, ok = <-stream.Arrivals:
		case <-ctx.Done():
		}
		if !ok {
			break
		}
		arrived[arrival.Provider] = true
		if arrival.Answered() {
			answered++
		}
		for _, hit := range arrival.Results {
			resultURLs = append(resultURLs, hit.URL)
		}
		writeLine(searchStreamArrival(arrival))
	}
	started := searchStarted(stream)
	route := stream.Route.Summary()
	done := SearchStreamDone{
		Type:      "done",
		Answered:  answered,
		Total:     len(started),
		Cooling:   searchCoolingProviders(append(append([]searchplane.Cooling{}, stream.Cooling...), route.Skipped...)),
		Escalated: searchEscalations(route),
	}
	for _, name := range started {
		if !arrived[name] {
			done.Pending = append(done.Pending, name)
		}
	}
	writeLine(done)

	// served/results/unique/seen measure reuse
	// (docs/problems/search-is-duplicated-and-discarded-2026-10-14.md):
	// seen is how many of this search's distinct URLs the computer already
	// received from an earlier search within a day.
	unique, seen := h.searchClient.observeResultURLs(computerID, resultURLs)
	// routing/escalated/paid_calls/today measure spend
	// (docs/reports/search-efficiency-and-reuse-2026-10-14.md §4.1).
	skipped := append(append([]searchplane.Cooling{}, stream.Cooling...), route.Skipped...)
	log.Printf("gateway: search for autoputer %s answered %d of %d providers (%d cooling, %d pending) skipped=[%s] served=%s results=%d unique=%d seen=%d %s",
		computerID, answered, len(started), len(skipped), len(done.Pending), searchSkippedSummary(skipped),
		searchServed(stream), len(resultURLs), unique, seen, searchRouteSummary(stream))
}

// HandleSearchHealth handles GET /provider/v1/search/health. Like the
// breaker routes below it is operator-only: computers reach this port for
// inference, so authority is the loopback transport, not a header.
func (h *Handler) HandleSearchHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "operator route"})
		return
	}
	if h.searchClient == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "search not configured"})
		return
	}
	store, err := h.searchClient.HealthStore()
	if err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, ErrorResponse{Error: sanitizeError(err)})
		return
	}
	snapshot, err := store.Snapshot()
	if err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, ErrorResponse{Error: sanitizeError(err)})
		return
	}
	out := map[string]ProviderHealthSummary{}
	for name, health := range snapshot {
		summary := ProviderHealthSummary{
			State:            health.State,
			StrikeCount:      health.StrikeCount,
			LastFailureClass: health.LastFailureClass,
			LastErrorSummary: searchplane.RedactSummary(health.LastErrorSummary),
		}
		if health.CooldownUntil != nil {
			summary.CooldownUntil = health.CooldownUntil.UTC().Format(time.RFC3339)
		}
		out[name] = summary
	}
	// daily_calls: today's (UTC) provider calls with cap and tier, the
	// counts behind budget routing's daily caps. cap -1 is no cap.
	counts, err := h.searchClient.DailyCounts()
	if err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, ErrorResponse{Error: sanitizeError(err)})
		return
	}
	daily := map[string]SearchDailyCalls{}
	for _, c := range counts {
		entry := SearchDailyCalls{Day: c.Day, Calls: c.Calls, Cap: c.Cap, Tier: string(c.Tier.Tier)}
		if m := c.Monthly; m != nil {
			entry.Monthly = &SearchMonthlyPace{PeriodStart: m.PeriodStart, PeriodEnd: m.PeriodEnd, UsedBeforeToday: m.Used, Target: m.Target, DaysLeft: m.DaysLeft, DayCap: m.Cap}
		}
		daily[c.Provider] = entry
	}
	writeGatewayJSON(w, http.StatusOK, map[string]any{"provider_health": out, "daily_calls": daily})
}

// SearchDailyCalls is one provider's calls today (UTC) on the search
// health route. Cap is -1 when the provider has no daily cap.
type SearchDailyCalls struct {
	Day   string `json:"day"`
	Calls int    `json:"calls"`
	Cap   int    `json:"cap"`
	Tier  string `json:"tier"`
	// Monthly is the provider's monthly-allowance pace (Keenable), which
	// sets Cap. Additive.
	Monthly *SearchMonthlyPace `json:"monthly,omitempty"`
}

// SearchMonthlyPace is a monthly-quota provider's pace for today: the
// billing period, the calls counted in it before today, the period's
// target and today's cap, (target - used) / days_left
// (docs/problems/search-throughput-keenable-underused-2026-10-15.md).
type SearchMonthlyPace struct {
	PeriodStart     string `json:"period_start"`
	PeriodEnd       string `json:"period_end"`
	UsedBeforeToday int    `json:"used_before_today"`
	Target          int    `json:"target"`
	DaysLeft        int    `json:"days_left"`
	DayCap          int    `json:"day_cap"`
}

type searchHealthResetRequest struct {
	Provider string `json:"provider"`
}

// HandleSearchHealthReset handles POST /provider/v1/search/health/reset.
func (h *Handler) HandleSearchHealthReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "operator route"})
		return
	}
	if h.searchClient == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "search not configured"})
		return
	}
	var req searchHealthResetRequest
	if err := h.decodeBody(w, r, bodyRouteOperator, "host", &req, false); err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Provider == "" {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "provider is required"})
		return
	}
	if err := h.searchClient.ResetProviderHealth(req.Provider); err != nil {
		writeGatewayJSON(w, http.StatusInternalServerError, ErrorResponse{Error: sanitizeError(err)})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "reset", "provider": req.Provider})
}

func rateLimitBucketKey(computerID, scope string) string {
	computerID = strings.TrimSpace(computerID)
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return computerID
	}
	return computerID + ":" + scope
}

// HandleProviderBreakers handles GET /provider/v1/breakers. It reports the
// circuit breaker state for each wrapped LLM provider so operators can see
// which upstreams are open/half-open/closed.
func (h *Handler) HandleProviderBreakers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "operator route"})
		return
	}
	if h.breakers == nil {
		writeGatewayJSON(w, http.StatusOK, map[string]any{"provider_breakers": map[string]any{}})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]any{"provider_breakers": h.breakers.Snapshot()})
}

// HandleProviderBreakerReset handles POST /provider/v1/breakers/reset. It
// forces a named provider's circuit breaker back to closed. The provider name
// is supplied as JSON {"provider":"<name>"}.
func (h *Handler) HandleProviderBreakerReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"})
		return
	}
	if !isAuthorizedCredentialCaller(r) {
		writeGatewayJSON(w, http.StatusForbidden, ErrorResponse{Error: "operator route"})
		return
	}
	if h.breakers == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "circuit breakers not configured"})
		return
	}
	var req searchHealthResetRequest
	if err := h.decodeBody(w, r, bodyRouteOperator, "host", &req, false); err != nil {
		if status, ok := bodyRefusal(w, err); ok {
			writeGatewayJSON(w, status, ErrorResponse{Error: err.Error()})
			return
		}
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}
	req.Provider = strings.TrimSpace(req.Provider)
	if req.Provider == "" {
		writeGatewayJSON(w, http.StatusBadRequest, ErrorResponse{Error: "provider is required"})
		return
	}
	if !h.breakers.Reset(req.Provider) {
		writeGatewayJSON(w, http.StatusNotFound, ErrorResponse{Error: "unknown provider breaker"})
		return
	}
	writeGatewayJSON(w, http.StatusOK, map[string]string{"status": "reset", "provider": req.Provider})
}

// RegisterRoutes registers all gateway routes on the given server.
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	// M22b / C20: per-service health endpoint. Public (no auth), exposes
	// only coarse status — never credentials or upstream bodies.
	s.HandleFunc("/health/{service}", h.HandleServiceHealth)
	s.HandleFunc("/provider/v1/inference", h.HandleInference)
	s.HandleFunc("/provider/v1/judgments", h.HandleJudgment)
	// SEC-16: host jobs (front-importance) reach Jev through the gateway
	// with the host-internal credential instead of holding the key.
	s.HandleFunc("/provider/v1/host/judgments", h.HandleHostJudgment)
	s.HandleFunc("/provider/openai/v1/chat/completions", h.HandleOpenAIChatCompletions)
	s.HandleFunc("/provider/openai/v1/models", h.HandleOpenAIModels)
	s.HandleFunc("/provider/v1/models", h.HandleModels)
	// SEC-08 Option K: the Claude delegate's Messages API calls, relayed with
	// the gateway-held API key. Dark unless a key, identities and a window
	// are configured (anthropic_relay.go).
	s.HandleFunc("/provider/anthropic/", h.HandleAnthropicRelay)
	s.HandleFunc("/provider/v1/search", h.HandleSearch)
	s.HandleFunc("/provider/v1/search/health", h.HandleSearchHealth)
	s.HandleFunc("/provider/v1/search/health/reset", h.HandleSearchHealthReset)
	s.HandleFunc("/provider/v1/source-api", h.HandleSourceAPI)
	s.HandleFunc(hostfetch.Path, h.HandleHostFetch)
	s.HandleFunc("/provider/v1/credentials/issue", h.HandleIssueCredential)
	s.HandleFunc("/provider/v1/credentials/ensure", h.HandleEnsureCredential)
	s.HandleFunc("/provider/v1/breakers", h.HandleProviderBreakers)
	s.HandleFunc("/provider/v1/breakers/reset", h.HandleProviderBreakerReset)
}
