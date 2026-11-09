package agentcore

// The finish rule (docs/problems/related-stories-no-entity-layer-
// 2026-10-26.md, slice c, with review 304 item 2). On a story's research
// desk with research.related_story on, a final Report to Texture is refused
// until the tape holds a story.entities record for the story, or this run's
// choir.Relate failed on the platform's side (no platform, a transport
// error, a 5xx). Ways it can fail, written first:
//
//  F1 The rule applies to a desk other than a story's research desk, or
//     with the setting off.
//  F2 A final Report passes with no record and no platform failure; a
//     record of another story counts.
//  F3 A platform failure holds the round forever (it must let this run's
//     final Report through), or another run's failure frees this run.
//  F3b (item 2) A host-side refusal frees the Report: a 4xx from the
//     platform's own checks, a missing basis or an unreasoned override is a
//     fixable error, and the rule still holds.
//  F4 An interim packet is refused: only the final one waits.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func (f *analysisFixture) finishGate(t *testing.T, runID, deskAgentID, profile string) error {
	t.Helper()
	run := types.RunRecord{RunID: runID, AgentID: deskAgentID, OwnerID: f.owner, ComputerID: f.computer, AgentProfile: profile, AgentRole: profile}
	scope := ReductionScope{FromAgentID: deskAgentID, DeskAgentID: deskAgentID, FromRole: profile, RunID: runID, OwnerID: f.owner, ComputerID: f.computer, CellID: "finish-" + runID}
	r := &rlmCallReduction{active: true, mb: f.rt, st: f.rt.store, ledger: f.rt.store, scope: scope, rec: &run}
	return r.requireStoryEntitiesRecorded(context.Background())
}

// relateOwn calls choir.Relate("") on the story desk for the analysis's
// first candidate.
func (f *analysisFixture) relateOwn(t *testing.T, runID string) error {
	t.Helper()
	a := f.analyse(t, agentprofile.Research, runID, "doc-story")
	if a == nil || len(a.Candidates) == 0 || a.Candidates[0].Role == nil {
		t.Fatalf("analysis = %+v", a)
	}
	c := a.Candidates[0]
	payload, _ := json.Marshal(yaegikernel.RelatePayload{Entities: []yaegikernel.RelateEntity{
		{EntityID: c.EntityID, Role: types.EntityRoleCentral, JudgmentRef: c.Role.ReceiptRef, QuestionID: c.Role.QuestionID},
	}})
	_, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.deskExec(runID, "doc-story"), yaegikernel.ActionRelate, payload)
	return err
}

func TestFinishRuleAppliesOnlyToAStoryResearchDeskWithTheSettingOn(t *testing.T) { // F1
	f := newAnalysisFixture(t)
	if err := f.finishGate(t, "run-f1", "research:doc-story", agentprofile.Research); err != nil {
		t.Fatalf("setting off: %v, want no rule", err)
	}
	f.turnOn(t)
	if err := f.finishGate(t, "run-f1", "research:"+f.owner, agentprofile.Research); err != nil {
		t.Fatalf("unbound research desk: %v, want no rule", err)
	}
	if err := f.finishGate(t, "run-f1", "management:"+f.owner, agentprofile.Management); err != nil {
		t.Fatalf("management: %v, want no rule", err)
	}
	if err := f.finishGate(t, "run-f1", "research:doc-story", agentprofile.Research); err == nil || !strings.Contains(err.Error(), "choir.Relate") {
		t.Fatalf("story research desk with no record: %v, want the refusal to name choir.Relate", err)
	}
}

func TestFinishRuleHoldsUntilTheStoryHasARecord(t *testing.T) { // F2
	f := newAnalysisFixture(t)
	f.turnOn(t)
	// A record of another story does not count.
	if _, err := f.attach(agentprofile.Research, "run-other", newVucic(f.judgedEntity(t, "run-other", "q1"), "q1", types.EntityRoleCentral)); err != nil {
		t.Fatalf("relate another story: %v", err)
	}
	if err := f.finishGate(t, "run-f2", "research:doc-story", agentprofile.Research); err == nil {
		t.Fatal("another story's record freed this story's final Report")
	}
	if err := f.relateOwn(t, "run-f2"); err != nil {
		t.Fatalf("relate own story: %v", err)
	}
	if err := f.finishGate(t, "run-f2", "research:doc-story", agentprofile.Research); err != nil {
		t.Fatalf("after the story's record: %v", err)
	}
	// The record is the story's, not the run's: a later run passes too.
	if err := f.finishGate(t, "run-f2-later", "research:doc-story", agentprofile.Research); err != nil {
		t.Fatalf("a later run on a recorded story: %v", err)
	}
}

