// Package provider implements real LLM provider bridges for the go-choir
// autoputer runtime. It supports Bedrock (Anthropic Messages API over AWS
// Bedrock invoke endpoint), Z.AI (Anthropic-compatible API), Fireworks AI
// (OpenAI-compatible Chat Completions API), ChatGPT, and OpenCode.
//
// Provider credentials are read from environment variables only; they are
// never committed to the repo, baked into the Nix store, or exposed to the
// browser or guest.
package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
)

// LLMRequest is the unified request shape for all provider backends.
type LLMRequest struct {
	// Provider is the provider identifier (e.g. "chatgpt", "zai",
	// "fireworks", "bedrock") for gateway-routed requests.
	Provider string `json:"provider,omitempty"`
	// ConversationID is the durable calling-run identity used by providers
	// that require request affinity/session headers.
	ConversationID string `json:"conversation_id,omitempty"`
	// CacheKey is the caller's prompt-cache thread key: one desk on one
	// computer at one prompt version, shared across its runs. Each adapter
	// uses it only where its provider documents a client cache-routing key
	// (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md): chatgpt sends it as
	// prompt_cache_key, OpenCode as x-opencode-session. Others ignore it.
	CacheKey string `json:"cache_key,omitempty"`

	// Model is the model identifier (provider-specific).
	Model string `json:"model"`

	// System is the optional system prompt.
	System string `json:"system,omitempty"`

	// Messages is the conversation history in Anthropic Messages format.
	Messages []Message `json:"messages"`

	// Tools is the list of tool definitions to include in the request.
	// When non-empty, the provider may return tool_use content blocks.
	Tools []ToolDef `json:"tools,omitempty"`

	// ToolChoice optionally constrains tool selection for compatible
	// providers. The shared string modes are "auto", "none", and "required";
	// "function:<name>" is the gateway/runtime wire form for an exact tool
	// choice, translated by OpenAI-compatible adapters into their provider
	// object shape.
	ToolChoice string `json:"tool_choice,omitempty"`

	// MaxTokens is the maximum number of output tokens.
	MaxTokens int `json:"max_tokens"`

	// Stream controls whether to use streaming (SSE) or non-streaming.
	// Bedrock forces this to false because it uses binary EventStream.
	Stream bool `json:"stream,omitempty"`

	// ReasoningEffort is an optional provider-specific reasoning control.
	// For ChatGPT/OpenAI Responses this maps to reasoning.effort.
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// Message is a single message in the conversation history.
type Message struct {
	Role             string  `json:"role"`
	Content          []Block `json:"content"`
	ReasoningContent string  `json:"reasoning_content,omitempty"`
}

// ToolDef is a tool definition for inclusion in LLM requests. The
// provider package uses this instead of importing from the runtime
// package to avoid circular dependencies.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"` // Anthropic uses "input_schema"
}

// Block is a content block within a message. It supports text, image,
// tool_use, and tool_result content. Image blocks carry a MediaSource so the
// runtime can preserve evidence provenance while adapters serialize to the
// provider-specific wire format.
// The fields are structured so that each block type uses its relevant
// subset:
//   - text: Type="text", Text=content
//   - image: Type="image", Source={Kind:"base64"|"url"|"artifact_ref", ...}
//   - tool_use: Type="tool_use", ID=call_id, Name=tool_name, Input=args
//   - tool_result: Type="tool_result", ToolUseID=call_id, Text=result_content
type Block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *MediaSource    `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`          // tool_use: provider-assigned call ID
	Name      string          `json:"name,omitempty"`        // tool_use: tool name to invoke
	Input     json.RawMessage `json:"input,omitempty"`       // tool_use: tool arguments as JSON
	ToolUseID string          `json:"tool_use_id,omitempty"` // tool_result: ID of the originating tool call
	IsError   bool            `json:"is_error,omitempty"`    // tool_result: true if result is an error
}

// MediaSource is the provider-neutral image/media source representation. The
// gateway/runtime should resolve artifact_ref sources into URL or base64 before
// the final provider call.
type MediaSource struct {
	Kind     string `json:"kind"` // base64, url, artifact_ref
	MIMEType string `json:"mime_type,omitempty"`
	Data     string `json:"data,omitempty"`
	URL      string `json:"url,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

// LLMResponse is the unified response shape from all provider backends.
type LLMResponse struct {
	// ID is the provider-assigned response identifier.
	ID string `json:"id"`

	// Text is the concatenated text content from the response.
	Text string `json:"text"`

	// ReasoningContent is provider-returned hidden reasoning state that some
	// OpenAI-compatible reasoning models require on subsequent tool-loop turns.
	// It is provider context, not user-facing answer text.
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// Model is the model that produced the response (may differ from request
	// if the provider resolved an alias).
	Model string `json:"model"`

	// StopReason is why the model stopped generating.
	StopReason string `json:"stop_reason"`

	// Usage contains token usage information.
	Usage Usage `json:"usage"`

	// ToolCalls contains structured tool invocation requests extracted from
	// tool_use content blocks in the provider response. Non-empty only when
	// StopReason is "tool_use". This field bridges the gap between the
	// Anthropic Messages API response format (where tool calls appear as
	// content blocks with type "tool_use") and the runtime's ToolCall type.
	ToolCalls []ContentToolCall `json:"tool_calls,omitempty"`

	// ProviderName identifies which provider handled this request, for
	// observability and redacted logging.
	ProviderName string `json:"provider_name"`
}

// Usage tracks token counts for a provider response.
type Usage struct {
	// InputTokens is the whole prompt, cached and uncached.
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`

	// CachedInputTokens is the part of InputTokens the provider served from
	// its prompt cache (billed at the cached-input rate). Always <= InputTokens.
	CachedInputTokens int `json:"cached_input_tokens,omitempty"`

	// CacheReported is true when the provider's usage carried a cache field
	// at all, so "0 cached" (a miss) is distinguishable from "not reported".
	CacheReported bool `json:"cache_reported,omitempty"`
}

// normalizeCache clamps CachedInputTokens to the prompt size.
func (u Usage) normalizeCache() Usage {
	if u.CachedInputTokens < 0 {
		u.CachedInputTokens = 0
	}
	if u.CachedInputTokens > u.InputTokens {
		u.CachedInputTokens = u.InputTokens
	}
	return u
}

func (u Usage) streamUsage() *StreamUsage {
	return &StreamUsage{
		InputTokens:       u.InputTokens,
		OutputTokens:      u.OutputTokens,
		CachedInputTokens: u.CachedInputTokens,
		CacheReported:     u.CacheReported,
	}
}

// ContentToolCall represents a tool invocation extracted from a tool_use
// content block in an Anthropic Messages API response. This is the
// provider-package-level representation; the bridge layer converts these
// into runtime.ToolCall values consumed by the tool-calling loop.
type ContentToolCall struct {
	// ID is the provider-assigned tool call identifier (e.g., "toolu_01").
	ID string `json:"id"`

	// Name is the tool name to invoke (must match a registered tool).
	Name string `json:"name"`

	// Arguments is the raw JSON arguments object for the tool call.
	Arguments json.RawMessage `json:"arguments"`
}

// StreamChunk represents an incremental SSE chunk from a streaming LLM
// response. The gateway and runtime use these to forward streaming data
// to callers in real-time.
type StreamChunk struct {
	// Type is the SSE event type (e.g., "content_block_delta", "message_start",
	// "message_delta", "message_stop").
	Type string `json:"type"`

	// Delta is the incremental text content from text_delta events.
	Delta string `json:"delta,omitempty"`

	// ReasoningDelta is provider-returned hidden reasoning state. Consumers may
	// accumulate it for provider context, but should not render it as user text.
	ReasoningDelta string `json:"reasoning_delta,omitempty"`

	// ToolCallDelta is partial JSON for tool_use input_json_delta events.
	ToolCallDelta string `json:"tool_call_delta,omitempty"`

	// ToolCallID is the tool call ID from content_block_start for tool_use blocks.
	ToolCallID string `json:"tool_call_id,omitempty"`

	// ToolCallName is the tool name from content_block_start for tool_use blocks.
	ToolCallName string `json:"tool_call_name,omitempty"`

	// StopReason is set on message_delta events (e.g., "end_turn", "tool_use").
	StopReason string `json:"stop_reason,omitempty"`

	// Usage is the cumulative token usage from message_delta events.
	Usage *StreamUsage `json:"usage,omitempty"`

	// Index is the content block index for multi-block responses.
	Index int `json:"index,omitempty"`

	// Model is set on message_start events.
	Model string `json:"model,omitempty"`

	// ID is the response ID from message_start events.
	ID string `json:"id,omitempty"`
}

// StreamUsage carries token counts for streaming responses.
type StreamUsage struct {
	InputTokens       int  `json:"input_tokens"`
	OutputTokens      int  `json:"output_tokens"`
	CachedInputTokens int  `json:"cached_input_tokens,omitempty"`
	CacheReported     bool `json:"cache_reported,omitempty"`
}

// Provider is the interface for executing LLM requests. Implementations
// must not leak credentials in error messages or logs.
type Provider interface {
	// Call executes the LLM request and returns the response.
	// Implementations should return structured errors that the runtime
	// can surface as task failures without crashing (VAL-RUNTIME-008).
	Call(ctx context.Context, req LLMRequest) (*LLMResponse, error)

	// Stream executes the LLM request with streaming (SSE) and calls
	// onChunk for each incremental chunk. If the provider does not support
	// streaming, it should fall back to Call and emit a single chunk.
	// Returns the accumulated LLMResponse on completion.
	Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error)

	// Name returns the provider name for observability.
	Name() string

	// IsReal returns true for providers that reach a real upstream
	// backend, as opposed to stub/canned providers.
	IsReal() bool
}

// ProviderConfig holds runtime configuration for which model each provider
// backend should use. This is the configuration surface that the host-side
// caller (gateway, direct autoputer) uses to select models. It is NOT resolved
// from environment variables — model selection is a runtime concern, not an
// infrastructure one.
//
// Provider credentials (API keys, tokens) are still resolved from env vars
// by the FromEnv functions. This struct only governs model selection.
type ProviderConfig struct {
	// BedrockModels lists Bedrock model IDs (e.g.,
	// "us.anthropic.claude-sonnet-4-5-20250514-v1:0"). The first entry seeds
	// the provider instance; request.Model still controls per-call selection.
	// If empty, Bedrock is not initialized even if credentials are available.
	BedrockModels []string

	// ZAIModels lists Z.AI model IDs (e.g., "glm-5.1", "glm-5-turbo").
	// The first entry seeds the provider instance; request.Model still
	// controls per-call selection. If empty, Z.AI is not initialized even if
	// ZAI_API_KEY is set.
	ZAIModels []string

	// FireworksModels lists Fireworks model IDs (e.g.,
	// "accounts/fireworks/models/deepseek-v4-flash"). The first entry seeds
	// the provider instance; request.Model still controls per-call selection.
	// If empty, Fireworks is not initialized even if FIREWORKS_API_KEY is set.
	FireworksModels []string

	// FireworksReasoningEffort seeds Fireworks reasoning effort for requests
	// that do not carry a per-request value.
	FireworksReasoningEffort string

	// ChatGPTModels lists ChatGPT/Codex Responses model IDs. The first entry
	// seeds the provider instance; request.Model still controls per-call
	// selection. If empty, ChatGPT is not initialized even if Codex OAuth auth
	// is available.
	ChatGPTModels []string

	// ChatGPTReasoningEffort seeds the ChatGPT provider's reasoning effort for
	// requests that do not carry a per-request value.
	ChatGPTReasoningEffort string
	// OpenCodeGoModels lists OpenCode Go model IDs. The first entry seeds the
	// provider instance; request.Model controls per-call selection.
	OpenCodeGoModels []string

	// OpenCodeZenModels lists OpenCode Zen model IDs. The first entry seeds the
	// provider instance; request.Model controls per-call selection.
	OpenCodeZenModels []string

	// SyntheticModels lists Synthetic model IDs. The first entry seeds the
	// provider instance; request.Model controls per-call selection. If empty,
	// Synthetic is not initialized even if SYNTHETIC_API_KEY is set.
	SyntheticModels []string

	// SelectedProvider is the explicitly selected provider for direct autoputer
	// runtime calls. Empty means no direct provider is selected.
	SelectedProvider string
}

const defaultProviderHTTPTimeout = 10 * time.Minute

// BedrockProvider implements the Provider interface for AWS Bedrock using
// the Anthropic Messages API format over the Bedrock invoke endpoint.
// Auth uses a bearer identity token (AWS_BEARER_TOKEN_BEDROCK) rather
// than SigV4 signing, matching the pattern from choiros-rs.
type BedrockProvider struct {
	region     string
	modelID    string
	authToken  string // loaded at init time, never logged
	httpClient *http.Client
	anthropicV string // anthropic version header
}

