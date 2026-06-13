package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

// Runtime is the core runtime engine that manages run lifecycle, event
// emission, and health state. It persists all state through
// the store so that run handles and events survive sandbox process restarts
// (VAL-RUNTIME-010).
type Runtime struct {
	cfg         Config
	store       *store.Store
	bus         *events.EventBus
	provider    Provider
	promptStore *PromptStore

	mu      sync.Mutex
	health  types.RuntimeHealthState
	running map[string]context.CancelFunc // loop_id → cancel function
	// residentAgents is the volatile actor-residency index for this process.
	// Durable run rows remain evidence; warm/cold decisions use this map.
	residentAgents map[string]string // owner_id + NUL + agent_id → loop_id

	wg           sync.WaitGroup
	toolRegistry *ToolRegistry
	toolProfiles map[string]*ToolRegistry
	channelMgr   *ChannelManager

	vtextWakeMu      sync.Mutex
	vtextWakePending map[string]pendingVTextWake
	vtextWakeAfter   func(time.Duration, func()) vtextWakeTimer

	wirePublishDebounceMu sync.Mutex
	wirePublishDebouncer  *wirePublishDebouncer
	wirePublishTimer      vtextWakeTimer
	wirePlatformPublisher func(context.Context, types.Document, types.Revision, *types.RunRecord) (*wirepublish.PublishVTextResponse, error)
	vtextEditMu           sync.Mutex
	superRequestMu        sync.Mutex
	childSpawnMu          sync.Mutex
	workerRequestMu       sync.Mutex
	workerRequests        map[string]string
	conductorRouteMu      sync.Mutex
	browserOpMu           sync.Mutex
	browserOps            map[string]*sync.Mutex
	browserCDPMu          sync.Mutex
	browserCDP            map[string]*browserCDPSession
	modelPolicyMu         sync.Mutex
	modelPolicies         map[string]ModelPolicy
}

type vtextWakeTimer interface {
	Stop() bool
}

// New creates a new Runtime with the given config, store, event bus, and
// provider. The runtime is idle until Start is called.
// If a tool registry is provided, the runtime will use the tool-calling
// loop for run execution instead of the simple provider bridge path.
func New(cfg Config, s *store.Store, bus *events.EventBus, provider Provider, opts ...RuntimeOption) *Runtime {
	cfg = normalizeConfig(cfg)
	rt := &Runtime{
		cfg:              cfg,
		store:            s,
		bus:              bus,
		provider:         provider,
		health:           types.HealthReady,
		running:          make(map[string]context.CancelFunc),
		residentAgents:   make(map[string]string),
		channelMgr:       NewChannelManager(),
		promptStore:      NewPromptStore(cfg.PromptRoot),
		vtextWakePending: make(map[string]pendingVTextWake),
		vtextWakeAfter:   func(d time.Duration, fn func()) vtextWakeTimer { return time.AfterFunc(d, fn) },
		workerRequests:   make(map[string]string),
		browserOps:       make(map[string]*sync.Mutex),
		browserCDP:       make(map[string]*browserCDPSession),
		modelPolicies:    make(map[string]ModelPolicy),
	}
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(metadata))
	for k, v := range metadata {
		cloned[k] = v
	}
	return cloned
}

