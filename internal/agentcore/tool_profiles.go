package agentcore

import (
	"context"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	runMetadataAgentProfile     = "agent_profile"
	runMetadataChannelID        = "channel_id"
	runMetadataAgentRole        = "agent_role"
	runMetadataAgentID          = "agent_id"
	runMetadataModel            = "model"
	runMetadataDesktopID        = "desktop_id"
	runMetadataToolCWD          = "tool_cwd"
	runMetadataOwnerEmail       = "owner_email"
	runMetadataEngineeringSlot  = "co_super_slot"
	runMetadataSpawnReused      = "spawn_reused_existing_child"
	runMetadataProcessorKey     = "processor_key"
	runMetadataReconcilerScope  = "reconciler_scope"
	runMetadataExplicitResearch = "explicit_researcher_request"
)

func toolExecutionContextForRun(rec *types.RunRecord) toolregistry.ExecutionContext {
	if rec == nil {
		return toolregistry.ExecutionContext{}
	}
	execution := toolregistry.ExecutionContext{
		RunID:      rec.RunID,
		AgentID:    agentIDForRun(rec),
		OwnerID:    rec.OwnerID,
		Profile:    configuredAgentProfileForRun(rec),
		Role:       agentRoleForRun(rec),
		ChannelID:  channelIDForRun(rec),
		ComputerID: rec.ComputerID,
		DesktopID:  desktopIDForRun(rec),
		RunRecord:  rec,
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
		profile, _ := agentprofile.Canonical(rec.AgentProfile)
		return profile
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			canonicalProfile, _ := agentprofile.Canonical(profile)
			return canonicalProfile
		}
	}
	return ""
}

func agentProfileForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Management
	}
	if strings.TrimSpace(rec.AgentProfile) != "" {
		profile, _ := agentprofile.Canonical(rec.AgentProfile)
		return profile
	}
	if rec.Metadata != nil {
		if profile, _ := rec.Metadata[runMetadataAgentProfile].(string); strings.TrimSpace(profile) != "" {
			canonicalProfile, _ := agentprofile.Canonical(profile)
			return canonicalProfile
		}
	}
	return agentprofile.Management
}

func runHasProfile(rec *types.RunRecord, profile string) bool {
	if rec == nil {
		return false
	}
	runProfile := agentProfileForRun(rec)
	targetProfile, _ := agentprofile.Canonical(profile)
	return runProfile == targetProfile
}

