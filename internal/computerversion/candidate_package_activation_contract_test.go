package computerversion

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildCandidatePackageDurableActivationContractEmitsBlockedPureContract(t *testing.T) {
	pkg, acceptance, decision := candidatePackageDurableActivationInputs(t)

	contract, err := BuildCandidatePackageDurableActivationContract(pkg, acceptance, decision)
	if err != nil {
		t.Fatalf("build candidate package durable activation contract: %v", err)
	}

	if contract.Kind != CandidatePackageDurableActivationContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackageDurableActivationContractKind)
	}
	if contract.CandidatePackageID != pkg.ID || contract.CandidatePackageManifestSHA256 != pkg.PackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, pkg.ID, pkg.PackageManifestSHA256)
	}
	if contract.Version != pkg.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, pkg.Version)
	}
	if contract.ProductPathAcceptanceKind != CandidatePackageProductPathAcceptanceKind {
		t.Fatalf("product path acceptance kind = %q, want %q", contract.ProductPathAcceptanceKind, CandidatePackageProductPathAcceptanceKind)
	}
	if contract.UsesLocalAcceptanceID != decision.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, decision.UsesLocalAcceptanceID)
	}
	if contract.OwnerDecisionState != CandidatePackageOwnerDecisionPreparableState {
		t.Fatalf("owner decision state = %q, want %q", contract.OwnerDecisionState, CandidatePackageOwnerDecisionPreparableState)
	}
	if contract.PreparedAction != CandidatePackagePrepareActivationDecisionAction {
		t.Fatalf("prepared action = %q, want %q", contract.PreparedAction, CandidatePackagePrepareActivationDecisionAction)
	}
	if contract.NextBoundary != CandidatePackagePromotionRequiresProductActivationContractBoundary {
		t.Fatalf("next boundary = %q, want %q", contract.NextBoundary, CandidatePackagePromotionRequiresProductActivationContractBoundary)
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false until a separate product activation contract exists")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for durable activation decision contract")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for prepared owner decision")
	}
	for _, want := range []string{
		"package_publication_not_authorized",
		"app_adoption_mutation_not_authorized",
		"deployed_route_mutation_not_authorized",
		"auth_session_mutation_not_authorized",
		"staging_acceptance_not_claimed",
		"vm_lifecycle_mutation_not_authorized",
		"run_acceptance_record_not_created",
	} {
		assertStringSliceContains(t, contract.ActivationBlockers, want)
	}
	for _, want := range []string{
		"POST /api/adoptions/{adoption_id}/promote",
		"POST /api/candidate-package-intakes/{intake_id}/publication-draft",
		"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch",
		"POST /api/run-acceptances/synthesize",
		"DELETE /auth/sessions/{session_id}",
		"POST /api/staging/claims",
		"POST /api/vm/lifecycle",
	} {
		assertStringSliceContains(t, contract.BlockedRoutes, want)
	}
	for _, want := range []string{
		"authenticated owner decision contract",
		"package publication contract",
		"AppAdoption mutation contract",
		"deployed route mutation contract",
		"staging identity contract",
		"VM lifecycle contract",
		"run-acceptance contract",
	} {
		assertStringSliceContains(t, contract.RequiredContracts, want)
	}
	if !reflect.DeepEqual(contract.EvidenceRefs, acceptance.EvidenceRefs) {
		t.Fatalf("evidence refs = %#v, want %#v", contract.EvidenceRefs, acceptance.EvidenceRefs)
	}
	assertObservationBundleKinds(t, contract.RequiredObservations, acceptance.RequiredObservations)
}

