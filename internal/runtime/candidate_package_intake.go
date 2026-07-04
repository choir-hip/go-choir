package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type candidatePackageIntakeCreateInput struct {
	IntakeID                       string          `json:"intake_id,omitempty"`
	OwnerID                        string          `json:"owner_id,omitempty"`
	CandidatePackageID             string          `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string          `json:"candidate_package_manifest_sha256"`
	SourceComputerID               string          `json:"source_computer_id,omitempty"`
	SourceCandidateID              string          `json:"source_candidate_id,omitempty"`
	CandidateSourceRef             string          `json:"candidate_source_ref,omitempty"`
	IntakeBoundary                 string          `json:"intake_boundary"`
	Status                         string          `json:"status,omitempty"`
	OwnerReviewState               string          `json:"owner_review_state,omitempty"`
	OwnerReviewRequired            bool            `json:"owner_review_required"`
	AdoptionReady                  bool            `json:"adoption_ready"`
	AdoptionBlockersJSON           json.RawMessage `json:"adoption_blockers_json,omitempty"`
	VerifierContractsJSON          json.RawMessage `json:"verifier_contracts_json,omitempty"`
	EvidenceRefsJSON               json.RawMessage `json:"evidence_refs_json,omitempty"`
	RequiredObservationsJSON       json.RawMessage `json:"required_observations_json,omitempty"`
	AcceptanceJSON                 json.RawMessage `json:"acceptance_json,omitempty"`
	TraceID                        string          `json:"trace_id,omitempty"`
}

type candidatePackageIntakeReviewInput struct {
	Decision          string `json:"decision"`
	ReviewEvidenceRef string `json:"review_evidence_ref,omitempty"`
}

type candidatePackageIntakeAdoptionBoundaryInput struct {
	AdoptionContractRef string `json:"adoption_contract_ref"`
	RollbackContractRef string `json:"rollback_contract_ref"`
	BoundaryEvidenceRef string `json:"boundary_evidence_ref,omitempty"`
}

type candidatePackageIntakePublicationDraftInput struct {
	PackageID              string `json:"package_id,omitempty"`
	AppID                  string `json:"app_id,omitempty"`
	PublicationContractRef string `json:"publication_contract_ref"`
	DraftEvidenceRef       string `json:"draft_evidence_ref,omitempty"`
}

type candidatePackageIntakeAdoptionReviewCreateInput struct {
	AdoptionID                string `json:"adoption_id,omitempty"`
	TargetComputerID          string `json:"target_computer_id"`
	TargetComputerKind        string `json:"target_computer_kind,omitempty"`
	TargetCandidateID         string `json:"target_candidate_id,omitempty"`
	CandidateSourceRef        string `json:"candidate_source_ref,omitempty"`
	PackageID                 string `json:"package_id,omitempty"`
	AdoptionReviewContractRef string `json:"adoption_review_contract_ref"`
	ReviewEvidenceRef         string `json:"review_evidence_ref,omitempty"`
}

type candidatePackageIntakeAdoptionReviewDecisionInput struct {
	Decision          string `json:"decision"`
	ReviewEvidenceRef string `json:"review_evidence_ref,omitempty"`
}

type candidatePackageIntakePromotionSwitchInput struct {
	SwitchEvidenceRef string `json:"switch_evidence_ref,omitempty"`
}

type candidatePackageIntakePromotionSwitchRollbackInput struct {
	RollbackEvidenceRef string `json:"rollback_evidence_ref,omitempty"`
}

type candidatePackageIntakePromotionSwitchRollForwardInput struct {
	RollForwardEvidenceRef string `json:"roll_forward_evidence_ref,omitempty"`
}

type candidatePackagePromotionAcceptanceEvidence struct {
	ArtifactKind                   string                                             `json:"artifact_kind"`
	AcceptanceID                   string                                             `json:"acceptance_id"`
	AcceptanceLevel                string                                             `json:"acceptance_level"`
	State                          string                                             `json:"state"`
	EvidenceScope                  string                                             `json:"evidence_scope"`
	ReviewScope                    string                                             `json:"review_scope"`
	IntakeID                       string                                             `json:"intake_id"`
	AdoptionID                     string                                             `json:"adoption_id"`
	PackageID                      string                                             `json:"package_id"`
	AppID                          string                                             `json:"app_id,omitempty"`
	CandidatePackageID             string                                             `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string                                             `json:"candidate_package_manifest_sha256"`
	SourceComputerID               string                                             `json:"source_computer_id"`
	SourceCandidateID              string                                             `json:"source_candidate_id"`
	TargetComputerID               string                                             `json:"target_computer_id"`
	TargetCandidateID              string                                             `json:"target_candidate_id,omitempty"`
	TargetActiveSourceRefAtCutover string                                             `json:"target_active_source_ref_at_cutover"`
	CandidateSourceRef             string                                             `json:"candidate_source_ref"`
	PreviousActiveRef              string                                             `json:"previous_active_source_ref"`
	CurrentAdoptionStatus          string                                             `json:"current_adoption_status"`
	OwnerReviewApproved            bool                                               `json:"owner_review_approved"`
	SourceLineageSwitched          bool                                               `json:"source_lineage_switched"`
	SourceLineageRolledBack        bool                                               `json:"source_lineage_rolled_back"`
	SourceLineageRollForwarded     bool                                               `json:"source_lineage_roll_forwarded"`
	PackagePublication             string                                             `json:"package_publication"`
	DeployedPromotion              string                                             `json:"deployed_promotion"`
	DeployedRouteMutation          string                                             `json:"deployed_route_mutation"`
	PromotionLevel                 string                                             `json:"promotion_level"`
	AuthSession                    string                                             `json:"auth_session"`
	Staging                        string                                             `json:"staging"`
	VMLifecycle                    string                                             `json:"vm_lifecycle"`
	RunAcceptanceRecord            string                                             `json:"run_acceptance_record"`
	Checkpoints                    []candidatePackagePromotionAcceptanceCheckpoint    `json:"checkpoints"`
	EvidenceRefs                   []string                                           `json:"evidence_refs"`
	BoundaryAssertions             map[string]string                                  `json:"boundary_assertions"`
	ResidualRisks                  []string                                           `json:"residual_risks"`
	VerifierContractState          []candidatePackagePromotionAcceptanceContractState `json:"verifier_contract_state,omitempty"`
}

type candidatePackageActivationDecisionBoundary struct {
	State                 string   `json:"state"`
	OwnerControlled       bool     `json:"owner_controlled"`
	RequiresAuthenticated bool     `json:"requires_authenticated_owner"`
	PreparedAction        string   `json:"prepared_action"`
	NoMutation            bool     `json:"no_mutation"`
	UsesAcceptanceID      string   `json:"uses_local_acceptance_id"`
	NextBoundary          string   `json:"next_boundary"`
	BlockedRoutes         []string `json:"blocked_routes"`
	RequiredContracts     []string `json:"required_contracts"`
}

type candidatePackagePromotionReviewSurface struct {
	ArtifactKind                   string                                      `json:"artifact_kind"`
	State                          string                                      `json:"state"`
	SurfaceScope                   string                                      `json:"surface_scope"`
	DeploymentState                string                                      `json:"deployment_state"`
	ProductVisible                 bool                                        `json:"product_visible"`
	ReadOnly                       bool                                        `json:"read_only"`
	ReviewScope                    string                                      `json:"review_scope"`
	IntakeID                       string                                      `json:"intake_id"`
	AdoptionID                     string                                      `json:"adoption_id"`
	PackageID                      string                                      `json:"package_id"`
	AppID                          string                                      `json:"app_id,omitempty"`
	CandidatePackageID             string                                      `json:"candidate_package_id"`
	CandidatePackageManifestSHA256 string                                      `json:"candidate_package_manifest_sha256"`
	SourceComputerID               string                                      `json:"source_computer_id"`
	SourceCandidateID              string                                      `json:"source_candidate_id"`
	TargetComputerID               string                                      `json:"target_computer_id"`
	TargetCandidateID              string                                      `json:"target_candidate_id,omitempty"`
	TargetActiveSourceRefAtCutover string                                      `json:"target_active_source_ref_at_cutover"`
	CandidateSourceRef             string                                      `json:"candidate_source_ref"`
	CurrentAdoptionStatus          string                                      `json:"current_adoption_status"`
	LocalAcceptanceID              string                                      `json:"local_acceptance_id"`
	LocalAcceptanceLevel           string                                      `json:"local_acceptance_level"`
	LocalAcceptanceState           string                                      `json:"local_acceptance_state"`
	PackagePublication             string                                      `json:"package_publication"`
	DeployedPromotion              string                                      `json:"deployed_promotion"`
	DeployedRouteMutation          string                                      `json:"deployed_route_mutation"`
	PromotionLevel                 string                                      `json:"promotion_level"`
	AuthSession                    string                                      `json:"auth_session"`
	Staging                        string                                      `json:"staging"`
	VMLifecycle                    string                                      `json:"vm_lifecycle"`
	RunAcceptanceRecord            string                                      `json:"run_acceptance_record"`
	AppChangePackageMutation       string                                      `json:"app_change_package_mutation"`
	AppAdoptionMutation            string                                      `json:"app_adoption_mutation"`
	AllowedActions                 []string                                    `json:"allowed_actions"`
	BlockedActions                 []string                                    `json:"blocked_actions"`
	ActivationDecisionBoundary     candidatePackageActivationDecisionBoundary  `json:"activation_decision_boundary"`
	AcceptanceEvidence             candidatePackagePromotionAcceptanceEvidence `json:"acceptance_evidence"`
	BoundaryAssertions             map[string]string                           `json:"boundary_assertions"`
	ResidualRisks                  []string                                    `json:"residual_risks"`
}
type candidatePackagePromotionAcceptanceCheckpoint struct {
	Kind  string `json:"kind"`
	State string `json:"state"`
}

