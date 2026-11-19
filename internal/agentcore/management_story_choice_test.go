package agentcore

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

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
	if got := managementStoryChoiceLine("run-9", "feed", "doc-4", 50, 25); got != "management story choice run=run-9 choice=feed story=doc-4 opening_shown=50 verb_shown=25 candidates_shown=75" {
		t.Fatalf("feed line = %q", got)
	}
	if got := managementStoryChoiceLine("run-9", "open", "", 0, 0); got != "management story choice run=run-9 choice=open story=- opening_shown=0 verb_shown=0 candidates_shown=0" {
		t.Fatalf("open line = %q", got)
	}
}

// Defect 2 (recurring-agent-failures record, 2026-11-19): the line counted only
// choir.Stories calls, not the page the task's opening delivered.
func TestOpeningStoriesShownIsCountedSeparately(t *testing.T) {
	rt := &Runtime{}
	page, _ := json.Marshal(yaegikernel.StoriesPage{Stories: make([]yaegikernel.StoryEntry, 50)})
	rt.noteOpeningStoriesShown("run-1", page)
	rt.noteOpeningStoriesShown("run-2", json.RawMessage(`not json`))
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
