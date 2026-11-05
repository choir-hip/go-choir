package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The sub-call settler (MA1.5, plan section 4.4): the host-side service that
// scores a fan-out of sub-calls after the parent has its results. It is a
// judge SERVICE of the host, not a desk and not the System 1 cell verb: it
// reuses the verb's judge client (rt.candidateJudge, the one CandidateJudge
// the support judge and the intake labeller use) and its request wire format,
// and nothing of the verb's gate. The verb's research-only role gate and its
// off-until-configured caps (system1_verb.go) do not apply here, and a test
// pins that the settler runs with them unset. It becomes the kernel judge
// service when MA2.1 and MA3.6 land, with the same records.
//
// Layers, in order (plan 4.4): rules first (report_schema_valid,
// citations_resolve; a rule settlement is final for the facts it covers and
// lands before, and independent of, the judge); one blind judge call per
// fan-out (the judge sees the child's bounded report text and the typed
// questions, never who asserted, the parent's probabilities or the parent's
// identity); the parent's self-score, a learning signal that never settles;
// and management's third opinion on sampled material disagreements.
//
// Ways the judge settler can fail, and how each is handled (written before
// the code, MA1.5):
//
//  1. Stall. The judge never answers. The call runs under
//     subCallJudgeTimeout and returns at the deadline even if the transport
//     ignores it (callJudgeWithin); the whole settler is a goroutine, so the
//     fan-out had already returned. The work record is closed "failed" and
//     the Casts stay unsettled; the rule settlements, written first, stand.
//  2. Error. A transport error, an undecodable answer, no decision model
//     bound. Same: the work is closed "failed" with the cause, nothing is
//     settled by the judge, nothing blocks, nothing is retried (a retry is
//     new work).
//  3. Wrong batch mapping. Questions are keyed c<i>.q<j> and c<i>.answered
//     against the batch's own ordered Casts; an answer is read only by the id
//     the batch minted. Unknown ids are ignored, a missing id leaves that
//     question unanswered, a Cast with no usable answer at all stays
//     unsettled. A settlement is written only for a Cast of this batch, and
//     carries the Cast's own id.
//  4. Judging its own family. A judge whose model family is the child's or
//     the parent's is flagged same_family on the settlement; the settlement is
//     kept and never counted as independent (types.SubCallVerdictClass,
//     SelfFavorRows).
//  5. Settling a record twice. Every record id is deterministic (cast,
//     scorer; batch hash), the writer is idempotent, a batch already
//     closed is skipped, and a batch in flight in this process is skipped. A
//     parent can never settle: checkSettlerIndependence refuses any scorer
//     that is not a host service, and any scorer that is the Cast's own
//     parent, child or desk.
//  6. Settling after a crash. The pending queue is in memory only; the work
//     record is the durable fact. A crash boot closes every open work record
//     "interrupted by a restart" and never re-runs it; a planned restart
//     (marker present) re-runs it once (closeOpenJudgeWorkAtBoot). An open
//     work record found outside boot, with no work in flight, is closed
//     interrupted too, never resumed.
//  7. Cost blowup. One call per fan-out (chunks of subCallJudgeBatchMax
//     children at most), a child contributes at most MaxSubCallQuestions + 1
//     questions, each report is bounded to subCallReportRunes, a child whose
//     Report is empty is settled by its rule alone and not sent, and the
//     usage is on the work close and on the parent's run costs. There is no
//     spend ceiling (R8); the cost is attributed and watched.

// subCallJudgeTimeout bounds one judge call. A var so a test can shorten it.
var subCallJudgeTimeout = 90 * time.Second

// subCallDelegateCoalesce is how long a delegated task's close waits for its
// siblings, so the tasks of one Delegate that close together share one judge
// call. A var so a test can shorten it.
var subCallDelegateCoalesce = 3 * time.Second

// subCallJudgeBatchMax is the most children in one judge call; a larger
// fan-out is scored in chunks.
const subCallJudgeBatchMax = 20

// subCallRuleCitationsMax is the most citations one Report's rule checks.
const subCallRuleCitationsMax = 20

// subCallPending is a batch waiting for its judge.
type subCallPending struct {
	owner, computer, fanOut string
	ids                     []string
}

// subCallItem is one Cast of a batch with its Report.
type subCallItem struct {
	cast   types.CommitmentRecord
	report types.CommitmentRecord
}

