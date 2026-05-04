package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	AgentProfileConductor  = "conductor"
	AgentProfileSuper      = "super"
	AgentProfileCoSuper    = "co-super"
	AgentProfileResearcher = "researcher"
	AgentProfileVText      = "vtext"
)

const (
	runMetadataAgentProfile = "agent_profile"
	runMetadataChannelID    = "channel_id"
	runMetadataAgentRole    = "agent_role"
	runMetadataAgentID      = "agent_id"
	runMetadataModel        = "model"
	runMetadataDesktopID    = "desktop_id"
)

type toolContextKey string

const (
	toolCtxRunID     toolContextKey = "loop_id"
	toolCtxAgentID   toolContextKey = "agent_id"
	toolCtxOwnerID   toolContextKey = "owner_id"
	toolCtxProfile   toolContextKey = "agent_profile"
	toolCtxRole      toolContextKey = "agent_role"
	toolCtxChannelID toolContextKey = "channel_id"
	toolCtxSandboxID toolContextKey = "sandbox_id"
	toolCtxDesktopID toolContextKey = "desktop_id"
	toolCtxRunRecord toolContextKey = "run_record"
)

func WithToolExecutionContext(ctx context.Context, rec *types.RunRecord) context.Context {
	ctx = context.WithValue(ctx, toolCtxRunID, rec.RunID)
	ctx = context.WithValue(ctx, toolCtxAgentID, agentIDForRun(rec))
	ctx = context.WithValue(ctx, toolCtxOwnerID, rec.OwnerID)
	ctx = context.WithValue(ctx, toolCtxProfile, configuredAgentProfileForRun(rec))
	ctx = context.WithValue(ctx, toolCtxRole, agentRoleForRun(rec))
	ctx = context.WithValue(ctx, toolCtxChannelID, channelIDForRun(rec))
	ctx = context.WithValue(ctx, toolCtxSandboxID, rec.SandboxID)
	ctx = context.WithValue(ctx, toolCtxDesktopID, desktopIDForRun(rec))
	ctx = context.WithValue(ctx, toolCtxRunRecord, rec)
	return ctx
}

func stringFromToolContext(ctx context.Context, key toolContextKey) string {
	value, _ := ctx.Value(key).(string)
	return strings.TrimSpace(value)
}

func configuredAgentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return strings.TrimSpace(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return strings.TrimSpace(profile)
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); taskType == "vtext_agent_revision" {
		return AgentProfileVText
	}
	return ""
}

func agentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return AgentProfileSuper
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return strings.TrimSpace(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return strings.TrimSpace(profile)
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); taskType == "vtext_agent_revision" {
		return AgentProfileVText
	}
	return AgentProfileSuper
}

func agentRoleForRun(rec *types.RunRecord) string {
	if rec == nil {
		return AgentProfileSuper
	}
	if strings.TrimSpace(rec.AgentRole) != "" {
		return strings.TrimSpace(rec.AgentRole)
	}
	if rec.Metadata != nil {
		if role, _ := rec.Metadata[runMetadataAgentRole].(string); strings.TrimSpace(role) != "" {
			return strings.TrimSpace(role)
		}
	}
	return agentProfileForRun(rec)
}

func agentIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.AgentID) != "" {
		return strings.TrimSpace(rec.AgentID)
	}
	if rec.Metadata != nil {
		if agentID, _ := rec.Metadata[runMetadataAgentID].(string); strings.TrimSpace(agentID) != "" {
			return strings.TrimSpace(agentID)
		}
	}
	return strings.TrimSpace(rec.RunID)
}

func channelIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.ChannelID) != "" {
		return strings.TrimSpace(rec.ChannelID)
	}
	if rec.Metadata != nil {
		if channelID, _ := rec.Metadata[runMetadataChannelID].(string); strings.TrimSpace(channelID) != "" {
			return strings.TrimSpace(channelID)
		}
		if legacyWorkID, _ := rec.Metadata["work_id"].(string); strings.TrimSpace(legacyWorkID) != "" {
			return strings.TrimSpace(legacyWorkID)
		}
	}
	if strings.TrimSpace(rec.AgentID) != "" {
		return strings.TrimSpace(rec.AgentID)
	}
	return strings.TrimSpace(rec.RunID)
}

func desktopIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return types.PrimaryDesktopID
	}
	if rec.Metadata != nil {
		if desktopID, _ := rec.Metadata[runMetadataDesktopID].(string); strings.TrimSpace(desktopID) != "" {
			return strings.TrimSpace(desktopID)
		}
	}
	return types.PrimaryDesktopID
}

