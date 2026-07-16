package proxy

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
)

func testProxyTextureBodyDoc(t *testing.T, docID, sourceEntityID, label string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"schema": "choir.texture_doc.v1",
		"doc": map[string]any{
			"type":  "doc",
			"attrs": map[string]any{"id": docID},
			"content": []map[string]any{{
				"type":  "paragraph",
				"attrs": map[string]any{"id": "p-" + docID},
				"content": []map[string]any{
					{"type": "text", "text": "public projection content "},
					{"type": "source_ref", "attrs": map[string]any{"id": "ref-" + sourceEntityID, "source_entity_id": sourceEntityID, "label": label, "display_mode": "numbered_ref"}},
				},
			}},
		},
	})
	if err != nil {
		t.Fatalf("marshal body_doc: %v", err)
	}
	return raw
}

func testProxySourceEntities(t *testing.T, entities ...map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(entities)
	if err != nil {
		t.Fatalf("marshal source_entities: %v", err)
	}
	return raw
}

func testProxySourceEntity(id, targetKind, targetID, title, quote, rightsScope string) map[string]any {
	target := map[string]any{"kind": targetKind}
	if strings.HasPrefix(targetID, "http://") || strings.HasPrefix(targetID, "https://") {
		target["uri"] = targetID
	} else if targetID != "" {
		target["id"] = targetID
	}
	return map[string]any{
		"source_entity_id": id,
		"target":           target,
		"selectors": []map[string]any{{
			"kind": "text_quote",
			"data": map[string]any{
				"text_quote": quote,
				"exact":      quote,
			},
		}},
		"display": map[string]any{
			"mode":  "numbered_ref",
			"title": title,
			"label": title,
		},
		"evidence": map[string]any{
			"state":        "available",
			"open_surface": "source",
		},
		"provenance": map[string]any{
			"created_by":   "proxy-test",
			"rights_scope": rightsScope,
		},
	}
}

