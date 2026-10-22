package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/llmcost"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// choir.System1, the research desk's closed-question verb. The ways it can
// fail are written first in
// docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md (F1-F11);
// each test below names the one it pins.

type fakeSystem1Judge struct {
	mu     sync.Mutex
	calls  int
	states []string
	// err fails the call.
	err error
	// omit is a question id the judge leaves unanswered.
	omit string
	// probs overrides the first choice's probability (default 0.7).
	first float64
}

func (f *fakeSystem1Judge) Judge(_ context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.states = append(f.states, string(state))
	if f.err != nil {
		return nil, f.err
	}
	first := f.first
	if first == 0 {
		first = 0.7
	}
	answers := map[string]any{}
	for id, raw := range questions {
		if id == f.omit {
			continue
		}
		var q struct {
			Choices []string `json:"choices"`
		}
		_ = json.Unmarshal(raw, &q)
		probs := map[string]float64{}
		rest := math.Round((1-first)/float64(len(q.Choices)-1)*100) / 100
		for i, c := range q.Choices {
			if i == 0 {
				probs[c] = first
			} else {
				probs[c] = rest
			}
		}
		// A "choice" is what the transport returns too; the verb must not hand it on.
		answers[id] = map[string]any{"type": "choice", "choice": q.Choices[0], "probabilities": probs, "confidence": 0.9}
	}
	return json.Marshal(map[string]any{"id": "gen-system1-test", "model": "typesafe/jev-1.13", "answers": answers,
		"usage": map[string]any{"input_tokens": 500, "output_tokens": 20, "cost": 0.00003}})
}

func (f *fakeSystem1Judge) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeSystem1Judge) lastState() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.states) == 0 {
		return ""
	}
	return f.states[len(f.states)-1]
}

type system1Fixture struct {
	rt       *Runtime
	s        *store.Store
	judge    *fakeSystem1Judge
	owner    string
	computer string
	capture  string // a capture id the fixture can read
	revision string // a story revision id the fixture can read
}

func newSystem1Fixture(t *testing.T) *system1Fixture {
	t.Helper()
	t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_RUN", "")
	t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_DAY", "")
	judge := &fakeSystem1Judge{}
	rt, s := testRuntime(t)
	rt.SetCandidateJudge(judge)
	owner, computer := "owner-system1", rt.cfg.ComputerID
	capture, err := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, owner, "court-ruling")
	if err != nil {
		t.Fatal(err)
	}
	rt.SetTextureCaptures(supportCaptures{capture: {Doc: capture, Version: "v1", URL: "https://example.org/court", Title: "Court ruling",
		Blocks: []quotematch.Block{{ID: "cid-1", Text: "The court ruled that the merger may proceed."}, {ID: "cid-2", Text: "The regulator appealed."}}}})
	seeded := seedListedSupportPairs(t, s, owner, computer, "doc-system1")
	return &system1Fixture{rt: rt, s: s, judge: judge, owner: owner, computer: computer, capture: capture, revision: seeded.RevisionID}
}

func (f *system1Fixture) enable(t *testing.T, perRun, perDay int) {
	t.Helper()
	t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_RUN", fmt.Sprint(perRun))
	t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_DAY", fmt.Sprint(perDay))
}

func (f *system1Fixture) exec(runID string) toolregistry.ExecutionContext {
	return toolregistry.ExecutionContext{RunID: runID, AgentID: "research:" + f.owner, OwnerID: f.owner, ComputerID: f.computer, Profile: agentprofile.Research}
}

func (f *system1Fixture) call(t *testing.T, role, runID string, qs ...yaegikernel.System1Question) (json.RawMessage, error) {
	t.Helper()
	payload, err := json.Marshal(yaegikernel.System1Payload{Questions: qs})
	if err != nil {
		t.Fatal(err)
	}
	return f.rt.deskEgress(context.Background(), role, f.exec(runID), yaegikernel.ActionSystem1, payload)
}

