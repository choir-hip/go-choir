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
		{"transport", func(f *analysisFixture) { f.epub.checkErr = errors.New("dial tcp 10.0.0.1:443: connect: connection refused") }, true},
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
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
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
	out = cell(`import "choir"
` + final)
	if !strings.Contains(out, "choir.Relate") {
		t.Fatalf("a final Report with no record was not refused naming choir.Relate: %s", out)
	}
	if got := textureRecords(); len(got) != 1 {
		t.Fatalf("the refused final Report reached Texture: %+v", got)
	}

	out = cell(`import "choir"
func() {
	c := choir.StoryAnalysis().Candidates[0]
	r, err := choir.Relate("", []map[string]any{{"entity_id": c.EntityID, "role": "central", "judgment_ref": c.Role.ReceiptRef, "question_id": c.Role.QuestionID}})
	fmt.Println("relate", err == nil, string(r))
}()
` + final)
	if !strings.Contains(out, "relate true ") || !strings.Contains(out, types.StoryPublicationID(f.owner, doc)) {
		t.Fatalf("the cell's Relate(\"\") did not record its own story: %s", out)
	}
	got := textureRecords()
	if len(got) != 2 || !got[1].Final {
		t.Fatalf("after Relate, Texture inbox = %+v, want the final Report", got)
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
