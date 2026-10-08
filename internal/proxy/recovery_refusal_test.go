package proxy

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// recoveryRefusalProxyEnvironment builds a proxy whose vmctl dependency
// answers route-absent and then a structured 503 recovery refusal.
func recoveryRefusalProxyEnvironment(t *testing.T) (*Handler, ed25519.PrivateKey, *int64, *int64) {
	t.Helper()
	var routeCalls, resolveCalls int64
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/computer-version-routes/resolve", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&routeCalls, 1)
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{RouteAbsent: true})
	})
	mux.HandleFunc("/internal/vmctl/resolve", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&resolveCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":               "computer recovery blocked",
			"reason":              "recovery tail 423720 events exceeds 10000; publish a fresher base (local=0 W=148431 H=572151)",
			"kind":                "recovery_tail_excess",
			"retry_after_seconds": 60,
			"repair":              map[string]any{"computer_id": "computer-blocked", "status": "running", "watermark_sequence": 148431},
		})
	})
	vmctlServer := httptest.NewServer(mux)
	t.Cleanup(vmctlServer.Close)

	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A refused resolve must never reach the guest; answer as a dead
		// upstream rather than failing from a non-test goroutine.
		http.Error(w, "unexpected upstream call for refused resolve", http.StatusBadGateway)
	}))
	t.Cleanup(autoputer.Close)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	h, err := NewHandler(&Config{
		Port:              "0",
		ComputerURL:       autoputer.URL,
		VmctlURL:          vmctlServer.URL,
		AuthPublicKeyPath: "/unused/in/test",
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	h.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	return h, priv, &routeCalls, &resolveCalls
}

// TestResolveComputerURLDoesNotRetryTypedRecoveryRefusal: a durable refusal is
// deterministic; the proxy must return it promptly instead of riding the
// transient retry window, and the typed error must carry reason, repair status
// and Retry-After.
func TestResolveComputerURLDoesNotRetryTypedRecoveryRefusal(t *testing.T) {
	h, _, routeCalls, resolveCalls := recoveryRefusalProxyEnvironment(t)

	_, err := h.resolveComputerURL(context.Background(), "owner", "primary")
	var refusal *vmctl.RecoveryRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("resolve error = %v, want *vmctl.RecoveryRefusalError", err)
	}
	if refusal.Kind != "recovery_tail_excess" || refusal.RetryAfterSeconds != 60 {
		t.Fatalf("refusal = %+v", refusal)
	}
	if refusal.Repair == nil || refusal.Repair.Status != "running" || refusal.Repair.WatermarkSequence != 148431 {
		t.Fatalf("repair status = %+v", refusal.Repair)
	}
	if !strings.Contains(refusal.Reason, "recovery tail 423720 events exceeds 10000") {
		t.Fatalf("reason = %q", refusal.Reason)
	}
	if atomic.LoadInt64(routeCalls) != 1 {
		t.Fatalf("route authority calls = %d, want 1 (route identity preserved)", *routeCalls)
	}
	// The transient retry window retries every 200ms doubling; any retry would
	// be visible well before this settle delay.
	time.Sleep(400 * time.Millisecond)
	if got := atomic.LoadInt64(resolveCalls); got != 1 {
		t.Fatalf("typed refusal resolve calls = %d, want 1 (no retry storm)", got)
	}
}

