package computerversion

import (
	"fmt"
	"strings"
)

const CandidatePackageOwnerActivationDecisionKind = "candidate_package_owner_activation_decision"

const CandidatePackageDurableActivationContractKind = "candidate_package_durable_activation_contract"

const CandidatePackageOwnerDecisionPreparableState = "owner_decision_preparable"

const CandidatePackagePrepareActivationDecisionAction = "prepare_activation_decision"

const CandidatePackagePromotionRequiresProductActivationContractBoundary = "app_adoption_promotion_requires_separate_product_activation_contract"

// CandidatePackageOwnerActivationDecision is the pure, durable shape of the
// Candidate Review surface's owner-controlled activation decision boundary. It
// is a prepared decision only: it must not publish a package, mutate
// AppAdoption, touch deployed routes or auth/session state, claim staging, change
// VM lifecycle state, or synthesize run-acceptance records.
type CandidatePackageOwnerActivationDecision struct {
	Kind                           string          `json:"kind"`
	State                          string          `json:"state"`
	OwnerControlled                bool            `json:"owner_controlled"`
	RequiresAuthenticatedOwner     bool            `json:"requires_authenticated_owner"`
	PreparedAction                 string          `json:"prepared_action"`
	NoMutation                     bool            `json:"no_mutation"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	NextBoundary                   string          `json:"next_boundary"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedRoutes                  []string        `json:"blocked_routes"`
	RequiredContracts              []string        `json:"required_contracts"`
}

// CandidatePackageDurableActivationContract records that a prepared owner
// decision has been bound to accepted candidate-package evidence without
// widening into activation. It is deliberately inert and deterministic.
type CandidatePackageDurableActivationContract struct {
	Kind                           string            `json:"kind"`
	CandidatePackageID             string            `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string            `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion   `json:"version"`
	ProductPathAcceptanceKind      string            `json:"product_path_acceptance_kind"`
	UsesLocalAcceptanceID          string            `json:"uses_local_acceptance_id"`
	OwnerDecisionState             string            `json:"owner_decision_state"`
	PreparedAction                 string            `json:"prepared_action"`
	NextBoundary                   string            `json:"next_boundary"`
	ActivationReady                bool              `json:"activation_ready"`
	NoMutation                     bool              `json:"no_mutation"`
	PromotionLevelClaimed          bool              `json:"promotion_level_claimed"`
	ActivationBlockers             []string          `json:"activation_blockers"`
	BlockedRoutes                  []string          `json:"blocked_routes"`
	RequiredContracts              []string          `json:"required_contracts"`
	EvidenceRefs                   []string          `json:"evidence_refs"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
}

// BuildCandidatePackageDurableActivationContract binds the existing
// candidate-package manifest and product-path acceptance contract to a prepared
// owner activation decision boundary. The returned contract cannot authorize
// activation; it only persists the blockers that keep activation behind a later
// product activation contract.
func BuildCandidatePackageDurableActivationContract(pkg CandidateComputerPackageManifest, acceptance CandidatePackageProductPathAcceptanceContract, decision CandidatePackageOwnerActivationDecision) (CandidatePackageDurableActivationContract, error) {
	if err := validateCandidatePackageForDurableActivation(pkg); err != nil {
		return CandidatePackageDurableActivationContract{}, err
	}
	if err := validateProductPathAcceptanceForDurableActivation(pkg, acceptance); err != nil {
		return CandidatePackageDurableActivationContract{}, err
	}
	if err := validateOwnerActivationDecision(pkg, decision); err != nil {
		return CandidatePackageDurableActivationContract{}, err
	}

	return CandidatePackageDurableActivationContract{
		Kind:                           CandidatePackageDurableActivationContractKind,
		CandidatePackageID:             pkg.ID,
		CandidatePackageManifestSHA256: pkg.PackageManifestSHA256,
		Version:                        pkg.Version,
		ProductPathAcceptanceKind:      acceptance.Kind,
		UsesLocalAcceptanceID:          strings.TrimSpace(decision.UsesLocalAcceptanceID),
		OwnerDecisionState:             decision.State,
		PreparedAction:                 decision.PreparedAction,
		NextBoundary:                   decision.NextBoundary,
		ActivationReady:                false,
		NoMutation:                     true,
		PromotionLevelClaimed:          false,
		ActivationBlockers:             candidatePackageDurableActivationBlockers(),
		BlockedRoutes:                  canonicalStrings(decision.BlockedRoutes),
		RequiredContracts:              canonicalStrings(decision.RequiredContracts),
		EvidenceRefs:                   canonicalStrings(acceptance.EvidenceRefs),
		RequiredObservations:           canonicalObservationKinds(acceptance.RequiredObservations),
	}, nil
}

func validateCandidatePackageForDurableActivation(pkg CandidateComputerPackageManifest) error {
	if err := pkg.Validate(); err != nil {
		return fmt.Errorf("candidate package durable activation: package: %w", err)
	}
	if strings.TrimSpace(pkg.PackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package durable activation: package manifest hash is required")
	}
	return nil
}

func validateProductPathAcceptanceForDurableActivation(pkg CandidateComputerPackageManifest, acceptance CandidatePackageProductPathAcceptanceContract) error {
	if strings.TrimSpace(acceptance.Kind) != CandidatePackageProductPathAcceptanceKind {
		return fmt.Errorf("candidate package durable activation: acceptance kind %q is not %q", acceptance.Kind, CandidatePackageProductPathAcceptanceKind)
	}
	if acceptance.CandidatePackageID != pkg.ID {
		return fmt.Errorf("candidate package durable activation: acceptance package id %q does not match package %q", acceptance.CandidatePackageID, pkg.ID)
	}
	if acceptance.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		return fmt.Errorf("candidate package durable activation: acceptance package hash does not match package hash")
	}
	if acceptance.Version != pkg.Version {
		return fmt.Errorf("candidate package durable activation: acceptance version does not match package version")
	}
	if strings.TrimSpace(acceptance.IntakeBoundary) != CandidatePackageEvidenceOnlyIntakeBoundary {
		return fmt.Errorf("candidate package durable activation: acceptance intake boundary %q is not %q", acceptance.IntakeBoundary, CandidatePackageEvidenceOnlyIntakeBoundary)
	}
	if !acceptance.OwnerReviewRequired {
		return fmt.Errorf("candidate package durable activation: acceptance owner review is required")
	}
	if acceptance.AdoptionReady {
		return fmt.Errorf("candidate package durable activation: acceptance cannot be adoption-ready")
	}
	if len(acceptance.AdoptionBlockers) == 0 {
		return fmt.Errorf("candidate package durable activation: acceptance adoption blockers are required")
	}
	return nil
}

func validateOwnerActivationDecision(pkg CandidateComputerPackageManifest, decision CandidatePackageOwnerActivationDecision) error {
	if strings.TrimSpace(decision.Kind) != CandidatePackageOwnerActivationDecisionKind {
		return fmt.Errorf("candidate package durable activation: owner decision kind %q is not %q", decision.Kind, CandidatePackageOwnerActivationDecisionKind)
	}
	if strings.TrimSpace(decision.State) != CandidatePackageOwnerDecisionPreparableState {
		return fmt.Errorf("candidate package durable activation: owner decision state %q is not %q", decision.State, CandidatePackageOwnerDecisionPreparableState)
	}
	if !decision.OwnerControlled {
		return fmt.Errorf("candidate package durable activation: owner decision must be owner-controlled")
	}
	if !decision.RequiresAuthenticatedOwner {
		return fmt.Errorf("candidate package durable activation: owner decision requires authenticated owner")
	}
	if strings.TrimSpace(decision.PreparedAction) != CandidatePackagePrepareActivationDecisionAction {
		return fmt.Errorf("candidate package durable activation: owner decision prepared action %q is not %q", decision.PreparedAction, CandidatePackagePrepareActivationDecisionAction)
	}
	if !decision.NoMutation {
		return fmt.Errorf("candidate package durable activation: owner decision cannot cross mutation boundary")
	}
	if strings.TrimSpace(decision.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package durable activation: owner decision acceptance id is required")
	}
	if decision.CandidatePackageID != pkg.ID {
		return fmt.Errorf("candidate package durable activation: owner decision package id %q does not match package %q", decision.CandidatePackageID, pkg.ID)
	}
	if decision.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		return fmt.Errorf("candidate package durable activation: owner decision package hash does not match package hash")
	}
	if decision.Version != pkg.Version {
		return fmt.Errorf("candidate package durable activation: owner decision version does not match package version")
	}
	if strings.TrimSpace(decision.NextBoundary) != CandidatePackagePromotionRequiresProductActivationContractBoundary {
		return fmt.Errorf("candidate package durable activation: owner decision next boundary %q is not %q", decision.NextBoundary, CandidatePackagePromotionRequiresProductActivationContractBoundary)
	}
	if decision.ActivationReady {
		return fmt.Errorf("candidate package durable activation: owner decision cannot mark activation ready")
	}
	if decision.PromotionLevelClaimed {
		return fmt.Errorf("candidate package durable activation: owner decision cannot claim promotion level")
	}
	return nil
}

func candidatePackageDurableActivationBlockers() []string {
	return []string{
		"package_publication_not_authorized",
		"app_adoption_mutation_not_authorized",
		"deployed_route_mutation_not_authorized",
		"auth_session_mutation_not_authorized",
		"staging_acceptance_not_claimed",
		"vm_lifecycle_mutation_not_authorized",
		"run_acceptance_record_not_created",
	}
}

const CandidatePackageProductActivationVerifierContractKind = "candidate_package_product_activation_verifier_contract"

const (
	CandidatePackageProductActivationPrerequisitePackagePublication = "package_publication_contract"
	CandidatePackageProductActivationPrerequisiteAppAdoption        = "app_adoption_mutation_contract"
	CandidatePackageProductActivationPrerequisiteDeployedRoute      = "deployed_route_mutation_contract"
	CandidatePackageProductActivationPrerequisiteAuthSession        = "auth_session_contract"
	CandidatePackageProductActivationPrerequisiteStagingAcceptance  = "staging_identity_contract"
	CandidatePackageProductActivationPrerequisiteVMLifecycle        = "vm_lifecycle_contract"
	CandidatePackageProductActivationPrerequisiteRunAcceptance      = "run_acceptance_contract"
)

const (
	CandidatePackageProductActivationEvidenceStatusCandidate = "candidate"
	CandidatePackageProductActivationEvidenceStatusPassed    = "passed"
)

const (
	CandidatePackageProductActivationVerifierStatusBindable = "bindable"
	CandidatePackageProductActivationVerifierStatusBlocked  = "blocked"
)

// CandidatePackageProductActivationPrerequisiteEvidence is a pure evidence
// proposal for one product-activation prerequisite. A candidate is only
// bindable when this verifier says it is; a passed status without an evidence
// ref is rejected or narrowed to blocked.
type CandidatePackageProductActivationPrerequisiteEvidence struct {
	Prerequisite string `json:"prerequisite"`
	Status       string `json:"status"`
	EvidenceRef  string `json:"evidence_ref"`
}

// CandidatePackageProductActivationEvidence binds prerequisite evidence to the
// durable activation contract identity. It does not mutate any runtime/product
// state.
type CandidatePackageProductActivationEvidence struct {
	CandidatePackageID             string                                                  `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string                                                  `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion                                         `json:"version"`
	UsesLocalAcceptanceID          string                                                  `json:"uses_local_acceptance_id"`
	Prerequisites                  []CandidatePackageProductActivationPrerequisiteEvidence `json:"prerequisites"`
}

// CandidatePackageProductActivationVerifierContract is the next pure verifier
// after the durable activation contract. It selects the first safe prerequisite
// that may be bound next and keeps activation blocked until every protected
// prerequisite has explicit evidence in a later pass.
type CandidatePackageProductActivationVerifierContract struct {
	Kind                           string            `json:"kind"`
	CandidatePackageID             string            `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string            `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion   `json:"version"`
	UsesLocalAcceptanceID          string            `json:"uses_local_acceptance_id"`
	FirstBindablePrerequisite      string            `json:"first_bindable_prerequisite,omitempty"`
	FirstBindableEvidenceRef       string            `json:"first_bindable_evidence_ref,omitempty"`
	FirstBindableStatus            string            `json:"first_bindable_status"`
	ActivationReady                bool              `json:"activation_ready"`
	NoMutation                     bool              `json:"no_mutation"`
	PromotionLevelClaimed          bool              `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string          `json:"blocked_prerequisites"`
	EvidenceRefs                   []string          `json:"evidence_refs,omitempty"`
	RequiredObservations           []ObservationKind `json:"required_observations,omitempty"`
}

