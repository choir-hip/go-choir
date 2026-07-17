package vmctl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

type FrozenRouteBootstrapCandidate struct {
	ID                  string                                         `json:"candidate_id"`
	RouteSlotID         string                                         `json:"route_slot_id"`
	Verification        computerversion.RealizationVerificationReceipt `json:"verification"`
	ApprovalEvidence    routeledger.AuthorizationEvidence              `json:"approval_evidence"`
	CertificateEvidence routeledger.AuthorizationEvidence              `json:"certificate_evidence"`
	Bootstrap           FrozenRouteTransitionPlan                      `json:"bootstrap_plan"`
	Rollback            FrozenRouteTransitionPlan                      `json:"rollback_plan"`
	PreparedAt          time.Time                                      `json:"prepared_at"`
}

func buildFrozenRouteBootstrapCandidate(slotID string, verification computerversion.RealizationVerificationReceipt, approval routeledger.AuthorizationEvidence, preparedAt time.Time) (FrozenRouteBootstrapCandidate, error) {
	ownerID, computerID, err := routeledger.ParseRouteSlotID(slotID)
	if err != nil || verification.Validate() != nil || verification.Identity.OwnerID != ownerID || verification.Identity.DesktopID != computerID || approval.Validate() != nil || approval.Kind != routeledger.AuthorizationEvidenceApproval || approval.RouteSlotID != slotID || approval.ComputerVersion != verification.Version || preparedAt.IsZero() {
		return FrozenRouteBootstrapCandidate{}, fmt.Errorf("vmctl bootstrap: candidate bindings are invalid")
	}
	certificatePayload, err := json.Marshal(struct {
		Kind         string                                         `json:"kind"`
		RouteSlotID  string                                         `json:"route_slot_id"`
		Verification computerversion.RealizationVerificationReceipt `json:"verification"`
		ApprovalRef  string                                         `json:"approval_ref"`
	}{"verified_route_bootstrap", slotID, verification, approval.Ref})
	if err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	certificate, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, verification.Version, certificatePayload, preparedAt.UTC())
	if err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	command := routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap, New: verification.Version, ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref), IdempotencyKey: routeledger.IdempotencyKey("idempotency:bootstrap:" + strings.TrimPrefix(verification.ID, "verification:sha256:"))}
	if err := command.Validate(); err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	rollback := routeledger.TransitionCommand{RouteSlotID: slotID, Kind: routeledger.TransitionBootstrapRollback, Old: verification.Version, New: verification.Version, ExpectedGeneration: 1, ApprovalRef: routeledger.ApprovalRef(approval.Ref), PromotionCertificateRef: routeledger.PromotionCertificateRef(certificate.Ref), RollbackTargetReceiptID: routeledger.BootstrapReceiptID(slotID, command.IdempotencyKey), IdempotencyKey: routeledger.IdempotencyKey("idempotency:bootstrap-rollback:" + strings.TrimPrefix(verification.ID, "verification:sha256:"))}
	if err := rollback.Validate(); err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	candidate := FrozenRouteBootstrapCandidate{RouteSlotID: slotID, Verification: verification, ApprovalEvidence: approval, CertificateEvidence: certificate, Bootstrap: transitionPlan(command), Rollback: transitionPlan(rollback), PreparedAt: preparedAt.UTC()}
	payload, err := frozenBootstrapPayload(candidate)
	if err != nil {
		return FrozenRouteBootstrapCandidate{}, err
	}
	digest := sha256.Sum256(payload)
	candidate.ID = "route-bootstrap:sha256:" + hex.EncodeToString(digest[:])
	return candidate, candidate.Validate()
}

