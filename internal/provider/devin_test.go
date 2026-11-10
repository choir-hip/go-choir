package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

const devinTestKey = "devin-session-token$aaaaaaaa.bbbbbbbb.cccccccc"

// devinFixture is a Devin backend double: GetUserJwt issues a counted token
// and GetChatMessage answers with the scripted response.
type devinFixture struct {
	srv      *httptest.Server
	jwtCalls atomic.Int32
	chatSeen atomic.Int32
	mu       sync.Mutex
	lastBody []byte
	// chat writes the chat response; n is the 1-based call number.
	chat func(w http.ResponseWriter, n int)
}

func newDevinFixture(t *testing.T, chat func(w http.ResponseWriter, n int)) (*devinFixture, *DevinProvider) {
	t.Helper()
	f := &devinFixture{chat: chat}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/GetUserJwt"):
			n := f.jwtCalls.Add(1)
			time.Sleep(20 * time.Millisecond)
			w.Write(pbString(1, "user-jwt-token-"+string(rune('a'+n))))
		case strings.HasSuffix(r.URL.Path, "/GetChatMessage"):
			f.mu.Lock()
			f.lastBody = body
			f.mu.Unlock()
			f.chat(w, int(f.chatSeen.Add(1)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	p, err := NewDevinProvider(devinTestKey, f.srv.URL, "swe-2-flash")
	if err != nil {
		t.Fatal(err)
	}
	return f, p
}

func devinTestReq(msgs ...Message) LLMRequest {
	if len(msgs) == 0 {
		msgs = []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}
	}
	return LLMRequest{Model: "swe-2-flash", Messages: msgs}
}

func writeFrames(w http.ResponseWriter, frames ...[]byte) {
	w.Header().Set("Content-Type", "application/connect+proto")
	for _, f := range frames {
		w.Write(f)
	}
}

func okFrames(text string) [][]byte {
	msg := append(pbString(1, "msg-1"), pbString(3, text)...)
	msg = append(msg, pbMessage(7, append(pbVarintField(2, 11), pbVarintField(3, 5)...))...)
	return [][]byte{devinConnectFrame(msg, 0), devinConnectFrame([]byte(`{}`), devinConnectFlagTrailer)}
}

func trailer(code, message string, extra string) []byte {
	b, _ := json.Marshal(map[string]any{"code": code, "message": message})
	return devinConnectFrame([]byte(`{"error":`+string(b)+extra+`}`), devinConnectFlagTrailer)
}

func TestDevinHappyPathUsageAndChunks(t *testing.T) {
	_, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) { writeFrames(w, okFrames("pong")...) })
	var kinds []string
	resp, err := p.Stream(context.Background(), devinTestReq(), func(c StreamChunk) { kinds = append(kinds, c.Type) })
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "pong" || resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 5 || !resp.Usage.CacheReported {
		t.Fatalf("resp = %+v", resp)
	}
	if len(kinds) == 0 || kinds[0] != "message_start" || kinds[len(kinds)-1] != "message_stop" {
		t.Fatalf("chunk order = %v", kinds)
	}
}

