package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// testRoundPredictions is a valid predictions block for tests that Cast
// engineering.
const testRoundPredictions = `{"verify_pass": 0.8, "judge_accepts": 0.7, "freeze_minutes": {"estimate": 20, "low": 10, "high": 40, "p": 0.75}}`

// withTestRoundPredictions adds the test predictions block to a JSON spec
// that states none.
func withTestRoundPredictions(t *testing.T, spec string) string {
	t.Helper()
	fields := map[string]json.RawMessage{}
	if strings.TrimSpace(spec) != "" {
		if err := json.Unmarshal([]byte(spec), &fields); err != nil {
			return spec
		}
	}
	if _, ok := fields[types.RoundPredictionsField]; !ok {
		fields[types.RoundPredictionsField] = json.RawMessage(testRoundPredictions)
	}
	out, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// P1: a Cast that states no predictions, or states them in a shape the
// runtime would drop, is refused with the shape to state them in.
func TestEngineeringCastWithoutPredictionsIsRefusedWithTheShape(t *testing.T) {
	f := newRebaseCastFixture(t)
	for name, spec := range map[string]string{
		"none":    f.spec(nil),
		"missing": withTestRoundPredictions(t, strings.TrimSuffix(f.spec(nil), "}")+`,"predictions":{"verify_pass":0.8,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.7}}}`),
		"typo":    strings.TrimSuffix(f.spec(nil), "}") + `,"predictions":{"verify_pass":0.8,"judge_accept":0.7,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.7}}}`,
		"dogma":   strings.TrimSuffix(f.spec(nil), "}") + `,"predictions":{"verify_pass":1,"judge_accepts":0.7,"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.7}}}`,
		"inverse": strings.TrimSuffix(f.spec(nil), "}") + `,"predictions":{"verify_pass":0.8,"judge_accepts":0.7,"freeze_minutes":{"estimate":60,"low":10,"high":40,"p":0.7}}}`,
	} {
		_, err := f.planRaw(t, spec, f.rebaseObjective())
		if err == nil || !strings.Contains(err.Error(), "states its predictions before the capsule starts") || !strings.Contains(err.Error(), types.RoundPredictionsHow) {
			t.Fatalf("%s: want the predictions refusal with its shape, got %v", name, err)
		}
	}
	if _, err := f.plan(t, f.spec(nil), f.rebaseObjective()); err != nil {
		t.Fatalf("a Cast with its predictions plans: %v", err)
	}
}

// P2, P4, R15, R17: the open derives the round's records from the Cast,
// anchored at the Cast's committed_at; the judgment settles judge_accepts and
// the Freeze settles freeze_minutes with the timestamps on the record.
func TestRoundPredictionsAreStatedAtTheOpenAndSettledByTheRuntime(t *testing.T) {
	f := newRebaseCastFixture(t)
	ctx := context.Background()
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(ctx, plan, records); err != nil {
		t.Fatalf("open: %v", err)
	}
	cast, err := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, plan.Cast.RecordID)
	if err != nil || cast == nil {
		t.Fatalf("cast record: %v", err)
	}
	stated, err := f.s.ListRulePrecommitRecords(ctx, f.owner, f.computer, plan.AssignmentID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[types.PrecommitRuleKind]types.CommitmentRecord{}
	for _, rec := range stated {
		kinds[rec.Precommit.Rule.Kind] = rec
		if rec.Precommit.Rule.Since != cast.Provenance.CommittedAt || rec.Provenance.AgentID != f.management || rec.SubjectRefs["cast"] != cast.RecordID {
			t.Fatalf("record %s: since %s (cast %s), author %s, refs %v", rec.RecordID, rec.Precommit.Rule.Since, cast.Provenance.CommittedAt, rec.Provenance.AgentID, rec.SubjectRefs)
		}
	}
	for _, kind := range types.RoundPredictionsMandatory {
		if _, ok := kinds[kind]; !ok {
			t.Fatalf("no %s record among %d", kind, len(stated))
		}
	}
	// A second derivation (the settlement pass's) writes nothing new.
	if err := f.rt.stateRoundPredictions(ctx, mustAssignment(t, f, plan.AssignmentID)); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.s.ListRulePrecommitRecords(ctx, f.owner, f.computer, plan.AssignmentID); len(again) != len(stated) {
		t.Fatalf("re-derivation changed the records: %d then %d", len(stated), len(again))
	}

	since, _ := parseCommitmentStamp(cast.Provenance.CommittedAt)
	frozenAt := since.Add(25 * time.Minute)
	freeze := types.EngineeringFreezeObservation{FrozenAt: frozenAt, BundleDigest: "sha256:" + strings.Repeat("a", 64), OperationID: "op-1"}
	f.rt.settleRulePrecommits(ctx, f.owner, f.computer, plan.AssignmentID, "freeze", frozenAt, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnFreeze(rule, freeze)
	})
	verdict := types.EngineeringVerifyObservation{Decision: "pass", BundleDigest: freeze.BundleDigest, VerifierRefs: []string{"ref-1"}, OperationID: "op-1", OperationState: "awaiting_approval"}
	f.rt.settleRulePrecommits(ctx, f.owner, f.computer, plan.AssignmentID, "verify", frozenAt.Add(time.Minute), func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnVerify(rule, verdict)
	})
	for kind, want := range map[types.PrecommitRuleKind]string{types.PrecommitRuleFreezeMinutes: "yes", types.PrecommitRuleJudgeAccepts: "yes", types.PrecommitRuleVerifyPass: "yes"} {
		resolve, err := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, types.RuleResolveRecordID(kinds[kind].RecordID))
		if err != nil || resolve == nil || resolve.Resolve == nil || resolve.Resolve.Verdict != want {
			t.Fatalf("%s: resolve %+v, %v", kind, resolve, err)
		}
		if kind == types.PrecommitRuleFreezeMinutes {
			refs := resolve.SubjectRefs
			if refs[types.RuleRefActualS] != "1500" || refs[types.RuleRefCastAt] != since.UTC().Format(time.RFC3339Nano) || refs[types.RuleRefFreezeAt] != frozenAt.UTC().Format(time.RFC3339Nano) {
				t.Fatalf("freeze_minutes resolve refs %v", refs)
			}
		}
	}
	// The calibration digest counts them on engineering's record.
	all, err := f.s.ListCommitmentRecords(ctx, f.owner, f.computer, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := collectCalibration(all, nil, nil)
	got := map[string]bool{}
	for _, o := range obs {
		if o.Desk == "engineering" && o.Thread == "engineering:"+plan.AssignmentID {
			got[o.ClaimType] = o.Y == 1
		}
	}
	for _, kind := range types.RoundPredictionsMandatory {
		if !got[string(kind)] {
			t.Fatalf("engineering observations %v lack a settled %s", got, kind)
		}
	}
}