func TestFinishRuleIsFreedOnlyByAPlatformSideFailureOfThisRun(t *testing.T) { // F3, F3b
	cases := []struct {
		name     string
		set      func(f *analysisFixture)
		platform bool
	}{
		{"5xx", func(f *analysisFixture) {
			f.epub.checkErr = &platformStatusError{Path: types.StoryEntitiesPath, Status: http.StatusServiceUnavailable, Body: "corpusd is restarting"}
		}, true},
		{"transport", func(f *analysisFixture) {
			f.epub.checkErr = errors.New("dial tcp 10.0.0.1:443: connect: connection refused")
		}, true},
		{"no platform", func(f *analysisFixture) { f.rt.SetStoryPublisher(f.pub) }, true},
		{"4xx refusal", func(f *analysisFixture) {
			f.epub.checkErr = &platformStatusError{Path: types.StoryEntitiesPath, Status: http.StatusUnprocessableEntity, Body: "entity 1 matches the existing entity wd:Q1"}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAnalysisFixture(t)
			f.turnOn(t)
			a := f.analyse(t, agentprofile.Research, "run-f3", "doc-story") // asked before the platform fails
			if a == nil || len(a.Candidates) == 0 {
				t.Fatalf("analysis = %+v", a)
			}
			tc.set(f)
			err := f.relateOwn(t, "run-f3")
			if err == nil {
				t.Fatal("Relate succeeded against a failing platform")
			}
			if n := len(f.entityEvents(t)); n != 0 {
				t.Fatalf("a failed Relate recorded %d attachments", n)
			}
			gate := f.finishGate(t, "run-f3", "research:doc-story", agentprofile.Research)
			if tc.platform {
				if gate != nil {
					t.Fatalf("platform-side failure (%v): the final Report is still held: %v", err, gate)
				}
				if !strings.Contains(err.Error(), "final Report") {
					t.Fatalf("err = %v, want it to say the final Report may go ahead", err)
				}
				if other := f.finishGate(t, "run-f3-other", "research:doc-story", agentprofile.Research); other == nil {
					t.Fatal("this run's platform failure freed another run's final Report")
				}
				return
			}
			if gate == nil {
				t.Fatalf("a 4xx refusal (%v) freed the final Report", err)
			}
		})
	}

	// A host-side refusal: a citation with no answer behind it.
	f := newAnalysisFixture(t)
	f.turnOn(t)
	payload, _ := json.Marshal(yaegikernel.RelatePayload{Entities: []yaegikernel.RelateEntity{
		{EntityID: "wd:Q3874799", Role: types.EntityRoleCentral, JudgmentRef: "system1:nothing", QuestionID: "role-1"},
	}})
	if _, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.deskExec("run-host", "doc-story"), yaegikernel.ActionRelate, payload); err == nil {
		t.Fatal("Relate with no basis succeeded")
	}
	if err := f.finishGate(t, "run-host", "research:doc-story", agentprofile.Research); err == nil {
		t.Fatal("a host-side refusal freed the final Report")
	}
}

