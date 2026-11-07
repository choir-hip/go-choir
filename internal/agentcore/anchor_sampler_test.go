package agentcore

// Failure modes of the live anchor sampler (MA7.10 wiring), written first:
//  L1 the pass is not running: the loop is started by Start and stopped by
//     Stop, and a pass over a real ledger writes anchors (TestAnchorPass...).
//  L2 a desk can tell a record was checked: the ledger's records are byte
//     for byte what they were, and a desk's pack and the judge's prompt carry
//     no marker.
//  L3 the pass blows the judge budget: at most anchorPassBudget judge calls
//     per pass; the rest wait for the next pass undecided, with no bias.
//  L4 a crash mid-pass leaves a half-settled anchor: an anchor is one row,
//     written only after the strongest judge settled; a judge error leaves
//     none, and the item is not retried.
//  L5 the pass resumes: a second pass over the same ledger decides nothing
//     twice and asks the judge nothing again.
//  L6 the log drifts from the sampler: a tampered log stops the pass.
//  L7 the strongest judge is substituted: with the judge unavailable, no
//     anchor settles.
//  L8 the harness reads no data: the served sample carries the anchors and
//     the corpus rows, and no text, no prompt and no seed.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type anchorTestJudge struct {
	mu      sync.Mutex
	calls   int
	prompts []string
	ids     []string
	answer  string
	err     error
	strong  int
	// poison, when set, makes the first item asked fail on every call.
	poison   bool
	poisonID string
}

func (j *anchorTestJudge) Name() string  { return "anchor-test-judge" }
func (j *anchorTestJudge) Strength() int { return j.strong }
func (j *anchorTestJudge) Judge(_ context.Context, q trust.JudgeQuestion) (string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls++
	j.prompts = append(j.prompts, q.Prompt)
	j.ids = append(j.ids, q.ItemID)
	if j.poison {
		if j.poisonID == "" {
			j.poisonID = q.ItemID
		}
		if q.ItemID == j.poisonID {
			return "", fmt.Errorf("the gateway rejected this item")
		}
	}
	return j.answer, j.err
}

type anchorFixture struct {
	rt       *Runtime
	owner    string
	computer string
	judge    *anchorTestJudge
}

// newAnchorFixture writes n judged calls: a Cast with one yes/no question
// (the parent says yes at 0.8), its Report, and the independent judge's
// settlement (answers alternate yes/no by index).
func newAnchorFixture(t *testing.T, n int) *anchorFixture {
	t.Helper()
	rt, _ := testRuntime(t)
	f := &anchorFixture{rt: rt, owner: "owner-anc", computer: rt.TextureComputerID(), judge: &anchorTestJudge{answer: "yes", strong: 1}}
	rt.anchorJudges = func() []trust.Judge { return []trust.Judge{f.judge} }
	for i := 0; i < n; i++ {
		f.writeJudged(t, i, false)
	}
	return f
}

