package agentcore

// POST /api/computers/<id>/self-development/operations/<X>/revert
// (docs/design/selfdev-apply-and-revert-2026-11-06.md, 3.5, R3). An owner act
// (the proxy's gate admits it only for the owner's session or an owner-acts
// key). The guest assembles the rollback bindings itself from its current
// head, so a stale tap cannot fail on bindings a browser read earlier.
//
// Reverting X restores the state before X. X's prior effective head is the
// one its accept event expected. If that head belongs to an applied or
// rolled-back operation with full evidence, the revert is the existing
// rollback to it. Otherwise (a genesis or platform-update checkpoint, or an
// earlier base restore) the booted base release is restored with no
// self-development overlay (selfdev_base_restore.go).
//
// Idempotency key: revert:<X>:<latest landed operation>. A double tap is one
// act; a revert after later applies is a new act.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
)

func (h *APIHandler) revertSelfDevelopmentOperation(w http.ResponseWriter, r *http.Request, ownerID, computerID, operationID string) {
	rt := h.rt
	if rt.eventAppender == nil || rt.store == nil || rt.selfdevRoute == nil || rt.selfdevOperations == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "revert authority unavailable"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid revert request"})
		return
	}
	if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		decoder.DisallowUnknownFields()
		var empty struct{}
		if err := decoder.Decode(&empty); err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "a revert request takes no fields"})
			return
		}
	}
	ctx := r.Context()
	operationID = strings.TrimSpace(operationID)
	reverted, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		if selfdevOperationMissing(err) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
			return
		}
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	if reverted.IsPackage() {
		h.revertPackageOperation(w, ctx, ownerID, computerID, reverted)
		return
	}
	latest, err := h.latestLandedOperation(ctx, computerID, false)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	since := "none"
	if latest != nil {
		since = latest.OperationID
	}
	key := "revert:" + operationID + ":" + since
	// A double request with the same key is one act.
	if existing, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, key); err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	} else if found {
		if existing.State != selfdev.StateRollbackPending && existing.State != selfdev.StateRolledBack {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "revert idempotency binding changed"})
			return
		}
		writeAPIJSON(w, http.StatusOK, existing)
		return
	}
	if reverted.State != selfdev.StateApplied || strings.HasPrefix(reverted.OperationID, "rollback-") || strings.HasPrefix(reverted.OperationID, "genesis-") {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation " + operationID + " is " + reverted.State + ": only an applied self-development operation is reverted"})
		return
	}
	accepted, found, err := rt.store.EventByDigest(ctx, computerID, reverted.DecisionEvent)
	if err != nil || !found || !computerevent.IsSHA256(accepted.ExpectedEffectiveEventHead) || !computerevent.IsSHA256(accepted.ExpectedEffectiveStateCommitment) {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "the state before operation " + operationID + " is not recorded on the tape"})
		return
	}
	priorHead, priorCommitment := accepted.ExpectedEffectiveEventHead, accepted.ExpectedEffectiveStateCommitment
	prior, priorErr := rt.selfdevOperations.GetByEffectiveHead(ctx, computerID, priorHead)
	if priorErr != nil && !selfdevOperationMissing(priorErr) {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	// The state before X skips package events: a package operation between X
	// and the platform state it follows changed no platform release, and a
	// platform revert never undoes a package install.
	var skipErr error
	prior, priorErr, priorHead, priorCommitment, skipErr = h.skipPackageEvents(ctx, computerID, prior, priorErr, priorHead, priorCommitment)
	if skipErr != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "the state before operation " + operationID + " is not recorded on the tape: " + skipErr.Error()})
		return
	}
	// The latest landed operation already restored this state and nothing has
	// landed since: a second tap after the revert settled.
	if latest != nil && latest.State == selfdev.StateRolledBack && latest.BaseHead == priorHead {
		writeAPIJSON(w, http.StatusOK, *latest)
		return
	}
	live, err := h.liveOperations(ctx, computerID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert lookup failed"})
		return
	}
	if !live[reverted.OperationID] {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation " + operationID + " was already undone by a later revert or rollback; reverting it would reinstall an earlier change"})
		return
	}
	head, err := rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "event head unavailable"})
		return
	}
	// An orphaned revert: this revert's own rollback_requested event committed
	// but its row was never created (a crash, a head read or a start that
	// failed). The pending transition is then ours, and the openers' replay
	// paths finish it; any other pending transition is not ours to touch.
	if head.PendingTransitionRef != "" {
		own, found, err := rt.store.EventByIdempotency(ctx, computerID, "selfdev-rollback-requested-"+key)
		ownDigest := ""
		if err == nil && found {
			ownDigest, _ = own.Digest()
		}
		if ownDigest == "" || head.PendingTransitionRef != ownDigest {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "a transition is pending on the event head; wait for it to settle before reverting"})
			return
		}
	}
	if head.EffectiveEventHead == priorHead {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "the computer is already in the state before operation " + operationID})
		return
	}
	var generation uint64
	if slotID, slotErr := routeledger.RouteSlotID(rt.selfdevRouteOwnerID, rt.selfdevRouteDesktopID); slotErr == nil {
		if route, routeErr := rt.selfdevRoute.ResolveComputerVersionRoute(ctx, slotID); routeErr == nil {
			generation = route.Slot.Generation
		}
	}
	// Open through a buffer: a tap that loses a race to the same act gets that
	// act's row, not a 409.
	buf := &revertBuffer{header: http.Header{}}
	if priorErr == nil && revertTargetHasEvidence(prior) {
		if generation == 0 {
			writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "rollback route authority unavailable"})
			return
		}
		h.openSelfDevelopmentRollback(buf, ctx, ownerID, computerID, selfDevelopmentRollbackRequest{
			ExpectedDesiredHead: head.DesiredEventHead, CurrentAppliedHead: head.EffectiveEventHead,
			ToAppliedHead: prior.EffectiveHead, PriorMaterialization: prior.MaterializationReceipt,
			PriorCheckpoint: prior.CheckpointRef, ExpectedRouteGeneration: generation, IdempotencyKey: key,
		})
	} else {
		// No evidence to roll back to: only a genesis row, an earlier base
		// restore, or a checkpoint event with no row is restored as the base.
		restorable := false
		if priorErr == nil {
			restorable = strings.HasPrefix(prior.OperationID, "genesis-") || selfdev.IsBaseRestore(prior)
		} else {
			priorEvent, ok, err := rt.store.EventByDigest(ctx, computerID, priorHead)
			restorable = err == nil && ok && (priorEvent.EventKind == computerevent.EventGenesisImported || priorEvent.EventKind == computerevent.EventMaterializationApplied)
		}
		if !restorable {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "the state before operation " + operationID + " is not a restorable checkpoint"})
			return
		}
		h.openBaseRestore(buf, ctx, ownerID, computerID, reverted, key, priorHead, priorCommitment, generation)
	}
	if buf.status == http.StatusConflict || buf.status >= 500 {
		for attempt := 0; attempt < 10; attempt++ {
			if existing, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, key); err == nil && found &&
				(existing.State == selfdev.StateRollbackPending || existing.State == selfdev.StateRolledBack) {
				writeAPIJSON(w, http.StatusOK, existing)
				return
			}
			if buf.status != http.StatusConflict {
				break
			}
			select {
			case <-ctx.Done():
				attempt = 10
			case <-time.After(50 * time.Millisecond):
			}
		}
	}
	buf.flushTo(w)
}

