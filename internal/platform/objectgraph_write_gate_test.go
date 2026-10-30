package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Store B refuses an unregistered object or edge kind on every write
// method and on the HTTP PUT route (H-OG1, failure mode F5 in
// docs/archive/problems/object-graph-registration-and-anchor-gaps-2026-10-14.md).
func TestObjectGraphStore_RefusesUnregisteredKinds(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()
	object := func(kind objectgraph.ObjectKind, key string) objectgraph.Object {
		id, err := objectgraph.BuildCanonicalID(kind, "gate-owner", objectgraph.StableSuffixFromKey(key))
		if err != nil {
			t.Fatal(err)
		}
		return objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "gate-owner", ContentHash: "sha256:" + key, Metadata: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now}
	}
	good := object("choir.publication_check", "good")
	bad := object("choir.not_a_kind", "bad")
	badEdge := objectgraph.Edge{EdgeID: "edge:not_an_edge:x", FromID: good.CanonicalID, ToID: good.CanonicalID, Kind: "not_an_edge", Metadata: json.RawMessage(`{}`), CreatedAt: now}

	if err := og.PutObject(ctx, bad); !errors.Is(err, objectgraph.ErrUnregisteredKind) {
		t.Errorf("PutObject: err = %v", err)
	}
	if _, err := og.InsertObjectIfAbsent(ctx, bad); !errors.Is(err, objectgraph.ErrUnregisteredKind) {
		t.Errorf("InsertObjectIfAbsent: err = %v", err)
	}
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{good, bad}}); !errors.Is(err, objectgraph.ErrUnregisteredKind) {
		t.Errorf("PutBatch object: err = %v", err)
	}
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{good}, Edges: []objectgraph.Edge{badEdge}}); !errors.Is(err, objectgraph.ErrUnregisteredKind) {
		t.Errorf("PutBatch edge: err = %v", err)
	}
	if err := og.PutEdge(ctx, badEdge); !errors.Is(err, objectgraph.ErrUnregisteredKind) {
		t.Errorf("PutEdge: err = %v", err)
	}
	// A refused batch is refused whole: the registered object in it did
	// not land.
	if _, err := og.GetObject(ctx, good.CanonicalID); !errors.Is(err, objectgraph.ErrNotFound) {
		t.Fatalf("refused batch wrote its registered object: err = %v", err)
	}

	handler := NewObjectGraphHandler(objectgraph.NewService(objectgraph.Config{Durable: og}), og)
	body, _ := json.Marshal(bad)
	req := httptest.NewRequest(http.MethodPut, "/internal/platform/objects", bytes.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleObjects(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT unregistered kind: status %d, want 400 (%s)", rec.Code, rec.Body.String())
	}

	// The registered kinds the publication path writes still land.
	if err := og.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{good, object("choir.publication_resolved_blocks", "rb"), object("choir.support_judgement", "sj")}}); err != nil {
		t.Fatalf("registered publication kinds refused: %v", err)
	}
}
