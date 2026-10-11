package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
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
	mu     sync.Mutex
	routes []string
}

func (p *researchChildProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	p.routes = append(p.routes, req.Provider+"/"+req.Model)
	p.mu.Unlock()
	raw := rawMessagesForTest(req.Messages)
	objective := ""
	for _, o := range []string{"alpha", "beta", "gamma", "report-attempt"} {
		if strings.Contains(raw, o) {
			objective = o
			break
		}
	}
	if objective == "gamma" {
		return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "findings for gamma via " + req.Provider + "/" + req.Model, Model: req.Model}, nil
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
// the whole call; a child's findings are not its final text. And from
// problems/research-child-picks-unserved-model-2026-10-11.md: a catalog
// model the computer's model policy does not name reaches the provider; its
// refusal does not name the models that would work; a policy model runs on
// the wrong provider; a policy role name (research asked for "research") is
// refused instead of meaning that role's model.
func TestResearchCellFansOutToChildActivations(t *testing.T) {
	deskTestWorkerBin(t)
	provider := &researchChildProvider{}
	rt := &Runtime{
		provider:    provider,
		modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{}),
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

	src := `res, err := choir.Research([]map[string]string{{"objective": "alpha"}, {"objective": "beta"}, {"objective": "report-attempt"}, {"objective": "alpha unknown model", "model": "no-such-model"}, {"objective": "alpha unserved model", "model": "gpt-5.4-mini"}, {"objective": "gamma", "model": "deepseek-v4.1-flash"}, {"objective": "gamma by role", "model": "engineering"}})
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
	if len(results) != 7 {
		t.Fatalf("results = %d, want 7: %+v", len(results), results)
	}
	for i, want := range []string{"alpha", "beta", "report-attempt", "alpha unknown model", "alpha unserved model", "gamma", "gamma by role"} {
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
	for _, r := range results[3:5] {
		if !strings.Contains(r.Error, "not configured on this computer") || !strings.Contains(r.Error, "gpt-5.6-luna") || !strings.Contains(r.Error, "deepseek-v4.1-flash") {
			t.Fatalf("a model outside the policy was not refused with the configured models: %+v", r)
		}
		if r.Findings != "" || r.DurationMs > 1000 {
			t.Fatalf("a refused model's child ran: %+v", r)
		}
	}
	if results[5].Findings != "findings for gamma via opencode-go/deepseek-v4.1-flash" || results[5].Error != "" {
		t.Fatalf("a policy model did not run on its policy provider: %+v", results[5])
	}
	if results[6].Findings != "findings for gamma via opencode-go/deepseek-v4.1-flash" || results[6].Model != "deepseek-v4.1-flash" {
		t.Fatalf("a policy role name did not select that role's model: %+v", results[6])
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	for _, route := range provider.routes {
		if strings.Contains(route, "gpt-5.4-mini") || strings.Contains(route, "no-such-model") {
			t.Fatalf("a model outside the policy reached the provider: %v", provider.routes)
		}
	}
}
