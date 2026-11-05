package agentcore

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Sub-call records (MA1.5, plan section 4.4): the Cast a call to a child is,
// the child's Report, and the parent's self-score. They are written through
// the store's plain commitment-record path: the sub-call plane is an audit
// plane (no inbox, no round, no wake), and nothing here asks a model.
//
// The parent's precommitment is never invented. A probability on a Cast is
// the one the deciding model stated in its own cell; with none, the Cast is
// marked weight zero and says why (types.CommitmentRecord.WeightZero). A
// stated p_success with no typed question becomes the one question it already
// is: "will the child achieve its objective", yes with p_success.

// subCallReportRunes bounds the text a Report record carries, and the text a
// judge is shown (cost: the judge's input tokens scale with it).
const subCallReportRunes = 6000

// subCallWeightZeroUnstated is the reason of a Cast whose parent stated no
// p_success or weight.
const subCallWeightZeroUnstated = "the parent stated no p_success and weight for this call"

// subCallCastParams is everything a sub-call Cast is built from.
type subCallCastParams struct {
	RecordID    string
	FanOutID    string
	ChildKey    string
	ChildRole   string
	ChildInbox  string
	Objective   string
	ParentAgent string
	ParentRunID string
	ParentModel string
	ComputerID  string
	Origin      string
	// Questions, PSuccess and Weight are the parent's own statements, taken
	// from its cell; any may be absent.
	Questions []types.TypedQuestion
	PSuccess  *float64
	Weight    *float64
	// WeightZeroReason is the reason to give when the Cast states no weight,
	// instead of subCallWeightZeroUnstated (a host-authored Cast).
	WeightZeroReason string
	// Settlement is how the Cast settles when the parent named one.
	Settlement string
	Now        time.Time
}

// buildSubCallCast builds the Cast of one sub-call. It invents nothing: the
// record states a weight and p_success only when the parent did, and
// otherwise carries the weight-zero mark.
func buildSubCallCast(p subCallCastParams) (types.CommitmentRecord, error) {
	stamp := p.Now.UTC().Format(time.RFC3339Nano)
	agent := firstNonEmpty(p.ParentAgent, "run:"+p.ParentRunID)
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: p.RecordID,
		Kind: types.CommitmentKindDirective, Discrepancy: types.DiscrepancyUnresolved,
		Directive:   &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: p.Objective},
		Cast:        &types.CommitmentCast{},
		Prediction:  types.CommitmentPrediction{Hypothesis: p.Objective, CommittedAt: stamp},
		Observation: types.CommitmentObservation{ObservedAt: stamp},
		Provenance: types.CommitmentProvenance{AgentID: agent, ModelID: p.ParentModel, CommittedAt: stamp,
			Desk: agent, RunID: p.ParentRunID, Origin: firstNonEmpty(p.Origin, types.OriginDeskRun)},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: p.ComputerID,
		Addressee: p.ChildInbox,
		ChildIDs:  []string{types.SubCallReportRecordID(p.RecordID)},
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallCast, FanOutID: p.FanOutID, Role: p.ChildRole,
			ParentRunID: p.ParentRunID, ParentAgent: agent, ChildKey: p.ChildKey},
	}
	if err := applySubCallPrecommitment(&rec, p); err != nil {
		return rec, err
	}
	return rec, nil
}

// applySubCallPrecommitment puts the parent's own statements on a sub-call
// Cast: its typed questions, and p_success and weight when it stated both
// (with a positive weight); otherwise the weight-zero mark. It then sets the
// cheap inertness flags and checks the Cast. Nothing is invented.
func applySubCallPrecommitment(rec *types.CommitmentRecord, p subCallCastParams) error {
	questions := append([]types.TypedQuestion(nil), p.Questions...)
	stated := p.PSuccess != nil && p.Weight != nil && *p.Weight > 0
	if len(questions) == 0 && p.PSuccess != nil && p.WeightZeroReason == "" {
		w := 0.0
		if p.Weight != nil {
			w = *p.Weight
		}
		questions = []types.TypedQuestion{{
			Question:      "Will the child achieve its objective, so that its Report answers what was asked?",
			Answers:       []string{"yes", "no"},
			Probabilities: map[string]float64{"yes": *p.PSuccess, "no": 1 - *p.PSuccess},
			Weight:        w,
		}}
	}
	if err := types.ValidateTypedQuestions(questions); err != nil {
		return fmt.Errorf("sub-call Cast %s: %w", p.RecordID, err)
	}
	rec.Prediction.Questions = questions
	if stated && p.WeightZeroReason == "" {
		f := &types.RequestFields{PSuccess: p.PSuccess, Weight: p.Weight, Settlement: firstNonEmpty(p.Settlement, types.SettlementRecipient), Questions: questions}
		if err := rec.StampRequest(types.RequestCast, f); err != nil {
			return fmt.Errorf("sub-call Cast %s: %w", p.RecordID, err)
		}
	} else {
		reason := subCallWeightZeroUnstated
		if p.Weight != nil && *p.Weight == 0 {
			reason = "the parent stated weight 0 for this call"
		}
		rec.WeightZero = firstNonEmpty(p.WeightZeroReason, reason)
	}
	rec.InertFlags = types.InertFlagsFor(p.ChildRole, *rec)
	return types.ValidateSubCallCast(*rec)
}

