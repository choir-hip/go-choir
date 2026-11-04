package textureowner

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// docs/problems/texture-settles-rejected-round-as-complete-2026-11-04.md.
// Ways the fix can fail, written before it:
//   - Done on a rejected round still settles it complete;
//   - the refusal commits part of the turn, or drops the reasons;
//   - with the budget spent the work item stays open or completed, with no
//     "rejected after N attempts" fate;
//   - a round that is not rejected (crash close, abandon, a pass) is refused.

func doneTurn(base string) yaegikernel.TextureTurn {
	return writeTurn(base, func(turn *yaegikernel.TextureTurn) { turn.Done = "Calendar round complete: all screens verified" })
}

func TestDoneOnARejectedRoundIsSentBackWithTheReasons(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "sendback")
	ctx := context.Background()
	h.rejectedRoundOf = func(context.Context, string, string, string) (agentcore.RejectedRound, bool) {
		return agentcore.RejectedRound{OperationID: "selfdev-1", Attempts: 2, Budget: 6, Reasons: "shot 3: clipped event"}, true
	}
	base := headOf(t, s, rec)
	_, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, doneTurn(base.RevisionID)), "done-rejected")
	if err == nil || !strings.Contains(err.Error(), "shot 3: clipped event") || !strings.Contains(err.Error(), "Cast engineering again") {
		t.Fatalf("Done on a rejected round = %v, want the send-back with the reasons", err)
	}
	if head := headOf(t, s, rec); head.RevisionID != base.RevisionID {
		t.Fatal("the refused turn moved the head")
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemOpen {
		t.Fatalf("work = %s, want open: the round is sent back, not settled", w.Status)
	}
}

func TestSpentBudgetEndsTheRoundRejectedNeverComplete(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "sendback-spent")
	ctx := context.Background()
	h.rejectedRoundOf = func(context.Context, string, string, string) (agentcore.RejectedRound, bool) {
		return agentcore.RejectedRound{OperationID: "selfdev-6", Attempts: 6, Budget: 6, Reasons: "10 of 12 shots rejected", Exhausted: true}, true
	}
	base := headOf(t, s, rec)
	_, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, doneTurn(base.RevisionID)), "done-spent")
	if err == nil || !strings.Contains(err.Error(), "rejected after 6 attempts") {
		t.Fatalf("Done on a spent round = %v, want the rejected fate", err)
	}
	w := textureWork(t, s, rec)
	if w.Status != types.WorkItemRefused || !strings.HasPrefix(w.Reason, "rejected after 6 attempts: 10 of 12 shots rejected") {
		t.Fatalf("work = %s %q, want refused with the fate", w.Status, w.Reason)
	}
	if head := headOf(t, s, rec); head.RevisionID != base.RevisionID {
		t.Fatal("the false completion's revision committed")
	}
}

func TestDoneOnARoundThatIsNotRejectedStillSettles(t *testing.T) {
	// A crash close, an owner abandon and a pass are not rejections
	// (agentcore.RejectedRoundOf reads none), so Done is not touched.
	h, s, rec := cellAuthorSetup(t, "sendback-none")
	h.rejectedRoundOf = func(context.Context, string, string, string) (agentcore.RejectedRound, bool) {
		return agentcore.RejectedRound{}, false
	}
	base := headOf(t, s, rec)
	if _, err := h.CommitCellTextureAuthor(context.Background(), &rec, turnBody(t, doneTurn(base.RevisionID)), "done-ok"); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if w := textureWork(t, s, rec); w.Status != types.WorkItemCompleted {
		t.Fatalf("work = %s, want completed", w.Status)
	}
}
