package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Synthetic (docs/problems/synthetic-provider-onboarding-2026-10-22.md):
// ways the provider could fail, written before the code. reasoning_effort
// outside its enum is a hard 400 upstream; streamed reasoning arrives as
// delta.reasoning, not reasoning_content; a forced tool_choice answers
// finish_reason "stop" with tool_calls; a 429 is a concurrency limit that must
// reach the failover untouched (no key parking); parallel calls must keep the
// model's order; the quota header is the overnight usage reading.

const syntheticTestModel = "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"

func newTestSynthetic(server *httptest.Server) *SyntheticProvider {
	return &SyntheticProvider{
		apiKey:     "syn-test-key",
		modelID:    syntheticTestModel,
		baseURL:    server.URL,
		httpClient: server.Client(),
	}
}

func syntheticUserRequest() LLMRequest {
	return LLMRequest{
		Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}},
	}
}

func TestSyntheticProviderRequestShape(t *testing.T) {
	var gotPath, gotAuth, gotSession string
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotSession = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("x-opencode-session")
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"c1","model":"deepseek-ai/DeepSeek-V4.1-Flash-1017","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`)
	}))
	defer server.Close()
	p := newTestSynthetic(server)

	req := syntheticUserRequest()
	req.ReasoningEffort = "high"
	req.MaxTokens = 0
	resp, err := p.Call(context.Background(), req)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.ProviderName != "synthetic" || resp.Text != "ok" {
		t.Fatalf("response = %+v", resp)
	}
	if gotPath != "/chat/completions" || gotAuth != "Bearer syn-test-key" || gotSession != "" {
		t.Fatalf("path %q auth %q session %q", gotPath, gotAuth, gotSession)
	}
	if raw["model"] != syntheticTestModel || raw["reasoning_effort"] != "high" {
		t.Fatalf("model/effort = %v / %v", raw["model"], raw["reasoning_effort"])
	}
	if _, ok := raw["max_tokens"]; ok {
		t.Fatalf("max_tokens sent with no budget: %v", raw["max_tokens"])
	}
	if _, ok := raw["temperature"]; ok {
		t.Fatalf("temperature must not be set: %v", raw["temperature"])
	}
}

// reasoning_effort outside none/minimal/low/medium/high/xhigh/max is a 400
// upstream; the provider sends a valid value or none at all.
func TestSyntheticReasoningEffortNeverSendsAnInvalidValue(t *testing.T) {
	for in, want := range map[string]string{
		"":         "",
		"none":     "none",
		"minimal":  "none",
		"low":      "low",
		"medium":   "high",
		"high":     "high",
		" High ":   "high",
		"xhigh":    "xhigh",
		"max":      "max",
		"off":      "none",
		"disabled": "none",
		"false":    "none",
		"turbo":    "",
		"ultra":    "",
	} {
		if got := syntheticReasoningEffort(in); got != want {
			t.Fatalf("syntheticReasoningEffort(%q) = %q, want %q", in, got, want)
		}
	}
}

