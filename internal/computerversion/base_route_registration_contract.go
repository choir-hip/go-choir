package computerversion

import (
	"fmt"
	"strings"
)

const BaseRouteRegistrationReadinessContractKind = "base_route_registration_readiness_contract"

const BaseRouteRegistrationReadinessBoundary = "local_base_route_registration_readiness_without_deployment"

const BaseRouteRegistrationReadinessStatusBlocked = "blocked_until_red_route_registration_ceremony"

const BaseRouteRegistrationPrerequisiteAuthSessionScope = "auth_session_scope_review"
const BaseRouteRegistrationPrerequisiteDeployedServiceRegistration = "deployed_service_registration_review"
const BaseRouteRegistrationPrerequisiteStagingBuildIdentity = "staging_build_identity_review"
const BaseRouteRegistrationPrerequisiteRollbackRouteRevert = "rollback_route_revert_review"
const BaseRouteRegistrationPrerequisiteProductionStateBoundary = "production_state_boundary_review"

// LocalBaseProductPathHarnessEvidence records local-only evidence that Base API
// routes can be mounted with explicit journal/blob/auth paths and then observed
// through computerversion tooling. It is not deployed route authorization.
type LocalBaseProductPathHarnessEvidence struct {
	Version                    ComputerVersion `json:"version"`
	HarnessEvidenceRef         string          `json:"harness_evidence_ref"`
	ObservationEvidenceRef     string          `json:"observation_evidence_ref"`
	ComparisonEvidenceRef      string          `json:"comparison_evidence_ref"`
	ExplicitStatePaths         bool            `json:"explicit_state_paths"`
	AuthBacked                 bool            `json:"auth_backed"`
	LocalRouteRegistered       bool            `json:"local_route_registered"`
	PersistedObservationSet    bool            `json:"persisted_observation_set"`
	SeededMismatchFailed       bool            `json:"seeded_mismatch_failed"`
	DeployedRouteRegistered    bool            `json:"deployed_route_registered"`
	ProductionAuthTouched      bool            `json:"production_auth_touched"`
	StagingClaimed             bool            `json:"staging_claimed"`
	ProductionStateMutated     bool            `json:"production_state_mutated"`
	VMLifecycleTouched         bool            `json:"vm_lifecycle_touched"`
	PromotionRollbackTouched   bool            `json:"promotion_rollback_touched"`
	RunAcceptanceRecordTouched bool            `json:"run_acceptance_record_touched"`
}

// BaseRouteRegistrationReadinessEvidence records the review refs required before
// local Base harness evidence can be considered for deployed route registration.
// It names blockers; it does not register a deployed route.
type BaseRouteRegistrationReadinessEvidence struct {
	Version                    ComputerVersion `json:"version"`
	HarnessEvidenceRef         string          `json:"harness_evidence_ref"`
	ObservationEvidenceRef     string          `json:"observation_evidence_ref"`
	ComparisonEvidenceRef      string          `json:"comparison_evidence_ref"`
	RoutePrefix                string          `json:"route_prefix"`
	RequiredScopes             []string        `json:"required_scopes"`
	RequiredPrerequisiteRefs   []string        `json:"required_prerequisite_refs"`
	RollbackPlanRef            string          `json:"rollback_plan_ref"`
	DeployedRouteRegistered    bool            `json:"deployed_route_registered"`
	ProductionAuthTouched      bool            `json:"production_auth_touched"`
	StagingClaimed             bool            `json:"staging_claimed"`
	ProductionStateMutated     bool            `json:"production_state_mutated"`
	VMLifecycleTouched         bool            `json:"vm_lifecycle_touched"`
	PromotionRollbackTouched   bool            `json:"promotion_rollback_touched"`
	RunAcceptanceRecordTouched bool            `json:"run_acceptance_record_touched"`
	RouteRegistrationAllowed   bool            `json:"route_registration_allowed"`
	NoMutation                 bool            `json:"no_mutation"`
}

