package agentcore

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Only a report that claims failure is relieved of certifying the frozen
// tree. The relaxation is an allow-list: completed, partial, and any result
// the code does not know keep the certified binding, so a new or malformed
// result cannot reach the relaxed path by default.
func TestAssignmentSubjectBindingRelaxesOnlyFailedAndBlocked(t *testing.T) {
	for result, want := range map[types.EngineeringAssignmentResultKind]capsule.SubjectBinding{
		types.EngineeringResultFailed:    capsule.SubjectObserved,
		types.EngineeringResultBlocked:   capsule.SubjectObserved,
		types.EngineeringResultCompleted: capsule.SubjectCertified,
		types.EngineeringResultPartial:   capsule.SubjectCertified,
		"":                               capsule.SubjectCertified,
		"Blocked":                        capsule.SubjectCertified,
		"succeeded":                      capsule.SubjectCertified,
	} {
		if got := assignmentSubjectBinding(result); got != want {
			t.Errorf("assignmentSubjectBinding(%q) = %q, want %q", result, got, want)
		}
	}
}

// An observed attestation records the tree the command itself observed beside
// the frozen subject and says no receipt certifies it; a certified one is
// unchanged (TestExecutionAttestationPinsFrozenFinalSubject).
func TestExecutionAttestationObservedRecordsReceiptTreeBesideFrozenSubject(t *testing.T) {
	subject := objectgraph.SHA256([]byte("binding-subject"))
	frozen := objectgraph.SHA256([]byte("frozen-after-receiptless-mutation"))
	observed := objectgraph.SHA256([]byte("tree-the-last-receipt-saw"))
	assignment := types.EngineeringAssignment{
		AssignmentID: "assignment-obs", BoundRunID: "run-obs",
		Binding: types.EngineeringAssignmentBinding{CapsuleID: "capsule-obs", SubjectDigest: subject},
	}
	receipt := capsule.ExecutionReceipt{
		AgentRunID: "run-obs", CapsuleID: "capsule-obs", Command: "npm run build",
		GrantedReceiptRef: "capsule-granted-exec:sha256:" + strings.Repeat("6", 64),
		SourceTreeDigest:  subject, WorktreeDigest: strings.TrimPrefix(observed, "sha256:"),
		StdoutDigest: objectgraph.SHA256([]byte("out")), StderrDigest: objectgraph.SHA256([]byte("")),
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	command := types.EngineeringRecordedCommand{CommandID: "cmd-1", CommandDigest: objectgraph.SHA256([]byte(receipt.Command)), ExecutionRef: "capsule-exec:sha256:x"}

	att, err := engineeringExecutionAttestationFromReceipt(assignment, "report-obs", command, receipt, frozen, capsule.SubjectObserved)
	if err != nil {
		t.Fatalf("observed attestation: %v", err)
	}
	if att.SubjectBinding != types.EngineeringSubjectBindingObserved || att.FinalSubjectDigest != frozen || att.WorktreeDigest != observed {
		t.Fatalf("observed attestation = binding %q final %s worktree %s", att.SubjectBinding, att.FinalSubjectDigest, att.WorktreeDigest)
	}
	if att.WorktreeDigest == att.FinalSubjectDigest {
		t.Fatal("the observed attestation certifies the frozen subject")
	}

	receipt.WorktreeDigest = ""
	att, err = engineeringExecutionAttestationFromReceipt(assignment, "report-obs", command, receipt, frozen, capsule.SubjectObserved)
	if err != nil || att.WorktreeDigest != "" || att.SubjectBinding != types.EngineeringSubjectBindingObserved {
		t.Fatalf("a receipt with no observed tree = %+v, %v", att, err)
	}

	certified, err := engineeringExecutionAttestationFromReceipt(assignment, "report-obs", command, receipt, frozen, capsule.SubjectCertified)
	if err != nil || certified.SubjectBinding != "" || certified.WorktreeDigest != frozen {
		t.Fatalf("certified attestation = %+v, %v", certified, err)
	}
}
