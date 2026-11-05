package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA1.5 proofs: every sub-RLM call is a Cast with a Report record, scored by
// rules and by one blind, asynchronous judge call per fan-out.

// subCallChildProvider answers each child with fixed findings, picked by a
// keyword in its objective.
type subCallChildProvider struct {
	provideriface.Provider
	findings map[string]string
}

func (p *subCallChildProvider) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	raw := rawMessagesForTest(req.Messages)
	for key, text := range p.findings {
		if strings.Contains(raw, key) {
			return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: text, Model: "child-model"}, nil
		}
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "no findings", Model: "child-model"}, nil
}

// subCallJudge is a CandidateJudge that answers the batch it is given. By
// default it picks, for each typed question, the first answer; pick overrides
// by question id. Answers can be withheld (omit) and the call can be held
// (hold) or failed (err).
type subCallJudge struct {
	mu       sync.Mutex
	calls    int
	states   []string
	asked    []map[string]json.RawMessage
	pick     map[string]string
	omit     map[string]bool
	extra    []string
	err      error
	hold     chan struct{}
	started  chan struct{}
	model    string
	answered string
}

func (j *subCallJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	j.mu.Lock()
	j.calls++
	j.states = append(j.states, string(state))
	j.asked = append(j.asked, questions)
	hold, started, err := j.hold, j.started, j.err
	j.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if err != nil {
		return nil, err
	}
	answers := map[string]any{}
	for id, rawQ := range questions {
		if j.omit[id] {
			continue
		}
		var q struct {
			Choices []string `json:"choices"`
		}
		_ = json.Unmarshal(rawQ, &q)
		top := q.Choices[0]
		if strings.HasSuffix(id, ".answered") {
			top = firstNonEmpty(j.answered, "answered")
		}
		if want, ok := j.pick[id]; ok {
			top = want
		}
		probs := map[string]float64{}
		for _, c := range q.Choices {
			if c == top {
				probs[c] = 0.8
			} else {
				probs[c] = 0.2 / float64(len(q.Choices)-1)
			}
		}
		answers[id] = map[string]any{"type": "choice", "choice": top, "probabilities": probs}
	}
	for _, id := range j.extra {
		answers[id] = map[string]any{"type": "choice", "choice": "x", "probabilities": map[string]float64{"x": 1}}
	}
	model := firstNonEmpty(j.model, "typesafe/jev-1.13")
	return json.Marshal(map[string]any{"id": "gen-subcall", "model": model, "answers": answers,
		"usage": map[string]any{"input_tokens": 900, "output_tokens": 40, "cost": 0.00004}})
}

func (j *subCallJudge) callCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.calls
}

type subCallFixture struct {
	rt       *Runtime
	s        *store.Store
	judge    *subCallJudge
	owner    string
	computer string
	parent   toolregistry.ExecutionContext
}

func newSubCallFixture(t *testing.T, findings map[string]string) *subCallFixture {
	t.Helper()
	// The settler must not depend on the System 1 verb's caps or setting.
	t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_RUN", "")
	t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_DAY", "")
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	rt.provider = &subCallChildProvider{findings: findings}
	judge := &subCallJudge{}
	rt.SetCandidateJudge(judge)
	f := &subCallFixture{rt: rt, s: s, judge: judge, owner: "owner-sub", computer: rt.TextureComputerID()}
	// The System 1 verb is off on this computer: its setting is explicitly
	// off and its caps are unset. Every settlement below runs that way.
	if err := s.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer,
		Key: store.SettingResearchSystem1, Value: false, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	f.parent = toolregistry.ExecutionContext{RunID: "run-sub-1", AgentID: "research:doc-sub", OwnerID: f.owner, ComputerID: f.computer,
		Profile: "research", Role: "research", WorkingDir: t.TempDir()}
	return f
}

func (f *subCallFixture) research(t *testing.T, children []yaegikernel.ResearchChild) []yaegikernel.ResearchChildResult {
	t.Helper()
	payload, _ := json.Marshal(yaegikernel.ResearchPayload{Children: children})
	ctx := toolregistry.WithExecutionContext(context.Background(), f.parent)
	ctx = context.WithValue(ctx, researchLineageKey{}, researchLineage{ownerID: f.owner, selection: provideriface.LLMSelection{Model: "child-model"}})
	out, err := f.rt.runResearchChildren(ctx, f.parent, payload)
	if err != nil {
		t.Fatalf("runResearchChildren: %v", err)
	}
	var results []yaegikernel.ResearchChildResult
	if err := json.Unmarshal(out, &results); err != nil {
		t.Fatalf("decode results: %v", err)
	}
	return results
}

func (f *subCallFixture) record(t *testing.T, id string) types.CommitmentRecord {
	t.Helper()
	rec, err := f.s.GetCommitmentRecord(context.Background(), f.owner, f.computer, id)
	if err != nil || rec == nil {
		t.Fatalf("record %s: %v %v", id, rec, err)
	}
	return *rec
}

