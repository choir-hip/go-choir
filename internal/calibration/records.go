package calibration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Claim status in views and list filters.
const (
	StatusOpen    = "open"
	StatusOverdue = "overdue"
	StatusSettled = "settled"
)

const (
	maxNameLen     = 200
	maxClaimLen    = 4000
	maxTextLen     = 2000
	maxTags        = 20
	maxTagLen      = 64
	maxEvidence    = 20
	claimResolver  = "multi-scorer"
	agentIDPrefix  = "agent:"
	ruleIDPrefix   = "rule:"
	ownerScorerTag = "owner"
)

// AgentIdentity names who makes a claim: the agent, its model and the
// session the claim was made in.
type AgentIdentity struct {
	Name      string `json:"name"`
	Model     string `json:"model"`
	SessionID string `json:"session_id,omitempty"`
}

// ClaimRequest is the body of POST /api/precommit/claims. P is the stated
// probability that the claim is true. StatedAt and Reconstructed are for
// claims recorded after the fact (the level-1 import); the record's commit
// time is always the server's.
type ClaimRequest struct {
	Agent          AgentIdentity `json:"agent"`
	Claim          string        `json:"claim"`
	Type           string        `json:"type,omitempty"`
	P              *float64      `json:"p"`
	Criterion      string        `json:"criterion,omitempty"`
	Deadline       string        `json:"deadline,omitempty"`
	Project        string        `json:"project,omitempty"`
	Tags           []string      `json:"tags,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
	StatedAt       string        `json:"stated_at,omitempty"`
	Reconstructed  bool          `json:"reconstructed,omitempty"`
	SourceRef      string        `json:"source_ref,omitempty"`
}

// ScorerIdentity names who settles: the method (self, independent, rule,
// owner), the scorer's name and its model.
type ScorerIdentity struct {
	Method string `json:"method,omitempty"`
	Name   string `json:"name,omitempty"`
	Model  string `json:"model,omitempty"`
}

// SettlementRequest is the body of POST /api/precommit/claims/{id}/settlements.
type SettlementRequest struct {
	Outcome  string         `json:"outcome"`
	Evidence []string       `json:"evidence"`
	Scorer   ScorerIdentity `json:"scorer,omitempty"`
	Note     string         `json:"note,omitempty"`
}

// SettlementView is one scorer's settlement as the API returns it.
type SettlementView struct {
	ID          string   `json:"id"`
	Outcome     string   `json:"outcome"`
	Method      string   `json:"method"`
	Scorer      string   `json:"scorer"`
	ScorerModel string   `json:"scorer_model,omitempty"`
	Brier       *float64 `json:"brier,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
	Note        string   `json:"note,omitempty"`
	SettledAt   string   `json:"settled_at"`
}