// BedrockConfig holds configuration for creating a BedrockProvider.
type BedrockConfig struct {
	Region    string
	ModelID   string
	AuthToken string
}

// NewBedrockProvider creates a Bedrock provider from the given config.
// Returns an error if required fields are missing.
func NewBedrockProvider(cfg BedrockConfig) (*BedrockProvider, error) {
	if cfg.Region == "" {
		return nil, fmt.Errorf("bedrock provider requires region")
	}
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("bedrock provider requires model_id")
	}
	if cfg.AuthToken == "" {
		return nil, fmt.Errorf("bedrock provider requires auth token")
	}

	return &BedrockProvider{
		region:     cfg.Region,
		modelID:    cfg.ModelID,
		authToken:  cfg.AuthToken,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
		anthropicV: "bedrock-2023-05-31",
	}, nil
}

// NewBedrockProviderFromEnv creates a Bedrock provider using credentials
// from environment variables (AWS_REGION, AWS_BEARER_TOKEN_BEDROCK) and
// the given model ID. The model is a runtime concern, not an env var.
func NewBedrockProviderFromEnv(modelID string) (*BedrockProvider, error) {
	region := os.Getenv("AWS_REGION")
	token := os.Getenv("AWS_BEARER_TOKEN_BEDROCK")

	p, err := NewBedrockProvider(BedrockConfig{
		Region:    region,
		ModelID:   modelID,
		AuthToken: token,
	})
	if err != nil {
		return nil, fmt.Errorf("bedrock from env: %w", err)
	}
	return p, nil
}

func (p *BedrockProvider) Name() string { return "bedrock" }
func (p *BedrockProvider) IsReal() bool { return true }

// Stream falls back to non-streaming Call because Bedrock uses binary
// EventStream (not SSE). The response is emitted as a single chunk.
func (p *BedrockProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	resp, err := p.Call(ctx, req)
	if err != nil {
		return nil, err
	}
	// Emit the complete response as a single chunk for consistency.
	onChunk(StreamChunk{
		Type:       "message_start",
		ID:         resp.ID,
		Model:      resp.Model,
		StopReason: resp.StopReason,
		Usage:      &StreamUsage{InputTokens: resp.Usage.InputTokens, OutputTokens: resp.Usage.OutputTokens},
	})
	if resp.Text != "" {
		onChunk(StreamChunk{
			Type:  "content_block_delta",
			Delta: resp.Text,
			Index: 0,
		})
	}
	onChunk(StreamChunk{
		Type:       "message_stop",
		StopReason: resp.StopReason,
	})
	return resp, nil
}

// Call sends the request to Bedrock's invoke endpoint.
// Bedrock uses the model ID in the URL path and forces non-streaming
// because streaming uses binary EventStream, not SSE.
func (p *BedrockProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "bedrock", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("bedrock: %w", mediaErr)
	}
	endpoint := fmt.Sprintf(
		"https://bedrock-runtime.%s.amazonaws.com/model/%s/invoke",
		p.region, pathEscape(modelID),
	)

	// Build Anthropic Messages API request body.
	body := p.buildRequestBody(req)

	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("bedrock: build request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.authToken)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("anthropic-version", p.anthropicV)

	log.Printf("provider: bedrock call model=%s region=%s", redactModel(modelID), p.region)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("bedrock: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseBedrockResponse(resp, modelID)
}

func (p *BedrockProvider) buildRequestBody(req LLMRequest) anthropicRequest {
	ar := anthropicRequest{
		MaxTokens:        defaultMaxTokens(req.MaxTokens),
		AnthropicVersion: p.anthropicV,
	}

	// Bedrock: system prompt with cache_control for prompt caching.
	if req.System != "" {
		ar.System = []anthropicSystemBlock{{
			Type:         "text",
			Text:         req.System,
			CacheControl: map[string]string{"type": "ephemeral"},
		}}
	}

	ar.Messages = convertMessages(req.Messages)
	// Bedrock caches only at explicit breakpoints: the system one above
	// (tools + system) and this one (the history so far).
	markHistoryCacheBreakpoint(ar.Messages)
	ar.Tools = convertToolDefs(req.Tools)
	return ar
}

// ZAIProvider implements the Provider interface for Z.AI using the
// Anthropic-compatible API at https://api.z.ai/api/anthropic.
type ZAIProvider struct {
	apiKey     string // loaded at init time, never logged
	modelID    string
	httpClient *http.Client
	baseURL    string
}

// ZAIConfig holds configuration for creating a ZAIProvider.
type ZAIConfig struct {
	APIKey  string
	ModelID string
	BaseURL string // defaults to https://api.z.ai/api/anthropic
}

// NewZAIProvider creates a Z.AI provider from the given config.
func NewZAIProvider(cfg ZAIConfig) (*ZAIProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("zai provider requires api key")
	}
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("zai provider requires model_id")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.z.ai/api/anthropic"
	}

	return &ZAIProvider{
		apiKey:     cfg.APIKey,
		modelID:    cfg.ModelID,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
	}, nil
}

// NewZAIProviderFromEnv creates a Z.AI provider using credentials from
// environment variables (ZAI_API_KEY) and the given model ID. The model
// is a runtime concern, not an env var. The base URL defaults to the GLM
// Coding Plan Anthropic-compatible endpoint https://api.z.ai/api/anthropic
// unless ZAI_BASE_URL is set.
func NewZAIProviderFromEnv(modelID string) (*ZAIProvider, error) {
	apiKey := os.Getenv("ZAI_API_KEY")
	baseURL := os.Getenv("ZAI_BASE_URL")

	p, err := NewZAIProvider(ZAIConfig{
		APIKey:  apiKey,
		ModelID: modelID,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("zai from env: %w", err)
	}
	return p, nil
}

func (p *ZAIProvider) Name() string { return "zai" }
func (p *ZAIProvider) IsReal() bool { return true }

// Call sends the request to Z.AI's Anthropic-compatible endpoint.
func (p *ZAIProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	endpoint := p.baseURL + "/v1/messages"
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "zai", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("zai: %w", mediaErr)
	}

	body := p.buildRequestBody(req, modelID)

	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("zai: build request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	log.Printf("provider: zai call model=%s", modelID)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("zai: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseAnthropicResponse(resp, modelID, "zai")
}

// Stream sends the request to Z.AI with stream=true and processes the
// SSE response. It parses each event and calls onChunk for incremental
// text deltas, tool use, and lifecycle events. Returns the accumulated
// LLMResponse on completion.
func (p *ZAIProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	endpoint := p.baseURL + "/v1/messages"
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "zai", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("zai: %w", mediaErr)
	}

	// Build streaming request.
	req.Stream = true
	body := p.buildRequestBody(req, modelID)
	body.Stream = true

	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("zai: build stream request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	log.Printf("provider: zai stream model=%s", modelID)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("zai: stream http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zai: status %s (sanitized)", resp.Status)
	}

	return parseSSEStream(resp.Body, modelID, "zai", onChunk)
}

func (p *ZAIProvider) buildRequestBody(req LLMRequest, modelID string) anthropicRequest {
	ar := anthropicRequest{
		Model:            modelID,
		MaxTokens:        defaultMaxTokens(req.MaxTokens),
		AnthropicVersion: "2023-06-01",
		Stream:           false,
	}

	if req.System != "" {
		ar.System = []anthropicSystemBlock{{
			Type: "text",
			Text: req.System,
		}}
	}

	ar.Messages = convertMessages(req.Messages)
	ar.Tools = convertToolDefs(req.Tools)
	return ar
}

// FireworksProvider implements the Provider interface for Fireworks AI using
// the OpenAI-compatible Chat Completions API.
type FireworksProvider struct {
	apiKey     string // loaded at init time, never logged
	modelID    string
	reasoning  string
	httpClient *http.Client
	baseURL    string
}

// FireworksConfig holds configuration for creating a FireworksProvider.
type FireworksConfig struct {
	APIKey          string
	ModelID         string
	ReasoningEffort string
	BaseURL         string // defaults to https://api.fireworks.ai/inference/v1
}

// NewFireworksProvider creates a Fireworks provider from the given config.
func NewFireworksProvider(cfg FireworksConfig) (*FireworksProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("fireworks provider requires api key")
	}
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("fireworks provider requires model_id")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.fireworks.ai/inference/v1"
	}

	return &FireworksProvider{
		apiKey:     cfg.APIKey,
		modelID:    cfg.ModelID,
		reasoning:  strings.TrimSpace(cfg.ReasoningEffort),
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
		baseURL:    strings.TrimRight(baseURL, "/"),
	}, nil
}

// NewFireworksProviderFromEnv creates a Fireworks provider using credentials
// from environment variables (FIREWORKS_API_KEY) and the given model ID.
// The model is a runtime concern, not an env var. The base URL defaults to
// https://api.fireworks.ai/inference/v1 unless FIREWORKS_BASE_URL is set.
func NewFireworksProviderFromEnv(modelID string) (*FireworksProvider, error) {
	apiKey := os.Getenv("FIREWORKS_API_KEY")
	baseURL := os.Getenv("FIREWORKS_BASE_URL")

	p, err := NewFireworksProvider(FireworksConfig{
		APIKey:  apiKey,
		ModelID: modelID,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("fireworks from env: %w", err)
	}
	return p, nil
}

func (p *FireworksProvider) Name() string { return "fireworks" }
func (p *FireworksProvider) IsReal() bool { return true }

// Call sends the request to Fireworks AI's OpenAI-compatible chat completions endpoint.
// Provider adapter edits should deploy as fast gateway-only changes; VM guests use gatewayruntime.
func (p *FireworksProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	endpoint := p.fireworksChatCompletionsEndpoint()
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "fireworks", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("fireworks: %w", mediaErr)
	}

	body := p.buildChatCompletionsRequestBody(req, modelID)

	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("fireworks: build request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	log.Printf("provider: fireworks call model=%s", modelID)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fireworks: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	return parseOpenAIChatCompletionsResponse(resp, modelID, "fireworks")
}

func (p *FireworksProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	endpoint := p.fireworksChatCompletionsEndpoint()
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "fireworks", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("fireworks: %w", mediaErr)
	}

	req.Stream = true
	body := p.buildChatCompletionsRequestBody(req, modelID)
	body.Stream = true

	httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("fireworks: build stream request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	log.Printf("provider: fireworks stream model=%s", modelID)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("fireworks: stream http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fireworks: status %s (sanitized)", resp.Status)
	}

	return parseOpenAIChatCompletionsStream(resp.Body, modelID, "fireworks", onChunk)
}

func (p *FireworksProvider) fireworksChatCompletionsEndpoint() string {
	base := strings.TrimRight(p.baseURL, "/")
	switch {
	case strings.HasSuffix(base, "/chat/completions"):
		return base
	case strings.HasSuffix(base, "/v1"):
		return base + "/chat/completions"
	case strings.HasSuffix(base, "/inference"):
		return base + "/v1/chat/completions"
	default:
		return base + "/chat/completions"
	}
}

func (p *FireworksProvider) buildChatCompletionsRequestBody(req LLMRequest, modelID string) openAIChatCompletionRequest {
	tools := convertOpenAIChatTools(req.Tools)
	body := openAIChatCompletionRequest{
		Model:           modelID,
		Messages:        convertOpenAIChatMessages(req.System, req.Messages),
		Tools:           tools,
		ToolChoice:      openAIChatToolChoice(req.ToolChoice),
		Stream:          false,
		ReasoningEffort: effectiveReasoning(req.ReasoningEffort, p.reasoning),
	}
	if req.MaxTokens > 0 {
		maxTokens := req.MaxTokens
		body.MaxTokens = &maxTokens
	}
	if len(tools) > 0 {
		temperature := 0.1
		body.Temperature = &temperature
	}
	return body
}

// SyntheticProvider implements the Provider interface for Synthetic
// (synthetic.new) over its OpenAI-compatible Chat Completions API. It has one
// key and no key pool: Synthetic's 429 is an account concurrency limit (about
// 10 in flight, no Retry-After), so it surfaces at once for the gateway
// breaker and the run's failover instead of parking the only key
// (docs/problems/synthetic-provider-onboarding-2026-10-22.md).
type SyntheticProvider struct {
	apiKey     string // loaded at init time, never logged
	modelID    string
	baseURL    string
	httpClient *http.Client
}

// SyntheticConfig holds configuration for creating a SyntheticProvider.
type SyntheticConfig struct {
	APIKey  string
	ModelID string
	BaseURL string // defaults to https://api.synthetic.new/openai/v1
}

const syntheticDefaultBaseURL = "https://api.synthetic.new/openai/v1"

