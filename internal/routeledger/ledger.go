package routeledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

var (
	ErrSlotNotFound     = errors.New("route ledger: slot not found")
	ErrStaleTransition  = errors.New("route ledger: stale transition")
	ErrIdempotencyReuse = errors.New("route ledger: idempotency key reused with different command")
)

type TransitionKind string

type ApprovalRef string
type PromotionCertificateRef string
type ReceiptID string
type IdempotencyKey string

type AuthorizationEvidenceKind string

const (
	AuthorizationEvidenceApproval             AuthorizationEvidenceKind = "approval"
	AuthorizationEvidencePromotionCertificate AuthorizationEvidenceKind = "promotion_certificate"
)

type AuthorizationEvidence struct {
	Ref             string                          `json:"evidence_ref"`
	Kind            AuthorizationEvidenceKind       `json:"evidence_kind"`
	RouteSlotID     string                          `json:"route_slot_id"`
	ComputerVersion computerversion.ComputerVersion `json:"computer_version"`
	Payload         json.RawMessage                 `json:"payload"`
	PayloadSHA256   string                          `json:"payload_sha256"`
	CreatedAt       time.Time                       `json:"created_at"`
}

const (
	TransitionBootstrap TransitionKind = "bootstrap"
	TransitionPromote   TransitionKind = "promote"
	TransitionRollback  TransitionKind = "rollback"
)

type Slot struct {
	ID              string                          `json:"route_slot_id"`
	Current         computerversion.ComputerVersion `json:"current_computer_version"`
	Generation      uint64                          `json:"generation"`
	LatestReceiptID ReceiptID                       `json:"latest_receipt_id"`
}

type TransitionCommand struct {
	RouteSlotID             string                          `json:"route_slot_id"`
	Kind                    TransitionKind                  `json:"transition_kind"`
	Old                     computerversion.ComputerVersion `json:"old_computer_version"`
	New                     computerversion.ComputerVersion `json:"new_computer_version"`
	ExpectedGeneration      uint64                          `json:"expected_generation"`
	ApprovalRef             ApprovalRef                     `json:"approval_ref"`
	PromotionCertificateRef PromotionCertificateRef         `json:"promotion_certificate_ref"`
	RollbackTargetReceiptID ReceiptID                       `json:"rollback_target_receipt_id,omitempty"`
	IdempotencyKey          IdempotencyKey                  `json:"idempotency_key"`
}

type TransitionReceipt struct {
	ID                      ReceiptID                       `json:"receipt_id"`
	RouteSlotID             string                          `json:"route_slot_id"`
	Kind                    TransitionKind                  `json:"transition_kind"`
	Old                     computerversion.ComputerVersion `json:"old_computer_version"`
	New                     computerversion.ComputerVersion `json:"new_computer_version"`
	ExpectedGeneration      uint64                          `json:"expected_generation"`
	CommittedGeneration     uint64                          `json:"committed_generation"`
	ApprovalRef             ApprovalRef                     `json:"approval_ref"`
	PromotionCertificateRef PromotionCertificateRef         `json:"promotion_certificate_ref"`
	RollbackTargetReceiptID ReceiptID                       `json:"rollback_target_receipt_id,omitempty"`
	IdempotencyKey          IdempotencyKey                  `json:"idempotency_key"`
	CommittedAt             time.Time                       `json:"committed_at"`
}

type Ledger interface {
	Resolve(context.Context, string) (Slot, TransitionReceipt, error)
	Transition(context.Context, TransitionCommand) (Slot, TransitionReceipt, error)
}

func RouteSlotID(ownerID, computerID string) (string, error) {
	ownerID = strings.TrimSpace(ownerID)
	computerID = strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return "", fmt.Errorf("route ledger: owner and computer IDs are required")
	}
	if strings.ContainsAny(ownerID, ":\x00\r\n") || strings.ContainsAny(computerID, ":\x00\r\n") {
		return "", fmt.Errorf("route ledger: invalid owner or computer ID")
	}
	return "computer:" + ownerID + ":" + computerID, nil
}

func ParseRouteSlotID(slotID string) (ownerID, computerID string, err error) {
	parts := strings.Split(strings.TrimSpace(slotID), ":")
	if len(parts) != 3 || parts[0] != "computer" || parts[1] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("route ledger: invalid route slot ID")
	}
	return parts[1], parts[2], nil
}

