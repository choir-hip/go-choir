// Package gatewayruntime adapts a sandbox runtime to the host gateway without
// importing host-side provider adapters. VM guests should only know the gateway
// wire contract; provider credentials and upstream adapter code stay host-side.
package gatewayruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Provider routes runtime LLM calls through the host gateway.
type Provider struct {
	gatewayURL      string
	token           string
	httpClient      *http.Client
	llmProvider     string
	llmModel        string
	reasoningEffort string
}

const gatewayClientTimeout = 5*time.Minute + 30*time.Second

// New creates a gateway-routed runtime provider.
func New(gatewayURL, token string) *Provider {
	return &Provider{
		gatewayURL: strings.TrimRight(strings.TrimSpace(gatewayURL), "/"),
		token:      strings.TrimSpace(token),
		httpClient: &http.Client{Timeout: gatewayClientTimeout},
	}
}

// SetRuntimeLLMConfig sets the explicit runtime LLM selection for gateway calls.
func (p *Provider) SetRuntimeLLMConfig(providerName, model, reasoningEffort string) {
	p.llmProvider = strings.TrimSpace(providerName)
	p.llmModel = strings.TrimSpace(model)
	p.reasoningEffort = strings.TrimSpace(reasoningEffort)
}

// ProviderName reports the runtime-visible provider boundary.
func (p *Provider) ProviderName() string { return "gateway" }

// RuntimeProviderPolicy reports gateway-routed model policy.
func (p *Provider) RuntimeProviderPolicy() runtime.ProviderPolicy {
	return runtime.ProviderPolicy{
		ActiveProvider:              "gateway",
		DefaultModel:                p.llmModel,
		ModelSelection:              "The sandbox routes through the host gateway with explicit runtime provider/model configuration.",
		SupportsPerRunModelOverride: true,
		Notes: []string{
			"Gateway-routed mode keeps provider credentials on the host side.",
			"Provider/model selection is supplied by runtime configuration, not inferred by the gateway.",
		},
	}
}

// Execute implements runtime.Provider.
func (p *Provider) Execute(ctx context.Context, task *types.RunRecord, emit runtime.EventEmitFunc) error {
	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started","provider":"gateway","routed":true}`))

	llmConfig := runtime.ResolvedLLMConfigFromMetadata(task.Metadata)
	providerName := firstNonEmpty(llmConfig.Provider, p.llmProvider)
	model := firstNonEmpty(llmConfig.Model, p.llmModel)
	reasoning := firstNonEmpty(llmConfig.ReasoningEffort, p.reasoningEffort)

	req := llmRequest{
		Provider:        providerName,
		Model:           model,
		System:          "You are a helpful assistant running inside the ChoirOS sandbox runtime. Respond concisely and helpfully.",
		ReasoningEffort: reasoning,
		Messages: []message{
			{Role: "user", Content: []block{{Type: "text", Text: task.Prompt}}},
		},
		MaxTokens: modelcatalog.MaxOutputTokensForModel(model),
		Stream:    true,
	}

	log.Printf("gateway-runtime: streaming task %s through gateway (prompt_len=%d)", task.RunID, len(task.Prompt))
	resp, err := p.stream(ctx, req, func(chunk streamChunk) {
		if chunk.Delta == "" {
			return
		}
		deltaPayload, _ := json.Marshal(map[string]string{
			"text":     chunk.Delta,
			"provider": "gateway",
			"routed":   "true",
		})
		emit(types.EventRunDelta, "execution", deltaPayload)
	})
	if err != nil {
		failPayload, _ := json.Marshal(map[string]string{
			"status":   "failed",
			"provider": "gateway",
			"routed":   "true",
			"error":    err.Error(),
		})
		emit(types.EventRunProgress, "execution", failPayload)
		return fmt.Errorf("gateway call failed: %w", err)
	}

	progressPayload, _ := json.Marshal(map[string]string{
		"status":      "completed",
		"provider":    resp.ProviderName,
		"routed":      "true",
		"model":       resp.Model,
		"stop_reason": resp.StopReason,
		"tokens_in":   fmt.Sprintf("%d", resp.Usage.InputTokens),
		"tokens_out":  fmt.Sprintf("%d", resp.Usage.OutputTokens),
	})
	emit(types.EventRunProgress, "execution", progressPayload)
	task.Result = resp.Text
	return nil
}

