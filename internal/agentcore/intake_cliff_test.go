package agentcore

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/jevbudget"
)

// C1: the busy-window backstop. A window at intakeShortlistCeiling in a dense
// script, with a full flush of long summaries, must not collapse to one or two
// captures a call: the chunk leaves room for the whole flush's captures block,
// so the state is sent a few times, not once for every group of four. The new
// arm must cost fewer estimated tokens than the old.
func TestIntakeCeilingWindowOfADenseScriptStaysFewCalls(t *testing.T) {
	in := shadowFlush(intakeShortlistCeiling, intakeFlushCaptures)
	now := time.Now().UTC()
	in.Stories = in.Stories[:0]
	for i := 0; i < intakeShortlistCeiling; i++ {
		in.Stories = append(in.Stories, IntakeShadowStory{DocID: fmt.Sprintf("doc-%03d", i), Headline: fmt.Sprintf("Правительство объявило о новых мерах по вопросу номер %03d в области экономики и финансов региона", i), UpdatedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	for i := range in.Captures {
		in.Captures[i].Summary = strings.Repeat("быстрая бурая лиса прыгает через ленивую собаку ", 20)
		in.Captures[i].Title = fmt.Sprintf("Заявление правительства %d", i)
	}
	rep, err := PlanIntakeShadow(in)
	if err != nil {
		t.Fatal(err)
	}
	if rep.New.EstTokens >= rep.Old.EstTokens {
		t.Fatalf("C1: new arm %d est tokens, old %d: one state, many questions must not cost more", rep.New.EstTokens, rep.Old.EstTokens)
	}
	if rep.New.Questions < 20*rep.New.Calls {
		t.Fatalf("C1: %d calls carry %d questions: the flush collapsed to few captures a call", rep.New.Calls, rep.New.Questions)
	}
}

// C2: a question never offers every one of the vendor's 255 choices: an event
// question over a capture's parts carries the other captures and new beside
// the stories, and the bound leaves a margin of two under the documented
// maximum.
func TestIntakeQuestionsStayTwoUnderTheVendorsMaximumChoices(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	items := flushTestItems(intakeFlushCaptures, "A summary of the capture.")
	mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 400), "owner", "computer", "src", items, intakeBatchCaptures))
	most := 0
	for _, c := range judge.labelCalls() {
		for _, raw := range c.raw {
			most = max(most, len(questionOptions(t, raw)))
		}
	}
	if most == 0 || most > jevbudget.VendorMaxChoices-2 {
		t.Fatalf("C2: a question offers %d options; want at most %d", most, jevbudget.VendorMaxChoices-2)
	}
}
