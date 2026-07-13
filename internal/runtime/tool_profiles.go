package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/runtime/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/runtime/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)


const (
	runMetadataAgentProfile        = "agent_profile"
	runMetadataChannelID           = "channel_id"
	runMetadataAgentRole           = "agent_role"
	runMetadataAgentID             = "agent_id"
	runMetadataModel               = "model"
	runMetadataDesktopID           = "desktop_id"
	runMetadataToolCWD             = "tool_cwd"
	runMetadataOwnerEmail          = "owner_email"
	runMetadataWorkerIsolation     = "worker_isolation"
	runMetadataWorkerBaseSHA       = "worker_base_sha"
	runMetadataWorkerBranch        = "worker_branch"
	runMetadataWorkerWorktree      = "worker_worktree_path"
	runMetadataWorkerRepoRemote    = "worker_repo_remote_url"
	runMetadataWorkerRepoBaseSHA   = "worker_repo_base_sha"
	runMetadataWorkerRepoBootstrap = "worker_repo_bootstrap"
	runMetadataCoSuperSlot         = "co_super_slot"
	runMetadataSpawnReused         = "spawn_reused_existing_child"
	runMetadataProcessorKey        = "processor_key"
	runMetadataReconcilerScope     = "reconciler_scope"
	runMetadataExplicitResearcher  = "explicit_researcher_request"
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
		return canonicalAgentProfile(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return canonicalAgentProfile(profile)
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); isTextureAgentRevisionTaskType(taskType) {
		return agentprofile.Texture
	}
	return ""
}

func agentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Super
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		return canonicalAgentProfile(rec.AgentProfile)
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			return canonicalAgentProfile(profile)
		}
	}
	if taskType, _ := rec.Metadata["type"].(string); isTextureAgentRevisionTaskType(taskType) {
		return agentprofile.Texture
	}
	return agentprofile.Super
}

func agentRoleForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Super
	}
	if strings.TrimSpace(rec.AgentRole) != "" {
		return canonicalAgentProfile(rec.AgentRole)
	}
	if rec.Metadata != nil {
		if role, _ := rec.Metadata[runMetadataAgentRole].(string); strings.TrimSpace(role) != "" {
			return canonicalAgentProfile(role)
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

type AgentRoleSpec struct {
	Profile                   string
	AllowReadOnlyFiles        bool
	AllowWritableFiles        bool
	AllowResearchTools        bool
	AllowEvidenceTools        bool
	AllowMemoryTools          bool
	AllowModelDiagnosticTools bool
	AllowCodingTools          bool
	AllowCoAgentTools         bool
	AllowedDelegateTargets    []string
}

func roleSpec(profile string) AgentRoleSpec {
	switch canonicalAgentProfile(profile) {
	case agentprofile.Conductor:
		return AgentRoleSpec{
			Profile:                agentprofile.Conductor,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: []string{agentprofile.Texture},
		}
	case agentprofile.Researcher:
		return AgentRoleSpec{
			Profile:                   agentprofile.Researcher,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedDelegateTargets:    nil,
		}
	case agentprofile.Texture:
		// Texture is the artifact control plane, not an evidence gatherer. It does
		// not receive researcher-owned evidence tools (save/read/list_evidence) or
		// the verify_model_capability diagnostic by default. It keeps run-memory
		// retrieval so it can recover its own compacted context.
		return AgentRoleSpec{
			Profile:                agentprofile.Texture,
			AllowMemoryTools:       true,
			AllowCoAgentTools:      true,
			AllowedDelegateTargets: []string{agentprofile.Researcher},
		}
	case agentprofile.Processor:
		return AgentRoleSpec{
			Profile:                   agentprofile.Processor,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedDelegateTargets:    []string{agentprofile.Texture},
		}
	case agentprofile.Reconciler:
		return AgentRoleSpec{
			Profile:                   agentprofile.Reconciler,
			AllowReadOnlyFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCoAgentTools:         true,
			AllowedDelegateTargets:    []string{agentprofile.Texture},
		}
	case agentprofile.Email:
		return AgentRoleSpec{
			Profile: agentprofile.Email,
		}
	case agentprofile.CoSuper:
		return AgentRoleSpec{
			Profile:                   agentprofile.CoSuper,
			AllowWritableFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCodingTools:          true,
			AllowCoAgentTools:         true,
			AllowedDelegateTargets:    []string{agentprofile.Researcher},
		}
	case agentprofile.VSuper:
		return AgentRoleSpec{
			Profile:                   agentprofile.VSuper,
			AllowWritableFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCodingTools:          true,
			AllowCoAgentTools:         true,
			AllowedDelegateTargets:    []string{agentprofile.Researcher, agentprofile.CoSuper},
		}
	case agentprofile.Super:
		return AgentRoleSpec{
			Profile:                   agentprofile.Super,
			AllowWritableFiles:        true,
			AllowResearchTools:        true,
			AllowEvidenceTools:        true,
			AllowMemoryTools:          true,
			AllowModelDiagnosticTools: true,
			AllowCodingTools:          true,
			AllowCoAgentTools:         true,
			AllowedDelegateTargets:    []string{agentprofile.Researcher, agentprofile.CoSuper},
		}
	default:
		return AgentRoleSpec{Profile: strings.TrimSpace(profile)}
	}
}

func canonicalAgentProfile(profile string) string {
	profile = strings.TrimSpace(profile)
	normalized := strings.ToLower(strings.ReplaceAll(profile, "_", "-"))
	switch normalized {
	case "researcher", "researchers", "research", "research-agent", "research-worker", "web-research", "web-researcher":
		return agentprofile.Researcher
	case "cosuper", "co-super", "coagent", "co-agent":
		return agentprofile.CoSuper
	case "vsuper", "v-super", "virtual-super", "vm-super", "candidate-super":
		return agentprofile.VSuper
	case "texture", "texture-agent", "document-agent":
		return agentprofile.Texture
	case "processor", "news-processor", "source-processor", "universal-wire-processor":
		return agentprofile.Processor
	case "reconciler", "news-reconciler", "story-reconciler", "corpus-reconciler", "universal-wire-reconciler":
		return agentprofile.Reconciler
	case "email", "email-agent", "email-appagent", "mail", "mail-agent":
		return agentprofile.Email
	case "super":
		return agentprofile.Super
	case "conductor":
		return agentprofile.Conductor
	default:
		return normalized
	}
}

func isTextureProfileValue(profile string) bool {
	return canonicalAgentProfile(profile) == agentprofile.Texture
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

func canDelegateTo(callerProfile, targetProfile string) bool {
	spec := roleSpec(callerProfile)
	targetProfile = canonicalAgentProfile(targetProfile)
	for _, allowed := range spec.AllowedDelegateTargets {
		if targetProfile == canonicalAgentProfile(allowed) {
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
	repoBootstrap := workerRepoBootstrapForRun(rec)
	if profile == agentprofile.VSuper {
		b.WriteString(runtimeprompts.VSuperRuntimeOverlay(runtimeprompts.VSuperRuntimeOptions{
			RepoBootstrap: repoBootstrap,
		}))
	}
	if profile == agentprofile.CoSuper {
		b.WriteString(runtimeprompts.CoSuperRuntimeOverlay(runtimeprompts.CoSuperRuntimeOptions{
			RepoBootstrap: repoBootstrap,
		}))
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

func workerRepoBootstrapForRun(rec *types.RunRecord) string {
	if rec == nil || rec.Metadata == nil {
		return ""
	}
	return runtimeprompts.WorkerRepoBootstrap(runtimeprompts.WorkerRepoBootstrapOptions{
		RemoteURL: metadataStringValue(rec.Metadata, runMetadataWorkerRepoRemote),
		BaseSHA:   metadataStringValue(rec.Metadata, runMetadataWorkerRepoBaseSHA),
		Bootstrap: metadataStringValue(rec.Metadata, runMetadataWorkerRepoBootstrap),
	})
}

func workerRepoContextForRun(rec *types.RunRecord) string {
	return workerRepoBootstrapForRun(rec)
}

func inheritWorkerRepoMetadata(metadata map[string]any, parent *types.RunRecord) {
	if metadata == nil || parent == nil || parent.Metadata == nil {
		return
	}
	for _, key := range []string{
		runMetadataWorkerRepoRemote,
		runMetadataWorkerRepoBaseSHA,
		runMetadataWorkerRepoBootstrap,
	} {
		if strings.TrimSpace(metadataStringValue(metadata, key)) != "" {
			continue
		}
		if value := metadataStringValue(parent.Metadata, key); value != "" {
			metadata[key] = value
		}
	}
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

func (rt *Runtime) buildRegistryForRole(spec AgentRoleSpec, cwd string, searchClient webSearchClient, sourceClient sourceSearchClient, httpClient *http.Client) (*toolregistry.ToolRegistry, error) {
	registry := toolregistry.MustNewToolRegistry()
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
		if err := RegisterResearchTools(registry, searchClient, sourceClient, httpClient, rt); err != nil {
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

// InstallDefaultAgentTools installs the default profile registries used by the
// local MAS. Capabilities are enforced by role spec, not by prompt warnings.
// Super is the privileged execution root, co-super is its supervised helper,
// researcher gets read-only local files plus research/evidence tools, and
// conductor/texture get lighter coordination-oriented registries.
func (rt *Runtime) InstallDefaultAgentTools(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve tool cwd: %w", err)
		}
		cwd = wd
	}

	searchClient := newGatewaySearchClientFromEnv()
	sourceClient := newSourceSearchClientFromEnv()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	superRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Super), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterVMControlTools(superRegistry, rt, cwd); err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(superRegistry, rt); err != nil {
		return err
	}
	if err := superRegistry.Register(newProductAPIRequestTool(rt)); err != nil {
		return err
	}
	if err := RegisterShipperTools(superRegistry, rt, cwd); err != nil {
		return err
	}
	coSuperRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.CoSuper), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(coSuperRegistry, rt); err != nil {
		return err
	}
	if err := RegisterShipperTools(coSuperRegistry, rt, cwd); err != nil {
		return err
	}
	vSuperRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.VSuper), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(vSuperRegistry, rt); err != nil {
		return err
	}
	if err := RegisterShipperTools(vSuperRegistry, rt, cwd); err != nil {
		return err
	}
	researcherRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Researcher), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(researcherRegistry, rt); err != nil {
		return err
	}
	processorRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Processor), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(processorRegistry, rt); err != nil {
		return err
	}
	if err := RegisterWireProcessorTools(processorRegistry, rt); err != nil {
		return err
	}
	reconcilerRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Reconciler), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(reconcilerRegistry, rt); err != nil {
		return err
	}
	conductorRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Conductor), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	textureRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Texture), cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterTextureTools(textureRegistry, rt); err != nil {
		return err
	}
	emailRegistry, err := rt.buildRegistryForRole(roleSpec(agentprofile.Email), cwd, searchClient, sourceClient, httpClient)
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
	rt.toolProfiles[agentprofile.VSuper] = vSuperRegistry
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

func toolResultJSON(v map[string]any) (string, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
