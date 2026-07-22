package types

import "time"

// RunAcceptanceLevel describes how much of the intended self-development
// product path has been proven by structured evidence.
type RunAcceptanceLevel string

const (
	RunAcceptanceDocsLevel         RunAcceptanceLevel = "docs-level"
	RunAcceptanceStagingSmokeLevel RunAcceptanceLevel = "staging-smoke-level"
	RunAcceptanceExportLevel       RunAcceptanceLevel = "export-level"
	RunAcceptancePromotionLevel    RunAcceptanceLevel = "promotion-level"
)

// RunAcceptanceState is the verifier outcome for the current evidence set.
type RunAcceptanceState string

const (
	RunAcceptanceSynthesized RunAcceptanceState = "synthesized"
	RunAcceptanceAccepted    RunAcceptanceState = "accepted"
	RunAcceptanceBlocked     RunAcceptanceState = "blocked"
	RunAcceptanceFailed      RunAcceptanceState = "failed"
)

// RunAcceptanceRecord is the durable verifier object for a Choir run. It
// records derived evidence only; product-path tests should synthesize it from
// traces, runs, worker exports, and promotion records instead of seeding
// checkpoints directly.
type RunAcceptanceRecord struct {
	AcceptanceID            string                          `json:"acceptance_id"`
	TargetMissionID         string                          `json:"target_mission_id"`
	SourcePromptObjective   string                          `json:"source_prompt_or_objective,omitempty"`
	OwnerID                 string                          `json:"user_id"`
	DesktopID               string                          `json:"desktop_id,omitempty"`
	TrajectoryID            string                          `json:"trajectory_id"`
	RunID                   string                          `json:"loop_id,omitempty"`
	AuthorityProfile        string                          `json:"authority_profile,omitempty"`
	BaseSHA                 string                          `json:"base_sha,omitempty"`
	DeploymentCommit        string                          `json:"deployment_commit,omitempty"`
	CIRunID                 string                          `json:"ci_run_id,omitempty"`
	DeployRunID             string                          `json:"deploy_run_id,omitempty"`
	StagingURL              string                          `json:"staging_url,omitempty"`
	HealthCommit            string                          `json:"health_commit,omitempty"`
	AcceptanceLevel         RunAcceptanceLevel              `json:"acceptance_level"`
	VMMode                  string                          `json:"vm_mode,omitempty"`
	GatewayProviderEvidence string                          `json:"gateway_provider_evidence,omitempty"`
	State                   RunAcceptanceState              `json:"state"`
	Checkpoints             []RunAcceptanceCheckpoint       `json:"checkpoints,omitempty"`
	InvariantChecks         []RunAcceptanceInvariantCheck   `json:"invariant_checks,omitempty"`
	VerifierContracts       []RunAcceptanceVerifierContract `json:"verifier_contracts,omitempty"`
	EvidenceRefs            []RunAcceptanceEvidenceRef      `json:"evidence_refs,omitempty"`
	RollbackRefs            []RunAcceptanceRollbackRef      `json:"rollback_refs,omitempty"`
	FailureResidualRisks    []string                        `json:"failure_or_residual_risks,omitempty"`
	CreatedAt               time.Time                       `json:"created_at"`
	UpdatedAt               time.Time                       `json:"updated_at"`
}

type RunAcceptanceCheckpoint struct {
	Kind           string         `json:"kind"`
	State          string         `json:"state"`
	At             time.Time      `json:"at,omitempty"`
	StreamSeq      int64          `json:"stream_seq,omitempty"`
	EvidenceRefIDs []string       `json:"evidence_ref_ids,omitempty"`
	Details        map[string]any `json:"details,omitempty"`
}

type RunAcceptanceInvariantCheck struct {
	Name           string   `json:"name"`
	State          string   `json:"state"`
	Detail         string   `json:"detail,omitempty"`
	EvidenceRefIDs []string `json:"evidence_ref_ids,omitempty"`
}

type RunAcceptanceVerifierContract struct {
	Name           string   `json:"name"`
	Purpose        string   `json:"purpose,omitempty"`
	State          string   `json:"state"`
	EvidenceRefIDs []string `json:"evidence_ref_ids,omitempty"`
}

type RunAcceptanceEvidenceRef struct {
	RefID      string         `json:"ref_id"`
	Kind       string         `json:"kind"`
	Summary    string         `json:"summary,omitempty"`
	RunID      string         `json:"loop_id,omitempty"`
	EventID    string         `json:"event_id,omitempty"`
	Trajectory string         `json:"trajectory_id,omitempty"`
	URL        string         `json:"url,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

type RunAcceptanceRollbackRef struct {
	Kind    string `json:"kind"`
	Ref     string `json:"ref"`
	Summary string `json:"summary,omitempty"`
}
