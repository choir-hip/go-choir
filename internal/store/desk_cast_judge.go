package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The store's part in judging desk-to-desk Casts (plan 4.4, amendment
// 2026-11-05). The store is the only writer of round state, so it is where
// "this round holds a scorable Cast" and "this round closed" are known
// atomically with the records that make them true. It writes two sub-call
// ledger records into the inbox batch itself:
//
//   - with the first scorable Cast of a round, the round's judge work
//     (attempt 0). The work is durable before the outcome, and before any
//     sleep of the judge;
//   - with the record that closes a round holding judge work, the round's
//     outcome (the closing record and its reason).
//
// After the batch commits, the runtime is told of each outcome. The judge
// itself runs in the runtime (agentcore/desk_cast_judge.go). Ways this part
// can fail, and how each is handled:
//
//  1. Settling before the outcome exists: the outcome record is in the
//     closing batch only, so it exists exactly when the round is closed.
//  2. Two scorable Casts of one round, in one batch or two: one work record
//     (deterministic id; a batch-local set and an existence read).
//  3. A round with no scorable Cast closes: no outcome, no judge, no records.
//  4. A batch that fails writes neither record (same batch).
//  5. A replayed command re-mints nothing (replay returns the receipt
//     before planning) and announces nothing.
//  6. A crash after commit, before the runtime is told: the work is open
//     with an outcome, and the boot sweep closes it visibly.

// DeskCastOutcomeNotifier is told, after a commit, of a round outcome it
// wrote. It must not block: the commit's caller may hold a store lock.
type DeskCastOutcomeNotifier func(ownerID, computerID, roundKey string)

// SetDeskCastOutcomeNotifier binds the runtime's judge to round outcomes.
func (s *Store) SetDeskCastOutcomeNotifier(fn DeskCastOutcomeNotifier) {
	if fn == nil {
		s.deskCastOutcomes.Store(nil)
		return
	}
	s.deskCastOutcomes.Store(&fn)
}

// commitmentRecordObject is the object of a plain (not inbox-plane)
// commitment record, with AppendCommitmentRecord's metadata.
func commitmentRecordObject(ownerID, computerID string, rec types.CommitmentRecord, now time.Time) (objectgraph.Object, error) {
	stamp := now.UTC().Format(time.RFC3339Nano)
	return lifecycleObject(ogKindCommitmentRecord, ownerID, computerID, rec.RecordID, rec, map[string]any{
		"record_id": rec.RecordID, "agent_id": rec.Provenance.AgentID, "model_id": rec.Provenance.ModelID,
		"discrepancy": string(rec.Discrepancy), "schema_id": rec.SchemaID, "created_at": stamp, "updated_at": stamp,
	}, now, now)
}

// deskCastJudgePlan is the batch-local state of planDeskCastJudge.
type deskCastJudgePlan struct {
	work map[string]bool // work ids known to exist (in the store or in this batch)
	done map[string]bool // outcome ids planned in this batch
}

// planDeskCastJudge returns the judge records rec adds to its batch: the
// round's work for a scorable Cast, the round's outcome when rec closed a
// round that has work. rec carries the round stepConversation stamped.
func (s *Store) planDeskCastJudge(ctx context.Context, ownerID, computerID string, rec types.CommitmentRecord, closed bool, plan *deskCastJudgePlan, now time.Time) ([]objectgraph.Object, []objectgraph.ObjectCondition, error) {
	if rec.Round <= 0 {
		return nil, nil, nil
	}
	key := types.DeskCastRoundKey(rec.ConversationID, rec.Round)
	workID := types.DeskCastJudgeWorkID(key, 0)
	stamp := now.UTC().Format(time.RFC3339Nano)
	var objects []objectgraph.Object
	var conditions []objectgraph.ObjectCondition
	add := func(r types.CommitmentRecord) error {
		obj, err := commitmentRecordObject(ownerID, computerID, r, now)
		if err != nil {
			return err
		}
		objects = append(objects, obj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: false})
		return nil
	}
	hasWork := func() (bool, error) {
		if plan.work[workID] {
			return true, nil
		}
		ok, err := s.CommitmentRecordExists(ctx, ownerID, computerID, workID)
		if ok {
			plan.work[workID] = true
		}
		return ok, err
	}
	if types.IsDeskCastScorable(rec) {
		ok, err := hasWork()
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			if err := add(types.DeskCastJudgeWork(computerID, key, 0, stamp)); err != nil {
				return nil, nil, err
			}
			plan.work[workID] = true
		}
	}
	if closed {
		ok, err := hasWork()
		if err != nil {
			return nil, nil, err
		}
		outcomeID := types.DeskCastOutcomeRecordID(key)
		if ok && !plan.done[outcomeID] {
			exists, err := s.CommitmentRecordExists(ctx, ownerID, computerID, outcomeID)
			if err != nil {
				return nil, nil, err
			}
			if !exists {
				if err := add(types.DeskCastOutcome(computerID, key, rec, stamp)); err != nil {
					return nil, nil, err
				}
			}
			plan.done[outcomeID] = true
		}
	}
	return objects, conditions, nil
}

// announceDeskCastOutcomes tells the runtime of each round outcome a
// committed batch wrote.
func (s *Store) announceDeskCastOutcomes(objects []objectgraph.Object) {
	p := s.deskCastOutcomes.Load()
	if p == nil {
		return
	}
	for _, obj := range objects {
		if obj.ObjectKind != ogKindCommitmentRecord || obj.Tombstone {
			continue
		}
		var meta struct {
			RecordID string `json:"record_id"`
		}
		if json.Unmarshal(obj.Metadata, &meta) != nil || !strings.HasPrefix(meta.RecordID, "deskjudge:") || !strings.HasSuffix(meta.RecordID, ":outcome") {
			continue
		}
		var rec types.CommitmentRecord
		if ogDecode(obj, &rec) != nil || rec.SubCall == nil || rec.SubCall.Kind != types.SubCallDeskOutcome {
			continue
		}
		(*p)(obj.OwnerID, obj.ComputerID, rec.SubCall.FanOutID)
	}
}
