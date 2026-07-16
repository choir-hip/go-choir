package vmctl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

type FrozenRoutePromotionCandidate struct {
	ID                  string                                         `json:"candidate_id"`
	Route               RouteResolution                                `json:"current_route"`
	Verification        computerversion.RealizationVerificationReceipt `json:"verification"`
	Certificate         computerversion.PromotionCertificate           `json:"promotion_certificate"`
	ApprovalEvidence    routeledger.AuthorizationEvidence              `json:"approval_evidence"`
	CertificateEvidence routeledger.AuthorizationEvidence              `json:"certificate_evidence"`
	Promote             FrozenRouteTransitionPlan                      `json:"promote_plan"`
	Rollback            FrozenRouteTransitionPlan                      `json:"rollback_plan"`
	PreparedAt          time.Time                                      `json:"prepared_at"`
}

func buildFrozenRoutePromotionCandidate(current RouteResolution, verification computerversion.RealizationVerificationReceipt, approval routeledger.AuthorizationEvidence, preparedAt time.Time) (FrozenRoutePromotionCandidate, error) {
	if err := verification.Validate(); err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	if err := approval.Validate(); err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	if _, _, err := routeledger.ParseRouteSlotID(current.Slot.ID); err != nil || !current.Slot.Current.Valid() || current.Slot.Generation == 0 {
		return FrozenRoutePromotionCandidate{}, fmt.Errorf("vmctl promotion: current route is invalid")
	}
	currentReceipt := current.LatestReceipt
	if current.TransitionReceipt != nil {
		currentReceipt = *current.TransitionReceipt
	}
	if currentReceipt.Validate() != nil || currentReceipt.ID != current.Slot.LatestReceiptID {
		return FrozenRoutePromotionCandidate{}, fmt.Errorf("vmctl promotion: current route receipt is missing or stale")
	}
	slotID := current.Slot.ID
	if approval.Kind != routeledger.AuthorizationEvidenceApproval || approval.RouteSlotID != slotID || !routeledger.SameVersion(approval.ComputerVersion, verification.Version) {
		return FrozenRoutePromotionCandidate{}, fmt.Errorf("vmctl promotion: approval does not bind candidate route and ComputerVersion")
	}
	if preparedAt.IsZero() {
		return FrozenRoutePromotionCandidate{}, fmt.Errorf("vmctl promotion: preparation time is required")
	}
	preparedAt = preparedAt.UTC()
	certificate := computerversion.PromotionCertificate{
		ID: "promotion:" + strings.TrimPrefix(verification.ID, "verification:"), RouteSlot: slotID,
		Active: current.Slot.Current, Base: current.Slot.Current, Candidate: verification.Version,
		EvidenceRef: verification.ID, OwnerApproved: true, RollbackRef: string(currentReceipt.ID),
		HealthWindow: computerversion.PromotionHealthConfirmed,
		Ledgers: []computerversion.PromotionLedgerCertificate{
			{Name: "construction", State: computerversion.PromotionLedgerVerified},
			{Name: "route", State: computerversion.PromotionLedgerPrepared},
		},
	}
	if err := certificate.Validate(); err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	certificatePayload, err := json.Marshal(certificate)
	if err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	certificateEvidence, err := routeledger.NewAuthorizationEvidence(routeledger.AuthorizationEvidencePromotionCertificate, slotID, verification.Version, certificatePayload, preparedAt)
	if err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	promote := routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionPromote, Old: current.Slot.Current, New: verification.Version,
		ExpectedGeneration: current.Slot.Generation, ApprovalRef: routeledger.ApprovalRef(approval.Ref),
		PromotionCertificateRef: routeledger.PromotionCertificateRef(certificateEvidence.Ref),
		IdempotencyKey:          routeledger.IdempotencyKey("idempotency:promote:" + strings.TrimPrefix(verification.ID, "verification:sha256:")),
	}
	rollback := routeledger.TransitionCommand{
		RouteSlotID: slotID, Kind: routeledger.TransitionRollback, Old: verification.Version, New: current.Slot.Current,
		ExpectedGeneration:      current.Slot.Generation + 1,
		ApprovalRef:             routeledger.ApprovalRef(currentReceipt.ApprovalRef),
		PromotionCertificateRef: routeledger.PromotionCertificateRef(currentReceipt.PromotionCertificateRef),
		RollbackTargetReceiptID: currentReceipt.ID,
		IdempotencyKey:          routeledger.IdempotencyKey("idempotency:rollback:" + strings.TrimPrefix(verification.ID, "verification:sha256:")),
	}
	if err := promote.Validate(); err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	if err := rollback.Validate(); err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	candidate := FrozenRoutePromotionCandidate{Route: current, Verification: verification, Certificate: certificate, ApprovalEvidence: approval, CertificateEvidence: certificateEvidence, Promote: transitionPlan(promote), Rollback: transitionPlan(rollback), PreparedAt: preparedAt}
	payload, err := frozenPromotionPayload(candidate)
	if err != nil {
		return FrozenRoutePromotionCandidate{}, err
	}
	digest := sha256.Sum256(payload)
	candidate.ID = "route-promotion:sha256:" + hex.EncodeToString(digest[:])
	return candidate, candidate.Validate()
}

