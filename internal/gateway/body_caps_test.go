package gateway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// B7: the gateway decoded guest request bodies with no size limit
// (docs/problems/gateway-request-bodies-unbounded-2026-10-22.md). Ways the
// repair can fail, written before the code (numbering follows the record):
//  1. a limit refuses a real model call
//     (TestBodyCapsPassALegitimateLargeModelBody, ...ObserveModeCountsButServes);
//  2. the oversized body is read whole before the refusal, or a declared
//     Content-Length over the limit is read at all
//     (TestBodyCapsRefuseAnOversizedBodyOnEveryRoute);
//  3. the refusal is a 400 or a dropped connection (same test, every route);
//  4. bodies pile up behind the scheduler (TestModelSchedulerQueuedCallReadsNoBody);
//  5. a cancelled caller is handed a slot
//     (TestModelSchedulerRefusesAnAlreadyCancelledCaller);
//  6. the limits are invisible or fixed in code
//     (TestBodyCapsEnvOverridesAndHealth);
//  7. the scheduler path reads the body somewhere the cap does not cover
//     (TestModelSchedulerQueuedCallReadsNoBody runs with the cap on).

const bodyCapsHostToken = "body-caps-test-token-0123456789abcdef0123456789"

// countingBody is a finite JSON body of about size bytes that counts what
// the handler read from it. It is finite so a gateway without a cap reads it
// to the end and answers, and the test fails instead of hanging.
type countingBody struct {
	r    io.Reader
	read atomic.Int64
}

func newCountingBody(size int) *countingBody {
	prefix := `{"x":"`
	suffix := `"}`
	n := size - len(prefix) - len(suffix)
	if n < 0 {
		n = 0
	}
	return &countingBody{r: io.MultiReader(strings.NewReader(prefix), io.LimitReader(repeatByte('a'), int64(n)), strings.NewReader(suffix))}
}

func (c *countingBody) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.read.Add(int64(n))
	return n, err
}

type repeatByte byte

func (b repeatByte) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}

// testBodyLimits are small limits so the oversize cases run fast.
const testBodyLimits = "model=64KiB,model_ceiling=256KiB,search=8KiB,source_api=4KiB,judgment=8KiB,operator=1KiB"

type bodyRouteCase struct {
	name    string
	path    string
	limit   int
	host    bool // host-only route: loopback, marker and the internal credential
	serve   func(h *Handler) http.HandlerFunc
	prepare func(t *testing.T, h *Handler, reg *IdentityRegistry, req *http.Request)
}

func bearerFor(computer string) func(t *testing.T, h *Handler, reg *IdentityRegistry, req *http.Request) {
	return func(t *testing.T, h *Handler, reg *IdentityRegistry, req *http.Request) {
		t.Helper()
		cred, err := reg.IssueCredential(computer)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	}
}

func bodyRouteCases() []bodyRouteCase {
	return []bodyRouteCase{
		{name: "inference", path: "/provider/v1/inference", limit: 64 << 10,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleInference }, prepare: bearerFor("computer-inference")},
		{name: "openai chat", path: "/provider/openai/v1/chat/completions", limit: 64 << 10,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleOpenAIChatCompletions }, prepare: bearerFor("computer-openai")},
		{name: "search", path: "/provider/v1/search", limit: 8 << 10,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleSearch }, prepare: bearerFor("computer-search")},
		{name: "source-api", path: "/provider/v1/source-api", limit: 4 << 10,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleSourceAPI }, prepare: bearerFor("computer-source")},
		{name: "jev judgments", path: "/provider/v1/judgments", limit: 8 << 10,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleJudgment },
			prepare: func(t *testing.T, h *Handler, reg *IdentityRegistry, req *http.Request) {
				h.SetJevTransport(&JevTransport{enabled: true, apiKey: "test-key", httpClient: &http.Client{Transport: &decisionsRoundTripper{}},
					receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "receipts.jsonl"))})
				bearerFor("computer-jev")(t, h, reg, req)
			}},
		{name: "host judgments", path: "/provider/v1/host/judgments", limit: 8 << 10, host: true,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleHostJudgment },
			prepare: func(t *testing.T, h *Handler, reg *IdentityRegistry, req *http.Request) {
				h.SetJevTransport(&JevTransport{enabled: true, apiKey: "test-key", httpClient: &http.Client{Transport: &decisionsRoundTripper{}},
					receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "receipts.jsonl"))})
				req.Header.Set(hostCallerHeader, "front-importance")
			}},
		{name: "credentials issue", path: "/provider/v1/credentials/issue", limit: 1 << 10, host: true,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleIssueCredential }},
		{name: "credentials ensure", path: "/provider/v1/credentials/ensure", limit: 1 << 10, host: true,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleEnsureCredential }},
		{name: "search health reset", path: "/provider/v1/search/health/reset", limit: 1 << 10, host: true,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleSearchHealthReset }},
		{name: "breakers reset", path: "/provider/v1/breakers/reset", limit: 1 << 10, host: true,
			serve: func(h *Handler) http.HandlerFunc { return h.HandleProviderBreakerReset },
			prepare: func(t *testing.T, h *Handler, reg *IdentityRegistry, req *http.Request) {
				h.SetBreakers(NewBreakerRegistry())
			}},
	}
}

