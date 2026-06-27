package objectgraph

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCanonicalIDRoundTripPreservesOwnerWithColon(t *testing.T) {
	id, err := BuildCanonicalID("choir.source_entity", "user:alice", "sha256-abc")
	if err != nil {
		t.Fatalf("BuildCanonicalID() error = %v", err)
	}
	if strings.Contains(id, "user:alice") {
		t.Fatalf("canonical id leaked raw owner component: %s", id)
	}
	kind, owner, suffix, err := ParseCanonicalID(id)
	if err != nil {
		t.Fatalf("ParseCanonicalID() error = %v", err)
	}
	if kind != "choir.source_entity" || owner != "user:alice" || suffix != "sha256-abc" {
		t.Fatalf("parsed (%s, %s, %s), want source kind/user:alice/sha256-abc", kind, owner, suffix)
	}
}

func TestContentHashNormalizesMetadataOrder(t *testing.T) {
	a, err := NormalizeMetadata(map[string]any{"b": 2, "a": 1})
	if err != nil {
		t.Fatal(err)
	}
	b := json.RawMessage(`{"a":1,"b":2}`)
	if ContentHash("choir.source_entity", []byte("body"), a) != ContentHash("choir.source_entity", []byte("body"), b) {
		t.Fatal("content hash should be stable across metadata key ordering")
	}
}

func TestDefaultRegistryIncludesNewsAndAutoradioKinds(t *testing.T) {
	registry := DefaultRegistry()
	for _, kind := range []ObjectKind{
		"choir.source_entity",
		"choir.source_ref",
		"choir.web_capture",
		"choir.universal_wire_story_cluster",
		"choir.media_item",
		"choir.audio_recording",
		"choir.transcript",
		"choir.autoradio_run_sheet",
	} {
		if _, err := registry.LookupKind(kind); err != nil {
			t.Fatalf("LookupKind(%s) error = %v", kind, err)
		}
	}
}

func TestServiceCreatesDeterministicContentAddressedObjects(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(Config{Memory: store, Durable: store})
	defer svc.Close()

	req := CreateObjectRequest{
		Kind:     "choir.source_entity",
		OwnerID:  "user:alice",
		Body:     []byte("source body"),
		Metadata: map[string]any{"display_title": "A", "kind": "web_url"},
		Now:      time.Unix(1, 0),
	}
	first, err := svc.CreateObject(ctx, req)
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	second, err := svc.CreateObject(ctx, req)
	if err != nil {
		t.Fatalf("CreateObject() second error = %v", err)
	}
	if first.CanonicalID != second.CanonicalID {
		t.Fatalf("same content produced different canonical IDs: %s != %s", first.CanonicalID, second.CanonicalID)
	}
	if first.ContentHash != second.ContentHash || !strings.HasPrefix(first.ContentHash, "sha256:") {
		t.Fatalf("unstable content hash: %s %s", first.ContentHash, second.ContentHash)
	}
}

func TestServiceExternalIdentityKeepsIDWhileContentChanges(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	registry.RegisterKind(KindRegistration{Kind: "choir.autoradio_run_sheet", Store: StoreTypeMemory, IdentityMode: IdentityExternalKey, Versioned: true})
	store := NewMemoryStore()
	svc := NewService(Config{Registry: registry, Memory: store, Durable: store})
	defer svc.Close()

	first, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.autoradio_run_sheet", OwnerID: "user:alice", IdentityKey: "station:morning", Body: []byte("v1")})
	if err != nil {
		t.Fatalf("CreateObject() error = %v", err)
	}
	second, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.autoradio_run_sheet", OwnerID: "user:alice", IdentityKey: "station:morning", Body: []byte("v2")})
	if err != nil {
		t.Fatalf("CreateObject() second error = %v", err)
	}
	if first.CanonicalID != second.CanonicalID {
		t.Fatalf("external identity did not preserve canonical ID: %s != %s", first.CanonicalID, second.CanonicalID)
	}
	if first.ContentHash == second.ContentHash {
		t.Fatal("changed body should produce a new content hash")
	}
}