// Streamed reasoning arrives as delta.reasoning (non-streaming and the
// message echo use reasoning_content). A delta can carry both fields. The
// trailing choices:[] chunk carries the cache report.
func TestSyntheticProviderStreamReadsReasoningFieldAndCacheUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-synthetic-quotas", `{"planWeeklyLimit":{"percentRemaining":98.5,"nextRegenAt":"2026-10-23T05:56:13.000Z"},"subscription":{"limit":500,"requests":0}}`)
		for _, line := range []string{
			`{"id":"s1","model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}],"usage":{"prompt_tokens":1100,"total_tokens":1100,"completion_tokens":0}}`,
			`{"id":"s1","model":"m","choices":[{"index":0,"delta":{"reasoning":"We need"},"finish_reason":null}],"usage":{"prompt_tokens":1100,"total_tokens":1103,"completion_tokens":3}}`,
			`{"id":"s1","model":"m","choices":[{"index":0,"delta":{"content":"27 sheep","reasoning":" to add."},"finish_reason":null}],"usage":{"prompt_tokens":1100,"total_tokens":1111,"completion_tokens":11}}`,
			`{"id":"s1","model":"m","choices":[{"index":0,"delta":{"content":"."},"finish_reason":"stop"}],"usage":{"prompt_tokens":1100,"total_tokens":1112,"completion_tokens":12}}`,
			`{"id":"s1","model":"m","choices":[],"usage":{"prompt_tokens":1100,"total_tokens":1112,"completion_tokens":12,"prompt_tokens_details":{"cached_tokens":512,"created_cache_tokens":0}}}`,
		} {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", line)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()
	p := newTestSynthetic(server)

	var deltas []string
	resp, err := p.Stream(context.Background(), syntheticUserRequest(), func(c StreamChunk) {
		if c.ReasoningDelta != "" {
			deltas = append(deltas, c.ReasoningDelta)
		}
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if resp.Text != "27 sheep." {
		t.Fatalf("text = %q", resp.Text)
	}
	if resp.ReasoningContent != "We need to add." || strings.Join(deltas, "") != "We need to add." {
		t.Fatalf("reasoning = %q deltas = %q", resp.ReasoningContent, deltas)
	}
	if resp.Usage.InputTokens != 1100 || resp.Usage.OutputTokens != 12 || resp.Usage.CachedInputTokens != 512 || !resp.Usage.CacheReported {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

// A forced tool_choice answers finish_reason "stop" with tool_calls present;
// the loop must read that as tool use (non-streaming and streaming).
func TestSyntheticProviderStopWithToolCallsIsToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprint(w, "data: {\"id\":\"s\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"read_file\",\"arguments\":\"{\\\"path\\\":\\\"/x\\\"}\"}}]},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"c","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1","index":0,"type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/x\"}"}}]}}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`)
	}))
	defer server.Close()
	p := newTestSynthetic(server)
	req := syntheticUserRequest()
	req.Tools = []ToolDef{{Name: "read_file", InputSchema: map[string]any{"type": "object"}}}
	req.ToolChoice = "read_file"

	resp, err := p.Call(context.Background(), req)
	if err != nil || resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 {
		t.Fatalf("call: %+v %v", resp, err)
	}
	resp, err = p.Stream(context.Background(), req, func(StreamChunk) {})
	if err != nil || resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 {
		t.Fatalf("stream: %+v %v", resp, err)
	}
}

// Parallel tool calls keep the model's order (the stream buffers them by
// index; ranging over the map shuffled them).
func TestChatCompletionsStreamKeepsParallelToolCallOrder(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 6; i++ {
		fmt.Fprintf(&b, "data: {\"id\":\"s\",\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":\"call_%d\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"n\\\":%d}\"}}]}}]}\n\n", i, i, i)
	}
	b.WriteString("data: {\"id\":\"s\",\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
	for attempt := 0; attempt < 20; attempt++ {
		resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(b.String()), "m", "synthetic", func(StreamChunk) {})
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		for i, call := range resp.ToolCalls {
			if call.ID != fmt.Sprintf("call_%d", i) {
				t.Fatalf("attempt %d: tool call %d is %s, want call_%d", attempt, i, call.ID, i)
			}
		}
		if len(resp.ToolCalls) != 6 {
			t.Fatalf("tool calls = %d", len(resp.ToolCalls))
		}
	}
}

// An assistant tool turn echoes its reasoning as reasoning_content, the
// field Synthetic honours (reasoning is ignored upstream).
func TestSyntheticProviderEchoesReasoningContentOnToolTurns(t *testing.T) {
	var raw struct {
		Messages []map[string]any `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"c","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`)
	}))
	defer server.Close()
	p := newTestSynthetic(server)
	_, err := p.Call(context.Background(), LLMRequest{Messages: []Message{
		{Role: "user", Content: []Block{{Type: "text", Text: "weather?"}}},
		{Role: "assistant", ReasoningContent: "need the tool", Content: []Block{{Type: "tool_use", ID: "call_1", Name: "get_weather", Input: json.RawMessage(`{"city":"Paris"}`)}}},
		{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "call_1", Text: "17C"}}},
	}})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	var found bool
	for _, m := range raw.Messages {
		if m["role"] == "assistant" {
			found = true
			if m["reasoning_content"] != "need the tool" {
				t.Fatalf("assistant message = %v", m)
			}
			if _, bad := m["reasoning"]; bad {
				t.Fatalf("assistant message carries reasoning: %v", m)
			}
		}
	}
	if !found {
		t.Fatalf("no assistant message in %v", raw.Messages)
	}
}