func serveBodyRoute(t *testing.T, tc bodyRouteCase, body io.Reader, contentLength int64) *httptest.ResponseRecorder {
	t.Helper()
	h, reg, _ := setupHandler(t)
	req := httptest.NewRequest(http.MethodPost, tc.path, body)
	req.ContentLength = contentLength
	req.Header.Set("Content-Type", "application/json")
	if tc.host {
		req.RemoteAddr = "127.0.0.1:41000"
		req.Host = "localhost:8084"
		req.Header.Set(internalCallerHeader, "true")
		req.Header.Set(hostauth.Header, bodyCapsHostToken)
	}
	if tc.prepare != nil {
		tc.prepare(t, h, reg, req)
	}
	rec := httptest.NewRecorder()
	var handler http.Handler = tc.serve(h)
	if tc.host {
		handler = hostauth.Middleware(handler)
	}
	handler.ServeHTTP(rec, req)
	return rec
}

func TestBodyCapsRefuseAnOversizedBodyOnEveryRoute(t *testing.T) {
	t.Setenv("GATEWAY_BODY_CAPS", "enforce")
	t.Setenv("GATEWAY_BODY_LIMITS", testBodyLimits)
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, bodyCapsHostToken)
	defer restore()
	for _, tc := range bodyRouteCases() {
		t.Run(tc.name, func(t *testing.T) {
			size := tc.limit * 8
			// A streamed body (no Content-Length): refused once the limit is
			// passed, never read to its end.
			body := newCountingBody(size)
			rec := serveBodyRoute(t, tc, body, -1)
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("%s with a %d-byte body (limit %d) = %d %s, want 413", tc.path, size, tc.limit, rec.Code, rec.Body.String())
			}
			if got := body.read.Load(); got > int64(tc.limit)+1 {
				t.Fatalf("%s read %d bytes of an oversized body; the limit is %d", tc.path, got, tc.limit)
			}
			if !strings.Contains(strings.ToLower(rec.Body.String()), "too large") {
				t.Fatalf("%s 413 body %q gives no short reason", tc.path, rec.Body.String())
			}
			// A declared Content-Length over the limit is refused unread.
			declared := newCountingBody(size)
			rec = serveBodyRoute(t, tc, declared, int64(size))
			if rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("%s with Content-Length %d = %d, want 413", tc.path, size, rec.Code)
			}
			if got := declared.read.Load(); got != 0 {
				t.Fatalf("%s read %d bytes of a body whose Content-Length is over the limit", tc.path, got)
			}
		})
	}
}

