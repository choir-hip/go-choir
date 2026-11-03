package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/server"
)

func main() {
	cfg := gateway.LoadConfig()

	s := server.NewServer("gateway", cfg.Port)

	// Initialize the identity registry for autoputer credential management.
	registry := gateway.NewIdentityRegistry(cfg.AutoputerTokenTTL)
	if cfg.IdentityStorePath != "" {
		if err := registry.SetPersistencePath(cfg.IdentityStorePath); err != nil {
			log.Fatalf("gateway: load identity store: %v", err)
		}
		log.Printf("gateway: identity persistence enabled (%s)", cfg.IdentityStorePath)
	}

	// Resolve configured real providers from environment credentials.
	// using the MultiProvider for multi-provider routing. The gateway
	// routes requests to the correct provider based on the provider field
	// or model parameter (VAL-LLM-001, VAL-LLM-005).
	// Provider credentials remain host-side and are never exposed to
	// autoputer callers or browsers (VAL-GATEWAY-004).
	providerCfg := loadProviderConfig()
	mp := provider.ResolveAll(providerCfg)
	providerNames := mp.Names()

	// Circuit breaker config for LLM provider calls. Repeated upstream
	// failures open the breaker so the gateway stops forwarding into a
	// failing provider instead of retrying endlessly (production-readiness
	// checklist: "LLM provider failures circuit-break").
	breakerCfg := health.BreakerConfig{
		FailureThreshold:  5,
		OpenTimeout:       30 * time.Second,
		HalfOpenMaxProbes: 1,
	}
	breakers := gateway.NewBreakerRegistry()

	var handler *gateway.Handler

	if len(providerNames) > 0 {
		log.Printf("gateway: resolved %d provider(s): %v", len(providerNames), providerNames)

		// Wrap each resolved provider with a circuit breaker.
		wrapped := gateway.WrapMultiProvider(mp, breakerCfg)
		for _, name := range wrapped.Names() {
			if cbp, ok := wrapped.Get(name).(*gateway.CircuitBreakingProvider); ok {
				breakers.RegisterProvider(name, cbp)
			}
		}
		log.Printf("gateway: circuit breakers enabled for %d provider(s)", len(breakers.Names()))

		// Initialize per-autoputer rate limiting (VAL-GATEWAY-005).
		rlCfg := gateway.LoadRateLimiterConfig()
		rl := gateway.NewPerAutoputerRateLimiter(rlCfg.MaxRequests, rlCfg.WindowSize)
		log.Printf("gateway: rate limiter enabled: %s", rl)

		handler = gateway.NewMultiHandlerWithRateLimit(registry, wrapped, rl)
	} else {
		log.Printf("gateway: no real provider configured; inference requests will fail")

		// Fall back to single-provider mode with nil provider.
		rlCfg := gateway.LoadRateLimiterConfig()
		rl := gateway.NewPerAutoputerRateLimiter(rlCfg.MaxRequests, rlCfg.WindowSize)
		handler = gateway.NewHandlerWithRateLimit(registry, nil, rl)
	}

	handler.SetBreakers(breakers)

	// SEC-05: bind each computer's bearer to its own address. Unset and
	// unknown values enforce; nix sets observe while the rollout is watched
	// (docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md).
	bindingMode := gateway.LoadBindingMode()
	handler.SetBearerBinding(bindingMode)
	log.Printf("gateway: bearer binding mode: %s", bindingMode)

	// RES-11: a fair model-call scheduler in place of the fixed window. Off
	// unless GATEWAY_MODEL_SCHEDULER is on.
	if gateway.ModelSchedulerEnabled(os.Getenv) {
		schedulerCfg := gateway.ModelSchedulerConfigFromGetenv(os.Getenv)
		handler.SetModelScheduler(gateway.NewModelScheduler(schedulerCfg))
		log.Printf("gateway: model scheduler on: %+v", schedulerCfg)
	} else {
		log.Printf("gateway: model scheduler off (fixed-window limiter)")
	}

	// SEC-08 Option K: the Claude delegate's key, held here. Dark unless the
	// owner's Console API key, the relay identities and a window are all set;
	// the status line names none of them.
	relay := gateway.NewAnthropicRelayFromEnv()
	handler.SetAnthropicRelay(relay)
	log.Printf("gateway: anthropic relay: %s", relay.Status())
	log.Printf("gateway: source api: %s", handler.SourceAPIStatus())
	// B7: request body caps, read from GATEWAY_BODY_CAPS and
	// GATEWAY_BODY_LIMITS when the handler was built.
	log.Printf("gateway: body caps: %s", handler.BodyCapsStatus())

	// The Jev Decisions route is independently gated. Its OpenRouter key stays
	// in the gateway environment; disabled transport never falls back to an
	// unpinned model or a direct TypeSafe endpoint.
	handler.SetJevTransport(gateway.NewJevTransportFromEnv(cfg.JevJudgmentsEnabled))

	// M22b / C20: wire per-service health checkers so GET /health/{service}
	// can probe the live host dependencies (gateway.DefaultServiceHealthURLs)
	// from outside the gateway. The checkers are lightweight HTTP
	// probes with a 2s timeout; they expose no secrets. The same checkers
	// feed the /health/ready aggregator so overall readiness reflects
	// backend dependency health, not just gateway liveness.
	serviceCheckers := buildServiceCheckers(cfg.ServiceHealthURLs)
	handler.SetServiceCheckers(serviceCheckers)

	gateway.RegisterRoutes(s, handler)

	// Readiness endpoint: reports gateway dependency health by aggregating
	// the per-service checkers. A cold cache runs the probes synchronously;
	// subsequent calls within the 5s TTL are served from cache so the
	// endpoint stays lightweight. Degraded (some deps down) returns 200;
	// only fully unhealthy returns 503.
	readyCheckers := make([]health.Checker, 0, len(serviceCheckers))
	for _, c := range serviceCheckers {
		readyCheckers = append(readyCheckers, c)
	}
	s.HandleFunc("/health/ready", health.ReadinessHandler("gateway", health.NewAggregator("gateway", 5*time.Second, readyCheckers...)))

	s.Start()
}

