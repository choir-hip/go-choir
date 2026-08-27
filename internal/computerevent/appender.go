package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
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

// BatchProjectionStore applies a payload already resolved before BeginTx.
// Head-only events pass a nil batch.
type BatchProjectionStore interface {
	FinalizeBatch(ctx context.Context, computerID, eventDigest string, receipt Receipt, batch *ProjectionBatch) error
}

// ReplayBatchProjectionStore is the explicit dry-run seam for projection
// compatibility that is safe only while reconstructing a canonical tape.
// Live finalization must continue through BatchProjectionStore.
type ReplayBatchProjectionStore interface {
	FinalizeReplayBatch(ctx context.Context, computerID, eventDigest string, receipt Receipt, batch *ProjectionBatch) error
}

// ReplayCommitter flushes deferred VM-local history after a complete replay.
// Implementations must not expose this boundary to live append paths.
type ReplayCommitter interface {
	CommitReplay(context.Context) error
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

// PagedEventSource exposes bounded replay pages so long chains do not have to
// materialize in guest memory before projection begins.
type PagedEventSource interface {
	EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]DurableEvent, error)
}
type ReceiptVerifier interface {
	VerifyEventHeadReceipt(ctx context.Context, receipt Receipt, request CASRequest) error
}

// ComputerEventAppender is the sole semantic event sequencer in the trusted
// guest core. Its dependencies expose only mechanical pin, prepare, and CAS
// operations; agents, capsules, reducers, vmctl, and route projections never
// receive this object or its append capability.
type ComputerEventAppender struct {
	computerID       string
	pins             ArtifactPinner
	projection       ProjectionStore
	cas              HeadCAS
	verifier         ReceiptVerifier
	reader           ArtifactReader
	cipher           *PrivateArtifactCipher
	livePayloads     map[string][]byte
	replayProjection bool
	mu               sync.Mutex
	// Replay progress snapshot (guarded by mu) so the guest health surface can
	// report a replay in progress with its sequence to the host wait-for-ready
	// probe without racing the replay goroutine.
	replayActive        bool
	replaySeq           uint64
	replayCommittedSeq  uint64
	replayCheckpointEvery int
	replayCheckpointInterval time.Duration
}

// ReplaySnapshot describes the durable replay progress for the liveness probe.
type ReplaySnapshot struct {
	InProgress        bool   `json:"in_progress"`
	Sequence          uint64 `json:"sequence"`
	CommittedSequence uint64 `json:"committed_sequence"`
}

// ReplaySnapshot returns the current durable replay progress. Zero value means
// no replay is running.
func (a *ComputerEventAppender) ReplaySnapshot() ReplaySnapshot {
	if a == nil {
		return ReplaySnapshot{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return ReplaySnapshot{InProgress: a.replayActive, Sequence: a.replaySeq, CommittedSequence: a.replayCommittedSeq}
}

// SetReplayCheckpointPolicy configures the durable checkpoint cadence during a
// replay: a commit is issued after checkpointEvery applied events or
// checkpointInterval wall time, whichever comes first. Non-positive values keep
// the defaults (500 events / 60s). Call before SetReplayMode(true).
func (a *ComputerEventAppender) SetReplayCheckpointPolicy(every int, interval time.Duration) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if every > 0 {
		a.replayCheckpointEvery = every
	}
	if interval > 0 {
		a.replayCheckpointInterval = interval
	}
}

func (a *ComputerEventAppender) setReplayActive(active bool) {
	a.mu.Lock()
	a.replayActive = active
	a.mu.Unlock()
}

func (a *ComputerEventAppender) setReplayProgress(seq, committedSeq uint64) {
	a.mu.Lock()
	a.replaySeq = seq
	if committedSeq > a.replayCommittedSeq {
		a.replayCommittedSeq = committedSeq
	}
	a.mu.Unlock()
}
func (a *ComputerEventAppender) committedReplaySeq() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.replayCommittedSeq
}

func NewComputerEventAppender(computerID string, pins ArtifactPinner, projection ProjectionStore, cas HeadCAS, verifier ReceiptVerifier) (*ComputerEventAppender, error) {
	if computerID == "" || pins == nil || projection == nil || cas == nil || verifier == nil {
		return nil, fmt.Errorf("computer event appender: complete dependencies are required")
	}
	return &ComputerEventAppender{
		computerID:              computerID,
		pins:                    pins,
		projection:              projection,
		cas:                     cas,
		verifier:                verifier,
		replayCheckpointEvery:   500,
		replayCheckpointInterval: 60 * time.Second,
	}, nil
}