type candidatePackagePromotionAcceptanceContractState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

func (rt *Runtime) CreateCandidatePackageIntake(ctx context.Context, ownerID string, in candidatePackageIntakeCreateInput) (types.CandidatePackageIntakeRecord, error) {
	if rt == nil || rt.store == nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake: owner_id is required")
	}
	if strings.TrimSpace(in.OwnerID) != "" && strings.TrimSpace(in.OwnerID) != ownerID {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake: owner_id does not match authenticated owner")
	}
	if in.AdoptionReady {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake: adoption_ready is not allowed at intake creation")
	}
	rec := types.CandidatePackageIntakeRecord{
		IntakeID:                       strings.TrimSpace(in.IntakeID),
		OwnerID:                        ownerID,
		CandidatePackageID:             strings.TrimSpace(in.CandidatePackageID),
		CandidatePackageManifestSHA256: strings.TrimSpace(in.CandidatePackageManifestSHA256),
		SourceComputerID:               strings.TrimSpace(in.SourceComputerID),
		SourceCandidateID:              strings.TrimSpace(in.SourceCandidateID),
		CandidateSourceRef:             strings.TrimSpace(in.CandidateSourceRef),
		IntakeBoundary:                 strings.TrimSpace(in.IntakeBoundary),
		Status:                         types.CandidatePackageIntakeStatus(strings.TrimSpace(in.Status)),
		OwnerReviewState:               types.CandidatePackageOwnerReviewState(strings.TrimSpace(in.OwnerReviewState)),
		OwnerReviewRequired:            in.OwnerReviewRequired,
		AdoptionReady:                  in.AdoptionReady,
		AdoptionBlockersJSON:           rawJSONOrFallback(in.AdoptionBlockersJSON, "[]"),
		VerifierContractsJSON:          rawJSONOrFallback(in.VerifierContractsJSON, "[]"),
		EvidenceRefsJSON:               rawJSONOrFallback(in.EvidenceRefsJSON, "[]"),
		RequiredObservationsJSON:       rawJSONOrFallback(in.RequiredObservationsJSON, "[]"),
		AcceptanceJSON:                 rawJSONOrFallback(in.AcceptanceJSON, "{}"),
		TraceID:                        strings.TrimSpace(in.TraceID),
	}
	if rec.Status == "" {
		rec.Status = types.CandidatePackageIntakeOwnerReviewPending
	}
	if rec.OwnerReviewState == "" {
		rec.OwnerReviewState = types.CandidatePackageOwnerReviewRequired
	}
	return rt.store.UpsertCandidatePackageIntake(ctx, rec)
}

func (rt *Runtime) ReviewCandidatePackageIntake(ctx context.Context, ownerID, intakeID string, in candidatePackageIntakeReviewInput) (types.CandidatePackageIntakeRecord, error) {
	if rt == nil || rt.store == nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake review: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	if ownerID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake review: owner_id is required")
	}
	if intakeID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake review: intake_id is required")
	}
	decision := strings.ToLower(strings.TrimSpace(in.Decision))
	if decision != "approve" && decision != "reject" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake review: decision must be approve or reject")
	}
	rec, err := rt.store.GetCandidatePackageIntake(ctx, ownerID, intakeID)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, err
	}
	previousUpdatedAt := rec.UpdatedAt
	if rec.Status != types.CandidatePackageIntakeOwnerReviewPending || rec.OwnerReviewState != types.CandidatePackageOwnerReviewRequired {
		return rec, fmt.Errorf("candidate package intake review: intake %s is already terminal with status %q and owner_review_state %q", rec.IntakeID, rec.Status, rec.OwnerReviewState)
	}
	rec.OwnerReviewRequired = false
	rec.AdoptionReady = false
	switch decision {
	case "approve":
		rec.Status = types.CandidatePackageIntakeOwnerApproved
		rec.OwnerReviewState = types.CandidatePackageOwnerReviewApproved
		blockers, err := candidatePackageIntakeStringArray(rec.AdoptionBlockersJSON)
		if err != nil {
			return rec, fmt.Errorf("candidate package intake review: adoption_blockers_json is invalid: %w", err)
		}
		rec.AdoptionBlockersJSON = candidatePackageIntakeStringArrayJSON(candidatePackageIntakeRemoveBlockers(blockers, "owner_review_not_recorded", "candidate_package_has_no_product_api_intake_record"), "adoption_rollback_boundary_not_bound")
	case "reject":
		rec.Status = types.CandidatePackageIntakeRejected
		rec.OwnerReviewState = types.CandidatePackageOwnerReviewRejected
		blockers, err := candidatePackageIntakeStringArray(rec.AdoptionBlockersJSON)
		if err != nil {
			return rec, fmt.Errorf("candidate package intake review: adoption_blockers_json is invalid: %w", err)
		}
		rec.AdoptionBlockersJSON = candidatePackageIntakeStringArrayJSON(append(blockers, "owner_review_rejected"), "owner_review_rejected")
	}
	evidenceRefs, err := candidatePackageIntakeStringArray(rec.EvidenceRefsJSON)
	if err != nil {
		return rec, fmt.Errorf("candidate package intake review: evidence_refs_json is invalid: %w", err)
	}
	if ref := strings.TrimSpace(in.ReviewEvidenceRef); ref != "" {
		evidenceRefs = append(evidenceRefs, ref)
	}
	rec.EvidenceRefsJSON = candidatePackageIntakeStringArrayJSON(evidenceRefs)
	return rt.store.UpdateCandidatePackageIntakeIfCurrent(ctx, rec, previousUpdatedAt)
}

func (rt *Runtime) BindCandidatePackageIntakeAdoptionBoundary(ctx context.Context, ownerID, intakeID string, in candidatePackageIntakeAdoptionBoundaryInput) (types.CandidatePackageIntakeRecord, error) {
	if rt == nil || rt.store == nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake adoption boundary: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	if ownerID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake adoption boundary: owner_id is required")
	}
	if intakeID == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake adoption boundary: intake_id is required")
	}
	adoptionContractRef := strings.TrimSpace(in.AdoptionContractRef)
	rollbackContractRef := strings.TrimSpace(in.RollbackContractRef)
	if adoptionContractRef == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake adoption boundary: adoption_contract_ref is required")
	}
	if rollbackContractRef == "" {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake adoption boundary: rollback_contract_ref is required")
	}
	rec, err := rt.store.GetCandidatePackageIntake(ctx, ownerID, intakeID)
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, err
	}
	previousUpdatedAt := rec.UpdatedAt
	if rec.Status != types.CandidatePackageIntakeOwnerApproved || rec.OwnerReviewState != types.CandidatePackageOwnerReviewApproved || rec.OwnerReviewRequired {
		return rec, fmt.Errorf("candidate package intake adoption boundary: intake %s is not owner-approved", rec.IntakeID)
	}
	if rec.AdoptionReady {
		return rec, fmt.Errorf("candidate package intake adoption boundary: intake %s is already adoption-ready", rec.IntakeID)
	}
	blockers, err := candidatePackageIntakeStringArray(rec.AdoptionBlockersJSON)
	if err != nil {
		return rec, fmt.Errorf("candidate package intake adoption boundary: adoption_blockers_json is invalid: %w", err)
	}
	rec.AdoptionBlockersJSON = candidatePackageIntakeStringArrayJSON(candidatePackageIntakeRemoveBlockers(blockers, "adoption_rollback_boundary_not_bound"))
	remainingBlockers, err := candidatePackageIntakeStringArray(rec.AdoptionBlockersJSON)
	if err != nil {
		return rec, fmt.Errorf("candidate package intake adoption boundary: adoption_blockers_json is invalid after update: %w", err)
	}
	rec.AdoptionReady = len(remainingBlockers) == 0
	rec.AcceptanceJSON, err = candidatePackageIntakeWithAdoptionBoundary(rec.AcceptanceJSON, adoptionContractRef, rollbackContractRef)
	if err != nil {
		return rec, err
	}
	evidenceRefs, err := candidatePackageIntakeStringArray(rec.EvidenceRefsJSON)
	if err != nil {
		return rec, fmt.Errorf("candidate package intake adoption boundary: evidence_refs_json is invalid: %w", err)
	}
	if ref := strings.TrimSpace(in.BoundaryEvidenceRef); ref != "" {
		evidenceRefs = append(evidenceRefs, ref)
	}
	rec.EvidenceRefsJSON = candidatePackageIntakeStringArrayJSON(evidenceRefs)
	return rt.store.UpdateCandidatePackageIntakeIfCurrent(ctx, rec, previousUpdatedAt)
}

