package computerversion

import (
	"fmt"
	"strings"
)

const BaseVerifierResultContractKind = "base_verifier_result_contract"

const BaseVerifierResultBoundary = "verifier_result_without_owner_approval_promotion_publication_or_run_acceptance"

const BaseVerifierResultScope = "verifier_readiness_to_pass_fail_result_only"

const BaseVerifierVerdictPass = "pass"
const BaseVerifierVerdictFail = "fail"

// BaseVerifierResultEvidence records one verifier verdict. It is the verifier
// outcome only; owner approval, promotion, publication, run acceptance, full
// substrate proof, and completion remain outside this boundary.
type BaseVerifierResultEvidence struct {
	VerifierReadinessRef         string `json:"verifier_readiness_ref"`
	VerifierRunRef               string `json:"verifier_run_ref"`
	VerifierResultRef            string `json:"verifier_result_ref"`
	VerifierLogRef               string `json:"verifier_log_ref"`
	Verdict                      string `json:"verdict"`
	FailureReason                string `json:"failure_reason"`
	RollbackPlanRef              string `json:"rollback_plan_ref"`
	NoOwnerApprovalMutation      bool   `json:"no_owner_approval_mutation"`
	NoPromotionMutation          bool   `json:"no_promotion_mutation"`
	NoPackagePublicationMutation bool   `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation      bool   `json:"no_run_acceptance_mutation"`
	NoProductionMutation         bool   `json:"no_production_mutation"`
	VerifierContractSatisfied    bool   `json:"verifier_contract_satisfied"`
	OwnerApproved                bool   `json:"owner_approved"`
	PromotionExecuted            bool   `json:"promotion_executed"`
	PackagePublished             bool   `json:"package_published"`
	RunAcceptanceRecordTouched   bool   `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed         bool   `json:"full_substrate_claimed"`
	CompletionClaimed            bool   `json:"completion_claimed"`
}

// BaseVerifierResultContract records a verifier pass/fail result. A pass is
// verifier evidence only. A fail is explicit blocking evidence. Neither outcome
// performs downstream authority changes.
type BaseVerifierResultContract struct {
	Kind                            string          `json:"kind"`
	Version                         ComputerVersion `json:"version"`
	Boundary                        string          `json:"boundary"`
	Scope                           string          `json:"scope"`
	TypedArtifactProgramRef         string          `json:"typed_artifact_program_ref"`
	VerifierReadinessRef            string          `json:"verifier_readiness_ref"`
	VerifierInputBundleRef          string          `json:"verifier_input_bundle_ref"`
	VerifierContractSpecRef         string          `json:"verifier_contract_spec_ref"`
	EvidenceManifestRef             string          `json:"evidence_manifest_ref"`
	ExpectedVerdictPolicyRef        string          `json:"expected_verdict_policy_ref"`
	VerifierRunRef                  string          `json:"verifier_run_ref"`
	VerifierResultRef               string          `json:"verifier_result_ref"`
	VerifierLogRef                  string          `json:"verifier_log_ref"`
	Verdict                         string          `json:"verdict"`
	FailureReason                   string          `json:"failure_reason"`
	RollbackPlanRef                 string          `json:"rollback_plan_ref"`
	VerifierContractSatisfied       bool            `json:"verifier_contract_satisfied"`
	VerifierContractFailed          bool            `json:"verifier_contract_failed"`
	VerifierFailureBlocksDownstream bool            `json:"verifier_failure_blocks_downstream"`
	OwnerApprovalRequired           bool            `json:"owner_approval_required"`
	PromotionRollbackReviewRequired bool            `json:"promotion_rollback_review_required"`
	PackagePublicationProofRequired bool            `json:"package_publication_proof_required"`
	RunAcceptanceProofRequired      bool            `json:"run_acceptance_proof_required"`
	FullSubstrateProofRequired      bool            `json:"full_substrate_proof_required"`
	OwnerApprovalAllowed            bool            `json:"owner_approval_allowed"`
	PromotionAllowed                bool            `json:"promotion_allowed"`
	PackagePublicationAllowed       bool            `json:"package_publication_allowed"`
	RunAcceptanceSynthesisAllowed   bool            `json:"run_acceptance_synthesis_allowed"`
	NoOwnerApprovalMutation         bool            `json:"no_owner_approval_mutation"`
	NoPromotionMutation             bool            `json:"no_promotion_mutation"`
	NoPackagePublicationMutation    bool            `json:"no_package_publication_mutation"`
	NoRunAcceptanceMutation         bool            `json:"no_run_acceptance_mutation"`
	NoProductionMutation            bool            `json:"no_production_mutation"`
	OwnerApproved                   bool            `json:"owner_approved"`
	PromotionExecuted               bool            `json:"promotion_executed"`
	PackagePublished                bool            `json:"package_published"`
	RunAcceptanceRecordTouched      bool            `json:"run_acceptance_record_touched"`
	FullSubstrateClaimed            bool            `json:"full_substrate_claimed"`
	CompletionClaimed               bool            `json:"completion_claimed"`
}

