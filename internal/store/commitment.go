package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Commitment records (mission R2): precommitment/act records persisted as
// objectgraph objects of kind choir.commitment_record — never a third store.
// The record body is the typed CommitmentRecord; scalar scores are derived
// views over it, so the rich body is retained for later rescoring.

// AppendCommitmentRecord mints a commitment record object idempotently: the
// canonical id derives from (owner, computer, recordID) and the not-exists
// condition makes a replayed commit re-mint nothing. Returns the canonical id.
func (s *Store) AppendCommitmentRecord(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord) (string, error) {
	if rec.RecordID == "" {
		return "", fmt.Errorf("commitment record requires a record_id")
	}
	if rec.Provenance.AgentID == "" {
		return "", fmt.Errorf("commitment record %s requires a committing agent", rec.RecordID)
	}
	now := time.Now().UTC()
	metadata := map[string]any{
		"record_id":   rec.RecordID,
		"agent_id":    rec.Provenance.AgentID,
		"model_id":    rec.Provenance.ModelID,
		"discrepancy": string(rec.Discrepancy),
		"schema_id":   rec.SchemaID,
		"created_at":  now.UTC().Format(time.RFC3339Nano),
		"updated_at":  now.UTC().Format(time.RFC3339Nano),
	}
	obj, err := lifecycleObject(ogKindCommitmentRecord, ownerID, computerID, rec.RecordID, rec, metadata, now, now)
	if err != nil {
		return "", err
	}
	condition := objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: false}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return obj.CanonicalID, nil // already committed — idempotent
		}
		return "", err
	}
	return obj.CanonicalID, nil
}

// CommitmentRecordExists reports whether the deterministic commitment-record
// identity is already durable. Reducer crash recovery uses this as the
// cross-store prepare witness before it idempotently replays the tray.
func (s *Store) CommitmentRecordExists(ctx context.Context, ownerID, computerID, recordID string) (bool, error) {
	if recordID == "" {
		return false, fmt.Errorf("commitment record requires a record_id")
	}
	canonicalID, err := lifecycleCanonicalID(ogKindCommitmentRecord, ownerID, computerID, recordID)
	if err != nil {
		return false, err
	}
	if s == nil || s.ogStore == nil {
		return false, fmt.Errorf("commitment record lookup: object graph not initialized")
	}
	obj, err := s.ogStore.GetObject(ctx, canonicalID)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return !obj.Tombstone && obj.ObjectKind == ogKindCommitmentRecord, nil
}

// ListCommitmentRecords lists commitment records for one owner/computer,
// optionally scoped to a single addressee (the ledger-side target-desk
// binding). addressee=="" returns all records. Used by the Texture
// evidence-seam dual-read; ordering is by canonical record id (stable,
// content-derived), newest-first by object update time is not meaningful for
// append-only records.
func (s *Store) ListCommitmentRecords(ctx context.Context, ownerID, computerID, addressee string, limit int) ([]types.CommitmentRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	ownerID, computerID, addressee = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(addressee)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("commitment record list requires owner and computer")
	}
	matches := []objectgraph.JSONFieldMatch{}
	if addressee != "" {
		matches = append(matches, objectgraph.JSONFieldMatch{JSONPath: "$.addressee", Value: addressee})
	}
	objs, err := s.ogListByOwnerAndBody(ctx, ogKindCommitmentRecord, ownerID, matches, limit)
	if err != nil {
		return nil, err
	}
	records := make([]types.CommitmentRecord, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		var rec types.CommitmentRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, fmt.Errorf("commitment record decode %s: %w", obj.CanonicalID, err)
		}
		records = append(records, rec)
	}
	return records, nil
}