func (f *subCallFixture) exists(id string) bool {
	ok, _ := f.s.CommitmentRecordExists(context.Background(), f.owner, f.computer, id)
	return ok
}

func subCallYesNo(p float64) types.TypedQuestion {
	return types.TypedQuestion{Question: "Will the child find a primary source?", Answers: []string{"yes", "no"},
		Probabilities: map[string]float64{"yes": p, "no": 1 - p}, Weight: 0.6}
}

func fp(v float64) *float64 { return &v }

// Every child call produces a Cast with at least one typed question unless
// marked weight zero, and a Report record.
func TestSubCallEveryChildIsACastWithTypedQuestionsAndAReport(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings for gamma, see https://example.test/a"})
	results := f.research(t, []yaegikernel.ResearchChild{
		{Objective: "gamma typed", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)},
		{Objective: "gamma p-only", PSuccess: fp(0.9)},
		{Objective: "gamma bare"},
	})
	f.rt.waitSubCallJudge()
	if len(results) != 3 {
		t.Fatalf("results: %+v", results)
	}
	for i, r := range results {
		if r.CastID == "" || r.ReportID == "" || r.Error != "" || !strings.Contains(r.Findings, "findings for gamma") {
			t.Fatalf("child %d: %+v", i, r)
		}
		cast, report := f.record(t, r.CastID), f.record(t, r.ReportID)
		if !cast.IsTyped() && !cast.IsWeightZero() {
			t.Fatalf("child %d: a Cast with no typed question that is not weight zero: %+v", i, cast)
		}
		if err := types.ValidateSubCallCast(cast); err != nil {
			t.Fatalf("child %d: %v", i, err)
		}
		if report.Report == nil || report.ParentID != cast.RecordID || cast.ChildIDs[0] != report.RecordID {
			t.Fatalf("child %d: the Report is not linked to its Cast: %+v / %+v", i, report, cast)
		}
		if cast.Provenance.Desk != "research:doc-sub" || cast.Provenance.RunID != "run-sub-1" || cast.Provenance.Origin != types.OriginDeskRun {
			t.Fatalf("child %d: provenance %+v", i, cast.Provenance)
		}
		if cast.SubCall.Role != researchChildRole {
			t.Fatalf("child %d: role %q", i, cast.SubCall.Role)
		}
		if len(report.EvidenceRefs) != 1 || report.EvidenceRefs[0] != "https://example.test/a" {
			t.Fatalf("child %d: citations %v", i, report.EvidenceRefs)
		}
	}
	typed := f.record(t, results[0].CastID)
	if typed.PSuccess == nil || *typed.PSuccess != 0.7 || typed.Weight == nil || *typed.Weight != 0.5 || typed.MaterialityProcedure == nil || typed.IsWeightZero() {
		t.Fatalf("the stated Cast was not stamped: %+v", typed)
	}
	// A stated p_success becomes the one question it is; nothing is invented.
	pOnly := f.record(t, results[1].CastID)
	if !pOnly.IsTyped() || pOnly.Prediction.Questions[0].Probabilities["yes"] != 0.9 || !pOnly.IsWeightZero() || pOnly.PSuccess != nil {
		t.Fatalf("p_success alone: %+v", pOnly)
	}
	// A bare child states nothing: weight zero, no question, no probability.
	bare := f.record(t, results[2].CastID)
	if bare.IsTyped() || !bare.IsWeightZero() || bare.PSuccess != nil || bare.Weight != nil {
		t.Fatalf("bare child: %+v", bare)
	}
	if results[2].WeightZero == "" {
		t.Fatalf("the result does not say the Cast is weight zero: %+v", results[2])
	}
}