func TestMemoryStoreObjectsAndEdges(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(Config{Memory: store, Durable: store})
	defer svc.Close()

	source, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.source_entity", OwnerID: "user:alice", Body: []byte("source")})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.source_ref", OwnerID: "user:alice", Metadata: map[string]any{"offset": 4}})
	if err != nil {
		t.Fatal(err)
	}
	edge, err := svc.PutEdge(ctx, ref.CanonicalID, source.CanonicalID, "cites", map[string]any{"display_mode": "numbered_ref"})
	if err != nil {
		t.Fatalf("PutEdge() error = %v", err)
	}
	if !strings.HasPrefix(edge.EdgeID, "edge:cites:") {
		t.Fatalf("unexpected edge id: %s", edge.EdgeID)
	}
	edges, err := svc.ListEdges(ctx, EdgeFilter{FromID: ref.CanonicalID, Kind: "cites"})
	if err != nil {
		t.Fatalf("ListEdges() error = %v", err)
	}
	if len(edges) != 1 || edges[0].ToID != source.CanonicalID {
		t.Fatalf("edges = %#v, want citation to source", edges)
	}
}

func TestSQLiteStoreObjectsAndEdges(t *testing.T) {
	ctx := context.Background()
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "objectgraph.db"))
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	svc := NewService(Config{Durable: store})
	defer svc.Close()

	source, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.source_entity", OwnerID: "user:alice", Body: []byte("source")})
	if err != nil {
		t.Fatal(err)
	}
	capture, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.web_capture", OwnerID: "user:alice", Body: []byte("<html></html>"), Metadata: map[string]any{"url": "https://example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PutEdge(ctx, capture.CanonicalID, source.CanonicalID, "captured_from", nil); err != nil {
		t.Fatalf("PutEdge() error = %v", err)
	}
	objects, err := svc.ListObjects(ctx, ListFilter{Kind: "choir.web_capture", OwnerID: "user:alice"})
	if err != nil {
		t.Fatalf("ListObjects() error = %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("got %d web_capture objects, want 1", len(objects))
	}
}