// BaseRouteRegistrationReadinessContract turns local Base product-path evidence
// into an explicit blocked contract for any future deployed route-registration
// ceremony. The contract is local/readiness evidence only.
type BaseRouteRegistrationReadinessContract struct {
	Kind                       string          `json:"kind"`
	Version                    ComputerVersion `json:"version"`
	HarnessEvidenceRef         string          `json:"harness_evidence_ref"`
	ObservationEvidenceRef     string          `json:"observation_evidence_ref"`
	ComparisonEvidenceRef      string          `json:"comparison_evidence_ref"`
	RoutePrefix                string          `json:"route_prefix"`
	ReadinessBoundary          string          `json:"readiness_boundary"`
	RequiredScopes             []string        `json:"required_scopes"`
	RequiredPrerequisiteRefs   []string        `json:"required_prerequisite_refs"`
	RollbackPlanRef            string          `json:"rollback_plan_ref"`
	ReadinessStatus            string          `json:"readiness_status"`
	DeployedRouteRegistered    bool            `json:"deployed_route_registered"`
	ProductionAuthTouched      bool            `json:"production_auth_touched"`
	StagingClaimed             bool            `json:"staging_claimed"`
	ProductionStateMutated     bool            `json:"production_state_mutated"`
	VMLifecycleTouched         bool            `json:"vm_lifecycle_touched"`
	PromotionRollbackTouched   bool            `json:"promotion_rollback_touched"`
	RunAcceptanceRecordTouched bool            `json:"run_acceptance_record_touched"`
	RouteRegistrationAllowed   bool            `json:"route_registration_allowed"`
	NoMutation                 bool            `json:"no_mutation"`
	BlockedPrerequisites       []string        `json:"blocked_prerequisites"`
}

// BuildBaseRouteRegistrationReadinessContract binds local harness evidence to a
// blocked deployed-route readiness contract. It never registers routes, touches
// production auth/session, claims staging, or mutates product state.
func BuildBaseRouteRegistrationReadinessContract(harness LocalBaseProductPathHarnessEvidence, evidence BaseRouteRegistrationReadinessEvidence) (BaseRouteRegistrationReadinessContract, error) {
	if err := validateLocalBaseProductPathHarnessEvidence(harness); err != nil {
		return BaseRouteRegistrationReadinessContract{}, err
	}
	if err := validateBaseRouteRegistrationEvidenceIdentity(harness, evidence); err != nil {
		return BaseRouteRegistrationReadinessContract{}, err
	}
	if err := validateBaseRouteRegistrationEvidenceRefs(evidence); err != nil {
		return BaseRouteRegistrationReadinessContract{}, err
	}
	if err := validateBaseRouteRegistrationEvidenceNoMutation(evidence); err != nil {
		return BaseRouteRegistrationReadinessContract{}, err
	}

	return BaseRouteRegistrationReadinessContract{
		Kind:                       BaseRouteRegistrationReadinessContractKind,
		Version:                    harness.Version,
		HarnessEvidenceRef:         strings.TrimSpace(harness.HarnessEvidenceRef),
		ObservationEvidenceRef:     strings.TrimSpace(harness.ObservationEvidenceRef),
		ComparisonEvidenceRef:      strings.TrimSpace(harness.ComparisonEvidenceRef),
		RoutePrefix:                strings.TrimSpace(evidence.RoutePrefix),
		ReadinessBoundary:          BaseRouteRegistrationReadinessBoundary,
		RequiredScopes:             canonicalStrings(evidence.RequiredScopes),
		RequiredPrerequisiteRefs:   canonicalStrings(evidence.RequiredPrerequisiteRefs),
		RollbackPlanRef:            strings.TrimSpace(evidence.RollbackPlanRef),
		ReadinessStatus:            BaseRouteRegistrationReadinessStatusBlocked,
		DeployedRouteRegistered:    false,
		ProductionAuthTouched:      false,
		StagingClaimed:             false,
		ProductionStateMutated:     false,
		VMLifecycleTouched:         false,
		PromotionRollbackTouched:   false,
		RunAcceptanceRecordTouched: false,
		RouteRegistrationAllowed:   false,
		NoMutation:                 true,
		BlockedPrerequisites:       baseRouteRegistrationBlockedPrerequisites(),
	}, nil
}

