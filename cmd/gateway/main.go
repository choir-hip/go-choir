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

	// Initialize the identity registry for sandbox credential management.
	registry := gateway.NewIdentityRegistry(cfg.SandboxTokenTTL)
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
	// sandbox callers or browsers (VAL-GATEWAY-004).
	providerCfg := loadProviderConfig()
	mp := provider.ResolveAll(providerCfg)
	providerNames := mp.Names()

	// Circuit breaker config for LLM provider calls. Repeated upstream
	// failures open the breaker so the gateway stops forwarding into a
	// failing provider instead of retrying endlessly (production-readiness
	// checklist: "LLM provider failures circuit-break").
	breakerCfg := health.BreakerConfig{
		FailureThreshold:   5,
		OpenTimeout:        30 * time.Second,
		HalfOpenMaxProbes:  1,
	}
	breakers := gateway.NewBreakerRegistry()

	var handler *gateway.Handler

	if len(providerNames) > 0 {
		log.Printf("gateway: resolved %d provider(s): %v", len(providerNames), providerNames)

		// Wrap each resolved provider with a circuit breaker.
		wrapped := gateway.WrapMultiProvider(mp, breakerCfg)
		for _, name := range wrapped.Names() {
			if cbp, ok := wrapped.Get(name).(*gateway.CircuitBreakingProvider); ok {
				breakers.Register(name, cbp.Breaker())
			}
		}
		log.Printf("gateway: circuit breakers enabled for %d provider(s)", len(breakers.Names()))

		// Initialize per-sandbox rate limiting (VAL-GATEWAY-005).
		rlCfg := gateway.LoadRateLimiterConfig()
		rl := gateway.NewPerSandboxRateLimiter(rlCfg.MaxRequests, rlCfg.WindowSize)
		log.Printf("gateway: rate limiter enabled: %s", rl)

		handler = gateway.NewMultiHandlerWithRateLimit(registry, wrapped, rl)
	} else {
		log.Printf("gateway: no real provider configured; inference requests will fail")

		// Fall back to single-provider mode with nil provider.
		rlCfg := gateway.LoadRateLimiterConfig()
		rl := gateway.NewPerSandboxRateLimiter(rlCfg.MaxRequests, rlCfg.WindowSize)
		handler = gateway.NewHandlerWithRateLimit(registry, nil, rl)
	}

	handler.SetBreakers(breakers)

	// M22b / C20: wire per-service health checkers so GET /health/{service}
	// can probe backend dependencies (sourcecycled, runtime, qdrant, dolt,
	// ollama) from outside the gateway. The checkers are lightweight HTTP
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
		ZAIModels: []string{"glm-5.2", "glm-5.1", "glm-5-turbo"},
		DeepSeekModels: []string{
			"deepseek-v4-flash",
			"deepseek-v4-pro",
		},
		DeepSeekAnthropicModels: []string{
			"deepseek-v4-flash",
			"deepseek-v4-pro",
		},
		XiaomiModels: []string{
			"mimo-v2.5",
			"mimo-v2.5-pro",
		},
		XiaomiAnthropicModels: []string{
			"mimo-v2.5",
			"mimo-v2.5-pro",
		},
		FireworksModels: []string{
			"accounts/fireworks/models/kimi-k2p6",
		},
		DeepSeekReasoningEffort:  "",
		XiaomiReasoningEffort:    "",
		FireworksReasoningEffort: "medium",
		ChatGPTModels:            []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini"},
		ChatGPTReasoningEffort:   "low",
	}

	// Allow overrides for non-default setups.
	if v := os.Getenv("GATEWAY_BEDROCK_MODELS"); v != "" {
		cfg.BedrockModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_ZAI_MODELS"); v != "" {
		cfg.ZAIModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_DEEPSEEK_MODELS"); v != "" {
		cfg.DeepSeekModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_DEEPSEEK_ANTHROPIC_MODELS"); v != "" {
		cfg.DeepSeekAnthropicModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_DEEPSEEK_REASONING_EFFORT"); v != "" {
		cfg.DeepSeekReasoningEffort = v
	}
	if v := os.Getenv("GATEWAY_XIAOMI_MODELS"); v != "" {
		cfg.XiaomiModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_XIAOMI_ANTHROPIC_MODELS"); v != "" {
		cfg.XiaomiAnthropicModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_XIAOMI_REASONING_EFFORT"); v != "" {
		cfg.XiaomiReasoningEffort = v
	}
	if v := os.Getenv("GATEWAY_FIREWORKS_MODELS"); v != "" {
		cfg.FireworksModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_FIREWORKS_REASONING_EFFORT"); v != "" {
		cfg.FireworksReasoningEffort = v
	}
	if v := os.Getenv("GATEWAY_CHATGPT_MODELS"); v != "" {
		cfg.ChatGPTModels = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_CHATGPT_REASONING_EFFORT"); v != "" {
		cfg.ChatGPTReasoningEffort = v
	}

	return cfg
}