func TestBuildCandidatePackageDurableActivationContractRejectsMismatchedInputs(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-durable-activation", ArtifactProgramRef: "tape:org/foreign-durable-activation@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidateComputerPackageManifest, *CandidatePackageProductPathAcceptanceContract, *CandidatePackageOwnerActivationDecision)
		wantErr string
	}{
		{
			name: "acceptance package id mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, acceptance *CandidatePackageProductPathAcceptanceContract, _ *CandidatePackageOwnerActivationDecision) {
				acceptance.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "acceptance package id",
		},
		{
			name: "acceptance package hash mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, acceptance *CandidatePackageProductPathAcceptanceContract, _ *CandidatePackageOwnerActivationDecision) {
				acceptance.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("0", 64)
			},
			wantErr: "acceptance package hash does not match",
		},
		{
			name: "acceptance version mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, acceptance *CandidatePackageProductPathAcceptanceContract, _ *CandidatePackageOwnerActivationDecision) {
				acceptance.Version = foreignVersion
			},
			wantErr: "acceptance version does not match",
		},
		{
			name: "owner decision package id mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, _ *CandidatePackageProductPathAcceptanceContract, decision *CandidatePackageOwnerActivationDecision) {
				decision.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "owner decision package id",
		},
		{
			name: "owner decision package hash mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, _ *CandidatePackageProductPathAcceptanceContract, decision *CandidatePackageOwnerActivationDecision) {
				decision.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("1", 64)
			},
			wantErr: "owner decision package hash does not match",
		},
		{
			name: "owner decision version mismatch",
			mutate: func(_ *CandidateComputerPackageManifest, _ *CandidatePackageProductPathAcceptanceContract, decision *CandidatePackageOwnerActivationDecision) {
				decision.Version = foreignVersion
			},
			wantErr: "owner decision version does not match",
		},
		{
			name: "missing owner decision acceptance id",
			mutate: func(_ *CandidateComputerPackageManifest, _ *CandidatePackageProductPathAcceptanceContract, decision *CandidatePackageOwnerActivationDecision) {
				decision.UsesLocalAcceptanceID = "  "
			},
			wantErr: "owner decision acceptance id is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg, acceptance, decision := candidatePackageDurableActivationInputs(t)
			tc.mutate(&pkg, &acceptance, &decision)

			contract, err := BuildCandidatePackageDurableActivationContract(pkg, acceptance, decision)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackageDurableActivationContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackageDurableActivationContractRejectsUnsafeDecisionAttempts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackageOwnerActivationDecision)
		wantErr string
	}{
		{
			name: "activation ready claim",
			mutate: func(decision *CandidatePackageOwnerActivationDecision) {
				decision.ActivationReady = true
			},
			wantErr: "cannot mark activation ready",
		},
		{
			name: "promotion level claim",
			mutate: func(decision *CandidatePackageOwnerActivationDecision) {
				decision.PromotionLevelClaimed = true
			},
			wantErr: "cannot claim promotion level",
		},
		{
			name: "mutation boundary crossed",
			mutate: func(decision *CandidatePackageOwnerActivationDecision) {
				decision.NoMutation = false
			},
			wantErr: "cannot cross mutation boundary",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pkg, acceptance, decision := candidatePackageDurableActivationInputs(t)
			tc.mutate(&decision)

			contract, err := BuildCandidatePackageDurableActivationContract(pkg, acceptance, decision)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackageDurableActivationContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackageProductActivationVerifierContractSelectsPackagePublicationWithoutActivating(t *testing.T) {
	durable, evidence := candidatePackageProductActivationVerifierInputs(t)
	evidence.Prerequisites = []CandidatePackageProductActivationPrerequisiteEvidence{
		{
			Prerequisite: CandidatePackageProductActivationPrerequisitePackagePublication,
			Status:       CandidatePackageProductActivationEvidenceStatusCandidate,
			EvidenceRef:  "publication-proof:candidate-package-local-acceptance-adoption-1",
		},
	}

	verifier, err := BuildCandidatePackageProductActivationVerifierContract(durable, evidence)
	if err != nil {
		t.Fatalf("build candidate package product activation verifier contract: %v", err)
	}

	if verifier.Kind != CandidatePackageProductActivationVerifierContractKind {
		t.Fatalf("kind = %q, want %q", verifier.Kind, CandidatePackageProductActivationVerifierContractKind)
	}
	if verifier.CandidatePackageID != durable.CandidatePackageID || verifier.CandidatePackageManifestSHA256 != durable.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", verifier.CandidatePackageID, verifier.CandidatePackageManifestSHA256, durable.CandidatePackageID, durable.CandidatePackageManifestSHA256)
	}
	if verifier.Version != durable.Version {
		t.Fatalf("version = %#v, want %#v", verifier.Version, durable.Version)
	}
	if verifier.UsesLocalAcceptanceID != durable.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", verifier.UsesLocalAcceptanceID, durable.UsesLocalAcceptanceID)
	}
	if verifier.FirstBindablePrerequisite != CandidatePackageProductActivationPrerequisitePackagePublication {
		t.Fatalf("first bindable prerequisite = %q, want %q", verifier.FirstBindablePrerequisite, CandidatePackageProductActivationPrerequisitePackagePublication)
	}
	if verifier.FirstBindableEvidenceRef != "publication-proof:candidate-package-local-acceptance-adoption-1" {
		t.Fatalf("first bindable evidence ref = %q, want package-publication proof candidate", verifier.FirstBindableEvidenceRef)
	}
	if verifier.FirstBindableStatus != CandidatePackageProductActivationVerifierStatusBindable {
		t.Fatalf("first bindable status = %q, want %q", verifier.FirstBindableStatus, CandidatePackageProductActivationVerifierStatusBindable)
	}
	assertProductActivationVerifierRefusesActivation(t, verifier)
	for _, want := range []string{
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		assertStringSliceContains(t, verifier.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackageProductActivationVerifierContractRejectsMissingOrMismatchedDurableIdentity(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-product-activation-verifier", ArtifactProgramRef: "tape:org/foreign-product-activation-verifier@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackageDurableActivationContract, *CandidatePackageProductActivationEvidence)
		wantErr string
	}{
		{
			name: "missing durable package id",
			mutate: func(durable *CandidatePackageDurableActivationContract, _ *CandidatePackageProductActivationEvidence) {
				durable.CandidatePackageID = "  "
			},
			wantErr: "durable contract package id is required",
		},
		{
			name: "evidence package id mismatch",
			mutate: func(_ *CandidatePackageDurableActivationContract, evidence *CandidatePackageProductActivationEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "missing durable package hash",
			mutate: func(durable *CandidatePackageDurableActivationContract, _ *CandidatePackageProductActivationEvidence) {
				durable.CandidatePackageManifestSHA256 = "  "
			},
			wantErr: "durable contract package hash is required",
		},
		{
			name: "evidence package hash mismatch",
			mutate: func(_ *CandidatePackageDurableActivationContract, evidence *CandidatePackageProductActivationEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("2", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "evidence version mismatch",
			mutate: func(_ *CandidatePackageDurableActivationContract, evidence *CandidatePackageProductActivationEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "missing durable local acceptance id",
			mutate: func(durable *CandidatePackageDurableActivationContract, _ *CandidatePackageProductActivationEvidence) {
				durable.UsesLocalAcceptanceID = "  "
			},
			wantErr: "durable contract acceptance id is required",
		},
		{
			name: "evidence local acceptance id mismatch",
			mutate: func(_ *CandidatePackageDurableActivationContract, evidence *CandidatePackageProductActivationEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			durable, evidence := candidatePackageProductActivationVerifierInputs(t)
			evidence.Prerequisites = []CandidatePackageProductActivationPrerequisiteEvidence{
				{
					Prerequisite: CandidatePackageProductActivationPrerequisitePackagePublication,
					Status:       CandidatePackageProductActivationEvidenceStatusCandidate,
					EvidenceRef:  "publication-proof:candidate-package-local-acceptance-adoption-1",
				},
			}
			tc.mutate(&durable, &evidence)

			verifier, err := BuildCandidatePackageProductActivationVerifierContract(durable, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackageProductActivationVerifierContract() = verifier %#v error %v, want error containing %q", verifier, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackageProductActivationVerifierContractBlocksPassedPrerequisitesWithoutEvidence(t *testing.T) {
	for _, prerequisite := range []string{
		CandidatePackageProductActivationPrerequisitePackagePublication,
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		t.Run(prerequisite, func(t *testing.T) {
			durable, evidence := candidatePackageProductActivationVerifierInputs(t)
			evidence.Prerequisites = []CandidatePackageProductActivationPrerequisiteEvidence{
				{
					Prerequisite: prerequisite,
					Status:       CandidatePackageProductActivationEvidenceStatusPassed,
					EvidenceRef:  "  ",
				},
			}

			verifier, err := BuildCandidatePackageProductActivationVerifierContract(durable, evidence)
			if err != nil {
				return
			}
			assertProductActivationVerifierRefusesActivation(t, verifier)
			assertStringSliceContains(t, verifier.BlockedPrerequisites, prerequisite)
			if verifier.FirstBindablePrerequisite == prerequisite && verifier.FirstBindableStatus == CandidatePackageProductActivationVerifierStatusBindable {
				t.Fatalf("prerequisite %q was bindable without explicit evidence: %#v", prerequisite, verifier)
			}
		})
	}
}

func TestBuildCandidatePackageProductActivationVerifierContractBlocksUnsafeFirstSliceClaims(t *testing.T) {
	for _, prerequisite := range []string{
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		t.Run(prerequisite, func(t *testing.T) {
			durable, evidence := candidatePackageProductActivationVerifierInputs(t)
			evidence.Prerequisites = []CandidatePackageProductActivationPrerequisiteEvidence{
				{
					Prerequisite: prerequisite,
					Status:       CandidatePackageProductActivationEvidenceStatusCandidate,
					EvidenceRef:  "proof-candidate:" + prerequisite,
				},
			}

			verifier, err := BuildCandidatePackageProductActivationVerifierContract(durable, evidence)
			if err != nil {
				return
			}
			assertProductActivationVerifierRefusesActivation(t, verifier)
			assertStringSliceContains(t, verifier.BlockedPrerequisites, prerequisite)
			if verifier.FirstBindablePrerequisite == prerequisite {
				t.Fatalf("first verifier slice selected unsafe prerequisite %q without package-publication evidence: %#v", prerequisite, verifier)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationProofContractBindsVerifierEvidenceWithoutPublishing(t *testing.T) {
	verifier, proof := candidatePackagePublicationProofInputs(t)

	contract, err := BuildCandidatePackagePublicationProofContract(verifier, proof)
	if err != nil {
		t.Fatalf("build candidate package publication proof contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationProofContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationProofContractKind)
	}
	if contract.CandidatePackageID != verifier.CandidatePackageID || contract.CandidatePackageManifestSHA256 != verifier.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, verifier.CandidatePackageID, verifier.CandidatePackageManifestSHA256)
	}
	if contract.Version != verifier.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, verifier.Version)
	}
	if contract.UsesLocalAcceptanceID != verifier.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, verifier.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != verifier.FirstBindableEvidenceRef {
		t.Fatalf("verifier evidence ref = %q, want %q", contract.VerifierEvidenceRef, verifier.FirstBindableEvidenceRef)
	}
	if !contract.PublicationBound {
		t.Fatalf("publication bound = false, want true for the verifier-selected proof evidence")
	}
	assertPublicationProofContractRefusesActivation(t, contract)
	for _, want := range []string{
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteDeployedRoute,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationProofContractRejectsMismatchedProofIdentity(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-proof", ArtifactProgramRef: "tape:org/foreign-publication-proof@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationProofEvidence)
		wantErr string
	}{
		{
			name: "package id mismatch",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "proof package id",
		},
		{
			name: "package hash mismatch",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("3", 64)
			},
			wantErr: "proof package hash does not match",
		},
		{
			name: "version mismatch",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.Version = foreignVersion
			},
			wantErr: "proof version does not match",
		},
		{
			name: "local acceptance id mismatch",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "proof acceptance id does not match",
		},
		{
			name: "evidence ref mismatch",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.EvidenceRef = "publication-proof:foreign"
			},
			wantErr: "proof evidence ref",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier, proof := candidatePackagePublicationProofInputs(t)
			tc.mutate(&proof)

			contract, err := BuildCandidatePackagePublicationProofContract(verifier, proof)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationProofContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationProofContractRejectsVerifierWithoutBindablePublication(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackageProductActivationVerifierContract)
		wantErr string
	}{
		{
			name: "blocked verifier",
			mutate: func(verifier *CandidatePackageProductActivationVerifierContract) {
				verifier.FirstBindablePrerequisite = ""
				verifier.FirstBindableStatus = CandidatePackageProductActivationVerifierStatusBlocked
				verifier.FirstBindableEvidenceRef = ""
			},
			wantErr: "first bindable prerequisite",
		},
		{
			name: "different prerequisite selected",
			mutate: func(verifier *CandidatePackageProductActivationVerifierContract) {
				verifier.FirstBindablePrerequisite = CandidatePackageProductActivationPrerequisiteAppAdoption
			},
			wantErr: "first bindable prerequisite",
		},
		{
			name: "non-bindable status",
			mutate: func(verifier *CandidatePackageProductActivationVerifierContract) {
				verifier.FirstBindableStatus = CandidatePackageProductActivationVerifierStatusBlocked
			},
			wantErr: "first bindable status",
		},
		{
			name: "missing verifier evidence ref",
			mutate: func(verifier *CandidatePackageProductActivationVerifierContract) {
				verifier.FirstBindableEvidenceRef = "  "
			},
			wantErr: "verifier evidence ref is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier, proof := candidatePackagePublicationProofInputs(t)
			tc.mutate(&verifier)

			contract, err := BuildCandidatePackagePublicationProofContract(verifier, proof)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationProofContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationProofContractRefusesUnsafeProofClaims(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*CandidatePackagePublicationProofEvidence)
	}{
		{
			name: "actual package published",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.ActualPackagePublished = true
			},
		},
		{
			name: "AppAdoption claimed",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.ClaimsAppAdoption = true
			},
		},
		{
			name: "deployed route touched",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.TouchesDeployedRoute = true
			},
		},
		{
			name: "staging claimed",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.ClaimsStaging = true
			},
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.ClaimsVMLifecycle = true
			},
		},
		{
			name: "run acceptance claimed",
			mutate: func(proof *CandidatePackagePublicationProofEvidence) {
				proof.ClaimsRunAcceptance = true
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier, proof := candidatePackagePublicationProofInputs(t)
			tc.mutate(&proof)

			contract, err := BuildCandidatePackagePublicationProofContract(verifier, proof)
			if err != nil {
				return
			}
			if !contract.PublicationBound {
				t.Fatalf("publication bound = false after narrowing unsafe proof, want true for bound review evidence")
			}
			assertPublicationProofContractRefusesActivation(t, contract)
		})
	}
}

func TestBuildCandidatePackagePublicationPayloadContractBindsSourceDeltaAndPayloadWithoutPublishing(t *testing.T) {
	proof, evidence := candidatePackagePublicationPayloadInputs(t)

	contract, err := BuildCandidatePackagePublicationPayloadContract(proof, evidence)
	if err != nil {
		t.Fatalf("build candidate package publication payload contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationPayloadContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationPayloadContractKind)
	}
	if contract.CandidatePackageID != proof.CandidatePackageID || contract.CandidatePackageManifestSHA256 != proof.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, proof.CandidatePackageID, proof.CandidatePackageManifestSHA256)
	}
	if contract.Version != proof.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, proof.Version)
	}
	if contract.UsesLocalAcceptanceID != proof.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, proof.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != proof.VerifierEvidenceRef {
		t.Fatalf("verifier evidence ref = %q, want %q", contract.VerifierEvidenceRef, proof.VerifierEvidenceRef)
	}
	if contract.SourceDeltaRef != evidence.SourceDeltaRef {
		t.Fatalf("source delta ref = %q, want %q", contract.SourceDeltaRef, evidence.SourceDeltaRef)
	}
	if contract.PayloadManifestRef != evidence.PayloadManifestRef {
		t.Fatalf("payload manifest ref = %q, want %q", contract.PayloadManifestRef, evidence.PayloadManifestRef)
	}
	if contract.PayloadBoundary != CandidatePackagePublicationPayloadBoundary {
		t.Fatalf("payload boundary = %q, want %q", contract.PayloadBoundary, CandidatePackagePublicationPayloadBoundary)
	}
	if !contract.ReviewablePublicationCandidate {
		t.Fatalf("reviewable publication candidate = false, want true for bound source-delta and payload refs")
	}
	assertPublicationPayloadContractRefusesActivation(t, contract)
	for _, want := range []string{
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteDeployedRoute,
		CandidatePackageProductActivationPrerequisiteAuthSession,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationPayloadContractRejectsMissingOrMismatchedPayloadEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-payload", ArtifactProgramRef: "tape:org/foreign-publication-payload@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationProofContract, *CandidatePackagePublicationPayloadEvidence)
		wantErr string
	}{
		{
			name: "unbound publication proof",
			mutate: func(proof *CandidatePackagePublicationProofContract, _ *CandidatePackagePublicationPayloadEvidence) {
				proof.PublicationBound = false
			},
			wantErr: "proof must be publication-bound",
		},
		{
			name: "package id mismatch",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "package hash mismatch",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("4", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "version mismatch",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "local acceptance id mismatch",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
		{
			name: "verifier evidence ref mismatch",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.VerifierEvidenceRef = "publication-proof:foreign"
			},
			wantErr: "evidence verifier ref",
		},
		{
			name: "missing source delta ref",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.SourceDeltaRef = "  "
			},
			wantErr: "source delta ref is required",
		},
		{
			name: "missing payload manifest ref",
			mutate: func(_ *CandidatePackagePublicationProofContract, evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.PayloadManifestRef = "  "
			},
			wantErr: "payload manifest ref is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof, evidence := candidatePackagePublicationPayloadInputs(t)
			tc.mutate(&proof, &evidence)

			contract, err := BuildCandidatePackagePublicationPayloadContract(proof, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationPayloadContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationPayloadContractRejectsUnsafePublicationClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationPayloadEvidence)
		wantErr string
	}{
		{
			name: "actual package published",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ActualPackagePublished = true
			},
			wantErr: "actual package publication",
		},
		{
			name: "direct publish ready",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.DirectPublishReady = true
			},
			wantErr: "direct-publish ready",
		},
		{
			name: "AppAdoption claimed",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ClaimsAppAdoption = true
			},
			wantErr: "AppAdoption",
		},
		{
			name: "deployed route touched",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.TouchesDeployedRoute = true
			},
			wantErr: "deployed route",
		},
		{
			name: "auth session claimed",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ClaimsAuthSession = true
			},
			wantErr: "auth session",
		},
		{
			name: "staging claimed",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ClaimsStaging = true
			},
			wantErr: "staging",
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ClaimsVMLifecycle = true
			},
			wantErr: "VM lifecycle",
		},
		{
			name: "run acceptance claimed",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ClaimsRunAcceptance = true
			},
			wantErr: "run acceptance",
		},
		{
			name: "activation ready",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.ActivationReady = true
			},
			wantErr: "activation ready",
		},
		{
			name: "promotion level claimed",
			mutate: func(evidence *CandidatePackagePublicationPayloadEvidence) {
				evidence.PromotionLevelClaimed = true
			},
			wantErr: "promotion level",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof, evidence := candidatePackagePublicationPayloadInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationPayloadContract(proof, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationPayloadContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationPreflightContractBindsPayloadAndCheckRefsWithoutPublishing(t *testing.T) {
	payload, evidence := candidatePackagePublicationPreflightInputs(t)

	contract, err := BuildCandidatePackagePublicationPreflightContract(payload, evidence)
	if err != nil {
		t.Fatalf("build candidate package publication preflight contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationPreflightContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationPreflightContractKind)
	}
	if contract.CandidatePackageID != payload.CandidatePackageID || contract.CandidatePackageManifestSHA256 != payload.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, payload.CandidatePackageID, payload.CandidatePackageManifestSHA256)
	}
	if contract.Version != payload.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, payload.Version)
	}
	if contract.UsesLocalAcceptanceID != payload.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, payload.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != payload.VerifierEvidenceRef {
		t.Fatalf("verifier evidence ref = %q, want %q", contract.VerifierEvidenceRef, payload.VerifierEvidenceRef)
	}
	if contract.SourceDeltaRef != payload.SourceDeltaRef {
		t.Fatalf("source delta ref = %q, want %q", contract.SourceDeltaRef, payload.SourceDeltaRef)
	}
	if contract.PayloadManifestRef != payload.PayloadManifestRef {
		t.Fatalf("payload manifest ref = %q, want %q", contract.PayloadManifestRef, payload.PayloadManifestRef)
	}
	if contract.PreflightBoundary != CandidatePackagePublicationPreflightBoundary {
		t.Fatalf("preflight boundary = %q, want %q", contract.PreflightBoundary, CandidatePackagePublicationPreflightBoundary)
	}
	wantCheckRefs := []string{
		"preflight:manifest-sha256-matches-payload",
		"preflight:payload-manifest-readonly",
		"preflight:source-delta-readonly",
	}
	if !reflect.DeepEqual(contract.PreflightCheckRefs, wantCheckRefs) {
		t.Fatalf("preflight check refs = %#v, want %#v", contract.PreflightCheckRefs, wantCheckRefs)
	}
	wantVerifierRefs := []string{
		"verifier:payload-review-committee",
		"verifier:publish-token-absence",
	}
	if !reflect.DeepEqual(contract.VerifierContractRefs, wantVerifierRefs) {
		t.Fatalf("verifier contract refs = %#v, want %#v", contract.VerifierContractRefs, wantVerifierRefs)
	}
	assertPublicationPreflightContractRefusesExecution(t, contract)
	for _, want := range []string{
		CandidatePackageProductActivationPrerequisitePackagePublication,
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteDeployedRoute,
		CandidatePackageProductActivationPrerequisiteAuthSession,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationPreflightContractRejectsMissingOrMismatchedPayloadEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-preflight", ArtifactProgramRef: "tape:org/foreign-publication-preflight@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationPayloadContract, *CandidatePackagePublicationPreflightEvidence)
		wantErr string
	}{
		{
			name: "payload kind mismatch",
			mutate: func(payload *CandidatePackagePublicationPayloadContract, _ *CandidatePackagePublicationPreflightEvidence) {
				payload.Kind = "candidate_package_publication_executor"
			},
			wantErr: "payload kind",
		},
		{
			name: "payload package id missing",
			mutate: func(payload *CandidatePackagePublicationPayloadContract, _ *CandidatePackagePublicationPreflightEvidence) {
				payload.CandidatePackageID = "  "
			},
			wantErr: "payload package id is required",
		},
		{
			name: "package id mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "package hash mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("5", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "version mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "local acceptance id mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
		{
			name: "verifier evidence ref mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.VerifierEvidenceRef = "publication-proof:foreign"
			},
			wantErr: "evidence verifier ref",
		},
		{
			name: "source delta ref mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.SourceDeltaRef = "source-delta:foreign"
			},
			wantErr: "evidence source delta ref",
		},
		{
			name: "payload manifest ref mismatch",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.PayloadManifestRef = "payload-manifest:foreign"
			},
			wantErr: "evidence payload manifest ref",
		},
		{
			name: "missing preflight check refs",
			mutate: func(_ *CandidatePackagePublicationPayloadContract, evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.PreflightCheckRefs = []string{" ", ""}
			},
			wantErr: "preflight check refs are required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, evidence := candidatePackagePublicationPreflightInputs(t)
			tc.mutate(&payload, &evidence)

			contract, err := BuildCandidatePackagePublicationPreflightContract(payload, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationPreflightContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationPreflightContractRejectsUnsafePublicationClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationPreflightEvidence)
		wantErr string
	}{
		{
			name: "executor allowed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ExecutorAllowed = true
			},
			wantErr: "allow executor",
		},
		{
			name: "actual package published",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ActualPackagePublished = true
			},
			wantErr: "actual package publication",
		},
		{
			name: "direct publish ready",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.DirectPublishReady = true
			},
			wantErr: "direct-publish ready",
		},
		{
			name: "AppAdoption claimed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ClaimsAppAdoption = true
			},
			wantErr: "AppAdoption",
		},
		{
			name: "deployed route touched",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.TouchesDeployedRoute = true
			},
			wantErr: "deployed route",
		},
		{
			name: "auth session claimed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ClaimsAuthSession = true
			},
			wantErr: "auth session",
		},
		{
			name: "staging claimed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ClaimsStaging = true
			},
			wantErr: "staging",
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ClaimsVMLifecycle = true
			},
			wantErr: "VM lifecycle",
		},
		{
			name: "run acceptance claimed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ClaimsRunAcceptance = true
			},
			wantErr: "run acceptance",
		},
		{
			name: "activation ready",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.ActivationReady = true
			},
			wantErr: "activation ready",
		},
		{
			name: "promotion level claimed",
			mutate: func(evidence *CandidatePackagePublicationPreflightEvidence) {
				evidence.PromotionLevelClaimed = true
			},
			wantErr: "promotion level",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, evidence := candidatePackagePublicationPreflightInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationPreflightContract(payload, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationPreflightContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorReviewGateContractBindsPreflightAndAuthorizationRefsWithoutPublishing(t *testing.T) {
	preflight, evidence := candidatePackagePublicationExecutorReviewGateInputs(t)

	contract, err := BuildCandidatePackagePublicationExecutorReviewGateContract(preflight, evidence)
	if err != nil {
		t.Fatalf("build candidate package publication executor review gate contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationExecutorReviewGateContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationExecutorReviewGateContractKind)
	}
	if contract.CandidatePackageID != preflight.CandidatePackageID || contract.CandidatePackageManifestSHA256 != preflight.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, preflight.CandidatePackageID, preflight.CandidatePackageManifestSHA256)
	}
	if contract.Version != preflight.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, preflight.Version)
	}
	if contract.UsesLocalAcceptanceID != preflight.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, preflight.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != preflight.VerifierEvidenceRef {
		t.Fatalf("verifier evidence ref = %q, want %q", contract.VerifierEvidenceRef, preflight.VerifierEvidenceRef)
	}
	if contract.SourceDeltaRef != preflight.SourceDeltaRef {
		t.Fatalf("source delta ref = %q, want %q", contract.SourceDeltaRef, preflight.SourceDeltaRef)
	}
	if contract.PayloadManifestRef != preflight.PayloadManifestRef {
		t.Fatalf("payload manifest ref = %q, want %q", contract.PayloadManifestRef, preflight.PayloadManifestRef)
	}
	if contract.ReviewGateBoundary != CandidatePackagePublicationExecutorReviewGateBoundary {
		t.Fatalf("review gate boundary = %q, want %q", contract.ReviewGateBoundary, CandidatePackagePublicationExecutorReviewGateBoundary)
	}
	if !reflect.DeepEqual(contract.PreflightCheckRefs, preflight.PreflightCheckRefs) {
		t.Fatalf("preflight check refs = %#v, want %#v", contract.PreflightCheckRefs, preflight.PreflightCheckRefs)
	}
	if !reflect.DeepEqual(contract.VerifierContractRefs, preflight.VerifierContractRefs) {
		t.Fatalf("verifier contract refs = %#v, want %#v", contract.VerifierContractRefs, preflight.VerifierContractRefs)
	}
	if contract.OwnerAuthorizationRef != strings.TrimSpace(evidence.OwnerAuthorizationRef) {
		t.Fatalf("owner authorization ref = %q, want trimmed %q", contract.OwnerAuthorizationRef, strings.TrimSpace(evidence.OwnerAuthorizationRef))
	}
	if contract.ReviewerAuthorizationRef != strings.TrimSpace(evidence.ReviewerAuthorizationRef) {
		t.Fatalf("reviewer authorization ref = %q, want trimmed %q", contract.ReviewerAuthorizationRef, strings.TrimSpace(evidence.ReviewerAuthorizationRef))
	}
	assertPublicationExecutorReviewGateContractRefusesExecution(t, contract)
	for _, want := range []string{
		CandidatePackageProductActivationPrerequisitePackagePublication,
		CandidatePackageProductActivationPrerequisiteAppAdoption,
		CandidatePackageProductActivationPrerequisiteDeployedRoute,
		CandidatePackageProductActivationPrerequisiteAuthSession,
		CandidatePackageProductActivationPrerequisiteStagingAcceptance,
		CandidatePackageProductActivationPrerequisiteVMLifecycle,
		CandidatePackageProductActivationPrerequisiteRunAcceptance,
	} {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationExecutorReviewGateContractRejectsMalformedOrMismatchedPreflightEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-executor-review-gate", ArtifactProgramRef: "tape:org/foreign-publication-executor-review-gate@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationPreflightContract, *CandidatePackagePublicationExecutorReviewGateEvidence)
		wantErr string
	}{
		{
			name: "preflight kind mismatch",
			mutate: func(preflight *CandidatePackagePublicationPreflightContract, _ *CandidatePackagePublicationExecutorReviewGateEvidence) {
				preflight.Kind = "candidate_package_publication_executor"
			},
			wantErr: "preflight kind",
		},
		{
			name: "preflight package id missing",
			mutate: func(preflight *CandidatePackagePublicationPreflightContract, _ *CandidatePackagePublicationExecutorReviewGateEvidence) {
				preflight.CandidatePackageID = "  "
			},
			wantErr: "preflight package id is required",
		},
		{
			name: "preflight boundary mismatch",
			mutate: func(preflight *CandidatePackagePublicationPreflightContract, _ *CandidatePackagePublicationExecutorReviewGateEvidence) {
				preflight.PreflightBoundary = "package_publication_executor_allowed"
			},
			wantErr: "preflight boundary",
		},
		{
			name: "preflight check refs missing",
			mutate: func(preflight *CandidatePackagePublicationPreflightContract, _ *CandidatePackagePublicationExecutorReviewGateEvidence) {
				preflight.PreflightCheckRefs = []string{" ", ""}
			},
			wantErr: "preflight check refs are required",
		},
		{
			name: "preflight allows executor",
			mutate: func(preflight *CandidatePackagePublicationPreflightContract, _ *CandidatePackagePublicationExecutorReviewGateEvidence) {
				preflight.ExecutorAllowed = true
			},
			wantErr: "preflight cannot allow executor",
		},
		{
			name: "evidence package id mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "evidence package hash mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("6", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "evidence version mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "evidence acceptance id mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
		{
			name: "evidence verifier evidence ref mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.VerifierEvidenceRef = "publication-proof:foreign"
			},
			wantErr: "evidence verifier ref",
		},
		{
			name: "evidence source delta ref mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.SourceDeltaRef = "source-delta:foreign"
			},
			wantErr: "evidence source delta ref",
		},
		{
			name: "evidence payload manifest ref mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.PayloadManifestRef = "payload-manifest:foreign"
			},
			wantErr: "evidence payload manifest ref",
		},
		{
			name: "preflight check refs mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.PreflightCheckRefs = []string{"preflight:manifest-sha256-matches-payload", "preflight:source-delta-readonly", "preflight:foreign"}
			},
			wantErr: "evidence preflight check refs",
		},
		{
			name: "verifier contract refs mismatch",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.VerifierContractRefs = []string{"verifier:payload-review-committee", "verifier:foreign"}
			},
			wantErr: "evidence verifier contract refs",
		},
		{
			name: "owner authorization ref missing",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.OwnerAuthorizationRef = " "
			},
			wantErr: "owner authorization ref is required",
		},
		{
			name: "reviewer authorization ref missing",
			mutate: func(_ *CandidatePackagePublicationPreflightContract, evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ReviewerAuthorizationRef = " "
			},
			wantErr: "reviewer authorization ref is required",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preflight, evidence := candidatePackagePublicationExecutorReviewGateInputs(t)
			tc.mutate(&preflight, &evidence)

			contract, err := BuildCandidatePackagePublicationExecutorReviewGateContract(preflight, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorReviewGateContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorReviewGateContractRejectsUnsafePublicationClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorReviewGateEvidence)
		wantErr string
	}{
		{
			name: "executor allowed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ExecutorAllowed = true
			},
			wantErr: "allow executor",
		},
		{
			name: "actual package published",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ActualPackagePublished = true
			},
			wantErr: "actual package publication",
		},
		{
			name: "direct publish ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.DirectPublishReady = true
			},
			wantErr: "direct-publish ready",
		},
		{
			name: "AppAdoption claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ClaimsAppAdoption = true
			},
			wantErr: "AppAdoption",
		},
		{
			name: "deployed route touched",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.TouchesDeployedRoute = true
			},
			wantErr: "deployed route",
		},
		{
			name: "auth session claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ClaimsAuthSession = true
			},
			wantErr: "auth session",
		},
		{
			name: "staging claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ClaimsStaging = true
			},
			wantErr: "staging",
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ClaimsVMLifecycle = true
			},
			wantErr: "VM lifecycle",
		},
		{
			name: "run acceptance claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ClaimsRunAcceptance = true
			},
			wantErr: "run acceptance",
		},
		{
			name: "activation ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.ActivationReady = true
			},
			wantErr: "activation ready",
		},
		{
			name: "promotion level claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReviewGateEvidence) {
				evidence.PromotionLevelClaimed = true
			},
			wantErr: "promotion level",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preflight, evidence := candidatePackagePublicationExecutorReviewGateInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorReviewGateContract(preflight, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorReviewGateContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorDesignSpecContractBindsReviewGateDesignEvidenceWithoutImplementation(t *testing.T) {
	gate, evidence := candidatePackagePublicationExecutorDesignSpecInputs(t)

	contract, err := BuildCandidatePackagePublicationExecutorDesignSpecContract(gate, evidence)
	if err != nil {
		t.Fatalf("build candidate package publication executor design spec contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationExecutorDesignSpecContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationExecutorDesignSpecContractKind)
	}
	if contract.CandidatePackageID != gate.CandidatePackageID || contract.CandidatePackageManifestSHA256 != gate.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, gate.CandidatePackageID, gate.CandidatePackageManifestSHA256)
	}
	if contract.Version != gate.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, gate.Version)
	}
	if contract.UsesLocalAcceptanceID != gate.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, gate.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != gate.VerifierEvidenceRef {
		t.Fatalf("verifier evidence ref = %q, want %q", contract.VerifierEvidenceRef, gate.VerifierEvidenceRef)
	}
	if contract.SourceDeltaRef != gate.SourceDeltaRef {
		t.Fatalf("source delta ref = %q, want %q", contract.SourceDeltaRef, gate.SourceDeltaRef)
	}
	if contract.PayloadManifestRef != gate.PayloadManifestRef {
		t.Fatalf("payload manifest ref = %q, want %q", contract.PayloadManifestRef, gate.PayloadManifestRef)
	}
	if contract.DesignSpecBoundary != CandidatePackagePublicationExecutorDesignSpecBoundary {
		t.Fatalf("design spec boundary = %q, want %q", contract.DesignSpecBoundary, CandidatePackagePublicationExecutorDesignSpecBoundary)
	}
	if contract.OwnerAuthorizationRef != strings.TrimSpace(gate.OwnerAuthorizationRef) {
		t.Fatalf("owner authorization ref = %q, want trimmed %q", contract.OwnerAuthorizationRef, strings.TrimSpace(gate.OwnerAuthorizationRef))
	}
	if contract.ReviewerAuthorizationRef != strings.TrimSpace(gate.ReviewerAuthorizationRef) {
		t.Fatalf("reviewer authorization ref = %q, want trimmed %q", contract.ReviewerAuthorizationRef, strings.TrimSpace(gate.ReviewerAuthorizationRef))
	}
	if contract.ExecutorDesignSpecRef != strings.TrimSpace(evidence.ExecutorDesignSpecRef) {
		t.Fatalf("executor design spec ref = %q, want trimmed %q", contract.ExecutorDesignSpecRef, strings.TrimSpace(evidence.ExecutorDesignSpecRef))
	}
	if !reflect.DeepEqual(contract.RequiredRedSurfaces, canonicalStrings(evidence.RequiredRedSurfaces)) {
		t.Fatalf("required red surfaces = %#v, want canonical %#v", contract.RequiredRedSurfaces, canonicalStrings(evidence.RequiredRedSurfaces))
	}
	if !reflect.DeepEqual(contract.RequiredEvidenceRefs, canonicalStrings(evidence.RequiredEvidenceRefs)) {
		t.Fatalf("required evidence refs = %#v, want canonical %#v", contract.RequiredEvidenceRefs, canonicalStrings(evidence.RequiredEvidenceRefs))
	}
	if contract.RollbackPlanRef != strings.TrimSpace(evidence.RollbackPlanRef) {
		t.Fatalf("rollback plan ref = %q, want trimmed %q", contract.RollbackPlanRef, strings.TrimSpace(evidence.RollbackPlanRef))
	}
	assertPublicationExecutorDesignSpecContractRefusesExecution(t, contract)
	for _, want := range productActivationProtectedPrerequisites() {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationExecutorDesignSpecContractRejectsMalformedOrMismatchedReviewGateAndEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-executor-design-spec", ArtifactProgramRef: "tape:org/foreign-publication-executor-design-spec@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorReviewGateContract, *CandidatePackagePublicationExecutorDesignSpecEvidence)
		wantErr string
	}{
		{
			name: "review gate kind mismatch",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.Kind = "candidate_package_publication_executor_design_spec"
			},
			wantErr: "review gate kind",
		},
		{
			name: "review gate package id missing",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.CandidatePackageID = "  "
			},
			wantErr: "review gate package id is required",
		},
		{
			name: "review gate boundary mismatch",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.ReviewGateBoundary = "package_publication_executor_red_design_spec_without_implementation"
			},
			wantErr: "review gate boundary",
		},
		{
			name: "review gate owner authorization missing",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.OwnerAuthorizationRef = " "
			},
			wantErr: "review gate owner authorization ref is required",
		},
		{
			name: "review gate reviewer authorization missing",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.ReviewerAuthorizationRef = " "
			},
			wantErr: "review gate reviewer authorization ref is required",
		},
		{
			name: "review gate not design-review-ready",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.ExecutorDesignReviewReady = false
			},
			wantErr: "design-review-ready",
		},
		{
			name: "review gate allows executor",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.ExecutorAllowed = true
			},
			wantErr: "review gate cannot allow executor",
		},
		{
			name: "review gate mutates",
			mutate: func(gate *CandidatePackagePublicationExecutorReviewGateContract, _ *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				gate.NoMutation = false
			},
			wantErr: "review gate must be no-mutation",
		},
		{
			name: "evidence package id mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "evidence package hash mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("7", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "evidence version mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "evidence acceptance id mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
		{
			name: "evidence verifier evidence ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.VerifierEvidenceRef = "publication-proof:foreign"
			},
			wantErr: "evidence verifier ref",
		},
		{
			name: "evidence source delta ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.SourceDeltaRef = "source-delta:foreign"
			},
			wantErr: "evidence source delta ref",
		},
		{
			name: "evidence payload manifest ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.PayloadManifestRef = "payload-manifest:foreign"
			},
			wantErr: "evidence payload manifest ref",
		},
		{
			name: "evidence owner authorization ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.OwnerAuthorizationRef = "owner-auth:foreign"
			},
			wantErr: "evidence owner authorization ref",
		},
		{
			name: "evidence reviewer authorization ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorReviewGateContract, evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ReviewerAuthorizationRef = "reviewer-auth:foreign"
			},
			wantErr: "evidence reviewer authorization ref",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate, evidence := candidatePackagePublicationExecutorDesignSpecInputs(t)
			tc.mutate(&gate, &evidence)

			contract, err := BuildCandidatePackagePublicationExecutorDesignSpecContract(gate, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorDesignSpecContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorDesignSpecContractRejectsMissingOrUnsupportedDesignEvidence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorDesignSpecEvidence)
		wantErr string
	}{
		{
			name: "executor design spec ref missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ExecutorDesignSpecRef = " "
			},
			wantErr: "executor design spec ref is required",
		},
		{
			name: "required evidence refs missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredEvidenceRefs = []string{" ", ""}
			},
			wantErr: "required evidence refs are required",
		},
		{
			name: "rollback plan ref missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RollbackPlanRef = " "
			},
			wantErr: "rollback plan ref is required",
		},
		{
			name: "required red surfaces missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredRedSurfaces = []string{" ", ""}
			},
			wantErr: "required red surfaces are required",
		},
		{
			name: "required red surface unsupported",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredRedSurfaces = append(evidence.RequiredRedSurfaces, "package_publication_network_egress")
			},
			wantErr: "unsupported red surface",
		},
		{
			name: "package artifact red surface missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredRedSurfaces = []string{
					CandidatePackagePublicationExecutorRedSurfaceProviderCredentials,
					CandidatePackagePublicationExecutorRedSurfacePublicationLedger,
					CandidatePackagePublicationExecutorRedSurfaceRollbackPath,
				}
			},
			wantErr: `required red surface "package_artifact_publication" is missing`,
		},
		{
			name: "provider credentials red surface missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredRedSurfaces = []string{
					CandidatePackagePublicationExecutorRedSurfacePackageArtifact,
					CandidatePackagePublicationExecutorRedSurfacePublicationLedger,
					CandidatePackagePublicationExecutorRedSurfaceRollbackPath,
				}
			},
			wantErr: `required red surface "provider_publish_credentials" is missing`,
		},
		{
			name: "publication ledger red surface missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredRedSurfaces = []string{
					CandidatePackagePublicationExecutorRedSurfacePackageArtifact,
					CandidatePackagePublicationExecutorRedSurfaceProviderCredentials,
					CandidatePackagePublicationExecutorRedSurfaceRollbackPath,
				}
			},
			wantErr: `required red surface "publication_ledger_write" is missing`,
		},
		{
			name: "rollback path red surface missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.RequiredRedSurfaces = []string{
					CandidatePackagePublicationExecutorRedSurfacePackageArtifact,
					CandidatePackagePublicationExecutorRedSurfaceProviderCredentials,
					CandidatePackagePublicationExecutorRedSurfacePublicationLedger,
				}
			},
			wantErr: `required red surface "rollback_path" is missing`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate, evidence := candidatePackagePublicationExecutorDesignSpecInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorDesignSpecContract(gate, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorDesignSpecContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorDesignSpecContractRejectsUnsafePublicationClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorDesignSpecEvidence)
		wantErr string
	}{
		{
			name: "executor implemented",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ExecutorImplemented = true
			},
			wantErr: "implement executor",
		},
		{
			name: "executor allowed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ExecutorAllowed = true
			},
			wantErr: "allow executor",
		},
		{
			name: "actual package published",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ActualPackagePublished = true
			},
			wantErr: "actual package publication",
		},
		{
			name: "direct publish ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.DirectPublishReady = true
			},
			wantErr: "direct-publish ready",
		},
		{
			name: "AppAdoption claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ClaimsAppAdoption = true
			},
			wantErr: "AppAdoption",
		},
		{
			name: "deployed route touched",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.TouchesDeployedRoute = true
			},
			wantErr: "deployed route",
		},
		{
			name: "auth session claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ClaimsAuthSession = true
			},
			wantErr: "auth session",
		},
		{
			name: "staging claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ClaimsStaging = true
			},
			wantErr: "staging",
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ClaimsVMLifecycle = true
			},
			wantErr: "VM lifecycle",
		},
		{
			name: "run acceptance claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ClaimsRunAcceptance = true
			},
			wantErr: "run acceptance",
		},
		{
			name: "activation ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.ActivationReady = true
			},
			wantErr: "activation ready",
		},
		{
			name: "promotion level claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorDesignSpecEvidence) {
				evidence.PromotionLevelClaimed = true
			},
			wantErr: "promotion level",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate, evidence := candidatePackagePublicationExecutorDesignSpecInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorDesignSpecContract(gate, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorDesignSpecContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorImplementationReadinessContractBindsDesignSpecReadinessEvidenceWithoutImplementation(t *testing.T) {
	design, evidence := candidatePackagePublicationExecutorImplementationReadinessInputs(t)

	contract, err := BuildCandidatePackagePublicationExecutorImplementationReadinessContract(design, evidence)
	if err != nil {
		t.Fatalf("build candidate package publication executor implementation readiness contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationExecutorImplementationReadinessContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationExecutorImplementationReadinessContractKind)
	}
	if contract.CandidatePackageID != design.CandidatePackageID || contract.CandidatePackageManifestSHA256 != design.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, design.CandidatePackageID, design.CandidatePackageManifestSHA256)
	}
	if contract.Version != design.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, design.Version)
	}
	if contract.UsesLocalAcceptanceID != design.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, design.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != strings.TrimSpace(design.VerifierEvidenceRef) {
		t.Fatalf("verifier evidence ref = %q, want trimmed %q", contract.VerifierEvidenceRef, strings.TrimSpace(design.VerifierEvidenceRef))
	}
	if contract.SourceDeltaRef != strings.TrimSpace(design.SourceDeltaRef) {
		t.Fatalf("source delta ref = %q, want trimmed %q", contract.SourceDeltaRef, strings.TrimSpace(design.SourceDeltaRef))
	}
	if contract.PayloadManifestRef != strings.TrimSpace(design.PayloadManifestRef) {
		t.Fatalf("payload manifest ref = %q, want trimmed %q", contract.PayloadManifestRef, strings.TrimSpace(design.PayloadManifestRef))
	}
	if contract.ImplementationReadinessBoundary != CandidatePackagePublicationExecutorImplementationReadinessBoundary {
		t.Fatalf("implementation readiness boundary = %q, want %q", contract.ImplementationReadinessBoundary, CandidatePackagePublicationExecutorImplementationReadinessBoundary)
	}
	if contract.OwnerAuthorizationRef != strings.TrimSpace(design.OwnerAuthorizationRef) {
		t.Fatalf("owner authorization ref = %q, want trimmed %q", contract.OwnerAuthorizationRef, strings.TrimSpace(design.OwnerAuthorizationRef))
	}
	if contract.ReviewerAuthorizationRef != strings.TrimSpace(design.ReviewerAuthorizationRef) {
		t.Fatalf("reviewer authorization ref = %q, want trimmed %q", contract.ReviewerAuthorizationRef, strings.TrimSpace(design.ReviewerAuthorizationRef))
	}
	if contract.ExecutorDesignSpecRef != strings.TrimSpace(design.ExecutorDesignSpecRef) {
		t.Fatalf("executor design spec ref = %q, want trimmed %q", contract.ExecutorDesignSpecRef, strings.TrimSpace(design.ExecutorDesignSpecRef))
	}
	if !reflect.DeepEqual(contract.RequiredRedSurfaces, canonicalStrings(design.RequiredRedSurfaces)) {
		t.Fatalf("required red surfaces = %#v, want canonical %#v", contract.RequiredRedSurfaces, canonicalStrings(design.RequiredRedSurfaces))
	}
	if !reflect.DeepEqual(contract.RequiredEvidenceRefs, canonicalStrings(design.RequiredEvidenceRefs)) {
		t.Fatalf("required evidence refs = %#v, want canonical %#v", contract.RequiredEvidenceRefs, canonicalStrings(design.RequiredEvidenceRefs))
	}
	if contract.RollbackPlanRef != strings.TrimSpace(design.RollbackPlanRef) {
		t.Fatalf("rollback plan ref = %q, want trimmed %q", contract.RollbackPlanRef, strings.TrimSpace(design.RollbackPlanRef))
	}
	if contract.RedCeremonyPlanRef != strings.TrimSpace(evidence.RedCeremonyPlanRef) {
		t.Fatalf("red ceremony plan ref = %q, want trimmed %q", contract.RedCeremonyPlanRef, strings.TrimSpace(evidence.RedCeremonyPlanRef))
	}
	if !reflect.DeepEqual(contract.RequiredGateRefs, canonicalStrings(evidence.RequiredGateRefs)) {
		t.Fatalf("required gate refs = %#v, want canonical %#v", contract.RequiredGateRefs, canonicalStrings(evidence.RequiredGateRefs))
	}
	if !reflect.DeepEqual(contract.EvidenceGateRefs, canonicalStrings(evidence.EvidenceGateRefs)) {
		t.Fatalf("evidence gate refs = %#v, want canonical %#v", contract.EvidenceGateRefs, canonicalStrings(evidence.EvidenceGateRefs))
	}
	if contract.RollbackDrillRef != strings.TrimSpace(evidence.RollbackDrillRef) {
		t.Fatalf("rollback drill ref = %q, want trimmed %q", contract.RollbackDrillRef, strings.TrimSpace(evidence.RollbackDrillRef))
	}
	assertPublicationExecutorImplementationReadinessContractRefusesExecution(t, contract)
	for _, want := range productActivationProtectedPrerequisites() {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationExecutorImplementationReadinessContractRejectsMalformedOrMismatchedDesignSpecAndEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-executor-implementation-readiness", ArtifactProgramRef: "tape:org/foreign-publication-executor-implementation-readiness@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorDesignSpecContract, *CandidatePackagePublicationExecutorImplementationReadinessEvidence)
		wantErr string
	}{
		{
			name: "design spec kind mismatch",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.Kind = "candidate_package_publication_executor_implementation_readiness"
			},
			wantErr: "design spec kind",
		},
		{
			name: "design spec package id missing",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.CandidatePackageID = "  "
			},
			wantErr: "design spec package id is required",
		},
		{
			name: "design spec boundary mismatch",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.DesignSpecBoundary = "package_publication_executor_implementation_readiness_without_code"
			},
			wantErr: "design spec boundary",
		},
		{
			name: "design spec owner authorization missing",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.OwnerAuthorizationRef = " "
			},
			wantErr: "design spec owner authorization ref is required",
		},
		{
			name: "design spec not ready",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.ExecutorDesignSpecReady = false
			},
			wantErr: "design spec must be ready",
		},
		{
			name: "design spec implements executor",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.ExecutorImplemented = true
			},
			wantErr: "design spec cannot implement executor",
		},
		{
			name: "design spec mutates",
			mutate: func(design *CandidatePackagePublicationExecutorDesignSpecContract, _ *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				design.NoMutation = false
			},
			wantErr: "design spec must be no-mutation",
		},
		{
			name: "evidence package id mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "evidence package hash mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("6", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "evidence version mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "evidence acceptance id mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
		{
			name: "evidence verifier evidence ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.VerifierEvidenceRef = "publication-proof:foreign"
			},
			wantErr: "evidence verifier ref",
		},
		{
			name: "evidence source delta ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.SourceDeltaRef = "source-delta:foreign"
			},
			wantErr: "evidence source delta ref",
		},
		{
			name: "evidence payload manifest ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.PayloadManifestRef = "payload-manifest:foreign"
			},
			wantErr: "evidence payload manifest ref",
		},
		{
			name: "evidence owner authorization ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.OwnerAuthorizationRef = "owner-auth:foreign"
			},
			wantErr: "evidence owner authorization ref",
		},
		{
			name: "evidence reviewer authorization ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ReviewerAuthorizationRef = "reviewer-auth:foreign"
			},
			wantErr: "evidence reviewer authorization ref",
		},
		{
			name: "evidence executor design spec ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ExecutorDesignSpecRef = "executor-design-spec:foreign"
			},
			wantErr: "evidence executor design spec ref",
		},
		{
			name: "evidence required red surfaces mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredRedSurfaces = []string{CandidatePackagePublicationExecutorRedSurfacePackageArtifact}
			},
			wantErr: "evidence required red surfaces",
		},
		{
			name: "evidence required evidence refs mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredEvidenceRefs = []string{"executor-design:foreign"}
			},
			wantErr: "evidence required evidence refs",
		},
		{
			name: "evidence rollback plan ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorDesignSpecContract, evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RollbackPlanRef = "rollback-plan:foreign"
			},
			wantErr: "evidence rollback plan ref",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			design, evidence := candidatePackagePublicationExecutorImplementationReadinessInputs(t)
			tc.mutate(&design, &evidence)

			contract, err := BuildCandidatePackagePublicationExecutorImplementationReadinessContract(design, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorImplementationReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorImplementationReadinessContractRejectsMissingOrUnsupportedImplementationEvidence(t *testing.T) {
	requiredGatesWithout := func(missing string) []string {
		var gates []string
		for _, gate := range publicationExecutorRequiredImplementationGates() {
			if gate != missing {
				gates = append(gates, gate)
			}
		}
		return gates
	}

	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorImplementationReadinessEvidence)
		wantErr string
	}{
		{
			name: "red ceremony plan ref missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RedCeremonyPlanRef = " "
			},
			wantErr: "red ceremony plan ref is required",
		},
		{
			name: "evidence gate refs missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.EvidenceGateRefs = []string{" ", ""}
			},
			wantErr: "evidence gate refs are required",
		},
		{
			name: "rollback drill ref missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RollbackDrillRef = " "
			},
			wantErr: "rollback drill ref is required",
		},
		{
			name: "required gate refs missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = []string{" ", ""}
			},
			wantErr: "required gate refs are required",
		},
		{
			name: "required gate unsupported",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = append(evidence.RequiredGateRefs, "provider_network_egress_required")
			},
			wantErr: "unsupported implementation gate",
		},
		{
			name: "red ceremony gate missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = requiredGatesWithout(CandidatePackagePublicationExecutorImplementationGateRedCeremony)
			},
			wantErr: `required implementation gate "red_ceremony_required" is missing`,
		},
		{
			name: "owner approval gate missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = requiredGatesWithout(CandidatePackagePublicationExecutorImplementationGateOwnerApproval)
			},
			wantErr: `required implementation gate "owner_approval_required" is missing`,
		},
		{
			name: "security review gate missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = requiredGatesWithout(CandidatePackagePublicationExecutorImplementationGateSecurityReview)
			},
			wantErr: `required implementation gate "security_review_required" is missing`,
		},
		{
			name: "provider credential proof gate missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = requiredGatesWithout(CandidatePackagePublicationExecutorImplementationGateProviderCredentialProof)
			},
			wantErr: `required implementation gate "provider_credential_proof_required" is missing`,
		},
		{
			name: "rollback drill gate missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RequiredGateRefs = requiredGatesWithout(CandidatePackagePublicationExecutorImplementationGateRollbackDrill)
			},
			wantErr: `required implementation gate "rollback_drill_required" is missing`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			design, evidence := candidatePackagePublicationExecutorImplementationReadinessInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorImplementationReadinessContract(design, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorImplementationReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorImplementationReadinessContractRejectsUnsafeImplementationAndProductClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorImplementationReadinessEvidence)
		wantErr string
	}{
		{
			name: "red ceremony opened",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.RedCeremonyOpened = true
			},
			wantErr: "open red ceremony",
		},
		{
			name: "code surface touched",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.CodeSurfaceTouched = true
			},
			wantErr: "touch code surface",
		},
		{
			name: "implementation ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ImplementationReady = true
			},
			wantErr: "implementation ready",
		},
		{
			name: "executor implemented",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ExecutorImplemented = true
			},
			wantErr: "implement executor",
		},
		{
			name: "executor allowed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ExecutorAllowed = true
			},
			wantErr: "allow executor",
		},
		{
			name: "actual package published",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ActualPackagePublished = true
			},
			wantErr: "actual package publication",
		},
		{
			name: "direct publish ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.DirectPublishReady = true
			},
			wantErr: "direct-publish ready",
		},
		{
			name: "AppAdoption claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ClaimsAppAdoption = true
			},
			wantErr: "AppAdoption",
		},
		{
			name: "deployed route touched",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.TouchesDeployedRoute = true
			},
			wantErr: "deployed route",
		},
		{
			name: "auth session claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ClaimsAuthSession = true
			},
			wantErr: "auth session",
		},
		{
			name: "staging claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ClaimsStaging = true
			},
			wantErr: "staging",
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ClaimsVMLifecycle = true
			},
			wantErr: "VM lifecycle",
		},
		{
			name: "run acceptance claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ClaimsRunAcceptance = true
			},
			wantErr: "run acceptance",
		},
		{
			name: "activation ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.ActivationReady = true
			},
			wantErr: "activation ready",
		},
		{
			name: "promotion level claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
				evidence.PromotionLevelClaimed = true
			},
			wantErr: "promotion level",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			design, evidence := candidatePackagePublicationExecutorImplementationReadinessInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorImplementationReadinessContract(design, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorImplementationReadinessContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorReadinessReviewContractRecordsChecklistWithoutAuthorization(t *testing.T) {
	readiness, evidence := candidatePackagePublicationExecutorReadinessReviewInputs(t)

	contract, err := BuildCandidatePackagePublicationExecutorReadinessReviewContract(readiness, evidence)
	if err != nil {
		t.Fatalf("build candidate package publication executor readiness review contract: %v", err)
	}

	if contract.Kind != CandidatePackagePublicationExecutorReadinessReviewContractKind {
		t.Fatalf("kind = %q, want %q", contract.Kind, CandidatePackagePublicationExecutorReadinessReviewContractKind)
	}
	if contract.CandidatePackageID != readiness.CandidatePackageID || contract.CandidatePackageManifestSHA256 != readiness.CandidatePackageManifestSHA256 {
		t.Fatalf("package refs = id:%q hash:%q, want id:%q hash:%q", contract.CandidatePackageID, contract.CandidatePackageManifestSHA256, readiness.CandidatePackageID, readiness.CandidatePackageManifestSHA256)
	}
	if contract.Version != readiness.Version {
		t.Fatalf("version = %#v, want %#v", contract.Version, readiness.Version)
	}
	if contract.UsesLocalAcceptanceID != readiness.UsesLocalAcceptanceID {
		t.Fatalf("local acceptance id = %q, want %q", contract.UsesLocalAcceptanceID, readiness.UsesLocalAcceptanceID)
	}
	if contract.VerifierEvidenceRef != strings.TrimSpace(readiness.VerifierEvidenceRef) {
		t.Fatalf("verifier evidence ref = %q, want trimmed %q", contract.VerifierEvidenceRef, strings.TrimSpace(readiness.VerifierEvidenceRef))
	}
	if contract.SourceDeltaRef != strings.TrimSpace(readiness.SourceDeltaRef) {
		t.Fatalf("source delta ref = %q, want trimmed %q", contract.SourceDeltaRef, strings.TrimSpace(readiness.SourceDeltaRef))
	}
	if contract.PayloadManifestRef != strings.TrimSpace(readiness.PayloadManifestRef) {
		t.Fatalf("payload manifest ref = %q, want trimmed %q", contract.PayloadManifestRef, strings.TrimSpace(readiness.PayloadManifestRef))
	}
	if contract.ReadinessReviewBoundary != CandidatePackagePublicationExecutorReadinessReviewBoundary {
		t.Fatalf("readiness review boundary = %q, want %q", contract.ReadinessReviewBoundary, CandidatePackagePublicationExecutorReadinessReviewBoundary)
	}
	if contract.OwnerAuthorizationRef != strings.TrimSpace(readiness.OwnerAuthorizationRef) {
		t.Fatalf("owner authorization ref = %q, want trimmed %q", contract.OwnerAuthorizationRef, strings.TrimSpace(readiness.OwnerAuthorizationRef))
	}
	if contract.ReviewerAuthorizationRef != strings.TrimSpace(readiness.ReviewerAuthorizationRef) {
		t.Fatalf("reviewer authorization ref = %q, want trimmed %q", contract.ReviewerAuthorizationRef, strings.TrimSpace(readiness.ReviewerAuthorizationRef))
	}
	if contract.ExecutorDesignSpecRef != strings.TrimSpace(readiness.ExecutorDesignSpecRef) {
		t.Fatalf("executor design spec ref = %q, want trimmed %q", contract.ExecutorDesignSpecRef, strings.TrimSpace(readiness.ExecutorDesignSpecRef))
	}
	if contract.RedCeremonyPlanRef != strings.TrimSpace(readiness.RedCeremonyPlanRef) {
		t.Fatalf("red ceremony plan ref = %q, want trimmed %q", contract.RedCeremonyPlanRef, strings.TrimSpace(readiness.RedCeremonyPlanRef))
	}
	if !reflect.DeepEqual(contract.RequiredGateRefs, canonicalStrings(readiness.RequiredGateRefs)) {
		t.Fatalf("required gate refs = %#v, want canonical %#v", contract.RequiredGateRefs, canonicalStrings(readiness.RequiredGateRefs))
	}
	if !reflect.DeepEqual(contract.EvidenceGateRefs, canonicalStrings(readiness.EvidenceGateRefs)) {
		t.Fatalf("evidence gate refs = %#v, want canonical %#v", contract.EvidenceGateRefs, canonicalStrings(readiness.EvidenceGateRefs))
	}
	if contract.RollbackDrillRef != strings.TrimSpace(readiness.RollbackDrillRef) {
		t.Fatalf("rollback drill ref = %q, want trimmed %q", contract.RollbackDrillRef, strings.TrimSpace(readiness.RollbackDrillRef))
	}
	if contract.ReviewReportRef != strings.TrimSpace(evidence.ReviewReportRef) {
		t.Fatalf("review report ref = %q, want trimmed %q", contract.ReviewReportRef, strings.TrimSpace(evidence.ReviewReportRef))
	}
	if !reflect.DeepEqual(contract.ChecklistItemRefs, canonicalStrings(evidence.ChecklistItemRefs)) {
		t.Fatalf("checklist item refs = %#v, want canonical %#v", contract.ChecklistItemRefs, canonicalStrings(evidence.ChecklistItemRefs))
	}
	if !reflect.DeepEqual(contract.ReviewerFindingRefs, canonicalStrings(evidence.ReviewerFindingRefs)) {
		t.Fatalf("reviewer finding refs = %#v, want canonical %#v", contract.ReviewerFindingRefs, canonicalStrings(evidence.ReviewerFindingRefs))
	}
	if !reflect.DeepEqual(contract.OpenQuestionRefs, canonicalStrings(evidence.OpenQuestionRefs)) {
		t.Fatalf("open question refs = %#v, want canonical %#v", contract.OpenQuestionRefs, canonicalStrings(evidence.OpenQuestionRefs))
	}
	assertPublicationExecutorReadinessReviewContractRefusesAuthorization(t, contract)
	for _, want := range productActivationProtectedPrerequisites() {
		assertStringSliceContains(t, contract.BlockedPrerequisites, want)
	}
}

func TestBuildCandidatePackagePublicationExecutorReadinessReviewContractRejectsMalformedReadinessPacketAndMismatchedEvidence(t *testing.T) {
	foreignVersion := ComputerVersion{CodeRef: "git:foreign-publication-executor-readiness-review", ArtifactProgramRef: "tape:org/foreign-publication-executor-readiness-review@2026-07-04"}
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorImplementationReadinessContract, *CandidatePackagePublicationExecutorReadinessReviewEvidence)
		wantErr string
	}{
		{
			name: "readiness kind mismatch",
			mutate: func(readiness *CandidatePackagePublicationExecutorImplementationReadinessContract, _ *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				readiness.Kind = "candidate_package_publication_executor_readiness_review"
			},
			wantErr: "readiness kind",
		},
		{
			name: "readiness boundary mismatch",
			mutate: func(readiness *CandidatePackagePublicationExecutorImplementationReadinessContract, _ *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				readiness.ImplementationReadinessBoundary = CandidatePackagePublicationExecutorReadinessReviewBoundary
			},
			wantErr: "readiness boundary",
		},
		{
			name: "readiness status not blocked",
			mutate: func(readiness *CandidatePackagePublicationExecutorImplementationReadinessContract, _ *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				readiness.ImplementationReadinessStatus = "implementation_ready"
			},
			wantErr: "readiness status",
		},
		{
			name: "readiness already opened red ceremony",
			mutate: func(readiness *CandidatePackagePublicationExecutorImplementationReadinessContract, _ *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				readiness.RedCeremonyOpened = true
			},
			wantErr: "readiness cannot have opened red ceremony",
		},
		{
			name: "evidence package id mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.CandidatePackageID = "foreign-candidate-package"
			},
			wantErr: "evidence package id",
		},
		{
			name: "evidence package hash mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.CandidatePackageManifestSHA256 = "sha256:" + strings.Repeat("7", 64)
			},
			wantErr: "evidence package hash does not match",
		},
		{
			name: "evidence version mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.Version = foreignVersion
			},
			wantErr: "evidence version does not match",
		},
		{
			name: "evidence acceptance id mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.UsesLocalAcceptanceID = "foreign-local-acceptance"
			},
			wantErr: "evidence acceptance id does not match",
		},
		{
			name: "evidence verifier evidence ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.VerifierEvidenceRef = "publication-proof:foreign"
			},
			wantErr: "evidence verifier ref",
		},
		{
			name: "evidence source delta ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.SourceDeltaRef = "source-delta:foreign"
			},
			wantErr: "evidence source delta ref",
		},
		{
			name: "evidence payload manifest ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.PayloadManifestRef = "payload-manifest:foreign"
			},
			wantErr: "evidence payload manifest ref",
		},
		{
			name: "evidence owner authorization ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.OwnerAuthorizationRef = "owner-auth:foreign"
			},
			wantErr: "evidence owner authorization ref",
		},
		{
			name: "evidence reviewer authorization ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ReviewerAuthorizationRef = "reviewer-auth:foreign"
			},
			wantErr: "evidence reviewer authorization ref",
		},
		{
			name: "evidence executor design spec ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ExecutorDesignSpecRef = "executor-design-spec:foreign"
			},
			wantErr: "evidence executor design spec ref",
		},
		{
			name: "evidence red ceremony plan ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.RedCeremonyPlanRef = "red-ceremony-plan:foreign"
			},
			wantErr: "evidence red ceremony plan ref",
		},
		{
			name: "evidence required gate refs mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.RequiredGateRefs = []string{CandidatePackagePublicationExecutorImplementationGateRedCeremony}
			},
			wantErr: "evidence required gate refs",
		},
		{
			name: "evidence gate refs mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.EvidenceGateRefs = []string{"implementation-readiness:foreign"}
			},
			wantErr: "evidence gate refs",
		},
		{
			name: "evidence rollback drill ref mismatch",
			mutate: func(_ *CandidatePackagePublicationExecutorImplementationReadinessContract, evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.RollbackDrillRef = "rollback-drill:foreign"
			},
			wantErr: "evidence rollback drill ref",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := candidatePackagePublicationExecutorReadinessReviewInputs(t)
			tc.mutate(&readiness, &evidence)

			contract, err := BuildCandidatePackagePublicationExecutorReadinessReviewContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorReadinessReviewContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorReadinessReviewContractRejectsMissingOrUnsupportedChecklistEvidence(t *testing.T) {
	checklistWithout := func(missing string) []string {
		var items []string
		for _, item := range publicationExecutorReadinessReviewRequiredChecklistItems() {
			if item != missing {
				items = append(items, item)
			}
		}
		return items
	}

	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorReadinessReviewEvidence)
		wantErr string
	}{
		{
			name: "review report ref missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ReviewReportRef = " "
			},
			wantErr: "review report ref is required",
		},
		{
			name: "reviewer finding refs missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ReviewerFindingRefs = []string{" ", ""}
			},
			wantErr: "reviewer finding refs are required",
		},
		{
			name: "open question refs missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.OpenQuestionRefs = []string{" ", ""}
			},
			wantErr: "open question refs are required",
		},
		{
			name: "checklist item refs missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = []string{" ", ""}
			},
			wantErr: "checklist item refs are required",
		},
		{
			name: "unsupported checklist item",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = append(evidence.ChecklistItemRefs, "executor_code_review")
			},
			wantErr: "unsupported checklist item",
		},
		{
			name: "red ceremony scope checklist item missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = checklistWithout(CandidatePackagePublicationExecutorReadinessReviewItemRedCeremonyScope)
			},
			wantErr: `checklist item "red_ceremony_scope_review" is missing`,
		},
		{
			name: "owner approval path checklist item missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = checklistWithout(CandidatePackagePublicationExecutorReadinessReviewItemOwnerApprovalPath)
			},
			wantErr: `checklist item "owner_approval_path_review" is missing`,
		},
		{
			name: "security scope checklist item missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = checklistWithout(CandidatePackagePublicationExecutorReadinessReviewItemSecurityScope)
			},
			wantErr: `checklist item "security_review_scope_review" is missing`,
		},
		{
			name: "provider credential boundary checklist item missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = checklistWithout(CandidatePackagePublicationExecutorReadinessReviewItemProviderCredentialBoundary)
			},
			wantErr: `checklist item "provider_credential_boundary_review" is missing`,
		},
		{
			name: "rollback drill checklist item missing",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ChecklistItemRefs = checklistWithout(CandidatePackagePublicationExecutorReadinessReviewItemRollbackDrill)
			},
			wantErr: `checklist item "rollback_drill_review" is missing`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := candidatePackagePublicationExecutorReadinessReviewInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorReadinessReviewContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorReadinessReviewContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func TestBuildCandidatePackagePublicationExecutorReadinessReviewContractRejectsUnsafeReviewMutationClaims(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*CandidatePackagePublicationExecutorReadinessReviewEvidence)
		wantErr string
	}{
		{
			name: "red ceremony opened",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.RedCeremonyOpened = true
			},
			wantErr: "open red ceremony",
		},
		{
			name: "red ceremony approved",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.RedCeremonyApproved = true
			},
			wantErr: "approve red ceremony",
		},
		{
			name: "implementation authorized",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ImplementationAuthorized = true
			},
			wantErr: "authorize implementation",
		},
		{
			name: "code surface touched",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.CodeSurfaceTouched = true
			},
			wantErr: "touch code surface",
		},
		{
			name: "implementation ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ImplementationReady = true
			},
			wantErr: "implementation ready",
		},
		{
			name: "executor implemented",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ExecutorImplemented = true
			},
			wantErr: "implement executor",
		},
		{
			name: "executor allowed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ExecutorAllowed = true
			},
			wantErr: "allow executor",
		},
		{
			name: "actual package published",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ActualPackagePublished = true
			},
			wantErr: "actual package publication",
		},
		{
			name: "direct publish ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.DirectPublishReady = true
			},
			wantErr: "direct-publish ready",
		},
		{
			name: "AppAdoption claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ClaimsAppAdoption = true
			},
			wantErr: "AppAdoption",
		},
		{
			name: "deployed route touched",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.TouchesDeployedRoute = true
			},
			wantErr: "deployed route",
		},
		{
			name: "auth session claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ClaimsAuthSession = true
			},
			wantErr: "auth session",
		},
		{
			name: "staging claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ClaimsStaging = true
			},
			wantErr: "staging",
		},
		{
			name: "VM lifecycle claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ClaimsVMLifecycle = true
			},
			wantErr: "VM lifecycle",
		},
		{
			name: "run acceptance claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ClaimsRunAcceptance = true
			},
			wantErr: "run acceptance",
		},
		{
			name: "activation ready",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.ActivationReady = true
			},
			wantErr: "activation ready",
		},
		{
			name: "promotion level claimed",
			mutate: func(evidence *CandidatePackagePublicationExecutorReadinessReviewEvidence) {
				evidence.PromotionLevelClaimed = true
			},
			wantErr: "promotion level",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readiness, evidence := candidatePackagePublicationExecutorReadinessReviewInputs(t)
			tc.mutate(&evidence)

			contract, err := BuildCandidatePackagePublicationExecutorReadinessReviewContract(readiness, evidence)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("BuildCandidatePackagePublicationExecutorReadinessReviewContract() = contract %#v error %v, want error containing %q", contract, err, tc.wantErr)
			}
		})
	}
}

