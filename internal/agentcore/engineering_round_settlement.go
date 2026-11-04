package agentcore

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Settlement past the judgment (types/commitment_rules.go, R18, R19):
// owner_approves and approval_minutes settle on the owner's act on the
// candidate, and approved_within_rounds on the request's Freezes and its
// approval, across every round cast for the same trajectory. Both run where
// every owner act passes (notifyTextureOfOwnerAct), best effort (R6).
//
// Failure modes, written before the code:
//
//	S1 A consensus or platform decision is counted as the owner's own.
//	   -> the settlement names the decision actor and marks it delegated
//	      when the actor is not the computer's owner.
//	S2 The approval time disagrees with the round receipt.
//	   -> the approve act's record time when there is one, else the act's,
//	      as recordRoundReceipt reads it.
//	S3 approved_within_rounds stays open forever on a request that ends.
//	   -> an abandon act settles it "no"; a request left without an act
//	      stays open (named residual).
//	S4 A Freeze refused and retried counts twice.
//	   -> Freezes are the trajectory's operations that froze a bundle, one
//	      per operation (a reused engineer's revision is its own operation).

// ownerActAssignmentID is the assignment an engineering operation belongs
// to (its freeze-open idempotency key), "" for any other operation.
func ownerActAssignmentID(operation selfdev.Operation) string {
	id, ok := strings.CutPrefix(strings.TrimSpace(operation.IdempotencyKey), store.EngineeringFreezeIdempotencyKey(""))
	if !ok {
		return ""
	}
	return strings.TrimSpace(id)
}

// ownerActUnknownActor marks a decision whose actor the operation does not
// name: it is never read as the owner's own (S1).
const ownerActUnknownActor = "unknown"

// settleOwnerActPrecommits settles what an owner act on an engineering
// candidate decides.
func (rt *Runtime) settleOwnerActPrecommits(ctx context.Context, ownerID string, operation selfdev.Operation, act string) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID = firstNonEmpty(strings.TrimSpace(ownerID), strings.TrimSpace(rt.selfdevRouteOwnerID))
	// The records' computer, as the round receipt reads it.
	computerID := firstNonEmpty(strings.TrimSpace(rt.TextureComputerID()), strings.TrimSpace(operation.ComputerID))
	if ownerID == "" || computerID == "" {
		return
	}
	ctx = context.WithoutCancel(ctx)
	// The request's claims settle whatever operation the act names (S3).
	defer rt.settleRequestPrecommits(ctx, ownerID, computerID, operation.TrajectoryID, act == types.OwnerActAbandon)
	assignmentID := ownerActAssignmentID(operation)
	if assignmentID == "" {
		return
	}
	approved := rt.operationApproved(ctx, ownerID, computerID, operation)
	if act == types.OwnerActFail && !approved {
		return // a failure that no approval preceded decides nothing here
	}
	now := time.Now().UTC()
	decisionAt := now
	switch act {
	case types.OwnerActApprove, types.OwnerActApply, types.OwnerActFail:
		// S2: the approve act's record time, as the round receipt reads it.
		if t := rt.ownerActApprovedAt(ctx, ownerID, computerID, operation.OperationID); !t.IsZero() {
			decisionAt = t
		}
	}
	actor := strings.TrimSpace(operation.DecisionActor)
	if actor == "" {
		actor = ownerActUnknownActor
	}
	obs := types.EngineeringOwnerActObservation{Act: act, OperationID: operation.OperationID, DecisionAt: decisionAt,
		Actor: actor, Delegated: actor != ownerID}
	rt.settleRulePrecommits(ctx, ownerID, computerID, assignmentID, "owner act "+act, now, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnOwnerAct(rule, obs)
	})
}

// ownerActApprovedAt is the approve act record's time, zero when none.
func (rt *Runtime) ownerActApprovedAt(ctx context.Context, ownerID, computerID, operationID string) time.Time {
	approve, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.OwnerActRecordID(operationID, types.OwnerActApprove))
	if err != nil || approve == nil {
		return time.Time{}
	}
	return parseRoundTime(approve.Provenance.CommittedAt)
}

// operationApproved reports an operation an approval decided: its approve
// act's record exists, or its state is one only an approval reaches. An
// abandon or a reject also records a decision, so a decision alone is not
// an approval (review r3).
func (rt *Runtime) operationApproved(ctx context.Context, ownerID, computerID string, op selfdev.Operation) bool {
	switch op.State {
	case selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateApplied, selfdev.StateRollbackPending, selfdev.StateRolledBack:
		return true
	case selfdev.StateRejected:
		return false
	}
	return !rt.ownerActApprovedAt(ctx, ownerID, computerID, op.OperationID).IsZero()
}

// settleRequestPrecommits settles every open approved_within_rounds claim
// of the trajectory's rounds (R19).
func (rt *Runtime) settleRequestPrecommits(ctx context.Context, ownerID, computerID, trajectoryID string, abandoned bool) {
	trajectoryID = strings.TrimSpace(trajectoryID)
	if rt == nil || rt.store == nil || rt.selfdevOperations == nil || trajectoryID == "" {
		return
	}
	ops, err := rt.selfdevOperations.ListByTrajectory(ctx, computerID, trajectoryID)
	if err != nil {
		log.Printf("runtime: trajectory %s approved_within_rounds: %v", trajectoryID, err)
		return
	}
	frozen := make([]selfdev.Operation, 0, len(ops))
	for _, op := range ops {
		if strings.TrimSpace(op.BundleDigest) != "" {
			frozen = append(frozen, op)
		}
	}
	sort.SliceStable(frozen, func(i, j int) bool {
		return parseRoundTime(frozen[i].CreatedAt).Before(parseRoundTime(frozen[j].CreatedAt))
	})
	assignments, err := rt.store.ListEngineeringAssignments(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		log.Printf("runtime: trajectory %s approved_within_rounds: %v", trajectoryID, err)
		return
	}
	now := time.Now().UTC()
	for _, assignment := range assignments {
		rt.settleRulePrecommits(ctx, ownerID, computerID, assignment.AssignmentID, "request", now, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
			if rule.Kind != types.PrecommitRuleApprovedWithinRounds {
				return types.RuleSettlement{}, false
			}
			since := parseRoundTime(rule.Since)
			obs := types.EngineeringRequestObservation{At: now, Abandoned: abandoned}
			for _, op := range frozen {
				// A Freeze whose time cannot be read counts (never undercounts).
				if created := parseRoundTime(op.CreatedAt); !since.IsZero() && !created.IsZero() && created.Before(since) {
					continue
				}
				obs.FreezesSince++
				if !obs.Approved && rt.operationApproved(ctx, ownerID, computerID, op) {
					obs.Approved, obs.FreezesToApproval, obs.ApprovedOperationID = true, obs.FreezesSince, op.OperationID
				}
			}
			return types.SettleRuleOnRequest(rule, obs)
		})
	}
}
