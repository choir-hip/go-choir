package researchtools

// Lead marking on Read and the bounded background call (search_leads.go).
// Failure list, written before the code:
//
//   R1 a Read that captured a page does not tell corpusd its URL, the final URL
//      and the capture id; one that failed (a 404) does not tell it the URL was
//      tried; a marker that errors fails or delays the Read
//   R2 the background call logs no `op= rows= ms= err=` line, waits for its
//      work on the caller's path, or drops or queues a call for volume (every
//      call runs, under its deadline, with the count in flight logged)
//   R3 the sink's lead write is not one POST of the search's results to
//      /internal/platform/search-leads

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type markCall struct {
	owner, computer, capture string
	urls                     []string
}

type markingSink struct {
	*CorpusdCaptureSink
	mu    sync.Mutex
	calls []markCall
	got   chan struct{}
	err   error
}

func (m *markingSink) MarkSearchLeadsTried(_ context.Context, owner, computer string, urls []string, capture string) error {
	m.mu.Lock()
	m.calls = append(m.calls, markCall{owner, computer, capture, urls})
	m.mu.Unlock()
	m.got <- struct{}{}
	return m.err
}

func TestReadMarksTheLeadTriedWithTheCaptureAndOnFailureAndNeverFailsOnTheMark(t *testing.T) {
	prev := sourcefetch.SetAllowPrivateNetworkForTests(true)
	defer sourcefetch.SetAllowPrivateNetworkForTests(prev)
	article := "<html><head><title>Harbor</title></head><body><article><p>The harbor board voted to reopen the quay after the storm, and the vote was carried by a wide margin of members.</p></article></body></html>"
	pages := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/harbor" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, article)
			return
		}
		http.NotFound(w, r)
	}))
	defer pages.Close()
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
		if r.URL.Path == "/internal/platform/captures" {
			og.HandleCaptures(w, r)
			return
		}
		og.HandleCaptureRead(w, r)
	}))
	defer corpusd.Close()
	sink := &markingSink{CorpusdCaptureSink: NewCorpusdCaptureSink(corpusd.URL), got: make(chan struct{}, 4), err: errors.New("corpusd is down")}
	deps := Dependencies{HTTP: sourcefetch.Client(10 * time.Second), Captures: sink}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "run", OwnerID: "owner-1", ComputerID: "computer-1", Profile: agentprofile.Research,
	})
	raw, err := deps.HostEgress(ctx, "read", json.RawMessage(`{"url":"`+pages.URL+`/harbor"}`))
	if err != nil {
		t.Fatalf("a failing marker failed the Read: %v", err)
	}
	var out struct {
		CaptureID string `json:"capture_id"`
	}
	_ = json.Unmarshal([]byte(raw), &out)
	wait := func() markCall {
		t.Helper()
		select {
		case <-sink.got:
		case <-time.After(5 * time.Second):
			t.Fatal("no mark reached the sink")
		}
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.calls[len(sink.calls)-1]
	}
	c := wait()
	if out.CaptureID == "" || c.capture != out.CaptureID || c.owner != "owner-1" || c.computer != "computer-1" || len(c.urls) == 0 || c.urls[0] != pages.URL+"/harbor" {
		t.Fatalf("mark after a captured read: %+v (capture %q)", c, out.CaptureID)
	}
	if _, err := deps.HostEgress(ctx, "read", json.RawMessage(`{"url":"`+pages.URL+`/missing"}`)); err == nil {
		t.Fatal("a 404 read succeeded")
	}
	c = wait()
	if c.capture != "" || len(c.urls) == 0 || c.urls[0] != pages.URL+"/missing" {
		t.Fatalf("mark after a failed read: %+v", c)
	}
}

func TestRunLeadCallRunsEveryCallAndLogsItsCountsAndInflight(t *testing.T) {
	logs := captureReadLog(t)
	hold := make(chan struct{})
	var ran atomic.Int64
	const n = 200 // far past the old bound of 32: none may be dropped
	var dones []<-chan struct{}
	for i := 0; i < n; i++ {
		d := RunLeadCall("lead_write", 3, func(ctx context.Context) error {
			ran.Add(1)
			select {
			case <-hold:
			case <-ctx.Done():
			}
			return errors.New("boom")
		})
		if d == nil {
			t.Fatalf("call %d was dropped", i)
		}
		dones = append(dones, d)
	}
	close(hold)
	for _, d := range dones {
		select {
		case <-d:
		case <-time.After(5 * time.Second):
			t.Fatal("a call did not finish")
		}
	}
	if ran.Load() != n {
		t.Fatalf("ran %d of %d calls", ran.Load(), n)
	}
	out := logs.String()
	if strings.Contains(out, "dropped") || strings.Count(out, "search leads op=lead_write rows=3 ms=") != n || !strings.Contains(out, "err=boom inflight=") {
		t.Fatalf("log lines: %.400q", out)
	}
}

