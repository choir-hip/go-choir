package agentcore

import (
	"errors"
	"testing"
	"time"

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

func TestDetachedAssignmentReportClosureSelectsOnlySoleTerminalResult(t *testing.T) {
	tests := []struct {
		name string
		call types.ToolCall
		want bool
	}{
		{name: "completed", call: types.ToolCall{ID: "call", Name: "record_assignment_result", Arguments: []byte(`{"result":"completed"}`)}, want: true},
		{name: "failed", call: types.ToolCall{ID: "call", Name: "record_assignment_result", Arguments: []byte(`{"result":"failed"}`)}, want: true},
		{name: "blocked", call: types.ToolCall{ID: "call", Name: "record_assignment_result", Arguments: []byte(`{"result":"blocked"}`)}, want: true},
		{name: "partial", call: types.ToolCall{ID: "call", Name: "record_assignment_result", Arguments: []byte(`{"result":"partial"}`)}, want: false},
		{name: "capsule effect", call: types.ToolCall{ID: "call", Name: "capsule_exec", Arguments: []byte(`{"result":"completed"}`)}, want: false},
		{name: "missing runtime call id", call: types.ToolCall{Name: "record_assignment_result", Arguments: []byte(`{"result":"completed"}`)}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalAssignedCoSuperReportCall(tc.call); got != tc.want {
				t.Fatalf("terminal closure selection = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestGrantedExecutionReceiptCopiesOnlySanitizedAttestationFacts(t *testing.T) {
	digest := objectgraph.SHA256([]byte("subject"))
	commandText := "secret command text"
	command := types.CoSuperRecordedCommand{CommandID: "command-one", CommandDigest: objectgraph.SHA256([]byte(commandText))}
	assignment := types.CoSuperAssignment{BoundRunID: "run-one", Binding: types.CoSuperAssignmentBinding{CapsuleID: "capsule-one", SubjectDigest: digest}}
	receipt := capsule.ExecutionReceipt{GrantedReceiptRef: "capsule-granted-exec:" + objectgraph.SHA256([]byte("granted")), AgentRunID: "run-one", CapsuleID: "capsule-one", ExitCode: 0, StdoutDigest: objectgraph.SHA256([]byte("stdout")), StderrDigest: objectgraph.SHA256([]byte("stderr")), SourceTreeDigest: digest, WorktreeDigest: digest, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), Command: commandText, Cwd: "/host/path"}
	att, err := coSuperExecutionAttestationFromReceipt(assignment, "report-one", command, receipt)
	if err != nil {
		t.Fatal(err)
	}
	if att.ReportID != "report-one" || att.CommandID != command.CommandID || att.CommandDigest != command.CommandDigest || att.StdoutDigest != receipt.StdoutDigest || att.SourceSubjectDigest != digest || !att.Granted || !att.Frozen {
		t.Fatalf("attestation=%+v", att)
	}
}
