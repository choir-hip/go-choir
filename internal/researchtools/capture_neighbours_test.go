package researchtools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The guest's neighbours client posts one request to corpusd's route with
// every capture named, and reads the answer, including its missing list and
// an unavailable vector index.
func TestCorpusdSinkAsksNeighboursInOneRequest(t *testing.T) {
	var got CaptureNeighboursQuery
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/internal/platform/captures/neighbours" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"neighbours":{"c1":[{"capture_id":"c9","score":0.8}]},"unembedded":1,"excluded":2,"cut":3,"state":"exact"}`))
	}))
	defer srv.Close()
	sink := NewCorpusdCaptureSink(srv.URL)
	var _ CaptureNeighbourer = sink
	ans, err := sink.CaptureNeighbours(context.Background(), CaptureNeighboursQuery{OwnerID: "o", ComputerID: "c", CaptureIDs: []string{"c1", "c2"}, Since: "2026-11-05", Limit: 12})
	if err != nil || calls != 1 {
		t.Fatalf("%v, %d calls", err, calls)
	}
	if got.OwnerID != "o" || got.ComputerID != "c" || len(got.CaptureIDs) != 2 || got.Since != "2026-11-05" || got.Limit != 12 {
		t.Fatalf("request = %+v", got)
	}
	if len(ans.Neighbours["c1"]) != 1 || ans.Neighbours["c1"][0].CaptureID != "c9" || ans.Unembedded != 1 || ans.Excluded != 2 || ans.Cut != 3 || ans.State != "exact" {
		t.Fatalf("answer = %+v", ans)
	}
	if _, err := NewCorpusdCaptureSink("").CaptureNeighbours(context.Background(), CaptureNeighboursQuery{}); err == nil {
		t.Fatal("no URL accepted")
	}
}
