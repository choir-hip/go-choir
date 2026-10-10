package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

type slowSearchClient struct{ delay time.Duration }

func (c slowSearchClient) Search(ctx context.Context, query string, maxResults int) (*search.Response, error) {
	select {
	case <-time.After(c.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &search.Response{Query: query}, nil
}

// TestDeskGoEvalOwnTimeoutNamesItself reproduces R2 (desk-cell-ergonomics
// 2026-10-10): research set timeout_ms=2000 on a cell whose web search took
// longer, and the result said only "context deadline exceeded" with duration
// 0. The result must name the cell's own limit, the time spent, and the lost
// session.
func TestDeskGoEvalOwnTimeoutNamesItself(t *testing.T) {
	deskTestWorkerBin(t)
	rt := &Runtime{
		researchDeps: &researchtools.Dependencies{
			Search: slowSearchClient{delay: 3 * time.Second},
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
r, err := choir.WebSearch("slow", 3)
print(string(r), err)`
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`,"timeout_ms":500}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	var got struct {
		Error    string `json:"error"`
		Duration int64  `json:"duration"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	for _, want := range []string{"timeout_ms=500", "session was killed", "choir.WebSearch"} {
		if !strings.Contains(got.Error, want) {
			t.Fatalf("timeout error %q does not name %q", got.Error, want)
		}
	}
	if got.Duration < 500 {
		t.Fatalf("duration = %d ms, want the time the cell ran", got.Duration)
	}
}
