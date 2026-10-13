package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Inbox record substrate (Cut 1 L3a). Ways it can fail, written before the
// code:
//
//  1. Two concurrent commits to one inbox take the same sequence number, or
//     a failed commit burns one (a gap). The head is advanced by CAS in the
//     record's own batch, so a lost race refuses the whole batch.
//  2. One inbox's sequence leaks into another's (a shared counter).
//  3. A Cast with no open round fails to open one, or an Ask, Reply or
//     Note closes one, or a non-final Report closes one.
//  4. A final Report from the round's caster, or with no open round,
//     closes something; a stale final names an older round and closes a
//     newer one.
//  5. A Cast racing a final Report lands in the round the Report closed.
//  6. Hypothesis ids collide or move; a revise names an unknown id; a
//     hypothesis settles or drops twice; the wrong side settles or
//     withdraws it.
//  7. A cursor moves from a stale value, moves backward, or passes its
//     head.
//  8. A replayed command mints again (new seq) or a different command
//     under the same id is accepted.
//  9. A cached head drifts from the records it summarizes.
// 10. An addressed record mints no wake, or a wake under a non-deterministic
//     key; a system closure wakes someone.
// 11. The planner composed into a caller's batch writes part of the batch
//     when a head CAS fails.

const inboxTestOwner, inboxTestComputer = "owner-inbox", "computer-inbox"

func inboxRecord(id, from, to string) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: from, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterAgent, Inbox: to, SenderInbox: from,
	}
}

func inboxNote(id, from, to string) types.CommitmentRecord {
	rec := inboxRecord(id, from, to)
	rec.Kind = types.CommitmentKindDirective
	rec.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveNote, Body: "note " + id}
	return rec
}

func inboxAsk(id, from, to string) types.CommitmentRecord {
	rec := inboxRecord(id, from, to)
	rec.Kind = types.CommitmentKindPrecommit
	rec.Precommit = &types.CommitmentPrecommit{Question: "which source?", Distribution: map[string]float64{"a": 1}, Resolver: to}
	return rec
}

func inboxCast(id, from, to string, cast types.CommitmentCast) types.CommitmentRecord {
	rec := inboxRecord(id, from, to)
	rec.Kind = types.CommitmentKindDirective
	rec.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "look into " + id}
	rec.Cast = &cast
	return rec
}

func inboxReport(id, from, to string, final bool, report types.CommitmentReport) types.CommitmentRecord {
	rec := inboxRecord(id, from, to)
	rec.Kind = types.CommitmentKindReport
	rec.Final = final
	rec.Report = &report
	return rec
}

func hyp(statement string, p float64) types.CastHypothesis {
	return types.CastHypothesis{Statement: statement, Probability: p, Stakes: "changes the lede", Crux: "a primary record"}
}

func commitInbox(t *testing.T, s *Store, commandID string, cursor *types.InboxCursorAdvance, records ...types.CommitmentRecord) (types.InboxCommitReceipt, error) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: commandID, Records: records, Cursor: cursor}
	digest, err := ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	return s.CommitInboxRecords(context.Background(), req)
}

func mustCommitInbox(t *testing.T, s *Store, commandID string, cursor *types.InboxCursorAdvance, records ...types.CommitmentRecord) types.InboxCommitReceipt {
	t.Helper()
	receipt, err := commitInbox(t, s, commandID, cursor, records...)
	if err != nil {
		t.Fatalf("commit %s: %v", commandID, err)
	}
	return receipt
}

func mustRefuseInbox(t *testing.T, s *Store, commandID string, records ...types.CommitmentRecord) {
	t.Helper()
	if _, err := commitInbox(t, s, commandID, nil, records...); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("commit %s: want ErrInboxRefused, got %v", commandID, err)
	}
}

func assertInboxHeadsMatchRebuild(t *testing.T, s *Store) {
	t.Helper()
	problems, err := s.VerifyInboxHeads(context.Background(), inboxTestOwner, inboxTestComputer)
	if err != nil {
		t.Fatalf("verify inbox heads: %v", err)
	}
	if len(problems) != 0 {
		t.Fatalf("inbox heads drift from records: %v", problems)
	}
}