// CallWithTools implements runtime.ToolLoopProvider.
func (p *Provider) CallWithTools(ctx context.Context, req runtime.ToolLoopRequest) (*runtime.ToolLoopResponse, error) {
	llmReq := llmRequest{
		Provider:        firstNonEmpty(req.Provider, p.llmProvider),
		Model:           firstNonEmpty(req.Model, p.llmModel),
		System:          req.System,
		Messages:        convertRawMessages(req.Messages),
		Tools:           convertToolLoopDefs(req.ToolDefinitions),
		MaxTokens:       req.MaxTokens,
		Stream:          false,
		ReasoningEffort: firstNonEmpty(req.ReasoningEffort, p.reasoningEffort),
	}

	resp, err := p.call(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("gateway call failed: %w", err)
	}

	out := &runtime.ToolLoopResponse{
		ID:         resp.ID,
		StopReason: convertStopReason(resp.StopReason),
		Text:       resp.Text,
		Usage: runtime.TokenUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
		},
		Model: resp.Model,
	}
	if out.StopReason == "tool_use" {
		out.ToolCalls = extractToolCalls(resp)
	}
	return out, nil
}

func (p *Provider) call(ctx context.Context, req llmRequest) (*llmResponse, error) {
	req.Stream = false
	httpResp, err := p.do(ctx, req, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("gateway client: read response: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, gatewayStatusError(httpResp.Status, body)
	}
	var out llmResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("gateway client: decode response: %w", err)
	}
	return &out, nil
}

func (p *Provider) stream(ctx context.Context, req llmRequest, onChunk func(streamChunk)) (*llmResponse, error) {
	req.Stream = true
	httpResp, err := p.do(ctx, req, "text/event-stream")
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()
	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return nil, gatewayStatusError(httpResp.Status, body)
	}
	return parseGatewaySSE(httpResp.Body, onChunk)
}

func (p *Provider) do(ctx context.Context, req llmRequest, accept string) (*http.Response, error) {
	if p.gatewayURL == "" {
		return nil, fmt.Errorf("gateway client: missing gateway URL")
	}
	if p.token == "" {
		return nil, fmt.Errorf("gateway client: missing sandbox credential")
	}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("gateway client: marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.gatewayURL+"/provider/v1/inference", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gateway client: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.token)
	if accept != "" {
		httpReq.Header.Set("Accept", accept)
	}
	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gateway client: http call: %w", err)
	}
	return resp, nil
}

func gatewayStatusError(status string, body []byte) error {
	var errResp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return fmt.Errorf("gateway client: %s", errResp.Error)
	}
	return fmt.Errorf("gateway client: status %s (sanitized)", status)
}

func parseGatewaySSE(body io.Reader, onChunk func(streamChunk)) (*llmResponse, error) {
	var accumulated llmResponse
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("gateway-runtime: unmarshal stream chunk: %v", err)
			continue
		}
		if chunk.Type == "error" {
			return nil, fmt.Errorf("gateway client: stream error: %s", chunk.Delta)
		}
		onChunk(chunk)
		if chunk.ID != "" {
			accumulated.ID = chunk.ID
		}
		if chunk.Model != "" {
			accumulated.Model = chunk.Model
		}
		if chunk.Delta != "" {
			accumulated.Text += chunk.Delta
		}
		if chunk.StopReason != "" {
			accumulated.StopReason = chunk.StopReason
		}
		if chunk.Usage != nil {
			accumulated.Usage.InputTokens = chunk.Usage.InputTokens
			accumulated.Usage.OutputTokens = chunk.Usage.OutputTokens
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("gateway client: read stream: %w", err)
	}
	accumulated.ProviderName = "gateway"
	return &accumulated, nil
}

