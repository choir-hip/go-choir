package trust

// Failure modes of evidence extraction and the trust view (MA7.2), written
// before the code:
//
//  T1  a score changes after the fact: a later outcome (Realized/Materialized
//      for an earlier record) must leave its settled accuracy unchanged.
//  T2  the procedure or weights are edited after commit: the freeze hash fails
//      and the record is excluded; a reference whose hash does not match the
//      record's freeze is excluded too.
//  T3  a world surprise (a consequence nobody named before the commitment)
//      moves no desk's posterior; it is recorded against no one.
//  T4  the importance-judgment score moves no posterior or lease.
//  T5  excluded classes count: crash-closed, self-fulfilling, same-publisher
//      scorer, same-family scorer, self-score, weight-zero.
//  T6  the agent's own stated weight is used to weight evidence: it must not
//      be (flaw d); changing it changes nothing.
//  T7  negative skill is clipped: an overconfident forecast worse than the base
//      rate must count as evidence against.
//  T8  one huge event counts as dozens of observations: both candidates bound
//      the weight.
//  T9  derived view stored or order-dependent: Derive is a pure function of its
//      inputs and independent of record order.
//  T10 poisoning foreseeability: a naming from the subject's own publisher, one
//      made after the commitment, or an unscored/below-floor one is not a
//      naming.
//  T11 replay picks a candidate and reports both candidates' calibration and
//      AUC.

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

var t0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func ts(d time.Duration) string { return t0.Add(d).Format(time.RFC3339) }

func f(v float64) *float64 { return &v }

func yesNo(pYes float64) types.TypedQuestion {
	return types.TypedQuestion{Question: "did it hold", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": pYes, "no": 1 - pYes}}
}

// subCast builds a settled-able sub-call Cast: parent desk, callee role.
func subCast(id, parent, role, model string, at time.Duration, impact types.ImpactClass, weight float64, qs ...types.TypedQuestion) types.CommitmentRecord {
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive:  &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast},
		Provenance: types.CommitmentProvenance{AgentID: parent + ":d1", ModelID: model, Desk: parent, CommittedAt: ts(at), Origin: types.OriginDeskRun},
		SubCall:    &types.CommitmentSubCall{Kind: types.SubCallCast, Role: role, ParentAgent: parent + ":d1"},
		Prediction: types.CommitmentPrediction{Hypothesis: "h", Questions: qs, CommittedAt: ts(at)},
	}
	if err := rec.StampRequest(types.RequestCast, &types.RequestFields{PSuccess: f(0.7), Weight: f(weight), ImpactClass: impact, Settlement: types.SettlementRecipient, Questions: qs}); err != nil {
		panic(err)
	}
	return rec
}

func settlement(castID, method, scorer string, at time.Duration, answer string, sameFamily bool) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallSettlementRecordID(castID, scorer),
		Provenance: types.CommitmentProvenance{AgentID: scorer, CommittedAt: ts(at)},
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallSettlement, CastID: castID, Method: method, Scorer: scorer, SameFamily: sameFamily,
			Verdicts: []types.SubCallVerdict{{Q: 0, Answer: answer, Probabilities: map[string]float64{answer: 1}}},
			Class:    classFor(answer)},
	}
}

func classFor(answer string) types.DiscrepancyClass {
	if answer == "yes" {
		return types.DiscrepancyConfirmed
	}
	return types.DiscrepancyContradicted
}

func judge(castID string, at time.Duration, answer string) types.CommitmentRecord {
	return settlement(castID, types.CalibrationScorerIndependent, types.SubCallJudgeScorerID, at, answer, false)
}

func row(v View, kind SubjectKind, id string, class ActionClass) (Row, bool) {
	for _, r := range v.Rows {
		if r.Subject.Kind == kind && r.Subject.ID == id && r.Class == class {
			return r, true
		}
	}
	return Row{}, false
}

var castI1 = ActionClass{Grant: "desk.request", Impact: types.ImpactI1}

func baseParams() Params {
	p := DefaultParams()
	p.Now = t0.Add(48 * time.Hour)
	return p
}

