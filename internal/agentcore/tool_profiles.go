package agentcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/runtimeprompts"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/store"
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
		return deskmanifest.AdmitterName()
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
	return deskmanifest.AdmitterName()
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
		return deskmanifest.AdmitterName()
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
	return deskmanifest.DocumentInbox(docID)
}

func isTextureAgentID(agentID string) bool {
	agentID = strings.TrimSpace(agentID)
	return deskmanifest.IsDocumentInbox(agentID)
}

// systemPromptForRun is a desk's stable prefix: the core, the role prompt,
// the role's overlays and its generated surface. It must be byte-identical
// for every run and activation of the same desk/role (and owner prompt):
// providers cache a prompt by its leading bytes, so anything volatile here
// (the time, a run id, an assignment, a requester) throws away the cached
// system prompt, tools and replayed history behind it on every turn
// (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md). Per-run
// facts and the time go in runContextForRun, which the run reads as a turn.
func (rt *Runtime) systemPromptForRun(rec *types.RunRecord) (string, error) {
	if isManagementTaskRun(rec) {
		return rt.managementTaskSystemPrompt(rec), nil
	}
	profile := agentProfileForRun(rec)
	ownerID := ""
	if rec != nil {
		ownerID = rec.OwnerID
	}
	rolePrompt := fmt.Sprintf("This is the system prompt for the %s agent in Choir.", profile)
	// A desk with no stored role prompt (an app desk: its instructions come
	// with its package) takes the generic role line above.
	if d, registered := deskmanifest.Lookup(profile); rt != nil && rt.promptStore != nil && !(registered && !d.Prompt) {
		prompt, err := rt.promptStore.Load(ownerID, profile)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(prompt.Content) != "" {
			rolePrompt = prompt.Content
		}
	}

	var b strings.Builder
	b.WriteString(rt.corePrompt())
	if strings.TrimSpace(rolePrompt) != "" {
		b.WriteString("\n\nRole-specific instructions:\n")
		b.WriteString(rolePrompt)
	}
	if skillContext := rt.skillContextForProfile(profile); strings.TrimSpace(skillContext) != "" {
		b.WriteString("\n\n")
		b.WriteString(skillContext)
	}
	if profile == deskmanifest.FrontDoorName() {
		// The requested app and seed prompt are per run (runContextForRun).
		b.WriteString(runtimeprompts.ConductorRunOverlay(runtimeprompts.ConductorRunOptions{}))
	}
	// Texture's prompt is its core (texture.yaml) and the generated verb
	// list; the frame carries the document and records each turn (Cut 2
	// 2a). Its self-development supervision note is per run.
	//
	// The computer's intake.labels setting adds management's intake text
	// and functions (L2a A6); off, the prompt is as before. A computer
	// setting, so stable across runs.
	intakeLabels := deskmanifest.AdmitsAssignments(profile) && rec != nil && rt.intakeLabelsEnabled(context.Background(), rec.OwnerID, rec.ComputerID)
	eventWakes := deskmanifest.AdmitsAssignments(profile) && rec != nil && rt.computerSettingOn(context.Background(), rec.OwnerID, rec.ComputerID, store.SettingIntakeEventWakes)
	if deskmanifest.AdmitsAssignments(profile) {
		b.WriteString(runtimeprompts.RLMManagementOverlay(runtimeprompts.RLMManagementOverlayOptions{IntakeLabels: intakeLabels, EventWakes: eventWakes}))
	}
	if deskmanifest.IsAssignmentPerformer(profile) {
		// The two flags pick one of four fixed overlay variants; the exact
		// assignment is per run (runContextForRun).
		hasSelfDevelopmentOperation := false
		if rt != nil && rt.selfdevOperations != nil && rec != nil && strings.TrimSpace(rec.ComputerID) != "" {
			// The assignment's own operation (C1e D2), not the trajectory's.
			if assignmentID := metadataStringValue(rec.Metadata, "assignment_id"); assignmentID != "" {
				_, _, found, err := assignmentOperation(context.Background(), rt.selfdevOperations, rec.ComputerID, assignmentID)
				hasSelfDevelopmentOperation = err == nil && found
			}
		}
		canProposeChange := !hasSelfDevelopmentOperation && rt != nil && rt.selfdevOperations != nil &&
			metadataStringValue(rec.Metadata, "assignment_kind") != string(types.EngineeringAssignmentVerification) &&
			rt.selfDevelopmentProposalAuthorized(context.Background()) == nil
		b.WriteString(runtimeprompts.RLMEngineeringOverlay(runtimeprompts.RLMEngineeringOverlayOptions{
			HasSelfDevelopmentOperation: hasSelfDevelopmentOperation,
			CanProposeChange:            canProposeChange,
			// The run's capsule mode, fixed at assignment open.
			ClaudeDelegate: rec != nil && metadataStringValue(rec.Metadata, "capsule_network_mode") == types.EngineeringCapsuleNetworkClaude,
		}))
	}
	if deskmanifest.SpawnsResearch(profile) {
		// SR: the research desk is always on the cell carrier — the legacy
		// tool-loop overlay branch is dead and removed with the typed surface.
		b.WriteString(runtimeprompts.RLMResearchOverlay())
	}
	var hidden []string
	if deskmanifest.AdmitsAssignments(profile) && !intakeLabels {
		hidden = yaegikernel.IntakeVerbs
	}
	if surface := deskSurfacePrompt(profile, rec, hidden...); surface != "" {
		b.WriteString("\n\n")
		b.WriteString(surface)
	}
	// Texture's own prompt carries the req rule (system-prompt-review.md B3),
	// so it gets no trailer; every other desk's trailer names only the verbs
	// that desk holds.
	if profile != deskmanifest.DocumentCustodianName() {
		b.WriteString(runtimeprompts.PeerCoordination(runtimeprompts.PeerCoordinationOptions{
			InCellCarrier: deskCarrierLive(profile) || deskmanifest.IsAssignmentPerformer(profile),
			Verbs:         peerActVerbs(profile, rec, hidden...),
		}))
	}
	return b.String(), nil
}

