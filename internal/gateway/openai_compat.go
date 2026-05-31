package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

type openAIChatCompletionRequest struct {
	Model               string               `json:"model"`
	Messages            []openAIChatMessage  `json:"messages"`
	Tools               []openAIChatTool     `json:"tools,omitempty"`
	ToolChoice          json.RawMessage      `json:"tool_choice,omitempty"`
	Stream              bool                 `json:"stream,omitempty"`
	ReasoningEffort     string               `json:"reasoning_effort,omitempty"`
	MaxTokens           int                  `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                  `json:"max_completion_tokens,omitempty"`
	StreamOptions       *openAIStreamOptions `json:"stream_options,omitempty"`
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type openAIChatMessage struct {
	Role             string               `json:"role"`
	Content          json.RawMessage      `json:"content,omitempty"`
	ToolCallID       string               `json:"tool_call_id,omitempty"`
	ToolCalls        []openAIChatToolCall `json:"tool_calls,omitempty"`
	ReasoningContent string               `json:"reasoning_content,omitempty"`
}

type openAIChatTool struct {
	Type     string             `json:"type"`
	Function openAIChatFunction `json:"function"`
}

type openAIChatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openAIChatToolCall struct {
	Index    int                `json:"index,omitempty"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIToolCallFunc `json:"function"`
}

type openAIToolCallFunc struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openAIChatCompletionResponse struct {
	ID                string             `json:"id"`
	Object            string             `json:"object"`
	Created           int64              `json:"created"`
	Model             string             `json:"model"`
	Choices           []openAIChatChoice `json:"choices"`
	Usage             openAIChatUsage    `json:"usage"`
	SystemFingerprint string             `json:"system_fingerprint,omitempty"`
}

type openAIChatChoice struct {
	Index        int                `json:"index"`
	Message      *openAIChatMessage `json:"message,omitempty"`
	Delta        *openAIChatDelta   `json:"delta,omitempty"`
	FinishReason string             `json:"finish_reason,omitempty"`
}

type openAIChatDelta struct {
	Role             string               `json:"role,omitempty"`
	Content          string               `json:"content,omitempty"`
	ReasoningContent string               `json:"reasoning_content,omitempty"`
	ToolCalls        []openAIChatToolCall `json:"tool_calls,omitempty"`
}

type openAIChatUsage struct {
	PromptTokens        int                       `json:"prompt_tokens"`
	CompletionTokens    int                       `json:"completion_tokens"`
	TotalTokens         int                       `json:"total_tokens"`
	PromptTokensDetails openAIPromptTokensDetails `json:"prompt_tokens_details,omitempty"`
}

type openAIPromptTokensDetails struct {
	CachedTokens int `json:"cached_tokens,omitempty"`
}

type openAIErrorEnvelope struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// HandleOpenAIChatCompletions adapts OpenAI Chat Completions requests to the
// sandbox-authenticated Choir gateway provider contract for OpenAI-compatible
// clients such as Zot.
func (h *Handler) HandleOpenAIChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, openAIErrorResponse("method not allowed"))
		return
	}

	sandboxID, ok := h.authorizeOpenAICompatibleRequest(w, r, "openai_chat")
	if !ok {
		return
	}

	var req openAIChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, openAIErrorResponse("invalid request body"))
		return
	}

	gwReq, err := convertOpenAIChatRequest(req)
	if err != nil {
		writeGatewayJSON(w, http.StatusBadRequest, openAIErrorResponse(err.Error()))
		return
	}
	p, err := h.resolveProvider(gwReq)
	if err != nil {
		log.Printf("gateway: openai-compatible provider resolution failed for sandbox %s: %v", sandboxID, err)
		writeGatewayJSON(w, http.StatusBadRequest, openAIErrorResponse(err.Error()))
		return
	}
	if p == nil {
		writeGatewayJSON(w, http.StatusServiceUnavailable, openAIErrorResponse("no provider configured"))
		return
	}

	llmReq := provider.LLMRequest{
		Provider:        gwReq.Provider,
		Model:           gwReq.Model,
		System:          gwReq.System,
		Messages:        gwReq.Messages,
		Tools:           gwReq.Tools,
		ToolChoice:      gwReq.ToolChoice,
		MaxTokens:       gwReq.MaxTokens,
		Stream:          gwReq.Stream,
		ReasoningEffort: gwReq.ReasoningEffort,
	}

	log.Printf("gateway: openai-compatible chat request from sandbox %s (provider=%s model=%s messages=%d tools=%d reasoning=%s stream=%v)",
		sandboxID, p.Name(), gwReq.Model, len(gwReq.Messages), len(gwReq.Tools), gwReq.ReasoningEffort, gwReq.Stream)

	if req.Stream {
		if len(llmReq.Tools) > 0 {
			h.handleOpenAIStreamingToolChatCompletions(w, r, p, llmReq, sandboxID)
			return
		}
		h.handleOpenAIStreamingChatCompletions(w, r, p, llmReq, sandboxID)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), inferenceTimeout)
	defer cancel()
	resp, err := p.Call(ctx, llmReq)
	if err != nil {
		sanitized := sanitizeError(err)
		log.Printf("gateway: openai-compatible provider call failed for sandbox %s: %v (sanitized: %s)", sandboxID, err, sanitized)
		writeGatewayJSON(w, http.StatusBadGateway, openAIErrorResponse(sanitized))
		return
	}

	writeGatewayJSON(w, http.StatusOK, openAIChatCompletionFromProvider(resp))
}

