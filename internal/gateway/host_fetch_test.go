package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostfetch"
)

// Ways the gateway verb could fail, written before the code:
//  1. it serves an unauthenticated caller or a non-runtime peer;
//  2. one computer's polling exhausts another's budget, or the budget is the
//     shared inference limit (60 a minute) and starves the paper's polls;
//  3. one computer holds unbounded concurrent upstream connections;
//  4. it logs nothing, or logs the full URL (queries carry tokens).

func hostFetchTestHandler(t *testing.T, upstream http.Handler, perMinute, inFlight int) (*Handler, *IdentityRegistry) {
	t.Helper()
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	var d net.Dialer
	exec := &hostfetch.Executor{
		Lookup: func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		},
		Self: func(net.IP) bool { return false },
	}
	registry := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(registry, nil, nil)
	h.hostFetch = newHostFetchService(exec, map[string]bool{"computer-fetch-a": true, "computer-fetch-b": true}, perMinute, inFlight)
	return h, registry
}

func callHostFetch(h *Handler, token, rawURL string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(hostfetch.Request{Method: "GET", URL: rawURL})
	req := httptest.NewRequest(http.MethodPost, hostfetch.Path, strings.NewReader(string(body)))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.HandleHostFetch(w, req)
	return w
}

func issue(t *testing.T, r *IdentityRegistry, id string) string {
	t.Helper()
	cred, err := r.IssueCredential(id)
	if err != nil {
		t.Fatal(err)
	}
	return cred.RawToken
}

func TestHostFetchRequiresGatewayToken(t *testing.T) {
	h, _ := hostFetchTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("an unauthenticated fetch reached upstream")
	}), 100, 4)
	for _, token := range []string{"", "not-a-token"} {
		if w := callHostFetch(h, token, "http://example.com/"); w.Code != http.StatusUnauthorized {
			t.Fatalf("token %q: status %d", token, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, hostfetch.Path, nil)
	w := httptest.NewRecorder()
	h.HandleHostFetch(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: status %d", w.Code)
	}
}

func TestHostFetchServesAndLogsHostNotURL(t *testing.T) {
	logs := captureGatewayLog(t)
	h, reg := hostFetchTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "page")
	}), 100, 4)
	tok := issue(t, reg, "computer-fetch-a")
	w := callHostFetch(h, tok, "http://news.example/story?token=s3cret")
	if w.Code != http.StatusOK || w.Header().Get(hostfetch.StatusHeader) != "200" || w.Body.String() != "page" {
		t.Fatalf("status %d upstream %q body %q", w.Code, w.Header().Get(hostfetch.StatusHeader), w.Body.String())
	}
	line := logs.String()
	if !strings.Contains(line, "computer=computer-fetch-a") || !strings.Contains(line, "host=news.example") || !strings.Contains(line, "status=200") || !strings.Contains(line, "bytes=4") {
		t.Fatalf("log line missing fields: %q", line)
	}
	if strings.Contains(line, "s3cret") || strings.Contains(line, "/story") {
		t.Fatalf("log line carries the URL: %q", line)
	}

	refused := callHostFetch(h, tok, "http://169.254.169.254/latest/meta-data")
	if refused.Code != http.StatusForbidden || !strings.Contains(logs.String(), "refused") {
		t.Fatalf("metadata: status %d, log %q", refused.Code, logs.String())
	}
}

