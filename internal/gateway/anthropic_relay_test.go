package gateway

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// SEC-08, Option K, gateway half: the gateway holds a Claude Console API key
// and relays the delegate's Messages API calls, so the guest never carries it
// (docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md).
// Every key in these tests is fake; the upstream is a local test server.
// Ways the relay can fail, written before the code (numbering follows the
// problem record):
//  1. a caller-supplied credential or cookie reaches the upstream, or the
//     gateway bearer does (TestAnthropicRelayForwardsOnlyTheGatewayKey);
//  2. the relay is open: an unlisted identity, an expired window, an
//     unauthenticated call or a bearer from another address is served
//     (TestAnthropicRelayRefusesWhoIsNotAllowed);
//  3. the relay is a general Anthropic proxy: other paths, methods, models,
//     query strings (TestAnthropicRelayOnlyServesTheDelegatesCalls);
//  4. the key reaches a log, an error body or a response header
//     (TestAnthropicRelayNeverLeaksTheKey);
//  5. the window ends and the relay keeps serving (in ...RefusesWhoIsNotAllowed);
//  6. a streamed response is buffered or cut
//     (TestAnthropicRelayStreamsEventsAsTheyArrive);
//  7. an oversize body, a hung upstream or a dropped client pins the gateway
//     (TestAnthropicRelayBoundsBodiesAndInflight,
//     TestAnthropicRelayClientDisconnectCancelsTheUpstream);
//  8. a request is relayed before authentication (...RefusesWhoIsNotAllowed);
//  9. no key is configured and the call falls back to something else
//     (TestAnthropicRelayIsDarkWithoutAKey);
//  10. an upstream redirect carries the key elsewhere
//     (TestAnthropicRelayDoesNotFollowRedirects).

const (
	relayTestKey      = "sk-ant-api03-FAKE-relay-test-key-0123456789abcdef"
	relayTestIdentity = "vm-relay-test"
	relayTestModel    = "claude-sonnet-5-5"
)

type relayHarness struct {
	t        *testing.T
	h        *Handler
	reg      *IdentityRegistry
	relay    *AnthropicRelay
	upstream *httptest.Server
	bearer   string
	// onUpstream handles each upstream request; it defaults to a small JSON
	// message with usage.
	onUpstream func(w http.ResponseWriter, r *http.Request, body []byte)
	mu         sync.Mutex
	seen       []seenRequest
}

type seenRequest struct {
	Path   string
	Query  string
	Header http.Header
	Body   []byte
}

func newRelayHarness(t *testing.T) *relayHarness {
	t.Helper()
	rh := &relayHarness{t: t}
	rh.upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rh.mu.Lock()
		rh.seen = append(rh.seen, seenRequest{Path: r.URL.Path, Query: r.URL.RawQuery, Header: r.Header.Clone(), Body: body})
		handler := rh.onUpstream
		rh.mu.Unlock()
		if handler != nil {
			handler(w, r, body)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","model":"` + relayTestModel + `","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":11,"output_tokens":7,"cache_read_input_tokens":3}}`))
	}))
	t.Cleanup(rh.upstream.Close)

	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(BindingEnforce)
	cred, err := reg.IssueCredential(relayTestIdentity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA); err != nil {
		t.Fatal(err)
	}
	rh.h, rh.reg, rh.bearer = h, reg, cred.RawToken
	rh.relay = &AnthropicRelay{
		key:         relayTestKey,
		identities:  map[string]bool{relayTestIdentity: true},
		expires:     time.Now().Add(time.Hour),
		models:      map[string]bool{relayTestModel: true},
		baseURL:     rh.upstream.URL,
		client:      rh.upstream.Client(),
		maxBody:     1 << 20,
		maxInflight: 2,
		now:         time.Now,
	}
	h.SetAnthropicRelay(rh.relay)
	return rh
}

func (rh *relayHarness) setUpstream(fn func(w http.ResponseWriter, r *http.Request, body []byte)) {
	rh.mu.Lock()
	rh.onUpstream = fn
	rh.mu.Unlock()
}

