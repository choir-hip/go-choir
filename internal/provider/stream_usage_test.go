package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Recorded shapes: River documents a last chunk with empty choices that
// carries usage (https://docs.river.ai/guides/inference/).
const riverFinalChunkStream = `data: {"id":"c1","model":"deepseek-ai/DeepSeek-V4.1-Flash-1017","choices":[{"delta":{"content":"hi"}}]}

data: {"id":"c1","choices":[{"delta":{},"finish_reason":"stop"}]}

data: {"id":"c1","choices":[],"usage":{"prompt_tokens":1200,"completion_tokens":34,"total_tokens":1234,"prompt_tokens_details":{"cached_tokens":1000}}}

data: [DONE]

`

func TestStreamUsageOnFinalChunkWithEmptyChoices(t *testing.T) {
	resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(riverFinalChunkStream), "m", "river", func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.InputTokens != 1200 || resp.Usage.OutputTokens != 34 || resp.Usage.CachedInputTokens != 1000 || !resp.Usage.CacheReported {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestStreamUsageOnLastDelta(t *testing.T) {
	stream := `data: {"id":"c1","choices":[{"delta":{"content":"hi"}}]}

data: {"id":"c1","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":50,"completion_tokens":7}}

data: [DONE]

`
	resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(stream), "m", "river", func(StreamChunk) {})
	if err != nil || resp.Usage.InputTokens != 50 || resp.Usage.OutputTokens != 7 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestStreamUsageDeepSeekCacheFields(t *testing.T) {
	stream := `data: {"id":"c1","choices":[{"delta":{"content":"hi"},"finish_reason":"stop"}]}

data: {"choices":[],"usage":{"completion_tokens":5,"prompt_cache_hit_tokens":90,"prompt_cache_miss_tokens":10}}

data: [DONE]

`
	resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(stream), "m", "river", func(StreamChunk) {})
	if err != nil || resp.Usage.InputTokens != 100 || resp.Usage.CachedInputTokens != 90 || resp.Usage.OutputTokens != 5 {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
}

func TestStreamEndingWithoutUsageIsLoggedAndCounted(t *testing.T) {
	stream := `data: {"id":"c1","choices":[{"delta":{"content":"hi"},"finish_reason":"stop"}]}

data: [DONE]

`
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(io.Discard)
	ResetCacheStats()
	resp, err := parseOpenAIChatCompletionsStream(strings.NewReader(stream), "m", "river", func(StreamChunk) {})
	if err != nil || resp.Text != "hi" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if !strings.Contains(buf.String(), "stream_usage_missing") {
		t.Fatalf("no distinct log reason: %q", buf.String())
	}
	if got := CacheStatsSnapshot()["river"].NoUsageCalls; got != 1 {
		t.Fatalf("NoUsageCalls = %d", got)
	}
}

func TestStreamWithUsageIsNotCountedMissing(t *testing.T) {
	ResetCacheStats()
	if _, err := parseOpenAIChatCompletionsStream(strings.NewReader(riverFinalChunkStream), "m", "river", func(StreamChunk) {}); err != nil {
		t.Fatal(err)
	}
	if got := CacheStatsSnapshot()["river"].NoUsageCalls; got != 0 {
		t.Fatalf("NoUsageCalls = %d", got)
	}
}

type errReader struct{ r io.Reader }

func (e errReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if err == io.EOF {
		return n, context.Canceled
	}
	return n, err
}

func TestCancelledStreamReturnsErrorNotMissingUsageCount(t *testing.T) {
	ResetCacheStats()
	r := errReader{strings.NewReader(`data: {"id":"c1","choices":[{"delta":{"content":"hi"}}]}` + "\n\n")}
	_, err := parseOpenAIChatCompletionsStream(r, "m", "river", func(StreamChunk) {})
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if got := CacheStatsSnapshot()["river"].NoUsageCalls; got != 0 {
		t.Fatalf("a cancelled stream counted as missing usage: %d", got)
	}
}

func TestOpenAIChatRequestAsksForUsageOnlyWhenStreaming(t *testing.T) {
	streamed, _ := json.Marshal(openAIChatCompletionRequest{Model: "m", Stream: true})
	var got map[string]any
	_ = json.Unmarshal(streamed, &got)
	opts, _ := got["stream_options"].(map[string]any)
	if opts["include_usage"] != true {
		t.Fatalf("streamed body = %s", streamed)
	}
	plain, _ := json.Marshal(openAIChatCompletionRequest{Model: "m"})
	if strings.Contains(string(plain), "stream_options") {
		t.Fatalf("non-streamed body asks for stream options: %s", plain)
	}
}

// The River request on the wire carries stream_options.
func TestRiverStreamRequestAsksForUsage(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, riverFinalChunkStream)
	}))
	defer srv.Close()
	p, err := NewRiverProvider(RiverConfig{APIKey: "k", ModelID: riverTestModel, BaseURL: srv.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Stream(context.Background(), LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "x"}}}}}, func(StreamChunk) {})
	if err != nil {
		t.Fatal(err)
	}
	opts, _ := body["stream_options"].(map[string]any)
	if opts["include_usage"] != true {
		t.Fatalf("request body lacks include_usage: %v", body)
	}
	if resp.Usage.InputTokens != 1200 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}