// Through a real research cell: the analysis is in the frame, an interim
// packet goes through, a final Report is refused until the cell records the
// story's entities with choir.Relate("") citing the analysis, and then it
// goes through.
func TestResearchCellFinishRuleThroughACell(t *testing.T) { // F2, F4
	deskTestWorkerBin(t)
	f := newAnalysisFixture(t)
	f.turnOn(t)
	ctx := context.Background()
	doc := "doc-story"
	texture, research := "texture:"+doc, "research:"+doc
	cast := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "seed-cast-finish", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:finish", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "find sources", Body: `{"kind":"question","summary":"sources"}`},
		Cast:      &types.CommitmentCast{},
		Inbox:     research, SenderInbox: texture, Addressee: research,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: texture},
	}}}
	cast.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(cast)
	if _, err := f.s.CommitInboxRecords(ctx, cast); err != nil {
		t.Fatalf("seed Cast: %v", err)
	}
	rec := types.RunRecord{RunID: "run-finish-cell", AgentID: research, OwnerID: f.owner, ComputerID: f.computer,
		ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := f.s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: f.owner, ChannelID: doc, ComputerID: f.computer,
		Profile: agentprofile.Research, Role: agentprofile.Research, RunRecord: &rec}
	tool := newDeskGoEvalTool(f.rt, newDeskSessionWorkers(), agentprofile.Research)
	calls := 0
	cell := func(src string) string {
		t.Helper()
		// Each cell is its own tool call, as in a run: its records commit
		// under its own cell id.
		calls++
		execCtx.ToolCallID = fmt.Sprintf("call-%d", calls)
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		// F6: the cell is the product's call path, so it runs with the gates
		// enforcing; its reads of the story analysis must declare a reader the
		// custody table admits.
		defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
		// The run's execution context carries its desk as the reader and writer
		// (runtime.go); an in-process cell must do as the run does.
		cellCtx := withDeskReader(withDeskWriter(ctx, execCtx.Profile), execCtx.Profile)
		out, err := tool.Func(toolregistry.WithExecutionContext(cellCtx, execCtx), payload)
		if err != nil {
			return "ERR " + err.Error() + " " + out
		}
		return out
	}
	textureRecords := func() []types.CommitmentRecord {
		got, err := f.s.ListInboxRecords(ctx, f.owner, f.computer, texture, 0, 10)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	out := cell(`import "choir"
a := choir.StoryAnalysis()
fmt.Println("analysis", a.Story.DocID, len(a.Candidates), a.Error == "")
choir.ReportPacket("texture", map[string]interface{}{"kind":"evidence_update","summary":"first source","claims":[]interface{}{map[string]interface{}{"text":"a first source"}}}, "")`)
	if !strings.Contains(out, "analysis doc-story 3 true") {
		t.Fatalf("the cell did not read its story's analysis: %s", out)
	}
	if got := textureRecords(); len(got) != 1 || got[0].Final {
		t.Fatalf("interim packet: Texture inbox = %+v, want one interim record", got)
	}

	final := `choir.ReportPacket("texture", map[string]interface{}{"kind":"evidence_update","summary":"done","claims":[]interface{}{map[string]interface{}{"text":"done"}},"work_disposition":"completed"}, "")`
	// B4: no Relate in the desk's cells, and the final Report goes through:
	// the host records the story's entities from System 1's top answers.
	out = cell(`import "choir"
_ = choir.Records()
` + final)
	if strings.Contains(out, "choir.Relate") || strings.Contains(out, "ERR") {
		t.Fatalf("the final Report was refused although the host records the entities: %s", out)
	}
	if recs := f.entityEvents(t); len(recs) == 0 {
		t.Fatal("the final Report passed but no story.entities record is on the tape")
	}
	got := textureRecords()
	if len(got) != 2 || !got[1].Final {
		t.Fatalf("after the default record, Texture inbox = %+v, want the final Report", got)
	}
}

// The research wake prompt asks for the record when the rule applies, and
// only then (slice c "Prompt").
func TestResearchWakePromptAsksForTheStoryEntitiesWhenTheRuleApplies(t *testing.T) {
	f := newAnalysisFixture(t)
	ctx := context.Background()
	asks := func(p string) bool {
		return strings.Contains(p, "choir.StoryAnalysis()") && strings.Contains(p, `choir.Relate("", entities)`)
	}
	if p := f.rt.recordsInboxPrompt(ctx, f.owner, f.computer, "research:doc-story"); asks(p) || !strings.Contains(p, "choir.Records()") {
		t.Fatalf("setting off: prompt = %q, want the plain research prompt", p)
	}
	f.turnOn(t)
	if p := f.rt.recordsInboxPrompt(ctx, f.owner, f.computer, "research:doc-story"); !asks(p) {
		t.Fatalf("setting on, story desk: prompt = %q, want it to ask for the story's entities", p)
	}
	if p := f.rt.recordsInboxPrompt(ctx, f.owner, f.computer, "research:"+f.owner); asks(p) {
		t.Fatalf("unbound desk: prompt = %q, want the plain research prompt", p)
	}
	if p := f.rt.recordsInboxPrompt(ctx, f.owner, f.computer, persistentManagementAgentID(f.owner)); asks(p) || p == "" {
		t.Fatalf("management: prompt = %q", p)
	}
}

