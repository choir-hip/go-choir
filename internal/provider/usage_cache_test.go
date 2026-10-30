package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// Prompt-cache measurement (docs/archive/reports/prompt-cache-analysis-2026-10-14.md).
// Each provider reports cached input tokens in its own usage shape; the parser
// must carry them into Usage.CachedInputTokens as a subset of InputTokens.
//
// Failure modes pinned here:
//   - OpenAI-style usage.prompt_tokens_details.cached_tokens is dropped;
//   - DeepSeek-native prompt_cache_hit_tokens / prompt_cache_miss_tokens is
//     dropped, or prompt_tokens is read as 0 when only hit/miss are sent;
//   - the Responses API's usage.input_tokens_details.cached_tokens is dropped;
//   - Anthropic cache_read_input_tokens is dropped, or InputTokens excludes
//     cached reads so cached > input;
//   - streamed chat completions lose cached tokens when usage arrives in the
//     trailing chunk with no choices;
//   - a response with no cache field is reported as "0% hit" instead of "not
//     reported" (CacheReported must be false);
//   - a bogus cached count larger than the prompt inflates the hit rate past
//     100% (clamped to InputTokens);
//   - the field does not survive the gateway's JSON relay.

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}
}

func TestChatCompletionsUsageCachedTokenShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		usage              string
		wantIn, wantCached int
		wantOut            int
		wantReported       bool
	}{
		{
			name:   "openai prompt_tokens_details",
			usage:  `{"prompt_tokens":1200,"completion_tokens":40,"total_tokens":1240,"prompt_tokens_details":{"cached_tokens":1024}}`,
			wantIn: 1200, wantCached: 1024, wantOut: 40, wantReported: true,
		},
		{
			name:   "deepseek hit and miss",
			usage:  `{"prompt_tokens":1000,"completion_tokens":10,"prompt_cache_hit_tokens":896,"prompt_cache_miss_tokens":104}`,
			wantIn: 1000, wantCached: 896, wantOut: 10, wantReported: true,
		},
		{
			name:   "deepseek hit and miss without prompt_tokens",
			usage:  `{"completion_tokens":10,"prompt_cache_hit_tokens":600,"prompt_cache_miss_tokens":400}`,
			wantIn: 1000, wantCached: 600, wantOut: 10, wantReported: true,
		},
		{
			name:   "both shapes present",
			usage:  `{"prompt_tokens":1000,"completion_tokens":10,"prompt_cache_hit_tokens":512,"prompt_cache_miss_tokens":488,"prompt_tokens_details":{"cached_tokens":512}}`,
			wantIn: 1000, wantCached: 512, wantOut: 10, wantReported: true,
		},
		{
			name:   "explicit zero hit is reported",
			usage:  `{"prompt_tokens":800,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":0}}`,
			wantIn: 800, wantCached: 0, wantOut: 10, wantReported: true,
		},
		{
			name:   "no cache field is not reported",
			usage:  `{"prompt_tokens":800,"completion_tokens":10,"total_tokens":810}`,
			wantIn: 800, wantCached: 0, wantOut: 10, wantReported: false,
		},
		{
			name:   "cached larger than prompt is clamped",
			usage:  `{"prompt_tokens":100,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":5000}}`,
			wantIn: 100, wantCached: 100, wantOut: 10, wantReported: true,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := `{"id":"c1","model":"deepseek-v4.1-flash","choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}],"usage":` + tc.usage + `}`
			resp, err := parseOpenAIChatCompletionsResponse(jsonHTTPResponse(body), "deepseek-v4.1-flash", "opencode-go")
			if err != nil {
				t.Fatal(err)
			}
			got := resp.Usage
			if got.InputTokens != tc.wantIn || got.CachedInputTokens != tc.wantCached || got.OutputTokens != tc.wantOut || got.CacheReported != tc.wantReported {
				t.Fatalf("usage = %+v, want in=%d cached=%d out=%d reported=%v", got, tc.wantIn, tc.wantCached, tc.wantOut, tc.wantReported)
			}
		})
	}
}

