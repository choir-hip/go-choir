package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/desktopstate"
	"github.com/yusefmosiah/go-choir/internal/imageblob"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	"github.com/yusefmosiah/go-choir/internal/promptstore"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/workitem"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/qdrant"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/steptrace"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/updater"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// ErrPromptCommandConflict marks reuse of a public command identity with a
// different request. Public handlers map it to a durable 409 refusal.
var ErrPromptCommandConflict = errors.New("prompt command conflict")

// ErrPreGenesis marks a write on a computer with no canonical genesis. Public
// handlers map it to a 503 "computer initializing" refusal; the computer is
// repairable via the owner-scoped bootstrap-chain route.
var ErrPreGenesis = errors.New("computer is pre-genesis")

// Runtime is the core runtime engine that manages run lifecycle, event
// emission, and health state. It persists all state through
// the store so that run handles and events survive autoputer process restarts
// (VAL-RUNTIME-010).
type Runtime struct {
	// recordsRunStart holds each running desk run's cursor at its first
	// cell (run id -> int64): choir.Records() shows the run every record
	// past it (B13).
	recordsRunStart sync.Map
	// recordsRunShown holds the highest record seq a choir.Records() call
	// in each running desk run returned (run id -> int64): a cell's commit
	// marks read only through it.
	recordsRunShown sync.Map
	// recordsFailedTurns counts each management desk's failed turns in a
	// row (inbox -> int), for the backoff of the wake that retries the
	// records a failed turn left unread. In memory: a restart resets it.
	recordsFailedTurns sync.Map
	// jobWakes holds each engineering run's capsule-job wake (run id ->
	// chan struct{}): a job's end signals it (capsule_jobs.go).
	jobWakes sync.Map
	// jobTable overrides the executor's job view for the completion guard
	// (tests).
	jobTable capsuleJobTable
	// storyLibraryShown holds the story-library head each running desk run
	// last read with choir.Stories() (run id -> int64), stamped on its
	// texture:new commits (management delegation R2-5).
	storyLibraryShown sync.Map
	// calibration caches each computer's calibration snapshot for the
	// ICRL digest (calibration_feedback.go); in memory, rebuilt on demand.
	calibration calibrationCache
	// managementTaskStartMu serializes the management task slot starter
	// (R2-6), so two wakes cannot both take the last slot.
	managementTaskStartMu sync.Mutex

	cfg         provideriface.Config
	store       *store.Store
	bus         *events.EventBus
	provider    provideriface.Provider
	promptStore *promptstore.Store

	// kernelMode, when true, drains the durable actor-wake outbox into actor
	// updates. The live boot-time worker-update sweep remains unchanged; the
	// kernel projector never binds or otherwise mutates resident runs.
	kernelMode    bool
	projectorStop chan struct{}
	// sourceSweepStop ends the periodic lost-wake sweep of source schedules.
	sourceSweepStop chan struct{}

	runningMu sync.Mutex
	running   map[string]context.CancelFunc // loop_id → cancel function
	healthMu  sync.Mutex
	health    types.RuntimeHealthState
	// bootInProgress is set only while Runtime.Start is executing. Health
	// probes that run while HTTP is already up must not JSON_EXTRACT/list
	// og_objects until boot sweeps finish; they contend with Management rewarm
	// and OOM the 4 GiB guest. Default false so tests and frozen historical
	// processor records still count durable running runs.
	bootInProgress atomic.Bool
	// lifecycleWorkReconcileMu makes the active-run check and replacement
	// creation one process-local critical section across terminal hooks and
	// boot/periodic sweeps.
	lifecycleWorkReconcileMu sync.Mutex
	// workspaceReplaceMu serializes the one-shot VM-local workspace
	// replacement so concurrent product-path calls cannot interleave
	// quarantine and OpenFresh.
	workspaceReplaceMu sync.Mutex
	// engineeringAssignmentOpenMu serializes the per-runtime open/spawn/bind
	// saga so a duplicate revision wake cannot drive a second saga against an
	// in-flight durable open (each delivery's cleanup would otherwise write
	// divergent fate intents on the same deterministic command identity).
	engineeringAssignmentOpenMu sync.Mutex
	// productAPISrv is the canonical product API route table the management
	// desk's choir.ProductAPI verb serves through (SMG). Bound at autoputer
	// boot via SetProductAPIHandler; nil in tests that never call the verb.
	// productAPIComputerID is this computer's own boot identity
	// (CHOIR_COMPUTER_ID), bound with productAPISrv. ProductAPI requests
	// carry it as the computer binding; cell input never supplies it.
	productAPIMu         sync.RWMutex
	productAPISrv        http.Handler
	productAPIComputerID string
	wg                   sync.WaitGroup
	toolRegistry         *toolregistry.ToolRegistry
	toolProfiles         map[string]*toolregistry.ToolRegistry
	// deskWorkers owns the host session workers for non-capsule desks on the
	// in-cell carrier (R3b). Lazy; nil until the first desk_go_eval call.
	deskWorkers   *deskSessionWorkers
	deskWorkersMu sync.Mutex
	// researchDeps are the search/HTTP/source clients bound at
	// tool-install for the desk cell's host-mediated egress verbs
	// (choir.Read → StreamBrokerEgress → deskEgress; choir.SearchStart →
	// startSearch, desk_search.go). nil until
	// InstallDefaultAgentTools builds them.
	researchDeps *researchtools.Dependencies
	// commitCaptureArrival commits a source poll's arrival record; nil
	// commits it through the store. Tests bind a failing commit to pin the
	// pending-arrival list (L2a A5, defect 2).
	commitCaptureArrival func(ctx context.Context, req types.CommitInboxRecordsRequest) error

	modelPolicy *modelpolicy.Manager
	// researchChildCalls spaces every sub-RLM's model calls on this
	// computer: research children and management tasks
	// (subRLMCallSpacer, research_children.go).
	researchChildCalls     *callSpacer
	researchChildCallsOnce sync.Once
	qdrantPipelineMu       sync.Mutex
	qdrantPipeline         *qdrant.Pipeline
	qdrantPipelineInitErr  error

	// dispatchActor is the function hook that the actor runtime adapter
	// sets. When the business logic needs to start a run or wake an agent,
	// it calls this function. If nil, activate() panics — there is no
	// fallback path. The actor runtime is the only execution substrate.
	dispatchActor func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error
	// dispatchActorRedrive is the outbox-sweep delivery hook: identical to
	// dispatchActor except a caller asserts the wake re-drives an open
	// obligation, so the adapter may mint a salted tape generation when the
	// deterministic update_id was already consumed.
	dispatchActorRedrive func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error
	// scheduleActor durably appends a deferred actor occurrence. It is bound
	// only in kernel mode, where NotBefore is interpreted by the dispatcher.
	scheduleActor        func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string, notBefore time.Time) error
	scheduleActorRedrive func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string, notBefore time.Time) error
	// admissionNow is the clock engineering admission reads to decide a
	// queued request's expiry. Nil is the wall clock; tests set it so expiry
	// does not depend on how fast the test runs.
	admissionNow func() time.Time

	desktopState    *desktopstate.Handler
	content         *contentowner.Service
	capsuleExecutor *capsule.Executor
	// steps tracks the named in-flight steps of long operations for
	// /api/runtime/observability (step_trace.go); stepsOnce creates it.
	steps     *steptrace.Tracker
	stepsOnce sync.Once
	// textureCellAuthorizer commits a Texture cell's turn (choir.Write,
	// Done, Wait, Reject) through the canonical ApplyTextureTurn
	// transaction. Bound by autoputer to the texture lifecycle owner —
	// the desk-cell carrier cannot import the owner, so this is an
	// interface seam (same shape as the actor-dispatch hooks).
	textureCellAuthorizer TextureCellAuthorizer
	// textureCaptures resolves a citation handle to its capture as a block
	// document (Cut 2 2a): the frame marks which sources are captured, and
	// the Write commit checks quotes against it. Nil until the capture store
	// is wired; then no source counts as captured.
	textureCaptures texturewrite.CaptureSource
	// stories publishes publish-auto story revisions and their support
	// verdicts to the platform, off the commit's path (Cut 2 2b).
	stories storyPublishing
	// textureCells records what each running Texture activation's cells
	// staged; the activation's end answers an unanswered trigger from it
	// (answerIdleTextureTrigger).
	textureCellsMu sync.Mutex
	textureCells   map[string]textureActivationCells
	// bootedAt is when this runtime started. The restart reconciler treats
	// only pre-bind assignments opened before it as restart strands; one this
	// boot opened belongs to its pending spawn saga.
	bootedAt time.Time
	// hibernate is the S3 snapshot-hibernate work admission fence
	// (hibernate.go).
	hibernate hibernateFence
	// replayProbeAfterCapture is a test seam: it runs after the
	// replay-completeness probe captures its head and live state.
	replayProbeAfterCapture func()
	// candidateJudge is the decision model (Jev): support judgement and
	// intake labels.
	candidateJudge CandidateJudge
	// system1Mu serializes choir.System1 calls, so a cap check, its request
	// record and the judge call are one step (system1_verb.go).
	system1Mu sync.Mutex
	// support is the support-judgement job state (support_judge.go).
	support supportJudging
	// events is the event layer's queue and index (event_layer.go).
	events eventLayer

	assignmentHandleResolver interface {
		AssignmentHandle(string, string) (string, error)
	}
	assignmentReceiptResolver interface {
		ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error)
	}
	assignmentLookup interface {
		GetEngineeringAssignment(context.Context, string, string, string, uint64) (types.EngineeringAssignment, error)
	}
	assignmentRuntime assignmentCapsuleRuntime
	bootLog           *bootLogRing
	// storeStall rate-limits /health's store_stalled holder log
	// (api_health_stall_holder.go).
	storeStall            storeStallCapture
	capsuleBuilder        *transaction.TransactionBuilder
	eventAppender         *computerevent.ComputerEventAppender
	selfdevOperations     *selfdev.Store
	privateArtifactCipher *computerevent.PrivateArtifactCipher
	selfdevUpdater        *updater.Client
	// eventPayloadReader fetches event payloads by digest (corpusd CAS). The
	// platform-update boot sweep needs it: an accepted event pins its offer,
	// and resuming that update after the self-restart reads it back.
	eventPayloadReader computerevent.ArtifactReader
	selfdevVerifier    *receiptsigner.Client
	// selfdevBallotSigner is the guest-core signer the standing-consensus
	// driver signs management's reviewer ballot with (P4); nil keeps the
	// guest from ever asking corpusd's decision poster.
	selfdevBallotSigner standingBallotSigner
	// selfdevStandingPoll overrides how often the standing-consensus driver
	// reads a queued or running host build (tests).
	selfdevStandingPoll time.Duration
	selfdevControl      *selfdev.GuestCredentials
	// selfdevHostBuilder overrides the corpusd host build client (P5);
	// nil derives it from eventPayloadReader. Tests inject a fake.
	selfdevHostBuilder selfdevHostBuilder
	// selfdevGuestImageManifest overrides CHOIR_GUEST_IMAGE_MANIFEST, the
	// booted base a host build is bound to.
	selfdevGuestImageManifest string
	// selfdevRestartCurrent overrides the updater's restart-into-current
	// call a restore makes when it changed the exec identity.
	selfdevRestartCurrent func(context.Context) error
	ownerRecoveryControl  *selfdev.GuestCredentials
	selfdevRoute          *vmctl.Client
	selfdevRouteOwnerID   string
	selfdevRouteDesktopID string
	selfdevUpdaterRoot    string
	selfdevComputerID     string
	selfdevRealizationID  string
	selfdevStartupMarker  string
	// bootRestart is how this boot began: a consumed planned-restart
	// marker, or a crash (bootRestartPlanned false).
	bootRestart        PlannedRestart
	bootRestartPlanned bool
	// telemetryDrainDeadline bounds the planned-restart telemetry drain; zero
	// means the default (telemetry_batching.go).
	telemetryDrainDeadline time.Duration
	// hostRestartTelemetryRestore releases the hold on synchronous mode the
	// host's planned-restart request took; nil when no host request holds it.
	// hostRestartDrainFailed records that the hold's drain lost records.
	hostRestartTelemetryMu      sync.Mutex
	hostRestartTelemetryRestore func()
	hostRestartDrainFailed      bool
	// deliveryHooksMu guards the actor delivery hooks against the projector
	// goroutine reading them while they are set.
	deliveryHooksMu sync.RWMutex
	// actorObligations reads the actor tape for the "what is owed" surface
	// (bound by the actor runtime adapter; guarded by deliveryHooksMu).
	actorObligations func(context.Context) (ActorObligations, error)
	// wakeAuditInterval overrides actorWakeAuditInterval (tests only).
	wakeAuditInterval time.Duration
	// wakeRetries bounds failed actor wake dispatches (SL slice 2).
	wakeRetries wakeRetryTracker
	// selfdevStartupMu guards the startup-manifest fields below:
	// recordStartupManifest rewrites them after a restore or rematerialize
	// while /health and the effective-release report read them from other
	// goroutines (docs/problems/p6-effective-release-report-data-race-ci-red-2026-10-15.md).
	// Read them through startupManifest(), write through setStartupManifest().
	selfdevStartupMu            sync.RWMutex
	selfdevStartupReleaseDigest string
	selfdevStartupRelease       *effectiveReleaseIdentity
	selfdevStartupEventSchema   uint64
	selfdevStartupReducer       uint64
	// Release identity and source adoption (P6/P8,
	// docs/reports/p6-p8-release-adopt-2026-10-15.md): the guest's source
	// repository (CHOIR_CAPSULE_SOURCE_ROOT unless set), the per-release
	// tree cache, the published-release reader and the owner-prompt
	// submitter the adoption request goes through.
	releaseSourceDir     string
	effectiveTreeCompute func(ctx context.Context, repo, commit string, patch []byte) (string, error)
	effectiveBuildCommit func() string
	effectiveTreeMu      sync.Mutex
	effectiveTrees       map[string]*effectiveTreeEntry
	releaseSource        ReleaseSource
	promptSubmitter      PromptSubmitter
	// managementReconcileMu serializes persistent-Management creation
	// across every reconcile entry path (inbox continuation, boot rewarm).
	managementReconcileMu sync.Mutex
	selfdevMaterializeMu  sync.Mutex
	// selfdevCrashInterrupted names the self-development operations a crash
	// boot found decided but unsettled (L1a-3b): the reconciler settles them
	// from the updater journal and never calls the updater for them. Guarded
	// by selfdevMaterializeMu.
	selfdevCrashInterrupted map[string]bool
	// selfdevReconcilePending is the M7 derivable-continuation signal: the
	// canonical post-commit observer sets it and a single drain loop
	// collapses bursts into one reconcile per settlement boundary.
	selfdevReconcilePending atomic.Bool
	// selfdevDecisionBeforeTransition is a test seam, nil in production: the
	// decision handler calls it after the decision event is final and before
	// it moves the operation, so a test can let the post-commit observer win
	// that race.
	selfdevDecisionBeforeTransition func(ctx context.Context, operationID string)
	// restoreBaseSource overrides verified-base resolution for recovery
	// (rematerialize, restore, replay-completeness). Nil builds a platform
	// source from CorpusdURL plus guest credentials per call; tests inject
	// a fake. There is no genesis fallback in either case.
	restoreBaseSource projectionbase.BaseSource
}

// New creates a new Runtime with the given config, store, event bus, and
// provider. The runtime is idle until Start is called.
// If a tool registry is provided, the runtime will use the tool-calling
// loop for run execution instead of the simple provider bridge path.
func New(cfg provideriface.Config, s *store.Store, bus *events.EventBus, provider provideriface.Provider, opts ...RuntimeOption) *Runtime {
	cfg = provideriface.NormalizeConfig(cfg)
	rt := &Runtime{
		bootedAt:    time.Now().UTC(),
		cfg:         cfg,
		store:       s,
		bus:         bus,
		provider:    provider,
		health:      types.HealthReady,
		running:     make(map[string]context.CancelFunc),
		promptStore: promptstore.New(cfg.PromptRoot),
		modelPolicy: modelpolicy.NewManager(modelpolicy.ManagerConfig{
			PolicyPath:     cfg.ModelPolicyPath,
			ProviderConfig: cfg,
			Provider:       provider,
			ImageStore:     imageblob.New(imageblob.DirForStorePath(cfg.StorePath)),
		}),
	}
	for _, opt := range opts {
		opt(rt)
	}
	if operations, err := selfdev.NewStore(s, s); err == nil {
		rt.selfdevOperations = operations
		if s != nil && s.ProjectionTapeBound() {
			operations.BindProjectionSink(s)
		}
	}
	return rt
}

// SetDispatchActor sets the function hook that dispatches actor messages.
// The actor runtime adapter calls this during construction. When set,
// activate() sends actor messages through this function. If not set,
// activate() panics — there is no fallback path.
func (rt *Runtime) SetDispatchActor(fn func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error) {
	rt.deliveryHooksMu.Lock()
	defer rt.deliveryHooksMu.Unlock()
	rt.dispatchActor = fn
}

// TextureCellAuthorizer commits a staged full-RLM texture authoring intent
// (R3d: a choir texture cell act) through the canonical Texture turn
// transaction. autoputer binds the texture lifecycle owner; the desk-cell
// reducer calls it — the desk carrier never imports the owner package, so
// this is the structural seam.
type TextureCellAuthorizer interface {
	// CommitCellTextureAuthor commits bodyJSON — a yaegikernel.TextureTurn
	// (Write, Done, Wait, Reject, or the runtime's Idle, with the dispatch
	// fold) — for the desk activation run rec, through the atomic
	// ApplyTextureTurn / lifecycle commit. revisionIdentity is the deterministic
	// per-cell idempotency identity the reducer derives so a replayed cell
	// replays the same commit rather than minting a second head. Returns a
	// JSON receipt describing the commit (revision_id/command/result).
	CommitCellTextureAuthor(ctx context.Context, rec *types.RunRecord, bodyJSON string, revisionIdentity string) (string, error)
}

// SetTextureCellAuthorizer binds the texture lifecycle owner's cell-authoring
// commit path (R3d). autoputer calls it after building the owner handler.
func (rt *Runtime) SetTextureCellAuthorizer(a TextureCellAuthorizer) {
	rt.textureCellAuthorizer = a
}

// SetTextureCaptures binds the capture store Texture cites from.
func (rt *Runtime) SetTextureCaptures(c texturewrite.CaptureSource) {
	rt.textureCaptures = c
}

// TextureCaptures is the capture store Texture cites from, or nil.
func (rt *Runtime) TextureCaptures() texturewrite.CaptureSource {
	if rt == nil {
		return nil
	}
	return rt.textureCaptures
}

// SetScheduleActor sets the kernel-only hook for durable not-before events.
// The adapter owns the actor tape and binds this alongside dispatchActor.
func (rt *Runtime) SetScheduleActor(fn func(context.Context, string, string, string, string, string, string, string, time.Time) error) {
	rt.deliveryHooksMu.Lock()
	defer rt.deliveryHooksMu.Unlock()
	rt.scheduleActor = fn
}

