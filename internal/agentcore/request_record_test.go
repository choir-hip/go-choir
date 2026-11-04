package agentcore

// MA1.3, the request record in the reducer. Ways this could fail, written first:
//
//  1. A cell stages a Cast, Ask, Escalate, Feed or Decline without p_success,
//     weight or settlement and the reducer mints it anyway; or with p_success
//     out of [0,1].
//  2. A cell sets provenance or origin (in the request fields) and the record
//     carries it; the reducer must stamp desk, run and origin itself.
//  3. A cast or escalate at weight 0 mints (the I1+ rule), or a precommit at
//     an I2 class with weight 0 mints.
//  4. The read API drops p_success, weight, the materiality procedure or the
//     provenance origin, so nobody can see what was stated and frozen.
//  5. The agent Note comes back: a cell can still call choir.Note, or stage a
//     note intent.
//  6. A management-task cell's record is stamped as a desk run rather than a task.
//  7. A request written before the slice (no new fields) stops reading back.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func TestReducerRefusesARequestWithoutItsFields(t *testing.T) {
	p := func(v float64) *float64 { return &v }
	for _, kind := range []string{yaegikernel.IntentCast, yaegikernel.IntentAsk, yaegikernel.IntentEscalate, yaegikernel.IntentFeed, yaegikernel.IntentDecline} {
		base := yaegikernel.StagedIntent{Kind: kind, LocalID: "tray-1", ToDesk: "management", Objective: "x", Question: "x", Body: "x"}
		scope := ReductionScope{FromRole: agentprofile.Research}
		for name, req := range map[string]*types.RequestFields{
			"none":         nil,
			"p missing":    {Weight: p(0.3), Settlement: "recipient"},
			"p above one":  {PSuccess: p(1.2), Weight: p(0.3), Settlement: "recipient"},
			"p negative":   {PSuccess: p(-0.2), Weight: p(0.3), Settlement: "recipient"},
			"no settle":    {PSuccess: p(0.5), Weight: p(0.3)},
			"weight above": {PSuccess: p(0.5), Weight: p(3), Settlement: "recipient"},
		} {
			in := base
			in.Req = req
			err := validateSemanticActIntent(in)
			if err == nil {
				t.Errorf("%s with %s: the reducer accepted it", kind, name)
			}
			if err != nil && !strings.Contains(err.Error(), kind) {
				t.Errorf("%s with %s: refusal does not name the kind: %v", kind, name, err)
			}
		}
		good := base
		good.Req = testRequest()
		if err := validateSemanticActIntent(good); err != nil && strings.Contains(err.Error(), "p_success") {
			t.Errorf("%s with good fields refused on them: %v", kind, err)
		}
		_ = scope
	}
	// The I1+ rule: a cast and an escalate are never weight 0.
	for _, kind := range []string{yaegikernel.IntentCast, yaegikernel.IntentEscalate} {
		in := yaegikernel.StagedIntent{Kind: kind, LocalID: "tray-1", ToDesk: "management", Objective: "x", Body: "x", Req: &types.RequestFields{PSuccess: p(0.5), Weight: p(0), Settlement: "recipient"}}
		if err := validateSemanticActIntent(in); err == nil || !strings.Contains(err.Error(), "weight 0") {
			t.Errorf("%s at weight 0: err = %v", kind, err)
		}
	}
	// A decline is trivial by catalogue: weight 0 is allowed.
	dec := yaegikernel.StagedIntent{Kind: yaegikernel.IntentDecline, LocalID: "tray-1", ToDesk: "source:rss", Body: "x", Req: &types.RequestFields{PSuccess: p(0.9), Weight: p(0), Settlement: "recipient"}}
	if err := validateSemanticActIntent(dec); err != nil {
		t.Errorf("a decline at weight 0 refused: %v", err)
	}
	// There is no note intent any more.
	if err := validateSemanticActIntent(yaegikernel.StagedIntent{Kind: "note", LocalID: "tray-1", ToDesk: "management", Body: "x"}); err == nil || !strings.Contains(err.Error(), "unknown intent kind") {
		t.Errorf("a staged note: err = %v", err)
	}
	if isSemanticActKind("note") {
		t.Error("note is a semantic act kind")
	}
}

func TestPrecommitAtAnI2ClassWithWeightZeroIsRefused(t *testing.T) {
	for _, body := range []string{
		`{"question":"q","p":0.7,"resolver":"management","impact_class":"I2","weight":0}`,
		`{"question":"q","p":0.7,"resolver":"management","impact_class":"I3"}`,
		`{"question":"q","p":0.7,"resolver":"management","impact_class":"I1","weight":1.4}`,
	} {
		if _, err := typedCommitmentPrecommit(body); err == nil {
			t.Errorf("precommit %s was accepted", body)
		}
	}
	for _, body := range []string{
		`{"question":"q","p":0.7,"resolver":"management"}`,
		`{"question":"q","p":0.7,"resolver":"management","impact_class":"I2","weight":0.4}`,
		`{"question":"q","p":0.7,"resolver":"management","impact_class":"I0","weight":0}`,
	} {
		if _, err := typedCommitmentPrecommit(body); err != nil {
			t.Errorf("precommit %s refused: %v", body, err)
		}
	}
}