func TestHandleTexturePublicationReadsPrivateRevisionAndPostsProjection(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishTextureRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/texture" {
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("corpusd missing internal caller header")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{
			PublicationID:        "pub-1",
			PublicationVersionID: "pubver-1",
			RoutePath:            "/pub/texture/my-note-pub1",
			ContentHash:          "hash",
			SourceRevisionHash:   "source-hash",
			State:                "published",
		})
	}))
	defer corpusd.Close()

	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != "user-1" {
			t.Fatalf("sandbox trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-1":
			_ = json.NewEncoder(w).Encode(sandboxTextureDocument{
				DocID:             "doc-1",
				OwnerID:           "user-1",
				Title:             "My Note",
				CurrentRevisionID: "rev-head",
			})
		case "/api/texture/documents/doc-1/revisions":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevisionList{Revisions: []sandboxTextureRevision{
				{RevisionID: "rev-2", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 2, Content: "public projection content [1]", RevisionHash: "h2", CreatedAt: "2026-01-02T00:00:00.000Z"},
				{RevisionID: "rev-1", DocID: "doc-1", OwnerID: "user-1", VersionNumber: 1, Content: "older draft", RevisionHash: "h1", CreatedAt: "2026-01-01T00:00:00.000Z"},
			}})
		case "/api/texture/revisions/rev-2":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevision{
				RevisionID:     "rev-2",
				DocID:          "doc-1",
				OwnerID:        "user-1",
				Content:        "public projection content [1]",
				BodyDoc:        testProxyTextureBodyDoc(t, "doc-1", "src-1", "Public Source"),
				SourceEntities: testProxySourceEntities(t, testProxySourceEntity("src-1", "content_item", "content-public-1", "Public Source", "Cleaned public source text", "public_source")),
				Citations:      json.RawMessage(`[{"url":"https://example.com"}]`),
				Metadata:       json.RawMessage(`{}`),
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

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-1","revision_id":"rev-2","slug":"my-note","access_policy":{"visibility":"unlisted","route":"public"},"export_policy":{"copy_allowed":true,"download_allowed":false,"formats":["md"]}}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	req.Header.Set("X-Authenticated-User", "attacker")
	w := httptest.NewRecorder()

	h.HandleTexturePublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if gotPlatformReq.OwnerID != "user-1" || gotPlatformReq.RequestedBy != "user-1" {
		t.Fatalf("platform owner/requester mismatch: %#v", gotPlatformReq)
	}
	if gotPlatformReq.SourceDocID != "doc-1" || gotPlatformReq.SourceRevisionID != "rev-2" {
		t.Fatalf("platform source mismatch: %#v", gotPlatformReq)
	}
	if !strings.Contains(string(gotPlatformReq.AccessPolicy), `"visibility":"unlisted"`) || !strings.Contains(string(gotPlatformReq.AccessPolicy), `"route":"public"`) {
		t.Fatalf("platform access policy not forwarded: %s", string(gotPlatformReq.AccessPolicy))
	}
	if !strings.Contains(string(gotPlatformReq.ExportPolicy), `"download_allowed":false`) || !strings.Contains(string(gotPlatformReq.ExportPolicy), `"formats":["md"]`) {
		t.Fatalf("platform export policy not forwarded: %s", string(gotPlatformReq.ExportPolicy))
	}
	if gotPlatformReq.Content != "public projection content [1]" {
		t.Fatalf("platform content: got %q", gotPlatformReq.Content)
	}
	if len(gotPlatformReq.History) != 2 {
		t.Fatalf("platform history len: got %d, want 2", len(gotPlatformReq.History))
	}
	if gotPlatformReq.History[0].RevisionID != "rev-1" || gotPlatformReq.History[1].RevisionID != "rev-2" {
		t.Fatalf("platform history not oldest-first: %s, %s", gotPlatformReq.History[0].RevisionID, gotPlatformReq.History[1].RevisionID)
	}
	if !strings.Contains(string(gotPlatformReq.SourceEntities), "content-public-1") {
		t.Fatalf("platform source_entities not forwarded: %s", string(gotPlatformReq.SourceEntities))
	}
	if !strings.Contains(string(gotPlatformReq.SourceEntities), "reader_snapshot") || !strings.Contains(string(gotPlatformReq.SourceEntities), "Cleaned public source text") {
		t.Fatalf("platform source_entities missing public source reader snapshot: %s", string(gotPlatformReq.SourceEntities))
	}
	snapshot := publicationReaderSnapshot(t, gotPlatformReq.SourceEntities, "src-1")
	if snapshot["media_type"] != "text/markdown" {
		t.Fatalf("reader snapshot media_type = %#v, want text/markdown", snapshot["media_type"])
	}
	if snapshot["original_media_type"] != "text/html; charset=utf-8" {
		t.Fatalf("reader snapshot original_media_type = %#v, want source html type", snapshot["original_media_type"])
	}
	var resp platform.PublishTextureResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PublicURL != "https://choir.news/pub/texture/my-note-pub1" {
		t.Fatalf("public url: got %q", resp.PublicURL)
	}
}

func TestHandleTexturePublicationRejectsSourceEntitiesWithoutBodyDocBeforeEnrichment(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	corpusdCalled := false
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpusdCalled = true
		t.Fatalf("corpusd should not be called for detached source_entities")
	}))
	defer corpusd.Close()

	contentFetches := 0
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-detached":
			_ = json.NewEncoder(w).Encode(sandboxTextureDocument{
				DocID:             "doc-detached",
				OwnerID:           "user-1",
				Title:             "Detached Note",
				CurrentRevisionID: "rev-detached",
			})
		case "/api/texture/revisions/rev-detached":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevision{
				RevisionID: "rev-detached",
				DocID:      "doc-detached",
				OwnerID:    "user-1",
				Content:    "public projection content with detached source identity.",
				SourceEntities: testProxySourceEntities(t, testProxySourceEntity(
					"src-detached",
					"url",
					"https://example.com/detached-source",
					"Detached Source",
					"Detached source excerpt",
					"public_url_snapshot",
				)),
				Metadata: json.RawMessage(`{}`),
			})
		case "/api/content/import-url":
			contentFetches += 1
			t.Fatalf("source import should not be called for detached source_entities")
		default:
			t.Fatalf("sandbox path: got %s", r.URL.Path)
		}
	}))
	defer sandbox.Close()

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-detached","revision_id":"rev-detached"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleTexturePublication(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "source_entities require body_doc") {
		t.Fatalf("body = %s, want body_doc requirement", w.Body.String())
	}
	if corpusdCalled {
		t.Fatalf("corpusd was called for detached source_entities")
	}
	if contentFetches != 0 {
		t.Fatalf("source import was called for detached source_entities: %d", contentFetches)
	}
}

