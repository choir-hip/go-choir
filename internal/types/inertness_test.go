package types

import (
	"strings"
	"testing"
)

func inertP(v float64) *float64 { return &v }

func yn(p float64) TypedQuestion {
	return TypedQuestion{Question: "Will the review find a defect?", Answers: []string{"yes", "no"},
		Probabilities: map[string]float64{"yes": p, "no": 1 - p}}
}

// castRec is an honest, informative Cast record to a desk: a typed question
// at 0.7, a weight, a named addressee. Each flag test breaks one property.
func castRec() CommitmentRecord {
	return CommitmentRecord{
		RecordID: "c1", Kind: CommitmentKindDirective, Directive: &CommitmentDirective{Subtype: CommitmentDirectiveCast, Objective: "Review the patch for defects"},
		Prediction: CommitmentPrediction{Hypothesis: "The review finds the defect in the parser", Questions: []TypedQuestion{yn(0.7)}},
		PSuccess:   inertP(0.7), Weight: inertP(0.4), Settlement: SettlementRecipient, Addressee: "research:doc-1",
		Provenance: CommitmentProvenance{Desk: "management:o1", Origin: OriginDeskRun},
	}
}

func wantFlags(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("%s: flags %v, want %v", name, got, want)
	}
}

func TestInertnessFlagEachRulePositiveAndNegative(t *testing.T) {
	wantFlags(t, "honest cast", DetectInertness("", castRec(), nil))

	// base rate: only fires against a measured class rate; a calibrated 0.5 is
	// not base rate unless the class base rate is 0.5.
	rates := map[string]float64{InertClass(RequestCast, "research"): 0.7}
	wantFlags(t, "at the class base rate", DetectInertness("", castRec(), rates), InertBaseRate)
	rec := castRec()
	rec.PSuccess = inertP(0.5)
	rec.Prediction.Questions = []TypedQuestion{yn(0.5)}
	wantFlags(t, "0.5 against a 0.7 class", DetectInertness("", rec, rates))
	wantFlags(t, "0.5 with no class rate", DetectInertness("", rec, nil))
	wantFlags(t, "0.5 against a 0.5 class", DetectInertness("", rec, map[string]float64{RequestCast: 0.5}), InertBaseRate)
	rec.PSuccess = inertP(0.7 + InertBaseRateEpsilon + 0.01)
	wantFlags(t, "just outside epsilon", DetectInertness("", rec, rates))

	// near-certain answer set.
	rec = castRec()
	rec.PSuccess = inertP(0.995)
	rec.Prediction.Questions = []TypedQuestion{yn(0.995)}
	wantFlags(t, "near-certain no risk", DetectInertness("", rec, nil), InertNearCertain)
	rec.Prediction.Assumptions = []string{"the patch applies cleanly"}
	wantFlags(t, "near-certain with a named risk", DetectInertness("", rec, nil))
	rec = castRec()
	rec.Prediction.Questions = []TypedQuestion{{Question: "Will it finish?", Answers: []string{"yes"}, Probabilities: map[string]float64{"yes": 1}}}
	wantFlags(t, "one-answer choice (it cannot falsify either)", DetectInertness("", rec, nil), InertNearCertain, InertNoFalsifier)
	rec = castRec()
	rec.PSuccess = inertP(0.02) // a near-certain failure is as inert as a near-certain success
	rec.Prediction.Questions = []TypedQuestion{yn(0.02)}
	wantFlags(t, "near-certain failure", DetectInertness("", rec, nil), InertNearCertain)

	// missing falsifier.
	rec = castRec()
	rec.Prediction.Questions = nil
	wantFlags(t, "cast with no question or criterion", DetectInertness("", rec, nil), InertNoFalsifier)
	rec.Cast = &CommitmentCast{Hypotheses: []CastHypothesis{{Statement: "s", Probability: 0.6, Criterion: "the parser test fails before and passes after"}}}
	wantFlags(t, "cast with a criterion", DetectInertness("", rec, nil))
	rec.Cast = &CommitmentCast{Hypotheses: []CastHypothesis{{Statement: "s", Probability: 0.6}}}
	wantFlags(t, "cast hypothesis with no criterion", DetectInertness("", rec, nil), InertNoFalsifier)
	esc := castRec()
	esc.Directive.Subtype = CommitmentDirectiveEscalate
	esc.Prediction.Questions = nil
	wantFlags(t, "an escalate needs no criterion", DetectInertness("", esc, nil))

	// missing decision dependency: no addressee, parent, related act, rule or sub-call parent.
	rec = castRec()
	rec.Addressee = ""
	wantFlags(t, "cast with nothing it feeds", DetectInertness("", rec, nil), InertNoDependency)
	rec.RelatedIDs = []string{"story-record-7"}
	wantFlags(t, "cast naming the act it feeds", DetectInertness("", rec, nil))
	rec.RelatedIDs = nil
	rec.Addressee = SubCallLedgerAddressee
	wantFlags(t, "the ledger is not a consumer", DetectInertness("", rec, nil), InertNoDependency)
	rec.SubCall = &CommitmentSubCall{Kind: SubCallCast, ParentRunID: "run-1", Role: "research_child"}
	wantFlags(t, "a sub-call names its parent run", DetectInertness("", rec, nil))

	// weight below the floor from the callee's grants.
	rec = castRec()
	rec.Weight = inertP(0.001)
	wantFlags(t, "weight under the floor", DetectInertness("", rec, nil), InertWeightBelowFloor)
	rec.Weight = inertP(SubCallWeightFloor("research"))
	wantFlags(t, "weight at the floor", DetectInertness("", rec, nil))
	rec.Weight = nil
	wantFlags(t, "no weight stated is the weight-zero mark, not a flag", DetectInertness("", rec, nil))

	// known-trivial template.
	for _, trivial := range []string{"The child returns a result.", "will the task complete?", "The agent responds with an answer", "It will work"} {
		rec = castRec()
		rec.Prediction.Hypothesis = trivial
		wantFlags(t, "trivial: "+trivial, DetectInertness("", rec, nil), InertTrivialTemplate)
	}
	for _, specific := range []string{"The child returns a result citing two primary sources", "The review finds the defect in the parser", ""} {
		rec = castRec()
		rec.Prediction.Hypothesis = specific
		wantFlags(t, "specific: "+specific, DetectInertness("", rec, nil))
	}
}

