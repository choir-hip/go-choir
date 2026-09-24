package store

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEngineeringCapsuleEvidenceOldRowsStayIncompleteAndDeterministic(t *testing.T) {
	s := openTestStore(t)
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, 0, "assignment-evidence-old", 1, types.EngineeringAssignmentImplementation, true, "cap-evidence-old", "capsule-evidence-old")
	if _, err := s.OpenEngineeringAssignment(context.Background(), open); err != nil {
		t.Fatal(err)
	}
	first, err := s.GetEngineeringCapsuleEvidence(context.Background(), f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"capsule_fate_history_missing", "grant_policy_attestation_missing", "isolation_probe_missing", "run_acceptance_gate_missing", "texture_source_missing"}
	if first.EvidenceComplete || !slices.Equal(first.Deficits, want) || first.SnapshotCursor != first.Watermark {
		t.Fatalf("old evidence = %+v", first)
	}
	a, _ := json.Marshal(first)
	second, err := s.GetEngineeringCapsuleEvidence(context.Background(), f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("repeat projection changed bytes")
	}
	if _, err := s.GetEngineeringCapsuleEvidence(context.Background(), "other-owner", f.computerID, f.trajectoryID, open.AssignmentID, 1); err != ErrNotFound {
		t.Fatalf("cross owner = %v", err)
	}
}

