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

// ogKindRebaseNoticeClaim is the claim of a stack rebase notice by the one
// assignment that serves it (U4, docs/design/u4-stack-rebase-build-2026-10-26.md,
// "Management's Cast is bound, and one notice opens one assignment"). It is
// keyed by the pair (operation, booted base), not by attempt: its existence
// means the pair is spent. The assignment open writes it in its own
// transaction under an Exists=false condition, so two racing opens cannot both
// commit. An additive kind: no table or column changes, and kind-filtered
// readers (the lifecycle snapshot, the inbox folds) skip an unknown kind, so a
// full revert leaves these objects inert.
const ogKindRebaseNoticeClaim = objectgraph.ObjectKind("choir.rebase_notice_claim")

func rebaseNoticeClaimKey(pairKey string) string { return strings.TrimSpace(pairKey) }

// validateRebaseNoticeClaim refuses a claim that names no pair, notice or
// assignment, or whose fields disagree with each other.
func validateRebaseNoticeClaim(claim types.StackRebaseClaim) error {
	pair := strings.TrimSpace(claim.PairKey)
	if pair == "" || strings.TrimSpace(claim.OperationID) == "" || strings.TrimSpace(claim.BootedBase) == "" ||
		strings.TrimSpace(claim.NoticeID) == "" || strings.TrimSpace(claim.AssignmentID) == "" || claim.Attempt < 1 {
		return fmt.Errorf("rebase notice claim: pair, operation, base, notice, attempt and assignment are required: %w", ErrEngineeringAssignmentInvalid)
	}
	if pair != types.StackRebasePairKey(claim.OperationID, claim.BootedBase) {
		return fmt.Errorf("rebase notice claim: pair %q is not the pair of %s on %s: %w", pair, claim.OperationID, claim.BootedBase, ErrEngineeringAssignmentInvalid)
	}
	if op, base, n, ok := types.ParseStackRebaseNoticeID(claim.NoticeID); !ok || op != strings.TrimSpace(claim.OperationID) || base != strings.TrimSpace(claim.BootedBase) || n != claim.Attempt {
		return fmt.Errorf("rebase notice claim: notice %q is not attempt %d of the pair: %w", claim.NoticeID, claim.Attempt, ErrEngineeringAssignmentInvalid)
	}
	return nil
}

// rebaseNoticeClaimObject builds the claim object and the condition that makes
// writing it atomic with the batch it joins (the object must be absent).
func rebaseNoticeClaimObject(ownerID, computerID string, claim types.StackRebaseClaim, now time.Time) (objectgraph.Object, objectgraph.ObjectCondition, error) {
	if err := validateRebaseNoticeClaim(claim); err != nil {
		return objectgraph.Object{}, objectgraph.ObjectCondition{}, err
	}
	claim.PairKey, claim.OperationID, claim.BootedBase = strings.TrimSpace(claim.PairKey), strings.TrimSpace(claim.OperationID), strings.TrimSpace(claim.BootedBase)
	claim.NoticeID, claim.AssignmentID = strings.TrimSpace(claim.NoticeID), strings.TrimSpace(claim.AssignmentID)
	if claim.ClaimedAt.IsZero() {
		claim.ClaimedAt = now
	}
	obj, err := lifecycleObject(ogKindRebaseNoticeClaim, ownerID, computerID, rebaseNoticeClaimKey(claim.PairKey), claim, map[string]any{
		"computer_id": computerID, "pair_key": claim.PairKey, "operation_id": claim.OperationID, "notice_id": claim.NoticeID, "assignment_id": claim.AssignmentID,
	}, claim.ClaimedAt, claim.ClaimedAt)
	if err != nil {
		return objectgraph.Object{}, objectgraph.ObjectCondition{}, err
	}
	return obj, objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID}, nil
}

// PutRebaseNoticeClaim writes a claim on its own, losing with
// ErrConcurrentStateChange when the pair already has one. The assignment open
// does not use it: it writes the claim in its own batch. This is the seam for
// repair and for tests of the readers.
func (s *Store) PutRebaseNoticeClaim(ctx context.Context, ownerID, computerID string, claim types.StackRebaseClaim) error {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if s.ogStore == nil {
		return fmt.Errorf("store: object graph not initialized")
	}
	obj, condition, err := rebaseNoticeClaimObject(ownerID, computerID, claim, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return ErrConcurrentStateChange
		}
		return err
	}
	return nil
}

// GetRebaseNoticeClaim returns the claim of a pair, or ErrNotFound.
func (s *Store) GetRebaseNoticeClaim(ctx context.Context, ownerID, computerID, pairKey string) (types.StackRebaseClaim, error) {
	obj, err := s.lifecycleGetObject(ctx, ogKindRebaseNoticeClaim, strings.TrimSpace(ownerID), strings.TrimSpace(computerID), rebaseNoticeClaimKey(pairKey))
	if err != nil {
		return types.StackRebaseClaim{}, err
	}
	return decodeLifecycleObject[types.StackRebaseClaim](obj)
}

// ListRebaseNoticeClaims returns every claim of one operation's pairs (one per
// booted base it was rebased toward), oldest first.
func (s *Store) ListRebaseNoticeClaims(ctx context.Context, ownerID, computerID, operationID string) ([]types.StackRebaseClaim, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	objs, err := s.ogListAllByMetadata(ctx, ogKindRebaseNoticeClaim, "operation_id", strings.TrimSpace(operationID))
	if err != nil {
		return nil, err
	}
	var out []types.StackRebaseClaim
	for _, obj := range objs {
		if obj.Tombstone || obj.OwnerID != ownerID || obj.ComputerID != computerID {
			continue
		}
		claim, err := decodeLifecycleObject[types.StackRebaseClaim](obj)
		if err != nil {
			return nil, err
		}
		out = append(out, claim)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ClaimedAt.Before(out[j].ClaimedAt) })
	return out, nil
}
