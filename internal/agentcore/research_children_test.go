package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
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
	at     []time.Time
}

func (p *researchChildProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	p.routes = append(p.routes, req.Provider+"/"+req.Model)
	p.at = append(p.at, time.Now())
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
		source := fmt.Sprintf("s, err := choir.SearchStart(%q)\nif err == nil { _, err = choir.SearchWait(s.ID, 5000) }\nr, _ := choir.Searches.Get(s.ID)\nb, _ := json.Marshal(r.Results)\nfmt.Println(string(b), err)", objective)
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

// Mission docs/archive/definitions/choir-research-fanout-2026-10-10.md. Failure
// modes pinned: the parent cell gets no results or results out of order; a
// child cannot reach the web; a child can report (authority leak); children
// are not budgeted against the root; one unknown model fails
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
			Search: stubSearchClient{},
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
	searched := 0
	for _, r := range results[:2] {
		if !strings.HasPrefix(r.Findings, "findings for "+r.Objective) {
			t.Fatalf("findings are not the child's final text: %+v", r)
		}
		if strings.Contains(r.Findings, "choir-egress-proof") {
			searched++
		}
	}
	if searched != 2 {
		t.Fatalf("children did not both reach the web (searched=%d): %+v", searched, results[:2])
	}
	if strings.Contains(results[2].Findings, "staged") || !strings.Contains(results[2].Findings, "Report") {
		t.Fatalf("a child reached choir.Report: %q", results[2].Findings)
	}
	for _, r := range results[3:5] {
		if !strings.Contains(r.Error, "not in the catalog") || !strings.Contains(r.Error, "gpt-6-luna") || !strings.Contains(r.Error, "deepseek-v4.1-flash") || !strings.Contains(r.Error, "pool:default") {
			t.Fatalf("a model outside the catalog was not refused with the models that work: %+v", r)
		}
		if r.Findings != "" || r.DurationMs > 1000 {
			t.Fatalf("a refused model's child ran: %+v", r)
		}
	}
	if results[5].Findings != "findings for gamma via opencode-go/deepseek-v4.1-flash" || results[5].Error != "" {
		t.Fatalf("a policy model did not run on its policy provider: %+v", results[5])
	}
	// A role name means that role's spec, the default pool: the child is
	// assigned one member and runs on its provider.
	viaPool := ""
	for _, m := range modelcatalog.Pools()[modelcatalog.DefaultPool] {
		if m.Model == results[6].Model {
			viaPool = m.Provider + "/" + m.Model
		}
	}
	if viaPool == "" || results[6].Findings != "findings for gamma via "+viaPool {
		t.Fatalf("a policy role name did not select a member of that role's pool: %+v", results[6])
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	for _, route := range provider.routes {
		if strings.Contains(route, "gpt-5.4-mini") || strings.Contains(route, "no-such-model") {
			t.Fatalf("a model outside the policy reached the provider: %v", provider.routes)
		}
	}
}

// problems/research-fanout-overruns-computer-model-budget-2026-10-11.md: a
// tree of children called the model as fast as it could and the gateway's
// per-computer limiter rejected thousands of calls. Failure mode pinned:
// child model calls are not spaced by one computer-wide gate, so a wide or
// deep tree bursts past the computer's budget.
func TestResearchChildModelCallsShareOneComputerGate(t *testing.T) {
	deskTestWorkerBin(t)
	provider := &researchChildProvider{}
	const interval = 150 * time.Millisecond
	rt := &Runtime{
		provider:           provider,
		modelPolicy:        modelpolicy.NewManager(modelpolicy.ManagerConfig{}),
		researchChildCalls: &callSpacer{interval: interval},
		researchDeps: &researchtools.Dependencies{
			Search: stubSearchClient{},
		},
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.RunID = "run-research-gate"
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	src := `res, err := choir.Research([]string{"alpha", "beta", "gamma", "alpha again"})
fmt.Println(len(res), err)`
	if _, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`}`)); err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	at := append([]time.Time(nil), provider.at...)
	sort.Slice(at, func(i, j int) bool { return at[i].Before(at[j]) })
	if len(at) < 4 {
		t.Fatalf("children made %d model calls, want at least 4", len(at))
	}
	for i := 1; i < len(at); i++ {
		if gap := at[i].Sub(at[i-1]); gap < interval-20*time.Millisecond {
			t.Fatalf("child model calls %d and %d were %s apart, want at least %s: %v", i-1, i, gap, interval, at)
		}
	}
}

// timedChildProvider: objective "hang" never answers until its deadline;
// objective "nest" tries to fan out once, then ends with what its cell
// printed. It records whether each child was told its time.
type timedChildProvider struct {
	provideriface.Provider
	mu   sync.Mutex
	told []bool
}

func (p *timedChildProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	raw := rawMessagesForTest(req.Messages)
	// The time budget is the child's run-context turn, not its system
	// prompt (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
	p.told = append(p.told, strings.Contains(raw, "You have about") && !strings.Contains(req.System, "You have about"))
	p.mu.Unlock()
	if strings.Contains(raw, "hang") && !strings.Contains(raw, "nest") {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if !strings.Contains(raw, "call-nest-cell") {
		args, _ := json.Marshal(map[string]any{"source": "r, err := choir.Research(\"hang\")\nfmt.Println(len(r), err)"})
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "call-nest-cell", Name: "desk_go_eval", Arguments: args}}, Model: "test-model"}, nil
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "nest saw: " + raw[strings.Index(raw, "call-nest-cell"):], Model: "test-model"}, nil
}

