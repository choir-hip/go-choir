package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/paper-crash-boot-holds-work-six-minutes-2026-10-22.md, the
// 2026-11-18 section: 5,229 open rounds closed as 5,229 commits took 25
// minutes. The crash boot now commits its closures in batches of
// bootRoundClosureBatch.
//
// Ways this could fail, written first: it still commits once per round (T1);
// a batch writes records, sequence numbers or heads unlike the one-at-a-time
// closure (T5); a second boot writes again (T3); one conversation the store
// refuses blocks, or half-applies, the rest of its batch (T4); the last
// partial chunk is dropped; a round is logged or tallied that did not close.

func seedOpenRoundsForBoot(t *testing.T, s *store.Store, owner, computer string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		caster, recipient := "management:"+owner, fmt.Sprintf("texture:doc-%04d", i)
		if i%2 == 0 {
			caster, recipient = fmt.Sprintf("texture:doc-%04d", i), fmt.Sprintf("research:doc-%04d", i)
		}
		id := "cast-" + recipient
		rec := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "o " + id}, Cast: &types.CommitmentCast{},
			Inbox: recipient, SenderInbox: caster, Addressee: recipient, EmitterKind: types.CommitmentEmitterAgent,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: caster}}
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: id, Records: []types.CommitmentRecord{rec}}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
}

// T1 and T3: N open rounds close in ceil(N/batch) commits; a second boot
// writes nothing.
func TestCrashRoundClosureCommitsInBatches(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-crash-batch", rt.cfg.ComputerID
	t.Setenv("CHOIR_OWNER_ID", owner)
	n := 2*bootRoundClosureBatch + 37
	seedOpenRoundsForBoot(t, s, owner, computer, n)

	before := s.InboxCommitsWritten()
	rt.closeOpenRoundsAfterCrash(ctx)
	commits, want := s.InboxCommitsWritten()-before, int64((n+bootRoundClosureBatch-1)/bootRoundClosureBatch)
	if commits != want {
		t.Fatalf("%d open rounds closed in %d commits, want %d (batch %d)", n, commits, want, bootRoundClosureBatch)
	}
	if open, err := s.OpenConversationHeads(ctx, owner, computer); err != nil || len(open) != 0 {
		t.Fatalf("%d rounds still open after the closure (%v)", len(open), err)
	}
	if problems, err := s.VerifyInboxHeads(ctx, owner, computer); err != nil || len(problems) != 0 {
		t.Fatalf("heads disagree with the records: %v %v", problems, err)
	}

	before = s.InboxCommitsWritten()
	rt.closeOpenRoundsAfterCrash(ctx)
	if got := s.InboxCommitsWritten() - before; got != 0 {
		t.Fatalf("a second crash boot wrote %d commits", got)
	}
}

// T4: a conversation the store refuses (here, a head read with a round the
// store no longer holds) writes nothing with its batch, the batch falls back
// to one commit per round, and every other round closes.
func TestCrashRoundClosureBatchFallsBackOnOneBadConversation(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-crash-fallback", rt.cfg.ComputerID
	seedOpenRoundsForBoot(t, s, owner, computer, 6)
	open, err := s.OpenConversationHeads(ctx, owner, computer)
	if err != nil || len(open) != 6 {
		t.Fatalf("seed: %d open (%v)", len(open), err)
	}
	chunk := append([]types.ConversationHead{}, open...)
	chunk[2].Round += 5 // the closure names a round the conversation is not at
	closed, commits := rt.closeRoundsChunkAfterCrash(ctx, owner, computer, chunk, "2026-11-18T00:00:00Z")
	if closed != 5 || commits != 5 {
		t.Fatalf("closed %d rounds in %d commits, want 5 and 5 (fallback one at a time)", closed, commits)
	}
	left, _ := s.OpenConversationHeads(ctx, owner, computer)
	if len(left) != 1 || left[0].ConversationID != open[2].ConversationID {
		t.Fatalf("still open: %+v, want only %s", left, open[2].ConversationID)
	}
}

// T5: the records the batched boot writes are the records the one-at-a-time
// closure wrote. Ignored, because they legitimately differ between two runs:
// each record's Provenance.CommittedAt (the store's commit clock) and
// Observation.ObservedAt (the boot's clock, passed in as one string).
func TestCrashRoundClosureBatchWritesWhatOneAtATimeWrote(t *testing.T) {
	const n = 30
	snapshot := func(t *testing.T, s *store.Store, owner, computer string) string {
		t.Helper()
		ctx := context.Background()
		fold, err := s.RebuildInboxHeads(ctx, owner, computer)
		if err != nil {
			t.Fatal(err)
		}
		all := map[string][]types.CommitmentRecord{}
		for inbox := range fold.Heads {
			recs, err := s.ListInboxRecords(ctx, owner, computer, inbox, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			for i := range recs {
				recs[i].Provenance.CommittedAt, recs[i].Observation.ObservedAt = "", ""
			}
			all[inbox] = recs
		}
		out, _ := json.Marshal(map[string]any{"records": all, "heads": fold.Heads, "conversations": fold.Conversations})
		return string(out)
	}
	owner := "owner-crash-golden"
	var oneAtATime, batched string
	t.Run("one_at_a_time", func(t *testing.T) {
		rt, s := testRuntime(t)
		ctx := context.Background()
		computer := rt.cfg.ComputerID
		seedOpenRoundsForBoot(t, s, owner, computer, n)
		open, _ := s.OpenConversationHeads(ctx, owner, computer)
		for _, c := range open {
			closure := types.NewRoundClosureRecord(c.CasterInbox, c.RecipientInbox, c.Round, types.ClosureReasonInterruptedByRestart, "x")
			closure.ComputerID = computer
			req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "boot-closure:" + closure.RecordID, Records: []types.CommitmentRecord{closure}}
			req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
			if _, err := s.CommitInboxRecords(withKernelWriter(ctx), req); err != nil {
				t.Fatal(err)
			}
		}
		oneAtATime = snapshot(t, s, owner, computer)
	})
	t.Run("batched", func(t *testing.T) {
		rt, s := testRuntime(t)
		t.Setenv("CHOIR_OWNER_ID", owner)
		computer := rt.cfg.ComputerID
		seedOpenRoundsForBoot(t, s, owner, computer, n)
		rt.closeOpenRoundsAfterCrash(context.Background())
		batched = snapshot(t, s, owner, computer)
	})
	// Both runtimes must share one computer id for the snapshots to compare.
	if oneAtATime != batched {
		t.Fatalf("batched boot differs from one-at-a-time\none: %.500s\nbatch: %.500s", oneAtATime, batched)
	}
}
