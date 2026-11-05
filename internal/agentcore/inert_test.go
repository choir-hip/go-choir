package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func loadInertFixture(t *testing.T) []types.CommitmentRecord {
	t.Helper()
	raw, err := os.ReadFile("testdata/inert_de_records.json")
	if err != nil {
		t.Fatal(err)
	}
	var recs []types.CommitmentRecord
	if err := json.Unmarshal(raw, &recs); err != nil {
		t.Fatal(err)
	}
	return recs
}

func replayInertFixture(t *testing.T, owner string) (*Runtime, string) {
	t.Helper()
	rt, s := testRuntime(t)
	for _, rec := range loadInertFixture(t) {
		if _, err := s.AppendCommitmentRecord(context.Background(), owner, "computer-de", rec); err != nil {
			t.Fatalf("replay %s: %v", rec.RecordID, err)
		}
	}
	return rt, owner
}

// A replay of records shaped like d@e's yields the rates per desk, model and
// prompt version, and another owner's records never enter the view.
func TestInertReplayOfDeRecordsYieldsPerDeskRates(t *testing.T) {
	rt, _ := replayInertFixture(t, "owner-de")
	// Another owner on the same store has its own inert records.
	for _, rec := range loadInertFixture(t)[:3] {
		rec.RecordID = "other:" + rec.RecordID
		if _, err := rt.store.AppendCommitmentRecord(context.Background(), "owner-other", "computer-de", rec); err != nil {
			t.Fatal(err)
		}
	}
	rows, agree, err := rt.InertRates(context.Background(), "owner-de", "computer-de")
	if err != nil {
		t.Fatal(err)
	}
	type want struct{ pre, inertPre, del, inertDel int }
	wants := map[string]want{
		"management|gpt-5.4-mini|pv-a":      {1, 1, 1, 0},
		"management|gpt-5.4-mini|pv-b":      {5, 1, 1, 0},
		"management|gpt-5.4-mini|":          {1, 1, 1, 0}, // written before the prompt stamp
		"research|deepseek-v4.1-flash|pv-a": {4, 2, 5, 1},
	}
	if len(rows) != len(wants) {
		t.Fatalf("rows: %+v", rows)
	}
	for _, r := range rows {
		w, ok := wants[r.Desk+"|"+r.Model+"|"+r.PromptVersion]
		if !ok || r.Precommitments != w.pre || r.InertPrecommitments != w.inertPre || r.Delegations != w.del || r.InertDelegations != w.inertDel {
			t.Fatalf("row %+v, want %+v", r, w)
		}
	}
	if rows[3].Desk != "research" || rows[3].InertPrecommitRate != 0.5 || rows[3].InertDelegateRate != 0.2 ||
		rows[3].ByFlag[types.InertNearCertain] != 1 || rows[3].ByFlag[types.InertTrivialTemplate] != 1 || rows[3].ByFlag[types.InertWeightBelowFloor] != 1 {
		t.Fatalf("research row: %+v", rows[3])
	}
	if agree.Sampled != 0 {
		t.Fatalf("agreement with no judgements: %+v", agree)
	}
	// A replay gives the same view.
	again, _, _ := rt.InertRates(context.Background(), "owner-de", "computer-de")
	if fmt.Sprint(rows) != fmt.Sprint(again) {
		t.Fatalf("replay differs")
	}
	// The other owner's view is only its own three records.
	other, _, _ := rt.InertRates(context.Background(), "owner-other", "computer-de")
	total := 0
	for _, r := range other {
		total += r.Delegations
	}
	if total != 3 {
		t.Fatalf("other owner's view: %+v", other)
	}
}

