package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// docs/problems/guest-can-write-public-routes-2026-10-30.md: corpusd sees a
// relayed guest write as the proxy's, and stamps every write as its own, so
// the proxy is the only gate. No guest code writes the shared graph through
// the generic object or edge routes; a guest that did could file a public
// route or a publication without the publish path. Ways this could fail:
//  1. a guest PUT or POST of a corpusd kind (a route, a publication) reaches corpusd;
//  2. a guest POST of any other kind reaches corpusd;
//  3. a guest edge write reaches corpusd;
//  4. a guest delete of an object reaches corpusd.
func TestGuestCannotWriteTheSharedGraphThroughGenericRoutes(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	og := platform.NewObjectGraphHandler(svc, mem)
	reached := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/platform/objects", func(w http.ResponseWriter, r *http.Request) { reached++; og.HandleObjects(w, r) })
	mux.HandleFunc("/internal/platform/objects/", func(w http.ResponseWriter, r *http.Request) { reached++; og.HandleObjects(w, r) })
	mux.HandleFunc("/internal/platform/edges", func(w http.ResponseWriter, r *http.Request) { reached++; og.HandleEdges(w, r) })
	corpusd := httptest.NewServer(mux)
	defer corpusd.Close()
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.GuestOwnership{Found: true, UserID: "owner-a", ComputerID: "computer-a1"})
	}))
	defer vmctlServer.Close()
	handler, _, _ := testProxyEnv(t)
	handler.cfg.CorpusdURL = corpusd.URL
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	srv := server.NewServer("proxy-test", "0")
	RegisterRoutes(srv, handler)

	send := func(method, path string, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.RemoteAddr = "10.200.1.2:40000"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}
	routeMeta, _ := objectgraph.NormalizeMetadata(map[string]any{"route_path": "/pub/texture/story", "target_kind": "publication", "target_id": "x", "state": "active"})
	routeID, _ := objectgraph.BuildCanonicalID("choir.public_route", "owner-a", "forged")
	route := objectgraph.Object{CanonicalID: routeID, ObjectKind: "choir.public_route", OwnerID: "owner-a", ComputerID: "computer-a1",
		Metadata: routeMeta, ContentHash: objectgraph.ContentHash("choir.public_route", nil, routeMeta)}
	for _, c := range []struct {
		name, method, path string
		body               any
	}{
		{"put route", http.MethodPut, "/internal/platform/objects", route},
		{"create publication", http.MethodPost, "/internal/platform/objects", map[string]any{"kind": "choir.publication", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "p", "metadata": map[string]any{"state": "published"}}},
		{"create other kind", http.MethodPost, "/internal/platform/objects", map[string]any{"kind": "choir.source_entity", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k", "metadata": map[string]any{}}},
		{"put edge", http.MethodPut, "/internal/platform/edges", objectgraph.Edge{FromID: routeID, ToID: routeID, Kind: "routes_to"}},
		{"delete object", http.MethodDelete, "/internal/platform/objects/" + routeID, nil},
	} {
		if rec := send(c.method, c.path, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s: a guest write passed the proxy: %d %s", c.name, rec.Code, rec.Body)
		}
	}
	if reached != 0 {
		t.Fatalf("corpusd was reached %d times by refused guest writes", reached)
	}
	if _, err := svc.GetObject(t.Context(), routeID); err == nil {
		t.Fatal("a forged route was written through the generic route")
	}
}
