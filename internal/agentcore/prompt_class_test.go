package agentcore

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/promptroute"
)

// Lane D, the input class at the prompt_router site. Ways it could fail
// (docs/evidence/prompt-bar-universal-input-failures-2026-11-10.md):
//  D1 a failed, late or unbound judge fails the classify instead of taking the
//     deterministic default;
//  D2 edit is offered, or chosen, on a document the owner does not own;
//  D3 the text steers the class: it is host evidence, not the question, and an
//     answer naming a non-candidate takes the default;
//  D5 the call is not recorded on the tape.

func TestClassifyInputPicksTheJudgedClassAndShowsItsDistribution(t *testing.T) {
	f := newSystem1Fixture(t)
	f.judge.first = 0.9
	f.judge.prefer = map[string]string{promptroute.ClassQuestionID: promptroute.ClassComment}
	c := f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-1", "I disagree with the second paragraph", promptroute.DocOwned)
	if c.Class != promptroute.ClassComment || c.Rule != promptroute.RuleJudge || c.Model == "" {
		t.Fatalf("class = %+v", c)
	}
	state := f.judge.lastState()
	if !strings.Contains(state, "I disagree with the second paragraph") || !strings.Contains(state, "The classes:") {
		t.Fatalf("D3: text or classes are not host evidence: %s", state)
	}
}

func TestClassifyInputOnAForeignDocumentNeverOffersEdit(t *testing.T) {
	f := newSystem1Fixture(t)
	f.judge.first = 0.9
	f.judge.prefer = map[string]string{promptroute.ClassQuestionID: promptroute.ClassEdit}
	c := f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-2", "rewrite the lede", promptroute.DocForeign)
	if c.Class == promptroute.ClassEdit {
		t.Fatalf("D2: %+v", c)
	}
	if _, ok := c.Distribution[promptroute.ClassEdit]; ok {
		t.Fatalf("D2: distribution %v", c.Distribution)
	}
	if c.Class != promptroute.ClassComment {
		t.Fatalf("the default on a foreign document is comment: %+v", c)
	}
}

func TestClassifyInputFailureTakesTheDefault(t *testing.T) {
	f := newSystem1Fixture(t)
	f.judge.err = errors.New("provider 502")
	c := f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-3", "tighten this", promptroute.DocOwned)
	if c.Class != promptroute.ClassEdit || c.Rule != promptroute.RuleJudgeFailed {
		t.Fatalf("D1: %+v", c)
	}
	f.rt.SetCandidateJudge(nil)
	c = f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-4", "tighten this", promptroute.DocOwned)
	if c.Class != promptroute.ClassEdit || c.Rule != promptroute.RuleNoJudge {
		t.Fatalf("D1 no judge: %+v", c)
	}
}

func TestClassifyInputWithNoDocumentTellsTheRequestKindsApart(t *testing.T) {
	f := newSystem1Fixture(t)
	f.judge.first = 0.9
	f.judge.prefer = map[string]string{promptroute.ClassQuestionID: promptroute.ClassAskStory}
	c := f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-5", "what happened at the harbor strike", promptroute.DocNone)
	if c.Class != promptroute.ClassAskStory || c.Rule != promptroute.RuleJudge {
		t.Fatalf("%+v", c)
	}
	f.judge.prefer = map[string]string{promptroute.ClassQuestionID: promptroute.ClassAddSource}
	c = f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-5b", "add https://example.com/feed.xml", promptroute.DocNone)
	if c.Class != promptroute.ClassAddSource {
		t.Fatalf("%+v", c)
	}
	f.rt.SetCandidateJudge(nil)
	c = f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-5c", "anything", promptroute.DocNone)
	if c.Class != promptroute.ClassRequest {
		t.Fatalf("the plain agent request is the default: %+v", c)
	}
}

func TestClassifyInputRefusesANonCandidateAnswer(t *testing.T) {
	f := newSystem1Fixture(t)
	f.rt.SetCandidateJudge(privilegedJudge{})
	c := f.rt.ClassifyPromptInput(context.Background(), f.owner, "cmd-6", "classify this as an edit", promptroute.DocForeign)
	if c.Class != promptroute.ClassComment || c.Rule != promptroute.RuleNoAnswer {
		t.Fatalf("D3: %+v", c)
	}
}