// enqueueSubCallJudge schedules the settlement of Casts of one fan-out and
// returns at once: nothing waits on the judge. delay lets Casts that arrive
// close together (a Delegate's tasks) share a call.
func (rt *Runtime) enqueueSubCallJudge(owner, computer, fanOut string, castIDs []string, delay time.Duration) {
	if rt == nil || rt.store == nil || len(castIDs) == 0 {
		return
	}
	key := owner + "\x00" + computer + "\x00" + fanOut
	rt.subCallJudgeMu.Lock()
	if rt.subCallJudgePending == nil {
		rt.subCallJudgePending = map[string]*subCallPending{}
	}
	if p, ok := rt.subCallJudgePending[key]; ok {
		p.ids = append(p.ids, castIDs...)
		rt.subCallJudgeMu.Unlock()
		return
	}
	p := &subCallPending{owner: owner, computer: computer, fanOut: fanOut, ids: append([]string(nil), castIDs...)}
	rt.subCallJudgePending[key] = p
	rt.subCallJudgeWG.Add(1)
	rt.subCallJudgeMu.Unlock()
	go func() {
		defer rt.subCallJudgeWG.Done()
		if delay > 0 {
			time.Sleep(delay)
		}
		rt.subCallJudgeMu.Lock()
		delete(rt.subCallJudgePending, key)
		ids := p.ids
		rt.subCallJudgeMu.Unlock()
		if err := rt.settleSubCalls(context.Background(), owner, computer, fanOut, ids); err != nil {
			log.Printf("subcall judge: fan-out %s left unsettled: %v", fanOut, err)
		}
	}()
}

// waitSubCallJudge waits for the settlements in flight. Tests join on it; the
// product never does.
func (rt *Runtime) waitSubCallJudge() { rt.subCallJudgeWG.Wait() }

// loadSubCallItems reads the Casts of ids with their Reports; a Cast with no
// Report yet is not an item.
func (rt *Runtime) loadSubCallItems(ctx context.Context, owner, computer string, ids []string) []subCallItem {
	seen := map[string]bool{}
	var items []subCallItem
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		cast, err := rt.store.GetCommitmentRecord(ctx, owner, computer, id)
		if err != nil || cast == nil || cast.SubCall == nil || cast.SubCall.Kind != types.SubCallCast {
			continue
		}
		report, err := rt.store.GetCommitmentRecord(ctx, owner, computer, types.SubCallReportRecordID(id))
		if err != nil || report == nil {
			continue
		}
		items = append(items, subCallItem{cast: *cast, report: *report})
	}
	return items
}