// providerPromptForRun is the one-shot prompt of the legacy Execute path:
// the stable system prompt, the run context, then the request.
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
	b.WriteString("\n\n")
	b.WriteString(runContextText(time.Now(), rt.runContextForRun(rec)))
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
	// The desk registry says (MA2.5, desks design F3): a confined desk-worker
	// carrier that is a root desk runs on the cell carrier. A run role is not
	// a desk anyone starts work on; an unknown name is false.
	d, ok := deskmanifest.Lookup(profile)
	return ok && d.Profile && d.Kind != deskmanifest.KindRole && d.Carrier == deskmanifest.CarrierDeskWorker
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
	if deskmanifest.AdmitsAssignments(deskRole) {
		// SMG (2026-10-06, owner directive): management is a full RLM —
		// exactly one tool, desk_go_eval. report_to_texture and
		// cancel_co_super_assignment are deleted, not dual-listed: bound
		// lifecycle reports ride choir.ReportPacket (packet work_disposition)
		// through persistentManagementBoundReport; assignment cancellation
		// rides choir.CancelAssignment. The validations moved host-side onto
		// those verb paths.
	}
	if deskmanifest.SpawnsResearch(deskRole) {
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
// conductor and engineering have empty static registries (the Mail desk is a
// package desk, MA4.7, and gets a package desk's); an exact
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
		// Keyed source APIs go through the gateway, which holds the keys.
		SourceAPI: search.NewGatewaySourceAPIClientFromEnv(),
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
	// Every desk-worker desk and run role in the registry gets the sealed
	// registry (F3); a management task's desk worker binds the task's module
	// set and handle (S2a).
	for _, deskProfile := range deskmanifest.CellCarrierNames() {
		deskReg, err := buildDeskCellRegistry(rt, deskProfile, researchDeps)
		if err != nil {
			return err
		}
		deskCellRegistries[deskProfile] = deskReg
	}

	conductorRegistry := toolregistry.MustNewToolRegistry()

	// D5: no fallback registry. A run whose profile has no entry below (the
	// verifier roles, frozen processor/reconciler records) gets no tools.
	rt.toolRegistry = nil
	if rt.toolProfiles == nil {
		rt.toolProfiles = make(map[string]*toolregistry.ToolRegistry)
	}
	rt.toolProfiles[deskmanifest.FrontDoorName()] = conductorRegistry
	rt.toolProfiles[deskmanifest.PerformerName()] = engineeringRegistry
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
	rt.lateToolMu.RLock()
	defer rt.lateToolMu.RUnlock()
	if isManagementTaskRun(rec) {
		// A task's desk_go_eval binds the task role's module set, not
		// management's (R2-2). With no task registry it gets no tools.
		if rt.toolProfiles != nil {
			return rt.toolProfiles[deskmanifest.TaskRoleName()]
		}
		return nil
	}
	if rt.toolProfiles != nil {
		if registry, ok := rt.toolProfiles[profile]; ok && registry != nil {
			return registry
		}
	}
	rt.lateToolMu.RUnlock()
	registry := rt.lateDeskCellRegistry(profile)
	rt.lateToolMu.RLock() // balanced by the deferred RUnlock
	if registry != nil {
		return registry
	}
	return rt.toolRegistry
}

func (rt *Runtime) ToolRegistryForProfile(profile string) *toolregistry.ToolRegistry {
	rt.lateToolMu.RLock()
	defer rt.lateToolMu.RUnlock()
	if rt.toolProfiles == nil {
		return nil
	}
	return rt.toolProfiles[strings.TrimSpace(profile)]
}