const CandidatePackagePublicationProofContractKind = "candidate_package_publication_proof_contract"

// CandidatePackagePublicationProofEvidence is the pure review evidence for the
// package-publication prerequisite selected by the product activation verifier.
// It may bind the verifier's evidence ref, but it must not claim that any
// package was actually published or that activation-side mutation happened.
type CandidatePackagePublicationProofEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	EvidenceRef                    string          `json:"evidence_ref"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
}

// CandidatePackagePublicationProofContract binds package-publication proof
// review evidence to the selected verifier prerequisite. It remains inert: a
// bound proof is not an actual publication and cannot activate, promote, stage,
// mutate deployed routes, change VM lifecycle, or synthesize run acceptance.
type CandidatePackagePublicationProofContract struct {
	Kind                           string          `json:"kind"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	PublicationBound               bool            `json:"publication_bound"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	NoMutation                     bool            `json:"no_mutation"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string        `json:"blocked_prerequisites"`
}

const CandidatePackagePublicationPayloadContractKind = "candidate_package_publication_payload_contract"

const CandidatePackagePublicationPayloadBoundary = "reviewable_package_publication_payload_without_publish"

// CandidatePackagePublicationPayloadEvidence is the pure evidence object for a
// package-publication payload/source-delta boundary. It can bind source-delta
// and payload-manifest refs for review, but it must not publish, activate,
// promote, mutate product routes, touch auth/session, claim staging, change VM
// lifecycle, or synthesize run acceptance.
type CandidatePackagePublicationPayloadEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAuthSession              bool            `json:"claims_auth_session"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
}

// CandidatePackagePublicationPayloadContract binds the publication proof to
// explicit source-delta and payload-manifest refs. It is reviewable publication
// input only; direct publish and every product activation surface remain blocked.
type CandidatePackagePublicationPayloadContract struct {
	Kind                           string          `json:"kind"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	PayloadBoundary                string          `json:"payload_boundary"`
	ReviewablePublicationCandidate bool            `json:"reviewable_publication_candidate"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	NoMutation                     bool            `json:"no_mutation"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string        `json:"blocked_prerequisites"`
}

const CandidatePackagePublicationPreflightContractKind = "candidate_package_publication_preflight_contract"

const CandidatePackagePublicationPreflightBoundary = "package_publication_executor_preflight_without_execution"

// CandidatePackagePublicationPreflightEvidence is the pure checker input for a
// future package-publication executor. It records review/check refs only; it
// cannot allow execution or claim any publication/product mutation occurred.
type CandidatePackagePublicationPreflightEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	PreflightCheckRefs             []string        `json:"preflight_check_refs"`
	VerifierContractRefs           []string        `json:"verifier_contract_refs,omitempty"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAuthSession              bool            `json:"claims_auth_session"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
}

// CandidatePackagePublicationPreflightContract records the non-mutating checks
// required before any future red executor may be considered. It never permits
// execution and never publishes a package.
type CandidatePackagePublicationPreflightContract struct {
	Kind                           string          `json:"kind"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	PreflightBoundary              string          `json:"preflight_boundary"`
	PreflightCheckRefs             []string        `json:"preflight_check_refs"`
	VerifierContractRefs           []string        `json:"verifier_contract_refs,omitempty"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	NoMutation                     bool            `json:"no_mutation"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string        `json:"blocked_prerequisites"`
}

const CandidatePackagePublicationExecutorReviewGateContractKind = "candidate_package_publication_executor_review_gate_contract"

const CandidatePackagePublicationExecutorReviewGateBoundary = "package_publication_executor_design_review_gate_without_execution"

// CandidatePackagePublicationExecutorReviewGateEvidence records owner/reviewer
// authorization refs for design-reviewing a future red publication executor. It
// is not executor permission and cannot publish or mutate product state.
type CandidatePackagePublicationExecutorReviewGateEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	PreflightCheckRefs             []string        `json:"preflight_check_refs"`
	VerifierContractRefs           []string        `json:"verifier_contract_refs,omitempty"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAuthSession              bool            `json:"claims_auth_session"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
}

// CandidatePackagePublicationExecutorReviewGateContract states that a preflight
// packet has enough owner/reviewer review refs to enter future red executor
// design review. It does not allow execution or publish anything.
type CandidatePackagePublicationExecutorReviewGateContract struct {
	Kind                           string          `json:"kind"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	ReviewGateBoundary             string          `json:"review_gate_boundary"`
	PreflightCheckRefs             []string        `json:"preflight_check_refs"`
	VerifierContractRefs           []string        `json:"verifier_contract_refs,omitempty"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorDesignReviewReady      bool            `json:"executor_design_review_ready"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	NoMutation                     bool            `json:"no_mutation"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string        `json:"blocked_prerequisites"`
}

const CandidatePackagePublicationExecutorDesignSpecContractKind = "candidate_package_publication_executor_design_spec_contract"

const CandidatePackagePublicationExecutorDesignSpecBoundary = "package_publication_executor_red_design_spec_without_implementation"

const CandidatePackagePublicationExecutorRedSurfacePackageArtifact = "package_artifact_publication"
const CandidatePackagePublicationExecutorRedSurfaceProviderCredentials = "provider_publish_credentials"
const CandidatePackagePublicationExecutorRedSurfacePublicationLedger = "publication_ledger_write"
const CandidatePackagePublicationExecutorRedSurfaceRollbackPath = "rollback_path"

// CandidatePackagePublicationExecutorDesignSpecEvidence records the pure design
// refs for a future red package-publication executor. It enumerates required red
// surfaces and evidence, but does not implement or authorize the executor.
type CandidatePackagePublicationExecutorDesignSpecEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorDesignSpecRef          string          `json:"executor_design_spec_ref"`
	RequiredRedSurfaces            []string        `json:"required_red_surfaces"`
	RequiredEvidenceRefs           []string        `json:"required_evidence_refs"`
	RollbackPlanRef                string          `json:"rollback_plan_ref"`
	ExecutorImplemented            bool            `json:"executor_implemented"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAuthSession              bool            `json:"claims_auth_session"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
}

// CandidatePackagePublicationExecutorDesignSpecContract is a pure design-spec
// object for a future red package-publication executor. It names required red
// surfaces and evidence but leaves implementation and execution disabled.
type CandidatePackagePublicationExecutorDesignSpecContract struct {
	Kind                           string          `json:"kind"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	DesignSpecBoundary             string          `json:"design_spec_boundary"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorDesignSpecRef          string          `json:"executor_design_spec_ref"`
	RequiredRedSurfaces            []string        `json:"required_red_surfaces"`
	RequiredEvidenceRefs           []string        `json:"required_evidence_refs"`
	RollbackPlanRef                string          `json:"rollback_plan_ref"`
	ExecutorDesignSpecReady        bool            `json:"executor_design_spec_ready"`
	ExecutorImplemented            bool            `json:"executor_implemented"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	NoMutation                     bool            `json:"no_mutation"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string        `json:"blocked_prerequisites"`
}

const CandidatePackagePublicationExecutorImplementationReadinessContractKind = "candidate_package_publication_executor_implementation_readiness_contract"

const CandidatePackagePublicationExecutorImplementationReadinessBoundary = "package_publication_executor_implementation_readiness_without_code"

const CandidatePackagePublicationExecutorImplementationStatusBlocked = "blocked_until_red_ceremony"

const CandidatePackagePublicationExecutorImplementationGateRedCeremony = "red_ceremony_required"
const CandidatePackagePublicationExecutorImplementationGateOwnerApproval = "owner_approval_required"
const CandidatePackagePublicationExecutorImplementationGateSecurityReview = "security_review_required"
const CandidatePackagePublicationExecutorImplementationGateProviderCredentialProof = "provider_credential_proof_required"
const CandidatePackagePublicationExecutorImplementationGateRollbackDrill = "rollback_drill_required"

const CandidatePackagePublicationExecutorReadinessReviewContractKind = "candidate_package_publication_executor_readiness_review_contract"

const CandidatePackagePublicationExecutorReadinessReviewBoundary = "package_publication_executor_readiness_review_without_authorization"

const CandidatePackagePublicationExecutorReadinessReviewStatusChecklistRecorded = "checklist_recorded_without_red_authorization"

const CandidatePackagePublicationExecutorReadinessReviewItemRedCeremonyScope = "red_ceremony_scope_review"
const CandidatePackagePublicationExecutorReadinessReviewItemOwnerApprovalPath = "owner_approval_path_review"
const CandidatePackagePublicationExecutorReadinessReviewItemSecurityScope = "security_review_scope_review"
const CandidatePackagePublicationExecutorReadinessReviewItemProviderCredentialBoundary = "provider_credential_boundary_review"
const CandidatePackagePublicationExecutorReadinessReviewItemRollbackDrill = "rollback_drill_review"

// CandidatePackagePublicationExecutorImplementationReadinessEvidence records
// the gates that must open before code may touch a future red
// package-publication executor. It does not open those gates or implement code.
type CandidatePackagePublicationExecutorImplementationReadinessEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorDesignSpecRef          string          `json:"executor_design_spec_ref"`
	RequiredRedSurfaces            []string        `json:"required_red_surfaces"`
	RequiredEvidenceRefs           []string        `json:"required_evidence_refs"`
	RollbackPlanRef                string          `json:"rollback_plan_ref"`
	RedCeremonyPlanRef             string          `json:"red_ceremony_plan_ref"`
	RequiredGateRefs               []string        `json:"required_gate_refs"`
	EvidenceGateRefs               []string        `json:"evidence_gate_refs"`
	RollbackDrillRef               string          `json:"rollback_drill_ref"`
	RedCeremonyOpened              bool            `json:"red_ceremony_opened"`
	CodeSurfaceTouched             bool            `json:"code_surface_touched"`
	ImplementationReady            bool            `json:"implementation_ready"`
	ExecutorImplemented            bool            `json:"executor_implemented"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAuthSession              bool            `json:"claims_auth_session"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
}

// CandidatePackagePublicationExecutorImplementationReadinessContract records
// the unopened red ceremony/evidence gates that must exist before implementation
// work can touch a package-publication executor surface.
type CandidatePackagePublicationExecutorImplementationReadinessContract struct {
	Kind                            string          `json:"kind"`
	CandidatePackageID              string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256  string          `json:"candidate_package_manifest_sha256"`
	Version                         ComputerVersion `json:"version"`
	UsesLocalAcceptanceID           string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef             string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                  string          `json:"source_delta_ref"`
	PayloadManifestRef              string          `json:"payload_manifest_ref"`
	ImplementationReadinessBoundary string          `json:"implementation_readiness_boundary"`
	OwnerAuthorizationRef           string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef        string          `json:"reviewer_authorization_ref"`
	ExecutorDesignSpecRef           string          `json:"executor_design_spec_ref"`
	RequiredRedSurfaces             []string        `json:"required_red_surfaces"`
	RequiredEvidenceRefs            []string        `json:"required_evidence_refs"`
	RollbackPlanRef                 string          `json:"rollback_plan_ref"`
	RedCeremonyPlanRef              string          `json:"red_ceremony_plan_ref"`
	RequiredGateRefs                []string        `json:"required_gate_refs"`
	EvidenceGateRefs                []string        `json:"evidence_gate_refs"`
	RollbackDrillRef                string          `json:"rollback_drill_ref"`
	ImplementationReadinessStatus   string          `json:"implementation_readiness_status"`
	RedCeremonyOpened               bool            `json:"red_ceremony_opened"`
	CodeSurfaceTouched              bool            `json:"code_surface_touched"`
	ImplementationReady             bool            `json:"implementation_ready"`
	ExecutorImplemented             bool            `json:"executor_implemented"`
	ExecutorAllowed                 bool            `json:"executor_allowed"`
	ActualPackagePublished          bool            `json:"actual_package_published"`
	DirectPublishReady              bool            `json:"direct_publish_ready"`
	NoMutation                      bool            `json:"no_mutation"`
	ActivationReady                 bool            `json:"activation_ready"`
	PromotionLevelClaimed           bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites            []string        `json:"blocked_prerequisites"`
}

// CandidatePackagePublicationExecutorReadinessReviewEvidence records a read-only
// reviewer checklist for the blocked implementation-readiness packet. It is not
// red ceremony approval and cannot authorize implementation.
type CandidatePackagePublicationExecutorReadinessReviewEvidence struct {
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorDesignSpecRef          string          `json:"executor_design_spec_ref"`
	RedCeremonyPlanRef             string          `json:"red_ceremony_plan_ref"`
	RequiredGateRefs               []string        `json:"required_gate_refs"`
	EvidenceGateRefs               []string        `json:"evidence_gate_refs"`
	RollbackDrillRef               string          `json:"rollback_drill_ref"`
	ReviewReportRef                string          `json:"review_report_ref"`
	ChecklistItemRefs              []string        `json:"checklist_item_refs"`
	ReviewerFindingRefs            []string        `json:"reviewer_finding_refs"`
	OpenQuestionRefs               []string        `json:"open_question_refs"`
	RedCeremonyOpened              bool            `json:"red_ceremony_opened"`
	RedCeremonyApproved            bool            `json:"red_ceremony_approved"`
	ImplementationAuthorized       bool            `json:"implementation_authorized"`
	CodeSurfaceTouched             bool            `json:"code_surface_touched"`
	ImplementationReady            bool            `json:"implementation_ready"`
	ExecutorImplemented            bool            `json:"executor_implemented"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	ClaimsAppAdoption              bool            `json:"claims_app_adoption"`
	TouchesDeployedRoute           bool            `json:"touches_deployed_route"`
	ClaimsAuthSession              bool            `json:"claims_auth_session"`
	ClaimsStaging                  bool            `json:"claims_staging"`
	ClaimsVMLifecycle              bool            `json:"claims_vm_lifecycle"`
	ClaimsRunAcceptance            bool            `json:"claims_run_acceptance"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
}

// CandidatePackagePublicationExecutorReadinessReviewContract binds a read-only
// reviewer checklist to the implementation-readiness packet. It records review
// questions and findings but leaves red ceremony and implementation blocked.
type CandidatePackagePublicationExecutorReadinessReviewContract struct {
	Kind                           string          `json:"kind"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion `json:"version"`
	UsesLocalAcceptanceID          string          `json:"uses_local_acceptance_id"`
	VerifierEvidenceRef            string          `json:"verifier_evidence_ref"`
	SourceDeltaRef                 string          `json:"source_delta_ref"`
	PayloadManifestRef             string          `json:"payload_manifest_ref"`
	ReadinessReviewBoundary        string          `json:"readiness_review_boundary"`
	OwnerAuthorizationRef          string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef       string          `json:"reviewer_authorization_ref"`
	ExecutorDesignSpecRef          string          `json:"executor_design_spec_ref"`
	RedCeremonyPlanRef             string          `json:"red_ceremony_plan_ref"`
	RequiredGateRefs               []string        `json:"required_gate_refs"`
	EvidenceGateRefs               []string        `json:"evidence_gate_refs"`
	RollbackDrillRef               string          `json:"rollback_drill_ref"`
	ReviewReportRef                string          `json:"review_report_ref"`
	ChecklistItemRefs              []string        `json:"checklist_item_refs"`
	ReviewerFindingRefs            []string        `json:"reviewer_finding_refs"`
	OpenQuestionRefs               []string        `json:"open_question_refs"`
	ReadinessReviewStatus          string          `json:"readiness_review_status"`
	RedCeremonyOpened              bool            `json:"red_ceremony_opened"`
	RedCeremonyApproved            bool            `json:"red_ceremony_approved"`
	ImplementationAuthorized       bool            `json:"implementation_authorized"`
	CodeSurfaceTouched             bool            `json:"code_surface_touched"`
	ImplementationReady            bool            `json:"implementation_ready"`
	ExecutorImplemented            bool            `json:"executor_implemented"`
	ExecutorAllowed                bool            `json:"executor_allowed"`
	ActualPackagePublished         bool            `json:"actual_package_published"`
	DirectPublishReady             bool            `json:"direct_publish_ready"`
	NoMutation                     bool            `json:"no_mutation"`
	ActivationReady                bool            `json:"activation_ready"`
	PromotionLevelClaimed          bool            `json:"promotion_level_claimed"`
	BlockedPrerequisites           []string        `json:"blocked_prerequisites"`
}