func validateLocalBaseProductPathHarnessEvidence(harness LocalBaseProductPathHarnessEvidence) error {
	if !harness.Version.Valid() {
		return fmt.Errorf("base route registration readiness: harness version is invalid")
	}
	if strings.TrimSpace(harness.HarnessEvidenceRef) == "" {
		return fmt.Errorf("base route registration readiness: harness evidence ref is required")
	}
	if strings.TrimSpace(harness.ObservationEvidenceRef) == "" {
		return fmt.Errorf("base route registration readiness: observation evidence ref is required")
	}
	if strings.TrimSpace(harness.ComparisonEvidenceRef) == "" {
		return fmt.Errorf("base route registration readiness: comparison evidence ref is required")
	}
	if !harness.ExplicitStatePaths {
		return fmt.Errorf("base route registration readiness: harness must prove explicit state paths")
	}
	if !harness.AuthBacked {
		return fmt.Errorf("base route registration readiness: harness must prove auth-backed local routes")
	}
	if !harness.LocalRouteRegistered {
		return fmt.Errorf("base route registration readiness: harness must prove local route registration")
	}
	if !harness.PersistedObservationSet {
		return fmt.Errorf("base route registration readiness: harness must prove persisted observation set")
	}
	if !harness.SeededMismatchFailed {
		return fmt.Errorf("base route registration readiness: harness must prove seeded mismatch failure")
	}
	return validateBaseRouteRegistrationUnsafeClaims("harness", harness.DeployedRouteRegistered, harness.ProductionAuthTouched, harness.StagingClaimed, harness.ProductionStateMutated, harness.VMLifecycleTouched, harness.PromotionRollbackTouched, harness.RunAcceptanceRecordTouched)
}

func validateBaseRouteRegistrationEvidenceIdentity(harness LocalBaseProductPathHarnessEvidence, evidence BaseRouteRegistrationReadinessEvidence) error {
	if evidence.Version != harness.Version {
		return fmt.Errorf("base route registration readiness: evidence version does not match harness")
	}
	if strings.TrimSpace(evidence.HarnessEvidenceRef) != strings.TrimSpace(harness.HarnessEvidenceRef) {
		return fmt.Errorf("base route registration readiness: evidence harness ref does not match harness")
	}
	if strings.TrimSpace(evidence.ObservationEvidenceRef) != strings.TrimSpace(harness.ObservationEvidenceRef) {
		return fmt.Errorf("base route registration readiness: evidence observation ref does not match harness")
	}
	if strings.TrimSpace(evidence.ComparisonEvidenceRef) != strings.TrimSpace(harness.ComparisonEvidenceRef) {
		return fmt.Errorf("base route registration readiness: evidence comparison ref does not match harness")
	}
	return nil
}

func validateBaseRouteRegistrationEvidenceRefs(evidence BaseRouteRegistrationReadinessEvidence) error {
	if strings.TrimSpace(evidence.RoutePrefix) != "/api/base/" {
		return fmt.Errorf("base route registration readiness: route prefix %q is not /api/base/", evidence.RoutePrefix)
	}
	if !sameCanonicalStrings(evidence.RequiredScopes, []string{"read:base", "write:base"}) {
		return fmt.Errorf("base route registration readiness: required scopes must be read:base and write:base")
	}
	refs := canonicalStrings(evidence.RequiredPrerequisiteRefs)
	if len(refs) == 0 {
		return fmt.Errorf("base route registration readiness: prerequisite refs are required")
	}
	for _, prerequisite := range baseRouteRegistrationBlockedPrerequisites() {
		if !stringSliceContains(refs, prerequisite) {
			return fmt.Errorf("base route registration readiness: prerequisite %q is missing", prerequisite)
		}
	}
	if strings.TrimSpace(evidence.RollbackPlanRef) == "" {
		return fmt.Errorf("base route registration readiness: rollback plan ref is required")
	}
	return nil
}

