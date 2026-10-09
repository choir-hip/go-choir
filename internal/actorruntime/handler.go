package actorruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// resumeState is the compact resume pointer encoded into the actor's memory
// snapshot. The store holds the full conversation history, tool results, and
// run metadata; memory holds only enough to know where to resume.
type resumeState struct {
	RunID string `json:"run_id,omitempty"`
	Phase string `json:"phase,omitempty"` // "parked" | "" (completed/cleared)
}

// actorHandler implements actor.Handler. It is the execution boundary: the
// actor goroutine IS the run goroutine. HandleUpdate calls
// runtime.ExecuteActivationSync synchronously — no startRunAsync, no separate
// goroutine.
//
// Park-resume: when the tool loop parks (waiting for coagent updates),
// executeActivation returns with rec.State == RunPassivated. The handler
// encodes a resume pointer into memory and returns. The actor passivates.
// When a new coagent update arrives (via actor.Send), the actor re-activates,
// the handler decodes memory, loads the passivated run from the store, sets
// actor_reactivate_existing_memory=true, and calls ExecuteActivationSync again.
// The tool loop loads the persisted conversation, injects the new update via
// injectUserTurns, and resumes from the park point.
type actorHandler struct {
	rt           *agentcore.Runtime
	textureOwner *textureowner.Handler
	// bootAt is when this process built its handler. A work occurrence
	// recorded earlier belongs to work the restart interrupted.
	bootAt time.Time
	// plannedBoot is true when this boot consumed a planned-restart marker
	// (an update apply restarted the guest on purpose): pre-boot work resumes.
	plannedBoot bool
}

// newActorHandler creates the handler. The rt must have its store, provider,
// and tool registry configured before runs are dispatched.
func newActorHandler(rt *agentcore.Runtime, textureOwner *textureowner.Handler) *actorHandler {
	h := &actorHandler{rt: rt, textureOwner: textureOwner, bootAt: time.Now().UTC()}
	if marker, planned := rt.BootWasPlannedRestart(); planned {
		h.plannedBoot = true
		log.Printf("actorruntime: boot follows a planned restart (%s %s); pre-boot work resumes", marker.Reason, marker.Target)
	} else {
		log.Printf("actorruntime: boot follows a crash or stop; pre-boot work is interrupted_by_restart")
	}
	return h
}

// errInterruptedByRestart is the durable fate of a work occurrence recorded
// before a crash boot: a crash restart never resumes work, for any desk
// (owner rule, AGENTS.md "Restarts End Work (Crash) Or Resume It"). The
// lifecycle keeps pending reports and owner revisions for the owner's next
// action.
var errInterruptedByRestart = fmt.Errorf("%w: interrupted_by_restart: work occurrence recorded before this boot", actor.ErrDurableInvalid)

// restartWorkKinds start or resume work. Cancels and fail-closed deadlines
// are absent: they close work and still apply across a restart, as does
// selfdev_materialization_retry, which finishes a planned apply.
var restartWorkKinds = map[string]bool{
	"initial_dispatch": true, "coagent_result": true, "channel_message": true,
	"owner_revision": true, "lifecycle_work_assigned": true,
	"delegated_assignment_spawn_deadline":   true,
	"fresh_mint_management_resume_deadline": true,
}

func (h *actorHandler) preBootWorkOccurrence(mailboxID string, u actor.Update) bool {
	if h.plannedBoot || h.bootAt.IsZero() || u.CreatedAt.IsZero() || !u.CreatedAt.Before(h.bootAt) || !restartWorkKinds[u.Kind] {
		return false
	}
	if strings.TrimSpace(u.ToAgentID) != "" {
		mailboxID = u.ToAgentID
	}
	_, _, _, err := parseScopedActorMailboxID(mailboxID)
	return err == nil
}

// logTextureOccurrenceConsumed records why a Texture occurrence was consumed
// without executing a turn. These paths incorporate the occurrence, so
// without this line its fate is invisible
// (texture-create-occurrence-deferred-never-refires-2026-10-09, O1).
func logTextureOccurrenceConsumed(agentID string, u actor.Update, reason string) {
	log.Printf("actorruntime: Texture occurrence %s/%s (%s) consumed without a turn: %s", agentID, u.UpdateID, u.Kind, reason)
}

func deferTextureOccurrence(err error) error {
	if err == nil {
		return actor.ErrDeferUnprocessed
	}
	return fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, err)
}

func textureRunRecord(rec types.RunRecord) bool {
	recordProfile, _ := agentprofile.Canonical(rec.AgentProfile)
	recordRole, _ := agentprofile.Canonical(rec.AgentRole)
	if recordProfile == agentprofile.Texture ||
		recordRole == agentprofile.Texture {
		return true
	}
	if rec.Metadata == nil {
		return false
	}
	for _, key := range []string{"agent_profile", "agent_role"} {
		if value, ok := rec.Metadata[key].(string); ok {
			metaProfile, _ := agentprofile.Canonical(value)
			if metaProfile == agentprofile.Texture {
				return true
			}
		}
	}
	return false
}