// BuildCandidatePackageProductActivationVerifierContract consumes the durable
// activation contract and prerequisite evidence candidates. In this first slice,
// only package publication is allowed to become the first bindable
// prerequisite; every mutation/staging/VM/run-acceptance prerequisite remains
// blocked.
func BuildCandidatePackageProductActivationVerifierContract(durable CandidatePackageDurableActivationContract, evidence CandidatePackageProductActivationEvidence) (CandidatePackageProductActivationVerifierContract, error) {
	if err := validateDurableActivationContractForProductVerifier(durable); err != nil {
		return CandidatePackageProductActivationVerifierContract{}, err
	}
	if err := validateProductActivationEvidenceIdentity(durable, evidence); err != nil {
		return CandidatePackageProductActivationVerifierContract{}, err
	}

	result := CandidatePackageProductActivationVerifierContract{
		Kind:                           CandidatePackageProductActivationVerifierContractKind,
		CandidatePackageID:             durable.CandidatePackageID,
		CandidatePackageManifestSHA256: durable.CandidatePackageManifestSHA256,
		Version:                        durable.Version,
		UsesLocalAcceptanceID:          durable.UsesLocalAcceptanceID,
		FirstBindableStatus:            CandidatePackageProductActivationVerifierStatusBlocked,
		ActivationReady:                false,
		NoMutation:                     true,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites:           productActivationProtectedPrerequisites(),
		RequiredObservations:           canonicalObservationKinds(durable.RequiredObservations),
	}

	for _, prerequisite := range evidence.Prerequisites {
		prerequisite.Prerequisite = strings.TrimSpace(prerequisite.Prerequisite)
		prerequisite.Status = strings.TrimSpace(prerequisite.Status)
		prerequisite.EvidenceRef = strings.TrimSpace(prerequisite.EvidenceRef)
		if !validProductActivationPrerequisite(prerequisite.Prerequisite) {
			return CandidatePackageProductActivationVerifierContract{}, fmt.Errorf("candidate package product activation verifier: unsupported prerequisite %q", prerequisite.Prerequisite)
		}
		if !validProductActivationEvidenceStatus(prerequisite.Status) {
			return CandidatePackageProductActivationVerifierContract{}, fmt.Errorf("candidate package product activation verifier: unsupported evidence status %q", prerequisite.Status)
		}
		if prerequisite.Status == CandidatePackageProductActivationEvidenceStatusPassed && prerequisite.EvidenceRef == "" {
			return CandidatePackageProductActivationVerifierContract{}, fmt.Errorf("candidate package product activation verifier: passed prerequisite %q requires evidence ref", prerequisite.Prerequisite)
		}
		if prerequisite.Prerequisite != CandidatePackageProductActivationPrerequisitePackagePublication {
			continue
		}
		if prerequisite.Status == CandidatePackageProductActivationEvidenceStatusCandidate && prerequisite.EvidenceRef != "" {
			result.FirstBindablePrerequisite = prerequisite.Prerequisite
			result.FirstBindableEvidenceRef = prerequisite.EvidenceRef
			result.FirstBindableStatus = CandidatePackageProductActivationVerifierStatusBindable
			result.EvidenceRefs = canonicalStrings(append(result.EvidenceRefs, prerequisite.EvidenceRef))
		}
	}

	return result, nil
}

// BuildCandidatePackagePublicationProofContract binds the verifier-selected
// package-publication prerequisite to a pure proof object. This builder never
// publishes a package and never converts publication proof into product
// activation authority.
func BuildCandidatePackagePublicationProofContract(verifier CandidatePackageProductActivationVerifierContract, proof CandidatePackagePublicationProofEvidence) (CandidatePackagePublicationProofContract, error) {
	if err := validateProductActivationVerifierForPublicationProof(verifier); err != nil {
		return CandidatePackagePublicationProofContract{}, err
	}
	if err := validatePublicationProofEvidenceIdentity(verifier, proof); err != nil {
		return CandidatePackagePublicationProofContract{}, err
	}
	if err := validatePublicationProofEvidenceNoMutation(proof); err != nil {
		return CandidatePackagePublicationProofContract{}, err
	}

	return CandidatePackagePublicationProofContract{
		Kind:                           CandidatePackagePublicationProofContractKind,
		CandidatePackageID:             verifier.CandidatePackageID,
		CandidatePackageManifestSHA256: verifier.CandidatePackageManifestSHA256,
		Version:                        verifier.Version,
		UsesLocalAcceptanceID:          verifier.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            strings.TrimSpace(verifier.FirstBindableEvidenceRef),
		PublicationBound:               true,
		ActualPackagePublished:         false,
		NoMutation:                     true,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites: []string{
			CandidatePackageProductActivationPrerequisiteAppAdoption,
			CandidatePackageProductActivationPrerequisiteDeployedRoute,
			CandidatePackageProductActivationPrerequisiteAuthSession,
			CandidatePackageProductActivationPrerequisiteStagingAcceptance,
			CandidatePackageProductActivationPrerequisiteVMLifecycle,
			CandidatePackageProductActivationPrerequisiteRunAcceptance,
		},
	}, nil
}

// BuildCandidatePackagePublicationPayloadContract binds a package-publication
// proof to explicit source-delta and payload-manifest refs. It creates only a
// reviewable publication candidate; it never publishes, activates, promotes, or
// mutates deployed product state.
func BuildCandidatePackagePublicationPayloadContract(proof CandidatePackagePublicationProofContract, evidence CandidatePackagePublicationPayloadEvidence) (CandidatePackagePublicationPayloadContract, error) {
	if err := validatePublicationProofContractForPayload(proof); err != nil {
		return CandidatePackagePublicationPayloadContract{}, err
	}
	if err := validatePublicationPayloadEvidenceIdentity(proof, evidence); err != nil {
		return CandidatePackagePublicationPayloadContract{}, err
	}
	if err := validatePublicationPayloadEvidenceNoMutation(evidence); err != nil {
		return CandidatePackagePublicationPayloadContract{}, err
	}

	return CandidatePackagePublicationPayloadContract{
		Kind:                           CandidatePackagePublicationPayloadContractKind,
		CandidatePackageID:             proof.CandidatePackageID,
		CandidatePackageManifestSHA256: proof.CandidatePackageManifestSHA256,
		Version:                        proof.Version,
		UsesLocalAcceptanceID:          proof.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            strings.TrimSpace(proof.VerifierEvidenceRef),
		SourceDeltaRef:                 strings.TrimSpace(evidence.SourceDeltaRef),
		PayloadManifestRef:             strings.TrimSpace(evidence.PayloadManifestRef),
		PayloadBoundary:                CandidatePackagePublicationPayloadBoundary,
		ReviewablePublicationCandidate: true,
		ActualPackagePublished:         false,
		DirectPublishReady:             false,
		NoMutation:                     true,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites: []string{
			CandidatePackageProductActivationPrerequisiteAppAdoption,
			CandidatePackageProductActivationPrerequisiteDeployedRoute,
			CandidatePackageProductActivationPrerequisiteAuthSession,
			CandidatePackageProductActivationPrerequisiteStagingAcceptance,
			CandidatePackageProductActivationPrerequisiteVMLifecycle,
			CandidatePackageProductActivationPrerequisiteRunAcceptance,
		},
	}, nil
}