func TestHandleTexturePublicationRejectsMalformedPolicy(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	corpusdCalled := false
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpusdCalled = true
	}))
	defer corpusd.Close()

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-1","access_policy":["public"],"export_policy":{"download_allowed":true}}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleTexturePublication(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "access_policy must be a JSON object") {
		t.Fatalf("error body = %s", w.Body.String())
	}
	if corpusdCalled {
		t.Fatalf("corpusd was called for malformed policy")
	}
}

func TestHandleTexturePublicationPublishesPublicURLSourceSnapshots(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishTextureRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/publications/texture" {
			t.Fatalf("corpusd path: got %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{
			PublicationID:        "pub-url",
			PublicationVersionID: "pubver-url",
			RoutePath:            "/pub/texture/url-note-pub1",
			ContentHash:          "hash",
			SourceRevisionHash:   "source-hash",
			State:                "published",
		})
	}))
	defer corpusd.Close()

	var importCalled bool
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != "user-1" {
			t.Fatalf("sandbox trusted user header: got %q", r.Header.Get("X-Authenticated-User"))
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-url":
			_ = json.NewEncoder(w).Encode(sandboxTextureDocument{
				DocID:             "doc-url",
				OwnerID:           "user-1",
				Title:             "URL Note",
				CurrentRevisionID: "rev-url",
			})
		case "/api/texture/documents/doc-url/revisions":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevisionList{Revisions: []sandboxTextureRevision{
				{RevisionID: "rev-url", DocID: "doc-url", OwnerID: "user-1", VersionNumber: 1, Content: "public projection content [1]", RevisionHash: "hurl", CreatedAt: "2026-01-01T00:00:00.000Z"},
			}})
		case "/api/texture/revisions/rev-url":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevision{
				RevisionID:     "rev-url",
				DocID:          "doc-url",
				OwnerID:        "user-1",
				Content:        "public projection content [1]",
				BodyDoc:        testProxyTextureBodyDoc(t, "doc-url", "src-url", "URL Source"),
				SourceEntities: testProxySourceEntities(t, testProxySourceEntity("src-url", "url", "https://example.com/source", "URL Source", "bounded excerpt", "public_url_snapshot")),
				Metadata:       json.RawMessage(`{}`),
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
			if body["query"] != "URL Source" {
				t.Fatalf("import query: got %q", body["query"])
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
				Metadata:     json.RawMessage(`{"retrieval_strategy":"direct_http_then_readability_lite"}`),
				Provenance:   json.RawMessage(`{"warnings":["extracted text is low-content","used html readable fallback"]}`),
			})
		default:
			t.Fatalf("sandbox path: got %s", r.URL.Path)
		}
	}))
	defer sandbox.Close()

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-url","revision_id":"rev-url","slug":"url-note"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleTexturePublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if !importCalled {
		t.Fatalf("expected public URL source import")
	}
	sourceEntities := string(gotPlatformReq.SourceEntities)
	if !strings.Contains(sourceEntities, "reader_snapshot") || !strings.Contains(sourceEntities, "Cleaned URL source text") {
		t.Fatalf("platform source_entities missing URL reader snapshot: %s", sourceEntities)
	}
	snapshot := publicationReaderSnapshot(t, gotPlatformReq.SourceEntities, "src-url")
	if snapshot["media_type"] != "text/markdown" {
		t.Fatalf("reader snapshot media_type = %#v, want text/markdown", snapshot["media_type"])
	}
	if snapshot["original_media_type"] != "text/html; charset=utf-8" {
		t.Fatalf("reader snapshot original_media_type = %#v, want source html type", snapshot["original_media_type"])
	}
	status := publicationReaderSnapshotStatus(t, gotPlatformReq.SourceEntities, "src-url")
	if status["state"] != sourcecontract.ReaderArtifactStateReady {
		t.Fatalf("reader snapshot state = %#v, want %q", status["state"], sourcecontract.ReaderArtifactStateReady)
	}
	if status["quality"] != "warning" {
		t.Fatalf("reader snapshot quality = %#v, want warning", status["quality"])
	}
	if status["retrieval_strategy"] != "direct_http_then_readability_lite" {
		t.Fatalf("reader snapshot retrieval_strategy = %#v", status["retrieval_strategy"])
	}
	if status["warning_count"] != float64(2) {
		t.Fatalf("reader snapshot warning_count = %#v, want 2", status["warning_count"])
	}
	warnings, ok := status["warnings"].([]any)
	if !ok || len(warnings) != 2 || warnings[0] != "extracted text is low-content" {
		t.Fatalf("reader snapshot warnings = %#v", status["warnings"])
	}
	if !strings.Contains(sourceEntities, "bounded excerpt") {
		t.Fatalf("platform source_entities lost bounded transclusion selector: %s", sourceEntities)
	}
}