func candidatePackagePublicationExecutorReadinessReviewInputs(t *testing.T) (CandidatePackagePublicationExecutorImplementationReadinessContract, CandidatePackagePublicationExecutorReadinessReviewEvidence) {
	t.Helper()

	design, readinessEvidence := candidatePackagePublicationExecutorImplementationReadinessInputs(t)
	readiness, err := BuildCandidatePackagePublicationExecutorImplementationReadinessContract(design, readinessEvidence)
	if err != nil {
		t.Fatalf("build valid candidate package publication executor implementation readiness contract: %v", err)
	}
	return readiness, CandidatePackagePublicationExecutorReadinessReviewEvidence{
		CandidatePackageID:             readiness.CandidatePackageID,
		CandidatePackageManifestSHA256: readiness.CandidatePackageManifestSHA256,
		Version:                        readiness.Version,
		UsesLocalAcceptanceID:          readiness.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            readiness.VerifierEvidenceRef,
		SourceDeltaRef:                 readiness.SourceDeltaRef,
		PayloadManifestRef:             readiness.PayloadManifestRef,
		OwnerAuthorizationRef:          " " + readiness.OwnerAuthorizationRef + " ",
		ReviewerAuthorizationRef:       " " + readiness.ReviewerAuthorizationRef + " ",
		ExecutorDesignSpecRef:          " " + readiness.ExecutorDesignSpecRef + " ",
		RedCeremonyPlanRef:             " " + readiness.RedCeremonyPlanRef + " ",
		RequiredGateRefs:               append(append([]string{}, readiness.RequiredGateRefs...), " "),
		EvidenceGateRefs:               append(append([]string{}, readiness.EvidenceGateRefs...), " "),
		RollbackDrillRef:               " " + readiness.RollbackDrillRef + " ",
		ReviewReportRef:                " readiness-review-report:candidate-package-publication-executor ",
		ChecklistItemRefs: []string{
			CandidatePackagePublicationExecutorReadinessReviewItemRollbackDrill,
			CandidatePackagePublicationExecutorReadinessReviewItemRedCeremonyScope,
			CandidatePackagePublicationExecutorReadinessReviewItemOwnerApprovalPath,
			CandidatePackagePublicationExecutorReadinessReviewItemSecurityScope,
			CandidatePackagePublicationExecutorReadinessReviewItemProviderCredentialBoundary,
			CandidatePackagePublicationExecutorReadinessReviewItemRollbackDrill,
			" ",
		},
		ReviewerFindingRefs: []string{
			"readiness-review-finding:red-ceremony-still-blocked",
			" readiness-review-finding:owner-approval-requires-red-authorization ",
			"readiness-review-finding:provider-credential-boundary-unopened",
			"readiness-review-finding:red-ceremony-still-blocked",
			" ",
		},
		OpenQuestionRefs: []string{
			"readiness-review-question:provider-credential-escrow",
			" readiness-review-question:rollback-drill-scheduling ",
			"readiness-review-question:provider-credential-escrow",
			" ",
		},
	}
}

