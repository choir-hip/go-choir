package computerevent

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	ErrNeedsProjectionRepair          = errors.New("computer event projection repair required")
	ErrSupervisionWritesDisabled      = errors.New("supervision writes disabled")
	ErrSupervisionIdempotencyConflict = errors.New("supervision command idempotency conflict")
)

const (
	SupervisionTransactionMediaTypeV1   = "application/vnd.choir.supervision-transaction.v1+json"
	SupervisionEvidenceMediaTypeV1      = "application/vnd.choir.supervision-evidence.v1+json"
	ProjectionImportMediaTypeV1         = "application/vnd.choir.projection-import.v1+json"
	ProjectionImportEvidenceMediaTypeV1 = "application/vnd.choir.projection-import-evidence.v1+json"
)

type PinResult struct {
	ArtifactDigest string
	Receipt        Receipt
}

// PrivateSupervisionArtifact is an immutable owner-private input that may be
// named by a later supervision transaction. Pinning it does not append an
// event or grant any semantic authority.
type PrivateSupervisionArtifact struct {
	Ref             ArtifactRef `json:"ref"`
	ArtifactDigest  string      `json:"artifact_digest"`
	PlaintextDigest string      `json:"plaintext_digest"`
	MediaType       string      `json:"media_type"`
	BindingID       string      `json:"binding_id"`
	Receipt         Receipt     `json:"pin_receipt"`
}

// PrivateSupervisionArtifactPayload is plaintext bound into one supervision
// command. BindingID is the stable logical identity used by the command digest;
// the private content address is derived only after that digest is reserved.
type PrivateSupervisionArtifactPayload struct {
	BindingID string
	Plaintext []byte
	MediaType string
	Finalize  func(map[string]string) ([]byte, map[string]string, error)
}

type ArtifactPinner interface {
	PinEvent(ctx context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (PinResult, error)
}

type NonPrivatePayloadPinner interface {
	PinNonPrivatePayload(ctx context.Context, computerID string, payload []byte, mediaType, privacyClass, pinIntentCommitment string) (PinResult, error)
}
type PrivatePayloadPinner interface {
	PreparePrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID, mediaType string, plaintext []byte) ([]byte, PrivateArtifactMetadata, error)
	PinPrivatePayload(ctx context.Context, cipher *PrivateArtifactCipher, computerID, eventID string, envelope []byte, pinIntentCommitment string) (PinResult, error)
}

type ProjectionStore interface {
	Head(ctx context.Context, computerID string) (*Head, error)
	Prepare(ctx context.Context, request CASRequest) error
	Prepared(ctx context.Context, computerID string) ([]CASRequest, error)
	Finalize(ctx context.Context, computerID, eventDigest string, receipt Receipt) error
	DiscardPrepared(ctx context.Context, computerID, eventDigest string) error
}

// SupervisionCommandStore provides the pre-pin idempotency lookup required to
// avoid creating a second private ciphertext for an accepted retry.
type SupervisionCommandStore interface {
	SupervisionCommand(ctx context.Context, computerID, commandID string) (receipt Receipt, artifactDigest, commandDigest string, found bool, err error)
}

// FinalizedSupervisionTransactionStore returns the exact accepted transaction
// so an accepted retry can recover referenced artifacts without encryption.
type FinalizedSupervisionTransactionStore interface {
	FinalizedSupervisionTransaction(ctx context.Context, computerID, commandID string) (SupervisionTransaction, bool, error)
}

// SupervisionReservationStore durably claims a command digest before the
// caller creates event entropy or a private payload.
type SupervisionReservationStore interface {
	ReserveSupervisionCommand(ctx context.Context, computerID, commandID, commandDigest string) (receipt Receipt, artifactDigest string, finalized bool, err error)
}

// SupervisionReservationLookup reports a non-final reservation without
// requiring callers to reconstruct its command digest.
type SupervisionReservationLookup interface {
	PendingSupervisionReservation(ctx context.Context, computerID, commandID string) (bool, error)
}

// PrepinSupervisionInputStore atomically claims a logical command and its
// encrypted pre-pin inputs, closing the reservation-to-freeze crash window.
type PrepinSupervisionInputStore interface {
	ReserveFrozenSupervisionInputs(ctx context.Context, computerID, commandID, commandDigest string, plan FrozenSupervisionPlan) (Receipt, string, bool, error)
}

// FrozenPrivateSupervisionInput is an encrypted, unpinned private payload
// frozen with its command before any external pin can occur.
type FrozenPrivateSupervisionInput struct {
	BindingID       string `json:"binding_id"`
	MediaType       string `json:"media_type"`
	Envelope        []byte `json:"envelope"`
	ArtifactDigest  string `json:"artifact_digest"`
	PlaintextDigest string `json:"plaintext_digest"`
}

// FrozenSupervisionPlan contains every entropy-bearing and private-payload
// value that must survive an interruption between reservation and preparation.
// The append head is intentionally excluded: sequence and previous head are
// rebased under the appender lock and are not part of a private pin intent.
type FrozenSupervisionPlan struct {
	EventID             string                          `json:"event_id"`
	OccurredAt          string                          `json:"occurred_at"`
	Transaction         SupervisionTransaction          `json:"transaction"`
	Envelope            []byte                          `json:"envelope"`
	ArtifactDigest      string                          `json:"artifact_digest"`
	ArtifactRef         string                          `json:"artifact_ref"`
	PinIntentCommitment string                          `json:"pin_intent_commitment"`
	PinReceipt          *Receipt                        `json:"pin_receipt"`
	PrivateInputs       []FrozenPrivateSupervisionInput `json:"private_inputs,omitempty"`
}

// ValidatePrivateInputs verifies the durable, encrypted pre-pin recovery
// material before any retry is allowed to contact an external pinner.
func (p FrozenSupervisionPlan) ValidatePrivateInputs(computerID, commandID, commandDigest string) error {
	computed, err := p.Transaction.ComputeCommandDigest()
	if err != nil || computed != commandDigest || strings.TrimSpace(computerID) == "" ||
		strings.TrimSpace(commandID) == "" || !IsSHA256(commandDigest) ||
		p.Transaction.ComputerID != computerID || p.Transaction.CommandID != commandID ||
		p.Transaction.CommandDigest != commandDigest || len(p.PrivateInputs) == 0 ||
		len(p.Transaction.ReferencedArtifacts) != len(p.PrivateInputs) ||
		p.EventID != "" || p.OccurredAt != "" || len(p.Envelope) != 0 ||
		p.ArtifactDigest != "" || p.ArtifactRef != "" || p.PinIntentCommitment != "" || p.PinReceipt != nil {
		return fmt.Errorf("frozen supervision inputs: scope or command mismatch")
	}
	synthetic := p.Transaction
	synthetic.ReferencedArtifacts = append([]ReferencedArtifact(nil), p.Transaction.ReferencedArtifacts...)
	for index := range synthetic.ReferencedArtifacts {
		synthetic.ReferencedArtifacts[index].PinReceipt = Receipt{ReceiptKind: "PinReceipt"}
	}
	if err := synthetic.Validate(); err != nil {
		return fmt.Errorf("frozen supervision inputs: invalid transaction: %w", err)
	}
	artifacts := make(map[string]ReferencedArtifact, len(p.Transaction.ReferencedArtifacts))
	for _, artifact := range p.Transaction.ReferencedArtifacts {
		if strings.TrimSpace(artifact.BindingID) == "" || artifact.PinReceipt.ReceiptID != "" {
			return fmt.Errorf("frozen supervision inputs: invalid unpinned artifact")
		}
		if _, duplicate := artifacts[artifact.BindingID]; duplicate {
			return fmt.Errorf("frozen supervision inputs: duplicate artifact binding %q", artifact.BindingID)
		}
		artifacts[artifact.BindingID] = artifact
	}
	seen := make(map[string]struct{}, len(p.PrivateInputs))
	for _, input := range p.PrivateInputs {
		artifact, found := artifacts[input.BindingID]
		ref, refErr := ArtifactRefFromDigest(input.ArtifactDigest)
		if strings.TrimSpace(input.BindingID) == "" || strings.TrimSpace(input.MediaType) == "" ||
			!IsSHA256(input.ArtifactDigest) || !IsSHA256(input.PlaintextDigest) ||
			DigestBytes(input.Envelope) != input.ArtifactDigest || refErr != nil || !found ||
			artifact.Ref != ref.String() || artifact.ArtifactDigest != input.ArtifactDigest ||
			artifact.PlaintextDigest != input.PlaintextDigest || artifact.MediaType != input.MediaType ||
			!IsSHA256(artifact.LogicalPlaintextDigest) {
			return fmt.Errorf("frozen supervision inputs: invalid encrypted input")
		}
		if _, duplicate := seen[input.BindingID]; duplicate {
			return fmt.Errorf("frozen supervision inputs: duplicate binding %q", input.BindingID)
		}
		seen[input.BindingID] = struct{}{}
	}
	return nil
}

// FrozenSupervisionPlanStore persists and retrieves the exact private plan.
// Pin receipt storage is separate because it is acquired only after the plan
// itself has been made durable.
type FrozenSupervisionPlanStore interface {
	FrozenSupervisionPlan(ctx context.Context, computerID, commandID string) (FrozenSupervisionPlan, bool, error)
	FreezeSupervisionPlan(ctx context.Context, computerID, commandID, commandDigest string, plan FrozenSupervisionPlan) error
	RecordSupervisionPin(ctx context.Context, computerID, commandID, commandDigest string, receipt Receipt) error
}

