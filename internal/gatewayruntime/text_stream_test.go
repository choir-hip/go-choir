package gatewayruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// StreamText, the Autoradio desk's streamed call. Ways it could fail, written
// before the code:
//
//  1. a delta is held until the stream ends (the first sentence is then late);
//  2. reasoning text is spoken;
//  3. the request carries tools, or is not marked stream, or loses the model,
//     provider, cache key or max_tokens;
//  4. a refusal before the first chunk (the typed provider quota, any non-200)
//     is not an error before any delta, so the pool cannot walk past it;
//  5. the gateway's untyped {"error":...} line after streaming began is read
//     as a clean end (parseGatewaySSE does), and a half answer is spoken as a
//     whole one;
//  6. a body that ends without [DONE] is taken for complete;
//  7. a cancelled context is not seen by the caller as cancellation.

func sseServer(t *testing.T, got *llmRequest, write func(w http.ResponseWriter, flush func())) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			_ = json.NewDecoder(r.Body).Decode(got)
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("Accept = %q, want text/event-stream", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		write(w, func() { w.(http.Flusher).Flush() })
	}))
}

func TestStreamTextDeliversEachDeltaAsItArrivesAndSendsNoTools(t *testing.T) {
	var got llmRequest
	release := make(chan struct{})
	server := sseServer(t, &got, func(w http.ResponseWriter, flush func()) {
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"model\":\"m1\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"reasoning_delta\":\"secret thoughts\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":\"Hello. \"}\n\n")
		flush()
		<-release // the rest of the answer is not there yet (1)
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":\"Bye.\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"stop_reason\":\"end_turn\",\"usage\":{\"input_tokens\":7,\"output_tokens\":3}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer server.Close()
	p := New(server.URL, "tok")
	first := make(chan string, 4)
	var deltas []string
	done := make(chan struct{})
	var resp *provideriface.TextStreamResponse
	var err error
	go func() {
		defer close(done)
		resp, err = p.StreamText(context.Background(), provideriface.TextStreamRequest{
			Provider: "river", Model: "deepseek-ai/DeepSeek-V4.1-Flash-1017", ReasoningEffort: "low",
			ConversationID: "turn-1", CacheKey: "autoradio:c1", System: "sys", User: "hi",
		}, func(d string) { deltas = append(deltas, d); first <- d })
	}()
	select {
	case d := <-first:
		if d != "Hello. " {
			t.Fatalf("first delta %q", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("(1) the first delta was held until the stream ended")
	}
	close(release)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deltas, "") != "Hello. Bye." || resp.Text != "Hello. Bye." || strings.Contains(resp.Text, "secret") {
		t.Fatalf("(2) deltas %q text %q", deltas, resp.Text)
	}
	if resp.Model != "m1" || resp.StopReason != "end_turn" || resp.Usage.InputTokens != 7 || resp.Usage.OutputTokens != 3 {
		t.Fatalf("response %+v", resp)
	}
	if !got.Stream || len(got.Tools) != 0 || got.ToolChoice != "" || got.Provider != "river" || got.Model != "deepseek-ai/DeepSeek-V4.1-Flash-1017" ||
		got.CacheKey != "autoradio:c1" || got.ConversationID != "turn-1" || got.MaxTokens != 0 || got.ReasoningOff || got.ReasoningEffort != "low" || got.System != "sys" {
		t.Fatalf("(3) request %+v", got)
	}
}

func TestStreamTextRefusalBeforeTheFirstChunkIsAnErrorBeforeAnyDelta(t *testing.T) {
	for name, c := range map[string]struct {
		status int
		body   string
		quota  bool
	}{
		"typed quota": {http.StatusTooManyRequests, `{"error":"spent","kind":"provider_quota","provider":"devin","model":"swe-2-flash","reason":"quota"}`, true},
		"server":      {http.StatusBadGateway, `{"error":"upstream down"}`, false},
		"limiter":     {http.StatusTooManyRequests, `{"error":"rate limit exceeded"}`, false},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if c.status == http.StatusTooManyRequests && !c.quota {
				w.Header().Set("Retry-After", "30") // a voice turn must not wait this out
			}
			w.WriteHeader(c.status)
			fmt.Fprint(w, c.body)
		}))
		called := false
		start := time.Now()
		_, err := New(server.URL, "tok").StreamText(context.Background(), provideriface.TextStreamRequest{User: "x"}, func(string) { called = true })
		server.Close()
		if err == nil || called {
			t.Fatalf("(4) %s: err %v, delta called %v", name, err, called)
		}
		if time.Since(start) > 2*time.Second {
			t.Fatalf("(4) %s: waited %v", name, time.Since(start))
		}
		if _, ok := provideriface.AsProviderQuota(err); ok != c.quota {
			t.Fatalf("(4) %s: quota %v, want %v (%v)", name, ok, c.quota, err)
		}
	}
}

