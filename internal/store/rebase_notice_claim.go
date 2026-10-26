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

// A claim's object key is types.StackRebaseClaimKey(pair, attempt): the
// automatic attempts share the pair's key, each owner retry has its own.

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
	obj, err := lifecycleObject(ogKindRebaseNoticeClaim, ownerID, computerID, rebaseNoticeClaimKey(types.StackRebaseClaimKey(claim.PairKey, claim.Attempt)), claim, map[string]any{
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

// GetRebaseNoticeClaim returns the claim at key (types.StackRebaseClaimKey of
// the pair and an attempt), or ErrNotFound.
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

// rebaseCastSpec is the part of management's engineering Cast spec the
// rebase admission reads. Other fields (parent, network, ...) are the
// ordinary path's.
type rebaseCastSpec struct {
	RebaseNotice string `json:"rebase_notice"`
	RebaseOf     string `json:"rebase_of"`
	Base         string `json:"base"`
}

// requireRebaseNoticeAdmission is U4's admission check, run inside the open's
// lock beside requireEngineeringRecordsAdmission (docs/design/
// u4-stack-rebase-build-2026-10-26.md, question 5). When management's Cast
// spec names a rebase notice it requires all of:
//   - the runtime admitted a rebase (req.Rebase) and this open's spec agrees;
//   - the notice record exists and passes IsStackRebaseNotice (an agent's
//     record never does);
//   - its inbox is the parent Cast's sender, and the parent's packet names
//     the same notice id (the root made this Cast);
//   - the spec's rebase_of and base equal the notice's subject refs, and the
//     objective contains the notice id;
//   - it is the latest attempt of its pair;
//   - it was not committed before this boot on a markerless boot (the runtime
//     passes the clock; the store does not read it);
//   - no claim exists for the pair.
//
// It returns the claim object, to join the open's batch, and the conditions
// that make it atomic: the claim must be absent, and so must any newer
// attempt of the notice. A spec without a notice returns nothing, and a
// runtime admission without a notice in the spec is refused.
func (s *Store) requireRebaseNoticeAdmission(ctx context.Context, req types.OpenEngineeringAssignmentRequest, records types.CommitInboxRecordsRequest, now time.Time) (*objectgraph.Object, []objectgraph.ObjectCondition, error) {
	refuse := func(code, format string, args ...any) error {
		return fmt.Errorf("co-super assignment: records admission: %s: "+format+": %w", append(append([]any{code}, args...), ErrEngineeringAssignmentInvalid)...)
	}
	// The Cast is the record the binding names as its parent control; the
	// ordinary admission has already refused a command whose record to the
	// assigned inbox is any other (review 321: do not rely on that ordering).
	var cast *types.CommitmentRecord
	for i := range records.Records {
		if strings.TrimSpace(records.Records[i].RecordID) == strings.TrimSpace(req.Binding.ParentControlID) &&
			strings.TrimSpace(records.Records[i].Inbox) == req.Binding.AssignedAgentID {
			cast = &records.Records[i]
		}
	}
	if cast == nil || cast.Directive == nil {
		return nil, nil, nil // requireEngineeringRecordsAdmission refuses this
	}
	noticeID := types.CastRebaseNotice(cast.Directive.Body)
	if noticeID == "" {
		if req.Rebase != nil {
			return nil, nil, refuse("rebase_notice_mismatch", "the runtime admitted a rebase but management's spec names no rebase_notice")
		}
		return nil, nil, nil
	}
	if req.Rebase == nil {
		return nil, nil, refuse("rebase_notice_mismatch", "management's spec names notice %s but the runtime did not admit a rebase", noticeID)
	}
	var spec rebaseCastSpec
	if err := json.Unmarshal([]byte(cast.Directive.Body), &spec); err != nil {
		return nil, nil, refuse("rebase_notice_mismatch", "management's spec is not JSON")
	}
	operationID, base, attempt, ok := types.ParseStackRebaseNoticeID(noticeID)
	if !ok {
		return nil, nil, refuse("rebase_notice_mismatch", "%q is not a rebase notice id", noticeID)
	}
	notice, err := s.GetCommitmentRecord(ctx, req.Binding.OwnerID, req.Binding.ComputerID, noticeID)
	if err != nil {
		return nil, nil, err
	}
	if notice == nil || !types.IsStackRebaseNotice(*notice) {
		return nil, nil, refuse("rebase_notice_mismatch", "notice %s is not a rebase notice on the tape", noticeID)
	}
	parent, err := s.GetCommitmentRecord(ctx, req.Binding.OwnerID, req.Binding.ComputerID, strings.TrimSpace(cast.ParentID))
	if err != nil {
		return nil, nil, err
	}
	if parent == nil || parent.Directive == nil {
		return nil, nil, refuse("rebase_notice_mismatch", "the parent Cast %q is not on the tape", cast.ParentID)
	}
	if strings.TrimSpace(notice.Inbox) != strings.TrimSpace(parent.SenderInbox) {
		return nil, nil, refuse("rebase_notice_mismatch", "notice %s was sent to %s, the parent Cast is from %s", noticeID, notice.Inbox, parent.SenderInbox)
	}
	if types.CastRebaseNotice(parent.Directive.Body) != noticeID {
		return nil, nil, refuse("rebase_notice_mismatch", "the parent Cast does not carry notice %s; the root did not make this request", noticeID)
	}
	if strings.TrimSpace(spec.RebaseOf) != operationID || strings.TrimSpace(notice.SubjectRefs["operation_id"]) != operationID {
		return nil, nil, refuse("rebase_notice_mismatch", "spec rebase_of %q is not the notice's operation %q", spec.RebaseOf, operationID)
	}
	if strings.TrimSpace(spec.Base) != base || strings.TrimSpace(notice.SubjectRefs["booted_base"]) != base {
		return nil, nil, refuse("rebase_notice_mismatch", "spec base %q is not the notice's base %q", spec.Base, base)
	}
	if !strings.Contains(cast.Directive.Objective, noticeID) {
		return nil, nil, refuse("rebase_notice_mismatch", "the objective does not name notice %s; a rebase Cast asks for the rebase", noticeID)
	}
	var newer []objectgraph.ObjectCondition
	for _, laterID := range types.StackRebaseNewerAttemptIDs(operationID, base, attempt) {
		later, err := s.GetCommitmentRecord(ctx, req.Binding.OwnerID, req.Binding.ComputerID, laterID)
		if err != nil {
			return nil, nil, err
		}
		if later != nil {
			return nil, nil, refuse("rebase_notice_superseded", "notice %s is superseded by %s", noticeID, laterID)
		}
		id, err := lifecycleCanonicalID(ogKindCommitmentRecord, req.Binding.OwnerID, req.Binding.ComputerID, laterID)
		if err != nil {
			return nil, nil, err
		}
		newer = append(newer, objectgraph.ObjectCondition{CanonicalID: id})
	}
	if !req.Rebase.PlannedBoot && !req.Rebase.BootedAt.IsZero() {
		committed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(notice.Provenance.CommittedAt))
		if err != nil || committed.Before(req.Rebase.BootedAt) {
			return nil, nil, refuse("rebase_notice_interrupted", "notice %s was committed before this boot and a crash closed it: interrupted by a restart", noticeID)
		}
	}
	pair := types.StackRebasePairKey(operationID, base)
	if _, err := s.GetRebaseNoticeClaim(ctx, req.Binding.OwnerID, req.Binding.ComputerID, types.StackRebaseClaimKey(pair, attempt)); err == nil {
		return nil, nil, refuse("rebase_notice_claimed", "pair %s is already claimed by an assignment", pair)
	} else if !errors.Is(err, ErrNotFound) {
		return nil, nil, err
	}
	claim, condition, err := rebaseNoticeClaimObject(req.Binding.OwnerID, req.Binding.ComputerID, types.StackRebaseClaim{
		PairKey: pair, OperationID: operationID, BootedBase: base, NoticeID: noticeID, Attempt: attempt, AssignmentID: req.AssignmentID,
	}, now)
	if err != nil {
		return nil, nil, err
	}
	return &claim, append(newer, condition), nil
}

// ErrRebaseRoundRunning refuses an owner's rebase notice while a rebase round
// of the operation has not ended.
var ErrRebaseRoundRunning = errors.New("a rebase round of this operation is still running")

// CommitRebaseOwnerNotice commits the owner's retry notice under the open
// path's lock, after re-checking that no rebase round of operationID runs
// (review 328: the check and the mint are one step, so an open for the last
// automatic attempt cannot slip between them; the open's own conditions refuse
// the other order, because attempt 4 supersedes attempts 1 to 3). A claim whose
// assignment cannot be read fails closed with its own error. A replay of the
// same command returns the committed receipt.
func (s *Store) CommitRebaseOwnerNotice(ctx context.Context, req types.CommitInboxRecordsRequest, operationID string) (types.InboxCommitReceipt, error) {
	if s.ogStore == nil {
		return types.InboxCommitReceipt{}, fmt.Errorf("commit rebase owner notice: object graph not initialized")
	}
	req, err := normalizeInboxCommand(req)
	if err != nil {
		return types.InboxCommitReceipt{}, err
	}
	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()
	if replay, found, err := s.replayInboxCommand(ctx, req.OwnerID, req.ComputerID, req.CommandID, req.CommandDigest); found || err != nil {
		return replay, err
	}
	claims, err := s.ListRebaseNoticeClaims(ctx, req.OwnerID, req.ComputerID, operationID)
	if err != nil {
		return types.InboxCommitReceipt{}, err
	}
	for _, claim := range claims {
		assignment, err := s.GetEngineeringAssignment(ctx, req.OwnerID, req.ComputerID, claim.AssignmentID, 1)
		if err != nil {
			return types.InboxCommitReceipt{}, fmt.Errorf("rebase claim %s: assignment %s cannot be read: %w", claim.PairKey, claim.AssignmentID, err)
		}
		if !assignment.Disposition.Terminal() {
			return types.InboxCommitReceipt{}, fmt.Errorf("%w: assignment %s", ErrRebaseRoundRunning, claim.AssignmentID)
		}
	}
	return s.commitInboxRecordsLocked(ctx, req)
}
