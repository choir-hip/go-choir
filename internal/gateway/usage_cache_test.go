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
