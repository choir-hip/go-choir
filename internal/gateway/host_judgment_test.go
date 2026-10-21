package gateway

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// SEC-16: a host job that scores with Jev calls the gateway instead of
// holding the OpenRouter key (docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md
// section "Host-side callers", docs/learn/03-gateway-and-providers.md 2.3).
// Ways the route can fail, written before the code:
//  1. a computer (a guest address) or any host process without the internal
//     credential reaches it and spends the Jev key
//     (TestHostJudgmentRefusesEveryoneButTheInternalCaller);
//  2. the caller names itself and the name is trusted: an unknown host caller
//     must be refused, and the label in the receipt comes from the gateway's
//     table, not the request (TestHostJudgmentCallerIsAnAllowlistedLabel);
//  3. a disabled Jev transport answers something other than 503
//     (TestHostJudgmentFollowsTheJevSwitch);
//  4. the request carries a model, a scope or extra fields upstream, or
//     oversize state (TestHostJudgmentValidatesTheRequest);
//  5. the call is not metered: no log line, no receipt
//     (TestHostJudgmentIsMeteredLikeAnyJevCall).

const hostJudgmentToken = "host-judgment-test-token-0123456789abcdef0123"

func hostJudgmentHandler(t *testing.T) (*Handler, *decisionsRoundTripper, string) {
	t.Helper()
	h, _ := setupHandlerNoProvider(t)
	upstream := &decisionsRoundTripper{}
	receipts := filepath.Join(t.TempDir(), "receipts.jsonl")
	h.SetJevTransport(&JevTransport{enabled: true, apiKey: "fake-openrouter-key", httpClient: &http.Client{Transport: upstream},
		receipts: newJevReceiptWriter(receipts)})
	return h, upstream, receipts
}

func hostJudgmentBody() string {
	body, _ := json.Marshal(map[string]any{
		"state":     map[string]any{"story": map[string]any{"title": "x"}},
		"questions": map[string]any{"q": map[string]any{"type": "choice", "choices": []string{"yes", "no"}}},
	})
	return string(body)
}

func hostJudge(h *Handler, remote, caller, token string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/host/judgments", strings.NewReader(body))
	req.RemoteAddr = remote
	req.Header.Set(internalCallerHeader, "true")
	if caller != "" {
		req.Header.Set(hostCallerHeader, caller)
	}
	if token != "" {
		req.Header.Set(hostauth.Header, token)
	}
	rec := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(h.HandleHostJudgment)).ServeHTTP(rec, req)
	return rec
}

func TestHostJudgmentRefusesEveryoneButTheInternalCaller(t *testing.T) {
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, hostJudgmentToken)
	defer restore()
	h, upstream, _ := hostJudgmentHandler(t)
	body := hostJudgmentBody()

	for _, tc := range []struct{ name, remote, token string }{
		{"guest with the credential", "10.200.1.2:6000", hostJudgmentToken},
		{"public address", "147.135.70.10:6000", hostJudgmentToken},
		{"loopback without the credential", "127.0.0.1:6000", ""},
		{"loopback with a wrong credential", "127.0.0.1:6000", "wrong-wrong-wrong-wrong-wrong-wrong-wrong"},
	} {
		if rec := hostJudge(h, tc.remote, "front-importance", tc.token, body); rec.Code != http.StatusForbidden {
			t.Fatalf("%s: %d, want 403", tc.name, rec.Code)
		}
	}
	// Loopback with the marker alone, the header anyone can send.
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/host/judgments", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:6000"
	req.Header.Set(hostCallerHeader, "front-importance")
	rec := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(h.HandleHostJudgment)).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("no marker: %d, want 403", rec.Code)
	}
	if len(upstream.bodies) != 0 {
		t.Fatalf("%d refused calls reached the upstream", len(upstream.bodies))
	}
	if rec := hostJudge(h, "127.0.0.1:6000", "front-importance", hostJudgmentToken, body); rec.Code != http.StatusOK {
		t.Fatalf("the internal caller: %d: %s", rec.Code, rec.Body.String())
	}
	// A computer's bearer is not a host credential.
	h2, reg := setupHandlerNoProvider(t)
	h2.SetJevTransport(h.jevTransport)
	cred, _ := reg.IssueCredential("computer-x")
	greq := httptest.NewRequest(http.MethodPost, "/provider/v1/host/judgments", strings.NewReader(body))
	greq.RemoteAddr = "10.200.1.2:6000"
	greq.Header.Set("Authorization", "Bearer "+cred.RawToken)
	greq.Header.Set(hostCallerHeader, "front-importance")
	grec := httptest.NewRecorder()
	hostauth.Middleware(http.HandlerFunc(h2.HandleHostJudgment)).ServeHTTP(grec, greq)
	if grec.Code != http.StatusForbidden {
		t.Fatalf("a computer's bearer reached the host route: %d", grec.Code)
	}
}

