package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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
	var event computerevent.Event
	if err := json.Unmarshal(canonicalEvent, &event); err != nil {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: invalid event: %w", err)
	}
	normalized, err := event.CanonicalBytes()
	if err != nil || !bytes.Equal(normalized, canonicalEvent) {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: event is not canonical")
	}
	if event.ComputerID != computerID || event.RequestCommitment != requestCommitment {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: event scope mismatch")
	}
	return s.pin(computerID, canonicalEvent, "application/vnd.choir.computer-event+json", "computer-event", "private", "request_commitment", requestCommitment)
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
	return s.pin(computerID, payload, mediaType, "computer-event-payload", privacyClass, "pin_intent_commitment", requestCommitment)
}

func (s *EventArtifactService) pin(computerID string, payload []byte, mediaType, namespace, privacyClass, commitmentField, commitment string) (computerevent.PinResult, error) {
	if computerID == "" || commitmentField == "" || !computerevent.IsSHA256(commitment) {
		return computerevent.PinResult{}, fmt.Errorf("event artifact service: computer and request commitment are required")
	}
	digest := computerevent.DigestBytes(payload)
	storageRef := filepath.Join("sha256", namespace, digest)
	if err := s.writeImmutable(storageRef, payload); err != nil {
		return computerevent.PinResult{}, err
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
	receiptDigest := computerevent.DigestBytes(receiptJSON)
	if err := s.writeImmutable(filepath.Join("sha256", "pin-receipts", receiptDigest+".json"), receiptJSON); err != nil {
		return computerevent.PinResult{}, err
	}
	return computerevent.PinResult{ArtifactDigest: digest, Receipt: receipt}, nil
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
	for index, digest := range digests {
		if !computerevent.IsSHA256(digest) {
			return fmt.Errorf("event artifact service: invalid pin receipt digest")
		}
		path, err := s.platform.artifactPath(filepath.Join("sha256", "pin-receipts", digest+".json"))
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
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
		artifactPath, err := s.platform.artifactPath(filepath.Join("sha256", namespace, artifactDigest))
		if err != nil {
			return err
		}
		artifact, err := os.ReadFile(artifactPath)
		if err != nil || computerevent.DigestBytes(artifact) != artifactDigest {
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

func (s *Service) computerEventSigningKey() computerevent.SigningKey {
	return computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: s.signingKey.KeyID},
		PrivateKey: s.signingKey.Private,
	}
}

func (s *EventArtifactService) writeImmutable(storageRef string, payload []byte) error {
	path, err := s.platform.artifactPath(storageRef)
	if err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err == nil {
		if !bytes.Equal(existing, payload) {
			return fmt.Errorf("event artifact service: immutable digest collision")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.platform.writeBlob(storageRef, payload)
}
