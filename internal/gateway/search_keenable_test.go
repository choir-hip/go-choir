package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type captureTransport struct {
	req  *http.Request
	body []byte
	resp string
}

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.req = r
	c.body, _ = io.ReadAll(r.Body)
	return &http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{}, Body: io.NopCloser(strings.NewReader(c.resp))}, nil
}

// Keenable adds search capacity beside the providers that rate-limit
// (docs/archive/problems/search-rate-limit-benches-provider-for-a-day-2026-10-11.md).
// It posts {query, max_results} with X-API-Key and maps description or
// snippet and published_at into the plane's result.
func TestKeenableProviderSearches(t *testing.T) {
	t.Setenv("KEENABLE_API_KEY", "keen_test")
	tr := &captureTransport{resp: `{"query":"q","mode":"realtime","results":[
		{"title":"A","url":"https://a.example/x","description":"","snippet":"long excerpt","published_at":"2026-10-08T20:40:56Z"},
		{"title":"B","url":"https://b.example/y","description":"short summary"},
		{"title":"no url","url":""}]}`}
	p := &KeenableProvider{httpClient: &http.Client{Transport: tr}}
	if !p.IsAvailable() || p.Name() != "keenable" {
		t.Fatalf("keenable must be available when KEENABLE_API_KEY is set")
	}
	results, err := p.Search(t.Context(), "AI news", 7)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if tr.req.Method != http.MethodPost || tr.req.URL.String() != "https://api.keenable.ai/v1/search" || tr.req.Header.Get("X-API-Key") != "keen_test" {
		t.Fatalf("request = %s %s key=%q", tr.req.Method, tr.req.URL, tr.req.Header.Get("X-API-Key"))
	}
	var body map[string]any
	_ = json.Unmarshal(tr.body, &body)
	if body["query"] != "AI news" || body["max_results"] != float64(7) {
		t.Fatalf("body = %s", tr.body)
	}
	if len(results) != 2 || results[0].Snippet != "long excerpt" || results[0].PublishedAt != "2026-10-08T20:40:56Z" || results[1].Snippet != "short summary" {
		t.Fatalf("results = %+v", results)
	}
	if searchProviderEndpoint("keenable") != "https://api.keenable.ai/v1/search" {
		t.Fatalf("the plane must name keenable's endpoint")
	}
}