func TestSettledSubCallIsE1ForParentAndE5ForChildRole(t *testing.T) {
	in := Inputs{Records: []types.CommitmentRecord{
		subCast("subcall:a:c0", "texture", "research_child", "m1", 0, types.ImpactI1, 0.5, yesNo(0.9)),
		judge("subcall:a:c0", time.Hour, "yes"),
	}}
	v := Derive(in, baseParams())
	parent, ok := row(v, SubjectAgent, "texture", castI1)
	if !ok || parent.Evidence[E1] != 1 || parent.Alpha <= parent.Alpha0 {
		t.Fatalf("parent row %+v ok=%v", parent, ok)
	}
	child, ok := row(v, SubjectAgent, "research_child", castI1)
	if !ok || child.Evidence[E5] != 1 || child.Alpha <= child.Alpha0 {
		t.Fatalf("child row %+v ok=%v", child, ok)
	}
	if child.Evidence[E1] != 0 {
		t.Fatalf("the child role gets E5, not E1: %+v", child.Evidence)
	}
}

func TestUnsettledSubCallIsNoEvidence(t *testing.T) {
	in := Inputs{Records: []types.CommitmentRecord{subCast("subcall:a:c0", "texture", "research_child", "m", 0, types.ImpactI1, 0.5, yesNo(0.9))}}
	v := Derive(in, baseParams())
	if len(v.Rows) != 0 {
		t.Fatalf("rows %+v", v.Rows)
	}
}

func TestExcludedClasses(t *testing.T) {
	cast := func(id string) types.CommitmentRecord {
		return subCast(id, "texture", "research_child", "m", 0, types.ImpactI1, 0.5, yesNo(0.9))
	}
	cases := map[string]struct {
		recs   []types.CommitmentRecord
		in     func(*Inputs)
		reason string
	}{
		"self score never settles": {recs: []types.CommitmentRecord{cast("c"), settlement("c", types.CalibrationScorerSelf, "texture:d1", time.Hour, "yes", false)}, reason: ExcludedSelfScored},
		"same family judge":        {recs: []types.CommitmentRecord{cast("c"), settlement("c", types.CalibrationScorerIndependent, types.SubCallJudgeScorerID, time.Hour, "yes", true)}, reason: ExcludedSameFamily},
		"crash closed": {recs: []types.CommitmentRecord{cast("c"), func() types.CommitmentRecord {
			r := judge("c", time.Hour, "yes")
			r.SubCall.Reason = types.ClosureReasonInterruptedByRestart
			return r
		}()}, reason: ExcludedInterrupted},
		"same publisher scorer": {recs: []types.CommitmentRecord{cast("c"), settlement("c", types.CalibrationScorerIndependent, "peer:d9", time.Hour, "yes", false)},
			in: func(in *Inputs) {
				in.Resolver = func(desk string) SubjectInfo { return SubjectInfo{Package: "pkg-" + desk, Publisher: "pub-1"} }
			}, reason: ExcludedSamePublisher},
		"self fulfilling": {recs: []types.CommitmentRecord{func() types.CommitmentRecord {
			r := cast("c")
			r.SubjectRefs = map[string]string{"outcome_emitter": "texture"}
			return r
		}(), judge("c", time.Hour, "yes")}, reason: ExcludedSelfFulfilling},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			in := Inputs{Records: tc.recs}
			if tc.in != nil {
				tc.in(&in)
			}
			v := Derive(in, baseParams())
			if _, ok := row(v, SubjectAgent, "texture", castI1); ok {
				t.Fatalf("%s moved a posterior: %+v", name, v.Rows)
			}
			found := false
			for _, e := range v.Excluded {
				if e.Reason == tc.reason {
					found = true
				}
			}
			if !found {
				t.Fatalf("no exclusion %q in %+v", tc.reason, v.Excluded)
			}
		})
	}
}