// Failure list for Devin's refusals (C): HTTP 429 with Retry-After; Connect
// resource_exhausted in an end-stream frame with a reset phrase; one with
// none; a short rate limit; permission_denied; unauthenticated that survives
// a refresh; unavailable (503); a stream cut before its end frame; a
// truncated frame; a hostile length prefix; an empty answer.
func TestDevinRefusalsClassifyForThePool(t *testing.T) {
	cases := []struct {
		name  string
		chat  func(w http.ResponseWriter, n int)
		check func(t *testing.T, err error)
	}{
		{"http 429 with Retry-After is quota with that reset", func(w http.ResponseWriter, _ int) {
			w.Header().Set("Retry-After", "120")
			w.WriteHeader(429)
			w.Write([]byte(`{"code":"resource_exhausted","message":"slow"}`))
		}, func(t *testing.T, err error) {
			q, ok := AsQuotaExhausted(err)
			if !ok || q.Provider != "devin" || time.Until(q.RetryAt) < 100*time.Second || time.Until(q.RetryAt) > 130*time.Second {
				t.Fatalf("err = %v quota=%+v", err, q)
			}
		}},
		{"trailer resource_exhausted with a phrase has a reset", func(w http.ResponseWriter, _ int) {
			writeFrames(w, trailer("resource_exhausted", "Usage limit reached, try again in 5 minutes", ""))
		}, func(t *testing.T, err error) {
			q, ok := AsQuotaExhausted(err)
			if !ok || time.Until(q.RetryAt) < 4*time.Minute || time.Until(q.RetryAt) > 6*time.Minute {
				t.Fatalf("err = %v quota=%+v", err, q)
			}
		}},
		{"trailer resource_exhausted reads a retryDelay detail", func(w http.ResponseWriter, _ int) {
			writeFrames(w, devinConnectFrame([]byte(`{"error":{"code":"resource_exhausted","message":"out","details":[{"debug":{"retryDelay":"90s"}}]}}`), devinConnectFlagTrailer))
		}, func(t *testing.T, err error) {
			q, ok := AsQuotaExhausted(err)
			if !ok || time.Until(q.RetryAt) < 80*time.Second || time.Until(q.RetryAt) > 100*time.Second {
				t.Fatalf("err = %v quota=%+v", err, q)
			}
		}},
		{"trailer resource_exhausted without a reset is quota with none", func(w http.ResponseWriter, _ int) {
			writeFrames(w, trailer("resource_exhausted", "You have run out of credits", ""))
		}, func(t *testing.T, err error) {
			q, ok := AsQuotaExhausted(err)
			if !ok || !q.RetryAt.IsZero() || !IsQuotaExhausted(err) {
				t.Fatalf("err = %v quota=%+v", err, q)
			}
		}},
		{"only a retry time under 10 seconds is a short rate limit", func(w http.ResponseWriter, _ int) {
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(429)
			w.Write([]byte(`{"code":"resource_exhausted","message":"Rate limit exceeded"}`))
		}, func(t *testing.T, err error) {
			if IsQuotaExhausted(err) || !errors.Is(err, ErrUpstreamConcurrencyLimit) {
				t.Fatalf("err = %v", err)
			}
		}},
		{"rate-limit words with no reset time are still quota (fail over at once)", func(w http.ResponseWriter, _ int) {
			writeFrames(w, trailer("resource_exhausted", "Rate limit exceeded: too many requests", ""))
		}, func(t *testing.T, err error) {
			if !IsQuotaExhausted(err) || errors.Is(err, ErrUpstreamConcurrencyLimit) {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a 30 second retry time is quota held until then", func(w http.ResponseWriter, _ int) {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(429)
			w.Write([]byte(`{"code":"resource_exhausted","message":"slow down"}`))
		}, func(t *testing.T, err error) {
			q, ok := AsQuotaExhausted(err)
			if !ok || time.Until(q.RetryAt) < 20*time.Second {
				t.Fatalf("err = %v quota=%+v", err, q)
			}
		}},
		{"permission_denied is held as quota", func(w http.ResponseWriter, _ int) {
			w.WriteHeader(403)
			w.Write([]byte(`{"code":"permission_denied","message":"model not available on your plan"}`))
		}, func(t *testing.T, err error) {
			if !IsQuotaExhausted(err) {
				t.Fatalf("err = %v", err)
			}
		}},
		{"unavailable keeps a 503 status for the breaker", func(w http.ResponseWriter, _ int) {
			w.WriteHeader(503)
			w.Write([]byte(`{"code":"unavailable","message":"overloaded"}`))
		}, func(t *testing.T, err error) {
			if IsQuotaExhausted(err) || !strings.Contains(err.Error(), ": status 503") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a trailer unavailable also reads as 503", func(w http.ResponseWriter, _ int) {
			writeFrames(w, trailer("unavailable", "try later", ""))
		}, func(t *testing.T, err error) {
			if !strings.Contains(err.Error(), ": status 503") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"stream cut before the end frame is an error", func(w http.ResponseWriter, _ int) {
			writeFrames(w, okFrames("partial")[0])
		}, func(t *testing.T, err error) {
			if err == nil || !strings.Contains(err.Error(), "truncated") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a frame cut mid payload is an error", func(w http.ResponseWriter, _ int) {
			writeFrames(w, okFrames("partial")[0][:8])
		}, func(t *testing.T, err error) {
			if err == nil || !strings.Contains(err.Error(), "truncated") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a hostile length prefix is refused without allocating it", func(w http.ResponseWriter, _ int) {
			writeFrames(w, []byte{0, 0xff, 0xff, 0xff, 0xff, 1, 2, 3})
		}, func(t *testing.T, err error) {
			if err == nil || !strings.Contains(err.Error(), "cap") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"a large legal prefix with few bytes sent is truncation", func(w http.ResponseWriter, _ int) {
			writeFrames(w, []byte{0, 0x00, 0xf0, 0x00, 0x00, 1, 2, 3})
		}, func(t *testing.T, err error) {
			if err == nil || !strings.Contains(err.Error(), "truncated") {
				t.Fatalf("err = %v", err)
			}
		}},
		{"an empty answer is an error", func(w http.ResponseWriter, _ int) {
			writeFrames(w, devinConnectFrame([]byte(`{}`), devinConnectFlagTrailer))
		}, func(t *testing.T, err error) {
			if err == nil || !strings.Contains(err.Error(), "no content") {
				t.Fatalf("err = %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, p := newDevinFixture(t, tc.chat)
			_, err := p.Call(context.Background(), devinTestReq())
			if err == nil {
				t.Fatal("expected an error")
			}
			tc.check(t, err)
			if strings.Contains(err.Error(), "aaaaaaaa") {
				t.Fatalf("error carries the credential: %v", err)
			}
		})
	}
}

// An expired user JWT (401 or an unauthenticated end frame before output) is
// refreshed once; a second refusal is held as quota.
func TestDevinUnauthenticatedRefreshesOnceThenHolds(t *testing.T) {
	f, p := newDevinFixture(t, func(w http.ResponseWriter, n int) {
		if n == 1 {
			w.WriteHeader(401)
			return
		}
		writeFrames(w, okFrames("again")...)
	})
	resp, err := p.Call(context.Background(), devinTestReq())
	if err != nil || resp.Text != "again" || f.jwtCalls.Load() != 2 {
		t.Fatalf("resp=%v err=%v jwt calls=%d", resp, err, f.jwtCalls.Load())
	}

	f2, p2 := newDevinFixture(t, func(w http.ResponseWriter, _ int) {
		writeFrames(w, trailer("unauthenticated", "token rejected", ""))
	})
	_, err = p2.Call(context.Background(), devinTestReq())
	if !IsQuotaExhausted(err) || f2.chatSeen.Load() != 2 {
		t.Fatalf("err=%v chat calls=%d", err, f2.chatSeen.Load())
	}
}

func TestDevinUserJWTRefreshIsSingleFlight(t *testing.T) {
	f, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) { writeFrames(w, okFrames("x")...) })
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Call(context.Background(), devinTestReq()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := f.jwtCalls.Load(); n != 1 {
		t.Fatalf("GetUserJwt called %d times for 12 concurrent calls, want 1", n)
	}
}

func TestDevinContextCancelEndsTheCall(t *testing.T) {
	release := make(chan struct{})
	_, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-time.After(10 * time.Second):
		}
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := p.Call(ctx, devinTestReq()); err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("err=%v after %v", err, time.Since(start))
	}
}

func TestDevinErrorsNeverCarryCredentials(t *testing.T) {
	_, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) {
		w.WriteHeader(500)
		w.Write([]byte(`{"code":"internal","message":"bad token ` + devinTestKey + ` and eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abcdefghij"}`))
	})
	_, err := p.Call(context.Background(), devinTestReq())
	if err == nil || strings.Contains(err.Error(), "aaaaaaaa") || strings.Contains(err.Error(), "eyJhbGci") || strings.Contains(err.Error(), "devin-session-token$") {
		t.Fatalf("err = %v", err)
	}
}

