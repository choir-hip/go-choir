package store

import (
	"context"
	"encoding/json"
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

// ListCommitmentClaimRecords lists the newest records that are not
// directives (note, escalate, cast, retract), filtered in the query before
// its limit. Directives carry no scored claim, and every Texture control
// mints a cast (P1, record-first controls); filtering them after the limit
// would let casts push a desk's own acts out of its pack window. A record
// with no directive body (including legacy ones) matches.
func (s *Store) ListCommitmentClaimRecords(ctx context.Context, ownerID, computerID string, limit int) ([]types.CommitmentRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("commitment claim list requires owner and computer")
	}
	// The pack is read per desk cell. With the claim index live it is a
	// reverse index walk that stops at limit rows; otherwise every record's
	// body is evaluated as JSON (11% of a busy computer's CPU,
	// guest-cpu-is-unindexed-store-scans-2026-10-14).
	//
	// Agent calibration records (claims external agents write through
	// /api/precommit) are excluded too: they belong to the agent that wrote
	// them, not to a desk. The unindexed read filters them in the query; the
	// claim index predicates on the directive alone, so the indexed read
	// over-fetches and drops them here. A burst of calibration records
	// larger than the over-fetch can still shorten a desk's window; a
	// calibration-aware index column is the repair if that is ever seen.
	var objs []objectgraph.Object
	indexed := false
	var err error
	fetch := limit * 2
	if graph := s.ogReadStore; graph != nil {
		objs, indexed, err = graph.ListCommitmentClaimObjects(ctx, ownerID, fetch)
	} else if s.ogStore != nil {
		objs, indexed, err = s.ogStore.ListCommitmentClaimObjects(ctx, ownerID, fetch)
	}
	if err == nil && indexed {
		objs = dropCalibrationRecords(objs, limit)
	}
	if err != nil {
		return nil, err
	}
	if !indexed {
		matches := []objectgraph.JSONFieldMatch{
			{JSONPath: "$.directive", Value: "", MissingMatchesEmpty: true},
			{JSONPath: "$.calibration", Value: "", MissingMatchesEmpty: true},
		}
		if objs, err = s.ogListByOwnerAndBody(ctx, ogKindCommitmentRecord, ownerID, matches, limit); err != nil {
			return nil, err
		}
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

// GetCommitmentRecord returns the stored record body for one record id,
// or nil when absent. Read-only — used by the learning-claims gate to
// check whether a verifier ref names a scored record.
func (s *Store) GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error) {
	ownerID, computerID, recordID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(recordID)
	if ownerID == "" || computerID == "" || recordID == "" {
		return nil, nil
	}
	objs, err := s.ogListByOwnerAndBody(ctx, ogKindCommitmentRecord, ownerID, []objectgraph.JSONFieldMatch{{JSONPath: "$.record_id", Value: recordID}}, 1)
	if err != nil {
		return nil, err
	}
	for _, obj := range objs {
		if obj.Tombstone || strings.TrimSpace(obj.ComputerID) != computerID {
			continue
		}
		var rec types.CommitmentRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, fmt.Errorf("commitment record decode %s: %w", obj.CanonicalID, err)
		}
		return &rec, nil
	}
	return nil, nil
}

// ListCalibrationRecords lists agent calibration records (claims and their
// settlements, calibration level 2) for one owner/computer, optionally for
// one claimant agent, newest first. It returns at most limit records; the
// caller reports truncation when the count reaches it.
func (s *Store) ListCalibrationRecords(ctx context.Context, ownerID, computerID, agent string, limit int) ([]types.CommitmentRecord, error) {
	if limit <= 0 {
		limit = 200
	}
	ownerID, computerID, agent = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agent)
	if ownerID == "" || computerID == "" {
		return nil, fmt.Errorf("calibration record list requires owner and computer")
	}
	matches := []objectgraph.JSONFieldMatch{{JSONPath: "$.calibration.schema", Value: types.CalibrationSchemaV1}}
	if agent != "" {
		matches = append(matches, objectgraph.JSONFieldMatch{JSONPath: "$.calibration.agent", Value: agent})
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
			return nil, fmt.Errorf("calibration record decode %s: %w", obj.CanonicalID, err)
		}
		records = append(records, rec)
	}
	return records, nil
}

// dropCalibrationRecords removes agent calibration records (a non-empty
// "calibration" body field) and keeps at most limit of the rest, in order.
func dropCalibrationRecords(objs []objectgraph.Object, limit int) []objectgraph.Object {
	out := objs[:0]
	for _, obj := range objs {
		var probe struct {
			Calibration json.RawMessage `json:"calibration"`
		}
		if len(obj.Body) > 0 && json.Unmarshal(obj.Body, &probe) == nil {
			if c := strings.TrimSpace(string(probe.Calibration)); c != "" && c != "null" && c != `""` {
				continue
			}
		}
		out = append(out, obj)
		if len(out) == limit {
			break
		}
	}
	return out
}