// buildServiceCheckers constructs the per-service health.Checker map used by
// GET /health/{service} and the /health/ready aggregator. Each entry is an
// HTTPChecker with a 2s timeout pointing at the configured probe URL for the
// service. Services with an empty URL are skipped (the endpoint reports
// "not configured" for them). The checkers expose no secrets: they only
// verify reachability via a 2xx response (M22b / C20).
func buildServiceCheckers(urls map[string]string) map[string]health.Checker {
	out := make(map[string]health.Checker, len(urls))
	for name, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		out[name] = health.HTTPChecker{
			NameStr: name,
			URL:     raw,
			Timeout: 2 * time.Second,
		}
	}
	return out
}

// loadProviderConfig builds a ProviderConfig from environment variables.
// Model selection is a runtime concern resolved here at the gateway entry
// point, not inside the provider package. Credentials remain in env vars
// and are resolved by the provider FromEnv functions.
func loadProviderConfig() provider.ProviderConfig {
	cfg := provider.ProviderConfig{
		BedrockModels: []string{
			"us.anthropic.claude-haiku-4-5-20251001-v1:0",
			"us.anthropic.claude-sonnet-4-6",
			"us.anthropic.claude-opus-4-6-v1",
		},
		ZAIModels:              []string{"glm-5.2", "glm-5.1", "glm-5-turbo"},
		ChatGPTModels:          []string{"gpt-6-luna"},
		ChatGPTReasoningEffort: "low",
		OpenCodeGoModels: []string{
			"deepseek-v4.1-flash",
			"glm-5.3-flash",
			"muse-spark-1.3-contributor",
			"hy3",
			"qwen3.7-max",
			"cat-jam-free",
			"longcat-2.5-preview-free",
			"step-5-preview-free",
		},
		OpenCodeZenModels: []string{"muse-spark-1.3-contributor-free"},
		SyntheticModels:   []string{"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"},
	}

	// Allow overrides for non-default setups.
	if v := os.Getenv("GATEWAY_BEDROCK_MODELS"); v != "" {
		cfg.BedrockModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_ZAI_MODELS"); v != "" {
		cfg.ZAIModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_CHATGPT_MODELS"); v != "" {
		cfg.ChatGPTModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_CHATGPT_REASONING_EFFORT"); v != "" {
		cfg.ChatGPTReasoningEffort = v
	}
	if v := os.Getenv("GATEWAY_OPENCODE_GO_MODELS"); v != "" {
		cfg.OpenCodeGoModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_OPENCODE_ZEN_MODELS"); v != "" {
		cfg.OpenCodeZenModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_SYNTHETIC_MODELS"); v != "" {
		cfg.SyntheticModels = strings.Split(v, ",")
	}

	return cfg
}