func (rh *relayHarness) requests() []seenRequest {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	return append([]seenRequest(nil), rh.seen...)
}

func relayMessageBody(model string) string {
	return `{"model":"` + model + `","max_tokens":64,"stream":false,"messages":[{"role":"user","content":"hi"}]}`
}

// do sends one request straight into the handler from the given peer.
func (rh *relayHarness) do(method, path, body, peer string, mutate func(*http.Request)) *httptest.ResponseRecorder {
	rh.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = peer + ":45678"
	req.Header.Set("Authorization", "Bearer "+rh.bearer)
	req.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(req)
	}
	w := httptest.NewRecorder()
	rh.h.HandleAnthropicRelay(w, req)
	return w
}

func TestAnthropicRelayForwardsOnlyTheGatewayKey(t *testing.T) {
	rh := newRelayHarness(t)
	body := relayMessageBody(relayTestModel)
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages?beta=true", body, peerA, func(r *http.Request) {
		r.Header.Set("X-Api-Key", "sk-ant-the-callers-own-key")
		r.Header.Set("Cookie", "session=abc")
		r.Header.Set("X-Forwarded-For", "203.0.113.9")
		r.Header.Set("Anthropic-Version", "2023-06-01")
		r.Header.Set("Anthropic-Beta", "fine-grained-tool-streaming-2025-05-14")
		r.Header.Set("X-Stainless-Os", "Linux")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	seen := rh.requests()
	if len(seen) != 1 {
		t.Fatalf("upstream saw %d requests, want 1", len(seen))
	}
	got := seen[0]
	if got.Path != "/v1/messages" || got.Query != "beta=true" {
		t.Fatalf("upstream path/query = %q %q", got.Path, got.Query)
	}
	if got.Header.Get("X-Api-Key") != relayTestKey {
		t.Fatalf("upstream key = %q, want the gateway-held key", got.Header.Get("X-Api-Key"))
	}
	for _, name := range []string{"Authorization", "Cookie", "X-Forwarded-For", "X-Stainless-Os"} {
		if v := got.Header.Get(name); v != "" {
			t.Fatalf("upstream received %s: %q", name, v)
		}
	}
	for k, vs := range got.Header {
		for _, v := range vs {
			if strings.Contains(v, rh.bearer) || strings.Contains(v, "the-callers-own-key") {
				t.Fatalf("header %s carries a caller credential", k)
			}
		}
	}
	if got.Header.Get("Anthropic-Version") != "2023-06-01" || got.Header.Get("Anthropic-Beta") != "fine-grained-tool-streaming-2025-05-14" {
		t.Fatalf("anthropic-version/beta not forwarded: %v", got.Header)
	}
	if string(got.Body) != body {
		t.Fatalf("upstream body changed: %q", got.Body)
	}
	if got.Header.Get("Anthropic-Version") == "" {
		t.Fatal("no anthropic-version")
	}
}

func TestAnthropicRelayDefaultsTheAPIVersion(t *testing.T) {
	rh := newRelayHarness(t)
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if v := rh.requests()[0].Header.Get("Anthropic-Version"); v != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want the default 2023-06-01", v)
	}
}

func TestAnthropicRelayRefusesWhoIsNotAllowed(t *testing.T) {
	rh := newRelayHarness(t)
	body := relayMessageBody(relayTestModel)
	assertNothingRelayed := func() {
		t.Helper()
		if n := len(rh.requests()); n != 0 {
			t.Fatalf("%d requests reached the upstream", n)
		}
	}

	// Unauthenticated.
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerA, func(r *http.Request) { r.Header.Del("Authorization") })
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", w.Code)
	}
	// A bearer from another computer's address.
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerB, nil); w.Code != http.StatusTooManyRequests {
		t.Fatalf("bearer from another address: %d", w.Code)
	}
	assertNothingRelayed()

	// An identity that is not on the allowlist.
	other, _ := rh.reg.IssueCredential("vm-someone-else")
	if _, err := rh.reg.EnsureCredentialForPeer(other.RawToken, peerB); err != nil {
		t.Fatal(err)
	}
	w = rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerB, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+other.RawToken)
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("identity not on the allowlist: %d", w.Code)
	}
	assertNothingRelayed()

	// The window closed.
	rh.relay.expires = time.Now().Add(-time.Second)
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerA, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expired window: %d", w.Code)
	}
	assertNothingRelayed()

	// No window at all.
	rh.relay.expires = time.Time{}
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerA, nil); w.Code != http.StatusForbidden {
		t.Fatalf("no window: %d", w.Code)
	}
	assertNothingRelayed()

	// The same identity inside its window is served.
	rh.relay.expires = time.Now().Add(time.Hour)
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("allowed identity inside the window: %d %s", w.Code, w.Body.String())
	}
}

