package types

import (
	"strings"
	"testing"
	"time"
)

func assignmentDigest(seed string) string {
	return "sha256:" + strings.Repeat(seed, 64)
}

func validEngineeringAssignmentFixture(kind EngineeringAssignmentKind, writable bool) EngineeringAssignment {
	now := time.Now().UTC()
	binding := EngineeringAssignmentBinding{
		OwnerID: "owner", ComputerID: "computer", TrajectoryID: "trajectory",
		ParentAgentID: "management:owner", ParentRunID: "run-super", ParentDecisionID: "decision:" + assignmentDigest("d"), ParentControlID: "control-1",
		ParentWorkItemID: "work-super", AssignedWorkItemID: "work-cosuper", AssignedAgentID: "engineering:one",
		Kind: kind, Attempt: 1, ScopeDigest: assignmentDigest("a"), RequestDigest: assignmentDigest("e"), CapabilityDigest: assignmentDigest("b"), ExecutionHandleDigest: assignmentDigest("f"),
		SubjectDigest: assignmentDigest("c"), SourceArtifactRef: "capsule-source-git:commit:" + assignmentDigest("c"), Writable: writable,
		NetworkMode:    EngineeringCapsuleNetworkForbidden,
		FilesystemMode: EngineeringCapsuleFilesystemAssignmentLocalWritableOverlay,
	}
	if writable {
		binding.CapsuleID = "capsule-one"
	}
	if kind == EngineeringAssignmentVerification {
		binding.SourceCandidateID = "candidate-one"
		binding.SourceArtifactRef = "capsule-subject:" + binding.SubjectDigest
	}
	return EngineeringAssignment{
		Schema: EngineeringAssignmentSchemaV1, AssignmentID: "assignment-one", Binding: binding,
		Disposition: EngineeringAssignmentOpen, CapsuleDisposition: EngineeringCapsuleUnbound,
		LifecycleVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
}

func TestEngineeringAssignmentBindingValidationExhaustive(t *testing.T) {
	base := validEngineeringAssignmentFixture(EngineeringAssignmentImplementation, true)
	if err := base.Validate(); err != nil {
		t.Fatalf("valid assignment: %v", err)
	}
	for name, mutate := range map[string]func(*EngineeringAssignment){
		"owner":                         func(a *EngineeringAssignment) { a.Binding.OwnerID = "" },
		"computer":                      func(a *EngineeringAssignment) { a.Binding.ComputerID = "" },
		"trajectory":                    func(a *EngineeringAssignment) { a.Binding.TrajectoryID = "" },
		"exact super":                   func(a *EngineeringAssignment) { a.Binding.ParentAgentID = "management:other" },
		"parent run":                    func(a *EngineeringAssignment) { a.Binding.ParentRunID = "" },
		"parent decision":               func(a *EngineeringAssignment) { a.Binding.ParentDecisionID = "" },
		"parent control":                func(a *EngineeringAssignment) { a.Binding.ParentControlID = "" },
		"parent work":                   func(a *EngineeringAssignment) { a.Binding.ParentWorkItemID = "" },
		"assigned work":                 func(a *EngineeringAssignment) { a.Binding.AssignedWorkItemID = "" },
		"assigned agent":                func(a *EngineeringAssignment) { a.Binding.AssignedAgentID = "" },
		"kind":                          func(a *EngineeringAssignment) { a.Binding.Kind = "review" },
		"attempt":                       func(a *EngineeringAssignment) { a.Binding.Attempt = 0 },
		"scope digest":                  func(a *EngineeringAssignment) { a.Binding.ScopeDigest = "sha256:no" },
		"request digest":                func(a *EngineeringAssignment) { a.Binding.RequestDigest = "" },
		"source artifact":               func(a *EngineeringAssignment) { a.Binding.SourceArtifactRef = "" },
		"capability digest":             func(a *EngineeringAssignment) { a.Binding.CapabilityDigest = "" },
		"subject digest":                func(a *EngineeringAssignment) { a.Binding.SubjectDigest = "" },
		"writable capsule":              func(a *EngineeringAssignment) { a.Binding.CapsuleID = "" },
		"path-shaped capsule":           func(a *EngineeringAssignment) { a.Binding.CapsuleID = ".." },
		"network mode":                  func(a *EngineeringAssignment) { a.Binding.NetworkMode = "allowed" },
		"filesystem mode":               func(a *EngineeringAssignment) { a.Binding.FilesystemMode = "host" },
		"partial coordination contract": func(a *EngineeringAssignment) { a.Binding.CoordinationContractID = "contract" },
		"version":                       func(a *EngineeringAssignment) { a.LifecycleVersion = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("invalid assignment accepted")
			}
		})
	}
	verification := validEngineeringAssignmentFixture(EngineeringAssignmentVerification, true)
	if err := verification.Validate(); err != nil {
		t.Fatalf("valid verification: %v", err)
	}
	noneNetwork := verification
	noneNetwork.Binding.NetworkMode = EngineeringCapsuleNetworkNone
	if err := noneNetwork.Validate(); err != nil {
		t.Fatalf("network_mode=none should remain machine-verifiable networkless policy: %v", err)
	}
	verification.Binding.Writable, verification.Binding.CapsuleID = false, ""
	if err := verification.Validate(); err == nil {
		t.Fatal("read-only verification accepted")
	}
}

