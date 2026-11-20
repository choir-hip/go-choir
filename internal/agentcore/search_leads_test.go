package agentcore

// The search arrival's lead write (search_leads.go). Failure list, written
// before the code:
//
//   A1 an arrival's results are not sent to corpusd in one call with the
//      owner, computer, conversation, query and every result's title, snippet,
//      URL and provider date; or an arrival with no results sends a call
//   A2 a sink that fails or hangs makes the observer block, panic or return an
//      error to the search
//   A3 a computer without a lead-taking sink or without an owner gets an
//      observer that does something

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type fakeLeadSink struct {
	researchtools.CaptureSink
	mu      sync.Mutex
	writes  []researchtools.SearchLeadsWrite
	err     error
	release chan struct{}
	got     chan struct{}
}

func (f *fakeLeadSink) WriteSearchLeads(ctx context.Context, w researchtools.SearchLeadsWrite) (int, error) {
	f.mu.Lock()
	f.writes = append(f.writes, w)
	f.mu.Unlock()
	f.got <- struct{}{}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
		}
	}
	return len(w.Results), f.err
}

func (f *fakeLeadSink) MarkSearchLeadsTried(context.Context, string, string, []string, string) error {
	return nil
}

// leadLogBuffer is a log sink a test may read while a background lead call is
// still writing to it.
type leadLogBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *leadLogBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *leadLogBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func leadRuntime(sink researchtools.CaptureSink) *Runtime {
	return &Runtime{researchDeps: &researchtools.Dependencies{Captures: sink}}
}

func TestSearchLeadObserverSendsOneCallPerSearchAndNothingForNoResults(t *testing.T) {
	sink := &fakeLeadSink{got: make(chan struct{}, 4)}
	obs := leadRuntime(sink).searchLeadObserver("owner-1", "comp-1", "conv-1")
	if obs == nil {
		t.Fatal("no observer")
	}
	obs("harbor reopens", nil)
	obs("harbor reopens", []search.Result{
		{Title: "Harbor", URL: "https://example.com/h", Snippet: "s1", PublishedAt: "2026-11-18T10:00:00Z", Provider: "keenable"},
		{Title: "Quay", URL: "https://example.com/q", Snippet: "s2"},
	})
	select {
	case <-sink.got:
	case <-time.After(5 * time.Second):
		t.Fatal("no write reached the sink")
	}
	select {
	case <-sink.got:
		t.Fatal("a second call was made for one arrival")
	case <-time.After(200 * time.Millisecond):
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(sink.writes))
	}
	w := sink.writes[0]
	if w.OwnerID != "owner-1" || w.ComputerID != "comp-1" || w.ConversationID != "conv-1" || w.Query != "harbor reopens" || len(w.Results) != 2 {
		t.Fatalf("write: %+v", w)
	}
	if w.Results[0] != (researchtools.SearchLeadHit{URL: "https://example.com/h", Title: "Harbor", Snippet: "s1", PublishedAt: "2026-11-18T10:00:00Z"}) {
		t.Fatalf("first hit: %+v", w.Results[0])
	}
}

func TestSearchLeadObserverNeverBlocksOrFailsTheSearchAndLogsTheCount(t *testing.T) {
	logs := &leadLogBuffer{}
	orig := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(orig) })
	sink := &fakeLeadSink{got: make(chan struct{}, 4), err: errors.New("corpusd said no"), release: make(chan struct{})}
	obs := leadRuntime(sink).searchLeadObserver("owner-1", "comp-1", "conv-1")
	returned := make(chan struct{})
	go func() {
		obs("q", []search.Result{{Title: "T", URL: "https://example.com/x", Snippet: "s"}})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("the observer waited for a hung corpusd")
	}
	select {
	case <-sink.got:
	case <-time.After(5 * time.Second):
		t.Fatal("no write reached the sink")
	}
	close(sink.release)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(logs.String(), "search leads op=lead_write rows=1 ms=") && strings.Contains(logs.String(), "err=corpusd said no inflight=") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no lead_write line with rows and err: %q", logs.String())
}

func TestSearchLeadObserverIsNilWithoutASinkOrAnOwner(t *testing.T) {
	if leadRuntime(&struct{ researchtools.CaptureSink }{}).searchLeadObserver("o", "c", "x") != nil {
		t.Fatal("a sink that takes no leads got an observer")
	}
	if leadRuntime(&fakeLeadSink{}).searchLeadObserver("", "c", "x") != nil {
		t.Fatal("no owner, an observer")
	}
	if (&Runtime{}).searchLeadObserver("o", "c", "x") != nil {
		t.Fatal("no research deps, an observer")
	}
}

// A4 a search's results reach the observer more than once, or an arrival's
// results are sent apart from the rest of the search's.
func TestSearchLeadBatchSendsOneCallWithEveryArrivalsResults(t *testing.T) {
	var calls [][]search.Result
	b := searchLeadBatch{query: "q", leads: func(q string, r []search.Result) {
		if q != "q" {
			t.Errorf("query %q", q)
		}
		calls = append(calls, r)
	}}
	b.add([]search.Result{{URL: "https://a/1"}, {URL: "https://a/2"}})
	b.add(nil)
	b.add([]search.Result{{URL: "https://b/1"}})
	if len(calls) != 0 {
		t.Fatal("a call was made before the search ended")
	}
	b.flush()
	b.flush()
	if len(calls) != 1 || len(calls[0]) != 3 {
		t.Fatalf("calls = %+v, want one call of 3 results", calls)
	}
	empty := searchLeadBatch{query: "q", leads: func(string, []search.Result) { t.Error("a search with no results made a call") }}
	empty.add(nil)
	empty.flush()
}

