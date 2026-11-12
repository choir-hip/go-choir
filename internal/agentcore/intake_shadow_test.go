package agentcore

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// The shadow comparison. Ways it can fail, written before the code:
//
//	H1. The plan-only form makes a call (it must be free to run anywhere).
//	H2. The old arm is not the old behaviour: it must cut a 450-story window
//	    into several 15,000-byte slices, so its calls and bytes are the
//	    baseline the audit measured (3 to 5 calls per capture group).
//	H3. The new arm does not beat the old on calls and request bytes for the
//	    same flush, which is the point of the change.
//	H4. A run reports agreement 1.0 when the two arms disagree, or counts a
//	    capture the new arm left unlabelled as agreeing.

func shadowFlush(stories, captures int) IntakeShadowInput {
	now := time.Now().UTC()
	var in IntakeShadowInput
	for i := 0; i < stories; i++ {
		in.Stories = append(in.Stories, IntakeShadowStory{DocID: fmt.Sprintf("doc-%03d", i), Headline: fmt.Sprintf("Story headline %03d about the matter", i), UpdatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	for i := 0; i < captures; i++ {
		in.Captures = append(in.Captures, IntakeShadowCapture{CaptureID: fmt.Sprintf("cap-%02d", i), Title: fmt.Sprintf("Capture %d", i), Summary: "A short summary of the capture.", Outlet: "Wire", Language: "en"})
	}
	return in
}

func TestIntakeShadowPlanOldCutsSlicesNewIsOneState(t *testing.T) {
	judge := &fakeIntakeJudge{}
	in := shadowFlush(450, 6)
	rep, err := PlanIntakeShadow(in)
	if err != nil {
		t.Fatal(err)
	}
	if judge.callCount() != 0 {
		t.Fatal("H1: the plan made a call")
	}
	if rep.Old.Calls < 3 {
		t.Fatalf("H2: the old arm made %d calls for 6 captures over 450 stories; want 3 or more (15,000-byte slices)", rep.Old.Calls)
	}
	if rep.New.Calls >= rep.Old.Calls || rep.New.Bytes >= rep.Old.Bytes || rep.New.EstTokens >= rep.Old.EstTokens {
		t.Fatalf("H3: new %+v is not smaller than old %+v", rep.New, rep.Old)
	}
	if rep.New.Calls > 2 {
		t.Fatalf("H3: %d calls for one flush of 6 captures over 450 stories", rep.New.Calls)
	}
	if rep.Old.EstCostUSD <= 0 || rep.New.EstCostUSD <= 0 {
		t.Fatalf("the plan must price both arms: %+v", rep)
	}
}

func TestIntakeShadowRunAgreesWhenBothArmsAnswerAlike(t *testing.T) {
	in := shadowFlush(120, 4)
	judge := &fakeIntakeJudge{answers: map[string]string{"Capture 0": "Story headline 003 about the matter", "Capture 1": "Story headline 100 about the matter"},
		topics: map[string]string{"Capture 2": "Story headline 050 about the matter"}}
	rep, err := RunIntakeShadow(context.Background(), judge, in)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Run || rep.Agreement == nil {
		t.Fatalf("H4: a run must carry an agreement: %+v", rep)
	}
	a := rep.Agreement
	if a.Captures != 4 || a.EventAgree != 4 || a.TopicAgree != 4 || a.Event != 1 || a.Topic != 1 {
		t.Fatalf("H4: identical answers must agree: %+v", a)
	}
	if rep.New.InputTokens == 0 || rep.Old.InputTokens == 0 {
		t.Fatalf("a run reports the billed tokens of each arm: %+v / %+v", rep.Old, rep.New)
	}
}

func TestIntakeShadowAgreementSeesADisagreementAndAnUnlabelledCapture(t *testing.T) {
	in := shadowFlush(5, 3)
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{answers: map[string]string{"Capture 0": "Story headline 001 about the matter"}}
	rt.SetCandidateJudge(judge)
	old := rt.labelPollWith(context.Background(), in.library(rt), "o", "c", "s", in.items(), 6, intakeLegacyBytePolicy)
	cur := rt.labelPollWith(context.Background(), in.library(rt), "o", "c", "s", in.items(), 6, intakeTokenPolicy)
	cur[0].Value.EventDocID = "doc-other" // the new arm named another story
	cur[2].Err = fmt.Errorf("Jev gave no event answer")
	a := intakeShadowAgreement(in, old, cur)
	if a.Captures != 2 || a.EventAgree != 1 || len(a.Disagreements) != 2 {
		t.Fatalf("H4: %+v", a)
	}
}