// packagePriorHops bounds how many package events a platform revert skips.
const packagePriorHops = 32

// skipPackageEvents follows a platform revert's prior head past package
// operations (docs/design/package-operation-decision-2026-11-06.md 2.5). While
// the prior head names a package operation (its row, or a package_applied
// event), the prior becomes that operation's own prior: the head and
// commitment its accept event expected. It stops at the first platform state
// (an applied platform operation, a base restore, genesis or a platform
// checkpoint); the terminal set is the caller's restorable fallback, not
// widened here.
func (h *APIHandler) skipPackageEvents(ctx context.Context, computerID string, prior selfdev.Operation, priorErr error, priorHead, priorCommitment string) (selfdev.Operation, error, string, string, error) {
	rt := h.rt
	for hops := 0; ; hops++ {
		decision := ""
		switch {
		case priorErr == nil && prior.IsPackage():
			decision = prior.DecisionEvent
		default:
			event, ok, err := rt.store.EventByDigest(ctx, computerID, priorHead)
			if err != nil {
				return prior, priorErr, priorHead, priorCommitment, err
			}
			if ok && event.EventKind == computerevent.EventPackageApplied {
				decision = event.DecisionRef
			}
		}
		if decision == "" {
			return prior, priorErr, priorHead, priorCommitment, nil
		}
		if hops >= packagePriorHops {
			return prior, priorErr, priorHead, priorCommitment, errors.New("too many package operations between the operation and its platform prior")
		}
		accepted, ok, err := rt.store.EventByDigest(ctx, computerID, decision)
		if err != nil || !ok || !computerevent.IsSHA256(accepted.ExpectedEffectiveEventHead) || !computerevent.IsSHA256(accepted.ExpectedEffectiveStateCommitment) {
			return prior, priorErr, priorHead, priorCommitment, errors.New("a package operation's own prior state is not recorded")
		}
		priorHead, priorCommitment = accepted.ExpectedEffectiveEventHead, accepted.ExpectedEffectiveStateCommitment
		var lookupErr error
		prior, lookupErr = rt.selfdevOperations.GetByEffectiveHead(ctx, computerID, priorHead)
		if lookupErr != nil && !selfdevOperationMissing(lookupErr) {
			return prior, lookupErr, priorHead, priorCommitment, lookupErr
		}
		priorErr = lookupErr
	}
}