func validateBaseRouteRegistrationEvidenceNoMutation(evidence BaseRouteRegistrationReadinessEvidence) error {
	if err := validateBaseRouteRegistrationUnsafeClaims("evidence", evidence.DeployedRouteRegistered, evidence.ProductionAuthTouched, evidence.StagingClaimed, evidence.ProductionStateMutated, evidence.VMLifecycleTouched, evidence.PromotionRollbackTouched, evidence.RunAcceptanceRecordTouched); err != nil {
		return err
	}
	if evidence.RouteRegistrationAllowed {
		return fmt.Errorf("base route registration readiness: evidence cannot allow route registration")
	}
	if !evidence.NoMutation {
		return fmt.Errorf("base route registration readiness: evidence must be no-mutation")
	}
	return nil
}

func validateBaseRouteRegistrationUnsafeClaims(label string, deployedRouteRegistered, productionAuthTouched, stagingClaimed, productionStateMutated, vmLifecycleTouched, promotionRollbackTouched, runAcceptanceRecordTouched bool) error {
	switch {
	case deployedRouteRegistered:
		return fmt.Errorf("base route registration readiness: %s cannot register deployed routes", label)
	case productionAuthTouched:
		return fmt.Errorf("base route registration readiness: %s cannot touch production auth/session", label)
	case stagingClaimed:
		return fmt.Errorf("base route registration readiness: %s cannot claim staging", label)
	case productionStateMutated:
		return fmt.Errorf("base route registration readiness: %s cannot mutate production state", label)
	case vmLifecycleTouched:
		return fmt.Errorf("base route registration readiness: %s cannot touch VM lifecycle", label)
	case promotionRollbackTouched:
		return fmt.Errorf("base route registration readiness: %s cannot touch promotion/rollback", label)
	case runAcceptanceRecordTouched:
		return fmt.Errorf("base route registration readiness: %s cannot touch run acceptance records", label)
	default:
		return nil
	}
}

func baseRouteRegistrationBlockedPrerequisites() []string {
	return []string{
		BaseRouteRegistrationPrerequisiteAuthSessionScope,
		BaseRouteRegistrationPrerequisiteDeployedServiceRegistration,
		BaseRouteRegistrationPrerequisiteStagingBuildIdentity,
		BaseRouteRegistrationPrerequisiteRollbackRouteRevert,
		BaseRouteRegistrationPrerequisiteProductionStateBoundary,
	}
}

const BaseRouteRegistrationAuthorityReviewContractKind = "base_route_registration_authority_review_contract"

const BaseRouteRegistrationAuthorityReviewBoundary = "base_route_registration_authority_review_without_route_registration"

const BaseRouteRegistrationAuthorityReviewStatusRecorded = "authority_review_recorded_without_red_authorization"

const BaseRouteRegistrationAuthorityReviewItemRedCeremonyScope = "base_route_red_ceremony_scope_review"
const BaseRouteRegistrationAuthorityReviewItemAuthSessionScope = "base_route_auth_session_scope_review"
const BaseRouteRegistrationAuthorityReviewItemServiceRouting = "base_route_deployed_service_routing_review"
const BaseRouteRegistrationAuthorityReviewItemStagingIdentity = "base_route_staging_identity_review"
const BaseRouteRegistrationAuthorityReviewItemRollbackRehearsal = "base_route_rollback_rehearsal_review"
const BaseRouteRegistrationAuthorityReviewItemProductionStateBoundary = "base_route_production_state_boundary_review"