func TestHandleTexturePublicationRecordsURLSnapshotImportFailureState(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishTextureRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{
			PublicationID:        "pub-failed-url",
			PublicationVersionID: "pubver-failed-url",
			RoutePath:            "/pub/texture/failed-url",
			SourceRevisionHash:   "source-hash",
			State:                "published",
		})
	}))
	defer corpusd.Close()

	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-url":
			_ = json.NewEncoder(w).Encode(sandboxTextureDocument{
				DocID:             "doc-url",
				OwnerID:           "user-1",
				Title:             "URL Note",
				CurrentRevisionID: "rev-url",
			})
		case "/api/texture/documents/doc-url/revisions":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevisionList{Revisions: []sandboxTextureRevision{
				{RevisionID: "rev-url", DocID: "doc-url", OwnerID: "user-1", VersionNumber: 1, Content: "public projection content [1]", RevisionHash: "hurl", CreatedAt: "2026-01-01T00:00:00.000Z"},
			}})
		case "/api/texture/revisions/rev-url":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevision{
				RevisionID:     "rev-url",
				DocID:          "doc-url",
				OwnerID:        "user-1",
				Content:        "public projection content [1]",
				BodyDoc:        testProxyTextureBodyDoc(t, "doc-url", "src-url", "Blocked Source"),
				SourceEntities: testProxySourceEntities(t, testProxySourceEntity("src-url", "url", "https://example.com/blocked", "Blocked Source", "bounded excerpt", "public_url_snapshot")),
				Metadata:       json.RawMessage(`{}`),
			})
		case "/api/content/import-url":
			http.Error(w, `{"error":"URL import failed: 403 Forbidden"}`, http.StatusBadGateway)
		default:
			t.Fatalf("sandbox path: got %s", r.URL.Path)
		}
	}))
	defer sandbox.Close()

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-url","revision_id":"rev-url","slug":"url-note"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleTexturePublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	sourceEntities := string(gotPlatformReq.SourceEntities)
	status := publicationReaderSnapshotStatus(t, gotPlatformReq.SourceEntities, "src-url")
	if status["state"] != sourcecontract.ReaderArtifactStateImportFailed {
		t.Fatalf("reader snapshot state = %#v, want %q", status["state"], sourcecontract.ReaderArtifactStateImportFailed)
	}
	if !strings.Contains(sourceEntities, "source_import_failed") || !strings.Contains(sourceEntities, "http_403") || !strings.Contains(sourceEntities, "http_status") {
		t.Fatalf("platform source_entities missing import failure diagnostics: %s", sourceEntities)
	}
	if strings.Contains(sourceEntities, "reader_snapshot\":") {
		t.Fatalf("failed import must not synthesize reader snapshot: %s", sourceEntities)
	}
	if !strings.Contains(sourceEntities, "bounded excerpt") {
		t.Fatalf("platform source_entities lost bounded transclusion selector: %s", sourceEntities)
	}
}