// HandleUpdate is the execution boundary. One call per incoming update.
// A single run may span many HandleUpdate calls (initial_dispatch → park →
// coagent_result → park → ... → completion).
func (h *actorHandler) HandleUpdate(ctx context.Context, agentID string, u actor.Update, memory []byte) ([]byte, error) {
	if h.preBootWorkOccurrence(agentID, u) {
		log.Printf("actorruntime: %s %s for %s recorded %s, before crash boot %s: interrupted_by_restart, not run",
			u.Kind, u.UpdateID, agentID, u.CreatedAt.Format(time.RFC3339), h.bootAt.Format(time.RFC3339))
		return nil, errInterruptedByRestart
	}
	switch u.Kind {
	case "initial_dispatch":
		return h.handleInitialDispatch(ctx, u, memory)
	case "coagent_result":
		return h.handleCoagentResult(ctx, u, memory)
	case "channel_message":
		return h.handleChannelMessage(ctx, u, memory)
	case "activation_budget_deadline":
		return h.handleActivationBudgetDeadline(ctx, u, memory)
	case "cell_terminal_deadline":
		return h.handleCellTerminalDeadline(ctx, u, memory)
	case "assigned_engineering_fate_deadline":
		return h.handleAssignedEngineeringFateDeadline(ctx, u, memory)
	case "delegated_assignment_spawn_deadline":
		return h.handleDelegatedAssignmentSpawnDeadline(ctx, u, memory)
	case "engineering_progress_overdue_deadline":
		return h.handleEngineeringProgressDeadline(ctx, u, memory)
	case "fresh_mint_management_resume_deadline":
		return h.handleFreshMintManagementResumeDeadline(ctx, u, memory)
	case "reactivated_management_resume_deadline":
		return h.handleReactivatedManagementResumeDeadline(ctx, u, memory)
	case "wire_reconciler_publish_deadline":
		// Frozen historical continuation: consume as a no-op after the
		// reconciler dispatch surface was retired.
		return memory, nil
	case "selfdev_materialization_retry":
		return h.handleSelfDevelopmentMaterializationRetry(ctx, u, memory)
	case "lifecycle_work_assigned":
		return h.handleLifecycleWorkAssigned(ctx, u, memory)
	case "lifecycle_cancellation":
		return h.handleLifecycleCancellation(ctx, u, memory)
	case "owner_revision":
		return h.handleOwnerRevision(ctx, u, memory)
	case "cancel":
		return h.handleCancel(ctx, u, memory)
	default:
		log.Printf("actorruntime: handler: unknown update kind %q for agent %s", u.Kind, agentID)
		return memory, nil // leave memory unchanged; update marked processed
	}
}

func (h *actorHandler) handleActivationBudgetDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve activation budget deadline scope: %w", err)
	}
	if err := h.rt.HandleActivationBudgetDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: activation budget deadline: %w", err)
	}
	return memory, nil
}

func (h *actorHandler) handleCellTerminalDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve cell terminal deadline scope: %w", err)
	}
	if err := h.rt.HandleCellTerminalDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: cell terminal deadline: %w", err)
	}
	return memory, nil
}

// handleSelfDevelopmentMaterializationRetry re-arms the self-development
// drain on a durable wake. The drain's own ListByStates is the sole state
// gate — replayed or superseded wakes are no-ops.
func (h *actorHandler) handleSelfDevelopmentMaterializationRetry(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	h.rt.TriggerSelfDevelopmentReconcile()
	return memory, nil
}

func (h *actorHandler) handleAssignedEngineeringFateDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve assigned Engineering fate deadline scope: %w", err)
	}
	if err := h.rt.HandleAssignedEngineeringFateDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: assigned Engineering fate deadline: %w", err)
	}
	return memory, nil
}

// handleDelegatedAssignmentSpawnDeadline resolves the scoped caster mailbox and
// re-drives a delegated cast's spawn/bind saga after its cell commit. The saga
// inputs ride on the wake; the saga itself re-derives the committed digests.
func (h *actorHandler) handleDelegatedAssignmentSpawnDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve delegated assignment spawn deadline scope: %w", err)
	}
	if err := h.rt.HandleDelegatedAssignmentSpawnDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: delegated assignment spawn deadline: %w", err)
	}
	return memory, nil
}

// handleEngineeringProgressDeadline resolves the scoped supervisor mailbox and
// evaluates the derivable progress-overdue wake. The handler re-reads durable
// state, so replayed or superseded wakes are no-ops.
func (h *actorHandler) handleEngineeringProgressDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve engineering progress deadline scope: %w", err)
	}
	if err := h.rt.HandleEngineeringProgressOverdueDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: engineering progress deadline: %w", err)
	}
	return memory, nil
}

func (h *actorHandler) handleFreshMintManagementResumeDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve fresh-mint Management deadline scope: %w", err)
	}
	if err := h.rt.HandleFreshMintManagementResumeDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: fresh-mint Management deadline: %w", err)
	}
	return memory, nil
}

func (h *actorHandler) handleReactivatedManagementResumeDeadline(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve reactivated Management deadline scope: %w", err)
	}
	if err := h.rt.HandleReactivatedManagementResumeDeadline(ctx, ownerID, computerID, agentID, u.Content); err != nil {
		return nil, fmt.Errorf("actorruntime: reactivated Management deadline: %w", err)
	}
	return memory, nil
}

func (h *actorHandler) handleLifecycleWorkAssigned(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve lifecycle work assignment scope: %w", err)
	}
	var content struct {
		WorkItemID   string `json:"work_item_id"`
		TrajectoryID string `json:"trajectory_id"`
	}
	if err := json.Unmarshal([]byte(u.Content), &content); err != nil {
		return nil, nil
	}
	if strings.TrimSpace(content.TrajectoryID) != strings.TrimSpace(u.TrajectoryID) || strings.TrimSpace(content.WorkItemID) == "" {
		return nil, nil
	}
	if err := h.rt.ReconcileLifecycleWorkAssignment(ctx, ownerID, computerID, agentID, content.TrajectoryID, content.WorkItemID); err != nil {
		return nil, fmt.Errorf("%w: actorruntime: reconcile lifecycle work assignment: %v", actor.ErrDeferUnprocessed, err)
	}
	return memory, nil
}

