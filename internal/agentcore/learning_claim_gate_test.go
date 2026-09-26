package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// R4 acceptance: a self-development outcome claim citing no scored
// commitment_record is flagged (the enforced posture — refusal is the
// owner-tightenable follow-up); one citing resolved records is backed.

type stubLedger struct {
	records map[string]types.CommitmentRecord
}

func (s *stubLedger) GetCommitmentRecord(_ context.Context, _, _, recordID string) (*types.CommitmentRecord, error) {
	if rec, ok := s.records[recordID]; ok {
		return &rec, nil
	}
	return nil, nil
}

func TestLearningClaimGateFlagsUnbackedClaims(t *testing.T) {
	ledger := &stubLedger{records: map[string]types.CommitmentRecord{
		"rec-open":     {RecordID: "rec-open", Discrepancy: types.DiscrepancyUnresolved},
		"rec-resolved": {RecordID: "rec-resolved", Discrepancy: types.DiscrepancyContradicted},
		"rec-scored": {
			RecordID:    "rec-scored",
			Discrepancy: types.DiscrepancyUnresolved,
			Scores:      []types.CommitmentScore{{ScorerModelID: "m"}},
		},
	}}
	toolCtx := &CapsuleToolCtx{OwnerID: "owner-1", ComputerID: "comp-1", Ledger: ledger}

	if got := learningClaimGate(context.Background(), toolCtx, []string{"receipt-abc", "commitment://rec-open"}); got != types.LearningClaimUnbacked {
		t.Fatalf("unscored-record citation gated %s", got)
	}
	if got := learningClaimGate(context.Background(), toolCtx, []string{"commitment://rec-resolved"}); got != types.LearningClaimBacked {
		t.Fatalf("resolved-record citation gated %s", got)
	}
	if got := learningClaimGate(context.Background(), toolCtx, []string{"rec-scored"}); got != types.LearningClaimBacked {
		t.Fatalf("score-stamped citation gated %s", got)
	}
	// Nil ledger and missing refs never fail open.
	if got := learningClaimGate(context.Background(), &CapsuleToolCtx{OwnerID: "o", ComputerID: "c"}, []string{"x"}); got != types.LearningClaimUnbacked {
		t.Fatalf("nil ledger gated %s", got)
	}
	if got := learningClaimGate(context.Background(), toolCtx, nil); got != types.LearningClaimUnbacked {
		t.Fatalf("empty refs gated %s", got)
	}
}