func TestHostFetchBudgetIsPerComputerAndItsOwn(t *testing.T) {
	h, reg := hostFetchTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), 2, 4)
	a, b := issue(t, reg, "computer-fetch-a"), issue(t, reg, "computer-fetch-b")
	for i := 0; i < 2; i++ {
		if w := callHostFetch(h, a, "http://example.com/"); w.Code != http.StatusOK {
			t.Fatalf("a #%d: %d", i, w.Code)
		}
	}
	w := callHostFetch(h, a, "http://example.com/")
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("a over budget: %d retry %q", w.Code, w.Header().Get("Retry-After"))
	}
	if w := callHostFetch(h, b, "http://example.com/"); w.Code != http.StatusOK {
		t.Fatalf("b starved by a: %d", w.Code)
	}

	if got := defaultHostFetchPerMinute(func(string) string { return "" }); got < 600 {
		t.Fatalf("default budget %d a minute would starve the paper's polls", got)
	}
	if got := defaultHostFetchPerMinute(func(string) string { return "1200" }); got != 1200 {
		t.Fatalf("override = %d", got)
	}
	if got := defaultHostFetchPerMinute(func(string) string { return "junk" }); got < 600 {
		t.Fatalf("junk override = %d", got)
	}
}

func TestHostFetchBoundsConcurrencyPerComputer(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	h, reg := hostFetchTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
	}), 100, 1)
	a, b := issue(t, reg, "computer-fetch-a"), issue(t, reg, "computer-fetch-b")
	done := make(chan int, 1)
	go func() { done <- callHostFetch(h, a, "http://example.com/slow").Code }()
	<-entered
	if w := callHostFetch(h, a, "http://example.com/"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second in flight for a: %d", w.Code)
	}
	go func() { <-entered }()
	bDone := make(chan int, 1)
	go func() { bDone <- callHostFetch(h, b, "http://example.com/").Code }()
	close(release)
	if code := <-done; code != http.StatusOK {
		t.Fatalf("first a: %d", code)
	}
	if code := <-bDone; code != http.StatusOK {
		t.Fatalf("b blocked by a's slot: %d", code)
	}
	if w := callHostFetch(h, a, "http://example.com/"); w.Code != http.StatusOK {
		t.Fatalf("a's slot not released: %d", w.Code)
	}
}

// The verb is off at the host unless the host lists the caller: a guest's
// grant is the guest's choice, the list is the host's
// (docs/problems/host-fetch-verb-open-to-every-guest-2026-10-22.md).
func TestHostFetchRefusesIdentitiesTheHostDoesNotList(t *testing.T) {
	logs := captureGatewayLog(t)
	h, reg := hostFetchTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "page")
	}), 100, 4)
	listed, unlisted := issue(t, reg, "computer-fetch-a"), issue(t, reg, "vm-not-listed")
	if w := callHostFetch(h, listed, "http://example.com/"); w.Code != http.StatusOK {
		t.Fatalf("listed: %d", w.Code)
	}
	w := callHostFetch(h, unlisted, "http://example.com/secret-path?q=1")
	var body struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusForbidden || body.Code != hostFetchNotGrantedCode || body.Error == "" {
		t.Fatalf("unlisted: status %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(logs.String(), "fetch refused computer=vm-not-listed reason=not_granted") || strings.Contains(logs.String(), "secret-path") {
		t.Fatalf("log: %q", logs.String())
	}
}

func TestHostFetchEmptyListRefusesEveryone(t *testing.T) {
	var hits int
	h, reg := hostFetchTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }), 100, 4)
	h.hostFetch = newHostFetchService(h.hostFetch.exec, loadHostFetchAllowList(func(string) string { return "" }), 100, 4)
	for _, id := range []string{"computer-fetch-a", "vm-any"} {
		if w := callHostFetch(h, issue(t, reg, id), "http://example.com/"); w.Code != http.StatusForbidden {
			t.Fatalf("%s with an empty list: %d", id, w.Code)
		}
	}
	if hits != 0 {
		t.Fatalf("upstream reached %d times with an empty list", hits)
	}
	got := loadHostFetchAllowList(func(k string) string {
		if k == EnvHostFetchVMs {
			return "vm-a, vm-b\nclass:test"
		}
		return ""
	})
	if !got["vm-a"] || !got["vm-b"] || got["class:test"] || len(got) != 2 {
		t.Fatalf("allow list = %v (class tokens are not identities)", got)
	}
}