// revertPackageOperation is the revert of a package operation: it uninstalls
// the install the operation made, appends package_reverted and records the row
// rolled back. It never calls the updater and moves no platform head. A second
// tap answers the settled row.
func (h *APIHandler) revertPackageOperation(w http.ResponseWriter, ctx context.Context, ownerID, computerID string, reverted selfdev.Operation) {
	switch reverted.State {
	case selfdev.StateRolledBack:
		writeAPIJSON(w, http.StatusOK, reverted)
		return
	case selfdev.StateApplied:
	default:
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "operation " + reverted.OperationID + " is " + reverted.State + ": only an applied self-development operation is reverted"})
		return
	}
	row, status, err := h.rt.revertPackageOperation(ctx, ownerID, computerID, reverted)
	if err != nil {
		if status == 0 {
			status = http.StatusConflict
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, row)
}

// revertBuffer holds an opener's answer until the revert decides it is final.
type revertBuffer struct {
	header http.Header
	status int
	body   []byte
}

func (b *revertBuffer) Header() http.Header { return b.header }
func (b *revertBuffer) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *revertBuffer) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}
	b.body = append(b.body, p...)
	return len(p), nil
}
func (b *revertBuffer) flushTo(w http.ResponseWriter) {
	for key, values := range b.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if b.status == 0 {
		b.status = http.StatusOK
	}
	w.WriteHeader(b.status)
	_, _ = w.Write(b.body)
}

// liveOperations is the set of operations whose change the computer's current
// state still carries (liveOperationSet).
func (h *APIHandler) liveOperations(ctx context.Context, computerID string) (map[string]bool, error) {
	rows, err := h.rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateApplied, selfdev.StateRolledBack)
	if err != nil {
		return nil, err
	}
	return liveOperationSet(rows), nil
}

// liveOperationSet replays the settled rows in creation order: an apply adds
// itself; a rollback or base restore returns to the live set recorded when its
// target head was current (empty for a checkpoint with no row). An operation a
// later rollback undid still reads applied, so state alone cannot say.
func liveOperationSet(rows []selfdev.Operation) map[string]bool {
	sorted := append([]selfdev.Operation(nil), rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, errA := time.Parse(time.RFC3339Nano, sorted[i].CreatedAt)
		b, errB := time.Parse(time.RFC3339Nano, sorted[j].CreatedAt)
		if errA != nil || errB != nil {
			return sorted[i].CreatedAt < sorted[j].CreatedAt
		}
		return a.Before(b)
	})
	// The platform layer only: a package operation is tracked on its own
	// (its row is applied until its revert records it rolled back).
	platform := make([]selfdev.Operation, 0, len(sorted))
	for _, row := range sorted {
		if !row.IsPackage() {
			platform = append(platform, row)
		}
	}
	sorted = platform
	live := map[string]bool{}
	snapshots := map[string]map[string]bool{}
	copySet := func(in map[string]bool) map[string]bool {
		out := make(map[string]bool, len(in))
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	for _, row := range sorted {
		if row.State == selfdev.StateRolledBack && strings.HasPrefix(row.OperationID, "rollback-") {
			live = copySet(snapshots[row.BaseHead])
		} else if row.State == selfdev.StateApplied {
			live[row.OperationID] = true
		} else {
			continue
		}
		snapshots[row.EffectiveHead] = copySet(live)
	}
	return live
}

// selfdevOperationMissing: the store's single-row reads answer sql.ErrNoRows.
func selfdevOperationMissing(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, selfdev.ErrOperationNotFound)
}

// revertTargetHasEvidence reports whether an operation can be the target of
// the existing rollback: an applied or rolled-back operation that carries its
// verifier, release, checkpoint and route evidence. A genesis baseline and a
// base restore carry none.
func revertTargetHasEvidence(op selfdev.Operation) bool {
	return !strings.HasPrefix(op.OperationID, "genesis-") && !selfdev.IsBaseRestore(op) &&
		(op.State == selfdev.StateApplied || op.State == selfdev.StateRolledBack) &&
		computerevent.IsSHA256(op.EffectiveHead) && computerevent.IsSHA256(op.BundleDigest) && computerevent.IsSHA256(op.ReleaseDigest) &&
		op.CodeRef != "" && op.ArtifactProgramRef != "" && len(op.VerifierRefs) > 0 && computerevent.IsSHA256(op.VerifierRefs[0]) &&
		op.MaterializationReceipt != "" && op.CheckpointRef != "" && op.RouteReceipt != ""
}

