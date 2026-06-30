package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// --- M22b / C20: per-service health endpoint tests ---

// TestHandleServiceHealth_OkWhenHealthy verifies that GET /health/{service}
// returns "ok" (200) when the probed backend dependency is reachable.
func TestHandleServiceHealth_OkWhenHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h, _ := setupHandlerNoProvider(t)
	h.SetServiceCheckers(map[string]health.Checker{
		"qdrant": health.HTTPChecker{NameStr: "qdrant", URL: srv.URL, Timeout: time.Second},
	})

	req := httptest.NewRequest(http.MethodGet, "/health/qdrant", nil)
	req.SetPathValue("service", "qdrant")
	w := httptest.NewRecorder()
	h.HandleServiceHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp serviceHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != string(health.StatusOK) {
		t.Errorf("status = %q, want %q", resp.Status, health.StatusOK)
	}
	if resp.Service != "qdrant" {
		t.Errorf("service = %q, want %q", resp.Service, "qdrant")
	}
}

// TestHandleServiceHealth_UnhealthyWhenDown verifies that GET /health/{service}
// returns "unhealthy" (503) with a generic public error when the probed
// dependency is unreachable.
func TestHandleServiceHealth_UnhealthyWhenDown(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)
	h.SetServiceCheckers(map[string]health.Checker{
		"ollama": health.HTTPChecker{NameStr: "ollama", URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond},
	})

	req := httptest.NewRequest(http.MethodGet, "/health/ollama", nil)
	req.SetPathValue("service", "ollama")
	w := httptest.NewRecorder()
	h.HandleServiceHealth(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %s", w.Code, w.Body.String())
	}
	var resp serviceHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != string(health.StatusUnhealthy) {
		t.Errorf("status = %q, want %q", resp.Status, health.StatusUnhealthy)
	}
	if resp.Error == "" {
		t.Error("error message empty for unhealthy dependency")
	}
}

// TestHandleServiceHealth_NotConfiguredForUnknownService verifies that an
// unknown service name reports "not configured" (200) rather than 404, so
// operators can distinguish "no probe wired" from "endpoint missing".
func TestHandleServiceHealth_NotConfiguredForUnknownService(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)
	h.SetServiceCheckers(map[string]health.Checker{
		"qdrant": health.HTTPChecker{NameStr: "qdrant", URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond},
	})

	req := httptest.NewRequest(http.MethodGet, "/health/unknown", nil)
	req.SetPathValue("service", "unknown")
	w := httptest.NewRecorder()
	h.HandleServiceHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp serviceHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "not configured" {
		t.Errorf("status = %q, want %q", resp.Status, "not configured")
	}
}

// TestHandleServiceHealth_NotConfiguredWhenNoCheckers verifies the endpoint
// reports "not configured" when SetServiceCheckers was never called.
func TestHandleServiceHealth_NotConfiguredWhenNoCheckers(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)

	req := httptest.NewRequest(http.MethodGet, "/health/qdrant", nil)
	req.SetPathValue("service", "qdrant")
	w := httptest.NewRecorder()
	h.HandleServiceHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp serviceHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "not configured" {
		t.Errorf("status = %q, want %q", resp.Status, "not configured")
	}
}

// TestHandleServiceHealth_RejectsNonGet verifies only GET is accepted.
func TestHandleServiceHealth_RejectsNonGet(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)
	h.SetServiceCheckers(map[string]health.Checker{
		"qdrant": health.CheckerFunc{NameStr: "qdrant", Fn: func(ctx context.Context) error { return nil }},
	})
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health/qdrant", nil)
			req.SetPathValue("service", "qdrant")
			w := httptest.NewRecorder()
			h.HandleServiceHealth(w, req)
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s: got %d, want 405", method, w.Code)
			}
		})
	}
}

// TestHandleServiceHealth_SurfacesBreakerState verifies that when a circuit
// breaker is registered for the probed service, the response includes the
// breaker state ("closed"/"open"/"half-open").
func TestHandleServiceHealth_SurfacesBreakerState(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)
	h.SetServiceCheckers(map[string]health.Checker{
		"qdrant": health.CheckerFunc{NameStr: "qdrant", Fn: func(ctx context.Context) error { return nil }},
	})
	breakers := NewBreakerRegistry()
	b := health.NewCircuitBreaker(health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	breakers.Register("qdrant", b)
	h.SetBreakers(breakers)

	// Force the breaker open.
	b.RecordFailure()

	req := httptest.NewRequest(http.MethodGet, "/health/qdrant", nil)
	req.SetPathValue("service", "qdrant")
	w := httptest.NewRecorder()
	h.HandleServiceHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp serviceHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Breaker == nil {
		t.Fatal("breaker field nil; want state")
	}
	if *resp.Breaker != "open" {
		t.Errorf("breaker = %q, want %q", *resp.Breaker, "open")
	}
}