func (rt *Runtime) CreateCandidatePackageIntakePublicationDraft(ctx context.Context, ownerID, intakeID string, in candidatePackageIntakePublicationDraftInput) (types.AppChangePackageRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	if ownerID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: owner_id is required")
	}
	if intakeID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: intake_id is required")
	}
	publicationContractRef := strings.TrimSpace(in.PublicationContractRef)
	if publicationContractRef == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: publication_contract_ref is required")
	}
	intake, err := rt.store.GetCandidatePackageIntake(ctx, ownerID, intakeID)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	if intake.Status != types.CandidatePackageIntakeOwnerApproved || intake.OwnerReviewState != types.CandidatePackageOwnerReviewApproved || intake.OwnerReviewRequired {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: intake %s is not owner-approved", intake.IntakeID)
	}
	if !intake.AdoptionReady {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: intake %s is not adoption-ready", intake.IntakeID)
	}
	blockers, err := candidatePackageIntakeStringArray(intake.AdoptionBlockersJSON)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: adoption_blockers_json is invalid: %w", err)
	}
	if len(blockers) != 0 {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: intake %s still has adoption blockers", intake.IntakeID)
	}
	adoptionContractRef, rollbackContractRef, err := candidatePackageIntakeAdoptionBoundaryRefs(intake.AcceptanceJSON)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	packageID := strings.TrimSpace(in.PackageID)
	if packageID == "" {
		packageID = strings.TrimSpace(intake.CandidatePackageID)
	}
	if packageID == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: package_id is required")
	}
	existing, err := rt.store.GetAppChangePackage(ctx, packageID)
	if err == nil {
		if existing.OwnerID == ownerID && candidatePackageDraftMatchesIntake(existing, intake.IntakeID) {
			return existing, nil
		}
		return types.AppChangePackageRecord{}, fmt.Errorf("candidate package intake publication draft: package_id %q already exists for another package", packageID)
	}
	if err != store.ErrNotFound {
		return types.AppChangePackageRecord{}, err
	}
	appID := strings.TrimSpace(in.AppID)
	if appID == "" {
		appID = "candidate-computer-package"
	}
	manifest := map[string]any{
		"kind":                                 "candidate_package_publication_draft",
		"package_id":                           packageID,
		"app_id":                               appID,
		"owner_id":                             ownerID,
		"candidate_package_intake_id":          intake.IntakeID,
		"candidate_package_id":                 intake.CandidatePackageID,
		"candidate_package_manifest_sha256":    intake.CandidatePackageManifestSHA256,
		"source_computer_id":                   intake.SourceComputerID,
		"source_candidate_id":                  intake.SourceCandidateID,
		"candidate_source_ref":                 intake.CandidateSourceRef,
		"intake_boundary":                      intake.IntakeBoundary,
		"adoption_ready":                       true,
		"publication_contract_ref":             publicationContractRef,
		"adoption_contract_ref":                adoptionContractRef,
		"rollback_contract_ref":                rollbackContractRef,
		"direct_app_change_package_publish":    "blocked",
		"app_adoption_creation":                "blocked",
		"promotion":                            "blocked",
		"deployed_route_mutation":              "blocked",
		"vm_lifecycle":                         "blocked",
		"requires_runtime_or_ui_source_delta":  true,
		"requires_adoption_consumer_follow_up": true,
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	verifierContracts, err := candidatePackagePublicationDraftContractsJSON(intake.VerifierContractsJSON, publicationContractRef, adoptionContractRef, rollbackContractRef)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	provenanceRefs, err := candidatePackagePublicationDraftProvenanceJSON(intake.EvidenceRefsJSON, strings.TrimSpace(in.DraftEvidenceRef), intake.IntakeID)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	rec := types.AppChangePackageRecord{
		PackageID:             packageID,
		OwnerID:               ownerID,
		AppID:                 appID,
		Status:                types.AppChangePackageDraft,
		Visibility:            "private",
		SourceComputerID:      intake.SourceComputerID,
		SourceCandidateID:     intake.SourceCandidateID,
		CandidateSourceRef:    intake.CandidateSourceRef,
		PackageManifestSHA256: sha256Hex(string(manifestJSON)),
		ManifestJSON:          manifestJSON,
		VerifierContractsJSON: verifierContracts,
		ProvenanceRefsJSON:    provenanceRefs,
		TraceID:               intake.TraceID,
	}
	return rt.store.UpsertAppChangePackage(ctx, rec)
}

func (rt *Runtime) CreateCandidatePackageIntakeAdoptionReview(ctx context.Context, ownerID, intakeID string, in candidatePackageIntakeAdoptionReviewCreateInput) (types.AppAdoptionRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: owner_id is required")
	}
	if intakeID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: intake_id is required")
	}
	adoptionReviewContractRef := strings.TrimSpace(in.AdoptionReviewContractRef)
	if adoptionReviewContractRef == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: adoption_review_contract_ref is required")
	}
	targetComputerID := strings.TrimSpace(in.TargetComputerID)
	if targetComputerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: target_computer_id is required")
	}
	intake, pkg, adoptionContractRef, rollbackContractRef, err := rt.loadCandidatePackagePublicationDraftForAdoptionReview(ctx, ownerID, intakeID, in.PackageID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if existing, err := rt.candidatePackageAdoptionReviewForPackage(ctx, ownerID, pkg.PackageID); err == nil {
		return existing, fmt.Errorf("candidate package intake adoption review: package %q already has an adoption review", pkg.PackageID)
	} else if err != store.ErrNotFound {
		return types.AppAdoptionRecord{}, err
	}
	targetKind := strings.TrimSpace(in.TargetComputerKind)
	if targetKind == "" {
		targetKind = computerKindForID(targetComputerID)
	}
	lineage, err := rt.EnsureComputerSourceLineage(ctx, ownerID, targetComputerID, targetKind, "")
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	targetCandidateID := strings.TrimSpace(in.TargetCandidateID)
	if targetCandidateID == "" {
		targetCandidateID = uuid.NewString()
	}
	candidateRef := strings.TrimSpace(in.CandidateSourceRef)
	if candidateRef == "" {
		candidateRef = candidateSourceRefForComputer(targetComputerID, targetKind, targetCandidateID)
	}
	rec := types.AppAdoptionRecord{
		AdoptionID:                            strings.TrimSpace(in.AdoptionID),
		OwnerID:                               ownerID,
		PackageID:                             pkg.PackageID,
		AppID:                                 pkg.AppID,
		TargetComputerID:                      targetComputerID,
		TargetComputerKind:                    targetKind,
		TargetCandidateID:                     targetCandidateID,
		Status:                                types.AppAdoptionOwnerReviewPending,
		TargetActiveSourceRefAtCandidateStart: lineage.ActiveSourceRef,
		CandidateSourceRef:                    candidateRef,
		MergeConflictsJSON:                    json.RawMessage(`[]`),
		VerifierResultsJSON:                   candidatePackageAdoptionReviewVerifierResultsJSON(intake, pkg, adoptionReviewContractRef, adoptionContractRef, rollbackContractRef, "pending", strings.TrimSpace(in.ReviewEvidenceRef)),
		RollbackProfileJSON:                   candidatePackageAdoptionReviewRollbackProfileJSON(lineage, adoptionReviewContractRef, adoptionContractRef, rollbackContractRef),
		RouteProfile:                          firstNonEmptyPromotion(lineage.RouteProfile, "route:"+safeRefPart(targetComputerID)),
		DefaultBaseProfile:                    lineage.DefaultBaseProfile,
		TraceID:                               firstNonEmptyPromotion(intake.TraceID, pkg.TraceID),
	}
	rec, err = rt.store.UpsertAppAdoption(ctx, rec)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionProposed, "adoption", map[string]any{
		"adoption_id":                  rec.AdoptionID,
		"package_id":                   rec.PackageID,
		"candidate_package_intake_id":  intake.IntakeID,
		"target_computer_id":           rec.TargetComputerID,
		"adoption_review_contract_ref": adoptionReviewContractRef,
		"package_publication":          "blocked",
		"promotion":                    "blocked",
		"deployed_route_mutation":      "blocked",
		"vm_lifecycle":                 "blocked",
		"continuous_app_change":        true,
	})
	return rec, nil
}

func (rt *Runtime) ReviewCandidatePackageIntakeAdoption(ctx context.Context, ownerID, intakeID, adoptionID string, in candidatePackageIntakeAdoptionReviewDecisionInput) (types.AppAdoptionRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	adoptionID = strings.TrimSpace(adoptionID)
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: owner_id is required")
	}
	if intakeID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: intake_id is required")
	}
	if adoptionID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: adoption_id is required")
	}
	decision := strings.ToLower(strings.TrimSpace(in.Decision))
	if decision != "approve" && decision != "reject" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake adoption review: decision must be approve or reject")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	previousUpdatedAt := rec.UpdatedAt
	intake, pkg, _, _, err := rt.loadCandidatePackagePublicationDraftForAdoptionReview(ctx, ownerID, intakeID, rec.PackageID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.PackageID != pkg.PackageID {
		return rec, fmt.Errorf("candidate package intake adoption review: adoption %s is not bound to intake %s", rec.AdoptionID, intake.IntakeID)
	}
	if rec.Status != types.AppAdoptionOwnerReviewPending {
		return rec, fmt.Errorf("candidate package intake adoption review: adoption %s is already terminal with status %q", rec.AdoptionID, rec.Status)
	}
	switch decision {
	case "approve":
		rec.Status = types.AppAdoptionOwnerReviewApproved
		rec.Error = ""
	case "reject":
		rec.Status = types.AppAdoptionOwnerReviewRejected
		rec.Error = "owner rejected candidate-package adoption review"
	}
	rec.VerifierResultsJSON = candidatePackageAppendAdoptionReviewDecisionJSON(rec.VerifierResultsJSON, decision, strings.TrimSpace(in.ReviewEvidenceRef))
	rec, err = rt.store.UpdateAppAdoptionIfCurrent(ctx, rec, previousUpdatedAt)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionOwnerReviewResolved, "adoption", map[string]any{
		"adoption_id":                 rec.AdoptionID,
		"package_id":                  rec.PackageID,
		"candidate_package_intake_id": intake.IntakeID,
		"decision":                    decision,
		"package_publication":         "blocked",
		"promotion":                   "blocked",
		"deployed_route_mutation":     "blocked",
		"vm_lifecycle":                "blocked",
		"continuous_app_change":       true,
	})
	return rec, nil
}

