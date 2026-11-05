package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The judge settles desk-to-desk Casts (plan 4.4, amendment 2026-11-05).
// Ways it can fail, written before the code:
//
//  1. It settles a Cast before the round's answer exists, or never settles
//     one whose round closed with an answer.
//  2. It settles twice: a retry, a second enqueue, a planned resume after
//     the settlement landed, or a party (the recipient) settling too.
//  3. It settles a Cast the recipient never answered, as if silence were
//     a contradiction.
//  4. A crash with judging in flight re-runs the judge at boot (crash
//     restarts never resume work), or leaves the work open forever.
//  5. It is not blind: the sender's p_success or its identity reach the
//     judge.
//  6. It costs a call per Cast: 17 Casts in one round must be one call.
//  7. The read surface cannot tell desk-to-desk settlements apart.

const deskTex, deskRes = "texture:doc-dj", "research:doc-dj"

func deskJudgeFixture(t *testing.T) *subCallFixture {
	t.Helper()
	old := deskCastJudgeCoalesce
	deskCastJudgeCoalesce = 0
	t.Cleanup(func() { deskCastJudgeCoalesce = old })
	return newSubCallFixture(t, nil)
}

func (f *subCallFixture) commitInbox(t *testing.T, commandID string, cursor *types.InboxCursorAdvance, recs ...types.CommitmentRecord) {
	t.Helper()
	for i := range recs {
		recs[i].ComputerID = f.computer
	}
	req := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: commandID, Records: recs, Cursor: cursor}
	var err error
	if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
		t.Fatal(err)
	}
	if _, err := f.s.CommitInboxRecords(context.Background(), req); err != nil {
		t.Fatalf("commit %s: %v", commandID, err)
	}
}

// readAll is the recipient's read through its inbox head, so its final
// Report is not refused for an unread Cast.
func (f *subCallFixture) readAll(t *testing.T, inbox string) *types.InboxCursorAdvance {
	t.Helper()
	ctx := context.Background()
	cur, err := f.s.ReaderCursor(ctx, f.owner, f.computer, inbox, inbox)
	if err != nil {
		t.Fatal(err)
	}
	head, err := f.s.InboxHead(ctx, f.owner, f.computer, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return &types.InboxCursorAdvance{ReaderAgentID: inbox, Inbox: inbox, ExpectedSeq: cur, Seq: head}
}

func deskRecord(id, from, to string) types.CommitmentRecord {
	return types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: from, Desk: from, ModelID: "sender-model", CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterAgent, Inbox: to, SenderInbox: from}
}

func deskCast(id, from, to, objective string, p float64) types.CommitmentRecord {
	rec := deskRecord(id, from, to)
	rec.Kind = types.CommitmentKindDirective
	rec.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: objective, Body: objective}
	rec.Cast = &types.CommitmentCast{}
	w := 0.4
	rec.PSuccess, rec.Weight, rec.Settlement = &p, &w, types.SettlementRecipient
	return rec
}

func deskReport(id, from, to, text string, final bool) types.CommitmentRecord {
	rec := deskRecord(id, from, to)
	rec.Kind = types.CommitmentKindReport
	rec.Final = final
	rec.Report = &types.CommitmentReport{Claims: []types.ReportClaim{{Text: text}}}
	if final {
		rec.Report.StopReason = "done"
	}
	return rec
}

func (f *subCallFixture) deskKey(round int) string {
	return types.DeskCastRoundKey(types.InboxConversationID(deskTex, deskRes), round)
}

func (f *subCallFixture) deskClose(t *testing.T, key string, attempt int) (types.CommitmentRecord, bool) {
	t.Helper()
	id := types.SubCallJudgeCloseRecordID(types.DeskCastJudgeWorkID(key, attempt))
	if !f.exists(id) {
		return types.CommitmentRecord{}, false
	}
	return f.record(t, id), true
}

func judgeSettlementID(castID string) string {
	return types.SubCallSettlementRecordID(castID, types.SubCallJudgeScorerID)
}