type AgentRoleSpec struct {
	Profile                string
	AllowReadOnlyFiles     bool
	AllowWritableFiles     bool
	AllowResearchTools     bool
	AllowEvidenceTools     bool
	AllowCodingTools       bool
	AllowCoAgentTools      bool
	AllowedDelegateTargets []string
}

func roleSpec(profile string) AgentRoleSpec {
	switch canonicalAgentProfile(profile) {
	case AgentProfileConductor:
		return AgentRoleSpec{
			Profile:                AgentProfileConductor,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: []string{AgentProfileVText},
		}
	case AgentProfileResearcher:
		return AgentRoleSpec{
			Profile:                AgentProfileResearcher,
			AllowReadOnlyFiles:     true,
			AllowResearchTools:     true,
			AllowEvidenceTools:     true,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: nil,
		}
	case AgentProfileVText:
		return AgentRoleSpec{
			Profile:                AgentProfileVText,
			AllowEvidenceTools:     true,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: []string{AgentProfileResearcher},
		}
	case AgentProfileCoSuper:
		return AgentRoleSpec{
			Profile:                AgentProfileCoSuper,
			AllowWritableFiles:     true,
			AllowResearchTools:     true,
			AllowEvidenceTools:     true,
			AllowCodingTools:       true,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: []string{AgentProfileResearcher},
		}
	case AgentProfileSuper:
		return AgentRoleSpec{
			Profile:                AgentProfileSuper,
			AllowWritableFiles:     true,
			AllowResearchTools:     true,
			AllowEvidenceTools:     true,
			AllowCodingTools:       true,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: []string{AgentProfileResearcher, AgentProfileCoSuper},
		}
	default:
		return AgentRoleSpec{Profile: strings.TrimSpace(profile)}
	}
}

func canonicalAgentProfile(profile string) string {
	profile = strings.TrimSpace(profile)
	normalized := strings.ToLower(strings.ReplaceAll(profile, "_", "-"))
	switch normalized {
	case "researcher", "research", "research-agent", "web-research", "web-researcher":
		return AgentProfileResearcher
	case "cosuper", "co-super", "coagent", "co-agent":
		return AgentProfileCoSuper
	case "vtext", "vtext-agent", "document-agent":
		return AgentProfileVText
	case "super":
		return AgentProfileSuper
	case "conductor":
		return AgentProfileConductor
	default:
		return normalized
	}
}

func canDelegateTo(callerProfile, targetProfile string) bool {
	spec := roleSpec(callerProfile)
	targetProfile = canonicalAgentProfile(targetProfile)
	for _, allowed := range spec.AllowedDelegateTargets {
		if targetProfile == allowed {
			return true
		}
	}
	return false
}

