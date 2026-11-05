package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/final-report-strands-unread-cast-2026-11-05.md, the d@e
// timeline: Cast #1, Cast #2 into the open round, then the recipient's final
// Report racing Cast #2. Ways it could fail, written first:
//   - the final Report closes the round over unread Cast #2 and the caster
//     waits forever (the defect);
//   - a refused final Report leaves the round closed or closes it at the
//     run's end while the Cast is unread (the run-end closure must hold it);
//   - the net writes twice for one stranded Cast, or on a crash-closed run
//     (crash closes work, it never resumes it), or for a Cast the run
//     answered, or for an ordinary run that read its Cast and reported.
func unreadCastRecord(id, from, to string, at time.Time) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Body: id}, Cast: &types.CommitmentCast{},
		Inbox: to, SenderInbox: from, Addressee: to, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: from, CommittedAt: at.UTC().Format(time.RFC3339Nano)},
	}
}

func finalReportRecord(id, from, to string, at time.Time) types.CommitmentRecord {
	return types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{StopReason: "done"}, Final: true,
		Inbox: to, SenderInbox: from, Addressee: to, EmitterKind: types.CommitmentEmitterAgent,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: from, CommittedAt: at.UTC().Format(time.RFC3339Nano)},
	}
}

func commitUnreadCast(t *testing.T, s *store.Store, owner, computer, cmd string, cursor *types.InboxCursorAdvance, recs ...types.CommitmentRecord) (types.InboxCommitReceipt, error) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: cmd, Records: recs, Cursor: cursor}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	return s.CommitInboxRecords(context.Background(), req)
}

func TestFinalReportRacingSecondCastDoesNotStrandCaster(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-race", rt.cfg.ComputerID
	research, texture := "research:doc-race", "texture:doc-race"
	var woke []string
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		if kind == "inbox_record" {
			woke = append(woke, to)
		}
		return nil
	})
	t0 := time.Now().Add(-time.Minute)
	if _, err := commitUnreadCast(t, s, owner, computer, "race-cast-1", nil, unreadCastRecord("race-cast-1", texture, research, t0)); err != nil {
		t.Fatal(err)
	}
	// Run 1 reads Cast #1 (cursor 1) and starts composing.
	run1 := &types.RunRecord{RunID: "race-run-1", AgentID: research, OwnerID: owner, ComputerID: computer,
		AgentProfile: agentprofile.Research, AgentRole: agentprofile.Research, State: types.RunCompleted, CreatedAt: t0.Add(time.Second)}
	rt.recordsRunStart.Store(run1.RunID, int64(0))
	if _, err := commitUnreadCast(t, s, owner, computer, "race-read-1", &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: 0, Seq: 1}); err != nil {
		t.Fatal(err)
	}
	// Cast #2 joins the open round while run 1 composes.
	if _, err := commitUnreadCast(t, s, owner, computer, "race-cast-2", nil, unreadCastRecord("race-cast-2", texture, research, time.Now())); err != nil {
		t.Fatal(err)
	}
	// Its final Report is refused with a refusal the cell can read.
	_, err := commitUnreadCast(t, s, owner, computer, "race-final", nil, finalReportRecord("race-final", research, texture, time.Now()))
	if !errors.Is(err, store.ErrInboxRefused) || !strings.Contains(err.Error(), "unread Cast (seq 2)") {
		t.Fatalf("the final Report over unread Cast #2: want the unread-Cast refusal, got %v", err)
	}
	// Run 1 ends without a final Report: Cast #2 arrived after it began and
	// is unread, so the round stays open for the next run.
	rt.closeRoundsOfEndedRun(ctx, run1)
	conversation := types.InboxConversationID(texture, research)
	state, err := s.ConversationState(ctx, owner, computer, conversation)
	if err != nil || !state.Open {
		t.Fatalf("round after run 1's end: open=%v err=%v; want it held open for the next run", state.Open, err)
	}
	if len(woke) != 0 {
		t.Fatalf("a held round woke someone: %v", woke)
	}
	// Run 2 reads Cast #2 and reports: the caster is woken by the Report.
	rec, err := commitUnreadCast(t, s, owner, computer, "race-final-2", &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: 1, Seq: 2},
		finalReportRecord("race-final-2", research, texture, time.Now()))
	if err != nil {
		t.Fatalf("final Report after reading Cast #2: %v", err)
	}
	if len(rec.Records) != 1 || rec.Records[0].WakeID == "" || !rec.Records[0].ClosedRound {
		t.Fatalf("the final Report must close the round and wake the caster: %+v", rec.Records)
	}
}