func (h *actorHandler) handleLifecycleCancellation(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve lifecycle cancellation scope: %w", err)
	}
	if strings.TrimSpace(u.Content) != strings.TrimSpace(u.TrajectoryID) || strings.TrimSpace(u.TrajectoryID) == "" {
		return nil, nil
	}
	if err := h.rt.HandleLifecycleCancellationWake(ctx, ownerID, computerID, agentID, u.TrajectoryID); err != nil {
		return nil, fmt.Errorf("%w: actorruntime: reconcile lifecycle cancellation: %v", actor.ErrDeferUnprocessed, err)
	}
	return memory, nil
}

func (h *actorHandler) handleOwnerRevision(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve owner revision scope: %w", err)
	}
	var content struct {
		RevisionID       string `json:"revision_id"`
		RequestID        string `json:"request_id"`
		LifecycleVersion int64  `json:"lifecycle_version"`
		ReducerSeq       int64  `json:"reducer_seq"`
	}
	if err := json.Unmarshal([]byte(u.Content), &content); err != nil || strings.TrimSpace(content.RevisionID) == "" {
		return nil, nil
	}
	revision, err := h.rt.Store().GetLifecycleRevision(ctx, ownerID, computerID, content.RevisionID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: actorruntime: load owner revision: %v", actor.ErrDeferUnprocessed, err)
	}
	profile := agentprofile.Texture
	if strings.HasPrefix(agentID, agentprofile.Engineering+":") {
		profile = agentprofile.Engineering
	}
	occurrence, err := agentcore.DocumentRevisionOccurrence(revision, profile, content.RequestID, content.ReducerSeq)
	if err != nil || occurrence.TargetAgentID != agentID || occurrence.TrajectoryID != u.TrajectoryID {
		return nil, nil
	}
	encoded, err := agentcore.EncodeTextureActorOccurrence(occurrence)
	if err != nil {
		return nil, fmt.Errorf("%w: actorruntime: encode owner revision occurrence: %v", actor.ErrDeferUnprocessed, err)
	}
	u.Kind, u.Content = "coagent_result", encoded
	return h.handleCoagentResult(ctx, u, memory)
}

// handleChannelMessage resumes a parked recipient against the durable channel
// log. The envelope is already persisted; Inbox() on the next cell observes it.
// Unlike coagent_result, this kind never mints a new run: spawn/assign create
// runs, channel mail only wakes an existing activation.
func (h *actorHandler) handleChannelMessage(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	_, _, agentID, err := parseScopedActorMailboxID(u.ToAgentID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: resolve channel_message scope: %w", err)
	}
	if strings.HasPrefix(agentID, agentprofile.Texture+":") {
		return memory, nil
	}
	rs, err := decodeResumeState(memory)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: decode resume state for channel_message: %w", err)
	}
	if rs.RunID == "" {
		return memory, nil
	}
	rec, err := h.scopedRunForUpdate(ctx, u, rs.RunID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: load parked run %s for channel_message: %w", rs.RunID, err)
	}
	if textureRunRecord(rec) {
		return memory, nil
	}
	if rec.State == types.RunPassivated || rec.State.Active() {
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]any)
		}
		rec.Metadata["actor_reactivate_existing_memory"] = true
		rec.Metadata["actor_reactivated_from_passivated"] = true
		rec.Metadata["request_source"] = "channel_message"
		rec.State = types.RunPending
		rec.Error = ""
		rec.Result = ""
		rec.FinishedAt = nil
		rec.UpdatedAt = time.Now().UTC()
		// Lifecycle-bound runs reactivate through the canonical reducer
		// command so the state change and run_reactivated event commit
		// atomically; non-lifecycle runs keep the bare UpdateRun path.
		if err := h.rt.ReactivateRunCanonical(ctx, &rec, types.RunPending, map[string]any{
			"actor_reactivate_existing_memory":  true,
			"actor_reactivated_from_passivated": true,
			"request_source":                    "channel_message",
		}); err != nil {
			return nil, fmt.Errorf("actorruntime: reactivate run %s from channel_message: %w", rs.RunID, err)
		}
		if err := h.rt.ExecuteActivationSyncChecked(ctx, &rec); err != nil {
			if errors.Is(err, agentcore.ErrActivationOccurrenceMustRemainUnprocessed) {
				return nil, fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, err)
			}
			return nil, fmt.Errorf("actorruntime: execute channel_message resumed activation: %w", err)
		}
		return h.memoryFromRunState(&rec)
	}
	return memory, nil
}

