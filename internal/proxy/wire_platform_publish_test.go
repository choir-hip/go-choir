package proxy

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
		"source":                     "edit_vtext",
		"revision_role":              wirepublish.RevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-proxy",
	})

	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != platformOwner {
			t.Fatalf("sandbox user header = %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/vtext/documents/doc-wire-proxy", "/api/texture/documents/doc-wire-proxy":
			_ = json.NewEncoder(w).Encode(sandboxVTextDocument{
				DocID:   "doc-wire-proxy",
				OwnerID: platformOwner,
				Title:   "Proxy story.vtext",
			})
		case "/internal/vtext/revisions/rev-wire-proxy", "/api/texture/revisions/rev-wire-proxy":
			_ = json.NewEncoder(w).Encode(sandboxVTextRevision{
				RevisionID: "rev-wire-proxy",
				DocID:      "doc-wire-proxy",
				OwnerID:    platformOwner,
				Content:    "# Proxy story\n\nMADRID -- Officials confirmed the route change.",
				Metadata:   meta,
			})
		case "/api/texture/documents/doc-wire-proxy/revisions":
			_ = json.NewEncoder(w).Encode([]sandboxVTextRevision{{RevisionID: "rev-wire-proxy", DocID: "doc-wire-proxy", OwnerID: platformOwner, Content: "# Proxy story", Metadata: meta}})
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
			var req platform.PublishVTextRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode platform request: %v", err)
			}
			if req.OwnerID != platformOwner || req.RequestedBy != wirepublish.RequestedByWirePolicy {
				t.Fatalf("platform request = %+v", req)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(platform.PublishVTextResponse{
				PublicationID: "pub-proxy",
				RoutePath:     "wire/proxy-story",
			})
		case "/internal/platform/texture/sync":
			// Async sync of all revisions — just accept it.
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
	var resp platform.PublishVTextResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RoutePath != "wire/proxy-story" {
		t.Fatalf("route_path = %q", resp.RoutePath)
	}
}