// TestHandleServiceHealth_NoSecretsInError verifies the error field is
// generic and never contains the raw checker error or secret material. This
// guards the "health endpoints are public but expose no secrets" invariant.
func TestHandleServiceHealth_NoSecretsInError(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)
	secret := "super-secret-api-key-value-1234567890"
	h.SetServiceCheckers(map[string]health.Checker{
		"ollama": health.CheckerFunc{
			NameStr: "ollama",
			Fn: func(ctx context.Context) error {
				return errors.New("dial " + secret + " failed with Authorization: Bearer sk-live-secret-token")
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/health/ollama", nil)
	req.SetPathValue("service", "ollama")
	w := httptest.NewRecorder()
	h.HandleServiceHealth(w, req)

	body := w.Body.String()
	var resp serviceHealthResponse
	if err := json.NewDecoder(strings.NewReader(body)).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == "" {
		t.Fatal("error empty for unhealthy dependency")
	}
	if resp.Error != "dependency check failed" {
		t.Fatalf("error = %q, want generic public health error", resp.Error)
	}
	for _, leak := range []string{secret, "Authorization:", "Bearer ", "sk-live-secret-token", "dial "} {
		if strings.Contains(body, leak) {
			t.Fatalf("public health response leaked %q: %s", leak, body)
		}
	}
}

// TestHandleServiceHealth_RouteViaMux verifies the /health/{service} route is
// registered on the server mux and reachable end-to-end (M22b / C20:
// "mounted in the gateway router without disrupting existing routing").
func TestHandleServiceHealth_RouteViaMux(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h, _ := setupHandlerNoProvider(t)
	h.SetServiceCheckers(map[string]health.Checker{
		"runtime": health.HTTPChecker{NameStr: "runtime", URL: srv.URL, Timeout: time.Second},
	})

	s := server.NewServer("gateway", "0")
	RegisterRoutes(s, h)

	// Existing /health route must still work (no disruption).
	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	wHealth := httptest.NewRecorder()
	s.ServeHTTP(wHealth, reqHealth)
	if wHealth.Code != http.StatusOK {
		t.Fatalf("existing /health route broken: got %d", wHealth.Code)
	}

	// New /health/{service} route must be reachable.
	reqSvc := httptest.NewRequest(http.MethodGet, "/health/runtime", nil)
	wSvc := httptest.NewRecorder()
	s.ServeHTTP(wSvc, reqSvc)
	if wSvc.Code != http.StatusOK {
		t.Fatalf("/health/runtime: got %d, want 200; body: %s", wSvc.Code, wSvc.Body.String())
	}
	var resp serviceHealthResponse
	if err := json.NewDecoder(wSvc.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != string(health.StatusOK) {
		t.Errorf("status = %q, want ok", resp.Status)
	}
}

// --- Config tests ---

// TestDefaultServiceHealthURLs verifies the default probe URLs cover all
// required services named in the M22b spec.
func TestDefaultServiceHealthURLs(t *testing.T) {
	required := []string{"sourcecycled", "runtime", "qdrant", "dolt", "ollama"}
	for _, name := range required {
		if _, ok := DefaultServiceHealthURLs[name]; !ok {
			t.Errorf("DefaultServiceHealthURLs missing %q", name)
		}
	}
}

// TestLoadServiceHealthURLs_Override verifies env var overrides are applied.
func TestLoadServiceHealthURLs_Override(t *testing.T) {
	t.Setenv("GATEWAY_HEALTH_QDRANT_URL", "http://override:6333/healthz")
	urls := loadServiceHealthURLs()
	if urls["qdrant"] != "http://override:6333/healthz" {
		t.Errorf("qdrant url = %q, want override", urls["qdrant"])
	}
	// Non-overridden entries keep defaults.
	if urls["ollama"] != DefaultServiceHealthURLs["ollama"] {
		t.Errorf("ollama url = %q, want default %q", urls["ollama"], DefaultServiceHealthURLs["ollama"])
	}
}

// --- Circuit breaker integration tests ---

// TestInferencePath_CircuitBreakerOpen verifies that the LLM provider circuit
// breaker is wired into the inference call path: when the breaker is open,
// HandleInference returns a circuit-open error without contacting the
// upstream provider (M22b / C20: "LLM provider calls: circuit break on
// repeated failures").
func TestInferencePath_CircuitBreakerOpen(t *testing.T) {
	reg := NewIdentityRegistry(1 * time.Hour)
	result, _ := reg.IssueCredential("sandbox-1")

	// Wrap a stub provider with a circuit breaker and force it open.
	stub := &stubProvider{name: "stub", real: true}
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 1, OpenTimeout: time.Hour})
	// Record one failure to open the breaker (threshold=1).
	_ = cbp.Breaker().Execute(func() error { return errors.New("upstream down") })
	if cbp.Breaker().State() != health.StateOpen {
		t.Fatalf("breaker state = %v, want open", cbp.Breaker().State())
	}

	mp := provider.NewMultiProvider()
	mp.Register("stub", cbp)
	h := NewMultiHandler(reg, mp)
	breakers := NewBreakerRegistry()
	breakers.Register("stub", cbp.Breaker())
	h.SetBreakers(breakers)

	payload := ProviderRequest{
		Provider:  "stub",
		Messages:  []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "hi"}}}},
		MaxTokens: 10,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bearer "+result.RawToken)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.HandleInference(w, req)

	// The inference path must surface the circuit-open failure as a 502
	// (sanitized Bad Gateway), not a 200 success.
	if w.Code != http.StatusBadGateway {
		t.Fatalf("inference with open breaker: got %d, want 502; body: %s", w.Code, w.Body.String())
	}
	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errResp.Error == "" {
		t.Fatal("expected sanitized circuit-open error message")
	}
}