// writeJudged writes judged call i; sameFamily marks the judge's settlement
// as same-family (not an independent label).
func (f *anchorFixture) writeJudged(t *testing.T, i int, sameFamily bool) {
	t.Helper()
	rt := f.rt
	{
		fan := fmt.Sprintf("fan-anc-%03d", i)
		cast, err := buildSubCallCast(subCallCastParams{RecordID: types.SubCallCastRecordID(fan, 0), FanOutID: fan, ChildKey: "k" + fan,
			ChildRole: "research_web", Objective: "find the primary source " + fan, ParentAgent: "research:doc-anc", ParentRunID: "run-anc",
			ParentModel: "parent-model", ComputerID: f.computer, PSuccess: fp(0.8), Weight: fp(0.5), Now: time.Now()})
		if err != nil {
			t.Fatal(err)
		}
		report := buildSubCallReport(subCallReportParams{Cast: cast, Child: "research_web:k", Model: "child-model", Text: "the source is found at the archive " + fan, Now: time.Now()})
		ans := []string{"yes", "no"}[i%2]
		dist := jevDistribution{Model: "judge-model"}
		dist.Answers = map[string]struct {
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
		}{types.SubCallQuestionID(0, 0): {Choice: ans, Probabilities: map[string]float64{ans: 0.9, map[string]string{"yes": "no", "no": "yes"}[ans]: 0.1}}}
		settle, ok := buildJudgeSettlement(subCallItem{cast: cast, report: report}, 0, dist, "judge-model", time.Now())
		if !ok {
			t.Fatal("no settlement built")
		}
		settle.SubCall.SameFamily = sameFamily
		for _, rec := range []types.CommitmentRecord{cast, report, settle} {
			if err := rt.appendSubCallRecord(context.Background(), f.owner, f.computer, rec); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func (f *anchorFixture) anchors(t *testing.T) []trust.AnchorItem {
	t.Helper()
	got, err := f.rt.store.TrustAnchors().ListAnchors(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func (f *anchorFixture) ledgerJSON(t *testing.T) string {
	t.Helper()
	var all []types.CommitmentRecord
	for _, k := range []string{types.SubCallCast, types.SubCallReport, types.SubCallSettlement} {
		recs, err := f.rt.store.ListSubCallRecords(context.Background(), f.owner, f.computer, k, 1000)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, recs...)
	}
	b, _ := json.Marshal(all)
	return string(b)
}

func TestAnchorPassSettlesSampledItemsAndLeavesTheLedgerAlone(t *testing.T) {
	f := newAnchorFixture(t, 100)
	before := f.ledgerJSON(t)
	res, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decided == 0 || res.Sampled == 0 || res.Settled != res.Sampled || res.Settled != len(f.anchors(t)) {
		t.Fatalf("pass %+v anchors %d", res, len(f.anchors(t)))
	}
	if f.judge.calls != res.Sampled {
		t.Fatalf("%d judge calls for %d sampled", f.judge.calls, res.Sampled)
	}
	// L2: nothing on the ledger changed
	if after := f.ledgerJSON(t); after != before {
		t.Fatal("L2: the pass touched a ledger record")
	}
	// the anchor carries both scorers' answers and the strongest judge's truth
	a := f.anchors(t)[0]
	if a.Truth != "yes" || a.Kind != trust.AnchorSpotCheck || len(a.Answers) != 2 || a.Agent != "research:doc-anc" || a.QuestionType != "research_web" {
		t.Fatalf("anchor %+v", a)
	}
	names := []string{a.Answers[0].Scorer, a.Answers[1].Scorer}
	if !(names[0] == "system:judge" && names[1] == "self:research:doc-anc") && !(names[1] == "system:judge" && names[0] == "self:research:doc-anc") {
		t.Fatalf("scorers %v", names)
	}
	// the decision log is under the computer's state and replays
	path := filepath.Join(filepath.Dir(f.rt.cfg.StorePath), spotLogName)
	entries, err := trust.LoadSpotLog(path)
	if err != nil || len(entries) != res.Decided {
		t.Fatalf("%d log entries for %d decisions, err %v", len(entries), res.Decided, err)
	}
	seed, _ := f.rt.store.TrustAnchors().Seed(context.Background(), f.owner, f.computer)
	s, _ := trust.NewSpotSampler(seed, anchorSpotRate)
	if bad := trust.VerifySpotLog(s, entries); len(bad) != 0 {
		t.Fatalf("replay: %v", bad)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), fmt.Sprintf("%x", seed)) {
		t.Fatal("the log carries the seed")
	}
}

func TestAnchorPassKeepsMarkersOutOfTheJudgeAndEveryDeskPack(t *testing.T) {
	f := newAnchorFixture(t, 100)
	if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err != nil || len(f.anchors(t)) == 0 {
		t.Fatalf("no sampled record: %v", err)
	}
	markers := []string{"spot", "anchor", "sampled", "trust_seed", "trust_anchor", "audit", "graded", "grading"}
	for _, p := range f.judge.prompts {
		low := strings.ToLower(p)
		for _, m := range markers {
			if strings.Contains(low, m) {
				t.Fatalf("the judge's prompt names %q: %s", m, p)
			}
		}
	}
	for _, agent := range []string{"research:doc-anc", "management", "texture:doc-anc"} {
		pack := actingCommitmentPackForDesk(context.Background(), f.rt, toolregistry.ExecutionContext{
			AgentID: agent, OwnerID: f.owner, ComputerID: f.computer, RunID: "run-pack"})
		b, _ := json.Marshal(pack)
		low := strings.ToLower(string(b))
		for _, m := range markers {
			if strings.Contains(low, m) {
				t.Fatalf("L2: %s's pack names %q", agent, m)
			}
		}
	}
}

func TestAnchorPassIsBudgetedAndNeverDecidesTwice(t *testing.T) {
	f := newAnchorFixture(t, 120)
	first, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	if first.Sampled > anchorPassBudget || f.judge.calls > anchorPassBudget {
		t.Fatalf("L3: %d sampled, %d judge calls, budget %d", first.Sampled, f.judge.calls, anchorPassBudget)
	}
	// run passes until nothing is left; no item is decided or judged twice
	total := first.Decided
	for i := 0; i < 20; i++ {
		r, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
		if err != nil {
			t.Fatal(err)
		}
		total += r.Decided
		if r.Decided == 0 && r.Sampled == 0 {
			break
		}
	}
	if total != 120 {
		t.Fatalf("L5: %d decisions for 120 items", total)
	}
	seen := map[string]bool{}
	for _, a := range f.anchors(t) {
		if seen[a.ItemID] {
			t.Fatalf("L5: %s anchored twice", a.ItemID)
		}
		seen[a.ItemID] = true
	}
	calls := f.judge.calls
	if r, _ := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); r.Decided != 0 || f.judge.calls != calls {
		t.Fatalf("L5: a settled ledger was judged again: %+v", r)
	}
}

func TestAnchorPassWithTheJudgeDownDefersTheItemAndAFinalAnswerDecidesIt(t *testing.T) {
	f := newAnchorFixture(t, 100)
	f.judge.err = fmt.Errorf("gateway down")
	res, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	// L4/L7: nothing settled, the failed item is not decided, the pass stopped at it
	if res.Settled != 0 || res.Deferred != anchorConsecutiveFailures || len(f.anchors(t)) != 0 || f.judge.calls != anchorConsecutiveFailures {
		t.Fatalf("%+v anchors %d calls %d", res, len(f.anchors(t)), f.judge.calls)
	}
	failed := f.judge.ids[0]
	path := filepath.Join(filepath.Dir(f.rt.cfg.StorePath), spotLogName)
	es, _ := trust.LoadSpotLog(path)
	for _, e := range es {
		if e.RecordID == failed && e.Deferral == 0 {
			t.Fatal("a transient failure was recorded as the item's decision")
		}
	}
	// R3: with the judge back, a later pass asks again and anchors the same item
	f.judge.err = nil
	if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range f.anchors(t) {
		found = found || a.ItemID == failed
	}
	if !found {
		t.Fatalf("R3: the deferred item %s was never anchored", failed)
	}
	// a judge whose answer is outside the closed set is final: decided, unknown, not retried
	g := newAnchorFixture(t, 100)
	g.judge.answer = "maybe"
	r, err := g.rt.runAnchorPass(context.Background(), g.owner, g.computer)
	if err != nil || r.Sampled == 0 || r.Unsettled != r.Sampled || r.Deferred != 0 || len(g.anchors(t)) != 0 {
		t.Fatalf("final: %+v %v", r, err)
	}
	calls := g.judge.calls
	if _, err := g.rt.runAnchorPass(context.Background(), g.owner, g.computer); err != nil {
		t.Fatal(err)
	}
	asked := map[string]bool{}
	for _, id := range g.judge.ids[:calls] {
		asked[id] = true
	}
	for _, id := range g.judge.ids[calls:] {
		if asked[id] {
			t.Fatalf("a final-unsettled item %s was asked again", id)
		}
	}
	// a cancelled context decides nothing
	h := newAnchorFixture(t, 100)
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, _ := h.rt.runAnchorPass(cctx, h.owner, h.computer); r.Decided != 0 || len(h.anchors(t)) != 0 {
		t.Fatalf("a cancelled pass decided: %+v", r)
	}
	// no judge wired: nothing is decided
	k := newAnchorFixture(t, 30)
	k.rt.anchorJudges = func() []trust.Judge { return nil }
	if r, err := k.rt.runAnchorPass(context.Background(), k.owner, k.computer); err != nil || r.Decided != 0 || len(k.anchors(t)) != 0 {
		t.Fatalf("L7: %+v err %v", r, err)
	}
}

func TestAnchorPassRefusesATamperedLog(t *testing.T) {
	f := newAnchorFixture(t, 100)
	if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(f.rt.cfg.StorePath), spotLogName)
	entries, _ := trust.LoadSpotLog(path)
	entries[0].Sampled = !entries[0].Sampled
	var b strings.Builder
	for _, e := range entries {
		j, _ := json.Marshal(e)
		b.Write(j)
		b.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err == nil {
		t.Fatal("L6: a tampered log was trusted")
	}
}

func TestAnchorSampleIsServedWithCorpusRowsAndNoContent(t *testing.T) {
	f := newAnchorFixture(t, 100)
	res, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	sample, err := f.rt.anchorSample(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	if len(sample.Items) != res.Settled || len(sample.Corpus) != sample.Decisions || sample.Decisions != res.Decided || sample.Sampled != res.Sampled {
		t.Fatalf("L8: items %d corpus %d decisions %d sampled %d, pass %+v", len(sample.Items), len(sample.Corpus), sample.Decisions, sample.Sampled, res)
	}
	anchored := 0
	for _, r := range sample.Corpus {
		if r.Anchored {
			anchored++
			if r.Truth == "" {
				t.Fatal("an anchored corpus row has no truth")
			}
		}
		if r.AgentAnswer != "yes" || r.JudgeAnswer == "" {
			t.Fatalf("row %+v", r)
		}
	}
	if anchored != res.Settled {
		t.Fatalf("anchored rows %d, anchors %d", anchored, res.Settled)
	}
	b, _ := json.Marshal(sample)
	for _, bad := range []string{"archive", "primary source", "seed_hex"} {
		if strings.Contains(string(b), bad) {
			t.Fatalf("L8: the served sample carries %q", bad)
		}
	}
}

func TestAnchorSamplerLoopIsStartedByStartAndStoppedByStop(t *testing.T) {
	rt, _ := testRuntime(t)
	started := make(chan struct{}, 1)
	rt.anchorTick = 5 * time.Millisecond
	rt.anchorInitialDelay = time.Millisecond
	rt.anchorOwner = func() string { return "owner-loop" }
	rt.anchorPassHook = func() {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	rt.startAnchorSampler(context.Background())
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("L1: the sampler loop never ran a pass")
	}
	rt.Stop()
}

var _ = provideriface.ToolLoopRequest{}

// L8: the owner's route serves the sample with the replay, empty and present
// (an absent field means an older build), and never with the plain view.
func TestTrustRouteServesTheAnchorSampleWithTheReplay(t *testing.T) {
	f := newAnchorFixture(t, 100)
	res, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatal(err)
	}
	h := &APIHandler{rt: f.rt}
	get := func(query string) trustResponse {
		req := httptest.NewRequest(http.MethodGet, "/api/trust"+query, nil)
		req.Header.Set("X-Authenticated-User", f.owner)
		rec := httptest.NewRecorder()
		h.HandleTrust(rec, req)
		var resp trustResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); rec.Code != http.StatusOK || err != nil {
			t.Fatalf("%d %v %s", rec.Code, err, rec.Body)
		}
		return resp
	}
	if plain := get(""); plain.AnchorSample != nil {
		t.Fatal("the plain view carried the anchored sample")
	}
	resp := get("?replay=1")
	if resp.AnchorSample == nil || len(resp.AnchorSample.Items) != res.Settled || len(resp.AnchorSample.Corpus) != resp.AnchorSample.Decisions {
		t.Fatalf("anchor sample: %+v", resp.AnchorSample)
	}
}

// L1: the runtime's Start runs the sampler.
func TestStartRunsTheAnchorSampler(t *testing.T) {
	src, err := os.ReadFile("runtime.go")
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(src), "func (rt *Runtime) Start(")
	j := strings.Index(string(src), "func (rt *Runtime) warmCorpusSearchIndex")
	if i < 0 || j < i || !strings.Contains(string(src)[i:j], "rt.startAnchorSampler(ctx)") {
		t.Fatal("L1: Start does not start the anchor sampler")
	}
}