// A fan-out of N children is scored by ONE judge call, blind to who asserted
// and to the parent's probabilities.
func TestSubCallFanOutIsScoredByOneJudgeCall(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings for gamma"})
	results := f.research(t, []yaegikernel.ResearchChild{
		{Objective: "gamma one", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)},
		{Objective: "gamma two", Questions: []types.TypedQuestion{subCallYesNo(0.2)}, PSuccess: fp(0.2), Weight: fp(0.5)},
		{Objective: "gamma three"},
	})
	f.rt.waitSubCallJudge()
	if n := f.judge.callCount(); n != 1 {
		t.Fatalf("judge calls = %d, want 1 for a fan-out of 3", n)
	}
	state := f.judge.states[0]
	for _, leak := range []string{"0.7", "0.2", "run-sub-1", "research:doc-sub", "p_success"} {
		if strings.Contains(state, leak) {
			t.Fatalf("the judge was shown %q: %s", leak, state)
		}
	}
	for _, id := range []string{"c0.q0", "c0.answered", "c1.q0", "c1.answered", "c2.answered"} {
		if _, ok := f.judge.asked[0][id]; !ok {
			t.Fatalf("question %s not asked; asked %v", id, f.judge.asked[0])
		}
	}
	for _, r := range results {
		st := f.record(t, types.SubCallSettlementRecordID(r.CastID, types.SubCallJudgeScorerID))
		if st.SubCall.Method != types.CalibrationScorerIndependent || st.Provenance.AgentID != types.SubCallJudgeScorerID || st.SubCall.CastID != r.CastID {
			t.Fatalf("settlement: %+v", st)
		}
	}
	// The judge picked "yes" (the first answer) for both: confirmed for the
	// 0.7 parent, contradicted for the 0.2 parent (it ranked "no" first).
	if c := f.record(t, types.SubCallSettlementRecordID(results[0].CastID, types.SubCallJudgeScorerID)).SubCall.Class; c != types.DiscrepancyConfirmed {
		t.Fatalf("child 0 class = %s", c)
	}
	if c := f.record(t, types.SubCallSettlementRecordID(results[1].CastID, types.SubCallJudgeScorerID)).SubCall.Class; c != types.DiscrepancyContradicted {
		t.Fatalf("child 1 class = %s", c)
	}
	if c := f.record(t, types.SubCallSettlementRecordID(results[2].CastID, types.SubCallJudgeScorerID)).SubCall.Class; c != types.DiscrepancyAnswered {
		t.Fatalf("weight-zero child class = %s", c)
	}
	// The work is on the tape with its close, and the cost is recorded.
	closes, _ := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallJudgeClose, 10)
	if len(closes) != 1 || closes[0].SubCall.Phase != types.SubCallPhaseAnswered || closes[0].SubCall.InputTokens != 900 {
		t.Fatalf("closes: %+v", closes)
	}
}

type fakeCaptures struct{ known map[string]bool }

func (c fakeCaptures) CaptureBlocks(_ context.Context, _ string, ref string) (texturewrite.Capture, error) {
	if c.known[ref] {
		return texturewrite.Capture{}, nil
	}
	return texturewrite.Capture{}, texturewrite.ErrNotCaptured
}

// A rule failure (a citation that does not resolve) contradicts regardless of
// the judge.
func TestSubCallRuleFailureContradictsWhateverTheJudgeSays(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "found it at https://example.test/real and https://example.test/invented"})
	f.rt.textureCaptures = fakeCaptures{known: map[string]bool{"https://example.test/real": true}}
	results := f.research(t, []yaegikernel.ResearchChild{
		{Objective: "gamma cited", Questions: []types.TypedQuestion{subCallYesNo(0.9)}, PSuccess: fp(0.9), Weight: fp(0.5)},
	})
	f.rt.waitSubCallJudge()
	id := results[0].CastID
	rule := f.record(t, types.SubCallSettlementRecordID(id, types.SubCallRuleScorerPrefix+string(types.PrecommitRuleCitationsResolve)))
	if !rule.SubCall.Contradicted || rule.Discrepancy != types.DiscrepancyContradicted || len(rule.Resolve.EvidenceRefs) != 1 || rule.Resolve.EvidenceRefs[0] != "https://example.test/invented" {
		t.Fatalf("rule settlement: %+v", rule)
	}
	judge := f.record(t, types.SubCallSettlementRecordID(id, types.SubCallJudgeScorerID))
	if judge.SubCall.Class != types.DiscrepancyConfirmed {
		t.Fatalf("the judge was meant to confirm: %+v", judge.SubCall)
	}
	all, _ := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallSettlement, 50)
	class, by := types.SubCallVerdictClass(all)
	if class != types.DiscrepancyContradicted || by != types.CalibrationScorerRule {
		t.Fatalf("effective verdict = %s by %q, want contradicted by rule", class, by)
	}
	// And the self-favor row scores the independent side as 0.
	rows := types.SelfFavorRows([]types.CommitmentRecord{f.record(t, id)}, all)
	if len(rows) != 1 || rows[0].Independent != 0 || math.Abs(rows[0].Self-0.9) > 1e-9 {
		t.Fatalf("rows: %+v", rows)
	}
	// A well-formed Report passes its schema rule.
	if ok := f.exists(types.SubCallSettlementRecordID(id, types.SubCallRuleScorerPrefix+string(types.PrecommitRuleReportSchemaValid))); !ok {
		t.Fatalf("no report_schema_valid settlement")
	}
}

