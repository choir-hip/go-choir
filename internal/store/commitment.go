package store

import (
	"context"
	"errors"
	"fmt"
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