// handleInitialDispatch starts a new run. memory should be nil (fresh start).
// The run ID is in u.Content.
func (h *actorHandler) handleInitialDispatch(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	runID := strings.TrimSpace(u.Content)
	if runID == "" {
		return nil, fmt.Errorf("actorruntime: initial_dispatch update has empty content (run ID)")
	}
	rec, err := h.scopedRunForUpdate(ctx, u, runID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: load run %s for initial dispatch: %w", runID, err)
	}
	if rec.State == types.RunPassivated &&
		strings.TrimSpace(metadataString(rec.Metadata, "passivated_reason")) == "runtime_restarted" {
		// The initial_dispatch occurrence is durable and may have been queued
		// before boot passivated its run. It is the sole resume authority, not
		// an already-consumed start: reactivate it before executing.
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]any)
		}
		rec.Metadata["actor_reactivated_from_passivated"] = true
		rec.State = types.RunRunning
		rec.Error = ""
		rec.Result = ""
		rec.FinishedAt = nil
		rec.UpdatedAt = time.Now().UTC()
		if err := h.rt.ReactivateRunCanonical(ctx, &rec, types.RunRunning, map[string]any{
			"actor_reactivated_from_passivated": true,
		}); err != nil {
			return nil, fmt.Errorf("actorruntime: reactivate restart-passivated run %s for initial dispatch: %w", runID, err)
		}
	}
	if rec.State != types.RunPending && rec.State != types.RunRunning {
		// Terminal/passivated runs were already handled by an earlier dispatch.
		return nil, nil
	}
	ownerID, computerID, agentID, scopeErr := parseScopedActorMailboxID(u.ToAgentID)
	if scopeErr != nil {
		return nil, fmt.Errorf("actorruntime: resolve initial_dispatch scope: %w", scopeErr)
	}
	if strings.HasPrefix(agentID, agentprofile.Texture+":") || textureRunRecord(rec) {
		if h.textureOwner == nil {
			return nil, fmt.Errorf("actorruntime: Texture owner is not bound")
		}
		if strings.TrimSpace(rec.AgentID) != agentID {
			return nil, fmt.Errorf("actorruntime: Texture initial_dispatch agent mismatch")
		}
		if err := h.textureOwner.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, runID); err != nil {
			return nil, fmt.Errorf("actorruntime: validate Texture initial_dispatch: %w", err)
		}
	}
	if err := h.rt.ExecuteActivationSyncChecked(ctx, &rec); err != nil {
		return nil, fmt.Errorf("%w: actorruntime: execute initial activation: %v", actor.ErrDeferUnprocessed, err)
	}
	return h.memoryFromRunState(&rec)
}

