package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/calibration"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Calibration experiment E1 (docs/design/calibration-experiment-e1-2026-11-04.md).
//
// E1-M: on a computer holding calibration.e1_restate, the cell result that
// committed management's engineering Cast asks for one restatement of the
// round's predictions; the Cast's arm (C2) decides whether the request
// shows management's own per-kind record (digest: the record arm) or not
// (control: blind). E1-E: a computer holding one of the engineering
// delivery settings gets that delivery (pull, stub, two-stage, no seed)
// instead of the default push; two-stage asks the engineer to restate the
// rule Precommits a cell committed, by the same record and blind arms (its
// thread's). A restatement is p1, recorded beside p0 and settled by the same
// event; the failure modes F1 to F9 are the protocol's.

// The engineering deliveries.
const (
	calibrationDeliveryPush     = "push"
	calibrationDeliveryPull     = "pull"
	calibrationDeliveryStub     = "stub"
	calibrationDeliveryTwoStage = "two_stage"
	calibrationDeliveryBaseline = "baseline"
)

// Claim refs E1 writes.
const (
	calibrationDeliveryRef    = "calibration_delivery"
	calibrationRecordShownRef = "record_shown"
	calibrationRestateCellRef = "restate_cell"
)

// calibrationRestateWindow bounds a management restatement after its Cast:
// later is late (F2), since the engineer's work has started to show.
const calibrationRestateWindow = 15 * time.Minute

// restateAsk is one first statement a two-stage engineer may restate: its
// kind and its own p0 record.
type restateAsk struct {
	kind string
	p0   string
}

// calibrationRestatement reports a p1 record (F1).
func calibrationRestatement(rec types.CommitmentRecord) bool {
	return types.RestatementRulePrecommit(rec)
}

// calibrationDeliveryFor is the computer's engineering delivery: the one
// E1-E setting it holds, or push. Two or more is a misconfiguration: push,
// logged (F6).
func (rt *Runtime) calibrationDeliveryFor(ctx context.Context, ownerID, computerID string) string {
	var on []string
	for _, c := range []struct{ key, delivery string }{
		{store.SettingCalibrationE1Pull, calibrationDeliveryPull},
		{store.SettingCalibrationE1Stub, calibrationDeliveryStub},
		{store.SettingCalibrationE1TwoStage, calibrationDeliveryTwoStage},
		{store.SettingCalibrationE1Baseline, calibrationDeliveryBaseline},
	} {
		if rt.computerSettingOn(ctx, ownerID, computerID, c.key) {
			on = append(on, c.delivery)
		}
	}
	switch len(on) {
	case 0:
		return calibrationDeliveryPush
	case 1:
		return on[0]
	}
	log.Printf("calibration: computer %s holds %d E1 engineering arms (%s); it runs the default, push", computerID, len(on), strings.Join(on, ", "))
	return calibrationDeliveryPush
}

// calibrationDeliveryOfRun is an engineering run's delivery; "" for any
// other desk.
func (rt *Runtime) calibrationDeliveryOfRun(ctx context.Context, rec *types.RunRecord) string {
	if calibrationDeskOfRun(rec) != agentprofile.Engineering {
		return ""
	}
	return rt.calibrationDeliveryFor(ctx, rec.OwnerID, rec.ComputerID)
}

// calibrationArmForDelivery is the arm label an engineer's claim carries:
// what the run was given. Push and two-stage split by the thread (digest is
// the pushed half, or the record half); pull and stub treat every run; no
// seed treats none.
func calibrationArmForDelivery(delivery, threadArm string) string {
	switch delivery {
	case calibrationDeliveryPull, calibrationDeliveryStub:
		return calibration.ArmDigest
	case calibrationDeliveryBaseline:
		return calibration.ArmControl
	}
	return threadArm
}