func TestStreamTextAnUpstreamErrorAfterStreamingBeganIsAnError(t *testing.T) {
	server := sseServer(t, nil, func(w http.ResponseWriter, flush func()) {
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":\"Half an \"}\n\n")
		fmt.Fprint(w, "data: {\"error\":\"upstream reset\"}\n\n") // the gateway's untyped shape
		flush()
	})
	defer server.Close()
	var deltas []string
	_, err := New(server.URL, "tok").StreamText(context.Background(), provideriface.TextStreamRequest{User: "x"}, func(d string) { deltas = append(deltas, d) })
	if err == nil || !strings.Contains(err.Error(), "upstream reset") {
		t.Fatalf("(5) err %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("deltas %q", deltas)
	}
}

func TestStreamTextABodyWithoutTheDoneMarkerIsNotComplete(t *testing.T) {
	server := sseServer(t, nil, func(w http.ResponseWriter, flush func()) {
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":\"cut off\"}\n\n")
	})
	defer server.Close()
	if _, err := New(server.URL, "tok").StreamText(context.Background(), provideriface.TextStreamRequest{User: "x"}, func(string) {}); err == nil {
		t.Fatal("(6) a stream with no completion marker was taken for complete")
	}
}

func TestStreamTextCancellationIsSeenAsCancellation(t *testing.T) {
	started := make(chan struct{})
	server := sseServer(t, nil, func(w http.ResponseWriter, flush func()) {
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":\"One. \"}\n\n")
		flush()
		close(started)
		time.Sleep(time.Second)
	})
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-started; cancel() }()
	_, err := New(server.URL, "tok").StreamText(ctx, provideriface.TextStreamRequest{User: "x"}, func(string) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("(7) err %v", err)
	}
}

// (8) usage reported in two chunks (the Anthropic-style message_start with
// input tokens and cache, then message_delta with output tokens and input 0)
// is merged: a non-zero count is never replaced by zero. The first test's fake
// sent one merged chunk, which hid this.
func TestStreamTextMergesUsageAcrossChunks(t *testing.T) {
	server := sseServer(t, nil, func(w http.ResponseWriter, flush func()) {
		fmt.Fprint(w, "data: {\"type\":\"message_start\",\"model\":\"m\",\"usage\":{\"input_tokens\":812,\"cached_input_tokens\":700,\"cache_reported\":true}}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"content_block_delta\",\"delta\":\"Hi.\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"stop_reason\":\"end_turn\",\"usage\":{\"output_tokens\":61}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer server.Close()
	resp, err := New(server.URL, "tok").StreamText(context.Background(), provideriface.TextStreamRequest{User: "x"}, func(string) {})
	if err != nil {
		t.Fatal(err)
	}
	u := resp.Usage
	if u.InputTokens != 812 || u.OutputTokens != 61 || u.CachedInputTokens != 700 || !u.CacheReported {
		t.Fatalf("(8) usage %+v", u)
	}
}

// (9) the reasoning-off exemption is the autoradio call's alone: StreamText
// sends it only when asked, a stable prefix is its own cache-marked block, and
// CallWithTools (every other role) can never send it.
func TestReasoningOffAndCacheBreakAreOnlySentWhenAsked(t *testing.T) {
	var got llmRequest
	server := sseServer(t, &got, func(w http.ResponseWriter, flush func()) {
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	defer server.Close()
	p := New(server.URL, "tok")
	_, _ = p.StreamText(context.Background(), provideriface.TextStreamRequest{StableUser: "page block", User: "words", ReasoningOff: true}, func(string) {})
	if !got.ReasoningOff || len(got.Messages) != 1 || len(got.Messages[0].Content) != 2 ||
		!got.Messages[0].Content[0].CacheBreak || got.Messages[0].Content[0].Text != "page block" ||
		got.Messages[0].Content[1].CacheBreak || got.Messages[0].Content[1].Text != "words" {
		t.Fatalf("(9) request %+v", got)
	}
	got = llmRequest{}
	_, _ = p.StreamText(context.Background(), provideriface.TextStreamRequest{User: "words"}, func(string) {})
	if got.ReasoningOff || len(got.Messages[0].Content) != 1 || got.Messages[0].Content[0].CacheBreak {
		t.Fatalf("(9) an unasked call carries the exemption or a cache break: %+v", got)
	}
}

func TestCallWithToolsNeverSendsTheReasoningExemption(t *testing.T) {
	var raw map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		writeJSON(t, w, llmResponse{ID: "r", Text: "ok", Model: "m", StopReason: "end_turn", ProviderName: "river"})
	}))
	defer server.Close()
	p := New(server.URL, "tok")
	for _, effort := range []string{"off", "none", "low", ""} {
		raw = nil
		_, err := p.CallWithTools(context.Background(), provideriface.ToolLoopRequest{
			System: "s", Messages: []json.RawMessage{json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hi"}]}`)}, ReasoningEffort: effort,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, has := raw["reasoning_off"]; has {
			t.Fatalf("(9) CallWithTools with effort %q sent reasoning_off", effort)
		}
	}
}
