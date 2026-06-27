package proxy

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

func TestHandleInternalWirePlatformPublishPostsToPlatformd(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	platformOwner := wirepublish.PlatformOwnerID()
	meta, _ := json.Marshal(map[string]any{
		"source":                     "edit_texture",
		"revision_role":              wirepublish.RevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-proxy",
	})
	bodyDoc := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Proxy story"},{"type":"source_ref","attrs":{"source_entity_id":"src-proxy"}}]}]}}`)
	sourceEntities := json.RawMessage(`[{"source_entity_id":"src-proxy","target":{"kind":"url","uri":"https://example.com/proxy"},"display":{"mode":"numbered_ref","title":"Proxy source"},"evidence":{"state":"available","open_surface":"source"}}]`)
	syncSeen := make(chan platform.SyncTextureDocumentRequest, 1)

	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != platformOwner {
			t.Fatalf("sandbox user header = %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/texture/documents/doc-wire-proxy", "/api/texture/documents/doc-wire-proxy":
			_ = json.NewEncoder(w).Encode(sandboxTextureDocument{
				DocID:   "doc-wire-proxy",
				OwnerID: platformOwner,
				Title:   "Proxy story.texture",
			})
		case "/internal/texture/revisions/rev-wire-proxy", "/api/texture/revisions/rev-wire-proxy":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevision{
				RevisionID:     "rev-wire-proxy",
				DocID:          "doc-wire-proxy",
				OwnerID:        platformOwner,
				Content:        "# Proxy story\n\nMADRID -- Officials confirmed the route change.",
				BodyDoc:        bodyDoc,
				SourceEntities: sourceEntities,
				Metadata:       meta,
			})
		case "/api/texture/documents/doc-wire-proxy/revisions":
			_ = json.NewEncoder(w).Encode([]sandboxTextureRevision{{RevisionID: "rev-wire-proxy", DocID: "doc-wire-proxy", OwnerID: platformOwner, Content: "# Proxy story", BodyDoc: bodyDoc, SourceEntities: sourceEntities, Metadata: meta}})
		default:
			// Async sync goroutine may hit unexpected paths; log instead of fatal.
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer sandbox.Close()

	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/platform/publications/texture":
			if r.Header.Get("X-Internal-Caller") != "true" {
				t.Fatalf("platformd internal header missing")
			}
			var req platform.PublishTextureRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode platform request: %v", err)
			}
			if req.OwnerID != platformOwner || req.RequestedBy != wirepublish.RequestedByWirePolicy {
				t.Fatalf("platform request = %+v", req)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{
				PublicationID: "pub-proxy",
				RoutePath:     "wire/proxy-story",
			})
		case "/internal/platform/texture/sync":
			var req platform.SyncTextureDocumentRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode sync request: %v", err)
			}
			syncSeen <- req
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"doc_id": "test", "revision_count": 0})
		default:
			t.Fatalf("platformd path = %s", r.URL.Path)
		}
	}))
	defer platformd.Close()

	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		PlatformdURL:      platformd.URL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"doc_id":         "doc-wire-proxy",
		"revision_id":    "rev-wire-proxy",
		"run_id":         "run-proxy",
		"request_intent": "universal_wire_processor_article_revision",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/wire/platform/publications/texture", bytes.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("X-Authenticated-User", platformOwner)
	w := httptest.NewRecorder()
	h.HandleInternalWirePlatformPublish(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var resp platform.PublishTextureResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RoutePath != "wire/proxy-story" {
		t.Fatalf("route_path = %q", resp.RoutePath)
	}
	select {
	case syncReq := <-syncSeen:
		if len(syncReq.Revisions) != 1 {
			t.Fatalf("sync revisions = %d, want 1", len(syncReq.Revisions))
		}
		if !strings.Contains(string(syncReq.Revisions[0].BodyDoc), `"source_ref"`) {
			t.Fatalf("sync body_doc not forwarded: %s", syncReq.Revisions[0].BodyDoc)
		}
		if !strings.Contains(string(syncReq.Revisions[0].SourceEntities), `"src-proxy"`) {
			t.Fatalf("sync source_entities not forwarded: %s", syncReq.Revisions[0].SourceEntities)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async platform texture sync")
	}
}

func TestHandleInternalWirePlatformPublishRejectsSourceEntitiesWithoutBodyDoc(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	platformOwner := wirepublish.PlatformOwnerID()
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer sandbox.Close()

	platformCalled := false
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		platformCalled = true
		t.Fatalf("platformd should not be called for detached source_entities")
	}))
	defer platformd.Close()

	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		PlatformdURL:      platformd.URL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	body, _ := json.Marshal(map[string]any{
		"doc_id":      "doc-wire-direct",
		"revision_id": "rev-wire-direct",
		"title":       "Wire Direct",
		"content":     "This direct payload carries detached source identity.",
		"source_entities": []map[string]any{{
			"source_entity_id": "src-detached-wire",
			"target":           map[string]any{"kind": "url", "uri": "https://example.com/source"},
			"display":          map[string]any{"mode": "numbered_ref", "title": "Detached source"},
			"evidence":         map[string]any{"state": "available", "open_surface": "source"},
			"provenance":       map[string]any{"created_by": "wire-test"},
		}},
		"run_id":         "run-wire-direct",
		"request_intent": "universal_wire_processor_article_revision",
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/wire/platform/publications/texture", bytes.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	req.Header.Set("X-Authenticated-User", platformOwner)
	w := httptest.NewRecorder()
	h.HandleInternalWirePlatformPublish(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "source_entities require body_doc") {
		t.Fatalf("body = %s, want body_doc requirement", w.Body.String())
	}
	if platformCalled {
		t.Fatalf("platformd was called for detached source_entities")
	}
}