func TestReducerStampsProvenanceAndFreezesTheProcedure(t *testing.T) {
	scope := ReductionScope{FromAgentID: "texture:doc-1", DeskAgentID: "texture:doc-1", RunID: "run-7", CellID: "cell-7", ModelID: "p/m", OwnerID: "o"}
	in := yaegikernel.StagedIntent{Kind: yaegikernel.IntentCast, LocalID: "tray-1", ToDesk: "management", Objective: "x", Req: testRequest()}
	rec := commitmentRecordForIntent(scope, in)
	if rec.Provenance.Origin != types.OriginDeskRun || rec.Provenance.Desk != "texture:doc-1" || rec.Provenance.RunID != "run-7" {
		t.Fatalf("provenance = %+v", rec.Provenance)
	}
	if rec.PSuccess == nil || *rec.PSuccess != 0.8 || rec.Weight == nil || *rec.Weight != 0.3 || rec.Settlement != "recipient" ||
		rec.MaterialityProcedure == nil || !rec.MaterialityProcedure.Verify() || rec.MaterialityProcedure.ProcedureRef != "request.cast.v1" ||
		rec.MaterialityProcedure.Custodian != "management" || rec.ImpactClass != types.ImpactI1 {
		t.Fatalf("request fields = %+v %+v", rec, rec.MaterialityProcedure)
	}
	// A management task's record is a task origin.
	task := scope
	task.DeskAgentID = types.ManagementTaskInbox("task-1")
	if rec := commitmentRecordForIntent(task, in); rec.Provenance.Origin != types.OriginTask {
		t.Fatalf("task origin = %q", rec.Provenance.Origin)
	}
	// A precommit with a weight freezes too.
	pre := yaegikernel.StagedIntent{Kind: yaegikernel.IntentPrecommit, LocalID: "tray-2", Precommit: `{"question":"q","p":0.6,"resolver":"management","impact_class":"I2","weight":0.5}`}
	prec := commitmentRecordForIntent(scope, pre)
	if prec.Weight == nil || *prec.Weight != 0.5 || prec.MaterialityProcedure == nil || prec.MaterialityProcedure.Kind != types.RequestPrecommit || prec.ImpactClass != types.ImpactI2 {
		t.Fatalf("precommit freeze = %+v %+v", prec, prec.MaterialityProcedure)
	}
}