func (rt *Runtime) SwitchCandidatePackageIntakeAdoptionReview(ctx context.Context, ownerID, intakeID, adoptionID string, in candidatePackageIntakePromotionSwitchInput) (types.AppAdoptionRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	adoptionID = strings.TrimSpace(adoptionID)
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch: owner_id is required")
	}
	if intakeID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch: intake_id is required")
	}
	if adoptionID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch: adoption_id is required")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	previousUpdatedAt := rec.UpdatedAt
	intake, pkg, adoptionContractRef, rollbackContractRef, err := rt.loadCandidatePackagePublicationDraftForAdoptionReview(ctx, ownerID, intakeID, rec.PackageID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.PackageID != pkg.PackageID {
		return rec, fmt.Errorf("candidate package intake promotion switch: adoption %s is not bound to intake %s", rec.AdoptionID, intake.IntakeID)
	}
	if rec.Status == types.AppAdoptionSourceLineageSwitched {
		return rec, fmt.Errorf("candidate package intake promotion switch: adoption %s already switched source lineage", rec.AdoptionID)
	}
	if rec.Status != types.AppAdoptionOwnerReviewApproved {
		return rec, fmt.Errorf("candidate package intake promotion switch: adoption status %q is not owner_review_approved", rec.Status)
	}
	if strings.TrimSpace(rec.CandidateSourceRef) == "" || !strings.Contains(rec.CandidateSourceRef, "/candidates/") {
		return rec, fmt.Errorf("candidate package intake promotion switch: candidate_source_ref must be a candidate ref")
	}
	lineage, err := rt.store.GetComputerSourceLineage(ctx, ownerID, rec.TargetComputerID)
	if err != nil {
		return rec, err
	}
	if strings.TrimSpace(lineage.ActiveSourceRef) != strings.TrimSpace(rec.TargetActiveSourceRefAtCandidateStart) {
		return rec, fmt.Errorf("candidate package intake promotion switch: foreground lineage moved since adoption review (reviewed against %q, now %q); re-review before switching", rec.TargetActiveSourceRefAtCandidateStart, lineage.ActiveSourceRef)
	}
	if err := candidatePackageAdoptionReviewSwitchProfileMatches(rec.RollbackProfileJSON, lineage.ActiveSourceRef, adoptionContractRef, rollbackContractRef); err != nil {
		return rec, err
	}
	rec.TargetActiveSourceRefAtCutover = lineage.ActiveSourceRef
	rec.Status = types.AppAdoptionSourceLineageSwitched
	rec.Error = ""
	rec.VerifierResultsJSON = candidatePackageAppendPromotionSwitchJSON(rec.VerifierResultsJSON, strings.TrimSpace(in.SwitchEvidenceRef))
	rec.RollbackProfileJSON = candidatePackageAdoptionReviewSwitchRollbackProfileJSON(rec.RollbackProfileJSON, rec.CandidateSourceRef, strings.TrimSpace(in.SwitchEvidenceRef))
	rec, err = rt.store.UpdateAppAdoptionIfCurrent(ctx, rec, previousUpdatedAt)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	lineage.ActiveSourceRef = rec.CandidateSourceRef
	lineage.RouteProfile = firstNonEmptyPromotion(rec.RouteProfile, lineage.RouteProfile)
	lineage.DefaultBaseProfile = firstNonEmptyPromotion(rec.DefaultBaseProfile, lineage.DefaultBaseProfile)
	lineage.LastAdoptionID = rec.AdoptionID
	lineage.LastPackageID = pkg.PackageID
	lineage.LastCandidateRef = rec.CandidateSourceRef
	lineage.UpdatedAt = time.Now().UTC()
	if _, err := rt.store.UpsertComputerSourceLineage(ctx, lineage); err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionSourceLineageSwitched, "adoption", map[string]any{
		"adoption_id":                 rec.AdoptionID,
		"package_id":                  rec.PackageID,
		"candidate_package_intake_id": intake.IntakeID,
		"target_computer_id":          rec.TargetComputerID,
		"candidate_source_ref":        rec.CandidateSourceRef,
		"previous_active_source_ref":  rec.TargetActiveSourceRefAtCutover,
		"package_publication":         "blocked",
		"deployed_route_mutation":     "blocked",
		"vm_lifecycle":                "blocked",
		"rollback_execution":          "blocked",
		"promotion_mode":              "source_lineage_only",
		"continuous_app_change":       true,
	})
	return rec, nil
}

func (rt *Runtime) RollbackCandidatePackageIntakeAdoptionReview(ctx context.Context, ownerID, intakeID, adoptionID string, in candidatePackageIntakePromotionSwitchRollbackInput) (types.AppAdoptionRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch rollback: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	adoptionID = strings.TrimSpace(adoptionID)
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch rollback: owner_id is required")
	}
	if intakeID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch rollback: intake_id is required")
	}
	if adoptionID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch rollback: adoption_id is required")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	previousUpdatedAt := rec.UpdatedAt
	intake, pkg, adoptionContractRef, rollbackContractRef, err := rt.loadCandidatePackagePublicationDraftForAdoptionReview(ctx, ownerID, intakeID, rec.PackageID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.PackageID != pkg.PackageID {
		return rec, fmt.Errorf("candidate package intake promotion switch rollback: adoption %s is not bound to intake %s", rec.AdoptionID, intake.IntakeID)
	}
	if rec.Status == types.AppAdoptionRolledBack {
		return rec, fmt.Errorf("candidate package intake promotion switch rollback: adoption %s is already rolled back", rec.AdoptionID)
	}
	if rec.Status != types.AppAdoptionSourceLineageSwitched {
		return rec, fmt.Errorf("candidate package intake promotion switch rollback: adoption status %q is not source_lineage_switched", rec.Status)
	}
	if strings.TrimSpace(rec.CandidateSourceRef) == "" || !strings.Contains(rec.CandidateSourceRef, "/candidates/") {
		return rec, fmt.Errorf("candidate package intake promotion switch rollback: candidate_source_ref must be a candidate ref")
	}
	if strings.TrimSpace(rec.TargetActiveSourceRefAtCutover) == "" {
		return rec, fmt.Errorf("candidate package intake promotion switch rollback: target_active_source_ref_at_cutover is required")
	}
	lineage, err := rt.store.GetComputerSourceLineage(ctx, ownerID, rec.TargetComputerID)
	if err != nil {
		return rec, err
	}
	if strings.TrimSpace(lineage.ActiveSourceRef) != strings.TrimSpace(rec.CandidateSourceRef) {
		return rec, fmt.Errorf("candidate package intake promotion switch rollback: foreground lineage moved since switch (switched to %q, now %q); re-review before rollback", rec.CandidateSourceRef, lineage.ActiveSourceRef)
	}
	previousRouteProfile, err := candidatePackageAdoptionReviewRollbackProfileForRollback(rec.RollbackProfileJSON, rec.TargetActiveSourceRefAtCutover, rec.CandidateSourceRef, adoptionContractRef, rollbackContractRef)
	if err != nil {
		return rec, err
	}
	rec.Status = types.AppAdoptionRolledBack
	rec.Error = ""
	rec.VerifierResultsJSON = candidatePackageAppendPromotionSwitchRollbackJSON(rec.VerifierResultsJSON, strings.TrimSpace(in.RollbackEvidenceRef))
	rec.RollbackProfileJSON = candidatePackageAdoptionReviewSwitchRolledBackProfileJSON(rec.RollbackProfileJSON, rec.TargetActiveSourceRefAtCutover, strings.TrimSpace(in.RollbackEvidenceRef))
	rec, err = rt.store.UpdateAppAdoptionIfCurrent(ctx, rec, previousUpdatedAt)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	lineage.ActiveSourceRef = rec.TargetActiveSourceRefAtCutover
	lineage.RouteProfile = previousRouteProfile
	lineage.DefaultBaseProfile = rec.DefaultBaseProfile
	lineage.LastAdoptionID = rec.AdoptionID
	lineage.LastPackageID = pkg.PackageID
	lineage.LastCandidateRef = ""
	lineage.UpdatedAt = time.Now().UTC()
	if _, err := rt.store.UpsertComputerSourceLineage(ctx, lineage); err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionRolledBack, "adoption", map[string]any{
		"adoption_id":                 rec.AdoptionID,
		"package_id":                  rec.PackageID,
		"candidate_package_intake_id": intake.IntakeID,
		"target_computer_id":          rec.TargetComputerID,
		"candidate_source_ref":        rec.CandidateSourceRef,
		"restored_active_source_ref":  rec.TargetActiveSourceRefAtCutover,
		"package_publication":         "blocked",
		"deployed_route_mutation":     "blocked",
		"vm_lifecycle":                "blocked",
		"rollback_mode":               "source_lineage_only",
		"continuous_app_change":       true,
	})
	return rec, nil
}

