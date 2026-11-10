package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// A bound guest reads one published version by id (Texture's take, merge and
// fork); an unbound caller, a write, a deeper path and every neighbouring
// publications route stay refused.
func TestGuestPublicationVersionReadCarveOutIsNarrow(t *testing.T) {
	var reached []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.Method+" "+r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{"publication_version_id": "pub-a-v1"})
	}))
	defer corpusd.Close()
	guests := map[string]vmctl.GuestOwnership{"10.200.1.2": {Found: true, UserID: "reader-owner", ComputerID: "computer-reader"}}
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RemoteAddr string `json:"remote_addr"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(guests[strings.Split(req.RemoteAddr, ":")[0]])
	}))
	defer vmctlServer.Close()
	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)
	send := func(method, path, from string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader("{}"))
		req.RemoteAddr = from + ":40000"
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	const path = "/internal/platform/publications/versions/pub-a-v1"
	if rec := send(http.MethodGet, path, "10.200.1.2"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "pub-a-v1") {
		t.Fatalf("a bound guest's version read = %d %s", rec.Code, rec.Body)
	}
	if len(reached) != 1 || reached[0] != "GET "+path {
		t.Fatalf("corpusd saw %v", reached)
	}
	reached = nil
	// Not bound to a live computer, or a foreign address.
	for _, from := range []string{"10.200.77.2", "203.0.113.9"} {
		if rec := send(http.MethodGet, path, from); rec.Code != http.StatusForbidden {
			t.Fatalf("an unbound caller (%s) read a version: %d", from, rec.Code)
		}
	}
	// Not a read.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := send(method, path, "10.200.1.2"); rec.Code != http.StatusForbidden {
			t.Fatalf("%s of a version by a guest = %d, want 403", method, rec.Code)
		}
	}
	// One segment only; no neighbouring publications route is opened.
	for _, other := range []string{
		"/internal/platform/publications/versions/",
		"/internal/platform/publications/versions/pub-a-v1/extra",
		"/internal/platform/publications/versions/a%2Fb",
		"/internal/platform/publications/owned?owner=reader-owner",
		"/internal/platform/publications/front",
		"/internal/platform/publications/export?route=/pub/texture/t",
	} {
		req := httptest.NewRequest(http.MethodGet, other, nil)
		req.RemoteAddr = "10.200.1.2:40000"
		if err := handler.requirePlatformObjectGraphCaller(req); err == nil {
			t.Fatalf("guest GET %s passed the object graph caller check", other)
		}
	}
	if len(reached) != 0 {
		t.Fatalf("corpusd was reached by a refused call: %v", reached)
	}
}