func TestInboxSeqGaplessUniqueUnderConcurrentCommits(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const writers = 12
	var wg sync.WaitGroup
	seqs := make(chan int64, writers)
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			from := fmt.Sprintf("research:doc-%d", i)
			receipt, err := commitInbox(t, s, fmt.Sprintf("cmd-note-%d", i), nil, inboxNote(fmt.Sprintf("note-%d", i), from, "texture:doc"))
			if err != nil {
				errs <- err
				return
			}
			seqs <- receipt.Records[0].InboxSeq
		}(i)
	}
	wg.Wait()
	close(seqs)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent commit: %v", err)
	}
	var got []int64
	for seq := range seqs {
		got = append(got, seq)
	}
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	for i, seq := range got {
		if seq != int64(i+1) {
			t.Fatalf("sequence not gapless and unique: %v", got)
		}
	}
	// Another inbox numbers independently.
	for i := 0; i < 3; i++ {
		receipt := mustCommitInbox(t, s, fmt.Sprintf("cmd-mgmt-%d", i), nil, inboxNote(fmt.Sprintf("mgmt-%d", i), "texture:doc", "management:owner"))
		if receipt.Records[0].InboxSeq != int64(i+1) {
			t.Fatalf("management inbox seq = %d, want %d", receipt.Records[0].InboxSeq, i+1)
		}
	}
	if head, err := s.InboxHead(ctx, inboxTestOwner, inboxTestComputer, "texture:doc"); err != nil || head != writers {
		t.Fatalf("texture head = %d, %v; want %d", head, err, writers)
	}
	records, err := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, "texture:doc", 4, 7)
	if err != nil || len(records) != 3 || records[0].InboxSeq != 5 || records[2].InboxSeq != 7 {
		t.Fatalf("list (4,7] = %d records, %v", len(records), err)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestInboxRoundsOpenStayOpenAndClose(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	tex, res := "texture:doc", "research:doc"
	conv := types.InboxConversationID(tex, res)

	// A final Report with no open round is refused.
	mustRefuseInbox(t, s, "cmd-early-final", inboxReport("early-final", res, tex, true, types.CommitmentReport{StopReason: "done"}))

	cast := mustCommitInbox(t, s, "cmd-cast-1", nil, inboxCast("cast-1", tex, res, types.CommitmentCast{Hypotheses: []types.CastHypothesis{hyp("a", 0.6)}}))
	if r := cast.Records[0]; r.Round != 1 || !r.OpenedRound || r.ConversationID != conv {
		t.Fatalf("first cast receipt = %+v", r)
	}
	// Ask, Reply (non-final Report) and Note stay in the open round.
	in := mustCommitInbox(t, s, "cmd-in-round", nil,
		inboxAsk("ask-1", res, tex),
		inboxReport("reply-1", tex, res, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "use a"}}}),
		inboxNote("note-1", res, tex),
		inboxReport("progress-1", res, tex, false, types.CommitmentReport{Findings: []types.ReportFinding{{Text: "x", BearsOn: []string{"H1"}}}}))
	for _, r := range in.Records {
		if r.Round != 1 || r.ClosedRound || r.ConversationID != conv {
			t.Fatalf("in-round record receipt = %+v", r)
		}
	}
	// The caster may keep casting into its open round; the recipient may not
	// cast into it, and the caster may not close it.
	if again := mustCommitInbox(t, s, "cmd-cast-1b", nil, inboxCast("cast-1b", tex, res, types.CommitmentCast{})); again.Records[0].Round != 1 || again.Records[0].OpenedRound {
		t.Fatalf("second cast in open round = %+v", again.Records[0])
	}
	mustRefuseInbox(t, s, "cmd-cast-wrong-side", inboxCast("cast-wrong-side", res, tex, types.CommitmentCast{}))
	mustRefuseInbox(t, s, "cmd-caster-final", inboxReport("caster-final", tex, res, true, types.CommitmentReport{StopReason: "x"}))
	mustRefuseInbox(t, s, "cmd-final-no-reason", inboxReport("final-no-reason", res, tex, true, types.CommitmentReport{}))
	open, err := s.OpenRounds(ctx, inboxTestOwner, inboxTestComputer, res)
	if err != nil || len(open) != 1 || open[0].Round != 1 || open[0].CasterInbox != tex {
		t.Fatalf("open rounds = %+v, %v", open, err)
	}

	closed := mustCommitInbox(t, s, "cmd-final-1", nil, inboxReport("final-1", res, tex, true, types.CommitmentReport{StopReason: "answered"}))
	if r := closed.Records[0]; r.Round != 1 || !r.ClosedRound {
		t.Fatalf("final report receipt = %+v", r)
	}
	if after := mustCommitInbox(t, s, "cmd-late-note", nil, inboxNote("late-note", res, tex)); after.Records[0].Round != 0 {
		t.Fatalf("note outside a round stamped round %d", after.Records[0].Round)
	}
	mustRefuseInbox(t, s, "cmd-second-final", inboxReport("second-final", res, tex, true, types.CommitmentReport{StopReason: "again"}))

	// The next Cast opens round 2; a closure naming round 1 is refused, the
	// round-2 system closure closes it and wakes no one.
	if next := mustCommitInbox(t, s, "cmd-cast-2", nil, inboxCast("cast-2", tex, res, types.CommitmentCast{})); next.Records[0].Round != 2 || !next.Records[0].OpenedRound {
		t.Fatalf("cast after close = %+v", next.Records[0])
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	mustRefuseInbox(t, s, "cmd-stale-closure", types.NewRoundClosureRecord(tex, res, 1, types.ClosureReasonInterruptedByRestart, now))
	closure := types.NewRoundClosureRecord(tex, res, 2, types.ClosureReasonInterruptedByRestart, now)
	if closure.RecordID != conv+":r2:restart" {
		t.Fatalf("closure id = %q", closure.RecordID)
	}
	got := mustCommitInbox(t, s, "cmd-closure-2", nil, closure)
	if r := got.Records[0]; !r.ClosedRound || r.Round != 2 || r.Inbox != tex || r.WakeID != "" {
		t.Fatalf("closure receipt = %+v", r)
	}
	if open, err := s.OpenRounds(ctx, inboxTestOwner, inboxTestComputer, tex); err != nil || len(open) != 0 {
		t.Fatalf("open rounds after closure = %+v, %v", open, err)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestInboxCastRacingFinalReportNeverLandsInClosedRound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		tex, res := fmt.Sprintf("texture:race-%d", i), fmt.Sprintf("research:race-%d", i)
		mustCommitInbox(t, s, fmt.Sprintf("cmd-open-%d", i), nil, inboxCast(fmt.Sprintf("open-%d", i), tex, res, types.CommitmentCast{}))
		var wg sync.WaitGroup
		var castReceipt, finalReceipt types.InboxCommitReceipt
		var castErr, finalErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			castReceipt, castErr = commitInbox(t, s, fmt.Sprintf("cmd-race-cast-%d", i), nil, inboxCast(fmt.Sprintf("race-cast-%d", i), tex, res, types.CommitmentCast{}))
		}()
		go func() {
			defer wg.Done()
			finalReceipt, finalErr = commitInbox(t, s, fmt.Sprintf("cmd-race-final-%d", i), nil, inboxReport(fmt.Sprintf("race-final-%d", i), res, tex, true, types.CommitmentReport{StopReason: "done"}))
		}()
		wg.Wait()
		if castErr != nil || finalErr != nil {
			t.Fatalf("race %d: cast %v, final %v", i, castErr, finalErr)
		}
		state, err := s.ConversationState(ctx, inboxTestOwner, inboxTestComputer, types.InboxConversationID(tex, res))
		if err != nil {
			t.Fatal(err)
		}
		switch castRound := castReceipt.Records[0].Round; castRound {
		case 1: // Cast first: it joined round 1, which the Report then closed.
			if state.Round != 1 || state.Open || !finalReceipt.Records[0].ClosedRound {
				t.Fatalf("race %d cast in round 1, state %+v", i, state)
			}
		case 2: // Report first: the Cast opened round 2.
			if state.Round != 2 || !state.Open || !castReceipt.Records[0].OpenedRound {
				t.Fatalf("race %d cast in round 2, state %+v", i, state)
			}
		default:
			t.Fatalf("race %d cast stamped round %d", i, castRound)
		}
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestInboxHypothesisIdsAndStates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	tex, res := "texture:hyp", "research:hyp"
	conv := types.InboxConversationID(tex, res)
	general := types.CastHypothesis{Statement: "there are important things I haven't mentioned", Probability: 0.5,
		General: "omission", Meaning: "a finding that changes the lede"}
	first := mustCommitInbox(t, s, "cmd-hyp-cast-1", nil, inboxCast("hyp-cast-1", tex, res, types.CommitmentCast{
		Hypotheses:    []types.CastHypothesis{hyp("h one", 0.7), hyp("h two", 0.4), hyp("h three", 0.3), general},
		Uncertainties: []types.CastUncertainty{{Question: "who funded it?", Stakes: "high"}},
	}))
	if ids := strings.Join(first.Records[0].HypothesisIDs, ","); ids != "H1,H2,H3,H4" {
		t.Fatalf("assigned ids = %s", ids)
	}
	// Malformed hypothesis bodies and caller-chosen ids are refused.
	mustRefuseInbox(t, s, "cmd-hyp-bad-p", inboxCast("hyp-bad-p", tex, res, types.CommitmentCast{Hypotheses: []types.CastHypothesis{hyp("p", 1.5)}}))
	mustRefuseInbox(t, s, "cmd-hyp-own-id", inboxCast("hyp-own-id", tex, res, types.CommitmentCast{Hypotheses: []types.CastHypothesis{{ID: "H7", Statement: "x", Probability: 0.1}}}))
	mustRefuseInbox(t, s, "cmd-hyp-unknown-finding", inboxReport("hyp-unknown-finding", res, tex, false, types.CommitmentReport{Findings: []types.ReportFinding{{Text: "x", BearsOn: []string{"H9"}}}}))
	// The opener may not settle; the recipient may not withdraw.
	mustRefuseInbox(t, s, "cmd-hyp-opener-settles", inboxReport("hyp-opener-settles", tex, res, false, types.CommitmentReport{Settled: []types.HypothesisSettlement{{ID: "H1", Outcome: "confirmed"}}}))
	mustRefuseInbox(t, s, "cmd-hyp-recipient-withdraws", inboxCast("hyp-recipient-withdraws", res, tex, types.CommitmentCast{Withdrawals: []types.CastWithdrawal{{ID: "H1"}}}))

	mustCommitInbox(t, s, "cmd-hyp-final-1", nil, inboxReport("hyp-final-1", res, tex, true, types.CommitmentReport{
		Findings:   []types.ReportFinding{{Text: "a filing", BearsOn: []string{"H1"}}, {Text: "a side note", Outside: true}},
		Settled:    []types.HypothesisSettlement{{ID: "H1", Outcome: "confirmed"}},
		Dropped:    []types.HypothesisDrop{{ID: "H2", Reason: "no source reachable"}},
		StopReason: "H1 settled",
	}))
	// Settle or drop at most once; unknown ids are refused.
	mustCommitInbox(t, s, "cmd-hyp-cast-2", nil, inboxCast("hyp-cast-2", tex, res, types.CommitmentCast{}))
	mustRefuseInbox(t, s, "cmd-hyp-resettle", inboxReport("hyp-resettle", res, tex, false, types.CommitmentReport{Settled: []types.HypothesisSettlement{{ID: "H1", Outcome: "again"}}}))
	mustRefuseInbox(t, s, "cmd-hyp-redrop", inboxReport("hyp-redrop", res, tex, false, types.CommitmentReport{Dropped: []types.HypothesisDrop{{ID: "H2", Reason: "again"}}}))
	p := 0.9
	mustRefuseInbox(t, s, "cmd-hyp-unknown-revise", inboxCast("hyp-unknown-revise", tex, res, types.CommitmentCast{Revisions: []types.CastRevision{{ID: "H9", Probability: &p}}}))
	mustRefuseInbox(t, s, "cmd-hyp-revise-settled", inboxCast("hyp-revise-settled", tex, res, types.CommitmentCast{Revisions: []types.CastRevision{{ID: "H1", Probability: &p}}}))

	// Round 2: revise H3, add H5, withdraw H4; ids stay stable.
	second := mustCommitInbox(t, s, "cmd-hyp-cast-3", nil, inboxCast("hyp-cast-3", tex, res, types.CommitmentCast{
		Hypotheses:  []types.CastHypothesis{hyp("h five", 0.2)},
		Revisions:   []types.CastRevision{{ID: "H3", Probability: &p, Reason: "new filing"}},
		Withdrawals: []types.CastWithdrawal{{ID: "H4", Reason: "covered"}},
	}))
	if r := second.Records[0]; r.Round != 2 || strings.Join(r.HypothesisIDs, ",") != "H5" {
		t.Fatalf("round-2 cast receipt = %+v", r)
	}
	states, err := s.HypothesisStates(ctx, inboxTestOwner, inboxTestComputer, conv)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]types.HypothesisStatus{"H1": types.HypothesisSettled, "H2": types.HypothesisDropped, "H3": types.HypothesisOpen,
		"H4": types.HypothesisWithdrawn, "H5": types.HypothesisOpen}
	if len(states) != len(want) {
		t.Fatalf("hypothesis states = %+v", states)
	}
	for i, state := range states {
		if wantID := fmt.Sprintf("H%d", i+1); state.ID != wantID || state.Status != want[state.ID] {
			t.Fatalf("state %d = %+v", i, state)
		}
	}
	if states[0].Outcome != "confirmed" || states[1].Reason != "no source reachable" || states[2].Probability != 0.9 ||
		states[3].General != "omission" || states[4].OpenedRound != 2 || states[0].OpenedBy != tex {
		t.Fatalf("hypothesis details = %+v", states)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestInboxCursorCompareAndSet(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	tex, mgmt := "texture:cur", "management:owner"
	for i := 1; i <= 3; i++ {
		mustCommitInbox(t, s, fmt.Sprintf("cmd-cur-%d", i), nil, inboxNote(fmt.Sprintf("cur-%d", i), mgmt, tex))
	}
	unread, err := s.UnreadFor(ctx, inboxTestOwner, inboxTestComputer, tex)
	if err != nil || len(unread) != 1 || unread[0].Head != 3 || unread[0].Cursor != 0 {
		t.Fatalf("unread before = %+v, %v", unread, err)
	}
	advance := func(expected, seq int64) *types.InboxCursorAdvance {
		return &types.InboxCursorAdvance{ReaderAgentID: tex, Inbox: tex, ExpectedSeq: expected, Seq: seq}
	}
	mustCommitInbox(t, s, "cmd-cursor-0-2", advance(0, 2))
	if _, err := commitInbox(t, s, "cmd-cursor-stale", advance(0, 3)); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale cursor: want ErrConcurrentStateChange, got %v", err)
	}
	if _, err := commitInbox(t, s, "cmd-cursor-past-head", advance(2, 4)); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("cursor past head: want ErrInboxRefused, got %v", err)
	}
	if _, err := commitInbox(t, s, "cmd-cursor-backward", advance(2, 1)); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("backward cursor: want ErrInboxRefused, got %v", err)
	}
	// The reader's own work and its cursor commit together.
	receipt := mustCommitInbox(t, s, "cmd-cursor-with-work", advance(2, 3), inboxNote("cur-answer", tex, mgmt))
	if receipt.Cursor == nil || receipt.Cursor.Seq != 3 || len(receipt.Records) != 1 {
		t.Fatalf("cursor-with-work receipt = %+v", receipt)
	}
	if cursor, err := s.ReaderCursor(ctx, inboxTestOwner, inboxTestComputer, tex, tex); err != nil || cursor != 3 {
		t.Fatalf("cursor = %d, %v", cursor, err)
	}
	if unread, err := s.UnreadFor(ctx, inboxTestOwner, inboxTestComputer, tex); err != nil || len(unread) != 0 {
		t.Fatalf("unread after = %+v, %v", unread, err)
	}
	// A cursor may never pass its head even when the head is from this commit.
	if _, err := commitInbox(t, s, "cmd-cursor-mgmt-ahead", &types.InboxCursorAdvance{ReaderAgentID: mgmt, Inbox: mgmt, Seq: 3}, inboxNote("cur-one-more", tex, mgmt)); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("cursor ahead of same-commit head: want ErrInboxRefused, got %v", err)
	}
	mustCommitInbox(t, s, "cmd-cursor-mgmt", &types.InboxCursorAdvance{ReaderAgentID: mgmt, Inbox: mgmt, Seq: 2}, inboxNote("cur-two-more", tex, mgmt))
	assertInboxHeadsMatchRebuild(t, s)
}