// NewSyntheticProvider creates a Synthetic provider from the given config.
func NewSyntheticProvider(cfg SyntheticConfig) (*SyntheticProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("synthetic provider requires api key")
	}
	modelID := strings.TrimSpace(cfg.ModelID)
	if modelID == "" {
		return nil, fmt.Errorf("synthetic provider requires model_id")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = syntheticDefaultBaseURL
	}
	return &SyntheticProvider{
		apiKey:     apiKey,
		modelID:    modelID,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
	}, nil
}

// NewSyntheticProviderFromEnv creates a Synthetic provider from
// SYNTHETIC_API_KEY and the optional SYNTHETIC_BASE_URL.
func NewSyntheticProviderFromEnv(modelID string) (*SyntheticProvider, error) {
	p, err := NewSyntheticProvider(SyntheticConfig{
		APIKey:  os.Getenv("SYNTHETIC_API_KEY"),
		ModelID: modelID,
		BaseURL: os.Getenv("SYNTHETIC_BASE_URL"),
	})
	if err != nil {
		return nil, fmt.Errorf("synthetic from env: %w", err)
	}
	return p, nil
}

func (p *SyntheticProvider) Name() string { return "synthetic" }
func (p *SyntheticProvider) IsReal() bool { return true }

func (p *SyntheticProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	return p.call(ctx, req, func(StreamChunk) {})
}

func (p *SyntheticProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	req.Stream = true
	return p.call(ctx, req, onChunk)
}

func (p *SyntheticProvider) call(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "synthetic", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("synthetic: %w", mediaErr)
	}
	body := p.buildChatCompletionsRequestBody(req, modelID)
	httpReq, err := newJSONRequest(ctx, http.MethodPost, openCodeEndpoint(p.baseURL, "chat/completions"), body)
	if err != nil {
		return nil, fmt.Errorf("synthetic: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "choir-gateway/0.1")
	if req.Stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	} else {
		httpReq.Header.Set("Accept", "application/json")
	}

	log.Printf("provider: synthetic call model=%s stream=%t reasoning=%s", modelID, req.Stream, body.ReasoningEffort)
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("synthetic: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if summary := syntheticQuotaSummary(resp.Header); summary != "" {
		log.Printf("provider: synthetic quota %s", summary)
	}
	if !req.Stream {
		return parseOpenAIChatCompletionsResponse(resp, modelID, "synthetic")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logRejectedRequest("synthetic", resp)
		return nil, fmt.Errorf("synthetic: status %s (sanitized)", resp.Status)
	}
	return parseOpenAIChatCompletionsStream(resp.Body, modelID, "synthetic", onChunk)
}

func (p *SyntheticProvider) buildChatCompletionsRequestBody(req LLMRequest, modelID string) openAIChatCompletionRequest {
	body := openAIChatCompletionRequest{
		Model:           modelID,
		Messages:        convertOpenAIChatMessages(req.System, req.Messages),
		Tools:           convertOpenAIChatTools(req.Tools),
		ToolChoice:      openAIChatToolChoice(req.ToolChoice),
		Stream:          req.Stream,
		ReasoningEffort: syntheticReasoningEffort(req.ReasoningEffort),
	}
	if req.MaxTokens > 0 {
		maxTokens := req.MaxTokens
		body.MaxTokens = &maxTokens
	}
	return body
}

// syntheticReasoningEffort maps Choir's effort to the values Synthetic
// advertises (models endpoint reasoning_parameters.efforts: none, low, high,
// xhigh, max). reasoning_effort is a mode switch plus a budget there: none
// and low run the non-thinking template; high, xhigh and max think. The
// server also accepts minimal and medium today (probe, docs/problems/
// synthetic-provider-onboarding-2026-10-22.md Q2), but they are outside the
// advertised set, so each maps to the advertised value in its own mode:
// minimal to none (non-thinking), medium to high (thinking). Any other
// string is a 400, so an unknown value is omitted (model default) and the
// disable spellings Choir uses elsewhere become none.
func syntheticReasoningEffort(effort string) string {
	switch e := strings.ToLower(strings.TrimSpace(effort)); e {
	case "none", "low", "high", "xhigh", "max":
		return e
	case "minimal", "off", "disabled", "false":
		return "none"
	case "medium":
		return "high"
	default:
		return ""
	}
}

// syntheticQuotaSummary renders the x-synthetic-quotas response header (the
// same JSON as GET /v2/quotas) as a log fragment: the weekly plan percentage
// is what an overnight run spends. Empty when the header is absent or bad.
func syntheticQuotaSummary(h http.Header) string {
	raw := strings.TrimSpace(h.Get("x-synthetic-quotas"))
	if raw == "" {
		return ""
	}
	var q struct {
		Subscription struct {
			Limit    int `json:"limit"`
			Requests int `json:"requests"`
		} `json:"subscription"`
		Plan struct {
			PercentRemaining *float64 `json:"percentRemaining"`
			NextRegenAt      string   `json:"nextRegenAt"`
		} `json:"planWeeklyLimit"`
	}
	if json.Unmarshal([]byte(raw), &q) != nil || q.Plan.PercentRemaining == nil {
		return ""
	}
	return fmt.Sprintf("weekly_percent_remaining=%.4f weekly_next_regen=%s subscription_requests=%d/%d",
		*q.Plan.PercentRemaining, q.Plan.NextRegenAt, q.Subscription.Requests, q.Subscription.Limit)
}

// ChatGPTProvider implements the Provider interface for ChatGPT subscription
// billing through Codex OAuth, using the OpenAI Responses-compatible endpoint
// exposed by ChatGPT.
type ChatGPTProvider struct {
	auth       *ChatGPTAuth
	modelID    string
	httpClient *http.Client
	baseURL    string
	reasoning  string
}

// ChatGPTConfig holds configuration for creating a ChatGPTProvider.
type ChatGPTConfig struct {
	ModelID         string
	BaseURL         string
	AuthPath        string
	ReasoningEffort string
}

const defaultChatGPTResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
const defaultChatGPTInstructions = "You are a helpful assistant."

// NewChatGPTProvider creates a ChatGPT provider using Codex OAuth auth.
func NewChatGPTProvider(cfg ChatGPTConfig) (*ChatGPTProvider, error) {
	if cfg.ModelID == "" {
		return nil, fmt.Errorf("chatgpt provider requires model_id")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultChatGPTResponsesURL
	}
	auth := SharedChatGPTAuth(cfg.AuthPath)
	if _, err := auth.Read(); err != nil {
		return nil, fmt.Errorf("chatgpt provider requires codex auth: %w", err)
	}
	reasoning := strings.TrimSpace(cfg.ReasoningEffort)
	if reasoning == "" {
		reasoning = "low"
	}
	return &ChatGPTProvider{
		auth:       auth,
		modelID:    cfg.ModelID,
		httpClient: &http.Client{Timeout: defaultProviderHTTPTimeout},
		baseURL:    baseURL,
		reasoning:  reasoning,
	}, nil
}

// NewChatGPTProviderFromEnv creates a ChatGPT provider using Codex OAuth from
// CHATGPT_AUTH_PATH or ~/.codex/auth.json. CHATGPT_BASE_URL can override the
// Responses endpoint for tests or custom deployments.
func NewChatGPTProviderFromEnv(modelID, reasoningEffort string) (*ChatGPTProvider, error) {
	p, err := NewChatGPTProvider(ChatGPTConfig{
		ModelID:         modelID,
		BaseURL:         os.Getenv("CHATGPT_BASE_URL"),
		AuthPath:        os.Getenv("CHATGPT_AUTH_PATH"),
		ReasoningEffort: reasoningEffort,
	})
	if err != nil {
		return nil, fmt.Errorf("chatgpt from env: %w", err)
	}
	return p, nil
}

func (p *ChatGPTProvider) Name() string { return "chatgpt" }
func (p *ChatGPTProvider) IsReal() bool { return true }

func (p *ChatGPTProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	req.Stream = true
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "chatgpt", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("chatgpt: %w", mediaErr)
	}
	body := p.buildRequestBody(req, modelID)

	log.Printf("provider: chatgpt call model=%s reasoning=%s", modelID, effectiveReasoning(req.ReasoningEffort, p.reasoning))

	resp, err := p.doStreamRequestWithAuthRetry(ctx, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return parseOpenAIStreamLogged(resp.Body, modelID, "chatgpt", func(StreamChunk) {}, newCacheLogMeta(body))
}

func (p *ChatGPTProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	req.Stream = true
	modelID := effectiveModel(req.Model, p.modelID)
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, "chatgpt", modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("chatgpt: %w", mediaErr)
	}
	body := p.buildRequestBody(req, modelID)
	body.Stream = true

	log.Printf("provider: chatgpt stream model=%s reasoning=%s", modelID, effectiveReasoning(req.ReasoningEffort, p.reasoning))

	resp, err := p.doStreamRequestWithAuthRetry(ctx, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return parseOpenAIStreamLogged(resp.Body, modelID, "chatgpt", onChunk, newCacheLogMeta(body))
}

func (p *ChatGPTProvider) doStreamRequestWithAuthRetry(ctx context.Context, body openAIRequest) (*http.Response, error) {
	resp, err := p.doStreamRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("chatgpt: status %s (sanitized)", resp.Status)
		}
		return resp, nil
	}

	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if _, err := p.auth.ForceRefresh(ctx); err != nil {
		return nil, fmt.Errorf("chatgpt: auth refresh after unauthorized response: %w", err)
	}
	resp, err = p.doStreamRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("chatgpt: status %s after auth refresh (sanitized)", resp.Status)
	}
	return resp, nil
}

func (p *ChatGPTProvider) doStreamRequest(ctx context.Context, body openAIRequest) (*http.Response, error) {
	httpReq, err := newJSONRequest(ctx, http.MethodPost, p.baseURL, body)
	if err != nil {
		return nil, fmt.Errorf("chatgpt: build request: %w", err)
	}
	authHeader, err := p.auth.Header(ctx)
	if err != nil {
		return nil, fmt.Errorf("chatgpt: auth: %w", err)
	}
	httpReq.Header.Set("Authorization", authHeader)
	httpReq.Header.Set("Accept", "text/event-stream")
	// Codex CLI, the reference client for this backend, sends the thread id
	// as session_id and conversation_id next to prompt_cache_key. Measured
	// without them: later calls of a run cache only the static prefix, as if
	// each landed on another cache machine. Rollback:
	// CHOIR_CHATGPT_SESSION_HEADERS=0.
	if body.PromptCacheKey != "" && chatGPTSessionHeadersEnabled() {
		httpReq.Header.Set("session_id", body.PromptCacheKey)
		httpReq.Header.Set("conversation_id", body.PromptCacheKey)
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chatgpt: http call: %w", err)
	}
	return resp, nil
}

func (p *ChatGPTProvider) buildRequestBody(req LLMRequest, modelID string) openAIRequest {
	instructions := strings.TrimSpace(req.System)
	if instructions == "" {
		instructions = defaultChatGPTInstructions
	}
	payload := openAIRequest{
		Model:        modelID,
		Instructions: instructions,
		Input:        convertOpenAIInput(req.Messages),
		Tools:        convertOpenAITools(req.Tools),
		ToolChoice:   openAIResponsesToolChoice(req.ToolChoice),
		Store:        false,
		Stream:       req.Stream,
		// Without a key the endpoint lands a request on a cache machine at
		// random: 4-10% hits on gpt-6-luna even mid-activation
		// (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
		// Codex CLI sends its conversation id here; we send the desk
		// thread's key, or the run's conversation id when there is none.
		PromptCacheKey: cacheRoutingKey(req),
	}
	if effort := effectiveReasoning(req.ReasoningEffort, p.reasoning); effort != "" && effort != "none" && effort != "off" {
		payload.Reasoning = &openAIReasoning{Effort: effort}
	}
	return payload
}

// OpenCodeProvider routes OpenCode Go and Zen models over their documented
// per-model API wire shapes. Every request carries the durable run identity
// OpenCode requires for session affinity.
type OpenCodeProvider struct {
	keys         *openCodeKeyPool
	modelID      string
	baseURL      string
	providerName string
	httpClient   *http.Client
}

// OpenCodeConfig configures an OpenCode Go or Zen provider.
type OpenCodeConfig struct {
	APIKey       string
	BaseURL      string
	ModelID      string
	ProviderName string
	// keys, when set, is the pool the provider draws from (the shared
	// environment pool); APIKey is then only the presence check.
	keys *openCodeKeyPool
}

type openCodeWireShape uint8

const (
	openCodeChatCompletions openCodeWireShape = iota + 1
	openCodeResponses
	openCodeMessages
)

