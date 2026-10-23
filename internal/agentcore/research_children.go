package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
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
const researchChildPreamble = `You are a child research activation working for a parent research desk on one objective. Do this objective's work yourself with the read-only world verbs. You cannot fan out further, report, message or write: whatever you end your turn with is returned to the parent as your findings. Investigate the objective deeply, then end with your findings: each claim with its date and source URL, and what remains uncertain.`

// researchChildCallInterval spaces research-child model calls across the
// whole computer: 40 a minute, under the gateway's default per-computer
// budget of 60 (internal/gateway/ratelimit.go), leaving the rest for desks.
// A tree of any depth or width queues on it instead of bursting into 429s
// (problems/research-fanout-overruns-computer-model-budget-2026-10-11.md).
// It gates calls, not child lifetimes, so a parent waiting on its children
// never holds a slot they need.
const researchChildCallInterval = 1500 * time.Millisecond

// The time carve-out (M6.5: "a carve-out of the parent's budget"). A
// desk's choir.Research call returns within researchCallWindow, so the desk
// can report; each child gets researchChildShare of that time and is told
// it (problems/research-fanout-overruns-computer-model-budget-2026-10-11.md,
// amendment 00:52Z). Children cannot call choir.Research: one level of
// fan-out, rounds over recursion (owner 2026-10-11, PD1).
var researchCallWindow = 10 * time.Minute

const researchChildShare = 0.75