func TestDeskCastAnsweredByAFinalReportIsSettledByTheJudge(t *testing.T) {
	f := deskJudgeFixture(t)
	f.commitInbox(t, "dj-cast", nil, deskCast("dj-cast-1", deskTex, deskRes, "Find the primary filing for the merger.", 0.8))
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 0 {
		t.Fatal("the judge ran before the round closed")
	}
	f.commitInbox(t, "dj-final", f.readAll(t, deskRes), deskReport("dj-final-1", deskRes, deskTex, "The filing is at https://example.test/filing.", true))
	f.rt.waitSubCallJudge()
	if n := f.judge.callCount(); n != 1 {
		t.Fatalf("judge calls = %d, want 1", n)
	}
	st := f.record(t, judgeSettlementID("dj-cast-1"))
	b := st.SubCall
	if b == nil || b.Kind != types.SubCallSettlement || b.Plane != types.SubCallPlaneDeskCast || b.Method != types.CalibrationScorerIndependent ||
		b.CastID != "dj-cast-1" || b.Class != types.DiscrepancyConfirmed || len(b.Verdicts) == 0 || st.ParentID != "dj-cast-1" {
		t.Fatalf("settlement = %+v", b)
	}
	// Blind: the report and the objective reach the judge; the sender's
	// probability and identities do not.
	asked, _ := json.Marshal(f.judge.asked[0])
	state := f.judge.states[0] + string(asked)
	if !strings.Contains(state, "primary filing") || !strings.Contains(state, "example.test/filing") {
		t.Fatalf("the judge did not see the objective and the answer: %s", state)
	}
	for _, leak := range []string{"0.8", deskTex, deskRes, "dj-cast-1"} {
		if strings.Contains(state, leak) {
			t.Fatalf("the judge request carries %q: %s", leak, state)
		}
	}
	cl, ok := f.deskClose(t, f.deskKey(1), 0)
	if !ok || cl.SubCall.Phase != types.SubCallPhaseAnswered || cl.SubCall.InputTokens == 0 {
		t.Fatalf("work close = %+v %v", cl.SubCall, ok)
	}
	// The read surface tells the planes apart.
	code, resp := getSubCalls(t, &APIHandler{rt: f.rt}, f.owner, "")
	if code != http.StatusOK || resp.Summary.ByPlane[types.SubCallPlaneDeskCast][string(types.DiscrepancyConfirmed)] != 1 ||
		resp.Summary.ByClass[string(types.DiscrepancyConfirmed)] != 1 {
		t.Fatalf("read surface: %d %+v", code, resp.Summary)
	}
}

func TestDeskCastDeclinedIsSettledAsNotAchieved(t *testing.T) {
	f := deskJudgeFixture(t)
	f.judge.pick = map[string]string{"c0.q0": "no"}
	f.judge.answered = "not_answered"
	f.commitInbox(t, "dj-cast", nil, deskCast("dj-cast-d", deskTex, deskRes, "Read the paywalled article.", 0.9))
	decline := deskRecord("dj-decline", deskRes, deskTex)
	decline.Kind = types.CommitmentKindDirective
	decline.Directive = &types.CommitmentDirective{Subtype: types.CommitmentDirectiveDecline, Body: "I cannot reach paywalled sources."}
	f.commitInbox(t, "dj-decline", f.readAll(t, deskRes), decline)
	// The recipient's run ends: the run-end closure closes the round.
	closure := types.NewRoundClosureRecord(deskTex, deskRes, 1, "run_completed", time.Now().UTC().Format(time.RFC3339Nano))
	closure.ComputerID = f.computer
	f.commitInbox(t, "dj-closure", nil, closure)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 || !strings.Contains(f.judge.states[0], "declined") {
		t.Fatalf("judge calls %d; the decline did not reach it", f.judge.callCount())
	}
	b := f.record(t, judgeSettlementID("dj-cast-d")).SubCall
	if b.Class != types.DiscrepancyContradicted || !b.Contradicted {
		t.Fatalf("a decline of a p=0.9 Cast settled %+v", b)
	}
}