// ClaimView is a claim joined with its settlements. Outcome, Brier and
// GoverningMethod come from the governing settlement (rule > owner >
// independent > self); Disagreement is set when settlements differ.
type ClaimView struct {
	ID              string           `json:"id"`
	Agent           string           `json:"agent"`
	Model           string           `json:"model"`
	SessionID       string           `json:"session_id,omitempty"`
	Claim           string           `json:"claim"`
	Type            string           `json:"type"`
	P               float64          `json:"p"`
	Criterion       string           `json:"criterion,omitempty"`
	Deadline        string           `json:"deadline,omitempty"`
	Project         string           `json:"project,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	RecordedAt      string           `json:"recorded_at"`
	StatedAt        string           `json:"stated_at,omitempty"`
	Reconstructed   bool             `json:"reconstructed,omitempty"`
	SourceRef       string           `json:"source_ref,omitempty"`
	Status          string           `json:"status"`
	Outcome         string           `json:"outcome,omitempty"`
	GoverningMethod string           `json:"governing_method,omitempty"`
	Brier           *float64         `json:"brier,omitempty"`
	Disagreement    bool             `json:"disagreement,omitempty"`
	Settlements     []SettlementView `json:"settlements"`
}

// ValidationError is a refused request; the handler maps it to 400.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

func cleanText(s string, max int, field string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) > max {
		return "", invalid("%s is longer than %d bytes", field, max)
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", invalid("%s contains a control character", field)
		}
	}
	return s, nil
}

func cleanName(s, field string) (string, error) {
	s, err := cleanText(s, maxNameLen, field)
	if err == nil && strings.ContainsAny(s, "\n\t") {
		return "", invalid("%s must be one line", field)
	}
	return s, err
}

func parseStamp(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return "", invalid("%s must be RFC 3339 (for example 2026-10-15T18:00:00Z)", field)
	}
	return t.UTC().Format(time.RFC3339), nil
}

// Slug makes a name safe inside a record id: lower case, [a-z0-9._-],
// anything else becomes '-'.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			return r
		}
		return '-'
	}, s)
	out = strings.Trim(out, "-")
	if out == "" {
		return "anon"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// ClaimRecordID derives the claim id. With an idempotency key it is
// deterministic per agent; without one the caller supplies random hex.
func ClaimRecordID(agent, idempotencyKey, randomHex string) string {
	suffix := randomHex
	if key := strings.TrimSpace(idempotencyKey); key != "" {
		sum := sha256.Sum256([]byte(strings.TrimSpace(agent) + "\x1f" + key))
		suffix = hex.EncodeToString(sum[:8])
	}
	return "calib:" + Slug(agent) + ":" + suffix
}

// NewClaimRecord validates a claim request and builds its precommit record.
// now is the server's commit time.
func NewClaimRecord(req ClaimRequest, now time.Time, randomHex string) (types.CommitmentRecord, error) {
	var rec types.CommitmentRecord
	agent, err := cleanName(req.Agent.Name, "agent.name")
	if err != nil {
		return rec, err
	}
	model, err := cleanName(req.Agent.Model, "agent.model")
	if err != nil {
		return rec, err
	}
	session, err := cleanName(req.Agent.SessionID, "agent.session_id")
	if err != nil {
		return rec, err
	}
	if agent == "" || model == "" {
		return rec, invalid("agent.name and agent.model are required (calibration is per agent and model)")
	}
	text, err := cleanText(req.Claim, maxClaimLen, "claim")
	if err != nil {
		return rec, err
	}
	if text == "" {
		return rec, invalid("claim is required")
	}
	claimType := strings.TrimSpace(req.Type)
	if claimType == "" {
		claimType = types.CalibrationClaimAssertion
	}
	switch claimType {
	case types.CalibrationClaimAssertion, types.CalibrationClaimForecast, types.CalibrationClaimCommitment:
	default:
		return rec, invalid("type must be assertion, forecast or commitment")
	}
	if req.P == nil {
		return rec, invalid("p is required: the probability that the claim is true")
	}
	p := *req.P
	if math.IsNaN(p) || p < 0 || p > 1 {
		return rec, invalid("p must be between 0 and 1")
	}
	criterion, err := cleanText(req.Criterion, maxTextLen, "criterion")
	if err != nil {
		return rec, err
	}
	project, err := cleanName(req.Project, "project")
	if err != nil {
		return rec, err
	}
	sourceRef, err := cleanText(req.SourceRef, maxTextLen, "source_ref")
	if err != nil {
		return rec, err
	}
	deadline, err := parseStamp(req.Deadline, "deadline")
	if err != nil {
		return rec, err
	}
	statedAt, err := parseStamp(req.StatedAt, "stated_at")
	if err != nil {
		return rec, err
	}
	if statedAt != "" && !req.Reconstructed {
		return rec, invalid("stated_at is only for reconstructed claims; a live claim's time is the server's")
	}
	var tags []string
	for _, tag := range req.Tags {
		tag, err := cleanName(tag, "tag")
		if err != nil {
			return rec, err
		}
		if tag == "" {
			continue
		}
		if len(tag) > maxTagLen {
			return rec, invalid("a tag is longer than %d bytes", maxTagLen)
		}
		tags = append(tags, tag)
	}
	if len(tags) > maxTags {
		return rec, invalid("at most %d tags", maxTags)
	}
	key, err := cleanName(req.IdempotencyKey, "idempotency_key")
	if err != nil {
		return rec, err
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	dist := map[string]float64{types.CalibrationOutcomeTrue: p, types.CalibrationOutcomeFalse: 1 - p}
	rec = types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1,
		RecordID: ClaimRecordID(agent, key, randomHex),
		Kind:     types.CommitmentKindPrecommit,
		Prediction: types.CommitmentPrediction{
			Hypothesis: text,
			Questions: []types.TypedQuestion{{
				Question:      text,
				Answers:       []string{types.CalibrationOutcomeTrue, types.CalibrationOutcomeFalse},
				Probabilities: dist,
			}},
			CommittedAt: stamp,
		},
		Precommit:   &types.CommitmentPrecommit{Question: text, Distribution: dist, Resolver: claimResolver},
		Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: agentIDPrefix + agent, ModelID: model, ContextRef: session, CommittedAt: stamp},
		EmitterKind: types.CommitmentEmitterAgent,
		Calibration: &types.CommitmentCalibration{
			Schema: types.CalibrationSchemaV1,
			Agent:  agent,
			Claim: &types.CalibrationClaim{
				Text: text, Type: claimType, Probability: p, Criterion: criterion, Deadline: deadline,
				Project: project, Tags: tags, SessionID: session, StatedAt: statedAt,
				Reconstructed: req.Reconstructed, SourceRef: sourceRef,
			},
		},
	}
	return rec, nil
}

// IsClaim reports whether a record is a calibration claim.
func IsClaim(rec *types.CommitmentRecord) bool {
	return rec != nil && rec.Calibration != nil && rec.Calibration.Schema == types.CalibrationSchemaV1 && rec.Calibration.Claim != nil
}

// SameClaim reports whether two claim records say the same thing; the
// server's commit time is not compared, so a replay matches.
func SameClaim(a, b types.CommitmentRecord) bool {
	if !IsClaim(&a) || !IsClaim(&b) {
		return false
	}
	return a.Calibration.Agent == b.Calibration.Agent && a.Provenance.ModelID == b.Provenance.ModelID &&
		reflect.DeepEqual(*a.Calibration.Claim, *b.Calibration.Claim)
}

// SettlementRecordID is one settlement per scorer per claim.
func SettlementRecordID(claimID, method, scorer string) string {
	return claimID + ":settle:" + method + ":" + Slug(scorer)
}

// NewSettlementRecord validates a settlement against its claim and builds
// the resolve record, with the Brier score computed.
func NewSettlementRecord(claim types.CommitmentRecord, req SettlementRequest, now time.Time) (types.CommitmentRecord, error) {
	var rec types.CommitmentRecord
	if !IsClaim(&claim) {
		return rec, invalid("not a calibration claim")
	}
	claimant := claim.Calibration.Agent
	outcome := strings.TrimSpace(req.Outcome)
	y, resolved, ok := OutcomeValue(outcome)
	if !ok {
		return rec, invalid("outcome must be true, false, partial or unresolvable")
	}
	var evidence []string
	for _, ref := range req.Evidence {
		ref, err := cleanText(ref, maxTextLen, "evidence")
		if err != nil {
			return rec, err
		}
		if ref != "" {
			evidence = append(evidence, ref)
		}
	}
	if len(evidence) == 0 {
		return rec, invalid("a settlement cites at least one evidence ref (commit, CI run, staging check, trace, problem doc)")
	}
	if len(evidence) > maxEvidence {
		return rec, invalid("at most %d evidence refs", maxEvidence)
	}
	note, err := cleanText(req.Note, maxTextLen, "note")
	if err != nil {
		return rec, err
	}
	name, err := cleanName(req.Scorer.Name, "scorer.name")
	if err != nil {
		return rec, err
	}
	model, err := cleanName(req.Scorer.Model, "scorer.model")
	if err != nil {
		return rec, err
	}
	method := strings.TrimSpace(req.Scorer.Method)
	if method == "" {
		method = types.CalibrationScorerSelf
		if name != "" && name != claimant {
			method = types.CalibrationScorerIndependent
		}
	}
	agentID := agentIDPrefix + name
	emitter := types.CommitmentEmitterAgent
	switch method {
	case types.CalibrationScorerSelf:
		if name == "" {
			name = claimant
		}
		if name != claimant {
			return rec, invalid("a self settlement is written by the claimant (%s)", claimant)
		}
		if model == "" {
			model = claim.Provenance.ModelID
		}
		agentID = agentIDPrefix + name
	case types.CalibrationScorerIndependent:
		if name == "" || name == claimant {
			return rec, invalid("an independent settlement names a scorer other than the claimant")
		}
	case types.CalibrationScorerRule:
		if name == "" {
			return rec, invalid("a rule settlement names the rule (scorer.name)")
		}
		agentID = ruleIDPrefix + name
	case types.CalibrationScorerOwner:
		if name == "" {
			name = ownerScorerTag
		}
		agentID = ownerScorerTag
		emitter = types.CommitmentEmitterOwner
	default:
		return rec, invalid("scorer.method must be self, independent, rule or owner")
	}
	p := claim.Calibration.Claim.Probability
	var brier *float64
	if resolved {
		b := BrierScore(p, y)
		brier = &b
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	excerpt := outcome
	if note != "" {
		excerpt = outcome + ": " + note
	}
	rec = types.CommitmentRecord{
		SchemaID:     types.CommitmentRecordSchemaV1,
		RecordID:     SettlementRecordID(claim.RecordID, method, name),
		Kind:         types.CommitmentKindResolve,
		Resolve:      &types.CommitmentResolve{Verdict: outcome, EvidenceRefs: evidence},
		ParentID:     claim.RecordID,
		RelatedIDs:   []string{claim.RecordID},
		EvidenceRefs: evidence,
		Observation:  types.CommitmentObservation{Excerpt: excerpt, SourceRef: evidence[0], ObservedAt: stamp},
		Discrepancy:  discrepancyFor(outcome),
		Provenance:   types.CommitmentProvenance{AgentID: agentID, ModelID: model, CommittedAt: stamp, ResolvedAt: stamp},
		EmitterKind:  emitter,
		Calibration: &types.CommitmentCalibration{
			Schema: types.CalibrationSchemaV1,
			Agent:  claimant,
			Settlement: &types.CalibrationSettlement{
				ClaimID: claim.RecordID, Outcome: outcome, ScorerMethod: method, ScorerName: name,
				ScorerModel: model, ClaimProbability: p, Brier: brier, Note: note,
			},
		},
	}
	return rec, nil
}

// SameSettlement reports whether two settlement records say the same thing.
func SameSettlement(a, b types.CommitmentRecord) bool {
	if a.Calibration == nil || b.Calibration == nil || a.Calibration.Settlement == nil || b.Calibration.Settlement == nil {
		return false
	}
	return reflect.DeepEqual(*a.Calibration.Settlement, *b.Calibration.Settlement) &&
		reflect.DeepEqual(a.EvidenceRefs, b.EvidenceRefs)
}

func discrepancyFor(outcome string) types.DiscrepancyClass {
	switch outcome {
	case types.CalibrationOutcomeTrue:
		return types.DiscrepancyConfirmed
	case types.CalibrationOutcomeFalse:
		return types.DiscrepancyContradicted
	case types.CalibrationOutcomePartial:
		return types.DiscrepancyQualified
	}
	return types.DiscrepancyUnresolved
}

func settlementView(rec types.CommitmentRecord) SettlementView {
	s := rec.Calibration.Settlement
	return SettlementView{
		ID: rec.RecordID, Outcome: s.Outcome, Method: s.ScorerMethod, Scorer: s.ScorerName, ScorerModel: s.ScorerModel,
		Brier: s.Brier, Evidence: rec.EvidenceRefs, Note: s.Note, SettledAt: rec.Provenance.ResolvedAt,
	}
}

// JoinClaims joins calibration records into claim views, newest claim
// first. Settlements whose claim is not in records are dropped.
func JoinClaims(records []types.CommitmentRecord, now time.Time) []ClaimView {
	settlements := map[string][]SettlementView{}
	var claims []types.CommitmentRecord
	for _, rec := range records {
		if rec.Calibration == nil || rec.Calibration.Schema != types.CalibrationSchemaV1 {
			continue
		}
		switch {
		case rec.Calibration.Claim != nil:
			claims = append(claims, rec)
		case rec.Calibration.Settlement != nil:
			id := rec.Calibration.Settlement.ClaimID
			settlements[id] = append(settlements[id], settlementView(rec))
		}
	}
	out := make([]ClaimView, 0, len(claims))
	for _, rec := range claims {
		out = append(out, claimView(rec, settlements[rec.RecordID], now))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RecordedAt != out[j].RecordedAt {
			return out[i].RecordedAt > out[j].RecordedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func claimView(rec types.CommitmentRecord, settlements []SettlementView, now time.Time) ClaimView {
	c := rec.Calibration.Claim
	sort.SliceStable(settlements, func(i, j int) bool {
		if settlements[i].SettledAt != settlements[j].SettledAt {
			return settlements[i].SettledAt < settlements[j].SettledAt
		}
		return settlements[i].ID < settlements[j].ID
	})
	if settlements == nil {
		settlements = []SettlementView{}
	}
	v := ClaimView{
		ID: rec.RecordID, Agent: rec.Calibration.Agent, Model: rec.Provenance.ModelID, SessionID: c.SessionID,
		Claim: c.Text, Type: c.Type, P: c.Probability, Criterion: c.Criterion, Deadline: c.Deadline,
		Project: c.Project, Tags: c.Tags, RecordedAt: rec.Provenance.CommittedAt, StatedAt: c.StatedAt,
		Reconstructed: c.Reconstructed, SourceRef: c.SourceRef, Settlements: settlements,
	}
	v.Status = statusOf(v, "", now)
	if g, ok := Governing(settlements, ""); ok {
		v.Outcome, v.GoverningMethod, v.Brier = g.Outcome, g.Method, g.Brier
	}
	seen := map[string]bool{}
	for _, s := range settlements {
		seen[s.Outcome] = true
	}
	v.Disagreement = len(seen) > 1
	return v
}

// statusOf is settled when the claim has a settlement under the scorer
// selection ("" = any), overdue when open past its deadline, else open.
func statusOf(v ClaimView, scorer string, now time.Time) string {
	if _, ok := Governing(v.Settlements, scorer); ok {
		return StatusSettled
	}
	if v.Deadline != "" {
		if d, err := time.Parse(time.RFC3339, v.Deadline); err == nil && now.After(d) {
			return StatusOverdue
		}
	}
	return StatusOpen
}
