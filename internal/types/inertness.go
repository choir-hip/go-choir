package types

import (
	"crypto/sha256"
	"math"
	"regexp"
	"sort"
	"strings"
)

// Inertness detection (modular architecture MA1.6, plan section 4.4, R29).
//
// A precommitment is inert when the prediction is true of almost any outcome,
// sits at the base rate, or has no falsifier or no named decision dependency:
// the fault is in the prediction. A delegation is inert when its weight is
// below the floor its callee's grants set: the fault is in the fan-out. The
// detector is a pure function of a record's own frozen content; the reducer
// runs it at commit and stores the result in CommitmentRecord.InertFlags. It
// never refuses anything (the record is the evidence), and a cell can neither
// set nor clear a flag: the reducer overwrites the field from the record's
// content, and a cell has no field to put one in.

// Inertness flags. InertWeightBelowFloor and InertNoFalsifier were set by the
// sub-call writer (MA1.5); the others are MA1.6's.
const (
	InertBaseRate        = "p_at_base_rate"
	InertNearCertain     = "near_certain_answer_set"
	InertNoDependency    = "no_decision_dependency"
	InertTrivialTemplate = "trivial_template"
)

// InertPredictionFlags are the flags whose fault is in the prediction (an
// inert precommitment). InertWeightBelowFloor alone is an inert delegation.
var InertPredictionFlags = []string{InertBaseRate, InertNearCertain, InertNoFalsifier, InertNoDependency, InertTrivialTemplate}

// InertFlagOrder is the order flags are stored in.
var InertFlagOrder = []string{InertBaseRate, InertNearCertain, InertNoFalsifier, InertNoDependency, InertTrivialTemplate, InertWeightBelowFloor}

// Thresholds. They are parameters of the Phase 1 ledger round (plan 10.2):
// chosen conservative so that a flag is rarely a false positive, and
// retuned from the measured baseline.
const (
	// InertBaseRateEpsilon is how close p_success may be to the class base
	// rate before it is "at the base rate".
	InertBaseRateEpsilon = 0.03
	// InertNearCertainCeiling is the confidence at or above which a
	// prediction with no named risk is "near-certain by construction".
	InertNearCertainCeiling = 0.98
	// InertDigestWindow is how many of a desk's latest precommitments the
	// digest's "N of your last M" counts; InertDigestMinM is the fewest M it
	// is shown with.
	InertDigestWindow = 20
	InertDigestMinM   = 5
	// InertJudgeRateUnflagged is the share of records the rules did not flag
	// that the blind judge is asked about; InertJudgeRateFlagged is the
	// smaller control share of flagged ones, so the judge's agreement with
	// the rules is measured both ways (score the scorer).
	InertJudgeRateUnflagged = 0.10
	InertJudgeRateFlagged   = 0.05
)

// DefaultInertBaseRates are the class base rates the base-rate rule uses,
// keyed by "<kind>" or "<kind>/<callee role>". They are measured, not
// guessed: the table is empty until the Phase 1 baseline round commits the
// measured share of requests of each class that achieved their outcome. With
// no entry the rule does not fire, so a calibrated 0.5 is never called base
// rate by default.
var DefaultInertBaseRates = map[string]float64{}

// InertTrivialTemplates are the known-trivial prediction forms: a prediction
// that only says the callee returns something. The list is data, grown from
// the traces. Each pattern is matched against the whole normalized text
// (lower case, punctuation stripped), so a prediction that says more than
// the template is not trivial.
var InertTrivialTemplates = []*regexp.Regexp{
	regexp.MustCompile(`^(will )?(the |this )?(child|agent|task|call|request|tool|desk|run|subagent)( will| should)? (return|returns|respond|responds|reply|replies|complete|completes|finish|finishes|succeed|succeeds|work|works|report|reports)( with)?( a| an| the| some| any)?( result| results| response| answer| reply| report| output| findings?)?$`),
	regexp.MustCompile(`^(it|this) (will )?(work|succeed|complete|return a result|return results)$`),
}

var nonWord = regexp.MustCompile(`[^a-z0-9 ]+`)

func normalizeTemplateText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonWord.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// IsTrivialTemplate reports whether text is a known-trivial prediction form.
func IsTrivialTemplate(text string) bool {
	n := normalizeTemplateText(text)
	if n == "" {
		return false
	}
	for _, re := range InertTrivialTemplates {
		if re.MatchString(n) {
			return true
		}
	}
	return false
}

// InertKind is the request kind of a record that carries a stated
// prediction, or "" for a record that is not a request (a Report, a
// settlement, a notice, a rule-settled engineering precommit, a record from
// a system emitter). Only these records are evaluated for inertness.
func InertKind(rec CommitmentRecord) string {
	if rec.EmitterKind == CommitmentEmitterSystem || strings.TrimSpace(rec.Provenance.Origin) == OriginSystem {
		return ""
	}
	if rec.Precommit != nil && rec.Precommit.Rule != nil {
		return ""
	}
	kind := requestKindOfRecord(rec)
	if kind == "" && rec.SubCall != nil && rec.SubCall.Kind == SubCallCast {
		kind = RequestCast
	}
	return kind
}

