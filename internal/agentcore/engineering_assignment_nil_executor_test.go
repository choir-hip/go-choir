package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Regression: assignedCapsule must return a nil INTERFACE (not a boxed
// typed-nil *capsule.Executor) when no executor is configured. The typed-nil
// box defeats every `exec == nil` guard downstream and previously caused a
// SIGSEGV in assignedEngineeringCapsuleUsable at the post-replay reconcile
// (evidence: recovery-post-replay-cosuper-fate-nil-executor-panic-2026-08-24.md).
func TestAssignedCapsuleWithoutExecutorIsNilInterface(t *testing.T) {
	rt := &Runtime{} // no capsuleExecutor, no assignmentRuntime
	if got := rt.assignedCapsule(); got != nil {
		t.Fatalf("assignedCapsule with no executor must be a nil interface, got %#v", got)
	}
}

// Regression: with a bound Engineering assignment and NO capsule executor,
// assignedEngineeringCapsuleUsable must report unusable — never dereference a
// nil *capsule.Executor.
func TestAssignedEngineeringCapsuleUsableWithoutExecutorDoesNotPanic(t *testing.T) {
	rt := &Runtime{}
	assignment := types.EngineeringAssignment{
		BoundRunID: "run-bound",
		Binding:    types.EngineeringAssignmentBinding{CapsuleID: "capsule-bound"},
	}
	if rt.assignedEngineeringCapsuleUsable(assignment) {
		t.Fatal("no capsule executor must never report a usable assignment capsule")
	}
}
