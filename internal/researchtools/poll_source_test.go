package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capturesegment"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// W1b (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// research polls one registry source into web_capture objects. Failure modes
// pinned: a re-poll writes duplicate captures; conditional GET state is lost
// between polls; a capture names a body that cannot be read; the result
// carries bodies instead of counts; an unknown source id is accepted; a
// huge feed is written without a bound.
func TestPollSourceWritesCapturesOnceAndKeepsConditionalState(t *testing.T) {
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	defer sourcefetch.SetAllowPrivateNetworkForTests(prev)
	var conditional atomic.Int32
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			conditional.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Type", "application/rss+xml")
		var items strings.Builder
		for i := 1; i <= 30; i++ {
			fmt.Fprintf(&items, `<item><title>Story %d</title><link>https://news.example/%d</link><description>Body of story %d</description><guid>g-%d</guid><pubDate>Sat, 10 Oct 2026 12:00:00 GMT</pubDate></item>`, i, i, i, i)
		}
		fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Example</title>%s</channel></rss>`, items.String())
	}))
	defer feed.Close()

	st := openTestStore(t)
	root := t.TempDir()
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: "rss:example", Type: sources.SourceTypeRSS, Name: "Example", URL: feed.URL, MaxItemsPerPoll: 20}}}
	deps := Dependencies{Store: st, Sources: reg, FilesRoot: root}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "research-run", OwnerID: "owner-1", ComputerID: "computer-1", Profile: agentprofile.Research,
	})
	poll := func() map[string]any {
		t.Helper()
		raw, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"rss:example"}`))
		if err != nil {
			t.Fatalf("poll_source: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		if strings.Contains(string(raw), "Body of story") {
			t.Fatalf("result carries bodies: %s", raw)
		}
		return out
	}
	first := poll()
	if first["new"] != float64(20) || first["fetched"] != float64(20) {
		t.Fatalf("first poll = %v, want 20 new of 20 fetched (bounded from 30)", first)
	}
	captured, _ := first["captured"].([]any)
	if len(captured) != 20 {
		t.Fatalf("first poll listed %d captures: %v", len(captured), first)
	}
	entry := captured[0].(map[string]any)
	if entry["title"] == "" || entry["url"] == "" {
		t.Fatalf("captured entry lacks title or url: %v", entry)
	}
	obj, err := st.ObjectGraph().GetObject(context.Background(), entry["capture_id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	if body, err := capturesegment.Read(root, meta.ContentBlobID); err != nil || !strings.Contains(string(body), "Body of story") {
		t.Fatalf("capture body unreadable: %q %v", body, err)
	}
	second := poll()
	if conditional.Load() != 1 || second["new"] != float64(0) || second["not_modified"] != true {
		t.Fatalf("second poll = %v (conditional requests %d), want a 304 with nothing written", second, conditional.Load())
	}
	if _, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"rss:missing"}`)); err == nil || !strings.Contains(err.Error(), "rss:example") {
		t.Fatalf("unknown source error = %v, want it to name known ids", err)
	}
}