func assertPublicationExecutorReadinessReviewContractRefusesAuthorization(t *testing.T, contract CandidatePackagePublicationExecutorReadinessReviewContract) {
	t.Helper()
	if contract.ReadinessReviewStatus != CandidatePackagePublicationExecutorReadinessReviewStatusChecklistRecorded {
		t.Fatalf("readiness review status = %q, want %q", contract.ReadinessReviewStatus, CandidatePackagePublicationExecutorReadinessReviewStatusChecklistRecorded)
	}
	if contract.RedCeremonyOpened {
		t.Fatalf("red ceremony opened = true, want false for read-only readiness review")
	}
	if contract.RedCeremonyApproved {
		t.Fatalf("red ceremony approved = true, want false for read-only readiness review")
	}
	if contract.ImplementationAuthorized {
		t.Fatalf("implementation authorized = true, want false until red authorization exists")
	}
	if contract.CodeSurfaceTouched {
		t.Fatalf("code surface touched = true, want false for read-only readiness review")
	}
	if contract.ImplementationReady {
		t.Fatalf("implementation ready = true, want false until protected prerequisites open")
	}
	if contract.ExecutorImplemented {
		t.Fatalf("executor implemented = true, want false for readiness review")
	}
	if contract.ExecutorAllowed {
		t.Fatalf("executor allowed = true, want false before separate executor implementation authorization")
	}
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for readiness review")
	}
	if contract.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false before red authorization")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure readiness review evidence")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication executor readiness review")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication executor readiness review")
	}
}

