package proxy

// Ways research's access to published pieces can fail
// (docs/problems/oped-not-connected-2026-10-20.md):
//
//  F7. A published piece cannot be captured, because the only export is a
//      JSON envelope choir.Read refuses; research would cite a snippet.
//  F8. The carve-out that lets a guest search published pieces widens into a
//      way to call other corpusd routes, write, or reach it unbound.

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestPublicationExportRawIsPlainText(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		format := r.URL.Query().Get("format")
		out := platform.PublicationExport{RoutePath: "/pub/texture/t", Format: format, MediaType: "text/plain; charset=utf-8", Content: "The harbor reopened.\n\nFerries resume."}
		if format == "pdf" {
			out = platform.PublicationExport{RoutePath: "/pub/texture/t", Format: "pdf", MediaType: "application/pdf", ContentBase64: "AAAA"}
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	defer corpusd.Close()
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test", CorpusdURL: corpusd.URL}, pub)
	if err != nil {
		t.Fatal(err)
	}
	get := func(query string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.HandleAPI(w, httptest.NewRequest(http.MethodGet, "/api/platform/publications/export?route=/pub/texture/t"+query, nil))
		return w
	}
	raw := get("&format=txt&raw=1")
	if raw.Code != http.StatusOK || !strings.HasPrefix(raw.Header().Get("Content-Type"), "text/plain") ||
		raw.Body.String() != "The harbor reopened.\n\nFerries resume." {
		t.Fatalf("raw export = %d %q %q", raw.Code, raw.Header().Get("Content-Type"), raw.Body)
	}
	if got := get("&format=pdf&raw=1"); got.Code != http.StatusBadRequest {
		t.Fatalf("raw export of a binary format = %d, want 400", got.Code)
	}
	envelope := get("&format=txt")
	if envelope.Code != http.StatusOK || !strings.HasPrefix(envelope.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("export without raw must stay the JSON envelope: %d %q", envelope.Code, envelope.Header().Get("Content-Type"))
	}
}

func TestGuestPublishedSearchCarveOutIsNarrow(t *testing.T) {
	var reached []string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = append(reached, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		_ = json.NewEncoder(w).Encode(platform.PublishedPieceSearch{Query: r.URL.Query().Get("q"), Pieces: []platform.PublishedPiece{{Title: "A story", Kind: "story"}}})
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

	if rec := send(http.MethodGet, "/internal/platform/published/search?q=harbor&limit=5", "10.200.1.2"); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), "A story") {
		t.Fatalf("a bound guest's published search = %d %s", rec.Code, rec.Body)
	}
	if len(reached) != 1 || !strings.HasPrefix(reached[0], "GET /internal/platform/published/search?q=harbor") {
		t.Fatalf("corpusd saw %v", reached)
	}
	reached = nil
	// Not bound to a live computer.
	if rec := send(http.MethodGet, "/internal/platform/published/search?q=x", "10.200.77.2"); rec.Code != http.StatusForbidden {
		t.Fatalf("an unbound caller reached published search: %d", rec.Code)
	}
	// Not a read.
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		if rec := send(method, "/internal/platform/published/search?q=x", "10.200.1.2"); rec.Code != http.StatusForbidden {
			t.Fatalf("%s published search by a guest = %d, want 403", method, rec.Code)
		}
	}
	// The carve-out is one path: the checks refuse every other corpusd read.
	for _, path := range []string{
		"/internal/platform/retrieval/search?q=x",
		"/internal/platform/publications/owned?owner=reader-owner",
		"/internal/platform/publications/front",
		"/internal/platform/published/search/extra",
		"/internal/platform/published/",
		"/internal/platform/texture/publications",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "10.200.1.2:40000"
		if err := handler.requirePlatformObjectGraphCaller(req); err == nil {
			t.Fatalf("guest GET %s passed the object graph caller check", path)
		}
	}
	if len(reached) != 0 {
		t.Fatalf("corpusd was reached by a refused call: %v", reached)
	}
}
