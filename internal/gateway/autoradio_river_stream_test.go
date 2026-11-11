package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
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
			Provider: "river", Model: riverStreamModel, ReasoningOff: true, System: "s", StableUser: "page", User: "u",
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

// Bedrock Haiku 5.5 is the autoradio pool's fallback (owner 2026-11-11).
// Bedrock does not stream (BedrockProvider.Stream makes one whole call and
// emits the text as one chunk), so ways it could fail, written before the
// test:
//
//  5. the call carries tools, or loses max_tokens, or runs with thinking off
//     (owner 2026-11-09: never off) or a budget_tokens shape Bedrock refuses;
//  6. the model's thinking block is spoken;
//  7. a Bedrock refusal before any chunk reaches the guest as a stream with no
//     delta rather than an error before any delta.
const bedrockStreamModel = "us.anthropic.claude-haiku-5-5"

func bedrockStreamRig(t *testing.T, bedrock http.HandlerFunc) *gatewayruntime.Provider {
	t.Helper()
	upstream := httptest.NewServer(bedrock)
	t.Cleanup(upstream.Close)
	bp, err := provider.NewBedrockProvider(provider.BedrockConfig{Region: "us-east-1", ModelID: bedrockStreamModel, AuthToken: "bd-test-token", BaseURL: upstream.URL})
	if err != nil {
		t.Fatal(err)
	}
	mp := provider.NewMultiProvider()
	mp.Register("bedrock", bp)
	reg := NewIdentityRegistry(time.Hour)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-bedrock-stream")
	mux := http.NewServeMux()
	mux.HandleFunc("/provider/v1/inference", h.HandleInference)
	gw := httptest.NewServer(mux)
	t.Cleanup(gw.Close)
	return gatewayruntime.New(gw.URL, cred.RawToken)
}

func TestBedrockHaikuAnswersTheAutoradioCallWithNoToolsAndASpendableBudget(t *testing.T) {
	var path string
	var body map[string]any
	bedrock := bedrockStreamRig(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_1","model":"`+bedrockStreamModel+`","stop_reason":"end_turn","content":[{"type":"thinking","thinking":"private reasoning","signature":"sig"},{"type":"text","text":"The top story is a strike. Talks resume Monday."}],"usage":{"input_tokens":90,"output_tokens":40}}`)
	})
	var deltas []string
	resp, err := bedrock.StreamText(context.Background(), provideriface.TextStreamRequest{
		Provider: "bedrock", Model: bedrockStreamModel, ReasoningOff: true, System: "s", StableUser: "page block", User: "u",
	}, func(d string) { deltas = append(deltas, d) })
	if err != nil {
		t.Fatal(err)
	}
	if path != "/model/"+bedrockStreamModel+"/invoke" {
		t.Fatalf("path %q", path)
	}
	if _, has := body["tools"]; has {
		t.Fatalf("(5) the call carries tools: %v", body["tools"])
	}
	// No token budget of the desk's: the model's own ceiling from the catalog.
	if want := float64(modelcatalog.MaxOutputTokensForModel(bedrockStreamModel)); body["max_tokens"] != want {
		t.Fatalf("(5) max_tokens %v, want the catalog ceiling %v", body["max_tokens"], want)
	}
	raw, _ := json.Marshal(body)
	// Reasoning off for the autoradio role: thinking disabled (Haiku 5.5
	// accepts it at effort high or below), effort stays low, never a budget.
	if !strings.Contains(string(raw), `"thinking":{"type":"disabled"}`) || !strings.Contains(string(raw), `"effort":"low"`) || strings.Contains(string(raw), "budget_tokens") {
		t.Fatalf("(5) thinking shape: %s", raw)
	}
	// Cache markers: the system block (1 hour) and the stable page block
	// (5 minutes), not the volatile block.
	msgs, _ := body["messages"].([]any)
	content, _ := msgs[0].(map[string]any)["content"].([]any)
	if len(content) != 2 || content[0].(map[string]any)["cache_control"] == nil || content[1].(map[string]any)["cache_control"] != nil {
		t.Fatalf("(5) cache markers on the user blocks: %v", content)
	}
	sys, _ := body["system"].([]any)
	if len(sys) != 1 || sys[0].(map[string]any)["cache_control"] == nil {
		t.Fatalf("(5) system block %v", body["system"])
	}
	spoken := strings.Join(deltas, "")
	if spoken != "The top story is a strike. Talks resume Monday." || strings.Contains(spoken, "private") || resp.Text != spoken {
		t.Fatalf("(6) deltas %q text %q", deltas, resp.Text)
	}
	if resp.Usage.InputTokens != 90 || resp.Usage.OutputTokens != 40 {
		t.Fatalf("usage %+v", resp.Usage)
	}
}

func TestBedrockRefusalBeforeAnyChunkIsAnErrorBeforeAnyDelta(t *testing.T) {
	bedrock := bedrockStreamRig(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"message":"Too many requests"}`)
	})
	called := false
	_, err := bedrock.StreamText(context.Background(), provideriface.TextStreamRequest{Provider: "bedrock", Model: bedrockStreamModel, User: "u"}, func(string) { called = true })
	if err == nil || called {
		t.Fatalf("(7) err %v, delta called %v", err, called)
	}
}

// River takes reasoning_effort "none" for the autoradio call (River documents
// it as off, and off as the DeepSeek default), sends no max_tokens, and still
// floors every other call at "low" (the 2026-11-09 rule is River-compatible
// by our choice, not River's). The page block and the words arrive as one
// message, in order, so River's prefix cache sees the stable part first.
func TestRiverAutoradioCallRequestBody(t *testing.T) {
	var bodies []map[string]any
	river := riverStreamRig(t, func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		bodies = append(bodies, b)
		w.Header().Set("Content-Type", "text/event-stream")
		riverChunk(w, `{"id":"r1","choices":[{"index":0,"delta":{"content":"Ok."},"finish_reason":"stop"}]}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	})
	for _, off := range []bool{true, false} {
		_, err := river.StreamText(context.Background(), provideriface.TextStreamRequest{
			Provider: "river", Model: riverStreamModel, ReasoningEffort: "none", ReasoningOff: off,
			System: "s", StableUser: "PAGE", User: "WORDS",
		}, func(string) {})
		if err != nil {
			t.Fatal(err)
		}
	}
	if bodies[0]["reasoning_effort"] != "none" {
		t.Fatalf("off: reasoning_effort %v", bodies[0]["reasoning_effort"])
	}
	if bodies[1]["reasoning_effort"] != "low" {
		t.Fatalf("an unexempted call must keep the floor: %v", bodies[1]["reasoning_effort"])
	}
	for i, b := range bodies {
		if _, has := b["max_tokens"]; has {
			t.Fatalf("body %d carries max_tokens: %v", i, b["max_tokens"])
		}
		if _, has := b["tools"]; has {
			t.Fatalf("body %d carries tools", i)
		}
		msgs := b["messages"].([]any)
		user := msgs[len(msgs)-1].(map[string]any)["content"]
		if user != "PAGE\nWORDS" {
			t.Fatalf("body %d user message %v", i, user)
		}
	}
}