// BuildCandidatePackagePublicationPreflightContract records the checks that must
// be satisfied before any future package-publication executor may exist. It is a
// preflight contract only: executor permission remains false and no publication
// or activation-side mutation is performed.
func BuildCandidatePackagePublicationPreflightContract(payload CandidatePackagePublicationPayloadContract, evidence CandidatePackagePublicationPreflightEvidence) (CandidatePackagePublicationPreflightContract, error) {
	if err := validatePublicationPayloadContractForPreflight(payload); err != nil {
		return CandidatePackagePublicationPreflightContract{}, err
	}
	if err := validatePublicationPreflightEvidenceIdentity(payload, evidence); err != nil {
		return CandidatePackagePublicationPreflightContract{}, err
	}
	if err := validatePublicationPreflightEvidenceNoMutation(evidence); err != nil {
		return CandidatePackagePublicationPreflightContract{}, err
	}

	return CandidatePackagePublicationPreflightContract{
		Kind:                           CandidatePackagePublicationPreflightContractKind,
		CandidatePackageID:             payload.CandidatePackageID,
		CandidatePackageManifestSHA256: payload.CandidatePackageManifestSHA256,
		Version:                        payload.Version,
		UsesLocalAcceptanceID:          payload.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            strings.TrimSpace(payload.VerifierEvidenceRef),
		SourceDeltaRef:                 strings.TrimSpace(payload.SourceDeltaRef),
		PayloadManifestRef:             strings.TrimSpace(payload.PayloadManifestRef),
		PreflightBoundary:              CandidatePackagePublicationPreflightBoundary,
		PreflightCheckRefs:             canonicalStrings(evidence.PreflightCheckRefs),
		VerifierContractRefs:           canonicalStrings(evidence.VerifierContractRefs),
		ExecutorAllowed:                false,
		ActualPackagePublished:         false,
		DirectPublishReady:             false,
		NoMutation:                     true,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites:           productActivationProtectedPrerequisites(),
	}, nil
}

// BuildCandidatePackagePublicationExecutorReviewGateContract records the
// owner/reviewer authorization refs required before a future red executor design
// review may be considered. It is still pure review evidence: no executor is
// allowed, no package is published, and no product state is mutated.
func BuildCandidatePackagePublicationExecutorReviewGateContract(preflight CandidatePackagePublicationPreflightContract, evidence CandidatePackagePublicationExecutorReviewGateEvidence) (CandidatePackagePublicationExecutorReviewGateContract, error) {
	if err := validatePublicationPreflightContractForExecutorReviewGate(preflight); err != nil {
		return CandidatePackagePublicationExecutorReviewGateContract{}, err
	}
	if err := validatePublicationExecutorReviewGateEvidenceIdentity(preflight, evidence); err != nil {
		return CandidatePackagePublicationExecutorReviewGateContract{}, err
	}
	if err := validatePublicationExecutorReviewGateEvidenceNoMutation(evidence); err != nil {
		return CandidatePackagePublicationExecutorReviewGateContract{}, err
	}

	return CandidatePackagePublicationExecutorReviewGateContract{
		Kind:                           CandidatePackagePublicationExecutorReviewGateContractKind,
		CandidatePackageID:             preflight.CandidatePackageID,
		CandidatePackageManifestSHA256: preflight.CandidatePackageManifestSHA256,
		Version:                        preflight.Version,
		UsesLocalAcceptanceID:          preflight.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            strings.TrimSpace(preflight.VerifierEvidenceRef),
		SourceDeltaRef:                 strings.TrimSpace(preflight.SourceDeltaRef),
		PayloadManifestRef:             strings.TrimSpace(preflight.PayloadManifestRef),
		ReviewGateBoundary:             CandidatePackagePublicationExecutorReviewGateBoundary,
		PreflightCheckRefs:             canonicalStrings(preflight.PreflightCheckRefs),
		VerifierContractRefs:           canonicalStrings(preflight.VerifierContractRefs),
		OwnerAuthorizationRef:          strings.TrimSpace(evidence.OwnerAuthorizationRef),
		ReviewerAuthorizationRef:       strings.TrimSpace(evidence.ReviewerAuthorizationRef),
		ExecutorDesignReviewReady:      true,
		ExecutorAllowed:                false,
		ActualPackagePublished:         false,
		DirectPublishReady:             false,
		NoMutation:                     true,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites:           productActivationProtectedPrerequisites(),
	}, nil
}

// BuildCandidatePackagePublicationExecutorDesignSpecContract records the design
// spec and evidence refs required for a future red package-publication executor.
// It creates no executor, allows no execution, publishes no package, and mutates
// no product state.
func BuildCandidatePackagePublicationExecutorDesignSpecContract(gate CandidatePackagePublicationExecutorReviewGateContract, evidence CandidatePackagePublicationExecutorDesignSpecEvidence) (CandidatePackagePublicationExecutorDesignSpecContract, error) {
	if err := validatePublicationExecutorReviewGateForDesignSpec(gate); err != nil {
		return CandidatePackagePublicationExecutorDesignSpecContract{}, err
	}
	if err := validatePublicationExecutorDesignSpecEvidenceIdentity(gate, evidence); err != nil {
		return CandidatePackagePublicationExecutorDesignSpecContract{}, err
	}
	if err := validatePublicationExecutorDesignSpecEvidenceRefs(evidence); err != nil {
		return CandidatePackagePublicationExecutorDesignSpecContract{}, err
	}
	if err := validatePublicationExecutorDesignSpecEvidenceNoMutation(evidence); err != nil {
		return CandidatePackagePublicationExecutorDesignSpecContract{}, err
	}

	return CandidatePackagePublicationExecutorDesignSpecContract{
		Kind:                           CandidatePackagePublicationExecutorDesignSpecContractKind,
		CandidatePackageID:             gate.CandidatePackageID,
		CandidatePackageManifestSHA256: gate.CandidatePackageManifestSHA256,
		Version:                        gate.Version,
		UsesLocalAcceptanceID:          gate.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            strings.TrimSpace(gate.VerifierEvidenceRef),
		SourceDeltaRef:                 strings.TrimSpace(gate.SourceDeltaRef),
		PayloadManifestRef:             strings.TrimSpace(gate.PayloadManifestRef),
		DesignSpecBoundary:             CandidatePackagePublicationExecutorDesignSpecBoundary,
		OwnerAuthorizationRef:          strings.TrimSpace(gate.OwnerAuthorizationRef),
		ReviewerAuthorizationRef:       strings.TrimSpace(gate.ReviewerAuthorizationRef),
		ExecutorDesignSpecRef:          strings.TrimSpace(evidence.ExecutorDesignSpecRef),
		RequiredRedSurfaces:            canonicalStrings(evidence.RequiredRedSurfaces),
		RequiredEvidenceRefs:           canonicalStrings(evidence.RequiredEvidenceRefs),
		RollbackPlanRef:                strings.TrimSpace(evidence.RollbackPlanRef),
		ExecutorDesignSpecReady:        true,
		ExecutorImplemented:            false,
		ExecutorAllowed:                false,
		ActualPackagePublished:         false,
		DirectPublishReady:             false,
		NoMutation:                     true,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites:           productActivationProtectedPrerequisites(),
	}, nil
}

// BuildCandidatePackagePublicationExecutorImplementationReadinessContract records
// the red ceremony and evidence gates that must open before any future
// package-publication executor implementation may touch code. It keeps the
// implementation blocked and performs no product mutation.
func BuildCandidatePackagePublicationExecutorImplementationReadinessContract(design CandidatePackagePublicationExecutorDesignSpecContract, evidence CandidatePackagePublicationExecutorImplementationReadinessEvidence) (CandidatePackagePublicationExecutorImplementationReadinessContract, error) {
	if err := validatePublicationExecutorDesignSpecForImplementationReadiness(design); err != nil {
		return CandidatePackagePublicationExecutorImplementationReadinessContract{}, err
	}
	if err := validatePublicationExecutorImplementationReadinessEvidenceIdentity(design, evidence); err != nil {
		return CandidatePackagePublicationExecutorImplementationReadinessContract{}, err
	}
	if err := validatePublicationExecutorImplementationReadinessEvidenceRefs(evidence); err != nil {
		return CandidatePackagePublicationExecutorImplementationReadinessContract{}, err
	}
	if err := validatePublicationExecutorImplementationReadinessEvidenceNoMutation(evidence); err != nil {
		return CandidatePackagePublicationExecutorImplementationReadinessContract{}, err
	}

	return CandidatePackagePublicationExecutorImplementationReadinessContract{
		Kind:                            CandidatePackagePublicationExecutorImplementationReadinessContractKind,
		CandidatePackageID:              design.CandidatePackageID,
		CandidatePackageManifestSHA256:  design.CandidatePackageManifestSHA256,
		Version:                         design.Version,
		UsesLocalAcceptanceID:           design.UsesLocalAcceptanceID,
		VerifierEvidenceRef:             strings.TrimSpace(design.VerifierEvidenceRef),
		SourceDeltaRef:                  strings.TrimSpace(design.SourceDeltaRef),
		PayloadManifestRef:              strings.TrimSpace(design.PayloadManifestRef),
		ImplementationReadinessBoundary: CandidatePackagePublicationExecutorImplementationReadinessBoundary,
		OwnerAuthorizationRef:           strings.TrimSpace(design.OwnerAuthorizationRef),
		ReviewerAuthorizationRef:        strings.TrimSpace(design.ReviewerAuthorizationRef),
		ExecutorDesignSpecRef:           strings.TrimSpace(design.ExecutorDesignSpecRef),
		RequiredRedSurfaces:             canonicalStrings(design.RequiredRedSurfaces),
		RequiredEvidenceRefs:            canonicalStrings(design.RequiredEvidenceRefs),
		RollbackPlanRef:                 strings.TrimSpace(design.RollbackPlanRef),
		RedCeremonyPlanRef:              strings.TrimSpace(evidence.RedCeremonyPlanRef),
		RequiredGateRefs:                canonicalStrings(evidence.RequiredGateRefs),
		EvidenceGateRefs:                canonicalStrings(evidence.EvidenceGateRefs),
		RollbackDrillRef:                strings.TrimSpace(evidence.RollbackDrillRef),
		ImplementationReadinessStatus:   CandidatePackagePublicationExecutorImplementationStatusBlocked,
		RedCeremonyOpened:               false,
		CodeSurfaceTouched:              false,
		ImplementationReady:             false,
		ExecutorImplemented:             false,
		ExecutorAllowed:                 false,
		ActualPackagePublished:          false,
		DirectPublishReady:              false,
		NoMutation:                      true,
		ActivationReady:                 false,
		PromotionLevelClaimed:           false,
		BlockedPrerequisites:            productActivationProtectedPrerequisites(),
	}, nil
}

