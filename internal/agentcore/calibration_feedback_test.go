package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The calibration digest (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md
// §5). Failure modes these tests pin: P1 the digest enters the system
// prompt; P2 it repeats when unchanged; P3 the control arm or a computer
// with the setting off sees it; P4 a pending item becomes an observation;
// P6 a management task's claim is attributed to the wrong thread; P8 a bad
// self-score is stored; P10 a score record reads as a verdict in a pack;
// P11 the settlement route accepts what it should refuse.

const calOwner = "owner-cal"

func castRecord(id, sender, task, conv string, hyps ...types.CastHypothesis) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "open"},
		Cast:        &types.CommitmentCast{Hypotheses: hyps},
		Provenance:  types.CommitmentProvenance{AgentID: sender, ModelID: "opencode-go/deepseek-v4.1-flash", ActingTask: task, CommittedAt: "2026-10-15T10:00:00Z"},
		EmitterKind: types.CommitmentEmitterAgent, SenderInbox: sender, Inbox: "texture:doc-1", ConversationID: conv,
	}
}

func scoreRecord(scorer, target string, answers map[string]string, coverage *float64) types.CommitmentRecord {
	sc := types.CommitmentScore{ScorerModelID: "google-antigravity/gemini-3.8-flash", ScorerAgentID: "scorer:" + scorer, Answers: answers}
	if coverage != nil {
		sc.Probabilities = map[string]float64{types.CalibrationScoreCoverage: *coverage}
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.CalibrationScoreRecordPrefix + scorer + ":" + target,
		Kind: types.CommitmentKindResolve, Discrepancy: types.DiscrepancyUnresolved, Resolve: &types.CommitmentResolve{Verdict: "calibration_score"},
		Provenance: types.CommitmentProvenance{AgentID: "scorer:" + scorer}, EmitterKind: types.CommitmentEmitterOwner,
		RelatedIDs: []string{target}, Scores: []types.CommitmentScore{sc},
	}
}

func finalResearchReport(id, doc string, selfP *float64) types.CommitmentRecord {
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindReport, Final: true,
		Report:      &types.CommitmentReport{StopReason: "completed", Claims: []types.ReportClaim{{Text: "c"}}},
		Provenance:  types.CommitmentProvenance{AgentID: "research:" + doc, ModelID: "chatgpt/gpt-6-luna", CommittedAt: "2026-10-15T11:00:00Z"},
		EmitterKind: types.CommitmentEmitterAgent, SenderInbox: "research:" + doc, Inbox: "texture:" + doc,
	}
	if selfP != nil {
		rec.Scores = []types.CommitmentScore{{ScorerAgentID: rec.SenderInbox, Self: true, Probabilities: map[string]float64{types.SelfScoreComplete: *selfP}}}
	}
	return rec
}

