package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The decision record (R0): append-only rows of kind choir.decision_record,
// written under the decisions service's writer (system:decisions), so no desk
// writes them. Each row is an object of the computer's store and so rides the
// tape: pinned on the host, its head receipted in corpusd, like every other
// write. A row's identity is its capture, stage, command, decision and
// target (two decisions of one capture in one command are two rows), so a
// replay of the same decision finds the row and writes nothing, and a row, once
// written, is never changed.

const ogKindDecisionRecord = objectgraph.ObjectKind("choir.decision_record")

// decisionBetweenCheckAndWrite is a test seam: it runs after the existence
// checks and before the batch is written, where a concurrent writer can
// append one of the rows. Nil in the product.
var decisionBetweenCheckAndWrite func()

const decisionListDefault, decisionListMax = 200, 5000

func decisionKey(r types.DecisionRecord) string {
	return strings.Join([]string{r.CaptureID, r.Stage, r.CommandID, r.Decision, r.Target}, "\x00")
}

// AppendDecisionRecords writes rows as one atomic batch under the decisions
// writer and returns how many were new. A row already written is skipped.
func (s *Store) AppendDecisionRecords(ctx context.Context, rows []types.DecisionRecord) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	// The idempotency read below is the decisions service's read of its own
	// kind, whoever the caller is (a management cell's replay, or a context
	// that declares nobody).
	ctx = objectgraph.WithReader(ctx, objectgraph.WriterDecisions)
	if s.ogStore == nil {
		return 0, fmt.Errorf("decision record: object graph not initialized")
	}
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	seen := map[string]bool{}
	for _, r := range rows {
		r.OwnerID, r.ComputerID = strings.TrimSpace(r.OwnerID), strings.TrimSpace(r.ComputerID)
		r.CaptureID, r.Stage, r.Decision = strings.TrimSpace(r.CaptureID), strings.TrimSpace(r.Stage), strings.TrimSpace(r.Decision)
		if r.OwnerID == "" || r.ComputerID == "" || r.CaptureID == "" || r.Stage == "" || r.Decision == "" || strings.TrimSpace(r.Decider) == "" || strings.TrimSpace(r.DeciderKind) == "" {
			return 0, fmt.Errorf("decision record: owner, computer, capture, stage, decision and decider are required")
		}
		if r.At.IsZero() {
			r.At = time.Now()
		}
		r.At = r.At.UTC()
		key := decisionKey(r)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, err := s.lifecycleGetObject(ctx, ogKindDecisionRecord, r.OwnerID, r.ComputerID, key); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return 0, err
		}
		obj, err := lifecycleObject(ogKindDecisionRecord, r.OwnerID, r.ComputerID, key, r,
			map[string]any{"capture_id": r.CaptureID, "stage": r.Stage, "computer_id": r.ComputerID}, r.At, r.At)
		if err != nil {
			return 0, err
		}
		objects = append(objects, obj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID})
	}
	if len(objects) == 0 {
		return 0, nil
	}
	if decisionBetweenCheckAndWrite != nil {
		decisionBetweenCheckAndWrite()
	}
	// A conflict means another writer appended one of these rows between the
	// check and the write: that row is there, which is what was asked. The
	// others are not written by a refused batch, so each round drops the rows
	// that now exist and writes the rest; a round that writes nothing new
	// ends it. Each conflict removes at least one row, so the loop ends.
	written := 0
	for len(objects) > 0 {
		parts := []objectgraph.WriterBatch{{Writer: objectgraph.WriterDecisions, Batch: objectgraph.Batch{Objects: objects}}}
		err := s.ogStore.PutBatchSegments(ctx, conditions, parts)
		if err == nil {
			return written + len(objects), nil
		}
		if !errors.Is(err, objectgraph.ErrConflict) {
			return written, err
		}
		var restObjects []objectgraph.Object
		var restConditions []objectgraph.ObjectCondition
		for i, obj := range objects {
			if _, getErr := s.ogStore.GetObject(ctx, obj.CanonicalID); getErr == nil {
				continue
			} else if !errors.Is(getErr, objectgraph.ErrNotFound) {
				return written, getErr
			}
			restObjects, restConditions = append(restObjects, obj), append(restConditions, conditions[i])
		}
		if len(restObjects) == len(objects) {
			// Nothing of the batch exists and it still conflicted: the
			// conflict is not a row of ours.
			return written, err
		}
		objects, conditions = restObjects, restConditions
	}
	return written, nil
}

// ListDecisionRecords reads the computer's rows, newest first.
func (s *Store) ListDecisionRecords(ctx context.Context, ownerID, computerID string, f types.DecisionFilter) ([]types.DecisionRecord, error) {
	ctx = objectgraph.WithReader(ctx, objectgraph.WriterDecisions)
	limit := f.Limit
	if limit <= 0 {
		limit = decisionListDefault
	}
	if limit > decisionListMax {
		limit = decisionListMax
	}
	matches := []objectgraph.JSONFieldMatch{{JSONPath: "$.computer_id", Value: strings.TrimSpace(computerID)}}
	if v := strings.TrimSpace(f.CaptureID); v != "" {
		matches = append(matches, objectgraph.JSONFieldMatch{JSONPath: "$.capture_id", Value: v})
	}
	if v := strings.TrimSpace(f.Stage); v != "" {
		matches = append(matches, objectgraph.JSONFieldMatch{JSONPath: "$.stage", Value: v})
	}
	objs, err := s.ogListByOwnerAndBodySince(ctx, ogKindDecisionRecord, strings.TrimSpace(ownerID), matches, f.Since, limit)
	if err != nil {
		return nil, err
	}
	out := make([]types.DecisionRecord, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone {
			continue
		}
		var r types.DecisionRecord
		if err := ogDecode(obj, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.After(out[j].At) })
	return out, nil
}
