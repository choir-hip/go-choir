package agentcore

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	runMetadataAgentProfile       = "agent_profile"
	runMetadataChannelID          = "channel_id"
	runMetadataAgentRole          = "agent_role"
	runMetadataAgentID            = "agent_id"
	runMetadataModel              = "model"
	runMetadataDesktopID          = "desktop_id"
	runMetadataToolCWD            = "tool_cwd"
	runMetadataOwnerEmail         = "owner_email"
	runMetadataCoSuperSlot        = "co_super_slot"
	runMetadataSpawnReused        = "spawn_reused_existing_child"
	runMetadataProcessorKey       = "processor_key"
	runMetadataReconcilerScope    = "reconciler_scope"
	runMetadataExplicitResearcher = "explicit_researcher_request"
)

func toolExecutionContextForRun(rec *types.RunRecord) toolregistry.ExecutionContext {
	if rec == nil {
		return toolregistry.ExecutionContext{}
	}
	execution := toolregistry.ExecutionContext{
		RunID:     rec.RunID,
		AgentID:   agentIDForRun(rec),
		OwnerID:   rec.OwnerID,
		Profile:   configuredAgentProfileForRun(rec),
		Role:      agentRoleForRun(rec),
		ChannelID: channelIDForRun(rec),
		SandboxID: rec.SandboxID,
		DesktopID: desktopIDForRun(rec),
		RunRecord: rec,
	}
	if rec.Metadata != nil {
		execution.WorkingDir, _ = rec.Metadata[runMetadataToolCWD].(string)
		execution.OwnerEmail, _ = rec.Metadata[runMetadataOwnerEmail].(string)
	}
	return execution
}

func configuredAgentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return agentprofile.Canonical(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return agentprofile.Canonical(profile)
		}
	}
	return ""
}

func agentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Super
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return agentprofile.Canonical(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return agentprofile.Canonical(profile)
		}
	}
	return agentprofile.Super
}

func runHasProfile(rec *types.RunRecord, profile string) bool {
	return rec != nil && agentprofile.Canonical(agentProfileForRun(rec)) == agentprofile.Canonical(profile)
}

func agentRoleForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Super
	}
	if strings.TrimSpace(rec.AgentRole) != "" {
		return agentprofile.Canonical(rec.AgentRole)
	}
	if rec.Metadata != nil {
		if role, _ := rec.Metadata[runMetadataAgentRole].(string); strings.TrimSpace(role) != "" {
			return agentprofile.Canonical(role)
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

func currentTextureAgentID(docID string) string {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return ""
	}
	return agentprofile.Texture + ":" + docID
}

func textureAgentIDMatchesDoc(agentID, docID string) bool {
	agentID = strings.TrimSpace(agentID)
	docID = strings.TrimSpace(docID)
	if agentID == "" || docID == "" {
		return false
	}
	return agentID == currentTextureAgentID(docID)
}

func isTextureAgentID(agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	return strings.HasPrefix(agentID, agentprofile.Texture+":") || strings.HasPrefix(agentID, agentprofile.Texture+":")
}

func docIDFromTextureAgentID(agentID string) string {
	agentID = strings.TrimSpace(agentID)
	for _, prefix := range []string{agentprofile.Texture + ":", agentprofile.Texture + ":"} {
		if strings.HasPrefix(agentID, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(agentID, prefix))
		}
	}
	return ""
}