func metadataStringValue(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func metadataBoolValue(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	switch value := metadata[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func metadataIntValue(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func defaultAgentID(profile, ownerID string, metadata map[string]any) string {
	if agentID := metadataStringValue(metadata, runMetadataAgentID); agentID != "" {
		return agentID
	}
	switch profile {
	case AgentProfileConductor:
		if ownerID != "" {
			return "conductor:" + ownerID
		}
	case AgentProfileSuper:
		if ownerID != "" {
			return persistentSuperAgentID(ownerID)
		}
	case AgentProfileVText:
		if docID := metadataStringValue(metadata, "doc_id"); docID != "" {
			return "vtext:" + docID
		}
	case AgentProfileProcessor:
		if key := metadataStringValue(metadata, runMetadataProcessorKey); key != "" {
			return "processor:" + safeRefPart(key)
		}
	case AgentProfileReconciler:
		if scope := metadataStringValue(metadata, runMetadataReconcilerScope); scope != "" {
			return "reconciler:" + safeRefPart(scope)
		}
	}
	return uuid.New().String()
}

func persistentSuperAgentID(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return "super"
	}
	return "super:" + ownerID
}

func defaultChannelID(profile string, metadata map[string]any, parent *types.RunRecord, agentID string) string {
	if channelID := metadataStringValue(metadata, runMetadataChannelID); channelID != "" {
		return channelID
	}
	if legacy := metadataStringValue(metadata, "work_id"); legacy != "" {
		return legacy
	}
	if parent != nil && strings.TrimSpace(parent.ChannelID) != "" {
		return strings.TrimSpace(parent.ChannelID)
	}
	if profile == AgentProfileVText {
		if docID := metadataStringValue(metadata, "doc_id"); docID != "" {
			return docID
		}
	}
	if profile == AgentProfileSuper || profile == AgentProfileProcessor || profile == AgentProfileReconciler {
		return agentID
	}
	return ""
}

func (rt *Runtime) EnsurePersistentSuperAgent(ctx context.Context, ownerID string) (types.AgentRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AgentRecord{}, fmt.Errorf("runtime store unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return types.AgentRecord{}, fmt.Errorf("owner_id is required")
	}
	now := time.Now().UTC()
	agentID := persistentSuperAgentID(ownerID)
	rec := types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: rt.cfg.SandboxID,
		Profile:   AgentProfileSuper,
		Role:      AgentProfileSuper,
		ChannelID: agentID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := rt.store.UpsertAgent(ctx, rec); err != nil {
		return types.AgentRecord{}, fmt.Errorf("persist persistent super agent: %w", err)
	}
	return rec, nil
}

func resolveRunIdentity(ownerID, sandboxID string, metadata map[string]any, parent *types.RunRecord) (types.AgentRecord, map[string]any) {
	metadata = cloneMetadata(metadata)
	profile := metadataStringValue(metadata, runMetadataAgentProfile)
	if profile == "" {
		if parent != nil && strings.TrimSpace(parent.AgentProfile) != "" && metadataStringValue(metadata, "type") != "vtext_agent_revision" {
			profile = parent.AgentProfile
		} else {
			profile = agentProfileForRun(&types.RunRecord{Metadata: metadata})
		}
	}
	profile = canonicalAgentProfile(profile)
	role := metadataStringValue(metadata, runMetadataAgentRole)
	if role == "" {
		role = profile
	} else {
		role = canonicalAgentProfile(role)
	}
	agentID := defaultAgentID(profile, ownerID, metadata)
	channelID := defaultChannelID(profile, metadata, parent, agentID)
	metadata[runMetadataAgentProfile] = profile
	metadata[runMetadataAgentRole] = role
	return types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: sandboxID,
		Profile:   profile,
		Role:      role,
		ChannelID: channelID,
	}, metadata
}

func ensureDesktopID(metadata map[string]any, parent *types.RunRecord, fallback string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if existing, _ := metadata[runMetadataDesktopID].(string); strings.TrimSpace(existing) != "" {
		metadata[runMetadataDesktopID] = strings.TrimSpace(existing)
		return metadata
	}
	if parent != nil && parent.Metadata != nil {
		if inherited, _ := parent.Metadata[runMetadataDesktopID].(string); strings.TrimSpace(inherited) != "" {
			metadata[runMetadataDesktopID] = strings.TrimSpace(inherited)
			return metadata
		}
	}
	if strings.TrimSpace(fallback) == "" {
		fallback = types.PrimaryDesktopID
	}
	metadata[runMetadataDesktopID] = strings.TrimSpace(fallback)
	return metadata
}

func (rt *Runtime) PromptStore() *PromptStore {
	return rt.promptStore
}

// RuntimeOption configures optional Runtime components.
type RuntimeOption func(*Runtime)

// WithToolRegistry sets the tool registry for the runtime. When a tool
// registry is provided, the runtime uses the tool-calling loop instead
// of the simple provider bridge path for run execution.
func WithToolRegistry(registry *ToolRegistry) RuntimeOption {
	return func(rt *Runtime) {
		rt.toolRegistry = registry
	}
}

// WithChannelManager sets a custom channel manager for the runtime.
// If not called, a default empty channel manager is created.
func WithChannelManager(mgr *ChannelManager) RuntimeOption {
	return func(rt *Runtime) {
		rt.channelMgr = mgr
	}
}

func withVTextWakeAfterFuncForTest(after func(time.Duration, func()) vtextWakeTimer) RuntimeOption {
	return func(rt *Runtime) {
		if after != nil {
			rt.vtextWakeAfter = after
		}
	}
}

// Start begins runtime boot recovery. On boot, no actors are resident; previous
// in-process activations are marked passivated, then durable update backlog and
// assigned open trajectory work are swept to re-warm cold actors.
func (rt *Runtime) Start(ctx context.Context) {
	rt.passivateInterruptedActivations(ctx)
	rt.sweepPendingUpdateActors(ctx)
	rt.sweepOpenWorkItemActors(ctx)
	rt.reconcileAllVTextDocuments(ctx)
	go func() {
		<-ctx.Done()
		rt.closeAllBrowserCDPSessions()
	}()
	log.Printf("runtime: started (sandbox=%s)", rt.cfg.SandboxID)
}

// Stop gracefully shuts down the runtime, cancelling all in-flight runs.
// It is safe to call Stop multiple times.
func (rt *Runtime) Stop() {
	rt.closeAllBrowserCDPSessions()
	rt.mu.Lock()
	for runID, cancel := range rt.running {
		cancel()
		rt.removeRunningLocked(runID)
	}
	rt.mu.Unlock()

	rt.wg.Wait()
	log.Printf("runtime: stopped")
}

// StartRun creates a new execution run, persists it, emits a submitted event,
// and begins execution in a goroutine. It returns the record with the stable
// run handle and initial pending state.
func (rt *Runtime) StartRun(ctx context.Context, prompt, ownerID string) (*types.RunRecord, error) {
	return rt.StartRunWithMetadata(ctx, prompt, ownerID, nil)
}

// StartRunWithMetadata creates a new run with the given metadata, persists it,
// emits a submitted event, and begins execution in a goroutine. Metadata is
// used to carry feature-specific context (e.g., vtext agent revision info).
func shouldLogWireLifecycle(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	profile := canonicalAgentProfile(agentProfileForRun(rec))
	if profile == AgentProfileProcessor || profile == AgentProfileVText || profile == AgentProfileResearcher || profile == AgentProfileCoSuper {
		if metadataStringValue(rec.Metadata, runMetadataProcessorKey) != "" || strings.TrimSpace(rec.OwnerID) == vmctl.UniversalWirePlatformOwnerID {
			return true
		}
	}
	return false
}

func wireLifecycleSummary(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	return fmt.Sprintf("run=%s profile=%s parent=%s channel=%s processor_key=%s state=%s", rec.RunID, canonicalAgentProfile(agentProfileForRun(rec)), strings.TrimSpace(rec.ParentRunID), strings.TrimSpace(rec.ChannelID), metadataStringValue(rec.Metadata, runMetadataProcessorKey), rec.State)
}

func (rt *Runtime) StartRunWithMetadata(ctx context.Context, prompt, ownerID string, metadata map[string]any) (*types.RunRecord, error) {
	rec, err := rt.createRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, err
	}
	rt.startRunAsync(rec)
	return rec, nil
}

func (rt *Runtime) createRunWithMetadata(ctx context.Context, prompt, ownerID string, metadata map[string]any) (*types.RunRecord, error) {
	now := time.Now().UTC()
	runID := uuid.New().String()
	metadata = ensureDesktopID(metadata, nil, metadataStringValue(metadata, runMetadataDesktopID))
	agentRec, metadata := resolveRunIdentity(ownerID, rt.cfg.SandboxID, metadata, nil)
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	metadata = ensureTrajectoryID(metadata, nil, runID)
	metadata = rt.ensureResolvedLLMMetadata(ctx, ownerID, metadata)
	agentRec.CreatedAt = now
	agentRec.UpdatedAt = now
	rec := &types.RunRecord{
		RunID:        runID,
		AgentID:      agentRec.AgentID,
		ChannelID:    agentRec.ChannelID,
		AgentProfile: agentRec.Profile,
		AgentRole:    agentRec.Role,
		OwnerID:      ownerID,
		SandboxID:    rt.cfg.SandboxID,
		State:        types.RunPending,
		Prompt:       prompt,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     metadata,
	}
	rt.stampAndMintTrajectory(ctx, rec)

	if err := persistSubmittedRun(ctx, rt.store, rt.bus, agentRec, rec, len(prompt)); err != nil {
		return nil, err
	}
	if canonicalAgentProfile(agentProfileForRun(rec)) == AgentProfileProcessor {
		if _, err := rt.beginWireProcessorDecisionWorkItem(ctx, rec); err != nil {
			log.Printf("runtime: wire processor decision work item run=%s: %v", rec.RunID, err)
		}
		if err := rt.beginWireProcessorSourceDecisionWorkItems(ctx, rec); err != nil {
			log.Printf("runtime: wire processor source decision work items run=%s: %v", rec.RunID, err)
		}
	}
	if shouldLogWireLifecycle(rec) {
		log.Printf("runtime: submitted %s", wireLifecycleSummary(rec))
	}
	return rec, nil
}

func (rt *Runtime) startRunAsync(rec *types.RunRecord) {
	// Begin execution in a goroutine. Use a copy of the record to avoid
	// racing with the caller (the returned rec must retain RunPending).
	runRec := *rec

	runCtx, cancel := context.WithCancel(context.Background())
	rt.registerRunActivation(rec, cancel)

	rt.wg.Add(1)
	go rt.executeActivation(runCtx, &runRec)
}

// completePromptBarDecisionRun records a server-owned conductor decision that
// does not require model inference. This is used for deterministic product
// routes such as bare content references, where routing through a provider
// would add latency and make display-app opening depend on LLM availability.
func (rt *Runtime) completePromptBarDecisionRun(ctx context.Context, prompt, ownerID string, metadata map[string]any, decision conductorDecision) (*types.RunRecord, error) {
	now := time.Now().UTC()
	runID := uuid.New().String()
	metadata = ensureDesktopID(metadata, nil, metadataStringValue(metadata, runMetadataDesktopID))
	metadata = ensureTrajectoryID(metadata, nil, runID)
	agentRec, metadata := resolveRunIdentity(ownerID, rt.cfg.SandboxID, metadata, nil)
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	agentRec.CreatedAt = now
	agentRec.UpdatedAt = now
	if err := rt.store.UpsertAgent(ctx, agentRec); err != nil {
		return nil, fmt.Errorf("persist agent: %w", err)
	}

	decision = fillConductorDecisionFromRun(&types.RunRecord{RunID: runID, Metadata: metadata}, decision)
	result, err := json.Marshal(decision)
	if err != nil {
		return nil, fmt.Errorf("marshal conductor decision: %w", err)
	}
	rec := &types.RunRecord{
		RunID:        runID,
		AgentID:      agentRec.AgentID,
		ChannelID:    agentRec.ChannelID,
		AgentProfile: agentRec.Profile,
		AgentRole:    agentRec.Role,
		OwnerID:      ownerID,
		SandboxID:    rt.cfg.SandboxID,
		State:        types.RunCompleted,
		Prompt:       prompt,
		Result:       string(result),
		CreatedAt:    now,
		UpdatedAt:    now,
		FinishedAt:   &now,
		Metadata:     metadata,
	}
	rt.stampAndMintTrajectory(ctx, rec)
	if err := rt.store.CreateRun(ctx, *rec); err != nil {
		return nil, fmt.Errorf("persist run: %w", err)
	}

	promptLenPayload, _ := json.Marshal(map[string]int{"prompt_length": len(prompt)})
	rt.emitEvent(ctx, rec, types.EventRunSubmitted, events.CauseTaskLifecycle, promptLenPayload)
	rt.emitEvent(ctx, rec, types.EventRunStarted, events.CauseTaskLifecycle, json.RawMessage(`{"route":"server_content_reference"}`))
	completedPayload, _ := json.Marshal(map[string]any{"route": "server_content_reference", "decision": decision})
	rt.emitEvent(ctx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, completedPayload)
	return rec, nil
}

// GetRun returns a run by ID, scoped to the given owner. If the run does
// not exist or does not belong to the owner, it returns ErrNotFound
// (VAL-RUNTIME-006: caller-scoped).
func (rt *Runtime) GetRun(ctx context.Context, runID, ownerID string) (*types.RunRecord, error) {
	rec, err := rt.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if rec.OwnerID != ownerID {
		return nil, store.ErrNotFound
	}
	return &rec, nil
}

// StartChildRun creates a child run linked to a parent run. It validates that
// the parent exists, creates a runtime record, and begins execution in a
// goroutine.
//
// The child run inherits the owner from the ownerID parameter (derived from
// auth context). Constraints are stored in the run metadata for use during
// execution.
func (rt *Runtime) StartChildRun(ctx context.Context, parentID, objective, ownerID string, constraints map[string]any) (*types.RunRecord, error) {
	// Validate that the parent run exists.
	parentRec, err := rt.store.GetRun(ctx, parentID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, fmt.Errorf("parent run not found: %s", parentID)
		}
		return nil, fmt.Errorf("lookup parent run: %w", err)
	}

	runID := uuid.New().String()

	// Build metadata from constraints and parent reference.
	metadata := map[string]any{
		"spawned_by": ownerID,
		"parent_id":  parentID,
	}
	for k, v := range constraints {
		metadata[k] = v
	}
	inheritWorkerRepoMetadata(metadata, &parentRec)
	if slot := normalizeVSuperCoSuperSlot(metadataStringValue(metadata, runMetadataCoSuperSlot)); slot != "" {
		metadata[runMetadataCoSuperSlot] = slot
	}
	metadata = ensureTrajectoryID(metadata, &parentRec, runID)

	if rt.childSpawnBudgetApplies(&parentRec) {
		childProfile := canonicalAgentProfile(metadataStringValue(metadata, runMetadataAgentProfile))
		if childProfile == "" {
			childProfile = canonicalAgentProfile(metadataStringValue(metadata, runMetadataAgentRole))
		}
		slot := normalizeVSuperCoSuperSlot(metadataStringValue(metadata, runMetadataCoSuperSlot))
		if strings.TrimSpace(metadataStringValue(metadata, runMetadataCoSuperSlot)) != "" && slot == "" && childProfile == AgentProfileCoSuper {
			return nil, fmt.Errorf("vsuper co-super child requires co_super_slot to be implementation or verifier")
		}
		if childProfile == AgentProfileCoSuper && slot == "" {
			return nil, fmt.Errorf("vsuper co-super child requires co_super_slot=\"implementation\" or co_super_slot=\"verifier\"")
		}
		rt.childSpawnMu.Lock()
		defer rt.childSpawnMu.Unlock()
		if slot != "" && childProfile == AgentProfileCoSuper {
			existing, found, err := rt.activeCoSuperSlotRun(ctx, ownerID, metadataStringValue(metadata, runMetadataTrajectoryID), slot)
			if err != nil {
				return nil, err
			}
			if found {
				existing.Metadata = cloneMetadata(existing.Metadata)
				existing.Metadata[runMetadataSpawnReused] = true
				return &existing, nil
			}
		}
		if err := rt.enforceCoSuperSlotBudget(ctx, &parentRec); err != nil {
			return nil, err
		}
		if slot == "verifier" && childProfile == AgentProfileCoSuper {
			if err := rt.enforceVSuperVerifierSequencing(ctx, &parentRec); err != nil {
				return nil, err
			}
		}
	}

	now := time.Now().UTC()
	if err := rt.channelMgr.ensureParentChildChannels(parentID, runID); err != nil {
		return nil, err
	}
	metadata = ensureDesktopID(metadata, &parentRec, metadataStringValue(metadata, runMetadataDesktopID))
	agentRec, metadata := resolveRunIdentity(ownerID, rt.cfg.SandboxID, metadata, &parentRec)
	metadata = ensureTrajectoryID(metadata, &parentRec, runID)
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	claimedCoSuperSlot := false
	claimedCoSuperTrajectoryID := ""
	claimedCoSuperSlotName := ""
	if slot := normalizeVSuperCoSuperSlot(metadataStringValue(metadata, runMetadataCoSuperSlot)); slot != "" &&
		canonicalAgentProfile(metadataStringValue(metadata, runMetadataAgentProfile)) == AgentProfileCoSuper &&
		rt.childSpawnBudgetApplies(&parentRec) {
		trajectoryID := metadataStringValue(metadata, runMetadataTrajectoryID)
		existing, claimed, err := rt.store.ClaimCoSuperSlot(ctx, ownerID, trajectoryID, slot, runID, agentRec.AgentID, parentID)
		if err != nil {
			return nil, err
		}
		if !claimed {
			existing.Metadata = cloneMetadata(existing.Metadata)
			existing.Metadata[runMetadataSpawnReused] = true
			return &existing, nil
		}
		claimedCoSuperSlot = true
		claimedCoSuperTrajectoryID = trajectoryID
		claimedCoSuperSlotName = slot
	}
	releaseCoSuperSlotClaim := func(cause error) error {
		if !claimedCoSuperSlot {
			return cause
		}
		if err := rt.store.ReleaseCoSuperSlotClaim(context.Background(), ownerID, claimedCoSuperTrajectoryID, claimedCoSuperSlotName, runID); err != nil {
			return fmt.Errorf("%w (also failed to release co-super slot claim: %v)", cause, err)
		}
		return cause
	}
	metadata = rt.ensureResolvedLLMMetadata(ctx, ownerID, metadata)
	agentRec.CreatedAt = now
	agentRec.UpdatedAt = now
	if err := rt.store.UpsertAgent(ctx, agentRec); err != nil {
		return nil, releaseCoSuperSlotClaim(fmt.Errorf("persist child agent: %w", err))
	}

	// Create the runtime run record.
	rec := &types.RunRecord{
		RunID:        runID,
		AgentID:      agentRec.AgentID,
		ChannelID:    agentRec.ChannelID,
		ParentRunID:  parentID,
		AgentProfile: agentRec.Profile,
		AgentRole:    agentRec.Role,
		OwnerID:      ownerID,
		SandboxID:    rt.cfg.SandboxID,
		State:        types.RunPending,
		Prompt:       objective,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata:     metadata,
	}
	rt.stampAndMintTrajectory(ctx, rec)
	if item, err := rt.createSpawnedChildWorkItem(ctx, rec, &parentRec); err != nil {
		return nil, releaseCoSuperSlotClaim(fmt.Errorf("persist spawned child work item: %w", err))
	} else if item.WorkItemID != "" {
		rec.Metadata = cloneMetadata(rec.Metadata)
		rec.Metadata["work_item_ids"] = appendUniqueString(metadataStringSlice(rec.Metadata["work_item_ids"]), item.WorkItemID)
	}

	if err := rt.store.CreateRun(ctx, *rec); err != nil {
		return nil, releaseCoSuperSlotClaim(fmt.Errorf("persist child run: %w", err))
	}
	rt.createAgentMutationForRun(ctx, rec)

	// Emit submitted event.
	objectiveLenPayload, _ := json.Marshal(map[string]any{
		"prompt_length": len(objective),
		"parent_id":     parentID,
	})
	rt.emitEvent(ctx, rec, types.EventRunSubmitted, events.CauseTaskLifecycle, objectiveLenPayload)
	if shouldLogWireLifecycle(rec) || shouldLogWireLifecycle(&parentRec) {
		log.Printf("runtime: started child %s from parent=%s parent_profile=%s", wireLifecycleSummary(rec), parentRec.RunID, canonicalAgentProfile(agentProfileForRun(&parentRec)))
	}

	// Begin execution in a goroutine. Use a copy of the record to avoid
	// racing with the caller (the returned rec must retain RunPending).
	runRec := *rec

	runCtx, cancel := context.WithCancel(context.Background())
	rt.registerRunActivation(rec, cancel)

	rt.wg.Add(1)
	go rt.executeActivation(runCtx, &runRec)

	log.Printf("runtime: started child run %s for parent %s (owner=%s)", rec.RunID, parentID, ownerID)

	if _, err := rt.channelMgr.Channel(parentRec.ChannelID); err != nil {
		log.Printf("runtime: ensure parent channel %s: %v", parentRec.ChannelID, err)
	}
	if rec.ChannelID != "" && rec.ChannelID != parentRec.ChannelID {
		if _, err := rt.channelMgr.Channel(rec.ChannelID); err != nil {
			log.Printf("runtime: ensure child channel %s: %v", rec.ChannelID, err)
		}
	}

	return rec, nil
}