type CASRequest struct {
	Event                    Event                   `json:"event"`
	EventDigest              string                  `json:"event_digest"`
	EventArtifactDigest      string                  `json:"event_artifact_digest"`
	EventPinReceiptDigest    string                  `json:"event_pin_receipt_digest"`
	PayloadPinReceiptDigests []string                `json:"payload_pin_receipt_digests"`
	PinIntentCommitment      string                  `json:"pin_intent_commitment"`
	Input                    TransitionInput         `json:"transition_input"`
	Next                     Head                    `json:"next_head"`
	SupervisionTransaction   *SupervisionTransaction `json:"-"`
}

type HeadCAS interface {
	Head(ctx context.Context, computerID string) (*Head, error)
	CompareAndSwap(ctx context.Context, request CASRequest) (Receipt, error)
}

type DurableEvent struct {
	Request CASRequest `json:"request"`
	Receipt Receipt    `json:"event_head_receipt"`
}

type EventSource interface {
	Events(ctx context.Context, computerID string, afterSequence uint64) ([]DurableEvent, error)
}

type ReceiptVerifier interface {
	VerifyEventHeadReceipt(ctx context.Context, receipt Receipt, request CASRequest) error
}

// PrivateArtifactSource retrieves authenticated private envelopes by their
// content address during an external canonical-chain reconstruction.
type PrivateArtifactSource interface {
	EventSource
	PrivateArtifact(ctx context.Context, computerID, artifactDigest string) ([]byte, PinResult, error)
}

type ArtifactPinReceiptVerifier interface {
	VerifyArtifactPinReceipt(ctx context.Context, receipt Receipt, computerID, artifactDigest string) error
}

// ComputerEventAppender is the sole semantic event sequencer in the trusted
// guest core. Its dependencies expose only mechanical pin, prepare, and CAS
// operations; agents, capsules, reducers, vmctl, and route projections never
// receive this object or its append capability.
type ComputerEventAppender struct {
	computerID string
	pins       ArtifactPinner
	projection ProjectionStore
	cas        HeadCAS
	verifier   ReceiptVerifier
	mu         sync.Mutex
}

// SupervisionWritesDisabled reports the process-wide break-glass gate. It is
// intentionally checked at every canonical supervision append rather than only
// while wiring the runtime, so no alternate caller can bypass it.
func SupervisionWritesDisabled() bool {
	return strings.TrimSpace(os.Getenv("CHOIR_SUPERVISION_WRITES_DISABLED")) != ""
}

// ReserveSupervisionTransaction creates the durable pre-entropy idempotency
// reservation for a command or returns its finalized original receipt.
func (a *ComputerEventAppender) ReserveSupervisionTransaction(ctx context.Context, transaction SupervisionTransaction) (Receipt, string, bool, error) {
	if SupervisionWritesDisabled() {
		return Receipt{}, "", false, ErrSupervisionWritesDisabled
	}
	if transaction.ComputerID != a.computerID {
		return Receipt{}, "", false, fmt.Errorf("computer event appender: supervision transaction targets wrong computer")
	}
	digest, err := transaction.ComputeCommandDigest()
	if err != nil {
		return Receipt{}, "", false, fmt.Errorf("computer event appender: compute supervision command digest: %w", err)
	}
	if transaction.CommandDigest != "" && transaction.CommandDigest != ZeroHead && transaction.CommandDigest != digest {
		return Receipt{}, "", false, fmt.Errorf("%w: command digest changed", ErrSupervisionIdempotencyConflict)
	}
	reservations, ok := a.projection.(SupervisionReservationStore)
	if !ok {
		return Receipt{}, "", false, fmt.Errorf("computer event appender: supervision reservation unavailable")
	}
	receipt, artifactDigest, finalized, err := reservations.ReserveSupervisionCommand(ctx, a.computerID, transaction.CommandID, digest)
	if err != nil {
		return Receipt{}, "", false, fmt.Errorf("computer event appender: reserve supervision command: %w", err)
	}
	return receipt, artifactDigest, finalized, nil
}

// RecoverFinalizedSupervisionTransaction returns the immutable accepted command
// before callers inspect or rebuild their mutable source state.
func (a *ComputerEventAppender) RecoverFinalizedSupervisionTransaction(ctx context.Context, commandID string) (Receipt, SupervisionTransaction, bool, error) {
	if a == nil {
		return Receipt{}, SupervisionTransaction{}, false, fmt.Errorf("computer event appender: unavailable")
	}
	finalized, ok := a.projection.(FinalizedSupervisionTransactionStore)
	if !ok {
		return Receipt{}, SupervisionTransaction{}, false, fmt.Errorf("computer event appender: finalized supervision transaction lookup unavailable")
	}
	transaction, found, err := finalized.FinalizedSupervisionTransaction(ctx, a.computerID, commandID)
	if err != nil || !found {
		return Receipt{}, SupervisionTransaction{}, found, err
	}
	if transaction.CommandID != commandID || transaction.ComputerID != a.computerID {
		return Receipt{}, SupervisionTransaction{}, false, fmt.Errorf("%w: finalized supervision transaction scope mismatch", ErrNeedsProjectionRepair)
	}
	commands, ok := a.projection.(SupervisionCommandStore)
	if !ok {
		return Receipt{}, SupervisionTransaction{}, false, fmt.Errorf("computer event appender: supervision command lookup unavailable")
	}
	receipt, _, _, accepted, err := commands.SupervisionCommand(ctx, a.computerID, commandID)
	if err != nil {
		return Receipt{}, SupervisionTransaction{}, false, err
	}
	if !accepted {
		return Receipt{}, SupervisionTransaction{}, false, fmt.Errorf("%w: finalized supervision receipt is missing", ErrNeedsProjectionRepair)
	}
	return receipt, transaction, true, nil
}

// RecoverFrozenSupervisionTransaction returns a reserved pre-finalization
// command exactly as persisted, including its time-bearing evidence bindings.
func (a *ComputerEventAppender) RecoverFrozenSupervisionTransaction(ctx context.Context, commandID string) (SupervisionTransaction, bool, error) {
	if a == nil {
		return SupervisionTransaction{}, false, fmt.Errorf("computer event appender: unavailable")
	}
	plans, ok := a.projection.(FrozenSupervisionPlanStore)
	if !ok {
		return SupervisionTransaction{}, false, fmt.Errorf("computer event appender: frozen supervision plan storage unavailable")
	}
	plan, found, err := plans.FrozenSupervisionPlan(ctx, a.computerID, commandID)
	if err != nil || !found {
		return SupervisionTransaction{}, found, err
	}
	if plan.Transaction.CommandID != commandID || plan.Transaction.ComputerID != a.computerID {
		return SupervisionTransaction{}, false, fmt.Errorf("%w: frozen supervision transaction scope mismatch", ErrNeedsProjectionRepair)
	}
	return plan.Transaction, true, nil
}

// HasPendingSupervisionReservation reports a reserved command whose frozen
// replay inputs must be recovered rather than regenerated.
func (a *ComputerEventAppender) HasPendingSupervisionReservation(ctx context.Context, commandID string) (bool, error) {
	if a == nil {
		return false, fmt.Errorf("computer event appender: unavailable")
	}
	lookup, ok := a.projection.(SupervisionReservationLookup)
	if !ok {
		return false, fmt.Errorf("computer event appender: supervision reservation lookup unavailable")
	}
	return lookup.PendingSupervisionReservation(ctx, a.computerID, commandID)
}

// ResumeFrozenSupervisionTransaction completes a persisted command without
// rebuilding its caller-owned inputs or generating new evidence.
func (a *ComputerEventAppender) ResumeFrozenSupervisionTransaction(ctx context.Context, transaction SupervisionTransaction, cipher *PrivateArtifactCipher) (Receipt, error) {
	if transaction.CommandID == "" || transaction.ComputerID != a.computerID {
		return Receipt{}, fmt.Errorf("computer event appender: frozen supervision transaction scope mismatch")
	}
	plans, ok := a.projection.(FrozenSupervisionPlanStore)
	if !ok {
		return Receipt{}, fmt.Errorf("computer event appender: frozen supervision plan storage unavailable")
	}
	plan, found, err := plans.FrozenSupervisionPlan(ctx, a.computerID, transaction.CommandID)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: load frozen supervision transaction: %w", err)
	}
	if !found {
		return Receipt{}, fmt.Errorf("%w: frozen supervision transaction is missing", ErrNeedsProjectionRepair)
	}
	event := Event{
		SchemaVersion: SchemaVersionV1, ComputerID: a.computerID,
		EventKind: EventSupervisionTransaction, PrivacyClass: "private", ReducerVersion: ReducerVersionV1,
	}
	if len(plan.PrivateInputs) > 0 {
		receipt, _, _, err := a.resumeFrozenPrivateSupervisionPlan(ctx, event, TransitionInput{}, plan, cipher)
		return receipt, err
	}
	plan.Transaction.TransactionID = ""
	receipt, _, err := a.AppendNewSupervisionTransaction(ctx, event, TransitionInput{}, plan.Transaction, cipher)
	return receipt, err
}

func NewComputerEventAppender(computerID string, pins ArtifactPinner, projection ProjectionStore, cas HeadCAS, verifier ReceiptVerifier) (*ComputerEventAppender, error) {
	if computerID == "" || pins == nil || projection == nil || cas == nil || verifier == nil {
		return nil, fmt.Errorf("computer event appender: complete dependencies are required")
	}
	return &ComputerEventAppender{computerID: computerID, pins: pins, projection: projection, cas: cas, verifier: verifier}, nil
}

