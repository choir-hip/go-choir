package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type paperGuest struct{}

func (paperGuest) LookupGuestContext(context.Context, string) (*vmctl.GuestOwnership, error) {
	return &vmctl.GuestOwnership{Found: true, UserID: "owner-1", ComputerID: "computer-1"}, nil
}

// W1b2 (docs/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
// research polls one registry source into shared web_capture objects in the
// store behind corpusd, never this computer's own graph. Failure modes
// pinned: a capture lands in the guest instead of the shared store; a re-poll
// writes duplicate captures; conditional GET state is lost
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

	// corpusd's real handler over a shared graph; requests arrive as the
	// paper computer's guest.
	mem := objectgraph.NewMemoryStore()
	shared := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer shared.Close()
	og := platform.NewObjectGraphHandler(shared, mem)
	if err := og.ConfigureGuestBinding(paperGuest{}); err != nil {
		t.Fatal(err)
	}
	og.ConfigureCaptureBodies(t.TempDir())
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "10.200.1.2:40000"
		switch {
		case r.URL.Path == "/internal/platform/captures":
			og.HandleCaptures(w, r)
		default:
			og.HandleCaptureBody(w, r)
		}
	}))
	defer corpusd.Close()
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: "rss:example", Type: sources.SourceTypeRSS, Name: "Example", URL: feed.URL, MaxItemsPerPoll: 20}}}
	deps := Dependencies{Sources: reg, Captures: NewCorpusdCaptureSink(corpusd.URL)}
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
	obj, err := shared.GetObject(context.Background(), entry["capture_id"].(string))
	if err != nil {
		t.Fatalf("capture not in the shared store: %v", err)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(corpusd.URL + "/internal/platform/captures/bodies/" + meta.ContentBlobID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "Body of story") {
		t.Fatalf("capture body unreadable: %d %q", resp.StatusCode, body)
	}
	second := poll()
	if conditional.Load() != 1 || second["new"] != float64(0) || second["not_modified"] != true {
		t.Fatalf("second poll = %v (conditional requests %d), want a 304 with nothing written", second, conditional.Load())
	}
	if _, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"rss:missing"}`)); err == nil || !strings.Contains(err.Error(), "rss:example") {
		t.Fatalf("unknown source error = %v, want it to name known ids", err)
	}
}

type flakySink struct {
	fail  bool
	items int
}

func (s *flakySink) WriteCaptures(_ context.Context, _, _ string, items []sources.Item) (sourcegraph.CaptureWriteResult, error) {
	if s.fail {
		s.fail = false
		return sourcegraph.CaptureWriteResult{}, fmt.Errorf("corpusd unavailable")
	}
	s.items += len(items)
	return sourcegraph.CaptureWriteResult{New: len(items)}, nil
}

func (s *flakySink) ReadCapture(context.Context, string, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

// A failed capture write keeps the feed's previous validators, so the next
// poll fetches the items again instead of getting a 304 and losing them
// (problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md).
func TestPollSourceKeepsValidatorsOnlyAfterTheWrite(t *testing.T) {
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
		fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>F</title><item><title>One</title><link>https://news.example/1</link><description>Body</description><guid>g-1</guid></item></channel></rss>`)
	}))
	defer feed.Close()
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: "rss:flaky", Type: sources.SourceTypeRSS, Name: "Flaky", URL: feed.URL}}}
	sink := &flakySink{fail: true}
	deps := Dependencies{Sources: reg, Captures: sink}

	first, err := PollSourceOnce(context.Background(), deps, "owner-1", "computer-1", "rss:flaky", 5)
	if err != nil || first.Error == "" || first.Fetched != 1 {
		t.Fatalf("first poll = %+v, %v; want a fetched item and a write error", first, err)
	}
	second, err := PollSourceOnce(context.Background(), deps, "owner-1", "computer-1", "rss:flaky", 5)
	if err != nil || second.NotModified || second.New != 1 || sink.items != 1 || conditional.Load() != 0 {
		t.Fatalf("second poll = %+v (conditional %d, written %d); want the item fetched again and written", second, conditional.Load(), sink.items)
	}
	third, _ := PollSourceOnce(context.Background(), deps, "owner-1", "computer-1", "rss:flaky", 5)
	if !third.NotModified || conditional.Load() != 1 {
		t.Fatalf("third poll = %+v; want a 304 once the items are written", third)
	}
}
