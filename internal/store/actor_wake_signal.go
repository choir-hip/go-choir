package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// SL slice 2: the projector waits on this signal instead of polling. The
// store signals only after a batch that wrote outbox rows has committed, so
// a sweep can never run ahead of the write it was told about.
// ActorWakeSignal returns the channel that receives a value after committed
// actor wake writes. It holds at most one pending signal; one sweep drains
// every committed wake.
func (s *Store) ActorWakeSignal() <-chan struct{} {
	return s.actorWakeSignalChan()
}

func (s *Store) actorWakeSignalChan() chan struct{} {
	s.actorWakeSignalOnce.Do(func() { s.actorWakeSignal = make(chan struct{}, 1) })
	return s.actorWakeSignal
}

// signalActorWakesIfAny signals when the committed batch held outbox rows.
func (s *Store) signalActorWakesIfAny(objects []objectgraph.Object) {
	for _, obj := range objects {
		if obj.ObjectKind == ogKindActorWakeOutbox {
			s.signalActorWakes()
			return
		}
	}
}

func (s *Store) signalActorWakes() {
	select {
	case s.actorWakeSignalChan() <- struct{}{}:
	default:
	}
}

// ExhaustedActorWake is an outbox wake whose dispatch failed past its budget:
// a visible dead letter for the "what is owed" surface.
type ExhaustedActorWake struct {
	ActorWakeOutbox
	DispatchAttempts  int       `json:"dispatch_attempts"`
	LastDispatchError string    `json:"last_dispatch_error"`
	ExhaustedAt       time.Time `json:"exhausted_at"`
}

const actorWakeFateDispatchExhausted = "dispatch_exhausted"

// MarkActorWakeExhausted takes a wake out of the drain with a durable fate.
func (s *Store) MarkActorWakeExhausted(ctx context.Context, canonicalID string, attempts int, lastErr string) error {
	if s.ogStore == nil {
		return fmt.Errorf("mark actor wake exhausted: object graph not initialized")
	}
	obj, err := s.ogStore.GetObject(ctx, strings.TrimSpace(canonicalID))
	if err != nil {
		return err
	}
	if obj.ObjectKind != ogKindActorWakeOutbox {
		return fmt.Errorf("mark actor wake exhausted: object %s is not an actor wake outbox", canonicalID)
	}
	metadata := map[string]any{}
	if err := json.Unmarshal(obj.Metadata, &metadata); err != nil {
		return fmt.Errorf("mark actor wake exhausted: decode metadata: %w", err)
	}
	if projected, _ := metadata["projected"].(bool); projected {
		return nil
	}
	if len(lastErr) > 500 {
		lastErr = lastErr[:500]
	}
	metadata["projected"] = true
	metadata["fate"] = actorWakeFateDispatchExhausted
	metadata["dispatch_attempts"] = attempts
	metadata["last_dispatch_error"] = lastErr
	metadata["exhausted_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	rawMetadata, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return fmt.Errorf("mark actor wake exhausted: normalize metadata: %w", err)
	}
	updated := obj
	updated.Metadata = rawMetadata
	updated.UpdatedAt = time.Now().UTC()
	updated.ContentHash = objectgraph.ContentHash(updated.ObjectKind, updated.Body, updated.Metadata)
	condition := objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: true, ExpectedContentHash: obj.ContentHash}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: []objectgraph.Object{updated}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return ErrConcurrentStateChange
		}
		return err
	}
	return nil
}

// ListExhaustedActorWakes lists wakes that left the drain as dead letters.
func (s *Store) ListExhaustedActorWakes(ctx context.Context) ([]ExhaustedActorWake, error) {
	objects, err := s.ogListAllByMetadata(ctx, ogKindActorWakeOutbox, "fate", actorWakeFateDispatchExhausted)
	if err != nil {
		return nil, fmt.Errorf("list exhausted actor wakes: %w", err)
	}
	out := make([]ExhaustedActorWake, 0, len(objects))
	for _, obj := range objects {
		wake, err := decodeLifecycleObject[ActorWakeOutbox](obj)
		if err != nil {
			return nil, fmt.Errorf("decode actor wake outbox %s: %w", obj.CanonicalID, err)
		}
		wake.CanonicalID = obj.CanonicalID
		var meta struct {
			DispatchAttempts  int    `json:"dispatch_attempts"`
			LastDispatchError string `json:"last_dispatch_error"`
			ExhaustedAt       string `json:"exhausted_at"`
		}
		_ = json.Unmarshal(obj.Metadata, &meta)
		at, _ := time.Parse(time.RFC3339Nano, meta.ExhaustedAt)
		out = append(out, ExhaustedActorWake{ActorWakeOutbox: wake, DispatchAttempts: meta.DispatchAttempts, LastDispatchError: meta.LastDispatchError, ExhaustedAt: at})
	}
	return out, nil
}
