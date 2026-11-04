package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Round predictions (docs/problems/selfdev-rounds-record-no-predictions-
// 2026-11-04.md; spec docs/design/calibration-seed-2026-11-03.md, lane F).
//
// Every engineering round states, in management's Cast, its verify_pass,
// judge_accepts and freeze_minutes predictions, before the capsule starts.
// A Cast without them is refused with the shape to state them in
// (refuseCastWithoutPredictions). The Cast record is the durable statement:
// the rule precommit records are derived from it, with deterministic ids,
// once the open commits and again before every settlement pass, so a crash
// between the open and the derivation loses nothing. The runtime settles
// them from events it records (types/commitment_rules.go, R13-R17).
//
// Failure modes, written before the code:
//
//	P1 A Cast states predictions in a shape the runtime drops (a typo, a
//	   missing claim), so the round predicts nothing.
//	   -> strict decode; unknown keys and missing claims refuse the Cast.
//	P2 The records are written after the open and lost on a crash.
//	   -> derived from the committed Cast record, idempotently, before
//	      every settlement pass as well as after the open.
//	P3 The engineer reads management's predictions and works to them.
//	   -> the engineer's view of its Cast drops the predictions block
//	      (requestIntoBody).
//	P4 The clock anchor is the stater's choice.
//	   -> Since is the Cast record's committed_at, stamped by the runtime,
//	      the same field the round's phase receipt reads.
//	P5 A Cast from before this landing has no predictions, and its
//	   settlement fails on them.
//	   -> no block, no records: old rounds settle as before.
//
// Who states them: management, in the Cast. The records name management's
// agent and model as their author, and the assignment as their subject;
// the calibration digest counts them on the round's desk, engineering
// (calibrationThreadOfRecord), as the spec's "engineering digest".

// refuseCastWithoutPredictions refuses an engineering Cast whose spec does
// not state the round's predictions, telling management how to.
func refuseCastWithoutPredictions(raw json.RawMessage) error {
	if _, err := types.ParseRoundPredictions(raw); err != nil {
		return fmt.Errorf("engineering cast: every round states its predictions before the capsule starts (%v). Add to the spec %s. "+
			"Each p is your chance, from 0.02 to 0.98, that the claim holds: verify_pass that your Verify passes the candidate; judge_accepts that the independent judgment accepts it, so it reaches the owner's decision; "+
			"freeze_minutes is your estimate in whole minutes from this Cast to the engineer's Freeze, a low-high interval around it, and your chance that the Freeze lands inside the interval. "+
			"The runtime settles each from what happens and scores the estimate against the actual minutes, so state what you expect, not a safe wide window", err, types.RoundPredictionsHow)
	}
	return nil
}

// roundPredictionRecordID is one prediction's record id: the Cast's, the
// kind, so a re-derivation writes the same record.
func roundPredictionRecordID(castRecordID string, kind types.PrecommitRuleKind) string {
	return strings.TrimSpace(castRecordID) + ":predict:" + string(kind)
}

// castPredictionsBlock reads the predictions block of a Cast record's spec;
// nil when it states none (P5).
func castPredictionsBlock(cast types.CommitmentRecord) json.RawMessage {
	if cast.Directive == nil {
		return nil
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(cast.Directive.Body)), &fields); err != nil {
		return nil
	}
	return fields[types.RoundPredictionsField]
}