func TestDeskCastNeverAnsweredIsNotSettled(t *testing.T) {
	f := deskJudgeFixture(t)
	// (1) The round never closes: no judge, the work stays open.
	f.commitInbox(t, "dj-open", nil, deskCast("dj-cast-open", deskTex, deskRes, "Find X.", 0.7))
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 0 || f.exists(judgeSettlementID("dj-cast-open")) {
		t.Fatal("an open round was judged")
	}
	if _, closed := f.deskClose(t, f.deskKey(1), 0); closed {
		t.Fatal("an open round's work was closed")
	}
	// (2) The round closes with no word from the recipient.
	closure := types.NewRoundClosureRecord(deskTex, deskRes, 1, "run_failed", time.Now().UTC().Format(time.RFC3339Nano))
	closure.ComputerID = f.computer
	f.commitInbox(t, "dj-silent-close", nil, closure)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 0 || f.exists(judgeSettlementID("dj-cast-open")) {
		t.Fatal("silence was judged")
	}
	cl, ok := f.deskClose(t, f.deskKey(1), 0)
	if !ok || !strings.Contains(cl.SubCall.Reason, "never answered") {
		t.Fatalf("work close = %+v %v", cl.SubCall, ok)
	}
}

func TestDeskCastRetryDoesNotDoubleSettle(t *testing.T) {
	f := deskJudgeFixture(t)
	// Seventeen Casts in one round are one call.
	var casts []types.CommitmentRecord
	for i := range 17 {
		casts = append(casts, deskCast("dj-many-"+string(rune('a'+i)), deskTex, deskRes, "Question "+string(rune('a'+i))+".", 0.6))
	}
	f.commitInbox(t, "dj-many", nil, casts...)
	f.commitInbox(t, "dj-many-final", f.readAll(t, deskRes), deskReport("dj-many-final", deskRes, deskTex, "Answers to every question.", true))
	f.rt.waitSubCallJudge()
	if n := f.judge.callCount(); n != 1 {
		t.Fatalf("17 Casts took %d judge calls, want 1", n)
	}
	key := f.deskKey(1)
	f.rt.enqueueDeskCastJudge(f.owner, f.computer, key, 0, 0)
	f.rt.enqueueDeskCastJudge(f.owner, f.computer, key, 0, 0)
	f.rt.waitSubCallJudge()
	// A planned resume of the same round under a new attempt finds every
	// Cast settled and calls nobody.
	f.rt.enqueueDeskCastJudge(f.owner, f.computer, key, 0, 1)
	f.rt.waitSubCallJudge()
	if n := f.judge.callCount(); n != 1 {
		t.Fatalf("a retry called the judge again: %d calls", n)
	}
	recs, err := f.s.ListSubCallRecords(context.Background(), f.owner, f.computer, types.SubCallSettlement, 500)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, r := range recs {
		if r.SubCall.Method == types.CalibrationScorerIndependent {
			seen[r.SubCall.CastID]++
		}
	}
	if len(seen) != 17 {
		t.Fatalf("settled %d Casts, want 17", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("%s settled %d times", id, n)
		}
	}
	// A party to the Cast cannot settle it.
	cast := f.record(t, "dj-many-a")
	for _, party := range []string{deskRes, deskTex} {
		if err := checkDeskCastSettler(cast, party); err == nil {
			t.Fatalf("%s may settle its own Cast", party)
		}
	}
}

