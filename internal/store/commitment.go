package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
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

// ListRulePrecommitRecords lists the rule-settled precommits the reducer
// minted for one engineering assignment (types.PrecommitRule.AssignmentID,
// runtime-stamped), oldest first.
func (s *Store) ListRulePrecommitRecords(ctx context.Context, ownerID, computerID, assignmentID string) ([]types.CommitmentRecord, error) {
	ownerID, computerID, assignmentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(assignmentID)
	if ownerID == "" || computerID == "" || assignmentID == "" {
		return nil, fmt.Errorf("rule precommit list requires owner, computer and assignment")
	}
	objs, err := s.ogListByOwnerAndBody(ctx, ogKindCommitmentRecord, ownerID,
		[]objectgraph.JSONFieldMatch{{JSONPath: "$.precommit.rule.assignment_id", Value: assignmentID}}, 200)
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
		if rec.Precommit == nil || rec.Precommit.Rule == nil || rec.Resolve != nil {
			continue
		}
		records = append(records, rec)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Provenance.CommittedAt != records[j].Provenance.CommittedAt {
			return records[i].Provenance.CommittedAt < records[j].Provenance.CommittedAt
		}
		return records[i].RecordID < records[j].RecordID
	})
	return records, nil
}

// SettleRulePrecommits appends the rule scorer's resolve record for every
// open rule precommit of the assignment that settle decides. Each
// settlement has one deterministic id written with a not-exists condition,
// so the first settlement wins and a replay writes nothing (failure mode
// R4, types/commitment_rules.go). It returns the records it appended.
func (s *Store) SettleRulePrecommits(ctx context.Context, ownerID, computerID, assignmentID string, at time.Time,
	settle func(types.PrecommitRule) (types.RuleSettlement, bool)) ([]types.CommitmentRecord, error) {
	precommits, err := s.ListRulePrecommitRecords(ctx, ownerID, computerID, assignmentID)
	if err != nil {
		return nil, err
	}
	var appended []types.CommitmentRecord
	for _, precommit := range precommits {
		resolveID := types.RuleResolveRecordID(precommit.RecordID)
		settled, err := s.CommitmentRecordExists(ctx, ownerID, computerID, resolveID)
		if err != nil {
			return appended, err
		}
		if settled {
			continue
		}
		settlement, ok := settle(*precommit.Precommit.Rule)
		if !ok {
			continue
		}
		resolve := types.NewRuleResolveRecord(precommit, settlement, at)
		if _, err := s.AppendCommitmentRecord(ctx, ownerID, computerID, resolve); err != nil {
			return appended, err
		}
		appended = append(appended, resolve)
	}
	return appended, nil
}

// GetCommitmentRecord returns the stored record body for one record id,
// or nil when absent. Read-only — used by the learning-claims gate to
// check whether a verifier ref names a scored record.
//
// It reads the record's object by its key: every writer keys a commitment
// record on (owner, computer, record id), the identity CommitmentRecordExists
// already trusts as the crash-recovery witness. It used to scan every record
// body of the owner for $.record_id, 2.3 s per call under the engine mutex on
// the paper (docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6).
func (s *Store) GetCommitmentRecord(ctx context.Context, ownerID, computerID, recordID string) (*types.CommitmentRecord, error) {
	ownerID, computerID, recordID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(recordID)
	if ownerID == "" || computerID == "" || recordID == "" {
		return nil, nil
	}
	if s == nil || s.ogStore == nil {
		return nil, fmt.Errorf("commitment record lookup: object graph not initialized")
	}
	canonicalID, err := lifecycleCanonicalID(ogKindCommitmentRecord, ownerID, computerID, recordID)
	if err != nil {
		return nil, err
	}
	obj, err := s.ogStore.GetObject(ctx, canonicalID)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if obj.Tombstone || obj.ObjectKind != ogKindCommitmentRecord || strings.TrimSpace(obj.ComputerID) != computerID {
		return nil, nil
	}
	var rec types.CommitmentRecord
	if err := ogDecode(obj, &rec); err != nil {
		return nil, fmt.Errorf("commitment record decode %s: %w", obj.CanonicalID, err)
	}
	if rec.RecordID != recordID {
		return nil, nil
	}
	return &rec, nil
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