// Review r1 (MA7.10), written first:
//  R1 a torn log tail neither stops the pass nor fails the route; a corrupt
//     middle line degrades the sample with a note and a 200.
//  R2 a log under another seed or rate is set aside and sampling goes on.
//  R6 an item is not decided twice in one pass.
//  R7 the sampler does not run in a shot sandbox, a stub-provider runtime or
//     a capsule over a store copy.
//  D2 a same-family settlement is not an independent label.
//  D6 an undecided item does not escape the ledger window.

func (f *anchorFixture) logPath() string {
	return filepath.Join(filepath.Dir(f.rt.cfg.StorePath), spotLogName)
}

func TestAnchorPassSurvivesATornLogTailAndTheRouteSurvivesACorruptOne(t *testing.T) {
	f := newAnchorFixture(t, 100)
	if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err != nil {
		t.Fatal(err)
	}
	before, _ := trust.LoadSpotLog(f.logPath())
	fh, _ := os.OpenFile(f.logPath(), os.O_APPEND|os.O_WRONLY, 0o600)
	fh.WriteString(`{"record_id":"torn","sam`)
	fh.Close()
	res, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
	if err != nil {
		t.Fatalf("R1: a torn tail stopped the pass: %v", err)
	}
	after, err := trust.LoadSpotLog(f.logPath())
	if err != nil || len(after) != len(before)+res.Decided {
		t.Fatalf("R1: %d before, %d decided, %d after, %v", len(before), res.Decided, len(after), err)
	}
	// a corrupt line in the middle stops the pass but not the route
	raw, _ := os.ReadFile(f.logPath())
	lines := strings.SplitN(string(raw), "\n", 3)
	os.WriteFile(f.logPath(), []byte(lines[0]+"\nnot json\n"+lines[2]), 0o600)
	if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err == nil {
		t.Fatal("R1: a corrupt middle line was trusted by the pass")
	}
	s, err := f.rt.anchorSample(context.Background(), f.owner, f.computer)
	if err != nil || s.LogNote == "" || s.Decisions != 0 {
		t.Fatalf("R1: the sample must degrade, not fail: %+v %v", s, err)
	}
}