func NewOpenCodeProvider(cfg OpenCodeConfig) (*OpenCodeProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("opencode provider requires api key")
	}
	modelID := strings.TrimSpace(cfg.ModelID)
	if modelID == "" {
		return nil, fmt.Errorf("opencode provider requires model_id")
	}
	if _, err := openCodeWireShapeForModel(modelID); err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("opencode provider requires base_url")
	}
	providerName := strings.TrimSpace(cfg.ProviderName)
	if providerName == "" {
		providerName = "opencode"
	}
	keys := cfg.keys
	if keys == nil {
		keys = newOpenCodeKeyPool([]openCodeKey{{name: "api_key", value: apiKey}})
	}
	return &OpenCodeProvider{
		keys:         keys,
		modelID:      modelID,
		baseURL:      baseURL,
		providerName: providerName,
		httpClient:   &http.Client{Timeout: defaultProviderHTTPTimeout},
	}, nil
}

func NewOpenCodeGoProviderFromEnv(modelID string) (*OpenCodeProvider, error) {
	baseURL := strings.TrimSpace(os.Getenv("OPENCODE_GO_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://opencode.ai/zen/go/v1"
	}
	return NewOpenCodeProvider(OpenCodeConfig{
		APIKey:       openCodeAPIKeyFromEnv(),
		BaseURL:      baseURL,
		ModelID:      modelID,
		ProviderName: "opencode-go",
		keys:         sharedOpenCodeKeyPool(),
	})
}

func NewOpenCodeZenProviderFromEnv(modelID string) (*OpenCodeProvider, error) {
	baseURL := strings.TrimSpace(os.Getenv("OPENCODE_ZEN_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://opencode.ai/zen/v1"
	}
	return NewOpenCodeProvider(OpenCodeConfig{
		APIKey:       openCodeAPIKeyFromEnv(),
		BaseURL:      baseURL,
		ModelID:      modelID,
		ProviderName: "opencode-zen",
		keys:         sharedOpenCodeKeyPool(),
	})
}

// openCodeAPIKeyFromEnv is the first configured key, the presence check
// for the env constructors; calls draw from sharedOpenCodeKeyPool.
func openCodeAPIKeyFromEnv() string {
	keys := openCodeKeysFromEnv()
	logOpenCodeKeysInService(keys)
	if len(keys) > 0 {
		return keys[0].value
	}
	return ""
}

func (p *OpenCodeProvider) Name() string { return p.providerName }
func (p *OpenCodeProvider) IsReal() bool { return true }

func (p *OpenCodeProvider) Call(ctx context.Context, req LLMRequest) (*LLMResponse, error) {
	return p.call(ctx, req, func(StreamChunk) {})
}

func (p *OpenCodeProvider) Stream(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	req.Stream = true
	return p.call(ctx, req, onChunk)
}

// cacheRoutingKey is the key a provider routes cache lookups by: the
// desk thread's cache key, else the run's conversation id.
func cacheRoutingKey(req LLMRequest) string {
	if key := strings.TrimSpace(req.CacheKey); key != "" {
		return key
	}
	return strings.TrimSpace(req.ConversationID)
}

func (p *OpenCodeProvider) call(ctx context.Context, req LLMRequest, onChunk func(StreamChunk)) (*LLMResponse, error) {
	// OpenCode routes a session to one upstream for "routing and
	// prompt-cache affinity" (docs/reports/choir-opencode-provider-research-
	// 2026-09-10.md §4). The session is the desk thread, so successive runs
	// of a desk land where its prefix is cached; the run id only when the
	// caller names no thread.
	session := cacheRoutingKey(req)
	if session == "" {
		return nil, fmt.Errorf("%s: conversation_id is required for OpenCode requests", p.providerName)
	}
	modelID := effectiveModel(req.Model, p.modelID)
	shape, err := openCodeWireShapeForModel(modelID)
	if err != nil {
		return nil, err
	}
	req, releaseMedia, mediaErr := prepareMediaRequest(ctx, p.providerName, modelID, req)
	defer releaseMedia() // the downscaled images live for the whole upstream call (F7)
	if mediaErr != nil {
		return nil, fmt.Errorf("%s: %w", p.providerName, mediaErr)
	}

	var endpoint string
	var body any
	var accept string
	switch shape {
	case openCodeChatCompletions:
		endpoint = openCodeEndpoint(p.baseURL, "chat/completions")
		body = p.buildChatCompletionsRequestBody(req, modelID)
		if req.Stream {
			accept = "text/event-stream"
		} else {
			accept = "application/json"
		}
	case openCodeResponses:
		endpoint = openCodeEndpoint(p.baseURL, "responses")
		body = p.buildResponsesRequestBody(req, modelID)
		accept = "text/event-stream"
	case openCodeMessages:
		endpoint = openCodeEndpoint(p.baseURL, "messages")
		body = p.buildMessagesRequestBody(req, modelID)
		if req.Stream {
			accept = "text/event-stream"
		} else {
			accept = "application/json"
		}
	default:
		return nil, fmt.Errorf("%s: unsupported OpenCode wire shape", p.providerName)
	}

	// Draw a key per attempt. A 402 or 429 parks that key and the request
	// is retried on another healthy key; no body has been read yet, so the
	// retry cannot duplicate output.
	var resp *http.Response
	tried := map[string]bool{}
	var lastStatus string
	for attempt := 0; attempt < p.keys.size(); attempt++ {
		key, ok := p.keys.pick(session, tried)
		if !ok {
			break
		}
		tried[key.name] = true
		httpReq, err := newJSONRequest(ctx, http.MethodPost, endpoint, body)
		if err != nil {
			return nil, fmt.Errorf("%s: build request: %w", p.providerName, err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", accept)
		httpReq.Header.Set("User-Agent", "choir-gateway/0.1")
		httpReq.Header.Set("x-opencode-session", session)
		if shape == openCodeMessages {
			httpReq.Header.Set("x-api-key", key.value)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
		} else {
			httpReq.Header.Set("Authorization", "Bearer "+key.value)
		}

		log.Printf("provider: %s call model=%s key=%s", p.providerName, modelID, key.name)
		r, err := p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("%s: http call: %w", p.providerName, err)
		}
		if p.keys.park(key.name, r.StatusCode) {
			logRejectedRequest(p.providerName, r)
			_ = r.Body.Close()
			lastStatus = r.Status
			log.Printf("provider: %s key=%s parked after status %d", p.providerName, key.name, r.StatusCode)
			continue
		}
		resp = r
		break
	}
	if resp == nil {
		if lastStatus != "" {
			return nil, fmt.Errorf("%s: status %s on every OpenCode key (sanitized)", p.providerName, lastStatus)
		}
		return nil, fmt.Errorf("%s: every OpenCode key is parked (out of funds, rate limited or a dead credential)", p.providerName)
	}
	defer func() { _ = resp.Body.Close() }()

	switch shape {
	case openCodeChatCompletions:
		if req.Stream {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				logRejectedRequest(p.providerName, resp)
				return nil, fmt.Errorf("%s: status %s (sanitized)", p.providerName, resp.Status)
			}
			return parseOpenAIChatCompletionsStream(resp.Body, modelID, p.providerName, onChunk)
		}
		return parseOpenAIChatCompletionsResponse(resp, modelID, p.providerName)
	case openCodeResponses:
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logRejectedRequest(p.providerName, resp)
			return nil, fmt.Errorf("%s: status %s (sanitized)", p.providerName, resp.Status)
		}
		return parseOpenAIStream(resp.Body, modelID, p.providerName, onChunk)
	case openCodeMessages:
		if req.Stream {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				logRejectedRequest(p.providerName, resp)
				return nil, fmt.Errorf("%s: status %s (sanitized)", p.providerName, resp.Status)
			}
			return parseSSEStream(resp.Body, modelID, p.providerName, onChunk)
		}
		return parseAnthropicResponse(resp, modelID, p.providerName)
	}
	return nil, fmt.Errorf("%s: unsupported OpenCode wire shape", p.providerName)
}

func (p *OpenCodeProvider) buildChatCompletionsRequestBody(req LLMRequest, modelID string) openAIChatCompletionRequest {
	body := openAIChatCompletionRequest{
		Model:      modelID,
		Messages:   convertOpenAIChatMessages(req.System, req.Messages),
		Tools:      convertOpenAIChatTools(req.Tools),
		ToolChoice: openAIChatToolChoice(req.ToolChoice),
		Stream:     req.Stream,
	}
	if req.MaxTokens > 0 {
		maxTokens := req.MaxTokens
		body.MaxTokens = &maxTokens
	}
	return body
}

func (p *OpenCodeProvider) buildResponsesRequestBody(req LLMRequest, modelID string) openAIRequest {
	body := openAIRequest{
		Model:           modelID,
		Instructions:    strings.TrimSpace(req.System),
		Input:           convertOpenAIInput(req.Messages),
		Tools:           convertOpenAITools(req.Tools),
		ToolChoice:      openAIResponsesToolChoice(req.ToolChoice),
		MaxOutputTokens: req.MaxTokens,
		Store:           false,
		// Responses parsing is event-stream based for both Call and Stream.
		Stream: true,
	}
	return body
}

func (p *OpenCodeProvider) buildMessagesRequestBody(req LLMRequest, modelID string) anthropicRequest {
	return anthropicRequest{
		Model:      modelID,
		MaxTokens:  defaultMaxTokens(req.MaxTokens),
		Stream:     req.Stream,
		Messages:   convertMessages(req.Messages),
		Tools:      convertToolDefs(req.Tools),
		ToolChoice: anthropicToolChoice(req.ToolChoice),
		System: func() []anthropicSystemBlock {
			if strings.TrimSpace(req.System) == "" {
				return nil
			}
			return []anthropicSystemBlock{{Type: "text", Text: req.System}}
		}(),
	}
}

func openCodeEndpoint(baseURL, suffix string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if strings.HasSuffix(base, "/"+suffix) {
		return base
	}
	return base + "/" + suffix
}

func openCodeWireShapeForModel(modelID string) (openCodeWireShape, error) {
	switch strings.TrimSpace(modelID) {
	case "deepseek-v4.1-flash", "glm-5.3-flash", "hy3",
		"cat-jam-free", "longcat-2.5-preview-free", "step-5-preview-free":
		return openCodeChatCompletions, nil
	case "muse-spark-1.3-contributor", "muse-spark-1.3-contributor-free":
		return openCodeResponses, nil
	case "qwen3.7-max":
		return openCodeMessages, nil
	default:
		return 0, fmt.Errorf("opencode provider: unsupported model %q", modelID)
	}
}

func effectiveReasoning(requested, fallback string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	return strings.TrimSpace(fallback)
}

func effectiveModel(requested, fallback string) string {
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	return fallback
}

func openAIChatToolChoice(choice string) any {
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return nil
	}
	if name, ok := exactToolChoiceName(choice); ok {
		return map[string]any{
			"type": "function",
			"function": map[string]string{
				"name": name,
			},
		}
	}
	return choice
}

func openAIResponsesToolChoice(choice string) any {
	choice = strings.TrimSpace(choice)
	if choice == "" {
		return nil
	}
	if name, ok := exactToolChoiceName(choice); ok {
		return map[string]string{
			"type": "function",
			"name": name,
		}
	}
	return choice
}

func exactToolChoiceName(choice string) (string, bool) {
	name, ok := strings.CutPrefix(strings.TrimSpace(choice), "function:")
	if !ok {
		return "", false
	}
	name = strings.TrimSpace(name)
	return name, name != ""
}

type openAIRequest struct {
	Model           string           `json:"model"`
	Instructions    string           `json:"instructions,omitempty"`
	Input           []openAIItem     `json:"input,omitempty"`
	Tools           []openAITool     `json:"tools,omitempty"`
	ToolChoice      any              `json:"tool_choice,omitempty"`
	MaxOutputTokens int              `json:"max_output_tokens,omitempty"`
	Store           bool             `json:"store"`
	Stream          bool             `json:"stream,omitempty"`
	Reasoning       *openAIReasoning `json:"reasoning,omitempty"`
	// PromptCacheKey routes the cache lookup with the prefix hash. Sent on
	// the chatgpt route only, where Codex CLI sends it.
	PromptCacheKey string `json:"prompt_cache_key,omitempty"`
}

type openAIReasoning struct {
	Effort string `json:"effort"`
}

type openAIItem struct {
	Type      string `json:"type,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   any    `json:"content,omitempty"`
	Name      string `json:"name,omitempty"`
	CallID    string `json:"call_id,omitempty"`
	Arguments string `json:"arguments,omitempty"`
	Output    string `json:"output,omitempty"`
}

type openAITool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIChatCompletionRequest struct {
	Model               string              `json:"model"`
	Messages            []openAIChatMessage `json:"messages"`
	Tools               []openAIChatTool    `json:"tools,omitempty"`
	ToolChoice          any                 `json:"tool_choice,omitempty"`
	MaxTokens           *int                `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                `json:"max_completion_tokens,omitempty"`
	Stream              bool                `json:"stream,omitempty"`
	ReasoningEffort     string              `json:"reasoning_effort,omitempty"`
	Temperature         *float64            `json:"temperature,omitempty"`
}

