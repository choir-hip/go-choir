package agentcore

// choir.RelateStories, the research desk's record of a judged pair relation
// (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md, "Build:
// the related_story edge"). The ways it can fail are written there first
// (B1 to B10); each test below names the ones it pins.
//
//  B1 The verb works for a desk other than research, or on a computer that
//     did not turn research.related_story on.
//  B2 A relation with no judgment behind it: a receipt the run never
//     recorded, another run's receipt, a question the receipt did not answer,
//     or a relation that is not one of that question's choices.
//  B3 The desk overrides System 1 silently: a relation that is not the top
//     choice needs a reason, and the tape says it overrode.
//  B4 Self pair, the same pair in both orders, a malformed or foreign-kind
//     id, an unknown relation, more than 12 pairs.
//  B9 The export fails after the tape write: the cell is told, the tape keeps
//     the record, and repeating the pair re-exports without a second record.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type fakeRelationPublisher struct {
	mu   sync.Mutex
	reqs []types.StoryRelationsRequest
	errs []error // one per call, in order; nil or missing is success
}

func (f *fakeRelationPublisher) PublishVersion(context.Context, StoryVersion) error { return nil }
func (f *fakeRelationPublisher) PublishVerdicts(context.Context, types.StoryVerdictsRequest) error {
	return nil
}
func (f *fakeRelationPublisher) PublishRelations(_ context.Context, req types.StoryRelationsRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	if n := len(f.reqs) - 1; n < len(f.errs) {
		return f.errs[n]
	}
	return nil
}

func (f *fakeRelationPublisher) calls() []types.StoryRelationsRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]types.StoryRelationsRequest(nil), f.reqs...)
}

type relateFixture struct {
	*system1Fixture
	pub *fakeRelationPublisher
	a   string // publication ids of two stories of the owner
	b   string
}

