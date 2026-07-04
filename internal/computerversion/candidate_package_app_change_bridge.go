package computerversion

import (
	"encoding/json"
	"fmt"
	"strings"
)

const CandidatePackageAppChangeBridgeKind = "candidate_package_app_change_bridge"

const CandidatePackageProductPathAcceptanceKind = "candidate_package_product_path_acceptance"

const CandidatePackageEvidenceOnlyIntakeBoundary = "candidate_package_evidence_only_intake"

// CandidatePackageAppChangeBridgePayload is the safe seam between the new
// candidate-computer package evidence and the existing AppChangePackage product
// path. It deliberately produces embeddable manifest/contract/provenance JSON,
// not a publish request: the current AppChangePackage path still requires source
// deltas and recipient build/adoption verification before transfer.
type CandidatePackageAppChangeBridgePayload struct {
	Kind                           string            `json:"kind"`
	CandidatePackageID             string            `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string            `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion   `json:"version"`
	SourceComputerID               string            `json:"source_computer_id,omitempty"`
	SourceCandidateID              string            `json:"source_candidate_id,omitempty"`
	CandidateSourceRef             string            `json:"candidate_source_ref,omitempty"`
	DirectPublishReady             bool              `json:"direct_publish_ready"`
	DirectPublishBlockers          []string          `json:"direct_publish_blockers"`
	ManifestJSON                   json.RawMessage   `json:"manifest_json"`
	VerifierContractsJSON          json.RawMessage   `json:"verifier_contracts_json"`
	ProvenanceRefsJSON             json.RawMessage   `json:"provenance_refs_json"`
	RequiredObservations           []ObservationKind `json:"required_observations"`
}

