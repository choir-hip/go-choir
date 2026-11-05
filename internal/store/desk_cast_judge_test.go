package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The store's part in judging desk-to-desk Casts (plan 4.4, amendment
// 2026-11-05). Ways it can fail, written before the code:
//
//  1. The work is not durable before the outcome: a crash between the Cast
//     and the close leaves nothing for the boot sweep to see.
//  2. The outcome exists before the round closes (the judge would read an
//     unfinished answer), or never when it closes.
//  3. Two scorable Casts in one round mint two work records, or the second
//     batch is refused for the existing one.
//  4. A round with no scorable Cast mints judge records (cost, noise).
//  5. A replayed close re-announces, so the judge runs twice.
//  6. A restart closure is not told apart from an answer.
//  7. A management task's Cast, already scored as a sub-call, is judged
//     again on the desk plane.

type outcomeLog struct {
	mu   sync.Mutex
	keys []string
}

func (l *outcomeLog) note(_, _, key string) {
	l.mu.Lock()
	l.keys = append(l.keys, key)
	l.mu.Unlock()
}

func (l *outcomeLog) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.keys...)
}

func scorableCast(id, from, to string, p float64) types.CommitmentRecord {
	rec := inboxCast(id, from, to, types.CommitmentCast{})
	w := 0.5
	rec.PSuccess, rec.Weight, rec.Settlement = &p, &w, types.SettlementRecipient
	return rec
}

func deskJudgeRecord(t *testing.T, s *Store, id string) (types.CommitmentRecord, bool) {
	t.Helper()
	ok, err := s.CommitmentRecordExists(context.Background(), inboxTestOwner, inboxTestComputer, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return types.CommitmentRecord{}, false
	}
	rec, err := s.GetCommitmentRecord(context.Background(), inboxTestOwner, inboxTestComputer, id)
	if err != nil || rec == nil {
		t.Fatalf("record %s: %v", id, err)
	}
	return *rec, true
}

