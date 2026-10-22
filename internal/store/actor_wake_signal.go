package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
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

// signalActorWakesIfAny signals when the committed batch held outbox rows,
// and announces those rows so the signalled sweep reads them by key.
func (s *Store) signalActorWakesIfAny(objects []objectgraph.Object) {
	announced := false
	for _, obj := range objects {
		if obj.ObjectKind == ogKindActorWakeOutbox {
			s.noteAnnouncedActorWake(obj.CanonicalID)
			announced = true
		}
	}
	if announced {
		s.notifyActorWakeProjector()
	}
}

// signalActorWakes signals a commit that wrote outbox rows without naming
// them: the next sweep must read the whole outbox.
func (s *Store) signalActorWakes() {
	s.announcedWakes.mu.Lock()
	s.announcedWakes.full = true
	s.announcedWakes.mu.Unlock()
	s.notifyActorWakeProjector()
}

func (s *Store) notifyActorWakeProjector() {
	select {
	case s.actorWakeSignalChan() <- struct{}{}:
	default:
	}
}

// announcedWakeLimit bounds the announced set; past it the next sweep reads
// the whole outbox instead.
const announcedWakeLimit = 4096

// announcedActorWakes are the outbox rows committed since the projector last
// took them (docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6).
// Process-local: after a restart the projector's first sweep reads the whole
// outbox, and its audit sweep still does every minute.
type announcedActorWakes struct {
	mu   sync.Mutex
	ids  map[string]struct{}
	full bool
}

func (s *Store) noteAnnouncedActorWake(canonicalID string) {
	canonicalID = strings.TrimSpace(canonicalID)
	if canonicalID == "" {
		return
	}
	s.announcedWakes.mu.Lock()
	defer s.announcedWakes.mu.Unlock()
	if s.announcedWakes.full {
		return
	}
	if len(s.announcedWakes.ids) >= announcedWakeLimit {
		s.announcedWakes.ids, s.announcedWakes.full = nil, true
		return
	}
	if s.announcedWakes.ids == nil {
		s.announcedWakes.ids = map[string]struct{}{}
	}
	s.announcedWakes.ids[canonicalID] = struct{}{}
}

// TakeAnnouncedActorWakes returns the unprojected wakes among the outbox rows
// committed since the last take, read by key, in canonical id order. full is
// true when a commit signalled without naming its rows, or the set overflowed:
// the caller must then read the whole outbox with ListUnprojectedActorWakes.
// Listing the outbox matches $.projected in the metadata of every outbox row
// ever written, 2 to 4 s on the paper before each wake.
func (s *Store) TakeAnnouncedActorWakes(ctx context.Context) ([]ActorWakeOutbox, bool, error) {
	s.announcedWakes.mu.Lock()
	ids, full := s.announcedWakes.ids, s.announcedWakes.full
	s.announcedWakes.ids, s.announcedWakes.full = nil, false
	s.announcedWakes.mu.Unlock()
	if full {
		return nil, true, nil
	}
	if s.ogStore == nil {
		return nil, false, fmt.Errorf("announced actor wakes: object graph not initialized")
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	wakes := make([]ActorWakeOutbox, 0, len(sorted))
	for _, id := range sorted {
		obj, err := s.ogStore.GetObject(ctx, id)
		if errors.Is(err, objectgraph.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, false, fmt.Errorf("announced actor wake %s: %w", id, err)
		}
		if obj.Tombstone || obj.ObjectKind != ogKindActorWakeOutbox {
			continue
		}
		var meta struct {
			Projected bool `json:"projected"`
		}
		if err := json.Unmarshal(obj.Metadata, &meta); err != nil || meta.Projected {
			continue
		}
		wake, err := decodeLifecycleObject[ActorWakeOutbox](obj)
		if err != nil {
			return nil, false, fmt.Errorf("decode actor wake outbox %s: %w", obj.CanonicalID, err)
		}
		wake.CanonicalID = obj.CanonicalID
		wake.MintedAt = obj.UpdatedAt.UTC()
		wakes = append(wakes, wake)
	}
	return wakes, false, nil
}

// ExhaustedActorWake is an outbox wake whose dispatch failed past its budget:
// a visible dead letter for the "what is owed" surface.
type ExhaustedActorWake struct {
	ActorWakeOutbox
	DispatchAttempts  int       `json:"dispatch_attempts"`
	LastDispatchError string    `json:"last_dispatch_error"`
	ExhaustedAt       time.Time `json:"exhausted_at"`
}

const (
	actorWakeFateDispatchExhausted    = "dispatch_exhausted"
	actorWakeFateInterruptedByRestart = "interrupted_by_restart"
)

// MarkActorWakeInterrupted takes a pre-boot work wake out of the drain after
// a crash boot, with the durable fate interrupted_by_restart: a crash restart
// never resumes work.
func (s *Store) MarkActorWakeInterrupted(ctx context.Context, canonicalID string) error {
	return s.markActorWakeFate(ctx, canonicalID, actorWakeFateInterruptedByRestart, func(metadata map[string]any) {
		metadata["interrupted_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	})
}

// MarkActorWakeExhausted takes a wake out of the drain with a durable fate.
func (s *Store) MarkActorWakeExhausted(ctx context.Context, canonicalID string, attempts int, lastErr string) error {
	if len(lastErr) > 500 {
		lastErr = lastErr[:500]
	}
	return s.markActorWakeFate(ctx, canonicalID, actorWakeFateDispatchExhausted, func(metadata map[string]any) {
		metadata["dispatch_attempts"] = attempts
		metadata["last_dispatch_error"] = lastErr
		metadata["exhausted_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	})
}

func (s *Store) markActorWakeFate(ctx context.Context, canonicalID, fate string, annotate func(map[string]any)) error {
	if s.ogStore == nil {
		return fmt.Errorf("mark actor wake %s: object graph not initialized", fate)
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
	metadata["projected"] = true
	metadata["fate"] = fate
	annotate(metadata)
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
