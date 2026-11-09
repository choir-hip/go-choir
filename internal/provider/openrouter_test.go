package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// OpenRouter chat provider (docs/problems/openrouter-provider-crouton-alpha-2026-11-03.md).
// Ways it could fail, written before the code: the key reaches the wrong
// header or a log; the path misses /chat/completions under /api/v1;
// reasoning_effort "none" is a 400 (reasoning is mandatory for
// nvidia/nemotron-3.5-super-vl:free) and an unknown value is a 400; tools are dropped;
// the stream's usage, which arrives on a second finish_reason chunk, is
// lost; a 402 (credits) or a quota 429 is not typed as quota; the provider
// registers without a key or without models.

const openRouterTestModel = "nvidia/nemotron-3.5-super-vl:free"

func newTestOpenRouter(t *testing.T, server *httptest.Server) *OpenRouterProvider {
	t.Helper()
	p, err := NewOpenRouterProvider(OpenRouterConfig{APIKey: "or-test-key", ModelID: openRouterTestModel, BaseURL: server.URL + "/api/v1"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpenRouterProviderRequestShape(t *testing.T) {
	var gotPath, gotAuth, gotAccept string
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotAccept = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Accept")
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"gen-1","model":"nvidia/nemotron-3.5-super-vl:free","provider":"Stealth","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning":null,"reasoning_details":[{"type":"reasoning.encrypted","data":"x"}],"tool_calls":[{"type":"function","index":0,"id":"call-1","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]}}],"usage":{"prompt_tokens":13,"completion_tokens":6,"total_tokens":19,"cost":0,"prompt_tokens_details":{"cached_tokens":0}}}`)
	}))
	defer server.Close()
	p := newTestOpenRouter(t, server)
	if p.Name() != "openrouter" || !p.IsReal() {
		t.Fatalf("name %q real %v", p.Name(), p.IsReal())
	}

	resp, err := p.Call(context.Background(), LLMRequest{
		Model:           openRouterTestModel,
		System:          "Use tools.",
		Messages:        []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Weather in Paris?"}}}},
		Tools:           []ToolDef{{Name: "get_weather", Description: "weather", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}}},
		ToolChoice:      "auto",
		ReasoningEffort: "none",
	})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if gotPath != "/api/v1/chat/completions" || gotAuth != "Bearer or-test-key" || gotAccept != "application/json" {
		t.Fatalf("path %q auth %q accept %q", gotPath, gotAuth, gotAccept)
	}
	if raw["model"] != openRouterTestModel {
		t.Fatalf("model = %v", raw["model"])
	}
	// Reasoning is mandatory upstream: "none" would be a 400.
	if raw["reasoning_effort"] != "minimal" {
		t.Fatalf("reasoning_effort = %v, want minimal", raw["reasoning_effort"])
	}
	tools, _ := raw["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", raw["tools"])
	}
	if _, ok := raw["max_tokens"]; ok {
		t.Fatalf("max_tokens sent with no budget: %v", raw["max_tokens"])
	}
	if resp.ProviderName != "openrouter" || resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Usage.InputTokens != 13 || resp.Usage.OutputTokens != 6 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

// The advertised efforts (models listing, 2026-11-03): max, xhigh, high,
// medium, low, minimal. "none" and an unknown value are 400s (probe).
func TestOpenRouterReasoningEffortNeverSendsAnInvalidValue(t *testing.T) {
	for in, want := range map[string]string{
		"":         "",
		"none":     "minimal",
		"off":      "minimal",
		"disabled": "minimal",
		"false":    "minimal",
		"minimal":  "minimal",
		"low":      "low",
		"medium":   "medium",
		" High ":   "high",
		"xhigh":    "xhigh",
		"max":      "max",
		"turbo":    "",
	} {
		if got := openRouterReasoningEffort("nvidia/nemotron-3.5-super-vl:free", in); got != want {
			t.Fatalf("openRouterReasoningEffort(%q) = %q, want %q", in, got, want)
		}
	}
}

// deepseek/deepseek-v4.1-flash-1017 on OpenRouter advertises only max,
// high, low and none (models listing 2026-11-04); anything else is a 400.
func TestOpenRouterReasoningEffortDeepSeekSendsOnlyAdvertisedValues(t *testing.T) {
	const model = "deepseek/deepseek-v4.1-flash-1017"
	for in, want := range map[string]string{
		"":        "",
		"none":    "low",
		"off":     "low",
		"minimal": "low",
		"low":     "low",
		"medium":  "high",
		"high":    "high",
		"xhigh":   "max",
		"max":     "max",
		"turbo":   "",
	} {
		if got := openRouterReasoningEffort(model, in); got != want {
			t.Fatalf("openRouterReasoningEffort(%q, %q) = %q, want %q", model, in, got, want)
		}
	}
}

// The probed stream shape (2026-11-03): reasoning_details deltas, tool-call
// deltas, then two finish_reason chunks, the second one carrying usage.
func TestOpenRouterStreamKeepsUsageFromTheSecondFinishChunk(t *testing.T) {
	var gotAccept string
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, ": OPENROUTER PROCESSING\n\n")
		for _, line := range []string{
			`{"id":"gen-1","model":"nvidia/nemotron-3.5-super-vl:free","choices":[{"index":0,"delta":{"content":"","role":"assistant","reasoning":null,"reasoning_details":[{"type":"reasoning.encrypted","data":"x"}]},"finish_reason":null}]}`,
			`{"id":"gen-1","model":"nvidia/nemotron-3.5-super-vl:free","choices":[{"index":0,"delta":{"content":null,"role":"assistant","tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
			`{"id":"gen-1","model":"nvidia/nemotron-3.5-super-vl:free","choices":[{"index":0,"delta":{"content":null,"role":"assistant","tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Paris\"}"}}]},"finish_reason":null}]}`,
			`{"id":"gen-1","model":"nvidia/nemotron-3.5-super-vl:free","choices":[{"index":0,"delta":{"content":"","role":"assistant","reasoning":null},"finish_reason":"tool_calls","native_finish_reason":"completed"}]}`,
			`{"id":"gen-1","model":"nvidia/nemotron-3.5-super-vl:free","choices":[{"index":0,"delta":{"content":"","role":"assistant"},"finish_reason":"tool_calls","native_finish_reason":"completed"}],"usage":{"prompt_tokens":13,"completion_tokens":6,"total_tokens":19,"cost":0,"prompt_tokens_details":{"cached_tokens":4}}}`,
		} {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	p := newTestOpenRouter(t, server)

	var deltaUsage *StreamUsage
	var deltas int
	resp, err := p.Stream(context.Background(), LLMRequest{
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Weather in Paris?"}}}},
		Tools:    []ToolDef{{Name: "get_weather", InputSchema: map[string]any{"type": "object"}}},
	}, func(c StreamChunk) {
		if c.Type == "message_delta" {
			deltas++
			deltaUsage = c.Usage
		}
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if gotAccept != "text/event-stream" || raw["stream"] != true {
		t.Fatalf("accept %q stream %v", gotAccept, raw["stream"])
	}
	if _, ok := raw["reasoning_effort"]; ok {
		t.Fatalf("reasoning_effort sent with no effort: %v", raw["reasoning_effort"])
	}
	if resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 || string(resp.ToolCalls[0].Arguments) != `{"city":"Paris"}` {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Usage.InputTokens != 13 || resp.Usage.OutputTokens != 6 || resp.Usage.CachedInputTokens != 4 {
		t.Fatalf("result usage = %+v", resp.Usage)
	}
	if deltas != 1 || deltaUsage == nil || deltaUsage.InputTokens != 13 || deltaUsage.OutputTokens != 6 {
		t.Fatalf("message_delta count %d usage %+v", deltas, deltaUsage)
	}
}

func openRouterStatusServer(t *testing.T, status int, body string) *OpenRouterProvider {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return newTestOpenRouter(t, server)
}

func TestOpenRouterStatusClassification(t *testing.T) {
	req := LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}}
	for _, tc := range []struct {
		name   string
		status int
		body   string
		quota  bool
		want   string
	}{
		{"credits", http.StatusPaymentRequired, `{"error":{"message":"Insufficient credits","code":402}}`, true, "status 402"},
		{"quota 429", http.StatusTooManyRequests, `{"error":{"message":"Free model usage limit has been reached for today","code":429}}`, true, "status 429"},
		{"daily cap 429", http.StatusTooManyRequests, `{"error":{"message":"Rate limit exceeded: free-models-per-day. Add 10 credits to unlock 1000 free model requests per day","code":429}}`, true, "status 429"},
		{"per-minute 429", http.StatusTooManyRequests, `{"error":{"message":"Rate limit exceeded: free-models-per-min. Add 10 credits to unlock 1000 free model requests per day","code":429}}`, false, "status 429"},
		{"rate 429", http.StatusTooManyRequests, `{"error":{"message":"Rate limit exceeded: too many requests","code":429}}`, false, "status 429"},
		{"bad request", http.StatusBadRequest, `{"error":{"message":"Reasoning is mandatory","code":400}}`, false, "status 400"},
	} {
		for _, stream := range []bool{false, true} {
			p := openRouterStatusServer(t, tc.status, tc.body)
			r := req
			r.Stream = stream
			_, err := p.call(context.Background(), r, func(StreamChunk) {})
			if err == nil {
				t.Fatalf("%s stream=%v: no error", tc.name, stream)
			}
			if tc.status == http.StatusTooManyRequests && !tc.quota && !errors.Is(err, ErrUpstreamConcurrencyLimit) {
				t.Fatalf("%s stream=%v: a short 429 is not typed a concurrency limit: %v", tc.name, stream, err)
			}
			if IsQuotaExhausted(err) != tc.quota || !strings.Contains(err.Error(), tc.want) || !strings.HasPrefix(err.Error(), "openrouter:") {
				t.Fatalf("%s stream=%v: err %v quota %v", tc.name, stream, err, IsQuotaExhausted(err))
			}
			if strings.Contains(err.Error(), "or-test-key") || strings.Contains(err.Error(), "Insufficient") {
				t.Fatalf("%s: unsanitized error %v", tc.name, err)
			}
		}
	}
}

func TestResolveAllRegistersOpenRouterOnlyWithKeyAndModels(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_BASE_URL", "")
	if mp := ResolveAll(ProviderConfig{OpenRouterModels: []string{openRouterTestModel}}); mp.Get("openrouter") != nil {
		t.Fatal("openrouter registered without a key")
	}
	t.Setenv("OPENROUTER_API_KEY", "or-test-key")
	if mp := ResolveAll(ProviderConfig{}); mp.Get("openrouter") != nil {
		t.Fatal("openrouter registered without models")
	}
	mp := ResolveAll(ProviderConfig{OpenRouterModels: []string{openRouterTestModel}})
	p, ok := mp.Get("openrouter").(*OpenRouterProvider)
	if !ok || p.modelID != openRouterTestModel || p.baseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("openrouter provider = %+v", mp.Get("openrouter"))
	}
}

// A trailing usage report that carries only cache fields keeps the earlier
// token counts (review r1, Devin): it must not zero the call's tokens.
func TestStreamSparseTrailingUsageKeepsTokenCounts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, line := range []string{
			`{"id":"s1","model":"m","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":5,"total_tokens":105}}`,
			`{"id":"s1","model":"m","choices":[],"usage":{"prompt_cache_hit_tokens":64,"prompt_cache_miss_tokens":36}}`,
		} {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	p := newTestOpenRouter(t, server)
	var delta *StreamUsage
	resp, err := p.Stream(context.Background(), LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}}, func(c StreamChunk) {
		if c.Type == "message_delta" {
			delta = c.Usage
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 100 || resp.Usage.OutputTokens != 5 || resp.Usage.CachedInputTokens != 64 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if delta == nil || delta.InputTokens != 100 || delta.OutputTokens != 5 {
		t.Fatalf("message_delta usage = %+v", delta)
	}
}

// A repeated finish chunk that echoes its delta (review r1, DeepSeek) must
// not append the text or the tool arguments a second time.
func TestStreamRepeatedFinishChunkDoesNotDuplicateDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := `{"id":"g","model":"m","choices":[{"index":0,"delta":{"content":"hi","tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"f","arguments":"{\"a\":1}"}}]},"finish_reason":"tool_calls"}]}`
		_, _ = fmt.Fprintf(w, "data: %s\n\ndata: %s\n\n", chunk, chunk)
		_, _ = fmt.Fprint(w, `data: {"id":"g","model":"m","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`+"\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	p := newTestOpenRouter(t, server)
	resp, err := p.Stream(context.Background(), LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "x"}}}}}, func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hi" || len(resp.ToolCalls) != 1 || string(resp.ToolCalls[0].Arguments) != `{"a":1}` || resp.Usage.InputTokens != 3 {
		t.Fatalf("resp = %+v", resp)
	}
}