// problems/research-fanout-overruns-computer-model-budget-2026-10-11.md
// (amendment 00:52Z): children delegated again and again, and the desk's
// choir.Research call never returned. Failure modes pinned: a desk's call
// waits past its window for a child that never answers; a child is not
// told how long it has; a child can fan out again (owner 2026-10-11: rounds,
// not recursion; PD1).
func TestResearchTreeReturnsWithinItsTime(t *testing.T) {
	deskTestWorkerBin(t)
	window := researchCallWindow
	researchCallWindow = 4 * time.Second
	defer func() { researchCallWindow = window }()
	provider := &timedChildProvider{}
	rt := &Runtime{
		provider:           provider,
		modelPolicy:        modelpolicy.NewManager(modelpolicy.ManagerConfig{}),
		researchChildCalls: &callSpacer{},
		researchDeps:       &researchtools.Dependencies{Search: stubSearchClient{}},
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.RunID = "run-research-timed"
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	src := `res, err := choir.Research([]string{"hang", "nest"})
b, _ := json.Marshal(res)
fmt.Println(string(b), err)`
	start := time.Now()
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second+2*time.Second {
		t.Fatalf("the desk's Research call took %s, past its 4 s window", elapsed)
	}
	var cell struct{ Stdout, Error string }
	if err := json.Unmarshal([]byte(out), &cell); err != nil || cell.Error != "" {
		t.Fatalf("cell failed: %v %s", err, out)
	}
	line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(cell.Stdout), "<nil>"))
	var results []yaegikernel.ResearchChildResult
	if err := json.Unmarshal([]byte(line), &results); err != nil || len(results) != 2 {
		t.Fatalf("decode results from %q: %v", cell.Stdout, err)
	}
	if !strings.Contains(results[0].Error, "deadline") {
		t.Fatalf("a child that never answered did not end at its deadline: %+v", results[0])
	}
	if !strings.Contains(results[1].Findings, "choir has no Research") {
		t.Fatalf("a child reached choir.Research (children cannot fan out further): %+v", results[1])
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	for i, told := range provider.told {
		if !told {
			t.Fatalf("child call %d was not told its time", i)
		}
	}
}

// spendingChildProvider: objective "two-calls" runs one cell, then ends;
// objective "one-call" ends at once. Every call reports fixed usage.
type spendingChildProvider struct {
	provideriface.Provider
}

func (p *spendingChildProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	raw := rawMessagesForTest(req.Messages)
	if strings.Contains(raw, "two-calls") && !strings.Contains(raw, "call-spend-cell") {
		args, _ := json.Marshal(map[string]any{"source": `fmt.Println("working")`})
		return &provideriface.ToolLoopResponse{StopReason: "tool_use", ToolCalls: []types.ToolCall{{ID: "call-spend-cell", Name: "desk_go_eval", Arguments: args}}, Model: "test-model",
			Usage: provideriface.TokenUsage{InputTokens: 100, OutputTokens: 10, CachedInputTokens: 40}}, nil
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done", Model: "test-model",
		Usage: provideriface.TokenUsage{InputTokens: 200, OutputTokens: 20, CachedInputTokens: 150}}, nil
}

// ARCH-08 slice 1 (docs/design/agent-tree-budgets-2026-10-21.md; problem
// docs/problems/agent-tree-budgets-are-time-only-2026-10-21.md): track a
// tree's spending, never cap it. Failure modes pinned: the parent cell
// cannot see what a child spent; a child's spending is its last call's, not
// the sum of its loop; a child refused before it ran reports tokens; the
// tree's total is not the sum of its children; a tree is refused or cut
// short for what it spent.
func TestResearchChildrenReportTheirSpending(t *testing.T) {
	deskTestWorkerBin(t)
	rt := &Runtime{
		provider:           &spendingChildProvider{},
		modelPolicy:        modelpolicy.NewManager(modelpolicy.ManagerConfig{}),
		researchChildCalls: &callSpacer{},
		researchDeps:       &researchtools.Dependencies{Search: stubSearchClient{}},
	}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.RunID = "run-research-spend"
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	src := `res, err := choir.Research([]map[string]string{{"objective": "two-calls"}, {"objective": "one-call"}, {"objective": "refused", "model": "no-such-model"}})
b, _ := json.Marshal(res)
fmt.Println(string(b), err)`
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`}`))
	if err != nil {
		t.Fatalf("desk_go_eval: %v", err)
	}
	var cell struct{ Stdout, Error string }
	if err := json.Unmarshal([]byte(out), &cell); err != nil || cell.Error != "" {
		t.Fatalf("cell failed: %v %s", err, out)
	}
	line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(cell.Stdout), "<nil>"))
	var results []yaegikernel.ResearchChildResult
	if err := json.Unmarshal([]byte(line), &results); err != nil || len(results) != 3 {
		t.Fatalf("decode results from %q: %v", cell.Stdout, err)
	}
	if r := results[0]; r.Findings != "done" || r.InputTokens != 300 || r.OutputTokens != 30 || r.CachedInputTokens != 190 {
		t.Fatalf("a two-call child did not report the sum of its loop: %+v", r)
	}
	if r := results[1]; r.Findings != "done" || r.InputTokens != 200 || r.OutputTokens != 20 || r.CachedInputTokens != 150 {
		t.Fatalf("a one-call child did not report its spending: %+v", r)
	}
	if r := results[2]; r.Error == "" || r.InputTokens != 0 || r.OutputTokens != 0 || r.CachedInputTokens != 0 {
		t.Fatalf("a refused child reported spending: %+v", r)
	}
	tree := researchTreeSpendLine("run-research-spend", results, 1500*time.Millisecond)
	for _, want := range []string{"root=run-research-spend", "children=3", "errors=1", "input_tokens=500", "output_tokens=50", "cached_input_tokens=340", "duration_ms=1500"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("tree line %q lacks %q", tree, want)
		}
	}
}