// BaseRouteRegistrationAuthorityReviewEvidence records read-only owner/reviewer
// attention for the blocked route-registration readiness contract. It is not
// red ceremony approval and cannot authorize deployed route registration.
type BaseRouteRegistrationAuthorityReviewEvidence struct {
	Version                     ComputerVersion `json:"version"`
	HarnessEvidenceRef          string          `json:"harness_evidence_ref"`
	ObservationEvidenceRef      string          `json:"observation_evidence_ref"`
	ComparisonEvidenceRef       string          `json:"comparison_evidence_ref"`
	RoutePrefix                 string          `json:"route_prefix"`
	ReadinessContractRef        string          `json:"readiness_contract_ref"`
	OwnerAuthorizationRef       string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef    string          `json:"reviewer_authorization_ref"`
	RedCeremonyPlanRef          string          `json:"red_ceremony_plan_ref"`
	RequiredPrerequisiteRefs    []string        `json:"required_prerequisite_refs"`
	ChecklistItemRefs           []string        `json:"checklist_item_refs"`
	ReviewerFindingRefs         []string        `json:"reviewer_finding_refs"`
	OpenQuestionRefs            []string        `json:"open_question_refs,omitempty"`
	ReviewReportRef             string          `json:"review_report_ref"`
	RollbackPlanRef             string          `json:"rollback_plan_ref"`
	DeployedRouteRegistered     bool            `json:"deployed_route_registered"`
	ProductionAuthTouched       bool            `json:"production_auth_touched"`
	StagingClaimed              bool            `json:"staging_claimed"`
	ProductionStateMutated      bool            `json:"production_state_mutated"`
	VMLifecycleTouched          bool            `json:"vm_lifecycle_touched"`
	PromotionRollbackTouched    bool            `json:"promotion_rollback_touched"`
	RunAcceptanceRecordTouched  bool            `json:"run_acceptance_record_touched"`
	RouteRegistrationAuthorized bool            `json:"route_registration_authorized"`
	RedCeremonyOpened           bool            `json:"red_ceremony_opened"`
	RedCeremonyApproved         bool            `json:"red_ceremony_approved"`
	NoMutation                  bool            `json:"no_mutation"`
}

// BaseRouteRegistrationAuthorityReviewContract binds read-only owner/reviewer
// review refs to the blocked readiness contract. It records review coverage and
// questions but leaves route registration and red ceremony blocked.
type BaseRouteRegistrationAuthorityReviewContract struct {
	Kind                        string          `json:"kind"`
	Version                     ComputerVersion `json:"version"`
	HarnessEvidenceRef          string          `json:"harness_evidence_ref"`
	ObservationEvidenceRef      string          `json:"observation_evidence_ref"`
	ComparisonEvidenceRef       string          `json:"comparison_evidence_ref"`
	RoutePrefix                 string          `json:"route_prefix"`
	ReadinessContractRef        string          `json:"readiness_contract_ref"`
	AuthorityReviewBoundary     string          `json:"authority_review_boundary"`
	OwnerAuthorizationRef       string          `json:"owner_authorization_ref"`
	ReviewerAuthorizationRef    string          `json:"reviewer_authorization_ref"`
	RedCeremonyPlanRef          string          `json:"red_ceremony_plan_ref"`
	RequiredPrerequisiteRefs    []string        `json:"required_prerequisite_refs"`
	ChecklistItemRefs           []string        `json:"checklist_item_refs"`
	ReviewerFindingRefs         []string        `json:"reviewer_finding_refs"`
	OpenQuestionRefs            []string        `json:"open_question_refs,omitempty"`
	ReviewReportRef             string          `json:"review_report_ref"`
	RollbackPlanRef             string          `json:"rollback_plan_ref"`
	AuthorityReviewStatus       string          `json:"authority_review_status"`
	DeployedRouteRegistered     bool            `json:"deployed_route_registered"`
	ProductionAuthTouched       bool            `json:"production_auth_touched"`
	StagingClaimed              bool            `json:"staging_claimed"`
	ProductionStateMutated      bool            `json:"production_state_mutated"`
	VMLifecycleTouched          bool            `json:"vm_lifecycle_touched"`
	PromotionRollbackTouched    bool            `json:"promotion_rollback_touched"`
	RunAcceptanceRecordTouched  bool            `json:"run_acceptance_record_touched"`
	RouteRegistrationAuthorized bool            `json:"route_registration_authorized"`
	RedCeremonyOpened           bool            `json:"red_ceremony_opened"`
	RedCeremonyApproved         bool            `json:"red_ceremony_approved"`
	NoMutation                  bool            `json:"no_mutation"`
	BlockedPrerequisites        []string        `json:"blocked_prerequisites"`
}