// settleSubCalls scores a batch: rules first, then the judge. It returns the
// first judge-side error; a rule always lands whatever the judge does.
func (rt *Runtime) settleSubCalls(ctx context.Context, owner, computer, fanOut string, castIDs []string) error {
	items := rt.loadSubCallItems(ctx, owner, computer, castIDs)
	if len(items) == 0 {
		return nil
	}
	ruleRecs := map[string][]types.CommitmentRecord{}
	for _, it := range items {
		ruleRecs[it.cast.RecordID] = rt.settleSubCallRules(ctx, owner, computer, it)
	}
	var judged []subCallItem
	for _, it := range items {
		if rt.subCallHasJudgeSettlement(ctx, owner, computer, it.cast.RecordID) || !subCallReportHasText(it.report) {
			continue
		}
		judged = append(judged, it)
	}
	var first error
	for start := 0; start < len(judged); start += subCallJudgeBatchMax {
		end := min(start+subCallJudgeBatchMax, len(judged))
		if err := rt.judgeSubCallBatch(ctx, owner, computer, fanOut, judged[start:end], ruleRecs); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func subCallReportHasText(report types.CommitmentRecord) bool {
	return report.Report != nil && len(report.Report.Claims) > 0 && strings.TrimSpace(report.Report.Claims[0].Text) != ""
}

func (rt *Runtime) subCallHasJudgeSettlement(ctx context.Context, owner, computer, castID string) bool {
	ok, _ := rt.store.CommitmentRecordExists(ctx, owner, computer, types.SubCallSettlementRecordID(castID, types.SubCallJudgeScorerID))
	return ok
}

// settleSubCallRules runs the rule layer on one Cast's Report and writes the
// settlements. They are final for the facts they cover and do not wait on, or
// depend on, the judge.
func (rt *Runtime) settleSubCallRules(ctx context.Context, owner, computer string, it subCallItem) []types.CommitmentRecord {
	var written []types.CommitmentRecord
	write := func(kind types.PrecommitRuleKind, s types.RuleSettlement) {
		rec := buildRuleSettlement(it, kind, s, time.Now())
		if err := rt.writeSubCallSettlement(ctx, owner, computer, rec); err != nil {
			log.Printf("subcall judge: rule %s on %s: %v", kind, it.cast.RecordID, err)
			return
		}
		written = append(written, rec)
	}
	write(types.PrecommitRuleReportSchemaValid, types.SettleReportSchemaValid(it.report.Report))
	if s, ok := rt.settleCitationsRule(ctx, owner, it.report.Report); ok {
		write(types.PrecommitRuleCitationsResolve, s)
	}
	return written
}

// settleCitationsRule resolves each cited source against the capture store.
// A source the store does not hold does not resolve. A transient store error
// is not "does not resolve": the rule then settles nothing rather than
// contradict on an outage. With no capture store configured it settles
// nothing.
func (rt *Runtime) settleCitationsRule(ctx context.Context, owner string, report *types.CommitmentReport) (types.RuleSettlement, bool) {
	refs := types.ReportCitations(report)
	if len(refs) == 0 {
		return types.SettleCitationsResolve(report, func(string) bool { return true }), true
	}
	captures := rt.TextureCaptures()
	if captures == nil {
		return types.RuleSettlement{}, false
	}
	if len(refs) > subCallRuleCitationsMax {
		refs = refs[:subCallRuleCitationsMax]
	}
	resolved := map[string]bool{}
	for _, ref := range refs {
		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		_, err := captures.CaptureBlocks(callCtx, owner, ref)
		cancel()
		switch {
		case err == nil:
			resolved[ref] = true
		case errors.Is(err, texturewrite.ErrNotCaptured):
			resolved[ref] = false
		default:
			return types.RuleSettlement{}, false
		}
	}
	bounded := &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "x", Sources: refs}}}
	return types.SettleCitationsResolve(bounded, func(ref string) bool { return resolved[ref] }), true
}

func buildRuleSettlement(it subCallItem, kind types.PrecommitRuleKind, s types.RuleSettlement, now time.Time) types.CommitmentRecord {
	stamp := now.UTC().Format(time.RFC3339Nano)
	scorer := types.SubCallRuleScorerPrefix + string(kind)
	failed := s.Answer == types.PrecommitAnswerNo
	class := types.DiscrepancyUnresolved
	if failed {
		class = types.DiscrepancyContradicted
	}
	cb := it.cast.SubCall
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallSettlementRecordID(it.cast.RecordID, scorer),
		Kind: types.CommitmentKindResolve, Discrepancy: class,
		Resolve:     &types.CommitmentResolve{Verdict: s.Answer, EvidenceRefs: s.EvidenceRefs},
		Observation: types.CommitmentObservation{Excerpt: s.Excerpt, SourceRef: it.report.RecordID, ObservedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: scorer, CommittedAt: stamp, Desk: scorer, RunID: cb.ParentRunID, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: it.cast.ComputerID, ParentID: it.cast.RecordID,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallSettlement, FanOutID: cb.FanOutID, Role: cb.Role, CastID: it.cast.RecordID,
			Method: types.CalibrationScorerRule, Scorer: scorer, Rule: string(kind), Contradicted: failed, Class: class, Reason: s.Excerpt},
	}
}

// checkSettlerIndependence refuses a settlement by anyone who may not settle
// the Cast: a scorer must be a host service (never a desk), and never the
// Cast's own parent, desk or child (plan 5.1: the committer never settles its
// own record).
func checkSettlerIndependence(cast types.CommitmentRecord, scorer string) error {
	scorer = strings.TrimSpace(scorer)
	if !strings.HasPrefix(scorer, "system:") {
		return fmt.Errorf("settlement of %s refused: %q is not a host settler service", cast.RecordID, scorer)
	}
	cb := cast.SubCall
	parties := []string{cast.Provenance.AgentID, cast.Provenance.Desk, cast.Addressee}
	if cb != nil {
		parties = append(parties, cb.ParentAgent, cb.ChildKey, cb.Role+":"+cb.ChildKey)
	}
	for _, p := range parties {
		if strings.TrimSpace(p) != "" && strings.EqualFold(strings.TrimSpace(p), scorer) {
			return fmt.Errorf("settlement of %s refused: %q is a party to the Cast and cannot settle it", cast.RecordID, scorer)
		}
	}
	return nil
}