// CandidatePackageProductPathAcceptanceContract names the product boundary that
// may accept candidate-computer evidence without turning it into adoption,
// promotion, or AppChangePackage publication. It is a verifier contract, not a
// product API request.
type CandidatePackageProductPathAcceptanceContract struct {
	Kind                           string                                        `json:"kind"`
	CandidatePackageID             string                                        `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string                                        `json:"candidate_package_manifest_sha256"`
	Version                        ComputerVersion                               `json:"version"`
	IntakeBoundary                 string                                        `json:"intake_boundary"`
	OwnerReviewRequired            bool                                          `json:"owner_review_required"`
	AdoptionReady                  bool                                          `json:"adoption_ready"`
	AdoptionBlockers               []string                                      `json:"adoption_blockers"`
	VerifierContracts              []CandidatePackageProductPathVerifierContract `json:"verifier_contracts"`
	EvidenceRefs                   []string                                      `json:"evidence_refs"`
	RequiredObservations           []ObservationKind                             `json:"required_observations"`
}

type CandidatePackageProductPathVerifierContract struct {
	ContractID string `json:"contract_id"`
	Status     string `json:"status"`
	Summary    string `json:"summary"`
}

// BuildCandidatePackageAppChangeBridge validates pkg and emits the product-path
// bridge payload that can be embedded in a future AppChangePackage without
// pretending that candidate evidence alone satisfies source-delta publication.
func BuildCandidatePackageAppChangeBridge(pkg CandidateComputerPackageManifest) (CandidatePackageAppChangeBridgePayload, error) {
	if strings.TrimSpace(pkg.PackageManifestSHA256) == "" {
		return CandidatePackageAppChangeBridgePayload{}, fmt.Errorf("candidate package app-change bridge: package manifest hash is required")
	}
	if err := pkg.Validate(); err != nil {
		return CandidatePackageAppChangeBridgePayload{}, fmt.Errorf("candidate package app-change bridge: package: %w", err)
	}
	required := canonicalObservationKinds(pkg.RequiredObservations)
	manifestJSON, err := json.Marshal(map[string]any{
		"bridge_kind":                       CandidatePackageAppChangeBridgeKind,
		"candidate_package_id":              pkg.ID,
		"candidate_package_manifest_sha256": pkg.PackageManifestSHA256,
		"candidate_computer_version":        pkg.Version,
		"source_computer_id":                pkg.SourceComputerID,
		"source_candidate_id":               pkg.SourceCandidateID,
		"candidate_source_ref":              pkg.CandidateSourceRef,
		"source_ledger_candidate_ref":       pkg.CandidateSourceRef,
		"evidence_root_id":                  pkg.EvidenceRoot.ID,
		"evidence_root_source":              pkg.EvidenceRoot.Source,
		"required_observations":             required,
		"recipient_build_required":          true,
	})
	if err != nil {
		return CandidatePackageAppChangeBridgePayload{}, fmt.Errorf("candidate package app-change bridge: encode manifest: %w", err)
	}
	contractsJSON, err := json.Marshal(candidatePackageVerifierContracts(pkg))
	if err != nil {
		return CandidatePackageAppChangeBridgePayload{}, fmt.Errorf("candidate package app-change bridge: encode verifier contracts: %w", err)
	}
	provenanceJSON, err := json.Marshal(map[string]any{
		"candidate_package_id":              pkg.ID,
		"candidate_package_manifest_sha256": pkg.PackageManifestSHA256,
		"evidence_refs":                     canonicalStrings(pkg.EvidenceRefs),
		"evidence_root_id":                  pkg.EvidenceRoot.ID,
		"realization_count":                 len(pkg.Realizations),
		"observation_count":                 len(pkg.EvidenceRootObservation.Observations),
	})
	if err != nil {
		return CandidatePackageAppChangeBridgePayload{}, fmt.Errorf("candidate package app-change bridge: encode provenance refs: %w", err)
	}
	return CandidatePackageAppChangeBridgePayload{
		Kind:                           CandidatePackageAppChangeBridgeKind,
		CandidatePackageID:             pkg.ID,
		CandidatePackageManifestSHA256: pkg.PackageManifestSHA256,
		Version:                        pkg.Version,
		SourceComputerID:               pkg.SourceComputerID,
		SourceCandidateID:              pkg.SourceCandidateID,
		CandidateSourceRef:             pkg.CandidateSourceRef,
		DirectPublishReady:             false,
		DirectPublishBlockers: []string{
			"app_change_package_publish_requires_runtime_or_ui_source_delta",
			"candidate_computer_package_is_evidence_payload_not_product_source_delta",
		},
		ManifestJSON:          manifestJSON,
		VerifierContractsJSON: contractsJSON,
		ProvenanceRefsJSON:    provenanceJSON,
		RequiredObservations:  required,
	}, nil
}

func candidatePackageVerifierContracts(pkg CandidateComputerPackageManifest) []map[string]any {
	contracts := make([]map[string]any, 0, len(pkg.ReviewContracts)+1)
	for _, contract := range pkg.ReviewContracts {
		contracts = append(contracts, map[string]any{
			"contract_id": strings.TrimSpace(contract.Name),
			"status":      strings.TrimSpace(contract.Status),
			"summary":     strings.TrimSpace(contract.Evidence),
		})
	}
	contracts = append(contracts, map[string]any{
		"contract_id": "direct-publish-blocked-without-source-delta",
		"status":      "blocked",
		"summary":     "Candidate-computer package evidence can be embedded for review, but current AppChangePackage publication still requires runtime or UI source delta payloads.",
	})
	return contracts
}

// BuildCandidatePackageProductPathAcceptanceContract defines the narrow product
// path that may ingest candidate-computer evidence today: evidence-only intake
// plus owner review. It intentionally leaves adoption and direct publication
// blocked until a separate source-delta or candidate-computer promotion path is
// implemented and verified.
func BuildCandidatePackageProductPathAcceptanceContract(pkg CandidateComputerPackageManifest, bridge CandidatePackageAppChangeBridgePayload) (CandidatePackageProductPathAcceptanceContract, error) {
	if err := pkg.Validate(); err != nil {
		return CandidatePackageProductPathAcceptanceContract{}, fmt.Errorf("candidate package product-path acceptance: package: %w", err)
	}
	if strings.TrimSpace(pkg.PackageManifestSHA256) == "" {
		return CandidatePackageProductPathAcceptanceContract{}, fmt.Errorf("candidate package product-path acceptance: package manifest hash is required")
	}
	if err := validateCandidatePackageBridgeForAcceptance(pkg, bridge); err != nil {
		return CandidatePackageProductPathAcceptanceContract{}, err
	}
	return CandidatePackageProductPathAcceptanceContract{
		Kind:                           CandidatePackageProductPathAcceptanceKind,
		CandidatePackageID:             pkg.ID,
		CandidatePackageManifestSHA256: pkg.PackageManifestSHA256,
		Version:                        pkg.Version,
		IntakeBoundary:                 CandidatePackageEvidenceOnlyIntakeBoundary,
		OwnerReviewRequired:            true,
		AdoptionReady:                  false,
		AdoptionBlockers: []string{
			"candidate_package_has_no_product_api_intake_record",
			"app_change_package_publish_requires_runtime_or_ui_source_delta",
			"owner_review_not_recorded",
			"adoption_rollback_boundary_not_bound",
		},
		VerifierContracts: []CandidatePackageProductPathVerifierContract{
			{
				ContractID: "candidate-package-hash",
				Status:     "passed",
				Summary:    "PackageManifestSHA256 is present and the package manifest validates.",
			},
			{
				ContractID: "candidate-evidence-non-production",
				Status:     "passed",
				Summary:    "Candidate evidence rejects production state and deployed route mutation.",
			},
			{
				ContractID: "required-observations-present",
				Status:     "passed",
				Summary:    "Required observation classes are present across the evidence root and realizations.",
			},
			{
				ContractID: "evidence-only-intake-boundary",
				Status:     "pending",
				Summary:    "A product API boundary must persist this evidence package for owner review before any recipient action.",
			},
			{
				ContractID: "direct-app-change-publish",
				Status:     "blocked",
				Summary:    "The bridge is review evidence only; direct AppChangePackage publication still requires runtime or UI source deltas.",
			},
			{
				ContractID: "adoption-and-rollback-boundary",
				Status:     "blocked",
				Summary:    "Adoption, rollback, and active-computer changes remain impossible until a verified promotion/adoption path binds them.",
			},
		},
		EvidenceRefs:         canonicalStrings(pkg.EvidenceRefs),
		RequiredObservations: canonicalObservationKinds(pkg.RequiredObservations),
	}, nil
}

func validateCandidatePackageBridgeForAcceptance(pkg CandidateComputerPackageManifest, bridge CandidatePackageAppChangeBridgePayload) error {
	if strings.TrimSpace(bridge.Kind) != CandidatePackageAppChangeBridgeKind {
		return fmt.Errorf("candidate package product-path acceptance: bridge kind %q is not %q", bridge.Kind, CandidatePackageAppChangeBridgeKind)
	}
	if bridge.CandidatePackageID != pkg.ID {
		return fmt.Errorf("candidate package product-path acceptance: bridge package id %q does not match package %q", bridge.CandidatePackageID, pkg.ID)
	}
	if bridge.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		return fmt.Errorf("candidate package product-path acceptance: bridge package hash does not match package hash")
	}
	if bridge.DirectPublishReady {
		return fmt.Errorf("candidate package product-path acceptance: bridge cannot be direct-publish ready")
	}
	if len(bridge.DirectPublishBlockers) == 0 {
		return fmt.Errorf("candidate package product-path acceptance: bridge direct-publish blockers are required")
	}
	for _, raw := range []struct {
		name string
		data json.RawMessage
	}{
		{name: "manifest_json", data: bridge.ManifestJSON},
		{name: "verifier_contracts_json", data: bridge.VerifierContractsJSON},
		{name: "provenance_refs_json", data: bridge.ProvenanceRefsJSON},
	} {
		if !json.Valid(raw.data) {
			return fmt.Errorf("candidate package product-path acceptance: bridge %s is not valid JSON", raw.name)
		}
	}
	return nil
}