func (rt *Runtime) RollForwardCandidatePackageIntakeAdoptionReview(ctx context.Context, ownerID, intakeID, adoptionID string, in candidatePackageIntakePromotionSwitchRollForwardInput) (types.AppAdoptionRecord, error) {
	if rt == nil || rt.store == nil {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch roll-forward: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	adoptionID = strings.TrimSpace(adoptionID)
	if ownerID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch roll-forward: owner_id is required")
	}
	if intakeID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch roll-forward: intake_id is required")
	}
	if adoptionID == "" {
		return types.AppAdoptionRecord{}, fmt.Errorf("candidate package intake promotion switch roll-forward: adoption_id is required")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, adoptionID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	previousUpdatedAt := rec.UpdatedAt
	intake, pkg, adoptionContractRef, rollbackContractRef, err := rt.loadCandidatePackagePublicationDraftForAdoptionReview(ctx, ownerID, intakeID, rec.PackageID)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	if rec.PackageID != pkg.PackageID {
		return rec, fmt.Errorf("candidate package intake promotion switch roll-forward: adoption %s is not bound to intake %s", rec.AdoptionID, intake.IntakeID)
	}
	if rec.Status != types.AppAdoptionRolledBack {
		return rec, fmt.Errorf("candidate package intake promotion switch roll-forward: adoption status %q is not rolled_back", rec.Status)
	}
	if strings.TrimSpace(rec.CandidateSourceRef) == "" || !strings.Contains(rec.CandidateSourceRef, "/candidates/") {
		return rec, fmt.Errorf("candidate package intake promotion switch roll-forward: candidate_source_ref must be a candidate ref")
	}
	if strings.TrimSpace(rec.TargetActiveSourceRefAtCutover) == "" {
		return rec, fmt.Errorf("candidate package intake promotion switch roll-forward: target_active_source_ref_at_cutover is required")
	}
	lineage, err := rt.store.GetComputerSourceLineage(ctx, ownerID, rec.TargetComputerID)
	if err != nil {
		return rec, err
	}
	if strings.TrimSpace(lineage.ActiveSourceRef) != strings.TrimSpace(rec.TargetActiveSourceRefAtCutover) {
		return rec, fmt.Errorf("candidate package intake promotion switch roll-forward: foreground lineage moved since rollback (rolled back to %q, now %q); re-review before roll-forward", rec.TargetActiveSourceRefAtCutover, lineage.ActiveSourceRef)
	}
	if _, err := candidatePackageAdoptionReviewRollbackProfileForRollForward(rec.RollbackProfileJSON, rec.TargetActiveSourceRefAtCutover, rec.CandidateSourceRef, adoptionContractRef, rollbackContractRef); err != nil {
		return rec, err
	}
	rec.Status = types.AppAdoptionSourceLineageSwitched
	rec.Error = ""
	rec.VerifierResultsJSON = candidatePackageAppendPromotionSwitchRollForwardJSON(rec.VerifierResultsJSON, strings.TrimSpace(in.RollForwardEvidenceRef))
	rec.RollbackProfileJSON = candidatePackageAdoptionReviewSwitchRolledForwardProfileJSON(rec.RollbackProfileJSON, rec.CandidateSourceRef, strings.TrimSpace(in.RollForwardEvidenceRef))
	rec, err = rt.store.UpdateAppAdoptionIfCurrent(ctx, rec, previousUpdatedAt)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	lineage.ActiveSourceRef = rec.CandidateSourceRef
	lineage.RouteProfile = firstNonEmptyPromotion(rec.RouteProfile, lineage.RouteProfile)
	lineage.DefaultBaseProfile = firstNonEmptyPromotion(rec.DefaultBaseProfile, lineage.DefaultBaseProfile)
	lineage.LastAdoptionID = rec.AdoptionID
	lineage.LastPackageID = pkg.PackageID
	lineage.LastCandidateRef = rec.CandidateSourceRef
	lineage.UpdatedAt = time.Now().UTC()
	if _, err := rt.store.UpsertComputerSourceLineage(ctx, lineage); err != nil {
		return types.AppAdoptionRecord{}, err
	}
	rt.emitAppPromotionEvent(ctx, ownerID, rec.TraceID, types.EventAppAdoptionSourceLineageSwitched, "adoption", map[string]any{
		"adoption_id":                 rec.AdoptionID,
		"package_id":                  rec.PackageID,
		"candidate_package_intake_id": intake.IntakeID,
		"target_computer_id":          rec.TargetComputerID,
		"candidate_source_ref":        rec.CandidateSourceRef,
		"previous_active_source_ref":  rec.TargetActiveSourceRefAtCutover,
		"package_publication":         "blocked",
		"deployed_route_mutation":     "blocked",
		"vm_lifecycle":                "blocked",
		"rollback_execution":          "blocked",
		"promotion_mode":              "source_lineage_only_roll_forward",
		"continuous_app_change":       true,
	})
	return rec, nil
}