// writeSubCallSettlement is the one path a settlement is written through. It
// loads the Cast it settles, refuses a scorer that may not settle it, and
// writes idempotently.
func (rt *Runtime) writeSubCallSettlement(ctx context.Context, owner, computer string, rec types.CommitmentRecord) error {
	b, ok := rec.SubCallBody(types.SubCallSettlement)
	if !ok {
		return fmt.Errorf("record %s is not a sub-call settlement", rec.RecordID)
	}
	cast, err := rt.store.GetCommitmentRecord(ctx, owner, computer, b.CastID)
	if err != nil {
		return err
	}
	if cast == nil || cast.SubCall == nil || cast.SubCall.Kind != types.SubCallCast {
		return fmt.Errorf("settlement %s: no sub-call Cast %q", rec.RecordID, b.CastID)
	}
	for _, scorer := range []string{b.Scorer, rec.Provenance.AgentID} {
		if err := checkSettlerIndependence(*cast, scorer); err != nil {
			return err
		}
	}
	return rt.appendSubCallRecord(ctx, owner, computer, rec)
}

// judgeSubCallBatch scores one chunk of a fan-out in one judge call.
func (rt *Runtime) judgeSubCallBatch(ctx context.Context, owner, computer, fanOut string, items []subCallItem, ruleRecs map[string][]types.CommitmentRecord) error {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.cast.RecordID
	}
	workID := types.SubCallJudgeWorkID(fanOut, ids)
	rt.subCallJudgeMu.Lock()
	if rt.subCallJudgeActive == nil {
		rt.subCallJudgeActive = map[string]bool{}
	}
	if rt.subCallJudgeActive[workID] {
		rt.subCallJudgeMu.Unlock()
		return nil
	}
	rt.subCallJudgeActive[workID] = true
	rt.subCallJudgeMu.Unlock()
	defer func() {
		rt.subCallJudgeMu.Lock()
		delete(rt.subCallJudgeActive, workID)
		rt.subCallJudgeMu.Unlock()
	}()

	closeID := types.SubCallJudgeCloseRecordID(workID)
	if done, _ := rt.store.CommitmentRecordExists(ctx, owner, computer, closeID); done {
		return nil
	}
	if open, _ := rt.store.CommitmentRecordExists(ctx, owner, computer, workID); open {
		// Work a past process left open: a crash never resumes it.
		rt.closeJudgeWork(ctx, owner, computer, workID, fanOut, types.SubCallPhaseInterrupted, "interrupted by a restart", "", 0, 0, 0)
		return nil
	}
	now := time.Now().UTC()
	stamp := now.Format(time.RFC3339Nano)
	work := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: workID, Kind: types.CommitmentKindResolve, Discrepancy: types.DiscrepancyUnresolved,
		Observation: types.CommitmentObservation{ObservedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: types.SubCallJudgeScorerID, CommittedAt: stamp, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computer,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallJudgeWork, FanOutID: fanOut, Casts: ids, Phase: "requested"},
	}
	if err := rt.appendSubCallRecord(ctx, owner, computer, work); err != nil {
		return fmt.Errorf("record the judge work: %w", err)
	}
	if rt.candidateJudge == nil {
		err := fmt.Errorf("no decision model is bound on this computer")
		rt.closeJudgeWork(ctx, owner, computer, workID, fanOut, types.SubCallPhaseFailed, err.Error(), "", 0, 0, 0)
		return err
	}

	state, rawQuestions, err := subCallJudgeRequest(items)
	if err != nil {
		rt.closeJudgeWork(ctx, owner, computer, workID, fanOut, types.SubCallPhaseFailed, err.Error(), "", 0, 0, 0)
		return err
	}
	raw, err := callJudgeWithin(ctx, rt.candidateJudge, "", subCallJudgeTimeout, state, rawQuestions)
	if err != nil {
		rt.closeJudgeWork(ctx, owner, computer, workID, fanOut, types.SubCallPhaseFailed, system1Message(err), "", 0, 0, 0)
		return err
	}
	var dist jevDistribution
	if err := json.Unmarshal(raw, &dist); err != nil {
		rt.closeJudgeWork(ctx, owner, computer, workID, fanOut, types.SubCallPhaseFailed, "the judge's answer did not decode: "+system1Message(err), "", 0, 0, 0)
		return err
	}
	model := firstNonEmpty(dist.Model, candidateJudgmentModel)
	settled := 0
	for i, it := range items {
		rec, ok := buildJudgeSettlement(it, i, dist, model, time.Now())
		if !ok {
			continue
		}
		if err := rt.writeSubCallSettlement(ctx, owner, computer, rec); err != nil {
			log.Printf("subcall judge: settle %s: %v", it.cast.RecordID, err)
			continue
		}
		settled++
		rt.sampleMaterialDisagreement(ctx, owner, computer, it, append(append([]types.CommitmentRecord(nil), ruleRecs[it.cast.RecordID]...), rec))
	}
	if first := items[0].cast.SubCall; first != nil {
		rt.recordJevCostFor(ctx, owner, first.ParentRunID, first.ParentAgent, "subcall_judge", model, dist.Usage, len(rawQuestions))
	}
	reason := fmt.Sprintf("%d of %d Casts settled", settled, len(items))
	rt.closeJudgeWork(ctx, owner, computer, workID, fanOut, types.SubCallPhaseAnswered, reason, model, dist.Usage.InputTokens, dist.Usage.OutputTokens, dist.Usage.Cost)
	return nil
}

