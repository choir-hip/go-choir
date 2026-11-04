package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// F6: no E1-E setting is push; one is that delivery; two is push again.
func TestCalibrationDeliveryIsTheOneArmTheComputerHolds(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "user-alice", rt.TextureComputerID()
	if got := rt.calibrationDeliveryFor(ctx, owner, computer); got != calibrationDeliveryPush {
		t.Fatalf("no setting: %s", got)
	}
	put := func(key string, v bool) {
		if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: key, Value: v}); err != nil {
			t.Fatal(err)
		}
	}
	put(store.SettingCalibrationE1Pull, true)
	if got := rt.calibrationDeliveryFor(ctx, owner, computer); got != calibrationDeliveryPull {
		t.Fatalf("pull: %s", got)
	}
	put(store.SettingCalibrationE1Stub, true)
	if got := rt.calibrationDeliveryFor(ctx, owner, computer); got != calibrationDeliveryPush {
		t.Fatalf("two arms: %s, want the default", got)
	}
	if calibrationArmForDelivery(calibrationDeliveryPull, "control") != calibration.ArmDigest || calibrationArmForDelivery(calibrationDeliveryBaseline, "digest") != calibration.ArmControl ||
		calibrationArmForDelivery(calibrationDeliveryPush, "control") != "control" {
		t.Fatal("arm labels by delivery")
	}
}