func (rt *Runtime) systemPromptForRun(rec *types.RunRecord) (string, error) {
	profile := agentProfileForRun(rec)
	channelID := channelIDForRun(rec)
	ownerID := ""
	if rec != nil {
		ownerID = rec.OwnerID
	}
	rolePrompt := fmt.Sprintf("You are Choir %s.", profile)
	if rt != nil && rt.promptStore != nil {
		prompt, err := rt.promptStore.Load(ownerID, profile)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(prompt.Content) != "" {
			rolePrompt = prompt.Content
		}
	}

	corePrompt := "You are one agent inside Choir, a multiagent writing, research, and execution system."
	if rt != nil && rt.promptStore != nil {
		if loaded, err := rt.promptStore.LoadCore(); err == nil && strings.TrimSpace(loaded) != "" {
			corePrompt = loaded
		}
	}

	var b strings.Builder
	b.WriteString(corePrompt)
	if strings.TrimSpace(rolePrompt) != "" {
		b.WriteString("\n\nRole-specific instructions:\n")
		b.WriteString(rolePrompt)
	}
	if profile == AgentProfileConductor {
		requestedApp, _ := rec.Metadata["requested_app"].(string)
		seedPrompt, _ := rec.Metadata["seed_prompt"].(string)
		if requestedApp == "" {
			requestedApp = AgentProfileVText
		}
		b.WriteString("\n\nFor substantial work, route by using coagent tools. Prefer spawn_agent with role=vtext so VText becomes the durable owner of the next step.")
		b.WriteString("\nFor lightweight acknowledgements with no app handoff, return one compact JSON object like {\"action\":\"toast\",\"message\":\"...\"}.")
		b.WriteString("\nIf you already opened the next owner with a tool call, you may finish tersely; the runtime will surface the opened app from the routed result.")
		b.WriteString("\nDefault to opening vtext unless there is a strong reason to do otherwise.")
		b.WriteString("\nWhen opening vtext, spawn_agent must include initial_content containing the complete v1 document text.")
		b.WriteString("\nThat v1 should be a brief document abstract, initial hypotheses, proposed shape, or whatever first version best fits the prompt. Do not write task instructions, do not label it conductor framing, and do not present factual/current claims as researched unless workers produced evidence.")
		b.WriteString("\nAfter spawning vtext for a prompt-bar request, do not also spawn researcher, super, or co-super. VText owns downstream worker requests for the document.")
		if requestedApp != "" {
			b.WriteString("\nRequested default app: ")
			b.WriteString(requestedApp)
			b.WriteString(".")
		}
		if strings.TrimSpace(seedPrompt) != "" {
			b.WriteString("\nSeed prompt: ")
			b.WriteString(strings.TrimSpace(seedPrompt))
			b.WriteString(".")
		}
	}
	if profile == AgentProfileVText {
		b.WriteString("\n\nVText is a durable document owner, not a one-shot answerer.")
		b.WriteString("\nCanonical document versions are created only when you call edit_vtext. Your final text is run output only and is never stored as document content.")
		b.WriteString("\nWhen the document should change, call edit_vtext with the exact current base_revision_id and either a precise edit list or a complete replacement document.")
		b.WriteString("\nDo not write knowledge or coding content from model priors. Depend on researcher messages for factual/current knowledge and super messages for coding, artifacts, execution, and verification.")
		b.WriteString("\nThe conductor-created v1 is the initial abstract. If there are no worker messages yet, start the needed researcher and/or super work, then end the run without edit_vtext.")
		b.WriteString("\nLater addressed worker deliveries can be threaded into this loop or wake the next VText run and trigger another revision.")
		b.WriteString("\nBuild each revision from the current canonical version, recent worker messages, recent change context, and user-authored diffs.")
		b.WriteString("\nIntermediate appagent revisions are compactable working memory. Keep the current canonical document and user-authored changes authoritative.")
		b.WriteString("\nWhen research is needed, choose researcher parallelism from the task shape and current resource pressure.")
		b.WriteString("\nFor broad current-events briefs, prefer one broad researcher checkpoint before widening; use parallel researchers when branches are distinct and the first checkpoint shows widening is useful.")
		b.WriteString("\nLet findings checkpoints, novelty, provider health, and rate-limit signals determine whether to widen, narrow, or continue.")
		b.WriteString("\nIf the request needs live evidence, spawn a researcher on the document channel.")
		b.WriteString("\nIf it needs generated artifacts, execution, or verification, call request_super_execution. Do not spawn super directly.")
		b.WriteString("\nAs soon as one grounded findings packet is enough to improve the document, call edit_vtext for the next revision instead of waiting for perfect coverage.")
	}
	if profile == AgentProfileResearcher {
		b.WriteString("\n\nResearcher loops must converge quickly.")
		b.WriteString("\nUse web_search and fetch_url with the parallelism appropriate to the model, task, novelty, and provider health.")
		b.WriteString("\nSearch tool results and Trace expose provider endpoints, latency, errors, rate limits, and result counts; adapt your breadth from that feedback.")
		b.WriteString("\nDo not keep issuing near-duplicate searches once you already have enough grounded material to checkpoint an improvement for the document.")
		b.WriteString("\nTreat rate-limit errors as backpressure: narrow, wait, or checkpoint what you already learned rather than continuing to issue searches.")
		b.WriteString("\nAs soon as you have at least one substantive grounded finding, call submit_research_findings as a durable checkpoint.")
		b.WriteString("\nAfter submit_research_findings, either continue with the next best sequential query if it can improve the document, or end the turn if the current packet is enough.")
		b.WriteString("\nYou are a persistent communicating coagent, not a one-shot subagent. Expect to support many vtext revisions over time.")
	}
	agentID := agentIDForRun(rec)
	if agentID != "" {
		b.WriteString("\n\nCurrent agent id: ")
		b.WriteString(agentID)
		b.WriteString(".")
	}
	if rec != nil && strings.TrimSpace(rec.ParentRunID) != "" && rt != nil && rt.store != nil {
		if parentRun, err := rt.store.GetRun(context.Background(), strings.TrimSpace(rec.ParentRunID)); err == nil {
			parentAgentID := agentIDForRun(&parentRun)
			if parentAgentID != "" {
				b.WriteString("\nParent agent id: ")
				b.WriteString(parentAgentID)
				b.WriteString(".")
			}
		}
	}
	if channelID != "" {
		b.WriteString("\nCurrent coordination channel: ")
		b.WriteString(channelID)
		b.WriteString(".")
	}
	b.WriteString("\nUse addressed casts for peer coordination and keep messages concise and actionable.")
	return b.String(), nil
}

