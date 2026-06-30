package platform

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestObjectGraphStore_PutAndGet(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ctx := context.Background()

	// Create a web capture object via the objectgraph Service so it gets
	// a proper canonical ID and content hash.
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})
	obj, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:    objectgraph.WebCaptureObjectKind,
		OwnerID: "test-owner",
		Body:    []byte("test body"),
		Metadata: map[string]any{
			"schema_version":       objectgraph.WebCaptureSchemaVersion,
			"url":                  "https://example.com/article",
			"canonical_url":        "https://example.com/article",
			"fetched_at":           time.Now().UTC().Format(time.RFC3339Nano),
			"content_blob_id":      "test:content",
			"extracted_text_blob_id": "test:extracted",
		},
	})
	if err != nil {
		t.Fatalf("CreateObject: %v", err)
	}
	if obj.CanonicalID == "" {
		t.Fatal("expected non-empty canonical_id")
	}

	// Retrieve it directly via the store.
	got, err := ogStore.GetObject(ctx, obj.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	if got.CanonicalID != obj.CanonicalID {
		t.Errorf("canonical_id: got %s, want %s", got.CanonicalID, obj.CanonicalID)
	}
	if got.ObjectKind != objectgraph.WebCaptureObjectKind {
		t.Errorf("object_kind: got %s, want %s", got.ObjectKind, objectgraph.WebCaptureObjectKind)
	}
	if got.OwnerID != "test-owner" {
		t.Errorf("owner_id: got %s, want test-owner", got.OwnerID)
	}
	if string(got.Body) != "test body" {
		t.Errorf("body: got %q, want %q", string(got.Body), "test body")
	}
}

func TestObjectGraphStore_ListObjects(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ctx := context.Background()
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})

	// Create two objects of the same kind.
	for i := 0; i < 2; i++ {
		_, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
			Kind:    objectgraph.WebCaptureObjectKind,
			OwnerID: "list-owner",
			Body:    []byte("body"),
			Metadata: map[string]any{
				"schema_version":       objectgraph.WebCaptureSchemaVersion,
				"url":                  "https://example.com/" + string(rune('a'+i)),
				"canonical_url":        "https://example.com/" + string(rune('a'+i)),
				"fetched_at":           time.Now().UTC().Format(time.RFC3339Nano),
				"content_blob_id":      "test:content",
				"extracted_text_blob_id": "test:extracted",
			},
		})
		if err != nil {
			t.Fatalf("CreateObject %d: %v", i, err)
		}
	}

	// List all web captures for this owner.
	objs, err := ogStore.ListObjects(ctx, objectgraph.ListFilter{
		Kind:    objectgraph.WebCaptureObjectKind,
		OwnerID: "list-owner",
	})
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if len(objs) != 2 {
		t.Errorf("expected 2 objects, got %d", len(objs))
	}
}

func TestObjectGraphStore_PutAndListEdges(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ctx := context.Background()
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})

	// Create two objects to link.
	capture, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:    objectgraph.WebCaptureObjectKind,
		OwnerID: "edge-owner",
		Body:    []byte("capture body"),
		Metadata: map[string]any{
			"schema_version":       objectgraph.WebCaptureSchemaVersion,
			"url":                  "https://example.com/cap",
			"canonical_url":        "https://example.com/cap",
			"fetched_at":           time.Now().UTC().Format(time.RFC3339Nano),
			"content_blob_id":      "test:content",
			"extracted_text_blob_id": "test:extracted",
		},
	})
	if err != nil {
		t.Fatalf("CreateObject capture: %v", err)
	}
	source, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:    "choir.source_entity",
		OwnerID: "edge-owner",
		Body:    []byte("source body"),
		Metadata: map[string]any{
			"schema_version": "choir.source_entity.v1",
		},
	})
	if err != nil {
		t.Fatalf("CreateObject source: %v", err)
	}

	// Create an edge.
	edge, err := ogService.PutEdge(ctx, capture.CanonicalID, source.CanonicalID, "captured_from", map[string]any{
		"relation": "test",
	})
	if err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if edge.EdgeID == "" {
		t.Fatal("expected non-empty edge_id")
	}

	// List edges from the capture.
	edges, err := ogStore.ListEdges(ctx, objectgraph.EdgeFilter{
		FromID: capture.CanonicalID,
	})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].FromID != capture.CanonicalID {
		t.Errorf("from_id: got %s, want %s", edges[0].FromID, capture.CanonicalID)
	}
	if edges[0].ToID != source.CanonicalID {
		t.Errorf("to_id: got %s, want %s", edges[0].ToID, source.CanonicalID)
	}
	if edges[0].Kind != "captured_from" {
		t.Errorf("kind: got %s, want captured_from", edges[0].Kind)
	}
}