func (rt *Runtime) CandidatePackagePromotionAcceptanceEvidence(ctx context.Context, ownerID, intakeID, adoptionID string) (candidatePackagePromotionAcceptanceEvidence, error) {
	if rt == nil || rt.store == nil {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	intakeID = strings.TrimSpace(intakeID)
	adoptionID = strings.TrimSpace(adoptionID)
	if ownerID == "" {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: owner_id is required")
	}
	if intakeID == "" {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: intake_id is required")
	}
	if adoptionID == "" {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: adoption_id is required")
	}
	rec, err := rt.store.GetAppAdoption(ctx, ownerID, adoptionID)
	if err != nil {
		return candidatePackagePromotionAcceptanceEvidence{}, err
	}
	intake, pkg, adoptionContractRef, rollbackContractRef, err := rt.loadCandidatePackagePublicationDraftForAdoptionReview(ctx, ownerID, intakeID, rec.PackageID)
	if err != nil {
		return candidatePackagePromotionAcceptanceEvidence{}, err
	}
	if rec.PackageID != pkg.PackageID {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: adoption %s is not bound to intake %s", rec.AdoptionID, intake.IntakeID)
	}
	if strings.TrimSpace(rec.CandidateSourceRef) == "" || strings.TrimSpace(rec.TargetActiveSourceRefAtCutover) == "" {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: source-lineage switch refs are incomplete")
	}
	lineage, err := rt.store.GetComputerSourceLineage(ctx, ownerID, rec.TargetComputerID)
	if err != nil {
		return candidatePackagePromotionAcceptanceEvidence{}, err
	}
	rawProfile := rawJSONOrFallback(rec.RollbackProfileJSON, "{}")
	var profile map[string]any
	if err := json.Unmarshal(rawProfile, &profile); err != nil {
		profile = map[string]any{}
	}
	switchEvidenceRef := strings.TrimSpace(stringFromMap(profile, "switch_evidence_ref"))
	rollbackEvidenceRef := strings.TrimSpace(stringFromMap(profile, "rollback_evidence_ref"))
	rollForwardEvidenceRef := strings.TrimSpace(stringFromMap(profile, "roll_forward_evidence_ref"))
	checkpoints := []candidatePackagePromotionAcceptanceCheckpoint{
		{Kind: "owner_review_approved", State: candidatePackagePromotionAcceptanceCheckpointState(rec.VerifierResultsJSON, "candidate-package-draft-owner-adoption-review-decision", "approve")},
		{Kind: "source_lineage_switched", State: candidatePackagePromotionAcceptanceCheckpointState(rec.VerifierResultsJSON, "candidate-package-source-lineage-switch", "source_lineage_switched")},
		{Kind: "source_lineage_rolled_back", State: candidatePackagePromotionAcceptanceCheckpointState(rec.VerifierResultsJSON, "candidate-package-source-lineage-switch-rollback", "source_lineage_rolled_back")},
		{Kind: "source_lineage_roll_forwarded", State: candidatePackagePromotionAcceptanceCheckpointState(rec.VerifierResultsJSON, "candidate-package-source-lineage-switch-roll-forward", "source_lineage_switched")},
	}
	ownerReviewApproved := checkpoints[0].State == "verified"
	sourceLineageSwitched := checkpoints[1].State == "verified"
	sourceLineageRolledBack := checkpoints[2].State == "verified"
	sourceLineageRollForwarded := checkpoints[3].State == "verified"
	acceptanceProfile, profileErr := candidatePackagePromotionAcceptanceProfile(rec.RollbackProfileJSON, rec.TargetActiveSourceRefAtCutover, rec.CandidateSourceRef, adoptionContractRef, rollbackContractRef)
	if profileErr == nil {
		profile = acceptanceProfile
	}
	if rec.Status != types.AppAdoptionSourceLineageSwitched {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: adoption status %q is not source_lineage_switched", rec.Status)
	}
	if strings.TrimSpace(lineage.ActiveSourceRef) != strings.TrimSpace(rec.CandidateSourceRef) {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: active lineage %q does not equal candidate source ref %q", lineage.ActiveSourceRef, rec.CandidateSourceRef)
	}
	if profileErr != nil {
		return candidatePackagePromotionAcceptanceEvidence{}, profileErr
	}
	if !ownerReviewApproved || !sourceLineageSwitched || !sourceLineageRolledBack || !sourceLineageRollForwarded {
		return candidatePackagePromotionAcceptanceEvidence{}, fmt.Errorf("candidate package promotion acceptance evidence: owner-review, switch, rollback, and roll-forward checkpoints are required")
	}
	reviewEvidenceRefs := candidatePackagePromotionAcceptanceReviewEvidenceRefs(rec.VerifierResultsJSON)
	evidenceRefs := candidatePackagePromotionAcceptanceStringSet(reviewEvidenceRefs...)
	evidenceRefs = candidatePackagePromotionAcceptanceAppendUnique(evidenceRefs, switchEvidenceRef, rollbackEvidenceRef, rollForwardEvidenceRef)
	return candidatePackagePromotionAcceptanceEvidence{
		ArtifactKind:                   "candidate_package_promotion_switch_acceptance_evidence",
		AcceptanceID:                   "candidate-package-local-acceptance-" + rec.AdoptionID,
		AcceptanceLevel:                "local-source-lineage-evidence",
		State:                          "accepted",
		EvidenceScope:                  "local_source_lineage",
		ReviewScope:                    "non-deployed-candidate-package-source-lineage",
		IntakeID:                       intake.IntakeID,
		AdoptionID:                     rec.AdoptionID,
		PackageID:                      pkg.PackageID,
		AppID:                          pkg.AppID,
		CandidatePackageID:             intake.CandidatePackageID,
		CandidatePackageManifestSHA256: intake.CandidatePackageManifestSHA256,
		SourceComputerID:               intake.SourceComputerID,
		SourceCandidateID:              intake.SourceCandidateID,
		TargetComputerID:               rec.TargetComputerID,
		TargetCandidateID:              rec.TargetCandidateID,
		TargetActiveSourceRefAtCutover: rec.TargetActiveSourceRefAtCutover,
		CandidateSourceRef:             rec.CandidateSourceRef,
		PreviousActiveRef:              rec.TargetActiveSourceRefAtCutover,
		CurrentAdoptionStatus:          string(rec.Status),
		OwnerReviewApproved:            ownerReviewApproved,
		SourceLineageSwitched:          sourceLineageSwitched,
		SourceLineageRolledBack:        sourceLineageRolledBack,
		SourceLineageRollForwarded:     sourceLineageRollForwarded,
		PackagePublication:             "blocked",
		DeployedPromotion:              "blocked",
		DeployedRouteMutation:          "blocked",
		PromotionLevel:                 "not_claimed",
		AuthSession:                    "unproven",
		Staging:                        "unproven",
		VMLifecycle:                    "blocked",
		RunAcceptanceRecord:            "not_created",
		Checkpoints:                    checkpoints,
		EvidenceRefs:                   evidenceRefs,
		BoundaryAssertions: map[string]string{
			"package_publication":     "blocked",
			"deployed_route_mutation": "blocked",
			"auth_session":            "unproven",
			"staging":                 "unproven",
			"vm_lifecycle":            "blocked",
			"run_acceptance_record":   "not_created",
			"promotion_level":         "not_claimed",
		},
		ResidualRisks: []string{
			"local-source-lineage-evidence is not deployed promotion-level acceptance",
			"deployed route registration, auth/session, staging identity, package publication, and VM lifecycle semantics remain unproven",
		},
		VerifierContractState: []candidatePackagePromotionAcceptanceContractState{
			{Name: "adoption_review_contract_ref", State: candidatePackagePromotionAcceptancePresent(profile, "adoption_review_contract_ref")},
			{Name: "adoption_contract_ref", State: candidatePackagePromotionAcceptancePresent(profile, "adoption_contract_ref")},
			{Name: "rollback_contract_ref", State: candidatePackagePromotionAcceptancePresent(profile, "rollback_contract_ref")},
		},
	}, nil
}

func (rt *Runtime) CandidatePackagePromotionReviewSurface(ctx context.Context, ownerID, intakeID, adoptionID string) (candidatePackagePromotionReviewSurface, error) {
	acceptance, err := rt.CandidatePackagePromotionAcceptanceEvidence(ctx, ownerID, intakeID, adoptionID)
	if err != nil {
		return candidatePackagePromotionReviewSurface{}, err
	}
	return candidatePackagePromotionReviewSurface{
		ArtifactKind:                   "candidate_package_adoption_promotion_review_surface",
		State:                          "reviewable",
		SurfaceScope:                   "product_visible_non_deployed",
		DeploymentState:                "non_deployed",
		ProductVisible:                 true,
		ReadOnly:                       true,
		ReviewScope:                    acceptance.ReviewScope,
		IntakeID:                       acceptance.IntakeID,
		AdoptionID:                     acceptance.AdoptionID,
		PackageID:                      acceptance.PackageID,
		AppID:                          acceptance.AppID,
		CandidatePackageID:             acceptance.CandidatePackageID,
		CandidatePackageManifestSHA256: acceptance.CandidatePackageManifestSHA256,
		SourceComputerID:               acceptance.SourceComputerID,
		SourceCandidateID:              acceptance.SourceCandidateID,
		TargetComputerID:               acceptance.TargetComputerID,
		TargetCandidateID:              acceptance.TargetCandidateID,
		TargetActiveSourceRefAtCutover: acceptance.TargetActiveSourceRefAtCutover,
		CandidateSourceRef:             acceptance.CandidateSourceRef,
		CurrentAdoptionStatus:          acceptance.CurrentAdoptionStatus,
		LocalAcceptanceID:              acceptance.AcceptanceID,
		LocalAcceptanceLevel:           acceptance.AcceptanceLevel,
		LocalAcceptanceState:           acceptance.State,
		PackagePublication:             "blocked",
		DeployedPromotion:              "blocked",
		DeployedRouteMutation:          "blocked",
		PromotionLevel:                 "not_claimed",
		AuthSession:                    "unproven",
		Staging:                        "unproven",
		VMLifecycle:                    "blocked",
		RunAcceptanceRecord:            "not_created",
		AppChangePackageMutation:       "not_created",
		AppAdoptionMutation:            "not_created",
		AllowedActions: []string{
			"review",
			"inspect",
			"prepare_activation_decision",
		},
		BlockedActions: []string{
			"publish_package",
			"deploy_route",
			"promote_product",
			"create_run_acceptance_record",
			"mutate_auth_session",
			"mutate_vm_lifecycle",
			"claim_staging_acceptance",
			"call_app_adoption_promote",
		},
		ActivationDecisionBoundary: candidatePackageActivationDecisionBoundary{
			State:                 "owner_decision_preparable",
			OwnerControlled:       true,
			RequiresAuthenticated: true,
			PreparedAction:        "prepare_activation_decision",
			NoMutation:            true,
			UsesAcceptanceID:      acceptance.AcceptanceID,
			NextBoundary:          "app_adoption_promotion_requires_separate_product_activation_contract",
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
		},
		AcceptanceEvidence: acceptance,
		BoundaryAssertions: map[string]string{
			"package_publication":      "blocked",
			"deployed_promotion":       "blocked",
			"deployed_route_mutation":  "blocked",
			"promotion_level":          "not_claimed",
			"run_acceptance_record":    "not_created",
			"auth_session":             "unproven",
			"staging":                  "unproven",
			"vm_lifecycle":             "blocked",
			"app_change_package_write": "not_created",
			"app_adoption_write":       "not_created",
		},
		ResidualRisks: []string{
			"product-visible review surface is non-deployed and local-route-harness scoped",
			"reviewability does not authorize package publication, deployed promotion, route mutation, run acceptance, auth/session, staging, or VM lifecycle claims",
		},
	}, nil
}

func (rt *Runtime) loadCandidatePackagePublicationDraftForAdoptionReview(ctx context.Context, ownerID, intakeID, packageID string) (types.CandidatePackageIntakeRecord, types.AppChangePackageRecord, string, string, error) {
	intake, err := rt.store.GetCandidatePackageIntake(ctx, ownerID, strings.TrimSpace(intakeID))
	if err != nil {
		return types.CandidatePackageIntakeRecord{}, types.AppChangePackageRecord{}, "", "", err
	}
	if intake.Status != types.CandidatePackageIntakeOwnerApproved || intake.OwnerReviewState != types.CandidatePackageOwnerReviewApproved || intake.OwnerReviewRequired {
		return intake, types.AppChangePackageRecord{}, "", "", fmt.Errorf("candidate package intake adoption review: intake %s is not owner-approved", intake.IntakeID)
	}
	if !intake.AdoptionReady {
		return intake, types.AppChangePackageRecord{}, "", "", fmt.Errorf("candidate package intake adoption review: intake %s is not adoption-ready", intake.IntakeID)
	}
	blockers, err := candidatePackageIntakeStringArray(intake.AdoptionBlockersJSON)
	if err != nil {
		return intake, types.AppChangePackageRecord{}, "", "", fmt.Errorf("candidate package intake adoption review: adoption_blockers_json is invalid: %w", err)
	}
	if len(blockers) != 0 {
		return intake, types.AppChangePackageRecord{}, "", "", fmt.Errorf("candidate package intake adoption review: intake %s still has adoption blockers", intake.IntakeID)
	}
	adoptionContractRef, rollbackContractRef, err := candidatePackageIntakeAdoptionBoundaryRefs(intake.AcceptanceJSON)
	if err != nil {
		return intake, types.AppChangePackageRecord{}, "", "", err
	}
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		packageID = strings.TrimSpace(intake.CandidatePackageID)
	}
	pkg, err := rt.store.GetAppChangePackageForViewer(ctx, ownerID, packageID)
	if err != nil {
		return intake, types.AppChangePackageRecord{}, "", "", fmt.Errorf("candidate package intake adoption review: publication draft not found")
	}
	if pkg.OwnerID != ownerID || pkg.Status != types.AppChangePackageDraft || pkg.Visibility != "private" || !candidatePackageDraftMatchesIntakeRecord(pkg, intake) {
		return intake, pkg, "", "", fmt.Errorf("candidate package intake adoption review: package %q is not a private draft for intake %s", pkg.PackageID, intake.IntakeID)
	}
	return intake, pkg, adoptionContractRef, rollbackContractRef, nil
}

func (rt *Runtime) candidatePackageAdoptionReviewForPackage(ctx context.Context, ownerID, packageID string) (types.AppAdoptionRecord, error) {
	adoptions, err := rt.store.ListAppAdoptions(ctx, ownerID, 500)
	if err != nil {
		return types.AppAdoptionRecord{}, err
	}
	for _, rec := range adoptions {
		if rec.PackageID == packageID {
			return rec, nil
		}
	}
	return types.AppAdoptionRecord{}, store.ErrNotFound
}

func candidatePackageAdoptionReviewVerifierResultsJSON(intake types.CandidatePackageIntakeRecord, pkg types.AppChangePackageRecord, adoptionReviewContractRef, adoptionContractRef, rollbackContractRef, state, evidenceRef string) json.RawMessage {
	result := map[string]any{
		"contract_id":                  "candidate-package-draft-owner-adoption-review",
		"status":                       state,
		"candidate_package_intake_id":  intake.IntakeID,
		"package_id":                   pkg.PackageID,
		"adoption_review_contract_ref": adoptionReviewContractRef,
		"adoption_contract_ref":        adoptionContractRef,
		"rollback_contract_ref":        rollbackContractRef,
		"package_publication":          "blocked",
		"promotion":                    "blocked",
		"deployed_route_mutation":      "blocked",
		"vm_lifecycle":                 "blocked",
	}
	if evidenceRef != "" {
		result["review_evidence_ref"] = evidenceRef
	}
	data, err := json.Marshal([]map[string]any{result})
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return data
}