// pinReservedPrivateSupervisionArtifact encrypts deterministically and pins a
// private input only after its enclosing supervision command is reserved.
func (a *ComputerEventAppender) pinReservedPrivateSupervisionArtifact(ctx context.Context, bindingID string, plaintext []byte, mediaType string, cipher *PrivateArtifactCipher) (PrivateSupervisionArtifact, error) {
	if SupervisionWritesDisabled() {
		return PrivateSupervisionArtifact{}, ErrSupervisionWritesDisabled
	}
	if a == nil || strings.TrimSpace(bindingID) == "" || len(plaintext) == 0 || cipher == nil || strings.TrimSpace(mediaType) == "" {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: complete private supervision artifact inputs are required")
	}
	pinner, ok := a.pins.(PrivatePayloadPinner)
	if !ok {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: private payload authority unavailable")
	}
	plaintextDigest := DigestBytes(plaintext)
	intentBytes, err := CanonicalJSON(map[string]string{
		"computer_id": a.computerID, "binding_id": bindingID,
		"media_type": mediaType, "plaintext_digest": plaintextDigest,
	})
	if err != nil {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: canonical private artifact intent: %w", err)
	}
	pinIntentCommitment := DigestBytes(intentBytes)
	envelope, metadata, err := cipher.EncryptSupervisionDeterministic(ctx, a.computerID, bindingID, mediaType, plaintext)
	if err != nil {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: encrypt private supervision artifact: %w", err)
	}
	if metadata.ComputerID != a.computerID || metadata.EventID != bindingID || metadata.MediaType != mediaType || metadata.PrivacyClass != "private" {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: private artifact metadata mismatch")
	}
	verifiedPlaintext, verifiedMetadata, err := cipher.Decrypt(ctx, envelope, a.computerID, bindingID)
	metadataBytes, metadataErr := CanonicalJSON(metadata)
	verifiedMetadataBytes, verifiedMetadataErr := CanonicalJSON(verifiedMetadata)
	if err != nil || metadataErr != nil || verifiedMetadataErr != nil || !bytes.Equal(metadataBytes, verifiedMetadataBytes) || !bytes.Equal(verifiedPlaintext, plaintext) || DigestBytes(verifiedPlaintext) != plaintextDigest {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: private artifact plaintext verification failed")
	}
	artifactDigest := DigestBytes(envelope)
	pin, err := pinner.PinPrivatePayload(ctx, cipher, a.computerID, bindingID, envelope, pinIntentCommitment)
	if err != nil {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: pin private supervision artifact: %w", err)
	}
	if pin.ArtifactDigest != artifactDigest {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: pinned private artifact digest mismatch")
	}
	verifier, ok := a.verifier.(ArtifactPinReceiptVerifier)
	if !ok {
		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: artifact pin receipt verifier unavailable")
	}
	if err := verifier.VerifyArtifactPinReceipt(ctx, pin.Receipt, a.computerID, artifactDigest); err != nil {

		return PrivateSupervisionArtifact{}, fmt.Errorf("computer event appender: verify private artifact pin receipt: %w", err)
	}
	ref, err := ArtifactRefFromDigest(artifactDigest)
	if err != nil {
		return PrivateSupervisionArtifact{}, err
	}
	return PrivateSupervisionArtifact{Ref: ref, ArtifactDigest: artifactDigest, PlaintextDigest: plaintextDigest, MediaType: mediaType, BindingID: bindingID, Receipt: pin.Receipt}, nil
}

// PinPrivateEvidenceArtifact pins a standalone evidence artifact. It must not
// be used to stage a payload owned by the same supervision command; those use
// AppendNewSupervisionTransactionWithPrivateArtifacts.
func (a *ComputerEventAppender) PinPrivateEvidenceArtifact(ctx context.Context, bindingID string, plaintext []byte, mediaType string, cipher *PrivateArtifactCipher) (PrivateSupervisionArtifact, error) {
	return a.pinReservedPrivateSupervisionArtifact(ctx, bindingID, plaintext, mediaType, cipher)
}

// AppendNewSupervisionTransactionWithPrivateArtifacts reserves the complete
// logical command before encrypting any referenced payload. Stable placeholders
// make retries compare the same command even though final content addresses and
// pin receipts are filled only after reservation.
func (a *ComputerEventAppender) AppendNewSupervisionTransactionWithPrivateArtifacts(
	ctx context.Context,
	event Event,
	input TransitionInput,
	transaction SupervisionTransaction,
	payloads []PrivateSupervisionArtifactPayload,
	cipher *PrivateArtifactCipher,
) (Receipt, string, []PrivateSupervisionArtifact, error) {
	if len(payloads) == 0 {
		receipt, digest, err := a.AppendNewSupervisionTransaction(ctx, event, input, transaction, cipher)
		return receipt, digest, nil, err
	}
	if SupervisionWritesDisabled() {
		return Receipt{}, "", nil, ErrSupervisionWritesDisabled
	}
	if a == nil || cipher == nil {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: private supervision payload authority unavailable")
	}
	if len(transaction.ReferencedArtifacts) != 0 {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: generated and pre-pinned supervision artifacts cannot be mixed")
	}

	logical := transaction
	transaction.Mutations = append([]SupervisionMutation(nil), transaction.Mutations...)
	logical.ReferencedArtifacts = make([]ReferencedArtifact, 0, len(payloads))
	seenBindings := make(map[string]struct{}, len(payloads))
	logicalPlaintextDigests := make(map[string]string, len(payloads))
	for _, payload := range payloads {
		bindingID := strings.TrimSpace(payload.BindingID)
		mediaType := strings.TrimSpace(payload.MediaType)
		if bindingID == "" || mediaType == "" || len(payload.Plaintext) == 0 {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: complete private supervision artifact inputs are required")
		}
		if _, exists := seenBindings[bindingID]; exists {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: duplicate private supervision binding %q", bindingID)
		}
		seenBindings[bindingID] = struct{}{}
		logicalDigest := DigestBytes(payload.Plaintext)
		logicalPlaintextDigests[bindingID] = logicalDigest
		logical.ReferencedArtifacts = append(logical.ReferencedArtifacts, ReferencedArtifact{
			Ref: SupervisionArtifactPlaceholder(bindingID), ArtifactDigest: ZeroHead,
			PlaintextDigest: logicalDigest, LogicalPlaintextDigest: logicalDigest,
			MediaType: mediaType, BindingID: bindingID,
		})
	}
	commandDigest, err := logical.ComputeCommandDigest()
	if err != nil {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: compute logical supervision command: %w", err)
	}
	if transaction.CommandDigest != "" && transaction.CommandDigest != ZeroHead && transaction.CommandDigest != commandDigest {
		return Receipt{}, "", nil, fmt.Errorf("%w: command digest changed", ErrSupervisionIdempotencyConflict)
	}
	logical.CommandDigest = commandDigest
	if err := logical.ValidateLogical(); err != nil {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: validate logical supervision command: %w", err)
	}

	plans, ok := a.projection.(FrozenSupervisionPlanStore)
	if !ok {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: frozen supervision plan storage unavailable")
	}
	if frozen, found, loadErr := plans.FrozenSupervisionPlan(ctx, a.computerID, transaction.CommandID); loadErr != nil {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: load frozen private inputs: %w", loadErr)
	} else if found {
		frozenDigest, digestErr := frozen.Transaction.ComputeCommandDigest()
		if digestErr != nil || frozenDigest != commandDigest || frozen.Transaction.CommandDigest != commandDigest {
			return Receipt{}, "", nil, fmt.Errorf("%w: frozen private inputs changed", ErrSupervisionIdempotencyConflict)
		}
		return a.resumeFrozenPrivateSupervisionPlan(ctx, event, input, frozen, cipher)
	}

	replacements := make(map[string]string, len(payloads)*2)
	finalArtifacts := make([]ReferencedArtifact, 0, len(payloads))
	frozenInputs := make([]FrozenPrivateSupervisionInput, 0, len(payloads))
	for _, payload := range payloads {
		plaintext := payload.Plaintext
		if payload.Finalize != nil {
			var additional map[string]string
			plaintext, additional, err = payload.Finalize(replacements)
			if err != nil {
				return Receipt{}, "", nil, fmt.Errorf("computer event appender: finalize private artifact payload: %w", err)
			}
			for logicalValue, boundValue := range additional {
				replacements[logicalValue] = boundValue
			}
		}
		bindingID, mediaType := strings.TrimSpace(payload.BindingID), strings.TrimSpace(payload.MediaType)
		envelope, metadata, encryptErr := cipher.EncryptSupervisionDeterministic(ctx, a.computerID, bindingID, mediaType, plaintext)
		if encryptErr != nil {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: encrypt frozen private input: %w", encryptErr)
		}
		verified, verifiedMetadata, decryptErr := cipher.Decrypt(ctx, envelope, a.computerID, bindingID)
		metadataBytes, metadataErr := CanonicalJSON(metadata)
		verifiedMetadataBytes, verifiedMetadataErr := CanonicalJSON(verifiedMetadata)
		if decryptErr != nil || metadataErr != nil || verifiedMetadataErr != nil ||
			!bytes.Equal(verified, plaintext) || !bytes.Equal(metadataBytes, verifiedMetadataBytes) {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: verify frozen private input")
		}
		artifactDigest := DigestBytes(envelope)
		artifactRef, refErr := ArtifactRefFromDigest(artifactDigest)
		if refErr != nil {
			return Receipt{}, "", nil, refErr
		}
		plaintextDigest := DigestBytes(plaintext)
		placeholder := SupervisionArtifactPlaceholder(bindingID)
		replacements[placeholder] = artifactRef.String()
		replacements[logicalPlaintextDigests[bindingID]] = plaintextDigest
		frozenInputs = append(frozenInputs, FrozenPrivateSupervisionInput{
			BindingID: bindingID, MediaType: mediaType, Envelope: envelope,
			ArtifactDigest: artifactDigest, PlaintextDigest: plaintextDigest,
		})
		finalArtifacts = append(finalArtifacts, ReferencedArtifact{
			Ref: artifactRef.String(), ArtifactDigest: artifactDigest,
			PlaintextDigest: plaintextDigest, LogicalPlaintextDigest: logicalPlaintextDigests[bindingID],
			MediaType: mediaType, BindingID: bindingID,
		})
	}
	transaction.ReferencedArtifacts = finalArtifacts
	transaction.CommandDigest = commandDigest
	for index := range transaction.Mutations {
		body, replaceErr := replaceSupervisionArtifactRefs(transaction.Mutations[index].Body, replacements)
		if replaceErr != nil {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: bind frozen private artifact: %w", replaceErr)
		}
		transaction.Mutations[index].Body = body
	}
	finalDigest, err := transaction.ComputeCommandDigest()
	if err != nil || finalDigest != commandDigest {
		return Receipt{}, "", nil, fmt.Errorf("%w: bound private artifacts changed command digest", ErrSupervisionIdempotencyConflict)
	}
	plan := FrozenSupervisionPlan{Transaction: transaction, PrivateInputs: frozenInputs}
	prepin, ok := a.projection.(PrepinSupervisionInputStore)
	if !ok {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: frozen private input storage unavailable")
	}
	receipt, artifactDigest, finalized, err := prepin.ReserveFrozenSupervisionInputs(ctx, a.computerID, transaction.CommandID, commandDigest, plan)
	if err != nil {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: reserve frozen private inputs: %w", err)
	}
	if finalized {
		finalizedStore, ok := a.projection.(FinalizedSupervisionTransactionStore)
		if !ok {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: finalized supervision transaction lookup unavailable")
		}
		accepted, found, lookupErr := finalizedStore.FinalizedSupervisionTransaction(ctx, a.computerID, transaction.CommandID)
		if lookupErr != nil || !found {
			return Receipt{}, "", nil, fmt.Errorf("%w: finalized supervision transaction is missing", ErrNeedsProjectionRepair)
		}
		recovered, recoverErr := privateSupervisionArtifacts(accepted)
		return receipt, artifactDigest, recovered, recoverErr
	}
	return a.resumeFrozenPrivateSupervisionPlan(ctx, event, input, plan, cipher)
}