func (rt *Runtime) createSpawnedChildWorkItem(ctx context.Context, rec *types.RunRecord, parent *types.RunRecord) (types.WorkItemRecord, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return types.WorkItemRecord{}, nil
	}
	profile := canonicalAgentProfile(agentProfileForRun(rec))
	if !spawnedChildWorkItemProfile(profile) {
		return types.WorkItemRecord{}, nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	agentID := strings.TrimSpace(rec.AgentID)
	objective := strings.TrimSpace(rec.Prompt)
	if ownerID == "" || trajectoryID == "" || agentID == "" || objective == "" {
		return types.WorkItemRecord{}, nil
	}
	parentID := strings.TrimSpace(rec.ParentRunID)
	if parentID == "" {
		parentID = metadataStringValue(rec.Metadata, "parent_id")
	}
	if parentID == "" {
		return types.WorkItemRecord{}, nil
	}
	details := map[string]any{
		"kind":           "spawned_child_run",
		"spawned_run_id": rec.RunID,
		"parent_run_id":  parentID,
		"agent_profile":  profile,
		"agent_role":     agentRoleForRun(rec),
	}
	if channelID := strings.TrimSpace(rec.ChannelID); channelID != "" {
		details["channel_id"] = channelID
	}
	if parent != nil {
		if parentProfile := canonicalAgentProfile(agentProfileForRun(parent)); parentProfile != "" {
			details["parent_agent_profile"] = parentProfile
		}
	}
	return rt.store.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              ownerID,
		TrajectoryID:         trajectoryID,
		Objective:            objective,
		Reason:               "spawn_agent child objective",
		AuthorityProfile:     profile,
		AssignedAgentID:      agentID,
		CreatedByRunID:       parentID,
		ObjectiveFingerprint: "spawned_child:" + objectiveFingerprint(ownerID, trajectoryID, rec.RunID, objective),
		Details:              details,
	})
}

func spawnedChildWorkItemProfile(profile string) bool {
	switch canonicalAgentProfile(profile) {
	case AgentProfileResearcher, AgentProfileSuper, AgentProfileVSuper, AgentProfileCoSuper:
		return true
	default:
		return false
	}
}

func appendUniqueString(existing []string, values ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(existing)+len(values))
	for _, value := range existing {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

const maxVSuperActiveCoSuperSlots = 2

func (rt *Runtime) childSpawnBudgetApplies(parentRec *types.RunRecord) bool {
	if parentRec == nil {
		return false
	}
	return canonicalAgentProfile(agentProfileForRun(parentRec)) == AgentProfileVSuper
}

func (rt *Runtime) enforceCoSuperSlotBudget(ctx context.Context, parentRec *types.RunRecord) error {
	if rt == nil || rt.store == nil || parentRec == nil {
		return nil
	}
	trajectoryID := trajectoryIDForRun(parentRec)
	active, err := rt.store.CountActiveCoSuperSlots(ctx, parentRec.OwnerID, trajectoryID)
	if err != nil {
		return fmt.Errorf("check active co-super slots for vsuper trajectory budget: %w", err)
	}
	if active >= maxVSuperActiveCoSuperSlots {
		return fmt.Errorf("vsuper active co-super slot limit reached for trajectory %s (%d/%d); coordinate existing worker/verifier agents over channels, cancel or wait for a co-super slot, or submit a precise blocker instead of spawning more", trajectoryID, active, maxVSuperActiveCoSuperSlots)
	}
	return nil
}

func (rt *Runtime) activeCoSuperSlotRun(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, nil
	}
	slot = normalizeVSuperCoSuperSlot(slot)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if slot == "" || trajectoryID == "" {
		return types.RunRecord{}, false, nil
	}
	return rt.store.ActiveCoSuperSlotRun(ctx, ownerID, trajectoryID, slot)
}

func (rt *Runtime) enforceVSuperVerifierSequencing(ctx context.Context, parentRec *types.RunRecord) error {
	if rt == nil || rt.store == nil || parentRec == nil {
		return nil
	}
	trajectoryID := trajectoryIDForRun(parentRec)
	impl, found, err := rt.store.CoSuperSlotRun(ctx, parentRec.OwnerID, trajectoryID, "implementation")
	if err != nil {
		return fmt.Errorf("lookup implementation co-super slot for verifier sequencing: %w", err)
	}
	if found && impl.State.Active() {
		return fmt.Errorf("vsuper verifier spawn blocked until implementation co-super %s reports commit/package/blocker evidence and finishes; wait for update_coagent evidence before spawning slot=\"verifier\"", impl.RunID)
	}
	if found && impl.State.Terminal() {
		return nil
	}
	return fmt.Errorf("vsuper verifier spawn requires prior implementation co-super evidence; spawn slot=\"implementation\" first, wait for commit/package/blocker evidence, then spawn slot=\"verifier\" with the exact evidence to inspect")
}

func (rt *Runtime) latestTrajectoryCoSuperAppChangePackage(ctx context.Context, parentRec *types.RunRecord) (map[string]any, bool, error) {
	if rt == nil || rt.store == nil {
		return nil, false, nil
	}
	if parentRec == nil {
		return nil, false, nil
	}
	trajectoryID := trajectoryIDForRun(parentRec)
	if trajectoryID == "" || strings.TrimSpace(parentRec.OwnerID) == "" {
		return nil, false, nil
	}
	child, found, err := rt.store.CoSuperSlotRun(ctx, parentRec.OwnerID, trajectoryID, "implementation")
	if err != nil {
		return nil, false, fmt.Errorf("lookup implementation co-super slot for app package reuse: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	childEvents, err := rt.store.ListEvents(ctx, child.RunID, 1000)
	if err != nil {
		return nil, false, fmt.Errorf("list implementation co-super events for export reuse: %w", err)
	}
	_, output, ok := latestSuccessfulToolResultOutput(childEvents, "publish_app_change_package")
	if !ok {
		return nil, false, nil
	}
	output["loop_id"] = child.RunID
	output["child_loop_id"] = child.RunID
	output["child_agent_id"] = child.AgentID
	if slot := metadataStringValue(child.Metadata, runMetadataCoSuperSlot); slot != "" {
		output["child_slot"] = slot
	}
	return output, true, nil
}

func (rt *Runtime) createAgentMutationForRun(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rt.store == nil || rec == nil {
		return
	}
	if metadataStringValue(rec.Metadata, "type") != "vtext_agent_revision" {
		return
	}
	mutation := agentMutationForRun(rec)
	if mutation == nil {
		log.Printf("runtime: vtext agent revision run %s: missing doc_id for mutation", rec.RunID)
		return
	}
	if err := rt.store.CreateAgentMutation(ctx, *mutation); err != nil {
		log.Printf("runtime: vtext agent revision run %s: create mutation: %v", rec.RunID, err)
	}
}

// CancelRun cancels a running or pending run. It validates that the run
// exists and belongs to the given owner, then cancels the run's context
// and transitions it to cancelled state (VAL-CHOIR-010).
//
// Returns an error if:
//   - the run does not exist
//   - the run belongs to a different owner
//   - the run is already in a terminal state
func (rt *Runtime) CancelRun(ctx context.Context, runID, ownerID string) error {
	rec, err := rt.store.GetRun(ctx, runID)
	if err != nil {
		if err == store.ErrNotFound {
			return fmt.Errorf("run not found: %s", runID)
		}
		return fmt.Errorf("lookup run: %w", err)
	}

	// Ownership check.
	if rec.OwnerID != ownerID {
		return store.ErrNotFound
	}

	// Only running or pending runs can be cancelled.
	if rec.State.Terminal() {
		return fmt.Errorf("cannot cancel run in %s state", rec.State)
	}

	// Cancel the run's execution context.
	rt.mu.Lock()
	cancel, ok := rt.running[runID]
	if ok {
		cancel()
		rt.removeRunningLocked(runID)
	}
	rt.mu.Unlock()

	if !ok {
		// Run was not running in this process (e.g., pending or recovered).
		// Transition it directly to cancelled.
		now := time.Now().UTC()
		rec.State = types.RunCancelled
		rec.UpdatedAt = now
		rec.FinishedAt = &now
		if err := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(ctx, &rec); err != nil {
			return fmt.Errorf("update cancelled run: %w", err)
		}

		errPayload, _ := json.Marshal(map[string]string{"error": "run cancelled"})
		rt.emitEvent(ctx, &rec, types.EventRunCancelled, events.CauseTaskLifecycle, errPayload)

	}

	return nil
}

// CancelAgent cancels the most recent non-terminal run owned by the given agent.
func (rt *Runtime) CancelAgent(ctx context.Context, agentID, ownerID string) error {
	if resident, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
		return fmt.Errorf("lookup resident agent run: %w", err)
	} else if found {
		return rt.CancelRun(ctx, resident.RunID, ownerID)
	}
	rec, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		if err == store.ErrNotFound {
			return fmt.Errorf("agent not found: %s", agentID)
		}
		return fmt.Errorf("lookup active agent run: %w", err)
	}
	return rt.CancelRun(ctx, rec.RunID, ownerID)
}

// CancelRunTrajectory cancels the trajectory that contains runID. The
// trajectory/work-item record is the authority; run cancellation is activation
// termination evidence, not a parent_loop_id graph walk.
func (rt *Runtime) CancelRunTrajectory(ctx context.Context, runID, ownerID string) ([]string, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("cancel trajectory: runtime store is unavailable")
	}
	runID = strings.TrimSpace(runID)
	ownerID = strings.TrimSpace(ownerID)
	if runID == "" || ownerID == "" {
		return nil, fmt.Errorf("cancel trajectory: run_id and owner_id are required")
	}
	rec, err := rt.store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	if rec.OwnerID != ownerID {
		return nil, store.ErrNotFound
	}
	trajectoryID := trajectoryIDForRun(&rec)
	if trajectoryID == "" {
		trajectoryID = rec.RunID
	}
	rec.Metadata = ensureTrajectoryID(rec.Metadata, nil, trajectoryID)
	rec.TrajectoryID = trajectoryID
	rt.stampAndMintTrajectory(ctx, &rec)
	if err := rt.store.UpdateRun(ctx, rec); err != nil {
		return nil, fmt.Errorf("persist trajectory identity on run %s: %w", rec.RunID, err)
	}

	openItems, err := rt.store.ListWorkItemsByTrajectory(ctx, ownerID, trajectoryID, true)
	if err != nil {
		return nil, fmt.Errorf("list open trajectory work items: %w", err)
	}
	for _, item := range openItems {
		if _, err := rt.store.UpdateWorkItemStatus(ctx, ownerID, item.WorkItemID, types.WorkItemCancelled); err != nil {
			return nil, fmt.Errorf("cancel trajectory work item %s: %w", item.WorkItemID, err)
		}
	}
	if _, err := rt.store.UpdateTrajectoryStatus(ctx, ownerID, trajectoryID, types.TrajectoryCancelled); err != nil {
		return nil, fmt.Errorf("cancel trajectory status: %w", err)
	}

	cancelled := []string{}
	excluded := []string{}
	for {
		active, err := rt.store.ListActiveRunsByTrajectoryExcluding(ctx, ownerID, trajectoryID, excluded, 200)
		if err != nil {
			return cancelled, fmt.Errorf("list active trajectory activations: %w", err)
		}
		if len(active) == 0 {
			break
		}
		for _, run := range active {
			excluded = append(excluded, run.RunID)
			if err := rt.CancelRun(ctx, run.RunID, ownerID); err != nil {
				if strings.Contains(err.Error(), "cannot cancel run in") {
					continue
				}
				return cancelled, err
			}
			cancelled = append(cancelled, run.RunID)
		}
	}
	return cancelled, nil
}

// ListRunsByOwner returns recent runs for the given owner, ordered by
// creation time descending.
func (rt *Runtime) ListRunsByOwner(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	return rt.store.ListRunsByOwner(ctx, ownerID, limit)
}

// HealthState returns the current runtime health state.
func (rt *Runtime) HealthState() types.RuntimeHealthState {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.health
}