func TestEngineeringGrantAttestationIsStampedValidatedAndDigested(t *testing.T) {
	s := openTestStore(t)
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, 0, "assignment-evidence-grant", 1, types.EngineeringAssignmentImplementation, true, "cap-evidence-grant", "capsule-evidence-grant")
	if _, err := s.OpenEngineeringAssignment(context.Background(), open); err != nil {
		t.Fatal(err)
	}
	bind := bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-evidence-grant")
	verbs := engineeringCompiledVerbs()
	now := time.Now().UTC()
	bind.GrantPolicyAttestation = &types.EngineeringGrantPolicyAttestation{Role: "engineering", GrantedVerbs: verbs, VerbSetDigest: engineeringVerbSetDigest(verbs), PolicyDigest: engineeringPolicyDigest("engineering", verbs, open.Binding.NetworkMode, open.Binding.FilesystemMode, true), SignedCapabilityDigest: objectgraph.SHA256([]byte("signed-capability")), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true, SpawnedAt: now.Add(-time.Second), GrantedAt: now}
	bind.CommandDigest, _ = ComputeBindEngineeringAssignmentDigest(bind)
	result, err := s.BindEngineeringAssignment(context.Background(), bind)
	if err != nil {
		t.Fatal(err)
	}
	g := result.Assignment.GrantPolicyAttestation
	if g == nil || g.BindCommandID != bind.CommandID || g.BindEventID != bind.CommandID+":1" || g.ReducerSeq <= 0 || g.AttestationRef == "" {
		t.Fatalf("grant=%+v", g)
	}
	replay, err := s.BindEngineeringAssignment(context.Background(), bind)
	if err != nil || !replay.Replay || replay.Assignment.GrantPolicyAttestation.AttestationRef != g.AttestationRef {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	conflict := bind
	copy := *bind.GrantPolicyAttestation
	copy.SignedCapabilityDigest = objectgraph.SHA256([]byte("other"))
	conflict.GrantPolicyAttestation = &copy
	conflict.CommandDigest, _ = ComputeBindEngineeringAssignmentDigest(conflict)
	if _, err := s.BindEngineeringAssignment(context.Background(), conflict); err != ErrEngineeringAssignmentCommandConflict {
		t.Fatalf("conflict=%v", err)
	}
	evidence, err := s.GetEngineeringCapsuleEvidence(context.Background(), f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
	if err != nil || evidence.GrantPolicyAttestation == nil || evidence.GrantPolicyAttestation.AttestationRef != g.AttestationRef {
		t.Fatalf("evidence=%+v %v", evidence, err)
	}
}

func TestEngineeringExecutionAndFateAttestationsFateShareAndRemainOrdered(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, 0, "assignment-evidence-full", 1, types.EngineeringAssignmentImplementation, true, "cap-evidence-full", "capsule-evidence-full")
	opened, err := s.OpenEngineeringAssignment(ctx, open)
	if err != nil {
		t.Fatal(err)
	}
	verbs := engineeringCompiledVerbs()
	now := time.Now().UTC()
	bind := bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-evidence-full")
	bind.GrantPolicyAttestation = &types.EngineeringGrantPolicyAttestation{Role: "engineering", GrantedVerbs: verbs, VerbSetDigest: engineeringVerbSetDigest(verbs), PolicyDigest: engineeringPolicyDigest("engineering", verbs, open.Binding.NetworkMode, open.Binding.FilesystemMode, true), SignedCapabilityDigest: objectgraph.SHA256([]byte("cap")), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true, SpawnedAt: now.Add(-time.Second), GrantedAt: now}
	bind.CommandDigest, _ = ComputeBindEngineeringAssignmentDigest(bind)
	bound, err := s.BindEngineeringAssignment(ctx, bind)
	if err != nil {
		t.Fatal(err)
	}
	set := func(a types.EngineeringAssignment, d types.EngineeringCapsuleDisposition, intent, ack string, step types.EngineeringCapsuleFateStep) types.EngineeringAssignment {
		req := types.SetEngineeringCapsuleDispositionRequest{CommandID: "evidence-fate-" + string(d), OwnerID: f.ownerID, ComputerID: f.computerID, AssignmentID: a.AssignmentID, Attempt: 1, ExpectedLifecycleVersion: a.LifecycleVersion, Disposition: d, IntentRef: intent, AckRef: ack, FateStep: &step}
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
	report := assignmentReportRequest(open, a.LifecycleVersion, "report-evidence-execution", open.Binding.SubjectDigest, types.EngineeringResultCompleted, types.EngineeringVerdictNone)
	receiptRef := "capsule-granted-exec:sha256:" + strings.TrimPrefix(objectgraph.SHA256([]byte("receipt")), "sha256:")
	report.Report.ExecutorReceiptRefs = []string{receiptRef}
	report.ExecutionAttestations = []types.EngineeringExecutionAttestation{{GrantedReceiptRef: receiptRef, CommandID: report.Report.Commands[0].CommandID, CommandDigest: report.Report.Commands[0].CommandDigest, ExitCode: 0, StdoutDigest: objectgraph.SHA256([]byte("stdout")), StderrDigest: objectgraph.SHA256([]byte("stderr")), SourceSubjectDigest: open.Binding.SubjectDigest, FinalSubjectDigest: open.Binding.SubjectDigest, WorktreeDigest: open.Binding.SubjectDigest, Granted: true, Frozen: true, OccurredAt: now}}
	report.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(report)
	recorded, err := s.RecordEngineeringAssignmentReport(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	revokeIntent := "capsule-revoke-intent:" + objectgraph.SHA256([]byte("revoke"))
	a = set(recorded.Assignment, types.EngineeringCapsuleRevokeRequested, revokeIntent, "", types.EngineeringCapsuleFateStep{})
	revokeAck := "capsule-revoke:sha256:" + strings.TrimPrefix(objectgraph.SHA256([]byte("revoked")), "sha256:")
	a = set(a, types.EngineeringCapsuleRevoked, revokeIntent, revokeAck, types.EngineeringCapsuleFateStep{CapsuleAbsent: true, OccurredAt: now})
	if len(a.CapsuleFateHistory) != 4 || a.CapsuleFateHistory[1].Disposition != types.EngineeringCapsuleFrozen || a.CapsuleFateHistory[3].Disposition != types.EngineeringCapsuleRevoked {
		t.Fatalf("history=%+v", a.CapsuleFateHistory)
	}
	evidence, err := s.GetEngineeringCapsuleEvidence(ctx, f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence.ExecutionAttestations) != 1 || len(evidence.CapsuleFateHistory) != 4 || evidence.CapsuleFateHistory[3].CapsuleAbsent != true {
		t.Fatalf("evidence=%+v", evidence)
	}
	_ = opened
}

func TestEngineeringAttestationCommandDigestsExcludeOnlyStoreStampedFields(t *testing.T) {
	base := types.BindEngineeringAssignmentRequest{CommandID: "bind", OwnerID: "owner", ComputerID: "computer", AssignmentID: "assignment", Attempt: 1, RunID: "run", OpaqueCapability: "opaque", CapsuleID: "capsule", GrantPolicyAttestation: &types.EngineeringGrantPolicyAttestation{Role: "engineering", GrantedVerbs: []string{"exec"}, VerbSetDigest: objectgraph.SHA256([]byte("verbs")), PolicyDigest: objectgraph.SHA256([]byte("policy")), SignedCapabilityDigest: objectgraph.SHA256([]byte("signed")), SpawnAcknowledged: true, ActiveAcknowledged: true, GrantAcknowledged: true, SpawnedAt: time.Now().UTC(), GrantedAt: time.Now().UTC()}}
	one, _ := ComputeBindEngineeringAssignmentDigest(base)
	derived := base
	copy := *base.GrantPolicyAttestation
	copy.BindEventID = "changed-derived"
	copy.ReducerSeq = 999
	copy.RecordedAt = time.Now().UTC()
	derived.GrantPolicyAttestation = &copy
	two, _ := ComputeBindEngineeringAssignmentDigest(derived)
	if one != two {
		t.Fatalf("store-derived grant fields changed digest")
	}
	actual := base
	copy2 := *base.GrantPolicyAttestation
	copy2.PolicyDigest = objectgraph.SHA256([]byte("changed-policy"))
	actual.GrantPolicyAttestation = &copy2
	three, _ := ComputeBindEngineeringAssignmentDigest(actual)
	if one == three {
		t.Fatalf("actual policy fact absent from digest")
	}
	report := types.RecordEngineeringAssignmentReportRequest{CommandID: "report", OwnerID: "owner", ComputerID: "computer", AssignmentID: "assignment", Attempt: 1, Report: types.EngineeringAssignmentReport{ExecutionAttestations: []types.EngineeringExecutionAttestation{{AttestationRef: "model-authored"}}}, ExecutionAttestations: []types.EngineeringExecutionAttestation{{GrantedReceiptRef: "receipt", StdoutDigest: objectgraph.SHA256([]byte("stdout"))}}}
	d1, _ := ComputeRecordEngineeringAssignmentReportDigest(report)
	report.Report.ExecutionAttestations[0].AttestationRef = "other-model-value"
	d2, _ := ComputeRecordEngineeringAssignmentReportDigest(report)
	if d1 != d2 {
		t.Fatalf("model nested attestation entered digest")
	}
	report.ExecutionAttestations[0].StdoutDigest = objectgraph.SHA256([]byte("changed"))
	d3, _ := ComputeRecordEngineeringAssignmentReportDigest(report)
	if d1 == d3 {
		t.Fatalf("runtime receipt fact absent from digest")
	}
}

func TestEngineeringCapsuleEvidenceIsByteEqualAfterStoreReopen(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, 0, "assignment-evidence-reopen", 1, types.EngineeringAssignmentImplementation, true, "cap-evidence-reopen", "capsule-evidence-reopen")
	if _, err = s.OpenEngineeringAssignment(context.Background(), open); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetEngineeringCapsuleEvidence(context.Background(), f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before)
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	after, err := s.GetEngineeringCapsuleEvidence(context.Background(), f.ownerID, f.computerID, f.trajectoryID, open.AssignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatalf("projection changed across reopen\nbefore=%s\nafter=%s", a, b)
	}
}