// Synthetic's 429 is a concurrency limit (about 10 in flight, no
// Retry-After). It surfaces at once as a status error the gateway breaker and
// the tool loop's failover read, and the next call goes upstream again: no
// key is parked.
func TestSyntheticProvider429FailsFastAndIsNeverParked(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(w, `{"error":"Too many concurrent requests"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"c","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()
	p := newTestSynthetic(server)

	_, err := p.Call(context.Background(), syntheticUserRequest())
	if err == nil || !strings.Contains(err.Error(), "synthetic: status 429") || strings.Contains(err.Error(), "concurrent") {
		t.Fatalf("429 error = %v, want a sanitized status 429", err)
	}
	if _, err := p.Stream(context.Background(), syntheticUserRequest(), func(StreamChunk) {}); err != nil {
		t.Fatalf("call after 429: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream calls = %d, want 2 (no retry, no parking)", calls.Load())
	}
}

func TestSyntheticProviderFromEnv(t *testing.T) {
	t.Setenv("SYNTHETIC_API_KEY", "")
	if _, err := NewSyntheticProviderFromEnv(syntheticTestModel); err == nil {
		t.Fatal("no key must be an error")
	}
	t.Setenv("SYNTHETIC_API_KEY", "syn-test-key")
	t.Setenv("SYNTHETIC_BASE_URL", "")
	p, err := NewSyntheticProviderFromEnv(syntheticTestModel)
	if err != nil {
		t.Fatalf("from env: %v", err)
	}
	if p.baseURL != "https://api.synthetic.new/openai/v1" || p.Name() != "synthetic" || !p.IsReal() {
		t.Fatalf("provider = %+v", p)
	}
	t.Setenv("SYNTHETIC_BASE_URL", "http://127.0.0.1:9/openai/v1/")
	p, err = NewSyntheticProviderFromEnv(syntheticTestModel)
	if err != nil || p.baseURL != "http://127.0.0.1:9/openai/v1" {
		t.Fatalf("override: %+v %v", p, err)
	}
}

func TestResolveAllRegistersSyntheticOnlyWithAKeyAndModels(t *testing.T) {
	t.Setenv("SYNTHETIC_API_KEY", "syn-test-key")
	if mp := ResolveAll(ProviderConfig{SyntheticModels: []string{syntheticTestModel}}); mp.Get("synthetic") == nil {
		t.Fatal("synthetic not registered with a key and a model")
	}
	if mp := ResolveAll(ProviderConfig{}); mp.Get("synthetic") != nil {
		t.Fatal("synthetic registered without models")
	}
	t.Setenv("SYNTHETIC_API_KEY", "")
	if mp := ResolveAll(ProviderConfig{SyntheticModels: []string{syntheticTestModel}}); mp.Get("synthetic") != nil {
		t.Fatal("synthetic registered without a key")
	}
}

// The overnight reading: every response carries the quota the /v2/quotas
// endpoint serves; the log line names the weekly percentage.
func TestSyntheticQuotaSummary(t *testing.T) {
	h := http.Header{}
	if got := syntheticQuotaSummary(h); got != "" {
		t.Fatalf("no header = %q", got)
	}
	h.Set("x-synthetic-quotas", `{"subscription":{"limit":500,"requests":3,"renewsAt":"2026-10-23T07:59:17.038Z"},"planWeeklyLimit":{"nextRegenAt":"2026-10-23T05:56:13.000Z","percentRemaining":98.5612528,"nextRegenPercent":2}}`)
	got := syntheticQuotaSummary(h)
	for _, want := range []string{"weekly_percent_remaining=98.5613", "weekly_next_regen=2026-10-23T05:56:13.000Z", "subscription_requests=3/500"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary %q missing %q", got, want)
		}
	}
	h.Set("x-synthetic-quotas", `not json`)
	if got := syntheticQuotaSummary(h); got != "" {
		t.Fatalf("bad header = %q", got)
	}
}
