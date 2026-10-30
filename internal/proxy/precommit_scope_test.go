package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Calibration level 2 (docs/archive/reports/choir-mcp-precommitments-2026-10-14.md
// §6 item 9). Ways the scope gate could fail, written before the code:
//   - /api/precommit/ falls through to read:runtime / write:runtime, so an
//     agent's claim key must also be able to start runs;
//   - a precommit:read key writes claims;
//   - a precommit:write key reaches other runtime routes;
//   - a write:runtime key writes claims (the scope must be the narrow one).
func TestPrecommitRoutesRequireThePrecommitScopes(t *testing.T) {
	handler, _, autoputer, store := testProxyEnvWithAuthStore(t)
	reached := map[string]int{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		reached[r.Method+" "+r.URL.Path]++
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	autoputer.Config.Handler = mux

	_, writer := createTestAPIKey(t, handler, store, "precommit-writer", []string{"precommit:write", "precommit:read"}, nil)
	_, reader := createTestAPIKey(t, handler, store, "precommit-reader", []string{"precommit:read"}, nil)
	_, runtime := createTestAPIKey(t, handler, store, "runtime-writer", []string{"read:runtime", "write:runtime"}, nil)

	send := func(secret, method, path string) int {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+secret)
		rec := httptest.NewRecorder()
		handler.HandleProtectedAPI(rec, req)
		return rec.Code
	}
	for _, tc := range []struct {
		name, secret, method, path string
		allowed                    bool
	}{
		{"writer adds a claim", writer, http.MethodPost, "/api/precommit/claims", true},
		{"writer settles", writer, http.MethodPost, "/api/precommit/claims/c1/settlements", true},
		{"writer reads calibration", writer, http.MethodGet, "/api/precommit/calibration", true},
		{"reader lists", reader, http.MethodGet, "/api/precommit/claims", true},
		{"reader cannot add", reader, http.MethodPost, "/api/precommit/claims", false},
		{"writer cannot start runs", writer, http.MethodPost, "/api/runs", false},
		{"writer cannot read runtime", writer, http.MethodGet, "/api/runs", false},
		{"runtime key cannot add claims", runtime, http.MethodPost, "/api/precommit/claims", false},
		{"runtime key cannot read claims", runtime, http.MethodGet, "/api/precommit/claims", false},
	} {
		before := reached[tc.method+" "+tc.path]
		code := send(tc.secret, tc.method, tc.path)
		got := reached[tc.method+" "+tc.path] > before
		if tc.allowed && (code != http.StatusNoContent || !got) {
			t.Fatalf("%s: status %d, reached %v; want forwarded", tc.name, code, got)
		}
		if !tc.allowed && (code != http.StatusForbidden || got) {
			t.Fatalf("%s: status %d, reached %v; want 403 before the upstream", tc.name, code, got)
		}
	}
}