func candidatePackageAppendAdoptionReviewDecisionJSON(raw json.RawMessage, decision, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "[]")
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		results = []map[string]any{}
	}
	result := map[string]any{
		"contract_id":             "candidate-package-draft-owner-adoption-review-decision",
		"status":                  decision,
		"package_publication":     "blocked",
		"promotion":               "blocked",
		"deployed_route_mutation": "blocked",
		"vm_lifecycle":            "blocked",
	}
	if evidenceRef != "" {
		result["review_evidence_ref"] = evidenceRef
	}
	results = append(results, result)
	data, err := json.Marshal(results)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return data
}

func candidatePackageAppendPromotionSwitchJSON(raw json.RawMessage, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "[]")
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		results = []map[string]any{}
	}
	result := map[string]any{
		"contract_id":             "candidate-package-source-lineage-switch",
		"status":                  "source_lineage_switched",
		"package_publication":     "blocked",
		"deployed_route_mutation": "blocked",
		"vm_lifecycle":            "blocked",
		"rollback_execution":      "blocked",
		"promotion_mode":          "source_lineage_only",
	}
	if evidenceRef != "" {
		result["switch_evidence_ref"] = evidenceRef
	}
	results = append(results, result)
	data, err := json.Marshal(results)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return data
}

func candidatePackageAdoptionReviewSwitchProfileMatches(raw json.RawMessage, activeSourceRef, adoptionContractRef, rollbackContractRef string) error {
	raw = rawJSONOrFallback(raw, "{}")
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		return fmt.Errorf("candidate package intake promotion switch: rollback profile is invalid")
	}
	if strings.TrimSpace(stringFromMap(profile, "previous_active_source_ref")) != strings.TrimSpace(activeSourceRef) {
		return fmt.Errorf("candidate package intake promotion switch: rollback profile does not match current active source ref")
	}
	if strings.TrimSpace(stringFromMap(profile, "lineage_ref_at_review")) != strings.TrimSpace(activeSourceRef) {
		return fmt.Errorf("candidate package intake promotion switch: adoption review lineage ref is stale")
	}
	if strings.TrimSpace(stringFromMap(profile, "adoption_review_contract_ref")) == "" {
		return fmt.Errorf("candidate package intake promotion switch: adoption_review_contract_ref is missing")
	}
	if strings.TrimSpace(stringFromMap(profile, "adoption_contract_ref")) != strings.TrimSpace(adoptionContractRef) {
		return fmt.Errorf("candidate package intake promotion switch: adoption_contract_ref does not match intake boundary")
	}
	if strings.TrimSpace(stringFromMap(profile, "rollback_contract_ref")) != strings.TrimSpace(rollbackContractRef) {
		return fmt.Errorf("candidate package intake promotion switch: rollback_contract_ref does not match intake boundary")
	}
	return nil
}

func candidatePackageAdoptionReviewSwitchRollbackProfileJSON(raw json.RawMessage, candidateSourceRef, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "{}")
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		profile = map[string]any{}
	}
	profile["source_lineage_switch_status"] = "source_lineage_switched"
	profile["source_lineage_switch_ref"] = strings.TrimSpace(candidateSourceRef)
	profile["package_publication"] = "blocked"
	profile["deployed_route_mutation"] = "blocked"
	profile["vm_lifecycle"] = "blocked"
	profile["rollback_execution"] = "blocked"
	profile["promotion_mode"] = "source_lineage_only"
	if evidenceRef != "" {
		profile["switch_evidence_ref"] = evidenceRef
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func candidatePackageAppendPromotionSwitchRollbackJSON(raw json.RawMessage, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "[]")
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		results = []map[string]any{}
	}
	result := map[string]any{
		"contract_id":             "candidate-package-source-lineage-switch-rollback",
		"status":                  "source_lineage_rolled_back",
		"package_publication":     "blocked",
		"deployed_route_mutation": "blocked",
		"vm_lifecycle":            "blocked",
		"rollback_mode":           "source_lineage_only",
	}
	if evidenceRef != "" {
		result["rollback_evidence_ref"] = evidenceRef
	}
	results = append(results, result)
	data, err := json.Marshal(results)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return data
}

func candidatePackageAppendPromotionSwitchRollForwardJSON(raw json.RawMessage, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "[]")
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		results = []map[string]any{}
	}
	result := map[string]any{
		"contract_id":             "candidate-package-source-lineage-switch-roll-forward",
		"status":                  "source_lineage_switched",
		"package_publication":     "blocked",
		"deployed_route_mutation": "blocked",
		"vm_lifecycle":            "blocked",
		"rollback_execution":      "blocked",
		"promotion_mode":          "source_lineage_only_roll_forward",
	}
	if evidenceRef != "" {
		result["roll_forward_evidence_ref"] = evidenceRef
	}
	results = append(results, result)
	data, err := json.Marshal(results)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return data
}

func candidatePackageAdoptionReviewRollbackProfileForRollback(raw json.RawMessage, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef string) (string, error) {
	profile, err := candidatePackageAdoptionReviewSwitchProfileForResolution(raw, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef, "source_lineage_switched")
	if err != nil {
		return "", fmt.Errorf("candidate package intake promotion switch rollback: %w", err)
	}
	return strings.TrimSpace(stringFromMap(profile, "previous_route_profile")), nil
}

func candidatePackageAdoptionReviewRollbackProfileForRollForward(raw json.RawMessage, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef string) (map[string]any, error) {
	profile, err := candidatePackageAdoptionReviewSwitchProfileForResolution(raw, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef, "source_lineage_rolled_back")
	if err != nil {
		return nil, fmt.Errorf("candidate package intake promotion switch roll-forward: %w", err)
	}
	return profile, nil
}

func candidatePackageAdoptionReviewSwitchProfileForResolution(raw json.RawMessage, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef, wantStatus string) (map[string]any, error) {
	raw = rawJSONOrFallback(raw, "{}")
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		return nil, fmt.Errorf("rollback profile is invalid")
	}
	if strings.TrimSpace(stringFromMap(profile, "previous_active_source_ref")) != strings.TrimSpace(previousActiveSourceRef) {
		return nil, fmt.Errorf("rollback profile does not match previous active source ref")
	}
	if strings.TrimSpace(stringFromMap(profile, "lineage_ref_at_review")) != strings.TrimSpace(previousActiveSourceRef) {
		return nil, fmt.Errorf("adoption review lineage ref is stale")
	}
	if strings.TrimSpace(stringFromMap(profile, "source_lineage_switch_ref")) != strings.TrimSpace(candidateSourceRef) {
		return nil, fmt.Errorf("source lineage switch ref does not match candidate source ref")
	}
	if strings.TrimSpace(stringFromMap(profile, "source_lineage_switch_status")) != strings.TrimSpace(wantStatus) {
		return nil, fmt.Errorf("source lineage switch status is not %s", wantStatus)
	}
	if strings.TrimSpace(stringFromMap(profile, "adoption_review_contract_ref")) == "" {
		return nil, fmt.Errorf("adoption_review_contract_ref is missing")
	}
	if strings.TrimSpace(stringFromMap(profile, "adoption_contract_ref")) != strings.TrimSpace(adoptionContractRef) {
		return nil, fmt.Errorf("adoption_contract_ref does not match intake boundary")
	}
	if strings.TrimSpace(stringFromMap(profile, "rollback_contract_ref")) != strings.TrimSpace(rollbackContractRef) {
		return nil, fmt.Errorf("rollback_contract_ref does not match intake boundary")
	}
	return profile, nil
}

func candidatePackagePromotionAcceptanceProfile(raw json.RawMessage, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef string) (map[string]any, error) {
	profile, err := candidatePackageAdoptionReviewSwitchProfileForResolution(raw, previousActiveSourceRef, candidateSourceRef, adoptionContractRef, rollbackContractRef, "source_lineage_switched")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(stringFromMap(profile, "source_lineage_restored_ref")) != strings.TrimSpace(previousActiveSourceRef) {
		return nil, fmt.Errorf("source lineage rollback ref does not match previous active source ref")
	}
	if strings.TrimSpace(stringFromMap(profile, "rollback_evidence_ref")) == "" {
		return nil, fmt.Errorf("rollback evidence ref is required")
	}
	if strings.TrimSpace(stringFromMap(profile, "roll_forward_evidence_ref")) == "" {
		return nil, fmt.Errorf("roll-forward evidence ref is required")
	}
	return profile, nil
}

func candidatePackagePromotionAcceptanceCheckpointState(raw json.RawMessage, contractID, status string) string {
	raw = rawJSONOrFallback(raw, "[]")
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return "missing"
	}
	for _, result := range results {
		if strings.TrimSpace(stringFromMap(result, "contract_id")) != contractID {
			continue
		}
		if strings.TrimSpace(stringFromMap(result, "status")) == status {
			return "verified"
		}
	}
	return "missing"
}

func candidatePackagePromotionAcceptanceReviewEvidenceRefs(raw json.RawMessage) []string {
	raw = rawJSONOrFallback(raw, "[]")
	var results []map[string]any
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil
	}
	refs := make([]string, 0, len(results))
	for _, result := range results {
		for key, value := range result {
			if !strings.HasSuffix(key, "_evidence_ref") {
				continue
			}
			ref := strings.TrimSpace(fmt.Sprint(value))
			if ref != "" {
				refs = candidatePackagePromotionAcceptanceAppendUnique(refs, ref)
			}
		}
	}
	return refs
}

func candidatePackagePromotionAcceptanceStringSet(values ...string) []string {
	out := make([]string, 0, len(values))
	return candidatePackagePromotionAcceptanceAppendUnique(out, values...)
}