func candidatePackagePublicationExecutorImplementationReadinessInputs(t *testing.T) (CandidatePackagePublicationExecutorDesignSpecContract, CandidatePackagePublicationExecutorImplementationReadinessEvidence) {
	t.Helper()

	gate, designEvidence := candidatePackagePublicationExecutorDesignSpecInputs(t)
	design, err := BuildCandidatePackagePublicationExecutorDesignSpecContract(gate, designEvidence)
	if err != nil {
		t.Fatalf("build valid candidate package publication executor design spec contract: %v", err)
	}
	return design, CandidatePackagePublicationExecutorImplementationReadinessEvidence{
		CandidatePackageID:             design.CandidatePackageID,
		CandidatePackageManifestSHA256: design.CandidatePackageManifestSHA256,
		Version:                        design.Version,
		UsesLocalAcceptanceID:          design.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            design.VerifierEvidenceRef,
		SourceDeltaRef:                 design.SourceDeltaRef,
		PayloadManifestRef:             design.PayloadManifestRef,
		OwnerAuthorizationRef:          " " + design.OwnerAuthorizationRef + " ",
		ReviewerAuthorizationRef:       " " + design.ReviewerAuthorizationRef + " ",
		ExecutorDesignSpecRef:          " " + design.ExecutorDesignSpecRef + " ",
		RequiredRedSurfaces:            append(append([]string{}, design.RequiredRedSurfaces...), " "),
		RequiredEvidenceRefs:           append(append([]string{}, design.RequiredEvidenceRefs...), " "),
		RollbackPlanRef:                " " + design.RollbackPlanRef + " ",
		RedCeremonyPlanRef:             " red-ceremony-plan:candidate-package-publication-executor-implementation ",
		RequiredGateRefs: []string{
			CandidatePackagePublicationExecutorImplementationGateRollbackDrill,
			CandidatePackagePublicationExecutorImplementationGateRedCeremony,
			CandidatePackagePublicationExecutorImplementationGateOwnerApproval,
			CandidatePackagePublicationExecutorImplementationGateSecurityReview,
			CandidatePackagePublicationExecutorImplementationGateProviderCredentialProof,
			CandidatePackagePublicationExecutorImplementationGateRollbackDrill,
			" ",
		},
		EvidenceGateRefs: []string{
			"implementation-readiness:red-ceremony-plan-reviewed",
			" implementation-readiness:owner-approval-ticket ",
			"implementation-readiness:security-review-scheduled",
			"implementation-readiness:provider-credential-proof-requested",
			"implementation-readiness:rollback-drill-scheduled",
			"implementation-readiness:red-ceremony-plan-reviewed",
			" ",
		},
		RollbackDrillRef: " rollback-drill:candidate-package-publication-executor-implementation ",
	}
}

