package researchtools

// The research desk reads the paper's own earlier story when the archive
// search shows it first. map[string]any marshals keys alphabetically, which put
// `published` after `captures` ("citable as it is") and the one run that saw
// prior captures never read the paper's story. Ways this could fail:
//
//  P1. The key order regresses to alphabetical (captures before published).
//  P2. published_first_note shows with an empty or failed published search.
//  P3. published_first_note is missing, or does not say to read the pieces at
//      their read_url before the web and cite the capture Read returns.
//  P4. A consumer that decodes into a map loses a field, or one of the
//      existing keys (published_note, captures_note, events_note, counts)
//      changes name or meaning.
//  P5. A head-truncation of long output (captures can be many) no longer shows
//      the published entries and the note.

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

func searchWireCorpusRaw(t *testing.T, baseURL string) string {
	t.Helper()
	guest, err := store.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = guest.Close() })
	tool := newSearchWireCorpusTool(Dependencies{Store: guest, Captures: NewCorpusdCaptureSink(baseURL)})
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{OwnerID: "owner-1", ComputerID: "computer-1"})
	out, err := tool.Func(ctx, json.RawMessage(`{"query":"harbor strike"}`))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func topLevelKeys(t *testing.T, raw string) []string {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("not an object: %v %v", tok, err)
	}
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatal(err)
		}
	}
	return keys
}

func indexOf(keys []string, k string) int {
	for i, v := range keys {
		if v == k {
			return i
		}
	}
	return -1
}

func publishedServer(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
}

func TestSearchWireCorpusPublishedComesFirstWithNote(t *testing.T) {
	srv := publishedServer(`{"query":"harbor strike","pieces":[{"publication_id":"p1","route_path":"/pub/texture/harbor-1","title":"Harbor strike ends","snippet":"Dockworkers voted.","kind":"story","read_url":"https://choir.news/r","score":4}]}`)
	defer srv.Close()
	raw := searchWireCorpusRaw(t, srv.URL)
	keys := topLevelKeys(t, raw)
	if keys[0] != "published_first_note" || keys[1] != "published" {
		t.Fatalf("keys = %v, want published_first_note then published first", keys)
	}
	for _, k := range []string{"captures", "events", "capture_count", "event_count"} {
		if i := indexOf(keys, k); i < 0 || i < indexOf(keys, "published") {
			t.Fatalf("%s at %d must exist and follow published: %v", k, i, keys)
		}
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"published", "published_count", "query", "events", "event_count", "captures", "capture_count"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("lost %q: %v", k, body)
		}
	}
	note, _ := body["published_first_note"].(string)
	for _, want := range []string{"own earlier stories", "read_url", "before the web", "capture"} {
		if !strings.Contains(note, want) {
			t.Fatalf("note %q lacks %q", note, want)
		}
	}
	head := raw
	if len(head) > 900 {
		head = head[:900]
	}
	if !strings.Contains(head, "Harbor strike ends") || !strings.Contains(head, "published_first_note") {
		t.Fatalf("head of output lacks published: %s", head)
	}
}

func TestSearchWireCorpusNoNoteWhenPublishedEmptyOrFailed(t *testing.T) {
	empty := publishedServer(`{"query":"harbor strike","pieces":[]}`)
	defer empty.Close()
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer failing.Close()
	for name, url := range map[string]string{"empty": empty.URL, "failed": failing.URL} {
		raw := searchWireCorpusRaw(t, url)
		if strings.Contains(raw, "published_first_note") {
			t.Fatalf("%s: note shown with no published pieces: %s", name, raw)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(raw), &body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["published"].([]any); !ok {
			t.Fatalf("%s: published must stay a list: %v", name, body["published"])
		}
		if name == "failed" && body["published_note"] == nil {
			t.Fatalf("failed search lost published_note: %v", body)
		}
	}
}