// SetDispatchActorRedrive binds the re-drive delivery hook used only by the
// actor-wake outbox sweep. When unset the sweep falls back to dispatchActor.
func (rt *Runtime) SetDispatchActorRedrive(fn func(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error) {
	rt.deliveryHooksMu.Lock()
	defer rt.deliveryHooksMu.Unlock()
	rt.dispatchActorRedrive = fn
}

// SetScheduleActorRedrive binds the deferred re-drive hook used only by the
// actor-wake outbox sweep. When unset the sweep falls back to scheduleActor.
func (rt *Runtime) SetScheduleActorRedrive(fn func(context.Context, string, string, string, string, string, string, string, time.Time) error) {
	rt.deliveryHooksMu.Lock()
	defer rt.deliveryHooksMu.Unlock()
	rt.scheduleActorRedrive = fn
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
	if err := rt.dispatchActor(context.Background(), rec.OwnerID, rec.ComputerID, agentID, "initial_dispatch", rec.RunID, trajectoryID, ""); err != nil {
		log.Printf("runtime: activate dispatch for run %s: %v", rec.RunID, err)
	}
	// Central arm point: a freshly minted pending persistent-Management run whose
	// initial_dispatch is lost (the boot-window race) has no watchdog and no
	// retry authority anywhere else; the fresh-mint watchdog re-drives it
	// through a recovery occurrence. Non-Management and reactivated runs are
	// filtered inside (the reactivation resume watchdog owns the latter).
	rt.armFreshMintManagementResumeWatchdog(rec)
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
	runCtx, cancel := context.WithTimeout(ctx, rt.activationBudgetForRun(rec))

	rt.runningMu.Lock()
	stored, err := rt.getRunForComputer(context.Background(), rec.OwnerID, rec.RunID)
	if err == nil && stored.State.Terminal() {
		rt.runningMu.Unlock()
		cancel()
		*rec = stored
		return
	}
	rt.running[rec.RunID] = cancel
	// Register the activation before releasing runningMu. Stop acquires the
	// same lock before waiting, so it cannot observe an activation after
	// WaitGroup.Wait has already begun.
	rt.wg.Add(1)
	rt.runningMu.Unlock()
	if deadline, ok := runCtx.Deadline(); ok {
		rt.scheduleContinuation(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID,
			activationBudgetDeadlineUpdateKind, rec.RunID, metadataStringValue(rec.Metadata, runMetadataTrajectoryID), "", deadline)
	}
	// No process-local deadline continuation. The durable
	// activation_budget_deadline scheduled above survives a restart and is the
	// sole authority that terminalizes the run (HandleActivationBudgetDeadline
	// is idempotent). A process-local AfterFunc would be progress invisible to
	// the tape — deleted under the "boot causes no work" ontology ruling.
	defer cancel()

	rt.executeActivation(runCtx, &runRec)
	*rec = runRec
}

// activationBudgetForRun bounds one activation. A management task's
// deadline is its activation budget (O7): the durable
// activation_budget_deadline closes its run, and the task, with reason
// deadline.
func (rt *Runtime) activationBudgetForRun(rec *types.RunRecord) time.Duration {
	budget := rt.cfg.ActivationBudget
	if isManagementTaskRun(rec) && (budget <= 0 || budget > managementTaskDeadline) {
		budget = managementTaskDeadline
	}
	return budget
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
	case agentprofile.Management:
		if ownerID != "" {
			return persistentManagementAgentID(ownerID)
		}
	case agentprofile.Texture:
		if docID := metadataStringValue(metadata, "doc_id"); docID != "" {
			return currentTextureAgentID(docID)
		}
	case frozenProcessorProfile:
		if key := metadataStringValue(metadata, runMetadataProcessorKey); key != "" {
			return "processor:" + safeRefPart(key)
		}
	case frozenReconcilerProfile:
		if scope := metadataStringValue(metadata, runMetadataReconcilerScope); scope != "" {
			return "reconciler:" + safeRefPart(scope)
		}
	}
	return uuid.New().String()
}

func persistentManagementAgentID(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return agentprofile.Management
	}
	return agentprofile.Management + ":" + ownerID
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
	if profile == agentprofile.Management || profile == frozenProcessorProfile || profile == frozenReconcilerProfile {
		return agentID
	}
	return ""
}

func (rt *Runtime) EnsurePersistentManagementAgent(ctx context.Context, ownerID string) (types.AgentRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AgentRecord{}, fmt.Errorf("runtime store unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return types.AgentRecord{}, fmt.Errorf("owner_id is required")
	}
	now := time.Now().UTC()
	agentID := persistentManagementAgentID(ownerID)
	rec := types.AgentRecord{
		AgentID:    agentID,
		OwnerID:    ownerID,
		ComputerID: rt.cfg.ComputerID,
		Profile:    agentprofile.Management,
		Role:       agentprofile.Management,
		ChannelID:  agentID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := rt.store.UpsertAgent(ctx, rec); err != nil {
		return types.AgentRecord{}, fmt.Errorf("persist persistent super agent: %w", err)
	}
	return rec, nil
}

func resolveRunIdentity(ownerID, computerID string, metadata map[string]any, parent *types.RunRecord) (types.AgentRecord, map[string]any, error) {
	metadata = cloneMetadata(metadata)
	rawProfile := metadataStringValue(metadata, runMetadataAgentProfile)
	profile := rawProfile
	if profile == "" {
		// rawProfile is empty here, so metadata carries no profile token;
		// inherit the parent profile unless the run is Texture-scoped.
		if parent != nil && strings.TrimSpace(parent.AgentProfile) != "" {
			profile = parent.AgentProfile
		} else {
			profile = agentProfileForRun(&types.RunRecord{Metadata: metadata})
		}
	}
	resolvedProfile, err := agentprofile.Canonical(profile)
	if err != nil {
		return types.AgentRecord{}, nil, fmt.Errorf("resolve run identity: %w", err)
	}
	profile = resolvedProfile
	if profile == frozenProcessorProfile || profile == frozenReconcilerProfile {
		return types.AgentRecord{}, nil, fmt.Errorf("resolve run identity: retired profile %q cannot start a new run", profile)
	}
	if strings.EqualFold(strings.TrimSpace(rawProfile), agentprofile.Texture) {
		profile = agentprofile.Texture
	}
	rawRole := metadataStringValue(metadata, runMetadataAgentRole)
	role := rawRole
	if role == "" {
		role = profile
	} else {
		resolvedRole, err := agentprofile.Canonical(role)
		if err != nil {
			return types.AgentRecord{}, nil, fmt.Errorf("resolve run identity: %w", err)
		}
		role = resolvedRole
	}
	if strings.EqualFold(strings.TrimSpace(rawRole), agentprofile.Texture) {
		role = agentprofile.Texture
	}
	if profile == "" {
		return types.AgentRecord{}, nil, fmt.Errorf("resolve run identity: empty profile for owner %s", ownerID)
	}
	agentID := defaultAgentID(profile, ownerID, metadata)
	channelID := defaultChannelID(profile, metadata, parent, agentID)
	metadata[runMetadataAgentProfile] = profile
	metadata[runMetadataAgentRole] = role
	return types.AgentRecord{
		AgentID:    agentID,
		OwnerID:    ownerID,
		ComputerID: computerID,
		Profile:    profile,
		Role:       role,
		ChannelID:  channelID,
	}, metadata, nil
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

// WithDesktopStateOwner composes Runtime tools with the canonical desktop-state
// owner without giving Runtime direct state persistence authority.
func WithDesktopStateOwner(owner *desktopstate.Handler) RuntimeOption {
	return func(rt *Runtime) {
		rt.desktopState = owner
	}
}

// WithContentService composes Runtime tools and Texture integrations with the
// canonical content owner. Runtime never constructs a fallback owner.
func WithContentService(service *contentowner.Service) RuntimeOption {
	return func(rt *Runtime) {
		rt.content = service
	}
}

// WithCapsuleExecutor binds the guest-local capsule authority. It is omitted
// outside the Linux guest; capsule tools then remain uninstalled.
func WithCapsuleExecutor(executor *capsule.Executor) RuntimeOption {
	return func(rt *Runtime) {
		rt.capsuleExecutor = executor
		rt.assignmentReceiptResolver = executor
		if executor != nil {
			executor.SetStepTrace(rt.stepTrace())
		}
		if executor != nil {
			rt.capsuleBuilder = transaction.NewTransactionBuilder(transaction.NewClassifier())
			// A capsule job's end commits its record and wakes its run
			// (capsule_jobs.go).
			executor.SetJobCompletionHandler(rt.handleCapsuleJobDone)
		}
	}
}

// WithPrivateArtifactCipher binds the guest-root private artifact authority.
func WithPrivateArtifactCipher(cipher *computerevent.PrivateArtifactCipher) RuntimeOption {
	return func(rt *Runtime) {
		rt.privateArtifactCipher = cipher
	}
}

// WithEventPayloadReader installs the event-payload fetcher (corpusd CAS
// reader). Required for the post-restart platform-update resume sweep.
func WithEventPayloadReader(reader computerevent.ArtifactReader) RuntimeOption {
	return func(rt *Runtime) { rt.eventPayloadReader = reader }
}
func WithSelfDevelopmentUpdater(client *updater.Client, root, computerID, realizationID string) RuntimeOption {
	return func(rt *Runtime) {
		rt.selfdevUpdater = client
		rt.selfdevUpdaterRoot = filepath.Clean(strings.TrimSpace(root))
		rt.selfdevComputerID = strings.TrimSpace(computerID)
		rt.selfdevRealizationID = strings.TrimSpace(realizationID)
		if manifest, err := updater.ReadCurrentManifest(rt.selfdevUpdaterRoot); err == nil {
			rt.setStartupManifest(manifest, effectiveReleaseFromManifest(manifest))
		}
	}
}

func WithSelfDevelopmentVerifier(client *receiptsigner.Client) RuntimeOption {
	return func(rt *Runtime) {
		rt.selfdevVerifier = client
	}
}

// WithSelfDevelopmentBallotSigner wires the guest-core signer for the
// standing-consensus reviewer ballot (P4).
func WithSelfDevelopmentBallotSigner(signer standingBallotSigner) RuntimeOption {
	return func(rt *Runtime) {
		rt.selfdevBallotSigner = signer
	}
}

// WithSelfDevelopmentControl wires platform-control credentials for signed mode
// reads. It does not set mode, arm the outbox, or start materialization by itself.
func WithSelfDevelopmentControl(credentials *selfdev.GuestCredentials) RuntimeOption {
	return func(rt *Runtime) {
		rt.selfdevControl = credentials
	}
}

// WithOwnerRecoveryControl wires platform-control credentials for owner-recovery
// checkpoint publication only. It does not enable the effects mode/materializer
// surface that WithSelfDevelopmentControl gates.
func WithOwnerRecoveryControl(credentials *selfdev.GuestCredentials) RuntimeOption {
	return func(rt *Runtime) {
		rt.ownerRecoveryControl = credentials
	}
}

func WithSelfDevelopmentRoute(client *vmctl.Client, ownerID, desktopID string) RuntimeOption {
	return func(rt *Runtime) {
		rt.selfdevRoute = client
		rt.selfdevRouteOwnerID = strings.TrimSpace(ownerID)
		rt.selfdevRouteDesktopID = strings.TrimSpace(desktopID)
	}
}

func WithComputerEventAppender(appender *computerevent.ComputerEventAppender) RuntimeOption {
	return func(rt *Runtime) {
		rt.eventAppender = appender
	}
}

// Start begins runtime boot recovery. On boot, legacy in-process activations
// are passivated. Lifecycle activations are re-dispatched from their canonical
// pending/running run projections, then durable backlog and assigned work are
// swept to re-warm cold actors.
func (rt *Runtime) Start(ctx context.Context) {
	rt.bootInProgress.Store(true)
	defer rt.bootInProgress.Store(false)
	// Telemetry batching is decided before the first event of this process, so
	// the sequence reseed gap applies to every number seeded after a crash.
	rt.configureTelemetryBatching(ctx)
	rt.configureGuestCheckpoint(ctx)
	if rt.maintenanceHeld() {
		log.Printf("runtime: maintenance hold active (RUNTIME_MAINTENANCE_HOLD=1); refusing run admission + agent rewake while held")
		return
	}
	bootPhase := func(name string, fn func()) {
		started := time.Now()
		log.Printf("runtime: boot phase begin %s", name)
		fn()
		log.Printf("runtime: boot phase end %s dur=%s", name, time.Since(started).Round(time.Millisecond))
	}
	// A crash restart never resumes work: a markerless boot closes every
	// pre-boot run with the fate interrupted_by_restart. A planned restart
	// (marker consumed before Start) parks them for its one resume.
	if _, planned := rt.BootWasPlannedRestart(); planned {
		bootPhase("passivate_interrupted_activations", func() { rt.passivateInterruptedActivations(ctx) })
	} else {
		bootPhase("close_pre_boot_runs_after_crash", func() { rt.closePreBootRunsAfterCrash(ctx) })
		bootPhase("close_open_rounds_after_crash", func() { rt.closeOpenRoundsAfterCrash(ctx) })
	}
	bootPhase("engineering_assignment_capsules", func() { rt.reconcileEngineeringAssignmentCapsulesAfterRestart(ctx) })
	// P6: tell vmctl what this computer runs (background; never blocks boot).
	rt.reportEffectiveReleaseAsync("boot")
	bootPhase("platform_update_resume", func() {
		// Spawn only — the sweep itself must not run inside Start: the resumed
		// apply self-restarts this guest and the daemon health-probes /health,
		// which stays gated until Start returns. resumePendingPlatformUpdate
		// AfterReady waits out boot before re-driving the pending transition.
		go rt.resumePendingPlatformUpdateAfterReady(ctx)
	})
	bootPhase("selfdev_close_after_crash", func() { rt.closeSelfDevelopmentOperationsAfterCrash(ctx) })
	bootPhase("source_schedules_after_restart", func() { rt.recoverSourceSchedulesAtBoot(ctx) })
	// The sweep repairs a wake lost while the computer stays up (an outage that
	// poisons wakes); its first pass is one interval after boot.
	rt.startSourceScheduleSweep(ctx)
	bootPhase("selfdev_materialization_reconcile", func() { rt.triggerSelfDevelopmentReconcile() })
	// M7 derivable continuations: every committed canonical event may carry
	// a self-development settlement boundary (verification -> approval,
	// decision -> materialization, rollback request -> rollback). The
	// appender fires this observer once per committed event; the trigger
	// drains under a coalesced pending flag, and the reconciler's state
	// query is the gate — no external API call is needed to advance an op.
	if rt.eventAppender != nil {
		rt.eventAppender.SetPostCommitObserver(func(computerevent.EventKind) {
			rt.triggerSelfDevelopmentReconcile()
		})
	}
	// Boot does not directly deliver work. Restart-resumption sweeps that
	// re-fired committed work (lifecycle rewarm, persistent-Management resume,
	// passivated spawned work, open work items, mailbox backlog) are deleted
	// under the owner ruling. The surviving phases never directly deliver;
	// the actor-wake outbox projector is the sole post-restart delivery path.
	rt.migrateActorWakeOutboxAsync(ctx)
	rt.startProjector(ctx)
	// Best-effort: ensure the production Qdrant collection exists so the
	// semantic dedup pass on ingestion has a target. Runs asynchronously so
	// a slow or unreachable Qdrant cannot block runtime startup; the dedup
	// path also ensures the collection lazily on first use.
	go rt.ensureProductionQdrantCollectionBestEffort(ctx)
	// The corpus search cache is derived and rebuilt here so the first
	// research search after a boot does not read every story's head
	// (docs/problems/research-more-slow-and-hidden-2026-10-21.md E1).
	go rt.warmCorpusSearchIndex(ctx)
	log.Printf("runtime: started (autoputer=%s)", rt.cfg.ComputerID)
}

func (rt *Runtime) warmCorpusSearchIndex(ctx context.Context) {
	if rt.store == nil {
		return
	}
	started := time.Now()
	docs, reads, err := rt.store.WarmSearchIndex(ctx)
	log.Printf("runtime: corpus search index warm docs=%d heads_read=%d dur=%s err=%v", docs, reads, time.Since(started).Round(time.Millisecond), err)
}

// maintenanceHeld reports whether the guest runtime was booted under a
// maintenance hold (RUNTIME_MAINTENANCE_HOLD=1), set by the host when it holds
// a computer. While held the runtime refuses run admission and skips every
// boot-time rewake/dispatch sweep so no run (self-dev or otherwise) is admitted
// or rewarmed and nothing is written; the runtime stays up for health and the
// bootstrap-chain route so the computer remains servable but mutation-fenced.
func (rt *Runtime) maintenanceHeld() bool {
	return os.Getenv("RUNTIME_MAINTENANCE_HOLD") == "1"
}

// MaintenanceHeld reports whether the runtime was booted under a maintenance
// hold. It is exported so holders of the Runtime (e.g. the Texture-owner boot
// reconcile) can treat the held state as a benign, mutation-fenced state rather
// than a fatal startup failure.
func (rt *Runtime) MaintenanceHeld() bool {
	return rt.maintenanceHeld()
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
	rt.stopProjector()
	rt.stopSourceScheduleSweep()
	rt.stopEventLayer()
	rt.runningMu.Lock()
	for runID, cancel := range rt.running {
		cancel()
		delete(rt.running, runID)
	}
	rt.runningMu.Unlock()

	rt.wg.Wait()

	// Seal the open telemetry window: a graceful stop loses none of it.
	if rt.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), rt.drainDeadline())
		if err := rt.store.DrainTelemetry(ctx); err != nil {
			log.Printf("runtime: stop: telemetry drain: %v", err)
		}
		cancel()
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
	profile := agentProfileForRun(rec)
	if profile == frozenProcessorProfile || profile == agentprofile.Texture || profile == agentprofile.Research || profile == agentprofile.Engineering {
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
	summaryProfile := agentProfileForRun(rec)
	return fmt.Sprintf("run=%s profile=%s requested_by=%s channel=%s processor_key=%s state=%s", rec.RunID, summaryProfile, strings.TrimSpace(rec.RequestedByRunID), strings.TrimSpace(rec.ChannelID), metadataStringValue(rec.Metadata, runMetadataProcessorKey), rec.State)
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
	// Maintenance-hold admission gate (Phase Fence): a computer booted under
	// RUNTIME_MAINTENANCE_HOLD=1 refuses run admission BEFORE any store mutation
	// while held, so no run (self-dev or otherwise) executes and nothing is
	// written. This is the guest-visible mirror of the host hold.
	if rt.maintenanceHeld() {
		return nil, fmt.Errorf("computer is under maintenance hold: run admission refused")
	}
	// Pre-genesis admission gate (PF-3 fix): a computer with an empty canonical
	// chain (no genesis_imported ever CAS'd) refuses run admission BEFORE any
	// store mutation. Unbounded write attempts would fail at Reduce(nil, ...)
	// with "invalid genesis" and re-mint into an endless failed-run churn; the
	// gate keeps the runtime up (health + bootstrap-chain route remain
	// servable) while refusing writes, so bootstrap-chain (owner authority) can
	// repair the computer. Test/unbound stores keep their direct path.
	if rt != nil && rt.store != nil && rt.store.ProjectionTapeBound() {
		if head, headErr := rt.store.Head(ctx, rt.cfg.ComputerID); headErr == nil && head == nil {
			return nil, fmt.Errorf("%w: run admission refused (bootstrap-chain required; no canonical genesis on the tape)", ErrPreGenesis)
		}
	}
	now := time.Now().UTC()
	runID := uuid.New().String()
	metadata = ensureDesktopID(metadata, nil, metadataStringValue(metadata, runMetadataDesktopID))
	agentRec, metadata, err := resolveRunIdentity(ownerID, rt.cfg.ComputerID, metadata, nil)
	if err != nil {
		return nil, err
	}
	agentRecProfile := agentRec.Profile
	if agentRecProfile == agentprofile.Conductor && metadataStringValue(metadata, "lifecycle_command_id") == "" {
		metadata["lifecycle_command_id"] = uuid.NewString()
	}
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	metadata = ensureTrajectoryID(metadata, nil, runID)
	role := firstNonEmptyString(metadataStringValue(metadata, runMetadataAgentRole), metadataStringValue(metadata, runMetadataAgentProfile))
	metadata = rt.modelPolicy.EnrichMetadata(ctx, ownerID, role, metadata)
	if model := metadataStringValue(metadata, modelpolicy.MetadataModel); model != "" {
		metadata[runMetadataModel] = model
		// Texture's AuthoringModel reads provider beside model.
		metadata["provider"] = metadataStringValue(metadata, modelpolicy.MetadataProvider)
	}
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
		ComputerID:       rt.cfg.ComputerID,
		State:            types.RunPending,
		Prompt:           prompt,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata:         metadata,
	}
	rec.TrajectoryID = trajectoryIDForRun(rec)
	existingAgent, existingAgentErr := rt.store.GetAgentByScope(ctx, ownerID, rt.cfg.ComputerID, rec.AgentID)
	if existingAgentErr == nil && existingAgent.LifecycleVersion > 0 {
		if rec.TrajectoryID == "" || existingAgent.ChannelID != rec.ChannelID ||
			existingAgent.Profile != rec.AgentProfile || existingAgent.Role != rec.AgentRole {
			return nil, fmt.Errorf("durable activation binding mismatch")
		}
		if err := persistLifecycleSubmittedRun(ctx, rt.store, rt.bus, rec, len(prompt)); err != nil {
			return nil, err
		}
	} else {
		if existingAgentErr != nil && !errors.Is(existingAgentErr, store.ErrNotFound) {
			return nil, fmt.Errorf("resolve run subject: %w", existingAgentErr)
		}
		rt.stampAndMintTrajectory(ctx, rec)
		if err := persistSubmittedRun(ctx, rt.store, rt.bus, agentRec, rec, len(prompt)); err != nil {
			return nil, err
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
	if commandID := strings.TrimSpace(metadataStringValue(metadata, "lifecycle_command_id")); commandID != "" {
		runID = uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{"choir:prompt-bar", ownerID, rt.cfg.ComputerID, commandID}, ":"))).String()
		existing, existingErr := rt.getRunForComputer(ctx, ownerID, runID)
		if existingErr == nil {
			if existing.Prompt != prompt || metadataStringValue(existing.Metadata, "lifecycle_command_id") != commandID {
				return nil, fmt.Errorf("%w: stored submission does not match request", ErrPromptCommandConflict)
			}
			return &existing, nil
		}
		if !errors.Is(existingErr, store.ErrNotFound) {
			return nil, fmt.Errorf("load prompt command replay: %w", existingErr)
		}
	}
	metadata = ensureDesktopID(metadata, nil, metadataStringValue(metadata, runMetadataDesktopID))
	metadata = ensureTrajectoryID(metadata, nil, runID)
	agentRec, metadata, err := resolveRunIdentity(ownerID, rt.cfg.ComputerID, metadata, nil)
	if err != nil {
		return nil, err
	}
	agentRecProfile := agentRec.Profile
	if agentRecProfile == agentprofile.Conductor && metadataStringValue(metadata, "lifecycle_command_id") == "" {
		metadata["lifecycle_command_id"] = uuid.NewString()
	}
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	agentRec.CreatedAt = now
	agentRec.UpdatedAt = now
	// Pre-genesis admission gate (same refusal as createRunWithMetadata): the
	// prompt-bar decision path persists the agent BEFORE StartRun admission,
	// so it needs its own gate or a pre-genesis submit surfaces a raw
	// invalid-genesis 500 instead of a clean 503 (SA slice 0).
	if rt != nil && rt.store != nil && rt.store.ProjectionTapeBound() {
		if head, headErr := rt.store.Head(ctx, rt.cfg.ComputerID); headErr == nil && head == nil {
			return nil, fmt.Errorf("%w: prompt-bar submit refused (bootstrap-chain required; no canonical genesis on the tape)", ErrPreGenesis)
		}
	}
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
		ComputerID:   rt.cfg.ComputerID,
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