func assertPublicationExecutorImplementationReadinessContractRefusesExecution(t *testing.T, contract CandidatePackagePublicationExecutorImplementationReadinessContract) {
	t.Helper()
	if contract.ImplementationReadinessStatus != CandidatePackagePublicationExecutorImplementationStatusBlocked {
		t.Fatalf("implementation readiness status = %q, want %q", contract.ImplementationReadinessStatus, CandidatePackagePublicationExecutorImplementationStatusBlocked)
	}
	if contract.RedCeremonyOpened {
		t.Fatalf("red ceremony opened = true, want false until the red ceremony actually opens")
	}
	if contract.CodeSurfaceTouched {
		t.Fatalf("code surface touched = true, want false for pure implementation-readiness evidence")
	}
	if contract.ImplementationReady {
		t.Fatalf("implementation ready = true, want false until protected prerequisites open")
	}
	if contract.ExecutorImplemented {
		t.Fatalf("executor implemented = true, want false for pure implementation-readiness evidence")
	}
	if contract.ExecutorAllowed {
		t.Fatalf("executor allowed = true, want false before a separate executor implementation exists")
	}
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for pure implementation-readiness evidence")
	}
	if contract.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false before protected gates open")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure implementation-readiness evidence")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication executor implementation readiness")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication executor implementation readiness")
	}
}