// F5 (review 324 F1): the rule holds a Report no legal call can free. When
// this run's analysis has no System 1 answers, choir.Relate has no basis:
// a story with no candidates is recorded with an empty list (the analysis
// was built and found none), and an analysis that failed (System 1 off, no
// decision model, the judge or the candidates read failing) frees this
// run's final Report as a platform failure does, recording nothing.
func TestFinishRuleIsSatisfiableWhenTheAnalysisHasNoAnswers(t *testing.T) {
	relateEmpty := func(f *analysisFixture, runID string) (attachDecoded, error) {
		payload, _ := json.Marshal(yaegikernel.RelatePayload{})
		raw, err := f.rt.deskEgress(context.Background(), agentprofile.Research, f.deskExec(runID, "doc-story"), yaegikernel.ActionRelate, payload)
		if err != nil {
			return attachDecoded{}, err
		}
		return decodeAttach(t, raw), nil
	}

	t.Run("no candidates", func(t *testing.T) {
		f := newAnalysisFixture(t)
		f.turnOn(t)
		f.cpub.resp = types.EntityCandidatesResponse{}
		a := f.analyse(t, agentprofile.Research, "run-none", "doc-story")
		if a == nil || len(a.Candidates) != 0 || a.Error != "" || f.judgeCalls() != 0 {
			t.Fatalf("analysis = %+v, judge calls %d", a, f.judgeCalls())
		}
		// B4: the host records the empty list itself at the final Report.
		if err := f.finishGate(t, "run-none", "research:doc-story", agentprofile.Research); err != nil {
			t.Fatalf("the Report was held although the host can record the empty list: %v", err)
		}
		if got, err := relateEmpty(f, "run-none"); err != nil || !got.Exported {
			t.Fatalf("a desk's own Relate(\"\", nil) after the default = %+v, %v", got, err)
		}
		recs := f.records(t)
		if len(recs) == 0 || len(recs[0].Attachments) != 0 || recs[0].Review == nil || recs[0].Review.Counts.Candidates != 0 {
			t.Fatalf("records = %+v, want a review with nothing attached", recs)
		}
		if exports := f.epub.entityCalls(); len(exports) == 0 || exports[0].Review == nil || len(exports[0].Entities) != 0 {
			t.Fatalf("exports = %+v", exports)
		}
		if err := f.finishGate(t, "run-none", "research:doc-story", agentprofile.Research); err != nil {
			t.Fatalf("after the empty record: %v", err)
		}
	})

	failures := []struct {
		name  string
		setup func(t *testing.T, f *analysisFixture)
	}{
		{"no decision model", func(t *testing.T, f *analysisFixture) { f.rt.candidateJudge = nil }},
		{"the judge fails", func(t *testing.T, f *analysisFixture) { f.judge.err = errors.New("the decision model is down") }},
		// Review 327 Low 3: answers that all carry an error are no answers.
		{"every answer errs", func(t *testing.T, f *analysisFixture) { f.judge.omitAll = true }},
		{"the candidates read fails", func(t *testing.T, f *analysisFixture) {
			f.cpub.err = &platformStatusError{Path: types.StoryEntityCandidatesPath, Status: http.StatusServiceUnavailable, Body: "corpusd is restarting"}
		}},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			f := newAnalysisFixture(t)
			f.turnOn(t)
			tc.setup(t, f)
			runID := "run-" + strings.ReplaceAll(tc.name, " ", "-")
			a := f.analyse(t, agentprofile.Research, runID, "doc-story")
			if a == nil || a.Error == "" {
				t.Fatalf("analysis = %+v, want its error", a)
			}
			// No basis for "about none": the analysis did not judge.
			if _, err := relateEmpty(f, runID); err == nil {
				t.Fatal("Relate(\"\", nil) recorded a story the analysis could not judge")
			}
			if err := f.finishGate(t, runID, "research:doc-story", agentprofile.Research); err != nil {
				t.Fatalf("the final Report is held though no call can free it: %v", err)
			}
			if recs := f.records(t); len(recs) != 0 {
				t.Fatalf("records = %+v, want none", recs)
			}
			// Another run's failed analysis frees nothing.
			if err := f.finishGate(t, "run-other", "research:doc-story", agentprofile.Research); err == nil {
				t.Fatal("another run's failed analysis freed this run's Report")
			}
		})
	}
}

// B4: the host applies the analysis's top answers when the desk recorded
// nothing. Ways it can fail, written first:
//
//	D1 The Report is refused although the analysis holds answers; the default
//	   attaches an entity System 1 ranked not_about, a phrase it judged not an
//	   entity, or something the analysis never listed; or it records twice.
//	D2 The default hides a refusal: the platform 4xx must still hold the
//	   Report, with the cause in the error.
//	D3 The desk's own Relate before the final Report is overridden by the
//	   default (the default must find the record and do nothing).
func TestFinishRuleRecordsTheAnalysisTopAnswersByDefault(t *testing.T) { // D1
	f := newAnalysisFixture(t)
	f.turnOn(t)
	a := f.analyse(t, agentprofile.Research, "run-d1", "doc-story")
	if a == nil || len(a.Candidates) == 0 {
		t.Fatalf("analysis = %+v", a)
	}
	want, answered := defaultStoryEntities(a)
	if !answered {
		t.Fatalf("the analysis has no answers: %+v", a)
	}
	for _, e := range want {
		if e.Role != types.EntityRoleCentral && e.Role != types.EntityRoleIncidental {
			t.Fatalf("the default attaches role %q: %+v", e.Role, e)
		}
	}
	if err := f.finishGate(t, "run-d1", "research:doc-story", agentprofile.Research); err != nil {
		t.Fatalf("the Report was held although the analysis holds answers: %v", err)
	}
	recs := f.records(t)
	if len(recs) != 1 || len(recs[0].Attachments) != len(want) {
		t.Fatalf("records = %+v, want one record with the %d default attachments", recs, len(want))
	}
	for i, att := range recs[0].Attachments {
		if att.Role != want[i].Role || att.JudgmentRef != want[i].JudgmentRef || att.QuestionID != want[i].QuestionID {
			t.Errorf("attachment %d = %+v, want the analysis's top answer %+v", i, att, want[i])
		}
	}
	// A second final Report finds the record and records nothing more.
	if err := f.finishGate(t, "run-d1", "research:doc-story", agentprofile.Research); err != nil {
		t.Fatalf("second check: %v", err)
	}
	if again := f.records(t); len(again) != 1 {
		t.Fatalf("the default recorded twice: %+v", again)
	}
}