// largeInferenceBody is a text-only model call of about size bytes.
func largeInferenceBody(t *testing.T, size int) []byte {
	t.Helper()
	body, err := json.Marshal(ProviderRequest{
		System:   strings.Repeat("s", size),
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "hello"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func inferWithBody(t *testing.T, h *Handler, reg *IdentityRegistry, computer string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	cred, err := reg.IssueCredential(computer)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	rec := httptest.NewRecorder()
	h.HandleInference(rec, req)
	return rec
}

func TestBodyCapsPassALegitimateLargeModelBody(t *testing.T) {
	// The default limits: a full 1M-token context plus one visual bundle is
	// about 48 MiB (the problem record's measure).
	t.Setenv("GATEWAY_BODY_CAPS", "enforce")
	t.Setenv("GATEWAY_BODY_LIMITS", "")
	h, reg, mp := setupHandler(t)
	size := 48 << 20
	rec := inferWithBody(t, h, reg, "computer-large", largeInferenceBody(t, size))
	if rec.Code != http.StatusOK {
		t.Fatalf("a %d-byte model call = %d %s, want 200", size, rec.Code, rec.Body.String())
	}
	mp.mu.Lock()
	got := len(mp.lastReq.System)
	mp.mu.Unlock()
	if got != size {
		t.Fatalf("the provider saw a %d-byte system prompt, want %d", got, size)
	}
}

func bodyCapsHealth(t *testing.T, h *Handler) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	caps, ok := out["body_caps"].(map[string]any)
	if !ok {
		t.Fatalf("/health has no body_caps: %s", rec.Body.String())
	}
	return caps
}

func healthNumber(t *testing.T, caps map[string]any, field, route string) int64 {
	t.Helper()
	m, ok := caps[field].(map[string]any)
	if !ok {
		t.Fatalf("body_caps.%s missing: %v", field, caps)
	}
	v, _ := m[route].(float64)
	return int64(v)
}

func TestBodyCapsObserveModeCountsButServes(t *testing.T) {
	t.Setenv("GATEWAY_BODY_CAPS", "observe")
	t.Setenv("GATEWAY_BODY_LIMITS", testBodyLimits)
	h, reg, _ := setupHandler(t)
	// Over the limit, under the ceiling: served, counted.
	rec := inferWithBody(t, h, reg, "computer-observe", largeInferenceBody(t, 128<<10))
	if rec.Code != http.StatusOK {
		t.Fatalf("observe: a body over the limit and under the ceiling = %d, want 200", rec.Code)
	}
	caps := bodyCapsHealth(t, h)
	if caps["mode"] != "observe" {
		t.Fatalf("body_caps.mode = %v, want observe", caps["mode"])
	}
	if n := healthNumber(t, caps, "would_refuse", "model"); n != 1 {
		t.Fatalf("body_caps.would_refuse.model = %d, want 1", n)
	}
	if n := healthNumber(t, caps, "max_seen", "model"); n < 128<<10 {
		t.Fatalf("body_caps.max_seen.model = %d, want at least %d", n, 128<<10)
	}
	// Over the ceiling: refused even in observe.
	rec = inferWithBody(t, h, reg, "computer-observe", largeInferenceBody(t, 512<<10))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("observe: a body over the ceiling = %d, want 413", rec.Code)
	}
	// Non-model routes enforce in every mode.
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, bodyCapsHostToken)
	defer restore()
	for _, tc := range bodyRouteCases() {
		if tc.name != "search" {
			continue
		}
		if rec := serveBodyRoute(t, tc, newCountingBody(tc.limit*8), -1); rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("observe: search over its limit = %d, want 413", rec.Code)
		}
	}
}

func TestBodyCapsEnvOverridesAndHealth(t *testing.T) {
	cases := []struct {
		mode, limits string
		wantMode     string
		wantModel    int64
		refuse128K   bool
	}{
		// F3 (bunch 88): 20 images at the 5 MiB downscale bound, as base64,
		// plus a full context is about 150 MiB.
		{mode: "", limits: "", wantMode: "enforce", wantModel: 160 << 20},
		{mode: "bogus", limits: "model=64KiB", wantMode: "enforce", wantModel: 64 << 10, refuse128K: true},
		{mode: "enforce", limits: "model=64KiB", wantMode: "enforce", wantModel: 64 << 10, refuse128K: true},
		{mode: "enforce", limits: "model=1048576", wantMode: "enforce", wantModel: 1 << 20},
		// An unreadable override keeps the default and is logged.
		{mode: "enforce", limits: "model=lots,search=-1", wantMode: "enforce", wantModel: 160 << 20},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("mode=%q limits=%q", tc.mode, tc.limits), func(t *testing.T) {
			t.Setenv("GATEWAY_BODY_CAPS", tc.mode)
			t.Setenv("GATEWAY_BODY_LIMITS", tc.limits)
			h, reg, _ := setupHandler(t)
			caps := bodyCapsHealth(t, h)
			if caps["mode"] != tc.wantMode {
				t.Fatalf("body_caps.mode = %v, want %s", caps["mode"], tc.wantMode)
			}
			if got := healthNumber(t, caps, "limits", "model"); got != tc.wantModel {
				t.Fatalf("body_caps.limits.model = %d, want %d", got, tc.wantModel)
			}
			if got := healthNumber(t, caps, "limits", "search"); got != 1<<20 && tc.limits != testBodyLimits {
				t.Fatalf("body_caps.limits.search = %d, want the 1 MiB default", got)
			}
			rec := inferWithBody(t, h, reg, "computer-env", largeInferenceBody(t, 128<<10))
			if tc.refuse128K && rec.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("a 128 KiB call under a 64 KiB limit = %d, want 413", rec.Code)
			}
			if !tc.refuse128K && rec.Code != http.StatusOK {
				t.Fatalf("a 128 KiB call = %d, want 200", rec.Code)
			}
			if tc.refuse128K {
				if n := healthNumber(t, bodyCapsHealth(t, h), "refused", "model"); n != 1 {
					t.Fatalf("body_caps.refused.model = %d, want 1", n)
				}
			}
		})
	}
}