// P3: the engineer's view of its Cast does not carry the predictions.
func TestEngineerViewDropsTheRoundPredictions(t *testing.T) {
	v := yaegikernel.InboxRecord{Body: withTestRoundPredictions(t, `{"parent":"cell:x"}`), Request: &yaegikernel.InboxRequest{RecordID: "cell:x", From: "texture:d", Objective: "o", Body: "b"}}
	requestIntoBody(&v)
	if strings.Contains(v.Body, "predictions") || strings.Contains(v.Body, "judge_accepts") || !strings.Contains(v.Body, `"parent":"cell:x"`) {
		t.Fatalf("engineer view: %s", v.Body)
	}
}

// P5: a Cast from before predictions derives no records.
func TestCastWithoutPredictionsBlockDerivesNothing(t *testing.T) {
	cast := types.CommitmentRecord{RecordID: "c", Directive: &types.CommitmentDirective{Body: `{"parent":"p"}`}}
	recs, err := roundPredictionRecords(cast, types.EngineeringAssignment{AssignmentID: "a"})
	if err != nil || len(recs) != 0 {
		t.Fatalf("old cast: %d records, %v", len(recs), err)
	}
}

func mustAssignment(t *testing.T, f *rebaseCastFixture, id string) types.EngineeringAssignment {
	t.Helper()
	a, err := f.s.GetEngineeringAssignment(context.Background(), f.owner, f.computer, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// live8 trace: every precommit refusal carries the shape to send.
func TestPrecommitRefusalsNameTheShape(t *testing.T) {
	for _, raw := range []string{`{"question":"q","resolver":"r"}`, `{"question":"q","distribution":"high","resolver":"r"}`, `{"p":0.5,"resolver":"r"}`, `{"question":"q","p":0.5}`} {
		_, err := typedCommitmentPrecommit(raw)
		if err == nil || !strings.Contains(err.Error(), types.PrecommitShapeExample) {
			t.Fatalf("%s: %v", raw, err)
		}
	}
	in := yaegikernel.StagedIntent{LocalID: "p1", Kind: yaegikernel.IntentPrecommit, Precommit: `{"question":"q","p":0.5,"resolver":"rule","rule":{"kind":"judge_accept"}}`}
	err := validateEngineeringPrecommit("engineering", in, []yaegikernel.StagedIntent{in})
	if err == nil || !strings.Contains(err.Error(), types.RulePrecommitClockExample) {
		t.Fatalf("rule refusal: %v", err)
	}
	ok := yaegikernel.StagedIntent{LocalID: "p2", Kind: yaegikernel.IntentPrecommit, Precommit: types.RulePrecommitClockExample}
	if err := validateEngineeringPrecommit("engineering", ok, []yaegikernel.StagedIntent{ok}); err != nil {
		t.Fatalf("the clock example is refused: %v", err)
	}
}
