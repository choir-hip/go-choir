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

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/promptstore"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/workitem"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/qdrant"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/trace"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/wirepublish"
)

// Runtime is the core runtime engine that manages run lifecycle, event
// emission, and health state. It persists all state through
// the store so that run handles and events survive sandbox process restarts
// (VAL-RUNTIME-010).
type Runtime struct {
	cfg         provideriface.Config
	store       *store.Store
	bus         *events.EventBus
	provider    provideriface.Provider
	promptStore *promptstore.Store

	// traceStore is the optional Dolt-backed observability store. When set,
	// every event emitted via emitEvent/persistEvent/persistSubmittedRun is
	// projected into the canonical trace schema (additive; existing event
	// recording and bus publishing are unchanged). Failures are logged and
	// never propagated so a Dolt outage degrades gracefully.
	traceStore trace.Store

	runningMu sync.Mutex
	running   map[string]context.CancelFunc // loop_id → cancel function
	healthMu  sync.Mutex
	health    types.RuntimeHealthState
	// internalIngestionSubmissionMu serializes the durable idempotency lookup
	// and creation of typed ingestion runs submitted through the internal runtime
	// API. It also owns the processor overload check. Without one critical
	// section, concurrent retries can both miss the persisted handoff identity
	// and activate duplicate runs.
	internalIngestionSubmissionMu sync.Mutex

	wg           sync.WaitGroup
	toolRegistry *toolregistry.ToolRegistry
	toolProfiles map[string]*toolregistry.ToolRegistry

	textureWakeAfter func(time.Duration, func()) textureWakeTimer

	wirePublishDebounceMu sync.Mutex
	wirePublishDebouncer  *wirePublishDebouncer
	wirePublishTimer      textureWakeTimer
	wirePlatformPublisher func(context.Context, types.Document, types.Revision, *types.RunRecord) (*wirepublish.PublishTextureResponse, error)
	textureEditMu         sync.Mutex
	browserOpMu           sync.Mutex
	browserOps            map[string]*sync.Mutex
	browserCDPMu          sync.Mutex
	browserCDP            map[string]*browserCDPSession
	modelPolicyMu         sync.Mutex
	modelPolicies         map[string]ModelPolicy
	qdrantPipelineMu      sync.Mutex
	qdrantPipeline        *qdrant.Pipeline
	qdrantPipelineInitErr error

	// dispatchActor is the function hook that the actor runtime adapter
	// sets. When the business logic needs to start a run or wake an agent,
	// it calls this function. If nil, activate() panics — there is no
	// fallback path. The actor runtime is the only execution substrate.
	dispatchActor func(ctx context.Context, toAgentID, kind, content, trajectoryID, fromAgentID string) error

	// promotionAdapter is the optional Dolt promotion adapter. When set,
	// the promotion runtime calls Fork/Promote/Rollback to create
	// tamper-evident DOLT_TAG certificates and DOLT_RESET rollback. When
	// nil, the promotion flow works exactly as before (no Dolt tags).
	// This is the safety net for the Dolt promotion integration.
	promotionAdapter *computerversion.DoltPromotionAdapter
}

type textureWakeTimer interface {
	Stop() bool
}