func (rt *Runtime) providerPromptForRun(rec *types.RunRecord) (string, error) {
	systemPrompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(systemPrompt) == "" {
		return rec.Prompt, nil
	}
	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\nUser request:\n")
	b.WriteString(rec.Prompt)
	return b.String(), nil
}

// WithToolProfileRegistry registers a profile-specific tool registry on the runtime.
func WithToolProfileRegistry(profile string, registry *ToolRegistry) RuntimeOption {
	return func(rt *Runtime) {
		if strings.TrimSpace(profile) == "" || registry == nil {
			return
		}
		if rt.toolProfiles == nil {
			rt.toolProfiles = make(map[string]*ToolRegistry)
		}
		rt.toolProfiles[strings.TrimSpace(profile)] = registry
	}
}

func (rt *Runtime) buildRegistryForRole(spec AgentRoleSpec, cwd string, searchClient webSearchClient, httpClient *http.Client) (*ToolRegistry, error) {
	registry := MustNewToolRegistry()
	if spec.AllowWritableFiles {
		if err := RegisterFileTools(registry, cwd); err != nil {
			return nil, err
		}
	} else if spec.AllowReadOnlyFiles {
		if err := RegisterReadOnlyFileTools(registry, cwd); err != nil {
			return nil, err
		}
	}
	if spec.AllowCodingTools {
		if err := RegisterCodingTools(registry, cwd); err != nil {
			return nil, err
		}
	}
	if spec.AllowResearchTools {
		if err := RegisterResearchTools(registry, searchClient, httpClient); err != nil {
			return nil, err
		}
	}
	if spec.AllowEvidenceTools {
		if err := RegisterEvidenceTools(registry, rt); err != nil {
			return nil, err
		}
	}
	if spec.AllowCoAgentTools {
		if err := RegisterCoAgentTools(registry, rt, spec); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// InstallDefaultAgentTools installs the default profile registries used by the
// local MAS. Capabilities are enforced by role spec, not by prompt warnings.
// Super is the privileged execution root, co-super is its supervised helper,
// researcher gets read-only local files plus research/evidence tools, and
// conductor/vtext get lighter coordination-oriented registries.
func (rt *Runtime) InstallDefaultAgentTools(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve tool cwd: %w", err)
		}
		cwd = wd
	}

	searchClient := newGatewaySearchClientFromEnv()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	superRegistry, err := rt.buildRegistryForRole(roleSpec(AgentProfileSuper), cwd, searchClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterVMControlTools(superRegistry, rt); err != nil {
		return err
	}
	if err := RegisterWorkerUpdateTools(superRegistry, rt); err != nil {
		return err
	}
	coSuperRegistry, err := rt.buildRegistryForRole(roleSpec(AgentProfileCoSuper), cwd, searchClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterWorkerUpdateTools(coSuperRegistry, rt); err != nil {
		return err
	}
	researcherRegistry, err := rt.buildRegistryForRole(roleSpec(AgentProfileResearcher), cwd, searchClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterResearcherTools(researcherRegistry, rt); err != nil {
		return err
	}
	if err := RegisterWorkerUpdateTools(researcherRegistry, rt); err != nil {
		return err
	}
	conductorRegistry, err := rt.buildRegistryForRole(roleSpec(AgentProfileConductor), cwd, searchClient, httpClient)
	if err != nil {
		return err
	}
	vtextRegistry, err := rt.buildRegistryForRole(roleSpec(AgentProfileVText), cwd, searchClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterVTextTools(vtextRegistry, rt); err != nil {
		return err
	}

	rt.toolRegistry = superRegistry
	if rt.toolProfiles == nil {
		rt.toolProfiles = make(map[string]*ToolRegistry)
	}
	rt.toolProfiles[AgentProfileConductor] = conductorRegistry
	rt.toolProfiles[AgentProfileSuper] = superRegistry
	rt.toolProfiles[AgentProfileCoSuper] = coSuperRegistry
	rt.toolProfiles[AgentProfileResearcher] = researcherRegistry
	rt.toolProfiles[AgentProfileVText] = vtextRegistry
	return nil
}

func (rt *Runtime) toolRegistryForRun(rec *types.RunRecord) *ToolRegistry {
	profile := configuredAgentProfileForRun(rec)
	if profile == "" {
		return nil
	}
	if rt.toolProfiles != nil {
		if registry, ok := rt.toolProfiles[profile]; ok && registry != nil {
			return registry
		}
	}
	return rt.toolRegistry
}

func (rt *Runtime) ToolRegistryForProfile(profile string) *ToolRegistry {
	if rt.toolProfiles == nil {
		return nil
	}
	return rt.toolProfiles[strings.TrimSpace(profile)]
}

func toolResultJSON(v map[string]any) (string, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