func TestInboxCommandReplayIsIdempotent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	rec := inboxCast("replay-cast", "texture:rp", "research:rp", types.CommitmentCast{Hypotheses: []types.CastHypothesis{hyp("a", 0.5)}})
	first := mustCommitInbox(t, s, "cmd-replay", nil, rec)
	rec.Provenance.CommittedAt = time.Now().Add(time.Second).UTC().Format(time.RFC3339Nano) // wall time is not identity
	second := mustCommitInbox(t, s, "cmd-replay", nil, rec)
	if !second.Replay || second.Records[0].InboxSeq != first.Records[0].InboxSeq ||
		strings.Join(second.Records[0].HypothesisIDs, ",") != "H1" {
		t.Fatalf("replay receipt = %+v, first %+v", second, first)
	}
	if head, _ := s.InboxHead(ctx, inboxTestOwner, inboxTestComputer, "research:rp"); head != 1 {
		t.Fatalf("replay advanced head to %d", head)
	}
	if _, err := commitInbox(t, s, "cmd-replay", nil, inboxNote("other", "texture:rp", "research:rp")); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("different command under same id: want ErrLifecycleCommandConflict, got %v", err)
	}
	if _, err := commitInbox(t, s, "cmd-replay-dup-record", nil, rec); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("same record under a new command: want ErrInboxRefused, got %v", err)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestInboxWakeRowPerAddressedRecord(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	receipt := mustCommitInbox(t, s, "cmd-wake", nil,
		inboxCast("wake-cast", "management:owner", "engineering:doc", types.CommitmentCast{}),
		inboxNote("wake-note", "management:owner", "engineering:doc"))
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]ActorWakeOutbox{}
	for _, wake := range wakes {
		byID[wake.CanonicalID] = wake
	}
	for _, r := range receipt.Records {
		wantID, err := lifecycleCanonicalID(ogKindActorWakeOutbox, inboxTestOwner, inboxTestComputer, inboxWakeKey(r.Inbox, r.InboxSeq))
		if err != nil {
			t.Fatal(err)
		}
		wake, ok := byID[wantID]
		if !ok || r.WakeID != wantID {
			t.Fatalf("no wake row keyed (inbox, seq) for %+v", r)
		}
		var content inboxWakeContent
		if wake.Kind != InboxRecordWakeKind || wake.TargetAgentID != "engineering:doc" || wake.AgentID != "management:owner" ||
			json.Unmarshal([]byte(wake.Content), &content) != nil || content.InboxSeq != r.InboxSeq || content.RecordID != r.RecordID {
			t.Fatalf("wake row = %+v", wake)
		}
	}
	if len(wakes) != 2 {
		t.Fatalf("wake rows = %d, want 2", len(wakes))
	}
	// A system closure wakes no one.
	mustCommitInbox(t, s, "cmd-wake-closure", nil, types.NewRoundClosureRecord("management:owner", "engineering:doc", 1,
		types.ClosureReasonInterruptedByRestart, time.Now().UTC().Format(time.RFC3339Nano)))
	if wakes, _ := s.ListUnprojectedActorWakes(ctx); len(wakes) != 2 {
		t.Fatalf("closure minted a wake: %d rows", len(wakes))
	}
}

