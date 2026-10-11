package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// researchChildConcurrency bounds how many child research activations run
// at once per choir.Research call. Each child is a host worker process, so
// this protects guest memory; it never refuses a child, it queues it.
const researchChildConcurrency = 4

// researchChildPreamble is prepended to the research desk prompt for a child
// activation. It states the child's job, not a procedure.
const researchChildPreamble = `You are a child research activation working for a parent research desk on one objective. You have the read-only world verbs and choir.Research to fan out further. You cannot report, message or write: whatever you end your turn with is returned to the parent as your findings. Investigate the objective deeply, then end with your findings: each claim with its date and source URL, and what remains uncertain.`

type researchLineageKey struct{}

// researchLineage is what a child inherits from the research run at the
// root of its tree: the owner's prompt identity and the model selection
// children default to.
type researchLineage struct {
	ownerID   string
	selection provideriface.LLMSelection
}

func researchLineageFrom(ctx context.Context, execCtx toolregistry.ExecutionContext) researchLineage {
	if lineage, ok := ctx.Value(researchLineageKey{}).(researchLineage); ok {
		return lineage
	}
	lineage := researchLineage{ownerID: execCtx.OwnerID}
	if execCtx.RunRecord != nil {
		lineage.selection = provideriface.ResolvedLLMConfigFromMetadata(execCtx.RunRecord.Metadata)
		if lineage.ownerID == "" {
			lineage.ownerID = execCtx.RunRecord.OwnerID
		}
	}
	return lineage
}

// runResearchChildren serves choir.Research (RLM actor recursion; settled
// design docs/reviews/rlm-implementation-consensus-2026-08-29.md D7b/M6.5,
// results as return values per the 2026-09-15 ontology consensus; mission
// docs/definitions/choir-research-fanout-2026-10-10.md). Each child is a
// research-child desk on its own model loop: read-only world verbs and
// further choir.Research, no messaging, reporting or writes, and an inert
// ledger reduction (no run record, no channel). The whole tree charges the
// root activation's egress budget and ends with the calling cell's deadline.
func (rt *Runtime) runResearchChildren(ctx context.Context, parent toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	var req yaegikernel.ResearchPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("research: decode children: %w", err)
	}
	if len(req.Children) == 0 {
		return nil, fmt.Errorf("research: no children")
	}
	if rt == nil || rt.provider == nil {
		return nil, fmt.Errorf("research: no model provider on this computer")
	}
	lineage := researchLineageFrom(ctx, parent)
	results := make([]yaegikernel.ResearchChildResult, len(req.Children))
	sem := make(chan struct{}, researchChildConcurrency)
	var wg sync.WaitGroup
	for i, spec := range req.Children {
		wg.Add(1)
		go func(i int, spec yaegikernel.ResearchChild) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[i] = yaegikernel.ResearchChildResult{Objective: spec.Objective, Model: spec.Model, Error: ctx.Err().Error()}
				return
			}
			results[i] = rt.runResearchChild(ctx, parent, lineage, spec)
		}(i, spec)
	}
	wg.Wait()
	return json.Marshal(results)
}

func (rt *Runtime) runResearchChild(ctx context.Context, parent toolregistry.ExecutionContext, lineage researchLineage, spec yaegikernel.ResearchChild) yaegikernel.ResearchChildResult {
	start := time.Now()
	result := yaegikernel.ResearchChildResult{Objective: spec.Objective}
	selection, err := rt.researchChildSelection(ctx, lineage, spec.Model)
	if err != nil {
		result.Model = spec.Model
		result.Error = err.Error()
		return result
	}
	result.Model = selection.Model
	childExec := parent
	childExec.RunRecord = nil
	childExec.ChannelID = ""
	childExec.ToolCallID = ""
	childExec.Profile = agentprofile.Research
	childExec.Role = yaegikernel.SessionRoleResearchChild
	childExec.RunID = researchBudgetKey(parent)
	childCtx := context.WithValue(ctx, researchLineageKey{}, researchLineage{ownerID: lineage.ownerID, selection: selection})
	childCtx = toolregistry.WithExecutionContext(childCtx, childExec)

	workers := newDeskSessionWorkers()
	defer workers.release(deskWorkerActivationID(childExec))
	registry, err := toolregistry.NewToolRegistryWithTools(newDeskGoEvalTool(rt, workers, yaegikernel.SessionRoleResearchChild))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	rolePrompt, err := rt.systemPromptForRun(&types.RunRecord{OwnerID: lineage.ownerID, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research})
	if err != nil {
		result.Error = err.Error()
		return result
	}
	userMsg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []any{map[string]string{"type": "text", "text": spec.Objective}},
	})
	noEmit := func(types.EventKind, string, json.RawMessage) {}
	text, usage, loopErr := toolregistry.RunToolLoop(childCtx, toolregistry.AsToolLoopProvider(rt.provider), registry,
		[]json.RawMessage{userMsg}, researchChildPreamble+"\n\n"+rolePrompt,
		provideriface.MaxInteractiveOutputTokensForSelection(selection, agentprofile.Research),
		noEmit, nil, toolregistry.WithToolLoopLLMConfig(selection))
	result.Findings = strings.TrimSpace(text)
	if loopErr != nil {
		result.Error = loopErr.Error()
	}
	result.DurationMs = time.Since(start).Milliseconds()
	log.Printf("research child: root=%s model=%s duration_ms=%d input_tokens=%d output_tokens=%d findings_chars=%d error=%q",
		childExec.RunID, selection.Model, result.DurationMs, usage.InputTokens, usage.OutputTokens, len(result.Findings), result.Error)
	return result
}

// researchBudgetKey is the egress budget key of the root research
// activation: children charge their root's budget, not one of their own.
func researchBudgetKey(execCtx toolregistry.ExecutionContext) string {
	if execCtx.RunRecord != nil {
		if id := strings.TrimSpace(execCtx.RunRecord.RunID); id != "" {
			return id
		}
	}
	return strings.TrimSpace(execCtx.RunID)
}

// researchChildSelection resolves a child's model: the caller's own when
// none is named, otherwise a model the computer's model policy names (by
// model ID or by role), on the provider the policy gives it. The catalog is not the gate: it lists
// models Choir can route somewhere, not ones this computer is configured to
// call (problems/research-child-picks-unserved-model-2026-10-11.md).
func (rt *Runtime) researchChildSelection(ctx context.Context, lineage researchLineage, model string) (provideriface.LLMSelection, error) {
	inherited := lineage.selection
	model = strings.TrimSpace(model)
	if model == "" || model == inherited.Model {
		return inherited, nil
	}
	var policy modelpolicy.Policy
	if rt.modelPolicy != nil {
		// An invalid policy file still yields the last valid or platform policy.
		policy, _ = rt.modelPolicy.Load(ctx, lineage.ownerID)
	}
	child := func(selection provideriface.LLMSelection) provideriface.LLMSelection {
		return provideriface.LLMSelection{Provider: selection.Provider, Model: selection.Model, ReasoningEffort: inherited.ReasoningEffort, Source: "research_child"}
	}
	// A role name means that role's model: the policy's own vocabulary.
	if _, ok := policy.Roles[model]; ok {
		return child(policy.Resolve(model)), nil
	}
	configured := policy.Selections()
	ids := make([]string, 0, len(configured))
	for _, selection := range configured {
		if selection.Model == model {
			return child(selection), nil
		}
		ids = append(ids, selection.Model)
	}
	return inherited, fmt.Errorf("research: model %q is not configured on this computer; configured models: %s (a role name such as %q also works)", model, strings.Join(ids, ", "), agentprofile.Research)
}
