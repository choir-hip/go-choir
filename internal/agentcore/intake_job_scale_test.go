package agentcore

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// Ways the intake job can still be decided by arithmetic of ours (the third
// review round), written before the fixes:
//
//	S1. Siblings are trimmed by shared title words, so a translated or
//	    reworded duplicate is never offered and is labelled new.
//	S2. A part is smaller than the 255 choices allow because the part sizing
//	    subtracts the poll's capture count where the offered siblings count.
//	S3. A flush is cut at a constant of ours before the planner sees it, so
//	    captures past it are never offered to the ones before it.
func scaleItems(n int, titleOf func(i int) string) (titles []string) {
	for i := 0; i < n; i++ {
		titles = append(titles, titleOf(i))
	}
	return titles
}

// S1: forty captures whose titles share no words each offer all thirty-nine
// others, over the parts of their questions.
func TestIntakeJobOffersEveryOtherCaptureNeverTrimmedByTitleWords(t *testing.T) {
	rt, _ := testRuntime(t)
	items := pollItems(scaleItems(40, func(i int) string { return fmt.Sprintf("Zq%c%c unrelated", 'a'+i%26, 'a'+i/26) })...)
	lib := pollLib(rt, 5)
	p, err := newIntakePoll(context.Background(), lib, items, intakeTokenPolicy)
	if err != nil {
		t.Fatal(err)
	}
	live := make([]int, len(items))
	for i := range live {
		live[i] = i
	}
	calls, err := rt.planIntakeJobCalls(lib, p, live)
	if err != nil {
		t.Fatal(err)
	}
	offered := map[string]map[string]bool{}
	for _, c := range calls {
		for name, raw := range c.questions {
			if !strings.HasPrefix(name, "event_") {
				continue
			}
			handle, _, _ := strings.Cut(strings.TrimPrefix(name, "event_"), "_")
			if offered[handle] == nil {
				offered[handle] = map[string]bool{}
			}
			for _, o := range questionOptions(t, raw) {
				if strings.HasPrefix(o, "n") && o != "new" {
					offered[handle][o] = true
				}
			}
		}
	}
	for i := range items {
		h := fmt.Sprintf("n%d", i+1)
		if len(offered[h]) != 39 {
			t.Fatalf("S1: capture %s is offered %d of 39 other captures", h, len(offered[h]))
		}
	}
}

// S2: 152 captures over 400 short stories. A capture's options are its 400
// stories and 151 other captures, 551 in all, so the fewest parts under the
// 255-choice limit (252 options a part) is three; each capture asks three
// event questions and, for the two parts that hold stories, two topic
// questions. 152 x 5 = 760 questions, and the planner may only split them on
// the token limits.
func TestIntakeJobPartsAreAsLargeAsTheChoiceLimitAllows(t *testing.T) {
	rt, _ := testRuntime(t)
	items := pollItems(scaleItems(152, func(i int) string { return fmt.Sprintf("Capture %03d", i) })...)
	for i := range items {
		items[i].Summary = "s"
	}
	lib := pollLib(rt, 400)
	p, err := newIntakePoll(context.Background(), lib, items, intakeTokenPolicy)
	if err != nil {
		t.Fatal(err)
	}
	live := make([]int, len(items))
	for i := range live {
		live[i] = i
	}
	calls, err := rt.planIntakeJobCalls(lib, p, live)
	if err != nil {
		t.Fatal(err)
	}
	questions, most := 0, 0
	for _, c := range calls {
		questions += len(c.questions)
		for _, raw := range c.questions {
			most = max(most, jevbudget.MaxOptions(raw))
		}
		if !jevbudget.Fits(c.state, c.questions) {
			t.Fatalf("S2: a call is over %s", jevbudget.Limit(c.state, c.questions))
		}
	}
	if questions != 152*5 {
		t.Fatalf("S2: %d questions; want 760 (three parts a capture)", questions)
	}
	if most != intakeOptionsMax+1 && most != intakeOptionsMax {
		t.Fatalf("S2: the largest question offers %d options; want a full part of %d (plus new)", most, intakeOptionsMax)
	}
}

// S3: 200 captures waiting are one flush and one job, whatever the planner
// then splits on the token limits: the first capture is offered the last.
func TestIntakeFlushTakesEveryWaitingCaptureIntoOneJob(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-all")
	f.enable(t)
	judge := &fakeIntakeJudge{}
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(200, "s")
	h := f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items)
	waitClosed(t, "the flush", h)
	offeredLast := false
	for _, c := range judge.labelCalls() {
		for name, raw := range c.raw {
			if name != "event_n1" && !strings.HasPrefix(name, "event_n1_") {
				continue
			}
			for _, o := range questionOptions(t, raw) {
				offeredLast = offeredLast || o == "n200"
			}
		}
	}
	if !offeredLast {
		t.Fatal("S3: capture 1 is never offered capture 200: the flush was cut before the job")
	}
}
