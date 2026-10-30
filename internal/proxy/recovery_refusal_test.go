package proxy

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
// transient retry window, and the typed error must carry reason and
// Retry-After.
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
// structured 503: reason + Retry-After: 60, after auth.
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
		Error             string `json:"error"`
		Reason            string `json:"reason"`
		Kind              string `json:"kind"`
		RetryAfterSeconds int    `json:"retry_after_seconds"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode refusal body: %v body=%s", err, recorder.Body.String())
	}
	if body.Error != "computer recovery blocked" || body.Kind != "recovery_tail_excess" || body.RetryAfterSeconds != 60 {
		t.Fatalf("refusal body = %+v", body)
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