// New creates a new Runtime with the given config, store, event bus, and
// provider. The runtime is idle until Start is called.
// If a tool registry is provided, the runtime will use the tool-calling
// loop for run execution instead of the simple provider bridge path.
func New(cfg provideriface.Config, s *store.Store, bus *events.EventBus, provider provideriface.Provider, opts ...RuntimeOption) *Runtime {
	cfg = provideriface.NormalizeConfig(cfg)
	rt := &Runtime{
		cfg:              cfg,
		store:            s,
		bus:              bus,
		provider:         provider,
		health:           types.HealthReady,
		running:          make(map[string]context.CancelFunc),
		promptStore:      promptstore.New(cfg.PromptRoot),
		textureWakeAfter: func(d time.Duration, fn func()) textureWakeTimer { return time.AfterFunc(d, fn) },
		browserOps:       make(map[string]*sync.Mutex),
		browserCDP:       make(map[string]*browserCDPSession),
		modelPolicies:    make(map[string]ModelPolicy),
	}
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// SetDispatchActor sets the function hook that dispatches actor messages.
// The actor runtime adapter calls this during construction. When set,
// activate() sends actor messages through this function. If not set,
// activate() panics — there is no fallback path.
func (rt *Runtime) SetDispatchActor(fn func(ctx context.Context, toAgentID, kind, content, trajectoryID, fromAgentID string) error) {
	rt.dispatchActor = fn
}

// DispatchActorActive reports whether the actor dispatch hook is set.
func (rt *Runtime) DispatchActorActive() bool {
	return rt.dispatchActor != nil
}

// activate starts execution of a run by dispatching an "initial_dispatch"
// actor message to the run's agent. The actor handler will call
// ExecuteActivationSync in the actor goroutine. There is no fallback —
// if dispatchActor is nil, activate panics.
func (rt *Runtime) activate(rec *types.RunRecord) {
	if rt.dispatchActor == nil {
		panic("runtime: activate called without dispatchActor set — actor runtime is required")
	}
	agentID := strings.TrimSpace(rec.AgentID)
	if agentID == "" {
		panic("runtime: activate called with empty AgentID")
	}
	trajectoryID := metadataStringValue(rec.Metadata, runMetadataTrajectoryID)
	if err := rt.dispatchActor(context.Background(), agentID, "initial_dispatch", rec.RunID, trajectoryID, ""); err != nil {
		log.Printf("runtime: activate dispatch for run %s: %v", rec.RunID, err)
	}
}

// ExecuteActivationSync runs executeActivation in the caller's goroutine. It
// is the actor-handler entry point: the caller's goroutine (the actor
// goroutine) IS the run goroutine. The rec is updated in place to reflect
// the final run state (RunCompleted, RunFailed, or RunPassivated).
//
// This is the synchronous replacement for startRunAsync: no goroutine is
// spawned, no channel is waited on. The actor runtime manages the goroutine
// lifecycle.
func (rt *Runtime) ExecuteActivationSync(ctx context.Context, rec *types.RunRecord) {
	runRec := *rec
	runCtx, cancel := context.WithTimeout(ctx, rt.cfg.ActivationBudget)

	rt.runningMu.Lock()
	stored, err := rt.store.GetRun(context.Background(), rec.RunID)
	if err == nil && stored.State.Terminal() {
		rt.runningMu.Unlock()
		cancel()
		*rec = stored
		return
	}
	rt.running[rec.RunID] = cancel
	rt.runningMu.Unlock()

	stopProgressDeadline := context.AfterFunc(runCtx, func() {
		if !errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return
		}
		if err := rt.terminalizeRun(
			context.Background(),
			rec.RunID,
			rec.OwnerID,
			"activation budget exceeded: progress deadline reached",
		); err != nil && !strings.Contains(err.Error(), "cannot cancel") {
			log.Printf("runtime: progress deadline for run %s: %v", rec.RunID, err)
		}
	})
	defer stopProgressDeadline()
	defer cancel()

	rt.wg.Add(1)
	rt.executeActivation(runCtx, &runRec)
	*rec = runRec
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
	case agentprofile.Conductor:
		if ownerID != "" {
			return "conductor:" + ownerID
		}
	case agentprofile.Super:
		if ownerID != "" {
			return persistentSuperAgentID(ownerID)
		}
	case agentprofile.Texture:
		if docID := metadataStringValue(metadata, "doc_id"); docID != "" {
			return currentTextureAgentID(docID)
		}
	case agentprofile.Processor:
		if key := metadataStringValue(metadata, runMetadataProcessorKey); key != "" {
			return "processor:" + safeRefPart(key)
		}
	case agentprofile.Reconciler:
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
	if profile == agentprofile.Texture {
		if docID := metadataStringValue(metadata, "doc_id"); docID != "" {
			return docID
		}
	}
	if profile == agentprofile.Super || profile == agentprofile.Processor || profile == agentprofile.Reconciler {
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
		Profile:   agentprofile.Super,
		Role:      agentprofile.Super,
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
	rawProfile := metadataStringValue(metadata, runMetadataAgentProfile)
	profile := rawProfile
	if profile == "" {
		if parent != nil && strings.TrimSpace(parent.AgentProfile) != "" && !isTextureAgentRevisionTaskType(metadataStringValue(metadata, "type")) {
			profile = parent.AgentProfile
		} else {
			profile = agentProfileForRun(&types.RunRecord{Metadata: metadata})
		}
	}
	profile = agentprofile.Canonical(profile)
	if strings.EqualFold(strings.TrimSpace(rawProfile), agentprofile.Texture) {
		profile = agentprofile.Texture
	}
	rawRole := metadataStringValue(metadata, runMetadataAgentRole)
	role := rawRole
	if role == "" {
		role = profile
	} else {
		role = agentprofile.Canonical(role)
	}
	if strings.EqualFold(strings.TrimSpace(rawRole), agentprofile.Texture) {
		role = agentprofile.Texture
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

func (rt *Runtime) PromptStore() *promptstore.Store {
	return rt.promptStore
}

// RuntimeOption configures optional Runtime components.
type RuntimeOption func(*Runtime)

// WithTraceStore mounts a Dolt-backed trace observability store into the
// runtime. When set, every emitted event is projected (via trace.FromEventRecord)
// and appended to the store in addition to the existing event recording and bus
// publishing. Append failures are logged and never propagated, so a Dolt outage
// degrades gracefully without changing request handling. The runtime closes the
// store on Stop when it owns the connection.
func WithTraceStore(s trace.Store) RuntimeOption {
	return func(rt *Runtime) {
		rt.traceStore = s
	}
}

// WithPromotionAdapter mounts a Dolt promotion adapter into the runtime.
// When set, the promotion runtime calls Fork/Promote/Rollback to create
// tamper-evident DOLT_TAG certificates and DOLT_RESET rollback. When nil,
// the promotion flow works exactly as before (no Dolt tags).
func WithPromotionAdapter(adapter *computerversion.DoltPromotionAdapter) RuntimeOption {
	return func(rt *Runtime) {
		rt.promotionAdapter = adapter
	}
}

func withTextureWakeAfterFuncForTest(after func(time.Duration, func()) textureWakeTimer) RuntimeOption {
	return func(rt *Runtime) {
		if after != nil {
			rt.textureWakeAfter = after
		}
	}
}

// Start begins runtime boot recovery. On boot, no actors are resident; previous
// in-process activations are marked passivated, then durable update backlog and
// assigned open trajectory work are swept to re-warm cold actors.
func (rt *Runtime) Start(ctx context.Context) {
	rt.passivateInterruptedActivations(ctx)
	rt.recoverOpenWirePublicationClaims(ctx)
	rt.sweepPassivatedSpawnedCoagentWork(ctx)
	rt.sweepPendingUpdateActors(ctx)
	rt.sweepOpenWorkItemActors(ctx)
	rt.reconcileAllTextureDocuments(ctx)
	// Best-effort: ensure the production Qdrant collection exists so the
	// semantic dedup pass on ingestion has a target. Runs asynchronously so
	// a slow or unreachable Qdrant cannot block runtime startup; the dedup
	// path also ensures the collection lazily on first use.
	go rt.ensureProductionQdrantCollectionBestEffort(ctx)
	go func() {
		<-ctx.Done()
		rt.closeAllBrowserCDPSessions()
	}()
	log.Printf("runtime: started (sandbox=%s)", rt.cfg.SandboxID)
}

// ensureProductionQdrantCollectionBestEffort attempts to create the production
// Qdrant collection with a short timeout. Failures are logged but never
// propagated: the ingestion dedup path falls back to pass-through when the
// collection is missing.
func (rt *Runtime) ensureProductionQdrantCollectionBestEffort(ctx context.Context) {
	if rt == nil || rt.QdrantPipeline() == nil {
		return
	}
	ensureCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := rt.EnsureProductionQdrantCollection(ensureCtx); err != nil {
		log.Printf("runtime: ensure production qdrant collection (best-effort): %v", err)
	}
}

// Stop gracefully shuts down the runtime, cancelling all in-flight runs.
// It is safe to call Stop multiple times.
func (rt *Runtime) Stop() {
	rt.closeAllBrowserCDPSessions()
	rt.runningMu.Lock()
	for runID, cancel := range rt.running {
		cancel()
		delete(rt.running, runID)
	}
	rt.runningMu.Unlock()

	rt.wg.Wait()

	// Close the trace observability store when the runtime owns it (e.g. the
	// SQLite test backend). The Dolt-backed production store does not own its
	// *sql.DB and Close is a no-op there; the caller manages the DB lifecycle.
	if rt.traceStore != nil {
		if err := rt.traceStore.Close(); err != nil {
			log.Printf("runtime: close trace store: %v", err)
		}
	}

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
// used to carry feature-specific context (e.g., texture agent revision info).
func shouldLogWireLifecycle(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	profile := agentprofile.Canonical(agentProfileForRun(rec))
	if profile == agentprofile.Processor || profile == agentprofile.Texture || profile == agentprofile.Researcher || profile == agentprofile.CoSuper {
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
	return fmt.Sprintf("run=%s profile=%s requested_by=%s channel=%s processor_key=%s state=%s", rec.RunID, agentprofile.Canonical(agentProfileForRun(rec)), strings.TrimSpace(rec.RequestedByRunID), strings.TrimSpace(rec.ChannelID), metadataStringValue(rec.Metadata, runMetadataProcessorKey), rec.State)
}

func (rt *Runtime) StartRunWithMetadata(ctx context.Context, prompt, ownerID string, metadata map[string]any) (*types.RunRecord, error) {
	rec, err := rt.createRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if err := rt.recordExplicitInitialTextureDecisionIfNeeded(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return nil, err
	}
	rt.activate(rec)
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
		RunID:            runID,
		AgentID:          agentRec.AgentID,
		ChannelID:        agentRec.ChannelID,
		RequestedByRunID: strings.TrimSpace(metadataStringValue(metadata, "requested_by_run_id")),
		AgentProfile:     agentRec.Profile,
		AgentRole:        agentRec.Role,
		OwnerID:          ownerID,
		SandboxID:        rt.cfg.SandboxID,
		State:            types.RunPending,
		Prompt:           prompt,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata:         metadata,
	}
	rt.stampAndMintTrajectory(ctx, rec)

	if err := persistSubmittedRun(ctx, rt.store, rt.bus, agentRec, rec, len(prompt), rt.traceStore); err != nil {
		return nil, err
	}
	if agentprofile.Canonical(agentProfileForRun(rec)) == agentprofile.Processor {
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

// StartCoagentRun creates a coagent run and records the requesting run as
// provenance. It validates that the requesting run exists, creates a runtime
// record, and begins execution in a goroutine. This is not parent/child run
// control: the new run is not owned, awaited, or cancelled by the requester;
// lifecycle stays trajectory/work-item scoped and coordination is via addressed
// channel updates and requester provenance.
//
// The coagent run inherits the owner from the ownerID parameter (derived from
// auth context). Constraints are stored in the run metadata for use during
// execution.
func (rt *Runtime) StartCoagentRun(ctx context.Context, requesterRunID, objective, ownerID string, constraints map[string]any) (*types.RunRecord, error) {
	// Validate that the requesting run exists.
	requesterRec, err := rt.store.GetRun(ctx, requesterRunID)
	if err != nil {
		if err == store.ErrNotFound {
			return nil, fmt.Errorf("requester run not found: %s", requesterRunID)
		}
		return nil, fmt.Errorf("lookup requester run: %w", err)
	}

	runID := uuid.New().String()

	// Build metadata from constraints and requester provenance.
	metadata := map[string]any{
		"spawned_by":   ownerID,
		"requested_by": requesterRunID,
	}
	for k, v := range constraints {
		metadata[k] = v
	}
	inheritWorkerRepoMetadata(metadata, &requesterRec)
	// A pinned model-policy overlay (e.g. an eval arm) covers the whole
	// trajectory: a child coagent inherits the requester's overlay when it does
	// not specify its own, so a Texture arm also pins the researchers it spawns.
	if strings.TrimSpace(metadataStringValue(metadata, runMetadataLLMPolicyOverlayID)) == "" {
		if overlayID := strings.TrimSpace(metadataStringValue(requesterRec.Metadata, runMetadataLLMPolicyOverlayID)); overlayID != "" {
			metadata[runMetadataLLMPolicyOverlayID] = overlayID
		}
	}
	if slot := normalizeVSuperCoSuperSlot(metadataStringValue(metadata, runMetadataCoSuperSlot)); slot != "" {
		metadata[runMetadataCoSuperSlot] = slot
	}
	metadata = ensureTrajectoryID(metadata, &requesterRec, runID)

	if rt.coagentSpawnBudgetApplies(&requesterRec) {
		coagentProfile := agentprofile.Canonical(metadataStringValue(metadata, runMetadataAgentProfile))
		if coagentProfile == "" {
			coagentProfile = agentprofile.Canonical(metadataStringValue(metadata, runMetadataAgentRole))
		}
		slot := normalizeVSuperCoSuperSlot(metadataStringValue(metadata, runMetadataCoSuperSlot))
		if strings.TrimSpace(metadataStringValue(metadata, runMetadataCoSuperSlot)) != "" && slot == "" && coagentProfile == agentprofile.CoSuper {
			return nil, fmt.Errorf("vsuper co-super coagent requires co_super_slot to be implementation or verifier")
		}
		if coagentProfile == agentprofile.CoSuper && slot == "" {
			return nil, fmt.Errorf("vsuper co-super coagent requires co_super_slot=\"implementation\" or co_super_slot=\"verifier\"")
		}
		if slot != "" && coagentProfile == agentprofile.CoSuper {
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
		if err := rt.enforceCoSuperSlotBudget(ctx, &requesterRec); err != nil {
			return nil, err
		}
		if slot == "verifier" && coagentProfile == agentprofile.CoSuper {
			if err := rt.enforceVSuperVerifierSequencing(ctx, &requesterRec); err != nil {
				return nil, err
			}
		}
	}

	now := time.Now().UTC()
	metadata = ensureDesktopID(metadata, &requesterRec, metadataStringValue(metadata, runMetadataDesktopID))
	metadata = inheritTextureRequesterMetadata(metadata, &requesterRec)
	agentRec, metadata := resolveRunIdentity(ownerID, rt.cfg.SandboxID, metadata, &requesterRec)
	metadata = ensureTrajectoryID(metadata, &requesterRec, runID)
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	claimedCoSuperSlot := false
	claimedCoSuperTrajectoryID := ""
	claimedCoSuperSlotName := ""
	if slot := normalizeVSuperCoSuperSlot(metadataStringValue(metadata, runMetadataCoSuperSlot)); slot != "" &&
		agentprofile.Canonical(metadataStringValue(metadata, runMetadataAgentProfile)) == agentprofile.CoSuper &&
		rt.coagentSpawnBudgetApplies(&requesterRec) {
		trajectoryID := metadataStringValue(metadata, runMetadataTrajectoryID)
		existing, claimed, err := rt.store.ClaimCoSuperSlot(ctx, ownerID, trajectoryID, slot, runID, agentRec.AgentID, requesterRunID)
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
		return nil, releaseCoSuperSlotClaim(fmt.Errorf("persist coagent agent: %w", err))
	}

	// Create the runtime run record.
	rec := &types.RunRecord{
		RunID:            runID,
		AgentID:          agentRec.AgentID,
		ChannelID:        agentRec.ChannelID,
		RequestedByRunID: requesterRunID,
		AgentProfile:     agentRec.Profile,
		AgentRole:        agentRec.Role,
		OwnerID:          ownerID,
		SandboxID:        rt.cfg.SandboxID,
		State:            types.RunPending,
		Prompt:           objective,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata:         metadata,
	}
	rt.stampAndMintTrajectory(ctx, rec)
	if item, err := rt.ensureSpawnedCoagentWorkItem(ctx, rec, &requesterRec, "spawned_work_item_id"); err != nil {
		return nil, releaseCoSuperSlotClaim(fmt.Errorf("persist spawned coagent work item: %w", err))
	} else if item.WorkItemID == "" && spawnedCoagentWorkItemProfile(agentProfileForRun(rec)) {
		log.Printf("runtime: spawned coagent work item not created for run=%s profile=%s trajectory=%s agent=%s requested_by=%s",
			rec.RunID, agentprofile.Canonical(agentProfileForRun(rec)), trajectoryIDForRun(rec), rec.AgentID, rec.RequestedByRunID)
	}

	if err := rt.store.CreateRun(ctx, *rec); err != nil {
		return nil, releaseCoSuperSlotClaim(fmt.Errorf("persist coagent run: %w", err))
	}
	rt.createAgentMutationForRun(ctx, rec)

	// Emit submitted event.
	objectiveLenPayload, _ := json.Marshal(map[string]any{
		"prompt_length": len(objective),
		"requested_by":  requesterRunID,
	})
	rt.emitEvent(ctx, rec, types.EventRunSubmitted, events.CauseTaskLifecycle, objectiveLenPayload)
	if shouldLogWireLifecycle(rec) || shouldLogWireLifecycle(&requesterRec) {
		log.Printf("runtime: started coagent %s requested by %s requester_profile=%s", wireLifecycleSummary(rec), requesterRec.RunID, agentprofile.Canonical(agentProfileForRun(&requesterRec)))
	}
	if err := rt.recordExplicitInitialTextureDecisionIfNeeded(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return nil, releaseCoSuperSlotClaim(err)
	}

	// Dispatch via actor runtime.
	rt.activate(rec)

	log.Printf("runtime: started coagent run %s requested by %s (owner=%s)", rec.RunID, requesterRunID, ownerID)

	return rec, nil
}

func (rt *Runtime) createSpawnedCoagentWorkItem(ctx context.Context, rec *types.RunRecord, requester *types.RunRecord) (types.WorkItemRecord, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return types.WorkItemRecord{}, nil
	}
	profile := agentprofile.Canonical(agentProfileForRun(rec))
	if !spawnedCoagentWorkItemProfile(profile) {
		return types.WorkItemRecord{}, nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	agentID := strings.TrimSpace(rec.AgentID)
	objective := strings.TrimSpace(rec.Prompt)
	if ownerID == "" || trajectoryID == "" || agentID == "" || objective == "" {
		return types.WorkItemRecord{}, nil
	}
	requesterRunID := strings.TrimSpace(rec.RequestedByRunID)
	if requesterRunID == "" {
		requesterRunID = metadataStringValue(rec.Metadata, "requested_by")
	}
	if requesterRunID == "" {
		return types.WorkItemRecord{}, nil
	}
	if requester == nil {
		if loaded, err := rt.store.GetRun(ctx, requesterRunID); err == nil && loaded.OwnerID == ownerID {
			requester = &loaded
			rec.Metadata = inheritTextureRequesterMetadata(rec.Metadata, requester)
		}
	}
	details := map[string]any{
		"kind":                "spawned_coagent_run",
		"spawned_run_id":      rec.RunID,
		"requested_by_run_id": requesterRunID,
		"agent_profile":       profile,
		"agent_role":          agentRoleForRun(rec),
	}
	if channelID := strings.TrimSpace(rec.ChannelID); channelID != "" {
		details["channel_id"] = channelID
	}
	if requester != nil {
		if requesterProfile := agentprofile.Canonical(agentProfileForRun(requester)); requesterProfile != "" {
			details["requested_by_agent_profile"] = requesterProfile
		}
	}
	copyMetadataStringToDetails(rec.Metadata, details, "requested_by_profile")
	copyMetadataStringToDetails(rec.Metadata, details, "requested_by_agent_id")
	copyMetadataStringToDetails(rec.Metadata, details, "requested_by_run_id")
	return rt.store.CreateWorkItem(ctx, types.WorkItemRecord{
		OwnerID:              ownerID,
		TrajectoryID:         trajectoryID,
		Objective:            objective,
		Reason:               "spawn_agent coagent objective",
		AuthorityProfile:     profile,
		AssignedAgentID:      agentID,
		CreatedByRunID:       requesterRunID,
		ObjectiveFingerprint: "spawned_coagent:" + workitem.ObjectiveFingerprint(ownerID, trajectoryID, rec.RunID, objective),
		Details:              details,
	})
}

func inheritTextureRequesterMetadata(metadata map[string]any, requesterRun *types.RunRecord) map[string]any {
	if requesterRun == nil || agentprofile.Canonical(agentProfileForRun(requesterRun)) != agentprofile.Texture {
		return metadata
	}
	metadata = cloneMetadata(metadata)
	if metadataStringValue(metadata, "requested_by_profile") == "" {
		metadata["requested_by_profile"] = agentprofile.Texture
	}
	if metadataStringValue(metadata, "requested_by_agent_id") == "" {
		metadata["requested_by_agent_id"] = agentIDForRun(requesterRun)
	}
	if metadataStringValue(metadata, "requested_by_run_id") == "" {
		metadata["requested_by_run_id"] = requesterRun.RunID
	}
	return metadata
}

func copyMetadataStringToDetails(metadata map[string]any, details map[string]any, key string) {
	if details == nil {
		return
	}
	if value := metadataStringValue(metadata, key); value != "" {
		details[key] = value
	}
}

func inheritRequesterMetadataFromWorkItem(ctx context.Context, s *store.Store, ownerID string, metadata map[string]any, item types.WorkItemRecord) map[string]any {
	metadata = cloneMetadata(metadata)
	for _, key := range []string{"requested_by_profile", "requested_by_agent_id", "requested_by_run_id"} {
		if metadataStringValue(metadata, key) != "" {
			continue
		}
		if value := metadataStringValue(item.Details, key); value != "" {
			metadata[key] = value
		}
	}
	if metadataStringValue(metadata, "requested_by_profile") != "" && metadataStringValue(metadata, "requested_by_agent_id") != "" {
		return metadata
	}
	requesterRunID := strings.TrimSpace(firstNonEmpty(item.CreatedByRunID, metadataStringValue(item.Details, "requested_by_run_id")))
	if s == nil || requesterRunID == "" {
		return metadata
	}
	requesterRun, err := s.GetRun(ctx, requesterRunID)
	if err != nil || requesterRun.OwnerID != ownerID {
		return metadata
	}
	return inheritTextureRequesterMetadata(metadata, &requesterRun)
}

func (rt *Runtime) ensureSpawnedCoagentWorkItem(ctx context.Context, rec *types.RunRecord, parent *types.RunRecord, metadataKey string) (types.WorkItemRecord, error) {
	item, err := rt.createSpawnedCoagentWorkItem(ctx, rec, parent)
	if err != nil || item.WorkItemID == "" || rec == nil {
		return item, err
	}
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata["work_item_ids"] = appendUniqueString(metadataStringSlice(rec.Metadata["work_item_ids"]), item.WorkItemID)
	if strings.TrimSpace(metadataKey) != "" {
		rec.Metadata[metadataKey] = item.WorkItemID
	}
	return item, nil
}

func spawnedCoagentWorkItemProfile(profile string) bool {
	switch agentprofile.Canonical(profile) {
	case agentprofile.Researcher, agentprofile.Super, agentprofile.VSuper, agentprofile.CoSuper:
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

func (rt *Runtime) coagentSpawnBudgetApplies(requesterRec *types.RunRecord) bool {
	if requesterRec == nil {
		return false
	}
	return agentprofile.Canonical(agentProfileForRun(requesterRec)) == agentprofile.VSuper
}

func (rt *Runtime) enforceCoSuperSlotBudget(ctx context.Context, requesterRec *types.RunRecord) error {
	if rt == nil || rt.store == nil || requesterRec == nil {
		return nil
	}
	trajectoryID := trajectoryIDForRun(requesterRec)
	active, err := rt.store.CountActiveCoSuperSlots(ctx, requesterRec.OwnerID, trajectoryID)
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

func (rt *Runtime) enforceVSuperVerifierSequencing(ctx context.Context, requesterRec *types.RunRecord) error {
	if rt == nil || rt.store == nil || requesterRec == nil {
		return nil
	}
	trajectoryID := trajectoryIDForRun(requesterRec)
	impl, found, err := rt.store.CoSuperSlotRun(ctx, requesterRec.OwnerID, trajectoryID, "implementation")
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

func (rt *Runtime) latestTrajectoryCoSuperAppChangePackage(ctx context.Context, requesterRec *types.RunRecord) (map[string]any, bool, error) {
	if rt == nil || rt.store == nil {
		return nil, false, nil
	}
	if requesterRec == nil {
		return nil, false, nil
	}
	trajectoryID := trajectoryIDForRun(requesterRec)
	if trajectoryID == "" || strings.TrimSpace(requesterRec.OwnerID) == "" {
		return nil, false, nil
	}
	child, found, err := rt.store.CoSuperSlotRun(ctx, requesterRec.OwnerID, trajectoryID, "implementation")
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
	if !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		return
	}
	mutation := agentMutationForRun(rec)
	if mutation == nil {
		log.Printf("runtime: texture agent revision run %s: missing doc_id for mutation", rec.RunID)
		return
	}
	if err := rt.store.CreateAgentMutation(ctx, *mutation); err != nil {
		log.Printf("runtime: texture agent revision run %s: create mutation: %v", rec.RunID, err)
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
	return rt.terminalizeRun(ctx, runID, ownerID, "run cancelled")
}

// terminalizeRun persists cancellation before releasing admission or signalling
// the resident activation. The shared lifecycle lock orders this transition
// against activation state writes, so a late provider return cannot replace it.
func (rt *Runtime) terminalizeRun(ctx context.Context, runID, ownerID, reason string) error {
	rt.runningMu.Lock()
	rec, err := rt.store.GetRun(ctx, runID)
	if err != nil {
		rt.runningMu.Unlock()
		if err == store.ErrNotFound {
			return fmt.Errorf("run not found: %s", runID)
		}
		return fmt.Errorf("lookup run: %w", err)
	}
	if rec.OwnerID != ownerID {
		rt.runningMu.Unlock()
		return store.ErrNotFound
	}
	if rec.State.Terminal() {
		rt.runningMu.Unlock()
		return fmt.Errorf("cannot cancel run in %s state", rec.State)
	}

	now := time.Now().UTC()
	rec.State = types.RunCancelled
	rec.Error = reason
	rec.UpdatedAt = now
	rec.FinishedAt = &now
	if err := rt.store.UpdateRun(ctx, rec); err != nil {
		rt.runningMu.Unlock()
		return fmt.Errorf("update cancelled run: %w", err)
	}
	cancel := rt.running[runID]
	delete(rt.running, runID)
	rt.runningMu.Unlock()

	if cancel != nil {
		cancel()
	}
	errPayload, _ := json.Marshal(map[string]string{"error": reason})
	rt.emitEvent(context.Background(), &rec, types.EventRunCancelled, events.CauseTaskLifecycle, errPayload)
	return nil
}

// persistActivationState serializes activation writes with cancellation and
// progress-deadline terminalization. A stored terminal state always wins.
func (rt *Runtime) persistActivationState(ctx context.Context, rec *types.RunRecord) (bool, error) {
	rt.runningMu.Lock()
	defer rt.runningMu.Unlock()

	stored, err := rt.store.GetRun(context.Background(), rec.RunID)
	if err != nil {
		return false, err
	}
	if stored.State.Terminal() {
		*rec = stored
		return false, nil
	}
	if err := rt.updateRunAndMarkSuccessfulCoagentActivationDelivered(ctx, rec); err != nil {
		return false, err
	}
	return true, nil
}

// CancelAgent cancels the most recent non-terminal run owned by the given agent.
func (rt *Runtime) CancelAgent(ctx context.Context, agentID, ownerID string) error {
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
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

const trajectoryActivationDrainTimeout = 30 * time.Second

// cancelTrajectoryAuthority delegates the durable cancellation transition to
// the store, which atomically closes open obligations and terminalizes live
// trajectories. The returned record is the authoritative durable state.
func (rt *Runtime) cancelTrajectoryAuthority(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	if rt == nil || rt.store == nil {
		return types.TrajectoryRecord{}, fmt.Errorf("cancel trajectory: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return types.TrajectoryRecord{}, fmt.Errorf("cancel trajectory: owner_id and trajectory_id are required")
	}
	return rt.store.CancelTrajectoryAuthority(ctx, ownerID, trajectoryID)
}

// CancelTrajectory cancels an owner-scoped trajectory and then terminates all
// of its active run activations. Durable trajectory state becomes terminal
// before any activation is signalled. A settled trajectory is reported
// unchanged and its activations are not cancelled.
func (rt *Runtime) CancelTrajectory(ctx context.Context, trajectoryID, ownerID string) (types.TrajectoryRecord, []string, error) {
	trajectory, err := rt.cancelTrajectoryAuthority(ctx, ownerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, nil, err
	}
	if trajectory.Status != types.TrajectoryCancelled {
		return trajectory, nil, nil
	}

	cancelled, err := rt.drainCancelledTrajectoryActivations(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID))
	return trajectory, cancelled, err
}

func (rt *Runtime) drainCancelledTrajectoryActivations(ctx context.Context, ownerID, trajectoryID string) ([]string, error) {
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), trajectoryActivationDrainTimeout)
	defer cancel()

	cancelled := []string{}
	active, err := rt.store.ListActiveRunsByTrajectory(drainCtx, ownerID, trajectoryID, 0)
	if err != nil {
		return cancelled, fmt.Errorf("list active trajectory activations: %w", err)
	}
	for _, run := range active {
		if err := rt.CancelRun(drainCtx, run.RunID, ownerID); err != nil {
			if strings.Contains(err.Error(), "cannot cancel run in") {
				continue
			}
			return cancelled, err
		}
		cancelled = append(cancelled, run.RunID)
	}
	return cancelled, nil
}

// CancelRunTrajectory derives the trajectory that contains runID, persists
// metadata-only identity, and delegates to CancelTrajectory.
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
	_, cancelled, err := rt.CancelTrajectory(ctx, trajectoryID, ownerID)
	return cancelled, err
}

// ListRunsByOwner returns recent runs for the given owner, ordered by
// creation time descending.
func (rt *Runtime) ListRunsByOwner(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	return rt.store.ListRunsByOwner(ctx, ownerID, limit)
}

// HealthState returns the current runtime health state.
func (rt *Runtime) HealthState() types.RuntimeHealthState {
	rt.healthMu.Lock()
	defer rt.healthMu.Unlock()
	return rt.health
}

// SetHealth updates the runtime health state. If the state changes, it emits
// a health or degraded event to make the transition externally visible
// (VAL-RUNTIME-001, VAL-RUNTIME-009).
func (rt *Runtime) SetHealth(state types.RuntimeHealthState) {
	rt.healthMu.Lock()
	prev := rt.health
	rt.health = state
	rt.healthMu.Unlock()

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
	rt.runningMu.Lock()
	defer rt.runningMu.Unlock()
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
	profile = agentprofile.Canonical(profile)
	count := 0
	for i := range runs {
		if agentprofile.Canonical(runs[i].AgentProfile) != profile {
			continue
		}
		if profile == agentprofile.Processor && !rt.processorRunOccupiesAdmission(ctx, runs[i]) {
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
	item, found, err := rt.store.FindWorkItemByFingerprint(ctx, ownerID, trajectoryID, workitem.ProcessorDecisionFingerprint(trajectoryID))
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
				if item, err := rt.ensureSpawnedCoagentWorkItem(ctx, rec, nil, "passivated_spawned_work_item_id"); err != nil {
					log.Printf("runtime: boot passivation: create spawned work item for run %s: %v", rec.RunID, err)
				} else if item.WorkItemID == "" && spawnedCoagentWorkItemProfile(agentProfileForRun(rec)) {
					log.Printf("runtime: boot passivation: spawned work item skipped for run=%s profile=%s trajectory=%s agent=%s requested_by=%s",
						rec.RunID, agentprofile.Canonical(agentProfileForRun(rec)), trajectoryIDForRun(rec), rec.AgentID, rec.RequestedByRunID)
				}

				if err := rt.store.UpdateRun(ctx, *rec); err != nil {
					log.Printf("runtime: boot passivation: update run %s: %v", rec.RunID, err)
					continue
				}
				progressed = true
				if isTextureAgentRevisionTaskType(metadataString(rec.Metadata, "type")) {
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
	updates, err := rt.store.ListCoagentMailboxBacklogAll(ctx, 1000)
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

func (rt *Runtime) sweepPassivatedSpawnedCoagentWork(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	runs, err := rt.store.ListRunsByState(ctx, types.RunPassivated, 1000)
	if err != nil {
		log.Printf("runtime: boot passivated spawned-work sweep: %v", err)
		return
	}
	for i := range runs {
		rec := &runs[i]
		item, err := rt.ensureSpawnedCoagentWorkItem(ctx, rec, nil, "passivated_spawned_work_item_id")
		if err != nil {
			log.Printf("runtime: boot passivated spawned-work sweep run=%s: %v", rec.RunID, err)
			continue
		}
		if item.WorkItemID == "" || item.Status != types.WorkItemOpen {
			continue
		}
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			log.Printf("runtime: boot passivated spawned-work annotate run=%s work_item=%s: %v", rec.RunID, item.WorkItemID, err)
		}
		if _, err := rt.reconcileAssignedWorkItemActor(ctx, []types.WorkItemRecord{item}); err != nil {
			log.Printf("runtime: boot passivated spawned-work rewarm run=%s work_item=%s: %v", rec.RunID, item.WorkItemID, err)
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
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
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
	profile := agentprofile.Canonical(firstNonEmpty(agent.Profile, first.AuthorityProfile))
	if profile == "" || profile == agentprofile.Email || profile == agentprofile.Conductor || profile == agentprofile.Texture {
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
	metadata = inheritRequesterMetadataFromWorkItem(ctx, rt.store, ownerID, metadata, first)
	rec, err := rt.createRunWithMetadata(ctx, buildAssignedWorkItemPrompt(workItems), ownerID, metadata)
	if err != nil {
		return nil, err
	}
	rt.activate(rec)
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
	defer func() {
		rt.runningMu.Lock()
		delete(rt.running, rec.RunID)
		rt.runningMu.Unlock()
		if rec.State == types.RunCompleted && !metadataBoolValue(rec.Metadata, "texture_revision_failed_no_write") {
			rt.reconcileCompletedTextureRun(rec)
		}
	}()

	now := time.Now().UTC()

	// Transition to running.
	rec.State = types.RunRunning
	rec.UpdatedAt = now
	persisted, err := rt.persistActivationState(ctx, rec)
	if err != nil {
		log.Printf("runtime: update run %s to running: %v", rec.RunID, err)
		rt.handleExecutionError(ctx, rec, fmt.Errorf("update run state: %w", err))
		return
	}
	if !persisted {
		return
	}

	rt.emitEvent(ctx, rec, types.EventRunStarted, events.CauseTaskLifecycle,
		json.RawMessage(`{}`))

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		cause := events.CauseProviderProgress
		if kind == types.EventToolInvoked || kind == types.EventToolResult {
			cause = events.CauseToolExecution
		}
		// Also emit texture-specific progress events for agent revision runs.
		if taskType, _ := rec.Metadata["type"].(string); isTextureAgentRevisionTaskType(taskType) {
			if docID, _ := rec.Metadata["doc_id"].(string); docID != "" {
				if kind == types.EventRunProgress {
					progressPayload, _ := json.Marshal(map[string]string{
						"doc_id":  docID,
						"loop_id": rec.RunID,
						"phase":   phase,
					})
					rt.emitTextureAgentEvent(ctx, rec, types.EventTextureAgentRevisionProgress,
						events.CauseProviderProgress, progressPayload)
				}
			}
		}
		rt.emitEvent(ctx, rec, kind, cause, payload)
	}

	registry := rt.toolRegistryForRun(rec)

	// Use the tool-calling loop if a tool registry is configured and the
	// provider supports the provideriface.ToolLoopProvider interface. Otherwise, fall back
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
func (rt *Runtime) executeWithToolLoop(ctx context.Context, rec *types.RunRecord, registry *toolregistry.ToolRegistry, emit provideriface.EventEmitFunc) {
	tlp := toolregistry.AsToolLoopProvider(rt.provider)

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
	ctx = toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(rec))
	reactivateExistingMemory := metadataBoolValue(rec.Metadata, "actor_reactivate_existing_memory")
	appendInitialMailboxTurns := shouldAppendInitialCoagentMailboxTurns(rec)
	if !reactivateExistingMemory && !appendInitialMailboxTurns {
		initialMessages, err = rt.prependInitialCoagentUpdatePackets(ctx, rec, initialMessages)
		if err != nil {
			rt.handleExecutionError(ctx, rec, fmt.Errorf("prepend coagent update packets: %w", err))
			return
		}
	}
	if err := rt.recordExplicitInitialTextureDecisionIfNeeded(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}
	llmConfig := provideriface.ResolvedLLMConfigFromMetadata(rec.Metadata)
	renderedSystemPrompt := systemPrompt
	if registry != nil {
		renderedSystemPrompt = toolregistry.BuildSystemPrompt(systemPrompt, registry)
	}
	memory := newRunMemoryManager(rt.store, rec, rt.cfg, emit).
		withLLMCompactor(tlp, llmConfig, estimateTextTokens(renderedSystemPrompt))
	initialMailboxPhase := ""
	if appendInitialMailboxTurns {
		initialMailboxPhase = coagentPacketDeliveryThread
	}
	injectUserTurns := rt.coagentUpdateTurnInjectorWithInitialPhase(rec, initialMailboxPhase)
	initialMessages, err = memory.initialize(ctx, initialMessages)
	if err != nil {
		rt.handleExecutionError(ctx, rec, fmt.Errorf("initialize run memory: %w", err))
		return
	}
	if appendInitialMailboxTurns && injectUserTurns != nil {
		injected, err := injectUserTurns(false)
		if err != nil {
			rt.handleExecutionError(ctx, rec, fmt.Errorf("inject initial mailbox turns for actor: %w", err))
			return
		}
		for _, msg := range injected {
			if err := memory.afterAppendMessage(ctx, "user", msg); err != nil {
				rt.handleExecutionError(ctx, rec, fmt.Errorf("persist initial mailbox turn: %w", err))
				return
			}
		}
		if len(injected) > 0 {
			initialMessages = append(initialMessages, injected...)
			rec.UpdatedAt = time.Now().UTC()
			if err := rt.store.UpdateRun(ctx, *rec); err != nil {
				rt.handleExecutionError(ctx, rec, fmt.Errorf("persist actor initial mailbox metadata: %w", err))
				return
			}
		}
	}
	maxOutputTokens := provideriface.MaxInteractiveOutputTokensForSelection(llmConfig, agentProfileForRun(rec))
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

	toolLoopOptions := []toolregistry.ToolLoopOption{
		toolregistry.WithToolLoopMemoryHooks(memory.hooks()),
		toolregistry.WithToolLoopLLMConfig(llmConfig),
		toolregistry.WithProviderPreconditionFallbacks(preconditionFallbacks...),
	}
	if waiter := rt.coagentParkWaiter(rec); waiter != nil {
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithParkWaiter(waiter))
	}
	if isTextureAgentRevisionTaskType(metadataString(rec.Metadata, "type")) {
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithInitialToolChoice(initialTextureToolChoice(rec)))
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithToolLoopBudget(textureActorToolLoopBudget(rec)))
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithTerminalToolSuccesses("patch_texture", "rewrite_texture"))
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithRequiredWriteTools("patch_texture", "rewrite_texture"))
	}

	text, usage, err := toolregistry.RunToolLoop(ctx, tlp, registry, initialMessages, systemPrompt, maxOutputTokens, emit, injectUserTurns, toolLoopOptions...)
	if err != nil {
		if errors.Is(err, toolregistry.ErrToolLoopPassivated) {
			rt.passivateIdleToolLoopRun(context.Background(), rec, text, usage, err)
			return
		}
		if ctx.Err() != nil {
			rt.handleExecutionError(ctx, rec, ctx.Err())
		} else {
			rt.handleExecutionError(ctx, rec, err)
		}
		return
	}
	if err := rt.awaitRequiredTextureRevisions(ctx, rec, 5*time.Minute); err != nil {
		rt.handleExecutionError(ctx, rec, err)
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

	// For texture agent revision runs, create the canonical revision and emit the
	// texture completion event before the run is surfaced as completed. This keeps
	// run completion aligned with document-version availability.
	if err := rt.handleRunCompletion(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}

	// Use a background context for post-provider persistence so that a fast
	// shutdown or cancellation after the provider returns cannot drop the
	// completed-run transition or parent notification.
	persistCtx := context.Background()

	if synthErr := rt.synthesizeResearcherUpdateOnCompletion(persistCtx, rec); synthErr != nil {
		log.Printf("runtime: synthesize researcher completion update for run %s: %v", rec.RunID, synthErr)
	}

	// Persist the terminal run state BEFORE publishing the completion
	// event. Otherwise a subscriber that reacts to the event and
	// immediately fetches run status can observe the run as still
	// running, and if the persist fails the store is left with a
	// completion event for a non-terminal run.
	persisted, err := rt.persistActivationState(persistCtx, rec)
	if err != nil {
		log.Printf("runtime: update run %s to completed: %v", rec.RunID, err)
		return
	}
	if !persisted {
		return
	}
	resultLenPayload, _ := json.Marshal(map[string]any{
		"result_length": len(text),
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	})
	rt.emitEvent(persistCtx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload)
	if shouldLogWireLifecycle(rec) {
		preview := rec.Result
		if len(preview) > 160 {
			preview = preview[:160]
		}
		log.Printf("runtime: completed %s result=%q", wireLifecycleSummary(rec), strings.ReplaceAll(preview, "\n", " "))
	}
	rt.maybeContinuePersistentSuperInbox(persistCtx, rec)
}

func (rt *Runtime) passivateIdleToolLoopRun(ctx context.Context, rec *types.RunRecord, text string, usage provideriface.TokenUsage, passivationErr error) {
	if rt == nil || rt.store == nil || rec == nil {
		return
	}
	reason := "idle_deadline"
	var passivatedErr *toolregistry.ToolLoopPassivatedError
	if errors.As(passivationErr, &passivatedErr) && strings.TrimSpace(passivatedErr.Reason) != "" {
		reason = strings.TrimSpace(passivatedErr.Reason)
	}
	if isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		if err := rt.sleepTextureMutationAfterIdle(ctx, rec); err != nil {
			rt.handleExecutionError(ctx, rec, err)
			return
		}
	}
	now := time.Now().UTC()
	rec.State = types.RunPassivated
	rec.Result = text
	rec.Error = ""
	rec.UpdatedAt = now
	rec.FinishedAt = nil
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.Metadata["input_tokens"] = usage.InputTokens
	rec.Metadata["output_tokens"] = usage.OutputTokens
	rec.Metadata["passivated_reason"] = reason
	rec.Metadata["actor_sleep_state"] = "idle"
	rec.Metadata["actor_sleep_at"] = now.Format(time.RFC3339Nano)

	persisted, err := rt.persistActivationState(context.Background(), rec)
	if err != nil {
		log.Printf("runtime: passivate idle run %s: %v", rec.RunID, err)
		return
	}
	if !persisted {
		return
	}
	payload := map[string]any{
		"reason":        reason,
		"result_length": len(text),
		"input_tokens":  usage.InputTokens,
		"output_tokens": usage.OutputTokens,
	}
	if isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		if docID := strings.TrimSpace(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID)); docID != "" {
			payload["doc_id"] = docID
		}
		if revisionID := strings.TrimSpace(metadataStringValue(rec.Metadata, "current_revision_id")); revisionID != "" {
			payload["current_revision_id"] = revisionID
		}
		if runID := strings.TrimSpace(rec.RunID); runID != "" {
			payload["loop_id"] = runID
		}
	}
	payloadJSON, _ := json.Marshal(payload)
	rt.emitEvent(ctx, rec, types.EventRunPassivated, events.CauseTaskLifecycle, payloadJSON)
	if isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		rt.reconcileCompletedTextureRun(rec)
	}
	if shouldLogWireLifecycle(rec) {
		log.Printf("runtime: passivated idle %s reason=%s", wireLifecycleSummary(rec), reason)
	}
}

func (rt *Runtime) sleepTextureMutationAfterIdle(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	mutation, err := rt.store.GetAgentMutationByRun(ctx, rec.RunID)
	if err != nil {
		return fmt.Errorf("get texture mutation for idle passivation: %w", err)
	}
	if mutation == nil {
		return nil
	}
	switch mutation.State {
	case "pending":
		if revisionID := strings.TrimSpace(mutation.RevisionID); revisionID != "" {
			if rec.Metadata == nil {
				rec.Metadata = map[string]any{}
			}
			rec.Metadata["current_revision_id"] = revisionID
			if err := rt.store.SleepAgentMutation(ctx, rec.RunID); err != nil && err != store.ErrMutationAlreadyCompleted {
				return err
			}
			return nil
		}
		if rt.textureRunRequestedWorkers(ctx, rec) {
			if err := rt.store.DeferAgentMutation(ctx, rec.RunID); err != nil {
				return err
			}
			return nil
		}
		_ = rt.store.FailAgentMutation(ctx, rec.RunID)
		if rec.Metadata == nil {
			rec.Metadata = map[string]any{}
		}
		rec.Metadata["texture_revision_failed_no_write"] = true
		return fmt.Errorf("Texture run passivated without storing a Texture revision")
	case "sleeping", "completed", "deferred":
		return nil
	default:
		return nil
	}
}

// executeWithProvider runs the run through the simple Provider.Execute path.
// This is the legacy execution path used when no tool registry is configured
// (stub provider or bridge provider without tool-calling support).
func (rt *Runtime) executeWithProvider(ctx context.Context, rec *types.RunRecord, emit provideriface.EventEmitFunc) {
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

	// For texture agent revision runs, create the canonical revision and emit the
	// texture completion event before the run is surfaced as completed. This keeps
	// run completion aligned with document-version availability.
	if err := rt.handleRunCompletion(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return
	}

	// Use a background context for post-provider persistence so that a fast
	// shutdown or cancellation after the provider returns cannot drop the
	// completed-run transition or parent notification.
	persistCtx := context.Background()

	if synthErr := rt.synthesizeResearcherUpdateOnCompletion(persistCtx, rec); synthErr != nil {
		log.Printf("runtime: synthesize researcher completion update for run %s: %v", rec.RunID, synthErr)
	}

	// Persist the terminal run state BEFORE publishing the completion
	// event. Otherwise a subscriber that reacts to the event and
	// immediately fetches run status can observe the run as still
	// running, and if the persist fails the store is left with a
	// completion event for a non-terminal run.
	persisted, err := rt.persistActivationState(persistCtx, rec)
	if err != nil {
		log.Printf("runtime: update run %s to completed: %v", rec.RunID, err)
		return
	}
	if !persisted {
		return
	}
	resultLenPayload, _ := json.Marshal(map[string]int{"result_length": len(result)})
	rt.emitEvent(persistCtx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload)
	rt.maybeContinuePersistentSuperInbox(persistCtx, rec)

}

func (rt *Runtime) normalizeCompletedRunResult(rec *types.RunRecord) {
	if rec == nil {
		return
	}
	if agentProfileForRun(rec) != agentprofile.Conductor {
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

func conductorRequestedApp(rec *types.RunRecord) string {
	if rec == nil {
		return agentprofile.Texture
	}
	requestedApp, _ := rec.Metadata["requested_app"].(string)
	if strings.TrimSpace(requestedApp) == "" {
		requestedApp = agentprofile.Texture
	}
	requestedApp = strings.TrimSpace(requestedApp)
	if isTextureDecisionApp(requestedApp) {
		return agentprofile.Texture
	}
	return requestedApp
}

func isTextureDecisionApp(app string) bool {
	switch strings.ToLower(strings.TrimSpace(app)) {
	case agentprofile.Texture:
		return true
	default:
		return false
	}
}

func conductorWindowTitle(rec *types.RunRecord, seedPrompt string) string {
	if rec == nil {
		if strings.TrimSpace(seedPrompt) != "" {
			return strings.TrimSpace(seedPrompt)
		}
		return "Texture"
	}
	title, _ := rec.Metadata["initial_document_title"].(string)
	if strings.TrimSpace(title) == "" {
		title = strings.TrimSpace(seedPrompt)
	}
	if strings.TrimSpace(title) == "" {
		title = "Texture"
	}
	return strings.TrimSpace(title)
}

func fillConductorDecisionFromRun(rec *types.RunRecord, decision conductorDecision) conductorDecision {
	seedPrompt := provider.ConductorSeedPrompt(rec)
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
		if isTextureDecisionApp(decision.App) {
			decision.App = agentprofile.Texture
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
		isTextureDecisionApp(storedDecision.App) &&
		strings.TrimSpace(storedDecision.DocID) != "" {
		rec.Result = stored.Result
	}
}

func normalizeConductorDecision(rec *types.RunRecord) string {
	defaultDecision := fillConductorDecisionFromRun(rec, conductorDecision{})
	if rec == nil {
		out, err := json.Marshal(defaultDecision)
		if err != nil {
			return `{"action":"open_app","app":"texture","title":"Texture","seed_prompt":"","create_initial_version":false}`
		}
		return string(out)
	}

	if raw := strings.TrimSpace(rec.Result); raw != "" {
		var parsed conductorDecision
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil && strings.TrimSpace(parsed.Action) != "" {
			switch strings.TrimSpace(parsed.Action) {
			case "toast":
				parsed = fillConductorDecisionFromRun(rec, parsed)
				if metadataStringValue(rec.Metadata, "doc_id") != "" && isTextureDecisionApp(metadataStringValue(rec.Metadata, "requested_app")) {
					parsed.Action = "open_app"
					parsed.App = agentprofile.Texture
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
		return `{"action":"open_app","app":"texture","title":"Texture","seed_prompt":"","create_initial_version":false}`
	}
	return string(out)
}

func ptrBool(v bool) *bool {
	return &v
}

func fallbackPromptBarInitialContent(rec *types.RunRecord, decision conductorDecision) string {
	if rec == nil || metadataStringValue(rec.Metadata, "input_source") != "prompt_bar" {
		return ""
	}
	if !isTextureDecisionApp(conductorRequestedApp(rec)) {
		return ""
	}
	seedPrompt := strings.TrimSpace(decision.SeedPrompt)
	if seedPrompt == "" {
		seedPrompt = provider.ConductorSeedPrompt(rec)
	}
	if seedPrompt == "" {
		return ""
	}
	title := strings.TrimSpace(decision.Title)
	if title == "" {
		title = conductorWindowTitle(rec, seedPrompt)
	}
	if title == "" {
		title = provider.InitialTextureTitle(seedPrompt, "")
	}
	if title == "" || strings.EqualFold(title, seedPrompt) {
		return seedPrompt
	}
	return "# " + title + "\n\n" + seedPrompt
}

func (rt *Runtime) ensureConductorTextureRoute(ctx context.Context, rec *types.RunRecord, objective, initialContent string) (conductorDecision, error) {
	if rec == nil || agentProfileForRun(rec) != agentprofile.Conductor {
		return conductorDecision{}, fmt.Errorf("conductor route requires a conductor record")
	}

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
				isTextureDecisionApp(parsedDecision.App) &&
				strings.TrimSpace(parsedDecision.DocID) != "" {
				return fillConductorDecisionFromRun(rec, parsedDecision), nil
			}
		}
	}
	existing := fillConductorDecisionFromRun(rec, conductorDecision{})
	if existing.Action == "open_app" && isTextureDecisionApp(existing.App) && strings.TrimSpace(existing.DocID) != "" {
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
		doc.Title = "Texture"
	}
	if err := rt.store.CreateDocument(ctx, doc); err != nil {
		return conductorDecision{}, fmt.Errorf("create texture document: %w", err)
	}

	userRevisionID := uuid.New().String()
	routeSeedPrompt := firstNonEmptyString(
		strings.TrimSpace(decision.SeedPrompt),
		provider.ConductorSeedPrompt(rec),
		strings.TrimSpace(rec.Prompt),
		metadataStringValue(rec.Metadata, "seed_prompt"),
	)
	userRevisionMetadata := map[string]any{
		"seed_prompt":                 routeSeedPrompt,
		"conductor_loop_id":           rec.RunID,
		runMetadataTrajectoryID:       trajectoryIDForRun(rec),
		runMetadataLLMPolicyOverlayID: metadataString(rec.Metadata, runMetadataLLMPolicyOverlayID),
		runMetadataOwnerEmail:         metadataString(rec.Metadata, runMetadataOwnerEmail),
		"created_from":                "conductor",
		"source":                      "user_prompt",
		"revision_role":               textureRevisionRoleInput,
		"input_origin":                textureInputOriginUserPrompt,
		"texture_version":             "v0",
		textureMetadataPromptUnixTS:   now.Unix(),
	}
	// The owner prompt is the canonical Texture V0. For prompt-bar-created
	// Texture, V0 content is exactly the owner's prompt text, not blank metadata
	// or a separate intake surface. seed_prompt is retained only as provenance.
	userRevisionContent := routeSeedPrompt
	if metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		if promptText := strings.TrimSpace(metadataStringValue(rec.Metadata, "seed_prompt")); promptText != "" {
			userRevisionContent = promptText
		}
	}
	userRevMeta, _ := json.Marshal(userRevisionMetadata)
	userRev := types.Revision{
		RevisionID:  userRevisionID,
		DocID:       doc.DocID,
		OwnerID:     rec.OwnerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: rec.OwnerID,
		Content:     userRevisionContent,
		Citations:   json.RawMessage("[]"),
		Metadata:    userRevMeta,
		CreatedAt:   now,
	}
	if err := rt.store.CreateRevision(ctx, userRev); err != nil {
		return conductorDecision{}, fmt.Errorf("create user prompt Texture revision: %w", err)
	}
	rt.emitTextureDocumentRevisionEventForRun(ctx, rec, userRev)

	doc.CurrentRevisionID = userRev.RevisionID
	if err := rt.store.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   currentTextureAgentID(doc.DocID),
		OwnerID:   rec.OwnerID,
		SandboxID: rt.cfg.SandboxID,
		Profile:   agentprofile.Texture,
		Role:      agentprofile.Texture,
		ChannelID: doc.DocID,
		CreatedAt: now,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return conductorDecision{}, fmt.Errorf("persist Texture appagent: %w", err)
	}
	decision.DocID = doc.DocID
	decision.UserRevisionID = userRev.RevisionID
	if decision.InitialRevisionID == "" {
		decision.InitialRevisionID = userRev.RevisionID
	}

	initialPrompt := strings.TrimSpace(objective)
	if initialPrompt == "" {
		initialPrompt = routeSeedPrompt
	}
	if initialPrompt == "" {
		initialPrompt = "Create the first useful current-state version of this Texture document."
	}
	initialRun, err := rt.submitTextureAgentRevisionRun(ctx, doc, rec.OwnerID, textureAgentRevisionRequest{
		Intent: "initial_conductor_workflow",
		Prompt: initialPrompt,
	}, 0)
	if err != nil {
		return conductorDecision{}, fmt.Errorf("start initial Texture agent revision: %w", err)
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
	if rec == nil || agentProfileForRun(rec) != agentprofile.Conductor {
		return
	}

	var decision conductorDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(rec.Result)), &decision); err != nil {
		return
	}
	if decision.Action == "toast" &&
		isTextureDecisionApp(metadataStringValue(rec.Metadata, "requested_app")) &&
		metadataStringValue(rec.Metadata, "input_source") == "prompt_bar" {
		if _, err := rt.ensureConductorTextureRoute(context.Background(), rec, "", decision.InitialContent); err != nil {
			log.Printf("runtime: conductor run %s: materialize prompt-bar Texture route: %v", rec.RunID, err)
		}
		return
	}
	if decision.Action != "open_app" || !isTextureDecisionApp(decision.App) || strings.TrimSpace(decision.DocID) != "" {
		return
	}

	if _, err := rt.ensureConductorTextureRoute(context.Background(), rec, "", decision.InitialContent); err != nil {
		log.Printf("runtime: conductor run %s: materialize decision: %v", rec.RunID, err)
	}
}

// initialTextureToolChoice is reserved for narrow mechanical continuation
// protocols. Ordinary first-paint Texture work must see the full Texture tool
// surface so the actor can choose an honest revision, decision, delegation, or
// blocker without hidden exact-tool choreography.
//
// For update_coagent continuations (worker evidence arrived), the model must
// produce a document revision but may choose patch_texture for small deltas or
// rewrite_texture for full-document drafts (especially v0→v1 and v1→v2). The
// post-turn required-write-tool check ensures a revision actually lands.
func initialTextureToolChoice(rec *types.RunRecord) string {
	if rec == nil || !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		return ""
	}
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
		return "required"
	}
	if metadataIntValue(rec.Metadata, "scheduled_message_seq") > 0 {
		return ""
	}
	if metadataStringValue(rec.Metadata, "request_intent") == "revise" &&
		metadataStringValue(rec.Metadata, "current_author_kind") == string(types.AuthorUser) {
		return "required"
	}
	return ""
}

const (
	defaultTextureActorMaxProviderCalls = 80
	defaultTextureActorMaxTotalTokens   = 1200000
	defaultTextureActorMaxElapsed       = 45 * time.Minute
)

func textureActorToolLoopBudget(rec *types.RunRecord) toolregistry.ToolLoopBudget {
	docID := ""
	if rec != nil {
		docID = strings.TrimSpace(firstNonEmpty(
			metadataStringValue(rec.Metadata, "doc_id"),
			rec.ChannelID,
		))
	}
	label := "texture"
	if docID != "" {
		label = "texture:" + docID
	}
	budget := toolregistry.ToolLoopBudget{
		Label:            label,
		MaxProviderCalls: defaultTextureActorMaxProviderCalls,
		MaxTotalTokens:   defaultTextureActorMaxTotalTokens,
		MaxElapsed:       defaultTextureActorMaxElapsed,
	}
	if rec == nil {
		return budget
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_provider_calls"); value > 0 {
		budget.MaxProviderCalls = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_input_tokens"); value > 0 {
		budget.MaxInputTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_output_tokens"); value > 0 {
		budget.MaxOutputTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_total_tokens"); value > 0 {
		budget.MaxTotalTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_max_elapsed_seconds"); value > 0 {
		budget.MaxElapsed = time.Duration(value) * time.Second
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_spent_provider_calls"); value > 0 {
		budget.SpentProviderCalls = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_spent_input_tokens"); value > 0 {
		budget.SpentInputTokens = value
	}
	if value := metadataIntValue(rec.Metadata, "actor_budget_spent_output_tokens"); value > 0 {
		budget.SpentOutputTokens = value
	}
	return budget
}

type actorToolLoopBudgetSpend struct {
	SourceRunID        string
	ProviderCalls      int
	InputTokens        int
	OutputTokens       int
	ObservedUsageEvent bool
}

func (rt *Runtime) latestActorToolLoopBudgetSpend(ctx context.Context, ownerID, agentID string) (actorToolLoopBudgetSpend, bool, error) {
	var spend actorToolLoopBudgetSpend
	if rt == nil || rt.store == nil {
		return spend, false, nil
	}
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return spend, false, nil
	}
	sourceRunID, _, err := rt.store.LatestActorRunMemoryEntries(ctx, ownerID, agentID, "")
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return spend, false, nil
		}
		return spend, false, err
	}
	spend.SourceRunID = sourceRunID
	eventsForRun, err := rt.store.ListEvents(ctx, sourceRunID, 5000)
	if err != nil {
		return spend, false, err
	}
	providerCallsFromPreflight := 0
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventRunProgress {
			continue
		}
		switch ev.Phase {
		case "provider_call":
			providerCallsFromPreflight++
		case "tool_loop_budget_usage", "tool_loop_budget":
			var payload map[string]any
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				continue
			}
			spend.ObservedUsageEvent = true
			if value := metadataIntValue(payload, "provider_calls"); value > spend.ProviderCalls {
				spend.ProviderCalls = value
			}
			if value := metadataIntValue(payload, "input_tokens"); value > spend.InputTokens {
				spend.InputTokens = value
			}
			if value := metadataIntValue(payload, "output_tokens"); value > spend.OutputTokens {
				spend.OutputTokens = value
			}
		}
	}
	if spend.ProviderCalls == 0 && providerCallsFromPreflight > 0 {
		spend.ProviderCalls = providerCallsFromPreflight
	}
	return spend, true, nil
}

