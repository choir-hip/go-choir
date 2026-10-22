package store

import (
	"context"
	"errors"
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
