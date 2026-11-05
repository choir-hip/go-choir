package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/final-report-strands-unread-cast-2026-11-05.md. Ways the
// check could fail, written before the code:
//   - U1 a final Report closes a round over a Cast its recipient has not read;
//   - U2 the refusal does not name the round and the Cast, so the model cannot
//     read and retry;
//   - U3 an interim Report is refused (it must be unaffected);
//   - U4 the final Report is refused after the recipient read the Cast, or in
//     the same commit that advances the cursor over it;
//   - U5 a system closure (run end, crash boot) is refused: it closes on
//     purpose and must always commit;
//   - U6 the check scans the whole inbox instead of the unread range, or
//     applies on replay: replay folds records and never runs it.
func TestFinalReportRefusedOverUnreadCast(t *testing.T) {
	s := openTestStore(t)
	tex, res := "texture:unread", "research:unread"
	mustCommitInbox(t, s, "u-cast-1", nil, inboxCast("u-cast-1", tex, res, types.CommitmentCast{}))
	// The recipient reads Cast #1 (cursor 1); Cast #2 joins the open round.
	mustCommitInbox(t, s, "u-read-1", &types.InboxCursorAdvance{ReaderAgentID: res, Inbox: res, ExpectedSeq: 0, Seq: 1})
	mustCommitInbox(t, s, "u-cast-2", nil, inboxCast("u-cast-2", tex, res, types.CommitmentCast{}))

	// U3: an interim Report is unaffected.
	mustCommitInbox(t, s, "u-interim", nil, inboxReport("u-interim", res, tex, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "partial"}}}))

	// U1/U2: the final Report is refused and names the round and the Cast.
	_, err := commitInbox(t, s, "u-final-a", nil, inboxReport("u-final-a", res, tex, true, types.CommitmentReport{StopReason: "done"}))
	if !errors.Is(err, ErrInboxRefused) || !strings.Contains(err.Error(), "round 1 has an unread Cast (seq 2)") || !strings.Contains(err.Error(), "choir.Records()") {
		t.Fatalf("final over an unread Cast: want a refusal naming round 1 and seq 2, got %v", err)
	}
	// A cursor advance that stops short of Cast #2 still refuses.
	_, err = commitInbox(t, s, "u-final-b", &types.InboxCursorAdvance{ReaderAgentID: res, Inbox: res, ExpectedSeq: 1, Seq: 1},
		inboxReport("u-final-b", res, tex, true, types.CommitmentReport{StopReason: "done"}))
	if !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("final with a cursor short of the Cast: want a refusal, got %v", err)
	}
	if open, _ := s.OpenRoundsAddressedTo(t.Context(), inboxTestOwner, inboxTestComputer, res); len(open) != 1 {
		t.Fatalf("the round must stay open after the refusals, got %+v", open)
	}

	// U4: the same final Report commits with the cursor advance over Cast #2.
	mustCommitInbox(t, s, "u-final-c", &types.InboxCursorAdvance{ReaderAgentID: res, Inbox: res, ExpectedSeq: 1, Seq: 2},
		inboxReport("u-final-c", res, tex, true, types.CommitmentReport{StopReason: "done"}))
	if open, _ := s.OpenRoundsAddressedTo(t.Context(), inboxTestOwner, inboxTestComputer, res); len(open) != 0 {
		t.Fatalf("round still open after the final Report: %+v", open)
	}
	assertInboxHeadsMatchRebuild(t, s)
}

func TestFinalReportAfterCursorPassedCastCommits(t *testing.T) {
	s := openTestStore(t)
	tex, res := "texture:unread2", "research:unread2"
	mustCommitInbox(t, s, "v-cast-1", nil, inboxCast("v-cast-1", tex, res, types.CommitmentCast{}))
	mustCommitInbox(t, s, "v-cast-2", nil, inboxCast("v-cast-2", tex, res, types.CommitmentCast{}))
	mustCommitInbox(t, s, "v-read", &types.InboxCursorAdvance{ReaderAgentID: res, Inbox: res, ExpectedSeq: 0, Seq: 2})
	mustCommitInbox(t, s, "v-final", nil, inboxReport("v-final", res, tex, true, types.CommitmentReport{StopReason: "done"}))
	assertInboxHeadsMatchRebuild(t, s)
}

// U5: a system closure commits over an unread Cast.
func TestSystemClosureCommitsOverUnreadCast(t *testing.T) {
	s := openTestStore(t)
	tex, res := "texture:unread3", "research:unread3"
	mustCommitInbox(t, s, "w-cast", nil, inboxCast("w-cast", tex, res, types.CommitmentCast{}))
	closure := types.NewRoundClosureRecord(tex, res, 1, "run_completed", "2026-11-05T00:00:00Z")
	mustCommitInbox(t, s, "w-closure", nil, closure)
	assertInboxHeadsMatchRebuild(t, s)
}

// readThrough is the cursor advance of a recipient that has read its inbox
// through the head, as a cell that called choir.Records() commits.
func readThrough(t *testing.T, s *Store, inbox string) *types.InboxCursorAdvance {
	t.Helper()
	return &types.InboxCursorAdvance{ReaderAgentID: inbox, Inbox: inbox, ExpectedSeq: mustReaderCursor(t, s, inbox), Seq: mustInboxHead(t, s, inbox)}
}