func TestWeightZeroCastIsExcluded(t *testing.T) {
	rec := types.CommitmentRecord{RecordID: "c", WeightZero: "no deciding model", SubCall: &types.CommitmentSubCall{Kind: types.SubCallCast, Role: "research_child", ParentAgent: "texture:d1"},
		Provenance: types.CommitmentProvenance{Desk: "texture", CommittedAt: ts(0)}}
	v := Derive(Inputs{Records: []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}}, baseParams())
	if len(v.Rows) != 0 || len(v.Excluded) == 0 || v.Excluded[0].Reason != ExcludedWeightZero {
		t.Fatalf("%+v %+v", v.Rows, v.Excluded)
	}
}

func TestStatedWeightDoesNotWeightEvidence(t *testing.T) {
	mk := func(w float64) View {
		in := Inputs{Records: []types.CommitmentRecord{
			subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, w, yesNo(0.9)),
			judge("c", time.Hour, "yes"),
		}}
		return Derive(in, baseParams())
	}
	a, b := mk(0.05), mk(0.95)
	if !reflect.DeepEqual(a.Rows, b.Rows) {
		t.Fatalf("the sender's stated weight moved the view:\n%+v\n%+v", a.Rows, b.Rows)
	}
}

func TestFrozenReferenceWeightsEvidence(t *testing.T) {
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
	set := func(pref, rel float64) View {
		ref := NewFrozenReference(rec, ts(0), pref, rel, nil, nil)
		return Derive(Inputs{Records: []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}, References: map[string]FrozenReference{"c": ref}}, baseParams())
	}
	lo, hi := set(0.1, 1), set(0.9, 1)
	cls := ActionClass{Grant: "desk.request", Impact: types.ImpactI2}
	rl, _ := row(lo, SubjectAgent, "texture", cls)
	rh, _ := row(hi, SubjectAgent, "texture", cls)
	if !(rh.Alpha-rh.Alpha0 > rl.Alpha-rl.Alpha0) {
		t.Fatalf("frozen P_ref must weight: lo %+v hi %+v", rl, rh)
	}
	zero := set(0.9, 0)
	if len(zero.Rows) != 0 {
		t.Fatalf("zero relevance earns nothing: %+v", zero.Rows)
	}
}

func TestChangedFreezeFailsHash(t *testing.T) {
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
	rec.MaterialityProcedure.ProcedureRef = "request.cast.lenient" // edited after commit
	v := Derive(Inputs{Records: []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}}, baseParams())
	if len(v.Rows) != 0 || len(v.Excluded) == 0 || v.Excluded[0].Reason != ExcludedFreezeHash {
		t.Fatalf("%+v %+v", v.Rows, v.Excluded)
	}
	// a reference frozen against a different procedure
	rec2 := subCast("c2", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
	ref := NewFrozenReference(rec2, ts(0), 0.5, 1, nil, nil)
	ref.ProcedureHash = "deadbeef"
	v = Derive(Inputs{Records: []types.CommitmentRecord{rec2, judge("c2", time.Hour, "yes")}, References: map[string]FrozenReference{"c2": ref}}, baseParams())
	if len(v.Rows) != 0 || v.Excluded[0].Reason != ExcludedReferenceHash {
		t.Fatalf("%+v %+v", v.Rows, v.Excluded)
	}
	ref = NewFrozenReference(rec2, ts(0), 0.5, 1, nil, nil)
	ref.PRef = 0.01 // edited after hashing
	v = Derive(Inputs{Records: []types.CommitmentRecord{rec2, judge("c2", time.Hour, "yes")}, References: map[string]FrozenReference{"c2": ref}}, baseParams())
	if len(v.Rows) != 0 || v.Excluded[0].Reason != ExcludedReferenceHash {
		t.Fatalf("edited reference: %+v %+v", v.Rows, v.Excluded)
	}
}