func candidatePackagePublicationExecutorDesignSpecInputs(t *testing.T) (CandidatePackagePublicationExecutorReviewGateContract, CandidatePackagePublicationExecutorDesignSpecEvidence) {
	t.Helper()

	preflight, reviewGateEvidence := candidatePackagePublicationExecutorReviewGateInputs(t)
	gate, err := BuildCandidatePackagePublicationExecutorReviewGateContract(preflight, reviewGateEvidence)
	if err != nil {
		t.Fatalf("build valid candidate package publication executor review gate contract: %v", err)
	}
	return gate, CandidatePackagePublicationExecutorDesignSpecEvidence{
		CandidatePackageID:             gate.CandidatePackageID,
		CandidatePackageManifestSHA256: gate.CandidatePackageManifestSHA256,
		Version:                        gate.Version,
		UsesLocalAcceptanceID:          gate.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            gate.VerifierEvidenceRef,
		SourceDeltaRef:                 gate.SourceDeltaRef,
		PayloadManifestRef:             gate.PayloadManifestRef,
		OwnerAuthorizationRef:          " " + gate.OwnerAuthorizationRef + " ",
		ReviewerAuthorizationRef:       " " + gate.ReviewerAuthorizationRef + " ",
		ExecutorDesignSpecRef:          " executor-design-spec:candidate-package-publication-executor-red-path ",
		RequiredRedSurfaces: []string{
			CandidatePackagePublicationExecutorRedSurfaceRollbackPath,
			CandidatePackagePublicationExecutorRedSurfacePackageArtifact,
			CandidatePackagePublicationExecutorRedSurfaceProviderCredentials,
			CandidatePackagePublicationExecutorRedSurfacePublicationLedger,
			CandidatePackagePublicationExecutorRedSurfaceRollbackPath,
			" ",
		},
		RequiredEvidenceRefs: []string{
			"executor-design:threat-model",
			" executor-design:credential-scope ",
			"executor-design:publication-ledger-audit",
			"executor-design:rollback-drill",
			"executor-design:threat-model",
			" ",
		},
		RollbackPlanRef: " rollback-plan:candidate-package-publication-executor-red-path ",
	}
}