func (rt *Runtime) getRunForComputer(ctx context.Context, ownerID, runID string) (types.RunRecord, error) {
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	rec, err := rt.store.GetRunByOwner(ctx, ownerID, runID)
	if err == nil {
		if strings.TrimSpace(rec.ComputerID) != "" &&
			(strings.TrimSpace(rec.TrajectoryID) != "" || metadataStringValue(rec.Metadata, runMetadataTrajectoryID) != "") &&
			strings.TrimSpace(rec.ComputerID) != computerID {
			return types.RunRecord{}, store.ErrNotFound
		}
		return rec, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return types.RunRecord{}, err
	}
	if computerID == "" {
		return types.RunRecord{}, store.ErrNotFound
	}
	return rt.store.GetLifecycleRun(ctx, ownerID, computerID, runID)
}

// GetRun returns a run by ID, scoped to the given owner. If the run does
// not exist or does not belong to the owner, it returns ErrNotFound
// (VAL-RUNTIME-006: caller-scoped).
func (rt *Runtime) GetRun(ctx context.Context, runID, ownerID string) (*types.RunRecord, error) {
	rec, err := rt.getRunForComputer(ctx, ownerID, runID)
	if err != nil {
		return nil, err
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
	requesterRec, err := rt.getRunForComputer(ctx, ownerID, requesterRunID)
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
	// A pinned model-policy overlay (e.g. an eval arm) covers the whole
	// trajectory: a child coagent inherits the requester's overlay when it does
	// not specify its own, so a Texture arm also pins the researchers it spawns.
	if strings.TrimSpace(metadataStringValue(metadata, modelpolicy.MetadataPolicyOverlayID)) == "" {
		if overlayID := strings.TrimSpace(metadataStringValue(requesterRec.Metadata, modelpolicy.MetadataPolicyOverlayID)); overlayID != "" {
			metadata[modelpolicy.MetadataPolicyOverlayID] = overlayID
		}
	}
	if slot := normalizeEngineeringSlot(metadataStringValue(metadata, runMetadataEngineeringSlot)); slot != "" {
		metadata[runMetadataEngineeringSlot] = slot
	}
	targetRaw := firstNonEmptyString(metadataStringValue(metadata, runMetadataAgentProfile), metadataStringValue(metadata, runMetadataAgentRole))
	targetProfile := ""
	if strings.TrimSpace(targetRaw) != "" {
		resolved, err := agentprofile.Canonical(targetRaw)
		if err != nil {
			return nil, fmt.Errorf("coagent run profile: %w", err)
		}
		targetProfile = resolved
	}
	if targetProfile == agentprofile.Engineering {
		return nil, fmt.Errorf("generic StartCoagentRun refuses all Engineering activation; use the authenticated persistent-Management assignment runtime")
	}
	metadata = ensureTrajectoryID(metadata, &requesterRec, runID)

	if rt.coagentSpawnBudgetApplies(&requesterRec) {
		coagentProfile := ""
		if raw := metadataStringValue(metadata, runMetadataAgentProfile); strings.TrimSpace(raw) != "" {
			resolved, err := agentprofile.Canonical(raw)
			if err != nil {
				return nil, fmt.Errorf("coagent run profile: %w", err)
			}
			coagentProfile = resolved
		}
		if coagentProfile == "" {
			if raw := metadataStringValue(metadata, runMetadataAgentRole); strings.TrimSpace(raw) != "" {
				resolved, err := agentprofile.Canonical(raw)
				if err != nil {
					return nil, fmt.Errorf("coagent run role: %w", err)
				}
				coagentProfile = resolved
			}
		}
		slot := normalizeEngineeringSlot(metadataStringValue(metadata, runMetadataEngineeringSlot))
		if strings.TrimSpace(metadataStringValue(metadata, runMetadataEngineeringSlot)) != "" && slot == "" && coagentProfile == agentprofile.Engineering {
			return nil, fmt.Errorf("super co-super run requires co_super_slot to be implementation or verifier")
		}
		if coagentProfile == agentprofile.Engineering && slot == "" {
			return nil, fmt.Errorf("super co-super run requires co_super_slot=\"implementation\" or co_super_slot=\"verifier\"")
		}
		if slot != "" && coagentProfile == agentprofile.Engineering {
			existing, found, err := rt.activeEngineeringSlotRun(ctx, ownerID, metadataStringValue(metadata, runMetadataTrajectoryID), slot)
			if err != nil {
				return nil, err
			}
			if found {
				existing.Metadata = cloneMetadata(existing.Metadata)
				existing.Metadata[runMetadataSpawnReused] = true
				return &existing, nil
			}
		}
		if err := rt.enforceEngineeringSlotBudget(ctx, &requesterRec); err != nil {
			return nil, err
		}
		if slot == "verifier" && coagentProfile == agentprofile.Engineering {
			if err := rt.enforceManagementVerifierSequencing(ctx, &requesterRec); err != nil {
				return nil, err
			}
		}
	}

	now := time.Now().UTC()
	metadata = ensureDesktopID(metadata, &requesterRec, metadataStringValue(metadata, runMetadataDesktopID))
	metadata = inheritTextureRequesterMetadata(metadata, &requesterRec)
	agentRec, metadata, err := resolveRunIdentity(ownerID, rt.cfg.ComputerID, metadata, &requesterRec)
	if err != nil {
		return nil, err
	}
	metadata = ensureTrajectoryID(metadata, &requesterRec, runID)
	if requesterAgent, lookupErr := rt.store.GetAgentByScope(ctx, ownerID, rt.cfg.ComputerID, requesterRec.AgentID); lookupErr == nil && requesterAgent.LifecycleVersion > 0 {
		switch agentRec.Profile {
		case agentprofile.Management, agentprofile.Engineering:
			return nil, fmt.Errorf("durable-work lifecycle refuses effects-capable %s activation", agentRec.Profile)
		}
	}
	if strings.TrimSpace(agentRec.ChannelID) == "" {
		agentRec.ChannelID = runID
	}
	claimedEngineeringSlot := false
	claimedEngineeringTrajectoryID := ""
	claimedEngineeringSlotName := ""
	slotAgentProfile := agentRec.Profile
	if slot := normalizeEngineeringSlot(metadataStringValue(metadata, runMetadataEngineeringSlot)); slot != "" &&
		slotAgentProfile == agentprofile.Engineering &&
		rt.coagentSpawnBudgetApplies(&requesterRec) {
		trajectoryID := metadataStringValue(metadata, runMetadataTrajectoryID)
		existing, claimed, err := rt.store.ClaimEngineeringSlot(ctx, ownerID, trajectoryID, slot, runID, agentRec.AgentID, requesterRunID)
		if err != nil {
			return nil, err
		}
		if !claimed {
			existing.Metadata = cloneMetadata(existing.Metadata)
			existing.Metadata[runMetadataSpawnReused] = true
			return &existing, nil
		}
		claimedEngineeringSlot = true
		claimedEngineeringTrajectoryID = trajectoryID
		claimedEngineeringSlotName = slot
	}
	grantedCapsuleHandle := ""
	releaseEngineeringSlotClaim := func(cause error) error {
		if grantedCapsuleHandle != "" && rt.capsuleExecutor != nil {
			if err := rt.capsuleExecutor.RevokeCapability(runID, grantedCapsuleHandle); err != nil {
				cause = fmt.Errorf("%w (also failed to revoke capsule capability: %v)", cause, err)
			}
		}
		if !claimedEngineeringSlot {
			return cause
		}
		if err := rt.store.ReleaseEngineeringSlotClaim(context.Background(), ownerID, claimedEngineeringTrajectoryID, claimedEngineeringSlotName, runID); err != nil {
			return fmt.Errorf("%w (also failed to release co-super slot claim: %v)", cause, err)
		}
		return cause
	}
	grantAgentProfile := agentRec.Profile
	if grantAgentProfile == agentprofile.Engineering &&
		normalizeEngineeringSlot(metadataStringValue(metadata, runMetadataEngineeringSlot)) == "implementation" {
		controlHandle := strings.TrimSpace(metadataStringValue(metadata, "capsule_control_handle"))
		if rt.capsuleExecutor == nil || controlHandle == "" {
			return nil, releaseEngineeringSlotClaim(fmt.Errorf("co-super implementation capsule authority unavailable"))
		}
		grantedCapsuleHandle, err = rt.capsuleExecutor.GrantEngineering(requesterRunID, controlHandle, runID, 24*time.Hour)
		if err != nil {
			return nil, releaseEngineeringSlotClaim(fmt.Errorf("grant co-super capsule: %w", err))
		}
		delete(metadata, "capsule_control_handle")
		metadata["capsule_handle"] = grantedCapsuleHandle
	}
	role := firstNonEmptyString(metadataStringValue(metadata, runMetadataAgentRole), metadataStringValue(metadata, runMetadataAgentProfile))
	metadata = rt.modelPolicy.EnrichMetadata(ctx, ownerID, role, metadata)
	if model := metadataStringValue(metadata, modelpolicy.MetadataModel); model != "" {
		metadata[runMetadataModel] = model
		// Texture's AuthoringModel reads provider beside model.
		metadata["provider"] = metadataStringValue(metadata, modelpolicy.MetadataProvider)
	}
	agentRec.CreatedAt = now
	agentRec.UpdatedAt = now
	if err := rt.store.UpsertAgent(ctx, agentRec); err != nil {
		return nil, releaseEngineeringSlotClaim(fmt.Errorf("persist coagent agent: %w", err))
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
		ComputerID:       rt.cfg.ComputerID,
		State:            types.RunPending,
		Prompt:           objective,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata:         metadata,
	}
	rt.stampAndMintTrajectory(ctx, rec)
	spawnedWork, err := rt.ensureSpawnedCoagentWorkItem(ctx, rec, &requesterRec, "spawned_work_item_id")
	if err != nil {
		return nil, releaseEngineeringSlotClaim(fmt.Errorf("persist spawned coagent work item: %w", err))
	}
	if spawnedWork.WorkItemID == "" && spawnedCoagentWorkItemProfile(agentProfileForRun(rec)) {
		spawnedProfile := agentProfileForRun(rec)
		log.Printf("runtime: spawned coagent work item not created for run=%s profile=%s trajectory=%s agent=%s requested_by=%s",
			rec.RunID, spawnedProfile, trajectoryIDForRun(rec), rec.AgentID, rec.RequestedByRunID)
	}
	if spawnedWork.LifecycleVersion > 0 {
		rec.TrajectoryID = spawnedWork.TrajectoryID
		project := types.ReplaceLifecycleActivationRequest{
			OwnerID: ownerID, ComputerID: rec.ComputerID,
			CommandID:    "lifecycle-project-spawned-run:" + rec.RunID,
			TrajectoryID: rec.TrajectoryID, AgentID: rec.AgentID, Run: *rec,
		}
		project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
		if _, err := rt.store.ReplaceLifecycleActivation(ctx, project); err != nil {
			return nil, releaseEngineeringSlotClaim(fmt.Errorf("project spawned lifecycle run: %w", err))
		}
	} else if err := rt.store.CreateRun(ctx, *rec); err != nil {
		return nil, releaseEngineeringSlotClaim(fmt.Errorf("persist coagent run: %w", err))
	}
	rt.createAgentMutationForRun(ctx, rec)

	// Emit submitted event.
	objectiveLenPayload, _ := json.Marshal(map[string]any{
		"prompt_length": len(objective),
		"requested_by":  requesterRunID,
	})
	rt.emitEvent(ctx, rec, types.EventRunSubmitted, events.CauseTaskLifecycle, objectiveLenPayload)
	if shouldLogWireLifecycle(rec) || shouldLogWireLifecycle(&requesterRec) {
		requesterSummaryProfile := agentProfileForRun(&requesterRec)
		log.Printf("runtime: started coagent %s requested by %s requester_profile=%s", wireLifecycleSummary(rec), requesterRec.RunID, requesterSummaryProfile)
	}
	if err := rt.recordExplicitInitialTextureDecisionIfNeeded(ctx, rec); err != nil {
		rt.handleExecutionError(ctx, rec, err)
		return nil, releaseEngineeringSlotClaim(err)
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
	profile := agentProfileForRun(rec)
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
		if loaded, err := rt.getRunForComputer(ctx, ownerID, requesterRunID); err == nil && loaded.OwnerID == ownerID {
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
		if requesterProfile := agentProfileForRun(requester); requesterProfile != "" {
			details["requested_by_agent_profile"] = requesterProfile
		}
	}
	copyMetadataStringToDetails(rec.Metadata, details, "requested_by_profile")
	copyMetadataStringToDetails(rec.Metadata, details, "requested_by_agent_id")
	copyMetadataStringToDetails(rec.Metadata, details, "requested_by_run_id")
	work := types.WorkItemRecord{
		OwnerID:              ownerID,
		ComputerID:           strings.TrimSpace(rec.ComputerID),
		TrajectoryID:         trajectoryID,
		Objective:            objective,
		Reason:               "spawn_agent coagent objective",
		AuthorityProfile:     profile,
		AssignedAgentID:      agentID,
		CreatedByRunID:       requesterRunID,
		ObjectiveFingerprint: "spawned_coagent:" + workitem.ObjectiveFingerprint(ownerID, trajectoryID, rec.RunID, objective),
		Details:              details,
	}
	if work.ComputerID != "" {
		if _, lifecycleErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, work.ComputerID, trajectoryID); lifecycleErr == nil {
			work.WorkItemID = uuid.NewSHA1(uuid.NameSpaceOID, []byte("choir:lifecycle:spawned-work:"+ownerID+":"+trajectoryID+":"+rec.RunID)).String()
			open := types.OpenLifecycleWorkRequest{
				OwnerID: ownerID, ComputerID: work.ComputerID,
				CommandID: "lifecycle-open-spawned-work:" + rec.RunID, TrajectoryID: trajectoryID, WorkItem: work,
			}
			open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
			opened, openErr := rt.store.OpenLifecycleWork(ctx, open)
			if openErr != nil {
				return types.WorkItemRecord{}, openErr
			}
			if opened.WorkItem == nil {
				return types.WorkItemRecord{}, fmt.Errorf("open spawned lifecycle work returned no work item")
			}
			return *opened.WorkItem, nil
		} else if !errors.Is(lifecycleErr, store.ErrNotFound) {
			return types.WorkItemRecord{}, lifecycleErr
		}
	}
	return rt.store.CreateWorkItem(ctx, work)
}

func inheritTextureRequesterMetadata(metadata map[string]any, requesterRun *types.RunRecord) map[string]any {
	requesterTexture := agentProfileForRun(requesterRun)
	if requesterRun == nil || requesterTexture != agentprofile.Texture {
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
	if item.LifecycleVersion > 0 {
		rec.Metadata["lifecycle_work_item_id"] = item.WorkItemID
	}
	if strings.TrimSpace(metadataKey) != "" {
		rec.Metadata[metadataKey] = item.WorkItemID
	}
	return item, nil
}

func spawnedCoagentWorkItemProfile(profile string) bool {
	workItemProfile, _ := agentprofile.Canonical(profile)
	switch workItemProfile {
	case agentprofile.Research, agentprofile.Management, agentprofile.Engineering:
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

const maxManagementActiveEngineeringSlots = 2

func (rt *Runtime) coagentSpawnBudgetApplies(requesterRec *types.RunRecord) bool {
	if requesterRec == nil {
		return false
	}
	budgetProfile := agentProfileForRun(requesterRec)
	return budgetProfile == agentprofile.Management
}

func (rt *Runtime) enforceEngineeringSlotBudget(ctx context.Context, requesterRec *types.RunRecord) error {
	if rt == nil || rt.store == nil || requesterRec == nil {
		return nil
	}
	trajectoryID := trajectoryIDForRun(requesterRec)
	active, err := rt.store.CountActiveEngineeringSlots(ctx, requesterRec.OwnerID, trajectoryID)
	if err != nil {
		return fmt.Errorf("check active co-super slots for super trajectory budget: %w", err)
	}
	if active >= maxManagementActiveEngineeringSlots {
		return fmt.Errorf("super active co-super slot limit reached for trajectory %s (%d/%d); coordinate existing implementation/verifier agents over channels, cancel or wait for a co-super slot, or submit a precise blocker instead of spawning more", trajectoryID, active, maxManagementActiveEngineeringSlots)
	}
	return nil
}

func (rt *Runtime) activeEngineeringSlotRun(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return types.RunRecord{}, false, nil
	}
	slot = normalizeEngineeringSlot(slot)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if slot == "" || trajectoryID == "" {
		return types.RunRecord{}, false, nil
	}
	return rt.store.ActiveEngineeringSlotRun(ctx, ownerID, trajectoryID, slot)
}

func (rt *Runtime) enforceManagementVerifierSequencing(ctx context.Context, requesterRec *types.RunRecord) error {
	if rt == nil || rt.store == nil || requesterRec == nil {
		return nil
	}
	trajectoryID := trajectoryIDForRun(requesterRec)
	impl, found, err := rt.store.EngineeringSlotRun(ctx, requesterRec.OwnerID, trajectoryID, "implementation")
	if err != nil {
		return fmt.Errorf("lookup implementation co-super slot for verifier sequencing: %w", err)
	}
	if found && impl.State.Active() {
		return fmt.Errorf("super verifier spawn blocked until implementation co-super %s reports commit/package/blocker evidence and finishes; wait for update_coagent evidence before spawning slot=\"verifier\"", impl.RunID)
	}
	if found && impl.State.Terminal() {
		return nil
	}
	return fmt.Errorf("super verifier spawn requires prior implementation co-super evidence; spawn slot=\"implementation\" first, wait for commit/package/blocker evidence, then spawn slot=\"verifier\" with the exact evidence to inspect")
}

func (rt *Runtime) createAgentMutationForRun(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rt.store == nil || rec == nil {
		return
	}
	if !runHasProfile(rec, agentprofile.Texture) {
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
	rec, err := rt.getRunForComputer(ctx, ownerID, runID)
	if err != nil {
		rt.runningMu.Unlock()
		if err == store.ErrNotFound {
			return fmt.Errorf("run not found: %s", runID)
		}
		return fmt.Errorf("lookup run: %w", err)
	}
	assignmentProjected := false
	if rec.State.Active() && metadataStringValue(rec.Metadata, "assignment_id") != "" {
		// Do not terminalize the generic run projection first: durable exact
		// assignment revoke intent must precede capability revoke/destroy/inspect,
		// structured executor ack, and assignment/work/run cancellation.
		rt.runningMu.Unlock()
		assignmentProjected, err = rt.cancelBoundEngineeringRun(context.WithoutCancel(ctx), rec, reason)
		if err != nil {
			return fmt.Errorf("cancel bound Engineering run fate: %w", err)
		}
		rt.runningMu.Lock()
		rec, err = rt.getRunForComputer(context.Background(), ownerID, runID)
		if err != nil {
			rt.runningMu.Unlock()
			return fmt.Errorf("reload assignment-cancelled run: %w", err)
		}
	}
	if rec.State.Terminal() && !assignmentProjected {
		trajectoryID := strings.TrimSpace(trajectoryIDForRun(&rec))
		trajectory, trajectoryErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, rec.ComputerID, trajectoryID)
		if trajectoryErr != nil || trajectory.Status != types.TrajectoryCancelled {
			rt.runningMu.Unlock()
			return fmt.Errorf("cannot cancel run in %s state", rec.State)
		}
	}

	now := time.Now().UTC()
	if !rec.State.Terminal() || !assignmentProjected {
		rec.State = types.RunCancelled
		rec.Error = reason
		rec.UpdatedAt = now
		rec.FinishedAt = &now
		// Lifecycle-bound runs terminalize through the canonical reducer
		// command so the state change and its run_terminalized event commit
		// in one batch. Non-lifecycle runs keep the bare UpdateRun path
		// (which fails closed in kernel mode until they are bound).
		if err := rt.terminalizeRunCanonical(ctx, &rec, reason); err != nil {
			rt.runningMu.Unlock()
			return fmt.Errorf("update cancelled run: %w", err)
		}
	}
	cancel := rt.running[runID]
	delete(rt.running, runID)
	rt.runningMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if bindErr := rt.bindTerminalRunOutcome(context.Background(), &rec, true); bindErr != nil {
		log.Printf("runtime: bind cancelled terminal outcome for run %s: %v", rec.RunID, bindErr)
	}
	errPayload, _ := json.Marshal(map[string]string{"error": reason})
	rt.emitEvent(context.Background(), &rec, types.EventRunCancelled, events.CauseTaskLifecycle, errPayload)
	// A cancelled or deadline-ended task run closes its task here: the
	// activation's own error path finds the stored state terminal and
	// skips the post-turn hook (management delegation R2-8, R2-20).
	rt.closeManagementTaskOfEndedRun(context.Background(), &rec)
	return nil
}

// terminalizeRunCanonical persists a run's terminal state through the
// canonical TerminalizeRun reducer command when the run is lifecycle-bound
// (its trajectory object exists), so the state change and the canonical
// run_terminalized event commit in one atomic batch. Runs without a
// lifecycle trajectory fall back to the bare UpdateRun write.
func (rt *Runtime) terminalizeRunCanonical(ctx context.Context, rec *types.RunRecord, reason string) error {
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if trajectoryID == "" || rec.OwnerID == "" || rec.ComputerID == "" {
		return rt.store.UpdateRun(ctx, *rec)
	}
	if _, err := rt.store.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.ComputerID, trajectoryID); err != nil {
		// No lifecycle trajectory object: not lifecycle-bound, keep the
		// bare write path.
		return rt.store.UpdateRun(ctx, *rec)
	}
	req := types.TerminalizeRunRequest{
		OwnerID:       rec.OwnerID,
		ComputerID:    rec.ComputerID,
		CommandID:     "terminalize-run:" + rec.RunID,
		TrajectoryID:  trajectoryID,
		AgentID:       rec.AgentID,
		RunID:         rec.RunID,
		TerminalState: rec.State,
		Reason:        reason,
		Result:        rec.Result,
	}
	digest, err := store.ComputeTerminalizeRunDigest(req)
	if err != nil {
		return err
	}
	req.CommandDigest = digest
	_, err = rt.store.TerminalizeRun(ctx, req)
	return err
}

// ReactivateRunCanonical persists a run's reactivation (passivated or
// interrupted -> pending/running) through the canonical ReactivateRun
// reducer command when the run is lifecycle-bound, so the state change and
// the canonical run_reactivated event commit in one atomic batch. Runs
// without a lifecycle trajectory fall back to the bare UpdateRun write.
// metadataPatch merges reactivation markers into the run's metadata.
func (rt *Runtime) ReactivateRunCanonical(ctx context.Context, rec *types.RunRecord, targetState types.RunState, metadataPatch map[string]any) error {
	trajectoryID := strings.TrimSpace(trajectoryIDForRun(rec))
	if trajectoryID == "" || rec.OwnerID == "" || rec.ComputerID == "" {
		return rt.store.UpdateRun(ctx, *rec)
	}
	if _, err := rt.store.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.ComputerID, trajectoryID); err != nil {
		return rt.store.UpdateRun(ctx, *rec)
	}
	req := types.ReactivateRunRequest{
		OwnerID:       rec.OwnerID,
		ComputerID:    rec.ComputerID,
		CommandID:     "reactivate-run:" + rec.RunID + ":" + rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		TrajectoryID:  trajectoryID,
		AgentID:       rec.AgentID,
		RunID:         rec.RunID,
		TargetState:   targetState,
		MetadataPatch: metadataPatch,
	}
	digest, err := store.ComputeReactivateRunDigest(req)
	if err != nil {
		return err
	}
	req.CommandDigest = digest
	_, err = rt.store.ReactivateRun(ctx, req)
	return err
}

// persistActivationState serializes activation writes with cancellation and
// progress-deadline terminalization. A stored terminal state always wins.
func (rt *Runtime) persistActivationState(ctx context.Context, rec *types.RunRecord) (bool, error) {
	return rt.persistActivationStateWithEvent(ctx, rec, nil)
}

// persistActivationStateWithEvent is persistActivationState plus an optional
// runtime event that is folded into the same atomic batch as the run-state
// projection for lifecycle-bound runs, so the state change and its
// choir.event commit together. The caller still publishes the live bus
// event after this returns.
func (rt *Runtime) persistActivationStateWithEvent(ctx context.Context, rec *types.RunRecord, event *types.EventRecord) (bool, error) {
	rt.runningMu.Lock()
	defer rt.runningMu.Unlock()

	stored, err := rt.getRunForComputer(context.Background(), rec.OwnerID, rec.RunID)
	if err != nil {
		return false, err
	}
	if stored.State.Terminal() {
		*rec = stored
		return false, nil
	}
	if err := rt.updateRunAndMarkSuccessfulCoagentActivationDeliveredWithEvent(ctx, rec, event); err != nil {
		return false, err
	}
	// A bound Engineering run that ends (completed, failed, cancelled, or
	// blocked, which is never resumed) strands its capsule unless the
	// assignment fate reconcile re-runs. Kick it here, at the single
	// persistence point every run-state transition shares. A passivated run
	// is parked, not ended: it keeps its capsule and round (C1e D7), so the
	// kick does not fire on passivation. Best-effort: the kick never masks
	// the persisted transition. Receipt:
	// docs/problems/engineering-run-death-leaves-bound-assignment-2026-09-29.md
	if assignedEngineeringRun(rec) && engineeringRunEnded(rec.State) {
		rt.kickEngineeringReconcileAfterPersist(rec)
	}
	return true, nil
}

// engineeringRunEnded reports whether an engineering run state ends the run
// for its assignment: terminal, or blocked (blocked runs are replaced, never
// resumed). Pending, running and passivated runs keep their assignment.
func engineeringRunEnded(state types.RunState) bool {
	return state.Terminal() || state == types.RunBlocked
}

// kickEngineeringReconcileAfterPersist re-drives the assignment reconcile for
// a trajectory whose bound run just ended. Path B's desk reconcile is
// deleted (C1e step 3): the assignment's own cancel and Report carry its
// outcome to management.
func (rt *Runtime) kickEngineeringReconcileAfterPersist(rec *types.RunRecord) {
	trajID := strings.TrimSpace(trajectoryIDForRun(rec))
	if trajID == "" {
		return
	}
	ownerID, computerID, runID, state := rec.OwnerID, rec.ComputerID, rec.RunID, rec.State
	go func() {
		kickCtx := context.Background()
		if reconErr := rt.ReconcileEngineeringAssignmentsForTrajectory(kickCtx, ownerID, computerID, trajID); reconErr != nil {
			log.Printf("runtime: engineering reconcile after assignment run %s left %s: %v", runID, state, reconErr)
		}
	}()
}

// persistActivationStateAndEmit writes the activation state and its runtime
// event in one atomic batch for lifecycle-bound runs (the event folds into
// the projection's PutBatchConditional), then publishes the live bus event
// and trace record after the commit. For non-lifecycle runs the event is
// appended separately. Returns whether the state was persisted.
func (rt *Runtime) persistActivationStateAndEmit(ctx context.Context, rec *types.RunRecord, kind types.EventKind, cause events.EventCause, payload json.RawMessage) (bool, error) {
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
	persisted, err := rt.persistActivationStateWithEvent(ctx, rec, evRec)
	if err != nil {
		return false, err
	}
	if !persisted {
		return false, nil
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  cause,
	})
	return true, nil
}