func TestDefaultStoryEntitiesLeavesOutWhatNeedsTheDesksJudgment(t *testing.T) { // D1
	ans := func(top string) *yaegikernel.StoryAnalysisAnswer {
		return &yaegikernel.StoryAnalysisAnswer{ReceiptRef: "system1:j1", QuestionID: "q-" + top, Top: top}
	}
	cand := func(entityID, phrase string, role, kind *yaegikernel.StoryAnalysisAnswer) yaegikernel.StoryAnalysisCandidate {
		c := yaegikernel.StoryAnalysisCandidate{Role: role, Kind: kind}
		c.EntityID, c.Phrase = entityID, phrase
		return c
	}
	a := &yaegikernel.StoryAnalysis{Candidates: []yaegikernel.StoryAnalysisCandidate{
		cand("wd:Q1", "Known", ans("central"), nil),
		cand("wd:Q2", "Passing", ans("incidental"), nil),
		cand("wd:Q3", "NotAbout", ans("not_about"), nil),
		cand("", "Newco", ans("central"), ans("organization")),
		cand("", "Vague", ans("central"), ans("not an entity")),
		cand("", "NoKind", ans("central"), nil),
		cand("wd:Q1", "Known again", ans("central"), nil),
		{}, // past the budget: no answers
	}}
	got, answered := defaultStoryEntities(a)
	if !answered {
		t.Fatal("answers present but not reported")
	}
	var ids []string
	for _, e := range got {
		ids = append(ids, e.ResolvedID()+":"+e.Role)
	}
	want := []string{"wd:Q1:central", "wd:Q2:incidental", types.MintedEntityID("Newco") + ":central"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("default = %v, want %v", ids, want)
	}
	if _, answered := defaultStoryEntities(&yaegikernel.StoryAnalysis{Candidates: []yaegikernel.StoryAnalysisCandidate{{}}, Error: "System 1 off"}); answered {
		t.Fatal("an analysis with no answers must not default")
	}
	if _, answered := defaultStoryEntities(nil); answered {
		t.Fatal("no analysis must not default")
	}
}

func TestFinishRuleDefaultKeepsAPlatformRefusalAndTheDesksOwnRecord(t *testing.T) { // D2, D3
	f := newAnalysisFixture(t)
	f.turnOn(t)
	f.analyse(t, agentprofile.Research, "run-d2", "doc-story")
	f.epub.checkErr = &platformStatusError{Path: types.StoryEntitiesPath, Status: http.StatusUnprocessableEntity, Body: "entity 1 matches the existing entity wd:Q1"}
	err := f.finishGate(t, "run-d2", "research:doc-story", agentprofile.Research)
	if err == nil || !strings.Contains(err.Error(), "choir.Relate") || !strings.Contains(err.Error(), "entity 1 matches") {
		t.Fatalf("a platform refusal must hold the Report and say why: %v", err)
	}
	if n := len(f.entityEvents(t)); n != 0 {
		t.Fatalf("a refused default recorded %d attachments", n)
	}
	f.epub.checkErr = nil
	if err := f.relateOwn(t, "run-d2"); err != nil {
		t.Fatalf("the desk's own Relate: %v", err)
	}
	before := len(f.entityEvents(t))
	if err := f.finishGate(t, "run-d2", "research:doc-story", agentprofile.Research); err != nil {
		t.Fatalf("after the desk's own record: %v", err)
	}
	if after := len(f.entityEvents(t)); after != before {
		t.Fatalf("the default recorded over the desk's own record: %d to %d", before, after)
	}
}