// handleCoagentResult resumes a parked run. memory carries the resume pointer
// (run ID + phase). The coagent update is already in the store mailbox; the
// tool loop's injectUserTurns will pick it up on re-entry.
//
// If there is no parked run (memory is nil or has no run ID), the handler
// calls ReconcileCoagentWake to create a new run for the coagent update —
// this handles cold starts (process restart) and first-ever updates.
func (h *actorHandler) handleCoagentResult(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	ownerID, computerID, agentID, scopeErr := parseScopedActorMailboxID(u.ToAgentID)
	if scopeErr != nil {
		return nil, fmt.Errorf("actorruntime: resolve coagent_result scope: %w", scopeErr)
	}
	if strings.HasPrefix(strings.TrimSpace(u.Content), agentcore.LifecycleResearchAdmissionRecoveryPrefix) {
		rec, terminal, recoveryErr := h.rt.ResolveLifecycleResearchAdmissionRecovery(ctx, ownerID, computerID, agentID, u.Content, u.TrajectoryID, u.FromAgentID)
		if recoveryErr != nil {
			if errors.Is(recoveryErr, agentcore.ErrInvalidLifecycleResearchRecovery) {
				return nil, nil // durable malformed/foreign recovery occurrence
			}
			return nil, fmt.Errorf("%w: actorruntime: defer lifecycle Research recovery until a distinct wake/restart: %v", actor.ErrDeferUnprocessed, recoveryErr)
		}
		if terminal {
			return nil, nil
		}
		if rec == nil {
			return nil, fmt.Errorf("actorruntime: lifecycle Research admission recovery returned no exact run")
		}
		if err := h.rt.ExecuteActivationSyncChecked(ctx, rec); err != nil {
			if errors.Is(err, agentcore.ErrActivationOccurrenceMustRemainUnprocessed) {
				return nil, fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, err)
			}
			return nil, fmt.Errorf("actorruntime: execute lifecycle Research admission recovery: %w", err)
		}
		return h.memoryFromRunState(rec)
	}
	if strings.HasPrefix(strings.TrimSpace(u.Content), agentcore.PersistentManagementRecoveryPrefix) {
		log.Printf("actorruntime: persistent Management recovery received agent=%s trajectory=%s from=%s", agentID, u.TrajectoryID, u.FromAgentID)
		rec, terminal, recoveryErr := h.rt.ResolvePersistentManagementRecovery(ctx, ownerID, computerID, agentID, u.Content, u.TrajectoryID, u.FromAgentID)
		if recoveryErr != nil {
			if errors.Is(recoveryErr, agentcore.ErrInvalidPersistentManagementRecovery) {
				log.Printf("actorruntime: persistent Management recovery discarded as invalid agent=%s: %v", agentID, recoveryErr)
				return nil, nil
			}
			return nil, fmt.Errorf("%w: actorruntime: defer persistent Management recovery until a distinct wake/restart: %v", actor.ErrDeferUnprocessed, recoveryErr)
		}
		if terminal {
			log.Printf("actorruntime: persistent Management recovery terminal agent=%s", agentID)
			return nil, nil
		}
		if rec == nil {
			return nil, fmt.Errorf("actorruntime: persistent Management recovery returned no exact run")
		}
		log.Printf("actorruntime: persistent Management recovery executing run=%s", rec.RunID)
		if err := h.rt.ExecuteActivationSyncChecked(ctx, rec); err != nil {
			if errors.Is(err, agentcore.ErrActivationOccurrenceMustRemainUnprocessed) {
				return nil, fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, err)
			}
			return nil, fmt.Errorf("actorruntime: execute persistent Management recovery: %w", err)
		}
		return h.memoryFromRunState(rec)
	}
	if strings.HasPrefix(strings.TrimSpace(u.Content), "sha256:") && agentID == agentprofile.Management+":"+ownerID {
		log.Printf("actorruntime: persistent Management live occurrence received agent=%s trajectory=%s from=%s", agentID, u.TrajectoryID, u.FromAgentID)
		rec, terminal, liveErr := h.rt.ResolvePersistentManagementLiveOccurrence(ctx, ownerID, computerID, agentID, u.Content, u.TrajectoryID, u.FromAgentID)
		if liveErr != nil {
			if errors.Is(liveErr, agentcore.ErrPersistentManagementReportOccurrence) {
				// A producer report: never mint execution. Fall through to the
				// generic park-resume path — it reactivates the parked run and
				// injectUserTurns delivers the queued packet.
				goto resumeGeneric
			}
			if errors.Is(liveErr, agentcore.ErrInvalidPersistentManagementRecovery) {
				log.Printf("actorruntime: persistent Management live occurrence discarded as invalid agent=%s: %v", agentID, liveErr)
				return nil, nil
			}
			if errors.Is(liveErr, agentcore.ErrActivationOccurrenceMustRemainUnprocessed) {
				return nil, fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, liveErr)
			}
			return nil, fmt.Errorf("%w: actorruntime: defer persistent Management live occurrence: %v", actor.ErrDeferUnprocessed, liveErr)
		}
		if terminal {
			log.Printf("actorruntime: persistent Management live occurrence terminal agent=%s", agentID)
			return nil, nil
		}
		if rec == nil {
			return nil, fmt.Errorf("actorruntime: persistent Management live occurrence returned no exact run")
		}
		// Locked mint already dispatched initial_dispatch. Resident exact-match
		// Management is already bound. Do not execute here (recovery prefix does).
		log.Printf("actorruntime: persistent Management live occurrence bound run=%s", rec.RunID)
		return nil, nil
	}
	if strings.HasPrefix(agentID, agentprofile.Engineering+":") {
		// Engineering desk occurrence: the document-channel cast. The desk
		// agent never runs; the occurrence's revision opens the assignment
		// directly and the assignment's own activation executes the work.
		// Non-occurrence content (worker updates to assigned engineering
		// agents) falls through to the generic resume path.
		occurrence, occurrenceErr := agentcore.DecodeTextureActorOccurrence(u.Content)
		if occurrenceErr == nil && occurrence.Kind == agentcore.TextureActorOccurrenceDocumentRevision {
			if occurrence.TargetAgentID != agentID || occurrence.OwnerID != ownerID || occurrence.ComputerID != computerID {
				return nil, nil // durable malformed/foreign engineering occurrence
			}
			if strings.TrimSpace(u.TrajectoryID) != "" && strings.TrimSpace(u.TrajectoryID) != occurrence.TrajectoryID {
				return nil, nil // durable foreign trajectory envelope
			}
			if strings.TrimSpace(u.FromAgentID) != "" && strings.TrimSpace(u.FromAgentID) != "owner:"+occurrence.OwnerID {
				return nil, nil // durable foreign source envelope
			}
			docID := strings.TrimSpace(strings.TrimPrefix(agentID, agentprofile.Engineering+":"))
			if docID == "" || docID != occurrence.DocumentID {
				return nil, nil
			}
			if _, reconcileErr := h.rt.ReconcileEngineeringRevisionCast(ctx, ownerID, docID, occurrence.HeadRevisionID); reconcileErr != nil {
				if errors.Is(reconcileErr, store.ErrNotFound) {
					return nil, nil // durable foreign document/revision envelope
				}
				return nil, fmt.Errorf("%w: actorruntime: reconcile engineering revision cast: %v", actor.ErrDeferUnprocessed, reconcileErr)
			}
			return nil, nil
		}
	}
	if strings.HasPrefix(agentID, agentprofile.Texture+":") {
		if h.textureOwner == nil {
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: Texture owner is not bound"))
		}
		occurrence, fate, occurrenceErr := h.textureOwner.ResolveTextureActorOccurrence(ctx, ownerID, computerID, agentID, u.Content)
		if occurrenceErr != nil {
			if errors.Is(occurrenceErr, textureowner.ErrInvalidTextureActorOccurrence) {
				logTextureOccurrenceConsumed(agentID, u, "invalid: "+occurrenceErr.Error())
				return nil, nil
			}
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: validate exact Texture occurrence: %w", occurrenceErr))
		}
		if strings.TrimSpace(u.TrajectoryID) != "" && strings.TrimSpace(u.TrajectoryID) != occurrence.TrajectoryID {
			logTextureOccurrenceConsumed(agentID, u, "foreign trajectory envelope")
			return nil, nil // durable foreign trajectory envelope
		}
		if strings.TrimSpace(u.FromAgentID) != "" {
			expectedSource := occurrence.ProducerAgentID
			if occurrence.Kind == agentcore.TextureActorOccurrenceDocumentRevision {
				expectedSource = "owner:" + occurrence.OwnerID
			}
			if strings.TrimSpace(u.FromAgentID) != expectedSource {
				logTextureOccurrenceConsumed(agentID, u, "foreign source envelope")
				return nil, nil // durable foreign source envelope
			}
		}
		if fate == textureowner.TextureActorOccurrenceTerminal {
			logTextureOccurrenceConsumed(agentID, u, "terminal (disposed, cancelled, late, or head already consumed by a Texture turn)")
			return nil, nil // explicit Store-owned disposed/cancelled/late outcome
		}

		// Reconcile canonical document/head/mutation authority and synchronously
		// execute the exact selected run inside this still-unprocessed occurrence.
		// Generic actor snapshot memory is deliberately ignored.
		rec, reconcileErr := h.textureOwner.ReconcileActorOccurrenceWake(ctx, ownerID, computerID, agentID, occurrence.ResolvedTargetWorkItemID, occurrence)
		if reconcileErr != nil {
			if errors.Is(reconcileErr, textureowner.ErrInvalidTextureActorOccurrence) {
				logTextureOccurrenceConsumed(agentID, u, "reconcile refused: "+reconcileErr.Error())
				return nil, nil
			}
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: reconcile Texture coagent wake: %w", reconcileErr))
		}
		if rec == nil {
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: pending Texture occurrence produced no exact run"))
		}
		if validateErr := h.textureOwner.ValidateActivationAuthority(ctx, ownerID, computerID, agentID, rec.RunID); validateErr != nil {
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: revalidate Texture provider authority: %w", validateErr))
		}
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]any)
		}
		rec.Metadata["actor_reactivate_existing_memory"] = true
		rec.Metadata["actor_reactivated_from_passivated"] = true
		rec.Metadata["texture_trigger_occurrence"] = u.Content
		if err := h.rt.ExecuteActivationSyncChecked(ctx, rec); err != nil {
			if errors.Is(err, agentcore.ErrActivationOccurrenceMustRemainUnprocessed) {
				return nil, fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, err)
			}
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: execute Texture resumed activation: %w", err))
		}
		post, postErr := h.textureOwner.TextureActorOccurrencePostcondition(ctx, occurrence, rec.RunID)
		if postErr != nil {
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: verify Texture occurrence postcondition: %w", postErr))
		}
		if post == textureowner.TextureActorOccurrencePending {
			// A run that reached a terminal state but left the trigger
			// undisposed can never dispose it on retry: decided-invalid.
			// A passivated/active run is genuinely parked awaiting its next
			// wake — the deferral is real, not a live-lock.
			if rec.State.Terminal() {
				return nil, fmt.Errorf("%w: actorruntime: Texture activation terminated without disposing exact trigger (run=%s state=%s)", actor.ErrDurableInvalid, rec.RunID, rec.State)
			}
			return nil, deferTextureOccurrence(fmt.Errorf("actorruntime: Texture activation returned without disposing exact trigger"))
		}
		return h.memoryFromRunState(rec)
	}