func (c FrozenRouteBootstrapCandidate) Validate() error {
	if c.ID == "" || c.PreparedAt.IsZero() || c.Verification.Validate() != nil || c.ApprovalEvidence.Validate() != nil || c.CertificateEvidence.Validate() != nil || c.Bootstrap.Validate() != nil || c.Rollback.Validate() != nil {
		return fmt.Errorf("vmctl bootstrap: frozen candidate is incomplete")
	}
	ownerID, computerID, err := routeledger.ParseRouteSlotID(c.RouteSlotID)
	if err != nil || c.Verification.Identity.OwnerID != ownerID || c.Verification.Identity.DesktopID != computerID || c.ApprovalEvidence.Kind != routeledger.AuthorizationEvidenceApproval || c.CertificateEvidence.Kind != routeledger.AuthorizationEvidencePromotionCertificate || c.Bootstrap.Kind != routeledger.TransitionBootstrap || c.Bootstrap.ExpectedGeneration != 0 || c.Bootstrap.Old.Valid() || c.Rollback.Kind != routeledger.TransitionBootstrapRollback || c.Rollback.ExpectedGeneration != 1 || c.ApprovalEvidence.RouteSlotID != c.RouteSlotID || c.CertificateEvidence.RouteSlotID != c.RouteSlotID || c.ApprovalEvidence.ComputerVersion != c.Verification.Version || c.CertificateEvidence.ComputerVersion != c.Verification.Version || c.Bootstrap.RouteSlotID != c.RouteSlotID || c.Rollback.RouteSlotID != c.RouteSlotID || c.Bootstrap.New != c.Verification.Version || c.Rollback.Old != c.Verification.Version || c.Rollback.New != c.Verification.Version || !c.CertificateEvidence.CreatedAt.Equal(c.PreparedAt) || c.PreparedAt.Before(c.Verification.VerifiedAt) || string(c.Bootstrap.PromotionCertificateRef) != c.CertificateEvidence.Ref || string(c.Rollback.PromotionCertificateRef) != c.CertificateEvidence.Ref {
		return fmt.Errorf("vmctl bootstrap: frozen candidate typed joins are inconsistent")
	}
	expectedBootstrap := FrozenRouteTransitionPlan{RouteSlotID: c.RouteSlotID, Kind: routeledger.TransitionBootstrap, New: c.Verification.Version, PromotionCertificateRef: routeledger.PromotionCertificateRef(c.CertificateEvidence.Ref), IdempotencyKey: routeledger.IdempotencyKey("idempotency:bootstrap:" + strings.TrimPrefix(c.Verification.ID, "verification:sha256:"))}
	expectedRollback := FrozenRouteTransitionPlan{RouteSlotID: c.RouteSlotID, Kind: routeledger.TransitionBootstrapRollback, Old: c.Verification.Version, New: c.Verification.Version, ExpectedGeneration: 1, PromotionCertificateRef: routeledger.PromotionCertificateRef(c.CertificateEvidence.Ref), RollbackTargetReceiptID: routeledger.BootstrapReceiptID(c.RouteSlotID, expectedBootstrap.IdempotencyKey), IdempotencyKey: routeledger.IdempotencyKey("idempotency:bootstrap-rollback:" + strings.TrimPrefix(c.Verification.ID, "verification:sha256:"))}
	if c.Bootstrap != expectedBootstrap || c.Rollback != expectedRollback {
		return fmt.Errorf("vmctl bootstrap: frozen transition command was substituted")
	}
	certificatePayload, err := json.Marshal(struct {
		Kind         string                                         `json:"kind"`
		RouteSlotID  string                                         `json:"route_slot_id"`
		Verification computerversion.RealizationVerificationReceipt `json:"verification"`
		ApprovalRef  string                                         `json:"approval_ref"`
	}{"verified_route_bootstrap", c.RouteSlotID, c.Verification, c.ApprovalEvidence.Ref})
	if err != nil || string(c.CertificateEvidence.Payload) != string(certificatePayload) {
		return fmt.Errorf("vmctl bootstrap: certificate evidence payload mismatch")
	}
	payload, err := frozenBootstrapPayload(c)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	if c.ID != "route-bootstrap:sha256:"+hex.EncodeToString(digest[:]) {
		return fmt.Errorf("vmctl bootstrap: frozen candidate hash mismatch")
	}
	return nil
}

func frozenBootstrapPayload(candidate FrozenRouteBootstrapCandidate) ([]byte, error) {
	candidate.ID = ""
	return json.Marshal(candidate)
}