// The net: a tape written before the store check holds a Cast inside a
// round a desk's final Report closed first (here built by committing the
// Cast and the Report in one batch, which the check cannot see).
func TestRunReadingCastOfClosedRoundWakesCasterOnce(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-net", rt.cfg.ComputerID
	research, texture := "research:doc-net", "texture:doc-net"
	var woke []string
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		if kind == "inbox_record" {
			woke = append(woke, to)
		}
		return nil
	})
	t0 := time.Now().Add(-time.Minute)
	if _, err := commitUnreadCast(t, s, owner, computer, "net-cast-1", nil, unreadCastRecord("net-cast-1", texture, research, t0)); err != nil {
		t.Fatal(err)
	}
	if _, err := commitUnreadCast(t, s, owner, computer, "net-read-1", &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: 0, Seq: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := commitUnreadCast(t, s, owner, computer, "net-legacy", nil,
		unreadCastRecord("net-cast-2", texture, research, t0.Add(time.Second)),
		finalReportRecord("net-final", research, texture, t0.Add(2*time.Second))); err != nil {
		t.Fatalf("legacy tape: %v", err)
	}
	ended := func(id string, errText string) *types.RunRecord {
		run := &types.RunRecord{RunID: id, AgentID: research, OwnerID: owner, ComputerID: computer, AgentProfile: agentprofile.Research,
			AgentRole: agentprofile.Research, State: types.RunCompleted, Error: errText, CreatedAt: time.Now().Add(-time.Second)}
		rt.recordsRunStart.Store(id, int64(1))
		return run
	}
	notices := func() int {
		recs, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, r := range recs {
			if r.Report != nil && r.Report.StopReason == types.ClosureReasonCastAfterRoundClosed {
				n++
			}
		}
		return n
	}
	// A crash-closed run writes and wakes nothing.
	rt.closeRoundsOfEndedRun(ctx, ended("net-crashed", runFateInterruptedByRestart))
	if notices() != 0 || len(woke) != 0 {
		t.Fatalf("a crash-closed run told the caster: notices=%d woke=%v", notices(), woke)
	}
	rt.closeRoundsOfEndedRun(ctx, ended("net-run", ""))
	if notices() != 1 || len(woke) != 1 || woke[0] != texture {
		t.Fatalf("the stranded Cast's caster was not told once: notices=%d woke=%v", notices(), woke)
	}
	// Idempotent: the next run over the same range adds nothing.
	rt.closeRoundsOfEndedRun(ctx, ended("net-run-again", ""))
	if notices() != 1 || len(woke) != 1 {
		t.Fatalf("the notice fired twice: notices=%d woke=%v", notices(), woke)
	}
}

// An ordinary run that read its Cast and sent the final Report itself gets no
// notice (the interim and final Reports of a healthy round are unaffected).
func TestOrdinaryRunEndWritesNoCastAfterCloseNotice(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	owner, computer := "owner-ok", rt.cfg.ComputerID
	research, texture := "research:doc-ok", "texture:doc-ok"
	rt.SetDispatchActor(func(_ context.Context, _, _, to, kind, _, _, _ string) error {
		t.Fatalf("an ordinary run end woke %s", to)
		return nil
	})
	t0 := time.Now().Add(-time.Minute)
	if _, err := commitUnreadCast(t, s, owner, computer, "ok-cast-1", nil, unreadCastRecord("ok-cast-1", texture, research, t0)); err != nil {
		t.Fatal(err)
	}
	run := &types.RunRecord{RunID: "ok-run", AgentID: research, OwnerID: owner, ComputerID: computer, AgentProfile: agentprofile.Research,
		AgentRole: agentprofile.Research, State: types.RunCompleted, CreatedAt: t0.Add(time.Second)}
	rt.recordsRunStart.Store(run.RunID, int64(0))
	if _, err := commitUnreadCast(t, s, owner, computer, "ok-final", &types.InboxCursorAdvance{ReaderAgentID: research, Inbox: research, ExpectedSeq: 0, Seq: 1},
		finalReportRecord("ok-final", research, texture, time.Now())); err != nil {
		t.Fatal(err)
	}
	rt.closeRoundsOfEndedRun(ctx, run)
	recs, _ := s.ListInboxRecords(ctx, owner, computer, texture, 0, 100)
	if len(recs) != 1 {
		t.Fatalf("texture inbox after an ordinary run: %d records, want only the final Report", len(recs))
	}
}