func (c TransitionCommand) normalized() TransitionCommand {
	c.RouteSlotID = strings.TrimSpace(c.RouteSlotID)
	c.Old.CodeRef = computerversion.CodeRef(strings.TrimSpace(string(c.Old.CodeRef)))
	c.Old.ArtifactProgramRef = computerversion.ArtifactProgramRef(strings.TrimSpace(string(c.Old.ArtifactProgramRef)))
	c.New.CodeRef = computerversion.CodeRef(strings.TrimSpace(string(c.New.CodeRef)))
	c.New.ArtifactProgramRef = computerversion.ArtifactProgramRef(strings.TrimSpace(string(c.New.ArtifactProgramRef)))
	c.IdempotencyKey = IdempotencyKey(strings.TrimSpace(string(c.IdempotencyKey)))
	c.ApprovalRef = ApprovalRef(strings.TrimSpace(string(c.ApprovalRef)))
	c.PromotionCertificateRef = PromotionCertificateRef(strings.TrimSpace(string(c.PromotionCertificateRef)))
	c.RollbackTargetReceiptID = ReceiptID(strings.TrimSpace(string(c.RollbackTargetReceiptID)))
	return c
}

func (c TransitionCommand) Validate() error {
	c = c.normalized()
	if c.RouteSlotID == "" {
		return fmt.Errorf("route ledger: route slot ID is required")
	}
	if _, _, err := ParseRouteSlotID(c.RouteSlotID); err != nil {
		return err
	}
	if !validEvidenceRef(string(c.IdempotencyKey), "idempotency:") {
		return fmt.Errorf("route ledger: typed idempotency key is required")
	}
	if !c.New.Valid() {
		return fmt.Errorf("route ledger: new ComputerVersion is invalid")
	}
	if !validHashEvidenceRef(string(c.ApprovalRef), "approval:sha256:") || !validHashEvidenceRef(string(c.PromotionCertificateRef), "certificate:sha256:") {
		return fmt.Errorf("route ledger: typed approval and promotion certificate refs are required")
	}
	switch c.Kind {
	case TransitionBootstrap:
		if c.ExpectedGeneration != 0 || c.Old.Valid() || c.RollbackTargetReceiptID != "" {
			return fmt.Errorf("route ledger: bootstrap requires generation zero and no old ComputerVersion")
		}
	case TransitionPromote:
		if !c.Old.Valid() || c.RollbackTargetReceiptID != "" {
			return fmt.Errorf("route ledger: promote requires an old ComputerVersion")
		}
	case TransitionRollback:
		if !c.Old.Valid() || !validReceiptID(c.RollbackTargetReceiptID) {
			return fmt.Errorf("route ledger: rollback requires old ComputerVersion and prior receipt ref")
		}
	default:
		return fmt.Errorf("route ledger: unsupported transition kind %q", c.Kind)
	}
	return nil
}

func NewAuthorizationEvidence(kind AuthorizationEvidenceKind, routeSlotID string, version computerversion.ComputerVersion, payload json.RawMessage, createdAt time.Time) (AuthorizationEvidence, error) {
	payloadCopy := append(json.RawMessage(nil), payload...)
	payloadDigest := sha256.Sum256(payloadCopy)
	evidence := AuthorizationEvidence{
		Kind: kind, RouteSlotID: strings.TrimSpace(routeSlotID), ComputerVersion: version,
		Payload: payloadCopy, PayloadSHA256: hex.EncodeToString(payloadDigest[:]), CreatedAt: createdAt.UTC(),
	}
	if err := evidence.validatePayload(); err != nil {
		return AuthorizationEvidence{}, err
	}
	payload, err := authorizationEvidencePayload(evidence)
	if err != nil {
		return AuthorizationEvidence{}, err
	}
	prefix := "approval:sha256:"
	if kind == AuthorizationEvidencePromotionCertificate {
		prefix = "certificate:sha256:"
	}
	digest := sha256.Sum256(payload)
	evidence.Ref = prefix + hex.EncodeToString(digest[:])
	return evidence, nil
}

func (e AuthorizationEvidence) Validate() error {
	if err := e.validatePayload(); err != nil {
		return err
	}
	payload, err := authorizationEvidencePayload(e)
	if err != nil {
		return err
	}
	prefix := "approval:sha256:"
	if e.Kind == AuthorizationEvidencePromotionCertificate {
		prefix = "certificate:sha256:"
	}
	digest := sha256.Sum256(payload)
	if e.Ref != prefix+hex.EncodeToString(digest[:]) {
		return fmt.Errorf("route ledger: authorization evidence ref hash mismatch")
	}
	return nil
}

