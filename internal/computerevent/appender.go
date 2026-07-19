package computerevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

var ErrNeedsProjectionRepair = errors.New("computer event projection repair required")

type PinResult struct {
	ArtifactDigest string
	Receipt        Receipt
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

type CASRequest struct {
	Event                    Event           `json:"event"`
	EventDigest              string          `json:"event_digest"`
	EventArtifactDigest      string          `json:"event_artifact_digest"`
	EventPinReceiptDigest    string          `json:"event_pin_receipt_digest"`
	PayloadPinReceiptDigests []string        `json:"payload_pin_receipt_digests"`
	PinIntentCommitment      string          `json:"pin_intent_commitment"`
	Input                    TransitionInput `json:"transition_input"`
	Next                     Head            `json:"next_head"`
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

func NewComputerEventAppender(computerID string, pins ArtifactPinner, projection ProjectionStore, cas HeadCAS, verifier ReceiptVerifier) (*ComputerEventAppender, error) {
	if computerID == "" || pins == nil || projection == nil || cas == nil || verifier == nil {
		return nil, fmt.Errorf("computer event appender: complete dependencies are required")
	}
	return &ComputerEventAppender{computerID: computerID, pins: pins, projection: projection, cas: cas, verifier: verifier}, nil
}

func (a *ComputerEventAppender) Append(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string) (Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendLocked(ctx, event, input, payloadPinReceiptDigests)
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
	return a.appendLocked(ctx, event, input, payloadPinReceiptDigests)
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
	receipt, err := a.appendLocked(ctx, event, input, []string{payloadReceiptDigest})
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
	receipt, err := a.appendLocked(ctx, event, input, []string{payloadReceiptDigest})
	return receipt, payloadDigest, err
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

func (a *ComputerEventAppender) appendLocked(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string) (Receipt, error) {
	if event.ComputerID != a.computerID {
		return Receipt{}, fmt.Errorf("computer event appender: wrong computer")
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