// HandleOpenAIModels returns a minimal OpenAI-compatible models response. Zot
// primarily uses launch flags here, but this keeps model discovery healthy for
// clients that probe the base URL.
func (h *Handler) HandleOpenAIModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeGatewayJSON(w, http.StatusMethodNotAllowed, openAIErrorResponse("method not allowed"))
		return
	}
	if _, ok := h.authorizeOpenAICompatibleRequest(w, r, "openai_models"); !ok {
		return
	}

	ids := []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini"}
	out := make([]map[string]any, 0, len(ids))
	now := time.Now().Unix()
	for _, id := range ids {
		out = append(out, map[string]any{
			"id":       id,
			"object":   "model",
			"created":  now,
			"owned_by": "choir-gateway",
		})
	}
	writeGatewayJSON(w, http.StatusOK, map[string]any{"object": "list", "data": out})
}

func (h *Handler) authorizeOpenAICompatibleRequest(w http.ResponseWriter, r *http.Request, scope string) (string, bool) {
	sandboxID, err := h.authenticateSandbox(r)
	if err != nil {
		log.Printf("gateway: openai-compatible authentication denied: %v", err)
		writeGatewayJSON(w, http.StatusUnauthorized, openAIErrorResponse("authentication required"))
		return "", false
	}
	if h.rateLimiter != nil {
		if !h.rateLimiter.Record(rateLimitBucketKey(sandboxID, scope)) {
			_, _, resetIn := h.rateLimiter.Status(rateLimitBucketKey(sandboxID, scope))
			retrySeconds := int(resetIn.Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySeconds))
			writeGatewayJSON(w, http.StatusTooManyRequests, openAIErrorResponse("rate limit exceeded"))
			return "", false
		}
	}
	return sandboxID, true
}

func convertOpenAIChatRequest(req openAIChatCompletionRequest) (ProviderRequest, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return ProviderRequest{}, fmt.Errorf("model is required")
	}
	maxTokens := req.MaxCompletionTokens
	if maxTokens == 0 {
		maxTokens = req.MaxTokens
	}

	gwReq := ProviderRequest{
		Model:           model,
		MaxTokens:       maxTokens,
		Stream:          req.Stream,
		ReasoningEffort: strings.TrimSpace(req.ReasoningEffort),
		Tools:           convertOpenAIChatTools(req.Tools),
		ToolChoice:      convertOpenAIChatToolChoice(req.ToolChoice),
	}
	if gwReq.ToolChoice == "" && len(gwReq.Tools) > 0 {
		gwReq.ToolChoice = "auto"
	}

	var system []string
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system", "developer":
			if text := textFromOpenAIContent(msg.Content); strings.TrimSpace(text) != "" {
				system = append(system, text)
			}
		case "user":
			gwReq.Messages = append(gwReq.Messages, provider.Message{
				Role:    "user",
				Content: []provider.Block{{Type: "text", Text: textFromOpenAIContent(msg.Content)}},
			})
		case "assistant":
			blocks := []provider.Block{}
			if text := textFromOpenAIContent(msg.Content); strings.TrimSpace(text) != "" {
				blocks = append(blocks, provider.Block{Type: "text", Text: text})
			}
			for _, call := range msg.ToolCalls {
				args := json.RawMessage(strings.TrimSpace(call.Function.Arguments))
				if len(args) == 0 || !json.Valid(args) {
					args = json.RawMessage("{}")
				}
				blocks = append(blocks, provider.Block{
					Type:  "tool_use",
					ID:    call.ID,
					Name:  call.Function.Name,
					Input: args,
				})
			}
			if len(blocks) > 0 {
				gwReq.Messages = append(gwReq.Messages, provider.Message{
					Role:             "assistant",
					Content:          blocks,
					ReasoningContent: msg.ReasoningContent,
				})
			}
		case "tool":
			gwReq.Messages = append(gwReq.Messages, provider.Message{
				Role: "tool",
				Content: []provider.Block{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Text:      textFromOpenAIContent(msg.Content),
				}},
			})
		}
	}
	gwReq.System = strings.Join(system, "\n\n")
	return gwReq, nil
}

