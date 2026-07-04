package computerversion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const CandidateComputerPackageKind = "candidate_computer_package"

// CandidateComputerPackageManifest is the minimum reviewable bundle for one
// candidate computer proof. It binds an admitted evidence root to the
// substrate-specific realizations produced from that same ComputerVersion. It is
// intentionally not an AppChangePackage and does not publish, adopt, promote, or
// mutate deployed state.
type CandidateComputerPackageManifest struct {
	ID                      string                           `json:"id"`
	Kind                    string                           `json:"kind"`
	Version                 ComputerVersion                  `json:"version"`
	SourceComputerID        string                           `json:"source_computer_id,omitempty"`
	SourceCandidateID       string                           `json:"source_candidate_id,omitempty"`
	CandidateSourceRef      string                           `json:"candidate_source_ref,omitempty"`
	EvidenceRoot            CandidateEvidenceRootManifest    `json:"evidence_root"`
	EvidenceRootObservation ObservationSet                   `json:"evidence_root_observation"`
	Realizations            []Realization                    `json:"realizations"`
	RequiredObservations    []ObservationKind                `json:"required_observations"`
	EvidenceRefs            []string                         `json:"evidence_refs,omitempty"`
	ReviewContracts         []CandidatePackageReviewContract `json:"review_contracts"`
	ContainsProduction      bool                             `json:"contains_production"`
	TouchesDeployedRoute    bool                             `json:"touches_deployed_route"`
	PackageManifestSHA256   string                           `json:"package_manifest_sha256"`
}

// CandidatePackageReviewContract is a small machine-checkable review obligation
// for the bundled artifact. It is not proof by itself; it names the evidence a
// reviewer must see before widening any claim.
type CandidatePackageReviewContract struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

// BuildCandidateComputerPackage validates and hashes one candidate-computer
// package manifest from existing local evidence. The hash is computed over the
// canonical manifest with PackageManifestSHA256 cleared.
func BuildCandidateComputerPackage(manifest CandidateComputerPackageManifest) (CandidateComputerPackageManifest, error) {
	manifest.ID = strings.TrimSpace(manifest.ID)
	if manifest.Kind == "" {
		manifest.Kind = CandidateComputerPackageKind
	}
	manifest.Kind = strings.TrimSpace(manifest.Kind)
	manifest.SourceComputerID = strings.TrimSpace(manifest.SourceComputerID)
	manifest.SourceCandidateID = strings.TrimSpace(manifest.SourceCandidateID)
	manifest.CandidateSourceRef = strings.TrimSpace(manifest.CandidateSourceRef)
	manifest.EvidenceRefs = canonicalStrings(manifest.EvidenceRefs)
	manifest.RequiredObservations = canonicalObservationKinds(packageRequiredKinds(manifest))
	if len(manifest.ReviewContracts) == 0 {
		manifest.ReviewContracts = defaultCandidatePackageReviewContracts()
	}
	manifest.PackageManifestSHA256 = ""
	if err := manifest.Validate(); err != nil {
		return CandidateComputerPackageManifest{}, err
	}
	hash, err := candidatePackageHash(manifest)
	if err != nil {
		return CandidateComputerPackageManifest{}, err
	}
	manifest.PackageManifestSHA256 = hash
	return manifest, nil
}

