package agentcore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The owner's rebase retry (U4 slice 8, docs/design/u4-stack-rebase-build-2026-10-26.md).
// A pair is spent when its three automatic attempts are closed with no claim,
// or when a claimed round ended: nothing automatic follows. This owner act
// re-opens it with one notice numbered above the cap and stamped as the
// owner's (types.IsStackRebaseNotice). The claim of that generation is its own
// (types.StackRebaseClaimKey), so the spent pair's claim stands. It is an owner
// act through the product API (POST /api/computers/<id>/self-development/
// rebase-retry); the computer routes are read-only to a desk
// (validateProductAPIRoute), so management cannot call it. The timer never
// mints above the cap; each retry is one notice, refused while the previous
// attempt is live, so the root is never asked twice at once.

type rebaseRetryResult struct {
	Minted  string
	Attempt int
}

// rebaseRetryRefusal is a refused retry; Code is the stable reason.
type rebaseRetryRefusal struct{ Code, Reason string }

func (e *rebaseRetryRefusal) Error() string { return e.Code + ": " + e.Reason }

func refuseRebaseRetry(code, format string, args ...any) error {
	return &rebaseRetryRefusal{Code: code, Reason: fmt.Sprintf(format, args...)}
}

// retryRebaseNotice mints the owner's next attempt for the moved stack's pair.
func (rt *Runtime) retryRebaseNotice(ctx context.Context, ownerID string) (rebaseRetryResult, error) {
	if rt == nil || rt.store == nil {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_unavailable", "the runtime has no store")
	}
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" || computerID == "" {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_unavailable", "the runtime has no store or scope")
	}
	if !rt.rebaseOfferOn(ctx, ownerID, computerID) {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_off", "source.rebase_offer, source.projection and texture.root_desk must all be on")
	}
	result, err := rt.ReconcileSourceProjection(ctx, rebaseReconcileReason)
	if err != nil {
		return rebaseRetryResult{}, fmt.Errorf("reconcile: %w", err)
	}
	if result.State != sourceproj.StateBaseMoved {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_not_moved", "the applied stack is %s, not on a moved base; there is nothing to rebase", result.State)
	}
	booted, stackBase, operationID := strings.TrimSpace(result.BootedBase), strings.TrimSpace(result.Base), strings.TrimSpace(result.OperationID)
	if booted == "" || stackBase == "" || operationID == "" || strings.HasPrefix(operationID, "genesis-") {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_not_moved", "the stack names no source operation to rebase")
	}
	if !sourceproj.HasCommit(ctx, rt.releaseRepo(), booted) {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_base_missing", "the booted base's objects are not in the guest repository yet")
	}
	rootInbox, docID, live, err := rt.rebaseRoot(ctx, ownerID, computerID)
	if err != nil {
		return rebaseRetryResult{}, fmt.Errorf("root Texture RLM: %w", err)
	}
	if !live {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_no_root", "there is no live root Texture RLM to ask")
	}
	// No rebase round of this operation runs, for any base. An assignment that
	// cannot be read is not known to be finished: fail closed. The commit below
	// repeats the check under the open path's lock.
	claims, err := rt.store.ListRebaseNoticeClaims(ctx, ownerID, computerID, operationID)
	if err != nil {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_unreadable", "the operation's rebase claims cannot be read: %v", err)
	}
	for _, claim := range claims {
		assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, claim.AssignmentID, 1)
		if err != nil {
			return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_unreadable", "assignment %s of a rebase claim cannot be read (%v), so it is not known to be finished", claim.AssignmentID, err)
		}
		if !assignment.Disposition.Terminal() {
			return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_round_running", "assignment %s still runs the rebase of %s", claim.AssignmentID, operationID)
		}
	}
	// The latest attempt of the pair: the automatic ones are contiguous from 1,
	// the owner's follow above the cap.
	var latest *types.CommitmentRecord
	attempt := 0
	for n := 1; ; n++ {
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.StackRebaseNoticeID(operationID, booted, n))
		if err != nil {
			return rebaseRetryResult{}, err
		}
		if rec == nil {
			if n <= types.RebaseNoticeMaxAttempts {
				continue // automatic attempts end where the timer stopped; owner retries may follow a gap
			}
			break
		}
		latest, attempt = rec, n
	}
	if latest == nil {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_no_attempt", "no notice has been minted for this pair yet; the computer's timer mints the first after it has stayed up")
	}
	pair := types.StackRebasePairKey(operationID, booted)
	if _, err := rt.store.GetRebaseNoticeClaim(ctx, ownerID, computerID, types.StackRebaseClaimKey(pair, attempt)); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return rebaseRetryResult{}, err
		}
		// No round ever claimed the latest attempt: it must be closed, and
		// the automatic attempts must be used up (or the owner's own).
		closed, _, err := rt.rebaseAttemptClosed(ctx, ownerID, computerID, *latest, rootInbox)
		if err != nil {
			return rebaseRetryResult{}, err
		}
		if !closed {
			return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_attempt_live", "attempt %d is still waiting for the root and management; ask again after its window passes", attempt)
		}
		if attempt < types.RebaseNoticeMaxAttempts {
			return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_attempts_remain", "attempt %d is closed but the computer's own attempts are not used up; the timer mints the next", attempt)
		}
	}
	next := attempt + 1
	if next <= types.RebaseNoticeMaxAttempts {
		next = types.RebaseNoticeMaxAttempts + 1
	}
	notice, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{
		TextureInbox: rootInbox, ComputerID: computerID, OperationID: operationID, BootedBase: booted, StackBase: stackBase,
		Attempt: next, DocID: docID, OwnerRetry: true,
	})
	if !ok {
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_unavailable", "the notice does not bind")
	}
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: notice.RecordID, Records: []types.CommitmentRecord{notice}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return rebaseRetryResult{}, err
	}
	req.CommandDigest = digest
	if _, err := rt.store.CommitRebaseOwnerNotice(ctx, req, operationID); err != nil {
		if errors.Is(err, store.ErrRebaseRoundRunning) {
			return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_round_running", "%v", err)
		}
		return rebaseRetryResult{}, refuseRebaseRetry("rebase_retry_unreadable", "commit %s: %v", notice.RecordID, err)
	}
	return rebaseRetryResult{Minted: notice.RecordID, Attempt: next}, nil
}

// retryRebaseOwnerAct is the HTTP form: an owner act on this computer.
func (h *APIHandler) retryRebaseOwnerAct(w http.ResponseWriter, r *http.Request, ownerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	res, err := h.rt.retryRebaseNotice(r.Context(), ownerID)
	var refusal *rebaseRetryRefusal
	switch {
	case errors.As(err, &refusal):
		writeAPIJSON(w, http.StatusConflict, apiError{Error: refusal.Error(), Reason: refusal.Code})
	case err != nil:
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
	default:
		writeAPIJSON(w, http.StatusOK, map[string]any{"notice_id": res.Minted, "attempt": res.Attempt})
	}
}