func textFromOpenAIContent(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, part := range parts {
			if part.Type == "text" && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return ""
}

func convertOpenAIChatTools(tools []openAIChatTool) []provider.ToolDef {
	out := make([]provider.ToolDef, 0, len(tools))
	for _, tool := range tools {
		if tool.Type != "" && tool.Type != "function" {
			continue
		}
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			continue
		}
		schema := map[string]any{}
		if len(tool.Function.Parameters) > 0 {
			_ = json.Unmarshal(tool.Function.Parameters, &schema)
		}
		out = append(out, provider.ToolDef{
			Name:        name,
			Description: tool.Function.Description,
			InputSchema: schema,
		})
	}
	return out
}

func convertOpenAIChatToolChoice(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Type == "function" && obj.Function.Name != "" {
		return "function:" + obj.Function.Name
	}
	return ""
}

func openAIChatCompletionFromProvider(resp *provider.LLMResponse) openAIChatCompletionResponse {
	model := resp.Model
	if model == "" {
		model = "gpt-5.5"
	}
	msg := &openAIChatMessage{
		Role:    "assistant",
		Content: json.RawMessage(strconvQuote(resp.Text)),
	}
	if len(resp.ToolCalls) > 0 {
		msg.ToolCalls = providerToolCallsToOpenAI(resp.ToolCalls)
	}
	return openAIChatCompletionResponse{
		ID:      responseID(resp.ID),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []openAIChatChoice{{
			Index:        0,
			Message:      msg,
			FinishReason: openAIFinishReason(resp.StopReason, len(resp.ToolCalls) > 0),
		}},
		Usage: openAIUsage(resp.Usage),
	}
}

func providerToolCallsToOpenAI(calls []provider.ContentToolCall) []openAIChatToolCall {
	out := make([]openAIChatToolCall, 0, len(calls))
	for i, call := range calls {
		out = append(out, openAIChatToolCall{
			Index: i,
			ID:    call.ID,
			Type:  "function",
			Function: openAIToolCallFunc{
				Name:      call.Name,
				Arguments: string(call.Arguments),
			},
		})
	}
	return out
}

func (h *Handler) handleOpenAIStreamingChatCompletions(w http.ResponseWriter, r *http.Request, p provider.Provider, llmReq provider.LLMRequest, sandboxID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, canFlush := w.(http.Flusher)

	writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{Role: "assistant"}, "", nil))

	ctx, cancel := context.WithTimeout(r.Context(), inferenceTimeout)
	defer cancel()

	toolIndexes := map[string]int{}
	nextToolIndex := 0
	resp, err := p.Stream(ctx, llmReq, func(chunk provider.StreamChunk) {
		switch {
		case chunk.Delta != "":
			writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{Content: chunk.Delta}, "", nil))
		case chunk.ReasoningDelta != "":
			writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{ReasoningContent: chunk.ReasoningDelta}, "", nil))
		case chunk.ToolCallID != "" && chunk.ToolCallName != "":
			idx, ok := toolIndexes[chunk.ToolCallID]
			if !ok {
				idx = nextToolIndex
				nextToolIndex++
				toolIndexes[chunk.ToolCallID] = idx
			}
			writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{ToolCalls: []openAIChatToolCall{{
				Index: idx,
				ID:    chunk.ToolCallID,
				Type:  "function",
				Function: openAIToolCallFunc{
					Name: chunk.ToolCallName,
				},
			}}}, "", nil))
		case chunk.ToolCallDelta != "":
			idx := 0
			if chunk.ToolCallID != "" {
				if known, ok := toolIndexes[chunk.ToolCallID]; ok {
					idx = known
				}
			}
			writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{ToolCalls: []openAIChatToolCall{{
				Index: idx,
				Function: openAIToolCallFunc{
					Arguments: chunk.ToolCallDelta,
				},
			}}}, "", nil))
		case chunk.Usage != nil:
			usage := openAIUsage(provider.Usage{InputTokens: chunk.Usage.InputTokens, OutputTokens: chunk.Usage.OutputTokens})
			writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{}, "", &usage))
		}
	})
	if err != nil {
		sanitized := sanitizeError(err)
		log.Printf("gateway: openai-compatible streaming provider call failed for sandbox %s: %v (sanitized: %s)", sandboxID, err, sanitized)
		writeOpenAISSE(w, canFlush, flusher, openAIErrorResponse(sanitized))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if canFlush {
			flusher.Flush()
		}
		return
	}

	finish := "stop"
	if resp != nil {
		finish = openAIFinishReason(resp.StopReason, len(resp.ToolCalls) > 0)
		if resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0 {
			usage := openAIUsage(resp.Usage)
			writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(resp.Model, openAIChatDelta{}, "", &usage))
		}
	}
	writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{}, finish, nil))
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}