// A5 a search with many results, among many concurrent searches, loses a lead
// (the old observer dropped calls past 32 in flight).
func TestSearchLeadObserverLosesNoLeadUnderManyConcurrentSearches(t *testing.T) {
	const searches, perSearch = 150, 40
	sink := &fakeLeadSink{got: make(chan struct{}, searches), release: make(chan struct{})}
	obs := leadRuntime(sink).searchLeadObserver("owner-1", "comp-1", "conv-1")
	var wg sync.WaitGroup
	for i := 0; i < searches; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rs := make([]search.Result, perSearch)
			for j := range rs {
				rs[j] = search.Result{Title: "t", URL: fmt.Sprintf("https://example.com/%d/%d", i, j)}
			}
			obs(fmt.Sprintf("q%d", i), rs)
		}(i)
	}
	wg.Wait()
	for i := 0; i < searches; i++ {
		select {
		case <-sink.got:
		case <-time.After(5 * time.Second):
			t.Fatalf("only %d of %d searches reached the sink", i, searches)
		}
	}
	close(sink.release)
	sink.mu.Lock()
	defer sink.mu.Unlock()
	var leads atomic.Int64
	for _, w := range sink.writes {
		leads.Add(int64(len(w.Results)))
	}
	if len(sink.writes) != searches || leads.Load() != searches*perSearch {
		t.Fatalf("writes = %d, leads = %d; want %d and %d", len(sink.writes), leads.Load(), searches, searches*perSearch)
	}
}

// leadSearchTool is researchSearchTool with a lead-taking sink on the runtime,
// so the real forwardSearch (through a real desk session worker) writes leads.
func leadSearchTool(t *testing.T, client search.Client, sink researchtools.CaptureSink) (toolregistry.Tool, context.Context, *deskSessionWorkers, string) {
	t.Helper()
	deskTestWorkerBin(t)
	rt := &Runtime{researchDeps: &researchtools.Dependencies{Search: client, Captures: sink}}
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:search", OwnerID: execCtx.OwnerID}
	key, _ := deskSessionKey(execCtx, 0)
	return newDeskGoEvalTool(rt, workers, agentprofile.Research), toolregistry.WithExecutionContext(context.Background(), execCtx), workers, key
}

const leadWaitForArrival = `import "time"
s, err := choir.SearchStart("alpha")
if err != nil { panic(err) }
r, _ := choir.Searches.Get(s.ID)
for i := 0; i < 500 && len(r.Arrivals) < 1; i++ {
	time.Sleep(10 * time.Millisecond)
	r, _ = choir.Searches.Get(s.ID)
}
fmt.Println("arrivals", len(r.Arrivals))`

// A6 a real search that ends normally, or by its session ending after one
// arrival, makes more or fewer than one lead write, or loses an arrival's
// results.
func TestForwardSearchWritesOneBatchWhenTheSearchEndsOrItsSessionDoes(t *testing.T) {
	t.Run("normal end", func(t *testing.T) {
		sink := &fakeLeadSink{got: make(chan struct{}, 8)}
		client := newPipeSearchClient("fast", "slow")
		tool, ctx, _, _ := leadSearchTool(t, client, sink)
		go func() {
			query := <-client.opened
			client.provider(t, query, "fast")
			client.provider(t, query, "slow")
			client.done(t, query, 2)
		}()
		if out := evalDeskCell(t, tool, ctx, leadWaitForArrival); out.Error != "" {
			t.Fatalf("cell: %s", out.Error)
		}
		select {
		case <-sink.got:
		case <-time.After(10 * time.Second):
			t.Fatal("no lead write after the search ended")
		}
		select {
		case <-sink.got:
			t.Fatal("a second lead write for one search")
		case <-time.After(300 * time.Millisecond):
		}
		sink.mu.Lock()
		defer sink.mu.Unlock()
		if len(sink.writes) != 1 || len(sink.writes[0].Results) != 2 || sink.writes[0].Query != "alpha" || sink.writes[0].OwnerID != "owner-1" {
			t.Fatalf("writes: %+v", sink.writes)
		}
	})
	t.Run("session ends after one arrival", func(t *testing.T) {
		sink := &fakeLeadSink{got: make(chan struct{}, 8)}
		client := newPipeSearchClient("fast", "never")
		tool, ctx, workers, key := leadSearchTool(t, client, sink)
		go func() {
			query := <-client.opened
			client.provider(t, query, "fast")
		}()
		if out := evalDeskCell(t, tool, ctx, leadWaitForArrival); out.Error != "" {
			t.Fatalf("cell: %s", out.Error)
		}
		workers.release(key)
		select {
		case <-sink.got:
		case <-time.After(10 * time.Second):
			t.Fatal("the arrived results were lost when the session ended")
		}
		sink.mu.Lock()
		defer sink.mu.Unlock()
		if len(sink.writes) != 1 || len(sink.writes[0].Results) != 1 {
			t.Fatalf("writes: %+v", sink.writes)
		}
	})
}