func TestAnthropicRelayIsDarkWithoutAKey(t *testing.T) {
	rh := newRelayHarness(t)
	rh.relay.key = ""
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("no key: %d, want 503", w.Code)
	}
	if n := len(rh.requests()); n != 0 {
		t.Fatalf("a keyless relay reached the upstream %d times", n)
	}

	// A handler nobody configured has no relay at all.
	h, reg, _ := setupHandler(t)
	cred, _ := reg.IssueCredential(relayTestIdentity)
	req := httptest.NewRequest(http.MethodPost, "/provider/anthropic/v1/messages", strings.NewReader(relayMessageBody(relayTestModel)))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	rec := httptest.NewRecorder()
	h.HandleAnthropicRelay(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured handler: %d, want 503", rec.Code)
	}
}

func TestAnthropicRelayOnlyServesTheDelegatesCalls(t *testing.T) {
	rh := newRelayHarness(t)
	body := relayMessageBody(relayTestModel)
	for _, tc := range []struct {
		name, method, path string
		want               int
	}{
		{"batches", http.MethodPost, "/provider/anthropic/v1/messages/batches", http.StatusNotFound},
		{"files", http.MethodPost, "/provider/anthropic/v1/files", http.StatusNotFound},
		{"legacy complete", http.MethodPost, "/provider/anthropic/v1/complete", http.StatusNotFound},
		{"organizations", http.MethodGet, "/provider/anthropic/v1/organizations", http.StatusNotFound},
		{"traversal", http.MethodPost, "/provider/anthropic/v1/messages/../files", http.StatusNotFound},
		{"encoded traversal", http.MethodPost, "/provider/anthropic/v1/messages%2f..%2ffiles", http.StatusNotFound},
		{"no version", http.MethodPost, "/provider/anthropic/messages", http.StatusNotFound},
		{"get on messages", http.MethodGet, "/provider/anthropic/v1/messages", http.StatusMethodNotAllowed},
		{"post on models", http.MethodPost, "/provider/anthropic/v1/models", http.StatusMethodNotAllowed},
		{"delete on messages", http.MethodDelete, "/provider/anthropic/v1/messages", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := rh.do(tc.method, tc.path, body, peerA, nil)
			if w.Code != tc.want {
				t.Fatalf("%s %s: %d, want %d", tc.method, tc.path, w.Code, tc.want)
			}
		})
	}
	if n := len(rh.requests()); n != 0 {
		t.Fatalf("%d refused requests reached the upstream", n)
	}

	// The served set.
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages/count_tokens", body, peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("count_tokens: %d", w.Code)
	}
	if w := rh.do(http.MethodGet, "/provider/anthropic/v1/models", "", peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("models: %d", w.Code)
	}

	// Models: the relay serves the delegate's model only.
	for _, bad := range []struct{ body, why string }{
		{relayMessageBody("claude-opus-5-5"), "another model"},
		{relayMessageBody(""), "no model"},
		{`{"messages":[]}`, "no model field"},
		{`not json`, "not json"},
		{`[]`, "not an object"},
	} {
		w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", bad.body, peerA, nil)
		if w.Code != http.StatusForbidden && w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d, want 403 or 400", bad.why, w.Code)
		}
	}
	// The model check must read the same model the upstream will. A body whose
	// "model" is ambiguous (repeated, or spelled in another case, which Go's
	// decoder would also match) is refused, never guessed at.
	for _, ambiguous := range []string{
		`{"model":"` + relayTestModel + `","model":"claude-opus-5-5","messages":[]}`,
		`{"model":"claude-opus-5-5","model":"` + relayTestModel + `","messages":[]}`,
		`{"Model":"claude-opus-5-5","model":"` + relayTestModel + `","messages":[]}`,
		`{"MODEL":"` + relayTestModel + `","messages":[]}`,
		`{"model":["` + relayTestModel + `"],"messages":[]}`,
		`{"model":{"id":"` + relayTestModel + `"},"messages":[]}`,
		`{"mod\u0065l":"claude-opus-5-5","model":"` + relayTestModel + `","messages":[]}`,
	} {
		if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", ambiguous, peerA, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("ambiguous model %s: %d, want 400", ambiguous, w.Code)
		}
	}
	// A "model" nested deeper is not the request's model and is harmless.
	nested := `{"model":"` + relayTestModel + `","metadata":{"model":"claude-opus-5-5"},"messages":[]}`
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", nested, peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("nested model key: %d", w.Code)
	}
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", body, peerA, func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") }); w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("non-JSON content type: %d", w.Code)
	}

	// Query: beta=true passes, nothing else does.
	rh.mu.Lock()
	rh.seen = nil
	rh.mu.Unlock()
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages?beta=true&evil=1&x-api-key=k", body, peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("query: %d", w.Code)
	}
	if q := rh.requests()[0].Query; q != "beta=true" {
		t.Fatalf("upstream query = %q, want only beta=true", q)
	}
}