resumeGeneric:
	rs, err := decodeResumeState(memory)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: decode resume state for coagent_result: %w", err)
	}
	if rs.RunID == "" {
		// The actor snapshot lost its resume pointer, but the obligation is
		// canonical: when this occurrence resolves to exactly one pending
		// lifecycle control bound to a passivated/blocked run, recover that
		// exact run id from the packet's DeliveredToRunID and continue into the
		// parked-run resume branch below — the packet is the same authority
		// class as the snapshot, not a guessed selection.
		if recoveredRunID, recoverErr := h.resolveBoundControlRunID(ctx, u, ownerID, computerID, agentID); recoverErr == nil && recoveredRunID != "" {
			rs.RunID = recoveredRunID
		} else if recoverErr != nil {
			return nil, fmt.Errorf("%w: actorruntime: resolve bound control run for coagent_result: %v", actor.ErrDeferUnprocessed, recoverErr)
		}
	}
	if rs.RunID == "" {
		// No parked run to resume. The coagent update is in the store
		// mailbox. Reconcile either creates a new run and dispatches it or
		// reports the classified, durably terminal activation outcome.
		_, reconcileErr := h.reconcileCoagentWake(ctx, u)
		if retryErr := acknowledgeDurablyTerminalLifecycleControlActivation(reconcileErr); retryErr != nil {
			return nil, fmt.Errorf("actorruntime: reconcile coagent wake: %w", retryErr)
		}
		// Return nil memory. Any new run will be started by its
		// initial_dispatch message, not by this handler call.
		return nil, nil
	}
	rec, err := h.scopedRunForUpdate(ctx, u, rs.RunID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: load parked run %s: %w", rs.RunID, err)
	}
	if textureRunRecord(rec) {
		return nil, fmt.Errorf("actorruntime: Texture run has noncanonical agent identity")
	}
	if rec.State == types.RunPassivated || rec.State.Active() {
		// Reactivate the run. The coagent update is in the store
		// mailbox; injectUserTurns will pick it up on re-entry.
		//
		// Active() covers RunPending, RunRunning, and RunBlocked:
		// - RunPassivated: normal park-resume (tool loop parked waiting
		//   for a coagent response).
		// - RunBlocked: the run hit a provider error and is blocked.
		//   The coagent update may provide new context that unblocks it.
		// - RunRunning (stale): after a process restart, runs that were
		//   RunRunning are stale — no goroutine is executing them. The
		//   actor handler is single-threaded; if we're processing this
		//   message, the previous HandleUpdate has returned and no one
		//   is executing the run. Reactivate.
		// - RunPending: the run was created but not yet started. The
		//   coagent update will be picked up when the tool loop runs.
		if rec.State.Active() {
			log.Printf("actorruntime: reactivating run %s in state %s (not passivated) for coagent_result", rs.RunID, rec.State)
		}
		wakeProfile, _ := agentprofile.Canonical(rec.AgentProfile)
		wakeRole, _ := agentprofile.Canonical(rec.AgentRole)
		lifecycleControlResearch :=
			(wakeProfile == agentprofile.Research ||
				wakeRole == agentprofile.Research) &&
				strings.TrimSpace(metadataString(rec.Metadata, "request_source")) == "lifecycle_texture_control"
		if lifecycleControlResearch {
			if strings.TrimSpace(u.TrajectoryID) == "" || strings.TrimSpace(u.TrajectoryID) != strings.TrimSpace(rec.TrajectoryID) {
				return nil, fmt.Errorf("actorruntime: parked lifecycle coagent wake trajectory mismatch")
			}
			// Bind pending lifecycle controls before any generic whole-run update,
			// provider execution, or actor acknowledgement. Reconcile also attempts
			// deterministic initial_dispatch; the already processed run dispatch
			// deduplicates, leaving this handler as the synchronous execution boundary.
			reconciled, reconcileErr := h.rt.ReconcileParkedLifecycleCoagentWake(ctx, ownerID, agentID, rs.RunID)
			if reconcileErr != nil {
				return nil, fmt.Errorf("actorruntime: reconcile parked lifecycle coagent wake: %w", reconcileErr)
			}
			if reconciled == nil || strings.TrimSpace(reconciled.RunID) != strings.TrimSpace(rs.RunID) {
				return nil, fmt.Errorf("actorruntime: reconcile parked lifecycle coagent wake returned a different run")
			}
			if strings.TrimSpace(metadataString(reconciled.Metadata, "request_source")) != "lifecycle_texture_control" {
				return nil, fmt.Errorf("actorruntime: reconciled lifecycle run lost canonical request source")
			}
			rec = *reconciled
		}

		// Mark the exact reconciled run for reactivation: load persisted
		// conversation, inject the newly bound update, and resume the tool loop.
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]any)
		}
		rec.Metadata["actor_reactivate_existing_memory"] = true
		rec.Metadata["actor_reactivated_from_passivated"] = true
		if !lifecycleControlResearch {
			rec.Metadata["request_source"] = "update_coagent"
		}
		rec.State = types.RunPending
		rec.Error = ""
		rec.Result = ""
		rec.FinishedAt = nil
		rec.UpdatedAt = time.Now().UTC()
		patch := map[string]any{
			"actor_reactivate_existing_memory":  true,
			"actor_reactivated_from_passivated": true,
		}
		if !lifecycleControlResearch {
			patch["request_source"] = "update_coagent"
		}
		// Lifecycle-bound runs reactivate through the canonical reducer
		// command so the state change and run_reactivated event commit
		// atomically; non-lifecycle runs keep the bare UpdateRun path.
		if err := h.rt.ReactivateRunCanonical(ctx, &rec, types.RunPending, patch); err != nil {
			return nil, fmt.Errorf("actorruntime: reactivate run %s: %w", rs.RunID, err)
		}
		if err := h.rt.ExecuteActivationSyncChecked(ctx, &rec); err != nil {
			if errors.Is(err, agentcore.ErrActivationOccurrenceMustRemainUnprocessed) {
				return nil, fmt.Errorf("%w: %v", actor.ErrDeferUnprocessed, err)
			}
			return nil, fmt.Errorf("actorruntime: execute resumed activation: %w", err)
		}
		return h.memoryFromRunState(&rec)
	}

	// Run is terminal (completed/failed/cancelled) — reconcile the
	// coagent update, either dispatching a new run or acknowledging the
	// classified, durably terminal activation outcome.
	_, reconcileErr := h.reconcileCoagentWake(ctx, u)
	if retryErr := acknowledgeDurablyTerminalLifecycleControlActivation(reconcileErr); retryErr != nil {
		return nil, fmt.Errorf("actorruntime: reconcile coagent wake: %w", retryErr)
	}
	return nil, nil
}