// InertClass is the class a record's base rate is keyed by.
func InertClass(kind, role string) string {
	if role != "" {
		return kind + "/" + role
	}
	return kind
}

// inertRole is the callee role of a request: the sub-call's role, else the
// desk its addressee names.
func inertRole(rec CommitmentRecord) string {
	if rec.SubCall != nil && rec.SubCall.Role != "" {
		return rec.SubCall.Role
	}
	a := strings.TrimSpace(rec.Addressee)
	if a == "" || a == SubCallLedgerAddressee {
		return ""
	}
	role, _, _ := strings.Cut(a, ":")
	return role
}

// namedRisk reports whether the record names a way the prediction could fail:
// assumptions, alternatives, or a Cast hypothesis' crux.
func namedRisk(rec CommitmentRecord) bool {
	if len(rec.Prediction.Assumptions) > 0 || len(rec.Prediction.Alternatives) > 0 {
		return true
	}
	if rec.Cast != nil {
		for _, h := range rec.Cast.Hypotheses {
			if strings.TrimSpace(h.Crux) != "" {
				return true
			}
		}
	}
	return false
}

func hasFalsifier(rec CommitmentRecord, kind string) bool {
	switch kind {
	case RequestCast:
		if rec.Cast != nil {
			for _, h := range rec.Cast.Hypotheses {
				if strings.TrimSpace(h.Criterion) != "" {
					return true
				}
			}
		}
		for _, q := range rec.Prediction.Questions {
			if len(q.Answers) >= 2 || len(q.Probabilities) >= 2 {
				return true
			}
		}
		return false
	case RequestPrecommit:
		if rec.Precommit != nil && len(rec.Precommit.Distribution) >= 2 {
			return true
		}
		for _, q := range rec.Prediction.Questions {
			if len(q.Answers) >= 2 || len(q.Probabilities) >= 2 {
				return true
			}
		}
		return false
	}
	// An ask, escalate, feed or decline is settled by the recipient's reply
	// or the catalogue's procedure; there is no criterion to be missing.
	return true
}

func hasDependency(rec CommitmentRecord, kind string) bool {
	switch kind {
	case RequestFeed, RequestDecline:
		return true // addressed to captures' source or story by construction
	}
	if strings.TrimSpace(rec.ParentID) != "" || len(rec.RelatedIDs) > 0 {
		return true
	}
	if rec.Settlement != "" && rec.Settlement != SettlementRecipient {
		return true // a rule settles it: it names what it is checked against
	}
	if rec.SubCall != nil && strings.TrimSpace(rec.SubCall.ParentRunID) != "" {
		return true
	}
	if rec.Precommit != nil && strings.TrimSpace(rec.Precommit.Resolver) != "" {
		return true
	}
	a := strings.TrimSpace(rec.Addressee)
	return a != "" && a != SubCallLedgerAddressee
}

// nearCertain reports a prediction true by construction: a one-answer
// question, or an answer at or above the ceiling with no named risk.
func nearCertain(rec CommitmentRecord) bool {
	risk := namedRisk(rec)
	for _, q := range rec.Prediction.Questions {
		if len(q.Answers) == 1 {
			return true
		}
		if _, p, ok := TopAnswer(q); ok && p >= InertNearCertainCeiling && !risk {
			return true
		}
	}
	if rec.Precommit != nil && len(rec.Precommit.Distribution) > 0 && !risk {
		top := 0.0
		for _, p := range rec.Precommit.Distribution {
			top = math.Max(top, p)
		}
		if len(rec.Precommit.Distribution) == 1 || top >= InertNearCertainCeiling {
			return true
		}
	}
	if rec.PSuccess != nil && !risk {
		if math.Max(*rec.PSuccess, 1-*rec.PSuccess) >= InertNearCertainCeiling {
			return true
		}
	}
	return false
}

// predictionTexts are the texts a trivial-template match reads: the
// hypothesis, a Cast's statements and each question.
func predictionTexts(rec CommitmentRecord) []string {
	out := []string{rec.Prediction.Hypothesis}
	if rec.Precommit != nil {
		out = append(out, rec.Precommit.Question)
	}
	if rec.Cast != nil {
		for _, h := range rec.Cast.Hypotheses {
			out = append(out, h.Statement)
		}
	}
	for _, q := range rec.Prediction.Questions {
		out = append(out, q.Question)
	}
	return out
}

