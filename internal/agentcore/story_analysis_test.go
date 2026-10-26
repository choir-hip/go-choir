package agentcore

// The story analysis (docs/problems/related-stories-no-entity-layer-
// 2026-10-26.md, slice c0 §3 and §4): with research.related_story on, the
// host builds a story's entity analysis at its research desk's first cell:
// the doc's head, the computer's own extraction, corpusd's candidates, and
// System 1's answers to the questions that follow from them, recorded as
// this run's judgments. Every cell's frame carries it. Ways it can fail,
// written before the code:
//
//  G9  The preload is over 32 KB; it carries story text; it is another
//      story's; it is present with the setting off, for an unbound desk, or
//      for another desk.
//  G10 A crash between the analysis and the first cell re-pays System 1 on
//      the next run; the replay is recorded under another run or story.
//  R1  (review 304 item 1) A replay keeps the old run's receipt: Relate's
//      same-run basis refuses every preloaded citation. A replay is a fresh
//      answered event of this run, its receipt_ref rewritten to it, with no
//      request record (no cap, no cost) and a pointer to what it replays.
//  A1  The cost bound: at most two System 1 calls of 12 questions a story.
//  A2  A second cell of the run asks again: the analysis is built once a run.
//  A3  The platform cannot answer: the analysis says so and asks nothing.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

type fakeCandidatePublisher struct {
	*fakeEntityPublisher
	cmu   sync.Mutex
	resp  types.EntityCandidatesResponse
	err   error
	reqs  []types.EntityCandidatesRequest
	reply func(types.EntityCandidatesRequest) types.EntityCandidatesResponse
}

func (f *fakeCandidatePublisher) EntityCandidates(_ context.Context, req types.EntityCandidatesRequest) (types.EntityCandidatesResponse, error) {
	f.cmu.Lock()
	defer f.cmu.Unlock()
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return types.EntityCandidatesResponse{}, f.err
	}
	if f.reply != nil {
		return f.reply(req), nil
	}
	return f.resp, nil
}

func (f *fakeCandidatePublisher) candidateCalls() []types.EntityCandidatesRequest {
	f.cmu.Lock()
	defer f.cmu.Unlock()
	return append([]types.EntityCandidatesRequest(nil), f.reqs...)
}

const analysisStoryTitle = "Zelenskyy meets Anthropic on AI safety"

const analysisStoryText = `Volodymyr Zelenskyy met executives from Anthropic in Kyiv on Tuesday to discuss the safety of artificial intelligence systems.
The meeting, which lasted three hours, covered the use of language models by the Ukrainian government during the war.
Zelenskyy said the talks with Anthropic were productive and that Ukraine would publish guidelines before the NATO summit.
Officials from NATO were not present at the meeting, according to a statement released by the presidential office afterwards.`

type analysisFixture struct {
	*entityFixture
	cpub *fakeCandidatePublisher
}

func newAnalysisFixture(t *testing.T) *analysisFixture {
	t.Helper()
	f := newEntityFixture(t)
	cpub := &fakeCandidatePublisher{fakeEntityPublisher: f.epub, resp: types.EntityCandidatesResponse{
		Candidates: []types.EntityCandidate{
			{Source: types.EntityCandidateExisting, EntityID: "wd:Q3874799", Name: "Volodymyr Zelenskyy", Type: types.EntityTypePerson, Phrase: "Volodymyr Zelenskyy", Count: 2, Sentences: 2, InTitle: true, InLead: true, Rarity: 4, Score: 9,
				Facts: &types.EntityFacts{Stories: 12, CentralStories: 9, PriorStories: []types.EntityStoryRef{{PublicationID: "pub-00000000000000000000000000000001", Title: "An earlier story"}}}},
			{Source: types.EntityCandidateExtracted, Phrase: "Anthropic", Count: 2, Sentences: 2, InTitle: true, InLead: true, Rarity: 5, Score: 8},
			{Source: types.EntityCandidateExtracted, Phrase: "NATO", Count: 2, Sentences: 2, Acronym: true, Rarity: 2, Score: 4},
		},
		Lexicon: types.EntityLexiconSize{Entities: 40, Keys: 55},
	}}
	f.rt.SetStoryPublisher(cpub)
	af := &analysisFixture{entityFixture: f, cpub: cpub}
	seedStoryDoc(t, f.s, f.owner, f.computer, "doc-story", analysisStoryTitle, analysisStoryText)
	return af
}