// The digest shows the inert count: "N of your last M precommitments were
// uninformative", computed from the desk's own records.
func TestInertDigestShowsTheInertCount(t *testing.T) {
	rt, owner := replayInertFixture(t, "owner-de")
	snap, err := rt.calibrationSnapshotFor(context.Background(), owner, "computer-de", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if c := snap.Inert["management"]; c.N != 3 || c.M != 7 {
		t.Fatalf("management inert count %+v, want 3 of 7", c)
	}
	if _, ok := snap.Inert["research"]; ok {
		t.Fatalf("research has only 4 stated predictions, under the minimum: %+v", snap.Inert)
	}
	var obs []calibration.Observation
	for i := 0; i < 25; i++ {
		obs = append(obs, calibration.Observation{Desk: "management", ClaimType: types.CalibrationClaimHypothesis, Key: fmt.Sprint("k", i),
			Thread: "t", RecordID: fmt.Sprint("r", i), P: 0.7, Y: float64(i % 2), CommittedAt: fmt.Sprintf("2026-11-04T10:%02d:00Z", i)})
	}
	d := calibration.Compute("management", obs, snap.digestOptions("management", "m"))
	if d.Inert == nil || d.Inert.N != 3 || d.Inert.M != 7 || strings.Contains(d.Text, "uninformative") {
		t.Fatalf("digest: inert %+v (computed and stored, not rendered)\n%s", d.Inert, d.Text)
	}
	// A desk with no inert precommitment sees no such line.
	d = calibration.Compute("management", obs, calibration.Options{Inert: &types.CalibrationInert{N: 0, M: 9}})
	if strings.Contains(d.Text, "uninformative") {
		t.Fatalf("a clean record is told it is uninformative:\n%s", d.Text)
	}
}

func inertReductionScope(rt *Runtime, cell string) ReductionScope {
	scope := testReductionScope()
	scope.CellID = cell
	scope.ComputerID = rt.TextureComputerID()
	scope.FromAgentID, scope.DeskAgentID, scope.FromRole = "research:doc-inert", "research:doc-inert", "research"
	scope.ModelID = "m-inert"
	return scope
}

// sampledLocalID finds a local id whose record the sampler picks.
func sampledLocalID(scope ReductionScope, kind string, flagged bool) string {
	for i := 0; ; i++ {
		local := fmt.Sprintf("a%d", i)
		if types.InertJudgeSampled(fmt.Sprintf("%s:%s:%s", scope.CellID, kind, local), flagged) {
			return local
		}
	}
}

// Nothing is refused for being inert: a Cast that is trivial, near-certain
// and has no falsifier commits, is flagged on the record, and is delivered;
// and the sampled judge, held here, never blocks the cell.
func TestInertCastStillCommitsAndRunsAndTheJudgeNeverBlocksIt(t *testing.T) {
	rt, _ := testRuntime(t)
	oldSpacing := subCallJudgeSpacing
	subCallJudgeSpacing = 0
	t.Cleanup(func() { subCallJudgeSpacing = oldSpacing })
	judge := &subCallJudge{hold: make(chan struct{}), started: make(chan struct{}, 1), pick: map[string]string{"p0.contradictable": "no"}}
	rt.SetCandidateJudge(judge)
	scope := inertReductionScope(rt, "cell-inert-cast")
	// Flagged (no falsifier, near-certain) and sampled at the control rate.
	local := sampledLocalID(scope, yaegikernel.IntentCast, true)
	p, w := 0.995, 0.4
	intent := yaegikernel.StagedIntent{LocalID: local, Kind: yaegikernel.IntentCast, ToDesk: "management", Objective: "The child returns a result",
		Req: &types.RequestFields{PSuccess: &p, Weight: &w, Settlement: types.SettlementRecipient}}
	ctx := testReductionCtx(scope)
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope}
	done := make(chan error, 1)
	go func() { done <- reduction.commit(ctx, []yaegikernel.StagedIntent{intent}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("an inert Cast was refused: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the cell's commit waited on the sampled judge")
	}
	if !reduction.receipt.Committed || len(reduction.receipt.Intents) != 1 {
		t.Fatalf("receipt %+v", reduction.receipt)
	}
	recordID := fmt.Sprintf("%s:%s:%s", scope.CellID, yaegikernel.IntentCast, local)
	rec, err := rt.store.GetCommitmentRecord(ctx, scope.OwnerID, scope.ComputerID, recordID)
	if err != nil || rec == nil {
		t.Fatalf("the inert Cast has no record: %v", err)
	}
	for _, f := range []string{types.InertNoFalsifier, types.InertNearCertain} {
		if !types.HasFlag(rec.InertFlags, f) {
			t.Fatalf("flags %v lack %s", rec.InertFlags, f)
		}
	}
	if rec.Provenance.PromptVersion == "" || rec.PSuccess == nil {
		t.Fatalf("record: %+v", rec.Provenance)
	}
	// It ran: the Cast was delivered to its desk.
	if msgs, _, err := rt.ChannelRead(scope.ChannelID, 0); err != nil || len(msgs) == 0 {
		t.Fatalf("the inert Cast was not delivered: %d %v", len(msgs), err)
	}
	// The judge was asked, asynchronously; release it and read the judgement.
	select {
	case <-judge.started:
	case <-time.After(20 * time.Second):
		t.Fatal("the sampled judge was never asked")
	}
	close(judge.hold)
	rt.waitSubCallJudge()
	j, err := rt.store.GetCommitmentRecord(ctx, scope.OwnerID, scope.ComputerID, types.InertJudgementRecordID(recordID))
	if err != nil || j == nil || j.SubCall == nil || j.SubCall.Inert == nil {
		t.Fatalf("no judgement: %v %+v", err, j)
	}
	if got := j.SubCall.Inert; !got.Flagged || !got.JudgeInert || !got.Agree || got.Contradictable != 0.2 {
		t.Fatalf("judgement: %+v", got)
	}
}

// The judge's sample is scored against the rules: agreement is computed from
// the judgement records, both ways, and the judge never sees who wrote the
// prediction, its model, desk or weight.
func TestInertJudgeSampleIsScoredAgainstTheRules(t *testing.T) {
	rt, s := testRuntime(t)
	oldSpacing := subCallJudgeSpacing
	subCallJudgeSpacing = 0
	t.Cleanup(func() { subCallJudgeSpacing = oldSpacing })
	// p0 is the flagged record, p1 the unflagged, p2 a flagged one the judge
	// calls informative, p3 an unflagged one it calls informative.
	judge := &subCallJudge{pick: map[string]string{"p0.contradictable": "no", "p1.contradictable": "no", "p2.contradictable": "yes", "p3.contradictable": "yes"}}
	rt.SetCandidateJudge(judge)
	scope := inertReductionScope(rt, "cell-inert-score")
	mk := func(i int, flagged bool) types.CommitmentRecord {
		var id string
		for n := 0; ; n++ {
			id = fmt.Sprintf("score-%d-%d", i, n)
			if types.InertJudgeSampled(id, flagged) {
				break
			}
		}
		w := 0.4123
		rec := types.CommitmentRecord{RecordID: id, Kind: types.CommitmentKindDirective, Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "Check the ledger totals"},
			Prediction: types.CommitmentPrediction{Hypothesis: "Check the ledger totals", CommittedAt: "2026-11-04T10:00:00Z", Questions: []types.TypedQuestion{{Question: "Will the totals match?", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.6, "no": 0.4}}}},
			PSuccess:   &w, Weight: &w, Settlement: types.SettlementRecipient, Addressee: "research:doc-1", EmitterKind: types.CommitmentEmitterAgent,
			Provenance: types.CommitmentProvenance{AgentID: "secretdesk:doc-9", Desk: "secretdesk:doc-9", ModelID: "secretmodel-9", RunID: "run-9", Origin: types.OriginDeskRun, CommittedAt: "2026-11-04T10:00:00Z"}}
		if flagged {
			rec.InertFlags = []string{types.InertNoFalsifier}
		}
		return rec
	}
	recs := []types.CommitmentRecord{mk(0, true), mk(1, false), mk(2, true), mk(3, false)}
	rt.sampleInertness(scope.OwnerID, scope.ComputerID, recs)
	rt.waitSubCallJudge()
	if judge.callCount() != 1 {
		t.Fatalf("the sample took %d judge calls, want one", judge.callCount())
	}
	for _, banned := range []string{"secretdesk", "secretmodel", "run-9", "0.4123", "doc-9"} {
		if strings.Contains(judge.states[0], banned) {
			t.Fatalf("the blind judge saw %q: %s", banned, judge.states[0])
		}
	}
	for id, raw := range judge.asked[0] {
		if !strings.Contains(string(raw), "Could any plausible outcome have contradicted this prediction") {
			t.Fatalf("question %s: %s", id, raw)
		}
	}
	judgements, err := rt.store.ListSubCallRecords(context.Background(), scope.OwnerID, scope.ComputerID, types.SubCallInertJudgement, 100)
	if err != nil {
		t.Fatal(err)
	}
	a := types.InertJudgeAgreementOf(judgements)
	if a.Sampled != 4 || a.BothInert != 1 || a.JudgeOnly != 1 || a.RulesOnly != 1 || a.BothInformative != 1 || a.Agree != 2 || a.AgreementRate != 0.5 {
		t.Fatalf("agreement: %+v", a)
	}
	// Recorded on the tape view too.
	_, view, err := rt.InertRates(context.Background(), scope.OwnerID, scope.ComputerID)
	if err != nil || view.Sampled != 4 {
		t.Fatalf("view: %+v %v", view, err)
	}
	// A second sample of the same records re-asks nothing.
	rt.sampleInertness(scope.OwnerID, scope.ComputerID, recs)
	rt.waitSubCallJudge()
	if judge.callCount() != 1 {
		t.Fatalf("judged again: %d calls", judge.callCount())
	}
	_ = s
}