func validAssignmentReportFixture(a EngineeringAssignment) EngineeringAssignmentReport {
	return EngineeringAssignmentReport{
		Schema: EngineeringAssignmentSchemaV1, ReportID: "report-one", AssignmentID: a.AssignmentID,
		Attempt: a.Binding.Attempt, OwnerID: a.Binding.OwnerID, ComputerID: a.Binding.ComputerID,
		TrajectoryID: a.Binding.TrajectoryID, RunID: a.BoundRunID, AssignedAgentID: a.Binding.AssignedAgentID,
		Result: EngineeringResultCompleted, Verdict: EngineeringVerdictPass, Summary: "exact verification result",
		ObservedSubjectDigest: a.Binding.SubjectDigest, CertifiesOriginalSubject: true, CreatedAt: time.Now().UTC(),
		Commands:            []EngineeringRecordedCommand{{CommandID: "cmd", CommandDigest: assignmentDigest("d"), ExecutionRef: "receipt:cmd"}},
		ExecutorReceiptRefs: []string{"capsule-granted-exec:sha256:test"},
		Outputs:             []EngineeringRecordedOutput{{OutputID: "out", Kind: "evidence", Digest: assignmentDigest("e"), Ref: "artifact:out"}},
	}
}

func TestVerificationReportSubjectIdentityContract(t *testing.T) {
	a := validEngineeringAssignmentFixture(EngineeringAssignmentVerification, true)
	a.Disposition, a.BoundRunID, a.LifecycleVersion = EngineeringAssignmentBound, "run-cosuper", 2
	a.CapsuleDisposition = EngineeringCapsuleActive
	report := validAssignmentReportFixture(a)
	if err := report.ValidateAgainst(a); err != nil {
		t.Fatalf("immutable verification pass: %v", err)
	}

	changed := report
	changed.ObservedSubjectDigest = assignmentDigest("f")
	changed.CertifiesOriginalSubject = false
	changed.CandidateSubjectDigest = changed.ObservedSubjectDigest
	changed.CandidateID = "obj:choir.co_super_subject_candidate:b3duZXI:candidate"
	changed.CandidateArtifactRef = "capsule-subject:" + changed.CandidateSubjectDigest
	changed.Mutations = []EngineeringRecordedMutation{{
		MutationID: "mutation", Kind: "subject_bytes", BeforeDigest: a.Binding.SubjectDigest,
		AfterDigest: changed.ObservedSubjectDigest, EvidenceRef: "receipt:mutation", SubjectBytesChanged: true,
	}}
	if err := changed.ValidateAgainst(a); err != nil {
		t.Fatalf("changed subject candidate: %v", err)
	}
	changed.CertifiesOriginalSubject = true
	if err := changed.ValidateAgainst(a); err == nil {
		t.Fatal("changed verification subject certified original")
	}

	late := report
	late.Late, late.CertifiesOriginalSubject = true, false
	if err := late.ValidateAgainst(a); err != nil {
		t.Fatalf("late non-certifying result: %v", err)
	}
	late.CertifiesOriginalSubject = true
	if err := late.ValidateAgainst(a); err == nil {
		t.Fatal("late verification certified original")
	}

	missing := report
	missing.ObservedSubjectDigest = ""
	if err := missing.ValidateAgainst(a); err == nil {
		t.Fatal("verification without immutable subject digest accepted")
	}
}

func TestVerificationPassRequiresCapsuleCommandEvidence(t *testing.T) {
	a := validEngineeringAssignmentFixture(EngineeringAssignmentVerification, true)
	a.Disposition, a.BoundRunID, a.LifecycleVersion = EngineeringAssignmentBound, "run-cosuper", 2
	a.CapsuleDisposition = EngineeringCapsuleFrozen
	report := validAssignmentReportFixture(a)
	report.Commands = nil
	if err := report.ValidateAgainst(a); err == nil || !strings.Contains(err.Error(), "executor receipt evidence") {
		t.Fatalf("empty verification pass error = %v", err)
	}
}
