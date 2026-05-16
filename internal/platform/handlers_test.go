package platform

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalPublishRequiresInternalCallerAndPublicRouteRenders(t *testing.T) {
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
	handler.HandleInternalPublishVText(denied, httptest.NewRequest(http.MethodPost, "/internal/platform/publications/vtext", bytes.NewReader(body)))
	if denied.Code != http.StatusForbidden {
		t.Fatalf("missing internal caller: got %d, want %d", denied.Code, http.StatusForbidden)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/platform/publications/vtext", bytes.NewReader(body))
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

	pageReq := httptest.NewRequest(http.MethodGet, resp.RoutePath, nil)
	pageW := httptest.NewRecorder()
	handler.HandlePublicVText(pageW, pageReq)
	if pageW.Code != http.StatusOK {
		t.Fatalf("public route status: got %d body %s", pageW.Code, pageW.Body.String())
	}
	html := pageW.Body.String()
	if !strings.Contains(html, "Visible public content") {
		t.Fatalf("public route missing content: %s", html)
	}
	if !strings.Contains(html, resp.ContentHash) || !strings.Contains(html, resp.SourceRevisionHash) {
		t.Fatalf("public route missing hashes")
	}
}
