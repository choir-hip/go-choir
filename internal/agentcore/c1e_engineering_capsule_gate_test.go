package agentcore

// C1e B1 follow-up: the capsule gate on records assignments
// (docs/archive/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md).
//
// The drill on staging e9d727a7: management casts engineering on records,
// the engineering run starts, and every capsule_go_eval cell is refused
// with "assigned capsule obligation is no longer executable: assigned work
// obligation is not open: record not found". The completion guard then
// fails the run and its failure path mints the cancel-report. The gate
// (assignedEngineeringObligation, the store half of validateAssignedEngineeringExecution) still read the work item a records
// binding no longer has.
//
// Ways the gate could fail, written before the fix:
//
//  1. A fresh records assignment (bound, capsule active, run pending, its
//     engineering round open) is refused because the gate reads a work item
//     the binding does not carry. This is the trace.
//     -> TestC1eCapsuleGateAdmitsAFreshRecordsAssignment
//  2. The fix admits too much: a records assignment that has ended (row
//     terminal, engineering round closed by its final Report) still passes.
//     -> TestC1eCapsuleGateRefusesAnEndedRecordsAssignment
//  3. The fix admits a run that is not the round's recipient: the gate must
//     still bind the open round to the run's own engineering agent.
//     -> TestC1eCapsuleGateRefusesARunThatIsNotTheRoundsRecipient
//  4. The obligation half passes but the capsule half is skipped: with no
//     live capsule the full validator must still refuse, on the capsule,
//     never on a work item.
//     -> TestC1eCapsuleGateAdmitsAFreshRecordsAssignment (second half)
//  5. A pre-records binding loses its work-item check. Pre-records bindings
//     cannot be bound any more (BindEngineeringAssignment refuses them), so
//     the legacy branch is kept unchanged rather than tested here.

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
)

func TestC1eCapsuleGateAdmitsAFreshRecordsAssignment(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-gate-fresh", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	if !assignment.Binding.Records() || assignment.Binding.AssignedWorkItemID != "" {
		t.Fatalf("seed is not a records assignment: %+v", assignment.Binding)
	}
	if !c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, assignment.Binding.AssignedConversationID) {
		t.Fatal("the Cast did not open the engineering round")
	}
	if _, err := rt.assignedEngineeringObligation(ctx, &run); err != nil {
		t.Fatalf("a fresh records assignment is refused: %v", err)
	}
	// The full gate on a computer without the assignment's capsule refuses
	// on the capsule, never on a work item.
	err = rt.validateAssignedEngineeringExecution(ctx, &run)
	if err == nil {
		t.Fatal("the gate admitted an assignment whose capsule is absent")
	}
	if strings.Contains(err.Error(), "work obligation") || !strings.Contains(err.Error(), "capsule") {
		t.Fatalf("gate refusal = %v; want a capsule refusal", err)
	}
}

func TestC1eCapsuleGateRefusesAnEndedRecordsAssignment(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-gate-ended", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	c1eCancel(t, rt, assignment, "management cancelled", false)
	if c1eRoundOpen(t, s, seed.OwnerID, seed.ComputerID, assignment.Binding.AssignedConversationID) {
		t.Fatal("the cancel's final Report did not close the engineering round")
	}
	if _, err := rt.assignedEngineeringObligation(ctx, &run); err == nil {
		t.Fatal("the gate admitted a cancelled records assignment")
	}
}

func TestC1eCapsuleGateRefusesARunThatIsNotTheRoundsRecipient(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-c1e-gate-recipient", rt.TextureComputerID(), 2)
	if err != nil {
		t.Fatal(err)
	}
	_, first := c1eOpenBound(t, s, seed, 0)
	second, _ := c1eOpenBound(t, s, seed, 1)
	// The first assignment's run presented under the second's agent.
	stray := first
	stray.AgentID = second.Binding.AssignedAgentID
	_, err = rt.assignedEngineeringObligation(ctx, &stray)
	if err == nil || !strings.Contains(err.Error(), "engineering round is not open") {
		t.Fatalf("gate on a run that is not its round's recipient = %v; want the round refusal", err)
	}
}