// BuildBaseRouteRegistrationAuthorityReviewContract records read-only authority
// review coverage for a blocked Base route-registration readiness contract. It
// never opens red ceremony and never registers deployed routes.
func BuildBaseRouteRegistrationAuthorityReviewContract(readiness BaseRouteRegistrationReadinessContract, evidence BaseRouteRegistrationAuthorityReviewEvidence) (BaseRouteRegistrationAuthorityReviewContract, error) {
	if err := validateBaseRouteRegistrationReadinessContractForAuthorityReview(readiness); err != nil {
		return BaseRouteRegistrationAuthorityReviewContract{}, err
	}
	if err := validateBaseRouteRegistrationAuthorityReviewIdentity(readiness, evidence); err != nil {
		return BaseRouteRegistrationAuthorityReviewContract{}, err
	}
	if err := validateBaseRouteRegistrationAuthorityReviewRefs(evidence); err != nil {
		return BaseRouteRegistrationAuthorityReviewContract{}, err
	}
	if err := validateBaseRouteRegistrationAuthorityReviewNoMutation(evidence); err != nil {
		return BaseRouteRegistrationAuthorityReviewContract{}, err
	}

	return BaseRouteRegistrationAuthorityReviewContract{
		Kind:                        BaseRouteRegistrationAuthorityReviewContractKind,
		Version:                     readiness.Version,
		HarnessEvidenceRef:          strings.TrimSpace(readiness.HarnessEvidenceRef),
		ObservationEvidenceRef:      strings.TrimSpace(readiness.ObservationEvidenceRef),
		ComparisonEvidenceRef:       strings.TrimSpace(readiness.ComparisonEvidenceRef),
		RoutePrefix:                 strings.TrimSpace(readiness.RoutePrefix),
		ReadinessContractRef:        strings.TrimSpace(evidence.ReadinessContractRef),
		AuthorityReviewBoundary:     BaseRouteRegistrationAuthorityReviewBoundary,
		OwnerAuthorizationRef:       strings.TrimSpace(evidence.OwnerAuthorizationRef),
		ReviewerAuthorizationRef:    strings.TrimSpace(evidence.ReviewerAuthorizationRef),
		RedCeremonyPlanRef:          strings.TrimSpace(evidence.RedCeremonyPlanRef),
		RequiredPrerequisiteRefs:    canonicalStrings(evidence.RequiredPrerequisiteRefs),
		ChecklistItemRefs:           canonicalStrings(evidence.ChecklistItemRefs),
		ReviewerFindingRefs:         canonicalStrings(evidence.ReviewerFindingRefs),
		OpenQuestionRefs:            canonicalStrings(evidence.OpenQuestionRefs),
		ReviewReportRef:             strings.TrimSpace(evidence.ReviewReportRef),
		RollbackPlanRef:             strings.TrimSpace(evidence.RollbackPlanRef),
		AuthorityReviewStatus:       BaseRouteRegistrationAuthorityReviewStatusRecorded,
		DeployedRouteRegistered:     false,
		ProductionAuthTouched:       false,
		StagingClaimed:              false,
		ProductionStateMutated:      false,
		VMLifecycleTouched:          false,
		PromotionRollbackTouched:    false,
		RunAcceptanceRecordTouched:  false,
		RouteRegistrationAuthorized: false,
		RedCeremonyOpened:           false,
		RedCeremonyApproved:         false,
		NoMutation:                  true,
		BlockedPrerequisites:        baseRouteRegistrationBlockedPrerequisites(),
	}, nil
}

