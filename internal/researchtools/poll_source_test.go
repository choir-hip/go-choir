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

// W1b2 (docs/archive/definitions/choir-autopaper-w1-content-in-the-graph-2026-10-11.md):
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
	if first["new"] != float64(20) || first["fetched"] != float64(30) || first["deferred"] != float64(10) {
		t.Fatalf("first poll = %v, want 20 new of 30 fetched with 10 deferred (L2a A5, defect 1)", first)
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
	// The first poll deferred items, so it kept no validators: the second
	// reads the feed again and writes the 10 it deferred.
	second := poll()
	if conditional.Load() != 0 || second["new"] != float64(10) || second["deferred"] != float64(0) {
		t.Fatalf("second poll = %v (conditional requests %d), want the 10 deferred items written", second, conditional.Load())
	}
	third := poll()
	if conditional.Load() != 1 || third["new"] != float64(0) || third["not_modified"] != true {
		t.Fatalf("third poll = %v (conditional requests %d), want a 304 with nothing written", third, conditional.Load())
	}
	if _, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"rss:missing"}`)); err == nil || !strings.Contains(err.Error(), "rss:example") {
		t.Fatalf("unknown source error = %v, want it to name known ids", err)
	}
}

type flakySink struct {
	fail  bool
	items int
}

func (s *flakySink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	if s.fail {
		s.fail = false
		return sourcegraph.CaptureWriteResult{}, fmt.Errorf("corpusd unavailable")
	}
	s.items += len(items)
	return sourcegraph.CaptureWriteResult{New: len(items)}, nil
}

func (s *flakySink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *flakySink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
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

// dedupSink captures a URL once and repeats it after, recording what each
// write was sent.
type dedupSink struct {
	seen map[string]string
	sent []sourcegraph.CaptureItem
}

func (s *dedupSink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	var out sourcegraph.CaptureWriteResult
	for _, item := range items {
		s.sent = append(s.sent, item)
		captured := sourcegraph.CapturedItem{Title: item.Title, URL: item.URL, Summary: item.Summary, Outlet: item.Outlet}
		if id, ok := s.seen[item.URL]; ok {
			captured.CaptureID = id
			out.Repeated++
			out.Repeats = append(out.Repeats, captured)
			continue
		}
		captured.CaptureID = fmt.Sprintf("choir.web_capture:c-%d", len(s.seen)+1)
		s.seen[item.URL] = captured.CaptureID
		out.New++
		out.Captured = append(out.Captured, captured)
	}
	return out, nil
}

func (s *dedupSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}

func (s *dedupSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

// L2a A5, defect 1 (docs/problems/autopaper-intake-defects-2026-10-14.md;
// design correction from panel round 2). Ways lossless per-poll truncation
// can fail, written before the code:
//   - the poller keeps the first N feed items before the repeat check, so
//     the next poll reads the same N, all repeat, and items past N are never
//     reached;
//   - items past the limit are dropped without a count;
//   - validators are kept on a poll that deferred items, so the next poll is
//     a 304 and the leftovers are never fetched;
//   - validators are never kept again for a feed longer than the limit, so
//     a quiet feed is re-read in full forever;
//   - the leftovers are captured out of feed order;
//   - an item already handled spends the limit (or a reader fetch) again;
//   - after a restart forgets what was handled, one poll re-reads every
//     article in the feed;
//   - the feed's summary and outlet do not reach the capture writer.
func TestPollSourceDefersNewItemsBeyondTheLimitAndCatchesUp(t *testing.T) {
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
			fmt.Fprintf(&items, `<item><title>Story %d</title><link>https://deferred.example/%d</link><description>Summary of story %d</description><guid>d-%d</guid></item>`, i, i, i, i)
		}
		fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Deferred</title>%s</channel></rss>`, items.String())
	}))
	defer feed.Close()
	const sourceID = "rss:deferred"
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: sourceID, Type: sources.SourceTypeRSS, Name: "Deferred Wire", URL: feed.URL}}}
	sink := &dedupSink{seen: map[string]string{}}
	deps := Dependencies{Sources: reg, Captures: sink}
	poll := func() PollReport {
		t.Helper()
		report, err := PollSourceOnce(context.Background(), deps, "owner-1", "computer-1", sourceID, 5)
		if err != nil || report.Error != "" {
			t.Fatalf("poll = %+v, %v", report, err)
		}
		return report
	}

	next := 1
	for round, wantDeferred := range []int{25, 20, 15, 10, 5, 0} {
		report := poll()
		if report.New != 5 || report.Deferred != wantDeferred || report.Fetched != 30 || report.Repeated != 0 || report.NotModified {
			t.Fatalf("poll %d = %+v; want 5 new, %d deferred, no repeats", round+1, report, wantDeferred)
		}
		for _, c := range report.Captured {
			if want := fmt.Sprintf("Story %d", next); c.Title != want {
				t.Fatalf("poll %d captured %q, want %q (feed order)", round+1, c.Title, want)
			}
			if c.Summary != fmt.Sprintf("Summary of story %d", next) || c.Outlet != "Deferred Wire" {
				t.Fatalf("captured item %+v lacks the feed summary or the outlet", c)
			}
			next++
		}
		if conditional.Load() != 0 {
			t.Fatalf("poll %d sent a conditional request while items were deferred", round+1)
		}
	}
	if len(sink.sent) != 30 {
		t.Fatalf("the writer was sent %d items over six polls, want each of the 30 once", len(sink.sent))
	}
	// Nothing was deferred on the sixth poll, so it kept the validators.
	if quiet := poll(); !quiet.NotModified || quiet.New != 0 || conditional.Load() != 1 {
		t.Fatalf("seventh poll = %+v (conditional %d); want a 304", quiet, conditional.Load())
	}

	// A restart forgets which items were handled (and the validators). The
	// next polls re-examine from the top; repeats do not spend the limit,
	// and each poll examines a bounded number of items.
	pollState.Lock()
	delete(pollState.bySource, sourceID)
	delete(pollState.handled, sourceID)
	pollState.Unlock()
	sent := len(sink.sent)
	warm := poll()
	if warm.New != 0 || warm.Repeated != 20 || warm.Deferred != 10 || len(sink.sent)-sent != 20 {
		t.Fatalf("first poll after a restart = %+v (sent %d); want 20 repeats examined (4x the limit) and 10 deferred", warm, len(sink.sent)-sent)
	}
	if rest := poll(); rest.Repeated != 10 || rest.Deferred != 0 {
		t.Fatalf("second poll after a restart = %+v; want the last 10 examined", rest)
	}
	if quiet := poll(); !quiet.NotModified {
		t.Fatalf("third poll after a restart = %+v; want a 304", quiet)
	}
}