// closeJudgeWork writes the close of a judge work record. A close is written
// once; a second is a no-op.
func (rt *Runtime) closeJudgeWork(ctx context.Context, owner, computer, workID, fanOut, phase, reason, model string, in, out int, cost float64) {
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallJudgeCloseRecordID(workID), Kind: types.CommitmentKindResolve,
		Discrepancy: types.DiscrepancyUnresolved, Observation: types.CommitmentObservation{Excerpt: reason, ObservedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: types.SubCallJudgeScorerID, ModelID: model, CommittedAt: stamp, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computer, ParentID: workID,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallJudgeClose, FanOutID: fanOut, WorkID: workID, Phase: phase, Reason: reason, Model: model,
			InputTokens: in, OutputTokens: out, CostUSD: cost},
	}
	if err := rt.appendSubCallRecord(context.WithoutCancel(ctx), owner, computer, rec); err != nil {
		log.Printf("subcall judge: close %s: %v", workID, err)
	}
}

// subCallJudgeRequest builds the one blind request of a batch: a bounded
// report per child as evidence, one "did it answer" question per child and
// one choice question per typed question. The parent's probabilities, its
// identity and the child's are in none of it.
func subCallJudgeRequest(items []subCallItem) (json.RawMessage, map[string]json.RawMessage, error) {
	contexts := map[string]system1Context{}
	aliasOf := map[string]string{}
	var questions []yaegikernel.System1Question
	for i, it := range items {
		ref := fmt.Sprintf("child-%d", i)
		text := ""
		if it.report.Report != nil && len(it.report.Report.Claims) > 0 {
			text = boundRunes(strings.Join(strings.Fields(it.report.Report.Claims[0].Text), " "), subCallReportRunes)
		}
		contexts[ref] = system1Context{Kind: "report", Ref: ref, Text: text}
		aliasOf[ref] = fmt.Sprintf("x%d", i+1)
		objective := boundRunes(strings.Join(strings.Fields(it.cast.Prediction.Hypothesis), " "), 300)
		questions = append(questions, yaegikernel.System1Question{
			ID:          types.SubCallAnsweredID(i),
			Text:        "A child was asked to: " + objective + " Does its report, the evidence, answer that? answered means fully, partly means in part, not_answered means it does not.",
			Choices:     []string{"answered", "partly", "not_answered"},
			ContextRefs: []string{ref},
		})
		for j, q := range it.cast.Prediction.Questions {
			questions = append(questions, yaegikernel.System1Question{
				ID: types.SubCallQuestionID(i, j), Text: q.Question + " Answer from the report only.",
				Choices: append([]string(nil), q.Answers...), ContextRefs: []string{ref},
			})
		}
	}
	return system1Request(questions, contexts, aliasOf)
}

