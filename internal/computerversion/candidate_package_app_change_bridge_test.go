package computerversion

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestBuildCandidatePackageAppChangeBridgeEmitsEvidenceOnlyPayload(t *testing.T) {
	pkg := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())

	bridge, err := BuildCandidatePackageAppChangeBridge(pkg)
	if err != nil {
		t.Fatalf("build candidate package app-change bridge: %v", err)
	}

	if bridge.Kind != CandidatePackageAppChangeBridgeKind {
		t.Fatalf("kind = %q, want %q", bridge.Kind, CandidatePackageAppChangeBridgeKind)
	}
	if bridge.CandidatePackageID != pkg.ID {
		t.Fatalf("candidate package id = %q, want %q", bridge.CandidatePackageID, pkg.ID)
	}
	if bridge.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		t.Fatalf("package manifest hash = %q, want %q", bridge.CandidatePackageManifestSHA256, pkg.PackageManifestSHA256)
	}
	if bridge.Version != pkg.Version {
		t.Fatalf("version = %#v, want %#v", bridge.Version, pkg.Version)
	}
	if bridge.SourceComputerID != pkg.SourceComputerID || bridge.SourceCandidateID != pkg.SourceCandidateID || bridge.CandidateSourceRef != pkg.CandidateSourceRef {
		t.Fatalf("source refs = computer:%q candidate:%q source:%q, want computer:%q candidate:%q source:%q", bridge.SourceComputerID, bridge.SourceCandidateID, bridge.CandidateSourceRef, pkg.SourceComputerID, pkg.SourceCandidateID, pkg.CandidateSourceRef)
	}
	if bridge.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false for evidence-only bridge")
	}
	wantBlockers := []string{
		"app_change_package_publish_requires_runtime_or_ui_source_delta",
		"candidate_computer_package_is_evidence_payload_not_product_source_delta",
	}
	if !reflect.DeepEqual(bridge.DirectPublishBlockers, wantBlockers) {
		t.Fatalf("direct publish blockers = %#v, want %#v", bridge.DirectPublishBlockers, wantBlockers)
	}
	if !reflect.DeepEqual(bridge.RequiredObservations, pkg.RequiredObservations) {
		t.Fatalf("required observations = %#v, want %#v", bridge.RequiredObservations, pkg.RequiredObservations)
	}

	var manifest struct {
		BridgeKind                     string            `json:"bridge_kind"`
		CandidatePackageID             string            `json:"candidate_package_id"`
		CandidatePackageManifestSHA256 string            `json:"candidate_package_manifest_sha256"`
		CandidateComputerVersion       ComputerVersion   `json:"candidate_computer_version"`
		SourceComputerID               string            `json:"source_computer_id"`
		SourceCandidateID              string            `json:"source_candidate_id"`
		CandidateSourceRef             string            `json:"candidate_source_ref"`
		SourceLedgerCandidateRef       string            `json:"source_ledger_candidate_ref"`
		EvidenceRootID                 string            `json:"evidence_root_id"`
		EvidenceRootSource             string            `json:"evidence_root_source"`
		RequiredObservations           []ObservationKind `json:"required_observations"`
		RecipientBuildRequired         bool              `json:"recipient_build_required"`
	}
	decodeBridgeJSON(t, bridge.ManifestJSON, &manifest)
	if manifest.BridgeKind != CandidatePackageAppChangeBridgeKind {
		t.Fatalf("manifest bridge kind = %q, want %q", manifest.BridgeKind, CandidatePackageAppChangeBridgeKind)
	}
	if manifest.CandidatePackageID != pkg.ID || manifest.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		t.Fatalf("manifest package refs = id:%q hash:%q, want id:%q hash:%q", manifest.CandidatePackageID, manifest.CandidatePackageManifestSHA256, pkg.ID, pkg.PackageManifestSHA256)
	}
	if manifest.CandidateComputerVersion != pkg.Version {
		t.Fatalf("manifest version = %#v, want %#v", manifest.CandidateComputerVersion, pkg.Version)
	}
	if manifest.SourceComputerID != pkg.SourceComputerID || manifest.SourceCandidateID != pkg.SourceCandidateID || manifest.CandidateSourceRef != pkg.CandidateSourceRef || manifest.SourceLedgerCandidateRef != pkg.CandidateSourceRef {
		t.Fatalf("manifest source refs = computer:%q candidate:%q source:%q ledger:%q, want computer:%q candidate:%q source:%q ledger:%q", manifest.SourceComputerID, manifest.SourceCandidateID, manifest.CandidateSourceRef, manifest.SourceLedgerCandidateRef, pkg.SourceComputerID, pkg.SourceCandidateID, pkg.CandidateSourceRef, pkg.CandidateSourceRef)
	}
	if manifest.EvidenceRootID != pkg.EvidenceRoot.ID || manifest.EvidenceRootSource != pkg.EvidenceRoot.Source {
		t.Fatalf("manifest evidence root = id:%q source:%q, want id:%q source:%q", manifest.EvidenceRootID, manifest.EvidenceRootSource, pkg.EvidenceRoot.ID, pkg.EvidenceRoot.Source)
	}
	if !reflect.DeepEqual(manifest.RequiredObservations, pkg.RequiredObservations) {
		t.Fatalf("manifest required observations = %#v, want %#v", manifest.RequiredObservations, pkg.RequiredObservations)
	}
	if !manifest.RecipientBuildRequired {
		t.Fatalf("manifest recipient build required = false, want true")
	}

	var contracts []struct {
		ContractID string `json:"contract_id"`
		Status     string `json:"status"`
		Summary    string `json:"summary"`
	}
	decodeBridgeJSON(t, bridge.VerifierContractsJSON, &contracts)
	if len(contracts) != len(pkg.ReviewContracts)+1 {
		t.Fatalf("verifier contracts count = %d, want %d: %#v", len(contracts), len(pkg.ReviewContracts)+1, contracts)
	}
	contractsByID := make(map[string]struct {
		Status  string
		Summary string
	}, len(contracts))
	for _, contract := range contracts {
		contractsByID[contract.ContractID] = struct {
			Status  string
			Summary string
		}{Status: contract.Status, Summary: contract.Summary}
	}
	for _, review := range pkg.ReviewContracts {
		got, ok := contractsByID[review.Name]
		if !ok {
			t.Fatalf("verifier contracts missing review contract %q: %#v", review.Name, contracts)
		}
		if got.Status != review.Status || got.Summary != review.Evidence {
			t.Fatalf("review contract %q = status:%q summary:%q, want status:%q summary:%q", review.Name, got.Status, got.Summary, review.Status, review.Evidence)
		}
	}
	blocked, ok := contractsByID["direct-publish-blocked-without-source-delta"]
	if !ok {
		t.Fatalf("verifier contracts missing direct-publish block contract: %#v", contracts)
	}
	if blocked.Status != "blocked" || !strings.Contains(blocked.Summary, "requires runtime or UI source delta") {
		t.Fatalf("direct-publish block contract = status:%q summary:%q, want blocked source-delta requirement", blocked.Status, blocked.Summary)
	}

	var provenance struct {
		CandidatePackageID             string   `json:"candidate_package_id"`
		CandidatePackageManifestSHA256 string   `json:"candidate_package_manifest_sha256"`
		EvidenceRefs                   []string `json:"evidence_refs"`
		EvidenceRootID                 string   `json:"evidence_root_id"`
		RealizationCount               int      `json:"realization_count"`
		ObservationCount               int      `json:"observation_count"`
	}
	decodeBridgeJSON(t, bridge.ProvenanceRefsJSON, &provenance)
	if provenance.CandidatePackageID != pkg.ID || provenance.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		t.Fatalf("provenance package refs = id:%q hash:%q, want id:%q hash:%q", provenance.CandidatePackageID, provenance.CandidatePackageManifestSHA256, pkg.ID, pkg.PackageManifestSHA256)
	}
	if !reflect.DeepEqual(provenance.EvidenceRefs, canonicalStrings(pkg.EvidenceRefs)) {
		t.Fatalf("provenance evidence refs = %#v, want canonical refs %#v", provenance.EvidenceRefs, canonicalStrings(pkg.EvidenceRefs))
	}
	if provenance.EvidenceRootID != pkg.EvidenceRoot.ID {
		t.Fatalf("provenance evidence root id = %q, want %q", provenance.EvidenceRootID, pkg.EvidenceRoot.ID)
	}
	if provenance.ObservationCount != len(pkg.EvidenceRootObservation.Observations) {
		t.Fatalf("provenance observation count = %d, want %d", provenance.ObservationCount, len(pkg.EvidenceRootObservation.Observations))
	}
	if provenance.RealizationCount != len(pkg.Realizations) {
		t.Fatalf("provenance realization count = %d, want %d", provenance.RealizationCount, len(pkg.Realizations))
	}
}