func (h *Handler) handleOpenAIStreamingToolChatCompletions(w http.ResponseWriter, r *http.Request, p provider.Provider, llmReq provider.LLMRequest, sandboxID string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher, canFlush := w.(http.Flusher)

	writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(llmReq.Model, openAIChatDelta{Role: "assistant"}, "", nil))

	ctx, cancel := context.WithTimeout(r.Context(), inferenceTimeout)
	defer cancel()
	resp, err := p.Call(ctx, llmReq)
	if err != nil {
		sanitized := sanitizeError(err)
		log.Printf("gateway: openai-compatible streaming tool provider call failed for sandbox %s: %v (sanitized: %s)", sandboxID, err, sanitized)
		writeOpenAISSE(w, canFlush, flusher, openAIErrorResponse(sanitized))
		fmt.Fprintf(w, "data: [DONE]\n\n")
		if canFlush {
			flusher.Flush()
		}
		return
	}

	model := llmReq.Model
	if resp != nil && strings.TrimSpace(resp.Model) != "" {
		model = resp.Model
	}
	if resp != nil && strings.TrimSpace(resp.Text) != "" {
		writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(model, openAIChatDelta{Content: resp.Text}, "", nil))
	}
	if resp != nil && len(resp.ToolCalls) > 0 {
		writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(model, openAIChatDelta{
			ToolCalls: providerToolCallsToOpenAI(resp.ToolCalls),
		}, "", nil))
	}
	if resp != nil && (resp.Usage.InputTokens > 0 || resp.Usage.OutputTokens > 0) {
		usage := openAIUsage(resp.Usage)
		writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(model, openAIChatDelta{}, "", &usage))
	}
	finish := "stop"
	if resp != nil {
		finish = openAIFinishReason(resp.StopReason, len(resp.ToolCalls) > 0)
	}
	writeOpenAISSE(w, canFlush, flusher, openAIChatCompletionChunk(model, openAIChatDelta{}, finish, nil))
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}

func openAIChatCompletionChunk(model string, delta openAIChatDelta, finish string, usage *openAIChatUsage) map[string]any {
	if model == "" {
		model = "gpt-5.5"
	}
	choice := openAIChatChoice{Index: 0, Delta: &delta, FinishReason: finish}
	chunk := map[string]any{
		"id":      responseID(""),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []openAIChatChoice{choice},
	}
	if usage != nil {
		chunk["usage"] = usage
	}
	return chunk
}

func writeOpenAISSE(w http.ResponseWriter, canFlush bool, flusher http.Flusher, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("gateway: marshal openai-compatible stream chunk: %v", err)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	if canFlush {
		flusher.Flush()
	}
}

func openAIUsage(usage provider.Usage) openAIChatUsage {
	return openAIChatUsage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.InputTokens + usage.OutputTokens,
	}
}

func openAIFinishReason(stopReason string, hasToolCalls bool) string {
	if hasToolCalls || strings.Contains(stopReason, "tool") {
		return "tool_calls"
	}
	switch stopReason {
	case "max_tokens", "length":
		return "length"
	default:
		return "stop"
	}
}

func openAIErrorResponse(message string) openAIErrorEnvelope {
	return openAIErrorEnvelope{Error: openAIError{Message: message, Type: "gateway_error"}}
}

func responseID(id string) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	return fmt.Sprintf("chatcmpl-choir-%d", time.Now().UnixNano())
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
