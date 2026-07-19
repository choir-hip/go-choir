package selfdevprotocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/verifierprotocol"
)

const (
	ReceiptKindCheckpoint      = "checkpoint"
	ReceiptKindRouteProjection = "route_projection"
)

type DecisionBinding struct {
	OperationID                      string  `json:"operation_id"`
	Decision                         string  `json:"decision"`
	IdempotencyKey                   string  `json:"idempotency_key"`
	BundleDigest                     string  `json:"bundle_digest"`
	VerifierRef                      string  `json:"verifier_ref"`
	Reason                           string  `json:"reason"`
	ExpectedDesiredEventHead         string  `json:"expected_desired_event_head"`
	ExpectedEffectiveEventHead       string  `json:"expected_effective_event_head"`
	ExpectedPendingTransitionRef     *string `json:"expected_pending_transition_ref"`
	ExpectedDesiredStateCommitment   string  `json:"expected_desired_state_commitment"`
	ExpectedEffectiveStateCommitment string  `json:"expected_effective_state_commitment"`
}

func DecisionBindingDigest(binding DecisionBinding) (string, error) {
	if strings.TrimSpace(binding.OperationID) == "" || binding.Decision != "approve" ||
		strings.TrimSpace(binding.IdempotencyKey) == "" || !computerevent.IsSHA256(strings.TrimSpace(binding.BundleDigest)) ||
		strings.TrimSpace(binding.VerifierRef) == "" || binding.Reason != "" || binding.ExpectedPendingTransitionRef == nil ||
		!computerevent.IsSHA256(strings.TrimSpace(binding.ExpectedDesiredEventHead)) ||
		!computerevent.IsSHA256(strings.TrimSpace(binding.ExpectedEffectiveEventHead)) ||
		!computerevent.IsSHA256(strings.TrimSpace(binding.ExpectedDesiredStateCommitment)) ||
		!computerevent.IsSHA256(strings.TrimSpace(binding.ExpectedEffectiveStateCommitment)) {
		return "", fmt.Errorf("decision binding: complete exact approval is required")
	}
	pending := strings.TrimSpace(*binding.ExpectedPendingTransitionRef)
	binding.OperationID = strings.TrimSpace(binding.OperationID)
	binding.IdempotencyKey = strings.TrimSpace(binding.IdempotencyKey)
	binding.BundleDigest = strings.TrimSpace(binding.BundleDigest)
	binding.VerifierRef = strings.TrimSpace(binding.VerifierRef)
	binding.ExpectedDesiredEventHead = strings.TrimSpace(binding.ExpectedDesiredEventHead)
	binding.ExpectedEffectiveEventHead = strings.TrimSpace(binding.ExpectedEffectiveEventHead)
	binding.ExpectedPendingTransitionRef = &pending
	binding.ExpectedDesiredStateCommitment = strings.TrimSpace(binding.ExpectedDesiredStateCommitment)
	binding.ExpectedEffectiveStateCommitment = strings.TrimSpace(binding.ExpectedEffectiveStateCommitment)
	canonical, err := computerevent.CanonicalJSON(binding)
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(canonical), nil
}

type VerifierCertificateRequest = verifierprotocol.Request

type VerifierCertificateResponse = verifierprotocol.Response

var NewVerifierCertificate = verifierprotocol.NewCertificate

var VerifyVerifierCertificate = verifierprotocol.Verify

type CheckpointRequest struct {
	ComputerID                   string                          `json:"computer_id"`
	IdempotencyKey               string                          `json:"idempotency_key"`
	ComputerVersion              computerversion.ComputerVersion `json:"computer_version"`
	AcceptedEventHead            string                          `json:"accepted_event_head"`
	EffectiveEventHead           string                          `json:"effective_event_head"`
	EffectiveStateCommitment     string                          `json:"effective_state_commitment"`
	EventHeadReceiptID           string                          `json:"event_head_receipt_id"`
	ReleaseDigest                string                          `json:"release_digest"`
	ReconstructionDigest         string                          `json:"reconstruction_digest"`
	MaterializationReceiptDigest string                          `json:"materialization_receipt_digest"`
	VerifierCertificateDigest    string                          `json:"verifier_certificate_digest"`
	VerifierCertificate          VerifierCertificateResponse     `json:"verifier_certificate"`
	VerifierTrustBootstrap       bool                            `json:"verifier_trust_bootstrap"`
	ReducerVersion               int                             `json:"reducer_version"`
}

type Checkpoint struct {
	Request CheckpointRequest `json:"request"`
	Digest  string            `json:"checkpoint_digest"`
}

