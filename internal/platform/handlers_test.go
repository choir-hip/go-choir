package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestInternalPublishRequiresInternalCallerAndBundleResolve(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), ""))

	body, _ := json.Marshal(PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Published Draft",
		Content:          "Visible public content",
		RequestedBy:      "user-1",
	})
	denied := httptest.NewRecorder()
	handler.HandleInternalPublishTexture(denied, httptest.NewRequest(http.MethodPost, "/internal/platform/publications/texture", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing internal caller: got %d, want %d", denied.Code, http.StatusForbidden)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/platform/publications/texture", bytes.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalPublishTexture(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("publish status: got %d body %s", w.Code, w.Body.String())
	}
	var resp PublishTextureResponse
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
	handler.HandlePublicTexture(publicW, httptest.NewRequest(http.MethodGet, resp.RoutePath, nil))
	if publicW.Code != http.StatusNotFound {
		t.Fatalf("corpusd public HTML route should be disabled, got %d", publicW.Code)
	}
}

func TestInternalListTextureRevisionsUsesTextureEnvelope(t *testing.T) {
	store, root := openTestPlatformStore(t)
	handler := NewHandler(NewService(store, filepath.Join(root, "artifacts"), ""))

	syncReq := SyncTextureDocumentRequest{
		DocID:   "doc-wire-1",
		OwnerID: "universal-wire-platform",
		Title:   "Wire article",
		Revisions: []SyncTextureRevision{{
			RevisionID:  "rev-wire-head",
			AuthorKind:  "appagent",
			AuthorLabel: "Universal Wire",
			Content:     "Wire article body",
		}},
	}
	body, err := json.Marshal(syncReq)
	if err != nil {
		t.Fatalf("marshal sync request: %v", err)
	}
	sync := httptest.NewRequest(http.MethodPost, "/internal/platform/texture/sync", bytes.NewReader(body))
	sync.Header.Set("X-Internal-Caller", "true")
	syncW := httptest.NewRecorder()
	handler.HandleInternalSyncTextureDocument(syncW, sync)
	if syncW.Code != http.StatusOK {
		t.Fatalf("sync status = %d body=%s", syncW.Code, syncW.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/platform/texture/documents/doc-wire-1/revisions", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalListTextureRevisions(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", w.Code, w.Body.String())
	}
	var got PlatformTextureRevisionListResponse
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode revision envelope: %v", err)
	}
	if len(got.Revisions) != 1 || got.Revisions[0].RevisionID != "rev-wire-head" || got.Revisions[0].Content != "Wire article body" {
		t.Fatalf("revision envelope = %+v, want synced revision", got)
	}
}
