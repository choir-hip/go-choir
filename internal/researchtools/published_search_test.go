package researchtools

// Research finds published Autopaper stories and opinion pieces
// (docs/problems/oped-not-connected-2026-10-20.md). Failure modes guarded:
//
//  F7. A hit carries no URL choir.Read can capture, or the verb summary does
//      not tell research to Read it and cite the capture, so a piece is
//      cited by its snippet.
//  F8. A failing published search hides the guest's own stories and captures,
//      or reads as "nothing published".

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func runSearchWireCorpus(t *testing.T, baseURL string) map[string]any {
	t.Helper()
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	sink := NewCorpusdCaptureSink(baseURL)
	tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: sink})
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: "owner-1", ComputerID: "computer-1"})
	out, err := tool.Func(ctx, json.RawMessage(`{"query":"harbor strike"}`))
	if err != nil {
		t.Fatalf("search_wire_corpus: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	return body
}

func TestSearchWireCorpusIncludesPublished(t *testing.T) {
	var gotQuery string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/published/search" {
			http.Error(w, "refused", http.StatusNotFound)
			return
		}
		gotQuery = r.URL.Query().Get("q")
		_, _ = w.Write([]byte(`{"query":"harbor strike","pieces":[
			{"publication_id":"p1","route_path":"/pub/texture/harbor-1","title":"Harbor strike ends","snippet":"Dockworkers voted.","kind":"story","read_url":"https://choir.news/api/platform/publications/export?route=%2Fpub%2Ftexture%2Fharbor-1&format=txt&raw=1","score":4},
			{"publication_id":"p2","route_path":"/pub/texture/my-take-2","title":"My take on the strike","snippet":"I think.","kind":"opinion","author":"writer-ab12cd","read_url":"https://choir.news/x","score":2}],"capped":true}`))
	}))
	defer corpusd.Close()
	body := runSearchWireCorpus(t, corpusd.URL)
	if gotQuery != "harbor strike" {
		t.Fatalf("published search got q=%q", gotQuery)
	}
	published, _ := body["published"].([]any)
	if len(published) != 2 || body["published_count"] != float64(2) {
		t.Fatalf("published = %v count %v", body["published"], body["published_count"])
	}
	first := published[0].(map[string]any)
	if first["kind"] != "story" || first["route_path"] != "/pub/texture/harbor-1" || first["title"] != "Harbor strike ends" ||
		!strings.Contains(first["read_url"].(string), "raw=1") {
		t.Fatalf("first published hit = %v", first)
	}
	if second := published[1].(map[string]any); second["kind"] != "opinion" || second["author"] != "writer-ab12cd" {
		t.Fatalf("opinion hit = %v", second)
	}
	if note, _ := body["published_note"].(string); !strings.Contains(note, "capped") {
		t.Fatalf("a capped search must say so: note %q", note)
	}
	// The other halves are still there.
	for _, key := range []string{"results", "captures", "events"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("the corpus answer lost %q: %v", key, body)
		}
	}
}

func TestSearchWireCorpusPublishedFailureIsNotEmpty(t *testing.T) {
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer corpusd.Close()
	body := runSearchWireCorpus(t, corpusd.URL)
	if published, _ := body["published"].([]any); len(published) != 0 {
		t.Fatalf("published = %v on a failing search", published)
	}
	if note, _ := body["published_note"].(string); !strings.Contains(note, "published pieces not searched") {
		t.Fatalf("a failing published search must say it did not search: note %q", note)
	}
	if _, ok := body["results"]; !ok {
		t.Fatalf("a failing published search hid the guest's own stories: %v", body)
	}
}
