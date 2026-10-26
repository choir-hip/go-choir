package projectionbase

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// DiskEventSource reads durable event envelopes and payload batch bodies directly
// from a platform-artifacts directory on disk for offline replay.
type DiskEventSource struct {
	artifactsRoot string
	computerID    string
	targetHead    string
	// tape reads events and payloads segment first, legacy second.
	tape *tapestore.Reader
}

// NewDiskEventSource returns an event and payload source reading from artifactsRoot.
// When targetHead is provided, the source traces backwards along the exact
// previous_head ancestry to genesis, avoiding scanning uncommitted or foreign
// event files on disk.
func NewDiskEventSource(artifactsRoot, computerID string, targetHead ...string) *DiskEventSource {
	s := &DiskEventSource{
		artifactsRoot: filepath.Clean(artifactsRoot),
		computerID:    strings.TrimSpace(computerID),
	}
	s.tape = tapestore.NewReader(s.artifactsRoot)
	if len(targetHead) > 0 {
		s.targetHead = strings.TrimSpace(targetHead[0])
	}
	return s
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
	var allEvents []computerevent.Event
	var allDigests []string
	if s.targetHead != "" && computerevent.IsSHA256(s.targetHead) {
		curr := s.targetHead
		for curr != computerevent.ZeroHead && computerevent.IsSHA256(curr) {
			raw, _, err := s.tape.Read(computerID, tapestore.NSEvent, curr)
			if err != nil {
				return nil, fmt.Errorf("disk event source: read event file %s: %w", curr, err)
			}
			if computerevent.DigestBytes(raw) != curr {
				return nil, fmt.Errorf("disk event source: digest mismatch for event %s", curr)
			}
			event, err := computerevent.DecodeHistoricEvent(raw)
			if err != nil {
				return nil, fmt.Errorf("disk event source: decode event %s: %w", curr, err)
			}
			if event.ComputerID != computerID {
				return nil, fmt.Errorf("disk event source: event %s computer %q mismatch with %q", curr, event.ComputerID, computerID)
			}
			allEvents = append(allEvents, event)
			allDigests = append(allDigests, curr)
			curr = event.PreviousHead
		}
		for i, j := 0, len(allEvents)-1; i < j; i, j = i+1, j-1 {
			allEvents[i], allEvents[j] = allEvents[j], allEvents[i]
			allDigests[i], allDigests[j] = allDigests[j], allDigests[i]
		}
	} else {
		// The enumeration lists legacy files only. Once the computer has
		// segments it would silently miss their events, so it refuses.
		hasSegments, err := s.tape.HasSegments(computerID)
		if err != nil {
			return nil, fmt.Errorf("disk event source: check segments of %s: %w", computerID, err)
		}
		if hasSegments {
			return nil, fmt.Errorf("disk event source: computer %s has tape segments; enumerating the events directory would miss segment-resident events (read from a target head instead)", computerID)
		}
		entries, err := os.ReadDir(eventsDir)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, fmt.Errorf("disk event source: read events directory: %w", err)
		}
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
			event, err := computerevent.DecodeHistoricEvent(raw)
			if err != nil {
				return nil, fmt.Errorf("disk event source: decode event %s: %w", digest, err)
			}
			if event.ComputerID != computerID {
				continue
			}
			allEvents = append(allEvents, event)
			allDigests = append(allDigests, digest)
		}
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
	if computerID = strings.TrimSpace(computerID); computerID == "" {
		computerID = s.computerID
	}
	// A segment error in one namespace does not end the probe (round 5,
	// choir-tape-segments-a2-repack-2026-10-21.md): the digest may live in
	// the next one. It is returned only when nothing matches.
	var firstErr error
	for _, namespace := range []tapestore.NS{tapestore.NSPayload, tapestore.NSEvent} {
		raw, _, err := s.tape.Read(computerID, namespace, artifactDigest)
		if err != nil {
			if !errors.Is(err, tapestore.ErrNotFound) && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if computerevent.DigestBytes(raw) != artifactDigest {
			return nil, fmt.Errorf("disk event source: payload digest mismatch for %s", artifactDigest)
		}
		return raw, nil
	}
	if firstErr != nil {
		return nil, fmt.Errorf("disk event source: read payload %s: %w", artifactDigest, firstErr)
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

// VerifyEventHeadReceipt fails closed: the artifact store carries event bytes
// and fabricated envelopes but no signed event head receipts, so offline replay
// through this source cannot verify the original receipts. Publication requires
// a source that verifies real receipts (an HTTP replay source with a pinned
// verifier, or a platform-composed CASReplaySource).
func (s *DiskEventSource) VerifyEventHeadReceipt(ctx context.Context, receipt computerevent.Receipt, request computerevent.CASRequest) error {
	return fmt.Errorf("%w: disk artifact store carries no signed event head receipts", ErrReceiptUnverifiable)
}

// CheckpointVerdictSource reads the host's verdict on a published checkpoint
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, slice 2). A
// checkpoint with no verdict row (published before the host judged
// checkpoints) reports found=false.
type CheckpointVerdictSource interface {
	CheckpointVerdict(ctx context.Context, computerID, checkpointDigest string) (verdict selfdevprotocol.CheckpointVerdict, found bool, err error)
}
