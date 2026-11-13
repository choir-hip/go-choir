package computerevent

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
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
// Live finalization must continue through BatchProjectionStore. Because this
// path re-derives a projection from the immutable tape, the store may apply
// the versioned projection-deposit upcast here: the event, receipt, payload
// digest and reducer commitment are verified upstream and never rewritten —
// only the resolved deposit view is transformed, deterministically, in
// canonical event order (internal/store vocab_upcast_deposit.go).
type ReplayBatchProjectionStore interface {
	FinalizeReplayBatch(ctx context.Context, computerID, eventDigest string, receipt Receipt, batch *ProjectionBatch) error
}

// BatchDryRunner validates a projection batch against committed state without
// writing. The appender consults it before pin/CAS so a batch that could never
// finalize is refused to the caller instead of becoming a canonical poison
// every future boot replay dies on (2026-09-03 seq 138612). Live guards remain
// authoritative: concurrent appends between dry-run and finalize can still
// fail closed at finalize time.
type BatchDryRunner interface {
	DryRunProjectionBatch(ctx context.Context, computerID string, batch *ProjectionBatch) error
}

// BatchHeadDryRunner is a BatchDryRunner that also reports the projection head
// sequence its dry run validated against (the store's dry run reads it in the
// same transaction that applies the batch to roll it back). The head and the
// projected rows commit together, so a batch validated at head N is still
// valid while the local head is N; the appender compares instead of guessing
// which event might have changed the state.
type BatchHeadDryRunner interface {
	DryRunProjectionBatchAt(ctx context.Context, computerID string, batch *ProjectionBatch) (headSequence uint64, err error)
}

type validatedHeadKey struct{}

// WithValidatedHead tells an append that its batch was dry-run (outside the
// appender mutex) against the local projection at head sequence seq.
func WithValidatedHead(ctx context.Context, seq uint64) context.Context {
	return context.WithValue(ctx, validatedHeadKey{}, seq)
}

func validatedHeadFrom(ctx context.Context) (uint64, bool) {
	seq, ok := ctx.Value(validatedHeadKey{}).(uint64)
	return seq, ok
}

// ReplayDurability relaxes per-commit durability for a bounded replay. Each
// replayed event commits at least twice, and at synchronous=FULL each commit
// syncs the WAL: about 2.7 ms an event on d@e (2026-10-30). The replayed
// state is re-derivable from the tape, so a commit a crash loses is replayed
// again from the store's durable head. RelaxReplayDurability returns the
// restore, which the appender runs on every exit of the replay; the restore
// and CommitReplay make every earlier commit durable.
type ReplayDurability interface {
	RelaxReplayDurability(ctx context.Context) (restore func(context.Context) error, err error)
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
	computerID   string
	pins         ArtifactPinner
	projection   ProjectionStore
	cas          HeadCAS
	verifier     ReceiptVerifier
	reader       ArtifactReader
	cipher       *PrivateArtifactCipher
	livePayloads map[string][]byte
	// replayPrefetched holds the current replay page's projection-batch
	// payloads, fetched concurrently (replay_prefetch.go).
	replayPrefetched map[string][]byte
	replayProjection bool
	// replayObserver receives tail-bounded recovery telemetry: page
	// enumerations, applied sequences, and durable commits. Nil disables
	// observation with a single branch per site; recovery evidence, not
	// control flow — the observer can never alter replay.
	replayObserver ReplayObserver
	// postCommitObserver receives each committed event kind after the head
	// CAS + receipt verification + projection finalize succeed (M7): the
	// canonical derivable-continuation seam. It runs detached — the
	// append never waits and the observer can never block or alter the
	// event commit. Atomic so it can be swapped while appends run
	// (docs/archive/problems/post-commit-observer-race-2026-10-13.md).
	postCommitObserver atomic.Pointer[func(kind EventKind)]
	// appendBarrier runs first in every append entry point, before a.mu is
	// taken (docs/reports/telemetry-batching-design-2026-10-20.md 4.3). The
	// store installs the telemetry sealer's flush here, so no acknowledged
	// write reaches the tape ahead of earlier telemetry. Nil is a single
	// atomic load and no call.
	appendBarrier atomic.Pointer[func(ctx context.Context)]
	mu            sync.Mutex
	// Replay progress snapshot (guarded by mu) so the guest health surface can
	// report a replay in progress with its sequence to the host wait-for-ready
	// probe without racing the replay goroutine.
	replayActive bool
	// reconstructing is set for the whole of any reconstruct; in-place
	// prepared recovery must not race a replay's own prepared rows.
	reconstructing atomic.Bool
	// recoveryFailedAt bounds a recovery that keeps failing (a poison
	// finalize): appends refuse fast for inPlaceRecoveryCooldown after it.
	recoveryFailedAt time.Time
	// revalidations counts dry runs repeated under the mutex because the local
	// head moved since the caller's dry run (see batchGuard).
	revalidations atomic.Int64
	// catchUpPages overrides catchUpMaxPages when positive (tests).
	catchUpPages int
	// orphanPayloads are cached plaintexts of prepared rows the host has
	// passed or contradicted: kept for one catch-up pass, then released.
	orphanPayloads map[string]struct{}
	// stop is the parent of every detached recovery, re-send, finalize and
	// catch-up context (detached below). InterruptDetachedWork cancels it so a
	// shutdown, a hibernate or a store close is not held for minutes by work
	// that deliberately ignores its caller's cancellation.
	stopMu             sync.Mutex
	stop               context.Context
	cancel             context.CancelFunc
	wedge              wedgeState
	wedgeNow           func() time.Time // test clock; nil is time.Now
	replaySeq          uint64
	replayCommittedSeq uint64
	replayAppliedRows  uint64 // rows applied during this Reconstruct call
	// replayPayloadNanos is time spent fetching projection-batch payloads
	// from the host during a replay: one request per event.
	replayPayloadNanos       atomic.Int64
	replayCheckpointEvery    int
	replayCheckpointInterval time.Duration
}

// ReplayObserver records recovery-critical replay work for tail-bounded cost
// evidence. Implementations must be goroutine-safe and non-blocking; replay
// never waits for observation.
type ReplayObserver interface {
	PageFetched(afterSequence uint64, count int)
	RecordApplied(sequence uint64)
	CheckpointCommitted(sequence uint64)
}

