package agentcore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The round receipt: one ledger record per self-development round, written
// when the round ends (apply, reject, fail, abandon), with the minutes per
// phase and the platform's share
// (docs/problems/selfdev-round-overhead-unmeasured-2026-11-04.md,
// types.BuildRoundReceipt). Every phase is derived from a record that
// already exists:
//
//   - Texture's Cast and management's Cast to engineering: their commit times;
//   - the bind: the assignment's grant attestation;
//   - the spawn and the minutes waited: the wait-summary record the
//     assignment's end put on the ledger (engineering_waits.go);
//   - build, choir-shot and other jobs: the job end records in the
//     engineer's inbox (start and finish);
//   - the Freeze: the commit time of the successful Freeze that settled the
//     round's freeze_minutes prediction (its resolve record's freeze_at), else
//     the operation's creation;
//   - the decision: the approve owner-act record, else the act itself.
//
// The record is addressed to management, so it is in management's acting
// pack; Texture's owner-act record for the act carries its one line. It is
// observational: a failure is logged and never fails the act.

func parseRoundTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

func recordTime(rec *types.CommitmentRecord) time.Time {
	if rec == nil {
		return time.Time{}
	}
	return parseRoundTime(rec.Provenance.CommittedAt)
}

// roundEndAct reports whether act ends a round.
func roundEndAct(act string) bool {
	switch act {
	case types.OwnerActApply, types.OwnerActReject, types.OwnerActFail, types.OwnerActAbandon:
		return true
	}
	return false
}

// recordRoundReceipt commits the operation's round receipt for act, once
// (the record id is the operation's). It returns the record, committed now
// or already there, or nil when the operation is not an engineering round
// or a source record is missing.
func (rt *Runtime) recordRoundReceipt(ctx context.Context, ownerID string, operation selfdev.Operation, act string) *types.CommitmentRecord {
	if rt == nil || rt.store == nil || !roundEndAct(act) {
		return nil
	}
	assignmentID, ok := store.EngineeringFreezeAssignmentOf(operation.IdempotencyKey)
	if !ok || assignmentID == "" {
		return nil
	}
	ownerID = firstNonEmpty(strings.TrimSpace(ownerID), strings.TrimSpace(rt.selfdevRouteOwnerID))
	computerID := firstNonEmpty(strings.TrimSpace(rt.TextureComputerID()), strings.TrimSpace(operation.ComputerID))
	if ownerID == "" || computerID == "" {
		return nil
	}
	ctx = context.WithoutCancel(ctx)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	recordID := types.RoundReceiptRecordID(operation.OperationID)
	if existing, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, recordID); err == nil && existing != nil {
		return existing
	}
	inbox := agentprofile.Engineering + ":" + assignmentID
	assignment, err := rt.store.EngineeringAssignmentForInbox(ctx, ownerID, computerID, inbox)
	if err != nil {
		log.Printf("selfdev round receipt: %s: assignment %s: %v", operation.OperationID, assignmentID, err)
		return nil
	}
	binding := assignment.Binding
	now := rt.assignmentWaitNow()
	in := types.RoundInputs{Outcome: act, EndAt: now, FreezeAt: parseRoundTime(operation.CreatedAt)}
	castRec, _ := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, binding.ParentControlID)
	// The Freeze is the fresh, successful one the freeze_minutes prediction
	// settled at (engineering_round_predictions.go), not the operation's
	// creation, which is the first Freeze attempt: a refused Freeze or a
	// re-Freeze after a rejection moves it. A round without that settlement
	// (an old Cast, a censored claim) keeps the operation's time.
	if resolve, _ := rt.store.GetCommitmentRecord(ctx, ownerID, computerID,
		types.RuleResolveRecordID(roundPredictionRecordID(binding.ParentControlID, types.PrecommitRuleFreezeMinutes))); resolve != nil {
		if at := parseRoundTime(resolve.SubjectRefs[types.RuleRefFreezeAt]); !at.IsZero() {
			in.FreezeAt = at
		}
	}
	in.CastAt = firstTime(recordTime(castRec), assignment.CreatedAt)
	if castRec != nil && strings.TrimSpace(castRec.ParentID) != "" {
		textureRec, _ := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, castRec.ParentID)
		in.TextureCastAt = recordTime(textureRec)
	}
	in.BindAt = assignment.CreatedAt
	if att := assignment.GrantPolicyAttestation; att != nil && !att.RecordedAt.IsZero() {
		in.BindAt = att.RecordedAt
	}
	in.EngineerEndAt = assignment.UpdatedAt
	if assignment.TerminalAt != nil {
		in.EngineerEndAt = *assignment.TerminalAt
	}
	// The decision: the approve act's record when there was one (an apply or
	// a failed apply follows it); otherwise the act itself.
	if act == types.OwnerActApply || act == types.OwnerActFail {
		approve, _ := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.OwnerActRecordID(operation.OperationID, types.OwnerActApprove))
		in.DecisionAt = recordTime(approve)
	}
	if summary, _ := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, assignmentWaitSummaryRecordID(assignment.AssignmentID, binding.Attempt)); summary != nil {
		in.WaitedS = types.RoundWaitedFromRefs(summary.SubjectRefs)
		in.SpawnS = types.RoundRefInt(summary.SubjectRefs, types.RoundRefSpawnS)
		in.JobWaitedS = types.RoundRefInt(summary.SubjectRefs, types.RoundRefJobWaitedS)
		for _, s := range in.WaitedS {
			in.AdmissionWaitedS += s
		}
		in.AdmissionWaitedS = max(in.AdmissionWaitedS-in.JobWaitedS, 0)
	}
	if records, err := rt.store.ListInboxRecords(ctx, ownerID, computerID, inbox, 0, 0); err == nil {
		for _, rec := range records {
			if rec.Job == nil {
				continue
			}
			in.Jobs = append(in.Jobs, types.RoundJob{Kind: types.RoundJobKind(rec.Job.Argv),
				Start: parseRoundTime(rec.Job.StartedAt), End: parseRoundTime(rec.Job.FinishedAt)})
		}
	}
	receipt := types.BuildRoundReceipt(in)
	stamp := now.UTC().Format(time.RFC3339Nano)
	summary := receipt.Summary()
	refs := receipt.Refs()
	// The round's governing predictions (its Cast's), stated against what
	// happened (calibration_claims_api.go).
	preds := rt.roundPredictions(ctx, ownerID, computerID, assignment.AssignmentID, binding.ParentControlID)
	types.RoundPredictionRefs(refs, preds)
	if line := types.RoundPredictionsSummary(preds); line != "" {
		summary += " " + line
	}
	refs["operation_id"], refs["assignment_id"] = operation.OperationID, assignment.AssignmentID
	rec := types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    recordID,
		Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{
			Hypothesis:  fmt.Sprintf("self-development round %s (assignment %s): %s", operation.OperationID, assignment.AssignmentID, summary),
			CommittedAt: stamp,
		},
		Observation: types.CommitmentObservation{Excerpt: summary, SourceRef: operation.OperationID, ObservedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: engineeringHoldObserverAgentID, ContextRef: binding.TrajectoryID, CommittedAt: stamp},
		Addressee:   persistentManagementAgentID(ownerID),
		SubjectRefs: refs, EvidenceRefs: []string{operation.OperationID, assignment.AssignmentID},
	}
	if _, err := rt.store.AppendCommitmentRecord(ctx, ownerID, computerID, rec); err != nil {
		log.Printf("selfdev round receipt: %s: %v", operation.OperationID, err)
		return nil
	}
	return &rec
}

