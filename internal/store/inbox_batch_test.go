package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/paper-crash-boot-holds-work-six-minutes-2026-10-22.md, the
// 2026-11-18 section: the crash boot closed 5,229 rounds as 5,229 commits,
// 25 minutes. CommitInboxRecordsBatch commits many commands in one write.
//
// Ways it can fail, written first: the batch takes the same sequence number
// twice in a shared inbox (every closure of a management-cast round lands in
// management's inbox), or skips one; a conversation head steps wrongly when
// closures of many conversations share a plan; a command's receipt is not
// stored under its own id, so a replay writes again; one bad command writes
// the commands before it (half a batch); a command that needs its own
// commit (a cursor, verdicts, a story open) rides in a batch; a command
// already committed alone is committed twice; the batch's stored records
// differ from the one-at-a-time ones in anything but the commit time.

const batchTestReason = types.ClosureReasonInterruptedByRestart

// seedOpenRounds opens n rounds in n conversations and returns the closure
// each needs. Even ones are cast by a texture desk to its research desk (each
// closure lands in its own texture inbox); odd ones are cast by management to
// a texture desk (every closure lands in management's one inbox, whose head
// the batch must advance n/2 times).
func seedOpenRounds(t *testing.T, s *Store, n int) []types.CommitmentRecord {
	t.Helper()
	var closures []types.CommitmentRecord
	for i := 0; i < n; i++ {
		var caster, recipient string
		if i%2 == 0 {
			caster, recipient = fmt.Sprintf("texture:doc-%03d", i), fmt.Sprintf("research:doc-%03d", i)
		} else {
			caster, recipient = "management:"+inboxTestOwner, fmt.Sprintf("texture:doc-%03d", i)
		}
		mustCommitInbox(t, s, "cast-"+recipient, nil, inboxCast("cast-"+recipient, caster, recipient, types.CommitmentCast{}))
		closures = append(closures, types.NewRoundClosureRecord(caster, recipient, 1, batchTestReason, "2026-11-18T00:00:00Z"))
	}
	return closures
}

func closureRequest(t *testing.T, closure types.CommitmentRecord) types.CommitInboxRecordsRequest {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: "boot-closure:" + closure.RecordID, Records: []types.CommitmentRecord{closure}}
	digest, err := ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	return req
}

func closureRequests(t *testing.T, closures []types.CommitmentRecord) []types.CommitInboxRecordsRequest {
	var reqs []types.CommitInboxRecordsRequest
	for _, c := range closures {
		reqs = append(reqs, closureRequest(t, c))
	}
	return reqs
}

// storedState is everything the closures wrote that must not depend on how
// they were committed: each inbox's records in sequence order, the inbox
// heads and the conversation heads. Fields that legitimately differ and are
// zeroed: Provenance.CommittedAt of each record (the store's commit clock);
// the receipts' CreatedAt (the same clock).
type storedState struct {
	Records       map[string][]types.CommitmentRecord
	Heads         map[string]int64
	Conversations map[string]types.ConversationHead
	Receipts      []types.InboxCommitReceipt
}

func readStoredState(t *testing.T, s *Store, closures []types.CommitmentRecord, receipts []types.InboxCommitReceipt) string {
	t.Helper()
	ctx := context.Background()
	inboxes := map[string]bool{}
	for _, c := range closures {
		inboxes[c.Inbox], inboxes[c.SenderInbox] = true, true
	}
	state := storedState{Records: map[string][]types.CommitmentRecord{}}
	for inbox := range inboxes {
		recs, err := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, inbox, 0, 0)
		if err != nil {
			t.Fatalf("list %s: %v", inbox, err)
		}
		for i := range recs {
			recs[i].Provenance.CommittedAt = ""
		}
		state.Records[inbox] = recs
	}
	fold, err := s.RebuildInboxHeads(ctx, inboxTestOwner, inboxTestComputer)
	if err != nil {
		t.Fatalf("fold: %v", err)
	}
	state.Heads, state.Conversations = fold.Heads, fold.Conversations
	for _, r := range receipts {
		r.CreatedAt = time.Time{}
		state.Receipts = append(state.Receipts, r)
	}
	// The cached heads are what the runtime reads; they must equal the fold.
	assertInboxHeadsMatchRebuild(t, s)
	out, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	return string(out)
}

// T2: the same closures, committed one at a time and committed as one batch,
// leave the same records (same sequence numbers in the shared inbox), the same
// inbox and conversation heads and the same receipts.
func TestCommitInboxRecordsBatchEqualsOneAtATime(t *testing.T) {
	const n = 40
	var single, batched string
	t.Run("one_at_a_time", func(t *testing.T) {
		s := openTestStore(t)
		closures := seedOpenRounds(t, s, n)
		var receipts []types.InboxCommitReceipt
		for _, req := range closureRequests(t, closures) {
			r, err := s.CommitInboxRecords(context.Background(), req)
			if err != nil {
				t.Fatalf("commit %s: %v", req.CommandID, err)
			}
			receipts = append(receipts, r)
		}
		single = readStoredState(t, s, closures, receipts)
	})
	t.Run("one_batch", func(t *testing.T) {
		s := openTestStore(t)
		closures := seedOpenRounds(t, s, n)
		before := s.InboxCommitsWritten()
		receipts, err := s.CommitInboxRecordsBatch(context.Background(), closureRequests(t, closures))
		if err != nil {
			t.Fatalf("batch: %v", err)
		}
		if got := s.InboxCommitsWritten() - before; got != 1 {
			t.Fatalf("a batch of %d closures took %d commits, want 1", n, got)
		}
		batched = readStoredState(t, s, closures, receipts)
	})
	if single != batched {
		t.Fatalf("batch and one-at-a-time differ\nsingle:  %.600s\nbatched: %.600s", single, batched)
	}
}