func TestInertnessIsPureAndNeverTouchesNonRequests(t *testing.T) {
	rec := castRec()
	rec.Prediction.Questions = nil
	a, b := DetectInertness("", rec, nil), DetectInertness("", rec, nil)
	wantFlags(t, "second run", b, a...)
	// A stored flag is not read: a cell-planted flag on an honest record
	// does not make it inert, and the detector overwrites rather than merges.
	honest := castRec()
	honest.InertFlags = []string{InertTrivialTemplate}
	wantFlags(t, "planted flag ignored", DetectInertness("", honest, nil))
	// A report, a notice and a system record are never flagged.
	report := CommitmentRecord{RecordID: "r", Kind: CommitmentKindReport, Report: &CommitmentReport{}}
	wantFlags(t, "report", DetectInertness("", report, nil))
	notice := castRec()
	notice.EmitterKind = CommitmentEmitterSystem
	notice.Prediction.Questions = nil
	wantFlags(t, "system record", DetectInertness("", notice, nil))
	// A rule-settled (engineering) precommit is mechanical and is left out.
	rule := CommitmentRecord{RecordID: "p", Kind: CommitmentKindPrecommit, Precommit: &CommitmentPrecommit{
		Question: "q", Distribution: map[string]float64{"yes": 0.99, "no": 0.01}, Resolver: "x", Rule: &PrecommitRule{}}}
	wantFlags(t, "rule-settled precommit", DetectInertness("", rule, nil))
}

func TestInertnessFlagsAreNotSettableByACell(t *testing.T) {
	for _, key := range []string{"inert", "inert_flags", "prompt_version"} {
		_, err := ParseRequestFields(`{"p_success":0.8,"weight":0.4,"settlement":"recipient","` + key + `":["x"]}`)
		if err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("a cell-supplied %q was admitted or unnamed: %v", key, err)
		}
	}
}

func TestInertJudgeSamplingIsDeterministicAndRated(t *testing.T) {
	if InertJudgeSampled("x", false) != InertJudgeSampled("x", false) {
		t.Fatal("not deterministic")
	}
	nu, nf := 0, 0
	for i := 0; i < 4000; i++ {
		id := SubCallCastRecordID("f", i)
		if InertJudgeSampled(id, false) {
			nu++
		}
		if InertJudgeSampled(id, true) {
			nf++
		}
	}
	if nu < 300 || nu > 500 || nf < 100 || nf > 300 {
		t.Fatalf("sampled %d unflagged, %d flagged of 4000", nu, nf)
	}
}

func TestInertCountsLastMAndDigestMinimum(t *testing.T) {
	var recs []CommitmentRecord
	for i := 0; i < 6; i++ {
		r := castRec()
		r.RecordID = SubCallCastRecordID("f", i)
		r.Provenance.CommittedAt = "2026-11-04T10:0" + string(rune('0'+i)) + ":00Z"
		if i%2 == 0 {
			r.Prediction.Questions = nil // no falsifier: inert
		}
		recs = append(recs, r)
	}
	got := InertCounts(recs, 20, nil)["management"]
	if got.M != 6 || got.N != 3 {
		t.Fatalf("counts %+v, want 3 of 6", got)
	}
	// The window keeps the newest M.
	if got := InertCounts(recs, 5, nil)["management"]; got.M != 5 || got.N != 2 {
		t.Fatalf("windowed %+v, want 2 of 5 (the oldest, inert, is out)", got)
	}
	if _, ok := InertCounts(recs[:4], 20, nil)["management"]; ok {
		t.Fatal("a desk under the minimum is shown a count")
	}
}
