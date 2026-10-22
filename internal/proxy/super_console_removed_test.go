package proxy

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// The Management Console's zot session is removed
// (docs/problems/remove-zot-2026-10-22.md). Its route answers 404 at the
// proxy, signed in or not, and never reaches a guest: a guest still on an
// old image keeps the route until it is refreshed, so the generic /api/*
// forward must not carry the request there.
func TestSuperConsoleRouteRemoved(t *testing.T) {
	handler, priv, autoputer := testProxyEnv(t)
	var downstreamCalls atomic.Int64
	autoputer.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downstreamCalls.Add(1)
		http.Error(w, "must not be reached", http.StatusInternalServerError)
	})

	for _, test := range []struct {
		name   string
		path   string
		signed bool
	}{
		{name: "anonymous", path: "/api/super-console/ws"},
		{name: "signed in", path: "/api/super-console/ws", signed: true},
		{name: "signed in with desktop", path: "/api/super-console/ws?desktop_id=primary", signed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			req.Header.Set("Connection", "Upgrade")
			req.Header.Set("Upgrade", "websocket")
			if test.signed {
				req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "owner-1")})
			}
			rec := httptest.NewRecorder()
			handler.HandleAPI(rec, req)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
			}
			if downstreamCalls.Load() != 0 {
				t.Fatalf("downstream calls=%d want 0", downstreamCalls.Load())
			}
		})
	}
}