// Every closure keeps its own command id: a replay of one finds its receipt.
func TestCommitInboxRecordsBatchKeepsEachCommandReplayable(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	closures := seedOpenRounds(t, s, 6)
	reqs := closureRequests(t, closures)
	first, err := s.CommitInboxRecordsBatch(ctx, reqs)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	written := s.InboxCommitsWritten()
	for i, req := range reqs {
		replay, err := s.CommitInboxRecords(ctx, req)
		if err != nil || !replay.Replay {
			t.Fatalf("command %s alone after the batch: replay=%v err=%v", req.CommandID, replay.Replay, err)
		}
		if replay.CommandID != req.CommandID || len(replay.Records) != 1 || replay.Records[0].InboxSeq != first[i].Records[0].InboxSeq {
			t.Fatalf("command %s replayed %+v, batch gave %+v", req.CommandID, replay, first[i])
		}
	}
	// T3: the whole batch again writes nothing.
	again, err := s.CommitInboxRecordsBatch(ctx, reqs)
	if err != nil {
		t.Fatalf("second batch: %v", err)
	}
	for _, r := range again {
		if !r.Replay {
			t.Fatalf("second batch wrote: %+v", r)
		}
	}
	if s.InboxCommitsWritten() != written {
		t.Fatalf("replays counted as commits")
	}
	assertInboxHeadsMatchRebuild(t, s)
}

// A command already committed alone is skipped, the rest commit.
func TestCommitInboxRecordsBatchSkipsCommandsAlreadyCommitted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	closures := seedOpenRounds(t, s, 5)
	reqs := closureRequests(t, closures)
	if _, err := s.CommitInboxRecords(ctx, reqs[1]); err != nil {
		t.Fatal(err)
	}
	receipts, err := s.CommitInboxRecordsBatch(ctx, reqs)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	for i, r := range receipts {
		if r.Replay != (i == 1) {
			t.Fatalf("command %d replay=%v", i, r.Replay)
		}
	}
	if open, _ := s.OpenConversationHeads(ctx, inboxTestOwner, inboxTestComputer); len(open) != 0 {
		t.Fatalf("%d rounds still open", len(open))
	}
	assertInboxHeadsMatchRebuild(t, s)
}

// T4 (store half): one command the store refuses leaves nothing written, so
// the caller can commit the others one at a time.
func TestCommitInboxRecordsBatchIsAllOrNothing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	closures := seedOpenRounds(t, s, 6)
	// Close round of conversation 3 first: its closure in the batch now has
	// no open round to close.
	if _, err := s.CommitInboxRecords(ctx, closureRequest(t, closures[3])); err != nil {
		t.Fatal(err)
	}
	other := closures[3]
	other.RecordID += ":again"
	reqs := closureRequests(t, append(append([]types.CommitmentRecord{}, closures[:3]...), other))
	before := s.InboxCommitsWritten()
	if _, err := s.CommitInboxRecordsBatch(ctx, reqs); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("want ErrInboxRefused, got %v", err)
	}
	if s.InboxCommitsWritten() != before {
		t.Fatalf("a refused batch counted as a commit")
	}
	open, err := s.OpenConversationHeads(ctx, inboxTestOwner, inboxTestComputer)
	if err != nil || len(open) != 5 {
		t.Fatalf("open rounds after a refused batch = %d (%v), want 5: the batch half-applied", len(open), err)
	}
	// Falling back one at a time closes the good ones.
	for _, req := range reqs[:3] {
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("fallback %s: %v", req.CommandID, err)
		}
	}
	assertInboxHeadsMatchRebuild(t, s)
}

// A command that needs its own commit is refused, not silently batched.
func TestCommitInboxRecordsBatchRefusesCommandsThatNeedTheirOwnCommit(t *testing.T) {
	s := openTestStore(t)
	closures := seedOpenRounds(t, s, 2)
	reqs := closureRequests(t, closures)
	reqs[1].Cursor = &types.InboxCursorAdvance{ReaderAgentID: "texture:x", Inbox: "texture:x", Seq: 0}
	reqs[1].CommandDigest, _ = ComputeCommitInboxRecordsDigest(reqs[1])
	if _, err := s.CommitInboxRecordsBatch(context.Background(), reqs); err == nil {
		t.Fatal("a command with a cursor rode in a batch")
	}
	if open, _ := s.OpenConversationHeads(context.Background(), inboxTestOwner, inboxTestComputer); len(open) != 2 {
		t.Fatalf("open = %d, want 2", len(open))
	}
}
