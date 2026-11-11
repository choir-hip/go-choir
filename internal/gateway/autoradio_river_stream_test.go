package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The Autoradio desk's streamed call through River (owner 2026-11-11: River
// DeepSeek leads the autoradio pool). The whole path is real except the far
// end: a fake River speaking OpenAI Chat Completions SSE, the real
// RiverProvider.Stream, the real gateway handler (handleStreamingInference),
// and the real guest client (gatewayruntime.StreamText). Ways it could fail,
// written before the test:
//
//  1. River's content deltas reach the guest only when the answer is finished
//     (the first sentence is then as late as the whole reply);
//  2. River's reasoning_content reaches the guest as text to speak;
//  3. River's 402 or quota 429, before any chunk, reaches the guest as a
//     stream with no delta rather than as an error before any delta;
//  4. a River failure after the first chunk reaches the guest as a clean end.

const riverStreamModel = "deepseek-ai/DeepSeek-V4.1-Flash-1017"

func riverStreamRig(t *testing.T, river http.HandlerFunc) *gatewayruntime.Provider {
	t.Helper()
	upstream := httptest.NewServer(river)
	t.Cleanup(upstream.Close)
	rp, err := provider.NewRiverProvider(provider.RiverConfig{APIKey: "rv-test-key", ModelID: riverStreamModel, BaseURL: upstream.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	mp := provider.NewMultiProvider()
	mp.Register("river", rp)
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-river-stream")
	mux := http.NewServeMux()
	mux.HandleFunc("/provider/v1/inference", h.HandleInference)
	gw := httptest.NewServer(mux)
	t.Cleanup(gw.Close)
	return gatewayruntime.New(gw.URL, cred.RawToken)
}

func riverChunk(w http.ResponseWriter, payload string) {
	fmt.Fprintf(w, "data: %s\n\n", payload)
	w.(http.Flusher).Flush()
}

func TestRiverStreamsTheAutoradioAnswerSentenceBySentence(t *testing.T) {
	release := make(chan struct{})
	river := riverStreamRig(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		riverChunk(w, `{"id":"r1","model":"`+riverStreamModel+`","choices":[{"index":0,"delta":{"role":"assistant","reasoning_content":"thinking about it"}}]}`)
		riverChunk(w, `{"id":"r1","choices":[{"index":0,"delta":{"content":"The top story is a strike. "}}]}`)
		<-release
		riverChunk(w, `{"id":"r1","choices":[{"index":0,"delta":{"content":"Talks resume Monday."},"finish_reason":"stop"}]}`)
		riverChunk(w, `{"id":"r1","choices":[],"usage":{"prompt_tokens":120,"completion_tokens":14,"total_tokens":134}}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	first := make(chan string, 4)
	type result struct {
		resp *provideriface.TextStreamResponse
		err  error
	}
	done := make(chan result, 1)
	var deltas []string
	go func() {
		resp, err := river.StreamText(context.Background(), provideriface.TextStreamRequest{
			Provider: "river", Model: riverStreamModel, ReasoningEffort: "low", System: "s", User: "u", MaxTokens: 220,
		}, func(d string) { deltas = append(deltas, d); first <- d })
		done <- result{resp, err}
	}()
	select {
	case d := <-first:
		if !strings.Contains(d, "top story") {
			t.Fatalf("first delta %q", d)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("(1) River's first delta did not reach the guest before the stream finished")
	}
	close(release)
	r := <-done
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.resp.Text != "The top story is a strike. Talks resume Monday." || strings.Contains(strings.Join(deltas, ""), "thinking") {
		t.Fatalf("(2) text %q deltas %q", r.resp.Text, deltas)
	}
	if r.resp.Usage.InputTokens != 120 || r.resp.Usage.OutputTokens != 14 {
		t.Fatalf("usage %+v", r.resp.Usage)
	}
}

func TestRiverRefusalBeforeAnyChunkIsAnErrorBeforeAnyDelta(t *testing.T) {
	river := riverStreamRig(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		fmt.Fprint(w, `{"error":{"message":"credit exhausted"}}`)
	})
	called := false
	_, err := river.StreamText(context.Background(), provideriface.TextStreamRequest{Provider: "river", Model: riverStreamModel, User: "u"}, func(string) { called = true })
	if err == nil || called {
		t.Fatalf("(3) err %v, delta called %v", err, called)
	}
	if _, ok := provideriface.AsProviderQuota(err); !ok {
		t.Fatalf("(3) a spent River is not the typed quota answer: %v", err)
	}
}

func TestRiverFailureAfterTheFirstChunkIsAnError(t *testing.T) {
	river := riverStreamRig(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		riverChunk(w, `{"id":"r1","model":"`+riverStreamModel+`","choices":[{"index":0,"delta":{"content":"Half an "}}]}`)
		// The connection ends with no finish chunk and no [DONE].
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	})
	var deltas []string
	_, err := river.StreamText(context.Background(), provideriface.TextStreamRequest{Provider: "river", Model: riverStreamModel, User: "u"}, func(d string) { deltas = append(deltas, d) })
	if err == nil {
		t.Fatalf("(4) a River stream cut after %q ended clean", deltas)
	}
}