func TestAnthropicRelayNeverLeaksTheKey(t *testing.T) {
	rh := newRelayHarness(t)
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)

	// An upstream error that echoes the key and a key-shaped string.
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "sid=upstream")
		w.Header().Set("Server", "upstream-edge")
		w.Header().Set("Request-Id", "req_123")
		w.Header().Set("Anthropic-Ratelimit-Requests-Remaining", "49")
		w.Header().Set("X-Echo-Key", r.Header.Get("X-Api-Key"))
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key ` + relayTestKey + ` also sk-ant-api03-OTHER-secret-value"}}`))
	})
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("the upstream status must pass through: %d", w.Code)
	}
	out := w.Body.String()
	if strings.Contains(out, relayTestKey) || strings.Contains(out, "OTHER-secret") {
		t.Fatalf("an error body carries a key: %s", out)
	}
	if !strings.Contains(out, "authentication_error") {
		t.Fatalf("the error type must survive redaction: %s", out)
	}
	for _, name := range []string{"Set-Cookie", "Server", "X-Echo-Key"} {
		if v := w.Header().Get(name); v != "" {
			t.Fatalf("response header %s passed through: %q", name, v)
		}
	}
	if w.Header().Get("Request-Id") != "req_123" || w.Header().Get("Anthropic-Ratelimit-Requests-Remaining") != "49" {
		t.Fatalf("the diagnostic headers must pass: %v", w.Header())
	}
	// Error paths of every kind.
	rh.relay.key = ""
	_ = rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	rh.relay.key = relayTestKey
	_ = rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody("claude-opus-5-5"), peerA, nil)
	if strings.Contains(logs.String(), relayTestKey) || strings.Contains(logs.String(), "OTHER-secret") {
		t.Fatalf("the key reached the log:\n%s", logs.String())
	}
	if strings.Contains(logs.String(), rh.bearer[strings.Index(rh.bearer, ":")+1:]) {
		t.Fatal("the bearer reached the log")
	}
	if !strings.Contains(logs.String(), "anthropic relay") {
		t.Fatalf("a relayed call left no log line:\n%s", logs.String())
	}
}

func TestAnthropicRelayDoesNotFollowRedirects(t *testing.T) {
	rh := newRelayHarness(t)
	var elsewhereHit bool
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhereHit = true
	}))
	defer elsewhere.Close()
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		http.Redirect(w, r, elsewhere.URL+"/steal", http.StatusTemporaryRedirect)
	})
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if elsewhereHit {
		t.Fatal("the relay followed a redirect with the key")
	}
	if w.Code == http.StatusOK {
		t.Fatalf("a redirect was relayed as success")
	}
}