// TestModelSchedulerQueuedCallReadsNoBody: with the scheduler on, a call
// waiting for a slot holds no body; it reads its body once it has the slot.
func TestModelSchedulerQueuedCallReadsNoBody(t *testing.T) {
	t.Setenv("GATEWAY_BODY_CAPS", "enforce")
	t.Setenv("GATEWAY_BODY_LIMITS", "")
	for _, route := range []string{"inference", "openai chat"} {
		t.Run(route, func(t *testing.T) {
			s := newTestScheduler(1, 1, 16, time.Minute)
			h, reg := schedulerHandler(t, &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "r", Text: "ok", Model: "m", StopReason: "end_turn"}}, nil, s)
			cred, _ := reg.IssueCredential("computer-q")
			hold := mustAcquire(t, s, "computer-q")

			raw := `{"model":"m","messages":[{"role":"user","content":"hi"}]}`
			path, serve := "/provider/openai/v1/chat/completions", h.HandleOpenAIChatCompletions
			if route == "inference" {
				raw = `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`
				path, serve = "/provider/v1/inference", h.HandleInference
			}
			body := &countingBody{r: strings.NewReader(raw)}
			req := httptest.NewRequest(http.MethodPost, path, body)
			req.Header.Set("Authorization", "Bearer "+cred.RawToken)
			rec := httptest.NewRecorder()
			done := make(chan struct{})
			go func() { serve(rec, req); close(done) }()

			waitUntil(t, "the call to queue", func() bool { return s.Stats().Queued == 1 })
			if got := body.read.Load(); got != 0 {
				t.Fatalf("a queued %s call read %d bytes of its body before it had a slot", route, got)
			}
			hold()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("the queued call did not finish after the slot freed")
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("the queued %s call = %d %s, want 200", route, rec.Code, rec.Body.String())
			}
		})
	}
}

// --- review round (bunch 88): F1 aggregate bound, F2/F4 body read deadline ---

// bodyServer serves h's inference route on a real listener with the given
// server read timeout, so read deadlines behave as they do in production.
func bodyServer(t *testing.T, serve http.HandlerFunc, readTimeout time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(serve)
	srv.Config.ReadTimeout = readTimeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

type postResult struct {
	code       int
	retryAfter string
	body       string
	err        error
}

func postAsync(url, token string, body io.Reader) chan postResult {
	out := make(chan postResult, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodPost, url, body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			out <- postResult{err: err}
			return
		}
		b, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		out <- postResult{code: resp.StatusCode, retryAfter: resp.Header.Get("Retry-After"), body: string(b)}
	}()
	return out
}