var subCallURLPattern = regexp.MustCompile(`https?://[^\s<>"'\x60\]\)]+`)

// subCallCitations returns the distinct URLs a child's text cites, in order,
// without trailing punctuation.
func subCallCitations(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range subCallURLPattern.FindAllString(text, -1) {
		m = strings.TrimRight(m, ".,;:!?")
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func boundRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// subCallReportParams is what a child's Report is built from.
type subCallReportParams struct {
	Cast  types.CommitmentRecord
	Child string
	Model string
	Text  string
	// Error is the child's failure, if it failed; the Report still records
	// whatever text it had.
	Error string
	// Sources, when set, are the Report's citations instead of the URLs in
	// its text (a task cites the captures it disposed).
	Sources []string
	Now     time.Time
}

// buildSubCallReport builds the child's Report: its final text as one claim
// with the sources it cites, a stop reason, and the nested-commitment edge to
// the Cast.
func buildSubCallReport(p subCallReportParams) types.CommitmentRecord {
	stamp := p.Now.UTC().Format(time.RFC3339Nano)
	text := strings.TrimSpace(p.Text)
	cites := subCallCitations(text)
	if p.Sources != nil {
		cites = p.Sources
	}
	stop := "done"
	if p.Error != "" {
		stop = "error: " + boundRunes(p.Error, 200)
	}
	var claims []types.ReportClaim
	if text != "" {
		claims = []types.ReportClaim{{Text: boundRunes(text, subCallReportRunes), Sources: cites}}
	}
	cb := p.Cast.SubCall
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallReportRecordID(p.Cast.RecordID),
		Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		Report:      &types.CommitmentReport{Claims: claims, StopReason: stop},
		Observation: types.CommitmentObservation{Excerpt: boundRunes(text, 500), SourceRef: p.Cast.RecordID, ObservedAt: stamp},
		Provenance: types.CommitmentProvenance{AgentID: p.Child, ModelID: p.Model, CommittedAt: stamp,
			Desk: p.Child, RunID: cb.ParentRunID, Origin: types.OriginDeskRun},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: p.Cast.ComputerID,
		Addressee: cb.ParentAgent, EvidenceRefs: cites,
		ParentID: p.Cast.RecordID, Final: true,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallReport, FanOutID: cb.FanOutID, Role: cb.Role,
			ParentRunID: cb.ParentRunID, ParentAgent: cb.ParentAgent, ChildKey: cb.ChildKey, CastID: p.Cast.RecordID, Model: p.Model},
	}
}

// buildSubCallSelfScore is the parent's own score of its Cast: the answer it
// ranked first for each question and its confidence. It is a learning signal;
// it never settles anything (plan 5.2).
func buildSubCallSelfScore(cast types.CommitmentRecord, now time.Time) (types.CommitmentRecord, bool) {
	self := types.SelfAnswers(cast.Prediction.Questions)
	if len(self) == 0 {
		return types.CommitmentRecord{}, false
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	cb := cast.SubCall
	answers := map[string]string{}
	probs := map[string]float64{}
	for _, a := range self {
		id := fmt.Sprintf("q%d", a.Q)
		answers[id], probs[id] = a.Answer, a.P
	}
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: types.SubCallSelfScoreRecordID(cast.RecordID),
		Kind: types.CommitmentKindResolve, Discrepancy: types.DiscrepancyUnresolved,
		Observation: types.CommitmentObservation{ObservedAt: stamp},
		Scores: []types.CommitmentScore{{ScorerModelID: cast.Provenance.ModelID, ScorerAgentID: cb.ParentAgent, Self: true,
			Answers: answers, Probabilities: probs, ScoredAt: stamp}},
		Provenance: types.CommitmentProvenance{AgentID: cb.ParentAgent, ModelID: cast.Provenance.ModelID, CommittedAt: stamp,
			Desk: cb.ParentAgent, RunID: cb.ParentRunID, Origin: types.OriginDeskRun},
		EmitterKind: types.CommitmentEmitterAgent, ComputerID: cast.ComputerID, ParentID: cast.RecordID,
		SubCall: &types.CommitmentSubCall{Kind: types.SubCallSelfScore, FanOutID: cb.FanOutID, Role: cb.Role,
			CastID: cast.RecordID, Self: self},
	}, true
}

