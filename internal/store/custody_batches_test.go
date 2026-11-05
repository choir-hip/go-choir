package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// MA3.1 review B1: custody is per kind, the writer gate is per batch. The
// engineering-assignment lifecycle commits batches that mix the desks (open
// with a rebase claim, report with the assignment and a candidate, cancel with
// a report), so every kind that rides in them is the kernel's commons. This
// drives the real store batches through the enforced gate, each as the desk
// whose act it is, and fails if a batch comes to mix desk custodians.
//
// Residual: the test covers the lifecycle batches it drives. It does not
// enumerate every batch in the store; a batch of another flow that mixes desk
// custodians is found by the writer-gate counters on d@e (MA3.2), not here.
func TestAssignmentLifecycleBatchesPassTheEnforcedWriterGateAsEachDesk(t *testing.T) {
	a := newRebaseAdmission(t, 1)
	open := a.open
	// The fixture's own setup wrote without declaring a writer; the gate is
	// enforced only for the lifecycle's commands.
	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateEnforce)()
	objectgraph.ResetWriterGateCounts()
	as := func(w objectgraph.Writer) context.Context { return objectgraph.WithWriter(context.Background(), w) }

	// Open, with a rebase notice: management's act, writing engineering's claim.
	if _, err := a.s.OpenEngineeringAssignment(as(objectgraph.WriterManagement), open); err != nil {
		t.Fatalf("open with a rebase notice as management: %v", err)
	}
	if _, err := a.claim(t); err != nil {
		t.Fatalf("the open left no rebase claim: %v", err)
	}
	// Bind: the assignment with the run, capability and capsule claims.
	bind := bindEngineeringRequest(open, a.seed.AssignedRunIDs[0], "cap-"+a.seed.AssignmentIDs[0])
	if _, err := a.s.BindEngineeringAssignment(as(objectgraph.WriterEngineering), bind); err != nil {
		t.Fatalf("bind as engineering: %v", err)
	}
	// Report: engineering's run, writing the assignment and subject candidate.
	// A changed subject digest makes the report write the subject candidate too.
	report := assignmentReportRequest(open, 2, "report-custody", objectgraph.SHA256([]byte("changed-subject-custody")), types.EngineeringResultPartial, types.EngineeringVerdictNone)
	if _, err := a.s.RecordEngineeringAssignmentReport(as(objectgraph.WriterEngineering), report); err != nil {
		t.Fatalf("report as engineering: %v", err)
	}
	fate := types.SetEngineeringCapsuleDispositionRequest{
		CommandID: "command-fate-custody", OwnerID: a.seed.OwnerID, ComputerID: a.seed.ComputerID,
		AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 3,
		Disposition: types.EngineeringCapsuleFreezeRequested, IntentRef: "intent:fate-custody",
	}
	fate.CommandDigest, _ = ComputeSetEngineeringCapsuleDispositionDigest(fate)
	if _, err := a.s.SetEngineeringCapsuleDisposition(as(objectgraph.WriterEngineering), fate); err != nil {
		t.Fatalf("fate as engineering: %v", err)
	}
	// Cancel: management's tool, writing engineering's report.
	cancel := types.CancelEngineeringAssignmentRequest{
		CommandID: "command-cancel-custody", OwnerID: a.seed.OwnerID, ComputerID: a.seed.ComputerID,
		AssignmentID: open.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: 4, Reason: "custody test",
	}
	cancel.CommandDigest, _ = ComputeCancelEngineeringAssignmentDigest(cancel)
	if _, err := a.s.CancelEngineeringAssignment(as(objectgraph.WriterManagement), cancel); err != nil {
		t.Fatalf("cancel as management: %v", err)
	}
	if counts := objectgraph.WriterGateCounts(); len(counts) != 0 {
		t.Fatalf("the lifecycle batches recorded writer violations: %v", counts)
	}
}