// CancelAgent cancels the most recent non-terminal run owned by the given agent.
func (rt *Runtime) CancelAgent(ctx context.Context, agentID, ownerID string) error {
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return fmt.Errorf("lookup resident agent run: %w", err)
	} else if found {
		return rt.CancelRun(ctx, resident.RunID, ownerID)
	}
	rec, err := rt.latestActiveRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		if err == store.ErrNotFound {
			return fmt.Errorf("agent not found: %s", agentID)
		}
		return fmt.Errorf("lookup active agent run: %w", err)
	}
	return rt.CancelRun(ctx, rec.RunID, ownerID)
}

const trajectoryActivationDrainTimeout = 120 * time.Second

// cancelTrajectoryAuthority delegates the durable cancellation transition to
// the store, which atomically closes open obligations and terminalizes live
// trajectories. The returned record is the authoritative durable state.
func (rt *Runtime) cancelTrajectoryAuthorityCommand(ctx context.Context, ownerID, trajectoryID, commandID, reason string, expectedVersion int64, expectedHead string) (types.LifecycleResult, error) {
	if rt == nil || rt.store == nil {
		return types.LifecycleResult{}, fmt.Errorf("cancel trajectory: runtime store is unavailable")
	}
	ownerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID)
	if ownerID == "" || trajectoryID == "" {
		return types.LifecycleResult{}, fmt.Errorf("cancel trajectory: owner_id and trajectory_id are required")
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID != "" {
		trajectory, lifecycleErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
		if lifecycleErr == nil {
			if strings.TrimSpace(commandID) == "" {
				commandID = "lifecycle-cancel:" + trajectoryID
			}
			if strings.TrimSpace(reason) == "" {
				reason = "owner cancellation"
			}
			// A restart/retry resumes the durable original authority rather than
			// deriving command identity from the trajectory version advanced by
			// assignment fate transitions.
			if intent, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, trajectoryID); intentErr == nil {
				if intent.CommandID != strings.TrimSpace(commandID) ||
					(expectedVersion > 0 && expectedVersion != intent.RequestedLifecycleVersion) ||
					(strings.TrimSpace(expectedHead) != "" && strings.TrimSpace(expectedHead) != intent.ExpectedHeadRevisionID) ||
					(strings.TrimSpace(reason) != "" && strings.TrimSpace(reason) != intent.Reason) {
					return types.LifecycleResult{}, store.ErrLifecycleCommandConflict
				}
				expectedVersion, expectedHead, reason = intent.RequestedLifecycleVersion, intent.ExpectedHeadRevisionID, intent.Reason
			} else if errors.Is(intentErr, store.ErrNotFound) {
				if expectedVersion <= 0 || strings.TrimSpace(expectedHead) == "" {
					snapshot, snapshotErr := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
					if snapshotErr != nil {
						return types.LifecycleResult{}, snapshotErr
					}
					expectedVersion, expectedHead = snapshot.Trajectory.LifecycleVersion, snapshot.HeadRevision.RevisionID
				}
			} else {
				return types.LifecycleResult{}, intentErr
			}
			original := types.CancelLifecycleRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: strings.TrimSpace(commandID),
				TrajectoryID: trajectory.TrajectoryID, Reason: strings.TrimSpace(reason), ExpectedLifecycleVersion: expectedVersion,
				RequestedLifecycleVersion: expectedVersion, ExpectedHeadRevisionID: strings.TrimSpace(expectedHead)}
			original.CommandDigest, _ = store.ComputeCancelLifecycleDigest(original)
			intent, prepErr := rt.store.PrepareLifecycleCancellation(ctx, original)
			if prepErr != nil {
				return types.LifecycleResult{}, prepErr
			}
			if trajectory.Status == types.TrajectoryLive {
				if _, fateErr := rt.prepareEngineeringTrajectoryCancellation(context.WithoutCancel(ctx), ownerID, computerID, trajectoryID, intent.Reason); fateErr != nil {
					return types.LifecycleResult{}, fmt.Errorf("prepare trajectory assignment fate: %w", fateErr)
				}
			}
			fresh, freshErr := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
			if freshErr != nil {
				return types.LifecycleResult{}, freshErr
			}
			final := original
			final.ExpectedLifecycleVersion = fresh.Trajectory.LifecycleVersion
			final.RequestedLifecycleVersion = intent.RequestedLifecycleVersion
			final.CommandDigest = intent.CommandDigest
			return rt.store.CancelLifecycleTrajectory(ctx, final)
		}
		if !errors.Is(lifecycleErr, store.ErrNotFound) {
			return types.LifecycleResult{}, lifecycleErr
		}
	}
	if expectedVersion > 0 || strings.TrimSpace(expectedHead) != "" {
		return types.LifecycleResult{}, store.ErrNotFound
	}
	trajectory, err := rt.store.CancelTrajectoryAuthority(ctx, ownerID, trajectoryID)
	return types.LifecycleResult{Trajectory: trajectory}, err
}

func (rt *Runtime) cancelTrajectoryAuthority(ctx context.Context, ownerID, trajectoryID string) (types.TrajectoryRecord, error) {
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID != "" {
		if trajectory, err := rt.store.GetLifecycleTrajectory(ctx, strings.TrimSpace(ownerID), computerID, strings.TrimSpace(trajectoryID)); err == nil && trajectory.Status != types.TrajectoryLive {
			return trajectory, nil
		}
	}
	result, err := rt.cancelTrajectoryAuthorityCommand(ctx, ownerID, trajectoryID, "", "", 0, "")
	return result.Trajectory, err
}

// CancelTrajectoryCommand applies a caller-bound cancellation command and then
// drains active realizations. Reusing commandID with the same canonical request
// replays the receipt; reusing it with different input conflicts in the reducer.
func (rt *Runtime) CancelTrajectoryCommand(ctx context.Context, trajectoryID, ownerID, commandID, reason string, expectedVersion int64, expectedHead string) (types.LifecycleResult, []string, error) {
	result, err := rt.cancelTrajectoryAuthorityCommand(ctx, ownerID, trajectoryID, commandID, reason, expectedVersion, expectedHead)
	if err != nil {
		return types.LifecycleResult{}, nil, err
	}
	if result.Trajectory.Status != types.TrajectoryCancelled {
		return result, nil, nil
	}
	if err := rt.finishEngineeringTrajectoryCancellation(context.WithoutCancel(ctx), ownerID, result.Trajectory.ComputerID, trajectoryID, reason); err != nil {
		return result, nil, fmt.Errorf("finish trajectory assignment cancellation: %w", err)
	}
	cancelled, err := rt.drainCancelledTrajectoryActivations(ctx, ownerID, result.Trajectory.ComputerID, trajectoryID)
	if err != nil {
		return result, cancelled, err
	}
	// The drain terminalizes live activations through TerminalizeRun, which
	// appends run_terminalized events and advances the trajectory reducer. Return
	// the post-drain trajectory so callers compare against the settled state, not
	// the pre-drain snapshot.
	if refreshed, refreshErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, result.Trajectory.ComputerID, trajectoryID); refreshErr == nil {
		result.Trajectory = refreshed
	}
	return result, cancelled, nil
}

// CancelTrajectory cancels an owner-scoped trajectory and then terminates all
// of its active run activations. Durable trajectory state becomes terminal
// before any activation is signalled. A settled trajectory is reported
// unchanged and its activations are not cancelled.
func (rt *Runtime) CancelTrajectory(ctx context.Context, trajectoryID, ownerID string) (types.TrajectoryRecord, []string, error) {
	ownerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID)
	trajectory, err := rt.cancelTrajectoryAuthority(ctx, ownerID, trajectoryID)
	if err != nil {
		return types.TrajectoryRecord{}, nil, err
	}
	if trajectory.Status != types.TrajectoryCancelled {
		return trajectory, nil, nil
	}
	if err := rt.finishEngineeringTrajectoryCancellation(context.WithoutCancel(ctx), ownerID, trajectory.ComputerID, trajectoryID, "owner cancellation"); err != nil {
		return trajectory, nil, fmt.Errorf("finish trajectory assignment cancellation: %w", err)
	}
	cancelled, err := rt.drainCancelledTrajectoryActivations(ctx, ownerID, trajectory.ComputerID, trajectoryID)
	if err != nil {
		return trajectory, cancelled, err
	}
	// The drain terminalizes live activations through TerminalizeRun, which
	// advances the trajectory reducer. Return the post-drain record so callers
	// observe the settled state, not the pre-drain snapshot.
	if refreshed, refreshErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, trajectory.ComputerID, trajectoryID); refreshErr == nil {
		trajectory = refreshed
	}
	return trajectory, cancelled, nil
}

