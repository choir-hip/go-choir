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
}

// newActorHandler creates the handler. The rt must have its store, provider,
// and tool registry configured before runs are dispatched.
func newActorHandler(rt *agentcore.Runtime, textureOwner *textureowner.Handler) *actorHandler {
	return &actorHandler{rt: rt, textureOwner: textureOwner}
}

// HandleUpdate is the execution boundary. One call per incoming update.
// A single run may span many HandleUpdate calls (initial_dispatch → park →
// coagent_result → park → ... → completion).
func (h *actorHandler) HandleUpdate(ctx context.Context, agentID string, u actor.Update, memory []byte) ([]byte, error) {
	switch u.Kind {
	case "initial_dispatch":
		return h.handleInitialDispatch(ctx, u, memory)
	case "coagent_result":
		return h.handleCoagentResult(ctx, u, memory)
	case "cancel":
		return h.handleCancel(ctx, u, memory)
	default:
		log.Printf("actorruntime: handler: unknown update kind %q for agent %s", u.Kind, agentID)
		return memory, nil // leave memory unchanged; update marked processed
	}
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
	if rec.State != types.RunPending && rec.State != types.RunRunning {
		// Terminal/passivated runs were already handled by an earlier dispatch.
		return nil, nil
	}
	h.rt.ExecuteActivationSync(ctx, &rec)
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
	rs, err := decodeResumeState(memory)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: decode resume state for coagent_result: %w", err)
	}
	if rs.RunID == "" {
		// No parked run to resume. The coagent update is in the store
		// mailbox. Create a new run via the reconcile logic, which
		// calls rt.activate(rec) → sends an initial_dispatch actor
		// message. The actor loop will process it next.
		if _, err := h.reconcileCoagentWake(ctx, u); err != nil {
			return nil, fmt.Errorf("actorruntime: reconcile coagent wake: %w", err)
		}
		// Return nil memory — the new run will be started by the
		// initial_dispatch message, not by this handler call.
		return nil, nil
	}
	rec, err := h.scopedRunForUpdate(ctx, u, rs.RunID)
	if err != nil {
		return nil, fmt.Errorf("actorruntime: load parked run %s: %w", rs.RunID, err)
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
		// Mark the run for reactivation: load persisted conversation,
		// inject the new coagent update via injectUserTurns, resume the
		// tool loop.
		if rec.Metadata == nil {
			rec.Metadata = make(map[string]any)
		}
		rec.Metadata["actor_reactivate_existing_memory"] = true
		rec.Metadata["actor_reactivated_from_passivated"] = true
		rec.Metadata["request_source"] = "update_coagent"
		rec.State = types.RunPending
		rec.Error = ""
		rec.Result = ""
		rec.FinishedAt = nil
		rec.UpdatedAt = time.Now().UTC()
		if err := h.rt.Store().UpdateRun(ctx, rec); err != nil {
			return nil, fmt.Errorf("actorruntime: reactivate run %s: %w", rs.RunID, err)
		}
		h.rt.ExecuteActivationSync(ctx, &rec)
		return h.memoryFromRunState(&rec)
	}

	// Run is terminal (completed/failed/cancelled) — create a new run
	// for the coagent update via the reconcile path.
	if _, err := h.reconcileCoagentWake(ctx, u); err != nil {
		return nil, fmt.Errorf("actorruntime: reconcile coagent wake: %w", err)
	}
	return nil, nil
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
	if _, err := h.rt.Store().GetAgentByScope(ctx, ownerID, computerID, agentID); err != nil {
		return nil, fmt.Errorf("lookup scoped agent for coagent_result: %w", err)
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

// memoryFromRunState encodes the resume pointer if the run passivated, or
// returns nil (clears memory) if the run completed or failed.
func (h *actorHandler) memoryFromRunState(rec *types.RunRecord) ([]byte, error) {
	if rec == nil {
		return nil, nil
	}
	switch rec.State {
	case types.RunPassivated:
		rs := resumeState{RunID: rec.RunID, Phase: "parked"}
		return json.Marshal(rs)
	default:
		// RunCompleted, RunFailed, or any other terminal state — clear memory.
		return nil, nil
	}
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
	if strings.TrimSpace(rec.SandboxID) != computerID {
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
