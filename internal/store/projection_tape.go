package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type projectionTape struct {
	computerID string
	appender   *computerevent.ComputerEventAppender
	// stats counts the tape events this binding accepts (telemetry_stats.go).
	stats *telemetryCounters
	// sealer encrypts each projection batch under the computer key before it
	// is pinned (build sheet decision 9). A tape bound without a sealer is
	// the test seam; production wiring (autoputer) always sets one.
	sealer atomic.Pointer[computerevent.PrivateArtifactCipher]
	// dryRun validates a batch against committed state before the private
	// path pins it: AppendNewPrivatePayload, unlike AppendNewPayload, does not
	// run the appender's dry run.
	dryRun func(ctx context.Context, batch *computerevent.ProjectionBatch) (headSequence uint64, err error)
	// afterDryRun, set only by tests, runs between the store's dry run and the
	// append (the gap another writer can use to move the local head).
	afterDryRun func()
	// group is the store's group-commit state (tape_group_commit.go) and
	// telemetryQueue its telemetry sealer, nil until batching is configured.
	group          *groupCommit
	telemetryQueue func() *telemetrySealer
}

// BindProjectionTape routes live desktop and OG mutations through append+project.
// Unbound stores keep direct SQL for tests that have no computer event chain.
// Production autoputer must bind. Residue import is not this method.
func (s *Store) BindProjectionTape(computerID string, appender *computerevent.ComputerEventAppender) error {
	if s == nil || appender == nil {
		return fmt.Errorf("store: projection tape requires store and appender")
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return fmt.Errorf("store: projection tape requires computer_id")
	}
	tape := &projectionTape{computerID: computerID, appender: appender, stats: &s.telemetryStats, group: &s.tapeGroup}
	tape.telemetryQueue = func() *telemetrySealer { return s.telemetry.Load() }
	tape.dryRun = func(ctx context.Context, batch *computerevent.ProjectionBatch) (uint64, error) {
		return s.DryRunProjectionBatchAt(ctx, computerID, batch)
	}
	if prior := s.projectionTape; prior != nil {
		tape.sealer.Store(prior.sealer.Load()) // a rebind keeps the sealer
	}
	s.projectionTape = tape
	revalidations := appender.Revalidations
	s.revalidationSource.Store(&revalidations)
	s.startTapeWriteSummary()
	if sl := s.telemetry.Load(); sl != nil {
		appender.SetAppendBarrier(sl.appendBarrier) // a rebind keeps the barrier
	}
	if s.ogStore != nil {
		s.ogStore.SetMutationInterceptor(tape.interceptOG)
	}
	return nil
}

// SetProjectionSealer makes the bound projection tape seal every projection
// batch payload under the computer key (data system build sheet decision 9:
// projection_batch_recorded payloads take the private path, so the host holds
// ciphertext). The replay path reads them back through the payload resolver
// with the same cipher (ComputerEventAppender.SetPayloadResolver). Production
// must call it with the guest cipher before the first append.
func (s *Store) SetProjectionSealer(cipher *computerevent.PrivateArtifactCipher) error {
	if s == nil || s.projectionTape == nil {
		return fmt.Errorf("store: projection tape is not bound")
	}
	if cipher == nil {
		return fmt.Errorf("store: projection sealer is required (refusing to pin projection batches in plaintext)")
	}
	s.projectionTape.sealer.Store(cipher)
	return nil
}

// ProjectionTapeBound reports whether production mutation routing is active.
// It lets dependent stores retain their direct-SQL test seam when no event
// authority is configured.
func (s *Store) ProjectionTapeBound() bool {
	return s != nil && s.projectionTape != nil
}

// AppendProjectionOps is the only production escape hatch for non-OG
// projection writers. It is intentionally unavailable until the event tape is
// bound; unbound stores remain a test-only direct-SQL seam.
func (s *Store) AppendProjectionOps(ctx context.Context, ops []computerevent.ProjectionOp) error {
	if s == nil || s.projectionTape == nil {
		return fmt.Errorf("store: projection tape is not bound")
	}
	return s.projectionTape.appendOps(ctx, ops)
}

func (t *projectionTape) interceptOG(ctx context.Context, objects []objectgraph.Object, edges []objectgraph.Edge) error {
	if t == nil {
		return fmt.Errorf("store: projection tape is not bound")
	}
	ops := make([]computerevent.ProjectionOp, 0, len(objects)+len(edges))
	for _, obj := range objects {
		body, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("store: marshal projected object: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{
			Kind:        computerevent.ProjectionOpObject,
			CanonicalID: obj.CanonicalID,
			Body:        body,
		})
	}
	for _, edge := range edges {
		body, err := json.Marshal(edge)
		if err != nil {
			return fmt.Errorf("store: marshal projected edge: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{
			Kind:        computerevent.ProjectionOpObjectEdge,
			CanonicalID: edge.EdgeID,
			Body:        body,
		})
	}
	return t.appendOps(ctx, ops)
}

func (t *projectionTape) appendDesktop(ctx context.Context, state projectedDesktopState) error {
	body, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("store: marshal projected desktop: %w", err)
	}
	return t.appendOps(ctx, []computerevent.ProjectionOp{{
		Kind: computerevent.ProjectionOpDesktopState,
		Body: body,
	}})
}

// projectionAuthority is the authority_ref of every ordinary projection batch;
// telemetryProjectionAuthority marks a batch the telemetry sealer wrote, so
// the tape itself says which batches are telemetry (design 3.4 layer 5).
const (
	projectionAuthority          = "authority:vm-local-projection"
	telemetryProjectionAuthority = "authority:vm-local-projection-telemetry"
)

