package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func (s *EventArtifactService) Events(ctx context.Context, computerID string, afterSequence uint64) ([]computerevent.DurableEvent, error) {
	if s == nil || s.platform == nil || s.platform.store == nil || computerID == "" {
		return nil, fmt.Errorf("event replay: service and computer are required")
	}
	var credentialEpoch uint64
	if err := s.platform.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=? AND sequence<=? AND event_kind=?`, computerID, afterSequence, computerevent.EventKeyRevoked).Scan(&credentialEpoch); err != nil {
		return nil, fmt.Errorf("event replay: resolve credential epoch: %w", err)
	}
	rows, err := s.platform.store.db.QueryContext(ctx, `SELECT sequence, event_digest, event_artifact_ref, event_pin_receipt_digest, pin_receipt_digests_json, event_head_receipt_json, event_head_receipt_digest, desired_event_head, effective_event_head, COALESCE(pending_transition_ref, ''), desired_state_commitment, effective_state_commitment FROM computer_event_append_receipts WHERE computer_id=? AND sequence>? ORDER BY sequence`, computerID, afterSequence)
	if err != nil {
		return nil, fmt.Errorf("event replay: query chain: %w", err)
	}
	defer rows.Close()
	var records []computerevent.DurableEvent
	for rows.Next() {
		var record computerevent.DurableEvent
		var sequence uint64
		var eventArtifactRef, rawPins, rawReceipt, receiptDigest string
		if err := rows.Scan(&sequence, &record.Request.EventDigest, &eventArtifactRef, &record.Request.EventPinReceiptDigest, &rawPins, &rawReceipt, &receiptDigest, &record.Request.Next.DesiredEventHead, &record.Request.Next.EffectiveEventHead, &record.Request.Next.PendingTransitionRef, &record.Request.Next.DesiredStateCommitment, &record.Request.Next.EffectiveStateCommitment); err != nil {
			return nil, err
		}
		if eventArtifactRef != record.Request.EventDigest {
			return nil, fmt.Errorf("event replay: artifact reference mismatch at sequence %d", sequence)
		}
		path, err := s.platform.artifactPath(filepath.Join("sha256", "computer-event", eventArtifactRef))
		if err != nil {
			return nil, err
		}
		eventJSON, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("event replay: read event sequence %d: %w", sequence, err)
		}
		if computerevent.DigestBytes(eventJSON) != record.Request.EventDigest {
			return nil, fmt.Errorf("event replay: event digest mismatch at sequence %d", sequence)
		}
		if err := json.Unmarshal(eventJSON, &record.Request.Event); err != nil {
			return nil, fmt.Errorf("event replay: decode event sequence %d: %w", sequence, err)
		}
		if record.Request.Event.Sequence != sequence || record.Request.Event.ComputerID != computerID {
			return nil, fmt.Errorf("event replay: event index mismatch at sequence %d", sequence)
		}
		if err := json.Unmarshal([]byte(rawPins), &record.Request.PayloadPinReceiptDigests); err != nil {
			return nil, fmt.Errorf("event replay: decode pins sequence %d: %w", sequence, err)
		}
		if err := json.Unmarshal([]byte(rawReceipt), &record.Receipt); err != nil {
			return nil, fmt.Errorf("event replay: decode receipt sequence %d: %w", sequence, err)
		}
		canonicalReceipt, err := record.Receipt.CanonicalBytes()
		if err != nil || computerevent.DigestBytes(canonicalReceipt) != receiptDigest {
			return nil, fmt.Errorf("event replay: receipt digest mismatch at sequence %d", sequence)
		}
		record.Request.EventArtifactDigest = eventArtifactRef
		record.Request.Next.ComputerID = computerID
		record.Request.Next.Sequence = sequence
		record.Request.Next.CanonicalEventHead = record.Request.EventDigest
		record.Request.Next.ReducerVersion = record.Request.Event.ReducerVersion
		if record.Request.Event.EventKind == computerevent.EventKeyRevoked {
			credentialEpoch++
		}
		record.Request.Next.CredentialRevocationEpoch = credentialEpoch
		record.Request.Input = replayTransitionInput(record.Request.Event.EventKind, record.Request.Next)
		record.Request.PinIntentCommitment, err = computerevent.ComputePinIntentCommitment(record.Request.Event, record.Request.Input)
		if err != nil {
			return nil, fmt.Errorf("event replay: compute pin intent sequence %d: %w", sequence, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if records == nil {
		return []computerevent.DurableEvent{}, nil
	}
	return records, nil
}

func replayTransitionInput(kind computerevent.EventKind, next computerevent.Head) computerevent.TransitionInput {
	switch kind {
	case computerevent.EventGenesisImported, computerevent.EventEffectAccepted, computerevent.EventRollbackRequested, computerevent.EventResearcherUpdate:
		return computerevent.TransitionInput{TargetStateCommitment: next.DesiredStateCommitment}
	case computerevent.EventMaterializationFailed:
		return computerevent.TransitionInput{RestoredPriorEffective: true}
	default:
		return computerevent.TransitionInput{}
	}
}