// A parent cannot settle its own record (negative), and no desk settles.
func TestSubCallParentCannotSettleItsOwnRecord(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	results := f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma mine", PSuccess: fp(0.8), Weight: fp(0.5)}})
	f.rt.waitSubCallJudge()
	cast := f.record(t, results[0].CastID)
	attempt := func(scorer, agent string) error {
		rec := buildRuleSettlement(subCallItem{cast: cast, report: f.record(t, results[0].ReportID)}, types.PrecommitRuleCompletes,
			types.RuleSettlement{Answer: types.PrecommitAnswerYes, Excerpt: "self"}, time.Now())
		rec.RecordID += ":" + agent
		rec.SubCall.Scorer, rec.Provenance.AgentID = scorer, agent
		return f.rt.writeSubCallSettlement(context.Background(), f.owner, f.computer, rec)
	}
	for _, c := range []struct{ scorer, agent string }{
		{"research:doc-sub", "research:doc-sub"},   // the parent itself
		{"system:judge", "research:doc-sub"},       // a record authored by the parent
		{cast.Addressee, cast.Addressee},           // the child
		{"owner-sub", "owner-sub"},                 // any desk or person: not a host service
		{"system:judge", cast.Provenance.Desk},     // provenance as the parent
		{cast.SubCall.ParentAgent, "system:judge"}, // scorer field names the parent
	} {
		if err := attempt(c.scorer, c.agent); err == nil {
			t.Fatalf("settlement by scorer %q / author %q was accepted", c.scorer, c.agent)
		}
	}
	// A self-score exists and is not a settlement: it never decides.
	self := f.record(t, types.SubCallSelfScoreRecordID(cast.RecordID))
	all, _ := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallSettlement, 50)
	for _, s := range all {
		if s.SubCall.Method == types.CalibrationScorerSelf || s.RecordID == self.RecordID {
			t.Fatalf("a self-score is among the settlements: %+v", s)
		}
	}
	if err := checkSettlerIndependence(cast, types.SubCallJudgeScorerID); err != nil {
		t.Fatalf("the judge service was refused: %v", err)
	}
}

// Settlement runs with the System 1 caps unset and the research.system1
// setting off: the settler does not go through the verb.
func TestSubCallSettlerDoesNotDependOnSystem1CapsOrSetting(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	perRun, perDay := f.rt.system1Limits(context.Background(), f.owner, f.computer)
	if perRun != 0 || perDay != 0 {
		t.Fatalf("the verb is on in this fixture (%d, %d); the negative test needs it off", perRun, perDay)
	}
	// The verb refuses here: its gate is real.
	if _, err := f.rt.system1Egress(context.Background(), yaegikernel.SessionRoleResearch, toolregistry.ExecutionContext{OwnerID: f.owner, ComputerID: f.computer, RunID: "r"}, json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "off") {
		t.Fatalf("the System 1 verb should be off: %v", err)
	}
	// A non-research role would be refused by the verb too; the settler has no role gate.
	results := f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma capless", Questions: []types.TypedQuestion{subCallYesNo(0.6)}, PSuccess: fp(0.6), Weight: fp(0.4)}})
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 || !f.exists(types.SubCallSettlementRecordID(results[0].CastID, types.SubCallJudgeScorerID)) {
		t.Fatalf("the settler did not settle with the caps unset (judge calls %d)", f.judge.callCount())
	}
	// And the verb's tape was not touched: no System1 event was written.
	if tape, _ := f.rt.system1ChannelTape(context.Background(), f.owner, f.computer); len(tape) != 0 {
		t.Fatalf("the settler wrote System1 events: %d", len(tape))
	}
}

// A fan-out completes with the judge stalled; the rule layer still lands, and
// a judge that never answers leaves the record unsettled, never the fan-out
// blocked.
func TestSubCallFanOutCompletesWithTheJudgeStalled(t *testing.T) {
	old := subCallJudgeTimeout
	subCallJudgeTimeout = 400 * time.Millisecond
	t.Cleanup(func() { subCallJudgeTimeout = old })
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	f.judge.hold = make(chan struct{}) // never released: a stalled judge
	f.judge.started = make(chan struct{}, 1)
	t.Cleanup(func() { close(f.judge.hold) })

	done := make(chan []yaegikernel.ResearchChildResult, 1)
	go func() {
		done <- f.research(t, []yaegikernel.ResearchChild{
			{Objective: "gamma stalled one", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)},
			{Objective: "gamma stalled two"},
		})
	}()
	var results []yaegikernel.ResearchChildResult
	select {
	case results = <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the fan-out waited on the judge")
	}
	select {
	case <-f.judge.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the judge was never called")
	}
	for _, r := range results {
		if r.Error != "" || r.Findings == "" || r.ReportID == "" {
			t.Fatalf("a child result was held up: %+v", r)
		}
	}
	f.rt.waitSubCallJudge() // the timeout ends the stalled call
	if f.exists(types.SubCallSettlementRecordID(results[0].CastID, types.SubCallJudgeScorerID)) {
		t.Fatal("a stalled judge settled a Cast")
	}
	if !f.exists(types.SubCallSettlementRecordID(results[0].CastID, types.SubCallRuleScorerPrefix+string(types.PrecommitRuleReportSchemaValid))) {
		t.Fatal("the rule layer did not land while the judge stalled")
	}
	closes, _ := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallJudgeClose, 10)
	if len(closes) != 1 || closes[0].SubCall.Phase != types.SubCallPhaseFailed {
		t.Fatalf("closes: %+v", closes)
	}
}