func TestSettledScoreUnchangedByLaterOutcome(t *testing.T) {
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
	recs := []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}
	ref := NewFrozenReference(rec, ts(0), 0.5, 1, nil, nil)
	refs := map[string]FrozenReference{"c": ref}
	before := Derive(Inputs{Records: recs, References: refs}, baseParams())
	after := Derive(Inputs{Records: recs, References: refs,
		Materialized: map[string][]string{"c": {"owner lost a story"}},
		Realized:     map[string]types.ImpactClass{"c": types.ImpactI4}}, baseParams())
	if !reflect.DeepEqual(before.Rows, after.Rows) {
		t.Fatalf("a later outcome moved a settled score:\n%+v\n%+v", before.Rows, after.Rows)
	}
}

func TestWorldSurpriseMovesNoPosterior(t *testing.T) {
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
	rec.Consequences = []types.CommitmentConsequence{{Label: "unforeseen outage", Predicted: false, Observed: true}}
	recs := []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}
	base := Derive(Inputs{Records: recs[:2]}, baseParams())
	v := Derive(Inputs{Records: recs}, baseParams())
	if !reflect.DeepEqual(base.Rows, v.Rows) {
		t.Fatalf("surprise moved a posterior")
	}
	if len(v.WorldSurprises) != 1 || v.WorldSurprises[0].Label != "unforeseen outage" || v.WorldSurprises[0].CountedAgainst != "" {
		t.Fatalf("surprise %+v", v.WorldSurprises)
	}
}

func TestForeseeableOmissionIsE2AndPoisonedNamingIsNot(t *testing.T) {
	mk := func(cons FrozenConsequence) View {
		rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
		rec.Consequences = []types.CommitmentConsequence{{Label: "deleted a draft", Predicted: false, Observed: true}}
		ref := NewFrozenReference(rec, ts(0), 0.5, 1, []FrozenConsequence{cons}, nil)
		return Derive(Inputs{Records: []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}, References: map[string]FrozenReference{"c": ref},
			Resolver: func(d string) SubjectInfo { return SubjectInfo{Publisher: "pub-" + d} }}, baseParams())
	}
	good := FrozenConsequence{Label: "deleted a draft", PRef: 0.4, Impact: types.ImpactI2, Relevance: 0.8, NamedBy: NamedByReference, NamedAt: ts(-time.Hour)}
	v := mk(good)
	r, ok := row(v, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2})
	if !ok || r.Evidence[E2] != 1 || r.Beta <= r.Beta0 {
		t.Fatalf("a foreseeable omission is E2 evidence against: %+v", r)
	}
	for name, bad := range map[string]FrozenConsequence{
		"own publisher":     {Label: "deleted a draft", PRef: 0.4, Impact: types.ImpactI2, Relevance: 0.8, NamedBy: NamedByPeer, NamerDesk: "texture", NamerScored: true, NamerP: 0.5, NamedAt: ts(-time.Hour)},
		"after commitment":  {Label: "deleted a draft", PRef: 0.4, Impact: types.ImpactI2, Relevance: 0.8, NamedBy: NamedByReference, NamedAt: ts(time.Minute)},
		"unscored peer":     {Label: "deleted a draft", PRef: 0.4, Impact: types.ImpactI2, Relevance: 0.8, NamedBy: NamedByPeer, NamerDesk: "mail", NamerScored: false, NamerP: 0.5, NamedAt: ts(-time.Hour)},
		"below the floor p": {Label: "deleted a draft", PRef: 0.4, Impact: types.ImpactI2, Relevance: 0.8, NamedBy: NamedByPeer, NamerDesk: "mail", NamerScored: true, NamerP: 0.001, NamedAt: ts(-time.Hour)},
	} {
		v := mk(bad)
		if r, ok := row(v, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2}); !ok || r.Evidence[E2] != 0 || r.Beta != r.Beta0 {
			t.Errorf("%s: a poisoned naming counted against the desk: %+v", name, r)
		}
		if len(v.WorldSurprises) != 1 {
			t.Errorf("%s: should be a world surprise: %+v", name, v.WorldSurprises)
		}
	}
	ok2 := mk(FrozenConsequence{Label: "deleted a draft", PRef: 0.4, Impact: types.ImpactI2, Relevance: 0.8, NamedBy: NamedByPeer, NamerDesk: "mail", NamerScored: true, NamerP: 0.4, NamedAt: ts(-time.Hour)})
	if r, _ := row(ok2, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2}); r.Evidence[E2] != 1 {
		t.Errorf("a scored peer naming from another publisher counts: %+v", r)
	}
}