func convertRawMessages(raw []json.RawMessage) []message {
	out := make([]message, 0, len(raw))
	for _, r := range raw {
		var msg struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(r, &msg); err != nil {
			continue
		}
		var blocks []struct {
			Type      string          `json:"type"`
			Text      string          `json:"text,omitempty"`
			Source    *mediaSource    `json:"source,omitempty"`
			ToolUseID string          `json:"tool_use_id,omitempty"`
			ID        string          `json:"id,omitempty"`
			Name      string          `json:"name,omitempty"`
			Input     json.RawMessage `json:"input,omitempty"`
			Content   string          `json:"content,omitempty"`
			IsError   bool            `json:"is_error,omitempty"`
		}
		if err := json.Unmarshal(msg.Content, &blocks); err != nil {
			var text string
			if err := json.Unmarshal(msg.Content, &text); err == nil {
				out = append(out, message{Role: msg.Role, Content: []block{{Type: "text", Text: text}}})
			}
			continue
		}
		content := make([]block, 0, len(blocks))
		for _, b := range blocks {
			switch b.Type {
			case "text":
				content = append(content, block{Type: "text", Text: b.Text})
			case "image":
				content = append(content, block{Type: "image", Source: b.Source})
			case "tool_result":
				text := b.Content
				if text == "" {
					text = b.Text
				}
				content = append(content, block{Type: "tool_result", Text: text, ToolUseID: b.ToolUseID, IsError: b.IsError})
			case "tool_use":
				content = append(content, block{Type: "tool_use", ID: b.ID, Name: b.Name, Input: canonicalJSON(b.Input)})
			}
		}
		if len(content) > 0 {
			out = append(out, message{Role: msg.Role, Content: content})
		}
	}
	return out
}

func convertToolLoopDefs(defs []runtime.ToolDefinition) []toolDef {
	if len(defs) == 0 {
		return nil
	}
	out := make([]toolDef, 0, len(defs))
	for _, def := range defs {
		out = append(out, toolDef{Name: def.Name, Description: def.Description, InputSchema: def.Parameters})
	}
	return out
}

func extractToolCalls(resp *llmResponse) []types.ToolCall {
	if len(resp.ToolCalls) == 0 {
		return nil
	}
	calls := make([]types.ToolCall, 0, len(resp.ToolCalls))
	for _, tc := range resp.ToolCalls {
		calls = append(calls, types.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: canonicalJSON(tc.Arguments)})
	}
	return calls
}

func convertStopReason(reason string) string {
	switch reason {
	case "end_turn", "stop":
		return "end_turn"
	case "tool_use":
		return "tool_use"
	default:
		return reason
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func canonicalJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return b
}

type llmRequest struct {
	Provider        string    `json:"provider,omitempty"`
	Model           string    `json:"model,omitempty"`
	Messages        []message `json:"messages"`
	System          string    `json:"system,omitempty"`
	Tools           []toolDef `json:"tools,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	Stream          bool      `json:"stream,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

type message struct {
	Role    string  `json:"role"`
	Content []block `json:"content"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Source    *mediaSource    `json:"source,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type mediaSource struct {
	Kind     string `json:"kind"`
	MIMEType string `json:"mime_type,omitempty"`
	Data     string `json:"data,omitempty"`
	URL      string `json:"url,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type llmResponse struct {
	ID           string            `json:"id"`
	Text         string            `json:"text"`
	Model        string            `json:"model"`
	StopReason   string            `json:"stop_reason"`
	Usage        tokenUsage        `json:"usage"`
	ToolCalls    []contentToolCall `json:"tool_calls,omitempty"`
	ProviderName string            `json:"provider_name"`
}

type tokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type contentToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type streamChunk struct {
	Type          string       `json:"type"`
	Delta         string       `json:"delta,omitempty"`
	ToolCallDelta string       `json:"tool_call_delta,omitempty"`
	ToolCallID    string       `json:"tool_call_id,omitempty"`
	ToolCallName  string       `json:"tool_call_name,omitempty"`
	StopReason    string       `json:"stop_reason,omitempty"`
	Usage         *streamUsage `json:"usage,omitempty"`
	Index         int          `json:"index,omitempty"`
	Model         string       `json:"model,omitempty"`
	ID            string       `json:"id,omitempty"`
}

type streamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