func validateBaseRouteRegistrationReadinessContractForAuthorityReview(readiness BaseRouteRegistrationReadinessContract) error {
	if readiness.Kind != BaseRouteRegistrationReadinessContractKind {
		return fmt.Errorf("base route registration authority review: readiness kind %q is not %q", readiness.Kind, BaseRouteRegistrationReadinessContractKind)
	}
	if !readiness.Version.Valid() {
		return fmt.Errorf("base route registration authority review: readiness version is invalid")
	}
	if strings.TrimSpace(readiness.HarnessEvidenceRef) == "" {
		return fmt.Errorf("base route registration authority review: readiness harness evidence ref is required")
	}
	if strings.TrimSpace(readiness.ObservationEvidenceRef) == "" {
		return fmt.Errorf("base route registration authority review: readiness observation evidence ref is required")
	}
	if strings.TrimSpace(readiness.ComparisonEvidenceRef) == "" {
		return fmt.Errorf("base route registration authority review: readiness comparison evidence ref is required")
	}
	if strings.TrimSpace(readiness.RoutePrefix) != "/api/base/" {
		return fmt.Errorf("base route registration authority review: readiness route prefix %q is not /api/base/", readiness.RoutePrefix)
	}
	if readiness.ReadinessBoundary != BaseRouteRegistrationReadinessBoundary {
		return fmt.Errorf("base route registration authority review: readiness boundary %q is not %q", readiness.ReadinessBoundary, BaseRouteRegistrationReadinessBoundary)
	}
	if readiness.ReadinessStatus != BaseRouteRegistrationReadinessStatusBlocked {
		return fmt.Errorf("base route registration authority review: readiness status %q is not %q", readiness.ReadinessStatus, BaseRouteRegistrationReadinessStatusBlocked)
	}
	if !sameCanonicalStrings(readiness.RequiredScopes, []string{"read:base", "write:base"}) {
		return fmt.Errorf("base route registration authority review: readiness scopes must be read:base and write:base")
	}
	if !sameCanonicalStrings(readiness.RequiredPrerequisiteRefs, baseRouteRegistrationBlockedPrerequisites()) {
		return fmt.Errorf("base route registration authority review: readiness prerequisite refs do not match blocked prerequisites")
	}
	if strings.TrimSpace(readiness.RollbackPlanRef) == "" {
		return fmt.Errorf("base route registration authority review: readiness rollback plan ref is required")
	}
	if err := validateBaseRouteRegistrationUnsafeClaims("readiness", readiness.DeployedRouteRegistered, readiness.ProductionAuthTouched, readiness.StagingClaimed, readiness.ProductionStateMutated, readiness.VMLifecycleTouched, readiness.PromotionRollbackTouched, readiness.RunAcceptanceRecordTouched); err != nil {
		return err
	}
	if readiness.RouteRegistrationAllowed {
		return fmt.Errorf("base route registration authority review: readiness cannot allow route registration")
	}
	if !readiness.NoMutation {
		return fmt.Errorf("base route registration authority review: readiness must be no-mutation")
	}
	return nil
}