// E1-M, F1-F4: a Cast restatement is asked only on a computer in E1-M, is
// refused in the Cast's own cell, writes one p1 per p0 with the same rule
// and anchor, names its p0 and whether the record was shown, is refused a
// second time, and is no observation.
func TestCastRestatementWritesP1BesideP0(t *testing.T) {
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
	castID := plan.Cast.RecordID
	if notice := f.rt.castRestateNotice(ctx, f.owner, f.computer, castID); notice != "" {
		t.Fatalf("a computer outside E1-M was asked: %q", notice)
	}
	body := strings.ReplaceAll(testRoundPredictions, "0.", "0.4") // a different statement
	reduction := func(cell string) *rlmCallReduction {
		return &rlmCallReduction{active: true, mb: f.rt, st: f.s, ledger: f.s,
			rec:   &types.RunRecord{RunID: "run-management-2", AgentProfile: agentprofile.Management},
			scope: ReductionScope{FromAgentID: f.management, OwnerID: f.owner, ComputerID: f.computer, RunID: "run-management-2", CellID: cell, ModelID: "m/x"}}
	}
	in := yaegikernel.StagedIntent{LocalID: "r1", Kind: yaegikernel.IntentRestate, TargetRef: castID, Body: body}
	if _, err := reduction("cell-later").commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "no restatement is asked") {
		t.Fatalf("outside E1-M: %v", err)
	}
	if err := f.s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer, Key: store.SettingCalibrationE1Restate, Value: true}); err != nil {
		t.Fatal(err)
	}
	notice := f.rt.castRestateNotice(ctx, f.owner, f.computer, castID)
	shown := f.rt.calibrationArmOfCast(ctx, f.computer, castID) == calibration.ArmDigest
	if !strings.Contains(notice, "choir.Restate(\""+castID+"\"") || strings.Contains(notice, "Your record") != shown {
		t.Fatalf("notice (record shown %v): %q", shown, notice)
	}
	castCell, _, _ := strings.Cut(castID, ":")
	if _, err := reduction(castCell).commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "later cell") {
		t.Fatalf("the Cast's own cell: %v", err)
	}
	if _, err := reduction("cell-later").commitRestateIntent(ctx, in); err != nil {
		t.Fatalf("restate: %v", err)
	}
	stated, err := f.s.ListRulePrecommitRecords(ctx, f.owner, f.computer, plan.AssignmentID)
	if err != nil {
		t.Fatal(err)
	}
	p0s, p1s := map[string]types.CommitmentRecord{}, map[string]types.CommitmentRecord{}
	for _, rec := range stated {
		if calibrationRestatement(rec) {
			p1s[rec.SubjectRefs[types.RuleRefRestates]] = rec
		} else {
			p0s[rec.RecordID] = rec
		}
	}
	if len(p1s) != len(types.RoundPredictionsMandatory) {
		t.Fatalf("%d restatements, want one per mandatory kind", len(p1s))
	}
	for id, p1 := range p1s {
		p0, ok := p0s[id]
		if !ok || p0.SubjectRefs[types.RuleRefStage] != types.RuleStageFirst {
			t.Fatalf("p1 %s restates %s, which is no p0", p1.RecordID, id)
		}
		if p1.Precommit.Rule.Since != p0.Precommit.Rule.Since || p1.Precommit.Rule.AssignmentID != p0.Precommit.Rule.AssignmentID || p1.Precommit.Rule.Kind != p0.Precommit.Rule.Kind {
			t.Fatalf("p1 %s rule %+v, p0 %+v", p1.RecordID, p1.Precommit.Rule, p0.Precommit.Rule)
		}
		if p1.SubjectRefs[types.RuleRefStatedBy] != "restate" || p1.SubjectRefs[calibrationRecordShownRef] != map[bool]string{true: "true", false: "false"}[shown] ||
			p1.SubjectRefs[calibrationArmRef] != p0.SubjectRefs[calibrationArmRef] || types.LateRulePrecommit(p1) {
			t.Fatalf("p1 refs %v", p1.SubjectRefs)
		}
	}
	if _, err := reduction("cell-third").commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "restated once") {
		t.Fatalf("a second restatement: %v", err)
	}
	if _, err := reduction("cell-later").commitRestateIntent(ctx, in); err != nil {
		t.Fatalf("a replayed cell: %v", err)
	}
	// Both settle on the same event; only p0 is an observation.
	frozenAt := time.Now().Add(20 * time.Minute)
	freeze := types.EngineeringFreezeObservation{FrozenAt: frozenAt, BundleDigest: "sha256:" + strings.Repeat("a", 64), OperationID: "op-1"}
	f.rt.settleRulePrecommits(ctx, f.owner, f.computer, plan.AssignmentID, "freeze", frozenAt, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnFreeze(rule, freeze)
	})
	for id, p1 := range p1s {
		if p0s[id].Precommit.Rule.Kind != types.PrecommitRuleFreezeMinutes {
			continue
		}
		for _, rid := range []string{id, p1.RecordID} {
			if ok, _ := f.s.CommitmentRecordExists(ctx, f.owner, f.computer, types.RuleResolveRecordID(rid)); !ok {
				t.Fatalf("%s did not settle on the Freeze", rid)
			}
		}
	}
	all, _ := f.s.ListCommitmentRecords(ctx, f.owner, f.computer, "", 1000)
	obs, pending := collectCalibration(f.rt.calibrationSalt(ctx, f.computer), all, nil, nil)
	for _, o := range obs {
		if strings.HasSuffix(o.RecordID, ":restate") {
			t.Fatalf("a restatement is an observation: %+v", o)
		}
	}
	for _, p := range pending {
		if strings.HasSuffix(p.RecordID, ":restate") {
			t.Fatalf("a restatement is pending: %+v", p)
		}
	}
}

// KindRecord shows a stater its own settled, shown claims per kind, and
// says so when a kind has too few; the stub is short and names the verb.
func TestKindRecordAndStub(t *testing.T) {
	var obs []calibration.Observation
	for i := 0; i < 6; i++ {
		obs = append(obs, calibration.Observation{Desk: "engineering", ClaimType: "verify_pass", StatedBy: "cast", P: 0.8, Y: float64(i % 2), Key: "k" + string(rune('a'+i))})
	}
	obs = append(obs, calibration.Observation{Desk: "engineering", ClaimType: "verify_pass", StatedBy: "precommit", P: 0.1, Y: 1, Key: "other"},
		calibration.Observation{Desk: "engineering", ClaimType: "verify_pass", StatedBy: "cast", P: 0.1, Y: 1, Key: "held", HeldOut: true})
	text := calibration.KindRecord(obs, "engineering", "cast", []string{"verify_pass", "judge_accepts"}, 5)
	if !strings.Contains(text, "verify_pass (n=6): you said 0.80 on average and 0.50 held") || !strings.Contains(text, "judge_accepts: 0 settled, too few to show") {
		t.Fatalf("kind record:\n%s", text)
	}
	d := calibration.Compute("engineering", append(obs, obs...), calibration.Options{MinN: 5})
	if stub := calibration.Stub(d); stub == "" || !strings.Contains(stub, "choir.Calibration()") || len(strings.Fields(stub)) > 40 {
		t.Fatalf("stub: %q", stub)
	}
}