func TestDevinModelUIDMapsEffort(t *testing.T) {
	cases := []struct{ model, effort, want string }{
		{"swe-2-flash", "low", "swe-2-flash-medium"}, // policy "low" runs medium, by design
		{"swe-2-flash", "minimal", "swe-2-flash-medium"},
		{"swe-2-flash", "medium", "swe-2-flash-medium"},
		{"swe-2-flash", "", "swe-2-flash-medium"},
		{"swe-2-flash", "high", "swe-2-flash-high"},
		{"swe-2-flash", "xhigh", "swe-2-flash-max"},
		{"swe-2-flash", "max", "swe-2-flash-max"},
		{"swe-2", "low", "swe-2-medium"},
		{"swe-2", "high", "swe-2-high"},
		{"swe-2", "", "swe-2-high"},
		{"swe-2-flash-high", "low", "swe-2-flash-high"},
	}
	for _, c := range cases {
		if got := devinModelUID(c.model, c.effort); got != c.want {
			t.Errorf("devinModelUID(%q,%q) = %q, want %q", c.model, c.effort, got, c.want)
		}
	}
}

// decodeDevinTurns reads the GetChatMessage request the fixture saw.
func decodeDevinTurns(t *testing.T, body []byte) (turns []map[string]any, fields []pbField) {
	t.Helper()
	if len(body) < 5 {
		t.Fatal("short request")
	}
	fields = pbFields(body[5:])
	for _, sub := range pbSubmessages(fields, 3) {
		turn := map[string]any{"source": pbVarintOf(sub, 2), "text": pbStringField(sub, 3), "tool_id": pbStringField(sub, 7)}
		turn["calls"] = len(pbSubmessages(sub, 6))
		turn["images"] = len(pbSubmessages(sub, 10))
		turn["thinking"] = pbStringField(sub, 11)
		turns = append(turns, turn)
	}
	return turns, fields
}

