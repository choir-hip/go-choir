package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/hostauth"
)

// The server strips the credential before any handler sees it, and pprof
// needs it on the host (docs/problems/loopback-is-a-credential-2026-10-14.md).
func TestServerStripsCredentialAndGatesPprof(t *testing.T) {
	const token = "server-gate-test-token-0123456789abcdef012345678"
	restore := hostauth.SetForTesting(hostauth.ModeEnforce, token)
	defer restore()
	s := NewServer("test", "0")
	var seen string
	s.HandleFunc("/internal/echo", func(w http.ResponseWriter, r *http.Request) { seen = r.Header.Get(hostauth.Header) })

	req := httptest.NewRequest(http.MethodGet, "/internal/echo", nil)
	req.Header.Set(hostauth.Header, token)
	s.ServeHTTP(httptest.NewRecorder(), req)
	if seen != "" {
		t.Fatal("credential header reached the handler")
	}

	for _, tc := range []struct {
		token string
		want  int
	}{{"", http.StatusForbidden}, {token, http.StatusOK}} {
		req := httptest.NewRequest(http.MethodGet, DebugPprofPrefix, nil)
		req.RemoteAddr = "127.0.0.1:9999"
		if tc.token != "" {
			req.Header.Set(hostauth.Header, tc.token)
		}
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Fatalf("pprof with token=%v: code %d, want %d", tc.token != "", rec.Code, tc.want)
		}
	}
}