type AcceptedEventAuthorizationEvidence struct {
	Version                       int                             `json:"version"`
	ComputerID                    string                          `json:"computer_id"`
	AcceptedOrRollbackEventDigest string                          `json:"accepted_or_rollback_event_digest"`
	EventHeadReceiptID            string                          `json:"event_head_receipt_id"`
	EffectiveEventHead            string                          `json:"effective_event_head"`
	OldComputerVersion            computerversion.ComputerVersion `json:"old_computer_version"`
	NewComputerVersion            computerversion.ComputerVersion `json:"new_computer_version"`
	DecisionActor                 string                          `json:"decision_actor"`
	DecisionScope                 string                          `json:"decision_scope"`
}

type PromotionJoinEvidence struct {
	Version                      int                             `json:"version"`
	ComputerID                   string                          `json:"computer_id"`
	EventHeadReceiptID           string                          `json:"event_head_receipt_id"`
	CheckpointReceiptDigest      string                          `json:"checkpoint_receipt_digest"`
	MaterializationReceiptDigest string                          `json:"materialization_receipt_digest"`
	VerifierCertificateDigest    string                          `json:"verifier_certificate_digest"`
	OldComputerVersion           computerversion.ComputerVersion `json:"old_computer_version"`
	NewComputerVersion           computerversion.ComputerVersion `json:"new_computer_version"`
}

type RouteProjectionCertificate struct {
	ComputerID                            string                        `json:"computer_id"`
	CanonicalEventHead                    string                        `json:"canonical_event_head"`
	EffectiveEventHead                    string                        `json:"effective_event_head"`
	EventHeadReceiptID                    string                        `json:"event_head_receipt_id"`
	AcceptedEventAuthorizationEvidenceRef string                        `json:"accepted_event_authorization_evidence_ref"`
	PromotionJoinEvidenceRef              string                        `json:"promotion_join_evidence_ref"`
	CheckpointReceiptDigest               string                        `json:"checkpoint_receipt_digest"`
	MaterializationReceiptDigest          string                        `json:"materialization_receipt_digest"`
	VerifierCertificateDigest             string                        `json:"verifier_certificate_digest"`
	RouteTransitionCommand                routeledger.TransitionCommand `json:"route_transition_command"`
	RouteTransitionCommandSHA256          string                        `json:"route_transition_command_sha256"`
	ExpiresAt                             string                        `json:"expires_at"`
}

type RouteProjectionRequest struct {
	ComputerID         string                            `json:"computer_id"`
	IdempotencyKey     string                            `json:"idempotency_key"`
	Checkpoint         CheckpointResponse                `json:"checkpoint"`
	CanonicalEventHead string                            `json:"canonical_event_head"`
	EventHeadReceiptID string                            `json:"event_head_receipt_id"`
	CodeClosure        computerversion.CodeClosure       `json:"code_closure"`
	ArtifactProgram    computerversion.ArtifactProgram   `json:"artifact_program"`
	ApprovalEvidence   routeledger.AuthorizationEvidence `json:"approval_evidence"`
	PromotionEvidence  routeledger.AuthorizationEvidence `json:"promotion_evidence"`
	Command            routeledger.TransitionCommand     `json:"command"`
	DecisionActor      string                            `json:"decision_actor"`
	DecisionScope      string                            `json:"decision_scope"`
	ExpiresAt          string                            `json:"expires_at"`
}

type RouteProjectionResponse struct {
	Certificate RouteProjectionCertificate `json:"certificate"`
	Receipt     AuthorityReceipt           `json:"receipt"`
}

type ApplyRouteProjectionRequest struct {
	Projection    RouteProjectionRequest  `json:"projection"`
	Authorization RouteProjectionResponse `json:"authorization"`
}

type AuthorityReceipt struct {
	Kind              string                  `json:"kind"`
	ComputerID        string                  `json:"computer_id"`
	RequestCommitment string                  `json:"request_commitment"`
	ArtifactDigest    string                  `json:"artifact_digest"`
	Issuer            string                  `json:"issuer"`
	Signer            computerevent.SignerRef `json:"signer"`
	IssuedAt          time.Time               `json:"issued_at"`
	Signature         string                  `json:"signature"`
}

type CheckpointResponse struct {
	Checkpoint Checkpoint       `json:"checkpoint"`
	Receipt    AuthorityReceipt `json:"receipt"`
}

