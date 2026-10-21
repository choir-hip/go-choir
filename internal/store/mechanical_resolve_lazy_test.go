package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: every
// Report commit read up to 10,000 commitment records (a body scan, about a
// second under the paper's engine mutex) before checking whether any record
// it links is an open ask. Research now sends several interim Reports per
// ask. Failure mode: a Report that links no precommit still reads them. (A
// Report that answers an ask is TestCommitLifecycleActReportMechanically-
// ResolvesOpenAsk.)
func TestReportLinkingNoAskReadsNoRecordList(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	committedAt := time.Now().UTC().Format(time.RFC3339Nano)
	note := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-note:note:lazy",
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: caller.AgentID, CommittedAt: committedAt},
		Prediction: types.CommitmentPrediction{Hypothesis: "a first finding"},
	}
	if _, err := s.CommitLifecycleAct(ctx, commitActRequestForTest(t, s, start, caller, note, "")); err != nil {
		t.Fatalf("commit first report: %v", err)
	}
	report := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-report:report:lazy",
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: caller.AgentID, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		Prediction: types.CommitmentPrediction{Hypothesis: "a second finding"},
		ParentID:   note.RecordID, RelatedIDs: []string{note.RecordID},
	}
	before := bodyScanCalls(s)
	if _, err := s.CommitLifecycleAct(ctx, commitActRequestForTest(t, s, start, caller, report, "")); err != nil {
		t.Fatalf("commit linking report: %v", err)
	}
	if got := bodyScanCalls(s) - before; got != 0 {
		t.Fatalf("a Report linking no ask made %d body scans", got)
	}
	if rec, _ := s.GetCommitmentRecord(ctx, start.OwnerID, start.ComputerID, report.RecordID+":resolve:answered"); rec != nil {
		t.Fatalf("a Report linking no ask derived a resolve: %+v", rec)
	}
}