// SetHealth updates the runtime health state. If the state changes, it emits
// a health or degraded event to make the transition externally visible
// (VAL-RUNTIME-001, VAL-RUNTIME-009).
func (rt *Runtime) SetHealth(state types.RuntimeHealthState) {
	rt.mu.Lock()
	prev := rt.health
	rt.health = state
	rt.mu.Unlock()

	if prev == state {
		return
	}

	log.Printf("runtime: health %s → %s", prev, state)

	ctx := context.Background()
	kind := types.EventRuntimeHealth
	cause := events.CauseTaskLifecycle
	if state == types.HealthDegraded || state == types.HealthFailed {
		kind = types.EventRuntimeDegraded
		cause = events.CauseProviderFailure
	}

	payload, _ := json.Marshal(map[string]string{
		"previous": string(prev),
		"current":  string(state),
	})

	evRec := &types.EventRecord{
		EventID:   uuid.New().String(),
		Timestamp: time.Now().UTC(),
		Kind:      kind,
		Payload:   payload,
	}
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist health event %s: %v", evRec.EventID, err)
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorSupervisor,
		Cause:  cause,
	})
}

// EventBus returns the runtime event bus for SSE subscription.
func (rt *Runtime) EventBus() *events.EventBus {
	return rt.bus
}

// Store returns the runtime store for direct queries.
func (rt *Runtime) Store() *store.Store {
	return rt.store
}

// RunningCount returns the number of currently executing runs.
func (rt *Runtime) RunningCount() int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return len(rt.running)
}

// RunningCountByProfile returns the number of running runs with the given
// agent profile that still occupy admission capacity. Note: for processors
// this issues one FindWorkItemByFingerprint per running run (an N+1 against
// the work-item table; acceptable at current run volumes), and any lookup
// error silently defaults to "occupies admission" — the conservative side.
func (rt *Runtime) RunningCountByProfile(ctx context.Context, profile string) int {
	runs, err := rt.store.ListRunsByState(ctx, types.RunRunning, 1000)
	if err != nil {
		log.Printf("runtime: count running %s runs: %v", profile, err)
		return rt.RunningCount()
	}
	profile = canonicalAgentProfile(profile)
	count := 0
	for i := range runs {
		if canonicalAgentProfile(runs[i].AgentProfile) != profile {
			continue
		}
		if profile == AgentProfileProcessor && !rt.processorRunOccupiesAdmission(ctx, runs[i]) {
			continue
		}
		count++
	}
	return count
}

func (rt *Runtime) processorRunOccupiesAdmission(ctx context.Context, rec types.RunRecord) bool {
	if rt == nil || rt.store == nil {
		return true
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(&rec))
	if ownerID == "" || trajectoryID == "" {
		return true
	}
	item, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, wireProcessorDecisionWorkItemFingerprint(trajectoryID))
	if err != nil || !found {
		return true
	}
	if item.Status != types.WorkItemCompleted {
		return true
	}
	if strings.EqualFold(metadataStringValue(item.Details, wireDetailKeyResolutionState), sourceapi.ResolutionStateDecidedWithStoryRoute) {
		return false
	}
	return true
}

// ToolRegistry returns the runtime's tool registry, or nil if none is configured.
func (rt *Runtime) ToolRegistry() *ToolRegistry {
	return rt.toolRegistry
}

// ChannelManager returns the runtime's channel manager.
func (rt *Runtime) ChannelManager() *ChannelManager {
	return rt.channelMgr
}

// passivateInterruptedActivations releases runs that were active in a previous
// process without converting the durable agent's work into a failure. A later
// update_coagent send or trajectory sweep may re-warm the actor.
func (rt *Runtime) passivateInterruptedActivations(ctx context.Context) {
	states := []types.RunState{types.RunPending, types.RunRunning}
	for _, state := range states {
		for {
			runs, err := rt.store.ListRunsByState(ctx, state, 100)
			if err != nil {
				log.Printf("runtime: boot passivation: query %s runs: %v", state, err)
				break
			}
			if len(runs) == 0 {
				break
			}
			progressed := false
			for i := range runs {
				rec := &runs[i]
				now := time.Now().UTC()
				rec.State = types.RunPassivated
				rec.Error = ""
				rec.UpdatedAt = now
				rec.FinishedAt = nil
				rec.Metadata = cloneMetadata(rec.Metadata)
				rec.Metadata["passivated_reason"] = "runtime_restarted"
				if item, err := rt.createSpawnedChildWorkItem(ctx, rec, nil); err != nil {
					log.Printf("runtime: boot passivation: create spawned work item for run %s: %v", rec.RunID, err)
				} else if item.WorkItemID != "" {
					rec.Metadata["work_item_ids"] = appendUniqueString(metadataStringSlice(rec.Metadata["work_item_ids"]), item.WorkItemID)
					rec.Metadata["passivated_spawned_work_item_id"] = item.WorkItemID
				}

				if err := rt.store.UpdateRun(ctx, *rec); err != nil {
					log.Printf("runtime: boot passivation: update run %s: %v", rec.RunID, err)
					continue
				}
				progressed = true
				if metadataString(rec.Metadata, "type") == "vtext_agent_revision" {
					if err := rt.store.MarkAgentMutationStale(ctx, rec.RunID); err != nil {
						log.Printf("runtime: boot passivation: stale mutation %s: %v", rec.RunID, err)
					}
				}
				rt.emitEvent(ctx, rec, types.EventRunPassivated, events.CauseSupervisorRecovery,
					json.RawMessage(`{"recovery":"passivated_on_restart"}`))
				log.Printf("runtime: passivated run %s (was %s) after restart", rec.RunID, state)
			}
			if !progressed {
				break
			}
		}
	}
}

func (rt *Runtime) sweepPendingUpdateActors(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	updates, err := rt.store.ListPendingWorkerUpdatesAll(ctx, 1000)
	if err != nil {
		log.Printf("runtime: boot update sweep: %v", err)
		return
	}
	seen := map[string]bool{}
	for _, update := range updates {
		ownerID := strings.TrimSpace(update.OwnerID)
		target := strings.TrimSpace(update.TargetAgentID)
		if ownerID == "" || target == "" {
			continue
		}
		key := ownerID + "\x00" + target
		if seen[key] {
			continue
		}
		seen[key] = true
		rt.wakeUpdatedCoagent(ctx, update)
	}
}

func (rt *Runtime) sweepOpenWorkItemActors(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	items, err := rt.store.ListOpenAssignedWorkItems(ctx, 1000)
	if err != nil {
		log.Printf("runtime: boot work-item sweep: %v", err)
		return
	}
	grouped := map[string][]types.WorkItemRecord{}
	for _, item := range items {
		ownerID := strings.TrimSpace(item.OwnerID)
		agentID := strings.TrimSpace(item.AssignedAgentID)
		trajectoryID := strings.TrimSpace(item.TrajectoryID)
		if ownerID == "" || agentID == "" || trajectoryID == "" {
			continue
		}
		key := ownerID + "\x00" + agentID + "\x00" + trajectoryID
		grouped[key] = append(grouped[key], item)
	}
	for _, workItems := range grouped {
		if _, err := rt.reconcileAssignedWorkItemActor(ctx, workItems); err != nil {
			first := workItems[0]
			log.Printf("runtime: boot work-item sweep owner=%s agent=%s trajectory=%s: %v",
				first.OwnerID, first.AssignedAgentID, first.TrajectoryID, err)
		}
	}
}

func (rt *Runtime) reconcileAssignedWorkItemActor(ctx context.Context, workItems []types.WorkItemRecord) (*types.RunRecord, error) {
	if rt == nil || rt.store == nil || len(workItems) == 0 {
		return nil, nil
	}
	first := workItems[0]
	ownerID := strings.TrimSpace(first.OwnerID)
	agentID := strings.TrimSpace(first.AssignedAgentID)
	trajectoryID := strings.TrimSpace(first.TrajectoryID)
	if ownerID == "" || agentID == "" || trajectoryID == "" {
		return nil, nil
	}
	if resident, found, err := rt.residentRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident assigned work-item actor: %w", err)
	} else if found {
		return &resident, nil
	}
	agent, err := rt.store.GetAgent(ctx, agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup assigned work-item actor: %w", err)
	}
	profile := canonicalAgentProfile(firstNonEmpty(agent.Profile, first.AuthorityProfile))
	if profile == "" || profile == AgentProfileEmail || profile == AgentProfileConductor || profile == AgentProfileVText {
		return nil, nil
	}
	role := strings.TrimSpace(firstNonEmpty(agent.Role, profile))
	channelID := strings.TrimSpace(agent.ChannelID)
	ids := make([]string, 0, len(workItems))
	for _, item := range workItems {
		if id := strings.TrimSpace(item.WorkItemID); id != "" {
			ids = append(ids, id)
		}
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      agentID,
		runMetadataTrajectoryID: trajectoryID,
		"request_source":        "trajectory_work_item_sweep",
		"work_item_ids":         ids,
	}
	if channelID != "" {
		metadata[runMetadataChannelID] = channelID
	}
	rec, err := rt.createRunWithMetadata(ctx, buildAssignedWorkItemPrompt(workItems), ownerID, metadata)
	if err != nil {
		return nil, err
	}
	rt.startRunAsync(rec)
	return rec, nil
}