func TestBuildCandidatePackageAppChangeBridgeRejectsMissingPackageManifestHash(t *testing.T) {
	pkg := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())
	pkg.PackageManifestSHA256 = "  "

	bridge, err := BuildCandidatePackageAppChangeBridge(pkg)
	if err == nil || !strings.Contains(err.Error(), "package manifest hash is required") {
		t.Fatalf("BuildCandidatePackageAppChangeBridge() = bridge %#v error %v, want missing hash error", bridge, err)
	}
}

func TestBuildCandidatePackageAppChangeBridgeRejectsInvalidPackageContent(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-candidate-package-bridge", ArtifactProgramRef: "tape:org/foreign-candidate-package-bridge@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateComputerPackageManifest)
		wantErr string
	}{
		{
			name: "package production flag",
			mutate: func(pkg *CandidateComputerPackageManifest) {
				pkg.ContainsProduction = true
			},
			wantErr: "production state is not admissible",
		},
		{
			name: "evidence root production flag",
			mutate: func(pkg *CandidateComputerPackageManifest) {
				pkg.EvidenceRoot.ContainsProduction = true
			},
			wantErr: "production state is not admissible",
		},
		{
			name: "mismatched realization version",
			mutate: func(pkg *CandidateComputerPackageManifest) {
				pkg.Realizations[0].Version = foreignVersion
			},
			wantErr: "version does not match package version",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())
			tc.mutate(&pkg)

			bridge, err := BuildCandidatePackageAppChangeBridge(pkg)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackageAppChangeBridge() = bridge %#v error %v, want error containing %q", bridge, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackageProductPathAcceptanceContractEmitsEvidenceOnlySafetyBoundary(t *testing.T) {
	pkg, bridge := candidatePackageAcceptanceInputs(t)

	contract, err := BuildCandidatePackageProductPathAcceptanceContract(pkg, bridge)
	if err != nil {
		t.Fatalf("build candidate package product-path acceptance contract: %v", err)
	}

	if contract.Kind != CandidatePackageProductPathAcceptanceKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackageProductPathAcceptanceKind)
	}
	if contract.CandidatePackageID != pkg.ID || contract.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, pkg.ID, pkg.PackageManifestSHA256)
	}
	if contract.Version != pkg.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, pkg.Version)
	}
	if contract.IntakeBoundary != CandidatePackageEvidenceOnlyIntakeBoundary {
		t.Fatalf("intake boundary = %q, want %q", contract.IntakeBoundary, CandidatePackageEvidenceOnlyIntakeBoundary)
	}
	if !contract.OwnerReviewRequired {
		t.Fatalf("owner review required = false, want true")
	}
	if contract.AdoptionReady {
		t.Fatalf("adoption ready = true, want false for evidence-only intake")
	}
	wantBlockers := []string{
		"candidate_package_has_no_product_api_intake_record",
		"app_change_package_publish_requires_runtime_or_ui_source_delta",
		"owner_review_not_recorded",
		"adoption_rollback_boundary_not_bound",
	}
	if !reflect.DeepEqual(contract.AdoptionBlockers, wantBlockers) {
		t.Fatalf("adoption blockers = %#v, want %#v", contract.AdoptionBlockers, wantBlockers)
	}

	wantVerifierContracts := map[string]struct {
		status          string
		summaryContains string
	}{
		"candidate-package-hash": {
			status:          "passed",
			summaryContains: "PackageManifestSHA256 is present",
		},
		"candidate-evidence-non-production": {
			status:          "passed",
			summaryContains: "rejects production state",
		},
		"required-observations-present": {
			status:          "passed",
			summaryContains: "Required observation classes are present",
		},
		"evidence-only-intake-boundary": {
			status:          "pending",
			summaryContains: "persist this evidence package for owner review",
		},
		"direct-app-change-publish": {
			status:          "blocked",
			summaryContains: "direct AppChangePackage publication still requires runtime or UI source deltas",
		},
		"adoption-and-rollback-boundary": {
			status:          "blocked",
			summaryContains: "Adoption, rollback, and active-computer changes remain impossible",
		},
	}
	if len(contract.VerifierContracts) != len(wantVerifierContracts) {
		t.Fatalf("verifier contract count = %d, want %d: %#v", len(contract.VerifierContracts), len(wantVerifierContracts), contract.VerifierContracts)
	}
	for _, got := range contract.VerifierContracts {
		want, ok := wantVerifierContracts[got.ContractID]
		if !ok {
			t.Fatalf("unexpected verifier contract %#v", got)
		}
		if got.Status != want.status || !strings.Contains(got.Summary, want.summaryContains) {
			t.Fatalf("verifier contract %q = status:%q summary:%q, want status:%q summary containing %q", got.ContractID, got.Status, got.Summary, want.status, want.summaryContains)
		}
		delete(wantVerifierContracts, got.ContractID)
	}
	if len(wantVerifierContracts) != 0 {
		t.Fatalf("missing verifier contracts: %#v", wantVerifierContracts)
	}

	wantEvidenceRefs := []string{"evidenceroot:manifest", "vmrealize:manifest"}
	if !reflect.DeepEqual(contract.EvidenceRefs, wantEvidenceRefs) {
		t.Fatalf("evidence refs = %#v, want canonical refs %#v", contract.EvidenceRefs, wantEvidenceRefs)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, []ObservationKind{
		ObservationBlobSet,
		ObservationFileManifest,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	})
}

