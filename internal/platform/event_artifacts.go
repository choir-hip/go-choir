package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

type EventArtifactService struct {
	platform *Service
	keys     computerevent.KeyResolver
	now      func() time.Time
}

func NewEventArtifactService(platform *Service, keys computerevent.KeyResolver) (*EventArtifactService, error) {
	if platform == nil || platform.store == nil || platform.signingKey == nil || keys == nil {
		return nil, fmt.Errorf("event artifact service: platform signer and key resolver are required")
	}
	return &EventArtifactService{platform: platform, keys: keys, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (s *EventArtifactService) PinEvent(_ context.Context, computerID string, canonicalEvent []byte, requestCommitment string) (computerevent.PinResult, error) {
	event, err := computerevent.DecodeHistoricEvent(canonicalEvent)
	if err != nil {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: invalid event: %w", err)
	}
	normalized, err := event.CanonicalBytes()
	if err != nil || !bytes.Equal(normalized, canonicalEvent) {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: event is not canonical")
	}
	if event.ComputerID != computerID || event.RequestCommitment != requestCommitment {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: event scope mismatch")
	}
	// Only a projection_batch_recorded event is segment-eligible (section 4.2).
	segmentable := event.EventKind == computerevent.EventProjectionBatchRecorded
	return s.pin(computerID, canonicalEvent, "application/vnd.choir.computer-event+json", "computer-event", "private", "request_commitment", requestCommitment, segmentable)
}

func (s *EventArtifactService) pinPayload(_ context.Context, computerID string, payload []byte, mediaType, privacyClass, requestCommitment string) (computerevent.PinResult, error) {
	if mediaType == "" || privacyClass == "" {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: media type and privacy class are required")
	}
	if privacyClass == "private" {
		if mediaType != computerevent.PrivateArtifactMediaType {
			return computerevent.PinResult{}, fmt.Errorf("event artifact service: private payload must use the encrypted envelope media type")
		}
		metadata, err := computerevent.InspectPrivateArtifactEnvelope(payload)
		if err != nil || metadata.ComputerID != computerID {
			return computerevent.PinResult{}, fmt.Errorf("event artifact service: invalid private artifact envelope")
		}
	}
	// Only a projection batch payload is segment-eligible (section 4.2).
	segmentable := mediaType == computerevent.ProjectionBatchMediaType
	return s.pin(computerID, payload, mediaType, "computer-event-payload", privacyClass, "pin_intent_commitment", requestCommitment, segmentable)
}

// pin writes the artifact and its signed pin receipt. segmentable says the
// caller's kind is eligible for the segment store; the artifact and its
// receipt go to the computer's segment only when it is eligible, the flag is
// PLATFORM_TAPE_SEGMENTS=write and the computer id can name a segment
// directory. Otherwise the legacy files are written exactly as before the
// flag existed. A segment append that fails (a failed write or fsync poisons
// the segment writer for the life of the process, internal/tapestore) falls
// back to the legacy files for this pin, so the event is still stored durably;
// a refusal by the pin's own checks (a digest collision, an unreadable
// segment) is an error, never a fallback.
func (s *EventArtifactService) pin(computerID string, payload []byte, mediaType, namespace, privacyClass, commitmentField, commitment string, segmentable bool) (computerevent.PinResult, error) {
	if computerID == "" || commitmentField == "" || !computerevent.IsSHA256(commitment) {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: computer and request commitment are required")
	}
	segmented := segmentable && s.platform.tapeSegmentable(computerID)
	digest := computerevent.DigestBytes(payload)
	storageRef := filepath.Join("sha256", namespace, digest)
	if !segmented {
		if err := s.writeImmutable(storageRef, payload); err != nil {
			return computerevent.PinResult{}, err
		}
	}
	fields := map[string]any{
		"computer_id": computerID, "artifact_digest": digest,
		"media_type": mediaType, "length": len(payload),
		"privacy_class": privacyClass, "pin_namespace": namespace,
		commitmentField: commitment,
	}
	receipt, err := computerevent.NewSignedReceipt("PinReceipt", "corpusd", fields, []computerevent.SigningKey{s.platform.computerEventSigningKey()}, s.now())
	if err != nil {
		return computerevent.PinResult{}, err
	}
	receiptJSON, err := receipt.CanonicalBytes()
	if err != nil {
		return computerevent.PinResult{}, err
	}
	if segmented {
		ns := tapestore.NSPayload
		if namespace == "computer-event" {
			ns = tapestore.NSEvent
		}
		appendErr, err := s.pinSegmented(computerID, ns, payload, receiptJSON)
		if err != nil {
			return computerevent.PinResult{}, err
		}
		if appendErr == nil {
			s.platform.tapeCount.segmentPins.Add(1)
			return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, nil
		}
		log.Printf("corpusd: tape segment append failed for computer %s, storing the pin in the legacy files: %v", computerID, appendErr)
		s.platform.tapeCount.segmentFallbacks.Add(1)
		if err := s.writeImmutable(storageRef, payload); err != nil {
			return computerevent.PinResult{}, err
		}
	}
	receiptDigest := computerevent.DigestBytes(receiptJSON)
	if err := s.writeImmutable(filepath.Join("sha256", "pin-receipts", receiptDigest+".json"), receiptJSON); err != nil {
		return computerevent.PinResult{}, err
	}
	s.platform.tapeCount.legacyPins.Add(1)
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, nil
}

// pinSegmented appends a pin's artifact and receipt to the computer's
// segment. It returns (appendErr, nil) when the segment writer failed, so the
// caller can fall back to legacy, and (nil, err) for an error of the pin's
// own checks, which the caller must return. The reuse lock is held from the
// collision checks to the append, as writeImmutable holds it for legacy files,
// and released before any fallback takes it again.
func (s *EventArtifactService) pinSegmented(computerID string, ns tapestore.NS, payload, receiptJSON []byte) (appendErr, err error) {
	s.platform.store.casReuseMu.RLock()
	defer s.platform.store.casReuseMu.RUnlock()
	// The reuse checks run inside the segment writer's per-computer lock
	// (AppendFunc), never beside it: what they read of the open segment is
	// then stable, so an artifact is omitted only if a finished call left
	// it there. One fsync covers the artifact and its receipt, and
	// returns before the guest can send the head CAS that names them.
	var checkErr error
	aerr := s.platform.tapeAppender().AppendFunc(computerID, func() ([]tapestore.Item, error) {
		items := make([]tapestore.Item, 0, 2)
		artifactItem, err := s.stageSegmented(computerID, ns, payload)
		if err != nil {
			checkErr = err
			return nil, err
		}
		if artifactItem != nil {
			items = append(items, *artifactItem)
		}
		receiptItem, err := s.stageSegmented(computerID, tapestore.NSReceipt, receiptJSON)
		if err != nil {
			checkErr = err
			return nil, err
		}
		if receiptItem != nil {
			items = append(items, *receiptItem)
		}
		return items, nil
	})
	if checkErr != nil {
		// A segment that cannot be read is a store failure, not a verdict on
		// this pin: it goes the legacy way, where a digest-verified read
		// finds it (round 5). Collisions stay errors.
		if errors.Is(checkErr, errSegmentReuseRead) {
			return checkErr, nil
		}
		return nil, checkErr
	}
	return aerr, nil
}

func (s *EventArtifactService) ValidateEventPins(_ context.Context, request computerevent.CASRequest) error {
	expectedPayloads := make([]string, 0, len(request.Event.InputArtifactRefs)+len(request.Event.OutputArtifactRefs))
	for _, ref := range request.Event.InputArtifactRefs {
		digest, ok := eventArtifactDigestFromRef(ref)
		if !ok {
			return fmt.Errorf("event artifact service: artifact ref %q is not content-addressed", ref)
		}
		expectedPayloads = append(expectedPayloads, digest)
	}
	for _, ref := range request.Event.OutputArtifactRefs {
		digest, ok := eventArtifactDigestFromRef(ref)
		if !ok {
			return fmt.Errorf("event artifact service: artifact ref %q is not content-addressed", ref)
		}
		expectedPayloads = append(expectedPayloads, digest)
	}
	if len(expectedPayloads) != len(request.PayloadPinReceiptDigests) {
		return fmt.Errorf("event artifact service: payload pin count mismatch")
	}
	digests := append([]string{request.EventPinReceiptDigest}, request.PayloadPinReceiptDigests...)
	tape := s.platform.tapeReader()
	for index, digest := range digests {
		if !computerevent.IsSHA256(digest) {
			return fmt.Errorf("event artifact service: invalid pin receipt digest")
		}
		// Segment first, legacy second; the library checks the digest of the
		// bytes it returns. A corrupt segment is an error, never a fallback.
		raw, _, err := tape.Read(request.Event.ComputerID, tapestore.NSReceipt, digest)
		if err != nil {
			return fmt.Errorf("event artifact service: read pin receipt: %w", err)
		}
		if computerevent.DigestBytes(raw) != digest {
			return fmt.Errorf("event artifact service: pin receipt digest mismatch")
		}
		var receipt computerevent.Receipt
		if err := json.Unmarshal(raw, &receipt); err != nil {
			return err
		}
		commitmentField := "pin_intent_commitment"
		expectedCommitment := request.PinIntentCommitment
		if index == 0 {
			commitmentField = "request_commitment"
			expectedCommitment = request.Event.RequestCommitment
		}
		if err := receipt.RequireKindFields("computer_id", "artifact_digest", "media_type", "length", "privacy_class", "pin_namespace", commitmentField); err != nil {
			return err
		}
		if receipt.ReceiptKind != "PinReceipt" || receipt.Issuer != "corpusd" || receipt.KindFields["computer_id"] != request.Event.ComputerID || receipt.KindFields[commitmentField] != expectedCommitment {
			return fmt.Errorf("event artifact service: pin scope mismatch")
		}
		if err := receipt.Verify(s.keys); err != nil {
			return err
		}
		artifactDigest, _ := receipt.KindFields["artifact_digest"].(string)
		namespace, _ := receipt.KindFields["pin_namespace"].(string)
		if index == 0 {
			if artifactDigest != request.EventDigest || namespace != "computer-event" || receipt.KindFields["media_type"] != "application/vnd.choir.computer-event+json" || receipt.KindFields["privacy_class"] != "private" {
				return fmt.Errorf("event artifact service: event pin mismatch")
			}
		} else {
			mediaType, _ := receipt.KindFields["media_type"].(string)
			if artifactDigest != expectedPayloads[index-1] || namespace != "computer-event-payload" || mediaType == "" || receipt.KindFields["privacy_class"] != request.Event.PrivacyClass {
				return fmt.Errorf("event artifact service: payload pin mismatch")
			}
		}
		artifactNS := tapestore.NSPayload // the checks above leave only the two pin namespaces
		if index == 0 {
			artifactNS = tapestore.NSEvent
		}
		artifact, _, err := tape.Read(request.Event.ComputerID, artifactNS, artifactDigest)
		if err != nil {
			if errors.Is(err, tapestore.ErrCorrupt) {
				return fmt.Errorf("event artifact service: pinned artifact unavailable: %w", err)
			}
			return fmt.Errorf("event artifact service: pinned artifact unavailable")
		}
		if computerevent.DigestBytes(artifact) != artifactDigest {
			return fmt.Errorf("event artifact service: pinned artifact unavailable")
		}
		if index > 0 && request.Event.PrivacyClass == "private" {
			metadata, err := computerevent.InspectPrivateArtifactEnvelope(artifact)
			if err != nil || metadata.ComputerID != request.Event.ComputerID || metadata.EventID != request.Event.EventID {
				return fmt.Errorf("event artifact service: private envelope event scope mismatch")
			}
		}
		if fmt.Sprint(receipt.KindFields["length"]) != strconv.Itoa(len(artifact)) {
			return fmt.Errorf("event artifact service: pin length mismatch")
		}
	}
	return nil
}

func eventArtifactDigestFromRef(raw string) (string, bool) {
	ref, err := computerevent.ParseArtifactRef(raw)
	if err != nil {
		return "", false
	}
	return ref.Digest().String(), true
}

// tapeReader is the one tape read path: segment first, legacy second, every
// returned byte digest-checked (internal/tapestore, forward-fix doc section
// 9). It follows the artifacts root, so a service whose root is set after
// construction still reads the right tree.
func (s *Service) tapeReader() *tapestore.Reader {
	s.tapeReaderMu.Lock()
	defer s.tapeReaderMu.Unlock()
	root := filepath.Clean(s.artifactsRoot)
	if s.tapeReaderVal == nil || s.tapeReaderVal.Root() != root {
		s.tapeReaderVal = tapestore.NewReader(root)
	}
	return s.tapeReaderVal
}

func (s *Service) computerEventSigningKey() computerevent.SigningKey {
	return computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: s.signingKey.KeyID},
		PrivateKey: s.signingKey.Private,
	}
}

