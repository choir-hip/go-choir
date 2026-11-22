package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// The desk and run on every model call's gateway line (first design, F0,
// docs/design/first-design-2026-11-22.md section 10 step 0). Failure list,
// written before the code:
//
//  1. The inference line carries neither the desk nor the run, so the paper's
//     calls cannot be counted by desk.
//  2. A caller that sends none (an old guest) logs an empty field or "-"
//     rather than the explicit desk=unknown; its run is not "-".
//  3. A header value with a space forges a field (the HTTP client refuses a
//     newline, the log cleaner drops it too).
//  4. The text parsers read ("inference request from autoputer", "provider=X
//     model=Y") change.
//  5. A Jev call line carries no desk or run, and a host-code call to Jev (the
//     host judgment route) carries none that says it is host code.

func postInferenceWith(t *testing.T, url, token, body string, hdr map[string]string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url+"/provider/v1/inference", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range hdr {
		req.Header[k] = []string{v}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func TestInferenceLineCarriesTheDeskAndTheRun(t *testing.T) {
	logs := captureLog(t)
	river := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Ok.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}
	url, token := gatewayRig(t, river, nil)
	body := `{"provider":"river","model":"` + riverStreamModel + `","stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	postInferenceWith(t, url, token, body, map[string]string{"X-Choir-Run": "run-9", "X-Choir-Desk": "texture"})
	postInferenceWith(t, url, token, body, nil)
	postInferenceWith(t, url, token, body, map[string]string{"X-Choir-Run": "run 1 forged", "X-Choir-Desk": "man agement"})
	out := logs.String()
	if !strings.Contains(out, " stream=true desk=texture run=run-9 ") {
		t.Fatalf("(1) the line lacks desk and run:\n%s", out)
	}
	if !strings.Contains(out, " stream=true desk=unknown run=- ") {
		t.Fatalf("(2) an unattributed call must say desk=unknown run=-:\n%s", out)
	}
	if !strings.Contains(out, " desk=management") || !strings.Contains(out, " run=run1forged ") {
		t.Fatalf("(3) header values are not cleaned:\n%s", out)
	}
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.Contains(l, "inference request from") {
			if !strings.Contains(l, "gateway: inference request from autoputer ") || !strings.Contains(l, "(provider=river model=") {
				t.Fatalf("(4) the line's existing text changed: %q", l)
			}
		} else if strings.Contains(l, "desk=") && !strings.Contains(l, "inference") {
			t.Fatalf("(3) a header forged a line: %q", l)
		}
	}
}

func TestModelCallerValuesAreExplicit(t *testing.T) {
	for _, c := range []struct{ desk, run, wantDesk, wantRun string }{
		{"", "", "unknown", "-"},
		{"research", "r1", "research", "r1"},
		{strings.Repeat("a", 80), "", strings.Repeat("a", 64), "-"},
	} {
		r, _ := http.NewRequest(http.MethodPost, "/", nil)
		if c.desk != "" {
			r.Header.Set("X-Choir-Desk", c.desk)
		}
		if c.run != "" {
			r.Header.Set("X-Choir-Run", c.run)
		}
		d, run := modelCaller(r)
		if d != c.wantDesk || run != c.wantRun {
			t.Fatalf("modelCaller(%q,%q) = %q,%q; want %q,%q", c.desk, c.run, d, run, c.wantDesk, c.wantRun)
		}
	}
}

func TestJevCallLineCarriesTheDeskAndRun(t *testing.T) {
	logs := captureLog(t)
	tr := &JevTransport{enabled: true, apiKey: "k", httpClient: &http.Client{Transport: &statusRoundTripper{status: http.StatusOK}},
		receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "r.jsonl"))}
	req := JudgmentRequest{State: json.RawMessage(`{"a":1}`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)}}
	for _, c := range []struct {
		hdr  map[string]string
		want string
	}{
		{map[string]string{"X-Choir-Desk": "host", "X-Choir-Run": "run-3"}, " desk=host run=run-3 "},
		{nil, " desk=unknown run=- "},
	} {
		logs.Reset()
		r, _ := http.NewRequest(http.MethodPost, "/", nil)
		for k, v := range c.hdr {
			r.Header.Set(k, v)
		}
		_, _ = tr.Decide(withJevSite(context.Background(), r), "c", req)
		line := logs.String()
		if !strings.Contains(line, "gateway: jev call computer=c ") || !strings.Contains(line, c.want) || !strings.Contains(line, "site=unknown ") {
			t.Fatalf("(5) jev line = %q; want %q", line, c.want)
		}
	}
}

func TestHostJudgmentRouteLogsDeskHost(t *testing.T) {
	logs := captureLog(t)
	tr := &JevTransport{enabled: true, apiKey: "k", httpClient: &http.Client{Transport: &statusRoundTripper{status: http.StatusOK}},
		receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "r.jsonl"))}
	req := JudgmentRequest{State: json.RawMessage(`{"a":1}`), Questions: map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","choices":["a","b"]}`)}}
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("X-Choir-Desk", "management") // a header cannot make host code a desk
	_, _ = tr.Decide(withHostDesk(withJevSite(context.Background(), r)), "host:front-importance", req)
	if line := logs.String(); !strings.Contains(line, " desk=host run=- ") {
		t.Fatalf("(5) host route line = %q", line)
	}
}