// callSpacer hands out model-call start times at least interval apart.
type callSpacer struct {
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func (s *callSpacer) wait(ctx context.Context) error {
	s.mu.Lock()
	at := time.Now()
	if s.next.After(at) {
		at = s.next
	}
	s.next = at.Add(s.interval)
	s.mu.Unlock()
	d := time.Until(at)
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// spacedProvider waits on the computer's sub-RLM call spacer before each
// model call.
type spacedProvider struct {
	provideriface.ToolLoopProvider
	spacer *callSpacer
}

func (p spacedProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	if err := p.spacer.wait(ctx); err != nil {
		return nil, err
	}
	return p.ToolLoopProvider.CallWithTools(ctx, req)
}

// subRLMCallSpacer is the one computer-wide spacer every sub-RLM's model
// calls share: research children and management tasks (management
// delegation O5, S2b FM14), 40 a minute together, so the root desks keep
// about 20 of the gateway's 60.
func (rt *Runtime) subRLMCallSpacer() *callSpacer {
	rt.researchChildCallsOnce.Do(func() {
		if rt.researchChildCalls == nil {
			rt.researchChildCalls = &callSpacer{interval: researchChildCallInterval}
		}
	})
	return rt.researchChildCalls
}

type researchLineageKey struct{}

// researchLineage is what a child inherits from the research run at the
// root of its tree: the owner's prompt identity and the model selection
// children default to.
type researchLineage struct {
	ownerID   string
	selection provideriface.LLMSelection
	fallbacks []provideriface.LLMSelection
}

func researchLineageFrom(ctx context.Context, execCtx toolregistry.ExecutionContext) researchLineage {
	if lineage, ok := ctx.Value(researchLineageKey{}).(researchLineage); ok {
		return lineage
	}
	lineage := researchLineage{ownerID: execCtx.OwnerID}
	if execCtx.RunRecord != nil {
		lineage.selection = provideriface.ResolvedLLMConfigFromMetadata(execCtx.RunRecord.Metadata)
		lineage.fallbacks = provideriface.FallbackLLMConfigsFromMetadata(execCtx.RunRecord.Metadata, lineage.selection)
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
// research-child desk on its own model loop: read-only world verbs, no
// further choir.Research (PD1), no messaging, reporting or writes, and an inert
// ledger reduction (no run record, no channel). The whole tree ends with the
// calling cell's deadline.
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
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > researchCallWindow {
		deadline = time.Now().Add(researchCallWindow)
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	start := time.Now()
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
	log.Print(researchTreeSpendLine(parent.RunID, results, time.Since(start)))
	return json.Marshal(results)
}

// researchTreeSpendLine is one research tree's spending: its children's
// token totals, summed. It is tracked, never capped (owner: track spending,
// don't cap; DEC-07, docs/design/agent-tree-budgets-2026-10-21.md).
func researchTreeSpendLine(root string, results []yaegikernel.ResearchChildResult, elapsed time.Duration) string {
	var errs, in, out, cached int
	for _, r := range results {
		if r.Error != "" {
			errs++
		}
		in += r.InputTokens
		out += r.OutputTokens
		cached += r.CachedInputTokens
	}
	return fmt.Sprintf("research tree: root=%s children=%d errors=%d input_tokens=%d output_tokens=%d cached_input_tokens=%d duration_ms=%d",
		root, len(results), errs, in, out, cached, elapsed.Milliseconds())
}

func (rt *Runtime) runResearchChild(ctx context.Context, parent toolregistry.ExecutionContext, lineage researchLineage, spec yaegikernel.ResearchChild) yaegikernel.ResearchChildResult {
	start := time.Now()
	result := yaegikernel.ResearchChildResult{Objective: spec.Objective}
	childKey := uuid.NewString()
	selection, fallbacks, err := rt.researchChildSelection(ctx, lineage, spec.Model, childKey)
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
	share := researchChildDeadline(ctx)
	ctx, cancel := context.WithDeadline(ctx, share)
	defer cancel()
	childCtx := context.WithValue(ctx, researchLineageKey{}, researchLineage{ownerID: lineage.ownerID, selection: selection, fallbacks: fallbacks})
	childCtx = toolregistry.WithExecutionContext(childCtx, childExec)

	workers := newResearchChildDeskWorkers(childKey)
	defer workers.release(deskWorkerActivationID(childExec))
	registry, err := toolregistry.NewToolRegistryWithTools(newDeskGoEvalTool(rt, workers, yaegikernel.SessionRoleResearchChild))
	if err != nil {
		result.Error = err.Error()
		return result
	}
	systemPrompt, err := rt.researchChildSystemPrompt(lineage.ownerID)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	userMsg, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []any{map[string]string{"type": "text", "text": spec.Objective}},
	})
	noEmit := func(types.EventKind, string, json.RawMessage) {}
	provider := spacedProvider{ToolLoopProvider: toolregistry.AsToolLoopProvider(rt.provider), spacer: rt.subRLMCallSpacer()}
	text, usage, loopErr := toolregistry.RunToolLoop(childCtx, provider, registry,
		[]json.RawMessage{runContextMessage(time.Now(), researchChildTime(share)), userMsg}, systemPrompt,
		provideriface.MaxInteractiveOutputTokensForSelection(selection, agentprofile.Research),
		noEmit, nil, toolregistry.WithToolLoopLLMConfig(selection),
		toolregistry.WithToolLoopFallbackLLMConfigs(fallbacks),
		// OpenCode refuses a call without a conversation id; each child is
		// its own provider conversation.
		toolregistry.WithToolLoopConversationID("research-child:"+childKey),
		// Siblings share one system prompt and tools, so they share one
		// cache thread under their parent desk.
		toolregistry.WithToolLoopPromptCache(researchChildCacheScope(parent)))
	result.Findings = strings.TrimSpace(text)
	result.InputTokens, result.OutputTokens, result.CachedInputTokens = usage.InputTokens, usage.OutputTokens, usage.CachedInputTokens
	if loopErr != nil {
		result.Error = loopErr.Error()
	}
	result.DurationMs = time.Since(start).Milliseconds()
	log.Printf("research child: root=%s model=%s duration_ms=%d input_tokens=%d output_tokens=%d findings_chars=%d error=%q",
		childExec.RunID, selection.Model, result.DurationMs, usage.InputTokens, usage.OutputTokens, len(result.Findings), result.Error)
	return result
}

// researchChildDeadline is a child's share of its parent's remaining time,
// measured when the child starts (queueing counts against the parent).
func researchChildDeadline(ctx context.Context) time.Time {
	parentDeadline, ok := ctx.Deadline()
	if !ok {
		parentDeadline = time.Now().Add(researchCallWindow)
	}
	return time.Now().Add(time.Duration(float64(time.Until(parentDeadline)) * researchChildShare))
}

// researchChildTime tells the child how long it has. It is per child and
// to the second, so it rides the child's run-context turn, never its system
// prompt (B3, docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
func researchChildTime(deadline time.Time) string {
	return fmt.Sprintf("You have about %s for this objective; findings you have not returned by then are lost.", time.Until(deadline).Round(time.Second))
}

// researchChildSystemPrompt is every child's stable system prompt: the
// child preamble and the research desk's prompt, byte-identical across
// children, parents and time.
func (rt *Runtime) researchChildSystemPrompt(ownerID string) (string, error) {
	rolePrompt, err := rt.systemPromptForRun(&types.RunRecord{OwnerID: ownerID, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research})
	if err != nil {
		return "", err
	}
	return researchChildPreamble + "\n\n" + rolePrompt, nil
}

// researchChildCacheScope is the cache thread research children share: the
// parent desk's agent on its computer.
func researchChildCacheScope(parent toolregistry.ExecutionContext) (label, scope string) {
	agentID := strings.TrimSpace(parent.AgentID)
	if agentID == "" {
		return "", ""
	}
	return "research_child", strings.TrimSpace(parent.OwnerID) + "\x00" + strings.TrimSpace(parent.ComputerID) + "\x00" + agentID + "\x00child"
}

// researchChildSelection resolves a child's model: the caller's own when
// none is named; a role name means that role's spec; "pool:<name>" is
// assigned per child like any thread; a model ID ("model" or
// "provider/model") must be in the host catalog, which is the authority for
// what can be called (docs/model-pools-design-2026-10-12.md). It returns the
// failover order after the chosen model.
func (rt *Runtime) researchChildSelection(ctx context.Context, lineage researchLineage, model, childKey string) (provideriface.LLMSelection, []provideriface.LLMSelection, error) {
	inherited := lineage.selection
	model = strings.TrimSpace(model)
	if model == "" || model == inherited.Model {
		return inherited, lineage.fallbacks, nil
	}
	var policy modelpolicy.Policy
	if rt.modelPolicy != nil {
		// An invalid policy file still yields the last valid or platform policy.
		policy, _ = rt.modelPolicy.Load(ctx, lineage.ownerID)
	}
	assign := func(selection provideriface.LLMSelection) (provideriface.LLMSelection, []provideriface.LLMSelection, error) {
		chosen, fallbacks, assignment := modelpolicy.Assign(selection, childKey)
		if assignment == modelpolicy.AssignmentUnknownPool {
			return inherited, lineage.fallbacks, fmt.Errorf("research: model policy names unknown pool %q", strings.TrimPrefix(selection.Model, modelcatalog.PoolSpecPrefix))
		}
		chosen.Source = "research_child"
		return chosen, fallbacks, nil
	}
	if _, ok := policy.Roles[model]; ok {
		return assign(policy.Resolve(model))
	}
	if strings.HasPrefix(model, modelcatalog.PoolSpecPrefix) {
		if _, ok := modelcatalog.Pools()[strings.TrimPrefix(model, modelcatalog.PoolSpecPrefix)]; ok {
			return assign(provideriface.LLMSelection{Model: model, ReasoningEffort: inherited.ReasoningEffort})
		}
	}
	providerName, modelID := "", model
	if slash := strings.Index(model, "/"); slash > 0 {
		providerName, modelID = model[:slash], model[slash+1:]
	}
	ids := make([]string, 0)
	for _, info := range modelcatalog.SupportedModels() {
		if info.ID == modelID && (providerName == "" || info.Provider == providerName) {
			return assign(provideriface.LLMSelection{Provider: info.Provider, Model: info.ID, ReasoningEffort: inherited.ReasoningEffort})
		}
		ids = append(ids, info.Provider+"/"+info.ID)
	}
	return inherited, lineage.fallbacks, fmt.Errorf("research: model %q is not in the catalog; use a role name such as %q, %q, or one of: %s", model, agentprofile.Research, modelcatalog.PoolSpecPrefix+modelcatalog.DefaultPool, strings.Join(ids, ", "))
}