// TestAnthropicRelayStreamsEventsAsTheyArrive runs the handler behind a real
// HTTP server: the first event must reach the client while the upstream is
// still holding the second.
func TestAnthropicRelayStreamsEventsAsTheyArrive(t *testing.T) {
	rh := newRelayHarness(t)
	release := make(chan struct{})
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		fl := w.(http.Flusher)
		_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":25,\"cache_read_input_tokens\":4,\"output_tokens\":1}}}\n\n")
		fl.Flush()
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			return
		}
		_, _ = io.WriteString(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":42}}\n\n")
		_, _ = io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
		fl.Flush()
	})
	gw := httptest.NewServer(http.HandlerFunc(rh.h.HandleAnthropicRelay))
	defer gw.Close()

	req, _ := http.NewRequest(http.MethodPost, gw.URL+"/provider/anthropic/v1/messages", strings.NewReader(`{"model":"`+relayTestModel+`","max_tokens":8,"stream":true,"messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+rh.bearer)
	req.Header.Set("Content-Type", "application/json")
	// The client is on loopback, not peerA; binding is covered elsewhere.
	rh.h.SetBearerBinding(BindingOff)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type %q", ct)
	}
	br := bufio.NewReader(resp.Body)
	first := make(chan string, 1)
	go func() {
		var sb strings.Builder
		for {
			line, err := br.ReadString('\n')
			sb.WriteString(line)
			if err != nil || line == "\n" {
				break
			}
		}
		first <- sb.String()
	}()
	select {
	case ev := <-first:
		if !strings.Contains(ev, "message_start") {
			t.Fatalf("first event = %q", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the first event did not reach the client while the upstream held the stream: the relay buffers")
	}
	close(release)
	rest, _ := io.ReadAll(br)
	if !strings.Contains(string(rest), "message_stop") {
		t.Fatalf("the stream was cut: %q", rest)
	}
	// Usage is read from the events for the spend record.
	deadline := time.Now().Add(2 * time.Second)
	for {
		s := rh.relay.usageSnapshot()
		if s.Requests == 1 && s.InputTokens == 25 && s.OutputTokens == 42 && s.CachedInputTokens == 4 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("usage = %+v, want 1 request, 25 in, 42 out, 4 cached", s)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAnthropicRelayReadsUsageFromAJSONReply(t *testing.T) {
	rh := newRelayHarness(t)
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if s := rh.relay.usageSnapshot(); s.Requests != 1 || s.InputTokens != 11 || s.OutputTokens != 7 || s.CachedInputTokens != 3 {
		t.Fatalf("usage = %+v", s)
	}
}

func TestAnthropicRelayBoundsBodiesAndInflight(t *testing.T) {
	rh := newRelayHarness(t)
	rh.relay.maxBody = 256
	big := `{"model":"` + relayTestModel + `","pad":"` + strings.Repeat("x", 1024) + `"}`
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", big, peerA, nil); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize body: %d, want 413", w.Code)
	}
	if n := len(rh.requests()); n != 0 {
		t.Fatalf("an oversize body reached the upstream")
	}
	rh.relay.maxBody = 1 << 20

	// In-flight cap per identity: hold two, the third waits for nothing and is
	// told to come back.
	hold := make(chan struct{})
	started := make(chan struct{}, 4)
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		started <- struct{}{}
		select {
		case <-hold:
		case <-r.Context().Done():
		case <-time.After(5 * time.Second): // a failing test must not hang its cleanup
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1}}`))
	})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("upstream calls did not start")
		}
	}
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("third concurrent call: %d retry-after=%q, want 429 with Retry-After", w.Code, w.Header().Get("Retry-After"))
	}
	close(hold)
	wg.Wait()
	// Slots come back.
	rh.setUpstream(nil)
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("after the held calls finished: %d (a slot leaked)", w.Code)
	}
}

