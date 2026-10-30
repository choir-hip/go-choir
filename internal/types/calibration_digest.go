package types

// Calibration feedback (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md).
// A desk's calibration digest is aggregate signal over its own settled
// self-scores: how often the claims it gave each probability held, by
// independent settlement after it made them. It never carries a per-item
// verdict or a pending item, the same relaxation of the score-free acting
// pack the issuer tallies already make
// (docs/desk-system-harness-architecture-2026-10-01.md).

// Claim types a desk scores itself on.
const (
	// CalibrationClaimHypothesis is a Cast hypothesis's probability,
	// settled confirmed (1), qualified (0.5) or contradicted (0).
	CalibrationClaimHypothesis = "hypothesis"
	// CalibrationClaimCompleted is a final research Report's p_complete,
	// settled by the share of the Cast's questions it answers.
	CalibrationClaimCompleted = "completed"
	// CalibrationClaimCitationSupport is a Texture Write's pSupported,
	// settled by the support judge's verdict on each new citation.
	CalibrationClaimCitationSupport = "citation_support"
)

// SelfScoreComplete is the Probabilities key of a research Report's self
// score: its probability that it answers every question it was asked.
const SelfScoreComplete = "complete"

// CalibrationScoreRecordPrefix starts the id of a calibration score
// record: an independent scorer's settlement of a desk's self-score,
// written through the owner's /api/calibration/settlements. Such a record
// is a resolve with discrepancy unresolved and no observation, so it
// never reads as a verdict in an acting pack.
const CalibrationScoreRecordPrefix = "calibration:"

// CalibrationScoreCoverage is the Probabilities key a scorer settles a
// completed Report with: the share of the Cast's questions it answers.
const CalibrationScoreCoverage = "coverage"

// CalibrationBin is one band of stated probability: how many settled
// claims fell in it, their mean stated p, and the share that held.
type CalibrationBin struct {
	Label    string  `json:"label"`
	N        int     `json:"n"`
	MeanP    float64 `json:"mean_p"`
	Observed float64 `json:"observed"`
}

// CalibrationStat is calibration over one slice (a claim type or a model).
type CalibrationStat struct {
	Key      string  `json:"key"`
	N        int     `json:"n"`
	Brier    float64 `json:"brier"`
	ECE      float64 `json:"ece"`
	MeanP    float64 `json:"mean_p"`
	Observed float64 `json:"observed"`
}

// CalibrationDigest is one desk's track record as the digest shows it.
// Text is the rendered block the desk reads; it carries no time, so it
// changes only when the record does.
type CalibrationDigest struct {
	Desk string `json:"desk"`
	// N is the settled observations the digest covers (the most recent
	// window, held-out items excluded); HeldOut counts those withheld.
	N       int `json:"n"`
	HeldOut int `json:"held_out"`
	// Brier is the mean squared error of stated p against the outcome;
	// ECE the n-weighted mean |mean p - observed| over the bins.
	Brier    float64 `json:"brier"`
	ECE      float64 `json:"ece"`
	MeanP    float64 `json:"mean_p"`
	Observed float64 `json:"observed"`
	// Direction is "under-confident", "over-confident" or "calibrated".
	Direction  string            `json:"direction"`
	Bins       []CalibrationBin  `json:"bins"`
	ClaimTypes []CalibrationStat `json:"claim_types,omitempty"`
	// Worst is the claim type with the largest calibration error.
	Worst string `json:"worst,omitempty"`
	// Model is the line for the model the reading run is on, when it has
	// enough settled observations of its own.
	Model      *CalibrationStat `json:"model,omitempty"`
	ComputedAt string           `json:"computed_at,omitempty"`
	Text       string           `json:"text"`
}
