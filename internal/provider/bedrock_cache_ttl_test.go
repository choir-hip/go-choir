package provider

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Mixed cache lifetimes on Bedrock (docs/definitions/choir-autopaper-improvement-2026-11-09.ledger.md).
//
// Failure modes pinned here:
//   - a 5-minute breakpoint before a 1-hour one (Bedrock answers 400);
//   - a history-carrying desk loses its 1-hour tail, so its next wake misses
//     the cache it wrote;
//   - a one-shot run pays the 2x write premium on a tail nobody reads again;
//   - the system breakpoint (tools and system) drops below 1 hour on any desk;
//   - another Anthropic-wire route or model gets cache fields it never had;
//   - a cache_creation split is dropped, so writes price at the wrong rate.

func ttlRequest(cacheKey, conv string) LLMRequest {
	return LLMRequest{
		System: "system", CacheKey: cacheKey, ConversationID: conv,
		Tools: []ToolDef{{Name: "x", InputSchema: map[string]any{"type": "object"}}},
		Messages: []Message{
			{Role: "user", Content: []Block{{Type: "text", Text: "hello"}}},
			{Role: "assistant", Content: []Block{{Type: "tool_use", ID: "t1", Name: "x", Input: json.RawMessage(`{}`)}}},
			{Role: "user", Content: []Block{{Type: "tool_result", ToolUseID: "t1", Text: "out"}}},
		},
	}
}

// breakpointTTLs lists the cache_control lifetimes in cache order (tools,
// system, messages): "1h", or "5m" for a breakpoint with no ttl. Tools carry
// no breakpoint of their own; the system one covers them.
func breakpointTTLs(t *testing.T, body anthropicRequest) []string {
	t.Helper()
	var out []string
	note := func(cc map[string]string) {
		if cc == nil {
			return
		}
		if cc["type"] != "ephemeral" {
			t.Fatalf("breakpoint type = %v", cc)
		}
		if cc["ttl"] == "" {
			out = append(out, "5m")
		} else {
			out = append(out, cc["ttl"])
		}
	}
	if blocks, ok := body.System.([]anthropicSystemBlock); ok {
		for _, s := range blocks {
			note(s.CacheControl)
		}
	}
	for _, m := range body.Messages {
		for _, c := range m.Content {
			note(c.CacheControl)
		}
	}
	return out
}

func TestBedrockMixedCacheLifetimes(t *testing.T) {
	p := &BedrockProvider{anthropicV: "bedrock-2023-05-31"}
	taskKey := "choir-" + cacheLabelForm(deskmanifest.TaskRoleName()) + "-aaaaaaaaaaaa-bbbbbbbbbbbb"
	cases := []struct {
		name      string
		key, conv string
		want      []string
	}{
		// History-carrying desks: 1 hour on every breakpoint.
		{"texture", "choir-texture-aaaaaaaaaaaa-bbbbbbbbbbbb", "run-1", []string{"1h", "1h"}},
		{"management", "choir-management-aaaaaaaaaaaa-bbbbbbbbbbbb", "run-2", []string{"1h", "1h"}},
		{"engineering", "choir-engineering-aaaaaaaaaaaa-bbbbbbbbbbbb", "run-3", []string{"1h", "1h"}},
		{"research desk", "choir-research-aaaaaaaaaaaa-bbbbbbbbbbbb", "run-4", []string{"1h", "1h"}},
		{"failover model suffix", "choir-texture-aaaaaaaaaaaa-bbbbbbbbbbbb-mabcdef", "run-5", []string{"1h", "1h"}},
		// One-shot runs: 1 hour on system and tools, 5 minutes on the tail.
		{"no cache key (judge, api caller)", "", "", []string{"1h", "5m"}},
		{"research child web", "choir-web-aaaaaaaaaaaa-bbbbbbbbbbbb", "research-child:k1", []string{"1h", "5m"}},
		// A child of role texture shares the Texture desk's label; its
		// conversation id tells it apart.
		{"research child texture", "choir-texture-aaaaaaaaaaaa-bbbbbbbbbbbb", "research-child:k2", []string{"1h", "5m"}},
		{"model verify probe", "choir-x-aaaaaaaaaaaa-bbbbbbbbbbbb", "model-verify:m", []string{"1h", "5m"}},
		{"management task", taskKey, "run-6", []string{"1h", "5m"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := breakpointTTLs(t, p.buildRequestBody(ttlRequest(tc.key, tc.conv)))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("breakpoint lifetimes = %v, want %v", got, tc.want)
			}
			// Ordering invariant: never a 5-minute breakpoint before a 1-hour one.
			seen5m := false
			for _, l := range got {
				if l == "5m" {
					seen5m = true
				} else if seen5m {
					t.Fatalf("1h breakpoint after a 5m one: %v", got)
				}
			}
		})
	}
}