func (rt *Runtime) systemPromptForRun(rec *types.RunRecord) (string, error) {
	profile := agentProfileForRun(rec)
	channelID := channelIDForRun(rec)
	ownerID := ""
	if rec != nil {
		ownerID = rec.OwnerID
	}
	rolePrompt := fmt.Sprintf("This is the system prompt for the %s agent in Choir.", profile)
	if rt != nil && rt.promptStore != nil {
		prompt, err := rt.promptStore.Load(ownerID, profile)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(prompt.Content) != "" {
			rolePrompt = prompt.Content
		}
	}

	corePrompt := "Choir is a multiagent writing, research, and execution system with one user-facing product, one runtime, and one standard of truth."
	if rt != nil && rt.promptStore != nil {
		if loaded, err := rt.promptStore.LoadCore(); err == nil && strings.TrimSpace(loaded) != "" {
			corePrompt = loaded
		}
	}

	var b strings.Builder
	b.WriteString(corePrompt)
	b.WriteString("\n\n")
	b.WriteString(runtimeprompts.TemporalContext(runtimeprompts.TemporalContextOptions{
		NowUTC: time.Now().UTC().Format(time.RFC3339),
	}))
	if strings.TrimSpace(rolePrompt) != "" {
		b.WriteString("\n\nRole-specific instructions:\n")
		b.WriteString(rolePrompt)
	}
	if skillContext := rt.skillContextForProfile(profile); strings.TrimSpace(skillContext) != "" {
		b.WriteString("\n\n")
		b.WriteString(skillContext)
	}
	if profile == agentprofile.Conductor {
		requestedApp, _ := rec.Metadata["requested_app"].(string)
		seedPrompt, _ := rec.Metadata["seed_prompt"].(string)
		if requestedApp == "" {
			requestedApp = agentprofile.Texture
		}
		b.WriteString(runtimeprompts.ConductorRunOverlay(runtimeprompts.ConductorRunOptions{
			RequestedApp: requestedApp,
			SeedPrompt:   strings.TrimSpace(seedPrompt),
		}))
	}
	if profile == agentprofile.Texture {
		b.WriteString(textureprompts.RunOverlay())
	}
	if profile == agentprofile.Processor {
		b.WriteString(runtimeprompts.ProcessorRuntimeOverlay())
	}
	if profile == agentprofile.Reconciler {
		b.WriteString(runtimeprompts.ReconcilerRuntimeOverlay())
	}
	if profile == agentprofile.Super {
		b.WriteString(runtimeprompts.SuperRuntimeOverlay())
	}
	if profile == agentprofile.CoSuper {
		b.WriteString(runtimeprompts.CoSuperRuntimeOverlay())
	}
	if profile == agentprofile.Researcher {
		b.WriteString(runtimeprompts.ResearcherRuntimeOverlay())
	}
	requesterAgentID := ""
	textureDeliveryAgentID := ""
	if rec != nil {
		requesterAgentID = metadataStringValue(rec.Metadata, "requested_by_agent_id")
		if profile == agentprofile.Researcher && isTextureAgentID(requesterAgentID) {
			textureDeliveryAgentID = requesterAgentID
		}
	}
	b.WriteString(runtimeprompts.RunContextOverlay(runtimeprompts.RunContextOptions{
		AgentID:                agentIDForRun(rec),
		RequesterAgentID:       requesterAgentID,
		TextureDeliveryAgentID: textureDeliveryAgentID,
		ChannelID:              channelID,
	}))
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

func (rt *Runtime) buildRegistryForRole(spec agentprofile.Policy, cwd string, searchClient search.Client, sourceClient researchtools.SourceSearchClient, httpClient *http.Client) (*toolregistry.ToolRegistry, error) {
	registry := toolregistry.MustNewToolRegistry()
	if spec.AllowReadOnlyFiles {
		if err := RegisterReadOnlyFileTools(registry, cwd); err != nil {
			return nil, err
		}
	}
	if spec.AllowResearchTools {
		if err := researchtools.Register(registry, researchtools.Dependencies{
			Store: rt.store, Content: rt.content, Search: searchClient, Source: sourceClient, HTTP: httpClient,
		}); err != nil {
			return nil, err
		}
	}
	if spec.AllowEvidenceTools {
		if err := RegisterEvidenceTools(registry, rt); err != nil {
			return nil, err
		}
	}
	if spec.AllowMemoryTools {
		if err := RegisterRunMemoryTools(registry, rt); err != nil {
			return nil, err
		}
	}
	if spec.AllowModelDiagnosticTools {
		if err := RegisterModelDiagnosticTools(registry, rt); err != nil {
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

// InstallDefaultAgentTools installs role-bound registries. Super receives
// read/orchestration and capsule lifecycle tools; CoSuper receives only typed
// update plus broker-backed capsule effects. Neither role receives direct host
// mutation, shipper, route, or VM tools.
func (rt *Runtime) InstallDefaultAgentTools(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve tool cwd: %w", err)
		}
		cwd = wd
	}

	searchClient := search.NewGatewayClientFromEnv()
	sourceClient := researchtools.NewSourceClientFromEnv()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	superRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Super), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(superRegistry, rt); err != nil {
		return err
	}
	if rt.capsuleExecutor != nil {
		if err := RegisterCapsuleTools(superRegistry); err != nil {
			return err
		}
	}
	coSuperRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.CoSuper), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(coSuperRegistry, rt); err != nil {
		return err
	}
	if rt.capsuleExecutor != nil {
		if err := RegisterCapsuleExecTools(coSuperRegistry); err != nil {
			return err
		}
	}
	researcherRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Researcher), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(researcherRegistry, rt); err != nil {
		return err
	}
	processorRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Processor), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(processorRegistry, rt); err != nil {
		return err
	}
	if err := RegisterWireProcessorTools(processorRegistry, rt); err != nil {
		return err
	}
	reconcilerRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Reconciler), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(reconcilerRegistry, rt); err != nil {
		return err
	}
	conductorRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Conductor), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	textureRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Texture), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	emailRegistry, err := rt.buildRegistryForRole(agentprofile.PolicyFor(agentprofile.Email), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}

	rt.toolRegistry = superRegistry
	if rt.toolProfiles == nil {
		rt.toolProfiles = make(map[string]*toolregistry.ToolRegistry)
	}
	rt.toolProfiles[agentprofile.Conductor] = conductorRegistry
	rt.toolProfiles[agentprofile.Super] = superRegistry
	rt.toolProfiles[agentprofile.CoSuper] = coSuperRegistry
	rt.toolProfiles[agentprofile.Researcher] = researcherRegistry
	rt.toolProfiles[agentprofile.Processor] = processorRegistry
	rt.toolProfiles[agentprofile.Reconciler] = reconcilerRegistry
	rt.toolProfiles[agentprofile.Texture] = textureRegistry
	rt.toolProfiles[agentprofile.Email] = emailRegistry
	return nil
}

func (rt *Runtime) toolRegistryForRun(rec *types.RunRecord) *toolregistry.ToolRegistry {
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

func (rt *Runtime) ToolRegistryForProfile(profile string) *toolregistry.ToolRegistry {
	if rt.toolProfiles == nil {
		return nil
	}
	return rt.toolProfiles[strings.TrimSpace(profile)]
}
