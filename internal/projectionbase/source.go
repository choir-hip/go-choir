package projectionbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// DiskEventSource reads durable event envelopes and payload batch bodies directly
// from a platform-artifacts directory on disk for offline replay.
type DiskEventSource struct {
	artifactsRoot string
	computerID    string
}

// NewDiskEventSource returns an event and payload source reading from artifactsRoot.
func NewDiskEventSource(artifactsRoot, computerID string) *DiskEventSource {
	return &DiskEventSource{
		artifactsRoot: filepath.Clean(artifactsRoot),
		computerID:    strings.TrimSpace(computerID),
	}
}

// EventsPage loads and parses sequence-ordered event envelopes from disk.
func (s *DiskEventSource) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	if s == nil || s.artifactsRoot == "" {
		return nil, fmt.Errorf("disk event source: artifacts root is required")
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		computerID = s.computerID
	}
	if computerID == "" {
		return nil, fmt.Errorf("disk event source: computer ID is required")
	}

	eventsDir := filepath.Join(s.artifactsRoot, "sha256", "computer-event")
	entries, err := os.ReadDir(eventsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("disk event source: read events directory: %w", err)
	}

	var allEvents []computerevent.Event
	var allDigests []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		digest := entry.Name()
		if !computerevent.IsSHA256(digest) {
			continue
		}
		path := filepath.Join(eventsDir, digest)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("disk event source: read event file %s: %w", digest, err)
		}
		if computerevent.DigestBytes(raw) != digest {
			return nil, fmt.Errorf("disk event source: digest mismatch for event %s", digest)
		}
		var event computerevent.Event
		if err := json.Unmarshal(raw, &event); err != nil {
			return nil, fmt.Errorf("disk event source: decode event %s: %w", digest, err)
		}
		if event.ComputerID != computerID {
			continue
		}
		allEvents = append(allEvents, event)
		allDigests = append(allDigests, digest)
	}

	// Sort all events strictly by sequence ascending.
	type indexedEvent struct {
		event  computerevent.Event
		digest string
	}
	indexed := make([]indexedEvent, len(allEvents))
	for i := range allEvents {
		indexed[i] = indexedEvent{event: allEvents[i], digest: allDigests[i]}
	}
	sort.Slice(indexed, func(i, j int) bool {
		return indexed[i].event.Sequence < indexed[j].event.Sequence
	})

	// Reduce chain sequentially to build valid Next heads and Inputs.
	var currentHead *computerevent.Head
	var filtered []computerevent.DurableEvent
	for _, item := range indexed {
		input := computerevent.TransitionInput{
			TargetStateCommitment: item.event.ResultingEffectiveCommitment,
		}
		nextHead, err := computerevent.Reduce(currentHead, item.event, input)
		if err != nil {
			return nil, fmt.Errorf("disk event source: reduce sequence %d: %w", item.event.Sequence, err)
		}

		if item.event.Sequence > afterSequence {
			filtered = append(filtered, computerevent.DurableEvent{
				Request: computerevent.CASRequest{
					Event:               item.event,
					EventDigest:         item.digest,
					EventArtifactDigest: item.digest,
					Input:               input,
					Next:                nextHead,
				},
				Receipt: computerevent.Receipt{
					ReceiptKind: "EventHeadReceipt",
					Issuer:      "corpusd",
					KindFields: map[string]any{
						"event_digest": item.digest,
					},
				},
			})
		}
		currentHead = &nextHead
	}

	if pageSize > 0 && len(filtered) > pageSize {
		filtered = filtered[:pageSize]
	}

	return filtered, nil
}

// Events returns all events after afterSequence.
func (s *DiskEventSource) Events(ctx context.Context, computerID string, afterSequence uint64) ([]computerevent.DurableEvent, error) {
	return s.EventsPage(ctx, computerID, afterSequence, 0)
}

// FetchPayload reads an event payload batch body from the computer-event-payload namespace.
func (s *DiskEventSource) FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error) {
	artifactDigest = strings.TrimSpace(artifactDigest)
	if !computerevent.IsSHA256(artifactDigest) {
		return nil, fmt.Errorf("disk event source: invalid payload commitment %q", artifactDigest)
	}
	for _, namespace := range []string{"computer-event-payload", "computer-event"} {
		path := filepath.Join(s.artifactsRoot, "sha256", namespace, artifactDigest)
		raw, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("disk event source: read payload %s: %w", artifactDigest, err)
		}
		if computerevent.DigestBytes(raw) != artifactDigest {
			return nil, fmt.Errorf("disk event source: payload digest mismatch for %s", artifactDigest)
		}
		return raw, nil
	}
	return nil, fmt.Errorf("disk event source: payload %s not found", artifactDigest)
}

// Head returns the latest head available on disk for computerID.
func (s *DiskEventSource) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	records, err := s.EventsPage(ctx, computerID, 0, 0)
	if err != nil || len(records) == 0 {
		return nil, err
	}
	last := records[len(records)-1]
	return &last.Request.Next, nil
}

// CompareAndSwap is a read-only stub for HeadCAS to prevent offline rebuilder from appending.
func (s *DiskEventSource) CompareAndSwap(ctx context.Context, req computerevent.CASRequest) (computerevent.Receipt, error) {
	return computerevent.Receipt{}, fmt.Errorf("disk event source: offline rebuilder cannot HeadCAS events")
}

// PinEvent is a stub for ArtifactPinner.
func (s *DiskEventSource) PinEvent(ctx context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (computerevent.PinResult, error) {
	return computerevent.PinResult{ArtifactDigest: computerevent.DigestBytes(canonicalEvent)}, nil
}

// PinEventPayload is a stub for ArtifactPinner.
func (s *DiskEventSource) PinEventPayload(ctx context.Context, computerID, eventID string, payload []byte, mediaType, privacyClass, requestCommitment string) (computerevent.PinResult, error) {
	return computerevent.PinResult{ArtifactDigest: computerevent.DigestBytes(payload)}, nil
}

// VerifyEventHeadReceipt verifies receipt for offline replay.
func (s *DiskEventSource) VerifyEventHeadReceipt(ctx context.Context, receipt computerevent.Receipt, request computerevent.CASRequest) error {
	if receipt.ReceiptKind != "EventHeadReceipt" {
		return fmt.Errorf("disk event source: receipt kind mismatch: %s", receipt.ReceiptKind)
	}
	return nil
}