func buildAssignedWorkItemPrompt(workItems []types.WorkItemRecord) string {
	var b strings.Builder
	b.WriteString("Resume the open trajectory work item records assigned to you.\n")
	b.WriteString("These durable obligations were discovered during runtime boot recovery; process them or report blockers with update_coagent.\n")
	for i, item := range workItems {
		b.WriteString("\nWork item ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if item.WorkItemID != "" {
			b.WriteString(" id=")
			b.WriteString(item.WorkItemID)
		}
		if item.TrajectoryID != "" {
			b.WriteString(" trajectory=")
			b.WriteString(item.TrajectoryID)
		}
		if item.AuthorityProfile != "" {
			b.WriteString(" authority=")
			b.WriteString(item.AuthorityProfile)
		}
		if item.StepBudget > 0 {
			b.WriteString(" step_budget=")
			b.WriteString(fmt.Sprintf("%d", item.StepBudget))
		}
		if item.TokenBudget > 0 {
			b.WriteString(" token_budget=")
			b.WriteString(fmt.Sprintf("%d", item.TokenBudget))
		}
		b.WriteString(":\nObjective: ")
		b.WriteString(strings.TrimSpace(item.Objective))
		if reason := strings.TrimSpace(item.Reason); reason != "" {
			b.WriteString("\nReason: ")
			b.WriteString(reason)
		}
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

// executeActivation runs one activation body using the configured provider.
// It transitions the run through pending → running → completed/failed/blocked,
// emitting events at each transition.
//
// When a tool registry is configured, the run executes through the real
// tool-calling loop (RunToolLoop), which handles tool_use stop reasons by
// invoking registered Go function-call tools and feeding results back to the
// provider. When no tool registry is configured, the run uses the simpler
// Provider.Execute path (stub or bridge provider).
func (rt *Runtime) executeActivation(ctx context.Context, rec *types.RunRecord) {
	defer rt.wg.Done()
	defer rt.removeRunning(rec.RunID)

	now := time.Now().UTC()

	// Transition to running.
	rec.State = types.RunRunning
	rec.UpdatedAt = now
	if err := rt.store.UpdateRun(ctx, *rec); err != nil {
		log.Printf("runtime: update run %s to running: %v", rec.RunID, err)
		rt.handleExecutionError(ctx, rec, fmt.Errorf("update run state: %w", err))
		return
	}

	rt.emitEvent(ctx, rec, types.EventRunStarted, events.CauseTaskLifecycle,
		json.RawMessage(`{}`))

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		cause := events.CauseProviderProgress
		if kind == types.EventToolInvoked || kind == types.EventToolResult {
			cause = events.CauseToolExecution
		}
		// Also emit vtext-specific progress events for agent revision runs.
		if taskType, _ := rec.Metadata["type"].(string); taskType == "vtext_agent_revision" {
			if docID, _ := rec.Metadata["doc_id"].(string); docID != "" {
				if kind == types.EventRunProgress {
					progressPayload, _ := json.Marshal(map[string]string{
						"doc_id":  docID,
						"loop_id": rec.RunID,
						"phase":   phase,
					})
					rt.emitVTextAgentEvent(ctx, rec, types.EventVTextAgentRevisionProgress,
						events.CauseProviderProgress, progressPayload)
				}
			}
		}
		rt.emitEvent(ctx, rec, kind, cause, payload)
	}

	registry := rt.toolRegistryForRun(rec)

	// Use the tool-calling loop if a tool registry is configured and the
	// provider supports the ToolLoopProvider interface. Otherwise, fall back
	// to the simple Provider.Execute path.
	if registry != nil && registry.Size() > 0 {
		rt.executeWithToolLoop(ctx, rec, registry, emit)
	} else {
		rt.executeWithProvider(ctx, rec, emit)
	}
}

// executeWithToolLoop runs the run through the real tool-calling loop.
// This is the primary execution path when a tool registry is configured,
// enabling the LLM to invoke registered Go function-call tools.
func (rt *Runtime) executeWithToolLoop(ctx context.Context, rec *types.RunRecord, registry *ToolRegistry, emit EventEmitFunc) {
	tlp := asToolLoopProvider(rt.provider)

	// Build the initial conversation from the run prompt.
	initialMessages := []json.RawMessage{}
	userMsg, _ := json.Marshal(map[string]any{
		"role": "user",
		"content": []any{
			map[string]string{"type": "text", "text": rec.Prompt},
		},
	})
	initialMessages = append(initialMessages, userMsg)

	systemPrompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}
	ctx = WithToolExecutionContext(ctx, rec)
	llmConfig := ResolvedLLMConfigFromMetadata(rec.Metadata)
	renderedSystemPrompt := systemPrompt
	if registry != nil {
		renderedSystemPrompt = buildSystemPromptWithTools(systemPrompt, registry)
	}
	memory := newRunMemoryManager(rt.store, rec, rt.cfg, emit).
		withLLMCompactor(tlp, llmConfig, estimateTextTokens(renderedSystemPrompt))
	initialMessages, err = memory.initialize(ctx, initialMessages)
	if err != nil {
		rt.handleExecutionError(ctx, rec, fmt.Errorf("initialize run memory: %w", err))
		return
	}
	maxOutputTokens := MaxInteractiveOutputTokensForSelection(llmConfig, agentProfileForRun(rec))
	terminalFallback := terminalProviderFallbackSelection()
	preconditionFallbacks := providerPreconditionFallbackSelections(llmConfig)
	if emit != nil {
		payload, _ := json.Marshal(map[string]any{
			"phase":                      "tool_loop_fallbacks_configured",
			"llm_provider":               llmConfig.Provider,
			"llm_model":                  llmConfig.Model,
			"terminal_fallback_provider": terminalFallback.Provider,
			"terminal_fallback_model":    terminalFallback.Model,
			"fallback_count":             len(preconditionFallbacks),
			"fallbacks":                  preconditionFallbacks,
		})
		emit(types.EventRunProgress, "tool_loop_fallbacks_configured", payload)
	}

	toolLoopOptions := []ToolLoopOption{
		WithToolLoopMemoryHooks(memory.hooks()),
		WithToolLoopLLMConfig(llmConfig),
		WithProviderPreconditionFallbacks(preconditionFallbacks...),
	}
	if metadataString(rec.Metadata, "type") == "vtext_agent_revision" {
		toolLoopOptions = append(toolLoopOptions, WithInitialToolChoice(initialVTextToolChoice(rec)))
		toolLoopOptions = append(toolLoopOptions, WithTerminalToolSuccesses(
			"spawn_agent",
			"request_super_execution",
			"request_email_draft",
		))
	}

	text, usage, err := RunToolLoop(ctx, tlp, registry, initialMessages, systemPrompt, maxOutputTokens, emit, rt.coagentUpdateTurnInjector(rec), toolLoopOptions...)
	if err != nil {
		if ctx.Err() != nil {
			rt.handleExecutionError(ctx, rec, ctx.Err())
		} else {
			rt.handleExecutionError(ctx, rec, err)
		}
		return
	}

	// Transition to completed.
	now := time.Now().UTC()
	rec.State = types.RunCompleted
	rec.Result = text
	rec.UpdatedAt = now
	rec.FinishedAt = &now

	rt.normalizeCompletedRunResult(rec)

	// Store token usage in metadata.
	if rec.Metadata == nil {
		rec.Metadata = make(map[string]any)
	}
	rec.Metadata["input_tokens"] = usage.InputTokens
	rec.Metadata["output_tokens"] = usage.OutputTokens

	// For vtext agent revision runs, create the canonical revision and emit the
	// vtext completion event before the run is surfaced as completed. This keeps
	// run completion aligned with document-version availability.
	if err := rt.handleRunCompletion(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}

	// Use a background context for post-provider persistence so that a fast
	// shutdown or cancellation after the provider returns cannot drop the
	// completed-run transition or parent notification.
	persistCtx := context.Background()

	if err := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(persistCtx, rec); err != nil {
		log.Printf("runtime: update run %s to completed: %v", rec.RunID, err)
		return
	}
	if shouldLogWireLifecycle(rec) {
		preview := rec.Result
		if len(preview) > 160 {
			preview = preview[:160]
		}
		log.Printf("runtime: completed %s result=%q", wireLifecycleSummary(rec), strings.ReplaceAll(preview, "\n", " "))
	}
	rt.reconcileCompletedVTextRun(rec)
	resultLenPayload, _ := json.Marshal(map[string]any{
		"result_length": len(text),
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	})
	rt.emitEvent(persistCtx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload)
	rt.maybeContinuePersistentSuperInbox(persistCtx, rec)

	rt.maybeStartConfiguredContinuation(persistCtx, rec)
}

// CompactRunMemory forces a durable run-memory checkpoint for an existing run.
// It is the runtime primitive behind manual compaction controls and uses the
// same compaction/event path as automatic threshold and overflow recovery.
func (rt *Runtime) CompactRunMemory(ctx context.Context, runID, ownerID, reason string) error {
	rec, err := rt.GetRun(ctx, runID, ownerID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" {
		reason = "manual"
	}
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		rt.emitEvent(ctx, rec, kind, events.CauseSupervisorRecovery, payload)
	}
	memory := newRunMemoryManager(rt.store, rec, rt.cfg, emit)
	memory.withLLMCompactor(asToolLoopProvider(rt.provider), ResolvedLLMConfigFromMetadata(rec.Metadata), 0)
	compacted, err := memory.compactIfNeeded(ctx, reason, true)
	if err != nil {
		return err
	}
	if !compacted {
		return fmt.Errorf("run memory compaction skipped: no compactable entries")
	}
	return nil
}

// executeWithProvider runs the run through the simple Provider.Execute path.
// This is the legacy execution path used when no tool registry is configured
// (stub provider or bridge provider without tool-calling support).
func (rt *Runtime) executeWithProvider(ctx context.Context, rec *types.RunRecord, emit EventEmitFunc) {
	// Execute through the provider. The provider may set rec.Result
	// directly (e.g., BridgeProvider sets it from the LLM response text).
	execRec := *rec
	execPrompt, err := rt.providerPromptForRun(rec)
	if err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}
	execRec.Prompt = execPrompt
	err = rt.provider.Execute(ctx, &execRec, emit)
	if err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}
	rec.Result = execRec.Result

	// Transition to completed.
	now := time.Now().UTC()
	rec.State = types.RunCompleted
	result := rec.Result
	if result == "" {
		result = rt.providerResult()
	}
	rec.Result = result
	rec.UpdatedAt = now
	rec.FinishedAt = &now

	rt.normalizeCompletedRunResult(rec)

	// For vtext agent revision runs, create the canonical revision and emit the
	// vtext completion event before the run is surfaced as completed. This keeps
	// run completion aligned with document-version availability.
	if err := rt.handleRunCompletion(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}

	// Use a background context for post-provider persistence so that a fast
	// shutdown or cancellation after the provider returns cannot drop the
	// completed-run transition or parent notification.
	persistCtx := context.Background()

	if err := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(persistCtx, rec); err != nil {
		log.Printf("runtime: update run %s to completed: %v", rec.RunID, err)
		return
	}
	rt.reconcileCompletedVTextRun(rec)
	resultLenPayload, _ := json.Marshal(map[string]int{"result_length": len(result)})
	rt.emitEvent(persistCtx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload)
	rt.maybeContinuePersistentSuperInbox(persistCtx, rec)

	rt.maybeStartConfiguredContinuation(persistCtx, rec)

}

func (rt *Runtime) normalizeCompletedRunResult(rec *types.RunRecord) {
	if rec == nil {
		return
	}
	if agentProfileForRun(rec) != AgentProfileConductor {
		return
	}
	rec.Result = normalizeConductorDecision(rec)
}

type conductorDecision struct {
	Action               string `json:"action"`
	App                  string `json:"app,omitempty"`
	Title                string `json:"title,omitempty"`
	SeedPrompt           string `json:"seed_prompt,omitempty"`
	InitialContent       string `json:"initial_content,omitempty"`
	CreateInitialVersion *bool  `json:"create_initial_version,omitempty"`
	Message              string `json:"message,omitempty"`
	SourceURL            string `json:"source_url,omitempty"`
	MediaType            string `json:"media_type,omitempty"`
	AppHint              string `json:"app_hint,omitempty"`
	ContentID            string `json:"content_id,omitempty"`
	DocID                string `json:"doc_id,omitempty"`
	UserRevisionID       string `json:"user_revision_id,omitempty"`
	FramingRevisionID    string `json:"framing_revision_id,omitempty"`
	InitialRevisionID    string `json:"initial_revision_id,omitempty"`
	InitialLoopID        string `json:"initial_loop_id,omitempty"`
}

func conductorSeedPrompt(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	seedPrompt, _ := rec.Metadata["seed_prompt"].(string)
	if strings.TrimSpace(seedPrompt) == "" {
		seedPrompt = strings.TrimSpace(rec.Prompt)
	}
	return strings.TrimSpace(seedPrompt)
}

func conductorRequestedApp(rec *types.RunRecord) string {
	if rec == nil {
		return AgentProfileVText
	}
	requestedApp, _ := rec.Metadata["requested_app"].(string)
	if strings.TrimSpace(requestedApp) == "" {
		requestedApp = AgentProfileVText
	}
	return strings.TrimSpace(requestedApp)
}

func conductorWindowTitle(rec *types.RunRecord, seedPrompt string) string {
	if rec == nil {
		if strings.TrimSpace(seedPrompt) != "" {
			return strings.TrimSpace(seedPrompt)
		}
		return "VText"
	}
	title, _ := rec.Metadata["initial_document_title"].(string)
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(seedPrompt)
	}
	if strings.TrimSpace(title) == "" {
		title = "VText"
	}
	return strings.TrimSpace(title)
}

func fillConductorDecisionFromRun(rec *types.RunRecord, decision conductorDecision) conductorDecision {
	seedPrompt := conductorSeedPrompt(rec)
	requestedApp := conductorRequestedApp(rec)
	if strings.TrimSpace(decision.Action) == "" {
		decision.Action = "open_app"
	}
	if decision.Action == "open_app" {
		if strings.TrimSpace(decision.App) == "" {
			decision.App = requestedApp
		}
		if strings.TrimSpace(decision.Title) == "" {
			decision.Title = conductorWindowTitle(rec, seedPrompt)
		}
		if strings.TrimSpace(decision.SeedPrompt) == "" {
			decision.SeedPrompt = seedPrompt
		}
		if decision.App == AgentProfileVText {
			decision.CreateInitialVersion = ptrBool(false)
			decision.InitialContent = ""
		}
		if rec != nil && rec.Metadata != nil {
			if decision.SourceURL == "" {
				decision.SourceURL = metadataStringValue(rec.Metadata, "content_source_url")
			}
			if decision.MediaType == "" {
				decision.MediaType = metadataStringValue(rec.Metadata, "content_media_type")
			}
			if decision.AppHint == "" {
				decision.AppHint = metadataStringValue(rec.Metadata, "content_app_hint")
			}
			if decision.ContentID == "" {
				decision.ContentID = metadataStringValue(rec.Metadata, "content_id")
			}
			if decision.DocID == "" {
				decision.DocID = metadataStringValue(rec.Metadata, "doc_id")
			}
			if decision.UserRevisionID == "" {
				decision.UserRevisionID = metadataStringValue(rec.Metadata, "user_revision_id")
			}
			if decision.FramingRevisionID == "" {
				decision.FramingRevisionID = metadataStringValue(rec.Metadata, "framing_revision_id")
			}
			if decision.InitialRevisionID == "" {
				decision.InitialRevisionID = metadataStringValue(rec.Metadata, "initial_revision_id")
			}
			if decision.InitialLoopID == "" {
				decision.InitialLoopID = metadataStringValue(rec.Metadata, "initial_loop_id")
			}
		}
	}
	if decision.Action == "toast" && strings.TrimSpace(decision.Message) == "" {
		decision.Message = "Conductor acknowledged the request."
	}
	return decision
}