// TestComputerSurfaceReturns503RetryAfterForRecoveryRefusal locks the public
// structured 503: reason + repair status + Retry-After: 60, after auth.
func TestComputerSurfaceReturns503RetryAfterForRecoveryRefusal(t *testing.T) {
	h, priv, _, resolveCalls := recoveryRefusalProxyEnvironment(t)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "owner")})
	recorder := httptest.NewRecorder()
	h.HandleComputerSurface(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
	var body struct {
		Error             string                     `json:"error"`
		Reason            string                     `json:"reason"`
		Kind              string                     `json:"kind"`
		RetryAfterSeconds int                        `json:"retry_after_seconds"`
		Repair            *vmctl.CheckpointJobStatus `json:"repair"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode refusal body: %v body=%s", err, recorder.Body.String())
	}
	if body.Error != "computer recovery blocked" || body.Kind != "recovery_tail_excess" || body.RetryAfterSeconds != 60 {
		t.Fatalf("refusal body = %+v", body)
	}
	if body.Repair == nil || body.Repair.Status != "running" {
		t.Fatalf("repair = %+v", body.Repair)
	}
	if !strings.Contains(body.Reason, "publish a fresher base") {
		t.Fatalf("reason = %q", body.Reason)
	}
	if atomic.LoadInt64(resolveCalls) != 1 {
		t.Fatalf("resolve calls = %d, want 1", *resolveCalls)
	}
}

// TestComputerSurfaceRequiresAuthBeforeRecoveryRefusal: the refusal path must
// not bypass authentication.
func TestComputerSurfaceRequiresAuthBeforeRecoveryRefusal(t *testing.T) {
	h, _, _, resolveCalls := recoveryRefusalProxyEnvironment(t)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	h.HandleComputerSurface(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := atomic.LoadInt64(resolveCalls); got != 0 {
		t.Fatalf("unauthenticated request reached vmctl resolve %d times", got)
	}
}

// TestComputerRecoveryJobRouteForwardsOwnerScopedRequest: the owner-visible
// job surface must authenticate, join ownership, and call the platform job
// endpoint as the trusted host with the owner attestation only.
func TestComputerRecoveryJobRouteForwardsOwnerScopedRequest(t *testing.T) {
	type call struct {
		method string
		owner  string
		query  string
		body   map[string]any
	}
	var mu sync.Mutex
	var calls []call
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/projection-base/jobs" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			http.Error(w, "internal caller required", http.StatusForbidden)
			return
		}
		recorded := call{method: r.Method, owner: r.Header.Get("X-Authenticated-User"), query: r.URL.RawQuery}
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&recorded.body)
		}
		mu.Lock()
		calls = append(calls, recorded)
		mu.Unlock()
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "status": "queued", "reason": "owner recheck",
			"target_sequence": 572151, "watermark_sequence": 148431, "base_ref": "base-stale",
		})
	}))
	t.Cleanup(corpusd.Close)

	lookup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/lookup" || r.URL.Query().Get("computer_id") != "computer-a" || r.URL.Query().Get("user_id") != "owner-user" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "user_id": "owner-user", "desktop_id": "primary", "state": "active",
		})
	}))
	t.Cleanup(lookup.Close)

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(lookup.URL)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	getRequest := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/recovery/job", nil)
	getRequest.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	getResponse := httptest.NewRecorder()
	handler.HandleAPI(getResponse, getRequest)
	if getResponse.Code != http.StatusOK || !strings.Contains(getResponse.Body.String(), `"status":"queued"`) {
		t.Fatalf("job GET status=%d body=%s", getResponse.Code, getResponse.Body.String())
	}

	postRequest := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/recovery/job", strings.NewReader(`{"reason":"owner recheck"}`))
	postRequest.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	postResponse := httptest.NewRecorder()
	handler.HandleAPI(postResponse, postRequest)
	if postResponse.Code != http.StatusAccepted {
		t.Fatalf("job POST status=%d body=%s", postResponse.Code, postResponse.Body.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("platform job calls = %d, want 2 (%+v)", len(calls), calls)
	}
	if calls[0].method != http.MethodGet || calls[0].owner != "owner-user" || calls[0].query != "computer_id=computer-a" {
		t.Fatalf("job GET call = %+v", calls[0])
	}
	if calls[1].method != http.MethodPost || calls[1].body["computer_id"] != "computer-a" || calls[1].body["reason"] != "owner recheck" {
		t.Fatalf("job POST call = %+v", calls[1])
	}
	if genesis, ok := calls[1].body["genesis_repair"]; !ok || genesis != false {
		t.Fatalf("owner job POST must pin genesis_repair=false, body=%+v", calls[1].body)
	}
}

// TestComputerRecoveryJobRouteDeniesForeignCallers: ownership join and auth
// still gate the owner-visible repair surface.
func TestComputerRecoveryJobRouteDeniesForeignCallers(t *testing.T) {
	var corpusdCalls int64
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&corpusdCalls, 1)
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": "computer-a", "status": "queued"})
	}))
	t.Cleanup(corpusd.Close)

	lookup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "user_id": "other-user", "desktop_id": "primary", "state": "active",
		})
	}))
	t.Cleanup(lookup.Close)

	handler, privateKey, _, _ := testProxyEnvWithAuthStore(t)
	handler.vmctlClient = vmctl.NewClient(lookup.URL)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.corpusd = corpusd.Client()

	unauthorized := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/recovery/job", nil)
	unauthorizedResponse := httptest.NewRecorder()
	handler.HandleAPI(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d body=%s", unauthorizedResponse.Code, unauthorizedResponse.Body.String())
	}

	foreign := httptest.NewRequest(http.MethodGet, "/api/computers/computer-a/recovery/job", nil)
	foreign.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, "owner-user")})
	foreignResponse := httptest.NewRecorder()
	handler.HandleAPI(foreignResponse, foreign)
	if foreignResponse.Code != http.StatusForbidden {
		t.Fatalf("foreign status=%d body=%s", foreignResponse.Code, foreignResponse.Body.String())
	}

	if got := atomic.LoadInt64(&corpusdCalls); got != 0 {
		t.Fatalf("denied callers reached the platform job endpoint %d times", got)
	}
}