// seedStoryDoc starts a document lifecycle whose head is the story text.
func seedStoryDoc(t *testing.T, s *store.Store, owner, computer, docID, title, text string) {
	t.Helper()
	trajectoryID, texture := "trajectory-"+docID, agentprofile.Texture+":"+docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "start-" + docID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-" + docID, Objective: "write the story", AssignedAgentID: texture, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, Title: title, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "rev-" + docID, DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: owner, Content: text, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: texture, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle %s: %v", docID, err)
	}
}

func (f *analysisFixture) deskExec(runID, docID string) toolregistry.ExecutionContext {
	e := f.exec(runID)
	e.AgentID = "research:" + docID
	return e
}

func (f *analysisFixture) analyse(t *testing.T, role, runID, docID string) *yaegikernel.StoryAnalysis {
	t.Helper()
	return f.rt.storyAnalysisFor(context.Background(), role, f.deskExec(runID, docID))
}

func (f *analysisFixture) judgeCalls() int {
	f.judge.mu.Lock()
	defer f.judge.mu.Unlock()
	return f.judge.calls
}

// system1Events returns the tape's System 1 events of one run and phase.
func (f *analysisFixture) system1Events(t *testing.T, runID, phase string) []types.EventRecord {
	t.Helper()
	var out []types.EventRecord
	for _, e := range f.tape(t) {
		if e.Kind == types.EventSystem1Judgment && e.RunID == runID && e.Phase == phase {
			out = append(out, e)
		}
	}
	return out
}

func analysisJSON(t *testing.T, a *yaegikernel.StoryAnalysis) string {
	t.Helper()
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestStoryAnalysisAsksSystem1AndItsCitationsHoldForRelate(t *testing.T) {
	f := newAnalysisFixture(t)
	f.turnOn(t)
	a := f.analyse(t, agentprofile.Research, "run-a1", "doc-story")
	if a == nil || a.Error != "" {
		t.Fatalf("analysis = %+v", a)
	}
	if want := types.StoryPublicationID(f.owner, "doc-story"); a.Story.PublicationID != want || a.Story.DocID != "doc-story" || a.Story.RevisionID != "rev-doc-story" || a.Story.Title != analysisStoryTitle {
		t.Fatalf("story = %+v, want the desk's own story %s at its head", a.Story, want)
	}
	reqs := f.cpub.candidateCalls()
	if len(reqs) != 1 || reqs[0].OwnerID != f.owner || reqs[0].ComputerID != f.computer || reqs[0].Title != analysisStoryTitle || reqs[0].Text != analysisStoryText || len(reqs[0].Proposals) == 0 {
		t.Fatalf("candidates requests = %+v, want one with the head text and the computer's proposals", reqs)
	}
	if len(a.Candidates) != 3 {
		t.Fatalf("candidates = %d, want 3", len(a.Candidates))
	}
	existing, phrase := a.Candidates[0], a.Candidates[1]
	if existing.Role == nil || existing.Role.ReceiptRef == "" || existing.Role.QuestionID == "" || existing.Role.Top != types.EntityRoleCentral || existing.Kind != nil {
		t.Fatalf("existing candidate answers = role %+v kind %+v, want a role answer only", existing.Role, existing.Kind)
	}
	if phrase.Role == nil || phrase.Kind == nil || phrase.Kind.Top != types.EntityTypePerson || len(phrase.Kind.Probabilities) != 8 {
		t.Fatalf("phrase answers = role %+v kind %+v, want role and a type over the six types, not an entity and none of these fit", phrase.Role, phrase.Kind)
	}
	if a.Novelty == nil || a.Novelty.ReceiptRef == "" || len(a.Novelty.Probabilities) != 2 {
		t.Fatalf("novelty = %+v", a.Novelty)
	}
	if f.judgeCalls() != 1 {
		t.Fatalf("judge calls = %d, want 1 (6 questions)", f.judgeCalls())
	}

	// The citation holds for Relate on the same run, for the own story.
	payload, _ := json.Marshal(yaegikernel.RelatePayload{Story: "", Entities: []yaegikernel.RelateEntity{
		{EntityID: existing.EntityID, Role: types.EntityRoleCentral, JudgmentRef: existing.Role.ReceiptRef, QuestionID: existing.Role.QuestionID},
	}})
	if _, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.deskExec("run-a1", "doc-story"), yaegikernel.ActionRelate, payload); err != nil {
		t.Fatalf("Relate citing the analysis: %v", err)
	}
}