func (a *ComputerEventAppender) resumeFrozenPrivateSupervisionPlan(
	ctx context.Context,
	event Event,
	input TransitionInput,
	plan FrozenSupervisionPlan,
	cipher *PrivateArtifactCipher,
) (Receipt, string, []PrivateSupervisionArtifact, error) {
	if len(plan.PrivateInputs) == 0 {
		transaction := plan.Transaction
		transaction.TransactionID = ""
		artifacts, artifactsErr := privateSupervisionArtifacts(transaction)
		if artifactsErr != nil {
			return Receipt{}, "", nil, artifactsErr
		}
		receipt, digest, err := a.AppendNewSupervisionTransaction(ctx, event, input, transaction, cipher)
		return receipt, digest, artifacts, err
	}
	if err := plan.ValidatePrivateInputs(a.computerID, plan.Transaction.CommandID, plan.Transaction.CommandDigest); err != nil {
		return Receipt{}, "", nil, fmt.Errorf("%w: %v", ErrNeedsProjectionRepair, err)
	}
	pinner, pinnerOK := a.pins.(PrivatePayloadPinner)
	verifier, verifierOK := a.verifier.(ArtifactPinReceiptVerifier)
	if !pinnerOK || !verifierOK || cipher == nil {
		return Receipt{}, "", nil, fmt.Errorf("computer event appender: private supervision payload authority unavailable")
	}
	transaction := plan.Transaction
	transaction.ReferencedArtifacts = append([]ReferencedArtifact(nil), plan.Transaction.ReferencedArtifacts...)
	byBinding := make(map[string]int, len(transaction.ReferencedArtifacts))
	for index, artifact := range transaction.ReferencedArtifacts {
		byBinding[artifact.BindingID] = index
	}
	pinned := make([]PrivateSupervisionArtifact, 0, len(plan.PrivateInputs))
	for _, frozen := range plan.PrivateInputs {
		index, found := byBinding[frozen.BindingID]
		if !found {
			return Receipt{}, "", nil, fmt.Errorf("%w: frozen private input binding is missing", ErrNeedsProjectionRepair)
		}
		artifact := &transaction.ReferencedArtifacts[index]
		ref, refErr := ArtifactRefFromDigest(frozen.ArtifactDigest)
		plaintext, metadata, decryptErr := cipher.Decrypt(ctx, frozen.Envelope, a.computerID, frozen.BindingID)
		if refErr != nil || decryptErr != nil || DigestBytes(plaintext) != frozen.PlaintextDigest ||
			metadata.MediaType != frozen.MediaType || metadata.PrivacyClass != "private" ||
			artifact.Ref != ref.String() || artifact.ArtifactDigest != frozen.ArtifactDigest ||
			artifact.PlaintextDigest != frozen.PlaintextDigest || artifact.MediaType != frozen.MediaType {
			return Receipt{}, "", nil, fmt.Errorf("%w: frozen private input does not bind transaction", ErrNeedsProjectionRepair)
		}
		intentBytes, intentErr := CanonicalJSON(map[string]string{
			"computer_id": a.computerID, "binding_id": frozen.BindingID,
			"media_type": frozen.MediaType, "plaintext_digest": frozen.PlaintextDigest,
		})
		if intentErr != nil {
			return Receipt{}, "", nil, intentErr
		}
		pin, pinErr := pinner.PinPrivatePayload(ctx, cipher, a.computerID, frozen.BindingID, frozen.Envelope, DigestBytes(intentBytes))
		if pinErr != nil {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: pin frozen private input: %w", pinErr)
		}
		if pin.ArtifactDigest != frozen.ArtifactDigest {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: frozen private input digest mismatch")
		}
		if verifyErr := verifier.VerifyArtifactPinReceipt(ctx, pin.Receipt, a.computerID, frozen.ArtifactDigest); verifyErr != nil {
			return Receipt{}, "", nil, fmt.Errorf("computer event appender: verify frozen private input receipt: %w", verifyErr)
		}
		artifact.PinReceipt = pin.Receipt
		event.InputArtifactRefs = appendUniqueString(event.InputArtifactRefs, ref.String())
		pinned = append(pinned, PrivateSupervisionArtifact{
			Ref: ref, ArtifactDigest: frozen.ArtifactDigest, PlaintextDigest: frozen.PlaintextDigest,
			MediaType: frozen.MediaType, BindingID: frozen.BindingID, Receipt: pin.Receipt,
		})
	}
	transaction.TransactionID = ""
	receipt, digest, err := a.AppendNewSupervisionTransaction(ctx, event, input, transaction, cipher)
	return receipt, digest, pinned, err
}

func privateSupervisionArtifacts(transaction SupervisionTransaction) ([]PrivateSupervisionArtifact, error) {
	artifacts := make([]PrivateSupervisionArtifact, 0, len(transaction.ReferencedArtifacts))
	for _, artifact := range transaction.ReferencedArtifacts {
		ref, err := ParseArtifactRef(artifact.Ref)
		if err != nil {
			return nil, fmt.Errorf("%w: finalized supervision artifact is malformed", ErrNeedsProjectionRepair)
		}
		artifacts = append(artifacts, PrivateSupervisionArtifact{
			Ref: ref, ArtifactDigest: artifact.ArtifactDigest, PlaintextDigest: artifact.PlaintextDigest,
			MediaType: artifact.MediaType, BindingID: artifact.BindingID, Receipt: artifact.PinReceipt,
		})
	}
	return artifacts, nil
}
func (a *ComputerEventAppender) Append(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string) (Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendLocked(ctx, event, input, payloadPinReceiptDigests, nil)
}

// AppendNew serializes a new semantic event, binds it to the current canonical
// and effective projections, computes the exact request commitment, and appends
// it through the sole event writer.
func (a *ComputerEventAppender) AppendNew(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string) (Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.bindCurrentHeadLocked(ctx, &event); err != nil {
		return Receipt{}, err
	}
	event.RequestCommitment = ZeroHead
	pinIntentCommitment, err := ComputePinIntentCommitment(event, input)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: compute new pin intent: %w", err)
	}
	event.RequestCommitment, err = ComputeRequestCommitment(event, input, pinIntentCommitment, payloadPinReceiptDigests)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: compute new request commitment: %w", err)
	}
	return a.appendLocked(ctx, event, input, payloadPinReceiptDigests, nil)
}

// AppendNewPayload content-addresses and pins one non-private payload before
// appending the event that names it. The payload receipt is bound into the
// event request commitment; no process-local tape is authoritative.
func (a *ComputerEventAppender) AppendNewPayload(ctx context.Context, event Event, input TransitionInput, payload []byte, mediaType, privacyClass string) (Receipt, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pinner, ok := a.pins.(NonPrivatePayloadPinner)
	if !ok {
		return Receipt{}, "", fmt.Errorf("computer event appender: non-private payload pinning unavailable")
	}
	if err := a.bindCurrentHeadLocked(ctx, &event); err != nil {
		return Receipt{}, "", err
	}
	payloadDigest := DigestBytes(payload)
	artifactRef, err := ArtifactRefFromDigest(payloadDigest)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: create payload artifact ref: %w", err)
	}
	event.PayloadCommitment = payloadDigest
	event.OutputArtifactRefs = append(nonNilStrings(event.OutputArtifactRefs), artifactRef.String())
	if event.ProposedEffectRef == "" {
		event.ProposedEffectRef = payloadDigest
	}
	event.RequestCommitment = ZeroHead
	pinIntentCommitment, err := ComputePinIntentCommitment(event, input)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute payload pin intent: %w", err)
	}
	pin, err := pinner.PinNonPrivatePayload(ctx, a.computerID, payload, mediaType, privacyClass, pinIntentCommitment)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: pin payload: %w", err)
	}
	if pin.ArtifactDigest != payloadDigest {
		return Receipt{}, "", fmt.Errorf("computer event appender: pinned payload digest mismatch")
	}
	receiptBytes, err := pin.Receipt.CanonicalBytes()
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: canonical payload receipt: %w", err)
	}
	payloadReceiptDigest := DigestBytes(receiptBytes)
	event.RequestCommitment, err = ComputeRequestCommitment(event, input, pinIntentCommitment, []string{payloadReceiptDigest})
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute payload request commitment: %w", err)
	}
	receipt, err := a.appendLocked(ctx, event, input, []string{payloadReceiptDigest}, nil)
	return receipt, payloadDigest, err
}