// BuildCandidatePackagePublicationExecutorReadinessReviewContract records a
// read-only reviewer checklist/report for the blocked implementation-readiness
// packet. It does not open red ceremony, authorize implementation, or publish.
func BuildCandidatePackagePublicationExecutorReadinessReviewContract(readiness CandidatePackagePublicationExecutorImplementationReadinessContract, evidence CandidatePackagePublicationExecutorReadinessReviewEvidence) (CandidatePackagePublicationExecutorReadinessReviewContract, error) {
	if err := validatePublicationExecutorImplementationReadinessForReview(readiness); err != nil {
		return CandidatePackagePublicationExecutorReadinessReviewContract{}, err
	}
	if err := validatePublicationExecutorReadinessReviewEvidenceIdentity(readiness, evidence); err != nil {
		return CandidatePackagePublicationExecutorReadinessReviewContract{}, err
	}
	if err := validatePublicationExecutorReadinessReviewEvidenceRefs(evidence); err != nil {
		return CandidatePackagePublicationExecutorReadinessReviewContract{}, err
	}
	if err := validatePublicationExecutorReadinessReviewEvidenceNoMutation(evidence); err != nil {
		return CandidatePackagePublicationExecutorReadinessReviewContract{}, err
	}

	return CandidatePackagePublicationExecutorReadinessReviewContract{
		Kind:                           CandidatePackagePublicationExecutorReadinessReviewContractKind,
		CandidatePackageID:             readiness.CandidatePackageID,
		CandidatePackageManifestSHA256: readiness.CandidatePackageManifestSHA256,
		Version:                        readiness.Version,
		UsesLocalAcceptanceID:          readiness.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            strings.TrimSpace(readiness.VerifierEvidenceRef),
		SourceDeltaRef:                 strings.TrimSpace(readiness.SourceDeltaRef),
		PayloadManifestRef:             strings.TrimSpace(readiness.PayloadManifestRef),
		ReadinessReviewBoundary:        CandidatePackagePublicationExecutorReadinessReviewBoundary,
		OwnerAuthorizationRef:          strings.TrimSpace(readiness.OwnerAuthorizationRef),
		ReviewerAuthorizationRef:       strings.TrimSpace(readiness.ReviewerAuthorizationRef),
		ExecutorDesignSpecRef:          strings.TrimSpace(readiness.ExecutorDesignSpecRef),
		RedCeremonyPlanRef:             strings.TrimSpace(readiness.RedCeremonyPlanRef),
		RequiredGateRefs:               canonicalStrings(readiness.RequiredGateRefs),
		EvidenceGateRefs:               canonicalStrings(readiness.EvidenceGateRefs),
		RollbackDrillRef:               strings.TrimSpace(readiness.RollbackDrillRef),
		ReviewReportRef:                strings.TrimSpace(evidence.ReviewReportRef),
		ChecklistItemRefs:              canonicalStrings(evidence.ChecklistItemRefs),
		ReviewerFindingRefs:            canonicalStrings(evidence.ReviewerFindingRefs),
		OpenQuestionRefs:               canonicalStrings(evidence.OpenQuestionRefs),
		ReadinessReviewStatus:          CandidatePackagePublicationExecutorReadinessReviewStatusChecklistRecorded,
		RedCeremonyOpened:              false,
		RedCeremonyApproved:            false,
		ImplementationAuthorized:       false,
		CodeSurfaceTouched:             false,
		ImplementationReady:            false,
		ExecutorImplemented:            false,
		ExecutorAllowed:                false,
		ActualPackagePublished:         false,
		DirectPublishReady:             false,
		NoMutation:                     true,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedPrerequisites:           productActivationProtectedPrerequisites(),
	}, nil
}

func validatePublicationPayloadContractForPreflight(payload CandidatePackagePublicationPayloadContract) error {
	if strings.TrimSpace(payload.Kind) != CandidatePackagePublicationPayloadContractKind {
		return fmt.Errorf("candidate package publication preflight: payload kind %q is not %q", payload.Kind, CandidatePackagePublicationPayloadContractKind)
	}
	if strings.TrimSpace(payload.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication preflight: payload package id is required")
	}
	if strings.TrimSpace(payload.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication preflight: payload package hash is required")
	}
	if !payload.Version.Valid() {
		return fmt.Errorf("candidate package publication preflight: payload version is invalid")
	}
	if strings.TrimSpace(payload.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication preflight: payload acceptance id is required")
	}
	if strings.TrimSpace(payload.VerifierEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication preflight: payload verifier evidence ref is required")
	}
	if strings.TrimSpace(payload.SourceDeltaRef) == "" {
		return fmt.Errorf("candidate package publication preflight: payload source delta ref is required")
	}
	if strings.TrimSpace(payload.PayloadManifestRef) == "" {
		return fmt.Errorf("candidate package publication preflight: payload manifest ref is required")
	}
	if !payload.ReviewablePublicationCandidate {
		return fmt.Errorf("candidate package publication preflight: payload must be a reviewable publication candidate")
	}
	if payload.ActualPackagePublished {
		return fmt.Errorf("candidate package publication preflight: payload cannot already publish a package")
	}
	if payload.DirectPublishReady {
		return fmt.Errorf("candidate package publication preflight: payload cannot be direct-publish ready")
	}
	if !payload.NoMutation {
		return fmt.Errorf("candidate package publication preflight: payload must be no-mutation")
	}
	if payload.ActivationReady {
		return fmt.Errorf("candidate package publication preflight: payload cannot already be activation-ready")
	}
	if payload.PromotionLevelClaimed {
		return fmt.Errorf("candidate package publication preflight: payload cannot claim promotion level")
	}
	return nil
}

func validatePublicationPreflightEvidenceIdentity(payload CandidatePackagePublicationPayloadContract, evidence CandidatePackagePublicationPreflightEvidence) error {
	if evidence.CandidatePackageID != payload.CandidatePackageID {
		return fmt.Errorf("candidate package publication preflight: evidence package id %q does not match payload %q", evidence.CandidatePackageID, payload.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != payload.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication preflight: evidence package hash does not match payload")
	}
	if evidence.Version != payload.Version {
		return fmt.Errorf("candidate package publication preflight: evidence version does not match payload")
	}
	if evidence.UsesLocalAcceptanceID != payload.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication preflight: evidence acceptance id does not match payload")
	}
	if strings.TrimSpace(evidence.VerifierEvidenceRef) != strings.TrimSpace(payload.VerifierEvidenceRef) {
		return fmt.Errorf("candidate package publication preflight: evidence verifier ref %q does not match payload verifier ref %q", evidence.VerifierEvidenceRef, payload.VerifierEvidenceRef)
	}
	if strings.TrimSpace(evidence.SourceDeltaRef) != strings.TrimSpace(payload.SourceDeltaRef) {
		return fmt.Errorf("candidate package publication preflight: evidence source delta ref %q does not match payload source delta ref %q", evidence.SourceDeltaRef, payload.SourceDeltaRef)
	}
	if strings.TrimSpace(evidence.PayloadManifestRef) != strings.TrimSpace(payload.PayloadManifestRef) {
		return fmt.Errorf("candidate package publication preflight: evidence payload manifest ref %q does not match payload manifest ref %q", evidence.PayloadManifestRef, payload.PayloadManifestRef)
	}
	if len(canonicalStrings(evidence.PreflightCheckRefs)) == 0 {
		return fmt.Errorf("candidate package publication preflight: preflight check refs are required")
	}
	return nil
}

