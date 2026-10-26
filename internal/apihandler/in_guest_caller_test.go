package apihandler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// An in-guest process (a capsule workload, any guest child) reaches the guest
// on loopback and can set any header. It must not act as the owner on any
// owner route, nor drive the internal routes that write the planned-restart
// marker, hibernate, append run events or read Texture documents
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2, F6 c).
func TestInGuestLoopbackCallerCannotActAsOwner(t *testing.T) {
	t.Setenv(server.EnvGuestTrustLoopback, "")
	srv := server.NewServer("apihandler-in-guest-test", "0")
	registerRoutesForTest(t, srv, false)

	routes := []struct{ method, path, body string }{
		{http.MethodGet, "/api/costs", ""},
		{http.MethodGet, "/api/runs", ""},
		{http.MethodPost, "/api/prompt-bar", `{"prompt":"x"}`},
		{http.MethodGet, "/api/trajectories", ""},
		{http.MethodGet, "/api/texture/documents", ""},
		{http.MethodPost, "/api/texture/documents", `{"title":"x"}`},
		{http.MethodPost, "/api/content/items", `{"title":"x"}`},
		{http.MethodGet, "/api/desktop/state", ""},
		{http.MethodGet, "/api/media/progress", ""},
		{http.MethodGet, "/api/preferences/theme", ""},
		{http.MethodGet, "/api/precommit/claims", ""},
		{http.MethodPost, "/api/computers/routes-test/lifecycle/restore", `{}`},
		{http.MethodPost, "/api/computers/routes-test/lifecycle/bootstrap-chain", `{}`},
		{http.MethodPost, "/api/computers/routes-test/lifecycle/import-residue-snapshot", `{}`},
		{http.MethodPost, "/api/computers/routes-test/self-development/operations/op-1/decision", `{"decision":"approve"}`},
		{http.MethodPost, "/internal/runtime/planned-restart", `{"reason":"forged","target":"x"}`},
		{http.MethodPost, "/internal/runtime/hibernate/prepare", `{}`},
		{http.MethodPost, "/internal/runtime/memory/cache-return", `{"mode":"idle","scope":"daemon"}`},
		{http.MethodPost, "/internal/runtime/platform-update", `{}`},
		{http.MethodPost, "/internal/runtime/runs/run-1/events?owner_id=owner-1", `{}`},
		{http.MethodGet, "/internal/texture/documents/document-1", ""},
		{http.MethodGet, "/internal/texture/revisions/revision-1", ""},
	}
	for _, route := range routes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
		req.RemoteAddr = "127.0.0.1:40000"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Authenticated-User", "owner-1")
		req.Header.Set("X-Authenticated-Computer", "routes-test")
		req.Header.Set("X-Internal-Caller", "true")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
			t.Errorf("%s %s from in-guest loopback = %d %s, want 401 or 403", route.method, route.path, rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	}
}