// BuildBaseVerifierResultContract records a verifier pass/fail outcome. It does
// not approve, promote, publish, synthesize run acceptance, or claim completion.
func BuildBaseVerifierResultContract(readiness BaseVerifierReadinessContract, evidence BaseVerifierResultEvidence) (BaseVerifierResultContract, error) {
	if err := validateBaseVerifierResultReadiness(readiness); err != nil {
		return BaseVerifierResultContract{}, err
	}
	verdict, failureReason, err := validateBaseVerifierResultEvidence(evidence)
	if err != nil {
		return BaseVerifierResultContract{}, err
	}
	passed := verdict == BaseVerifierVerdictPass
	failed := verdict == BaseVerifierVerdictFail

	return BaseVerifierResultContract{
		Kind:                            BaseVerifierResultContractKind,
		Version:                         readiness.Version,
		Boundary:                        BaseVerifierResultBoundary,
		Scope:                           BaseVerifierResultScope,
		TypedArtifactProgramRef:         string(readiness.Version.ArtifactProgramRef),
		VerifierReadinessRef:            strings.TrimSpace(evidence.VerifierReadinessRef),
		VerifierInputBundleRef:          strings.TrimSpace(readiness.VerifierInputBundleRef),
		VerifierContractSpecRef:         strings.TrimSpace(readiness.VerifierContractSpecRef),
		EvidenceManifestRef:             strings.TrimSpace(readiness.EvidenceManifestRef),
		ExpectedVerdictPolicyRef:        strings.TrimSpace(readiness.ExpectedVerdictPolicyRef),
		VerifierRunRef:                  strings.TrimSpace(evidence.VerifierRunRef),
		VerifierResultRef:               strings.TrimSpace(evidence.VerifierResultRef),
		VerifierLogRef:                  strings.TrimSpace(evidence.VerifierLogRef),
		Verdict:                         verdict,
		FailureReason:                   failureReason,
		RollbackPlanRef:                 strings.TrimSpace(evidence.RollbackPlanRef),
		VerifierContractSatisfied:       passed,
		VerifierContractFailed:          failed,
		VerifierFailureBlocksDownstream: failed,
		OwnerApprovalRequired:           true,
		PromotionRollbackReviewRequired: true,
		PackagePublicationProofRequired: true,
		RunAcceptanceProofRequired:      true,
		FullSubstrateProofRequired:      true,
		OwnerApprovalAllowed:            false,
		PromotionAllowed:                false,
		PackagePublicationAllowed:       false,
		RunAcceptanceSynthesisAllowed:   false,
		NoOwnerApprovalMutation:         true,
		NoPromotionMutation:             true,
		NoPackagePublicationMutation:    true,
		NoRunAcceptanceMutation:         true,
		NoProductionMutation:            true,
	}, nil
}