func (c FrozenRoutePromotionCandidate) Validate() error {
	if c.ID == "" || c.PreparedAt.IsZero() {
		return fmt.Errorf("vmctl promotion: frozen candidate is incomplete")
	}
	if err := c.Verification.Validate(); err != nil {
		return err
	}
	if err := c.Certificate.Validate(); err != nil {
		return err
	}
	if err := c.ApprovalEvidence.Validate(); err != nil {
		return err
	}
	if err := c.CertificateEvidence.Validate(); err != nil {
		return err
	}
	if err := c.Promote.Validate(); err != nil {
		return err
	}
	if err := c.Rollback.Validate(); err != nil {
		return err
	}
	current := c.Route.Slot
	ownerID, computerID, err := routeledger.ParseRouteSlotID(current.ID)
	if err != nil || c.Verification.Identity.OwnerID != ownerID || c.Verification.Identity.DesktopID != computerID {
		return fmt.Errorf("vmctl promotion: verification identity does not match route slot")
	}
	receipt := c.Route.LatestReceipt
	if c.Route.TransitionReceipt != nil {
		receipt = *c.Route.TransitionReceipt
	}
	expectedCertificate := computerversion.PromotionCertificate{
		ID: "promotion:" + strings.TrimPrefix(c.Verification.ID, "verification:"), RouteSlot: current.ID,
		Active: current.Current, Base: current.Current, Candidate: c.Verification.Version,
		EvidenceRef: c.Verification.ID, OwnerApproved: true, RollbackRef: string(receipt.ID),
		HealthWindow: computerversion.PromotionHealthConfirmed,
		Ledgers:      []computerversion.PromotionLedgerCertificate{{Name: "construction", State: computerversion.PromotionLedgerVerified}, {Name: "route", State: computerversion.PromotionLedgerPrepared}},
	}
	expectedPromote := FrozenRouteTransitionPlan{RouteSlotID: current.ID, Kind: routeledger.TransitionPromote, Old: current.Current, New: c.Verification.Version, ExpectedGeneration: current.Generation, PromotionCertificateRef: routeledger.PromotionCertificateRef(c.CertificateEvidence.Ref), IdempotencyKey: routeledger.IdempotencyKey("idempotency:promote:" + strings.TrimPrefix(c.Verification.ID, "verification:sha256:"))}
	expectedRollback := FrozenRouteTransitionPlan{RouteSlotID: current.ID, Kind: routeledger.TransitionRollback, Old: c.Verification.Version, New: current.Current, ExpectedGeneration: current.Generation + 1, PromotionCertificateRef: receipt.PromotionCertificateRef, RollbackTargetReceiptID: receipt.ID, IdempotencyKey: routeledger.IdempotencyKey("idempotency:rollback:" + strings.TrimPrefix(c.Verification.ID, "verification:sha256:"))}
	if !reflect.DeepEqual(c.Certificate, expectedCertificate) || c.Promote != expectedPromote || c.Rollback != expectedRollback {
		return fmt.Errorf("vmctl promotion: frozen certificate or transition commands were substituted")
	}
	if receipt.Validate() != nil || receipt.ID != current.LatestReceiptID || receipt.RouteSlotID != current.ID || receipt.CommittedGeneration != current.Generation || !routeledger.SameVersion(receipt.New, current.Current) || current.Generation == 0 || !current.Current.Valid() || c.Certificate.RouteSlot != current.ID || c.Certificate.RollbackRef != string(receipt.ID) ||
		!routeledger.SameVersion(c.Certificate.Active, current.Current) || !routeledger.SameVersion(c.Certificate.Base, current.Current) ||
		!routeledger.SameVersion(c.Certificate.Candidate, c.Verification.Version) || c.Certificate.EvidenceRef != c.Verification.ID ||
		c.ApprovalEvidence.Kind != routeledger.AuthorizationEvidenceApproval ||
		c.ApprovalEvidence.RouteSlotID != current.ID || !routeledger.SameVersion(c.ApprovalEvidence.ComputerVersion, c.Verification.Version) ||
		c.CertificateEvidence.Kind != routeledger.AuthorizationEvidencePromotionCertificate || c.CertificateEvidence.Ref != string(c.Promote.PromotionCertificateRef) ||
		c.CertificateEvidence.RouteSlotID != current.ID || !routeledger.SameVersion(c.CertificateEvidence.ComputerVersion, c.Verification.Version) || !c.CertificateEvidence.CreatedAt.Equal(c.PreparedAt) || c.PreparedAt.Before(c.Verification.VerifiedAt) ||
		c.Promote.ExpectedGeneration != current.Generation || !routeledger.SameVersion(c.Promote.Old, current.Current) || !routeledger.SameVersion(c.Promote.New, c.Verification.Version) ||
		c.Rollback.ExpectedGeneration != current.Generation+1 || c.Rollback.RollbackTargetReceiptID != receipt.ID ||
		!routeledger.SameVersion(c.Rollback.Old, c.Verification.Version) || !routeledger.SameVersion(c.Rollback.New, current.Current) ||
		c.Rollback.PromotionCertificateRef != receipt.PromotionCertificateRef {
		return fmt.Errorf("vmctl promotion: frozen candidate typed joins are inconsistent")
	}
	certificatePayload, err := json.Marshal(c.Certificate)
	if err != nil || string(c.CertificateEvidence.Payload) != string(certificatePayload) {
		return fmt.Errorf("vmctl promotion: certificate evidence payload mismatch")
	}
	payload, err := frozenPromotionPayload(c)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	if c.ID != "route-promotion:sha256:"+hex.EncodeToString(digest[:]) {
		return fmt.Errorf("vmctl promotion: frozen candidate hash mismatch")
	}
	return nil
}

func frozenPromotionPayload(candidate FrozenRoutePromotionCandidate) ([]byte, error) {
	candidate.ID = ""
	return json.Marshal(candidate)
}
