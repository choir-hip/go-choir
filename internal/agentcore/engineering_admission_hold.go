package agentcore

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// engineeringHoldRecordAfter is the consecutive operation_open deferrals of
// one assignment after which the hold is recorded on the ledger.
const engineeringHoldRecordAfter = 3

// engineeringHoldObserverAgentID is the committing identity of the hold
// record: a system-committed observation, like the reconciler's.
const engineeringHoldObserverAgentID = "engineering-admission"

// engineeringHoldRecordID is the deterministic id of the one hold record of
// one assignment behind one operation, so a later deferral records nothing.
// The state is part of the id: the same operation blocking from a new state is
// a new fact.
func engineeringHoldRecordID(operationID, state, assignmentID string) string {
	return fmt.Sprintf("selfdev-observation:%s:blocks-engineering:%s:%s", operationID, state, assignmentID)
}

// engineeringHoldSettlingActs says which owner acts settle an operation in
// the given state.
func engineeringHoldSettlingActs(operationID, state string) string {
	abandon := fmt.Sprintf("`choir self-dev abandon --operation %s --reason ...`", operationID)
	reject := fmt.Sprintf("`choir self-dev reject --operation %s --reason ...`", operationID)
	switch state {
	case "awaiting_approval":
		return "It waits for the owner's decision; the owner acts that settle it are " + reject + " (withdraw it) and " + abandon + " (close it as failed)."
	case "degraded":
		return "It was applied with an unsettled tail the materializer could not finish; the installed release stays live. The owner act that closes it is " + abandon + "."
	case "materializing", "rollback_pending":
		return "It is in flight with the updater and settles by itself or when its pinned release is found unusable; if it is parked, " + abandon + " settles it when the updater journal shows nothing applied."
	}
	return "It installed nothing; the owner act that closes it is " + abandon + "."
}

// engineeringHoldRecord is the ledger record of an engineering request held by
// an open self-development operation: the blocker, its state and age, and the
// owner acts that settle it. It is the shape of the decision-boundary
// observation (notifySelfDevelopmentDecisionBoundary), addressed to the
// computer's management desk, whose acting pack and Texture's supervision
// surface render the ledger.
func engineeringHoldRecord(assignment types.EngineeringAssignment, busy *engineeringAdmissionBusy, now time.Time) types.CommitmentRecord {
	age := "unknown"
	if created, err := time.Parse(time.RFC3339Nano, busy.OperationCreatedAt); err == nil {
		age = now.Sub(created).Round(time.Minute).String()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	return types.CommitmentRecord{
		SchemaID:    types.CommitmentRecordSchemaV1,
		RecordID:    engineeringHoldRecordID(busy.OperationID, busy.OperationState, assignment.AssignmentID),
		Discrepancy: types.DiscrepancyUnresolved,
		Prediction: types.CommitmentPrediction{
			Hypothesis: fmt.Sprintf("engineering assignment %s cannot start: self-development operation %s is %s (age %s) and every engineering admission on this computer waits for it. %s",
				assignment.AssignmentID, busy.OperationID, busy.OperationState, age, engineeringHoldSettlingActs(busy.OperationID, busy.OperationState)),
			CommittedAt: stamp,
		},
		Observation: types.CommitmentObservation{
			Excerpt:    fmt.Sprintf("admission of %s deferred %d times behind operation %s (%s, age %s); retries continue every minute", assignment.AssignmentID, engineeringHoldRecordAfter, busy.OperationID, busy.OperationState, age),
			SourceRef:  busy.OperationID,
			ObservedAt: stamp,
		},
		Provenance: types.CommitmentProvenance{
			AgentID:     engineeringHoldObserverAgentID,
			ContextRef:  assignment.Binding.TrajectoryID,
			CommittedAt: stamp,
		},
		Addressee:    persistentManagementAgentID(assignment.Binding.OwnerID),
		EvidenceRefs: []string{busy.OperationID, assignment.AssignmentID},
	}
}

// recordEngineeringAdmissionHold writes the hold record. It is observational:
// a ledger failure is logged and the retry chain goes on.
func (rt *Runtime) recordEngineeringAdmissionHold(ctx context.Context, assignment types.EngineeringAssignment, busy *engineeringAdmissionBusy) {
	if rt == nil || rt.store == nil {
		return
	}
	rec := engineeringHoldRecord(assignment, busy, time.Now())
	if _, err := rt.store.AppendCommitmentRecord(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, rec); err != nil {
		log.Printf("runtime: engineering admission: hold record for %s behind %s: %v", assignment.AssignmentID, busy.OperationID, err)
	}
}