func TestDeskCastRestartWithJudgingInFlightClosesIt(t *testing.T) {
	f := deskJudgeFixture(t)
	ctx := context.Background()
	// The computer dies after the round closed and before the judge ran: the
	// runtime was never told.
	f.s.SetDeskCastOutcomeNotifier(nil)
	f.commitInbox(t, "dj-crash-cast", nil, deskCast("dj-crash", deskTex, deskRes, "Find Y.", 0.7))
	f.commitInbox(t, "dj-crash-final", f.readAll(t, deskRes), deskReport("dj-crash-final", deskRes, deskTex, "Y is here.", true))
	f.rt.sweepSubCalls(ctx, f.owner, f.computer, false)
	f.rt.waitSubCallJudge()
	cl, ok := f.deskClose(t, f.deskKey(1), 0)
	if !ok || cl.SubCall.Phase != types.SubCallPhaseInterrupted || !strings.Contains(cl.SubCall.Reason, "interrupted by a restart") {
		t.Fatalf("crash boot close = %+v %v", cl.SubCall, ok)
	}
	if f.judge.callCount() != 0 || f.exists(judgeSettlementID("dj-crash")) {
		t.Fatal("a crash boot ran the judge")
	}
	f.rt.sweepSubCalls(ctx, f.owner, f.computer, false)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 0 {
		t.Fatal("a second crash boot ran the judge")
	}

	// A planned restart re-runs it once, under a new attempt.
	f.commitInbox(t, "dj-plan-cast", nil, deskCast("dj-plan", deskTex, deskRes, "Find Z.", 0.7))
	f.commitInbox(t, "dj-plan-final", f.readAll(t, deskRes), deskReport("dj-plan-final", deskRes, deskTex, "Z is here.", true))
	f.rt.sweepSubCalls(ctx, f.owner, f.computer, true)
	f.rt.waitSubCallJudge()
	if !f.exists(judgeSettlementID("dj-plan")) || f.judge.callCount() != 1 {
		t.Fatalf("planned resume: settled %v, calls %d", f.exists(judgeSettlementID("dj-plan")), f.judge.callCount())
	}
	if old, ok := f.deskClose(t, f.deskKey(2), 0); !ok || old.SubCall.Phase != types.SubCallPhaseInterrupted {
		t.Fatalf("stale work not superseded: %+v", old.SubCall)
	}
	if again, ok := f.deskClose(t, f.deskKey(2), 1); !ok || again.SubCall.Phase != types.SubCallPhaseAnswered {
		t.Fatalf("resume's work not closed: %+v", again.SubCall)
	}
	f.rt.sweepSubCalls(ctx, f.owner, f.computer, false)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 {
		t.Fatal("the boot after the resume ran the judge again")
	}

	// A round the crash closure itself closed has no observation.
	f.s.SetDeskCastOutcomeNotifier(f.rt.noteDeskCastOutcome)
	f.commitInbox(t, "dj-rs-cast", nil, deskCast("dj-rs", deskTex, deskRes, "Find W.", 0.7))
	closure := types.NewRoundClosureRecord(deskTex, deskRes, 3, types.ClosureReasonInterruptedByRestart, time.Now().UTC().Format(time.RFC3339Nano))
	closure.ComputerID = f.computer
	f.commitInbox(t, "dj-rs-closure", nil, closure)
	f.rt.waitSubCallJudge()
	if cl, ok := f.deskClose(t, f.deskKey(3), 0); !ok || cl.SubCall.Phase != types.SubCallPhaseInterrupted {
		t.Fatalf("restart-closed round: %+v %v", cl.SubCall, ok)
	}
	if f.judge.callCount() != 1 {
		t.Fatal("a restart-closed round was judged")
	}
}

