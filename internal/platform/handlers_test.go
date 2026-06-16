package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/server"
)

func TestInternalPublishRequiresInternalCallerAndBundleResolve(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts")))

	body, _ := json.Marshal(PublishVTextRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Published Draft",
		Content:          "Visible public content",
		RequestedBy:      "user-1",
	})
	denied := httptest.NewRecorder()
	handler.HandleInternalPublishVText(denied, httptest.NewRequest(http.MethodPost, "/internal/platform/publications/texture", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing internal caller: got %d, want %d", denied.Code, http.StatusForbidden)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/platform/publications/texture", bytes.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalPublishVText(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish status: got %d body %s", w.Code, w.Body.String())
	}
	var resp PublishVTextResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode publish response: %v", err)
	}

	resolveReq := httptest.NewRequest(http.MethodGet, "/internal/platform/publications/resolve?route="+resp.RoutePath, nil)
	resolveReq.Header.Set("X-Internal-Caller", "true")
	resolveW := httptest.NewRecorder()
	handler.HandleInternalResolvePublication(resolveW, resolveReq)
	if resolveW.Code != http.StatusOK {
		t.Fatalf("resolve status: got %d body %s", resolveW.Code, resolveW.Body.String())
	}
	var bundle PublicationBundle
	if err := json.NewDecoder(resolveW.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.Artifact.Content != "Visible public content" {
		t.Fatalf("bundle content mismatch: %q", bundle.Artifact.Content)
	}
	if bundle.Version.ContentHash != resp.ContentHash || bundle.Version.SourceRevisionHash != resp.SourceRevisionHash {
		t.Fatalf("bundle hashes did not round trip")
	}
	if len(bundle.Retrieval.Spans) != 1 {
		t.Fatalf("bundle retrieval spans: got %d, want 1", len(bundle.Retrieval.Spans))
	}
	if len(bundle.Citations) == 0 {
		t.Fatalf("bundle citations missing")
	}

	publicW := httptest.NewRecorder()
	handler.HandlePublicVText(publicW, httptest.NewRequest(http.MethodGet, resp.RoutePath, nil))
	if publicW.Code != http.StatusNotFound {
		t.Fatalf("platformd public HTML route should be disabled, got %d", publicW.Code)
	}
}

func TestRegisteredTextureRoutesExcludeLegacyVTextPlatformPrefix(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts")))
	s := server.NewServer("platformd-test", "0")
	RegisterRoutes(s, handler)

	syncBody, _ := json.Marshal(SyncVTextDocumentRequest{
		DocID:   "doc-1",
		OwnerID: "user-1",
		Title:   "Platform Note",
		Revisions: []SyncVTextRevision{{
			RevisionID: "rev-1",
			AuthorKind: "agent",
			Content:    "Platform content",
		}},
	})
	syncReq := httptest.NewRequest(http.MethodPost, "/internal/platform/texture/sync", bytes.NewReader(syncBody))
	syncReq.Header.Set("X-Internal-Caller", "true")
	syncW := httptest.NewRecorder()
	s.ServeHTTP(syncW, syncReq)
	if syncW.Code != http.StatusOK {
		t.Fatalf("registered texture sync status: got %d body %s", syncW.Code, syncW.Body.String())
	}

	readReq := httptest.NewRequest(http.MethodGet, "/internal/platform/texture/documents/doc-1", nil)
	readReq.Header.Set("X-Internal-Caller", "true")
	readW := httptest.NewRecorder()
	s.ServeHTTP(readW, readReq)
	if readW.Code != http.StatusOK {
		t.Fatalf("registered texture document read status: got %d body %s", readW.Code, readW.Body.String())
	}

	legacyCases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/internal/platform/publications/vtext", `{"owner_id":"user-1","source_doc_id":"doc-1","source_revision_id":"rev-1","title":"Platform Note","content":"Platform content"}`},
		{http.MethodPost, "/internal/platform/vtext/sync", string(syncBody)},
		{http.MethodGet, "/internal/platform/vtext/documents/doc-1", ""},
		{http.MethodGet, "/internal/platform/vtext/revisions/rev-1", ""},
	}
	for _, tc := range legacyCases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("X-Internal-Caller", "true")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s status: got %d body %s, want 404", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