func question(id string, refs ...string) yaegikernel.System1Question {
	return yaegikernel.System1Question{ID: id, Text: "Do these report the same incident?", Choices: []string{"same incident", "later development", "unrelated"}, ContextRefs: refs}
}

type system1Decoded struct {
	ReceiptRef string `json:"receipt_ref"`
	Model      string `json:"model"`
	Replayed   bool   `json:"replayed"`
	Answers    []struct {
		ID            string             `json:"id"`
		Choices       []string           `json:"choices"`
		Probabilities map[string]float64 `json:"probabilities"`
		Choice        string             `json:"choice"`
		Error         string             `json:"error"`
	} `json:"answers"`
}

func decodeSystem1(t *testing.T, raw json.RawMessage) system1Decoded {
	t.Helper()
	var out system1Decoded
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

func (f *system1Fixture) tape(t *testing.T) []types.EventRecord {
	t.Helper()
	evs, err := f.s.ListAcknowledgedEventsByChannel(context.Background(), f.owner, system1Channel(f.computer), 1000)
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

func phases(evs []types.EventRecord) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Phase)
	}
	return out
}

// F1: off by default; with either cap unset the verb says what to set, and
// nothing reaches the judge or the tape.
func TestSystem1IsOffUntilBothCapsAreConfigured(t *testing.T) {
	f := newSystem1Fixture(t)
	for _, set := range []struct{ run, day string }{{"", ""}, {"5", ""}, {"", "50"}, {"0", "0"}, {"x", "9"}, {"-3", "9"}} {
		t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_RUN", set.run)
		t.Setenv("CHOIR_SYSTEM1_MAX_CALLS_PER_DAY", set.day)
		_, err := f.call(t, agentprofile.Research, "run-off", question("q1"))
		if err == nil || !strings.Contains(err.Error(), "CHOIR_SYSTEM1_MAX_CALLS_PER_RUN") || !strings.Contains(err.Error(), "CHOIR_SYSTEM1_MAX_CALLS_PER_DAY") || !strings.Contains(err.Error(), "off") {
			t.Fatalf("caps %+v: err = %v, want an error naming both settings", set, err)
		}
	}
	if f.judge.callCount() != 0 || len(f.tape(t)) != 0 {
		t.Fatalf("an off verb reached the judge (%d calls) or the tape (%d events)", f.judge.callCount(), len(f.tape(t)))
	}
}