// resolveBoundControlRunID recovers the run id a coagent_result is bound to
// when the actor snapshot lost its resume pointer. u.Content is a canonical
// occurrence identity ("sha256:"+digest over UpdateID+ProducerUpdateID), so the
// packet is resolved by exact content match over the agent's actionable pending
// lifecycle controls — bound rows included. Returns the bound DeliveredToRunID
// only when exactly one pending control matches; zero matches is not an error
// (the caller falls through to generic reconcile), multiple is a fail-closed
// error (never guess). The result feeds rs.RunID so the normal parked-run
// resume branch — ReconcileParkedLifecycleCoagentWake — applies its own run
// guards; this helper only supplies the authoritative run id.
func (h *actorHandler) resolveBoundControlRunID(ctx context.Context, u actor.Update, ownerID, computerID, agentID string) (string, error) {
	content := strings.TrimSpace(u.Content)
	if !strings.HasPrefix(content, "sha256:") {
		return "", nil // not a canonical lifecycle occurrence — nothing to resolve
	}
	updates, err := h.rt.Store().ListActionablePendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
	if err != nil {
		return "", err
	}
	var runID string
	for i := range updates {
		update := updates[i]
		if types.LifecycleControlActorOccurrenceContent(update) != content {
			continue
		}
		// Exact occurrence match. Require the packet's own scope fields to agree
		// with this delivery before trusting its binding.
		if strings.TrimSpace(update.TrajectoryID) != strings.TrimSpace(u.TrajectoryID) ||
			strings.TrimSpace(update.TargetAgentID) != strings.TrimSpace(agentID) ||
			update.Disposition != types.UpdatePending ||
			update.DeliveredAt == nil {
			return "", nil // matched but not a live bound-pending obligation — let reconcile classify it
		}
		bound := strings.TrimSpace(update.DeliveredToRunID)
		if bound == "" {
			return "", nil
		}
		if runID != "" && runID != bound {
			return "", fmt.Errorf("ambiguous bound control runs %s and %s for occurrence", runID, bound)
		}
		runID = bound
	}
	return runID, nil
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return value
}