func (e AuthorizationEvidence) validatePayload() error {
	if _, _, err := ParseRouteSlotID(e.RouteSlotID); err != nil {
		return err
	}
	if !e.ComputerVersion.Valid() || len(e.Payload) == 0 || !json.Valid(e.Payload) || len(e.PayloadSHA256) != 64 {
		return fmt.Errorf("route ledger: authorization evidence payload is incomplete")
	}
	payloadDigest := sha256.Sum256(e.Payload)
	if hex.EncodeToString(payloadDigest[:]) != e.PayloadSHA256 {
		return fmt.Errorf("route ledger: authorization evidence payload hash is invalid")
	}
	if e.CreatedAt.IsZero() {
		return fmt.Errorf("route ledger: authorization evidence creation time is required")
	}
	if e.Kind != AuthorizationEvidenceApproval && e.Kind != AuthorizationEvidencePromotionCertificate {
		return fmt.Errorf("route ledger: authorization evidence kind is invalid")
	}
	return nil
}

func authorizationEvidencePayload(e AuthorizationEvidence) ([]byte, error) {
	return json.Marshal(struct {
		Kind            AuthorizationEvidenceKind       `json:"evidence_kind"`
		RouteSlotID     string                          `json:"route_slot_id"`
		ComputerVersion computerversion.ComputerVersion `json:"computer_version"`
		PayloadSHA256   string                          `json:"payload_sha256"`
		CreatedAt       time.Time                       `json:"created_at"`
	}{e.Kind, e.RouteSlotID, e.ComputerVersion, e.PayloadSHA256, e.CreatedAt.UTC()})
}

type TransitionEvidenceResolver interface {
	VerifyTransitionEvidence(context.Context, TransitionCommand) error
}

type TransitionEvidenceCatalog interface {
	TransitionEvidenceResolver
	PinAuthorizationEvidence(context.Context, AuthorizationEvidence) (AuthorizationEvidence, error)
}

// AtomicTransitionEvidenceCatalog fate-shares authorization evidence and route
// mutation in one transaction. Production route publication must use this path.
type AtomicTransitionEvidenceCatalog interface {
	TransitionEvidenceCatalog
	TransitionWithEvidence(context.Context, TransitionCommand, []AuthorizationEvidence) (Slot, TransitionReceipt, error)
}

func validHashEvidenceRef(value, prefix string) bool {
	return strings.HasPrefix(value, prefix) && len(value) == len(prefix)+64 && validHexString(value[len(prefix):])
}

func validHexString(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func validEvidenceRef(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) <= len(prefix) || len(value) > 512 {
		return false
	}
	return validToken(value)
}

func validToken(value string) bool {
	if value == "" || len(value) > 512 {
		return false
	}
	return !strings.ContainsAny(value, " \t\r\n\x00")
}

func validReceiptID(id ReceiptID) bool {
	_, err := uuid.Parse(string(id))
	return err == nil
}

func (r TransitionReceipt) Validate() error {
	if !validReceiptID(r.ID) {
		return fmt.Errorf("route ledger: persisted receipt ID is invalid")
	}
	if _, _, err := ParseRouteSlotID(r.RouteSlotID); err != nil {
		return err
	}
	if !r.New.Valid() || r.CommittedGeneration == 0 || r.CommittedAt.IsZero() {
		return fmt.Errorf("route ledger: persisted receipt is incomplete")
	}
	if !validHashEvidenceRef(string(r.ApprovalRef), "approval:sha256:") || !validHashEvidenceRef(string(r.PromotionCertificateRef), "certificate:sha256:") || !validEvidenceRef(string(r.IdempotencyKey), "idempotency:") {
		return fmt.Errorf("route ledger: persisted receipt evidence is invalid")
	}
	switch r.Kind {
	case TransitionBootstrap:
		if r.ExpectedGeneration != 0 || r.Old.Valid() || r.CommittedGeneration != 1 || r.RollbackTargetReceiptID != "" {
			return fmt.Errorf("route ledger: persisted bootstrap receipt is invalid")
		}
	case TransitionPromote:
		if !r.Old.Valid() || r.RollbackTargetReceiptID != "" || r.CommittedGeneration != r.ExpectedGeneration+1 {
			return fmt.Errorf("route ledger: persisted promotion receipt is invalid")
		}
	case TransitionRollback:
		if !r.Old.Valid() || !validReceiptID(r.RollbackTargetReceiptID) || r.CommittedGeneration != r.ExpectedGeneration+1 {
			return fmt.Errorf("route ledger: persisted rollback receipt is invalid")
		}
	default:
		return fmt.Errorf("route ledger: persisted receipt kind is invalid")
	}
	return nil
}

