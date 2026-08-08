package agentcore

import (
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type fixedRawAssignmentReceipts struct {
	receipts []capsule.ExecutionReceipt
	err      error
}

func (f fixedRawAssignmentReceipts) ResolveExecutionReceipts([]string) ([]capsule.ExecutionReceipt, error) {
	return f.receipts, f.err
}

func TestLateAssignmentExecutionReceiptsAuthenticateExactDetachedAuthority(t *testing.T) {
	handleDigest := objectgraph.SHA256([]byte("opaque-handle"))
	subject := objectgraph.SHA256([]byte("subject"))
	command := "go test ./..."
	receipt := capsule.ExecutionReceipt{ReceiptRef: "capsule-exec:sha256:raw", AgentRunID: "run-assigned", CapabilityHandleDigest: handleDigest[len("sha256:"):], CapsuleID: "capsule-assigned", Command: command, SourceTreeDigest: subject}
	assignment := types.CoSuperAssignment{AssignmentID: "assignment", BoundRunID: "run-assigned", Binding: types.CoSuperAssignmentBinding{CapsuleID: "capsule-assigned", ExecutionHandleDigest: handleDigest, SubjectDigest: subject}}
	report := types.CoSuperAssignmentReport{Commands: []types.CoSuperRecordedCommand{{CommandID: "command", CommandDigest: objectgraph.SHA256([]byte(command)), ExecutionRef: receipt.ReceiptRef}}}
	rt := &Runtime{assignmentReceiptResolver: fixedRawAssignmentReceipts{receipts: []capsule.ExecutionReceipt{receipt}}}
	got, err := rt.bindLateAssignmentExecutionReceipts(assignment, report)
	if err != nil || len(got.ExecutorReceiptRefs) != 1 || got.ExecutorReceiptRefs[0] != receipt.ReceiptRef {
		t.Fatalf("late raw receipt: %+v %v", got, err)
	}
	changed := receipt
	changed.AgentRunID = "foreign"
	rt.assignmentReceiptResolver = fixedRawAssignmentReceipts{receipts: []capsule.ExecutionReceipt{changed}}
	if _, err := rt.bindLateAssignmentExecutionReceipts(assignment, report); err == nil {
		t.Fatal("foreign run raw receipt accepted")
	}
	legacy := assignment
	legacy.Binding.ExecutionHandleDigest = ""
	rt.assignmentReceiptResolver = fixedRawAssignmentReceipts{receipts: []capsule.ExecutionReceipt{receipt}}
	if _, err := rt.bindLateAssignmentExecutionReceipts(legacy, report); err == nil {
		t.Fatal("receipt-bearing late evidence accepted without exact stored handle digest")
	}
	if narrated, err := rt.bindLateAssignmentExecutionReceipts(legacy, types.CoSuperAssignmentReport{Summary: "late command-free evidence"}); err != nil || narrated.Summary == "" {
		t.Fatalf("command-free legacy narration rejected: %+v %v", narrated, err)
	}
	rt.assignmentReceiptResolver = fixedRawAssignmentReceipts{err: errors.New("detached receipt unavailable")}
	if _, err := rt.bindLateAssignmentExecutionReceipts(assignment, report); err == nil {
		t.Fatal("missing persisted raw receipt accepted")
	}
}
