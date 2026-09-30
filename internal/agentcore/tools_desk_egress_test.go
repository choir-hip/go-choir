package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// stubSearchClient is a deterministic search.Client for the end-to-end egress
// proof — it returns a fixed result so the cell observes a real host-mediated
// web_search without a live gateway.
type stubSearchClient struct{ gotQuery string }

func (c *stubSearchClient) Search(ctx context.Context, query string, maxResults int) (*search.Response, error) {
	c.gotQuery = query
	return &search.Response{
		Query: query,
		Results: []map[string]any{
			{"title": "choir-egress-proof", "url": "https://example.test/x"},
		},
	}, nil
}

// TestDeskGoEvalWebSearchEgressRoundTrip drives a real desk_go_eval cell that
// calls choir.WebSearch: the request crosses the session socket as a
// StreamBrokerEgress frame, the host's workerCfg.Egress dispatches to
// researchDeps.HostEgress -> the deps-bound web_search ToolFunc -> the stubbed
// search client, and the bounded JSON result returns into the cell. This is
// the one-tool cutover's load-bearing path: capability reachable as a verb,
// egress budget charged, never an open socket.
func TestDeskGoEvalWebSearchEgressRoundTrip(t *testing.T) {
	deskTestWorkerBin(t)
	stub := &stubSearchClient{}
	rt := &Runtime{
		researchDeps: &researchtools.Dependencies{
			Search: stub,
			Egress: researchtools.NewEgressBudgetLedger(64, 32<<20),
		},
	}
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src := `import "choir"
res, err := choir.WebSearch("choir supervision", 3)
if err != nil { print("ws_err: ", err) } else { print("ws_result: ", string(res)) }`
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`,"timeout_ms":20000}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	if strings.Contains(out, "ws_err") {
		t.Fatalf("choir.WebSearch errored inside the cell: %s", out)
	}
	if !strings.Contains(out, "choir-egress-proof") {
		t.Fatalf("cell did not receive host-resolved search result: %s", out)
	}
	if stub.gotQuery != "choir supervision" {
		t.Fatalf("host search saw query %q, want choir supervision", stub.gotQuery)
	}
}

// TestDeskGoEvalWebSearchUnavailableRefused proves a missing host egress
// surface refuses the verb into the cell (never a silent socket or hang) —
// the governance the cutover must preserve.
func TestDeskGoEvalWebSearchUnavailableRefused(t *testing.T) {
	deskTestWorkerBin(t)
	rt := &Runtime{} // nil researchDeps -> HostEgress reports host unavailable.
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src := `import "choir"
r, err := choir.WebSearch("q", 1)
if err != nil { print("ws_err: ", err) } else { print("ws_ok res=", string(r)) }`
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`,"timeout_ms":20000}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	if !strings.Contains(out, "ws_err") || !strings.Contains(out, "unavailable") {
		t.Fatalf("nil-deps WebSearch must refuse into the cell, got: %s", out)
	}
}

// cellSource JSON-encodes a cell source string for embedding in the eval payload.
func cellSource(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
