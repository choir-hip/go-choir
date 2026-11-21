package agentcore

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The feed-or-open choice, logged (search-reuse commit 4, from
// docs/problems/research-does-not-start-from-the-corpus-2026-11-18.md,
// "Remaining error"). Failure list, written before the code:
//  1. The choice management makes at a commit (a task to an existing story,
//     texture:<doc>, or texture:new) leaves no line with the run id, the
//     choice, the existing story's id when fed, and how many candidate
//     stories the run was shown.
//  2. The shown count adds a page twice, or carries one run's count into
//     another's, or is never forgotten (one entry per run, for ever).
//  3. The line carries a title or a lede (ids and counts only).
func TestStoriesShownCountsPerRunAndIsForgotten(t *testing.T) {
	rt := &Runtime{}
	rt.noteStoriesShown("run-1", 20)
	rt.noteStoriesShown("run-1", 5)
	rt.noteStoriesShown("run-2", 3)
	if got := rt.storiesShownCount("run-1"); got != 25 {
		t.Fatalf("run-1 shown = %d, want 25", got)
	}
	if got := rt.storiesShownCount("run-2"); got != 3 {
		t.Fatalf("run-2 shown = %d, want 3", got)
	}
	rt.forgetStoriesShown("run-1")
	if got := rt.storiesShownCount("run-1"); got != 0 {
		t.Fatalf("run-1 shown after forget = %d, want 0", got)
	}
}

func TestManagementStoryChoiceLine(t *testing.T) {
	if got := managementStoryChoiceLine("run-9", "feed", "doc-4", 50, 25, choiceDetail{}); got != "management story choice run=run-9 choice=feed story=doc-4 opening_shown=50 verb_shown=25 candidates_shown=75 window=- page_in_window=0 page_left_out=0 offered=- answers=- captures=0 sources=near:0,lead:0 main_point=0 succeeds=-" {
		t.Fatalf("feed line = %q", got)
	}
	if got := managementStoryChoiceLine("run-9", "open", "", 0, 0, choiceDetail{}); got != "management story choice run=run-9 choice=open story=- opening_shown=0 verb_shown=0 candidates_shown=0 window=- page_in_window=0 page_left_out=0 offered=- answers=- captures=0 sources=near:0,lead:0 main_point=0 succeeds=-" {
		t.Fatalf("open line = %q", got)
	}
}

// Defect 2 (recurring-agent-failures record, 2026-11-19): the line counted only
// choir.Stories calls, not the page the task's opening delivered.
func TestOpeningStoriesShownIsCountedSeparately(t *testing.T) {
	rt := &Runtime{}
	rt.noteOpeningOffer("run-1", &openingOffer{Shown: 50})
	rt.noteOpeningOffer("run-2", nil)
	rt.noteStoriesShown("run-1", 7)
	if got := rt.openingShownCount("run-1"); got != 50 {
		t.Fatalf("opening shown = %d, want 50", got)
	}
	if got := rt.storiesShownCount("run-1"); got != 7 {
		t.Fatalf("verb shown = %d, want 7", got)
	}
	if got := rt.openingShownCount("run-2"); got != 0 {
		t.Fatalf("unreadable page counted %d", got)
	}
	rt.forgetStoriesShown("run-1")
	if rt.openingShownCount("run-1") != 0 || rt.storiesShownCount("run-1") != 0 {
		t.Fatal("forget left a count behind")
	}
}

// Defect 1: a cursor the host issued must pass the broker's own bound, at its
// longest (nine fractional digits, the longest document id).
func TestStoriesCursorFitsTheBrokerBound(t *testing.T) {
	at := time.Date(2026, 11, 19, 3, 39, 12, 123456789, time.UTC)
	for _, doc := range []string{strings.Repeat("a", 36), strings.Repeat("a", 64)} {
		next := storiesCursor(at, doc)
		payload, _ := json.Marshal(yaegikernel.StoriesPayload{Limit: 50, Before: next})
		if err := yaegikernel.ValidatePayload(yaegikernel.ActionStories, payload); err != nil {
			t.Errorf("a cursor of %d bytes the host issued is refused: %v", len(next), err)
		}
	}
	junk, _ := json.Marshal(yaegikernel.StoriesPayload{Before: strings.Repeat("x", yaegikernel.MaxStoriesBeforeBytes+1)})
	if yaegikernel.ValidatePayload(yaegikernel.ActionStories, junk) == nil {
		t.Error("an oversize before was accepted")
	}
}

// Each Cast a story's Texture sends to its research desk leaves one line with
// its cause decided at the source; nothing else does (another desk's Cast, a
// Cast to another inbox). A successor is a first round of a story whose
// trajectory names a succeeds ref.
func TestStoryResearchCastLineCarriesItsCauseAtTheSource(t *testing.T) {
	rt, s := testRuntime(t)
	owner, computer := "owner-research-cause", rt.cfg.ComputerID
	t0 := time.Now().UTC().Add(-time.Hour)
	plain := openStoryOf(t, s, owner, computer, "cell-1:cast:c1", "Plain story", "Whether the plain thing happens at all", "", []string{"c1"}, t0)
	succ := openStoryOf(t, s, owner, computer, "cell-2:cast:c1", "Next story", "What the plain thing led to afterwards", "texture:"+plain, []string{"c2"}, t0)
	var out strings.Builder
	prev := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(prev)
	rec := func(inbox string, opened bool, round int) types.InboxRecordReceipt {
		return types.InboxRecordReceipt{Inbox: inbox, OpenedRound: opened, Round: round}
	}
	ctx := context.Background()
	tex := deskmanifest.DocumentCustodianName()
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(plain), true, 1))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(succ), true, 1))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(plain), true, 3))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec(deskmanifest.ResearchInbox(plain), false, 3))
	rt.logStoryResearchCast(ctx, owner, computer, "run-t", tex, rec("management:o", true, 1))
	rt.logStoryResearchCast(ctx, owner, computer, "run-m", "management", rec(deskmanifest.ResearchInbox(plain), true, 1))
	lines := []string{}
	for _, l := range strings.Split(out.String(), "\n") {
		if i := strings.Index(l, "story research cast"); i >= 0 {
			lines = append(lines, l[i:])
		}
	}
	want := []string{
		"story research cast run=run-t story=" + plain + " round=1 opened=true cause=open",
		"story research cast run=run-t story=" + succ + " round=1 opened=true cause=successor",
		"story research cast run=run-t story=" + plain + " round=3 opened=true cause=feed",
		"story research cast run=run-t story=" + plain + " round=3 opened=false cause=continue",
	}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("lines = %q\nwant   %q", lines, want)
	}
}
