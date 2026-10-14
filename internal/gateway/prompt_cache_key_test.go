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

// The guest's cache key reaches the provider adapter unchanged, on the
// native inference endpoint and (as prompt_cache_key) on the
// OpenAI-compatible one (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
//
// Failure modes: the gateway drops the field when it rebuilds the request;
// it confuses it with the conversation id; an old guest without the field
// is refused (decoding must stay non-strict).
func TestHandleInferencePassesCacheKeyToProvider(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantKey, wantConversation string
	}{
		{name: "with key", body: `{"conversation_id":"run-1","cache_key":"choir-texture-abc-def","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`, wantKey: "choir-texture-abc-def", wantConversation: "run-1"},
		{name: "old guest", body: `{"conversation_id":"run-1","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`, wantConversation: "run-1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, reg, mp := setupHandler(t)
			credential, _ := reg.IssueCredential("autoputer-cache")
			req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+credential.RawToken)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.HandleInference(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
			if mp.lastReq == nil {
				t.Fatal("provider was not called")
			}
			if mp.lastReq.CacheKey != tc.wantKey || mp.lastReq.ConversationID != tc.wantConversation {
				t.Fatalf("provider request cache_key %q conversation %q, want %q %q", mp.lastReq.CacheKey, mp.lastReq.ConversationID, tc.wantKey, tc.wantConversation)
			}
		})
	}
}

func TestHandleOpenAIChatCompletionsPassesPromptCacheKey(t *testing.T) {
	reg := NewIdentityRegistry(1 * time.Hour)
	chatProvider := &mockProvider{name: "chatgpt", real: true, response: &provider.LLMResponse{ID: "x", Text: "ok", Model: "gpt-6-luna", StopReason: "end_turn", ProviderName: "chatgpt"}}
	mp := provider.NewMultiProvider()
	mp.Register("chatgpt", chatProvider)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	credential, _ := reg.IssueCredential("autoputer-openai-cache")
	body, _ := json.Marshal(map[string]any{
		"model":            "gpt-6-luna",
		"messages":         []map[string]string{{"role": "user", "content": "hi"}},
		"prompt_cache_key": "zot-thread-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/provider/openai/v1/chat/completions", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+credential.RawToken)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleOpenAIChatCompletions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if chatProvider.lastReq == nil || chatProvider.lastReq.CacheKey != "zot-thread-1" {
		t.Fatalf("provider request = %+v, want cache key zot-thread-1", chatProvider.lastReq)
	}
}