func (rt *Runtime) recordExplicitInitialTextureDecisionIfNeeded(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	if !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) ||
		!metadataBoolValue(rec.Metadata, "texture_initial_decision_required") {
		return nil
	}
	docID := metadataStringValue(rec.Metadata, "doc_id")
	reason := metadataStringValue(rec.Metadata, "texture_initial_decision_reason")
	kind := metadataStringValue(rec.Metadata, "texture_initial_decision_kind")
	if docID == "" || reason == "" || kind != "no_worker_needed" {
		return nil
	}
	existing, err := rt.store.ListTextureDecisionsByDocument(ctx, rec.OwnerID, docID, 100)
	if err != nil {
		return fmt.Errorf("list initial Texture decisions: %w", err)
	}
	for _, decision := range existing {
		if decision.RunID == rec.RunID && decision.DecisionKind == kind && decision.Reason == reason {
			rec.Metadata["texture_initial_decision_recorded"] = true
			return nil
		}
	}
	decision := types.TextureDecisionRecord{
		DecisionID:   uuid.New().String(),
		OwnerID:      rec.OwnerID,
		DocID:        docID,
		RunID:        rec.RunID,
		TrajectoryID: trajectoryIDForRun(rec),
		ActorID:      strings.TrimSpace(rec.AgentID),
		DecisionKind: kind,
		Reason:       reason,
		EvidenceRefs: metadataStringSliceValue(rec.Metadata, "texture_initial_decision_evidence_refs"),
		NextAction:   metadataStringValue(rec.Metadata, "texture_initial_decision_next_action"),
		CreatedAt:    time.Now().UTC(),
	}
	if decision.ActorID == "" {
		decision.ActorID = currentTextureAgentID(docID)
	}
	if err := rt.store.CreateTextureDecision(ctx, decision); err != nil {
		return fmt.Errorf("record initial Texture decision: %w", err)
	}
	rt.emitTextureDecisionRecordedEvent(ctx, rec, decision)
	rec.Metadata["texture_initial_decision_recorded"] = true
	return nil
}