// roundPredictionRecords derives the round's rule precommit records from
// the Cast record that opened the assignment.
func roundPredictionRecords(cast types.CommitmentRecord, assignment types.EngineeringAssignment) ([]types.CommitmentRecord, error) {
	raw := castPredictionsBlock(cast)
	if raw == nil {
		return nil, nil
	}
	predictions, err := types.ParseRoundPredictions(raw)
	if err != nil {
		return nil, err
	}
	since := strings.TrimSpace(cast.Provenance.CommittedAt)
	if _, ok := parseCommitmentStamp(since); !ok {
		since = assignment.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	var out []types.CommitmentRecord
	for _, precommit := range predictions.Precommits() {
		rule := *precommit.Rule
		rule.AssignmentID = assignment.AssignmentID
		rule.Since = since
		precommit.Rule = &rule
		out = append(out, types.CommitmentRecord{
			SchemaID:    types.CommitmentRecordSchemaV1,
			RecordID:    roundPredictionRecordID(cast.RecordID, rule.Kind),
			Kind:        types.CommitmentKindPrecommit,
			Discrepancy: types.DiscrepancyUnresolved,
			Precommit:   &precommit,
			Prediction: types.CommitmentPrediction{
				Hypothesis: precommit.Question,
				Questions: []types.TypedQuestion{{Question: precommit.Question,
					Answers: []string{types.PrecommitAnswerYes, types.PrecommitAnswerNo}, Probabilities: precommit.Distribution}},
				CommittedAt: since,
			},
			Provenance: types.CommitmentProvenance{
				AgentID: cast.Provenance.AgentID, ModelID: cast.Provenance.ModelID,
				CommittedAt: since, ContextRef: cast.Provenance.ContextRef,
			},
			RelatedIDs:  []string{cast.RecordID},
			EmitterKind: types.CommitmentEmitterAgent,
			ComputerID:  assignment.Binding.ComputerID,
			SubjectRefs: map[string]string{"assignment": assignment.AssignmentID, "cast": cast.RecordID, "stated_at": "cast"},
		})
	}
	return out, nil
}

// parseCommitmentStamp parses a record's RFC 3339 stamp.
func parseCommitmentStamp(raw string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	return t, err == nil
}

// stateRoundPredictions writes the round's prediction records, deriving
// them from its Cast record (P2). Idempotent: each id is written once.
func (rt *Runtime) stateRoundPredictions(ctx context.Context, assignment types.EngineeringAssignment) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("round predictions: store unavailable")
	}
	b := assignment.Binding
	castID := strings.TrimSpace(b.ParentControlID)
	if castID == "" || b.Kind != types.EngineeringAssignmentImplementation {
		return nil
	}
	cast, err := rt.store.GetCommitmentRecord(ctx, b.OwnerID, b.ComputerID, castID)
	if err != nil {
		return fmt.Errorf("round predictions: cast %s: %w", castID, err)
	}
	if cast == nil {
		return nil
	}
	records, err := roundPredictionRecords(*cast, assignment)
	if err != nil {
		return fmt.Errorf("round predictions: cast %s: %w", castID, err)
	}
	for _, rec := range records {
		exists, err := rt.store.CommitmentRecordExists(ctx, b.OwnerID, b.ComputerID, rec.RecordID)
		if err != nil {
			return fmt.Errorf("round predictions: %s: %w", rec.RecordID, err)
		}
		if exists {
			continue
		}
		if _, err := rt.store.AppendCommitmentRecord(ctx, b.OwnerID, b.ComputerID, rec); err != nil {
			return fmt.Errorf("round predictions: %s: %w", rec.RecordID, err)
		}
	}
	return nil
}

// ensureRoundPredictionsFor states an assignment's round predictions
// before a settlement pass, best effort (R6): it logs, never fails the act.
func (rt *Runtime) ensureRoundPredictionsFor(ctx context.Context, ownerID, computerID, assignmentID string) {
	if rt == nil || rt.store == nil || strings.TrimSpace(assignmentID) == "" {
		return
	}
	assignment, err := rt.store.EngineeringAssignmentForInbox(ctx, ownerID, computerID, agentprofile.Engineering+":"+strings.TrimSpace(assignmentID))
	if err != nil {
		log.Printf("runtime: assignment %s round predictions: %v", assignmentID, err)
		return
	}
	if err := rt.stateRoundPredictions(ctx, assignment); err != nil {
		log.Printf("runtime: assignment %s %v", assignmentID, err)
	}
}