// A judge error leaves the record unsettled, once; nothing retries.
func TestSubCallJudgeErrorLeavesTheRecordUnsettled(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	f.judge.err = errors.New("judge down")
	results := f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma err", PSuccess: fp(0.7), Weight: fp(0.5)}})
	f.rt.waitSubCallJudge()
	if f.exists(types.SubCallSettlementRecordID(results[0].CastID, types.SubCallJudgeScorerID)) || f.judge.callCount() != 1 {
		t.Fatalf("calls %d", f.judge.callCount())
	}
	// With no decision model bound at all, the same: unsettled, no error to the fan-out.
	f.rt.candidateJudge = nil
	results = f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma none"}})
	f.rt.waitSubCallJudge()
	if results[0].Error != "" || f.exists(types.SubCallSettlementRecordID(results[0].CastID, types.SubCallJudgeScorerID)) {
		t.Fatalf("no-judge fan-out: %+v", results[0])
	}
}

// Wrong batch mapping: answers are read by the ids the batch minted. An
// unknown id is ignored and a missing one leaves its question unanswered.
func TestSubCallBatchMappingIgnoresStrayAndMissingAnswers(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	f.judge.extra = []string{"c9.q0", "c0.q7"}
	f.judge.omit = map[string]bool{"c1.q0": true, "c1.answered": true}
	f.judge.pick = map[string]string{"c2.q0": "no"}
	results := f.research(t, []yaegikernel.ResearchChild{
		{Objective: "gamma zero", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)},
		{Objective: "gamma one", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)},
		{Objective: "gamma two", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)},
	})
	f.rt.waitSubCallJudge()
	get := func(i int) (types.CommitmentRecord, bool) {
		id := types.SubCallSettlementRecordID(results[i].CastID, types.SubCallJudgeScorerID)
		if !f.exists(id) {
			return types.CommitmentRecord{}, false
		}
		return f.record(t, id), true
	}
	if s, ok := get(0); !ok || s.SubCall.Verdicts[0].Answer != "yes" || s.SubCall.Class != types.DiscrepancyConfirmed {
		t.Fatalf("child 0: %+v %v", s, ok)
	}
	if _, ok := get(1); ok {
		t.Fatal("child 1 got a settlement from answers the judge did not give")
	}
	if s, ok := get(2); !ok || s.SubCall.Verdicts[0].Answer != "no" || s.SubCall.Class != types.DiscrepancyContradicted || s.SubCall.CastID != results[2].CastID {
		t.Fatalf("child 2 (answered 'no'): %+v %v", s, ok)
	}
}

// Settling a record twice: the same batch settled again re-judges nothing.
func TestSubCallBatchIsNotSettledTwice(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	results := f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma once", PSuccess: fp(0.7), Weight: fp(0.5)}})
	f.rt.waitSubCallJudge()
	var ids []string
	for _, r := range results {
		ids = append(ids, r.CastID)
	}
	fan := f.record(t, ids[0]).SubCall.FanOutID
	for i := 0; i < 3; i++ {
		_ = f.rt.settleSubCalls(context.Background(), f.owner, f.computer, fan, ids)
	}
	f.rt.enqueueSubCallJudge(f.owner, f.computer, fan, ids, 0)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 {
		t.Fatalf("judge calls = %d, want 1", f.judge.callCount())
	}
}

// A judge of the child's own family settles, but is not independent.
func TestSubCallSameFamilyJudgeIsNotIndependent(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	f.judge.model = "child-model-2" // the children run as child-model
	results := f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma family", Questions: []types.TypedQuestion{subCallYesNo(0.7)}, PSuccess: fp(0.7), Weight: fp(0.5)}})
	f.rt.waitSubCallJudge()
	st := f.record(t, types.SubCallSettlementRecordID(results[0].CastID, types.SubCallJudgeScorerID))
	if !st.SubCall.SameFamily {
		t.Fatalf("not flagged: %+v", st.SubCall)
	}
	all, _ := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallSettlement, 50)
	if class, _ := types.SubCallVerdictClass(all); class != types.DiscrepancyUnresolved {
		t.Fatalf("a same-family judge decided: %s", class)
	}
	casts, _ := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallCast, 50)
	if rows := types.SelfFavorRows(casts, all); len(rows) != 0 {
		t.Fatalf("a same-family score counted as independent: %+v", rows)
	}
}