func pbVarintOf(fields []pbField, num int) uint64 {
	for _, f := range fields {
		if f.Num == num && f.Wire == 0 {
			return f.Varint
		}
	}
	return 0
}

// A transcript that began on another provider: the tool loop's user message
// of tool_result blocks, parallel calls, an orphan result, an unanswered
// call, thinking, and an image for a text-only model.
func TestDevinHistoryFromOtherProviders(t *testing.T) {
	f, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) { writeFrames(w, okFrames("ok")...) })
	img := &MediaSource{Kind: "base64", MIMEType: "image/png", Data: "AAAA"}
	msgs := []Message{
		{Role: "user", Content: []Block{{Type: "text", Text: "go"}, {Type: "image", Source: img}}},
		{Role: "assistant", ReasoningContent: "secret reasoning", Content: []Block{
			{Type: "thinking", Text: "hmm"},
			{Type: "text", Text: "calling"},
			{Type: "tool_use", ID: "c1", Name: "a", Input: json.RawMessage(`{"x":1}`)},
			{Type: "tool_use", ID: "c2", Name: "b", Input: json.RawMessage(`{}`)},
		}},
		{Role: "user", Content: []Block{
			{Type: "tool_result", ToolUseID: "c2", Text: "result two"},
			{Type: "tool_result", ToolUseID: "c1", Text: "result one", IsError: true},
			{Type: "tool_result", ToolUseID: "ghost", Text: "orphan"},
		}},
		{Role: "assistant", Content: []Block{{Type: "tool_use", ID: "c3", Name: "z", Input: json.RawMessage(`{}`)}}},
		{Role: "user", Content: []Block{{Type: "text", Text: "continue"}}},
	}
	if _, err := p.Call(context.Background(), devinTestReq(msgs...)); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	body := f.lastBody
	f.mu.Unlock()
	turns, _ := decodeDevinTurns(t, body)
	type want struct {
		source uint64
		tool   string
		text   string
	}
	var got []want
	for _, tn := range turns {
		got = append(got, want{tn["source"].(uint64), tn["tool_id"].(string), tn["text"].(string)})
		if tn["thinking"].(string) != "" || tn["images"].(int) != 0 {
			t.Fatalf("thinking or image reached a text-only model: %v", tn)
		}
	}
	exp := []want{
		{1, "", "go\n[an image was omitted: this model reads text only]"},
		{2, "", "calling"},
		{4, "c1", "result one"}, // in the order of the calls, not of the results
		{4, "c2", "result two"},
		{1, "", "[tool result] orphan"}, // answers no call: plain text
		{2, "", ""},
		{4, "c3", devinNoToolResult}, // unanswered call is answered
		{1, "", "continue"},
	}
	if len(got) != len(exp) {
		t.Fatalf("turns = %+v", got)
	}
	for i := range exp {
		if got[i] != exp[i] {
			t.Fatalf("turn %d = %+v, want %+v (all: %+v)", i, got[i], exp[i], got)
		}
	}
	if turns[1]["calls"].(int) != 2 || turns[5]["calls"].(int) != 1 {
		t.Fatalf("assistant calls = %v, %v", turns[1]["calls"], turns[5]["calls"])
	}
}