func assertPublicationExecutorDesignSpecContractRefusesExecution(t *testing.T, contract CandidatePackagePublicationExecutorDesignSpecContract) {
	t.Helper()
	if !contract.ExecutorDesignSpecReady {
		t.Fatalf("executor design spec ready = false, want true for owner/reviewer-authorized design spec")
	}
	if contract.ExecutorImplemented {
		t.Fatalf("executor implemented = true, want false for pure publication executor design spec")
	}
	if contract.ExecutorAllowed {
		t.Fatalf("executor allowed = true, want false for pure publication executor design spec")
	}
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for pure publication executor design spec")
	}
	if contract.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false until a separate package publication executor exists")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure publication executor design spec")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication executor design spec")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication executor design spec")
	}
}

func candidatePackagePublicationExecutorReviewGateInputs(t *testing.T) (CandidatePackagePublicationPreflightContract, CandidatePackagePublicationExecutorReviewGateEvidence) {
	t.Helper()

	payload, preflightEvidence := candidatePackagePublicationPreflightInputs(t)
	preflight, err := BuildCandidatePackagePublicationPreflightContract(payload, preflightEvidence)
	if err != nil {
		t.Fatalf("build valid candidate package publication preflight contract: %v", err)
	}
	return preflight, CandidatePackagePublicationExecutorReviewGateEvidence{
		CandidatePackageID:             preflight.CandidatePackageID,
		CandidatePackageManifestSHA256: preflight.CandidatePackageManifestSHA256,
		Version:                        preflight.Version,
		UsesLocalAcceptanceID:          preflight.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            preflight.VerifierEvidenceRef,
		SourceDeltaRef:                 preflight.SourceDeltaRef,
		PayloadManifestRef:             preflight.PayloadManifestRef,
		PreflightCheckRefs:             preflight.PreflightCheckRefs,
		VerifierContractRefs:           preflight.VerifierContractRefs,
		OwnerAuthorizationRef:          " owner-auth:package-publication-executor-design-review ",
		ReviewerAuthorizationRef:       " reviewer-auth:package-publication-executor-design-review ",
	}
}

func assertPublicationExecutorReviewGateContractRefusesExecution(t *testing.T, contract CandidatePackagePublicationExecutorReviewGateContract) {
	t.Helper()
	if !contract.ExecutorDesignReviewReady {
		t.Fatalf("executor design review ready = false, want true for owner/reviewer-authorized design review")
	}
	if contract.ExecutorAllowed {
		t.Fatalf("executor allowed = true, want false for pure publication executor review gate")
	}
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for pure publication executor review gate")
	}
	if contract.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false until a separate package publication executor exists")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure publication executor review gate")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication executor review gate")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication executor review gate")
	}
}

func candidatePackagePublicationPreflightInputs(t *testing.T) (CandidatePackagePublicationPayloadContract, CandidatePackagePublicationPreflightEvidence) {
	t.Helper()

	proof, payloadEvidence := candidatePackagePublicationPayloadInputs(t)
	payload, err := BuildCandidatePackagePublicationPayloadContract(proof, payloadEvidence)
	if err != nil {
		t.Fatalf("build valid candidate package publication payload contract: %v", err)
	}
	return payload, CandidatePackagePublicationPreflightEvidence{
		CandidatePackageID:             payload.CandidatePackageID,
		CandidatePackageManifestSHA256: payload.CandidatePackageManifestSHA256,
		Version:                        payload.Version,
		UsesLocalAcceptanceID:          payload.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            payload.VerifierEvidenceRef,
		SourceDeltaRef:                 payload.SourceDeltaRef,
		PayloadManifestRef:             payload.PayloadManifestRef,
		PreflightCheckRefs: []string{
			"preflight:source-delta-readonly",
			" preflight:payload-manifest-readonly ",
			"preflight:manifest-sha256-matches-payload",
			"preflight:source-delta-readonly",
			" ",
		},
		VerifierContractRefs: []string{
			"verifier:publish-token-absence",
			" verifier:payload-review-committee ",
			"verifier:publish-token-absence",
		},
	}
}

func assertPublicationPreflightContractRefusesExecution(t *testing.T, contract CandidatePackagePublicationPreflightContract) {
	t.Helper()
	if contract.ExecutorAllowed {
		t.Fatalf("executor allowed = true, want false for pure publication preflight")
	}
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for pure publication preflight")
	}
	if contract.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false until a separate package publication executor exists")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure publication preflight")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication preflight")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication preflight")
	}
}

func candidatePackagePublicationPayloadInputs(t *testing.T) (CandidatePackagePublicationProofContract, CandidatePackagePublicationPayloadEvidence) {
	t.Helper()

	verifier, proofEvidence := candidatePackagePublicationProofInputs(t)
	proof, err := BuildCandidatePackagePublicationProofContract(verifier, proofEvidence)
	if err != nil {
		t.Fatalf("build valid candidate package publication proof contract: %v", err)
	}
	return proof, CandidatePackagePublicationPayloadEvidence{
		CandidatePackageID:             proof.CandidatePackageID,
		CandidatePackageManifestSHA256: proof.CandidatePackageManifestSHA256,
		Version:                        proof.Version,
		UsesLocalAcceptanceID:          proof.UsesLocalAcceptanceID,
		VerifierEvidenceRef:            proof.VerifierEvidenceRef,
		SourceDeltaRef:                 "source-delta:candidate-package-local-acceptance-adoption-1",
		PayloadManifestRef:             "payload-manifest:candidate-package-local-acceptance-adoption-1",
	}
}

func assertPublicationPayloadContractRefusesActivation(t *testing.T, contract CandidatePackagePublicationPayloadContract) {
	t.Helper()
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for pure publication payload boundary")
	}
	if contract.DirectPublishReady {
		t.Fatalf("direct publish ready = true, want false until a separate package publication executor exists")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure publication payload boundary")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication payload boundary")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication payload boundary")
	}
}

func candidatePackagePublicationProofInputs(t *testing.T) (CandidatePackageProductActivationVerifierContract, CandidatePackagePublicationProofEvidence) {
	t.Helper()

	durable, evidence := candidatePackageProductActivationVerifierInputs(t)
	evidence.Prerequisites = []CandidatePackageProductActivationPrerequisiteEvidence{
		{
			Prerequisite: CandidatePackageProductActivationPrerequisitePackagePublication,
			Status:       CandidatePackageProductActivationEvidenceStatusCandidate,
			EvidenceRef:  "publication-proof:candidate-package-local-acceptance-adoption-1",
		},
	}
	verifier, err := BuildCandidatePackageProductActivationVerifierContract(durable, evidence)
	if err != nil {
		t.Fatalf("build valid candidate package product activation verifier contract: %v", err)
	}
	return verifier, CandidatePackagePublicationProofEvidence{
		CandidatePackageID:             verifier.CandidatePackageID,
		CandidatePackageManifestSHA256: verifier.CandidatePackageManifestSHA256,
		Version:                        verifier.Version,
		UsesLocalAcceptanceID:          verifier.UsesLocalAcceptanceID,
		EvidenceRef:                    verifier.FirstBindableEvidenceRef,
	}
}

func assertPublicationProofContractRefusesActivation(t *testing.T, contract CandidatePackagePublicationProofContract) {
	t.Helper()
	if contract.ActualPackagePublished {
		t.Fatalf("actual package published = true, want false for pure publication proof")
	}
	if !contract.NoMutation {
		t.Fatalf("no mutation = false, want true for pure publication proof")
	}
	if contract.ActivationReady {
		t.Fatalf("activation ready = true, want false for package-publication proof")
	}
	if contract.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for package-publication proof")
	}
}

func candidatePackageProductActivationVerifierInputs(t *testing.T) (CandidatePackageDurableActivationContract, CandidatePackageProductActivationEvidence) {
	t.Helper()

	pkg, acceptance, decision := candidatePackageDurableActivationInputs(t)
	durable, err := BuildCandidatePackageDurableActivationContract(pkg, acceptance, decision)
	if err != nil {
		t.Fatalf("build valid candidate package durable activation contract: %v", err)
	}
	return durable, CandidatePackageProductActivationEvidence{
		CandidatePackageID:             durable.CandidatePackageID,
		CandidatePackageManifestSHA256: durable.CandidatePackageManifestSHA256,
		Version:                        durable.Version,
		UsesLocalAcceptanceID:          durable.UsesLocalAcceptanceID,
	}
}

func assertProductActivationVerifierRefusesActivation(t *testing.T, verifier CandidatePackageProductActivationVerifierContract) {
	t.Helper()
	if verifier.ActivationReady {
		t.Fatalf("activation ready = true, want false until every protected prerequisite has explicit passed evidence")
	}
	if !verifier.NoMutation {
		t.Fatalf("no mutation = false, want true for pure product activation verifier")
	}
	if verifier.PromotionLevelClaimed {
		t.Fatalf("promotion level claimed = true, want false for pure verifier")
	}
}

func candidatePackageDurableActivationInputs(t *testing.T) (CandidateComputerPackageManifest, CandidatePackageProductPathAcceptanceContract, CandidatePackageOwnerActivationDecision) {
	t.Helper()

	pkg, bridge := candidatePackageAcceptanceInputs(t)
	acceptance, err := BuildCandidatePackageProductPathAcceptanceContract(pkg, bridge)
	if err != nil {
		t.Fatalf("build valid candidate package product-path acceptance contract: %v", err)
	}
	decision := CandidatePackageOwnerActivationDecision{
		Kind:                           CandidatePackageOwnerActivationDecisionKind,
		State:                          CandidatePackageOwnerDecisionPreparableState,
		OwnerControlled:                true,
		RequiresAuthenticatedOwner:     true,
		PreparedAction:                 CandidatePackagePrepareActivationDecisionAction,
		NoMutation:                     true,
		UsesLocalAcceptanceID:          "candidate-package-local-acceptance-adoption-1",
		CandidatePackageID:             pkg.ID,
		CandidatePackageManifestSHA256: pkg.PackageManifestSHA256,
		Version:                        pkg.Version,
		NextBoundary:                   CandidatePackagePromotionRequiresProductActivationContractBoundary,
		ActivationReady:                false,
		PromotionLevelClaimed:          false,
		BlockedRoutes: []string{
			"POST /api/adoptions/{adoption_id}/verify",
			"POST /api/adoptions/{adoption_id}/approve",
			"POST /api/adoptions/{adoption_id}/promote",
			"POST /api/candidate-package-intakes",
			"POST /api/candidate-package-intakes/{intake_id}/review",
			"POST /api/candidate-package-intakes/{intake_id}/adoption-boundary",
			"POST /api/candidate-package-intakes/{intake_id}/publication-draft",
			"POST /api/candidate-package-intakes/{intake_id}/adoption-review",
			"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}",
			"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch",
			"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/rollback",
			"POST /api/candidate-package-intakes/{intake_id}/adoption-review/{adoption_id}/promotion-switch/roll-forward",
			"POST /api/run-acceptances/synthesize",
			"DELETE /auth/sessions/{session_id}",
			"POST /auth/logout",
			"POST /api/staging/claims",
			"POST /api/vm/lifecycle",
		},
		RequiredContracts: []string{
			"authenticated owner decision contract",
			"package publication contract",
			"AppAdoption mutation contract",
			"deployed route mutation contract",
			"staging identity contract",
			"VM lifecycle contract",
			"run-acceptance contract",
		},
	}
	return pkg, acceptance, decision
}

func assertStringSliceContains(t *testing.T, got []string, want string) {
	t.Helper()
	for _, value := range got {
		if value == want {
			return
		}
	}
	t.Fatalf("slice = %#v, want to contain %q", got, want)
}