func (rt *Runtime) drainCancelledTrajectoryActivations(ctx context.Context, ownerID, computerID, trajectoryID string) ([]string, error) {
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), trajectoryActivationDrainTimeout)
	defer cancel()

	cancelled := []string{}
	computerID = strings.TrimSpace(computerID)
	resident := map[string]bool{}
	rt.runningMu.Lock()
	for runID := range rt.running {
		resident[runID] = true
	}
	rt.runningMu.Unlock()
	var active []types.RunRecord
	var err error
	if computerID != "" {
		var lifecycleRuns []types.RunRecord
		lifecycleRuns, err = rt.store.ListLifecycleRunsByTrajectory(drainCtx, ownerID, computerID, trajectoryID, 0)
		for _, run := range lifecycleRuns {
			// The reducer may already have projected the durable run Cancelled.
			// Retain only live durable records or the exact process-local
			// resident intersection; never dispatch a historical terminal actor.
			if run.State.Active() || resident[run.RunID] {
				active = append(active, run)
			}
		}
		if err == nil && len(lifecycleRuns) == 0 {
			// Legacy trajectories may carry a run ComputerID without a scoped
			// lifecycle authority. Lifecycle projections are excluded from this
			// owner-only compatibility query.
			active, err = rt.store.ListActiveRunsByTrajectory(drainCtx, ownerID, trajectoryID, 0)
		}
	} else {
		active, err = rt.store.ListActiveRunsByTrajectory(drainCtx, ownerID, trajectoryID, 0)
	}
	if err != nil {
		return cancelled, fmt.Errorf("list trajectory activations for resident drain: %w", err)
	}
	// Legacy trajectories have no computer identity; infer it only for their
	// activation updates. Lifecycle trajectories always arrive scoped.
	if computerID == "" && len(active) > 0 {
		computerID = strings.TrimSpace(active[0].ComputerID)
	}
	if computerID != "" {
		snapshot, snapshotErr := rt.store.GetLifecycleSnapshot(drainCtx, ownerID, computerID, trajectoryID)
		if snapshotErr == nil && snapshot.Activation.RunID != "" && snapshot.Activation.State != types.RunCancelled {
			found := false
			for _, run := range active {
				found = found || run.RunID == snapshot.Activation.RunID
			}
			if !found {
				latest, getErr := rt.getRunForComputer(drainCtx, ownerID, snapshot.Activation.RunID)
				if getErr != nil {
					return cancelled, fmt.Errorf("load latest trajectory activation: %w", getErr)
				}
				active = append(active, latest)
			}
		}
	}
	for _, run := range active {
		if run.State.Active() || resident[run.RunID] {
			if rt.dispatchActor == nil {
				return cancelled, fmt.Errorf("deliver trajectory cancellation: actor runtime is unavailable")
			}
			if err := rt.dispatchActor(drainCtx, run.OwnerID, run.ComputerID, run.AgentID, "cancel", run.RunID, trajectoryID, ""); err != nil {
				return cancelled, fmt.Errorf("deliver trajectory cancellation for run %s: %w", run.RunID, err)
			}
		}
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
	rec, err := rt.getRunForComputer(ctx, ownerID, runID)
	if err != nil {
		return nil, err
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

// ListTrajectoriesByOwner returns recent legacy trajectories plus canonical
// lifecycle trajectories scoped to this runtime's computer.
func (rt *Runtime) ListTrajectoriesByOwner(ctx context.Context, ownerID string, limit int) ([]types.TrajectoryRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	legacy, err := rt.store.ListTrajectoriesByOwner(ctx, ownerID, limit)
	if err != nil {
		return nil, err
	}
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	if computerID == "" {
		return legacy, nil
	}
	lifecycle, err := rt.store.ListLifecycleTrajectoriesByOwner(ctx, ownerID, computerID, limit)
	if err != nil {
		return nil, err
	}
	trajectories := make([]types.TrajectoryRecord, 0, len(legacy)+len(lifecycle))
	seen := make(map[string]struct{}, len(legacy)+len(lifecycle))
	for _, trajectory := range append(lifecycle, legacy...) {
		if trajectory.LifecycleVersion > 0 && trajectory.ComputerID != computerID {
			continue
		}
		if _, ok := seen[trajectory.TrajectoryID]; ok {
			continue
		}
		seen[trajectory.TrajectoryID] = struct{}{}
		trajectories = append(trajectories, trajectory)
	}
	sort.Slice(trajectories, func(i, j int) bool {
		if !trajectories[i].UpdatedAt.Equal(trajectories[j].UpdatedAt) {
			return trajectories[i].UpdatedAt.After(trajectories[j].UpdatedAt)
		}
		return trajectories[i].TrajectoryID < trajectories[j].TrajectoryID
	})
	if len(trajectories) > limit {
		trajectories = trajectories[:limit]
	}
	return trajectories, nil
}

// ListRunsByOwner returns recent runs for the given owner, ordered by
// creation time descending.
func (rt *Runtime) ListRunsByOwner(ctx context.Context, ownerID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	legacy, err := rt.store.ListRunsByOwner(ctx, ownerID, limit)
	if err != nil {
		return nil, err
	}
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	if computerID == "" {
		return legacy, nil
	}
	lifecycle, err := rt.store.ListLifecycleRunsByOwner(ctx, ownerID, computerID, limit)
	if err != nil {
		return nil, err
	}
	runs := append(legacy, lifecycle...)
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

// ListRunsByChannel returns recent runs for one channel, including canonical
// lifecycle projections for this runtime's computer.
func (rt *Runtime) ListRunsByChannel(ctx context.Context, ownerID, channelID string, limit int) ([]types.RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	legacy, err := rt.store.ListRunsByChannel(ctx, ownerID, channelID, limit)
	if err != nil {
		return nil, err
	}
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	if computerID == "" {
		return legacy, nil
	}
	lifecycle, err := rt.store.ListLifecycleRunsByChannel(ctx, ownerID, computerID, channelID, limit)
	if err != nil {
		return nil, err
	}
	runs := append(legacy, lifecycle...)
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].RunID < runs[j].RunID
	})
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
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
// agent profile that still occupy admission capacity. For frozen processor
// history this issues one FindWorkItemByFingerprint per running run (an N+1
// against the work-item table; acceptable at current run volumes), and any
// lookup error silently defaults to "occupies admission" — the conservative side.
func (rt *Runtime) RunningCountByProfile(ctx context.Context, profile string) int {
	if rt.bootInProgress.Load() {
		return rt.RunningCount()
	}
	profile, _ = agentprofile.Canonical(profile)
	var runs []types.RunRecord
	var err error
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID != "" {
		runs, err = rt.store.ListRunsByOwnerStates(ctx, ownerID, computerID, []types.RunState{types.RunRunning}, 1024)
	} else {
		runs, err = rt.store.ListRunsByState(ctx, types.RunRunning, 1000)
	}
	if err != nil {
		log.Printf("runtime: count running %s runs: %v", profile, err)
		return rt.RunningCount()
	}
	count := 0
	for i := range runs {
		runProfile, _ := agentprofile.Canonical(runs[i].AgentProfile)
		if runProfile != profile {
			continue
		}
		if profile == frozenProcessorProfile && !rt.processorRunOccupiesAdmission(ctx, runs[i]) {
			continue
		}
		count++
	}
	return count
}

// pendingDeskMutations reports how many pending texture-desk mutations exist
// for this computer's owner. A pending mutation means the controller owes the
// document another activation even while no run is in flight — the window
// between activations must not read as "idle" to host lifecycle (idle sweep,
// pressure reclaim). Receipt: docs/problems/vmctl-pressure-reclaim-mid-respawn-gap-2026-09-28.md.
func (rt *Runtime) pendingDeskMutations(ctx context.Context) int {
	if rt == nil || rt.store == nil {
		return 0
	}
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || computerID == "" {
		return 0
	}
	count, err := rt.store.CountPendingAgentMutations(ctx, ownerID, computerID)
	if err != nil {
		return 0
	}
	return count
}

// activeSelfdevOperations counts self-development operations holding
// in-flight transactional state that would be lost to a guest termination:
// executing (cast running), accepted (approved, apply pending),
// materializing, and rollback_pending. Parked states — frozen, verified,
// awaiting_approval — are deliberately excluded: with artifacts on durable
// storage the boot reconcile can resume them after a restart, and a wedged
// parked op must not pin the guest busy forever. An executing op orphaned by
// a crash did pin it until closeSelfDevelopmentOperationsAfterCrash
// (docs/problems/selfdev-zombie-operations-pin-owner-computer-2026-10-10.md).
func (rt *Runtime) activeSelfdevOperations(ctx context.Context) int {
	if rt == nil || rt.selfdevOperations == nil {
		return 0
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return 0
	}
	ops, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdevBusyStates...)
	if err != nil {
		return 0
	}
	return len(ops)
}

// selfdevBusyStates are the states activeSelfdevOperations counts.
var selfdevBusyStates = []string{selfdev.StateExecuting, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending}

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

// runFateInterruptedByRestart is the fate a crash boot gives every pre-boot
// run (CLAUDE.md, restarts end work): the run is cancelled with this error
// and the same metadata fate, and nothing re-dispatches it.
const runFateInterruptedByRestart = "interrupted_by_restart"

// forEachPreBootRun visits the runs of this computer in the given states, as
// boot sees them before any actor runs. Production computers list by owner,
// over every run header; a computer without an owner id scans by state. fn
// reports whether it changed the run (a by-state scan stops when a page
// changes nothing).
func (rt *Runtime) forEachPreBootRun(ctx context.Context, phase string, states []types.RunState, fn func(rec *types.RunRecord, was types.RunState) bool) {
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	productionComputer := rt.cfg.ComputerID != "" && strings.HasPrefix(strings.TrimSpace(rt.cfg.ComputerID), "computer-")
	if ownerID != "" {
		runs, err := rt.store.ListRunsByOwnerStates(ctx, ownerID, computerID, states, store.ListAllRunHeaders)
		if err != nil {
			log.Printf("runtime: %s owner-scoped list: %v", phase, err)
			return
		}
		log.Printf("runtime: %s owner-scoped owner=%s computer=%s candidates=%d", phase, ownerID, computerID, len(runs))
		for i := range runs {
			fn(&runs[i], runs[i].State)
		}
		return
	}
	if productionComputer {
		log.Printf("runtime: %s skipped: owner_id required on production computer %s", phase, rt.cfg.ComputerID)
		return
	}
	for _, state := range states {
		for {
			runs, err := rt.store.ListRunsByState(ctx, state, 100)
			if err != nil {
				log.Printf("runtime: %s: query %s runs: %v", phase, state, err)
				break
			}
			progressed := false
			for i := range runs {
				if fn(&runs[i], state) {
					progressed = true
				}
			}
			if len(runs) == 0 || !progressed {
				break
			}
		}
	}
}

// closePreBootRunsAfterCrash ends every run a markerless boot finds pending,
// running, parked or blocked: cancelled, error and metadata fate
// interrupted_by_restart, finished now. It creates no work and wakes no
// one; the owner's next act starts fresh work.
func (rt *Runtime) closePreBootRunsAfterCrash(ctx context.Context) {
	states := []types.RunState{types.RunPending, types.RunRunning, types.RunPassivated, types.RunBlocked}
	rt.forEachPreBootRun(ctx, "crash boot close", states, func(rec *types.RunRecord, was types.RunState) bool {
		now := time.Now().UTC()
		rec.State = types.RunCancelled
		rec.Error = runFateInterruptedByRestart
		rec.UpdatedAt = now
		rec.FinishedAt = &now
		rec.Metadata = cloneMetadata(rec.Metadata)
		rec.Metadata["fate"] = runFateInterruptedByRestart
		if err := rt.terminalizeRunCanonical(ctx, rec, runFateInterruptedByRestart); err != nil {
			log.Printf("runtime: crash boot close: run %s: %v", rec.RunID, err)
			return false
		}
		if runHasProfile(rec, agentprofile.Texture) {
			if err := rt.store.MarkAgentMutationStale(context.WithoutCancel(ctx), rec.OwnerID, agentMutationComputerID(rec), rec.RunID); err != nil {
				log.Printf("runtime: crash boot close: stale mutation %s: %v", rec.RunID, err)
			}
		}
		rt.emitEvent(ctx, rec, types.EventRunCancelled, events.CauseSupervisorRecovery,
			crashClosePayload(rt.store.TelemetryBatchingConfigured()))
		log.Printf("runtime: closed run %s (was %s) after a crash restart", rec.RunID, was)
		return true
	})
}

// passivateInterruptedActivations parks the runs that were active before a
// planned restart, for the one resume a planned restart allows.
func (rt *Runtime) passivateInterruptedActivations(ctx context.Context) {
	states := []types.RunState{types.RunPending, types.RunRunning}
	rt.forEachPreBootRun(ctx, "boot passivation", states, func(rec *types.RunRecord, was types.RunState) bool {
		now := time.Now().UTC()
		if isManagementTaskRun(rec) {
			// Tasks are not resumed (R2-12): the run ends here, never
			// parked for a resume, and the task's round closes
			// run_passivated with its residuals, which wakes the parent.
			// The consumed marker permits that one wake.
			rec.State = types.RunCancelled
			rec.Error = "interrupted by a planned restart; management tasks are not resumed"
			rec.UpdatedAt, rec.FinishedAt = now, &now
			rec.Metadata = cloneMetadata(rec.Metadata)
			rec.Metadata["fate"] = "planned_restart_closed"
			if err := rt.terminalizeRunCanonical(ctx, rec, rec.Error); err != nil {
				log.Printf("runtime: boot passivation: close task run %s: %v", rec.RunID, err)
				return false
			}
			passivated := *rec
			passivated.State = types.RunPassivated
			rt.closeManagementTaskOfEndedRun(ctx, &passivated)
			log.Printf("runtime: closed management task run %s (was %s) at a planned restart dur=%s", rec.RunID, was, time.Since(now).Round(time.Millisecond))
			return true
		}
		rec.State = types.RunPassivated
		rec.Error = ""
		rec.UpdatedAt = now
		rec.FinishedAt = nil
		rec.Metadata = cloneMetadata(rec.Metadata)
		rec.Metadata["passivated_reason"] = "runtime_restarted"
		// No spawned work item here: the spawn created it (ensure at
		// spawnCoagent), and the resume reads only the state and reason.
		// Ensuring it again looked it up by a JSON scan of every work item,
		// 75 s on the paper (docs/problems/paper-post-refresh-health-windows-2026-10-22.md A3).
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			log.Printf("runtime: boot passivation: update run %s: %v", rec.RunID, err)
			return false
		}
		if runHasProfile(rec, agentprofile.Texture) {
			if err := rt.store.MarkAgentMutationStale(context.WithoutCancel(ctx), rec.OwnerID, agentMutationComputerID(rec), rec.RunID); err != nil {
				log.Printf("runtime: boot passivation: stale mutation %s: %v", rec.RunID, err)
			}
		}
		rt.emitEvent(ctx, rec, types.EventRunPassivated, events.CauseSupervisorRecovery,
			passivatedOnRestartPayload(rt.bootRestart))
		// Each run's duration and profile, so a slow step names itself.
		log.Printf("runtime: passivated run %s (was %s) after restart profile=%s dur=%s", rec.RunID, was, agentProfileForRun(rec), time.Since(now).Round(time.Millisecond))
		return true
	})
}

// bootPersistentManagementRewarmLimit caps the owner-indexed header window used
// to find passivated Management control runs. ListAllRunsByState(passivated)
// JSON_EXTRACT-scans every choir.run body and is what kills the 4 GiB guest
// before Management dispatch.
const bootPersistentManagementRewarmLimit = 1024

// actorWakeSweep reports what one outbox pass did, so the projector can
// decide when to look again.
// wakeLatencyLogAfter is the wait, outbox row to dispatch or inside the
// dispatch, past which the sweep logs it.
const wakeLatencyLogAfter = 2 * time.Second

type actorWakeSweep struct {
	dispatched int
	// more: wakes were left for pacing or budget; sweep again at once.
	more bool
}

// sweepActorWakeOutbox reads the whole outbox: the start, audit, retry and
// pacing sweeps.
func (rt *Runtime) sweepActorWakeOutbox(ctx context.Context) actorWakeSweep {
	return rt.sweepActorWakes(ctx, false)
}

// sweepActorWakes dispatches unprojected wakes. A signalled sweep (announced)
// reads only the outbox rows committed since the last one, by key; it reads
// the whole outbox when a commit did not name its rows. Listing the whole
// outbox matches every row ever written, 2 to 4 s on the paper before each
// wake (docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6).
func (rt *Runtime) sweepActorWakes(ctx context.Context, announced bool) actorWakeSweep {
	var out actorWakeSweep
	if rt == nil || rt.store == nil {
		return out
	}
	listStarted := time.Now()
	var wakes []store.ActorWakeOutbox
	full := !announced
	var err error
	if announced {
		wakes, full, err = rt.store.TakeAnnouncedActorWakes(ctx)
	}
	if err == nil && full {
		wakes, err = rt.store.ListUnprojectedActorWakes(ctx)
	}
	if err != nil {
		log.Printf("runtime: actor wake outbox sweep: %v", err)
		return out
	}
	// Wake latency (docs/problems/research-more-slow-and-hidden-2026-10-21.md
	// E6): a slow outbox listing delays every wake behind it.
	if listed := time.Since(listStarted); listed >= wakeLatencyLogAfter {
		log.Printf("runtime: actor wake outbox list wakes=%d dur=%s", len(wakes), listed.Round(time.Millisecond))
	}
	// The projector goroutine runs beside hook setters, so read the hooks as
	// one locked snapshot.
	rt.deliveryHooksMu.RLock()
	dispatchActor, scheduleActor := rt.dispatchActor, rt.scheduleActor
	dispatch, schedule := rt.dispatchActorRedrive, rt.scheduleActorRedrive
	rt.deliveryHooksMu.RUnlock()
	if dispatchActor == nil && scheduleActor == nil {
		log.Printf("runtime: actor wake outbox sweep: actor delivery unavailable")
		return out
	}
	// The sweep is the obligation re-drive authority: dispatch through the
	// redrive hooks so a wake whose deterministic tape row was already consumed
	// (delivered-bound run died before consuming its control) mints a salted
	// generation instead of deduping into nothing. First-time wakes are
	// unaffected — an empty family keeps the base update_id.
	if dispatch == nil {
		dispatch = dispatchActor
	}
	if schedule == nil {
		schedule = scheduleActor
	}
	// Paced drain: one target desk gets at most a handful of dispatches per
	// sweep. A boot storm that mints thousands of wakes for the same desk
	// collapses to a bounded burst per pass — resolve folds the desk's whole
	// pending set anyway, so extra dispatches to a hot desk carry no
	// additional obligation (sa1 rearm storm).
	perTarget := map[string]int{}
	const sweepPerTargetLimit = 4
	const sweepDispatchBudget = 64
	for _, wake := range wakes {
		if rt.preBootWorkWake(wake.Kind, wake.CreatedAt) {
			if err := rt.store.MarkActorWakeInterrupted(ctx, wake.CanonicalID); err != nil {
				log.Printf("runtime: actor wake outbox interrupt pre-boot wake %s: %v", wake.SourceUpdateID, err)
			} else {
				log.Printf("runtime: actor wake outbox: pre-boot wake %s (%s) interrupted_by_restart", wake.SourceUpdateID, wake.Kind)
			}
			continue
		}
		if !rt.wakeRetries.due(wake.CanonicalID) {
			continue // backing off after a failed dispatch
		}
		var dispatchErr error
		if !wake.NotBefore.IsZero() {
			if schedule == nil {
				log.Printf("runtime: actor wake outbox schedule unavailable for %s", wake.SourceUpdateID)
				continue
			}
			dispatchErr = schedule(ctx, wake.OwnerID, wake.ComputerID, wake.TargetAgentID, wake.Kind, wake.Content, wake.TrajectoryID, wake.AgentID, wake.NotBefore.UTC())
		} else {
			if out.dispatched >= sweepDispatchBudget {
				out.more = true
				break // leave the rest unprojected for the next pass
			}
			if perTarget[wake.TargetAgentID] >= sweepPerTargetLimit {
				out.more = true
				continue
			}
			dispatchStarted := time.Now()
			dispatchErr = dispatch(ctx, wake.OwnerID, wake.ComputerID, wake.TargetAgentID, wake.Kind, wake.Content, wake.TrajectoryID, wake.AgentID)
			// From the wake's own mint, not its source's creation: a wake
			// minted by a later version of an old assignment is not that old.
			minted := wake.MintedAt
			if minted.IsZero() {
				minted = wake.CreatedAt
			}
			if waited := dispatchStarted.Sub(minted); dispatchErr == nil && (waited >= wakeLatencyLogAfter || time.Since(dispatchStarted) >= wakeLatencyLogAfter) {
				log.Printf("runtime: actor wake latency kind=%s target=%s waited=%s dispatch=%s", wake.Kind, wake.TargetAgentID, waited.Round(time.Millisecond), time.Since(dispatchStarted).Round(time.Millisecond))
			}
		}
		if dispatchErr != nil {
			if errors.Is(dispatchErr, store.ErrNoPendingActorOccurrence) {
				// The backing occurrence is gone (consumed, disposed, or
				// delivery-exhausted before this wake projected). Retrying can
				// never succeed — mark projected so the wake leaves the drain.
				rt.wakeRetries.clear(wake.CanonicalID)
				if markErr := rt.store.MarkActorWakeProjected(ctx, wake.CanonicalID); markErr != nil {
					log.Printf("runtime: actor wake outbox dispose dead wake %s: %v", wake.SourceUpdateID, markErr)
				} else {
					log.Printf("runtime: actor wake outbox disposed dead wake %s", wake.SourceUpdateID)
				}
				continue
			}
			attempts, exhausted := rt.wakeRetries.failed(wake.CanonicalID, dispatchErr)
			if !exhausted {
				log.Printf("runtime: actor wake outbox dispatch %s failed (attempt %d/%d, backing off): %v", wake.SourceUpdateID, attempts, wakeDispatchMaxAttempts, dispatchErr)
				continue
			}
			// O1: a bounded number of attempts, then a recorded, visible fate.
			if markErr := rt.store.MarkActorWakeExhausted(ctx, wake.CanonicalID, attempts, dispatchErr.Error()); markErr != nil {
				log.Printf("runtime: actor wake outbox mark %s exhausted: %v", wake.SourceUpdateID, markErr)
				continue
			}
			rt.wakeRetries.clear(wake.CanonicalID)
			log.Printf("runtime: actor wake outbox dispatch %s exhausted after %d attempts: %v", wake.SourceUpdateID, attempts, dispatchErr)
			continue
		}
		rt.wakeRetries.clear(wake.CanonicalID)
		perTarget[wake.TargetAgentID]++
		out.dispatched++
		if err := rt.store.MarkActorWakeProjected(ctx, wake.CanonicalID); err != nil && !errors.Is(err, store.ErrConcurrentStateChange) {
			log.Printf("runtime: actor wake outbox mark projected %s: %v", wake.SourceUpdateID, err)
		}
	}
	return out
}