func metadataStringSliceValue(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	switch values := metadata[key].(type) {
	case []string:
		return append([]string(nil), values...)
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	case string:
		if strings.TrimSpace(values) == "" {
			return nil
		}
		return []string{strings.TrimSpace(values)}
	default:
		return nil
	}
}

type explicitInitialTextureDecision struct {
	DecisionKind string
	Reason       string
	EvidenceRefs []string
	NextAction   string
}

func explicitNoWorkerDecisionRequestFromPrompt(prompt string) (explicitInitialTextureDecision, bool) {
	text := strings.TrimSpace(prompt)
	if !texturePromptExplicitlyRequestsNoWorkerDecision(text) {
		return explicitInitialTextureDecision{}, false
	}
	lower := strings.ToLower(text)
	reason := extractDelimitedPromptValue(text, lower, "exact reason ", []string{", evidence ref", ", evidence refs", ", next action", ". then "})
	if reason == "" {
		reason = extractDelimitedPromptValue(text, lower, "reason ", []string{", evidence ref", ", evidence refs", ", next action", ". then "})
	}
	if reason == "" {
		return explicitInitialTextureDecision{}, false
	}
	evidence := extractDelimitedPromptValue(text, lower, "evidence ref ", []string{", next action", ". then "})
	if evidence == "" {
		evidence = extractDelimitedPromptValue(text, lower, "evidence refs ", []string{", next action", ". then "})
	}
	nextAction := extractDelimitedPromptValue(text, lower, "next action ", []string{". then ", " then "})
	return explicitInitialTextureDecision{
		DecisionKind: "no_worker_needed",
		Reason:       strings.TrimSpace(reason),
		EvidenceRefs: splitPromptRefs(evidence),
		NextAction:   strings.TrimSpace(nextAction),
	}, true
}

