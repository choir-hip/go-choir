package textureowner

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The PUT/DELETE document routes, diff, blame and the tell/correct/
// agent-revision tombstones are deleted (D0 census A9/A10). An unlisted
// document sub-route must not fall through to the document handler, and the
// document resource itself is read-only: only the Texture desk writes a
// Texture.
func TestTextureRouterRefusesDeletedRoutes(t *testing.T) {
	_, handler := testAPISetup(t)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/api/texture/documents/doc-1/tell", http.StatusNotFound},
		{http.MethodPost, "/api/texture/documents/doc-1/correct", http.StatusNotFound},
		{http.MethodPost, "/api/texture/documents/doc-1/agent-revision", http.StatusNotFound},
		{http.MethodGet, "/api/texture/documents/doc-1/unknown", http.StatusNotFound},
		{http.MethodGet, "/api/texture/diff?from=a&to=b", http.StatusNotFound},
		{http.MethodPut, "/api/texture/documents/doc-1", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/api/texture/documents/doc-1", http.StatusMethodNotAllowed},
	} {
		request := textureRequest(t, tc.method, tc.path, nil)
		response := httptest.NewRecorder()
		handler.HandleTextureRouter(response, request)
		if response.Code != tc.want {
			t.Errorf("%s %s = %d, want %d (body=%s)", tc.method, tc.path, response.Code, tc.want, response.Body.String())
		}
	}
}
