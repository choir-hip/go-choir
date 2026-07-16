package routeledger

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

// FrozenTransitionPlan is the pre-acceptance route intent embedded in the
// signed execution envelope. ApprovalRef is deliberately absent: the committed
// command must point at the content-addressed execution envelope itself.
type FrozenTransitionPlan struct {
	RouteSlotID             string                          `json:"route_slot_id"`
	Kind                    TransitionKind                  `json:"transition_kind"`
	Old                     computerversion.ComputerVersion `json:"old_computer_version"`
	New                     computerversion.ComputerVersion `json:"new_computer_version"`
	ExpectedGeneration      uint64                          `json:"expected_generation"`
	PromotionCertificateRef PromotionCertificateRef         `json:"promotion_certificate_ref"`
	RollbackTargetReceiptID ReceiptID                       `json:"rollback_target_receipt_id,omitempty"`
	IdempotencyKey          IdempotencyKey                  `json:"idempotency_key"`
}

func (p FrozenTransitionPlan) command(ref ApprovalRef) TransitionCommand {
	return TransitionCommand{RouteSlotID: p.RouteSlotID, Kind: p.Kind, Old: p.Old, New: p.New, ExpectedGeneration: p.ExpectedGeneration, ApprovalRef: ref, PromotionCertificateRef: p.PromotionCertificateRef, RollbackTargetReceiptID: p.RollbackTargetReceiptID, IdempotencyKey: p.IdempotencyKey}
}

type SignedOwnerApproval struct {
	RouteSlotID        string                          `json:"route_slot_id"`
	OwnerID            string                          `json:"owner_id"`
	ComputerVersion    computerversion.ComputerVersion `json:"computer_version"`
	ConstructionSHA256 string                          `json:"construction_sha256"`
	Decision           string                          `json:"decision"`
	KeyID              string                          `json:"key_id"`
	ApprovedAt         time.Time                       `json:"approved_at"`
	Signature          string                          `json:"signature"`
}