// F2: a call that waited in the queue longer than the server's read
// timeout still reads its body: the body's deadline starts when the slot
// is granted. Fails on 02010a2e0: the read hits the server's absolute
// deadline and the handler answers 400.
func TestBodyCapsQueuedCallReadsItsBodyAfterTheServerReadTimeout(t *testing.T) {
	t.Setenv("GATEWAY_BODY_CAPS", "enforce")
	t.Setenv("GATEWAY_BODY_LIMITS", "")
	s := newTestScheduler(1, 1, 16, time.Minute)
	h, reg := schedulerHandler(t, &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "r", Text: "ok", Model: "m", StopReason: "end_turn"}}, nil, s)
	cred, _ := reg.IssueCredential("computer-late")
	srv := bodyServer(t, h.HandleInference, 300*time.Millisecond)
	hold := mustAcquire(t, s, "computer-late")

	pr, pw := io.Pipe()
	res := postAsync(srv.URL+"/provider/v1/inference", cred.RawToken, pr)
	waitUntil(t, "the call to queue", func() bool { return s.Stats().Queued == 1 })
	time.Sleep(600 * time.Millisecond) // past the server's read timeout
	hold()
	go func() {
		_, _ = pw.Write([]byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`))
		_ = pw.Close()
	}()
	r := <-res
	if r.err != nil || r.code != http.StatusOK {
		t.Fatalf("a call granted after the server read timeout = %d %q (%v), want 200", r.code, r.body, r.err)
	}
}

// F4: a body trickled after the slot is granted is cut at the body read
// deadline and answered with a status the guest retries; the slot is
// freed. Fails on 02010a2e0: the slot is held until the server's read
// timeout (here longer than the client waits).
func TestBodyCapsTrickledBodyIsCutAtTheBodyReadDeadline(t *testing.T) {
	t.Setenv("GATEWAY_BODY_CAPS", "enforce")
	t.Setenv("GATEWAY_BODY_LIMITS", "")
	t.Setenv("GATEWAY_BODY_READ_TIMEOUT", "300ms")
	s := newTestScheduler(1, 1, 16, time.Minute)
	h, reg := schedulerHandler(t, &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "r", Text: "ok", Model: "m", StopReason: "end_turn"}}, nil, s)
	cred, _ := reg.IssueCredential("computer-trickle")
	srv := bodyServer(t, h.HandleInference, 30*time.Second)

	pr, pw := io.Pipe()
	defer pw.Close()
	// The client cannot give up while its body write is blocked, so the
	// test ends the body itself after 4 s.
	giveUp := time.AfterFunc(4*time.Second, func() { _ = pw.CloseWithError(errors.New("test gave up")) })
	defer giveUp.Stop()
	res := postAsync(srv.URL+"/provider/v1/inference", cred.RawToken, pr)
	_, _ = pw.Write([]byte(`{"messages":`)) // then nothing
	r := <-res
	if r.err != nil || !giveUp.Stop() {
		t.Fatalf("a trickled body was not answered within 4 s: %d %q (%v)", r.code, r.body, r.err)
	}
	if r.code != http.StatusServiceUnavailable || r.retryAfter == "" {
		t.Fatalf("a trickled body = %d (Retry-After %q) %q, want 503 with Retry-After (transient to the guest)", r.code, r.retryAfter, r.body)
	}
	waitUntil(t, "the slot to free", func() bool { return s.Stats().InFlight == 0 })
}

// F1: with the scheduler off, the total bytes of bodies in flight are
// bounded process-wide, in observe mode too. A body that does not fit
// waits, then is answered 429 with Retry-After, unread. Fails on
// 02010a2e0: the in_flight limit is unknown and the second body is read
// and reaches the provider.
func TestBodyCapsBoundTheBytesInFlightWithTheSchedulerOff(t *testing.T) {
	t.Setenv("GATEWAY_BODY_CAPS", "observe")
	t.Setenv("GATEWAY_BODY_LIMITS", "model=64KiB,model_ceiling=256KiB,in_flight=256KiB")
	t.Setenv("GATEWAY_BODY_READ_TIMEOUT", "300ms")
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	h, reg := schedulerHandler(t, gp, nil, nil)
	defer close(gp.gate)

	first := make(chan int, 1)
	go func() { first <- inferWithBody(t, h, reg, "computer-a", largeInferenceBody(t, 200<<10)).Code }()
	waitUntil(t, "the first call to reach the provider", func() bool { return gp.calls.Load() == 1 })

	cred, _ := reg.IssueCredential("computer-b")
	body := &countingBody{r: bytes.NewReader(largeInferenceBody(t, 200<<10))}
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", body)
	req.ContentLength = int64(len(largeInferenceBody(t, 200<<10)))
	req.Header.Set("Authorization", "Bearer "+cred.RawToken)
	rec := httptest.NewRecorder()
	second := make(chan struct{})
	go func() { h.HandleInference(rec, req); close(second) }()
	select {
	case <-second:
	case <-time.After(3 * time.Second):
		t.Fatalf("the second 200 KiB body was taken in while the first held 200 of 256 KiB (provider calls %d)", gp.calls.Load())
	}
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("a body over the bytes in flight = %d (Retry-After %q) %s, want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"), rec.Body.String())
	}
	if n := body.read.Load(); n != 0 {
		t.Fatalf("the refused body was read: %d bytes", n)
	}
	caps := bodyCapsHealth(t, h)
	if got := healthNumber(t, caps, "limits", "in_flight"); got != 256<<10 {
		t.Fatalf("body_caps.limits.in_flight = %d, want %d", got, 256<<10)
	}
	if got := healthNumber(t, caps, "refused", "in_flight"); got != 1 {
		t.Fatalf("body_caps.refused.in_flight = %d, want 1", got)
	}

	// The first call's bytes are given back when it ends.
	releaseOne(t, gp)
	if code := <-first; code != http.StatusOK {
		t.Fatalf("the first call = %d, want 200", code)
	}
	third := make(chan int, 1)
	go func() { third <- inferWithBody(t, h, reg, "computer-b", largeInferenceBody(t, 200<<10)).Code }()
	releaseOne(t, gp)
	if code := <-third; code != http.StatusOK {
		t.Fatalf("a call after the first ended = %d, want 200", code)
	}
}
