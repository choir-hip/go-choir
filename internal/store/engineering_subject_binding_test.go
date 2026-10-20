package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A failed or blocked report may carry execution attestations whose receipt
// observed a different tree than the frozen subject (the receipt-less mutation
// of docs/problems/engineering-completion-path-cluster-2026-10-20.md); a
// completed report may not. The binding is recorded in the evidence packet.
func TestExecutionAttestationSubjectBindingIsForFailureReportsOnly(t *testing.T) {
	observedTree := objectgraph.SHA256([]byte("tree the receipt observed"))
	cases := []struct {
		name    string
		result  types.EngineeringAssignmentResultKind
		mutate  func(att *types.EngineeringExecutionAttestation)
		refused string
	}{
		{name: "blocked observed records", result: types.EngineeringResultBlocked, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.SubjectBinding, att.WorktreeDigest = types.EngineeringSubjectBindingObserved, observedTree
		}},
		{name: "failed observed without a tree records", result: types.EngineeringResultFailed, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.SubjectBinding, att.WorktreeDigest = types.EngineeringSubjectBindingObserved, ""
		}},
		{name: "blocked certified records", result: types.EngineeringResultBlocked, mutate: func(att *types.EngineeringExecutionAttestation) {}},
		{name: "blocked certified with another tree is refused", result: types.EngineeringResultBlocked, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.WorktreeDigest = observedTree
		}, refused: "final subject digest"},
		{name: "completed observed is refused", result: types.EngineeringResultCompleted, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.SubjectBinding, att.WorktreeDigest = types.EngineeringSubjectBindingObserved, observedTree
		}, refused: "subject binding"},
		{name: "completed with another tree is refused", result: types.EngineeringResultCompleted, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.WorktreeDigest = observedTree
		}, refused: "final subject digest"},
		{name: "unknown binding is refused", result: types.EngineeringResultBlocked, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.SubjectBinding = "certified-ish"
		}, refused: "subject binding"},
		{name: "observed with a malformed tree is refused", result: types.EngineeringResultBlocked, mutate: func(att *types.EngineeringExecutionAttestation) {
			att.SubjectBinding, att.WorktreeDigest = types.EngineeringSubjectBindingObserved, "not-a-digest"
		}, refused: "observed worktree digest"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := openTestStore(t)
			ctx := context.Background()
			f := installEngineeringAssignmentAuthority(t, s, 1)
			open := engineeringOpenRequest(f, "assignment-subject-binding", "cap-subject-binding", "capsule-subject-binding")
			if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
				t.Fatal(err)
			}
			verbs := engineeringCompiledVerbs()
			now := time.Now().UTC()
			bind := bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-subject-binding")
			bind.GrantPolicyAttestation = &types.EngineeringGrantPolicyAttestation{Role: "engineering", GrantedVerbs: verbs, VerbSetDigest: engineeringVerbSetDigest(verbs), PolicyDigest: engineeringPolicyDigest("engineering", verbs, open.Binding.NetworkMode, open.Binding.FilesystemMode, true), SignedCapabilityDigest: objectgraph.SHA256([]byte("cap")), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true, SpawnedAt: now.Add(-time.Second), GrantedAt: now}
			bind.CommandDigest, _ = ComputeBindEngineeringAssignmentDigest(bind)
			bound, err := s.BindEngineeringAssignment(ctx, bind)
			if err != nil {
				t.Fatal(err)
			}
			set := func(a types.EngineeringAssignment, d types.EngineeringCapsuleDisposition, intent, ack string, step types.EngineeringCapsuleFateStep) types.EngineeringAssignment {
				req := types.SetEngineeringCapsuleDispositionRequest{CommandID: "subject-binding-fate-" + string(d), OwnerID: f.ownerID, ComputerID: f.computerID, AssignmentID: a.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: a.LifecycleVersion, Disposition: d, IntentRef: intent, AckRef: ack, FateStep: &step}
				req.CommandDigest, _ = ComputeSetEngineeringCapsuleDispositionDigest(req)
				r, e := s.SetEngineeringCapsuleDisposition(ctx, req)
				if e != nil {
					t.Fatalf("%s: %v", d, e)
				}
				return r.Assignment
			}
			freezeIntent := "capsule-freeze-intent:" + objectgraph.SHA256([]byte("freeze"))
			a := set(bound.Assignment, types.EngineeringCapsuleFreezeRequested, freezeIntent, "", types.EngineeringCapsuleFateStep{})
			freezeAck := "capsule-fate:sha256:" + strings.TrimPrefix(objectgraph.SHA256([]byte("frozen")), "sha256:")
			a = set(a, types.EngineeringCapsuleFrozen, freezeIntent, freezeAck, types.EngineeringCapsuleFateStep{SourceSubjectDigest: open.Binding.SubjectDigest, FinalSubjectDigest: open.Binding.SubjectDigest, OccurredAt: now})
			report := assignmentReportRequest(open, a.LifecycleVersion, "report-subject-binding", open.Binding.SubjectDigest, tc.result, types.EngineeringVerdictNone)
			receiptRef := "capsule-granted-exec:sha256:" + strings.TrimPrefix(objectgraph.SHA256([]byte("receipt")), "sha256:")
			report.Report.ExecutorReceiptRefs = []string{receiptRef}
			att := types.EngineeringExecutionAttestation{GrantedReceiptRef: receiptRef, CommandID: report.Report.Commands[0].CommandID, CommandDigest: report.Report.Commands[0].CommandDigest, ExitCode: report.Report.Commands[0].ExitCode, StdoutDigest: objectgraph.SHA256([]byte("stdout")), StderrDigest: objectgraph.SHA256([]byte("stderr")), SourceSubjectDigest: open.Binding.SubjectDigest, FinalSubjectDigest: open.Binding.SubjectDigest, WorktreeDigest: open.Binding.SubjectDigest, Granted: true, Frozen: true, OccurredAt: now}
			tc.mutate(&att)
			report.ExecutionAttestations = []types.EngineeringExecutionAttestation{att}
			report.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(report)
			_, err = s.RecordEngineeringAssignmentReport(ctx, report)
			if tc.refused != "" {
				if err == nil || !strings.Contains(err.Error(), tc.refused) {
					t.Fatalf("record = %v, want a refusal naming %q", err, tc.refused)
				}
				return
			}
			if err != nil {
				t.Fatalf("record: %v", err)
			}
			evidence, err := s.GetEngineeringCapsuleEvidence(ctx, f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
			if err != nil || len(evidence.ExecutionAttestations) != 1 {
				t.Fatalf("evidence = %+v, %v", evidence, err)
			}
			got := evidence.ExecutionAttestations[0]
			if got.SubjectBinding != att.SubjectBinding || got.WorktreeDigest != att.WorktreeDigest || got.FinalSubjectDigest != open.Binding.SubjectDigest {
				t.Fatalf("evidence does not carry the binding and both digests: %+v", got)
			}
		})
	}
}