func validatePublicationPreflightEvidenceNoMutation(evidence CandidatePackagePublicationPreflightEvidence) error {
	switch {
	case evidence.ExecutorAllowed:
		return fmt.Errorf("candidate package publication preflight: evidence cannot allow executor")
	case evidence.ActualPackagePublished:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim actual package publication")
	case evidence.DirectPublishReady:
		return fmt.Errorf("candidate package publication preflight: evidence cannot be direct-publish ready")
	case evidence.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim AppAdoption mutation")
	case evidence.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim deployed route mutation")
	case evidence.ClaimsAuthSession:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim auth session mutation")
	case evidence.ClaimsStaging:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim staging")
	case evidence.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim VM lifecycle")
	case evidence.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim run acceptance")
	case evidence.ActivationReady:
		return fmt.Errorf("candidate package publication preflight: evidence cannot mark activation ready")
	case evidence.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication preflight: evidence cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationPreflightContractForExecutorReviewGate(preflight CandidatePackagePublicationPreflightContract) error {
	if strings.TrimSpace(preflight.Kind) != CandidatePackagePublicationPreflightContractKind {
		return fmt.Errorf("candidate package publication executor review gate: preflight kind %q is not %q", preflight.Kind, CandidatePackagePublicationPreflightContractKind)
	}
	if strings.TrimSpace(preflight.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication executor review gate: preflight package id is required")
	}
	if strings.TrimSpace(preflight.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication executor review gate: preflight package hash is required")
	}
	if !preflight.Version.Valid() {
		return fmt.Errorf("candidate package publication executor review gate: preflight version is invalid")
	}
	if strings.TrimSpace(preflight.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication executor review gate: preflight acceptance id is required")
	}
	if strings.TrimSpace(preflight.VerifierEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication executor review gate: preflight verifier evidence ref is required")
	}
	if strings.TrimSpace(preflight.SourceDeltaRef) == "" {
		return fmt.Errorf("candidate package publication executor review gate: preflight source delta ref is required")
	}
	if strings.TrimSpace(preflight.PayloadManifestRef) == "" {
		return fmt.Errorf("candidate package publication executor review gate: preflight payload manifest ref is required")
	}
	if strings.TrimSpace(preflight.PreflightBoundary) != CandidatePackagePublicationPreflightBoundary {
		return fmt.Errorf("candidate package publication executor review gate: preflight boundary %q is not %q", preflight.PreflightBoundary, CandidatePackagePublicationPreflightBoundary)
	}
	if len(canonicalStrings(preflight.PreflightCheckRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor review gate: preflight check refs are required")
	}
	switch {
	case preflight.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor review gate: preflight cannot allow executor")
	case preflight.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor review gate: preflight cannot claim actual package publication")
	case preflight.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor review gate: preflight cannot be direct-publish ready")
	case !preflight.NoMutation:
		return fmt.Errorf("candidate package publication executor review gate: preflight must be no-mutation")
	case preflight.ActivationReady:
		return fmt.Errorf("candidate package publication executor review gate: preflight cannot mark activation ready")
	case preflight.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor review gate: preflight cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorReviewGateEvidenceIdentity(preflight CandidatePackagePublicationPreflightContract, evidence CandidatePackagePublicationExecutorReviewGateEvidence) error {
	if evidence.CandidatePackageID != preflight.CandidatePackageID {
		return fmt.Errorf("candidate package publication executor review gate: evidence package id %q does not match preflight %q", evidence.CandidatePackageID, preflight.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != preflight.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication executor review gate: evidence package hash does not match preflight")
	}
	if evidence.Version != preflight.Version {
		return fmt.Errorf("candidate package publication executor review gate: evidence version does not match preflight")
	}
	if evidence.UsesLocalAcceptanceID != preflight.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication executor review gate: evidence acceptance id does not match preflight")
	}
	if strings.TrimSpace(evidence.VerifierEvidenceRef) != strings.TrimSpace(preflight.VerifierEvidenceRef) {
		return fmt.Errorf("candidate package publication executor review gate: evidence verifier ref %q does not match preflight verifier ref %q", evidence.VerifierEvidenceRef, preflight.VerifierEvidenceRef)
	}
	if strings.TrimSpace(evidence.SourceDeltaRef) != strings.TrimSpace(preflight.SourceDeltaRef) {
		return fmt.Errorf("candidate package publication executor review gate: evidence source delta ref %q does not match preflight source delta ref %q", evidence.SourceDeltaRef, preflight.SourceDeltaRef)
	}
	if strings.TrimSpace(evidence.PayloadManifestRef) != strings.TrimSpace(preflight.PayloadManifestRef) {
		return fmt.Errorf("candidate package publication executor review gate: evidence payload manifest ref %q does not match preflight payload manifest ref %q", evidence.PayloadManifestRef, preflight.PayloadManifestRef)
	}
	if !sameCanonicalStrings(evidence.PreflightCheckRefs, preflight.PreflightCheckRefs) {
		return fmt.Errorf("candidate package publication executor review gate: evidence preflight check refs do not match preflight")
	}
	if !sameCanonicalStrings(evidence.VerifierContractRefs, preflight.VerifierContractRefs) {
		return fmt.Errorf("candidate package publication executor review gate: evidence verifier contract refs do not match preflight")
	}
	if strings.TrimSpace(evidence.OwnerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor review gate: owner authorization ref is required")
	}
	if strings.TrimSpace(evidence.ReviewerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor review gate: reviewer authorization ref is required")
	}
	return nil
}

func validatePublicationExecutorReviewGateEvidenceNoMutation(evidence CandidatePackagePublicationExecutorReviewGateEvidence) error {
	switch {
	case evidence.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot allow executor")
	case evidence.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim actual package publication")
	case evidence.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot be direct-publish ready")
	case evidence.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim AppAdoption mutation")
	case evidence.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim deployed route mutation")
	case evidence.ClaimsAuthSession:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim auth session mutation")
	case evidence.ClaimsStaging:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim staging")
	case evidence.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim VM lifecycle")
	case evidence.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim run acceptance")
	case evidence.ActivationReady:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot mark activation ready")
	case evidence.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor review gate: evidence cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorReviewGateForDesignSpec(gate CandidatePackagePublicationExecutorReviewGateContract) error {
	if strings.TrimSpace(gate.Kind) != CandidatePackagePublicationExecutorReviewGateContractKind {
		return fmt.Errorf("candidate package publication executor design spec: review gate kind %q is not %q", gate.Kind, CandidatePackagePublicationExecutorReviewGateContractKind)
	}
	if strings.TrimSpace(gate.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate package id is required")
	}
	if strings.TrimSpace(gate.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate package hash is required")
	}
	if !gate.Version.Valid() {
		return fmt.Errorf("candidate package publication executor design spec: review gate version is invalid")
	}
	if strings.TrimSpace(gate.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate acceptance id is required")
	}
	if strings.TrimSpace(gate.VerifierEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate verifier evidence ref is required")
	}
	if strings.TrimSpace(gate.SourceDeltaRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate source delta ref is required")
	}
	if strings.TrimSpace(gate.PayloadManifestRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate payload manifest ref is required")
	}
	if strings.TrimSpace(gate.ReviewGateBoundary) != CandidatePackagePublicationExecutorReviewGateBoundary {
		return fmt.Errorf("candidate package publication executor design spec: review gate boundary %q is not %q", gate.ReviewGateBoundary, CandidatePackagePublicationExecutorReviewGateBoundary)
	}
	if strings.TrimSpace(gate.OwnerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate owner authorization ref is required")
	}
	if strings.TrimSpace(gate.ReviewerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: review gate reviewer authorization ref is required")
	}
	if !gate.ExecutorDesignReviewReady {
		return fmt.Errorf("candidate package publication executor design spec: review gate must be design-review-ready")
	}
	switch {
	case gate.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor design spec: review gate cannot allow executor")
	case gate.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor design spec: review gate cannot claim actual package publication")
	case gate.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor design spec: review gate cannot be direct-publish ready")
	case !gate.NoMutation:
		return fmt.Errorf("candidate package publication executor design spec: review gate must be no-mutation")
	case gate.ActivationReady:
		return fmt.Errorf("candidate package publication executor design spec: review gate cannot mark activation ready")
	case gate.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor design spec: review gate cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorDesignSpecEvidenceIdentity(gate CandidatePackagePublicationExecutorReviewGateContract, evidence CandidatePackagePublicationExecutorDesignSpecEvidence) error {
	if evidence.CandidatePackageID != gate.CandidatePackageID {
		return fmt.Errorf("candidate package publication executor design spec: evidence package id %q does not match review gate %q", evidence.CandidatePackageID, gate.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != gate.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication executor design spec: evidence package hash does not match review gate")
	}
	if evidence.Version != gate.Version {
		return fmt.Errorf("candidate package publication executor design spec: evidence version does not match review gate")
	}
	if evidence.UsesLocalAcceptanceID != gate.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication executor design spec: evidence acceptance id does not match review gate")
	}
	if strings.TrimSpace(evidence.VerifierEvidenceRef) != strings.TrimSpace(gate.VerifierEvidenceRef) {
		return fmt.Errorf("candidate package publication executor design spec: evidence verifier ref %q does not match review gate verifier ref %q", evidence.VerifierEvidenceRef, gate.VerifierEvidenceRef)
	}
	if strings.TrimSpace(evidence.SourceDeltaRef) != strings.TrimSpace(gate.SourceDeltaRef) {
		return fmt.Errorf("candidate package publication executor design spec: evidence source delta ref %q does not match review gate source delta ref %q", evidence.SourceDeltaRef, gate.SourceDeltaRef)
	}
	if strings.TrimSpace(evidence.PayloadManifestRef) != strings.TrimSpace(gate.PayloadManifestRef) {
		return fmt.Errorf("candidate package publication executor design spec: evidence payload manifest ref %q does not match review gate payload manifest ref %q", evidence.PayloadManifestRef, gate.PayloadManifestRef)
	}
	if strings.TrimSpace(evidence.OwnerAuthorizationRef) != strings.TrimSpace(gate.OwnerAuthorizationRef) {
		return fmt.Errorf("candidate package publication executor design spec: evidence owner authorization ref does not match review gate")
	}
	if strings.TrimSpace(evidence.ReviewerAuthorizationRef) != strings.TrimSpace(gate.ReviewerAuthorizationRef) {
		return fmt.Errorf("candidate package publication executor design spec: evidence reviewer authorization ref does not match review gate")
	}
	return nil
}

func validatePublicationExecutorDesignSpecEvidenceRefs(evidence CandidatePackagePublicationExecutorDesignSpecEvidence) error {
	if strings.TrimSpace(evidence.ExecutorDesignSpecRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: executor design spec ref is required")
	}
	if len(canonicalStrings(evidence.RequiredEvidenceRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor design spec: required evidence refs are required")
	}
	if strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("candidate package publication executor design spec: rollback plan ref is required")
	}
	surfaces := canonicalStrings(evidence.RequiredRedSurfaces)
	if len(surfaces) == 0 {
		return fmt.Errorf("candidate package publication executor design spec: required red surfaces are required")
	}
	for _, surface := range surfaces {
		if !validPublicationExecutorRedSurface(surface) {
			return fmt.Errorf("candidate package publication executor design spec: unsupported red surface %q", surface)
		}
	}
	for _, surface := range publicationExecutorRequiredRedSurfaces() {
		if !stringSliceContains(surfaces, surface) {
			return fmt.Errorf("candidate package publication executor design spec: required red surface %q is missing", surface)
		}
	}
	return nil
}

func validatePublicationExecutorDesignSpecEvidenceNoMutation(evidence CandidatePackagePublicationExecutorDesignSpecEvidence) error {
	switch {
	case evidence.ExecutorImplemented:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot implement executor")
	case evidence.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot allow executor")
	case evidence.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim actual package publication")
	case evidence.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot be direct-publish ready")
	case evidence.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim AppAdoption mutation")
	case evidence.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim deployed route mutation")
	case evidence.ClaimsAuthSession:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim auth session mutation")
	case evidence.ClaimsStaging:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim staging")
	case evidence.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim VM lifecycle")
	case evidence.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim run acceptance")
	case evidence.ActivationReady:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot mark activation ready")
	case evidence.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor design spec: evidence cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorDesignSpecForImplementationReadiness(design CandidatePackagePublicationExecutorDesignSpecContract) error {
	if strings.TrimSpace(design.Kind) != CandidatePackagePublicationExecutorDesignSpecContractKind {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec kind %q is not %q", design.Kind, CandidatePackagePublicationExecutorDesignSpecContractKind)
	}
	if strings.TrimSpace(design.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec package id is required")
	}
	if strings.TrimSpace(design.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec package hash is required")
	}
	if !design.Version.Valid() {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec version is invalid")
	}
	if strings.TrimSpace(design.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec acceptance id is required")
	}
	if strings.TrimSpace(design.VerifierEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec verifier evidence ref is required")
	}
	if strings.TrimSpace(design.SourceDeltaRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec source delta ref is required")
	}
	if strings.TrimSpace(design.PayloadManifestRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec payload manifest ref is required")
	}
	if strings.TrimSpace(design.DesignSpecBoundary) != CandidatePackagePublicationExecutorDesignSpecBoundary {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec boundary %q is not %q", design.DesignSpecBoundary, CandidatePackagePublicationExecutorDesignSpecBoundary)
	}
	if strings.TrimSpace(design.OwnerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec owner authorization ref is required")
	}
	if strings.TrimSpace(design.ReviewerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec reviewer authorization ref is required")
	}
	if strings.TrimSpace(design.ExecutorDesignSpecRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec ref is required")
	}
	if len(canonicalStrings(design.RequiredRedSurfaces)) == 0 {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec required red surfaces are required")
	}
	if len(canonicalStrings(design.RequiredEvidenceRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec required evidence refs are required")
	}
	if strings.TrimSpace(design.RollbackPlanRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec rollback plan ref is required")
	}
	if !design.ExecutorDesignSpecReady {
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec must be ready")
	}
	switch {
	case design.ExecutorImplemented:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec cannot implement executor")
	case design.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec cannot allow executor")
	case design.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec cannot claim actual package publication")
	case design.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec cannot be direct-publish ready")
	case !design.NoMutation:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec must be no-mutation")
	case design.ActivationReady:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec cannot mark activation ready")
	case design.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor implementation readiness: design spec cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorImplementationReadinessEvidenceIdentity(design CandidatePackagePublicationExecutorDesignSpecContract, evidence CandidatePackagePublicationExecutorImplementationReadinessEvidence) error {
	if evidence.CandidatePackageID != design.CandidatePackageID {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence package id %q does not match design spec %q", evidence.CandidatePackageID, design.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != design.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence package hash does not match design spec")
	}
	if evidence.Version != design.Version {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence version does not match design spec")
	}
	if evidence.UsesLocalAcceptanceID != design.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence acceptance id does not match design spec")
	}
	if strings.TrimSpace(evidence.VerifierEvidenceRef) != strings.TrimSpace(design.VerifierEvidenceRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence verifier ref %q does not match design spec verifier ref %q", evidence.VerifierEvidenceRef, design.VerifierEvidenceRef)
	}
	if strings.TrimSpace(evidence.SourceDeltaRef) != strings.TrimSpace(design.SourceDeltaRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence source delta ref %q does not match design spec source delta ref %q", evidence.SourceDeltaRef, design.SourceDeltaRef)
	}
	if strings.TrimSpace(evidence.PayloadManifestRef) != strings.TrimSpace(design.PayloadManifestRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence payload manifest ref %q does not match design spec payload manifest ref %q", evidence.PayloadManifestRef, design.PayloadManifestRef)
	}
	if strings.TrimSpace(evidence.OwnerAuthorizationRef) != strings.TrimSpace(design.OwnerAuthorizationRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence owner authorization ref does not match design spec")
	}
	if strings.TrimSpace(evidence.ReviewerAuthorizationRef) != strings.TrimSpace(design.ReviewerAuthorizationRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence reviewer authorization ref does not match design spec")
	}
	if strings.TrimSpace(evidence.ExecutorDesignSpecRef) != strings.TrimSpace(design.ExecutorDesignSpecRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence executor design spec ref does not match design spec")
	}
	if !sameCanonicalStrings(evidence.RequiredRedSurfaces, design.RequiredRedSurfaces) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence required red surfaces do not match design spec")
	}
	if !sameCanonicalStrings(evidence.RequiredEvidenceRefs, design.RequiredEvidenceRefs) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence required evidence refs do not match design spec")
	}
	if strings.TrimSpace(evidence.RollbackPlanRef) != strings.TrimSpace(design.RollbackPlanRef) {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence rollback plan ref does not match design spec")
	}
	return nil
}

func validatePublicationExecutorImplementationReadinessEvidenceRefs(evidence CandidatePackagePublicationExecutorImplementationReadinessEvidence) error {
	if strings.TrimSpace(evidence.RedCeremonyPlanRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: red ceremony plan ref is required")
	}
	if len(canonicalStrings(evidence.EvidenceGateRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence gate refs are required")
	}
	if strings.TrimSpace(evidence.RollbackDrillRef) == "" {
		return fmt.Errorf("candidate package publication executor implementation readiness: rollback drill ref is required")
	}
	gates := canonicalStrings(evidence.RequiredGateRefs)
	if len(gates) == 0 {
		return fmt.Errorf("candidate package publication executor implementation readiness: required gate refs are required")
	}
	for _, gate := range gates {
		if !validPublicationExecutorImplementationGate(gate) {
			return fmt.Errorf("candidate package publication executor implementation readiness: unsupported implementation gate %q", gate)
		}
	}
	for _, gate := range publicationExecutorRequiredImplementationGates() {
		if !stringSliceContains(gates, gate) {
			return fmt.Errorf("candidate package publication executor implementation readiness: required implementation gate %q is missing", gate)
		}
	}
	return nil
}

func validatePublicationExecutorImplementationReadinessEvidenceNoMutation(evidence CandidatePackagePublicationExecutorImplementationReadinessEvidence) error {
	switch {
	case evidence.RedCeremonyOpened:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot open red ceremony")
	case evidence.CodeSurfaceTouched:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot touch code surface")
	case evidence.ImplementationReady:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot mark implementation ready")
	case evidence.ExecutorImplemented:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot implement executor")
	case evidence.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot allow executor")
	case evidence.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim actual package publication")
	case evidence.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot be direct-publish ready")
	case evidence.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim AppAdoption mutation")
	case evidence.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim deployed route mutation")
	case evidence.ClaimsAuthSession:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim auth session mutation")
	case evidence.ClaimsStaging:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim staging")
	case evidence.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim VM lifecycle")
	case evidence.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim run acceptance")
	case evidence.ActivationReady:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot mark activation ready")
	case evidence.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor implementation readiness: evidence cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorImplementationReadinessForReview(readiness CandidatePackagePublicationExecutorImplementationReadinessContract) error {
	if strings.TrimSpace(readiness.Kind) != CandidatePackagePublicationExecutorImplementationReadinessContractKind {
		return fmt.Errorf("candidate package publication executor readiness review: readiness kind %q is not %q", readiness.Kind, CandidatePackagePublicationExecutorImplementationReadinessContractKind)
	}
	if strings.TrimSpace(readiness.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness package id is required")
	}
	if strings.TrimSpace(readiness.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness package hash is required")
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("candidate package publication executor readiness review: readiness version is invalid")
	}
	if strings.TrimSpace(readiness.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness acceptance id is required")
	}
	if strings.TrimSpace(readiness.VerifierEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness verifier evidence ref is required")
	}
	if strings.TrimSpace(readiness.SourceDeltaRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness source delta ref is required")
	}
	if strings.TrimSpace(readiness.PayloadManifestRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness payload manifest ref is required")
	}
	if strings.TrimSpace(readiness.ImplementationReadinessBoundary) != CandidatePackagePublicationExecutorImplementationReadinessBoundary {
		return fmt.Errorf("candidate package publication executor readiness review: readiness boundary %q is not %q", readiness.ImplementationReadinessBoundary, CandidatePackagePublicationExecutorImplementationReadinessBoundary)
	}
	if strings.TrimSpace(readiness.OwnerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness owner authorization ref is required")
	}
	if strings.TrimSpace(readiness.ReviewerAuthorizationRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness reviewer authorization ref is required")
	}
	if strings.TrimSpace(readiness.ExecutorDesignSpecRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness executor design spec ref is required")
	}
	if len(canonicalStrings(readiness.RequiredRedSurfaces)) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: readiness required red surfaces are required")
	}
	if len(canonicalStrings(readiness.RequiredEvidenceRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: readiness required evidence refs are required")
	}
	if strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness rollback plan ref is required")
	}
	if strings.TrimSpace(readiness.RedCeremonyPlanRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness red ceremony plan ref is required")
	}
	if len(canonicalStrings(readiness.RequiredGateRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: readiness required gate refs are required")
	}
	if len(canonicalStrings(readiness.EvidenceGateRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: readiness evidence gate refs are required")
	}
	if strings.TrimSpace(readiness.RollbackDrillRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: readiness rollback drill ref is required")
	}
	if readiness.ImplementationReadinessStatus != CandidatePackagePublicationExecutorImplementationStatusBlocked {
		return fmt.Errorf("candidate package publication executor readiness review: readiness status %q is not %q", readiness.ImplementationReadinessStatus, CandidatePackagePublicationExecutorImplementationStatusBlocked)
	}
	switch {
	case readiness.RedCeremonyOpened:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot have opened red ceremony")
	case readiness.CodeSurfaceTouched:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot have touched code surface")
	case readiness.ImplementationReady:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot mark implementation ready")
	case readiness.ExecutorImplemented:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot implement executor")
	case readiness.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot allow executor")
	case readiness.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot claim actual package publication")
	case readiness.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot be direct-publish ready")
	case !readiness.NoMutation:
		return fmt.Errorf("candidate package publication executor readiness review: readiness must be no-mutation")
	case readiness.ActivationReady:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot mark activation ready")
	case readiness.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor readiness review: readiness cannot claim promotion level")
	default:
		return nil
	}
}

func validatePublicationExecutorReadinessReviewEvidenceIdentity(readiness CandidatePackagePublicationExecutorImplementationReadinessContract, evidence CandidatePackagePublicationExecutorReadinessReviewEvidence) error {
	if evidence.CandidatePackageID != readiness.CandidatePackageID {
		return fmt.Errorf("candidate package publication executor readiness review: evidence package id %q does not match readiness %q", evidence.CandidatePackageID, readiness.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != readiness.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication executor readiness review: evidence package hash does not match readiness")
	}
	if evidence.Version != readiness.Version {
		return fmt.Errorf("candidate package publication executor readiness review: evidence version does not match readiness")
	}
	if evidence.UsesLocalAcceptanceID != readiness.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication executor readiness review: evidence acceptance id does not match readiness")
	}
	if strings.TrimSpace(evidence.VerifierEvidenceRef) != strings.TrimSpace(readiness.VerifierEvidenceRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence verifier ref %q does not match readiness verifier ref %q", evidence.VerifierEvidenceRef, readiness.VerifierEvidenceRef)
	}
	if strings.TrimSpace(evidence.SourceDeltaRef) != strings.TrimSpace(readiness.SourceDeltaRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence source delta ref %q does not match readiness source delta ref %q", evidence.SourceDeltaRef, readiness.SourceDeltaRef)
	}
	if strings.TrimSpace(evidence.PayloadManifestRef) != strings.TrimSpace(readiness.PayloadManifestRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence payload manifest ref %q does not match readiness payload manifest ref %q", evidence.PayloadManifestRef, readiness.PayloadManifestRef)
	}
	if strings.TrimSpace(evidence.OwnerAuthorizationRef) != strings.TrimSpace(readiness.OwnerAuthorizationRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence owner authorization ref does not match readiness")
	}
	if strings.TrimSpace(evidence.ReviewerAuthorizationRef) != strings.TrimSpace(readiness.ReviewerAuthorizationRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence reviewer authorization ref does not match readiness")
	}
	if strings.TrimSpace(evidence.ExecutorDesignSpecRef) != strings.TrimSpace(readiness.ExecutorDesignSpecRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence executor design spec ref does not match readiness")
	}
	if strings.TrimSpace(evidence.RedCeremonyPlanRef) != strings.TrimSpace(readiness.RedCeremonyPlanRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence red ceremony plan ref does not match readiness")
	}
	if !sameCanonicalStrings(evidence.RequiredGateRefs, readiness.RequiredGateRefs) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence required gate refs do not match readiness")
	}
	if !sameCanonicalStrings(evidence.EvidenceGateRefs, readiness.EvidenceGateRefs) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence gate refs do not match readiness")
	}
	if strings.TrimSpace(evidence.RollbackDrillRef) != strings.TrimSpace(readiness.RollbackDrillRef) {
		return fmt.Errorf("candidate package publication executor readiness review: evidence rollback drill ref does not match readiness")
	}
	return nil
}

func validatePublicationExecutorReadinessReviewEvidenceRefs(evidence CandidatePackagePublicationExecutorReadinessReviewEvidence) error {
	if strings.TrimSpace(evidence.ReviewReportRef) == "" {
		return fmt.Errorf("candidate package publication executor readiness review: review report ref is required")
	}
	if len(canonicalStrings(evidence.ReviewerFindingRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: reviewer finding refs are required")
	}
	if len(canonicalStrings(evidence.OpenQuestionRefs)) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: open question refs are required")
	}
	items := canonicalStrings(evidence.ChecklistItemRefs)
	if len(items) == 0 {
		return fmt.Errorf("candidate package publication executor readiness review: checklist item refs are required")
	}
	for _, item := range items {
		if !validPublicationExecutorReadinessReviewItem(item) {
			return fmt.Errorf("candidate package publication executor readiness review: unsupported checklist item %q", item)
		}
	}
	for _, required := range publicationExecutorReadinessReviewRequiredChecklistItems() {
		if !stringSliceContains(items, required) {
			return fmt.Errorf("candidate package publication executor readiness review: checklist item %q is missing", required)
		}
	}
	return nil
}

func validatePublicationExecutorReadinessReviewEvidenceNoMutation(evidence CandidatePackagePublicationExecutorReadinessReviewEvidence) error {
	switch {
	case evidence.RedCeremonyOpened:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot open red ceremony")
	case evidence.RedCeremonyApproved:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot approve red ceremony")
	case evidence.ImplementationAuthorized:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot authorize implementation")
	case evidence.CodeSurfaceTouched:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot touch code surface")
	case evidence.ImplementationReady:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot mark implementation ready")
	case evidence.ExecutorImplemented:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot implement executor")
	case evidence.ExecutorAllowed:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot allow executor")
	case evidence.ActualPackagePublished:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim actual package publication")
	case evidence.DirectPublishReady:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot be direct-publish ready")
	case evidence.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim AppAdoption mutation")
	case evidence.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim deployed route mutation")
	case evidence.ClaimsAuthSession:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim auth session mutation")
	case evidence.ClaimsStaging:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim staging")
	case evidence.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim VM lifecycle")
	case evidence.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim run acceptance")
	case evidence.ActivationReady:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot mark activation ready")
	case evidence.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication executor readiness review: evidence cannot claim promotion level")
	default:
		return nil
	}
}
func validatePublicationProofContractForPayload(proof CandidatePackagePublicationProofContract) error {
	if strings.TrimSpace(proof.Kind) != CandidatePackagePublicationProofContractKind {

		return fmt.Errorf("candidate package publication payload: proof kind %q is not %q", proof.Kind, CandidatePackagePublicationProofContractKind)
	}
	if strings.TrimSpace(proof.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication payload: proof package id is required")
	}
	if strings.TrimSpace(proof.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication payload: proof package hash is required")
	}
	if !proof.Version.Valid() {
		return fmt.Errorf("candidate package publication payload: proof version is invalid")
	}
	if strings.TrimSpace(proof.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication payload: proof acceptance id is required")
	}
	if strings.TrimSpace(proof.VerifierEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication payload: proof verifier evidence ref is required")
	}
	if !proof.PublicationBound {
		return fmt.Errorf("candidate package publication payload: proof must be publication-bound")
	}
	if proof.ActualPackagePublished {
		return fmt.Errorf("candidate package publication payload: proof cannot already publish a package")
	}
	if !proof.NoMutation {
		return fmt.Errorf("candidate package publication payload: proof must be no-mutation")
	}
	if proof.ActivationReady {
		return fmt.Errorf("candidate package publication payload: proof cannot already be activation-ready")
	}
	if proof.PromotionLevelClaimed {
		return fmt.Errorf("candidate package publication payload: proof cannot claim promotion level")
	}
	return nil
}

func validatePublicationPayloadEvidenceIdentity(proof CandidatePackagePublicationProofContract, evidence CandidatePackagePublicationPayloadEvidence) error {
	if evidence.CandidatePackageID != proof.CandidatePackageID {
		return fmt.Errorf("candidate package publication payload: evidence package id %q does not match proof %q", evidence.CandidatePackageID, proof.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != proof.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication payload: evidence package hash does not match proof")
	}
	if evidence.Version != proof.Version {
		return fmt.Errorf("candidate package publication payload: evidence version does not match proof")
	}
	if evidence.UsesLocalAcceptanceID != proof.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication payload: evidence acceptance id does not match proof")
	}
	if strings.TrimSpace(evidence.VerifierEvidenceRef) != strings.TrimSpace(proof.VerifierEvidenceRef) {
		return fmt.Errorf("candidate package publication payload: evidence verifier ref %q does not match proof verifier ref %q", evidence.VerifierEvidenceRef, proof.VerifierEvidenceRef)
	}
	if strings.TrimSpace(evidence.SourceDeltaRef) == "" {
		return fmt.Errorf("candidate package publication payload: source delta ref is required")
	}
	if strings.TrimSpace(evidence.PayloadManifestRef) == "" {
		return fmt.Errorf("candidate package publication payload: payload manifest ref is required")
	}
	return nil
}

func validatePublicationPayloadEvidenceNoMutation(evidence CandidatePackagePublicationPayloadEvidence) error {
	switch {
	case evidence.ActualPackagePublished:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim actual package publication")
	case evidence.DirectPublishReady:
		return fmt.Errorf("candidate package publication payload: evidence cannot be direct-publish ready")
	case evidence.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim AppAdoption mutation")
	case evidence.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim deployed route mutation")
	case evidence.ClaimsAuthSession:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim auth session mutation")
	case evidence.ClaimsStaging:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim staging")
	case evidence.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim VM lifecycle")
	case evidence.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim run acceptance")
	case evidence.ActivationReady:
		return fmt.Errorf("candidate package publication payload: evidence cannot mark activation ready")
	case evidence.PromotionLevelClaimed:
		return fmt.Errorf("candidate package publication payload: evidence cannot claim promotion level")
	default:
		return nil
	}
}

func validateProductActivationVerifierForPublicationProof(verifier CandidatePackageProductActivationVerifierContract) error {
	if strings.TrimSpace(verifier.Kind) != CandidatePackageProductActivationVerifierContractKind {
		return fmt.Errorf("candidate package publication proof: verifier kind %q is not %q", verifier.Kind, CandidatePackageProductActivationVerifierContractKind)
	}
	if strings.TrimSpace(verifier.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package publication proof: verifier package id is required")
	}
	if strings.TrimSpace(verifier.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package publication proof: verifier package hash is required")
	}
	if !verifier.Version.Valid() {
		return fmt.Errorf("candidate package publication proof: verifier version is invalid")
	}
	if strings.TrimSpace(verifier.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package publication proof: verifier acceptance id is required")
	}
	if verifier.FirstBindablePrerequisite != CandidatePackageProductActivationPrerequisitePackagePublication {

		return fmt.Errorf("candidate package publication proof: verifier first bindable prerequisite %q is not %q", verifier.FirstBindablePrerequisite, CandidatePackageProductActivationPrerequisitePackagePublication)
	}
	if verifier.FirstBindableStatus != CandidatePackageProductActivationVerifierStatusBindable {
		return fmt.Errorf("candidate package publication proof: verifier first bindable status %q is not %q", verifier.FirstBindableStatus, CandidatePackageProductActivationVerifierStatusBindable)
	}
	if strings.TrimSpace(verifier.FirstBindableEvidenceRef) == "" {
		return fmt.Errorf("candidate package publication proof: verifier evidence ref is required")
	}
	if verifier.ActivationReady {
		return fmt.Errorf("candidate package publication proof: verifier cannot already be activation-ready")
	}
	if !verifier.NoMutation {
		return fmt.Errorf("candidate package publication proof: verifier must be no-mutation")
	}
	if verifier.PromotionLevelClaimed {
		return fmt.Errorf("candidate package publication proof: verifier cannot claim promotion level")
	}
	return nil
}

func validatePublicationProofEvidenceIdentity(verifier CandidatePackageProductActivationVerifierContract, proof CandidatePackagePublicationProofEvidence) error {
	if proof.CandidatePackageID != verifier.CandidatePackageID {
		return fmt.Errorf("candidate package publication proof: proof package id %q does not match verifier %q", proof.CandidatePackageID, verifier.CandidatePackageID)
	}
	if proof.CandidatePackageManifestSHA256 != verifier.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package publication proof: proof package hash does not match verifier")
	}
	if proof.Version != verifier.Version {
		return fmt.Errorf("candidate package publication proof: proof version does not match verifier")
	}
	if proof.UsesLocalAcceptanceID != verifier.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package publication proof: proof acceptance id does not match verifier")
	}
	if strings.TrimSpace(proof.EvidenceRef) != strings.TrimSpace(verifier.FirstBindableEvidenceRef) {
		return fmt.Errorf("candidate package publication proof: proof evidence ref %q does not match verifier evidence ref %q", proof.EvidenceRef, verifier.FirstBindableEvidenceRef)
	}
	return nil
}

func validatePublicationProofEvidenceNoMutation(proof CandidatePackagePublicationProofEvidence) error {
	switch {
	case proof.ActualPackagePublished:
		return fmt.Errorf("candidate package publication proof: proof cannot claim actual package publication")
	case proof.ClaimsAppAdoption:
		return fmt.Errorf("candidate package publication proof: proof cannot claim AppAdoption mutation")
	case proof.TouchesDeployedRoute:
		return fmt.Errorf("candidate package publication proof: proof cannot claim deployed route mutation")
	case proof.ClaimsStaging:
		return fmt.Errorf("candidate package publication proof: proof cannot claim staging")
	case proof.ClaimsVMLifecycle:
		return fmt.Errorf("candidate package publication proof: proof cannot claim VM lifecycle")
	case proof.ClaimsRunAcceptance:
		return fmt.Errorf("candidate package publication proof: proof cannot claim run acceptance")
	default:
		return nil
	}
}

func validateDurableActivationContractForProductVerifier(durable CandidatePackageDurableActivationContract) error {
	if strings.TrimSpace(durable.Kind) != CandidatePackageDurableActivationContractKind {
		return fmt.Errorf("candidate package product activation verifier: durable contract kind %q is not %q", durable.Kind, CandidatePackageDurableActivationContractKind)
	}
	if strings.TrimSpace(durable.CandidatePackageID) == "" {
		return fmt.Errorf("candidate package product activation verifier: durable contract package id is required")
	}
	if strings.TrimSpace(durable.CandidatePackageManifestSHA256) == "" {
		return fmt.Errorf("candidate package product activation verifier: durable contract package hash is required")
	}
	if !durable.Version.Valid() {
		return fmt.Errorf("candidate package product activation verifier: durable contract version is invalid")
	}
	if strings.TrimSpace(durable.UsesLocalAcceptanceID) == "" {
		return fmt.Errorf("candidate package product activation verifier: durable contract acceptance id is required")
	}
	if durable.ActivationReady {
		return fmt.Errorf("candidate package product activation verifier: durable contract cannot already be activation-ready")
	}
	if !durable.NoMutation {
		return fmt.Errorf("candidate package product activation verifier: durable contract must be no-mutation")
	}
	if durable.PromotionLevelClaimed {
		return fmt.Errorf("candidate package product activation verifier: durable contract cannot claim promotion level")
	}
	return nil
}

func validateProductActivationEvidenceIdentity(durable CandidatePackageDurableActivationContract, evidence CandidatePackageProductActivationEvidence) error {
	if evidence.CandidatePackageID != durable.CandidatePackageID {
		return fmt.Errorf("candidate package product activation verifier: evidence package id %q does not match durable contract %q", evidence.CandidatePackageID, durable.CandidatePackageID)
	}
	if evidence.CandidatePackageManifestSHA256 != durable.CandidatePackageManifestSHA256 {
		return fmt.Errorf("candidate package product activation verifier: evidence package hash does not match durable contract")
	}
	if evidence.Version != durable.Version {
		return fmt.Errorf("candidate package product activation verifier: evidence version does not match durable contract")
	}
	if evidence.UsesLocalAcceptanceID != durable.UsesLocalAcceptanceID {
		return fmt.Errorf("candidate package product activation verifier: evidence acceptance id does not match durable contract")
	}
	return nil
}

func productActivationProtectedPrerequisites() []string {
	return []string{
		CandidatePackageProductActivationPrerequisitePackagePublication,
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteDeployedRoute,
		CandidatePackageProductActivationPrerequisiteAuthSession,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	}
}

func publicationExecutorRequiredRedSurfaces() []string {
	return []string{
		CandidatePackagePublicationExecutorRedSurfacePackageArtifact,
		CandidatePackagePublicationExecutorRedSurfaceProviderCredentials,
		CandidatePackagePublicationExecutorRedSurfacePublicationLedger,
		CandidatePackagePublicationExecutorRedSurfaceRollbackPath,
	}
}

func validPublicationExecutorRedSurface(surface string) bool {
	for _, known := range publicationExecutorRequiredRedSurfaces() {
		if surface == known {
			return true
		}
	}
	return false
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func publicationExecutorRequiredImplementationGates() []string {
	return []string{
		CandidatePackagePublicationExecutorImplementationGateRedCeremony,
		CandidatePackagePublicationExecutorImplementationGateOwnerApproval,
		CandidatePackagePublicationExecutorImplementationGateSecurityReview,
		CandidatePackagePublicationExecutorImplementationGateProviderCredentialProof,
		CandidatePackagePublicationExecutorImplementationGateRollbackDrill,
	}
}

func validPublicationExecutorImplementationGate(gate string) bool {
	for _, known := range publicationExecutorRequiredImplementationGates() {
		if gate == known {
			return true
		}
	}
	return false
}

func publicationExecutorReadinessReviewRequiredChecklistItems() []string {
	return []string{
		CandidatePackagePublicationExecutorReadinessReviewItemRedCeremonyScope,
		CandidatePackagePublicationExecutorReadinessReviewItemOwnerApprovalPath,
		CandidatePackagePublicationExecutorReadinessReviewItemSecurityScope,
		CandidatePackagePublicationExecutorReadinessReviewItemProviderCredentialBoundary,
		CandidatePackagePublicationExecutorReadinessReviewItemRollbackDrill,
	}
}

func validPublicationExecutorReadinessReviewItem(item string) bool {
	for _, known := range publicationExecutorReadinessReviewRequiredChecklistItems() {
		if item == known {
			return true
		}
	}
	return false
}

func sameCanonicalStrings(left []string, right []string) bool {
	left = canonicalStrings(left)
	right = canonicalStrings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func validProductActivationPrerequisite(prerequisite string) bool {
	for _, known := range productActivationProtectedPrerequisites() {
		if prerequisite == known {
			return true
		}
	}
	return false
}

func validProductActivationEvidenceStatus(status string) bool {
	switch status {
	case CandidatePackageProductActivationEvidenceStatusCandidate, CandidatePackageProductActivationEvidenceStatusPassed:
		return true
	default:
		return false
	}
}
