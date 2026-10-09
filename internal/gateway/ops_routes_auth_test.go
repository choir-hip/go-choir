package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// docs/problems/search-plane-cooldown-on-caller-cancel-2026-10-09.md
// (ops routes amendment). Failure modes: a computer on its tap reads
// provider health or resets a search or inference breaker, with or without
// asserting X-Internal-Caller; an operator on Node B loopback is refused.

func TestOpsRoutesRefuseComputerCallers(t *testing.T) {
	h, _ := setupHandlerNoProvider(t)
	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/provider/v1/search/health", ""},
		{http.MethodPost, "/provider/v1/search/health/reset", `{"provider":"brave"}`},
		{http.MethodGet, "/provider/v1/breakers", ""},
		{http.MethodPost, "/provider/v1/breakers/reset", `{"provider":"bedrock"}`},
	}
	handlers := map[string]http.HandlerFunc{
		"/provider/v1/search/health":       h.HandleSearchHealth,
		"/provider/v1/search/health/reset": h.HandleSearchHealthReset,
		"/provider/v1/breakers":            h.HandleProviderBreakers,
		"/provider/v1/breakers/reset":      h.HandleProviderBreakerReset,
	}
	for _, rt := range routes {
		for _, remote := range []string{"10.200.3.2:41000", "147.135.70.10:41000"} {
			req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
			req.RemoteAddr = remote
			req.Header.Set(internalCallerHeader, "true")
			rec := httptest.NewRecorder()
			handlers[rt.path](rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("%s %s from %s = %d, want 403", rt.method, rt.path, remote, rec.Code)
			}
		}
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
		req.RemoteAddr = "127.0.0.1:41000"
		req.Header.Set(internalCallerHeader, "true")
		rec := httptest.NewRecorder()
		handlers[rt.path](rec, req)
		if rec.Code == http.StatusForbidden {
			t.Fatalf("%s %s from loopback operator refused", rt.method, rt.path)
		}
	}
}