func TestInboxPlanComposesIntoBatchAllOrNothing(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	mustCommitInbox(t, s, "cmd-compose-seed", nil, inboxNote("compose-seed", "research:cmp", "texture:cmp"))

	// A caller's transaction: one unrelated object plus the planned records
	// and cursor advance.
	now := time.Now().UTC()
	unrelated, err := lifecycleObject(ogKindMigrationMarker, inboxTestOwner, inboxTestComputer, "compose-unrelated", map[string]string{"x": "y"}, map[string]any{}, now, now)
	if err != nil {
		t.Fatal(err)
	}
	objects, conditions, receipt, err := s.planInboxRecords(ctx, inboxTestOwner, inboxTestComputer,
		[]types.CommitmentRecord{inboxNote("compose-planned", "research:cmp", "texture:cmp")},
		&types.InboxCursorAdvance{ReaderAgentID: "texture:cmp", Inbox: "texture:cmp", Seq: 1}, now)
	if err != nil || receipt.Records[0].InboxSeq != 2 {
		t.Fatalf("plan: %+v, %v", receipt, err)
	}
	objects = append(objects, unrelated)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: unrelated.CanonicalID})

	// Another commit takes seq 2 first; the planned batch must fail whole.
	mustCommitInbox(t, s, "cmd-compose-race", nil, inboxNote("compose-race", "management:owner", "texture:cmp"))
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects}); !errors.Is(err, objectgraph.ErrConflict) {
		t.Fatalf("composed batch after head moved: want ErrConflict, got %v", err)
	}
	if _, err := s.ogStore.GetObject(ctx, unrelated.CanonicalID); !errors.Is(err, objectgraph.ErrNotFound) {
		t.Fatalf("unrelated object written by a failed batch: %v", err)
	}
	if exists, _ := s.CommitmentRecordExists(ctx, inboxTestOwner, inboxTestComputer, "compose-planned"); exists {
		t.Fatal("planned record written by a failed batch")
	}
	if cursor, _ := s.ReaderCursor(ctx, inboxTestOwner, inboxTestComputer, "texture:cmp", "texture:cmp"); cursor != 0 {
		t.Fatalf("cursor advanced by a failed batch: %d", cursor)
	}

	// Re-planned against the new head, the same batch commits whole.
	objects, conditions, receipt, err = s.planInboxRecords(ctx, inboxTestOwner, inboxTestComputer,
		[]types.CommitmentRecord{inboxNote("compose-planned", "research:cmp", "texture:cmp")},
		&types.InboxCursorAdvance{ReaderAgentID: "texture:cmp", Inbox: "texture:cmp", Seq: 3}, now)
	if err != nil || receipt.Records[0].InboxSeq != 3 {
		t.Fatalf("re-plan: %+v, %v", receipt, err)
	}
	objects = append(objects, unrelated)
	conditions = append(conditions, objectgraph.ObjectCondition{CanonicalID: unrelated.CanonicalID})
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects}); err != nil {
		t.Fatalf("re-planned batch: %v", err)
	}
	if _, err := s.ogStore.GetObject(ctx, unrelated.CanonicalID); err != nil {
		t.Fatalf("unrelated object missing after commit: %v", err)
	}
	assertInboxHeadsMatchRebuild(t, s)
}
