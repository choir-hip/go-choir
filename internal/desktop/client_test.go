package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
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

func TestBaseClientFetchDeltaAuth(t *testing.T) {
	srv := httptest.NewServer(&stubBaseAPI{
		t:     t,
		delta: DeltaResponse{Events: []model.Event{}, Cursor: 5, Head: 5},
	})
	defer srv.Close()

	c := NewBaseClient(srv.URL, "choir_sk_test")
	c.SetHTTPClient(srv.Client())

	out, err := c.FetchDelta(3)
	if err != nil {
		t.Fatalf("FetchDelta: %v", err)
	}
	if out.Cursor != 5 || out.Head != 5 {
		t.Errorf("FetchDelta: got cursor=%d head=%d, want 5/5", out.Cursor, out.Head)
	}
	if got := c.apiKey; got != "choir_sk_test" {
		t.Errorf("apiKey: got %q", got)
	}
}

func TestBaseClientPutBlob(t *testing.T) {
	api := &stubBaseAPI{
		t:        t,
		blobResp: PutBlobResponse{BlobRef: "sha256:abc", SizeBytes: 4, SHA256: "abc"},
	}
	srv := httptest.NewServer(api)
	defer srv.Close()

	c := NewBaseClient(srv.URL, "choir_sk_test")
	c.SetHTTPClient(srv.Client())

	out, err := c.PutBlob([]byte("data"), "text/plain")
	if err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if out.BlobRef != "sha256:abc" {
		t.Errorf("BlobRef: got %q", out.BlobRef)
	}
	if api.gotAuth != "Bearer choir_sk_test" {
		t.Errorf("auth header: got %q, want Bearer choir_sk_test", api.gotAuth)
	}
}

func TestBaseClientPutItem(t *testing.T) {
	api := &stubBaseAPI{
		t:        t,
		itemResp: PutItemResponse{EventID: "base_evt_1", CursorSeq: 7, ItemID: "base_item_x"},
	}
	srv := httptest.NewServer(api)
	defer srv.Close()

	c := NewBaseClient(srv.URL, "choir_sk_test")
	c.SetHTTPClient(srv.Client())

	resp, err := c.PutItem(PutItemRequest{
		ItemID:    "base_item_x",
		EventType: model.EventCreate,
		Kind:      model.KindFile,
		Name:      "hello.txt",
	})
	if err != nil {
		t.Fatalf("PutItem: %v", err)
	}
	if resp.CursorSeq != 7 {
		t.Errorf("CursorSeq: got %d, want 7", resp.CursorSeq)
	}
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
