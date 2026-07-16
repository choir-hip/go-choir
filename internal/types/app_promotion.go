package types

import (
	"encoding/json"
	"time"
)

// ComputerSourceLineageRecord is evidence-only source/build metadata for one
// computer. ActiveSourceRef and the legacy-named RouteProfile support candidate
// comparison and adoption receipts; neither authorizes or resolves a served
// route. The vmctl-owned D-ROUTE slot is the sole routing authority.
type ComputerSourceLineageRecord struct {
	OwnerID            string    `json:"owner_id"`
	ComputerID         string    `json:"computer_id"`
	ComputerKind       string    `json:"computer_kind"`
	ActiveSourceRef    string    `json:"active_source_ref"`
	RuntimeDigest      string    `json:"runtime_digest,omitempty"`
	UIDigest           string    `json:"ui_digest,omitempty"`
	RouteProfile       string    `json:"route_profile,omitempty"`
	DefaultBaseProfile string    `json:"default_base_profile,omitempty"`
	LastAdoptionID     string    `json:"last_adoption_id,omitempty"`
	LastPackageID      string    `json:"last_package_id,omitempty"`
	LastCandidateRef   string    `json:"last_candidate_ref,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AppChangePackageStatus string

const (
	AppChangePackageDraft             AppChangePackageStatus = "draft"
	AppChangePackageExported          AppChangePackageStatus = "exported"
	AppChangePackagePublishedPrivate  AppChangePackageStatus = "published_private"
	AppChangePackagePublishedUnlisted AppChangePackageStatus = "published_unlisted"
	AppChangePackagePublishedPublic   AppChangePackageStatus = "published_public"
	AppChangePackageArchived          AppChangePackageStatus = "archived"
)

// AppChangePackageRecord is the durable source-package object that moves
// between divergent computers. It carries source deltas and contracts, not
// copied user runtime/UI binaries.
type AppChangePackageRecord struct {
	PackageID                   string                 `json:"package_id"`
	OwnerID                     string                 `json:"owner_id"`
	AppID                       string                 `json:"app_id"`
	Status                      AppChangePackageStatus `json:"status"`
	Visibility                  string                 `json:"visibility"`
	SourceComputerID            string                 `json:"source_computer_id"`
	SourceCandidateID           string                 `json:"source_candidate_id"`
	SourceActiveRef             string                 `json:"source_active_ref"`
	CandidateSourceRef          string                 `json:"candidate_source_ref"`
	RuntimeSourceDelta          string                 `json:"runtime_source_delta,omitempty"`
	UISourceDelta               string                 `json:"ui_source_delta,omitempty"`
	RuntimeSourceDeltaSHA256    string                 `json:"runtime_source_delta_sha256,omitempty"`
	UISourceDeltaSHA256         string                 `json:"ui_source_delta_sha256,omitempty"`
	PackageManifestSHA256       string                 `json:"package_manifest_sha256"`
	AppProtocolContract         string                 `json:"app_protocol_contract,omitempty"`
	AppProtocolContractSHA256   string                 `json:"app_protocol_contract_sha256,omitempty"`
	SourceRuntimeArtifactDigest string                 `json:"source_runtime_artifact_digest,omitempty"`
	SourceUIArtifactDigest      string                 `json:"source_ui_artifact_digest,omitempty"`
	ManifestJSON                json.RawMessage        `json:"manifest_json,omitempty"`
	VerifierContractsJSON       json.RawMessage        `json:"verifier_contracts_json,omitempty"`
	ProvenanceRefsJSON          json.RawMessage        `json:"provenance_refs_json,omitempty"`
	TraceID                     string                 `json:"trace_id,omitempty"`
	CreatedAt                   time.Time              `json:"created_at"`
	UpdatedAt                   time.Time              `json:"updated_at"`
}

type CandidatePackageIntakeStatus string

const (
	CandidatePackageIntakeOwnerReviewPending CandidatePackageIntakeStatus = "owner_review_pending"
	CandidatePackageIntakeOwnerApproved      CandidatePackageIntakeStatus = "owner_approved"
	CandidatePackageIntakeRejected           CandidatePackageIntakeStatus = "rejected"
	CandidatePackageIntakeArchived           CandidatePackageIntakeStatus = "archived"
)

type CandidatePackageOwnerReviewState string

const (
	CandidatePackageOwnerReviewRequired CandidatePackageOwnerReviewState = "required"
	CandidatePackageOwnerReviewApproved CandidatePackageOwnerReviewState = "approved"
	CandidatePackageOwnerReviewRejected CandidatePackageOwnerReviewState = "rejected"
)

// CandidatePackageIntakeRecord is the evidence-only owner-review record for a
// candidate-computer package. It deliberately does not publish an
// AppChangePackage, create an adoption, promote a computer, or change active
// routes; it persists the review boundary and blockers that must be cleared
// before a later product path may act on the package.
type CandidatePackageIntakeRecord struct {
	IntakeID                       string                           `json:"intake_id"`
	OwnerID                        string                           `json:"owner_id"`
	CandidatePackageID             string                           `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string                           `json:"candidate_package_manifest_sha256"`
	SourceComputerID               string                           `json:"source_computer_id,omitempty"`
	SourceCandidateID              string                           `json:"source_candidate_id,omitempty"`
	CandidateSourceRef             string                           `json:"candidate_source_ref,omitempty"`
	IntakeBoundary                 string                           `json:"intake_boundary"`
	Status                         CandidatePackageIntakeStatus     `json:"status"`
	OwnerReviewState               CandidatePackageOwnerReviewState `json:"owner_review_state"`
	OwnerReviewRequired            bool                             `json:"owner_review_required"`
	AdoptionReady                  bool                             `json:"adoption_ready"`
	AdoptionBlockersJSON           json.RawMessage                  `json:"adoption_blockers_json,omitempty"`
	VerifierContractsJSON          json.RawMessage                  `json:"verifier_contracts_json,omitempty"`
	EvidenceRefsJSON               json.RawMessage                  `json:"evidence_refs_json,omitempty"`
	RequiredObservationsJSON       json.RawMessage                  `json:"required_observations_json,omitempty"`
	AcceptanceJSON                 json.RawMessage                  `json:"acceptance_json,omitempty"`
	TraceID                        string                           `json:"trace_id,omitempty"`
	CreatedAt                      time.Time                        `json:"created_at"`
	UpdatedAt                      time.Time                        `json:"updated_at"`
}