func mergeStoredConductorRoute(rec *types.RunRecord, stored types.RunRecord) {
	if rec == nil {
		return
	}
	if rec.Metadata == nil {
		rec.Metadata = make(map[string]any)
	}
	for _, key := range []string{
		"doc_id",
		"user_revision_id",
		"framing_revision_id",
		"initial_revision_id",
		"initial_loop_id",
	} {
		if value := metadataStringValue(stored.Metadata, key); value != "" {
			rec.Metadata[key] = value
		}
	}
	var storedDecision conductorDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(stored.Result)), &storedDecision); err == nil &&
		storedDecision.Action == "open_app" &&
		storedDecision.App == AgentProfileVText &&
		strings.TrimSpace(storedDecision.DocID) != "" {
		rec.Result = stored.Result
	}
}

func normalizeConductorDecision(rec *types.RunRecord) string {
	defaultDecision := fillConductorDecisionFromRun(rec, conductorDecision{})
	if rec == nil {
		out, err := json.Marshal(defaultDecision)
		if err != nil {
			return `{"action":"open_app","app":"vtext","title":"VText","seed_prompt":"","create_initial_version":false}`
		}
		return string(out)
	}

	if raw := strings.TrimSpace(rec.Result); raw != "" {
		var parsed conductorDecision
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && strings.TrimSpace(parsed.Action) != "" {
			switch strings.TrimSpace(parsed.Action) {
			case "toast":
				parsed = fillConductorDecisionFromRun(rec, parsed)
				if metadataStringValue(rec.Metadata, "doc_id") != "" && metadataStringValue(rec.Metadata, "requested_app") == AgentProfileVText {
					parsed.Action = "open_app"
					parsed.App = AgentProfileVText
					parsed = fillConductorDecisionFromRun(rec, parsed)
				}
			case "open_app":
				parsed = fillConductorDecisionFromRun(rec, parsed)
				if !isAllowedProductApp(strings.TrimSpace(parsed.App)) {
					parsed.App = defaultDecision.App
				}
			default:
				parsed = defaultDecision
			}
			if out, err := json.Marshal(parsed); err == nil {
				return string(out)
			}
		}
	}

	out, err := json.Marshal(defaultDecision)
	if err != nil {
		return `{"action":"open_app","app":"vtext","title":"VText","seed_prompt":"","create_initial_version":false}`
	}
	return string(out)
}

func ptrBool(v bool) *bool {
	return &v
}

func buildInitialVTextTitle(seedPrompt, objective string) string {
	source := strings.TrimSpace(seedPrompt)
	if source == "" {
		source = strings.TrimSpace(objective)
	}
	source = strings.Trim(source, " \t\r\n.:;!?")
	if source == "" {
		return "Working Document"
	}
	words := strings.Fields(source)
	if len(words) > 9 {
		words = words[:9]
	}
	title := strings.Join(words, " ")
	title = strings.Trim(title, " \t\r\n.:;!?")
	if title == "" {
		return "Working Document"
	}
	return title
}

func fallbackPromptBarInitialContent(rec *types.RunRecord, decision conductorDecision) string {
	if rec == nil || metadataStringValue(rec.Metadata, "input_source") != "prompt_bar" {
		return ""
	}
	if conductorRequestedApp(rec) != AgentProfileVText {
		return ""
	}
	seedPrompt := strings.TrimSpace(decision.SeedPrompt)
	if seedPrompt == "" {
		seedPrompt = conductorSeedPrompt(rec)
	}
	if seedPrompt == "" {
		return ""
	}
	title := strings.TrimSpace(decision.Title)
	if title == "" {
		title = conductorWindowTitle(rec, seedPrompt)
	}
	if title == "" {
		title = buildInitialVTextTitle(seedPrompt, "")
	}
	if title == "" || strings.EqualFold(title, seedPrompt) {
		return seedPrompt
	}
	return "# " + title + "\n\n" + seedPrompt
}

func (rt *Runtime) ensureConductorVTextRoute(ctx context.Context, rec *types.RunRecord, objective, initialContent string) (conductorDecision, error) {
	if rec == nil || agentProfileForRun(rec) != AgentProfileConductor {
		return conductorDecision{}, fmt.Errorf("conductor route requires a conductor record")
	}
	rt.conductorRouteMu.Lock()
	defer rt.conductorRouteMu.Unlock()

	if current, err := rt.store.GetRun(ctx, rec.RunID); err == nil {
		mergeStoredConductorRoute(rec, current)
	}

	var parsedDecision conductorDecision
	if raw := strings.TrimSpace(rec.Result); raw != "" {
		if err := json.Unmarshal([]byte(raw), &parsedDecision); err == nil {
			if strings.TrimSpace(initialContent) == "" {
				initialContent = parsedDecision.InitialContent
			}
			if parsedDecision.Action == "open_app" &&
				parsedDecision.App == AgentProfileVText &&
				strings.TrimSpace(parsedDecision.DocID) != "" {
				return fillConductorDecisionFromRun(rec, parsedDecision), nil
			}
		}
	}
	existing := fillConductorDecisionFromRun(rec, conductorDecision{})
	if existing.Action == "open_app" && existing.App == AgentProfileVText && strings.TrimSpace(existing.DocID) != "" {
		return existing, nil
	}

	now := time.Now().UTC()
	decision := fillConductorDecisionFromRun(rec, parsedDecision)
	decision.CreateInitialVersion = ptrBool(false)
	decision.InitialContent = ""
	initialContent = ""
	doc := types.Document{
		DocID:     uuid.New().String(),
		OwnerID:   rec.OwnerID,
		Title:     decision.Title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if strings.TrimSpace(doc.Title) == "" {
		doc.Title = "VText"
	}
	if err := rt.store.CreateDocument(ctx, doc); err != nil {
		return conductorDecision{}, fmt.Errorf("create vtext document: %w", err)
	}

	userRevisionID := uuid.New().String()
	userRevMeta, _ := json.Marshal(map[string]any{
		"seed_prompt":         decision.SeedPrompt,
		"conductor_loop_id":   rec.RunID,
		runMetadataOwnerEmail: metadataString(rec.Metadata, runMetadataOwnerEmail),
		"created_from":        "conductor",
		"source":              "user_prompt",
		"revision_role":       vtextRevisionRoleInput,
		"input_origin":        vtextInputOriginUserPrompt,
		"vtext_version":       "v0",
	})
	userRev := types.Revision{
		RevisionID:  userRevisionID,
		DocID:       doc.DocID,
		OwnerID:     rec.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: rec.OwnerID,
		Content:     decision.SeedPrompt,
		Citations:   json.RawMessage("[]"),
		Metadata:    userRevMeta,
		CreatedAt:   now,
	}
	if err := rt.store.CreateRevision(ctx, userRev); err != nil {
		return conductorDecision{}, fmt.Errorf("create user prompt vtext revision: %w", err)
	}
	rt.emitVTextDocumentRevisionEventForRun(ctx, rec, userRev)

	doc.CurrentRevisionID = userRev.RevisionID
	if err := rt.store.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + doc.DocID,
		OwnerID:   rec.OwnerID,
		SandboxID: rt.cfg.SandboxID,
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: doc.DocID,
		CreatedAt: now,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return conductorDecision{}, fmt.Errorf("persist vtext appagent: %w", err)
	}
	if _, err := rt.EnsurePersistentSuperAgent(ctx, rec.OwnerID); err != nil {
		return conductorDecision{}, fmt.Errorf("persist persistent super appagent: %w", err)
	}

	decision.DocID = doc.DocID
	decision.UserRevisionID = userRev.RevisionID
	if decision.InitialRevisionID == "" {
		decision.InitialRevisionID = userRev.RevisionID
	}

	initialPrompt := strings.TrimSpace(objective)
	if initialPrompt == "" {
		initialPrompt = strings.TrimSpace(decision.SeedPrompt)
	}
	if initialPrompt == "" {
		initialPrompt = "Create the first useful current-state version of this vtext document."
	}
	if vtextPromptNeedsSuperExecution(decision.SeedPrompt + " " + initialPrompt) {
		requestCtx := WithToolExecutionContext(ctx, &types.RunRecord{
			RunID:        rec.RunID,
			AgentID:      "vtext:" + doc.DocID,
			ChannelID:    doc.DocID,
			AgentProfile: AgentProfileVText,
			AgentRole:    AgentProfileVText,
			OwnerID:      rec.OwnerID,
			SandboxID:    rec.SandboxID,
			Metadata:     rec.Metadata,
		})
		superResult, err := rt.requestPersistentSuperExecution(requestCtx, rec.OwnerID, doc.DocID, rec.RunID, "vtext:"+doc.DocID, initialPrompt, "")
		if err != nil {
			return conductorDecision{}, fmt.Errorf("request initial super execution: %w", err)
		}
		if loopID, _ := superResult["loop_id"].(string); strings.TrimSpace(loopID) != "" {
			decision.InitialLoopID = loopID
		}
		decision = fillConductorDecisionFromRun(rec, decision)
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]any)
		}
		rec.Metadata["doc_id"] = decision.DocID
		rec.Metadata["user_revision_id"] = decision.UserRevisionID
		rec.Metadata["initial_revision_id"] = decision.InitialRevisionID
		rec.Metadata["initial_loop_id"] = decision.InitialLoopID
		rec.Metadata["initial_handoff"] = "persistent_super"
		if out, err := json.Marshal(decision); err == nil {
			rec.Result = string(out)
		}
		rec.UpdatedAt = time.Now().UTC()

		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			return conductorDecision{}, fmt.Errorf("persist conductor route: %w", err)
		}
		return decision, nil
	}
	initialRun, err := rt.submitVTextAgentRevisionRun(ctx, doc, rec.OwnerID, vtextAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: initialPrompt,
	}, rec.RunID, 0)
	if err != nil {
		return conductorDecision{}, fmt.Errorf("start initial vtext agent revision: %w", err)
	}
	decision.InitialLoopID = initialRun.RunID
	decision = fillConductorDecisionFromRun(rec, decision)

	if rec.Metadata == nil {
		rec.Metadata = make(map[string]any)
	}
	rec.Metadata["doc_id"] = decision.DocID
	rec.Metadata["user_revision_id"] = decision.UserRevisionID
	rec.Metadata["initial_revision_id"] = decision.InitialRevisionID
	rec.Metadata["initial_loop_id"] = decision.InitialLoopID
	if out, err := json.Marshal(decision); err == nil {
		rec.Result = string(out)
	}
	rec.UpdatedAt = time.Now().UTC()

	if err := rt.store.UpdateRun(ctx, *rec); err != nil {
		return conductorDecision{}, fmt.Errorf("persist conductor route: %w", err)
	}
	return decision, nil
}

func (rt *Runtime) materializeConductorDecision(rec *types.RunRecord) {
	if rec == nil || agentProfileForRun(rec) != AgentProfileConductor {
		return
	}

	var decision conductorDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(rec.Result)), &decision); err != nil {
		return
	}
	if decision.Action == "toast" &&
		metadataStringValue(rec.Metadata, "requested_app") == AgentProfileVText &&
		metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		if _, err := rt.ensureConductorVTextRoute(context.Background(), rec, "", decision.InitialContent); err != nil {
			log.Printf("runtime: conductor run %s: materialize prompt-bar vtext route: %v", rec.RunID, err)
		}
		return
	}
	if decision.Action != "open_app" || decision.App != AgentProfileVText || strings.TrimSpace(decision.DocID) != "" {
		return
	}

	if _, err := rt.ensureConductorVTextRoute(context.Background(), rec, "", decision.InitialContent); err != nil {
		log.Printf("runtime: conductor run %s: materialize decision: %v", rec.RunID, err)
	}
}