// A guest can claim any desk. The gateway trusts the claim for nothing: it
// changes the log line only. Failure list: a forged desk or run header
// changes (1) who is authorised, (2) what is sent to the provider (body,
// headers, cache key, routing), (3) the status of the answer.
func TestForgedDeskHeaderChangesOnlyTheLogLine(t *testing.T) {
	logs := captureLog(t)
	type seen struct {
		body    string
		headers http.Header
	}
	var ups []seen
	river := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ups = append(ups, seen{string(b), r.Header.Clone()})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Ok.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}
	url, token := gatewayRig(t, river, nil)
	body := `{"provider":"river","model":"` + riverStreamModel + `","stream":true,"cache_key":"k1","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	forged := map[string]string{"X-Choir-Desk": "management", "X-Choir-Run": "someone-elses-run"}
	postInferenceWith(t, url, token, body, nil)
	postInferenceWith(t, url, token, body, forged)
	if len(ups) != 2 {
		t.Fatalf("upstream calls = %d, want 2", len(ups))
	}
	if ups[0].body != ups[1].body {
		t.Fatalf("(2) a desk header changed the request sent to the provider:\n%s\n%s", ups[0].body, ups[1].body)
	}
	for k := range ups[1].headers {
		if strings.HasPrefix(strings.ToLower(k), "x-choir") {
			t.Fatalf("(2) the provider was sent %s", k)
		}
	}
	// (1) a forged desk does not authorise: no or a wrong token still fails.
	for _, tok := range []string{"", "not-a-token"} {
		req, _ := http.NewRequest(http.MethodPost, url+"/provider/v1/inference", strings.NewReader(body))
		if tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		req.Header.Set("X-Choir-Desk", "management")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("(1) token %q with a forged desk answered %d", tok, resp.StatusCode)
		}
	}
	if !strings.Contains(logs.String(), "desk=management run=someone-elses-run") {
		t.Fatalf("the claim must appear in the log line:\n%s", logs.String())
	}
}

func TestOpenAICompatibleLineCarriesTheDeskAndRun(t *testing.T) {
	logs := captureLog(t)
	reg := NewIdentityRegistry(time.Hour)
	mp := provider.NewMultiProvider()
	mp.Register("chatgpt", &mockProvider{name: "chatgpt", real: true, response: &provider.LLMResponse{Text: "x", Model: "gpt-6-luna", StopReason: "end_turn", ProviderName: "chatgpt"}})
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-openai")
	for _, hdr := range []map[string]string{{"X-Choir-Desk": "autoradio", "X-Choir-Run": "r8"}, nil} {
		req := httptest.NewRequest(http.MethodPost, "/provider/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-6-luna","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		h.HandleOpenAIChatCompletions(httptest.NewRecorder(), req)
	}
	out := logs.String()
	if !strings.Contains(out, " stream=false desk=autoradio run=r8)") || !strings.Contains(out, " stream=false desk=unknown run=-)") {
		t.Fatalf("openai-compatible line lacks desk and run:\n%s", out)
	}
}

// Seat 2's findings on the first review (first design F0). Failure list,
// written before the fixes:
//
//	A. a forged desk header on the Anthropic relay reaches the upstream request
//	   or changes more than the log; a relay failure line (redirect) names no
//	   desk or run.
//	B. the same on the OpenAI-compatible route: the typed provider request
//	   differs with a forged header.
//	C. a guest's model, tool_choice or reasoning_effort with spaces, "=" and a
//	   newline writes a second field or a second line: a forged desk=, or a
//	   forged "provider= model=" the spend report reads.
func TestAnthropicRelayDeskHeaderIsLoggedAndNeverForwarded(t *testing.T) {
	logs := captureLog(t)
	rh := newRelayHarness(t)
	forged := func(r *http.Request) {
		r.Header.Set("X-Choir-Desk", "management")
		r.Header.Set("X-Choir-Run", "run-relay")
	}
	rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, forged)
	seen := rh.requests()
	if len(seen) != 2 || string(seen[0].Body) != string(seen[1].Body) {
		t.Fatalf("(A) a desk header changed the relayed body: %d requests", len(seen))
	}
	for k := range seen[1].Header {
		if strings.HasPrefix(strings.ToLower(k), "x-choir") {
			t.Fatalf("(A) the upstream was sent %s", k)
		}
	}
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		http.Redirect(w, r, "http://127.0.0.1:1/x", http.StatusTemporaryRedirect)
	})
	rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, forged)
	out := logs.String()
	if !strings.Contains(out, "with a redirect; not followed desk=management run=run-relay") {
		t.Fatalf("(A) the failure line names no desk or run:\n%s", out)
	}
	if !strings.Contains(out, "dur=") || !strings.Contains(out, " desk=management run=run-relay") {
		t.Fatalf("(A) the success line names no desk or run:\n%s", out)
	}
	if !strings.Contains(out, " desk=unknown run=-") {
		t.Fatalf("(A) an unattributed relay call must say desk=unknown run=-:\n%s", out)
	}
}

func TestOpenAICompatibleForgedDeskChangesOnlyTheLogLine(t *testing.T) {
	reg := NewIdentityRegistry(time.Hour)
	mp := provider.NewMultiProvider()
	mock := &mockProvider{name: "chatgpt", real: true, response: &provider.LLMResponse{Text: "x", Model: "gpt-6-luna", StopReason: "end_turn", ProviderName: "chatgpt"}}
	mp.Register("chatgpt", mock)
	h := NewMultiHandlerWithRateLimit(reg, mp, nil)
	cred, _ := reg.IssueCredential("autoputer-openai")
	var reqs []provider.LLMRequest
	for _, hdr := range []map[string]string{nil, {"X-Choir-Desk": "management", "X-Choir-Run": "r"}} {
		req := httptest.NewRequest(http.MethodPost, "/provider/openai/v1/chat/completions", strings.NewReader(`{"model":"gpt-6-luna","prompt_cache_key":"k","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.HandleOpenAIChatCompletions(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("(B) status %d", w.Code)
		}
		reqs = append(reqs, *mock.lastReq)
	}
	if !reflect.DeepEqual(reqs[0], reqs[1]) {
		t.Fatalf("(B) a desk header changed the provider request:\n%+v\n%+v", reqs[0], reqs[1])
	}
}