type openAIChatMessage struct {
	Role             string               `json:"role"`
	Content          any                  `json:"content,omitempty"`
	ReasoningContent string               `json:"reasoning_content,omitempty"`
	ToolCallID       string               `json:"tool_call_id,omitempty"`
	ToolCalls        []openAIChatToolCall `json:"tool_calls,omitempty"`
}

type openAIChatTool struct {
	Type     string             `json:"type"`
	Function openAIChatFunction `json:"function"`
}

type openAIChatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Arguments   string         `json:"arguments,omitempty"`
}

type openAIChatToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIChatFunction `json:"function"`
}

type openAIChatCompletionResponse struct {
	ID      string                    `json:"id"`
	Model   string                    `json:"model,omitempty"`
	Choices []openAIChatChoice        `json:"choices"`
	Usage   openAIChatCompletionUsage `json:"usage"`
}

type openAIChatChoice struct {
	FinishReason string            `json:"finish_reason"`
	Message      openAIChatMessage `json:"message"`
}

type openAIChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// OpenAI-style cache report (also sent by DeepSeek and most
	// OpenAI-compatible routers).
	PromptTokensDetails *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
	// DeepSeek-native cache report.
	PromptCacheHitTokens  *int `json:"prompt_cache_hit_tokens,omitempty"`
	PromptCacheMissTokens *int `json:"prompt_cache_miss_tokens,omitempty"`
}

func (u openAIChatCompletionUsage) present() bool {
	return u.PromptTokens > 0 || u.CompletionTokens > 0 || u.PromptCacheHitTokens != nil
}

// usage maps a chat-completions usage object to Usage. When both the OpenAI
// and the DeepSeek cache fields are present the larger is taken (they report
// the same quantity); when prompt_tokens is absent, hit+miss is the prompt.
func (u openAIChatCompletionUsage) usage() Usage {
	out := Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
	if d := u.PromptTokensDetails; d != nil && d.CachedTokens != nil {
		out.CacheReported = true
		out.CachedInputTokens = *d.CachedTokens
	}
	if u.PromptCacheHitTokens != nil {
		out.CacheReported = true
		if *u.PromptCacheHitTokens > out.CachedInputTokens {
			out.CachedInputTokens = *u.PromptCacheHitTokens
		}
		if out.InputTokens == 0 && u.PromptCacheMissTokens != nil {
			out.InputTokens = *u.PromptCacheHitTokens + *u.PromptCacheMissTokens
		}
	}
	return out.normalizeCache()
}

func convertOpenAIInput(messages []Message) []openAIItem {
	out := make([]openAIItem, 0, len(messages))
	for _, msg := range messages {
		var parts []map[string]string
		flushText := func() {
			if len(parts) == 0 {
				return
			}
			out = append(out, openAIItem{Role: msg.Role, Content: parts})
			parts = nil
		}
		contentType := "input_text"
		if msg.Role == "assistant" {
			contentType = "output_text"
		}
		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					parts = append(parts, map[string]string{"type": contentType, "text": block.Text})
				}
			case "image":
				if source := openAIImageSource(block.Source); source != "" {
					parts = append(parts, map[string]string{"type": "input_image", "image_url": source})
				}
			case "tool_use":
				flushText()
				out = append(out, openAIItem{
					Type:      "function_call",
					CallID:    block.ID,
					Name:      block.Name,
					Arguments: string(canonicalJSON(block.Input)),
				})
			case "tool_result":
				flushText()
				out = append(out, openAIItem{
					Type:   "function_call_output",
					CallID: block.ToolUseID,
					Output: block.Text,
				})
			}
		}
		flushText()
	}
	return out
}

func convertOpenAIChatMessages(system string, messages []Message) []openAIChatMessage {
	out := make([]openAIChatMessage, 0, len(messages)+1)
	if strings.TrimSpace(system) != "" {
		out = append(out, openAIChatMessage{Role: "system", Content: system})
	}
	for _, msg := range messages {
		var textParts []string
		var multimodal []map[string]any
		var assistantToolCalls []openAIChatToolCall

		flushText := func() {
			if len(textParts) == 0 && len(multimodal) == 0 {
				return
			}
			reasoningContent := ""
			if msg.Role == "assistant" {
				reasoningContent = strings.TrimSpace(msg.ReasoningContent)
			}
			if len(multimodal) > 0 {
				out = append(out, openAIChatMessage{Role: msg.Role, Content: multimodal, ReasoningContent: reasoningContent})
			} else {
				out = append(out, openAIChatMessage{Role: msg.Role, Content: strings.Join(textParts, "\n"), ReasoningContent: reasoningContent})
			}
			textParts = nil
			multimodal = nil
		}

		for _, block := range msg.Content {
			switch block.Type {
			case "text":
				if block.Text == "" {
					continue
				}
				if len(multimodal) > 0 {
					multimodal = append(multimodal, map[string]any{"type": "text", "text": block.Text})
				} else {
					textParts = append(textParts, block.Text)
				}
			case "image":
				if msg.Role != "user" {
					continue
				}
				if len(textParts) > 0 {
					for _, text := range textParts {
						multimodal = append(multimodal, map[string]any{"type": "text", "text": text})
					}
					textParts = nil
				}
				if source := openAIImageSource(block.Source); source != "" {
					multimodal = append(multimodal, map[string]any{
						"type":      "image_url",
						"image_url": map[string]string{"url": source},
					})
				}
			case "tool_use":
				assistantToolCalls = append(assistantToolCalls, openAIChatToolCall{
					ID:   block.ID,
					Type: "function",
					Function: openAIChatFunction{
						Name:      block.Name,
						Arguments: string(canonicalJSON(block.Input)),
					},
				})
			case "tool_result":
				flushText()
				out = append(out, openAIChatMessage{
					Role:       "tool",
					ToolCallID: block.ToolUseID,
					Content:    block.Text,
				})
			}
		}
		if len(assistantToolCalls) > 0 {
			content := strings.Join(textParts, "\n")
			out = append(out, openAIChatMessage{Role: "assistant", Content: content, ReasoningContent: strings.TrimSpace(msg.ReasoningContent), ToolCalls: assistantToolCalls})
			continue
		}
		flushText()
	}
	return out
}

func convertOpenAITools(tools []ToolDef) []openAITool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, openAITool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.InputSchema,
		})
	}
	return out
}

func convertOpenAIChatTools(tools []ToolDef) []openAIChatTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openAIChatTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, openAIChatTool{
			Type: "function",
			Function: openAIChatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}
	return out
}

// --- Shared types and helpers ---

type anthropicRequest struct {
	Model            string             `json:"model,omitempty"` // empty for Bedrock (model in URL)
	System           any                `json:"system,omitempty"`
	Messages         []anthropicMessage `json:"messages"`
	Tools            []anthropicTool    `json:"tools,omitempty"`
	ToolChoice       any                `json:"tool_choice,omitempty"`
	Thinking         *anthropicThinking `json:"thinking,omitempty"`
	MaxTokens        int                `json:"max_tokens"`
	Stream           bool               `json:"stream,omitempty"`
	AnthropicVersion string             `json:"anthropic_version,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

type anthropicSystemBlock struct {
	Type         string            `json:"type"`
	Text         string            `json:"text"`
	CacheControl map[string]string `json:"cache_control,omitempty"`
}

// anthropicTool is the tool definition format for the Anthropic Messages API.
type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	Source    any             `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`          // tool_use: call ID
	Name      string          `json:"name,omitempty"`        // tool_use: tool name
	Input     json.RawMessage `json:"input,omitempty"`       // tool_use: arguments
	ToolUseID string          `json:"tool_use_id,omitempty"` // tool_result: originating call ID
	Content   string          `json:"content,omitempty"`     // tool_result: result content (when string)
	IsError   bool            `json:"is_error,omitempty"`    // tool_result: error flag
	// CacheControl marks an Anthropic prompt-cache breakpoint on this block.
	CacheControl map[string]string `json:"cache_control,omitempty"`
}

// markHistoryCacheBreakpoint puts an ephemeral cache breakpoint on the last
// cacheable block of the last message, so an Anthropic-native endpoint
// caches the conversation so far and the next call of the append-only loop
// reads it back (the system breakpoint alone caches only tools and system).
// Thinking blocks cannot carry one.
func markHistoryCacheBreakpoint(messages []anthropicMessage) {
	if len(messages) == 0 {
		return
	}
	content := messages[len(messages)-1].Content
	for i := len(content) - 1; i >= 0; i-- {
		switch content[i].Type {
		case "thinking", "redacted_thinking":
			continue
		}
		content[i].CacheControl = map[string]string{"type": "ephemeral"}
		return
	}
}

type anthropicResponse struct {
	ID         string                   `json:"id"`
	Content    []anthropicResponseBlock `json:"content"`
	StopReason string                   `json:"stop_reason"`
	Usage      anthropicUsage           `json:"usage"`
	Model      string                   `json:"model,omitempty"`
}

type anthropicResponseBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Signature string          `json:"signature,omitempty"`
	ID        string          `json:"id,omitempty"`    // tool_use: provider-assigned call ID
	Name      string          `json:"name,omitempty"`  // tool_use: tool name
	Input     json.RawMessage `json:"input,omitempty"` // tool_use: tool arguments
}

type anthropicUsage struct {
	InputTokens              int  `json:"input_tokens"`
	OutputTokens             int  `json:"output_tokens"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens,omitempty"`
}

// usage maps an Anthropic usage object to Usage. Anthropic's input_tokens
// excludes cache reads and cache writes; InputTokens here is the whole
// prompt so CachedInputTokens stays a subset of it.
func (u anthropicUsage) usage() Usage {
	out := Usage{InputTokens: u.InputTokens, OutputTokens: u.OutputTokens}
	if u.CacheReadInputTokens != nil {
		out.CacheReported = true
		out.CachedInputTokens = *u.CacheReadInputTokens
		out.InputTokens += *u.CacheReadInputTokens
	}
	if u.CacheCreationInputTokens != nil {
		out.CacheReported = true
		out.InputTokens += *u.CacheCreationInputTokens
	}
	return out.normalizeCache()
}

func convertMessages(msgs []Message) []anthropicMessage {
	out := make([]anthropicMessage, 0, len(msgs))
	for _, msg := range msgs {
		item := anthropicMessage{
			Role:    msg.Role,
			Content: make([]anthropicContent, 0, len(msg.Content)),
		}
		if msg.Role == "assistant" {
			item.Content = append(item.Content, decodeAnthropicThinkingBlocks(msg.ReasoningContent)...)
		}
		for _, block := range msg.Content {
			ac := anthropicContent{
				Type:      block.Type,
				Text:      block.Text,
				Source:    anthropicImageSource(block.Source),
				ID:        block.ID,
				Name:      block.Name,
				Input:     block.Input,
				ToolUseID: block.ToolUseID,
				IsError:   block.IsError,
			}
			// For tool_result blocks, the result content goes in the
			// "content" field (not "text") per Anthropic Messages API.
			if block.Type == "tool_result" {
				ac.Content = block.Text
				ac.Text = "" // clear text field for tool_result
			}
			item.Content = append(item.Content, ac)
		}
		out = append(out, item)
	}
	return out
}

func anthropicToolChoice(choice string) any {
	switch strings.TrimSpace(choice) {
	case "", "auto":
		return nil
	case "none":
		return map[string]string{"type": "none"}
	case "required":
		return map[string]string{"type": "any"}
	default:
		if name, ok := strings.CutPrefix(choice, "function:"); ok && strings.TrimSpace(name) != "" {
			return map[string]string{"type": "tool", "name": strings.TrimSpace(name)}
		}
		return nil
	}
}