// RebindProjection replaces the local projection store after a tape rematerialize
// flips the VM-local realization in place. CAS and pin clients stay bound to the
// platform event authority; only the local Dolt/SQLite projection moves.
// SetPayloadResolver installs the pre-SQL artifact fetch/decrypt seam used
// for projection_batch_recorded replay. Live pin still happens before CAS.
func (a *ComputerEventAppender) SetPayloadResolver(reader ArtifactReader, cipher *PrivateArtifactCipher) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reader = reader
	a.cipher = cipher
}

// SetReplayMode enables the replay-only projection path for a bounded
// reconstruction. Callers must disable it before using the appender for live
// semantic appends.
func (a *ComputerEventAppender) SetReplayMode(enabled bool) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.replayProjection = enabled
	a.mu.Unlock()
}

func (a *ComputerEventAppender) RebindProjection(projection ProjectionStore) error {
	if a == nil || projection == nil {
		return fmt.Errorf("computer event appender: projection rebind requires complete dependencies")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.projection = projection
	return nil
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
	if a.livePayloads == nil {
		a.livePayloads = map[string][]byte{}
	}
	a.livePayloads[payloadDigest] = append([]byte(nil), payload...)
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
	receipt, err := a.appendLocked(ctx, event, input, receiptDigests)
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
	if err := a.finalizeProjection(ctx, event, digest, receipt); err != nil {
		return Receipt{}, fmt.Errorf("%w: finalize embedded projection: %w", ErrNeedsProjectionRepair, ClassifyProjectionFailure(err))
	}
	return receipt, nil
}

func (a *ComputerEventAppender) RecoverPrepared(ctx context.Context) error {
	prepared, err := a.projection.Prepared(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: list prepared projections: %w", err)
	}
	for _, request := range prepared {
		if a.replayProjection {
			// Replay is rebuilding from the canonical tape: any leftover prepared
			// row is re-derivable, so discard it and let the replay re-apply from
			// the tape. The replay path NEVER performs a semantic CAS (it must not
			// become a second writer), and a prepared row at seq << platform head
			// must not trip ErrNeedsProjectionRepair and brick the resume.
			if err := a.projection.DiscardPrepared(ctx, a.computerID, request.EventDigest); err != nil {
				return fmt.Errorf("computer event appender: discard replay prepared projection: %w", err)
			}
			continue
		}
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
			if err := a.finalizeProjection(ctx, request.Event, request.EventDigest, receipt); err != nil {
				return fmt.Errorf("%w: finalize recovery: %w", ErrNeedsProjectionRepair, ClassifyProjectionFailure(err))
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
	return a.reconstruct(ctx, source, "")
}

// ReconstructThrough replays the durable chain until the canonical event head
// matches targetHead, then halts. Later events stay on the tape and are not
// applied to the projection. Missing the target is a hard failure.
func (a *ComputerEventAppender) ReconstructThrough(ctx context.Context, source EventSource, targetHead string) error {
	targetHead = strings.TrimSpace(targetHead)
	if !IsSHA256(targetHead) {
		return fmt.Errorf("computer event appender: restore target head is required")
	}
	return a.reconstruct(ctx, source, targetHead)
}

func (a *ComputerEventAppender) reconstruct(ctx context.Context, source EventSource, targetHead string) error {
	if source == nil {
		return fmt.Errorf("computer event appender: event source is required")
	}
	if a.replayProjection {
		a.setReplayActive(true)
		defer a.setReplayActive(false)
	}
	if err := a.RecoverPrepared(ctx); err != nil {
		return err
	}
	localHead, err := a.projection.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: reconstruction local head: %w", err)
	}
	if targetHead != "" && localHead != nil && localHead.CanonicalEventHead == targetHead {
		if a.replayProjection {
			a.setReplayProgress(localHead.Sequence, localHead.Sequence)
		}
		return nil
	}
	var after uint64
	if localHead != nil {
		after = localHead.Sequence
	}
	current := localHead
	apply := func(record DurableEvent) error {
		applyStarted := time.Now()
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
		if err := a.finalizeProjection(ctx, record.Request.Event, record.Request.EventDigest, record.Receipt); err != nil {
			return fmt.Errorf("computer event appender: replay finalize sequence %d: %w", record.Request.Event.Sequence, err)
		}
		a.setReplayProgress(record.Request.Event.Sequence, a.committedReplaySeq())
		if elapsed := time.Since(applyStarted); elapsed > 2*time.Second {
			log.Printf("computer event appender: replay apply slow seq=%d elapsed=%s", record.Request.Event.Sequence, elapsed)
		}
		current = &next
		return nil
	}
	// Periodic durable checkpoint: every replayCheckpointEvery applied events or
	// replayCheckpointInterval wall time, whichever comes first. Progress is only
	// considered durable when the Dolt checkpoint commit succeeds; the SQL working
	// set alone is not the resume authority for crash safety (B7/B10).
	eventsSinceCommit := 0
	var lastCheckpointAt time.Time
	checkpoint := func(seq uint64) error {
		if !a.replayProjection {
			a.setReplayProgress(seq, seq)
			return nil
		}
		eventsSinceCommit++
		committed := a.committedReplaySeq()
		due := eventsSinceCommit >= a.replayCheckpointEvery || lastCheckpointAt.IsZero() || time.Since(lastCheckpointAt) >= a.replayCheckpointInterval
		if due {
			commitStarted := time.Now()
			if err := a.commitReplay(ctx); err != nil {
				return fmt.Errorf("computer event appender: replay periodic checkpoint: %w", err)
			}
			if elapsed := time.Since(commitStarted); elapsed > 2*time.Second {
				log.Printf("computer event appender: replay checkpoint seq=%d commit elapsed=%s", seq, elapsed)
			}
			eventsSinceCommit = 0
			lastCheckpointAt = time.Now()
			committed = seq
		}
		a.setReplayProgress(seq, committed)
		return nil
	}
	pageSize := EventReplayPageSize
	if pageSource, ok := source.(PagedEventSource); ok {
		for {
			pageStarted := time.Now()
			page, err := pageSource.EventsPage(ctx, a.computerID, after, pageSize)
			if err != nil {
				return fmt.Errorf("computer event appender: fetch durable chain: %w", err)
			}
			if elapsed := time.Since(pageStarted); elapsed > 2*time.Second {
				log.Printf("computer event appender: replay page fetch after=%d count=%d elapsed=%s", after, len(page), elapsed)
			}
			if len(page) == 0 {
				break
			}
			for _, record := range page {
				if err := apply(record); err != nil {
					return err
				}
				after = record.Request.Event.Sequence
				if err := checkpoint(after); err != nil {
					return err
				}
				if targetHead != "" && current.CanonicalEventHead == targetHead {
					if err := a.commitReplay(ctx); err != nil {
						return fmt.Errorf("computer event appender: replay commit: %w", err)
					}
					a.setReplayProgress(after, after)
					return nil
				}
				// Quantum-end flush: when the replay context expires, publish a
				// final durable checkpoint before erroring so the next boot resumes
				// from the committed head (B7 30m resume quantum).
				if ctxErr := ctx.Err(); ctxErr != nil {
					if ferr := a.commitReplay(context.WithoutCancel(ctx)); ferr != nil {
						return fmt.Errorf("computer event appender: replay quantum flush: %w", ferr)
					}
					a.setReplayProgress(after, after)
					return ctxErr
				}
			}
			if len(page) < pageSize || after == ^uint64(0) {
				break
			}
		}
	} else {
		records, err := source.Events(ctx, a.computerID, after)
		if err != nil {
			return fmt.Errorf("computer event appender: fetch durable chain: %w", err)
		}
		for _, record := range records {
			if err := apply(record); err != nil {
				return err
			}
			if err := checkpoint(record.Request.Event.Sequence); err != nil {
				return err
			}
			if targetHead != "" && current.CanonicalEventHead == targetHead {
				if err := a.commitReplay(ctx); err != nil {
					return fmt.Errorf("computer event appender: replay commit: %w", err)
				}
				return nil
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				if ferr := a.commitReplay(context.WithoutCancel(ctx)); ferr != nil {
					return fmt.Errorf("computer event appender: replay quantum flush: %w", ferr)
				}
				a.setReplayProgress(record.Request.Event.Sequence, record.Request.Event.Sequence)
				return ctxErr
			}
		}
	}
	if targetHead != "" {
		return fmt.Errorf("computer event appender: restore target head was not reached")
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
		return fmt.Errorf("%w: local=%s platform=%s", ErrNeedsProjectionRepair, replayHeadSummary(finalLocal), replayHeadSummary(platformHead))
	}
	if err := a.commitReplay(ctx); err != nil {
		return fmt.Errorf("computer event appender: replay commit: %w", err)
	}
	return nil
}

func (a *ComputerEventAppender) commitReplay(ctx context.Context) error {
	if a == nil || !a.replayProjection {
		return nil
	}
	committer, ok := a.projection.(ReplayCommitter)
	if !ok {
		return nil
	}
	if err := committer.CommitReplay(ctx); err != nil {
		return err
	}
	// A successful durable checkpoint makes the last applied sequence the
	// committed resume head for the liveness probe.
	a.mu.Lock()
	if a.replaySeq > a.replayCommittedSeq {
		a.replayCommittedSeq = a.replaySeq
	}
	a.mu.Unlock()
	return nil
}

func replayHeadSummary(head *Head) string {
	if head == nil {
		return "<nil>"
	}
	return fmt.Sprintf("seq=%d canonical=%s desired=%s effective=%s desired_state=%s effective_state=%s pending=%s reducer=%d credential_epoch=%d",
		head.Sequence, head.CanonicalEventHead, head.DesiredEventHead, head.EffectiveEventHead,
		head.DesiredStateCommitment, head.EffectiveStateCommitment, head.PendingTransitionRef,
		head.ReducerVersion, head.CredentialRevocationEpoch)
}

func (a *ComputerEventAppender) finalizeProjection(ctx context.Context, event Event, digest string, receipt Receipt) error {
	batch, err := a.resolveProjectionBatch(ctx, event, digest)
	if err != nil {
		return err
	}
	if a.replayProjection {
		if applier, ok := a.projection.(ReplayBatchProjectionStore); ok {
			return applier.FinalizeReplayBatch(ctx, a.computerID, digest, receipt, batch)
		}
	}
	if applier, ok := a.projection.(BatchProjectionStore); ok {
		return applier.FinalizeBatch(ctx, a.computerID, digest, receipt, batch)
	}
	if batch != nil {
		return fmt.Errorf("computer event appender: projection store cannot apply batches")
	}
	return a.projection.Finalize(ctx, a.computerID, digest, receipt)
}

func (a *ComputerEventAppender) resolveProjectionBatch(ctx context.Context, event Event, digest string) (*ProjectionBatch, error) {
	if event.EventKind != EventProjectionBatchRecorded {
		return nil, nil
	}
	plaintext, ok := a.livePayloads[event.PayloadCommitment]
	if !ok {
		if a.reader == nil {
			return nil, ErrPayloadResolverRequired
		}
		privacy := strings.TrimSpace(event.PrivacyClass)
		if privacy == "" || privacy == "owner" {
			privacy = "public"
		}
		refs := []PayloadRef{{
			ArtifactDigest: event.PayloadCommitment,
			MediaType:      ProjectionBatchMediaType,
			PrivacyClass:   privacy,
			Role:           "projection_batch",
			SchemaVersion:  1,
		}}
		resolved, err := ResolvePayloads(ctx, a.reader, a.cipher, event.ComputerID, event.EventID, refs)
		if err != nil {
			return nil, err
		}
		if len(resolved) != 1 {
			return nil, fmt.Errorf("computer event projection: one projection batch payload is required")
		}
		plaintext = resolved[0].Plaintext
	}
	batch, err := DecodeProjectionBatch(plaintext)
	if err != nil {
		return nil, err
	}
	if batch.ComputerID != event.ComputerID || batch.EventID != event.EventID {
		return nil, fmt.Errorf("%w: batch identity", ErrProjectionBatchInvalid)
	}
	if batch.EventDigest == "" {
		batch.EventDigest = digest
	} else if batch.EventDigest != digest {
		return nil, fmt.Errorf("%w: batch event digest", ErrProjectionBatchInvalid)
	}
	return &batch, nil
}

// ReconstructInto replays the canonical event source into a separate
// projection. It is intentionally a dry-run seam for state-completeness
// probes: the appender's live projection is never touched. The CAS dependency
// must also expose EventSource, as the production HTTP client does.
func (a *ComputerEventAppender) ReconstructInto(ctx context.Context, projection ProjectionStore) error {
	return a.replayInto(ctx, projection, "")
}

// ReconstructThroughTarget replays the durable chain into a separate
// projection and halts at targetHead. Restore uses this so later canonical
// events, including the restore intent, are not applied to the realized
// witness.
func (a *ComputerEventAppender) ReconstructThroughTarget(ctx context.Context, projection ProjectionStore, targetHead string) error {
	targetHead = strings.TrimSpace(targetHead)
	if !IsSHA256(targetHead) {
		return fmt.Errorf("computer event appender: restore target head is required")
	}
	return a.replayInto(ctx, projection, targetHead)
}

func (a *ComputerEventAppender) replayInto(ctx context.Context, projection ProjectionStore, targetHead string) error {
	if a == nil || projection == nil {
		return fmt.Errorf("computer event appender: replay projection is required")
	}
	source, ok := a.cas.(EventSource)
	if !ok {
		return fmt.Errorf("computer event appender: CAS does not expose event replay")
	}
	dryRun := &ComputerEventAppender{
		computerID:       a.computerID,
		pins:             a.pins,
		projection:       projection,
		cas:              a.cas,
		verifier:         a.verifier,
		reader:           a.reader,
		cipher:           a.cipher,
		replayProjection: true,
	}
	if targetHead == "" {
		return dryRun.Reconstruct(ctx, source)
	}
	return dryRun.ReconstructThrough(ctx, source, targetHead)
}

const (
	// PlatformControlTrustKeyID and PlatformControlTrustPublicKey pin the
	// repository-reviewed staging trust root. Rotation requires a reviewed
	// overlap policy before either value changes.
	PlatformControlTrustKeyID     = "868f96cca8726f99"
	PlatformControlTrustPublicKey = "sjka9TOy/Zx3Nl608bpWQ2Dft/s1yJEiMo7NzVBxwZs"
)

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