// F3, F4: the full distribution comes back as the judge gave it, the call is
// on the tape with its digest, distributions, model and cost, and the cost is
// on the run's cost line. The transport's pick is not handed on.
func TestSystem1ReturnsTheDistributionAndRecordsTheCall(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 3, 10)
	raw, err := f.call(t, agentprofile.Research, "run-a", question("q1", f.capture, "story:"+f.revision), question("q2"))
	if err != nil {
		t.Fatal(err)
	}
	got := decodeSystem1(t, raw)
	if got.Replayed || !strings.HasPrefix(got.ReceiptRef, "system1:") || len(got.Answers) != 2 || got.Answers[0].ID != "q1" || got.Answers[1].ID != "q2" {
		t.Fatalf("result = %s", raw)
	}
	a := got.Answers[0]
	if a.Probabilities["same incident"] != 0.7 || a.Probabilities["later development"] != 0.15 || a.Probabilities["unrelated"] != 0.15 || len(a.Choices) != 3 {
		t.Fatalf("answer = %+v; want the judge's full distribution", a)
	}
	if a.Choice != "" || strings.Contains(string(raw), "jev") || strings.Contains(strings.ToLower(string(raw)), "model") {
		t.Fatalf("result %s hands on the judge's pick or names the backing judge", raw)
	}

	// The judge saw the resolved text, in the context, and the desk's wording as a question.
	state := f.judge.lastState()
	if !strings.Contains(state, "The court ruled that the merger may proceed.") || !strings.Contains(state, "Court ruling") || !strings.Contains(state, "The merger may proceed.") {
		t.Fatalf("judge state %s lacks the resolved capture and story text", state)
	}

	// Tape: requested then answered, on the run, with the digest.
	evs := f.tape(t)
	if p := phases(evs); len(p) != 2 || p[0] != "requested" || p[1] != "answered" {
		t.Fatalf("tape phases = %v", p)
	}
	for _, e := range evs {
		if e.Kind != types.EventSystem1Judgment || e.RunID != "run-a" || e.OwnerID != f.owner {
			t.Fatalf("event = %+v", e)
		}
	}
	var answered struct {
		Digest    string          `json:"request_digest"`
		Computer  string          `json:"computer_id"`
		Questions []string        `json:"question_ids"`
		Model     string          `json:"model"`
		Cost      float64         `json:"cost_usd"`
		RespID    string          `json:"response_id"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(evs[1].Payload, &answered); err != nil {
		t.Fatal(err)
	}
	if len(answered.Digest) != 64 || answered.Computer != f.computer || len(answered.Questions) != 2 || answered.Model != "typesafe/jev-1.13" || answered.Cost != 0.00003 || answered.RespID != "gen-system1-test" {
		t.Fatalf("answered payload = %s", evs[1].Payload)
	}
	if got.ReceiptRef != "system1:"+evs[1].EventID {
		t.Fatalf("receipt_ref %q does not name the answered event %q", got.ReceiptRef, evs[1].EventID)
	}
	if !strings.Contains(string(answered.Result), `"same incident":0.7`) {
		t.Fatalf("tape result %s lacks the distribution", answered.Result)
	}

	// Cost: on the run, where /api/costs reads it.
	all, err := f.s.ListEvents(context.Background(), "run-a", 100)
	if err != nil {
		t.Fatal(err)
	}
	var jev []llmcost.CostEntry
	for _, e := range llmcost.ExtractCostEntries(all) {
		if e.Model == "typesafe/jev-1.13" {
			jev = append(jev, e)
		}
	}
	if len(jev) != 1 || jev[0].InputTokens != 500 || jev[0].OutputTokens != 20 || jev[0].RunID != "run-a" {
		t.Fatalf("cost entries = %+v", jev)
	}
}

// F5: the same questions over the same context, in the same run, return the
// recorded answer: no second call, no second spend, no cap used.
func TestSystem1ReplayReturnsTheRecordedAnswerWithoutCallingTheJudge(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 1, 1)
	first, err := f.call(t, agentprofile.Research, "run-r", question("q1", f.capture))
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.call(t, agentprofile.Research, "run-r", question("q1", f.capture))
	if err != nil {
		t.Fatalf("replay with both caps spent: %v", err)
	}
	a, b := decodeSystem1(t, first), decodeSystem1(t, again)
	if f.judge.callCount() != 1 || !b.Replayed || a.Replayed || b.ReceiptRef != a.ReceiptRef || b.Answers[0].Probabilities["same incident"] != 0.7 {
		t.Fatalf("calls = %d; first %s; replay %s", f.judge.callCount(), first, again)
	}
	if n := len(f.tape(t)); n != 2 {
		t.Fatalf("replay wrote to the tape: %d events", n)
	}
	costs := 0
	all, _ := f.s.ListEvents(context.Background(), "run-r", 100)
	for _, e := range llmcost.ExtractCostEntries(all) {
		if e.Model == "typesafe/jev-1.13" {
			costs++
		}
	}
	if costs != 1 {
		t.Fatalf("replay recorded cost again: %d entries", costs)
	}
	// A different question is new work and the spent cap refuses it.
	if _, err := f.call(t, agentprofile.Research, "run-r", question("q2", f.capture)); err == nil || !strings.Contains(err.Error(), "per run") {
		t.Fatalf("a new question past the run cap: err = %v", err)
	}
	// Another run does not replay this run's answer: it is its own call.
	if _, err := f.call(t, agentprofile.Research, "run-other", question("q1", f.capture)); err == nil || !strings.Contains(err.Error(), "per day") {
		t.Fatalf("another run past the day cap: err = %v", err)
	}
}

// F5: the digest covers the resolved text, so a changed context is a new
// question and gets a new answer, not a stale one.
func TestSystem1ReplayIsKeyedToTheContextText(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	if _, err := f.call(t, agentprofile.Research, "run-k", question("q1", f.capture)); err != nil {
		t.Fatal(err)
	}
	f.rt.SetTextureCaptures(supportCaptures{f.capture: {Doc: f.capture, Version: "v2", Title: "Court ruling",
		Blocks: []quotematch.Block{{ID: "cid-1", Text: "The court reversed itself."}}}})
	again, err := f.call(t, agentprofile.Research, "run-k", question("q1", f.capture))
	if err != nil {
		t.Fatal(err)
	}
	if decodeSystem1(t, again).Replayed || f.judge.callCount() != 2 {
		t.Fatalf("changed context was answered from the tape: calls %d, %s", f.judge.callCount(), again)
	}
}

// F1: per-run and per-computer-day caps count attempts.
func TestSystem1CapsBoundTheCallsPerRunAndPerDay(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 2, 3)
	for i := 1; i <= 2; i++ {
		if _, err := f.call(t, agentprofile.Research, "run-1", question(fmt.Sprintf("q%d", i))); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if _, err := f.call(t, agentprofile.Research, "run-1", question("q3")); err == nil || !strings.Contains(err.Error(), "2 calls per run") {
		t.Fatalf("third call in a run: err = %v", err)
	}
	if _, err := f.call(t, agentprofile.Research, "run-2", question("q1")); err != nil {
		t.Fatalf("a second run has its own run cap: %v", err)
	}
	if _, err := f.call(t, agentprofile.Research, "run-3", question("q1")); err == nil || !strings.Contains(err.Error(), "3 calls per day") {
		t.Fatalf("fourth call in the day: err = %v", err)
	}
	if f.judge.callCount() != 3 {
		t.Fatalf("judge calls = %d, want 3", f.judge.callCount())
	}
	// Yesterday's calls do not count against today.
	yesterday := time.Now().UTC().Add(-26 * time.Hour)
	old := types.EventRecord{EventID: uuid.New().String(), Timestamp: yesterday, RunID: "run-old", OwnerID: f.owner, ChannelID: system1Channel(f.computer),
		Kind: types.EventSystem1Judgment, Phase: "requested", Payload: json.RawMessage(`{"computer_id":"` + f.computer + `"}`)}
	if err := f.s.AppendEvent(context.Background(), &old); err != nil {
		t.Fatal(err)
	}
	f.enable(t, 2, 4)
	if _, err := f.call(t, agentprofile.Research, "run-3", question("q1")); err != nil {
		t.Fatalf("yesterday's request counted against today: %v", err)
	}
}

// F6, F7: a failed call is on the tape as a failure, counts as an attempt,
// and the retry calls the judge again.
func TestSystem1AFailedCallCountsAndTheRetryCallsAgain(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 2, 10)
	f.judge.err = errors.New("Jev judgment provider unavailable")
	_, err := f.call(t, agentprofile.Research, "run-f", question("q1"))
	if err == nil || strings.Contains(strings.ToLower(err.Error()), "jev") || !strings.Contains(err.Error(), "choir.System1") {
		t.Fatalf("err = %v; want an error that says the call failed without naming the backing judge", err)
	}
	if p := phases(f.tape(t)); len(p) != 2 || p[0] != "requested" || p[1] != "failed" {
		t.Fatalf("tape phases = %v", p)
	}
	f.judge.mu.Lock()
	f.judge.err = nil
	f.judge.mu.Unlock()
	raw, err := f.call(t, agentprofile.Research, "run-f", question("q1"))
	if err != nil || decodeSystem1(t, raw).Replayed {
		t.Fatalf("retry = %s, %v; a failure is never replayed", raw, err)
	}
	if f.judge.callCount() != 2 {
		t.Fatalf("judge calls = %d", f.judge.callCount())
	}
	// Both attempts counted: the run's cap of 2 is spent.
	if _, err := f.call(t, agentprofile.Research, "run-f", question("q2")); err == nil || !strings.Contains(err.Error(), "per run") {
		t.Fatalf("third attempt: err = %v", err)
	}
}

// F6: a request left on the tape with no outcome (the computer restarted
// mid-call) counts toward the caps and is not an answer to replay.
func TestSystem1ACrashedRequestCountsAndNeverReplays(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 1, 10)
	req := types.EventRecord{EventID: uuid.New().String(), Timestamp: time.Now().UTC(), RunID: "run-x", OwnerID: f.owner, ChannelID: system1Channel(f.computer),
		Kind: types.EventSystem1Judgment, Phase: "requested", Payload: json.RawMessage(`{"computer_id":"` + f.computer + `","request_digest":"abc"}`)}
	if err := f.s.AppendEvent(context.Background(), &req); err != nil {
		t.Fatal(err)
	}
	if _, err := f.call(t, agentprofile.Research, "run-x", question("q1")); err == nil || !strings.Contains(err.Error(), "per run") {
		t.Fatalf("a crashed request did not count: err = %v", err)
	}
	if f.judge.callCount() != 0 {
		t.Fatalf("judge called %d times", f.judge.callCount())
	}
}

// F2: a context ref the runtime cannot read refuses the call before any
// spend or record; so does a ref that is not a capture id or a story
// version, or free text where evidence belongs.
func TestSystem1RefusesContextItCannotResolveBeforeAnySpend(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	missing, _ := objectgraph.BuildCanonicalID(objectgraph.WebCaptureObjectKind, f.owner, "nobody-captured-this")
	for _, ref := range []string{missing, "story:no-such-revision", "https://example.org/page", "The court ruled X and that settles it.", "story:", "obj:choir.texture_document:owner:doc"} {
		if _, err := f.call(t, agentprofile.Research, "run-ref", question("q1", ref)); err == nil || !strings.Contains(err.Error(), "context_refs") {
			t.Errorf("ref %q: err = %v, want a context_refs refusal", ref, err)
		}
	}
	if f.judge.callCount() != 0 || len(f.tape(t)) != 0 {
		t.Fatalf("a refused call spent: %d judge calls, %d events", f.judge.callCount(), len(f.tape(t)))
	}
}

// F2: context is bounded per ref, so one huge capture cannot bury the judge.
func TestSystem1BoundsEachContextText(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	big := strings.Repeat("word ", 5000)
	f.rt.SetTextureCaptures(supportCaptures{f.capture: {Doc: f.capture, Version: "v1", Title: "Big", Blocks: []quotematch.Block{{ID: "cid-1", Text: big}}}})
	if _, err := f.call(t, agentprofile.Research, "run-big", question("q1", f.capture)); err != nil {
		t.Fatal(err)
	}
	if n := len(f.judge.lastState()); n > 4000 {
		t.Fatalf("judge state is %d bytes for one capture; want it bounded", n)
	}
}

// F7: a judge that leaves a question unanswered gets that question reported
// as unanswered, never filled.
func TestSystem1ReportsAnUnansweredQuestionWithoutFillingIt(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	f.judge.omit = "q2"
	raw, err := f.call(t, agentprofile.Research, "run-o", question("q1"), question("q2"))
	if err != nil {
		t.Fatal(err)
	}
	got := decodeSystem1(t, raw)
	if len(got.Answers) != 2 || got.Answers[0].Error != "" || got.Answers[1].ID != "q2" || got.Answers[1].Error == "" || len(got.Answers[1].Probabilities) != 0 {
		t.Fatalf("result = %s", raw)
	}
}

// F7: no decision model bound on this computer.
func TestSystem1NeedsADecisionModel(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	f.rt.SetCandidateJudge(nil)
	if _, err := f.call(t, agentprofile.Research, "run-n", question("q1")); err == nil || !strings.Contains(err.Error(), "no decision model") {
		t.Fatalf("err = %v", err)
	}
	if len(f.tape(t)) != 0 {
		t.Fatal("a call with no judge wrote to the tape")
	}
}

// F8: the host validates too; a worker could be modified.
func TestSystem1HostRefusesAMalformedPayload(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	for name, payload := range map[string]string{
		"empty":         `{"questions":[]}`,
		"one choice":    `{"questions":[{"id":"q1","text":"t","choices":["a"]}]}`,
		"unknown field": `{"questions":[{"id":"q1","text":"t","choices":["a","b"],"evidence":"the court ruled X"}]}`,
		"not json":      `[`,
	} {
		if _, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.exec("run-v"), yaegikernel.ActionSystem1, json.RawMessage(payload)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if f.judge.callCount() != 0 {
		t.Fatal("a malformed payload reached the judge")
	}
}

// F9: the host refuses every session role but research, whatever the worker
// sent.
func TestSystem1HostRefusesEveryDeskButResearch(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	for _, role := range []string{agentprofile.Management, agentprofile.Engineering, agentprofile.Texture, yaegikernel.SessionRoleResearchChild, yaegikernel.SessionRoleManagementTask, ""} {
		if _, err := f.call(t, role, "run-role", question("q1")); err == nil || !strings.Contains(err.Error(), "research desk") {
			t.Errorf("role %q: err = %v, want a refusal naming the research desk", role, err)
		}
	}
	if f.judge.callCount() != 0 || len(f.tape(t)) != 0 {
		t.Fatalf("a refused role spent: %d calls, %d events", f.judge.callCount(), len(f.tape(t)))
	}
	if _, err := f.call(t, agentprofile.Research, "run-role", question("q1")); err != nil {
		t.Fatalf("research: %v", err)
	}
}

// The context a call resolves is read for the owner of the run: a capture
// store failure is an error, not an empty context.
func TestSystem1AStoreThatCannotReadACaptureRefusesTheCall(t *testing.T) {
	f := newSystem1Fixture(t)
	f.enable(t, 5, 5)
	f.rt.SetTextureCaptures(failingCaptures{})
	if _, err := f.call(t, agentprofile.Research, "run-s", question("q1", f.capture)); err == nil || !strings.Contains(err.Error(), "context_refs") {
		t.Fatalf("err = %v", err)
	}
	if f.judge.callCount() != 0 {
		t.Fatal("judge called with an unreadable context")
	}
}

type failingCaptures struct{}

func (failingCaptures) CaptureBlocks(context.Context, string, string) (texturewrite.Capture, error) {
	return texturewrite.Capture{}, errors.New("store down")
}

// Owner ruling (owner notes §29): System 1 does the related-stories reading,
// and a guest has no way to receive per-computer environment, so the owner
// turns it on with the research.system1 setting. On with no env caps, the
// default caps apply; env caps, when both are set, override them; off and no
// env is off, and the refusal names the setting.
func TestSystem1OwnerSettingTurnsItOnWithDefaultCaps(t *testing.T) {
	f := newSystem1Fixture(t)
	ctx := context.Background()
	if run, day := f.rt.system1Limits(ctx, f.owner, f.computer); run != 0 || day != 0 {
		t.Fatalf("no setting, no env: caps %d/%d, want off", run, day)
	}
	if _, err := f.call(t, agentprofile.Research, "run-setting-off", question("q1")); err == nil || !strings.Contains(err.Error(), store.SettingResearchSystem1) {
		t.Fatalf("off: err = %v, want the refusal to name %s", err, store.SettingResearchSystem1)
	}
	if err := f.s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer, Key: store.SettingResearchSystem1, Value: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if run, day := f.rt.system1Limits(ctx, f.owner, f.computer); run != system1DefaultCallsPerRun || day != system1DefaultCallsPerDay {
		t.Fatalf("setting on: caps %d/%d, want %d/%d", run, day, system1DefaultCallsPerRun, system1DefaultCallsPerDay)
	}
	if _, err := f.call(t, agentprofile.Research, "run-setting-on", question("q1")); err != nil {
		t.Fatalf("setting on: %v", err)
	}
	f.enable(t, 3, 10)
	if run, day := f.rt.system1Limits(ctx, f.owner, f.computer); run != 3 || day != 10 {
		t.Fatalf("env caps with the setting on: %d/%d, want 3/10", run, day)
	}
}