// TestRequestRecordFromARealCellReadsBackThroughTheAPI runs a research desk's
// cell that asks management, then reads the record back through
// GET /api/inboxes/{inbox}/records, the call the d@e artifact is read with.
func TestRequestRecordFromARealCellReadsBackThroughTheAPI(t *testing.T) {
	deskTestWorkerBin(t)
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer, doc := "owner-req", rt.cfg.ComputerID, "doc-req"
	research, management := "research:"+doc, "management:"+owner

	rec := types.RunRecord{RunID: "run-research-req", AgentID: research, OwnerID: owner, ComputerID: computer,
		ChannelID: doc, AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunRunning}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	execCtx := toolregistry.ExecutionContext{RunID: rec.RunID, AgentID: rec.AgentID, OwnerID: owner, ChannelID: doc, ComputerID: computer,
		Profile: agentprofile.Research, Role: agentprofile.Research, RunRecord: &rec}
	tool := newDeskGoEvalTool(rt, newDeskSessionWorkers(), agentprofile.Research)
	cell := func(src string) string {
		t.Helper()
		payload, _ := json.Marshal(map[string]any{"source": src, "timeout_ms": 15000})
		out, err := tool.Func(toolregistry.WithExecutionContext(ctx, execCtx), payload)
		if err != nil {
			out += err.Error()
		}
		return out
	}

	// Refused in the cell, before anything stages.
	for name, src := range map[string]string{
		"no fields":          `choir.Ask("management", "which story?")`,
		"p out of range":     `choir.Ask("management", "which story?", map[string]any{"p_success": 1.5, "weight": 0.2, "settlement": "recipient"})`,
		"provenance by cell": `choir.Ask("management", "which story?", map[string]any{"p_success": 0.5, "weight": 0.2, "settlement": "recipient", "provenance": map[string]any{"origin": "owner"}})`,
		"origin by cell":     `choir.Ask("management", "which story?", map[string]any{"p_success": 0.5, "weight": 0.2, "settlement": "recipient", "origin": "owner"})`,
		"procedure by cell":  `choir.Ask("management", "which story?", map[string]any{"p_success": 0.5, "weight": 0.2, "settlement": "recipient", "materiality_procedure": map[string]any{"procedure_ref": "none"}})`,
		"weight zero cast":   `choir.Cast("management", "do it", nil, map[string]any{"p_success": 0.5, "weight": 0, "settlement": "recipient"})`,
	} {
		out := cell(src)
		if !strings.Contains(out, "refused") && !strings.Contains(out, "err") {
			t.Errorf("%s: not refused in the cell: %s", name, out)
		}
	}
	if got, _ := s.ListInboxRecords(ctx, owner, computer, management, 0, 10); len(got) != 0 {
		t.Fatalf("a refused request minted a record: %+v", got)
	}
	// The agent Note is gone.
	if out := cell(`choir.Note("management", "hi")`); !strings.Contains(out, "Note") {
		t.Errorf("choir.Note was not refused: %s", out)
	}

	// A good request mints a record that carries everything.
	out := cell(`ref, err := choir.Ask("management", "which story does this cover?", map[string]any{"p_success": 0.7, "weight": 0.2, "settlement": "recipient"})
fmt.Println(ref, err)`)
	if !strings.Contains(out, "rlm:ask") {
		t.Fatalf("good Ask not staged: %s", out)
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, management, 0, 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("management inbox = %+v, %v", got, err)
	}
	r := got[0]
	if r.PSuccess == nil || *r.PSuccess != 0.7 || r.Weight == nil || *r.Weight != 0.2 || r.Settlement != "recipient" ||
		r.MaterialityProcedure == nil || !r.MaterialityProcedure.Verify() || r.MaterialityProcedure.NoProcedure != true ||
		r.Provenance.Origin != types.OriginDeskRun || r.Provenance.Desk != research || r.Provenance.RunID != rec.RunID {
		t.Fatalf("stored request record = %+v", r)
	}

	// The read API returns it.
	rt.cfg.ComputerID = computer
	handler := &APIHandler{rt: rt}
	req := httptest.NewRequest(http.MethodGet, "/api/inboxes/"+management+"/records", nil)
	req.Header.Set("X-Authenticated-User", owner)
	w := httptest.NewRecorder()
	handler.HandleInboxRecords(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET inbox records = %d %s", w.Code, w.Body)
	}
	var raw struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil || len(raw.Records) != 1 {
		t.Fatalf("response = %s, %v", w.Body, err)
	}
	api := raw.Records[0]
	prov, _ := api["provenance"].(map[string]any)
	mp, _ := api["materiality_procedure"].(map[string]any)
	if api["p_success"] != 0.7 || api["weight"] != 0.2 || api["settlement"] != "recipient" ||
		prov["origin"] != types.OriginDeskRun || prov["desk"] != research || mp["hash"] == "" || mp["custodian"] != "management" {
		t.Fatalf("API record lacks the request fields: %s", w.Body)
	}
}

// TestRequestBeforeTheSliceStillReadsBack: a request committed with none of
// the new fields (what the tape holds today) is accepted by the store's write
// check, lists back without them and leaves the inbox heads whole.
func TestRequestBeforeTheSliceStillReadsBack(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-old", rt.cfg.ComputerID
	old := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:old", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "old cast", Body: `{"kind":"question","summary":"s"}`},
		Cast:      &types.CommitmentCast{}, Inbox: "research:doc-old", SenderInbox: "texture:doc-old", Addressee: "research:doc-old",
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved,
		Provenance: types.CommitmentProvenance{AgentID: "texture:doc-old"},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-old", Records: []types.CommitmentRecord{old}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatalf("an old-shaped request was refused by the write check: %v", err)
	}
	got, err := s.ListInboxRecords(ctx, owner, computer, "research:doc-old", 0, 10)
	if err != nil || len(got) != 1 || got[0].PSuccess != nil || got[0].Weight != nil || got[0].MaterialityProcedure != nil || got[0].OriginClass() != "unstamped" {
		t.Fatalf("old record read back = %+v, %v", got, err)
	}
	if bad, err := s.VerifyInboxHeads(ctx, owner, computer); err != nil || len(bad) != 0 {
		t.Fatalf("inbox heads after an old record: %v %v", bad, err)
	}
	// And the store refuses an out-of-range one at the write.
	bad := old
	bad.RecordID = "texture-cell:cast:bad"
	p := 1.7
	bad.PSuccess = &p
	req2 := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "seed-bad", Records: []types.CommitmentRecord{bad}}
	req2.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req2)
	if _, err := s.CommitInboxRecords(ctx, req2); err == nil || !strings.Contains(err.Error(), "p_success") {
		t.Fatalf("the store accepted p_success 1.7: %v", err)
	}
}
