package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Regression: assignedCapsule must return a nil INTERFACE (not a boxed
// typed-nil *capsule.Executor) when no executor is configured. The typed-nil
// box defeats every `exec == nil` guard downstream and previously caused a
// SIGSEGV in assignedCoSuperCapsuleUsable at the post-replay reconcile
// (evidence: recovery-post-replay-cosuper-fate-nil-executor-panic-2026-08-24.md).
func TestAssignedCapsuleWithoutExecutorIsNilInterface(t *testing.T) {
	rt := &Runtime{} // no capsuleExecutor, no assignmentRuntime
	if got := rt.assignedCapsule(); got != nil {
		t.Fatalf("assignedCapsule with no executor must be a nil interface, got %#v", got)
	}
}

// Regression: with a bound CoSuper assignment and NO capsule executor,
// assignedCoSuperCapsuleUsable must report unusable — never dereference a
// nil *capsule.Executor.
func TestAssignedCoSuperCapsuleUsableWithoutExecutorDoesNotPanic(t *testing.T) {
	rt := &Runtime{}
	assignment := types.CoSuperAssignment{
		BoundRunID: "run-bound",
		Binding:    types.CoSuperAssignmentBinding{CapsuleID: "capsule-bound"},
	}
	if rt.assignedCoSuperCapsuleUsable(assignment) {
		t.Fatal("no capsule executor must never report a usable assignment capsule")
	}
}