func TestHostJudgmentCallerIsAnAllowlistedLabel(t *testing.T) {
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, hostJudgmentToken)
	defer restore()
	h, _, receipts := hostJudgmentHandler(t)
	for _, caller := range []string{"", "other", "Front-Importance", "host:front-importance", "computer-abc"} {
		if rec := hostJudge(h, "127.0.0.1:6000", caller, hostJudgmentToken, hostJudgmentBody()); rec.Code != http.StatusForbidden {
			t.Fatalf("caller %q: %d, want 403", caller, rec.Code)
		}
	}
	if rec := hostJudge(h, "127.0.0.1:6000", "front-importance", hostJudgmentToken, hostJudgmentBody()); rec.Code != http.StatusOK {
		t.Fatalf("front-importance: %d", rec.Code)
	}
	data, err := os.ReadFile(receipts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"computer_id":"host:front-importance"`) {
		t.Fatalf("the receipt does not carry the gateway's own label: %s", data)
	}
}

func TestHostJudgmentFollowsTheJevSwitch(t *testing.T) {
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, hostJudgmentToken)
	defer restore()
	h, _, _ := hostJudgmentHandler(t)
	h.jevTransport.enabled = false
	if rec := hostJudge(h, "127.0.0.1:6000", "front-importance", hostJudgmentToken, hostJudgmentBody()); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled transport: %d, want 503", rec.Code)
	}
	h.SetJevTransport(nil)
	if rec := hostJudge(h, "127.0.0.1:6000", "front-importance", hostJudgmentToken, hostJudgmentBody()); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no transport: %d, want 503", rec.Code)
	}
}

func TestHostJudgmentValidatesTheRequest(t *testing.T) {
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, hostJudgmentToken)
	defer restore()
	h, upstream, _ := hostJudgmentHandler(t)
	for name, body := range map[string]string{
		"not json":     `nope`,
		"no questions": `{"state":{"a":1},"questions":{}}`,
		"state array":  `{"state":[1],"questions":{"q":{"type":"choice"}}}`,
		"bad question": `{"state":{"a":1},"questions":{"q":{"type":"essay"}}}`,
		"scope":        `{"state":{"a":1},"questions":{"q":{"type":"choice"}},"scope":"intake_labels"}`,
		"oversize":     `{"state":{"a":"` + strings.Repeat("x", jevTransportMaxBodySize+10) + `"},"questions":{"q":{"type":"choice"}}}`,
	} {
		rec := hostJudge(h, "127.0.0.1:6000", "front-importance", hostJudgmentToken, body)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s: %d, want 400 or 413", name, rec.Code)
		}
	}
	if len(upstream.bodies) != 0 {
		t.Fatalf("an invalid request reached the upstream: %v", upstream.bodies)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/provider/v1/host/judgments", nil)
	h.HandleHostJudgment(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET: %d, want 405", rec.Code)
	}
}

func TestHostJudgmentIsMeteredLikeAnyJevCall(t *testing.T) {
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, hostJudgmentToken)
	defer restore()
	h, upstream, receipts := hostJudgmentHandler(t)
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(prev)
	rec := hostJudge(h, "127.0.0.1:6000", "front-importance", hostJudgmentToken, hostJudgmentBody())
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(logs.String(), "host judgment") || !strings.Contains(logs.String(), "front-importance") {
		t.Fatalf("no metering log line: %s", logs.String())
	}
	if strings.Contains(logs.String(), `"title"`) || strings.Contains(logs.String(), "fake-openrouter-key") {
		t.Fatalf("the log carries the request state or the key: %s", logs.String())
	}
	if len(upstream.bodies) != 1 || strings.Contains(upstream.bodies[0], "front-importance") {
		t.Fatalf("upstream bodies: %v (the label must not ride upstream)", upstream.bodies)
	}
	if data, _ := os.ReadFile(receipts); !strings.Contains(string(data), "choir.jev_transport_receipt.v1") {
		t.Fatalf("no transport receipt: %s", data)
	}
	if rec.Body.Len() == 0 || !strings.Contains(rec.Body.String(), "answers") {
		t.Fatalf("the exact upstream body must come back: %s", rec.Body.String())
	}
}