func TestStoryAnalysisIsBuiltOnceARun(t *testing.T) { // A2
	f := newAnalysisFixture(t)
	f.turnOn(t)
	a := f.analyse(t, agentprofile.Research, "run-once", "doc-story")
	b := f.analyse(t, agentprofile.Research, "run-once", "doc-story")
	if f.judgeCalls() != 1 || len(f.cpub.candidateCalls()) != 1 {
		t.Fatalf("judge calls %d, candidate reads %d; want one each for two cells of one run", f.judgeCalls(), len(f.cpub.candidateCalls()))
	}
	if a.Candidates[0].Role.ReceiptRef != b.Candidates[0].Role.ReceiptRef {
		t.Fatalf("second cell's receipt %q differs from the first's %q", b.Candidates[0].Role.ReceiptRef, a.Candidates[0].Role.ReceiptRef)
	}
}

func TestStoryAnalysisReplaysAcrossRunsAsAFreshEventOfTheNewRun(t *testing.T) { // G10, R1
	f := newAnalysisFixture(t)
	f.turnOn(t)
	first := f.analyse(t, agentprofile.Research, "run-r1", "doc-story")
	if first == nil || first.Error != "" {
		t.Fatalf("first analysis = %+v", first)
	}
	calls := f.judgeCalls()
	old := f.system1Events(t, "run-r1", system1PhaseAnswered)

	second := f.analyse(t, agentprofile.Research, "run-r2", "doc-story")
	if second == nil || second.Error != "" {
		t.Fatalf("second analysis = %+v", second)
	}
	if f.judgeCalls() != calls {
		t.Fatalf("judge calls %d -> %d: the replay re-paid System 1", calls, f.judgeCalls())
	}
	if req := f.system1Events(t, "run-r2", system1PhaseRequested); len(req) != 0 {
		t.Fatalf("the replay wrote %d request records; a replay counts toward no cap", len(req))
	}
	fresh := f.system1Events(t, "run-r2", system1PhaseAnswered)
	if len(fresh) != len(old) {
		t.Fatalf("run-r2 answered events = %d, want %d (one replay per call)", len(fresh), len(old))
	}
	oldIDs := map[string]bool{}
	for _, e := range old {
		oldIDs[e.EventID] = true
	}
	freshIDs := map[string]bool{}
	for _, e := range fresh {
		if oldIDs[e.EventID] {
			t.Fatalf("replay event %s reuses the old run's id", e.EventID)
		}
		freshIDs[e.EventID] = true
		var rec struct {
			ReplayOf string          `json:"replay_of"`
			Result   json.RawMessage `json:"result"`
			Cost     float64         `json:"cost_usd"`
		}
		if err := json.Unmarshal(e.Payload, &rec); err != nil || !oldIDs[rec.ReplayOf] || rec.Cost != 0 {
			t.Fatalf("replay payload = %s, want replay_of an old answer and no cost", e.Payload)
		}
		var result system1Result
		if err := json.Unmarshal(rec.Result, &result); err != nil || result.ReceiptRef != "system1:"+e.EventID {
			t.Fatalf("replay result receipt = %q, want system1:%s", result.ReceiptRef, e.EventID)
		}
	}
	ref := second.Candidates[0].Role.ReceiptRef
	if !freshIDs[strings.TrimPrefix(ref, "system1:")] {
		t.Fatalf("second analysis cites %q, want one of the new run's events", ref)
	}
	payload, _ := json.Marshal(yaegikernel.RelatePayload{Story: "", Entities: []yaegikernel.RelateEntity{
		{EntityID: second.Candidates[0].EntityID, Role: types.EntityRoleCentral, JudgmentRef: ref, QuestionID: second.Candidates[0].Role.QuestionID},
	}})
	if _, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.deskExec("run-r2", "doc-story"), yaegikernel.ActionRelate, payload); err != nil {
		t.Fatalf("Relate on the new run citing the replay: %v", err)
	}

	// Another story is asked afresh, never answered with this one's replay.
	seedStoryDoc(t, f.s, f.owner, f.computer, "doc-other", "Another story", strings.ReplaceAll(analysisStoryText, "Kyiv", "Lviv"))
	other := f.analyse(t, agentprofile.Research, "run-r3", "doc-other")
	if other == nil || other.Error != "" || other.Story.DocID != "doc-other" {
		t.Fatalf("other analysis = %+v", other)
	}
	if f.judgeCalls() != calls+1 {
		t.Fatalf("judge calls = %d, want %d: another story's text is another request", f.judgeCalls(), calls+1)
	}
}

