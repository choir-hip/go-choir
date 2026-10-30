package agentcore

import (
	"context"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/paper-crash-boot-holds-work-six-minutes-2026-10-22.md C1:
// the crash round closure folded every inbox record of the computer (a
// paged JSON pass over commitment records) to find the open rounds, 4m36s
// on the paper. The conversation heads are written atomically with their
// records (C2), so the open set is read from them.
//
// Ways this could fail, written first: the closure still reads commitment
// records by a JSON path; it reads the heads but misses an open round (one
// whose recipient is not a desk the boot knows, or a second round of a
// conversation that closed its first); it closes a closed round; it writes
// a closure to the wrong inbox; a second crash boot writes again.
func TestCrashRoundClosureReadsHeadsNotRecords(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-crash-heads", rt.cfg.ComputerID
	t.Setenv("CHOIR_OWNER_ID", owner)
	commit := func(id string, recs ...types.CommitmentRecord) {
		t.Helper()
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: id, Records: recs}
		req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
		if _, err := s.CommitInboxRecords(ctx, req); err != nil {
			t.Fatalf("commit %s: %v", id, err)
		}
	}
	cast := func(id, to, from string) types.CommitmentRecord {
		return types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
			Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "o " + id}, Cast: &types.CommitmentCast{},
			Inbox: to, SenderInbox: from, Addressee: to, EmitterKind: types.CommitmentEmitterAgent,
			Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: from}}
	}
	// History: rounds opened and closed by a system closure, the shape every
	// earlier crash left behind.
	for i := 0; i < 20; i++ {
		texture, research := fmt.Sprintf("texture:doc-old-%02d", i), fmt.Sprintf("research:doc-old-%02d", i)
		commit("old-cast-"+texture, cast("old-cast-"+texture, research, texture))
		commit("old-close-"+texture, closureFor(texture, research, 1))
	}
	// Open now: two conversations, one of them in its second round.
	commit("open-a", cast("open-a", "research:doc-a", "texture:doc-a"))
	commit("open-b1", cast("open-b1", "research:doc-b", "texture:doc-b"))
	commit("close-b1", closureFor("texture:doc-b", "research:doc-b", 1))
	commit("open-b2", cast("open-b2", "research:doc-b", "texture:doc-b"))
	if fold, err := s.RebuildInboxHeads(ctx, owner, computer); err != nil || countOpen(fold) != 2 {
		t.Fatalf("seed: fold has %d open conversations (%v), want 2", countOpen(fold), err)
	}

	rt.closeOpenRoundsAfterCrash(ctx)

	fold, err := s.RebuildInboxHeads(ctx, owner, computer)
	if err != nil {
		t.Fatalf("fold after closure: %v", err)
	}
	if open := countOpen(fold); open != 0 {
		t.Fatalf("after the crash closure %d conversations are still open", open)
	}
	for _, texture := range []string{"texture:doc-a", "texture:doc-b"} {
		recs, _ := s.ListInboxRecords(ctx, owner, computer, texture, 0, 0)
		last := recs[len(recs)-1]
		if last.EmitterKind != types.CommitmentEmitterSystem || !last.Final || last.Report == nil ||
			last.Report.StopReason != types.ClosureReasonInterruptedByRestart {
			t.Fatalf("%s: last record %+v, want a system closure with %s", texture, last, types.ClosureReasonInterruptedByRestart)
		}
	}
	total := len(fold.Heads)
	seqs := map[string]int64{}
	for inbox, seq := range fold.Heads {
		seqs[inbox] = seq
	}
	rt.closeOpenRoundsAfterCrash(ctx)
	again, _ := s.RebuildInboxHeads(ctx, owner, computer)
	if len(again.Heads) != total {
		t.Fatalf("a second crash boot added inboxes: %d, want %d", len(again.Heads), total)
	}
	for inbox, seq := range again.Heads {
		if seqs[inbox] != seq {
			t.Fatalf("a second crash boot wrote to %s: seq %d, was %d", inbox, seq, seqs[inbox])
		}
	}
	if problems, err := s.VerifyInboxHeads(ctx, owner, computer); err != nil || len(problems) != 0 {
		t.Fatalf("heads disagree with the records after the closure: %v %v", problems, err)
	}
}

func closureFor(caster, recipient string, round int) types.CommitmentRecord {
	rec := types.NewRoundClosureRecord(caster, recipient, round, types.ClosureReasonInterruptedByRestart, "2026-10-22T00:00:00Z")
	return rec
}

func countOpen(fold store.InboxFold) int {
	n := 0
	for _, c := range fold.Conversations {
		if c.Open {
			n++
		}
	}
	return n
}