// AppendNewPrivatePayload encrypts, authenticates, and pins one private payload
// before appending the event that names its immutable envelope.
func (a *ComputerEventAppender) AppendNewPrivatePayload(ctx context.Context, event Event, input TransitionInput, plaintext []byte, mediaType string, cipher *PrivateArtifactCipher) (Receipt, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	pinner, ok := a.pins.(PrivatePayloadPinner)
	if !ok || cipher == nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: private payload authority unavailable")
	}
	if err := a.bindCurrentHeadLocked(ctx, &event); err != nil {
		return Receipt{}, "", err
	}
	envelope, _, err := pinner.PreparePrivatePayload(ctx, cipher, a.computerID, event.EventID, mediaType, plaintext)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: encrypt private payload: %w", err)
	}
	payloadDigest := DigestBytes(envelope)
	artifactRef, err := ArtifactRefFromDigest(payloadDigest)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: create private payload artifact ref: %w", err)
	}
	event.PayloadCommitment = payloadDigest
	event.OutputArtifactRefs = append(nonNilStrings(event.OutputArtifactRefs), artifactRef.String())
	if event.ProposedEffectRef == "" {
		event.ProposedEffectRef = payloadDigest
	}
	event.RequestCommitment = ZeroHead
	pinIntentCommitment, err := ComputePinIntentCommitment(event, input)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute private payload pin intent: %w", err)
	}
	pin, err := pinner.PinPrivatePayload(ctx, cipher, a.computerID, event.EventID, envelope, pinIntentCommitment)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: pin private payload: %w", err)
	}
	if pin.ArtifactDigest != payloadDigest {
		return Receipt{}, "", fmt.Errorf("computer event appender: pinned private payload digest mismatch")
	}
	receiptBytes, err := pin.Receipt.CanonicalBytes()
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: canonical private payload receipt: %w", err)
	}
	payloadReceiptDigest := DigestBytes(receiptBytes)
	event.RequestCommitment, err = ComputeRequestCommitment(event, input, pinIntentCommitment, []string{payloadReceiptDigest})
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute private payload request commitment: %w", err)
	}
	receipt, err := a.appendLocked(ctx, event, input, []string{payloadReceiptDigest}, nil)
	return receipt, payloadDigest, err
}

type EventPayloadDirection string

const (
	EventPayloadInput  EventPayloadDirection = "input"
	EventPayloadOutput EventPayloadDirection = "output"
)

type EventPayload struct {
	Content      []byte
	MediaType    string
	PrivacyClass string
	Direction    EventPayloadDirection
	Private      bool
}

// AppendNewPayloadSet pins every input before every output and binds their
// canonical artifact references and pin receipts into one event append.
func (a *ComputerEventAppender) AppendNewPayloadSet(ctx context.Context, event Event, input TransitionInput, payloads []EventPayload, cipher *PrivateArtifactCipher) (Receipt, []string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(payloads) == 0 {
		return Receipt{}, nil, fmt.Errorf("computer event appender: payload set is empty")
	}
	if err := a.bindCurrentHeadLocked(ctx, &event); err != nil {
		return Receipt{}, nil, err
	}
	type preparedPayload struct {
		EventPayload
		stored []byte
		digest string
	}
	prepared := make([]preparedPayload, 0, len(payloads))
	outputCount := 0
	for _, direction := range []EventPayloadDirection{EventPayloadInput, EventPayloadOutput} {
		for _, payload := range payloads {
			if payload.Direction != direction || len(payload.Content) == 0 || payload.MediaType == "" {
				if payload.Direction != direction {
					continue
				}
				return Receipt{}, nil, fmt.Errorf("computer event appender: invalid payload set member")
			}
			stored := payload.Content
			if payload.Private {
				pinner, ok := a.pins.(PrivatePayloadPinner)
				if !ok || cipher == nil {
					return Receipt{}, nil, fmt.Errorf("computer event appender: private payload authority unavailable")
				}
				envelope, _, err := pinner.PreparePrivatePayload(ctx, cipher, a.computerID, event.EventID, payload.MediaType, payload.Content)
				if err != nil {
					return Receipt{}, nil, fmt.Errorf("computer event appender: encrypt payload set member: %w", err)
				}
				stored = envelope
			}
			digest := DigestBytes(stored)
			ref, err := ArtifactRefFromDigest(digest)
			if err != nil {
				return Receipt{}, nil, fmt.Errorf("computer event appender: create payload set artifact ref: %w", err)
			}
			if direction == EventPayloadInput {
				event.InputArtifactRefs = append(nonNilStrings(event.InputArtifactRefs), ref.String())
			} else {
				outputCount++
				event.OutputArtifactRefs = append(nonNilStrings(event.OutputArtifactRefs), ref.String())
				event.PayloadCommitment = digest
			}
			prepared = append(prepared, preparedPayload{EventPayload: payload, stored: stored, digest: digest})
		}
	}
	if len(prepared) != len(payloads) || outputCount > 1 {
		return Receipt{}, nil, fmt.Errorf("computer event appender: payload set direction or output cardinality is invalid")
	}
	event.RequestCommitment = ZeroHead
	pinIntentCommitment, err := ComputePinIntentCommitment(event, input)
	if err != nil {
		return Receipt{}, nil, fmt.Errorf("computer event appender: compute payload set pin intent: %w", err)
	}
	receiptDigests := make([]string, 0, len(prepared))
	artifactDigests := make([]string, 0, len(prepared))
	for _, payload := range prepared {
		var pin PinResult
		if payload.Private {
			pinner := a.pins.(PrivatePayloadPinner)
			pin, err = pinner.PinPrivatePayload(ctx, cipher, a.computerID, event.EventID, payload.stored, pinIntentCommitment)
		} else {
			pinner, ok := a.pins.(NonPrivatePayloadPinner)
			if !ok {
				return Receipt{}, nil, fmt.Errorf("computer event appender: non-private payload pinning unavailable")
			}
			pin, err = pinner.PinNonPrivatePayload(ctx, a.computerID, payload.stored, payload.MediaType, payload.PrivacyClass, pinIntentCommitment)
		}
		if err != nil {
			return Receipt{}, nil, fmt.Errorf("computer event appender: pin payload set member: %w", err)
		}
		if pin.ArtifactDigest != payload.digest {
			return Receipt{}, nil, fmt.Errorf("computer event appender: pinned payload set digest mismatch")
		}
		receiptBytes, receiptErr := pin.Receipt.CanonicalBytes()
		if receiptErr != nil {
			return Receipt{}, nil, fmt.Errorf("computer event appender: canonical payload set receipt: %w", receiptErr)
		}
		receiptDigests = append(receiptDigests, DigestBytes(receiptBytes))
		artifactDigests = append(artifactDigests, payload.digest)
	}
	event.RequestCommitment, err = ComputeRequestCommitment(event, input, pinIntentCommitment, receiptDigests)
	if err != nil {
		return Receipt{}, nil, fmt.Errorf("computer event appender: compute payload set request commitment: %w", err)
	}
	receipt, err := a.appendLocked(ctx, event, input, receiptDigests, nil)
	return receipt, artifactDigests, err
}

func (a *ComputerEventAppender) bindCurrentHeadLocked(ctx context.Context, event *Event) error {
	head, err := a.cas.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: resolve head for new event: %w", err)
	}
	if event.RequireExpectedHead {
		expectedDesiredEventHead, expectedEffectiveEventHead := ZeroHead, ZeroHead
		expectedDesiredStateCommitment, expectedEffectiveStateCommitment := ZeroHead, ZeroHead
		expectedCanonicalHead := ZeroHead
		if head != nil {
			expectedDesiredEventHead, expectedEffectiveEventHead = head.DesiredEventHead, head.EffectiveEventHead
			expectedDesiredStateCommitment, expectedEffectiveStateCommitment = head.DesiredStateCommitment, head.EffectiveStateCommitment
			expectedCanonicalHead = head.CanonicalEventHead
		}
		for _, binding := range []struct {
			name, supplied, current string
		}{
			{"canonical event head", event.PreviousHead, expectedCanonicalHead},
			{"desired event head", event.ExpectedDesiredEventHead, expectedDesiredEventHead},
			{"effective event head", event.ExpectedEffectiveEventHead, expectedEffectiveEventHead},
			{"desired state commitment", event.ExpectedDesiredStateCommitment, expectedDesiredStateCommitment},
			{"effective state commitment", event.ExpectedEffectiveStateCommitment, expectedEffectiveStateCommitment},
		} {
			if binding.supplied != binding.current {
				return fmt.Errorf("computer event appender: expected %s changed", binding.name)
			}
		}
	}
	if head == nil {
		event.Sequence = 1
		event.PreviousHead = ZeroHead
		event.ExpectedDesiredEventHead = ZeroHead
		event.ExpectedEffectiveEventHead = ZeroHead
		event.ExpectedPendingTransitionRef = ""
		event.ExpectedDesiredStateCommitment = ZeroHead
		event.ExpectedEffectiveStateCommitment = ZeroHead
		return nil
	}
	event.Sequence = head.Sequence + 1
	event.PreviousHead = head.CanonicalEventHead
	event.ExpectedDesiredEventHead = head.DesiredEventHead
	event.ExpectedEffectiveEventHead = head.EffectiveEventHead
	event.ExpectedPendingTransitionRef = head.PendingTransitionRef
	event.ExpectedDesiredStateCommitment = head.DesiredStateCommitment
	event.ExpectedEffectiveStateCommitment = head.EffectiveStateCommitment
	return nil
}