func extractDelimitedPromptValue(original, lower, marker string, delimiters []string) string {
	start := strings.Index(lower, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := len(original)
	tailLower := lower[start:]
	for _, delimiter := range delimiters {
		if idx := strings.Index(tailLower, delimiter); idx >= 0 && start+idx < end {
			end = start + idx
		}
	}
	return strings.Trim(strings.TrimSpace(original[start:end]), " ,")
}

func splitPromptRefs(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func texturePromptExplicitlyRequestsDecisionNote(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	if strings.Contains(text, "record_texture_decision") {
		return true
	}
	if strings.Contains(text, "decision_kind") && strings.Contains(text, "off-document") && strings.Contains(text, "decision") {
		return true
	}
	if strings.Contains(text, "record") && strings.Contains(text, "off-document") && strings.Contains(text, "decision note") {
		return true
	}
	if strings.Contains(text, "record") && strings.Contains(text, "texture decision") {
		return true
	}
	if strings.Contains(text, "record") && strings.Contains(text, "texture decision") {
		return true
	}
	return false
}

func texturePromptExplicitlyRequestsNoWorkerDecision(prompt string) bool {
	text := strings.ToLower(strings.TrimSpace(prompt))
	if text == "" {
		return false
	}
	if strings.Contains(text, "decision_kind") && strings.Contains(text, "no_worker_needed") {
		return true
	}
	if strings.Contains(text, "no-worker") && strings.Contains(text, "decision") {
		return true
	}
	if strings.Contains(text, "no worker") && strings.Contains(text, "decision") {
		return true
	}
	if strings.Contains(text, "no research or execution worker") && texturePromptExplicitlyRequestsDecisionNote(text) {
		return true
	}
	return false
}

// handleRunCompletion processes feature-specific side effects after a run
// completes successfully. Texture document writes are intentionally not handled
// here: canonical appagent revisions are created only by Texture write tools.
func (rt *Runtime) handleRunCompletion(ctx context.Context, rec *types.RunRecord) error {
	if agentProfileForRun(rec) == agentprofile.Conductor {
		rt.materializeConductorDecision(rec)
		return nil
	}

	taskType, _ := rec.Metadata["type"].(string)
	if !isTextureAgentRevisionTaskType(taskType) {
		return nil
	}

	persistCtx := context.Background()

	docID, _ := rec.Metadata["doc_id"].(string)
	if docID == "" {
		log.Printf("runtime: texture agent revision run %s: missing doc_id in metadata", rec.RunID)
		return nil
	}

	mutation, err := rt.store.GetAgentMutationByRun(persistCtx, rec.RunID)
	if err != nil {
		log.Printf("runtime: texture agent revision run %s: get mutation: %v", rec.RunID, err)
		return nil
	}
	if mutation == nil {
		log.Printf("runtime: texture agent revision run %s: no mutation record found", rec.RunID)
		return nil
	}
	if mutation.State == "completed" {
		return nil
	}
	if mutation.State != "pending" {
		return nil
	}

	if strings.TrimSpace(mutation.RevisionID) != "" {
		if err := rt.store.CompleteAgentMutation(persistCtx, rec.RunID, mutation.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
			log.Printf("runtime: texture agent revision run %s: complete written mutation: %v", rec.RunID, err)
			return nil
		}
		return nil
	}

	if rt.textureRunRequestedWorkers(persistCtx, rec) {
		if err := rt.store.DeferAgentMutation(persistCtx, rec.RunID); err != nil {
			log.Printf("runtime: texture agent revision run %s: defer no-edit mutation: %v", rec.RunID, err)
			return nil
		}
		progressPayload, _ := json.Marshal(map[string]string{
			"doc_id":  docID,
			"loop_id": rec.RunID,
			"status":  "waiting_for_worker_updates",
		})
		rt.emitTextureAgentEvent(persistCtx, rec, types.EventTextureAgentRevisionProgress,
			events.CauseToolExecution, progressPayload)
		log.Printf("runtime: texture agent revision run %s requested workers and completed without document edit; waiting for worker updates", rec.RunID)
		return nil
	}
	_ = rt.store.FailAgentMutation(persistCtx, rec.RunID)
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.Metadata["texture_revision_failed_no_write"] = true
	failPayload, _ := json.Marshal(map[string]string{
		"doc_id":  docID,
		"loop_id": rec.RunID,
		"error":   "Texture run completed without storing a Texture revision",
	})
	rt.emitTextureAgentEvent(persistCtx, rec, types.EventTextureAgentRevisionFailed,
		events.CauseTaskLifecycle, failPayload)
	log.Printf("runtime: Texture agent revision run %s completed without a Texture write tool; no canonical revision created", rec.RunID)
	return nil
}

func (rt *Runtime) textureRunRequestedWorkers(ctx context.Context, rec *types.RunRecord) bool {
	if rt == nil || rt.store == nil || rec == nil {
		return false
	}
	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 500)
	if err != nil {
		log.Printf("runtime: texture run %s: list events for worker requests: %v", rec.RunID, err)
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
			if strings.TrimSpace(profile) == agentprofile.Researcher || strings.TrimSpace(role) == agentprofile.Researcher {
				return true
			}
		}
	}
	return false
}

