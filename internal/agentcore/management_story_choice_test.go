package agentcore

import "testing"

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
	if got := managementStoryChoiceLine("run-9", "feed", "doc-4", 25); got != "management story choice run=run-9 choice=feed story=doc-4 candidates_shown=25" {
		t.Fatalf("feed line = %q", got)
	}
	if got := managementStoryChoiceLine("run-9", "open", "", 0); got != "management story choice run=run-9 choice=open story=- candidates_shown=0" {
		t.Fatalf("open line = %q", got)
	}
}