// E1-E two-stage, F2, F4: the engineer is asked once per kind after the
// cell that stated it; it restates only what was asked, with {"p": ...}
// (and minutes for a clock kind), once; p1 keeps p0's rule anchor.
func TestEngineerRestatementIsAskedOnceAndRecordedBesideP0(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "user-alice", rt.TextureComputerID()
	run := &types.RunRecord{RunID: "run-eng", AgentID: "engineering:a1", AgentProfile: agentprofile.Engineering, OwnerID: owner, ComputerID: computer,
		Metadata: map[string]any{"assignment_id": "a1"}}
	p0 := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-1:precommit:p1", Kind: types.CommitmentKindPrecommit, EmitterKind: types.CommitmentEmitterAgent,
		Precommit: &types.CommitmentPrecommit{Question: "the tests pass", Distribution: map[string]float64{"yes": 0.8, "no": 0.2}, Resolver: types.PrecommitRuleResolver,
			Rule: &types.PrecommitRule{Kind: types.PrecommitRuleTestsPass, AssignmentID: "a1", Since: "2026-11-04T01:00:00Z"}},
		Provenance:  types.CommitmentProvenance{AgentID: "engineering:a1", CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		ComputerID:  computer,
		SubjectRefs: map[string]string{"assignment": "a1", types.RuleRefStatedBy: "precommit", types.RuleRefStage: types.RuleStageFirst}}
	// An earlier attempt's claim of the same kind, older: the restatement
	// never binds to it (review r1).
	older := p0
	older.RecordID = "cell-0:precommit:p1"
	older.Provenance.CommittedAt = "2026-11-04T00:00:00Z"
	older.SubjectRefs = map[string]string{"assignment": "a1", types.RuleRefStatedBy: "precommit", types.RuleRefStage: types.RuleStageFirst}
	for _, rec := range []types.CommitmentRecord{older, p0} {
		if _, err := s.AppendCommitmentRecord(ctx, owner, computer, rec); err != nil {
			t.Fatal(err)
		}
	}
	r := &rlmCallReduction{active: true, mb: rt, st: s, ledger: s, rec: run,
		scope: ReductionScope{FromAgentID: "engineering:a1", OwnerID: owner, ComputerID: computer, RunID: run.RunID, CellID: "cell-2", ModelID: "m/x"}, restateKinds: []restateAsk{{kind: "tests_pass", p0: p0.RecordID}}}
	if notice := rt.engineerRestateNotice(ctx, r); notice != "" {
		t.Fatalf("a push computer was asked: %q", notice)
	}
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingCalibrationE1TwoStage, Value: true}); err != nil {
		t.Fatal(err)
	}
	in := yaegikernel.StagedIntent{LocalID: "r1", Kind: yaegikernel.IntentRestate, TargetRef: "tests_pass", Body: `{"p": 0.6}`}
	if _, err := r.commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "was asked") {
		t.Fatalf("before the ask: %v", err)
	}
	if notice := rt.engineerRestateNotice(ctx, r); !strings.Contains(notice, "tests_pass") {
		t.Fatalf("notice: %q", notice)
	}
	if again := rt.engineerRestateNotice(ctx, r); again != "" {
		t.Fatalf("asked twice: %q", again)
	}
	if _, err := r.commitRestateIntent(ctx, yaegikernel.StagedIntent{LocalID: "r0", Kind: yaegikernel.IntentRestate, TargetRef: "tests_pass", Body: `{"p": 0.6, "estimate_minutes": 3}`}); err == nil {
		t.Fatal("an outcome kind restated minutes")
	}
	if _, err := r.commitRestateIntent(ctx, in); err != nil {
		t.Fatalf("restate: %v", err)
	}
	if _, err := r.commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "this cell already restated") {
		t.Fatalf("a second Restate in one cell: %v", err)
	}
	if ok, _ := s.CommitmentRecordExists(ctx, owner, computer, older.RecordID+":restate"); ok {
		t.Fatal("the restatement bound to another attempt's claim")
	}
	p1, err := s.GetCommitmentRecord(ctx, owner, computer, p0.RecordID+":restate")
	if err != nil || p1 == nil {
		t.Fatalf("p1: %v", err)
	}
	if p1.Precommit.YesProbability() != 0.6 || p1.Precommit.Rule.Since != p0.Precommit.Rule.Since || p1.SubjectRefs[types.RuleRefRestates] != p0.RecordID ||
		p1.SubjectRefs[calibrationDeliveryRef] != calibrationDeliveryTwoStage || p1.SubjectRefs[calibrationRecordShownRef] == "" {
		t.Fatalf("p1 %+v refs %v", p1.Precommit, p1.SubjectRefs)
	}
	replay := &rlmCallReduction{active: true, mb: rt, st: s, ledger: s, rec: run, scope: r.scope}
	if _, err := replay.commitRestateIntent(ctx, in); err != nil {
		t.Fatalf("a replayed cell: %v", err)
	}
	replay.scope.CellID = "cell-3"
	if _, err := replay.commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "restated once") {
		t.Fatalf("twice: %v", err)
	}
}