func TestAnchorPassSetsAStaleLogAsideAndGoesOn(t *testing.T) {
	f := newAnchorFixture(t, 100)
	for name, mk := range map[string]func() *trust.SpotSampler{
		"another seed": func() *trust.SpotSampler {
			s, _ := trust.NewSpotSampler([]byte("an-older-store-seed-xyz"), anchorSpotRate)
			return s
		},
		"another rate": func() *trust.SpotSampler {
			seed, _ := f.rt.store.TrustAnchors().Seed(context.Background(), f.owner, f.computer)
			s, _ := trust.NewSpotSampler(seed, 0.5)
			return s
		},
	} {
		os.Remove(f.logPath())
		l, _ := trust.OpenSpotLog(f.logPath())
		for i := 0; i < 7; i++ {
			_ = l.Append(mk().Decide(fmt.Sprintf("old-%d", i), time.Now()))
		}
		_ = l.Close()
		res, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
		if err != nil || res.Decided == 0 {
			t.Fatalf("R2 (%s): %+v %v", name, res, err)
		}
		aside, _ := filepath.Glob(f.logPath() + ".stale-*")
		if len(aside) == 0 {
			t.Fatalf("R2 (%s): the stale log was not set aside", name)
		}
		s, _ := f.rt.anchorSample(context.Background(), f.owner, f.computer)
		if s.Unreplayable < 7 {
			t.Fatalf("R2 (%s): unreplayable %d", name, s.Unreplayable)
		}
		for _, a := range aside {
			os.Remove(a)
		}
	}
}