// A cell can neither set nor clear the flag: the reducer overwrites it from
// the record's content.
func TestInertFlagCannotBeSetOrClearedByACell(t *testing.T) {
	scope := inertReductionScope(&Runtime{}, "cell-flag")
	p, w := 0.995, 0.4
	inertIn := yaegikernel.StagedIntent{LocalID: "x", Kind: yaegikernel.IntentCast, ToDesk: "management", Objective: "o",
		Req: &types.RequestFields{PSuccess: &p, Weight: &w, Settlement: types.SettlementRecipient}}
	rec := commitmentRecordForIntent(scope, inertIn)
	if !types.HasFlag(rec.InertFlags, types.InertNearCertain) {
		t.Fatalf("flags %v", rec.InertFlags)
	}
	// Clear: stamping again over a record whose flags were emptied restores them.
	rec.InertFlags = nil
	stampRequestRecord(scope, inertIn, &rec)
	if !types.HasFlag(rec.InertFlags, types.InertNearCertain) {
		t.Fatalf("a cleared flag stayed cleared: %v", rec.InertFlags)
	}
	// Set: a planted flag on an informative request is overwritten.
	p2 := 0.7
	honest := inertIn
	honest.Req = &types.RequestFields{PSuccess: &p2, Weight: &w, Settlement: types.SettlementRecipient,
		Questions: []types.TypedQuestion{{Question: "Will it hold?", Answers: []string{"yes", "no"}, Probabilities: map[string]float64{"yes": 0.7, "no": 0.3}}}}
	honest.Objective = "Verify the migration kept every row"
	rec = commitmentRecordForIntent(scope, honest)
	rec.InertFlags = []string{types.InertTrivialTemplate}
	stampRequestRecord(scope, honest, &rec)
	if len(rec.InertFlags) != 0 {
		t.Fatalf("a planted flag survived: %v", rec.InertFlags)
	}
}