// ErrBatchRefused marks a projection batch refused before it was sent to the
// host: it did not decode, or the projection's dry run rejected it (at the
// caller, or again under the appender mutex after the local state moved).
// Nothing of it is canonical: it is not on the tape, and the host never saw the
// event (a payload or event pin may exist, which is storage and not canon). A
// caller that built it from separable records may safely split it and write
// the pieces. Any other AppendNewPayload failure may have an unknown outcome.
var ErrBatchRefused = errors.New("computer event appender: projection batch refused")

// batchRefusedError carries the original message unchanged and matches
// ErrBatchRefused under errors.Is, so existing error text does not move.
type batchRefusedError struct{ err error }

func (e batchRefusedError) Error() string        { return e.err.Error() }
func (e batchRefusedError) Unwrap() error        { return e.err }
func (e batchRefusedError) Is(target error) bool { return target == ErrBatchRefused }

func batchRefused(err error) error { return batchRefusedError{err: err} }

type barrierExemptKey struct{}

// WithBarrierExemption marks ctx so appends made under it skip the append
// barrier. Exactly three callers may use it: the telemetry sealer's own flush,
// which is what the barrier waits for; the run-memory writer, which is
// deliberately not ordered against telemetry (design 4.3, "the one
// exemption"); and the tape group-commit leader, which holds the sealer's
// flush lock and writes the pending telemetry into its own event ahead of its
// members (docs/design/tape-group-commit-2026-10-31.md). A source-scan test
// fails if a fourth appears.
func WithBarrierExemption(ctx context.Context) context.Context {
	return context.WithValue(ctx, barrierExemptKey{}, true)
}

// SetAppendBarrier installs the hook every append entry point runs first.
// Nil removes it.
func (a *ComputerEventAppender) SetAppendBarrier(fn func(ctx context.Context)) {
	if a == nil {
		return
	}
	if fn == nil {
		a.appendBarrier.Store(nil)
		return
	}
	a.appendBarrier.Store(&fn)
}

// awaitBarrier runs the append barrier unless ctx is exempt. It must be called
// before a.mu is taken: the barrier flushes through this appender.
func (a *ComputerEventAppender) awaitBarrier(ctx context.Context) {
	fn := a.appendBarrier.Load()
	if fn == nil {
		return
	}
	if exempt, _ := ctx.Value(barrierExemptKey{}).(bool); exempt {
		return
	}
	(*fn)(ctx)
}

// HeadsAligned reports whether the local projection head is the platform's
// canonical head: nil when they are equal (two nil heads included), an error
// wrapping ErrNeedsProjectionRepair when they differ in either direction. It
// is the precondition of the stream_seq lease's exact seed
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 2):
// with equal heads replay cannot add events during the scan, and the lease
// append, which appendLocked admits only on equal heads, lands on the state
// that was scanned. It takes no appender lock; the caller serialises.
func (a *ComputerEventAppender) HeadsAligned(ctx context.Context) error {
	if a == nil {
		return fmt.Errorf("computer event appender: heads check requires an appender")
	}
	platformHead, err := a.cas.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: resolve canonical head: %w", err)
	}
	localHead, err := a.projection.Head(ctx, a.computerID)
	if err != nil {
		return fmt.Errorf("computer event appender: resolve embedded head: %w", err)
	}
	if !sameHead(platformHead, localHead) {
		return fmt.Errorf("%w: local=%s platform=%s", ErrNeedsProjectionRepair, replayHeadSummary(localHead), replayHeadSummary(platformHead))
	}
	return nil
}

// batchGuard keeps a projection batch from reaching the CAS after the local
// state it was validated against has moved. It records the local head sequence
// the batch's last dry run saw (the store's dry run reads it in the same
// transaction that would apply the batch; head and projected rows commit
// together, so an unchanged head means unchanged state). Under the appender
// mutex, before the event is pinned or prepared and again after any in-place
// recovery, check compares that sequence with the current local head: equal
// costs one comparison; different, for any reason (this append's recovery,
// another writer's recovery or ordinary finalize), it re-runs the dry run
// against the current state and refuses the batch (ErrBatchRefused) if the
// state no longer accepts it. It replaces a "did a recovery happen in this
// append" trigger, which missed every other writer's head move.
type batchGuard struct {
	a       *ComputerEventAppender
	payload []byte // the bytes dry-run: what finalize and replay read
	seq     uint64 // local head sequence the last dry run validated against
	known   bool   // the dry run reported a head (a projection without one never does)
}

// newBatchGuard is nil for anything but a projection batch, and when the
// projection cannot dry-run.
func (a *ComputerEventAppender) newBatchGuard(mediaType string, payload []byte) *batchGuard {
	if strings.TrimSpace(mediaType) != ProjectionBatchMediaType {
		return nil
	}
	if _, ok := a.projection.(BatchDryRunner); !ok {
		return nil
	}
	return &batchGuard{a: a, payload: payload}
}

// dryRunBatch dry-runs a decoded batch through the projection, reporting the
// head it validated against when the projection can.
func (a *ComputerEventAppender) dryRunBatch(ctx context.Context, batch *ProjectionBatch) (seq uint64, known bool, err error) {
	if runner, ok := a.projection.(BatchHeadDryRunner); ok {
		seq, err = runner.DryRunProjectionBatchAt(ctx, a.computerID, batch)
		return seq, err == nil, err
	}
	runner, ok := a.projection.(BatchDryRunner)
	if !ok {
		return 0, false, nil
	}
	return 0, false, runner.DryRunProjectionBatch(ctx, a.computerID, batch)
}

// record notes the head a dry run just validated against.
func (g *batchGuard) record(seq uint64, known bool) {
	if g != nil {
		g.seq, g.known = seq, known
	}
}