func (rt *Runtime) reconcileCompletedTextureRun(rec *types.RunRecord) {
	if rec == nil {
		return
	}
	docID, _ := rec.Metadata["doc_id"].(string)
	if strings.TrimSpace(docID) == "" {
		agentID := agentIDForRun(rec)
		if isTextureAgentID(agentID) {
			docID = docIDFromTextureAgentID(agentID)
		}
	}
	if strings.TrimSpace(docID) == "" && agentProfileForRun(rec) == agentprofile.Texture {
		docID = channelIDForRun(rec)
	}
	if strings.TrimSpace(docID) == "" || strings.TrimSpace(rec.OwnerID) == "" {
		return
	}
	if err := rt.reconcileTextureWorkerState(context.Background(), rec.OwnerID, docID); err != nil {
		log.Printf("runtime: texture agent revision run %s: post-complete reconcile: %v", rec.RunID, err)
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
		case agentprofile.Researcher, agentprofile.Super, agentprofile.CoSuper:
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

func (rt *Runtime) maybeWakeTextureOnWorkerMessage(ctx context.Context, ownerID string, message ChannelMessage) {
	channelID := strings.TrimSpace(message.ChannelID)
	fromRunID := strings.TrimSpace(message.FromRunID)
	targetAgentID := strings.TrimSpace(message.ToAgentID)
	if strings.TrimSpace(ownerID) == "" || channelID == "" || targetAgentID == "" {
		return
	}

	doc, err := rt.store.GetDocument(ctx, channelID, ownerID)
	if err != nil {
		if err != store.ErrNotFound {
			log.Printf("runtime: wake texture for channel %s: get document: %v", channelID, err)
		}
		return
	}

	if fromRunID != "" {
		sourceRun, err := rt.store.GetRun(ctx, fromRunID)
		if err != nil {
			log.Printf("runtime: wake texture for doc %s: get source run %s: %v", doc.DocID, fromRunID, err)
			return
		}
		switch agentProfileForRun(&sourceRun) {
		case agentprofile.Researcher, agentprofile.Super, agentprofile.VSuper, agentprofile.CoSuper:
		default:
			return
		}
	}

	if !textureAgentIDMatchesDoc(targetAgentID, doc.DocID) {
		return
	}
	rt.scheduleTextureWorkerWake(ownerID, doc.DocID, fromRunID)
}

const (
	canonicalTextureSourcePathMetadataKey = "canonical_texture_source_path"
	textureAvailableSourceEntitiesKey     = "texture_available_source_entities"
)

// durableMetadataKeys lists the revision metadata keys that must survive
// across appagent revisions so that subsequent revise requests retain
// the original user context (seed_prompt, source_path, etc.).
var durableMetadataKeys = []string{
	"seed_prompt",
	runMetadataExplicitResearcher,
	"source_path",
	canonicalTextureSourcePathMetadataKey,
	"import_manifest",
	"migration_manifest",
	"conductor_loop_id",
	runMetadataTrajectoryID,
	"artifact_kind",
	"revision_role",
	"input_origin",
	"texture_version_stage",
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
	runMetadataLLMPolicyOverlayID,
	textureAvailableSourceEntitiesKey,
}

// buildAppagentRevisionMetadata constructs the metadata JSON for an
// appagent-authored revision, carrying forward durable context keys
// from the parent revision so they remain available on the next revise.
func (rt *Runtime) buildAppagentRevisionMetadata(ctx context.Context, rec *types.RunRecord, doc types.Document, ownerID string, mutation *store.AgentMutation, consumedThroughSeq int64) json.RawMessage {
	meta := map[string]any{
		"source":  "patch_texture",
		"loop_id": rec.RunID,
	}

	// Carry forward durable keys from the parent revision metadata.
	if doc.CurrentRevisionID != "" {
		if parentRev, err := rt.store.GetRevision(context.Background(), doc.CurrentRevisionID, ownerID); err == nil {
			parentMeta := decodeRevisionMetadata(parentRev.Metadata)
			for _, key := range durableMetadataKeys {
				if val, ok := parentMeta[key]; ok && hasNonEmptyTextureMetadataValue(val) {
					meta[key] = val
				}
			}
			promoteCanonicalTextureSourcePath(meta, parentMeta)
		}
	}

	// Also carry forward from run metadata (the initial agent revision
	// request sets these directly).
	if rec.Metadata != nil {
		for _, key := range durableMetadataKeys {
			if val, ok := rec.Metadata[key]; ok && hasNonEmptyTextureMetadataValue(val) {
				// Run metadata takes precedence over parent revision.
				meta[key] = val
			}
		}
		if val, ok := canonicalTextureSourcePathMetadataValue(rec.Metadata); ok {
			meta[canonicalTextureSourcePathMetadataKey] = val
		}
		if requestedByRunID := metadataStringValue(rec.Metadata, "requested_by_run_id"); requestedByRunID != "" {
			meta["requested_by_run_id"] = requestedByRunID
		}
	}
	promptOnlyInitialModelPrior := promptOnlyInitialModelPriorTextureRevision(rec, meta, consumedThroughSeq)
	if wirepublish.IsWireArticleRevisionRun(rec) && !promptOnlyInitialModelPrior {
		meta["artifact_kind"] = "article_revision"
		meta["revision_role"] = textureRevisionRoleCanonical
		meta["texture_version_stage"] = "article_revision"
	}
	workerUpdateMeta := rt.workerUpdateRevisionMetadata(ctx, ownerID, doc.DocID, mutation, consumedThroughSeq)
	if promptOnlyInitialModelPrior {
		meta["grounding_status"] = "model_prior_interim"
		meta["revision_grounding"] = "model_prior"
		meta["texture_version_stage"] = "interim"
		meta["model_prior_interim"] = true
		if metadataString(meta, "artifact_kind") == "article_revision" {
			meta["artifact_kind"] = "working_revision"
		}
		if metadataString(meta, "revision_role") == textureRevisionRoleCanonical {
			meta["revision_role"] = textureRevisionRoleInput
		}
	}
	for key, value := range workerUpdateMeta {
		meta[key] = value
	}
	// Available source entities are run-time prompt context, not durable revision metadata.
	// Keep them out of the persisted revision so they do not leak into the next run's parent
	// revision projection and are recomputed from the actual revision's source_entities.
	delete(meta, textureAvailableSourceEntitiesKey)

	data, err := json.Marshal(meta)
	if err != nil {
		return json.RawMessage(`{"source":"patch_texture","loop_id":"` + rec.RunID + `"}`)
	}
	return data
}

func promptOnlyInitialModelPriorTextureRevision(rec *types.RunRecord, meta map[string]any, consumedThroughSeq int64) bool {
	if rec == nil || !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		return false
	}
	if consumedThroughSeq != 0 || metadataIntValue(rec.Metadata, "scheduled_message_seq") != 0 {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") == "update_coagent" {
		return false
	}
	inputOrigin := firstNonEmpty(
		metadataString(meta, "input_origin"),
		metadataStringValue(rec.Metadata, "input_origin"),
	)
	if inputOrigin == textureInputOriginUserPrompt {
		return true
	}
	if strings.TrimSpace(metadataStringValue(rec.Metadata, "request_intent")) == "initial_conductor_workflow" &&
		strings.TrimSpace(metadataStringValue(rec.Metadata, "seed_prompt")) != "" {
		return true
	}
	return false
}

func textureWorkerUpdateMetadataHasRole(value any, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	switch updates := value.(type) {
	case []textureWorkerUpdateMetadata:
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

type textureWorkerUpdateMetadata struct {
	ChannelID      string `json:"channel_id"`
	Seq            int64  `json:"seq"`
	FromAgentID    string `json:"from_agent_id,omitempty"`
	FromLoopID     string `json:"from_loop_id,omitempty"`
	Role           string `json:"role,omitempty"`
	ContentPreview string `json:"content_preview,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

func (rt *Runtime) workerUpdateRevisionMetadata(ctx context.Context, ownerID, docID string, mutation *store.AgentMutation, consumedThroughSeq int64) map[string]any {
	out := map[string]any{
		"worker_updates_policy":         "eligible_addressed_channel_messages",
		"worker_updates_checkpoint_seq": int64(0),
		"worker_updates_scheduled_seq":  int64(0),
		"worker_updates_consumed":       []textureWorkerUpdateMetadata{},
		"worker_updates_skipped":        []textureWorkerUpdateMetadata{},
		"worker_updates_pending":        []textureWorkerUpdateMetadata{},
	}
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(docID) == "" {
		return out
	}

	scheduledSeq := int64(0)
	if mutation != nil {
		scheduledSeq = mutation.ScheduledMessageSeq
	}
	if consumedThroughSeq > scheduledSeq {
		scheduledSeq = consumedThroughSeq
	}
	out["worker_updates_scheduled_seq"] = scheduledSeq

	checkpointSeq := int64(0)
	checkpoint, err := rt.store.GetTextureControllerCheckpoint(ctx, docID, ownerID)
	if err != nil {
		log.Printf("runtime: load texture worker update checkpoint for metadata: %v", err)
		return out
	}
	if checkpoint != nil {
		checkpointSeq = checkpoint.IntegratedMessageSeq
	}
	out["worker_updates_checkpoint_seq"] = checkpointSeq

	messageAfterSeq := checkpointSeq
	if scheduledSeq > 0 && checkpointSeq >= scheduledSeq {
		if previousSeq := rt.previousTextureWorkerMetadataSeq(ctx, ownerID, docID); previousSeq < scheduledSeq {
			messageAfterSeq = previousSeq
		}
	}

	messages, err := rt.store.ListChannelMessages(ctx, ownerID, docID, messageAfterSeq, 500)
	if err != nil {
		log.Printf("runtime: load texture worker update messages for metadata: %v", err)
		return out
	}

	cache := make(map[string]bool)
	consumed := []textureWorkerUpdateMetadata{}
	skipped := []textureWorkerUpdateMetadata{}
	pending := []textureWorkerUpdateMetadata{}
	for _, message := range messages {
		if !textureAgentIDMatchesDoc(message.ToAgentID, docID) {
			continue
		}
		eligible, err := rt.isEligibleWorkerMessage(ctx, docID, message, cache)
		if err != nil {
			log.Printf("runtime: classify texture worker update for metadata: %v", err)
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

func (rt *Runtime) textureWorkerUpdateCommitSeq(ctx context.Context, rec *types.RunRecord, docID string, mutation *store.AgentMutation) int64 {
	seq := int64(0)
	if mutation != nil {
		seq = mutation.ScheduledMessageSeq
	}
	if rt == nil || rt.store == nil || rec == nil {
		return seq
	}
	if seq == 0 {
		seq = int64(metadataIntValue(rec.Metadata, "scheduled_message_seq"))
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	targetAgentID := currentTextureAgentID(docID)
	if ownerID == "" || strings.TrimSpace(targetAgentID) == "" {
		return seq
	}
	for _, updateID := range coagentUpdateIDsForRun(rec) {
		updateID = strings.TrimSpace(updateID)
		if updateID == "" {
			continue
		}
		update, err := rt.store.GetWorkerUpdate(ctx, ownerID, updateID)
		if err != nil {
			log.Printf("runtime: load injected texture worker update %s for revision metadata: %v", updateID, err)
			continue
		}
		if strings.TrimSpace(update.TargetAgentID) != targetAgentID || strings.TrimSpace(update.ChannelID) != strings.TrimSpace(docID) {
			continue
		}
		if update.MessageSeq > seq {
			seq = update.MessageSeq
		}
	}
	return seq
}

func (rt *Runtime) previousTextureWorkerMetadataSeq(ctx context.Context, ownerID, docID string) int64 {
	if rt == nil || rt.store == nil {
		return 0
	}
	doc, err := rt.store.GetDocument(ctx, docID, ownerID)
	if err != nil || strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return 0
	}
	rev, err := rt.store.GetRevision(ctx, doc.CurrentRevisionID, ownerID)
	if err != nil {
		return 0
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	return maxTextureWorkerMetadataSeq(meta)
}

func maxTextureWorkerMetadataSeq(meta map[string]any) int64 {
	maxSeq := int64(metadataIntValue(meta, "worker_updates_scheduled_seq"))
	if checkpointSeq := int64(metadataIntValue(meta, "worker_updates_checkpoint_seq")); checkpointSeq > maxSeq {
		maxSeq = checkpointSeq
	}
	for _, key := range []string{"worker_updates_consumed", "worker_updates_skipped"} {
		for _, item := range metadataArray(meta[key]) {
			itemMeta, _ := item.(map[string]any)
			if seq := int64(metadataIntValue(itemMeta, "seq")); seq > maxSeq {
				maxSeq = seq
			}
		}
	}
	return maxSeq
}

func metadataArray(value any) []any {
	switch items := value.(type) {
	case []any:
		return items
	case []textureWorkerUpdateMetadata:
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, map[string]any{
				"seq": item.Seq,
			})
		}
		return out
	default:
		return nil
	}
}

func summarizeWorkerUpdateForMetadata(message types.ChannelMessage, reason string) textureWorkerUpdateMetadata {
	return textureWorkerUpdateMetadata{
		ChannelID:      message.ChannelID,
		Seq:            message.Seq,
		FromAgentID:    strings.TrimSpace(message.FromAgentID),
		FromLoopID:     strings.TrimSpace(message.FromRunID),
		Role:           strings.TrimSpace(message.Role),
		ContentPreview: truncatePromptSnippet(message.Content, 240),
		Reason:         strings.TrimSpace(reason),
	}
}

func (rt *Runtime) markTextureWorkerUpdatesDelivered(ctx context.Context, rec *types.RunRecord, docID string, maxSeq int64) error {
	if rt == nil || rt.store == nil || rec == nil || maxSeq <= 0 {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil
	}
	targetAgentIDs := []string{currentTextureAgentID(docID)}
	updates := make([]types.CoagentSourcePacket, 0)
	seenUpdates := make(map[string]bool)
	targetByUpdateID := make(map[string]string)
	for _, targetAgentID := range targetAgentIDs {
		if targetAgentID == "" {
			continue
		}
		targetUpdates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, targetAgentID, 500)
		if err != nil {
			return err
		}
		for _, update := range targetUpdates {
			if seenUpdates[update.UpdateID] {
				continue
			}
			seenUpdates[update.UpdateID] = true
			targetByUpdateID[update.UpdateID] = targetAgentID
			updates = append(updates, update)
		}
	}
	updateIDsByTarget := make(map[string][]string)
	for _, update := range updates {
		if strings.TrimSpace(update.ChannelID) == docID && update.MessageSeq > 0 && update.MessageSeq <= maxSeq {
			targetAgentID := targetByUpdateID[update.UpdateID]
			updateIDsByTarget[targetAgentID] = append(updateIDsByTarget[targetAgentID], update.UpdateID)
		}
	}
	for targetAgentID, updateIDs := range updateIDsByTarget {
		if len(updateIDs) == 0 {
			continue
		}
		if err := rt.store.MarkWorkerUpdatesDelivered(ctx, ownerID, targetAgentID, updateIDs, rec.RunID); err != nil {
			return err
		}
	}
	if err := rt.advanceTextureControllerCheckpoint(ctx, ownerID, docID, maxSeq); err != nil {
		return err
	}
	return nil
}

func (rt *Runtime) markTextureRevisionRunUpdatesDelivered(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil || !isTextureAgentRevisionTaskType(metadataStringValue(rec.Metadata, "type")) {
		return nil
	}
	docID := firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID)
	if strings.TrimSpace(docID) == "" {
		return nil
	}
	consumedThroughSeq := rt.textureWorkerUpdateCommitSeq(ctx, rec, docID, nil)
	if consumedThroughSeq <= 0 {
		return nil
	}
	return rt.markTextureWorkerUpdatesDelivered(ctx, rec, docID, consumedThroughSeq)
}

func (rt *Runtime) advanceTextureControllerCheckpoint(ctx context.Context, ownerID, docID string, seq int64) error {
	if rt == nil || rt.store == nil || seq <= 0 {
		return nil
	}
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil
	}
	checkpoint, err := rt.store.GetTextureControllerCheckpoint(ctx, docID, ownerID)
	if err != nil {
		return fmt.Errorf("load texture controller checkpoint: %w", err)
	}
	if checkpoint != nil && checkpoint.IntegratedMessageSeq >= seq {
		return nil
	}
	return rt.store.UpsertTextureControllerCheckpoint(ctx, store.TextureControllerCheckpoint{
		DocID:                docID,
		OwnerID:              ownerID,
		IntegratedMessageSeq: seq,
		UpdatedAt:            time.Now().UTC(),
	})
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
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = errors.New("activation budget exceeded: progress deadline reached")
		}
		// Context cancellation means the runtime is shutting down or the
		// run was cancelled, not a provider failure. Treat as cancelled.
		state = types.RunCancelled
		kind = types.EventRunCancelled
		cause = events.CauseTaskLifecycle
	} else if isRunMemoryBlockedError(err) {
		state = types.RunBlocked
		kind = types.EventRunBlocked
		cause = events.CauseSupervisorRecovery
	} else if toolregistry.IsProviderRateLimitError(err) {
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
	persisted, updateErr := rt.persistActivationState(persistCtx, rec)
	if updateErr != nil {
		log.Printf("runtime: update run %s to %s: %v", rec.RunID, state, updateErr)
	}
	if !persisted {
		return
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

	// If this is a Texture agent revision task, settle the mutation before any
	// reconcile pass. A no-write failure must not immediately requeue the same
	// undelivered packet forever; a failure after a successful write should still
	// close the mutation on the latest stored revision.
	if taskType, _ := rec.Metadata["type"].(string); isTextureAgentRevisionTaskType(taskType) {
		failedNoWrite := true
		if mutation, mutationErr := rt.store.GetAgentMutationByRun(persistCtx, rec.RunID); mutationErr != nil {
			log.Printf("runtime: texture agent revision run %s: get mutation after failure: %v", rec.RunID, mutationErr)
		} else if mutation != nil {
			if strings.TrimSpace(mutation.RevisionID) != "" {
				if completeErr := rt.store.CompleteAgentMutation(persistCtx, rec.RunID, mutation.RevisionID); completeErr != nil && completeErr != store.ErrMutationAlreadyCompleted {
					log.Printf("runtime: texture agent revision run %s: complete written mutation after failure: %v", rec.RunID, completeErr)
				}
				failedNoWrite = false
			} else {
				_ = rt.store.FailAgentMutation(persistCtx, rec.RunID)
			}
		} else {
			_ = rt.store.FailAgentMutation(persistCtx, rec.RunID)
		}
		if failedNoWrite {
			if rec.Metadata == nil {
				rec.Metadata = map[string]any{}
			}
			rec.Metadata["texture_revision_failed_no_write"] = true
		}
		if docID, _ := rec.Metadata["doc_id"].(string); docID != "" {
			failPayload, _ := json.Marshal(map[string]string{
				"doc_id":  docID,
				"loop_id": rec.RunID,
				"error":   err.Error(),
			})
			rt.emitTextureAgentEvent(persistCtx, rec, types.EventTextureAgentRevisionFailed,
				events.CauseProviderFailure, failPayload)
		}
	}
	if state.Terminal() && !metadataBoolValue(rec.Metadata, "texture_revision_failed_no_write") {
		rt.reconcileCompletedTextureRun(rec)
	}

	log.Printf("runtime: run %s → %s: %v", rec.RunID, state, err)

}

// providerResult returns fallback result text when a completed provider
// execution did not populate rec.Result directly. This text is run output only;
// texture document revisions are materialized exclusively by Texture write tools.
func (rt *Runtime) providerResult() string {
	if sp, ok := rt.provider.(*provider.StubProvider); ok {
		return sp.Result
	}
	return "Run completed."
}

const runMetadataTrajectoryID = "trajectory_id"

// ensureTrajectoryID guarantees that metadata carries a trajectory_id, falling
// back to parent metadata (or parent RunID) when inherited. The trajectory_id
// is the unit that spans prompt-bar → conductor → texture → workers → further
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

	rt.appendTraceEvent(ctx, evRec)

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
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		return err
	}
	rt.appendTraceEvent(ctx, evRec)
	return nil
}

// appendTraceEvent projects the runtime event record into the canonical trace
// observability schema and persists it to the mounted trace store. This is
// additive: it runs after the existing store append and never changes request
// handling. Failures (including a nil store) are logged and swallowed so a Dolt
// outage degrades gracefully — the event bus and existing recording continue.
func (rt *Runtime) appendTraceEvent(ctx context.Context, evRec *types.EventRecord) {
	if rt == nil || rt.traceStore == nil || evRec == nil {
		return
	}
	tev := trace.FromEventRecord(evRec)
	if err := rt.traceStore.Append(ctx, &tev); err != nil {
		log.Printf("runtime: trace store append %s: %v", evRec.EventID, err)
	}
}

// activeRunByAgent is the store-backed replacement for the old in-memory
// residentRunByAgent. It queries the store for the latest executing run
// (pending or running, NOT blocked) for an agent. Blocked runs are excluded
// because they are not actively executing and should be replaced, not reused.
func (rt *Runtime) activeRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, nil
	}
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return types.RunRecord{}, false, nil
	}
	rec, err := rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	// Exclude blocked runs — they are not actively executing and should
	// be replaced by a fresh activation, not reused.
	if rec.State == types.RunBlocked {
		return types.RunRecord{}, false, nil
	}
	return rec, true, nil
}