func TestImportanceJudgmentMovesNoPosterior(t *testing.T) {
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.9))
	rec.ImpactClass = types.ImpactI2
	recs := []types.CommitmentRecord{rec, judge("c", time.Hour, "yes")}
	a := Derive(Inputs{Records: recs}, baseParams())
	b := Derive(Inputs{Records: recs, Realized: map[string]types.ImpactClass{"c": types.ImpactI4}}, baseParams())
	if !reflect.DeepEqual(a.Rows, b.Rows) {
		t.Fatal("score 2 moved a posterior")
	}
	j := b.Judgment[Subject{SubjectAgent, "texture"}]
	if j.N != 1 || j.MeanRPS <= 0 {
		t.Fatalf("score 2 must be shown beside the posterior: %+v", j)
	}
	// exact: forecast I2 point mass, realized I4 -> ranked probability score
	// sum over thresholds: cumulative forecast (0,0,1,1,1) vs realized (0,0,0,0,1):
	// differences at I2->I3 cumulative: 1 vs 0 =1, I3: 1 vs 0 = 1 ; (1+1)/(5-1) = 0.5
	if math.Abs(j.MeanRPS-0.5) > 1e-9 {
		t.Fatalf("rps %v", j.MeanRPS)
	}
}

func TestNegativeSkillIsNotClipped(t *testing.T) {
	// an overconfident forecast that misses is worse than the base rate: it
	// counts as evidence against, not as zero.
	rec := subCast("c", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.99))
	v := Derive(Inputs{Records: []types.CommitmentRecord{rec, judge("c", time.Hour, "no")}}, baseParams())
	r, ok := row(v, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2})
	if !ok || r.Beta <= r.Beta0 || r.Alpha != r.Alpha0 {
		t.Fatalf("%+v", r)
	}
	// copying the base rate (uniform) earns zero both ways
	rec2 := subCast("c2", "texture", "research_child", "m", 0, types.ImpactI2, 0.5, yesNo(0.5))
	v = Derive(Inputs{Records: []types.CommitmentRecord{rec2, judge("c2", time.Hour, "yes")}}, baseParams())
	if r, ok := row(v, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2}); ok && (r.Alpha != r.Alpha0 || r.Beta != r.Beta0) {
		t.Fatalf("copying the base rate must earn zero: %+v", r)
	}
}

func TestDeriveIsPureAndOrderIndependent(t *testing.T) {
	var recs []types.CommitmentRecord
	for i, id := range []string{"a", "b", "c", "d"} {
		recs = append(recs, subCast(id, "texture", "research_child", "m", time.Duration(i)*time.Hour, types.ImpactI2, 0.5, yesNo(0.8)))
		ans := "yes"
		if i == 2 {
			ans = "no"
		}
		recs = append(recs, judge(id, time.Duration(i)*time.Hour+time.Minute, ans))
	}
	a := Derive(Inputs{Records: recs}, baseParams())
	rev := make([]types.CommitmentRecord, len(recs))
	for i := range recs {
		rev[len(recs)-1-i] = recs[i]
	}
	b := Derive(Inputs{Records: rev}, baseParams())
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("order changed the view")
	}
	if a.TableDigest != ImpactTableDigest() || a.Params.Lambda == 0 {
		t.Fatalf("the view must log its parameters and table: %+v", a.Params)
	}
}

