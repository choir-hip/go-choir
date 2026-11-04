package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// testRoundPredictions is a valid predictions block for tests that Cast
// engineering.
const testRoundPredictions = `{"verify_pass": 0.8, "judge_accepts": 0.7, "owner_approves": 0.6, "freeze_minutes": {"estimate": 20, "low": 10, "high": 40, "p": 0.75}, "approval_minutes": {"estimate": 90, "low": 40, "high": 240, "p": 0.6}}`

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
		"typo":    strings.TrimSuffix(f.spec(nil), "}") + `,"predictions":{"verify_pass":0.8,"judge_accept":0.7,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.7}}}`,
		"dogma":   strings.TrimSuffix(f.spec(nil), "}") + `,"predictions":{"verify_pass":1,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":20,"low":10,"high":40,"p":0.7}}}`,
		"inverse": strings.TrimSuffix(f.spec(nil), "}") + `,"predictions":{"verify_pass":0.8,"judge_accepts":0.7,"owner_approves":0.6,"approval_minutes":{"estimate":90,"low":40,"high":240,"p":0.6},"freeze_minutes":{"estimate":60,"low":10,"high":40,"p":0.7}}}`,
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
		if want := f.rt.calibrationArmOfThread(ctx, f.computer, "engineering:"+plan.AssignmentID); rec.SubjectRefs[calibrationArmRef] != want {
			t.Fatalf("record %s arm %q, want %q", rec.RecordID, rec.SubjectRefs[calibrationArmRef], want)
		}
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
	// The owner's approval settles owner_approves and approval_minutes,
	// naming the actor (S1).
	op := selfdev.Operation{OperationID: "op-1", ComputerID: f.computer, IdempotencyKey: store.EngineeringFreezeIdempotencyKey(plan.AssignmentID), DecisionActor: f.owner}
	f.rt.settleOwnerActPrecommits(ctx, f.owner, op, types.OwnerActApprove)
	for _, kind := range []types.PrecommitRuleKind{types.PrecommitRuleOwnerApproves, types.PrecommitRuleApprovalMinutes} {
		resolve, err := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, types.RuleResolveRecordID(kinds[kind].RecordID))
		if err != nil || resolve == nil || resolve.Resolve == nil || resolve.SubjectRefs[types.RuleRefDelegated] != "false" || resolve.SubjectRefs[types.RuleRefDecisionActor] != f.owner {
			t.Fatalf("%s: resolve %+v, %v", kind, resolve, err)
		}
		if kind == types.PrecommitRuleOwnerApproves && resolve.Resolve.Verdict != "yes" {
			t.Fatalf("owner_approves: %s", resolve.Resolve.Verdict)
		}
	}
	// The calibration digest counts them on engineering's record.
	all, err := f.s.ListCommitmentRecords(ctx, f.owner, f.computer, "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	obs, _ := collectCalibration(f.rt.calibrationSalt(ctx, f.computer), all, nil, nil)
	got := map[string]bool{}
	for _, o := range obs {
		if o.Desk == "engineering" && o.Thread == "engineering:"+plan.AssignmentID {
			got[o.ClaimType] = o.Y == 1
		}
	}
	// Approved at once, outside the 40-240 minute interval: settled "no".
	want := map[string]bool{"verify_pass": true, "judge_accepts": true, "owner_approves": true, "freeze_minutes": true, "approval_minutes": false}
	for _, kind := range types.RoundPredictionsMandatory {
		if y, ok := got[string(kind)]; !ok || y != want[string(kind)] {
			t.Fatalf("engineering observations %v: %s settled %v (present %v)", got, kind, y, ok)
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

// R20 (live8: "Freeze within 40 minutes" staged 21 s before its Freeze):
// every rule precommit records stated_at; a clock claim after the run's
// first build is kept, marked late, and never scored; an outcome claim is
// not late before the Freeze; the late share is counted per stating desk.
func TestLateClockPrecommitIsKeptMarkedAndNeverScored(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-late", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	r := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, rec: &run, scope: ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID, FromRole: agentprofile.Engineering, ChannelID: run.AgentID,
		RunID: run.RunID, OwnerID: seed.OwnerID, ComputerID: seed.ComputerID, CellID: "cell-late-1",
	}}
	early := rulePrecommitIntent(t, "early", 0.7, map[string]any{"kind": "freeze_minutes", "estimate_minutes": 20, "low_minutes": 10, "high_minutes": 40})
	if _, err := r.commitEngineeringPrecommit(ctx, early); err != nil {
		t.Fatal(err)
	}
	rt.jobTable = &fakeJobTable{view: yaegikernel.JobView{ID: "job-1", Argv: []string{"go", "build", "./..."}}}
	r.scope.CellID = "cell-late-2"
	late := rulePrecommitIntent(t, "late", 0.9, map[string]any{"kind": "freeze_within", "within_minutes": 40})
	outcome := rulePrecommitIntent(t, "outcome", 0.8, map[string]any{"kind": "tests_pass"})
	for _, in := range []yaegikernel.StagedIntent{late, outcome} {
		if _, err := r.commitEngineeringPrecommit(ctx, in); err != nil {
			t.Fatalf("%s: %v", in.LocalID, err)
		}
	}
	listed, err := s.ListRulePrecommitRecords(ctx, seed.OwnerID, seed.ComputerID, assignment.AssignmentID)
	if err != nil || len(listed) != 3 {
		t.Fatalf("listed %d, %v", len(listed), err)
	}
	lateKinds := map[types.PrecommitRuleKind]bool{}
	for _, rec := range listed {
		if rec.SubjectRefs[types.RuleRefStatedAt] == "" || rec.SubjectRefs[types.RuleRefStatedBy] != "precommit" {
			t.Fatalf("%s: no stated_at: %v", rec.RecordID, rec.SubjectRefs)
		}
		lateKinds[rec.Precommit.Rule.Kind] = types.LateRulePrecommit(rec)
	}
	if lateKinds[types.PrecommitRuleFreezeMinutes] || !lateKinds[types.PrecommitRuleFreezeWithin] || lateKinds[types.PrecommitRuleTestsPass] {
		t.Fatalf("late marks %v: want only the clock claim after the build", lateKinds)
	}
	all, _ := s.ListCommitmentRecords(ctx, seed.OwnerID, seed.ComputerID, "", 500)
	if sd := types.RulePrecommitSayDo(all, "engineering:"); sd.Late != 1 || sd.Overall.Committed != 2 {
		t.Fatalf("say-do %+v: the late claim must be counted apart, not scored", sd)
	}
	share := calibrationLateShares(all)["engineering"]
	if share.Stated != 3 || share.Late != 1 {
		t.Fatalf("late share %+v", share)
	}
	_, pending := collectCalibration(rt.calibrationSalt(ctx, seed.ComputerID), all, nil, nil)
	for _, p := range pending {
		if p.ClaimType == string(types.PrecommitRuleFreezeWithin) {
			t.Fatal("the late claim is pending in the digest")
		}
	}
}

// C1-C3: the claims read lists the round's rule precommits stated against
// what happened, and refuses another computer's id.
func TestCalibrationClaimsListStatedAgainstActual(t *testing.T) {
	f := newRebaseCastFixture(t)
	ctx := context.Background()
	plan, err := f.plan(t, f.spec(nil), f.rebaseObjective())
	if err != nil {
		t.Fatal(err)
	}
	records := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:" + plan.AssignmentID, Records: []types.CommitmentRecord{plan.Cast}}
	records.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(records)
	if _, err := f.rt.openDelegatedCastAssignment(ctx, plan, records); err != nil {
		t.Fatal(err)
	}
	cast, _ := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, plan.Cast.RecordID)
	since, _ := parseCommitmentStamp(cast.Provenance.CommittedAt)
	frozenAt := since.Add(35 * time.Minute)
	f.rt.settleRulePrecommits(ctx, f.owner, f.computer, plan.AssignmentID, "freeze", frozenAt, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnFreeze(rule, types.EngineeringFreezeObservation{FrozenAt: frozenAt, OperationID: "op-9"})
	})
	for _, assignment := range []string{plan.AssignmentID, ""} {
		claims, err := f.rt.calibrationClaims(ctx, f.owner, f.computer, assignment)
		if err != nil || len(claims) != len(types.RoundPredictionsMandatory) {
			t.Fatalf("assignment %q: %d claims, %v", assignment, len(claims), err)
		}
		for _, c := range claims {
			if c.Desk != "management" || c.StatedBy != "cast" || c.Late || c.Cast != cast.RecordID || c.Assignment != plan.AssignmentID || c.StatedAt != cast.Provenance.CommittedAt {
				t.Fatalf("claim %+v", c)
			}
			if c.Kind == string(types.PrecommitRuleFreezeMinutes) {
				if c.Answer != "yes" || c.ActualS == nil || *c.ActualS != 2100 || c.Estimate != 20 || c.Low != 10 || c.High != 40 || c.P != 0.75 || c.Operation != "op-9" || c.SettledAt == "" {
					t.Fatalf("freeze_minutes claim %+v", c)
				}
			} else if c.Answer != "" {
				t.Fatalf("%s settled before its event: %+v", c.Kind, c)
			}
		}
	}
	h := &APIHandler{rt: f.rt}
	req := httptest.NewRequest(http.MethodGet, "/api/calibration/claims?computer_id=computer-elsewhere", nil)
	req.Header.Set("X-Authenticated-User", f.owner)
	rec := httptest.NewRecorder()
	h.HandleCalibrationClaims(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("another computer's id: %d %s", rec.Code, rec.Body)
	}
}
