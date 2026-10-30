package autoputer

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The SPA catch-all must not answer API paths. A client of a removed /api
// route got 200 index.html (a JSON parse error, not a 404), and an unknown
// API POST got a 405 text body, which hid route drift
// (docs/archive/problems/surface-broken-calls-2026-10-14.md, item 6).
//
// Failure modes pinned: unknown /api GET serves the SPA; unknown /api POST
// answers 405; the 404 is not JSON; a non-API path that merely starts with
// "api" (or an SPA route) loses the fallback.
func TestComputerSurfaceAnswersUnknownAPIPathsWithJSON404(t *testing.T) {
	root := filepath.Join(t.TempDir(), "current", "frontend")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("computer-a-shell"), 0o644); err != nil {
		t.Fatal(err)
	}
	surface := &ComputerSurface{Root: root}

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/removed-route"},
		{http.MethodGet, "/api/computers/c-1/events/abc"},
		{http.MethodGet, "/api"},
		{http.MethodHead, "/api/removed-route"},
		{http.MethodPost, "/api/removed-route"},
		{http.MethodDelete, "/api/"},
	} {
		response := httptest.NewRecorder()
		surface.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s %s status=%d body=%s, want 404", tc.method, tc.path, response.Code, response.Body.String())
			continue
		}
		if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
			t.Errorf("%s %s content type=%q, want JSON", tc.method, tc.path, got)
		}
		if tc.method != http.MethodHead && !strings.Contains(response.Body.String(), `"error"`) {
			t.Errorf("%s %s body=%s, want a JSON error", tc.method, tc.path, response.Body.String())
		}
	}

	for _, path := range []string{"/apiary", "/desktop/api/x", "/desktop/texture"} {
		response := httptest.NewRecorder()
		surface.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "computer-a-shell") {
			t.Errorf("GET %s status=%d body=%s, want the SPA", path, response.Code, response.Body.String())
		}
	}
}
