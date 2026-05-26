package proxy

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestHandleVTextPublicationReadsPrivateRevisionAndPostsProjection(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishVTextRequest
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/vtext" {
			t.Fatalf("platformd path: got %s", r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("platformd missing internal caller header")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishVTextResponse{
			PublicationID:        "pub-1",
			PublicationVersionID: "pubver-1",
			RoutePath:            "/pub/vtext/my-note-pub1",
			ContentHash:          "hash",
			SourceRevisionHash:   "source-hash",
			State:                "published",
		})
	}))
	defer platformd.Close()

	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != "user-1" {
			t.Fatalf("sandbox trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/vtext/documents/doc-1":
			_ = json.NewEncoder(w).Encode(sandboxVTextDocument{
				DocID:             "doc-1",
				OwnerID:           "user-1",
				Title:             "My Note",
				CurrentRevisionID: "rev-head",
			})
		case "/api/vtext/revisions/rev-2":
			_ = json.NewEncoder(w).Encode(sandboxVTextRevision{
				RevisionID: "rev-2",
				DocID:      "doc-1",
				OwnerID:    "user-1",
				Content:    "public projection content",
				Citations:  json.RawMessage(`[{"url":"https://example.com"}]`),
			})
		default:
			t.Fatalf("sandbox path: got %s", r.URL.Path)
		}
	}))
	defer sandbox.Close()

	h, err := NewHandler(&Config{
		Port:              "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		PlatformdURL:      platformd.URL,
	}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/vtext/publications", strings.NewReader(`{"doc_id":"doc-1","revision_id":"rev-2","slug":"my-note"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	req.Header.Set("X-Authenticated-User", "attacker")
	w := httptest.NewRecorder()

	h.HandleVTextPublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if gotPlatformReq.OwnerID != "user-1" || gotPlatformReq.RequestedBy != "user-1" {
		t.Fatalf("platform owner/requester mismatch: %#v", gotPlatformReq)
	}
	if gotPlatformReq.SourceDocID != "doc-1" || gotPlatformReq.SourceRevisionID != "rev-2" {
		t.Fatalf("platform source mismatch: %#v", gotPlatformReq)
	}
	if gotPlatformReq.Content != "public projection content" {
		t.Fatalf("platform content: got %q", gotPlatformReq.Content)
	}
	var resp platform.PublishVTextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PublicURL != "https://choir.news/pub/vtext/my-note-pub1" {
		t.Fatalf("public url: got %q", resp.PublicURL)
	}
}