func TestSinkWritesAnArrivalAsOnePost(t *testing.T) {
	var posts int
	var body platform.SearchLeadsWrite
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.URL.Path != "/internal/platform/search-leads" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"rows":2}`)
	}))
	defer srv.Close()
	n, err := NewCorpusdCaptureSink(srv.URL).WriteSearchLeads(context.Background(), SearchLeadsWrite{OwnerID: "o", ComputerID: "c", Query: "q", ConversationID: "v",
		Results: []SearchLeadHit{{URL: "https://a", Title: "A"}, {URL: "https://b", Title: "B"}}})
	if err != nil || n != 2 || posts != 1 || len(body.Results) != 2 || body.Query != "q" || body.ConversationID != "v" {
		t.Fatalf("n=%d err=%v posts=%d body=%+v", n, err, posts, body)
	}
	if _, err := NewCorpusdCaptureSink("").WriteSearchLeads(context.Background(), SearchLeadsWrite{}); err == nil {
		t.Fatal("no corpusd URL, no error")
	}
}

func TestSinkReadsLeadConversationsAsOnePost(t *testing.T) {
	var posts int
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.URL.Path != "/internal/platform/search-leads/by-capture" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"conversations":{"cap-1":["doc-a","doc-b"]}}`)
	}))
	defer srv.Close()
	var _ SearchLeadStoryReader = NewCorpusdCaptureSink(srv.URL)
	got, err := NewCorpusdCaptureSink(srv.URL).SearchLeadConversations(context.Background(), "o", "c", []string{"cap-1", "cap-2"})
	if err != nil || posts != 1 || len(got["cap-1"]) != 2 || len(got) != 1 || body["owner_id"] != "o" {
		t.Fatalf("got=%v err=%v posts=%d body=%v", got, err, posts, body)
	}
	if _, err := NewCorpusdCaptureSink("").SearchLeadConversations(context.Background(), "o", "c", nil); err == nil {
		t.Fatal("no corpusd URL, no error")
	}
}

func TestSinkReadsLeadPagesOfAConversationAsOnePost(t *testing.T) {
	var posts int
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts++
		if r.URL.Path != "/internal/platform/search-leads/by-conversation" || r.Method != http.MethodPost {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"pages":[{"capture_id":"cap-1","url":"https://a","title":"A"}]}`)
	}))
	defer srv.Close()
	var _ SearchLeadPageReader = NewCorpusdCaptureSink(srv.URL)
	got, err := NewCorpusdCaptureSink(srv.URL).SearchLeadPages(context.Background(), "o", "c", "doc-1", 50)
	if err != nil || posts != 1 || len(got) != 1 || got[0].CaptureID != "cap-1" || body["conversation_id"] != "doc-1" {
		t.Fatalf("got=%v err=%v posts=%d body=%v", got, err, posts, body)
	}
	if _, err := NewCorpusdCaptureSink("").SearchLeadPages(context.Background(), "o", "c", "d", 1); err == nil {
		t.Fatal("no corpusd URL, no error")
	}
}

// A task of more arrivals than the route takes in one request costs one request
// for each further leadCapturesPerRequest, and loses no lead (the route answers
// a 400 over its cap).
func TestSinkChunksLeadConversationReadsAtTheRoutesCap(t *testing.T) {
	var sizes []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []string `json:"capture_ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		sizes = append(sizes, len(body.IDs))
		out := map[string][]string{}
		for _, id := range body.IDs {
			if id == "cap-0" || id == "cap-1200" {
				out[id] = []string{"doc-" + id}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"conversations": out})
	}))
	defer srv.Close()
	var ids []string
	for i := 0; i <= 1200; i++ {
		ids = append(ids, fmt.Sprintf("cap-%d", i))
	}
	got, err := NewCorpusdCaptureSink(srv.URL).SearchLeadConversations(context.Background(), "o", "c", ids)
	if err != nil || len(sizes) != 3 || sizes[0] != 500 || sizes[1] != 500 || sizes[2] != 201 || len(got) != 2 || got["cap-1200"][0] != "doc-cap-1200" {
		t.Fatalf("sizes=%v got=%v err=%v", sizes, got, err)
	}
}
