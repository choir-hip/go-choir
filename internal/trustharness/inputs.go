// Package trustharness is the evidence harness for the paper (modular
// architecture plan MA7.9): from the instrumented records of one computer it
// produces the pre-registered tests of MA0.6
// (docs/evidence/preregistration-2026-11-04.md) and the mission's own
// precommitment ledger scores, as a deterministic artifact.
//
// The harness is two pure steps. The inputs (Inputs) are what the computer's
// owner routes served, reduced to the fields the tests read (no record
// content, no run or owner ids), plus the parsed ledger; they are stored
// beside the result. Compute turns stored inputs into the report, and
// Render writes the report's JSON and markdown bytes; the same inputs give
// the same bytes on every run (the plan's proof: the harness reproduces a
// stored result byte for byte from the stored inputs).
//
// Where a test has no data the report says so with its n, and a test below
// its pre-registered minimum n before its deadline reports its counts only:
// the pre-registration forbids interim looks at outcome metrics (section 1,
// Deadline), so they are sealed, not computed.
package trustharness

import (
	"time"

	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// InputsSchema and ReportSchema version the stored files.
const (
	InputsSchema = "choir.trust_harness.inputs.v1"
	ReportSchema = "choir.trust_harness.report.v1"
)

// Deadline is the pre-registered evaluation deadline: 12 weeks after MA7.9
// lands (2026-11-06). A test is evaluated at the first of its minimum n or
// this date (pre-registration section 1).
var Deadline = time.Date(2027, 1, 29, 0, 0, 0, 0, time.UTC)

// BootstrapSeed and BootstrapResamples are the pre-registered interval
// procedure (section 1: 2,000 resamples over subjects, seed recorded).
const (
	BootstrapSeed      = 20261104
	BootstrapResamples = 2000
)

// Inputs are the stored inputs of one harness run.
type Inputs struct {
	Schema     string    `json:"schema"`
	ComputerID string    `json:"computer_id"`
	FetchedAt  time.Time `json:"fetched_at"`
	// BuildCommit is the commit the computer reported serving.
	BuildCommit string `json:"build_commit"`
	// Served names the optional fields the computer's build served; a field
	// it did not serve is reported as such, never as zero.
	Served map[string]bool `json:"served"`

	// From GET /api/trust?replay=1.
	ViewRows    int                  `json:"view_rows"`
	Replay      *trust.ReplayReport  `json:"replay,omitempty"`
	GateReplay  *trust.GateReport    `json:"gate_replay,omitempty"`
	PreregUnits *trust.PreregUnitSet `json:"prereg_units,omitempty"`
	GatedActs   []GatedAct           `json:"gated_acts,omitempty"`
	// AnchorSample is the spot-check anchors and corpus rows (MA7.10), the
	// input of P6 and P7.
	AnchorSample *trust.AnchorSample `json:"anchor_sample,omitempty"`

	// From GET /api/subcalls: the inert rates (MA1.6) and the settlement
	// counts by plane and class.
	InertRates         []types.InertRateRow      `json:"inert_rates"`
	InertAgreement     types.InertJudgeAgreement `json:"inert_agreement"`
	SettlementsByPlane map[string]map[string]int `json:"settlements_by_plane,omitempty"`

	// From GET /api/leases and GET /api/runtime/observability.
	Leases        []LeaseObs       `json:"leases"`
	ActGateCounts map[string]int64 `json:"act_gate_counts,omitempty"`

	// Drills are the planted-package drills on d@e (MA7.9 d), when any ran.
	Drills []Drill `json:"drills"`

	// Ledger is the mission's own precommitment ledger, parsed.
	Ledger LedgerInput `json:"ledger"`
}

// GatedAct is one act gate tap as the disclosure tape measured it.
type GatedAct struct {
	Desk       string            `json:"desk"`
	Class      trust.ActionClass `json:"class"`
	Breaker    bool              `json:"breaker,omitempty"`
	AskedAt    time.Time         `json:"asked_at"`
	Answer     string            `json:"answer,omitempty"`
	AnsweredAt *time.Time        `json:"answered_at,omitempty"`
}

// LeaseObs is a lease reduced to what MA7.9 (c) measures.
type LeaseObs struct {
	Holder    string     `json:"holder"`
	Durable   bool       `json:"durable,omitempty"`
	State     string     `json:"state"`
	EndReason string     `json:"end_reason,omitempty"`
	IssuedAt  time.Time  `json:"issued_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	ExpiresAt time.Time  `json:"expires_at,omitempty"`
	Renewals  int        `json:"renewals,omitempty"`
}

// Drill is one planted known-bad package on d@e: when it was installed and
// when its first act was gated (zero if never).
type Drill struct {
	Name         string     `json:"name"`
	InstalledAt  time.Time  `json:"installed_at"`
	FirstGatedAt *time.Time `json:"first_gated_at,omitempty"`
}