// DetectInertness runs the rules on one request record and returns its flags
// in InertFlagOrder, or nil. role is the callee role whose grants set the
// weight floor ("" takes it from the record). baseRates is the class base
// rate table (nil takes DefaultInertBaseRates). A record that is not a
// request is never flagged. Nothing here refuses anything.
func DetectInertness(role string, rec CommitmentRecord, baseRates map[string]float64) []string {
	kind := InertKind(rec)
	if rec.EmitterKind == CommitmentEmitterSystem || (rec.Precommit != nil && rec.Precommit.Rule != nil) {
		return nil
	}
	if kind == "" && rec.SubCall == nil && rec.Weight == nil && rec.PSuccess == nil {
		return nil
	}
	if role == "" {
		role = inertRole(rec)
	}
	if baseRates == nil {
		baseRates = DefaultInertBaseRates
	}
	set := map[string]bool{}
	if rec.Weight != nil && *rec.Weight < SubCallWeightFloor(role) {
		set[InertWeightBelowFloor] = true
	}
	if kind == "" {
		// A bare sub-call record with no kind yet: the MA1.5 writer's two
		// cheap flags only.
		if !rec.IsTyped() {
			set[InertNoFalsifier] = true
		}
	} else {
		if !hasFalsifier(rec, kind) {
			set[InertNoFalsifier] = true
		}
		if !hasDependency(rec, kind) {
			set[InertNoDependency] = true
		}
		if rec.PSuccess != nil {
			rate, ok := baseRates[InertClass(kind, role)]
			if !ok {
				rate, ok = baseRates[kind]
			}
			if ok && math.Abs(*rec.PSuccess-rate) <= InertBaseRateEpsilon {
				set[InertBaseRate] = true
			}
		}
		if nearCertain(rec) {
			set[InertNearCertain] = true
		}
		for _, t := range predictionTexts(rec) {
			if IsTrivialTemplate(t) {
				set[InertTrivialTemplate] = true
				break
			}
		}
	}
	var out []string
	for _, f := range InertFlagOrder {
		if set[f] {
			out = append(out, f)
		}
	}
	return out
}

// HasPredictionFault reports an inert precommitment: a flag whose fault is
// in the prediction.
func HasPredictionFault(flags []string) bool {
	for _, f := range flags {
		for _, p := range InertPredictionFlags {
			if f == p {
				return true
			}
		}
	}
	return false
}

// HasFlag reports whether flags holds want.
func HasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

// InertJudgeSampled reports whether the blind judge is asked about a record:
// a hash of the record id, so a replay samples the same ones. Unflagged
// records are sampled at InertJudgeRateUnflagged, flagged ones at the lower
// control rate InertJudgeRateFlagged.
func InertJudgeSampled(recordID string, flagged bool) bool {
	rate := InertJudgeRateUnflagged
	if flagged {
		rate = InertJudgeRateFlagged
	}
	sum := sha256.Sum256([]byte("inert-judge:" + recordID))
	u := float64(uint32(sum[0])<<16|uint32(sum[1])<<8|uint32(sum[2])) / float64(1<<24)
	return u < rate
}

// InertDesk is the desk a record's rates and digest count are for: the
// reducer-stamped desk, else the agent id, before any ":".
func InertDesk(rec CommitmentRecord) string {
	id := firstNonEmptyString(rec.Provenance.Desk, rec.Provenance.AgentID)
	desk, _, _ := strings.Cut(strings.TrimSpace(id), ":")
	return desk
}

// InertCounts is each desk's "N of your last M precommitments were
// uninformative": over the desk's M most recent evaluated requests (M at most
// window), the number with a prediction fault. A desk with fewer than
// InertDigestMinM is left out. The flags are recomputed from the record's own
// content, so records written before the slice count too.
func InertCounts(records []CommitmentRecord, window int, baseRates map[string]float64) map[string]CalibrationInert {
	if window <= 0 {
		window = InertDigestWindow
	}
	byDesk := map[string][]CommitmentRecord{}
	for _, rec := range records {
		if InertKind(rec) == "" || !statesPrediction(rec) {
			continue
		}
		if d := InertDesk(rec); d != "" {
			byDesk[d] = append(byDesk[d], rec)
		}
	}
	out := map[string]CalibrationInert{}
	for desk, recs := range byDesk {
		sort.SliceStable(recs, func(i, j int) bool {
			if recs[i].Provenance.CommittedAt != recs[j].Provenance.CommittedAt {
				return recs[i].Provenance.CommittedAt > recs[j].Provenance.CommittedAt
			}
			return recs[i].RecordID < recs[j].RecordID
		})
		if len(recs) > window {
			recs = recs[:window]
		}
		if len(recs) < InertDigestMinM {
			continue
		}
		c := CalibrationInert{M: len(recs)}
		for _, rec := range recs {
			if HasPredictionFault(DetectInertness("", rec, baseRates)) {
				c.N++
			}
		}
		out[desk] = c
	}
	return out
}

// statesPrediction reports a request that states a probability or a
// distribution: the records a precommitment count is over. A weight-zero
// sub-call Cast states none (the parent invented nothing) and is not counted.
func statesPrediction(rec CommitmentRecord) bool {
	if rec.PSuccess != nil || len(rec.Prediction.Questions) > 0 {
		return true
	}
	return rec.Precommit != nil && len(rec.Precommit.Distribution) > 0
}