func TestStoryAnalysisIsAbsentOffUnboundOrForAnotherDesk(t *testing.T) { // G9
	f := newAnalysisFixture(t)
	if a := f.analyse(t, agentprofile.Research, "run-off", "doc-story"); a != nil {
		t.Fatalf("setting off: analysis = %+v, want none", a)
	}
	f.turnOn(t)
	if a := f.analyse(t, agentprofile.Research, "run-unbound", f.owner); a != nil {
		t.Fatalf("unbound desk: analysis = %+v, want none", a)
	}
	for _, role := range []string{agentprofile.Texture, agentprofile.Management, yaegikernel.SessionRoleResearchChild, ""} {
		if a := f.analyse(t, role, "run-desk-"+role, "doc-story"); a != nil {
			t.Fatalf("desk %q: analysis = %+v, want none", role, a)
		}
	}
	if f.judgeCalls() != 0 || len(f.cpub.candidateCalls()) != 0 {
		t.Fatalf("absent analyses made %d judge calls and %d candidate reads", f.judgeCalls(), len(f.cpub.candidateCalls()))
	}
}

func TestStoryAnalysisIsBoundedAndCarriesNoStoryText(t *testing.T) { // G9, A1
	f := newAnalysisFixture(t)
	f.turnOn(t)
	long := func(prefix string, n int) string { return prefix + strings.Repeat("x", n-len(prefix)) }
	var cands []types.EntityCandidate
	for i := 0; i < types.EntityCandidatesMax; i++ {
		facts := &types.EntityFacts{Stories: 40, CentralStories: 20}
		for j := 0; j < types.EntityFactsMaxStories; j++ {
			facts.PriorStories = append(facts.PriorStories, types.EntityStoryRef{PublicationID: fmt.Sprintf("pub-%032x", i*10+j), Title: long(fmt.Sprintf("Title %d.%d ", i, j), 300)})
		}
		for j := 0; j < types.EntityFactsMaxCoEntities; j++ {
			facts.CoEntities = append(facts.CoEntities, types.EntityRef{EntityID: fmt.Sprintf("ent:%024x", i*10+j), Name: long(fmt.Sprintf("Co %d.%d ", i, j), types.EntityNameMaxRunes), Type: types.EntityTypeOrganization})
		}
		name := long(fmt.Sprintf("Entity %d ", i), types.EntityNameMaxRunes)
		cands = append(cands, types.EntityCandidate{Source: types.EntityCandidateExisting, EntityID: fmt.Sprintf("ent:%024x", i+1), Name: name, Type: types.EntityTypePerson, Phrase: name, Count: 2, Sentences: 2, Facts: facts})
	}
	f.cpub.resp = types.EntityCandidatesResponse{Candidates: cands}
	a := f.analyse(t, agentprofile.Research, "run-big", "doc-story")
	if a == nil || a.Error != "" {
		t.Fatalf("analysis = %+v", a)
	}
	raw := analysisJSON(t, a)
	if len(raw) > yaegikernel.StoryAnalysisMaxBytes {
		t.Fatalf("analysis is %d bytes, over %d", len(raw), yaegikernel.StoryAnalysisMaxBytes)
	}
	if len(a.Candidates) == 0 || a.Candidates[0].Role == nil {
		t.Fatalf("the bound dropped the top candidate or its answer: %+v", a.Candidates)
	}
	if f.judgeCalls() > 2 {
		t.Fatalf("judge calls = %d, want at most two a story", f.judgeCalls())
	}
	for _, sentence := range strings.Split(analysisStoryText, "\n") {
		if strings.Contains(raw, sentence[:40]) {
			t.Fatalf("the analysis carries story text: %q", sentence[:40])
		}
	}
}

func TestStoryAnalysisSaysWhenThePlatformCannotAnswer(t *testing.T) { // A3
	f := newAnalysisFixture(t)
	f.turnOn(t)
	f.cpub.err = errors.New("platform down")
	a := f.analyse(t, agentprofile.Research, "run-down", "doc-story")
	if a == nil || !strings.Contains(a.Error, "platform down") || len(a.Candidates) != 0 {
		t.Fatalf("analysis = %+v, want an error naming the platform failure and no candidates", a)
	}
	if f.judgeCalls() != 0 {
		t.Fatalf("judge calls = %d, want none without candidates", f.judgeCalls())
	}
}