func initialVTextToolChoice(rec *types.RunRecord) string {
	if rec == nil || metadataStringValue(rec.Metadata, "type") != "vtext_agent_revision" {
		return ""
	}
	if metadataIntValue(rec.Metadata, "scheduled_message_seq") > 0 {
		return ""
	}
	if vtextPromptNeedsSuperExecution(metadataStringValue(rec.Metadata, "seed_prompt") + " " + metadataStringValue(rec.Metadata, "original_prompt")) {
		return exactRequiredToolChoice("request_super_execution")
	}
	return exactRequiredToolChoice("edit_vtext")
}

func vtextPromptNeedsSuperExecution(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	superMarkers := []string{
		"app/harness",
		"choir-in-choir",
		"candidate-world",
		"candidate world",
		"worker vm",
		"vsuper",
		"co-super",
		"cosuper",
		"appchangepackage",
		"app change package",
		"promotion",
		"promote",
		"rollback",
		"deploy",
		"ci/cd",
		"nixos",
		"github",
		"repo",
		"code",
		"coding",
		"implement",
		"fix",
		"debug",
		"test",
		"verify",
		"verifier",
		"staging proof",
		"product path",
		"product-path",
		"evidence ids",
		"source-refresh",
		"publication flow",
		"benchmark",
		"artifact",
		"execution",
		"execute",
		"bash",
		"shell",
		"terminal",
		"script",
		"run a command",
		"run one command",
		"run exactly one command",
		"run exactly one tiny command",
		"write a command",
		"command output",
		"command evidence",
		"[cmd]",
	}
	for _, marker := range superMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// handleRunCompletion processes feature-specific side effects after a run
// completes successfully. VText document writes are intentionally not handled
// here: canonical appagent revisions are created only by the edit_vtext tool.
func (rt *Runtime) handleRunCompletion(ctx context.Context, rec *types.RunRecord) error {
	if agentProfileForRun(rec) == AgentProfileConductor {
		rt.materializeConductorDecision(rec)
		return nil
	}

	taskType, _ := rec.Metadata["type"].(string)
	if taskType != "vtext_agent_revision" {
		return nil
	}

	persistCtx := context.Background()

	docID, _ := rec.Metadata["doc_id"].(string)
	if docID == "" {
		log.Printf("runtime: vtext agent revision run %s: missing doc_id in metadata", rec.RunID)
		return nil
	}

	mutation, err := rt.store.GetAgentMutationByRun(persistCtx, rec.RunID)
	if err != nil {
		log.Printf("runtime: vtext agent revision run %s: get mutation: %v", rec.RunID, err)
		return nil
	}
	if mutation == nil {
		log.Printf("runtime: vtext agent revision run %s: no mutation record found", rec.RunID)
		return nil
	}
	if mutation.State == "completed" {
		return nil
	}
	if mutation.State != "pending" {
		return nil
	}

	if rt.vtextRunRequestedWorkers(persistCtx, rec) {
		if err := rt.store.DeferAgentMutation(persistCtx, rec.RunID); err != nil {
			log.Printf("runtime: vtext agent revision run %s: defer no-edit mutation: %v", rec.RunID, err)
			return nil
		}
		progressPayload, _ := json.Marshal(map[string]string{
			"doc_id":  docID,
			"loop_id": rec.RunID,
			"status":  "waiting_for_worker_updates",
		})
		rt.emitVTextAgentEvent(persistCtx, rec, types.EventVTextAgentRevisionProgress,
			events.CauseToolExecution, progressPayload)
		log.Printf("runtime: vtext agent revision run %s requested workers and completed without document edit; waiting for worker updates", rec.RunID)
		return nil
	}
	_ = rt.store.FailAgentMutation(persistCtx, rec.RunID)
	if mutation.ScheduledMessageSeq > 0 {
		if err := rt.store.UpsertVTextControllerCheckpoint(persistCtx, store.VTextControllerCheckpoint{
			DocID:                docID,
			OwnerID:              rec.OwnerID,
			IntegratedMessageSeq: mutation.ScheduledMessageSeq,
			UpdatedAt:            time.Now().UTC(),
		}); err != nil {
			log.Printf("runtime: vtext agent revision run %s: update no-edit checkpoint: %v", rec.RunID, err)
		} else if err := rt.markVTextWorkerUpdatesDelivered(persistCtx, rec, docID, mutation.ScheduledMessageSeq); err != nil {
			log.Printf("runtime: vtext agent revision run %s: mark worker updates delivered: %v", rec.RunID, err)
		}
	}
	failPayload, _ := json.Marshal(map[string]string{
		"doc_id":  docID,
		"loop_id": rec.RunID,
		"error":   "vtext run completed without calling edit_vtext",
	})
	rt.emitVTextAgentEvent(persistCtx, rec, types.EventVTextAgentRevisionFailed,
		events.CauseTaskLifecycle, failPayload)
	log.Printf("runtime: vtext agent revision run %s completed without edit_vtext; no canonical revision created", rec.RunID)
	return nil
}

func (rt *Runtime) vtextRunRequestedWorkers(ctx context.Context, rec *types.RunRecord) bool {
	if rt == nil || rt.store == nil || rec == nil {
		return false
	}
	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 500)
	if err != nil {
		log.Printf("runtime: vtext run %s: list events for worker requests: %v", rec.RunID, err)
		return false
	}
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload struct {
			Tool    string `json:"tool"`
			IsError bool   `json:"is_error"`
			Output  string `json:"output"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError {
			continue
		}
		switch strings.TrimSpace(payload.Tool) {
		case "request_super_execution":
			return true
		case "spawn_agent":
			var output map[string]any
			if err := json.Unmarshal([]byte(payload.Output), &output); err != nil {
				continue
			}
			profile, _ := output["profile"].(string)
			role, _ := output["role"].(string)
			if strings.TrimSpace(profile) == AgentProfileResearcher || strings.TrimSpace(role) == AgentProfileResearcher {
				return true
			}
		}
	}
	return false
}

func (rt *Runtime) reconcileCompletedVTextRun(rec *types.RunRecord) {
	if rec == nil {
		return
	}
	docID, _ := rec.Metadata["doc_id"].(string)
	if strings.TrimSpace(docID) == "" {
		agentID := agentIDForRun(rec)
		if strings.HasPrefix(agentID, "vtext:") {
			docID = strings.TrimPrefix(agentID, "vtext:")
		}
	}
	if strings.TrimSpace(docID) == "" && agentProfileForRun(rec) == AgentProfileVText {
		docID = channelIDForRun(rec)
	}
	if strings.TrimSpace(docID) == "" || strings.TrimSpace(rec.OwnerID) == "" {
		return
	}
	if err := rt.reconcileVTextWorkerState(context.Background(), rec.OwnerID, docID); err != nil {
		log.Printf("runtime: vtext agent revision run %s: post-complete reconcile: %v", rec.RunID, err)
	}
}

func (rt *Runtime) channelHasGroundedHistory(ctx context.Context, ownerID, channelID string, before time.Time) (bool, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return false, nil
	}
	runs, err := rt.store.ListRunsByChannel(ctx, ownerID, channelID, 500)
	if err != nil {
		return false, err
	}
	groundedRunIDs := make(map[string]struct{})
	for _, run := range runs {
		if !before.IsZero() && !run.CreatedAt.Before(before) {
			continue
		}
		switch agentProfileForRun(&run) {
		case AgentProfileResearcher, AgentProfileSuper, AgentProfileCoSuper:
			groundedRunIDs[run.RunID] = struct{}{}
		}
	}
	if len(groundedRunIDs) == 0 {
		return false, nil
	}
	messages, err := rt.store.ListChannelMessages(ctx, ownerID, channelID, 0, 500)
	if err != nil {
		return false, err
	}
	for _, message := range messages {
		if !before.IsZero() && !message.Timestamp.Before(before) {
			continue
		}
		if _, ok := groundedRunIDs[strings.TrimSpace(message.FromRunID)]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (rt *Runtime) maybeWakeVTextOnWorkerMessage(ctx context.Context, ownerID string, message ChannelMessage) {
	channelID := strings.TrimSpace(message.ChannelID)
	fromRunID := strings.TrimSpace(message.FromRunID)
	targetAgentID := strings.TrimSpace(message.ToAgentID)
	if strings.TrimSpace(ownerID) == "" || channelID == "" || targetAgentID == "" {
		return
	}

	doc, err := rt.store.GetDocument(ctx, channelID, ownerID)
	if err != nil {
		if err != store.ErrNotFound {
			log.Printf("runtime: wake vtext for channel %s: get document: %v", channelID, err)
		}
		return
	}

	if fromRunID != "" {
		sourceRun, err := rt.store.GetRun(ctx, fromRunID)
		if err != nil {
			log.Printf("runtime: wake vtext for doc %s: get source run %s: %v", doc.DocID, fromRunID, err)
			return
		}
		switch agentProfileForRun(&sourceRun) {
		case AgentProfileResearcher, AgentProfileSuper, AgentProfileVSuper, AgentProfileCoSuper:
		default:
			return
		}
	}

	agentID := "vtext:" + doc.DocID
	if targetAgentID != agentID {
		return
	}
	rt.scheduleVTextWorkerWake(ownerID, doc.DocID, fromRunID)
}

// durableMetadataKeys lists the revision metadata keys that must survive
// across appagent revisions so that subsequent revise requests retain
// the original user context (seed_prompt, source_path, etc.).
var durableMetadataKeys = []string{
	"seed_prompt",
	"source_path",
	"canonical_vtext_source_path",
	"import_manifest",
	"migration_manifest",
	"conductor_loop_id",
	"media_source_refs",
	"media_source_research_required",
	"source_entities",
	"artifact_kind",
	"revision_role",
	"input_origin",
	"vtext_version_stage",
	"source_network_cycle_id",
	"source_network_request_id",
	"source_network_request_kind",
	"ingestion_handoff_cycle_id",
	"ingestion_handoff_request_id",
	"ingestion_handoff_request_kind",
	"source_item_ids",
	"processor_key",
	"reconciler_scope",
	"selected_style_sources",
	"selected_style_rationale",
	runMetadataOwnerEmail,
}

// buildAppagentRevisionMetadata constructs the metadata JSON for an
// appagent-authored revision, carrying forward durable context keys
// from the parent revision so they remain available on the next revise.
func (rt *Runtime) buildAppagentRevisionMetadata(ctx context.Context, rec *types.RunRecord, doc types.Document, ownerID string, mutation *store.AgentMutation) json.RawMessage {
	meta := map[string]any{
		"source":  "edit_vtext",
		"loop_id": rec.RunID,
	}

	// Carry forward durable keys from the parent revision metadata.
	if doc.CurrentRevisionID != "" {
		if parentRev, err := rt.store.GetRevision(context.Background(), doc.CurrentRevisionID, ownerID); err == nil {
			parentMeta := decodeRevisionMetadata(parentRev.Metadata)
			for _, key := range durableMetadataKeys {
				if val, ok := parentMeta[key]; ok && hasNonEmptyVTextMetadataValue(val) {
					meta[key] = val
				}
			}
		}
	}

	// Also carry forward from run metadata (the initial agent revision
	// request sets these directly).
	if rec.Metadata != nil {
		for _, key := range durableMetadataKeys {
			if val, ok := rec.Metadata[key]; ok && hasNonEmptyVTextMetadataValue(val) {
				// Run metadata takes precedence over parent revision.
				meta[key] = val
			}
		}
	}
	if wirepublish.IsWireArticleRevisionRun(rec) {
		meta["artifact_kind"] = "article_revision"
		meta["revision_role"] = vtextRevisionRoleCanonical
		meta["vtext_version_stage"] = "article_revision"
	}
	workerUpdateMeta := rt.workerUpdateRevisionMetadata(ctx, ownerID, doc.DocID, mutation)
	if vtextWorkerUpdateMetadataHasRole(workerUpdateMeta["worker_updates_consumed"], AgentProfileResearcher) {
		markVTextMediaSourceRefsResearchState(meta, "represented")
	}
	for key, value := range workerUpdateMeta {
		meta[key] = value
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return json.RawMessage(`{"source":"edit_vtext","loop_id":"` + rec.RunID + `"}`)
	}
	return data
}

func vtextWorkerUpdateMetadataHasRole(value any, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	switch updates := value.(type) {
	case []vtextWorkerUpdateMetadata:
		for _, update := range updates {
			if strings.TrimSpace(update.Role) == role {
				return true
			}
		}
	case []any:
		for _, raw := range updates {
			item, _ := raw.(map[string]any)
			if strings.TrimSpace(fmt.Sprint(item["role"])) == role {
				return true
			}
		}
	}
	return false
}

type vtextWorkerUpdateMetadata struct {
	ChannelID      string `json:"channel_id"`
	Seq            int64  `json:"seq"`
	FromAgentID    string `json:"from_agent_id,omitempty"`
	FromLoopID     string `json:"from_loop_id,omitempty"`
	Role           string `json:"role,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

func (rt *Runtime) workerUpdateRevisionMetadata(ctx context.Context, ownerID, docID string, mutation *store.AgentMutation) map[string]any {
	out := map[string]any{
		"worker_updates_policy":         "eligible_addressed_channel_messages",
		"worker_updates_checkpoint_seq": int64(0),
		"worker_updates_scheduled_seq":  int64(0),
		"worker_updates_consumed":       []vtextWorkerUpdateMetadata{},
		"worker_updates_skipped":        []vtextWorkerUpdateMetadata{},
		"worker_updates_pending":        []vtextWorkerUpdateMetadata{},
	}
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(docID) == "" {
		return out
	}

	scheduledSeq := int64(0)
	if mutation != nil {
		scheduledSeq = mutation.ScheduledMessageSeq
	}
	out["worker_updates_scheduled_seq"] = scheduledSeq

	checkpointSeq := int64(0)
	checkpoint, err := rt.store.GetVTextControllerCheckpoint(ctx, docID, ownerID)
	if err != nil {
		log.Printf("runtime: load vtext worker update checkpoint for metadata: %v", err)
		return out
	}
	if checkpoint != nil {
		checkpointSeq = checkpoint.IntegratedMessageSeq
	}
	out["worker_updates_checkpoint_seq"] = checkpointSeq

	messages, err := rt.store.ListChannelMessages(ctx, ownerID, docID, checkpointSeq, 500)
	if err != nil {
		log.Printf("runtime: load vtext worker update messages for metadata: %v", err)
		return out
	}

	targetAgentID := "vtext:" + strings.TrimSpace(docID)
	cache := make(map[string]bool)
	consumed := []vtextWorkerUpdateMetadata{}
	skipped := []vtextWorkerUpdateMetadata{}
	pending := []vtextWorkerUpdateMetadata{}
	for _, message := range messages {
		if strings.TrimSpace(message.ToAgentID) != targetAgentID {
			continue
		}
		eligible, err := rt.isEligibleWorkerMessage(ctx, docID, message, cache)
		if err != nil {
			log.Printf("runtime: classify vtext worker update for metadata: %v", err)
			continue
		}
		if scheduledSeq > 0 && message.Seq <= scheduledSeq {
			if eligible {
				consumed = append(consumed, summarizeWorkerUpdateForMetadata(message, ""))
			} else {
				skipped = append(skipped, summarizeWorkerUpdateForMetadata(message, "ineligible_sender"))
			}
			continue
		}
		if eligible {
			pending = append(pending, summarizeWorkerUpdateForMetadata(message, "after_scheduled_checkpoint"))
		} else if scheduledSeq > 0 && message.Seq <= scheduledSeq {
			skipped = append(skipped, summarizeWorkerUpdateForMetadata(message, "ineligible_sender"))
		}
	}

	out["worker_updates_consumed"] = consumed
	out["worker_updates_skipped"] = skipped
	out["worker_updates_pending"] = pending
	return out
}

func summarizeWorkerUpdateForMetadata(message types.ChannelMessage, reason string) vtextWorkerUpdateMetadata {
	return vtextWorkerUpdateMetadata{
		ChannelID:      message.ChannelID,
		Seq:            message.Seq,
		FromAgentID:    strings.TrimSpace(message.FromAgentID),
		FromLoopID:     strings.TrimSpace(message.FromRunID),
		Role:           strings.TrimSpace(message.Role),
		ContentPreview: truncatePromptSnippet(message.Content, 240),
		Reason:         strings.TrimSpace(reason),
	}
}

func (rt *Runtime) markVTextWorkerUpdatesDelivered(ctx context.Context, rec *types.RunRecord, docID string, maxSeq int64) error {
	if rt == nil || rt.store == nil || rec == nil || maxSeq <= 0 {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil
	}
	targetAgentID := "vtext:" + docID
	updates, err := rt.store.ListPendingWorkerUpdates(ctx, ownerID, targetAgentID, 500)
	if err != nil {
		return err
	}
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if strings.TrimSpace(update.ChannelID) == docID && update.MessageSeq > 0 && update.MessageSeq <= maxSeq {
			updateIDs = append(updateIDs, update.UpdateID)
		}
	}
	if len(updateIDs) == 0 {
		return nil
	}
	return rt.store.MarkWorkerUpdatesDelivered(ctx, ownerID, targetAgentID, updateIDs, rec.RunID)
}

// handleExecutionError transitions a run to failed/blocked and emits the
// appropriate event. The runtime remains available for later runs
// (VAL-RUNTIME-008).
//
// Note: When the error is caused by context cancellation (runtime shutdown),
// the passed ctx will be cancelled. We use context.Background() for the
// critical store updates so that the run state is properly persisted even
// during shutdown (VAL-CHOIR-009, VAL-CHOIR-010).
func (rt *Runtime) handleExecutionError(ctx context.Context, rec *types.RunRecord, err error) {
	now := time.Now().UTC()

	// Determine if the failure is recoverable (blocked) or permanent (failed).
	state := types.RunFailed
	kind := types.EventRunFailed
	cause := events.CauseProviderFailure

	if ctx.Err() != nil {
		// Context cancellation means the runtime is shutting down or the
		// run was cancelled, not a provider failure. Treat as cancelled.
		state = types.RunCancelled
		kind = types.EventRunCancelled
		cause = events.CauseTaskLifecycle
	} else if isRunMemoryBlockedError(err) {
		state = types.RunBlocked
		kind = types.EventRunBlocked
		cause = events.CauseSupervisorRecovery
	} else if isProviderRateLimitError(err) {
		state = types.RunBlocked
		kind = types.EventRunBlocked
		cause = events.CauseSupervisorRecovery
	}

	rec.State = state
	rec.Error = err.Error()
	rec.UpdatedAt = now
	if state.Terminal() {
		rec.FinishedAt = &now
	} else {
		rec.FinishedAt = nil
	}

	// Use background context for persistence so that cancelled-run state
	// transitions are persisted even when the run context is cancelled.
	persistCtx := context.Background()
	if updateErr := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(persistCtx, rec); updateErr != nil {
		log.Printf("runtime: update run %s to %s: %v", rec.RunID, state, updateErr)
	}

	errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
	rt.emitEvent(persistCtx, rec, kind, cause, errPayload)
	if synthErr := rt.synthesizeDelegateWorkerUpdateOnSuperFailure(persistCtx, rec, err); synthErr != nil {
		log.Printf("runtime: synthesize delegate worker update for run %s: %v", rec.RunID, synthErr)
	}
	if synthErr := rt.synthesizeSuperFailureUpdate(persistCtx, rec, err); synthErr != nil {
		log.Printf("runtime: synthesize super failure update for run %s: %v", rec.RunID, synthErr)
	}
	if synthErr := rt.synthesizeResearcherUpdateOnFailure(persistCtx, rec, err); synthErr != nil {
		log.Printf("runtime: synthesize researcher update for run %s: %v", rec.RunID, synthErr)
	}

	// If this is an vtext agent revision task, mark the mutation as failed
	// and emit the vtext-specific failure event.
	if taskType, _ := rec.Metadata["type"].(string); taskType == "vtext_agent_revision" {
		_ = rt.store.FailAgentMutation(persistCtx, rec.RunID)
		if docID, _ := rec.Metadata["doc_id"].(string); docID != "" {
			failPayload, _ := json.Marshal(map[string]string{
				"doc_id":  docID,
				"loop_id": rec.RunID,
				"error":   err.Error(),
			})
			rt.emitVTextAgentEvent(persistCtx, rec, types.EventVTextAgentRevisionFailed,
				events.CauseProviderFailure, failPayload)
		}
	}
	if state.Terminal() {
		rt.reconcileCompletedVTextRun(rec)
	}

	log.Printf("runtime: run %s → %s: %v", rec.RunID, state, err)

}

// providerResult returns fallback result text when a completed provider
// execution did not populate rec.Result directly. This text is run output only;
// vtext document revisions are materialized exclusively by edit_vtext.
func (rt *Runtime) providerResult() string {
	if sp, ok := rt.provider.(*StubProvider); ok {
		return sp.Result
	}
	return "Run completed."
}

const runMetadataTrajectoryID = "trajectory_id"

// ensureTrajectoryID guarantees that metadata carries a trajectory_id, falling
// back to parent metadata (or parent RunID) when inherited. The trajectory_id
// is the unit that spans prompt-bar → conductor → vtext → workers → further
// revisions; Trace groups workflows by it so the whole chain renders as one
// run.
func ensureTrajectoryID(metadata map[string]any, parent *types.RunRecord, selfRunID string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	if existing, _ := metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(existing) != "" {
		return metadata
	}
	if parent != nil {
		if parent.Metadata != nil {
			if inherited, _ := parent.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(inherited) != "" {
				metadata[runMetadataTrajectoryID] = inherited
				return metadata
			}
		}
		if strings.TrimSpace(parent.RunID) != "" {
			metadata[runMetadataTrajectoryID] = parent.RunID
			return metadata
		}
	}
	if strings.TrimSpace(selfRunID) != "" {
		metadata[runMetadataTrajectoryID] = selfRunID
	}
	return metadata
}

func trajectoryIDForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	if trajectoryID := strings.TrimSpace(rec.TrajectoryID); trajectoryID != "" {
		return trajectoryID
	}
	if rec.Metadata == nil {
		return ""
	}
	if inherited, _ := rec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(inherited) != "" {
		return strings.TrimSpace(inherited)
	}
	return ""
}

