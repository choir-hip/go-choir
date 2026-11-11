package provider

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// RiverProvider implements the Provider interface for River
// (https://docs.river.ai/guides/inference/), an OpenAI-compatible Chat
// Completions API. One key, RIVER_API_KEY, held only by the gateway (owner
// 2026-11-09: River for self-development while Synthetic DeepSeek is spent).
type RiverProvider struct {
	apiKey     string // loaded at init time, never logged
	modelID    string
	baseURL    string
	httpClient *http.Client
}

// RiverConfig holds configuration for creating a RiverProvider.
type RiverConfig struct {
	APIKey  string
	ModelID string
	BaseURL string // defaults to https://api.river.ai/v1
}

const riverDefaultBaseURL = "https://api.river.ai/v1"

// NewRiverProvider creates a River provider from the given config.
func NewRiverProvider(cfg RiverConfig) (*RiverProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("river provider requires api key")
	}
	modelID := strings.TrimSpace(cfg.ModelID)
	if modelID == "" {
		return nil, fmt.Errorf("river provider requires model_id")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = riverDefaultBaseURL
	}
	return &RiverProvider{
		apiKey:     apiKey,
		modelID:    modelID,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
	}, nil
}

// NewRiverProviderFromEnv creates a River provider from RIVER_API_KEY and the
// optional RIVER_BASE_URL.
func NewRiverProviderFromEnv(modelID string) (*RiverProvider, error) {
	p, err := NewRiverProvider(RiverConfig{
		APIKey:  os.Getenv("RIVER_API_KEY"),
		ModelID: modelID,
		BaseURL: os.Getenv("RIVER_BASE_URL"),
	})
	if err != nil {
		return nil, fmt.Errorf("river from env: %w", err)
	}
	return p, nil
}

func (p *RiverProvider) Name() string { return "river" }
func (p *RiverProvider) IsReal() bool { return true }

func (p *RiverProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	return p.call(ctx, req, func(StreamChunk) {})
}

func (p *RiverProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	req.Stream = true
	return p.call(ctx, req, onChunk)
}

func (p *RiverProvider) call(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "river", modelID, req)
	defer releaseMedia()
	if mediaErr != nil {
		return nil, fmt.Errorf("river: %w", mediaErr)
	}
	body := openAIChatCompletionRequest{
		Model:           modelID,
		Messages:        convertOpenAIChatMessages(req.System, req.Messages),
		Tools:           convertOpenAIChatTools(req.Tools),
		ToolChoice:      openAIChatToolChoice(req.ToolChoice),
		Stream:          req.Stream,
		ReasoningEffort: riverEffortFor(req),
	}
	if req.MaxTokens > 0 {
		maxTokens := req.MaxTokens
		body.MaxTokens = &maxTokens
	}
	httpReq, err := newJSONRequest(ctx, http.MethodPost, openCodeEndpoint(p.baseURL, "chat/completions"), body)
	if err != nil {
		return nil, fmt.Errorf("river: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "choir-gateway/0.1")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	log.Printf("provider: river call model=%s stream=%t reasoning=%s", modelID, req.Stream, body.ReasoningEffort)
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("river: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		reason := logRejectedRequest("river", resp)
		// 402 is spent credit; a 429 whose body names a quota or limit is a
		// spent allowance. Both cool the model at once. Any other 429 is a
		// short rate limit the guest retries on the same model.
		if resp.StatusCode == http.StatusPaymentRequired ||
			(resp.StatusCode == http.StatusTooManyRequests && reasonSaysQuota(reason)) {
			return nil, newQuotaError("river", resp)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, &upstreamConcurrencyError{provider: "river", status: resp.Status}
		}
		return nil, fmt.Errorf("river: status %s (sanitized)", resp.Status)
	}
	if !req.Stream {
		return parseOpenAIChatCompletionsResponse(resp, modelID, "river")
	}
	return parseOpenAIChatCompletionsStream(resp.Body, modelID, "river", onChunk)
}

// riverEffortFor is the effort one River call sends. River documents
// reasoning_effort "none" as off (and off as the DeepSeek default), so the
// floor at "low" below is ours, from the 2026-11-09 rule that thinking is never
// off, not River's. The autoradio role is the one exemption.
func riverEffortFor(req LLMRequest) string {
	if req.ReasoningOff {
		return "none"
	}
	return riverReasoningEffort(req.ReasoningEffort)
}

// riverReasoningEffort maps Choir's effort to what River accepts. Probed
// 2026-11-09 on deepseek-ai/DeepSeek-V4.1-Flash-1017: "low" and "high" work;
// "medium" with tools is a 400 invalid_request; omitting the field works
// with tools. So medium (and any unknown value) is omitted, which runs the
// model default, and xhigh and max collapse to high. Thinking is never off
// (owner, 2026-11-09), so the off spellings ask for low.
func riverReasoningEffort(effort string) string {
	switch strings.ToLower(strings.TrimSpace(effort)) {
	case "low", "none", "minimal", "off", "disabled", "false":
		return "low"
	case "high", "xhigh", "max":
		return "high"
	default:
		return ""
	}
}
