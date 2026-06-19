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
		t.Fatalf("platformd public HTML route should be disabled, got %d", publicW.Code)
	}
}