func SameVersion(a, b computerversion.ComputerVersion) bool {
	return a.CodeRef == b.CodeRef && a.ArtifactProgramRef == b.ArtifactProgramRef
}

// MemoryLedger is a deterministic contract implementation for focused tests.
// Production must use SQLLedger so route authority survives process restart.
type MemoryLedger struct {
	mu       sync.Mutex
	slots    map[string]Slot
	receipts map[ReceiptID]TransitionReceipt
	byKey    map[IdempotencyKey]ReceiptID
	evidence map[string]AuthorizationEvidence
	now      func() time.Time
}

func NewMemoryLedger() *MemoryLedger {
	return &MemoryLedger{
		slots:    make(map[string]Slot),
		receipts: make(map[ReceiptID]TransitionReceipt),
		byKey:    make(map[IdempotencyKey]ReceiptID),
		evidence: make(map[string]AuthorizationEvidence),
		now:      time.Now,
	}
}

func (l *MemoryLedger) Resolve(ctx context.Context, slotID string) (Slot, TransitionReceipt, error) {
	if err := ctx.Err(); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	slot, ok := l.slots[strings.TrimSpace(slotID)]
	if !ok {
		return Slot{}, TransitionReceipt{}, ErrSlotNotFound
	}
	return slot, l.receipts[slot.LatestReceiptID], nil
}

func (l *MemoryLedger) Transition(ctx context.Context, command TransitionCommand) (Slot, TransitionReceipt, error) {
	if err := ctx.Err(); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	command = command.normalized()
	if err := command.Validate(); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.transitionLocked(command)
}

func (l *MemoryLedger) transitionLocked(command TransitionCommand) (Slot, TransitionReceipt, error) {
	if receiptID, ok := l.byKey[command.IdempotencyKey]; ok {
		receipt := l.receipts[receiptID]
		if !receiptMatchesCommand(receipt, command) {
			return Slot{}, TransitionReceipt{}, ErrIdempotencyReuse
		}
		return l.slots[command.RouteSlotID], receipt, nil
	}
	current, exists := l.slots[command.RouteSlotID]
	if !exists {
		if command.Kind != TransitionBootstrap {
			return Slot{}, TransitionReceipt{}, ErrSlotNotFound
		}
	} else if command.Kind == TransitionBootstrap || current.Generation != command.ExpectedGeneration || !SameVersion(current.Current, command.Old) {
		return Slot{}, TransitionReceipt{}, ErrStaleTransition
	}
	if command.Kind == TransitionRollback {
		target, ok := l.receipts[command.RollbackTargetReceiptID]
		if !ok || target.RouteSlotID != command.RouteSlotID || !SameVersion(target.New, command.New) || target.CommittedGeneration >= current.Generation {
			return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: rollback target receipt does not prove the requested prior ComputerVersion")
		}
	}
	committedGeneration := uint64(1)
	if exists {
		committedGeneration = current.Generation + 1
	}
	receipt := newReceipt(command, committedGeneration, l.now().UTC())
	slot := Slot{ID: command.RouteSlotID, Current: command.New, Generation: committedGeneration, LatestReceiptID: receipt.ID}
	l.receipts[receipt.ID] = receipt
	l.byKey[command.IdempotencyKey] = receipt.ID
	l.slots[slot.ID] = slot
	return slot, receipt, nil
}

