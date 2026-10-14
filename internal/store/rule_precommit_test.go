package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// An engineering assignment's rule precommits persist as ordinary
// commitment records, list by their runtime-stamped assignment id, and
// settle once each from freeze, verify and terminal observations; the say–do
// view scores them.
func TestRulePrecommitsSettleFromReceipts(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const owner, computer, assignment = "owner-1", "computer-1", "assignment-abc"
	opened := time.Date(2026, 10, 14, 18, 0, 0, 0, time.UTC)
	since := opened.Format(time.RFC3339Nano)
	mint := func(local string, p float64, rule types.PrecommitRule) types.CommitmentRecord {
		rule.AssignmentID, rule.Since = assignment, since
		pc := &types.CommitmentPrecommit{Question: local, Resolver: types.PrecommitRuleResolver, Rule: &rule,
			Distribution: map[string]float64{types.PrecommitAnswerYes: p, types.PrecommitAnswerNo: 1 - p}}
		rec := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-1:precommit:" + local,
			Kind: types.CommitmentKindPrecommit, Discrepancy: types.DiscrepancyUnresolved, Precommit: pc,
			Prediction: types.CommitmentPrediction{Hypothesis: local},
			Provenance: types.CommitmentProvenance{AgentID: "engineering:" + assignment, CommittedAt: opened.Add(time.Minute).Format(time.RFC3339Nano)},
			ComputerID: computer}
		if _, err := s.AppendCommitmentRecord(ctx, owner, computer, rec); err != nil {
			t.Fatal(err)
		}
		return rec
	}
	mint("tests", 0.8, types.PrecommitRule{Kind: types.PrecommitRuleTestsPass})
	mint("freeze", 0.7, types.PrecommitRule{Kind: types.PrecommitRuleFreezeWithin, WithinMinutes: 20})
	mint("paths", 0.9, types.PrecommitRule{Kind: types.PrecommitRulePathsConfined, Paths: []string{"frontend"}})
	mint("verify", 0.6, types.PrecommitRule{Kind: types.PrecommitRuleVerifyPass})
	mint("completes", 0.9, types.PrecommitRule{Kind: types.PrecommitRuleCompletes})
	// Another assignment's precommit never settles with this one.
	other := types.PrecommitRule{Kind: types.PrecommitRuleCompletes, AssignmentID: "assignment-other"}
	otherRec := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-9:precommit:other", Kind: types.CommitmentKindPrecommit,
		Precommit:  &types.CommitmentPrecommit{Question: "other", Resolver: "rule", Rule: &other, Distribution: map[string]float64{"yes": 0.5, "no": 0.5}},
		Provenance: types.CommitmentProvenance{AgentID: "engineering:assignment-other"}}
	if _, err := s.AppendCommitmentRecord(ctx, owner, computer, otherRec); err != nil {
		t.Fatal(err)
	}

	listed, err := s.ListRulePrecommitRecords(ctx, owner, computer, assignment)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 5 {
		t.Fatalf("listed %d rule precommits for the assignment, want 5", len(listed))
	}

	// Freeze 25 minutes in: late against the 20m bound; one test failed;
	// the change stays under frontend.
	freeze := types.EngineeringFreezeObservation{FrozenAt: opened.Add(25 * time.Minute), BundleDigest: "sha256:b", OperationID: "op-1",
		TestReceipts:     []types.EngineeringReceiptExit{{Ref: "capsule-go-eval:sha256:1", ExitCode: 0}, {Ref: "capsule-go-eval:sha256:2", ExitCode: 2}},
		SourcePaths:      []string{"frontend/src/a.ts"},
		SourcePathsKnown: true}
	settledAtFreeze, err := s.SettleRulePrecommits(ctx, owner, computer, assignment, freeze.FrozenAt, func(r types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnFreeze(r, freeze)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := answers(settledAtFreeze); len(got) != 3 || got["freeze"] != "no" || got["tests"] != "no" || got["paths"] != "yes" {
		t.Fatalf("freeze settlements = %v", got)
	}
	// A replayed freeze settles nothing new (R4).
	again, err := s.SettleRulePrecommits(ctx, owner, computer, assignment, freeze.FrozenAt.Add(time.Minute), func(r types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnFreeze(r, types.EngineeringFreezeObservation{FrozenAt: opened, SourcePathsKnown: true})
	})
	if err != nil || len(again) != 0 {
		t.Fatalf("replayed freeze appended %d settlements (err %v), want 0", len(again), err)
	}

	terminal := types.EngineeringTerminalObservation{Disposition: types.EngineeringAssignmentCompleted, At: opened.Add(30 * time.Minute), CandidateFrozen: true, CandidateRef: "selfdev-operation:op-1"}
	settledAtEnd, err := s.SettleRulePrecommits(ctx, owner, computer, assignment, terminal.At, func(r types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleAtTerminal(r, terminal)
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := answers(settledAtEnd); len(got) != 1 || got["completes"] != "yes" {
		t.Fatalf("terminal settlements = %v (verify must wait for Verify)", got)
	}

	verifyAt := opened.Add(40 * time.Minute)
	settledAtVerify, err := s.SettleRulePrecommits(ctx, owner, computer, assignment, verifyAt, func(r types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnVerify(r, types.EngineeringVerifyObservation{Decision: "pass", BundleDigest: "sha256:b", VerifierRefs: []string{"verifier:1"}})
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := answers(settledAtVerify); len(got) != 1 || got["verify"] != "yes" {
		t.Fatalf("verify settlements = %v", got)
	}

	all, err := s.ListCommitmentRecords(ctx, owner, computer, "", 100)
	if err != nil {
		t.Fatal(err)
	}
	record := types.RulePrecommitSayDo(all, "engineering:"+assignment)
	o := record.Overall
	if o.Committed != 5 || o.Open != 0 || o.Yes != 3 || o.No != 2 || o.Unresolved != 0 {
		t.Fatalf("say–do overall = %+v", o)
	}
	// (0.8-0)^2 + (0.7-0)^2 + (0.9-1)^2 + (0.6-1)^2 + (0.9-1)^2 = 0.64+0.49+0.01+0.16+0.01
	if want := (0.64 + 0.49 + 0.01 + 0.16 + 0.01) / 5; abs(o.Brier-want) > 1e-9 {
		t.Fatalf("say–do Brier = %v, want %v", o.Brier, want)
	}
	// The other assignment stays open.
	if rest := types.RulePrecommitSayDo(all, "engineering:assignment-other").Overall; rest.Committed != 1 || rest.Open != 1 {
		t.Fatalf("other assignment = %+v", rest)
	}
}

func answers(records []types.CommitmentRecord) map[string]string {
	out := map[string]string{}
	for _, rec := range records {
		for question, answer := range rec.Scores[0].Answers {
			out[question] = answer
		}
	}
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