// appendSubCallRecord writes one sub-call record. A record that is already
// there is left as it is (the writer is idempotent), so a replay or a second
// writer re-mints nothing.
func (rt *Runtime) appendSubCallRecord(ctx context.Context, owner, computer string, rec types.CommitmentRecord) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("sub-call record %s: no store", rec.RecordID)
	}
	if err := types.ValidateRequestRecord(rec); err != nil {
		return err
	}
	_, err := rt.store.AppendCommitmentRecord(ctx, owner, computer, rec)
	return err
}

// subCallRecorded is a research child's Cast, as written.
type subCallRecorded struct {
	cast          types.CommitmentRecord
	owner, comput string
}

// openChildCast writes the Cast of one research child before the child
// starts: the parent's own typed questions and probabilities when it stated
// them, otherwise the weight-zero mark. A computer with no store (a test
// harness) records nothing and the child runs as before; with a store, a Cast
// that cannot be recorded means the child does not run.
func (rt *Runtime) openChildCast(ctx context.Context, parent toolregistry.ExecutionContext, lineage researchLineage, spec yaegikernel.ResearchChild, fan researchFanOut, childKey string) (*subCallRecorded, error) {
	owner, computer := firstNonEmpty(parent.OwnerID, lineage.ownerID), strings.TrimSpace(parent.ComputerID)
	if rt == nil || rt.store == nil || owner == "" || computer == "" {
		return nil, nil
	}
	cast, err := buildSubCallCast(subCallCastParams{
		RecordID: types.SubCallCastRecordID(fan.id, fan.index), FanOutID: fan.id, ChildKey: childKey,
		ChildRole: researchChildRole, ChildInbox: researchChildRole + ":" + childKey, Objective: spec.Objective,
		ParentAgent: parent.AgentID, ParentRunID: parent.RunID, ParentModel: lineage.selection.Model, ComputerID: computer,
		Questions: spec.Questions, PSuccess: spec.PSuccess, Weight: spec.Weight, Now: time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("the call could not be made a Cast: %w", err)
	}
	if err := rt.appendSubCallRecord(context.WithoutCancel(ctx), owner, computer, cast); err != nil {
		return nil, fmt.Errorf("the call's Cast could not be recorded, so the child did not run: %w", err)
	}
	return &subCallRecorded{cast: cast, owner: owner, comput: computer}, nil
}

// closeChildCast records the child's Report and the parent's self-score, and
// only then hands the findings back. A Report that cannot be recorded is not
// returned: an unrecorded result would be unscored (no sub-call is exempt).
func (rt *Runtime) closeChildCast(ctx context.Context, rec *subCallRecorded, result yaegikernel.ResearchChildResult, childKey string) yaegikernel.ResearchChildResult {
	if rec == nil {
		return result
	}
	ctx = context.WithoutCancel(ctx)
	now := time.Now()
	report := buildSubCallReport(subCallReportParams{Cast: rec.cast, Child: researchChildRole + ":" + childKey,
		Model: result.Model, Text: result.Findings, Error: result.Error, Now: now})
	result.CastID, result.WeightZero = rec.cast.RecordID, rec.cast.WeightZero
	if err := rt.appendSubCallRecord(ctx, rec.owner, rec.comput, report); err != nil {
		log.Printf("subcall: record the Report of %s: %v", rec.cast.RecordID, err)
		result.Findings = ""
		result.Error = strings.TrimSpace(result.Error + "; the child's Report could not be recorded, so its findings are not returned: " + err.Error())
		return result
	}
	result.ReportID = report.RecordID
	if self, ok := buildSubCallSelfScore(rec.cast, now); ok {
		if err := rt.appendSubCallRecord(ctx, rec.owner, rec.comput, self); err != nil {
			log.Printf("subcall: record the self-score of %s: %v", rec.cast.RecordID, err)
		}
	}
	return result
}