func acknowledgeDurablyTerminalLifecycleControlActivation(err error) error {
	if errors.Is(err, agentcore.ErrDurablyTerminalLifecycleControlActivation) {
		return nil
	}
	return err
}

func (h *actorHandler) reconcileCoagentWake(ctx context.Context, update actor.Update) (*types.RunRecord, error) {
	ownerID, computerID, agentID, err := parseScopedActorMailboxID(update.ToAgentID)
	if err != nil {
		return nil, err
	}
	prefix := agentprofile.Texture + ":"
	if strings.HasPrefix(agentID, prefix) {
		if h.textureOwner == nil {
			return nil, fmt.Errorf("Texture owner is not bound")
		}
		if strings.TrimSpace(strings.TrimPrefix(agentID, prefix)) == "" {
			return nil, fmt.Errorf("Texture agent id has no document id")
		}
		return h.textureOwner.ReconcileActorWake(ctx, ownerID, computerID, agentID)
	}
	agent, err := h.rt.Store().GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("lookup scoped agent for coagent_result: %w", err)
	}
	wakeAgentProfile, _ := agentprofile.Canonical(agent.Profile)
	wakeAgentRole, _ := agentprofile.Canonical(agent.Role)
	if wakeAgentProfile == agentprofile.Texture ||
		wakeAgentRole == agentprofile.Texture {
		return nil, fmt.Errorf("Texture subject has noncanonical agent identity")
	}
	return h.rt.ReconcileCoagentWake(ctx, ownerID, agentID)
}

// handleCancel aborts a parked run.
func (h *actorHandler) handleCancel(ctx context.Context, u actor.Update, memory []byte) ([]byte, error) {
	rs, err := decodeResumeState(memory)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: decode resume state for cancel: %w", err)
	}
	if rs.RunID == "" {
		return nil, nil
	}
	rec, err := h.scopedRunForUpdate(ctx, u, rs.RunID)
	if err != nil {
		return nil, nil // run gone — nothing to cancel
	}
	if rec.State == types.RunPassivated || rec.State == types.RunPending || rec.State == types.RunRunning || rec.State == types.RunBlocked {
		rec.State = types.RunCancelled
		rec.Error = "cancelled by durable trajectory"
		now := time.Now().UTC()
		rec.UpdatedAt = now
		rec.FinishedAt = &now
		_ = h.rt.Store().UpdateRun(ctx, rec)
	}
	return nil, nil
}

// memoryFromRunState encodes the resume pointer for passivated and still-active
// runs (including blocked). Terminal states clear memory.
func (h *actorHandler) memoryFromRunState(rec *types.RunRecord) ([]byte, error) {
	if rec == nil {
		return nil, nil
	}
	if rec.State == types.RunPassivated || rec.State.Active() {
		phase := "parked"
		if rec.State == types.RunBlocked {
			phase = "blocked"
		}
		rs := resumeState{RunID: rec.RunID, Phase: phase}
		return json.Marshal(rs)
	}
	return nil, nil
}

func decodeResumeState(memory []byte) (resumeState, error) {
	var rs resumeState
	if len(memory) == 0 {
		return rs, nil
	}
	if err := json.Unmarshal(memory, &rs); err != nil {
		return rs, err
	}
	return rs, nil
}

func (h *actorHandler) scopedRunForUpdate(ctx context.Context, update actor.Update, runID string) (types.RunRecord, error) {
	ownerID, computerID, _, err := parseScopedActorMailboxID(update.ToAgentID)
	if err != nil {
		return types.RunRecord{}, err
	}
	rec, err := h.rt.Store().GetLifecycleRun(ctx, ownerID, computerID, strings.TrimSpace(runID))
	if err == nil {
		return rec, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return types.RunRecord{}, err
	}
	rec, err = h.rt.Store().GetRunByOwner(ctx, ownerID, strings.TrimSpace(runID))
	if err != nil {
		return types.RunRecord{}, err
	}
	if strings.TrimSpace(rec.ComputerID) != computerID {
		return types.RunRecord{}, fmt.Errorf("actorruntime: run computer identity mismatch")
	}
	return rec, nil
}

func scopedActorMailboxID(ownerID, computerID, agentID string) string {
	return strings.TrimSpace(ownerID) + "\x00" + strings.TrimSpace(computerID) + "\x00" + strings.TrimSpace(agentID)
}

func parseScopedActorMailboxID(mailboxID string) (string, string, string, error) {
	parts := strings.Split(strings.TrimSpace(mailboxID), "\x00")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("actorruntime: malformed scoped actor mailbox id")
	}
	return parts[0], parts[1], parts[2], nil
}

// Compile-time assertion that actorHandler implements actor.Handler.
var _ actor.Handler = (*actorHandler)(nil)