func TestOpenRouterSendsSessionIDForStickyRouting(t *testing.T) {
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"g","model":"m","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":10,"completion_tokens":1,"prompt_tokens_details":{"cached_tokens":8}}}`)
	}))
	defer server.Close()
	p := newTestOpenRouter(t, server)
	resp, err := p.Call(context.Background(), LLMRequest{CacheKey: "desk-1", ConversationID: "run-9",
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if raw["session_id"] != "desk-1" {
		t.Fatalf("session_id %v, want desk-1", raw["session_id"])
	}
	if resp.Usage.CachedInputTokens != 8 {
		t.Fatalf("cached %d, want 8", resp.Usage.CachedInputTokens)
	}
}

// Every OpenRouter request carries Choir's own app attribution and nothing
// naming another app; inkling, whose listing lacks tool_choice, is sent the
// tools without one while a model that supports it keeps it
// (docs/problems/crouton-retired-inkling-gated-2026-11-09.md).
func TestOpenRouterAttributionAndInklingToolChoice(t *testing.T) {
	var hdr http.Header
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Clone()
		raw = nil
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"gen-1","model":"m","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hi"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	defer server.Close()
	p := newTestOpenRouter(t, server)
	tools := []ToolDef{{Name: "t", Description: "d", InputSchema: map[string]any{"type": "object"}}}
	for model, wantChoice := range map[string]bool{
		"thinkingmachines/inkling-2-small:free": false,
		"nvidia/nemotron-3.5-super-vl:free":     true,
	} {
		_, err := p.Call(context.Background(), LLMRequest{Model: model, Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "x"}}}}, Tools: tools, ToolChoice: "required"})
		if err != nil {
			t.Fatalf("%s: %v", model, err)
		}
		if _, has := raw["tool_choice"]; has != wantChoice {
			t.Fatalf("%s: tool_choice present = %v, want %v", model, has, wantChoice)
		}
		if raw["tools"] == nil {
			t.Fatalf("%s: tools dropped", model)
		}
		for k, want := range map[string]string{"Http-Referer": "https://choir.news", "X-Openrouter-Title": "Choir", "X-Title": "Choir", "X-Openrouter-Categories": "cli-agent"} {
			if got := hdr.Get(k); got != want {
				t.Fatalf("%s: header %s = %q, want %q", model, k, got, want)
			}
		}
	}
}
