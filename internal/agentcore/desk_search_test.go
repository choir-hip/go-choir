package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Async search, host side (Cut 2 unit 2a,
// docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md).
// Ways it can fail, written before the code:
//  1. A stream outlives its desk session: release or reclaim closes the
//     worker but the gateway stream keeps running (and pushing into a dead
//     socket) until the gateway's bound.
//  2. An arrival is pushed under another search's id, or two searches in
//     one cell share an id.
//  3. A provider that answers after SearchStart returned never reaches the
//     running cell, or reaches it only at the next cell.
//  4. The count line is missing from the desk's cell result, or arrival
//     payloads leak into the result although the cell never read them.
//  5. SearchStart without a search client hangs or succeeds silently.

// stubSearchClient answers every search at once from one provider whose
// only result is choir-egress-proof.
type stubSearchClient struct{}

func (stubSearchClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	body := `{"type":"start","query":` + jsonString(query) + `,"providers":["stub"],"cooling":[]}
{"type":"provider","provider":"stub","status":"success","latency_ms":1,"results":[{"title":"choir-egress-proof","url":"https://example.test/x"}]}
{"type":"done","answered":1,"total":1,"cooling":[]}
`
	return search.ReadStream(io.NopCloser(strings.NewReader(body)))
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// cellSource JSON-encodes a cell source string for embedding in the eval payload.
func cellSource(s string) string {
	return jsonString(s)
}

// pipeSearchClient streams each search through a pipe the test writes:
// the start line at once, provider lines when the test sends them. A
// search's context ending closes its pipe, as it would an HTTP body.
type pipeSearchClient struct {
	providers []string
	mu        sync.Mutex
	searches  map[string]*pipeSearch
	opened    chan string
}

type pipeSearch struct {
	w   *io.PipeWriter
	ctx context.Context
}

func newPipeSearchClient(providers ...string) *pipeSearchClient {
	return &pipeSearchClient{providers: providers, searches: map[string]*pipeSearch{}, opened: make(chan string, 16)}
}

func (c *pipeSearchClient) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	pr, pw := io.Pipe()
	context.AfterFunc(ctx, func() { _ = pw.CloseWithError(ctx.Err()) })
	c.mu.Lock()
	c.searches[query] = &pipeSearch{w: pw, ctx: ctx}
	c.mu.Unlock()
	providers, _ := json.Marshal(c.providers)
	go func() {
		_, _ = fmt.Fprintf(pw, `{"type":"start","query":%s,"providers":%s,"cooling":[{"provider":"cold","state":"cooling_down"}]}`+"\n", jsonString(query), providers)
	}()
	stream, err := search.ReadStream(pr)
	if err == nil {
		c.opened <- query
	}
	return stream, err
}

func (c *pipeSearchClient) search(query string) *pipeSearch {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.searches[query]
}

func (c *pipeSearchClient) provider(t *testing.T, query, provider string) {
	t.Helper()
	line := fmt.Sprintf(`{"type":"provider","provider":%q,"status":"success","latency_ms":3,"results":[{"title":"%s from %s","url":"https://%s.example/%s"}]}`+"\n", provider, query, provider, query, provider)
	if _, err := io.WriteString(c.search(query).w, line); err != nil {
		t.Errorf("write %s line for %q: %v", provider, query, err)
	}
}

func (c *pipeSearchClient) done(t *testing.T, query string, answered int) {
	t.Helper()
	line := fmt.Sprintf(`{"type":"done","answered":%d,"total":%d,"cooling":[]}`+"\n", answered, len(c.providers))
	if _, err := io.WriteString(c.search(query).w, line); err != nil {
		t.Errorf("write done line for %q: %v", query, err)
	}
	_ = c.search(query).w.Close()
}

func (c *pipeSearchClient) cancelled(query string) bool {
	s := c.search(query)
	if s == nil {
		return false
	}
	deadline := time.Now().Add(2 * time.Second)
	for s.ctx.Err() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	return s.ctx.Err() != nil
}

func researchSearchTool(t *testing.T, client search.Client) (toolregistry.Tool, context.Context, *deskSessionWorkers, string) {
	t.Helper()
	return researchSearchToolWith(t, client, nil)
}

// researchSearchToolWith is researchSearchTool with a hook on the runtime
// (a per-computer setting under test).
func researchSearchToolWith(t *testing.T, client search.Client, configure func(*Runtime)) (toolregistry.Tool, context.Context, *deskSessionWorkers, string) {
	t.Helper()
	deskTestWorkerBin(t)
	rt := &Runtime{}
	if configure != nil {
		configure(rt)
	}
	if client != nil {
		rt.researchDeps = &researchtools.Dependencies{Search: client}
	}
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:search", OwnerID: execCtx.OwnerID}
	key, _ := deskSessionKey(execCtx)
	return newDeskGoEvalTool(rt, workers, agentprofile.Research), toolregistry.WithExecutionContext(context.Background(), execCtx), workers, key
}

func TestDeskSearchArrivalsReachTheRunningCellAndCountOnce(t *testing.T) {
	client := newPipeSearchClient("fast", "slow")
	tool, ctx, _, _ := researchSearchTool(t, client)
	go func() {
		query := <-client.opened
		client.provider(t, query, "fast")
		time.Sleep(300 * time.Millisecond)
		client.provider(t, query, "slow")
		client.done(t, query, 2)
	}()
	out := evalDeskCell(t, tool, ctx, `import "time"
s, err := choir.SearchStart("alpha")
if err != nil { panic(err) }
fmt.Println(s)
r, _ := choir.Searches.Get(s.ID)
for i := 0; i < 500 && !r.Done; i++ {
	time.Sleep(10 * time.Millisecond)
	r, _ = choir.Searches.Get(s.ID)
}
fmt.Println("arrivals", len(r.Arrivals), "answered", r.Answered, "done", r.Done)`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	if !strings.Contains(out.Stdout, "s1 started: 2 providers (1 cooling: cold)") || !strings.Contains(out.Stdout, "arrivals 2 answered 2 done true") {
		t.Fatalf("the running cell did not see both providers: %q", out.Stdout)
	}
	if got := strings.Join(out.Searches, "|"); got != "s1: 2/2 answered, +2 new, done" {
		t.Fatalf("count lines = %q", got)
	}
	if strings.Contains(out.Stdout, "alpha.example") {
		t.Fatalf("arrival payloads reached a result the cell never printed: %q", out.Stdout)
	}
	if again := evalDeskCell(t, tool, ctx, `fmt.Println(len(choir.Searches.List()))`); strings.TrimSpace(again.Stdout) != "1" || len(again.Searches) != 0 {
		t.Fatalf("next cell = %+v, want the search kept and no count line", again)
	}
}

func TestDeskSearchesKeepTheirOwnIDs(t *testing.T) {
	client := newPipeSearchClient("a", "b")
	tool, ctx, _, _ := researchSearchTool(t, client)
	go func() {
		first, second := <-client.opened, <-client.opened
		for _, p := range []string{"a", "b"} {
			client.provider(t, second, p)
			client.provider(t, first, p)
		}
		client.done(t, first, 2)
		client.done(t, second, 2)
	}()
	out := evalDeskCell(t, tool, ctx, `x, _ := choir.SearchStart("alpha")
y, _ := choir.SearchStart("beta")
for _, id := range []string{x.ID, y.ID} {
	c, err := choir.SearchWait(id, 5000)
	r, _ := choir.Searches.Get(id)
	own := true
	for _, a := range r.Arrivals {
		for _, hit := range a.Results {
			own = own && strings.HasPrefix(hit.URL, "https://"+r.Query+".example/")
		}
	}
	fmt.Println(id, r.Query, c, err, "own", own)
}`)
	if out.Error != "" {
		t.Fatalf("cell error: %s", out.Error)
	}
	for _, want := range []string{"s1 alpha s1: 2/2 answered, done <nil> own true", "s2 beta s2: 2/2 answered, done <nil> own true"} {
		if !strings.Contains(out.Stdout, want) {
			t.Fatalf("stdout %q lacks %q", out.Stdout, want)
		}
	}
}

func TestDeskSearchStreamsEndWithTheirSession(t *testing.T) {
	client := newPipeSearchClient("never")
	tool, ctx, workers, key := researchSearchTool(t, client)

	// release: the search started in this session is cancelled.
	if out := evalDeskCell(t, tool, ctx, `s, err := choir.SearchStart("released"); fmt.Println(s, err)`); out.Error != "" || !strings.Contains(out.Stdout, "s1 started") {
		t.Fatalf("start cell: %+v", out)
	}
	<-client.opened
	workers.release(key)
	if !client.cancelled("released") {
		t.Fatal("release left the session's search streaming")
	}

	// reclaim: an idle session past retention takes its searches with it.
	if out := evalDeskCell(t, tool, ctx, `s, err := choir.SearchStart("reclaimed"); fmt.Println(s, err)`); out.Error != "" || !strings.Contains(out.Stdout, "s1 started") {
		t.Fatalf("start cell in the new session: %+v", out)
	}
	<-client.opened
	workers.mu.Lock()
	workers.reclaimLocked(time.Now().Add(2 * deskSessionRetention))
	workers.mu.Unlock()
	if !client.cancelled("reclaimed") {
		t.Fatal("reclaim left the session's search streaming")
	}
}

func TestDeskSearchStartWithoutAClientRefuses(t *testing.T) {
	tool, ctx, _, _ := researchSearchTool(t, nil)
	out := evalDeskCell(t, tool, ctx, `s, err := choir.SearchStart("q"); fmt.Println("err:", err, s.ID == "")`)
	if !strings.Contains(out.Stdout, "err:") || !strings.Contains(out.Stdout, "unavailable") || !strings.Contains(out.Stdout, "true") {
		t.Fatalf("SearchStart without a search client = %+v, want a refusal", out)
	}
}