// SetKernelMode enables the derivable-continuation projector. Called by the
// adapter when the dispatcher is the delivery authority. It also marks the
// store post-ontology-kernel so bare OG state transitions that bypass the
// canonical reducer path fail closed instead of writing a non-event-backed
// mutation.
func (rt *Runtime) SetKernelMode() {
	rt.kernelMode = true
	if rt.store != nil {
		rt.store.SetKernelMode()
	}
}

// migrateActorWakeOutboxAsync mints durable actor-wake outbox objects for
// every obligation-bearing canonical object that predates the outbox fold, so
// the projector delivers pre-cutover pending rows from the tape. It runs off
// the boot-critical path: each mint is a projection append (network CAS), so
// doing N of them synchronously inside New/SetKernelMode stalls boot for a
// populated account. Idempotent - deterministic wake keys make re-entry a
// no-op; the projector delivers minted wakes on its next tick.
func (rt *Runtime) migrateActorWakeOutboxAsync(ctx context.Context) {
	if rt == nil || rt.store == nil || !rt.kernelMode {
		return
	}
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		minted, err := rt.store.MigrateActorWakeOutbox(ctx)
		if err != nil {
			log.Printf("runtime: actor wake outbox migration: %v", err)
		} else if minted > 0 {
			log.Printf("runtime: actor wake outbox migration minted %d pending wakes", minted)
		}
	}()
}

// startProjector launches the store→actor projection. It drains the durable
// metadata-indexed actor-wake outbox when the store signals a committed wake
// write, never scanning the canonical worker-update backlog. The fold only appends idempotent actor
// occurrences and marks successful outbox entries projected; activation owns
// all resident-run binding.
func (rt *Runtime) startProjector(ctx context.Context) {
	if !rt.kernelMode || rt.projectorStop != nil {
		return
	}
	rt.projectorStop = make(chan struct{})
	// Capture the channel locally: the goroutine must never read rt.projectorStop
	// (stopProjector writes it to nil under Stop, which would race this select).
	stop := rt.projectorStop
	rt.wg.Add(1)
	go func() {
		defer rt.wg.Done()
		rt.runActorWakeProjector(ctx, stop)
	}()
}

// stopProjector halts the projection loop.
func (rt *Runtime) stopProjector() {
	if rt.projectorStop != nil {
		close(rt.projectorStop)
		rt.projectorStop = nil
	}
}

// ReconcileLifecycleWorkAssignment applies the same durable work-item routing
// as boot recovery to one canonical assignment wake. A closed, replaced, or
// already-claimed work item is deliberately a no-op.
func (rt *Runtime) ReconcileLifecycleWorkAssignment(ctx context.Context, ownerID, computerID, agentID, trajectoryID, workItemID string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("lifecycle work assignment: store unavailable")
	}
	work, err := rt.store.GetLifecycleWorkItem(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(workItemID))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if work.Status != types.WorkItemOpen || work.OwnerID != strings.TrimSpace(ownerID) ||
		work.ComputerID != strings.TrimSpace(computerID) || work.AssignedAgentID != strings.TrimSpace(agentID) ||
		work.TrajectoryID != strings.TrimSpace(trajectoryID) {
		return nil
	}
	if intent, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, work.OwnerID, work.ComputerID, work.TrajectoryID); intentErr == nil {
		_, _, cancelErr := rt.CancelTrajectoryCommand(ctx, work.TrajectoryID, work.OwnerID, intent.CommandID, intent.Reason, intent.RequestedLifecycleVersion, intent.ExpectedHeadRevisionID)
		return cancelErr
	} else if !errors.Is(intentErr, store.ErrNotFound) {
		return intentErr
	}
	profile, _ := agentprofile.Canonical(work.AuthorityProfile)
	switch profile {
	case agentprofile.Engineering:
		return rt.ReconcileEngineeringAssignmentsForTrajectory(ctx, work.OwnerID, work.ComputerID, work.TrajectoryID)
	case agentprofile.Research:
		updates, updateErr := rt.store.ListAllPendingLifecycleUpdates(ctx, work.OwnerID, work.ComputerID, work.AssignedAgentID)
		if updateErr != nil {
			return updateErr
		}
		if len(updates) > 0 {
			_, reconcileErr := rt.reconcileUpdatedCoagentActor(ctx, work.OwnerID, work.AssignedAgentID)
			if errors.Is(reconcileErr, ErrDurablyTerminalLifecycleControlActivation) {
				return nil
			}
			return reconcileErr
		}
		// A spawned Research work item carries no pending control - the open
		// work item itself is the obligation, so bind an actor for it.
		_, reconcileErr := rt.reconcileAssignedWorkItemActor(ctx, []types.WorkItemRecord{work})
		return reconcileErr
	default:
		_, reconcileErr := rt.reconcileAssignedWorkItemActor(ctx, []types.WorkItemRecord{work})
		return reconcileErr
	}
}

// HandleLifecycleCancellationWake re-drives the durable cancellation authority;
// once the intent has been consumed or the trajectory is terminal it has no
// further effect.
func (rt *Runtime) HandleLifecycleCancellationWake(ctx context.Context, ownerID, computerID, agentID, trajectoryID string) error {
	if strings.TrimSpace(agentID) != persistentManagementAgentID(strings.TrimSpace(ownerID)) {
		return nil
	}
	intent, err := rt.store.GetLifecycleCancellationIntent(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(trajectoryID))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, _, err = rt.CancelTrajectoryCommand(ctx, trajectoryID, ownerID, intent.CommandID, intent.Reason, intent.RequestedLifecycleVersion, intent.ExpectedHeadRevisionID)
	return err
}

func (rt *Runtime) reconcileAssignedWorkItemActor(ctx context.Context, workItems []types.WorkItemRecord) (*types.RunRecord, error) {
	return rt.reconcileAssignedWorkItemActorWithSource(ctx, workItems, "trajectory_work_item_sweep")
}

func (rt *Runtime) reconcileAssignedWorkItemActorWithSource(ctx context.Context, workItems []types.WorkItemRecord, requestSource string) (*types.RunRecord, error) {
	if rt == nil || rt.store == nil || len(workItems) == 0 {
		return nil, nil
	}
	rt.lifecycleWorkReconcileMu.Lock()
	reconcileLocked := true
	defer func() {
		if reconcileLocked {
			rt.lifecycleWorkReconcileMu.Unlock()
		}
	}()
	first := workItems[0]
	ownerID := strings.TrimSpace(first.OwnerID)
	agentID := strings.TrimSpace(first.AssignedAgentID)
	trajectoryID := strings.TrimSpace(first.TrajectoryID)
	computerID := firstNonEmpty(strings.TrimSpace(first.ComputerID), rt.TextureComputerID())
	if ownerID == "" || agentID == "" || trajectoryID == "" || computerID == "" {
		return nil, nil
	}
	if first.LifecycleVersion > 0 {
		snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
		if err != nil {
			return nil, fmt.Errorf("load assigned work-item lifecycle snapshot: %w", err)
		}
		currentWork := make(map[string]types.WorkItemRecord, len(snapshot.WorkItems))
		for _, item := range snapshot.WorkItems {
			currentWork[strings.TrimSpace(item.WorkItemID)] = item
		}
		pendingTerminal := make(map[string]bool)
		for _, update := range snapshot.Updates {
			if update.AgentID == agentID && update.Disposition == types.UpdatePending &&
				update.WorkDisposition != "" && update.WorkDisposition != types.WorkItemOpen {
				pendingTerminal[update.WorkItemID] = true
			}
		}
		filtered := workItems[:0]
		for _, stale := range workItems {
			item, found := currentWork[strings.TrimSpace(stale.WorkItemID)]
			if found && item.Status == types.WorkItemOpen &&
				strings.TrimSpace(item.OwnerID) == ownerID &&
				firstNonEmpty(strings.TrimSpace(item.ComputerID), computerID) == computerID &&
				strings.TrimSpace(item.TrajectoryID) == trajectoryID &&
				strings.TrimSpace(item.AssignedAgentID) == agentID &&
				!pendingTerminal[item.WorkItemID] {
				filtered = append(filtered, item)
			}
		}
		workItems = filtered
		if len(workItems) == 0 {
			return nil, nil
		}
		first = workItems[0]
	}
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident assigned work-item actor: %w", err)
	} else if found {
		return &resident, nil
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, firstNonEmpty(first.ComputerID, rt.TextureComputerID()), agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup assigned work-item actor: %w", err)
	}
	profile, _ := agentprofile.Canonical(firstNonEmpty(agent.Profile, first.AuthorityProfile))
	if profile == agentprofile.Research && (agent.LifecycleVersion > 0 || first.LifecycleVersion > 0) {
		// Lifecycle Research are activated only by the exact Texture-control
		// fingerprint reconciler. Generic assigned-work recovery preserves the
		// open obligation without creating or dispatching a run.
		return nil, nil
	}
	switch profile {
	case agentprofile.Research, frozenProcessorProfile, frozenReconcilerProfile:
		// Texture reconstruction belongs to textureowner.ReconcileAgentWake,
		// which derives revision authority from the canonical document head.
		// A generic assigned-work run cannot safely synthesize that authority.
	case agentprofile.Texture:
		return nil, nil
	default:
		return nil, nil
	}
	agentRole, _ := agentprofile.Canonical(agent.Role)
	if agentRole != profile {
		return nil, fmt.Errorf("assigned lifecycle actor has conflicting profile and role")
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
		"request_source":        firstNonEmpty(strings.TrimSpace(requestSource), "trajectory_work_item_sweep"),
		"work_item_ids":         ids,
	}
	if len(ids) == 1 {
		metadata["lifecycle_work_item_id"] = ids[0]
	}
	if channelID != "" {
		metadata[runMetadataChannelID] = channelID
	}
	metadata = inheritRequesterMetadataFromWorkItem(ctx, rt.store, ownerID, metadata, first)
	rec, err := rt.createRunWithMetadata(ctx, buildAssignedWorkItemPromptForCarrier(workItems, profile == agentprofile.Management || profile == agentprofile.Research), ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if profile == agentprofile.Research {
		updates, listErr := rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
		if listErr != nil {
			return nil, listErr
		}
		updates, listErr = rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, updates, false)
		if listErr != nil {
			return nil, listErr
		}
		updates = selectLifecycleControlActivation(updates, trajectoryID, lifecycleControlWorkIDsForRun(rec))
		if len(updates) > 0 {
			if _, bindErr := rt.bindLifecycleControlsToRun(ctx, rec, updates); bindErr != nil {
				rt.failUnactivatedLifecycleControlRun(ctx, rec, bindErr)
				return nil, bindErr
			}
		}
	}
	rt.lifecycleWorkReconcileMu.Unlock()
	reconcileLocked = false
	rt.activate(rec)
	return rec, nil
}

func buildAssignedWorkItemPromptForCarrier(workItems []types.WorkItemRecord, choirCarrier bool) string {
	var b strings.Builder
	b.WriteString("Resume the open trajectory work item records assigned to you.\n")
	if choirCarrier {
		b.WriteString("These durable obligations remain open in canonical state. Before ending this activation, use choir.Report to assert work_disposition=completed only when the assigned lifecycle work is fully satisfied; otherwise use choir.Report with work_disposition=open and a precise blocker. Use choir.Cast to open assignments. Final text and RunRecord completion do not settle work.\n")
	} else {
		b.WriteString("These durable obligations remain open in canonical state. Before ending this activation, call update_coagent with work_disposition=completed only when the assigned lifecycle work is fully satisfied; otherwise send work_disposition=open with a precise blocker. Final text and RunRecord completion do not settle work.\n")
	}
	if len(workItems) > 1 {
		if choirCarrier {
			b.WriteString("This activation carries multiple work items. Every choir.Report must set work_item_id to the specific item it addresses.\n")
		} else {
			b.WriteString("This activation carries multiple work items. Every update_coagent call must set work_item_id to the specific item it addresses.\n")
		}
	}
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

// admitLifecycleResearchProviderEntry is the single provider-boundary
// authority check for durable lifecycle Research. Open work records are
// inspectable obligations, not provider authority: execution requires one exact
// current Texture-control fingerprint joined to the canonical run. Legacy
// (lifecycle-version-zero) Research runs remain outside this invariant.
func (rt *Runtime) admitLifecycleResearchProviderEntry(ctx context.Context, rec *types.RunRecord) (lifecycle, admitted bool, reason string, err error) {
	if rt == nil || rt.store == nil || rec == nil {
		return false, false, "runtime or run unavailable", nil
	}
	profile, _ := agentprofile.Canonical(rec.AgentProfile)
	role, _ := agentprofile.Canonical(rec.AgentRole)
	if profile != agentprofile.Research && role != agentprofile.Research {
		return false, true, "", nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	computerID := strings.TrimSpace(rec.ComputerID)
	trajectoryID := strings.TrimSpace(rec.TrajectoryID)
	agentID := strings.TrimSpace(rec.AgentID)
	declaresLifecycleControl := metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" ||
		metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata) != "" ||
		metadataStringValue(rec.Metadata, lifecycleFailedAttemptKeyMetadata) != "" ||
		metadataStringValue(rec.Metadata, lifecycleActivationBuildMetadata) != ""
	if _, hasVersions := rec.Metadata[lifecycleActivationVersionsMetadata]; hasVersions {
		declaresLifecycleControl = true
	}
	if ownerID == "" || computerID == "" || agentID == "" {
		return true, false, "Research run has incomplete owner/computer/agent authority", nil
	}
	if trajectoryID == "" {
		if declaresLifecycleControl {
			return true, false, "declared lifecycle Research has no trajectory authority", nil
		}
		return false, true, "", nil
	}
	if !declaresLifecycleControl {
		scopedAgent, scopeErr := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
		if errors.Is(scopeErr, store.ErrNotFound) {
			return false, true, "", nil
		}
		if scopeErr != nil {
			return true, false, "", fmt.Errorf("classify lifecycle Research agent: %w", scopeErr)
		}
		if scopedAgent.LifecycleVersion <= 0 {
			// A legacy version-zero Research stays outside lifecycle provider
			// admission even when its historical metadata names a lifecycle
			// trajectory. Only Store-owned lifecycle subject version or an exact
			// Texture-control fingerprint opts into this boundary.
			return false, true, "", nil
		}
		declaresLifecycleControl = true
	}
	trajectory, loadErr := rt.store.GetLifecycleTrajectory(ctx, ownerID, computerID, trajectoryID)
	if errors.Is(loadErr, store.ErrNotFound) {
		return true, false, "declared lifecycle Research has no canonical trajectory authority", nil
	}
	if loadErr != nil {
		return true, false, "", fmt.Errorf("load lifecycle trajectory: %w", loadErr)
	}
	lifecycle = true
	deny := func(why string) (bool, bool, string, error) { return true, false, why, nil }
	if ctx.Err() != nil {
		return deny("activation context is cancelled")
	}
	if profile != agentprofile.Research || role != agentprofile.Research ||
		trajectory.OwnerID != ownerID || trajectory.ComputerID != computerID || trajectory.TrajectoryID != trajectoryID ||
		trajectory.Status != types.TrajectoryLive {
		return deny("lifecycle Research scope is not exact and live")
	}
	if _, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, trajectoryID); intentErr == nil {
		return deny("trajectory has cancellation intent")
	} else if !errors.Is(intentErr, store.ErrNotFound) {
		return true, false, "", fmt.Errorf("load lifecycle cancellation intent: %w", intentErr)
	}

	stored, loadErr := rt.store.GetLifecycleRun(ctx, ownerID, computerID, rec.RunID)
	if loadErr != nil {
		return true, false, "", fmt.Errorf("load exact lifecycle Research run: %w", loadErr)
	}
	storedProfile, _ := agentprofile.Canonical(stored.AgentProfile)
	storedRole, _ := agentprofile.Canonical(stored.AgentRole)
	if stored.RunID != strings.TrimSpace(rec.RunID) || stored.OwnerID != ownerID || stored.ComputerID != computerID ||
		stored.TrajectoryID != trajectoryID || stored.AgentID != agentID || stored.State != rec.State ||
		(stored.State != types.RunPending && stored.State != types.RunRunning) ||
		storedProfile != agentprofile.Research || storedRole != agentprofile.Research {
		return deny("run projection is stale or outside the exact lifecycle Research scope")
	}
	agent, loadErr := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if loadErr != nil {
		return true, false, "", fmt.Errorf("load exact lifecycle Research agent: %w", loadErr)
	}
	scopeAgentProfile, _ := agentprofile.Canonical(agent.Profile)
	scopeAgentRole, _ := agentprofile.Canonical(agent.Role)
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.AgentID != agentID || agent.LifecycleVersion <= 0 ||
		scopeAgentProfile != agentprofile.Research || scopeAgentRole != agentprofile.Research ||
		strings.TrimSpace(agent.ActiveRunID) != stored.RunID {
		return deny("Research agent does not own the exact live run")
	}
	if metadataStringValue(stored.Metadata, "request_source") == inboxRecordRequestSource {
		// Records route (Cut 1 L3a): the authority is unread records or an
		// open round addressed to this desk, not a control fingerprint.
		unread, unreadErr := rt.inboxUnread(ctx, ownerID, computerID, agentID)
		if unreadErr != nil {
			return true, false, "", fmt.Errorf("load Research inbox: %w", unreadErr)
		}
		if !unread {
			rounds, roundsErr := rt.store.OpenRounds(ctx, ownerID, computerID, agentID)
			if roundsErr != nil {
				return true, false, "", fmt.Errorf("load Research open rounds: %w", roundsErr)
			}
			for _, round := range rounds {
				if round.RecipientInbox == agentID {
					unread = true
					break
				}
			}
		}
		if !unread {
			return deny("Research desk has no unread records and no open round addressed to it")
		}
		*rec = stored
		return true, true, "", nil
	}
	if metadataStringValue(stored.Metadata, "request_source") != "lifecycle_texture_control" ||
		metadataStringValue(stored.Metadata, lifecycleLogicalActivationKeyMetadata) == "" ||
		metadataStringValue(stored.Metadata, lifecycleFailedAttemptKeyMetadata) == "" ||
		metadataStringValue(stored.Metadata, lifecycleActivationBuildMetadata) == "" {
		return deny("run has no exact lifecycle Texture-control fingerprint")
	}
	versions, versionErr := lifecycleActivationVersionsForRun(&stored)
	if versionErr != nil || len(versions) == 0 {
		return deny("run has no valid lifecycle control/work versions")
	}
	snapshot, loadErr := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if loadErr != nil {
		return true, false, "", fmt.Errorf("load lifecycle Research admission snapshot: %w", loadErr)
	}
	workByID := make(map[string]types.WorkItemRecord, len(snapshot.WorkItems))
	for _, work := range snapshot.WorkItems {
		workByID[strings.TrimSpace(work.WorkItemID)] = work
	}
	controlByID := make(map[string]types.CoagentSourcePacket, len(snapshot.Updates))
	for _, update := range snapshot.Updates {
		controlByID[strings.TrimSpace(update.UpdateID)] = update
	}
	deliveredByID := map[string]types.CoagentSourcePacket{}
	var after int64
	for {
		page, pageErr := rt.store.ListLifecycleControlsDeliveredToRunPage(ctx, ownerID, computerID, trajectoryID, agentID, stored.RunID, after, 100)
		if pageErr != nil {
			return true, false, "", fmt.Errorf("load canonical lifecycle control delivery: %w", pageErr)
		}
		for _, control := range page.Packets {
			deliveredByID[strings.TrimSpace(control.UpdateID)] = control
		}
		if !page.HasMore {
			break
		}
		if page.NextCursor <= after {
			return deny("canonical lifecycle control delivery cursor did not advance")
		}
		after = page.NextCursor
	}

	fingerprintUpdates := make([]types.CoagentSourcePacket, 0, len(versions))
	fingerprintWork := make(map[string]types.WorkItemRecord, len(versions))
	seenUpdates := make(map[string]bool, len(versions))
	deliveryMode := ""
	for _, version := range versions {
		updateID := strings.TrimSpace(version.UpdateID)
		workID := strings.TrimSpace(version.TargetWorkItemID)
		if updateID == "" || workID == "" || version.ControlLifecycleVersion <= 0 || version.WorkLifecycleVersion <= 0 || seenUpdates[updateID] {
			return deny("lifecycle control fingerprint contains an invalid version join")
		}
		seenUpdates[updateID] = true
		work, ok := workByID[workID]
		if !ok || work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != trajectoryID ||
			work.AssignedAgentID != agentID || work.Status != types.WorkItemOpen || work.LifecycleVersion != version.WorkLifecycleVersion ||
			!lifecycleControlWorkIDsForRun(&stored)[workID] {
			return deny("lifecycle control is not joined to the exact open work version")
		}
		control, ok := controlByID[updateID]
		if !ok || control.OwnerID != ownerID || control.ComputerID != computerID || control.TrajectoryID != trajectoryID ||
			control.TargetAgentID != agentID || control.TargetWorkItemID != workID || control.Direction != types.LifecyclePacketDirectionControl ||
			control.Packet.SchemaVersion != types.CoagentSourcePacketSchemaV1 || strings.TrimSpace(control.Content) == "" {
			return deny("lifecycle control fingerprint does not name the exact canonical Texture control")
		}
		payloadDigest, digestErr := store.ComputeLifecycleUpdatePayloadDigest(control.Packet, control.Content)
		if digestErr != nil || payloadDigest != control.PayloadDigest {
			return deny("lifecycle control payload digest is invalid")
		}
		sourceAgent, sourceErr := rt.store.GetAgentByScope(ctx, ownerID, computerID, strings.TrimSpace(control.AgentID))
		if sourceErr != nil {
			return true, false, "", fmt.Errorf("load lifecycle control source Texture: %w", sourceErr)
		}
		sourceTextureProfile, _ := agentprofile.Canonical(sourceAgent.Profile)
		sourceTextureRole, _ := agentprofile.Canonical(sourceAgent.Role)
		if sourceAgent.OwnerID != ownerID || sourceAgent.ComputerID != computerID || sourceAgent.LifecycleVersion <= 0 ||
			sourceTextureProfile != agentprofile.Texture || sourceTextureRole != agentprofile.Texture ||
			sourceAgent.ChannelID == "" || sourceAgent.ChannelID != control.ChannelID || control.ChannelID != snapshot.Document.DocID ||
			control.AgentID != sourceAgent.AgentID || snapshot.Document.TrajectoryID != trajectoryID {
			return deny("lifecycle control source is not the exact document-bound lifecycle Texture")
		}
		mode := "pending"
		if delivered, ok := deliveredByID[updateID]; ok {
			mode = "delivered"
			if delivered.DeliveredAt == nil || strings.TrimSpace(delivered.DeliveredToRunID) != stored.RunID ||
				control.DeliveredAt == nil || strings.TrimSpace(control.DeliveredToRunID) != stored.RunID ||
				(control.Disposition != types.UpdatePending && control.Disposition != types.UpdateIncorporated) ||
				control.LifecycleVersion <= version.ControlLifecycleVersion {
				return deny("canonical lifecycle control delivery does not match its fingerprinted run/version")
			}
		} else if control.Disposition != types.UpdatePending || control.DeliveredAt != nil || strings.TrimSpace(control.DeliveredToRunID) != "" ||
			control.LifecycleVersion != version.ControlLifecycleVersion {
			return deny("lifecycle control is neither exact pending authority nor canonically delivered to this run")
		}
		if deliveryMode != "" && deliveryMode != mode {
			return deny("lifecycle control fingerprint is only partially delivered")
		}
		deliveryMode = mode
		fingerprintControl := control
		fingerprintControl.LifecycleVersion = version.ControlLifecycleVersion
		fingerprintUpdates = append(fingerprintUpdates, fingerprintControl)
		fingerprintWork[workID] = work
	}
	logicalKey, failedKey, _, keyErr := lifecycleActivationKeys(ownerID, computerID, trajectoryID, agentID,
		metadataStringValue(stored.Metadata, lifecycleActivationBuildMetadata), fingerprintUpdates, fingerprintWork)
	if keyErr != nil || logicalKey != metadataStringValue(stored.Metadata, lifecycleLogicalActivationKeyMetadata) ||
		failedKey != metadataStringValue(stored.Metadata, lifecycleFailedAttemptKeyMetadata) {
		return deny("lifecycle control fingerprint does not match the canonical control/work versions")
	}
	if deliveryMode == "pending" {
		// Close the projection-before-bind crash cut before provider entry. The
		// Store CAS joins the exact pending fingerprint to this run; a conflict or
		// reload failure is a zero-provider retry, never permission to proceed.
		if _, bindErr := rt.bindLifecycleControlsToRun(ctx, &stored, fingerprintUpdates); bindErr != nil {
			return true, false, "", fmt.Errorf("bind exact pending lifecycle controls before provider entry: %w", bindErr)
		}
		*rec = stored
		return rt.admitLifecycleResearchProviderEntry(ctx, rec)
	}
	*rec = stored
	return true, true, "", nil
}