func TestDevinRequestForbidsParallelCallsForFlash(t *testing.T) {
	f, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) { writeFrames(w, okFrames("ok")...) })
	req := devinTestReq()
	req.Tools = []ToolDef{{Name: "t", InputSchema: map[string]any{"type": "object"}}}
	if _, err := p.Call(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	_, fields := decodeDevinTurns(t, f.lastBody)
	f.mu.Unlock()
	if pbVarintOf(fields, 11) != 1 {
		t.Fatal("swe-2-flash request does not disable parallel tool calls")
	}
	if got := pbStringField(fields, 21); got != "swe-2-flash-medium" {
		t.Fatalf("model uid = %q", got)
	}
}

func TestDevinToolCallArgumentsAccumulateAndTruncationFails(t *testing.T) {
	call := func(args string) []byte {
		return pbMessage(6, append(append(pbString(1, "call-1"), pbString(2, "get")...), pbString(3, args)...))
	}
	_, p := newDevinFixture(t, func(w http.ResponseWriter, _ int) {
		writeFrames(w, devinConnectFrame(call(`{"a"`), 0), devinConnectFrame(call(`{"a": 1}`), 0), devinConnectFrame([]byte(`{}`), devinConnectFlagTrailer))
	})
	var deltas []string
	resp, err := p.Stream(context.Background(), devinTestReq(), func(c StreamChunk) {
		if c.Type == "toolcall_delta" {
			deltas = append(deltas, c.ToolCallDelta)
		}
	})
	if err != nil || len(resp.ToolCalls) != 1 || string(resp.ToolCalls[0].Arguments) != `{"a": 1}` || resp.StopReason != "tool_use" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if strings.Join(deltas, "") != `{"a": 1}` {
		t.Fatalf("deltas = %q, want the arguments once", deltas)
	}

	_, p2 := newDevinFixture(t, func(w http.ResponseWriter, _ int) {
		writeFrames(w, devinConnectFrame(call(`{"a": `), 0), devinConnectFrame([]byte(`{}`), devinConnectFlagTrailer))
	})
	if _, err := p2.Call(context.Background(), devinTestReq()); err == nil {
		t.Fatal("a cut-off tool call must not run with empty arguments")
	}
}