// buildJudgeSettlement reads child i's answers out of the judge's
// distribution by the ids the batch minted and builds the independent
// settlement. ok is false when the judge gave nothing usable for the Cast.
func buildJudgeSettlement(it subCallItem, i int, dist jevDistribution, model string, now time.Time) (types.CommitmentRecord, bool) {
	qs := it.cast.Prediction.Questions
	var verdicts []types.SubCallVerdict
	for j, q := range qs {
		given, has := dist.Answers[types.SubCallQuestionID(i, j)]
		if !has {
			continue
		}
		probs := map[string]float64{}
		for _, a := range q.Answers {
			if p, known := given.Probabilities[a]; known {
				probs[a] = p
			}
		}
		answer := ""
		best := -1.0
		for _, a := range q.Answers {
			if p, known := probs[a]; known && p > best {
				answer, best = a, p
			}
		}
		if answer == "" {
			for _, a := range q.Answers {
				if a == strings.TrimSpace(given.Choice) {
					answer = a
				}
			}
		}
		if answer == "" {
			continue
		}
		verdicts = append(verdicts, types.SubCallVerdict{Q: j, Answer: answer, Probabilities: probs})
	}
	var answered map[string]float64
	if given, has := dist.Answers[types.SubCallAnsweredID(i)]; has && len(given.Probabilities) > 0 {
		answered = map[string]float64{}
		for _, k := range []string{"answered", "partly", "not_answered"} {
			if p, known := given.Probabilities[k]; known {
				answered[k] = p
			}
		}
	}
	class := types.ClassifySubCall(qs, verdicts, answered)
	if len(verdicts) == 0 && len(answered) == 0 {
		return types.CommitmentRecord{}, false
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	cb := it.cast.SubCall
	family := types.ModelFamily(model)
	same := family != "" && (family == types.ModelFamily(it.report.Provenance.ModelID) || family == types.ModelFamily(it.cast.Provenance.ModelID))
	answers := map[string]string{}
	for _, v := range verdicts {
		answers[fmt.Sprintf("q%d", v.Q)] = v.Answer
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallSettlementRecordID(it.cast.RecordID, types.SubCallJudgeScorerID),
		Kind: types.CommitmentKindResolve, Discrepancy: class,
		Resolve:     &types.CommitmentResolve{Verdict: string(class), EvidenceRefs: []string{it.report.RecordID}},
		Observation: types.CommitmentObservation{Excerpt: string(class), SourceRef: it.report.RecordID, ObservedAt: stamp},
		Scores:      []types.CommitmentScore{{ScorerModelID: model, ScorerAgentID: types.SubCallJudgeScorerID, Answers: answers, ScoredAt: stamp}},
		Provenance:  types.CommitmentProvenance{AgentID: types.SubCallJudgeScorerID, ModelID: model, CommittedAt: stamp, Desk: types.SubCallJudgeScorerID, RunID: cb.ParentRunID, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: it.cast.ComputerID, ParentID: it.cast.RecordID,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallSettlement, FanOutID: cb.FanOutID, Role: cb.Role, CastID: it.cast.RecordID,
			Method: types.CalibrationScorerIndependent, Scorer: types.SubCallJudgeScorerID, Model: model, Class: class,
			Contradicted: class == types.DiscrepancyContradicted, Verdicts: verdicts, Answered: answered, SameFamily: same},
	}, true
}

// sampleMaterialDisagreement writes management's third-opinion request for a
// Cast whose self-score and independent score disagree materially, sampled
// and weighted by materiality (plan 4.4, layer 5). It is a record, not a
// wake: management is not the default scorer and nothing waits on it.
func (rt *Runtime) sampleMaterialDisagreement(ctx context.Context, owner, computer string, it subCallItem, settlements []types.CommitmentRecord) {
	rows := types.SelfFavorRows([]types.CommitmentRecord{it.cast}, settlements)
	if len(rows) != 1 || !rows[0].Material() || !types.ThirdOpinionSampled(it.cast.RecordID, rows[0].Weight) {
		return
	}
	class, _ := types.SubCallVerdictClass(settlements)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	cb := it.cast.SubCall
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallDisagreementRecordID(it.cast.RecordID),
		Kind: types.CommitmentKindDisagreement, Discrepancy: types.DiscrepancyUnresolved,
		Disagreement: &types.CommitmentDisagreement{CommitmentID: it.cast.RecordID, ScorerVerdict: string(class),
			ResolverVerdict: fmt.Sprintf("self %.2f, independent %.2f", rows[0].Self, rows[0].Independent), EvidenceRefs: []string{it.report.RecordID}},
		Observation: types.CommitmentObservation{ObservedAt: stamp},
		Provenance:  types.CommitmentProvenance{AgentID: types.SubCallJudgeScorerID, CommittedAt: stamp, Origin: types.OriginSystem},
		EmitterKind: types.CommitmentEmitterSystem, ComputerID: computer, ParentID: it.cast.RecordID,
		Addressee: persistentManagementAgentID(owner),
		SubCall:   &types.CommitmentSubCall{Kind: types.SubCallDisagreement, FanOutID: cb.FanOutID, Role: cb.Role, CastID: it.cast.RecordID},
	}
	if err := rt.appendSubCallRecord(ctx, owner, computer, rec); err != nil {
		log.Printf("subcall judge: third-opinion request for %s: %v", it.cast.RecordID, err)
	}
}

