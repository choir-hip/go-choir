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

// InboxCommitsWritten is the number of inbox command writes this process has
// made through commitInboxRecordsLocked (CommitInboxRecords and the rebase
// notice claim, which calls it directly) and CommitInboxRecordsBatch, one per
// write however many commands a batch held. A replay writes nothing and is
// not counted. Only tests read it, to count commits; the crash boot's closure
// counts its own commits locally.
func (s *Store) InboxCommitsWritten() int64 { return s.inboxCommits.Load() }

// CommitInboxRecordsBatch commits several independent inbox commands in ONE
// atomic write: all of them land or none does.
//
// Why a batch can carry commands of different conversations: planInboxRecords
// already plans any number of records in one pass. It keeps one cached inbox
// head and one cached conversation head per id, so two records of one inbox
// take consecutive sequence numbers in list order and a conversation's head
// steps once per record; the heads are written once at the end, each under one
// compare-and-set. What forbade batching closures was only the command: one
// command object per command id and one lock-and-replay-lookup per command.
// Here every command keeps its own id, digest and stored receipt (the
// inbox_command object), so a replay of any one of them, alone or in another
// batch, finds its receipt exactly as if it had committed alone, and the
// records, sequence numbers and heads come out as if the commands had
// committed one at a time in list order.
//
// A command already committed (same id and digest) is skipped and returns its
// stored receipt with Replay set; the same id under another digest fails the
// whole batch (ErrLifecycleCommandConflict), as it does alone. The batch
// takes plain records only: a cursor, support verdicts, task rows, a story
// library head or a Cast to texture:new make a command that needs its own
// commit (stories open in a second custodian part), and are refused.
// Receipts come back in request order. Any refusal, concurrent change or
// write failure leaves nothing written, and the caller commits the commands
// one at a time instead.
func (s *Store) CommitInboxRecordsBatch(ctx context.Context, reqs []types.CommitInboxRecordsRequest) ([]types.InboxCommitReceipt, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	if s.ogStore == nil {
		return nil, fmt.Errorf("commit inbox records batch: object graph not initialized")
	}
	normalized := make([]types.CommitInboxRecordsRequest, len(reqs))
	seenCommands := map[string]bool{}
	for i, req := range reqs {
		req, err := normalizeInboxCommand(req)
		if err != nil {
			return nil, err
		}
		if req.OwnerID != strings.TrimSpace(reqs[0].OwnerID) || req.ComputerID != strings.TrimSpace(reqs[0].ComputerID) {
			return nil, fmt.Errorf("commit inbox records batch: command %s is scoped to another computer", req.CommandID)
		}
		if req.Cursor != nil || len(req.Verdicts) > 0 || len(req.Tasks) > 0 || req.StoryLibraryHead != nil || strings.TrimSpace(req.Judge) != "" {
			return nil, fmt.Errorf("commit inbox records batch: command %s carries a cursor, verdicts, tasks or a library head; commit it alone", req.CommandID)
		}
		if len(req.Records) == 0 {
			return nil, inboxRefused("command %s has no records", req.CommandID)
		}
		for _, rec := range req.Records {
			if strings.TrimSpace(rec.Inbox) == types.StoryOpenInbox() {
				return nil, fmt.Errorf("commit inbox records batch: command %s opens a story; commit it alone", req.CommandID)
			}
		}
		if seenCommands[req.CommandID] {
			return nil, fmt.Errorf("commit inbox records batch: command %s appears twice", req.CommandID)
		}
		seenCommands[req.CommandID] = true
		normalized[i] = req
	}
	ownerID, computerID := normalized[0].OwnerID, normalized[0].ComputerID

	s.trajectoryMu.Lock()
	defer s.trajectoryMu.Unlock()

	receipts := make([]types.InboxCommitReceipt, len(normalized))
	var pending []int
	for i, req := range normalized {
		replay, found, err := s.replayInboxCommand(ctx, ownerID, computerID, req.CommandID, req.CommandDigest)
		if err != nil {
			return nil, err
		}
		if found {
			receipts[i] = replay
			continue
		}
		pending = append(pending, i)
	}
	if len(pending) == 0 {
		return receipts, nil
	}

	merged := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: normalized[pending[0]].CommandID}
	for _, i := range pending {
		merged.Records = append(merged.Records, normalized[i].Records...)
	}
	now := time.Now().UTC()
	objects, conditions, _, mergedReceipt, err := s.planInboxCommandBody(ctx, merged, now)
	if err != nil {
		return nil, err
	}
	byRecord := make(map[string]types.InboxRecordReceipt, len(mergedReceipt.Records))
	for _, row := range mergedReceipt.Records {
		byRecord[row.RecordID] = row
	}
	for _, i := range pending {
		req := normalized[i]
		receipt := types.InboxCommitReceipt{CommandID: req.CommandID, CommandDigest: req.CommandDigest, CreatedAt: now}
		for _, rec := range req.Records {
			row, ok := byRecord[strings.TrimSpace(rec.RecordID)]
			if !ok {
				return nil, fmt.Errorf("commit inbox records batch: no receipt for record %s", rec.RecordID)
			}
			receipt.Records = append(receipt.Records, row)
		}
		commandObj, err := inboxCommandObject(ownerID, computerID, receipt, now)
		if err != nil {
			return nil, err
		}
		objects = append(objects, commandObj)
		conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: commandObj.CanonicalID})
		receipts[i] = receipt
	}
	parts := s.inboxCommandParts(ctx, objects)
	if err := s.ogStore.PutBatchSegments(ctx, conditions, parts); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			return nil, ErrConcurrentStateChange
		}
		return nil, err
	}
	s.inboxCommits.Add(1)
	s.signalActorWakesIfAny(objects)
	return receipts, nil
}