// Validate checks that the bundle is reviewable without widening into production
// state, deployed route mutation, or unsupported realization claims.
func (m CandidateComputerPackageManifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("candidate computer package: id is required")
	}
	if strings.TrimSpace(m.Kind) != CandidateComputerPackageKind {
		return fmt.Errorf("candidate computer package: unsupported kind %q", m.Kind)
	}
	if !m.Version.Valid() {
		return fmt.Errorf("candidate computer package: invalid computer version")
	}
	if m.ContainsProduction || m.EvidenceRoot.ContainsProduction {
		return fmt.Errorf("candidate computer package: production state is not admissible")
	}
	if m.TouchesDeployedRoute || m.EvidenceRoot.TouchesDeployedRoute {
		return fmt.Errorf("candidate computer package: deployed route mutation is not admissible")
	}
	if err := m.EvidenceRoot.Validate(); err != nil {
		return fmt.Errorf("candidate computer package: evidence root: %w", err)
	}
	if m.EvidenceRoot.Fixture.Version != m.Version {
		return fmt.Errorf("candidate computer package: evidence root version does not match package version")
	}
	if m.EvidenceRootObservation.Version != m.Version {
		return fmt.Errorf("candidate computer package: evidence root observation version does not match package version")
	}
	if len(m.EvidenceRootObservation.Observations) == 0 {
		return fmt.Errorf("candidate computer package: evidence root observation is empty")
	}
	for _, observation := range m.EvidenceRootObservation.Observations {
		if !observation.Valid() {
			return fmt.Errorf("candidate computer package: invalid evidence root observation kind=%q key=%q", observation.Kind, observation.Key)
		}
	}
	if len(m.Realizations) == 0 {
		return fmt.Errorf("candidate computer package: at least one realization is required")
	}
	for i, realization := range m.Realizations {
		if err := validateCandidatePackageRealization(m.Version, realization); err != nil {
			return fmt.Errorf("candidate computer package: realization %d: %w", i, err)
		}
	}
	if err := requirePackageObservationKinds(m.RequiredObservations, m.EvidenceRootObservation, m.Realizations); err != nil {
		return err
	}
	for _, contract := range m.ReviewContracts {
		if strings.TrimSpace(contract.Name) == "" || strings.TrimSpace(contract.Status) == "" || strings.TrimSpace(contract.Evidence) == "" {
			return fmt.Errorf("candidate computer package: review contracts require name, status, and evidence")
		}
	}
	return nil
}

func validateCandidatePackageRealization(version ComputerVersion, realization Realization) error {
	if strings.TrimSpace(realization.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if realization.Version != version {
		return fmt.Errorf("version does not match package version")
	}
	if strings.TrimSpace(realization.Capabilities.Materializer) == "" || strings.TrimSpace(realization.Capabilities.Substrate) == "" {
		return fmt.Errorf("capability manifest requires materializer and substrate")
	}
	if realization.Observations.Version != version {
		return fmt.Errorf("observation set version does not match realization version")
	}
	if len(realization.Observations.Observations) == 0 {
		return fmt.Errorf("observation set is empty")
	}
	for _, observation := range realization.Observations.Observations {
		if !observation.Valid() {
			return fmt.Errorf("invalid observation kind=%q key=%q", observation.Kind, observation.Key)
		}
	}
	if missing := realization.Capabilities.MissingRequired(realization.Observations.RequiredKinds()); len(missing) > 0 {
		return fmt.Errorf("capability manifest does not support required observations: %v", missing)
	}
	return nil
}

func requirePackageObservationKinds(required []ObservationKind, root ObservationSet, realizations []Realization) error {
	available := make(map[ObservationKind]struct{})
	for _, observation := range root.Observations {
		available[observation.Kind] = struct{}{}
	}
	for _, realization := range realizations {
		for _, observation := range realization.Observations.Observations {
			available[observation.Kind] = struct{}{}
		}
	}
	for _, kind := range required {
		if !kind.Valid() {
			return fmt.Errorf("candidate computer package: invalid required observation kind %q", kind)
		}
		if _, ok := available[kind]; !ok {
			return fmt.Errorf("candidate computer package: required observation %q is missing from bundled evidence", kind)
		}
	}
	return nil
}

func packageRequiredKinds(m CandidateComputerPackageManifest) []ObservationKind {
	kinds := m.EvidenceRootObservation.RequiredKinds()
	for _, realization := range m.Realizations {
		kinds = mergeKinds(kinds, realization.Observations.RequiredKinds())
	}
	return kinds
}

func canonicalObservationKinds(kinds []ObservationKind) []ObservationKind {
	seen := make(map[ObservationKind]struct{}, len(kinds))
	out := make([]ObservationKind, 0, len(kinds))
	for _, kind := range kinds {
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func canonicalStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func defaultCandidatePackageReviewContracts() []CandidatePackageReviewContract {
	return []CandidatePackageReviewContract{
		{Name: "evidence-root-admitted", Status: "required", Evidence: "CandidateEvidenceRootManifest validates as non-production and route-inert"},
		{Name: "realization-scoped", Status: "required", Evidence: "Every realization declares capabilities and supports its required observations"},
		{Name: "package-hash", Status: "required", Evidence: "PackageManifestSHA256 hashes the canonical manifest with the hash field cleared"},
	}
}

func candidatePackageHash(manifest CandidateComputerPackageManifest) (string, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("candidate computer package: encode manifest for hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