type SignedG3Acceptance struct {
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

// SignedRouteExecution is the complete post-G3 authority consumed at the SQL
// mutation boundary. Every semantic input needed for authorization is inside
// the content-addressed envelope; SQLLedger does not trust a vmctl callback.
type SignedRouteExecution struct {
	CandidateID      string                                         `json:"candidate_id"`
	Action           string                                         `json:"action"`
	OwnerApprovalRef string                                         `json:"owner_approval_ref"`
	VerificationRef  string                                         `json:"verification_ref"`
	CandidateCertRef string                                         `json:"candidate_certificate_ref"`
	Acceptance       SignedG3Acceptance                             `json:"g3_acceptance"`
	Plan             FrozenTransitionPlan                           `json:"transition_plan"`
	OwnerApproval    SignedOwnerApproval                            `json:"owner_approval"`
	Verification     computerversion.RealizationVerificationReceipt `json:"verification"`
	PreparedAt       time.Time                                      `json:"prepared_at"`
}

func frozenPlanSHA256(plan FrozenTransitionPlan) string {
	payload, err := json.Marshal(plan)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func verifySignature(publicKey ed25519.PublicKey, value any, signature string) error {
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || len(decoded) != ed25519.SignatureSize {
		return fmt.Errorf("route ledger: signed execution signature is invalid")
	}
	payload, err := json.Marshal(value)
	if err != nil || !ed25519.Verify(publicKey, payload, decoded) {
		return fmt.Errorf("route ledger: signed execution signature verification failed")
	}
	return nil
}

func validateSignedExecution(command TransitionCommand, evidence []AuthorizationEvidence, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("route ledger: pinned promotion authority key is invalid")
	}
	byRef := make(map[string]AuthorizationEvidence, len(evidence))
	for _, item := range evidence {
		if err := item.Validate(); err != nil {
			return err
		}
		byRef[item.Ref] = item
	}
	gate, ok := byRef[string(command.ApprovalRef)]
	if !ok || gate.Kind != AuthorizationEvidenceApproval || gate.RouteSlotID != command.RouteSlotID || !SameVersion(gate.ComputerVersion, command.New) {
		return fmt.Errorf("route ledger: signed execution gate is missing or inconsistent")
	}
	var execution SignedRouteExecution
	if err := json.Unmarshal(gate.Payload, &execution); err != nil {
		return fmt.Errorf("route ledger: decode signed execution: %w", err)
	}
	if execution.CandidateID == "" || execution.Action != string(command.Kind) || execution.Plan.command(command.ApprovalRef) != command || execution.PreparedAt.IsZero() || execution.Verification.Validate() != nil || execution.Verification.ID != execution.VerificationRef {
		return fmt.Errorf("route ledger: signed execution command or verification binding is invalid")
	}
	planDigest := frozenPlanSHA256(execution.Plan)
	if (command.Kind == TransitionBootstrap && (execution.Acceptance.BootstrapPlanSHA256 != planDigest || execution.Acceptance.PromotePlanSHA256 != "" || execution.Acceptance.RollbackPlanSHA256 != "")) || (command.Kind == TransitionPromote && execution.Acceptance.PromotePlanSHA256 != planDigest) || (command.Kind == TransitionRollback && execution.Acceptance.RollbackPlanSHA256 != planDigest) {
		return fmt.Errorf("route ledger: signed G3 acceptance does not authorize the transition plan")
	}
	ownerID, computerID, err := ParseRouteSlotID(command.RouteSlotID)
	if err != nil || execution.Verification.Identity.OwnerID != ownerID || execution.Verification.Identity.DesktopID != computerID {
		return fmt.Errorf("route ledger: signed execution route identity is invalid")
	}
	approvalEvidence, ok := byRef[execution.OwnerApprovalRef]
	if !ok || approvalEvidence.Kind != AuthorizationEvidenceApproval || approvalEvidence.RouteSlotID != command.RouteSlotID || !SameVersion(approvalEvidence.ComputerVersion, execution.Verification.Version) || !approvalEvidence.CreatedAt.Equal(execution.OwnerApproval.ApprovedAt) {
		return fmt.Errorf("route ledger: signed owner approval evidence is missing or inconsistent")
	}
	approvalPayload, err := json.Marshal(execution.OwnerApproval)
	if err != nil || string(approvalPayload) != string(approvalEvidence.Payload) {
		return fmt.Errorf("route ledger: signed owner approval payload is inconsistent")
	}
	approval := execution.OwnerApproval
	approvalSignature := approval.Signature
	approval.Signature = ""
	if approval.Decision != "approve" || approval.RouteSlotID != command.RouteSlotID || approval.OwnerID != ownerID || approval.KeyID == "" || approval.ApprovedAt.IsZero() || approval.ApprovedAt.After(execution.Verification.VerifiedAt) || !SameVersion(approval.ComputerVersion, execution.Verification.Version) || approval.ConstructionSHA256 != execution.Verification.ConstructionSHA256 {
		return fmt.Errorf("route ledger: signed owner approval bindings are invalid")
	}
	if err := verifySignature(publicKey, approval, approvalSignature); err != nil {
		return err
	}
	certificate, ok := byRef[execution.CandidateCertRef]
	if !ok || certificate.Kind != AuthorizationEvidencePromotionCertificate || certificate.RouteSlotID != command.RouteSlotID || !SameVersion(certificate.ComputerVersion, execution.Verification.Version) || !certificate.CreatedAt.Equal(execution.PreparedAt) {
		return fmt.Errorf("route ledger: candidate certificate evidence is missing or inconsistent")
	}
	acceptance := execution.Acceptance
	acceptanceSignature := acceptance.Signature
	acceptance.Signature = ""
	if acceptance.Decision != "accept" || acceptance.CandidateID != execution.CandidateID || acceptance.RouteSlotID != command.RouteSlotID || acceptance.OwnerID != ownerID || acceptance.KeyID == "" || !SameVersion(acceptance.ComputerVersion, execution.Verification.Version) || acceptance.VerificationRef != execution.VerificationRef || acceptance.CertificateRef != execution.CandidateCertRef || acceptance.AcceptedAt.IsZero() || !acceptance.AcceptedAt.After(execution.PreparedAt) || !gate.CreatedAt.Equal(acceptance.AcceptedAt) {
		return fmt.Errorf("route ledger: signed G3 acceptance bindings are invalid")
	}
	return verifySignature(publicKey, acceptance, acceptanceSignature)
}

func queryPinnedPromotionKey(ctx context.Context, query queryRower) (ed25519.PublicKey, error) {
	var encoded string
	if err := query.QueryRowContext(ctx, `SELECT public_key_base64 FROM computer_version_route_authority_config WHERE config_id = 1`).Scan(&encoded); err != nil {
		return nil, fmt.Errorf("route ledger: read pinned promotion authority key: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("route ledger: pinned promotion authority key is invalid")
	}
	return ed25519.PublicKey(decoded), nil
}

func configurePinnedPromotionKey(ctx context.Context, db *sql.DB, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("route ledger: promotion authority public key is invalid")
	}
	encoded := base64.StdEncoding.EncodeToString(publicKey)
	if _, err := db.ExecContext(ctx, `INSERT INTO computer_version_route_authority_config (config_id, public_key_base64) VALUES (1, ?) ON DUPLICATE KEY UPDATE config_id = config_id`, encoded); err != nil {
		return fmt.Errorf("route ledger: pin promotion authority key: %w", err)
	}
	pinned, err := queryPinnedPromotionKey(ctx, db)
	if err != nil {
		return err
	}
	if !bytes.Equal(pinned, publicKey) {
		return fmt.Errorf("route ledger: promotion authority public key conflicts with pinned authority")
	}
	return nil
}