func CheckpointFromRequest(request CheckpointRequest) (Checkpoint, []byte, error) {
	if strings.TrimSpace(request.ComputerID) == "" || strings.TrimSpace(request.IdempotencyKey) == "" || !request.ComputerVersion.Valid() ||
		!computerevent.IsSHA256(request.AcceptedEventHead) || request.AcceptedEventHead != request.EffectiveEventHead ||
		!computerevent.IsSHA256(request.EffectiveStateCommitment) || strings.TrimSpace(request.EventHeadReceiptID) == "" ||
		!computerevent.IsSHA256(request.ReleaseDigest) || !computerevent.IsSHA256(request.ReconstructionDigest) ||
		!computerevent.IsSHA256(request.MaterializationReceiptDigest) || !computerevent.IsSHA256(request.VerifierCertificateDigest) || request.ReducerVersion == 0 {
		return Checkpoint{}, nil, fmt.Errorf("self-development checkpoint: complete accepted/effective bindings are required")
	}
	if VerifyVerifierCertificate(request.VerifierCertificate) != nil {
		return Checkpoint{}, nil, fmt.Errorf("self-development checkpoint: verifier certificate refused")
	}
	certificateJSON, err := computerevent.CanonicalJSON(request.VerifierCertificate.Certificate)
	if err != nil || computerevent.DigestBytes(certificateJSON) != request.VerifierCertificateDigest {
		return Checkpoint{}, nil, fmt.Errorf("self-development checkpoint: verifier certificate digest mismatch")
	}
	verifierRequest := request.VerifierCertificate.Request
	if verifierRequest.ComputerID != request.ComputerID || verifierRequest.CodeRef != string(request.ComputerVersion.CodeRef) ||
		verifierRequest.ArtifactProgramRef != string(request.ComputerVersion.ArtifactProgramRef) || verifierRequest.ReleaseDigest != request.ReleaseDigest {
		return Checkpoint{}, nil, fmt.Errorf("self-development checkpoint: verifier certificate join mismatch")
	}
	canonical, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return Checkpoint{}, nil, err
	}
	digest := sha256.Sum256(canonical)
	checkpoint := Checkpoint{Request: request, Digest: hex.EncodeToString(digest[:])}
	artifact, err := computerevent.CanonicalJSON(checkpoint)
	return checkpoint, artifact, err
}