const (
	lifecycleResearchAdmissionRefusalReason = "lifecycle_researcher_provider_admission_refused"
	lifecycleResearchAdmissionRetryReason   = "lifecycle_researcher_provider_admission_retry"
	activationRetryableErrorMetadata        = "runtime_activation_retryable_error"
)

func markActivationRetryableError(rec *types.RunRecord, err error) {
	if rec == nil || err == nil {
		return
	}
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata[activationRetryableErrorMetadata] = strings.TrimSpace(err.Error())
}

func (rt *Runtime) passivateLifecycleResearchWithoutProviderAuthority(ctx context.Context, rec *types.RunRecord, reason string) error {
	return rt.passivateLifecycleResearchProviderEntry(ctx, rec, reason, lifecycleResearchAdmissionRefusalReason)
}

func (rt *Runtime) passivateLifecycleResearchAfterAdmissionError(ctx context.Context, rec *types.RunRecord, admissionErr error) error {
	if admissionErr == nil {
		return nil
	}
	return rt.passivateLifecycleResearchProviderEntry(ctx, rec, admissionErr.Error(), lifecycleResearchAdmissionRetryReason)
}

func (rt *Runtime) passivateLifecycleResearchProviderEntry(ctx context.Context, rec *types.RunRecord, reason, passivatedReason string) error {
	if rt == nil || rt.store == nil || rec == nil || strings.TrimSpace(rec.RunID) == "" {
		return fmt.Errorf("passivate lifecycle Research: runtime, store, and run are required")
	}
	stored, err := rt.getRunForComputer(context.WithoutCancel(ctx), rec.OwnerID, rec.RunID)
	if err != nil {
		return fmt.Errorf("load lifecycle Research for passivation: %w", err)
	}
	if !stored.State.Active() {
		*rec = stored
		return nil
	}
	if _, intentErr := rt.store.GetLifecycleCancellationIntent(context.WithoutCancel(ctx), stored.OwnerID, stored.ComputerID, stored.TrajectoryID); intentErr == nil {
		*rec = stored
		return nil
	} else if !errors.Is(intentErr, store.ErrNotFound) {
		return fmt.Errorf("load lifecycle Research cancellation intent before passivation: %w", intentErr)
	}
	stored.State = types.RunPassivated
	stored.Error = ""
	stored.Result = ""
	stored.FinishedAt = nil
	stored.UpdatedAt = time.Now().UTC()
	stored.Metadata = cloneMetadata(stored.Metadata)
	stored.Metadata["passivated_reason"] = strings.TrimSpace(passivatedReason)
	stored.Metadata["provider_admission_refusal"] = strings.TrimSpace(reason)
	delete(stored.Metadata, activationRetryableErrorMetadata)
	if err := rt.store.UpdateRun(context.WithoutCancel(ctx), stored); err != nil {
		return fmt.Errorf("persist lifecycle Research provider-admission passivation: %w", err)
	}
	*rec = stored
	return nil
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
	}()

	lifecycleResearch, admitted, refusal, admissionErr := rt.admitLifecycleResearchProviderEntry(ctx, rec)
	if admissionErr != nil {
		if passivateErr := rt.passivateLifecycleResearchAfterAdmissionError(ctx, rec, admissionErr); passivateErr != nil {
			markActivationRetryableError(rec, fmt.Errorf("provider admission failed (%v) and retry passivation failed: %w", admissionErr, passivateErr))
		}
		log.Printf("runtime: refuse lifecycle Research provider entry for run %s: %v", rec.RunID, admissionErr)
		return
	}
	if lifecycleResearch && !admitted {
		if passivateErr := rt.passivateLifecycleResearchWithoutProviderAuthority(ctx, rec, refusal); passivateErr != nil {
			markActivationRetryableError(rec, fmt.Errorf("provider admission refusal %q could not passivate: %w", refusal, passivateErr))
		}
		log.Printf("runtime: lifecycle Research run %s remains idle: %s", rec.RunID, refusal)
		return
	}

	now := time.Now().UTC()

	// Transition to running.
	rec.State = types.RunRunning
	rec.UpdatedAt = now
	persisted, err := rt.persistActivationStateAndEmit(ctx, rec, types.EventRunStarted, events.CauseTaskLifecycle,
		json.RawMessage(`{}`))
	if err != nil {
		log.Printf("runtime: update run %s to running: %v", rec.RunID, err)
		rt.handleExecutionError(ctx, rec, fmt.Errorf("update run state: %w", err))
		return
	}
	if !persisted {
		return
	}

	emit := rt.runEventEmitter(ctx, rec)

	registry := rt.toolRegistryForRun(rec)
	overlayProfile := agentProfileForRun(rec)
	if overlayProfile == agentprofile.Engineering {
		var bindErr error
		registry, _, bindErr = rt.assignedEngineeringToolOverlay(ctx, rec, registry)
		if bindErr != nil {
			rt.handleExecutionError(ctx, rec, fmt.Errorf("bind assigned Engineering registry: %w", bindErr))
			return
		}
	}

	// Use the tool-calling loop if a tool registry is configured and the
	// provider supports the provideriface.ToolLoopProvider interface. Otherwise, fall back
	// to the simple Provider.Execute path.
	if registry != nil && registry.Size() > 0 {
		rt.executeWithToolLoop(ctx, rec, registry, emit)
	} else {
		rt.executeWithProvider(ctx, rec, emit)
	}
}

// runEventEmitter returns the emit function a run's provider loop reports
// progress through. The store keeps the payload, not EventRecord.Phase, so a
// progress event's phase ("tool_loop", "tool_loop_budget_usage", ...) is
// written into its payload unless the payload names one already; cost
// extraction and the trace export read it there.
func (rt *Runtime) runEventEmitter(ctx context.Context, rec *types.RunRecord) provideriface.EventEmitFunc {
	return func(kind types.EventKind, phase string, payload json.RawMessage) {
		cause := events.CauseProviderProgress
		if kind == types.EventToolInvoked || kind == types.EventToolResult {
			cause = events.CauseToolExecution
		}
		if kind == types.EventRunProgress {
			payload = payloadWithPhase(payload, phase)
		}
		if runHasProfile(rec, agentprofile.Texture) && kind == types.EventRunProgress {
			if docID := metadataStringValue(rec.Metadata, "doc_id"); docID != "" {
				progressPayload, _ := json.Marshal(map[string]string{
					"doc_id":  docID,
					"loop_id": rec.RunID,
					"phase":   phase,
				})
				rt.emitEvent(ctx, rec, types.EventTextureAgentRevisionProgress,
					events.CauseProviderProgress, progressPayload)
			}
		}
		rt.emitEvent(ctx, rec, kind, cause, payload)
	}
}