// AppendNewSupervisionTransaction binds, pins, and appends one closed
// supervision transaction while holding the canonical appender lock. This is
// the fan-out serialization seam: callers retain their observed working base,
// while acknowledgement binds to the event head current at append time.
func (a *ComputerEventAppender) AppendNewSupervisionTransaction(ctx context.Context, event Event, input TransitionInput, transaction SupervisionTransaction, cipher *PrivateArtifactCipher) (Receipt, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if SupervisionWritesDisabled() {
		return Receipt{}, "", ErrSupervisionWritesDisabled
	}
	if transaction.ComputerID != a.computerID {
		return Receipt{}, "", fmt.Errorf("computer event appender: supervision transaction targets wrong computer")
	}
	if transaction.TransactionID != "" && transaction.TransactionID != transaction.CommandID {
		return Receipt{}, "", fmt.Errorf("computer event appender: pre-reservation transaction_id must be empty or stable command_id")
	}
	if event.EventID != "" || event.OccurredAt != "" {
		return Receipt{}, "", fmt.Errorf("computer event appender: supervision event entropy must be appender-owned")
	}
	commandDigest, err := transaction.ComputeCommandDigest()
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute supervision command digest: %w", err)
	}
	artifactReceiptDigests, err := a.verifyReferencedArtifacts(ctx, transaction)
	if err != nil {
		return Receipt{}, "", err
	}
	if transaction.CommandDigest != "" && transaction.CommandDigest != ZeroHead && transaction.CommandDigest != commandDigest {
		return Receipt{}, "", fmt.Errorf("%w: command digest changed", ErrSupervisionIdempotencyConflict)
	}
	transaction.CommandDigest = commandDigest
	reservations, ok := a.projection.(SupervisionReservationStore)
	if !ok {
		return Receipt{}, "", fmt.Errorf("computer event appender: supervision reservation unavailable")
	}

	receipt, artifactDigest, finalized, err := reservations.ReserveSupervisionCommand(ctx, a.computerID, transaction.CommandID, commandDigest)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: reserve supervision command: %w", err)
	}
	if finalized {
		return receipt, artifactDigest, nil
	}
	plans, ok := a.projection.(FrozenSupervisionPlanStore)
	if !ok {
		return Receipt{}, "", fmt.Errorf("computer event appender: frozen supervision plan storage unavailable")
	}
	plan, found, err := plans.FrozenSupervisionPlan(ctx, a.computerID, transaction.CommandID)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: load frozen supervision plan: %w", err)
	}
	if found && len(plan.PrivateInputs) > 0 {
		// Verified private-input receipts are now bound in transaction. Upgrade
		// the durable input plan to the ordinary frozen event plan before the
		// event envelope is pinned.
		found = false
		plan = FrozenSupervisionPlan{}
	}
	pinner, ok := a.pins.(PrivatePayloadPinner)
	if !ok || cipher == nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: private supervision payload authority unavailable")
	}
	if !found {
		eventID, err := NewEventID()
		if err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: create reserved supervision event identity: %w", err)
		}
		transaction.TransactionID = eventID
		payload, err := transaction.CanonicalBytes()
		if err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: canonical supervision transaction: %w", err)
		}
		envelope, _, err := cipher.EncryptSupervisionDeterministic(ctx, a.computerID, eventID, SupervisionTransactionMediaTypeV1, payload)
		if err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: encrypt supervision transaction: %w", err)
		}
		artifactDigest = DigestBytes(envelope)
		artifactRef, err := ArtifactRefFromDigest(artifactDigest)
		if err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: create supervision artifact ref: %w", err)
		}
		plan = FrozenSupervisionPlan{
			EventID: eventID, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
			Transaction: transaction, Envelope: envelope, ArtifactDigest: artifactDigest, ArtifactRef: artifactRef.String(),
		}
		event.EventID, event.OccurredAt = plan.EventID, plan.OccurredAt
		event.EventKind, event.TrajectoryID, event.IdempotencyKey = EventSupervisionTransaction, transaction.TrajectoryID, transaction.CommandID
		event.ActorProfile, event.AuthorityRef, event.PrivacyClass = transaction.Actor.Role, transaction.Actor.AuthorityRef, "private"
		event.PayloadCommitment, event.DecisionRef = commandDigest, artifactDigest
		event.InputArtifactRefs = appendUniqueString(event.InputArtifactRefs, artifactRef.String())
		event.RequestCommitment = ZeroHead
		if err := a.bindCurrentHeadLocked(ctx, &event); err != nil {
			return Receipt{}, "", err
		}
		plan.PinIntentCommitment, err = ComputePinIntentCommitment(event, input)
		if err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: compute supervision pin intent: %w", err)
		}
		if err := plans.FreezeSupervisionPlan(ctx, a.computerID, transaction.CommandID, commandDigest, plan); err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: freeze supervision plan: %w", err)
		}
	} else {
		frozenDigest, err := plan.Transaction.ComputeCommandDigest()
		if err != nil || plan.Transaction.CommandDigest != commandDigest || frozenDigest != commandDigest {
			return Receipt{}, "", fmt.Errorf("%w: frozen supervision plan command changed", ErrSupervisionIdempotencyConflict)
		}
	}
	artifactRef, err := ParseArtifactRef(plan.ArtifactRef)
	if err != nil || artifactRef.Digest().String() != plan.ArtifactDigest || plan.EventID == "" || plan.OccurredAt == "" {
		return Receipt{}, "", fmt.Errorf("%w: frozen supervision plan is malformed", ErrNeedsProjectionRepair)
	}
	transaction = plan.Transaction
	event.EventID, event.OccurredAt = plan.EventID, plan.OccurredAt
	event.EventKind, event.TrajectoryID, event.IdempotencyKey = EventSupervisionTransaction, transaction.TrajectoryID, transaction.CommandID
	event.ActorProfile, event.AuthorityRef, event.PrivacyClass = transaction.Actor.Role, transaction.Actor.AuthorityRef, "private"
	event.PayloadCommitment, event.DecisionRef = commandDigest, plan.ArtifactDigest
	event.InputArtifactRefs = appendUniqueString(event.InputArtifactRefs, plan.ArtifactRef)
	event.RequestCommitment = ZeroHead
	if err := a.bindCurrentHeadLocked(ctx, &event); err != nil {
		return Receipt{}, "", err
	}
	pinIntentCommitment, err := ComputePinIntentCommitment(event, input)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute supervision pin intent: %w", err)
	}
	if pinIntentCommitment != plan.PinIntentCommitment {
		return Receipt{}, "", fmt.Errorf("%w: frozen supervision pin intent changed", ErrSupervisionIdempotencyConflict)
	}
	if plan.PinReceipt == nil {
		pin, err := pinner.PinPrivatePayload(ctx, cipher, a.computerID, plan.EventID, plan.Envelope, pinIntentCommitment)
		if err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: pin private supervision transaction: %w", err)
		}
		if pin.ArtifactDigest != plan.ArtifactDigest {
			return Receipt{}, "", fmt.Errorf("computer event appender: pinned supervision artifact digest mismatch")
		}
		verifier, ok := a.verifier.(ArtifactPinReceiptVerifier)
		if !ok {
			return Receipt{}, "", fmt.Errorf("computer event appender: artifact pin receipt verifier unavailable")
		}
		if err := verifier.VerifyArtifactPinReceipt(ctx, pin.Receipt, a.computerID, plan.ArtifactDigest); err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: verify supervision pin receipt: %w", err)
		}
		if err := plans.RecordSupervisionPin(ctx, a.computerID, transaction.CommandID, commandDigest, pin.Receipt); err != nil {
			return Receipt{}, "", fmt.Errorf("computer event appender: record supervision pin receipt: %w", err)
		}
		plan.PinReceipt = &pin.Receipt
	}
	verifier, ok := a.verifier.(ArtifactPinReceiptVerifier)
	if !ok {
		return Receipt{}, "", fmt.Errorf("computer event appender: artifact pin receipt verifier unavailable")
	}
	if err := verifier.VerifyArtifactPinReceipt(ctx, *plan.PinReceipt, a.computerID, plan.ArtifactDigest); err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: verify frozen supervision pin receipt: %w", err)
	}
	receiptBytes, err := plan.PinReceipt.CanonicalBytes()
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: canonical supervision payload receipt: %w", err)
	}
	payloadReceiptDigests := append(artifactReceiptDigests, DigestBytes(receiptBytes))
	event.RequestCommitment, err = ComputeRequestCommitment(event, input, pinIntentCommitment, payloadReceiptDigests)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: compute supervision request commitment: %w", err)
	}
	receipt, err = a.appendLocked(ctx, event, input, payloadReceiptDigests, &transaction)
	return receipt, plan.ArtifactDigest, err
}

