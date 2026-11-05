package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The model provenance reads (docs/problems/model-provenance-unwired-2026-11-05.md):
// a run's model.served lines, and the Claude delegate's use on the reports
// an engineering run recorded.
func TestModelProvenanceReadsServedLinesAndDelegateReports(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	f := installEngineeringAssignmentAuthority(t, s, 1)
	open := engineeringOpenRequest(f, "assignment-provenance-delegate", "cap-provenance-delegate", "capsule-provenance-delegate")
	if _, err := s.OpenEngineeringAssignment(ctx, open); err != nil {
		t.Fatal(err)
	}
	bound, err := s.BindEngineeringAssignment(ctx, bindEngineeringRequest(open, f.assignedRunIDs[0], "cap-provenance-delegate"))
	if err != nil {
		t.Fatal(err)
	}
	report := assignmentReportRequest(open, bound.Assignment.LifecycleVersion, "report-provenance-delegate", open.Binding.SubjectDigest, types.EngineeringResultFailed, types.EngineeringVerdictNone)
	report.Report.DelegateUse = &types.EngineeringDelegateUse{Available: true, Calls: 3, Succeeded: 1, Model: "claude-sonnet-5-5"}
	report.CommandDigest, _ = ComputeRecordEngineeringAssignmentReportDigest(report)
	if _, err := s.RecordEngineeringAssignmentReport(ctx, report); err != nil {
		t.Fatal(err)
	}
	runID := f.assignedRunIDs[0]
	reports, err := s.ListEngineeringAssignmentReportsByRun(ctx, f.ownerID, runID)
	if err != nil || len(reports) != 1 || reports[0].DelegateUse == nil || reports[0].DelegateUse.Calls != 3 {
		t.Fatalf("reports of %s = %+v, %v", runID, reports, err)
	}
	if other, err := s.ListEngineeringAssignmentReportsByRun(ctx, "another-owner", runID); err != nil || len(other) != 0 {
		t.Fatalf("another owner read %d reports, %v", len(other), err)
	}

	served, _ := json.Marshal(types.ModelServedRecord{Schema: types.ModelServedSchema, Purpose: types.ModelServedPurposeToolLoop,
		Models: []types.ModelServedUsage{{Provider: "chatgpt", Model: "gpt-6", Calls: 1}}})
	for i, ev := range []types.EventRecord{
		{EventID: "ev-served-1", RunID: runID, OwnerID: f.ownerID, Kind: types.EventModelServed, Payload: served},
		{EventID: "ev-served-2", RunID: runID, OwnerID: f.ownerID, Kind: types.EventModelServed, Payload: served},
		{EventID: "ev-other-kind", RunID: runID, OwnerID: f.ownerID, Kind: types.EventRunCompleted, Payload: json.RawMessage(`{}`)},
		{EventID: "ev-other-run", RunID: "run-elsewhere", OwnerID: f.ownerID, Kind: types.EventModelServed, Payload: served},
	} {
		ev.Timestamp = time.Now().UTC().Add(time.Duration(i) * time.Millisecond)
		if err := s.AppendEvent(ctx, &ev); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := s.ListRunEventsOfKind(ctx, f.ownerID, runID, types.EventModelServed)
	if err != nil || len(lines) != 2 || lines[0].EventID != "ev-served-1" || lines[1].EventID != "ev-served-2" {
		t.Fatalf("served lines = %+v, %v", lines, err)
	}
	if _, err := s.ListRunEventsOfKind(ctx, f.ownerID, runID, types.EventToolResult); err == nil {
		t.Fatal("an event-table kind was read from the object graph half only")
	}
}