// payloadWithPhase adds "phase" to a JSON object payload that lacks one.
// Any other payload is returned unchanged.
func payloadWithPhase(payload json.RawMessage, phase string) json.RawMessage {
	if strings.TrimSpace(phase) == "" {
		return payload
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil || obj == nil {
		return payload
	}
	if _, ok := obj["phase"]; ok {
		return payload
	}
	obj["phase"], _ = json.Marshal(phase)
	out, err := json.Marshal(obj)
	if err != nil {
		return payload
	}
	return out
}

// executeWithToolLoop runs the run through the real tool-calling loop.
// This is the primary execution path when a tool registry is configured,
// enabling the LLM to invoke registered Go function-call tools.
func (rt *Runtime) executeWithToolLoop(ctx context.Context, rec *types.RunRecord, registry *toolregistry.ToolRegistry, emit provideriface.EventEmitFunc) {
	tlp := toolregistry.AsToolLoopProvider(rt.provider)
	if isManagementTaskRun(rec) {
		// K tasks call at once: every task model call, compaction
		// included, waits on the sub-RLM spacer research children share
		// (S2b, FM14).
		tlp = spacedProvider{ToolLoopProvider: tlp, spacer: rt.subRLMCallSpacer()}
	}

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
	assignedEngineeringOverlay := false
	if rt.capsuleExecutor != nil && !isManagementTaskRun(rec) {
		switch agentProfileForRun(rec) {
		case agentprofile.Management:
			ctx = WithCapsuleCtx(ctx, &CapsuleToolCtx{
				Executor: rt.capsuleExecutor, AgentRunID: rec.RunID, ComputerID: rt.selfdevComputerID, Role: capsule.RoleManagement,
				EventAppender: rt.eventAppender, TransactionBuilder: rt.capsuleBuilder,
			})
		case agentprofile.Engineering:
			overlay, handle, overlayErr := rt.assignedEngineeringToolOverlay(ctx, rec, registry)
			if overlayErr != nil {
				rt.handleExecutionError(ctx, rec, fmt.Errorf("bind assigned Engineering tool overlay: %w", overlayErr))
				return
			}
			if handle != "" {
				assignedEngineeringOverlay = true
				registry = overlay
				// Exact assigned Engineering receive the runtime-held capsule handle
				// for in-cell capsule effects. Host file, spawn, materialize,
				// checkpoint, route, VM, and owner decision authority are not
				// injected.
				ctx = WithCapsuleCtx(ctx, rt.assignedEngineeringCapsuleToolCtx(rec, handle))
			}
		}
	}
	reactivateExistingMemory := metadataBoolValue(rec.Metadata, "actor_reactivate_existing_memory")
	appendInitialMailboxTurns := shouldAppendInitialCoagentMailboxTurns(rec)
	if !reactivateExistingMemory && !appendInitialMailboxTurns {
		initialMessages, err = rt.prependInitialCoagentUpdatePackets(ctx, rec, initialMessages)
		if err != nil {
			rt.handleExecutionError(ctx, rec, fmt.Errorf("prepend coagent update packets: %w", err))
			return
		}
	}
	// The time and the run's own facts are its first turn, not part of the
	// system prompt, which stays the desk's stable cached prefix
	// (run_context.go).
	runFacts := rt.runContextForRun(rec)
	initialMessages = append([]json.RawMessage{runContextMessage(time.Now(), runFacts)}, initialMessages...)
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
		withLLMCompactor(tlp, llmConfig, estimateTextTokens(renderedSystemPrompt)).
		withRunContext(
			func() json.RawMessage { return runContextMessage(time.Now(), "") },
			func() json.RawMessage { return runContextMessage(time.Now(), runFacts) },
		)
	initialMailboxPhase := ""
	if appendInitialMailboxTurns {
		initialMailboxPhase = coagentPacketDeliveryThread
	}
	// A Texture run reads its frame before its first model call and again
	// whenever its document or records change (Cut 2 2a).
	frameTurns := rt.textureFrameInjector(rec)
	injectUserTurns := composeInjectors(frameTurns, rt.coagentUpdateTurnInjectorWithInitialPhase(rec, initialMailboxPhase))
	initialMessages, err = memory.initialize(ctx, initialMessages)
	if err != nil {
		rt.handleExecutionError(ctx, rec, fmt.Errorf("initialize run memory: %w", err))
		return
	}
	// The calibration digest (ICRL experiment) is a user turn after the
	// history, never the system prompt: the cached prefix holds.
	calibrationTurns := rt.calibrationDigestInjector(rec, initialMessages)
	injectUserTurns = composeInjectors(injectUserTurns, calibrationTurns)
	initialTurns := composeInjectors(frameTurns, calibrationTurns)
	if appendInitialMailboxTurns {
		initialTurns = injectUserTurns
	}
	if initialTurns != nil {
		injected, err := initialTurns(false)
		if err != nil {
			rt.handleExecutionError(ctx, rec, fmt.Errorf("inject initial mailbox turns for actor: %w", err))
			return
		}
		for _, msg := range injected {
			if err := memory.afterAppendMessage(ctx, types.RunMemoryRoleRuntimeInjection, msg); err != nil {
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

	toolLoopOptions := []toolregistry.ToolLoopOption{
		toolregistry.WithToolLoopMemoryHooks(memory.hooks()),
		toolregistry.WithToolLoopLLMConfig(llmConfig),
		toolregistry.WithToolLoopFallbackLLMConfigs(provideriface.FallbackLLMConfigsFromMetadata(rec.Metadata, llmConfig)),
		toolregistry.WithToolLoopConversationID(rec.RunID),
		toolregistry.WithToolLoopPromptCache(promptCacheScope(rec)),
	}
	if assignedEngineeringOverlay {
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithTerminalToolResult("capsule_go_eval", func(output string) bool {
			var decoded struct {
				FateTerminal bool `json:"fate_terminal"`
			}
			return json.Unmarshal([]byte(output), &decoded) == nil && decoded.FateTerminal
		}))
		// The desk's only tool is capsule_go_eval; its durable transition is a
		// committed terminal fate (freeze/verify for selfdev casts). An
		// end_turn without one used to close the run with prose and strand the
		// self-development operation in `executing` forever — the probe wedge
		// recorded in docs/problems/m11-engineering-desk-deferral-2026-09-27.md
		// (fourth wedge). The guard rejects narrative-only endings: retry with
		// a bounded reminder, then fail the run so assignment fate joins.
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithCompletionGuard(func(ctx context.Context, state toolregistry.ToolLoopCompletionState) (toolregistry.ToolLoopCompletionGuardResult, error) {
			if engineeringOverlayTerminalFateCommitted(state.Messages) {
				return toolregistry.ToolLoopCompletionGuardResult{}, nil
			}
			// A turn that ends while its capsule runs a job waits for the
			// job and resumes with its outcome (capsule_jobs.go).
			if waited, ok := rt.engineeringJobWait(ctx, rec); ok {
				return waited, nil
			}
			instruction := "The turn ended without a committed terminal fate. Finish the assignment inside a capsule_go_eval cell: call choir.Freeze when a self-development bundle is bound, then choir.Complete once with execution_refs — or commit the corresponding terminal failure fate — before ending the turn. Narrative text alone does not end the run."
			reason := engineeringTerminalFateGuardReason(state.Messages)
			if reason != engineeringTerminalFateRequired {
				instruction = "The freeze was refused because another self-development candidate is open. A partial report does not end the assignment: Complete with result failed and the refusal as the reason, in a capsule_go_eval cell, before ending the turn."
			}
			return toolregistry.ToolLoopCompletionGuardResult{
				Continue:    true,
				Reason:      reason,
				Instruction: instruction,
			}, nil
		}))
	}
	if waiter := rt.coagentParkWaiter(rec); waiter != nil {
		toolLoopOptions = append(toolLoopOptions, toolregistry.WithParkWaiter(waiter))
	}
	if isManagementTaskRun(rec) {
		// A task ends when its Done commits (R2-8), and no model call may
		// take its whole deadline (R2-9).
		toolLoopOptions = append(toolLoopOptions,
			toolregistry.WithTerminalToolResult("desk_go_eval", func(output string) bool {
				return strings.Contains(output, "rlm:"+yaegikernel.IntentTaskDone+":")
			}),
			toolregistry.WithToolLoopProviderCallTimeout(managementTaskCallTimeout))
	}
	if runHasProfile(rec, agentprofile.Texture) {
		// Texture's durable transition is a committed texture_turn cell intent
		// (choir.Write, Done, Wait, Reject, or a dispatch committed as a Wait),
		// committed by the bound authorizer inside ApplyTextureTurn.
		// desk_go_eval receipts carry it as rlm:texture_turn.
		// No initial-tool-choice or required-write choreography: the desk has
		// one tool, and a turn without a committed write is a completed no-op —
		// the next semantic event re-arms the reconcile loop. Forcing a write
		// tool only converted model slips into run-fatal retries.
		textureAuthored := func(output string) bool { return strings.Contains(output, "rlm:"+yaegikernel.IntentTextureTurn+":") }
		if strings.TrimSpace(rec.TrajectoryID) != "" && strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id")) != "" {
			// One committed authoring turn parks the resident run instead of
			// turning it terminal.
			toolLoopOptions = append(toolLoopOptions, toolregistry.WithPassivatingToolResult("desk_go_eval", textureAuthored))
		} else {
			// Pre-lifecycle Texture tasks retain their single-write terminal contract.
			toolLoopOptions = append(toolLoopOptions, toolregistry.WithTerminalToolResult("desk_go_eval", textureAuthored))
		}
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
	rec.Metadata["cached_input_tokens"] = usage.CachedInputTokens

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

	// Persist the terminal run state and the completion event in one atomic
	// batch for lifecycle-bound runs (the event folds into the projection's
	// PutBatchConditional). The bus publish happens inside
	// persistActivationStateAndEmit after the commit, so a subscriber that
	// reacts to the event and fetches run status observes the terminal state.
	resultLenPayload, _ := json.Marshal(map[string]any{
		"result_length":       len(text),
		"input_tokens":        usage.InputTokens,
		"output_tokens":       usage.OutputTokens,
		"cached_input_tokens": usage.CachedInputTokens,
	})
	persisted, err := rt.persistActivationStateAndEmit(persistCtx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload)
	if err != nil {
		log.Printf("runtime: update run %s to completed: %v", rec.RunID, err)
		rt.persistRunEndInBackground(*rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload, rt.afterRunCompleted)
		return
	}
	if !persisted {
		return
	}
	rt.afterRunCompleted(persistCtx, rec)
}

// afterRunCompleted runs a completed run's follow-ups once its end is
// stored: the outcome binding and the desk's inbox recheck.
func (rt *Runtime) afterRunCompleted(persistCtx context.Context, rec *types.RunRecord) {
	if bindErr := rt.bindTerminalRunOutcome(persistCtx, rec, true); bindErr != nil {
		log.Printf("runtime: bind completion outcome for run %s: %v", rec.RunID, bindErr)
	}
	if shouldLogWireLifecycle(rec) {
		preview := rec.Result
		if len(preview) > 160 {
			preview = preview[:160]
		}
		log.Printf("runtime: completed %s result=%q", wireLifecycleSummary(rec), strings.ReplaceAll(preview, "\n", " "))
	}
	rt.maybeContinuePersistentManagementInbox(persistCtx, rec)
	rt.recheckRecordsInbox(persistCtx, rec)
}

const runtimeInjectionAppendFailurePassivationReason = "runtime_injection_append_failed"

func retryableLifecycleRuntimeInjectionFailure(rec *types.RunRecord, err error) bool {
	if rec == nil || !errors.Is(err, ErrRuntimeInjectionAppendFailed) {
		return false
	}
	profile, _ := agentprofile.Canonical(rec.AgentProfile)
	role, _ := agentprofile.Canonical(rec.AgentRole)
	if profile != role || (profile != agentprofile.Research && profile != agentprofile.Texture && profile != agentprofile.Management) {
		return false
	}
	if profile == agentprofile.Management && rec.AgentID != persistentManagementAgentID(rec.OwnerID) {
		return false
	}
	return metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" &&
		strings.TrimSpace(rec.OwnerID) != "" && strings.TrimSpace(rec.ComputerID) != "" &&
		lifecycleControlTrajectoryForRun(rec) != "" && strings.TrimSpace(rec.AgentID) != "" &&
		strings.TrimSpace(rec.RunID) != ""
}

// passivateRuntimeInjectionAppendFailure preserves the exact lifecycle run and
// its delivery bindings when authenticated memory append fails. The control is
// still pending and was never durably model-visible, so terminalizing this run
// would strand already-delivered authority and invite a replacement run.
func (rt *Runtime) passivateRuntimeInjectionAppendFailure(rec *types.RunRecord, appendErr error) {
	if rt == nil || rt.store == nil || rec == nil {
		return
	}
	now := time.Now().UTC()
	rec.State = types.RunPassivated
	rec.Error = ""
	rec.FinishedAt = nil
	rec.UpdatedAt = now
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata["passivated_reason"] = runtimeInjectionAppendFailurePassivationReason
	rec.Metadata["actor_sleep_state"] = "retryable_runtime_failure"
	rec.Metadata["runtime_injection_append_error"] = appendErr.Error()
	payload, _ := json.Marshal(map[string]string{
		"reason": runtimeInjectionAppendFailurePassivationReason,
		"error":  appendErr.Error(),
	})
	persisted, err := rt.persistActivationStateAndEmit(context.Background(), rec, types.EventRunPassivated, events.CauseSupervisorRecovery, payload)
	if err != nil {
		// Never fall through to terminal failure. If durable passivation itself is
		// unavailable, boot recovery still sees the stored pending/running exact
		// lifecycle run and re-dispatches it without changing its identity.
		log.Printf("runtime: preserve retryable lifecycle injection failure for run %s: %v", rec.RunID, err)
		return
	}
	if !persisted {
		return
	}
	log.Printf("runtime: passivated exact lifecycle run %s after runtime injection append failure", rec.RunID)
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
	if runHasProfile(rec, agentprofile.Texture) {
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
	rec.Metadata["cached_input_tokens"] = usage.CachedInputTokens
	rec.Metadata["passivated_reason"] = reason
	rec.Metadata["actor_sleep_state"] = "idle"
	rec.Metadata["actor_sleep_at"] = now.Format(time.RFC3339Nano)

	payload := map[string]any{
		"reason":              reason,
		"result_length":       len(text),
		"input_tokens":        usage.InputTokens,
		"output_tokens":       usage.OutputTokens,
		"cached_input_tokens": usage.CachedInputTokens,
	}
	if runHasProfile(rec, agentprofile.Texture) {
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
	// Fold the passivated event into the same batch as the state write for
	// lifecycle-bound runs; the bus publish happens post-commit inside.
	persisted, err := rt.persistActivationStateAndEmit(ctx, rec, types.EventRunPassivated, events.CauseTaskLifecycle, payloadJSON)
	if err != nil {
		log.Printf("runtime: passivate idle run %s: %v", rec.RunID, err)
		return
	}
	if !persisted {
		return
	}
	if shouldLogWireLifecycle(rec) {
		log.Printf("runtime: passivated idle %s reason=%s", wireLifecycleSummary(rec), reason)
	}
	rt.maybeContinuePersistentManagementInbox(context.Background(), rec)
	rt.recheckRecordsInbox(context.Background(), rec)
}

func (rt *Runtime) sleepTextureMutationAfterIdle(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	mutation, err := rt.store.GetAgentMutationByRun(ctx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
	if err != nil {
		return fmt.Errorf("get texture mutation for idle passivation: %w", err)
	}
	if mutation == nil {
		return nil
	}
	switch mutation.State {
	case "pending":
		// Consume-at-commit: a no-write idle passivation is a completed no-op,
		// not a failed mutation. A committed non-revision turn sleeps the
		// mutation; nothing committed means the activation died mid-flight and
		// the row goes stale for the next reconcile to replace.
		if turn, err := rt.store.GetAppliedTextureTurnByCallerRun(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID, rec.RunID); err == nil && turn.TextureTurn != nil {
			if turn.TextureTurn.Outcome == types.TextureTurnRevision && turn.Revision != nil {
				if err := rt.store.CompleteAgentMutation(ctx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID, turn.Revision.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
					return err
				}
				return nil
			}
			if err := rt.store.SleepAgentMutationAfterTextureTurn(ctx, rec.OwnerID, agentMutationComputerID(rec), rec.TrajectoryID, rec.RunID); err != nil && err != store.ErrMutationAlreadyCompleted {
				return err
			}
			return nil
		}
		// Legacy-protocol recovery (mirrors handleRunCompletion): a pending
		// mutation carrying RevisionID is a committed write that died before
		// the applied-turn row existed. Completing it is the recovery the
		// postcondition waits on.
		if strings.TrimSpace(mutation.RevisionID) != "" {
			if err := rt.store.CompleteAgentMutation(ctx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID, mutation.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
				return err
			}
			return nil
		}
		_ = rt.store.MarkAgentMutationStale(ctx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
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
	if bindErr := rt.bindTerminalRunOutcome(persistCtx, rec, true); bindErr != nil {
		log.Printf("runtime: bind completion outcome for run %s: %v", rec.RunID, bindErr)
	}
	resultLenPayload, _ := json.Marshal(map[string]int{"result_length": len(result)})
	rt.emitEvent(persistCtx, rec, types.EventRunCompleted, events.CauseTaskLifecycle, resultLenPayload)
	rt.maybeContinuePersistentManagementInbox(persistCtx, rec)
	rt.recheckRecordsInbox(persistCtx, rec)

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
				if !contentowner.IsAllowedProductApp(strings.TrimSpace(parsed.App)) {
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

const engineeringTerminalFateRequired = "engineering_overlay_terminal_fate_required"

// freezeRefusedByOpenCandidate matches the refusal openFreezeCandidate
// returns when another self-development operation is open, and captures that
// operation's id and state.
var freezeRefusedByOpenCandidate = regexp.MustCompile(`cannot freeze until the open candidate settles \(` +
	regexp.QuoteMeta(selfdev.ErrOperationOpen.Error()) + `: (\S+) is (\w+)\)`)

// engineeringTerminalFateGuardReason is the completion guard's reason, which
// becomes the system cancel's reason when the guard gives up. When the run's
// freeze was refused by an open candidate it names that candidate, so the
// assignment's fate says why it could not finish (FP4 round 3, P2). It
// reports the candidate and the state the refusal gave, and nothing about
// what that state waits for. It keeps the run's last refusal, even if a
// later freeze succeeded (residual: a successful freeze leaves no mark in
// the cell output this reads).
func engineeringTerminalFateGuardReason(messages []json.RawMessage) string {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	var blocks []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	var refused []string
	for _, raw := range messages {
		msg.Content = nil
		if err := json.Unmarshal(raw, &msg); err != nil || len(msg.Content) == 0 {
			continue
		}
		blocks = blocks[:0]
		if err := json.Unmarshal(msg.Content, &blocks); err != nil {
			continue
		}
		for _, block := range blocks {
			if block.Type != "tool_result" {
				continue
			}
			if m := freezeRefusedByOpenCandidate.FindStringSubmatch(block.Content); m != nil {
				refused = m
			}
		}
	}
	if refused == nil {
		return engineeringTerminalFateRequired
	}
	return fmt.Sprintf("%s; the freeze was refused because self-development candidate %s was open (state %s)",
		engineeringTerminalFateRequired, refused[1], refused[2])
}

// engineeringOverlayTerminalFateCommitted reports whether any persisted
// tool_result block in the run's message log carries a capsule_go_eval output
// with fate_terminal=true — the same predicate the terminal-tool check applies
// per batch, extended across the whole cell history for the completion guard.
// Tool results persist as user messages whose content is a list of
// {type:"tool_result", content:<eval output JSON string>} blocks.
func engineeringOverlayTerminalFateCommitted(messages []json.RawMessage) bool {
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	var blocks []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	var eval struct {
		FateTerminal bool `json:"fate_terminal"`
	}
	for _, raw := range messages {
		msg.Content = nil
		if err := json.Unmarshal(raw, &msg); err != nil || len(msg.Content) == 0 {
			continue
		}
		blocks = blocks[:0]
		if err := json.Unmarshal(msg.Content, &blocks); err != nil {
			continue
		}
		for _, block := range blocks {
			if block.Type != "tool_result" || block.Content == "" {
				continue
			}
			eval.FateTerminal = false
			if err := json.Unmarshal([]byte(block.Content), &eval); err != nil {
				continue
			}
			if eval.FateTerminal {
				return true
			}
		}
	}
	return false
}

// handleRunCompletion processes feature-specific side effects after a run
// completes successfully. Texture document writes are intentionally not handled
// here: canonical appagent revisions are created only by Texture write tools.
func (rt *Runtime) handleRunCompletion(ctx context.Context, rec *types.RunRecord) error {
	if agentProfileForRun(rec) == agentprofile.Conductor {
		return nil
	}

	if !runHasProfile(rec, agentprofile.Texture) {
		return nil
	}

	persistCtx := context.Background()

	docID, _ := rec.Metadata["doc_id"].(string)
	if docID == "" {
		log.Printf("runtime: texture agent revision run %s: missing doc_id in metadata", rec.RunID)
		return nil
	}

	mutation, err := rt.store.GetAgentMutationByRun(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
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

	if strings.TrimSpace(rec.TrajectoryID) != "" && strings.TrimSpace(rec.ComputerID) != "" {
		if turn, turnErr := rt.store.GetAppliedTextureTurnByCallerRun(persistCtx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID, rec.RunID); turnErr == nil && turn.TextureTurn != nil {
			if turn.TextureTurn.Outcome == types.TextureTurnRevision && turn.Revision != nil {
				_ = rt.store.CompleteAgentMutation(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID, turn.Revision.RevisionID)
			} else {
				_ = rt.store.SleepAgentMutationAfterTextureTurn(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.TrajectoryID, rec.RunID)
			}
			return nil
		} else if turnErr != nil && !errors.Is(turnErr, store.ErrNotFound) {
			log.Printf("runtime: recover committed Texture turn for run %s: %v", rec.RunID, turnErr)
		}
	}
	// Legacy-protocol recovery: a pending mutation carrying RevisionID is a
	// committed write that died before the applied-turn row existed (or a
	// pre-restart in-flight write). Completing it is the recovery the
	// postcondition waits on; without this leg the occurrence defers forever.
	if strings.TrimSpace(mutation.RevisionID) != "" {
		if err := rt.store.CompleteAgentMutation(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID, mutation.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
			log.Printf("runtime: texture agent revision run %s: complete written mutation: %v", rec.RunID, err)
		}
		return nil
	}
	// No committed turn: the activation ended without a durable write — a
	// completed no-op. The mutation row goes stale so reconcile's stranded
	// repair reclaims its covered packets; it is never a failure.
	_ = rt.store.MarkAgentMutationStale(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
	return nil
}

// recordExplicitInitialTextureDecisionIfNeeded persists the deterministic
// no_worker_needed decision the prompt-bar flow stamps on a Texture run's
// metadata. The decision is the audit row explaining why the desk answered
// inline without delegating; a run completing without a Texture write must
// still carry this receipt (receipt point 5, respawn-loop cluster).
func (rt *Runtime) recordExplicitInitialTextureDecisionIfNeeded(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	if !runHasProfile(rec, agentprofile.Texture) ||
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
	}
	return nil
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
	// A stored terminal state always wins: a late execution error must not
	// re-terminalize a run that already carries its fate (for example an
	// owner-cancelled assignment whose tool loop then observes context
	// cancellation). Mirrors persistActivationState.
	if stored, loadErr := rt.getRunForComputer(context.Background(), rec.OwnerID, rec.RunID); loadErr == nil && stored.State.Terminal() {
		*rec = stored
		return
	}
	if retryableLifecycleRuntimeInjectionFailure(rec, err) {
		rt.passivateRuntimeInjectionAppendFailure(rec, err)
		return
	}
	if assignedEngineeringRun(rec) {
		termErr := rt.terminalizeRun(context.Background(), rec.RunID, rec.OwnerID, err.Error())
		if termErr == nil {
			if stored, loadErr := rt.getRunForComputer(context.Background(), rec.OwnerID, rec.RunID); loadErr == nil {
				*rec = stored
			}
			return
		}
		log.Printf("runtime: assigned Engineering execution error could not join assignment fate for run %s: %v (%v)", rec.RunID, err, termErr)
		return
	}
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
	errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
	persisted, updateErr := rt.persistActivationStateAndEmit(persistCtx, rec, kind, cause, errPayload)
	if updateErr != nil {
		log.Printf("runtime: update run %s to %s: %v", rec.RunID, state, updateErr)
		rt.persistRunEndInBackground(*rec, kind, cause, errPayload, func(ctx context.Context, stored *types.RunRecord) {
			rt.afterRunFailed(ctx, stored, state, err)
		})
		return
	}
	if !persisted {
		return
	}
	rt.afterRunFailed(persistCtx, rec, state, err)
}

// afterRunFailed runs a failed, cancelled or blocked run's follow-ups once
// its state is stored: the outcome binding, the Texture mutation
// settlement, and the desk's inbox recheck.
func (rt *Runtime) afterRunFailed(persistCtx context.Context, rec *types.RunRecord, state types.RunState, err error) {
	if bindErr := rt.bindTerminalRunOutcome(persistCtx, rec, true); bindErr != nil {
		log.Printf("runtime: bind terminal outcome for run %s: %v", rec.RunID, bindErr)
	}

	// If this is a Texture agent revision task, settle the mutation before any
	// reconcile pass. A no-write failure must not immediately requeue the same
	// undelivered packet forever; a failure after a successful write should still
	// close the mutation on the latest stored revision.
	if runHasProfile(rec, agentprofile.Texture) {
		if mutation, mutationErr := rt.store.GetAgentMutationByRun(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID); mutationErr != nil {
			log.Printf("runtime: texture agent revision run %s: get mutation after failure: %v", rec.RunID, mutationErr)
		} else if mutation != nil {
			if strings.TrimSpace(mutation.RevisionID) != "" {
				if completeErr := rt.store.CompleteAgentMutation(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID, mutation.RevisionID); completeErr != nil && completeErr != store.ErrMutationAlreadyCompleted {
					log.Printf("runtime: texture agent revision run %s: complete written mutation after failure: %v", rec.RunID, completeErr)
				}
			} else {
				_ = rt.store.FailAgentMutation(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
			}
		} else {
			_ = rt.store.FailAgentMutation(persistCtx, rec.OwnerID, agentMutationComputerID(rec), rec.RunID)
		}
		if docID := metadataStringValue(rec.Metadata, "doc_id"); docID != "" {
			failPayload, _ := json.Marshal(map[string]string{
				"doc_id":  docID,
				"loop_id": rec.RunID,
				"error":   err.Error(),
			})
			rt.emitEvent(persistCtx, rec, types.EventTextureAgentRevisionFailed,
				events.CauseProviderFailure, failPayload)
		}
	}

	log.Printf("runtime: run %s → %s: %v", rec.RunID, state, err)
	if state.Terminal() {
		rt.maybeContinuePersistentManagementInbox(persistCtx, rec)
		rt.recheckRecordsInbox(persistCtx, rec)
	}
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
	return nil
}

func (rt *Runtime) latestActiveRunByAgent(ctx context.Context, ownerID, agentID string) (types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return types.RunRecord{}, store.ErrNotFound
	}
	if computerID := strings.TrimSpace(rt.cfg.ComputerID); computerID != "" {
		rec, err := rt.store.GetLatestActiveLifecycleRunByAgent(ctx, ownerID, computerID, agentID)
		if err == nil {
			return rec, nil
		}
		if !errors.Is(err, store.ErrNotFound) {
			return types.RunRecord{}, err
		}
	}
	return rt.store.GetLatestActiveRunByAgent(ctx, ownerID, agentID)
}

// runFateNoLiveLoop ends a run stored as running whose loop is gone.
const runFateNoLiveLoop = "interrupted: stored as running with no live loop in this process"

// deadRunGrace is how long a run may be stored as running before its loop
// registers. Setting a run running and registering its loop happen in one
// activation, well inside this; a dead run's state has not changed for
// longer.
const deadRunGrace = 2 * time.Minute

// runHasNoLiveLoop reports a run stored as running whose loop this process
// does not have, unchanged for longer than deadRunGrace. A pending run is
// never dead here: its dispatch may be in flight, and the fresh-mint
// watchdog owns a lost one.
func (rt *Runtime) runHasNoLiveLoop(rec types.RunRecord, now time.Time) bool {
	if rt == nil || rec.State != types.RunRunning || rec.UpdatedAt.IsZero() || now.Sub(rec.UpdatedAt) < deadRunGrace {
		return false
	}
	rt.runningMu.Lock()
	_, live := rt.running[rec.RunID]
	rt.runningMu.Unlock()
	return !live
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
	rec, err := rt.latestActiveRunByAgent(ctx, ownerID, agentID)
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
