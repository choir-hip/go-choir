package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Prompt-cache measurement: the gateway must relay the upstream provider's
// cached-input count to the runtime (native inference) and to OpenAI-shape
// clients (prompt_tokens_details.cached_tokens). Failure mode: the gateway
// rebuilds usage from input/output only and drops the cache report.
func TestGatewayRelaysCachedInputTokens(t *testing.T) {
	usage := provider.Usage{InputTokens: 2000, OutputTokens: 9, CachedInputTokens: 1792, CacheReported: true}

	h, reg, mp := setupHandler(t)
	mp.response.Usage = usage
	cred, _ := reg.IssueCredential("vm-guest-peer")
	body, _ := json.Marshal(ProviderRequest{
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "Hello"}}}},
	})
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.200.209.2:45678"
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("native status = %d: %s", w.Code, w.Body.String())
	}
	var native ProviderResponse
	if err := json.Unmarshal(w.Body.Bytes(), &native); err != nil {
		t.Fatal(err)
	}
	if native.Usage != usage {
		t.Fatalf("native usage = %+v, want %+v", native.Usage, usage)
	}

	reg2 := NewIdentityRegistry(time.Hour)
	chat := &mockProvider{name: "chatgpt", real: true, response: &provider.LLMResponse{
		ID: "c1", Text: "hi", Model: "gpt-6-luna", StopReason: "end_turn", ProviderName: "chatgpt", Usage: usage,
	}}
	multi := provider.NewMultiProvider()
	multi.Register("chatgpt", chat)
	compat := NewMultiHandlerWithRateLimit(reg2, multi, nil)
	cred2, _ := reg2.IssueCredential("autoputer-openai")
	req = httptest.NewRequest(http.MethodPost, "/provider/openai/v1/chat/completions",
		strings.NewReader(`{"model":"gpt-6-luna","messages":[{"role":"user","content":"hi"}],"stream":false}`))
	req.Header.Set("Authorization", "Bearer "+cred2.RawToken)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	compat.HandleOpenAIChatCompletions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compat status = %d: %s", w.Code, w.Body.String())
	}
	var resp openAIChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Usage.PromptTokens != 2000 || resp.Usage.PromptTokensDetails.CachedTokens != 1792 {
		t.Fatalf("compat usage = %+v", resp.Usage)
	}
}

// F6 (docs/archive/reports/autopaper-compute-halving-2026-10-15.md): the cached share
// per provider must be readable on staging from GET /health, so a deploy's
// effect on the chatgpt hit rate is a counter delta, not a guest cost pull.
func TestGatewayHealthReportsPromptCacheHitRatePerProvider(t *testing.T) {
	provider.ResetCacheStats()
	provider.RecordCacheUsage("chatgpt", provider.Usage{InputTokens: 1000, CachedInputTokens: 900, CacheReported: true})
	provider.RecordCacheUsage("chatgpt", provider.Usage{InputTokens: 1000, CachedInputTokens: 100, CacheReported: true})
	h, _, _ := setupHandler(t)
	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var got struct {
		PromptCache map[string]struct {
			Calls             int64   `json:"calls"`
			InputTokens       int64   `json:"input_tokens"`
			CachedInputTokens int64   `json:"cached_input_tokens"`
			HitRate           float64 `json:"hit_rate"`
		} `json:"prompt_cache"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	c := got.PromptCache["chatgpt"]
	if c.Calls != 2 || c.InputTokens != 2000 || c.CachedInputTokens != 1000 || c.HitRate != 0.5 {
		t.Fatalf("prompt_cache.chatgpt = %+v (body %s)", c, rec.Body.String())
	}
}