func TestAnchorItemsAreNotDecidedTwiceAndSameFamilyIsNotAnIndependentLabel(t *testing.T) {
	dup := []anchorItemRef{{id: "a#q0"}, {id: "b#q0"}, {id: "a#q0"}}
	if got := dedupeAnchorItems(dup); len(got) != 2 {
		t.Fatalf("R6: %d", len(got))
	}
	f := newAnchorFixture(t, 40)
	for i := 40; i < 60; i++ {
		f.writeJudged(t, i, true)
	}
	items, err := f.rt.anchorItems(context.Background(), f.owner, f.computer, map[string]bool{})
	if err != nil || len(items) != 40 {
		t.Fatalf("D2: %d items (want the 40 independent ones): %v", len(items), err)
	}
}

func TestAnchorSamplerRefusesAShotSandboxAStubProviderAndACapsule(t *testing.T) {
	ok := provideriface.Config{StorePath: "/mnt/persistent/state", LLMProvider: "chatgpt"}
	if why := anchorSamplerRefusal(ok); why != "" {
		t.Fatalf("R7: the computer's own runtime was refused: %s", why)
	}
	if anchorSamplerRefusal(provideriface.Config{StorePath: "/computer/state", LLMProvider: "chatgpt"}) == "" {
		t.Fatal("R7: a capsule runtime ran the sampler")
	}
	if anchorSamplerRefusal(provideriface.Config{StorePath: "/x/state", LLMProvider: "stub"}) == "" {
		t.Fatal("R7: a stub-provider runtime ran the sampler")
	}
	t.Setenv("CHOIR_GUEST_TRUST_LOOPBACK", "1")
	if anchorSamplerRefusal(ok) == "" {
		t.Fatal("R7: a shot-sandbox runtime ran the sampler")
	}
	rt, _ := testRuntime(t)
	ran := make(chan struct{}, 1)
	rt.anchorTick, rt.anchorInitialDelay = time.Millisecond, time.Millisecond
	rt.anchorOwner = func() string { return "o" }
	rt.anchorPassHook = func() { ran <- struct{}{} }
	rt.startAnchorSampler(context.Background())
	select {
	case <-ran:
		t.Fatal("R7: the loop started under the sandbox marker")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAnchorPassReachesItemsOlderThanTheLedgerWindow(t *testing.T) {
	old := anchorLedgerWindow
	anchorLedgerWindow = 10
	t.Cleanup(func() { anchorLedgerWindow = old })
	f := newAnchorFixture(t, 100)
	for i := 0; i < 40; i++ {
		if r, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err != nil {
			t.Fatal(err)
		} else if r.Decided == 0 && r.Deferred == 0 {
			break
		}
	}
	s, _ := f.rt.anchorSample(context.Background(), f.owner, f.computer)
	if s.Decisions != 100 {
		t.Fatalf("D6: %d of 100 items decided with a window of 10", s.Decisions)
	}
}

// A sample read while a pass runs is safe (run under -race).
func TestAnchorSampleReadsWhileAPassRuns(t *testing.T) {
	f := newAnchorFixture(t, 100)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20; i++ {
			if _, err := f.rt.anchorSample(context.Background(), f.owner, f.computer); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	for i := 0; i < 5; i++ {
		if _, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("the reader never finished")
	}
}

// Review r2: a deferred item does not wedge the line.

func TestAPoisonItemDoesNotStopLaterItemsAndIsGivenUpAfterTheDeferralBound(t *testing.T) {
	f := newAnchorFixture(t, 100)
	f.judge.poison = true
	var res anchorPassResult
	for i := 0; i < anchorMaxDeferrals+3; i++ {
		r, err := f.rt.runAnchorPass(context.Background(), f.owner, f.computer)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && (r.Decided == 0 || r.Deferred != 1) {
			t.Fatalf("Q: the first pass stopped at the poison item: %+v", r)
		}
		res.Decided += r.Decided
	}
	s, _ := f.rt.anchorSample(context.Background(), f.owner, f.computer)
	if s.GaveUp != 1 {
		t.Fatalf("Q: the poison item was not given up after %d deferrals: %+v", anchorMaxDeferrals, s)
	}
	if s.Decisions != 100 {
		t.Fatalf("Q: %d of 100 decided behind a poison item", s.Decisions)
	}
	// it is counted as unsettled after retries, and it was asked exactly M times
	asked := 0
	for _, id := range f.judge.ids {
		if id == f.judge.poisonID {
			asked++
		}
	}
	if asked != anchorMaxDeferrals || s.Unsettled < 1 {
		t.Fatalf("Q: asked %d times (bound %d), unsettled %d", asked, anchorMaxDeferrals, s.Unsettled)
	}
	for _, a := range f.anchors(t) {
		if a.ItemID == f.judge.poisonID {
			t.Fatal("the poison item was anchored")
		}
	}
	// a stop (a cancelled context) is not a per-call failure: nothing deferred
	g := newAnchorFixture(t, 100)
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, _ := g.rt.runAnchorPass(cctx, g.owner, g.computer); r.Deferred != 0 || r.Decided != 0 {
		t.Fatalf("a stop was counted as a failure: %+v", r)
	}
}

func TestAnchorSamplerRefusesACapsuleRootExactly(t *testing.T) {
	if anchorSamplerRefusal(provideriface.Config{StorePath: "/computer", LLMProvider: "chatgpt"}) == "" {
		t.Fatal("the capsule root itself ran the sampler")
	}
}