// managementTaskSystemPrompt is a management task's prompt (S2a): the core,
// the task's own overlay (not management's, whose authority the task does
// not hold), and its generated surface. Every task shares it byte for byte;
// the task id, capture count, inbox and the time are in its run context.
func (rt *Runtime) managementTaskSystemPrompt(_ *types.RunRecord) string {
	var b strings.Builder
	b.WriteString(rt.corePrompt())
	b.WriteString(runtimeprompts.RLMManagementTaskOverlay(runtimeprompts.RLMManagementTaskOverlayOptions{
		Deadline: managementTaskDeadline.String(),
	}))
	b.WriteString("\n\n")
	b.WriteString(yaegikernel.DeskSurface(deskmanifest.TaskRoleName(), "", true))
	return b.String()
}

// deskSurfacePrompt is the generated block naming the desk's exact choir
// surface (trace review F13); empty for profiles without a REPL desk.
// Hidden names are left out (management's intake functions while the
// computer's intake.labels setting is off).
func deskSurfacePrompt(profile string, rec *types.RunRecord, hidden ...string) string {
	switch profile {
	case deskmanifest.DocumentCustodianName(), deskmanifest.AdmitterName():
		return yaegikernel.DeskSurface(profile, "", false, hidden...)
	case deskmanifest.ResearcherName():
		return yaegikernel.DeskSurface(profile, "", true)
	case deskmanifest.PerformerName():
		slot := ""
		if rec != nil {
			slot = metadataStringValue(rec.Metadata, runMetadataEngineeringSlot)
		}
		return yaegikernel.DeskSurface(profile, slot, false)
	default:
		return ""
	}
}

// LiveSelfDevelopmentOperation returns the trajectory's live
// self-development operation and its state, or "" when it has none. The
// runtime runs a live operation, so it is a next actor for the Texture
// request the trajectory serves.
func (rt *Runtime) LiveSelfDevelopmentOperation(computerID, trajectoryID string) (operationID, state string) {
	return textureRunSelfDevelopmentOperation(rt, &types.RunRecord{ComputerID: computerID, TrajectoryID: trajectoryID})
}

// textureRunSelfDevelopmentOperation names the self-development operation a
// Texture run's trajectory supervises and its state, or "" when there is none.
// Only an operation still in flight is supervised: when every operation on the
// trajectory is terminal (failed, rejected, rolled back, applied) there is
// nothing to supervise (R10-1).
func textureRunSelfDevelopmentOperation(rt *Runtime, rec *types.RunRecord) (operationID, state string) {
	if rt == nil || rt.selfdevOperations == nil || rec == nil || strings.TrimSpace(rec.ComputerID) == "" {
		return "", ""
	}
	trajectoryID := strings.TrimSpace(firstNonEmpty(rec.TrajectoryID, metadataStringValue(rec.Metadata, runMetadataTrajectoryID)))
	if trajectoryID == "" {
		return "", ""
	}
	// Texture has no assignment: it is a trajectory-wide caller (C1e D2) and
	// picks explicitly among the trajectory's operations.
	operations, err := rt.selfdevOperations.ListByTrajectory(context.Background(), rec.ComputerID, trajectoryID)
	if err != nil {
		return "", ""
	}
	operation, ok := pickLiveTrajectoryOperation(operations)
	if !ok {
		return "", ""
	}
	return operation.OperationID, operation.State
}

// lateDeskCellRegistry builds the sealed desk_go_eval registry for a root desk
// registered after InstallDefaultAgentTools ran (an app desk installed at
// runtime), and keeps it. It fails closed: a name that is not a live
// desk-worker desk, or a runtime that has not installed its tools, gets nil.
func (rt *Runtime) lateDeskCellRegistry(profile string) *toolregistry.ToolRegistry {
	if rt == nil || rt.toolProfiles == nil || rt.researchDeps == nil || !deskCarrierLive(profile) {
		return nil
	}
	rt.lateToolMu.Lock()
	defer rt.lateToolMu.Unlock()
	if registry := rt.toolProfiles[profile]; registry != nil {
		return registry
	}
	registry, err := buildDeskCellRegistry(rt, profile, *rt.researchDeps)
	if err != nil {
		return nil
	}
	rt.toolProfiles[profile] = registry
	return registry
}

// peerActVerbs names, from the desk's own surface, the addressed acts it can
// call, so the peer-coordination trailer never names a verb the desk lacks
// (the Texture trailer named choir.ReportPacket, which Texture cannot call).
func peerActVerbs(profile string, rec *types.RunRecord, hidden ...string) string {
	var slot string
	readOnly := false
	switch profile {
	case deskmanifest.ResearcherName():
		readOnly = true
	case deskmanifest.PerformerName():
		if rec != nil {
			slot = metadataStringValue(rec.Metadata, runMetadataEngineeringSlot)
		}
	}
	have := map[string]bool{}
	for _, n := range yaegikernel.SurfaceNames(profile, slot, readOnly, hidden...) {
		have[n] = true
	}
	var out []string
	for _, n := range []string{"ReportPacket", "Cast", "Ask"} {
		if have[n] {
			out = append(out, "choir."+n)
		}
	}
	return strings.Join(out, ", ")
}