// check is called under the appender mutex with the current local head. force
// re-validates when the dry run never reported a head (after a recovery, when
// the state may have moved and there is nothing to compare).
func (g *batchGuard) check(ctx context.Context, local *Head, force bool) error {
	if g == nil {
		return nil
	}
	if g.known {
		if headSeq(local) == g.seq {
			return nil
		}
	} else if !force {
		return nil
	}
	g.a.revalidations.Add(1)
	batch, err := DecodeProjectionBatch(g.payload)
	if err != nil {
		return batchRefused(fmt.Errorf("computer event appender: projection dry-run decode after the local head moved: %w", err))
	}
	seq, known, err := g.a.dryRunBatch(ctx, &batch)
	if err != nil {
		return batchRefused(fmt.Errorf("computer event appender: projection dry-run refused batch after the local head moved since it was validated: %w", err))
	}
	g.record(seq, known)
	return nil
}

// Revalidations is how many dry runs this appender repeated under its mutex
// because the local head had moved since the caller's dry run.
func (a *ComputerEventAppender) Revalidations() int64 { return a.revalidations.Load() }

// HoldAppends runs fn while no event can be appended. Every append and its
// live projection run under the same lock, so fn sees a head and a projection
// that belong together. Keep fn short: every writer waits for it.
func (a *ComputerEventAppender) HoldAppends(fn func() error) error {
	if a == nil {
		return fn()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return fn()
}

// SetReplayObserver installs the recovery telemetry observer. Nil disables.
func (a *ComputerEventAppender) SetReplayObserver(observer ReplayObserver) {
	if a == nil {
		return
	}
	a.replayObserver = observer
}

// SetPostCommitObserver installs the derivable-continuation observer
// (M7): it receives each event's kind exactly once, after the commit is
// durable (head CAS + receipt verify + projection finalize). The call is
// detached — implementors must be non-blocking and must not append back
// into the same appender synchronously. Nil disables.
func (a *ComputerEventAppender) SetPostCommitObserver(observer func(kind EventKind)) {
	if a == nil {
		return
	}
	if observer == nil {
		a.postCommitObserver.Store(nil)
		return
	}
	a.postCommitObserver.Store(&observer)
}

// ReplaySnapshot describes the durable replay progress for the liveness probe.
type ReplaySnapshot struct {
	InProgress        bool   `json:"in_progress"`
	Sequence          uint64 `json:"sequence"`
	CommittedSequence uint64 `json:"committed_sequence"`
	AppliedRows       uint64 `json:"applied_rows"` // rows applied during the current/last Reconstruct
}

// ReplaySnapshot returns the current durable replay progress. Zero value means
// no replay is running.
func (a *ComputerEventAppender) ReplaySnapshot() ReplaySnapshot {
	if a == nil {
		return ReplaySnapshot{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return ReplaySnapshot{InProgress: a.replayActive, Sequence: a.replaySeq, CommittedSequence: a.replayCommittedSeq, AppliedRows: a.replayAppliedRows}
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
		computerID:               computerID,
		pins:                     pins,
		projection:               projection,
		cas:                      cas,
		verifier:                 verifier,
		replayCheckpointEvery:    500,
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
	a.awaitBarrier(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.appendLocked(ctx, event, input, payloadPinReceiptDigests)
}

// AppendNew serializes a new semantic event, binds it to the current canonical
// and effective projections, computes the exact request commitment, and appends
// it through the sole event writer.
func (a *ComputerEventAppender) AppendNew(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string) (Receipt, error) {
	a.awaitBarrier(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()

	bound, err := a.bindCurrentHeadLocked(ctx, &event)
	if err != nil {
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
	return a.appendBoundLocked(ctx, event, input, payloadPinReceiptDigests, bound, nil)
}

// AppendNewPayload content-addresses and pins one non-private payload before
// appending the event that names it. The payload receipt is bound into the
// event request commitment; no process-local tape is authoritative.
func (a *ComputerEventAppender) AppendNewPayload(ctx context.Context, event Event, input TransitionInput, payload []byte, mediaType, privacyClass string) (Receipt, string, error) {
	a.awaitBarrier(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()

	pinner, ok := a.pins.(NonPrivatePayloadPinner)
	if !ok {
		return Receipt{}, "", fmt.Errorf("computer event appender: non-private payload pinning unavailable")
	}
	bound, err := a.bindCurrentHeadLocked(ctx, &event)
	if err != nil {
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
	var guard *batchGuard
	if strings.TrimSpace(mediaType) == ProjectionBatchMediaType {
		if guard = a.newBatchGuard(mediaType, payload); guard != nil {
			batch, derr := DecodeProjectionBatch(payload)
			if derr != nil {
				return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: projection dry-run decode: %w", derr))
			}
			seq, known, derr := a.dryRunBatch(ctx, &batch)
			if derr != nil {
				return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: projection dry-run refused batch: %w", derr))
			}
			guard.record(seq, known)
		}
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
	receipt, err := a.appendBoundLocked(ctx, event, input, []string{payloadReceiptDigest}, bound, guard)
	if err == nil {
		// Finalized: drop the cached plaintext, or every payload this process
		// ever appended stays on the heap (docs/problems/guest-appender-
		// retains-every-projection-payload-2026-10-19.md).
		delete(a.livePayloads, payloadDigest)
	} else {
		// After a failure it stays only while a prepared row may still be
		// finalized from it (the recovery paths read through to the artifact
		// store without it, but need not); a refusal left no row, and 1,600
		// refusals of a wedge must not each keep a plaintext (F23).
		pctx, pcancel := a.detached(ctx, 30*time.Second)
		prepared, perr := a.projection.Prepared(pctx, a.computerID)
		pcancel()
		held := perr != nil
		for _, row := range prepared {
			if row.Event.PayloadCommitment == payloadDigest {
				held = true
			}
		}
		if !held {
			delete(a.livePayloads, payloadDigest)
		}
	}
	return receipt, payloadDigest, err
}

// AppendNewPrivatePayload encrypts, authenticates, and pins one private payload
// before appending the event that names its immutable envelope.
func (a *ComputerEventAppender) AppendNewPrivatePayload(ctx context.Context, event Event, input TransitionInput, plaintext []byte, mediaType string, cipher *PrivateArtifactCipher) (Receipt, string, error) {
	a.awaitBarrier(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()

	pinner, ok := a.pins.(PrivatePayloadPinner)
	if !ok || cipher == nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: private payload authority unavailable")
	}
	bound, err := a.bindCurrentHeadLocked(ctx, &event)
	if err != nil {
		return Receipt{}, "", err
	}
	envelope, sealedMeta, err := pinner.PreparePrivatePayload(ctx, cipher, a.computerID, event.EventID, mediaType, plaintext)
	if err != nil {
		return Receipt{}, "", fmt.Errorf("computer event appender: encrypt private payload: %w", err)
	}
	// The store dry-ran this batch before sealing and says against which local
	// head (the context value); check compares it under the mutex.
	guard := a.newBatchGuard(mediaType, plaintext)
	if seq, ok := validatedHeadFrom(ctx); ok && guard != nil {
		guard.record(seq, true)
	} else if guard != nil {
		// The caller did not validate the batch (a direct append: no leader
		// adjacency to make an outside dry run worth it): validate it here,
		// once, under the mutex, against the head it will be appended on.
		batch, derr := DecodeProjectionBatch(plaintext)
		if derr != nil {
			return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: projection dry-run decode: %w", derr))
		}
		seq, known, derr := a.dryRunBatch(ctx, &batch)
		if derr != nil {
			return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: projection dry-run refused batch: %w", derr))
		}
		guard.record(seq, known)
	}
	if strings.TrimSpace(mediaType) == ProjectionBatchMediaType && len(sealedMeta.SecretHandles) > 0 {
		// The caller dry-ran the batch before sealing; redaction changed it
		// (it named handles; with none the sealed bytes are the caller's).
		// What finalize and every replay will read is this envelope, so it
		// is opened and dry-run here, and a batch that does not survive its
		// own sealing is refused, never canonical
		// (docs/problems/redaction-breaks-sealed-projection-batch-2026-10-31.md).
		sealed, _, err := cipher.Decrypt(ctx, envelope, a.computerID, event.EventID)
		if err != nil {
			return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: open sealed batch: %w", err))
		}
		if guard != nil {
			guard.payload = sealed // what finalize and replay read
		}
		batch, err := DecodeProjectionBatch(sealed)
		if err != nil {
			return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: sealed batch decode: %w", err))
		}
		if guard != nil {
			seq, known, err := a.dryRunBatch(ctx, &batch)
			if err != nil {
				return Receipt{}, "", batchRefused(fmt.Errorf("computer event appender: projection dry-run refused sealed batch: %w", err))
			}
			guard.record(seq, known)
		}
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
	receipt, err := a.appendBoundLocked(ctx, event, input, []string{payloadReceiptDigest}, bound, guard)
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
	a.awaitBarrier(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(payloads) == 0 {
		return Receipt{}, nil, fmt.Errorf("computer event appender: payload set is empty")
	}
	bound, err := a.bindCurrentHeadLocked(ctx, &event)
	if err != nil {
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
	receipt, err := a.appendBoundLocked(ctx, event, input, receiptDigests, bound, nil)
	return receipt, artifactDigests, err
}

// boundHead is the platform head a new event was bound to, read once under
// the appender mutex. appendLockedInner reuses it as the platform head for the
// embedded-head comparison and the reducer instead of reading it again: two
// reads per append were two of the six corpusd calls each tape event cost.
//
// The reuse is safe because the head is a guess the CAS checks. The event
// carries the bound head as its previous head and the host's compare-and-swap
// refuses a request whose previous head is not the host's (definitely, so the
// prepared row is discarded); the reducer refuses it earlier when a recovery
// moved the head. A head that moves between the bind and the CAS (a host-side
// event during the payload pin) used to be seen by the second read and now
// fails at the CAS, and the next append's bind reads the new head and the
// embedded-head comparison repairs the gap, as it does for any host-side
// event (heads_differ recovery). Nothing is committed on a stale head either
// way (TestAppendNewPayloadHeadMovedDuringPinIsRefused).
type boundHead struct{ head *Head }

func (a *ComputerEventAppender) bindCurrentHeadLocked(ctx context.Context, event *Event) (*boundHead, error) {
	head, err := a.cas.Head(ctx, a.computerID)
	if err != nil {
		return nil, fmt.Errorf("computer event appender: resolve head for new event: %w", err)
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
				return nil, fmt.Errorf("computer event appender: expected %s changed", binding.name)
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
		return &boundHead{head: head}, nil
	}
	event.Sequence = head.Sequence + 1
	event.PreviousHead = head.CanonicalEventHead
	event.ExpectedDesiredEventHead = head.DesiredEventHead
	event.ExpectedEffectiveEventHead = head.EffectiveEventHead
	event.ExpectedPendingTransitionRef = head.PendingTransitionRef
	event.ExpectedDesiredStateCommitment = head.DesiredStateCommitment
	event.ExpectedEffectiveStateCommitment = head.EffectiveStateCommitment
	return &boundHead{head: head}, nil
}

func (a *ComputerEventAppender) appendLocked(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string) (Receipt, error) {
	return a.appendBoundLocked(ctx, event, input, payloadPinReceiptDigests, nil, nil)
}

// appendBoundLocked is appendLocked for an event bindCurrentHeadLocked just
// bound: bound carries the platform head that bind read, so the append does
// not read it a second time. A nil bound reads it.
//
// guard, when set, keeps a projection batch from reaching the CAS after the
// local state it was validated against has moved (see batchGuard); nil for
// events that carry no batch.
func (a *ComputerEventAppender) appendBoundLocked(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string, bound *boundHead, guard *batchGuard) (Receipt, error) {
	receipt, err := a.appendLockedInner(ctx, event, input, payloadPinReceiptDigests, bound, guard)
	a.noteAppendResult(err)
	return receipt, err
}

func (a *ComputerEventAppender) appendLockedInner(ctx context.Context, event Event, input TransitionInput, payloadPinReceiptDigests []string, bound *boundHead, guard *batchGuard) (Receipt, error) {
	if event.ComputerID != a.computerID {
		return Receipt{}, fmt.Errorf("computer event appender: wrong computer")
	}
	// Serving fence: every live append must speak the active V2 vocabulary.
	// Replay never reaches appendLocked (reconstruct applies tape events
	// directly), so historic V1 events are unaffected; frozen protocol
	// (owner, trusted-core) always passes.
	if err := vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV2,
		vocabmigrate.EventActorFields(event.ActorProfile)...); err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: %w", err)
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
	var platformHead *Head
	if bound != nil {
		platformHead = bound.head
	} else if platformHead, err = a.cas.Head(ctx, a.computerID); err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: resolve canonical head: %w", err)
	}
	embeddedHead, err := a.projection.Head(ctx, a.computerID)
	if err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: resolve embedded head: %w", err)
	}
	a.noteHeads(headSeq(embeddedHead), headSeq(platformHead))
	// The batch was validated against some local head; if the local head is
	// not that one, another writer (or a recovery) changed the state since:
	// validate it again now, under the mutex, before anything else is sent.
	if err := guard.check(ctx, embeddedHead, false); err != nil {
		return Receipt{}, err
	}
	if !sameHead(platformHead, embeddedHead) {
		// A prepared row a failed earlier append left behind (a post-CAS
		// finalize, or a discard that never ran) wedged the running computer
		// until its next boot (2026-11-02, the paper, twice). Run the boot's
		// own recovery in place; appends are serialized here, so every
		// prepared row is an orphan.
		if embeddedHead, err = a.recoverPreparedInPlaceLocked(ctx, "heads differ"); err != nil {
			return Receipt{}, err
		}
		// Recovery may have moved the host's head as seen by this append (a
		// resent committed row, a catch-up); bind to the head as it is now.
		if platformHead, err = a.cas.Head(ctx, a.computerID); err != nil {
			return Receipt{}, fmt.Errorf("computer event appender: resolve canonical head after recovery: %w", err)
		}
		if !sameHead(platformHead, embeddedHead) {
			return Receipt{}, &wedgeError{kind: "heads differ after in-place recovery", err: ErrNeedsProjectionRepair}
		}
		// The recovery may have applied events (a landed row finalized, a
		// catch-up from the host's tape) that the batch was validated without:
		// compare heads again (forced when the dry run could not report one).
		if err := guard.check(ctx, embeddedHead, true); err != nil {
			return Receipt{}, err
		}
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
		// An orphaned prepared row at this sequence fails the insert on the
		// index's (computer_id, sequence) key; clear it and retry once.
		recovered, rerr := a.recoverPreparedInPlaceLocked(ctx, "prepare failed: "+err.Error())
		if rerr != nil {
			return Receipt{}, &wedgeError{kind: prepareKind(err), err: fmt.Errorf("computer event appender: prepare embedded projection: %w (recovery: %v)", err, rerr)}
		}
		if headSeq(recovered)+1 != event.Sequence {
			// The recovery moved the local head (a catch-up applied events at
			// or past this event's sequence): the bound event is stale. Refuse
			// it cleanly; the next append binds to the head as it is now.
			return Receipt{}, fmt.Errorf("computer event appender: the local head moved to %d during prepare recovery; event bound at sequence %d not appended", headSeq(recovered), event.Sequence)
		}
		// Same guard as after the heads-differ recovery. The event and payload
		// are pinned by now (a pin is storage, not canon): refuse, send nothing.
		if gerr := guard.check(ctx, recovered, true); gerr != nil {
			return Receipt{}, gerr
		}
		if err := a.projection.Prepare(ctx, request); err != nil {
			return Receipt{}, &wedgeError{kind: prepareKind(err), err: fmt.Errorf("computer event appender: prepare embedded projection: %w", err)}
		}
	}
	receipt, err := a.cas.CompareAndSwap(ctx, request)
	if err != nil {
		if CASRefusedDefinitely(err) {
			a.discardPreparedLocked(ctx, request)
			return Receipt{}, fmt.Errorf("computer event appender: head CAS: %w", err)
		}
		// The outcome is unknown: the host may have committed before the
		// reply was lost (a reset, a cancelled caller, a 5xx after the
		// commit). Deleting the prepared row here left the local projection
		// one behind with nothing to recover from, and the computer refused
		// every write until a boot (the paper, 2026-11-02 and 2026-11-12).
		// Ask the host by re-sending the same request.
		if receipt, err = a.resendUnclearCASLocked(ctx, request, err); err != nil {
			return Receipt{}, err
		}
	}
	// With a receipt in hand the event is canonical: the caller going away
	// (a cancelled turn, the incident's likeliest trigger) must not fail the
	// local finalize, which begins its transaction on the ctx it is given.
	fctx, fcancel := a.detached(ctx, finalizeTimeout)
	defer fcancel()
	if err := a.verifier.VerifyEventHeadReceipt(fctx, receipt, request); err != nil {
		return Receipt{}, fmt.Errorf("computer event appender: verify head receipt: %w", err)
	}
	if err := a.finalizeProjection(fctx, event, digest, receipt); err != nil {
		if errors.Is(err, ErrProjectionBatchUnreadable) {
			return Receipt{}, fmt.Errorf("computer event appender: sequence %d: %w", event.Sequence, err)
		}
		return Receipt{}, fmt.Errorf("%w: finalize embedded projection: %w", ErrNeedsProjectionRepair, ClassifyProjectionFailure(err))
	}
	if observer := a.postCommitObserver.Load(); observer != nil {
		go (*observer)(event.EventKind)
	}
	return receipt, nil
}

const (
	// casResendTimeout bounds the re-send of a CAS whose outcome is unknown.
	// It is a deadline, not a limit on calls; appends wait behind it under a.mu.
	casResendTimeout = 30 * time.Second
	// finalizeTimeout bounds verify and finalize once a receipt is in hand. It
	// replaces the caller's context, which used to govern them, so it is sized
	// from the worst payload: an event payload is at most
	// EventPayloadMaxResponseBytes (64 MiB); fetched at a pessimistic 1 MiB/s
	// across the host tap under load that is about 64 s, decode, decrypt and
	// the SQLite apply of a batch add tens of seconds. Five minutes is more
	// than three times that and stays half of the 10 minute bound of the
	// guest's HTTP client.
	finalizeTimeout = 5 * time.Minute
)

// detached returns a context that keeps ctx's values but not its cancellation,
// ends at timeout, and ends when InterruptDetachedWork runs. Work that must
// settle canonical state after its caller left uses it; shutdown still stops it.
func (a *ComputerEventAppender) detached(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	stop := a.currentStop()
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	unregister := context.AfterFunc(stop, cancel)
	return dctx, func() { unregister(); cancel() }
}

func (a *ComputerEventAppender) currentStop() context.Context {
	a.stopMu.Lock()
	defer a.stopMu.Unlock()
	if a.stop == nil {
		a.stop, a.cancel = context.WithCancel(context.Background())
	}
	return a.stop
}

// InterruptDetachedWork cancels the detached recovery, re-send, finalize and
// catch-up work in flight, so shutdown (Runtime.Stop, Store.Close) is not held
// by it. Whatever it leaves is recovered by the next append or the next boot:
// a prepared row is resolved from the host's chain, and a half-applied
// catch-up resumes. The appender stays usable; later detached work runs under
// a fresh stop context.
func (a *ComputerEventAppender) InterruptDetachedWork() {
	if a == nil {
		return
	}
	a.stopMu.Lock()
	cancel := a.cancel
	a.stop, a.cancel = context.WithCancel(context.Background())
	a.stopMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// discardPreparedLocked drops the prepared row of a request the host refused,
// on a context that outlives a cancelled caller.
func (a *ComputerEventAppender) discardPreparedLocked(ctx context.Context, request CASRequest) {
	// The discard must not share a cancelled ctx with the CAS, or the
	// prepared row outlives the failed append and blocks the next one.
	dctx, cancel := a.detached(ctx, 30*time.Second)
	defer cancel()
	if derr := a.projection.DiscardPrepared(dctx, a.computerID, request.EventDigest); derr != nil {
		log.Printf("computer event appender: discard prepared seq=%d after a refused CAS or failed apply: %v", request.Event.Sequence, derr)
	} else {
		a.dropLivePayload(request.Event)
	}
}

// dropLivePayload releases the cached plaintext of a projection payload once
// no prepared row needs it.
func (a *ComputerEventAppender) dropLivePayload(event Event) {
	delete(a.livePayloads, event.PayloadCommitment)
}

// resendUnclearCASLocked settles a CAS that returned an error without proving
// a refusal. It re-sends the same request on a context that outlives a
// cancelled caller: the host answers a request it already committed with the
// stored receipt (it checks the idempotency key under the computer's lock),
// commits one it never saw, and refuses one that cannot commit.
//
// Any failure of the re-send keeps the prepared row. A refusal of the re-send
// does not prove that the first send did not commit: the host runs
// authorization and pin validation before it looks up the idempotency key, and
// maps unclassified internal errors to 400. RecoverPrepared decides on the
// next pass against the host's chain: when the host's head is that event it
// re-sends once more and finalizes, and if that re-send is refused too it
// drops the row and the catch-up applies the event from the host's tape; it
// discards the row when the host holds another event at its sequence or has
// not reached it. Only a definite refusal of the FIRST send
// discards at once.
func (a *ComputerEventAppender) resendUnclearCASLocked(ctx context.Context, request CASRequest, first error) (Receipt, error) {
	rctx, cancel := a.detached(ctx, casResendTimeout)
	defer cancel()
	receipt, err := a.cas.CompareAndSwap(rctx, request)
	if err == nil {
		log.Printf("computer event appender: seq=%d head CAS outcome was unclear (%v); the re-send settled it", request.Event.Sequence, first)
		return receipt, nil
	}
	log.Printf("computer event appender: seq=%d head CAS outcome unknown after a re-send (%v; first: %v); prepared row kept for recovery", request.Event.Sequence, err, first)
	return Receipt{}, fmt.Errorf("computer event appender: head CAS outcome unknown, prepared row kept for recovery: %w (re-send: %v)", first, err)
}

// recoverPreparedInPlaceLocked runs RecoverPrepared on a running computer and
// returns the embedded head after it. Callers hold a.mu.
func (a *ComputerEventAppender) recoverPreparedInPlaceLocked(ctx context.Context, why string) (*Head, error) {
	if a.replayProjection || a.replayActive || a.reconstructing.Load() {
		return nil, ErrNeedsProjectionRepair
	}
	if !a.recoveryFailedAt.IsZero() && time.Since(a.recoveryFailedAt) < inPlaceRecoveryCooldown {
		return nil, ErrNeedsProjectionRepair
	}
	stop := a.currentStop()
	// A failure while the work was interrupted (a shutdown, a store close) is
	// not the recovery failing: it gets no cooldown.
	failed := func() {
		if stop.Err() == nil {
			a.recoveryFailedAt = time.Now()
		}
	}
	rctx, cancel := a.detached(ctx, time.Minute)
	defer cancel()
	err := a.RecoverPrepared(rctx)
	head, herr := a.projection.Head(rctx, a.computerID)
	log.Printf("computer event appender: in-place prepared recovery (%s): err=%v embedded_seq=%d", why, err, headSeq(head))
	if err != nil {
		failed()
		return nil, err
	}
	a.recoveryFailedAt = time.Time{}
	if herr != nil {
		return nil, fmt.Errorf("computer event appender: embedded head after recovery: %w", herr)
	}
	// No prepared row explains the gap: if the host is simply ahead, fetch
	// what the local projection missed, as the boot does.
	head, err = a.catchUpLocked(ctx, head)
	if err != nil {
		var we *wedgeError
		if !(errors.As(err, &we) && we.kind == wedgeKindCatchUpIncomplete) {
			failed() // an incomplete catch-up made progress: resume at once
		}
		return nil, err
	}
	return head, nil
}

const inPlaceRecoveryCooldown = 30 * time.Second

func headSeq(h *Head) uint64 {
	if h == nil {
		return 0
	}
	return h.Sequence
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
		// Serving fence for recovered prepared events: a prepared row may
		// carry a historic V1 actor (committed pre-cutover, legal tape
		// history) or a V2 actor, so the check accepts either vocabulary and
		// refuses only tokens known to neither.
		fields := vocabmigrate.EventActorFields(request.Event.ActorProfile)
		if errV2 := vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV2, fields...); errV2 != nil {
			if errV1 := vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV1, fields...); errV1 != nil {
				return fmt.Errorf("computer event appender: prepared event actor %q is not in any known vocabulary", request.Event.ActorProfile)
			}
		}
		platformHead, err := a.cas.Head(ctx, a.computerID)
		if err != nil {
			return fmt.Errorf("computer event appender: recovery head: %w", err)
		}
		if platformHead != nil && platformHead.Sequence == request.Event.Sequence && platformHead.CanonicalEventHead == request.EventDigest {
			receipt, err := a.cas.CompareAndSwap(ctx, request)
			if err != nil {
				// The head read already proves this event canonical, so a
				// failed re-send cannot be left to wedge the recovery: the host
				// runs authorization and pin validation before it looks up the
				// idempotency key, and may refuse a re-send of a committed
				// event. Drop the row and let the catch-up that follows apply
				// the event from the host's tape with its signed receipt. The
				// cached plaintext stays for that apply, which releases it.
				log.Printf("computer event appender: seq=%d the re-send of a committed row failed (%v); resolving it from the host chain", request.Event.Sequence, err)
				if derr := a.projection.DiscardPrepared(ctx, a.computerID, request.EventDigest); derr != nil {
					return fmt.Errorf("computer event appender: recover durable receipt: %w (discard: %v)", err, derr)
				}
				continue
			}
			if err := a.verifier.VerifyEventHeadReceipt(ctx, receipt, request); err != nil {
				return fmt.Errorf("computer event appender: verify recovery receipt: %w", err)
			}
			if err := a.finalizeProjection(ctx, request.Event, request.EventDigest, receipt); err != nil && !errors.Is(err, ErrProjectionBatchUnreadable) {
				return fmt.Errorf("%w: finalize recovery: %w", ErrNeedsProjectionRepair, ClassifyProjectionFailure(err))
			}
			a.dropLivePayload(request.Event)
			continue
		}
		if (platformHead == nil && request.Event.Sequence == 1 && request.Event.PreviousHead == ZeroHead) ||
			(platformHead != nil && platformHead.Sequence+1 == request.Event.Sequence && platformHead.CanonicalEventHead == request.Event.PreviousHead) {
			if err := a.projection.DiscardPrepared(ctx, a.computerID, request.EventDigest); err != nil {
				return fmt.Errorf("computer event appender: discard uncommitted projection: %w", err)
			}
			a.dropLivePayload(request.Event)
			continue
		}
		if platformHead != nil && request.Event.Sequence <= platformHead.Sequence {
			// The host's chain has moved past this row, or holds another event
			// at its sequence. The host is authoritative and the row is only a
			// local note of an attempt: drop it. When the host did commit this
			// very event (the row of an append whose outcome was unknown, and
			// the chain moved on since), the catch-up applies it from the host's
			// tape through the same step as the boot replay. Its cached
			// plaintext stays for that apply; whatever the pass does not use is
			// released when the catch-up ends (orphanPayloads), since the host
			// may hold a different event at that sequence.
			if err := a.projection.DiscardPrepared(ctx, a.computerID, request.EventDigest); err != nil {
				return fmt.Errorf("computer event appender: discard prepared projection the host has passed: %w", err)
			}
			if _, cached := a.livePayloads[request.Event.PayloadCommitment]; cached {
				if a.orphanPayloads == nil {
					a.orphanPayloads = map[string]struct{}{}
				}
				a.orphanPayloads[request.Event.PayloadCommitment] = struct{}{}
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

func (a *ComputerEventAppender) reconstruct(ctx context.Context, source EventSource, targetHead string) (err error) {
	if source == nil {
		return fmt.Errorf("computer event appender: event source is required")
	}
	a.reconstructing.Store(true)
	defer a.reconstructing.Store(false)
	if a.replayProjection {
		a.setReplayActive(true)
		defer a.setReplayActive(false)
		if relaxer, ok := a.projection.(ReplayDurability); ok {
			restore, err := relaxer.RelaxReplayDurability(ctx)
			if err != nil {
				return fmt.Errorf("computer event appender: relax replay durability: %w", err)
			}
			defer func() {
				if rerr := restore(context.WithoutCancel(ctx)); rerr != nil {
					log.Printf("computer event appender: restore replay durability: %v", rerr)
					if err == nil {
						err = fmt.Errorf("computer event appender: restore replay durability: %w", rerr)
					}
				}
			}()
		}
	}
	// Zero the per-boot applied counter so the snapshot's applied_rows names
	// this reconstruct's delta, not the running process's cumulative history.
	a.mu.Lock()
	a.replayAppliedRows = 0
	a.mu.Unlock()
	// One line per replay that applied anything: the replay rate is what the
	// recovery bound should be stated in (design object-graph §11.1), and it
	// was unmeasured.
	// Where the time goes: fetching pages from the host, applying events
	// (reduce, receipt verify, projection writes), and periodic commits.
	var replayFetch, replayPrefetch, replayApply, replayCommit time.Duration
	var replayReduce, replayVerify, replayPrepare, replayFinalize time.Duration
	replayPages := 0
	a.replayPayloadNanos.Store(0)
	defer func() { a.replayPrefetched = nil }()
	replayStarted := time.Now()
	defer func() {
		snap := a.ReplaySnapshot()
		if snap.AppliedRows == 0 {
			return
		}
		elapsed := time.Since(replayStarted)
		log.Printf("computer event appender: replay applied %d events through seq=%d in %s (%.0f events/s; fetch %s in %d pages, payload prefetch %s, apply %s: reduce %s, verify %s, prepare %s, finalize %s of which payload fetch %s; commit %s)", snap.AppliedRows, snap.Sequence, elapsed.Round(time.Millisecond), float64(snap.AppliedRows)/max(elapsed.Seconds(), 1e-3),
			replayFetch.Round(time.Millisecond), replayPages, replayPrefetch.Round(time.Millisecond), replayApply.Round(time.Millisecond),
			replayReduce.Round(time.Millisecond), replayVerify.Round(time.Millisecond), replayPrepare.Round(time.Millisecond), replayFinalize.Round(time.Millisecond),
			time.Duration(a.replayPayloadNanos.Load()).Round(time.Millisecond), replayCommit.Round(time.Millisecond))
	}()
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
		next, timings, err := a.applyDurableEvent(ctx, current, record)
		replayReduce += timings.reduce
		replayVerify += timings.verify
		replayPrepare += timings.prepare
		replayFinalize += timings.finalize
		if err != nil {
			return err
		}
		a.setReplayProgress(record.Request.Event.Sequence, a.committedReplaySeq())
		a.mu.Lock()
		a.replayAppliedRows++
		a.mu.Unlock()
		if observer := a.replayObserver; observer != nil {
			observer.RecordApplied(record.Request.Event.Sequence)
		}
		replayApply += time.Since(applyStarted)
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
			replayCommit += time.Since(commitStarted)
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
			replayFetch += time.Since(pageStarted)
			replayPages++
			if observer := a.replayObserver; observer != nil {
				observer.PageFetched(after, len(page))
			}
			if elapsed := time.Since(pageStarted); elapsed > 2*time.Second {
				log.Printf("computer event appender: replay page fetch after=%d count=%d elapsed=%s", after, len(page), elapsed)
			}
			if len(page) == 0 {
				break
			}
			prefetchStarted := time.Now()
			a.replayPrefetched = a.prefetchReplayPayloads(ctx, page)
			replayPrefetch += time.Since(prefetchStarted)
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
			a.replayPrefetched = nil
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

// applyTimings splits the cost of one applyDurableEvent for the replay log.
type applyTimings struct{ reduce, verify, prepare, finalize time.Duration }

// applyDurableEvent applies one event of the canonical tape to the local
// projection: reduce it against current, check the result against the head the
// host committed, verify the host's receipt, prepare, and finalize. The boot
// replay (reconstruct) and the in-place catch-up (catchUpLocked) both apply
// events through this one step: the reduce, the check against the committed
// head, the receipt verification, the prepare and the finalize. They differ in
// one respect: replay mode finalizes through FinalizeReplayBatch (which may
// upcast a legacy deposit) and the catch-up through the strict live
// FinalizeBatch, so an old event that only the upcast accepts is a poison for
// the catch-up and waits for the boot. It never performs a CAS.
func (a *ComputerEventAppender) applyDurableEvent(ctx context.Context, current *Head, record DurableEvent) (Head, applyTimings, error) {
	var timings applyTimings
	applyStarted := time.Now()
	next, err := Reduce(current, record.Request.Event, record.Request.Input)
	if err != nil {
		return Head{}, timings, fmt.Errorf("computer event appender: replay sequence %d: %w", record.Request.Event.Sequence, err)
	}
	if !sameHead(&next, &record.Request.Next) {
		return Head{}, timings, fmt.Errorf("computer event appender: replay projection mismatch at sequence %d: %w", record.Request.Event.Sequence, ErrProjectionMismatch)
	}
	verifyStarted := time.Now()
	if err := a.verifier.VerifyEventHeadReceipt(ctx, record.Receipt, record.Request); err != nil {
		return Head{}, timings, fmt.Errorf("computer event appender: replay receipt sequence %d: %w", record.Request.Event.Sequence, err)
	}
	prepareStarted := time.Now()
	timings.reduce = verifyStarted.Sub(applyStarted)
	timings.verify = prepareStarted.Sub(verifyStarted)
	if err := a.projection.Prepare(ctx, record.Request); err != nil {
		return Head{}, timings, fmt.Errorf("computer event appender: replay prepare sequence %d: %w", record.Request.Event.Sequence, err)
	}
	finalizeStarted := time.Now()
	timings.prepare = finalizeStarted.Sub(prepareStarted)
	if err := a.finalizeProjection(ctx, record.Request.Event, record.Request.EventDigest, record.Receipt); err != nil && !errors.Is(err, ErrProjectionBatchUnreadable) {
		return Head{}, timings, fmt.Errorf("computer event appender: replay finalize sequence %d: %w", record.Request.Event.Sequence, err)
	}
	timings.finalize = time.Since(finalizeStarted)
	return next, timings, nil
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
	if observer := a.replayObserver; observer != nil {
		observer.CheckpointCommitted(a.committedReplaySeq())
	}
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
	if err := a.applyProjection(ctx, digest, receipt, batch); err != nil {
		return err
	}
	if batch != nil && batch.Unreadable {
		return ErrProjectionBatchUnreadable
	}
	return nil
}

func (a *ComputerEventAppender) applyProjection(ctx context.Context, digest string, receipt Receipt, batch *ProjectionBatch) error {
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
	sealed := false
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
		fetchStarted := time.Now()
		reader := a.reader
		if a.replayPrefetched != nil {
			reader = prefetchedReader{computerID: a.computerID, held: a.replayPrefetched, next: a.reader}
		}
		resolved, err := ResolvePayloads(ctx, reader, a.cipher, event.ComputerID, event.EventID, refs)
		a.replayPayloadNanos.Add(int64(time.Since(fetchStarted)))
		if err != nil {
			return nil, err
		}
		if len(resolved) != 1 {
			return nil, fmt.Errorf("computer event projection: one projection batch payload is required")
		}
		plaintext = resolved[0].Plaintext
		sealed = privacy == "private"
	}
	batch, err := DecodeProjectionBatch(plaintext)
	var syntax *json.SyntaxError
	if err != nil && sealed && errors.As(err, &syntax) {
		// The envelope's digest matched and it authenticated, so these are the
		// bytes the guest sealed, and the redaction splice broke them; the
		// unredacted batch is gone. The live append that sealed it failed its
		// finalize, so no live state ever held it: no change is the state
		// replay must reach (docs/problems/redaction-breaks-sealed-projection-batch-2026-10-31.md).
		log.Printf("computer event appender: sequence %d event %s: %v at byte %d of %d; applied as no change", event.Sequence, event.EventID, err, syntax.Offset, len(plaintext))
		return &ProjectionBatch{Version: ProjectionBatchV2, ProjectorVersion: ProjectorVersionV2, ComputerID: event.ComputerID, EventID: event.EventID, EventDigest: digest, Unreadable: true}, nil
	}
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
		replayObserver:   a.replayObserver,
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