func TestCollectCalibrationTakesOnlySettledSelfScores(t *testing.T) {
	mgmt := castRecord("cell-m:cast:1", "management:o", "task-1", "conv:a",
		types.CastHypothesis{ID: "H1", Statement: "a", Probability: 0.6}, types.CastHypothesis{ID: "H2", Statement: "b", Probability: 0.3},
		types.CastHypothesis{ID: "H3", Statement: "c", Probability: 0.8}, types.CastHypothesis{ID: "H4", Statement: "d", Probability: 0.5})
	withdraw := castRecord("cell-m:cast:2", "management:o", "task-1", "conv:a")
	withdraw.Cast = &types.CommitmentCast{Withdrawals: []types.CastWithdrawal{{ID: "H4", Reason: "moot"}}}
	texture := castRecord("cell-t:cast:1", "texture:doc-1", "", "conv:b", types.CastHypothesis{ID: "H1", Statement: "e", Probability: 0.7})
	protocol := types.CommitmentRecord{RecordID: "cell-r:report:1", Kind: types.CommitmentKindReport, SenderInbox: "research:doc-1", ConversationID: "conv:b",
		Report: &types.CommitmentReport{Settled: []types.HypothesisSettlement{{ID: "H1", Outcome: "confirmed"}}}, EmitterKind: types.CommitmentEmitterAgent}
	system := castRecord("closure:1", "management:o", "", "conv:c", types.CastHypothesis{ID: "H1", Statement: "s", Probability: 0.9})
	system.EmitterKind = types.CommitmentEmitterSystem
	settledReport := finalResearchReport("cell-r:report:2", "doc-1", ptr(0.9))
	noSelf := finalResearchReport("cell-r:report:3", "doc-2", nil)
	unsettled := finalResearchReport("cell-r:report:4", "doc-3", ptr(0.7))
	records := []types.CommitmentRecord{mgmt, withdraw, texture, protocol, system, settledReport, noSelf, unsettled,
		scoreRecord("e6a", mgmt.RecordID, map[string]string{"H1": "confirmed", "H2": "contradicted"}, nil),
		scoreRecord("luna", mgmt.RecordID, map[string]string{"H1": "qualified", "H3": "unresolved"}, nil),
		scoreRecord("e6a", settledReport.RecordID, nil, ptr(0.5)),
	}
	pair := func(key string, selfP *float64) store.SupportMemo[types.CitationCheck] {
		return store.SupportMemo[types.CitationCheck]{Value: types.CitationCheck{Pair: types.CitationPair{Key: key, DocID: "doc-1", RevisionID: "rev-1"}, SelfP: selfP, ModelID: "chatgpt/gpt-6-luna"}, At: time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)}
	}
	checks := []store.SupportMemo[types.CitationCheck]{pair("sp-1", ptr(0.9)), pair("sp-2", ptr(0.9)), pair("sp-3", ptr(0.9)), pair("sp-4", nil)}
	verdicts := []store.SupportMemo[types.SupportVerdict]{
		{Value: types.SupportVerdict{Key: "sp-1", Verdict: types.SupportSupported, Judge: "jev:m"}},
		{Value: types.SupportVerdict{Key: "sp-2", Verdict: types.SupportUnsupported, Judge: "jev:m"}},
		{Value: types.SupportVerdict{Key: "sp-4", Verdict: types.SupportSupported, Judge: "jev:m"}},
	}
	obs, pending := collectCalibration(records, checks, verdicts)

	byKey := map[string]calibration.Observation{}
	for _, o := range obs {
		byKey[o.Key] = o
	}
	want := map[string]struct {
		desk, claim, thread string
		p, y                float64
	}{
		"cell-m:cast:1#H1": {"management", "hypothesis", "management-task:task-1", 0.6, 0.75}, // two scorers averaged
		"cell-m:cast:1#H2": {"management", "hypothesis", "management-task:task-1", 0.3, 0},
		"cell-t:cast:1#H1": {"texture", "hypothesis", "texture:doc-1", 0.7, 1}, // settled in protocol
		"cell-r:report:2":  {"research", "completed", "research:doc-1", 0.9, 0.5},
		"sp-1":             {"texture", "citation_support", "texture:doc-1", 0.9, 1},
		"sp-2":             {"texture", "citation_support", "texture:doc-1", 0.9, 0},
	}
	if len(obs) != len(want) {
		t.Fatalf("observations = %d, want %d: %+v", len(obs), len(want), obs)
	}
	for key, w := range want {
		o, ok := byKey[key]
		if !ok || o.Desk != w.desk || o.ClaimType != w.claim || o.Thread != w.thread || o.P != w.p || o.Y != w.y {
			t.Errorf("%s = %+v, want %+v", key, o, w)
		}
		if o.HeldOut != calibration.HeldOut(key) {
			t.Errorf("%s held-out flag %v", key, o.HeldOut)
		}
	}
	if o := byKey["cell-m:cast:1#H1"]; o.ModelID != "opencode-go/deepseek-v4.1-flash" || len(o.Scorers) != 2 || o.Story != "texture:doc-1" {
		t.Errorf("H1 model/scorers/story = %q %v %q", o.ModelID, o.Scorers, o.Story)
	}
	if o := byKey["sp-1"]; o.Story != "texture:doc-1" {
		t.Errorf("citation story = %q", o.Story)
	}
	pend := map[string]CalibrationPending{}
	for _, p := range pending {
		pend[p.RecordID+"#"+p.HypothesisID] = p
	}
	// H3 was only "unresolved": still pending. H4 was withdrawn: not
	// pending. The system record is nobody's claim. Unsettled completed
	// Reports are pending, with or without a self-score.
	if _, ok := pend["cell-m:cast:1#H3"]; !ok {
		t.Errorf("H3 not pending: %+v", pending)
	}
	if _, ok := pend["cell-m:cast:1#H4"]; ok {
		t.Errorf("withdrawn H4 is pending")
	}
	if _, ok := pend["closure:1#H1"]; ok {
		t.Errorf("a system record is pending")
	}
	if p, ok := pend["cell-r:report:3#"]; !ok || p.P != nil || p.ClaimType != types.CalibrationClaimCompleted {
		t.Errorf("report without self-score pending = %+v %v", p, ok)
	}
	if p, ok := pend["cell-r:report:4#"]; !ok || p.P == nil || *p.P != 0.7 {
		t.Errorf("unsettled report pending = %+v %v", p, ok)
	}
	if len(pending) != 3 {
		t.Errorf("pending = %d, want 3: %+v", len(pending), pending)
	}
}