func encodeAnthropicThinkingBlocks(blocks []anthropicResponseBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	type hiddenThinkingBlock struct {
		Type      string `json:"type"`
		Thinking  string `json:"thinking,omitempty"`
		Signature string `json:"signature,omitempty"`
	}
	out := make([]hiddenThinkingBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "thinking" || strings.TrimSpace(block.Thinking) == "" {
			continue
		}
		out = append(out, hiddenThinkingBlock{
			Type:      "thinking",
			Thinking:  block.Thinking,
			Signature: block.Signature,
		})
	}
	if len(out) == 0 {
		return ""
	}
	data, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeAnthropicThinkingBlocks(reasoningContent string) []anthropicContent {
	reasoningContent = strings.TrimSpace(reasoningContent)
	if reasoningContent == "" {
		return nil
	}
	type hiddenThinkingBlock struct {
		Type      string `json:"type"`
		Thinking  string `json:"thinking,omitempty"`
		Signature string `json:"signature,omitempty"`
	}
	var blocks []hiddenThinkingBlock
	if err := json.Unmarshal([]byte(reasoningContent), &blocks); err != nil {
		return nil
	}
	out := make([]anthropicContent, 0, len(blocks))
	for _, block := range blocks {
		if block.Type != "thinking" || strings.TrimSpace(block.Thinking) == "" {
			continue
		}
		out = append(out, anthropicContent{
			Type:      "thinking",
			Thinking:  block.Thinking,
			Signature: block.Signature,
		})
	}
	return out
}

func anthropicImageSource(source *MediaSource) any {
	if source == nil {
		return nil
	}
	switch strings.TrimSpace(source.Kind) {
	case "base64":
		if source.Data == "" {
			return nil
		}
		mimeType := strings.TrimSpace(source.MIMEType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		return map[string]string{"type": "base64", "media_type": mimeType, "data": source.Data}
	case "url":
		if source.URL == "" {
			return nil
		}
		return map[string]string{"type": "url", "url": source.URL}
	case "artifact_ref":
		if source.Ref == "" {
			return nil
		}
		return map[string]string{"type": "artifact_ref", "ref": source.Ref, "media_type": source.MIMEType}
	default:
		return nil
	}
}

func openAIImageSource(source *MediaSource) string {
	if source == nil {
		return ""
	}
	switch strings.TrimSpace(source.Kind) {
	case "base64":
		mimeType := strings.TrimSpace(source.MIMEType)
		if mimeType == "" {
			mimeType = "image/png"
		}
		if source.Data == "" {
			return ""
		}
		return "data:" + mimeType + ";base64," + source.Data
	case "url":
		return strings.TrimSpace(source.URL)
	case "artifact_ref":
		return ""
	default:
		return ""
	}
}

// convertToolDefs converts provider ToolDef values to the Anthropic
// tool definition format. Empty input returns nil (omitted from JSON).
func convertToolDefs(tools []ToolDef) []anthropicTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropicTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, anthropicTool(tool))
	}
	return out
}

func defaultMaxTokens(n int) int {
	if n <= 0 {
		return 4096
	}
	return n
}

func maxOutputTokensForModel(modelID string) int {
	return modelcatalog.MaxOutputTokensForModel(modelID)
}

func pathEscape(s string) string {
	return strings.ReplaceAll(s, "/", "~1")
}

func redactModel(modelID string) string {
	// Show first and last segment of Bedrock model IDs for logging.
	// Bedrock IDs look like "us.anthropic.claude-sonnet-4-5-20250514-v1:0"
	// which splits into 3 dot-separated parts: us, anthropic, <long model>.
	// We want to redact the middle portion of the model string.
	parts := strings.Split(modelID, ".")
	if len(parts) >= 3 {
		// Redact the middle of the last part (the actual model identifier).
		lastPart := parts[len(parts)-1]
		redacted := redactMiddle(lastPart)
		return parts[0] + "." + parts[1] + "." + redacted
	}
	if len(parts) == 2 {
		return parts[0] + ".***"
	}
	return modelID
}

// redactMiddle shows the first and last few characters of a string,
// replacing the middle with "***".
func redactMiddle(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "***" + s[len(s)-4:]
}

func newJSONRequest(ctx context.Context, method, url string, body interface{}) (*http.Request, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func parseBedrockResponse(resp *http.Response, modelID string) (*LLMResponse, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain and discard the body to allow connection reuse, but do not
		// include it in the error (may contain provider details or credentials).
		_, _ = io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bedrock: status %s (sanitized)", resp.Status)
	}

	var payload anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("bedrock: decode response: %w", err)
	}

	result := &LLMResponse{
		ID:           payload.ID,
		Model:        coalesce(payload.Model, modelID),
		StopReason:   payload.StopReason,
		Usage:        payload.Usage.usage(),
		ProviderName: "bedrock",
	}

	for _, block := range payload.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				result.Text += block.Text
			}
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ContentToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: canonicalJSON(block.Input),
			})
		}
	}

	log.Printf("provider: bedrock response model=%s stop=%s tokens_in=%d tokens_out=%d text_len=%d tool_calls=%d",
		redactModel(result.Model), result.StopReason,
		result.Usage.InputTokens, result.Usage.OutputTokens, len(result.Text), len(result.ToolCalls))

	return result, nil
}

func parseAnthropicResponse(resp *http.Response, modelID string, providerName string) (*LLMResponse, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Drain and discard the body to allow connection reuse, but do not
		// include it in the error (may contain provider details or credentials).
		logRejectedRequest(providerName, resp)
		return nil, fmt.Errorf("%s: status %s (sanitized)", providerName, resp.Status)
	}

	var payload anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s: decode response: %w", providerName, err)
	}

	result := &LLMResponse{
		ID:           payload.ID,
		Model:        coalesce(payload.Model, modelID),
		StopReason:   payload.StopReason,
		Usage:        payload.Usage.usage(),
		ProviderName: providerName,
	}

	for _, block := range payload.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				result.Text += block.Text
			}
		case "thinking":
			// Anthropic-compatible reasoning providers return signed thinking
			// blocks that must stay hidden from user-visible text while still
			// being available for provider-continuity replay on later turns.
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ContentToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: canonicalJSON(block.Input),
			})
		}
	}
	result.ReasoningContent = encodeAnthropicThinkingBlocks(payload.Content)

	log.Printf("provider: %s response model=%s stop=%s tokens_in=%d tokens_out=%d text_len=%d tool_calls=%d",
		providerName, result.Model, result.StopReason,
		result.Usage.InputTokens, result.Usage.OutputTokens, len(result.Text), len(result.ToolCalls))

	return result, nil
}

func parseOpenAIChatCompletionsResponse(resp *http.Response, modelID string, providerName string) (*LLMResponse, error) {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logRejectedRequest(providerName, resp)
		return nil, fmt.Errorf("%s: status %s (sanitized)", providerName, resp.Status)
	}

	var payload openAIChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%s: decode chat completion response: %w", providerName, err)
	}

	result := &LLMResponse{
		ID:           payload.ID,
		Model:        coalesce(payload.Model, modelID),
		Usage:        payload.Usage.usage(),
		ProviderName: providerName,
	}

	if len(payload.Choices) == 0 {
		result.StopReason = "end_turn"
		return result, nil
	}
	choice := payload.Choices[0]
	if content, ok := choice.Message.Content.(string); ok {
		result.Text = content
	}
	result.ReasoningContent = choice.Message.ReasoningContent
	switch choice.FinishReason {
	case "tool_calls":
		result.StopReason = "tool_use"
	case "length":
		result.StopReason = "max_tokens"
	case "", "stop":
		result.StopReason = "end_turn"
	default:
		result.StopReason = choice.FinishReason
	}
	for _, call := range choice.Message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ContentToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: canonicalJSON(json.RawMessage(call.Function.Arguments)),
		})
	}
	// A forced tool_choice can answer finish_reason "stop" with the calls
	// present (Synthetic does); the calls make it tool use.
	if len(result.ToolCalls) > 0 && result.StopReason == "end_turn" {
		result.StopReason = "tool_use"
	}

	RecordCacheUsage(providerName, result.Usage)
	log.Printf("provider: %s chat completion model=%s stop=%s tokens_in=%d tokens_out=%d cached_in=%d cache_reported=%t text_len=%d tool_calls=%d",
		providerName, result.Model, result.StopReason, result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.CachedInputTokens, result.Usage.CacheReported, len(result.Text), len(result.ToolCalls))

	return result, nil
}

