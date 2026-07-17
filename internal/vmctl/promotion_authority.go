package vmctl

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
)

type OwnerPromotionApproval struct {
	RouteSlotID        string                          `json:"route_slot_id"`
	OwnerID            string                          `json:"owner_id"`
	ComputerVersion    computerversion.ComputerVersion `json:"computer_version"`
	ConstructionSHA256 string                          `json:"construction_sha256"`
	Decision           string                          `json:"decision"`
	KeyID              string                          `json:"key_id"`
	ApprovedAt         time.Time                       `json:"approved_at"`
	Signature          string                          `json:"signature"`
}

type G3PromotionAcceptance struct {
	CandidateID         string                          `json:"candidate_id"`
	RouteSlotID         string                          `json:"route_slot_id"`
	OwnerID             string                          `json:"owner_id"`
	ComputerVersion     computerversion.ComputerVersion `json:"computer_version"`
	VerificationRef     string                          `json:"verification_ref"`
	CertificateRef      string                          `json:"certificate_ref"`
	BootstrapPlanSHA256 string                          `json:"bootstrap_plan_sha256,omitempty"`
	PromotePlanSHA256   string                          `json:"promote_plan_sha256,omitempty"`
	RollbackPlanSHA256  string                          `json:"rollback_plan_sha256,omitempty"`
	Decision            string                          `json:"decision"`
	KeyID               string                          `json:"key_id"`
	AcceptedAt          time.Time                       `json:"accepted_at"`
	Signature           string                          `json:"signature"`
}

func ParsePromotionAuthorityPublicKey(encoded string) (ed25519.PublicKey, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("vmctl promotion authority: invalid Ed25519 public key")
	}
	return ed25519.PublicKey(append([]byte(nil), decoded...)), nil
}

func (a OwnerPromotionApproval) SigningPayload() ([]byte, error) {
	a.Signature = ""
	return json.Marshal(a)
}

func (a OwnerPromotionApproval) verify(publicKey ed25519.PublicKey, routeSlotID string, verification computerversion.RealizationVerificationReceipt) error {
	if len(publicKey) != ed25519.PublicKeySize || a.Decision != "approve" || a.RouteSlotID != routeSlotID || a.OwnerID == "" || a.KeyID == "" || a.ApprovedAt.IsZero() || a.ComputerVersion != verification.Version || a.ConstructionSHA256 != verification.ConstructionSHA256 || verification.Identity.OwnerID != a.OwnerID || a.ApprovedAt.After(verification.VerifiedAt) {
		return fmt.Errorf("vmctl promotion authority: owner approval bindings are invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("vmctl promotion authority: owner approval signature is invalid")
	}
	payload, err := a.SigningPayload()
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("vmctl promotion authority: owner approval signature verification failed")
	}
	return nil
}

func transitionPlanSHA256(plan FrozenRouteTransitionPlan) string {
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (a G3PromotionAcceptance) SigningPayload() ([]byte, error) {
	a.Signature = ""
	return json.Marshal(a)
}

func (a G3PromotionAcceptance) verify(publicKey ed25519.PublicKey, candidate FrozenRoutePromotionCandidate) error {
	ownerID, _, err := parseCandidateRoute(candidate)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || a.Decision != "accept" || a.CandidateID != candidate.ID || a.RouteSlotID != candidate.Route.Slot.ID || a.OwnerID != ownerID || a.ComputerVersion != candidate.Verification.Version || a.VerificationRef != candidate.Verification.ID || a.CertificateRef != candidate.CertificateEvidence.Ref || a.PromotePlanSHA256 != transitionPlanSHA256(candidate.Promote) || a.RollbackPlanSHA256 != transitionPlanSHA256(candidate.Rollback) || a.BootstrapPlanSHA256 != "" || a.KeyID == "" || a.AcceptedAt.IsZero() || !a.AcceptedAt.After(candidate.PreparedAt) {
		return fmt.Errorf("vmctl promotion authority: G3 acceptance bindings are invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("vmctl promotion authority: G3 acceptance signature is invalid")
	}
	payload, err := a.SigningPayload()
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("vmctl promotion authority: G3 acceptance signature verification failed")
	}
	return nil
}

func (a G3PromotionAcceptance) verifyBootstrap(publicKey ed25519.PublicKey, candidate FrozenRouteBootstrapCandidate) error {
	ownerID, _, err := routeledger.ParseRouteSlotID(candidate.RouteSlotID)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || a.Decision != "accept" || a.CandidateID != candidate.ID || a.RouteSlotID != candidate.RouteSlotID || a.OwnerID != ownerID || a.ComputerVersion != candidate.Verification.Version || a.VerificationRef != candidate.Verification.ID || a.CertificateRef != candidate.CertificateEvidence.Ref || a.BootstrapPlanSHA256 != transitionPlanSHA256(candidate.Bootstrap) || a.PromotePlanSHA256 != "" || a.RollbackPlanSHA256 != transitionPlanSHA256(candidate.Rollback) || a.KeyID == "" || a.AcceptedAt.IsZero() || !a.AcceptedAt.After(candidate.PreparedAt) {
		return fmt.Errorf("vmctl promotion authority: G3 bootstrap acceptance bindings are invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("vmctl promotion authority: G3 bootstrap acceptance signature is invalid")
	}
	payload, err := a.SigningPayload()
	if err != nil || !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("vmctl promotion authority: G3 bootstrap acceptance signature verification failed")
	}
	return nil
}

func parseCandidateRoute(candidate FrozenRoutePromotionCandidate) (string, string, error) {
	return routeledger.ParseRouteSlotID(candidate.Route.Slot.ID)
}
