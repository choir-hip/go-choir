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
				Metadata:   json.RawMessage(`{"source_entities":[{"entity_id":"src-1","kind":"legal_source","target":{"target_kind":"content_item","content_id":"content-public-1"},"display":{"inline_mode":"collapsed_citation"},"provenance":{"rights_scope":"public_source"}}]}`),
			})
		case "/api/content/items/content-public-1":
			_ = json.NewEncoder(w).Encode(sandboxContentItem{
				ContentID:    "content-public-1",
				OwnerID:      "user-1",
				SourceType:   "extracted_url",
				MediaType:    "text/html; charset=utf-8",
				AppHint:      "browser",
				Title:        "Public Source",
				SourceURL:    "https://example.com/source",
				CanonicalURL: "https://example.com/source",
				TextContent:  "Cleaned public source text that should be available to publication readers.",
				ContentHash:  "hash-public-source",
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
	if !strings.Contains(string(gotPlatformReq.Metadata), "content-public-1") {
		t.Fatalf("platform metadata not forwarded: %s", string(gotPlatformReq.Metadata))
	}
	if !strings.Contains(string(gotPlatformReq.Metadata), "reader_snapshot") || !strings.Contains(string(gotPlatformReq.Metadata), "Cleaned public source text") {
		t.Fatalf("platform metadata missing public source reader snapshot: %s", string(gotPlatformReq.Metadata))
	}
	var resp platform.PublishVTextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PublicURL != "https://choir.news/pub/vtext/my-note-pub1" {
		t.Fatalf("public url: got %q", resp.PublicURL)
	}
}

func TestHandleVTextPublicationPublishesPublicURLSourceSnapshots(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishVTextRequest
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/vtext" {
			t.Fatalf("platformd path: got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishVTextResponse{
			PublicationID:        "pub-url",
			PublicationVersionID: "pubver-url",
			RoutePath:            "/pub/vtext/url-note-pub1",
			ContentHash:          "hash",
			SourceRevisionHash:   "source-hash",
			State:                "published",
		})
	}))
	defer platformd.Close()

	var importCalled bool
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != "user-1" {
			t.Fatalf("sandbox trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/vtext/documents/doc-url":
			_ = json.NewEncoder(w).Encode(sandboxVTextDocument{
				DocID:             "doc-url",
				OwnerID:           "user-1",
				Title:             "URL Note",
				CurrentRevisionID: "rev-url",
			})
		case "/api/vtext/revisions/rev-url":
			_ = json.NewEncoder(w).Encode(sandboxVTextRevision{
				RevisionID: "rev-url",
				DocID:      "doc-url",
				OwnerID:    "user-1",
				Content:    "public projection content [1](source:src-url)",
				Metadata:   json.RawMessage(`{"source_entities":[{"entity_id":"src-url","kind":"legal_source","target":{"target_kind":"url","url":"https://example.com/source","canonical_url":"https://example.com/source"},"selectors":[{"selector_kind":"text_quote","text_quote":"bounded excerpt"}],"display":{"inline_mode":"embedded_excerpt","open_surface":"source"},"provenance":{"rights_scope":"public_url_snapshot"}}]}`),
			})
		case "/api/content/import-url":
			importCalled = true
			if r.Method != http.MethodPost {
				t.Fatalf("import method: got %s", r.Method)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode import body: %v", err)
			}
			if body["url"] != "https://example.com/source" {
				t.Fatalf("import url: got %q", body["url"])
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(sandboxContentItem{
				ContentID:    "content-url-1",
				OwnerID:      "user-1",
				SourceType:   "extracted_url",
				MediaType:    "text/html; charset=utf-8",
				AppHint:      "browser",
				Title:        "Imported URL Source",
				SourceURL:    "https://example.com/source",
				CanonicalURL: "https://example.com/source",
				TextContent:  "Cleaned URL source text that is longer than the bounded citation excerpt.",
				ContentHash:  "hash-url-source",
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

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/vtext/publications", strings.NewReader(`{"doc_id":"doc-url","revision_id":"rev-url","slug":"url-note"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleVTextPublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if !importCalled {
		t.Fatalf("expected public URL source import")
	}
	metadata := string(gotPlatformReq.Metadata)
	if !strings.Contains(metadata, "reader_snapshot") || !strings.Contains(metadata, "Cleaned URL source text") {
		t.Fatalf("platform metadata missing URL reader snapshot: %s", metadata)
	}
	if !strings.Contains(metadata, "bounded excerpt") {
		t.Fatalf("platform metadata lost bounded transclusion selector: %s", metadata)
	}
}

func TestHandleVTextPublicationDoesNotPublishPrivateSourceSnapshots(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishVTextRequest
	platformd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishVTextResponse{
			PublicationID:        "pub-private",
			PublicationVersionID: "pubver-private",
			RoutePath:            "/pub/vtext/private-note-pub",
			State:                "published",
		})
	}))
	defer platformd.Close()

	contentFetches := 0
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/vtext/documents/doc-private":
			_ = json.NewEncoder(w).Encode(sandboxVTextDocument{
				DocID:             "doc-private",
				OwnerID:           "user-1",
				Title:             "Private Note",
				CurrentRevisionID: "rev-private",
			})
		case "/api/vtext/revisions/rev-private":
			_ = json.NewEncoder(w).Encode(sandboxVTextRevision{
				RevisionID: "rev-private",
				DocID:      "doc-private",
				OwnerID:    "user-1",
				Content:    "public projection with private source excerpt",
				Metadata:   json.RawMessage(`{"source_entities":[{"entity_id":"src-private","kind":"client_note","target":{"target_kind":"content_item","content_id":"content-private-1"},"selectors":[{"selector_kind":"text_quote","text_quote":"bounded excerpt"}],"provenance":{"rights_scope":"private_user_source"}}]}`),
			})
		case "/api/content/items/content-private-1":
			contentFetches += 1
			_ = json.NewEncoder(w).Encode(sandboxContentItem{
				ContentID:   "content-private-1",
				OwnerID:     "user-1",
				TextContent: "private full source text",
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

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/vtext/publications", strings.NewReader(`{"doc_id":"doc-private","revision_id":"rev-private"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleVTextPublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if contentFetches != 0 {
		t.Fatalf("private source content was fetched for publication: %d", contentFetches)
	}
	if strings.Contains(string(gotPlatformReq.Metadata), "reader_snapshot") || strings.Contains(string(gotPlatformReq.Metadata), "private full source text") {
		t.Fatalf("private source snapshot leaked into platform metadata: %s", string(gotPlatformReq.Metadata))
	}
}

func TestContentItemAllowsPublishedSnapshotRejectsPrivateProvenance(t *testing.T) {
	if !contentItemAllowsPublishedSnapshot(sandboxContentItem{}) {
		t.Fatalf("empty provenance should not block an entity-level public publication decision")
	}
	if contentItemAllowsPublishedSnapshot(sandboxContentItem{
		Provenance: json.RawMessage(`{"rights_scope":"private_user_source"}`),
	}) {
		t.Fatalf("private_user_source content item must not publish a reader snapshot")
	}
	if !contentItemAllowsPublishedSnapshot(sandboxContentItem{
		Provenance: json.RawMessage(`{"rights_scope":"public_source"}`),
	}) {
		t.Fatalf("public_source content item should allow a reader snapshot")
	}
}