// calibrationComputedDigest is the run desk's digest without the push gate,
// for the pull and stub deliveries; nil under MinN.
func (rt *Runtime) calibrationComputedDigest(ctx context.Context, rec *types.RunRecord) *types.CalibrationDigest {
	desk := calibrationDeskOfRun(rec)
	if rt == nil || rt.store == nil || desk == "" {
		return nil
	}
	snap, err := rt.calibrationSnapshotFor(ctx, rec.OwnerID, rec.ComputerID, calibrationRefresh)
	if err != nil {
		log.Printf("calibration: %s: %v", rec.RunID, err)
		return nil
	}
	digest := calibration.Compute(desk, snap.Observations, snap.digestOptions(desk, recordModelID(ctx, rec)))
	if digest.Text == "" {
		return nil
	}
	digest.ComputedAt = snap.At.Format(time.RFC3339)
	return &digest
}

// calibrationReadableForRun is what choir.Calibration() returns to the run,
// and the delivery it is read under: push gives the pushed half its digest;
// pull and stub give every engineer run the record; two-stage and no seed
// give none (two-stage's record reaches the run only through its
// restatement request).
func (rt *Runtime) calibrationReadableForRun(ctx context.Context, rec *types.RunRecord) (*types.CalibrationDigest, string) {
	delivery := rt.calibrationDeliveryOfRun(ctx, rec)
	switch delivery {
	case calibrationDeliveryPull, calibrationDeliveryStub:
		return rt.calibrationComputedDigest(ctx, rec), delivery
	case calibrationDeliveryTwoStage, calibrationDeliveryBaseline:
		return nil, delivery
	}
	if delivery == "" {
		delivery = calibrationDeliveryPush
	}
	return rt.calibrationDigestForRun(ctx, rec), delivery
}

// stampEngineerDelivery marks an engineer's own claim with the delivery it
// got and its arm under that delivery, and as a first statement (F6).
func (rt *Runtime) stampEngineerDelivery(ctx context.Context, rec *types.CommitmentRecord, ownerID, computerID string) {
	delivery := rt.calibrationDeliveryFor(ctx, ownerID, computerID)
	rec.SubjectRefs[calibrationDeliveryRef] = delivery
	rec.SubjectRefs[calibrationArmRef] = calibrationArmForDelivery(delivery, rec.SubjectRefs[calibrationArmRef])
	rec.SubjectRefs[types.RuleRefStage] = types.RuleStageFirst
}

// --- the restatement request ---

// castRestateNotice is the request a committed engineering Cast's cell
// result carries on a computer in E1-M; "" off it. The record arm shows
// management's own record for the Cast's claim kinds (F3, F8).
func (rt *Runtime) castRestateNotice(ctx context.Context, ownerID, computerID, castRecordID string) string {
	if !rt.computerSettingOn(ctx, ownerID, computerID, store.SettingCalibrationE1Restate) {
		return ""
	}
	kinds := make([]string, 0, len(types.RoundPredictionsMandatory))
	for _, k := range types.RoundPredictionsMandatory {
		kinds = append(kinds, string(k))
	}
	shown := rt.calibrationArmOfCast(ctx, computerID, castRecordID) == calibration.ArmDigest
	var b strings.Builder
	fmt.Fprintf(&b, "Restate this round's predictions once, before the engineer's work shows: choir.Restate(%q, map[string]any{...}) with the Cast's predictions block, every number your own now. You may repeat or change any of them; your first statement stays on the record and both are scored on the same outcome.", castRecordID)
	if shown {
		b.WriteString("\nYour record on these claims, from your earlier Casts:\n")
		b.WriteString(rt.calibrationKindRecord(ctx, ownerID, computerID, "cast", kinds))
	}
	return b.String()
}