func candidatePackagePromotionAcceptanceAppendUnique(values []string, next ...string) []string {
	seen := make(map[string]bool, len(values)+len(next))
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, value := range next {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func candidatePackagePromotionAcceptancePresent(profile map[string]any, key string) string {
	if strings.TrimSpace(stringFromMap(profile, key)) == "" {
		return "missing"
	}
	return "present"
}

func candidatePackageAdoptionReviewSwitchRolledBackProfileJSON(raw json.RawMessage, restoredSourceRef, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "{}")
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		profile = map[string]any{}
	}
	profile["source_lineage_switch_status"] = "source_lineage_rolled_back"
	profile["source_lineage_restored_ref"] = strings.TrimSpace(restoredSourceRef)
	profile["package_publication"] = "blocked"
	profile["deployed_route_mutation"] = "blocked"
	profile["vm_lifecycle"] = "blocked"
	profile["rollback_mode"] = "source_lineage_only"
	if evidenceRef != "" {
		profile["rollback_evidence_ref"] = evidenceRef
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func candidatePackageAdoptionReviewSwitchRolledForwardProfileJSON(raw json.RawMessage, candidateSourceRef, evidenceRef string) json.RawMessage {
	raw = rawJSONOrFallback(raw, "{}")
	var profile map[string]any
	if err := json.Unmarshal(raw, &profile); err != nil {
		profile = map[string]any{}
	}
	profile["source_lineage_switch_status"] = "source_lineage_switched"
	profile["source_lineage_switch_ref"] = strings.TrimSpace(candidateSourceRef)
	profile["package_publication"] = "blocked"
	profile["deployed_route_mutation"] = "blocked"
	profile["vm_lifecycle"] = "blocked"
	profile["rollback_execution"] = "blocked"
	profile["promotion_mode"] = "source_lineage_only_roll_forward"
	if evidenceRef != "" {
		profile["roll_forward_evidence_ref"] = evidenceRef
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func candidatePackageAdoptionReviewRollbackProfileJSON(lineage types.ComputerSourceLineageRecord, adoptionReviewContractRef, adoptionContractRef, rollbackContractRef string) json.RawMessage {
	data, err := json.Marshal(map[string]any{
		"previous_active_source_ref":      lineage.ActiveSourceRef,
		"previous_runtime_digest":         lineage.RuntimeDigest,
		"previous_ui_digest":              lineage.UIDigest,
		"previous_route_profile":          lineage.RouteProfile,
		"lineage_ref_at_review":           lineage.ActiveSourceRef,
		"adoption_review_contract_ref":    adoptionReviewContractRef,
		"adoption_contract_ref":           adoptionContractRef,
		"rollback_contract_ref":           rollbackContractRef,
		"package_publication":             "blocked",
		"promotion":                       "blocked",
		"deployed_route_mutation":         "blocked",
		"vm_lifecycle":                    "blocked",
		"rollback_execution":              "blocked",
		"requires_verified_promotion_ref": true,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

func candidatePackageIntakeWithAdoptionBoundary(raw json.RawMessage, adoptionContractRef, rollbackContractRef string) (json.RawMessage, error) {
	raw = rawJSONOrFallback(raw, "{}")
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("candidate package intake adoption boundary: acceptance_json is invalid: %w", err)
	}
	if envelope == nil {
		envelope = map[string]any{}
	}
	envelope["adoption_rollback_boundary"] = map[string]any{
		"status":                            "bound",
		"adoption_contract_ref":             adoptionContractRef,
		"rollback_contract_ref":             rollbackContractRef,
		"direct_app_change_package_publish": "blocked",
		"app_adoption_creation":             "blocked",
		"promotion":                         "blocked",
		"deployed_route_mutation":           "blocked",
		"vm_lifecycle":                      "blocked",
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("candidate package intake adoption boundary: marshal acceptance_json: %w", err)
	}
	return data, nil
}

func candidatePackageIntakeAdoptionBoundaryRefs(raw json.RawMessage) (string, string, error) {
	raw = rawJSONOrFallback(raw, "{}")
	var envelope map[string]any
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", "", fmt.Errorf("candidate package intake publication draft: acceptance_json is invalid: %w", err)
	}
	boundary, ok := envelope["adoption_rollback_boundary"].(map[string]any)
	if !ok {
		return "", "", fmt.Errorf("candidate package intake publication draft: adoption_rollback_boundary is required")
	}
	status, _ := boundary["status"].(string)
	if strings.TrimSpace(status) != "bound" {
		return "", "", fmt.Errorf("candidate package intake publication draft: adoption_rollback_boundary is not bound")
	}
	adoptionContractRef, _ := boundary["adoption_contract_ref"].(string)
	rollbackContractRef, _ := boundary["rollback_contract_ref"].(string)
	adoptionContractRef = strings.TrimSpace(adoptionContractRef)
	rollbackContractRef = strings.TrimSpace(rollbackContractRef)
	if adoptionContractRef == "" || rollbackContractRef == "" {
		return "", "", fmt.Errorf("candidate package intake publication draft: adoption and rollback contract refs are required")
	}
	return adoptionContractRef, rollbackContractRef, nil
}

func candidatePackageDraftMatchesIntake(rec types.AppChangePackageRecord, intakeID string) bool {
	var manifest map[string]any
	if err := json.Unmarshal(rawJSONOrFallback(rec.ManifestJSON, "{}"), &manifest); err != nil {
		return false
	}
	kind, _ := manifest["kind"].(string)
	manifestIntakeID, _ := manifest["candidate_package_intake_id"].(string)
	return strings.TrimSpace(kind) == "candidate_package_publication_draft" && strings.TrimSpace(manifestIntakeID) == strings.TrimSpace(intakeID)
}

func candidatePackageDraftMatchesIntakeRecord(rec types.AppChangePackageRecord, intake types.CandidatePackageIntakeRecord) bool {
	var manifest map[string]any
	if err := json.Unmarshal(rawJSONOrFallback(rec.ManifestJSON, "{}"), &manifest); err != nil {
		return false
	}
	if kind, _ := manifest["kind"].(string); strings.TrimSpace(kind) != "candidate_package_publication_draft" {
		return false
	}
	for field, want := range map[string]string{
		"candidate_package_intake_id":       intake.IntakeID,
		"candidate_package_id":              intake.CandidatePackageID,
		"candidate_package_manifest_sha256": intake.CandidatePackageManifestSHA256,
		"source_computer_id":                intake.SourceComputerID,
		"source_candidate_id":               intake.SourceCandidateID,
		"candidate_source_ref":              intake.CandidateSourceRef,
	} {
		got, _ := manifest[field].(string)
		if strings.TrimSpace(got) != strings.TrimSpace(want) {
			return false
		}
	}
	return true
}

func candidatePackagePublicationDraftContractsJSON(raw json.RawMessage, publicationContractRef, adoptionContractRef, rollbackContractRef string) (json.RawMessage, error) {
	raw = rawJSONOrFallback(raw, "[]")
	var contracts []any
	if err := json.Unmarshal(raw, &contracts); err != nil {
		return nil, fmt.Errorf("candidate package intake publication draft: verifier_contracts_json is invalid: %w", err)
	}
	contracts = append(contracts,
		map[string]any{
			"name":         "candidate-package-publication-draft",
			"state":        "draft_only",
			"contract_ref": publicationContractRef,
			"publish":      "blocked",
		},
		map[string]any{
			"name":         "candidate-package-adoption-boundary",
			"state":        "bound",
			"contract_ref": adoptionContractRef,
		},
		map[string]any{
			"name":         "candidate-package-rollback-boundary",
			"state":        "bound",
			"contract_ref": rollbackContractRef,
		},
	)
	data, err := json.Marshal(contracts)
	if err != nil {
		return nil, fmt.Errorf("candidate package intake publication draft: marshal verifier_contracts_json: %w", err)
	}
	return data, nil
}

func candidatePackagePublicationDraftProvenanceJSON(raw json.RawMessage, draftEvidenceRef, intakeID string) (json.RawMessage, error) {
	refs, err := candidatePackageIntakeStringArray(raw)
	if err != nil {
		return nil, fmt.Errorf("candidate package intake publication draft: evidence_refs_json is invalid: %w", err)
	}
	refs = append(refs, "candidate-package-intake:"+strings.TrimSpace(intakeID))
	if draftEvidenceRef != "" {
		refs = append(refs, draftEvidenceRef)
	}
	return candidatePackageIntakeStringArrayJSON(refs), nil
}

func candidatePackageIntakeStringArray(raw json.RawMessage) ([]string, error) {
	raw = rawJSONOrFallback(raw, "[]")
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func candidatePackageIntakeStringArrayJSON(values []string, required ...string) json.RawMessage {
	out := make([]string, 0, len(values)+len(required))
	seen := map[string]bool{}
	for _, value := range append(values, required...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	if len(out) == 0 {
		out = append(out, required...)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return data
}

func candidatePackageIntakeRemoveBlockers(values []string, remove ...string) []string {
	removed := map[string]bool{}
	for _, value := range remove {
		removed[value] = true
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || removed[value] {
			continue
		}
		out = append(out, value)
	}
	return out
}

func (rt *Runtime) GetCandidatePackageIntake(ctx context.Context, ownerID, intakeID string) (types.CandidatePackageIntakeRecord, error) {
	if rt == nil || rt.store == nil {
		return types.CandidatePackageIntakeRecord{}, fmt.Errorf("candidate package intake: runtime store is unavailable")
	}
	return rt.store.GetCandidatePackageIntake(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(intakeID))
}

func (rt *Runtime) ListCandidatePackageIntakes(ctx context.Context, ownerID string, limit int) ([]types.CandidatePackageIntakeRecord, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("candidate package intake: runtime store is unavailable")
	}
	return rt.store.ListCandidatePackageIntakes(ctx, strings.TrimSpace(ownerID), limit)
}