func (s *EventArtifactService) FetchPayload(computerID, artifactDigest string) ([]byte, error) {
	if s == nil || s.platform == nil {
		return nil, fmt.Errorf("event artifact service: payload fetch unavailable")
	}
	computerID = strings.TrimSpace(computerID)
	artifactDigest = strings.TrimSpace(artifactDigest)
	if computerID == "" || !computerevent.IsSHA256(artifactDigest) {
		return nil, fmt.Errorf("event artifact service: computer and payload digest are required")
	}
	if err := s.platform.store.refuseTombstonedTape(context.Background(), computerID); err != nil {
		return nil, fmt.Errorf("event artifact service: %w", err)
	}
	tape := s.platform.tapeReader()
	// A segment error in one namespace does not end the probe: the digest
	// may be a verified object in the next one. With no match anywhere, the
	// first such error is returned rather than "not found".
	var firstErr error
	for _, namespace := range []string{"computer-event-payload", "computer-event"} {
		var raw []byte
		var err error
		switch namespace {
		case "computer-event-payload":
			raw, _, err = tape.Read(computerID, tapestore.NSPayload, artifactDigest)
		case "computer-event":
			raw, _, err = tape.Read(computerID, tapestore.NSEvent, artifactDigest)
		}
		if err != nil {
			if errors.Is(err, tapestore.ErrNotFound) {
				continue
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if computerevent.DigestBytes(raw) != artifactDigest {
			return nil, fmt.Errorf("event artifact service: payload digest mismatch")
		}
		return raw, nil
	}
	if firstErr != nil {
		return nil, fmt.Errorf("event artifact service: read payload: %w", firstErr)
	}
	return nil, fmt.Errorf("event artifact service: payload not found")
}

func (s *EventArtifactService) writeImmutable(storageRef string, payload []byte) error {
	path, err := s.platform.artifactPath(storageRef)
	if err != nil {
		return err
	}
	// Reuse wins against the tape sweep: the sweep re-checks and unlinks an
	// old unreferenced tape file only under the exclusive CAS reuse lock, and
	// a pin that finds the file already present refreshes its mtime, so a
	// re-pinned orphan is inside grace before any CAS can name it
	// (docs/problems/tape-sweeps-destroyed-and-orphans-2026-10-20.md).
	s.platform.store.casReuseMu.RLock()
	defer s.platform.store.casReuseMu.RUnlock()
	existing, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(existing, payload) {
			return fmt.Errorf("event artifact service: immutable digest collision")
		}
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			return fmt.Errorf("event artifact service: refresh reused artifact: %w", err)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.platform.writeBlob(storageRef, payload); err != nil {
		return err
	}
	s.platform.tapeCount.legacyFiles.Add(1)
	return nil
}