func TestObjectGraphStore_GetObjectNotFound(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ctx := context.Background()

	_, err := ogStore.GetObject(ctx, "obj:nonexistent:owner:suffix")
	if err != objectgraph.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestObjectGraphHandler_CreateAndGet(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})
	handler := NewObjectGraphHandler(ogService)

	// POST an object.
	body := `{"kind":"choir.source_entity","owner_id":"handler-owner","body":"test","metadata":{"schema_version":"choir.source_entity.v1"}}`
	req := httptest.NewRequest(http.MethodPost, "/internal/platform/objects", strings.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleObjects(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status: got %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var created objectgraph.Object
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if created.CanonicalID == "" {
		t.Fatal("expected non-empty canonical_id")
	}

	// GET the object by ID.
	req2 := httptest.NewRequest(http.MethodGet, "/internal/platform/objects/"+created.CanonicalID, nil)
	req2.Header.Set("X-Internal-Caller", "true")
	rec2 := httptest.NewRecorder()
	handler.HandleObjectByID(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET status: got %d, want %d; body: %s", rec2.Code, http.StatusOK, rec2.Body.String())
	}
	var got objectgraph.Object
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.CanonicalID != created.CanonicalID {
		t.Errorf("canonical_id: got %s, want %s", got.CanonicalID, created.CanonicalID)
	}
}

func TestObjectGraphHandler_ListObjects(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})
	handler := NewObjectGraphHandler(ogService)
	ctx := context.Background()

	// Seed an object.
	_, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:     "choir.source_entity",
		OwnerID:  "list-handler-owner",
		Body:     []byte("seed"),
		Metadata: map[string]any{"schema_version": "choir.source_entity.v1"},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// GET list with kind filter.
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/objects?kind=choir.source_entity&owner=list-handler-owner", nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleObjects(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var objs []objectgraph.Object
	if err := json.Unmarshal(rec.Body.Bytes(), &objs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
}

func TestObjectGraphHandler_RequireInternalCaller(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})
	handler := NewObjectGraphHandler(ogService)

	// No X-Internal-Caller header → 403.
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/objects", nil)
	rec := httptest.NewRecorder()
	handler.HandleObjects(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestObjectGraphHandler_CreateEdge(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	ogStore := NewObjectGraphStore(store)
	ogService := objectgraph.NewService(objectgraph.Config{Durable: ogStore})
	handler := NewObjectGraphHandler(ogService)
	ctx := context.Background()

	// Seed two objects.
	capture, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:    objectgraph.WebCaptureObjectKind,
		OwnerID: "edge-handler-owner",
		Body:    []byte("cap"),
		Metadata: map[string]any{
			"schema_version":         objectgraph.WebCaptureSchemaVersion,
			"url":                    "https://example.com/edge",
			"canonical_url":          "https://example.com/edge",
			"fetched_at":             time.Now().UTC().Format(time.RFC3339Nano),
			"content_blob_id":        "test:content",
			"extracted_text_blob_id": "test:extracted",
		},
	})
	if err != nil {
		t.Fatalf("seed capture: %v", err)
	}
	source, err := ogService.CreateObject(ctx, objectgraph.CreateObjectRequest{
		Kind:     "choir.source_entity",
		OwnerID:  "edge-handler-owner",
		Body:     []byte("src"),
		Metadata: map[string]any{"schema_version": "choir.source_entity.v1"},
	})
	if err != nil {
		t.Fatalf("seed source: %v", err)
	}

	// POST an edge.
	body := `{"from_id":"` + capture.CanonicalID + `","to_id":"` + source.CanonicalID + `","kind":"captured_from","metadata":{"relation":"test"}}`
	req := httptest.NewRequest(http.MethodPost, "/internal/platform/edges", strings.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	handler.HandleEdges(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST edge status: got %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var edge objectgraph.Edge
	if err := json.Unmarshal(rec.Body.Bytes(), &edge); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if edge.FromID != capture.CanonicalID {
		t.Errorf("from_id: got %s, want %s", edge.FromID, capture.CanonicalID)
	}

	// GET edges from the capture.
	req2 := httptest.NewRequest(http.MethodGet, "/internal/platform/edges?from="+capture.CanonicalID, nil)
	req2.Header.Set("X-Internal-Caller", "true")
	rec2 := httptest.NewRecorder()
	handler.HandleEdges(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("GET edges status: got %d, want %d", rec2.Code, http.StatusOK)
	}
	var edges []objectgraph.Edge
	if err := json.Unmarshal(rec2.Body.Bytes(), &edges); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}