func TestDevinServerNamedBaseURLIsPinned(t *testing.T) {
	const base = "https://server.codeium.com"
	for raw, ok := range map[string]bool{
		"https://server.codeium.com":                true,
		"https://SERVER.Codeium.com":                true,
		"https://server.codeium.com:443":            true,
		"https://eu.server.codeium.com":             true,
		"https://api.windsurf.com":                  true,
		"https://x.devin.ai":                        true,
		"https://codeium.com":                       true,
		"https://server.codeium.com.evil.example":   false,
		"https://evilcodeium.com":                   false,
		"https://evil.example:8443":                 false,
		"https://server.codeium.com:8443":           false,
		"https://localhost.":                        false,
		"https://localhost":                         false,
		"https://foo.localhost":                     false,
		"https://169.254.169.254.nip.io":            false,
		"https://0x7f.0.0.1":                        false,
		"https://127.1":                             false,
		"https://[fe80::1%25a.b]":                   false,
		"https://[::1]":                             false,
		"https://server.codeium.com.":               false,
		"https://user@server.codeium.com":           false,
		"https://user:pw@server.codeium.com":        false,
		"http://server.codeium.com":                 false,
		"https://":                                  false,
		"https://-bad.codeium.com":                  false,
		"https://under_score.codeium.com":           false,
		"https://server.codeium.com%2eevil.example": false,
		"https://xn--e1afmkfd.codeium.com.evil":     false,
		"https://server.codeium.com@evil.example":   false,
		"https://evil.example/server.codeium.com":   false,
	} {
		if got := devinAcceptableOverride(raw, base); got != ok {
			t.Errorf("devinAcceptableOverride(%q) = %v, want %v", raw, got, ok)
		}
	}
	// A configured base host is accepted exactly, whatever its domain.
	if !devinAcceptableOverride("https://gw.example.org", "https://gw.example.org") || devinAcceptableOverride("https://other.example.org", "https://gw.example.org") {
		t.Error("the configured host must be the only non-allowlisted host")
	}
}

func TestDevinConstructorValidatesBaseURL(t *testing.T) {
	for raw, ok := range map[string]bool{
		"":                                true, // default
		"https://server.codeium.com":      true,
		"http://127.0.0.1:9999":           true, // loopback only: no remote host can receive the key
		"http://localhost:9999":           true,
		"http://[::1]:9999":               true,
		"http://server.codeium.com":       false,
		"http://evil.example":             false,
		"https://user@server.codeium.com": false,
		"https://user:pw@127.0.0.1":       false,
		"ftp://server.codeium.com":        false,
		"https://":                        false,
		"server.codeium.com":              false,
	} {
		_, err := NewDevinProvider(devinTestKey, raw, "")
		if (err == nil) != ok {
			t.Errorf("NewDevinProvider(base %q) err=%v, want ok=%v", raw, err, ok)
		}
		if err != nil && strings.Contains(err.Error(), "aaaaaaaa") {
			t.Errorf("constructor error carries the key: %v", err)
		}
	}
}

// A redirect must never carry the request body (the session key, the user
// JWT) to another host: any 3xx is an error and the target sees nothing.
func TestDevinRedirectsAreNeverFollowed(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer target.Close()
	for _, code := range []int{301, 302, 307, 308} {
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, target.URL+r.URL.Path, code)
		}))
		p, err := NewDevinProvider(devinTestKey, origin.URL, "swe-2-flash")
		if err != nil {
			t.Fatal(err)
		}
		_, err = p.Call(context.Background(), devinTestReq())
		origin.Close()
		if err == nil {
			t.Fatalf("redirect %d was followed to success", code)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("redirect target received %d requests", hits.Load())
	}
}

func TestDevinSanitizeDoesNotSplitARune(t *testing.T) {
	p := &DevinProvider{}
	for pad := range 4 {
		in := strings.Repeat("a", 298-pad) + strings.Repeat("é", 10)
		out := p.devinSanitize(in)
		if !utf8.ValidString(out) || len(out) > 300 {
			t.Fatalf("pad %d: valid=%v len=%d", pad, utf8.ValidString(out), len(out))
		}
	}
}
