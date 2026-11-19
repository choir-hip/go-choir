package researchtools

// Station V5: a page research reads, or a source research polls, reaches the
// event layer. The hook (Dependencies.OnCaptured) is the one place research's
// captures are announced. Failure list, written before the code:
//
//   - O1 a page choir.Read captures is never announced, so the event layer
//     (which only a source schedule fed) never sees research's pages;
//   - O2 an unchanged re-read is announced again (the capture is not new), or a
//     read that failed is announced;
//   - O3 the announced capture has no title or no lead, so Jev is asked about a
//     blank item (a read page has no feed summary: its lead stands for it);
//   - O4 the lead is the whole page, or a heading, instead of a bounded run of
//     the page's own text;
//   - O5 research's own poll_source captures are not announced, or are
//     announced as page reads;
//   - O6 a hook that is not set (a test, a computer with no event layer)
//     changes what Read or poll_source returns.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/sources"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// holdSink is a capture store that remembers what it was given: an item with
// a URL it already holds is a repeat.
type holdSink struct {
	mu   sync.Mutex
	held map[string]string
}

func (s *holdSink) WriteCaptures(_ context.Context, _, _ string, items []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held == nil {
		s.held = map[string]string{}
	}
	var out sourcegraph.CaptureWriteResult
	for _, item := range items {
		if id, ok := s.held[item.URL]; ok {
			out.Repeated++
			out.Repeats = append(out.Repeats, sourcegraph.CapturedItem{CaptureID: id, URL: item.URL})
			continue
		}
		id := fmt.Sprintf("capture-%d", len(s.held)+1)
		s.held[item.URL] = id
		out.New++
		out.Captured = append(out.Captured, sourcegraph.CapturedItem{CaptureID: id, Title: item.Title, URL: item.URL, Summary: item.Summary})
	}
	return out, nil
}
func (s *holdSink) ReadCapture(context.Context, string, int, int) (json.RawMessage, error) {
	return nil, fmt.Errorf("not used")
}
func (s *holdSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, fmt.Errorf("not used")
}

type announced struct {
	owner, computer, source string
	items                   []sourcegraph.CapturedItem
	research                bool
}

func TestResearchReadAndPollAnnounceTheirNewCapturesToTheEventLayer(t *testing.T) {
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	defer sourcefetch.SetAllowPrivateNetworkForTests(prev)
	var longLead strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&longLead, "<p>Sentence %d of the harbour report says the council met and voted on the flood wall.</p>", i)
	}
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/harbor":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><title>Harbor Council Approves Flood Wall</title></head><body><article><h1>Harbor Council Approves Flood Wall</h1>`+longLead.String()+`</article></body></html>`)
		case "/untitled":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><body><article><p>Quake strikes off the coast and a tsunami warning is issued for the bay.</p></article></body></html>`)
		case "/feed":
			w.Header().Set("Content-Type", "application/rss+xml")
			fmt.Fprint(w, `<?xml version="1.0"?><rss version="2.0"><channel><title>Example</title><item><title>Ferries resume</title><link>https://news.example/1</link><description>Body one</description><guid>g-1</guid></item></channel></rss>`)
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nxxxx"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pages.Close()

	var mu sync.Mutex
	var got []announced
	reg := &sources.Registry{UserAgent: "test", Sources: []sources.Source{{ID: "rss:example", Type: sources.SourceTypeRSS, Name: "Example", URL: pages.URL + "/feed", MaxItemsPerPoll: 20}}}
	deps := Dependencies{HTTP: sourcefetch.Client(10 * time.Second), Sources: reg, Captures: &holdSink{},
		OnCaptured: func(_ context.Context, owner, computer, source string, items []sourcegraph.CapturedItem, research bool) {
			mu.Lock()
			defer mu.Unlock()
			got = append(got, announced{owner, computer, source, items, research})
		}}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "research-run", OwnerID: "owner-1", ComputerID: "computer-1", Profile: agentprofile.Research,
	})
	read := func(path string) (json.RawMessage, error) {
		return deps.HostEgress(ctx, "read", json.RawMessage(`{"url":"`+pages.URL+path+`"}`))
	}

	first, err := read("/harbor")
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		CaptureID string `json:"capture_id"`
	}
	_ = json.Unmarshal(first, &out)
	if len(got) != 1 { // O1
		t.Fatalf("a new page read announced %d times, want once", len(got))
	}
	a := got[0]
	if a.owner != "owner-1" || a.computer != "computer-1" || a.source != sourcegraph.ReadSourceID || !a.research || len(a.items) != 1 {
		t.Fatalf("announcement = %+v", a)
	}
	item := a.items[0]
	if item.CaptureID != out.CaptureID || item.Title != "Harbor Council Approves Flood Wall" { // O3
		t.Fatalf("announced item = %+v, read returned %s", item, out.CaptureID)
	}
	// O3, O4: the lead is the page's own running text, not its heading, and is
	// bounded.
	if !strings.HasPrefix(item.Summary, "Sentence 0 of the harbour report") || strings.Contains(item.Summary, "Harbor Council Approves Flood Wall") {
		t.Fatalf("lead = %q", item.Summary)
	}
	if n := len([]rune(item.Summary)); n > readLeadRunes {
		t.Fatalf("lead is %d runes, over %d", n, readLeadRunes)
	}

	// O2: an unchanged re-read, and a read that fails, announce nothing.
	if _, err := read("/harbor"); err != nil {
		t.Fatal(err)
	}
	if _, err := read("/image.png"); err == nil {
		t.Fatal("an image was read")
	}
	if len(got) != 1 {
		t.Fatalf("re-read and failed read announced: %d announcements", len(got))
	}

	// O3: a page with no title is announced with its first text for a title.
	if _, err := read("/untitled"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !strings.HasPrefix(got[1].items[0].Title, "Quake strikes off the coast") {
		t.Fatalf("untitled page announced as %+v", got)
	}

	// O5: research's own poll announces its new captures, as a poll.
	if _, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"rss:example"}`)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[2].research || got[2].source != "rss:example" || len(got[2].items) != 1 || got[2].items[0].Summary != "Body one" {
		t.Fatalf("poll announced %+v", got[2:])
	}
	// A repeat poll writes nothing new and announces nothing.
	if _, err := deps.HostEgress(ctx, "poll_source", json.RawMessage(`{"source_id":"rss:example"}`)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("a repeat poll announced: %d", len(got))
	}

	// O6: with no hook the tools answer the same.
	deps.OnCaptured = nil
	deps2 := deps
	deps2.Captures = &holdSink{}
	again, err := deps2.HostEgress(ctx, "read", json.RawMessage(`{"url":"`+pages.URL+`/harbor"}`))
	if err != nil || !strings.Contains(string(again), `"new":true`) {
		t.Fatalf("read without a hook: %s %v", again, err)
	}
}