// engineerRestateNotice is the request after a cell committed an
// engineer's first rule Precommits on a two-stage computer; "" otherwise.
// Each kind is asked once per run.
func (rt *Runtime) engineerRestateNotice(ctx context.Context, r *rlmCallReduction) string {
	if r == nil || len(r.restateKinds) == 0 || r.rec == nil {
		return ""
	}
	if rt.calibrationDeliveryFor(ctx, r.scope.OwnerID, r.scope.ComputerID) != calibrationDeliveryTwoStage {
		return ""
	}
	state := rt.calibrationReadsOf(r.scope.RunID)
	if state == nil {
		return ""
	}
	var kinds []string
	firstP0 := ""
	state.mu.Lock()
	if state.restateAsked == nil {
		state.restateAsked = map[string]string{}
	}
	for _, ask := range r.restateKinds {
		if _, asked := state.restateAsked[ask.kind]; !asked {
			state.restateAsked[ask.kind] = ask.p0
			kinds = append(kinds, ask.kind)
			if firstP0 == "" {
				firstP0 = ask.p0
			}
		}
	}
	state.mu.Unlock()
	if len(kinds) == 0 {
		return ""
	}
	sort.Strings(kinds)
	// The arm is the one stamped on the claims (never recomputed from a
	// thread that might differ).
	threadArm := ""
	if p0, err := rt.store.GetCommitmentRecord(ctx, r.scope.OwnerID, r.scope.ComputerID, firstP0); err == nil && p0 != nil {
		threadArm = p0.SubjectRefs[calibrationArmRef]
	} else {
		log.Printf("calibration: run %s: p0 %s unreadable for the restatement request (shown blind): %v", r.scope.RunID, firstP0, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Restate each of these predictions once, before your first build: choir.Restate(\"<kind>\", map[string]any{\"p\": <p>}) for %s (a clock kind may add estimate_minutes, low_minutes and high_minutes). You may repeat or change the numbers; your first statement stays on the record and both are scored on the same outcome.", strings.Join(kinds, ", "))
	if threadArm == calibration.ArmDigest {
		b.WriteString("\nYour record on these claims, from your earlier rounds:\n")
		b.WriteString(rt.calibrationKindRecord(ctx, r.scope.OwnerID, r.scope.ComputerID, "precommit", kinds))
	}
	return b.String()
}

// calibrationKindRecord renders a stater's own per-kind record from the
// computer's snapshot (F8: settled, shown observations only).
func (rt *Runtime) calibrationKindRecord(ctx context.Context, ownerID, computerID, statedBy string, kinds []string) string {
	snap, err := rt.calibrationSnapshotFor(ctx, ownerID, computerID, calibrationRefresh)
	if err != nil {
		log.Printf("calibration: kind record on %s: %v", computerID, err)
		return "(the record could not be read)"
	}
	return calibration.KindRecord(snap.Observations, agentprofile.Engineering, statedBy, kinds, calibration.DefaultOptions.MinBin)
}

// --- the restatement ---

// restatementBody is an engineer's restatement of one kind.
type restatementBody struct {
	P               *float64 `json:"p"`
	EstimateMinutes *int     `json:"estimate_minutes,omitempty"`
	LowMinutes      *int     `json:"low_minutes,omitempty"`
	HighMinutes     *int     `json:"high_minutes,omitempty"`
}

// commitRestateIntent records one staged restatement (F1-F4).
func (r *rlmCallReduction) commitRestateIntent(ctx context.Context, in yaegikernel.StagedIntent) (uint64, error) {
	rt := r.rt()
	if rt == nil || rt.store == nil || r.ledger == nil || r.rec == nil {
		return 0, fmt.Errorf("reduce: restate %s without ledger authority", in.LocalID)
	}
	var err error
	switch strings.TrimSpace(r.rec.AgentProfile) {
	case agentprofile.Management:
		err = r.commitCastRestatement(ctx, rt, in)
	case agentprofile.Engineering:
		err = r.commitEngineerRestatement(ctx, rt, in)
	default:
		return 0, fmt.Errorf("reduce: restate %s: only management and engineering restate", in.LocalID)
	}
	if err == nil {
		if r.restated == nil {
			r.restated = map[string]bool{}
		}
		r.restated[strings.TrimSpace(in.TargetRef)] = true
	}
	return 0, err
}

// commitCastRestatement restates a Cast's predictions block (E1-M).
func (r *rlmCallReduction) commitCastRestatement(ctx context.Context, rt *Runtime, in yaegikernel.StagedIntent) error {
	owner, computer, castID := r.scope.OwnerID, r.scope.ComputerID, strings.TrimSpace(in.TargetRef)
	if err := r.refuseSecondRestateInCell(in); err != nil {
		return err
	}
	// A replayed cell skips the gate and completes its own set in
	// appendRestatements (review r2: a half-written set is finished).
	replay := r.replayedRestatement(ctx, rt, roundPredictionRecordID(castID, types.PrecommitRuleVerifyPass)+":restate")
	if !replay && !rt.computerSettingOn(ctx, owner, computer, store.SettingCalibrationE1Restate) {
		return fmt.Errorf("reduce: restate %s: no restatement is asked on this computer", in.LocalID)
	}
	if strings.HasPrefix(castID, r.scope.CellID+":") {
		return fmt.Errorf("reduce: restate %s: restate in a later cell than the Cast, after its result asked", in.LocalID)
	}
	predictions, err := types.ParseRoundPredictions(json.RawMessage(in.Body))
	if err != nil {
		return fmt.Errorf("reduce: restate %s: %w; the shape is %s", in.LocalID, err, types.RoundPredictionsHow)
	}
	var stated []types.CommitmentRecord
	for _, p1 := range predictions.Precommits() {
		p0, err := rt.store.GetCommitmentRecord(ctx, owner, computer, roundPredictionRecordID(castID, p1.Rule.Kind))
		if err != nil || p0 == nil {
			if p1.Rule.Kind == types.PrecommitRuleApprovedWithinRounds {
				continue // optional at the Cast: nothing to restate
			}
			return fmt.Errorf("reduce: restate %s: the Cast %s states no %s (F4)", in.LocalID, castID, p1.Rule.Kind)
		}
		stated = append(stated, restatementRecord(*p0, p1, r.scope, "restate"))
	}
	shown := len(stated) > 0 && stated[0].SubjectRefs[calibrationArmRef] == calibration.ArmDigest
	late := ""
	if len(stated) > 0 {
		since, ok := parseCommitmentStamp(stated[0].Precommit.Rule.Since)
		if ok && time.Since(since) > calibrationRestateWindow {
			late = fmt.Sprintf("restated more than %s after the Cast", calibrationRestateWindow)
		}
	}
	return r.appendRestatements(ctx, rt, in, stated, shown, late, false)
}

// commitEngineerRestatement restates one kind of the engineer's own first
// Precommits (E1-E two-stage).
func (r *rlmCallReduction) commitEngineerRestatement(ctx context.Context, rt *Runtime, in yaegikernel.StagedIntent) error {
	owner, computer, kind := r.scope.OwnerID, r.scope.ComputerID, strings.TrimSpace(in.TargetRef)
	if err := r.refuseSecondRestateInCell(in); err != nil {
		return err
	}
	state := rt.calibrationReadsOf(r.scope.RunID)
	p0ID := ""
	if state != nil {
		state.mu.Lock()
		p0ID = state.restateAsked[kind]
		state.mu.Unlock()
	}
	if p0ID != "" && r.replayedRestatement(ctx, rt, p0ID+":restate") {
		return nil // a replayed cell: its one record stands
	}
	if rt.calibrationDeliveryFor(ctx, owner, computer) != calibrationDeliveryTwoStage {
		return fmt.Errorf("reduce: restate %s: no restatement is asked on this computer", in.LocalID)
	}
	if p0ID == "" {
		return fmt.Errorf("reduce: restate %s: no restatement of %q was asked of this run", in.LocalID, kind)
	}
	// The ask names this run's own first statement (F4, review r1: never
	// another attempt's claim of the same kind).
	p0, err := rt.store.GetCommitmentRecord(ctx, owner, computer, p0ID)
	if err != nil || p0 == nil || p0.Precommit == nil || p0.Precommit.Rule == nil || string(p0.Precommit.Rule.Kind) != kind {
		return fmt.Errorf("reduce: restate %s: the Precommit %s it restates is not readable (F4): %v", in.LocalID, p0ID, err)
	}
	if strings.HasPrefix(p0.RecordID, r.scope.CellID+":") {
		return fmt.Errorf("reduce: restate %s: restate in a later cell than the Precommit, after its result asked", in.LocalID)
	}
	dec := json.NewDecoder(strings.NewReader(in.Body))
	dec.DisallowUnknownFields()
	var body restatementBody
	if err := dec.Decode(&body); err != nil || body.P == nil || dec.More() {
		return fmt.Errorf("reduce: restate %s: the statement is {\"p\": <p>} (a clock kind may add estimate_minutes, low_minutes, high_minutes)", in.LocalID)
	}
	p1 := *p0.Precommit
	rule := *p1.Rule
	if body.EstimateMinutes != nil || body.LowMinutes != nil || body.HighMinutes != nil {
		if !types.ClockRule(rule.Kind) || rule.Kind == types.PrecommitRuleFreezeWithin {
			return fmt.Errorf("reduce: restate %s: only freeze_minutes and approval_minutes restate minutes", in.LocalID)
		}
		if body.EstimateMinutes != nil {
			rule.EstimateMinutes = *body.EstimateMinutes
		}
		if body.LowMinutes != nil {
			rule.LowMinutes = *body.LowMinutes
		}
		if body.HighMinutes != nil {
			rule.HighMinutes = *body.HighMinutes
		}
	}
	p1.Rule = &rule
	p1.Distribution = map[string]float64{types.PrecommitAnswerYes: *body.P, types.PrecommitAnswerNo: 1 - *body.P}
	if err := types.ValidateRulePrecommit(p1); err != nil {
		return fmt.Errorf("reduce: restate %s: %w", in.LocalID, err)
	}
	shown := p0.SubjectRefs[calibrationArmRef] == calibration.ArmDigest
	late := ""
	if types.ClockRule(rule.Kind) && r.runStartedBuildOrShot() {
		late = "a clock claim restated after the run's first build or shot job"
	}
	return r.appendRestatements(ctx, rt, in, []types.CommitmentRecord{restatementRecord(*p0, p1, r.scope, "restate")}, shown, late, true)
}

// restatementRecord is p1 for p0: the same rule, assignment and clock
// anchor (so the same event settles both), the new statement, and its own
// id, stater and time.
func restatementRecord(p0 types.CommitmentRecord, p1 types.CommitmentPrecommit, scope ReductionScope, by string) types.CommitmentRecord {
	rule := *p1.Rule
	rule.AssignmentID, rule.Since = p0.Precommit.Rule.AssignmentID, p0.Precommit.Rule.Since
	p1.Rule = &rule
	if strings.TrimSpace(p1.Question) == "" {
		p1.Question = p0.Precommit.Question
	}
	p1.Resolver = types.PrecommitRuleResolver
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rec := p0
	rec.RecordID = p0.RecordID + ":restate"
	rec.Precommit = &p1
	rec.Prediction = types.CommitmentPrediction{
		Hypothesis: p1.Question,
		Questions: []types.TypedQuestion{{Question: p1.Question,
			Answers: []string{types.PrecommitAnswerYes, types.PrecommitAnswerNo}, Probabilities: p1.Distribution}},
		CommittedAt: now,
	}
	rec.Provenance = types.CommitmentProvenance{AgentID: scope.FromAgentID, ModelID: scope.ModelID, CommittedAt: now, ContextRef: scope.ChannelID}
	rec.RelatedIDs = append([]string{p0.RecordID}, p0.RelatedIDs...)
	rec.SubjectRefs = make(map[string]string, len(p0.SubjectRefs)+6)
	for k, v := range p0.SubjectRefs {
		switch k {
		case types.RuleRefLate, types.RuleRefLateReason, calibrationReadAtRef, calibrationReadsRef:
			continue
		}
		rec.SubjectRefs[k] = v
	}
	rec.SubjectRefs[types.RuleRefStage] = types.RuleStageRestated
	rec.SubjectRefs[types.RuleRefRestates] = p0.RecordID
	rec.SubjectRefs[calibrationRestateCellRef] = scope.CellID
	stampRulePrecommitStatement(&rec, by, now, "")
	return rec
}

// appendRestatements writes the p1 records once each (F4), marking them
// late when the candidate froze or late says so (F2), and refusing them when a
// p0 already settled.
// A replayed cell finds its own records and passes.
func (r *rlmCallReduction) appendRestatements(ctx context.Context, rt *Runtime, in yaegikernel.StagedIntent, records []types.CommitmentRecord, shown bool, late string, engineer bool) error {
	owner, computer := r.scope.OwnerID, r.scope.ComputerID
	if len(records) == 0 {
		return fmt.Errorf("reduce: restate %s: nothing to restate", in.LocalID)
	}
	assignmentID := records[0].Precommit.Rule.AssignmentID
	if late == "" && rt.selfdevOperations != nil {
		op, _, found, err := assignmentOperation(ctx, rt.selfdevOperations, computer, assignmentID)
		if err != nil {
			return fmt.Errorf("reduce: restate %s: %w", in.LocalID, err)
		}
		if found && strings.TrimSpace(op.BundleDigest) != "" {
			late = "restated after the candidate froze"
		}
	}
	// A prediction is restated once (F4). A set some of whose records an
	// earlier cell wrote before failing is completed, not refused for ever;
	// a set wholly written is refused.
	// A record this same cell wrote is a replay's and stands.
	missing := make([]bool, len(records))
	anyMissing, ownWritten := false, false
	for i := range records {
		existing, err := rt.store.GetCommitmentRecord(ctx, owner, computer, records[i].RecordID)
		if err != nil {
			return fmt.Errorf("reduce: restate %s: %w", in.LocalID, err)
		}
		exists := existing != nil
		missing[i] = !exists
		anyMissing = anyMissing || !exists
		ownWritten = ownWritten || (exists && existing.SubjectRefs[calibrationRestateCellRef] == r.scope.CellID)
	}
	if !anyMissing && ownWritten {
		return nil // a replayed cell whose set is whole
	}
	if !anyMissing {
		return fmt.Errorf("reduce: restate %s: %s was already restated; a prediction is restated once (F4)", in.LocalID, records[0].SubjectRefs[types.RuleRefRestates])
	}
	// A first statement already settled leaves nothing to restate: its p1
	// would never settle (a stale Cast id, review r3), so it is refused.
	for i := range records {
		if !missing[i] {
			continue
		}
		p0 := records[i].SubjectRefs[types.RuleRefRestates]
		settled, err := rt.store.CommitmentRecordExists(ctx, owner, computer, types.RuleResolveRecordID(p0))
		if err != nil {
			return fmt.Errorf("reduce: restate %s: %w", in.LocalID, err)
		}
		if settled {
			return fmt.Errorf("reduce: restate %s: %s has already settled; restate only the current round's predictions, before its outcome is known (F2)", in.LocalID, p0)
		}
	}
	for i := range records {
		rec := &records[i]
		if !missing[i] {
			continue
		}
		reason := late
		if reason != "" {
			rec.SubjectRefs[types.RuleRefLate] = "true"
			rec.SubjectRefs[types.RuleRefLateReason] = reason
		}
		rec.SubjectRefs[calibrationRecordShownRef] = fmt.Sprintf("%t", shown)
		if engineer {
			rec.SubjectRefs[calibrationDeliveryRef] = calibrationDeliveryTwoStage
		}
		if _, err := r.ledger.AppendCommitmentRecord(ctx, owner, computer, *rec); err != nil {
			return fmt.Errorf("reduce: restate %s: %s: %w", in.LocalID, rec.RecordID, err)
		}
	}
	return nil
}

// refuseSecondRestateInCell refuses a second Restate of one target in a
// cell (F4); a replayed cell is told apart by its records, not by this.
func (r *rlmCallReduction) refuseSecondRestateInCell(in yaegikernel.StagedIntent) error {
	target := strings.TrimSpace(in.TargetRef)
	if r.restated[target] {
		return fmt.Errorf("reduce: restate %s: %s is restated once, and this cell already restated it (F4)", in.LocalID, target)
	}
	return nil
}

// replayedRestatement reports a p1 this same cell already wrote: a replay
// passes before any gate, so clearing an arm mid-flight never fails a cell
// that committed (E1 review r1).
func (r *rlmCallReduction) replayedRestatement(ctx context.Context, rt *Runtime, p1ID string) bool {
	existing, err := rt.store.GetCommitmentRecord(ctx, r.scope.OwnerID, r.scope.ComputerID, p1ID)
	return err == nil && existing != nil && existing.SubjectRefs[calibrationRestateCellRef] == r.scope.CellID
}