// Stop cancels the coalesce sleep and joins; the work stays open for the
// next boot.
func TestDeskCastStopJoinsTheJudge(t *testing.T) {
	f := deskJudgeFixture(t)
	f.s.SetDeskCastOutcomeNotifier(nil)
	f.commitInbox(t, "dj-stop-cast", nil, deskCast("dj-stop", deskTex, deskRes, "Find V.", 0.7))
	f.commitInbox(t, "dj-stop-final", f.readAll(t, deskRes), deskReport("dj-stop-final", deskRes, deskTex, "V.", true))
	f.rt.enqueueDeskCastJudge(f.owner, f.computer, f.deskKey(1), time.Hour, 0)
	done := make(chan struct{})
	go func() { f.rt.stopSubCallJudge(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop waited out the coalesce sleep")
	}
	if f.judge.callCount() != 0 {
		t.Fatal("a stopped judge ran")
	}
	if _, closed := f.deskClose(t, f.deskKey(1), 0); closed {
		t.Fatal("Stop closed the work; it must stay open for the next boot")
	}
}

// Devin r1, finding 1: a desk's own final Report cannot forge the host's
// restart fate. Only the system closure record makes a round restart-closed.
func TestDeskCastForgedRestartStopReasonIsStillJudged(t *testing.T) {
	f := deskJudgeFixture(t)
	f.commitInbox(t, "dj-forge-cast", nil, deskCast("dj-forge-1", deskTex, deskRes, "Find the filing.", 0.8))
	final := deskReport("dj-forge-final", deskRes, deskTex, "The filing is at https://example.test/filing.", true)
	final.Report.StopReason = types.ClosureReasonInterruptedByRestart
	f.commitInbox(t, "dj-forge-final", f.readAll(t, deskRes), final)
	f.rt.waitSubCallJudge()
	if f.judge.callCount() != 1 || !f.exists(judgeSettlementID("dj-forge-1")) {
		t.Fatalf("a forged StopReason vetoed the settlement: calls %d", f.judge.callCount())
	}
	if cl, ok := f.deskClose(t, f.deskKey(1), 0); !ok || cl.SubCall.Phase != types.SubCallPhaseAnswered {
		t.Fatalf("close = %+v %v", cl.SubCall, ok)
	}
}

// Finding 2: a Cast the recipient never addressed has no evidence and is
// not asked about from its wording alone.
func TestDeskCastWithNoEvidenceIsNotJudged(t *testing.T) {
	f := deskJudgeFixture(t)
	f.commitInbox(t, "dj-ne-cast", nil, deskCast("dj-ne-1", deskTex, deskRes, "Find A.", 0.7), deskCast("dj-ne-2", deskTex, deskRes, "Find B.", 0.7))
	reply := deskReport("dj-ne-final", deskRes, deskTex, "A is at https://example.test/a.", true)
	reply.ParentID = "dj-ne-1"
	f.commitInbox(t, "dj-ne-final", f.readAll(t, deskRes), reply)
	f.rt.waitSubCallJudge()
	if !f.exists(judgeSettlementID("dj-ne-1")) || f.exists(judgeSettlementID("dj-ne-2")) {
		t.Fatalf("settled 1: %v, settled 2: %v", f.exists(judgeSettlementID("dj-ne-1")), f.exists(judgeSettlementID("dj-ne-2")))
	}
	cl, ok := f.deskClose(t, f.deskKey(1), 0)
	if !ok || !strings.Contains(cl.SubCall.Reason, "no evidence") {
		t.Fatalf("close = %+v %v", cl.SubCall, ok)
	}
}

// Finding 3: a round that could not be read is closed failed, never answered.
func TestDeskCastUnreadableRoundClosesFailed(t *testing.T) {
	f := deskJudgeFixture(t)
	ctx := context.Background()
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	closing := deskReport("dj-bad-final", deskRes, deskTex, "x", true)
	if err := f.rt.appendSubCallRecord(ctx, f.owner, f.computer, types.DeskCastOutcome(f.computer, "badkey", closing, stamp)); err != nil {
		t.Fatal(err)
	}
	f.rt.settleDeskCastRounds(ctx, f.owner, f.computer, 0, []string{"badkey"})
	cl, ok := f.deskClose(t, "badkey", 0)
	if !ok || cl.SubCall.Phase != types.SubCallPhaseFailed || !strings.Contains(cl.SubCall.Reason, "unreadable round key") {
		t.Fatalf("close = %+v %v", cl.SubCall, ok)
	}
}