// P8 and replay: a final research Report's p_complete becomes its self
// score, deterministically; an interim Report, or a bad value, none.
func TestResearchFinalReportCarriesItsSelfScore(t *testing.T) {
	scope := ReductionScope{FromAgentID: "research:doc-1", DeskAgentID: "research:doc-1", CellID: "cell-r", OwnerID: "o", ComputerID: "c", ModelID: "chatgpt/gpt-6-luna"}
	packet := func(p string) yaegikernel.StagedIntent {
		return yaegikernel.StagedIntent{Kind: yaegikernel.IntentReport, LocalID: "t1", ToDesk: "texture",
			Packet: `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"s","claims":[{"text":"c"}],"work_disposition":"completed"` + p + `}`}
	}
	rec, err := inboxRecordForIntent(scope, packet(`,"p_complete":0.7`), "texture:doc-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Final || len(rec.Scores) != 1 || !rec.Scores[0].Self || rec.Scores[0].Probabilities[types.SelfScoreComplete] != 0.7 || rec.Scores[0].ScorerAgentID != "research:doc-1" {
		t.Fatalf("final report scores = %+v", rec.Scores)
	}
	again, _ := inboxRecordForIntent(scope, packet(`,"p_complete":0.7`), "texture:doc-1", true)
	a, _ := json.Marshal(rec)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		// CommittedAt differs between derivations; compare the scores only.
		if fmt.Sprint(rec.Scores) != fmt.Sprint(again.Scores) {
			t.Fatalf("self score is not deterministic: %v vs %v", rec.Scores, again.Scores)
		}
	}
	if rec, _ := inboxRecordForIntent(scope, packet(`,"p_complete":0.7`), "texture:doc-1", false); len(rec.Scores) != 0 {
		t.Fatalf("an interim report carries a self score: %+v", rec.Scores)
	}
	if rec, _ := inboxRecordForIntent(scope, packet(`,"p_complete":1.5`), "texture:doc-1", true); len(rec.Scores) != 0 {
		t.Fatalf("p_complete 1.5 stored: %+v", rec.Scores)
	}
}

// P10: a score record targets a research Report; the Report's acting pack
// item stays unresolved with no observation, so research never sees a
// per-item verdict.
func TestCalibrationScoreRecordNeverReadsAsAVerdictInThePack(t *testing.T) {
	report := finalResearchReport("cell-r:report:9", "doc-9", ptr(0.8))
	report.Addressee = "texture:doc-9"
	pack := types.BuildActingPack([]types.CommitmentRecord{report, scoreRecord("e6a", report.RecordID, nil, ptr(0.2))}, "research:doc-9", 16)
	if len(pack.Items) != 1 {
		t.Fatalf("pack items = %+v", pack.Items)
	}
	item := pack.Items[0]
	if item.Discrepancy != types.DiscrepancyUnresolved || item.ObservationExcerpt != "" || item.ResolvedAt != "" {
		t.Fatalf("pack item leaks the score: %+v", item)
	}
}