func validateBaseVerifierResultReadiness(readiness BaseVerifierReadinessContract) error {
	if readiness.Kind != BaseVerifierReadinessContractKind {
		return fmt.Errorf("base verifier result: readiness kind is %q", readiness.Kind)
	}
	if readiness.Boundary != BaseVerifierReadinessBoundary {
		return fmt.Errorf("base verifier result: readiness boundary is %q", readiness.Boundary)
	}
	if readiness.Scope != BaseVerifierReadinessScope {
		return fmt.Errorf("base verifier result: readiness scope is %q", readiness.Scope)
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("base verifier result: readiness version is invalid")
	}
	if !readiness.Version.ArtifactProgramRef.Valid() || ArtifactProgramRef(readiness.TypedArtifactProgramRef) != readiness.Version.ArtifactProgramRef {
		return fmt.Errorf("base verifier result: readiness typed artifact-program ref is invalid")
	}
	if strings.TrimSpace(readiness.OwnerReviewReadinessRef) == "" || strings.TrimSpace(readiness.VerifierInputBundleRef) == "" || strings.TrimSpace(readiness.VerifierContractSpecRef) == "" || strings.TrimSpace(readiness.EvidenceManifestRef) == "" || strings.TrimSpace(readiness.ExpectedVerdictPolicyRef) == "" || strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("base verifier result: readiness refs are required")
	}
	if readiness.ReadinessStatus != BaseVerifierReadinessStatusReady || !readiness.VerifierReviewReady {
		return fmt.Errorf("base verifier result: readiness must be verifier-ready")
	}
	if !readiness.VerifierContractProofRequired || !readiness.OwnerApprovalRequired || !readiness.PromotionRollbackReviewRequired || !readiness.PackagePublicationProofRequired || !readiness.RunAcceptanceProofRequired || !readiness.FullSubstrateProofRequired {
		return fmt.Errorf("base verifier result: readiness must preserve downstream proof requirements")
	}
	if readiness.VerifierContractSatisfactionAllowed || readiness.OwnerApprovalAllowed || readiness.PromotionAllowed || readiness.PackagePublicationAllowed || readiness.RunAcceptanceSynthesisAllowed {
		return fmt.Errorf("base verifier result: readiness allows downstream execution")
	}
	if !readiness.NoOwnerApprovalMutation || !readiness.NoPromotionMutation || !readiness.NoPackagePublicationMutation || !readiness.NoVerifierSatisfaction || !readiness.NoRunAcceptanceMutation || !readiness.NoProductionMutation {
		return fmt.Errorf("base verifier result: readiness must prove no owner approval, promotion, package publication, verifier satisfaction, run-acceptance, or production mutation")
	}
	if readiness.OwnerApproved || readiness.PromotionExecuted || readiness.PackagePublished || readiness.VerifierContractSatisfied || readiness.RunAcceptanceRecordTouched || readiness.FullSubstrateClaimed || readiness.CompletionClaimed {
		return fmt.Errorf("base verifier result: readiness carries downstream execution or completion claims")
	}
	return nil
}

func validateBaseVerifierResultEvidence(evidence BaseVerifierResultEvidence) (string, string, error) {
	if strings.TrimSpace(evidence.VerifierReadinessRef) == "" || strings.TrimSpace(evidence.VerifierRunRef) == "" || strings.TrimSpace(evidence.VerifierResultRef) == "" || strings.TrimSpace(evidence.VerifierLogRef) == "" || strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return "", "", fmt.Errorf("base verifier result: result refs are required")
	}
	verdict := strings.TrimSpace(evidence.Verdict)
	failureReason := strings.TrimSpace(evidence.FailureReason)
	if verdict != BaseVerifierVerdictPass && verdict != BaseVerifierVerdictFail {
		return "", "", fmt.Errorf("base verifier result: verdict must be pass or fail")
	}
	if verdict == BaseVerifierVerdictPass && failureReason != "" {
		return "", "", fmt.Errorf("base verifier result: passing verdict cannot include a failure reason")
	}
	if verdict == BaseVerifierVerdictFail && failureReason == "" {
		return "", "", fmt.Errorf("base verifier result: failing verdict requires a failure reason")
	}
	if !evidence.NoOwnerApprovalMutation || !evidence.NoPromotionMutation || !evidence.NoPackagePublicationMutation || !evidence.NoRunAcceptanceMutation || !evidence.NoProductionMutation {
		return "", "", fmt.Errorf("base verifier result: evidence must prove no owner approval, promotion, package publication, run-acceptance, or production mutation")
	}
	if evidence.VerifierContractSatisfied || evidence.OwnerApproved || evidence.PromotionExecuted || evidence.PackagePublished || evidence.RunAcceptanceRecordTouched || evidence.FullSubstrateClaimed || evidence.CompletionClaimed {
		return "", "", fmt.Errorf("base verifier result: evidence carries downstream execution or completion claims")
	}
	return verdict, failureReason, nil
}
