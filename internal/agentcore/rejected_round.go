package agentcore

import (
	"context"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Rejected rounds are sent back, not completed (owner, 2026-11-04:
// "Rejected rounds don't complete but get sent back";
// docs/problems/texture-settles-rejected-round-as-complete-2026-11-04.md).
//
// The send-back loop already exists inside one engineering assignment
// (engineer_reuse.go): a rejected candidate resumes the live engineer with
// the reasons, up to store.EngineeringFreezeMaxRevisions candidates. What was
// missing is the end: when the last candidate is rejected the engineer's
// Complete commits, the round reads as complete, and Texture's Done settled
// it. A round's attempt budget is that same constant; the round whose last
// candidate was rejected is "rejected after N attempts", never complete.
// A crash close and an owner abandon fail an operation with no verdict (no
// verifier refs, not the judgment's words), so neither reads as a rejection.

// RejectedRound is a trajectory whose last candidate was rejected and that
// nothing is still working on.
type RejectedRound struct {
	OperationID string
	// Attempts counts the trailing rejected candidates; Budget bounds them.
	Attempts, Budget int
	// Reasons are the last rejection's reasons.
	Reasons string
	// Exhausted: the budget is spent, so the round ends rejected.
	Exhausted bool
}

// Fate is the visible fate of an exhausted round.
func (r RejectedRound) Fate() string {
	return fmt.Sprintf("rejected after %d attempts: %s", r.Attempts, r.Reasons)
}

// SendBackInstruction is what Texture reads when its Done on a rejected
// round is refused with attempts left.
func (r RejectedRound) SendBackInstruction() string {
	return fmt.Sprintf("candidate %s was rejected (attempt %d of %d): %s. A rejected round is sent back, not completed: Cast engineering again with these reasons so it makes the next attempt, or Wait(why) if the owner must answer first. Nothing was committed",
		r.OperationID, r.Attempts, r.Budget, r.Reasons)
}

func candidateRejected(op selfdev.Operation) bool {
	return judgmentRejected(op) || op.State == selfdev.StateRejected
}

// rejectedRoundFrom reads a trajectory's operations (oldest first).
func rejectedRoundFrom(ops []selfdev.Operation) (RejectedRound, bool) {
	if _, live := pickLiveTrajectoryOperation(ops); live {
		return RejectedRound{}, false
	}
	var candidates []selfdev.Operation
	for _, op := range ops {
		if strings.TrimSpace(op.BundleDigest) != "" {
			candidates = append(candidates, op)
		}
	}
	if len(candidates) == 0 || !candidateRejected(candidates[len(candidates)-1]) {
		return RejectedRound{}, false
	}
	last := candidates[len(candidates)-1]
	attempts := 0
	for i := len(candidates) - 1; i >= 0 && candidateRejected(candidates[i]); i-- {
		attempts++
	}
	reasons := ""
	if last.State == selfdev.StateRejected {
		who := "the owner"
		if actor := strings.TrimSpace(last.DecisionActor); actor != "" && actor != "owner" {
			who = "a consensus decision (" + actor + ")"
		}
		reasons = "rejected by " + who + "; their note is in the owner-act record " + types.OwnerActRecordID(last.OperationID, types.OwnerActReject)
	} else {
		reasons = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.TrimPrefix(last.TerminalError, selfDevelopmentJudgmentRejected)), ":"))
		if reasons == "" {
			reasons = "the judge gave no reason"
		}
	}
	return RejectedRound{OperationID: last.OperationID, Attempts: attempts, Budget: store.EngineeringFreezeMaxRevisions,
		Reasons: reasons, Exhausted: attempts >= store.EngineeringFreezeMaxRevisions}, true
}

// RejectedRoundOf reads the trajectory's operations. An owner's abandon of
// the last candidate ends the round by the owner's act, so it is no
// rejection to send back.
func (rt *Runtime) RejectedRoundOf(ctx context.Context, ownerID, computerID, trajectoryID string) (RejectedRound, bool) {
	if rt == nil || rt.selfdevOperations == nil || strings.TrimSpace(computerID) == "" || strings.TrimSpace(trajectoryID) == "" {
		return RejectedRound{}, false
	}
	ops, err := rt.selfdevOperations.ListByTrajectory(ctx, computerID, trajectoryID)
	if err != nil {
		return RejectedRound{}, false
	}
	round, ok := rejectedRoundFrom(ops)
	if !ok {
		return RejectedRound{}, false
	}
	if rt.store != nil {
		if rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.OwnerActRecordID(round.OperationID, types.OwnerActAbandon)); err == nil && rec != nil {
			return RejectedRound{}, false
		}
	}
	return round, true
}