// emitEvent creates and persists an event record, then publishes it on the
// event bus for live streaming.
func (rt *Runtime) emitEvent(ctx context.Context, rec *types.RunRecord, kind types.EventKind, cause events.EventCause, payload json.RawMessage) {
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Payload:      payload,
	}

	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist event %s: %v", evRec.EventID, err)
	}

	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  cause,
	})
}

// persistEvent persists an event record without publishing it on the bus.
// Used for recovery events that may have occurred before subscribers connect.
func (rt *Runtime) persistEvent(ctx context.Context, rec *types.RunRecord, kind types.EventKind, payload json.RawMessage) error {
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Payload:      payload,
	}
	return rt.store.AppendEvent(ctx, evRec)
}

func agentResidencyKey(ownerID, agentID string) string {
	return strings.TrimSpace(ownerID) + "\x00" + strings.TrimSpace(agentID)
}

func (rt *Runtime) registerRunActivation(rec *types.RunRecord, cancel context.CancelFunc) {
	if rt == nil || rec == nil {
		return
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.running[rec.RunID] = cancel
	if key := agentResidencyKey(rec.OwnerID, rec.AgentID); key != "\x00" {
		rt.residentAgents[key] = rec.RunID
	}
}

func (rt *Runtime) removeRunningLocked(runID string) {
	delete(rt.running, runID)
	for key, residentRunID := range rt.residentAgents {
		if residentRunID == runID {
			delete(rt.residentAgents, key)
		}
	}
}

func (rt *Runtime) residentRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, nil
	}
	key := agentResidencyKey(ownerID, agentID)
	if key == "\x00" {
		return types.RunRecord{}, false, nil
	}
	rt.mu.Lock()
	runID := rt.residentAgents[key]
	rt.mu.Unlock()
	if strings.TrimSpace(runID) == "" {
		return types.RunRecord{}, false, nil
	}
	rec, err := rt.store.GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			rt.removeRunning(runID)
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	if strings.TrimSpace(rec.OwnerID) != strings.TrimSpace(ownerID) ||
		strings.TrimSpace(rec.AgentID) != strings.TrimSpace(agentID) ||
		!rec.State.Active() {
		rt.removeRunning(runID)
		return types.RunRecord{}, false, nil
	}
	return rec, true, nil
}

// removeRunning removes a run from the running and resident-agent maps.
func (rt *Runtime) removeRunning(runID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.removeRunningLocked(runID)
}