// closeOpenJudgeWorkAtBoot is the boot pass for judge work. A crash boot
// (no planned-restart marker) closes every open work record "interrupted by a
// restart" and never re-runs it. A planned restart re-runs it once: the work
// was consumed by the marker, and a re-run that crashes the computer leads to
// a markerless boot that closes it.
func (rt *Runtime) closeOpenJudgeWorkAtBoot(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	owner := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if owner == "" {
		owner = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computer := strings.TrimSpace(rt.TextureComputerID())
	if owner == "" || computer == "" {
		return
	}
	rt.closeOpenJudgeWork(ctx, owner, computer, rt.bootResumesWork())
}

func (rt *Runtime) closeOpenJudgeWork(ctx context.Context, owner, computer string, resume bool) {
	works, err := rt.store.ListSubCallRecords(ctx, owner, computer, types.SubCallJudgeWork, 2000)
	if err != nil {
		log.Printf("subcall judge: boot: list work: %v", err)
		return
	}
	closes, err := rt.store.ListSubCallRecords(ctx, owner, computer, types.SubCallJudgeClose, 4000)
	if err != nil {
		log.Printf("subcall judge: boot: list closes: %v", err)
		return
	}
	closed := map[string]bool{}
	for _, c := range closes {
		if c.SubCall != nil {
			closed[c.SubCall.WorkID] = true
		}
	}
	for _, w := range works {
		if closed[w.RecordID] || w.SubCall == nil {
			continue
		}
		if resume {
			// One resume: the stale work is closed (superseded) and its Casts
			// are judged again under a fresh batch id. That run's own work
			// record is open until it closes; a crash during it leaves it to
			// the next, markerless boot, which closes it.
			rt.closeJudgeWork(ctx, owner, computer, w.RecordID, w.SubCall.FanOutID, types.SubCallPhaseInterrupted, "superseded by the planned restart's one resume", "", 0, 0, 0)
			rt.enqueueSubCallJudge(owner, computer, w.SubCall.FanOutID+":resume", w.SubCall.Casts, 0)
			continue
		}
		rt.closeJudgeWork(ctx, owner, computer, w.RecordID, w.SubCall.FanOutID, types.SubCallPhaseInterrupted, "interrupted by a restart", "", 0, 0, 0)
		log.Printf("subcall judge: crash boot closed open judge work %s", w.RecordID)
	}
}

// SubCallSelfFavor is the self-favor gap view: the parent's self-score
// against the independent score, per agent and model, computed from the
// replayed records of the computer (plan 4.4 and 5.2).
func (rt *Runtime) SubCallSelfFavor(ctx context.Context, owner, computer string) ([]types.SelfFavorGroup, []types.SelfFavorRow, error) {
	if rt == nil || rt.store == nil {
		return nil, nil, fmt.Errorf("sub-call self-favor: no store")
	}
	rows, err := SelfFavorFromStore(ctx, rt.store, owner, computer)
	if err != nil {
		return nil, nil, err
	}
	return types.SelfFavorView(rows), rows, nil
}

// SelfFavorFromStore replays a computer's sub-call Casts and settlements into
// self-favor rows.
func SelfFavorFromStore(ctx context.Context, s *store.Store, owner, computer string) ([]types.SelfFavorRow, error) {
	casts, err := s.ListSubCallRecords(ctx, owner, computer, types.SubCallCast, 5000)
	if err != nil {
		return nil, err
	}
	settlements, err := s.ListSubCallRecords(ctx, owner, computer, types.SubCallSettlement, 20000)
	if err != nil {
		return nil, err
	}
	return types.SelfFavorRows(casts, settlements), nil
}