// Review r3: a restatement of a first statement that already settled (a
// stale Cast id) is refused, never written to stay open for ever.
func TestRestatementOfASettledClaimIsRefused(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "user-alice", rt.TextureComputerID()
	run := &types.RunRecord{RunID: "run-eng", AgentID: "engineering:a1", AgentProfile: agentprofile.Engineering, OwnerID: owner, ComputerID: computer, Metadata: map[string]any{"assignment_id": "a1"}}
	p0 := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "cell-1:precommit:p1", Kind: types.CommitmentKindPrecommit, EmitterKind: types.CommitmentEmitterAgent,
		Precommit: &types.CommitmentPrecommit{Question: "q", Distribution: map[string]float64{"yes": 0.8, "no": 0.2}, Resolver: types.PrecommitRuleResolver,
			Rule: &types.PrecommitRule{Kind: types.PrecommitRuleTestsPass, AssignmentID: "a1", Since: "2026-11-04T01:00:00Z"}},
		Provenance: types.CommitmentProvenance{AgentID: "engineering:a1", CommittedAt: "2026-11-04T01:00:00Z"}, ComputerID: computer,
		SubjectRefs: map[string]string{"assignment": "a1", types.RuleRefStatedBy: "precommit", types.RuleRefStage: types.RuleStageFirst}}
	settled := types.NewRuleResolveRecord(p0, types.RuleSettlement{Answer: "yes"}, time.Now())
	for _, rec := range []types.CommitmentRecord{p0, settled} {
		if _, err := s.AppendCommitmentRecord(ctx, owner, computer, rec); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: owner, ComputerID: computer, Key: store.SettingCalibrationE1TwoStage, Value: true}); err != nil {
		t.Fatal(err)
	}
	r := &rlmCallReduction{active: true, mb: rt, st: s, ledger: s, rec: run,
		scope: ReductionScope{FromAgentID: "engineering:a1", OwnerID: owner, ComputerID: computer, RunID: run.RunID, CellID: "cell-2"}, restateKinds: []restateAsk{{kind: "tests_pass", p0: p0.RecordID}}}
	rt.engineerRestateNotice(ctx, r)
	in := yaegikernel.StagedIntent{LocalID: "r1", Kind: yaegikernel.IntentRestate, TargetRef: "tests_pass", Body: `{"p": 0.6}`}
	if _, err := r.commitRestateIntent(ctx, in); err == nil || !strings.Contains(err.Error(), "already settled") {
		t.Fatalf("restating a settled claim: %v", err)
	}
	if ok, _ := s.CommitmentRecordExists(ctx, owner, computer, p0.RecordID+":restate"); ok {
		t.Fatal("a p1 was written for a settled p0")
	}
}
