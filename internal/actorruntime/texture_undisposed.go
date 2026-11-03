package actorruntime

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// textureUndisposedRecheckLimit and textureUndisposedMinWait bound a Texture
// trigger whose turn ended without disposing it: it is closed with the fate
// "trigger not disposed" once it was re-checked this many times AND this long
// after its turn. The re-checks never call the model; the wait gives a sibling
// turn (another report, a newer owner revision) time to dispose it first, and
// does not shrink when two actor updates carry the one trigger and share the
// count (docs/problems/texture-trigger-not-disposed-loop-2026-11-03.md).
const textureUndisposedRecheckLimit = 6

var textureUndisposedMinWait = 30 * time.Second

// textureUndisposedRetention prunes records, including the tombstones of
// closed triggers, so the record cannot grow without bound.
const textureUndisposedRetention = time.Hour

// undisposedTrigger is one Texture trigger whose turn ran and ended without
// disposing it. closed marks a trigger already given its fate: another
// delivery of it is consumed with no work and no model call.
type undisposedTrigger struct {
	runID    string
	rechecks int
	since    time.Time
	closed   bool
}

// undisposedTriggers records, per process, the Texture triggers that already
// had their model turn. A trigger gets one model turn however many actor
// updates carry it (an owner revision arrives as both an owner_revision outbox
// wake and a coagent_result) and however often the dispatcher re-delivers it;
// later deliveries only re-check it. Losing the record at a restart costs at
// most one more turn per planned restart (a crash boot interrupts the work).
type undisposedTriggers struct {
	mu      sync.Mutex
	entries map[string]*undisposedTrigger
}

// textureTriggerKey names the canonical trigger an occurrence carries, not the
// actor update that delivered it.
func textureTriggerKey(o agentcore.TextureActorOccurrence) string {
	fields := []string{o.Kind, o.OwnerID, o.ComputerID, o.TrajectoryID, o.TargetAgentID, o.DocumentID}
	switch o.Kind {
	case agentcore.TextureActorOccurrenceDocumentRevision:
		fields = append(fields, o.HeadRevisionID)
	default:
		fields = append(fields, o.ProducerAgentID, o.ProducerUpdateID)
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return strings.Join(fields, "\x00")
}

func (t *undisposedTriggers) pruneLocked(now time.Time) {
	for k, e := range t.entries {
		if now.Sub(e.since) > textureUndisposedRetention {
			delete(t.entries, k)
		}
	}
}

// lookup reports whether the trigger already had its turn, and whether it was
// already closed with its fate.
func (t *undisposedTriggers) lookup(key string, now time.Time) (hadTurn, closed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(now)
	e, ok := t.entries[key]
	if !ok {
		return false, false
	}
	return true, e.closed
}

func (t *undisposedTriggers) hadTurn(key string) bool {
	had, _ := t.lookup(key, time.Now().UTC())
	return had
}

// recordTurn notes that the trigger's turn ended without disposing it.
func (t *undisposedTriggers) recordTurn(key, runID string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.entries == nil {
		t.entries = make(map[string]*undisposedTrigger)
	}
	t.pruneLocked(now)
	if _, ok := t.entries[key]; !ok {
		t.entries[key] = &undisposedTrigger{runID: runID, since: now}
	}
}

// recheck counts one more re-check without progress and returns the count,
// the time since the turn, and the run whose turn left the trigger.
func (t *undisposedTriggers) recheck(key string, now time.Time) (int, time.Duration, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.entries[key]
	if !ok {
		return 0, 0, ""
	}
	e.rechecks++
	return e.rechecks, now.Sub(e.since), e.runID
}

// close keeps a tombstone so a sibling delivery of the closed trigger is
// consumed without work, even if it would resolve to new open work.
func (t *undisposedTriggers) close(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if e, ok := t.entries[key]; ok {
		e.closed = true
	}
}

func (t *undisposedTriggers) forget(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}

// recheckUndisposedTextureTrigger handles a delivery of a trigger whose turn
// already ended without disposing it. It never executes the run. The caller
// has already resolved the occurrence as still pending.
func (h *actorHandler) recheckUndisposedTextureTrigger(ctx context.Context, u actor.Update, ownerID, computerID, agentID, key string, o agentcore.TextureActorOccurrence, closed bool) error {
	if closed {
		logTextureOccurrenceConsumed(agentID, u, "trigger already closed: trigger not disposed")
		return fmt.Errorf("%w: actorruntime: Texture trigger already closed as not disposed", actor.ErrDurableInvalid)
	}
	n, waited, runID := h.undisposed.recheck(key, time.Now().UTC())
	if n < textureUndisposedRecheckLimit || waited < textureUndisposedMinWait {
		return deferTextureOccurrence(fmt.Errorf("actorruntime: Texture trigger not disposed by run %s's turn; re-check %d (closes at %d and %s), no new model call", runID, n, textureUndisposedRecheckLimit, textureUndisposedMinWait))
	}
	reason := fmt.Sprintf("%s: Texture's turn (run %s) ended without a revision, decision or answer that disposes the %s trigger; re-checked %d times over %s with no progress", textureowner.TriggerNotDisposedReason, runID, o.Kind, n, waited.Round(time.Second))
	if err := h.textureOwner.CloseUndisposedTextureTrigger(ctx, o, runID, reason); err != nil {
		return deferTextureOccurrence(fmt.Errorf("actorruntime: close undisposed Texture trigger: %w", err))
	}
	h.undisposed.close(key)
	h.noteTextureRootTrajectory(ctx, ownerID, computerID, o.TrajectoryID)
	log.Printf("actorruntime: Texture occurrence %s/%s (%s) closed: %s", agentID, u.UpdateID, u.Kind, reason)
	return fmt.Errorf("%w: actorruntime: %s", actor.ErrDurableInvalid, reason)
}

// textureTurnParkedIdleSince reports whether the run's model turn ran to its
// idle end in the activation that started at start: the tool loop's idle park
// (passivateIdleToolLoopRun) stamps actor_sleep_state "idle" and
// actor_sleep_at. Other passivations (a runtime injection failure, a restart,
// a refused authority) are not a spent turn.
func textureTurnParkedIdleSince(rec types.RunRecord, start time.Time) bool {
	if rec.State != types.RunPassivated || rec.Metadata == nil {
		return false
	}
	if state, _ := rec.Metadata["actor_sleep_state"].(string); state != "idle" {
		return false
	}
	at, _ := rec.Metadata["actor_sleep_at"].(string)
	sleptAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(at))
	return err == nil && !sleptAt.Before(start)
}