func validateBaseRouteRegistrationAuthorityReviewIdentity(readiness BaseRouteRegistrationReadinessContract, evidence BaseRouteRegistrationAuthorityReviewEvidence) error {
	if evidence.Version != readiness.Version {
		return fmt.Errorf("base route registration authority review: evidence version does not match readiness")
	}
	if strings.TrimSpace(evidence.HarnessEvidenceRef) != strings.TrimSpace(readiness.HarnessEvidenceRef) {
		return fmt.Errorf("base route registration authority review: evidence harness ref does not match readiness")
	}
	if strings.TrimSpace(evidence.ObservationEvidenceRef) != strings.TrimSpace(readiness.ObservationEvidenceRef) {
		return fmt.Errorf("base route registration authority review: evidence observation ref does not match readiness")
	}
	if strings.TrimSpace(evidence.ComparisonEvidenceRef) != strings.TrimSpace(readiness.ComparisonEvidenceRef) {
		return fmt.Errorf("base route registration authority review: evidence comparison ref does not match readiness")
	}
	if strings.TrimSpace(evidence.RoutePrefix) != strings.TrimSpace(readiness.RoutePrefix) {
		return fmt.Errorf("base route registration authority review: evidence route prefix does not match readiness")
	}
	if strings.TrimSpace(evidence.RollbackPlanRef) != strings.TrimSpace(readiness.RollbackPlanRef) {
		return fmt.Errorf("base route registration authority review: evidence rollback plan ref does not match readiness")
	}
	if !sameCanonicalStrings(evidence.RequiredPrerequisiteRefs, readiness.RequiredPrerequisiteRefs) {
		return fmt.Errorf("base route registration authority review: evidence prerequisite refs do not match readiness")
	}
	return nil
}

func validateBaseRouteRegistrationAuthorityReviewRefs(evidence BaseRouteRegistrationAuthorityReviewEvidence) error {
	if strings.TrimSpace(evidence.ReadinessContractRef) == "" {
		return fmt.Errorf("base route registration authority review: readiness contract ref is required")
	}
	if strings.TrimSpace(evidence.OwnerAuthorizationRef) == "" {
		return fmt.Errorf("base route registration authority review: owner authorization ref is required")
	}
	if strings.TrimSpace(evidence.ReviewerAuthorizationRef) == "" {
		return fmt.Errorf("base route registration authority review: reviewer authorization ref is required")
	}
	if strings.TrimSpace(evidence.RedCeremonyPlanRef) == "" {
		return fmt.Errorf("base route registration authority review: red ceremony plan ref is required")
	}
	if !sameCanonicalStrings(evidence.ChecklistItemRefs, baseRouteRegistrationAuthorityReviewRequiredChecklistItems()) {
		return fmt.Errorf("base route registration authority review: checklist item refs do not match required review items")
	}
	if len(canonicalStrings(evidence.ReviewerFindingRefs)) == 0 {
		return fmt.Errorf("base route registration authority review: reviewer finding refs are required")
	}
	if strings.TrimSpace(evidence.ReviewReportRef) == "" {
		return fmt.Errorf("base route registration authority review: review report ref is required")
	}
	return nil
}

func validateBaseRouteRegistrationAuthorityReviewNoMutation(evidence BaseRouteRegistrationAuthorityReviewEvidence) error {
	if err := validateBaseRouteRegistrationUnsafeClaims("authority review evidence", evidence.DeployedRouteRegistered, evidence.ProductionAuthTouched, evidence.StagingClaimed, evidence.ProductionStateMutated, evidence.VMLifecycleTouched, evidence.PromotionRollbackTouched, evidence.RunAcceptanceRecordTouched); err != nil {
		return err
	}
	if evidence.RouteRegistrationAuthorized {
		return fmt.Errorf("base route registration authority review: evidence cannot authorize route registration")
	}
	if evidence.RedCeremonyOpened {
		return fmt.Errorf("base route registration authority review: evidence cannot open red ceremony")
	}
	if evidence.RedCeremonyApproved {
		return fmt.Errorf("base route registration authority review: evidence cannot approve red ceremony")
	}
	if !evidence.NoMutation {
		return fmt.Errorf("base route registration authority review: evidence must be no-mutation")
	}
	return nil
}

func baseRouteRegistrationAuthorityReviewRequiredChecklistItems() []string {
	return []string{
		BaseRouteRegistrationAuthorityReviewItemRedCeremonyScope,
		BaseRouteRegistrationAuthorityReviewItemAuthSessionScope,
		BaseRouteRegistrationAuthorityReviewItemServiceRouting,
		BaseRouteRegistrationAuthorityReviewItemStagingIdentity,
		BaseRouteRegistrationAuthorityReviewItemRollbackRehearsal,
		BaseRouteRegistrationAuthorityReviewItemProductionStateBoundary,
	}
}