func digestObservations(n int) []calibration.Observation {
	out := make([]calibration.Observation, n)
	for i := range out {
		out[i] = calibration.Observation{Desk: "management", ClaimType: types.CalibrationClaimHypothesis, Thread: "management-task:x",
			Key: fmt.Sprintf("k%d", i), P: 0.55, Y: 0.8, CommittedAt: "2026-10-15T10:00:00Z"}
	}
	return out
}

// management-task:t-1 is in the digest arm and management:owner-1 in
// control (calibration.TestArmAndHeldOutAreTheScriptsRules).
func calibrationRuns(rt *Runtime) (digestRun, controlRun *types.RunRecord) {
	computer := rt.cfg.ComputerID
	digestRun = &types.RunRecord{RunID: "run-d", OwnerID: calOwner, ComputerID: computer, AgentProfile: agentprofile.Management,
		AgentRole: agentprofile.ManagementTask, AgentID: "management-task:t-1", Metadata: map[string]any{"llm_provider": "opencode-go", "llm_model": "deepseek-v4.1-flash"}}
	controlRun = &types.RunRecord{RunID: "run-c", OwnerID: calOwner, ComputerID: computer, AgentProfile: agentprofile.Management,
		AgentID: "management:owner-1"}
	return digestRun, controlRun
}

func seedCalibrationSnapshot(rt *Runtime, obs []calibration.Observation) {
	rt.calibration.mu.Lock()
	defer rt.calibration.mu.Unlock()
	rt.calibration.byComputer = map[string]*calibrationSnapshot{calOwner + "\x00" + rt.cfg.ComputerID: {At: time.Now(), Observations: obs}}
}

// P3 and the setting: off by default, an owner act, and only the digest
// arm reads it, in the turn and in choir.Pack().
func TestCalibrationDigestFollowsTheSettingAndTheArm(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	if def, known := store.KnownComputerSettings[store.SettingCalibrationFeedback]; !known || def {
		t.Fatalf("calibration.feedback known=%v default=%v, want a known setting, off", known, def)
	}
	seedCalibrationSnapshot(rt, digestObservations(30))
	digestRun, controlRun := calibrationRuns(rt)
	if d := rt.calibrationDigestForRun(ctx, digestRun); d != nil {
		t.Fatalf("setting off, digest = %+v", d)
	}
	if inject := rt.calibrationDigestInjector(digestRun, nil); inject != nil {
		if turns, _ := inject(false); len(turns) != 0 {
			t.Fatalf("setting off, injected %d turns", len(turns))
		}
	}
	handler := &APIHandler{rt: rt}
	req := httptest.NewRequest(http.MethodPost, "/api/computer-settings", strings.NewReader(`{"key":"calibration.feedback","value":true}`))
	req.Header.Set("X-Authenticated-User", calOwner)
	rec := httptest.NewRecorder()
	handler.HandleComputerSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set calibration.feedback = %d %s", rec.Code, rec.Body)
	}
	if got, err := s.GetComputerSetting(ctx, calOwner, rt.cfg.ComputerID, store.SettingCalibrationFeedback); err != nil || !got.Value {
		t.Fatalf("stored setting = %+v %v", got, err)
	}

	d := rt.calibrationDigestForRun(ctx, digestRun)
	if d == nil || d.N != 30 || d.Direction != "under-confident" || !strings.Contains(d.Text, "Under-confident") {
		t.Fatalf("digest arm digest = %+v", d)
	}
	if d := rt.calibrationDigestForRun(ctx, controlRun); d != nil {
		t.Fatalf("control arm read a digest: %+v", d)
	}
	if inject := rt.calibrationDigestInjector(controlRun, nil); inject != nil {
		t.Fatal("control arm has an injector")
	}
	execCtx := func(run *types.RunRecord) toolregistry.ExecutionContext {
		return toolregistry.ExecutionContext{AgentID: run.AgentID, OwnerID: calOwner, ComputerID: rt.cfg.ComputerID, RunRecord: run}
	}
	if pack := actingCommitmentPackForDesk(ctx, rt, execCtx(digestRun)); pack == nil || pack.Calibration == nil || pack.Calibration.Text != d.Text {
		t.Fatalf("digest arm pack calibration = %+v", pack)
	}
	if pack := actingCommitmentPackForDesk(ctx, rt, execCtx(controlRun)); pack == nil || pack.Calibration != nil {
		t.Fatalf("control arm pack calibration = %+v", pack)
	}

	// Under 20 settled observations there is nothing to read (P5).
	seedCalibrationSnapshot(rt, digestObservations(19))
	if d := rt.calibrationDigestForRun(ctx, digestRun); d != nil {
		t.Fatalf("19 observations gave a digest: %+v", d)
	}
}

