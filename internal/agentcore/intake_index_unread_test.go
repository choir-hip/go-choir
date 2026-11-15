package agentcore

import (
	"context"
	"testing"
	"time"
)

// A flush whose story index was not reconciled or could not be read leaves
// every capture unlabelled (docs/problems/jev-misuse-structural-assessment-
// 2026-11-15.md). Before this change a lone capture is written "new" with
// probability 1 and model "none", and a flush of two or more is asked against
// its siblings only and written with Jev's model and a confident probability,
// both indistinguishable from a full judgment.
//
// Failure list, written before the code:
//
//	B1. The story index read fails (its budget has run out): a lone capture is
//	    unlabelled, no label is written, no Jev call is made.
//	B2. The same for a flush of two or more captures: none is asked against
//	    siblings only, none is written, no Jev call is made.
//	B3. A read that succeeds and finds no stories (a genuinely empty index)
//	    keeps today's behavior: a lone capture is labelled new, model none,
//	    with no call, and a flush of two or more is asked.
//	B4. The counts say it: labelled 0, unlabelled N.

func unreadIndex(t *testing.T) {
	t.Helper()
	prev := intakeLabelPrepBudget
	// A negative budget gives the prep context a deadline already past, so
	// PublishStoryIndex fails at once, whatever the machine's speed. (The
	// ListStoryIndex branch is not reachable apart from it without a
	// production seam: both calls share the one prep context.)
	intakeLabelPrepBudget = -time.Second
	t.Cleanup(func() { intakeLabelPrepBudget = prev })
}

func TestIntakeUnreadStoryIndexLeavesALoneCaptureUnlabelled(t *testing.T) {
	rt, s := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	owner, computer := "owner-unread-1", rt.cfg.ComputerID
	unreadIndex(t)
	items := flushTestItems(1, "s")
	labelled, unlabelled := labelNow(rt, context.Background(), owner, computer, "src", items)
	if labelled != 0 || unlabelled != 1 {
		t.Fatalf("B4: labelled %d unlabelled %d; want 0 and 1", labelled, unlabelled)
	}
	if got, err := s.CaptureLabels(context.Background(), owner, computer, items[0].CaptureID); err != nil || len(got) != 0 {
		t.Fatalf("B1: labels %+v, %v; want none written", got, err)
	}
	if len(judge.calls) != 0 {
		t.Fatalf("B1: %d Jev calls", len(judge.calls))
	}
}

func TestIntakeUnreadStoryIndexLeavesEveryCaptureOfALargerFlushUnlabelled(t *testing.T) {
	rt, s := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	owner, computer := "owner-unread-2", rt.cfg.ComputerID
	unreadIndex(t)
	items := flushTestItems(3, "s")
	labelled, unlabelled := labelNow(rt, context.Background(), owner, computer, "src", items)
	if labelled != 0 || unlabelled != 3 {
		t.Fatalf("B4: labelled %d unlabelled %d; want 0 and 3", labelled, unlabelled)
	}
	for _, it := range items {
		if got, err := s.CaptureLabels(context.Background(), owner, computer, it.CaptureID); err != nil || len(got) != 0 {
			t.Fatalf("B2: %s labels %+v, %v; want none written", it.CaptureID, got, err)
		}
	}
	if len(judge.calls) != 0 {
		t.Fatalf("B2: %d Jev calls against siblings only", len(judge.calls))
	}
}

func TestIntakeEmptyStoryIndexKeepsTheHonestDefault(t *testing.T) {
	rt, s := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	owner, computer := "owner-empty-1", rt.cfg.ComputerID
	lone := flushTestItems(1, "s")
	if labelled, _ := labelNow(rt, context.Background(), owner, computer, "src", lone); labelled != 1 {
		t.Fatal("B3: a lone capture over an empty index is not labelled")
	}
	got, err := s.CaptureLabels(context.Background(), owner, computer, lone[0].CaptureID)
	if err != nil || len(got) != 1 || got[0].Model != intakeNoCandidatesModel {
		t.Fatalf("B3: labels %+v, %v; want one, model none", got, err)
	}
	if len(judge.calls) != 0 {
		t.Fatalf("B3: %d Jev calls for a lone capture with nothing to compare", len(judge.calls))
	}
	several := flushTestItems(3, "t")
	for i := range several {
		several[i].CaptureID += "-b"
	}
	if labelled, _ := labelNow(rt, context.Background(), owner, computer, "src", several); labelled != 3 {
		t.Fatalf("B3: %d of 3 captures labelled over an empty index", labelled)
	}
	if len(judge.calls) == 0 {
		t.Fatal("B3: a flush of several captures over an empty index was not asked")
	}
}