func (t *projectionTape) appendOps(ctx context.Context, ops []computerevent.ProjectionOp) error {
	return t.appendOpsAs(ctx, ops, projectionAuthority)
}

// appendOpsAs writes ops as one acknowledged tape event, or, when every op is
// an unconditional upsert, as a member of a shared one (tape_group_commit.go):
// the writers queued behind the append in flight go out in one event, in
// arrival order. Either way it returns after the event is durable.
func (t *projectionTape) appendOpsAs(ctx context.Context, ops []computerevent.ProjectionOp, authority string) error {
	if t != nil && t.appender != nil && len(ops) > 0 {
		defer t.stats.enterTapeWrite()()
	}
	// A guarded batch (PutBatchConditional and its kin) never shares an event:
	// its conditions were read from committed state before it got here, and
	// the interceptor sees plain object ops, so only the context says it was
	// guarded. It takes the direct path, alone in its event.
	if t != nil && t.appender != nil && authority == projectionAuthority && t.group != nil && groupableOps(ops) && !objectgraph.ConditionalBatch(ctx) {
		return t.appendGrouped(ctx, ops)
	}
	return t.appendDirect(ctx, ops, authority)
}

// appendDirect writes ops as their own tape event.
func (t *projectionTape) appendDirect(ctx context.Context, ops []computerevent.ProjectionOp, authority string) error {
	return t.appendDirectParts(ctx, [][]computerevent.ProjectionOp{ops}, authority, false, false)
}

// appendDirectParts writes the parts' ops, in order, as one tape event. Each
// part is one writer's ops (a group's members); firstIsTelemetry marks the
// first part as telemetry that rode the event. The parts only matter to the
// counters: the tape sees one flat op list.
//
// validateOutside says where a sealed batch is dry-run. A group leader holds
// the leader token, so its dry run and its turn under the appender mutex are
// adjacent, and the dry run goes outside the mutex (shorter critical section);
// the appender compares the head it saw (see computerevent.batchGuard) and
// validates again only if another writer moved it. Every other append has no
// such adjacency (writers queue on the mutex after validating, so the head
// would nearly always have moved): its dry run runs once, under the mutex.
func (t *projectionTape) appendDirectParts(ctx context.Context, parts [][]computerevent.ProjectionOp, authority string, firstIsTelemetry, validateOutside bool) error {
	if t == nil || t.appender == nil {
		return fmt.Errorf("store: projection tape is not bound")
	}
	n := 0
	for _, part := range parts {
		n += len(part)
	}
	if n == 0 {
		return nil
	}
	ops := make([]computerevent.ProjectionOp, 0, n)
	for _, part := range parts {
		ops = append(ops, part...)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(computerevent.ProjectionBatch{
		Version:          computerevent.ProjectionBatchV2,
		ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID:       t.computerID,
		EventID:          eventID,
		Ops:              ops,
	})
	if err != nil {
		return fmt.Errorf("store: marshal projection batch: %w", err)
	}
	event := computerevent.Event{
		SchemaVersion:  computerevent.SchemaVersionV1,
		EventID:        eventID,
		ComputerID:     t.computerID,
		EventKind:      computerevent.EventProjectionBatchRecorded,
		OccurredAt:     time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "projection-batch:" + eventID,
		ActorProfile:   "trusted-core",
		AuthorityRef:   authority,
		PrivacyClass:   "owner",
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	if cipher := t.sealer.Load(); cipher != nil {
		// Sealed: the event names a private envelope. The appender resolves it
		// at finalize and at replay with the payload resolver's cipher, which
		// reads the event's PrivacyClass to decide to decrypt.
		event.PrivacyClass = "private"
		var batch computerevent.ProjectionBatch
		if err := json.Unmarshal(payload, &batch); err != nil {
			return fmt.Errorf("store: decode projection batch for dry run: %w", err)
		}
		if t.dryRun != nil && validateOutside {
			head, err := t.dryRun(ctx, &batch)
			if err != nil {
				t.stats.tapeBatchesRefused.Add(1)
				return fmt.Errorf("store: append projection batch: %w: %v", computerevent.ErrBatchRefused, err)
			}
			// The appender compares this head with the local head under its
			// mutex and validates again if another writer moved it.
			ctx = computerevent.WithValidatedHead(ctx, head)
			if t.afterDryRun != nil {
				t.afterDryRun()
			}
		}
		_, _, err = t.appender.AppendNewPrivatePayload(ctx, event, computerevent.TransitionInput{}, payload, computerevent.ProjectionBatchMediaType, cipher)
	} else {
		_, _, err = t.appender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, computerevent.ProjectionBatchMediaType, "owner")
	}
	if err != nil {
		if errors.Is(err, computerevent.ErrBatchRefused) {
			t.stats.tapeBatchesRefused.Add(1)
		}
		return fmt.Errorf("store: append projection batch: %w", err)
	}
	t.stats.recordTapeEvent(event.AuthorityRef, parts, firstIsTelemetry, len(payload))
	return nil
}

func desktopStateProjection(state types.DesktopState, desktopID, sessionID string, now time.Time) projectedDesktopState {
	return projectedDesktopState{
		OwnerID:            strings.TrimSpace(state.OwnerID),
		DesktopID:          desktopID,
		Windows:            state.Windows,
		ActiveWindowID:     state.ActiveWindowID,
		UpdatedAt:          now.UTC(),
		CreatedBySessionID: sessionID,
	}
}
