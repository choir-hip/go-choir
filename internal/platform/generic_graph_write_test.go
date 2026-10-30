package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// docs/problems/guest-can-write-public-routes-2026-10-30.md: a guest reaches
// corpusd directly at its platform URL, and the store stamps every write as
// corpusd's, so this handler is the gate. No guest writes the shared graph
// through the generic object and edge routes. Ways this could fail:
//  1. a bound guest files a public route or a publication under its own owner;
//  2. a bound guest files any other kind;
//  3. a bound guest links two objects it owns (a route to a publication);
//  4. the host's own writes through these routes stop working.
func TestGenericGraphWritesAreHostOnly(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, mem)
	guest := &vmctl.GuestOwnership{Found: true, UserID: "owner-a", ComputerID: "computer-a1"}
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": guest}); err != nil {
		t.Fatal(err)
	}
	send := func(handle http.HandlerFunc, method, path, from string, internal bool, body any) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.RemoteAddr = from + ":40000"
		if internal {
			req.Header.Set("X-Internal-Caller", "true")
		}
		rec := httptest.NewRecorder()
		handle(rec, req)
		return rec
	}
	object := func(kind, suffix string, meta map[string]any) objectgraph.Object {
		id, _ := objectgraph.BuildCanonicalID(objectgraph.ObjectKind(kind), "owner-a", suffix)
		normalized, _ := objectgraph.NormalizeMetadata(meta)
		return objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.ObjectKind(kind), OwnerID: "owner-a", ComputerID: "computer-a1",
			Metadata: normalized, ContentHash: objectgraph.ContentHash(objectgraph.ObjectKind(kind), nil, normalized)}
	}
	route := object("choir.public_route", "forged", map[string]any{"route_path": "/pub/texture/story", "state": "active"})
	publication := object("choir.publication", "forged", map[string]any{"state": "published"})

	for _, c := range []struct {
		name, method string
		body         any
	}{
		{"put route", http.MethodPut, route},
		{"put publication", http.MethodPut, publication},
		{"create publication", http.MethodPost, map[string]any{"kind": "choir.publication", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "p", "metadata": map[string]any{"state": "published"}}},
		{"create other kind", http.MethodPost, map[string]any{"kind": "choir.source_entity", "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k", "metadata": map[string]any{}}},
	} {
		if rec := send(h.HandleObjects, c.method, "/internal/platform/objects", "10.200.1.2", false, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s by a bound guest = %d %s, want 403", c.name, rec.Code, rec.Body)
		}
	}
	if _, err := svc.GetObject(t.Context(), route.CanonicalID); err == nil {
		t.Fatal("a guest's route was written through the generic route")
	}

	// The host's own writes still pass; then a guest may not link the objects
	// even though it owns both ends.
	for _, obj := range []objectgraph.Object{route, publication} {
		if rec := send(h.HandleObjects, http.MethodPut, "/internal/platform/objects", "127.0.0.1", true, obj); rec.Code != http.StatusOK {
			t.Fatalf("host put of %s = %d %s", obj.ObjectKind, rec.Code, rec.Body)
		}
	}
	edge := objectgraph.Edge{EdgeID: "edge-forged", FromID: route.CanonicalID, ToID: publication.CanonicalID, Kind: "routes_to"}
	if rec := send(h.HandleEdges, http.MethodPut, "/internal/platform/edges", "10.200.1.2", false, edge); rec.Code != http.StatusForbidden {
		t.Errorf("guest put edge = %d %s, want 403", rec.Code, rec.Body)
	}
	create := map[string]any{"from_id": route.CanonicalID, "to_id": publication.CanonicalID, "kind": "routes_to"}
	if rec := send(h.HandleEdges, http.MethodPost, "/internal/platform/edges", "10.200.1.2", false, create); rec.Code != http.StatusForbidden {
		t.Errorf("guest create edge = %d %s, want 403", rec.Code, rec.Body)
	}
	if rec := send(h.HandleEdges, http.MethodPut, "/internal/platform/edges", "127.0.0.1", true, edge); rec.Code != http.StatusOK {
		t.Fatalf("host put edge = %d %s", rec.Code, rec.Body)
	}
}
