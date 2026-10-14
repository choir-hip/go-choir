package toolregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Cache key derivation (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
//
// Failure modes: the key changes between calls of one thread (per-call or
// per-time input), so the provider spreads them; it stays the same across a
// prompt change, so a new prompt version inherits a stale routing; it leaks
// the owner, computer or agent id; two desks or two computers share a key;
// it grows past what a provider accepts; failover keeps the key pinned to
// the failing route.
func TestPromptCacheKeyDerivation(t *testing.T) {
	tools := []provideriface.ToolDefinition{{Name: "desk_go_eval", Description: "cell", Parameters: map[string]any{"type": "object"}}}
	scope := "owner-1\x00vm-computer-1\x00texture:doc-1"
	key := PromptCacheKey("texture", scope, "system", tools)
	if key == "" || key != PromptCacheKey("texture", scope, "system", tools) {
		t.Fatalf("key not deterministic: %q", key)
	}
	if !strings.HasPrefix(key, "choir-texture-") || len(key) > 64 {
		t.Fatalf("key shape = %q (len %d)", key, len(key))
	}
	for _, secret := range []string{"owner-1", "vm-computer-1", "doc-1"} {
		if strings.Contains(key, secret) {
			t.Fatalf("key %q leaks %q", key, secret)
		}
	}
	keys := map[string]string{
		"base":           key,
		"other desk":     PromptCacheKey("texture", "owner-1\x00vm-computer-1\x00texture:doc-2", "system", tools),
		"other computer": PromptCacheKey("texture", "owner-1\x00vm-computer-2\x00texture:doc-1", "system", tools),
		"new system":     PromptCacheKey("texture", scope, "system v2", tools),
		"new tools":      PromptCacheKey("texture", scope, "system", append(append([]provideriface.ToolDefinition(nil), tools...), provideriface.ToolDefinition{Name: "other"})),
		"other label":    PromptCacheKey("management", scope, "system", tools),
	}
	seen := map[string]string{}
	for name, k := range keys {
		if prior, ok := seen[k]; ok {
			t.Fatalf("%s and %s share key %q", name, prior, k)
		}
		seen[k] = name
	}
	if PromptCacheKey("texture", "", "system", tools) != "" {
		t.Fatal("empty scope must send no key")
	}
	if got := PromptCacheKey("Management Task/../x", scope, "s", nil); !strings.HasPrefix(got, "choir-management_task____x-") {
		t.Fatalf("label not sanitized: %q", got)
	}
}

// One activation sends one key on every call; a failover moves to a
// suffixed key with the conversation id.
func TestRunToolLoopSendsOneCacheKeyPerThreadAndMovesItOnFailover(t *testing.T) {
	const scope = "owner\x00computer\x00management:owner"
	provider := &modelFailingProvider{fail: map[string]error{}}
	guarded := 0
	_, _, err := RunToolLoop(context.Background(), provider, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, noEmit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}),
		WithToolLoopConversationID("run-1"),
		WithToolLoopPromptCache("management", scope),
		WithCompletionGuard(func(context.Context, ToolLoopCompletionState) (ToolLoopCompletionGuardResult, error) {
			guarded++
			return ToolLoopCompletionGuardResult{Continue: guarded < 3, Instruction: "again"}, nil
		}))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(provider.requests) < 3 {
		t.Fatalf("requests = %d, want at least 3", len(provider.requests))
	}
	want := PromptCacheKey("management", scope, "system", []provideriface.ToolDefinition{})
	for i, req := range provider.requests {
		if req.CacheKey != want {
			t.Fatalf("call %d cache key = %q, want %q", i, req.CacheKey, want)
		}
	}

	failing := &modelFailingProvider{fail: map[string]error{"deepseek-v4.1-flash": fmt.Errorf("gateway: status 503 service unavailable")}}
	if _, _, err := RunToolLoop(context.Background(), failing, nil,
		[]json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, noEmit, nil,
		WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}),
		WithToolLoopFallbackLLMConfigs([]provideriface.LLMSelection{{Provider: "chatgpt", Model: "gpt-6-luna"}}),
		WithToolLoopConversationID("run-1"),
		WithToolLoopPromptCache("management", scope)); err != nil {
		t.Fatalf("failover run: %v", err)
	}
	first, last := failing.requests[0], failing.requests[len(failing.requests)-1]
	if first.CacheKey != want || last.CacheKey != want+"-failover" {
		t.Fatalf("cache keys first %q last %q, want %q then %q", first.CacheKey, last.CacheKey, want, want+"-failover")
	}
}

func noEmit(types.EventKind, string, json.RawMessage) {}