// latestLandedOperation is the newest-created operation of one layer whose
// head landed or whose attempt ended (the CLI's anchor for a rollback intent):
// package operations only when packageLayer, platform operations only
// otherwise.
func (h *APIHandler) latestLandedOperation(ctx context.Context, computerID string, packageLayer bool) (*selfdev.Operation, error) {
	landed, err := h.rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateApplied, selfdev.StateDegraded, selfdev.StateFailed, selfdev.StateRolledBack)
	if err != nil {
		return nil, err
	}
	var best *selfdev.Operation
	var bestAt time.Time
	for index := range landed {
		if landed[index].IsPackage() != packageLayer {
			continue
		}
		at, parseErr := time.Parse(time.RFC3339Nano, landed[index].CreatedAt)
		if parseErr != nil {
			continue
		}
		if best == nil || at.After(bestAt) || (at.Equal(bestAt) && landed[index].OperationID > best.OperationID) {
			best, bestAt = &landed[index], at
		}
	}
	return best, nil
}

// openBaseRestore opens the revert whose target is the booted base release.
func (h *APIHandler) openBaseRestore(w http.ResponseWriter, ctx context.Context, ownerID, computerID string, reverted selfdev.Operation, key, priorHead, priorCommitment string, generation uint64) {
	rt := h.rt
	baselineRoot := rt.packageMigrationBaselineRoot()
	if baselineRoot == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the booted base release is unavailable"})
		return
	}
	target, err := rt.servingBaselineManifest(ctx, computerID, baselineRoot)
	if err != nil || !computerevent.IsSHA256(target.ContentDigest) {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "the booted base release is unavailable"})
		return
	}
	eventIdempotency := "selfdev-rollback-requested-" + key
	existingEvent, eventFound, err := rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert event lookup failed"})
		return
	}
	// The release the row is bound to. An orphaned request keeps the digest its
	// event recorded, even if the booted base has moved since: the row then
	// refuses at drive (the target no longer matches), the refusal discharges
	// the pending transition and the row closes failed, so the tape is never
	// left pending with no owner.
	releaseDigest := target.ContentDigest
	if eventFound {
		if existingEvent.EventKind != computerevent.EventRollbackRequested || existingEvent.DecisionRef != priorHead || !computerevent.IsSHA256(existingEvent.ProposedEffectRef) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "revert event binding changed"})
			return
		}
		releaseDigest = existingEvent.ProposedEffectRef
	}
	commitmentBytes, err := computerevent.CanonicalJSON(map[string]string{
		"kind": "base_restore", "reverted_operation": reverted.OperationID, "target_event_head": priorHead,
		"base_release": releaseDigest, "idempotency_key": key,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "invalid revert request"})
		return
	}
	commitment := computerevent.DigestBytes(commitmentBytes)
	if !eventFound {
		event, err := rollbackRequestedEvent(computerID, ownerID, eventIdempotency, commitment, reverted.TrajectoryID, reverted.CapsuleID, releaseDigest, priorHead)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert event identity unavailable"})
			return
		}
		if _, err := rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{TargetStateCommitment: priorCommitment}, nil); err != nil {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
			return
		}
	}
	next, err := rt.store.Head(ctx, computerID)
	if err != nil || next == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert projection unavailable"})
		return
	}
	// The row names the rollback_requested event itself, not whatever event is
	// the head by now.
	stored, ok, err := rt.store.EventByIdempotency(ctx, computerID, eventIdempotency)
	requestedDigest := ""
	if err == nil && ok {
		requestedDigest, _ = stored.Digest()
	}
	if requestedDigest == "" {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "revert event unavailable"})
		return
	}
	operation, err := rt.selfdevOperations.StartBaseRestore(ctx, selfdev.BaseRestoreStartRequest{
		ComputerID: computerID, IdempotencyKey: key, RequestCommitment: commitment,
		RollbackEvent: requestedDigest, DecisionActor: ownerID, CurrentDesired: next.DesiredEventHead,
		CurrentEffective: next.EffectiveEventHead, PriorEffectiveHead: priorHead, RevertedOperation: reverted.OperationID,
		TrajectoryID: reverted.TrajectoryID, BaseReleaseDigest: releaseDigest,
		CodeRef: target.CodeRef, ArtifactProgramRef: target.ArtifactProgramRef, RouteGeneration: generation,
	})
	if err != nil {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	rt.triggerSelfDevelopmentReconcile()
	writeAPIJSON(w, http.StatusCreated, operation)
}