func TestHandleTexturePublicationDoesNotPublishPrivateSourceSnapshots(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotPlatformReq platform.PublishTextureRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPlatformReq); err != nil {
			t.Fatalf("decode platform request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(platform.PublishTextureResponse{
			PublicationID:        "pub-private",
			PublicationVersionID: "pubver-private",
			RoutePath:            "/pub/texture/private-note-pub",
			State:                "published",
		})
	}))
	defer corpusd.Close()

	contentFetches := 0
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/texture/documents/doc-private":
			_ = json.NewEncoder(w).Encode(sandboxTextureDocument{
				DocID:             "doc-private",
				OwnerID:           "user-1",
				Title:             "Private Note",
				CurrentRevisionID: "rev-private",
			})
		case "/api/texture/documents/doc-private/revisions":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevisionList{Revisions: []sandboxTextureRevision{
				{RevisionID: "rev-private", DocID: "doc-private", OwnerID: "user-1", VersionNumber: 1, Content: "public projection with private source excerpt [1]", RevisionHash: "hpriv", CreatedAt: "2026-01-01T00:00:00.000Z"},
			}})
		case "/api/texture/revisions/rev-private":
			_ = json.NewEncoder(w).Encode(sandboxTextureRevision{
				RevisionID:     "rev-private",
				DocID:          "doc-private",
				OwnerID:        "user-1",
				Content:        "public projection content [1]",
				BodyDoc:        testProxyTextureBodyDoc(t, "doc-private", "src-private", "Private Source"),
				SourceEntities: testProxySourceEntities(t, testProxySourceEntity("src-private", "content_item", "content-private-1", "Private Source", "bounded excerpt", "private_user_source")),
				Metadata:       json.RawMessage(`{}`),
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

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        sandbox.URL,
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-private","revision_id":"rev-private"}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()

	h.HandleTexturePublication(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status: got %d body %s", w.Code, w.Body.String())
	}
	if contentFetches != 0 {
		t.Fatalf("private source content was fetched for publication: %d", contentFetches)
	}
	if strings.Contains(string(gotPlatformReq.SourceEntities), "reader_snapshot") || strings.Contains(string(gotPlatformReq.SourceEntities), "private full source text") {
		t.Fatalf("private source snapshot leaked into platform source_entities: %s", string(gotPlatformReq.SourceEntities))
	}
}

func publicationReaderSnapshot(t *testing.T, raw json.RawMessage, entityID string) map[string]any {
	t.Helper()
	entity := publicationSourceEntity(t, raw, entityID)
	snapshot, ok := entity["reader_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("entity %s reader_snapshot = %#v, want object", entityID, entity["reader_snapshot"])
	}
	return snapshot
}

func publicationReaderSnapshotStatus(t *testing.T, raw json.RawMessage, entityID string) map[string]any {
	t.Helper()
	entity := publicationSourceEntity(t, raw, entityID)
	status, ok := entity["reader_snapshot_status"].(map[string]any)
	if !ok {
		t.Fatalf("entity %s reader_snapshot_status = %#v, want object", entityID, entity["reader_snapshot_status"])
	}
	return status
}

func publicationSourceEntity(t *testing.T, raw json.RawMessage, entityID string) map[string]any {
	t.Helper()
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("decode publication source_entities: %v", err)
	}
	for _, value := range values {
		entity, ok := value.(map[string]any)
		if !ok || (entity["source_entity_id"] != entityID && entity["entity_id"] != entityID) {
			continue
		}
		return entity
	}
	t.Fatalf("source entity %s not found in source_entities: %s", entityID, string(raw))
	return nil
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

// TestHandleAPIDispatchesTexturePublication guards against a regression where a
// duplicate router case (introduced by the predecessor->Texture blind rename) shadowed
// the canonical publish route, returning 404 for the exact path the frontend
// posts to. Routing through HandleAPI must reach the handler, not a 404 shadow.
func TestHandleAPIDispatchesTexturePublication(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("corpusd should not be reached for a malformed policy")
	}))
	defer corpusd.Close()

	h, err := NewHandler(&Config{AllowDirectSandboxForTests: true, Port: "0",
		SandboxURL:        "http://127.0.0.1:1",
		AuthPublicKeyPath: "/unused/in/test",
		CorpusdURL:        corpusd.URL}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "https://choir.news/api/platform/texture/publications", strings.NewReader(`{"doc_id":"doc-1","access_policy":["public"]}`))
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("router must dispatch /api/platform/texture/publications to the publish handler: got status %d body %s (want 400 = handler reached; 404 = shadowed)", w.Code, w.Body.String())
	}
}