func firstTime(times ...time.Time) time.Time {
	for _, t := range times {
		if !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

// roundLineSentence is the receipt's one line for Texture's owner-act
// record, or "" when the act ends no round or the receipt is not on the
// ledger. It reads the record, so a replay of the act commits the same text.
func (rt *Runtime) roundLineSentence(ctx context.Context, ownerID, computerID string, operation selfdev.Operation, act string) string {
	if rt == nil || rt.store == nil || !roundEndAct(act) {
		return ""
	}
	ctx = context.WithoutCancel(ctx)
	rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.RoundReceiptRecordID(operation.OperationID))
	if err != nil || rec == nil {
		return ""
	}
	if line := types.RoundLineFromRefs(rec.SubjectRefs); line != "" {
		return "\n\n" + line + "."
	}
	return ""
}

// roundPredictions are the round's Cast predictions with their settlements.
func (rt *Runtime) roundPredictions(ctx context.Context, ownerID, computerID, assignmentID, castID string) []types.RoundPrediction {
	claims, err := rt.calibrationClaims(ctx, ownerID, computerID, assignmentID)
	if err != nil {
		log.Printf("selfdev round receipt: assignment %s predictions: %v", assignmentID, err)
		return nil
	}
	var out []types.RoundPrediction
	for _, c := range claims {
		if c.StatedBy != "cast" || c.Cast != strings.TrimSpace(castID) {
			continue
		}
		p := types.RoundPrediction{Kind: c.Kind, P: c.P, Estimate: c.Estimate, Low: c.Low, High: c.High, Answer: c.Answer, Censored: c.Censored}
		if c.ActualS != nil {
			p.ActualS, p.HasActual = *c.ActualS, true
		}
		out = append(out, p)
	}
	return out
}