func TestBuildCandidatePackageProductPathAcceptanceContractRejectsUnsafeOrMismatchedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateComputerPackageManifest, *CandidatePackageAppChangeBridgePayload)
		wantErr string
	}{
		{
			name: "bridge package id mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "bridge package id",
		},
		{
			name: "bridge package hash mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("0", 64)
			},
			wantErr: "bridge package hash does not match",
		},
		{
			name: "direct publish ready bridge",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.DirectPublishReady = true
			},
			wantErr: "bridge cannot be direct-publish ready",
		},
		{
			name: "missing direct publish blockers",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.DirectPublishBlockers = nil
			},
			wantErr: "bridge direct-publish blockers are required",
		},
		{
			name: "invalid manifest JSON",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.ManifestJSON = json.RawMessage("{")
			},
			wantErr: "bridge manifest_json is not valid JSON",
		},
		{
			name: "invalid verifier contracts JSON",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.VerifierContractsJSON = json.RawMessage("{")
			},
			wantErr: "bridge verifier_contracts_json is not valid JSON",
		},
		{
			name: "invalid provenance refs JSON",
			mutate: func(_ *CandidateComputerPackageManifest, bridge *CandidatePackageAppChangeBridgePayload) {
				bridge.ProvenanceRefsJSON = json.RawMessage("{")
			},
			wantErr: "bridge provenance_refs_json is not valid JSON",
		},
		{
			name: "invalid package content",
			mutate: func(pkg *CandidateComputerPackageManifest, _ *CandidatePackageAppChangeBridgePayload) {
				pkg.EvidenceRoot.TouchesDeployedRoute = true
			},
			wantErr: "deployed route mutation is not admissible",
		},
		{
			name: "missing package hash",
			mutate: func(pkg *CandidateComputerPackageManifest, _ *CandidatePackageAppChangeBridgePayload) {
				pkg.PackageManifestSHA256 = "  "
			},
			wantErr: "package manifest hash is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg, bridge := candidatePackageAcceptanceInputs(t)
			tc.mutate(&pkg, &bridge)

			contract, err := BuildCandidatePackageProductPathAcceptanceContract(pkg, bridge)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackageProductPathAcceptanceContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func candidatePackageAcceptanceInputs(t *testing.T) (CandidateComputerPackageManifest, CandidatePackageAppChangeBridgePayload) {
	t.Helper()

	pkg := candidateComputerPackageBuiltManifest(t, candidateComputerPackageVersion())
	bridge, err := BuildCandidatePackageAppChangeBridge(pkg)
	if err != nil {
		t.Fatalf("build valid candidate package app-change bridge: %v", err)
	}
	return pkg, bridge
}

func decodeBridgeJSON(t *testing.T, data json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode bridge JSON %s: %v", string(data), err)
	}
}
