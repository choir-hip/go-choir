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
		"source":                   "edit_vtext",
		"revision_role":            wirepublish.RevisionRoleCanonical,
		"ingestion_handoff_cycle_id": "cycle-proxy",
	})

	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != platformOwner {
			t.Fatalf("sandbox user header = %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/vtext/documents/doc-wire-proxy":
			_ = json.NewEncoder(w).Encode(sandboxVTextDocument{
				DocID:   "doc-wire-proxy",
				OwnerID: platformOwner,
				Title:   "Proxy story.vtext",
			})
		case "/api/vtext/revisions/rev-wire-proxy":
			_ = json.NewEncoder(w).Encode(sandboxVTextRevision{
				RevisionID: "rev-wire-proxy",
				DocID:      "doc-wire-proxy",
				OwnerID:    platformOwner,
				Content:    "# Proxy story\n\nMADRID -- Officials confirmed the route change.",
				Metadata:   meta,
			})
		default:
			t.Fatalf("unexpected sandbox path %s", r.URL.Path)
		}
	}))
	defer sandbox.Close()

	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/vtext" {
			t.Fatalf("platformd path = %s", r.URL.Path)
		}
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
	req := httptest.NewRequest(http.MethodPost, "/internal/wire/platform/publications/vtext", bytes.NewReader(body))
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
