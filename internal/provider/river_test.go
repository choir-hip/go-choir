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

// River chat provider (owner 2026-11-09: River for self-development while
// Synthetic DeepSeek is spent). Ways it could fail, written before the code:
// the key reaches the wrong header or a log; the path misses
// /chat/completions under /v1; reasoning_effort "medium" is sent with tools
// (River answers 400 invalid_request) or low/high are lost; tools are
// dropped or tool_calls are not parsed; a quota 429 is not typed as quota, or
// a plain rate limit is; the provider registers without a key or models.

const riverTestModel = "deepseek-ai/DeepSeek-V4.1-Flash-1017"

func newTestRiver(t *testing.T, server *httptest.Server) *RiverProvider {
	t.Helper()
	p, err := NewRiverProvider(RiverConfig{APIKey: "rv-test-key", ModelID: riverTestModel, BaseURL: server.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRiverProviderRequestShapeAndToolCalls(t *testing.T) {
	for _, tc := range []struct {
		effort  string
		want    string
		present bool
	}{
		{"low", "low", true},
		{"high", "high", true},
		{"xhigh", "high", true},
		{"max", "high", true},
		{"medium", "", false},
		{"", "", false},
		{"turbo", "", false},
	} {
		var gotPath, gotAuth string
		var raw map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
			_ = json.NewDecoder(r.Body).Decode(&raw)
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"id":"r1","model":"deepseek-ai/DeepSeek-V4.1-Flash-1017","choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","reasoning_content":"think","tool_calls":[{"type":"function","index":0,"id":"call-1","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}]}}],"usage":{"prompt_tokens":13,"completion_tokens":6,"total_tokens":19,"completion_tokens_details":{"reasoning_tokens":3}}}`)
		}))
		p := newTestRiver(t, server)
		if p.Name() != "river" || !p.IsReal() {
			t.Fatalf("name %q real %v", p.Name(), p.IsReal())
		}
		resp, err := p.Call(context.Background(), LLMRequest{
			Model:           riverTestModel,
			Messages:        []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Weather?"}}}},
			Tools:           []ToolDef{{Name: "get_weather", Description: "w", InputSchema: map[string]any{"type": "object"}}},
			ToolChoice:      "auto",
			ReasoningEffort: tc.effort,
		})
		server.Close()
		if err != nil {
			t.Fatalf("%q: %v", tc.effort, err)
		}
		if gotPath != "/v1/chat/completions" || gotAuth != "Bearer rv-test-key" || raw["model"] != riverTestModel {
			t.Fatalf("path %q auth %q model %v", gotPath, gotAuth, raw["model"])
		}
		got, ok := raw["reasoning_effort"]
		if ok != tc.present || (ok && got != tc.want) {
			t.Fatalf("effort %q: reasoning_effort = %v (present %v), want %q (present %v)", tc.effort, got, ok, tc.want, tc.present)
		}
		if tools, _ := raw["tools"].([]any); len(tools) != 1 {
			t.Fatalf("tools = %v", raw["tools"])
		}
		if resp.ProviderName != "river" || resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_weather" || string(resp.ToolCalls[0].Arguments) != `{"city":"Paris"}` {
			t.Fatalf("response = %+v", resp)
		}
	}
}

func TestRiverStatusClassification(t *testing.T) {
	req := LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}}
	for _, tc := range []struct {
		name   string
		status int
		body   string
		quota  bool
	}{
		{"quota 429", http.StatusTooManyRequests, `{"error":{"message":"You have exceeded your quota","type":"rate_limit"}}`, true},
		{"limit 429", http.StatusTooManyRequests, `{"error":{"message":"Monthly usage limit reached"}}`, true},
		{"credits 402", http.StatusPaymentRequired, `{"error":{"message":"payment required"}}`, true},
		{"rate 429", http.StatusTooManyRequests, `{"error":{"message":"too many requests"}}`, false},
		{"bad request", http.StatusBadRequest, `{"error":{"message":"Invalid inference request"}}`, false},
	} {
		for _, stream := range []bool{false, true} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			p := newTestRiver(t, server)
			r := req
			r.Stream = stream
			_, err := p.call(context.Background(), r, func(StreamChunk) {})
			server.Close()
			if err == nil {
				t.Fatalf("%s: no error", tc.name)
			}
			if IsQuotaExhausted(err) != tc.quota || !strings.HasPrefix(err.Error(), "river:") {
				t.Fatalf("%s stream=%v: err %v quota %v", tc.name, stream, err, IsQuotaExhausted(err))
			}
			if tc.status == http.StatusTooManyRequests && !tc.quota && !errors.Is(err, ErrUpstreamConcurrencyLimit) {
				t.Fatalf("%s: plain 429 not typed a concurrency limit: %v", tc.name, err)
			}
			if strings.Contains(err.Error(), "rv-test-key") {
				t.Fatalf("key in error: %v", err)
			}
		}
	}
}

func TestResolveAllRegistersRiverOnlyWithKeyAndModels(t *testing.T) {
	t.Setenv("RIVER_API_KEY", "")
	t.Setenv("RIVER_BASE_URL", "")
	if mp := ResolveAll(ProviderConfig{RiverModels: []string{riverTestModel}}); mp.Get("river") != nil {
		t.Fatal("river registered without a key")
	}
	t.Setenv("RIVER_API_KEY", "rv-test-key")
	if mp := ResolveAll(ProviderConfig{}); mp.Get("river") != nil {
		t.Fatal("river registered without models")
	}
	p, ok := ResolveAll(ProviderConfig{RiverModels: []string{riverTestModel}}).Get("river").(*RiverProvider)
	if !ok || p.modelID != riverTestModel || p.baseURL != "https://api.river.ai/v1" {
		t.Fatalf("river provider = %+v", p)
	}
}