func TestOneHugeEventIsBoundedByBothCandidates(t *testing.T) {
	var recs []types.CommitmentRecord
	for i := 0; i < 20; i++ {
		id := string(rune('a' + i))
		recs = append(recs, subCast(id, "texture", "research_child", "m", time.Duration(i)*time.Minute, types.ImpactI1, 0.5, yesNo(0.8)), judge(id, time.Duration(i)*time.Minute+time.Second, "yes"))
	}
	fid := []FrameObservation{{Desk: "texture", Declared: []string{"desk.act"}, Used: []string{"desk.act", "desk.capsule.mutate"}, At: ts(time.Hour)}}
	for _, mode := range []WeightMode{ModeCapped, ModeTempered} {
		p := baseParams()
		p.Mode = mode
		v := Derive(Inputs{Records: recs, Frames: fid}, p)
		r, ok := row(v, SubjectAgent, "texture", ActionClass{Grant: "desk.capsule.mutate", Impact: types.ImpactI4})
		if !ok {
			t.Fatalf("%s: no fidelity row: %+v", mode, v.Rows)
		}
		raw := ImpactNumber(types.ImpactI4) * 20 // w times the E3 multiplier
		if r.Beta-r.Beta0 >= raw/2 {
			t.Errorf("%s: one fidelity event counted as %v observations", mode, r.Beta-r.Beta0)
		}
		if r.Evidence[E3] != 1 || !r.Breaker {
			t.Errorf("%s: E3 must set the breaker: %+v", mode, r)
		}
	}
}

func TestE3E4E6E7E8(t *testing.T) {
	in := Inputs{
		Refusals:  []Refusal{{Desk: "mail", Grant: "desk.net.egress", At: ts(0)}},
		OwnerActs: []OwnerAct{{Desk: "mail", Act: "approve", Class: ActionClass{Grant: "desk.net.egress", Impact: types.ImpactI3}, At: ts(time.Hour)}, {Desk: "mail", Act: "revert", Class: ActionClass{Grant: "desk.net.egress", Impact: types.ImpactI3}, At: ts(2 * time.Hour)}},
		Platform:  []PlatformEvent{{Package: "pkg-x", Publisher: "pub-x", Kind: PlatformYank, Classes: []ActionClass{{Grant: "desk.net.egress", Impact: types.ImpactI3}}, At: ts(3 * time.Hour)}},
		Costs:     []CostObservation{{Desk: "mail", Class: ActionClass{Grant: "desk.net.egress", Impact: types.ImpactI3}, Estimated: 1, Metered: 1.2, At: ts(4 * time.Hour)}},
		Resolver:  func(d string) SubjectInfo { return SubjectInfo{Package: "pkg-x", Publisher: "pub-x"} },
	}
	v := Derive(in, baseParams())
	cls := ActionClass{Grant: "desk.net.egress", Impact: types.ImpactI3}
	r, ok := row(v, SubjectAgent, "mail", cls)
	if !ok || r.Evidence[E4] != 1 || r.Evidence[E6] != 2 || r.Evidence[E8] != 1 {
		t.Fatalf("%+v", r)
	}
	p, ok := row(v, SubjectPackage, "pkg-x", cls)
	if !ok || p.Evidence[E7] != 1 || !p.Breaker {
		t.Fatalf("E7 at package level: %+v", p)
	}
	if _, ok := row(v, SubjectPublisher, "pub-x", cls); !ok {
		t.Fatal("E7 at publisher level")
	}
}

func TestLowerBoundAndDecay(t *testing.T) {
	if lcb := betaQuantile(0.05, 100, 1); lcb < 0.95 {
		t.Fatalf("lcb(100,1) = %v", lcb)
	}
	if a, b := betaQuantile(0.05, 10, 1), betaQuantile(0.05, 1000, 100); !(a < b) {
		t.Fatalf("sample size must fold into the bound: %v %v", a, b)
	}
	if q := betaQuantile(0.5, 1, 1); math.Abs(q-0.5) > 1e-6 {
		t.Fatalf("median of beta(1,1) = %v", q)
	}
	// decay: the same evidence an older time counts less
	rec := func(id string, at time.Duration) []types.CommitmentRecord {
		return []types.CommitmentRecord{subCast(id, "texture", "research_child", "m", at, types.ImpactI2, 0.5, yesNo(0.9)), judge(id, at+time.Minute, "yes")}
	}
	p := baseParams()
	p.Now = t0.Add(400 * 24 * time.Hour)
	old := Derive(Inputs{Records: rec("o", 0)}, p)
	p.Now = t0.Add(time.Hour)
	fresh := Derive(Inputs{Records: rec("o", 0)}, p)
	ro, _ := row(old, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2})
	rf, _ := row(fresh, SubjectAgent, "texture", ActionClass{Grant: "desk.request", Impact: types.ImpactI2})
	if !(ro.Alpha-ro.Alpha0 < rf.Alpha-rf.Alpha0) {
		t.Fatalf("decay: %v vs %v", ro.Alpha, rf.Alpha)
	}
}

