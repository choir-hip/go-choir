package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Failure modes pinned: a non-host caller reading process internals; the
// named profiles unreachable under the /internal prefix (net/http/pprof's
// Index only resolves names under /debug/pprof/); contention profiles empty
// because sampling was never enabled; an unknown name served as the index.
func TestDebugPprofServedOnlyToHostCallers(t *testing.T) {
	mux := NewServer("profile-test", "0").handler

	get := func(remote, path string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	if rec := get("203.0.113.9:4000", DebugPprofPrefix+"goroutine?debug=1"); rec.Code != http.StatusForbidden {
		t.Fatalf("foreign caller status = %d, want 403", rec.Code)
	}
	if rec := get("127.0.0.1:4000", DebugPprofPrefix+"goroutine?debug=1"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "goroutine") {
		t.Fatalf("goroutine profile status=%d body=%.80q", rec.Code, rec.Body.String())
	}
	for _, name := range []string{"mutex", "block", "heap"} {
		if rec := get("127.0.0.1:4000", DebugPprofPrefix+name+"?debug=1"); rec.Code != http.StatusOK {
			t.Fatalf("%s profile status = %d", name, rec.Code)
		}
	}
	if rec := get("127.0.0.1:4000", DebugPprofPrefix+"profile?seconds=1"); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("cpu profile status=%d bytes=%d", rec.Code, rec.Body.Len())
	}
	if rec := get("127.0.0.1:4000", DebugPprofPrefix+"no-such-profile"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown profile status = %d, want 404", rec.Code)
	}
	if rec := get("127.0.0.1:4000", DebugPprofPrefix); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "mutex") {
		t.Fatalf("index status=%d", rec.Code)
	}
}