func RouteProjectionFromRequest(request RouteProjectionRequest, now time.Time) (RouteProjectionCertificate, []byte, error) {
	if strings.TrimSpace(request.ComputerID) == "" || strings.TrimSpace(request.IdempotencyKey) == "" || request.Checkpoint.Checkpoint.Request.ComputerID != request.ComputerID ||
		request.Checkpoint.Receipt.Kind != ReceiptKindCheckpoint || request.Checkpoint.Receipt.ComputerID != request.ComputerID ||
		request.Checkpoint.Receipt.ArtifactDigest != request.Checkpoint.Checkpoint.Digest || request.CodeClosure.Verify() != nil || request.ArtifactProgram.Verify() != nil ||
		!computerevent.IsSHA256(request.CanonicalEventHead) || strings.TrimSpace(request.EventHeadReceiptID) == "" ||
		request.ApprovalEvidence.Validate() != nil || request.PromotionEvidence.Validate() != nil ||
		request.ApprovalEvidence.Kind != routeledger.AuthorizationEvidenceApproval || request.PromotionEvidence.Kind != routeledger.AuthorizationEvidencePromotionCertificate ||
		request.Command.ApprovalRef != routeledger.ApprovalRef(request.ApprovalEvidence.Ref) ||
		request.Command.PromotionCertificateRef != routeledger.PromotionCertificateRef(request.PromotionEvidence.Ref) ||
		request.ApprovalEvidence.RouteSlotID != request.Command.RouteSlotID || request.PromotionEvidence.RouteSlotID != request.Command.RouteSlotID ||
		request.ApprovalEvidence.ComputerVersion != request.Command.New || request.PromotionEvidence.ComputerVersion != request.Command.New ||
		request.CodeClosure.Ref != request.Command.New.CodeRef || request.ArtifactProgram.Ref != request.Command.New.ArtifactProgramRef ||
		request.Command.Old == request.Command.New || request.Command.New != request.Checkpoint.Checkpoint.Request.ComputerVersion ||
		strings.TrimSpace(request.DecisionActor) == "" || strings.TrimSpace(request.DecisionScope) == "" {
		return RouteProjectionCertificate{}, nil, fmt.Errorf("self-development route projection: complete checkpoint, evidence, and command joins are required")
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
	if err != nil || expiresAt.Location() != time.UTC || !expiresAt.After(now.UTC()) || expiresAt.After(now.UTC().Add(5*time.Minute)) {
		return RouteProjectionCertificate{}, nil, fmt.Errorf("self-development route projection: short canonical expiry is required")
	}
	checkpointReceiptDigest, err := Digest(request.Checkpoint.Receipt)
	if err != nil {
		return RouteProjectionCertificate{}, nil, err
	}
	checkpoint := request.Checkpoint.Checkpoint.Request
	accepted := AcceptedEventAuthorizationEvidence{
		Version: 1, ComputerID: request.ComputerID, AcceptedOrRollbackEventDigest: checkpoint.AcceptedEventHead,
		EventHeadReceiptID: checkpoint.EventHeadReceiptID, EffectiveEventHead: checkpoint.EffectiveEventHead,
		OldComputerVersion: request.Command.Old, NewComputerVersion: request.Command.New,
		DecisionActor: request.DecisionActor, DecisionScope: request.DecisionScope,
	}
	promotion := PromotionJoinEvidence{
		Version: 1, ComputerID: request.ComputerID, EventHeadReceiptID: request.EventHeadReceiptID,
		CheckpointReceiptDigest: checkpointReceiptDigest, MaterializationReceiptDigest: checkpoint.MaterializationReceiptDigest,
		VerifierCertificateDigest: checkpoint.VerifierCertificateDigest,
		OldComputerVersion:        request.Command.Old, NewComputerVersion: request.Command.New,
	}
	acceptedJSON, err := computerevent.CanonicalJSON(accepted)
	if err != nil || !bytes.Equal(acceptedJSON, request.ApprovalEvidence.Payload) {
		return RouteProjectionCertificate{}, nil, fmt.Errorf("self-development route projection: accepted-event evidence payload mismatch")
	}
	promotionJSON, err := computerevent.CanonicalJSON(promotion)
	if err != nil || !bytes.Equal(promotionJSON, request.PromotionEvidence.Payload) {
		return RouteProjectionCertificate{}, nil, fmt.Errorf("self-development route projection: promotion evidence payload mismatch")
	}
	commandDigest, err := Digest(request.Command)
	if err != nil {
		return RouteProjectionCertificate{}, nil, err
	}
	certificate := RouteProjectionCertificate{
		ComputerID: request.ComputerID, CanonicalEventHead: request.CanonicalEventHead, EffectiveEventHead: checkpoint.EffectiveEventHead,
		EventHeadReceiptID: request.EventHeadReceiptID, AcceptedEventAuthorizationEvidenceRef: request.ApprovalEvidence.Ref,
		PromotionJoinEvidenceRef: request.PromotionEvidence.Ref, CheckpointReceiptDigest: checkpointReceiptDigest,
		MaterializationReceiptDigest: checkpoint.MaterializationReceiptDigest, VerifierCertificateDigest: checkpoint.VerifierCertificateDigest,
		RouteTransitionCommand: request.Command, RouteTransitionCommandSHA256: commandDigest, ExpiresAt: request.ExpiresAt,
	}
	artifact, err := computerevent.CanonicalJSON(certificate)
	return certificate, artifact, err
}

func (r AuthorityReceipt) signingPayload() ([]byte, error) {
	unsigned := r
	unsigned.Signature = ""
	return computerevent.CanonicalJSON(unsigned)
}

func NewAuthorityReceipt(kind, computerID, requestCommitment, artifactDigest, issuer string, signer computerevent.SigningKey, issuedAt time.Time) (AuthorityReceipt, error) {
	if (kind != ReceiptKindCheckpoint && kind != ReceiptKindRouteProjection) || strings.TrimSpace(computerID) == "" || !computerevent.IsSHA256(requestCommitment) || !computerevent.IsSHA256(artifactDigest) || strings.TrimSpace(issuer) == "" || len(signer.PrivateKey) != ed25519.PrivateKeySize || signer.SignerDomain != "platform-control" || signer.KeyID == "" || issuedAt.IsZero() {
		return AuthorityReceipt{}, fmt.Errorf("self-development authority receipt: complete platform-control bindings are required")
	}
	receipt := AuthorityReceipt{Kind: kind, ComputerID: computerID, RequestCommitment: requestCommitment, ArtifactDigest: artifactDigest, Issuer: issuer, Signer: signer.SignerRef, IssuedAt: issuedAt.UTC()}
	payload, err := receipt.signingPayload()
	if err != nil {
		return AuthorityReceipt{}, err
	}
	receipt.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(signer.PrivateKey, payload))
	return receipt, nil
}

func (r AuthorityReceipt) Verify(publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize || r.Signer.SignerDomain != "platform-control" || r.Signer.KeyID == "" || !computerevent.IsSHA256(r.RequestCommitment) || !computerevent.IsSHA256(r.ArtifactDigest) || r.IssuedAt.IsZero() {
		return fmt.Errorf("self-development authority receipt: invalid bindings")
	}
	signature, err := base64.StdEncoding.DecodeString(r.Signature)
	if err != nil {
		return err
	}
	payload, err := r.signingPayload()
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("self-development authority receipt: invalid signature")
	}
	return nil
}

func Digest(value any) (string, error) {
	canonical, err := computerevent.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func DecodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
