package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// OpenRouterProvider implements the Provider interface for OpenRouter's
// OpenAI-compatible Chat Completions API (owner 2026-11-03: a free week of
// stealth/crouton-alpha). One key, OPENROUTER_API_KEY, held only by the
// gateway; it is the key the Jev Decisions route already uses
// (docs/problems/openrouter-provider-crouton-alpha-2026-11-03.md).
type OpenRouterProvider struct {
	apiKey     string // loaded at init time, never logged
	modelID    string
	baseURL    string
	httpClient *http.Client
}

// OpenRouterConfig holds configuration for creating an OpenRouterProvider.
type OpenRouterConfig struct {
	APIKey  string
	ModelID string
	BaseURL string // defaults to https://openrouter.ai/api/v1
}

const openRouterDefaultBaseURL = "https://openrouter.ai/api/v1"

// NewOpenRouterProvider creates an OpenRouter provider from the given config.
func NewOpenRouterProvider(cfg OpenRouterConfig) (*OpenRouterProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("openrouter provider requires api key")
	}
	modelID := strings.TrimSpace(cfg.ModelID)
	if modelID == "" {
		return nil, fmt.Errorf("openrouter provider requires model_id")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = openRouterDefaultBaseURL
	}
	return &OpenRouterProvider{
		apiKey:     apiKey,
		modelID:    modelID,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
	}, nil
}

// NewOpenRouterProviderFromEnv creates an OpenRouter provider from
// OPENROUTER_API_KEY and the optional OPENROUTER_BASE_URL.
func NewOpenRouterProviderFromEnv(modelID string) (*OpenRouterProvider, error) {
	p, err := NewOpenRouterProvider(OpenRouterConfig{
		APIKey:  os.Getenv("OPENROUTER_API_KEY"),
		ModelID: modelID,
		BaseURL: os.Getenv("OPENROUTER_BASE_URL"),
	})
	if err != nil {
		return nil, fmt.Errorf("openrouter from env: %w", err)
	}
	return p, nil
}

func (p *OpenRouterProvider) Name() string { return "openrouter" }
func (p *OpenRouterProvider) IsReal() bool { return true }

func (p *OpenRouterProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	return p.call(ctx, req, func(StreamChunk) {})
}

func (p *OpenRouterProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	req.Stream = true
	return p.call(ctx, req, onChunk)
}

func (p *OpenRouterProvider) call(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "openrouter", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("openrouter: %w", mediaErr)
	}
	body := openAIChatCompletionRequest{
		Model:           modelID,
		Messages:        convertOpenAIChatMessages(req.System, req.Messages),
		Tools:           convertOpenAIChatTools(req.Tools),
		ToolChoice:      openAIChatToolChoice(req.ToolChoice),
		Stream:          req.Stream,
		ReasoningEffort: openRouterReasoningEffort(modelID, req.ReasoningEffort),
	}
	if req.MaxTokens > 0 {
		maxTokens := req.MaxTokens
		body.MaxTokens = &maxTokens
	}
	httpReq, err := newJSONRequest(ctx, http.MethodPost, openCodeEndpoint(p.baseURL, "chat/completions"), body)
	if err != nil {
		return nil, fmt.Errorf("openrouter: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "choir-gateway/0.1")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	log.Printf("provider: openrouter call model=%s stream=%t reasoning=%s", modelID, req.Stream, body.ReasoningEffort)
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason := logRejectedRequest("openrouter", resp)
		// 402 is spent credits; a 429 that names a quota or limit is a
		// spent allowance (a free model's daily cap). Both are quota: the
		// gateway cools the model at once. Any other status keeps the
		// sanitized status text the breaker reads.
		if resp.StatusCode == http.StatusPaymentRequired ||
			(resp.StatusCode == http.StatusTooManyRequests && openRouterRateLimitIsQuota(reason)) {
			return nil, newQuotaError("openrouter", resp)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			// A short rate limit (the per-minute cap): the guest retries
			// the same model, and the breaker counts it only in a long
			// streak, as Synthetic's concurrency 429. The arm pins one
			// model, so a burst must not open its breaker (review r1b).
			return nil, &upstreamConcurrencyError{provider: "openrouter", status: resp.Status}
		}
		return nil, fmt.Errorf("openrouter: status %s (sanitized)", resp.Status)
	}
	if !req.Stream {
		return parseOpenAIChatCompletionsResponse(resp, modelID, "openrouter")
	}
	return parseOpenAIChatCompletionsStream(resp.Body, modelID, "openrouter", onChunk)
}

// openRouterRateLimitIsQuota reports whether an OpenRouter 429 is a spent
// allowance: the shared quota markers, or OpenRouter's daily free-model cap
// (code "free-models-per-day"). Its per-minute cap ("free-models-per-min")
// clears in seconds and is not quota, though its message also offers
// credits and names a daily allowance.
func openRouterRateLimitIsQuota(reason string) bool {
	r := strings.ToLower(reason)
	if strings.Contains(r, "per-min") {
		return false
	}
	return reasonSaysQuota(r) || strings.Contains(r, "per-day")
}

// openRouterReasoningEffort maps Choir's effort to the values the models
// listing advertises for the model. stealth/crouton-alpha (2026-11-03: max,
// xhigh, high, medium, low, minimal) makes reasoning mandatory, so "none" is
// a 400 ("cannot be disabled") and the disable spellings map to minimal.
// deepseek/deepseek-v4.1-flash-1017 (2026-11-04: max, high, low, none;
// optional) takes Synthetic's meaning for the same snapshot: the disable
// spellings and minimal are none, medium is high, and xhigh, which this
// listing lacks, is max. An unknown value is a 400 on either, so it is
// omitted (model default).
func openRouterReasoningEffort(model, effort string) string {
	e := strings.ToLower(strings.TrimSpace(effort))
	if strings.HasPrefix(strings.ToLower(model), "deepseek/") {
		switch e {
		case "none", "low", "high", "max":
			return e
		case "minimal", "off", "disabled", "false":
			return "none"
		case "medium":
			return "high"
		case "xhigh":
			return "max"
		default:
			return ""
		}
	}
	switch e {
	case "minimal", "low", "medium", "high", "xhigh", "max":
		return e
	case "none", "off", "disabled", "false":
		return "minimal"
	default:
		return ""
	}
}
