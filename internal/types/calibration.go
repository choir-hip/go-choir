package types

// Agent calibration records (calibration level 2,
// docs/archive/reports/choir-mcp-precommitments-2026-10-14.md). An agent outside the
// computer's desks (Claude Code, any MCP client) writes a claim with a
// probability, and scorers append settlements. Both are
// choir.commitment_record objects with existing record kinds: the claim is a
// precommit, each settlement a resolve linked to it by ParentID. This body
// carries what the generic record has no field for.

// CalibrationSchemaV1 marks every calibration body; the store lists
// calibration records by it.
const CalibrationSchemaV1 = "choir.calibration.v1"

// Claim types.
const (
	CalibrationClaimAssertion  = "assertion"
	CalibrationClaimForecast   = "forecast"
	CalibrationClaimCommitment = "commitment"
)

// Settlement outcomes. Partial scores against 0.5; unresolvable has no
// Brier score (strict Brier counts it as false).
const (
	CalibrationOutcomeTrue         = "true"
	CalibrationOutcomeFalse        = "false"
	CalibrationOutcomePartial      = "partial"
	CalibrationOutcomeUnresolvable = "unresolvable"
)

// Scorer methods, in governing order rule > owner > independent > self
// (docs/precommitment-records.md §5; first stated in
// docs/archive/precommitment/precommitment-predictions-and-scoring-design-2026-10-11.md §4.3).
const (
	CalibrationScorerRule        = "rule"
	CalibrationScorerOwner       = "owner"
	CalibrationScorerIndependent = "independent"
	CalibrationScorerSelf        = "self"
)

// CommitmentCalibration is the calibration body. Agent is the claimant on
// both the claim and its settlements, so one query reads an agent's whole
// history. Exactly one of Claim and Settlement is set.
type CommitmentCalibration struct {
	Schema     string                 `json:"schema"`
	Agent      string                 `json:"agent"`
	Claim      *CalibrationClaim      `json:"claim,omitempty"`
	Settlement *CalibrationSettlement `json:"settlement,omitempty"`
}

// CalibrationClaim is what the agent claimed. Probability is the stated
// probability that the claim is true (an assertion holds, a forecast
// happens, a commitment is kept). The claimant's model is on
// Provenance.ModelID and the session on Provenance.ContextRef.
type CalibrationClaim struct {
	Text        string   `json:"text"`
	Type        string   `json:"type"`
	Probability float64  `json:"probability"`
	Criterion   string   `json:"criterion,omitempty"`
	Deadline    string   `json:"deadline,omitempty"`
	Project     string   `json:"project,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SessionID   string   `json:"session_id,omitempty"`
	// StatedAt is when the claim was originally made, for rows imported
	// after the fact; Reconstructed marks them. The record's own commit time
	// is always the server's.
	StatedAt      string `json:"stated_at,omitempty"`
	Reconstructed bool   `json:"reconstructed,omitempty"`
	SourceRef     string `json:"source_ref,omitempty"`
}

// CalibrationSettlement is one scorer's settlement of a claim. The scorer's
// identity is also the record's Provenance (agent and model). Brier is
// (ClaimProbability - y)^2 and is absent for an unresolvable outcome.
type CalibrationSettlement struct {
	ClaimID          string   `json:"claim_id"`
	Outcome          string   `json:"outcome"`
	ScorerMethod     string   `json:"scorer_method"`
	ScorerName       string   `json:"scorer_name"`
	ScorerModel      string   `json:"scorer_model,omitempty"`
	ClaimProbability float64  `json:"claim_probability"`
	Brier            *float64 `json:"brier,omitempty"`
	Note             string   `json:"note,omitempty"`
}