func TestCreateWebCaptureUsesTypedMetadataAndDeterministicIdentity(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(Config{Memory: store, Durable: store})
	defer svc.Close()

	req := CreateWebCaptureRequest{
		OwnerID:             "user:alice",
		ComputerID:          "computer:local",
		URL:                 "https://example.com/story?utm_source=wire#section",
		CanonicalURL:        "https://example.com/story",
		Title:               "Example story",
		FetchedAt:           time.Date(2026, 6, 26, 10, 11, 12, 0, time.FixedZone("offset", -4*60*60)),
		ContentBlobID:       "blob:raw-html",
		ExtractedTextBlobID: "blob:extracted-text",
		EmbeddingModel:      "test-embed",
		EmbeddingVersion:    "v1",
		ExtractedText:       []byte("Durable extracted text for News indexing."),
		Now:                 time.Unix(20, 0),
	}
	first, err := svc.CreateWebCapture(ctx, req)
	if err != nil {
		t.Fatalf("CreateWebCapture() error = %v", err)
	}
	second, err := svc.CreateWebCapture(ctx, req)
	if err != nil {
		t.Fatalf("CreateWebCapture() second error = %v", err)
	}
	if first.CanonicalID != second.CanonicalID {
		t.Fatalf("web capture canonical id changed for same capture: %s != %s", first.CanonicalID, second.CanonicalID)
	}
	if first.ObjectKind != WebCaptureObjectKind || first.OwnerID != "user:alice" || string(first.Body) != string(req.ExtractedText) {
		t.Fatalf("web capture object lost graph fields: %#v", first)
	}
	metadata, err := WebCaptureMetadataFromObject(first)
	if err != nil {
		t.Fatalf("WebCaptureMetadataFromObject() error = %v", err)
	}
	if metadata.SchemaVersion != WebCaptureSchemaVersion ||
		metadata.URL != "https://example.com/story?utm_source=wire" ||
		metadata.CanonicalURL != "https://example.com/story" ||
		metadata.FetchedAt != "2026-06-26T14:11:12Z" ||
		metadata.ContentBlobID != "blob:raw-html" ||
		metadata.ExtractedTextBlobID != "blob:extracted-text" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestCreateWebCaptureRejectsIncompleteMetadata(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(Config{Memory: store, Durable: store})
	defer svc.Close()

	_, err := svc.CreateWebCapture(ctx, CreateWebCaptureRequest{
		OwnerID:             "user:alice",
		URL:                 "https://example.com/story",
		FetchedAt:           time.Unix(20, 0),
		ExtractedTextBlobID: "blob:extracted-text",
	})
	if err == nil || !strings.Contains(err.Error(), "content_blob_id is required") {
		t.Fatalf("CreateWebCapture() error = %v, want missing content_blob_id", err)
	}

	_, err = svc.CreateWebCapture(ctx, CreateWebCaptureRequest{
		OwnerID:             "user:alice",
		URL:                 "ftp://example.com/story",
		FetchedAt:           time.Unix(20, 0),
		ContentBlobID:       "blob:raw-html",
		ExtractedTextBlobID: "blob:extracted-text",
	})
	if err == nil || !strings.Contains(err.Error(), "url must use http or https") {
		t.Fatalf("CreateWebCapture() error = %v, want URL scheme validation", err)
	}
}

func TestWebCapturePersistsWithCapturedFromEdge(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "objectgraph.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	svc := NewService(Config{Durable: store})

	source, err := svc.CreateObject(ctx, CreateObjectRequest{
		Kind:    "choir.source_entity",
		OwnerID: "universal-wire-platform",
		Metadata: map[string]any{
			"schema_version": "choir.source_entity.v1",
			"source_kind":    "web_url",
			"target":         map[string]any{"kind": "url", "identity": "https://example.com/story"},
		},
	})
	if err != nil {
		t.Fatalf("CreateObject(source_entity) error = %v", err)
	}
	capture, err := svc.CreateWebCapture(ctx, CreateWebCaptureRequest{
		OwnerID:             "universal-wire-platform",
		URL:                 "https://example.com/story",
		Title:               "Example story",
		FetchedAt:           time.Unix(30, 0),
		ContentBlobID:       "blob:raw-html",
		ExtractedTextBlobID: "blob:extracted-text",
		ExtractedText:       []byte("A capture body that a later feed query can project."),
	})
	if err != nil {
		t.Fatalf("CreateWebCapture() error = %v", err)
	}
	edge, err := svc.PutEdge(ctx, capture.CanonicalID, source.CanonicalID, "captured_from", map[string]any{"relation": "original_web_source"})
	if err != nil {
		t.Fatalf("PutEdge(captured_from) error = %v", err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopenedStore, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen SQLite store: %v", err)
	}
	reopenedSvc := NewService(Config{Durable: reopenedStore})
	defer reopenedSvc.Close()

	got, err := reopenedSvc.GetObject(ctx, capture.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject(web_capture) after reopen error = %v", err)
	}
	metadata, err := WebCaptureMetadataFromObject(got)
	if err != nil {
		t.Fatalf("WebCaptureMetadataFromObject() after reopen error = %v", err)
	}
	if metadata.CanonicalURL != "https://example.com/story" || string(got.Body) != string(capture.Body) {
		t.Fatalf("reopened capture = metadata %#v body %q", metadata, got.Body)
	}
	edges, err := reopenedSvc.ListEdges(ctx, EdgeFilter{FromID: capture.CanonicalID, Kind: "captured_from"})
	if err != nil {
		t.Fatalf("ListEdges(captured_from) after reopen error = %v", err)
	}
	if len(edges) != 1 || edges[0].EdgeID != edge.EdgeID || edges[0].ToID != source.CanonicalID {
		t.Fatalf("edges after reopen = %#v, want captured_from %s", edges, source.CanonicalID)
	}
}

func TestSQLiteStoreReopenPreservesObjectsAndEdges(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "durable-objectgraph.db")

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	svc := NewService(Config{Durable: store})
	source, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.source_entity", OwnerID: "user:alice", Body: []byte("source")})
	if err != nil {
		t.Fatal(err)
	}
	transcript, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.transcript", OwnerID: "user:alice", Body: []byte("spoken words")})
	if err != nil {
		t.Fatal(err)
	}
	edge, err := svc.PutEdge(ctx, transcript.CanonicalID, source.CanonicalID, "references", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("reopen SQLite store: %v", err)
	}
	reopenedSvc := NewService(Config{Durable: reopened})
	defer reopenedSvc.Close()

	got, err := reopenedSvc.GetObject(ctx, transcript.CanonicalID)
	if err != nil {
		t.Fatalf("GetObject() after reopen error = %v", err)
	}
	if string(got.Body) != "spoken words" {
		t.Fatalf("body after reopen = %q", got.Body)
	}
	edges, err := reopenedSvc.ListEdges(ctx, EdgeFilter{FromID: transcript.CanonicalID})
	if err != nil {
		t.Fatalf("ListEdges() after reopen error = %v", err)
	}
	if len(edges) != 1 || edges[0].EdgeID != edge.EdgeID {
		t.Fatalf("edges after reopen = %#v, want %s", edges, edge.EdgeID)
	}
}

func TestServiceRejectsMissingEndpointForEdge(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(Config{Memory: store, Durable: store})
	defer svc.Close()
	source, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.source_entity", OwnerID: "user:alice", Body: []byte("source")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PutEdge(ctx, source.CanonicalID, "obj:choir.source_entity:dXNlcjphbGljZQ:sha256-missing", "references", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("PutEdge() error = %v, want ErrNotFound", err)
	}
}