// AppendSupervisionTransaction appends one closed supervision command. The
// transaction artifact must already be pinned and named by the event.
func (a *ComputerEventAppender) AppendSupervisionTransaction(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string, transaction SupervisionTransaction) (Receipt, error) {
	if SupervisionWritesDisabled() {
		return Receipt{}, ErrSupervisionWritesDisabled
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	commandDigest, err := transaction.ComputeCommandDigest()
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: compute supervision command digest: %w", err)
	}
	if transaction.CommandDigest != commandDigest {
		return Receipt{}, fmt.Errorf("computer event appender: supervision command digest mismatch")
	}
	artifactReceiptDigests, err := a.verifyReferencedArtifacts(ctx, transaction)
	if err != nil {
		return Receipt{}, err
	}
	for _, digest := range artifactReceiptDigests {
		found := false
		for _, supplied := range payloadPinReceiptDigests {
			if supplied == digest {
				found = true
				break
			}
		}
		if !found {
			return Receipt{}, fmt.Errorf("computer event appender: referenced artifact receipt is missing from request")
		}
	}
	lookup, ok := a.projection.(SupervisionCommandStore)
	if !ok {
		return Receipt{}, fmt.Errorf("computer event appender: supervision command lookup unavailable")
	}
	receipt, _, storedDigest, found, err := lookup.SupervisionCommand(ctx, a.computerID, transaction.CommandID)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: lookup supervision command: %w", err)
	}
	if found {
		if storedDigest != commandDigest {
			return Receipt{}, fmt.Errorf("%w: command digest changed", ErrSupervisionIdempotencyConflict)
		}
		return receipt, nil
	}
	if err := ValidateSupervisionEventBinding(event, transaction); err != nil {
		return Receipt{}, err
	}
	return a.appendLocked(ctx, event, input, payloadPinReceiptDigests, &transaction)
}

func (a *ComputerEventAppender) appendLocked(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string, supervision *SupervisionTransaction) (Receipt, error) {
	if event.ComputerID != a.computerID {
		return Receipt{}, fmt.Errorf("computer event appender: wrong computer")
	}
	if event.EventKind == EventSupervisionTransaction {
		if SupervisionWritesDisabled() {
			return Receipt{}, ErrSupervisionWritesDisabled
		}
		if supervision == nil {
			return Receipt{}, fmt.Errorf("computer event appender: supervision transaction is required")
		}
		if err := ValidateSupervisionEventBinding(event, *supervision); err != nil {
			return Receipt{}, err
		}
	}
	payloadPinReceiptDigests = nonNilStrings(payloadPinReceiptDigests)
	pinIntentCommitment, err := ComputePinIntentCommitment(event, input)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: pin intent commitment: %w", err)
	}
	requestCommitment, err := ComputeRequestCommitment(event, input, pinIntentCommitment, payloadPinReceiptDigests)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: request commitment: %w", err)
	}
	if event.RequestCommitment != requestCommitment {
		return Receipt{}, fmt.Errorf("computer event appender: request commitment mismatch")
	}
	platformHead, err := a.cas.Head(ctx, a.computerID)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: resolve canonical head: %w", err)
	}
	embeddedHead, err := a.projection.Head(ctx, a.computerID)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: resolve embedded head: %w", err)
	}
	if !sameHead(platformHead, embeddedHead) {
		return Receipt{}, ErrNeedsProjectionRepair
	}
	next, err := Reduce(platformHead, event, input)
	if err != nil {
		return Receipt{}, err
	}
	body, err := event.CanonicalBytes()
	if err != nil {
		return Receipt{}, err
	}
	digest, err := event.Digest()
	if err != nil {
		return Receipt{}, err
	}
	pin, err := a.pins.PinEvent(ctx, a.computerID, body, event.RequestCommitment)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: pin event: %w", err)
	}
	if pin.ArtifactDigest != digest {
		return Receipt{}, fmt.Errorf("computer event appender: pinned event digest mismatch")
	}
	pinReceiptBytes, err := pin.Receipt.CanonicalBytes()
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: canonical pin receipt: %w", err)
	}
	pinReceiptDigest := DigestBytes(pinReceiptBytes)
	request := CASRequest{
		Event:                    event,
		EventDigest:              digest,
		EventArtifactDigest:      pin.ArtifactDigest,
		EventPinReceiptDigest:    pinReceiptDigest,
		PayloadPinReceiptDigests: nonNilStrings(payloadPinReceiptDigests),
		PinIntentCommitment:      pinIntentCommitment,
		Next:                     next,
		SupervisionTransaction:   supervision,
		Input:                    input,
	}
	if err := a.projection.Prepare(ctx, request); err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: prepare embedded projection: %w", err)
	}
	receipt, err := a.cas.CompareAndSwap(ctx, request)
	if err != nil {
		_ = a.projection.DiscardPrepared(ctx, a.computerID, digest)
		return Receipt{}, fmt.Errorf("computer event appender: head CAS: %w", err)
	}
	if err := a.verifier.VerifyEventHeadReceipt(ctx, receipt, request); err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: verify head receipt: %w", err)
	}
	if err := a.projection.Finalize(ctx, a.computerID, digest, receipt); err != nil {
		return Receipt{}, fmt.Errorf("%w: finalize embedded projection: %v", ErrNeedsProjectionRepair, err)
	}
	return receipt, nil
}

func (a *ComputerEventAppender) RecoverPrepared(ctx context.Context) error {
	prepared, err := a.projection.Prepared(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: list prepared projections: %w", err)
	}
	for _, request := range prepared {
		platformHead, err := a.cas.Head(ctx, a.computerID)
		if err != nil {
			return fmt.Errorf("computer event appender: recovery head: %w", err)
		}
		if platformHead != nil && platformHead.Sequence == request.Event.Sequence && platformHead.CanonicalEventHead == request.EventDigest {
			receipt, err := a.cas.CompareAndSwap(ctx, request)
			if err != nil {
				return fmt.Errorf("computer event appender: recover durable receipt: %w", err)
			}
			if err := a.verifier.VerifyEventHeadReceipt(ctx, receipt, request); err != nil {
				return fmt.Errorf("computer event appender: verify recovery receipt: %w", err)
			}
			if err := a.projection.Finalize(ctx, a.computerID, request.EventDigest, receipt); err != nil {
				return fmt.Errorf("computer event appender: finalize recovery: %w", err)
			}
			continue
		}
		if (platformHead == nil && request.Event.Sequence == 1 && request.Event.PreviousHead == ZeroHead) ||
			(platformHead != nil && platformHead.Sequence+1 == request.Event.Sequence && platformHead.CanonicalEventHead == request.Event.PreviousHead) {
			if err := a.projection.DiscardPrepared(ctx, a.computerID, request.EventDigest); err != nil {
				return fmt.Errorf("computer event appender: discard uncommitted projection: %w", err)
			}
			continue
		}
		return ErrNeedsProjectionRepair
	}
	return nil
}

func (a *ComputerEventAppender) Reconstruct(ctx context.Context, source EventSource) error {
	if source == nil {
		return fmt.Errorf("computer event appender: event source is required")
	}
	if err := a.RecoverPrepared(ctx); err != nil {
		return err
	}
	localHead, err := a.projection.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: reconstruction local head: %w", err)
	}
	var after uint64
	if localHead != nil {
		after = localHead.Sequence
	}
	records, err := source.Events(ctx, a.computerID, after)
	if err != nil {
		return fmt.Errorf("computer event appender: fetch durable chain: %w", err)
	}
	current := localHead
	for _, record := range records {
		next, err := Reduce(current, record.Request.Event, record.Request.Input)
		if err != nil {
			return fmt.Errorf("computer event appender: replay sequence %d: %w", record.Request.Event.Sequence, err)
		}
		if !sameHead(&next, &record.Request.Next) {
			return fmt.Errorf("computer event appender: replay projection mismatch at sequence %d", record.Request.Event.Sequence)
		}
		if err := a.verifier.VerifyEventHeadReceipt(ctx, record.Receipt, record.Request); err != nil {
			return fmt.Errorf("computer event appender: replay receipt sequence %d: %w", record.Request.Event.Sequence, err)
		}
		if err := a.projection.Prepare(ctx, record.Request); err != nil {
			return fmt.Errorf("computer event appender: replay prepare sequence %d: %w", record.Request.Event.Sequence, err)
		}
		if err := a.projection.Finalize(ctx, a.computerID, record.Request.EventDigest, record.Receipt); err != nil {
			return fmt.Errorf("computer event appender: replay finalize sequence %d: %w", record.Request.Event.Sequence, err)
		}
		current = &next
	}
	platformHead, err := a.cas.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: reconstruction canonical head: %w", err)
	}
	finalLocal, err := a.projection.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: reconstruction final local head: %w", err)
	}
	if !sameHead(platformHead, finalLocal) {
		return ErrNeedsProjectionRepair
	}
	return nil
}

// LoadPrivateSupervisionArtifact fetches one canonical private artifact,
// verifies its signed pin receipt, and decrypts it under the binding identity
// frozen by the supervision command. It is the read side used by derived actor
// delivery and restart recovery; the returned plaintext carries no authority
// independent of the canonical event that references it.
func (a *ComputerEventAppender) LoadPrivateSupervisionArtifact(ctx context.Context, refValue, bindingID string, cipher *PrivateArtifactCipher) ([]byte, PrivateArtifactMetadata, error) {
	if a == nil || cipher == nil || strings.TrimSpace(bindingID) == "" {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: private supervision artifact authority unavailable")
	}
	ref, err := ParseArtifactRef(strings.TrimSpace(refValue))
	if err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: parse private supervision artifact: %w", err)
	}
	source, ok := a.pins.(PrivateArtifactSource)
	if !ok {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: canonical private artifact source unavailable")
	}
	digest := ref.Digest().String()
	envelope, pin, err := source.PrivateArtifact(ctx, a.computerID, digest)
	if err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: fetch private supervision artifact: %w", err)
	}
	if pin.ArtifactDigest != digest || DigestBytes(envelope) != digest {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: private supervision artifact digest mismatch")
	}
	verifier, ok := a.verifier.(ArtifactPinReceiptVerifier)
	if !ok {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: artifact pin receipt verifier unavailable")
	}
	if err := verifier.VerifyArtifactPinReceipt(ctx, pin.Receipt, a.computerID, digest); err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: verify private supervision artifact receipt: %w", err)
	}
	plaintext, metadata, err := cipher.Decrypt(ctx, envelope, a.computerID, strings.TrimSpace(bindingID))
	if err != nil {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: decrypt private supervision artifact: %w", err)
	}
	if metadata.ComputerID != a.computerID || metadata.EventID != strings.TrimSpace(bindingID) || metadata.PrivacyClass != "private" {
		return nil, PrivateArtifactMetadata{}, fmt.Errorf("computer event appender: private supervision artifact metadata mismatch")
	}
	return plaintext, metadata, nil
}

