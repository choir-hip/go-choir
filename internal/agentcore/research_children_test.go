package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// researchChildProvider scripts each child's model loop from its objective:
// first a desk_go_eval cell, then end_turn with findings that quote the
// cell's output.
type researchChildProvider struct {
	provideriface.Provider
}

func (p *researchChildProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	raw := rawMessagesForTest(req.Messages)
	objective := ""
	for _, o := range []string{"alpha", "beta", "report-attempt"} {
		if strings.Contains(raw, o) {
			objective = o
			break
		}
	}
	if !strings.Contains(raw, "call-child-cell") {
		source := fmt.Sprintf("r, err := choir.WebSearch(%q, 2)\nfmt.Println(string(r), err)", objective)
		if objective == "report-attempt" {
			source = `x, err := choir.Report("texture", "leak", nil, "")` + "\nfmt.Println(x, err)"
		}
		args, _ := json.Marshal(map[string]any{"source": source})
		return &provideriface.ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls:  []types.ToolCall{{ID: "call-child-cell", Name: "desk_go_eval", Arguments: args}},
			Model:      "test-model",
		}, nil
	}
	return &provideriface.ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "findings for " + objective + ": " + raw[strings.Index(raw, "call-child-cell"):],
		Model:      "test-model",
	}, nil
}

// Mission docs/definitions/choir-research-fanout-2026-10-10.md. Failure
// modes pinned: the parent cell gets no results or results out of order; a
// child cannot reach the web; a child can report (authority leak); children
// do not share the root activation's egress budget; one unknown model fails
// the whole call; a child's findings are not its final text.
func TestResearchCellFansOutToChildActivations(t *testing.T) {
	deskTestWorkerBin(t)
	rt := &Runtime{
		provider: &researchChildProvider{},
		researchDeps: &researchtools.Dependencies{
			Search: &stubSearchClient{},
			// One network call for the whole tree: the second child's search
			// must hit the root's exhausted budget.
			Egress: researchtools.NewEgressBudgetLedger(1, 32<<20),
		},
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.RunID = "run-research-root"
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src := `res, err := choir.Research([]map[string]string{{"objective": "alpha"}, {"objective": "beta"}, {"objective": "report-attempt"}, {"objective": "alpha unknown model", "model": "no-such-model"}})
b, _ := json.Marshal(res)
fmt.Println(string(b), err)`
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	var cell struct {
		Stdout string `json:"stdout"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &cell); err != nil || cell.Error != "" {
		t.Fatalf("cell failed: %v %s", err, out)
	}
	line := strings.TrimSpace(cell.Stdout)
	line = strings.TrimSuffix(line, "<nil>")
	var results []yaegikernel.ResearchChildResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &results); err != nil {
		t.Fatalf("decode results from %q: %v", cell.Stdout, err)
	}
	if len(results) != 4 {
		t.Fatalf("results = %d, want 4: %+v", len(results), results)
	}
	for i, want := range []string{"alpha", "beta", "report-attempt", "alpha unknown model"} {
		if results[i].Objective != want {
			t.Fatalf("result %d objective = %q, want %q", i, results[i].Objective, want)
		}
	}
	searched, refused := 0, 0
	for _, r := range results[:2] {
		if !strings.HasPrefix(r.Findings, "findings for "+r.Objective) {
			t.Fatalf("findings are not the child's final text: %+v", r)
		}
		if strings.Contains(r.Findings, "choir-egress-proof") {
			searched++
		}
		if strings.Contains(r.Findings, "budget exhausted") {
			refused++
		}
	}
	if searched != 1 || refused != 1 {
		t.Fatalf("children did not share the root budget (searched=%d refused=%d): %+v", searched, refused, results[:2])
	}
	if strings.Contains(results[2].Findings, "staged") || !strings.Contains(results[2].Findings, "Report") {
		t.Fatalf("a child reached choir.Report: %q", results[2].Findings)
	}
	if results[3].Error == "" || !strings.Contains(results[3].Error, "not in the catalog") {
		t.Fatalf("unknown model did not fail only its own child: %+v", results[3])
	}
}
