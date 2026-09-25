package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubBaseAPI is a minimal in-process Base API for client tests. It records
// requests and returns canned responses.
type stubBaseAPI struct {
	t        *testing.T
	delta    DeltaResponse
	blobResp PutBlobResponse
	itemResp PutItemResponse
	gotAuth  string
}

func (s *stubBaseAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.gotAuth = r.Header.Get("Authorization")
	switch r.URL.Path {
	case "/api/base/delta":
		writeJSONTest(w, http.StatusOK, s.delta)
	case "/api/base/blobs":
		writeJSONTest(w, http.StatusOK, s.blobResp)
	case "/api/base/items":
		writeJSONTest(w, http.StatusOK, s.itemResp)
	default:
		writeJSONTest(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func writeJSONTest(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func TestBaseClientErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONTest(w, http.StatusUnauthorized, map[string]string{"error": "bad key"})
	}))
	defer srv.Close()

	c := NewBaseClient(srv.URL, "choir_sk_bad")
	c.SetHTTPClient(srv.Client())

	if _, err := c.FetchDelta(0); err == nil {
		t.Fatal("FetchDelta with 401 should error")
	}
}
