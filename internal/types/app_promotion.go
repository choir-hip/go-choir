package types

import (
	"encoding/json"
	"time"
)

// ComputerSourceLineageRecord is the product-visible source/build profile for
// one computer. It is the control-plane record that active computers expose as
// a base for candidate work and promotion.
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

type AppAdoptionStatus string

const (
	AppAdoptionProposed         AppAdoptionStatus = "adoption_proposed"
	AppAdoptionCandidateApplied AppAdoptionStatus = "candidate_applied"
	AppAdoptionVerifying        AppAdoptionStatus = "verifying"
	AppAdoptionBuilt            AppAdoptionStatus = "built"
	AppAdoptionVerified         AppAdoptionStatus = "verified"
	AppAdoptionOwnerApproved    AppAdoptionStatus = "owner_approved"
	AppAdoptionAdopted          AppAdoptionStatus = "adopted"
	AppAdoptionRolledBack       AppAdoptionStatus = "rolled_back"
	AppAdoptionBlocked          AppAdoptionStatus = "blocked"
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