// RebuildPrivateProjectionFromPinnedSource reconstructs solely from the
// canonical source which already owns pinning and head CAS for this appender.
// It is the no-SSH repair seam used by the product API.
func (a *ComputerEventAppender) RebuildPrivateProjectionFromPinnedSource(ctx context.Context, cipher *PrivateArtifactCipher) error {
	source, ok := a.pins.(PrivateArtifactSource)
	if !ok {
		return fmt.Errorf("computer event appender: canonical private event source unavailable")
	}
	return a.RebuildPrivateProjection(ctx, source, cipher)
}

// RebuildPrivateProjection verifies the complete external chain before asking
// the projection store to atomically replace its replay-owned state. A store
// lacking the atomic rebuild seam fails closed rather than deleting a usable
// local projection.
func (a *ComputerEventAppender) RebuildPrivateProjection(ctx context.Context, source PrivateArtifactSource, cipher *PrivateArtifactCipher) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if source == nil || cipher == nil {
		return fmt.Errorf("computer event appender: private event source and cipher are required")
	}
	rebuilder, ok := a.projection.(interface {
		RebuildComputerEventProjection(context.Context, []DurableEvent, *Head) error
	})
	if !ok {
		return fmt.Errorf("computer event appender: atomic projection rebuild unavailable")
	}
	records, err := source.Events(ctx, a.computerID, 0)
	if err != nil {
		return fmt.Errorf("computer event appender: fetch rebuild chain: %w", err)
	}
	var current *Head
	for index := range records {
		record := &records[index]
		if err := a.hydrateSupervisionReplay(ctx, source, cipher, record); err != nil {
			return fmt.Errorf("computer event appender: hydrate replay sequence %d: %w", record.Request.Event.Sequence, err)
		}
		if transaction := record.Request.SupervisionTransaction; transaction != nil {
			artifactReceiptDigests, err := a.verifyReferencedArtifacts(ctx, *transaction)
			if err != nil {
				return fmt.Errorf("computer event appender: verify replay artifacts sequence %d: %w", record.Request.Event.Sequence, err)
			}
			for _, digest := range artifactReceiptDigests {
				found := false
				for _, recorded := range record.Request.PayloadPinReceiptDigests {
					if recorded == digest {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("%w: replay artifact receipt missing at sequence %d", ErrNeedsProjectionRepair, record.Request.Event.Sequence)
				}
			}
		}
		next, err := Reduce(current, record.Request.Event, record.Request.Input)
		if err != nil || !sameHead(&next, &record.Request.Next) {
			return fmt.Errorf("%w: replay sequence %d", ErrNeedsProjectionRepair, record.Request.Event.Sequence)
		}
		if err := a.verifier.VerifyEventHeadReceipt(ctx, record.Receipt, record.Request); err != nil {
			return fmt.Errorf("computer event appender: verify replay receipt sequence %d: %w", record.Request.Event.Sequence, err)
		}
		current = &next
	}
	platformHead, err := a.cas.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: rebuild canonical head: %w", err)
	}
	if len(records) == 0 && platformHead == nil {
		platformHead = &Head{ComputerID: a.computerID}
		current = platformHead
	}
	if !sameHead(platformHead, current) {
		return ErrNeedsProjectionRepair
	}
	if err := rebuilder.RebuildComputerEventProjection(ctx, records, current); err != nil {
		return fmt.Errorf("computer event appender: atomic rebuild projection: %w", err)
	}
	return nil
}

func (a *ComputerEventAppender) hydrateSupervisionReplay(ctx context.Context, source PrivateArtifactSource, cipher *PrivateArtifactCipher, record *DurableEvent) error {
	event := record.Request.Event
	if event.EventKind != EventSupervisionTransaction {
		return nil
	}
	envelope, pin, err := source.PrivateArtifact(ctx, a.computerID, event.DecisionRef)
	if err != nil {
		return err
	}
	if pin.ArtifactDigest != event.DecisionRef || DigestBytes(envelope) != event.DecisionRef {
		return fmt.Errorf("private artifact digest mismatch")
	}
	verifier, ok := a.verifier.(ArtifactPinReceiptVerifier)
	if !ok {
		return fmt.Errorf("artifact pin receipt verifier unavailable")
	}
	if err := verifier.VerifyArtifactPinReceipt(ctx, pin.Receipt, a.computerID, event.DecisionRef); err != nil {
		return err
	}
	plaintext, metadata, err := cipher.Decrypt(ctx, envelope, a.computerID, event.EventID)
	if err != nil {
		return err
	}
	if metadata.MediaType != SupervisionTransactionMediaTypeV1 || metadata.PrivacyClass != "private" {
		return fmt.Errorf("private artifact metadata mismatch")
	}
	transaction, err := DecodeSupervisionTransaction(plaintext)
	if err != nil {
		return err
	}
	canonical, err := transaction.CanonicalBytes()
	if err != nil || !bytes.Equal(canonical, plaintext) {
		return fmt.Errorf("supervision transaction is not canonical")
	}
	if err := ValidateSupervisionEventBinding(event, transaction); err != nil {
		return err
	}
	record.Request.SupervisionTransaction = &transaction
	return nil
}

const (
	// PlatformControlTrustKeyID and PlatformControlTrustPublicKey are the
	// repository-reviewed staging trust root. Rotation requires a reviewed
	// overlap policy before either value changes.
	PlatformControlTrustKeyID     = "868f96cca8726f99"
	PlatformControlTrustPublicKey = "sjka9TOy/Zx3Nl608bpWQ2Dft/s1yJEiMo7NzVBxwZs"
)

func (a *ComputerEventAppender) verifyReferencedArtifacts(ctx context.Context, transaction SupervisionTransaction) ([]string, error) {
	named := make(map[string]struct{})
	for _, mutation := range transaction.Mutations {
		var body any
		decoder := json.NewDecoder(bytes.NewReader(mutation.Body))
		if err := decoder.Decode(&body); err != nil {
			return nil, fmt.Errorf("computer event appender: decode mutation artifact refs: %w", err)
		}
		collectNamedArtifactRefs(body, "", named)
	}
	if len(named) != len(transaction.ReferencedArtifacts) {
		return nil, fmt.Errorf("computer event appender: referenced artifacts do not exactly cover mutation refs")
	}
	if len(transaction.ReferencedArtifacts) == 0 {
		return nil, nil
	}
	verifier, ok := a.verifier.(ArtifactPinReceiptVerifier)
	if !ok {
		return nil, fmt.Errorf("computer event appender: artifact pin receipt verifier unavailable")
	}
	digests := make([]string, 0, len(transaction.ReferencedArtifacts))
	for _, artifact := range transaction.ReferencedArtifacts {
		if _, exists := named[artifact.Ref]; !exists {
			return nil, fmt.Errorf("computer event appender: unexpected referenced artifact %q", artifact.Ref)
		}
		if err := verifier.VerifyArtifactPinReceipt(ctx, artifact.PinReceipt, a.computerID, artifact.ArtifactDigest); err != nil {
			return nil, fmt.Errorf("computer event appender: verify referenced artifact receipt: %w", err)
		}
		if artifact.PinReceipt.KindFields["media_type"] != artifact.MediaType {
			return nil, fmt.Errorf("computer event appender: referenced artifact media type mismatch")
		}
		raw, err := artifact.PinReceipt.CanonicalBytes()
		if err != nil {
			return nil, fmt.Errorf("computer event appender: canonical referenced artifact receipt: %w", err)
		}
		digests = append(digests, DigestBytes(raw))
		delete(named, artifact.Ref)
	}
	if len(named) != 0 {
		return nil, fmt.Errorf("computer event appender: referenced artifacts are incomplete")
	}
	return digests, nil
}

func collectNamedArtifactRefs(value any, key string, refs map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			collectNamedArtifactRefs(child, childKey, refs)
		}
	case []any:
		for _, child := range typed {
			collectNamedArtifactRefs(child, key, refs)
		}
	case string:
		if strings.Contains(key, "artifact_ref") || strings.HasSuffix(key, "_artifact_refs") || key == "import_ref" || strings.HasSuffix(key, "_receipt_ref") || strings.HasSuffix(key, "_receipt_refs") {
			if _, err := ParseArtifactRef(typed); err == nil {
				refs[typed] = struct{}{}
			}
		}
	}
}

// PlatformControlTrustDigest validates and returns the pinned key digest.
func PlatformControlTrustDigest() (string, error) {
	publicKey, err := base64.RawStdEncoding.DecodeString(PlatformControlTrustPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", fmt.Errorf("platform-control trust key is invalid")
	}
	digest := sha256.Sum256(publicKey)
	if keyID := hex.EncodeToString(digest[:8]); keyID != PlatformControlTrustKeyID {
		return "", fmt.Errorf("platform-control trust key id does not match its digest")
	}
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func DigestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func sameHead(left, right *Head) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