func TestAnthropicRelayClientDisconnectCancelsTheUpstream(t *testing.T) {
	rh := newRelayHarness(t)
	rh.relay.maxInflight = 1
	cancelled := make(chan struct{})
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		select {
		case <-r.Context().Done():
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/provider/anthropic/v1/messages", strings.NewReader(relayMessageBody(relayTestModel))).WithContext(ctx)
	req.RemoteAddr = peerA + ":45678"
	req.Header.Set("Authorization", "Bearer "+rh.bearer)
	req.Header.Set("Content-Type", "application/json")
	done := make(chan struct{})
	go func() {
		rh.h.HandleAnthropicRelay(httptest.NewRecorder(), req)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case <-cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("a client that went away left the upstream call running")
	}
	<-done
	// The cancelled call released its slot.
	rh.setUpstream(nil)
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("after a cancelled call: %d (a slot leaked)", w.Code)
	}
}

func TestAnthropicRelayConfigFromEnvFailsClosed(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if r := newAnthropicRelayFromGetenv(get(nil)); r.status() != "off" {
		t.Fatalf("nothing set: %q, want off", r.status())
	}
	base := map[string]string{
		"ANTHROPIC_API_KEY":                  relayTestKey,
		"GATEWAY_ANTHROPIC_RELAY_IDENTITIES": "vm-a, vm-b\nvm-c",
		"GATEWAY_ANTHROPIC_RELAY_EXPIRES":    "2999-01-01",
	}
	r := newAnthropicRelayFromGetenv(get(base))
	if r.status() != "on" || !r.identities["vm-a"] || !r.identities["vm-b"] || !r.identities["vm-c"] || len(r.identities) != 3 {
		t.Fatalf("parsed relay = %q %v", r.status(), r.identities)
	}
	if !r.models[relayTestModel] || len(r.models) != 1 {
		t.Fatalf("default models = %v, want only %s", r.models, relayTestModel)
	}
	if r.baseURL != "https://api.anthropic.com" {
		t.Fatalf("base URL = %q", r.baseURL)
	}
	for name, mutate := range map[string]func(map[string]string){
		"no key":         func(m map[string]string) { delete(m, "ANTHROPIC_API_KEY") },
		"blank key":      func(m map[string]string) { m["ANTHROPIC_API_KEY"] = "  " },
		"no identities":  func(m map[string]string) { delete(m, "GATEWAY_ANTHROPIC_RELAY_IDENTITIES") },
		"no expiry":      func(m map[string]string) { delete(m, "GATEWAY_ANTHROPIC_RELAY_EXPIRES") },
		"garbled expiry": func(m map[string]string) { m["GATEWAY_ANTHROPIC_RELAY_EXPIRES"] = "next tuesday" },
		"past expiry":    func(m map[string]string) { m["GATEWAY_ANTHROPIC_RELAY_EXPIRES"] = "2001-01-01" },
	} {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		mutate(m)
		if st := newAnthropicRelayFromGetenv(get(m)).status(); st == "on" {
			t.Fatalf("%s: relay is on", name)
		}
	}
	// The base URL is not configurable from the environment.
	m := map[string]string{"ANTHROPIC_BASE_URL": "https://evil.example", "GATEWAY_ANTHROPIC_RELAY_BASE_URL": "https://evil.example"}
	for k, v := range base {
		m[k] = v
	}
	if got := newAnthropicRelayFromGetenv(get(m)).baseURL; got != "https://api.anthropic.com" {
		t.Fatalf("the environment moved the upstream to %q", got)
	}
}

func TestAnthropicRelayHealthNamesNoKeyAndNoIdentity(t *testing.T) {
	rh := newRelayHarness(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	rh.h.HandleHealth(w, req)
	body := w.Body.String()
	if !strings.Contains(body, `"anthropic_relay":"on"`) {
		t.Fatalf("health does not report the relay: %s", body)
	}
	if strings.Contains(body, relayTestKey) || strings.Contains(body, relayTestIdentity) {
		t.Fatalf("health names the key or an identity: %s", body)
	}
}
