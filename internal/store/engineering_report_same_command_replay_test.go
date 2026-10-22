package store

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// FP4 round 4, P2
// (docs/problems/fp4-r4-report-conflict-guard-scope-dead-letter-2026-10-22.md).
// Two writers of one terminal report (the worker's Complete and the fate
// wake's resume) share its command id, because the id is derived from the
// proposition, but their requests differ in evidence, so the second got
// "lifecycle command digest conflict" for a report that had landed. Ways
// the fix can fail: the second writer still gets the conflict; it gets
// success but a second outcome is minted (lifecycle advanced, a second
// final report); a different proposition under the same command id is
// accepted instead of conflicting; a partial report is treated the same.
func TestReportUnderACommittedCommandIDWithTheSamePropositionReplays(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, f.seed.AssignmentIDs[0], "cap-same-command", "capsule-same-command")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-same-command")); err != nil {
		t.Fatal(err)
	}
	firstReq := assignmentReportRequest(open, 2, "report-same-command", open.Binding.SubjectDigest, types.EngineeringResultCompleted, types.EngineeringVerdictNone)
	first, err := s.RecordEngineeringAssignmentReport(ctx, firstReq)
	if err != nil || first.Report == nil || first.Assignment.Disposition != types.EngineeringAssignmentCompleted {
		t.Fatalf("first terminal: %+v err=%v", first, err)
	}

	// The other writer: same command id and report id, same proposition,
	// different evidence (summary, outcome refs), so a different digest.
	second := firstReq
	second.ExpectedLifecycleVersion = first.Assignment.LifecycleVersion
	second.Report.Summary = "the same outcome, authored by the other writer"
	second.OutcomeRefs = []string{"selfdev-operation:other-writer"}
	second.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(second)
	if second.CommandDigest == firstReq.CommandDigest {
		t.Fatal("fixture: the second writer's digest must differ")
	}
	replayed, err := s.RecordEngineeringAssignmentReport(ctx, second)
	if err != nil {
		t.Fatalf("a report already committed under this command id answered %v, want its committed result", err)
	}
	if !replayed.Replay || replayed.Report == nil || replayed.Report.ReportID != first.Report.ReportID || replayed.Report.Summary != first.Report.Summary {
		t.Fatalf("the replay is not the committed report: %+v", replayed)
	}
	if replayed.Assignment.LifecycleVersion != first.Assignment.LifecycleVersion {
		t.Fatalf("the replay advanced the lifecycle %d -> %d", first.Assignment.LifecycleVersion, replayed.Assignment.LifecycleVersion)
	}
	requireFinalEngineeringReport(t, s, f, open, 1, first.Report.ReportID, types.EngineeringAssignmentCompleted)

	// A different proposition under the same command id still conflicts.
	changed := firstReq
	changed.Report.Outputs = append([]types.EngineeringRecordedOutput(nil), firstReq.Report.Outputs...)
	changed.Report.Outputs[0].Digest = objectgraph.SHA256([]byte("a different output"))
	changed.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(changed)
	if _, err := s.RecordEngineeringAssignmentReport(ctx, changed); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("a different proposition under a committed command id = %v, want the conflict", err)
	}
}

// Review round 4b (Sonnet F2): the replay runs the fresh path's checks
// first. A request whose stated proposition digest is not the one its
// report computes, or whose report id is not the one that digest derives,
// is refused as the fresh path refuses it, not acknowledged as landed.
func TestSameCommandReplayRefusesAMismatchedPropositionOrReportID(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, f.seed.AssignmentIDs[0], "cap-replay-checks", "capsule-replay-checks")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-replay-checks")); err != nil {
		t.Fatal(err)
	}
	firstReq := assignmentReportRequest(open, 2, "report-replay-checks", open.Binding.SubjectDigest, types.EngineeringResultCompleted, types.EngineeringVerdictNone)
	first, err := s.RecordEngineeringAssignmentReport(ctx, firstReq)
	if err != nil {
		t.Fatalf("first terminal: %v", err)
	}

	bogus := firstReq
	bogus.ExpectedLifecycleVersion = first.Assignment.LifecycleVersion
	bogus.Report.PropositionDigest = objectgraph.SHA256([]byte("not this report's proposition"))
	bogus.OutcomeRefs = []string{"selfdev-operation:other-writer"} // the second writer's digest differs
	bogus.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(bogus)
	if res, err := s.RecordEngineeringAssignmentReport(ctx, bogus); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a bogus proposition digest under the committed command id = %+v, %v; want ErrEngineeringAssignmentInvalid", res.Replay, err)
	}

	stated := firstReq
	stated.ExpectedLifecycleVersion = first.Assignment.LifecycleVersion
	stated.Report.PropositionDigest = first.Report.PropositionDigest // right digest, but the report id is not TerminalReportID(digest)
	stated.OutcomeRefs = []string{"selfdev-operation:other-writer"}
	stated.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(stated)
	if res, err := s.RecordEngineeringAssignmentReport(ctx, stated); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a report id that its proposition does not derive = %+v, %v; want ErrEngineeringAssignmentInvalid", res.Replay, err)
	}
}

