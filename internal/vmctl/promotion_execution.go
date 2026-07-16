package vmctl

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

// FrozenRouteTransitionPlan is pre-G3 authority. ApprovalRef is deliberately
// absent: the executable command must reference the content-addressed,
// post-G3 AuthorizedRouteExecution evidence that authorizes it.
type FrozenRouteTransitionPlan struct {
	RouteSlotID             string                              `json:"route_slot_id"`
	Kind                    routeledger.TransitionKind          `json:"transition_kind"`
	Old                     computerversion.ComputerVersion     `json:"old_computer_version"`
	New                     computerversion.ComputerVersion     `json:"new_computer_version"`
	ExpectedGeneration      uint64                              `json:"expected_generation"`
	PromotionCertificateRef routeledger.PromotionCertificateRef `json:"promotion_certificate_ref"`
	RollbackTargetReceiptID routeledger.ReceiptID               `json:"rollback_target_receipt_id,omitempty"`
	IdempotencyKey          routeledger.IdempotencyKey          `json:"idempotency_key"`
}

func transitionPlan(command routeledger.TransitionCommand) FrozenRouteTransitionPlan {
	return FrozenRouteTransitionPlan{
		RouteSlotID: command.RouteSlotID, Kind: command.Kind, Old: command.Old, New: command.New,
		ExpectedGeneration: command.ExpectedGeneration, PromotionCertificateRef: command.PromotionCertificateRef,
		RollbackTargetReceiptID: command.RollbackTargetReceiptID, IdempotencyKey: command.IdempotencyKey,
	}
}

func (p FrozenRouteTransitionPlan) command(authorizationRef routeledger.ApprovalRef) routeledger.TransitionCommand {
	return routeledger.TransitionCommand{
		RouteSlotID: p.RouteSlotID, Kind: p.Kind, Old: p.Old, New: p.New,
		ExpectedGeneration: p.ExpectedGeneration, ApprovalRef: authorizationRef,
		PromotionCertificateRef: p.PromotionCertificateRef,
		RollbackTargetReceiptID: p.RollbackTargetReceiptID, IdempotencyKey: p.IdempotencyKey,
	}
}

func (p FrozenRouteTransitionPlan) Validate() error {
	placeholder := routeledger.ApprovalRef("approval:sha256:" + strings.Repeat("0", 64))
	return p.command(placeholder).Validate()
}

type AuthorizedRouteExecution struct {
	CandidateID      string                                         `json:"candidate_id"`
	Action           string                                         `json:"action"`
	OwnerApprovalRef string                                         `json:"owner_approval_ref"`
	VerificationRef  string                                         `json:"verification_ref"`
	CandidateCertRef string                                         `json:"candidate_certificate_ref"`
	Acceptance       G3PromotionAcceptance                          `json:"g3_acceptance"`
	Plan             FrozenRouteTransitionPlan                      `json:"transition_plan"`
	OwnerApproval    OwnerPromotionApproval                         `json:"owner_approval"`
	Verification     computerversion.RealizationVerificationReceipt `json:"verification"`
	PreparedAt       time.Time                                      `json:"prepared_at"`
}

func approvalEvidenceRef(approval OwnerPromotionApproval, routeSlotID string, verification computerversion.RealizationVerificationReceipt) string {
	payload, err := json.Marshal(approval)
	if err != nil {
		return ""
	}
	evidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, routeSlotID, verification.Version, payload, approval.ApprovedAt)
	if err != nil {
		return ""
	}
	return evidence.Ref
}

func newAuthorizedRouteExecution(candidateID, action string, approval OwnerPromotionApproval, verification computerversion.RealizationVerificationReceipt, candidateCertRef string, acceptance G3PromotionAcceptance, preparedAt time.Time, plan FrozenRouteTransitionPlan) (AuthorizedRouteExecution, routeledger.AuthorizationEvidence, error) {
	execution := AuthorizedRouteExecution{
		CandidateID: candidateID, Action: action, OwnerApprovalRef: approvalEvidenceRef(approval, plan.RouteSlotID, verification),
		VerificationRef: verification.ID, CandidateCertRef: candidateCertRef,
		Acceptance: acceptance, Plan: plan, OwnerApproval: approval, Verification: verification, PreparedAt: preparedAt.UTC(),
	}
	if candidateID == "" || execution.OwnerApprovalRef == "" || execution.VerificationRef == "" || candidateCertRef == "" || acceptance.CandidateID != candidateID || action != string(plan.Kind) {
		return AuthorizedRouteExecution{}, routeledger.AuthorizationEvidence{}, fmt.Errorf("vmctl promotion authority: execution authorization bindings are invalid")
	}
	if err := plan.Validate(); err != nil {
		return AuthorizedRouteExecution{}, routeledger.AuthorizationEvidence{}, err
	}
	payload, err := json.Marshal(execution)
	if err != nil {
		return AuthorizedRouteExecution{}, routeledger.AuthorizationEvidence{}, err
	}
	evidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidenceApproval, plan.RouteSlotID, plan.New, payload, acceptance.AcceptedAt)
	if err != nil {
		return AuthorizedRouteExecution{}, routeledger.AuthorizationEvidence{}, err
	}
	if err := execution.command(evidence).Validate(); err != nil {
		return AuthorizedRouteExecution{}, routeledger.AuthorizationEvidence{}, err
	}
	return execution, evidence, nil
}

func (e AuthorizedRouteExecution) command(evidence routeledger.AuthorizationEvidence) routeledger.TransitionCommand {
	return e.Plan.command(routeledger.ApprovalRef(evidence.Ref))
}

func transitionReceiptMatchesCommand(receipt routeledger.TransitionReceipt, command routeledger.TransitionCommand) bool {
	return receipt.Validate() == nil && receipt.RouteSlotID == command.RouteSlotID && receipt.Kind == command.Kind &&
		receipt.Old == command.Old && receipt.New == command.New && receipt.ExpectedGeneration == command.ExpectedGeneration &&
		receipt.CommittedGeneration == command.ExpectedGeneration+1 && receipt.ApprovalRef == command.ApprovalRef &&
		receipt.PromotionCertificateRef == command.PromotionCertificateRef && receipt.RollbackTargetReceiptID == command.RollbackTargetReceiptID &&
		receipt.IdempotencyKey == command.IdempotencyKey
}