func TestDeskCastJudgeWorkWithTheCastOutcomeWithTheClose(t *testing.T) {
	s := openTestStore(t)
	log := &outcomeLog{}
	s.SetDeskCastOutcomeNotifier(log.note)
	tex, res := "texture:doc", "research:doc"
	key := types.DeskCastRoundKey(types.InboxConversationID(tex, res), 1)

	mustCommitInbox(t, s, "cmd-cast-a", nil, scorableCast("cast-a", tex, res, 0.7))
	work, ok := deskJudgeRecord(t, s, types.DeskCastJudgeWorkID(key, 0))
	if !ok || work.SubCall == nil || work.SubCall.Kind != types.SubCallJudgeWork || work.SubCall.Plane != types.SubCallPlaneDeskCast ||
		work.SubCall.FanOutID != key || work.Addressee != types.SubCallLedgerAddressee {
		t.Fatalf("no durable judge work with the Cast: %+v %v", work.SubCall, ok)
	}
	// A second scorable Cast in the round, alone and beside a third in one
	// batch: one work record, no refusal.
	mustCommitInbox(t, s, "cmd-cast-b", nil, scorableCast("cast-b", tex, res, 0.6), scorableCast("cast-c", tex, res, 0.5))
	if _, ok := deskJudgeRecord(t, s, types.DeskCastOutcomeRecordID(key)); ok {
		t.Fatal("an outcome before the round closed")
	}
	if got := log.seen(); len(got) != 0 {
		t.Fatalf("told of an outcome before the close: %v", got)
	}
	// A non-final Report does not close the round.
	mustCommitInbox(t, s, "cmd-progress", nil, inboxReport("progress", res, tex, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "partial"}}}))
	if _, ok := deskJudgeRecord(t, s, types.DeskCastOutcomeRecordID(key)); ok {
		t.Fatal("a progress Report wrote the outcome")
	}
	closeCmd := "cmd-final"
	final := inboxReport("final", res, tex, true, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "the answer"}}, StopReason: "done"})
	cursor := readThrough(t, s, res)
	mustCommitInbox(t, s, closeCmd, cursor, final)
	outcome, ok := deskJudgeRecord(t, s, types.DeskCastOutcomeRecordID(key))
	if !ok || outcome.SubCall.Kind != types.SubCallDeskOutcome || outcome.SubCall.Reason != "" || outcome.Observation.SourceRef != "final" {
		t.Fatalf("the close wrote no outcome: %+v %v", outcome, ok)
	}
	if got := log.seen(); len(got) != 1 || got[0] != key {
		t.Fatalf("outcome announcements = %v, want [%s]", got, key)
	}
	// A replay of the closing command re-mints and re-announces nothing.
	if _, err := commitInbox(t, s, closeCmd, cursor, final); err != nil {
		t.Fatalf("replay: %v", err)
	}
	if got := log.seen(); len(got) != 1 {
		t.Fatalf("a replay re-announced: %v", got)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestDeskCastJudgeNothingForUnscorableRoundsAndTasks(t *testing.T) {
	s := openTestStore(t)
	log := &outcomeLog{}
	s.SetDeskCastOutcomeNotifier(log.note)
	tex, res := "texture:doc2", "research:doc2"
	key := types.DeskCastRoundKey(types.InboxConversationID(tex, res), 1)
	// A Cast with no p_success: no judge records, whatever happens.
	mustCommitInbox(t, s, "cmd-bare", nil, inboxCast("bare", tex, res, types.CommitmentCast{}))
	mustCommitInbox(t, s, "cmd-bare-final", readThrough(t, s, res), inboxReport("bare-final", res, tex, true, types.CommitmentReport{StopReason: "done"}))
	for _, id := range []string{types.DeskCastJudgeWorkID(key, 0), types.DeskCastOutcomeRecordID(key)} {
		if _, ok := deskJudgeRecord(t, s, id); ok {
			t.Fatalf("%s written for a round with no scorable Cast", id)
		}
	}
	if got := log.seen(); len(got) != 0 {
		t.Fatalf("announced %v", got)
	}
	// A rule-settled Cast and a management task's Cast are not the judge's.
	rule := scorableCast("rule", tex, res, 0.5)
	rule.Settlement = string(types.PrecommitRuleKinds[0])
	if types.IsDeskCastScorable(rule) {
		t.Fatal("a rule-settled Cast is scorable")
	}
	task := scorableCast("task", "management:o", types.ManagementTaskDesk+":t1", 0.5)
	if types.IsDeskCastScorable(task) {
		t.Fatal("a management task's Cast is judged on the desk plane")
	}
	if !types.IsDeskCastScorable(scorableCast("ok", tex, res, 0.5)) {
		t.Fatal("a recipient-settled desk Cast is not scorable")
	}
}

func TestDeskCastJudgeRestartClosureIsMarked(t *testing.T) {
	s := openTestStore(t)
	log := &outcomeLog{}
	s.SetDeskCastOutcomeNotifier(log.note)
	tex, res := "texture:doc3", "research:doc3"
	key := types.DeskCastRoundKey(types.InboxConversationID(tex, res), 1)
	mustCommitInbox(t, s, "cmd-cast", nil, scorableCast("cast-r", tex, res, 0.8))
	closure := types.NewRoundClosureRecord(tex, res, 1, types.ClosureReasonInterruptedByRestart, time.Now().UTC().Format(time.RFC3339Nano))
	mustCommitInbox(t, s, "cmd-closure", nil, closure)
	outcome, ok := deskJudgeRecord(t, s, types.DeskCastOutcomeRecordID(key))
	if !ok || outcome.SubCall.Reason != types.ClosureReasonInterruptedByRestart {
		t.Fatalf("restart closure outcome = %+v %v", outcome.SubCall, ok)
	}
	if got := log.seen(); len(got) != 1 {
		t.Fatalf("announced %v", got)
	}
	// A never-closed round keeps its work and has no outcome.
	tex2, res2 := "texture:doc4", "research:doc4"
	key2 := types.DeskCastRoundKey(types.InboxConversationID(tex2, res2), 1)
	mustCommitInbox(t, s, "cmd-cast-open", nil, scorableCast("cast-open", tex2, res2, 0.8))
	if _, ok := deskJudgeRecord(t, s, types.DeskCastJudgeWorkID(key2, 0)); !ok {
		t.Fatal("no work for an open round")
	}
	if _, ok := deskJudgeRecord(t, s, types.DeskCastOutcomeRecordID(key2)); ok {
		t.Fatal("an open round has an outcome")
	}
}