// P1 and P2: the digest is a user turn after the history, appended once
// per change; the system prompt is byte-identical with it on or off.
func TestCalibrationDigestIsALateUserTurnAndTheSystemPromptIsUnchanged(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	digestRun, controlRun := calibrationRuns(rt)
	texture := &types.RunRecord{RunID: "run-t", OwnerID: calOwner, ComputerID: rt.cfg.ComputerID, AgentProfile: agentprofile.Texture, AgentID: "texture:doc-1"}
	research := &types.RunRecord{RunID: "run-r", OwnerID: calOwner, ComputerID: rt.cfg.ComputerID, AgentProfile: agentprofile.Research, AgentID: "research:doc-1"}
	clock := regexp.MustCompile(`Current UTC date/time: [^.]*\.`)
	prompts := func() []string {
		var out []string
		for _, run := range []*types.RunRecord{digestRun, controlRun, texture, research} {
			p, err := rt.systemPromptForRun(run)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, clock.ReplaceAllString(p, "Current UTC date/time: T."))
		}
		return out
	}
	off := prompts()
	seedCalibrationSnapshot(rt, digestObservations(30))
	if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: calOwner, ComputerID: rt.cfg.ComputerID, Key: store.SettingCalibrationFeedback, Value: true}); err != nil {
		t.Fatal(err)
	}
	on := prompts()
	for i := range off {
		if off[i] != on[i] {
			t.Fatalf("system prompt %d changed with calibration.feedback on", i)
		}
		if strings.Contains(on[i], calibrationDigestHeader) || strings.Contains(on[i], "Your track record") {
			t.Fatalf("system prompt %d carries the digest", i)
		}
	}

	inject := rt.calibrationDigestInjector(digestRun, nil)
	if inject == nil {
		t.Fatal("digest arm has no injector")
	}
	turns, err := inject(false)
	if err != nil || len(turns) != 1 {
		t.Fatalf("first call = %d turns, %v", len(turns), err)
	}
	var msg struct {
		Role    string `json:"role"`
		Content []struct {
			Type, Text string
		} `json:"content"`
	}
	if err := json.Unmarshal(turns[0], &msg); err != nil || msg.Role != "user" || !strings.HasPrefix(msg.Content[0].Text, calibrationDigestHeader+"\n") {
		t.Fatalf("digest turn = %s", turns[0])
	}
	if again, _ := inject(false); len(again) != 0 {
		t.Fatalf("an unchanged digest was appended again")
	}
	// A resumed activation replays the turn; it is not repeated.
	resumed := rt.calibrationDigestInjector(digestRun, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`), turns[0]})
	if again, _ := resumed(false); len(again) != 0 {
		t.Fatalf("a replayed digest was appended again on resume")
	}
	// A changed record is appended once more.
	seedCalibrationSnapshot(rt, append(digestObservations(30), digestObservations(10)...))
	if changed, _ := resumed(false); len(changed) != 1 {
		t.Fatalf("a changed digest was not appended")
	}
}

func postSettlements(t *testing.T, h *APIHandler, body string, auth bool) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/calibration/settlements", strings.NewReader(body))
	if auth {
		req.Header.Set("X-Authenticated-User", calOwner)
	}
	rec := httptest.NewRecorder()
	h.HandleCalibrationSettlements(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// P11: the settlement route is the owner's, refuses what it should, is
// idempotent, and its records feed the digest the owner reads back.
func TestCalibrationSettlementRoute(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	computer := rt.cfg.ComputerID
	cast := castRecord("cell-m:cast:7", "management:"+calOwner, "task-7", "conv:x",
		types.CastHypothesis{ID: "H1", Statement: "a", Probability: 0.6}, types.CastHypothesis{ID: "H2", Statement: "b", Probability: 0.4})
	report := finalResearchReport("cell-r:report:7", "doc-7", ptr(0.9))
	for _, r := range []types.CommitmentRecord{cast, report} {
		if _, err := s.AppendCommitmentRecord(ctx, calOwner, computer, r); err != nil {
			t.Fatal(err)
		}
	}
	h := &APIHandler{rt: rt}
	body := func(scorer, items string) string {
		return `{"scorer":"` + scorer + `","model_id":"google-antigravity/gemini-3.8-flash","settlements":[` + items + `]}`
	}
	good := `{"target_record_id":"cell-m:cast:7","claim_type":"hypothesis","answers":{"H1":"confirmed","H2":"contradicted"}},` +
		`{"target_record_id":"cell-r:report:7","claim_type":"completed","coverage":0.5}`
	if code, _ := postSettlements(t, h, body("e6a", good), false); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", code)
	}
	if code, _ := postSettlements(t, h, body("E6A!", good), true); code != http.StatusBadRequest {
		t.Fatalf("bad scorer name = %d", code)
	}
	bad := `{"target_record_id":"nope","claim_type":"hypothesis","answers":{"H1":"confirmed"}},` +
		`{"target_record_id":"cell-m:cast:7","claim_type":"hypothesis","answers":{"H9":"confirmed"}},` +
		`{"target_record_id":"cell-m:cast:7","claim_type":"hypothesis","answers":{"H1":"probably"}},` +
		`{"target_record_id":"cell-r:report:7","claim_type":"completed","coverage":1.5},` +
		`{"target_record_id":"cell-m:cast:7","claim_type":"completed","coverage":0.5},` +
		`{"target_record_id":"cell-r:report:7","claim_type":"citation_support"}`
	code, out := postSettlements(t, h, body("e6a", bad), true)
	if refused, _ := out["refused"].([]any); code != http.StatusOK || out["written"].(float64) != 0 || len(refused) != 6 {
		t.Fatalf("bad settlements = %d %v", code, out)
	}
	code, out = postSettlements(t, h, body("e6a", good), true)
	if code != http.StatusOK || out["written"].(float64) != 2 {
		t.Fatalf("good settlements = %d %v", code, out)
	}
	code, out = postSettlements(t, h, body("e6a", good), true)
	if code != http.StatusOK || out["written"].(float64) != 0 || out["existing"].(float64) != 2 {
		t.Fatalf("repeat = %d %v, want idempotent", code, out)
	}
	stored, err := s.GetCommitmentRecord(ctx, calOwner, computer, "calibration:e6a:cell-m:cast:7")
	if err != nil || stored == nil || stored.Discrepancy != types.DiscrepancyUnresolved || stored.Provenance.ModelID != "google-antigravity/gemini-3.8-flash" ||
		stored.Observation.Excerpt != "" || stored.Provenance.ResolvedAt != "" || stored.Scores[0].Answers["H1"] != "confirmed" {
		t.Fatalf("score record = %+v %v", stored, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/calibration?detail=1", nil)
	req.Header.Set("X-Authenticated-User", calOwner)
	rec := httptest.NewRecorder()
	h.HandleCalibration(rec, req)
	var view struct {
		Observations []struct {
			Key, Arm, Thread string
			P, Y             float64
		} `json:"observations"`
		Pending []CalibrationPending `json:"pending"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &view) != nil {
		t.Fatalf("GET = %d %s", rec.Code, rec.Body)
	}
	got := map[string]float64{}
	for _, o := range view.Observations {
		got[o.Key] = o.Y
		if o.Arm != calibration.Arm(o.Thread) {
			t.Fatalf("observation %s arm %s, thread %s", o.Key, o.Arm, o.Thread)
		}
	}
	if len(got) != 3 || got["cell-m:cast:7#H1"] != 1 || got["cell-m:cast:7#H2"] != 0 || got["cell-r:report:7"] != 0.5 || len(view.Pending) != 0 {
		t.Fatalf("observations = %v, pending %d", got, len(view.Pending))
	}
	// No desk reaches either route through choir.ProductAPI.
	for _, path := range []string{"/api/calibration", "/api/calibration/settlements"} {
		if err := validateProductAPIRoute(http.MethodPost, path); err == nil {
			t.Fatalf("ProductAPI allows %s", path)
		}
	}
}