func agentRoleForRun(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Management
	}
	if strings.TrimSpace(rec.AgentRole) != "" {
		role, _ := agentprofile.Canonical(rec.AgentRole)
		return role
	}
	if rec.Metadata != nil {
		if role, _ := rec.Metadata[runMetadataAgentRole].(string); strings.TrimSpace(role) != "" {
			canonicalRole, _ := agentprofile.Canonical(role)
			return canonicalRole
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
		if strings.TrimSpace(rec.TrajectoryID) != "" && strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id")) != "" {
			b.WriteString("\n\nLifecycle Texture control authority:\nDo not call spawn_agent. Open each new Research atomically inside the successful patch_texture, rewrite_texture, or record_texture_decision transition: add one controls item with open_researcher=true, an objective, and the first typed downward packet. Continue an existing bound Research only by target_work_item_id. Agent/work/control/update/target identities and direction are runtime-derived; never author them in packet fields.")
		}
	}
	if profile == agentprofile.Processor {
		b.WriteString(runtimeprompts.ProcessorRuntimeOverlay())
	}
	if profile == agentprofile.Reconciler {
		b.WriteString(runtimeprompts.ReconcilerRuntimeOverlay())
	}
	if profile == agentprofile.Management {
		if deskCarrierLive(agentprofile.Management) {
			b.WriteString(runtimeprompts.RLMManagementOverlay())
		} else {
			b.WriteString(runtimeprompts.ManagementRuntimeOverlay())
		}
	}
	if profile == agentprofile.Engineering {
		if capsule.HostSelectsRLM() {
			hasSelfDevelopmentOperation := false
			if rt != nil && rt.selfdevOperations != nil && rec != nil && strings.TrimSpace(rec.ComputerID) != "" {
				if trajectoryID := trajectoryIDForRun(rec); trajectoryID != "" {
					_, err := rt.selfdevOperations.GetByTrajectory(context.Background(), rec.ComputerID, trajectoryID)
					hasSelfDevelopmentOperation = err == nil
				}
			}
			b.WriteString(runtimeprompts.RLMEngineeringOverlay(runtimeprompts.RLMEngineeringOverlayOptions{
				HasSelfDevelopmentOperation: hasSelfDevelopmentOperation,
			}))
		} else {
			b.WriteString(runtimeprompts.EngineeringRuntimeOverlay())
		}
		kind := metadataStringValue(rec.Metadata, "assignment_kind")
		if assignmentID := metadataStringValue(rec.Metadata, "assignment_id"); assignmentID != "" {
			b.WriteString("\n\nExact authenticated assignment: assignment_id=")
			b.WriteString(assignmentID)
			b.WriteString(" kind=")
			b.WriteString(kind)
			b.WriteString(" subject_digest=")
			b.WriteString(metadataStringValue(rec.Metadata, "subject_digest"))
			if kind == string(types.EngineeringAssignmentVerification) {
				b.WriteString(" candidate_id=")
				b.WriteString(metadataStringValue(rec.Metadata, "source_candidate_id"))
				b.WriteString(". This verification capsule contains that exact immutable candidate subject.")
			} else {
				b.WriteString(". Implement only the bounded objective in /workspace/platform and return a typed result.")
			}
		}
	}
	if profile == agentprofile.Research {
		b.WriteString(runtimeprompts.ResearchRuntimeOverlay())
	}
	requesterAgentID := ""
	textureDeliveryAgentID := ""
	if rec != nil {
		requesterAgentID = metadataStringValue(rec.Metadata, "requested_by_agent_id")
		if profile == agentprofile.Research && isTextureAgentID(requesterAgentID) {
			textureDeliveryAgentID = requesterAgentID
		}
	}
	b.WriteString(runtimeprompts.RunContextOverlay(runtimeprompts.RunContextOptions{
		AgentID:                agentIDForRun(rec),
		RequesterAgentID:       requesterAgentID,
		TextureDeliveryAgentID: textureDeliveryAgentID,
		ChannelID:              channelID,
		InCellCarrier:          deskCarrierLive(profile) || (profile == agentprofile.Engineering && capsule.HostSelectsRLM()),
		NoReportChannel:        profile == agentprofile.Engineering && !capsule.HostSelectsRLM(),
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

type registryToolInstaller func(*toolregistry.ToolRegistry) error

// delegatedEngineeringRegistryInputs is deliberately a closed set of assignment
// capabilities. Host self-development, event, updater, materialization,
// acceptance, route, VM, path-mutation, and owner-decision installers do not
// belong in this input type, so the delegated registry cannot receive their
// backing callbacks by configuration accident.
func buildAssignedEngineeringRegistry(rt *Runtime) (*toolregistry.ToolRegistry, error) {
	if capsule.HostSelectsRLM() {
		return buildRLMAssignedEngineeringRegistry(rt)
	}
	registry := toolregistry.MustNewToolRegistry()
	if err := RegisterCapsuleLocalTools(registry, rt); err != nil {
		return nil, fmt.Errorf("build assigned co-super registry: %w", err)
	}
	return registry, nil
}

// buildRLMAssignedEngineeringRegistry is the sealed-Go overlay (Def 2 item 4):
// capsule_go_eval is the sole JSON envelope — the desk's only tool. Every
// other affordance is a typed in-cell choir function staging intents for the
// one reducer: files, commands, messages, spawning, completion, freeze,
// verify, and bundle inspection.
func buildRLMAssignedEngineeringRegistry(rt *Runtime) (*toolregistry.ToolRegistry, error) {
	registry := toolregistry.MustNewToolRegistry()
	if err := registry.Register(newCapsuleGoEvalTool(rt)); err != nil {
		return nil, fmt.Errorf("build RLM assigned co-super registry: %w", err)
	}
	return registry, nil
}

// deskCarrierLive reports whether a non-capsule desk profile currently runs
// on the host desk-cell carrier (sealed desk_go_eval registry). R3c promotes
// management live — unconditionally, the first non-engineering desk on the
// in-cell carrier; texture and research remain behind actuator=rlm until
// R3d/R3r promote them. The predicate is per-profile so a desk promotion
// never drags an unpromoted desk onto cells.
func deskCarrierLive(profile string) bool {
	switch profile {
	case agentprofile.Management:
		return true // R3c: management is live on the cell carrier
	case agentprofile.Texture, agentprofile.Research:
		return capsule.HostSelectsRLM() // R3d/R3r promotion
	default:
		return false
	}
}

// buildDeskCellRegistry is the host-cell sealed overlay for a non-capsule desk
// (R3b): desk_go_eval is the cell doorway. For management (R3c) it
// additionally carries the typed producer-report and cancellation control
// tools — report_to_texture and cancel_co_super_assignment are the lifecycle
// control surface, orthogonal to the cell-eval seal; "retire to the staged
// path" retires the unstructured report flow, not these typed controls.
// Texture/research get desk_go_eval only.
func buildDeskCellRegistry(rt *Runtime, deskRole string) (*toolregistry.ToolRegistry, error) {
	registry := toolregistry.MustNewToolRegistry()
	if err := registry.Register(newDeskGoEvalTool(rt, rt.deskSessionWorkers(), deskRole)); err != nil {
		return nil, fmt.Errorf("build desk cell registry for %s: %w", deskRole, err)
	}
	if deskRole == agentprofile.Management {
		if err := RegisterPersistentManagementReportTools(registry, rt); err != nil {
			return nil, fmt.Errorf("build desk cell registry for %s: %w", deskRole, err)
		}
		if rt.capsuleExecutor != nil {
			if err := RegisterAssignedEngineeringTools(registry, rt); err != nil {
				return nil, fmt.Errorf("build desk cell registry for %s: %w", deskRole, err)
			}
		}
	}
	return registry, nil
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

// InstallDefaultAgentTools installs role-bound registries. Management receives only
// the persistent assignment/cancel authority; capsule effects are runtime-owned.
// Engineering has an empty static registry. An exact assigned run receives a fresh
// closed capsule-local registry; under actuator=rlm the desk is the in-cell
// carrier (capsule_go_eval only), under actuator=tools it is capsule effects
// only. Reporting, freeze, and verification are in-cell affordances.
func (rt *Runtime) InstallDefaultAgentTools(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve tool cwd: %w", err)
		}
		cwd = wd
	}
	engineeringRegistry := toolregistry.MustNewToolRegistry()

	searchClient := search.NewGatewayClientFromEnv()
	sourceClient := researchtools.NewSourceClientFromEnv()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	managementPolicy, err := agentprofile.PolicyFor(agentprofile.Management)
	if err != nil {
		return err
	}
	managementRegistry, err := rt.buildRegistryForRole(managementPolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterPersistentManagementReportTools(managementRegistry, rt); err != nil {
		return err
	}
	if rt.capsuleExecutor != nil {
		if err := RegisterAssignedEngineeringTools(managementRegistry, rt); err != nil {
			return err
		}
	}
	researchPolicy, err := agentprofile.PolicyFor(agentprofile.Research)
	if err != nil {
		return err
	}
	researchRegistry, err := rt.buildRegistryForRole(researchPolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}

	// InCellCarrier fan: a non-capsule desk runs on the cell carrier when
	// deskCarrierLive(profile) promotes it — management is live (R3c);
	// texture/research stay behind actuator=rlm until R3d/R3r. Unpromoted
	// desks keep their live host-tool registries.
	var deskCellRegistries = map[string]*toolregistry.ToolRegistry{}
	for _, deskProfile := range []string{agentprofile.Management, agentprofile.Texture, agentprofile.Research} {
		if !deskCarrierLive(deskProfile) {
			continue
		}
		if deskReg, derr := buildDeskCellRegistry(rt, deskProfile); derr == nil {
			deskCellRegistries[deskProfile] = deskReg
		}
	}
	processorPolicy, err := agentprofile.PolicyFor(agentprofile.Processor)
	if err != nil {
		return err
	}
	processorRegistry, err := rt.buildRegistryForRole(processorPolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	// update_coagent remains on wire roles until their migration phase.
	if err := RegisterCoagentUpdateTools(processorRegistry, rt); err != nil {
		return err
	}
	if err := RegisterWireProcessorTools(processorRegistry, rt); err != nil {
		return err
	}
	reconcilerPolicy, err := agentprofile.PolicyFor(agentprofile.Reconciler)
	if err != nil {
		return err
	}
	reconcilerRegistry, err := rt.buildRegistryForRole(reconcilerPolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	if err := RegisterCoagentUpdateTools(reconcilerRegistry, rt); err != nil {
		return err
	}
	conductorPolicy, err := agentprofile.PolicyFor(agentprofile.Conductor)
	if err != nil {
		return err
	}
	conductorRegistry, err := rt.buildRegistryForRole(conductorPolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	texturePolicy, err := agentprofile.PolicyFor(agentprofile.Texture)
	if err != nil {
		return err
	}
	textureRegistry, err := rt.buildRegistryForRole(texturePolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}
	emailPolicy, err := agentprofile.PolicyFor(agentprofile.Email)
	if err != nil {
		return err
	}
	emailRegistry, err := rt.buildRegistryForRole(emailPolicy, cwd, searchClient, sourceClient, httpClient)
	if err != nil {
		return err
	}

	rt.toolRegistry = managementRegistry
	if rt.toolProfiles == nil {
		rt.toolProfiles = make(map[string]*toolregistry.ToolRegistry)
	}
	rt.toolProfiles[agentprofile.Conductor] = conductorRegistry
	rt.toolProfiles[agentprofile.Management] = managementRegistry
	rt.toolProfiles[agentprofile.Engineering] = engineeringRegistry
	rt.toolProfiles[agentprofile.Research] = researchRegistry
	rt.toolProfiles[agentprofile.Processor] = processorRegistry
	rt.toolProfiles[agentprofile.Reconciler] = reconcilerRegistry
	rt.toolProfiles[agentprofile.Texture] = textureRegistry
	rt.toolProfiles[agentprofile.Email] = emailRegistry
	// R3b: swap in the sealed desk-cell registry for each desk profile the
	// InCellCarrier fan built — actuator=rlm puts the desk on the cell
	// carrier; actuator=tools leaves its live host-tool registry in place.
	for deskProfile, deskReg := range deskCellRegistries {
		rt.toolProfiles[deskProfile] = deskReg
	}
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
