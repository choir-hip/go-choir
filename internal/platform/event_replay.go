package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// EventsPage returns one validated, sequence-ordered durable event page.
func (s *EventArtifactService) EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error) {
	if s == nil || s.platform == nil || s.platform.store == nil || computerID == "" {
		return nil, fmt.Errorf("event replay: service and computer are required")
	}
	if err := s.platform.store.refuseTombstonedTape(ctx, computerID); err != nil {
		return nil, fmt.Errorf("event replay: %w", err)
	}
	if pageSize <= 0 || pageSize > computerevent.EventReplayMaxPageSize {
		return nil, fmt.Errorf("event replay: page size %d exceeds maximum %d", pageSize, computerevent.EventReplayMaxPageSize)
	}
	// The chain is gapless (sequence = head + 1, receipt and head written in
	// one transaction), so a page is the point lookups after+1..min(head,
	// after+pageSize) on the (computer_id, sequence) index. A range predicate
	// (sequence > ?) makes Dolt plan the primary key and read the computer's
	// whole chain per page
	// (docs/problems/replay-page-query-scans-whole-chain-2026-10-09.md).
	head, err := readComputerEventHead(ctx, s.platform.store.db, computerID, false)
	if err != nil {
		return nil, fmt.Errorf("event replay: %w", err)
	}
	if head == nil || afterSequence >= head.Sequence {
		return []computerevent.DurableEvent{}, nil
	}
	var credentialEpoch uint64
	if afterSequence > 0 {
		if err := s.platform.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM computer_event_append_receipts WHERE computer_id=? AND sequence<=? AND event_kind=?`, computerID, afterSequence, computerevent.EventKeyRevoked).Scan(&credentialEpoch); err != nil {
			return nil, fmt.Errorf("event replay: resolve credential epoch: %w", err)
		}
	}
	last := min(head.Sequence, afterSequence+uint64(pageSize))
	args := []any{computerID}
	for sequence := afterSequence + 1; sequence <= last; sequence++ {
		args = append(args, sequence)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(args)-1), ",")
	rows, err := s.platform.store.db.QueryContext(ctx, `SELECT sequence, event_digest, event_pin_receipt_digest, event_head_receipt_json FROM computer_event_append_receipts WHERE computer_id=? AND sequence IN (`+placeholders+`) ORDER BY sequence`, args...)
	if err != nil {
		return nil, fmt.Errorf("event replay: query chain: %w", err)
	}
	defer rows.Close()
	var records []computerevent.DurableEvent
	expected := afterSequence + 1
	for rows.Next() {
		var record computerevent.DurableEvent
		var sequence uint64
		var rawReceipt string
		if err := rows.Scan(&sequence, &record.Request.EventDigest, &record.Request.EventPinReceiptDigest, &rawReceipt); err != nil {
			return nil, err
		}
		if sequence != expected {
			return nil, fmt.Errorf("event replay: chain gap at sequence %d below head %d", expected, head.Sequence)
		}
		expected++
		eventJSON, _, err := s.platform.tapeReader().Read(computerID, tapestore.NSEvent, record.Request.EventDigest)
		if err != nil {
			return nil, fmt.Errorf("event replay: read event sequence %d: %w", sequence, err)
		}
		if computerevent.DigestBytes(eventJSON) != record.Request.EventDigest {
			return nil, fmt.Errorf("event replay: event digest mismatch at sequence %d", sequence)
		}
		event, err := computerevent.DecodeHistoricEvent(eventJSON)
		if err != nil {
			return nil, fmt.Errorf("event replay: decode event sequence %d: %w", sequence, err)
		}
		record.Request.Event = event
		if record.Request.Event.Sequence != sequence || record.Request.Event.ComputerID != computerID {
			return nil, fmt.Errorf("event replay: event index mismatch at sequence %d", sequence)
		}
		if err := json.Unmarshal([]byte(rawReceipt), &record.Receipt); err != nil {
			return nil, fmt.Errorf("event replay: decode receipt sequence %d: %w", sequence, err)
		}
		fields := record.Receipt.KindFields
		str := func(name string) string { v, _ := fields[name].(string); return v }
		record.Request.Next.DesiredEventHead = str("desired_event_head")
		record.Request.Next.EffectiveEventHead = str("effective_event_head")
		record.Request.Next.PendingTransitionRef = str("pending_transition_ref")
		record.Request.Next.DesiredStateCommitment = str("desired_state_commitment")
		record.Request.Next.EffectiveStateCommitment = str("effective_state_commitment")
		pins, ok := receiptPinDigests(fields)
		if !ok || len(pins) == 0 || pins[0] != record.Request.EventPinReceiptDigest {
			return nil, fmt.Errorf("event replay: receipt pin digests mismatch at sequence %d", sequence)
		}
		if len(pins) > 1 {
			record.Request.PayloadPinReceiptDigests = pins[1:]
		}
		record.Request.EventArtifactDigest = record.Request.EventDigest
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
	if expected <= last {
		return nil, fmt.Errorf("event replay: chain gap at sequence %d below head %d", expected, head.Sequence)
	}
	return records, nil
}

func replayTransitionInput(kind computerevent.EventKind, next computerevent.Head) computerevent.TransitionInput {
	switch kind {
	case computerevent.EventGenesisImported, computerevent.EventEffectAccepted, computerevent.EventRollbackRequested, computerevent.EventResearchUpdate:
		return computerevent.TransitionInput{TargetStateCommitment: next.DesiredStateCommitment}
	case computerevent.EventMaterializationFailed:
		return computerevent.TransitionInput{RestoredPriorEffective: true}
	default:
		return computerevent.TransitionInput{}
	}
}

// receiptPinDigests reads the pin_receipt_digests field of an event head
// receipt: the event's own pin digest first, then its payload pins.
func receiptPinDigests(fields map[string]any) ([]string, bool) {
	switch raw := fields["pin_receipt_digests"].(type) {
	case []string:
		return append([]string(nil), raw...), true
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			v, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, v)
		}
		return out, true
	}
	return nil, false
}