// Review round 4b (Sonnet F7): the replay acknowledges the proposition, not
// the evidence; when the second writer's evidence differs from the
// committed report's, one line says so.
func TestSameCommandReplayLogsDifferingEvidence(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, f.seed.AssignmentIDs[0], "cap-replay-evidence", "capsule-replay-evidence")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-replay-evidence")); err != nil {
		t.Fatal(err)
	}
	firstReq := assignmentReportRequest(open, 2, "report-replay-evidence", open.Binding.SubjectDigest, types.EngineeringResultCompleted, types.EngineeringVerdictNone)
	first, err := s.RecordEngineeringAssignmentReport(ctx, firstReq)
	if err != nil {
		t.Fatalf("first terminal: %v", err)
	}
	var buf bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(previous) })

	second := firstReq
	second.ExpectedLifecycleVersion = first.Assignment.LifecycleVersion
	second.Report.ExecutorReceiptRefs = []string{"capsule-granted-exec:sha256:other-writer"}
	second.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(second)
	if _, err := s.RecordEngineeringAssignmentReport(ctx, second); err != nil {
		t.Fatalf("same-proposition replay: %v", err)
	}
	if !strings.Contains(buf.String(), "evidence differs") || !strings.Contains(buf.String(), first.Report.ReportID) {
		t.Fatalf("no line says the replayed writer's evidence differs:\n%s", buf.String())
	}
}

// Review round 4c (Sonnet E4): each replay check and each evidence leg on its
// own, so dropping any one of them fails a test.
func TestSameCommandReplayChecksAndEvidenceLegsEachHold(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, f.seed.AssignmentIDs[0], "cap-replay-legs", "capsule-replay-legs")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-replay-legs")); err != nil {
		t.Fatal(err)
	}
	base := assignmentReportRequest(open, 2, "report-placeholder", open.Binding.SubjectDigest, types.EngineeringResultCompleted, types.EngineeringVerdictNone)
	digest, err := ComputeTerminalPropositionDigest(open.Binding.SubjectDigest, base.Report.Result, base.Report.Verdict, base.Report.Commands, base.Report.Outputs, base.Report.EvidenceRefs)
	if err != nil {
		t.Fatal(err)
	}
	// A report id derived from the proposition, as the runtime commits it.
	base.Report.ReportID = TerminalReportID(open.Binding.OwnerID, open.Binding.ComputerID, open.AssignmentID, open.Binding.Attempt, digest)
	base.CommandID = "engineering-report:" + open.AssignmentID + ":" + base.Report.ReportID
	base.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(base)
	first, err := s.RecordEngineeringAssignmentReport(ctx, base)
	if err != nil {
		t.Fatalf("first terminal: %v", err)
	}
	// The digest check alone: a derived report id, a wrong stated digest.
	wrong := base
	wrong.ExpectedLifecycleVersion = first.Assignment.LifecycleVersion
	wrong.Report.PropositionDigest = objectgraph.SHA256([]byte("a stated digest the report does not compute"))
	wrong.OutcomeRefs = []string{"selfdev-operation:other-writer"}
	wrong.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(wrong)
	if _, err := s.RecordEngineeringAssignmentReport(ctx, wrong); !errors.Is(err, ErrEngineeringAssignmentInvalid) {
		t.Fatalf("a wrong stated digest with a derived report id = %v, want ErrEngineeringAssignmentInvalid", err)
	}
	// Each evidence leg alone is named.
	committed := *first.Report
	for leg, mutate := range map[string]func(*types.RecordEngineeringAssignmentReportRequest){
		"candidate": func(r *types.RecordEngineeringAssignmentReportRequest) {
			r.Report.CandidateArtifactRef = "capsule-subject:other"
		},
		"mutations": func(r *types.RecordEngineeringAssignmentReportRequest) {
			r.Report.Mutations = []types.EngineeringRecordedMutation{{SubjectBytesChanged: true, AfterDigest: objectgraph.SHA256([]byte("other"))}}
		},
		"attestations": func(r *types.RecordEngineeringAssignmentReportRequest) {
			r.ExecutionAttestations = append(r.ExecutionAttestations, types.EngineeringExecutionAttestation{})
		},
	} {
		req := base
		req.Report = committed
		req.Report.Mutations = append([]types.EngineeringRecordedMutation(nil), committed.Mutations...)
		req.ExecutionAttestations = append([]types.EngineeringExecutionAttestation(nil), committed.ExecutionAttestations...)
		mutate(&req)
		if got := reportEvidenceDifferences(committed, req); len(got) != 1 || got[0] != leg {
			t.Fatalf("evidence leg %s: differences = %q", leg, got)
		}
	}
}
