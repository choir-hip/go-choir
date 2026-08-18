package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type projectionTape struct {
	computerID string
	appender   *computerevent.ComputerEventAppender
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
	tape := &projectionTape{computerID: computerID, appender: appender}
	s.projectionTape = tape
	if s.ogStore != nil {
		s.ogStore.SetMutationInterceptor(tape.interceptOG)
	}
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

func (t *projectionTape) appendOps(ctx context.Context, ops []computerevent.ProjectionOp) error {
	if t == nil || t.appender == nil {
		return fmt.Errorf("store: projection tape is not bound")
	}
	if len(ops) == 0 {
		return nil
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
		AuthorityRef:   "authority:vm-local-projection",
		PrivacyClass:   "owner",
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	_, _, err = t.appender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, computerevent.ProjectionBatchMediaType, "owner")
	if err != nil {
		return fmt.Errorf("store: append projection batch: %w", err)
	}
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