// A history-carrying desk's body is today's body byte for byte (d2adcb6ca):
// both breakpoints are {"type":"ephemeral","ttl":"1h"}. The one-shot body
// differs from it only in the tail's lifetime.
func TestBedrockHistoryDeskBodyUnchanged(t *testing.T) {
	p := &BedrockProvider{anthropicV: "bedrock-2023-05-31"}
	hist, _ := json.Marshal(p.buildRequestBody(ttlRequest("choir-texture-aaaaaaaaaaaa-bbbbbbbbbbbb", "run-1")))
	oneShot, _ := json.Marshal(p.buildRequestBody(ttlRequest("", "")))
	const long = `"cache_control":{"ttl":"1h","type":"ephemeral"}`
	const short = `"cache_control":{"type":"ephemeral"}`
	if got := strings.Count(string(hist), long); got != 2 || strings.Contains(string(hist), short) {
		t.Fatalf("history body is not 2x 1h: %s", hist)
	}
	tail := strings.LastIndex(string(hist), long)
	swapped := string(hist)[:tail] + short + string(hist)[tail+len(long):]
	if swapped != string(oneShot) {
		t.Fatalf("one-shot body differs by more than the tail lifetime:\n%s\n%s", swapped, oneShot)
	}
}

// Other Anthropic-wire routes get no cache fields, whatever the request.
func TestOtherRoutesGetNoMixedLifetimeFields(t *testing.T) {
	for _, req := range []LLMRequest{ttlRequest("", "research-child:k"), ttlRequest("choir-texture-a-b", "run-1")} {
		b, _ := json.Marshal((&ZAIProvider{modelID: "glm"}).buildRequestBody(req, "glm"))
		if strings.Contains(string(b), "cache_control") || strings.Contains(string(b), `"ttl"`) {
			t.Fatalf("zai body carries cache fields: %s", b)
		}
	}
}

func TestAnthropicUsageCacheWriteSplit(t *testing.T) {
	t.Parallel()
	body := `{"id":"m1","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"cache_read_input_tokens":600,"cache_creation_input_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":100,"ephemeral_1h_input_tokens":200},"output_tokens":9}}`
	resp, err := parseAnthropicResponse(jsonHTTPResponse(body), "m", "bedrock")
	if err != nil {
		t.Fatal(err)
	}
	u := resp.Usage
	if u.InputTokens != 910 || u.CachedInputTokens != 600 || u.CacheWrite5mTokens != 100 || u.CacheWrite1hTokens != 200 {
		t.Fatalf("usage = %+v, want in=910 read=600 w5m=100 w1h=200", u)
	}
	// A total with no split counts at the 1-hour rate.
	body = `{"id":"m1","model":"m","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":10,"cache_creation_input_tokens":300,"output_tokens":9}}`
	resp, err = parseAnthropicResponse(jsonHTTPResponse(body), "m", "bedrock")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.CacheWrite1hTokens != 300 || resp.Usage.CacheWrite5mTokens != 0 {
		t.Fatalf("unsplit usage = %+v", resp.Usage)
	}
}