func TestChatCompletionsStreamCachedTokensInTrailingUsageChunk(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`data: {"id":"c1","model":"deepseek-v4.1-flash","choices":[{"delta":{"content":"hi"}}]}`,
		`data: {"id":"c1","model":"deepseek-v4.1-flash","choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`data: {"id":"c1","model":"deepseek-v4.1-flash","choices":[],"usage":{"prompt_tokens":2000,"completion_tokens":7,"prompt_cache_hit_tokens":1920,"prompt_cache_miss_tokens":80}}`,
		`data: [DONE]`,
	}, "\n\n") + "\n\n"
	resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(stream), "deepseek-v4.1-flash", "opencode-go", func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 2000 || resp.Usage.CachedInputTokens != 1920 || resp.Usage.OutputTokens != 7 || !resp.Usage.CacheReported {
		t.Fatalf("usage = %+v, want 2000 in, 1920 cached, 7 out, reported", resp.Usage)
	}
}

func TestChatCompletionsStreamCachedTokensOnFinishChunk(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`data: {"id":"c1","model":"gpt-x","choices":[{"delta":{"content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":300,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":256}}}`,
		`data: [DONE]`,
	}, "\n\n") + "\n\n"
	var deltaUsage *StreamUsage
	resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(stream), "gpt-x", "fireworks", func(c StreamChunk) {
		if c.Type == "message_delta" && c.Usage != nil {
			deltaUsage = c.Usage
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.CachedInputTokens != 256 || resp.Usage.InputTokens != 300 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if deltaUsage == nil || deltaUsage.CachedInputTokens != 256 {
		t.Fatalf("message_delta usage = %+v, want cached 256 relayed", deltaUsage)
	}
}

func TestResponsesStreamCachedTokens(t *testing.T) {
	t.Parallel()
	events := []string{
		`{"type":"response.created","response":{"id":"r1","model":"gpt-6-luna"}}`,
		`{"type":"response.output_text.delta","delta":"ok"}`,
		`{"type":"response.completed","response":{"id":"r1","model":"gpt-6-luna","usage":{"input_tokens":5000,"input_tokens_details":{"cached_tokens":4864},"output_tokens":20,"output_tokens_details":{"reasoning_tokens":8},"total_tokens":5020}}}`,
	}
	var body strings.Builder
	for _, ev := range events {
		body.WriteString("data: " + ev + "\n\n")
	}
	resp, err := parseOpenAIStream(strings.NewReader(body.String()), "gpt-6-luna", "chatgpt", func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 5000 || resp.Usage.CachedInputTokens != 4864 || resp.Usage.OutputTokens != 20 || !resp.Usage.CacheReported {
		t.Fatalf("usage = %+v", resp.Usage)
	}

	// No input_tokens_details: not reported.
	plain := `data: {"type":"response.completed","response":{"id":"r2","usage":{"input_tokens":10,"output_tokens":5}}}` + "\n\n"
	resp, err = parseOpenAIStream(strings.NewReader(plain), "gpt-6-luna", "chatgpt", func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.CacheReported || resp.Usage.CachedInputTokens != 0 {
		t.Fatalf("usage = %+v, want not reported", resp.Usage)
	}
}

func TestAnthropicUsageCacheReadCountsAsInput(t *testing.T) {
	t.Parallel()
	body := `{"id":"m1","model":"qwen3.7-max","stop_reason":"end_turn","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":50,"cache_read_input_tokens":3000,"cache_creation_input_tokens":200,"output_tokens":9}}`
	resp, err := parseAnthropicResponse(jsonHTTPResponse(body), "qwen3.7-max", "opencode-go")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 3250 || resp.Usage.CachedInputTokens != 3000 || resp.Usage.OutputTokens != 9 || !resp.Usage.CacheReported {
		t.Fatalf("usage = %+v, want in=3250 (50+3000+200) cached=3000 out=9", resp.Usage)
	}

	stream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"m2","model":"qwen3.7-max","usage":{"input_tokens":40,"cache_read_input_tokens":960}}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
		`data: {"type":"message_stop"}`,
	}, "\n\n") + "\n\n"
	resp, err = parseSSEStream(strings.NewReader(stream), "qwen3.7-max", "opencode-go", func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 1000 || resp.Usage.CachedInputTokens != 960 || resp.Usage.OutputTokens != 3 {
		t.Fatalf("stream usage = %+v, want in=1000 cached=960 out=3", resp.Usage)
	}
}

// The gateway relays provider.Usage as JSON to the runtime; the cached count
// and the reported bit must survive, and an old peer's payload without them
// must decode to zero / not reported.
func TestUsageJSONRoundTripKeepsCachedTokens(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(Usage{InputTokens: 10, OutputTokens: 2, CachedInputTokens: 8, CacheReported: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"cached_input_tokens":8`) {
		t.Fatalf("usage json = %s", raw)
	}
	var back Usage
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.CachedInputTokens != 8 || !back.CacheReported {
		t.Fatalf("round trip = %+v", back)
	}
	var old Usage
	if err := json.Unmarshal([]byte(`{"input_tokens":10,"output_tokens":2}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.CachedInputTokens != 0 || old.CacheReported {
		t.Fatalf("old payload = %+v", old)
	}
}
