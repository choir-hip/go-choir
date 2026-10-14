package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Request shapes for prompt-cache routing, per provider
// (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
//
// Failure modes written first:
//   - chatgpt sends no prompt_cache_key, so the endpoint lands each call on
//     a random cache machine (the measured 4-10% luna hit rate);
//   - the key is sent to a route that does not document it (an unknown
//     field can 400 a strict upstream);
//   - OpenCode keeps the per-run session, so successive runs of a desk lose
//     their upstream affinity;
//   - a caller with no cache key loses the session OpenCode requires;
//   - Bedrock caches only tools and system, never the history;
//   - a breakpoint lands on a thinking block, which Anthropic refuses.

func TestChatGPTRequestCarriesPromptCacheKey(t *testing.T) {
	p := &ChatGPTProvider{modelID: "gpt-6-luna", reasoning: "low"}
	for _, tc := range []struct {
		name string
		req  LLMRequest
		want string
	}{
		{name: "desk thread key", req: LLMRequest{CacheKey: "choir-texture-a-b", ConversationID: "run-1"}, want: "choir-texture-a-b"},
		{name: "run conversation when no thread", req: LLMRequest{ConversationID: "run-1"}, want: "run-1"},
		{name: "nothing", req: LLMRequest{}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := p.buildRequestBody(tc.req, "gpt-6-luna")
			if body.PromptCacheKey != tc.want {
				t.Fatalf("prompt_cache_key = %q, want %q", body.PromptCacheKey, tc.want)
			}
			raw, _ := json.Marshal(body)
			if has := strings.Contains(string(raw), `"prompt_cache_key"`); has != (tc.want != "") {
				t.Fatalf("wire body %s: prompt_cache_key present = %v", raw, has)
			}
		})
	}
}

func TestOpenCodeSessionIsTheDeskCacheKeyAndBodiesCarryNoKey(t *testing.T) {
	for _, model := range []string{"deepseek-v4.1-flash", "cat-jam-free", "muse-spark-1.3-contributor-free", "qwen3.7-max"} {
		t.Run(model, func(t *testing.T) {
			var session, body string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				session = r.Header.Get("x-opencode-session")
				raw, _ := io.ReadAll(r.Body)
				body = string(raw)
				switch {
				case strings.HasSuffix(r.URL.Path, "/messages"):
					_ = json.NewEncoder(w).Encode(anthropicResponse{ID: "m", Model: model, Content: []anthropicResponseBlock{{Type: "text", Text: "ok"}}, StopReason: "end_turn"})
				case strings.HasSuffix(r.URL.Path, "/responses"):
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"r\",\"model\":%q,\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n", model)
				default:
					_ = json.NewEncoder(w).Encode(openAIChatCompletionResponse{ID: "c", Model: model, Choices: []openAIChatChoice{{Message: openAIChatMessage{Content: "ok"}, FinishReason: "stop"}}})
				}
			}))
			defer server.Close()
			p, err := NewOpenCodeProvider(OpenCodeConfig{APIKey: "k", BaseURL: server.URL, ModelID: model})
			if err != nil {
				t.Fatal(err)
			}
			p.httpClient = server.Client()
			if _, err := p.Call(context.Background(), LLMRequest{
				Model: model, ConversationID: "run-1", CacheKey: "choir-management-aaa-bbb",
				Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}},
			}); err != nil {
				t.Fatalf("Call: %v", err)
			}
			if session != "choir-management-aaa-bbb" {
				t.Fatalf("x-opencode-session = %q, want the desk cache key", session)
			}
			for _, field := range []string{"prompt_cache_key", "cache_key", "cache_control"} {
				if strings.Contains(body, field) {
					t.Fatalf("OpenCode body carries %s, which the route does not document: %s", field, body)
				}
			}
		})
	}
}

func TestBedrockMarksHistoryCacheBreakpoint(t *testing.T) {
	p := &BedrockProvider{anthropicV: "bedrock-2023-05-31"}
	req := LLMRequest{
		System: "system",
		Messages: []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: "hello"}}},
			{Role: "assistant", Content: []Block{{Type: "tool_use", ID: "t1", Name: "x", Input: json.RawMessage(`{}`)}}},
			{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "t1", Text: "out"}}},
		},
	}
	body := p.buildRequestBody(req)
	raw, _ := json.Marshal(body)
	if got := strings.Count(string(raw), `"cache_control"`); got != 2 {
		t.Fatalf("cache_control breakpoints = %d, want 2 (system, last message): %s", got, raw)
	}
	last := body.Messages[len(body.Messages)-1].Content
	if last[len(last)-1].CacheControl["type"] != "ephemeral" {
		t.Fatalf("last block has no ephemeral breakpoint: %+v", last)
	}
	for _, msg := range body.Messages[:len(body.Messages)-1] {
		for _, block := range msg.Content {
			if block.CacheControl != nil {
				t.Fatalf("earlier block carries a breakpoint: %+v", block)
			}
		}
	}

	// A breakpoint never lands on a thinking block.
	messages := []anthropicMessage{{Role: "assistant", Content: []anthropicContent{{Type: "text", Text: "a"}, {Type: "thinking", Thinking: "t"}}}}
	markHistoryCacheBreakpoint(messages)
	if messages[0].Content[1].CacheControl != nil || messages[0].Content[0].CacheControl == nil {
		t.Fatalf("breakpoint placement = %+v", messages[0].Content)
	}
}

// Z.AI caches GLM prompts on its own; its request stays as it was.
func TestZAIRequestCarriesNoCacheFields(t *testing.T) {
	p := &ZAIProvider{modelID: "glm"}
	raw, _ := json.Marshal(p.buildRequestBody(LLMRequest{System: "s", CacheKey: "k", Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}}, "glm"))
	for _, field := range []string{"cache_control", "prompt_cache_key", `"k"`} {
		if strings.Contains(string(raw), field) {
			t.Fatalf("zai body carries %s: %s", field, raw)
		}
	}
}