func newRelateFixture(t *testing.T) *relateFixture {
	t.Helper()
	f := newSystem1Fixture(t)
	f.enable(t, 10, 100)
	pub := &fakeRelationPublisher{}
	f.rt.SetStoryPublisher(pub)
	id := func(suffix string) string {
		out, err := objectgraph.BuildCanonicalID(objectgraph.ObjectKind("choir.publication"), f.owner, suffix)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	return &relateFixture{system1Fixture: f, pub: pub, a: id("story-a"), b: id("story-b")}
}

func (f *relateFixture) turnOn(t *testing.T) {
	t.Helper()
	if err := f.s.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: f.owner, ComputerID: f.computer, Key: store.SettingResearchRelatedStory, Value: true, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
}

// judged asks System 1 one question in the run and returns its receipt.
func (f *relateFixture) judged(t *testing.T, runID, questionID string) string {
	t.Helper()
	raw, err := f.call(t, agentprofile.Research, runID, question(questionID, f.capture))
	if err != nil {
		t.Fatalf("system1: %v", err)
	}
	return decodeSystem1(t, raw).ReceiptRef
}

func (f *relateFixture) relate(role, runID string, pairs ...yaegikernel.RelateStoriesPair) (json.RawMessage, error) {
	payload, err := json.Marshal(yaegikernel.RelateStoriesPayload{Pairs: pairs})
	if err != nil {
		return nil, err
	}
	return f.rt.deskEgress(context.Background(), role, f.exec(runID), yaegikernel.ActionRelateStories, payload)
}

func (f *relateFixture) pair(receipt, questionID, relation string) yaegikernel.RelateStoriesPair {
	return yaegikernel.RelateStoriesPair{StoryA: f.a, StoryB: f.b, Relation: relation, JudgmentRef: receipt, QuestionID: questionID}
}

func (f *relateFixture) relationEvents(t *testing.T) []types.EventRecord {
	t.Helper()
	var out []types.EventRecord
	for _, e := range f.tape(t) {
		if e.Kind == types.EventStoryRelation {
			out = append(out, e)
		}
	}
	return out
}

type relateDecoded struct {
	Recorded []struct {
		StoryA      string  `json:"story_a"`
		StoryB      string  `json:"story_b"`
		Relation    string  `json:"relation"`
		Probability float64 `json:"probability"`
		Top         bool    `json:"top"`
		Overrode    bool    `json:"overrode"`
		Replayed    bool    `json:"replayed"`
	} `json:"recorded"`
	Exported    bool   `json:"exported"`
	ExportError string `json:"export_error"`
}

func decodeRelate(t *testing.T, raw json.RawMessage) relateDecoded {
	t.Helper()
	var out relateDecoded
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

// B1
func TestRelateStoriesIsResearchOnlyAndOffUntilTheOwnerTurnsItOn(t *testing.T) {
	f := newRelateFixture(t)
	receipt := f.judged(t, "run-b1", "q1")
	pair := f.pair(receipt, "q1", types.StoryRelationSameIncident)

	_, err := f.relate(agentprofile.Research, "run-b1", pair)
	if err == nil || !strings.Contains(err.Error(), store.SettingResearchRelatedStory) {
		t.Fatalf("setting off: err = %v, want the refusal to name %s", err, store.SettingResearchRelatedStory)
	}
	f.turnOn(t)
	for _, role := range []string{agentprofile.Management, agentprofile.Texture, agentprofile.Engineering, yaegikernel.SessionRoleResearchChild, yaegikernel.SessionRoleManagementTask, ""} {
		if _, err := f.relate(role, "run-b1", pair); err == nil || !strings.Contains(err.Error(), "research") {
			t.Errorf("role %q: err = %v, want a refusal naming the research desk", role, err)
		}
	}
	if n := len(f.relationEvents(t)); n != 0 || len(f.pub.calls()) != 0 {
		t.Fatalf("refused calls left %d tape events and %d exports", n, len(f.pub.calls()))
	}
}

// B2
func TestRelateStoriesNeedsAJudgmentThisRunRecorded(t *testing.T) {
	f := newRelateFixture(t)
	f.turnOn(t)
	receipt := f.judged(t, "run-b2", "q1")
	otherRun := f.judged(t, "run-b2-other", "q9")

	cases := []struct {
		name string
		pair yaegikernel.RelateStoriesPair
		want string
	}{
		{"no receipt", f.pair("", "q1", types.StoryRelationSameIncident), "judgment_ref"},
		{"malformed receipt", f.pair("q1", "q1", types.StoryRelationSameIncident), "judgment_ref"},
		{"a receipt the run never recorded", f.pair("system1:00000000-0000-0000-0000-000000000000", "q1", types.StoryRelationSameIncident), "no System 1 answer"},
		{"another run's receipt", f.pair(otherRun, "q9", types.StoryRelationSameIncident), "no System 1 answer"},
		{"a question the receipt did not answer", f.pair(receipt, "q2", types.StoryRelationSameIncident), "q2"},
		{"a relation that is not a choice of the question", f.pair(receipt, "q1", types.StoryRelationSameSubject), "choices"},
	}
	for _, tc := range cases {
		if _, err := f.relate(agentprofile.Research, "run-b2", tc.pair); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want a refusal naming %q", tc.name, err, tc.want)
		}
	}
	if n := len(f.relationEvents(t)); n != 0 || len(f.pub.calls()) != 0 {
		t.Fatalf("refused calls left %d tape events and %d exports", n, len(f.pub.calls()))
	}
}

// B3, and the happy path: the tape holds the whole record, the export the minimum.
func TestRelateStoriesRecordsTheJudgmentOnTheTapeAndExportsTheEdge(t *testing.T) {
	f := newRelateFixture(t)
	f.turnOn(t)
	receipt := f.judged(t, "run-ok", "q1")

	raw, err := f.relate(agentprofile.Research, "run-ok", f.pair(receipt, "q1", types.StoryRelationSameIncident))
	if err != nil {
		t.Fatal(err)
	}
	got := decodeRelate(t, raw)
	if !got.Exported || got.ExportError != "" || len(got.Recorded) != 1 {
		t.Fatalf("result = %s", raw)
	}
	r := got.Recorded[0]
	if r.Relation != types.StoryRelationSameIncident || r.Probability != 0.7 || !r.Top || r.Overrode || r.Replayed {
		t.Fatalf("recorded = %+v, want same_incident at 0.7, top, not overrode, not replayed", r)
	}
	if r.StoryA >= r.StoryB {
		t.Fatalf("pair = %s %s, want the stored order a < b", r.StoryA, r.StoryB)
	}

	evs := f.relationEvents(t)
	if len(evs) != 1 || evs[0].Phase != "recorded" || evs[0].RunID != "run-ok" || evs[0].OwnerID != f.owner || evs[0].ChannelID != system1Channel(f.computer) {
		t.Fatalf("tape = %+v, want one recorded story.relation on the run and the system1 channel", evs)
	}
	var payload struct {
		StoryA, StoryB string
		Relation       string  `json:"relation"`
		Probability    float64 `json:"probability"`
		Top            bool    `json:"top"`
		Overrode       bool    `json:"overrode"`
		JudgmentRef    string  `json:"judgment_ref"`
		QuestionID     string  `json:"question_id"`
		Computer       string  `json:"computer_id"`
	}
	if err := json.Unmarshal(evs[0].Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Relation != "same_incident" || payload.Probability != 0.7 || !payload.Top || payload.JudgmentRef != receipt || payload.QuestionID != "q1" || payload.Computer != f.computer {
		t.Fatalf("payload = %s", evs[0].Payload)
	}

	calls := f.pub.calls()
	if len(calls) != 1 || calls[0].OwnerID != f.owner || calls[0].ComputerID != f.computer || len(calls[0].Relations) != 1 {
		t.Fatalf("export = %+v, want one request for the owner and computer", calls)
	}
	if e := calls[0].Relations[0]; e.StoryA != r.StoryA || e.StoryB != r.StoryB || e.Relation != "same_incident" {
		t.Fatalf("exported %+v, want the ordered pair and the relation", e)
	}

	// The relation event does not count as a System 1 call.
	for i := 0; i < 9; i++ {
		if _, err := f.call(t, agentprofile.Research, "run-ok", question(fmt.Sprintf("more%d", i), f.capture)); err != nil {
			t.Fatalf("system1 call %d after a relation: %v", i, err)
		}
	}
}

// B3
func TestRelateStoriesOverridingSystem1NeedsAReason(t *testing.T) {
	f := newRelateFixture(t)
	f.turnOn(t)
	receipt := f.judged(t, "run-b3", "q1") // same incident 0.7, later development 0.15

	p := f.pair(receipt, "q1", types.StoryRelationLaterDevelopment)
	for _, reason := range []string{"", "no", "because I said so"} { // under 20 characters
		p.Reason = reason
		if _, err := f.relate(agentprofile.Research, "run-b3", p); err == nil || !strings.Contains(err.Error(), "reason") {
			t.Errorf("reason %q: err = %v, want a refusal asking for a reason", reason, err)
		}
	}
	if n := len(f.relationEvents(t)); n != 0 {
		t.Fatalf("%d events after refused overrides", n)
	}
	p.Reason = "The second story reports the follow-up ruling the first only anticipates."
	raw, err := f.relate(agentprofile.Research, "run-b3", p)
	if err != nil {
		t.Fatal(err)
	}
	r := decodeRelate(t, raw).Recorded[0]
	if r.Top || !r.Overrode || r.Probability != 0.15 {
		t.Fatalf("recorded = %+v, want not top, overrode, p 0.15", r)
	}
	var payload struct {
		Overrode bool   `json:"overrode"`
		Reason   string `json:"reason"`
	}
	evs := f.relationEvents(t)
	if len(evs) != 1 || json.Unmarshal(evs[0].Payload, &payload) != nil || !payload.Overrode || !strings.Contains(payload.Reason, "follow-up ruling") {
		t.Fatalf("tape = %+v", evs)
	}
}

// B4
func TestRelateStoriesRefusesAMalformedPairBeforeAnyWrite(t *testing.T) {
	f := newRelateFixture(t)
	f.turnOn(t)
	receipt := f.judged(t, "run-b4", "q1")
	good := f.pair(receipt, "q1", types.StoryRelationSameIncident)

	self := good
	self.StoryB = self.StoryA
	badID := good
	badID.StoryA = "not-an-id"
	capture := good
	capture.StoryA = f.capture // a capture is not a publication
	unknown := good
	unknown.Relation = "unrelated"
	many := make([]yaegikernel.RelateStoriesPair, 0, 13)
	for i := 0; i < 13; i++ {
		p := good
		p.QuestionID = fmt.Sprintf("q%d", i)
		many = append(many, p)
	}
	cases := []struct {
		name  string
		pairs []yaegikernel.RelateStoriesPair
		want  string
	}{
		{"no pairs", nil, "at least one"},
		{"self pair", []yaegikernel.RelateStoriesPair{self}, "itself"},
		{"malformed id", []yaegikernel.RelateStoriesPair{badID}, "publication"},
		{"a capture id", []yaegikernel.RelateStoriesPair{capture}, "publication"},
		{"unknown relation", []yaegikernel.RelateStoriesPair{unknown}, "relation"},
		{"13 pairs", many, "12"},
	}
	for _, tc := range cases {
		if _, err := f.relate(agentprofile.Research, "run-b4", tc.pairs...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want a refusal naming %q", tc.name, err, tc.want)
		}
	}
	if n := len(f.relationEvents(t)); n != 0 || len(f.pub.calls()) != 0 {
		t.Fatalf("refused calls left %d tape events and %d exports", n, len(f.pub.calls()))
	}

	// The same pair in both orders is one relation.
	reversed := good
	reversed.StoryA, reversed.StoryB = good.StoryB, good.StoryA
	raw, err := f.relate(agentprofile.Research, "run-b4", good, reversed)
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeRelate(t, raw); len(got.Recorded) != 1 || len(f.relationEvents(t)) != 1 {
		t.Fatalf("both orders: %d recorded, %d events, want 1 and 1", len(got.Recorded), len(f.relationEvents(t)))
	}
}

// B9
func TestRelateStoriesExportFailureKeepsTheRecordAndARepeatReExports(t *testing.T) {
	f := newRelateFixture(t)
	f.turnOn(t)
	f.pub.errs = []error{fmt.Errorf("corpusd down")}
	receipt := f.judged(t, "run-b9", "q1")
	p := f.pair(receipt, "q1", types.StoryRelationSameIncident)

	raw, err := f.relate(agentprofile.Research, "run-b9", p)
	if err != nil {
		t.Fatalf("an export failure is a result, not a refusal of the record: %v", err)
	}
	got := decodeRelate(t, raw)
	if got.Exported || !strings.Contains(got.ExportError, "corpusd down") || len(got.Recorded) != 1 || got.Recorded[0].Replayed {
		t.Fatalf("result = %s, want recorded, not exported, the error named", raw)
	}
	if n := len(f.relationEvents(t)); n != 1 {
		t.Fatalf("%d relation events after a failed export, want the record", n)
	}

	raw, err = f.relate(agentprofile.Research, "run-b9", p)
	if err != nil {
		t.Fatal(err)
	}
	got = decodeRelate(t, raw)
	if !got.Exported || !got.Recorded[0].Replayed {
		t.Fatalf("repeat = %s, want exported and replayed", raw)
	}
	if n := len(f.relationEvents(t)); n != 1 {
		t.Fatalf("%d relation events after the repeat, want still 1", n)
	}
	if n := len(f.pub.calls()); n != 2 {
		t.Fatalf("%d export attempts, want 2 (the failure, then the repeat)", n)
	}

	// No platform to publish to: recorded, not exported, and it says so.
	g := newRelateFixture(t)
	g.turnOn(t)
	g.rt.SetStoryPublisher(nil)
	r2 := g.judged(t, "run-b9-b", "q1")
	raw, err = g.relate(agentprofile.Research, "run-b9-b", g.pair(r2, "q1", types.StoryRelationSameIncident))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeRelate(t, raw); got.Exported || got.ExportError == "" {
		t.Fatalf("no publisher: %s, want not exported with a reason", raw)
	}
}