func (l *MemoryLedger) PinAuthorizationEvidence(ctx context.Context, evidence AuthorizationEvidence) (AuthorizationEvidence, error) {
	if err := ctx.Err(); err != nil {
		return AuthorizationEvidence{}, err
	}
	if err := evidence.Validate(); err != nil {
		return AuthorizationEvidence{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if existing, ok := l.evidence[evidence.Ref]; ok && !authorizationEvidenceEqual(existing, evidence) {
		return AuthorizationEvidence{}, fmt.Errorf("route ledger: authorization evidence ref collision")
	}
	l.evidence[evidence.Ref] = evidence
	return evidence, nil
}

func (l *MemoryLedger) ResolveAuthorizationEvidence(ctx context.Context, ref string) (AuthorizationEvidence, error) {
	if err := ctx.Err(); err != nil {
		return AuthorizationEvidence{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	evidence, ok := l.evidence[ref]
	if !ok {
		return AuthorizationEvidence{}, fmt.Errorf("authorization evidence not found")
	}
	return evidence, nil
}

func (l *MemoryLedger) VerifyTransitionEvidence(ctx context.Context, command TransitionCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return verifyTransitionEvidenceMap(l.evidence, command)
}

func (l *MemoryLedger) TransitionWithEvidence(ctx context.Context, command TransitionCommand, evidence []AuthorizationEvidence) (Slot, TransitionReceipt, error) {
	if err := ctx.Err(); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	command = command.normalized()
	if err := command.Validate(); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	if len(evidence) == 0 {
		return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: atomic transition evidence is required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	combined := make(map[string]AuthorizationEvidence, len(l.evidence)+len(evidence))
	for ref, item := range l.evidence {
		combined[ref] = item
	}
	for _, item := range evidence {
		if err := item.Validate(); err != nil {
			return Slot{}, TransitionReceipt{}, err
		}
		if existing, ok := combined[item.Ref]; ok && !authorizationEvidenceEqual(existing, item) {
			return Slot{}, TransitionReceipt{}, fmt.Errorf("route ledger: authorization evidence ref collision")
		}
		combined[item.Ref] = item
	}
	if err := verifyTransitionEvidenceMap(combined, command); err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	slot, receipt, err := l.transitionLocked(command)
	if err != nil {
		return Slot{}, TransitionReceipt{}, err
	}
	l.evidence = combined
	return slot, receipt, nil
}

func verifyTransitionEvidenceMap(evidence map[string]AuthorizationEvidence, command TransitionCommand) error {
	approval, approvalOK := evidence[string(command.ApprovalRef)]
	certificate, certificateOK := evidence[string(command.PromotionCertificateRef)]
	if !approvalOK || !certificateOK || approval.Kind != AuthorizationEvidenceApproval || certificate.Kind != AuthorizationEvidencePromotionCertificate || approval.RouteSlotID != command.RouteSlotID || certificate.RouteSlotID != command.RouteSlotID || !SameVersion(approval.ComputerVersion, command.New) || !SameVersion(certificate.ComputerVersion, command.New) {
		return fmt.Errorf("route ledger: authorization evidence does not bind the requested route and ComputerVersion")
	}
	return nil
}

func authorizationEvidenceEqual(left, right AuthorizationEvidence) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

func newReceipt(command TransitionCommand, generation uint64, committedAt time.Time) TransitionReceipt {
	return TransitionReceipt{
		ID:                      ReceiptID(uuid.NewString()),
		RouteSlotID:             command.RouteSlotID,
		Kind:                    command.Kind,
		Old:                     command.Old,
		New:                     command.New,
		ExpectedGeneration:      command.ExpectedGeneration,
		CommittedGeneration:     generation,
		ApprovalRef:             ApprovalRef(strings.TrimSpace(string(command.ApprovalRef))),
		PromotionCertificateRef: PromotionCertificateRef(strings.TrimSpace(string(command.PromotionCertificateRef))),
		RollbackTargetReceiptID: ReceiptID(strings.TrimSpace(string(command.RollbackTargetReceiptID))),
		IdempotencyKey:          IdempotencyKey(strings.TrimSpace(string(command.IdempotencyKey))),
		CommittedAt:             committedAt,
	}
}

// ReceiptMatchesCommand verifies that an append-only receipt records the exact
// normalized transition command, including its independent approval, promotion,
// rollback, and idempotency evidence.
func ReceiptMatchesCommand(receipt TransitionReceipt, command TransitionCommand) bool {
	return receiptMatchesCommand(receipt, command.normalized())
}

func receiptMatchesCommand(receipt TransitionReceipt, command TransitionCommand) bool {
	return receipt.RouteSlotID == command.RouteSlotID && receipt.Kind == command.Kind &&
		SameVersion(receipt.Old, command.Old) && SameVersion(receipt.New, command.New) &&
		receipt.ExpectedGeneration == command.ExpectedGeneration &&
		receipt.ApprovalRef == ApprovalRef(strings.TrimSpace(string(command.ApprovalRef))) &&
		receipt.PromotionCertificateRef == PromotionCertificateRef(strings.TrimSpace(string(command.PromotionCertificateRef))) &&
		receipt.RollbackTargetReceiptID == ReceiptID(strings.TrimSpace(string(command.RollbackTargetReceiptID))) &&
		receipt.IdempotencyKey == IdempotencyKey(strings.TrimSpace(string(command.IdempotencyKey)))
}