type AppAdoptionStatus string

const (
	AppAdoptionProposed              AppAdoptionStatus = "adoption_proposed"
	AppAdoptionOwnerReviewPending    AppAdoptionStatus = "owner_review_pending"
	AppAdoptionOwnerReviewApproved   AppAdoptionStatus = "owner_review_approved"
	AppAdoptionOwnerReviewRejected   AppAdoptionStatus = "owner_review_rejected"
	AppAdoptionSourceLineageSwitched AppAdoptionStatus = "source_lineage_switched"
	AppAdoptionCandidateApplied      AppAdoptionStatus = "candidate_applied"
	AppAdoptionVerifying             AppAdoptionStatus = "verifying"
	AppAdoptionBuilt                 AppAdoptionStatus = "built"
	AppAdoptionVerified              AppAdoptionStatus = "verified"
	AppAdoptionOwnerApproved         AppAdoptionStatus = "owner_approved"
	AppAdoptionAdopted               AppAdoptionStatus = "adopted"
	AppAdoptionRolledBack            AppAdoptionStatus = "rolled_back"
	AppAdoptionBlocked               AppAdoptionStatus = "blocked"
)

// AppAdoptionRecord is the recipient-side promotion record for one package
// entering one target computer through a candidate.
type AppAdoptionRecord struct {
	AdoptionID                            string            `json:"adoption_id"`
	OwnerID                               string            `json:"owner_id"`
	PackageID                             string            `json:"package_id"`
	AppID                                 string            `json:"app_id"`
	TargetComputerID                      string            `json:"target_computer_id"`
	TargetComputerKind                    string            `json:"target_computer_kind"`
	TargetCandidateID                     string            `json:"target_candidate_id"`
	Status                                AppAdoptionStatus `json:"status"`
	TargetActiveSourceRefAtCandidateStart string            `json:"target_active_source_ref_at_candidate_start"`
	TargetActiveSourceRefAtCutover        string            `json:"target_active_source_ref_at_cutover,omitempty"`
	CandidateSourceRef                    string            `json:"candidate_source_ref"`
	ForegroundTailMergeResult             string            `json:"foreground_tail_merge_result,omitempty"`
	MergeStrategy                         string            `json:"merge_strategy,omitempty"`
	MergeConflictsJSON                    json.RawMessage   `json:"merge_conflicts_json,omitempty"`
	RuntimeArtifactDigest                 string            `json:"runtime_artifact_digest,omitempty"`
	UIArtifactDigest                      string            `json:"ui_artifact_digest,omitempty"`
	VerifierResultsJSON                   json.RawMessage   `json:"verifier_results_json,omitempty"`
	RollbackProfileJSON                   json.RawMessage   `json:"rollback_profile_json,omitempty"`
	RouteProfile                          string            `json:"route_profile,omitempty"`
	DefaultBaseProfile                    string            `json:"default_base_profile,omitempty"`
	TraceID                               string            `json:"trace_id,omitempty"`
	Error                                 string            `json:"error,omitempty"`
	CreatedAt                             time.Time         `json:"created_at"`
	UpdatedAt                             time.Time         `json:"updated_at"`
}
