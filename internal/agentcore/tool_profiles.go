package agentcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const (
	runMetadataAgentProfile    = "agent_profile"
	runMetadataChannelID       = "channel_id"
	runMetadataAgentRole       = "agent_role"
	runMetadataAgentID         = "agent_id"
	runMetadataModel           = "model"
	runMetadataDesktopID       = "desktop_id"
	runMetadataToolCWD         = "tool_cwd"
	runMetadataOwnerEmail      = "owner_email"
	runMetadataEngineeringSlot = "co_super_slot"
	runMetadataSpawnReused     = "spawn_reused_existing_child"
	runMetadataProcessorKey    = "processor_key"
	runMetadataReconcilerScope = "reconciler_scope"
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

func isTextureAgentID(agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	return strings.HasPrefix(agentID, agentprofile.Texture+":") || strings.HasPrefix(agentID, agentprofile.Texture+":")
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
		// Texture's prompt is its core (texture.yaml) and the generated verb
		// list; the frame carries the document and records each turn (Cut 2
		// 2a). The per-run overlays were deleted with ApplyTexture.
		if operationID := textureRunSelfDevelopmentOperation(rt, rec); operationID != "" {
			b.WriteString("\n\nSelf-development supervision:\nThis document supervises self-development operation " + operationID + ". The runtime runs the operation: engineering builds and freezes the change, an independent judgment checks it, the owner approves or rejects it, and the update applies. Do not ask to verify, approve, apply or re-run it. Keep the document a true account of where the operation stands, from the reports in choir.Updates(): what was built, what the evidence shows, and what is still pending. When a report adds nothing the document needs, call choir.Wait(why) instead of writing.")
		}
	}
	if profile == agentprofile.Management {
		b.WriteString(runtimeprompts.RLMManagementOverlay())
	}
	if profile == agentprofile.Engineering {
		hasSelfDevelopmentOperation := false
		if rt != nil && rt.selfdevOperations != nil && rec != nil && strings.TrimSpace(rec.ComputerID) != "" {
			if trajectoryID := trajectoryIDForRun(rec); trajectoryID != "" {
				_, err := rt.selfdevOperations.GetByTrajectory(context.Background(), rec.ComputerID, trajectoryID)
				hasSelfDevelopmentOperation = err == nil
			}
		}
		canProposeChange := !hasSelfDevelopmentOperation && rt != nil && rt.selfdevOperations != nil &&
			metadataStringValue(rec.Metadata, "assignment_kind") != string(types.EngineeringAssignmentVerification) &&
			rt.selfDevelopmentProposalAuthorized(context.Background()) == nil
		b.WriteString(runtimeprompts.RLMEngineeringOverlay(runtimeprompts.RLMEngineeringOverlayOptions{
			HasSelfDevelopmentOperation: hasSelfDevelopmentOperation,
			CanProposeChange:            canProposeChange,
		}))
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
		// SR: the research desk is always on the cell carrier — the legacy
		// tool-loop overlay branch is dead and removed with the typed surface.
		b.WriteString(runtimeprompts.RLMResearchOverlay())
	}
	var hidden []string
	if profile == agentprofile.Management && (rec == nil || !rt.intakeLabelsEnabled(context.Background(), rec.OwnerID, rec.ComputerID)) {
		hidden = yaegikernel.IntakeVerbs
	}
	if surface := deskSurfacePrompt(profile, rec, hidden...); surface != "" {
		b.WriteString("\n\n")
		b.WriteString(surface)
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
		InCellCarrier:          deskCarrierLive(profile) || profile == agentprofile.Engineering,
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

// delegatedEngineeringRegistryInputs is deliberately a closed set of assignment
// capabilities. Host self-development, event, updater, materialization,
// acceptance, route, VM, path-mutation, and owner-decision installers do not
// belong in this input type, so the delegated registry cannot receive their
// backing callbacks by configuration accident.
func buildAssignedEngineeringRegistry(rt *Runtime) (*toolregistry.ToolRegistry, error) {
	return buildRLMAssignedEngineeringRegistry(rt)
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
// on the host desk-cell carrier (sealed desk_go_eval registry). The predicate
// is per-profile so each desk owns its promoted carrier state.
func deskCarrierLive(profile string) bool {
	switch profile {
	case agentprofile.Management:
		return true // R3c: management is live on the cell carrier
	case agentprofile.Texture:
		return true // R3d-a: texture is live on the cell carrier
	case agentprofile.Research:
		return true // R3r: research is live on the cell carrier (D2 caps resolved)
	default:
		return false
	}
}

// buildDeskCellRegistry is the host-cell sealed overlay for a non-capsule desk
// (R3b): desk_go_eval is the sole tool on every desk registry. Management's
// lifecycle control surface is in-cell too: bound producer reports ride
// choir.ReportPacket -> persistentManagementBoundReport; assignment
// cancellation rides choir.CancelAssignment. Texture authors with the
// staged choir.Write turn committed through ApplyTextureTurn, not a
// registered tool.
func buildDeskCellRegistry(rt *Runtime, deskRole string, researchDeps researchtools.Dependencies) (*toolregistry.ToolRegistry, error) {
	registry := toolregistry.MustNewToolRegistry()
	if err := registry.Register(newDeskGoEvalTool(rt, rt.deskSessionWorkers(), deskRole)); err != nil {
		return nil, fmt.Errorf("build desk cell registry for %s: %w", deskRole, err)
	}
	if deskRole == agentprofile.Management {
		// SMG (2026-10-06, owner directive): management is a full RLM —
		// exactly one tool, desk_go_eval. report_to_texture and
		// cancel_co_super_assignment are deleted, not dual-listed: bound
		// lifecycle reports ride choir.ReportPacket (packet work_disposition)
		// through persistentManagementBoundReport; assignment cancellation
		// rides choir.CancelAssignment. The validations moved host-side onto
		// those verb paths.
	}
	if deskRole == agentprofile.Research {
		// SR (2026-10-05, owner-ratified): the research desk is a full RLM —
		// exactly one tool, desk_go_eval. The typed research surface
		// (researchtools.Register: 9 network/content tools) and the typed
		// evidence/run-memory registrations are deleted, not dual-listed:
		// every capability is a choir.* egress verb inside the cell.
	}
	return registry, nil
}

// InstallDefaultAgentTools installs role-bound registries. Management, Texture
// and research get the sealed desk-cell registry (desk_go_eval only). The
// conductor, email and engineering have empty static registries; an exact
// assigned Engineering run receives a fresh closed capsule-local registry with
// capsule_go_eval as its sole capsule-effect entry. Reporting, freeze, and
// verification are in-cell affordances.
func (rt *Runtime) InstallDefaultAgentTools() error {
	engineeringRegistry := toolregistry.MustNewToolRegistry()

	searchClient := search.NewGatewayClientFromEnv()
	// choir.Read's client refuses loopback, private, link-local, CGNAT and
	// metadata addresses and dials the address it checked
	// (research-fetch-url-has-no-address-guard-2026-10-10.md, D1/D2).
	httpClient := sourcefetch.Client(30 * time.Second)

	// InCellCarrier fan: a non-capsule desk runs on the cell carrier when
	// deskCarrierLive(profile) promotes it — management (R3c), texture
	// (R3d), and research (R3r) are live unconditionally.
	researchDeps := researchtools.Dependencies{
		Store: rt.store, Search: searchClient,
		HTTP: httpClient,
		// Autopaper captures are shared content: written to Store B through
		// corpusd, never this computer's embedded graph (W1b2).
		Captures: researchtools.NewCorpusdCaptureSink(rt.cfg.CorpusdURL),
	}
	rt.researchDeps = &researchDeps
	// Texture cites from the same captures research reads into (Cut 2 2a,
	// A2). With no corpusd configured nothing is captured, so every new
	// citation is refused as not captured.
	if strings.TrimSpace(rt.cfg.CorpusdURL) != "" {
		rt.SetTextureCaptures(researchtools.NewTextureCaptures(researchDeps.Captures, rt.cfg.ComputerID))
	}
	var deskCellRegistries = map[string]*toolregistry.ToolRegistry{}
	for _, deskProfile := range []string{agentprofile.Management, agentprofile.Texture, agentprofile.Research} {
		if !deskCarrierLive(deskProfile) {
			continue
		}
		deskReg, err := buildDeskCellRegistry(rt, deskProfile, researchDeps)
		if err != nil {
			return err
		}
		deskCellRegistries[deskProfile] = deskReg
	}

	conductorRegistry := toolregistry.MustNewToolRegistry()
	emailRegistry := toolregistry.MustNewToolRegistry()

	// D5: no fallback registry. A run whose profile has no entry below (the
	// verifier roles, frozen processor/reconciler records) gets no tools.
	rt.toolRegistry = nil
	if rt.toolProfiles == nil {
		rt.toolProfiles = make(map[string]*toolregistry.ToolRegistry)
	}
	rt.toolProfiles[agentprofile.Conductor] = conductorRegistry
	rt.toolProfiles[agentprofile.Engineering] = engineeringRegistry
	rt.toolProfiles[agentprofile.Email] = emailRegistry
	// Install the sealed desk-cell registry for every promoted desk profile.
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

// deskSurfacePrompt is the generated block naming the desk's exact choir
// surface (trace review F13); empty for profiles without a REPL desk.
// Hidden names are left out (management's intake functions while the
// computer's intake.labels setting is off).
func deskSurfacePrompt(profile string, rec *types.RunRecord, hidden ...string) string {
	switch profile {
	case agentprofile.Texture, agentprofile.Management:
		return yaegikernel.DeskSurface(profile, "", false, hidden...)
	case agentprofile.Research:
		return yaegikernel.DeskSurface(profile, "", true)
	case agentprofile.Engineering:
		slot := ""
		if rec != nil {
			slot = metadataStringValue(rec.Metadata, runMetadataEngineeringSlot)
		}
		return yaegikernel.DeskSurface(profile, slot, false)
	default:
		return ""
	}
}

// textureRunSelfDevelopmentOperation names the self-development operation a
// Texture run's trajectory supervises, or "" when there is none.
func textureRunSelfDevelopmentOperation(rt *Runtime, rec *types.RunRecord) string {
	if rt == nil || rt.selfdevOperations == nil || rec == nil || strings.TrimSpace(rec.ComputerID) == "" {
		return ""
	}
	trajectoryID := strings.TrimSpace(firstNonEmpty(rec.TrajectoryID, metadataStringValue(rec.Metadata, runMetadataTrajectoryID)))
	if trajectoryID == "" {
		return ""
	}
	operation, err := rt.selfdevOperations.GetByTrajectory(context.Background(), rec.ComputerID, trajectoryID)
	if err != nil {
		return ""
	}
	return operation.OperationID
}