func parseOpenAIChatCompletionsStream(body io.Reader, modelID string, providerName string, onChunk func(StreamChunk)) (*LLMResponse, error) {
	result := &LLMResponse{
		Model:        modelID,
		ProviderName: providerName,
	}
	type toolBuffer struct {
		callID string
		name   string
		args   string
		open   bool
	}
	tools := map[int]*toolBuffer{}
	started := false
	completed := false

	emitStart := func() {
		if started {
			return
		}
		started = true
		onChunk(StreamChunk{Type: "message_start", ID: result.ID, Model: result.Model})
	}
	flushTools := func() {
		// The model's order: index ascending, not map order.
		order := make([]int, 0, len(tools))
		for idx := range tools {
			order = append(order, idx)
		}
		sort.Ints(order)
		for _, idx := range order {
			tool := tools[idx]
			if tool == nil {
				continue
			}
			result.ToolCalls = append(result.ToolCalls, ContentToolCall{
				ID:        tool.callID,
				Name:      tool.name,
				Arguments: canonicalJSON(json.RawMessage(tool.args)),
			})
			if tool.open {
				onChunk(StreamChunk{Type: "content_block_stop", ToolCallID: tool.callID, ToolCallName: tool.name})
			}
			delete(tools, idx)
		}
	}
	finish := func(reason string, usage openAIChatCompletionUsage) {
		if completed {
			return
		}
		completed = true
		hadTools := len(tools) > 0
		flushTools()
		if hadTools && (reason == "" || reason == "stop") {
			// A forced tool_choice answers "stop" with the calls present.
			reason = "tool_calls"
		}
		switch reason {
		case "tool_calls":
			result.StopReason = "tool_use"
		case "length":
			result.StopReason = "max_tokens"
		case "", "stop":
			result.StopReason = "end_turn"
		default:
			result.StopReason = reason
		}
		if usage.present() {
			result.Usage = usage.usage()
		}
		emitStart()
		onChunk(StreamChunk{
			Type:       "message_delta",
			StopReason: result.StopReason,
			Usage:      result.Usage.streamUsage(),
		})
	}

	if err := readOpenAISSE(body, func(data []byte) error {
		if len(data) == 0 || strings.TrimSpace(string(data)) == "[DONE]" {
			return nil
		}
		var payload struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content          any    `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					// Reasoning is the streamed spelling on Synthetic
					// (non-streaming and echo use reasoning_content).
					Reasoning string `json:"reasoning"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage openAIChatCompletionUsage `json:"usage"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil
		}
		if payload.ID != "" {
			result.ID = payload.ID
		}
		if payload.Model != "" {
			result.Model = payload.Model
		}
		emitStart()
		for _, choice := range payload.Choices {
			if text := chatDeltaText(choice.Delta.Content); text != "" {
				result.Text += text
				onChunk(StreamChunk{Type: "content_block_delta", Delta: text})
			}
			if reasoning := choice.Delta.ReasoningContent + choice.Delta.Reasoning; reasoning != "" {
				result.ReasoningContent += reasoning
				onChunk(StreamChunk{Type: "reasoning_delta", ReasoningDelta: reasoning})
			}
			for _, call := range choice.Delta.ToolCalls {
				tool := tools[call.Index]
				if tool == nil {
					tool = &toolBuffer{}
					tools[call.Index] = tool
				}
				if call.ID != "" {
					tool.callID = call.ID
				}
				if call.Function.Name != "" {
					tool.name = call.Function.Name
				}
				if !tool.open && (tool.callID != "" || tool.name != "") {
					tool.open = true
					onChunk(StreamChunk{Type: "content_block_start", ToolCallID: tool.callID, ToolCallName: tool.name})
				}
				if call.Function.Arguments != "" {
					tool.args += call.Function.Arguments
					onChunk(StreamChunk{Type: "content_block_delta", ToolCallDelta: call.Function.Arguments, ToolCallID: tool.callID, ToolCallName: tool.name})
				}
			}
			if choice.FinishReason != "" {
				finish(choice.FinishReason, payload.Usage)
			}
		}
		if len(payload.Choices) == 0 && payload.Usage.present() {
			result.Usage = payload.Usage.usage()
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%s: read chat completions stream: %w", providerName, err)
	}
	if !completed {
		finish("stop", openAIChatCompletionUsage{})
	}
	onChunk(StreamChunk{Type: "message_stop", StopReason: result.StopReason})
	log.Printf("provider: %s chat completion stream model=%s stop=%s tokens_in=%d tokens_out=%d text_len=%d tool_calls=%d",
		providerName, result.Model, result.StopReason, result.Usage.InputTokens, result.Usage.OutputTokens, len(result.Text), len(result.ToolCalls))
	return result, nil
}

func chatDeltaText(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, part := range v {
			if m, ok := part.(map[string]any); ok {
				b.WriteString(stringValue(m["text"]))
			}
		}
		return b.String()
	default:
		return ""
	}
}

func coalesce(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// canonicalJSON normalizes a JSON raw message: trims whitespace,
// unmarshals and re-marshals to ensure consistent formatting, and
// returns "{}" for empty or null input. This matches the Cogent
// reference behavior for tool_use input arguments.
func canonicalJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage([]byte("{}"))
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage([]byte("{}"))
	}
	var v any
	if err := json.Unmarshal(trimmed, &v); err != nil {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return b
}

// parseSSEStream reads an Anthropic-compatible SSE stream from the response
// body, calling onChunk for each relevant event, and returns the accumulated
// LLMResponse. This handles the standard Anthropic Messages API streaming
// format used by both Z.AI and Fireworks AI.
//
// Event flow:
//  1. message_start — contains response ID, model, initial usage
//  2. content_block_start — indicates a new content block (text or tool_use)
//  3. content_block_delta — incremental text (text_delta) or tool input (input_json_delta)
//  4. content_block_stop — end of content block
//  5. message_delta — final stop_reason and cumulative output token count
//  6. message_stop — stream complete
//
// Unknown events (ping, error, etc.) are silently skipped.
func parseSSEStream(body io.Reader, modelID string, providerName string, onChunk func(StreamChunk)) (*LLMResponse, error) {
	result := &LLMResponse{
		Model:        modelID,
		ProviderName: providerName,
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	var toolCallInputs map[int]*struct { // index → accumulated JSON
		id   string
		name string
		json strings.Builder
	}
	toolCallInputs = make(map[int]*struct {
		id   string
		name string
		json strings.Builder
	})

	for scanner.Scan() {
		line := scanner.Text()

		// SSE lines starting with "event: " set the event type.
		// We don't need to track this — the event type is in the JSON data.
		if strings.HasPrefix(line, "event: ") {
			continue
		}

		// SSE lines starting with "data: " contain the payload.
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// "[DONE]" signals stream completion.
		if data == "[DONE]" {
			break
		}

		var event struct {
			Type         string          `json:"type"`
			Index        int             `json:"index"`
			Model        string          `json:"model"`
			ID           string          `json:"id"`
			Delta        json.RawMessage `json:"delta"`
			Message      json.RawMessage `json:"message"`
			ContentBlock json.RawMessage `json:"content_block"`
			Usage        anthropicUsage  `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// Skip malformed events.
			continue
		}

		switch event.Type {
		case "message_start":
			// Extract message fields.
			var msg struct {
				ID    string         `json:"id"`
				Model string         `json:"model"`
				Usage anthropicUsage `json:"usage"`
			}
			if err := json.Unmarshal(event.Message, &msg); err == nil {
				result.ID = msg.ID
				if msg.Model != "" {
					result.Model = msg.Model
				}
				start := msg.Usage.usage()
				result.Usage.InputTokens = start.InputTokens
				result.Usage.CachedInputTokens = start.CachedInputTokens
				result.Usage.CacheReported = start.CacheReported
			}
			onChunk(StreamChunk{
				Type:  "message_start",
				ID:    result.ID,
				Model: result.Model,
				Usage: &StreamUsage{InputTokens: result.Usage.InputTokens, CachedInputTokens: result.Usage.CachedInputTokens, CacheReported: result.Usage.CacheReported},
			})

		case "content_block_start":
			// Check if this is a tool_use block.
			var cb struct {
				Type  string          `json:"type"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			}
			if err := json.Unmarshal(event.ContentBlock, &cb); err == nil {
				if cb.Type == "tool_use" {
					toolCallInputs[event.Index] = &struct {
						id   string
						name string
						json strings.Builder
					}{id: cb.ID, name: cb.Name}
					onChunk(StreamChunk{
						Type:         "content_block_start",
						Index:        event.Index,
						ToolCallID:   cb.ID,
						ToolCallName: cb.Name,
					})
				} else {
					onChunk(StreamChunk{
						Type:  "content_block_start",
						Index: event.Index,
					})
				}
			}

		case "content_block_delta":
			// Parse delta based on type.
			var delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			}
			if err := json.Unmarshal(event.Delta, &delta); err == nil {
				switch delta.Type {
				case "text_delta":
					result.Text += delta.Text
					onChunk(StreamChunk{
						Type:  "content_block_delta",
						Delta: delta.Text,
						Index: event.Index,
					})
				case "input_json_delta":
					// Accumulate tool input JSON.
					if tc, ok := toolCallInputs[event.Index]; ok {
						tc.json.WriteString(delta.PartialJSON)
					}
					onChunk(StreamChunk{
						Type:          "content_block_delta",
						Index:         event.Index,
						ToolCallDelta: delta.PartialJSON,
					})
				}
			}

		case "content_block_stop":
			// Finalize any tool_use blocks.
			if tc, ok := toolCallInputs[event.Index]; ok {
				result.ToolCalls = append(result.ToolCalls, ContentToolCall{
					ID:        tc.id,
					Name:      tc.name,
					Arguments: canonicalJSON(json.RawMessage(tc.json.String())),
				})
			}
			onChunk(StreamChunk{
				Type:  "content_block_stop",
				Index: event.Index,
			})

		case "message_delta":
			var delta struct {
				StopReason string `json:"stop_reason"`
			}
			if err := json.Unmarshal(event.Delta, &delta); err == nil {
				result.StopReason = delta.StopReason
			}
			result.Usage.OutputTokens = event.Usage.OutputTokens
			onChunk(StreamChunk{
				Type:       "message_delta",
				StopReason: result.StopReason,
				Usage:      &StreamUsage{OutputTokens: result.Usage.OutputTokens},
			})

		case "message_stop":
			onChunk(StreamChunk{
				Type:       "message_stop",
				StopReason: result.StopReason,
			})

		case "ping":
			// Ignore ping events.

		case "error":
			var errData struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &errData); err == nil {
				log.Printf("provider: %s stream error: %s (sanitized)", providerName, errData.Error.Type)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: read stream: %w", providerName, err)
	}

	RecordCacheUsage(providerName, result.Usage)
	log.Printf("provider: %s stream complete model=%s stop=%s tokens_in=%d tokens_out=%d cached_in=%d cache_reported=%t text_len=%d tool_calls=%d",
		providerName, result.Model, result.StopReason,
		result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.CachedInputTokens, result.Usage.CacheReported, len(result.Text), len(result.ToolCalls))

	return result, nil
}

// responsesUsage maps a Responses API usage object, whose cache report is
// input_tokens_details.cached_tokens.
func responsesUsage(usage map[string]any) Usage {
	out := Usage{
		InputTokens:  int(numberValue(usage["input_tokens"])),
		OutputTokens: int(numberValue(usage["output_tokens"])),
	}
	if details, ok := usage["input_tokens_details"].(map[string]any); ok {
		if cached, ok := details["cached_tokens"]; ok {
			out.CacheReported = true
			out.CachedInputTokens = int(numberValue(cached))
		}
	}
	return out.normalizeCache()
}

func parseOpenAIStream(body io.Reader, modelID string, providerName string, onChunk func(StreamChunk)) (*LLMResponse, error) {
	return parseOpenAIStreamLogged(body, modelID, providerName, onChunk, nil)
}

// parseOpenAIStreamLogged is parseOpenAIStream plus request cache metadata
// (nil for none) appended to the per-call usage line.
func parseOpenAIStreamLogged(body io.Reader, modelID string, providerName string, onChunk func(StreamChunk), meta *cacheLogMeta) (*LLMResponse, error) {
	result := &LLMResponse{
		Model:        modelID,
		ProviderName: providerName,
	}
	type toolBuffer struct {
		callID string
		name   string
		args   string
	}
	tools := map[string]*toolBuffer{}
	// The Responses stream reports each function call twice (arguments.done
	// and output_item.done); record each call id once.
	recorded := map[string]bool{}
	appendToolCall := func(call ContentToolCall) {
		if call.ID != "" && recorded[call.ID] {
			return
		}
		recorded[call.ID] = true
		result.ToolCalls = append(result.ToolCalls, call)
		result.StopReason = "tool_use"
	}

	if err := readOpenAISSE(body, func(data []byte) error {
		if len(data) == 0 || strings.TrimSpace(string(data)) == "[DONE]" {
			return nil
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			return nil
		}
		typ := stringValue(payload["type"])
		switch typ {
		case "response.created":
			if response, ok := payload["response"].(map[string]any); ok {
				result.ID = stringValue(response["id"])
				if model := stringValue(response["model"]); model != "" {
					result.Model = model
				}
				onChunk(StreamChunk{Type: "message_start", ID: result.ID, Model: result.Model})
			}
		case "response.output_text.delta":
			text := stringValue(payload["delta"])
			if text != "" {
				result.Text += text
				onChunk(StreamChunk{Type: "content_block_delta", Delta: text})
			}
		case "response.output_item.added":
			item, _ := payload["item"].(map[string]any)
			if stringValue(item["type"]) == "function_call" {
				key := stringValue(item["id"])
				tools[key] = &toolBuffer{
					callID: stringValue(item["call_id"]),
					name:   stringValue(item["name"]),
					args:   stringValue(item["arguments"]),
				}
				onChunk(StreamChunk{Type: "content_block_start", ToolCallID: tools[key].callID, ToolCallName: tools[key].name})
			}
		case "response.function_call_arguments.delta":
			key := stringValue(payload["item_id"])
			if tools[key] == nil {
				tools[key] = &toolBuffer{}
			}
			tools[key].args += stringValue(payload["delta"])
			if tools[key].callID == "" {
				tools[key].callID = stringValue(payload["call_id"])
			}
			if tools[key].name == "" {
				tools[key].name = stringValue(payload["name"])
			}
			onChunk(StreamChunk{Type: "content_block_delta", ToolCallDelta: stringValue(payload["delta"]), ToolCallID: tools[key].callID, ToolCallName: tools[key].name})
		case "response.function_call_arguments.done":
			key := stringValue(payload["item_id"])
			if tools[key] == nil {
				tools[key] = &toolBuffer{}
			}
			if arguments := stringValue(payload["arguments"]); arguments != "" {
				tools[key].args = arguments
			}
			if tools[key].callID == "" {
				tools[key].callID = stringValue(payload["call_id"])
			}
			if tools[key].name == "" {
				tools[key].name = stringValue(payload["name"])
			}
			appendToolCall(ContentToolCall{
				ID:        tools[key].callID,
				Name:      tools[key].name,
				Arguments: canonicalJSON(json.RawMessage(tools[key].args)),
			})
			onChunk(StreamChunk{Type: "content_block_stop", ToolCallID: tools[key].callID, ToolCallName: tools[key].name})
			delete(tools, key)
		case "response.output_item.done":
			item, _ := payload["item"].(map[string]any)
			if stringValue(item["type"]) == "function_call" {
				args := stringValue(item["arguments"])
				appendToolCall(ContentToolCall{
					ID:        stringValue(item["call_id"]),
					Name:      stringValue(item["name"]),
					Arguments: canonicalJSON(json.RawMessage(args)),
				})
			}
		case "response.completed", "response.incomplete":
			response, _ := payload["response"].(map[string]any)
			if response != nil {
				result.ID = stringValue(response["id"])
				if model := stringValue(response["model"]); model != "" {
					result.Model = model
				}
				if usage, ok := response["usage"].(map[string]any); ok {
					result.Usage = responsesUsage(usage)
				}
				if typ == "response.incomplete" {
					if details, ok := response["incomplete_details"].(map[string]any); ok {
						if reason := stringValue(details["reason"]); reason != "" {
							result.StopReason = reason
						}
					}
				}
			}
			if result.StopReason == "" {
				result.StopReason = "end_turn"
			}
			onChunk(StreamChunk{
				Type:       "message_delta",
				StopReason: result.StopReason,
				Usage:      result.Usage.streamUsage(),
			})
		case "response.failed":
			response, _ := payload["response"].(map[string]any)
			errType := "failed"
			if response != nil {
				if e, ok := response["error"].(map[string]any); ok {
					if code := stringValue(e["code"]); code != "" {
						errType = code
					}
				}
			}
			return fmt.Errorf("%s: response %s (sanitized)", providerName, errType)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("%s: read stream: %w", providerName, err)
	}

	onChunk(StreamChunk{Type: "message_stop", StopReason: result.StopReason})
	RecordCacheUsage(providerName, result.Usage)
	log.Printf("provider: %s stream complete model=%s stop=%s tokens_in=%d tokens_out=%d cached_in=%d cache_reported=%t text_len=%d tool_calls=%d%s",
		providerName, result.Model, result.StopReason, result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.CachedInputTokens, result.Usage.CacheReported, len(result.Text), len(result.ToolCalls), meta.logFields())
	return result, nil
}

type ChatGPTAuthOptions struct {
	Path          string
	RefreshURL    string
	ClientID      string
	RefreshBefore time.Duration
	HTTPClient    *http.Client
	Now           func() time.Time
}

const defaultChatGPTOAuthClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

type ChatGPTAuth struct {
	path          string
	refreshURL    string
	clientID      string
	refreshBefore time.Duration
	httpClient    *http.Client
	now           func() time.Time
	mu            sync.Mutex
}

type codexAuthFile struct {
	AuthMode     string          `json:"auth_mode"`
	OpenAIAPIKey string          `json:"OPENAI_API_KEY"`
	ClientID     string          `json:"client_id,omitempty"`
	Tokens       codexAuthTokens `json:"tokens"`
	LastRefresh  string          `json:"last_refresh"`
}

type codexAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	AccountID    string `json:"account_id"`
	ClientID     string `json:"client_id,omitempty"`
}

type oauthRefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// sharedChatGPTAuths holds the process's one ChatGPTAuth per auth path.
var sharedChatGPTAuths sync.Map

// SharedChatGPTAuth returns the process's one ChatGPTAuth for path (the
// default Codex path when empty). Refresh tokens rotate, so every user of an
// auth file (each inference model, the search adapter) must refresh under
// one mutex: two refreshers spending the same refresh token make the second
// fail.
func SharedChatGPTAuth(path string) *ChatGPTAuth {
	auth := NewChatGPTAuth(ChatGPTAuthOptions{Path: path})
	shared, _ := sharedChatGPTAuths.LoadOrStore(filepath.Clean(auth.path), auth)
	return shared.(*ChatGPTAuth)
}

func NewChatGPTAuth(opts ChatGPTAuthOptions) *ChatGPTAuth {
	auth := &ChatGPTAuth{
		path:          opts.Path,
		refreshURL:    opts.RefreshURL,
		clientID:      strings.TrimSpace(opts.ClientID),
		refreshBefore: opts.RefreshBefore,
		httpClient:    opts.HTTPClient,
		now:           opts.Now,
	}
	if auth.path == "" {
		auth.path = filepath.Join(userHomeDir(), ".codex", "auth.json")
	}
	if auth.refreshURL == "" {
		auth.refreshURL = "https://auth.openai.com/oauth/token"
	}
	if auth.clientID == "" {
		auth.clientID = strings.TrimSpace(os.Getenv("CHATGPT_CLIENT_ID"))
	}
	if auth.clientID == "" {
		auth.clientID = defaultChatGPTOAuthClientID
	}
	if auth.refreshBefore == 0 {
		auth.refreshBefore = 45 * time.Minute
	}
	if auth.httpClient == nil {
		auth.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if auth.now == nil {
		auth.now = time.Now
	}
	return auth
}

func (a *ChatGPTAuth) Header(ctx context.Context) (string, error) {
	record, err := a.AccessToken(ctx)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(record.Tokens.AccessToken)
	if token == "" {
		return "", fmt.Errorf("chatgpt auth missing access token")
	}
	return "Bearer " + token, nil
}

func (a *ChatGPTAuth) AccessToken(ctx context.Context) (*codexAuthFile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	record, err := a.Read()
	if err != nil {
		return nil, err
	}
	if !a.needsRefresh(record) {
		return record, nil
	}
	refreshed, err := a.refresh(ctx, record)
	if err == nil {
		return refreshed, nil
	}
	return nil, err
}

func (a *ChatGPTAuth) ForceRefresh(ctx context.Context) (*codexAuthFile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	record, err := a.Read()
	if err != nil {
		return nil, err
	}
	return a.refresh(ctx, record)
}

func (a *ChatGPTAuth) Read() (*codexAuthFile, error) {
	data, err := os.ReadFile(a.path)
	if err != nil {
		return nil, fmt.Errorf("read chatgpt auth file: %w", err)
	}
	var record codexAuthFile
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode chatgpt auth file: %w", err)
	}
	return &record, nil
}

func (a *ChatGPTAuth) needsRefresh(record *codexAuthFile) bool {
	if record == nil || strings.TrimSpace(record.Tokens.AccessToken) == "" {
		return true
	}
	if exp, ok := jwtExpiry(record.Tokens.AccessToken); ok {
		return !a.now().UTC().Before(exp.UTC().Add(-a.refreshBefore))
	}
	if strings.TrimSpace(record.LastRefresh) == "" {
		return false
	}
	lastRefresh, err := time.Parse(time.RFC3339, record.LastRefresh)
	if err != nil {
		return false
	}
	return a.now().UTC().After(lastRefresh.UTC().Add(a.refreshBefore))
}

func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0).UTC(), true
}

func (a *ChatGPTAuth) refresh(ctx context.Context, record *codexAuthFile) (*codexAuthFile, error) {
	if record == nil {
		return nil, fmt.Errorf("refresh chatgpt auth: missing auth record")
	}
	if strings.TrimSpace(record.Tokens.RefreshToken) == "" {
		return nil, fmt.Errorf("refresh chatgpt auth: missing refresh token")
	}
	refreshed, err := a.refreshViaHTTP(ctx, record)
	if err == nil {
		return refreshed, nil
	}
	if a.usesDefaultAuthPath() {
		if cliErr := a.refreshViaCodexCLI(ctx); cliErr != nil {
			return nil, err
		}
		return a.Read()
	}
	return nil, err
}

func (a *ChatGPTAuth) usesDefaultAuthPath() bool {
	return filepath.Clean(a.path) == filepath.Clean(filepath.Join(userHomeDir(), ".codex", "auth.json"))
}

func (a *ChatGPTAuth) refreshViaHTTP(ctx context.Context, record *codexAuthFile) (*codexAuthFile, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", record.Tokens.RefreshToken)
	values.Set("client_id", a.refreshClientID(record))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.refreshURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refresh chatgpt auth via http: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		return nil, fmt.Errorf("refresh chatgpt auth via http: status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload oauthRefreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode chatgpt refresh response: %w", err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		return nil, fmt.Errorf("chatgpt refresh response missing access_token")
	}

	record.Tokens.AccessToken = payload.AccessToken
	if payload.RefreshToken != "" {
		record.Tokens.RefreshToken = payload.RefreshToken
	}
	if payload.IDToken != "" {
		record.Tokens.IDToken = payload.IDToken
	}
	record.LastRefresh = a.now().UTC().Format(time.RFC3339)
	if err := a.write(record); err != nil {
		return nil, err
	}
	return record, nil
}

func (a *ChatGPTAuth) refreshClientID(record *codexAuthFile) string {
	if record != nil {
		if clientID := strings.TrimSpace(record.ClientID); clientID != "" {
			return clientID
		}
		if clientID := strings.TrimSpace(record.Tokens.ClientID); clientID != "" {
			return clientID
		}
	}
	return a.clientID
}

func (a *ChatGPTAuth) refreshViaCodexCLI(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "codex", "login", "status")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("refresh chatgpt auth via codex cli: %w", err)
	}
	return nil
}

func (a *ChatGPTAuth) write(record *codexAuthFile) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode refreshed chatgpt auth file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o700); err != nil {
		return fmt.Errorf("create chatgpt auth dir: %w", err)
	}
	// Replace the file whole by rename: a concurrent Read never parses a
	// half-written file.
	tmp, err := os.CreateTemp(filepath.Dir(a.path), filepath.Base(a.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("write chatgpt auth file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write chatgpt auth file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write chatgpt auth file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write chatgpt auth file: %w", err)
	}
	if err := os.Rename(tmp.Name(), a.path); err != nil {
		return fmt.Errorf("write chatgpt auth file: %w", err)
	}
	return nil
}

func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "."
}

func readOpenAISSE(body io.Reader, fn func([]byte) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var data []byte
	dispatch := func() error {
		if len(data) == 0 {
			return nil
		}
		payload := bytes.TrimSpace(data)
		data = nil
		return fn(payload)
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))...)
			data = append(data, '\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return dispatch()
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func numberValue(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

type ModelInfo = modelcatalog.ModelInfo

// SupportedModels returns the list of models that the provider package
// can serve. These are derived from the Droid settings.json customModels
// configuration.
func SupportedModels() []ModelInfo {
	return modelcatalog.SupportedModels()
}

// MultiProvider holds provider backends keyed by provider name. Model
// selection stays on LLMRequest.Model; provider names must not encode model
// names.
//
// Usage: callers initialize specific providers and register them; the
// MultiProvider simply stores them for later retrieval. Model routing is
// explicitly out of scope for this type.
type MultiProvider struct {
	providers map[string]Provider
}

// NewMultiProvider creates an empty MultiProvider.
func NewMultiProvider() *MultiProvider {
	return &MultiProvider{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider under the given name. If a provider with the
// same name already exists it is replaced.
func (mp *MultiProvider) Register(name string, p Provider) {
	mp.providers[name] = p
}

// Get returns the provider registered under the given name, or nil.
func (mp *MultiProvider) Get(name string) Provider {
	return mp.providers[name]
}

// Names returns all registered provider names.
func (mp *MultiProvider) Names() []string {
	names := make([]string, 0, len(mp.providers))
	for name := range mp.providers {
		names = append(names, name)
	}
	return names
}

// ResolveAll creates and registers all providers for which credentials are
// available in the environment, using the given ProviderConfig for model
// selection. It returns the MultiProvider with whatever providers could be
// initialized (may be empty if no credentials are set).
//
// Model selection is a runtime concern passed via cfg; credentials are
// resolved from environment variables. Model routing is out of scope.
func ResolveAll(cfg ProviderConfig) *MultiProvider {
	mp := NewMultiProvider()

	// Try ChatGPT. Register one provider; req.Model selects the model.
	if len(cfg.ChatGPTModels) > 0 {
		if p, err := NewChatGPTProviderFromEnv(cfg.ChatGPTModels[0], cfg.ChatGPTReasoningEffort); err == nil {
			mp.Register("chatgpt", p)
			log.Printf("provider: resolved chatgpt (seed_model=%s reasoning=%s)", p.modelID, p.reasoning)
		}
	}

	// Try Bedrock. Register one provider; req.Model selects the model.
	if os.Getenv("AWS_BEARER_TOKEN_BEDROCK") != "" && len(cfg.BedrockModels) > 0 {
		if p, err := NewBedrockProviderFromEnv(cfg.BedrockModels[0]); err == nil {
			mp.Register("bedrock", p)
			log.Printf("provider: resolved bedrock (region=%s seed_model=%s)", p.region, redactModel(p.modelID))
		}
	}

	// Try Z.AI. Register one provider; req.Model selects the model.
	if os.Getenv("ZAI_API_KEY") != "" && len(cfg.ZAIModels) > 0 {
		if p, err := NewZAIProviderFromEnv(cfg.ZAIModels[0]); err == nil {
			mp.Register("zai", p)
			log.Printf("provider: resolved zai (seed_model=%s)", p.modelID)
		}
	}

	// Try Fireworks. Register one provider; req.Model selects the model.
	if os.Getenv("FIREWORKS_API_KEY") != "" && len(cfg.FireworksModels) > 0 {
		if p, err := NewFireworksProvider(FireworksConfig{
			APIKey:          os.Getenv("FIREWORKS_API_KEY"),
			ModelID:         cfg.FireworksModels[0],
			BaseURL:         os.Getenv("FIREWORKS_BASE_URL"),
			ReasoningEffort: cfg.FireworksReasoningEffort,
		}); err == nil {
			mp.Register("fireworks", p)
			log.Printf("provider: resolved fireworks (seed_model=%s reasoning=%s)", p.modelID, p.reasoning)
		}
	}

	// Try OpenCode Go and Zen. Both use the same primary credential with the
	// historical Go-specific variable accepted only as a fallback.
	if openCodeAPIKeyFromEnv() != "" && len(cfg.OpenCodeGoModels) > 0 {
		if p, err := NewOpenCodeGoProviderFromEnv(cfg.OpenCodeGoModels[0]); err == nil {
			mp.Register("opencode-go", p)
			log.Printf("provider: resolved opencode-go (seed_model=%s)", p.modelID)
		}
	}
	if openCodeAPIKeyFromEnv() != "" && len(cfg.OpenCodeZenModels) > 0 {
		if p, err := NewOpenCodeZenProviderFromEnv(cfg.OpenCodeZenModels[0]); err == nil {
			mp.Register("opencode-zen", p)
			log.Printf("provider: resolved opencode-zen (seed_model=%s)", p.modelID)
		}
	}

	// Try Synthetic. Register one provider; req.Model selects the model.
	if os.Getenv("SYNTHETIC_API_KEY") != "" && len(cfg.SyntheticModels) > 0 {
		if p, err := NewSyntheticProviderFromEnv(cfg.SyntheticModels[0]); err == nil {
			mp.Register("synthetic", p)
			log.Printf("provider: resolved synthetic (seed_model=%s)", p.modelID)
		}
	}

	return mp
}

// ResolveProvider resolves the explicitly selected provider for direct
// autoputer mode. It does not guess or fall back; callers that want a provider
// must set ProviderConfig.SelectedProvider.
func ResolveProvider(cfg ProviderConfig) (Provider, error) {
	selected := strings.TrimSpace(cfg.SelectedProvider)
	if selected == "" {
		return nil, nil
	}
	mp := ResolveAll(cfg)
	p := mp.Get(selected)
	if p == nil {
		return nil, fmt.Errorf("resolve provider: selected provider %q is not configured", selected)
	}
	return p, nil
}