// A crash never resumes a pending judge call; a planned restart resumes it
// once.
func TestSubCallOpenJudgeWorkIsClosedAtCrashBootAndResumedOnceWhenPlanned(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	// Build a fan-out whose judge work was open when the process died: the
	// Cast and Report exist, the work record exists, nothing closed it.
	cast, err := buildSubCallCast(subCallCastParams{RecordID: types.SubCallCastRecordID("fan-crash", 0), FanOutID: "fan-crash", ChildKey: "k",
		ChildRole: researchChildRole, ChildInbox: researchChildRole + ":k", Objective: "gamma crash", ParentAgent: "research:doc-sub", ParentRunID: "run-sub-1",
		ParentModel: "parent-model", ComputerID: f.computer, PSuccess: fp(0.7), Weight: fp(0.5), Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	report := buildSubCallReport(subCallReportParams{Cast: cast, Child: researchChildRole + ":k", Model: "child-model", Text: "findings", Now: time.Now()})
	ctx := context.Background()
	for _, rec := range []types.CommitmentRecord{cast, report} {
		if err := f.rt.appendSubCallRecord(ctx, f.owner, f.computer, rec); err != nil {
			t.Fatal(err)
		}
	}
	workID := types.SubCallJudgeWorkID("fan-crash", []string{cast.RecordID})
	work := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: workID, Kind: types.CommitmentKindResolve,
		Provenance:  types.CommitmentProvenance{AgentID: types.SubCallJudgeScorerID, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: f.computer,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallJudgeWork, FanOutID: "fan-crash", Casts: []string{cast.RecordID}, Phase: "requested"}}
	if err := f.rt.appendSubCallRecord(ctx, f.owner, f.computer, work); err != nil {
		t.Fatal(err)
	}

	// Crash boot: closed "interrupted", the judge is never called.
	f.rt.closeOpenJudgeWork(ctx, f.owner, f.computer, false)
	f.rt.waitSubCallJudge()
	closeRec := f.record(t, types.SubCallJudgeCloseRecordID(workID))
	if closeRec.SubCall.Phase != types.SubCallPhaseInterrupted || f.judge.callCount() != 0 {
		t.Fatalf("crash boot: %+v, judge calls %d", closeRec.SubCall, f.judge.callCount())
	}
	// A second crash boot finds nothing open.
	f.rt.closeOpenJudgeWork(ctx, f.owner, f.computer, false)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 0 {
		t.Fatal("a crash boot ran the judge")
	}

	// A planned restart (marker present): open work is resumed once.
	cast2 := cast
	cast2.RecordID = types.SubCallCastRecordID("fan-planned", 0)
	cast2.SubCall = &types.CommitmentSubCall{}
	*cast2.SubCall = *cast.SubCall
	cast2.SubCall.FanOutID = "fan-planned"
	report2 := buildSubCallReport(subCallReportParams{Cast: cast2, Child: researchChildRole + ":k", Model: "child-model", Text: "findings", Now: time.Now()})
	work2 := work
	work2.RecordID = types.SubCallJudgeWorkID("fan-planned", []string{cast2.RecordID})
	work2.SubCall = &types.CommitmentSubCall{Kind: types.SubCallJudgeWork, FanOutID: "fan-planned", Casts: []string{cast2.RecordID}, Phase: "requested"}
	for _, rec := range []types.CommitmentRecord{cast2, report2, work2} {
		if err := f.rt.appendSubCallRecord(ctx, f.owner, f.computer, rec); err != nil {
			t.Fatal(err)
		}
	}
	f.rt.closeOpenJudgeWork(ctx, f.owner, f.computer, true)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 || !f.exists(types.SubCallSettlementRecordID(cast2.RecordID, types.SubCallJudgeScorerID)) {
		t.Fatalf("planned restart: judge calls %d", f.judge.callCount())
	}
	// And it is closed, so a markerless boot after it has nothing to resume.
	f.rt.closeOpenJudgeWork(ctx, f.owner, f.computer, false)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 {
		t.Fatalf("judge calls after the next boot = %d", f.judge.callCount())
	}
}

// The self-favor gap, computed on records shaped like d@e's (the fixture is a
// tape excerpt: Casts with stated probabilities, settlements by the judge and
// a rule, replayed from the store).
func TestSubCallSelfFavorGapOnReplayedRecords(t *testing.T) {
	raw, err := os.ReadFile("testdata/subcall_de_records.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture []types.CommitmentRecord
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	rt, s := testRuntime(t)
	ctx := context.Background()
	for _, rec := range fixture {
		if _, err := s.AppendCommitmentRecord(ctx, "owner-de", "computer-de", rec); err != nil {
			t.Fatalf("replay %s: %v", rec.RecordID, err)
		}
	}
	groups, rows, err := rt.SubCallSelfFavor(ctx, "owner-de", "computer-de")
	if err != nil {
		t.Fatal(err)
	}
	// Five Casts: A (self 0.9, judge 0.3), B (self 0.8, judge 0.8), C (self
	// 0.7, rule contradicted), D (self 0.6, judge 0.9, an unfavourable gap),
	// E (no independent settlement: not counted), and a same-family F.
	want := map[string][2]float64{"subcall:fan-de:c0": {0.9, 0.3}, "subcall:fan-de:c1": {0.8, 0.8}, "subcall:fan-de:c2": {0.7, 0}, "subcall:fan-de:c3": {0.6, 0.9}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d: %+v", len(rows), len(want), rows)
	}
	for _, r := range rows {
		w, ok := want[r.CastID]
		if !ok || math.Abs(r.Self-w[0]) > 1e-9 || math.Abs(r.Independent-w[1]) > 1e-9 {
			t.Fatalf("row %+v, want %v", r, w)
		}
	}
	if len(groups) != 1 {
		t.Fatalf("groups: %+v", groups)
	}
	g := groups[0]
	if g.Agent != "research:doc-de" || g.Model != "deepseek-v4.1-flash" || g.N != 4 || g.SelfFavorable != 2 || g.SelfUnfavorable != 1 || g.Material != 2 {
		t.Fatalf("group: %+v", g)
	}
	if math.Abs(g.Gap-((0.9+0.8+0.7+0.6)/4-(0.3+0.8+0+0.9)/4)) > 1e-9 {
		t.Fatalf("gap = %v", g.Gap)
	}
	// Replaying the same tape again gives the same view.
	groups2, _, _ := rt.SubCallSelfFavor(ctx, "owner-de", "computer-de")
	if fmt.Sprint(groups) != fmt.Sprint(groups2) {
		t.Fatalf("replay differs: %v vs %v", groups, groups2)
	}
}

// Sub-call records stay out of a desk's claim pack, and legacy records (no
// sub-call body) still decode.
func TestSubCallRecordsStayOutOfDeskPacksAndLegacyRecordsDecode(t *testing.T) {
	f := newSubCallFixture(t, map[string]string{"gamma": "findings"})
	results := f.research(t, []yaegikernel.ResearchChild{{Objective: "gamma pack", PSuccess: fp(0.7), Weight: fp(0.5)}})
	f.rt.waitSubCallJudge()
	pack, err := f.s.ListCommitmentClaimRecords(context.Background(), f.owner, f.computer, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range pack {
		if rec.SubCall != nil {
			t.Fatalf("a sub-call record is in a desk's pack: %s", rec.RecordID)
		}
	}
	legacy := `{"schema_id":"choir.commitment_record.v1","record_id":"legacy-1","kind":"directive","directive":{"subtype":"cast","objective":"x"},"cast":{},"prediction":{"hypothesis":"x","committed_at":"t"},"observation":{},"scores":null,"discrepancy":"unresolved","provenance":{"agent_id":"a","model_id":"m","committed_at":"t"}}`
	var rec types.CommitmentRecord
	if err := json.Unmarshal([]byte(legacy), &rec); err != nil || rec.SubCall != nil || rec.IsWeightZero() {
		t.Fatalf("legacy decode: %+v %v", rec, err)
	}
	if _, ok := rec.SubCallBody(types.SubCallCast); ok {
		t.Fatal("a legacy record claims a sub-call body")
	}
	_ = results
}

// A Delegate Cast carries the delegator's own typed questions, and is weight
// zero without them; the host states no probability.
func TestSubCallDelegateCastIsTypedOrWeightZero(t *testing.T) {
	r := &rlmCallReduction{scope: ReductionScope{CellID: "cell-1", RunID: "run-m", ComputerID: "c1", OwnerID: "o1"}}
	spec := yaegikernel.DelegateSpec{Kind: "intake", Objective: "Dispose of these captures."}
	base := func() types.CommitmentRecord {
		return types.CommitmentRecord{RecordID: "cell-1:delegate:L1:0", Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast}, Cast: &types.CommitmentCast{},
			Addressee: "management-task:t1", SenderInbox: "management:o1", Provenance: types.CommitmentProvenance{ModelID: "m"}}
	}
	stated := base()
	in := yaegikernel.StagedIntent{Kind: yaegikernel.IntentDelegate, LocalID: "L1", Req: &types.RequestFields{
		PSuccess: fp(0.8), Weight: fp(0.5), Settlement: types.SettlementRecipient, Questions: []types.TypedQuestion{subCallYesNo(0.8)}}}
	if err := r.typeDelegateCast(&stated, in, spec, 0, "t1"); err != nil {
		t.Fatal(err)
	}
	if !stated.IsTyped() || stated.IsWeightZero() || stated.PSuccess == nil || *stated.PSuccess != 0.8 || stated.SubCall.Role != "management_task" ||
		stated.SubCall.FanOutID != delegateFanOutID("cell-1", "L1") {
		t.Fatalf("stated: %+v", stated)
	}
	if hasFlag(stated.InertFlags, types.InertWeightBelowFloor) {
		t.Fatalf("a 0.5 weight is flagged inert: %v", stated.InertFlags)
	}
	bare := base()
	if err := r.typeDelegateCast(&bare, yaegikernel.StagedIntent{Kind: yaegikernel.IntentDelegate, LocalID: "L1"}, spec, 0, "t1"); err != nil {
		t.Fatal(err)
	}
	if bare.IsTyped() || !bare.IsWeightZero() || bare.PSuccess != nil || bare.Weight != nil {
		t.Fatalf("bare delegate: %+v", bare)
	}
	low := base()
	inLow := yaegikernel.StagedIntent{Kind: yaegikernel.IntentDelegate, LocalID: "L1", Req: &types.RequestFields{
		PSuccess: fp(0.8), Weight: fp(0.01), Settlement: types.SettlementRecipient}}
	if err := r.typeDelegateCast(&low, inLow, spec, 0, "t1"); err != nil {
		t.Fatal(err)
	}
	if !hasFlag(low.InertFlags, types.InertWeightBelowFloor) || low.PSuccess == nil {
		t.Fatalf("a weight under the grants' floor is not flagged (and must not be refused): %+v", low)
	}
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

// A Delegate's task Cast carries the delegating cell's own typed questions,
// and the task's final Report is scored: the closing Report becomes the
// sub-call's Report, and the judge settles the Cast against it.
func TestSubCallDelegateTaskReportIsScoredEndToEnd(t *testing.T) {
	old := subCallDelegateCoalesce
	subCallDelegateCoalesce = time.Millisecond
	t.Cleanup(func() { subCallDelegateCoalesce = old })
	f := newDelegationFixture(t, "owner-subcall")
	judge := &subCallJudge{}
	f.rt.SetCandidateJudge(judge)
	f.model.parentCell = `var ids []string
for _, r := range choir.Records() { if r.Captures != nil { ids = append(ids, r.RecordID) } }
choir.Delegate("intake", "", ids, "per_record", map[string]any{"p_success": 0.8, "weight": 0.4, "settlement": "recipient",
	"questions": []map[string]any{{"question": "Will the task dispose of every capture?", "answers": []string{"yes", "no"}, "probabilities": map[string]float64{"yes": 0.8, "no": 0.2}, "weight": 0.5}}})`
	seedArrival(t, f.s, f.owner, f.computer, "rss:a", []types.CaptureArrival{
		{CaptureID: "cap-a1", Title: "Pope recovers", URL: "https://a.example/1"},
		{CaptureID: "cap-a2", Title: "Newsletter", URL: "https://a.example/2"},
	})
	f.parentTurn()
	queued := f.tasks(types.ManagementTaskQueued)
	if len(queued) != 1 {
		t.Fatalf("queued: %+v", queued)
	}
	task := queued[0]
	ctx := context.Background()
	cast, err := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, task.OpeningRecordID)
	if err != nil || cast == nil {
		t.Fatalf("cast: %v %v", cast, err)
	}
	if !cast.IsTyped() || cast.IsWeightZero() || cast.PSuccess == nil || *cast.PSuccess != 0.8 || cast.Weight == nil || *cast.Weight != 0.4 ||
		cast.SubCall == nil || cast.SubCall.Kind != types.SubCallCast || cast.Provenance.Origin != types.OriginDeskRun || cast.Provenance.Desk == "" {
		t.Fatalf("the Delegate Cast is not typed and stamped: %+v", cast)
	}
	// The task runs to its Done; its closing Report is scored.
	run, err := f.rt.WakeRecordsInbox(ctx, f.owner, f.computer, task.TaskInbox)
	if err != nil || run == nil {
		t.Fatalf("start task: %v %v", run, err)
	}
	waitForStoredRunTerminalState(t, f.s, run.RunID, 60*time.Second)
	eventually(t, func() bool { return f.task(task.TaskID).State == types.ManagementTaskClosed })
	reportID := types.SubCallReportRecordID(cast.RecordID)
	eventually(t, func() bool {
		ok, _ := f.s.CommitmentRecordExists(ctx, f.owner, f.computer, types.SubCallSettlementRecordID(cast.RecordID, types.SubCallJudgeScorerID))
		return ok
	})
	f.rt.waitSubCallJudge()
	report, _ := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, reportID)
	if report == nil || report.Report == nil || report.ParentID != cast.RecordID || len(report.Report.Claims) == 0 {
		t.Fatalf("no sub-call Report for the closed task: %+v", report)
	}
	if judge.callCount() != 1 {
		t.Fatalf("judge calls = %d", judge.callCount())
	}
	if _, ok := judge.asked[0]["c0.q0"]; !ok {
		t.Fatalf("the Delegate's typed question was not put to the judge: %v", judge.asked[0])
	}
}