func TestGuestStringsCannotForgeAModelCallLine(t *testing.T) {
	logs := captureLog(t)
	river := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"r\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Ok.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}
	url, token := gatewayRig(t, river, nil)
	hostile := `x desk=texture run=fake stream=false\n2026/11/22 05:00:00 gateway: inference request from autoputer vm-x (provider=devin model=swe-2-flash desk=management`
	body := `{"provider":"river","model":"` + riverStreamModel + `","stream":true,"tool_choice":"` + hostile + `","reasoning_effort":"` + hostile + `","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	postInferenceWith(t, url, token, body, map[string]string{"X-Choir-Run": "run-true", "X-Choir-Desk": "management"})
	// A hostile model name fails resolution: its line must be one line too.
	bad := `{"provider":"river","model":"m desk=fake\n2026/11/22 05:00:00 gateway: jev call computer=x desk=fake","stream":true,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
	postInferenceWith(t, url, token, bad, nil)
	out := logs.String()
	var inf []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "inference request from") {
			inf = append(inf, l)
		}
		if regexp.MustCompile(`^\S+ \S+ gateway: jev call`).MatchString(l) {
			t.Fatalf("(C) a guest string forged a jev call line: %q", l)
		}
		if strings.HasPrefix(l, "2026/11/22 05:00:00") {
			t.Fatalf("(C) a guest string started a forged log line: %q", l)
		}
	}
	if len(inf) != 1 {
		t.Fatalf("(C) %d inference lines, want 1:\n%s", len(inf), out)
	}
	m := regexp.MustCompile(` tool_choice=(\S+) .* reasoning=(\S+) stream=true desk=(\S+) run=(\S+) `).FindStringSubmatch(inf[0])
	if m == nil || m[3] != "management" || m[4] != "run-true" {
		t.Fatalf("(C) the true desk and run do not parse from the tail: %v in %q", m, inf[0])
	}
	if strings.Contains(m[1], " ") || len(m[1]) > 96 || len(m[2]) > 96 {
		t.Fatalf("(C) tool_choice or reasoning not cleaned: %q %q", m[1], m[2])
	}
	if n := strings.Count(out, "provider resolution failed for autoputer"); n != 1 {
		t.Fatalf("(C) resolution failure lines = %d:\n%s", n, out)
	}
}