func TestReplayReportsBothCandidatesAndPicksOne(t *testing.T) {
	var recs []types.CommitmentRecord
	// a desk that is mostly right, with sporadic failures; another that fails often
	rng := rand.New(rand.NewSource(7))
	for _, d := range []string{"mail", "texture"} {
		failRate := map[string]float64{"texture": 0.1, "mail": 0.5}[d]
		for i := 0; i < 60; i++ {
			id := d + string(rune('A'+i))
			at := time.Duration(i) * time.Hour
			recs = append(recs, subCast(id, d, "research_child", "m", at, types.ImpactI2, 0.5, yesNo(0.8)))
			ans := "yes"
			if rng.Float64() < failRate {
				ans = "no"
			}
			recs = append(recs, judge(id, at+time.Minute, ans))
		}
	}
	rep := Replay(Inputs{Records: recs}, baseParams())
	if len(rep.Candidates) < 2 {
		t.Fatalf("%+v", rep)
	}
	modes := map[WeightMode]bool{}
	for _, c := range rep.Candidates {
		modes[c.Mode] = true
		if c.N == 0 || math.IsNaN(c.Brier) || math.IsNaN(c.AUC) {
			t.Fatalf("candidate %+v", c)
		}
	}
	if !modes[ModeCapped] || !modes[ModeTempered] {
		t.Fatalf("both candidates must be reported: %+v", modes)
	}
	if rep.Chosen.Mode == "" {
		t.Fatal("the replay picks one")
	}
	// the sporadic failer is trusted more than the frequent failer
	if rep.Chosen.AUC < 0.5 {
		t.Fatalf("lower-bound ordering should predict next failure: auc %v", rep.Chosen.AUC)
	}
	// a replay on identical inputs reproduces
	rep2 := Replay(Inputs{Records: recs}, baseParams())
	if !reflect.DeepEqual(rep, rep2) {
		t.Fatal("replay is not reproducible")
	}
}

func TestRulePrecommitIsE1FromItsResolve(t *testing.T) {
	pre := types.CommitmentRecord{
		RecordID: "pc1", Kind: types.CommitmentKindPrecommit,
		Provenance: types.CommitmentProvenance{AgentID: "engineering:a1", Desk: "engineering", ModelID: "m", CommittedAt: ts(0)},
		Precommit:  &types.CommitmentPrecommit{Question: "tests pass", Distribution: map[string]float64{"yes": 0.9, "no": 0.1}, ImpactClass: types.ImpactI4},
	}
	res := types.CommitmentRecord{
		RecordID: "pc1:resolve", Kind: types.CommitmentKindResolve, ParentID: "pc1",
		Provenance: types.CommitmentProvenance{AgentID: "system:rule", CommittedAt: ts(time.Hour)},
		Resolve:    &types.CommitmentResolve{Verdict: "yes"},
	}
	v := Derive(Inputs{Records: []types.CommitmentRecord{pre, res}}, baseParams())
	r, ok := row(v, SubjectAgent, "engineering", ActionClass{Grant: "desk.request", Impact: types.ImpactI4})
	if !ok || r.Evidence[E1] != 1 {
		t.Fatalf("%+v %+v", r, v.Rows)
	}
	// a desk resolving its own precommit never settles it
	res.Provenance.AgentID = "engineering:a1"
	v = Derive(Inputs{Records: []types.CommitmentRecord{pre, res}}, baseParams())
	if len(v.Rows) != 0 || v.Excluded[0].Reason != ExcludedSelfScored {
		t.Fatalf("%+v %+v", v.Rows, v.Excluded)
	}
}
