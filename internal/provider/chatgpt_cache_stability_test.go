package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// chatgpt prompt-cache stability and routing
// (docs/archive/reports/autopaper-compute-halving-2026-10-15.md §5).
//
// Measured 2026-10-15 on the paper computer: gpt-6-luna reports a cached
// count that equals the desk's static prefix (system plus tools: 2,560,
// 3,584 or 5,632 tokens) on every later call of a run while the prompt
// grows from 5k to 25k tokens, at gaps of 2 to 10 seconds. The bytes we send
// are an append-only extension of the previous call's, so the lookup is not
// finding the previous call's cache: it lands on another machine.
//
// Failure modes written first:
//   - F1. A later call's instructions, tools or earlier input items differ
//     from the previous call's bytes (a per-call value, a re-serialised
//     tool output, map order), so the cached prefix ends early. Guard:
//     two consecutive requests for one desk share every byte up to the
//     newly appended turns.
//   - F2. The key changes between calls of one thread. Guard: same key in
//     both bodies.
//   - F3. The routing headers the reference client (Codex CLI) sends with
//     the key are missing, so the ChatGPT backend does not pin the thread
//     to one cache machine. Guard: session_id and conversation_id equal the
//     key on the wire.
//   - F4. The headers go out with no key (an empty header value, or the
//     run id leaking where no thread exists). Guard: absent without a key.
//   - F5. The headers cannot be turned off if the backend rejects them.
//     Guard: CHOIR_CHATGPT_SESSION_HEADERS=0 removes them.
//   - F6. The cached share cannot be read on staging without the guest's
//     cost API. Guard: the provider log line and the gateway health carry
//     cached input tokens per provider.

func chatgptTestConversation(turns int) []Message {
	msgs := []Message{{Role: "user", Content: []Block{{Type: "text", Text: "Run context:\nCurrent UTC date/time: 2026-10-15T05:11:38Z\nassignment A"}}}}
	for i := 0; i < turns; i++ {
		id := fmt.Sprintf("call_%d", i)
		msgs = append(msgs,
			Message{Role: "assistant", Content: []Block{
				{Type: "text", Text: fmt.Sprintf("step %d", i)},
				{Type: "tool_use", ID: id, Name: "cell", Input: json.RawMessage(fmt.Sprintf(`{"source":"x%d","b":1,"a":{"z":2,"y":3}}`, i))},
			}},
			Message{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: id, Text: fmt.Sprintf("output %d", i)}}},
		)
	}
	return msgs
}

func TestChatGPTConsecutiveRequestsShareEveryByteUpToTheNewTurn(t *testing.T) {
	p := &ChatGPTProvider{modelID: "gpt-6-luna", reasoning: "low"}
	tools := []ToolDef{
		{Name: "cell", Description: "run a cell", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"source": map[string]any{"type": "string"}, "a": map[string]any{"type": "string"}}}},
		{Name: "wait", Description: "wait", InputSchema: map[string]any{"type": "object"}},
	}
	build := func(turns int) openAIRequest {
		return p.buildRequestBody(LLMRequest{
			System: "stable system prompt", Tools: tools, CacheKey: "choir-research-aaaa-bbbb", ConversationID: fmt.Sprintf("run-%d", turns),
			Messages: chatgptTestConversation(turns),
		}, "gpt-6-luna")
	}
	prev := build(3)
	for turns := 4; turns <= 8; turns++ {
		next := build(turns)
		if prev.Instructions != next.Instructions {
			t.Fatalf("turn %d: instructions changed between calls", turns)
		}
		if prev.PromptCacheKey != next.PromptCacheKey || next.PromptCacheKey != "choir-research-aaaa-bbbb" {
			t.Fatalf("turn %d: cache key %q then %q", turns, prev.PromptCacheKey, next.PromptCacheKey)
		}
		pt, _ := json.Marshal(prev.Tools)
		nt, _ := json.Marshal(next.Tools)
		if string(pt) != string(nt) {
			t.Fatalf("turn %d: tools changed between calls", turns)
		}
		if len(next.Input) <= len(prev.Input) {
			t.Fatalf("turn %d: input did not grow", turns)
		}
		for i := range prev.Input {
			a, _ := json.Marshal(prev.Input[i])
			b, _ := json.Marshal(next.Input[i])
			if string(a) != string(b) {
				t.Fatalf("turn %d: input item %d changed between calls:\n%s\n%s", turns, i, a, b)
			}
		}
		prev = next
	}
}

func chatgptHeaderServer(t *testing.T, got *http.Header) (*ChatGPTProvider, func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"model\":\"gpt-6-luna\",\"usage\":{\"input_tokens\":4000,\"output_tokens\":5,\"input_tokens_details\":{\"cached_tokens\":3584}}}}\n\n")
	}))
	authPath := writeTestCodexAuth(t, testJWTWithExpiry(t, time.Now().Add(time.Hour)), "refresh", time.Now())
	p, err := NewChatGPTProvider(ChatGPTConfig{ModelID: "gpt-6-luna", BaseURL: server.URL + "/responses", AuthPath: authPath})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	p.httpClient = server.Client()
	return p, server.Close
}

func TestChatGPTRequestCarriesSessionRoutingHeaders(t *testing.T) {
	msgs := []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}
	for _, tc := range []struct {
		name string
		env  string
		req  LLMRequest
		want string
	}{
		{name: "desk key on by default", req: LLMRequest{CacheKey: "choir-texture-a-b", ConversationID: "run-1", Messages: msgs}, want: "choir-texture-a-b"},
		{name: "run id when no thread", req: LLMRequest{ConversationID: "run-1", Messages: msgs}, want: "run-1"},
		{name: "no key sends no header", req: LLMRequest{Messages: msgs}, want: ""},
		{name: "flag off removes them", env: "0", req: LLMRequest{CacheKey: "choir-texture-a-b", Messages: msgs}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("CHOIR_CHATGPT_SESSION_HEADERS", tc.env)
			}
			var h http.Header
			p, closeFn := chatgptHeaderServer(t, &h)
			defer closeFn()
			resp, err := p.Call(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("Call: %v", err)
			}
			for _, name := range []string{"Session_id", "Conversation_id"} {
				got := ""
				if vs := h.Values(name); len(vs) > 0 {
					got = vs[0]
				}
				if got != tc.want {
					t.Fatalf("header %s = %q, want %q (headers %v)", name, got, tc.want, h)
				}
			}
			if resp.Usage.CachedInputTokens != 3584 {
				t.Fatalf("cached tokens = %d", resp.Usage.CachedInputTokens)
			}
		})
	}
}

func TestProviderCacheStatsCountCachedInputPerProvider(t *testing.T) {
	ResetCacheStats()
	var h http.Header
	p, closeFn := chatgptHeaderServer(t, &h)
	defer closeFn()
	for i := 0; i < 2; i++ {
		if _, err := p.Call(context.Background(), LLMRequest{ConversationID: "r", Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}}); err != nil {
			t.Fatal(err)
		}
	}
	s := CacheStatsSnapshot()["chatgpt"]
	if s.Calls != 2 || s.InputTokens != 8000 || s.CachedInputTokens != 7168 || s.CacheReportedCalls != 2 {
		t.Fatalf("stats = %+v", s)
	}
	if got := s.HitRate(); got < 0.89 || got > 0.90 {
		t.Fatalf("hit rate = %v", got)
	}
}