// Seat 2's second review: every relay line names the desk and run, and a
// hostile string cannot start a line on the relay.
func TestAnthropicRelayFailureLinesNameTheDeskAndRun(t *testing.T) {
	forged := func(r *http.Request) {
		r.Header.Set("X-Choir-Desk", "texture")
		r.Header.Set("X-Choir-Run", "run-f")
	}
	// upstream answered 4xx
	logs := captureLog(t)
	rh := newRelayHarness(t)
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) { w.WriteHeader(http.StatusBadRequest) })
	rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, forged)
	if !strings.Contains(logs.String(), "status=400") || !strings.Contains(logs.String(), " desk=texture run=run-f") {
		t.Fatalf("(4xx line) no desk or run:\n%s", logs.String())
	}
	// upstream call failed (the upstream is gone)
	logs.Reset()
	rh.upstream.Close()
	rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, forged)
	if !strings.Contains(logs.String(), "upstream call failed:") || !strings.Contains(logs.String(), " desk=texture run=run-f") {
		t.Fatalf("(upstream failed line) no desk or run:\n%s", logs.String())
	}
}

func TestAnthropicRelayClientWentAwayLineNamesTheDeskAndRun(t *testing.T) {
	logs := captureLog(t)
	rh := newRelayHarness(t)
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/provider/anthropic/v1/messages", strings.NewReader(relayMessageBody(relayTestModel))).WithContext(ctx)
	req.RemoteAddr = peerA + ":45678"
	req.Header.Set("Authorization", "Bearer "+rh.bearer)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Choir-Desk", "texture")
	req.Header.Set("X-Choir-Run", "run-f")
	done := make(chan struct{})
	go func() {
		rh.h.HandleAnthropicRelay(httptest.NewRecorder(), req)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
	if !strings.Contains(logs.String(), "client went away") || !strings.Contains(logs.String(), " desk=texture run=run-f") {
		t.Fatalf("(client went away line) no desk or run:\n%s", logs.String())
	}
}

func TestAnthropicRelayHostileModelNameStaysOneLine(t *testing.T) {
	logs := captureLog(t)
	rh := newRelayHarness(t)
	hostile := "claude-x desk=fake\n2026/11/22 05:00:00 gateway: inference succeeded for autoputer vm-forged (provider=bedrock tokens=9+9 cached=0)"
	rh.relay.models[hostile] = true
	rh.do(http.MethodPost, "/provider/anthropic/v1/messages", `{"model":`+quote(hostile)+`,"max_tokens":64,"stream":false,"messages":[{"role":"user","content":"hi"}]}`, peerA, nil)
	if !strings.Contains(logs.String(), "anthropic relay") || !strings.Contains(logs.String(), "credential=") {
		t.Fatalf("the relay call did not reach its success line:\n%s", logs.String())
	}
	for _, l := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if strings.HasPrefix(l, "2026/11/22 05:00:00") || regexp.MustCompile(`^\S+ \S+ gateway: inference succeeded`).MatchString(l) {
			t.Fatalf("a hostile model started a line: %q", l)
		}
		if strings.Contains(l, " desk=fake") {
			t.Fatalf("a hostile model wrote a field: %q", l)
		}
	}
}

// A resolution failure quotes the guest's model; the model route's body cap is
// large, so the line must be bounded (seat 2, third review).
func TestResolutionFailureLinesAreLengthBounded(t *testing.T) {
	logs := captureLog(t)
	url, token := gatewayRig(t, func(w http.ResponseWriter, r *http.Request) {}, nil)
	huge := strings.Repeat("m", 20000)
	postInferenceWith(t, url, token, `{"provider":"river","model":"`+huge+`","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`, nil)
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, "provider resolution failed") && len(l) > 700 {
			t.Fatalf("a resolution failure line is %d bytes", len(l))
		}
	}
	if !strings.Contains(logs.String(), "provider resolution failed") {
		t.Fatalf("no resolution failure line:\n%.300s", logs.String())
	}
}
